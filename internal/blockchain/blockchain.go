// Package blockchain 维护本地节点看到的区块链视图，负责：
//   - 区块的验证与追加（全序共识校验，见 ValidateBlock）
//   - UTXO 状态机的持有与原子迁移（委托 internal/utxo）
//   - 难度调整的触发（委托 internal/pow）
//
// 线程模型：全链操作受 mu 保护；UTXO 集合自身亦有内部锁，
// 对外暴露的 UTXOSnapshot() 返回克隆快照，避免调用方直接持有可变状态。
//
// 分叉处理（reorg）已实现：基于 internal/blocktree 的区块树与 ShouldReorg（累积工作量比较），由 Blockchain.executeReorg 执行 disconnect→apply→persist 的整链替换（详见 executeReorg / AddBlock）。
package blockchain

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/blocktree"
	"p2pchain/internal/obs"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
	"p2pchain/internal/utxo"
)

// obsHashHex 观测专用：[32]byte → 完整十六进制字符串。
func obsHashHex(h [32]byte) string { return hex.EncodeToString(h[:]) }

var (
	ErrInvalidPrevHash     = errors.New("区块的前置哈希与当前链尾不匹配")
	ErrInvalidPoW          = errors.New("区块哈希未达到难度目标，工作量证明无效")
	ErrInvalidVersion      = errors.New("区块版本与当前激活高度的共识规则不一致")
	ErrEmptyChain          = errors.New("链为空")
	ErrUnknownHeight       = errors.New("请求的区块高度不存在")
	ErrUnexpectedBits      = errors.New("区块难度位与当前共识难度不一致")
	ErrTimestampOutOfRange = errors.New("区块时间戳超出允许范围")
	ErrMerkleMismatch      = errors.New("区块头 Merkle 根与交易列表不匹配")
	ErrBlockTooLarge       = errors.New("区块超过最大体积限制")
	ErrBadTxLayout         = errors.New("区块交易布局非法（coinbase 位置/数量）")

	// ErrOrphanParent 是 ErrInvalidPrevHash 的特化：区块自身结构可被解析，
	// 但它的父区块不在本地区块树中（P2P 到达顺序导致的 orphan）。
	//
	// 以 fmt.Errorf("%w", ErrInvalidPrevHash) 构造，故
	// errors.Is(err, ErrInvalidPrevHash) 依然为 true —— 「非链尾父哈希一律
	// ErrInvalidPrevHash」的全部既有断言保持成立；新增该哨兵只是让上层 P2P 能
	// 区分「已知父的合法分叉块」（走 reorg 决策）与「缺父的孤块」（走 by-hash
	// 分支拉取）。REORG-1H 依赖这一区分。
	ErrOrphanParent = fmt.Errorf("%w: 父区块未知（orphan）", ErrInvalidPrevHash)
)

const (
	// MaxBlockSize 区块最大体积（字节）：按 block 的规范 JSON 序列化长度计算。
	// JSON 字段顺序由结构体定义固定，跨节点计算结果一致。
	MaxBlockSize = 1 << 20 // 1 MiB

	// maxFutureTimestampDrift 允许区块时间戳超前本地时钟的最大秒数（比特币为 2 小时）。
	maxFutureTimestampDrift = 7200
)

// Blockchain 内存中的区块链结构。
// blocks 按高度顺序存储；utxo 为当前链的未花费输出集合（与 blocks 尾部一致）。
// store 非空时，运行时新区块（AddBlock）追加成功后同步落盘；
// 启动时的历史区块回放在此读取并重建，**回放过程不写回存储**。
type Blockchain struct {
	mu     sync.RWMutex
	blocks []*block.Block
	utxo   *utxo.UTXOSet
	store  storage.BlockStore

	// tree 是区块的内存树索引（REORG-1C），用于 fork detection、chainwork 比较、
	// common ancestor 计算。不持久化，启动时从 storage 重建。
	tree *blocktree.BlockTree

	// hashIndex 是 C-1（AUTH-2-C1）引入的内存 hash→block 索引：bc.blocks 的
	// 派生投影，仅用于降低 blockAtHash 的查找成本（O(h) 线性扫描 → 平均 O(1)）。
	// key = Header.Hash()；value = 与 bc.blocks 共享的 *block.Block（不复制）。
	// 非共识状态、绝不持久化；重启后由 NewBlockchainWithGenesis /
	// NewBlockchainFromStore（recovery 回放）路径自动重建。
	// 维护不变量：hashIndex 的 key 集合 ≡ bc.blocks 的哈希集合。仅有的两个
	// mutation 点是 extendChain 的 canonical append（同步写入）与 executeReorg
	// 的整链替换（同步重建），二者都必须与 bc.blocks 在同一 bc.mu 临界区内
	// 完成。index 命中 ≠ canonical 判据（同 blockAtHash 的调用方须知）。
	hashIndex map[[32]byte]*block.Block

	// canonicalSet 是 C-d（AUTH-2-Cd）引入的内存 canonical member hash set：
	// bc.blocks 的纯派生投影，把 canonicalContains 的 membership 判定由
	// O(h) 线性扫描降为平均 O(1)。key = Header.Hash()。
	// 非共识状态、绝不持久化；重启后由 NewBlockchainWithGenesis /
	// NewBlockchainFromStore（recovery 回放经 extendChain）路径自动重建。
	// 维护不变量：canonicalSet ≡ { b.Header.Hash() : b ∈ bc.blocks }（双向集合相等）。
	// 仅有的两个 mutation 点是 extendChain 的 canonical append（同步插入）与
	// executeReorg 的整链替换（同步全量重建），二者都必须与 bc.blocks 在同一
	// bc.mu 临界区内完成。
	// 与 hashIndex 的区别：hashIndex 的「命中」不等于 canonical 判据（其查找
	// 路径含 store 侧扩展语义）；本 set 仅投影 bc.blocks，故「命中」即 canonical。
	canonicalSet map[[32]byte]struct{}

	// activationHeight 是难度浮动/新时间戳-MTP/新版本强制生效的高度。
	// 默认取共识常量 pow.ActivationHeight（2000，> 当前生产高度，保证存量链不破）；
	// 测试可注入更小的高度以越过激活边界而无需真挖 2000 块。
	// 该字段是共识真值的一部分，绝不在运行期变更。
	activationHeight int

	// lastReorgResult 记录最近一次 AddBlock 触发的 reorg 结果（REORG-1F）。
	// 由 executeReorg 设置，由 AddBlock/AddBlockWithResult 的调用方读取。
	// 非共识状态——mempool resurrection 使用，不影响 canonical tip 选择。
	lastReorgResult *ReorgResult
}

// ReorgResult 记录一次链重组的完整信息，供上层（service/mempool）做 resurrection。
type ReorgResult struct {
	OldTip           *blocktree.BlockNode
	NewTip           *blocktree.BlockNode
	DisconnectBlocks []*block.Block
	ConnectBlocks    []*block.Block
}

// reorgStore 是 storage.FileBlockStore 提供的最小 v2 接口子集，
// 供 blockchain 通过类型断言调用 reorg 原语，避免修改 storage.BlockStore 骨架接口。
type reorgStore interface {
	storage.BlockStore
	AppendCanonicalBlock(b *block.Block, undo utxo.BlockUndo) error
	SaveBlockDetached(b *block.Block) error
	SaveBlockWithUndo(b *block.Block, undo utxo.BlockUndo) error
	CommitTip(hash [32]byte) error
	CommitReorg(detachedUndos map[[32]byte]utxo.BlockUndo, newBlocks []*block.Block, newUndos []utxo.BlockUndo, newTipHash [32]byte) error
	HasBlock(hash [32]byte) bool
	HasUndo(hash [32]byte) bool
	V2Mode() bool
}

// NewBlockchainWithGenesis 使用给定的创世区块初始化链（不持久化，供测试/临时链使用）。
// 创世区块视为可信（不做头校验），但其交易必须能成功建立初始 UTXO 状态。
func NewBlockchainWithGenesis(genesis *block.Block) (*Blockchain, error) {
	genesisSet, _, err := utxo.ApplyBlock(utxo.NewUTXOSet(), genesis.Transactions, 0)
	if err != nil {
		return nil, fmt.Errorf("创世区块 UTXO 初始化失败: %w", err)
	}
	tree := blocktree.NewBlockTree()
	node, err := tree.AddBlock(genesis.Header.Hash(), [32]byte{}, 0, genesis.Header.Bits, genesis.Header.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("创世区块加入树索引失败: %w", err)
	}
	_ = tree.SetTip(node)
	bc := &Blockchain{
		blocks:           []*block.Block{genesis},
		utxo:             genesisSet,
		tree:             tree,
		activationHeight: pow.ActivationHeight,
	}
	// C-1：创世块进入 hash index（此后 recovery 回放经 extendChain 逐块补全）。
	bc.hashIndex = map[[32]byte]*block.Block{genesis.Header.Hash(): genesis}
	// C-d：创世块进入 canonical member hash set（同上，recovery 回放经 extendChain 补全）。
	bc.canonicalSet = map[[32]byte]struct{}{genesis.Header.Hash(): {}}
	return bc, nil
}

// rebuildHashIndexLocked 从当前 bc.blocks 全量重建 hashIndex（C-1）。
// 调用方必须持有 bc.mu 写锁（当前唯一调用点在 executeReorg 临界区内）。
// O(len(bc.blocks))，相对 reorg 本身的成本可忽略。
func (bc *Blockchain) rebuildHashIndexLocked() {
	idx := make(map[[32]byte]*block.Block, len(bc.blocks))
	for _, b := range bc.blocks {
		idx[b.Header.Hash()] = b
	}
	bc.hashIndex = idx
}

// rebuildCanonicalSetLocked 从当前 bc.blocks 全量重建 canonicalSet（C-d）。
// 调用方必须持有 bc.mu 写锁（唯一常规调用点在 executeReorg 临界区内；另在
// extendChain 的防御路径中使用）。O(len(bc.blocks))，相对 reorg 本身的成本可忽略。
func (bc *Blockchain) rebuildCanonicalSetLocked() {
	set := make(map[[32]byte]struct{}, len(bc.blocks))
	for _, b := range bc.blocks {
		set[b.Header.Hash()] = struct{}{}
	}
	bc.canonicalSet = set
}

// NewBlockchainWithGenesisAndActivation 同 NewBlockchainWithGenesis，但允许显式指定
// 激活高度——**仅供测试**越过难度共识硬分叉边界（无需真挖 2000 块）。
// 生产路径一律使用 NewBlockchainWithGenesis（取共识默认 pow.ActivationHeight）。
func NewBlockchainWithGenesisAndActivation(genesis *block.Block, activationHeight int) (*Blockchain, error) {
	bc, err := NewBlockchainWithGenesis(genesis)
	if err != nil {
		return nil, err
	}
	bc.activationHeight = activationHeight
	return bc, nil
}

// rebuildTree 从当前 bc.blocks 重建 blocktree（启动回放后调用）。
func (bc *Blockchain) rebuildTree() error {
	bc.tree = blocktree.NewBlockTree()
	for i, b := range bc.blocks {
		_, err := bc.tree.AddBlock(b.Header.Hash(), b.Header.PrevBlockHash, i, b.Header.Bits, b.Header.Timestamp)
		if err != nil {
			return fmt.Errorf("重建树索引 高度 %d 失败: %w", i, err)
		}
	}
	// 初始活动链尾 = 当前链尾
	if tip := bc.blocks[len(bc.blocks)-1]; len(bc.blocks) > 0 {
		if node := bc.tree.LookupNode(tip.Header.Hash()); node != nil {
			_ = bc.tree.SetTip(node)
		}
	}
	return nil
}

// NewBlockchainFromStore 从已显式初始化的持久化存储加载链。
//
// 空库不会自动生成创世：必须先由显式 init 流程调用
// InitializeBlockchainStore。启动回放前统一经过 VerifyGenesisIdentity，
// 任何身份不匹配或未初始化状态都在 consensus replay 前 fail closed。
func NewBlockchainFromStore(store storage.BlockStore) (*Blockchain, error) {
	h, err := store.Height()
	if err != nil {
		return nil, fmt.Errorf("读取存储高度失败: %w", err)
	}
	if h < 0 {
		return nil, ErrUninitializedStore
	}

	first, err := VerifyGenesisIdentity(store)
	if err != nil {
		return nil, err
	}
	bc, err := NewBlockchainWithGenesis(first)
	if err != nil {
		return nil, err
	}
	bc.store = store
	for i := 1; i <= h; i++ {
		b, err := store.GetBlockByHeight(i)
		if err != nil {
			return nil, fmt.Errorf("读取高度 %d 区块失败: %w", i, err)
		}
		// 回放入口：这些区块本就来自磁盘，只校验并重建内存，绝不写回（否则会把
		// blocks.dat 变成不断增长的重复日志，见 addBlock 注释）。
		if err := bc.applyBlock(b); err != nil {
			return nil, fmt.Errorf("回放高度 %d 区块失败（数据可能损坏）: %w", i, err)
		}
	}
	// REORG-1C：从回放后的 canonical 链重建 blocktree
	if err := bc.rebuildTree(); err != nil {
		return nil, fmt.Errorf("重建 blocktree 失败: %w", err)
	}
	return bc, nil
}

// Height 返回当前链的高度（创世区块高度为 0）。
func (bc *Blockchain) Height() int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return len(bc.blocks) - 1
}

// Tip 返回当前链尾（最新）区块。
func (bc *Blockchain) Tip() (*block.Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	if len(bc.blocks) == 0 {
		return nil, ErrEmptyChain
	}
	return bc.blocks[len(bc.blocks)-1], nil
}

// UTXOSnapshot 返回当前 UTXO 集合的克隆快照（供 mempool / 余额查询使用，
// 避免调用方直接持有可变状态）。
func (bc *Blockchain) UTXOSnapshot() *utxo.UTXOSet {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.utxo.Clone()
}

// BlockByHeight 按高度取出区块（创世区块高度为 0）。
func (bc *Blockchain) BlockByHeight(height int) (*block.Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	if height < 0 || height >= len(bc.blocks) {
		return nil, fmt.Errorf("%w: %d（当前高度 %d）", ErrUnknownHeight, height, len(bc.blocks)-1)
	}
	return bc.blocks[height], nil
}

// BlocksFrom 从 from 高度（含）开始返回最多 count 个区块，并告知是否已到链尾。
// 供 P2P 同步响应使用；count <= 0 时返回空切片。
func (bc *Blockchain) BlocksFrom(from, count int) (blocks []*block.Block, atTip bool) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	if count <= 0 || from < 0 || from >= len(bc.blocks) {
		return nil, from >= len(bc.blocks)
	}
	end := from + count
	if end > len(bc.blocks) {
		end = len(bc.blocks)
	}
	out := make([]*block.Block, 0, end-from)
	out = append(out, bc.blocks[from:end]...)
	return out, end == len(bc.blocks)
}

// CurrentBits 返回下一个待挖区块应当使用的难度目标。
// 只有到达调整周期的整数倍高度才重新计算，否则沿用链尾的难度。
func (bc *Blockchain) CurrentBits() uint32 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.expectedBitsFor(len(bc.blocks))
}

// currentBitsLocked 已被 expectedBitsFor 取代：始终基于本链（活动链）视图计算候选块难度，
// 与 ComputeExpectedBitsAt 共享同一份冻结契约。
func (bc *Blockchain) currentBitsLocked() uint32 {
	return bc.expectedBitsFor(len(bc.blocks))
}

// validateBlock 对区块执行全序共识校验；全部通过时返回应用后的新 UTXO 集合。
// 调用方必须已持有锁（AddBlock 写锁 / ValidateBlock 读锁）。
// 校验顺序（任何一步失败立即拒绝）：
//  1. PrevHash        1.5 版本          1.6 难度位共识域（F-4 廉价闸门）
//  2. PoW             3. Bits == 共识难度
//  4. 时间戳范围      5. Merkle 重验    6. 体积上限
//  7. coinbase 位置/数量 + 全部交易的状态迁移（签名/双花/maturity/金额/coinbase 上限）
//
// 关于 1.6（PHASE F-4-CONSENSUS-INPUT-HARDENING-REMEDIATION）：
// bits 完全由对端控制，而目标构造 target=2^(256-bits) 在 bits>256 时按 uint32
// 回绕，产生 2^32 bit（≈512 MiB）级分配；必须在任何大整数构造之前拒绝。
// 该闸门只拒绝「任何合法区块都不可能取到」的值（共识域 [1,256]，由
// pow.IsBitsInConsensusDomain 定义，与 blocktree/storage 既有判定同域），
// 因此**不改变接受集合**；权威规则仍是第 3 步的等值校验。
// 它也不改变「PoW 先于等值校验」这一防 DoS 顺序——PoW 仍在第 2 步。
//
// skipPoW 仅供本地挖矿模板预校验使用（PHASE MINING-REMEDIATION-1）：
// 未求解的候选区块必然不满足 PoW，若照常执行第 2 步会恒返回 ErrInvalidPoW，
// 从而掩盖 coinbase / UTXO / 交易 / 结构层面的真实错误。skipPoW = true 时
// **只跳过第 2 步**，其余步骤与顺序完全不变 —— 因此它与完整校验共享同一份规则，
// 不存在「两套共识规则」的漂移风险。
//
// 安全边界：skipPoW = true 绝不能用于来自网络的区块（跳过 PoW 即放弃
// 防伪造区块的 DoS 保护）；该路径只允许由 ValidateTemplate 在本地挖矿路径调用。
//
// 本函数的规则顺序（尤其是 PoW 位于第 2 步）不得调整：对网络入块而言，
// 先验 PoW 是必要且正确的防 DoS 设计。
func (bc *Blockchain) validateBlock(b *block.Block, skipPoW bool) (*utxo.UTXOSet, error) {
	tip := bc.blocks[len(bc.blocks)-1]
	height := len(bc.blocks) // 新区块高度

	// 1. 链式结构
	if b.Header.PrevBlockHash != tip.Header.Hash() {
		return nil, ErrInvalidPrevHash
	}
	// 1.5 版本号必须与该高度激活的共识规则一致（硬分叉强制，见 consensus.go）
	if err := bc.validateVersion(b, height); err != nil {
		return nil, err
	}
	// 1.6 难度位共识域闸门（F-4 输入加固）：拒绝越界 bits，且**不构造任何目标值**。
	// 合法区块 bits == 期望难度 ∈ [1, MaxDifficultyBits=32] ⊂ [1,256] ⇒ 接受集合不变。
	if !pow.IsBitsInConsensusDomain(b.Header.Bits) {
		return nil, fmt.Errorf("%w: 区块难度位 %d 超出共识域（拒绝，未构造目标值；F-4 输入加固）",
			ErrUnexpectedBits, b.Header.Bits)
	}
	// 2. 工作量证明（skipPoW 时跳过 —— 见函数注释的安全边界）
	if !skipPoW && !pow.Validate(&b.Header) {
		return nil, ErrInvalidPoW
	}
	// 3. 难度位必须与基于本链计算的期望难度一致（防止矿工私降/私升难度）
	if err := bc.validateBits(b, height); err != nil {
		return nil, err
	}
	// 4. 时间戳：按激活状态分叉（旧规则保留墙钟上限；新规则改用 MTP，剥离墙钟）
	if err := bc.validateTimestamp(b, tip, height); err != nil {
		return nil, err
	}
	// 5. Merkle 重验（防「同 Merkle 根不同交易集合」与头/体不一致）
	if block.ComputeMerkleRoot(b.Transactions) != b.Header.MerkleRoot {
		return nil, ErrMerkleMismatch
	}
	// 6. 体积上限
	if size := b.Size(); size > MaxBlockSize {
		return nil, fmt.Errorf("%w: %d > %d", ErrBlockTooLarge, size, MaxBlockSize)
	}
	// 7. 交易层：coinbase 布局 + 签名 + 双花 + maturity + 金额 + coinbase 上限，
	//    在克隆集合上原子迁移（内部保证失败不留半迁移状态）
	newSet, _, err := utxo.ApplyBlock(bc.utxo, b.Transactions, height)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadTxLayout, unwrapLayoutErr(err))
	}
	return newSet, nil
}

