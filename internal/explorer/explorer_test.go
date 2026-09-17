package explorer

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeControl 模拟本机 control 接口，验证代理的路径改写与透传（既有正向测试用）。
func fakeControl(t *testing.T, gotPath *string, gotQuery *url.Values) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotPath = r.URL.Path
		*gotQuery = r.URL.Query()
		switch r.URL.Path {
		case "/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"height": 42, "tip_hash": strings.Repeat("ab", 32), "peers": []string{},
				"mempool_size": 0, "mining": false, "mining_state": "STOPPED",
			})
		case "/blocks":
			if r.URL.Query().Get("from") == "" || r.URL.Query().Get("count") == "" {
				http.Error(w, `{"error":"missing params"}`, http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"from": 40, "returned": 2, "at_tip": true, "height": 42, "blocks": []any{}})
		default:
			http.Error(w, `{"error":"no route"}`, http.StatusNotFound)
		}
	}))
}

func TestProxyRewritesAPIPrefix(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	ctrl := fakeControl(t, &gotPath, &gotQuery)
	defer ctrl.Close()

	h, err := NewHandler(ctrl.URL)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	// /api/status → control /status
	resp, err := http.Get(srv.URL + "/api/status")
	if err != nil {
		t.Fatalf("GET /api/status: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if gotPath != "/status" {
		t.Fatalf("upstream path = %q, want /status（前缀必须剥离）", gotPath)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["height"].(float64) != 42 {
		t.Fatalf("height = %v, want 42（透传）", body["height"])
	}
}

func TestProxyForwardsQueryParams(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	ctrl := fakeControl(t, &gotPath, &gotQuery)
	defer ctrl.Close()

	h, _ := NewHandler(ctrl.URL)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/blocks?from=40&count=20")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if gotPath != "/blocks" {
		t.Fatalf("upstream path = %q, want /blocks", gotPath)
	}
	if gotQuery.Get("from") != "40" || gotQuery.Get("count") != "20" {
		t.Fatalf("query 未透传: %v", gotQuery)
	}
}

func TestProxyUpstreamUnavailable502(t *testing.T) {
	// 不可达端口 → 502（UI 据此显示 Explorer API unavailable）
	h, _ := NewHandler("http://127.0.0.1:1") // port 1 不可达
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/status")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
}

func TestStaticServesIndexAndSPAFallback(t *testing.T) {
	h, _ := NewHandler("http://127.0.0.1:1")
	srv := httptest.NewServer(h)
	defer srv.Close()

	// "/" → index.html
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body := new(strings.Builder)
	_, _ = io.Copy(body, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(body.String(), "P2PChain Explorer") {
		t.Fatalf("/ 未正确服务 index.html: %d %q", resp.StatusCode, body.String())
	}

	// 未知路径 → SPA 回落 index.html（hash 路由由前端优雅处理）
	resp2, err := http.Get(srv.URL + "/some/unknown/path")
	if err != nil {
		t.Fatalf("GET unknown: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("unknown path status = %d, want 200（SPA 回落）", resp2.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// F-1 回归套件（OPTION A + P1 + GET-only）
// 核心原则：状态码不能证明「未转发」。每条被阻断用例必须断言 upstreamHits == 0，
// 且响应体不含上游措辞（fake 上游的标记串是 "upstream no route"，
// 真实上游 requireMethod 的标记串是 "仅支持"）。
// ---------------------------------------------------------------------------

// countingUpstream 记录上游收到的每一次请求（次数 / 路径 / 方法 / 查询串）。
type countingUpstream struct {
	hits    atomic.Int64
	mu      sync.Mutex
	paths   map[string]int
	methods map[string]int
	queries map[string]int
	srv     *httptest.Server
}

func newCountingUpstream(t *testing.T) *countingUpstream {
	t.Helper()
	cu := &countingUpstream{paths: map[string]int{}, methods: map[string]int{}, queries: map[string]int{}}
	cu.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cu.hits.Add(1)
		cu.mu.Lock()
		cu.paths[r.URL.Path]++
		cu.methods[r.Method]++
		cu.queries[r.URL.RawQuery]++
		cu.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/status", "/blocks", "/block":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "upstream_path": r.URL.Path})
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"upstream no route"}`))
		}
	}))
	t.Cleanup(cu.srv.Close)
	return cu
}

func (cu *countingUpstream) count() int64 { return cu.hits.Load() }

// localBodyMarkers 判定响应体确为 Explorer 本地产出（而非上游透传）。
func localBodyMarkers(t *testing.T, body string) {
	t.Helper()
	if strings.Contains(body, "upstream no route") {
		t.Fatalf("响应体含 fake 上游标记串 ⇒ 请求被转发到了上游（违反上游零接触）")
	}
	if strings.Contains(body, "仅支持") {
		t.Fatalf("响应体含上游 requireMethod 措辞「仅支持」⇒ 405 来自上游而非本地")
	}
}

func newTestExplorer(t *testing.T, cu *countingUpstream) *httptest.Server {
	t.Helper()
	h, err := NewHandler(cu.srv.URL)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, srv *httptest.Server, method, path string, hdr map[string]string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("NewRequest %s %s: %v", method, path, err)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(b)
}

// TestAllowedRoutesReachUpstream：白名单三条 GET 必须到达上游且路径/查询精确。
func TestAllowedRoutesReachUpstream(t *testing.T) {
	cu := newCountingUpstream(t)
	srv := newTestExplorer(t, cu)

	cases := []struct {
		path     string
		upstream string
		rawQuery string // 期望到达上游的原始查询串（"" = 无）
	}{
		{"/api/status", "/status", ""},
		{"/api/blocks?from=40&count=20", "/blocks", "from=40&count=20"},
		{"/api/block?hash=" + strings.Repeat("ab", 32), "/block", "hash=" + strings.Repeat("ab", 32)},
	}
	for _, c := range cases {
		before := cu.count()
		code, _, body := do(t, srv, http.MethodGet, c.path, nil)
		if code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200（白名单路径不得被误拒）", c.path, code)
		}
		if cu.count() != before+1 {
			t.Fatalf("GET %s: upstreamHits = %d, want +1", c.path, cu.count()-before)
		}
		if !strings.Contains(body, `"upstream_path":"`+c.upstream+`"`) {
			t.Fatalf("GET %s: 上游收到的不是 %s（body=%s）", c.path, c.upstream, body)
		}
		cu.mu.Lock()
		_, qSeen := cu.queries[c.rawQuery]
		cu.mu.Unlock()
		if c.rawQuery != "" && !qSeen {
			t.Fatalf("GET %s: 期望上游收到 query %q，实测未见（query 丢失或被改写）", c.path, c.rawQuery)
		}
	}
}

// TestForbiddenEndpointsZeroUpstreamContact：禁用端点 × 全方法 ⇒ 本地 404 + 上游 0 次。
func TestForbiddenEndpointsZeroUpstreamContact(t *testing.T) {
	cu := newCountingUpstream(t)
	srv := newTestExplorer(t, cu)

	endpoints := []string{
		"/api/mine", "/api/send", "/api/stop",
		"/api/console", "/api/balance", "/api/utxos", "/api/logs",
		"/api/unknown", "/api/",
	}
	methods := []string{
		http.MethodGet, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodHead,
	}
	for _, ep := range endpoints {
		for _, m := range methods {
			before := cu.count()
			code, _, body := do(t, srv, m, ep, nil)
			if code != http.StatusNotFound {
				t.Fatalf("%s %s = %d, want 404（本地拒绝，未注册路径任何方法都是 404）", m, ep, code)
			}
			if cu.count() != before {
				t.Fatalf("%s %s: upstreamHits = %d, want 0（必须本地拒绝）", m, ep, cu.count()-before)
			}
			localBodyMarkers(t, body)
		}
	}
}

// TestRegisteredRoutesRejectNonGET：三条已注册读路径收到非 GET ⇒ 本地 405 + Allow: GET。
func TestRegisteredRoutesRejectNonGET(t *testing.T) {
	cu := newCountingUpstream(t)
	srv := newTestExplorer(t, cu)

	for _, p := range []string{"/api/status", "/api/blocks", "/api/block"} {
		for _, m := range []string{
			http.MethodPost, http.MethodPut, http.MethodPatch,
			http.MethodDelete, http.MethodOptions, http.MethodHead,
		} {
			before := cu.count()
			code, hdr, body := do(t, srv, m, p, nil)
			if code != http.StatusMethodNotAllowed {
				t.Fatalf("%s %s = %d, want 405（D-HEAD：方法闸门在 Explorer 边缘）", m, p, code)
			}
			if got := hdr.Get("Allow"); got != "GET" {
				t.Fatalf("%s %s: Allow = %q, want GET", m, p, got)
			}
			if cu.count() != before {
				t.Fatalf("%s %s: upstreamHits = %d, want 0（方法闸门必须在转发前拒绝）", m, p, cu.count()-before)
			}
			localBodyMarkers(t, body)
			// HEAD 响应按 HTTP 语义无 body（net/http 抑制），仅非 HEAD 断言本地措辞。
			if m != http.MethodHead && !strings.Contains(body, "仅允许 GET") {
				t.Fatalf("%s %s: body = %q, want 本地方法闸门措辞", m, p, body)
			}
		}
	}
}

// rawRequest 以原始请求行直发 TCP（绕过客户端路径规范化）。
func rawRequest(t *testing.T, addr, rawPath, method string) string {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	fmt.Fprintf(c, "%s %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", method, rawPath, addr)
	b, _ := io.ReadAll(c)
	return string(b)
}

// TestPathBoundaryRawTCP：路径操纵不得把禁用路径变成允许路径。
// 期望值 = 实测行为（先测量后固化），非对 ServeMux 的假设性推导。
func TestPathBoundaryRawTCP(t *testing.T) {
	cu := newCountingUpstream(t)
	srv := newTestExplorer(t, cu)
	addr := strings.TrimPrefix(srv.URL, "http://")

	cases := []struct {
		note      string
		rawPath   string
		wantCode  string // 响应行中的状态码
		wantHits  int64
		wantInLoc string // 期望 Location 包含（可为空）
		spaOK     bool   // 允许 SPA 回落 200
	}{
		{"尾斜杠不匹配精确路由", "/api/status/", "404", 0, "", false},
		{"重复斜杠被清洗", "/api//status", "301", 0, "/api/status", false},
		{"点段被清洗", "/api/./status", "301", 0, "/api/status", false},
		{"穿越被清洗出 /api 前缀", "/api/../status", "301", 0, "/status", false},
		// 实测固化：path.Clean("/api/../api/status") = "/api/status"（.. 先在段间消解），
		// ServeMux 301 重定向到清洗后的 /api/status，原始请求本身【不】产生上游命中。
		// 若客户端跟随重定向再 GET，则属第二次请求，按白名单正常放行（hits=1）。
		{"穿越回 /api 先被 301 清洗", "/api/../api/status", "301", 0, "/api/status", false},
		// 实测固化（比设计假设更严格）：Go 1.22 ServeMux 按编码路径的【段】匹配，
		// %2f 不折叠为 / ⇒ "/api%2fstatus" 是单一未知段，不命中白名单，
		// 落入静态 handler → SPA 回落 200 index.html，上游 0 次命中。
		{"编码分隔符不折叠为段分隔", "/api%2fstatus", "200", 0, "", true},
		{"大小写敏感", "/api/STATUS", "404", 0, "", false},
		{"子树根重定向补斜杠", "/api", "301", 0, "/api/", false},
		{"非 /api 前缀不误剥", "/apix/status", "200", 0, "", true},
		{"双前导斜杠被清洗", "//api/status", "301", 0, "/api/status", false},
		{"NUL 不构成绕过", "/api/status%00", "404", 0, "", false},
	}
	for _, c := range cases {
		before := cu.count()
		resp := rawRequest(t, addr, c.rawPath, http.MethodGet)
		if !strings.HasPrefix(resp, "HTTP/1.1 "+c.wantCode) && !strings.Contains(resp, "HTTP/1.1 "+c.wantCode+" ") {
			t.Fatalf("%s: GET %s 响应行不含期望状态码 %s；实测响应头:\n%s", c.note, c.rawPath, c.wantCode, firstLines(resp, 8))
		}
		if c.wantInLoc != "" && !strings.Contains(resp, "Location: ") {
			t.Fatalf("%s: GET %s 期望重定向到 %s 但无 Location 头:\n%s", c.note, c.rawPath, c.wantInLoc, firstLines(resp, 8))
		}
		if c.wantInLoc != "" && !strings.Contains(resp, c.wantInLoc) {
			t.Fatalf("%s: GET %s Location 不含 %s:\n%s", c.note, c.rawPath, c.wantInLoc, firstLines(resp, 8))
		}
		if got := cu.count() - before; got != c.wantHits {
			t.Fatalf("%s: GET %s upstreamHits = %d, want %d", c.note, c.rawPath, got, c.wantHits)
		}
		if c.wantCode == "200" && !c.spaOK {
			// 命中白名单的 200 必须来自上游（由 wantHits=1 保证），此处无额外断言。
			_ = resp
		}
	}
}

// TestMethodOverrideHeaderIgnored：方法覆盖头不得改变 Explorer 的方法判定。
func TestMethodOverrideHeaderIgnored(t *testing.T) {
	cu := newCountingUpstream(t)
	srv := newTestExplorer(t, cu)

	cases := []struct {
		path     string
		wantCode int
	}{
		{"/api/stop", http.StatusNotFound},           // 未注册路径：任何方法都 404
		{"/api/mine", http.StatusNotFound},           // 未注册路径
		{"/api/status", http.StatusMethodNotAllowed}, // 已注册路径：POST 仍是 POST
	}
	for _, c := range cases {
		before := cu.count()
		code, hdr, body := do(t, srv, http.MethodPost, c.path, map[string]string{
			"X-HTTP-Method-Override": "GET",
		})
		if code != c.wantCode {
			t.Fatalf("POST %s (override=GET) = %d, want %d", c.path, code, c.wantCode)
		}
		if cu.count() != before {
			t.Fatalf("POST %s (override=GET): upstreamHits = %d, want 0", c.path, cu.count()-before)
		}
		if code == http.StatusMethodNotAllowed && hdr.Get("Allow") != "GET" {
			t.Fatalf("POST %s: Allow = %q, want GET", c.path, hdr.Get("Allow"))
		}
		localBodyMarkers(t, body)
	}
}

// TestForeignOriginHostRefererBehaviour：固化「Explorer 无来源/Host 策略」的现行为。
// 本测试【不声称】这是安全边界——来源策略缺失属于 F-2 / MODE B 范围（OPEN）。
func TestForeignOriginHostRefererBehaviour(t *testing.T) {
	cu := newCountingUpstream(t)
	srv := newTestExplorer(t, cu)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/status", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Host = "evil.example"
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("Referer", "http://evil.example/exploit")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || cu.count() != 1 {
		t.Fatalf("GET /api/status (evil Origin/Host) = %d hits=%d；期望按现行为放行（无来源策略，F-2 OPEN）", resp.StatusCode, cu.count())
	}
	if !strings.Contains(string(b), `"upstream_path":"/status"`) {
		t.Fatalf("上游未收到 /status: %s", b)
	}
}

// TestEmbeddedUINoMutationSurface：嵌入 UI 不得再引用任何控制端点或 POST。
func TestEmbeddedUINoMutationSurface(t *testing.T) {
	sub, err := fs.Sub(uiFS, "ui")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	appJS, err := fs.ReadFile(sub, "app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	indexHTML, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	js := string(appJS)
	html := string(indexHTML)

	for _, bad := range []string{`method: "POST"`, `"/mine"`, `"/send"`, `"/stop"`, `"/api/console"`} {
		if strings.Contains(js, bad) {
			t.Fatalf("app.js 含被禁调用 %q（F-1/P1：UI 不得构造任何 mutation 请求）", bad)
		}
	}
	if strings.Contains(js, `"POST"`) {
		t.Fatalf("app.js 仍含 POST 字面量（UI 应只构造 GET）")
	}
	for _, bad := range []string{"Mine 1 Block", "doMine"} {
		if strings.Contains(js, bad) || strings.Contains(html, bad) {
			t.Fatalf("UI 残留 Mine 控件痕迹 %q（P1：按钮与其调用须一并移除）", bad)
		}
	}
	if !strings.Contains(html, "READ-ONLY") {
		t.Fatalf("index.html 页脚缺少 READ-ONLY 声明")
	}
}

// firstLines 返回响应前 n 行（诊断输出用）。
func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
