package block_test

import (
	"bytes"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
)

// TestBlockEncodeDecodeRoundTrip 规范编码往返后区块头哈希与内容完全一致。
func TestBlockEncodeDecodeRoundTrip(t *testing.T) {
	cb := transaction.NewCoinbaseTx([20]byte{0x01}, 50, 3)
	tx := &transaction.Transaction{
		Inputs: []transaction.TxInput{{
			PrevTxHash: [32]byte{0xAB},
			OutIndex:   2,
			Signature:  []byte{1, 2, 3, 4, 5},
			PubKey:     bytes.Repeat([]byte{7}, 65),
		}},
		Outputs: []transaction.TxOutput{
			{Value: 30, PubKeyHash: [20]byte{0x02}},
			{Value: 19, PubKeyHash: [20]byte{0x03}},
		},
	}
	b := block.NewCandidateBlock([32]byte{0xEE}, pow.MaxTargetBits,
		[]*transaction.Transaction{cb, tx})
	pow.Mine(b, 0)

	encoded := b.Encode()
	if len(encoded) == 0 || len(encoded) != b.Size() {
		t.Fatalf("编码长度异常: %d vs Size %d", len(encoded), b.Size())
	}

	decoded, err := block.DecodeBlock(encoded)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if decoded.Header.Hash() != b.Header.Hash() {
		t.Fatal("往返后区块头哈希不一致")
	}
	if len(decoded.Transactions) != 2 {
		t.Fatalf("交易数 = %d, want 2", len(decoded.Transactions))
	}
	if decoded.Transactions[0].Hash() != cb.Hash() {
		t.Fatal("coinbase 往返后 TxID 不一致")
	}
	if decoded.Transactions[1].Hash() != tx.Hash() {
		t.Fatal("普通交易往返后 TxID 不一致")
	}
	if !bytes.Equal(decoded.Transactions[1].Inputs[0].Signature, tx.Inputs[0].Signature) {
		t.Fatal("签名字段往返后不一致")
	}
	// 再编码必须字节级一致（规范性）
	if !bytes.Equal(decoded.Encode(), encoded) {
		t.Fatal("二次编码结果不一致（编码非规范）")
	}
}

// TestDecodeRejectsTruncated 截断数据必须报错而非 panic。
func TestDecodeRejectsTruncated(t *testing.T) {
	cb := transaction.NewCoinbaseTx([20]byte{0x01}, 50, 0)
	b := block.NewCandidateBlock([32]byte{}, pow.MaxTargetBits, []*transaction.Transaction{cb})
	encoded := b.Encode()

	for _, cut := range []int{1, 10, len(encoded) - 1} {
		if _, err := block.DecodeBlock(encoded[:cut]); err == nil {
			t.Fatalf("截断到 %d 字节未被拒绝", cut)
		}
	}
	if _, err := block.DecodeBlock(append(encoded, 0x00)); err == nil {
		t.Fatal("尾部多余字节未被拒绝")
	}
}

// TestDecodeRejectsHugeCounts 恶意长度字段必须被上限拦截。
func TestDecodeRejectsHugeCounts(t *testing.T) {
	cb := transaction.NewCoinbaseTx([20]byte{0x01}, 50, 0)
	b := block.NewCandidateBlock([32]byte{}, pow.MaxTargetBits, []*transaction.Transaction{cb})
	encoded := b.Encode()
	// 篡改交易计数为 0xFFFFFFFF（位于头部之后）
	tampered := append([]byte{}, encoded...)
	off := len(b.Header.SerializeHeader())
	tampered[off] = 0xFF
	tampered[off+1] = 0xFF
	tampered[off+2] = 0xFF
	tampered[off+3] = 0xFF
	if _, err := block.DecodeBlock(tampered); err == nil {
		t.Fatal("超大交易计数未被拒绝")
	}
}
