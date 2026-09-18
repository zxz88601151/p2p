package blockchain_test

// PHASE F-4-CONSENSUS-INPUT-HARDENING-REMEDIATION：validateBlock「1.6 难度位共识域闸门」
// 的回归测试。
//
// 核心证据设计：**错误类**即「闸门先于目标构造」的证明 ——
//   - 若越界 bits 被拒为 ErrUnexpectedBits 且**不是** ErrInvalidPoW，
//     说明 PoW（会构造 target = 2^(256-bits) 的步骤）从未执行；
//   - 若错误类退化为 ErrInvalidPoW，则说明目标值已被构造（F-4 回归）。
//
// 保真要求（§7/§16）：bits=256 必须仍走 PoW 并返回 ErrInvalidPoW（既有测试语义不得改变）；
// 合法区块的接受集合必须完全不变。

import (
	"errors"
	"math"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

// f4Setup 构造一条可用的测试链（复用本包既有助手 newTestWallet / mineGenesis）。
func f4Setup(t *testing.T) (*blockchain.Blockchain, *wallet.Wallet) {
	t.Helper()
	miner := newTestWallet(t)
	genesis := mineGenesis(t, miner)
	bc, err := blockchain.NewBlockchainWithGenesis(genesis)
	if err != nil {
		t.Fatalf("初始化链失败: %v", err)
	}
	return bc, miner
}

// f4Candidate 构造高度 1 的候选区块（父哈希/版本合法），bits 为**对端可控**字段。
func f4Candidate(t *testing.T, bc *blockchain.Blockchain, miner *wallet.Wallet, bits uint32) *block.Block {
	t.Helper()
	tip, err := bc.Tip()
	if err != nil {
		t.Fatalf("取链尾失败: %v", err)
	}
	cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(1), 1)
	b := block.NewCandidateBlock(tip.Header.Hash(), bits, []*transaction.Transaction{cb})
	b.Header.Bits = bits // 显式声明：该字段完全由区块提供者控制
	return b
}

// TestF4ValidateBlockRejectsOutOfDomainBitsBeforePoW：越界 bits → 1.6 闸门拒绝，
// 且**从未进入** PoW（错误类不是 ErrInvalidPoW）。
func TestF4ValidateBlockRejectsOutOfDomainBitsBeforePoW(t *testing.T) {
	bc, miner := f4Setup(t)
	for _, bits := range []uint32{257, 258, 1000, 1 << 20, math.MaxUint32} {
		b := f4Candidate(t, bc, miner, bits)
		err := bc.ValidateBlock(b)
		if err == nil {
			t.Fatalf("bits=%d 未被拒绝", bits)
		}
		if errors.Is(err, blockchain.ErrInvalidPoW) {
			t.Fatalf("bits=%d 进入了 PoW 路径（说明目标值已被构造，F-4 回归）: %v", bits, err)
		}
		if !errors.Is(err, blockchain.ErrUnexpectedBits) {
			t.Fatalf("bits=%d 期望 ErrUnexpectedBits，实际 %v", bits, err)
		}
	}
}

// TestF4ValidateTemplateRejectsOutOfDomainBits：本地模板预校验路径（skipPoW=true）
// 同样受闸门保护（两条路径共享同一份规则，不存在规则漂移）。
func TestF4ValidateTemplateRejectsOutOfDomainBits(t *testing.T) {
	bc, miner := f4Setup(t)
	b := f4Candidate(t, bc, miner, 1000)
	if err := bc.ValidateTemplate(b); !errors.Is(err, blockchain.ErrUnexpectedBits) {
		t.Fatalf("模板校验期望 ErrUnexpectedBits，实际 %v", err)
	}
}

// TestF4BitsBoundaryCompatibility：边界保真。
func TestF4BitsBoundaryCompatibility(t *testing.T) {
	bc, miner := f4Setup(t)

	// 域上界 256：**必须保持**既有语义 —— 域内 ⇒ 进入 PoW ⇒ target=1 不可能满足 ⇒ ErrInvalidPoW。
	// （既有用例 TestValidateBlockRejectsHeaderTampering 以 bits=256 断言 ErrInvalidPoW。）
	b256 := f4Candidate(t, bc, miner, 256)
	if err := bc.ValidateBlock(b256); !errors.Is(err, blockchain.ErrInvalidPoW) {
		t.Fatalf("bits=256 期望 ErrInvalidPoW（既有语义不得改变），实际 %v", err)
	}

	// 零值 0：域外 ⇒ 闸门拒绝。加固前由第 3 步拒绝 —— 错误类同为 ErrUnexpectedBits，行为等价。
	b0 := f4Candidate(t, bc, miner, 0)
	if err := bc.ValidateBlock(b0); !errors.Is(err, blockchain.ErrUnexpectedBits) {
		t.Fatalf("bits=0 期望 ErrUnexpectedBits，实际 %v", err)
	}
}

// TestF4ValidConsensusUnaffected：合法路径完全不受影响（真实挖矿 + 上链）。
func TestF4ValidConsensusUnaffected(t *testing.T) {
	bc, miner := f4Setup(t)
	before := bc.Height()
	mineBlock(t, bc, miner) // 内部断言 AddBlock 成功
	if got := bc.Height(); got != before+1 {
		t.Fatalf("合法区块未被接受：高度 %d → %d", before, got)
	}
	// 链上 bits 必须仍在共识域内（正例覆盖 §11 矩阵的「normal valid bits」）
	tip, err := bc.Tip()
	if err != nil {
		t.Fatalf("取链尾失败: %v", err)
	}
	if tip.Header.Bits < 1 || tip.Header.Bits > 256 {
		t.Fatalf("已接受区块的 bits=%d 落在共识域外", tip.Header.Bits)
	}
}
