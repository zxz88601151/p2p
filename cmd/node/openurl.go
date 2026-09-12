package main

import (
	"os/exec"
	"runtime"
)

// openBrowser 用系统默认程序打开一个 URL。
//
// 这是 `node ui` 子命令引入的唯一一处外部进程调用，用途单一：把已经由本机
// 控制接口提供好的控制台页面交给系统浏览器（或 WebView）。它不接收用户输入、
// 不参与节点运行路径，且失败不致命——调用方会打印 URL 供手动打开。
//
// 说明：本阶段规格 §20 允许为「UI 宿主」做最小改动；此处仅使用标准库 os/exec，
// 仍然是零第三方依赖。
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
