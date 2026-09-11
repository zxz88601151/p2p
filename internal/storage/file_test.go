package storage_test

import (
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
