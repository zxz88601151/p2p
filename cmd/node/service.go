// nodeService 把区块链、交易池、P2P 网络与矿工钱包粘合在一起，并实现 p2p.Handler 接口。
//
// 职责边界：本文件只做「消息 → 状态机」的翻译，不实现共识规则本身——
// 区块校验、UTXO 迁移、交易签名验证全部委托给 blockchain / utxo / mempool 层。
// 这样网络层可以独立测试，共识层也可脱离网络单独验证（见各包的单元测试）。
//
// 线程模型：P2P 的分发 goroutine 会并发调用下列 On* 回调，因此对共享状态
// （交易池、同步计数、挖矿中断信号）的访问都由内部互斥保护；
// blockchain 自身有锁，可安全并发调用。
package main

import (
	"encoding/hex"
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/mempool"
	"p2pchain/internal/p2p"
	"p2pchain/internal/transaction"
	"p2pchain/internal/wallet"
)

// MaxSyncBatch 单次同步请求的区块数上限。
const MaxSyncBatch = 200

// MaxBlockTxs 单个区块最多打包的非 coinbase 交易数，
// 与区块体积上限共同约束候选区块规模，避免产出被共识拒绝的超大区块。
const MaxBlockTxs = 500

// nodeService 实现 p2p.Handler。
type nodeService struct {
	chain *blockchain.Blockchain
	pool  *mempool.Mempool
	net   *p2p.Node
	miner *wallet.Wallet

	mu sync.Mutex
	// pending 记录尚未收到响应的同步请求数，避免重复发起造成请求风暴。
	pending int
	// syncing 表示当前正在追赶（本地落后于对端），用于抑制重复触发。
	syncing bool
	// tipChanged 为容量 1 的信号通道：链尾变化时通知挖矿循环放弃当前候选区块。
	// 用缓冲通道 + 非阻塞发送实现「合并多次通知为一次唤醒」。
	tipChanged chan struct{}

	// mining 表示本节点是否正在挖矿（供 /status 查询）。原子读写即可，
	// 避免为一次状态展示引入锁竞争。
	//
	// 语义（PHASE MINING-REMEDIATION-1）：本字段仅表示「挖矿循环是否处于活动状态」
	// （含 STARTING/RUNNING/STALLED），**不代表**「正在对一个合法候选区块执行 PoW」。
	// 真实语义状态见 mineState；`/status` 的权威字段是 mining_state。
	mining atomic.Bool

	// ---- 挖矿运行时语义状态（PHASE MINING-REMEDIATION-1）----
	//
	// 背景：此前「worker goroutine 存在」被直接当作「运行中」（svc.mining），
	// 导致在「补贴耗尽且无手续费交易」时控制接口仍报告「运行中」，而实际上
	// 模板根本不存在合法候选（无意义 PoW 热循环）。以下字段把该状态显式化。
	//
	// mineState 是 miningState 枚举字符串（原子读写，供 /status 查询）。
	mineState atomic.Value
	// mineReason 是最近一次状态变化的可读原因（用于区分「政策终态」与「真实故障」）。
	mineReason atomic.Value
	// powAttempts 是自启动以来累计实际执行的 PoW 尝试次数（双 SHA-256 计数）。
	powAttempts atomic.Uint64
	// mineRetries 是模板重建/重试次数（链尾变化、陈旧竞争等合法重试亦计入）。
	mineRetries atomic.Int64
	// acceptedBlocks 是自启动以来成功上链的区块数。
	acceptedBlocks atomic.Int64
	// rejectedBlocks 是自启动以来上链被拒的次数（预校验通过后的意外拒绝）。
	rejectedBlocks atomic.Int64
	// lastAcceptedAt 是最近一次成功出块的 Unix 秒（0 表示尚未出块）。
	lastAcceptedAt atomic.Int64
	// startHeight 是挖矿循环启动时的链高（用于回答「链是否真的在推进」）。
	startHeight atomic.Int64

	// mineMu 串行化所有挖矿入口（持续挖矿循环与按需出块），
	// 保证任一时刻只有一个候选区块在被求解。
	mineMu sync.Mutex

	// miners 是并行挖矿的 worker 数（<=1 表示单线程，结果确定）。
	miners int
}

func newNodeService(chain *blockchain.Blockchain, pool *mempool.Mempool, miner *wallet.Wallet) *nodeService {
	s := &nodeService{
		chain:      chain,
		pool:       pool,
		miner:      miner,
		tipChanged: make(chan struct{}, 1),
	}
	// atomic.Value 必须先 Store 再 Load，否则 Load 返回 nil 接口导致类型断言 panic。
	s.mineState.Store(miningState(MiningStopped))
	s.mineReason.Store("init")
	return s
}

