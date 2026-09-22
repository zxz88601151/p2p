package blockchain_test

import (
	"errors"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

// ---- 测试辅助 ----

// testReward 是测试自带的「本高度补贴值」常量：与创世高度（0）的
// Subsidy(0) 保持一致，使 mineGenesis 产出的创世 coinbase 恰好落在共识上限上。
// C1 经济政策（A-2.3-G2）：Subsidy(0) 由 50 变为 5。
const testReward = 5

func newTestWallet(t *testing.T) *wallet.Wallet {
	t.Helper()
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("生成钱包失败: %v", err)
	}
	return w
}

func mineGenesis(t *testing.T, miner *wallet.Wallet) *block.Block {
	t.Helper()
	cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), testReward, 0)
	gb := block.NewCandidateBlock([32]byte{}, pow.MaxTargetBits, []*transaction.Transaction{cb})
	if found, _ := pow.Mine(gb); !found {
		t.Fatal("创世区块挖矿失败")
	}
	return gb
}

// mineBlock 在链尾追加一个仅含 coinbase 的新区块（真实挖出，满足 PoW）。
func mineBlock(t *testing.T, bc *blockchain.Blockchain, miner *wallet.Wallet) *block.Block {
	t.Helper()
	tip, err := bc.Tip()
	if err != nil {
		t.Fatalf("获取链尾失败: %v", err)
	}
	height := bc.Height() + 1
	cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(height), height)
	candidate := block.NewCandidateBlock(tip.Header.Hash(), bc.CurrentBits(), []*transaction.Transaction{cb})
	if found, _ := pow.Mine(candidate); !found {
		t.Fatal("区块挖矿失败")
	}
	if err := bc.AddBlock(candidate); err != nil {
		t.Fatalf("合法区块被拒绝: %v", err)
	}
	return candidate
}

// buildSpendTx 消费属于 spender 的「已成熟且金额匹配」的第一个输出，构造已签名交易。
// map 迭代无序 → 必须显式过滤未成熟 coinbase，否则选币结果不确定。
func buildSpendTx(t *testing.T, snap *utxo.UTXOSet, spender *wallet.Wallet, to [20]byte, amount, height int) *transaction.Transaction {
	t.Helper()
	for op, e := range snap.AllEntries() {
		if e.PubKeyHash != spender.PubKeyHash() || e.Value != uint64(amount) {
			continue
		}
		if e.IsCoinbase && height-e.Height < utxo.CoinbaseMaturity {
			continue
		}
		return spendFrom(t, op, spender, to, e.Value)
	}
	t.Fatal("没有满足条件（成熟且金额匹配）的可用 UTXO")
	return nil
}

// spendFrom 显式消费指定输出（不做 maturity 过滤，供非法区块构造用例使用）。
func spendFrom(t *testing.T, op utxo.OutPoint, spender *wallet.Wallet, to [20]byte, value uint64) *transaction.Transaction {
	t.Helper()
	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
		Outputs: []transaction.TxOutput{{Value: value, PubKeyHash: to}},
	}
	h := tx.Hash()
	sig, err := spender.Sign(h)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	tx.Inputs[0].Signature = sig
	tx.Inputs[0].PubKey = spender.PublicKey
	return tx
}

// ---- 用例 ----

func TestAddBlockAcceptsValidChain(t *testing.T) {
	miner := newTestWallet(t)
	genesis := mineGenesis(t, miner)
	bc, err := blockchain.NewBlockchainWithGenesis(genesis)
	if err != nil {
		t.Fatalf("初始化链失败: %v", err)
	}

	for i := 0; i < 3; i++ {
		mineBlock(t, bc, miner)
	}
	if bc.Height() != 3 {
		t.Fatalf("高度 = %d, want 3", bc.Height())
	}

	snap := bc.UTXOSnapshot()
	// 创世 coinbase(5, 高度 0，已成熟) + 3 个新 coinbase（未成熟不影响 includeImmative 统计）
	if got := snap.Balance(miner.PubKeyHash(), bc.Height(), true); got != testReward*4 {
		t.Fatalf("矿工总余额 = %d, want %d", got, testReward*4)
	}
}

