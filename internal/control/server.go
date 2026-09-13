// Package control 提供节点的本地控制接口（JSON over HTTP），供命令行工具
// 查询状态、查看余额与提交交易。
//
// 设计取舍：
//   - 只做本机控制，不做远程钱包服务。默认绑定 127.0.0.1，且没有任何鉴权，
//     因此绝不可绑定到公网地址（需要远程访问时应加 TLS + 认证，属本阶段范围外）。
//   - 本包不依赖 blockchain / mempool / utxo——它只声明一个 Node 接口，
//     由 cmd/node 侧的节点服务实现（消费方定义接口）。这样协议与业务状态解耦，
//     本包可以用假实现独立测试。
package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// ErrAddressRequired 请求缺少 address 参数。
var ErrAddressRequired = errors.New("缺少 address 参数")

// ErrStopUnsupported 节点未启用远程停止（未注入停止回调）。
var ErrStopUnsupported = errors.New("该节点未启用远程停止")

// Node 是控制接口所需的节点能力（由节点服务实现）。
type Node interface {
	// Status 返回节点运行状态。
	Status() (StatusInfo, error)
	// Balance 按地址查询余额（spendable 排除未成熟 coinbase）。
	Balance(address string) (BalanceInfo, error)
	// UTXOs 列出某地址当前未花费输出。
	UTXOs(address string) ([]UTXOInfo, error)
	// Send 用节点钱包构造、签名并广播一笔支付交易，返回交易 ID。
	Send(to string, amount, fee uint64) (SendResponse, error)
	// BlockHex 按高度返回区块的规范编码（十六进制）。
	BlockHex(height int) (string, error)
	// Mine 按需立即挖出 count 个区块（测试网/开发用，对标 bitcoind 的 generatetoaddress）。
	// 若节点正在持续挖矿（-mine）则返回错误，避免两个挖矿路径互相干扰。
	Mine(count int) (MineResponse, error)
}

// StatusInfo 节点状态。
type StatusInfo struct {
	Height      int      `json:"height"`
	TipHash     string   `json:"tip_hash"`
	Peers       []string `json:"peers"`
	MempoolSize int      `json:"mempool_size"`
	Mining      bool     `json:"mining"`
	Address     string   `json:"address"` // 节点钱包地址

	// Bits 是「下一个待挖区块」的难度目标（前导零位数语义，见 pow.BitsToTarget）。
	// 这是共识真值，UI 展示难度必须以此为准。
	Bits uint32 `json:"bits"`
	// Difficulty 是相对最低难度（pow.MaxTargetBits）的倍数：2^(Bits-MaxTargetBits)。
	// 纯展示用派生量，不参与任何共识判断；位宽 <= 53 时 float64 可精确表示。
	Difficulty float64 `json:"difficulty"`
}

// BalanceInfo 地址余额。
type BalanceInfo struct {
	Address   string `json:"address"`
	Spendable uint64 `json:"spendable"` // 可花费（已成熟）
	Total     uint64 `json:"total"`     // 含未成熟 coinbase
	UTXOCount int    `json:"utxo_count"`
	Height    int    `json:"height"`
}

// UTXOInfo 单个未花费输出。
type UTXOInfo struct {
	OutPoint   string `json:"outpoint"` // txid:index
	Value      uint64 `json:"value"`
	Height     int    `json:"height"`
	IsCoinbase bool   `json:"is_coinbase"`
	Mature     bool   `json:"mature"`
}

// SendRequest 转账请求。
type SendRequest struct {
	To     string `json:"to"`
	Amount uint64 `json:"amount"`
	Fee    uint64 `json:"fee"`
}

// SendResponse 转账结果。
type SendResponse struct {
	TxID     string `json:"txid"`
	Fee      uint64 `json:"fee"`
	Amount   uint64 `json:"amount"`
	To       string `json:"to"`
	InputNum int    `json:"input_num"`
}

// MineRequest 按需出块请求。
type MineRequest struct {
	Count int `json:"count"`
}

// MineResponse 按需出块结果。
type MineResponse struct {
	Mined  int `json:"mined"`
	Height int `json:"height"`
}

// errorResponse 统一错误体。
type errorResponse struct {
	Error string `json:"error"`
}

// StopResponse 停止请求的结果。
type StopResponse struct {
	Accepted bool   `json:"accepted"`
	Message  string `json:"message"`
}

// Server 控制接口服务端。
type Server struct {
	node Node
	http *http.Server
	ln   net.Listener
	mu   sync.Mutex
	// logs 是可选的日志来源；未注入时 /logs 返回空数组（诚实空态，不伪造日志）。
	logs LogProvider

	// stopHook 由节点注入：收到 POST /stop 时调用，用于请求节点优雅退出。
	// 未注入时 /stop 返回 501（能力未启用）——绝不假装有能力。
	stopHook func()
	stopOnce sync.Once
}

// NewServer 创建服务端。
func NewServer(node Node) *Server { return &Server{node: node} }

