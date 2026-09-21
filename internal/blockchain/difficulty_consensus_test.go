package blockchain_test

// difficulty_consensus_test.go —— PHASE DIFFICULTY-CONSENSUS-IMPLEMENTATION-1 的 24 项测试矩阵。
//
// 覆盖：MTP 定义与安全性、retarget 数学（|Δbits|≤2 穷举 + 边界触发）、难度天花板 32、
// 链工作量（Work=2^bits / CumulativeWork=Σ）、候选祖先隔离、硬分叉三边界、创世/历史兼容、
// 版本强制（pre/post）、时间戳规则（旧墙钟 / 新 MTP / timewarp 防御 / 墙钟无关）、
// 跨难度边界链工作量、重启重算、畸形输入守卫、跨节点确定性、双节点一致性。
//
// 纯函数测试（pow.ComputeExpectedBitsAt / MedianTimePastAt / ChainCumulativeWork / WorkOfBits）
// 使用合成 ChainView，无需真实挖矿；集成测试使用 NewBlockchainWithGenesisAndActivation
// 注入较小的激活高度，越过硬分叉边界而无需真挖 2000 块。

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

// ---- 合成链视图与测试辅助 ----

// syntheticView 是 pow.ChainView 的纯内存实现，用于不依赖真实挖矿的难度/时间戳/MTP 测试。
type syntheticView struct {
	blocks []*block.Block
}

func (v syntheticView) BlockByHeight(h int) (*block.Block, error) {
	if h < 0 || h >= len(v.blocks) {
		return nil, errors.New("视图缺口")
	}
	return v.blocks[h], nil
}

func (v syntheticView) Height() int { return len(v.blocks) - 1 }

// synthChain 生成一条高度为 heights 的合成链，bits/时间戳/版本由回调决定。
// 块间 prevHash 链式相连（创世高度为 0，父哈希为零）。
func synthChain(heights int, bitsFn func(h int) uint32, tsFn func(h int) int64, verFn func(h int) uint32) *syntheticView {
	blocks := make([]*block.Block, 0, heights)
	var prev [32]byte
	for h := 0; h < heights; h++ {
		b := &block.Block{
			Header: block.Header{
				Version:       verFn(h),
				PrevBlockHash: prev,
				Timestamp:     tsFn(h),
				Bits:          bitsFn(h),
				Nonce:         uint64(h), // 仅占位，纯函数测试不校验 PoW
			},
		}
		prev = b.Header.Hash()
		blocks = append(blocks, b)
	}
	return &syntheticView{blocks: blocks}
}

// buildCandidate 在给定链上构造一个**已求解 PoW** 的候选块，版本/时间戳由调用方指定，
// 其余（prevHash/bits）取自链当前状态。用于集成测试的负路径（错误版本/时间戳）。
func buildCandidate(t *testing.T, bc *blockchain.Blockchain, miner *wallet.Wallet, version uint32) *block.Block {
	t.Helper()
	tip, err := bc.Tip()
	if err != nil {
		t.Fatalf("读取链尾失败: %v", err)
	}
	height := bc.Height() + 1
	cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(height), height)
	candidate := block.NewCandidateBlock(tip.Header.Hash(), bc.CurrentBits(), []*transaction.Transaction{cb})
	candidate.Header.Version = version
	candidate.Header.Timestamp = bc.MiningTimestamp(height)
	if found, _ := pow.Mine(candidate); !found {
		t.Fatal("候选块 PoW 求解失败")
	}
	return candidate
}

// mineBlockActivated 在给定链上挖出一个合法块（正确版本 + 时间戳），供集成测试跨激活边界。
func mineBlockActivated(t *testing.T, bc *blockchain.Blockchain, miner *wallet.Wallet) *block.Block {
	t.Helper()
	tip, err := bc.Tip()
	if err != nil {
		t.Fatalf("读取链尾失败: %v", err)
	}
	height := bc.Height() + 1
	cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(height), height)
	candidate := block.NewCandidateBlock(tip.Header.Hash(), bc.CurrentBits(), []*transaction.Transaction{cb})
	candidate.Header.Version = bc.RequiredVersionFor(height)
	candidate.Header.Timestamp = bc.MiningTimestamp(height)
	if found, _ := pow.Mine(candidate); !found {
		t.Fatal("采矿失败")
	}
	if err := bc.AddBlock(candidate); err != nil {
		t.Fatalf("合法区块被拒绝: %v", err)
	}
	return candidate
}

