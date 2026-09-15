package main

// 本文件是 REORG-1H（P2P 分支投递 / 分叉同步）的端到端验收测试。
//
// 与 service_test.go 的区别：那里用的是「纯内存链」的服务实例，detached 分叉块
// 无法被对端取到（内存链没有 store，detached 块不可寻址）；这里全部使用
// **真实 nodeRuntime**（真实 FileBlockStore + 真实控制接口 + 真实 TCP），
// 因此 detached 分叉块可持久化、可按哈希被对端取回——这正是分支拉取的前提。
//
// 覆盖点：
//   - M1 分叉块不再被丢弃：父已知的竞争块能进入 blocktree 并触发 reorg 决策
//   - M2/M3 按哈希取块 + 分支补齐：缺父孤儿能拉回祖先并跨节点 reorg
//   - M4 work-aware：同步判据是累积工作量而非高度
//   - M5 孤儿边界：请求去重、有界、不放大；孤儿不落盘
//   - M6 网络安全：非法块不中继、重复响应幂等

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/p2p"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
)

// ---- 脚手架 ----

// newBranchRuntime 在给定目录启动一个真实节点（随机端口，无种子）。
func newBranchRuntime(t *testing.T, dir string) *nodeRuntime {
	t.Helper()
	rt, err := newNodeRuntime(nodeConfig{
		ListenAddr: "127.0.0.1:0",
		RPCAddr:    "127.0.0.1:0",
		DataDir:    dir,
	})
	if err != nil {
		t.Fatalf("启动节点失败（%s）: %v", dir, err)
	}
	t.Cleanup(rt.Close)
	return rt
}

// cloneChainData 复制区块数据文件（不复制 node.lock / wallet.json）。
// 用于构造「两个节点共享同一段链前缀」的真实分叉前提。
func cloneChainData(t *testing.T, srcDir, dstDir string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(srcDir, "blocks.dat"))
	if err != nil {
		t.Fatalf("读取源链数据失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "blocks.dat"), data, 0o600); err != nil {
		t.Fatalf("写入目标链数据失败: %v", err)
	}
}

// forkPair 构造一对「共享 prefixHeight 高度链前缀、初始互不相连」的真实节点。
//
// 做法：先在 A 上挖出前缀 → 关闭 A → 复制链数据到 B → 分别以各自目录重启。
// 这是唯一能真实复现「同前缀而后分叉」的方式：两个独立创世节点即使哈希相同，
// coinbase 收款地址与时间戳也不同，从高度 1 起就是两条不同的链。
//
// 目录由本函数自建（t.TempDir()）。需要拿到数据目录路径的用例请用 forkPairIn。
func forkPair(t *testing.T, prefixHeight int) (a, b *nodeRuntime) {
	t.Helper()
	return forkPairIn(t, t.TempDir(), t.TempDir(), prefixHeight)
}

// forkPairIn 与 forkPair 行为完全一致，但由调用方提供两个数据目录，
// 以便用例直接读取 blocks.dat（如 append-only / 逐字节不可变性证明）。
func forkPairIn(t *testing.T, dirA, dirB string, prefixHeight int) (a, b *nodeRuntime) {
	t.Helper()
	seed := newBranchRuntime(t, dirA)
	for i := 0; i < prefixHeight; i++ {
		if got := mineOnce(seed.svc, nil); got != mineOutcomeMined {
			t.Fatalf("挖前缀第 %d 块失败: outcome=%s", i+1, got)
		}
	}
	if seed.chain.Height() != prefixHeight {
		t.Fatalf("前缀高度 = %d, want %d", seed.chain.Height(), prefixHeight)
	}
	seed.Close() // 释放数据目录锁，随后两节点各自持有自己的目录

	cloneChainData(t, dirA, dirB)

	a = newBranchRuntime(t, dirA)
	b = newBranchRuntime(t, dirB)

	if ha, hb := a.chain.Height(), b.chain.Height(); ha != prefixHeight || hb != prefixHeight {
		t.Fatalf("分叉对初始高度 = (%d, %d), want (%d, %d)", ha, hb, prefixHeight, prefixHeight)
	}
	ta, _ := a.chain.Tip()
	tb, _ := b.chain.Tip()
	if ta.Header.Hash() != tb.Header.Hash() {
		t.Fatal("分叉对未共享同一链尾：前提不成立")
	}
	return a, b
}

// blobOf 读取数据目录下 blocks.dat 的全部字节（用于 append-only / 尺寸证明）。
func blobOf(t *testing.T, dir string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "blocks.dat"))
	if err != nil {
		t.Fatalf("读取 blocks.dat 失败: %v", err)
	}
	return b
}

