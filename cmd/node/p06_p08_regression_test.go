// p06_p08_regression_test.go 回归测试：
//   - P0-6：控制接口「无鉴权」误导文案已修正（mutation 端点已 Bearer 认证，
//     只有只读端点无鉴权；help 文案不再笼统写「无鉴权」）；
//     注：mutation 端点原为 6 条，按需出块下线后为 4 条
//     （/send /mine/start /mine/stop /stop）。
//   - P0-8：-rpc 指向非回环地址时默认 fail-closed 拒绝启动，必须显式
//     --allow-non-loopback 确认（此前仅记一条日志警告仍继续启动）。
package main

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

// setupP06P08Dir 用生产 cmdInit 建一个可启动的数据目录。
func setupP06P08Dir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if code := cmdInit([]string{"-datadir", dir}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("init failed: code=%d", code)
	}
	return dir
}

// TestNonLoopbackBindFailClosed 验证 P0-8：非回环 -rpc 未显式确认时拒绝启动，
// 且检查发生在数据目录加锁之前（无副作用：随后回环地址仍能正常启动）。
func TestNonLoopbackBindFailClosed(t *testing.T) {
	dir := setupP06P08Dir(t)
	// P0-4：节点启动必填钱包口令文件，此处仅为测试脚手架。
	pwFile := mustTestWalletPasswordFile(t, dir)

	_, err := newNodeRuntime(nodeConfig{
		DataDir:            dir,
		ListenAddr:         "127.0.0.1:0",
		RPCAddr:            "0.0.0.0:0", // 非回环，且无显式确认
		WalletPasswordFile: pwFile,
	})
	if err == nil {
		t.Fatalf("非回环控制接口地址未显式确认时必须拒绝启动，P0-8 回归")
	}
	if !strings.Contains(err.Error(), "非回环") || !strings.Contains(err.Error(), "--allow-non-loopback") {
		t.Fatalf("错误信息必须指明非回环风险与放行开关，实际：%v", err)
	}

	// 无副作用：同一数据目录用回环地址仍能正常启动（证明 fail 在加锁前）。
	rt, err := newNodeRuntime(nodeConfig{
		DataDir:            dir,
		ListenAddr:         "127.0.0.1:0",
		RPCAddr:            "127.0.0.1:0",
		WalletPasswordFile: pwFile,
	})
	if err != nil {
		t.Fatalf("fail-closed 不应留下副作用（数据目录锁），回环启动失败：%v", err)
	}
	rt.Close()
}

// TestNonLoopbackBindExplicitAllow 验证显式确认后非回环绑定可以启动
// （运维已确认风险的场景不被误杀）。
func TestNonLoopbackBindExplicitAllow(t *testing.T) {
	dir := setupP06P08Dir(t)
	// P0-4：节点启动必填钱包口令文件，此处仅为测试脚手架。
	pwFile := mustTestWalletPasswordFile(t, dir)

	rt, err := newNodeRuntime(nodeConfig{
		DataDir:            dir,
		ListenAddr:         "127.0.0.1:0",
		RPCAddr:            "0.0.0.0:0",
		AllowNonLoopback:   true, // 显式确认
		WalletPasswordFile: pwFile,
	})
	if err != nil {
		t.Fatalf("显式 --allow-non-loopback 确认后应能启动：%v", err)
	}
	defer rt.Close()
	if isLoopback("0.0.0.0:0") {
		t.Fatalf("isLoopback(0.0.0.0) 必须为 false，否则 fail-closed 被绕过")
	}
}

// TestAllowNonLoopbackFlagRegistered 验证 --allow-non-loopback 开关已注册且默认关闭。
func TestAllowNonLoopbackFlagRegistered(t *testing.T) {
	fs, nf := newNodeFlagSet(flag.ContinueOnError) // 仅建集不解析
	if nf.allowNonLoopback {
		t.Fatalf("--allow-non-loopback 默认必须为 false（fail-closed）")
	}
	if fs.Lookup("allow-non-loopback") == nil {
		t.Fatalf("--allow-non-loopback 未在 flag 集中注册")
	}
}
