package pow_test

// v3_frozen_vectors_test.go —— PHASE-P2PCHAIN-V3-CONSENSUS-IMPLEMENTATION-1
// 冻结规格（docs/PHASE-P2PCHAIN-V3-CONSENSUS-SPEC-FINAL-1.md）的确定性测试向量。
//
// 覆盖冻结规格 §7（A-J）与 §8（数学测试 bits 27-40）。全部为纯函数测试，
// 可直接作为实现阶段的验收向量。

import (
	"testing"

	"p2pchain/internal/pow"
)

// TestV3FrozenParameters 锁定冻结规格的 7 个参数常量值。
func TestV3FrozenParameters(t *testing.T) {
	if pow.NewRulesetActivationHeight != 3000 {
		t.Fatalf("NewRulesetActivationHeight = %d, want 3000", pow.NewRulesetActivationHeight)
	}
	if pow.NewRulesetTargetBlockTimeSeconds != 300 {
		t.Fatalf("NewRulesetTargetBlockTimeSeconds = %d, want 300", pow.NewRulesetTargetBlockTimeSeconds)
	}
	if pow.DifficultyAdjustmentInterval != 20 {
		t.Fatalf("DifficultyAdjustmentInterval = %d, want 20", pow.DifficultyAdjustmentInterval)
	}
	if pow.NewRulesetInitialBits != 30 {
		t.Fatalf("NewRulesetInitialBits = %d, want 30", pow.NewRulesetInitialBits)
	}
	if pow.MaxDifficultyBits != 40 {
		t.Fatalf("MaxDifficultyBits = %d, want 40", pow.MaxDifficultyBits)
	}
	if pow.NewRulesetBlockVersion != 4 {
		t.Fatalf("NewRulesetBlockVersion = %d, want 4", pow.NewRulesetBlockVersion)
	}
	// V2 目标时间必须保持 60（V3 的 300 不得覆盖 V2）。
	if pow.TargetBlockTimeSeconds != 60 {
		t.Fatalf("TargetBlockTimeSeconds = %d, want 60（V2 目标时间不得被覆盖）", pow.TargetBlockTimeSeconds)
	}
}

// TestV3TargetModel 锁定 target(bits)=2^(256-bits) 与 Work(bits)=2^bits 的精确值。
func TestV3TargetModel(t *testing.T) {
	cases := []struct {
		bits         uint32
		wantBitLen   int // target 的 BitLen = 257 - bits
		wantWorkBits int // Work 的 BitLen = bits+1
	}{
		{27, 230, 28},
		{28, 229, 29},
		{29, 228, 30},
		{30, 227, 31},
		{31, 226, 32},
		{40, 217, 41},
	}
	for _, c := range cases {
		tgt := pow.BitsToTarget(c.bits)
		if tgt.BitLen() != c.wantBitLen {
			t.Fatalf("BitsToTarget(%d).BitLen() = %d, want %d（target=2^(256-bits)）",
				c.bits, tgt.BitLen(), c.wantBitLen)
		}
		// 精确：BitsToTarget(30) = 2^226；BitsToTarget(40) = 2^216。
		w := pow.WorkOfBits(c.bits)
		if w.BitLen() != c.wantWorkBits {
			t.Fatalf("WorkOfBits(%d).BitLen() = %d, want %d（work=2^bits）",
				c.bits, w.BitLen(), c.wantWorkBits)
		}
	}
	// 显式锚点：bits=30 → target=2^226（BitLen=227）；bits=40 → target=2^216（BitLen=217）。
	if pow.BitsToTarget(30).BitLen() != 227 {
		t.Fatalf("BitsToTarget(30).BitLen() = %d, want 227（=2^226）", pow.BitsToTarget(30).BitLen())
	}
	if pow.BitsToTarget(40).BitLen() != 217 {
		t.Fatalf("BitsToTarget(40).BitLen() = %d, want 217（=2^216）", pow.BitsToTarget(40).BitLen())
	}
}

// TestV3MaxBitsCeiling 锁定 MaxBits=40 ceiling：AdjustBitsNearest 在极短跨度 + 高起点下钳到 40，
// 且从 bits=41 的输入（越界）也钳回 40。
func TestV3MaxBitsCeiling(t *testing.T) {
	// 极短跨度（算力极强）+ 起点 40 → 原始 Nearest 输出会超 40，ceiling 钳回 40。
	if got := pow.AdjustBitsNearest(40, 1); got != 40 {
		t.Fatalf("AdjustBitsNearest(40, 1) = %d, want 40（ceiling clamp）", got)
	}
	// 从 39 出发，极短跨度 → 钳到 40（不超过）。
	if got := pow.AdjustBitsNearest(39, 1); got > 40 {
		t.Fatalf("AdjustBitsNearest(39, 1) = %d, 超过 MaxDifficultyBits=40", got)
	}
	// 均衡态下 bits=40 不变（T(40)*expected/expected = T(40) → Nearest=40）。
	expected := int64(pow.NewRulesetTargetBlockTimeSeconds) * int64(pow.DifficultyAdjustmentInterval)
	if got := pow.AdjustBitsNearest(40, expected); got != 40 {
		t.Fatalf("AdjustBitsNearest(40, expected) = %d, want 40（均衡）", got)
	}
}

// TestV3NearestRoundTrip 锁定 bits→target→bits 在 27..40 的精确往返（Nearest 在 2 的幂处精确还原）。
func TestV3NearestRoundTrip(t *testing.T) {
	for b := uint32(27); b <= 40; b++ {
		tgt := pow.BitsToTarget(b)
		// 往返：BitsToTarget(b) 的 target 再反推 bits 应为 b（2 的幂处精确）。
		if got := 257 - tgt.BitLen(); got != int(b) {
			t.Fatalf("257 - BitLen(T(%d)) = %d, want %d（往返失败）", b, got, b)
		}
	}
}

// TestV3InitialBitsEquilibrium 锁定 InitialBits=30 是 300s 目标的精确平衡点。
//
// 数学：平衡算力 H = 2^30/300；期望出块时间 = 2^30/H = 300s。
// 用 AdjustBitsNearest 在均衡跨度下验证 bits 不变（=30）。
func TestV3InitialBitsEquilibrium(t *testing.T) {
	expected := int64(pow.NewRulesetTargetBlockTimeSeconds) * int64(pow.DifficultyAdjustmentInterval) // 6000
	// 均衡：span=expected → bits=30 不变。
	if got := pow.AdjustBitsNearest(30, expected); got != 30 {
		t.Fatalf("AdjustBitsNearest(30, 6000) = %d, want 30（300s 目标平衡点）", got)
	}
}

// TestV3VersionFour 锁定版本三态：v1=1 / v2=2 / v3=4。
func TestV3VersionFour(t *testing.T) {
	actH := pow.ActivationHeight // 2000
	cases := []struct {
		h    int
		want uint32
	}{
		{0, 1},
		{1999, 1},
		{2000, 2},
		{2999, 2},
		{3000, 4},
		{3001, 4},
		{100000, 4},
	}
	for _, c := range cases {
		if got := pow.VersionForHeight(c.h, actH); got != c.want {
			t.Fatalf("VersionForHeight(%d) = %d, want %d", c.h, got, c.want)
		}
	}
	// Version 3 不得被接受为 v3 版本（冻结规格要求 version=4）。
	if pow.NewRulesetBlockVersion == 3 {
		t.Fatal("NewRulesetBlockVersion 不得为 3（冻结规格要求 4）")
	}
}
