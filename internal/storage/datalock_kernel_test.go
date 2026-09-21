package storage_test

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"p2pchain/internal/storage"
)

// 本文件是 R2（SEC-CLOSE MUST FIX 3）内核生命周期锁的判别性测试。
//
// 与既有 datalock_test.go 的差异：那些测试验证同进程内的 API 行为（互斥、
// 幂等、Release 语义），本文件用真实子进程验证跨进程内核锁的核心性质——
// **持有进程被强杀（TerminateProcess / SIGKILL，绝不走 Release）后：
// ① 锁文件残留且内容仍指向死者；② 其他进程立刻可接管（内核锁已由 OS
// 释放）；③ 接管者复写诊断信息；④ 接管后互斥仍然成立**。
// 旧实现（O_CREATE|O_EXCL 文件存在性锁）在 ② 处必然失败（残留文件挡死重启，
// 须人工删锁）——这是本修复的判别性所在。

// helperHoldLockProc 以 helper 模式运行当前测试二进制：获取 dir 的数据目录锁
// 后打印 HELPER-READY，然后永久阻塞，等待父测试强杀（绝不优雅退出、绝不 Release）。
func helperHoldLockProc(t *testing.T, dir string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperHoldLockProcess$", "-test.v")
	cmd.Env = append(os.Environ(),
		"P2P_DATALOCK_HELPER=1",
		"P2P_DATALOCK_HELPER_DIR="+dir,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// 等待 helper 完成持锁（读到 HELPER-READY；EOF = 进程异常退出）。
	ready := false
	r := bufio.NewReader(stdout)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		line, rerr := r.ReadString('\n')
		if rerr != nil {
			break // EOF：helper 未就绪即退出
		}
		if strings.Contains(line, "HELPER-READY") {
			ready = true
			break
		}
	}
	if !ready {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal("helper 未能完成持锁（未读到 HELPER-READY）")
	}
	return cmd
}

// TestHelperHoldLockProcess 仅供 helperHoldLockProc 以子进程方式运行；
// 在常规测试进程中因缺少 P2P_DATALOCK_HELPER 环境变量而被 Skip。
func TestHelperHoldLockProcess(t *testing.T) {
	if os.Getenv("P2P_DATALOCK_HELPER") != "1" {
		t.Skip("仅作为 datalock 内核锁测试的 helper 子进程运行")
	}
	dir := os.Getenv("P2P_DATALOCK_HELPER_DIR")
	lock, err := storage.AcquireDirLock(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "HELPER-ACQUIRE-ERR: %v\n", err)
		os.Exit(3)
	}
	defer lock.Release() // 不会被走到：父进程强杀绕过一切 defer
	fmt.Println("HELPER-READY")
	os.Stdout.Sync()
	time.Sleep(10 * time.Minute) // 阻塞等待父进程强杀（不触发 deadlock 检测）
}

// assertLockContentPID 断言 path 处 node.lock 存在且 pid= 字段等于 wantPID。
func assertLockContentPID(t *testing.T, path string, wantPID int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("node.lock 应存在: %v", err)
	}
	s := string(data)
	if !strings.HasPrefix(s, "pid=") {
		t.Fatalf("node.lock 缺少 pid= 前缀: %q", s)
	}
	line := s[len("pid="):]
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	got, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("node.lock 的 pid 无法解析: %q", s)
	}
	if got != wantPID {
		t.Fatalf("node.lock 的 pid=%d, want %d", got, wantPID)
	}
}

// TestKernelLockMutualExclusionWhileHolderAlive 活持有者在位时，其他进程获取
// 必须被拒（ErrDatadirLocked），且持有者的锁文件内容不被破坏。
func TestKernelLockMutualExclusionWhileHolderAlive(t *testing.T) {
	dir := t.TempDir()
	cmd := helperHoldLockProc(t, dir)
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	_, err := storage.AcquireDirLock(dir)
	if !errors.Is(err, storage.ErrDatadirLocked) {
		t.Fatalf("活持有者在位时获取应被拒为 ErrDatadirLocked，实际: %v", err)
	}
	// 拒绝路径不得破坏持有者的锁文件
	assertLockContentPID(t, filepath.Join(dir, "node.lock"), cmd.Process.Pid)
}

// TestKernelLockReleasedOnForcedKill 判别性核心：强杀持有进程后——
// 锁文件残留（pid=死者）→ 无需人工清锁即可重新获取 → 接管者复写内容 →
// 接管后互斥恢复。旧实现在「重新获取」处必然失败（残留文件挡死启动）。
func TestKernelLockReleasedOnForcedKill(t *testing.T) {
	dir := t.TempDir()
	cmd := helperHoldLockProc(t, dir)
	helperPID := cmd.Process.Pid

	// 强杀：Windows = TerminateProcess，Unix = SIGKILL——绝不走 Release/defer。
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("强杀 helper 失败: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		// 预期非零退出（被杀），仅确认回收
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("回收 helper 失败: %v", err)
		}
	}

	// ① 强杀残留：文件存在且内容仍指向死者
	assertLockContentPID(t, filepath.Join(dir, "node.lock"), helperPID)

	// ② 判别性核心：残留锁文件不再是屏障——内核锁已由 OS 随进程死亡释放
	lock2, err := storage.AcquireDirLock(dir)
	if err != nil {
		t.Fatalf("强杀后重启被拒（内核生命周期锁未生效，残留文件挡死启动）: %v", err)
	}
	defer lock2.Release()

	// ③ 接管者复写诊断信息
	assertLockContentPID(t, filepath.Join(dir, "node.lock"), os.Getpid())

	// ④ 接管后互斥恢复：新的第三方获取仍必须被拒
	if _, err := storage.AcquireDirLock(dir); !errors.Is(err, storage.ErrDatadirLocked) {
		t.Fatalf("接管后第二次获取应被拒为 ErrDatadirLocked，实际: %v", err)
	}
}

// TestForcedKillThenReacquireSameProcess 强杀 helper 后，同一进程内先拒绝、
// 后接管的顺序敏感性：确保「拒绝」分支（helper 活着）与「接管」分支（死亡后）
// 在同一测试进程内先后成立，排除一次性通过的状态污染。
func TestForcedKillThenReacquireSameProcess(t *testing.T) {
	dir := t.TempDir()
	cmd := helperHoldLockProc(t, dir)

	// 活着：拒绝
	if _, err := storage.AcquireDirLock(dir); !errors.Is(err, storage.ErrDatadirLocked) {
		t.Fatalf("持有者存活时应被拒，实际: %v", err)
	}
	// 强杀：内核释放
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("回收 helper 失败: %v", err)
		}
	}
	// 死亡：接管
	lock, err := storage.AcquireDirLock(dir)
	if err != nil {
		t.Fatalf("强杀后应可接管，实际: %v", err)
	}
	defer lock.Release()
	// 正常 Release 后文件删除（同进程闭环）
	if err := lock.Release(); err != nil {
		t.Fatalf("接管后释放失败: %v", err)
	}
	if _, e := os.Stat(filepath.Join(dir, "node.lock")); !errors.Is(e, os.ErrNotExist) {
		t.Fatalf("释放后 node.lock 应被删除: %v", e)
	}
}
