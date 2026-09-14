// undo_test.go 实现 PHASE REORG-INFRASTRUCTURE-IMPLEMENTATION-2 (REORG-1D)
// UTXO 反向状态基础设施的完整测试矩阵（§七 + §八 12 项强制测试）。
//
// 测试覆盖：
//
//	§7.1  TestUndoCoinbaseCreation
//	§7.2  TestUndoTxSpend
//	§7.3  TestUndoMaturityRecompute
//	§7.4  TestRoundTripApplyDisconnectApply          (S1 == S1')
//	§7.5  TestUndoMidChain                           (UTXO 层 reverse primitive)
//	§7.6  TestDoubleSpendCrossBranch
//	§7.7  TestUndoLogPersistence                     (Encode/Decode round-trip)
//	§8.A  TestUndoJournalCompleteness
//	§8.B  TestUndoMultiTransactionBlock              (coinbase + 独立 + 依赖 + 同块花费)
//	§8.C  TestUndoExactStateEquality                 (完整 Outpoint+Entry 比较)
//	§8.D  TestUndoDeterminism
//	§8.E  TestUndoRejectsCorruption                  (3 子用例：缺/重/冲突)
package utxo

import (
	"bytes"
	"crypto/sha256"
	"sort"
	"testing"

	"p2pchain/internal/transaction"
	"p2pchain/internal/wallet"
)

// ---- 辅助 ----

// assertSetEqual 断言两个 UTXOSet 状态完全相等（全部 OutPoint + 全部 Entry 字段）。
// 这是 §八 TestUndoExactStateEquality 的核心断言原语：不得仅比较 Balance/Len。
func assertSetEqual(t *testing.T, label string, got, want *UTXOSet) {
	t.Helper()
	if got == nil || want == nil {
		t.Fatalf("%s: nil set (got=%v want=%v)", label, got, want)
	}
	if got.Len() != want.Len() {
		t.Fatalf("%s: Len 不一致 got=%d want=%d", label, got.Len(), want.Len())
	}
	a := got.AllEntries()
	b := want.AllEntries()
	// 全部 key 必须一致
	if len(a) != len(b) {
		t.Fatalf("%s: AllEntries 长度不一致 got=%d want=%d", label, len(a), len(b))
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			t.Fatalf("%s: outpoint %s 在 got 存在但 want 不存在", label, k)
		}
		if !entryEqual(va, vb) {
			t.Fatalf("%s: outpoint %s Entry 不一致 got=%+v want=%+v", label, k, va, vb)
		}
	}
	// 反向扫描：确保 want 中存在但 got 中不存在的 outpoint 也被捕获（防 Len 巧合相等）
	for k := range b {
		if _, ok := a[k]; !ok {
			t.Fatalf("%s: outpoint %s 在 want 存在但 got 不存在", label, k)
		}
	}
}

// buildDependentTx 构造一笔消费 exactly `op` 的交易。
// 接收方为 `spender` 自身（value+fee），fee 自定义。
// 用于构造多 tx 链式测试。
func buildDependentTx(t *testing.T, spender *wallet.Wallet, op OutPoint, value, fee uint64, height int) *transaction.Transaction {
	t.Helper()
	tx := &transaction.Transaction{
		Inputs: []transaction.TxInput{
			{PrevTxHash: op.Hash, OutIndex: op.Index},
		},
		Outputs: []transaction.TxOutput{
			{Value: value, PubKeyHash: spender.PubKeyHash()},
		},
	}
	if fee > 0 {
		// 单输出 = value+fee 给 spender；fee 由 coinbase 吸收（不再给 spender change）。
		tx.Outputs[0].Value = value + fee
	}
	signTx(t, tx, spender)
	_ = height
	return tx
}

// pubKeyHashOf 工具函数：把任意 wallet 的公钥哈希取出供测试断言。
func pubKeyHashOf(t *testing.T, w *wallet.Wallet) [20]byte {
	t.Helper()
	return w.PubKeyHash()
}

// ────────────────────────────────────────────────────────────────────────────
// §7.1 TestUndoCoinbaseCreation
// ────────────────────────────────────────────────────────────────────────────

func TestUndoCoinbaseCreation(t *testing.T) {
	base := NewUTXOSet()
	w := newTestWallet(t)

	cb := transaction.NewCoinbaseTx(pubKeyHashOf(t, w), Subsidy(0), 0)
	s1, undo, fees, err := ApplyBlockWithUndo(base, []*transaction.Transaction{cb}, 0)
	if err != nil {
		t.Fatalf("ApplyBlockWithUndo 失败: %v", err)
	}
	if fees != 0 {
		t.Fatalf("coinbase 手续费 = %d, want 0", fees)
	}

	// sanity: post-block 包含 coinbase output
	if s1.Len() != 1 {
		t.Fatalf("post-block Len = %d, want 1", s1.Len())
	}
	cbOp := outpointAt(cb, 0)
	if !s1.Has(cbOp) {
		t.Fatalf("post-block 不含 coinbase output %s", cbOp)
	}

	// DisconnectBlock 必须把 coinbase output 完全消除
	restored, err := DisconnectBlock(s1, undo)
	if err != nil {
		t.Fatalf("DisconnectBlock 失败: %v", err)
	}
	assertSetEqual(t, "coinbase undo", restored, base)
}

// ────────────────────────────────────────────────────────────────────────────
// §7.2 TestUndoTxSpend
// ────────────────────────────────────────────────────────────────────────────

