// Package p2p 实现 P2P 网络层：TCP + 换行分隔 JSON 消息协议，承载握手、区块广播、
// 交易广播与新节点同步（GetBlocks / BlocksResp）。
//
// 架构：本包只负责「传输与连接管理」，不关心共识语义——收到的原始载荷交给上层
// 实现的 Handler（区块验证、交易校验、入池、上链等由 blockchain/mempool 层完成）。
//
// 协议：每条消息为一行 JSON（以 '\n' 结尾），信封结构为 Message{Type, Payload}。
// 区块与交易在 Payload 内以其规范编码的十六进制字符串传输（block.Encode()），
// 保证与磁盘/共识层使用同一字节表示，避免 JSON 结构体演进带来的不一致。
//
// 安全与健壮性措施（学习项目范围内）：
//   - 单条消息大小上限（MaxMessageSize），防止恶意超大帧耗尽内存；
//   - 读取超时与写超时，避免死连接长期占用；
//   - 发送失败达到阈值即断开并清理，避免僵尸连接；
//   - 消息按类型分发，未知类型忽略并记录；
//   - 广播支持 except 参数，避免中继回环导致的广播风暴；
//   - R1-A：每对端有界出站队列 + 独立 writer goroutine——广播只做非阻塞入队，
//     慢对端只能拖垮自己的队列（溢出丢帧），绝不阻塞调用方（尤其是挖矿临界区）
//     与其他健康对端的投递。
package p2p

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"p2pchain/internal/obs"
)

// MessageType 标识网络消息的类型。
type MessageType string

const (
	MsgHandshake       MessageType = "handshake"          // 握手：交换版本/高度/工作量/链尾/监听地址/已知节点
	MsgNewBlock        MessageType = "new_block"          // 广播新区块（Payload = BlockPayload）
	MsgNewTx           MessageType = "new_tx"             // 广播新交易（Payload = TxPayload）
	MsgGetBlocks       MessageType = "get_blocks"         // 请求从某高度开始的区块（Payload = GetBlocksPayload）
	MsgBlocksResp      MessageType = "blocks_resp"        // 区块请求响应（Payload = BlocksRespPayload）
	MsgGetBlockByHash  MessageType = "get_block_by_hash"  // 按哈希请求区块及其祖先（Payload = GetBlockByHashPayload）
	MsgBlockByHashResp MessageType = "block_by_hash_resp" // 按哈希请求响应（Payload = BlockByHashRespPayload）
)

const (
	// MaxMessageSize 单条消息上限（1 MiB），与区块体积上限同量级。
	MaxMessageSize = 1 << 20
	// MaxBlocksPerResp 单次同步响应最多携带的区块数。
	MaxBlocksPerResp = 500
	// MaxAncestorsPerResp 单次「按哈希取块」响应最多回溯的祖先数（REORG-1H）。
	//
	// 分支拉取必须**有界**：孤儿块不知道缺口有多深，若无上限，一个恶意/故障对端
	// 可以诱导本节点在一条长链上无限回溯，把内存与带宽吃光。
	// 64 个祖先足以覆盖「几分钟分区」级别的正常分叉；更深的缺口由请求方
	// 分多轮补齐（每轮同样有上限与轮次上限）。
	MaxAncestorsPerResp = 64
	// handshakeTimeout 建立连接后必须在该时间内收到握手消息（R1-B 起真正强制：
	// 未完成握手前每次读都以该值为 deadline，超时即断开）。
	handshakeTimeout = 10 * time.Second
	// R1-B（SEC-CLOSE MUST FIX 2）：连接资源限额，堵住无上限 accept 的资源耗尽面
	//（连接洪水 / 慢握手槽位占用 / 日食式连接挤占）。测试按生产值直接验证。
	maxInbound     = 125 // 入站连接上限（含未完成握手者）
	maxPeers       = 128 // 对端总数上限（入站 + 外拨）
	handshakeQuota = 32  // 同时处于「已注册未握手」状态的连接配额
	// readTimeout / writeTimeout 单次读写超时。
	readTimeout  = 60 * time.Second
	writeTimeout = 10 * time.Second
	// maxSendFailures 连续发送失败阈值，达到即断开该连接。
	maxSendFailures = 3
	// outboundQueueCap R1-A：每对端出站队列容量（消息条数，有界）。
	// 写端被慢对端卡住时，队列填满后新广播对该对端直接丢弃（非阻塞语义），
	// 丢帧的对端靠既有同步/孤儿恢复机制补齐。
	outboundQueueCap = 256
	// dispatchQueueCap P0-2：每对端分发队列容量（消息条数，有界）。
	// 读循环只做帧解析与非阻塞入队，重业务（区块 PoW 校验、磁盘 IO）由该连接
	// 专属的单个分发 worker 串行消费——慢业务不再卡住读循环，恶意对端也无法
	// 用消息流水线拖住连接。队列满时丢弃并记数（观测），不阻塞读循环。
	dispatchQueueCap = 64
)

