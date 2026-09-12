package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestReleaseIsIdempotent （PHASE P3.1 §6 Test 4）
// 验证 Release 幂等：重复调用不报错；且 pid 校验生效——
// 不匹配本进程 pid 的 lock 不会被误删（防御「本进程崩溃后他人重新获取同名锁」）。
func TestReleaseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	lock, err := AcquireDirLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("首次释放失败: %v", err)
	}
	// 第二次释放：必须幂等、无错、不 panic
	if err := lock.Release(); err != nil {
		t.Fatalf("二次释放应幂等无错，实际: %v", err)
	}
	if _, e := os.Stat(filepath.Join(dir, "node.lock")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("释放后 node.lock 应已被删除")
	}
}

// TestReleaseSkipsForeignLockByPID （§2.2 pid 校验）
// 模拟「另一进程持有同名 lock」：文件内容为不同 pid，且本 DirLock 持有一个打开句柄。
// Release 必须在 pid 不匹配时静默跳过删除，绝误删他人锁。
func TestReleaseSkipsForeignLockByPID(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "node.lock")
	// 写入「其他进程」的 pid（远非本进程）
	if err := os.WriteFile(p, []byte("pid=999999\nstarted_at=2000-01-01T00:00:00Z\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(p, os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	l := &DirLock{path: p, f: f}
	if err := l.Release(); err != nil {
		t.Fatalf("pid 不匹配应静默跳过且无错误，实际: %v", err)
	}
	// 关键断言：不得误删他人的锁
	if _, statErr := os.Stat(p); statErr != nil {
		t.Fatalf("pid 不匹配时不应删除 node.lock（误删他人锁）: %v", statErr)
	}
}

// TestReleaseDeletesOwnLockByPID （§2.2 pid 校验的正向）
// 文件内容 pid == 本进程 pid 时，Release 必须删除 lock。
func TestReleaseDeletesOwnLockByPID(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "node.lock")
	content := fmt.Sprintf("pid=%d\nstarted_at=2000-01-01T00:00:00Z\n", os.Getpid())
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(p, os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	l := &DirLock{path: p, f: f}
	if err := l.Release(); err != nil {
		t.Fatalf("pid 匹配释放失败: %v", err)
	}
	if _, statErr := os.Stat(p); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("pid 匹配时应删除 node.lock")
	}
}

// TestEmptyCorruptedLockContentTolerated （PHASE P3.1 §6 Test 5）
// 预置 node.lock 内容为空 / 仅 pid 字段 / 乱码三种情况：启动必须稳定返回
// ErrDatadirLocked，不得 panic，不得误报为回放/其他错误类型。
func TestEmptyCorruptedLockContentTolerated(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"空内容", ""},
		{"仅 pid 字段", "pid=4242\n"},
		{"乱码", "this is not a valid lock file\x00garbage"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "node.lock")
			if err := os.WriteFile(p, []byte(c.content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := AcquireDirLock(dir)
			if err == nil {
				t.Fatal("空/损坏 lock 内容时应拒绝启动")
			}
			if !errors.Is(err, ErrDatadirLocked) {
				t.Fatalf("应统一识别为 ErrDatadirLocked，实际: %v", err)
			}
		})
	}
}
