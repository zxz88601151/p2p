package control_test

// PHASE EXPLORER-API-IMPLEMENTATION-1：Explorer 只读 API 的 HTTP 契约测试。
//
// 覆盖范围（授权 §10 冻结清单）：
//   /blocks    — 正常请求、count 边界（1/100/101/0/负）、from 非法/负/越界、
//                空区间（200 + 空列表而非 500）、at_tip、JSON 形状、参数透传。
//   /block?hash= — 合法哈希、非法 hex、错误长度、未找到（404）、
//                与 height= 互斥（400）、JSON 形状。
//   回归       — 既有 /block?height= 与 /mine 契约不回归（既有用例另在
//                server_test.go 全量执行）。
//
// 本文件只验证**协议层**；真实链上语义（canonical 排序、atTip、哈希命中）
// 由 cmd/node/explorer_api_test.go 用真实节点全栈验证。

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"p2pchain/internal/control"
)

func getJSON(t *testing.T, url string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	return resp.StatusCode, body
}

// TestBlocksEndpointContract /blocks 正常请求：参数透传 + JSON 契约形状。
func TestBlocksEndpointContract(t *testing.T) {
	h := 4
	node := &fakeNode{blocks: control.BlocksPageResult{
		From: 2, Requested: 3, Returned: 2, AtTip: false, Height: 4,
		Blocks: []control.BlockJSON{
			{Height: &h, Hash: "aa", Canonical: true, Persisted: true, ConsensusEra: "pre-hardfork", Transactions: []control.TxJSON{}},
			{Hash: "bb", Canonical: true, Persisted: true, ConsensusEra: "pre-hardfork", Transactions: []control.TxJSON{}},
		},
	}}
	_, srv := newTestPair(t, node)

	code, body := getJSON(t, srv.URL+"/blocks?from=2&count=3")
	if code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", code)
	}
	if node.lastFrom != 2 || node.lastCount != 3 {
		t.Fatalf("参数未透传: from=%d count=%d", node.lastFrom, node.lastCount)
	}
	if body["from"].(float64) != 2 || body["requested"].(float64) != 3 ||
		body["returned"].(float64) != 2 || body["at_tip"].(bool) ||
		body["height"].(float64) != 4 {
		t.Fatalf("顶层字段不符: %v", body)
	}
	blocks, ok := body["blocks"].([]any)
	if !ok || len(blocks) != 2 {
		t.Fatalf("blocks 形状不符: %v", body["blocks"])
	}
	first := blocks[0].(map[string]any)
	if first["hash"] != "aa" || first["canonical"] != true || first["persisted"] != true {
		t.Fatalf("块字段不符: %v", first)
	}
	if _, has := first["height"]; !has {
		t.Fatal("已知高度的块必须携带 height")
	}
	if _, has := first["consensus_era"]; !has {
		t.Fatal("缺少 consensus_era")
	}
}

// TestBlocksCountBoundaries count 边界：1/100 合法，101/0/负 → 400。
func TestBlocksCountBoundaries(t *testing.T) {
	_, srv := newTestPair(t, &fakeNode{})

	for _, ok := range []string{"1", "100"} {
		code, _ := getJSON(t, srv.URL+"/blocks?from=0&count="+ok)
		if code != http.StatusOK {
			t.Fatalf("count=%s 状态码 = %d, want 200", ok, code)
		}
	}
	for _, bad := range []string{"101", "0", "-5", "1000000"} {
		code, body := getJSON(t, srv.URL+"/blocks?from=0&count="+bad)
		if code != http.StatusBadRequest {
			t.Fatalf("count=%s 状态码 = %d, want 400", bad, code)
		}
		if _, has := body["error"]; !has {
			t.Fatalf("count=%s 400 响应缺少 error 字段: %v", bad, body)
		}
	}
}

// TestBlocksInvalidParams from 非法/缺失、count 缺失 → 400（参数均为必填）。
func TestBlocksInvalidParams(t *testing.T) {
	_, srv := newTestPair(t, &fakeNode{})

	for _, url := range []string{
		"/blocks?from=abc&count=5", // from 非整数
		"/blocks?from=-1&count=5",  // from 为负
		"/blocks?count=5",          // from 缺失
		"/blocks?from=0",           // count 缺失
		"/blocks?from=&count=5",    // from 空串
		"/blocks",                  // 全缺
	} {
		code, _ := getJSON(t, srv.URL+url)
		if code != http.StatusBadRequest {
			t.Fatalf("%s 状态码 = %d, want 400", url, code)
		}
	}
}

// TestBlocksBeyondTip from 超过链尾 = 合法空区间 → 200 + 空列表（不是 500）。
func TestBlocksBeyondTip(t *testing.T) {
	node := &fakeNode{blocks: control.BlocksPageResult{
		From: 9999, Requested: 10, Returned: 0, AtTip: true, Height: 5,
		Blocks: []control.BlockJSON{},
	}}
	_, srv := newTestPair(t, node)

	code, body := getJSON(t, srv.URL+"/blocks?from=9999&count=10")
	if code != http.StatusOK {
		t.Fatalf("空区间状态码 = %d, want 200", code)
	}
	if got := body["returned"].(float64); got != 0 {
		t.Fatalf("空区间 returned = %v, want 0", body["returned"])
	}
	if !body["at_tip"].(bool) {
		t.Fatal("空区间 at_tip 应为 true")
	}
	blocks := body["blocks"].([]any)
	if len(blocks) != 0 {
		t.Fatalf("空区间 blocks 应为空列表: %v", blocks)
	}
}