// Message 传输信封；Payload 按 Type 解析为不同结构。
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// HandshakePayload 握手信息。
//
// REORG-1H 新增 ChainWork / TipHash 两个**可选**字段：
//   - 两者都是「缺失即未知」语义（零值 = 对端未提供），旧版本节点发来的握手
//     不含这两个字段，接收方自动退化为「按高度比较」的既有行为，协议向后兼容；
//   - ChainWork 让同步判据从「谁更高」升级为「谁的工作量更大」（work-aware）；
//   - TipHash 让本节点能发现「对端在一条我不认识的分支上」，从而按哈希拉分支。
type HandshakePayload struct {
	NodeID      string   `json:"node_id"`
	ChainHeight int      `json:"chain_height"`
	ListenAddr  string   `json:"listen_addr"`
	GenesisHash string   `json:"genesis_hash"` // 用于快速识别网络不一致
	KnownPeers  []string `json:"known_peers"`  // 节点发现：我方已知的其他节点
	ChainWork   string   `json:"chain_work"`   // 链尾累积工作量（十进制字符串；空 = 未知）
	TipHash     string   `json:"tip_hash"`     // 链尾区块哈希（十六进制；空 = 未知）
}

// BlockPayload 区块广播载荷（十六进制编码的规范区块字节）。
type BlockPayload struct {
	Encoded string `json:"encoded"`
}

// TxPayload 交易广播载荷（十六进制编码的规范交易字节）。
type TxPayload struct {
	Encoded string `json:"encoded"`
}

// GetBlocksPayload 区块同步请求：从 FromHeight 开始（含）请求最多 Count 个区块。
type GetBlocksPayload struct {
	FromHeight int `json:"from_height"`
	Count      int `json:"count"`
}

// BlocksRespPayload 区块同步响应。
type BlocksRespPayload struct {
	EncodedBlocks []string `json:"encoded_blocks"`
	Done          bool     `json:"done"` // 是否已到请求方链尾
}

// GetBlockByHashPayload 按哈希请求一个区块及其祖先（REORG-1H）。
//
// 语义：请把哈希为 Hash 的区块给我，并沿其 PrevBlockHash 回溯最多 MaxAncestors 个祖先。
// 接收方用于对缺父的孤儿块补齐缺口；MaxAncestors 由服务端按 MaxAncestorsPerResp 裁剪。
type GetBlockByHashPayload struct {
	Hash         string `json:"hash"`          // 请求的区块哈希（十六进制，64 字符）
	MaxAncestors int    `json:"max_ancestors"` // 期望回溯的祖先数（服务端仍会裁剪）
}

// BlockByHashRespPayload 按哈希请求的响应（REORG-1H）。
//
// Blocks 的顺序固定为：**索引 0 = 被请求的区块，其后依次是父、祖父……（由新到旧）**。
// 请求方从后往前应用即可天然满足「父先于子」的插入约束。
// Hash 回显请求中的哈希，便于请求方在并发/多轮场景下定位在途请求。
// Found=false 表示对端没有这个区块（响应体 Blocks 为空）。
type BlockByHashRespPayload struct {
	Hash   string   `json:"hash"`
	Blocks []string `json:"blocks"`
	Found  bool     `json:"found"`
}

// Handler 由上层（节点服务）实现，处理各类消息的业务语义。
type Handler interface {
	OnHandshake(peerAddr string, payload HandshakePayload)
	OnNewBlock(peerAddr string, raw json.RawMessage)
	OnNewTx(peerAddr string, raw json.RawMessage)
	OnGetBlocks(peerAddr string, payload GetBlocksPayload)
	OnBlocksResp(peerAddr string, payload BlocksRespPayload)
	OnGetBlockByHash(peerAddr string, payload GetBlockByHashPayload)
	OnBlockByHashResp(peerAddr string, payload BlockByHashRespPayload)
}

// outboundMsg 出站队列元素：已序列化的完整线上帧（含结尾 '\n'）。
// 广播路径对同一条消息只做一次 Marshal，随后逐对端入队共享同一底层数组（只读）。
type outboundMsg struct {
	data  []byte
	mtype string // 观测用（WRITE_TIMEOUT / SEND_ERROR / OVERFLOW 事件载荷）
}

// dispatchJob 分发队列元素：读循环解析出的待业务处理消息。
type dispatchJob struct {
	peerAddr string
	msg      Message
}

// Peer 一条已建立的连接及其状态。
type Peer struct {
	Addr        string
	Conn        net.Conn
	mu          sync.Mutex
	sendFails   int
	handshaked  atomic.Bool // R1-B：原子化——accept 侧限额统计需无锁读取
	lastActive  time.Time
	pendingResp int
	inbound     bool // R1-B：入站/外拨标记（限额统计用）

	// R1-A 出站队列：广播只入队，由该连接唯一的 writer goroutine 串行写出。
	outQ       chan outboundMsg
	closed     chan struct{} // 连接终结信号（dropPeer / handleConn 清理时关闭）
	closeOnce  sync.Once
	queueDrops atomic.Uint64 // 队列满被丢弃的广播条数（观测用）
	// P0-2 分发队列：读循环只做帧解析与非阻塞入队，重业务由该连接唯一的
	// 分发 worker 串行消费（见 peerDispatchWorker）。
	dispatchQ     chan dispatchJob
	dispatchDrops atomic.Uint64 // 分发队列满被丢弃的消息条数（观测用）
}