// Handler 返回路由（便于测试直接挂到 httptest）。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/balance", s.handleBalance)
	mux.HandleFunc("/utxos", s.handleUTXOs)
	mux.HandleFunc("/send", s.handleSend)
	mux.HandleFunc("/mine", s.handleMine)
	mux.HandleFunc("/block", s.handleBlock)
	mux.HandleFunc("/logs", s.handleLogs)
	mux.HandleFunc("/stop", s.handleStop)
	// 根路径提供本机 Developer Console 页面（单页、零外部资源）。
	// 放在最后注册：ServeMux 以「最长前缀」匹配，不会遮蔽上面的精确路由。
	mux.HandleFunc("/", s.handleConsole)
	return mux
}

// Start 在 addr 上启动服务，返回实际监听地址（addr 端口为 0 时由系统分配）。
func (s *Server) Start(addr string) (string, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("控制接口监听失败: %w", err)
	}
	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
	}
	s.mu.Lock()
	s.ln = ln
	s.http = srv
	s.mu.Unlock()

	// 关键：后台 serve goroutine 必须捕获「局部」 srv，而非读取字段 s.http。
	// Stop() 会在关闭后将 s.http 置为 nil；若此处读字段，则 Stop 与 goroutine 存在
	// 数据竞争，且 goroutine 可能在字段被置 nil 后才执行 Serve，触发 nil 解引用崩溃
	// （表现为 net/http.(*Server).Serve(0x0, ...) 的并发 panic，会打断调用方的清理链）。
	// 捕获局部变量后，goroutine 永远持有有效 *http.Server，Stop 经同一指针 Close 即可让
	// Serve 干净返回，无竞争、无崩溃。
	go func() { _ = srv.Serve(ln) }()
	return ln.Addr().String(), nil
}

// SetStopHook 注入「收到停止请求」的回调（PHASE PRODUCT-DEV-1B）。
//
// 回调由节点侧提供，语义是「请求节点优雅退出」，而不是「在这里执行关闭」——
// 真正的关闭仍由节点主流程统一执行，从而保证 /stop 与 SIGINT/SIGTERM
// 走完全相同的清理链（控制接口 → P2P → 存储 → 数据目录锁）。
//
// 未注入时 POST /stop 返回 501，而不是伪装成成功。
// 回调最多被调用一次：重复的 /stop 请求幂等，不会重复触发关闭。
func (s *Server) SetStopHook(hook func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopHook = hook
}

// Addr 返回实际监听地址（Start 之前为空）。
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

// Stop 关闭控制接口。
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.http == nil {
		return nil
	}
	err := s.http.Close()
	s.http = nil
	s.ln = nil
	return err
}

// ---- 处理函数 ----

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	info, err := s.node.Status()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	addr := r.URL.Query().Get("address")
	if addr == "" {
		writeError(w, http.StatusBadRequest, ErrAddressRequired)
		return
	}
	info, err := s.node.Balance(addr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleUTXOs(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	addr := r.URL.Query().Get("address")
	if addr == "" {
		writeError(w, http.StatusBadRequest, ErrAddressRequired)
		return
	}
	list, err := s.node.UTXOs(addr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if list == nil {
		list = []UTXOInfo{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req SendRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("请求体解析失败: %w", err))
		return
	}
	resp, err := s.node.Send(req.To, req.Amount, req.Fee)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleBlock(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	raw := r.URL.Query().Get("height")
	height, err := strconv.Atoi(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("height 参数非法: %q", raw))
		return
	}
	encoded, err := s.node.BlockHex(height)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"height": height, "encoded": encoded})
}

func (s *Server) handleMine(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	// 请求体可省略：默认挖 1 个区块
	req := MineRequest{Count: 1}
	if r.Body != nil {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil && err != io.EOF {
			writeError(w, http.StatusBadRequest, fmt.Errorf("请求体解析失败: %w", err))
			return
		}
	}
	if req.Count <= 0 || req.Count > MaxMineCount {
		writeError(w, http.StatusBadRequest,
			fmt.Errorf("count 必须在 1..%d 之间，实际 %d", MaxMineCount, req.Count))
		return
	}
	resp, err := s.node.Mine(req.Count)
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// MaxMineCount 单次按需出块的上限，避免一次请求长时间占用节点。
const MaxMineCount = 1000

// handleStop 请求节点优雅停止（PHASE PRODUCT-DEV-1B）。
//
// 仅接受 POST：停止是破坏性动作，绝不能由 GET 触发——浏览器预取、
// 控制台页面刷新、爬虫抓取都可能发出 GET，那会让节点被间接地关掉。
//
// 顺序至关重要：先把响应写出并 flush，再触发停止回调。
// 若反过来，控制接口会在响应到达调用方之前被关闭，调用方只能看到
// 「连接被重置」，无法区分「已接受」与「节点崩了」。
func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	s.mu.Lock()
	hook := s.stopHook
	s.mu.Unlock()
	if hook == nil {
		writeError(w, http.StatusNotImplemented, ErrStopUnsupported)
		return
	}
	writeJSON(w, http.StatusOK, StopResponse{
		Accepted: true,
		Message:  "节点已接受停止请求，正在释放数据目录锁并退出",
	})
	// 确保响应字节已经离开本进程，再开始关闭控制接口。
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	s.stopOnce.Do(hook)
}

// requireMethod 校验请求方法；不符时写 405 并返回 false。
func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 %s", method))
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, errorResponse{Error: err.Error()})
}
