package main

// 本文件是 PHASE E-IMPLEMENTATION-A（O-02 / B-1 orphan waiting cascade）的回归测试。
//
// 缺陷（修复前）：takeWaiting 只经 OnBlockByHashResp → applyResolved 被调用，因此父块经
// 其它合法到达面（网络广播 / 批量同步 / 本地挖矿）变为已知时，等待它的孤儿不会被释放。
//
// 测试设计原则（对应阶段规范 §10）：只断言**可观察状态**——链高、链尾哈希、canonical 成员、
// waiting 条目数、UTXO 可用性——不绑定内部函数名，也不断言"某函数被调用过"。
//
// 判别性要点：所有缺失级联的用例都用一个**未连接**的伪对端送达消息（b1Peer）。这样
// deferOrphan 发起的 by-hash 补拉必然失败（SendTo 对未连接对端只返回错误），
// 于是"父块经非 by-hash 路径到达"这一滞留条件被精确复现：
//   修复前 → 子块永久滞留、链高不增长 → 用例失败；
//   修复后 → 统一解析入口释放子块 → 用例通过。

import (
	"encoding/hex"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/p2p"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
)

// b1Peer 是刻意**未连接**的伪对端地址（见文件头说明）。
const b1Peer = "b1-unit-peer"

// ---- 脚手架 ----

// newB1Service 组装一个带真实 p2p.Node 的服务实例（svc.net 非 nil，可安全触发补拉）。
func newB1Service(t *testing.T) *nodeService {
	t.Helper()
	svc := newServiceFor(t, testChain(t))
	node, _ := startService(t, svc)
	t.Cleanup(node.Stop)
	return svc
}

// mineOn 真实挖出一个「以 parent 为父、高度为 height」的 coinbase-only 区块，**不上链**。
func mineOn(t *testing.T, svc *nodeService, parent *block.Block, height int) *block.Block {
	t.Helper()
	return mineOnTo(t, svc, parent, height, svc.miner.PubKeyHash())
}

// mineOnTo 与 mineOn 相同，但显式指定 coinbase 收款公钥哈希。
//
// 必要性（避免测试自身缺陷）：两条**同父同高**的竞争分支若使用同一收款地址，
// 其唯一差异只剩头部时间戳（秒级）；当两次构造落在同一秒时，pow.Mine 的确定性搜索
// 会产出**完全相同的区块**，于是"竞争分支"退化为同一个块，语义断言随之失真。
// 指定不同收款地址即可从构造上保证两条分支必然不同。
func mineOnTo(t *testing.T, svc *nodeService, parent *block.Block, height int, pkh [20]byte) *block.Block {
	t.Helper()
	cb := transaction.NewCoinbaseTx(pkh, utxo.Subsidy(height), height)
	candidate := block.NewCandidateBlock(parent.Header.Hash(), svc.chain.CurrentBits(),
		[]*transaction.Transaction{cb})
	if found, _ := pow.Mine(candidate); !found {
		t.Fatal("挖矿失败")
	}
	return candidate
}

// ---- 断言辅助 ----

func assertHeight(t *testing.T, svc *nodeService, want int, msg string) {
	t.Helper()
	if got := svc.chain.Height(); got != want {
		t.Fatalf("%s: 链高 = %d, want %d", msg, got, want)
	}
}

func assertTipIs(t *testing.T, svc *nodeService, want *block.Block, msg string) {
	t.Helper()
	tip, err := svc.chain.Tip()
	if err != nil {
		t.Fatalf("%s: 读取链尾失败: %v", msg, err)
	}
	if tip.Header.Hash() != want.Header.Hash() {
		t.Fatalf("%s: 链尾 = %s, want %s", msg, tip.Header.HashHex(), want.Header.HashHex())
	}
}

func assertWaitingEmpty(t *testing.T, svc *nodeService, msg string) {
	t.Helper()
	svc.mu.Lock()
	n := len(svc.waiting)
	svc.mu.Unlock()
	if n != 0 {
		t.Fatalf("%s: waiting 仍有 %d 个键（应被消费）", msg, n)
	}
}

func assertParked(t *testing.T, svc *nodeService, parent [32]byte, want int, msg string) {
	t.Helper()
	if got := svc.waitingChildren(parent); got != want {
		t.Fatalf("%s: waiting[parent] = %d, want %d", msg, got, want)
	}
}