// newPeer 构造 Peer 并初始化出站队列（R1-A）。
func newPeer(addr string, conn net.Conn, inbound bool) *Peer {
	return &Peer{
		Addr:       addr,
		Conn:       conn,
		inbound:    inbound,
		lastActive: time.Now(),
		outQ:       make(chan outboundMsg, outboundQueueCap),
		dispatchQ:  make(chan dispatchJob, dispatchQueueCap),
		closed:     make(chan struct{}),
	}
}

// signalClosed 发出连接终结信号（幂等）；出站 writer 收到后退出。
func (p *Peer) signalClosed() {
	p.closeOnce.Do(func() { close(p.closed) })
}

// Node 一个 P2P 节点。
type Node struct {
	listenAddr  string
	nodeID      string
	genesisHash string
	handler     Handler

	heightFn func() int // 链高度提供者（由上层注入）
	// statusFn 提供链尾工作量与链尾哈希（REORG-1H，work-aware 握手 + 分支发现）。
	// 返回 (累积工作量十进制字符串, 链尾哈希十六进制字符串)；空串表示未知。
	statusFn func() (work string, tipHash string)

	mu       sync.RWMutex
	peers    map[string]*Peer // 远端地址 → 连接
	known    map[string]struct{}
	closing  bool
	listener net.Listener
}

// NewNode 创建节点；nodeID / genesisHash 用于握手时的网络识别。
func NewNode(listenAddr, nodeID, genesisHash string, handler Handler) *Node {
	return &Node{
		listenAddr:  listenAddr,
		nodeID:      nodeID,
		genesisHash: genesisHash,
		handler:     handler,
		peers:       make(map[string]*Peer),
		known:       make(map[string]struct{}),
	}
}

// Start 启动监听（阻塞调用，通常在独立 goroutine 中运行）。
// Start 同步完成端口绑定并返回实际地址，随后在后台 goroutine 接受连接。
//
// PHASE TEST-INFRASTRUCTURE-REMEDIATION-1：Start 返回时 listener 必已就绪——
// 这是 ListenAddr() 的就绪契约（调用方拿到的是已解析的真实地址，
// 127.0.0.1:0 这类配置占位值不会再作为运行时监听地址出现）。
// 修复前 Start 阻塞式注入 Serve，调用方只能丢进 goroutine 异步等待，
// 导致「newNodeRuntime 返回但 listener 尚未赋值」的竞态
// （TestSeedReconnectAfterRestart 偶发 30s 超时的根因）。
func (n *Node) Start() error {
	ln, err := net.Listen("tcp", n.listenAddr)
	if err != nil {
		return fmt.Errorf("监听失败: %w", err)
	}
	n.mu.Lock()
	if n.closing {
		n.mu.Unlock()
		_ = ln.Close()
		return nil
	}
	n.listener = ln
	n.mu.Unlock()
	log.Printf("[p2p] 节点已启动，监听 %s（nodeID=%s）", ln.Addr().String(), n.nodeID)
	go n.acceptLoop(ln)
	return nil
}

// Serve 在调用方提供的监听器上接受连接（阻塞）。
//
// 与 Start 的区别：监听器由外部创建并注入。这样上层可以先绑定端口、拿到真实地址，
// 再把监听器交给节点——把「端口已就绪」变成确定性事实，避免测试里用
// 「探测端口能否连接」来猜测就绪状态（那种做法本身会引入竞态：
// 探测连接可能恰好占住端口，使节点绑定失败且难以察觉）。
func (n *Node) Serve(ln net.Listener) error {
	n.mu.Lock()
	if n.closing {
		n.mu.Unlock()
		_ = ln.Close()
		return nil
	}
	n.listener = ln
	n.mu.Unlock()
	log.Printf("[p2p] 节点已启动，监听 %s（nodeID=%s）", ln.Addr().String(), n.nodeID)
	n.acceptLoop(ln)
	return nil
}

// acceptLoop 是 Start/Serve 共用的连接接受循环（阻塞直至 listener 关闭且节点 closing）。
func (n *Node) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			n.mu.RLock()
			closing := n.closing
			n.mu.RUnlock()
			if closing {
				return
			}
			log.Printf("[p2p] 接受连接出错: %v", err)
			continue
		}
		go n.handleConn(conn, false)
	}
}

// ListenAddr 返回实际监听地址（Start 之后可用；Serve 注入时即注入监听器的地址）。
func (n *Node) ListenAddr() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.listener != nil {
		return n.listener.Addr().String()
	}
	return n.listenAddr
}