func TestUndoTxSpend(t *testing.T) {
	base := NewUTXOSet()
	alice := newTestWallet(t)
	bob := newTestWallet(t)

	// alice 在 height=0 拥有一个 100 的 UTXO（pre-existing 链上输出）
	opOld := fund(t, base, [32]byte{0xAA}, 0, alice, 100, 0)
	preEntry, _ := base.Get(opOld)
	baseSnapshot := base.Clone()

	// 构造区块 h=1：消费 opOld → 给 bob 80，fee=20
	tx := &transaction.Transaction{
		Inputs: []transaction.TxInput{{PrevTxHash: opOld.Hash, OutIndex: opOld.Index}},
		Outputs: []transaction.TxOutput{
			{Value: 80, PubKeyHash: bob.PubKeyHash()},
		},
	}
	signTx(t, tx, alice)
	cb := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+20, 1)
	s1, undo, _, err := ApplyBlockWithUndo(base, []*transaction.Transaction{cb, tx}, 1)
	if err != nil {
		t.Fatalf("ApplyBlockWithUndo 失败: %v", err)
	}

	// post-block: opOld 已消失，coinbase output + bob output 存在
	if s1.Has(opOld) {
		t.Fatal("opOld 应已消失")
	}

	// Disconnect → 必须恢复 opOld 且 Entry 完全一致
	restored, err := DisconnectBlock(s1, undo)
	if err != nil {
		t.Fatalf("DisconnectBlock 失败: %v", err)
	}
	assertSetEqual(t, "tx undo", restored, baseSnapshot)

	// 进一步断言：opOld 的 Entry 完整恢复（不只是 Has）
	got, ok := restored.Get(opOld)
	if !ok {
		t.Fatalf("opOld 未恢复")
	}
	if !entryEqual(got, preEntry) {
		t.Fatalf("opOld Entry 不一致: got=%+v want=%+v", got, preEntry)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// §7.3 TestUndoMaturityRecompute
// ────────────────────────────────────────────────────────────────────────────

func TestUndoMaturityRecompute(t *testing.T) {
	// 验证：在不同 currentHeight 下查询 maturity，
	// 当我们 Disconnect 一个区块后，coinbase 输出恢复，
	// maturity 判定必须按恢复后的 height 重算。
	base := NewUTXOSet()
	alice := newTestWallet(t)

	// 区块 100：alice 收到 coinbase（Height=100, IsCoinbase=true）
	cbAt100 := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(100), 100)
	s1, undo, _, err := ApplyBlockWithUndo(base, []*transaction.Transaction{cbAt100}, 100)
	if err != nil {
		t.Fatalf("ApplyBlockWithUndo h=100 失败: %v", err)
	}
	cbOp := outpointAt(cbAt100, 0)

	// 当前 h=100：coinbase 未成熟（maturity=10，h-Height=0 < 10）
	if got := s1.Balance(alice.PubKeyHash(), 100, false /* excludeImmature */); got != 0 {
		t.Fatalf("h=100 includeImmature=false 余额 = %d, want 0", got)
	}

	// 当前 h=110：coinbase 已成熟（h-Height=10 >= 10）
	if got := s1.Balance(alice.PubKeyHash(), 110, false); got != Subsidy(100) {
		t.Fatalf("h=110 includeImmature=false 余额 = %d, want %d", got, Subsidy(100))
	}

	// 构造一笔 alice 自己的 tx 在 h=110 花费该 coinbase → 应该成功
	spendTx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: cbOp.Hash, OutIndex: cbOp.Index}},
		Outputs: []transaction.TxOutput{{Value: Subsidy(100), PubKeyHash: alice.PubKeyHash()}},
	}
	signTx(t, spendTx, alice)
	// 但因为我们还要 undo 这个花费，所以先存 s1 = post-cb100 的快照
	s1Snapshot := s1.Clone()

	// 应用「花费区块 h=110」
	s2, undo2, _, err := ApplyBlockWithUndo(s1, []*transaction.Transaction{
		transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(110), 110),
		spendTx,
	}, 110)
	if err != nil {
		t.Fatalf("ApplyBlockWithUndo h=110 失败: %v", err)
	}
	if s2.Has(cbOp) {
		t.Fatal("coinbase output 应已被 h=110 花费")
	}

	// 关键验证：DisconnectBlock(h=110) → 恢复 s1（包含 cbAt100）
	// 在恢复的 s1 上：h=100 应未成熟、h=110 应成熟
	restoredAt110, err := DisconnectBlock(s2, undo2)
	if err != nil {
		t.Fatalf("DisconnectBlock h=110 失败: %v", err)
	}
	assertSetEqual(t, "maturity recompute: restoredAt110", restoredAt110, s1Snapshot)
	if !restoredAt110.Has(cbOp) {
		t.Fatal("cbOp 应在 undo 后恢复")
	}
	if got := restoredAt110.Balance(alice.PubKeyHash(), 100, false); got != 0 {
		t.Fatalf("undo 后 h=100 余额 = %d, want 0（maturity 语义保持）", got)
	}
	if got := restoredAt110.Balance(alice.PubKeyHash(), 110, false); got != Subsidy(100) {
		t.Fatalf("undo 后 h=110 余额 = %d, want %d", got, Subsidy(100))
	}

	// 再继续：DisconnectBlock(h=100) → 完全恢复 base
	restoredAt100, err := DisconnectBlock(restoredAt110, undo)
	if err != nil {
		t.Fatalf("DisconnectBlock h=100 失败: %v", err)
	}
	assertSetEqual(t, "maturity recompute: restoredAt100", restoredAt100, base)
}

// ────────────────────────────────────────────────────────────────────────────
// §7.4 TestRoundTripApplyDisconnectApply
// ────────────────────────────────────────────────────────────────────────────

