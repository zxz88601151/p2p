package control_test

// 按需出块（POST /mine 与 POST /console/mine）**下线后**的端点契约测试。
//
// 历史：本文件原为 PHASE RPC-CONTROL-PLANE-AUTH-HARDENING-1 的
// /console/mine 认证加固矩阵（E6-C-1 修复）—— 当时该端点仍然存在，只是从
// 「同源闸门放行」收紧为「强制 Bearer Token」，因为同源闸门信任的
// Sec-Fetch-Site / Origin 对 curl / python 等非浏览器客户端可任意伪造。
//
// 现状：按需出块已**整体下线** —— 端点、客户端方法、服务层实现、CLI 子命令
// 全部移除。于是「401 还是 404」的问题不再成立：路由根本不存在，这是比 401
// 更强的边界。本文件据此改写为「端点已移除」的回归锁：
//
//	若有人把路由加回来（哪怕加了认证），下面的用例会立刻失败。
//
// 锁定的不变量：
//  1. POST /console/mine → 404，且与请求头、是否携带 token 无关；
//  2. POST /mine → 404，同上；
//  3. 控制台页面不再残留出块按钮与相关脚本（UI 与后端能力一致）。

import (
	"io"
	"net/http"
	"testing"
	"strings"
)

// consoleMinePost 向 url 发送 POST，并附加 headers 指定的头；返回状态码与响应体文本。
func consoleMinePost(t *testing.T, url string, headers map[string]string, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(data)
}

// TestOnDemandMineEndpointsRemoved 锁定「按需出块端点已整体下线」。
//
// 输入维度刻意沿用原加固矩阵：伪造同源头、跨站 Origin、有效 token、无头。
// 在原实现下这些维度会产生不同的状态码（401 / 200）；现在必须**一律 404** ——
// 这恰好证明判定发生在路由层，而不是「认证层碰巧挡住」。
func TestOnDemandMineEndpointsRemoved(t *testing.T) {
	cases := []struct {
		note    string
		headers map[string]string
	}{
		{"无任何头", nil},
		{"伪造 Sec-Fetch-Site: same-origin", map[string]string{"Sec-Fetch-Site": "same-origin"}},
		{"跨站 Origin（CSRF）", map[string]string{"Origin": "http://evil.example"}},
		{"有效 Bearer Token", map[string]string{"Authorization": "Bearer " + goodToken}},
	}
	paths := []string{"/mine", "/console/mine"}
	for _, path := range paths {
		for _, c := range cases {
			_, srv := newAuthServer(t, &fakeNode{}, goodToken, 0)
			code, body := consoleMinePost(t, srv.URL+path, c.headers, `{"count":1}`)
			if code != http.StatusNotFound {
				t.Fatalf("POST %s（%s）= %d, want 404（按需出块已下线，端点不应存在）: %s",
					path, c.note, code, body)
			}
		}
	}
}

// TestConsolePageHasNoMiningButton 控制台页面必须与后端能力一致：
// 出块端点下线后，页面不得再保留按钮、脚本调用或「可用」声明。
//
// 保留「痕迹扫描」而非仅删代码：UI 与后端不一致（页面按钮必然 404）是
// 一类典型回归，静态扫描能在无人点按钮时就抓住它。
func TestConsolePageHasNoMiningButton(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{}, nil)
	_, body, _ := getBody(t, srv.URL+"/")

	for _, banned := range []string{
		`fetch(API + "/mine"`,
		`fetch(API + "/console/mine"`,
		`id="mineBtn"`,
		`id="mineBtnText"`,
		"可用：POST /console/mine",
		"可用：POST /mine count=1",
	} {
		if strings.Contains(body, banned) {
			t.Fatalf("控制台页面仍残留已下线功能的痕迹 %q（UI 必须与后端能力一致）", banned)
		}
	}
}
