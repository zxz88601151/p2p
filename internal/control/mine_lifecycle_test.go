package control_test

// PHASE MINING-LIFECYCLE-1：/mine/start 与 /mine/stop 的 HTTP 契约测试。
//
// 覆盖（设计冻结 §10 API 契约）：
//   - 认证：无 token / 错 token / 未配置 token ⇒ 401 fail-closed；
//   - 方法：GET ⇒ 405 + Allow: POST；
//   - 请求体：空体/{} 合法；未知字段 ⇒ 400（DisallowUnknownFields 冻结）；
//   - START：200 响应体 {"accepted","state","height"}；冲突 ⇒ 409 {"error","state"}；
//     其他错误 ⇒ 500；
//   - STOP：200 幂等 {"accepted","state"}。
//
// 测试替身 fakeNode（server_test.go）仅协议层，与真实挖矿语义无关——
// 真实生命周期语义在 cmd/node/mining_lifecycle_test.go 验证。

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"p2pchain/internal/control"
)

func newMineTestServer(node control.Node) *httptest.Server {
	s := control.NewServer(node)
	s.SetAuthToken(testToken)
	return httptest.NewServer(s.Handler())
}

// postMine 以给定 header/body 发送 POST，返回状态码与解码后的 JSON 对象。
func postMine(t *testing.T, url, token, body string, extraHeaders map[string]string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer res.Body.Close()
	var obj map[string]any
	_ = json.NewDecoder(res.Body).Decode(&obj)
	return res.StatusCode, obj
}

func TestMineStartContract(t *testing.T) {
	node := &fakeNode{
		mineStartResp: control.MineStartResponse{Accepted: true, State: "STARTING", Height: 7},
	}
	ts := newMineTestServer(node)
	defer ts.Close()

	// 401：无 token。
	if code, _ := postMine(t, ts.URL+"/mine/start", "", "{}", nil); code != http.StatusUnauthorized {
		t.Fatalf("无 token 应 401，实际 %d", code)
	}
	// 401：错 token。
	if code, _ := postMine(t, ts.URL+"/mine/start", "wrong-token-0123456789", "{}", nil); code != http.StatusUnauthorized {
		t.Fatalf("错 token 应 401，实际 %d", code)
	}
	// 405：GET（requireMethod 守卫）。
	res, err := http.Get(ts.URL + "/mine/start")
	if err != nil {
		t.Fatalf("GET 请求失败: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /mine/start 应 405，实际 %d", res.StatusCode)
	}
	if allow := res.Header.Get("Allow"); allow != "POST" {
		t.Fatalf("Allow 头应为 POST，实际 %q", allow)
	}
	// 400：未知字段（DisallowUnknownFields 冻结）。
	if code, _ := postMine(t, ts.URL+"/mine/start", testToken, `{"foo":1}`, nil); code != http.StatusBadRequest {
		t.Fatalf("未知字段应 400，实际 %d", code)
	}
	// 200：空体合法。
	code, body := postMine(t, ts.URL+"/mine/start", testToken, "{}", nil)
	if code != http.StatusOK {
		t.Fatalf("合法 START 应 200，实际 %d（%v）", code, body)
	}
	if body["accepted"] != true || body["state"] != "STARTING" {
		t.Fatalf("响应体不符: %v", body)
	}
	if h, ok := body["height"].(float64); !ok || h != 7 {
		t.Fatalf("height 应为 7: %v", body)
	}
	// 409：冲突错误映射 {"error","state"}。
	node.mineStartErr = &control.MineConflictError{Message: "挖矿已在运行", State: "RUNNING"}
	code, body = postMine(t, ts.URL+"/mine/start", testToken, "{}", nil)
	if code != http.StatusConflict {
		t.Fatalf("冲突应 409，实际 %d", code)
	}
	if body["state"] != "RUNNING" || body["error"] == nil {
		t.Fatalf("409 响应体不符: %v", body)
	}
	// 500：非冲突内部错误。
	node.mineStartErr = errors.New("boom")
	if code, _ := postMine(t, ts.URL+"/mine/start", testToken, "{}", nil); code != http.StatusInternalServerError {
		t.Fatalf("内部错误应 500，实际 %d", code)
	}
}

func TestMineStopContract(t *testing.T) {
	node := &fakeNode{
		mineStopResp: control.MineStopResponse{Accepted: true, State: "STOPPING"},
	}
	ts := newMineTestServer(node)
	defer ts.Close()

	// 401：无 token（fail-closed 不因 stop 幂等而放宽）。
	if code, _ := postMine(t, ts.URL+"/mine/stop", "", "{}", nil); code != http.StatusUnauthorized {
		t.Fatalf("无 token 应 401，实际 %d", code)
	}
	// 200：幂等 STOP。
	code, body := postMine(t, ts.URL+"/mine/stop", testToken, "{}", nil)
	if code != http.StatusOK {
		t.Fatalf("STOP 应 200，实际 %d", code)
	}
	if body["accepted"] != true || body["state"] != "STOPPING" {
		t.Fatalf("响应体不符: %v", body)
	}
	// 200：FAILED 状态如实返回（证据保留语义）。
	node.mineStopResp = control.MineStopResponse{Accepted: true, State: "FAILED"}
	_, body = postMine(t, ts.URL+"/mine/stop", testToken, "{}", nil)
	if body["state"] != "FAILED" {
		t.Fatalf("FAILED 状态应如实返回: %v", body)
	}
	// 405：GET。
	res, err := http.Get(ts.URL + "/mine/stop")
	if err != nil {
		t.Fatalf("GET 请求失败: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /mine/stop 应 405，实际 %d", res.StatusCode)
	}
}

// TestMineMutationFailClosedWhenNoToken：token 未配置时 mutation 一律 401
// （既有 fail-closed 契约在新端点上保持不变）。
func TestMineMutationFailClosedWhenNoToken(t *testing.T) {
	s := control.NewServer(&fakeNode{})
	// 不调用 SetAuthToken ⇒ 未配置。
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	for _, p := range []string{"/mine/start", "/mine/stop"} {
		code, _ := postMine(t, ts.URL+p, testToken, "{}", nil)
		if code != http.StatusUnauthorized {
			t.Fatalf("未配置 token 时 %s 应 401，实际 %d", p, code)
		}
	}
}
