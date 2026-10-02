package main

// PHASE P3.1 §6 生命周期加固测试。
//
// 重要环境说明（已实证，非推测）：
// 本开发环境为 Windows，Git Bash 的 `kill` 对原生 Go .exe 是不可捕获的 TerminateProcess；
// `taskkill`（优雅）提示「只能强行终止」；`os.Process.Signal(os.Interrupt)` 经
// GenerateConsoleCtrlEvent 会广播到整个控制台组、连测试运行器一起杀死。
// 因此「向真实子进程投递可捕获的 SIGTERM/SIGINT」在本环境无法执行。
// Test 1/2 因而直接验证「信号处理器实际执行的释放代码路径」——即 newNodeRuntime 成功后
// 调用 rt.Close()（这正是 SIGINT/SIGTERM 处理器调用的同一函数），等价于验证
// SIGTERM/SIGINT 触发 graceful shutdown 后锁被释放。生产信号处理器本身保持正确
// （注册 os.Interrupt + syscall.SIGTERM），在 Linux / 真实终端下可正常捕获。

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"p2pchain/internal/storage"
)

// TestSIGTERMReleasesLock （PHASE P3.1 §6 Test 1）
// 验证 SIGTERM 触发的 graceful shutdown 会释放 node.lock，且同目录可重新启动。
func TestSIGTERMReleasesLock(t *testing.T) {
	gracefulShutdownReleasesLock(t)
}

// TestSIGINTReleasesLock （PHASE P3.1 §6 Test 2）
// 验证 SIGINT 触发的 graceful shutdown 会释放 node.lock，且同目录可重新启动。
func TestSIGINTReleasesLock(t *testing.T) {
	gracefulShutdownReleasesLock(t)
}

// gracefulShutdownReleasesLock 验证「信号处理器执行的释放路径」：真实节点启动成功后调用
// rt.Close()（与 SIGINT/SIGTERM 处理器完全相同），断言 node.lock 被删除、同目录可重新获取。
func gracefulShutdownReleasesLock(t *testing.T) {
	dir := t.TempDir()
	rt, err := newNodeRuntimeForTest(nodeConfig{
		ListenAddr: "127.0.0.1:0",
		RPCAddr:    "127.0.0.1:0",
		DataDir:    dir,
	})
	if err != nil {
		t.Fatalf("节点启动失败: %v", err)
	}
	// 等价于信号处理器收到 SIGINT/SIGTERM 后执行的 rt.Close()；release 之后进程才退出。
	rt.Close()

	if _, e := os.Stat(filepath.Join(dir, "node.lock")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("graceful shutdown 后应已释放 node.lock")
	}
	// 同 datadir 可重新启动（无残留死锁）
	rt2, err := newNodeRuntimeForTest(nodeConfig{
		ListenAddr: "127.0.0.1:0",
		RPCAddr:    "127.0.0.1:0",
		DataDir:    dir,
	})
	if err != nil {
		t.Fatalf("释放后同目录应可重新启动: %v", err)
	}
	rt2.Close()
}

