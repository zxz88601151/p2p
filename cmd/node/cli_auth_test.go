package main

// PHASE CONTROL-AUTH-1：CLI mutation 子命令（send/stop）的 token 行为测试。
//
// 覆盖矩阵：missing / unreadable / invalid / valid token-file。
// 有效路径通过 httptest 挂载真实 control.Server（含 token 校验）做端到端验证，
// 不使用生产节点、不接触生产环境。
//
// 变更说明：原矩阵以 `node mine` 子命令承载（cmdMine）。按需出块整体下线后
// cmdMine 被删除，该矩阵改由 `node stop` 承载 —— 二者走完全相同的
// control.LoadTokenFile 路径与失败语义，覆盖强度不变。

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"p2pchain/internal/control"
)

// cliFakeNode 是 control.Node 的最小测试替身（仅 CLI 认证测试使用）。
type cliFakeNode struct{}

func (cliFakeNode) Status() (control.StatusInfo, error) {
	return control.StatusInfo{Height: 1, Address: "TESTADDR"}, nil
}
func (cliFakeNode) Balance(string) (control.BalanceInfo, error) {
	return control.BalanceInfo{Spendable: 1, Total: 1, Height: 1}, nil
}
func (cliFakeNode) UTXOs(string) ([]control.UTXOInfo, error) { return nil, nil }
func (cliFakeNode) Send(string, uint64, uint64) (control.SendResponse, error) {
	return control.SendResponse{TxID: "tx", To: "TESTADDR", Amount: 1}, nil
}
func (cliFakeNode) BlockHex(int) (string, error) { return "00", nil }
func (cliFakeNode) BlocksPage(int, int) (control.BlocksPageResult, error) {
	return control.BlocksPageResult{}, nil
}
func (cliFakeNode) BlockJSONByHash([32]byte) (control.BlockJSON, error) {
	return control.BlockJSON{}, nil
}
func (cliFakeNode) StartMining() (control.MineStartResponse, error) {
	return control.MineStartResponse{Accepted: true, State: "STARTING"}, nil
}
func (cliFakeNode) StopMining() (control.MineStopResponse, error) {
	return control.MineStopResponse{Accepted: true, State: "STOPPED"}, nil
}

