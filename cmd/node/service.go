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
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/mempool"
	"p2pchain/internal/obs"
	"p2pchain/internal/p2p"
	"p2pchain/internal/pow"
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
	//
	// 注意：这是**不同父哈希**的个数上限，不是总孤儿个数上限——见
	// maxWaitingChildrenPerParent。
	maxWaitingBlocks = 256
	// maxWaitingChildrenPerParent 同一个父哈希下最多保留的孤儿个数（Q）。
	//
	// R-1（孤儿资源边界修复 / F-1）。性质：
	//   - 这是 **LOCAL ORPHAN RETENTION POLICY**，不是共识参数、不是区块有效性规则；
	//     超限只表示「本节点不再保留另一份」，该区块仍然有效，后续可重新传播/重新提交。
	//   - 它把此前**无界**的「每键子块数」变为有限，配合上面的 256 键上限即得
	//     total waiting entries <= 256 × 64 = 16,384（C5 实测此前 20/20、200/200 全保留）。
	//   - 这是**计数**上界，不是字节上界（块大小 352 B…1 MiB，保留对象是 Go 堆对象图）。
	//   - 取值 64 锚定本仓既有的三个 64（`MaxBranchAncestors`、`maxInflightBranch`、
	//     p2p.MaxAncestorsPerResp），不引入新的魔数量级；相对测试中最大的合法
	//     每父子块数（1）有 64× 余量，不影响合法孤儿。
	maxWaitingChildrenPerParent = 64
	// maxBranchRounds 同一条孤儿链允许的「继续回溯」轮次上限。
	//
	// 为什么必须有限：若对端持续返回一段我们挂不上的链（例如彼此创世不同），
	// 没有轮次上限就会形成「请求 → 响应 → 再请求」的无限循环。
	maxBranchRounds = 8
	// branchReqTTL 在途请求去重窗口：窗口内同一哈希绝不重复请求（幂等 + 防抖）。
	branchReqTTL = 30 * time.Second

	// ---- PHASE E-IMPLEMENTATION-A：统一级联的到达面标签（纯观测用）----
	//
	// 这些字符串只出现在 obs 事件的 via 字段里，不参与任何业务分支判断。
	// cascadeViaByHash 的值与修复前逐字相同，以保证 by-hash 路径的观测输出不变。
	cascadeViaByHash      = "on_block_by_hash_resp_cascade"
	cascadeViaBroadcast   = "resolve_arrival_on_new_block"
	cascadeViaBatchSync   = "resolve_arrival_on_blocks_resp"
	cascadeViaLocalMining = "resolve_arrival_local_mining"
)

// ---- PHASE P2P-SYNC-LIVENESS-MINIMUM-SAFE-FIX-1：批量同步在途注册表 ----
//
// 背景（设计审计 PHASE-P2P-CONNECTION-RECOVERY-DESIGN-AUDIT-1）：
// 修复前批量同步用一个**全局计数器** `pending` 兼做三件事——去重、在途账期、
// 进度游标。三者共用一个整数导致：一个坏/僵死的 peer 只要占住这个槽，
// 其余所有 peer 的同步请求都会被静默抑制，且该槽**既无 TTL 也无断连清理**，
// 极端情况下永久停留在 1（CASE 7）。
//
// 本阶段只做 MINIMUM SAFE FIX，复用既有 by-hash `inflight` 的四个性质
// （registry / TTL / 并发上限 / 超时重发），不另起一套平行机制：
//   - registry：按**区间起点高度**建全局登记表（去重仍是全局的，避免请求风暴）；
//   - per-request state：条目自带归属 peer、尝试次数、截止时间、重试时刻；
//   - TTL：条目超时即失效，可被重试或被彻底释放；
//   - bounded retry：最大尝试次数 + 指数退避 + 终态，绝不紧循环；
//   - peer failover：重试优先改投**另一个**仍连接的 peer；
//   - disconnect cleanup：对端已不在 PeerAddrs 中的条目立即判定失效；
//   - global concurrency cap：在途区间数上限。
//
// 明确不做（保持 DEFER）：readTimeout / heartbeat / ping-pong / TCP keepalive /
// 协议消息类型 / 握手能力声明 / NodeID / known_peers / maxInbound /
// BroadcastExcept / connection layer 重构。
const (
	// maxInflightSync 全局在途「区间起点」条目数上限。
	//
	// 与 maxInflightBranch=64 不同量级是**故意的**：每个批量条目最多换回
	// MaxSyncBatch=200 个区块（by-hash 条目只换回 <=65 个），
	// 因此本上限对应的在途区块上界为 4 × 200 = 800，与分支路径的
	// 64 × 65 = 4160 同数量级、但更保守。
	maxInflightSync = 4
	// syncReqTTL 单次批量同步请求的等待窗口：超时即判定本次尝试失败。
	//
	// 取值与 branchReqTTL（30s）同量级，不引入新的时间尺度；
	// 实测跨主机 200 块批量响应耗时 ~0.2s，30s 有 >100× 余量。
	defaultSyncReqTTL = 30 * time.Second
	// maxSyncAttempts 同一个区间起点允许的**总**尝试次数（含首次），
	// 达到即进入终态并释放条目 —— retry 必须有 terminal state。
	maxSyncAttempts = 3
	// defaultSyncRetryBase 退避基数：第 n 次失败后等待 base × 2^(n-1)。
	// 1st→2s、2nd→4s；第 3 次失败即终态，不再等待。
	defaultSyncRetryBase = 2 * time.Second
	// defaultSyncSweepInterval 调度器巡检周期（唯一执行重试发送的 goroutine）。
	defaultSyncSweepInterval = 2 * time.Second
)