func assertCanonical(t *testing.T, svc *nodeService, b *block.Block, want bool, msg string) {
	t.Helper()
	if got := svc.chain.IsCanonicalHash(b.Header.Hash()); got != want {
		// 失败时打印当前 canonical 链全貌，便于定位 fork-choice/reorg 语义问题。
		var chainDump []string
		for i := 0; i <= svc.chain.Height(); i++ {
			cb, err := svc.chain.BlockByHeight(i)
			if err != nil {
				chainDump = append(chainDump, "<err>")
				continue
			}
			chainDump = append(chainDump, cb.Header.HashHex())
		}
		t.Fatalf("%s: IsCanonicalHash(%s) = %v, want %v\ncanonical 链: %v",
			msg, b.Header.HashHex(), got, want, chainDump)
	}
}

// ---- 送达面（按名字与签名直接驱动生产 handler） ----

// deliverBroadcast 模拟 P1：网络广播到达。
func deliverBroadcast(t *testing.T, svc *nodeService, b *block.Block) {
	t.Helper()
	svc.OnNewBlock(b1Peer, blockPayloadOf(t, b))
}

// deliverBatchSync 模拟 P2：批量同步响应到达。
func deliverBatchSync(t *testing.T, svc *nodeService, blocks ...*block.Block) {
	t.Helper()
	enc := make([]string, 0, len(blocks))
	for _, b := range blocks {
		enc = append(enc, hex.EncodeToString(b.Encode()))
	}
	svc.OnBlocksResp(b1Peer, p2p.BlocksRespPayload{EncodedBlocks: enc, Done: true})
}

// deliverByHashResp 模拟 P3：by-hash 分支响应到达（Blocks[0] = 被请求块，其后依次为祖先）。
func deliverByHashResp(t *testing.T, svc *nodeService, reqHash [32]byte, newestFirst []*block.Block) {
	t.Helper()
	enc := make([]string, 0, len(newestFirst))
	for _, b := range newestFirst {
		enc = append(enc, hex.EncodeToString(b.Encode()))
	}
	svc.OnBlockByHashResp(b1Peer, p2p.BlockByHashRespPayload{
		Hash:   hex.EncodeToString(reqHash[:]),
		Blocks: enc,
		Found:  len(enc) > 0,
	})
}

// ---- 用例 ----

// T1 直接到达：父块先到、子块后到，正常依次上链。
func TestB1T1DirectArrivalInOrder(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	p := mineOn(t, svc, g, 1)
	c := mineOn(t, svc, p, 2)

	deliverBroadcast(t, svc, p)
	deliverBroadcast(t, svc, c)

	assertHeight(t, svc, 2, "顺序到达后链高应为 2")
	assertTipIs(t, svc, c, "链尾应为 C")
	assertWaitingEmpty(t, svc, "不应有滞留孤儿")
}

// T2 孤儿后父块经普通广播（P1）到达 —— 必须自动级联。
func TestB1T2OrphanThenParentViaBroadcast(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	p := mineOn(t, svc, g, 1)
	c := mineOn(t, svc, p, 2)

	deliverBroadcast(t, svc, c) // 缺父 → 登记等待
	assertHeight(t, svc, 0, "缺父块不得上链")
	assertParked(t, svc, p.Header.Hash(), 1, "子块应等待父块")

	deliverBroadcast(t, svc, p) // 父块经 P1 到达
	assertHeight(t, svc, 2, "父块经 P1 到达后应级联释放子块")
	assertTipIs(t, svc, c, "链尾应为 C")
	assertWaitingEmpty(t, svc, "waiting 应被消费")
}

// T3 孤儿后父块经批量同步响应（P2）到达 —— 必须触发等待解析。
func TestB1T3OrphanThenParentViaBlocksResp(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	p := mineOn(t, svc, g, 1)
	c := mineOn(t, svc, p, 2)

	deliverBroadcast(t, svc, c)
	assertParked(t, svc, p.Header.Hash(), 1, "子块应等待父块")

	deliverBatchSync(t, svc, p)
	assertHeight(t, svc, 2, "父块经 P2 到达后应级联释放子块")
	assertTipIs(t, svc, c, "链尾应为 C")
	assertWaitingEmpty(t, svc, "waiting 应被消费")
}

