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
	"errors"
	"log"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/mempool"
	"p2pchain/internal/obs"
	"p2pchain/internal/p2p"
	"p2pchain/internal/transaction"
	"p2pchain/internal/wallet"
)

// MaxSyncBatch 单次同步请求的区块数上限。
const MaxSyncBatch = 200

// MaxBlockTxs 单个区块最多打包的非 coinbase 交易数，
// 与区块体积上限共同约束候选区块规模，避免产出被共识拒绝的超大区块。
const MaxBlockTxs = 500

// ---- REORG-1H：P2P 分支投递（by-hash 拉取桥接）的常量与边界 ----
//
// 目标（本阶段范围内）：让「合法但非 canonical 链尾后继」的区块能被网络送达
// 并进入 blocktree/reorg 流水线。孤儿块缺父时，按哈希把缺口补齐。
//
// 明确**不做**（属 REORG-1G/B5 的完整 orphan pool，本阶段只做桥接）：
//   - 不持久化孤儿（进程重启即丢弃）；
//   - 不做无限深度追溯（有轮次上限）；
//   - 不做孤儿块的定时重播/评分/封禁。
const (
	// MaxBranchAncestors 单次 by-hash 请求期望回溯的祖先数（服务端仍会按
	// p2p.MaxAncestorsPerResp 再裁剪一次，双端都是有界的）。
	MaxBranchAncestors = 64
	// maxInflightBranch 在途 by-hash 请求上限：防止一次大规模分叉瞬间打出
	// 成千上万条请求（请求风暴）。
	maxInflightBranch = 64
	// maxWaitingBlocks 等待父块的孤儿区块上限（纯内存）。
	maxWaitingBlocks = 256
	// maxBranchRounds 同一条孤儿链允许的「继续回溯」轮次上限。
	//
	// 为什么必须有限：若对端持续返回一段我们挂不上的链（例如彼此创世不同），
	// 没有轮次上限就会形成「请求 → 响应 → 再请求」的无限循环。
	maxBranchRounds = 8
	// branchReqTTL 在途请求去重窗口：窗口内同一哈希绝不重复请求（幂等 + 防抖）。
	branchReqTTL = 30 * time.Second
)

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

	// ---- REORG-1H：分支拉取运行时状态（全部由 mu 保护，纯内存，绝不持久化）----
	//
	// 不持久化的理由：孤儿是「到达顺序」问题，重启后本节点会重新走握手 +
	// 批量同步 + 分支拉取，状态可完全重建；把它落盘只会引入又一份需要在
	// 崩溃恢复中证明正确性的状态（完整 orphan pool 属 REORG-1G/B5）。
	inflight map[[32]byte]time.Time      // 已发出、尚未回来的 by-hash 请求
	waiting  map[[32]byte][]*block.Block // 缺父哈希 → 等待该父块的孤儿区块
	rounds   map[[32]byte]int            // 按哈希请求 → 已回溯轮次（有界追溯）
	// syncResume 记录「批量同步因缺父转入分支拉取」时的对端地址；
	// 分支补齐后由 resumeSync 恢复下一批 GetBlocks，避免 syncing 永久卡死。
	syncResume string

	// obsReqIDs 是纯观测辅助映射（I0）：hash → 最近一次在途请求的 request_id。
	// 只被观测代码读写，绝不参与任何业务分支判断；删除条目时一并清理。
	obsReqIDs map[[32]byte]uint64

	// ---- 观测计数器（诊断与测试证据，不参与共识）----
	branchReqSent  atomic.Int64 // 已发出的 by-hash 请求数
	branchRespRecv atomic.Int64 // 收到的 by-hash 响应数
	branchApplied  atomic.Int64 // 经分支拉取成功上链的区块数
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

	// minerLife 是持续挖矿生命周期管理器（PHASE MINING-LIFECYCLE-1）。
	// 持有与 node stopCh 完全分离的 minerStop 通道；构造时即创建，
	// 之后只读（atomic.Pointer 保证 StartMining/StopMining 的无锁安全读取）。
	minerLife atomic.Pointer[minerLifecycle]

	// miners 是并行挖矿的 worker 数（<=1 表示单线程，结果确定）。
	miners int
}

