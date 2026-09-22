package utxo

// A-2.3-G UINT64 OVERFLOW REMEDIATION — 对抗性测试矩阵。
//
// 覆盖（对应阶段规范 §8–§11）：
//   - Coinbase overflow：2^63+2^63 / +5 / 2^63+2^62 全部必须拒绝（修复前
//     Case 1/2 因回绕可通过——BLK-A23F-1 的判别性用例）；
//   - 正常边界：coinbase == Subsidy / == Subsidy+fees 必须 PASS，
//     == Subsidy+fees+1 必须 REJECT；合法 multi-output 必须 PASS；
//   - Fee accumulation overflow：三笔 fee = 2^63-1 的交易累计必须拒绝
//     （修复前第三笔静默回绕）；
//   - Forward/Undo 对称：Apply→Disconnect→原状态、重放一致、undo 字节级确定；
//     overflow 区块拒绝且 UTXO state 不变（无 partial state mutation）。

import (
	"bytes"
	"errors"
	"testing"

	"p2pchain/internal/transaction"
)

// multiOutCoinbase 构造多输出 coinbase（BIP34 风格高度绑定，与
// NewCoinbaseTx 同构，但允许多个输出）。
func multiOutCoinbase(pkh [20]byte, height int, values ...uint64) *transaction.Transaction {
	tx := &transaction.Transaction{
		Inputs: []transaction.TxInput{{
			PrevTxHash: [32]byte{},
			OutIndex:   0xFFFFFFFF,
			Signature:  transaction.EncodeCoinbaseHeight(height),
		}},
	}
	for _, v := range values {
		tx.Outputs = append(tx.Outputs, transaction.TxOutput{Value: v, PubKeyHash: pkh})
	}
	return tx
}

// snapshotEntries 拷贝集合全部条目（深度比较用）。
func snapshotEntries(set *UTXOSet) map[OutPoint]Entry {
	snap := make(map[OutPoint]Entry, set.Len())
	for op, e := range set.AllEntries() {
		snap[op] = e
	}
	return snap
}

func entriesEqual(a, b map[OutPoint]Entry) bool {
	if len(a) != len(b) {
		return false
	}
	for op, e := range a {
		eb, ok := b[op]
		if !ok || !entryEqual(e, eb) {
			return false
		}
	}
	return true
}

// ---- §8 Coinbase overflow 对抗矩阵 ----

func TestCoinbaseOverflowAdversarial(t *testing.T) {
	cases := []struct {
		name   string
		values []uint64
		note   string
	}{
		{"Case1: 2^63+2^63 wraps to 0", []uint64{1 << 63, 1 << 63}, "修复前回绕为 0 绕过上限检查（BLK-A23F-1 主判别）"},
		{"Case2: 2^63+2^63+5 wraps to 5", []uint64{1 << 63, 1 << 63, 5}, "修复前回绕为 5 ≤ Subsidy 通过"},
		{"Case3: 2^63+2^62 no wrap", []uint64{1 << 63, 1 << 62}, "真实合计 3*2^62 无回绕，被上限比较拒绝"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, path := range []struct {
				name  string
				apply func(base *UTXOSet, txs []*transaction.Transaction, h int) error
			}{
				{"ApplyBlock", func(b *UTXOSet, txs []*transaction.Transaction, h int) error {
					_, _, err := ApplyBlock(b, txs, h)
					return err
				}},
				{"ApplyBlockWithUndo", func(b *UTXOSet, txs []*transaction.Transaction, h int) error {
					_, _, _, err := ApplyBlockWithUndo(b, txs, h)
					return err
				}},
			} {
				t.Run(path.name, func(t *testing.T) {
					base := NewUTXOSet()
					w := newTestWallet(t)
					before := snapshotEntries(base)
					txs := []*transaction.Transaction{multiOutCoinbase(w.PubKeyHash(), 0, tc.values...)}
					err := path.apply(base, txs, 0)
					if err == nil {
						t.Fatalf("%s: 回绕/超额 coinbase 被接受（%s）", path.name, tc.note)
					}
					if !errors.Is(err, ErrExcessiveCoinbase) {
						t.Fatalf("%s: 错误类型 = %v, want ErrExcessiveCoinbase", path.name, err)
					}
					// §11：拒绝后无 partial state mutation。
					if after := snapshotEntries(base); !entriesEqual(before, after) {
						t.Fatalf("%s: 拒绝后基础集合被改变（partial state mutation）", path.name)
					}
				})
			}
		})
	}
}

// ---- §8 正常边界 ----

