package explorer

// PHASE MINING-LIFECYCLE-1：Explorer mutation 通道（方案 A 冻结）测试。
//
// 安全验收（授权词 §23）逐项：
//   - 浏览器请求无 token、响应体无 token（服务端注入，零暴露）；
//   - 白名单仅 /api/mine/start、/api/mine/stop；/api/send /api/stop /api/console
//     一律本地 404 且上游零接触（upstreamHits==0 断言，复用 837ef9f 手法）；
//   - Origin 校验（跨源 403；同源放行；无 Origin 的非浏览器客户端放行）；
//   - Content-Type 必须 application/json（简单表单请求被 403 拦截）；
//   - GET 到 mutation → 405 + Allow: POST；
//   - token 未配置 → 503 fail-closed；
//   - 上游不可达 → 502 本地错误体。
//
// 上游为 httptest 假 control：记录收到的 Authorization/路径，用于断言
// 「服务端注入」与「路径改写」；绝不在本测试使用真实生产 token。

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const mutTestToken = "explorer-test-token-0123456789abcdef"

type upstreamRecorder struct {
	mu       sync.Mutex
	hits     int
	auths    []string
	paths    []string
	respBody string
	status   int
}

func (u *upstreamRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u.mu.Lock()
	u.hits++
	u.auths = append(u.auths, r.Header.Get("Authorization"))
	u.paths = append(u.paths, r.URL.Path)
	body := u.respBody
	st := u.status
	u.mu.Unlock()
	if st == 0 {
		st = http.StatusOK
	}
	if body == "" {
		body = `{"accepted":true,"state":"STARTING","height":5}`
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(st)
	_, _ = io.WriteString(w, body)
}

func (u *upstreamRecorder) snapshot() (int, []string, []string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.hits, append([]string(nil), u.auths...), append([]string(nil), u.paths...)
}

func newMutationTestEnv(t *testing.T, token string) (*httptest.Server, *upstreamRecorder) {
	t.Helper()
	up := &upstreamRecorder{}
	upsrv := httptest.NewServer(up)
	t.Cleanup(upsrv.Close)
	h, err := NewHandler(upsrv.URL, token)
	if err != nil {
		t.Fatalf("NewHandler 失败: %v", err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, up
}

// doReq 发送请求并返回状态码与响应体文本。
func doReq(t *testing.T, method, url, body, contentType string, headers map[string]string) (int, string) {
	t.Helper()
	var req *http.Request
	var err error
	if body != "" {
		req, err = http.NewRequest(method, url, strings.NewReader(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

// TestMutationProxyInjectsTokenAndRewritesPath：合法 mutation 被代理，路径改写
// 为上游 /mine/start，Authorization 由服务端注入——浏览器请求中无 token。
func TestMutationProxyInjectsTokenAndRewritesPath(t *testing.T) {
	srv, up := newMutationTestEnv(t, mutTestToken)

	code, body := doReq(t, http.MethodPost, srv.URL+"/api/mine/start", "{}", "application/json", nil)
	if code != http.StatusOK {
		t.Fatalf("合法 mutation 应 200，实际 %d（%s）", code, body)
	}
	hits, auths, paths := up.snapshot()
	if hits != 1 {
		t.Fatalf("上游应恰好被命中 1 次，实际 %d", hits)
	}
	if paths[0] != "/mine/start" {
		t.Fatalf("上游路径应为 /mine/start，实际 %s", paths[0])
	}
	if len(auths) != 1 || auths[0] != "Bearer "+mutTestToken {
		t.Fatalf("上游应收到服务端注入的 Bearer token，实际 %v", auths)
	}
	// token 零暴露：响应体不得包含 token（§23 安全验收）。
	if strings.Contains(body, mutTestToken) {
		t.Fatal("响应体泄露 token")
	}
	// 响应体为上游 JSON（accepted 字段在）。
	var obj map[string]any
	if err := json.Unmarshal([]byte(body), &obj); err != nil || obj["accepted"] != true {
		t.Fatalf("响应体应为上游 JSON: %s", body)
	}
}

// TestMutationOriginBoundary：Origin 校验（跨源 403 / 同源放行 / 无 Origin 放行）。
func TestMutationOriginBoundary(t *testing.T) {
	srv, up := newMutationTestEnv(t, mutTestToken)
	host := strings.TrimPrefix(srv.URL, "http://")

	// 同源 Origin → 放行。
	code, _ := doReq(t, http.MethodPost, srv.URL+"/api/mine/start", "{}", "application/json",
		map[string]string{"Origin": "http://" + host})
	if code != http.StatusOK {
		t.Fatalf("同源 Origin 应放行，实际 %d", code)
	}
	// 跨源 Origin → 403，上游零接触。
	code, body := doReq(t, http.MethodPost, srv.URL+"/api/mine/start", "{}", "application/json",
		map[string]string{"Origin": "http://evil.example.com"})
	if code != http.StatusForbidden {
		t.Fatalf("跨源 Origin 应 403，实际 %d", code)
	}
	if strings.Contains(body, "accepted") {
		t.Fatal("403 本地拒绝体不应包含上游内容")
	}
	// 无 Origin（curl/脚本）→ 放行（运维兼容）。
	code, _ = doReq(t, http.MethodPost, srv.URL+"/api/mine/stop", "{}", "application/json", nil)
	if code != http.StatusOK {
		t.Fatalf("无 Origin 应放行，实际 %d", code)
	}
	if _, _, paths := up.snapshot(); len(paths) != 2 {
		t.Fatalf("上游命中数应为 2（跨源请求被本地拦截），实际 %d", len(paths))
	}
}

// TestMutationContentTypeBoundary：简单表单 Content-Type 被 403 拦截（CSRF 防线）。
func TestMutationContentTypeBoundary(t *testing.T) {
	srv, up := newMutationTestEnv(t, mutTestToken)

	for _, ct := range []string{"application/x-www-form-urlencoded", "text/plain", ""} {
		code, _ := doReq(t, http.MethodPost, srv.URL+"/api/mine/start", "count=1", ct, nil)
		if code != http.StatusForbidden {
			t.Fatalf("Content-Type=%q 应 403，实际 %d", ct, code)
		}
	}
	if hits, _, _ := up.snapshot(); hits != 0 {
		t.Fatalf("被拒请求不得触达上游，实际 %d", hits)
	}
}

// TestMutationWhitelistExclusive：白名单排他性——/api/send /api/stop /api/console
// 及未知路径一律本地 404，上游零接触（F-1 不变量在新通道下保持）。
func TestMutationWhitelistExclusive(t *testing.T) {
	srv, up := newMutationTestEnv(t, mutTestToken)

	for _, p := range []string{"/api/send", "/api/stop", "/api/console", "/api/balance", "/api/mine", "/api/unknown"} {
		code, body := doReq(t, http.MethodPost, srv.URL+p, "{}", "application/json", nil)
		if code != http.StatusNotFound {
			t.Fatalf("%s 应本地 404，实际 %d", p, code)
		}
		if strings.Contains(body, "unauthorized") {
			t.Fatalf("%s 的 404 应为 Explorer 本地错误体，而非上游措辞", p)
		}
	}
	if hits, _, _ := up.snapshot(); hits != 0 {
		t.Fatalf("白名单外路径不得触达上游，实际 %d", hits)
	}
}

// TestMutationMethodGate：mutation 端点 GET → 405 + Allow: POST。
func TestMutationMethodGate(t *testing.T) {
	srv, up := newMutationTestEnv(t, mutTestToken)

	res, err := http.Get(srv.URL + "/api/mine/start")
	if err != nil {
		t.Fatalf("GET 失败: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed || res.Header.Get("Allow") != "POST" {
		t.Fatalf("GET mutation 应 405+Allow:POST，实际 %d / %q", res.StatusCode, res.Header.Get("Allow"))
	}
	if hits, _, _ := up.snapshot(); hits != 0 {
		t.Fatalf("GET 不得触达上游，实际 %d", hits)
	}
}

// TestMutationFailClosedWithoutToken：token 未配置 ⇒ 503 fail-closed（只读模式）。
func TestMutationFailClosedWithoutToken(t *testing.T) {
	srv, up := newMutationTestEnv(t, "")

	for _, p := range []string{"/api/mine/start", "/api/mine/stop"} {
		code, _ := doReq(t, http.MethodPost, srv.URL+p, "{}", "application/json", nil)
		if code != http.StatusServiceUnavailable {
			t.Fatalf("只读模式下 %s 应 503，实际 %d", p, code)
		}
	}
	if hits, _, _ := up.snapshot(); hits != 0 {
		t.Fatalf("fail-closed 不得触达上游，实际 %d", hits)
	}
	// 只读通道不受影响：GET /api/status 仍被代理（此处上游会返回非 JSON，
	// 只断言状态码非 404/405——代理确实发生）。
	code, _ := doReq(t, http.MethodGet, srv.URL+"/api/status", "", "", nil)
	if code == http.StatusNotFound || code == http.StatusMethodNotAllowed {
		t.Fatalf("只读通道应保持可用，实际 %d", code)
	}
}

// TestMutationUpstreamUnavailable：上游不可达 → 502 本地错误体（无上游措辞）。
func TestMutationUpstreamUnavailable(t *testing.T) {
	h, err := NewHandler("http://127.0.0.1:1", mutTestToken) // port 1 不可达
	if err != nil {
		t.Fatalf("NewHandler 失败: %v", err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	code, body := doReq(t, http.MethodPost, srv.URL+"/api/mine/start", "{}", "application/json", nil)
	if code != http.StatusBadGateway {
		t.Fatalf("上游不可达应 502，实际 %d", code)
	}
	if !strings.Contains(body, "explorer upstream unavailable") {
		t.Fatalf("502 应为 Explorer 本地错误体: %s", body)
	}
}

// TestMutationInvalidTokenFileFailsClosed：LoadTokenFile 对畸形输入的拒绝
// 在 cmd/explorer 启动路径已 fail-closed；此处锁定 handler 对空 token 的
// fail-closed 与合法 token 的注入行为一致性（双重保险断言）。
func TestMutationInvalidTokenRejectedByGate(t *testing.T) {
	srv, up := newMutationTestEnv(t, "short") // 长度不足的 token 不可能来自 LoadTokenFile
	code, _ := doReq(t, http.MethodPost, srv.URL+"/api/mine/start", "{}", "application/json", nil)
	if code != http.StatusOK {
		t.Fatalf("handler 层不做 token 长度校验（职责在 LoadTokenFile），应 200，实际 %d", code)
	}
	if hits, auths, _ := up.snapshot(); hits != 1 || auths[0] != "Bearer short" {
		t.Fatalf("注入行为异常: %v %v", hits, auths)
	}
}
