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
	found, _ := pow.Mine(b)
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

// TestDifficultyAdjustmentBounds 验证难度调整结果始终落在 [1, MaxDifficultyBits]（现 32，浮动上限）。
func TestDifficultyAdjustmentBounds(t *testing.T) {
	short := pow.AdjustBits(pow.MaxTargetBits, 1)    // 远快于期望 -> 更难
	long := pow.AdjustBits(pow.MaxTargetBits, 1<<40) // 远慢于期望 -> 更易
	if short < 1 || short > pow.MaxDifficultyBits {
		t.Fatalf("AdjustBits(short)=%d out of [1,%d]", short, pow.MaxDifficultyBits)
	}
	if long < 1 || long > pow.MaxDifficultyBits {
		t.Fatalf("AdjustBits(long)=%d out of [1,%d]", long, pow.MaxDifficultyBits)
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

// TestAdjustBitsDirection 验证调整方向不变量（新阶段：难度真实浮动，上限 MaxDifficultyBits=32）：
//
//	AdjustBits 输出方向由 newTarget ∝ actualTimespan 决定（先算 target 后取整），
//	下限 clamp（>= MaxTargetBits 的 target）与上限 clamp（<= MaxDifficultyBits 的 bits）只压缩可达范围，
//	不改变方向。从链起点 bits=MaxTargetBits(16) 出发：
//	  - 短跨度（算力强）：原始 target 更小 ⇒ bits 更高（>16），方向「更难」；
//	  - 均衡跨度：目标不变 ⇒ bits 不变（=16）；
//	  - 长跨度（算力弱）：原始 target 更大 ⇒ bits 更低（<16），方向「更易」。
//
// 方向不变量：时间跨度单调变长时，结果 bits 单调不增（允许持平，但绝不反向上升）。
func TestAdjustBitsDirection(t *testing.T) {
	expected := int64(pow.TargetBlockTimeSeconds) * int64(pow.DifficultyAdjustmentInterval)

	prev := uint32(0)
	for _, span := range []int64{expected / 4, expected, expected * 4} {
		got := pow.AdjustBits(pow.MaxTargetBits, span)
		if prev != 0 && got > prev {
			t.Fatalf("时间跨度变长时难度反而上升：prev=%d got=%d（span=%d）", prev, got, span)
		}
		prev = got
	}

	// 具体值锚定（起点 bits=16）：短跨度必须更难（>16），均衡必须不变（=16）。
	if short := pow.AdjustBits(pow.MaxTargetBits, expected/4); short <= pow.MaxTargetBits {
		t.Fatalf("短跨度应使难度升高（>%d），实际=%d", pow.MaxTargetBits, short)
	}
	if eq := pow.AdjustBits(pow.MaxTargetBits, expected); eq != pow.MaxTargetBits {
		t.Fatalf("均衡跨度难度应不变（=%d），实际=%d", pow.MaxTargetBits, eq)
	}
}

// TestMaxDifficultyBitsIsTheDesignedCeiling 固化新阶段的设计：
//
//	难度在 [1, MaxDifficultyBits] 内**真实浮动**，上限抬至 32（仍 >= 初始最低难度 MaxTargetBits）。
//
// 该上限是 AdjustBits 的天花板钳制（见 pow.go 常量注释）：解锁了旧链「钉死 16」的有意限制，
// 但仍保证链上可达 bits 不超过 32（避免单周期难度爆炸）。本用例守护「上限语义不被破坏」：
// AdjustBits 产出永不超过 MaxDifficultyBits，且 MaxDifficultyBits 必须 >= 初始最低难度。
func TestMaxDifficultyBitsIsTheDesignedCeiling(t *testing.T) {
	if pow.MaxDifficultyBits < pow.MaxTargetBits {
		t.Fatalf("MaxDifficultyBits=%d 低于 MaxTargetBits=%d：难度上限不应低于初始最低难度",
			pow.MaxDifficultyBits, pow.MaxTargetBits)
	}
	if pow.MaxDifficultyBits >= 256 {
		t.Fatalf("MaxDifficultyBits=%d 非法：BitsToTarget 在 bits>=256 时目标失去意义", pow.MaxDifficultyBits)
	}
	// 上限钳制：从任意合法起始难度出发，极短跨度（算力极强）的产出不得越过 MaxDifficultyBits。
	for b := uint32(1); b <= pow.MaxTargetBits; b++ {
		if got := pow.AdjustBits(b, 1); got > pow.MaxDifficultyBits {
			t.Fatalf("AdjustBits(%d, 1) = %d 越过 MaxDifficultyBits=%d", b, got, pow.MaxDifficultyBits)
		}
	}
}

// TestAdjustBitsNeverExceedsDesignedCeiling 穷举 [1, MaxDifficultyBits] 的全部合法输入难度
// × 各类时间跨度（含 0、负数、极小、均衡、极大、极端），断言输出恒落在 [1, MaxDifficultyBits]。
//
// 这是「难度浮动但不超过设计上限」的**代数级**保证：不论算力多强、时间跨度多极端，
// 链上可达 bits 都不会越过天花板 MaxDifficultyBits（现 32），也不会低于 1。
func TestAdjustBitsNeverExceedsDesignedCeiling(t *testing.T) {
	expected := int64(pow.TargetBlockTimeSeconds) * int64(pow.DifficultyAdjustmentInterval)
	spans := []int64{
		-1 << 40, -1, 0, 1,
		expected / 4, expected/4 - 1, expected,
		expected * 4, expected*4 + 1,
		1 << 20, 1 << 40,
	}
	for b := uint32(1); b <= pow.MaxDifficultyBits; b++ {
		for _, span := range spans {
			got := pow.AdjustBits(b, span)
			if got < 1 || got > pow.MaxDifficultyBits {
				t.Fatalf("AdjustBits(%d, %d) = %d，越出设计区间 [1, %d]",
					b, span, got, pow.MaxDifficultyBits)
			}
		}
	}
}

// TestAdjustBitsDirectionIsMonotonicBeforeClamp 验证「方向推导」本身正确：
// 时间跨度越短，结果越难（bits 越大，或至少相等）；越长则越易。
// 钳制只压缩可达范围，不允许出现方向反转。
func TestAdjustBitsDirectionIsMonotonicBeforeClamp(t *testing.T) {
	expected := int64(pow.TargetBlockTimeSeconds) * int64(pow.DifficultyAdjustmentInterval)
	prev := uint32(0)
	for _, span := range []int64{
		expected / 4, // 封顶（最短允许跨度）
		expected / 2, // 更快于期望
		expected,     // 均衡
		expected * 2, // 慢于期望
		expected * 4, // 触底（最长允许跨度）
	} {
		got := pow.AdjustBits(pow.MaxTargetBits, span)
		if prev != 0 && got > prev {
			t.Fatalf("时间跨度变长时难度反而上升：prev=%d got=%d（span=%d）", prev, got, span)
		}
		prev = got
	}
}
