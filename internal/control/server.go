// Package control 提供节点的本地控制接口（JSON over HTTP），供命令行工具
// 查询状态、查看余额与提交交易。
//
// 设计取舍：
//   - 只做本机控制，不做远程钱包服务。默认绑定 127.0.0.1。
//   - 读端点（GET /status /balance /utxos /block /blocks /logs）无鉴权——
//     回环绑定即是其安全边界（Explorer 依赖这些端点，保持免认证）。
//   - mutation 端点（POST /send /mine /stop）自 PHASE CONTROL-AUTH-1 起要求
//     Bearer Token（授权头：Authorization: Bearer <token>），认证失败统一 401
//     并带固定延迟；未配置 token 时 fail-closed（一律 401）。
//   - 即便如此仍绝不可绑定到公网地址（远程管理走 SSH 隧道，属 Option A 基线）。
//   - 本包不依赖 blockchain / mempool / utxo——它只声明一个 Node 接口，
//     由 cmd/node 侧的节点服务实现（消费方定义接口）。这样协议与业务状态解耦，
//     本包可以用假实现独立测试。
package control

import (
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrAddressRequired 请求缺少 address 参数。
var ErrAddressRequired = errors.New("缺少 address 参数")

// ErrBlockNotFound 按哈希查询的区块不存在（HTTP 404）。
var ErrBlockNotFound = errors.New("区块不存在")

// ErrAmbiguousBlockQuery 同时提供 height 与 hash 参数（二者互斥）。
var ErrAmbiguousBlockQuery = errors.New("height 与 hash 参数只能提供其一")

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
	// BlocksPage 按高度批量返回 canonical 主链区块的 Explorer JSON 视图。
	// from 为起始高度（含），count 为请求块数（1..MaxBlocksPerPage）。
	// 一次调用一次临界区完成批量读取（复用 blockchain.BlocksFrom），
	// 空区间（from 超过链尾）返回空 blocks 列表而非错误。
	BlocksPage(from, count int) (BlocksPageResult, error)
	// BlockJSONByHash 按区块哈希（32 字节）返回区块的 Explorer JSON 视图。
	// 复用 blockchain.BlockByHash（canonical 链 → store fallback）；
	// 未命中返回 ErrBlockNotFound（由 HTTP 层映射为 404）。
	BlockJSONByHash(hash [32]byte) (BlockJSON, error)
	// Mine 按需立即挖出 count 个区块（测试网/开发用，对标 bitcoind 的 generatetoaddress）。
	// 若节点正在持续挖矿（-mine）则返回错误，避免两个挖矿路径互相干扰。
	Mine(count int) (MineResponse, error)
	// StartMining 启动持续挖矿（PHASE MINING-LIFECYCLE-1，设计冻结）。
	// 单飞语义：同一时间最多一个 miner instance；重复/并发 START 返回
	// *MineConflictError（HTTP 409），无副作用；FAILED 状态拒绝且不自动清除。
	StartMining() (MineStartResponse, error)
	// StopMining 仅停止挖矿（停挖 ≠ 停节点）：节点/P2P/RPC/Explorer 全部存活。
	// 幂等：从未启动、已停止、重复 STOP 均成功返回；绝不触碰 node stopCh。
	StopMining() (MineStopResponse, error)
}

