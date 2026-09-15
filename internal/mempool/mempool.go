// Package mempool 实现内存交易池：暂存已通过校验、等待被打包的交易。
//
// 关键设计：组合视图（composite view）
// 校验池中新交易时，不能只看链上 UTXO——还要把池内待打包交易的输出与花费计入，
// 否则「同一输出被池内两笔交易同时花费」与「链式交易（子交易花费父交易输出）」
// 都无法正确判定。本包通过 UTXO 克隆 + 按序应用池内交易构造组合视图。
//
// 线程安全：全部方法受 RWMutex 保护。
package mempool

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"p2pchain/internal/block"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
)

var (
	ErrKnownTx    = errors.New("交易已在内存池中")
	ErrConflict   = errors.New("交易与内存池中的交易冲突（输入已被占用）")
	ErrPoolFull   = errors.New("内存池已满")
	ErrCoinbaseIn = errors.New("coinbase 交易不能进入内存池")
	ErrInvalidTx  = errors.New("交易未通过校验")
)

// Mempool 内存交易池。
type Mempool struct {
	mu      sync.RWMutex
	maxSize int
	txs     map[[32]byte]*transaction.Transaction
	fees    map[[32]byte]uint64
	order   [][32]byte // 插入顺序（FIFO），用于打包时的稳定决定性
}

// New 创建内存池；maxSize ≤ 0 表示不限制。
func New(maxSize int) *Mempool {
	return &Mempool{
		maxSize: maxSize,
		txs:     make(map[[32]byte]*transaction.Transaction),
		fees:    make(map[[32]byte]uint64),
	}
}

// Len 返回池中交易数量。
func (m *Mempool) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.txs)
}

// Has 判断交易是否已在池中。
func (m *Mempool) Has(txid [32]byte) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.txs[txid]
	return ok
}

// Fee 返回池中某交易的手续费。
func (m *Mempool) Fee(txid [32]byte) (uint64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, ok := m.fees[txid]
	return f, ok
}

// compositeView 构造「链上 UTXO + 池内待打包交易」的组合视图。
func (m *Mempool) compositeView(base *utxo.UTXOSet, height int, skip [32]byte) *utxo.UTXOSet {
	view := base.Clone()
	for _, id := range m.order {
		if id == skip {
			continue
		}
		tx, ok := m.txs[id]
		if !ok {
			continue
		}
		// 池内交易应始终合法；若因链状态推进而失效，跳过（不阻断新交易校验）
		if _, err := utxo.ValidateTransaction(tx, view, height); err != nil {
			continue
		}
	}
	return view
}

// Add 校验并把交易加入内存池。
//
// 校验视图 = base 的克隆 + 池内全部待打包交易（按插入序应用）。
// 依赖此视图可同时覆盖：
//   - 双花（同一 UTXO 被池内交易占用 → ErrConflict）
//   - 链式交易（花费池内交易的输出 → 正常通过）
func (m *Mempool) Add(base *utxo.UTXOSet, tx *transaction.Transaction, height int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tx.IsCoinbase() {
		return ErrCoinbaseIn
	}
	id := tx.Hash()
	if _, dup := m.txs[id]; dup {
		return fmt.Errorf("%w: %x", ErrKnownTx, id)
	}
	if m.maxSize > 0 && len(m.txs) >= m.maxSize {
		return ErrPoolFull
	}

	view := m.compositeView(base, height, id)
	fee, err := utxo.ValidateTransaction(tx, view, height)
	if err != nil {
		// 输入被池内其他交易占用 → 明确归类为冲突
		if errors.Is(err, utxo.ErrUnknownUTXO) {
			return fmt.Errorf("%w: %v", ErrConflict, err)
		}
		return fmt.Errorf("%w: %v", ErrInvalidTx, err)
	}

	m.txs[id] = tx
	m.fees[id] = fee
	m.order = append(m.order, id)
	return nil
}

// Remove 从池中移除指定交易。
func (m *Mempool) Remove(txid [32]byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeLocked(txid)
}

func (m *Mempool) removeLocked(txid [32]byte) {
	delete(m.txs, txid)
	delete(m.fees, txid)
	for i, id := range m.order {
		if id == txid {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
}

// RemoveIncluded 在区块上链后更新内存池：
//   - 移除已被该区块打包的交易；
//   - 对剩余交易在新链状态上重验，剔除已失效者（输入被占用/UTXO 消失）。
func (m *Mempool) RemoveIncluded(b *block.Block, newBase *utxo.UTXOSet, height int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, tx := range b.Transactions {
		m.removeLocked(tx.Hash())
	}

	// 重验剩余交易：按插入序在新链状态上重建组合视图
	kept := make(map[[32]byte]*transaction.Transaction, len(m.txs))
	var keptFees = make(map[[32]byte]uint64, len(m.txs))
	var keptOrder [][32]byte
	view := newBase.Clone()
	for _, id := range m.order {
		tx, ok := m.txs[id]
		if !ok {
			continue
		}
		fee, err := utxo.ValidateTransaction(tx, view, height)
		if err != nil {
			continue // 失效交易被剔除
		}
		kept[id] = tx
		keptFees[id] = fee
		keptOrder = append(keptOrder, id)
	}
	m.txs = kept
	m.fees = keptFees
	m.order = keptOrder
}

// Pending 返回按手续费降序（同费按插入序）排列的待打包交易，最多 max 笔。
func (m *Mempool) Pending(max int) []*transaction.Transaction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([][32]byte, 0, len(m.order))
	ids = append(ids, m.order...)
	sort.SliceStable(ids, func(i, j int) bool {
		return m.fees[ids[i]] > m.fees[ids[j]]
	})
	if max > 0 && len(ids) > max {
		ids = ids[:max]
	}
	out := make([]*transaction.Transaction, 0, len(ids))
	for _, id := range ids {
		out = append(out, m.txs[id])
	}
	return out
}

// TotalFees 返回给定交易集合的手续费总和（用于构造 coinbase 金额）。
func (m *Mempool) TotalFees(txs []*transaction.Transaction) uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var total uint64
	for _, tx := range txs {
		total += m.fees[tx.Hash()]
	}
	return total
}

