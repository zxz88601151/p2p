package control_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"p2pchain/internal/control"
)

// fakeNode 是 control.Node 的测试替身：仅验证协议层行为，
// 不代表真实共识逻辑（真实实现的端到端测试在 cmd/node）。
type fakeNode struct {
	status    control.StatusInfo
	balance   control.BalanceInfo
	utxos     []control.UTXOInfo
	sendResp  control.SendResponse
	blockHex  string
	mineResp  control.MineResponse
	blocks    control.BlocksPageResult
	blockJSON control.BlockJSON

	mineStartResp control.MineStartResponse
	mineStopResp  control.MineStopResponse

	balanceErr   error
	sendErr      error
	blockErr     error
	mineErr      error
	blocksErr    error
	blockJSONErr error
	mineStartErr error
	mineStopErr  error

	lastAddress string
	lastTo      string
	lastAmount  uint64
	lastFee     uint64
	lastMineN   int
	lastFrom    int
	lastCount   int
	lastHash    [32]byte
}

func (f *fakeNode) Status() (control.StatusInfo, error) { return f.status, nil }

func (f *fakeNode) Balance(address string) (control.BalanceInfo, error) {
	f.lastAddress = address
	if f.balanceErr != nil {
		return control.BalanceInfo{}, f.balanceErr
	}
	b := f.balance
	b.Address = address
	return b, nil
}

func (f *fakeNode) UTXOs(address string) ([]control.UTXOInfo, error) {
	f.lastAddress = address
	return f.utxos, nil
}

func (f *fakeNode) Send(to string, amount, fee uint64) (control.SendResponse, error) {
	f.lastTo, f.lastAmount, f.lastFee = to, amount, fee
	if f.sendErr != nil {
		return control.SendResponse{}, f.sendErr
	}
	return f.sendResp, nil
}

func (f *fakeNode) BlockHex(height int) (string, error) {
	if f.blockErr != nil {
		return "", f.blockErr
	}
	return f.blockHex, nil
}

func (f *fakeNode) Mine(count int) (control.MineResponse, error) {
	f.lastMineN = count
	if f.mineErr != nil {
		return control.MineResponse{}, f.mineErr
	}
	r := f.mineResp
	if r.Mined == 0 {
		r.Mined = count
	}
	return r, nil
}

// StartMining / StopMining（PHASE MINING-LIFECYCLE-1）：测试替身仅协议层——
// 返回预设响应/错误，供 handler 契约测试（200/409/500 映射）使用。
func (f *fakeNode) StartMining() (control.MineStartResponse, error) {
	if f.mineStartErr != nil {
		return control.MineStartResponse{}, f.mineStartErr
	}
	return f.mineStartResp, nil
}

func (f *fakeNode) StopMining() (control.MineStopResponse, error) {
	if f.mineStopErr != nil {
		return control.MineStopResponse{}, f.mineStopErr
	}
	return f.mineStopResp, nil
}

func (f *fakeNode) BlocksPage(from, count int) (control.BlocksPageResult, error) {
	f.lastFrom, f.lastCount = from, count
	if f.blocksErr != nil {
		return control.BlocksPageResult{}, f.blocksErr
	}
	return f.blocks, nil
}

func (f *fakeNode) BlockJSONByHash(hash [32]byte) (control.BlockJSON, error) {
	f.lastHash = hash
	if f.blockJSONErr != nil {
		return control.BlockJSON{}, f.blockJSONErr
	}
	return f.blockJSON, nil
}

// testToken 供 newTestPair 注入的 mutation 测试 token（PHASE CONTROL-AUTH-1）。
// 仅测试用临时凭据，严禁在测试中出现真实生产 token。
const testToken = "test-token-0123456789abcdef"

func newTestPair(t *testing.T, node control.Node) (*control.Client, *httptest.Server) {
	t.Helper()
	s := control.NewServer(node)
	s.SetAuthToken(testToken)
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	c := control.NewClient(strings.TrimPrefix(srv.URL, "http://"))
	c.SetToken(testToken)
	return c, srv
}

