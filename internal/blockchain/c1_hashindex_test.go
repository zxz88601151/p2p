package blockchain

// AUTH-2-C1 §8：C-1 hash index 八项契约测试（白盒，package blockchain）。
//
// 覆盖：
//
//	TEST A HIT        —— canonical 块 index 命中，返回同一 *block.Block 指针
//	TEST B MISS       —— 不存在 hash → 既有 store fallback → ErrNotFound
//	TEST C DETACHED   —— detached 块语义保持：不在 canonical index、经 store 查得
//	TEST D DISCONNECT —— reorg disconnect 后旧链块移出 canonical index
//	TEST E CONNECT    —— reorg connect 后新链块进入 index
//	TEST F REORG      —— reorg 后 index ≡ bc.blocks 全量一致
//	TEST G RECOVERY   —— NewBlockchainFromStore 重建后 index 完整
//	TEST H OBS COUNT  —— obsBlockLookups 计数与 hit/miss 无关，各 +1
//
// 全部测试为行为/不变量断言，不修改任何共识语义。

import (
	"errors"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
)

// indexConsistentWithBlocks 校验 C-1 核心不变量：hashIndex ≡ bc.blocks 投影。
func indexConsistentWithBlocks(t *testing.T, bc *Blockchain) bool {
	t.Helper()
	if len(bc.hashIndex) != len(bc.blocks) {
		t.Logf("index size=%d blocks=%d", len(bc.hashIndex), len(bc.blocks))
		return false
	}
	for i, b := range bc.blocks {
		if got, ok := bc.hashIndex[b.Header.Hash()]; !ok || got != b {
			t.Logf("高度 %d 投影不一致 (ok=%v samePtr=%v)", i, ok, got == b)
			return false
		}
	}
	return true
}

// TEST A：canonical append 后 index 命中且返回同一指针；genesis 自初始化即在 index。
func TestC1_IndexHit(t *testing.T) {
	f := newReorgFixture(t)

	if got, err := f.chain.blockAtHash(f.genesis.Header.Hash()); err != nil || got != f.genesis {
		t.Fatalf("TEST A: genesis index 命中失败: got=%v err=%v", got, err)
	}
	b1 := f.mineNext(pow.MaxTargetBits)
	b2 := f.mineNext(pow.MaxTargetBits)

	if got, err := f.chain.blockAtHash(b1.Header.Hash()); err != nil || got != b1 {
		t.Fatalf("TEST A: index hit 应返回同一块指针: got=%v err=%v", got, err)
	}
	if got, err := f.chain.blockAtHash(b2.Header.Hash()); err != nil || got != b2 {
		t.Fatalf("TEST A: b2 hit 指针不等: got=%v err=%v", got, err)
	}
	if !indexConsistentWithBlocks(t, f.chain) {
		t.Fatal("TEST A: append 后 index 与 bc.blocks 不一致")
	}
}

// TEST B：不存在的 hash 走 store fallback，最终 ErrNotFound，无 panic、无副作用。
func TestC1_IndexMiss(t *testing.T) {
	f := newReorgFixture(t)
	f.mineNext(pow.MaxTargetBits)

	var missing [32]byte
	missing[0] = 0xAB
	missing[31] = 0xCD
	b, err := f.chain.blockAtHash(missing)
	if b != nil {
		t.Fatalf("TEST B: miss 应返回 nil block, got=%v", b)
	}
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("TEST B: miss 应传播 storage.ErrNotFound, got err=%v", err)
	}
}

// TEST C：detached 块不在 canonical index，但经 store fallback 查得（语义不变）。
func TestC1_DetachedSemantics(t *testing.T) {
	f := newReorgFixture(t)
	f.mineNext(pow.MaxTargetBits) // h1

	fk := f.mineFork(f.genesis.Header.Hash(), 1, pow.MaxTargetBits)
	f.saveDetached(fk)

	if _, inIndex := f.chain.hashIndex[fk.Header.Hash()]; inIndex {
		t.Fatal("TEST C: detached 块不应进入 canonical index")
	}
	got, err := f.chain.blockAtHash(fk.Header.Hash())
	if err != nil || got != fk {
		t.Fatalf("TEST C: detached 块应经 store 查得且为原对象: got=%v err=%v", got, err)
	}
}

