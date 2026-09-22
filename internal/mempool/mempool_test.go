package mempool_test

import (
	"errors"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/mempool"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

func newWallet(t *testing.T) *wallet.Wallet {
	t.Helper()
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("生成钱包失败: %v", err)
	}
	return w
}

func fund(set *utxo.UTXOSet, txid [32]byte, owner *wallet.Wallet, value uint64, height int) utxo.OutPoint {
	op := utxo.OutPoint{Hash: txid}
	set.Add(op, utxo.Entry{Value: value, PubKeyHash: owner.PubKeyHash(), Height: height})
	return op
}

func sign(t *testing.T, tx *transaction.Transaction, w *wallet.Wallet) {
	t.Helper()
	h := tx.Hash()
	for i := range tx.Inputs {
		sig, err := w.Sign(h)
		if err != nil {
			t.Fatalf("签名失败: %v", err)
		}
		tx.Inputs[i].Signature = sig
		tx.Inputs[i].PubKey = w.PublicKey
	}
}

// payTo 构造一笔「消费 src 并支付 amount 给 to、找零回 w」的交易。
func payTo(t *testing.T, src utxo.OutPoint, w *wallet.Wallet, to [20]byte, amount, change uint64) *transaction.Transaction {
	t.Helper()
	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: src.Hash, OutIndex: src.Index}},
		Outputs: []transaction.TxOutput{{Value: amount, PubKeyHash: to}},
	}
	if change > 0 {
		tx.Outputs = append(tx.Outputs, transaction.TxOutput{Value: change, PubKeyHash: w.PubKeyHash()})
	}
	sign(t, tx, w)
	return tx
}

func TestAddValidTransaction(t *testing.T) {
	base := utxo.NewUTXOSet()
	pool := mempool.New(100)
	w := newWallet(t)
	op := fund(base, [32]byte{1}, w, 100, 0)

	tx := payTo(t, op, w, w.PubKeyHash(), 90, 5) // fee = 5
	if err := pool.Add(base, tx, 1); err != nil {
		t.Fatalf("合法交易入池失败: %v", err)
	}
	if pool.Len() != 1 {
		t.Fatalf("池长度 = %d, want 1", pool.Len())
	}
	if fee, ok := pool.Fee(tx.Hash()); !ok || fee != 5 {
		t.Fatalf("手续费记录错误: %d, %v", fee, ok)
	}
}

func TestDuplicateRejected(t *testing.T) {
	base := utxo.NewUTXOSet()
	pool := mempool.New(100)
	w := newWallet(t)
	op := fund(base, [32]byte{2}, w, 100, 0)
	tx := payTo(t, op, w, w.PubKeyHash(), 100, 0)

	if err := pool.Add(base, tx, 1); err != nil {
		t.Fatalf("首次入池失败: %v", err)
	}
	if err := pool.Add(base, tx, 1); !errors.Is(err, mempool.ErrKnownTx) {
		t.Fatalf("重复交易未拒绝: %v", err)
	}
}

func TestPoolConflictRejected(t *testing.T) {
	base := utxo.NewUTXOSet()
	pool := mempool.New(100)
	w := newWallet(t)
	op := fund(base, [32]byte{3}, w, 100, 0)

	tx1 := payTo(t, op, w, w.PubKeyHash(), 95, 0)
	if err := pool.Add(base, tx1, 1); err != nil {
		t.Fatalf("tx1 入池失败: %v", err)
	}
	// 同一输出被第二笔交易花费 → 池内冲突
	tx2 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
		Outputs: []transaction.TxOutput{{Value: 90, PubKeyHash: w.PubKeyHash()}},
	}
	sign(t, tx2, w)
	if err := pool.Add(base, tx2, 1); !errors.Is(err, mempool.ErrConflict) {
		t.Fatalf("池内双花未拒绝: %v", err)
	}
}

func TestChainedTransactionAccepted(t *testing.T) {
	base := utxo.NewUTXOSet()
	pool := mempool.New(100)
	w := newWallet(t)
	op := fund(base, [32]byte{4}, w, 100, 0)

	parent := payTo(t, op, w, w.PubKeyHash(), 60, 30) // fee 10
	if err := pool.Add(base, parent, 1); err != nil {
		t.Fatalf("父交易入池失败: %v", err)
	}

	// 子交易花费父交易的输出（链式交易）
	childSrc := utxo.OutPoint{Hash: parent.Hash(), Index: 0}
	child := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: childSrc.Hash, OutIndex: childSrc.Index}},
		Outputs: []transaction.TxOutput{{Value: 55, PubKeyHash: w.PubKeyHash()}},
	}
	sign(t, child, w)
	if err := pool.Add(base, child, 1); err != nil {
		t.Fatalf("链式交易入池失败: %v", err)
	}
	if pool.Len() != 2 {
		t.Fatalf("池长度 = %d, want 2", pool.Len())
	}
}

func TestCoinbaseRejectedFromPool(t *testing.T) {
	base := utxo.NewUTXOSet()
	pool := mempool.New(100)
	w := newWallet(t)
	cb := transaction.NewCoinbaseTx(w.PubKeyHash(), 50, 1)
	if err := pool.Add(base, cb, 1); !errors.Is(err, mempool.ErrCoinbaseIn) {
		t.Fatalf("coinbase 入池未被拒绝: %v", err)
	}
}

