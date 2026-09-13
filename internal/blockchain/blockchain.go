// Package blockchain 维护本地节点看到的区块链视图，负责：
//   - 区块的验证与追加（全序共识校验，见 ValidateBlock）
//   - UTXO 状态机的持有与原子迁移（委托 internal/utxo）
//   - 难度调整的触发（委托 internal/pow）
//
// 线程模型：全链操作受 mu 保护；UTXO 集合自身亦有内部锁，
// 对外暴露的 UTXOSnapshot() 返回克隆快照，避免调用方直接持有可变状态。
//
// 分叉处理（reorg）仍为单链追加实现，设计要点见文末 TODO 注释。
package blockchain

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
	"p2pchain/internal/utxo"
)

var (
	ErrInvalidPrevHash     = errors.New("区块的前置哈希与当前链尾不匹配")
	ErrInvalidPoW          = errors.New("区块哈希未达到难度目标，工作量证明无效")
	ErrEmptyChain          = errors.New("链为空")
	ErrUnknownHeight       = errors.New("请求的区块高度不存在")
	ErrUnexpectedBits      = errors.New("区块难度位与当前共识难度不一致")
	ErrTimestampOutOfRange = errors.New("区块时间戳超出允许范围")
	ErrMerkleMismatch      = errors.New("区块头 Merkle 根与交易列表不匹配")
	ErrBlockTooLarge       = errors.New("区块超过最大体积限制")
	ErrBadTxLayout         = errors.New("区块交易布局非法（coinbase 位置/数量）")
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
}

// NewBlockchainWithGenesis 使用给定的创世区块初始化链（不持久化，供测试/临时链使用）。
// 创世区块视为可信（不做头校验），但其交易必须能成功建立初始 UTXO 状态。
func NewBlockchainWithGenesis(genesis *block.Block) (*Blockchain, error) {
	genesisSet, _, err := utxo.ApplyBlock(utxo.NewUTXOSet(), genesis.Transactions, 0)
	if err != nil {
		return nil, fmt.Errorf("创世区块 UTXO 初始化失败: %w", err)
	}
	return &Blockchain{
		blocks: []*block.Block{genesis},
		utxo:   genesisSet,
	}, nil
}

// NewBlockchainFromStore 从持久化存储加载链；空库时创建确定性创世并落盘。
//
// 启动回放策略（证据优先）：逐块按高度重新执行完整共识校验（AddBlock），
// 任何一块不合法即拒绝启动——避免带着损坏数据继续运行。
func NewBlockchainFromStore(store storage.BlockStore) (*Blockchain, error) {
	h, err := store.Height()
	if err != nil {
		return nil, fmt.Errorf("读取存储高度失败: %w", err)
	}

	if h < 0 {
		genesis := NewGenesisBlock()
		if err := store.SaveBlock(genesis); err != nil {
			return nil, fmt.Errorf("创世区块落盘失败: %w", err)
		}
		bc, err := NewBlockchainWithGenesis(genesis)
		if err != nil {
			return nil, err
		}
		bc.store = store
		return bc, nil
	}

	first, err := store.GetBlockByHeight(0)
	if err != nil {
		return nil, fmt.Errorf("读取创世区块失败: %w", err)
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
	return bc.currentBitsLocked()
}

func (bc *Blockchain) currentBitsLocked() uint32 {
	if len(bc.blocks) == 0 {
		return pow.MaxTargetBits
	}
	tip := bc.blocks[len(bc.blocks)-1]
	height := len(bc.blocks) - 1
	if height == 0 || height%pow.DifficultyAdjustmentInterval != 0 {
		return tip.Header.Bits
	}

	periodStartHeight := height - pow.DifficultyAdjustmentInterval
	if periodStartHeight < 0 {
		periodStartHeight = 0
	}
	periodStart := bc.blocks[periodStartHeight]
	actualTimespan := tip.Header.Timestamp - periodStart.Header.Timestamp
	return pow.AdjustBits(tip.Header.Bits, actualTimespan)
}

// validateBlock 对区块执行全序共识校验；全部通过时返回应用后的新 UTXO 集合。
// 调用方必须已持有锁（AddBlock 写锁 / ValidateBlock 读锁）。
// 校验顺序（任何一步失败立即拒绝）：
//  1. PrevHash        2. PoW           3. Bits == 共识难度
//  4. 时间戳范围      5. Merkle 重验   6. 体积上限
//  7. coinbase 位置/数量 + 全部交易的状态迁移（签名/双花/maturity/金额/coinbase 上限）
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
	// 2. 工作量证明（skipPoW 时跳过 —— 见函数注释的安全边界）
	if !skipPoW && !pow.Validate(&b.Header) {
		return nil, ErrInvalidPoW
	}
	// 3. 难度位必须与当前共识难度一致（防止矿工私降难度）
	if b.Header.Bits != bc.currentBitsLocked() {
		return nil, fmt.Errorf("%w: 区块 %d，共识 %d", ErrUnexpectedBits, b.Header.Bits, bc.currentBitsLocked())
	}
	// 4. 时间戳：不得早于父块（保证难度调整的时间跨度单调），不得大幅超前
	if b.Header.Timestamp < tip.Header.Timestamp {
		return nil, fmt.Errorf("%w: 时间戳 %d 早于父块 %d", ErrTimestampOutOfRange, b.Header.Timestamp, tip.Header.Timestamp)
	}
	if b.Header.Timestamp > time.Now().Unix()+maxFutureTimestampDrift {
		return nil, fmt.Errorf("%w: 时间戳 %d 超前本地时钟超过 %d 秒", ErrTimestampOutOfRange, b.Header.Timestamp, maxFutureTimestampDrift)
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
	return bc.addBlock(b, true)
}

// applyBlock 回放一个已持久化的历史区块：完整执行与 AddBlock 相同的一致性校验，
// 但绝不写回存储。仅供 NewBlockchainFromStore 启动回放使用。
func (bc *Blockchain) applyBlock(b *block.Block) error {
	return bc.addBlock(b, false)
}

// addBlock 是追加的唯一实现：validateBlock →（可选）落盘 → 原子替换 UTXO 与链尾。
// persist=false 时只重建内存状态。
func (bc *Blockchain) addBlock(b *block.Block, persist bool) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	newSet, err := bc.validateBlock(b, false)
	if err != nil {
		return fmt.Errorf("添加区块失败: %w", err)
	}
	// 先落盘再更新内存状态：落盘失败则内存状态不变，保证两者一致
	if persist && bc.store != nil {
		if err := bc.store.SaveBlock(b); err != nil {
			return fmt.Errorf("区块持久化失败: %w", err)
		}
	}
	bc.blocks = append(bc.blocks, b)
	bc.utxo = newSet
	return nil
}

// TODO 分叉处理（reorg，设计要点，当前为单链追加实现）：
//
// 真实网络中，不同节点可能几乎同时挖出不同的区块，形成临时分叉。
// 处理原则（最长有效链 / 最大累积工作量）：
//  1. 收到新区块时若 PrevBlockHash 不指向当前链尾，判断其所在链的累积工作量；
//  2. 更高则执行重组：回滚链尾区块（需要按高度逆序反向应用 UTXO——
//     因此持久化层保存每个 UTXO 条目的产生高度与消费记录），
//     切换到工作量更大的链，被回滚交易重新入池；
//  3. 需要把 blocks []*Block 升级为 map[哈希]BlockNode 的树状索引。
//  本项目保留单链实现 + 上述设计说明，作为下一阶段工作项。