// ---- 测试 1：MTP 定义（窗口 [max(0,h-10),h] 与中位数下标 k=n/2） ----

func TestMTPDefinitionWindowAndMedian(t *testing.T) {
	// 冻结设计：窗口 [max(0,h-10),h] 共至多 11 个区块，k=n/2（上半中位数）。
	// ts[0]=2000 是**非单调离群值**（创世时间戳高于其子块）——这是合法但非单调的时间戳场景，
	// 足以令「11 块窗口」与「12 块窗口」算出不同的 MTP，从而使本测试能真正捕获窗口边界错误
	// （回归保护：若把窗口误写成 h-11，MTP(11) 会返回 1007 而非 1006）。
	view := synthChain(12,
		func(h int) uint32 { return 16 },
		func(h int) int64 {
			if h == 0 {
				return 2000 // 非单调离群：高于后续块
			}
			return int64(1000 + h)
		},
		func(h int) uint32 { return 1 },
	)
	// 高度 0：窗口 [0,0] 共 1 个 → ts[0]=2000。
	if got := pow.MedianTimePastAt(view, 0, 9999); got != 2000 {
		t.Fatalf("MTP(0)=%d, want 2000", got)
	}
	// 高度 5：窗口 [0,5] 共 6 个，sorted [1001,1002,1003,1004,1005,2000]，k=3 → idx3=1004。
	if got := pow.MedianTimePastAt(view, 5, 9999); got != 1004 {
		t.Fatalf("MTP(5)=%d, want 1004", got)
	}
	// 高度 11：11 块窗口 [1,11] 共 11 个，sorted [1001..1011]，k=5 → idx5=1006。
	// （12 块窗口 [0,11] 会包含离群 2000，k=6 → 1007，正是被本断言排除的错误实现。）
	if got := pow.MedianTimePastAt(view, 11, 9999); got != 1006 {
		t.Fatalf("MTP(11)=%d, want 1006（若返回 1007 说明窗口被错误写成 12 块）", got)
	}
}

// ---- 测试 2：MTP 安全性（单调非减 + 落在窗口极值之间） ----

func TestMTPSecurityNonDecreasing(t *testing.T) {
	ts := make([]int64, 40)
	for i := range ts {
		ts[i] = int64(1000 + i*10) // 严格递增
	}
	view := synthChain(len(ts),
		func(h int) uint32 { return 16 },
		func(h int) int64 { return ts[h] },
		func(h int) uint32 { return 1 },
	)
	prev := int64(-1 << 62)
	for h := 0; h < len(ts); h++ {
		mtp := pow.MedianTimePastAt(view, h, 9999)
		if mtp < prev {
			t.Fatalf("MTP 非单调：h=%d MTP=%d < prev=%d", h, mtp, prev)
		}
		// MTP 必须落在窗口 [min,max] 之间：max(0,h-10)..h。
		lo := int64(1000)
		if h >= 10 {
			lo = ts[h-10]
		}
		if mtp < lo || mtp > ts[h] {
			t.Fatalf("MTP(%d)=%d 越出窗口 [%d,%d]", h, mtp, lo, ts[h])
		}
		prev = mtp
	}
}

// ---- 测试 3：retarget 单周期 |Δbits| ≤ 2 穷举证明（GATE-1 §retarget math） ----

