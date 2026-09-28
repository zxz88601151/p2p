package main

// 本文件覆盖 PHASE PRODUCT-DEV-1A 的 `reset` 离线破坏性命令（§8）。
//
// 覆盖矩阵：
//   1  干净目录 reset（幂等）
//   2  挖矿后 reset
//   3  有交易/钱包后 reset
//   4  重启后 reset
//   5  节点已停止时 reset
//   6  节点运行中（存在 node.lock）时 reset —— 默认拒绝
//   7  带 --force 处理陈旧锁
//   8  损坏状态后 reset
//   9  连续两次 reset
//   10 reset 后重新启动（自动重建创世）
//   11 reset 后 verify
//   12 reset 后继续挖矿
//   13 reset 后创世仍然确定性
//   14 JSON 输出
//   15 退出码语义（0/1/2）与确认分支（yes / no / 非终端）
//
// 所有用例都在 t.TempDir() 内的独立数据目录运行，不触碰 ~/.p2pchain。

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"p2pchain/internal/blockchain"
	"p2pchain/internal/storage"
	"p2pchain/internal/wallet"
)

// ---- 夹具 ----

const stateFileNames = 3 // blocks.dat / wallet.json / node.lock

// seedFullState 在 dir 中建立一个「做过实验」的数据目录：链 + 钱包。
func seedFullState(t *testing.T, dir string, blocks int) {
	t.Helper()
	buildChainForVerify(t, dir, blocks)
	if _, _, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json")); err != nil {
		t.Fatalf("创建钱包失败: %v", err)
	}
}

// runReset 执行 reset 并返回退出码与输出。
func runReset(t *testing.T, dir string, extra ...string) (int, string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code := cmdReset(append([]string{"-datadir", dir}, extra...), &out, &errBuf)
	return code, out.String() + errBuf.String()
}

// runResetErr 单独返回 stdout / stderr，用于断言错误落在 stderr。
func runResetErr(t *testing.T, dir string, extra ...string) (int, string, string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code := cmdReset(append([]string{"-datadir", dir}, extra...), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

// assertClean 断言目录里三个已知状态文件全部不存在（RESET-INV-01/02/03）。
func assertClean(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"blocks.dat", "wallet.json", "node.lock"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Fatalf("reset 后 %s 仍存在", name)
		}
	}
}

// withStdin 临时替换 reset 的确认输入源，测试结束后恢复。
// asTerminal=true 时同时把终端判定打开，用于覆盖「终端 + 输入 yes / 其它」分支。
func withStdin(t *testing.T, input string, asTerminal bool) {
	t.Helper()
	oldIn, oldTerm := cliStdin, isTerminalFn
	cliStdin = strings.NewReader(input)
	if asTerminal {
		isTerminalFn = func(io.Reader) bool { return true }
	} else {
		isTerminalFn = func(io.Reader) bool { return false }
	}
	t.Cleanup(func() { cliStdin, isTerminalFn = oldIn, oldTerm })
}

// ---- 1. 干净目录 ----

func TestResetCleanDirectoryIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	code, out := runReset(t, dir, "-force")
	if code != 0 {
		t.Fatalf("干净目录 reset 应成功，exit=%d, out=%s", code, out)
	}
	if !strings.Contains(out, "已是干净状态") {
		t.Fatalf("应提示已是干净状态: %s", out)
	}
	assertClean(t, dir)
}

// ---- 2. 挖矿后 ----

func TestResetAfterMining(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 3)
	code, out := runReset(t, dir, "-force")
	if code != 0 {
		t.Fatalf("挖矿后 reset 应成功，exit=%d, out=%s", code, out)
	}
	assertClean(t, dir)
}

// ---- 3. 有交易/钱包后 ----

func TestResetAfterTransactionsAndWallet(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 2)
	// 明确确认 wallet.json 在 reset 前确实存在
	if _, err := os.Stat(filepath.Join(dir, "wallet.json")); err != nil {
		t.Fatalf("夹具未创建 wallet.json: %v", err)
	}
	code, out := runReset(t, dir, "-force")
	if code != 0 {
		t.Fatalf("exit=%d, out=%s", code, out)
	}
	assertClean(t, dir)
}

// ---- 4. 重启后 ----