// syncRequest 一次批量同步请求（区间起点）的完整生命周期状态。
//
// 全部字段由 nodeService.mu 保护；纯内存，绝不持久化（与 by-hash inflight 一致）。
type syncRequest struct {
	from    int       // 区间起点高度（= 登记表键，全局去重维度）
	peer    string    // 本次尝试的归属对端
	attempt int       // 已尝试次数（含当前这次），与 maxSyncAttempts 比较
	created time.Time // 本次尝试发出的时刻
	// deadline 本次尝试的失效时刻（created + TTL）。过期即判定本次尝试失败。
	deadline time.Time
	// failed 本次尝试已被判定失败（等待调度器按 backoff 重试）。
	// true 的条目仍占着去重槽 —— 这是「不重复请求同一区间」的保证，
	// 但它**有寿命**：TTL/退避/终态三者任一到达即被释放。
	failed bool
	// nextRetry 允许下一次重试的最早时刻（backoff 闸门，杜绝紧循环）。
	nextRetry time.Time
}

// nodeService 实现 p2p.Handler。
type nodeService struct {
	chain *blockchain.Blockchain
	pool  *mempool.Mempool
	net   *p2p.Node
	miner *wallet.Wallet

	mu sync.Mutex
	// syncInflight 批量同步在途登记表：键 = 区间起点高度（全局去重维度）。
	//
	// 取代了修复前的全局计数器 `pending`：去重语义保留（同一区间绝不并发重复请求），
	// 但每个条目现在有归属 peer、尝试次数、TTL、退避闸门与终态，
	// 因此「一个坏 peer 占住唯一全局槽」的 chokepoint 不复存在。
	syncInflight map[int]*syncRequest
	// syncing 表示当前正在追赶（本地落后于对端），用于抑制重复触发。
	syncing bool
	// syncSchedStarted 调度器 goroutine 是否已启动。
	//
	// 未启动时（例如只做单元测试的最小装配）在途条目一旦失败即**立即释放**，
	// 行为与修复前 `pending=0` 完全一致 —— 保证「不启动调度器 ⇒ 行为不变」。
	syncSchedStarted bool
	// syncStopCh / syncStopOnce / syncStartOnce / syncWG：调度器 goroutine 生命周期。
	// 调度器是**唯一**执行重试发送的地方（见 runSyncSweep）。
	syncStopCh   chan struct{}
	syncStopOnce sync.Once
	syncStartOne sync.Once
	syncWG       sync.WaitGroup
	// syncTTL / syncRetryBase / syncSweepInterval：可调时序参数。
	// 生产一律使用 default* 常量；仅测试可覆盖以压缩时序（不影响任何业务分支）。
	syncTTL           time.Duration
	syncRetryBase     time.Duration
	syncSweepInterval time.Duration

	// ---- REORG-1H：分支拉取运行时状态（全部由 mu 保护，纯内存，绝不持久化）----
	//
	// 不持久化的理由：孤儿是「到达顺序」问题，重启后本节点会重新走握手 +
	// 批量同步 + 分支拉取，状态可完全重建；把它落盘只会引入又一份需要在
	// 崩溃恢复中证明正确性的状态（完整 orphan pool 属 REORG-1G/B5）。
	inflight map[[32]byte]time.Time      // 已发出、尚未回来的 by-hash 请求
	waiting  map[[32]byte][]*block.Block // 缺父哈希 → 等待该父块的孤儿区块
	// parkedHashes 已入队孤儿的哈希索引（R-1 / F-3）：孤儿身份 = Header.Hash()。
	// 与 waiting 同一把 mu 保护、同为纯内存；用于「同一区块只保留一份」，
	// 并在 takeWaiting 释放时同步删除，使同一区块之后仍可再次入队。
	parkedHashes map[[32]byte]struct{}
	rounds       map[[32]byte]int // 按哈希请求 → 已回溯轮次（有界追溯）
	// orphanCP 孤儿等待检查点（ORPHAN-DURABILITY-IMPLEMENTATION-1，纯持久化投影）。
	// 仅被 deferOrphan（入队）/ takeWaiting（删除）两个挂钩访问；不参与任何共识判断。
	// nil 表示未启用（单元测试/最小装配），此时持久化挂钩为 no-op。
	orphanCP *orphanCheckpoint
	// restorePending 是启动恢复的「待恢复父键集」（内存态，§4-B1）。
	// 由 prepareOrphanRestore 从检查点加载而来；仅存父哈希，不存块指针、不重建 waiting。
	// 由后续阶段（§4-B2，OnHandshake）消费以触发 requestBranch；本阶段只填充，不消费。
	restorePending map[[32]byte]struct{}
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
		chain:        chain,
		pool:         pool,
		miner:        miner,
		tipChanged:   make(chan struct{}, 1),
		inflight:     make(map[[32]byte]time.Time),
		waiting:      make(map[[32]byte][]*block.Block),
		parkedHashes: make(map[[32]byte]struct{}),
		rounds:       make(map[[32]byte]int),
		obsReqIDs:    make(map[[32]byte]uint64),
		// MSF：批量同步在途登记表与其生命周期。
		syncInflight:      make(map[int]*syncRequest),
		syncStopCh:        make(chan struct{}),
		syncTTL:           defaultSyncReqTTL,
		syncRetryBase:     defaultSyncRetryBase,
		syncSweepInterval: defaultSyncSweepInterval,
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

	// (3) §4-B2 启动恢复：消费 restorePending（待恢复父键集）。
	//     复用既有 requestBranch 重新拉取缺失分支；幂等、可跨多次握手推进；
	//     已知父即时 drain，未知父保留待下一握手重试。
	s.consumeRestorePending(peerAddr)
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
		// P0-5：畸形消息记分（重复发送畸形块的对端会被封禁）。
		s.net.Penalize(peerAddr, 10, "区块广播解析失败")
		return
	}
	rawBytes, err := hex.DecodeString(payload.Encoded)
	if err != nil {
		log.Printf("[node] 区块编码非法（来自 %s）: %v", peerAddr, err)
		s.net.Penalize(peerAddr, 10, "区块编码非法")
		return
	}
	b, err := block.DecodeBlock(rawBytes)
	if err != nil {
		log.Printf("[node] 区块解码失败（来自 %s）: %v", peerAddr, err)
		s.net.Penalize(peerAddr, 10, "区块解码失败")
		return
	}

	applied, orphan, err := s.ingestBlock(peerAddr, b)
	switch {
	case err != nil:
		log.Printf("[node] 区块拒绝（来自 %s）: %v", peerAddr, err)
		// P0-5：共识层拒绝的非法块是明确的不良行为：记分并立即断开，
		// 防止其零成本无限重发垃圾块占据连接位。
		s.net.Penalize(peerAddr, 50, "发送共识非法区块")
		s.net.Disconnect(peerAddr, "发送共识非法区块")
		return
	case orphan:
		log.Printf("[node] 区块 %s 父块 %s 未知，已发起 by-hash 分支拉取",
			b.Header.HashHex(), shortHash(hex.EncodeToString(b.Header.PrevBlockHash[:])))
		return
	case applied:
		// I0/§7：父块经 OnNewBlock 到达。
		// PHASE E-IMPLEMENTATION-A（B-1）：修复前此路径**没有** takeWaiting 级联，
		// 现在统一经 resolveKnownBlock 释放等待该父块的孤儿。
		obs.Emit("WAITING_PARENT_RESCAN_TRIGGER", "parent", hashHex32(b.Header.Hash()),
			"waiting_children", s.waitingChildren(b.Header.Hash()), "via", "on_new_block")
		s.relayBlock(b, peerAddr)
		s.resolveKnownBlock(peerAddr, cascadeViaBroadcast, b)
	}
}

