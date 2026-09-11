package control

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// DefaultAddr 控制接口默认监听地址（仅本机回环）。
const DefaultAddr = "127.0.0.1:6689"

// Client 控制接口客户端。
type Client struct {
	base string
	hc   *http.Client
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

// Status 查询节点状态。
func (c *Client) Status() (*StatusInfo, error) {
	var out StatusInfo
	if err := c.get("/status", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
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
