// settip_test.go 覆盖 REORG-1B 树级 tip 基础设施的测试矩阵（T6–T11）。
//
// T1–T5（Index insertion / Parent lookup / Height / Cumulative work / 不同难度分支比较）
// 已由 blocktree_test.go 的 TestA–TestJ 覆盖，本文件不重复，仅在 §REPORT 中引用。
//
// 本文件所有用例均【不触及】UTXO / mempool / 存储 / cmd/node —— blocktree 包
// 是纯内存结构，reorg 执行保持 OFF（见 TestT10_ReorgOff_NoAutoSwitchOnAddBlock
// 与生产路径隔离 grep 证明）。
package blocktree

import (
	"math/big"
	"testing"
)

// =====================================================================
// T6 — Lower-height / higher cumulative work（chainwork ≠ height 的关键证明）
// =====================================================================

// 构造：共享 genesis G(bits=16)。
//
//	LONG  分支：G → L1 → L2 → L3 → L4   （每块 bits=16，height 4）
//	SHORT 分支：G → S1 → S2 → S3        （每块 bits=32，height 3）
//
// 期望：S3(height=3) 的 CumulativeWork >> L4(height=4) 的 CumulativeWork。
// 证明：解钳后 chainwork 不再 ≡ height×const，低高度链可具更高 work。
func TestT6_LowerHeightHigherWork(t *testing.T) {
	tr := NewBlockTree()
	g := mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)

	// LONG 分支（低难度、长链）
	l1 := mustAdd(t, tr, h32(2), g.Hash, 1, 16, 1100)
	l2 := mustAdd(t, tr, h32(3), l1.Hash, 2, 16, 1200)
	l3 := mustAdd(t, tr, h32(4), l2.Hash, 3, 16, 1300)
	l4 := mustAdd(t, tr, h32(5), l3.Hash, 4, 16, 1400)

	// SHORT 分支（高难度、短链）—— 共享 genesis
	s1 := mustAdd(t, tr, h32(6), g.Hash, 1, 32, 1100)
	s2 := mustAdd(t, tr, h32(7), s1.Hash, 2, 32, 1200)
	s3 := mustAdd(t, tr, h32(8), s2.Hash, 3, 32, 1300)

	// 显式数学断言：CW(L4) = 5×2^16；CW(S3) = 2^16 + 3×2^32
	wantL4 := new(big.Int).Add(big.NewInt(0), WorkOfBits(16))
	for i := 0; i < 4; i++ { // L1..L4 共 4 块 + G = 5 块
		wantL4.Add(wantL4, WorkOfBits(16))
	}
	wantS3 := new(big.Int).Set(WorkOfBits(16)) // G
	for i := 0; i < 3; i++ {                   // S1..S3
		wantS3.Add(wantS3, WorkOfBits(32))
	}
	if l4.CumulativeWork.Cmp(wantL4) != 0 {
		t.Fatalf("L4 CW = %s, want %s", l4.CumulativeWork, wantL4)
	}
	if s3.CumulativeWork.Cmp(wantS3) != 0 {
		t.Fatalf("S3 CW = %s, want %s", s3.CumulativeWork, wantS3)
	}

	// 关键断言：S3(height 3) work > L4(height 4) work
	if s3.CumulativeWork.Cmp(l4.CumulativeWork) <= 0 {
		t.Fatalf("T6 失败：短高分链 work(%s) 应 > 长低分链 work(%s)",
			s3.CumulativeWork, l4.CumulativeWork)
	}
	if s3.Height >= l4.Height {
		t.Fatalf("前置错误：S3 height %d 应 < L4 height %d", s3.Height, l4.Height)
	}

	// SetTip 到长链 L4，再 ShouldReorg(S3) 应返回 true（work 反超）
	if err := tr.SetTip(l4); err != nil {
		t.Fatalf("SetTip(L4): %v", err)
	}
	want, _, err := tr.ShouldReorg(s3)
	if err != nil {
		t.Fatalf("ShouldReorg(S3): %v", err)
	}
	if !want {
		t.Fatal("ShouldReorg(S3) 应=true（低高度高 work 反超），得 false")
	}

	if errs := tr.CheckInvariant(); errs != nil {
		t.Fatalf("CheckInvariant: %v", errs)
	}
}

// =====================================================================
// T7 — SetTip 契约
// =====================================================================