func TestValidateBlockRejectsHeaderTampering(t *testing.T) {
	miner := newTestWallet(t)
	genesis := mineGenesis(t, miner)
	bc, err := blockchain.NewBlockchainWithGenesis(genesis)
	if err != nil {
		t.Fatalf("初始化链失败: %v", err)
	}
	tip, _ := bc.Tip()

	newCandidate := func() *block.Block {
		cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(1), 1)
		return block.NewCandidateBlock(tip.Header.Hash(), bc.CurrentBits(), []*transaction.Transaction{cb})
	}

	// ErrInvalidPrevHash：前置哈希错误（无需挖矿即被拒——校验顺序在 PoW 之前）
	badPrev := newCandidate()
	badPrev.Header.PrevBlockHash = [32]byte{0xEE}
	if err := bc.ValidateBlock(badPrev); !errors.Is(err, blockchain.ErrInvalidPrevHash) {
		t.Fatalf("前置哈希校验: got %v, want ErrInvalidPrevHash", err)
	}

	// ErrInvalidPoW：难度位设为不可能满足
	badPoW := newCandidate()
	badPoW.Header.Bits = 256
	if err := bc.ValidateBlock(badPoW); !errors.Is(err, blockchain.ErrInvalidPoW) {
		t.Fatalf("PoW 校验: got %v, want ErrInvalidPoW", err)
	}

	// ErrUnexpectedBits：用更简单的难度挖矿（PoW 合法但难度位不符共识）
	// 注意：这里必须相对 MaxTargetBits 取值，不能写死数字——难度常量一旦调整，
	// 写死的值可能恰好等于共识难度，导致该分支被静默跳过（本用例曾因此失效）。
	easierBits := uint32(pow.MaxTargetBits - 1)
	wrongBits := block.NewCandidateBlock(tip.Header.Hash(), easierBits, []*transaction.Transaction{
		transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(1), 1),
	})
	if found, _ := pow.Mine(wrongBits); !found {
		t.Fatal("低难度挖矿失败")
	}
	if err := bc.ValidateBlock(wrongBits); !errors.Is(err, blockchain.ErrUnexpectedBits) {
		t.Fatalf("难度位校验: got %v, want ErrUnexpectedBits", err)
	}

	// ErrTimestampOutOfRange：时间戳早于父块
	oldTs := newCandidate()
	oldTs.Header.Timestamp = tip.Header.Timestamp - 1
	if found, _ := pow.Mine(oldTs); !found {
		t.Fatal("旧时间戳挖矿失败")
	}
	if err := bc.ValidateBlock(oldTs); !errors.Is(err, blockchain.ErrTimestampOutOfRange) {
		t.Fatalf("时间戳校验: got %v, want ErrTimestampOutOfRange", err)
	}

	// ErrMerkleMismatch：挖矿后篡改交易列表
	tampered := newCandidate()
	if found, _ := pow.Mine(tampered); !found {
		t.Fatal("挖矿失败")
	}
	tampered.Transactions[0].Outputs[0].Value = 999 // 头未重算 → Merkle 不匹配
	if err := bc.ValidateBlock(tampered); !errors.Is(err, blockchain.ErrMerkleMismatch) {
		t.Fatalf("Merkle 重验: got %v, want ErrMerkleMismatch", err)
	}
}

func TestCoinbaseSpendFlow(t *testing.T) {
	miner := newTestWallet(t)
	receiver := newTestWallet(t)
	genesis := mineGenesis(t, miner)
	bc, err := blockchain.NewBlockchainWithGenesis(genesis)
	if err != nil {
		t.Fatalf("初始化链失败: %v", err)
	}

	// 挖到高度 9（genesis coinbase 在高度 10 才成熟）
	for bc.Height() < 9 {
		mineBlock(t, bc, miner)
	}

	// 高度 10 的区块包含：coinbase + 消费创世 coinbase 的交易
	tip, _ := bc.Tip()
	spend := buildSpendTx(t, bc.UTXOSnapshot(), miner, receiver.PubKeyHash(), testReward, bc.Height()+1)
	cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(10), 10)
	candidate := block.NewCandidateBlock(tip.Header.Hash(), bc.CurrentBits(),
		[]*transaction.Transaction{cb, spend})
	if found, _ := pow.Mine(candidate); !found {
		t.Fatal("区块挖矿失败")
	}
	if err := bc.AddBlock(candidate); err != nil {
		t.Fatalf("包含成熟 coinbase 花费的区块被拒绝: %v", err)
	}
	if bc.Height() != 10 {
		t.Fatalf("高度 = %d, want 10", bc.Height())
	}

	snap := bc.UTXOSnapshot()
	if got := snap.Balance(receiver.PubKeyHash(), 10, true); got != testReward {
		t.Fatalf("接收方余额 = %d, want %d", got, testReward)
	}
	// 创世输出已被消费
	genesisOP := utxo.OutPoint{Hash: genesis.Transactions[0].Hash(), Index: 0}
	if snap.Has(genesisOP) {
		t.Fatal("创世 coinbase 输出未被消费")
	}
}

func TestImmatureCoinbaseSpendRejected(t *testing.T) {
	miner := newTestWallet(t)
	receiver := newTestWallet(t)
	genesis := mineGenesis(t, miner)
	bc, err := blockchain.NewBlockchainWithGenesis(genesis)
	if err != nil {
		t.Fatalf("初始化链失败: %v", err)
	}

	// 只挖到高度 2：创世 coinbase（高度 0）尚未成熟
	mineBlock(t, bc, miner)
	mineBlock(t, bc, miner)

	tip, _ := bc.Tip()
	// 显式消费创世 coinbase（高度 0，在高度 3 处必然未成熟）
	genesisOP := utxo.OutPoint{Hash: genesis.Transactions[0].Hash(), Index: 0}
	spend := spendFrom(t, genesisOP, miner, receiver.PubKeyHash(), testReward)
	cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(3), 3)
	candidate := block.NewCandidateBlock(tip.Header.Hash(), bc.CurrentBits(),
		[]*transaction.Transaction{cb, spend})
	if found, _ := pow.Mine(candidate); !found {
		t.Fatal("区块挖矿失败")
	}
	if err := bc.ValidateBlock(candidate); err == nil {
		t.Fatal("花费未成熟 coinbase 的区块被接受")
	}
}

func TestGenesisBadTransactionRejected(t *testing.T) {
	miner := newTestWallet(t)
	// 创世 coinbase 超额 → 链初始化必须失败
	bad := block.NewCandidateBlock([32]byte{}, pow.MaxTargetBits,
		[]*transaction.Transaction{transaction.NewCoinbaseTx(miner.PubKeyHash(), testReward+1, 0)})
	pow.Mine(bad)
	if _, err := blockchain.NewBlockchainWithGenesis(bad); err == nil {
		t.Fatal("超额创世 coinbase 被接受")
	}
}

// ---- 持久化与确定性创世（PHASE 4）----

// TestGenesisDeterministic 两次生成的创世区块必须字节级一致。
func TestGenesisDeterministic(t *testing.T) {
	g1 := blockchain.NewGenesisBlock()
	g2 := blockchain.NewGenesisBlock()
	if g1.Header.Hash() != g2.Header.Hash() {
		t.Fatal("创世区块不确定（哈希不同）")
	}
	if g1.Header.Timestamp != blockchain.GenesisTimestamp {
		t.Fatalf("创世时间戳 = %d, want %d", g1.Header.Timestamp, blockchain.GenesisTimestamp)
	}
	if g1.Header.PrevBlockHash != ([32]byte{}) {
		t.Fatal("创世区块的父哈希必须为零")
	}
	if !pow.Validate(&g1.Header) {
		t.Fatal("创世区块 PoW 无效")
	}
}

// TestChainPersistsAcrossRestart 链重启后高度、链尾与 UTXO 余额一致。
func TestChainPersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}

	bc, err := blockchain.NewBlockchainFromStore(store)
	if err != nil {
		t.Fatalf("从存储加载链失败: %v", err)
	}
	if bc.Height() != 0 {
		t.Fatalf("空库应只有创世，高度 = %d", bc.Height())
	}
	miner := newTestWallet(t)
	// 直接在确定性创世之上挖 2 个区块（矿工另建钱包以拥有可花费输出）
	for i := 0; i < 2; i++ {
		mineBlock(t, bc, miner)
	}
	tipHash := mustTip(t, bc).Header.Hash()
	heightBefore := bc.Height()
	balBefore := bc.UTXOSnapshot().Balance(miner.PubKeyHash(), heightBefore, true)
	if err := store.Close(); err != nil {
		t.Fatalf("关闭存储失败: %v", err)
	}

	// 重开并加载
	store2, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("重开存储失败: %v", err)
	}
	defer store2.Close()
	bc2, err := blockchain.NewBlockchainFromStore(store2)
	if err != nil {
		t.Fatalf("重启加载链失败: %v", err)
	}
	if bc2.Height() != heightBefore {
		t.Fatalf("重启后高度 = %d, want %d", bc2.Height(), heightBefore)
	}
	if mustTip(t, bc2).Header.Hash() != tipHash {
		t.Fatal("重启后链尾哈希不一致")
	}
	if got := bc2.UTXOSnapshot().Balance(miner.PubKeyHash(), heightBefore, true); got != balBefore {
		t.Fatalf("重启后余额 = %d, want %d", got, balBefore)
	}
}

func mustTip(t *testing.T, bc *blockchain.Blockchain) *block.Block {
	t.Helper()
	tip, err := bc.Tip()
	if err != nil {
		t.Fatalf("获取链尾失败: %v", err)
	}
	return tip
}

// TestBlockByHeight 按高度取块：合法高度返回对应区块，越界返回 ErrUnknownHeight。
func TestBlockByHeight(t *testing.T) {
	miner := newTestWallet(t)
	bc, err := blockchain.NewBlockchainWithGenesis(mineGenesis(t, miner))
	if err != nil {
		t.Fatal(err)
	}
	b1 := mineBlock(t, bc, miner)
	b2 := mineBlock(t, bc, miner)

	for _, c := range []struct {
		height int
		want   *block.Block
	}{
		{0, mustHeight(t, bc, 0)},
		{1, b1},
		{2, b2},
	} {
		got, err := bc.BlockByHeight(c.height)
		if err != nil {
			t.Fatalf("高度 %d 取块失败: %v", c.height, err)
		}
		if got.Header.Hash() != c.want.Header.Hash() {
			t.Fatalf("高度 %d 取到错误区块", c.height)
		}
	}

	for _, bad := range []int{-1, 3, 100} {
		if _, err := bc.BlockByHeight(bad); !errors.Is(err, blockchain.ErrUnknownHeight) {
			t.Fatalf("越界高度 %d 未返回 ErrUnknownHeight，实际: %v", bad, err)
		}
	}
}