// unwrapLayoutErr 把 utxo.ApplyBlock 的「布局类」错误归一到 ErrBadTxLayout 语义；
// 金额超限等保留原始错误以便上层识别。
func unwrapLayoutErr(err error) error { return err }

// ValidateBlock 对一个待追加的区块做全序共识校验（不改变链状态）。
// 通过意味着：该区块可以直接被 AddBlock 接受。
func (bc *Blockchain) ValidateBlock(b *block.Block) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	if _, err := bc.validateBlock(b, false); err != nil {
		return err
	}
	return nil
}

// ValidateTemplate 对一个**本地构造、尚未求解 PoW** 的挖矿候选区块做结构校验。
//
// 与 ValidateBlock 使用完全相同的共识规则与校验顺序，唯一差异是跳过第 2 步 PoW。
// 用途：在付出 PoW 代价之前判定「该模板在当前链状态下是否存在合法候选」，
// 使 `Subsidy(height)+fees == 0`（补贴耗尽且无手续费交易）这类**机械可判定为
// 不可满足**的模板立即被识别，而不是先烧掉整套 PoW 再被拒绝。
//
// 返回值语义：
//   - ErrInvalidPrevHash：链尾已变化（模板陈旧）—— 重建模板即可，不是缺陷；
//   - 其它错误：结构性错误 —— 属于代码/策略不一致，挖矿应进入 FAILED；
//   - nil：模板结构合法，可以进入 PoW。
//
// 不改变链状态：内部在 UTXO 集合的克隆上做状态迁移，返回值被丢弃。
func (bc *Blockchain) ValidateTemplate(b *block.Block) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	if _, err := bc.validateBlock(b, true); err != nil {
		return err
	}
	return nil
}

// IsTemplateStale 判定 ValidateTemplate 的失败是否仅为「链/链尾已变化」这一类
// 可重试原因（模板本身并不非法）：
//
//   - ErrInvalidPrevHash      ：模板构造所依据的链尾已被替换；
//   - ErrTimestampOutOfRange  ：链尾时间戳已前移到本模板时间戳之后（对端区块领先）；
//   - ErrUnexpectedBits       ：共识难度位已变化（因链尾变化导致）。
//
// 这三者都无法由模板构造逻辑本身产生，重建模板后重试即可，不应误判为结构性缺陷。
// 其余错误（coinbase 布局/金额、UTXO 迁移、Merkle、体积等）一律视为结构性错误。
func IsTemplateStale(err error) bool {
	return errors.Is(err, ErrInvalidPrevHash) ||
		errors.Is(err, ErrTimestampOutOfRange) ||
		errors.Is(err, ErrUnexpectedBits)
}

// 区块追加语义（务必注意两类入口的区别）：
//
//   - 运行时新区块（挖矿产出 / P2P 收到）：AddBlock  —— 校验 + 入内存 + 落盘
//   - 启动时回放历史区块：               applyBlock —— 校验 + 入内存 + **不落盘**
//
// 回放必须不落盘：历史区块本来就来自磁盘，回放时再 SaveBlock 一次等于把同一条记录
// 重复追加进 append-only 的 blocks.dat，导致存储里的记录数多于链的实际高度
// （同一个区块占据两个高度），下一次启动时回放会因 prev-hash 不匹配而拒绝加载，
// 节点永久无法启动（P0 · GENESIS-0）。
func (bc *Blockchain) AddBlock(b *block.Block) error {
	bc.mu.Lock()
	bc.lastReorgResult = nil
	bc.mu.Unlock()
	return bc.addBlock(b, true)
}

// AddBlockWithResult 与 AddBlock 相同，但额外返回 reorg 结果（如有）。
// 若触发 reorg，返回的 *ReorgResult 包含断开/连接分支的区块信息。
func (bc *Blockchain) AddBlockWithResult(b *block.Block) (*ReorgResult, error) {
	if err := bc.AddBlock(b); err != nil {
		return nil, err
	}
	bc.mu.RLock()
	result := bc.lastReorgResult
	bc.mu.RUnlock()
	return result, nil
}

// applyBlock 回放一个已持久化的历史区块：完整执行与 AddBlock 相同的一致性校验，
// 但绝不写回存储。仅供 NewBlockchainFromStore 启动回放使用。
func (bc *Blockchain) applyBlock(b *block.Block) error {
	return bc.addBlock(b, false)
}

