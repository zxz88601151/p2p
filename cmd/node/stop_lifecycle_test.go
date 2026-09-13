package main

// PHASE PRODUCT-DEV-1B §7 节点生命周期测试。
//
// 核心原则（§7 明确要求）：
//
//	如果测试需要真实 subprocess：必须真实执行。不能只 mock process lifecycle。
//
// 因此本文件中凡涉及「进程是否真的退出」「node.lock 是否真的被删除」的断言，
// 全部通过 `go build` 出来的真实 node 二进制 + os/exec 真实子进程完成。
// 这与 reset_test.go 只用进程内 cmdReset 的做法不同——因为 graceful stop 的价值
// 恰恰在于「跨进程」：它必须让另一个进程走完清理链后退出。
//
// 覆盖的 §5 不变量：
//
//	STOP-INV-01 正常 stop 后进程退出
//	STOP-INV-02 正常 stop 后 lock 不残留
//	STOP-INV-03 正常 stop 后可以立即 restart
//	STOP-INV-04 restart 不需要 reset --force
//	STOP-INV-05 stop 不损坏 blocks.dat
//	STOP-INV-06 stop 不损坏 wallet.json
//	STOP-INV-07 stop 后 verify 正常
//	STOP-INV-08 stop → restart → verify 结果与 stop 前一致
//	STOP-INV-09 stop 过程发生异常时，不得静默报告成功
//	STOP-INV-10 /stop 仅接受 POST
//	STOP-INV-11 停止请求幂等
//	STOP-INV-12 节点不存在时给出可操作错误，退出码非 0

import (
	"bytes"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"p2pchain/internal/blockchain"
	"p2pchain/internal/control"
	"p2pchain/internal/storage"
)

// ---- 真实子进程辅助 ----

// freePort 取一个当前空闲的本机端口（尽力而为：释放后极小概率被他人占用）。
func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("申请空闲端口失败: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

// realNode 一个通过真实二进制启动的节点子进程。
type realNode struct {
	t      *testing.T
	dir    string
	rpc    string
	cmd    *exec.Cmd
	output *bytes.Buffer
	exited chan error

	waitErr error
}

// startRealNode 用真实二进制启动节点，等待控制接口就绪后返回。
func startRealNode(t *testing.T, dir string, extra ...string) *realNode {
	t.Helper()
	bin := buildNodeBinary(t)
	rpc := freePort(t)
	listen := freePort(t)

	args := []string{"-datadir", dir, "-listen", listen, "-rpc", rpc}
	args = append(args, extra...)

	cmd := exec.Command(bin, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动节点子进程失败: %v", err)
	}
	n := &realNode{t: t, dir: dir, rpc: rpc, cmd: cmd, output: &out, exited: make(chan error, 1)}
	go func() { n.exited <- cmd.Wait() }()
	// 无论测试成败都必须回收子进程，避免残留进程占用数据目录
	t.Cleanup(n.kill)

	n.waitReady(30 * time.Second)
	return n
}

// waitReady 轮询控制接口直到就绪。
func (n *realNode) waitReady(timeout time.Duration) {
	n.t.Helper()
	client := control.NewClient(n.rpc)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-n.exited:
			n.t.Fatalf("节点在就绪前就退出了: %v\n输出:\n%s", err, n.output.String())
		default:
		}
		if _, err := client.Status(); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	n.t.Fatalf("节点控制接口 %s 在 %v 内未就绪\n输出:\n%s", n.rpc, timeout, n.output.String())
}

// kill 强制结束子进程（仅用于清理与「模拟异常停止」；正常停止必须走 stop）。
func (n *realNode) kill() {
	if n.cmd == nil || n.cmd.Process == nil {
		return
	}
	_ = n.cmd.Process.Kill()
	select {
	case <-n.exited:
	case <-time.After(5 * time.Second):
	}
}

// waitExit 等待子进程退出；超时返回 false。
func (n *realNode) waitExit(timeout time.Duration) bool {
	n.t.Helper()
	select {
	case err := <-n.exited:
		n.waitErr = err
		return true
	case <-time.After(timeout):
		return false
	}
}

