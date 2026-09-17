// cmd/explorer: P2PChain Explorer V1 独立进程。
//
// 架构（PHASE EXPLORER-UI-READINESS-AUDIT 冻结）：
//
//	浏览器 → Explorer(默认 127.0.0.1:9091) → control(默认 http://127.0.0.1:17881) → Node
//
// 仅本机监听、无鉴权 —— LAN 暴露/限流/鉴权为 DEFERRED，需独立安全授权。
// Explorer 暴露【显式只读 API 面】：仅 GET /api/status、/api/blocks、/api/block
// 三个数据端点被代理；/mine /send /stop /console 与 /balance /utxos /logs 等
// 一律由 Explorer 本地拒绝，上游零接触（F-1 修复，OPTION A + P1）。
// Explorer 不包含任何挖矿/转帐/停机控制；Node 控制面本身（127.0.0.1:17881）
// 仍无鉴权，其加固为独立项 F-2（OPEN），不属于本进程的职责范围。
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"p2pchain/internal/control"
	"p2pchain/internal/explorer"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:9091", "Explorer HTTP 监听地址（仅本机）")
	rpc := flag.String("rpc", "http://127.0.0.1:17881", "本机 control 接口地址")
	// PHASE MINING-LIFECYCLE-1（方案 A 冻结）：mutation 凭据仅由服务端经 0600
	// 文件持有并注入上游；浏览器零接触。缺省=只读模式（mutation 端点本地
	// fail-closed 拒绝）；提供了 -token-file 但读取/校验失败 ⇒ 启动失败。
	tokenFile := flag.String("token-file", "",
		"mutation Bearer Token 文件（0600；缺省=只读模式，/api/mine/* 一律 503）")
	flag.Parse()

	mutationToken := ""
	if *tokenFile != "" {
		tok, err := control.LoadTokenFile(*tokenFile)
		if err != nil {
			log.Fatalf("[explorer] mutation token 不可用（fail-closed）: %v", err)
		}
		mutationToken = tok
	}

	handler, err := explorer.NewHandler(*rpc, mutationToken)
	if err != nil {
		log.Fatalf("[explorer] 无效的 control 地址 %q: %v", *rpc, err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", handler)

	log.Printf("[explorer] P2PChain Explorer V1 启动: http://%s  (control=%s, mutation=%s)",
		*listen, *rpc, map[bool]string{true: "enabled(-token-file)", false: "read-only"}[mutationToken != ""])
	if err := http.ListenAndServe(*listen, mux); err != nil {
		log.Fatalf("[explorer] 监听失败 %s: %v", *listen, fmt.Sprintf("%v", err))
	}
}
