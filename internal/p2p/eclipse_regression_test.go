// eclipse_regression_test.go P0-5 回归测试：Eclipse 缓解。
//
// 覆盖：
//  1. 外拨槽位预留（入站打满不堵死外拨）
//  2. per-IP 入站上限（回环豁免）
//  3. 外拨 /16 多样性（回环豁免）
//  4. 不良行为记分 → 封禁 → 封禁期拒绝连接
//  5. 记分衰减
//  6. get_blocks 洪水检测
//  7. 种子优先驱逐
//  8. 地址本硬化（上限/格式校验）
package p2p

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// fakePeer 构造测试用 Peer（直接注入 n.peers，不经过网络）。
func fakePeer(addr string, inbound bool, handshaked bool) *Peer {
	p := &Peer{Addr: addr, inbound: inbound, lastActive: time.Now(), closed: make(chan struct{})}
	if handshaked {
		p.handshaked.Store(true)
	}
	return p
}

// fillPeers 向节点注入 n 个 fake peer。
func fillPeers(n *Node, addrs []string, inbound bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, a := range addrs {
		n.peers[a] = fakePeer(a, inbound, true)
	}
}

// eclipseTestHandler 空实现（本文件只测连接管理，不测业务分发）。
type eclipseTestHandler struct{}

func (eclipseTestHandler) OnHandshake(string, HandshakePayload)             {}
func (eclipseTestHandler) OnNewBlock(string, json.RawMessage)               {}
func (eclipseTestHandler) OnNewTx(string, json.RawMessage)                  {}
func (eclipseTestHandler) OnGetBlocks(string, GetBlocksPayload)             {}
func (eclipseTestHandler) OnBlocksResp(string, BlocksRespPayload)           {}
func (eclipseTestHandler) OnGetBlockByHash(string, GetBlockByHashPayload)   {}
func (eclipseTestHandler) OnBlockByHashResp(string, BlockByHashRespPayload) {}

func newEclipseTestNode(t *testing.T) *Node {
	t.Helper()
	return NewNode("127.0.0.1:0", "test-node", "test-genesis", eclipseTestHandler{})
}

// TestOutboundSlotReservation P0-5 D5-1：入站打满 maxPeers 时，外拨预留仍可用。
func TestOutboundSlotReservation(t *testing.T) {
	n := newEclipseTestNode(t)

	// 入站填满到 maxPeers（128）；per-IP 上限为 4，构造时分散 IP
	addrs := make([]string, 0, maxPeers)
	for i := 0; i < maxPeers; i++ {
		addrs = append(addrs, fmt.Sprintf("10.%d.%d.%d:1000", i/65025, (i/255)%255, i%255+1))
	}
	fillPeers(n, addrs, true)

	n.mu.Lock()
	reason := n.peerLimitRejectionLocked(&Peer{Addr: "10.9.9.9:2000", inbound: true})
	n.mu.Unlock()
	if reason == "" {
		t.Fatalf("入站已满时新入站应被拒绝")
	}

	// 外拨：预留未用满 → 放行
	n.mu.Lock()
	reason = n.peerLimitRejectionLocked(&Peer{Addr: "10.9.9.9:2000", inbound: false})
	n.mu.Unlock()
	if reason != "" {
		t.Fatalf("外拨预留未用满时外拨应放行，实际拒绝：%s", reason)
	}

	// 外拨填满预留（16）→ 新外拨也拒绝
	outAddrs := make([]string, 0, maxOutboundReserved)
	for i := 0; i < maxOutboundReserved; i++ {
		outAddrs = append(outAddrs, fmt.Sprintf("11.0.%d.1:1000", i+1))
	}
	fillPeers(n, outAddrs, false)
	n.mu.Lock()
	reason = n.peerLimitRejectionLocked(&Peer{Addr: "10.9.9.9:2000", inbound: false})
	n.mu.Unlock()
	if reason == "" {
		t.Fatalf("外拨预留用满后新外拨应被拒绝")
	}
}