// stopViaCLI 通过真实的 cmdStop（与 `node stop` 完全同一条代码路径）停止节点。
func (n *realNode) stopViaCLI() (int, string) {
	n.t.Helper()
	var out, errBuf bytes.Buffer
	code := cmdStop([]string{"-rpc", n.rpc}, &out, &errBuf)
	return code, out.String() + errBuf.String()
}

// status 取节点状态。
func (n *realNode) status() (*control.StatusInfo, error) {
	return control.NewClient(n.rpc).Status()
}

// p2pListenAddr 从节点日志中解析 P2P 监听地址
// （日志形如「[p2p] 节点已启动，监听 127.0.0.1:PORT（nodeID=...）」）。
func (n *realNode) p2pListenAddr() string {
	n.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		text := n.output.String()
		const marker = "[p2p] 节点已启动，监听 "
		if i := strings.Index(text, marker); i >= 0 {
			rest := text[i+len(marker):]
			if j := strings.IndexAny(rest, "（\n"); j > 0 {
				return strings.TrimSpace(rest[:j])
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	n.t.Fatalf("未能从日志解析 P2P 监听地址:\n%s", n.output.String())
	return ""
}

// ---- §5 不变量断言辅助 ----

// assertNoLock 断言数据目录中没有 node.lock（STOP-INV-02）。
func assertNoLock(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, "node.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("STOP-INV-02 违反：node.lock 仍存在（%v）", err)
	}
}

// verifyDir 在给定数据目录上执行离线只读校验，返回是否通过。
func verifyDir(t *testing.T, dir string) bool {
	t.Helper()
	store, err := storage.OpenFileBlockStoreReadOnly(dir)
	if err != nil {
		t.Fatalf("打开区块数据失败: %v", err)
	}
	defer func() { _ = store.Close() }()
	rep, err := blockchain.VerifyStoredChain(store)
	if err != nil {
		t.Fatalf("执行校验失败: %v", err)
	}
	if !rep.Valid {
		t.Logf("校验未通过: 高度=%d 原因=%s", rep.FailHeight, rep.Reason)
	}
	return rep.Valid
}

// ---- 用例：控制接口层（STOP-INV-10 / 11 / 12）----

// TestStopEndpointRequiresPOST 覆盖 STOP-INV-10：停止是破坏性动作，绝不能由 GET 触发。
func TestStopEndpointRequiresPOST(t *testing.T) {
	rt, _, _ := startTestRuntime(t, false)
	defer rt.Close()

	srv := control.NewServer(rt.svc)
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("启动控制接口失败: %v", err)
	}
	defer func() { _ = srv.Stop() }()

	// 未注入 hook：POST 应返回 501（不假装有能力）
	if _, err := control.NewClient(addr).Stop(); err == nil {
		t.Fatal("未注入停止回调时 POST /stop 应失败")
	} else if !strings.Contains(err.Error(), "501") {
		t.Fatalf("应返回 501，实际: %v", err)
	}

	resp, err := http.Get("http://" + addr + "/stop")
	if err != nil {
		t.Fatalf("GET /stop 请求失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /stop 应返回 405，实际 %d", resp.StatusCode)
	}
}

// TestStopHookFiresOnce 覆盖 STOP-INV-11：重复的停止请求不得重复触发关闭。
func TestStopHookFiresOnce(t *testing.T) {
	rt, _, _ := startTestRuntime(t, false)
	defer rt.Close()

	srv := control.NewServer(rt.svc)
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("启动控制接口失败: %v", err)
	}
	defer func() { _ = srv.Stop() }()

	calls := 0
	srv.SetStopHook(func() { calls++ })

	for i := 0; i < 3; i++ {
		resp, err := control.NewClient(addr).Stop()
		if err != nil {
			t.Fatalf("第 %d 次 POST /stop 失败: %v", i+1, err)
		}
		if !resp.Accepted {
			t.Fatalf("第 %d 次请求应被接受", i+1)
		}
	}
	if calls != 1 {
		t.Fatalf("停止回调应只触发 1 次，实际 %d 次", calls)
	}
}

