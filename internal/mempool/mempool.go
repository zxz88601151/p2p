// Package mempool 实现内存交易池：暂存已通过校验、等待被打包的交易。
//
// 关键设计：共享校验视图（shared validation view）
// 校验池中新交易时，不能只看链上 UTXO——还要把池内待打包交易的输出与花费计入，
// 否则「同一输出被池内两笔交易同时花费」与「链式交易（子交易花费父交易输出）」
// 都无法正确判定。本池持有 view = base 克隆 + 池内交易按插入序应用的共享视图，
// 每笔新交易的校验 O(输入数)，与池大小无关（R4）。
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
//
// R4（SEC-CLOSE MUST FIX 5）校验视图模型：池持有一个**共享校验视图**
// （view = base 克隆 + 池内交易按插入序应用），Add 的完整校验直接在 view 上
// 进行——O(输入数)，与池大小无关（旧实现每次 Add 重建组合视图 = O(池大小)，
// 批量入池 O(N²)，N=2000 实测 ~286ms）。
//
// 一致性依赖两条不变量：
//  1. utxo.ValidateTransaction 失败时不修改集合（apply.go 原子性保证）——
//     因此 Add 校验失败不污染 view；
//  2. view 只在写锁内访问；任何使 view 与池状态失同步的写（删除、换 base）
//     都将 view 标脏或整体重建（rebuild），重建源 = base + 已登记池内交易，
//     因此「校验成功但尚未登记」的交易天然被重建排除（= 撤销）。
//
// base 状态版本信号：view 以 height 判定可复用性（同一 height 内 base 内容
// 必须不变）。生产不变量（封闭可证）：链状态 bc.utxo 仅在块 append（高度
// +1）或 reorg（整体替换，可能同高）时更换，且两条路径后调用方总是先经
// RemoveIncluded / ReaddDisconnected（内部无条件 rebuild）再继续入池；Add
// 的调用方（service.go/nodeapi.go）每次传入 UTXOSnapshot() 新克隆，因此
// **不得**以 base 指针相等判定内容一致（指针每次必不同）。
type Mempool struct {
	mu      sync.RWMutex
	maxSize int
	txs     map[[32]byte]*transaction.Transaction
	fees    map[[32]byte]uint64
	order   [][32]byte // 插入顺序（FIFO），用于打包时的稳定决定性

	view       *utxo.UTXOSet // 共享校验视图（仅写锁内访问）
	viewHeight int           // view 构建时的链高度（base 状态版本）
	viewStale  bool          // 池内容与 view 失同步（有删除发生）→ 下次使用前重建
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

// rebuildLocked 整体重建共享校验视图：view = base 克隆 + 池内交易按插入序
// 应用（失效者剔除出池）。O(base+池大小)。
//
// 调用时机：首次使用、链状态版本变化（height 变化，或 reorg/上链后经
// RemoveIncluded/ReaddDisconnected 显式重建）、或发生过删除（viewStale）。
// 重建源是已登记的 m.txs/m.order——校验成功但尚未登记的交易不会出现在
// 重建结果中（调用方利用这一性质实现「校验成功后的撤销」）。重放语义与旧
// compositeView 完全一致：某笔失效只跳过自身，其效果（若有）不回溯；依赖
// 它的后续交易会在各自校验时自然失败并被剔除。
func (m *Mempool) rebuildLocked(base *utxo.UTXOSet, height int) {
	view := base.Clone()
	kept := make(map[[32]byte]*transaction.Transaction, len(m.txs))
	keptFees := make(map[[32]byte]uint64, len(m.txs))
	keptOrder := make([][32]byte, 0, len(m.order))
	for _, id := range m.order {
		tx, ok := m.txs[id]
		if !ok {
			continue
		}
		fee, err := utxo.ValidateTransaction(tx, view, height)
		if err != nil {
			continue // 失效交易被剔除（含链式级联：父被删后子输入悬空）
		}
		kept[id] = tx
		keptFees[id] = fee
		keptOrder = append(keptOrder, id)
	}
	m.txs = kept
	m.fees = keptFees
	m.order = keptOrder
	m.view = view
	m.viewHeight = height
	m.viewStale = false
}

// ensureViewLocked 返回可用的共享校验视图；必要时惰性构建或重建。
// 复用条件：height 与构建时一致（base 状态版本未变）且 view 未失同步。
// 调用约定：同一 height 内调用方传入的 base 内容必须不变（生产不变量见
// Mempool 文档）；链推进/reorg 后经 RemoveIncluded/ReaddDisconnected 显式
// 重建，或由 height 变化触发。
func (m *Mempool) ensureViewLocked(base *utxo.UTXOSet, height int) *utxo.UTXOSet {
	if m.view != nil && m.viewHeight == height && !m.viewStale {
		return m.view
	}
	m.rebuildLocked(base, height)
	return m.view
}

// Add 校验并把交易加入内存池。
//
// 校验在共享视图（base 克隆 + 池内全部待打包交易按插入序应用）上进行，
// O(交易输入数)，与池大小无关。视图同时覆盖：
//   - 双花（同一 UTXO 被池内交易占用 → ErrConflict）
//   - 链式交易（花费池内交易的输出 → 正常通过）
//
// ValidateTransaction 是 apply 型：校验成功时效果已应用到共享视图（与登记
// 保持同步）；失败时集合保持原样（原子性），视图无污染。因此**池满拒绝路径
// 绝不能在共享视图上完整校验**（校验成功但不登记会留下幽灵效果）——驱逐
// 决策用只读 probeFee（见 admitWithEvictionLocked）。
//
// base 契约：同一 height 内 base 内容必须不变（生产由「bc.utxo 仅在块
// append/reorg 替换且替换后池先经 RemoveIncluded/ReaddDisconnected 重建」
// 保证，见 Mempool 文档）；调用方每次可传 UTXOSnapshot() 新克隆。
//
// R4 池满驱逐（fee-aware，最小化）：池满时仅当新交易 fee>0 且池中存在零费
// 交易时，驱逐**最早插入**的零费交易以腾出位置（依赖其输出的池内交易在重建
// 中被级联剔除）；其余情况（新交易 fee==0、池中无零费者、驱逐后新交易仍无法
// 入池）返回 ErrPoolFull 且池状态完全回滚，不误伤。池满路径的行为与旧实现
// 一致：不先做完整校验（旧实现池满直接拒绝）。
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
		return m.admitWithEvictionLocked(base, tx, id, height)
	}

	view := m.ensureViewLocked(base, height)
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

