// 本文件是 PHASE GENESIS-0.1 的 P0 回归测试，锁定 replay / persistence 不变式：
//
//  1. 启动回放不得写回存储（blocks.dat 大小与持久化记录数都不变）
//  2. 重启后 Height / TipHash 不变
//  3. 持久化记录数恒等于「链高 + 1」
//  4. 第二次及以后的重启必须仍然成功
//  5. 运行时产生的新区块必须仍然真实落盘（防止用「干脆不 SaveBlock」来伪修复）
//  6. 已损坏（含重复记录）的存储必须被启动回放明确拒绝，不得静默接受
//
// 背景：修复前 NewBlockchainFromStore 的回放循环调用 AddBlock，而 AddBlock 会
// SaveBlock，于是每次启动都把历史区块重复追加进 append-only 的 blocks.dat，
// 第二次启动时因 prev-hash 不匹配而永久无法启动（GENESIS-0 实测 EXIT=1）。
package blockchain_test

import (
	"os"
	"path/filepath"
	"testing"

	"p2pchain/internal/blockchain"
	"p2pchain/internal/storage"
)

// ---- 辅助 ----

func blocksDatPath(dir string) string { return filepath.Join(dir, "blocks.dat") }

// datSize 返回 blocks.dat 的字节数（Stat 不需要文件句柄，句柄打开时也可读）。
func datSize(t *testing.T, dir string) int64 {
	t.Helper()
	st, err := os.Stat(blocksDatPath(dir))
	if err != nil {
		t.Fatalf("stat blocks.dat 失败: %v", err)
	}
	return st.Size()
}

// persistedCount 以只读方式统计已持久化记录数（不持有写句柄，可在节点运行期间安全调用）。
func persistedCount(t *testing.T, dir string) int {
	t.Helper()
	s, err := storage.OpenFileBlockStoreReadOnly(dir)
	if err != nil {
		t.Fatalf("只读打开存储失败: %v", err)
	}
	defer s.Close()
	h, err := s.Height()
	if err != nil {
		t.Fatalf("读取存储高度失败: %v", err)
	}
	return h + 1
}

// openChain 模拟一次节点启动：打开（必要时初始化）存储，并从磁盘回放重建链。
func openChain(t *testing.T, dir string) (*blockchain.Blockchain, *storage.FileBlockStore) {
	t.Helper()
	store, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	bc, err := blockchain.NewBlockchainFromStore(store)
	if err != nil {
		store.Close()
		t.Fatalf("从存储加载链失败: %v", err)
	}
	return bc, store
}

// restartCycle 模拟「节点停止 → 重启」：关闭当前写句柄后重新打开并回放。
// store 可为 nil（表示调用方已关闭）。
func restartCycle(t *testing.T, dir string, store *storage.FileBlockStore) (*blockchain.Blockchain, *storage.FileBlockStore) {
	t.Helper()
	if store != nil {
		if err := store.Close(); err != nil {
			t.Fatalf("关闭存储失败: %v", err)
		}
	}
	return openChain(t, dir)
}

// ---- TEST A / TEST B / TEST D ----

// TestRestartDoesNotDuplicatePersistedBlocks 覆盖：
//
//	TEST A —— Mine → Stop → Restart，Height / TipHash / 区块数都不变
//	TEST B —— 重启前后 blocks.dat 大小必须完全一致
//	TEST D —— 持久化记录数 == 链高 + 1（Genesis + Block#1 == 2，而不是 3）
func TestRestartDoesNotDuplicatePersistedBlocks(t *testing.T) {
	dir := t.TempDir()
	miner := newTestWallet(t)

	bc, store := openChain(t, dir)
	if bc.Height() != 0 {
		t.Fatalf("空库应只有创世区块，实际高度 = %d", bc.Height())
	}
	mineBlock(t, bc, miner) // 运行时路径：必须真实落盘

	heightBefore := bc.Height()
	tipBefore := mustTip(t, bc).Header.Hash()
	if err := store.Close(); err != nil {
		t.Fatalf("关闭存储失败: %v", err)
	}

	sizeBefore := datSize(t, dir)
	countBefore := persistedCount(t, dir)
	if countBefore != heightBefore+1 {
		t.Fatalf("首次启动后持久化记录数 = %d, want %d（链高+1）", countBefore, heightBefore+1)
	}

	// —— 重启 #1 ——
	bc2, store2 := restartCycle(t, dir, nil)
	if got := bc2.Height(); got != heightBefore {
		t.Fatalf("重启后高度 = %d, want %d", got, heightBefore)
	}
	if got := mustTip(t, bc2).Header.Hash(); got != tipBefore {
		t.Fatalf("重启后链尾哈希 = %s, want %s", got, tipBefore)
	}
	if err := store2.Close(); err != nil {
		t.Fatalf("关闭存储失败: %v", err)
	}

	if got := datSize(t, dir); got != sizeBefore {
		t.Fatalf("重启后 blocks.dat 大小 = %d, want %d：回放不得写回存储", got, sizeBefore)
	}
	if got := persistedCount(t, dir); got != countBefore {
		t.Fatalf("重启后持久化记录数 = %d, want %d：历史区块被重复落盘", got, countBefore)
	}
}

// ---- TEST C ----

