package mempool_test

import (
	"errors"
	"testing"

	"p2pchain/internal/mempool"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

// childOf 构造一笔花费 parent 第 idx 个输出、支付 amount 给 to（无找零）的交易。
func childOf(t *testing.T, parent *transaction.Transaction, idx int, w *wallet.Wallet, amount uint64) *transaction.Transaction {
	t.Helper()
	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: parent.Hash(), OutIndex: uint32(idx)}},
		Outputs: []transaction.TxOutput{{Value: amount, PubKeyHash: w.PubKeyHash()}},
	}
	sign(t, tx, w)
	return tx
}

// TestZeroFeeEviction 覆盖 R4 零费驱逐决策树：
//
//	场景 A：驱逐最早零费交易 + 依赖其输出的子交易级联剔除 + 新交易入池；
//	场景 B：池满 + 新交易零费 → ErrPoolFull，池不变（不驱逐）；
//	场景 C：池满全为 fee>0 + 新交易 fee>0 → ErrPoolFull（无零费候选）；
//	场景 D：驱逐候选被新交易依赖 → 完全回滚（victim 保留、新交易拒绝）；
//	场景 E：多个零费候选时驱逐最早插入者（FIFO）；
//	场景 F：驱逐后池继续正常工作（视图与池状态保持同步）。
func TestZeroFeeEviction(t *testing.T) {
	t.Run("A_驱逐与级联剔除", func(t *testing.T) {
		base := utxo.NewUTXOSet()
		pool := mempool.New(2)
		w := newWallet(t)
		opA := fund(base, [32]byte{20}, w, 100, 0)
		opB := fund(base, [32]byte{21}, w, 50, 0) // 所有 fund 先于首个 Add（同 height base 稳定契约）

		txA := payTo(t, opA, w, w.PubKeyHash(), 100, 0) // fee 0（victim）
		if err := pool.Add(base, txA, 1); err != nil {
			t.Fatal(err)
		}
		// txC 花费 txA 的输出（链式、零费）→ 池满 {txA, txC}
		txC := childOf(t, txA, 0, w, 100)
		if err := pool.Add(base, txC, 1); err != nil {
			t.Fatal(err)
		}
		if pool.Len() != 2 {
			t.Fatalf("前置池长度 = %d, want 2", pool.Len())
		}

		txB := payTo(t, opB, w, w.PubKeyHash(), 40, 0) // fee 10
		if err := pool.Add(base, txB, 1); err != nil {
			t.Fatalf("池满驱逐路径入池失败: %v", err)
		}
		if pool.Has(txA.Hash()) {
			t.Fatal("零费 victim 未被驱逐")
		}
		if pool.Has(txC.Hash()) {
			t.Fatal("依赖 victim 的子交易未被级联剔除")
		}
		if !pool.Has(txB.Hash()) {
			t.Fatal("新交易未入池")
		}
		if pool.Len() != 1 {
			t.Fatalf("池长度 = %d, want 1", pool.Len())
		}
		// 手续费记账一致性
		pend := pool.Pending(10)
		if len(pend) != 1 || pend[0].Hash() != txB.Hash() {
			t.Fatalf("Pending 异常: %d 笔", len(pend))
		}
		if got := pool.TotalFees(pend); got != 10 {
			t.Fatalf("TotalFees = %d, want 10", got)
		}
	})

	t.Run("B_新交易零费不驱逐", func(t *testing.T) {
		base := utxo.NewUTXOSet()
		pool := mempool.New(2)
		w := newWallet(t)
		opA := fund(base, [32]byte{22}, w, 100, 0)
		opD := fund(base, [32]byte{23}, w, 100, 0)
		opE := fund(base, [32]byte{24}, w, 100, 0)

		txA := payTo(t, opA, w, w.PubKeyHash(), 100, 0) // fee 0
		txD := payTo(t, opD, w, w.PubKeyHash(), 100, 0) // fee 0
		if err := pool.Add(base, txA, 1); err != nil {
			t.Fatal(err)
		}
		if err := pool.Add(base, txD, 1); err != nil {
			t.Fatal(err)
		}

		txE := payTo(t, opE, w, w.PubKeyHash(), 100, 0) // fee 0
		err := pool.Add(base, txE, 1)
		if !errors.Is(err, mempool.ErrPoolFull) {
			t.Fatalf("零费新交易未被拒绝: %v", err)
		}
		if pool.Len() != 2 || !pool.Has(txA.Hash()) || !pool.Has(txD.Hash()) {
			t.Fatal("拒绝时池状态被改动")
		}
	})

	t.Run("C_无零费候选拒绝", func(t *testing.T) {
		base := utxo.NewUTXOSet()
		pool := mempool.New(2)
		w := newWallet(t)
		opA := fund(base, [32]byte{25}, w, 100, 0)
		opD := fund(base, [32]byte{26}, w, 100, 0)
		opE := fund(base, [32]byte{27}, w, 100, 0)

		txA := payTo(t, opA, w, w.PubKeyHash(), 90, 0) // fee 10
		txD := payTo(t, opD, w, w.PubKeyHash(), 80, 0) // fee 20
		if err := pool.Add(base, txA, 1); err != nil {
			t.Fatal(err)
		}
		if err := pool.Add(base, txD, 1); err != nil {
			t.Fatal(err)
		}

		txE := payTo(t, opE, w, w.PubKeyHash(), 95, 0) // fee 5
		err := pool.Add(base, txE, 1)
		if !errors.Is(err, mempool.ErrPoolFull) {
			t.Fatalf("无零费候选时未被拒绝: %v", err)
		}
		if pool.Len() != 2 || !pool.Has(txA.Hash()) || !pool.Has(txD.Hash()) {
			t.Fatal("拒绝时池状态被改动")
		}
	})

	t.Run("D_驱逐候选被新交易依赖则回滚", func(t *testing.T) {
		base := utxo.NewUTXOSet()
		pool := mempool.New(2)
		w := newWallet(t)
		opA := fund(base, [32]byte{28}, w, 100, 0)
		opD := fund(base, [32]byte{29}, w, 50, 0)

		txA := payTo(t, opA, w, w.PubKeyHash(), 100, 0) // fee 0（victim）
		if err := pool.Add(base, txA, 1); err != nil {
			t.Fatal(err)
		}
		txD := payTo(t, opD, w, w.PubKeyHash(), 40, 0) // fee 10
		if err := pool.Add(base, txD, 1); err != nil {
			t.Fatal(err)
		}

		// txB 花费 victim（txA）的输出 → 驱逐后输入悬空 → 必须回滚
		txB := childOf(t, txA, 0, w, 90) // fee 10
		err := pool.Add(base, txB, 1)
		if !errors.Is(err, mempool.ErrPoolFull) {
			t.Fatalf("依赖 victim 的新交易未被拒绝: %v", err)
		}
		// 完全回滚：victim 与无关交易保留，池大小不变
		if pool.Len() != 2 || !pool.Has(txA.Hash()) || !pool.Has(txD.Hash()) || pool.Has(txB.Hash()) {
			t.Fatalf("驱逐回滚不完整: len=%d", pool.Len())
		}
	})

	t.Run("E_多个零费候选驱逐最早插入者", func(t *testing.T) {
		base := utxo.NewUTXOSet()
		pool := mempool.New(3)
		w := newWallet(t)
		opA := fund(base, [32]byte{30}, w, 100, 0)
		opD := fund(base, [32]byte{31}, w, 100, 0)
		opF := fund(base, [32]byte{32}, w, 100, 0)
		opE := fund(base, [32]byte{33}, w, 100, 0)

		txA := payTo(t, opA, w, w.PubKeyHash(), 100, 0) // fee 0，先入
		txD := payTo(t, opD, w, w.PubKeyHash(), 100, 0) // fee 0，后入
		txF := payTo(t, opF, w, w.PubKeyHash(), 95, 0)  // fee 5
		if err := pool.Add(base, txA, 1); err != nil {
			t.Fatal(err)
		}
		if err := pool.Add(base, txD, 1); err != nil {
			t.Fatal(err)
		}
		if err := pool.Add(base, txF, 1); err != nil {
			t.Fatal(err)
		}

		txE := payTo(t, opE, w, w.PubKeyHash(), 80, 0) // fee 20
		if err := pool.Add(base, txE, 1); err != nil {
			t.Fatalf("驱逐路径入池失败: %v", err)
		}
		if pool.Has(txA.Hash()) {
			t.Fatal("应驱逐最早插入的 txA")
		}
		if !pool.Has(txD.Hash()) || !pool.Has(txF.Hash()) || !pool.Has(txE.Hash()) || pool.Len() != 3 {
			t.Fatalf("池状态异常: len=%d", pool.Len())
		}
	})

	t.Run("F_驱逐后池继续正常工作", func(t *testing.T) {
		base := utxo.NewUTXOSet()
		pool := mempool.New(3)
		w := newWallet(t)
		opA := fund(base, [32]byte{34}, w, 100, 0)
		opB := fund(base, [32]byte{35}, w, 50, 0)
		opD := fund(base, [32]byte{36}, w, 50, 0)

		txA := payTo(t, opA, w, w.PubKeyHash(), 100, 0) // fee 0（victim）
		if err := pool.Add(base, txA, 1); err != nil {
			t.Fatal(err)
		}
		txB := payTo(t, opB, w, w.PubKeyHash(), 40, 0) // fee 10
		if err := pool.Add(base, txB, 1); err != nil {
			t.Fatal(err)
		}
		// txC 花费 txB 输出（链式）→ 池满 {txA, txB, txC}
		txC := childOf(t, txB, 0, w, 35) // fee 5
		if err := pool.Add(base, txC, 1); err != nil {
			t.Fatal(err)
		}

		txD := payTo(t, opD, w, w.PubKeyHash(), 40, 0) // fee 10 → 驱逐 txA
		if err := pool.Add(base, txD, 1); err != nil {
			t.Fatalf("驱逐路径入池失败: %v", err)
		}
		// 驱逐 txA 后重建视图保留无依赖的 txB、txC（链式对），txD 正常登记
		if pool.Has(txA.Hash()) {
			t.Fatal("零费 victim 未被驱逐")
		}
		if !pool.Has(txB.Hash()) || !pool.Has(txC.Hash()) || !pool.Has(txD.Hash()) || pool.Len() != 3 {
			t.Fatalf("驱逐后池状态异常: len=%d", pool.Len())
		}
		if got := pool.TotalFees(pool.Pending(10)); got != 25 {
			t.Fatalf("TotalFees = %d, want 25", got)
		}
	})
}