func TestT7_SetTipContract(t *testing.T) {
	tr := NewBlockTree()
	g := mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	c := mustAdd(t, tr, h32(2), g.Hash, 1, 16, 1100)

	// 初始无 tip
	if tr.BestTip() != nil {
		t.Fatal("新树 BestTip 应为 nil")
	}
	if tr.ActiveHeight() != -1 {
		t.Fatalf("ActiveHeight 初始 = %d, want -1", tr.ActiveHeight())
	}
	if tr.BestTipWork() != nil {
		t.Fatal("BestTipWork 初始应为 nil")
	}

	// SetTip(nil) → ErrNilTip
	if err := tr.SetTip(nil); err != ErrNilTip {
		t.Fatalf("SetTip(nil) err = %v, want ErrNilTip", err)
	}

	// SetTip(游离节点) → ErrTipNotInTree
	fake := &BlockNode{Hash: h32(99)}
	if err := tr.SetTip(fake); err == nil || err == ErrNilTip {
		t.Fatalf("SetTip(游离节点) 应返回 ErrTipNotInTree，得 %v", err)
	}

	// SetTip(合法 C)
	if err := tr.SetTip(c); err != nil {
		t.Fatalf("SetTip(C): %v", err)
	}
	if tr.BestTip() != c {
		t.Fatal("SetTip 后 BestTip != C")
	}
	if tr.ActiveHeight() != 1 {
		t.Fatalf("ActiveHeight = %d, want 1", tr.ActiveHeight())
	}
	if tr.BestTipWork() == nil || tr.BestTipWork().Cmp(c.CumulativeWork) != 0 {
		t.Fatalf("BestTipWork != C.CumulativeWork")
	}

	// SetTip(G) 切回 genesis
	if err := tr.SetTip(g); err != nil {
		t.Fatalf("SetTip(G): %v", err)
	}
	if tr.ActiveHeight() != 0 {
		t.Fatalf("ActiveHeight = %d, want 0", tr.ActiveHeight())
	}

	// 失败路径不应改动 bestTip（原子语义）：SetTip(游离) 后 bestTip 仍是 G
	_ = tr.SetTip(fake)
	if tr.BestTip() != g {
		t.Fatal("SetTip 失败后 bestTip 应保持不变（原子语义）")
	}
}

// =====================================================================
// T8 — Active-chain membership
// =====================================================================

func TestT8_ActiveChainMembership(t *testing.T) {
	tr := NewBlockTree()
	g := mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	c := mustAdd(t, tr, h32(2), g.Hash, 1, 16, 1100)
	d := mustAdd(t, tr, h32(3), g.Hash, 1, 16, 1100) // 侧链兄弟
	e := mustAdd(t, tr, h32(4), d.Hash, 2, 16, 1200) // 侧链续块

	if err := tr.SetTip(c); err != nil {
		t.Fatalf("SetTip(C): %v", err)
	}

	// C 在活动链
	if !tr.IsActiveChain(c) {
		t.Fatal("C 应在活动链上")
	}
	// G（根）在活动链
	if !tr.IsActiveChain(g) {
		t.Fatal("G 应在活动链上")
	}
	// D（侧链兄弟）不在活动链
	if tr.IsActiveChain(d) {
		t.Fatal("D 是侧链兄弟，不应在活动链上")
	}
	// E（侧链续块）不在活动链
	if tr.IsActiveChain(e) {
		t.Fatal("E 是侧链续块，不应在活动链上")
	}
	// nil 不在活动链
	if tr.IsActiveChain(nil) {
		t.Fatal("nil 不应在活动链上")
	}

	// ActiveChain() 顺序 [C, G]
	chain := tr.ActiveChain()
	if len(chain) != 2 || chain[0] != c || chain[1] != g {
		t.Fatalf("ActiveChain = %v, want [C, G]", chain)
	}

	// ResetTip 后无活动链
	tr.ResetTip()
	if tr.BestTip() != nil || tr.IsActiveChain(c) || tr.ActiveChain() != nil {
		t.Fatal("ResetTip 后应无活动链")
	}
}

// =====================================================================
// T9 — Competing branch（§10 拓扑：fork at B, A→B→C / A→B→D→E）
// =====================================================================

