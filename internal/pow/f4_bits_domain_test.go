package pow_test

// PHASE F-4-CONSENSUS-INPUT-HARDENING-REMEDIATION：bits 输入域回归测试。
//
// 目标（§10/§11/§15）：
//  1. 越界 bits 不再触发 2^32 bit（≈512 MiB）级 big.Int 构造；
//  2. 不 panic、不构造巨大 big.Int、分配量有界；
//  3. 合法域（[1,256]）输出与加固前逐字节一致（§7 保真）。

import (
	"math"
	"math/big"
	"runtime"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
)

// TestF4BitsToTargetDomain：全值域矩阵。BitLen 期望值同时充当「未构造巨值」的证明。
func TestF4BitsToTargetDomain(t *testing.T) {
	cases := []struct {
		bits       uint32
		wantBitLen int
	}{
		{0, 257}, // 2^256：合法算术、无放大（修复前后一致）
		{1, 256},
		{15, 242},
		{16, 241}, // MaxTargetBits
		{32, 225}, // 历史 MaxDifficultyBits（V2 时代）
		{40, 217}, // MaxDifficultyBits（V3，冻结规格）
		{255, 2},
		{256, 1}, // 域上界：target = 1（既有测试以 bits=256 断言 ErrInvalidPoW）
		// ---- 域外：F-4 加固后必须返回零目标（BitLen 0），绝不做巨量位移 ----
		{257, 0},
		{258, 0},
		{1000, 0},
		{1 << 20, 0},
		{math.MaxUint32, 0},
	}
	for _, c := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("bits=%d 触发 panic: %v", c.bits, r)
				}
			}()
			got := pow.BitsToTarget(c.bits)
			if got == nil {
				t.Fatalf("bits=%d 返回 nil", c.bits)
			}
			if bl := got.BitLen(); bl != c.wantBitLen {
				t.Fatalf("bits=%d BitLen=%d, want %d", c.bits, bl, c.wantBitLen)
			}
		}()
	}
}

// TestF4BitsToTargetValidDomainPreserved：合法域数值保真（与显式公式逐一比对）。
func TestF4BitsToTargetValidDomainPreserved(t *testing.T) {
	for bits := uint32(0); bits <= 256; bits++ {
		want := new(big.Int).Lsh(big.NewInt(1), uint(256-bits))
		got := pow.BitsToTarget(bits)
		if got.Cmp(want) != 0 {
			t.Fatalf("bits=%d 目标值发生变化: got=%s want=%s", bits, got.String(), want.String())
		}
	}
	// 常量路径：MaxTarget() 与 BitsToTarget(MaxTargetBits) 必须同源。
	if pow.MaxTarget().Cmp(pow.BitsToTarget(pow.MaxTargetBits)) != 0 {
		t.Fatal("MaxTarget() 与 BitsToTarget(MaxTargetBits) 不再同源")
	}
}

// TestF4BitsToTargetAllocationBounded：100 次越界调用总分配必须 < 1 MiB。
// （对照证据：修复前单次调用即分配 512.0 MiB，实测见阶段报告的树外探针。）
func TestF4BitsToTargetAllocationBounded(t *testing.T) {
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	for i := 0; i < 100; i++ {
		_ = pow.BitsToTarget(257)
		_ = pow.BitsToTarget(1000)
		_ = pow.BitsToTarget(math.MaxUint32)
	}
	runtime.ReadMemStats(&m2)
	if d := m2.TotalAlloc - m1.TotalAlloc; d > 1<<20 {
		t.Fatalf("300 次越界调用分配 %d B（>1 MiB）—— 守卫失效", d)
	}
}

// TestF4IsBitsInConsensusDomain：域判定真值表（与 blocktree/storage 既有判定同域）。
func TestF4IsBitsInConsensusDomain(t *testing.T) {
	cases := []struct {
		bits uint32
		want bool
	}{
		{0, false}, {1, true}, {15, true}, {16, true}, {32, true},
		{255, true}, {256, true},
		{257, false}, {1000, false}, {math.MaxUint32, false},
	}
	for _, c := range cases {
		if got := pow.IsBitsInConsensusDomain(c.bits); got != c.want {
			t.Fatalf("IsBitsInConsensusDomain(%d)=%v, want %v", c.bits, got, c.want)
		}
	}
}

// TestF4PowValidateMalformedBitsCheapFalse：越界 bits 的 PoW 判定恒为 false（零目标）。
func TestF4PowValidateMalformedBitsCheapFalse(t *testing.T) {
	for _, bits := range []uint32{257, 1000, math.MaxUint32} {
		h := block.Header{Version: 1, Bits: bits}
		if pow.Validate(&h) {
			t.Fatalf("bits=%d 的 PoW 判定为 true（零目标应使任何哈希都不满足）", bits)
		}
	}
}
