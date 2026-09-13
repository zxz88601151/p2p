package main

// PHASE PRODUCT-DEV-1C.0 §7 —— CLI 分发安全测试。
//
// 覆盖的 P1：
//
//	node -datadir X verify      过去会启动完整节点（P1）
//	node -datadir X printchain  同上
//	node -datadir X reset       同上（最危险：本该破坏性清空，实际却重建了链）
//
// 覆盖的 §6 不变量：
//
//	DISPATCH-INV-01  node -datadir X verify     一定执行 verify
//	DISPATCH-INV-02  verify 保持 READ ONLY / NO LOCK / NO WRITE / NO SERVER / NO P2P
//	DISPATCH-INV-03  node -datadir X printchain 一定执行 printchain
//	DISPATCH-INV-04  node -datadir X reset      一定执行 reset
//	DISPATCH-INV-05  错误 command 绝不静默 fallback 到 runNode()
//	DISPATCH-INV-06  现有正常启动语义不被破坏
//
// 测试手段（§7 明确要求）：真实二进制 + 真实临时目录 + 真实子进程。
// 不 mock process lifecycle，也不只测 parser 函数——因为 P1 的全部危害
// （建链 / 建钱包 / 取锁 / 监听端口 / 永久阻塞）都只发生在真实进程里。

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---- 子进程执行辅助 ----

// cmdRun 一次真实子进程执行的结果。
type cmdRun struct {
	code     int    // 进程退出码；超时时为 -1
	out      string // stdout + stderr
	timedOut bool   // 是否在 timeout 内没有退出（P1 的直接症状：永久阻塞）
}

// runNodeBin 用真实二进制执行一次命令，超过 timeout 未退出则判定为阻塞并杀掉。
func runNodeBin(t *testing.T, timeout time.Duration, args ...string) cmdRun {
	t.Helper()
	bin := buildNodeBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := buf.String()

	if ctx.Err() == context.DeadlineExceeded {
		return cmdRun{code: -1, out: out, timedOut: true}
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return cmdRun{code: exitErr.ExitCode(), out: out}
		}
		t.Fatalf("执行 %v 失败: %v\n输出:\n%s", args, err, out)
	}
	return cmdRun{code: 0, out: out}
}

// ---- 数据目录快照辅助 ----

// dirSnapshot 记录目录内每个文件的名字与内容哈希，用于断言「没有任何写入」。
func dirSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取数据目录失败: %v", err)
	}
	snap := make(map[string]string, len(entries))
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", e.Name(), err)
		}
		sum := sha256.Sum256(data)
		snap[e.Name()] = hex.EncodeToString(sum[:])
	}
	return snap
}

// assertDirUnchanged 断言目录内容与快照完全一致（DISPATCH-INV-02：NO WRITE）。
func assertDirUnchanged(t *testing.T, dir string, before map[string]string) {
	t.Helper()
	after := dirSnapshot(t, dir)
	if len(after) != len(before) {
		t.Fatalf("DISPATCH-INV-02 违反：数据目录文件数变化 %d -> %d\n之前=%v\n之后=%v",
			len(before), len(after), before, after)
	}
	for name, sum := range before {
		if after[name] != sum {
			t.Fatalf("DISPATCH-INV-02 违反：%s 内容被修改", name)
		}
	}
}

// assertNoNodeStartup 断言输出里没有节点启动的痕迹（NO SERVER / NO P2P）。
//
// 节点一旦真正启动，必然打印「控制接口已启动」与「[p2p] 节点已启动」，
// 因此这两条同时缺失即可证明既没有起 HTTP 也没有起 P2P。
func assertNoNodeStartup(t *testing.T, r cmdRun) {
	t.Helper()
	for _, marker := range []string{"控制接口已启动", "[p2p] 节点已启动", "已生成新钱包"} {
		if strings.Contains(r.out, marker) {
			t.Fatalf("DISPATCH-INV-02 违反：输出中出现节点启动痕迹 %q\n输出:\n%s", marker, r.out)
		}
	}
}