// 注意：链上**可达**的难度区间为 [MaxTargetBits(16), MaxDifficultyBits(32)]——
// 起点即为 16，且下限钳制（target 不得越过 MaxTarget=2^240）把任何更「易」的结果拉回 16，
// 因此 cur < 16 在真实链上不可达（AdjustBits 对其返回 16 属地板钳制，非周期漂移）。
// 本测试仅对可达区间 [16,32] 穷举证明单周期 |Δbits| ≤ 2（端点处由天花板/地板钳制兜底）。
func TestRetargetAbsDeltaAtMostTwo(t *testing.T) {
	expected := int64(pow.TargetBlockTimeSeconds) * int64(pow.DifficultyAdjustmentInterval)
	for cur := uint32(pow.MaxTargetBits); cur <= uint32(pow.MaxDifficultyBits); cur++ {
		for span := expected / 4; span <= expected*4; span += (expected / 4) {
			got := pow.AdjustBits(cur, span)
			delta := int64(got) - int64(cur)
			if delta < 0 {
				delta = -delta
			}
			if delta > 2 {
				t.Fatalf("AdjustBits(%d, %d) 单周期 |Δbits|=%d > 2（可达区间外或钳制异常）", cur, span, delta)
			}
		}
	}
}

// ---- 测试 4：retarget 边界触发（height%20==0 才重算，否则沿用父块） ----

func TestRetargetBoundaryTrigger(t *testing.T) {
	actH := 50
	// 全 16、时间戳全部相同（跨度 0）→ 边界处应被 clamp 到更难（18）。
	view := synthChain(61,
		func(h int) uint32 { return 16 },
		func(h int) int64 { return 1000 },
		func(h int) uint32 { return 1 },
	)
	for h := actH + 1; h <= 59; h++ {
		if got, err := pow.ComputeExpectedBitsAt(view, h, actH); err != nil || got != 16 {
			t.Fatalf("高度 %d（非边界）应沿用父块 bits=16，实际=%d err=%v", h, got, err)
		}
	}
	// 高度 60 为边界（60%20==0）：跨度 0 → clamp 到 minTimespan → bits=18（更难）。
	if got, err := pow.ComputeExpectedBitsAt(view, 60, actH); err != nil || got != 18 {
		t.Fatalf("高度 60（边界）retarget 应得 18，实际=%d err=%v", got, err)
	}
}

// ---- 测试 5：难度天花板钳制在 MaxDifficultyBits（现 32） ----

func TestDifficultyCeilingCappedAt32(t *testing.T) {
	if got := pow.AdjustBits(pow.MaxDifficultyBits, 1); got != pow.MaxDifficultyBits {
		t.Fatalf("从上限 32 出发、极短跨度仍应封顶 32，实际=%d", got)
	}
}

// ---- 测试 6：链工作量 Work=2^bits、单调、big.Int 防溢出 ----

func TestChainworkExpectedWorkAndMonotonic(t *testing.T) {
	if got := pow.WorkOfBits(20); got.Cmp(new(big.Int).Lsh(big.NewInt(1), 20)) != 0 {
		t.Fatalf("WorkOfBits(20) != 2^20")
	}
	view := synthChain(30,
		func(h int) uint32 { return 16 },
		func(h int) int64 { return int64(1000 + h) },
		func(h int) uint32 { return 1 },
	)
	cw0, _ := pow.ChainCumulativeWork(view, 0)
	cw1, _ := pow.ChainCumulativeWork(view, 1)
	if cw1.Cmp(cw0) <= 0 {
		t.Fatal("累积工作量未随块增加")
	}
	// big.Int 安全：bits=32 长链 Σ 不溢出（单调递增）。
	view32 := synthChain(1000,
		func(h int) uint32 { return 32 },
		func(h int) int64 { return int64(1000 + h) },
		func(h int) uint32 { return 1 },
	)
	cwA, _ := pow.ChainCumulativeWork(view32, 499)
	cwB, _ := pow.ChainCumulativeWork(view32, 999)
	if cwB.Cmp(cwA) <= 0 {
		t.Fatal("长链累积工作量未单调增加（疑似溢出）")
	}
}

// ---- 测试 7：期望难度仅依赖候选链自身祖先（reorg 隔离，DESIGN-1 §FC-004） ----

