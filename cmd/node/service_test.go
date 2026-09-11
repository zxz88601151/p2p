package main

// 本文件是节点服务层的端到端测试：用两个「真实 TCP 节点 + 真实区块校验 + 真实 UTXO 状态机」
// 验证 P2P 接线是否正确，而不是只测网络层能否收发字节。
//
// 覆盖点：
//   - 区块广播：A 挖出区块 → 广播 → B 通过完整共识校验后上链
//   - 追赶同步：B 落后于 A 时，握手触发的 GetBlocks/BlocksResp 能把链补齐
//   - 非法区块拒绝：PoW 不合格的区块不得改变对端链高度
//   - 重复广播幂等：同一区块重复到达不产生副作用
//
// 这些都是「真实证据」而非桩数据：区块真实挖出（PoW 有效）、真实编解码、真实验签。

import (
	"encoding/hex"
	"encoding/json"
	"net"
	"testing"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/mempool"
	"p2pchain/internal/p2p"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

// ---- 测试脚手架 ----

func testWallet(t *testing.T) *wallet.Wallet {
	t.Helper()
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("生成钱包失败: %v", err)
	}
	return w
}

// testChain 用确定性创世区块建一条新链（两个节点共享同一创世 → 可互联）。
func testChain(t *testing.T) *blockchain.Blockchain {
	t.Helper()
	bc, err := blockchain.NewBlockchainWithGenesis(blockchain.NewGenesisBlock())
	if err != nil {
		t.Fatalf("初始化链失败: %v", err)
	}
	return bc
}

// startService 在随机端口启动一个由 svc 驱动的真实 P2P 节点。
// 监听器由这里创建后注入 Serve，因此函数返回时端口一定处于监听状态。
func startService(t *testing.T, svc *nodeService) (*p2p.Node, string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("分配端口失败: %v", err)
	}
	addr := ln.Addr().String()

	genesis, err := svc.chain.BlockByHeight(0)
	if err != nil {
		t.Fatalf("读取创世区块失败: %v", err)
	}
	node := p2p.NewNode(addr, "test", genesis.Header.HashHex(), svc)
	svc.net = node
	node.SetHeightProvider(func() int { return svc.chain.Height() })
	go func() { _ = node.Serve(ln) }()
	return node, addr
}

// waitFor 轮询等待条件成立。
func waitFor(t *testing.T, cond func() bool, timeout time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("等待超时: %s", msg)
}

// mineAndBroadcast 真实挖出并上链一个仅含 coinbase 的区块，然后广播。
func mineAndBroadcast(t *testing.T, svc *nodeService) *block.Block {
	t.Helper()
	b := mineOnly(t, svc)
	svc.broadcastBlock(b)
	return b
}

// mineOnly 真实挖出并上链一个区块（不广播）。
func mineOnly(t *testing.T, svc *nodeService) *block.Block {
	t.Helper()
	tip, err := svc.chain.Tip()
	if err != nil {
		t.Fatalf("读取链尾失败: %v", err)
	}
	height := svc.chain.Height() + 1
	cb := transaction.NewCoinbaseTx(svc.miner.PubKeyHash(), utxo.Subsidy(height), height)
	candidate := block.NewCandidateBlock(tip.Header.Hash(), svc.chain.CurrentBits(),
		[]*transaction.Transaction{cb})
	if found, _ := pow.Mine(candidate, 0); !found {
		t.Fatal("挖矿失败")
	}
	if err := svc.chain.AddBlock(candidate); err != nil {
		t.Fatalf("合法区块被拒绝: %v", err)
	}
	return candidate
}

// newServiceFor 组装一个「链 + 空交易池 + 新矿工钱包」的服务实例。
func newServiceFor(t *testing.T, bc *blockchain.Blockchain) *nodeService {
	t.Helper()
	return newNodeService(bc, mempool.New(128), testWallet(t))
}

func blockPayloadOf(t *testing.T, b *block.Block) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(p2p.BlockPayload{Encoded: hex.EncodeToString(b.Encode())})
	if err != nil {
		t.Fatalf("构造区块载荷失败: %v", err)
	}
	return raw
}

// ---- 用例 ----

// TestNodeServicePropagatesBlock 区块广播：A 挖出区块后 B 通过完整校验上链。
func TestNodeServicePropagatesBlock(t *testing.T) {
	svcA := newServiceFor(t, testChain(t))
	svcB := newServiceFor(t, testChain(t))
	nodeA, addrA := startService(t, svcA)
	defer nodeA.Stop()
	nodeB, _ := startService(t, svcB)
	defer nodeB.Stop()

	if err := nodeB.ConnectToPeer(addrA); err != nil {
		t.Fatalf("B 连接 A 失败: %v", err)
	}
	waitFor(t, func() bool { return nodeA.PeerCount() == 1 && nodeB.PeerCount() == 1 },
		5*time.Second, "两节点未建立连接")

	mined := mineAndBroadcast(t, svcA)

	waitFor(t, func() bool { return svcB.chain.Height() == 1 }, 5*time.Second, "B 未收到并接受区块")

	tipB, err := svcB.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	if tipB.Header.Hash() != mined.Header.Hash() {
		t.Fatalf("B 链尾哈希与 A 挖出的区块不一致")
	}
}

