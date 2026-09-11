package p2p_test

import (
	"encoding/hex"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"p2pchain/internal/p2p"
)

// recordHandler 记录收到的消息，供断言使用。
type recordHandler struct {
	mu         sync.Mutex
	handshakes []p2p.HandshakePayload
	blocks     []string
	txs        []string
	getBlocks  []p2p.GetBlocksPayload
	blockResps []p2p.BlocksRespPayload
}

func (h *recordHandler) OnHandshake(_ string, payload p2p.HandshakePayload) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handshakes = append(h.handshakes, payload)
}

func (h *recordHandler) OnNewBlock(_ string, raw json.RawMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.blocks = append(h.blocks, string(raw))
}

func (h *recordHandler) OnNewTx(_ string, raw json.RawMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.txs = append(h.txs, string(raw))
}

func (h *recordHandler) OnGetBlocks(_ string, payload p2p.GetBlocksPayload) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.getBlocks = append(h.getBlocks, payload)
}

func (h *recordHandler) OnBlocksResp(_ string, payload p2p.BlocksRespPayload) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.blockResps = append(h.blockResps, payload)
}

func (h *recordHandler) counts() (int, int, int, int, int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.handshakes), len(h.blocks), len(h.txs), len(h.getBlocks), len(h.blockResps)
}

// startTestNode 在随机端口启动一个节点，返回节点与地址。
// 监听器由测试创建后注入（Serve），因此返回时端口已确定处于监听状态。
func startTestNode(t *testing.T, h p2p.Handler) (*p2p.Node, string) {
	t.Helper()
	ln, err := netListenEphemeral()
	if err != nil {
		t.Fatalf("分配端口失败: %v", err)
	}
	addr := ln.Addr().String()
	n := p2p.NewNode(addr, "test-node", "genesis-hash", h)
	go func() { _ = n.Serve(ln) }()
	return n, addr
}

// ---- 用例 ----

// TestHandshakeOverRealTCP 两个节点真实 TCP 互联并完成握手。
func TestHandshakeOverRealTCP(t *testing.T) {
	h1 := &recordHandler{}
	h2 := &recordHandler{}
	n1, addr1 := startTestNode(t, h1)
	defer n1.Stop()
	n2, _ := startTestNode(t, h2)
	defer n2.Stop()

	n1.SetHeightProvider(func() int { return 7 })
	n2.SetHeightProvider(func() int { return 3 })

	if err := n2.ConnectToPeer(addr1); err != nil {
		t.Fatalf("连接失败: %v", err)
	}

	waitFor(t, func() bool {
		hs1, _, _, _, _ := h1.counts()
		hs2, _, _, _, _ := h2.counts()
		return hs1 >= 1 && hs2 >= 1
	}, 5*time.Second, "双方未完成握手")

	if n1.PeerCount() != 1 || n2.PeerCount() != 1 {
		t.Fatalf("对等连接数异常: n1=%d n2=%d", n1.PeerCount(), n2.PeerCount())
	}

	// 高度信息通过握手传递
	h1.mu.Lock()
	gotHeight := h1.handshakes[0].ChainHeight
	h1.mu.Unlock()
	if gotHeight != 3 {
		t.Fatalf("n1 收到的高度 = %d, want 3", gotHeight)
	}
}

// TestBroadcastBlockAndRelayExcept 区块广播能到达对端；except 参数避免回环。
func TestBroadcastBlockAndRelayExcept(t *testing.T) {
	h1 := &recordHandler{}
	h2 := &recordHandler{}
	h3 := &recordHandler{}
	n1, addr1 := startTestNode(t, h1)
	defer n1.Stop()
	n2, _ := startTestNode(t, h2)
	defer n2.Stop()
	n3, addr3 := startTestNode(t, h3)
	defer n3.Stop()

	// 星形拓扑：n2 连接 n1 与 n3
	if err := n2.ConnectToPeer(addr1); err != nil {
		t.Fatal(err)
	}
	if err := n2.ConnectToPeer(addr3); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return n1.PeerCount() == 1 && n3.PeerCount() == 1 }, 5*time.Second, "拓扑未建立")

	payload, _ := json.Marshal(p2p.BlockPayload{Encoded: hex.EncodeToString([]byte("fake-block-bytes"))})
	n2.Broadcast(p2p.Message{Type: p2p.MsgNewBlock, Payload: payload})

	waitFor(t, func() bool {
		_, b1, _, _, _ := h1.counts()
		_, b3, _, _, _ := h3.counts()
		return b1 == 1 && b3 == 1
	}, 5*time.Second, "区块广播未到达两端")

	// except：n2 广播时排除 n3，仅 n1 收到
	_, b1Before, _, _, _ := h1.counts()
	_, b3Before, _, _, _ := h3.counts()
	n2.BroadcastExcept(p2p.Message{Type: p2p.MsgNewBlock, Payload: payload}, addr3)
	waitFor(t, func() bool {
		_, b1Now, _, _, _ := h1.counts()
		return b1Now == b1Before+1
	}, 5*time.Second, "except 广播未送达 n1")
	time.Sleep(200 * time.Millisecond)
	_, b3Now, _, _, _ := h3.counts()
	if b3Now != b3Before {
		t.Fatalf("except 未生效：n3 收到了被排除的消息（%d → %d）", b3Before, b3Now)
	}
}