func newNodeService(chain *blockchain.Blockchain, pool *mempool.Mempool, miner *wallet.Wallet) *nodeService {
	s := &nodeService{
		chain:      chain,
		pool:       pool,
		miner:      miner,
		tipChanged: make(chan struct{}, 1),
		inflight:   make(map[[32]byte]time.Time),
		waiting:    make(map[[32]byte][]*block.Block),
		rounds:     make(map[[32]byte]int),
		obsReqIDs:  make(map[[32]byte]uint64),
	}
	// atomic.Value 必须先 Store 再 Load，否则 Load 返回 nil 接口导致类型断言 panic。
	s.mineState.Store(miningState(MiningStopped))
	s.mineReason.Store("init")
	// PHASE MINING-LIFECYCLE-1：生命周期管理器随服务创建（惰性使用；
	// 未 START 前完全不活动）。构造即挂载 ⇒ StartMining/StopMining 无 nil 分支。
	s.minerLife.Store(newMinerLifecycle(s))
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

// OnHandshake 对方握手：按「工作量优先、高度兜底」决定是否追赶，
// 并在发现对端处于未知分支时按哈希拉取其链尾（REORG-1H M4）。
//
// 修复前：只要 `PrevBlockHash != 当前链尾` 的区块就被就地丢弃（service.go:154），
// 真实分叉块永远进不了 blocktree —— 跨节点 reorg 在物理上不可能发生。
func (s *nodeService) OnHandshake(peerAddr string, payload p2p.HandshakePayload) {
	localHeight := s.chain.Height()
	localWork := s.chain.BestTipWork()
	log.Printf("[node] 握手完成: 对端=%s 对端高度=%d 本地高度=%d 对端工作量=%q 对端链尾=%s",
		peerAddr, payload.ChainHeight, localHeight, payload.ChainWork, shortHash(payload.TipHash))

	// (1) 追赶：工作量更大（或工作量未知时高度更高）才值得拉批次。
	if shouldSyncFrom(payload.ChainWork, payload.ChainHeight, localWork, localHeight) {
		s.requestSync(peerAddr, localHeight+1)
	}

	// (2) 分支发现：对端链尾我没见过，且它的工作量不低于我 → 它可能在我
	//     不知道的分支上（或领先我）。按哈希把它的链尾拉过来，交由共识层
	//     做 fork-choice；缺父则由 by-hash 分支补齐。
	if tip, ok := parseHash32(payload.TipHash); ok && !s.chain.HasBlockHash(tip) &&
		workAtLeast(payload.ChainWork, payload.ChainHeight, localWork, localHeight) {
		s.requestBranch(peerAddr, tip)
	}
}

// shouldSyncFrom 判断是否应向对端发起批量追赶。
//
// 判据优先级：
//  1. 双方工作量都已知 → **只比工作量**（work-aware）。
//     这是 M4 的实质：难度浮动后「更高」不再等于「更重」，只看高度会被
//     一条低难度长链牵走。
//  2. 任一方工作量未知（旧版本节点不填 chain_work）→ 退化为比高度，
//     行为与 REORG-1H 之前完全一致（向后兼容）。
func shouldSyncFrom(peerWork string, peerHeight int, localWork *big.Int, localHeight int) bool {
	peer, ok := parseWork(peerWork)
	if !ok || localWork == nil {
		return peerHeight > localHeight
	}
	return peer.Cmp(localWork) > 0
}

// workAtLeast 判断对端工作量是否**不低于**本节点（用于「要不要拉对端分支」）。
// 工作量未知时退化为「对端高度不低于本地高度」。
func workAtLeast(peerWork string, peerHeight int, localWork *big.Int, localHeight int) bool {
	peer, ok := parseWork(peerWork)
	if !ok || localWork == nil {
		return peerHeight >= localHeight
	}
	return peer.Cmp(localWork) >= 0
}

// parseWork 解析十进制工作量字符串；空串/非法值返回 false（= 未知）。
func parseWork(s string) (*big.Int, bool) {
	if s == "" {
		return nil, false
	}
	w, ok := new(big.Int).SetString(s, 10)
	if !ok || w.Sign() < 0 {
		return nil, false
	}
	return w, true
}

// parseHash32 解析 32 字节十六进制哈希；空串/长度不符返回 false。
func parseHash32(s string) ([32]byte, bool) {
	var h [32]byte
	if s == "" {
		return h, false
	}
	raw, err := hex.DecodeString(s)
	if err != nil || len(raw) != 32 {
		return h, false
	}
	copy(h[:], raw)
	return h, true
}

// shortHash 把十六进制哈希截断为前 8 字符，仅供日志（避免刷屏）。
func shortHash(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

// hashHex32 观测专用：把 32 字节哈希编码为完整十六进制（事件字段用）。
func hashHex32(h [32]byte) string { return hex.EncodeToString(h[:]) }

// waitingChildren 观测专用：返回当前等待该父哈希的孤儿块数（只读，不影响行为）。
func (s *nodeService) waitingChildren(parent [32]byte) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.waiting[parent])
}

