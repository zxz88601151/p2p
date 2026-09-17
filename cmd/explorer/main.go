// cmd/explorer: P2PChain Explorer V1 独立进程。
//
// 架构（PHASE EXPLORER-UI-READINESS-AUDIT 冻结）：
//
//	浏览器 → Explorer(默认 127.0.0.1:9091) → control(默认 http://127.0.0.1:17881) → Node
//
// 仅本机监听、无鉴权 —— LAN 暴露/限流/鉴权为 DEFERRED，需独立安全授权。
// Explorer = READ；/mine 为唯一被允许的受控调用；/send /stop 永不代理 UI 暴露之外
// 的任何额外路径（代理是通用的 /api/* 透传，UI 前端代码永不构造 /send /stop 请求，
// 见 ui/app.js 安全注释）。
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"p2pchain/internal/explorer"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:9091", "Explorer HTTP 监听地址（仅本机）")
	rpc := flag.String("rpc", "http://127.0.0.1:17881", "本机 control 接口地址")
	flag.Parse()

	handler, err := explorer.NewHandler(*rpc)
	if err != nil {
		log.Fatalf("[explorer] 无效的 control 地址 %q: %v", *rpc, err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", handler)

	log.Printf("[explorer] P2PChain Explorer V1 启动: http://%s  (control=%s)", *listen, *rpc)
	if err := http.ListenAndServe(*listen, mux); err != nil {
		log.Fatalf("[explorer] 监听失败 %s: %v", *listen, fmt.Sprintf("%v", err))
	}
}
