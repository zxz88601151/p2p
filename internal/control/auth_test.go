package control_test

// PHASE CONTROL-AUTH-1：mutation 端点 Bearer Token 认证测试矩阵。
//
// 冻结契约（CONTROL-PLANE AUTH DESIGN / IMPLEMENTATION）：
//   - 读端点（GET /status /balance /utxos /block /blocks /logs）无 token = 原有行为；
//   - mutation 端点（POST /send /mine /stop）：missing/invalid/wrong token → 统一 401；
//   - 有效 token → 进入既有 handler（原有行为不变）；
//   - 认证失败固定延迟（本文件中单独用例验证），无锁定/黑名单/全局限流；
//   - 失败响应不回显 token、无 panic/stack trace；
//   - 未配置 token 时 fail-closed（mutation 一律 401）。

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"p2pchain/internal/control"
)

const (
	goodToken   = "auth-test-token-0123456789abcdef"
	wrongToken  = "auth-test-token-ffffffffffffffff"
	invalidForm = "not-a-bearer"
)

// newAuthServer 起一个带 token 的 httptest 服务；delay 可覆写失败延迟。
func newAuthServer(t *testing.T, node control.Node, token string, delay time.Duration) (*control.Client, *httptest.Server) {
	t.Helper()
	s := control.NewServer(node)
	if token != "" {
		s.SetAuthToken(token)
	}
	if delay >= 0 {
		s.SetAuthFailureDelay(delay)
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	c := control.NewClient(strings.TrimPrefix(srv.URL, "http://"))
	return c, srv
}

// rawPost 发送可自定义 Authorization 的 POST；auth 为空则不带该头。
func rawPost(t *testing.T, url, auth, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// --- 读端点：无 token = 原有行为 ---

func TestReadEndpointsWithoutTokenUnchanged(t *testing.T) {
	node := &fakeNode{
		status:  control.StatusInfo{Height: 9, Address: "ADDR"},
		balance: control.BalanceInfo{Spendable: 1},
		blocks:  control.BlocksPageResult{From: 0, Returned: 0, AtTip: true, Height: 9},
	}
	_, srv := newAuthServer(t, node, goodToken, 0) // 服务端要求 token

	cases := map[string]int{
		"/status":                http.StatusOK,
		"/balance?address=A1":    http.StatusOK,
		"/utxos?address=A1":      http.StatusOK,
		"/block?height=1":        http.StatusOK,
		"/blocks?from=0&count=1": http.StatusOK,
		"/logs":                  http.StatusOK,
	}
	for path, want := range cases {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("GET %s = %d, want %d（读端点不得要求 token）", path, resp.StatusCode, want)
		}
	}
}

// --- mutation 端点：missing / invalid / wrong → 统一 401 ---

func TestMutationAuthMatrix(t *testing.T) {
	cases := []struct {
		name string
		path string
		body string
	}{
		{"send", "/send", `{"to":"A1","amount":1,"fee":0}`},
		// 原为 /mine（按需出块）。该端点已整体下线，矩阵改以 /mine/start 承载 ——
		// 它同为 mutation 端点、同经 requireAuth，认证矩阵的覆盖强度不变。
		{"mine-start", "/mine/start", `{}`},
		{"stop", "/stop", `{}`},
	}
	authCases := []struct {
		name string
		auth string
	}{
		{"missing", ""},
		{"malformed", "Bearer"},
		{"malformed-scheme", "Basic " + goodToken},
		{"invalid-token", "Bearer " + invalidForm},
		{"wrong-token", "Bearer " + wrongToken},
		{"empty-bearer", "Bearer "},
	}
	for _, tc := range cases {
		for _, ac := range authCases {
			t.Run(tc.name+"/"+ac.name, func(t *testing.T) {
				_, srv := newAuthServer(t, &fakeNode{}, goodToken, 0)
				resp := rawPost(t, srv.URL+tc.path, ac.auth, tc.body)
				if resp.StatusCode != http.StatusUnauthorized {
					t.Fatalf("%s auth=%q = %d, want 401", tc.path, ac.auth, resp.StatusCode)
				}
				body := readBody(t, resp)
				if strings.Contains(body, goodToken) || (ac.auth != "" && strings.Contains(body, ac.auth)) {
					t.Fatalf("响应回显了凭据: %s", body)
				}
			})
		}
	}
}

// --- 有效 token → 进入既有 handler ---

func TestMutationValidTokenReachesHandler(t *testing.T) {
	node := &fakeNode{}
	_, srv := newAuthServer(t, node, goodToken, 0)

	// /mine/start：有效 token → 既有行为（200，StartMining 被调用）
	// 原用例用 /mine（按需出块）验证「认证通过后进入既有 handler」；
	// 该端点已下线，改由 /mine/start 承载同样的证明。
	node.mineStartResp = control.MineStartResponse{Accepted: true, State: "STARTING", Height: 3}
	resp := rawPost(t, srv.URL+"/mine/start", "Bearer "+goodToken, `{}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/mine/start 有效 token = %d, want 200", resp.StatusCode)
	}
	if body := readBody(t, resp); !strings.Contains(body, `"accepted":true`) {
		t.Fatalf("/mine/start 未到达 handler，响应体 = %s", body)
	}

	// /send：有效 token → 既有行为
	resp = rawPost(t, srv.URL+"/send", "Bearer "+goodToken, `{"to":"A1","amount":5,"fee":0}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/send 有效 token = %d, want 200", resp.StatusCode)
	}
	if node.lastTo != "A1" || node.lastAmount != 5 {
		t.Fatalf("Send 未到达 handler: to=%q amount=%d", node.lastTo, node.lastAmount)
	}

	// /stop：无 stopHook 注入时既有行为 = 501（证明已通过认证层进入 handler）
	resp = rawPost(t, srv.URL+"/stop", "Bearer "+goodToken, `{}`)
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("/stop 有效 token = %d, want 501（未注入 hook 的既有行为）", resp.StatusCode)
	}
}

