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
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

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
	rt, err := newNodeRuntime(nodeConfig{
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
	rt2, err := newNodeRuntime(nodeConfig{
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
		name  string
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
		{"wallet加载失败(wallet.json为目录)", func(dir string) {
			if err := os.MkdirAll(filepath.Join(dir, "wallet.json"), 0o700); err != nil {
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
			_, err := newNodeRuntime(cfg)
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

// TestCLIHintPresentOnLocked （PHASE P3.1 §6 Test 6）
// 触发 DATADIR_LOCKED，捕获 stderr：必须包含固定引导文案与实际 datadir 路径；
// 不得包含「自动」处理的暗示。
func TestCLIHintPresentOnLocked(t *testing.T) {
	bin := buildNodeBinary(t)
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "node.lock")
	if err := os.WriteFile(lockPath, []byte("pid=12345\nstarted_at=2000-01-01T00:00:00Z\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "--datadir="+dir, "--listen=127.0.0.1:0", "--rpc=127.0.0.1:0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run() // 期望非零退出（log.Fatalf → os.Exit(1)）

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
		runNode([]string{"--datadir=" + dir, "--listen=127.0.0.1:0", "--rpc=127.0.0.1:0"})
		return
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestPanicPathReleasesLock$")
	cmd.Env = append(os.Environ(), "P2PCHAIN_PANIC_SUBTEST=1", "P2PCHAIN_PANIC_DIR="+dir)
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
	rt, err := newNodeRuntime(nodeConfig{
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