// TestPerIPInboundCap P0-5 D5-1：单 IP 入站上限 4；回环豁免。
func TestPerIPInboundCap(t *testing.T) {
	n := newEclipseTestNode(t)
	for i := 0; i < maxInboundPerIP; i++ {
		n.mu.Lock()
		n.peers[fmt.Sprintf("10.1.2.3:%d", 1000+i)] = fakePeer(fmt.Sprintf("10.1.2.3:%d", 1000+i), true, true)
		n.mu.Unlock()
	}
	n.mu.Lock()
	reason := n.peerLimitRejectionLocked(&Peer{Addr: "10.1.2.3:2000", inbound: true})
	n.mu.Unlock()
	if reason == "" {
		t.Fatalf("同一 IP 第 5 条入站应被拒绝（per-IP 上限 %d）", maxInboundPerIP)
	}

	// 回环豁免：127.0.0.1 不受 per-IP 限制
	n2 := newEclipseTestNode(t)
	for i := 0; i < maxInboundPerIP+2; i++ {
		n2.mu.Lock()
		n2.peers[fmt.Sprintf("127.0.0.1:%d", 3000+i)] = fakePeer(fmt.Sprintf("127.0.0.1:%d", 3000+i), true, true)
		n2.mu.Unlock()
	}
	n2.mu.Lock()
	reason = n2.peerLimitRejectionLocked(&Peer{Addr: "127.0.0.1:4000", inbound: true})
	n2.mu.Unlock()
	if reason != "" {
		t.Fatalf("回环地址应豁免 per-IP 限制，实际拒绝：%s", reason)
	}
}

// TestOutboundSubnetDiversity P0-5 D5-1：同一 /16 最多 2 条外拨；回环豁免。
func TestOutboundSubnetDiversity(t *testing.T) {
	n := newEclipseTestNode(t)
	fillPeers(n, []string{"10.5.1.1:1000", "10.5.2.2:1000"}, false)

	// 第三条同 /16 外拨 → ConnectToPeer 应在拨号前拒绝（用不可达地址验证"拨号前"）
	err := n.ConnectToPeer("10.5.9.9:9999")
	if err == nil {
		t.Fatalf("同 /16 第三条外拨应被拒绝")
	}
	// 不同 /16 → 通过多样性检查（拨号失败是预期的，但错误不应是 /16 上限）
	err = n.ConnectToPeer("10.6.9.9:9999")
	if err != nil && strings.Contains(err.Error(), "/16") {
		t.Fatalf("不同 /16 不应触发多样性拒绝：%v", err)
	}
	// 回环豁免
	err = n.ConnectToPeer("127.0.0.1:9999")
	if err != nil && strings.Contains(err.Error(), "/16") {
		t.Fatalf("回环应豁免 /16 限制：%v", err)
	}
}

// TestPenalizeAndBan P0-5 D5-3：记分达阈值 → 封禁 IP → 封禁期拒绝连接。
func TestPenalizeAndBan(t *testing.T) {
	n := newEclipseTestNode(t)
	addr := "10.7.7.7:1234"

	n.Penalize(addr, 50, "测试记分")
	if n.isBannedIP("10.7.7.7") {
		t.Fatalf("50 分未达阈值，不应封禁")
	}
	n.Penalize(addr, 50, "测试记分")
	if !n.isBannedIP("10.7.7.7") {
		t.Fatalf("累计 100 分应触发封禁")
	}

	// banIP 应断开该 IP 的全部现有连接（回环豁免记分，故用非回环 IP 验证）
	n2 := newEclipseTestNode(t)
	victim := fakePeer("10.8.8.8:1111", true, true)
	c1, _ := net.Pipe()
	victim.Conn = c1
	defer c1.Close()
	n2.mu.Lock()
	n2.peers[victim.Addr] = victim
	n2.mu.Unlock()
	n2.Penalize("10.8.8.8:2222", banThreshold, "测试封禁断开")
	n2.mu.RLock()
	_, stillThere := n2.peers[victim.Addr]
	n2.mu.RUnlock()
	if stillThere {
		t.Fatalf("封禁时应断开该 IP 的全部现有连接")
	}
}

