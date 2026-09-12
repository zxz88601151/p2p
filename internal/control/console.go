package control

import (
	_ "embed"
	"errors"
	"net/http"
)

// errConsoleMethod 控制台页面只接受 GET / HEAD。
var errConsoleMethod = errors.New("控制台页面仅支持 GET")

// consoleHTML 是 Developer Console 的单页前端。
//
// 设计约束（本阶段规格 §19 / §20）：
//   - 零第三方依赖、零构建步骤：页面用 go:embed 直接打进二进制；
//   - 页面**不产生任何数据**：全部指标通过同源 fetch 读取 /status /balance /utxos
//     /block /logs，没有真实来源的字段一律渲染为 Unavailable；
//   - 无 Math.random、无硬编码"生产状态"。
//
//go:embed web/console.html
var consoleHTML []byte

// handleConsole 提供 Developer Console 页面（GET / 或 /console）。
func (s *Server) handleConsole(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/console" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, errConsoleMethod)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(consoleHTML)
}
