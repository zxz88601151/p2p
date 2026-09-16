package blockchain

// AUTH-2-Cd §14.2：C-d canonical member hash set 八项契约测试（白盒，package blockchain）。
//
// 核心 correctness invariant（**非**性能指标）：
//
//	canonical member hash set ≡ { b.Header.Hash() : b ∈ bc.blocks }
//
// 覆盖 §14.2 八项 MANDATORY 契约：
//
//	TEST 1 CANONICAL-HASH-SET-EQUALITY —— 内容在任意观测时刻确定且一致（含 freeze/thaw 重启重建前后相等）
//	TEST 2 MEMBER-SET-EQUALITY         —— 双向集合相等（显式禁止 same-count-only）
//	TEST 3 PROJECTION-EQUALITY         —— 纯派生投影；M1/M2 写点后、任何读点前成立
//	TEST 4 ORDERING-EQUALITY           —— 不引入对顺序的依赖，不改变 bc.blocks 顺序语义
//	TEST 5 UNIQUENESS                  —— 投影哈希一一对应，无重复入 set
//	TEST 6 MEMBERSHIP-COMPLETENESS     —— bc.blocks 每块哈希均在 set 中（无遗漏）
//	TEST 7 MEMBERSHIP-EXCLUSIVITY      —— set 中不存在 bc.blocks 之外的哈希（无多余）
//	TEST 8 DETERMINISTIC-EQUALITY      —— 等价判定确定可复现（同状态必得同结论）
//
// 另含 §8 membership 返回语义逐位不变（空链/存在/不存在三态 + 绝不回退 storage）验证。
//
// 纪律：
//   - 全部断言为**双向、确定性、完整集合等价**；不使用 same-count-only、不使用单向 contains；
//   - 不修改任何共识语义、不修改 canonical hashing、不修改既有行为；
//   - **不引用任何性能数字**（本文件不含、也不得含任何 ms/µs/百分比/吞吐/speedup 断言）。
//
// 复用的既有 fixture/helper（未修改）：
//   - reorgTestFixture / newReorgFixture / mineNext / mineFork / saveDetached（reorg_test.go）
//   - blockAtHash（blockchain.go，C-1 投影查询）

import (
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
)

// canonicalSetConsistentWithBlocks 校验 C-d 核心不变式（双向）：
//
//	canonicalSet ≡ { b.Header.Hash() : b ∈ bc.blocks }
//
// 实现为**双向逐项**校验：方向 1 = completeness（blocks→set 无遗漏）；
// 方向 2 = exclusivity（set→blocks 无多余）。len 比较仅作快速失败提示，
// **不构成**契约判据（§14.2 明令 same-count 不是 contract）。
func canonicalSetConsistentWithBlocks(t *testing.T, bc *Blockchain) bool {
	t.Helper()

	// 方向 1 —— completeness：bc.blocks 每块哈希都在 set 中（无遗漏）
	for i, b := range bc.blocks {
		if _, ok := bc.canonicalSet[b.Header.Hash()]; !ok {
			t.Logf("方向1 失败：高度 %d 的哈希不在 set 中（遗漏）", i)
			return false
		}
	}

	// 方向 2 —— exclusivity：set 中不存在 bc.blocks 之外的哈希（无多余）
	projection := make(map[[32]byte]struct{}, len(bc.blocks))
	for _, b := range bc.blocks {
		projection[b.Header.Hash()] = struct{}{}
	}
	for h := range bc.canonicalSet {
		if _, ok := projection[h]; !ok {
			t.Logf("方向2 失败：set 中的哈希 %x 不在 projection 中（多余）", h[:4])
			return false
		}
	}

	// 双向逐项通过后的基数一致性（由 ⊆ 双向蕴含；此处仅作健全性核对）
	if len(bc.canonicalSet) != len(projection) {
		t.Logf("基数不一致：set=%d projection=%d", len(bc.canonicalSet), len(projection))
		return false
	}
	return true
}

