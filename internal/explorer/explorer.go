// Package explorer 提供 P2PChain Explorer V1 的独立进程壳：
//
//	浏览器 → Explorer(127.0.0.1:9091) → control(127.0.0.1:17881) → Node
//
// 职责严格限定为两件事：
//  1. 通过 go:embed 提供 vanilla HTML/CSS/JS 静态 UI（零第三方依赖）；
//  2. 将 /api/* 请求反向代理到本机 control 接口（唯一数据通道，禁止任何
//     blocks.dat / blockchain 内部 / storage 直接访问）。
//
// 本包不 import blockchain/storage/wallet 等核心包——Explorer = READ，
// 与链核心完全隔离（PHASE EXPLORER-UI-READINESS-AUDIT §4 DATA SOURCE MATRIX 冻结）。
package explorer

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

//go:embed ui
var uiFS embed.FS

// NewHandler 返回 Explorer 完整 HTTP handler：
//   - /api/* → 反向代理到 controlAddr（如 http://127.0.0.1:17881），前缀剥离；
//   - 其余路径 → 静态 UI（SPA 使用 hash 路由，统一回落到 index.html）。
func NewHandler(controlAddr string) (http.Handler, error) {
	target, err := url.Parse(controlAddr)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		// /api/status → /status：剥离 API 前缀后转发给 control。
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/api")
		req.Host = target.Host
	}
	// 上游不可达时返回 502（UI 据此显示 Explorer API unavailable，而非静默）。
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, `{"error":"explorer upstream unavailable"}`, http.StatusBadGateway)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", proxy)
	mux.Handle("/", staticHandler())
	return mux, nil
}

// staticHandler 服务嵌入的静态 UI；非 /api 的未知路径统一回落 index.html
// （hash 路由由前端解析，malformed route 由前端优雅呈现，不产生服务端 404 白屏）。
func staticHandler() http.Handler {
	sub, err := fs.Sub(uiFS, "ui")
	if err != nil {
		// embed 路径是编译期常量，此分支不可达；防御性返回空 FS。
		sub = errorFS{}
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Explorer 资源随二进制 go:embed 发版，禁止启发式缓存（防旧 JS/CSS 停留）。
		w.Header().Set("Cache-Control", "no-store")
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(sub, p); err != nil {
			// 未知路径 → SPA 入口。
			r2 := new(http.Request)
			*r2 = *r
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

type errorFS struct{}

func (errorFS) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }
