package mempool_test

import (
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/mempool"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

// ---- 辅助：区块链 + mempool 集成测试基础设施 ----

// reorgMempoolFixture 提供 reorg + mempool 集成的测试环境。
type reorgMempoolFixture struct {
	t       *testing.T
	chain   *blockchain.Blockchain
	pool    *mempool.Mempool
	wallet  *wallet.Wallet
	genesis *block.Block
}

func newReorgMempoolFixture(t *testing.T) *reorgMempoolFixture {
	t.Helper()
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("wallet: %v", err)
	}

	dir := t.TempDir()
	store, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	genesis := blockchain.NewGenesisBlock()
	genesisUndo := utxo.BlockUndo{Height: 0}
	if err := store.AppendCanonicalBlock(genesis, genesisUndo); err != nil {
		t.Fatalf("append genesis v2: %v", err)
	}

	chain, err := blockchain.NewBlockchainFromStore(store)
	if err != nil {
		t.Fatalf("new chain from store: %v", err)
	}

	return &reorgMempoolFixture{
		t:       t,
		chain:   chain,
		pool:    mempool.New(100),
		wallet:  w,
		genesis: genesis,
	}
}

// mineNext 挖一枚 coinbase 付给 fixture wallet 的区块并上链。
func (f *reorgMempoolFixture) mineNext() *block.Block {
	f.t.Helper()
	tip, _ := f.chain.Tip()
	h := f.chain.Height() + 1
	cb := transaction.NewCoinbaseTx(f.wallet.PubKeyHash(), utxo.Subsidy(h), h)
	b := block.NewCandidateBlock(tip.Header.Hash(), pow.MaxTargetBits, []*transaction.Transaction{cb})
	if found, _ := pow.Mine(b); !found {
		f.t.Fatal("mine failed")
	}
	if _, err := f.chain.AddBlockWithResult(b); err != nil {
		f.t.Fatalf("add block h=%d: %v", h, err)
	}
	return b
}

// mineBlockWithTxs 挖一枚包含指定非 coinbase 交易的区块并上链。
func (f *reorgMempoolFixture) mineBlockWithTxs(txs []*transaction.Transaction) *block.Block {
	f.t.Helper()
	tip, _ := f.chain.Tip()
	h := f.chain.Height() + 1
	cb := transaction.NewCoinbaseTx(f.wallet.PubKeyHash(), utxo.Subsidy(h), h)
	allTxs := append([]*transaction.Transaction{cb}, txs...)
	b := block.NewCandidateBlock(tip.Header.Hash(), pow.MaxTargetBits, allTxs)
	if found, _ := pow.Mine(b); !found {
		f.t.Fatal("mine with txs failed")
	}
	if _, err := f.chain.AddBlockWithResult(b); err != nil {
		f.t.Fatalf("add block with txs h=%d: %v", h, err)
	}
	return b
}

// mineFork 从指定 parent 挖一枚 fork 区块（不经过 AddBlock）。
// Timestamp+1 确保即使与主链块在同一秒产生，也不会哈希碰撞。
func (f *reorgMempoolFixture) mineFork(parentHash [32]byte, height int, txs []*transaction.Transaction) *block.Block {
	f.t.Helper()
	cb := transaction.NewCoinbaseTx(f.wallet.PubKeyHash(), utxo.Subsidy(height), height)
	allTxs := append([]*transaction.Transaction{cb}, txs...)
	b := block.NewCandidateBlock(parentHash, pow.MaxTargetBits, allTxs)
	b.Header.Timestamp++ // 避免与主链同高度块哈希碰撞
	if found, _ := pow.Mine(b); !found {
		f.t.Fatal("mine fork failed")
	}
	return b
}

// mineToMaturity 挖足够块使 coinbase 成熟（CoinbaseMaturity=10）。
func (f *reorgMempoolFixture) mineToMaturity() {
	f.t.Helper()
	for f.chain.Height() < utxo.CoinbaseMaturity {
		f.mineNext()
	}
}