// Stop 关闭监听与全部连接。
func (n *Node) Stop() {
	n.mu.Lock()
	n.closing = true
	if n.listener != nil {
		_ = n.listener.Close()
	}
	peers := make([]*Peer, 0, len(n.peers))
	for _, p := range n.peers {
		peers = append(peers, p)
	}
	n.peers = make(map[string]*Peer)
	n.mu.Unlock()

	for _, p := range peers {
		p.signalClosed() // R1-A：终结各连接的出站 writer
		_ = p.Conn.Close()
	}
}

// ConnectToPeer 主动连接对等节点并发送握手。
func (n *Node) ConnectToPeer(addr string) error {
	n.mu.RLock()
	if _, exists := n.peers[addr]; exists {
		n.mu.RUnlock()
		return nil // 已连接
	}
	full := len(n.peers) >= maxPeers // R1-B：外拨预检（handleConn 侧还有第二道闸）
	n.mu.RUnlock()
	if full {
		return fmt.Errorf("对端总数已达上限 %d，拒绝外拨 %s", maxPeers, addr)
	}

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("连接对等节点 %s 失败: %w", addr, err)
	}
	go n.handleConn(conn, true)
	return nil
}

// PeerCount 返回已连接对等节点数量。
func (n *Node) PeerCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.peers)
}

// PeerAddrs 返回已连接对等节点的远端地址列表。
func (n *Node) PeerAddrs() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	out := make([]string, 0, len(n.peers))
	for addr := range n.peers {
		out = append(out, addr)
	}
	return out
}

// KnownPeers 返回节点发现过程中累积的已知地址（不含自身）。
func (n *Node) KnownPeers() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	out := make([]string, 0, len(n.known))
	for a := range n.known {
		out = append(out, a)
	}
	return out
}

// learnPeer 记录一个在网络上可达的对等地址（忽略空值与自身监听地址）。
func (n *Node) learnPeer(addr string) {
	if addr == "" || addr == n.listenAddr {
		return
	}
	n.mu.Lock()
	n.known[addr] = struct{}{}
	n.mu.Unlock()
}

// Broadcast 向全部对等节点发送消息。
func (n *Node) Broadcast(msg Message) { n.BroadcastExcept(msg, "") }

// BroadcastExcept 向除 except 之外的全部对等节点发送消息（避免中继回环）。
//
// R1-A 语义变更——**非阻塞入队**：消息经每对端有界出站队列（outboundQueueCap）
// 由该连接唯一的 writer goroutine 串行写出；队列满即对该对端丢弃本条
// （OUTQ_OVERFLOW 观测）。调用方（包括挖矿临界区内的 commitMinedBlock → broadcastBlock）
// 永不被慢对端的 TCP 写阻塞；per-peer FIFO、writeTimeout、连续失败断开语义保持不变。
// 注意：BROADCAST_EXIT.duration_us 自此只度量「序列化 + 入队」耗时，不再包含网络写出时间。
//
// I0/§5：本函数是观测包装——计时与事件在外层，实现体在 broadcastExcept。
func (n *Node) BroadcastExcept(msg Message, except string) {
	start := time.Now()
	obs.Emit("BROADCAST_ENTER", "msg_type", string(msg.Type), "except", except,
		"peer_count", n.PeerCount(), "height", n.currentHeight())
	n.broadcastExcept(msg, except)
	obs.Emit("BROADCAST_EXIT", "msg_type", string(msg.Type), "except", except,
		"duration_us", time.Since(start).Microseconds())
}

// broadcastExcept 是 BroadcastExcept 的原始实现体（R1-A：同步逐对端写 → 非阻塞入队）。
func (n *Node) broadcastExcept(msg Message, except string) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[p2p] 序列化消息失败: %v", err)
		return
	}
	if len(data)+1 > MaxMessageSize {
		log.Printf("[p2p] 消息过大（%d 字节），拒绝广播", len(data))
		return
	}
	data = append(data, '\n')
	m := outboundMsg{data: data, mtype: string(msg.Type)}

	n.mu.RLock()
	targets := make([]*Peer, 0, len(n.peers))
	for addr, p := range n.peers {
		if addr == except {
			continue
		}
		targets = append(targets, p)
	}
	n.mu.RUnlock()

	for _, p := range targets {
		// R1-A：非阻塞入队。队列满 = 该对端消费过慢（writer 正被 writeTimeout 卡住），
		// 丢弃本条，绝不阻塞调用方或拖累其他对端的投递。
		select {
		case p.outQ <- m:
		default:
			p.queueDrops.Add(1)
			obs.Emit("OUTQ_OVERFLOW", "peer", p.Addr, "msg_type", m.mtype,
				"queue_cap", outboundQueueCap)
			log.Printf("[p2p] %s 出站队列已满，丢弃 %s 消息（累计丢弃 %d）",
				p.Addr, m.mtype, p.queueDrops.Load())
		}
	}
}

// peerWriter R1-A：单连接出站写 goroutine（每连接恰一个，握手写出后启动）。
//   - FIFO：按入队顺序写出；与 SendTo / sendHandshake 的直接写经 p.mu 互斥，
//     保证同一连接上的写永不交错；
//   - writeTimeout 保留：单次写最多阻塞 writeTimeout；
//   - maxSendFailures 保留：连续写失败达阈值 → dropPeer；
//   - 终结：p.closed 关闭（连接清理 / dropPeer）即退出。
func (n *Node) peerWriter(p *Peer) {
	for {
		select {
		case <-p.closed:
			return
		case m := <-p.outQ:
			p.mu.Lock()
			_ = p.Conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			_, werr := p.Conn.Write(m.data)
			if werr != nil {
				p.sendFails++
				fails := p.sendFails
				p.mu.Unlock()
				log.Printf("[p2p] 向 %s 发送失败(%d/%d): %v", p.Addr, fails, maxSendFailures, werr)
				emitWriteFailure(p.Addr, m.mtype, werr)
				if fails >= maxSendFailures {
					n.dropPeer(p, "连续发送失败")
					return
				}
				continue
			}
			p.sendFails = 0
			p.lastActive = time.Now()
			p.mu.Unlock()
		}
	}
}

// peerDispatchWorker P0-2：单连接分发 worker（每连接恰一个，与 peerWriter 同时启动）。
//
// 读循环只做帧解析与非阻塞入队；业务 Handler（含区块 PoW 校验、磁盘 IO、
// 上层 SendTo）在本 worker 中串行执行——慢业务不再卡住读循环，恶意对端也
// 无法用消息流水线拖住连接。单 worker + FIFO 队列保证同连接消息处理顺序
// 与同步分发一致。
//
// 终结：p.closed 关闭（连接清理 / dropPeer）即退出；残留未消费消息随连接
// 丢弃（连接已死，重传由对端同步机制负责）。
func (n *Node) peerDispatchWorker(p *Peer) {
	for {
		select {
		case <-p.closed:
			return
		case job := <-p.dispatchQ:
			n.dispatch(job.peerAddr, job.msg)
		}
	}
}

// emitWriteFailure 观测专用：按错误类型区分 WRITE_TIMEOUT 与 SEND_ERROR。
// 在调用方锁外调用（本函数不获取任何业务锁）。
func emitWriteFailure(peer, msgType string, err error) {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		obs.Emit("WRITE_TIMEOUT", "peer", peer, "msg_type", msgType, "write_timeout_s", writeTimeout.Seconds())
		return
	}
	obs.Emit("SEND_ERROR", "peer", peer, "msg_type", msgType, "error", err.Error())
}

// SendTo 向指定远端地址发送消息（用于定向响应，如 GetBlocks → BlocksResp）。
//
// I0/§5：观测包装——计时与事件在外层，实现体在 sendTo，行为逐字节保持不变。
func (n *Node) SendTo(addr string, msg Message) error {
	start := time.Now()
	obs.Emit("SEND_ENTER", "peer", addr, "msg_type", string(msg.Type), "height", n.currentHeight())
	err := n.sendTo(addr, msg)
	if err != nil {
		// I0/§5：写失败细分（WRITE_TIMEOUT / SEND_ERROR），错误返回值不变。
		emitWriteFailure(addr, string(msg.Type), err)
	}
	obs.Emit("SEND_EXIT", "peer", addr, "msg_type", string(msg.Type),
		"duration_us", time.Since(start).Microseconds(), "error", errorString(err))
	return err
}