// TEST D/E/F：真实 reorg（AddBlock 自动触发）后 index 与 bc.blocks 全量一致；
// disconnect 旧链块移出投影、connect 新链块进入投影。
func TestC1_ReorgIndexConsistency(t *testing.T) {
	f := newReorgFixture(t)
	old1 := f.mineNext(pow.MaxTargetBits) // h1
	old2 := f.mineNext(pow.MaxTargetBits) // h2

	// fork：genesis → a1 → a2 → a3（a2 可能 tie 胜提前 reorg，a3 兜底严格反超；
	// 两种结果下 reorg 必然发生且最终高度=3，断言对两种 tie 结果均成立）
	a1 := f.mineFork(f.genesis.Header.Hash(), 1, pow.MaxTargetBits)
	a2 := f.mineFork(a1.Header.Hash(), 2, pow.MaxTargetBits)
	a3 := f.mineFork(a2.Header.Hash(), 3, pow.MaxTargetBits)
	fork := []*block.Block{a1, a2, a3}
	for i, b := range fork {
		if err := f.chain.AddBlock(b); err != nil {
			t.Fatalf("TEST D: fork 块 %d 投递失败: %v", i+1, err)
		}
	}
	if got := f.chain.Height(); got != 3 {
		t.Fatalf("TEST D: reorg 后高度应为 3, got %d", got)
	}

	// TEST D：旧 canonical 块必须移出 index
	for i, old := range []*block.Block{old1, old2} {
		if _, in := f.chain.hashIndex[old.Header.Hash()]; in {
			t.Fatalf("TEST D: disconnect 旧块 h%d 不应残留于 canonical index", i+1)
		}
	}
	// TEST E：connect 新链块必须全部在 index（且为同一对象）
	for i, nb := range fork {
		if got := f.chain.hashIndex[nb.Header.Hash()]; got != nb {
			t.Fatalf("TEST E: connect 块 %d 应以同一对象进入 index", i+1)
		}
	}
	// TEST F：全量一致性
	if !indexConsistentWithBlocks(t, f.chain) {
		t.Fatal("TEST F: reorg 后 index 与 bc.blocks 不一致")
	}
	// 旧链块经 store fallback 仍可查得（v2 records 保留 detached 语义）
	if got, err := f.chain.blockAtHash(old1.Header.Hash()); err != nil || got != old1 {
		t.Fatalf("TEST D 附加: 旧链块应经 store 查得: got=%v err=%v", got, err)
	}
}

// TEST G：recovery（NewBlockchainFromStore）后 index 由回放路径自动重建并完整。
func TestC1_RecoveryIndex(t *testing.T) {
	f := newReorgFixture(t)
	b1 := f.mineNext(pow.MaxTargetBits)
	b2 := f.mineNext(pow.MaxTargetBits)
	b3 := f.mineNext(pow.MaxTargetBits)
	if err := f.store.Close(); err != nil {
		t.Fatalf("关闭原 store 失败: %v", err)
	}

	store2, err := storage.OpenFileBlockStore(f.dir)
	if err != nil {
		t.Fatalf("重开存储失败: %v", err)
	}
	t.Cleanup(func() { _ = store2.Close() })
	bc2, err := NewBlockchainFromStore(store2)
	if err != nil {
		t.Fatalf("recovery 加载失败: %v", err)
	}

	if got, want := bc2.Height(), 3; got != want {
		t.Fatalf("TEST G: recovery 高度应为 %d, got %d", want, got)
	}
	if !indexConsistentWithBlocks(t, bc2) {
		t.Fatal("TEST G: recovery 后 index 与 bc.blocks 不一致")
	}
	// 逐块验证：recovery 链中每个原块哈希都能经 blockAtHash 命中
	for i, orig := range []*block.Block{b1, b2, b3} {
		got, err := bc2.blockAtHash(orig.Header.Hash())
		if err != nil || got == nil {
			t.Fatalf("TEST G: recovery 后原块 h%d 未命中: got=%v err=%v", i+1, got, err)
		}
		if got.Header.Hash() != orig.Header.Hash() {
			t.Fatalf("TEST G: recovery 后原块 h%d 哈希不匹配", i+1)
		}
	}
}

// TEST H：obsBlockLookups 计数位置与语义不变——hit 与 miss 各恰好 +1。
func TestC1_ObservationCount(t *testing.T) {
	f := newReorgFixture(t)
	f.mineNext(pow.MaxTargetBits) // h1（canonical，index 命中路径）

	var missing [32]byte
	missing[0] = 0x11

	before := obsBlockLookups.Load()
	_, _ = f.chain.blockAtHash(f.genesis.Header.Hash()) // hit
	afterHit := obsBlockLookups.Load()
	_, _ = f.chain.blockAtHash(missing) // miss → store fallback
	afterMiss := obsBlockLookups.Load()

	if d := afterHit - before; d != 1 {
		t.Fatalf("TEST H: hit 应恰好 +1, got Δ=%d", d)
	}
	if d := afterMiss - afterHit; d != 1 {
		t.Fatalf("TEST H: miss 应恰好 +1（计数与 hit/miss 无关）, got Δ=%d", d)
	}
}