func TestT9_CompetingBranch(t *testing.T) {
	tr := NewBlockTree()
	a := mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000) // A = genesis, height 0
	b := mustAdd(t, tr, h32(2), a.Hash, 1, 16, 1100)     // B, height 1（fork 点）
	c := mustAdd(t, tr, h32(3), b.Hash, 2, 16, 1200)     // C, height 2（活动分支）
	d := mustAdd(t, tr, h32(4), b.Hash, 2, 16, 1200)     // D, height 2（侧链）
	e := mustAdd(t, tr, h32(5), d.Hash, 3, 16, 1300)     // E, height 3（侧链续块）

	// 两条分支均存在（未被丢弃）
	if tr.LookupNode(c.Hash) == nil {
		t.Fatal("C 应存在于树中")
	}
	if tr.LookupNode(d.Hash) == nil {
		t.Fatal("D 应存在于树中")
	}
	if tr.LookupNode(e.Hash) == nil {
		t.Fatal("E 应存在于树中")
	}

	// 设置活动链尾 = C
	if err := tr.SetTip(c); err != nil {
		t.Fatalf("SetTip(C): %v", err)
	}
	// candidate = E（侧链续块）
	want, _, err := tr.ShouldReorg(e)
	if err != nil {
		t.Fatalf("ShouldReorg(E): %v", err)
	}
	// 注意：此处 E 的 work(4×2^16) > C 的 work(3×2^16)，故应切换
	if !want {
		t.Fatal("ShouldReorg(E) 应=true（E work > C work）")
	}
	// 但【尚未】SetTip → bestTip 仍是 C（决策与执行分离）
	if tr.BestTip() != c {
		t.Fatal("ShouldReorg 不应改动 bestTip（决策与执行分离）")
	}

	if errs := tr.CheckInvariant(); errs != nil {
		t.Fatalf("CheckInvariant: %v", errs)
	}
}

// =====================================================================
// T10 — Reorg OFF：AddBlock 不自动触发 SetTip / 链切换
// =====================================================================

// TestT10_ReorgOff_NoAutoSwitchOnAddBlock 证明 blocktree.AddBlock 即便收到
// 更高 work 的侧链块，也不会自动切换 bestTip——SetTip 必须由调用方显式发起。
// 这是 REORG-1B「REORG EXECUTION = OFF」在数据结构层的直接证据。
func TestT10_ReorgOff_NoAutoSwitchOnAddBlock(t *testing.T) {
	tr := NewBlockTree()
	g := mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	c := mustAdd(t, tr, h32(2), g.Hash, 1, 16, 1100) // 活动分支（低 work）

	if err := tr.SetTip(c); err != nil {
		t.Fatalf("SetTip(C): %v", err)
	}

	// 现在加入一条 work 更高的侧链续块（bits=32，远超 16）
	// 注意：必须从已存在的父接入。g 已存在 → 从 g 接侧链。
	high := mustAdd(t, tr, h32(3), g.Hash, 1, 32, 1100) // 侧链兄弟，bits=32
	// high.CumulativeWork = 2^16 + 2^32 >> c.CumulativeWork = 2×2^16

	// 关键断言：AddBlock(high) 后 bestTip 仍是 C，没有自动切换
	if tr.BestTip() != c {
		t.Fatalf("REORG OFF 失败：AddBlock(high) 后 bestTip 被自动切到 %p，应保持 C", tr.BestTip())
	}
	// 即便 high work >> active，也不自动 SetTip
	want, _, _ := tr.ShouldReorg(high)
	if !want {
		t.Fatal("前置错误：high 应当 ShouldReorg=true")
	}
	// 但 bestTip 仍是 C（未执行）
	if tr.BestTip() != c {
		t.Fatal("ShouldReorg=true 不等于已切换：bestTip 应仍是 C")
	}

	// 只有显式 SetTip 才切换
	if err := tr.SetTip(high); err != nil {
		t.Fatalf("显式 SetTip(high): %v", err)
	}
	if tr.BestTip() != high {
		t.Fatal("显式 SetTip(high) 后 bestTip 应=high")
	}
}

// TestT10b_TieBreakDeterministic 证明等 work 时 tie-break 由 tip hash 大端较大者胜，
// 全网可复算（不依赖到达顺序）。
func TestT10b_TieBreakDeterministic(t *testing.T) {
	tr := NewBlockTree()
	g := mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	// 两个等 work 兄弟：C1=hash(10), C2=hash(20)，均 bits=16 height 1 → CW 相等
	c1 := mustAdd(t, tr, h32(10), g.Hash, 1, 16, 1100)
	c2 := mustAdd(t, tr, h32(20), g.Hash, 1, 16, 1100)

	if c1.CumulativeWork.Cmp(c2.CumulativeWork) != 0 {
		t.Fatal("前置错误：C1/C2 work 应相等")
	}

	// SetTip(C1) 为活动；ShouldReorg(C2) 应 tie → tie-break：h32(20)>h32(10) → C2 胜
	if err := tr.SetTip(c1); err != nil {
		t.Fatalf("SetTip(C1): %v", err)
	}
	want, reason, err := tr.ShouldReorg(c2)
	if err != nil || !want {
		t.Fatalf("ShouldReorg(C2) 应=true(tie-break)，err=%v want=%v reason=%s", err, want, reason)
	}
	// 反向：SetTip(C2) 为活动；ShouldReorg(C1) 应 tie → C1 hash 较小 → C1 负 → 不切换
	if err := tr.SetTip(c2); err != nil {
		t.Fatalf("SetTip(C2): %v", err)
	}
	want2, _, err2 := tr.ShouldReorg(c1)
	if err2 != nil || want2 {
		t.Fatalf("ShouldReorg(C1) 应=false(tie-break C1 较小负)，err=%v want=%v", err2, want2)
	}
}