func TestResetAfterRestart(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 2)
	// 模拟一次「重启」：重新打开存储 + 回放，再关闭（不产生 node.lock）
	store, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("重启打开存储失败: %v", err)
	}
	if _, err := blockchain.NewBlockchainFromStoreForTest(store); err != nil {
		t.Fatalf("重启回放失败: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	code, out := runReset(t, dir, "-force")
	if code != 0 {
		t.Fatalf("exit=%d, out=%s", code, out)
	}
	assertClean(t, dir)
}

// ---- 5. 节点已停止（无锁）----

func TestResetWhileNodeStopped(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 1)
	if _, err := os.Stat(filepath.Join(dir, "node.lock")); err == nil {
		t.Fatalf("夹具不应留下 node.lock")
	}
	code, _ := runReset(t, dir, "-force")
	if code != 0 {
		t.Fatalf("节点已停止时应可直接 reset，exit=%d", code)
	}
	assertClean(t, dir)
}

// ---- 6. 节点运行中（存在 node.lock）：默认拒绝 ----

func TestResetRefusedWhenLockExists(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 2)
	// 以真实的锁机制占用目录（等价于一个活着的节点进程）
	lock, err := storage.AcquireDirLock(dir)
	if err != nil {
		t.Fatalf("占用数据目录失败: %v", err)
	}
	defer func() { _ = lock.Release() }()

	before, _ := os.Stat(filepath.Join(dir, "blocks.dat"))
	code, _, errOut := runResetErr(t, dir) // 无 --force
	if code != 1 {
		t.Fatalf("存在锁且无 --force 时应拒绝，exit=%d, stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, "node.lock") {
		t.Fatalf("错误信息应指出 node.lock: %s", errOut)
	}
	after, err := os.Stat(filepath.Join(dir, "blocks.dat"))
	if err != nil || after.Size() != before.Size() {
		t.Fatalf("被拒绝时绝不能删除任何文件")
	}
}

// ---- 7. --force 处理陈旧锁 ----

func TestResetForceRemovesStaleLock(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 1)
	// 写一个「陈旧」锁：pid 指向一个几乎不存在的进程号，模拟崩溃残留
	if err := os.WriteFile(filepath.Join(dir, "node.lock"),
		[]byte("pid=999999\nstarted_at=2020-01-01T00:00:00Z\n"), 0o600); err != nil {
		t.Fatalf("写入陈旧锁失败: %v", err)
	}
	code, out := runReset(t, dir, "-force")
	if code != 0 {
		t.Fatalf("--force 应能清理陈旧锁，exit=%d, out=%s", code, out)
	}
	assertClean(t, dir)
}

// ---- 8. 损坏状态后 ----

func TestResetAfterCorruptedState(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 2)
	// 把链文件写成垃圾：verify 会失败，但 reset 必须仍然能清干净
	if err := os.WriteFile(filepath.Join(dir, "blocks.dat"), []byte("not a chain"), 0o600); err != nil {
		t.Fatalf("写坏链文件失败: %v", err)
	}
	code, out := runReset(t, dir, "-force")
	if code != 0 {
		t.Fatalf("损坏状态下 reset 应成功，exit=%d, out=%s", code, out)
	}
	assertClean(t, dir)
}

// ---- 9. 连续两次 reset ----

func TestResetTwiceIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 2)
	if code, out := runReset(t, dir, "-force"); code != 0 {
		t.Fatalf("第一次 reset 失败: exit=%d, out=%s", code, out)
	}
	code, out := runReset(t, dir, "-force")
	if code != 0 {
		t.Fatalf("第二次 reset 应仍成功（幂等），exit=%d, out=%s", code, out)
	}
	if !strings.Contains(out, "已是干净状态") {
		t.Fatalf("第二次应报告已是干净状态: %s", out)
	}
	assertClean(t, dir)
}

// ---- 10. reset 后重新启动自动重建创世 ----

func TestResetThenStartRebuildsGenesis(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 3)
	if code, out := runReset(t, dir, "-force"); code != 0 {
		t.Fatalf("reset 失败: exit=%d, out=%s", code, out)
	}
	store, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("重新启动打开存储失败: %v", err)
	}
	defer func() { _ = store.Close() }()
	chain, err := blockchain.NewBlockchainFromStoreForTest(store)
	if err != nil {
		t.Fatalf("重新启动失败: %v", err)
	}
	if chain.Height() != 0 {
		t.Fatalf("reset 后启动应为高度 0，实际 %d", chain.Height())
	}
}

// ---- 11. reset 后 verify ----