// TestStopCommandFailsWhenNodeAbsent 覆盖 STOP-INV-12：节点不在时必须明确失败。
func TestStopCommandFailsWhenNodeAbsent(t *testing.T) {
	dead := freePort(t) // 立即释放，几乎不可能有人在监听
	var out, errBuf bytes.Buffer
	code := cmdStop([]string{"-rpc", dead}, &out, &errBuf)
	if code == 0 {
		t.Fatalf("节点不存在时应返回非 0，out=%s", out.String())
	}
	body := out.String() + errBuf.String()
	if !strings.Contains(body, "启动节点") {
		t.Fatalf("应给出可操作提示，实际输出: %s", body)
	}
}

// TestStopCommandUsageError 参数错误必须走退出码 2。
func TestStopCommandUsageError(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := cmdStop([]string{"-nope"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("参数错误应返回 2，实际 %d", code)
	}
}

// ---- 用例：真实子进程生命周期（STOP-INV-01..09）----

// TestGracefulStopExitsAndReleasesLock 是 F-1 的核心回归（STOP-INV-01 / 02）。
func TestGracefulStopExitsAndReleasesLock(t *testing.T) {
	dir := t.TempDir()
	n := startRealNode(t, dir)

	code, out := n.stopViaCLI()
	if code != 0 {
		t.Fatalf("node stop 应成功，退出码=%d 输出=%s\n节点输出:\n%s", code, out, n.output.String())
	}
	if !n.waitExit(10 * time.Second) {
		t.Fatalf("STOP-INV-01 违反：进程未退出\n节点输出:\n%s", n.output.String())
	}
	if n.waitErr != nil {
		t.Fatalf("节点应以 0 退出，实际: %v", n.waitErr)
	}
	assertNoLock(t, dir)
	if !strings.Contains(n.output.String(), "释放数据目录锁") {
		t.Fatalf("节点日志应包含关闭提示，实际:\n%s", n.output.String())
	}
}

// TestStopPreservesChainAndWallet 覆盖 STOP-INV-05 / 06：stop 不得损坏数据文件。
func TestStopPreservesChainAndWallet(t *testing.T) {
	dir := t.TempDir()
	n := startRealNode(t, dir)

	if _, err := control.NewClient(n.rpc).Mine(3); err != nil {
		t.Fatalf("挖矿失败: %v", err)
	}
	before := map[string][]byte{}
	for _, name := range []string{"blocks.dat", "wallet.json"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", name, err)
		}
		before[name] = data
	}

	code, out := n.stopViaCLI()
	if code != 0 {
		t.Fatalf("stop 失败: %s", out)
	}
	if !n.waitExit(10 * time.Second) {
		t.Fatal("进程未退出")
	}

	for name, want := range before {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("stop 后 %s 丢失: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("STOP-INV-05/06 违反：%s 在 stop 后内容发生变化", name)
		}
	}
}

// TestStopThenRestartNeedsNoForce 覆盖 STOP-INV-03 / 04。
func TestStopThenRestartNeedsNoForce(t *testing.T) {
	dir := t.TempDir()
	n1 := startRealNode(t, dir)
	if _, err := control.NewClient(n1.rpc).Mine(2); err != nil {
		t.Fatalf("挖矿失败: %v", err)
	}
	if code, out := n1.stopViaCLI(); code != 0 {
		t.Fatalf("stop 失败: %s", out)
	}
	if !n1.waitExit(10 * time.Second) {
		t.Fatal("进程未退出")
	}
	assertNoLock(t, dir)

	// 立即重启：不得需要 reset，更不得需要 --force
	n2 := startRealNode(t, dir)
	st, err := n2.status()
	if err != nil {
		t.Fatalf("STOP-INV-03 违反：重启后无法查询状态: %v", err)
	}
	if st.Height != 2 {
		t.Fatalf("重启后应保留高度 2，实际 %d", st.Height)
	}
}

