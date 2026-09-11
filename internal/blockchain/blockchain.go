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
	ErrInvalidPrevHash   = errors.New("区块的前置哈希与当前链尾不匹配")
	ErrInvalidPoW        = errors.New("区块哈希未达到难度目标，工作量证明无效")
	ErrEmptyChain        = errors.New("链为空")
	ErrUnknownHeight     = errors.New("请求的区块高度不存在")
	ErrUnexpectedBits    = errors.New("区块难度位与当前共识难度不一致")
	ErrTimestampOutOfRange = errors.New("区块时间戳超出允许范围")
	ErrMerkleMismatch    = errors.New("区块头 Merkle 根与交易列表不匹配")
	ErrBlockTooLarge     = errors.New("区块超过最大体积限制")
	ErrBadTxLayout       = errors.New("区块交易布局非法（coinbase 位置/数量）")
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
// store 非空时，每次成功追加都会同步落盘，启动时从磁盘重建。
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
		if err := bc.AddBlock(b); err != nil {
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
func (bc *Blockchain) validateBlock(b *block.Block) (*utxo.UTXOSet, error) {
	tip := bc.blocks[len(bc.blocks)-1]
	height := len(bc.blocks) // 新区块高度

	// 1. 链式结构
	if b.Header.PrevBlockHash != tip.Header.Hash() {
		return nil, ErrInvalidPrevHash
	}
	// 2. 工作量证明
	if !pow.Validate(&b.Header) {
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
	if _, err := bc.validateBlock(b); err != nil {
		return err
	}
	return nil
}

// AddBlock 校验并把区块原子地追加到链尾（校验 + UTXO 整体替换 + 追加 + 落盘）。
func (bc *Blockchain) AddBlock(b *block.Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	newSet, err := bc.validateBlock(b)
	if err != nil {
		return fmt.Errorf("添加区块失败: %w", err)
	}
	// 先落盘再更新内存状态：落盘失败则内存状态不变，保证两者一致
	if bc.store != nil {
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