// legacyPrefixBytes 截取 legacy blocks.dat 中前 records 条记录的字节切片。
//
// legacy 记录格式：[u32 LE 长度][区块字节]，记录序号即高度（0 = 创世）。
// 此处只做**整记录前向切片**，不改写任何字节——用于证明 reorg 之后 legacy
// 前缀字节逐字节未变（即纯追加，未发生物理改写/truncate）。
func legacyPrefixBytes(t *testing.T, dir string, records int) []byte {
	t.Helper()
	data := blobOf(t, dir)
	out := make([]byte, 0, len(data))
	off := 0
	for i := 0; i < records; i++ {
		if off+4 > len(data) {
			t.Fatalf("blocks.dat 记录不足：需要 %d 条，实际在第 %d 条处截断", records, i)
		}
		l := int(binary.LittleEndian.Uint32(data[off : off+4]))
		if l <= 0 || off+4+l > len(data) {
			t.Fatalf("blocks.dat 第 %d 条记录长度非法: %d（剩余 %d 字节）", i, l, len(data)-off-4)
		}
		out = append(out, data[off:off+4+l]...)
		off += 4 + l
	}
	return out
}

// sealV2Mode 把节点的区块存储切换进 v2 语义（测试前置，不是被测行为）。
//
// ── 为什么需要它（GAP-1H-A 的测试侧绕行）──────────────────────────────
// 现状：FileBlockStore 在出现第一条 v2 记录之前，canonical 追加走的是 **legacy
// 路径**（blockchain.extendChain 判据是 `V2Mode()`）。legacy 记录「高度 = 记录
// 序号」，其隐含语义是不可变历史前缀：setCanonicalFrom 明确拒绝让 v2 区块占据
// 高度 < legacyLen 的位置（internal/storage/v2.go:644）。
//
// 后果：**任何落点对 legacy 前缀内部的 reorg 都会被存储层拒绝**
//（ErrCorruptStore: 高度 N 由 v2 区块占据）。而节点自启动以来写的第一个 canonical
// 块必然是 legacy 的，所以「现实持久化链上的首次 reorg」当前一定失败——
// 生产 .123 的 1284 块全是 legacy，影响面是真实且严重的。
//
// 本函数用一枚**累积工作量必定低于当前链尾**的短链分叉块把存储切进 v2：
// 该块只会被存为 detached（v2 记录），不会触发 reorg，之后本节点所有新的
// canonical 块都走 AppendCanonicalBlock（v2 记录，高度 >= legacyLen），
// reorg 才具备可行性。
//
// 这是**绕行不是修复**：GAP-1H-A 本身仍需独立的存储层阶段处理。
func sealV2Mode(t *testing.T, rt *nodeRuntime) {
	t.Helper()
	h := rt.chain.Height()
	if h < 2 {
		t.Fatalf("密封 v2 需要链高 >= 2，实际 %d", h)
	}
	parent := mustBlockAt(t, rt, h-2) // 以 h-2 为父 → 新块高度 h-1
	child := h - 1                    // 严格低于当前链尾 → 累积工作量必定更小
	cb := transaction.NewCoinbaseTx(rt.svc.miner.PubKeyHash(), utxo.Subsidy(child), child)
	candidate := block.NewCandidateBlock(parent.Header.Hash(), rt.chain.CurrentBits(),
		[]*transaction.Transaction{cb})
	// 时间戳刻意取「未来 1 小时」：一是保证本块与任何既有区块都不同（否则会与
	// 同高度、同父、同 coinbase 的 canonical 块**逐字节相同**而被判为重复投递，
	// 密封静默失败——这是本用例最初偶发失败的根因）；二是该偏移仍在共识允许的
	// maxFutureTimestampDrift（7200s）之内，不会触发时间戳校验拒绝。
	candidate.Header.Timestamp = time.Now().Unix() + 3600
	if found, _ := pow.Mine(candidate); !found {
		t.Fatal("密封块挖矿失败")
	}
	rt.svc.OnNewBlock("", blockPayloadOf(t, candidate))

	if !rt.store.V2Mode() {
		t.Fatal("密封失败：存储未进入 v2 语义（detached 分叉块未被写入）")
	}
	if got := rt.chain.Height(); got != h {
		t.Fatalf("密封块不应改变链高: %d → %d", h, got)
	}
}

