package utxo

import (
	"testing"

	"p2pchain/internal/transaction"
	"p2pchain/internal/wallet"
)

// ---- 测试辅助 ----

func newTestWallet(t *testing.T) *wallet.Wallet {
	t.Helper()
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("生成钱包失败: %v", err)
	}
	return w
}

// fund 直接向集合注入一个属于 owner 的普通 UTXO（模拟既有链上输出）。
func fund(t *testing.T, set *UTXOSet, txid [32]byte, index uint32, owner *wallet.Wallet, value uint64, height int) OutPoint {
	t.Helper()
	op := OutPoint{Hash: txid, Index: index}
	set.Add(op, Entry{Value: value, PubKeyHash: owner.PubKeyHash(), Height: height})
	return op
}

// buildSpendTx 构造一笔消费 spender 全部可用输出中 amount+fee 的交易并签名。
func buildSpendTx(t *testing.T, set *UTXOSet, spender *wallet.Wallet, to [20]byte, amount, fee uint64) *transaction.Transaction {
	t.Helper()
	target := amount + fee
	var inTotal uint64
	tx := &transaction.Transaction{}
	for op, e := range set.AllEntries() {
		if e.PubKeyHash != spender.PubKeyHash() {
			continue
		}
		if e.IsCoinbase { // 测试里只消费普通 UTXO，maturity 用独立用例覆盖
			continue
		}
		tx.Inputs = append(tx.Inputs, transaction.TxInput{PrevTxHash: op.Hash, OutIndex: op.Index})
		inTotal += e.Value
		if inTotal >= target {
			break
		}
	}
	if inTotal < target {
		t.Fatalf("测试搭建失败：可用余额 %d 不足 %d", inTotal, target)
	}
	tx.Outputs = append(tx.Outputs, transaction.TxOutput{Value: amount, PubKeyHash: to})
	if change := inTotal - target; change > 0 {
		tx.Outputs = append(tx.Outputs, transaction.TxOutput{Value: change, PubKeyHash: spender.PubKeyHash()})
	}
	signTx(t, tx, spender)
	return tx
}

// signTx 用 spender 的私钥为交易全部输入签名（sighash = tx.Hash()）。
func signTx(t *testing.T, tx *transaction.Transaction, signer *wallet.Wallet) {
	t.Helper()
	h := tx.Hash()
	for i := range tx.Inputs {
		sig, err := signer.Sign(h)
		if err != nil {
			t.Fatalf("签名失败: %v", err)
		}
		tx.Inputs[i].Signature = sig
		tx.Inputs[i].PubKey = signer.PublicKey
	}
}

// ---- 正向路径 ----

func TestValidTransactionAccepted(t *testing.T) {
	set := NewUTXOSet()
	w := newTestWallet(t)
	to := newTestWallet(t)
	fund(t, set, [32]byte{1}, 0, w, 100, 0)

	tx := buildSpendTx(t, set, w, to.PubKeyHash(), 60, 10)
	if _, err := ValidateTransaction(tx, set, 1); err != nil {
		t.Fatalf("合法交易被拒绝: %v", err)
	}
	// 输入被消费
	if set.Has(OutPoint{Hash: [32]byte{1}, Index: 0}) {
		t.Fatal("输入 UTXO 未被消费")
	}
	// 输出与找零已入集合，余额正确
	if got := set.Balance(to.PubKeyHash(), 1, true); got != 60 {
		t.Fatalf("接收方余额 = %d, want 60", got)
	}
	if got := set.Balance(w.PubKeyHash(), 1, true); got != 30 {
		t.Fatalf("找零余额 = %d, want 30", got)
	}
}

// ---- 攻击矩阵：负向路径 ----

func TestUnknownUTXORejected(t *testing.T) {
	set := NewUTXOSet()
	w := newTestWallet(t)
	to := w.PubKeyHash()
	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: [32]byte{9}, OutIndex: 0}},
		Outputs: []transaction.TxOutput{{Value: 1, PubKeyHash: to}},
	}
	signTx(t, tx, w)
	if _, err := ValidateTransaction(tx, set, 1); err == nil {
		t.Fatal("不存在的 UTXO 被接受")
	}
}

func TestAlreadySpentRejected(t *testing.T) {
	set := NewUTXOSet()
	w := newTestWallet(t)
	op := fund(t, set, [32]byte{2}, 0, w, 100, 0)

	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
		Outputs: []transaction.TxOutput{{Value: 100, PubKeyHash: w.PubKeyHash()}},
	}
	signTx(t, tx, w)
	if _, err := ValidateTransaction(tx, set, 1); err != nil {
		t.Fatalf("首次花费被拒绝: %v", err)
	}
	// 第二次花费同一输出 → 双花拒绝
	if _, err := ValidateTransaction(tx, set, 1); err == nil {
		t.Fatal("已花费 UTXO 的二次消费被接受（双花）")
	}
}

func TestDuplicateInputRejected(t *testing.T) {
	set := NewUTXOSet()
	w := newTestWallet(t)
	op := fund(t, set, [32]byte{3}, 0, w, 100, 0)

	in := transaction.TxInput{PrevTxHash: op.Hash, OutIndex: op.Index}
	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{in, in},
		Outputs: []transaction.TxOutput{{Value: 100, PubKeyHash: w.PubKeyHash()}},
	}
	signTx(t, tx, w)
	if _, err := ValidateTransaction(tx, set, 1); err == nil {
		t.Fatal("重复输入被接受")
	}
}

func TestInvalidSignatureRejected(t *testing.T) {
	set := NewUTXOSet()
	w := newTestWallet(t)
	attacker := newTestWallet(t)
	op := fund(t, set, [32]byte{4}, 0, w, 100, 0)

	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
		Outputs: []transaction.TxOutput{{Value: 100, PubKeyHash: attacker.PubKeyHash()}},
	}
	signTx(t, tx, attacker) // 用攻击者私钥签名他人 UTXO
	if _, err := ValidateTransaction(tx, set, 1); err == nil {
		t.Fatal("伪造签名被接受")
	}
}

func TestModifiedOutputAfterSigningRejected(t *testing.T) {
	set := NewUTXOSet()
	w := newTestWallet(t)
	to := newTestWallet(t)
	op := fund(t, set, [32]byte{5}, 0, w, 100, 0)

	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
		Outputs: []transaction.TxOutput{{Value: 100, PubKeyHash: to.PubKeyHash()}},
	}
	signTx(t, tx, w)
	// 签名后篡改输出金额
	tx.Outputs[0].Value = 200
	if _, err := ValidateTransaction(tx, set, 1); err == nil {
		t.Fatal("签名后篡改输出的交易被接受")
	}
}

func TestModifiedInputAfterSigningRejected(t *testing.T) {
	set := NewUTXOSet()
	w := newTestWallet(t)
	op := fund(t, set, [32]byte{6}, 0, w, 100, 0)

	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
		Outputs: []transaction.TxOutput{{Value: 100, PubKeyHash: w.PubKeyHash()}},
	}
	signTx(t, tx, w)
	// 签名后篡改输入引用
	tx.Inputs[0].OutIndex = 7
	if _, err := ValidateTransaction(tx, set, 1); err == nil {
		t.Fatal("签名后篡改输入引用的交易被接受")
	}
}

func TestOverspendRejected(t *testing.T) {
	set := NewUTXOSet()
	w := newTestWallet(t)
	to := w.PubKeyHash()
	op := fund(t, set, [32]byte{7}, 0, w, 100, 0)

	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
		Outputs: []transaction.TxOutput{{Value: 101, PubKeyHash: to}},
	}
	signTx(t, tx, w)
	if _, err := ValidateTransaction(tx, set, 1); err == nil {
		t.Fatal("超额支出被接受")
	}
}

func TestZeroValueOutputRejected(t *testing.T) {
	set := NewUTXOSet()
	w := newTestWallet(t)
	op := fund(t, set, [32]byte{8}, 0, w, 100, 0)

	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
		Outputs: []transaction.TxOutput{{Value: 0, PubKeyHash: w.PubKeyHash()}},
	}
	signTx(t, tx, w)
	if _, err := ValidateTransaction(tx, set, 1); err == nil {
		t.Fatal("零金额输出被接受")
	}
}

func TestMalformedPubKeyRejected(t *testing.T) {
	set := NewUTXOSet()
	w := newTestWallet(t)
	op := fund(t, set, [32]byte{9}, 0, w, 100, 0)

	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
		Outputs: []transaction.TxOutput{{Value: 100, PubKeyHash: w.PubKeyHash()}},
	}
	h := tx.Hash()
	sig, _ := w.Sign(h)
	tx.Inputs[0].Signature = sig
	tx.Inputs[0].PubKey = []byte{0x01, 0x02, 0x03} // 非法公钥
	if _, err := ValidateTransaction(tx, set, 1); err == nil {
		t.Fatal("非法公钥被接受")
	}
}

func TestPubKeyMismatchRejected(t *testing.T) {
	set := NewUTXOSet()
	w := newTestWallet(t)
	attacker := newTestWallet(t)
	op := fund(t, set, [32]byte{10}, 0, w, 100, 0)

	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
		Outputs: []transaction.TxOutput{{Value: 100, PubKeyHash: attacker.PubKeyHash()}},
	}
	// 攻击者用自己的私钥签名，但提供 w 的公钥 → 哈希匹配但签名失败
	h := tx.Hash()
	sig, _ := attacker.Sign(h)
	tx.Inputs[0].Signature = sig
	tx.Inputs[0].PubKey = w.PublicKey
	if _, err := ValidateTransaction(tx, set, 1); err == nil {
		t.Fatal("公钥/签名不匹配的交易被接受")
	}

	// 另一形态：公钥哈希不匹配（attacker 公钥 + attacker 签名）
	tx2 := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
		Outputs: []transaction.TxOutput{{Value: 100, PubKeyHash: attacker.PubKeyHash()}},
	}
	signTx(t, tx2, attacker)
	if _, err := ValidateTransaction(tx2, set, 1); err == nil {
		t.Fatal("他人 UTXO + 自签交易被接受")
	}
}

// ---- Coinbase ----

func TestCoinbaseStructureRules(t *testing.T) {
	set := NewUTXOSet()

	// 合法：高度字段正确，无公钥
	cb := transaction.NewCoinbaseTx([20]byte{1}, 50, 7)
	if _, err := ValidateTransaction(cb, set, 7); err != nil {
		t.Fatalf("合法 coinbase 被拒绝: %v", err)
	}

	// 非法：高度与所在区块不一致
	if _, err := ValidateTransaction(cb, set, 8); err == nil {
		t.Fatal("高度不匹配的 coinbase 被接受")
	}

	// 非法：高度字段缺失（空签名）
	noHeight := transaction.NewCoinbaseTx([20]byte{1}, 50, 7)
	noHeight.Inputs[0].Signature = nil
	if _, err := ValidateTransaction(noHeight, set, 7); err == nil {
		t.Fatal("缺少高度字段的 coinbase 被接受")
	}

	// 非法：携带公钥
	bad := transaction.NewCoinbaseTx([20]byte{1}, 50, 7)
	bad.Inputs[0].PubKey = []byte{1, 2, 3}
	if _, err := ValidateTransaction(bad, set, 7); err == nil {
		t.Fatal("带公钥的 coinbase 被接受")
	}

	// 非法：无输出
	empty := &transaction.Transaction{Inputs: cb.Inputs}
	if _, err := ValidateTransaction(empty, set, 7); err == nil {
		t.Fatal("无输出 coinbase 被接受")
	}
}

func TestCoinbaseMaturity(t *testing.T) {
	set := NewUTXOSet()
	w := newTestWallet(t)
	cb := transaction.NewCoinbaseTx(w.PubKeyHash(), 50, 100)
	if _, err := ValidateTransaction(cb, set, 100); err != nil {
		t.Fatalf("coinbase 应用失败: %v", err)
	}

	op := OutPoint{Hash: cb.Hash(), Index: 0}
	spend := func(height int) *transaction.Transaction {
		tx := &transaction.Transaction{
			Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
			Outputs: []transaction.TxOutput{{Value: 50, PubKeyHash: w.PubKeyHash()}},
		}
		signTx(t, tx, w)
		return tx
	}

	// 成熟期内拒绝（height 100..109 不可花）
	if _, err := ValidateTransaction(spend(109), set, 109); err == nil {
		t.Fatal("未成熟 coinbase 被花费")
	}
	// 成熟期后接受（height 110 = 100 + CoinbaseMaturity）
	if _, err := ValidateTransaction(spend(100+CoinbaseMaturity), set, 100+CoinbaseMaturity); err != nil {
		t.Fatalf("成熟 coinbase 花费被拒绝: %v", err)
	}
}

// ---- 区块级状态迁移 ----

