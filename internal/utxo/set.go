package utxo

import (
	"sync"

	"p2pchain/internal/transaction"
)

// Entry 一个未花费输出及其元数据。
type Entry struct {
	Value      uint64   // 输出金额（最小单位整数）
	PubKeyHash [20]byte // 锁定的接收方公钥哈希
	Height     int      // 该输出所在区块高度（coinbase maturity 依据）
	IsCoinbase bool     // 是否来自 coinbase 交易
}

// UTXOSet 未花费输出集合。并发安全（RWMutex）。
//
// 并发模型：读操作（Get/Balance/Clone）走读锁；写操作（Add/Spend）走写锁。
// 区块级状态迁移不在真实集合上直接进行，而是在 Clone 上完成后整体替换（见 ApplyBlock），
// 因此不存在「并发可见的中间状态」。
type UTXOSet struct {
	mu    sync.RWMutex
	items map[OutPoint]Entry
}

// NewUTXOSet 创建空集合。
func NewUTXOSet() *UTXOSet {
	return &UTXOSet{items: make(map[OutPoint]Entry)}
}

// Add 添加一个未花费输出。
func (s *UTXOSet) Add(op OutPoint, e Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[op] = e
}

// Get 查询一个未花费输出。
func (s *UTXOSet) Get(op OutPoint) (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.items[op]
	return e, ok
}

// Has 判断输出是否仍未花费。
func (s *UTXOSet) Has(op OutPoint) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.items[op]
	return ok
}

// Spend 消费（移除）一个未花费输出，返回其条目。
// 若不存在（已被消费或从未存在）返回 ErrUnknownUTXO。
func (s *UTXOSet) Spend(op OutPoint) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[op]
	if !ok {
		return Entry{}, ErrUnknownUTXO
	}
	delete(s.items, op)
	return e, nil
}

// Balance 计算属于某公钥哈希（地址）的全部未花费金额。
// includeImmature=false 时排除尚未成熟的 coinbase 输出。
func (s *UTXOSet) Balance(pubKeyHash [20]byte, currentHeight int, includeImmature bool) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var total uint64
	for _, e := range s.items {
		if e.PubKeyHash != pubKeyHash {
			continue
		}
		if !includeImmature && e.IsCoinbase && currentHeight-e.Height < CoinbaseMaturity {
			continue
		}
		total += e.Value
	}
	return total
}

// Clone 深拷贝集合（用于区块级原子状态迁移与 mempool 组合视图）。
func (s *UTXOSet) Clone() *UTXOSet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := &UTXOSet{items: make(map[OutPoint]Entry, len(s.items))}
	for k, v := range s.items {
		c.items[k] = v
	}
	return c
}

// Len 返回集合中未花费输出的数量。
func (s *UTXOSet) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// AllEntries 返回全部未花费条目的快照（调试/测试用）。
func (s *UTXOSet) AllEntries() map[OutPoint]Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[OutPoint]Entry, len(s.items))
	for k, v := range s.items {
		out[k] = v
	}
	return out
}

// outpointAt 构造某笔交易第 i 个输出的 OutPoint。
func outpointAt(tx *transaction.Transaction, i int) OutPoint {
	return OutPoint{Hash: tx.Hash(), Index: uint32(i)}
}