func TestRoundTripApplyDisconnectApply(t *testing.T) {
	// 必须证明：
	//   S0 + Apply(B) → S1 + undo
	//   S1 + Disconnect(undo) → S0'
	//   S0' + Apply(B) → S1'
	//   S1 == S1'  （完整 state identity，不仅是 Balance / Len）
	base := NewUTXOSet()
	alice := newTestWallet(t)
	bob := newTestWallet(t)

	// 准备 base: alice 拥有两个 UTXO
	op1 := fund(t, base, [32]byte{0xB1}, 0, alice, 100, 0)
	op2 := fund(t, base, [32]byte{0xB2}, 1, alice, 100, 0)

	// 构造区块 h=1：
	//   tx[1] 消费 op1 → 给 bob 60，fee=20
	//   tx[2] 消费 op2 → 给 bob 50，fee=30
	tx1 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op1.Hash, OutIndex: op1.Index}},
		Outputs: []transaction.TxOutput{{Value: 60, PubKeyHash: bob.PubKeyHash()}},
	}
	signTx(t, tx1, alice)
	tx2 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op2.Hash, OutIndex: op2.Index}},
		Outputs: []transaction.TxOutput{{Value: 50, PubKeyHash: bob.PubKeyHash()}},
	}
	signTx(t, tx2, alice)
	// tx1: in=100, out=60 → fee=40; tx2: in=100, out=50 → fee=50; total=90
	totalFee := uint64(40 + 50)
	cb := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+totalFee, 1)

	// 第一次 Apply
	s1, undo1, fees1, err := ApplyBlockWithUndo(base, []*transaction.Transaction{cb, tx1, tx2}, 1)
	if err != nil {
		t.Fatalf("ApplyBlockWithUndo #1 失败: %v", err)
	}
	if fees1 != totalFee {
		t.Fatalf("fees1 = %d, want %d", fees1, totalFee)
	}

	// 保留 s1 副本用于最终 S1==S1' 比较（DisconnectBlock 会原地修改 set）
	s1Snapshot := s1.Clone()

	// Disconnect → S0'（注意：DisconnectBlock 修改 s1 原地；保留 s1Snapshot 作对照）
	s0Prime, err := DisconnectBlock(s1, undo1)
	if err != nil {
		t.Fatalf("DisconnectBlock 失败: %v", err)
	}
	assertSetEqual(t, "S0'", s0Prime, base)

	// 第二次 Apply on S0' → S1'
	s1Prime, _, fees1Prime, err := ApplyBlockWithUndo(s0Prime, []*transaction.Transaction{cb, tx1, tx2}, 1)
	if err != nil {
		t.Fatalf("ApplyBlockWithUndo #2 失败: %v", err)
	}
	if fees1Prime != totalFee {
		t.Fatalf("fees1Prime = %d, want %d", fees1Prime, totalFee)
	}

	// 关键断言：S1 == S1'
	assertSetEqual(t, "S1 == S1'", s1Prime, s1Snapshot)
}

// ────────────────────────────────────────────────────────────────────────────
// §7.5 TestUndoMidChain
// ────────────────────────────────────────────────────────────────────────────

func TestUndoMidChain(t *testing.T) {
	// UTXO 层 reverse primitive：连续 3 个区块，每个区块都可独立 undo；
	// 中间区块的 undo 必须能完整恢复它的 pre-block state，而不影响其他区块。
	base := NewUTXOSet()
	alice := newTestWallet(t)
	bob := newTestWallet(t)

	// base 预置 alice 的 UTXO
	fundOp := fund(t, base, [32]byte{0xC1}, 0, alice, 100, 0)

	// 区块 h=0: 只有 coinbase（Alice 收 50）
	cb0 := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(0), 0)
	s1, undo0, _, err := ApplyBlockWithUndo(base, []*transaction.Transaction{cb0}, 0)
	if err != nil {
		t.Fatalf("h=0 失败: %v", err)
	}

	// 区块 h=1: 消费 fundOp → 给 bob 80，fee=20
	txH1 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: fundOp.Hash, OutIndex: fundOp.Index}},
		Outputs: []transaction.TxOutput{{Value: 80, PubKeyHash: bob.PubKeyHash()}},
	}
	signTx(t, txH1, alice)
	cbH1 := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+20, 1)
	s2, undo1, _, err := ApplyBlockWithUndo(s1, []*transaction.Transaction{cbH1, txH1}, 1)
	if err != nil {
		t.Fatalf("h=1 失败: %v", err)
	}

	// 区块 h=2: bob 消费 h=1 给自己 80 的输出 → 给 alice 50，fee=30
	bobOp := outpointAt(txH1, 0)
	txH2 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: bobOp.Hash, OutIndex: bobOp.Index}},
		Outputs: []transaction.TxOutput{{Value: 50, PubKeyHash: alice.PubKeyHash()}},
	}
	signTx(t, txH2, bob)
	cbH2 := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(2)+30, 2)
	s3, undo2, _, err := ApplyBlockWithUndo(s2, []*transaction.Transaction{cbH2, txH2}, 2)
	if err != nil {
		t.Fatalf("h=2 失败: %v", err)
	}

	// sanity: 三区块连续应用
	// cb0(1) + cbH1(1) + txH2 out(1) + cbH2(1) = 4
	if s3.Len() != 4 {
		t.Fatalf("s3.Len = %d, want 4", s3.Len())
	}

	// 关键验证：必须按 reverse 顺序 undo（s3 → s2 → s1 → base）
	restoredS2, err := DisconnectBlock(s3, undo2)
	if err != nil {
		t.Fatalf("DisconnectBlock h=2 失败: %v", err)
	}
	assertSetEqual(t, "S2", restoredS2, s2)

	restoredS1, err := DisconnectBlock(restoredS2, undo1)
	if err != nil {
		t.Fatalf("DisconnectBlock h=1 失败: %v", err)
	}
	assertSetEqual(t, "S1", restoredS1, s1)

	restoredBase, err := DisconnectBlock(restoredS1, undo0)
	if err != nil {
		t.Fatalf("DisconnectBlock h=0 失败: %v", err)
	}
	assertSetEqual(t, "base", restoredBase, base)
}