// OnNewBlock 收到区块广播：分叉块不再被就地丢弃（REORG-1H M1）。
//
// 三种结局：
//   - 已在 canonical 链上 → 静默幂等（重复广播）；
//   - 父已知（延长链 / 分叉 / 触发 reorg）→ 交给共识层，成功后中继；
//   - 父未知（orphan）→ 登记等待并按哈希把缺口补齐。
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

	applied, orphan, err := s.ingestBlock(peerAddr, b)
	switch {
	case err != nil:
		log.Printf("[node] 区块拒绝（来自 %s）: %v", peerAddr, err)
		return
	case orphan:
		log.Printf("[node] 区块 %s 父块 %s 未知，已发起 by-hash 分支拉取",
			b.Header.HashHex(), shortHash(hex.EncodeToString(b.Header.PrevBlockHash[:])))
		return
	case applied:
		// I0/§7：父块经 OnNewBlock 到达 —— 此路径**没有** takeWaiting 级联（B-1 证据点）。
		obs.Emit("WAITING_PARENT_RESCAN_TRIGGER", "parent", hashHex32(b.Header.Hash()),
			"waiting_children", s.waitingChildren(b.Header.Hash()), "via", "on_new_block")
		s.relayBlock(b, peerAddr)
	}
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
		obs.Emit("SYNC_RESPONSE", "peer", peerAddr, "from_height", payload.FromHeight,
			"block_count", len(blocks), "at_tip", atTip, "result", "send_failed")
		return
	}
	obs.Emit("SYNC_RESPONSE", "peer", peerAddr, "from_height", payload.FromHeight,
		"block_count", len(blocks), "at_tip", atTip, "result", "sent")
	log.Printf("[node] 已响应同步请求: 对端=%s 起始高度=%d 返回=%d 个区块 到链尾=%v",
		peerAddr, payload.FromHeight, len(blocks), atTip)
}

// OnBlocksResp 处理同步响应：按序校验并追加区块，未到链尾则继续请求下一批。
//
// REORG-1H：批量同步里同样可能出现「缺父」的区块（对端在我们不知道的分支上）。
// 修复前这里遇到任何 AddBlock 失败就 `return`，既不复位 syncing/pending
// （标志永久卡死，节点再也不会发起同步），也不会去补齐缺口。
// 现在改为：非法块 → 终止本批并复位；缺父块 → 登记为孤儿并触发 by-hash 拉取，
// 等分支补齐后由 resumeSync 继续拉下一批。
func (s *nodeService) OnBlocksResp(peerAddr string, payload p2p.BlocksRespPayload) {
	// I0/§6：批生命周期 —— 收到批（含 cursor_before）。
	obs.Emit("SYNC_BATCH_RECEIVED", "peer", peerAddr, "batch_size", len(payload.EncodedBlocks),
		"done", payload.Done, "cursor_before", s.chain.Height())
	applied, deferred := 0, 0
	for _, enc := range payload.EncodedBlocks {
		rawBytes, err := hex.DecodeString(enc)
		if err != nil {
			log.Printf("[node] 同步区块编码非法: %v", err)
			obs.Emit("SYNC_BATCH_DROPPED", "peer", peerAddr, "reason", "bad_encoding",
				"applied_so_far", applied, "cursor_after", s.chain.Height())
			s.endSync("")
			return
		}
		b, err := block.DecodeBlock(rawBytes)
		if err != nil {
			log.Printf("[node] 同步区块解码失败: %v", err)
			obs.Emit("SYNC_BATCH_DROPPED", "peer", peerAddr, "reason", "bad_decode",
				"applied_so_far", applied, "cursor_after", s.chain.Height())
			s.endSync("")
			return
		}
		ok, orphan, err := s.ingestBlock(peerAddr, b)
		if err != nil {
			log.Printf("[node] 同步区块拒绝: %v", err)
			obs.Emit("SYNC_BATCH_REJECTED", "peer", peerAddr, "block", b.Header.HashHex(),
				"applied_so_far", applied, "cursor_after", s.chain.Height())
			s.endSync("")
			return
		}
		switch {
		case orphan:
			deferred++
		case ok:
			applied++
			// I0/§7：父块经 OnBlocksResp 到达 —— 此路径**没有** takeWaiting 级联（B-1 证据点）。
			obs.Emit("WAITING_PARENT_RESCAN_TRIGGER", "parent", hashHex32(b.Header.Hash()),
				"waiting_children", s.waitingChildren(b.Header.Hash()), "via", "on_blocks_resp")
		}
	}
	// I0/§6：批应用汇总（含 cursor_after 与 deferred 计数）。
	obs.Emit("SYNC_BATCH_APPLIED", "peer", peerAddr, "applied", applied, "deferred", deferred,
		"done", payload.Done, "cursor_after", s.chain.Height())
	log.Printf("[node] 同步进度: 应用 %d 个区块，缺父待补 %d 个，本地高度=%d，对方已到链尾=%v",
		applied, deferred, s.chain.Height(), payload.Done)

	s.mu.Lock()
	s.pending = 0
	resume := ""
	switch {
	case deferred > 0:
		// 转入分支拉取：批量请求让位，分支补齐后由 resumeSync 续拉。
		s.syncing = true
		s.syncResume = peerAddr
	case payload.Done || applied == 0:
		s.syncing = false
		s.syncResume = ""
	default:
		s.syncing = true
		resume = peerAddr
	}
	s.mu.Unlock()

	if resume != "" {
		s.requestSync(resume, s.chain.Height()+1)
	}
}

// endSync 复位批量同步状态（正常结束或异常终止），并清空待恢复目标。
func (s *nodeService) endSync(resume string) {
	s.mu.Lock()
	s.syncing = false
	s.pending = 0
	s.syncResume = resume
	s.mu.Unlock()
}

// resumeSync 在分支补齐后恢复被打断的批量同步。
// 没有待恢复目标（syncResume 为空）时是空操作，绝不主动发起新同步。
func (s *nodeService) resumeSync() {
	s.mu.Lock()
	target := s.syncResume
	s.syncing = false
	s.pending = 0
	s.syncResume = ""
	s.mu.Unlock()
	if target == "" {
		return
	}
	s.requestSync(target, s.chain.Height()+1)
}

// ---- REORG-1H：分叉投递 / 分支拉取 ----

// ingestBlock 把一个来自网络的区块送入共识层，并给出可操作的分类结果。
//
// 返回：
//   - applied=true ：共识层接受（延长了链、作为分叉块入树、或触发了 reorg）。
//     注意「入树但未 reorg」也算 applied —— 它已是合法候选，占住了分支。
//   - orphan=true  ：父区块未知，已登记等待并触发 by-hash 分支拉取。
//   - err != nil   ：被共识拒绝（非法区块），调用方只应记录日志，不得中继。
func (s *nodeService) ingestBlock(peerAddr string, b *block.Block) (applied bool, orphan bool, err error) {
	hash := b.Header.Hash()
	// 已在 canonical 链上：重复投递，静默幂等（既不报错也不重复应用）。
	if s.chain.IsCanonicalHash(hash) {
		return false, false, nil
	}
	if err := s.addBlockAndUpdatePool(b); err != nil {
		if errors.Is(err, blockchain.ErrOrphanParent) {
			s.deferOrphan(peerAddr, b)
			return false, true, nil
		}
		return false, false, err
	}
	s.clearInflight(hash)
	return true, false, nil
}

// deferOrphan 登记一个缺父的孤儿区块，并向来源对端请求它缺的父块。
func (s *nodeService) deferOrphan(peerAddr string, b *block.Block) {
	parent := b.Header.PrevBlockHash
	// I0/§3：ORPHAN_SEEN —— 孤儿高度在父未知时不可知，记 -1（UNKNOWN，禁止猜测）。
	obs.Emit("ORPHAN_SEEN", "orphan", b.Header.HashHex(), "parent", hashHex32(parent),
		"orphan_height", -1, "peer", peerAddr)

	s.mu.Lock()
	existing := len(s.waiting[parent])
	if existing == 0 && len(s.waiting) >= maxWaitingBlocks {
		s.mu.Unlock()
		obs.Emit("RECOVERY_DROPPED", "orphan", b.Header.HashHex(), "parent", hashHex32(parent),
			"reason", "waiting_limit", "waiting_total", len(s.waiting), "peer", peerAddr)
		log.Printf("[node] 等待父块的孤儿区块已达上限 %d，丢弃 %s", maxWaitingBlocks, b.Header.HashHex())
		return
	}
	s.waiting[parent] = append(s.waiting[parent], b)
	s.mu.Unlock()

	// I0/§3+§7：登记成功 → 具备恢复资格；同时记录 waiting-add 观测点。
	obs.Emit("RECOVERY_ELIGIBLE", "orphan", b.Header.HashHex(), "parent", hashHex32(parent),
		"waiting_children", existing+1, "peer", peerAddr)
	obs.Emit("WAITING_PARENT_ADD", "parent", hashHex32(parent), "orphan", b.Header.HashHex(),
		"waiting_children", existing+1, "via", "defer_orphan")
	s.requestBranch(peerAddr, parent)
}

// requestBranch 按哈希向指定对端请求一个区块及其祖先（分支补齐）。
//
// 防风暴三重约束：
//  1. 在途请求数上限 maxInflightBranch；
//  2. 同一哈希在 branchReqTTL 窗口内**绝不重复请求**（去重，杜绝请求循环）；
//  3. 单次回溯深度上限 MaxBranchAncestors（服务端再按 MaxAncestorsPerResp 裁剪）。
func (s *nodeService) requestBranch(peerAddr string, hash [32]byte) {
	s.mu.Lock()
	if len(s.inflight) >= maxInflightBranch {
		n := len(s.inflight)
		s.mu.Unlock()
		// I0/§3+§4：撞在途上限被静默抑制（原实现无日志无事件——本事件是 173→64 的直接观测点）。
		obs.Emit("RECOVERY_SUPPRESSED_LIMIT", "parent", hashHex32(hash), "request_id", uint64(0),
			"inflight", n, "max_inflight", maxInflightBranch, "peer", peerAddr)
		obs.Inc("recovery_suppressed_limit")
		return
	}
	if t, ok := s.inflight[hash]; ok && time.Since(t) < branchReqTTL {
		s.mu.Unlock()
		// I0/§3+§4：TTL 窗口内去重抑制（幂等路径的显式观测）。
		obs.Emit("RECOVERY_SUPPRESSED_INFLIGHT", "parent", hashHex32(hash), "request_id", uint64(0),
			"age_s", time.Since(t).Seconds(), "peer", peerAddr)
		obs.Inc("recovery_suppressed_inflight")
		return // 已在途：幂等，不再发一次
	}
	if t, ok := s.inflight[hash]; ok {
		// I0/§4：存在但已过 TTL —— 旧条目被覆盖重发（RECOVERY_TIMEOUT 语义观测点）。
		obs.Emit("RECOVERY_TIMEOUT", "parent", hashHex32(hash), "request_id", uint64(0),
			"age_s", time.Since(t).Seconds(), "peer", peerAddr)
	}
	reqID := obs.NextRequestID()
	s.inflight[hash] = time.Now()
	s.obsReqIDs[hash] = reqID
	n := len(s.inflight)
	s.mu.Unlock()

	// I0/§4：inflight 生命周期 —— 创建（含 current/max）。
	obs.Emit("INFLIGHT_CREATED", "request_id", reqID, "parent", hashHex32(hash),
		"peer", peerAddr, "inflight", n, "max_inflight", maxInflightBranch)

	payload, err := json.Marshal(p2p.GetBlockByHashPayload{
		Hash:         hex.EncodeToString(hash[:]),
		MaxAncestors: MaxBranchAncestors,
	})
	if err != nil {
		log.Printf("[node] 构造 by-hash 请求失败: %v", err)
		obs.Emit("INFLIGHT_CANCEL", "request_id", reqID, "parent", hashHex32(hash),
			"peer", peerAddr, "reason", "marshal_failed")
		s.clearInflight(hash)
		return
	}
	if err := s.net.SendTo(peerAddr, p2p.Message{Type: p2p.MsgGetBlockByHash, Payload: payload}); err != nil {
		log.Printf("[node] 发送 by-hash 请求给 %s 失败: %v", peerAddr, err)
		obs.Emit("INFLIGHT_CANCEL", "request_id", reqID, "parent", hashHex32(hash),
			"peer", peerAddr, "reason", "send_failed")
		s.clearInflight(hash)
		return
	}
	s.branchReqSent.Add(1)
	// I0/§3+§4：请求真正发出（RECOVERY_REQUESTED / INFLIGHT_SENT）。
	obs.Emit("INFLIGHT_SENT", "request_id", reqID, "parent", hashHex32(hash),
		"peer", peerAddr, "inflight", n, "max_inflight", maxInflightBranch)
	obs.Emit("RECOVERY_REQUESTED", "parent", hashHex32(hash), "request_id", reqID,
		"peer", peerAddr, "max_ancestors", MaxBranchAncestors)
	obs.Inc("recovery_requested")
	log.Printf("[node] 已向 %s 请求分支区块 %s（最多回溯 %d 个祖先）",
		peerAddr, shortHash(hex.EncodeToString(hash[:])), MaxBranchAncestors)
}

