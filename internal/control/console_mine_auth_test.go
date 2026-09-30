package control_test

// PHASE CONSOLE-MINE-AUTH-FIX-1：Developer Console「立即出块」同源闸门测试矩阵。
//
// 回归背景（实测复现）：
//   - 控制台页面于 2026-09-12 冻结（commit 7926ed5），当时控制接口**尚无鉴权**，
//     页面直接 `fetch(API + "/mine")`。
//   - 2026-09-17/09-18 引入 mutation Bearer Token（cd1b870 / 31b18eb）后 /mine
//     被 requireAuth 保护，而控制台页面**按设计不持有任何凭据**（浏览器零凭据红线）。
//   - 后果：「立即出块」按钮自那时起恒返回 401，页面却仍渲染
//     「可用：POST /mine count=1」——**虚假可用性声明**。
//
// 修复：新增 /console/mine，由服务端按「同源」判定放行（Sec-Fetch-Site / Origin），
// 浏览器侧依旧零凭据（token 绝不下发）。/mine 的 Bearer Token 语义完全不变。
//
// 本文件锁定以下不变量：
//  1. 同源证据成立（Sec-Fetch-Site: same-origin 或 Origin == 本服务源）→ 放行并真正出块；
//  2. 跨站 Origin / 非 same-origin 的 Sec-Fetch-Site → 403，且**不触达** node.Mine；
//  3. 无任何同源证据 → 403（fail-closed，非浏览器本地进程须走 /mine + token）；
//  4. 非 POST → 405 + Allow: POST（由既有方法守卫产生，行为不变）；
//  5. 页面确实改用 /console/mine，且不再直接 POST /mine；
//  6. /mine 仍要求 Bearer Token（修复不得削弱既有边界）。

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// consoleMinePost 向 url 发送 POST，并附加 headers 指定的头。
// 返回状态码与响应体文本。
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

// newConsoleMineServer 起一个**已配置 token** 的 httptest 服务 —— 这正是回归场景：
// token 已启用，而控制台页面没有任何凭据。delay 传 0 以免测试被 500ms 失败延迟拖慢。
func newConsoleMineServer(t *testing.T) (*fakeNode, *httptest.Server) {
	t.Helper()
	node := &fakeNode{}
	_, srv := newAuthServer(t, node, goodToken, 0)
	return node, srv
}

// --- 1. 同源证据成立 → 放行并真正出块 ---

func TestConsoleMineSameOriginViaSecFetchSite(t *testing.T) {
	node, srv := newConsoleMineServer(t)

	code, body := consoleMinePost(t, srv.URL+"/console/mine",
		map[string]string{"Sec-Fetch-Site": "same-origin"}, `{"count":1}`)
	if code != http.StatusOK {
		t.Fatalf("同源（Sec-Fetch-Site: same-origin）应放行，实际 %d: %s", code, body)
	}
	if node.lastMineN != 1 {
		t.Fatalf("应真正触达出块逻辑且 count=1，实际 lastMineN=%d", node.lastMineN)
	}
}

func TestConsoleMineSameOriginViaOriginHeader(t *testing.T) {
	node, srv := newConsoleMineServer(t)

	// 旧浏览器兜底路径：不带 Sec-Fetch-Site，仅带严格等于本服务源的 Origin。
	code, body := consoleMinePost(t, srv.URL+"/console/mine",
		map[string]string{"Origin": srv.URL}, `{"count":1}`)
	if code != http.StatusOK {
		t.Fatalf("Origin 严格等于本服务源时应放行，实际 %d: %s", code, body)
	}
	if node.lastMineN != 1 {
		t.Fatalf("应真正触达出块逻辑且 count=1，实际 lastMineN=%d", node.lastMineN)
	}
}

// --- 2. 跨站 → 403，且不触达出块逻辑 ---