// errorString 观测专用：nil → ""（避免事件里出现 "<nil>" 噪声）。
func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// sendTo 是 SendTo 的原始实现体（I0 拆分，仅观测包装变更）。
//
// P0-3 修复：定向发送改走与广播同一套有界出站队列（非阻塞入队），不再持 p.mu
// 做最长 10s 的同步写——此前 500 区块的 blocks_resp 经 service.go:494 的 SendTo
// 会把该连接的读循环回包与 writer 串行卡住。
//
// 调用方语义变化：返回 nil 仅表示"已入队"（不再表示"对端已收到"）；写失败改由
// peerWriter 按 maxSendFailures 统一处理（记数 + 阈值断开），与广播路径一致。
// 上层 4 处 SendTo 调用方均只记日志不依赖同步送达语义（service.go:497/800/847/1268）。
func (n *Node) sendTo(addr string, msg Message) error {
	n.mu.RLock()
	p, ok := n.peers[addr]
	n.mu.RUnlock()
	if !ok {
		return fmt.Errorf("对等节点 %s 未连接", addr)
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if len(data)+1 > MaxMessageSize {
		return fmt.Errorf("消息过大（%d 字节）", len(data))
	}
	data = append(data, '\n')
	// P0-3：非阻塞入队（与 broadcastExcept 同语义）。队列满 = 对端消费过慢，
	// 丢弃并返回错误（调用方按既有逻辑记日志，对端靠同步重试补齐），绝不阻塞。
	// 注意：此处刻意不取 p.mu——peerWriter 在慢写时最长持有该锁 10s（writeTimeout），
	// 取锁会把"非阻塞"拖回阻塞；lastActive 由 writer 在实际写出时（:489）与读循环
	// 在收到消息时（:731）维护，语义不变。
	select {
	case p.outQ <- outboundMsg{data: data, mtype: string(msg.Type)}:
		return nil
	default:
		p.queueDrops.Add(1)
		obs.Emit("OUTQ_OVERFLOW", "peer", p.Addr, "msg_type", string(msg.Type),
			"queue_cap", outboundQueueCap)
		log.Printf("[p2p] %s 出站队列已满，定向 %s 消息被丢弃（累计丢弃 %d）",
			p.Addr, msg.Type, p.queueDrops.Load())
		return fmt.Errorf("对等节点 %s 出站队列已满", addr)
	}
}

// dropPeer 移除并关闭一条连接。
func (n *Node) dropPeer(p *Peer, reason string) {
	n.mu.Lock()
	if cur, ok := n.peers[p.Addr]; ok && cur == p {
		delete(n.peers, p.Addr)
	}
	n.mu.Unlock()
	_ = p.Conn.Close()
	p.signalClosed() // R1-A：终结出站 writer
	// I0/§5：连接拆除事件（D 受害链终点：连续发送失败即此处的 reason）。
	obs.Emit("DISCONNECT", "peer", p.Addr, "reason", reason)
	log.Printf("[p2p] 断开对等节点 %s（%s）", p.Addr, reason)
}

// dropPeerByAddr 按远端地址查找并关闭对应连接（O1 GENESIS IDENTITY GUARD 握手拒绝用）。
// 未找到（连接已消失）时静默返回；在 dropPeer 之前释放读锁，避免嵌套加锁。
func (n *Node) dropPeerByAddr(addr, reason string) {
	n.mu.RLock()
	p := n.peers[addr]
	n.mu.RUnlock()
	if p != nil {
		n.dropPeer(p, reason)
	}
}

// peerLimitRejectionLocked 在持有 n.mu 时判断是否应拒绝该连接，返回拒绝原因
// （空串 = 放行）。统计口径：maxPeers=全部已注册对端；maxInbound=其中入站者；
// handshakeQuota=已注册但尚未完成握手者（含入站与外拨）。
func (n *Node) peerLimitRejectionLocked(peer *Peer) string {
	total := len(n.peers)
	inboundN, pending := 0, 0
	for _, p := range n.peers {
		if p.inbound {
			inboundN++
		}
		if !p.handshaked.Load() {
			pending++
		}
	}
	if total >= maxPeers {
		return fmt.Sprintf("对端总数已达上限 %d", maxPeers)
	}
	if peer.inbound && inboundN >= maxInbound {
		return fmt.Sprintf("入站连接已达上限 %d", maxInbound)
	}
	if pending >= handshakeQuota {
		return fmt.Sprintf("未握手连接已达配额 %d", handshakeQuota)
	}
	return ""
}

// handleConn 处理一条连接的完整生命周期：握手 → 消息循环 → 清理。
func (n *Node) handleConn(conn net.Conn, outbound bool) {
	remote := conn.RemoteAddr().String()
	inbound := !outbound
	peer := newPeer(remote, conn, inbound)

	n.mu.Lock()
	if n.closing {
		n.mu.Unlock()
		_ = conn.Close()
		return
	}
	if old, dup := n.peers[remote]; dup && old != peer {
		n.mu.Unlock()
		_ = conn.Close() // 同一地址已有活跃连接
		return
	}
	// R1-B：连接资源限额（在同一临界区内「统计 + 注册」，避免竞态超额）。
	// 入站受 maxInbound / handshakeQuota / maxPeers 约束；外拨受 maxPeers 约束
	//（ConnectToPeer 预检之外的第二道闸，防并发外拨穿透）。
	if reason := n.peerLimitRejectionLocked(peer); reason != "" {
		n.mu.Unlock()
		obs.Emit("CONN_REJECTED", "peer", remote, "reason", reason,
			"peer_count", len(n.peers))
		log.Printf("[p2p] 拒绝连接 %s（%s）", remote, reason)
		_ = conn.Close()
		return
	}
	n.peers[remote] = peer
	n.mu.Unlock()

	defer func() {
		n.mu.Lock()
		if cur, ok := n.peers[remote]; ok && cur == peer {
			delete(n.peers, remote)
		}
		n.mu.Unlock()
		_ = conn.Close()
		peer.signalClosed() // R1-A：终结出站 writer
		log.Printf("[p2p] 与 %s 的连接已关闭", remote)
	}()

	// 双方都主动发握手：入站连接也立即回送，简化协议（幂等处理）。
	// 握手必须同步先写：出站 writer 尚未启动，保证「握手先于任何广播」的线上顺序
	//（修复旧实现中广播与握手竞争 p.mu 时广播可能先于握手上线、被对端当未握手消息丢弃的隐患）。
	n.sendHandshake(peer)

	// R1-A：此后本连接的全部出站写都由独立 writer 串行完成——
	// 广播调用方（含挖矿临界区）只做入队，永不被本连接的慢写阻塞。
	go n.peerWriter(peer)

	// P0-2：与 writer 同时启动分发 worker——读循环只做帧解析与入队，
	// 业务 Handler 在 worker 中串行执行，不再阻塞读循环。
	go n.peerDispatchWorker(peer)

	// I0/§5：读循环生命周期 —— ENTER 在循环前，EXIT 由 defer 在连接关闭时补记。
	obs.Emit("READ_LOOP_ENTER", "peer", remote, "height", n.currentHeight())
	defer func() {
		obs.Emit("READ_LOOP_EXIT", "peer", remote, "height", n.currentHeight())
	}()

	reader := bufio.NewReaderSize(conn, 64*1024)
	prevCycleEnd := time.Now() // READ_WAIT 语义：上一轮 dispatch 结束（或连接建立）到本轮 read 返回的间隔
	for {
		// R1-B：握手死线强制——未完成握手前，每次读都以 handshakeTimeout 为 deadline；
		// 完成握手后恢复常规 readTimeout。（原实现声明 handshakeTimeout 却从未执行。）
		readDeadline := readTimeout
		if !peer.handshaked.Load() {
			readDeadline = handshakeTimeout
		}
		_ = conn.SetReadDeadline(time.Now().Add(readDeadline))
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		if len(line) > MaxMessageSize {
			log.Printf("[p2p] 来自 %s 的消息超过大小上限，断开", remote)
			return
		}
		readGap := time.Since(prevCycleEnd)

		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			log.Printf("[p2p] 解析来自 %s 的消息失败: %v", remote, err)
			prevCycleEnd = time.Now()
			continue
		}
		peer.mu.Lock()
		peer.lastActive = time.Now()
		peer.mu.Unlock()
		if msg.Type == MsgHandshake {
			peer.handshaked.Store(true)
		}
		handshaked := peer.handshaked.Load()

		// 未握手前只接受握手消息，避免未识别连接直接注入区块/交易
		if !handshaked && msg.Type != MsgHandshake {
			log.Printf("[p2p] 来自 %s 的 %s 消息在握手前到达，忽略", remote, msg.Type)
			prevCycleEnd = time.Now()
			continue
		}
		// I0/§5：READ_WAIT —— 读循环两轮处理之间的间隔（含阻塞读与 dispatch 耗时之外的空窗）。
		obs.Emit("READ_WAIT", "peer", remote, "read_gap_us", readGap.Microseconds(),
			"msg_type", string(msg.Type))
		// P0-2：业务分发异步化。握手消息保持同步处理——它便宜（无 PoW/磁盘 IO），
		// 且后继消息的门控（handshaked 检查）与创世不一致时的立即断开都依赖同步语义；
		// 其余消息非阻塞入队，由 peerDispatchWorker 串行消费（FIFO，保序）。
		if msg.Type == MsgHandshake {
			n.dispatch(remote, msg)
		} else {
			select {
			case peer.dispatchQ <- dispatchJob{peerAddr: remote, msg: msg}:
			default:
				peer.dispatchDrops.Add(1)
				obs.Emit("DISPATCHQ_OVERFLOW", "peer", remote, "msg_type", string(msg.Type))
				log.Printf("[p2p] %s 分发队列已满，丢弃 %s 消息（累计丢弃 %d）",
					remote, msg.Type, peer.dispatchDrops.Load())
			}
		}
		prevCycleEnd = time.Now()
	}
}