func TestPoolFull(t *testing.T) {
	base := utxo.NewUTXOSet()
	pool := mempool.New(1)
	w := newWallet(t)
	op1 := fund(base, [32]byte{5}, w, 100, 0)
	op2 := fund(base, [32]byte{6}, w, 100, 0)

	if err := pool.Add(base, payTo(t, op1, w, w.PubKeyHash(), 100, 0), 1); err != nil {
		t.Fatalf("第一笔入池失败: %v", err)
	}
	if err := pool.Add(base, payTo(t, op2, w, w.PubKeyHash(), 100, 0), 1); !errors.Is(err, mempool.ErrPoolFull) {
		t.Fatalf("池满未拒绝: %v", err)
	}
}

func TestPendingSortedByFee(t *testing.T) {
	base := utxo.NewUTXOSet()
	pool := mempool.New(100)
	w := newWallet(t)
	opLow := fund(base, [32]byte{7}, w, 100, 0)
	opHigh := fund(base, [32]byte{8}, w, 100, 0)

	low := payTo(t, opLow, w, w.PubKeyHash(), 99, 0)   // fee 1
	high := payTo(t, opHigh, w, w.PubKeyHash(), 80, 0) // fee 20
	if err := pool.Add(base, low, 1); err != nil {
		t.Fatal(err)
	}
	if err := pool.Add(base, high, 1); err != nil {
		t.Fatal(err)
	}

	pend := pool.Pending(10)
	if len(pend) != 2 {
		t.Fatalf("待打包交易数 = %d, want 2", len(pend))
	}
	if pend[0].Hash() != high.Hash() {
		t.Fatal("待打包交易未按手续费降序排列")
	}
	if got := pool.TotalFees(pend); got != 21 {
		t.Fatalf("手续费总额 = %d, want 21", got)
	}
	// max 限制
	if got := len(pool.Pending(1)); got != 1 {
		t.Fatalf("Pending(1) 返回 %d 笔, want 1", got)
	}
}

func TestRemoveIncludedClearsAndPrunesStale(t *testing.T) {
	base := utxo.NewUTXOSet()
	pool := mempool.New(100)
	w := newWallet(t)
	opA := fund(base, [32]byte{9}, w, 100, 0)
	opB := fund(base, [32]byte{10}, w, 100, 0)

	txA := payTo(t, opA, w, w.PubKeyHash(), 90, 0) // 将被区块打包
	txB := payTo(t, opB, w, w.PubKeyHash(), 95, 0) // 留在池中
	if err := pool.Add(base, txA, 1); err != nil {
		t.Fatal(err)
	}
	if err := pool.Add(base, txB, 1); err != nil {
		t.Fatal(err)
	}

	// 区块打包 txA 与 coinbase，并推进链状态
	// C1（A-2.3-G2）：coinbase 受 Subsidy(height)+fees 上限约束，取 Subsidy(1)=5
	// （txA 手续费 10，上限 15 ≥ 5；legacy 下曾直接写 50）。
	newBase := base.Clone()
	cb := transaction.NewCoinbaseTx(w.PubKeyHash(), utxo.Subsidy(1), 1)
	blk := block.NewCandidateBlock([32]byte{}, 20, []*transaction.Transaction{cb, txA})
	if _, _, err := utxo.ApplyBlock(newBase, blk.Transactions, 1); err != nil {
		t.Fatalf("区块状态迁移失败: %v", err)
	}

	pool.RemoveIncluded(blk, mustSet(t, newBase), 1)
	if pool.Has(txA.Hash()) {
		t.Fatal("已上链交易仍留在池中")
	}
	if !pool.Has(txB.Hash()) {
		t.Fatal("有效交易被误剔除")
	}
	if pool.Len() != 1 {
		t.Fatalf("池长度 = %d, want 1", pool.Len())
	}
}

// mustSet 返回应用区块后的集合（RemoveIncluded 期望新链状态）。
func mustSet(t *testing.T, base *utxo.UTXOSet) *utxo.UTXOSet {
	t.Helper()
	return base
}

func TestStaleTransactionPruned(t *testing.T) {
	base := utxo.NewUTXOSet()
	pool := mempool.New(100)
	w := newWallet(t)
	op := fund(base, [32]byte{11}, w, 100, 0)

	tx := payTo(t, op, w, w.PubKeyHash(), 90, 0)
	if err := pool.Add(base, tx, 1); err != nil {
		t.Fatal(err)
	}

	// 另一个区块已经花费了同一输出（模拟别处上链）→ 池中交易失效
	other := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
		Outputs: []transaction.TxOutput{{Value: 50, PubKeyHash: w.PubKeyHash()}},
	}
	sign(t, other, w)
	cb := transaction.NewCoinbaseTx(w.PubKeyHash(), 50, 1)
	blk := block.NewCandidateBlock([32]byte{}, 20, []*transaction.Transaction{cb, other})
	newBase, _, err := utxo.ApplyBlock(base, blk.Transactions, 1)
	if err != nil {
		t.Fatalf("区块状态迁移失败: %v", err)
	}

	pool.RemoveIncluded(blk, newBase, 1)
	if pool.Len() != 0 {
		t.Fatalf("失效交易未被剔除: len=%d", pool.Len())
	}
}