// notifyTipChanged 非阻塞地通知挖矿循环：链尾可能已变化，应重新组装候选区块。
func (s *nodeService) notifyTipChanged() {
	select {
	case s.tipChanged <- struct{}{}:
	default: // 已有未消费的通知，无需重复投递
	}
}

// drainTipChanged 丢弃尚未消费的链尾变化通知（开始新一轮挖矿前调用）。
func drainTipChanged(s *nodeService) {
	select {
	case <-s.tipChanged:
	default:
	}
}

// ---- p2p.Handler 实现 ----

// OnHandshake 对方握手：若对方链更高，则请求缺失区块追赶。
func (s *nodeService) OnHandshake(peerAddr string, payload p2p.HandshakePayload) {
	localHeight := s.chain.Height()
	log.Printf("[node] 握手完成: 对端=%s 对端高度=%d 本地高度=%d", peerAddr, payload.ChainHeight, localHeight)

	if payload.ChainHeight <= localHeight {
		return
	}
	s.requestSync(peerAddr, localHeight+1)
}

// OnNewBlock 收到区块广播：校验并追加，成功则清理交易池、继续中继。
func (s *nodeService) OnNewBlock(peerAddr string, raw json.RawMessage) {
	var payload p2p.BlockPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		log.Printf("[node] 解析区块广播失败（来自 %s）: %v", peerAddr, err)
		return
	}
	rawBytes, err := hex.DecodeString(payload.Encoded)
	if err != nil {
		log.Printf("[node] 区块编码非法（来自 %s）: %v", peerAddr, err)
		return
	}
	b, err := block.DecodeBlock(rawBytes)
	if err != nil {
		log.Printf("[node] 区块解码失败（来自 %s）: %v", peerAddr, err)
		return
	}

	tip, err := s.chain.Tip()
	if err != nil {
		log.Printf("[node] 读取链尾失败: %v", err)
		return
	}
	if b.Header.PrevBlockHash != tip.Header.Hash() {
		// 重复广播（其父块已不是链尾）或分叉区块。单链实现不处理 reorg，
		// 分叉场景已在 blockchain 包文档中列为后续工作项。
		log.Printf("[node] 忽略区块 %s（父块不是当前链尾，可能为重复广播或分叉）", b.Header.HashHex())
		return
	}

	if err := s.addBlockAndUpdatePool(b); err != nil {
		log.Printf("[node] 区块拒绝（来自 %s）: %v", peerAddr, err)
		return
	}
	// 继续中继给其它对等节点（except 来源方向，避免回环风暴）
	s.relayBlock(b, peerAddr)
}

// OnNewTx 收到交易广播：校验后入池，成功则继续中继。
func (s *nodeService) OnNewTx(peerAddr string, raw json.RawMessage) {
	var payload p2p.TxPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		log.Printf("[node] 解析交易广播失败（来自 %s）: %v", peerAddr, err)
		return
	}
	rawBytes, err := hex.DecodeString(payload.Encoded)
	if err != nil {
		log.Printf("[node] 交易编码非法（来自 %s）: %v", peerAddr, err)
		return
	}
	tx, err := transaction.DecodeTx(rawBytes)
	if err != nil {
		log.Printf("[node] 交易解码失败（来自 %s）: %v", peerAddr, err)
		return
	}

	if err := s.pool.Add(s.chain.UTXOSnapshot(), tx, s.chain.Height()+1); err != nil {
		log.Printf("[node] 交易未入池（来自 %s）: %v", peerAddr, err)
		return
	}
	log.Printf("[node] 交易已入池: %s", tx.HashHex())
	s.relayTx(tx, peerAddr)
}

// OnGetBlocks 响应区块同步请求。
func (s *nodeService) OnGetBlocks(peerAddr string, payload p2p.GetBlocksPayload) {
	count := payload.Count
	if count <= 0 || count > MaxSyncBatch {
		count = MaxSyncBatch
	}
	blocks, atTip := s.chain.BlocksFrom(payload.FromHeight, count)

	encoded := make([]string, 0, len(blocks))
	for _, b := range blocks {
		encoded = append(encoded, hex.EncodeToString(b.Encode()))
	}
	respPayload, err := json.Marshal(p2p.BlocksRespPayload{EncodedBlocks: encoded, Done: atTip})
	if err != nil {
		log.Printf("[node] 构造同步响应失败: %v", err)
		return
	}
	if err := s.net.SendTo(peerAddr, p2p.Message{Type: p2p.MsgBlocksResp, Payload: respPayload}); err != nil {
		log.Printf("[node] 发送同步响应给 %s 失败: %v", peerAddr, err)
		return
	}
	log.Printf("[node] 已响应同步请求: 对端=%s 起始高度=%d 返回=%d 个区块 到链尾=%v",
		peerAddr, payload.FromHeight, len(blocks), atTip)
}