// TestSecondRestartSucceeds 覆盖 TEST C：Start → Mine → Stop → Restart → Stop → Restart，
// 最终必须成功启动，且 Height / TipHash 与首次一致。
// 修复前的失败点正是这一步（第二次重启 EXIT=1）。
func TestSecondRestartSucceeds(t *testing.T) {
	dir := t.TempDir()
	miner := newTestWallet(t)

	bc, store := openChain(t, dir)
	mineBlock(t, bc, miner)
	heightBefore := bc.Height()
	tipBefore := mustTip(t, bc).Header.Hash()

	sizeAfterMine := func() int64 {
		t.Helper()
		if err := store.Close(); err != nil {
			t.Fatalf("关闭存储失败: %v", err)
		}
		return datSize(t, dir)
	}()
	countAfterMine := persistedCount(t, dir)
	if countAfterMine != heightBefore+1 {
		t.Fatalf("持久化记录数 = %d, want %d", countAfterMine, heightBefore+1)
	}

	// —— 重启 #1 ——
	bc2, store2 := restartCycle(t, dir, nil)
	if got := mustTip(t, bc2).Header.Hash(); got != tipBefore {
		t.Fatalf("重启 #1 后链尾哈希 = %s, want %s", got, tipBefore)
	}
	// —— 重启 #2（修复前必崩） ——
	bc3, store3 := restartCycle(t, dir, store2)
	defer store3.Close()

	if got := bc3.Height(); got != heightBefore {
		t.Fatalf("重启 #2 后高度 = %d, want %d", got, heightBefore)
	}
	if got := mustTip(t, bc3).Header.Hash(); got != tipBefore {
		t.Fatalf("重启 #2 后链尾哈希 = %s, want %s", got, tipBefore)
	}
	if got := datSize(t, dir); got != sizeAfterMine {
		t.Fatalf("两次重启后 blocks.dat 大小 = %d, want %d", got, sizeAfterMine)
	}
}

// ---- TEST E ----

// TestRuntimeBlockIsStillPersisted 覆盖 TEST E：不能通过「完全禁止 SaveBlock」伪修复。
// 必须证明运行时新区块（含重启之后新挖的区块）仍然真实写入磁盘。
func TestRuntimeBlockIsStillPersisted(t *testing.T) {
	dir := t.TempDir()
	miner := newTestWallet(t)

	bc, store := openChain(t, dir)
	b1 := mineBlock(t, bc, miner)
	if err := store.Close(); err != nil {
		t.Fatalf("关闭存储失败: %v", err)
	}

	// Block #1 必须真实存在于磁盘
	s, err := storage.OpenFileBlockStoreReadOnly(dir)
	if err != nil {
		t.Fatalf("只读打开存储失败: %v", err)
	}
	h, err := s.Height()
	if err != nil {
		s.Close()
		t.Fatalf("读取存储高度失败: %v", err)
	}
	if h != 1 {
		s.Close()
		t.Fatalf("磁盘高度 = %d, want 1", h)
	}
	stored, err := s.GetBlockByHeight(1)
	if err != nil {
		s.Close()
		t.Fatalf("读取磁盘区块 #1 失败: %v", err)
	}
	if stored.Header.Hash() != b1.Header.Hash() {
		s.Close()
		t.Fatalf("磁盘区块 #1 哈希 = %s, want %s", stored.Header.HashHex(), b1.Header.HashHex())
	}
	s.Close()

	// 重启后继续挖矿：新区块仍必须落盘（文件大小增长、高度推进）
	sizeBefore := datSize(t, dir)
	bc2, store2 := restartCycle(t, dir, nil)
	b2 := mineBlock(t, bc2, miner)
	sizeAfter := datSize(t, dir)
	if err := store2.Close(); err != nil {
		t.Fatalf("关闭存储失败: %v", err)
	}

	if sizeAfter <= sizeBefore {
		t.Fatalf("重启后新挖区块未落盘：blocks.dat 大小 %d 未增长（before=%d）", sizeAfter, sizeBefore)
	}
	if got := persistedCount(t, dir); got != 3 {
		t.Fatalf("持久化记录数 = %d, want 3（Genesis + #1 + #2）", got)
	}
	if got := mustTip(t, bc2).Header.Hash(); got != b2.Header.Hash() {
		t.Fatalf("链尾哈希 = %s, want %s", got, b2.Header.HashHex())
	}
}

// ---- §12 NEGATIVE / CORRUPTION SAFETY ----

// TestDuplicatePersistedRecordIsRejected 确认「含重复记录的已损坏存储」不会被
// 静默加载成一条有效链：必须明确报错。（这正是旧版本写脏的数据形态。）
func TestDuplicatePersistedRecordIsRejected(t *testing.T) {
	dir := t.TempDir()
	miner := newTestWallet(t)

	bc, store := openChain(t, dir)
	b := mineBlock(t, bc, miner)
	// 人为制造旧 bug 的存储形态：同一个区块被追加第二次
	if err := store.SaveBlock(b); err != nil {
		t.Fatalf("追加重复记录失败: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("关闭存储失败: %v", err)
	}
	if got := persistedCount(t, dir); got != 3 {
		t.Fatalf("构造后的持久化记录数 = %d, want 3", got)
	}

	s, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	defer s.Close()
	if _, err := blockchain.NewBlockchainFromStore(s); err == nil {
		t.Fatal("含重复记录的存储被静默接受：损坏数据不得被转换成有效链")
	} else {
		t.Logf("已按预期拒绝加载重复记录: %v", err)
	}
}