// TestScoreDecay P0-5 D5-3：记分按 60s 周期半衰。
func TestScoreDecay(t *testing.T) {
	n := newEclipseTestNode(t)
	addr := "10.9.9.9:1234"
	n.Penalize(addr, 60, "测试")
	// 回拨 updated 时间，模拟经过 61s
	n.mu.Lock()
	n.scores["10.9.9.9"].updated = time.Now().Add(-61 * time.Second)
	n.mu.Unlock()
	n.Penalize(addr, 10, "测试")
	n.mu.RLock()
	total := n.scores["10.9.9.9"].points
	n.mu.RUnlock()
	if total != 40 { // 60/2 + 10
		t.Fatalf("衰减后记分应为 40，实际 %d", total)
	}
	if n.isBannedIP("10.9.9.9") {
		t.Fatalf("40 分不应触发封禁")
	}
}

// TestSyncFloodDetection P0-5 D5-3：窗口内超 20 次 get_blocks → 判洪水并记分。
func TestSyncFloodDetection(t *testing.T) {
	n := newEclipseTestNode(t)
	addr := "10.10.10.10:1234"
	flooded := false
	for i := 0; i < syncFloodMax+1; i++ {
		if n.NoteSyncRequest(addr) {
			flooded = true
		}
	}
	if !flooded {
		t.Fatalf("第 %d 次请求应判定为洪水", syncFloodMax+1)
	}
	n.mu.RLock()
	pts := n.scores["10.10.10.10"].points
	n.mu.RUnlock()
	if pts < scoreSyncFlood {
		t.Fatalf("洪水应记分，实际 %d", pts)
	}
}

// TestSeedEviction P0-5 D5-2：满员时 ConnectToSeed 驱逐价值最低入站。
func TestSeedEviction(t *testing.T) {
	n := newEclipseTestNode(t)
	// 填满 maxPeers，全部入站；其中一个未握手（最老），应被优先驱逐
	victimAddr := "10.11.0.1:1000"
	for i := 0; i < maxPeers; i++ {
		a := fmt.Sprintf("10.11.%d.%d:1000", (i+1)/255, (i+1)%255+1)
		p := fakePeer(a, true, true)
		p.lastActive = time.Now().Add(-time.Duration(i) * time.Second)
		n.mu.Lock()
		n.peers[a] = p
		n.mu.Unlock()
	}
	// 替换其中一个为未握手最老
	n.mu.Lock()
	delete(n.peers, victimAddr) // 确保不存在
	v := fakePeer(victimAddr, true, false)
	v.lastActive = time.Now().Add(-time.Hour)
	// 腾一个位置给 victim（保持总数 maxPeers）
	for a := range n.peers {
		delete(n.peers, a)
		break
	}
	n.peers[victimAddr] = v
	n.mu.Unlock()

	n.mu.Lock()
	got := n.evictLowestValueInboundLocked()
	n.mu.Unlock()
	if got == nil || got.Addr != victimAddr {
		t.Fatalf("应驱逐未握手最老的入站 %s，实际 %v", victimAddr, got)
	}

	// 无入站时返回 nil
	n2 := newEclipseTestNode(t)
	fillPeers(n2, []string{"10.12.0.1:1000"}, false)
	n2.mu.Lock()
	got = n2.evictLowestValueInboundLocked()
	n2.mu.Unlock()
	if got != nil {
		t.Fatalf("无入站时应返回 nil")
	}
}

// TestLearnPeerHardening P0-5 D5-4：格式校验 + 上限。
func TestLearnPeerHardening(t *testing.T) {
	n := newEclipseTestNode(t)
	n.learnPeer("not-an-addr")
	n.learnPeer("")
	if len(n.known) != 0 {
		t.Fatalf("非法地址不应进入地址本")
	}
	n.learnPeer("10.13.0.1:6688")
	if len(n.known) != 1 {
		t.Fatalf("合法地址应进入地址本")
	}
	// 上限
	for i := 0; i < maxKnownPeers+10; i++ {
		n.learnPeer(fmt.Sprintf("10.14.%d.%d:6688", i/255, i%255+1))
	}
	if len(n.known) > maxKnownPeers {
		t.Fatalf("地址本应限制在 %d，实际 %d", maxKnownPeers, len(n.known))
	}
}