// seedChain 在给定目录里真实地建出一条链（启动节点 → 停止），
// 用于让 verify / printchain / reset 有真实数据可操作。
func seedChain(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("创建数据目录失败: %v", err)
	}
	n := startRealNode(t, dir)
	code, out := n.stopViaCLI()
	if code != 0 {
		t.Fatalf("停止节点失败（code=%d）:\n%s", code, out)
	}
	if !n.waitExit(15 * time.Second) {
		t.Fatalf("节点在 stop 之后未退出:\n%s", n.output.String())
	}
}

// ---- DISPATCH-INV-01 / 02：verify ----

// TestVerifyWithDatadirBeforeCommand 覆盖 DISPATCH-INV-01/02：
// `node -datadir X verify` 必须进入离线只读 verify，绝不启动节点。
func TestVerifyWithDatadirBeforeCommand(t *testing.T) {
	dir := t.TempDir()
	seedChain(t, dir)

	before := dirSnapshot(t, dir)
	r := runNodeBin(t, 30*time.Second, "-datadir", dir, "verify")

	if r.timedOut {
		t.Fatalf("DISPATCH-INV-01 违反：命令未退出（仍在阻塞等待，说明误入了节点启动路径）\n输出:\n%s", r.out)
	}
	if r.code != 0 {
		t.Fatalf("verify 应通过（exit 0），实际 exit=%d\n输出:\n%s", r.code, r.out)
	}
	if !strings.Contains(r.out, "[verify] 结果") {
		t.Fatalf("输出中没有 verify 结果，说明没有进入 verify 语义\n输出:\n%s", r.out)
	}
	assertNoNodeStartup(t, r)
	assertDirUnchanged(t, dir, before)
	// NO LOCK：verify 绝不能创建 node.lock
	if _, err := os.Stat(filepath.Join(dir, "node.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("DISPATCH-INV-02 违反：verify 之后出现了 node.lock（%v）", err)
	}
}

// TestVerifyWithDatadirAfterCommand 既有形式 `node verify -datadir X` 不得回归。
func TestVerifyWithDatadirAfterCommand(t *testing.T) {
	dir := t.TempDir()
	seedChain(t, dir)

	before := dirSnapshot(t, dir)
	r := runNodeBin(t, 30*time.Second, "verify", "-datadir", dir)

	if r.timedOut {
		t.Fatalf("命令未退出\n输出:\n%s", r.out)
	}
	if r.code != 0 || !strings.Contains(r.out, "[verify] 结果") {
		t.Fatalf("既有形式 verify 回归：exit=%d\n输出:\n%s", r.code, r.out)
	}
	assertNoNodeStartup(t, r)
	assertDirUnchanged(t, dir, before)
}

// TestVerifyOnEmptyDirMustNotStartNode 在**空目录**上复现 P1 的原始场景：
// 修复前这里会建链、建钱包、取锁、监听端口并永久阻塞。
func TestVerifyOnEmptyDirMustNotStartNode(t *testing.T) {
	dir := t.TempDir()

	r := runNodeBin(t, 30*time.Second, "-datadir", dir, "verify")

	if r.timedOut {
		t.Fatalf("DISPATCH-INV-01 违反：空目录上的 verify 仍在阻塞（P1 未修复）\n输出:\n%s", r.out)
	}
	// 没有链时 verify 会报「区块数据文件不存在」并退出码 1——这是既有语义（P2），
	// 本阶段不改；关键是它必须**走 verify 语义并退出**，而不是启动节点。
	if !strings.Contains(r.out, "打开区块数据失败") && !strings.Contains(r.out, "[verify]") {
		t.Fatalf("空目录上的 verify 未进入 verify 语义\n输出:\n%s", r.out)
	}
	assertNoNodeStartup(t, r)
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("DISPATCH-INV-02 违反：空目录被写入了文件 %v（err=%v）", entries, err)
	}
}

// ---- DISPATCH-INV-03：printchain ----

