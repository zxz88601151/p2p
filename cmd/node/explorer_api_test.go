package main

// PHASE EXPLORER-API-IMPLEMENTATION-1：Explorer 只读 API 的真实链全栈验证。
//
// 复用 fullstack_test.go 的真实节点夹具（无 mock：真实 PoW、真实持久化、
// 真实 HTTP 控制接口），验证 /blocks 与 /block?hash= 的**链上语义**：
//   - canonical 升序排序、at_tip 边界、count 分页截断；
//   - by-hash 命中 canonical 块（字段与链一致、height 可解析）；
//   - 合法格式但链上不存在的哈希 → 404；
//   - /status、/block?height=、/mine 既有契约无回归。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// getJSONFull 对测试节点控制接口发起 GET 并解码 JSON。
func getJSONFull(t *testing.T, base, path string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(base + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("%s 响应不是合法 JSON: %v", path, err)
	}
	return resp.StatusCode, body
}

// TestExplorerBlocksAPIRealChain /blocks：真实链上的排序、分页与 at_tip 语义。
func TestExplorerBlocksAPIRealChain(t *testing.T) {
	rt, _, _ := startTestRuntime(t, false)
	base := "http://" + rt.ctl.Addr()
	mineBlocks(t, rt, 5)

	// 正常分页：from=2&count=3 → 高度 2,3,4 升序，at_tip=false
	code, body := getJSONFull(t, base, "/blocks?from=2&count=3")
	if code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", code)
	}
	if got := int(body["height"].(float64)); got != 5 {
		t.Fatalf("链尾高度 = %d, want 5", got)
	}
	blocks := body["blocks"].([]any)
	if len(blocks) != 3 {
		t.Fatalf("returned = %d, want 3", len(blocks))
	}
	wantHeights := []float64{2, 3, 4}
	for i, b := range blocks {
		bm := b.(map[string]any)
		if got := bm["height"].(float64); got != wantHeights[i] {
			t.Fatalf("blocks[%d].height = %v, want %v（canonical 必须升序）", i, got, wantHeights[i])
		}
		if bm["canonical"] != true || bm["persisted"] != true {
			t.Fatalf("blocks[%d] canonical/persisted 标签错误: %v", i, bm)
		}
		if bm["consensus_era"] != "pre-hardfork" {
			t.Fatalf("blocks[%d] era = %v, want pre-hardfork（h<2000）", i, bm["consensus_era"])
		}
		if len(bm["transactions"].([]any)) < 1 {
			t.Fatalf("blocks[%d] 缺少 coinbase 交易", i)
		}
	}
	if body["at_tip"].(bool) {
		t.Fatal("from=2&count=3 不应到达链尾（高度 5）")
	}

	// at_tip=true：from=3&count=100 → 高度 3,4,5，at_tip=true
	code, body = getJSONFull(t, base, "/blocks?from=3&count=100")
	if code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", code)
	}
	if !body["at_tip"].(bool) {
		t.Fatal("from=3&count=100 应到达链尾")
	}
	if len(body["blocks"].([]any)) != 3 {
		t.Fatalf("count 截断错误: returned=%d, want 3", len(body["blocks"].([]any)))
	}

	// count=100 合法
	code, _ = getJSONFull(t, base, "/blocks?from=0&count=100")
	if code != http.StatusOK {
		t.Fatalf("count=100 状态码 = %d, want 200", code)
	}

	// count=101 → 400
	code, _ = getJSONFull(t, base, "/blocks?from=0&count=101")
	if code != http.StatusBadRequest {
		t.Fatalf("count=101 状态码 = %d, want 400", code)
	}

	// from 越界 = 合法空区间
	code, body = getJSONFull(t, base, "/blocks?from=6&count=10")
	if code != http.StatusOK || len(body["blocks"].([]any)) != 0 {
		t.Fatalf("越界 from 应 200+空列表: code=%d body=%v", code, body)
	}
}

// TestExplorerBlockByHashRealChain /block?hash=：真实链命中与错误映射。
func TestExplorerBlockByHashRealChain(t *testing.T) {
	rt, _, _ := startTestRuntime(t, false)
	base := "http://" + rt.ctl.Addr()
	mineBlocks(t, rt, 3)

	tip, err := rt.chain.BlockByHeight(2)
	if err != nil {
		t.Fatal(err)
	}
	tipHash := tip.Header.HashHex()

	// canonical 命中：字段与链一致、height 可解析
	code, body := getJSONFull(t, base, "/block?hash="+tipHash)
	if code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", code)
	}
	if body["hash"].(string) != tipHash {
		t.Fatalf("hash 不符: %v", body["hash"])
	}
	if got := int(body["height"].(float64)); got != 2 {
		t.Fatalf("canonical 块 height = %v, want 2", body["height"])
	}
	if body["canonical"] != true {
		t.Fatal("canonical 块 canonical 标签应为 true")
	}
	if body["previous_hash"].(string) == "" || body["merkle_root"].(string) == "" {
		t.Fatal("previous_hash/merkle_root 缺失")
	}

	// 合法格式但链上不存在 → 404
	unknown := fmt.Sprintf("%064x", 0xdeadbeef)
	code, _ = getJSONFull(t, base, "/block?hash="+unknown)
	if code != http.StatusNotFound {
		t.Fatalf("未知哈希状态码 = %d, want 404", code)
	}

	// 非法 hex → 400
	code, _ = getJSONFull(t, base, "/block?hash="+fmt.Sprintf("%064s", "zz"))
	if code != http.StatusBadRequest {
		t.Fatalf("非法 hex 状态码 = %d, want 400", code)
	}

	// 错误长度 → 400
	code, _ = getJSONFull(t, base, "/block?hash=abcd")
	if code != http.StatusBadRequest {
		t.Fatalf("错误长度状态码 = %d, want 400", code)
	}
}

// TestExplorerRegressionRealChain 回归：/status、/block?height= 既有契约不变；
// 按需出块（POST /mine）已下线，端点必须返回 404。
func TestExplorerRegressionRealChain(t *testing.T) {
	rt, _, _ := startTestRuntime(t, false)
	base := "http://" + rt.ctl.Addr()

	// /status
	code, body := getJSONFull(t, base, "/status")
	if code != http.StatusOK || int(body["height"].(float64)) != 0 {
		t.Fatalf("/status 回归: code=%d height=%v", code, body["height"])
	}

	mineBlocks(t, rt, 2)

	// /block?height=1 → 既有 {height, encoded} 契约
	resp, err := http.Get(base + "/block?height=1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var blk struct {
		Height  int    `json:"height"`
		Encoded string `json:"encoded"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&blk); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || blk.Height != 1 || len(blk.Encoded) == 0 {
		t.Fatalf("/block?height= 回归: code=%d body=%+v", resp.StatusCode, blk)
	}

	// 按需出块已整体下线：POST /mine 必须不再存在（404）。
	// 原用例断言「/mine 既有契约不变」；端点下线后该断言改为「端点已移除」，
	// 使「误把路由加回来」这类回归立刻暴露。
	mresp := postAuthed(t, base+"/mine", `{"count":1}`)
	defer mresp.Body.Close()
	if mresp.StatusCode != http.StatusNotFound {
		t.Fatalf("POST /mine 状态码 = %d, want 404（按需出块已下线）", mresp.StatusCode)
	}
}

func string2Reader(s string) *stringReader { return &stringReader{s: s} }

type stringReader struct{ s string }

func (r *stringReader) Read(p []byte) (int, error) {
	if len(r.s) == 0 {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.s)
	r.s = r.s[n:]
	return n, nil
}