// OnNewTx 收到交易广播：校验后入池，成功则继续中继。
func (s *nodeService) OnNewTx(peerAddr string, raw json.RawMessage) {
	var payload p2p.TxPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		log.Printf("[node] 解析交易广播失败（来自 %s）: %v", peerAddr, err)
		s.net.Penalize(peerAddr, 10, "交易广播解析失败")
		return
	}
	rawBytes, err := hex.DecodeString(payload.Encoded)
	if err != nil {
		log.Printf("[node] 交易编码非法（来自 %s）: %v", peerAddr, err)
		s.net.Penalize(peerAddr, 10, "交易编码非法")
		return
	}
	tx, err := transaction.DecodeTx(rawBytes)
	if err != nil {
		log.Printf("[node] 交易解码失败（来自 %s）: %v", peerAddr, err)
		s.net.Penalize(peerAddr, 10, "交易解码失败")
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
	// P0-5：同步请求洪水检测（超限自动记分，本次请求直接忽略）。
	if s.net.NoteSyncRequest(peerAddr) {
		log.Printf("[node] 来自 %s 的 get_blocks 请求过于频繁，已忽略", peerAddr)
		return
	}
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
			// I0/§7：父块经 OnBlocksResp 到达。
			// PHASE E-IMPLEMENTATION-A（B-1）：修复前此路径**没有** takeWaiting 级联，
			// 现在统一经 resolveKnownBlock 释放等待该父块的孤儿。
			obs.Emit("WAITING_PARENT_RESCAN_TRIGGER", "parent", hashHex32(b.Header.Hash()),
				"waiting_children", s.waitingChildren(b.Header.Hash()), "via", "on_blocks_resp")
			s.resolveKnownBlock(peerAddr, cascadeViaBatchSync, b)
		}
	}
	// I0/§6：批应用汇总（含 cursor_after 与 deferred 计数）。
	obs.Emit("SYNC_BATCH_APPLIED", "peer", peerAddr, "applied", applied, "deferred", deferred,
		"done", payload.Done, "cursor_after", s.chain.Height())
	log.Printf("[node] 同步进度: 应用 %d 个区块，缺父待补 %d 个，本地高度=%d，对方已到链尾=%v",
		applied, deferred, s.chain.Height(), payload.Done)

	s.mu.Lock()
	s.clearSyncInflightLocked()
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
	s.clearSyncInflightLocked()
	s.syncResume = resume
	s.mu.Unlock()
}

