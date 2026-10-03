package pow

// ruleset_v3_internal_test.go —— MNC-IMPLEMENT-01 的 Nearest 取整内部单元测试。
//
// 使用 package pow（而非 pow_test）以直接测试未导出的 nearestBitsFromTarget，
// 从而精确覆盖「b0 = ceil(b_cont) = 257-BitLen 快捷式 + 单次平方整数比较」这一实现细节。
//
// 交叉验证使用两个独立 oracle：
//  1. float 近似 oracle（math.Log2 + math.Round，与实现完全不同的路径）；
//  2. 精确整数穷举 oracle（对 n 从大到小搜索，不使用 b0 快捷式）。
//
// 冻结语义（OD-13 §7 / OD-15 §6）：newBits = round(256 - log2(t))；tie 不可达
// （中点 M = 2^(256.5-b0) 为无理数）；判据 t² ≤ 2^(513-2·b0) → b0，否则 b0-1。

import (
	"math"
	"math/big"
	"testing"
)

// nearestFloatOracle 用 float64 独立估算 round(256-log2(t))。
// float64 尾数 53 位 ⇒ b_cont 误差 ~1e-14；当 |frac-0.5| < 1e-9 时结果不可靠，
// 返回 ok=false（tie 不可达，这些点本就不应成为边界）。
func nearestFloatOracle(t *big.Int) (uint32, bool) {
	f, _ := new(big.Float).SetInt(t).Float64()
	if f <= 0 || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, false
	}
	x := 256.0 - math.Log2(f)
	frac := x - math.Floor(x)
	if math.Abs(frac-0.5) < 1e-9 {
		return 0, false
	}
	return uint32(math.Round(x)), true
}

// nearestBruteOracle 用精确整数判据对 n 做**穷举搜索**（不使用 b0 快捷式）：
// 返回最大的 n 使 t² ≤ 2^(513-2n)。这与 nearestBitsFromTarget 的实现路径独立。
func nearestBruteOracle(t *big.Int) uint32 {
	t2 := new(big.Int).Mul(t, t)
	for n := 256; n >= 1; n-- {
		e := 513 - 2*n
		if e <= 0 {
			continue
		}
		mid := new(big.Int).Lsh(big.NewInt(1), uint(e))
		if t2.Cmp(mid) <= 0 {
			return uint32(n)
		}
	}
	return 1
}

// TestNearestEquilibriumAtEveryPowerOfTwo 断言均衡态精确还原：
// 对 t = T(b) = 2^(256-b)，Nearest 必须返回 b（与 Ceil 在 2 的幂处一致）。
func TestNearestEquilibriumAtEveryPowerOfTwo(t *testing.T) {
	for b := 1; b <= 64; b++ {
		tgt := BitsToTarget(uint32(b))
		if got := nearestBitsFromTarget(tgt); got != uint32(b) {
			t.Fatalf("nearestBitsFromTarget(T(%d)) = %d, want %d（均衡态必须精确还原）", b, got, b)
		}
	}
}

// TestNearestTieUnreachable 断言中点不可达：b0 处的中点 M = 2^(256.5-b0) 是无理数，
// 故 2^(513-2·b0) 永不是完全平方数 ⇒ 不存在整数 t 恰落在中点。
func TestNearestTieUnreachable(t *testing.T) {
	for b0 := 16; b0 <= 40; b0++ {
		e := 513 - 2*b0 // 奇数（513 奇 − 偶数）⇒ 2^e 非完全平方
		if e%2 == 0 {
			t.Fatalf("指数 %d 应为奇数（中点必须无理）", e)
		}
		pow2e := new(big.Int).Lsh(big.NewInt(1), uint(e))
		root := new(big.Int).Sqrt(pow2e)
		if new(big.Int).Mul(root, root).Cmp(pow2e) == 0 {
			t.Fatalf("2^%d 不应为完全平方数", e)
		}
	}
}

// TestNearestExactMidpointAdjacent 在 b0=17 处取「最贴近中点的整数对」：
// floor(M) ≤ M < floor(M)+1 ⇒ floor(M) 取 17、floor(M)+1 取 16。
// 这是对平方整数比较的最紧边界测试。
func TestNearestExactMidpointAdjacent(t *testing.T) {
	pow479 := new(big.Int).Lsh(big.NewInt(1), 479) // 2^(513-2*17)
	mf := new(big.Int).Sqrt(pow479)                // floor(M)
	if mf.BitLen() != 240 {
		t.Fatalf("floor(M) 位宽=%d，期望 240（b0 必须为 17）", mf.BitLen())
	}
	if got := nearestBitsFromTarget(mf); got != 17 {
		t.Fatalf("nearestBitsFromTarget(floor(M)) = %d, want 17", got)
	}
	above := new(big.Int).Add(mf, big.NewInt(1))
	if got := nearestBitsFromTarget(above); got != 16 {
		t.Fatalf("nearestBitsFromTarget(floor(M)+1) = %d, want 16", got)
	}
}

// TestNearestBelowAtAboveHalf 覆盖 exact / below-half / above-half 三类边界。
func TestNearestBelowAtAboveHalf(t *testing.T) {
	cases := []struct {
		name string
		tgt  *big.Int
		want uint32
	}{
		{"exact T(17)", BitsToTarget(17), 17},
		{"just above T(17) → below half → b0", new(big.Int).Add(BitsToTarget(17), big.NewInt(1)), 17},
		{"just below T(16) → above half → b0-1", new(big.Int).Sub(BitsToTarget(16), big.NewInt(1)), 16},
		{"exact T(16)", BitsToTarget(16), 16},
		{"exact T(30)", BitsToTarget(30), 30},
		{"exact T(32)", BitsToTarget(32), 32},
		{"6*2^237 (frac>0.5 → b0-1)", new(big.Int).Mul(big.NewInt(6), new(big.Int).Lsh(big.NewInt(1), 237)), 16},
	}
	for _, c := range cases {
		if got := nearestBitsFromTarget(c.tgt); got != c.want {
			t.Fatalf("%s: nearestBitsFromTarget = %d, want %d", c.name, got, c.want)
		}
	}
}

