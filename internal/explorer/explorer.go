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
//   - Explorer 不提供通用 /api/* 反向代理。未命中 readOnlyRoutes 与
//     mutationRoutes 的 /api/* 请求（含 /send /stop /console 与 /balance
//     /utxos /logs 等）一律由 Explorer 本地拒绝，【上游零接触】（upstreamHits == 0）。
//   - Explorer API 数据面为 GET-only（冻结决策 D-HEAD：不提供 HEAD 能力）。
//     错误方法在 Explorer 边缘被拒，不依赖上游 control 的 requireMethod 守卫。
//   - Node 控制面本身（127.0.0.1:17881）的鉴权已于 PHASE CONTROL-AUTH-1 落地
//     （mutation 端点 Bearer Token）。LAN/公网暴露仍为独立项 F-2（OPEN）。
//
// PHASE MINING-LIFECYCLE-1（方案 A 冻结）：新增 mutationRoutes 白名单——
// 仅 POST /api/mine/start、/api/mine/stop 两条；token 只存在于 Explorer
// 进程内（-token-file 读入），转发时在服务端注入 Authorization，浏览器
// 零接触（禁止 JS/localStorage/URL/HTML/cookie 任何通道）。
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

// readOnlyRoutes 是 Explorer 允许代理的只读上游映射（冻结白名单）。
// key = Explorer 暴露的路径；value = 上游 control 路径。
// 新增条目 = 显式扩大 Explorer 攻击面，必须走独立安全授权。
var readOnlyRoutes = map[string]string{
	"/api/status": "/status",
	"/api/blocks": "/blocks",
	"/api/block":  "/block",
}

// mutationRoutes 是 Explorer 允许代理的 mutation 上游映射（PHASE
// MINING-LIFECYCLE-1 方案 A 冻结白名单）。**仅此两条**：/send（价值转移）、
// /stop（节点停机）、/console 等一律不在表内 ⇒ 本地 404，上游零接触。
// 新增条目 = 显式扩大 Explorer 攻击面，必须走独立安全授权。
var mutationRoutes = map[string]string{
	"/api/mine/start": "/mine/start",
	"/api/mine/stop":  "/mine/stop",
}

// NewHandler 返回 Explorer 完整 HTTP handler：
//   - GET /api/status、/api/blocks、/api/block → 反向代理到 controlAddr
//     （前缀语义由白名单显式给出，不再做通用前缀变换）；
//   - POST /api/mine/start、/api/mine/stop → 服务端注入 Bearer token 后代理
//     （token=="" 时 fail-closed：本地 503，不发上游请求）；
//   - 其余 /api/*（任何方法：控制端点、敏感读、未知路径）→ Explorer 本地拒绝，
//     不产生任何上游请求（F-1 上游零接触不变量）；
//   - 其余路径 → 静态 UI（SPA 使用 hash 路由，统一回落到 index.html）。
//
// mutationToken 为上游 mutation 端点的 Bearer Token：只存在于本进程内存，
// 永不出现在任何响应体 / URL / 日志中（方案 A 冻结红线）。
func NewHandler(controlAddr, mutationToken string) (http.Handler, error) {
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

	// mutation 代理：独立 Director（查 mutationRoutes 表 + 服务端 token 注入）。
	mutProxy := httputil.NewSingleHostReverseProxy(target)
	mutOrigDirector := mutProxy.Director
	mutProxy.Director = func(req *http.Request) {
		upstream := mutationRoutes[req.URL.Path]
		mutOrigDirector(req)
		req.URL.Path = upstream
		req.Host = target.Host
		// 方案 A 核心：Authorization 在【服务端】注入。浏览器请求必须不含
		// token——Explorer 不读取请求中的任何凭据，注入值只来自 -token-file。
		req.Header.Set("Authorization", "Bearer "+mutationToken)
	}
	mutProxy.ErrorHandler = proxy.ErrorHandler

	mux := http.NewServeMux()
	for exposed := range readOnlyRoutes {
		mux.Handle(exposed, methodGate(proxy))
	}
	for exposed := range mutationRoutes {
		mux.Handle(exposed, mutationGate(mutProxy, mutationToken))
	}
	// 白名单之外的 /api/* → 本地 404，上游零接触。
	// 注意：这些路径【未注册】精确模式，因此任何方法（含 GET）都是 404；
	// 405 + Allow: GET/POST 只出现在已注册路径收到错误方法时。
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeLocalError(w, http.StatusNotFound, "endpoint 不在 Explorer 白名单内")
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

// mutationGate 在 Explorer 边缘收紧 mutation 通道（PHASE MINING-LIFECYCLE-1，
// 方案 A 冻结）。逐层守卫，任一层不过即本地拒绝、上游零接触：
//
//  1. fail-closed：token 未配置（只读模式）→ 503，绝不转发；
//  2. 方法：仅 POST → 405 + Allow: POST；
//  3. Origin：请求携带 Origin 头时必须是本源（http(s)://<Host>）→ 否则 403。
//     浏览器对 POST 恒发 Origin（含同源），跨站表单/no-cors fetch 因此被拦；
//     无 Origin 头 = 非浏览器客户端（curl/脚本），放行（运维兼容）；
//  4. Content-Type：必须 application/json（含 charset 后缀）→ 否则 403。
//     简单表单（x-www-form-urlencoded / text/plain）因此无法触达 mutation。
//
// 通过后由 mutProxy.Director 注入 Authorization 并转发。
func mutationGate(next http.Handler, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			writeLocalError(w, http.StatusServiceUnavailable, "mutation 未配置凭据（只读模式，fail-closed）")
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			writeLocalError(w, http.StatusMethodNotAllowed, "mutation 接口仅允许 POST 方法")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			if origin != "http://"+r.Host && origin != "https://"+r.Host {
				writeLocalError(w, http.StatusForbidden, "origin 不被允许")
				return
			}
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			writeLocalError(w, http.StatusForbidden, "mutation 请求必须使用 application/json")
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