// addBlock 是追加的唯一实现：validateBlock →（可选）落盘 → 原子替换 UTXO 与链尾。
// persist=false 时只重建内存状态。
// REORG-1C：已扩展为支持 fork detection + reorg execution。
func (bc *Blockchain) addBlock(b *block.Block, persist bool) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if len(bc.blocks) == 0 {
		return ErrEmptyChain
	}

	currentTip := bc.blocks[len(bc.blocks)-1]
	parentHash := b.Header.PrevBlockHash

	// ── Case 1: 直接延长当前链 ──
	if parentHash == currentTip.Header.Hash() {
		return bc.extendChain(b, persist)
	}

	// ── Case 2: Fork block（父在当前链但非链尾，或在另一条 branch 上）──
	// 损坏防护：区块哈希若已存在于 canonical 链（bc.blocks），则这是重复持久化记录
	//（旧 bug 的存储形态：同一区块被追加两次），必须显式拒绝，绝不能当作幂等
	// re-delivery 静默吞掉——否则「含重复记录的损坏存储」会被加载成一条有效链。
	//
	// 判据必须**只**查 canonical 内存链（bc.blocks），绝不能用 blockAtHash：
	// 后者在 bc.blocks 未命中时会回退 store.GetBlockByHash，而 v2 存储索引
	//（s.v2.records）同时登记 detached（非 canonical）区块——SaveBlockDetached
	// 经 registerBlock 写入。若在此处误用 blockAtHash，一枚已落盘的合法 fork
	// 区块被再次投递时会被误判成「损坏重复记录」而拒绝，既破坏 re-delivery 的
	// 幂等语义，也会让 OnBlocksResp 直接 return，导致 syncing 标志永久卡死。
	if bc.canonicalContains(b.Header.Hash()) {
		bh := b.Header.Hash()
		return fmt.Errorf("%w: 区块 %x 已存在于 canonical 链（疑似重复持久化记录）", ErrInvalidPrevHash, bh[:4])
	}
	parentNode := bc.tree.LookupNode(parentHash)
	if parentNode == nil {
		// 父不存在：orphan（P2P 到达顺序问题）。
		//
		// REORG-1H：这里必须用 ErrOrphanParent 而非裸 ErrInvalidPrevHash——
		// 上层 P2P 需要据此区分「已知父的合法分叉块」与「缺父孤块」，
		// 对后者触发 by-hash 分支拉取把祖先补齐，跨节点 reorg 才可能发生。
		// errors.Is(err, ErrInvalidPrevHash) 仍为 true，既有断言不受影响。
		return fmt.Errorf("%w: parent %x not in tree", ErrOrphanParent, parentHash[:4])
	}

	// 加入 blocktree（轻量级元数据索引）
	height := parentNode.Height + 1
	node, err := bc.tree.AddBlock(b.Header.Hash(), parentHash, height, b.Header.Bits, b.Header.Timestamp)
	if err != nil {
		if errors.Is(err, blocktree.ErrDuplicateHash) {
			return nil // 已存在，幂等
		}
		return fmt.Errorf("blocktree add failed: %w", err)
	}

	// 对 fork block 做完整共识校验（基于父分支的 UTXO）
	if err := bc.validateForkBlock(b, parentNode); err != nil {
		// 用 ErrInvalidPrevHash 包装 fork 校验失败，以保持既有测试断言兼容：
		// 旧语义下「非 tip 父哈希」一律返回 ErrInvalidPrevHash；新语义下仍拒绝，
		// 但附带真实失败原因。errors.Is(err, ErrInvalidPrevHash) 保持为 true。
		return fmt.Errorf("%w: %v", ErrInvalidPrevHash, err)
	}

	// 决定是否应 reorg
	shouldReorg, _, err := bc.tree.ShouldReorg(node)
	if err != nil {
		return err
	}

	if !shouldReorg {
		// 保存为 detached，但不切换 canonical
		//
		// P1（REORG SILENT-STATE-DIVERGENCE REMEDIATION）：保存失败不得再被静默丢弃。
		// 这里刻意 **不** 改成 return error：`tree.AddBlock` 已在上面成功（块已进入候选
		// 索引），而「fork choice 拒绝」本身是合法结果、不是错误——把它变成 error 会让
		// 调用方误判为「块非法/处理失败」。因此如实写进结构化事件，保持控制流不变。
		detachedSaveErr := ""
		if persist && bc.store != nil {
			if v2s, ok := bc.store.(reorgStore); ok {
				// 与下方 reorg 路径同判据：以「块是否在存储中就位」为准，
				// 幂等的「已存在」不算失败，避免把正常情形写成噪声。
				if err := v2s.SaveBlockDetached(b); err != nil && !v2s.HasBlock(b.Header.Hash()) {
					detachedSaveErr = err.Error()
				}
			}
		}
		// I0/§8：fork choice 拒绝 —— 新块未能赢得 canonical 竞争（合法，非缺陷）。
		obs.Emit("REORG_REJECT", "block", b.Header.HashHex(), "fork_height", height,
			"canonical_height", len(bc.blocks)-1, "reason", "chainwork_not_won",
			"detached_save_error", detachedSaveErr)
		return nil
	}

	// 触发 reorg 的块必须先持久化，否则 executeReorg 中 blockAtHash 找不到它。
	//
	// P1（REORG SILENT-STATE-DIVERGENCE REMEDIATION）：这条持久化是 executeReorg 的
	// **前置条件**（见上句注释）。失败必须显式返回，绝不能让 executeReorg 在
	// 「块不在存储里」的前提下继续跑——那正是 silent state divergence 的入口。
	// 此处任何 canonical 状态（bc.blocks / utxo / hashIndex / canonicalSet / tree tip）
	// 尚未变更，因此 return error 是干净的 fail-before-commit。
	if persist && bc.store != nil {
		if v2s, ok := bc.store.(reorgStore); ok {
			// 判据是**后置条件**（该块在存储中就位），不是「调用是否返回 nil」：
			// SaveBlockDetached 对**已存在**的哈希会返回「区块哈希已存在（重复记录被拒绝）」，
			// 那是幂等成功的正常情形，绝不能当失败处理（否则会改变既有行为）。
			// 只有「保存返回错误 **且** 块在存储中确实不存在」才意味着前置条件未满足。
			if err := v2s.SaveBlockDetached(b); err != nil && !v2s.HasBlock(b.Header.Hash()) {
				return fmt.Errorf("reorg: 触发 reorg 的区块未能在存储中就位（%s）: %w",
					b.Header.HashHex(), err)
			}
		}
	}
	// 执行 reorg
	return bc.executeReorg(node, persist)
}

// extendChain 处理直接延长当前 canonical 链的区块。
func (bc *Blockchain) extendChain(b *block.Block, persist bool) error {
	newSet, err := bc.validateBlock(b, false)
	if err != nil {
		return fmt.Errorf("添加区块失败: %w", err)
	}
	if persist && bc.store != nil {
		if v2s, ok := bc.store.(reorgStore); ok && v2s.V2Mode() {
			// v2 模式：生成 undo 并使用 AppendCanonicalBlock
			_, undo, _, err := utxo.ApplyBlockWithUndo(bc.utxo, b.Transactions, len(bc.blocks))
			if err != nil {
				return fmt.Errorf("生成 undo 失败: %w", err)
			}
			if err := v2s.AppendCanonicalBlock(b, undo); err != nil {
				return fmt.Errorf("区块持久化失败: %w", err)
			}
		} else {
			// legacy 模式或 v2 尚未激活：保持逐字节不变
			if err := bc.store.SaveBlock(b); err != nil {
				return fmt.Errorf("区块持久化失败: %w", err)
			}
		}
	}
	bc.blocks = append(bc.blocks, b)
	// C-1：canonical append 同步写入 hash index（唯一 append mutation 点，
	// 与 bc.blocks 同一临界区，维持「index ≡ bc.blocks 哈希集合」不变量）。
	if bc.hashIndex == nil {
		// 防御路径：正常构造链路（NewBlockchainWithGenesis）必已初始化；
		// 若因异常缺失则全量重建，避免部分投影。
		bc.rebuildHashIndexLocked()
	} else {
		bc.hashIndex[b.Header.Hash()] = b
	}
	// C-d：canonical append 同步写入 canonical member hash set（同一 append
	// mutation 点、同一 bc.mu 临界区，维持「set ≡ bc.blocks 哈希集合」不变量）。
	if bc.canonicalSet == nil {
		// 防御路径：同 hashIndex——正常构造链路（NewBlockchainWithGenesis）必已初始化。
		bc.rebuildCanonicalSetLocked()
	} else {
		bc.canonicalSet[b.Header.Hash()] = struct{}{}
	}
	bc.utxo = newSet
	// 同步更新 blocktree：若节点尚不在树中则先加入，再设 tip。
	// 1F 测试暴露：缺少 AddBlock 导致 BestTip 永久停留在 genesis，
	// ShouldReorg 基于错误基准触发 premature reorg。
	height := len(bc.blocks) - 1
	node := bc.tree.LookupNode(b.Header.Hash())
	if node == nil {
		_, _ = bc.tree.AddBlock(b.Header.Hash(), b.Header.PrevBlockHash, height, b.Header.Bits, b.Header.Timestamp)
		node = bc.tree.LookupNode(b.Header.Hash())
	}
	if node != nil {
		_ = bc.tree.SetTip(node)
	}
	return nil
}

