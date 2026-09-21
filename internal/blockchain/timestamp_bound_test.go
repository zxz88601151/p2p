package blockchain_test

// timestamp_bound_test.go —— R3（SEC-CLOSE MUST FIX 4）激活后时间戳上界的
// 边界矩阵测试（真实链 + 真实 PoW，注入激活高度 11 越过硬分叉边界）。
//
// 共识窗口（post-activation）：MTP(parent) < ts <= MTP(parent) + 7200。
// 上下边界（mtp+1 / mtp+7200）必须接受；越界（mtp / mtp+7201）必须以
// ErrTimestampOutOfRange 拒绝。MTP 随出块推进后矩阵复测一遍，保证上界
// 不是只对初始 MTP 成立。
//
// 判别性：旧实现（无上界）在 ts==mtp+7201 的「应被拒」断言处必然失败
// （远未来时间戳被接受）。

import (
	"errors"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
)

// buildTimestampBoundChain 已并入各测试函数体内（NewBlockchainWithGenesisAndActivation
// + mineBlockActivated 循环），不再单独抽 helper。

func TestTimestampPostActivationUpperBound(t *testing.T) {
	const actH = 11
	miner := newTestWallet(t)
	bc, err := blockchain.NewBlockchainWithGenesisAndActivation(mineGenesis(t, miner), actH)
	if err != nil {
		t.Fatal(err)
	}
	for h := 1; h <= 12; h++ {
		mineBlockActivated(t, bc, miner) // 推进到 post-activation
	}

	// mk 构造给定时间戳的已求解候选块（不加入链）。
	mk := func(ts int64) *block.Block {
		tip, err := bc.Tip()
		if err != nil {
			t.Fatalf("读取链尾失败: %v", err)
		}
		height := bc.Height() + 1
		cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(height), height)
		c := block.NewCandidateBlock(tip.Header.Hash(), bc.CurrentBits(), []*transaction.Transaction{cb})
		c.Header.Version = pow.NewBlockVersion
		c.Header.Timestamp = ts
		if found, _ := pow.Mine(c); !found {
			t.Fatal("采矿失败")
		}
		return c
	}

	// 两轮矩阵：第一轮基于初始 MTP；接受 mtp+7200 后 MTP 推进，第二轮复测。
	for round := 0; round < 2; round++ {
		parentH := bc.Height()
		mtp := pow.MedianTimePastAt(bc, parentH, actH)

		if err := bc.AddBlock(mk(mtp)); !errors.Is(err, blockchain.ErrTimestampOutOfRange) {
			t.Fatalf("[轮 %d] ts==MTP 应被拒 ErrTimestampOutOfRange，实际 %v", round, err)
		}
		if err := bc.AddBlock(mk(mtp + 7201)); !errors.Is(err, blockchain.ErrTimestampOutOfRange) {
			t.Fatalf("[轮 %d] ts==MTP+7201 超上界应被拒 ErrTimestampOutOfRange，实际 %v", round, err)
		}
		if err := bc.AddBlock(mk(mtp + 1)); err != nil {
			t.Fatalf("[轮 %d] ts==MTP+1（下边界）应被接受，实际 %v", round, err)
		}
		if err := bc.AddBlock(mk(mtp + 7200)); err != nil {
			t.Fatalf("[轮 %d] ts==MTP+7200（上边界）应被接受，实际 %v", round, err)
		}
		// 本轮已推进 2 块（+1 与 +7200），进入下一轮基于新 MTP 复测。
	}
}

// TestMiningTimestampWithinBound 采矿策略恒产出共识合法时间戳：
// 激活后连续挖矿过程中，MiningTimestamp 的取值必须始终落在
// [MTP(parent)+1, MTP(parent)+7200]。（激活前高度受墙钟规则约束、
// MiningTimestamp=now 恒合法，不属于本窗口断言的适用范围。）
func TestMiningTimestampWithinBound(t *testing.T) {
	const actH = 11
	miner := newTestWallet(t)
	bc, err := blockchain.NewBlockchainWithGenesisAndActivation(mineGenesis(t, miner), actH)
	if err != nil {
		t.Fatal(err)
	}
	for h := 1; h <= 15; h++ { // 12 → 跨激活，再连续 3 块验证
		height := bc.Height() + 1
		if height > actH { // 仅激活后高度受 MTP 窗口约束
			ts := bc.MiningTimestamp(height)
			mtp := pow.MedianTimePastAt(bc, height-1, actH)
			if ts <= mtp || ts > mtp+7200 {
				t.Fatalf("高度 %d 的 MiningTimestamp=%d 越出共识窗口 (%d, %d]", height, ts, mtp, mtp+7200)
			}
		}
		mineBlockActivated(t, bc, miner)
	}
}