// mineForkBlockAt 在指定高度的区块之上挖出一个**分叉块**（不广播），返回该块。
//
// 时间戳偏移 +3600s（仍在共识允许的 maxFutureTimestampDrift=7200s 内）：
// 保证本块与任何「同高度、同父、同 coinbase」的既有区块逐字节不同，
// 否则会被判为重复投递而静默失效。
func mineForkBlockAt(t *testing.T, rt *nodeRuntime, parentHeight int) *block.Block {
	t.Helper()
	parent := mustBlockAt(t, rt, parentHeight)
	child := parentHeight + 1
	cb := transaction.NewCoinbaseTx(rt.svc.miner.PubKeyHash(), utxo.Subsidy(child), child)
	candidate := block.NewCandidateBlock(parent.Header.Hash(), rt.chain.CurrentBits(),
		[]*transaction.Transaction{cb})
	candidate.Header.Timestamp = time.Now().Unix() + 3600
	if found, _ := pow.Mine(candidate); !found {
		t.Fatalf("以高度 %d 为父的分叉块挖矿失败", parentHeight)
	}
	return candidate
}

// mustBlockAt 取出指定高度的区块（失败即终止）。
func mustBlockAt(t *testing.T, rt *nodeRuntime, h int) *block.Block {
	t.Helper()
	b, err := rt.chain.BlockByHeight(h)
	if err != nil {
		t.Fatalf("取高度 %d 区块失败: %v", h, err)
	}
	return b
}

// tipHashOf 返回节点当前链尾哈希。
func tipHashOf(t *testing.T, rt *nodeRuntime) [32]byte {
	t.Helper()
	tip, err := rt.chain.Tip()
	if err != nil {
		t.Fatalf("读取链尾失败: %v", err)
	}
	return tip.Header.Hash()
}

// connectPair 双向建立连接并等待握手完成。
func connectPair(t *testing.T, from, to *nodeRuntime) {
	t.Helper()
	addr := to.p2p.ListenAddr()
	if err := from.p2p.ConnectToPeer(addr); err != nil {
		t.Fatalf("连接 %s 失败: %v", addr, err)
	}
	waitFor(t, func() bool {
		return from.p2p.PeerCount() == 1 && to.p2p.PeerCount() == 1
	}, 15*time.Second, "两节点未完成互联")
}

// waitSameTip 等待两个节点收敛到同一链尾（高度 + 哈希都相同）。
func waitSameTip(t *testing.T, a, b *nodeRuntime, timeout time.Duration, msg string) {
	t.Helper()
	waitFor(t, func() bool {
		if a.chain.Height() != b.chain.Height() {
			return false
		}
		return tipHashOf(t, a) == tipHashOf(t, b)
	}, timeout, msg)
}

// ---- M1：父已知的分叉块不再被丢弃，且收敛结果与到达顺序无关 ----

// TestBranchForkBlockWithKnownParentConverges 两个节点各自在同一父块上挖出竞争块
// 并互相广播：双方必须收敛到同一链尾，且胜者由**确定性 tie-break（tip 哈希大端较大者）**
// 决定，而不是由网络到达顺序决定。
func TestBranchForkBlockWithKnownParentConverges(t *testing.T) {
	a, b := forkPair(t, 3)
	sealV2Mode(t, a) // GAP-1H-A 绕行，见 sealV2Mode 注释
	sealV2Mode(t, b)
	connectPair(t, a, b)

	// 各自在自己的链尾（同一父块 T3）上挖出竞争块，暂不广播。
	a4 := mineOnly(t, a.svc)
	b4 := mineOnly(t, b.svc)
	if a4.Header.PrevBlockHash != b4.Header.PrevBlockHash {
		t.Fatal("前提不成立：两个竞争块父块不同")
	}
	if a4.Header.Hash() == b4.Header.Hash() {
		t.Fatal("前提不成立：两个竞争块哈希相同（未构成分叉）")
	}

	// 同时广播：REORG-1H 之前，两个块都会因「父块不是当前链尾」被就地丢弃。
	a.svc.broadcastBlock(a4)
	b.svc.broadcastBlock(b4)

	waitSameTip(t, a, b, 20*time.Second, "分叉后两节点未收敛到同一链尾")

	// 胜者必须是确定性的：tip 哈希大端较大者（与谁先到达无关）。
	wantA, wantB := a4.Header.Hash(), b4.Header.Hash()
	want := wantA
	if bytes.Compare(wantB[:], wantA[:]) > 0 {
		want = wantB
	}
	for name, rt := range map[string]*nodeRuntime{"A": a, "B": b} {
		got := tipHashOf(t, rt)
		if got != want {
			t.Fatalf("%s 链尾 = %x, want %x（tie-break 必须是确定性的）", name, got[:4], want[:4])
		}
	}
	if a.chain.Height() != 4 || b.chain.Height() != 4 {
		t.Fatalf("收敛后高度 = (%d, %d), want (4, 4)", a.chain.Height(), b.chain.Height())
	}
}

// ---- M2/M3：缺父孤儿 → 按哈希拉回祖先 → 跨节点 reorg ----