// ────────────────────────────────────────────────────────────────────────────
// §7.6 TestDoubleSpendCrossBranch
// ────────────────────────────────────────────────────────────────────────────

func TestDoubleSpendCrossBranch(t *testing.T) {
	// 场景：
	//   base 包含 opX
	//   Branch A: ApplyBlock(B_A) → A1（消费 opX）→ Disconnect(A1, undo_A) → base
	//   Branch B: 在恢复后的 base 上 ApplyBlock(B_B) → B1（消费 opX）
	//   验证：undo_A 不影响 B1 的可行性——即 Disconnect 完整恢复 opX。
	//
	// 注意：forward 双花是不可能的（同一 op 只能被消费一次）。
	// 本测试验证 undo 之后状态可被另一条分支独立使用。
	base := NewUTXOSet()
	alice := newTestWallet(t)
	bob := newTestWallet(t)
	opX := fund(t, base, [32]byte{0xD0}, 0, alice, 100, 0)

	// Branch A: opX → 给 bob 80，fee=20（h=1）
	txA := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: opX.Hash, OutIndex: opX.Index}},
		Outputs: []transaction.TxOutput{{Value: 80, PubKeyHash: bob.PubKeyHash()}},
	}
	signTx(t, txA, alice)
	cbA := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+20, 1)
	a1, undoA, _, err := ApplyBlockWithUndo(base, []*transaction.Transaction{cbA, txA}, 1)
	if err != nil {
		t.Fatalf("Branch A Apply 失败: %v", err)
	}
	if a1.Has(opX) {
		t.Fatal("Branch A 后 opX 应已消失")
	}

	// Disconnect Branch A → 必须恢复 opX
	restored, err := DisconnectBlock(a1, undoA)
	if err != nil {
		t.Fatalf("DisconnectBranch A 失败: %v", err)
	}
	if !restored.Has(opX) {
		t.Fatal("opX 未恢复")
	}
	assertSetEqual(t, "Branch A undo", restored, base)

	// Branch B: 在 restored 上 ApplyBlock(B_B) → 消费 opX → 给 charlie 70，fee=30
	charlie := newTestWallet(t)
	txB := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: opX.Hash, OutIndex: opX.Index}},
		Outputs: []transaction.TxOutput{{Value: 70, PubKeyHash: charlie.PubKeyHash()}},
	}
	signTx(t, txB, alice)
	cbB := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+30, 1)
	b1, _, _, err := ApplyBlockWithUndo(restored, []*transaction.Transaction{cbB, txB}, 1)
	if err != nil {
		t.Fatalf("Branch B Apply 失败: %v", err)
	}

	// 验证：Branch A 的 undo 不影响 Branch B 的成功应用
	if b1.Has(opX) {
		t.Fatal("Branch B 后 opX 应已消失")
	}
	if got := b1.Balance(charlie.PubKeyHash(), 1, true); got != 70 {
		t.Fatalf("charlie 余额 = %d, want 70", got)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// §7.7 TestUndoLogPersistence
// ────────────────────────────────────────────────────────────────────────────

func TestUndoLogPersistence(t *testing.T) {
	// 验证：BlockUndo 可以被确定性 Encode → Decode → 内容完全一致。
	// 持久化到 disk 属 REORG-1E（本阶段仅提供接口 + round-trip 验证）。
	base := NewUTXOSet()
	alice := newTestWallet(t)
	bob := newTestWallet(t)

	op1 := fund(t, base, [32]byte{0xE1}, 0, alice, 100, 0)
	op2 := fund(t, base, [32]byte{0xE2}, 1, alice, 200, 0)

	tx1 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op1.Hash, OutIndex: op1.Index}},
		Outputs: []transaction.TxOutput{{Value: 60, PubKeyHash: bob.PubKeyHash()}},
	}
	signTx(t, tx1, alice)
	tx2 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op2.Hash, OutIndex: op2.Index}},
		Outputs: []transaction.TxOutput{{Value: 150, PubKeyHash: bob.PubKeyHash()}},
	}
	signTx(t, tx2, alice)
	cb := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+(40+50), 1)
	_, undo, _, err := ApplyBlockWithUndo(base, []*transaction.Transaction{cb, tx1, tx2}, 1)
	if err != nil {
		t.Fatalf("ApplyBlockWithUndo 失败: %v", err)
	}

	// Encode
	encoded, err := EncodeUndo(undo)
	if err != nil {
		t.Fatalf("EncodeUndo 失败: %v", err)
	}
	if len(encoded) == 0 {
		t.Fatal("encoded 为空")
	}

	// Decode
	decoded, err := DecodeUndo(encoded)
	if err != nil {
		t.Fatalf("DecodeUndo 失败: %v", err)
	}

	// 关键断言：decoded 与 undo 字段完全一致
	if decoded.Height != undo.Height {
		t.Fatalf("Height 不一致: decoded=%d undo=%d", decoded.Height, undo.Height)
	}
	if len(decoded.Created) != len(undo.Created) {
		t.Fatalf("Created 长度不一致: decoded=%d undo=%d", len(decoded.Created), len(undo.Created))
	}
	if len(decoded.Spent) != len(undo.Spent) {
		t.Fatalf("Spent 长度不一致: decoded=%d undo=%d", len(decoded.Spent), len(undo.Spent))
	}
	for i := range undo.Created {
		if decoded.Created[i] != undo.Created[i] {
			t.Fatalf("Created[%d] 不一致: decoded=%+v undo=%+v", i, decoded.Created[i], undo.Created[i])
		}
	}
	for i := range undo.Spent {
		if decoded.Spent[i] != undo.Spent[i] {
			t.Fatalf("Spent[%d] 不一致: decoded=%+v undo=%+v", i, decoded.Spent[i], undo.Spent[i])
		}
	}

	// 进一步：解码后的 undo 应当能用于 DisconnectBlock
	// 重新 ApplyBlockWithUndo（需要 fresh working set）→ Disconnect with decoded undo
	freshBase := base.Clone()
	s1, _, _, err := ApplyBlockWithUndo(freshBase, []*transaction.Transaction{cb, tx1, tx2}, 1)
	if err != nil {
		t.Fatalf("fresh ApplyBlockWithUndo 失败: %v", err)
	}
	if _, err := DisconnectBlock(s1, decoded); err != nil {
		t.Fatalf("DisconnectBlock with decoded undo 失败: %v", err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// §8.A TestUndoJournalCompleteness
// ────────────────────────────────────────────────────────────────────────────

func TestUndoJournalCompleteness(t *testing.T) {
	// 验证 BlockUndo 本身足以把 set 从 post-block 恢复到 pre-block，
	// 不需要重新执行 ApplyBlock 或访问任何外部状态。
	base := NewUTXOSet()
	alice := newTestWallet(t)
	bob := newTestWallet(t)

	op1 := fund(t, base, [32]byte{0xF1}, 0, alice, 100, 0)
	op2 := fund(t, base, [32]byte{0xF2}, 1, alice, 200, 0)

	tx1 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op1.Hash, OutIndex: op1.Index}},
		Outputs: []transaction.TxOutput{{Value: 60, PubKeyHash: bob.PubKeyHash()}},
	}
	signTx(t, tx1, alice)
	tx2 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op2.Hash, OutIndex: op2.Index}},
		Outputs: []transaction.TxOutput{{Value: 150, PubKeyHash: bob.PubKeyHash()}},
	}
	signTx(t, tx2, alice)
	cb := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+(40+50), 1)
	s1, undo, _, err := ApplyBlockWithUndo(base, []*transaction.Transaction{cb, tx1, tx2}, 1)
	if err != nil {
		t.Fatalf("ApplyBlockWithUndo 失败: %v", err)
	}

	// completeness 自证：undo 内部应当包含本次 Apply 的全部信息：
	//  - Spent 中恰好包含 op1, op2 的 (OutPoint, Entry)
	//  - Created 中应当包含所有新增 outpoint（coinbase + bob 两次收 + change 等）
	if len(undo.Spent) != 2 {
		t.Fatalf("Spent 长度 = %d, want 2", len(undo.Spent))
	}
	spentOps := make(map[OutPoint]bool)
	for _, ue := range undo.Spent {
		spentOps[ue.OutPoint] = true
	}
	if !spentOps[op1] {
		t.Fatal("undo.Spent 缺 op1")
	}
	if !spentOps[op2] {
		t.Fatal("undo.Spent 缺 op2")
	}

	// Created 至少包含：coinbase 1 + bob tx1 output 1 + bob tx2 output 1 = 3
	if len(undo.Created) < 3 {
		t.Fatalf("Created 长度 = %d, want >= 3", len(undo.Created))
	}

	// Disconnect 验证：必须能完整恢复 base
	restored, err := DisconnectBlock(s1, undo)
	if err != nil {
		t.Fatalf("DisconnectBlock 失败: %v", err)
	}
	assertSetEqual(t, "completeness", restored, base)
}