// resumeSync 在分支补齐后恢复被打断的批量同步。
// 没有待恢复目标（syncResume 为空）时是空操作，绝不主动发起新同步。
func (s *nodeService) resumeSync() {
	s.mu.Lock()
	target := s.syncResume
	s.syncing = false
	s.clearSyncInflightLocked()
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

// preParkRejectReason 返回孤儿「入队前」校验的失败原因；空串表示通过。
//
// R-1（F-4）。只做**不依赖父块**的廉价检查，顺序刻意由最便宜到最贵，避免为超大载荷
// 付出昂贵的默克尔/PoW 成本：
//
//	size → bits 共识域 → 默克尔 → PoW
//
// 依赖父块或本链的校验——时间戳 vs tip/MTP、由本链推导的期望 bits、UTXO/交易校验、
// 以及完整的 validateForkBlock——**绝不**出现在这里，它们仍留在各自既有位置；
// 本函数的目标只是「客观无效的候选不占用孤儿保留资源」，不是「入队前做全量共识校验」。
func preParkRejectReason(b *block.Block) string {
	if b.Size() > blockchain.MaxBlockSize {
		return "size"
	}
	if !pow.IsBitsInConsensusDomain(b.Header.Bits) {
		return "bits"
	}
	if block.ComputeMerkleRoot(b.Transactions) != b.Header.MerkleRoot {
		return "merkle"
	}
	if !pow.Validate(&b.Header) {
		return "pow"
	}
	return ""
}

// deferOrphan 登记一个缺父的孤儿区块，并向来源对端请求它缺的父块。
func (s *nodeService) deferOrphan(peerAddr string, b *block.Block) {
	parent := b.Header.PrevBlockHash
	hash := b.Header.Hash()
	hashHex := b.Header.HashHex()
	// I0/§3：ORPHAN_SEEN —— 孤儿高度在父未知时不可知，记 -1（UNKNOWN，禁止猜测）。
	obs.Emit("ORPHAN_SEEN", "orphan", hashHex, "parent", hashHex32(parent),
		"orphan_height", -1, "peer", peerAddr)

	// ---- F-3（快速路径）：同一区块哈希只保留一份。
	//
	// 授权设计（§5.1 / 决策审计 1.1r-4）要求 dedup 排在昂贵校验**之前**：
	// 已入队区块的重复到达不应重复支付默克尔/PoW 成本。这里先做一次无副作用的
	// 快速查重；真正的权威查重与插入在同一临界区内完成（见下方 F-3 二次复查），
	// 两个检查点之间没有状态被写入 ⇒ 不存在「查重后、插入前」被并发抢先的双插窗口。
	s.mu.Lock()
	_, dup := s.parkedHashes[hash]
	s.mu.Unlock()
	if dup {
		obs.Emit("ORPHAN_DUPLICATE_IGNORED", "orphan", hashHex, "parent", hashHex32(parent),
			"peer", peerAddr)
		s.emitWaitingState("duplicate_ignored")
		return
	}

	// ---- F-4：入队前的父无关校验（拒绝客观无效的候选，避免占用保留资源）----
	if reason := preParkRejectReason(b); reason != "" {
		obs.Emit("ORPHAN_REJECTED_PREPARK", "orphan", hashHex, "parent", hashHex32(parent),
			"reason", reason, "peer", peerAddr)
		log.Printf("[node] 孤儿候选在入队前被拒绝（父无关校验失败: %s）: %s", reason, hashHex)
		s.emitWaitingState("prepark_reject")
		return
	}

	s.mu.Lock()
	// ---- F-3（权威复查，与插入同临界区）：快速路径与插入之间该哈希可能已被并发入队 ----
	if _, dup := s.parkedHashes[hash]; dup {
		s.mu.Unlock()
		obs.Emit("ORPHAN_DUPLICATE_IGNORED", "orphan", hashHex, "parent", hashHex32(parent),
			"peer", peerAddr)
		s.emitWaitingState("duplicate_ignored")
		return
	}
	existing := len(s.waiting[parent])
	// ---- F-1：每个父哈希的子块配额（LOCAL ORPHAN RETENTION POLICY，Q=64）----
	if existing >= maxWaitingChildrenPerParent {
		total := len(s.waiting)
		s.mu.Unlock()
		obs.Emit("RECOVERY_DROPPED", "orphan", hashHex, "parent", hashHex32(parent),
			"reason", "per_parent_quota", "waiting_total", total,
			"waiting_children", existing, "quota", maxWaitingChildrenPerParent, "peer", peerAddr)
		log.Printf("[node] 父块 %s 的等待子块已达每键配额 %d，丢弃 %s", hashHex32(parent),
			maxWaitingChildrenPerParent, hashHex)
		s.emitWaitingState("per_parent_quota_drop")
		return
	}
	// 既有的**不同父哈希**个数上限（256）——与 F-1 的每键配额相互独立，语义保持不变。
	if existing == 0 && len(s.waiting) >= maxWaitingBlocks {
		total := len(s.waiting)
		s.mu.Unlock()
		obs.Emit("RECOVERY_DROPPED", "orphan", hashHex, "parent", hashHex32(parent),
			"reason", "waiting_limit", "waiting_total", total, "peer", peerAddr)
		log.Printf("[node] 等待父块的孤儿区块已达上限 %d，丢弃 %s", maxWaitingBlocks, hashHex)
		s.emitWaitingState("waiting_limit_drop")
		return
	}
	s.waiting[parent] = append(s.waiting[parent], b)
	s.parkedHashes[hash] = struct{}{}
	s.mu.Unlock()

	// ORPHAN-DURABILITY-IMPLEMENTATION-1：入队成功 → 记录持久投影（延迟落盘）。
	// 锁纪律：在 s.mu.Unlock() 之后调用，orphanCheckpoint 内部自锁（独立锁），
	// 不与 nodeService.mu 嵌套（SPEC §5.3 R3）。nil 检查保证最小装配/单测无副作用。
	if s.orphanCP != nil {
		s.orphanCP.MarkDirty(parent, hash)
	}

	// I0/§3+§7：登记成功 → 具备恢复资格；同时记录 waiting-add 观测点。
	obs.Emit("RECOVERY_ELIGIBLE", "orphan", hashHex, "parent", hashHex32(parent),
		"waiting_children", existing+1, "peer", peerAddr)
	obs.Emit("WAITING_PARENT_ADD", "parent", hashHex32(parent), "orphan", hashHex,
		"waiting_children", existing+1, "via", "defer_orphan")
	s.emitWaitingState("add")
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
//
// PHASE E-IMPLEMENTATION-A（O-02/B-1）：本函数现在只是统一级联实现的入口之一；
// 唯一的级联实现在 cascadeFrom，via 标签与修复前逐字保持相同（行为等价）。
func (s *nodeService) applyResolved(peerAddr string, b *block.Block) {
	s.cascadeFrom(peerAddr, cascadeViaByHash, []*block.Block{b})
}

// resolveKnownBlock 是「一个区块刚刚变为已知（已 apply / 已 canonical）」的统一解析入口。
//
// PHASE E-IMPLEMENTATION-A（O-02/B-1）修复要点：
// 修复前 takeWaiting 只在 by-hash 响应路径（OnBlockByHashResp → applyResolved）被调用，
// 于是父块经其它合法到达面（网络广播 OnNewBlock / 批量同步 OnBlocksResp / 本地挖矿）
// 变为已知时，等待它的孤儿永远不会被释放——`waiting` 键恒为「不在 blocktree 中」的父哈希，
// 因此只有「把新块准入 blocktree 的到达面」才能解析它。
//
// 契约（不得违反）：
//   - 所有此类到达面必须且只能经本函数进入级联；到达点不得各自复制 takeWaiting。
//   - b 自身由调用方在此之前应用；本函数只处理「以 b 为父的等待孤儿」。
//   - 不改变任何校验/链选择/reorg 语义：每个被释放的子块仍逐个走 ingestBlock → 完整共识校验。
//
// 边界说明：本次不覆盖 reorg 采纳面。等待键恒为「当时不在 blocktree 中」的父哈希，
// 而 reorg 只能连接「已在 blocktree 中」的区块（ShouldReorg 作用于已入树节点），
// 故 reorg 采纳面在语义上不可能释放任何 waiting 条目——无需、也不应为其增加钩子。
func (s *nodeService) resolveKnownBlock(peerAddr, via string, b *block.Block) {
	s.cascadeFrom(peerAddr, via, s.takeWaiting(b.Header.Hash()))
}

// cascadeFrom 是全局唯一的 waiting 级联实现（Option A 单一漏斗）。
//
// 显式 FIFO 队列 + 有界展开，不递归。队列元素只来自 takeWaiting，
// 而 takeWaiting 取走即删除该键，因此同一孤儿最多入队一次（I-1/I-2/I-9）。
func (s *nodeService) cascadeFrom(peerAddr, via string, seeds []*block.Block) {
	queue := seeds
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
			// I0/§3+§7：级联路径应用成功。
			obs.Emit("RECOVERY_APPLIED", "block", cur.Header.HashHex(),
				"height", s.chain.Height(), "via", via)
			obs.Emit("WAITING_PARENT_APPLY", "parent", hashHex32(cur.Header.Hash()),
				"via", via)
			s.relayBlock(cur, peerAddr)
		}
		kids := s.takeWaiting(cur.Header.Hash())
		obs.Emit("WAITING_PARENT_MATCH", "parent", hashHex32(cur.Header.Hash()),
			"children", len(kids), "via", "take_waiting_cascade")
		queue = append(queue, kids...)
	}
}