// TestBlocksMethodNotAllowed POST /blocks → 405。
func TestBlocksMethodNotAllowed(t *testing.T) {
	_, srv := newTestPair(t, &fakeNode{})
	resp, err := http.Post(srv.URL+"/blocks", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("状态码 = %d, want 405", resp.StatusCode)
	}
}

// TestBlockByHashContract /block?hash= 合法路径：透传 + JSON 形状。
func TestBlockByHashContract(t *testing.T) {
	hashHex := strings.Repeat("ab", 32)
	node := &fakeNode{blockJSON: control.BlockJSON{
		Hash: hashHex, PreviousHash: strings.Repeat("00", 32),
		Timestamp: 1758067100, Version: 1, Bits: 16, Difficulty: 1,
		Nonce: 42, MerkleRoot: strings.Repeat("cd", 32),
		Size: 200, TxCount: 1, Canonical: true, Persisted: true,
		ConsensusEra: "pre-hardfork",
		Transactions: []control.TxJSON{{TxID: "ee", Coinbase: true, InputCount: 1, OutputCount: 1, TotalOut: 5000000000}},
	}}
	_, srv := newTestPair(t, node)

	code, body := getJSON(t, srv.URL+"/block?hash="+hashHex)
	if code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", code)
	}
	if body["hash"].(string) != hashHex || body["nonce"].(float64) != 42 ||
		body["tx_count"].(float64) != 1 || body["canonical"] != true {
		t.Fatalf("块 JSON 形状不符: %v", body)
	}
	txs := body["transactions"].([]any)
	if len(txs) != 1 {
		t.Fatalf("transactions 形状不符: %v", txs)
	}
	// hash 参数应以 32 字节解码后透传
	var want [32]byte
	for i := range want {
		want[i] = 0xab
	}
	if node.lastHash != want {
		t.Fatal("hash 未按 32 字节解码透传")
	}
	// detached（height 未知）→ height 字段应被省略
	if _, has := body["height"]; has {
		t.Fatal("height 未知的响应不应携带 height 字段")
	}
}

// TestBlockByHashErrors /block?hash= 错误映射：非法 hex/错长度 → 400；未找到 → 404。
func TestBlockByHashErrors(t *testing.T) {
	node := &fakeNode{blockJSONErr: control.ErrBlockNotFound}
	_, srv := newTestPair(t, node)

	// 非法 hex
	code, _ := getJSON(t, srv.URL+"/block?hash="+strings.Repeat("zz", 32))
	if code != http.StatusBadRequest {
		t.Fatalf("非法 hex 状态码 = %d, want 400", code)
	}
	// 错误长度（合法 hex 但 63 字符）
	code, _ = getJSON(t, srv.URL+"/block?hash="+strings.Repeat("ab", 31)+"a")
	if code != http.StatusBadRequest {
		t.Fatalf("错误长度状态码 = %d, want 400", code)
	}
	// 合法格式但不存在 → 404
	code, body := getJSON(t, srv.URL+"/block?hash="+strings.Repeat("ab", 32))
	if code != http.StatusNotFound {
		t.Fatalf("未找到状态码 = %d, want 404", code)
	}
	if _, has := body["error"]; !has {
		t.Fatalf("404 响应缺少 error 字段: %v", body)
	}
}

// TestBlockHashHeightExclusive hash 与 height 同时提供 → 400（互斥）。
func TestBlockHashHeightExclusive(t *testing.T) {
	_, srv := newTestPair(t, &fakeNode{})
	code, _ := getJSON(t, srv.URL+"/block?height=3&hash="+strings.Repeat("ab", 32))
	if code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, want 400", code)
	}
}

// TestBlockLegacyHeightContractUnchanged 回归：既有 /block?height= 契约逐字节不变。
func TestBlockLegacyHeightContractUnchanged(t *testing.T) {
	node := &fakeNode{blockHex: "0011"}
	_, srv := newTestPair(t, node)

	resp, err := http.Get(srv.URL + "/block?height=3")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Height  int    `json:"height"`
		Encoded string `json:"encoded"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || body.Height != 3 || body.Encoded != "0011" {
		t.Fatalf("既有 height 契约回归: code=%d body=%+v", resp.StatusCode, body)
	}
	// 缺失 hash 时 height=abc 仍为 400
	code, _ := getJSON(t, srv.URL+"/block?height=abc")
	if code != http.StatusBadRequest {
		t.Fatalf("height=abc 状态码 = %d, want 400", code)
	}
}

// TestMineContractUnchanged 回归：/mine 契约不变（详见 server_test.go TestMineEndpoint）。
func TestMineContractUnchanged(t *testing.T) {
	node := &fakeNode{}
	_, srv := newTestPair(t, node)
	// /mine 为 mutation 端点，PHASE CONTROL-AUTH-1 起需携带 token（postAuth 已带）。
	resp := postAuth(t, srv.URL+"/mine", `{"count":2}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || node.lastMineN != 2 {
		t.Fatalf("/mine 契约回归: code=%d lastMineN=%d", resp.StatusCode, node.lastMineN)
	}
}

// TestBlocksNodeError500 Node 实现侧内部错误 → 500（而非 400）。
func TestBlocksNodeError500(t *testing.T) {
	node := &fakeNode{blocksErr: errors.New("fake internal")}
	_, srv := newTestPair(t, node)
	code, _ := getJSON(t, srv.URL+"/blocks?from=0&count=1")
	if code != http.StatusInternalServerError {
		t.Fatalf("内部错误状态码 = %d, want 500", code)
	}
}