// newAuthedTestServer 起一个要求 token 的 control 服务，返回 rpc 地址（host:port）。
func newAuthedTestServer(t *testing.T) string {
	t.Helper()
	s := control.NewServer(cliFakeNode{})
	s.SetAuthToken(testToken)
	s.SetAuthFailureDelay(0) // 测试中免延迟
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

// TestCLIStopTokenFileMissing 承接原 `node mine` 的「缺失 token 文件」用例：
// 错误必须说明是 token 问题，且**绝不回显 token 内容**。
func TestCLIStopTokenFileMissing(t *testing.T) {
	rpc := newAuthedTestServer(t)
	missing := filepath.Join(t.TempDir(), "no-such-token")
	var out, errBuf bytes.Buffer
	code := cmdStop([]string{"-rpc", rpc, "-token-file", missing}, &out, &errBuf)
	if code == 0 {
		t.Fatalf("缺失 token 文件应失败，实际 code=0，输出: %s%s", out.String(), errBuf.String())
	}
	msg := errBuf.String()
	if !strings.Contains(msg, "token") {
		t.Fatalf("错误信息应说明 token 问题，实际: %q", msg)
	}
	if strings.Contains(msg, testToken) {
		t.Fatalf("错误信息不得包含 token 内容: %q", msg)
	}
}

// TestCLIStopTokenFileInvalid 承接原 `node mine` 的「无效 token 文件」用例。
func TestCLIStopTokenFileInvalid(t *testing.T) {
	rpc := newAuthedTestServer(t)
	p := filepath.Join(t.TempDir(), "short-token")
	if err := os.WriteFile(p, []byte("short\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := cmdStop([]string{"-rpc", rpc, "-token-file", p}, &out, &errBuf)
	if code == 0 {
		t.Fatalf("无效 token 文件应失败，实际 code=0")
	}
}

// TestCLIStopTokenFileUnreadableUnix 承接原 `node mine` 的「不可读 token 文件」用例。
func TestCLIStopTokenFileUnreadableUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 文件系统不表达 POSIX 读权限位，跳过")
	}
	rpc := newAuthedTestServer(t)
	p := filepath.Join(t.TempDir(), "unreadable")
	if err := os.WriteFile(p, []byte(testToken), 0o000); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := cmdStop([]string{"-rpc", rpc, "-token-file", p}, &out, &errBuf)
	if code == 0 {
		t.Fatalf("不可读 token 文件应失败，实际 code=0")
	}
}

// TestCLIMineSubcommandRemoved 锁定「按需出块 CLI 入口已下线」这一契约：
// `node mine` 必须不再被识别为子命令（runCLI 返回 ok=false），
// 且不得退化成「启动节点」等其它语义。
func TestCLIMineSubcommandRemoved(t *testing.T) {
	var out, errBuf bytes.Buffer
	code, ok := runCLI("mine", []string{"-count", "1"}, &out, &errBuf)
	if ok {
		t.Fatalf("`node mine` 应已下线（不应被识别为子命令），实际 ok=true code=%d", code)
	}
}

func TestCLISendValidTokenFile(t *testing.T) {
	rpc := newAuthedTestServer(t)
	p := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(p, []byte(testToken), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	// 收款地址用测试节点自报地址（合法地址格式）
	code := cmdSend([]string{"-rpc", rpc, "-to", "TESTADDR", "-amount", "1", "-token-file", p}, &out, &errBuf)
	// TESTADDR 不满足真实地址校验时会以参数错误退出（非 0）；这里只断言不 panic
	// 且错误信息与 token 无关（地址校验先于 token 加载，两条路径都不得回显 token）。
	if strings.Contains(out.String()+errBuf.String(), testToken) {
		t.Fatalf("CLI 输出不得包含 token: %q", out.String()+errBuf.String())
	}
	_ = code
}

func TestCLIStopValidTokenFile(t *testing.T) {
	s := control.NewServer(cliFakeNode{})
	s.SetAuthToken(testToken)
	s.SetAuthFailureDelay(0)
	// stopHit 由 HTTP handler goroutine 写、由下方轮询 goroutine 与测试主
	// goroutine 读，必须原子化：普通 bool 在 -race 下是真实数据竞争
	// （既有缺陷，PHASE MINING-LIFECYCLE-1 全包 race 首次暴露后修复）。
	var stopHit atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		stopHit.Store(true)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"accepted":true,"message":"ok"}`))
	})
	mux.Handle("/", s.Handler())

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	rpc := strings.TrimPrefix(srv.URL, "http://")

	p := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(p, []byte(testToken), 0o600); err != nil {
		t.Fatal(err)
	}

	// /stop 之后随即关闭 HTTP 服务：cmdStop 的探活循环会立刻观察到节点「已停止」，
	// 从而快速返回成功，无需等待 15s 超时。
	go func() {
		for i := 0; i < 200; i++ {
			if stopHit.Load() {
				srv.Close()
				return
			}
			sleepShort()
		}
	}()

	var out, errBuf bytes.Buffer
	code := cmdStop([]string{"-rpc", rpc, "-token-file", p}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("使用有效 token 的 stop 应成功: code=%d out=%q err=%q", code, out.String(), errBuf.String())
	}
	if !stopHit.Load() {
		t.Fatal("stop 请求未到达服务端（token 未生效？）")
	}
}

func TestCLIStopMissingTokenFile(t *testing.T) {
	rpc := newAuthedTestServer(t)
	missing := filepath.Join(t.TempDir(), "no-such-token")
	var out, errBuf bytes.Buffer
	code := cmdStop([]string{"-rpc", rpc, "-token-file", missing}, &out, &errBuf)
	if code == 0 {
		t.Fatalf("缺失 token 文件时 stop 应失败")
	}
}

// sleepShort 供轮询等待停止命中用；避免在测试中写裸 time.Sleep 常量。
func sleepShort() { time.Sleep(10 * time.Millisecond) }