// OnBlocksResp 处理同步响应：按序校验并追加区块，未到链尾则继续请求下一批。
func (s *nodeService) OnBlocksResp(peerAddr string, payload p2p.BlocksRespPayload) {
	applied := 0
	for _, enc := range payload.EncodedBlocks {
		rawBytes, err := hex.DecodeString(enc)
		if err != nil {
			log.Printf("[node] 同步区块编码非法: %v", err)
			return
		}
		b, err := block.DecodeBlock(rawBytes)
		if err != nil {
			log.Printf("[node] 同步区块解码失败: %v", err)
			return
		}
		if err := s.addBlockAndUpdatePool(b); err != nil {
			log.Printf("[node] 同步区块拒绝: %v", err)
			return
		}
		applied++
	}
	log.Printf("[node] 同步进度: 应用 %d 个区块，本地高度=%d，对方已到链尾=%v",
		applied, s.chain.Height(), payload.Done)

	if payload.Done || applied == 0 {
		s.mu.Lock()
		s.syncing = false
		s.pending = 0
		s.mu.Unlock()
		return
	}
	s.requestSync(peerAddr, s.chain.Height()+1)
}

// ---- 内部工具 ----

// addBlockAndUpdatePool 追加区块并同步交易池：移除已上链交易、剔除失效交易。
func (s *nodeService) addBlockAndUpdatePool(b *block.Block) error {
	height := s.chain.Height() + 1
	if err := s.chain.AddBlock(b); err != nil {
		return err
	}
	s.pool.RemoveIncluded(b, s.chain.UTXOSnapshot(), height)
	log.Printf("[node] 新区块已上链: 高度=%d 哈希=%s 交易数=%d",
		s.chain.Height(), b.Header.HashHex(), len(b.Transactions))
	// 链尾变化 → 通知挖矿循环放弃当前候选区块
	s.notifyTipChanged()
	return nil
}

// requestSync 向指定对端请求从 from 高度开始的区块。
func (s *nodeService) requestSync(peerAddr string, from int) {
	s.mu.Lock()
	if s.pending > 0 {
		s.mu.Unlock()
		return // 已有请求在途，等待响应即可
	}
	s.pending++
	s.syncing = true
	s.mu.Unlock()

	payload, err := json.Marshal(p2p.GetBlocksPayload{FromHeight: from, Count: MaxSyncBatch})
	if err != nil {
		log.Printf("[node] 构造同步请求失败: %v", err)
		return
	}
	if err := s.net.SendTo(peerAddr, p2p.Message{Type: p2p.MsgGetBlocks, Payload: payload}); err != nil {
		log.Printf("[node] 发送同步请求给 %s 失败: %v", peerAddr, err)
		s.mu.Lock()
		s.pending = 0
		s.syncing = false
		s.mu.Unlock()
		return
	}
	log.Printf("[node] 已向 %s 请求区块，起始高度=%d", peerAddr, from)
}

// relayBlock 把区块继续中继给除来源之外的对等节点。
func (s *nodeService) relayBlock(b *block.Block, except string) {
	payload, err := json.Marshal(p2p.BlockPayload{Encoded: hex.EncodeToString(b.Encode())})
	if err != nil {
		log.Printf("[node] 区块序列化失败，取消中继: %v", err)
		return
	}
	s.net.BroadcastExcept(p2p.Message{Type: p2p.MsgNewBlock, Payload: payload}, except)
}

// relayTx 把交易继续中继给除来源之外的对等节点。
func (s *nodeService) relayTx(tx *transaction.Transaction, except string) {
	payload, err := json.Marshal(p2p.TxPayload{Encoded: hex.EncodeToString(tx.Encode())})
	if err != nil {
		log.Printf("[node] 交易序列化失败，取消中继: %v", err)
		return
	}
	s.net.BroadcastExcept(p2p.Message{Type: p2p.MsgNewTx, Payload: payload}, except)
}

// broadcastBlock 向全部对等节点广播区块（本地挖出时调用）。
func (s *nodeService) broadcastBlock(b *block.Block) { s.relayBlock(b, "") }

// broadcastTx 向全部对等节点广播交易（本地发起转账时调用）。
func (s *nodeService) broadcastTx(tx *transaction.Transaction) { s.relayTx(tx, "") }