func TestConsoleMineCrossOriginRejected(t *testing.T) {
	node, srv := newConsoleMineServer(t)

	cases := []struct {
		note    string
		headers map[string]string
	}{
		{"跨站 Origin（CSRF）", map[string]string{"Origin": "http://evil.example"}},
		{"DNS rebinding 后的跨站 Origin", map[string]string{"Origin": "http://attacker.test:8789"}},
		{"Sec-Fetch-Site: cross-site", map[string]string{"Sec-Fetch-Site": "cross-site"}},
		{"Sec-Fetch-Site: same-site（非 same-origin）", map[string]string{"Sec-Fetch-Site": "same-site"}},
		// 关键：Sec-Fetch-Site 出现即以其为准，不得回落到 Origin 分支放行。
		{"cross-site 头 + 伪造同源 Origin", map[string]string{
			"Sec-Fetch-Site": "cross-site", "Origin": srv.URL,
		}},
	}
	for _, c := range cases {
		code, body := consoleMinePost(t, srv.URL+"/console/mine", c.headers, `{"count":1}`)
		if code != http.StatusForbidden {
			t.Fatalf("%s：应 403，实际 %d: %s", c.note, code, body)
		}
	}
	if node.lastMineN != 0 {
		t.Fatalf("被拒请求绝不应触达出块逻辑，实际 lastMineN=%d", node.lastMineN)
	}
}

// --- 3. 无同源证据 → 403（fail-closed）---

func TestConsoleMineWithoutOriginEvidenceRejected(t *testing.T) {
	node, srv := newConsoleMineServer(t)

	code, body := consoleMinePost(t, srv.URL+"/console/mine", nil, `{"count":1}`)
	if code != http.StatusForbidden {
		t.Fatalf("无 Sec-Fetch-Site 且无 Origin 时应 fail-closed 为 403，实际 %d: %s", code, body)
	}
	if node.lastMineN != 0 {
		t.Fatalf("被拒请求绝不应触达出块逻辑，实际 lastMineN=%d", node.lastMineN)
	}
}

// --- 4. 非 POST → 405 + Allow: POST（既有方法守卫语义不变）---

func TestConsoleMineRejectsNonPost(t *testing.T) {
	_, srv := newConsoleMineServer(t)

	resp, err := http.Get(srv.URL + "/console/mine")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /console/mine 应 405，实际 %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Allow"); got != http.MethodPost {
		t.Fatalf("405 应带 Allow: POST，实际 %q", got)
	}
}

// --- 5. 页面确实改用同源端点 ---

func TestConsolePageTargetsConsoleMineEndpoint(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{}, nil)
	_, body, _ := getBody(t, srv.URL+"/")

	if !strings.Contains(body, `fetch(API + "/console/mine"`) {
		t.Fatal("控制台页面未使用 /console/mine：页面无凭据，走 /mine 必然 401")
	}
	// 不得再存在「直接 POST /mine」的调用（读端点拼接不在此列）。
	if strings.Contains(body, `fetch(API + "/mine"`) {
		t.Fatal("控制台页面仍直接调用 /mine（受 Bearer Token 保护，页面无凭据必 401）")
	}
	// 虚假可用性声明必须一并修正。
	if strings.Contains(body, "可用：POST /mine count=1") {
		t.Fatal("页面仍声明 /mine 可用 —— 虚假可用性声明")
	}
}

// --- 6. 既有边界不得被削弱 ---

func TestMineEndpointStillRequiresToken(t *testing.T) {
	node, srv := newConsoleMineServer(t)

	// 无 token：401（与修复前一致）。
	code, _ := consoleMinePost(t, srv.URL+"/mine",
		map[string]string{"Sec-Fetch-Site": "same-origin"}, `{"count":1}`)
	if code != http.StatusUnauthorized {
		t.Fatalf("/mine 无 token 应仍为 401，实际 %d", code)
	}
	if node.lastMineN != 0 {
		t.Fatalf("未认证请求绝不应触达出块逻辑，实际 lastMineN=%d", node.lastMineN)
	}

	// 有效 token：200（原有行为不变）。
	code, body := consoleMinePost(t, srv.URL+"/mine",
		map[string]string{"Authorization": "Bearer " + goodToken}, `{"count":1}`)
	if code != http.StatusOK {
		t.Fatalf("/mine 带有效 token 应 200，实际 %d: %s", code, body)
	}
}
