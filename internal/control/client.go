package control

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// DefaultAddr 控制接口默认监听地址（仅本机回环）。
const DefaultAddr = "127.0.0.1:6689"

// Client 控制接口客户端。
type Client struct {
	base  string
	hc    *http.Client
	token string
}

// NewClient 创建客户端；addr 为空时使用 DefaultAddr。
func NewClient(addr string) *Client {
	if addr == "" {
		addr = DefaultAddr
	}
	return &Client{
		base: "http://" + addr,
		hc:   &http.Client{Timeout: 15 * time.Second},
	}
}

// SetToken 设置 mutation 请求（POST /send /mine /stop）携带的 Bearer Token；
// 空串表示不携带。token 仅保存在内存中，绝不写入日志或错误信息。
// 非 goroutine-safe：调用方应在并发使用前完成设置。
func (c *Client) SetToken(tok string) { c.token = tok }

// Status 查询节点状态。
func (c *Client) Status() (*StatusInfo, error) {
	var out StatusInfo
	if err := c.get("/status", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Logs 查询节点最近的运行日志（tail 为最大行数，<=0 时使用服务端默认值）。
//
// 日志来源在节点侧属于可选能力：未接入时返回空切片，而不是错误。
func (c *Client) Logs(tail int) ([]LogEntry, error) {
	var out []LogEntry
	var q map[string]string
	if tail > 0 {
		q = map[string]string{"tail": strconv.Itoa(tail)}
	}
	if err := c.get("/logs", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Balance 查询地址余额。
func (c *Client) Balance(address string) (*BalanceInfo, error) {
	var out BalanceInfo
	if err := c.get("/balance", map[string]string{"address": address}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UTXOs 列出某地址的未花费输出。
func (c *Client) UTXOs(address string) ([]UTXOInfo, error) {
	var out []UTXOInfo
	if err := c.get("/utxos", map[string]string{"address": address}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Send 提交一笔转账。
func (c *Client) Send(req SendRequest) (*SendResponse, error) {
	var out SendResponse
	if err := c.post("/send", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BlockHex 按高度取区块编码（十六进制）。
func (c *Client) BlockHex(height int) (string, error) {
	var out struct {
		Height  int    `json:"height"`
		Encoded string `json:"encoded"`
	}
	if err := c.get("/block", map[string]string{"height": fmt.Sprint(height)}, &out); err != nil {
		return "", err
	}
	return out.Encoded, nil
}

// Mine 按需立即挖出 count 个区块（测试网用）。
func (c *Client) Mine(count int) (*MineResponse, error) {
	var out MineResponse
	if err := c.post("/mine", MineRequest{Count: count}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Stop 请求节点优雅停止（PHASE PRODUCT-DEV-1B）。
//
// 注意：节点可能在写出响应之前就关闭了连接，这属于正常现象而非失败，
// 调用方应以「节点是否真的退出」作为最终判据（见 cmdStop 的轮询确认）。
func (c *Client) Stop() (*StopResponse, error) {
	var out StopResponse
	if err := c.post("/stop", struct{}{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) get(path string, query map[string]string, out any) error {
	u := c.base + path
	if len(query) > 0 {
		q := url.Values{}
		for k, v := range query {
			q.Set(k, v)
		}
		u += "?" + q.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) post(path string, body any, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	// mutation 端点自 PHASE CONTROL-AUTH-1 起要求 Bearer Token；已设置则自动携带。
	// 仅 POST（mutation）携带；GET（read 端点）保持原行为。
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("无法连接节点控制接口（%s）: %w", c.base, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		var e errorResponse
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return fmt.Errorf("节点返回错误（HTTP %d）: %s", resp.StatusCode, e.Error)
		}
		return fmt.Errorf("节点返回错误（HTTP %d）: %s", resp.StatusCode, string(data))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	return nil
}