// setHashes 拷贝当前 canonicalSet 的哈希集合（用于重建前后逐哈希比对）。
func setHashes(bc *Blockchain) map[[32]byte]struct{} {
	out := make(map[[32]byte]struct{}, len(bc.canonicalSet))
	for h := range bc.canonicalSet {
		out[h] = struct{}{}
	}
	return out
}

// TEST 1：canonical hash set equality —— 内容确定且一致，含 freeze/thaw 重启重建前后相等。
func TestCd_CanonicalHashSetEquality(t *testing.T) {
	f := newReorgFixture(t)
	b1 := f.mineNext(pow.MaxTargetBits) // h1
	b2 := f.mineNext(pow.MaxTargetBits) // h2

	// 任意观测时刻：内容确定且一致
	if !canonicalSetConsistentWithBlocks(t, f.chain) {
		t.Fatal("TEST 1: 构造/扩展后立即不一致")
	}
	beforeSet := setHashes(f.chain)
	beforeHeight := f.chain.Height()

	// freeze：关闭 store；thaw：重开并 recovery 重建
	if err := f.store.Close(); err != nil {
		t.Fatalf("TEST 1: 关闭 store 失败: %v", err)
	}
	store2, err := storage.OpenFileBlockStore(f.dir)
	if err != nil {
		t.Fatalf("TEST 1: 重开 store 失败: %v", err)
	}
	t.Cleanup(func() { _ = store2.Close() })
	bc2, err := NewBlockchainFromStore(store2)
	if err != nil {
		t.Fatalf("TEST 1: freeze/thaw recovery 失败: %v", err)
	}

	// 重建前后高度相等
	if got := bc2.Height(); got != beforeHeight {
		t.Fatalf("TEST 1: freeze/thaw 高度不等 got=%d want=%d", got, beforeHeight)
	}
	// 重建后 set 与 bc.blocks 一致
	if !canonicalSetConsistentWithBlocks(t, bc2) {
		t.Fatal("TEST 1: freeze/thaw 重建后 set 与 bc.blocks 不一致")
	}
	// 重建前后 set **逐哈希**相等（非仅基数）
	afterSet := setHashes(bc2)
	if len(afterSet) != len(beforeSet) {
		t.Fatalf("TEST 1: 重建前后 set 基数不等 got=%d want=%d", len(afterSet), len(beforeSet))
	}
	for h := range beforeSet {
		if _, ok := afterSet[h]; !ok {
			t.Fatalf("TEST 1: 重建后丢失哈希 %x", h[:4])
		}
	}
	// 重建后逐块 membership 保持
	for i, orig := range []*block.Block{b1, b2} {
		if !bc2.canonicalContains(orig.Header.Hash()) {
			t.Fatalf("TEST 1: 重建后原块 h%d membership 丢失", i+1)
		}
	}
	if !bc2.canonicalContains(f.genesis.Header.Hash()) {
		t.Fatal("TEST 1: 重建后 genesis membership 丢失")
	}
}

// TEST 2：member-set equality —— 双向集合相等（显式禁止 same-count-only）。
func TestCd_MemberSetEqualityBidirectional(t *testing.T) {
	f := newReorgFixture(t)
	for i := 0; i < 3; i++ {
		f.mineNext(pow.MaxTargetBits)
	}

	if !canonicalSetConsistentWithBlocks(t, f.chain) {
		t.Fatal("TEST 2: 双向集合相等校验失败")
	}

	// 显式双向量化断言（不依赖 helper 的等价实现）
	proj := make(map[[32]byte]struct{}, len(f.chain.blocks))
	for _, b := range f.chain.blocks {
		proj[b.Header.Hash()] = struct{}{}
	}
	for h := range proj { // set ⊇ projection
		if _, ok := f.chain.canonicalSet[h]; !ok {
			t.Fatalf("TEST 2: set ⊉ projection，缺失 %x", h[:4])
		}
	}
	for h := range f.chain.canonicalSet { // set ⊆ projection
		if _, ok := proj[h]; !ok {
			t.Fatalf("TEST 2: set ⊄ projection，多余 %x", h[:4])
		}
	}
}