// All 返回池中全部交易（用于中继广播）。
func (m *Mempool) All() []*transaction.Transaction {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*transaction.Transaction, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.txs[id])
	}
	return out
}

// ReaddDisconnected 在链重组后将断开区块中的非 coinbase 交易重新加入内存池。
//
// 参数：
//   - disconnectBlocks：被断开分支上的区块列表（从 old tip 到 ancestor）。
//   - newBase：新 canonical 链的 UTXO 状态。
//   - height：新链当前高度（用于 maturity 等校验）。
//   - newChainTxs：新 canonical 链中已确认的交易哈希集合（用于去重）。
//
// 返回 (accepted, rejected) 计数。
//
// 算法：
//  1. 扫描断开区块，收集非 coinbase 候选（跳过已在新链确认的）。
//  2. 多轮尝试 mempool.Add，处理 parent→child 依赖链。
//  3. 每轮将成功入池的交易移出候选集；无进展时终止。
//
// 约束：
//   - 不修改共识状态；失败静默丢弃。
//   - 遵守 pool 容量限制（maxSize）。
//   - 已存在于 pool 中的交易跳过（幂等）。
func (m *Mempool) ReaddDisconnected(disconnectBlocks []*block.Block, newBase *utxo.UTXOSet, height int, newChainTxs map[[32]byte]struct{}) (accepted, rejected int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Phase 1: 收集候选（确定性顺序：disconnectBlocks 顺序 × tx 索引顺序）
	candidates := make([]*transaction.Transaction, 0)
	seen := make(map[[32]byte]struct{})
	for _, b := range disconnectBlocks {
		for i, tx := range b.Transactions {
			if i == 0 {
				continue // coinbase 永不复活
			}
			id := tx.Hash()
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			if _, confirmed := newChainTxs[id]; confirmed {
				continue
			}
			candidates = append(candidates, tx)
		}
	}

	// Phase 2: 多轮入池
	remaining := candidates
	for len(remaining) > 0 {
		progress := false
		nextRemaining := make([]*transaction.Transaction, 0)
		for _, tx := range remaining {
			id := tx.Hash()
			if _, inPool := m.txs[id]; inPool {
				// 已存在于 pool（前序轮次或 reorg 前遗留）
				continue
			}
			fee, err := m.addLocked(newBase, tx, height, id)
			if err != nil {
				nextRemaining = append(nextRemaining, tx)
				continue
			}
			m.txs[id] = tx
			m.fees[id] = fee
			m.order = append(m.order, id)
			accepted++
			progress = true
		}
		if !progress {
			rejected += len(nextRemaining)
			break
		}
		remaining = nextRemaining
	}

	return accepted, rejected
}

// addLocked 是 Add 的核心校验逻辑（无锁版本），供 ReaddDisconnected 内部复用。
// 返回值：fee（成功）或 error（失败）。
func (m *Mempool) addLocked(base *utxo.UTXOSet, tx *transaction.Transaction, height int, id [32]byte) (uint64, error) {
	if tx.IsCoinbase() {
		return 0, ErrCoinbaseIn
	}
	if m.maxSize > 0 && len(m.txs) >= m.maxSize {
		return 0, ErrPoolFull
	}
	view := m.compositeViewLocked(base, height, id)
	fee, err := utxo.ValidateTransaction(tx, view, height)
	if err != nil {
		if errors.Is(err, utxo.ErrUnknownUTXO) {
			return 0, fmt.Errorf("%w: %v", ErrConflict, err)
		}
		return 0, fmt.Errorf("%w: %v", ErrInvalidTx, err)
	}
	return fee, nil
}

// compositeViewLocked 是 compositeView 的无锁版本，供已持有 m.mu 的调用方使用。
func (m *Mempool) compositeViewLocked(base *utxo.UTXOSet, height int, skip [32]byte) *utxo.UTXOSet {
	view := base.Clone()
	for _, id := range m.order {
		if id == skip {
			continue
		}
		tx, ok := m.txs[id]
		if !ok {
			continue
		}
		if _, err := utxo.ValidateTransaction(tx, view, height); err != nil {
			continue
		}
	}
	return view
}