// StatusInfo 节点状态。
type StatusInfo struct {
	Height      int      `json:"height"`
	TipHash     string   `json:"tip_hash"`
	Peers       []string `json:"peers"`
	MempoolSize int      `json:"mempool_size"`
	// Mining 表示**挖矿循环是否处于活动状态**（含 STARTING/RUNNING/STALLED），
	// 不表示「正在对合法候选区块执行 PoW」。判断真实挖矿语义请用 MiningState。
	Mining  bool   `json:"mining"`
	Address string `json:"address"` // 节点钱包地址

	// ---- 挖矿运行时语义状态（PHASE MINING-REMEDIATION-1，新增字段）----
	//
	// 这些字段是**新增**的，不改变既有字段语义，因此对既有调用方保持兼容。
	// 它们存在的理由：修复前控制接口只能看到 Mining=true，无法区分
	// 「正在有效挖矿」「因补贴耗尽而不存在合法候选（STALLED）」「结构性错误（FAILED）」。

	// MiningState 是挖矿运行时的语义状态：
	// STOPPED / STARTING / RUNNING / STALLED / FAILED / STOPPING。
	// 权威字段——RUNNING 才表示正在对**已通过预校验的**模板求解 PoW。
	MiningState string `json:"mining_state"`
	// MiningReason 是最近一次状态变化的原因（如 subsidy-exhausted-no-fee-tx）。
	MiningReason string `json:"mining_reason"`
	// PowAttempts 是自启动以来累计实际执行的 PoW 尝试次数（双 SHA-256 计数）。
	// 在「不存在合法候选」时该值必须零增长。
	PowAttempts uint64 `json:"pow_attempts"`
	// MiningRetries 是模板重建/重试次数。
	MiningRetries int64 `json:"mining_retries"`
	// AcceptedBlocks / RejectedBlocks 是自启动以来成功上链与被拒计数。
	AcceptedBlocks int64 `json:"accepted_blocks"`
	RejectedBlocks int64 `json:"rejected_blocks"`

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

// MineStartResponse POST /mine/start 的结果（PHASE MINING-LIFECYCLE-1，设计冻结契约）。
type MineStartResponse struct {
	Accepted bool   `json:"accepted"`
	State    string `json:"state"`
	Height   int    `json:"height"`
}

// MineStopResponse POST /mine/stop 的结果。STOP 幂等：重复/空闲时 Accepted 仍为
// true，State 返回真实 runtime 状态（STOPPING / STOPPED / FAILED）。
type MineStopResponse struct {
	Accepted bool   `json:"accepted"`
	State    string `json:"state"`
}

// MineConflictError 表示 START/STOP 与当前 mining lifecycle 状态冲突（HTTP 409）。
// State 携带当前 runtime 状态，供响应体 {"error","state"} 使用。
type MineConflictError struct {
	Message string
	State   string
}

func (e *MineConflictError) Error() string { return e.Message }

// MaxBlocksPerPage GET /blocks 单次请求的区块数上限（PHASE EXPLORER-API-IMPLEMENTATION-1 冻结预算）。
const MaxBlocksPerPage = 100

// TxJSON Explorer 交易摘要。
//
// 字段全部来自 transaction.Transaction 真实结构，无虚构：
// TxID = serializeForHash 的 SHA-256；coinbase 由 IsCoinbase() 判定；
// total_out = Σoutputs.Value。
//
// Fee 仅在可**零成本诚实计算**时出现（coinbase 恒为 0）：普通交易的输入金额
// 需要历史输出索引（被花费的输出已不在当前 UTXO 快照中，当前无该索引，
// 属设计审计登记的 Tier 2 能力）→ 省略该字段，绝不伪装成 0。
type TxJSON struct {
	TxID        string `json:"txid"`
	Coinbase    bool   `json:"coinbase"`
	InputCount  int    `json:"input_count"`
	OutputCount int    `json:"output_count"`
	TotalOut    uint64 `json:"total_out"`
	Fee         *int64 `json:"fee,omitempty"`
}

// BlockJSON 单个区块的 Explorer JSON 视图。
//
// 字段全部来自 block.Header / block.Block / 派生量（设计审计 §5 冻结契约）：
// hash = SerializeHeader 的双 SHA-256；difficulty = 2^(Bits-MaxTargetBits)（纯展示）；
// size = len(Encode())；consensus_era 按硬分叉激活高度标注（纯展示，不改共识）。
//
// Height 仅在真实可知时出现：canonical 块高度可得；detached 块无公开高度
// 来源（新增高度索引需触碰 blockchain 边界，本阶段禁止）→ 省略。
type BlockJSON struct {
	Height       *int     `json:"height,omitempty"`
	Hash         string   `json:"hash"`
	PreviousHash string   `json:"previous_hash"`
	Timestamp    int64    `json:"timestamp"`
	Version      uint32   `json:"version"`
	Bits         uint32   `json:"bits"`
	Difficulty   float64  `json:"difficulty"`
	Nonce        uint64   `json:"nonce"`
	MerkleRoot   string   `json:"merkle_root"`
	Size         int      `json:"size"`
	TxCount      int      `json:"tx_count"`
	Canonical    bool     `json:"canonical"`
	Persisted    bool     `json:"persisted"`
	Transactions []TxJSON `json:"transactions"`
	// ConsensusEra "pre-hardfork"（height < 激活高度）或 "post-hardfork"。
	ConsensusEra string `json:"consensus_era"`
}

// BlocksPageResult GET /blocks 响应。
//
// blocks 按**高度升序**（链序）返回；latest-first 分页语义由调用方以
// from = max(0, 链尾高度-页大小+1) 表达（设计审计 §6 冻结的页面语义）。
// at_tip = 本次返回区间已到达链尾（from 超过链尾时同样为 true，blocks 为空）。
type BlocksPageResult struct {
	From      int         `json:"from"`
	Requested int         `json:"requested"`
	Returned  int         `json:"returned"`
	AtTip     bool        `json:"at_tip"`
	Height    int         `json:"height"` // 查询时刻的链尾高度（供分页计算）
	Blocks    []BlockJSON `json:"blocks"`
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

	// authToken 为 mutation 端点（/send /mine /stop）的 Bearer Token
	//（PHASE CONTROL-AUTH-1，Option E-lite 冻结设计）。
	// 为空（未配置）时 mutation 端点 fail-closed：一律 401；读端点不受影响。
	authToken string
	// authFailureDelay 为认证失败后的固定延迟（防爆破）。
	// 设计冻结 500ms；测试可覆写为小值。无锁定/无黑名单/无全局限流。
	authFailureDelay time.Duration
}

// defaultAuthFailureDelay 认证失败固定延迟（设计报告 §8 冻结值）。
const defaultAuthFailureDelay = 500 * time.Millisecond

// token 长度边界（LoadTokenFile 归一化校验用，冻结于实现报告）。
const (
	minTokenLen = 16
	maxTokenLen = 1024
)

// NewServer 创建服务端。
func NewServer(node Node) *Server {
	return &Server{node: node, authFailureDelay: defaultAuthFailureDelay}
}

// SetAuthToken 设置 mutation 端点（/send /mine /stop）的 Bearer Token。
// 必须在 Start 之前调用。传入空串等价于「未配置」：mutation 端点保持
// fail-closed（一律 401）。token 来源文件请用 LoadTokenFile 读取，
// 绝不通过命令行参数或环境变量传递 token 本身。
func (s *Server) SetAuthToken(tok string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authToken = tok
}

// SetAuthFailureDelay 覆写认证失败固定延迟（仅供测试；生产保持默认 500ms）。
// 传入 0 表示测试中禁用延迟。不允许负值。
func (s *Server) SetAuthFailureDelay(d time.Duration) {
	if d < 0 {
		d = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authFailureDelay = d
}

// Handler 返回路由（便于测试直接挂到 httptest）。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/balance", s.handleBalance)
	mux.HandleFunc("/utxos", s.handleUTXOs)
	mux.HandleFunc("/send", s.requireAuth(s.handleSend))
	mux.HandleFunc("/mine", s.requireAuth(s.handleMine))
	// PHASE MINING-LIFECYCLE-1：持续挖矿生命周期控制（设计冻结契约）。
	// /mine/start 与 /mine/stop 仅影响挖矿循环（独立 minerStop 通道），
	// 绝不触碰节点停机路径（/stop 语义保持不变）。
	mux.HandleFunc("/mine/start", s.requireAuth(s.handleMineStart))
	mux.HandleFunc("/mine/stop", s.requireAuth(s.handleMineStop))
	// PHASE CONSOLE-MINE-AUTH-FIX-1：Developer Console 页面专用出块端点。
	// 控制台页面不持有任何凭据（浏览器零凭据是既有设计红线），因此不能走
	// requireAuth；改用「同源闸门」恢复其「立即出块」按钮（见 consoleOriginGate）。
	// 与 /mine 并存：CLI / 第三方脚本继续用 /mine + Bearer Token，语义不变。
	mux.HandleFunc("/console/mine", s.consoleOriginGate(s.handleMine))
	mux.HandleFunc("/block", s.handleBlock)
	mux.HandleFunc("/blocks", s.handleBlocks)
	mux.HandleFunc("/logs", s.handleLogs)
	mux.HandleFunc("/stop", s.requireAuth(s.handleStop))
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
	// PHASE EXPLORER-API-IMPLEMENTATION-1：新增 hash= 查询路径（与 height= 互斥）。
	// 既有 height= → hex 编码契约保持逐字节不变（回归由既有测试保证）。
	hashRaw := r.URL.Query().Get("hash")
	if hashRaw != "" {
		if r.URL.Query().Get("height") != "" {
			writeError(w, http.StatusBadRequest, ErrAmbiguousBlockQuery)
			return
		}
		// 严格遵循现有编码规范：64 个十六进制字符 = 32 字节区块哈希。
		if len(hashRaw) != 64 {
			writeError(w, http.StatusBadRequest, fmt.Errorf("hash 长度非法: 需要 64 个十六进制字符，实际 %d", len(hashRaw)))
			return
		}
		hashBytes, err := hex.DecodeString(hashRaw)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("hash 非法十六进制: %w", err))
			return
		}
		var hash [32]byte
		copy(hash[:], hashBytes)
		bj, err := s.node.BlockJSONByHash(hash)
		if err != nil {
			if errors.Is(err, ErrBlockNotFound) {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, bj)
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

// handleBlocks 处理 GET /blocks?from=&count=（PHASE EXPLORER-API-IMPLEMENTATION-1）。
//
// 契约（授权 §4 冻结）：
//   - from/count 均为必填整数；from < 0、count 不在 1..MaxBlocksPerPage → 400；
//   - from 超过链尾 = 合法空区间 → 200 + 空 blocks 列表（不是 500）；
//   - 仅返回 canonical 主链区块（由 Node 实现经 blockchain.BlocksFrom 保证，
//     detached 块绝不混入）；
//   - 一次 Node 调用一次临界区完成批量读取，禁止 N 次 /block 分页。
func (s *Server) handleBlocks(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	from, err := strconv.Atoi(r.URL.Query().Get("from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("from 参数非法: %q", r.URL.Query().Get("from")))
		return
	}
	if from < 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("from 不能为负，实际 %d", from))
		return
	}
	count, err := strconv.Atoi(r.URL.Query().Get("count"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("count 参数非法: %q", r.URL.Query().Get("count")))
		return
	}
	if count < 1 || count > MaxBlocksPerPage {
		writeError(w, http.StatusBadRequest, fmt.Errorf("count 必须在 1..%d 之间，实际 %d", MaxBlocksPerPage, count))
		return
	}
	page, err := s.node.BlocksPage(from, count)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
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

// handleMineStart 启动持续挖矿（PHASE MINING-LIFECYCLE-1，设计冻结契约）。
//
// 契约：body 可省略或 {}（DisallowUnknownFields，未知字段 400——保留扩展位但不静默吞错）；
// 200 {"accepted","state","height"}；409 {"error","state"}（已在跑/FAILED/STOPPING，
// reject-duplicate 语义，无副作用）；401 认证失败（requireAuth）；405 非 POST。
// 状态以 runtime 为唯一事实源：响应只反映受理瞬间状态，最终状态以 GET /status 为准。
func (s *Server) handleMineStart(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if r.Body != nil {
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12))
		dec.DisallowUnknownFields()
		var req struct{}
		if err := dec.Decode(&req); err != nil && err != io.EOF {
			writeError(w, http.StatusBadRequest, fmt.Errorf("请求体解析失败（仅接受空体或 {}）: %w", err))
			return
		}
	}
	resp, err := s.node.StartMining()
	if err != nil {
		var cf *MineConflictError
		if errors.As(err, &cf) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": cf.Message, "state": cf.State})
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleMineStop 仅停止挖矿，绝不停节点（PHASE MINING-LIFECYCLE-1，设计冻结契约）。
//
// 幂等：从未启动 / 已停止 / 重复 STOP 一律 200 Accepted=true，State 返回真实
// runtime 状态；FAILED 状态下无可停 miner，保持 FAILED（不抹掉失败证据）。
// 响应返回受理状态（STOPPING），最终 STOPPED 由挖矿循环收尾后经 /status 可见。
func (s *Server) handleMineStop(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	resp, err := s.node.StopMining()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

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

// requireAuth 包装 mutation 端点：强制 Bearer Token 认证（PHASE CONTROL-AUTH-1）。
//
// 契约（CONTROL-PLANE AUTH DESIGN 冻结）：
//   - 缺失/无效/过期/被撤销 token 统一返回 401，响应不区分具体原因；
//   - 认证失败先做固定延迟 authFailureDelay（防爆破；无锁定/无黑名单/无全局限流）；
//   - token 比较使用 constant-time（crypto/subtle），不引入自研密码学；
//   - 不打印、不回显 Authorization 头；失败响应体最小化（固定 "unauthorized"）。
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 方法契约优先：非 POST 请求不可能是 mutation，直接交给既有方法守卫，
		// 保持「GET /stop → 405 + Allow: POST」的既有行为（不因新增认证而改变）。
		if r.Method != http.MethodPost {
			next(w, r)
			return
		}

		s.mu.Lock()
		expected := s.authToken
		delay := s.authFailureDelay
		s.mu.Unlock()

		given := bearerToken(r)
		ok := expected != "" && given != "" &&
			subtle.ConstantTimeCompare([]byte(expected), []byte(given)) == 1
		if !ok {
			if delay > 0 {
				time.Sleep(delay)
			}
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
			return
		}
		next(w, r)
	}
}

// bearerToken 提取 Authorization: Bearer <token>；头缺失或格式不符返回空串。
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return h[len(prefix):]
}