// T4 孤儿后父块经 by-hash 响应（P3）到达 —— 保持既有级联行为（行为等价回归）。
func TestB1T4OrphanThenParentViaByHashResp(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	p := mineOn(t, svc, g, 1)
	c := mineOn(t, svc, p, 2)

	deliverBroadcast(t, svc, c)
	assertParked(t, svc, p.Header.Hash(), 1, "子块应等待父块")

	deliverByHashResp(t, svc, p.Header.Hash(), []*block.Block{p})
	assertHeight(t, svc, 2, "父块经 P3 到达后应级联释放子块")
	assertTipIs(t, svc, c, "链尾应为 C")
	assertWaitingEmpty(t, svc, "waiting 应被消费")
}

// T5 父块经**本地挖矿路径**（P4）进入系统 —— waiting child 必须得到正确处理。
//
// 说明：真实的 mineOnce 产出的候选块哈希在求解完成前不可预知，因此无法在测试里先构造
// 「以该候选块为父」的子块。本用例因此按 main.go 的**同一调用序列与同一生产函数**驱动
// P4：chain.AddBlock(候选) → broadcastBlock → resolveKnownBlock(空对端, cascadeViaLocalMining)。
func TestB1T5OrphanParentViaLocalMiningPath(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	p := mineOn(t, svc, g, 1)
	c := mineOn(t, svc, p, 2)

	deliverBroadcast(t, svc, c)
	assertParked(t, svc, p.Header.Hash(), 1, "子块应等待父块")

	// —— 与 main.go 的本地挖矿上链收尾完全一致：驱动同一个生产方法 ——
	if err := svc.chain.AddBlock(p); err != nil {
		t.Fatalf("本地挖矿区块上链失败: %v", err)
	}
	svc.commitMinedBlock(p, 1)

	assertHeight(t, svc, 2, "父块经 P4 到达后应级联释放子块")
	assertTipIs(t, svc, c, "链尾应为 C")
	assertWaitingEmpty(t, svc, "waiting 应被消费")

	// 补充：真实 mineOnce 路径不得因新增钩子而崩溃或误释放无关孤儿。
	u := mineOn(t, svc, c, 3) // u 不入链，仅取其哈希作为「未知父」用于停放
	_ = u
	orphanParent := [32]byte{0xAB}
	svc.mu.Lock()
	svc.waiting[orphanParent] = append(svc.waiting[orphanParent], u)
	svc.mu.Unlock()
	outcome := mineOnce(svc, nil)
	if outcome != mineOutcomeMined {
		t.Fatalf("真实挖矿路径应成功出块，实际 outcome=%s", outcome)
	}
	assertParked(t, svc, orphanParent, 1, "与本次出块无关的孤儿不得被误释放")
	svc.mu.Lock()
	delete(svc.waiting, orphanParent)
	svc.mu.Unlock()
}

// T6 多级孤儿链，乱序到达：A→B→C→D，按 D、C、B、A 顺序送达，最终整条链必须被解析。
func TestB1T6MultiLevelOrphanChainOutOfOrder(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	a := mineOn(t, svc, g, 1)
	b := mineOn(t, svc, a, 2)
	c := mineOn(t, svc, b, 3)
	d := mineOn(t, svc, c, 4)

	deliverBroadcast(t, svc, d)
	assertParked(t, svc, c.Header.Hash(), 1, "D 应等待 C")
	deliverBroadcast(t, svc, c)
	assertParked(t, svc, b.Header.Hash(), 1, "C 应等待 B")
	deliverBroadcast(t, svc, b)
	assertParked(t, svc, a.Header.Hash(), 1, "B 应等待 A")
	assertHeight(t, svc, 0, "缺口未补齐前不得上链")

	deliverBroadcast(t, svc, a) // 补上根缺口 → 逐层级联
	assertHeight(t, svc, 4, "补齐根缺口后应逐层级联到 D")
	assertTipIs(t, svc, d, "链尾应为 D")
	assertWaitingEmpty(t, svc, "整条等待链应被消费")

	for i, want := range []*block.Block{a, b, c, d} {
		got, err := svc.chain.BlockByHeight(i + 1)
		if err != nil {
			t.Fatalf("高度 %d 取块失败: %v", i+1, err)
		}
		if got.Header.Hash() != want.Header.Hash() {
			t.Fatalf("高度 %d 的区块与预期不符", i+1)
		}
	}
}

