package storage_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
	"p2pchain/internal/transaction"
)

func makeBlock(t *testing.T, prev [32]byte, height int) *block.Block {
	t.Helper()
	cb := transaction.NewCoinbaseTx([20]byte{0x01}, 50, height)
	b := block.NewCandidateBlock(prev, pow.MaxTargetBits, []*transaction.Transaction{cb})
	if found, _ := pow.Mine(b, 0); !found {
		t.Fatal("挖矿失败")
	}
	return b
}

// TestFileStoreSaveLoad 保存后重新打开，索引与内容完整恢复。
func TestFileStoreSaveLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}

	if h, _ := store.Height(); h != -1 {
		t.Fatalf("空库高度 = %d, want -1", h)
	}

	genesis := makeBlock(t, [32]byte{}, 0)
	next := makeBlock(t, genesis.Header.Hash(), 1)
	if err := store.SaveBlock(genesis); err != nil {
		t.Fatalf("保存创世失败: %v", err)
	}
	if err := store.SaveBlock(next); err != nil {
		t.Fatalf("保存区块 1 失败: %v", err)
	}
	if h, _ := store.Height(); h != 1 {
		t.Fatalf("高度 = %d, want 1", h)
	}
	// 未落盘的父哈希必须被拒绝
	bogus := makeBlock(t, [32]byte{0x99}, 2)
	if err := store.SaveBlock(bogus); err == nil {
		t.Fatal("未知父区块被接受")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}

	// 重新打开：索引重建
	store2, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("重开存储失败: %v", err)
	}
	defer store2.Close()
	if h, _ := store2.Height(); h != 1 {
		t.Fatalf("重开后高度 = %d, want 1", h)
	}
	got, err := store2.GetBlockByHeight(1)
	if err != nil {
		t.Fatalf("按高度读取失败: %v", err)
	}
	if got.Header.Hash() != next.Header.Hash() {
		t.Fatal("重开后区块内容不一致")
	}
	byHash, err := store2.GetBlockByHash(genesis.Header.Hash())
	if err != nil {
		t.Fatalf("按哈希读取失败: %v", err)
	}
	if byHash.Header.Hash() != genesis.Header.Hash() {
		t.Fatal("按哈希读取内容不一致")
	}
	if _, err := store2.GetBlockByHeight(5); err != storage.ErrNotFound {
		t.Fatalf("越界读取应为 ErrNotFound, got %v", err)
	}
}

// TestFileStoreDetectsCorruption 截断的数据文件必须被识别为损坏。
func TestFileStoreDetectsCorruption(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveBlock(makeBlock(t, [32]byte{}, 0)); err != nil {
		t.Fatal(err)
	}
	store.Close()

	// 截断文件尾部 3 字节
	path := filepath.Join(dir, "blocks.dat")
	info, _ := os.Stat(path)
	if err := os.Truncate(path, info.Size()-3); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.OpenFileBlockStore(dir); err == nil {
		t.Fatal("损坏的数据文件未被拒绝")
	}
}

// TestFileStoreReadOnly 只读打开：可正常读取，但写入被明确拒绝；
// 且在可写存储仍持有句柄（模拟节点运行中）时也能打开——离线 printchain 依赖这一点。
func TestFileStoreReadOnly(t *testing.T) {
	dir := t.TempDir()
	rw, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()

	b0 := makeBlock(t, [32]byte{}, 0)
	if err := rw.SaveBlock(b0); err != nil {
		t.Fatal(err)
	}

	// 可写句柄未关闭的情况下只读打开
	ro, err := storage.OpenFileBlockStoreReadOnly(dir)
	if err != nil {
		t.Fatalf("只读打开失败: %v", err)
	}
	defer ro.Close()

	h, err := ro.Height()
	if err != nil {
		t.Fatal(err)
	}
	if h != 0 {
		t.Fatalf("只读高度 = %d, want 0", h)
	}
	got, err := ro.GetBlockByHeight(0)
	if err != nil {
		t.Fatalf("只读取块失败: %v", err)
	}
	if got.Header.Hash() != b0.Header.Hash() {
		t.Fatal("只读取到的区块与写入的不一致")
	}
	if _, err := ro.GetBlockByHash(b0.Header.Hash()); err != nil {
		t.Fatalf("只读按哈希取块失败: %v", err)
	}

	// 写入必须被拒绝，且不能产生部分写入
	if err := ro.SaveBlock(makeBlock(t, b0.Header.Hash(), 1)); !errors.Is(err, storage.ErrReadOnlyStore) {
		t.Fatalf("只读存储写入未返回 ErrReadOnlyStore，实际: %v", err)
	}
	if h2, _ := ro.Height(); h2 != 0 {
		t.Fatalf("只读存储写入后高度变了: %d", h2)
	}

	// 文件不存在时只读打开应报错（不会静默创建空库）
	if _, err := storage.OpenFileBlockStoreReadOnly(filepath.Join(dir, "nope")); err == nil {
		t.Fatal("对不存在的目录只读打开应报错")
	}
}
