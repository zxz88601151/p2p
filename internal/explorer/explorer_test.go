package explorer

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// fakeControl 模拟本机 control 接口，验证代理的路径改写与透传。
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