// ────────────────────────────────────────────────────────────────────────────
// §8.B TestUndoMultiTransactionBlock
// ────────────────────────────────────────────────────────────────────────────

func TestUndoMultiTransactionBlock(t *testing.T) {
	// 区块包含：
	//   coinbase (Alice, 50)
	//   tx[1] 独立花费：opOld → 给 bob 60，change 30 给 alice（fee=10）
	//   tx[2] 依赖：消费 tx[1] 给 bob 的输出 → 给 charlie 50（fee=10）
	//   tx[3] 依赖：消费 tx[2] 给 charlie 的输出 → 给 alice 40（fee=10）
	// 所有 tx 都依赖前一 tx 的输出（同区块链）。
	base := NewUTXOSet()
	alice := newTestWallet(t)
	bob := newTestWallet(t)
	charlie := newTestWallet(t)

	opOld := fund(t, base, [32]byte{0xA0}, 0, alice, 100, 0)
	preEntry, _ := base.Get(opOld)

	// tx[1]: opOld → bob 60 + alice change 30, fee=10
	tx1 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: opOld.Hash, OutIndex: opOld.Index}},
		Outputs: []transaction.TxOutput{{Value: 60, PubKeyHash: bob.PubKeyHash()}},
	}
	signTx(t, tx1, alice)

	// 取得 tx1 的 hash → bobOp
	bobOp := outpointAt(tx1, 0)

	// tx[2]: bobOp → charlie 50, fee=10
	tx2 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: bobOp.Hash, OutIndex: bobOp.Index}},
		Outputs: []transaction.TxOutput{{Value: 50, PubKeyHash: charlie.PubKeyHash()}},
	}
	signTx(t, tx2, bob)
	charlieOp := outpointAt(tx2, 0)

	// tx[3]: charlieOp → alice 40, fee=10
	tx3 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: charlieOp.Hash, OutIndex: charlieOp.Index}},
		Outputs: []transaction.TxOutput{{Value: 40, PubKeyHash: alice.PubKeyHash()}},
	}
	signTx(t, tx3, charlie)
	aliceTx3Op := outpointAt(tx3, 0)

	// tx1: in=100, out=60 → fee=40
	// tx2: in=60 (bobOp), out=50 → fee=10
	// tx3: in=50 (charlieOp), out=40 → fee=10
	// total: 60
	totalFee := uint64(40 + 10 + 10)
	cb := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+totalFee, 1)

	s1, undo, fees, err := ApplyBlockWithUndo(base, []*transaction.Transaction{cb, tx1, tx2, tx3}, 1)
	if err != nil {
		t.Fatalf("ApplyBlockWithUndo 失败: %v", err)
	}
	if fees != totalFee {
		t.Fatalf("fees = %d, want %d", fees, totalFee)
	}

	// post-block 状态：
	//   opOld 已消失（在 Spent）
	//   bobOp 已消失（被 tx[2] 消费；不在 Created，因同块内被消费）
	//   charlieOp 已消失（被 tx[3] 消费；不在 Created）
	//   aliceTx3Op 存在（Created）
	//   cbOp 存在（Created）
	if s1.Has(opOld) {
		t.Fatal("opOld 应消失")
	}
	if s1.Has(bobOp) {
		t.Fatal("bobOp 应消失（被同块 tx[2] 消费）")
	}
	if s1.Has(charlieOp) {
		t.Fatal("charlieOp 应消失（被同块 tx[3] 消费）")
	}
	if !s1.Has(aliceTx3Op) {
		t.Fatal("aliceTx3Op 应存活")
	}
	cbOp2 := outpointAt(cb, 0)
	if !s1.Has(cbOp2) {
		t.Fatal("cbOp 应存活")
	}

	// 关键断言：undo 中不能包含 bobOp/charlieOp（它们同块被消费）。
	for _, ue := range undo.Spent {
		if ue.OutPoint == bobOp {
			t.Fatal("bobOp 不应在 Spent 中（pre-block 不存在）")
		}
		if ue.OutPoint == charlieOp {
			t.Fatal("charlieOp 不应在 Spent 中（pre-block 不存在）")
		}
	}
	for _, ue := range undo.Created {
		if ue.OutPoint == bobOp {
			t.Fatal("bobOp 不应在 Created 中（post-block 不存活）")
		}
		if ue.OutPoint == charlieOp {
			t.Fatal("charlieOp 不应在 Created 中（post-block 不存活）")
		}
	}

	// Spent 中应当只有 opOld
	if len(undo.Spent) != 1 {
		t.Fatalf("Spent 长度 = %d, want 1（仅 opOld）", len(undo.Spent))
	}
	if undo.Spent[0].OutPoint != opOld {
		t.Fatalf("Spent[0] = %s, want opOld", undo.Spent[0].OutPoint)
	}
	if !entryEqual(undo.Spent[0].Entry, preEntry) {
		t.Fatal("Spent[0].Entry 与 pre-block Entry 不一致")
	}

	// Created 中应当有 cbOp + aliceTx3Op（共 2 项；无 change 因为 tx1/2/3 都是单输出）
	if len(undo.Created) != 2 {
		t.Fatalf("Created 长度 = %d, want 2", len(undo.Created))
	}
	// Created 排序后应为：[aliceTx3Op, cbOp2]（按 OutPoint 字典序）
	// 排序顺序依赖 hash 字典序；显式断言它们都在集合内
	hasAliceTx3 := false
	hasCb := false
	for _, ue := range undo.Created {
		if ue.OutPoint == aliceTx3Op {
			hasAliceTx3 = true
			if ue.Entry.IsCoinbase {
				t.Fatal("aliceTx3Op Entry IsCoinbase 应为 false")
			}
			if ue.Entry.Height != 1 {
				t.Fatalf("aliceTx3Op Entry.Height = %d, want 1", ue.Entry.Height)
			}
		}
		if ue.OutPoint == cbOp2 {
			hasCb = true
			if !ue.Entry.IsCoinbase {
				t.Fatal("cbOp2 Entry IsCoinbase 应为 true")
			}
		}
	}
	if !hasAliceTx3 {
		t.Fatal("Created 缺 aliceTx3Op")
	}
	if !hasCb {
		t.Fatal("Created 缺 cbOp2")
	}

	// Disconnect → base
	restored, err := DisconnectBlock(s1, undo)
	if err != nil {
		t.Fatalf("DisconnectBlock 失败: %v", err)
	}
	assertSetEqual(t, "multi-tx undo", restored, base)
}