func TestResetThenVerifyStartsFromCleanGenesis(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 2)
	if code, out := runReset(t, dir, "-force"); code != 0 {
		t.Fatalf("reset 失败: exit=%d, out=%s", code, out)
	}
	// 干净目录里没有 blocks.dat，verify 应明确失败而不是假装通过
	// （此处不复用 runVerify：它会要求 stdout 是合法 JSON，而失败路径只写 stderr）
	{
		var out, errBuf bytes.Buffer
		code := cmdVerify([]string{"-datadir", dir}, &out, &errBuf)
		if code == 0 {
			t.Fatalf("reset 后未启动时 verify 不应返回 PASS（out=%s）", out.String())
		}
	}
	// 启动重建创世后，verify 应从干净的 1 区块状态重新开始
	store, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	if _, err := blockchain.NewBlockchainFromStoreForTest(store); err != nil {
		t.Fatalf("重建创世失败: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("关闭存储失败: %v", err)
	}
	rep, code, out := runVerify(t, dir)
	if code != 0 || !rep.Valid {
		t.Fatalf("reset+启动后 verify 应 PASS: exit=%d, out=%s", code, out)
	}
	if rep.Blocks != 1 || rep.Height != 0 {
		t.Fatalf("应从干净创世重新开始: blocks=%d height=%d", rep.Blocks, rep.Height)
	}
}

// ---- 12. reset 后继续挖矿 ----

func TestResetThenMineAgain(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 4)
	if code, out := runReset(t, dir, "-force"); code != 0 {
		t.Fatalf("reset 失败: exit=%d, out=%s", code, out)
	}
	// 重新建立 2 个区块：等价于 reset 后再次实验
	buildChainForVerify(t, dir, 2)
	rep, code, out := runVerify(t, dir)
	if code != 0 || !rep.Valid {
		t.Fatalf("reset 后再挖矿应可验证: exit=%d, out=%s", code, out)
	}
	if rep.Height != 2 {
		t.Fatalf("期望高度 2，实际 %d", rep.Height)
	}
}

// ---- 13. reset 后创世仍然确定性 ----

func TestResetPreservesDeterministicGenesis(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 3)

	var first string
	{
		store, err := storage.OpenFileBlockStore(dir)
		if err != nil {
			t.Fatalf("打开存储失败: %v", err)
		}
		chain, err := blockchain.NewBlockchainFromStoreForTest(store)
		if err != nil {
			t.Fatalf("加载链失败: %v", err)
		}
		g, err := chain.BlockByHeight(0)
		if err != nil {
			t.Fatalf("读取创世失败: %v", err)
		}
		first = g.Header.HashHex()
		_ = store.Close()
	}

	if code, out := runReset(t, dir, "-force"); code != 0 {
		t.Fatalf("reset 失败: exit=%d, out=%s", code, out)
	}

	store, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("reset 后打开存储失败: %v", err)
	}
	defer func() { _ = store.Close() }()
	chain, err := blockchain.NewBlockchainFromStoreForTest(store)
	if err != nil {
		t.Fatalf("reset 后重建链失败: %v", err)
	}
	g, err := chain.BlockByHeight(0)
	if err != nil {
		t.Fatalf("读取重建后的创世失败: %v", err)
	}
	if g.Header.HashHex() != first {
		t.Fatalf("reset 后创世哈希变了: %s -> %s", first, g.Header.HashHex())
	}
}

// ---- 14. JSON 输出 ----

func TestResetJSONOutput(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 2)

	var out bytes.Buffer
	code := cmdReset([]string{"-datadir", dir, "-force", "-json"}, &out, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	var rep resetReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("reset 报告不是合法 JSON: %v (%s)", err, out.String())
	}
	if !rep.Clean {
		t.Fatalf("clean 应为 true: %+v", rep)
	}
	if len(rep.Removed) != stateFileNames-1 && len(rep.Removed) != stateFileNames {
		t.Fatalf("removed 应包含链与钱包: %+v", rep)
	}
	if len(rep.Failed) != 0 {
		t.Fatalf("不应有失败项: %+v", rep)
	}
}

// ---- 15. 退出码与确认分支 ----

func TestResetExitCodeOnBadFlag(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := cmdReset([]string{"-datadir", t.TempDir(), "-bogus"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("参数错误应返回 2，实际 %d", code)
	}
}

func TestResetConfirmationAccepted(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 2)
	withStdin(t, "yes\n", true)
	code, out := runReset(t, dir) // 无 --force，靠交互确认
	if code != 0 {
		t.Fatalf("输入 yes 应继续并完成，exit=%d, out=%s", code, out)
	}
	assertClean(t, dir)
}