// --- fail-closed：未配置 token → mutation 一律 401 ---

func TestMutationFailClosedWhenNoTokenConfigured(t *testing.T) {
	_, srv := newAuthServer(t, &fakeNode{}, "", 0)
	for _, path := range []string{"/send", "/mine/start", "/mine/stop", "/stop"} {
		resp := rawPost(t, srv.URL+path, "Bearer "+goodToken, `{}`)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s 未配置 token = %d, want 401（fail-closed）", path, resp.StatusCode)
		}
	}
}

// --- 固定延迟：失败响应耗时 >= 配置延迟；无锁定/无升级 ---

func TestAuthFailureFixedDelay(t *testing.T) {
	const delay = 80 * time.Millisecond
	_, srv := newAuthServer(t, &fakeNode{}, goodToken, delay)

	start := time.Now()
	resp := rawPost(t, srv.URL+"/mine/start", "Bearer "+invalidForm, `{}`)
	elapsed := time.Since(start)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("状态码 = %d, want 401", resp.StatusCode)
	}
	if elapsed < delay {
		t.Fatalf("失败响应耗时 %v < 固定延迟 %v", elapsed, delay)
	}
}

// --- 方法守卫在认证之前不成立时的既有 405 行为保持（读端点） ---

func TestReadMethodGuardUnchanged(t *testing.T) {
	_, srv := newAuthServer(t, &fakeNode{}, goodToken, 0)
	resp := rawPost(t, srv.URL+"/status", "", `{}`)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST /status = %d, want 405（读端点方法守卫不变）", resp.StatusCode)
	}
}

// --- 方法守卫优先于认证：GET /stop 保持既有 405（不因新增认证而变为 401） ---

func TestMutationMethodGuardPrecedesAuth(t *testing.T) {
	_, srv := newAuthServer(t, &fakeNode{}, goodToken, 0)
	resp, err := http.Get(srv.URL + "/stop")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /stop = %d, want 405（既有方法契约不变）", resp.StatusCode)
	}
	if got := resp.Header.Get("Allow"); got != http.MethodPost {
		t.Fatalf("Allow 头 = %q, want POST", got)
	}
}

// --- LoadTokenFile：归一化与权限语义 ---

func TestLoadTokenFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("valid_with_trailing_newline", func(t *testing.T) {
		p := write("t1", goodToken+"\n")
		got, err := control.LoadTokenFile(p)
		if err != nil || got != goodToken {
			t.Fatalf("got %q err %v, want %q", got, err, goodToken)
		}
	})
	t.Run("crlf_normalized", func(t *testing.T) {
		p := write("t2", goodToken+"\r\n")
		got, err := control.LoadTokenFile(p)
		if err != nil || got != goodToken {
			t.Fatalf("got %q err %v, want %q", got, err, goodToken)
		}
	})
	t.Run("surrounding_space_trimmed", func(t *testing.T) {
		p := write("t3", "  "+goodToken+"  ")
		got, err := control.LoadTokenFile(p)
		if err != nil || got != goodToken {
			t.Fatalf("got %q err %v, want %q", got, err, goodToken)
		}
	})
	t.Run("empty_file_rejected", func(t *testing.T) {
		p := write("t4", "")
		if _, err := control.LoadTokenFile(p); err == nil {
			t.Fatal("空文件应被拒绝")
		}
	})
	t.Run("whitespace_only_rejected", func(t *testing.T) {
		p := write("t5", " \r\n\t ")
		if _, err := control.LoadTokenFile(p); err == nil {
			t.Fatal("纯空白文件应被拒绝")
		}
	})
	t.Run("too_short_rejected", func(t *testing.T) {
		p := write("t6", "short")
		if _, err := control.LoadTokenFile(p); err == nil {
			t.Fatal("过短 token 应被拒绝")
		}
	})
	t.Run("too_long_rejected", func(t *testing.T) {
		p := write("t7", strings.Repeat("a", 2000))
		if _, err := control.LoadTokenFile(p); err == nil {
			t.Fatal("超长 token 应被拒绝")
		}
	})
	t.Run("missing_file_rejected", func(t *testing.T) {
		if _, err := control.LoadTokenFile(filepath.Join(dir, "nonexistent")); err == nil {
			t.Fatal("缺失文件应被拒绝")
		}
	})
	if runtime.GOOS != "windows" {
		t.Run("wide_permissions_rejected_unix", func(t *testing.T) {
			p := write("t8", goodToken)
			if err := os.Chmod(p, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := control.LoadTokenFile(p); err == nil {
				t.Fatal("0644 权限应被拒绝")
			}
		})
	}
}

// readBody 读取响应体（用于断言不回显凭据）。
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	return string(buf[:n])
}
