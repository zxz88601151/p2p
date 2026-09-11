package pow_test

import (
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
)

// mineValid 在给定难度下挖出一个合法区块（单测用，难度调低以加速）。
func mineValid(t *testing.T, bits uint32) *block.Block {
	t.Helper()
	cb := transaction.NewCoinbaseTx([20]byte{0x01}, 50)
	b := block.NewCandidateBlock([32]byte{}, bits, []*transaction.Transaction{cb})
	found, _ := pow.Mine(b, 0) // 0 = 无限迭代
	if !found {
		t.Fatalf("Mine returned false with unlimited iterations at bits=%d", bits)
	}
	return b
}

// TestMineProducesValidNonce 验证 Mine 找到的 Nonce 确实满足 PoW。
func TestMineProducesValidNonce(t *testing.T) {
	b := mineValid(t, 18)
	if !pow.Validate(&b.Header) {
		t.Fatal("Mine produced a nonce that does not satisfy PoW")
	}
}

// TestValidateAcceptsValidPoW 验证合法挖出的区块通过 Validate。
func TestValidateAcceptsValidPoW(t *testing.T) {
	b := mineValid(t, 18)
	if !pow.Validate(&b.Header) {
		t.Fatal("Validate rejected a legitimately mined block")
	}
}

// TestValidateRejectsInvalidPoW 验证无法满足目标的区块被拒绝。
// 将 Bits 设为 256 => target=1，哈希 < 1 永不成立。
func TestValidateRejectsInvalidPoW(t *testing.T) {
	b := mineValid(t, 18)
	b.Header.Bits = 256
	if pow.Validate(&b.Header) {
		t.Fatal("Validate accepted a block whose hash cannot meet the target")
	}
}

// TestDifficultyAdjustmentBounds 验证难度调整结果始终落在 [1, MaxTargetBits]。
func TestDifficultyAdjustmentBounds(t *testing.T) {
	short := pow.AdjustBits(pow.MaxTargetBits, 1)    // 远快于期望 -> 更难, 封顶
	long := pow.AdjustBits(pow.MaxTargetBits, 1<<40) // 远慢于期望 -> 更易
	if short < 1 || short > pow.MaxTargetBits {
		t.Fatalf("AdjustBits(short)=%d out of [1,%d]", short, pow.MaxTargetBits)
	}
	if long < 1 || long > pow.MaxTargetBits {
		t.Fatalf("AdjustBits(long)=%d out of [1,%d]", long, pow.MaxTargetBits)
	}
	if short < long {
		t.Fatalf("shorter timespan should yield >= bits than longer: short=%d long=%d", short, long)
	}
}

// TestDifficultyAdjustmentStableAtExpected 验证实际出块时间恰为期望值时难度不变。
func TestDifficultyAdjustmentStableAtExpected(t *testing.T) {
	got := pow.AdjustBits(pow.MaxTargetBits,
		int64(pow.TargetBlockTimeSeconds)*int64(pow.DifficultyAdjustmentInterval))
	if got != pow.MaxTargetBits {
		t.Fatalf("AdjustBits at expected timespan = %d, want %d", got, pow.MaxTargetBits)
	}
}

// TestBitsToTargetMonotonic 验证 Bits 越大 target 越小（越难）。
func TestBitsToTargetMonotonic(t *testing.T) {
	if pow.BitsToTarget(21).Cmp(pow.BitsToTarget(20)) >= 0 {
		t.Fatal("BitsToTarget must decrease as Bits increase")
	}
}
