package pow_test

import (
	"math/big"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
)

// mineValid 在给定难度下挖出一个合法区块（单测用，难度调低以加速）。
func mineValid(t *testing.T, bits uint32) *block.Block {
	t.Helper()
	cb := transaction.NewCoinbaseTx([20]byte{0x01}, 50, 0)
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

// TestTargetBitsRoundTripAllSupportedBits 验证 target→bits 逆变换（257-BitLen）
// 在全部支持难度范围内对精确 2 的幂 target 无 off-by-one：
// T(b)=2^(256-b) 的 BitLen=257-b，逆变换必须精确还原 b。
func TestTargetBitsRoundTripAllSupportedBits(t *testing.T) {
	for b := uint32(1); b <= pow.MaxTargetBits; b++ {
		target := pow.BitsToTarget(b)
		got := uint32(257 - target.BitLen())
		if got != b {
			t.Fatalf("round-trip failed for bits=%d: inverse gave %d", b, got)
		}
	}
}

// TestAdjustBitsRoundTripEquilibrium 是 TestDifficultyAdjustmentStableAtExpected
// 的推广：任意 supported bits 在均衡时间跨度下 round-trip 不漂移。
// 注意：bits < MaxTargetBits 时 newTarget 超过 MaxTarget 会被难度下限 clamp，
// 因此均衡不漂移仅对 bits = MaxTargetBits 成立，这里验证该唯一均衡点。
func TestAdjustBitsRoundTripEquilibrium(t *testing.T) {
	expected := int64(pow.TargetBlockTimeSeconds) * int64(pow.DifficultyAdjustmentInterval)
	if got := pow.AdjustBits(pow.MaxTargetBits, expected); got != pow.MaxTargetBits {
		t.Fatalf("equilibrium drift: AdjustBits(%d, expected) = %d", pow.MaxTargetBits, got)
	}
}

// TestTargetBitsConservativeRounding 验证非 2 的幂 target 的保守取整语义：
// 逆变换 bits' = 257-BitLen(t) 必须满足 T(bits') ≤ t < T(bits'-1)，
// 即 target 只会向下（偏难）取整，永不比计算值更易。
// 用例：t = 1.5×T(b)（BitLen 不变 → bits'=b）；t = 0.75×T(b)（BitLen 减 1 → bits'=b+1）。
// bits=21 作为 target 表示合法（既有 TestBitsToTargetMonotonic 即使用 T(21)；
// AdjustBits 的天花板 clamp 是输出策略，不限制表示）。
func TestTargetBitsConservativeRounding(t *testing.T) {
	three := big.NewInt(3)
	four := big.NewInt(4)
	for b := uint32(1); b <= pow.MaxTargetBits; b++ {
		base := pow.BitsToTarget(b)

		halfUp := new(big.Int).Div(new(big.Int).Mul(base, three), four) // 0.75×T(b)
		gotUp := uint32(257 - halfUp.BitLen())
		if gotUp != b+1 {
			t.Fatalf("0.75×T(%d): inverse bits = %d, want %d", b, gotUp, b+1)
		}
		if pow.BitsToTarget(gotUp).Cmp(halfUp) > 0 || halfUp.Cmp(pow.BitsToTarget(gotUp-1)) >= 0 {
			t.Fatalf("0.75×T(%d): conservative invariant T(bits')≤t<T(bits'-1) violated", b)
		}

		halfDown := new(big.Int).Div(new(big.Int).Mul(base, three), big.NewInt(2)) // 1.5×T(b)
		gotDown := uint32(257 - halfDown.BitLen())
		if gotDown != b {
			t.Fatalf("1.5×T(%d): inverse bits = %d, want %d", b, gotDown, b)
		}
		if pow.BitsToTarget(gotDown).Cmp(halfDown) > 0 || halfDown.Cmp(pow.BitsToTarget(gotDown-1)) >= 0 {
			t.Fatalf("1.5×T(%d): conservative invariant T(bits')≤t<T(bits'-1) violated", b)
		}
	}
}

// TestAdjustBitsDirection 验证既有 clamp 语义下的调整方向不变量：
//
//	AdjustBits 输出方向由 newTarget ∝ actualTimespan 决定（先算 target 后取整），
//	随后的两层 clamp（既有语义，本阶段不改）决定链可达行为：
//	  - MaxTarget 下限 clamp：target 不得超过 T(20)，难度不得低于初始值
//	  - MaxTargetBits 天花板 clamp：bits 不得超过 20
//
// 因此从链唯一可达 bits=20 出发，短/均衡/长三种时间跨度结果均为 20：
//   - 短周期（算力强）：原始 target=2^234（更难，bits 本应为 22），被天花板 clamp 回 20 —— 不变易
//   - 长周期（算力弱）：原始 target=2^238（更易，bits 本应为 18），被下限 clamp 回 20 —— 不低于初始难度
//
// 修复前的缺陷正是绕过了这条不变量：均衡态返回 19（比下限还易 2 倍）。
func TestAdjustBitsDirection(t *testing.T) {
	expected := int64(pow.TargetBlockTimeSeconds) * int64(pow.DifficultyAdjustmentInterval)

	cases := []struct {
		name      string
		timespan  int64
		direction string
	}{
		{"short timespan (hashpower up)", expected / 4, "harder-or-equal (ceiling clamp)"},
		{"expected timespan", expected, "no drift"},
		{"long timespan (hashpower down)", expected * 4, "easier-or-equal (floor clamp)"},
	}
	for _, c := range cases {
		got := pow.AdjustBits(pow.MaxTargetBits, c.timespan)
		if got != pow.MaxTargetBits {
			t.Fatalf("%s: AdjustBits = %d, want %d (%s)", c.name, got, pow.MaxTargetBits, c.direction)
		}
	}
}