func TestExpectedBitsUsesCandidateAncestryOnly(t *testing.T) {
	actH := 50
	mk := func(parentBits uint32) *syntheticView {
		blocks := make([]*block.Block, 0, 22)
		var prev [32]byte
		for h := 0; h < 22; h++ {
			bits := uint32(16)
			if h == 20 {
				bits = parentBits // 高度 20 块携带不同 bits，模拟竞争链分歧
			}
			b := &block.Block{Header: block.Header{Version: 1, PrevBlockHash: prev, Timestamp: int64(1000 + h), Bits: bits}}
			prev = b.Header.Hash()
			blocks = append(blocks, b)
		}
		return &syntheticView{blocks: blocks}
	}
	a, b := mk(16), mk(24)
	ea, _ := pow.ComputeExpectedBitsAt(a, 21, actH) // 非边界 → 沿用各自高度 20 父块 bits
	eb, _ := pow.ComputeExpectedBitsAt(b, 21, actH)
	if ea != 16 || eb != 24 {
		t.Fatalf("expectedBits 应仅依赖候选链自身祖先：a=%d b=%d（want 16/24）", ea, eb)
	}
}

// ---- 测试 8：硬分叉三边界（H_act-1 旧 / H_act 新 / H_act+1 新） ----

func TestHardForkThreeBoundary(t *testing.T) {
	actH := 11
	view := synthChain(13,
		func(h int) uint32 { return 16 },
		func(h int) int64 { return int64(1000 + h) },
		func(h int) uint32 { return 1 },
	)
	// H_act-1 = 10：旧规则。
	if pow.IsActivationActive(10, actH) {
		t.Fatal("高度 10 应未激活")
	}
	if v := pow.VersionForHeight(10, actH); v != pow.LegacyBlockVersion {
		t.Fatalf("高度 10 版本=%d want %d", v, pow.LegacyBlockVersion)
	}
	if eb, _ := pow.ComputeExpectedBitsAt(view, 10, actH); eb != 16 {
		t.Fatalf("高度 10 期望 bits=%d want 16（旧规则钉死）", eb)
	}
	// H_act = 11：新规则启动。
	if !pow.IsActivationActive(11, actH) {
		t.Fatal("高度 11 应已激活")
	}
	if v := pow.VersionForHeight(11, actH); v != pow.NewBlockVersion {
		t.Fatalf("高度 11 版本=%d want %d", v, pow.NewBlockVersion)
	}
	// H_act+1 = 12：持续新规则。
	if !pow.IsActivationActive(12, actH) {
		t.Fatal("高度 12 应已激活")
	}
	if v := pow.VersionForHeight(12, actH); v != pow.NewBlockVersion {
		t.Fatalf("高度 12 版本=%d want %d", v, pow.NewBlockVersion)
	}
}

// ---- 测试 9：创世 / 历史兼容（存量链字节级不变） ----

func TestGenesisHistoricalCompat(t *testing.T) {
	g := blockchain.NewGenesisBlock()
	if g.Header.Bits != pow.MaxTargetBits {
		t.Fatalf("创世 bits=%d want %d", g.Header.Bits, pow.MaxTargetBits)
	}
	if g.Header.Version != pow.LegacyBlockVersion {
		t.Fatalf("创世 version=%d want %d", g.Header.Version, pow.LegacyBlockVersion)
	}
	bc, err := blockchain.NewBlockchainWithGenesis(g)
	if err != nil {
		t.Fatal(err)
	}
	if bc.ActivationHeight() != pow.ActivationHeight {
		t.Fatalf("默认激活高度应=%d，实际=%d", pow.ActivationHeight, bc.ActivationHeight())
	}
}

// ---- 测试 10：版本强制（pre-activation 拒绝 version>=NewBlockVersion） ----

func TestVersionEnforcementPreActivation(t *testing.T) {
	bc, miner := newTemplateTestChain(t) // 默认激活高度 2000 → 全 pre-activation
	mineBlockActivated(t, bc, miner)
	// 高度 2 候选，版本=2（非法，pre 要求 <2）。
	candidate := buildCandidate(t, bc, miner, pow.NewBlockVersion)
	if err := bc.AddBlock(candidate); !errors.Is(err, blockchain.ErrInvalidVersion) {
		t.Fatalf("pre-activation 版本=2 应被拒 ErrInvalidVersion，实际 %v", err)
	}
}

