package transaction_test

import (
	"testing"

	"p2pchain/internal/transaction"
)

// TestNewCoinbaseTxIsCoinbase 验证出块奖励交易被正确识别为 Coinbase。
func TestNewCoinbaseTxIsCoinbase(t *testing.T) {
	cb := transaction.NewCoinbaseTx([20]byte{0x01}, 50)
	if !cb.IsCoinbase() {
		t.Fatal("NewCoinbaseTx must be a coinbase transaction")
	}
	if len(cb.Outputs) != 1 || cb.Outputs[0].Value != 50 {
		t.Fatalf("unexpected coinbase outputs: %+v", cb.Outputs)
	}
}

// TestNonCoinbaseNotFlagged 验证带真实输入的交易不会被误判为 Coinbase。
func TestNonCoinbaseNotFlagged(t *testing.T) {
	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: [32]byte{0xaa}, OutIndex: 0}},
		Outputs: []transaction.TxOutput{{Value: 10, PubKeyHash: [20]byte{0x02}}},
	}
	if tx.IsCoinbase() {
		t.Fatal("a transaction with a real input must not be a coinbase")
	}
}

// TestTransactionHashDeterminism 验证交易 ID 计算稳定。
func TestTransactionHashDeterminism(t *testing.T) {
	tx := transaction.NewCoinbaseTx([20]byte{0x01}, 50)
	if tx.Hash() != tx.Hash() {
		t.Fatal("transaction hash is not deterministic")
	}
}

// TestTransactionHashVariesByValue 验证不同金额产出不同 txid。
func TestTransactionHashVariesByValue(t *testing.T) {
	a := transaction.NewCoinbaseTx([20]byte{0x01}, 50)
	b := transaction.NewCoinbaseTx([20]byte{0x01}, 51)
	if a.Hash() == b.Hash() {
		t.Fatal("different reward amounts must produce different txids")
	}
}