// TEST 3：projection equality —— 纯派生投影；在 M1（append）与 M2（整链替换）后立即成立。
func TestCd_ProjectionEqualityAtMutationPoints(t *testing.T) {
	f := newReorgFixture(t)

	// 构造点（创世初始化）后
	if !canonicalSetConsistentWithBlocks(t, f.chain) {
		t.Fatal("TEST 3: 构造点后不一致")
	}

	// M1：extendChain 的 canonical append —— 每次 append 后都必须立即成立
	for i := 1; i <= 2; i++ {
		f.mineNext(pow.MaxTargetBits)
		if !canonicalSetConsistentWithBlocks(t, f.chain) {
			t.Fatalf("TEST 3: M1 第 %d 次 append 后不一致", i)
		}
	}

	// M2：executeReorg 的整链替换 —— 反超 fork 触发
	a1 := f.mineFork(f.genesis.Header.Hash(), 1, pow.MaxTargetBits)
	a2 := f.mineFork(a1.Header.Hash(), 2, pow.MaxTargetBits)
	a3 := f.mineFork(a2.Header.Hash(), 3, pow.MaxTargetBits)
	for i, b := range []*block.Block{a1, a2, a3} {
		if err := f.chain.AddBlock(b); err != nil {
			t.Fatalf("TEST 3: fork 块 %d 投递失败: %v", i+1, err)
		}
	}
	if got := f.chain.Height(); got != 3 {
		t.Fatalf("TEST 3: reorg 后高度应为 3, got %d", got)
	}
	if !canonicalSetConsistentWithBlocks(t, f.chain) {
		t.Fatal("TEST 3: M2（executeReorg 整链替换）后不一致")
	}
}

// TEST 4：ordering equality —— set 不引入顺序依赖，且不改变 bc.blocks 既有顺序语义。
func TestCd_OrderingEquality(t *testing.T) {
	f := newReorgFixture(t)
	b1 := f.mineNext(pow.MaxTargetBits)
	b2 := f.mineNext(pow.MaxTargetBits)
	b3 := f.mineNext(pow.MaxTargetBits)

	hashes := [][32]byte{
		f.genesis.Header.Hash(), b1.Header.Hash(),
		b2.Header.Hash(), b3.Header.Hash(),
	}

	// 正序查询
	for _, h := range hashes {
		if !f.chain.canonicalContains(h) {
			t.Fatalf("TEST 4: 正序查询 %x 应为 true", h[:4])
		}
	}
	// 逆序查询 —— 结论必须与正序完全一致（不引入顺序依赖）
	for i := len(hashes) - 1; i >= 0; i-- {
		if !f.chain.canonicalContains(hashes[i]) {
			t.Fatalf("TEST 4: 逆序查询 %x 应为 true", hashes[i][:4])
		}
	}
	// bc.blocks 既有顺序语义未被 set 改变：高度序列必须严格递增且与自身一致
	for i := 1; i < len(f.chain.blocks); i++ {
		prev := f.chain.blocks[i-1].Header.Hash()
		if prev == f.chain.blocks[i].Header.Hash() {
			t.Fatalf("TEST 4: 高度 %d 与 %d 哈希相同（顺序语义异常）", i-1, i)
		}
	}
	if got := f.chain.Height(); got != 3 {
		t.Fatalf("TEST 4: 高度应为 3, got %d", got)
	}
	if !canonicalSetConsistentWithBlocks(t, f.chain) {
		t.Fatal("TEST 4: 顺序查询后不一致")
	}
}