// TestBranchMissingAncestorFetchTriggersReorg B 在共享前缀上多挖两块，A 只挖一块，
// 随后互联：A 收到的同步块缺父，必须按哈希把缺口补齐，并最终 reorg 到 B 的链。
//
// 这是 REORG-1H 的核心证据：**跨节点 reorg 在真实网络上真实发生**。
func TestBranchMissingAncestorFetchTriggersReorg(t *testing.T) {
	a, b := forkPair(t, 3)
	sealV2Mode(t, a) // GAP-1H-A 绕行
	sealV2Mode(t, b)

	// 未互联状态下各自延长：B 领先两块，A 只有一块。
	for i := 0; i < 2; i++ {
		if got := mineOnce(b.svc, nil); got != mineOutcomeMined {
			t.Fatalf("B 挖第 %d 块失败: outcome=%s", i+1, got)
		}
	}
	a4 := mineOnly(t, a.svc)
	if a.chain.Height() != 4 || b.chain.Height() != 5 {
		t.Fatalf("分叉前提高度 = (%d, %d), want (4, 5)", a.chain.Height(), b.chain.Height())
	}

	connectPair(t, a, b)
	waitSameTip(t, a, b, 30*time.Second, "A 未通过分支拉取收敛到 B 的链")

	if a.chain.Height() != 5 {
		t.Fatalf("A 收敛后高度 = %d, want 5", a.chain.Height())
	}
	// A 原本在高度 4 上的块必须已被 reorg 掉（高度 4 现在是 B 的块）。
	if got := mustBlockAt(t, a, 4).Header.Hash(); got == a4.Header.Hash() {
		t.Fatal("A 未发生 reorg：高度 4 仍是 A 自己挖的块")
	}
	if got := mustBlockAt(t, a, 4).Header.Hash(); got != mustBlockAt(t, b, 4).Header.Hash() {
		t.Fatal("A 与 B 在高度 4 上的区块不一致")
	}
	if got := a.svc.branchReqSent.Load(); got < 1 {
		t.Fatalf("分支拉取未被触发：by-hash 请求数 = %d, want >= 1（若此处为 0，说明走的是纯高度同步，本用例失去意义）", got)
	}
	if got := a.svc.branchApplied.Load(); got < 1 {
		t.Fatalf("分支拉取没有真正上链任何区块：branchApplied = %d, want >= 1", got)
	}
}

// ---- M5：孤儿请求去重、有界、不放大 ----

// TestBranchOrphanRequestIsBoundedAndNotAmplified 一个父块不存在的孤儿块
// 必须只引发**一次** by-hash 请求；对端回「没有」后不得无限重试。
func TestBranchOrphanRequestIsBoundedAndNotAmplified(t *testing.T) {
	a, b := forkPair(t, 2)
	connectPair(t, a, b)

	orphan := mineOrphanBlock(t, a)
	payload := blockPayloadOf(t, orphan)
	peer := b.p2p.ListenAddr()

	a.svc.OnNewBlock(peer, payload)

	// 等待「一次请求 + 一次响应」落地
	waitFor(t, func() bool { return a.svc.branchRespRecv.Load() >= 1 },
		10*time.Second, "未收到 by-hash 响应")

	// 再观察一段时间：请求数不得继续增长（无重试风暴、无循环请求）
	time.Sleep(1500 * time.Millisecond)
	if got := a.svc.branchReqSent.Load(); got != 1 {
		t.Fatalf("by-hash 请求数 = %d, want 1（一个孤儿只应触发一次请求，不得放大）", got)
	}
	if a.chain.Height() != 2 {
		t.Fatalf("孤儿块不得改变链高度: %d, want 2", a.chain.Height())
	}
}

// mineOrphanBlock 挖一个 PoW 有效但父块为随机未知哈希的孤儿块。
// 它被共识层判定为 ErrOrphanParent（父不在树中），从而触发分支拉取。
func mineOrphanBlock(t *testing.T, rt *nodeRuntime) *block.Block {
	t.Helper()
	var unknownParent [32]byte
	for i := range unknownParent {
		unknownParent[i] = byte(0xA5 ^ i)
	}
	h := rt.chain.Height() + 1
	cb := transaction.NewCoinbaseTx(rt.svc.miner.PubKeyHash(), utxo.Subsidy(h), h)
	candidate := block.NewCandidateBlock(unknownParent, rt.chain.CurrentBits(),
		[]*transaction.Transaction{cb})
	if found, _ := pow.Mine(candidate); !found {
		t.Fatal("孤儿块挖矿失败")
	}
	return candidate
}

// ---- M6：重复分支响应必须幂等 ----