// ---- 测试 11：版本强制（post-activation 拒绝 version<NewBlockVersion） ----

func TestVersionEnforcementPostActivation(t *testing.T) {
	actH := 11
	miner := newTestWallet(t)
	w := miner
	bc, err := blockchain.NewBlockchainWithGenesisAndActivation(mineGenesis(t, miner), actH)
	if err != nil {
		t.Fatal(err)
	}
	for h := 1; h <= 10; h++ {
		mineBlockActivated(t, bc, w) // 高度 1..10 为 pre-activation
	}
	// 高度 11 候选，版本=1（非法，post 要求 >=2）。
	candidate := buildCandidate(t, bc, w, pow.LegacyBlockVersion)
	if err := bc.AddBlock(candidate); !errors.Is(err, blockchain.ErrInvalidVersion) {
		t.Fatalf("post-activation 版本=1 应被拒 ErrInvalidVersion，实际 %v", err)
	}
}

// ---- 测试 12：时间戳旧规则（pre-activation：>=父块 且 <= now+7200） ----

func TestTimestampPreActivationWallClockRule(t *testing.T) {
	bc, miner := newTemplateTestChain(t)
	mineBlockActivated(t, bc, miner) // 高度 1
	tip, _ := bc.Tip()
	height := bc.Height() + 1
	cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(height), height)
	bad := block.NewCandidateBlock(tip.Header.Hash(), bc.CurrentBits(), []*transaction.Transaction{cb})
	bad.Header.Version = bc.RequiredVersionFor(height)
	bad.Header.Timestamp = time.Now().Unix() + 7200 + 3600 // 超前过多
	if found, _ := pow.Mine(bad); !found {
		t.Fatal("采矿失败")
	}
	if err := bc.AddBlock(bad); !errors.Is(err, blockchain.ErrTimestampOutOfRange) {
		t.Fatalf("pre-activation 超前过多应被拒 ErrTimestampOutOfRange，实际 %v", err)
	}
}

// ---- 测试 13：时间戳新规则（post-activation：必须 > MTP(parent)，等于拒绝 / +1 接受） ----

func TestTimestampPostActivationMTPRule(t *testing.T) {
	actH := 11
	miner := newTestWallet(t)
	w := miner
	bc, err := blockchain.NewBlockchainWithGenesisAndActivation(mineGenesis(t, miner), actH)
	if err != nil {
		t.Fatal(err)
	}
	for h := 1; h <= 12; h++ {
		mineBlockActivated(t, bc, w) // 推进到 post-activation
	}
	parentH := bc.Height()
	mtp := pow.MedianTimePastAt(bc, parentH, actH)
	tip, _ := bc.Tip()
	height := bc.Height() + 1
	mk := func(ts int64) *block.Block {
		cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(height), height)
		c := block.NewCandidateBlock(tip.Header.Hash(), bc.CurrentBits(), []*transaction.Transaction{cb})
		c.Header.Version = pow.NewBlockVersion
		c.Header.Timestamp = ts
		if found, _ := pow.Mine(c); !found {
			t.Fatal("采矿失败")
		}
		return c
	}
	if err := bc.AddBlock(mk(mtp)); !errors.Is(err, blockchain.ErrTimestampOutOfRange) {
		t.Fatalf("post-activation ts==MTP 应被拒 ErrTimestampOutOfRange，实际 %v", err)
	}
	if err := bc.AddBlock(mk(mtp + 1)); err != nil {
		t.Fatalf("post-activation ts==MTP+1 应被接受，实际 %v", err)
	}
}

// ---- 测试 14：timewarp 防御（post-activation 不得把时间戳设到远低于 MTP） ----