// consoleOriginGate 包装「Developer Console 页面专用」的 mutation 端点。
//
// 背景（CONTROL-AUTH-1 回归，PHASE CONSOLE-MINE-AUTH-FIX-1）：
// 控制台页面于 2026-09-12 冻结时控制接口尚无鉴权，页面直接 POST /mine。
// 2026-09-17 引入 mutation Bearer Token 后 /mine 被 requireAuth 保护，
// 而控制台页面**按设计不持有任何凭据**（浏览器零凭据红线），于是「立即出块」
// 按钮自那时起恒返回 401；页面却仍渲染「可用：POST /mine count=1」，
// 构成**虚假可用性声明**（实测：无 token POST /mine → 401，带 token → 200）。
//
// 本闸门以「同源」判定恢复该按钮，且**不引入任何浏览器可见凭据**
// （无 token、无 cookie、无 URL 参数、不注入 HTML），因此不触碰
// CONTROL-AUTH-1 冻结的凭据边界——mutation Bearer Token 依旧绝不出现在浏览器侧。
//
// 判定（fail-closed）：
//   - Sec-Fetch-Site 存在 → 必须等于 "same-origin"（该头由浏览器自动附加，属
//     forbidden header name，页面 JS 既不能设置也不能删除，无法伪造）；
//   - Sec-Fetch-Site 缺失 → Origin 必须严格等于 http(s)://<r.Host>（旧浏览器兜底）；
//   - 两者皆不满足 → 403。
//
// 威胁模型：跨站页面（CSRF）与 DNS rebinding 的 Sec-Fetch-Site / Origin 均非本
// 服务源 ⇒ 被拒；非浏览器本地进程无法伪造 Sec-Fetch-Site，应改用 /mine + Bearer
// Token（该路径不受本闸门影响）。
func (s *Server) consoleOriginGate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 方法契约优先：非 POST 交给既有方法守卫，保持 405 + Allow 语义不变。
		if r.Method != http.MethodPost {
			next(w, r)
			return
		}
		if !isSameOriginRequest(r) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "forbidden"})
			return
		}
		next(w, r)
	}
}