// TestPrintchainWithDatadirBeforeAndAfter 覆盖 DISPATCH-INV-03：
// 两种参数排列都必须执行 printchain。
func TestPrintchainWithDatadirBeforeAndAfter(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"datadir 在子命令前", []string{"-datadir", "@", "printchain"}},
		{"datadir 在子命令后", []string{"printchain", "-datadir", "@"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			seedChain(t, dir)

			before := dirSnapshot(t, dir)
			args := make([]string, len(tc.args))
			for i, a := range tc.args {
				if a == "@" {
					args[i] = dir
				} else {
					args[i] = a
				}
			}
			r := runNodeBin(t, 30*time.Second, args...)

			if r.timedOut {
				t.Fatalf("DISPATCH-INV-03 违反：命令未退出\n输出:\n%s", r.out)
			}
			if r.code != 0 || !strings.Contains(r.out, "本地区块链:") {
				t.Fatalf("未进入 printchain 语义：exit=%d\n输出:\n%s", r.code, r.out)
			}
			assertNoNodeStartup(t, r)
			assertDirUnchanged(t, dir, before)
		})
	}
}

// ---- DISPATCH-INV-04：reset ----

// TestResetWithDatadirBeforeAndAfter 覆盖 DISPATCH-INV-04：
// 两种参数排列都必须执行 reset（真实删除链与钱包）。
func TestResetWithDatadirBeforeAndAfter(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"datadir 在子命令前", []string{"-datadir", "@", "reset", "-force"}},
		{"datadir 在子命令后", []string{"reset", "-datadir", "@", "-force"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			seedChain(t, dir)
			if _, err := os.Stat(filepath.Join(dir, "blocks.dat")); err != nil {
				t.Fatalf("前置条件失败：blocks.dat 不存在: %v", err)
			}

			args := make([]string, len(tc.args))
			for i, a := range tc.args {
				if a == "@" {
					args[i] = dir
				} else {
					args[i] = a
				}
			}
			r := runNodeBin(t, 30*time.Second, args...)

			if r.timedOut {
				t.Fatalf("DISPATCH-INV-04 违反：命令未退出\n输出:\n%s", r.out)
			}
			if r.code != 0 || !strings.Contains(r.out, "[reset] 结果") {
				t.Fatalf("未进入 reset 语义：exit=%d\n输出:\n%s", r.code, r.out)
			}
			assertNoNodeStartup(t, r)
			if _, err := os.Stat(filepath.Join(dir, "blocks.dat")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("reset 之后 blocks.dat 仍存在（%v）", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "wallet.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("reset 之后 wallet.json 仍存在（%v）", err)
			}
		})
	}
}

// ---- DISPATCH-INV-05：错误 command 不得静默启动节点 ----

// TestUnknownSubcommandDoesNotStartNode 覆盖 DISPATCH-INV-05：
// 识别不了的子命令必须报错退出，绝不退化成「启动节点」。
func TestUnknownSubcommandDoesNotStartNode(t *testing.T) {
	dir := t.TempDir()

	r := runNodeBin(t, 20*time.Second, "-datadir", dir, "definitely-not-a-command")

	if r.timedOut {
		t.Fatalf("DISPATCH-INV-05 违反：未知子命令仍在阻塞（说明退化了 runNode）\n输出:\n%s", r.out)
	}
	if r.code != 2 {
		t.Fatalf("未知子命令应退出码 2，实际 %d\n输出:\n%s", r.code, r.out)
	}
	if !strings.Contains(r.out, "未知子命令") {
		t.Fatalf("应给出「未知子命令」提示\n输出:\n%s", r.out)
	}
	assertNoNodeStartup(t, r)
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("未知子命令不得产生任何状态文件，实际: %v（err=%v）", entries, err)
	}
}