// postAuth 以携带测试 token 的 POST 请求访问 srv 的 mutation 端点。
func postAuth(t *testing.T, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// TestServerClientRoundTrip 状态/余额/UTXO 的完整 HTTP 往返。
func TestServerClientRoundTrip(t *testing.T) {
	node := &fakeNode{
		status:  control.StatusInfo{Height: 7, TipHash: "abc", Peers: []string{"1.2.3.4:1"}, MempoolSize: 2, Mining: true, Address: "ADDR1"},
		balance: control.BalanceInfo{Spendable: 30, Total: 50, UTXOCount: 2, Height: 7},
		utxos: []control.UTXOInfo{
			{OutPoint: "aa:0", Value: 50, Height: 1, IsCoinbase: true, Mature: false},
			{OutPoint: "bb:1", Value: 30, Height: 5, IsCoinbase: false, Mature: true},
		},
	}
	client, _ := newTestPair(t, node)

	st, err := client.Status()
	if err != nil {
		t.Fatalf("Status 失败: %v", err)
	}
	if st.Height != 7 || st.TipHash != "abc" || st.MempoolSize != 2 || !st.Mining || len(st.Peers) != 1 {
		t.Fatalf("Status 结果不符: %+v", st)
	}

	bal, err := client.Balance("A1")
	if err != nil {
		t.Fatalf("Balance 失败: %v", err)
	}
	if bal.Address != "A1" || bal.Spendable != 30 || bal.Total != 50 || bal.UTXOCount != 2 {
		t.Fatalf("Balance 结果不符: %+v", bal)
	}
	if node.lastAddress != "A1" {
		t.Fatalf("服务端未收到 address 参数: %q", node.lastAddress)
	}

	list, err := client.UTXOs("A1")
	if err != nil {
		t.Fatalf("UTXOs 失败: %v", err)
	}
	if len(list) != 2 || list[1].OutPoint != "bb:1" || !list[1].Mature {
		t.Fatalf("UTXOs 结果不符: %+v", list)
	}
}

// TestSendRoundTrip Send 请求体与响应正确传递。
func TestSendRoundTrip(t *testing.T) {
	node := &fakeNode{sendResp: control.SendResponse{TxID: "deadbeef", Fee: 2, Amount: 100, To: "TOADDR", InputNum: 1}}
	client, _ := newTestPair(t, node)

	resp, err := client.Send(control.SendRequest{To: "TOADDR", Amount: 100, Fee: 2})
	if err != nil {
		t.Fatalf("Send 失败: %v", err)
	}
	if resp.TxID != "deadbeef" || resp.InputNum != 1 {
		t.Fatalf("Send 结果不符: %+v", resp)
	}
	if node.lastTo != "TOADDR" || node.lastAmount != 100 || node.lastFee != 2 {
		t.Fatalf("服务端未收到正确参数: to=%q amount=%d fee=%d", node.lastTo, node.lastAmount, node.lastFee)
	}
}

// TestServerErrorMapping 业务错误以 4xx + error 字段返回，客户端能读出可读信息。
func TestServerErrorMapping(t *testing.T) {
	node := &fakeNode{sendErr: errors.New("余额不足"), balanceErr: errors.New("地址非法")}
	client, _ := newTestPair(t, node)

	if _, err := client.Send(control.SendRequest{To: "X", Amount: 1}); err == nil {
		t.Fatal("预期报错但成功了")
	} else if !strings.Contains(err.Error(), "余额不足") {
		t.Fatalf("错误信息未透传: %v", err)
	}
	if _, err := client.Balance("BAD"); err == nil || !strings.Contains(err.Error(), "地址非法") {
		t.Fatalf("Balance 错误未透传: %v", err)
	}
}

// TestMissingAddressRejected 缺少 address 参数应返回 400。
func TestMissingAddressRejected(t *testing.T) {
	client, srv := newTestPair(t, &fakeNode{})

	resp, err := http.Get(srv.URL + "/balance")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, want 400", resp.StatusCode)
	}

	// 客户端侧同样应报错（而非返回空结果）
	if _, err := client.Balance(""); err == nil {
		t.Fatal("空地址预期报错")
	}
}

