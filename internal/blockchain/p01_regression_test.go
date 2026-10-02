package blockchain

// P0-1 回归测试（随补丁附带，验证用，不建议合入主线测试集之外的断言可按需保留）：
// PoW 无效的 fork 块不得进入 blocktree。
//
// 攻击链（修复前）：X（PoW 无效、其余合法）→ AddBlock 先入树、后校验 →
// 校验失败但节点以 StatusUnknown 残留树中 → 攻击者在 X 上挖出合法 PoW 的 Y →
// Y 的 validateForkBlock 只查 Y 自身 PoW（utxoAtNode 重放交易不验 PoW）→
// ShouldReorg 通过 → executeReorg 把含无效 PoW 的 X 拱上主链。
//
// 修复后：X 在入树前即被 validateForkHeader 拒绝，树中无残留；Y 因父未知
// 被判 orphan，主链不受任何影响。

import (
	"errors"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
)

func TestP01_InvalidPoWForkBlockNeverEntersTree(t *testing.T) {
	f := newReorgFixture(t)
	f.mineNext(pow.MaxTargetBits) // h1
	f.mineNext(pow.MaxTargetBits) // h2

	// 构造 X：除 PoW 外一切合法的 fork 块（parent=genesis，高度 1）
	cb := transaction.NewCoinbaseTx([20]byte{9}, utxo.Subsidy(1), 1)
	x := block.NewCandidateBlock(f.genesis.Header.Hash(), pow.MaxTargetBits, []*transaction.Transaction{cb})
	if found, _ := pow.Mine(x); !found {
		t.Fatal("mine X failed")
	}
	x.Header.Nonce++ // 破坏 PoW（其余头部字段保持合法）
	if pow.Validate(&x.Header) {
		t.Fatal("test setup broken: X PoW unexpectedly valid after nonce tamper")
	}
	xHash := x.Header.Hash()

	// 1. 提交 X：必须被拒绝（包装语义与既有 fork 校验失败一致）
	err := f.chain.AddBlock(x)
	if err == nil {
		t.Fatal("expected error for PoW-invalid fork block, got nil")
	}
	if !errors.Is(err, ErrInvalidPrevHash) {
		t.Fatalf("err = %v, want errors.Is(..., ErrInvalidPrevHash)", err)
	}

	// 2. 关键断言：X 不得残留在 tree 中（修复前此处为非 nil，即漏洞成立）
	if n := f.chain.tree.LookupNode(xHash); n != nil {
		t.Fatal("P0-1 回归：PoW 无效块已进入 blocktree（残留节点），攻击者可在其上接块绕过校验")
	}

	// 3. 在 X 上挖合法 PoW 的 Y 并提交：父未知 → 必须判 orphan，不能触发 reorg
	cbY := transaction.NewCoinbaseTx([20]byte{9}, utxo.Subsidy(2), 2)
	y := block.NewCandidateBlock(xHash, pow.MaxTargetBits, []*transaction.Transaction{cbY})
	if found, _ := pow.Mine(y); !found {
		t.Fatal("mine Y failed")
	}
	if !pow.Validate(&y.Header) {
		t.Fatal("test setup broken: Y PoW invalid")
	}
	if err := f.chain.AddBlock(y); err == nil {
		t.Fatal("expected orphan error for Y (parent X never entered tree), got nil")
	} else if !errors.Is(err, ErrOrphanParent) {
		t.Fatalf("Y err = %v, want errors.Is(..., ErrOrphanParent)", err)
	}

	// 4. 主链不受影响：高度仍为 2，Y 也不得入树
	if h := f.chain.Height(); h != 2 {
		t.Fatalf("canonical height = %d, want 2", h)
	}
	if n := f.chain.tree.LookupNode(y.Header.Hash()); n != nil {
		t.Fatal("Y 不应进入 tree")
	}
}