// TestStopThenVerifyConsistent 覆盖 STOP-INV-07 / 08。
func TestStopThenVerifyConsistent(t *testing.T) {
	dir := t.TempDir()
	n1 := startRealNode(t, dir)
	if _, err := control.NewClient(n1.rpc).Mine(5); err != nil {
		t.Fatalf("挖矿失败: %v", err)
	}
	st1, err := n1.status()
	if err != nil {
		t.Fatalf("取状态失败: %v", err)
	}
	if code, out := n1.stopViaCLI(); code != 0 {
		t.Fatalf("stop 失败: %s", out)
	}
	if !n1.waitExit(10 * time.Second) {
		t.Fatal("进程未退出")
	}
	if !verifyDir(t, dir) {
		t.Fatal("STOP-INV-07 违反：stop 后 verify 不通过")
	}

	n2 := startRealNode(t, dir)
	st2, err := n2.status()
	if err != nil {
		t.Fatalf("重启后取状态失败: %v", err)
	}
	if st2.Height != st1.Height || st2.TipHash != st1.TipHash {
		t.Fatalf("STOP-INV-08 违反：重启后状态不一致 before=%+v after=%+v", st1, st2)
	}
	if !verifyDir(t, dir) {
		t.Fatal("STOP-INV-08 违反：restart 后 verify 不通过")
	}
}