// sendHandshake 发送本节点握手消息。
func (n *Node) sendHandshake(p *Peer) {
	work, tipHash := n.currentStatus()
	payload := HandshakePayload{
		NodeID:      n.nodeID,
		ChainHeight: n.currentHeight(),
		ListenAddr:  n.listenAddr,
		GenesisHash: n.genesisHash,
		KnownPeers:  n.KnownPeers(),
		ChainWork:   work,
		TipHash:     tipHash,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	msg := Message{Type: MsgHandshake, Payload: raw}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	data = append(data, '\n')
	p.mu.Lock()
	defer p.mu.Unlock()
	_ = p.Conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if _, err := p.Conn.Write(data); err != nil {
		log.Printf("[p2p] 向 %s 发送握手失败: %v", p.Addr, err)
	}
}

// SetHeightProvider 注册链高度提供者（启动时由上层注入），用于握手与同步。
// 允许在节点运行期间调用，因此与 currentHeight 之间用互斥保护。
func (n *Node) SetHeightProvider(f func() int) {
	n.mu.Lock()
	n.heightFn = f
	n.mu.Unlock()
}

// SetChainStatusProvider 注册「链尾工作量 + 链尾哈希」提供者（REORG-1H）。
//
// 这两个值让握手从「只比高度」升级为「比累积工作量 + 发现未知分支」：
//   - work：十进制字符串（math/big.Int.String()），空串表示未知；
//   - tipHash：十六进制字符串，空串表示未知。
//
// 提供者未注册或返回空串时，握手消息里对应字段为空，对端自动退化为按高度比较
// ——这是与旧版本节点互通的兼容路径。
func (n *Node) SetChainStatusProvider(f func() (work string, tipHash string)) {
	n.mu.Lock()
	n.statusFn = f
	n.mu.Unlock()
}

// currentHeight 返回注入的链高度；未注入时返回 0。
func (n *Node) currentHeight() int {
	n.mu.RLock()
	f := n.heightFn
	n.mu.RUnlock()
	if f == nil {
		return 0
	}
	return f()
}

// currentStatus 返回注入的链尾工作量与哈希；未注入时返回两个空串（= 未知）。
func (n *Node) currentStatus() (work string, tipHash string) {
	n.mu.RLock()
	f := n.statusFn
	n.mu.RUnlock()
	if f == nil {
		return "", ""
	}
	return f()
}

// dispatch 按消息类型分发到上层 Handler。
//
// I0/§5：观测包装——ENTER/EXIT + duration 在外层，原始分发体在 dispatchInner。
// P0-2 后：本函数运行在 peerDispatchWorker（握手消息除外，仍由读循环同步调用），
// duration 度量的是 worker 上的 Handler 耗时——读循环不再被它阻塞。
func (n *Node) dispatch(peerAddr string, msg Message) {
	start := time.Now()
	obs.Emit("DISPATCH_ENTER", "peer", peerAddr, "msg_type", string(msg.Type), "height", n.currentHeight())
	n.dispatchInner(peerAddr, msg)
	obs.Emit("DISPATCH_EXIT", "peer", peerAddr, "msg_type", string(msg.Type),
		"dispatch_duration_us", time.Since(start).Microseconds())
}

// dispatchInner 是 dispatch 的原始分发体（I0 拆分，仅观测包装变更）。
func (n *Node) dispatchInner(peerAddr string, msg Message) {
	switch msg.Type {
	case MsgHandshake:
		var payload HandshakePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[p2p] 解析握手失败: %v", err)
			return
		}
		// O1 GENESIS IDENTITY GUARD：创世块一致性校验（握手接受路径）。
		// 双方都声明 genesis 且不一致 ⇒ 判为异网对端：拒绝握手并关闭连接，
		// 且**不**记录邻居、**不**进入业务层（跳过 OnHandshake）——因此绝不触发
		// 同步(get_blocks) / 按哈希分支拉取(get_block_by_hash) / 中继。
		// 任一方未声明（空）时保持向后兼容（旧节点不填该字段），不据此拒绝。
		if n.genesisHash != "" && payload.GenesisHash != "" && payload.GenesisHash != n.genesisHash {
			log.Printf("[p2p] 拒绝握手 %s：创世块不一致（对端=%s 本地=%s）",
				peerAddr, payload.GenesisHash, n.genesisHash)
			n.dropPeerByAddr(peerAddr, "创世块不一致")
			return
		}
		// 记录已知节点（节点发现）：既学习对端声明的可达监听地址，也吸收其转告的邻居
		n.learnPeer(payload.ListenAddr)
		for _, a := range payload.KnownPeers {
			n.learnPeer(a)
		}
		n.handler.OnHandshake(peerAddr, payload)
	case MsgNewBlock:
		n.handler.OnNewBlock(peerAddr, msg.Payload)
	case MsgNewTx:
		n.handler.OnNewTx(peerAddr, msg.Payload)
	case MsgGetBlocks:
		var payload GetBlocksPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[p2p] 解析 GetBlocks 失败: %v", err)
			return
		}
		n.handler.OnGetBlocks(peerAddr, payload)
	case MsgBlocksResp:
		var payload BlocksRespPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[p2p] 解析 BlocksResp 失败: %v", err)
			return
		}
		n.handler.OnBlocksResp(peerAddr, payload)
	case MsgGetBlockByHash:
		var payload GetBlockByHashPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[p2p] 解析 GetBlockByHash 失败: %v", err)
			return
		}
		n.handler.OnGetBlockByHash(peerAddr, payload)
	case MsgBlockByHashResp:
		var payload BlockByHashRespPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[p2p] 解析 BlockByHashResp 失败: %v", err)
			return
		}
		n.handler.OnBlockByHashResp(peerAddr, payload)
	default:
		log.Printf("[p2p] 未知消息类型 %s（来自 %s），忽略", msg.Type, peerAddr)
	}
}

// TODO 后续可扩展（当前为学习项目范围外的生产级能力）：
//   1. 消息校验和与版本协商（protocol version handshake）；
//   2. 对等节点评分与封禁（恶意节点识别）；
//   3. 日食攻击防护（连接来源多样化、锚点节点）；
//   4. 心跳与断线重连退避策略（当前依赖读超时清理）。