// OnGetBlockByHash 响应 by-hash 分支请求：返回该区块及其祖先（M2 服务端）。
//
// 只提供**本节点已知**的区块（canonical + 已落盘的 detached 分叉块）。
// 不认识的哈希返回 Found=false，不猜测、不伪造（禁止 Fake Implementation）。
func (s *nodeService) OnGetBlockByHash(peerAddr string, payload p2p.GetBlockByHashPayload) {
	hash, ok := parseHash32(payload.Hash)
	if !ok {
		log.Printf("[node] by-hash 请求哈希非法（来自 %s）: %q", peerAddr, payload.Hash)
		return
	}
	n := payload.MaxAncestors
	if n <= 0 || n > p2p.MaxAncestorsPerResp {
		n = p2p.MaxAncestorsPerResp
	}
	chain := s.chain.BlockByHashWithAncestors(hash, n)

	encoded := make([]string, 0, len(chain))
	for _, b := range chain {
		encoded = append(encoded, hex.EncodeToString(b.Encode()))
	}
	respPayload, err := json.Marshal(p2p.BlockByHashRespPayload{
		Hash:   hex.EncodeToString(hash[:]),
		Blocks: encoded,
		Found:  len(encoded) > 0,
	})
	if err != nil {
		log.Printf("[node] 构造 by-hash 响应失败: %v", err)
		return
	}
	if err := s.net.SendTo(peerAddr, p2p.Message{Type: p2p.MsgBlockByHashResp, Payload: respPayload}); err != nil {
		log.Printf("[node] 发送 by-hash 响应给 %s 失败: %v", peerAddr, err)
		return
	}
	log.Printf("[node] 已响应 by-hash: 对端=%s 请求=%s 返回=%d 个区块",
		peerAddr, shortHash(hex.EncodeToString(hash[:])), len(encoded))
}