// TestBranchDuplicateResponseIsIdempotent 同一条分支响应被重复投递两次，
// 第二次不得产生任何副作用（高度与链尾不变，不报错）。
func TestBranchDuplicateResponseIsIdempotent(t *testing.T) {
	a, b := forkPair(t, 2)
	sealV2Mode(t, a) // GAP-1H-A 绕行
	sealV2Mode(t, b)

	// A 先自己延长一块，使随后送达的 B 分支构成**真正的 reorg**（而非单纯追赶）。
	a3 := mineOnly(t, a.svc)
	// B 在未互联状态下延长两块，构造一条 A 完全不知道的分支。
	for i := 0; i < 2; i++ {
		if got := mineOnce(b.svc, nil); got != mineOutcomeMined {
			t.Fatalf("B 挖第 %d 块失败: outcome=%s", i+1, got)
		}
	}
	connectPair(t, a, b)

	bTip := tipHashOf(t, b)
	branch := b.chain.BlockByHashWithAncestors(bTip, 64)
	if len(branch) == 0 {
		t.Fatal("B 无法按哈希取回自己的链尾：BlockByHashWithAncestors 失效")
	}
	encoded := make([]string, 0, len(branch))
	for _, blk := range branch {
		encoded = append(encoded, hex.EncodeToString(blk.Encode()))
	}
	resp := p2p.BlockByHashRespPayload{Hash: hex.EncodeToString(bTip[:]), Blocks: encoded, Found: true}
	peer := b.p2p.ListenAddr()

	a.svc.OnBlockByHashResp(peer, resp)
	waitFor(t, func() bool { return tipHashOf(t, a) == bTip }, 15*time.Second, "A 未应用分支")

	// 必须是 reorg（A 自己的 a3 被换掉），而不是简单地把分支接在后面。
	if mustBlockAt(t, a, 3).Header.Hash() == a3.Header.Hash() {
		t.Fatal("A 未发生 reorg：高度 3 仍是 A 自己挖的块")
	}

	h1, tip1 := a.chain.Height(), tipHashOf(t, a)

	// 重复投递同一条响应
	a.svc.OnBlockByHashResp(peer, resp)
	time.Sleep(500 * time.Millisecond)

	if h2, tip2 := a.chain.Height(), tipHashOf(t, a); h2 != h1 || tip2 != tip1 {
		t.Fatalf("重复分支响应产生了副作用: 高度 %d→%d, 链尾 %x→%x",
			h1, h2, tip1[:4], tip2[:4])
	}
}

// ---- M4：work-aware 同步判据（纯函数）----