// waitingSnapshotLocked 在持有 s.mu 时统计 waiting 规模（R-1 / F-7，仅用于观测）。
//
// 只读计数、不修改任何状态；不引入 goroutine、不落盘、不改 internal/*、不改 RPC schema。
func (s *nodeService) waitingSnapshotLocked() (keys, blocks, inflight int) {
	blocks = 0
	for _, v := range s.waiting {
		blocks += len(v)
	}
	return len(s.waiting), blocks, len(s.inflight)
}

// emitWaitingState 以既有 obs 事件路径输出 waiting / inflight 状态（R-1 / F-7）。
//
// 每一次 waiting 的插入或移除都恰好对应一次状态事件（Invariant H）。调用方**不得**在持有 s.mu 时调用。
func (s *nodeService) emitWaitingState(event string) {
	s.mu.Lock()
	keys, blocks, inflight := s.waitingSnapshotLocked()
	s.mu.Unlock()
	obs.Emit("WAITING_STATE", "waiting_keys", keys, "waiting_blocks", blocks, "inflight", inflight, "event", event)
}

// takeWaiting 取出并清空等待该父哈希的孤儿区块。
func (s *nodeService) takeWaiting(parentHash [32]byte) []*block.Block {
	s.mu.Lock()
	out := s.waiting[parentHash]
	delete(s.waiting, parentHash)
	// F-3：释放时同步移除哈希索引——既不残留，也允许同一区块之后再次入队（不重复释放、不重复计数）。
	for _, b := range out {
		delete(s.parkedHashes, b.Header.Hash())
	}
	s.mu.Unlock()
	// ORPHAN-DURABILITY-IMPLEMENTATION-1：父到达取走即删 → 删除持久投影（延迟落盘）。
	// 锁纪律：同 deferOrphan，在 s.mu.Unlock() 之后调用。
	if s.orphanCP != nil {
		s.orphanCP.Remove(parentHash)
	}
	s.emitWaitingState("release")
	return out
}

// prepareOrphanRestore 从检查点加载孤儿等待投影，构建待恢复父键集 restorePending（§4-B1）。
//
// 语义（对齐 STARTUP-RECOVERY-PLAN-1 §2）：
//   - 只从 cp 加载「父哈希 → 子哈希」引用，**不**重建 s.waiting、**不**伪造块指针；
//   - 对每个父键做去重（map 天然去重）；
//   - 跳过「本节点已知」的父（HasBlockHash 覆盖 canonical + 已落盘 detached），
//     因为父已知则其分支可经正常 sync 到达，无需恢复拉取；
//   - fail-closed：cp 为 nil 或 Load 失败（损坏/版本不兼容/读错误）→ restorePending 置空，
//     退化为 memory-only，**绝不 panic、绝不反向影响 canonical**。
//
// 本阶段只**填充** restorePending，不消费（消费属 §4-B2，OnHandshake 触发 requestBranch）。
func (s *nodeService) prepareOrphanRestore(cp *orphanCheckpoint) {
	s.restorePending = make(map[[32]byte]struct{})
	if cp == nil {
		return
	}
	entries, err := cp.Load()
	if err != nil {
		// fail-closed：检查点损坏/不兼容 → 空恢复，退化 memory-only（SPEC §3.2）。
		log.Printf("[node] 孤儿等待检查点不可用（%v），跳过恢复（退化为内存态）", err)
		return
	}
	for _, e := range entries {
		if s.chain.HasBlockHash(e.parentHash) {
			// §4-B1.5 D1：父已知（canonical 或 detached）→ 无需恢复拉取，且不在 waiting，
			// 从持久投影剪枝，避免 checkpoint 累积陈旧条目（下次 flush 即剔除）。
			cp.Remove(e.parentHash)
			continue
		}
		s.restorePending[e.parentHash] = struct{}{} // map 去重
	}
}