// TestNearestMatchesIndependentOracles 在 b∈[16,32) 的每个 2 的幂区间内做确定性扫描，
// 同时与 float oracle 及精确穷举 oracle 比对（三重交叉验证，无浮点边界判定）。
func TestNearestMatchesIndependentOracles(t *testing.T) {
	checked := 0
	for b := 16; b <= 31; b++ {
		lo := BitsToTarget(uint32(b + 1)) // 2^(255-b)
		hi := BitsToTarget(uint32(b))     // 2^(256-b)
		step := new(big.Int).Sub(hi, lo)
		step.Div(step, big.NewInt(997))
		if step.Sign() <= 0 {
			continue
		}
		for k := int64(1); k <= 996; k++ {
			tgt := new(big.Int).Add(lo, new(big.Int).Mul(step, big.NewInt(k)))
			if tgt.Cmp(hi) >= 0 || tgt.Sign() <= 0 {
				continue
			}
			got := nearestBitsFromTarget(tgt)
			if brute := nearestBruteOracle(tgt); got != brute {
				t.Fatalf("b=%d k=%d t=%s: impl=%d, brute=%d", b, k, tgt.String(), got, brute)
			}
			if want, ok := nearestFloatOracle(tgt); ok && got != want {
				t.Fatalf("b=%d k=%d t=%s: impl=%d, float=%d", b, k, tgt.String(), got, want)
			}
			checked++
		}
	}
	if checked < 1000 {
		t.Fatalf("交叉验证样本过少：%d", checked)
	}
}

// TestAdjustBitsNearestClampsAndEquilibrium 覆盖 floor / ceiling / 均衡态。
// V3 目标时间 = NewRulesetTargetBlockTimeSeconds(300)，expected = 300*20 = 6000。
func TestAdjustBitsNearestClampsAndEquilibrium(t *testing.T) {
	expected := int64(NewRulesetTargetBlockTimeSeconds * DifficultyAdjustmentInterval) // 6000

	// floor：极长跨度 → target 越过 MaxTarget → 钳回 MaxTargetBits。
	if got := AdjustBitsNearest(MaxTargetBits, expected*4); got != MaxTargetBits {
		t.Fatalf("floor clamp: AdjustBitsNearest(%d, %d) = %d, want %d",
			MaxTargetBits, expected*4, got, MaxTargetBits)
	}
	// ceiling：极短跨度 + 高起点 → 钳到 MaxDifficultyBits。
	if got := AdjustBitsNearest(MaxDifficultyBits, expected/4); got != MaxDifficultyBits {
		t.Fatalf("ceiling clamp: AdjustBitsNearest(%d, %d) = %d, want %d",
			MaxDifficultyBits, expected/4, got, MaxDifficultyBits)
	}
	// 均衡态：span == expected → bits 不变。
	for b := uint32(MaxTargetBits); b <= MaxDifficultyBits; b++ {
		if got := AdjustBitsNearest(b, expected); got != b {
			t.Fatalf("equilibrium: AdjustBitsNearest(%d, expected) = %d, want %d", b, got, b)
		}
	}
	// 可达区间穷举：结果必须始终落在 [MaxTargetBits, MaxDifficultyBits]。
	for cur := uint32(1); cur <= MaxDifficultyBits+4; cur++ {
		for span := expected / 4; span <= expected*4; span += expected / 4 {
			got := AdjustBitsNearest(cur, span)
			if got < MaxTargetBits || got > MaxDifficultyBits {
				t.Fatalf("AdjustBitsNearest(%d, %d) = %d 越出 [%d,%d]",
					cur, span, got, MaxTargetBits, MaxDifficultyBits)
			}
		}
	}
}

// TestNearestDiffersFromCeilAsDesigned 锁定「Nearest 抑制 Ceil overshoot」的预期差异，
// 在**同一目标时间下**（用 V2 的 60s，expected=1200）验证：span=960（=expected*4/5）
// ⇒ newTarget = 0.8·2^240 ∈ (2^239.5, 2^240)：Ceil=17，Nearest=16。
//
// 注意：本测试用 AdjustBitsNearest 时，其内部 expected=6000（300s），故不能直接用
// span=960 与 AdjustBits 对比。改为直接测 nearestBitsFromTarget 与 ceilBitsFromTarget
// 在同一 newTarget 上的差异（二者只差取整方式，与目标时间无关）。
func TestNearestDiffersFromCeilAsDesigned(t *testing.T) {
	// newTarget = 0.8·2^240 = 2^240·4/5 ∈ (2^239.5, 2^240)，BitLen=241（因为 0.8·2^240 = 2^240·4/5
	// 仍 > 2^239，最高位在 2^240 位 ⇒ BitLen=241）。
	newTarget := new(big.Int).Div(new(big.Int).Mul(BitsToTarget(16), big.NewInt(4)), big.NewInt(5))
	// 更精确：0.8·T(16) = T(16)*4/5。
	if got := ceilBitsFromTarget(newTarget); got != 17 {
		t.Fatalf("ceilBitsFromTarget(0.8·T(16)) = %d, want 17", got)
	}
	if got := nearestBitsFromTarget(newTarget); got != 16 {
		t.Fatalf("nearestBitsFromTarget(0.8·T(16)) = %d, want 16", got)
	}
}