// TEST 5：uniqueness —— 投影哈希一一对应，无重复入 set。
func TestCd_Uniqueness(t *testing.T) {
	f := newReorgFixture(t)
	f.mineNext(pow.MaxTargetBits)
	f.mineNext(pow.MaxTargetBits)

	// bc.blocks 内不得出现重复哈希（一一对应）
	seen := make(map[[32]byte]struct{}, len(f.chain.blocks))
	for i, b := range f.chain.blocks {
		h := b.Header.Hash()
		if _, dup := seen[h]; dup {
			t.Fatalf("TEST 5: bc.blocks 高度 %d 出现重复哈希", i)
		}
		seen[h] = struct{}{}
	}
	// set 基数必须等于去重后的投影基数（无重复入 set）
	if len(f.chain.canonicalSet) != len(seen) {
		t.Fatalf("TEST 5: set 基数 %d ≠ 去重投影基数 %d（存在重复入 set）",
			len(f.chain.canonicalSet), len(seen))
	}

	// 重复投递同一 canonical 块必须被拒绝，且不得改变 set
	tip := f.chain.blocks[len(f.chain.blocks)-1]
	before := len(f.chain.canonicalSet)
	if err := f.chain.AddBlock(tip); err == nil {
		t.Fatal("TEST 5: 重复投递 canonical 块应被拒绝")
	}
	if got := len(f.chain.canonicalSet); got != before {
		t.Fatalf("TEST 5: 重复投递改变了 set 基数 got=%d want=%d", got, before)
	}
	if !canonicalSetConsistentWithBlocks(t, f.chain) {
		t.Fatal("TEST 5: 重复投递后不一致")
	}
}

// TEST 6：membership completeness —— bc.blocks 每块哈希均在 set 中（无遗漏）。
func TestCd_MembershipCompleteness(t *testing.T) {
	f := newReorgFixture(t)
	for i := 0; i < 3; i++ {
		f.mineNext(pow.MaxTargetBits)
	}

	// 逐块（含 genesis）membership 必须为 true
	for i, b := range f.chain.blocks {
		if !f.chain.canonicalContains(b.Header.Hash()) {
			t.Fatalf("TEST 6: 高度 %d 的哈希遗漏（canonicalContains=false）", i)
		}
	}
	if len(f.chain.canonicalSet) < len(f.chain.blocks) {
		t.Fatalf("TEST 6: set 基数 %d < blocks 数 %d（存在遗漏）",
			len(f.chain.canonicalSet), len(f.chain.blocks))
	}
}

// TEST 7：membership exclusivity —— set 中不存在 bc.blocks 之外的哈希（无多余）。
func TestCd_MembershipExclusivity(t *testing.T) {
	f := newReorgFixture(t)
	f.mineNext(pow.MaxTargetBits) // h1

	// (a) detached 块（经 store 登记但非 canonical）绝不在 set 中
	fk := f.mineFork(f.genesis.Header.Hash(), 1, pow.MaxTargetBits)
	f.saveDetached(fk)
	if _, in := f.chain.canonicalSet[fk.Header.Hash()]; in {
		t.Fatal("TEST 7: detached 块不应进入 canonical set（多余）")
	}
	if f.chain.canonicalContains(fk.Header.Hash()) {
		t.Fatal("TEST 7: detached 块 canonicalContains 应为 false")
	}

	// (b) 任意不存在的哈希绝不在 set 中
	var missing [32]byte
	missing[0] = 0xAB
	missing[31] = 0xCD
	if _, in := f.chain.canonicalSet[missing]; in {
		t.Fatal("TEST 7: 不存在的哈希不应在 set 中")
	}
	if f.chain.canonicalContains(missing) {
		t.Fatal("TEST 7: 不存在的哈希 canonicalContains 应为 false")
	}

	// (c) 动态 exclusivity：reorg 后旧链块必须移出 set
	old := f.chain.blocks[len(f.chain.blocks)-1]
	a1 := f.mineFork(f.genesis.Header.Hash(), 1, pow.MaxTargetBits)
	a2 := f.mineFork(a1.Header.Hash(), 2, pow.MaxTargetBits)
	a3 := f.mineFork(a2.Header.Hash(), 3, pow.MaxTargetBits)
	for i, b := range []*block.Block{a1, a2, a3} {
		if err := f.chain.AddBlock(b); err != nil {
			t.Fatalf("TEST 7: fork 块 %d 投递失败: %v", i+1, err)
		}
	}
	if got := f.chain.Height(); got != 3 {
		t.Fatalf("TEST 7: reorg 后高度应为 3, got %d", got)
	}
	if _, in := f.chain.canonicalSet[old.Header.Hash()]; in {
		t.Fatal("TEST 7: reorg 后旧链块应移出 set（多余）")
	}
	if f.chain.canonicalContains(old.Header.Hash()) {
		t.Fatal("TEST 7: reorg 后旧链块 canonicalContains 应为 false")
	}
	if !canonicalSetConsistentWithBlocks(t, f.chain) {
		t.Fatal("TEST 7: reorg 后不一致")
	}
}