// obsBlockLookups 是 I0/§9 观测计数器：blockAtHash 的累计调用次数。
// validateForkBlock 用「前后快照差值」得到单次 fork 校验的 lookup 数；
// 由于校验路径全程持有 bc.mu 写锁，差值在同锁串行下是精确的。
var obsBlockLookups atomic.Uint64

// forkView 是「以某条 fork 分支自身祖先」构造的 pow.ChainView（O-3 修复核心）。
//
// 与 unsafeView（读 canonical bc.blocks）相对：forkView 按高度返回 **fork 分支上的
// 区块**——由 parentNode.PathToRoot() 得到 fork 祖先链，再经 blockAtHash 取块
// （blockAtHash 同时登记 detached 非 canonical 区块，见其调用方须知）。
//
// 为什么必须如此（O-3）：fork 块的 bits 与 MTP 必须基于 **fork 自身历史**计算；
// 若沿用 canonical 视图，则跨 ruleset 边界（2000 / 3000）且 fork 深度 ≥ 一个难度周期
// （20 块）时，会把 fork 自身的 AdjustBits/AdjustBitsNearest 结果与 canonical 期望值
// 错误比较 ⇒ 误拒合法 fork（或误收非法 bits）。这正是 pow.ChainView 抽象与
// DESIGN-1 §FC-004 的设计意图。
//
// 惰性求值：BlockByHeight 经 AncestorAtHeight 定位节点后再取块，只读取实际需要的高度
// （bits 至多 2 个高度、MTP 至多 11 个），避免一次性扫描整条路径。
type forkView struct {
	bc   *Blockchain
	node *blocktree.BlockNode // fork 分支末端（= 待校验块的父节点）
}

func (v forkView) BlockByHeight(h int) (*block.Block, error) {
	if h < 0 || h > v.node.Height {
		return nil, fmt.Errorf("%w: %d（fork 视图高度 %d）", ErrUnknownHeight, h, v.node.Height)
	}
	a := v.node.AncestorAtHeight(h)
	if a == nil {
		return nil, fmt.Errorf("%w: %d（fork 视图无该祖先）", ErrUnknownHeight, h)
	}
	return v.bc.blockAtHash(a.Hash)
}

func (v forkView) Height() int {
	return v.node.Height
}

// validateForkBlock 对一条 fork branch 上的区块执行共识校验。
// 需要重建父节点处的 UTXO 状态（replay from genesis）。
//
// I0/§9：本函数只包了一层观测（START/END + 计时 + lookup 增量），
// 校验逻辑在 validateForkBlockInner 中逐字节保持不变。禁止在此路径引入
// Header.Hash() 记忆化 / 共识缓存 / 增量 UTXO —— 只观测，不修复。
func (bc *Blockchain) validateForkBlock(b *block.Block, parentNode *blocktree.BlockNode) error {
	start := time.Now()
	lookups0 := obsBlockLookups.Load()
	pathLen := len(parentNode.PathToRoot())
	forkHeight := parentNode.Height + 1
	obs.Emit("FORK_VALIDATION_START", "fork_height", forkHeight,
		"canonical_height", len(bc.blocks)-1, "path_length", pathLen)
	err := bc.validateForkBlockInner(b, parentNode)
	result := "ok"
	if err != nil {
		result = "rejected"
	}
	obs.Emit("FORK_VALIDATION_END", "fork_height", forkHeight,
		"canonical_height", len(bc.blocks)-1, "path_length", pathLen,
		"block_lookup_count", obsBlockLookups.Load()-lookups0,
		"duration_us", time.Since(start).Microseconds(), "result", result)
	obs.Inc("fork_validation_total")
	if err != nil {
		obs.Inc("fork_validation_rejected")
	}
	return err
}

// validateForkBlockInner 是 validateForkBlock 的原始实现体（I0 拆分，仅观测包装变更）。
func (bc *Blockchain) validateForkBlockInner(b *block.Block, parentNode *blocktree.BlockNode) error {
	baseUTXO, err := bc.utxoAtNode(parentNode)
	if err != nil {
		return fmt.Errorf("rebuild parent UTXO failed: %w", err)
	}
	parentBlock, err := bc.blockAtHash(parentNode.Hash)
	if err != nil {
		return fmt.Errorf("get parent block failed: %w", err)
	}
	height := parentNode.Height + 1

	// 1. 链式结构
	if b.Header.PrevBlockHash != parentBlock.Header.Hash() {
		return ErrInvalidPrevHash
	}
	// 2. 版本号
	if err := bc.validateVersion(b, height); err != nil {
		return err
	}
	// 3. PoW
	if !pow.Validate(&b.Header) {
		return ErrInvalidPoW
	}
	// O-3 修复：fork 块的 bits 与 MTP 必须基于 **fork 自身祖先视图**计算，
	// 而非 canonical bc.blocks（否则跨 ruleset 边界 2000/3000 的 reorg 会误判）。
	view := forkView{bc: bc, node: parentNode}
	// 4. 难度（基于 fork 自身祖先视图；canonical 视图会误拒合法 fork）
	if err := bc.validateBitsWithView(view, b, height); err != nil {
		return err
	}
	// 5. 时间戳（同一 fork 视图提供 MTP；与步 4 同属 O-3 修复范围）
	if err := bc.validateTimestampWithView(view, b, parentBlock, height); err != nil {
		return err
	}
	// 6. Merkle
	if block.ComputeMerkleRoot(b.Transactions) != b.Header.MerkleRoot {
		return ErrMerkleMismatch
	}
	// 7. 体积
	if size := b.Size(); size > MaxBlockSize {
		return fmt.Errorf("%w: %d > %d", ErrBlockTooLarge, size, MaxBlockSize)
	}
	// 8. 交易层
	_, _, err = utxo.ApplyBlock(baseUTXO, b.Transactions, height)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadTxLayout, unwrapLayoutErr(err))
	}
	return nil
}