func TestApplyBlockCoinbaseRules(t *testing.T) {
	base := NewUTXOSet()
	w := newTestWallet(t)

	// 正常：单 coinbase，金额 = subsidy
	ok := []*transaction.Transaction{transaction.NewCoinbaseTx(w.PubKeyHash(), Subsidy(0), 0)}
	s1, _, err := ApplyBlock(base, ok, 0)
	if err != nil {
		t.Fatalf("合法区块被拒绝: %v", err)
	}
	if s1.Len() != 1 {
		t.Fatalf("coinbase 输出未入集合: len=%d", s1.Len())
	}

	// 拒绝：coinbase 超额
	excess := []*transaction.Transaction{transaction.NewCoinbaseTx(w.PubKeyHash(), Subsidy(0)+1, 0)}
	if _, _, err := ApplyBlock(base, excess, 0); err == nil {
		t.Fatal("超额 coinbase 被接受")
	}

	// 拒绝：无交易
	if _, _, err := ApplyBlock(base, nil, 0); err == nil {
		t.Fatal("空交易区块被接受")
	}

	// 拒绝：首笔不是 coinbase（用普通 UTXO 构造一笔正常交易放在首位）
	opX := fund(t, base, [32]byte{0xB1}, 0, w, 100, 0)
	normal := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: opX.Hash, OutIndex: opX.Index}},
		Outputs: []transaction.TxOutput{{Value: 100, PubKeyHash: w.PubKeyHash()}},
	}
	signTx(t, normal, w)
	wrongOrder := []*transaction.Transaction{normal, transaction.NewCoinbaseTx(w.PubKeyHash(), Subsidy(0), 0)}
	if _, _, err := ApplyBlock(base, wrongOrder, 0); err == nil {
		t.Fatal("coinbase 不在首位的区块被接受")
	}

	// 拒绝：双 coinbase
	two := []*transaction.Transaction{
		transaction.NewCoinbaseTx(w.PubKeyHash(), Subsidy(0), 0),
		transaction.NewCoinbaseTx(w.PubKeyHash(), Subsidy(0), 0),
	}
	if _, _, err := ApplyBlock(base, two, 0); err == nil {
		t.Fatal("双 coinbase 区块被接受")
	}
}

func TestApplyBlockFeesAndAtomicity(t *testing.T) {
	base := NewUTXOSet()
	w := newTestWallet(t)
	to := newTestWallet(t)

	// 准备两个 UTXO（各 100），模拟既有链状态
	op1 := fund(t, base, [32]byte{0xA1}, 0, w, 100, 0)
	op2 := fund(t, base, [32]byte{0xA2}, 0, w, 100, 0)
	baseSnapshot := base.Len()

	// 区块：消费 op1 付 80 给 to，fee=20；coinbase 恰好 = subsidy(1) + 20
	fee := uint64(20)
	normal := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op1.Hash, OutIndex: op1.Index}},
		Outputs: []transaction.TxOutput{{Value: 80, PubKeyHash: to.PubKeyHash()}},
	}
	signTx(t, normal, w)
	cb := transaction.NewCoinbaseTx(w.PubKeyHash(), Subsidy(1)+fee, 1)
	txs := []*transaction.Transaction{cb, normal}

	newSet, fees, err := ApplyBlock(base, txs, 1)
	if err != nil {
		t.Fatalf("合法区块被拒绝: %v", err)
	}
	if fees != fee {
		t.Fatalf("手续费 = %d, want %d", fees, fee)
	}
	// op1 消失，op2 保留；普通输出 + coinbase 输出
	if newSet.Has(op1) || !newSet.Has(op2) {
		t.Fatal("UTXO 消费/保留状态不正确")
	}
	if got := newSet.Balance(to.PubKeyHash(), 1, true); got != 80 {
		t.Fatalf("接收方余额 = %d, want 80", got)
	}

	// 原子性：包含非法交易的区块失败时，base 不变
	bad := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op2.Hash, OutIndex: op2.Index}},
		Outputs: []transaction.TxOutput{{Value: 999, PubKeyHash: to.PubKeyHash()}}, // 超额
	}
	signTx(t, bad, w)
	badTxs := []*transaction.Transaction{transaction.NewCoinbaseTx(w.PubKeyHash(), Subsidy(1), 1), bad}
	if _, _, err := ApplyBlock(base, badTxs, 1); err == nil {
		t.Fatal("含非法交易的区块被接受")
	}
	if base.Len() != baseSnapshot {
		t.Fatalf("原子性破坏：失败区块改变了基础集合 %d → %d", baseSnapshot, base.Len())
	}
}

func TestSubsidySchedule(t *testing.T) {
	// 50 = 0b110010，第 6 次减半（高度 1260）后归零
	cases := []struct {
		height int
		want   uint64
	}{
		{0, 50},
		{1, 50},
		{209, 50},
		{210, 25},
		{419, 25},
		{420, 12},
		{1050, 1},  // 50>>5
		{1260, 0},  // 50>>6
		{210 * 63, 0},
		{1 << 20, 0},
	}
	for _, c := range cases {
		if got := Subsidy(c.height); got != c.want {
			t.Errorf("Subsidy(%d) = %d, want %d", c.height, got, c.want)
		}
	}
}
