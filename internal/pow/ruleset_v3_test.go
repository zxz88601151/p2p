package pow_test

// ruleset_v3_test.go —— ruleset v1/v2/v3 分派、版本三态与 30 注入测试。
//
// 冻结契约（冻结规格 docs/PHASE-P2PCHAIN-V3-CONSENSUS-SPEC-FINAL-1.md）：
//
//	h < 2000            v1  Ceil    钉死 16（父块 bits），目标 60s
//	2000 <= h < 3000    v2  Ceil    浮动 [16,40]，目标 60s
//	h == 3000           v3  Nearest 注入 30（无条件，先于周期边界判断），目标 300s
//	h >  3000           v3  Nearest 浮动 [16,40]，目标 300s
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
// v2/v3 周期边界（2020 / 3020），并证明 v2 用 Ceil+60s、v3 用 Nearest+300s（无 retroactive）。
func TestRulesetDispatchAtBoundaries(t *testing.T) {
	actH := pow.ActivationHeight // 2000

	// 视图 A（v2 区段）：时间戳统一 1000，仅 2019 设为 2500，使 2020 的 span=1500。
	viewV2 := rsChain(2020, func(h int) uint32 { return 16 }, func(h int) int64 {
		if h == 2019 {
			return 2500 // ts(2019)-ts(2000) = 1500 = 1200*1.25（v2 偏慢）
		}
		return 1000
	})

	v2Cases := []struct {
		h    int
		want uint32
		note string
	}{
		{1999, 16, "v1 非激活 → 钉死父块 bits(16)"},
		{2000, 18, "v2 起点：周期边界 Ceil(16, span=0→clamp min 300) → newTarget=T(16)*300/1200=2^238 → ceil=18，且不注入 30"},
		{2019, 16, "v2 非边界 → 沿用父块 bits(16)"},
		{2020, 16, "v2 周期边界 span=1500 → newTarget=T(16)*1500/1200=5*2^238 → BitLen=241 → ceil=16"},
	}
	for _, c := range v2Cases {
		got, err := pow.ComputeExpectedBitsAt(viewV2, c.h, actH)
		if err != nil {
			t.Fatalf("h=%d 计算失败: %v", c.h, err)
		}
		if got != c.want {
			t.Fatalf("h=%d got=%d want=%d（%s）", c.h, got, c.want, c.note)
		}
	}

	// 视图 B（v3 区段）：3000 注入 30，3001+ 继承 30（真实链在 3000 注入后 3001..3019 均继承 30）。
	// 夹具让 h>=3000 的 bits=30，模拟注入后的继承链。
	viewV3 := rsChain(3020, func(h int) uint32 {
		if h >= pow.NewRulesetActivationHeight {
			return pow.NewRulesetInitialBits // 30（注入 + 继承）
		}
		return 16
	}, func(h int) int64 {
		if h == 3019 {
			return 8000 // ts(3019)-ts(3000) = 8000-2000 = 6000 = v3 expected
		}
		return 2000
	})

	v3Cases := []struct {
		h    int
		want uint32
		note string
	}{
		{3000, pow.NewRulesetInitialBits, "v3 起点：无条件注入 30"},
		{3001, pow.NewRulesetInitialBits, "v3 非边界 → 沿用父块 bits(=30)"},
		{3019, pow.NewRulesetInitialBits, "v3 非边界 → 沿用父块 bits(=30)"},
		{3020, pow.NewRulesetInitialBits, "v3 首重算 span=6000=均衡 → Nearest(30,6000)=T(30)*6000/6000=T(30) → Nearest=30"},
	}
	for _, c := range v3Cases {
		got, err := pow.ComputeExpectedBitsAt(viewV3, c.h, actH)
		if err != nil {
			t.Fatalf("h=%d 计算失败: %v", c.h, err)
		}
		if got != c.want {
			t.Fatalf("h=%d got=%d want=%d（%s）", c.h, got, c.want, c.note)
		}
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
	// 旧节点（v2 二进制）视角：对 v3 区块返回 2 ≠ 4 ⇒ ErrInvalidVersion 的确定性来源。
	if pow.VersionForHeight(3000, actH) == pow.NewBlockVersion {
		t.Fatal("v3 区块不得被判为 v2 版本（否则旧节点不会拒绝）")
	}
}

// TestInitialBitsIndependenceFromGenesis 断言 30 与 16 的语义独立 + genesis 恒 16。
func TestInitialBitsIndependenceFromGenesis(t *testing.T) {
	if pow.MaxTargetBits != 16 {
		t.Fatalf("MaxTargetBits 必须保持 16，实际 %d", pow.MaxTargetBits)
	}
	if pow.NewRulesetInitialBits != 30 {
		t.Fatalf("NewRulesetInitialBits 必须为 30，实际 %d", pow.NewRulesetInitialBits)
	}
	if pow.MaxTargetBits == pow.NewRulesetInitialBits {
		t.Fatal("MaxTargetBits 与 NewRulesetInitialBits 必须是独立参数")
	}
	// 同源不变量：MaxTarget() 必须仍等于 BitsToTarget(MaxTargetBits)。
	if pow.MaxTarget().Cmp(pow.BitsToTarget(pow.MaxTargetBits)) != 0 {
		t.Fatal("MaxTarget() 与 BitsToTarget(MaxTargetBits) 必须同源（拆分不得破坏）")
	}

	// genesis 恒 16；30 绝不外溢到 genesis。
	view := rsChain(3001, func(int) uint32 { return 16 }, func(h int) int64 { return int64(1000 + h) })
	if got, err := pow.ComputeExpectedBitsAt(view, 0, pow.ActivationHeight); err != nil || got != pow.MaxTargetBits {
		t.Fatalf("genesis bits=%d err=%v, want %d", got, err, pow.MaxTargetBits)
	}
	if pow.IsNewRulesetActive(0) {
		t.Fatal("genesis 不得被判为 v3 激活")
	}

	// h=3000 无条件注入 30：即使父块 bits=32、跨度极大。
	view2 := rsChain(3000, func(int) uint32 { return 32 }, func(h int) int64 { return int64(1000 + 1000*h) })
	if got, err := pow.ComputeExpectedBitsAt(view2, 3000, pow.ActivationHeight); err != nil || got != pow.NewRulesetInitialBits {
		t.Fatalf("h=3000 注入应为 30（无条件），实际 %d err=%v", got, err)
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