// ────────────────────────────────────────────────────────────────────────────
// §8.C TestUndoExactStateEquality
// ────────────────────────────────────────────────────────────────────────────

func TestUndoExactStateEquality(t *testing.T) {
	// §八 强约束：不得仅比较 Balance/Len/单个 Get。
	// 必须比较全部 OutPoint + 全部 Entry 字段 + 确定性排序后的完整集合。
	//
	// 这里通过「构造两个不同的 base，碰巧有相同 Balance，但 outpoint 不同」
	// 来证明仅比较 Balance 会漏报差异。
	base1 := NewUTXOSet()
	alice := newTestWallet(t)
	bob := newTestWallet(t)

	opA := fund(t, base1, [32]byte{0xA1}, 0, alice, 50, 0)
	opB := fund(t, base1, [32]byte{0xB1}, 0, alice, 50, 0)
	// base1 有两个 50 的 UTXO；Balance(alice, 0, true) = 100

	base2 := NewUTXOSet()
	opC := fund(t, base2, [32]byte{0xC1}, 0, alice, 100, 0)
	// base2 有一个 100 的 UTXO；Balance(alice, 0, true) = 100
	// —— Balance 与 base1 相同，但 outpoint 完全不同。

	if base1.Balance(alice.PubKeyHash(), 0, true) != base2.Balance(alice.PubKeyHash(), 0, true) {
		t.Fatal("setup 错：Balance 不等")
	}

	// outpoint 必须互不相交（证明 Balance+Len 不能区分）
	a := base1.AllEntries()
	b := base2.AllEntries()
	for k := range a {
		if _, ok := b[k]; ok {
			t.Fatalf("base1 与 base2 共享 outpoint %s", k)
		}
	}
	for k := range b {
		if _, ok := a[k]; ok {
			t.Fatalf("base2 与 base1 共享 outpoint %s", k)
		}
	}
	_ = opA
	_ = opB
	_ = opC

	// 真正的断言：对 Apply + Disconnect 之后的状态做完整比较
	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: opA.Hash, OutIndex: opA.Index}},
		Outputs: []transaction.TxOutput{{Value: 30, PubKeyHash: bob.PubKeyHash()}},
	}
	signTx(t, tx, alice)
	cb := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+20, 1)
	s1, undo, _, err := ApplyBlockWithUndo(base1, []*transaction.Transaction{cb, tx}, 1)
	if err != nil {
		t.Fatalf("ApplyBlockWithUndo 失败: %v", err)
	}

	// Disconnect → 必须 == base1
	restored, err := DisconnectBlock(s1, undo)
	if err != nil {
		t.Fatalf("DisconnectBlock 失败: %v", err)
	}
	assertSetEqual(t, "restored == base1", restored, base1)

	// 进一步：restored 应当 != base2（即使 Balance/Len 巧合相等）
	// 通过 outpoint 集合互不相交证明：
	for k := range restored.AllEntries() {
		if _, ok := base2.AllEntries()[k]; ok {
			t.Fatalf("restored 与 base2 共享 outpoint %s（说明 undo 错误地恢复到了 base2）", k)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// §8.D TestUndoDeterminism
// ────────────────────────────────────────────────────────────────────────────

func TestUndoDeterminism(t *testing.T) {
	// 同一 (prior, txs, height) 多次调用 ApplyBlockWithUndo → undo 完全一致。
	base := NewUTXOSet()
	alice := newTestWallet(t)
	bob := newTestWallet(t)
	op1 := fund(t, base, [32]byte{0x11}, 0, alice, 100, 0)
	op2 := fund(t, base, [32]byte{0x22}, 1, alice, 100, 0)

	tx1 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op1.Hash, OutIndex: op1.Index}},
		Outputs: []transaction.TxOutput{{Value: 70, PubKeyHash: bob.PubKeyHash()}},
	}
	signTx(t, tx1, alice)
	tx2 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op2.Hash, OutIndex: op2.Index}},
		Outputs: []transaction.TxOutput{{Value: 70, PubKeyHash: bob.PubKeyHash()}},
	}
	signTx(t, tx2, alice)
	cb := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+(30+30), 1)

	// 第一次
	_, undoA, _, err := ApplyBlockWithUndo(base, []*transaction.Transaction{cb, tx1, tx2}, 1)
	if err != nil {
		t.Fatalf("ApplyBlockWithUndo #1 失败: %v", err)
	}

	// 第二次（fresh base clone）
	baseFresh := base.Clone()
	_, undoB, _, err := ApplyBlockWithUndo(baseFresh, []*transaction.Transaction{cb, tx1, tx2}, 1)
	if err != nil {
		t.Fatalf("ApplyBlockWithUndo #2 失败: %v", err)
	}

	// 第三次（调换 tx 顺序：先 tx2 后 tx1）
	baseFresh2 := base.Clone()
	_, undoC, _, err := ApplyBlockWithUndo(baseFresh2, []*transaction.Transaction{cb, tx2, tx1}, 1)
	if err != nil {
		t.Fatalf("ApplyBlockWithUndo #3 失败: %v", err)
	}

	// undoA == undoB（同一顺序）
	if undoA.Height != undoB.Height ||
		len(undoA.Created) != len(undoB.Created) ||
		len(undoA.Spent) != len(undoB.Spent) {
		t.Fatalf("undoA 与 undoB 不一致")
	}
	for i := range undoA.Created {
		if undoA.Created[i] != undoB.Created[i] {
			t.Fatalf("undoA.Created[%d] != undoB.Created[%d]", i, i)
		}
	}
	for i := range undoA.Spent {
		if undoA.Spent[i] != undoB.Spent[i] {
			t.Fatalf("undoA.Spent[%d] != undoB.Spent[%d]", i, i)
		}
	}

	// undoA == undoC（不同 tx 顺序——Created 是按 OutPoint 排序的，应相同；
	//                Spent 是按 tx 顺序的，可能不同！）
	// Created 应相同：
	for i := range undoA.Created {
		if undoA.Created[i] != undoC.Created[i] {
			t.Fatalf("undoA.Created[%d] != undoC.Created[%d]（按 OutPoint 排序应一致）", i, i)
		}
	}

	// Spent 顺序可能不同但内容应一致（排序后比较）
	undoASpentSorted := make([]UndoEntry, len(undoA.Spent))
	copy(undoASpentSorted, undoA.Spent)
	sort.Slice(undoASpentSorted, func(i, j int) bool {
		return outPointLess(undoASpentSorted[i].OutPoint, undoASpentSorted[j].OutPoint)
	})
	undoCSpentSorted := make([]UndoEntry, len(undoC.Spent))
	copy(undoCSpentSorted, undoC.Spent)
	sort.Slice(undoCSpentSorted, func(i, j int) bool {
		return outPointLess(undoCSpentSorted[i].OutPoint, undoCSpentSorted[j].OutPoint)
	})
	for i := range undoASpentSorted {
		if undoASpentSorted[i] != undoCSpentSorted[i] {
			t.Fatalf("undoA/C Spent 排序后[%d] 不一致", i)
		}
	}

	// 进一步：EncodeUndo 应当也确定性
	encA, _ := EncodeUndo(undoA)
	encB, _ := EncodeUndo(undoB)
	if !bytes.Equal(encA, encB) {
		t.Fatal("EncodeUndo 不确定性")
	}
	_ = sha256.Sum256 // 避免 unused import 警告（如果不需要的话）
}