// consumeRestorePending 在每次握手末尾消费启动恢复集（§4-B2）。
//
// 设计契约（对齐 STARTUP-RECOVERY-PLAN-1 §2 与 §4-B2 READINESS AUDIT §2/§4.2）：
//   - 仅消费 restorePending 中「本节点仍未知」的父哈希，复用既有 requestBranch
//     （inflight / branchReqTTL / maxBranchRounds 去重限流）重新拉取缺失分支；
//   - 父一旦已知（HasBlockHash）→ 立即从 restorePending 与 checkpoint 双删（drain）；
//   - 父未知且已（尝试）请求 → 保留在 restorePending，等待下一轮握手/对端再尝试
//     （隐式有界重试：inflight/TTL/rounds 压制成环）；
//   - 绝不伪造块指针、绝不旁路孤儿准入、绝不腐蚀 s.waiting；
//   - 仅 OnHandshake 调用，幂等、可跨多次握手推进。
//
// 锁纪律：restorePending 读写全程 s.mu 保护；chain.HasBlockHash 与 requestBranch
// 在 s.mu 外调用（requestBranch 内部自锁 s.mu，不可嵌套）；orphanCP 内部自锁。
func (s *nodeService) consumeRestorePending(peerAddr string) {
	s.mu.Lock()
	if len(s.restorePending) == 0 {
		s.mu.Unlock()
		return // 快速路径：无待恢复项（含 restorePending 未初始化的最小装配）
	}
	// 复制出键集后释放锁：避免在持有 s.mu 时执行网络 I/O，杜绝 s.mu → p2p.Node.mu 锁序嵌套。
	pending := make([][32]byte, 0, len(s.restorePending))
	for p := range s.restorePending {
		pending = append(pending, p)
	}
	s.mu.Unlock()

	triggered := 0
	for _, p := range pending {
		if s.chain.HasBlockHash(p) {
			// 父已知（可能经正常 sync / 本次分支到达）→ drain：双删 restorePending + checkpoint。
			s.mu.Lock()
			delete(s.restorePending, p)
			s.mu.Unlock()
			if s.orphanCP != nil {
				s.orphanCP.Remove(p) // 幂等；仅在父已知时执行，绝不误删仍存活的 waiting 条目
			}
			obs.Emit("ORPHAN_RESTORE_DRAINED", "parent", hashHex32(p), "peer", peerAddr)
			continue
		}
		// 父仍未知 → 复用既有 by-hash 分支拉取（inflight/TTL/rounds 去重限流）。
		// 不在此处判定终态：成功由「父变为已知」在下一握手 drain；失败保留待重试。
		s.requestBranch(peerAddr, p)
		triggered++
	}
	if triggered > 0 {
		obs.Emit("ORPHAN_RESTORE_TRIGGERED", "count", uint64(triggered), "peer", peerAddr)
	}
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

// commitMinedBlock 是本地挖矿成功上链后的统一收尾（PHASE E-IMPLEMENTATION-A 抽出）。
//
// 抽出理由：P4（本地挖矿）是「父块刚变为已知」的到达面之一，必须与其它到达面共用同一
// 解析契约。把它固定在**一个可被测试直接驱动的生产方法**上，才能对"接线"本身做行为验证
// ——主流程 mineOnce 的候选块哈希在求解完成前不可预知，测试无法预先构造其子块，
// 因此无法从黑盒驱动 P4。
//
// 语义与抽出前逐条相同：交易池同步 → 计数 → 广播 → 统一解析入口级联。
func (s *nodeService) commitMinedBlock(b *block.Block, height int) {
	s.pool.RemoveIncluded(b, s.chain.UTXOSnapshot(), height)
	s.acceptedBlocks.Add(1)
	s.lastAcceptedAt.Store(time.Now().Unix())
	s.broadcastBlock(b)
	s.resolveKnownBlock("", cascadeViaLocalMining, b)
}

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
//
// MSF 状态机：
//
//	request → sweep（只判定与释放，绝不发送）
//	        → 去重 / 并发上限判定
//	        → 登记条目（归属 peer + 尝试次数 + TTL）
//	        → 发送一次
//	        → 响应到达   → clearSyncInflightLocked → 完成
//	        → TTL 到期 / 对端断开 / 发送失败 → 条目标记 failed
//	        → 由调度器 goroutine 按 backoff 重试（优先改投其他 peer）
//	        → 尝试次数耗尽 → 终态释放
//
// 关键不变量：**本函数只发送一次，失败后绝不在此处重试。**
// requestSync 运行在 peer 自己的读循环 goroutine 上，而 SendTo 是同步直写
// （持有 peer.mu，最长阻塞 writeTimeout=10s）；在此处重试会形成
// send→fail→retry 紧循环并卡死该 peer 的读循环。重试只由 runSyncSweep 执行。
func (s *nodeService) requestSync(peerAddr string, from int) {
	now := time.Now()
	peers, checkPeers := s.syncPeerSet()

	s.mu.Lock()
	s.sweepSyncLocked(now, peers, checkPeers)

	if r, ok := s.syncInflight[from]; ok {
		// RACE-REMEDIATION：在持有 s.mu 时先把归属/尝试/失败标志拷贝为不可变快照，
		// 解锁后再用于 obs.Emit —— 解锁后不得再读取该在途记录的任何并发可变字段
		//（调度器 goroutine 会在锁内并发写 r.peer/r.attempt/r.failed）。
		owner, attempt, failed := r.peer, r.attempt, r.failed
		s.mu.Unlock()
		// I0/§6：请求被在途批抑制（抑制也是 SYNC 生命周期的合法状态）。
		// MSF：事件名与 reason 逐字保持不变（观测兼容），仅补充归属与尝试信息。
		obs.Emit("SYNC_REQUEST", "peer", peerAddr, "from_height", from,
			"suppressed", true, "reason", "pending_in_flight",
			"owner", owner, "attempt", attempt, "failed", failed)
		return // 同一区间已在途（无论归属哪个 peer）：等响应，或等调度器重投
	}
	if len(s.syncInflight) >= maxInflightSync {
		n := len(s.syncInflight)
		s.mu.Unlock()
		obs.Emit("SYNC_REQUEST", "peer", peerAddr, "from_height", from,
			"suppressed", true, "reason", "sync_cap",
			"inflight", n, "max_inflight", maxInflightSync)
		return
	}
	r := &syncRequest{from: from, peer: peerAddr, attempt: 1, created: now}
	r.deadline = now.Add(s.syncTTL)
	s.syncInflight[from] = r
	s.syncing = true
	n := len(s.syncInflight)
	s.mu.Unlock()

	// I0/§6：SYNC_REQUEST 真正发出。
	obs.Emit("SYNC_REQUEST", "peer", peerAddr, "from_height", from, "suppressed", false,
		"batch", MaxSyncBatch)
	obs.Emit("SYNC_REQ_CREATED", "peer", peerAddr, "from_height", from, "attempt", 1,
		"inflight", n, "max_inflight", maxInflightSync, "ttl_s", s.syncTTL.Seconds())
	s.sendSyncRequest(r)
}

// sendSyncRequest 发出一次 GetBlocks，并把结果落到条目状态上。
// 调用方必须已在本函数之外设置好 r.peer / r.attempt / r.deadline。
func (s *nodeService) sendSyncRequest(r *syncRequest) {
	payload, err := json.Marshal(p2p.GetBlocksPayload{FromHeight: r.from, Count: MaxSyncBatch})
	if err != nil {
		log.Printf("[node] 构造同步请求失败: %v", err)
		// 修复前此路径**不复位** pending（潜在泄漏）；现在统一走失败判定。
		s.markSyncAttemptFailed(r, "marshal_failed")
		return
	}
	if err := s.net.SendTo(r.peer, p2p.Message{Type: p2p.MsgGetBlocks, Payload: payload}); err != nil {
		log.Printf("[node] 发送同步请求给 %s 失败: %v", r.peer, err)
		obs.Emit("SYNC_REQUEST", "peer", r.peer, "from_height", r.from,
			"suppressed", false, "result", "send_failed")
		s.markSyncAttemptFailed(r, "send_failed")
		return
	}
	log.Printf("[node] 已向 %s 请求区块，起始高度=%d", r.peer, r.from)
	obs.Emit("SYNC_REQ_SENT", "peer", r.peer, "from_height", r.from, "attempt", r.attempt)
}

// markSyncAttemptFailed 把一次尝试判定为失败，并决定「等待重试」还是「终态释放」。
//
// 本函数**从不发送**任何报文 —— 重试发送只由调度器 goroutine 执行，
// 这是「不在 peer 读循环内重试」的结构性保证。
func (s *nodeService) markSyncAttemptFailed(r *syncRequest, reason string) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.syncInflight[r.from]
	if !ok || cur != r {
		return // 已被响应或已被释放：本次结果作废
	}
	s.markSyncFailedLocked(r, reason, now)
}