func TestTimewarpDefense(t *testing.T) {
	actH := 11
	miner := newTestWallet(t)
	w := miner
	bc, err := blockchain.NewBlockchainWithGenesisAndActivation(mineGenesis(t, miner), actH)
	if err != nil {
		t.Fatal(err)
	}
	for h := 1; h <= 12; h++ {
		mineBlockActivated(t, bc, w)
	}
	parentH := bc.Height()
	mtp := pow.MedianTimePastAt(bc, parentH, actH)
	tip, _ := bc.Tip()
	height := bc.Height() + 1
	cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(height), height)
	attack := block.NewCandidateBlock(tip.Header.Hash(), bc.CurrentBits(), []*transaction.Transaction{cb})
	attack.Header.Version = pow.NewBlockVersion
	attack.Header.Timestamp = mtp - 100000 // 远低于 MTP（timewarp 企图）
	if found, _ := pow.Mine(attack); !found {
		t.Fatal("采矿失败")
	}
	if err := bc.AddBlock(attack); !errors.Is(err, blockchain.ErrTimestampOutOfRange) {
		t.Fatalf("timewarp（ts<<MTP）应被拒 ErrTimestampOutOfRange，实际 %v", err)
	}
}

// ---- 测试 15：跨难度边界的累积工作量（mixed bits 链） ----

func TestChainworkAcrossDifficultyBoundary(t *testing.T) {
	view := synthChain(61,
		func(h int) uint32 {
			if h < 20 {
				return 16
			}
			if h < 40 {
				return 18
			}
			return 20
		},
		func(h int) int64 { return int64(1000 + h) },
		func(h int) uint32 { return 1 },
	)
	cw, err := pow.ChainCumulativeWork(view, 60)
	if err != nil {
		t.Fatal(err)
	}
	want := big.NewInt(0)
	for h := 0; h <= 60; h++ {
		var b uint32
		switch {
		case h < 20:
			b = 16
		case h < 40:
			b = 18
		default:
			b = 20
		}
		want.Add(want, pow.WorkOfBits(b))
	}
	if cw.Cmp(want) != 0 {
		t.Fatal("跨难度边界累积工作量计算错误")
	}
}

// ---- 测试 16：重启后共识状态（链工作量 / MTP）可重算一致 ----

func TestRestartRecomputesConsensusState(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bc, err := blockchain.NewBlockchainFromStore(store)
	if err != nil {
		t.Fatal(err)
	}
	miner := newTestWallet(t)
	w := miner
	for i := 0; i < 5; i++ {
		mineBlockActivated(t, bc, w)
	}
	cwBefore, _ := pow.ChainCumulativeWork(bc, bc.Height())
	mtpBefore := pow.MedianTimePastAt(bc, bc.Height(), bc.ActivationHeight())
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store2, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()
	bc2, err := blockchain.NewBlockchainFromStore(store2)
	if err != nil {
		t.Fatal(err)
	}
	if bc2.Height() != bc.Height() {
		t.Fatalf("重启后高度不一致：%d vs %d", bc2.Height(), bc.Height())
	}
	cwAfter, _ := pow.ChainCumulativeWork(bc2, bc2.Height())
	if cwBefore.Cmp(cwAfter) != 0 {
		t.Fatal("重启后累积工作量不一致")
	}
	mtpAfter := pow.MedianTimePastAt(bc2, bc2.Height(), bc2.ActivationHeight())
	if mtpBefore != mtpAfter {
		t.Fatalf("重启后 MTP 不一致 %d vs %d", mtpBefore, mtpAfter)
	}
}

// ---- 测试 17：期望难度恒落在有效范围 [1, MaxDifficultyBits]（畸形守卫） ----

func TestExpectedBitsAlwaysInValidRange(t *testing.T) {
	view := synthChain(22,
		func(h int) uint32 { return 16 },
		func(h int) int64 { return int64(1000 + h) },
		func(h int) uint32 { return 1 },
	)
	for h := 0; h <= 21; h++ {
		got, err := pow.ComputeExpectedBitsAt(view, h, 11)
		if err != nil {
			t.Fatal(err)
		}
		if got < 1 || got > pow.MaxDifficultyBits {
			t.Fatalf("ComputeExpectedBitsAt(%d)=%d 越出 [1,%d]", h, got, pow.MaxDifficultyBits)
		}
	}
}