// =====================================================================
// T11 — Restart / reconstruction 确定性
// =====================================================================

// TestT11_RestartReconstructionDeterministic 证明 blocktree 状态是其插入块的纯函数，
// 「停止→重启→按相同顺序重放」得到完全一致的 bestTip/bestTipWork/active-chain。
//
// 本文件【不持久化】BlockIndex（落盘属 REORG-1E / R5）。本测试用「新建第二棵树 +
// 按【固定顺序切片】重放」模拟重启重建，刻意避开 map 迭代序（BT-1 的根因）。
// 生产重建路径（设计 §C.7）从 blocks.dat 顺序扫描重放，亦为确定序，与本测试同构。
func TestT11_RestartReconstructionDeterministic(t *testing.T) {
	// 第一棵树：构建 A→B→C，SetTip(C)
	tr1 := NewBlockTree()
	a := mustAdd(t, tr1, h32(1), [32]byte{}, 0, 16, 1000)
	b := mustAdd(t, tr1, h32(2), a.Hash, 1, 16, 1100)
	c := mustAdd(t, tr1, h32(3), b.Hash, 2, 16, 1200)
	if err := tr1.SetTip(c); err != nil {
		t.Fatalf("tr1 SetTip(C): %v", err)
	}

	// 模拟重启：新建第二棵树，按【固定切片顺序】重放（非 map 迭代，避开 BT-1）
	replay := []struct {
		hash, parent [32]byte
		height       int
		bits         uint32
		ts           int64
	}{
		{h32(1), [32]byte{}, 0, 16, 1000},
		{h32(2), h32(1), 1, 16, 1100},
		{h32(3), h32(2), 2, 16, 1200},
	}
	tr2 := NewBlockTree()
	for _, r := range replay {
		mustAdd(t, tr2, r.hash, r.parent, r.height, r.bits, r.ts)
	}
	c2 := tr2.LookupNode(h32(3))
	if c2 == nil {
		t.Fatal("tr2 重建后 C 不存在")
	}
	if err := tr2.SetTip(c2); err != nil {
		t.Fatalf("tr2 SetTip(C): %v", err)
	}

	// 一致性断言：两棵树 tip/work/active-chain 完全一致
	if tr1.ActiveHeight() != tr2.ActiveHeight() {
		t.Fatalf("ActiveHeight 不一致：%d vs %d", tr1.ActiveHeight(), tr2.ActiveHeight())
	}
	if tr1.BestTipWork().Cmp(tr2.BestTipWork()) != 0 {
		t.Fatalf("BestTipWork 不一致：%s vs %s", tr1.BestTipWork(), tr2.BestTipWork())
	}
	if tr1.BestTip().Hash != tr2.BestTip().Hash {
		t.Fatal("BestTip.Hash 不一致")
	}
	// active-chain 长度与各节点 hash 序列一致
	ch1, ch2 := tr1.ActiveChain(), tr2.ActiveChain()
	if len(ch1) != len(ch2) {
		t.Fatalf("ActiveChain 长度不一致：%d vs %d", len(ch1), len(ch2))
	}
	for i := range ch1 {
		if ch1[i].Hash != ch2[i].Hash {
			t.Fatalf("ActiveChain[%d] hash 不一致", i)
		}
	}
	// CheckInvariant 两树均干净
	if errs := tr1.CheckInvariant(); errs != nil {
		t.Fatalf("tr1 CheckInvariant: %v", errs)
	}
	if errs := tr2.CheckInvariant(); errs != nil {
		t.Fatalf("tr2 CheckInvariant: %v", errs)
	}
}

// TestT11b_NoPersistenceByDesign 文档化断言：BlockTree 无任何落盘字段/方法。
// 持久化属 REORG-1E（R5：block_index.dat + active tip 指针）。
// 本测试仅以「调用 ResetTip 后第二棵树可独立重建」佐证 tip 状态非持久、
// 完全由重放确定——即本阶段不需要、也不应实现持久化重建。
func TestT11b_NoPersistenceByDesign(t *testing.T) {
	tr := NewBlockTree()
	g := mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	if err := tr.SetTip(g); err != nil {
		t.Fatalf("SetTip(G): %v", err)
	}
	tr.ResetTip()
	// 重置后无 tip；重建须由调用方按块重放 + SetTip（见 TestT11）
	if tr.BestTip() != nil {
		t.Fatal("ResetTip 后 BestTip 应为 nil（tip 状态非持久）")
	}
}