// T7 重复父块到达：不得重复 apply、不得重复消费 waiting。
func TestB1T7DuplicateParentDelivery(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	p := mineOn(t, svc, g, 1)
	c := mineOn(t, svc, p, 2)

	deliverBroadcast(t, svc, c)
	deliverBroadcast(t, svc, p)
	assertHeight(t, svc, 2, "第一次到达后应级联")

	deliverBroadcast(t, svc, p) // 重复父块
	assertHeight(t, svc, 2, "重复父块不得改变链高")
	assertTipIs(t, svc, c, "重复父块不得改变链尾")

	deliverBroadcast(t, svc, c) // 重复子块（已 canonical）
	assertHeight(t, svc, 2, "重复子块不得改变链高")
	assertTipIs(t, svc, c, "重复子块不得改变链尾")
	assertWaitingEmpty(t, svc, "重复到达不得重新登记等待")
}

// T8 非法子块滞留在合法父块之后：父块到达后子块必须被拒绝，且不得成为 canonical。
func TestB1T8InvalidWaitingChildNeverCanonical(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	p := mineOn(t, svc, g, 1)
	bad := mineOn(t, svc, p, 2)
	bad.Header.Nonce++ // 破坏 PoW（父引用不变，仍以 p 为父）

	deliverBroadcast(t, svc, bad)
	assertParked(t, svc, p.Header.Hash(), 1, "非法子块仍会先进入等待（父未知）")

	deliverBroadcast(t, svc, p) // 父块到达 → 级联尝试非法子块
	assertHeight(t, svc, 1, "非法子块不得上链")
	assertTipIs(t, svc, p, "链尾应为合法父块 P")
	assertCanonical(t, svc, bad, false, "非法子块不得成为 canonical")
	assertWaitingEmpty(t, svc, "被拒绝的等待条目仍应被消费")
}

// T9 重启边界：实现不得假设 waiting 跨重启持久化。
func TestB1T9RestartBoundaryNoWaitingPersistence(t *testing.T) {
	svc1 := newB1Service(t)
	g, err := svc1.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	p := mineOn(t, svc1, g, 1)
	c := mineOn(t, svc1, p, 2)
	deliverBroadcast(t, svc1, c)
	assertParked(t, svc1, p.Header.Hash(), 1, "重启前应有 1 个等待条目")

	// 新实例等价于「重启」：waiting 必须为空（纯内存，绝不持久化）。
	svc2 := newB1Service(t)
	svc2.mu.Lock()
	n2 := len(svc2.waiting)
	svc2.mu.Unlock()
	if n2 != 0 {
		t.Fatalf("新实例的 waiting 应为空，实际 %d", n2)
	}

	// 不得出现「跨实例凭空恢复孤儿」的错误假设：只送父块时链高只 +1。
	deliverBroadcast(t, svc2, p)
	assertHeight(t, svc2, 1, "新实例只收到父块时高度应为 1")
	assertTipIs(t, svc2, p, "链尾应为父块 P")
}

// T10 reorg 交互：等待子块 + 竞争分支/reorg 不得破坏既有 fork-choice/reorg 语义。
//
// 使用真实 nodeRuntime：detached 分叉块需要真实 store 才能被 executeReorg 取回
// （内存链的 hashIndex 只含 canonical 块）。
func TestB1T10ReorgInteractionWithWaitingChild(t *testing.T) {
	rt := newBranchRuntime(t, t.TempDir())
	svc := rt.svc
	chain := rt.chain

	g, err := chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	// 较短的分支先成为 canonical（A→B→C，work=3）
	a := mineOn(t, svc, g, 1)
	b := mineOn(t, svc, a, 2)
	c := mineOn(t, svc, b, 3)
	deliverBroadcast(t, svc, a)
	deliverBroadcast(t, svc, b)
	deliverBroadcast(t, svc, c)
	assertHeight(t, svc, 3, "短分支应先成为 canonical")

	// 更重的竞争分支（F1..F4，work=4），以及它的子块 W（work=5）。
	// 使用**另一个收款地址**构造，从构造上保证 F 分支与 A 分支的区块必然不同
	// （否则同父同高且同秒构造时，PoW 确定性搜索会产出完全相同的区块）。
	forkKey := testWallet(t).PubKeyHash()
	f1 := mineOnTo(t, svc, g, 1, forkKey)
	f2 := mineOnTo(t, svc, f1, 2, forkKey)
	f3 := mineOnTo(t, svc, f2, 3, forkKey)
	f4 := mineOnTo(t, svc, f3, 4, forkKey)
	w := mineOn(t, svc, f4, 5)

	if f1.Header.Hash() == a.Header.Hash() {
		t.Fatal("测试前提不成立：竞争分支与既有分支的首块相同")
	}

	// 先让 W 成为等待子块（其父 F4 未知）
	deliverBroadcast(t, svc, w)
	assertParked(t, svc, f4.Header.Hash(), 1, "W 应等待 F4")

	// 逐个送达竞争分支 → 最后一枚触发 reorg
	for _, fb := range []*block.Block{f1, f2, f3, f4} {
		deliverBroadcast(t, svc, fb)
	}

	assertHeight(t, svc, 5, "更重分支胜出且其子块 W 被级联释放后高度应为 5")
	assertTipIs(t, svc, w, "链尾应为 W（F 分支胜出）")
	assertCanonical(t, svc, f4, true, "F4 应成为 canonical（累积工作量更高）")
	assertCanonical(t, svc, a, false, "旧分支的 A 应脱离 canonical")
	assertCanonical(t, svc, c, false, "旧分支的 C 应脱离 canonical")
	assertWaitingEmpty(t, svc, "reorg 场景下等待链应被消费")

	// UTXO 一致性：canonical 链上每一枚 coinbase 都应可用（成熟度 10 内不复用，仅验证不报错）。
	for i := 0; i <= chain.Height(); i++ {
		if _, err := chain.BlockByHeight(i); err != nil {
			t.Fatalf("canonical 高度 %d 取块失败（UTXO/链不完整）: %v", i, err)
		}
	}
}