// admitWithEvictionLocked 池满时的零费驱逐路径。返回 nil 表示 tx 已入池；
// 返回非 nil（总是 ErrPoolFull 包装）表示未接纳且池状态已回滚到驱逐前。
//
// 与共享视图的交互纪律：本路径在驱逐并重建视图**之前**不做完整校验——
// fee 判定用只读 probeFee（不修改集合）；完整校验只在驱逐腾位后的新视图上
// 进行一次（成功即应用，与登记同步；失败则回滚驱逐快照并标脏）。
func (m *Mempool) admitWithEvictionLocked(base *utxo.UTXOSet, tx *transaction.Transaction, id [32]byte, height int) error {
	// 驱逐候选：最早插入（FIFO 顺序首个）的零费交易。无候选 → 直接拒绝
	// （旧实现池满即拒，保持一致；不触碰共享视图）。
	victimIdx := -1
	for i, oid := range m.order {
		if m.fees[oid] == 0 {
			victimIdx = i
			break
		}
	}
	if victimIdx < 0 {
		return ErrPoolFull
	}

	// fee 只读估算（fee=in−out 与池状态无关）：新交易自身零费 → 驱逐他人
	// 没有意义；输入悬空等无效形态 → 同样拒绝（旧语义：池满不校验直接拒）。
	view := m.ensureViewLocked(base, height)
	fee, ferr := probeFee(view, tx)
	if ferr != nil || fee == 0 {
		return ErrPoolFull
	}
	victim := m.order[victimIdx]

	// 快照池状态：若驱逐后 tx 仍无法入池（直接或经由被级联剔除的下游依赖
	// victim），完全回滚，避免「白丢 victim 及其依赖链、tx 又没进来」。
	snapTxs := make(map[[32]byte]*transaction.Transaction, len(m.txs))
	for k, v := range m.txs {
		snapTxs[k] = v
	}
	snapFees := make(map[[32]byte]uint64, len(m.fees))
	for k, v := range m.fees {
		snapFees[k] = v
	}
	snapOrder := make([][32]byte, len(m.order))
	copy(snapOrder, m.order)

	m.removeLocked(victim)
	// 重建视图：victim 及依赖其输出的池内交易被剔除；tx 尚未登记、不参与
	// 重建。重建用当前 Add 传入的 base 快照（与 probeFee 同链状态，height
	// 未变）。
	view = m.ensureViewLocked(base, height)
	fee2, err := utxo.ValidateTransaction(tx, view, height)
	if err != nil {
		m.txs = snapTxs
		m.fees = snapFees
		m.order = snapOrder
		m.viewStale = true
		return fmt.Errorf("%w: 驱逐候选 %x 被新交易依赖（直接或经由其下游），已回滚驱逐", ErrPoolFull, victim)
	}

	m.txs[id] = tx
	m.fees[id] = fee2
	m.order = append(m.order, id)
	return nil
}