func TestShouldSyncFromPrefersWorkOverHeight(t *testing.T) {
	localWork := big.NewInt(1000)

	cases := []struct {
		name       string
		peerWork   string
		peerHeight int
		localH     int
		want       bool
	}{
		// 工作量已知：只比工作量，高度再高也不追（M4 的实质）
		{"对端更高但工作量更低 → 不追", "900", 99, 10, false},
		{"对端更矮但工作量更高 → 追", "1100", 3, 10, true},
		{"工作量相同 → 不追", "1000", 99, 10, false},
		// 工作量未知（旧版本节点不填 chain_work）→ 退化为按高度
		{"工作量未知且对端更高 → 追（兼容旧行为）", "", 11, 10, true},
		{"工作量未知且对端不高 → 不追", "", 9, 10, false},
		{"工作量非法 → 退化为按高度", "not-a-number", 11, 10, true},
	}
	for _, c := range cases {
		if got := shouldSyncFrom(c.peerWork, c.peerHeight, localWork, c.localH); got != c.want {
			t.Fatalf("%s: shouldSyncFrom = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestWorkAtLeastGate(t *testing.T) {
	localWork := big.NewInt(1000)

	cases := []struct {
		name       string
		peerWork   string
		peerHeight int
		localH     int
		want       bool
	}{
		{"工作量更高 → 拉分支", "1100", 3, 10, true},
		{"工作量相等 → 拉分支（可能同高度分叉）", "1000", 10, 10, true},
		{"工作量更低 → 不拉", "900", 20, 10, false},
		{"工作量未知且高度不低于 → 拉", "", 10, 10, true},
		{"工作量未知且高度更低 → 不拉", "", 9, 10, false},
	}
	for _, c := range cases {
		if got := workAtLeast(c.peerWork, c.peerHeight, localWork, c.localH); got != c.want {
			t.Fatalf("%s: workAtLeast = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestBranchLowerWorkForkIsAcceptedButNotCanonical （M8 work comparison / 验收 G）
//
// 低工作量分叉块必须：
//   1. 进入接收端 BlockTree（不再被丢弃，M1）；
//   2. **不得**成为 canonical（M7：detached ≠ canonicality）；
//   3. 可按哈希被取回（M2 可取回性，供后续分支补齐使用）。
func TestBranchLowerWorkForkIsAcceptedButNotCanonical(t *testing.T) {
	a, b := forkPair(t, 3)
	sealV2Mode(t, a) // GAP-1H-A 绕行，见 sealV2Mode 注释
	sealV2Mode(t, b)
	connectPair(t, a, b)

	before := tipHashOf(t, b)
	// 以高度 1 为父 ⇒ 新块高度 2，累积工作量严格低于链尾（高度 3）
	low := mineForkBlockAt(t, a, 1)
	lowHash := low.Header.Hash()

	a.svc.broadcastBlock(low)

	waitFor(t, func() bool { return b.chain.HasBlockHash(lowHash) },
		20*time.Second, "低工作量分叉块未进入对端 BlockTree（仍被丢弃？）")

	// (2) 不得触发 reorg
	if got := b.chain.Height(); got != 3 {
		t.Fatalf("低工作量分叉不得改变链高: %d, want 3", got)
	}
	if got := tipHashOf(t, b); got != before {
		t.Fatalf("低工作量分叉不得改变链尾: %x, want %x", got[:4], before[:4])
	}
	if b.chain.IsCanonicalHash(lowHash) {
		t.Fatal("detached 分叉块不得成为 canonical（persistence ≠ canonicality）")
	}
	// (3) 可按哈希取回
	got, ok := b.chain.BlockByHash(lowHash)
	if !ok {
		t.Fatal("分叉块应按哈希可取回（M2），实际取不到")
	}
	if got.Header.Hash() != lowHash {
		t.Fatalf("按哈希取回的区块不匹配: %x", got.Header.Hash())
	}
	// 且区块高度索引未被 detached 块污染
	if blk, err := b.chain.BlockByHeight(2); err != nil {
		t.Fatalf("读取高度 2 失败: %v", err)
	} else if blk.Header.Hash() == lowHash {
		t.Fatal("detached 分叉块污染了 canonical 高度索引")
	}
}

// TestBranchDetachedForkSurvivesRestart （M8 restart）
//
// detach 分叉块写入后重启节点：该块必须仍然可恢复、可按哈希取回，
// 且重启不得改变 canonical 链。
//
// 若持久化架构对 detached 恢复有限制，本用例会**明确失败**并把限制暴露出来
// （按 1H 规范要求：不得伪造 PASS）。
func TestBranchDetachedForkSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	rt := newBranchRuntime(t, dir)
	for i := 0; i < 3; i++ {
		mineOnly(t, rt.svc)
	}
	if rt.chain.Height() != 3 {
		t.Fatalf("前置链高 = %d, want 3", rt.chain.Height())
	}
	sealV2Mode(t, rt) // GAP-1H-A 绕行，见 sealV2Mode 注释

	fork := mineForkBlockAt(t, rt, 1) // 高度 2 的低工作量分叉块 ⇒ 只会被存为 detached
	forkHash := fork.Header.Hash()
	rt.svc.OnNewBlock("", blockPayloadOf(t, fork))
	if !rt.chain.HasBlockHash(forkHash) {
		t.Fatal("前提不成立：重启前分叉块未进入 BlockTree")
	}
	tipBefore := tipHashOf(t, rt)

	// ── 重启（真实关闭 + 真实重开同一数据目录）──
	rt.Close()
	rt2 := newBranchRuntime(t, dir)

	if got := rt2.chain.Height(); got != 3 {
		t.Fatalf("重启后链高 = %d, want 3", got)
	}
	if got := tipHashOf(t, rt2); got != tipBefore {
		t.Fatalf("重启后链尾 = %x, want %x", got[:4], tipBefore[:4])
	}
	if rt2.chain.IsCanonicalHash(forkHash) {
		t.Fatal("重启后 detached 分叉块不得变成 canonical")
	}
	if !rt2.chain.HasBlockHash(forkHash) {
		t.Fatal("重启后 detached 分叉块不可恢复：persistence 对 detached recovery 存在限制（须登记，不得伪造 PASS）")
	}
	if blk, ok := rt2.chain.BlockByHash(forkHash); !ok || blk.Header.Hash() != forkHash {
		t.Fatal("重启后分叉块无法按哈希取回（后续分支补齐将失效）")
	}
}

// 注：新增消息类型在真实 TCP 上的往返验证放在 internal/p2p/node_test.go
//（TestGetBlockByHashTravelsOverTCP）——那是协议层自身的测试，且能复用
// 该包已有的 recordHandler 记录型 Handler。

// ── legacy 前缀 canonical reorg 正向验收（R1）────────────────────────────
//
// ════════════════════════════════════════════════════════════════════════════
// REORG-1J-G09 · R1 —— POSITIVE REORG ACCEPTANCE TRIPWIRE CONVERSION
//
// ── 旧契约（已废止）──────────────────────────────────────────────────────
// 旧用例名 TestBranchKnownGapLegacyPrefixBlocksFirstReorg，断言的是缺陷行为：
//
//	legacy 语义下写入的 canonical 块构成不可变历史前缀，
//	FileBlockStore.setCanonicalFrom 拒绝让 v2 区块占据高度 < legacyLen 的槽位
//	⇒ 现实持久化链上的首次 reorg 在存储层被拒绝。
//
// 旧断言链：B 高度必须恒为 4、链尾必须恒为 b4、日志必须出现「legacy 前缀不可变」。
//
// ── 旧断言为何已废止 ─────────────────────────────────────────────────────
// REORG-1J §1/§6/§7/§11 明确废止「legacy canonical 归属永久不可变化」：
// canonical ownership 由 **TIP + hash linkage** 决定，与记录物理形态（legacy/v2）
// 解耦；穿越 legacy prefix 的 reorg **必须被允许**。
// 「legacy 前缀不可变」这一拒绝路径已被移除（生产代码中该字符串已不存在）。
// 因此旧断言把**已废止的行为**当作正确行为来固化，属于过时契约。
//
// ── 新契约（正向 acceptance test）────────────────────────────────────────
//
//	当 legacy prefix 中存在旧 canonical 块，而更长/更优的 V2 branch 通过
//	hash linkage 成为合法候选时，reorg 必须成功，并最终使 active TIP /
//	canonical height 正确收敛；同时 legacy 的物理不可变性仍必须成立。
//
// ── 被替换的更强不变量（acceptance chain，1–10）─────────────────────────
// 本用例不满足于「old error → new nil」，而是建立完整验收链：
//
//  1. legacy block 仍然存在（HasBlockHash）
//  2. legacy block physical bytes 未变化（日志前缀逐字节不变）
//  3. branch block 通过 hash linkage 被识别（BlockByHash 可取回）
//  4. reorg 落点穿越 legacy prefix 不再因 legacy ownership 被拒绝
//  5. active TIP 更新到正确 branch（== a5）
//  6. canonical height 正确（== 5）
//  7. detached old branch 不再被视为 canonical（IsCanonicalHash(b4)==false）
//  8. legacy block 仍可通过 hash 查询
//  9. legacy block 内容与 reorg 前完全一致（Encode() 逐字节相等）
//  10. append-only 存储未发生非法 truncate（文件长度只增不减）
//
// ════════════════════════════════════════════════════════════════════════════
func TestBranchLegacyPrefixReorgConvergesToLongerBranch(t *testing.T) {
	dirA, dirB := t.TempDir(), t.TempDir()
	a, b := forkPairIn(t, dirA, dirB, 3)
	// 刻意**不做** v2 密封：完全复刻「现实持久化链的首次 reorg」场景。
	if a.store.V2Mode() || b.store.V2Mode() {
		t.Fatal("前提不成立：本用例要求存储仍处于 legacy 语义")
	}

	// A 挖两块（工作量必然更大），B 挖一块。
	a4 := mineOnly(t, a.svc)
	a5 := mineOnly(t, a.svc)
	b4 := mineOnly(t, b.svc)
	if b.chain.Height() != 4 || a.chain.Height() != 5 {
		t.Fatalf("前提高度 = (A %d, B %d), want (5, 4)", a.chain.Height(), b.chain.Height())
	}
	a5Hash, a4Hash, b4Hash := a5.Header.Hash(), a4.Header.Hash(), b4.Header.Hash()

	// ── 变更前基准：legacy 物理字节 + 高度 4 的 canonical 归属 ──────────────
	// B 的高度 3 是 legacy 记录（共享前缀），高度 4 的 b4 也是 legacy 写入。
	// reorg 落点（公共祖先 = 高度 3）位于 legacy 前缀边界，正是本用例要穿越的点。
	b4Before, ok := b.chain.BlockByHash(b4Hash)
	if !ok {
		t.Fatal("前提不成立：B 的 b4 在 reorg 前不可按哈希取回")
	}
	b4BytesBefore := b4Before.Encode()
	if b.chain.IsCanonicalHash(a5Hash) {
		t.Fatal("前提不成立：a5 在 reorg 前不应已是 canonical")
	}
	legacyPrefixHashes := make([][32]byte, 0, 4)
	for h := 0; h <= 3; h++ {
		legacyPrefixHashes = append(legacyPrefixHashes, mustBlockAt(t, b, h).Header.Hash())
	}
	sizeBefore := len(blobOf(t, dirB))
	// legacy 前缀（高度 0..3）在 reorg 前的逐字节快照，用于证明「未被改写」
	legacyBytesBefore := legacyPrefixBytes(t, dirB, 4)

	// ── 注入 A 的更长分支（跳过网络，聚焦存储/共识行为）─────────────────────
	// Blocks[0] = a5（最新），Blocks[1] = a4（其父，父 T3 为 B 已知 → 挂载点）。
	encoded := make([]string, 0, 2)
	for _, blk := range []*block.Block{a5, a4} {
		encoded = append(encoded, hex.EncodeToString(blk.Encode()))
	}
	resp := p2p.BlockByHashRespPayload{
		Hash:   hex.EncodeToString(a5Hash[:]),
		Blocks: encoded,
		Found:  true,
	}

	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)
	b.svc.OnBlockByHashResp("", resp)
	waitFor(t, func() bool { return tipHashOf(t, b) == a5Hash },
		20*time.Second, "REORG-1J：B 未收敛到 A 的更长分支（legacy-prefix reorg 被拒绝？）")
	log.SetOutput(os.Stderr)
	captured := logs.String()

	// 性质 5：active TIP 必须更新到正确 branch。
	if got := tipHashOf(t, b); got != a5Hash {
		t.Fatalf("性质 5 失败：active TIP = %x, want a5 %x", got[:4], a5Hash[:4])
	}
	// 性质 6：canonical height 正确。
	if got := b.chain.Height(); got != 5 {
		t.Fatalf("性质 6 失败：canonical height = %d, want 5", got)
	}

	// 性质 4：reorg 必须真实穿越 legacy prefix —— 高度 4 的 canonical 归属
	// 已从 B 的 legacy 块 b4 切换为 A 的 a4，且未被任何 legacy 守卫拒绝。
	if got := mustBlockAt(t, b, 4).Header.Hash(); got != a4Hash {
		t.Fatalf("性质 4 失败：高度 4 未被 a4 取代（got %x）——legacy-prefix reorg 未生效", got[:4])
	}
	// 反向证据：日志中不得出现任何 legacy 前缀拒绝痕迹。
	if strings.Contains(captured, "legacy 前缀不可变") {
		t.Fatalf("性质 4 失败：reorg 仍被 legacy 前缀守卫拒绝，捕获日志:\n%s", captured)
	}

	// 性质 3：branch block 经 hash linkage 被识别且可按哈希取回。
	for _, h := range []struct {
		name string
		hash [32]byte
	}{{"a5", a5Hash}, {"a4", a4Hash}} {
		blk, ok := b.chain.BlockByHash(h.hash)
		if !ok {
			t.Fatalf("性质 3 失败：分支块 %s 未被识别（hash linkage 未建立）", h.name)
		}
		if blk.Header.Hash() != h.hash {
			t.Fatalf("性质 3 失败：按哈希取回的 %s 不匹配", h.name)
		}
	}

	// 性质 7：detached old branch 不再被视为 canonical，但**仍可按哈希取回**。
	if b.chain.IsCanonicalHash(b4Hash) {
		t.Fatal("性质 7 失败：被 reorg 换下的 b4 仍被标记 canonical")
	}
	// 性质 1 + 8：legacy block 仍然存在，且仍可通过 hash 查询。
	if !b.chain.HasBlockHash(b4Hash) {
		t.Fatal("性质 1/8 失败：被 reorg 换下的 legacy 块 b4 不再存在（应保留为 detached）")
	}
	b4After, ok := b.chain.BlockByHash(b4Hash)
	if !ok {
		t.Fatal("性质 8 失败：legacy 块 b4 在 reorg 后无法按哈希查询")
	}
	// 性质 9：legacy block 内容与 reorg 前完全一致（逐字节）。
	if !bytes.Equal(b4After.Encode(), b4BytesBefore) {
		t.Fatal("性质 9 失败：legacy 块 b4 的内容在 reorg 后被改写")
	}
	// 共享 legacy 前缀（高度 0..3）的全部 canonical 哈希必须原样保留。
	for h, want := range legacyPrefixHashes {
		if got := mustBlockAt(t, b, h).Header.Hash(); got != want {
			t.Fatalf("性质 9 失败：legacy 前缀高度 %d 的 canonical 哈希被改写（got %x, want %x）",
				h, got[:4], want[:4])
		}
	}

	// 性质 2 + 10：append-only —— 日志前缀逐字节不变，且总长只增不减。
	legacyBytesAfter := legacyPrefixBytes(t, dirB, 4)
	if !bytes.Equal(legacyBytesAfter, legacyBytesBefore) {
		t.Fatal("性质 2 失败：reorg 后 legacy 前缀字节被改写（非纯追加）")
	}
	sizeAfter := len(blobOf(t, dirB))
	if sizeAfter < sizeBefore {
		t.Fatalf("性质 10 失败：日志被截短（%d < %d）——发生非法 truncate", sizeAfter, sizeBefore)
	}
	if sizeAfter == sizeBefore {
		t.Fatal("性质 10 失败：reorg 未追加任何字节（TIP 提交必须留下记录）")
	}
}