// TestBlocksFrom 区间取块：含起止边界、数量截断与「已到链尾」标志。
func TestBlocksFrom(t *testing.T) {
	miner := newTestWallet(t)
	bc, err := blockchain.NewBlockchainWithGenesis(mineGenesis(t, miner))
	if err != nil {
		t.Fatal(err)
	}
	mineBlock(t, bc, miner) // 高度 1
	mineBlock(t, bc, miner) // 高度 2
	mineBlock(t, bc, miner) // 高度 3

	// 从 1 开始取 2 个 → 高度 1、2；未到链尾
	got, atTip := bc.BlocksFrom(1, 2)
	if len(got) != 2 || atTip {
		t.Fatalf("BlocksFrom(1,2): 数量=%d atTip=%v, want 2/false", len(got), atTip)
	}
	if got[0].Header.Hash() != mustHeight(t, bc, 1).Header.Hash() || got[1].Header.Hash() != mustHeight(t, bc, 2).Header.Hash() {
		t.Fatal("BlocksFrom 返回的区块顺序或内容错误")
	}

	// 从 2 开始取 10 个 → 只剩高度 2、3；已到链尾
	got, atTip = bc.BlocksFrom(2, 10)
	if len(got) != 2 || !atTip {
		t.Fatalf("BlocksFrom(2,10): 数量=%d atTip=%v, want 2/true", len(got), atTip)
	}

	// 从链尾取 → 1 个且 atTip
	got, atTip = bc.BlocksFrom(3, 10)
	if len(got) != 1 || !atTip {
		t.Fatalf("BlocksFrom(3,10): 数量=%d atTip=%v, want 1/true", len(got), atTip)
	}

	// 超出链尾 → 空且 atTip（视为已同步完成，不再请求）
	got, atTip = bc.BlocksFrom(10, 10)
	if len(got) != 0 || !atTip {
		t.Fatalf("BlocksFrom(10,10): 数量=%d atTip=%v, want 0/true", len(got), atTip)
	}

	// count<=0 → 空切片，不 panic
	if got, _ := bc.BlocksFrom(1, 0); len(got) != 0 {
		t.Fatalf("BlocksFrom(1,0) 应返回空切片，实际 %d 个", len(got))
	}
	if got, _ := bc.BlocksFrom(1, -5); len(got) != 0 {
		t.Fatalf("BlocksFrom(1,-5) 应返回空切片，实际 %d 个", len(got))
	}
}

func mustHeight(t *testing.T, bc *blockchain.Blockchain, height int) *block.Block {
	t.Helper()
	b, err := bc.BlockByHeight(height)
	if err != nil {
		t.Fatalf("高度 %d 取块失败: %v", height, err)
	}
	return b
}

// ---- 难度方向：链级 runtime 验证（PHASE 0.1 §17 caveat #2 的闭环） ----

// expectedNextBits 按共识规则**独立重算**「链尾之后下一块应使用的 bits」。
// 与 Blockchain.expectedBitsFor 是两套独立实现：链级值若与它一致，说明
// 「周期起点选择 + 实际跨度计算 + 激活门控 + AdjustBits 调用」的接线正确（非空断言）。
//
// 必须走门控的 ComputeExpectedBitsAt：pre-activation 钉死父块 bits（=MaxTargetBits），
// post-activation 才真正 retarget——这保证了即使 AdjustBits 上限已抬到 32，
// 存量（pre-activation）链在每个边界高度的期望难度仍是 16（与旧实现逐字节等价）。
func expectedNextBits(t *testing.T, bc *blockchain.Blockchain, tipHeight, interval int) uint32 {
	t.Helper()
	_ = interval
	want, err := pow.ComputeExpectedBitsAt(bc, tipHeight+1, bc.ActivationHeight())
	if err != nil {
		t.Fatalf("独立重算期望难度失败: %v", err)
	}
	return want
}

