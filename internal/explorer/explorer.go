// Package explorer 提供 P2PChain Explorer V1 的独立进程壳：
//
//	浏览器 → Explorer(127.0.0.1:9091) → control(127.0.0.1:17881) → Node
//
// 职责严格限定为两件事：
//  1. 通过 go:embed 提供 vanilla HTML/CSS/JS 静态 UI（零第三方依赖）；
//  2. 以【显式只读路由集】把仅有的三个 GET 数据端点转发到本机 control
//     接口（唯一数据通道，禁止任何 blocks.dat / blockchain 内部 / storage
//     直接访问）。
//
// F-1 边界（PHASE EXPLORER-F1-REMEDIATION-DESIGN 冻结 / OPTION A + P1 + GET-only）：
//   - Explorer 不提供通用 /api/* 反向代理。未命中 readOnlyRoutes 的 /api/*
//     请求（含 /mine /send /stop /console 与 /balance /utxos /logs 等）
//     一律由 Explorer 本地拒绝，【上游零接触】（upstreamHits == 0）。
//   - Explorer API 数据面为 GET-only（冻结决策 D-HEAD：不提供 HEAD 能力）。
//     错误方法在 Explorer 边缘被拒，不依赖上游 control 的 requireMethod 守卫。
//   - Node 控制面本身（127.0.0.1:17881）仍然存在且无鉴权，其加固为独立项
//     F-2（OPEN）。本包不涉及、不得据此宣称控制面已安全或 MODE B 就绪。
//
// 本包不 import blockchain/storage/wallet 等核心包——Explorer 与链核心
// 完全隔离（PHASE EXPLORER-UI-READINESS-AUDIT §4 DATA SOURCE MATRIX 冻结）。
package explorer

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

//go:embed ui
var uiFS embed.FS

// readOnlyRoutes 是 Explorer 允许代理的唯一上游映射（冻结白名单）。
// key = Explorer 暴露的路径；value = 上游 control 路径。
// 新增条目 = 显式扩大 Explorer 攻击面，必须走独立安全授权。
var readOnlyRoutes = map[string]string{
	"/api/status": "/status",
	"/api/blocks": "/blocks",
	"/api/block":  "/block",
}

// NewHandler 返回 Explorer 完整 HTTP handler：
//   - GET /api/status、/api/blocks、/api/block → 反向代理到 controlAddr
//     （前缀语义由白名单显式给出，不再做通用前缀变换）；
//   - 其余 /api/*（任何方法：控制端点、敏感读、未知路径）→ Explorer 本地拒绝，
//     不产生任何上游请求（F-1 上游零接触不变量）；
//   - 其余路径 → 静态 UI（SPA 使用 hash 路由，统一回落到 index.html）。
func NewHandler(controlAddr string) (http.Handler, error) {
	target, err := url.Parse(controlAddr)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		// 仅 readOnlyRoutes 中的路径可达此处（ServeMux 按表分发）。
		// 查表赋值取代通用 TrimPrefix 变换：未命中即不存在可构造的下游路径。
		upstream := readOnlyRoutes[req.URL.Path]
		origDirector(req)
		req.URL.Path = upstream
		req.Host = target.Host
	}
	// 上游不可达时返回 502（UI 据此显示 Explorer API unavailable，而非静默）。
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		writeLocalError(w, http.StatusBadGateway, "explorer upstream unavailable")
	}

	mux := http.NewServeMux()
	for exposed := range readOnlyRoutes {
		mux.Handle(exposed, methodGate(proxy))
	}
	// 白名单之外的 /api/* → 本地 404，上游零接触。
	// 注意：这些路径【未注册】精确模式，因此任何方法（含 GET）都是 404；
	// 405 + Allow: GET 只出现在三条已注册读路径收到错误方法时。
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeLocalError(w, http.StatusNotFound, "endpoint 不在 Explorer 只读白名单内")
	})
	mux.Handle("/", staticHandler())
	return mux, nil
}

// methodGate 在 Explorer 边缘把方法收紧为 GET（冻结决策 D-HEAD）。
// 守卫位于 Explorer 自身，而非上游——请求一旦被转发，边界已被越过。
func methodGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			writeLocalError(w, http.StatusMethodNotAllowed, "Explorer 只读接口仅允许 GET 方法")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeLocalError 输出 Explorer【本地】JSON 错误体。
// 回归测试据此区分「本地拒绝」与「转发后被上游拒绝」：本地错误体不包含
// 任何上游措辞（上游 requireMethod 的错误串为「仅支持 <METHOD>」）。
func writeLocalError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
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