func TestCoinbaseNormalBoundaries(t *testing.T) {
	w := newTestWallet(t)
	to := newTestWallet(t)

	t.Run("coinbase == Subsidy passes", func(t *testing.T) {
		base := NewUTXOSet()
		txs := []*transaction.Transaction{transaction.NewCoinbaseTx(w.PubKeyHash(), Subsidy(1), 1)}
		if _, _, err := ApplyBlock(base, txs, 1); err != nil {
			t.Fatalf("恰好等于补贴的 coinbase 被拒绝: %v", err)
		}
	})

	t.Run("coinbase == Subsidy + fees passes", func(t *testing.T) {
		base := NewUTXOSet()
		op := fund(t, base, [32]byte{0xF1}, 0, w, 100, 0)
		fee := uint64(20)
		normal := &transaction.Transaction{
			Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
			Outputs: []transaction.TxOutput{{Value: 80, PubKeyHash: to.PubKeyHash()}},
		}
		signTx(t, normal, w)
		txs := []*transaction.Transaction{
			transaction.NewCoinbaseTx(w.PubKeyHash(), Subsidy(1)+fee, 1),
			normal,
		}
		s1, fees, err := ApplyBlock(base, txs, 1)
		if err != nil {
			t.Fatalf("恰好等于补贴+手续费的 coinbase 被拒绝: %v", err)
		}
		if fees != fee {
			t.Fatalf("fees = %d, want %d", fees, fee)
		}
		if s1.Len() != 2 { // op 消失，接收方输出 + coinbase 输出
			t.Fatalf("post-set len = %d, want 2", s1.Len())
		}
	})

	t.Run("coinbase == Subsidy + fees + 1 rejects", func(t *testing.T) {
		base := NewUTXOSet()
		op := fund(t, base, [32]byte{0xF2}, 0, w, 100, 0)
		fee := uint64(20)
		normal := &transaction.Transaction{
			Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
			Outputs: []transaction.TxOutput{{Value: 80, PubKeyHash: to.PubKeyHash()}},
		}
		signTx(t, normal, w)
		txs := []*transaction.Transaction{
			transaction.NewCoinbaseTx(w.PubKeyHash(), Subsidy(1)+fee+1, 1),
			normal,
		}
		if _, _, err := ApplyBlock(base, txs, 1); err == nil {
			t.Fatal("补贴+手续费+1 的 coinbase 被接受")
		} else if !errors.Is(err, ErrExcessiveCoinbase) {
			t.Fatalf("错误类型 = %v, want ErrExcessiveCoinbase", err)
		}
	})

	t.Run("legal multi-output coinbase passes", func(t *testing.T) {
		base := NewUTXOSet()
		w2 := newTestWallet(t)
		// 30(矿工) + 20(w2) = 50 = Subsidy(1)：合法多输出 coinbase，
		// 证明修复 overflow ≠ 禁止 multi-output。
		cb := &transaction.Transaction{
			Inputs: []transaction.TxInput{{
				PrevTxHash: [32]byte{},
				OutIndex:   0xFFFFFFFF,
				Signature:  transaction.EncodeCoinbaseHeight(1),
			}},
			Outputs: []transaction.TxOutput{
				{Value: 30, PubKeyHash: w.PubKeyHash()},
				{Value: 20, PubKeyHash: w2.PubKeyHash()},
			},
		}
		txs := []*transaction.Transaction{cb}
		s1, _, _, err := ApplyBlockWithUndo(base, txs, 1)
		if err != nil {
			t.Fatalf("合法多输出 coinbase 被拒绝: %v", err)
		}
		if got := s1.Balance(w.PubKeyHash(), 1, true); got != 30 {
			t.Fatalf("矿工余额 = %d, want 30", got)
		}
		if got := s1.Balance(w2.PubKeyHash(), 1, true); got != 20 {
			t.Fatalf("第二收款方余额 = %d, want 20", got)
		}
	})
}

// ---- §9 Fee accumulation overflow ----

