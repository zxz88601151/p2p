package main

// PHASE CONTROL-AUTH-1 测试共享凭据 helper。
//
// 背景自 PHASE CONTROL-AUTH-1 起，mutation 端点（POST /send /mine/start /mine/stop /stop）
// 要求 Bearer Token（fail-closed：未配置即 401）。既有生命周期/全栈测试
// 需要以「已授权客户端」身份访问 mutation 端点，故统一经由本文件的
// 测试 token 与 helper。token 为临时测试凭据，严禁替换为真实生产 token。

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"p2pchain/internal/control"
)

// testToken 供测试节点（-auth-token-file）与测试客户端共用的临时凭据。
const testToken = "p2pchain-itest-token-0123456789abcdef"

// writeTestTokenFile 在临时目录写出一个 0600 token 文件，返回其绝对路径。
// 供 newNodeRuntime 的 AuthTokenFile 与 startRealNode 的 -auth-token-file 使用。
func writeTestTokenFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "control-token")
	if err := os.WriteFile(p, []byte(testToken), 0o600); err != nil {
		t.Fatalf("写入测试 token 文件失败: %v", err)
	}
	return p
}

// testWalletPassword 供测试节点（-wallet-password-file）使用的固定口令。
const testWalletPassword = "p2pchain-itest-wallet-password-0123456789"

// writeTestWalletPWFile 在临时目录写出一个 0600 钱包口令文件（P0-4）。
func writeTestWalletPWFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "wallet-password")
	if err := os.WriteFile(p, []byte(testWalletPassword), 0o600); err != nil {
		t.Fatalf("写入测试口令文件失败: %v", err)
	}
	return p
}

// authedClient 返回携带测试 token 的 control 客户端（mutation 调用必须用它）。
func authedClient(rpc string) *control.Client {
	c := control.NewClient(rpc)
	c.SetToken(testToken)
	return c
}

// postAuthed 以携带测试 token 的 POST 访问 mutation 端点（需要裸 HTTP 断言时用）。
func postAuthed(t *testing.T, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// ---- 出块 helper：取代已下线的按需出块（POST /mine）------------------------
//
// 按需出块整体下线后，「让运行中的节点出块」只剩持续挖矿一条路径：
//
//	POST /mine/start  →  轮询链高  →  POST /mine/stop
//
// 这比原来的按需出块更贴近生产路径（smoke 与运维走的也是这条），
// 因此不降低覆盖强度，反而消除了「测试专用出块通道」与生产路径的偏差。

const (
	// minePollInterval 轮询链高的间隔。
	minePollInterval = 100 * time.Millisecond
	// mineWaitTimeout 等待出块的时限。创世难度下出块极快，60s 对
	// 「挖若干块」的场景非常宽裕；超时即视为真实故障（如同步门挡住挖矿）。
	mineWaitTimeout = 60 * time.Second
	// miningStopTimeout 等待「停止挖矿」生效的时限。
	miningStopTimeout = 30 * time.Second
)

// postMineLifecycle 以测试 token POST /mine/start 或 /mine/stop。
func postMineLifecycle(t *testing.T, rpc, path string) {
	t.Helper()
	resp := postAuthed(t, "http://"+rpc+path, "{}")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s 状态码 = %d, want 200", path, resp.StatusCode)
	}
}

// waitMiningStopped 轮询直到节点的 mining_state 不再是活动态。
func waitMiningStopped(t *testing.T, c *control.Client) {
	t.Helper()
	deadline := time.Now().Add(miningStopTimeout)
	for time.Now().Before(deadline) {
		if st, err := c.Status(); err == nil && st.MiningState == "STOPPED" {
			return
		}
		time.Sleep(minePollInterval)
	}
	t.Fatalf("停止挖矿在 %v 内未生效", miningStopTimeout)
}

// mineViaRPC 让运行中的节点通过持续挖矿生命周期挖出至少 count 个区块，
// 返回相对调用前的新增高度。
//
// 前置条件：节点当前**未在挖矿** —— minerLifecycle 是单飞的，
// 已在挖矿时 /mine/start 会返回 409（本 helper 会因此直接失败）。
func mineViaRPC(t *testing.T, rpc string, count int) int {
	t.Helper()
	c := control.NewClient(rpc)
	st, err := c.Status()
	if err != nil {
		t.Fatalf("取节点状态失败: %v", err)
	}
	start, target := st.Height, st.Height+count

	postMineLifecycle(t, rpc, "/mine/start")

	deadline := time.Now().Add(mineWaitTimeout)
	for time.Now().Before(deadline) {
		if cur, err := c.Status(); err == nil && cur.Height >= target {
			postMineLifecycle(t, rpc, "/mine/stop")
			waitMiningStopped(t, c)
			final, err := c.Status()
			if err != nil {
				t.Fatalf("停止挖矿后取状态失败: %v", err)
			}
			return final.Height - start
		}
		time.Sleep(minePollInterval)
	}

	// 超时：尽力停掉挖矿，避免把正在挖矿的节点留给后续断言。
	postMineLifecycle(t, rpc, "/mine/stop")
	t.Fatalf("持续挖矿在 %v 内未使链高从 %d 达到 %d", mineWaitTimeout, start, target)
	return 0
}