func TestResetConfirmationDeclined(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 2)
	withStdin(t, "no\n", true)
	code, _, errOut := runResetErr(t, dir)
	if code != 1 {
		t.Fatalf("输入非 yes 应取消并返回非 0，exit=%d", code)
	}
	if !strings.Contains(errOut, "已取消") {
		t.Fatalf("应提示已取消: %s", errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "blocks.dat")); err != nil {
		t.Fatalf("取消后 blocks.dat 必须仍然存在")
	}
}

func TestResetRefusedWhenStdinIsNotTerminal(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 2)
	withStdin(t, "", false) // 非终端：脚本 / CI 场景
	code, _, errOut := runResetErr(t, dir)
	if code != 1 {
		t.Fatalf("非终端且无 --force 应失败，exit=%d", code)
	}
	if !strings.Contains(errOut, "--force") {
		t.Fatalf("应提示使用 --force: %s", errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "blocks.dat")); err != nil {
		t.Fatalf("失败后不应删除任何文件")
	}
}

// ---- Windows 文件占用：部分失败不得静默成功（RESET-INV-07）----

func TestResetReportsFailureWhenFileIsLocked(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 1)
	// 持有 blocks.dat 的写句柄，模拟 Windows 上「节点仍在运行」的文件占用
	f, err := os.OpenFile(filepath.Join(dir, "blocks.dat"), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("打开 blocks.dat 失败: %v", err)
	}
	defer func() { _ = f.Close() }()

	// 探测当前平台是否禁止删除已打开的文件（Windows 是，POSIX 不是）
	if err := os.Remove(filepath.Join(dir, "blocks.dat")); err == nil {
		t.Skip("当前平台允许删除已打开的文件，跳过 Windows 占用语义用例")
	}

	code, out := runReset(t, dir, "-force")
	if code == 0 {
		t.Fatalf("占用时应返回失败，out=%s", out)
	}
	if !strings.Contains(out, "FAIL") {
		t.Fatalf("应输出 FAIL: %s", out)
	}
	if !strings.Contains(out, "blocks.dat") {
		t.Fatalf("应指出失败的文件: %s", out)
	}

	// RESET-INV-07（关键）：失败必须中止，绝不能出现「链还在、钱包没了」。
	// blocks.dat 是第一个被处理的文件，它失败后 wallet.json 必须原封不动。
	for _, name := range []string{"blocks.dat", "wallet.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("删除失败后 %s 不应被删除/丢失: %v", name, err)
		}
	}
}

// TestResetAbortsAndKeepsWalletWhenChainDeleteFails 覆盖 RESET-INV-07 的机器可读路径：
// 删除失败时报告的 aborted=true、failed 只含失败项，且剩余状态文件全部保留。
func TestResetAbortsAndKeepsWalletWhenChainDeleteFails(t *testing.T) {
	dir := t.TempDir()
	seedFullState(t, dir, 1)
	f, err := os.OpenFile(filepath.Join(dir, "blocks.dat"), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("打开 blocks.dat 失败: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := os.Remove(filepath.Join(dir, "blocks.dat")); err == nil {
		t.Skip("当前平台允许删除已打开的文件，跳过 Windows 占用语义用例")
	}

	code, out, _ := runResetErr(t, dir, "-force", "-json")
	if code == 0 {
		t.Fatalf("占用时应返回非 0，out=%s", out)
	}
	var rep resetReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("reset -json 输出不是合法 JSON: %v\nout=%s", err, out)
	}
	if !rep.Aborted {
		t.Fatalf("应在首个失败处中止: %+v", rep)
	}
	if rep.Clean {
		t.Fatalf("不应报告 clean: %+v", rep)
	}
	if len(rep.Failed) != 1 || rep.Failed[0] != "blocks.dat" {
		t.Fatalf("failed 应仅为 blocks.dat: %+v", rep)
	}
	if len(rep.Removed) != 0 {
		t.Fatalf("不应有任何文件被删除: %+v", rep)
	}
	if _, err := os.Stat(filepath.Join(dir, "wallet.json")); err != nil {
		t.Fatalf("wallet.json 必须保留: %v", err)
	}
}