// utxoAtNode 通过从创世 replay 到 node，返回 node 处的 UTXO 状态。
func (bc *Blockchain) utxoAtNode(node *blocktree.BlockNode) (*utxo.UTXOSet, error) {
	path := node.PathToRoot()
	// 反转为 genesis → node
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	if len(path) == 0 {
		return nil, errors.New("empty path")
	}
	genesisBlock, err := bc.blockAtHash(path[0].Hash)
	if err != nil {
		return nil, fmt.Errorf("genesis: %w", err)
	}
	set, _, err := utxo.ApplyBlock(utxo.NewUTXOSet(), genesisBlock.Transactions, 0)
	if err != nil {
		return nil, fmt.Errorf("genesis apply: %w", err)
	}
	for i := 1; i < len(path); i++ {
		b, err := bc.blockAtHash(path[i].Hash)
		if err != nil {
			return nil, fmt.Errorf("height %d: %w", i, err)
		}
		newSet, _, err := utxo.ApplyBlock(set, b.Transactions, i)
		if err != nil {
			return nil, fmt.Errorf("height %d apply: %w", i, err)
		}
		set = newSet
	}
	return set, nil
}

// blockAtHash 按哈希查找区块：先查内存 hash index（C-1），再回退到 storage。
//
// ⚠️ 调用方须知：本函数**不是** canonical 成员判据。hash index 是 bc.blocks
// 的派生投影，storage 侧索引（v2 的 s.v2.records）同时登记 detached（非
// canonical）区块，故「查得到」不等于「在 canonical 链上」。需要 canonical
// 判据时必须用 canonicalContains。
//
// C-1（AUTH-2-C1）语义保持说明：
//   - lookup 计数（obsBlockLookups.Add(1)）位置与语义逐字节不变（I0 KPI 连续性）；
//   - index 与 bc.blocks 在同一 bc.mu 临界区内维护（extendChain append /
//     executeReorg 重建两个 mutation 点），key 集合 ≡ bc.blocks 哈希集合，
//     故「index 命中」与原线性扫描命中返回完全相同的 *block.Block 指针；
//   - index miss 时不再做 O(h) 线性扫描，直接走既有 store fallback：canonical
//     块均已在 store 登记（v2 AppendCanonicalBlock / legacy SaveBlock / recovery
//     回放源自 store），store 未命中时返回 storage.ErrNotFound —— 与原实现的
//     线性扫描 miss 后回退 store 的最终结果完全一致，仅省去必败扫描。
func (bc *Blockchain) blockAtHash(hash [32]byte) (*block.Block, error) {
	obsBlockLookups.Add(1) // I0/§9：lookup 计数（纯观测，不改查找语义）
	if b, ok := bc.hashIndex[hash]; ok {
		return b, nil
	}
	if bc.store != nil {
		return bc.store.GetBlockByHash(hash)
	}
	return nil, storage.ErrNotFound
}

// canonicalContains 判定区块哈希是否已在 canonical 链（bc.blocks）上。
// 只查内存 canonical 链，绝不回退 storage —— 见 blockAtHash 的调用方须知。
//
// C-d（AUTH-2-Cd）语义保持说明：
//   - membership 判定由 canonical member hash set（bc.canonicalSet）承担，
//     复杂度由 O(h) 线性扫描降为平均 O(1)；
//   - set 与 bc.blocks 在同一 bc.mu 临界区内维护（extendChain 的 append 同步插入、
//     executeReorg 的整链重建两个 mutation 点），不变量为
//     set ≡ { b.Header.Hash() : b ∈ bc.blocks }（双向集合相等），故「set 命中」
//     与原线性扫描命中返回完全相同的 bool 结果；
//   - set miss ≡ 原线性扫描 miss：均返回 false，且都不回退 storage；
//   - 锁语义不变：本函数自身不加锁（与改写前一致），写路径 addBlock 持写锁、
//     读路径 IsCanonicalHash 持读锁。
func (bc *Blockchain) canonicalContains(hash [32]byte) bool {
	_, ok := bc.canonicalSet[hash]
	return ok
}