// TestStopWhileMining 覆盖 §7-10：挖矿过程中停止，链不得损坏。
func TestStopWhileMining(t *testing.T) {
	dir := t.TempDir()
	n := startRealNode(t, dir, "-mine")
	// 等至少一个区块，确保停止时正处于挖矿循环内
	deadline := time.Now().Add(30 * time.Second)
	mined := 0
	for time.Now().Before(deadline) {
		if st, err := n.status(); err == nil && st.Height > 0 {
			mined = st.Height
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if mined == 0 {
		t.Fatal("挖矿模式未产出任何区块")
	}

	code, out := n.stopViaCLI()
	if code != 0 {
		t.Fatalf("挖矿中 stop 应成功，退出码=%d 输出=%s\n节点输出:\n%s", code, out, n.output.String())
	}
	if !n.waitExit(15 * time.Second) {
		t.Fatalf("挖矿中 stop 后进程未退出\n节点输出:\n%s", n.output.String())
	}
	assertNoLock(t, dir)
	// 关键：挖矿/停止最容易写出半截区块，这里必须仍然校验通过
	if !verifyDir(t, dir) {
		t.Fatal("STOP-INV-05 违反：挖矿中停止后链校验失败")
	}
}

// TestStopAfterTransaction 覆盖 §7-11：有交易历史后停止。
func TestStopAfterTransaction(t *testing.T) {
	dir := t.TempDir()
	n := startRealNode(t, dir)
	if _, err := control.NewClient(n.rpc).Mine(12); err != nil {
		t.Fatalf("挖矿失败: %v", err)
	}
	st, err := n.status()
	if err != nil {
		t.Fatalf("取状态失败: %v", err)
	}
	if _, err := control.NewClient(n.rpc).Send(control.SendRequest{
		To: st.Address, Amount: 1, Fee: 1,
	}); err != nil {
		t.Fatalf("发送交易失败: %v", err)
	}
	if _, err := control.NewClient(n.rpc).Mine(1); err != nil {
		t.Fatalf("打包失败: %v", err)
	}

	if code, out := n.stopViaCLI(); code != 0 {
		t.Fatalf("stop 失败: %s", out)
	}
	if !n.waitExit(10 * time.Second) {
		t.Fatal("进程未退出")
	}
	assertNoLock(t, dir)
	if !verifyDir(t, dir) {
		t.Fatal("STOP-INV-07 违反：含交易的链在 stop 后校验失败")
	}
}

// TestStopAfterP2PActivity 覆盖 §7-12：建立过 P2P 连接后停止。
func TestStopAfterP2PActivity(t *testing.T) {
	dirA, dirB := t.TempDir(), t.TempDir()
	a := startRealNode(t, dirA)
	b := startRealNode(t, dirB, "-seed", a.p2pListenAddr())

	deadline := time.Now().Add(20 * time.Second)
	connected := false
	for time.Now().Before(deadline) {
		if st, err := b.status(); err == nil && len(st.Peers) > 0 {
			connected = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !connected {
		t.Fatalf("P2P 连接未建立\nB 输出:\n%s", b.output.String())
	}

	if code, out := b.stopViaCLI(); code != 0 {
		t.Fatalf("P2P 活动后 stop 失败: %s", out)
	}
	if !b.waitExit(10 * time.Second) {
		t.Fatal("进程未退出")
	}
	assertNoLock(t, dirB)
	if !verifyDir(t, dirB) {
		t.Fatal("STOP-INV-07 违反：P2P 活动后 stop 使链校验失败")
	}
}

// TestRepeatedStartStop 覆盖 §7-13：反复 start/stop 不得劣化。
func TestRepeatedStartStop(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 3; i++ {
		n := startRealNode(t, dir)
		if code, out := n.stopViaCLI(); code != 0 {
			t.Fatalf("第 %d 轮 stop 失败: %s", i, out)
		}
		if !n.waitExit(10 * time.Second) {
			t.Fatalf("第 %d 轮进程未退出", i)
		}
		assertNoLock(t, dir)
	}
	if !verifyDir(t, dir) {
		t.Fatal("反复 start/stop 后链校验失败")
	}
}

// TestResetAfterGracefulStopNeedsNoForce 覆盖 §7-14 / 15 / 16：
// graceful stop 之后 reset 不再需要 --force；reset 后重启仍是确定性创世。
func TestResetAfterGracefulStopNeedsNoForce(t *testing.T) {
	dir := t.TempDir()
	n := startRealNode(t, dir)
	st0, err := n.status()
	if err != nil {
		t.Fatalf("取状态失败: %v", err)
	}
	genesisBefore := st0.TipHash // 高度 0 时链尾即创世
	if _, err := control.NewClient(n.rpc).Mine(4); err != nil {
		t.Fatalf("挖矿失败: %v", err)
	}
	if code, out := n.stopViaCLI(); code != 0 {
		t.Fatalf("stop 失败: %s", out)
	}
	if !n.waitExit(10 * time.Second) {
		t.Fatal("进程未退出")
	}

	// 关键产品收益：正常停止后，reset 无需 --force（输入 yes 模拟终端确认）
	withStdin(t, "yes\n", true)
	var out, errBuf bytes.Buffer
	code := cmdReset([]string{"-datadir", dir}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("graceful stop 后 reset 应无需 --force，退出码=%d 输出=%s%s",
			code, out.String(), errBuf.String())
	}
	assertClean(t, dir)

	n2 := startRealNode(t, dir)
	st, err := n2.status()
	if err != nil {
		t.Fatalf("reset 后重启失败: %v", err)
	}
	if st.TipHash != genesisBefore {
		t.Fatalf("确定性创世不一致: before=%s after=%s", genesisBefore, st.TipHash)
	}
	if !verifyDir(t, dir) {
		t.Fatal("reset 重启后校验失败")
	}
}

// TestResetRefusesWhenStopFailed 覆盖 STOP-INV-09 的边界：
// 若停止确实失败（锁残留），reset 必须拒绝并提示，而不是静默成功。
func TestResetRefusesWhenStopFailed(t *testing.T) {
	dir := t.TempDir()
	n := startRealNode(t, dir)
	n.kill() // 硬杀：模拟「停止失败」的最坏情况
	if _, err := os.Stat(filepath.Join(dir, "node.lock")); err != nil {
		t.Fatalf("硬杀后应残留 node.lock，实际: %v", err)
	}
	withStdin(t, "yes\n", true)
	var out, errBuf bytes.Buffer
	code := cmdReset([]string{"-datadir", dir}, &out, &errBuf)
	if code == 0 {
		t.Fatal("锁残留时 reset 应拒绝并报错，实际返回 0")
	}
	body := out.String() + errBuf.String()
	if !strings.Contains(body, "已被占用") {
		t.Fatalf("应报告目录被占用，实际: %s", body)
	}
}
