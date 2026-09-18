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
//   - 广播支持 except 参数，避免中继回环导致的广播风暴。
package p2p

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
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
	// handshakeTimeout 建立连接后必须在该时间内收到握手消息。
	handshakeTimeout = 10 * time.Second
	// readTimeout / writeTimeout 单次读写超时。
	readTimeout  = 60 * time.Second
	writeTimeout = 10 * time.Second
	// maxSendFailures 连续发送失败阈值，达到即断开该连接。
	maxSendFailures = 3
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

// Peer 一条已建立的连接及其状态。
type Peer struct {
	Addr        string
	Conn        net.Conn
	mu          sync.Mutex
	sendFails   int
	handshaked  bool
	lastActive  time.Time
	pendingResp int
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
	n.mu.RUnlock()

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
// I0/§5：本函数是观测包装——计时与事件在外层，实现体在 broadcastExcept，
// 行为逐字节保持不变（OBSERVE ≠ CHANGE）。
func (n *Node) BroadcastExcept(msg Message, except string) {
	start := time.Now()
	obs.Emit("BROADCAST_ENTER", "msg_type", string(msg.Type), "except", except,
		"peer_count", n.PeerCount(), "height", n.currentHeight())
	n.broadcastExcept(msg, except)
	obs.Emit("BROADCAST_EXIT", "msg_type", string(msg.Type), "except", except,
		"duration_us", time.Since(start).Microseconds())
}

// broadcastExcept 是 BroadcastExcept 的原始实现体（I0 拆分，仅观测包装变更）。
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
		p.mu.Lock()
		_ = p.Conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		_, werr := p.Conn.Write(data)
		if werr != nil {
			p.sendFails++
			fails := p.sendFails
			p.mu.Unlock()
			log.Printf("[p2p] 向 %s 发送失败(%d/%d): %v", p.Addr, fails, maxSendFailures, werr)
			// I0/§5：写失败细分——超时（WRITE_TIMEOUT）与其他（SEND_ERROR）。
			emitWriteFailure(p.Addr, string(msg.Type), werr)
			if fails >= maxSendFailures {
				n.dropPeer(p, "连续发送失败")
			}
			continue
		}
		p.sendFails = 0
		p.lastActive = time.Now()
		p.mu.Unlock()
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
	p.mu.Lock()
	defer p.mu.Unlock()
	_ = p.Conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if _, err := p.Conn.Write(data); err != nil {
		p.sendFails++
		if p.sendFails >= maxSendFailures {
			go n.dropPeer(p, "连续发送失败")
		}
		return err
	}
	p.sendFails = 0
	p.lastActive = time.Now()
	return nil
}

// dropPeer 移除并关闭一条连接。
func (n *Node) dropPeer(p *Peer, reason string) {
	n.mu.Lock()
	if cur, ok := n.peers[p.Addr]; ok && cur == p {
		delete(n.peers, p.Addr)
	}
	n.mu.Unlock()
	_ = p.Conn.Close()
	// I0/§5：连接拆除事件（D 受害链终点：连续发送失败即此处的 reason）。
	obs.Emit("DISCONNECT", "peer", p.Addr, "reason", reason)
	log.Printf("[p2p] 断开对等节点 %s（%s）", p.Addr, reason)
}

// handleConn 处理一条连接的完整生命周期：握手 → 消息循环 → 清理。
func (n *Node) handleConn(conn net.Conn, outbound bool) {
	remote := conn.RemoteAddr().String()
	peer := &Peer{Addr: remote, Conn: conn, lastActive: time.Now()}

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
	n.peers[remote] = peer
	n.mu.Unlock()

	defer func() {
		n.mu.Lock()
		if cur, ok := n.peers[remote]; ok && cur == peer {
			delete(n.peers, remote)
		}
		n.mu.Unlock()
		_ = conn.Close()
		log.Printf("[p2p] 与 %s 的连接已关闭", remote)
	}()

	// 双方都主动发握手：入站连接也立即回送，简化协议（幂等处理）
	n.sendHandshake(peer)

	// I0/§5：读循环生命周期 —— ENTER 在循环前，EXIT 由 defer 在连接关闭时补记。
	obs.Emit("READ_LOOP_ENTER", "peer", remote, "height", n.currentHeight())
	defer func() {
		obs.Emit("READ_LOOP_EXIT", "peer", remote, "height", n.currentHeight())
	}()

	reader := bufio.NewReaderSize(conn, 64*1024)
	prevCycleEnd := time.Now() // READ_WAIT 语义：上一轮 dispatch 结束（或连接建立）到本轮 read 返回的间隔
	for {
		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
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
		if msg.Type == MsgHandshake {
			peer.handshaked = true
		}
		handshaked := peer.handshaked
		peer.mu.Unlock()

		// 未握手前只接受握手消息，避免未识别连接直接注入区块/交易
		if !handshaked && msg.Type != MsgHandshake {
			log.Printf("[p2p] 来自 %s 的 %s 消息在握手前到达，忽略", remote, msg.Type)
			prevCycleEnd = time.Now()
			continue
		}
		// I0/§5：READ_WAIT —— 读循环两轮处理之间的间隔（含阻塞读与 dispatch 耗时之外的空窗）。
		obs.Emit("READ_WAIT", "peer", remote, "read_gap_us", readGap.Microseconds(),
			"msg_type", string(msg.Type))
		n.dispatch(remote, msg)
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
// duration 是证明「reader loop 被 dispatch 阻塞」的直接证据（读循环同步调用本函数）。
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