// OnBlockByHashResp 处理 by-hash 响应：挂载点定位 → 父先于子应用 → 级联孤儿（M3）。
//
// 响应约定：Blocks[0] = 被请求块，其后依次是父、祖父（由新到旧）。
// 因此先找到第一个「父已知」的下标 i，再从 i 递减到 0 应用，即可满足
// 共识层「父必须先于子存在」的要求——**不依赖 map 迭代顺序**（BT-1 教训）。
func (s *nodeService) OnBlockByHashResp(peerAddr string, payload p2p.BlockByHashRespPayload) {
	s.branchRespRecv.Add(1)
	// I0/§3：RECOVERY_RESPONSE —— 响应到达（Found=false 亦记录，不混入失败笼统口径）。
	obs.Emit("RECOVERY_RESPONSE", "req_hash", payload.Hash, "block_count", len(payload.Blocks),
		"found", payload.Found, "peer", peerAddr)

	decoded := make([]*block.Block, 0, len(payload.Blocks))
	for _, enc := range payload.Blocks {
		rawBytes, err := hex.DecodeString(enc)
		if err != nil {
			log.Printf("[node] by-hash 响应编码非法: %v", err)
			return
		}
		b, err := block.DecodeBlock(rawBytes)
		if err != nil {
			log.Printf("[node] by-hash 响应解码失败: %v", err)
			return
		}
		decoded = append(decoded, b)
	}

	reqHash, _ := parseHash32(payload.Hash)
	if len(decoded) == 0 {
		// 对端没有这块：清理在途，放弃（完整 orphan 重播策略属 B5）。
		obs.Emit("RECOVERY_DROPPED", "req_hash", payload.Hash, "reason", "not_found", "peer", peerAddr)
		s.clearInflight(reqHash)
		log.Printf("[node] by-hash: 对端 %s 没有区块 %s，放弃该分支", peerAddr, shortHash(payload.Hash))
		return
	}

	start := -1
	for i, b := range decoded {
		s.clearInflight(b.Header.Hash())
		if s.chain.KnowsParent(b.Header.PrevBlockHash) {
			start = i
			break
		}
	}
	// I0/§7：挂载点扫描结果（找到 start 下标或整段挂不上）。
	obs.Emit("WAITING_PARENT_SCAN", "req_hash", payload.Hash, "blocks_scanned", len(decoded),
		"start_index", start, "via", "on_block_by_hash_resp")

	if start < 0 {
		// 整段都挂不上：缺口更深。继续回溯，但有轮次上限——
		// 否则对端持续返回接不上的链时会形成请求/响应无限循环。
		s.mu.Lock()
		round := s.rounds[reqHash]
		s.mu.Unlock()
		if round >= maxBranchRounds {
			obs.Emit("RECOVERY_DROPPED", "req_hash", payload.Hash, "reason", "rounds_limit",
				"rounds", round, "peer", peerAddr)
			s.clearInflight(reqHash)
			log.Printf("[node] by-hash 回溯已达 %d 轮上限，放弃分支 %s", maxBranchRounds, shortHash(payload.Hash))
			return
		}
		deeper := decoded[len(decoded)-1].Header.PrevBlockHash
		s.clearInflight(reqHash)
		s.mu.Lock()
		s.rounds[deeper] = round + 1
		s.mu.Unlock()
		s.requestBranch(peerAddr, deeper)
		return
	}

	s.mu.Lock()
	delete(s.rounds, reqHash)
	s.mu.Unlock()

	for i := start; i >= 0; i-- {
		s.applyResolved(peerAddr, decoded[i])
	}
	// 分支补齐后恢复此前被打断的批量同步。
	s.resumeSync()
}

// applyResolved 应用一个「父已就位」的分支区块，并级联处理等待它的孤儿。
// 级联用显式队列 + 有界展开，不递归（避免深度分支导致栈膨胀）。
func (s *nodeService) applyResolved(peerAddr string, b *block.Block) {
	queue := []*block.Block{b}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		applied, orphan, err := s.ingestBlock(peerAddr, cur)
		switch {
		case err != nil:
			log.Printf("[node] 分支区块被拒绝: %s: %v", cur.Header.HashHex(), err)
			continue
		case orphan:
			// 又缺父：已登记并继续回溯拉取，等下一次响应再级联。
			continue
		case applied:
			s.branchApplied.Add(1)
			// I0/§3+§7：级联路径应用成功 —— 这是唯一会 takeWaiting 重扫的父块来源。
			obs.Emit("RECOVERY_APPLIED", "block", cur.Header.HashHex(),
				"height", s.chain.Height(), "via", "on_block_by_hash_resp_cascade")
			obs.Emit("WAITING_PARENT_APPLY", "parent", hashHex32(cur.Header.Hash()),
				"via", "on_block_by_hash_resp_cascade")
			s.relayBlock(cur, peerAddr)
		}
		kids := s.takeWaiting(cur.Header.Hash())
		obs.Emit("WAITING_PARENT_MATCH", "parent", hashHex32(cur.Header.Hash()),
			"children", len(kids), "via", "take_waiting_cascade")
		queue = append(queue, kids...)
	}
}