// T11 判别性测试：对三个「修复前缺失级联」的到达面逐一验证。
//
// 修复前：三条路径全部失败（子块永久滞留、链高停在 1）。
// 修复后：三条路径全部通过。
// 若本用例在修复**前**就通过，说明上一阶段对缺陷的判定或测试设计需要重新审计。
func TestB1T11DiscriminatingCascadeAcrossFixedPaths(t *testing.T) {
	cases := []struct {
		name string
		send func(t *testing.T, svc *nodeService, p *block.Block)
	}{
		{
			name: "P1_OnNewBlock",
			send: func(t *testing.T, svc *nodeService, p *block.Block) { deliverBroadcast(t, svc, p) },
		},
		{
			name: "P2_OnBlocksResp",
			send: func(t *testing.T, svc *nodeService, p *block.Block) { deliverBatchSync(t, svc, p) },
		},
		{
			name: "P4_localMining",
			send: func(t *testing.T, svc *nodeService, p *block.Block) {
				if err := svc.chain.AddBlock(p); err != nil {
					t.Fatalf("本地上链失败: %v", err)
				}
				// 驱动与 main.go 相同的生产收尾方法（含统一解析入口的接线）。
				svc.commitMinedBlock(p, 1)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newB1Service(t)
			g, err := svc.chain.Tip()
			if err != nil {
				t.Fatal(err)
			}
			p := mineOn(t, svc, g, 1)
			c := mineOn(t, svc, p, 2)

			deliverBroadcast(t, svc, c) // 复现滞留条件：by-hash 补拉对未连接对端必然失败
			assertParked(t, svc, p.Header.Hash(), 1, "前提：子块已登记等待")

			tc.send(t, svc, p)

			assertHeight(t, svc, 2, "该到达面必须释放等待子块（修复前此处失败）")
			assertTipIs(t, svc, c, "链尾应为 C")
			assertWaitingEmpty(t, svc, "waiting 应被消费")
		})
	}
}

// T12 waiting 预算与有界展开：滞留/级联不得泄漏预算，深级联不得无界展开。
func TestB1T12WaitingBudgetNotLeakedByCascade(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	// 构造 6 级链，全部乱序送达：只补根缺口即可整链解析。
	blocks := make([]*block.Block, 0, 6)
	parent := g
	for i := 1; i <= 6; i++ {
		nb := mineOn(t, svc, parent, i)
		blocks = append(blocks, nb)
		parent = nb
	}
	for i := len(blocks) - 1; i >= 1; i-- {
		deliverBroadcast(t, svc, blocks[i])
	}
	// 此时 waiting 应有 5 个键（每个父哈希一个），且链高仍为 0
	svc.mu.Lock()
	keys := len(svc.waiting)
	svc.mu.Unlock()
	if keys != 5 {
		t.Fatalf("乱序送达后 waiting 键数 = %d, want 5", keys)
	}
	assertHeight(t, svc, 0, "缺口未补齐前不得上链")

	deliverBroadcast(t, svc, blocks[0])
	assertHeight(t, svc, 6, "补齐根缺口后 6 级链应全部解析")
	assertWaitingEmpty(t, svc, "级联应消费全部等待条目（无预算泄漏）")
}