// buildSpendTx 构造一笔消费指定 src 给自身的交易。
// C1（A-2.3-G2）：src 为单个 coinbase 输出，值 = Subsidy(h)（C1 下为 5，
// legacy 下曾为 50）；amount + change 必须 ≤ 该值（测试取 fee=1）。
func (f *reorgMempoolFixture) buildSpendTx(src utxo.OutPoint, amount, change uint64) *transaction.Transaction {
	f.t.Helper()
	tx := &transaction.Transaction{
		Inputs: []transaction.TxInput{
			{PrevTxHash: src.Hash, OutIndex: src.Index},
		},
		Outputs: []transaction.TxOutput{
			{Value: amount, PubKeyHash: f.wallet.PubKeyHash()},
		},
	}
	if change > 0 {
		tx.Outputs = append(tx.Outputs, transaction.TxOutput{Value: change, PubKeyHash: f.wallet.PubKeyHash()})
	}
	h := tx.Hash()
	for i := range tx.Inputs {
		sig, err := f.wallet.Sign(h)
		if err != nil {
			f.t.Fatalf("sign: %v", err)
		}
		tx.Inputs[i].Signature = sig
		tx.Inputs[i].PubKey = f.wallet.PublicKey
	}
	return tx
}

// coinbaseOutPoint 返回指定区块 coinbase 的第 idx 个输出对应的 OutPoint。
func coinbaseOutPoint(b *block.Block, idx uint32) utxo.OutPoint {
	return utxo.OutPoint{Hash: b.Transactions[0].Hash(), Index: idx}
}

// buildNewChainTxSet 从 ReorgResult.ConnectBlocks 构造交易去重集。
func buildNewChainTxSet(result *blockchain.ReorgResult) map[[32]byte]struct{} {
	set := make(map[[32]byte]struct{})
	for _, b := range result.ConnectBlocks {
		for _, tx := range b.Transactions {
			set[tx.Hash()] = struct{}{}
		}
	}
	return set
}

// triggerReorg 通过添加更高 work 的 fork 块触发 reorg。
// 返回 ReorgResult（nil 表示未触发 reorg）。
// 注意：reorg 可能在中间块触发，后续块变为 canonical 延长；
// 因此保留任意中间块产生的非 nil 结果。
func (f *reorgMempoolFixture) triggerReorg(forkParentHash [32]byte, forkHeight int, forkTxs ...[]*transaction.Transaction) *blockchain.ReorgResult {
	f.t.Helper()
	var prevHash = forkParentHash
	var result *blockchain.ReorgResult
	for i, txs := range forkTxs {
		h := forkHeight + i
		b := f.mineFork(prevHash, h, txs)
		r, err := f.chain.AddBlockWithResult(b)
		if err != nil {
			f.t.Fatalf("add fork block h=%d: %v", h, err)
		}
		if r != nil {
			result = r
		}
		prevHash = b.Header.Hash()
	}
	return result
}

// ---- P0 测试 ----