// markSyncFailedLocked 在持有 s.mu 时把条目判负：终态释放 或 标记等待重试。
func (s *nodeService) markSyncFailedLocked(r *syncRequest, reason string, now time.Time) {
	switch {
	case !s.syncSchedStarted:
		// 调度器未启动 ⇒ 行为与修复前 `pending = 0` 完全一致：
		// 立即释放，把机会交回下一次握手（不引入任何新的重试语义）。
		delete(s.syncInflight, r.from)
		s.syncing = len(s.syncInflight) > 0
		obs.Emit("SYNC_REQ_RELEASED", "peer", r.peer, "from_height", r.from,
			"reason", reason, "terminal", true, "attempt", r.attempt,
			"detail", "scheduler_not_started")
	case r.attempt >= maxSyncAttempts:
		delete(s.syncInflight, r.from)
		s.syncing = len(s.syncInflight) > 0
		obs.Emit("SYNC_REQ_RELEASED", "peer", r.peer, "from_height", r.from,
			"reason", reason, "terminal", true, "attempt", r.attempt,
			"detail", "attempts_exhausted")
		log.Printf("[node] 同步请求（起始高度=%d）已尝试 %d 次仍未成功，本轮放弃",
			r.from, r.attempt)
	default:
		backoff := s.backoffFor(r.attempt)
		r.failed = true
		r.nextRetry = now.Add(backoff)
		obs.Emit("SYNC_REQ_FAILED", "peer", r.peer, "from_height", r.from,
			"reason", reason, "attempt", r.attempt, "max_attempts", maxSyncAttempts,
			"retry_in_s", backoff.Seconds())
	}
}

// sweepSyncLocked 在持有 s.mu 时扫描在途登记表：
//   - 已过 TTL，或归属 peer 已不在连接列表中 ⇒ 本次尝试判负；
//   - 判负后按 markSyncFailedLocked 决定终态释放还是等待重试。
//
// 本函数**绝不发送**报文：只做判定与释放，因此可以从 peer 读循环安全调用。
func (s *nodeService) sweepSyncLocked(now time.Time, peers map[string]bool, checkPeers bool) {
	for _, r := range s.syncInflight {
		if r.failed {
			continue // 已在退避闸门后等待调度器重试
		}
		switch {
		case now.After(r.deadline):
			s.markSyncFailedLocked(r, "ttl_expired", now)
		case checkPeers && !peers[r.peer]:
			s.markSyncFailedLocked(r, "peer_disconnected", now)
		}
	}
}

// clearSyncInflightLocked 释放全部在途条目（批完成 / 异常终止 / 续拉下一批时调用）。
//
// 语义与修复前的 `pending = 0` 完全一致：批量同步串行推进，
// 一批收到（或终止）后此前登记的所有区间都不再需要在途。
func (s *nodeService) clearSyncInflightLocked() {
	if len(s.syncInflight) == 0 {
		return
	}
	n := len(s.syncInflight)
	s.syncInflight = make(map[int]*syncRequest)
	obs.Emit("SYNC_REQ_CLEARED", "cleared", n)
}

