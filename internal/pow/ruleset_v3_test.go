package pow_test

// ruleset_v3_test.go —— MNC-IMPLEMENT-01 的 ruleset v1/v2/v3 分派、版本三态与 27 注入测试。
//
// 冻结契约（OD-14 / OD-15 / OD-15-D）：
//
//	h < 2000            v1  Ceil    钉死 16（父块 bits）
//	2000 <= h < 3000    v2  Ceil    浮动 [16,32]
//	h == 3000           v3  Nearest 注入 27（无条件，先于周期边界判断）
//	h >  3000           v3  Nearest 浮动 [16,32]
//	genesis (h == 0)    —           恒 16
//
// 全部为纯函数测试（合成 ChainView，无需挖矿）。

import (
	"errors"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
)

var errRSViewRange = errors.New("rsView: height out of range")

// rsView 是 pow.ChainView 的纯内存实现。
type rsView struct{ blocks []*block.Block }

func (v rsView) BlockByHeight(h int) (*block.Block, error) {
	if h < 0 || h >= len(v.blocks) {
		return nil, errRSViewRange
	}
	return v.blocks[h], nil
}

func (v rsView) Height() int { return len(v.blocks) - 1 }

// rsChain 生成高度 0..tip 的合成链，bits/时间戳由回调决定（块间 prevHash 链式相连）。
func rsChain(tip int, bitsFn func(int) uint32, tsFn func(int) int64) rsView {
	blocks := make([]*block.Block, 0, tip+1)
	var prev [32]byte
	for h := 0; h <= tip; h++ {
		b := &block.Block{
			Header: block.Header{
				Version:       pow.VersionForHeight(h, pow.ActivationHeight),
				PrevBlockHash: prev,
				Timestamp:     tsFn(h),
				Bits:          bitsFn(h),
				Nonce:         uint64(h), // 占位：纯函数测试不校验 PoW
			},
		}
		prev = b.Header.Hash()
		blocks = append(blocks, b)
	}
	return rsView{blocks: blocks}
}

// TestRulesetDispatchAtBoundaries 覆盖 1999 / 2000 / 2999 / 3000 / 3001 与
// v2/v3 周期边界（2020 / 3020），并证明 v2 用 Ceil、v3 用 Nearest（无 retroactive）。
func TestRulesetDispatchAtBoundaries(t *testing.T) {
	actH := pow.ActivationHeight // 2000
	// 时间戳：除 2019 / 3019 外全为 1000 ⇒ h=2020 与 h=3020 的周期跨度均为 960
	// （= expected*4/5），使 Ceil=17、Nearest=16。
	tsFn := func(h int) int64 {
		if h == 2019 || h == 3019 {
			return 1960
		}
		return 1000
	}
	bitsFn := func(h int) uint32 {
		if h == pow.NewRulesetActivationHeight {
			return pow.NewRulesetInitialBits
		}
		return 16
	}
	view := rsChain(3021, bitsFn, tsFn)

	cases := []struct {
		h    int
		want uint32
		note string
	}{
		{1999, 16, "v1 非激活 → 钉死父块 bits(16)"},
		{2000, 18, "v2 起点：周期边界 Ceil(16, span=0→clamp min)=18，且不注入 27"},
		{2999, 16, "v2 非边界 → 沿用父块 bits(16)"},
		{3000, pow.NewRulesetInitialBits, "v3 起点：无条件注入 27"},
		{3001, pow.NewRulesetInitialBits, "v3 非边界 → 沿用父块 bits(=27)"},
	}
	for _, c := range cases {
		got, err := pow.ComputeExpectedBitsAt(view, c.h, actH)
		if err != nil {
			t.Fatalf("h=%d 计算失败: %v", c.h, err)
		}
		if got != c.want {
			t.Fatalf("h=%d got=%d want=%d（%s）", c.h, got, c.want, c.note)
		}
	}

	// 无 retroactive reinterpretation：同一跨度 960 下，v2 边界用 Ceil(17)、v3 边界用 Nearest(16)。
	if got, err := pow.ComputeExpectedBitsAt(view, 2020, actH); err != nil || got != 17 {
		t.Fatalf("h=2020（v2 周期边界）应为 Ceil 值 17，实际 %d err=%v", got, err)
	}
	if got, err := pow.ComputeExpectedBitsAt(view, 3020, actH); err != nil || got != 16 {
		t.Fatalf("h=3020（v3 周期边界）应为 Nearest 值 16，实际 %d err=%v", got, err)
	}
}