// TestMeaninglessFlagComboDoesNotStartNode 无意义组合（节点专属选项 + 离线子命令）
// 必须报 usage 错误，而不是静默启动节点。
func TestMeaninglessFlagComboDoesNotStartNode(t *testing.T) {
	dir := t.TempDir()

	r := runNodeBin(t, 20*time.Second, "-datadir", dir, "-mine", "verify")

	if r.timedOut {
		t.Fatalf("DISPATCH-INV-05 违反：无意义组合仍在阻塞\n输出:\n%s", r.out)
	}
	if r.code != 2 {
		t.Fatalf("无意义组合应退出码 2（usage 错误），实际 %d\n输出:\n%s", r.code, r.out)
	}
	assertNoNodeStartup(t, r)
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("无意义组合不得产生任何状态文件，实际: %v（err=%v）", entries, err)
	}
}

// ---- DISPATCH-INV-06：正常启动语义不被破坏 ----

// TestNodeStartupUnaffected 覆盖 DISPATCH-INV-06：
// 只有选项、没有子命令时，仍然按原样启动节点。
func TestNodeStartupUnaffected(t *testing.T) {
	dir := t.TempDir()
	n := startRealNode(t, dir) // 内部即 `-datadir dir -listen … -rpc …`
	st, err := n.status()
	if err != nil {
		t.Fatalf("节点未正常启动: %v\n输出:\n%s", err, n.output.String())
	}
	if st.Height != 0 {
		t.Fatalf("新节点高度应为 0，实际 %d", st.Height)
	}
	code, out := n.stopViaCLI()
	if code != 0 {
		t.Fatalf("stop 失败（code=%d）:\n%s", code, out)
	}
	if !n.waitExit(15 * time.Second) {
		t.Fatalf("节点未退出:\n%s", n.output.String())
	}
}

// ---- splitCommandArgs 单元测试（补充，不构成主要证据）----

// TestSplitCommandArgs 断言 argv 归一化结果：参数排列不改变 command identity。
func TestSplitCommandArgs(t *testing.T) {
	cases := []struct {
		name    string
		argv    []string
		wantCmd string
		wantArg []string
	}{
		{
			name:    "子命令在首位（既有形式）",
			argv:    []string{"verify", "-datadir", "X"},
			wantCmd: "verify", wantArg: []string{"-datadir", "X"},
		},
		{
			name:    "datadir 在子命令前（P1 场景）",
			argv:    []string{"-datadir", "X", "verify"},
			wantCmd: "verify", wantArg: []string{"-datadir=X"},
		},
		{
			name:    "datadir 在子命令前且带子命令选项",
			argv:    []string{"-datadir", "X", "printchain", "-limit", "5"},
			wantCmd: "printchain", wantArg: []string{"-datadir=X", "-limit", "5"},
		},
		{
			name:    "只有选项：启动节点",
			argv:    []string{"-datadir", "X", "-mine"},
			wantCmd: "", wantArg: []string{"-datadir", "X", "-mine"},
		},
		{
			name:    "空 argv：启动节点",
			argv:    nil,
			wantCmd: "", wantArg: nil,
		},
		{
			name:    "未知子命令也要原样交给 runCLI 去报错",
			argv:    []string{"-datadir", "X", "bogus"},
			wantCmd: "bogus", wantArg: []string{"-datadir=X"},
		},
		{
			name:    "布尔选项用 name=value 形式搬运",
			argv:    []string{"-mine", "-datadir", "X", "verify"},
			wantCmd: "verify", wantArg: []string{"-datadir=X", "-mine=true"},
		},
		{
			name:    "datadir 的值恰好是子命令名时不得误判",
			argv:    []string{"-datadir", "verify"},
			wantCmd: "", wantArg: []string{"-datadir", "verify"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, args := splitCommandArgs(tc.argv)
			if cmd != tc.wantCmd {
				t.Fatalf("cmd = %q, 期望 %q", cmd, tc.wantCmd)
			}
			if strings.Join(args, " ") != strings.Join(tc.wantArg, " ") {
				t.Fatalf("args = %v, 期望 %v", args, tc.wantArg)
			}
		})
	}
}
