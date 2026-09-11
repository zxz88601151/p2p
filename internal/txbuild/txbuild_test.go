package txbuild_test

import (
	"errors"
	"testing"

	"p2pchain/internal/txbuild"
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

// fundNormal 注入一个属于 owner 的普通（非 coinbase）UTXO，模拟既有链上输出。
func fundNormal(set *utxo.UTXOSet, txid [32]byte, owner *wallet.Wallet, value uint64, height int) utxo.OutPoint {
	op := utxo.OutPoint{Hash: txid}
	set.Add(op, utxo.Entry{Value: value, PubKeyHash: owner.PubKeyHash(), Height: height})
	return op
}

// TestBuildTransactionEndToEnd 构造 → 通过 utxo 全量校验 → 状态正确迁移。
func TestBuildTransactionEndToEnd(t *testing.T) {
	set := utxo.NewUTXOSet()
	sender := newWallet(t)
	receiver := newWallet(t)

	fundNormal(set, [32]byte{0x01}, sender, 1000, 0)

	tx, fee, err := txbuild.BuildTransaction(set, sender, receiver.PubKeyHash(), 400,
		txbuild.Options{Fee: 5, Height: 1})
	if err != nil {
		t.Fatalf("构造交易失败: %v", err)
	}
	if fee != 5 {
		t.Fatalf("手续费 = %d, want 5", fee)
	}

	gotFee, err := utxo.ValidateTransaction(tx, set, 1)
	if err != nil {
		t.Fatalf("构造的交易未通过 UTXO 校验: %v", err)
	}
	if gotFee != fee {
		t.Fatalf("链上手续费 = %d, want %d", gotFee, fee)
	}
	if bal := set.Balance(receiver.PubKeyHash(), 1, true); bal != 400 {
		t.Fatalf("接收方余额 = %d, want 400", bal)
	}
	if bal := set.Balance(sender.PubKeyHash(), 1, true); bal != 595 {
		t.Fatalf("发送方找零余额 = %d, want 595", bal)
	}
}

// TestSelectUTXOMultiInput 需要多输入时按金额降序选币，找零正确。
// 300+200+100 可用；amount=500 fee=10 → target=510 → 需 3 个输入（600）。
// 找零 = 600 - 510 = 90，输出合计 = 500 + 90 = 590。
func TestSelectUTXOMultiInput(t *testing.T) {
	set := utxo.NewUTXOSet()
	w := newWallet(t)
	fundNormal(set, [32]byte{0x11}, w, 300, 0)
	fundNormal(set, [32]byte{0x12}, w, 200, 0)
	fundNormal(set, [32]byte{0x13}, w, 100, 0)

	tx, fee, err := txbuild.BuildTransaction(set, w, w.PubKeyHash(), 500,
		txbuild.Options{Fee: 10, Height: 1})
	if err != nil {
		t.Fatalf("构造交易失败: %v", err)
	}
	if len(tx.Inputs) != 3 {
		t.Fatalf("输入数 = %d, want 3", len(tx.Inputs))
	}
	if fee != 10 {
		t.Fatalf("手续费 = %d, want 10", fee)
	}
	var totalOut uint64
	for _, o := range tx.Outputs {
		totalOut += o.Value
	}
	if totalOut != 590 {
		t.Fatalf("输出合计 = %d, want 590", totalOut)
	}
	if len(tx.Outputs) != 2 || tx.Outputs[1].Value != 90 {
		t.Fatalf("找零输出不正确: %+v", tx.Outputs)
	}
	if _, err := utxo.ValidateTransaction(tx, set, 1); err != nil {
		t.Fatalf("多输入交易未通过校验: %v", err)
	}
}

// TestInsufficientFunds 余额不足时拒绝。
func TestInsufficientFunds(t *testing.T) {
	set := utxo.NewUTXOSet()
	w := newWallet(t)
	fundNormal(set, [32]byte{0x21}, w, 100, 0)

	_, _, err := txbuild.BuildTransaction(set, w, w.PubKeyHash(), 100, txbuild.Options{Fee: 1, Height: 1})
	if !errors.Is(err, txbuild.ErrInsufficientFunds) {
		t.Fatalf("余额不足未拒绝: %v", err)
	}
}

// TestImmatureCoinbaseExcluded 未成熟 coinbase 不参与选币。
func TestImmatureCoinbaseExcluded(t *testing.T) {
	set := utxo.NewUTXOSet()
	w := newWallet(t)
	set.Add(utxo.OutPoint{Hash: [32]byte{0x31}}, utxo.Entry{
		Value: 500, PubKeyHash: w.PubKeyHash(), Height: 100, IsCoinbase: true,
	})

	// height=105 → 未成熟（需 110），选币应失败
	if _, _, err := txbuild.BuildTransaction(set, w, w.PubKeyHash(), 100,
		txbuild.Options{Fee: 1, Height: 105}); !errors.Is(err, txbuild.ErrInsufficientFunds) {
		t.Fatalf("未成熟 coinbase 被用于支付: %v", err)
	}
	// height=110 → 成熟，可以花费
	if _, _, err := txbuild.BuildTransaction(set, w, w.PubKeyHash(), 100,
		txbuild.Options{Fee: 1, Height: 110}); err != nil {
		t.Fatalf("成熟 coinbase 无法花费: %v", err)
	}
}

// TestSpendableVsTotalBalance 余额口径区分未成熟部分。
func TestSpendableVsTotalBalance(t *testing.T) {
	set := utxo.NewUTXOSet()
	w := newWallet(t)
	set.Add(utxo.OutPoint{Hash: [32]byte{0x41}}, utxo.Entry{
		Value: 500, PubKeyHash: w.PubKeyHash(), Height: 100, IsCoinbase: true,
	})
	fundNormal(set, [32]byte{0x42}, w, 30, 0)

	if got := txbuild.SpendableBalance(set, w, 101); got != 30 {
		t.Fatalf("可花费余额 = %d, want 30", got)
	}
	if got := txbuild.TotalBalance(set, w, 101); got != 530 {
		t.Fatalf("总余额 = %d, want 530", got)
	}
}
