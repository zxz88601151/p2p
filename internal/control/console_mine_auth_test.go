package control_test

// PHASE RPC-CONTROL-PLANE-AUTH-HARDENING-1：/console/mine 认证加固测试矩阵（E6-C-1 修复）。
//
// 回归背景（E6-SECURITY-HARDENING-AUDIT-1 E6-C-1，HIGH）：
//   - 旧 /console/mine 由「同源闸门」放行（Sec-Fetch-Site: same-origin 或
//     Origin == 本服务源）；其信任的请求头对**非浏览器 HTTP 客户端**（curl/python）
//     可任意伪造 ⇒ 无 Bearer 即可出块，是一条未授权 mutation 路径。
//   - 本阶段退役该闸门：/console/mine 与 /mine 一致，仅认 Bearer Token。
//
// 本文件锁定以下不变量（hardened contract）：
//  1. 无 token（无论是否伪造 Sec-Fetch-Site / Origin）→ 401，且**不触达** node.Mine；
//  2. 伪造 Sec-Fetch-Site: same-origin（无 token）→ 401（伪造头不再是凭据）；
//  3. 伪造 Origin == 本服务源（无 token）→ 401；
//  4. 跨站 Origin / cross-site Sec-Fetch-Site（无 token）→ 401；
//  5. 有效 token → 200 并真正出块（与 /mine 行为一致）；
//  6. 非 POST → 405 + Allow: POST（既有方法守卫语义不变）；
//  7. 控制台页面不再声明「同源可用」，不得保留零凭据可用性声明；
//  8. /mine 仍要求 Bearer Token（加固不得削弱既有边界）。

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

// newConsoleMineServer 起一个**已配置 token** 的 httptest 服务。
// delay 传 0 以免测试被 500ms 失败延迟拖慢。
func newConsoleMineServer(t *testing.T) (*fakeNode, *httptest.Server) {
	t.Helper()
	node := &fakeNode{}
	_, srv := newAuthServer(t, node, goodToken, 0)
	return node, srv
}

// --- 1. 伪造同源头不再放行：无 token → 401（且不触达出块）---

func TestConsoleMineRejectedWithoutToken(t *testing.T) {
	cases := []struct {
		note    string
		headers map[string]string
	}{
		{"伪造 Sec-Fetch-Site: same-origin", map[string]string{"Sec-Fetch-Site": "same-origin"}},
		{"无任何头", nil},
		{"跨站 Origin（CSRF）", map[string]string{"Origin": "http://evil.example"}},
		{"Sec-Fetch-Site: cross-site", map[string]string{"Sec-Fetch-Site": "cross-site"}},
		{"cross-site 头 + 伪造同源 Origin", map[string]string{
			"Sec-Fetch-Site": "cross-site", "Origin": "http://127.0.0.1:1",
		}},
	}
	for _, c := range cases {
		node, srv := newConsoleMineServer(t)
		code, body := consoleMinePost(t, srv.URL+"/console/mine", c.headers, `{"count":1}`)
		if code != http.StatusUnauthorized {
			t.Fatalf("%s：无 token 应 401（加固后伪造头不再放行），实际 %d: %s", c.note, code, body)
		}
		if node.lastMineN != 0 {
			t.Fatalf("%s：未认证请求绝不应触达出块逻辑，实际 lastMineN=%d", c.note, node.lastMineN)
		}
	}
}

// --- 2. 伪造 Origin == 本服务源（无 token）→ 401 ---

func TestConsoleMineRejectsForgedSelfOrigin(t *testing.T) {
	// 该头在旧实现下被当作同源证据放行；加固后必须被拒（它是可伪造的）。
	node := &fakeNode{}
	_, srv := newAuthServer(t, node, goodToken, 0)

	code, body := consoleMinePost(t, srv.URL+"/console/mine",
		map[string]string{"Origin": srv.URL}, `{"count":1}`)
	if code != http.StatusUnauthorized {
		t.Fatalf("伪造 Origin == 本服务源且无 token 应 401，实际 %d: %s", code, body)
	}
	if node.lastMineN != 0 {
		t.Fatalf("未认证请求绝不应触达出块逻辑，实际 lastMineN=%d", node.lastMineN)
	}
}

// --- 3. 有效 token → 200 并真正出块（与 /mine 一致）---

func TestConsoleMineWithValidTokenReachesHandler(t *testing.T) {
	node, srv := newConsoleMineServer(t)

	code, body := consoleMinePost(t, srv.URL+"/console/mine",
		map[string]string{"Authorization": "Bearer " + goodToken}, `{"count":1}`)
	if code != http.StatusOK {
		t.Fatalf("/console/mine 带有效 token 应 200，实际 %d: %s", code, body)
	}
	if node.lastMineN != 1 {
		t.Fatalf("应真正触达出块逻辑且 count=1，实际 lastMineN=%d", node.lastMineN)
	}
}

// --- 4. fail-closed：未配置 token 时 /console/mine 恒 401 ---

func TestConsoleMineFailClosedWhenNoTokenConfigured(t *testing.T) {
	node := &fakeNode{}
	_, srv := newAuthServer(t, node, "", 0) // 未配置 token
	code, _ := consoleMinePost(t, srv.URL+"/console/mine",
		map[string]string{"Sec-Fetch-Site": "same-origin"}, `{"count":1}`)
	if code != http.StatusUnauthorized {
		t.Fatalf("未配置 token 时 /console/mine 应 401（fail-closed），实际 %d", code)
	}
	if node.lastMineN != 0 {
		t.Fatalf("未认证请求绝不应触达出块逻辑，实际 lastMineN=%d", node.lastMineN)
	}
}

// --- 5. 非 POST → 405 + Allow: POST（既有方法守卫语义不变）---

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

// --- 6. 控制台页面：不得保留零凭据可用性声明 ---

func TestConsolePageNoLongerClaimsCredentiallessMining(t *testing.T) {
	_, srv := newConsoleTestPair(t, &fakeNode{}, nil)
	_, body, _ := getBody(t, srv.URL+"/")

	// 旧的「同源可用」声明必须消失（网页零凭据，出块端点已要求 token）。
	if strings.Contains(body, "可用：POST /console/mine") {
		t.Fatal("页面仍声明 /console/mine 同源可用 —— 加固后该声明为虚假可用性")
	}
	if strings.Contains(body, "可用：POST /mine count=1") {
		t.Fatal("页面仍声明 /mine 可用 —— 虚假可用性声明")
	}
	// 不得再直接调用受 token 保护的 /mine。
	if strings.Contains(body, `fetch(API + "/mine"`) {
		t.Fatal("控制台页面仍直接调用 /mine（受 Bearer Token 保护，页面无凭据必 401）")
	}
}

// --- 7. 既有边界不得被削弱：/mine 仍要求 token ---

func TestMineEndpointStillRequiresToken(t *testing.T) {
	node, srv := newConsoleMineServer(t)

	// 无 token：401（即使伪造同源头）。
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