// takeWaiting 取出并清空等待该父哈希的孤儿区块。
func (s *nodeService) takeWaiting(parentHash [32]byte) []*block.Block {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.waiting[parentHash]
	delete(s.waiting, parentHash)
	return out
}

// clearInflight 清除一个哈希的在途请求标记（收到块或放弃后调用）。
func (s *nodeService) clearInflight(hash [32]byte) {
	s.mu.Lock()
	_, existed := s.inflight[hash]
	delete(s.inflight, hash)
	reqID := s.obsReqIDs[hash]
	delete(s.obsReqIDs, hash)
	n := len(s.inflight)
	s.mu.Unlock()
	if existed {
		// I0/§4：inflight 释放（释放原因由相邻的 RECOVERY_* 事件区分）。
		obs.Emit("INFLIGHT_RELEASED", "request_id", reqID, "parent", hashHex32(hash),
			"inflight", n, "max_inflight", maxInflightBranch)
	}
}

// ---- 内部工具 ----

// addBlockAndUpdatePool 追加区块并同步交易池：移除已上链交易、剔除失效交易。
// REORG-1F：若触发 reorg，在 canonical TIP 提交后复活断开区块中的交易。
func (s *nodeService) addBlockAndUpdatePool(b *block.Block) error {
	height := s.chain.Height() + 1
	result, err := s.chain.AddBlockWithResult(b)
	if err != nil {
		return err
	}

	// REORG-1F：若发生 reorg，复活断开区块中的非 coinbase 交易。
	if result != nil {
		s.resurrectMempool(result, height)
	}

	s.pool.RemoveIncluded(b, s.chain.UTXOSnapshot(), height)
	log.Printf("[node] 新区块已上链: 高度=%d 哈希=%s 交易数=%d",
		s.chain.Height(), b.Header.HashHex(), len(b.Transactions))
	// 链尾变化 → 通知挖矿循环放弃当前候选区块
	s.notifyTipChanged()
	return nil
}

// resurrectMempool 根据 ReorgResult 将断开区块中的交易重新加入内存池。
func (s *nodeService) resurrectMempool(result *blockchain.ReorgResult, height int) {
	// 构造新 canonical 链的交易去重集
	newChainTxs := make(map[[32]byte]struct{})
	for _, b := range result.ConnectBlocks {
		for _, tx := range b.Transactions {
			newChainTxs[tx.Hash()] = struct{}{}
		}
	}

	accepted, rejected := s.pool.ReaddDisconnected(
		result.DisconnectBlocks,
		s.chain.UTXOSnapshot(),
		height,
		newChainTxs,
	)
	if accepted > 0 || rejected > 0 {
		log.Printf("[node] reorg resurrection: 复活=%d 拒绝=%d 断开块=%d 新块=%d",
			accepted, rejected, len(result.DisconnectBlocks), len(result.ConnectBlocks))
	}
}

// requestSync 向指定对端请求从 from 高度开始的区块。
func (s *nodeService) requestSync(peerAddr string, from int) {
	s.mu.Lock()
	if s.pending > 0 {
		s.mu.Unlock()
		// I0/§6：请求被在途批抑制（抑制也是 SYNC 生命周期的合法状态）。
		obs.Emit("SYNC_REQUEST", "peer", peerAddr, "from_height", from,
			"suppressed", true, "reason", "pending_in_flight")
		return // 已有请求在途，等待响应即可
	}
	s.pending++
	s.syncing = true
	s.mu.Unlock()

	// I0/§6：SYNC_REQUEST 真正发出。
	obs.Emit("SYNC_REQUEST", "peer", peerAddr, "from_height", from, "suppressed", false,
		"batch", MaxSyncBatch)
	payload, err := json.Marshal(p2p.GetBlocksPayload{FromHeight: from, Count: MaxSyncBatch})
	if err != nil {
		log.Printf("[node] 构造同步请求失败: %v", err)
		return
	}
	if err := s.net.SendTo(peerAddr, p2p.Message{Type: p2p.MsgGetBlocks, Payload: payload}); err != nil {
		log.Printf("[node] 发送同步请求给 %s 失败: %v", peerAddr, err)
		obs.Emit("SYNC_REQUEST", "peer", peerAddr, "from_height", from,
			"suppressed", false, "result", "send_failed")
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