// probeFee 只读估算交易手续费（in−out）：遍历输入在集合上查询金额、累计
// 输出，均带 uint64 溢出防护。不验证签名/成熟期/花费权、不修改集合——仅
// 用于池满驱逐决策（完整校验随后在重建视图上进行）。
func probeFee(view *utxo.UTXOSet, tx *transaction.Transaction) (uint64, error) {
	var inTotal, outTotal uint64
	for _, in := range tx.Inputs {
		op := utxo.OutPoint{Hash: in.PrevTxHash, Index: in.OutIndex}
		entry, ok := view.Get(op)
		if !ok {
			return 0, fmt.Errorf("%w: %s", utxo.ErrUnknownUTXO, op)
		}
		if inTotal+entry.Value < inTotal {
			return 0, utxo.ErrOverspend
		}
		inTotal += entry.Value
	}
	for _, out := range tx.Outputs {
		if outTotal+out.Value < outTotal {
			return 0, utxo.ErrOverspend
		}
		outTotal += out.Value
	}
	if inTotal < outTotal {
		return 0, utxo.ErrOverspend
	}
	return inTotal - outTotal, nil
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
	// R4：删除使共享视图与池状态失同步 → 标脏，下次使用前整体重建。
	m.viewStale = true
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

	// R4：单次整体重建完成重验（新 base 上按插入序应用剩余交易，失效者
	// 剔除），替代旧实现的逐笔重验循环；同时刷新共享视图。
	m.rebuildLocked(newBase, height)
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

	// Phase 2: 多轮入池（R4：入口先按新链状态整体重建共享视图；后续所有
	// 轮次直接在视图上校验——ValidateTransaction 成功即应用效果，链式依赖
	// 的下游交易在后续轮次自然可见，失败原子不污染视图。不驱逐：池满即
	// 留在 remaining，与旧 addLocked 的 ErrPoolFull 行为一致）
	m.rebuildLocked(newBase, height)
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
			if tx.IsCoinbase() {
				nextRemaining = append(nextRemaining, tx)
				continue
			}
			if m.maxSize > 0 && len(m.txs) >= m.maxSize {
				nextRemaining = append(nextRemaining, tx)
				continue
			}
			fee, err := utxo.ValidateTransaction(tx, m.view, height)
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
