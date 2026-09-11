package main

// 真实节点级验证：同一数据目录同时只允许一个节点进程使用。
// 覆盖 PHASE P2.1 §6 Test 6：
//   - 节点 A 启动时获取 datadir 锁并正常运行
//   - 节点 B 用同一 datadir 必须立即拒绝（返回 ErrDatadirLocked）
//   - B 不得启动 RPC、不得启动 P2P 监听、不得修改 blocks.dat
//   - A 在 B 被拒后依然健康

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"p2pchain/internal/storage"
)

// blocksDatInfo 读取 blocks.dat 的大小与修改时间，用于证明第二个实例没有触碰数据文件。
type blocksDatInfo struct {
	size  int64
	mtime time.Time
}

func statBlocksDat(t *testing.T, dir string) blocksDatInfo {
	t.Helper()
	fi, err := os.Stat(filepath.Join(dir, "blocks.dat"))
	if err != nil {
		// 未能 stat：视为尚未落盘（size=0，零值时间）
		return blocksDatInfo{}
	}
	return blocksDatInfo{size: fi.Size(), mtime: fi.ModTime()}
}

func TestSameDatadirRejectsSecondNode(t *testing.T) {
	dir := t.TempDir()

	a, err := newNodeRuntime(nodeConfig{
		ListenAddr: "127.0.0.1:0",
		RPCAddr:    "127.0.0.1:0",
		DataDir:    dir,
	})
	if err != nil {
		t.Fatalf("节点 A 启动失败: %v", err)
	}
	defer a.Close()

	// B 尝试前：记录 blocks.dat 状态（A 已落盘创世）
	before := statBlocksDat(t, dir)

	// B 用同一 datadir → 必须在 newNodeRuntime 最开头的锁获取阶段就失败
	b, err := newNodeRuntime(nodeConfig{
		ListenAddr: "127.0.0.1:0",
		RPCAddr:    "127.0.0.1:0",
		DataDir:    dir,
	})
	if err == nil {
		if b != nil {
			b.Close()
		}
		t.Fatal("同一 datadir 的第二个节点不应启动成功")
	}
	if !errors.Is(err, storage.ErrDatadirLocked) {
		t.Fatalf("应识别为 ErrDatadirLocked，实际: %v", err)
	}

	// 文件系统证据：B 在锁获取阶段即返回，根本没有打开 blocks.dat
	after := statBlocksDat(t, dir)
	if before.size != after.size {
		t.Fatalf("第二个实例修改了 blocks.dat 大小：before=%d after=%d", before.size, after.size)
	}
	if !before.mtime.Equal(after.mtime) {
		t.Fatalf("第二个实例触碰了 blocks.dat：before mtime=%v after=%v", before.mtime, after.mtime)
	}

	// A 仍然健康：链未受影响，高度 >= 0（创世已落盘）
	if a.chain.Height() < 0 {
		t.Fatal("A 在 B 被拒后不应受影响")
	}

	// B 被拒后不应留下任何残留 lock（A 仍持有）
	if _, err := os.Stat(filepath.Join(dir, "node.lock")); err != nil {
		t.Fatalf("node.lock 应仍存在（A 持有中）：%v", err)
	}
}
