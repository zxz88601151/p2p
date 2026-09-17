package main

// PHASE CONTROL-AUTH-1 测试共享凭据 helper。
//
// 背景自 PHASE CONTROL-AUTH-1 起，mutation 端点（POST /send /mine /stop）
// 要求 Bearer Token（fail-closed：未配置即 401）。既有生命周期/全栈测试
// 需要以「已授权客户端」身份访问 mutation 端点，故统一经由本文件的
// 测试 token 与 helper。token 为临时测试凭据，严禁替换为真实生产 token。

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