// ────────────────────────────────────────────────────────────────────────────
// §8.E TestUndoRejectsCorruption
// ────────────────────────────────────────────────────────────────────────────

func TestUndoRejectsCorruption(t *testing.T) {
	// 三个子用例：
	//   (1) 缺少 entry（Created/Spent 数量被人为减少）
	//   (2) 重复 created outpoint（人为让 Created 中某个 outpoint 已在 set 中）
	//   (3) 恢复冲突 outpoint（Spent 中某个 outpoint 已在 set 中）
	//
	// 任何一种 corruption 都必须明确失败，绝不静默继续。

	base := NewUTXOSet()
	alice := newTestWallet(t)
	bob := newTestWallet(t)
	op1 := fund(t, base, [32]byte{0xA1}, 0, alice, 100, 0)
	op2 := fund(t, base, [32]byte{0xA2}, 1, alice, 100, 0)

	tx1 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op1.Hash, OutIndex: op1.Index}},
		Outputs: []transaction.TxOutput{{Value: 80, PubKeyHash: bob.PubKeyHash()}},
	}
	signTx(t, tx1, alice)
	cb := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+20, 1)
	s1, undo, _, err := ApplyBlockWithUndo(base, []*transaction.Transaction{cb, tx1}, 1)
	if err != nil {
		t.Fatalf("ApplyBlockWithUndo 失败: %v", err)
	}

	// ─── (1) Created 缺一项：截短 Created slice
	t.Run("missing-created-entry", func(t *testing.T) {
		corrupted := undo
		if len(corrupted.Created) > 0 {
			corrupted.Created = corrupted.Created[:len(corrupted.Created)-1]
		}
		// Disconnect 应当失败：剩余 Created 中的某项无法删除（已不存在），
		// 或更早的 Created 项已被篡改导致 Entry mismatch。
		if _, err := DisconnectBlock(s1, corrupted); err == nil {
			t.Fatal("corrupted Created（缺项）未触发错误")
		}
	})

	// ─── (2) Created 重复 outpoint：人为在 Created 中追加一个 set 已存在的 outpoint
	t.Run("duplicate-created-outpoint", func(t *testing.T) {
		corrupted := undo
		// op2 在 base 中存在、s1 中也存在（未被消费）—— 把 op2 加进 Created
		// 故意让 Entry 的 Value 与 set 中的实际 Entry 不同（避免「巧合相等」）。
		// set 中的 op2.Entry.Value = 100，我们写入 9999。
		cur, _ := s1.Get(op2)
		dup := UndoEntry{
			OutPoint: op2,
			Entry: Entry{
				Value:      9999, // 故意与 cur.Value (=100) 不一致
				PubKeyHash: cur.PubKeyHash,
				Height:     cur.Height,
				IsCoinbase: cur.IsCoinbase,
			},
		}
		// 防御性：若意外仍相等则强制失败（不应发生，因为 Value 字段已故意设不同）
		if entryEqual(cur, dup.Entry) {
			t.Fatalf("setup 错：Entry 仍相等 cur=%+v dup=%+v", cur, dup.Entry)
		}
		// 必须保持排序，否则会先触发排序错误。先找到 op2 应该插入的位置：
		newCreated := append([]UndoEntry{}, corrupted.Created...)
		newCreated = append(newCreated, dup)
		sort.Slice(newCreated, func(i, j int) bool {
			return outPointLess(newCreated[i].OutPoint, newCreated[j].OutPoint)
		})
		corrupted.Created = newCreated
		// Disconnect 应当失败：Created 中的 op2 Entry 与 set 不一致
		if _, err := DisconnectBlock(s1, corrupted); err == nil {
			t.Fatal("corrupted Created（重复 outpoint + Entry 不匹配）未触发错误")
		}
	})

	// ─── (3) Spent 恢复冲突：人为让 Spent 中出现 set 已存在的 outpoint
	t.Run("restore-conflicting-outpoint", func(t *testing.T) {
		corrupted := undo
		// op2 在 s1 中仍存在（未被本区块消费）；把它加进 Spent → 恢复时冲突
		conflict := UndoEntry{
			OutPoint: op2,
			Entry:    Entry{Value: 100, PubKeyHash: alice.PubKeyHash(), Height: 0, IsCoinbase: false},
		}
		corrupted.Spent = append(corrupted.Spent, conflict)
		if _, err := DisconnectBlock(s1, corrupted); err == nil {
			t.Fatal("corrupted Spent（冲突 outpoint）未触发错误")
		}
	})
}