// TestVersionThreeState 断言版本三态强制。
func TestVersionThreeState(t *testing.T) {
	actH := pow.ActivationHeight
	cases := []struct {
		h    int
		want uint32
	}{
		{0, pow.LegacyBlockVersion},
		{1, pow.LegacyBlockVersion},
		{1999, pow.LegacyBlockVersion},
		{2000, pow.NewBlockVersion},
		{2999, pow.NewBlockVersion},
		{3000, pow.NewRulesetBlockVersion},
		{3001, pow.NewRulesetBlockVersion},
		{100000, pow.NewRulesetBlockVersion},
	}
	for _, c := range cases {
		if got := pow.VersionForHeight(c.h, actH); got != c.want {
			t.Fatalf("VersionForHeight(%d) = %d, want %d", c.h, got, c.want)
		}
	}
	// 旧节点（v2 二进制）视角：对 v3 区块返回 2 ≠ 3 ⇒ ErrInvalidVersion 的确定性来源。
	if pow.VersionForHeight(3000, actH) == pow.NewBlockVersion {
		t.Fatal("v3 区块不得被判为 v2 版本（否则旧节点不会拒绝）")
	}
}

// TestInitialBitsIndependenceFromGenesis 断言 27 与 16 的语义独立 + genesis 恒 16。
func TestInitialBitsIndependenceFromGenesis(t *testing.T) {
	if pow.MaxTargetBits != 16 {
		t.Fatalf("MaxTargetBits 必须保持 16，实际 %d", pow.MaxTargetBits)
	}
	if pow.NewRulesetInitialBits != 27 {
		t.Fatalf("NewRulesetInitialBits 必须为 27，实际 %d", pow.NewRulesetInitialBits)
	}
	if pow.MaxTargetBits == pow.NewRulesetInitialBits {
		t.Fatal("MaxTargetBits 与 NewRulesetInitialBits 必须是独立参数")
	}
	// 同源不变量：MaxTarget() 必须仍等于 BitsToTarget(MaxTargetBits)。
	if pow.MaxTarget().Cmp(pow.BitsToTarget(pow.MaxTargetBits)) != 0 {
		t.Fatal("MaxTarget() 与 BitsToTarget(MaxTargetBits) 必须同源（拆分不得破坏）")
	}

	// genesis 恒 16；27 绝不外溢到 genesis。
	view := rsChain(3001, func(int) uint32 { return 16 }, func(h int) int64 { return int64(1000 + h) })
	if got, err := pow.ComputeExpectedBitsAt(view, 0, pow.ActivationHeight); err != nil || got != pow.MaxTargetBits {
		t.Fatalf("genesis bits=%d err=%v, want %d", got, err, pow.MaxTargetBits)
	}
	if pow.IsNewRulesetActive(0) {
		t.Fatal("genesis 不得被判为 v3 激活")
	}

	// h=3000 无条件注入 27：即使父块 bits=32、跨度极大。
	view2 := rsChain(3000, func(int) uint32 { return 32 }, func(h int) int64 { return int64(1000 + 1000*h) })
	if got, err := pow.ComputeExpectedBitsAt(view2, 3000, pow.ActivationHeight); err != nil || got != pow.NewRulesetInitialBits {
		t.Fatalf("h=3000 注入应为 27（无条件），实际 %d err=%v", got, err)
	}
}

// TestRulesetDeterminism 断言同一 (view,height) 多次调用结果完全相同（无隐藏状态/缓存）。
func TestRulesetDeterminism(t *testing.T) {
	view := rsChain(3001, func(h int) uint32 { return 16 }, func(h int) int64 { return int64(1000 + h) })
	for _, h := range []int{1999, 2000, 2999, 3000, 3001} {
		a, errA := pow.ComputeExpectedBitsAt(view, h, pow.ActivationHeight)
		b, errB := pow.ComputeExpectedBitsAt(view, h, pow.ActivationHeight)
		if errA != nil || errB != nil || a != b {
			t.Fatalf("h=%d 非确定：%d/%v vs %d/%v", h, a, errA, b, errB)
		}
	}
}
