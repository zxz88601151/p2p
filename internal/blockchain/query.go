package blockchain

// 本文件是 blockchain 的**只读查询层**，为 REORG-1H（P2P 分支投递）提供
// 「按哈希取块 / 按哈希回溯祖先 / 树成员判据 / 链尾工作量」四类原语。
//
// 为什么要单独一个文件：
//   - 这些都是**纯查询**，不产生任何状态迁移，也不写存储；
//   - 把它们与 addBlock/executeReorg 等状态机代码分开，可让「网络层能读到什么」
//     这一安全边界一目了然（M7：持久化边界）。
//
// 与 blockAtHash / canonicalContains 的关系：
//   - blockAtHash 假设调用方已持锁（被 addBlock 等内部路径复用），不便对外暴露；
//   - 这里的方法自行加读锁，供 cmd/node 与 internal/p2p 的对外调用使用。

import (
	"math/big"

	"p2pchain/internal/block"
)

// BlockByHash 按哈希查询区块。
//
// 查找顺序：内存 canonical 链 → 持久化存储。
//
// ⚠️ 语义边界：命中**不等于**在 canonical 链上。v2 存储索引同时登记 detached
//（非 canonical）区块，本函数故意也能取到它们——因为分支投递正是要靠它把
// 非 canonical 的合法分叉块送给对端。需要 canonical 判据时用 IsCanonicalHash。
func (bc *Blockchain) BlockByHash(hash [32]byte) (*block.Block, bool) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.findBlockLocked(hash)
}

// findBlockLocked 在持锁前提下按哈希查找区块（canonical 内存链 → 存储）。
func (bc *Blockchain) findBlockLocked(hash [32]byte) (*block.Block, bool) {
	for _, b := range bc.blocks {
		if b.Header.Hash() == hash {
			return b, true
		}
	}
	if bc.store == nil {
		return nil, false
	}
	b, err := bc.store.GetBlockByHash(hash)
	if err != nil || b == nil {
		return nil, false
	}
	return b, true
}

// HasBlockHash 报告该哈希对应的区块是否为本节点**已知**（canonical 或已落盘的 detached）。
// 用于判断「对端链尾我是否见过」，从而决定要不要按哈希去拉分支。
func (bc *Blockchain) HasBlockHash(hash [32]byte) bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	_, ok := bc.findBlockLocked(hash)
	return ok
}

// IsCanonicalHash 报告该哈希是否位于当前 canonical 链上（仅查内存 canonical 链，
// 绝不回退存储——存储索引含 detached 块，见 blockAtHash 的调用方须知）。
func (bc *Blockchain) IsCanonicalHash(hash [32]byte) bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.canonicalContains(hash)
}

// KnowsParent 报告给定父哈希是否已存在于本地区块树（可作为分叉块的挂载点）。
// 树索引只包含「经过共识校验并加入树」的区块：canonical 链 + 已验证的 detached 分支。
func (bc *Blockchain) KnowsParent(parentHash [32]byte) bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	if bc.tree == nil {
		return false
	}
	return bc.tree.LookupNode(parentHash) != nil
}

// BestTipWork 返回当前活动链尾的累积工作量（拷贝，调用方可安全持有）。
// 树尚未设置 tip 时返回 nil。
func (bc *Blockchain) BestTipWork() *big.Int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	if bc.tree == nil {
		return nil
	}
	w := bc.tree.BestTipWork()
	if w == nil {
		return nil
	}
	return new(big.Int).Set(w)
}

// BlockByHashWithAncestors 返回以 hash 为起点、沿 PrevBlockHash 回溯的一条**父链切片**。
//
// 返回顺序固定为：**索引 0 是请求的区块本身，其后依次是它的父、祖父……**
// （即「由新到旧」）。接收方只要从后往前应用即可满足「父先于子」的插入约束。
//
// 终止条件（任一满足即停）：
//   - 已返回 1 + maxAncestors 个区块；
//   - 回溯到创世区块（PrevBlockHash 为零哈希）；
//   - 链上某个祖先本节点没有（缺口更深，交给接收方决定是否继续回溯）。
//
// 有界性：maxAncestors <= 0 时取 1（只返回请求块本身）；调用方应自行做上限裁剪，
// 这里再做一次防御，确保绝不会因参数异常而无限回溯。
func (bc *Blockchain) BlockByHashWithAncestors(hash [32]byte, maxAncestors int) []*block.Block {
	if maxAncestors < 0 {
		maxAncestors = 0
	}
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	cur, ok := bc.findBlockLocked(hash)
	if !ok {
		return nil
	}
	out := make([]*block.Block, 0, maxAncestors+1)
	out = append(out, cur)
	var zero [32]byte
	for i := 0; i < maxAncestors; i++ {
		if cur.Header.PrevBlockHash == zero {
			break // 已到创世
		}
		parent, ok := bc.findBlockLocked(cur.Header.PrevBlockHash)
		if !ok {
			break // 缺口更深：交给请求方继续回溯
		}
		out = append(out, parent)
		cur = parent
	}
	return out
}