// TestNodeServiceCatchUpSync B 落后时通过握手触发的同步请求把链补齐。
func TestNodeServiceCatchUpSync(t *testing.T) {
	svcA := newServiceFor(t, testChain(t))
	svcB := newServiceFor(t, testChain(t))

	// A 先独自挖出 3 个区块，B 此时完全不知情（高度仍为 0）
	var hashes [][32]byte
	for i := 0; i < 3; i++ {
		b := mineOnly(t, svcA)
		hashes = append(hashes, b.Header.Hash())
	}
	if svcB.chain.Height() != 0 {
		t.Fatalf("B 初始高度应为 0，实际 %d", svcB.chain.Height())
	}

	nodeA, addrA := startService(t, svcA)
	defer nodeA.Stop()
	nodeB, _ := startService(t, svcB)
	defer nodeB.Stop()

	// B 连接 A → 握手中得知 A 高度更高 → 自动发起 GetBlocks → 追赶
	if err := nodeB.ConnectToPeer(addrA); err != nil {
		t.Fatalf("B 连接 A 失败: %v", err)
	}

	waitFor(t, func() bool { return svcB.chain.Height() == 3 }, 10*time.Second, "B 未完成追赶同步")

	for i, want := range hashes {
		got, err := svcB.chain.BlockByHeight(i + 1)
		if err != nil {
			t.Fatalf("B 高度 %d 取块失败: %v", i+1, err)
		}
		if got.Header.Hash() != want {
			t.Fatalf("B 高度 %d 的区块哈希与 A 不一致", i+1)
		}
	}
}

// TestNodeServiceRejectsInvalidBlock PoW 不合格的区块不得改变对端链高度。
func TestNodeServiceRejectsInvalidBlock(t *testing.T) {
	svcA := newServiceFor(t, testChain(t))
	svcB := newServiceFor(t, testChain(t))
	nodeA, addrA := startService(t, svcA)
	defer nodeA.Stop()
	nodeB, _ := startService(t, svcB)
	defer nodeB.Stop()

	if err := nodeB.ConnectToPeer(addrA); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return nodeA.PeerCount() == 1 && nodeB.PeerCount() == 1 },
		5*time.Second, "两节点未建立连接")

	// 构造一个父哈希正确但工作量证明必然不合格的区块。
	// 必须显式搜索出一个「验不过」的 Nonce：Nonce=0 有 1/65536 的概率恰好满足目标，
	// 直接依赖 Nonce=0 会让本用例以极低概率偶发通过（假绿）。
	tip, _ := svcB.chain.Tip()
	cb := transaction.NewCoinbaseTx(svcB.miner.PubKeyHash(), utxo.Subsidy(1), 1)
	bad := block.NewCandidateBlock(tip.Header.Hash(), svcB.chain.CurrentBits(),
		[]*transaction.Transaction{cb})
	for nonce := uint64(0); ; nonce++ {
		bad.Header.Nonce = nonce
		if !pow.Validate(&bad.Header) {
			break
		}
	}

	payload := blockPayloadOf(t, bad)
	nodeA.Broadcast(p2p.Message{Type: p2p.MsgNewBlock, Payload: payload})

	// 给足时间让消息到达并处理
	time.Sleep(500 * time.Millisecond)
	if svcB.chain.Height() != 0 {
		t.Fatalf("非法区块被接受，B 高度 = %d, want 0", svcB.chain.Height())
	}
}

// TestNodeServiceDuplicateBlockIdempotent 重复广播同一区块不产生副作用（链高度不变）。
func TestNodeServiceDuplicateBlockIdempotent(t *testing.T) {
	svcA := newServiceFor(t, testChain(t))
	svcB := newServiceFor(t, testChain(t))
	nodeA, addrA := startService(t, svcA)
	defer nodeA.Stop()
	nodeB, _ := startService(t, svcB)
	defer nodeB.Stop()

	if err := nodeB.ConnectToPeer(addrA); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return nodeA.PeerCount() == 1 && nodeB.PeerCount() == 1 },
		5*time.Second, "两节点未建立连接")

	mined := mineAndBroadcast(t, svcA)
	waitFor(t, func() bool { return svcB.chain.Height() == 1 }, 5*time.Second, "B 未接受首个区块")

	// 再次广播同一个区块：其父块已不是 B 的链尾 → 必须被忽略（幂等）
	nodeA.Broadcast(p2p.Message{Type: p2p.MsgNewBlock, Payload: blockPayloadOf(t, mined)})
	time.Sleep(300 * time.Millisecond)

	if got := svcB.chain.Height(); got != 1 {
		t.Fatalf("重复区块导致高度异常: %d, want 1", got)
	}
}