// TestGetBlocksAndResponse 同步请求与响应（定向发送）。
func TestGetBlocksAndResponse(t *testing.T) {
	h1 := &recordHandler{}
	h2 := &recordHandler{}
	n1, addr1 := startTestNode(t, h1)
	defer n1.Stop()
	n2, _ := startTestNode(t, h2)
	defer n2.Stop()

	if err := n2.ConnectToPeer(addr1); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return n1.PeerCount() == 1 && n2.PeerCount() == 1 }, 5*time.Second, "未连接")

	// n2 → n1 请求区块
	req, _ := json.Marshal(p2p.GetBlocksPayload{FromHeight: 1, Count: 10})
	if err := n2.SendTo(addr1, p2p.Message{Type: p2p.MsgGetBlocks, Payload: req}); err != nil {
		t.Fatalf("发送 GetBlocks 失败: %v", err)
	}
	waitFor(t, func() bool {
		_, _, _, g, _ := h1.counts()
		return g == 1
	}, 5*time.Second, "GetBlocks 未到达")
	h1.mu.Lock()
	got := h1.getBlocks[0]
	h1.mu.Unlock()
	if got.FromHeight != 1 || got.Count != 10 {
		t.Fatalf("GetBlocks 载荷错误: %+v", got)
	}

	// n1 → n2 响应区块（n1 侧对端地址为入站连接的临时端口，通过 PeerAddrs 获取）
	peers := n1.PeerAddrs()
	if len(peers) != 1 {
		t.Fatalf("n1 对等节点数 = %d, want 1", len(peers))
	}
	resp, _ := json.Marshal(p2p.BlocksRespPayload{
		EncodedBlocks: []string{hex.EncodeToString([]byte("blk1"))},
		Done:          true,
	})
	if err := n1.SendTo(peers[0], p2p.Message{Type: p2p.MsgBlocksResp, Payload: resp}); err != nil {
		t.Fatalf("发送 BlocksResp 失败: %v", err)
	}
	waitFor(t, func() bool {
		_, _, _, _, r := h2.counts()
		return r == 1
	}, 5*time.Second, "BlocksResp 未到达")
	h2.mu.Lock()
	gotResp := h2.blockResps[0]
	h2.mu.Unlock()
	if len(gotResp.EncodedBlocks) != 1 || !gotResp.Done {
		t.Fatalf("BlocksResp 载荷错误: %+v", gotResp)
	}
}

// TestDisconnectCleansUp 断开后对等连接数归零。
func TestDisconnectCleansUp(t *testing.T) {
	h1 := &recordHandler{}
	h2 := &recordHandler{}
	n1, addr1 := startTestNode(t, h1)
	defer n1.Stop()
	n2, _ := startTestNode(t, h2)
	defer n2.Stop()

	if err := n2.ConnectToPeer(addr1); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return n1.PeerCount() == 1 && n2.PeerCount() == 1 }, 5*time.Second, "未连接")

	n1.Stop()
	waitFor(t, func() bool { return n2.PeerCount() == 0 }, 5*time.Second, "断开后未清理连接")
}

// TestKnownPeersGossip 握手交换已知节点列表：n2→n1 后，n1 学会 addr2 并转告给后来接入的 n3。
func TestKnownPeersGossip(t *testing.T) {
	h1 := &recordHandler{}
	h2 := &recordHandler{}
	h3 := &recordHandler{}
	n1, addr1 := startTestNode(t, h1)
	defer n1.Stop()
	n2, addr2 := startTestNode(t, h2)
	defer n2.Stop()
	n3, _ := startTestNode(t, h3)
	defer n3.Stop()

	// 第一步：n2 连接 n1，n1 从握手中学习到 n2 的可达监听地址
	if err := n2.ConnectToPeer(addr1); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		hs, _, _, _, _ := h1.counts()
		return n1.PeerCount() == 1 && hs >= 1
	}, 5*time.Second, "未连接或握手未到达")

	// 握手载荷必须携带对端可达地址，供节点发现使用
	h1.mu.Lock()
	peerAddrInHandshake := h1.handshakes[0].ListenAddr
	h1.mu.Unlock()
	if peerAddrInHandshake != addr2 {
		t.Fatalf("对端握手未携带监听地址: %q want %q", peerAddrInHandshake, addr2)
	}

	waitFor(t, func() bool { return containsAddr(n1.KnownPeers(), addr2) }, 5*time.Second, "n1 未学习到 addr2")

	// 第二步：n3 连接 n1，n1 把自己的已知邻居（addr2）转告 n3
	if err := n3.ConnectToPeer(addr1); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return n3.PeerCount() == 1 }, 5*time.Second, "n3 未连上 n1")
	waitFor(t, func() bool { return containsAddr(n3.KnownPeers(), addr2) }, 5*time.Second, "邻居地址未通过握手转告")
}

// containsAddr 判断地址切片是否包含目标地址。
func containsAddr(addrs []string, target string) bool {
	for _, a := range addrs {
		if a == target {
			return true
		}
	}
	return false
}