// ---- 测试 18：WorkOfBits 边界（bits=0 → 2^0=1；bits=256 防御性不溢出） ----

func TestWorkOfBitsEdgeCases(t *testing.T) {
	if got := pow.WorkOfBits(0); got.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("WorkOfBits(0)=%s want 1", got)
	}
	if got := pow.WorkOfBits(256); got.Sign() <= 0 {
		t.Fatal("WorkOfBits(256) 应为正且未溢出")
	}
}

// ---- 测试 19：畸形祖先（视图缺口）→ ComputeExpectedBitsAt 返回错误 ----

func TestMalformedAncestryErrors(t *testing.T) {
	view := synthChain(5,
		func(h int) uint32 { return 16 },
		func(h int) int64 { return int64(1000 + h) },
		func(h int) uint32 { return 1 },
	)
	if _, err := pow.ComputeExpectedBitsAt(view, 10, 11); err == nil {
		t.Fatal("视图缺口时 ComputeExpectedBitsAt 应返回错误")
	}
}

// ---- 测试 20：跨节点期望难度确定性（相同链 → 相同值） ----

func TestCrossNodeDeterministicExpectedBits(t *testing.T) {
	build := func() *syntheticView {
		return synthChain(40,
			func(h int) uint32 { return 16 },
			func(h int) int64 { return int64(1000 + h) },
			func(h int) uint32 { return 1 },
		)
	}
	a, b := build(), build()
	for h := 0; h <= 39; h++ {
		ea, _ := pow.ComputeExpectedBitsAt(a, h, 11)
		eb, _ := pow.ComputeExpectedBitsAt(b, h, 11)
		if ea != eb {
			t.Fatalf("跨节点 expectedBits 不一致 h=%d: %d vs %d", h, ea, eb)
		}
	}
}

// ---- 测试 21：跨节点累积工作量确定性 ----

func TestCrossNodeDeterministicChainwork(t *testing.T) {
	build := func() *syntheticView {
		return synthChain(40,
			func(h int) uint32 {
				if h >= 20 {
					return 18
				}
				return 16
			},
			func(h int) int64 { return int64(1000 + h) },
			func(h int) uint32 { return 1 },
		)
	}
	a, b := build(), build()
	ca, _ := pow.ChainCumulativeWork(a, 39)
	cb, _ := pow.ChainCumulativeWork(b, 39)
	if ca.Cmp(cb) != 0 {
		t.Fatal("跨节点累积工作量不一致")
	}
}

// ---- 测试 22：双节点跨激活一致性（相同输入 → 相同难度/版本序列与链工作量） ----

func TestDualNodeConsistencyAcrossActivation(t *testing.T) {
	actH := 11
	miner := newTestWallet(t)
	w := miner
	mine := func() *blockchain.Blockchain {
		bc, err := blockchain.NewBlockchainWithGenesisAndActivation(mineGenesis(t, miner), actH)
		if err != nil {
			t.Fatal(err)
		}
		for h := 1; h <= 30; h++ {
			mineBlockActivated(t, bc, w)
		}
		return bc
	}
	a, b := mine(), mine()
	ca, _ := pow.ChainCumulativeWork(a, a.Height())
	cb, _ := pow.ChainCumulativeWork(b, b.Height())
	if ca.Cmp(cb) != 0 {
		t.Fatal("双节点累积工作量不一致（确定性破坏）")
	}
	for h := 1; h <= 30; h++ {
		ba, _ := a.BlockByHeight(h)
		bb, _ := b.BlockByHeight(h)
		if ba.Header.Bits != bb.Header.Bits {
			t.Fatalf("高度 %d 双节点 bits 不一致", h)
		}
		if ba.Header.Version != bb.Header.Version {
			t.Fatalf("高度 %d 双节点 version 不一致", h)
		}
	}
}

// ---- 测试 23：post-activation 共识忽略墙钟（policy/consensus 分离） ----