// executeReorg 执行完整的链重组：disconnect old → apply new → persist → update memory。
//
// I0/§8：ATTEMPT 在入口、ACCEPT 在成功尾部、FAILED 在错误返回（defer 判定）。
// 目的不是改变 reorg，而是事后能回答「孤儿是否来自正常 canonical 链替换」。
func (bc *Blockchain) executeReorg(newTip *blocktree.BlockNode, persist bool) error {
	start := time.Now()
	oldTip := bc.tree.BestTip()
	if oldTip == nil {
		return errors.New("no active tip")
	}
	// P1：与 blocktree.SetTip 的 ErrNilTip 前置条件对齐，消除提交尾部的 nil 解引用面。
	if newTip == nil {
		return errors.New("reorg: nil new tip")
	}
	obs.Emit("REORG_ATTEMPT", "old_tip", obsHashHex(oldTip.Hash), "old_height", oldTip.Height,
		"new_tip", obsHashHex(newTip.Hash), "new_height", newTip.Height)
	defer func() {
		obs.Emit("REORG_DURATION_US", "old_tip", obsHashHex(oldTip.Hash), "new_tip", obsHashHex(newTip.Hash),
			"duration_us", time.Since(start).Microseconds())
	}()

	ancestor := bc.tree.FindCommonAncestor(oldTip, newTip)
	if ancestor == nil {
		return errors.New("no common ancestor found")
	}

	// disconnect path: oldTip → ... → ancestor's child (reverse order)
	disconnectPath := make([]*blocktree.BlockNode, 0)
	for cur := oldTip; cur != nil && cur.Hash != ancestor.Hash; cur = cur.Parent {
		disconnectPath = append(disconnectPath, cur)
	}

	// connect path: ancestor's child → ... → newTip
	connectPath := make([]*blocktree.BlockNode, 0)
	for cur := newTip; cur != nil && cur.Hash != ancestor.Hash; cur = cur.Parent {
		connectPath = append(connectPath, cur)
	}
	for i, j := 0, len(connectPath)-1; i < j; i, j = i+1, j-1 {
		connectPath[i], connectPath[j] = connectPath[j], connectPath[i]
	}

	// 从 common ancestor replay 得到起始 UTXO
	utxoSet, err := bc.utxoAtNode(ancestor)
	if err != nil {
		return fmt.Errorf("reorg: ancestor UTXO rebuild failed: %w", err)
	}

	// 若 ancestor == oldTip（即 newTip 是 oldTip 的后代），disconnectPath 为空
	// 否则需要 disconnect old branch；但当前实现选择「从 ancestor 全量 replay」，
	// 不依赖旧 UTXO 的增量回滚。这简化了实现且对开发者节点规模可接受。
	_ = disconnectPath

	// Apply new branch
	newBlocks := make([]*block.Block, 0, len(connectPath))
	newUndos := make([]utxo.BlockUndo, 0, len(connectPath))
	for _, node := range connectPath {
		b, err := bc.blockAtHash(node.Hash)
		if err != nil {
			return fmt.Errorf("reorg: get block %x: %w", node.Hash[:4], err)
		}
		newSet, undo, _, err := utxo.ApplyBlockWithUndo(utxoSet, b.Transactions, node.Height)
		if err != nil {
			return fmt.Errorf("reorg: apply block %x: %w", node.Hash[:4], err)
		}
		utxoSet = newSet
		newBlocks = append(newBlocks, b)
		newUndos = append(newUndos, undo)
	}

	// ── Phase A：提交前完整解析与校验（P1 fail-before-commit）──────────────────
	//
	// 在触碰任何 canonical 状态之前，把所有必需的 path 区块与 tree tip 前置条件
	// 一次性解析并校验。任何一项不满足立即 return error —— 此时
	// bc.blocks / bc.utxo / bc.hashIndex / bc.canonicalSet / tree tip 全部未被修改。
	//
	// 修复前的行为：ancestorPath / disconnectPath 的区块在**状态已提交之后**才取，
	// 且 `blockAtHash` 的错误被 `_` 丢弃、取不到就静默跳过 ⇒ 装上一条被截断的
	// canonical 链，而 bc.utxo 仍是完整集合 ⇒ bc.blocks / bc.utxo / tree 三方静默分歧。
	ancestorPath := ancestor.PathToRoot()
	for i, j := 0, len(ancestorPath)-1; i < j; i, j = i+1, j-1 {
		ancestorPath[i], ancestorPath[j] = ancestorPath[j], ancestorPath[i]
	}
	ancestorBlocks := make([]*block.Block, 0, len(ancestorPath))
	for _, n := range ancestorPath {
		b, err := bc.blockAtHash(n.Hash)
		if err != nil {
			return fmt.Errorf("reorg: get ancestor block %x: %w", n.Hash[:4], err)
		}
		if b == nil {
			return fmt.Errorf("reorg: ancestor block %x missing (nil without error)", n.Hash[:4])
		}
		ancestorBlocks = append(ancestorBlocks, b)
	}

	// disconnect 分支区块（供 ReorgResult 使用）：与上面同理，提前到提交前解析，
	// 避免在状态已提交之后失败时静默产出残缺的 ReorgResult。
	disconnectBlocks := make([]*block.Block, 0, len(disconnectPath))
	for _, n := range disconnectPath {
		b, err := bc.blockAtHash(n.Hash)
		if err != nil {
			return fmt.Errorf("reorg: get disconnect block %x: %w", n.Hash[:4], err)
		}
		if b == nil {
			return fmt.Errorf("reorg: disconnect block %x missing (nil without error)", n.Hash[:4])
		}
		disconnectBlocks = append(disconnectBlocks, b)
	}

	// tree.SetTip 的前置条件（与 blocktree.SetTip 内部校验逐条一致），同样提前。
	// nil 与「不在树中」是 SetTip 仅有的两种失败原因；两者在此已排除 ⇒ Phase B
	// 中的 SetTip 不会失败，故不存在「tip 未推进但状态已提交」的窗口。
	if bc.tree.LookupNode(newTip.Hash) != newTip {
		return fmt.Errorf("reorg: new tip %x… is not in the block tree", newTip.Hash[:4])
	}

	// Persist
	if persist && bc.store != nil {
		if v2s, ok := bc.store.(reorgStore); ok {
			// detached undos = 已存储但缺 undo 的区块
			detachedUndos := make(map[[32]byte]utxo.BlockUndo)
			for i, node := range connectPath {
				if v2s.HasBlock(node.Hash) && !v2s.HasUndo(node.Hash) {
					detachedUndos[node.Hash] = newUndos[i]
				}
			}
			// actual new blocks = 尚未存储的区块
			var actualNewBlocks []*block.Block
			var actualNewUndos []utxo.BlockUndo
			for i, b := range newBlocks {
				if !v2s.HasBlock(b.Header.Hash()) {
					actualNewBlocks = append(actualNewBlocks, b)
					actualNewUndos = append(actualNewUndos, newUndos[i])
				}
			}
			if err := v2s.CommitReorg(detachedUndos, actualNewBlocks, actualNewUndos, newTip.Hash); err != nil {
				return fmt.Errorf("reorg: persist failed: %w", err)
			}
		}
	}

	// Update memory canonical state
	//
	// Phase B：全部输入已在 Phase A 解析并校验完成，这里只做不可失败的赋值，
	// 不再二次查找、不再存在「取不到就跳过」的静默路径。
	newChain := make([]*block.Block, 0, len(ancestorBlocks)+len(newBlocks))
	newChain = append(newChain, ancestorBlocks...)
	newChain = append(newChain, newBlocks...)

	bc.blocks = newChain
	// C-1：整链替换后全量重建 hash index（disconnect 旧链块随之移出投影，
	// connect 新链块随之进入），与 bc.blocks 同一临界区。
	bc.rebuildHashIndexLocked()
	// C-d：整链替换后全量重建 canonical member hash set（同上语义：
	// disconnect 旧链块移出投影、connect 新链块进入投影）。
	bc.rebuildCanonicalSetLocked()
	bc.utxo = utxoSet
	// P1：SetTip 的前置条件已在 Phase A 校验（newTip 非 nil 且确在树中），故此处
	// 不会失败；但错误依然不允许被丢弃——若该不变量被破坏，必须显式暴露，而不是
	// 留下「bc.blocks / bc.utxo 已推进而 tree tip 未推进」的分歧。
	if err := bc.tree.SetTip(newTip); err != nil {
		return fmt.Errorf("reorg: set tip %x…: %w", newTip.Hash[:4], err)
	}

	// Build ReorgResult for service-layer mempool resurrection (REORG-1F).
	// Phase A 已解析并校验 disconnectPath / connectPath 的区块，这里直接使用，
	// 不再二次查找（此前二次查找失败会被静默丢弃，产出残缺的 ReorgResult）。
	connectBlocks := newBlocks
	bc.lastReorgResult = &ReorgResult{
		OldTip:           oldTip,
		NewTip:           newTip,
		DisconnectBlocks: disconnectBlocks,
		ConnectBlocks:    connectBlocks,
	}
	// I0/§8：REORG_ACCEPT —— canonical 链替换完成（detached/attached 为分支规模）。
	obs.Emit("REORG_ACCEPT", "old_tip", obsHashHex(oldTip.Hash), "old_height", oldTip.Height,
		"new_tip", obsHashHex(newTip.Hash), "new_height", newTip.Height,
		"detached_count", len(disconnectPath), "attached_count", len(connectPath))
	obs.Inc("reorg_accepted")
	return nil
}