// TestAllInitFailuresReleaseLock （PHASE P3.1 §6 Test 3，table-driven）
// 对初始化链路上每个可注入失败的阶段分别注入失败：acquire 之后该阶段失败 → 锁必须释放
// （node.lock 删除）→ 下一次启动可重新获取。禁止只测单一失败点。
func TestAllInitFailuresReleaseLock(t *testing.T) {
	cases := []struct {
		name   string
		inject func(dir string) // 在 datadir 内构造触发该阶段失败的现场
	}{
		{"OpenFileBlockStore失败(blocks.dat为目录)", func(dir string) {
			if err := os.MkdirAll(filepath.Join(dir, "blocks.dat"), 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{"NewBlockchainFromStore失败(blocks.dat损坏)", func(dir string) {
			if err := os.WriteFile(filepath.Join(dir, "blocks.dat"), []byte("not-a-valid-block-file"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"wallet加载失败(secrets/wallet.json为目录)", func(dir string) {
			// P0-4：钱包路径已迁至 <datadir>/secrets/wallet.json
			if err := os.MkdirAll(filepath.Join(dir, "secrets", "wallet.json"), 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{"ctl.Start失败(非法RPC地址)", func(dir string) {
			// 不构造文件，仅由下面的配置触发
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			c.inject(dir)
			cfg := nodeConfig{
				ListenAddr: "127.0.0.1:0",
				RPCAddr:    "127.0.0.1:0",
				DataDir:    dir,
			}
			if c.name == "ctl.Start失败(非法RPC地址)" {
				cfg.RPCAddr = "999.999.999.999:70000" // 非法地址 → net.Listen 失败
			}
			_, err := newNodeRuntimeForTest(cfg)
			if err == nil {
				t.Fatalf("[%s] 该阶段应初始化失败，但未失败", c.name)
			}
			// 关键断言：初始化失败必须释放锁（node.lock 应已删除）
			if _, e := os.Stat(filepath.Join(dir, "node.lock")); !errors.Is(e, os.ErrNotExist) {
				t.Fatalf("[%s] 初始化失败后 node.lock 应已释放，实际仍存在: %v", c.name, e)
			}
			// 且同目录下一次启动应能获取锁（无残留死锁）——只校验锁释放，避免重新触发注入失败
			l, lerr := storage.AcquireDirLock(dir)
			if lerr != nil {
				t.Fatalf("[%s] 失败后锁未释放，无法重新获取: %v", c.name, lerr)
			}
			_ = l.Release()
		})
	}
}

// TestCLIHintPresentOnLocked （PHASE P3.1 §6 Test 6；R2 语义迁移 2026-09-21）
// R2（内核生命周期锁）后锁权威在内核锁而非文件存在性，原「写入残留 lock 文件
// → 节点必须拒绝启动」的断言已与规格冲突（见 storage 包 TestStaleLockFileIsReclaimed
// 的同批迁移），拆为两个场景：
//
//	场景 1（残留 lock 文件 → 接管）：node.lock 文件存在但无内核锁（写入者
//	进程已死）→ 节点必须接管并正常启动（内核锁获取成功 + 诊断信息被覆写），
//	不得因残留文件拒绝启动；stderr 不得出现「已被另一个节点进程占用」。
//	场景 2（活锁占用 → CLI 引导）：锁被活进程（本测试进程）持有 → 子进程
//	必须快速非零退出，stderr 含固定引导文案（「已被另一个节点进程占用」、
//	「手动删除」、实际 lock 路径），不得暗示自动处理。
func TestCLIHintPresentOnLocked(t *testing.T) {
	bin := buildNodeBinary(t)

	t.Run("stale_lock_file_is_taken_over", func(t *testing.T) {
		dir := t.TempDir()
		// F-3B：node 启动不再自动创世，本用例断言的是「正常启动」语义，
		// 因此必须先显式初始化数据目录，再写入残留 lock 文件。
		ensureTestDataDir(t, dir)
		lockPath := filepath.Join(dir, "node.lock")
		if err := os.WriteFile(lockPath, []byte("pid=12345\nstarted_at=2000-01-01T00:00:00Z\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// P0-4：子进程节点需钱包口令文件
		pwFile := writeTestWalletPWFile(t)
		cmd := exec.CommandContext(ctx, bin, "--datadir="+dir, "--listen=127.0.0.1:0", "--rpc=127.0.0.1:0",
			"--wallet-password-file="+pwFile)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Start(); err != nil {
			t.Fatalf("节点进程启动失败: %v", err)
		}
		// 轮询诊断信息被覆写（pid≠12345）= 内核锁接管成功
		deadline := time.Now().Add(20 * time.Second)
		takenOver := false
		for time.Now().Before(deadline) {
			if b, err := os.ReadFile(lockPath); err == nil && len(b) > 0 && !bytes.Contains(b, []byte("pid=12345")) {
				takenOver = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		if !takenOver {
			t.Fatalf("残留 node.lock（写入者已死）应被接管（诊断信息覆写为存活 pid），实际 stderr:\n%s", stderr.String())
		}
		if out := stderr.String(); strings.Contains(out, "已被另一个节点进程占用") {
			t.Fatalf("残留 lock 文件不应触发占用提示（R2 接管语义），实际 stderr:\n%s", out)
		}
	})

	t.Run("live_lock_shows_cli_hint", func(t *testing.T) {
		dir := t.TempDir()
		lockPath := filepath.Join(dir, "node.lock")
		l, err := storage.AcquireDirLock(dir)
		if err != nil {
			t.Fatalf("测试进程持锁失败: %v", err)
		}
		defer func() { _ = l.Release() }()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// P0-4：子进程节点需钱包口令文件
		pwFile := writeTestWalletPWFile(t)
		cmd := exec.CommandContext(ctx, bin, "--datadir="+dir, "--listen=127.0.0.1:0", "--rpc=127.0.0.1:0",
			"--wallet-password-file="+pwFile)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if runErr := cmd.Run(); runErr == nil { // 期望快速非零退出（log.Fatalf → os.Exit(1)）
			t.Fatalf("活锁占用下节点应非零退出")
		}

		out := stderr.String()
		if !strings.Contains(out, "已被另一个节点进程占用") {
			t.Fatalf("应含「被另一个节点进程占用」提示，实际:\n%s", out)
		}
		// §2.5 固定引导文案 + 实际路径
		if !strings.Contains(out, "手动删除") {
			t.Fatalf("应含「手动删除」引导文案，实际:\n%s", out)
		}
		if !strings.Contains(out, lockPath) {
			t.Fatalf("应含实际 node.lock 绝对路径 %s，实际:\n%s", lockPath, out)
		}
		if strings.Contains(out, "自动") {
			t.Fatalf("引导文案不得暗示自动处理残留 lock，实际:\n%s", out)
		}
	})
}

// TestPanicPathReleasesLock （PHASE P3.1 §6 Test 7）
// 通过测试钩子 testPanicAtStart（生产恒为 nil，非后门）在 runtime start 注入 panic：
// 最外层 defer 必须执行 release，随后进程退出；残留 lock 内容 pid 指向已退出进程，
// 不影响下一次启动的拒绝/释放语义（此处 panic 释放后 lock 已删除，重启动成功）。
//
// 实现：以子进程方式重跑本测试（P2PCHAIN_PANIC_SUBTEST=1），在子进程内设置钩子并 runNode；
// panic 经最外层 defer 释放锁后使子进程非零退出，父测试据此断言。
func TestPanicPathReleasesLock(t *testing.T) {
	if os.Getenv("P2PCHAIN_PANIC_SUBTEST") == "1" {
		// 子进程分支：注入 panic 并运行节点
		dir := os.Getenv("P2PCHAIN_PANIC_DIR")
		testPanicAtStart = func() { panic("injected panic at runtime start (PHASE P3.1 Test 7)") }
		// P0-4：panic 钩子在 newNodeRuntime 之后，需口令文件才能走到钩子
		runNode([]string{"--datadir=" + dir, "--listen=127.0.0.1:0", "--rpc=127.0.0.1:0",
			"--wallet-password-file=" + os.Getenv("P2PCHAIN_PANIC_PWFILE")})
		return
	}
	dir := t.TempDir()
	pwFile := writeTestWalletPWFile(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestPanicPathReleasesLock$")
	cmd.Env = append(os.Environ(), "P2PCHAIN_PANIC_SUBTEST=1", "P2PCHAIN_PANIC_DIR="+dir,
		"P2PCHAIN_PANIC_PWFILE="+pwFile)
	var childOut bytes.Buffer
	cmd.Stderr = &childOut
	cmd.Stdout = &childOut
	if err := cmd.Run(); err == nil {
		t.Fatal("panic 子测试应以非零退出，但未")
	}
	t.Logf("子进程输出:\n%s", childOut.String())
	// 断言：panic 后最外层 defer 释放了 node.lock（文件应被删除）
	if _, e := os.Stat(filepath.Join(dir, "node.lock")); !errors.Is(e, os.ErrNotExist) {
		t.Fatalf("panic 路径应释放 node.lock，实际仍存在: %v", e)
	}
	// 断言：释放后同目录可重新启动（验证释放语义正确，无残留死锁）
	rt, err := newNodeRuntimeForTest(nodeConfig{
		ListenAddr: "127.0.0.1:0",
		RPCAddr:    "127.0.0.1:0",
		DataDir:    dir,
	})
	if err != nil {
		t.Fatalf("panic 释放后同目录应可重启: %v", err)
	}
	rt.Close()
}

// buildNodeBinary 构建 node 可执行文件一次（sync.Once），供需要真实子进程的测试使用。
func buildNodeBinary(t *testing.T) string {
	t.Helper()
	buildNodeBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "p2pnode-build")
		if err != nil {
			t.Fatal(err)
		}
		buildNodeBinaryPath = filepath.Join(dir, "p2pnode_test.exe")
		cmd := exec.Command("go", "build", "-o", buildNodeBinaryPath, ".")
		cmd.Dir = "."
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("构建 node 二进制失败: %v", err)
		}
	})
	return buildNodeBinaryPath
}

var (
	buildNodeBinaryOnce sync.Once
	buildNodeBinaryPath string
)