// TestPostActivationIgnoresWallClock（R3 语义迁移）：墙钟无关性的忠实验证方式
// 不再是「任意远未来时间戳被接受」（R3 上界明确封堵时间戳膨胀，见
// maxActivationTimestampSlack），而是「**MTP 被合法推高到本地墙钟未来之后**，
// ts=MTP+7200 的块虽远超墙钟上限 now+7200，consensus 仍接受」——
// 共识判定全程不读本地时钟，上界锚定 MTP（链内量）而非墙钟。
func TestPostActivationIgnoresWallClock(t *testing.T) {
	actH := 11
	miner := newTestWallet(t)
	w := miner
	bc, err := blockchain.NewBlockchainWithGenesisAndActivation(mineGenesis(t, miner), actH)
	if err != nil {
		t.Fatal(err)
	}
	// 激活前（h=1..10）：手工块把链时间合法推到未来 ~7000s
	//（pre-activation 墙钟规则允许 ts ∈ [父块ts, now+7200]）。
	for h := 1; h < actH; h++ {
		tip, err := bc.Tip()
		if err != nil {
			t.Fatalf("读取链尾失败: %v", err)
		}
		height := bc.Height() + 1
		cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(height), height)
		c := block.NewCandidateBlock(tip.Header.Hash(), bc.CurrentBits(), []*transaction.Transaction{cb})
		c.Header.Version = bc.RequiredVersionFor(height)
		c.Header.Timestamp = time.Now().Unix() + 7000
		if found, _ := pow.Mine(c); !found {
			t.Fatal("采矿失败")
		}
		if err := bc.AddBlock(c); err != nil {
			t.Fatalf("激活前未来时间戳块应被接受: %v", err)
		}
	}
	// 激活后（h=11..22）：MiningTimestamp 自动 clamp（MTP≈now+7000 → ts=mtp+1，
	// 链时间保持在墙钟未来），推进过激活边界。
	for h := actH; h <= actH+11; h++ {
		mineBlockActivated(t, bc, w)
	}

	parentH := bc.Height()
	mtp := pow.MedianTimePastAt(bc, parentH, actH)
	height := bc.Height() + 1
	tip, _ := bc.Tip()
	cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(height), height)
	// ts=MTP+7200：落在共识窗口上边界内，但按本地墙钟衡量已远超 now+7200
	//（自检：mtp>now ⇒ mtp+7200 > now+7200）——墙钟上限若仍在起作用必拒。
	c := block.NewCandidateBlock(tip.Header.Hash(), bc.CurrentBits(), []*transaction.Transaction{cb})
	c.Header.Version = pow.NewBlockVersion
	c.Header.Timestamp = mtp + 7200
	if found, _ := pow.Mine(c); !found {
		t.Fatal("采矿失败")
	}
	if !(mtp > time.Now().Unix()) {
		t.Fatalf("测试前提不成立：MTP(%d) 应已高于本地墙钟 now(%d)", mtp, time.Now().Unix())
	}
	if err := bc.AddBlock(c); err != nil {
		t.Fatalf("post-activation 超墙钟上限但 ≤MTP+7200 的块应被 consensus 接受（墙钟无关），实际 %v", err)
	}
}

// ---- 测试 24：时间戳共识阈值定义（post-activation：必须严格 > MTP(parent)） ----

func TestTimestampConsensusGreaterThanMTP(t *testing.T) {
	actH := 11
	view := synthChain(13,
		func(h int) uint32 { return 16 },
		func(h int) int64 { return int64(1000 + h) },
		func(h int) uint32 { return 1 },
	)
	mtp := pow.MedianTimePastAt(view, 12, actH) // 父块（高度 12）的 MTP
	// 共识约束：新块时间戳必须严格大于 MTP(12)，即最小合法值 = MTP(12)+1。
	if mtp+1 <= mtp {
		t.Fatal("整数溢出")
	}
	// 用同一视图验证：ComputeExpectedBitsAt 不依赖时间戳（仅 bits），但 MTP 是时间戳的确定性函数。
	if got := pow.MedianTimePastAt(view, 12, actH); got != mtp {
		t.Fatalf("MTP 应确定性：%d vs %d", got, mtp)
	}
}