// runSyncSweep 一次调度巡检：先判定释放，再按 backoff 重试发送。
// 这是**唯一**执行重试发送的地方，且只运行在调度器 goroutine 上。
func (s *nodeService) runSyncSweep(now time.Time) {
	peers, checkPeers := s.syncPeerSet()

	s.mu.Lock()
	s.sweepSyncLocked(now, peers, checkPeers)
	retries := make([]*syncRequest, 0, len(s.syncInflight))
	for _, r := range s.syncInflight {
		if r.failed && !now.Before(r.nextRetry) {
			// 先摘牌并续期：避免并发巡检把同一条目重复判负/重复取用。
			r.failed = false
			r.created = now
			r.deadline = now.Add(s.syncTTL)
			retries = append(retries, r)
		}
	}
	s.mu.Unlock()

	for _, r := range retries {
		s.retrySync(r, now)
	}

	// §4-B1.5 D2：孤儿等待检查点周期落盘（best-effort，脏状态 + 节流）。
	// 复用既有 sync 巡检节拍；仅在脏时写盘，且按 orphanCPFlushInterval 节流，
	// 文件极小（≤ ~512KB），写放大可忽略。失败仅 warning，不退化、不反向影响 canonical。
	if s.orphanCP != nil {
		if wrote, err := s.orphanCP.FlushIfDirty(orphanCPFlushInterval); err != nil {
			log.Printf("[node] 孤儿等待检查点周期落盘失败（仅影响孤儿可用性）: %v", err)
		} else if wrote {
			obs.Emit("ORPHAN_CP_FLUSH", "trigger", "periodic")
		}
	}
}

// retrySync 重新投递一次同步请求：peer failover + 次数递增 + TTL 重置。
func (s *nodeService) retrySync(r *syncRequest, now time.Time) {
	peer, ok := s.pickSyncPeer(r.peer)
	if !ok {
		// 已无任何可用对端：终态释放（等新 peer 握手时重新登记）。
		// 不消耗尝试次数 —— 无对端不是对端的错，但也不该让条目无限占位。
		s.mu.Lock()
		if cur := s.syncInflight[r.from]; cur == r {
			delete(s.syncInflight, r.from)
			s.syncing = len(s.syncInflight) > 0
		}
		s.mu.Unlock()
		obs.Emit("SYNC_REQ_RELEASED", "peer", r.peer, "from_height", r.from,
			"reason", "no_peer", "terminal", true, "attempt", r.attempt)
		return
	}
	s.mu.Lock()
	if cur := s.syncInflight[r.from]; cur != r {
		s.mu.Unlock()
		return // 期间已被响应或释放
	}
	r.peer = peer
	r.attempt++
	r.created = now
	r.deadline = now.Add(s.syncTTL)
	r.failed = false
	r.nextRetry = time.Time{}
	s.mu.Unlock()

	obs.Emit("SYNC_REQ_RETRY", "peer", r.peer, "from_height", r.from, "attempt", r.attempt)
	log.Printf("[node] 重试同步请求（起始高度=%d）：改投 %s（第 %d/%d 次）",
		r.from, r.peer, r.attempt, maxSyncAttempts)
	s.sendSyncRequest(r)
}

// pickSyncPeer 为重试挑选对端：**优先**与上次不同的 peer（failover），
// 只有它一个时才退化为复用（仍受 backoff 节流）。
// 按地址字典序确定，不引入随机性，保证结果可复现。
func (s *nodeService) pickSyncPeer(exclude string) (string, bool) {
	if s.net == nil {
		return "", false
	}
	addrs := s.net.PeerAddrs()
	if len(addrs) == 0 {
		return "", false
	}
	sort.Strings(addrs)
	for _, a := range addrs {
		if a != exclude {
			return a, true
		}
	}
	return addrs[0], true
}

// syncPeerSet 当前仍连接的对端集合；s.net 未装配时 ok=false（跳过断连判定）。
//
// 刻意**在取 s.mu 之前**调用：避免 s.mu → p2p.Node.mu 的锁序嵌套。
func (s *nodeService) syncPeerSet() (set map[string]bool, ok bool) {
	if s.net == nil {
		return nil, false
	}
	addrs := s.net.PeerAddrs()
	out := make(map[string]bool, len(addrs))
	for _, a := range addrs {
		out[a] = true
	}
	return out, true
}

// backoffFor 第 attempt 次失败后的退避时长：base × 2^(attempt-1)。
func (s *nodeService) backoffFor(attempt int) time.Duration {
	shift := attempt - 1
	if shift < 0 {
		shift = 0
	}
	if shift > 8 {
		shift = 8
	}
	return s.syncRetryBase << shift
}

// startSyncScheduler 启动在途同步调度器（幂等；生命周期内只启动一次）。
//
// 调度器运行在自己的 goroutine 上，与任何 peer 的读循环解耦，
// 因此重试不可能退化为 send→fail→retry 紧循环。
func (s *nodeService) startSyncScheduler() {
	s.syncStartOne.Do(func() {
		s.mu.Lock()
		s.syncSchedStarted = true
		interval := s.syncSweepInterval
		s.mu.Unlock()
		if interval <= 0 {
			interval = defaultSyncSweepInterval
		}
		s.syncWG.Add(1)
		go s.syncSchedulerLoop(interval)
	})
}

// syncSchedulerLoop 调度器主循环：按固定周期巡检，直到 stopSyncScheduler。
func (s *nodeService) syncSchedulerLoop(interval time.Duration) {
	defer s.syncWG.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.syncStopCh:
			return
		case <-ticker.C:
			s.runSyncSweep(time.Now())
		}
	}
}

// stopSyncScheduler 停止调度器（幂等）。
//
// 只关闭通道、不等待 goroutine 退出：避免在 SendTo 被慢对端卡住时拖长关停路径
// （goroutine 最多再活一个巡检周期即自行退出）。
func (s *nodeService) stopSyncScheduler() {
	s.syncStopOnce.Do(func() {
		s.mu.Lock()
		s.syncSchedStarted = false
		s.mu.Unlock()
		close(s.syncStopCh)
	})
}

// syncInflightSnapshot 测试与诊断用的在途快照（不参与任何业务分支）。
func (s *nodeService) syncInflightSnapshot() map[int]syncRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[int]syncRequest, len(s.syncInflight))
	for k, r := range s.syncInflight {
		out[k] = *r
	}
	return out
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