// TEST 8：deterministic equality —— 同状态必得同结论，且与直接投影计算一致。
func TestCd_DeterministicEquality(t *testing.T) {
	f := newReorgFixture(t)
	b1 := f.mineNext(pow.MaxTargetBits)
	b2 := f.mineNext(pow.MaxTargetBits)

	var missing [32]byte
	missing[0] = 0x7F

	targets := []struct {
		name string
		h    [32]byte
		want bool
	}{
		{"genesis", f.genesis.Header.Hash(), true},
		{"h1", b1.Header.Hash(), true},
		{"h2", b2.Header.Hash(), true},
		{"absent", missing, false},
	}

	// 同状态重复查询必得同结论（3 轮）
	for round := 1; round <= 3; round++ {
		for _, tc := range targets {
			if got := f.chain.canonicalContains(tc.h); got != tc.want {
				t.Fatalf("TEST 8: 第 %d 轮 %s(%x) 得 %v want %v（判定不确定）",
					round, tc.name, tc.h[:4], got, tc.want)
			}
		}
	}

	// 与「直接用 bc.blocks 投影计算」的结论逐项一致
	proj := make(map[[32]byte]struct{}, len(f.chain.blocks))
	for _, b := range f.chain.blocks {
		proj[b.Header.Hash()] = struct{}{}
	}
	for _, tc := range targets {
		_, wantProj := proj[tc.h]
		if got := f.chain.canonicalContains(tc.h); got != wantProj {
			t.Fatalf("TEST 8: %s(%x) 与投影计算不一致 got=%v proj=%v",
				tc.name, tc.h[:4], got, wantProj)
		}
	}
}

// TestCd_LookupSemanticsPreserved 验证 §8：membership 返回语义逐位不变（三态 + 绝不回退 storage）。
func TestCd_LookupSemanticsPreserved(t *testing.T) {
	f := newReorgFixture(t)

	// 空链态（仅 genesis）
	if !f.chain.canonicalContains(f.genesis.Header.Hash()) {
		t.Fatal("TEST L: 空链态 genesis 应为 true")
	}
	var missing [32]byte
	missing[0] = 0x01
	missing[31] = 0x02
	if f.chain.canonicalContains(missing) {
		t.Fatal("TEST L: 空链态不存在哈希应为 false")
	}

	// 存在态
	b1 := f.mineNext(pow.MaxTargetBits)
	if !f.chain.canonicalContains(b1.Header.Hash()) {
		t.Fatal("TEST L: 存在态应为 true")
	}

	// 不存在态
	if f.chain.canonicalContains(missing) {
		t.Fatal("TEST L: 不存在态应为 false")
	}

	// 绝不回退 storage：detached 块虽可经 blockAtHash/store 查得，
	// canonicalContains 必须返回 false（语义与改写前的线性扫描一致）
	fk := f.mineFork(f.genesis.Header.Hash(), 1, pow.MaxTargetBits)
	f.saveDetached(fk)
	if got, err := f.chain.blockAtHash(fk.Header.Hash()); err != nil || got == nil {
		t.Fatalf("TEST L: 前置失败—detached 块应可经 store 查得: got=%v err=%v", got, err)
	}
	if f.chain.canonicalContains(fk.Header.Hash()) {
		t.Fatal("TEST L: canonicalContains 不得回退 storage（detached 应为 false）")
	}

	// 公开 API（IsCanonicalHash）一致性
	if !f.chain.IsCanonicalHash(f.genesis.Header.Hash()) {
		t.Fatal("TEST L: IsCanonicalHash(genesis) 应为 true")
	}
	if f.chain.IsCanonicalHash(fk.Header.Hash()) {
		t.Fatal("TEST L: IsCanonicalHash(detached) 应为 false")
	}
	if f.chain.IsCanonicalHash(missing) {
		t.Fatal("TEST L: IsCanonicalHash(不存在) 应为 false")
	}
}