// TestChainDifficultyIsPinnedAtDesignedCeilingAcrossAdjustmentBoundaries 是
// PHASE 0.1 §17 caveat #2「未通过 runtime 验证完整难度方向」的 runtime 闭环。
//
// 真实挖出跨过 ≥2 个难度调整周期（高度 20、40）的链，验证两条**不同的** clamp 路径
// 都会把链上难度固定在设计上限（= 最低难度 MaxTargetBits）：
//
//	路径① 下限 clamp：确定性创世的固定时间戳（2023-11-14）使首周期实际跨度为数年，
//	        远大于 maxTimespan(=expected×4) → target×4 > MaxTarget → 回落 MaxTargetBits。
//	路径② 上限 clamp：新创世的时间戳为当下，首周期跨度极短（< expected/4 = minTimespan）
//	        → 原始方向是「更难」（bits 本应为 MaxTargetBits+2）→ 被 MaxDifficultyBits 钳回。
//
// 因此本链难度**不浮动是有意设计**（测试网毫秒级出块优先），而非方向推导错误：
// 方向推导本身由 pow.TestAdjustBitsDirectionIsMonotonicBeforeClamp 单测锁定。
func TestChainDifficultyIsPinnedAtDesignedCeilingAcrossAdjustmentBoundaries(t *testing.T) {
	interval := pow.DifficultyAdjustmentInterval
	expected := int64(pow.TargetBlockTimeSeconds) * int64(interval)
	lastBoundary := 2 * interval // 高度 40

	cases := []struct {
		name         string
		genesis      func(t *testing.T) *block.Block
		expectSpanGt bool // 首周期跨度是否应 > maxTimespan（下限 clamp 路径）
	}{
		{
			name:         "确定性创世(固定 2023 时间戳) → 下限 clamp 路径",
			genesis:      func(t *testing.T) *block.Block { return blockchain.NewGenesisBlock() },
			expectSpanGt: true,
		},
		{
			name:         "测试创世(时间戳为当下) → 上限 clamp 路径",
			genesis:      func(t *testing.T) *block.Block { return mineGenesis(t, newTestWallet(t)) },
			expectSpanGt: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			miner := newTestWallet(t)
			genesis := tc.genesis(t)
			bc, err := blockchain.NewBlockchainWithGenesis(genesis)
			if err != nil {
				t.Fatalf("初始化链失败: %v", err)
			}
			if got := bc.CurrentBits(); got != pow.MaxTargetBits {
				t.Fatalf("创世后 CurrentBits=%d, want %d", got, pow.MaxTargetBits)
			}

			for h := 1; h <= lastBoundary; h++ {
				mineBlock(t, bc, miner)

				tip, err := bc.Tip()
				if err != nil {
					t.Fatalf("高度 %d 读取链尾失败: %v", h, err)
				}
				// 全链 bits 恒为设计上限：任何高度都不允许偏离
				if tip.Header.Bits != pow.MaxTargetBits {
					t.Fatalf("高度 %d 的区块 bits=%d，偏离设计上限 %d：本链难度不应浮动",
						h, tip.Header.Bits, pow.MaxTargetBits)
				}

				if h%interval != 0 {
					continue
				}
				// 边界高度：链级 CurrentBits 必须等于独立重算值，且等于设计上限
				want := expectedNextBits(t, bc, h, interval)
				if got := bc.CurrentBits(); got != want {
					t.Fatalf("高度 %d 边界：链级 CurrentBits=%d，独立重算=%d（接线不一致）", h, got, want)
				}
				if got := bc.CurrentBits(); got != pow.MaxTargetBits {
					t.Fatalf("高度 %d 边界：CurrentBits=%d, want %d", h, got, pow.MaxTargetBits)
				}

				if h != interval {
					continue // 只对首周期校验 clamp 路径，后续周期起点已是真实区块
				}
				periodStart, err := bc.BlockByHeight(0)
				if err != nil {
					t.Fatalf("读取创世失败: %v", err)
				}
				span := tip.Header.Timestamp - periodStart.Header.Timestamp
				isFloorPath := span > expected*4
				if tc.expectSpanGt != isFloorPath {
					t.Fatalf("首周期跨度 %d 秒未落在预期 clamp 路径上（want 下限路径=%v, got=%v）",
						span, tc.expectSpanGt, isFloorPath)
				}
			}

			if bc.Height() != lastBoundary {
				t.Fatalf("链高 = %d, want %d", bc.Height(), lastBoundary)
			}
			t.Logf("链高 %d：全链 bits 恒为 %d（期望跨度 %d 秒）",
				bc.Height(), pow.MaxTargetBits, expected)
		})
	}
}