// isSameOriginRequest 判定请求是否来自本服务自身的浏览器同源上下文。
// 无同源证据一律返回 false（fail-closed）。
func isSameOriginRequest(r *http.Request) bool {
	// Sec-Fetch-Site 一旦出现即以其为准：浏览器必然填写，且属 forbidden header，
	// 页面 JS 无法伪造或删除；不允许它在「非 same-origin」时回落到 Origin 分支，
	// 否则等于给了伪造者可乘之隙。
	if sfs := r.Header.Get("Sec-Fetch-Site"); sfs != "" {
		return sfs == "same-origin"
	}
	// 兜底：严格比对 Origin（同源 POST 恒带 Origin；缺失即无证据）。
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	return origin == "http://"+r.Host || origin == "https://"+r.Host
}

// LoadTokenFile 从 path 读取 Bearer Token 并做归一化。
//
// 归一化语义（实现报告冻结）：去除首尾空白（兼容编辑器追加的换行/CRLF）；
// 归一化后长度必须在 [minTokenLen, maxTokenLen] 区间。
// Unix 上要求文件权限 owner-only（0600），group/other 位非零即拒绝；
// Windows 文件系统不表达 POSIX 权限位，跳过该检查。
// 错误信息只描述原因与文件路径，绝不包含文件内容。
func LoadTokenFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取 token 文件 %s 失败: %w", path, err)
	}
	tok := strings.TrimSpace(string(raw))
	if n := len(tok); n < minTokenLen {
		return "", fmt.Errorf("token 文件 %s 内容无效（归一化后长度 %d，要求 >= %d）", path, n, minTokenLen)
	} else if n > maxTokenLen {
		return "", fmt.Errorf("token 文件 %s 内容无效（归一化后长度 %d，要求 <= %d）", path, n, maxTokenLen)
	}
	if runtime.GOOS != "windows" {
		if fi, statErr := os.Stat(path); statErr == nil {
			if perm := fi.Mode().Perm(); perm&0o077 != 0 {
				return "", fmt.Errorf("token 文件 %s 权限过宽（%04o，要求 owner-only 0600）", path, perm)
			}
		}
	}
	return tok, nil
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
