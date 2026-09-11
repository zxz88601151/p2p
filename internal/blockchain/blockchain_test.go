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

const testReward = 50

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
	if found, _ := pow.Mine(gb, 0); !found {
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
	if found, _ := pow.Mine(candidate, 0); !found {
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
	// 创世 coinbase(50, 高度 0，已成熟) + 3 个新 coinbase（未成熟不影响 includeImmative 统计）
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
	wrongBits := block.NewCandidateBlock(tip.Header.Hash(), 16, []*transaction.Transaction{
		transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(1), 1),
	})
	if found, _ := pow.Mine(wrongBits, 0); !found {
		t.Fatal("低难度挖矿失败")
	}
	if err := bc.ValidateBlock(wrongBits); !errors.Is(err, blockchain.ErrUnexpectedBits) {
		t.Fatalf("难度位校验: got %v, want ErrUnexpectedBits", err)
	}

	// ErrTimestampOutOfRange：时间戳早于父块
	oldTs := newCandidate()
	oldTs.Header.Timestamp = tip.Header.Timestamp - 1
	if found, _ := pow.Mine(oldTs, 0); !found {
		t.Fatal("旧时间戳挖矿失败")
	}
	if err := bc.ValidateBlock(oldTs); !errors.Is(err, blockchain.ErrTimestampOutOfRange) {
		t.Fatalf("时间戳校验: got %v, want ErrTimestampOutOfRange", err)
	}

	// ErrMerkleMismatch：挖矿后篡改交易列表
	tampered := newCandidate()
	if found, _ := pow.Mine(tampered, 0); !found {
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
	if found, _ := pow.Mine(candidate, 0); !found {
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
	if found, _ := pow.Mine(candidate, 0); !found {
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
	pow.Mine(bad, 0)
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
