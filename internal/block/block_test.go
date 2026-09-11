package block_test

import (
	"bytes"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/transaction"
)

// TestHeaderSerializationDeterminism 验证区块头序列化是确定性的（共识安全前提：
// 不同节点必须用完全相同的字段顺序/长度计算哈希）。
func TestHeaderSerializationDeterminism(t *testing.T) {
	h := block.Header{
		Version:       1,
		PrevBlockHash: [32]byte{0xab},
		MerkleRoot:    [32]byte{0xcd},
		Timestamp:     1234567890,
		Bits:          20,
		Nonce:         42,
	}
	a := h.SerializeHeader()
	b := h.SerializeHeader()
	if !bytes.Equal(a, b) {
		t.Fatal("SerializeHeader is not deterministic")
	}
	// 4 + 32 + 32 + 8 + 4 + 8 = 88 bytes, fixed layout
	if len(a) != 88 {
		t.Fatalf("SerializeHeader length = %d, want 88", len(a))
	}
}

// TestBlockHashDeterminism 验证同一区块头的双 SHA-256 哈希稳定。
func TestBlockHashDeterminism(t *testing.T) {
	cb := transaction.NewCoinbaseTx([20]byte{0x01}, 50, 0)
	b := block.NewCandidateBlock([32]byte{}, 20, []*transaction.Transaction{cb})
	if b.Header.Hash() != b.Header.Hash() {
		t.Fatal("block header hash is not deterministic")
	}
}

// TestMerkleRootDeterminism 验证 Merkle 根计算稳定。
func TestMerkleRootDeterminism(t *testing.T) {
	txs := []*transaction.Transaction{
		transaction.NewCoinbaseTx([20]byte{0x01}, 50, 0),
		transaction.NewCoinbaseTx([20]byte{0x02}, 50, 0),
	}
	if block.ComputeMerkleRoot(txs) != block.ComputeMerkleRoot(txs) {
		t.Fatal("Merkle root is not deterministic")
	}
}

// TestMerkleRootNonZero 验证单笔交易也能得到非零 Merkle 根。
func TestMerkleRootNonZero(t *testing.T) {
	tx := transaction.NewCoinbaseTx([20]byte{0x01}, 50, 0)
	if block.ComputeMerkleRoot([]*transaction.Transaction{tx}) == ([32]byte{}) {
		t.Fatal("expected non-zero Merkle root for a single transaction")
	}
}

// TestNewCandidateBlockSetsPrevHash 验证候选区块正确写入父哈希与交易列表。
func TestNewCandidateBlockSetsPrevHash(t *testing.T) {
	prev := [32]byte{0x99}
	cb := transaction.NewCoinbaseTx([20]byte{0x01}, 50, 0)
	b := block.NewCandidateBlock(prev, 20, []*transaction.Transaction{cb})
	if b.Header.PrevBlockHash != prev {
		t.Fatalf("PrevBlockHash = %x, want %x", b.Header.PrevBlockHash, prev)
	}
	if len(b.Transactions) != 1 {
		t.Fatalf("want 1 transaction, got %d", len(b.Transactions))
	}
}