func TestFeeAccumulationOverflow(t *testing.T) {
	for _, path := range []struct {
		name  string
		apply func(base *UTXOSet, txs []*transaction.Transaction, h int) error
	}{
		{"ApplyBlock", func(b *UTXOSet, txs []*transaction.Transaction, h int) error {
			_, _, err := ApplyBlock(b, txs, h)
			return err
		}},
		{"ApplyBlockWithUndo", func(b *UTXOSet, txs []*transaction.Transaction, h int) error {
			_, _, _, err := ApplyBlockWithUndo(b, txs, h)
			return err
		}},
	} {
		t.Run(path.name, func(t *testing.T) {
			base := NewUTXOSet()
			w := newTestWallet(t)
			to := [20]byte{0x9A}
			// 三笔交易，每笔 fee = 2^63 - 1：
			//   tx1 后 fees = 2^63-1；tx2 后 fees = 2^64-2（未回绕）；
			//   tx3 累加 2^63-1 ⇒ 真实合计 3*(2^63-1) >= 2^64 ⇒ 回绕。
			// 修复前第三笔静默回绕为 2^63-3（记账失真）；修复后必须拒绝。
			big := uint64(1) << 63
			var txs []*transaction.Transaction
			txs = append(txs, transaction.NewCoinbaseTx(w.PubKeyHash(), Subsidy(1), 1))
			for i := 0; i < 3; i++ {
				op := fund(t, base, [32]byte{0xE1, byte(i)}, 0, w, big, 0)
				tx := &transaction.Transaction{
					Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
					Outputs: []transaction.TxOutput{{Value: 1, PubKeyHash: to}},
				}
				signTx(t, tx, w)
				txs = append(txs, tx)
			}
			before := snapshotEntries(base)
			err := path.apply(base, txs, 1)
			if err == nil {
				t.Fatal("手续费累计回绕的区块被接受（静默回绕）")
			}
			if !errors.Is(err, ErrFeeOverflow) {
				t.Fatalf("错误类型 = %v, want ErrFeeOverflow", err)
			}
			if after := snapshotEntries(base); !entriesEqual(before, after) {
				t.Fatal("拒绝后基础集合被改变（partial state mutation）")
			}
		})
	}
}

// ---- §10 Forward / Undo 对称性 ----

func TestForwardUndoSymmetryRoundTrip(t *testing.T) {
	base := NewUTXOSet()
	w := newTestWallet(t)
	to := newTestWallet(t)

	op1 := fund(t, base, [32]byte{0xD1}, 0, w, 100, 0)
	op2 := fund(t, base, [32]byte{0xD2}, 0, w, 100, 0)
	s0 := snapshotEntries(base)

	fee := uint64(20)
	normal := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op1.Hash, OutIndex: op1.Index}},
		Outputs: []transaction.TxOutput{{Value: 80, PubKeyHash: to.PubKeyHash()}},
	}
	signTx(t, normal, w)
	txs := []*transaction.Transaction{
		transaction.NewCoinbaseTx(w.PubKeyHash(), Subsidy(1)+fee, 1),
		normal,
	}

	// Apply → undo
	s1, undo, fees, err := ApplyBlockWithUndo(base, txs, 1)
	if err != nil {
		t.Fatalf("合法区块被拒绝: %v", err)
	}
	if fees != fee {
		t.Fatalf("fees = %d, want %d", fees, fee)
	}
	s1Snap := snapshotEntries(s1)
	if s1.Has(op1) || !s1.Has(op2) {
		t.Fatal("apply 后消费/保留状态不正确")
	}

	// Undo → 必须恢复原状态（完整 Entry 等价，不是 Len 近似）
	restored, err := DisconnectBlock(s1, undo)
	if err != nil {
		t.Fatalf("disconnect 失败: %v", err)
	}
	if got := snapshotEntries(restored); !entriesEqual(s0, got) {
		t.Fatal("undo 后状态 != apply 前状态（round-trip 破坏）")
	}

	// 重放：S0' + Apply → S1'，与原 S1 完全一致（含 undo 的确定性字节编码）
	s1b, undo2, fees2, err := ApplyBlockWithUndo(restored, txs, 1)
	if err != nil {
		t.Fatalf("重放失败: %v", err)
	}
	if fees2 != fee || !entriesEqual(s1Snap, snapshotEntries(s1b)) {
		t.Fatal("重放状态与首次 apply 不一致")
	}
	b1, err := EncodeUndo(undo)
	if err != nil {
		t.Fatalf("encode undo 失败: %v", err)
	}
	b2, err := EncodeUndo(undo2)
	if err != nil {
		t.Fatalf("encode undo2 失败: %v", err)
	}
	if !bytes.Equal(b1, b2) {
		t.Fatal("两次 apply 产出的 undo 字节不一致（确定性破坏）")
	}

	// overflow 区块：拒绝且状态不变（forward/undo 路径同样拒绝）
	overflow := []*transaction.Transaction{
		multiOutCoinbase(w.PubKeyHash(), 1, 1<<63, 1<<63),
	}
	before := snapshotEntries(base)
	if _, _, _, err := ApplyBlockWithUndo(base, overflow, 1); err == nil {
		t.Fatal("overflow 区块被 ApplyBlockWithUndo 接受")
	}
	if _, _, err := ApplyBlock(base, overflow, 1); err == nil {
		t.Fatal("overflow 区块被 ApplyBlock 接受")
	}
	if after := snapshotEntries(base); !entriesEqual(before, after) {
		t.Fatal("overflow 拒绝后状态被改变")
	}
}