// TestMethodNotAllowed 方法不符应返回 405 且带 Allow 头。
func TestMethodNotAllowed(t *testing.T) {
	_, srv := newTestPair(t, &fakeNode{})

	resp, err := http.Post(srv.URL+"/status", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("状态码 = %d, want 405", resp.StatusCode)
	}
	if got := resp.Header.Get("Allow"); got != http.MethodGet {
		t.Fatalf("Allow 头 = %q, want GET", got)
	}
}

// TestBlockEndpoint 区块编码端点：合法高度返回编码，非法高度返回 400。
func TestBlockEndpoint(t *testing.T) {
	client, srv := newTestPair(t, &fakeNode{blockHex: "0011"})

	got, err := client.BlockHex(3)
	if err != nil {
		t.Fatalf("BlockHex 失败: %v", err)
	}
	if got != "0011" {
		t.Fatalf("BlockHex = %q, want 0011", got)
	}

	resp, err := http.Get(srv.URL + "/block?height=abc")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("非法 height 状态码 = %d, want 400", resp.StatusCode)
	}
}

// TestMineEndpoint 按需出块端点：显式 count、非法 count 返回 400、
// 业务冲突（如持续挖矿中）返回 409。
func TestMineEndpoint(t *testing.T) {
	node := &fakeNode{}
	client, srv := newTestPair(t, node)

	resp, err := client.Mine(3)
	if err != nil {
		t.Fatalf("Mine 失败: %v", err)
	}
	if resp.Mined != 3 || node.lastMineN != 3 {
		t.Fatalf("Mine 结果不符: %+v, lastMineN=%d", resp, node.lastMineN)
	}

	for _, bad := range []string{`{"count":0}`, `{"count":-1}`, `{"count":99999}`} {
		r := postAuth(t, srv.URL+"/mine", bad)
		_ = r.Body.Close()
		if r.StatusCode != http.StatusBadRequest {
			t.Fatalf("count=%s 状态码 = %d, want 400", bad, r.StatusCode)
		}
	}

	// 节点报告业务冲突（例如正在持续挖矿）→ 409
	node.mineErr = errors.New("节点正在持续挖矿")
	r := postAuth(t, srv.URL+"/mine", `{"count":1}`)
	_ = r.Body.Close()
	if r.StatusCode != http.StatusConflict {
		t.Fatalf("业务冲突状态码 = %d, want 409", r.StatusCode)
	}
	if _, err := client.Mine(1); err == nil || !strings.Contains(err.Error(), "持续挖矿") {
		t.Fatalf("冲突错误未透传: %v", err)
	}
}

// TestServerStartStop 真实监听随机端口，Start 返回可用地址并能被客户端访问。
func TestServerStartStop(t *testing.T) {
	node := &fakeNode{status: control.StatusInfo{Height: 1}}
	srv := control.NewServer(node)

	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	if addr == "" || srv.Addr() != addr {
		t.Fatalf("监听地址异常: addr=%q srv.Addr()=%q", addr, srv.Addr())
	}

	st, err := control.NewClient(addr).Status()
	if err != nil {
		t.Fatalf("通过真实端口访问失败: %v", err)
	}
	if st.Height != 1 {
		t.Fatalf("Height = %d, want 1", st.Height)
	}

	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop 失败: %v", err)
	}
	// 关闭后再访问应失败
	if _, err := control.NewClient(addr).Status(); err == nil {
		t.Fatal("Stop 之后仍能访问控制接口")
	}
}