// TestReorgResurrectsDisconnectedTx 验证断开区块中的非 coinbase 交易被正确复活。
func TestReorgResurrectsDisconnectedTx(t *testing.T) {
	f := newReorgMempoolFixture(t)
	f.mineToMaturity() // h1..h10

	// h11：coinbase 已成熟，花费 h1 coinbase
	b1, _ := f.chain.BlockByHeight(1)
	src := coinbaseOutPoint(b1, 0)
	tx1 := f.buildSpendTx(src, 4, 0) // fee=1
	f.mineBlockWithTxs([]*transaction.Transaction{tx1})

	// 触发 reorg：从 genesis 分出的更长链（h1'..h12'）
	result := f.triggerReorg(f.genesis.Header.Hash(), 1, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if result == nil {
		t.Fatal("expected reorg")
	}

	newChainTxs := buildNewChainTxSet(result)
	accepted, rejected := f.pool.ReaddDisconnected(
		result.DisconnectBlocks, f.chain.UTXOSnapshot(), f.chain.Height(), newChainTxs,
	)
	if accepted != 1 {
		t.Fatalf("accepted=%d, want 1", accepted)
	}
	if rejected != 0 {
		t.Fatalf("rejected=%d, want 0", rejected)
	}
	if !f.pool.Has(tx1.Hash()) {
		t.Fatal("tx1 not resurrected in mempool")
	}
}

// TestReorgDoesNotResurrectCoinbase 验证 coinbase 不会被复活。
func TestReorgDoesNotResurrectCoinbase(t *testing.T) {
	f := newReorgMempoolFixture(t)
	f.mineNext() // h1
	f.mineNext() // h2

	result := f.triggerReorg(f.genesis.Header.Hash(), 1, nil, nil, nil)
	if result == nil {
		t.Fatal("expected reorg")
	}

	newChainTxs := buildNewChainTxSet(result)
	accepted, _ := f.pool.ReaddDisconnected(
		result.DisconnectBlocks, f.chain.UTXOSnapshot(), f.chain.Height(), newChainTxs,
	)
	if accepted != 0 {
		t.Fatalf("accepted=%d, want 0 (no non-coinbase)", accepted)
	}
}

// TestReorgDoesNotResurrectConfirmedTx 验证已在新链确认的交易不被复活。
func TestReorgDoesNotResurrectConfirmedTx(t *testing.T) {
	f := newReorgMempoolFixture(t)
	f.mineToMaturity()

	b1, _ := f.chain.BlockByHeight(1)
	src := coinbaseOutPoint(b1, 0)
	tx1 := f.buildSpendTx(src, 4, 0)
	f.mineBlockWithTxs([]*transaction.Transaction{tx1})

	// 新链也包含 tx1
	var prevHash = f.genesis.Header.Hash()
	forkTxs := [][]*transaction.Transaction{
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		{tx1},
		nil,
	}
	var result *blockchain.ReorgResult
	for i, txs := range forkTxs {
		h := 1 + i
		b := f.mineFork(prevHash, h, txs)
		var err error
		r, err := f.chain.AddBlockWithResult(b)
		if err != nil {
			t.Fatalf("add fork block h=%d: %v", h, err)
		}
		if r != nil {
			result = r
		}
		prevHash = b.Header.Hash()
	}
	if result == nil {
		t.Fatal("expected reorg")
	}

	newChainTxs := buildNewChainTxSet(result)
	accepted, _ := f.pool.ReaddDisconnected(
		result.DisconnectBlocks, f.chain.UTXOSnapshot(), f.chain.Height(), newChainTxs,
	)
	if accepted != 0 {
		t.Fatalf("accepted=%d, want 0 (tx1 confirmed in new chain)", accepted)
	}
}

// TestReorgRejectsInvalidUnderNewUTXO 验证在新 UTXO 下无效的交易被拒绝。
func TestReorgRejectsInvalidUnderNewUTXO(t *testing.T) {
	f := newReorgMempoolFixture(t)
	f.mineToMaturity()

	b1, _ := f.chain.BlockByHeight(1)
	src := coinbaseOutPoint(b1, 0)
	tx1 := f.buildSpendTx(src, 4, 0)
	f.mineBlockWithTxs([]*transaction.Transaction{tx1})

	// 新链中某块已花费同一 src（金额与 tx1 不同 ⇒ 哈希不同，否则会被
	// ReaddDisconnected 视为「已确认」而跳过，rejected 将为 0）
	txSpendSrc := f.buildSpendTx(src, 3, 0)
	// fork 从 genesis 分出 14 块（h1'..h14'），严格长于 canonical（h1..h11）。
	// txSpendSrc 置于 fork h12'（height=12，h1 coinbase 已度过成熟期 12-1>=10），
	// 在新链中合法花费 src；reorg 后 canonical 中的 tx1 因双花被拒绝。
	result := f.triggerReorg(f.genesis.Header.Hash(), 1, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, []*transaction.Transaction{txSpendSrc}, nil, nil)
	if result == nil {
		t.Fatal("expected reorg")
	}

	newChainTxs := buildNewChainTxSet(result)
	accepted, rejected := f.pool.ReaddDisconnected(
		result.DisconnectBlocks, f.chain.UTXOSnapshot(), f.chain.Height(), newChainTxs,
	)
	if accepted != 0 {
		t.Fatalf("accepted=%d, want 0", accepted)
	}
	if rejected != 1 {
		t.Fatalf("rejected=%d, want 1", rejected)
	}
}

// TestReorgRejectsDoubleSpend 验证双花交易被拒绝。
func TestReorgRejectsDoubleSpend(t *testing.T) {
	f := newReorgMempoolFixture(t)
	f.mineToMaturity()

	b1, _ := f.chain.BlockByHeight(1)
	src := coinbaseOutPoint(b1, 0)
	tx1 := f.buildSpendTx(src, 4, 0)

	// 直接调用：newBase 中 src 已不存在
	newBase := utxo.NewUTXOSet()
	b := &block.Block{Transactions: []*transaction.Transaction{nil, tx1}}
	accepted, rejected := f.pool.ReaddDisconnected(
		[]*block.Block{b}, newBase, 1, map[[32]byte]struct{}{},
	)
	if accepted != 0 {
		t.Fatalf("accepted=%d, want 0", accepted)
	}
	if rejected != 1 {
		t.Fatalf("rejected=%d, want 1", rejected)
	}
}

// ---- P1 测试 ----

// TestReorgResurrectsDependentTxs 验证父子交易链正确复活。
func TestReorgResurrectsDependentTxs(t *testing.T) {
	f := newReorgMempoolFixture(t)
	f.mineToMaturity()

	b1, _ := f.chain.BlockByHeight(1)
	src := coinbaseOutPoint(b1, 0)
	tx1 := f.buildSpendTx(src, 4, 0) // fee=1, out0=4
	op1 := utxo.OutPoint{Hash: tx1.Hash(), Index: 0}
	tx2 := f.buildSpendTx(op1, 3, 0) // fee=1

	f.mineBlockWithTxs([]*transaction.Transaction{tx1, tx2})

	result := f.triggerReorg(f.genesis.Header.Hash(), 1, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if result == nil {
		t.Fatal("expected reorg")
	}

	newChainTxs := buildNewChainTxSet(result)
	accepted, rejected := f.pool.ReaddDisconnected(
		result.DisconnectBlocks, f.chain.UTXOSnapshot(), f.chain.Height(), newChainTxs,
	)
	if accepted != 2 {
		t.Fatalf("accepted=%d, want 2", accepted)
	}
	if rejected != 0 {
		t.Fatalf("rejected=%d, want 0", rejected)
	}
	if !f.pool.Has(tx1.Hash()) || !f.pool.Has(tx2.Hash()) {
		t.Fatal("tx1 or tx2 not in mempool")
	}
}

// TestReorgMultiLevelDependency 验证三级依赖链正确复活。
func TestReorgMultiLevelDependency(t *testing.T) {
	f := newReorgMempoolFixture(t)
	f.mineToMaturity()

	b1, _ := f.chain.BlockByHeight(1)
	src := coinbaseOutPoint(b1, 0)
	tx1 := f.buildSpendTx(src, 4, 0) // fee=1, out0=4
	op1 := utxo.OutPoint{Hash: tx1.Hash(), Index: 0}
	tx2 := f.buildSpendTx(op1, 3, 0) // fee=1, out0=3
	op2 := utxo.OutPoint{Hash: tx2.Hash(), Index: 0}
	tx3 := f.buildSpendTx(op2, 2, 0) // fee=1

	f.mineBlockWithTxs([]*transaction.Transaction{tx1, tx2, tx3})

	result := f.triggerReorg(f.genesis.Header.Hash(), 1, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if result == nil {
		t.Fatal("expected reorg")
	}

	newChainTxs := buildNewChainTxSet(result)
	accepted, _ := f.pool.ReaddDisconnected(
		result.DisconnectBlocks, f.chain.UTXOSnapshot(), f.chain.Height(), newChainTxs,
	)
	if accepted != 3 {
		t.Fatalf("accepted=%d, want 3", accepted)
	}
}

// TestReorgRepeatedResurrectionNoDuplicate 验证重复复活不重复入池。
func TestReorgRepeatedResurrectionNoDuplicate(t *testing.T) {
	f := newReorgMempoolFixture(t)
	f.mineToMaturity()

	b1, _ := f.chain.BlockByHeight(1)
	src := coinbaseOutPoint(b1, 0)
	tx1 := f.buildSpendTx(src, 4, 0)
	f.mineBlockWithTxs([]*transaction.Transaction{tx1})

	result := f.triggerReorg(f.genesis.Header.Hash(), 1, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if result == nil {
		t.Fatal("expected reorg")
	}

	newChainTxs := buildNewChainTxSet(result)
	base := f.chain.UTXOSnapshot()
	h := f.chain.Height()

	acc1, _ := f.pool.ReaddDisconnected(result.DisconnectBlocks, base, h, newChainTxs)
	if acc1 != 1 {
		t.Fatalf("first accepted=%d, want 1", acc1)
	}
	acc2, _ := f.pool.ReaddDisconnected(result.DisconnectBlocks, base, h, newChainTxs)
	if acc2 != 0 {
		t.Fatalf("second accepted=%d, want 0", acc2)
	}
	if f.pool.Len() != 1 {
		t.Fatalf("pool len=%d, want 1", f.pool.Len())
	}
}

// TestReorgMempoolCapacityRespected 验证 pool 容量限制复活数量。
// 设计：pool 容量=1，disconnect 两块各含 1 笔可复活交易（tx1@h11, tx2@h12）。
// 复活时 tx1 入池（1/1 满），tx2 因容量耗尽被拒绝 → accepted=1, rejected=1。
func TestReorgMempoolCapacityRespected(t *testing.T) {
	f := newReorgMempoolFixture(t)
	f.pool = mempool.New(1) // capacity 1
	f.mineToMaturity()

	b1, _ := f.chain.BlockByHeight(1)
	src1 := coinbaseOutPoint(b1, 0)
	tx1 := f.buildSpendTx(src1, 4, 0)
	f.mineBlockWithTxs([]*transaction.Transaction{tx1})

	b2, _ := f.chain.BlockByHeight(2)
	src2 := coinbaseOutPoint(b2, 0)
	tx2 := f.buildSpendTx(src2, 4, 0)
	f.mineBlockWithTxs([]*transaction.Transaction{tx2})

	result := f.triggerReorg(f.genesis.Header.Hash(), 1, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if result == nil {
		t.Fatal("expected reorg")
	}

	newChainTxs := buildNewChainTxSet(result)
	accepted, rejected := f.pool.ReaddDisconnected(
		result.DisconnectBlocks, f.chain.UTXOSnapshot(), f.chain.Height(), newChainTxs,
	)
	if accepted != 1 {
		t.Fatalf("accepted=%d, want 1", accepted)
	}
	if rejected != 1 {
		t.Fatalf("rejected=%d, want 1", rejected)
	}
}

// TestReorgResurrectionFailureDoesNotAffectTIP 验证复活失败不影响 canonical TIP。
func TestReorgResurrectionFailureDoesNotAffectTIP(t *testing.T) {
	f := newReorgMempoolFixture(t)
	f.mineToMaturity()

	b1, _ := f.chain.BlockByHeight(1)
	src := coinbaseOutPoint(b1, 0)
	tx1 := f.buildSpendTx(src, 4, 0)
	f.mineBlockWithTxs([]*transaction.Transaction{tx1})

	result := f.triggerReorg(f.genesis.Header.Hash(), 1, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if result == nil {
		t.Fatal("expected reorg")
	}

	// 验证 TIP 已更新到新链
	newTip, _ := f.chain.Tip()
	newTipHash := newTip.Header.Hash()
	if newTipHash == f.genesis.Header.Hash() {
		t.Fatal("TIP did not change")
	}

	// 真正制造一次复活失败：换用容量为 1 的池，并先用**另一笔**交易把它填满，
	// 使随后对 tx1 的复活必然撞上 ErrPoolFull。
	// 注意 mempool.New(0) 表示「不限制容量」，不能用来制造池满。
	f.pool = mempool.New(1)
	base := f.chain.UTXOSnapshot()
	h := f.chain.Height()
	b2, err := f.chain.BlockByHeight(2)
	if err != nil {
		t.Fatalf("block at height 2: %v", err)
	}
	txFill := f.buildSpendTx(coinbaseOutPoint(b2, 0), 4, 0)
	if err := f.pool.Add(base, txFill, h); err != nil {
		t.Fatalf("pre-fill pool: %v", err)
	}

	newChainTxs := buildNewChainTxSet(result)
	accepted, rejected := f.pool.ReaddDisconnected(result.DisconnectBlocks, base, h, newChainTxs)
	if accepted != 0 {
		t.Fatalf("accepted=%d, want 0 (pool full)", accepted)
	}
	if rejected != 1 {
		t.Fatalf("rejected=%d, want 1 (pool full)", rejected)
	}

	// 复活失败后 canonical chain 仍正确
	tipAfter, _ := f.chain.Tip()
	if tipAfter.Header.Hash() != newTipHash {
		t.Fatal("TIP changed after failed resurrection")
	}
	if f.pool.Has(tx1.Hash()) {
		t.Fatal("tx1 must not be resurrected when pool is full")
	}
}

// ---- P2 测试 ----

// TestReorgZeroResurrectableTransactions 验证无候选时安全处理。
func TestReorgZeroResurrectableTransactions(t *testing.T) {
	f := newReorgMempoolFixture(t)
	f.mineNext()
	f.mineNext()

	result := f.triggerReorg(f.genesis.Header.Hash(), 1, nil, nil, nil)
	if result == nil {
		t.Fatal("expected reorg")
	}

	newChainTxs := buildNewChainTxSet(result)
	accepted, rejected := f.pool.ReaddDisconnected(
		result.DisconnectBlocks, f.chain.UTXOSnapshot(), f.chain.Height(), newChainTxs,
	)
	if accepted != 0 || rejected != 0 {
		t.Fatalf("accepted=%d rejected=%d, want 0/0", accepted, rejected)
	}
}

// TestReorgMultipleDisconnectedBlocks 验证多个断开块按确定性顺序扫描。
// 设计：fork 从 h10 分出（h11'..h13'），严格长于 canonical（h1..h12）。
// 由于 h11' 的工作量严格小于 canonical tip，reorg 不会在 tie 点提前触发；
// reorg 稳定落在 h12'（tie）或 h13'（严格反超），两者公共祖先均为 h10，
// 故断开块恒为 [h11, h12] 两块，tx1/tx2 均被复活。
func TestReorgMultipleDisconnectedBlocks(t *testing.T) {
	f := newReorgMempoolFixture(t)
	f.mineToMaturity()

	b1, _ := f.chain.BlockByHeight(1)
	src1 := coinbaseOutPoint(b1, 0)
	tx1 := f.buildSpendTx(src1, 4, 0)
	f.mineBlockWithTxs([]*transaction.Transaction{tx1})

	b2, _ := f.chain.BlockByHeight(2)
	src2 := coinbaseOutPoint(b2, 0)
	tx2 := f.buildSpendTx(src2, 4, 0)
	f.mineBlockWithTxs([]*transaction.Transaction{tx2})

	// fork 从 h10 分出，3 个 fork 块（h11'..h13'），高度 13 > canonical 12
	b10, _ := f.chain.BlockByHeight(10)
	result := f.triggerReorg(b10.Header.Hash(), 11, nil, nil, nil)
	if result == nil {
		t.Fatal("expected reorg")
	}

	if len(result.DisconnectBlocks) != 2 {
		t.Fatalf("disconnect blocks=%d, want 2", len(result.DisconnectBlocks))
	}

	newChainTxs := buildNewChainTxSet(result)
	accepted, _ := f.pool.ReaddDisconnected(
		result.DisconnectBlocks, f.chain.UTXOSnapshot(), f.chain.Height(), newChainTxs,
	)
	if accepted != 2 {
		t.Fatalf("accepted=%d, want 2", accepted)
	}
}

// ---- AUDIT-2 追加回归测试（守护 Case 2 损坏防护的边界） ----

// TestReorgDetachedRedeliveryIsIdempotent 锁住 Case 2「重复持久化记录」防护的两个方向：
//
//  1. 已落盘的 **detached（非 canonical）** fork 区块被再次投递 → 必须按幂等处理（返回 nil）。
//     防护判据只能查 canonical 内存链；若误用 blockAtHash（会回退 storage 索引，
//     而 v2 records 同时登记 detached 块），此处会被错误拒绝，并让 OnBlocksResp
//     直接 return，导致 syncing 标志永久卡死。
//  2. canonical 链上已存在的区块被再次投递 → 仍必须被拒绝（防护不得被削弱）。
func TestReorgDetachedRedeliveryIsIdempotent(t *testing.T) {
	f := newReorgMempoolFixture(t)
	f.mineNext() // h1
	f.mineNext() // h2
	f.mineNext() // h3

	h1, err := f.chain.BlockByHeight(1)
	if err != nil {
		t.Fatalf("block at height 1: %v", err)
	}

	// 从 h1 分出一枚 work 更低的 fork 块：走 Case 2 → SaveBlockDetached，不触发 reorg。
	a1 := f.mineFork(h1.Header.Hash(), 2, nil)
	res, err := f.chain.AddBlockWithResult(a1)
	if err != nil {
		t.Fatalf("first delivery of detached fork block: %v", err)
	}
	if res != nil {
		t.Fatal("lower-work fork must not trigger reorg")
	}

	// 方向 1：二次投递同一枚 detached 块 → 幂等吞掉
	if _, err := f.chain.AddBlockWithResult(a1); err != nil {
		t.Fatalf("re-delivery of detached fork block must be idempotent, got: %v", err)
	}

	// 方向 2：canonical 链上已存在的块被再次投递 → 必须拒绝
	h2, err := f.chain.BlockByHeight(2)
	if err != nil {
		t.Fatalf("block at height 2: %v", err)
	}
	if _, err := f.chain.AddBlockWithResult(h2); err == nil {
		t.Fatal("re-delivery of canonical block must be rejected")
	}

	// 边界不被破坏：canonical 链与 TIP 未因上述投递而改变
	tip, _ := f.chain.Tip()
	want, err := f.chain.BlockByHeight(3)
	if err != nil {
		t.Fatalf("block at height 3: %v", err)
	}
	if tip.Header.Hash() != want.Header.Hash() {
		t.Fatal("TIP changed after idempotent/rejected redelivery")
	}
}
