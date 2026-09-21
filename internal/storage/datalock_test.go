package storage_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"p2pchain/internal/storage"
)

// TestAcquireDirLockFirstSucceeds 空数据目录首次获取锁必须成功并创建 node.lock。
func TestAcquireDirLockFirstSucceeds(t *testing.T) {
	dir := t.TempDir()
	lock, err := storage.AcquireDirLock(dir)
	if err != nil {
		t.Fatalf("首次获取锁失败: %v", err)
	}
	defer lock.Release()
	if _, err := os.Stat(filepath.Join(dir, "node.lock")); err != nil {
		t.Fatalf("node.lock 未创建: %v", err)
	}
}

// TestAcquireDirLockSecondFails 持有期间第二个进程获取必须稳定失败，且识别为 ErrDatadirLocked。
func TestAcquireDirLockSecondFails(t *testing.T) {
	dir := t.TempDir()
	lock, err := storage.AcquireDirLock(dir)
	if err != nil {
		t.Fatalf("首次获取失败: %v", err)
	}
	defer lock.Release()

	_, err = storage.AcquireDirLock(dir)
	if err == nil {
		t.Fatal("第二个进程获取锁应失败，但成功了")
	}
	if !errors.Is(err, storage.ErrDatadirLocked) {
		t.Fatalf("应识别为 ErrDatadirLocked，实际: %v", err)
	}
}

// TestStaleLockFileIsReclaimed R2 语义迁移（原 TestAcquireDirLockNeverOverwritesExisting）：
// 预置 stale 残留 lock（无活持有者）——R2 内核生命周期锁下，接管与否由内核锁
// 判定（锁空闲 = 持有者已死），文件存在性不再是拒绝依据：必须成功接管，
// 并以 Truncate+覆写复写 stale 内容为本进程 pid（Release 的归属校验依赖它）。
// 「绝不覆盖」约束相应迁移为：活持有者在位时（内核锁被持有）绝不触碰文件
// （见 TestKernelLockMutualExclusionWhileHolderAlive）。
func TestStaleLockFileIsReclaimed(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "node.lock")
	known := []byte("pid=9999\nstarted_at=2000-01-01T00:00:00Z\n")
	if err := os.WriteFile(lockPath, known, 0o600); err != nil {
		t.Fatal(err)
	}

	lock, err := storage.AcquireDirLock(dir)
	if err != nil {
		t.Fatalf("stale 残留（无活持有者）应被接管，实际被拒: %v", err)
	}
	defer lock.Release()
	got, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("pid=%d\n", os.Getpid())
	if !strings.HasPrefix(string(got), want) {
		t.Fatalf("接管后内容应被复写为本进程 pid（前缀 %q），实际: %q", want, got)
	}
}

// TestDirLockReleaseAllowsReacquire acquire → release → acquire 必须成功。
func TestDirLockReleaseAllowsReacquire(t *testing.T) {
	dir := t.TempDir()
	lock, err := storage.AcquireDirLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("释放锁失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "node.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("释放后 node.lock 应被删除: %v", err)
	}
	lock2, err := storage.AcquireDirLock(dir)
	if err != nil {
		t.Fatalf("释放后重新获取失败: %v", err)
	}
	defer lock2.Release()
}

// TestDirLockReleasedOnInitFailure 模拟「获取锁后初始化失败」：调用方负责 Release，
// 之后下一次启动应当能正常获取——保证初始化失败不会留下死锁。
func TestDirLockReleasedOnInitFailure(t *testing.T) {
	dir := t.TempDir()
	lock, err := storage.AcquireDirLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	// 初始化失败路径：获取锁的调用方必须在放弃前 Release
	if err := lock.Release(); err != nil {
		t.Fatalf("失败路径释放锁失败: %v", err)
	}
	// 下一次启动应能正常获取（锁已释放，无残留）
	lock2, err := storage.AcquireDirLock(dir)
	if err != nil {
		t.Fatalf("失败后锁未释放，下次启动无法获取: %v", err)
	}
	defer lock2.Release()
}
