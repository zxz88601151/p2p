package main

// 本文件覆盖 PHASE MINING-REMEDIATION-1 §12 测试矩阵的**挖矿运行时**部分。
//
// 与 internal/blockchain/template_validation_test.go 的分工：
//   - 那一半证明「模板是否合法」这一**判定**是共用的、正确的；
//   - 这一半证明挖矿循环**据该判定做出的行为**是正确的：
//     合法模板才执行 PoW；非法模板 0 次 PoW 尝试；政策终态不烧 CPU 也不报故障；
//     结构性错误显式 FAILED；链尾变化只丢弃重试、不计入「被拒」。
//
// 贯穿全部用例的核心不变量（§9，本阶段的第一目标）：
//
//	**凡是被判为「不存在合法候选」的模板，PoW 尝试次数必须恰好为 0。**
//
// 断言方式统一为「前后差值」：nodeService 的 powAttempts 是累计计数器，
// 记录的是真实执行的双 SHA-256 尝试次数（pow.MineCancelable 的返回值），
// 因此 delta == 0 是「真的没算」而非「日志级别被调低了」的证据。

import (
	"errors"
	"fmt"
	"runtime"
	"sort"
	"testing"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

// ---- §5 分类决策（纯函数，覆盖 A–E 全部落点）----

// TestTemplateFailureClassificationCoversEveryCause 把「某一类失败如何处置」的决策
// 从挖矿循环里抽出来单独锁定。抽出来的理由是可测性：分类函数是纯函数，
// 因此不需构造长链也能覆盖「补贴归零」这类需要 1260 个区块才能自然到达的输入。
//
// 覆盖的五个落点（§5）：
//
//	A 结构性失败        → 终止挖矿（FAILED）
//	B 链尾已变化        → 丢弃并重建（不烧 PoW）
//	C 陈旧/竞争         → 同 B（由 IsTemplateStale 一并覆盖）
//	D 政策终态          → STALLED（不是 FAILED，也不是 RUNNING）
//	E 未预见失败        → 由挖矿循环以有界退避重试，超限升级 FAILED（不在本函数内）
func TestTemplateFailureClassificationCoversEveryCause(t *testing.T) {
	// 预校验失败链上的错误形态与生产一致：ApplyBlock 的错误被包进 ErrBadTxLayout
	wrapped := func(inner error) error { return fmt.Errorf("%w: %v", blockchain.ErrBadTxLayout, inner) }

	cases := []struct {
		name        string
		err         error
		totalReward uint64
		want        templateFailureClass
	}{
		{"A Merkle 不匹配且奖励非零", blockchain.ErrMerkleMismatch, 50, classTemplateStructural},
		{"A 交易层错误且奖励非零", wrapped(utxo.ErrExcessiveCoinbase), 51, classTemplateStructural},
		{"A 体积超限且奖励非零", blockchain.ErrBlockTooLarge, 50, classTemplateStructural},
		{"B 链尾已被替换", blockchain.ErrInvalidPrevHash, 50, classTemplateStale},
		{"B 链尾时间戳已领先", blockchain.ErrTimestampOutOfRange, 50, classTemplateStale},
		{"B 共识难度位已变化", blockchain.ErrUnexpectedBits, 50, classTemplateStale},
		{"C 陈旧优先于政策终态（链尾变化且奖励为 0）", blockchain.ErrInvalidPrevHash, 0, classTemplateStale},
		{"D 补贴耗尽：零值输出错误", wrapped(utxo.ErrZeroOutput), 0, classTemplatePolicyTerminal},
		{"D 补贴耗尽：超上限错误", wrapped(utxo.ErrExcessiveCoinbase), 0, classTemplatePolicyTerminal},
		{"D 奖励为 0 时未预见失败按政策终态处置", errors.New("某种未分类的模板错误"), 0, classTemplatePolicyTerminal},
		{"奖励为 1 时同一错误即回到结构性", wrapped(utxo.ErrZeroOutput), 1, classTemplateStructural},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyTemplateFailure(tc.err, tc.totalReward)
			if got != tc.want {
				t.Fatalf("classifyTemplateFailure(%v, totalReward=%d) = %d, want %d",
					tc.err, tc.totalReward, got, tc.want)
			}
		})
	}
}

// ---- §12 Test A / Test G：补贴与手续费均正常时真实执行 PoW ----

// TestMiningRuntimeMinesWhenSubsidyIsPositiveAndNoFees tests A（subsidy>0, fees=0）
// 与 Test G（合法模板确实执行 PoW）合并断言：模板合法、PoW 真实发生、区块内容符合
// 「coinbase = Subsidy(height) + 手续费合计」这一共识上限。
func TestMiningRuntimeMinesWhenSubsidyIsPositiveAndNoFees(t *testing.T) {
	rt, _, _ := startTestRuntime(t, false)
	svc := rt.svc

	if svc.pool.Len() != 0 {
		t.Fatalf("新节点交易池应为空，实际 %d 笔", svc.pool.Len())
	}
	powBefore := svc.powAttempts.Load()
	heightBefore := svc.chain.Height()

	got := mineOnce(svc, nil)
	if got != mineOutcomeMined {
		t.Fatalf("无手续费的合法模板应挖出区块，结果 = %s", got)
	}

	// Test G：PoW 必须真实执行（而不是「跳过求解直接成功」）
	if delta := svc.powAttempts.Load() - powBefore; delta == 0 {
		t.Fatal("合法模板必须真实执行 PoW，powAttempts 未增长")
	}
	if svc.acceptedBlocks.Load() != 1 {
		t.Fatalf("acceptedBlocks = %d, want 1", svc.acceptedBlocks.Load())
	}
	if svc.rejectedBlocks.Load() != 0 {
		t.Fatalf("rejectedBlocks = %d, want 0", svc.rejectedBlocks.Load())
	}
	st, reason := svc.miningStateSnapshot()
	if st != MiningRunning || reason != "mined" {
		t.Fatalf("挖矿状态 = (%s, %q), want (RUNNING, \"mined\")", st, reason)
	}

	height := heightBefore + 1
	b, err := svc.chain.BlockByHeight(height)
	if err != nil {
		t.Fatalf("读取高度 %d 区块失败: %v", height, err)
	}
	if len(b.Transactions) != 1 {
		t.Fatalf("交易数 = %d, want 1（仅 coinbase）", len(b.Transactions))
	}
	if !b.Transactions[0].IsCoinbase() {
		t.Fatal("区块首笔交易必须是 coinbase")
	}
	if gotTotal := sumOutputs(b.Transactions[0]); gotTotal != utxo.Subsidy(height) {
		t.Fatalf("coinbase 总额 = %d, want %d（补贴 + 0 手续费）", gotTotal, utxo.Subsidy(height))
	}
}

// ---- §12 Test B：补贴正常且手续费为正 ----

// TestMiningRuntimePacksFeePayingTransaction tests B（subsidy>0, fees>0）：
// 池中带手续费的真实交易必须被打包，且 coinbase 上限随之提升。
func TestMiningRuntimePacksFeePayingTransaction(t *testing.T) {
	rt, _, _ := startTestRuntime(t, false)
	svc := rt.svc

	// 挖到第一个 coinbase 成熟，使节点有钱支付手续费
	mineBlocks(t, rt, utxo.CoinbaseMaturity+1)

	recipient, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("生成收款钱包失败: %v", err)
	}
	const amount, fee = uint64(1), uint64(1)
	resp, err := svc.Send(recipient.Address(), amount, fee)
	if err != nil {
		t.Fatalf("提交手续费交易失败: %v", err)
	}
	if resp.Fee != fee {
		t.Fatalf("实际手续费 = %d, want %d", resp.Fee, fee)
	}
	if svc.pool.Len() != 1 {
		t.Fatalf("交易池大小 = %d, want 1", svc.pool.Len())
	}

	height := svc.chain.Height() + 1
	powBefore := svc.powAttempts.Load()
	got := mineOnce(svc, nil)
	if got != mineOutcomeMined {
		t.Fatalf("含手续费交易的模板应挖出区块，结果 = %s", got)
	}
	if delta := svc.powAttempts.Load() - powBefore; delta == 0 {
		t.Fatal("合法模板必须真实执行 PoW，powAttempts 未增长")
	}
	if svc.pool.Len() != 0 {
		t.Fatalf("打包后交易池应清空，实际 %d 笔", svc.pool.Len())
	}

	b, err := svc.chain.BlockByHeight(height)
	if err != nil {
		t.Fatalf("读取高度 %d 区块失败: %v", height, err)
	}
	if len(b.Transactions) != 2 {
		t.Fatalf("交易数 = %d, want 2（coinbase + 手续费交易）", len(b.Transactions))
	}
	want := utxo.Subsidy(height) + fee
	if gotTotal := sumOutputs(b.Transactions[0]); gotTotal != want {
		t.Fatalf("coinbase 总额 = %d, want %d（补贴 %d + 手续费 %d）",
			gotTotal, want, utxo.Subsidy(height), fee)
	}
}

// ---- §12 Test E：结构性错误必须 0 次 PoW 且显式 FAILED ----

// TestMiningRuntimeFailsWithoutPoWWhenPooledTransactionIsNoLongerValid tests E。
//
// 场景（真实且不人为造错）：交易池里有一笔**入池时合法**的交易，其输入在随后被
// 另一个区块以竞争方式花费掉（本地交易池未及时重验），于是下一轮模板必然不合法。
// 此时模板的结构性错误与「补贴是否归零」无关，必须走 A 类：不执行 PoW、显式 FAILED。
//
// 关键断言：
//   - PoW 尝试次数增量恰为 0（**不得**先烧完整套 PoW 再被拒绝）；
//   - 挖矿状态 = FAILED（不是 STALLED，也不是继续 RUNNING）；
//   - 该失败不计入 rejectedBlocks（那个计数器专指「预校验通过后仍被拒」，是真正的不变量破坏）。
func TestMiningRuntimeFailsWithoutPoWWhenPooledTransactionIsNoLongerValid(t *testing.T) {
	rt, _, _ := startTestRuntime(t, false)
	svc := rt.svc

	mineBlocks(t, rt, utxo.CoinbaseMaturity+1)
	nextHeight := svc.chain.Height() + 1

	snap := svc.chain.UTXOSnapshot()
	op, value := pickMatureCoinbaseOutpoint(t, snap, svc.miner, nextHeight)
	if value < 3 {
		t.Fatalf("选中的 coinbase 输出金额 %d 过小，无法构造含手续费交易", value)
	}

	// ① 一笔合法交易入池（手续费 1）
	pooled := signSpend(t, svc.miner, op, [20]byte{0x41}, value-1)
	if err := svc.pool.Add(snap, pooled, nextHeight); err != nil {
		t.Fatalf("合法交易被交易池拒绝: %v", err)
	}
	if svc.pool.Len() != 1 {
		t.Fatalf("交易池大小 = %d, want 1", svc.pool.Len())
	}

	// ② 竞争区块直接用同一个输入上链（刻意绕过交易池清理，模拟池未及时同步的竞态）
	competing := signSpend(t, svc.miner, op, [20]byte{0x42}, value-2)
	tip, err := svc.chain.Tip()
	if err != nil {
		t.Fatalf("读取链尾失败: %v", err)
	}
	cb := transaction.NewCoinbaseTx(svc.miner.PubKeyHash(), utxo.Subsidy(nextHeight), nextHeight)
	comp := block.NewCandidateBlock(tip.Header.Hash(), svc.chain.CurrentBits(),
		[]*transaction.Transaction{cb, competing})
	if found, _ := pow.Mine(comp); !found {
		t.Fatal("竞争区块 PoW 求解失败")
	}
	if err := svc.chain.AddBlock(comp); err != nil {
		t.Fatalf("竞争区块应合法: %v", err)
	}
	if svc.pool.Len() != 1 {
		t.Fatalf("池中失效交易应仍被保留（Pending 不做重验），实际 %d 笔", svc.pool.Len())
	}

	// ③ 现在的模板必然含双花交易 ⇒ 预校验必须在不执行 PoW 的前提下拒绝它
	powBefore := svc.powAttempts.Load()
	heightBefore := svc.chain.Height()
	acceptedBefore := svc.acceptedBlocks.Load()

	got := mineOnce(svc, nil)
	if got != mineOutcomeStructFail {
		t.Fatalf("结果 = %s, want STRUCT_FAIL", got)
	}
	if delta := svc.powAttempts.Load() - powBefore; delta != 0 {
		t.Fatalf("结构性错误必须 0 次 PoW 尝试，实际 %d 次", delta)
	}
	st, reason := svc.miningStateSnapshot()
	if st != MiningFailed {
		t.Fatalf("挖矿状态 = %s（原因 %q），want FAILED", st, reason)
	}
	if svc.chain.Height() != heightBefore {
		t.Fatalf("失败模板不得上链：链高 %d → %d", heightBefore, svc.chain.Height())
	}
	if svc.acceptedBlocks.Load() != acceptedBefore {
		t.Fatalf("失败模板不得计入 acceptedBlocks：%d → %d", acceptedBefore, svc.acceptedBlocks.Load())
	}
	if svc.rejectedBlocks.Load() != 0 {
		t.Fatalf("预校验阶段被拒不得计入 rejectedBlocks（该计数专指预校验通过后仍被拒），实际 %d",
			svc.rejectedBlocks.Load())
	}
	if svc.mineRetries.Load() != 0 {
		t.Fatalf("结构性错误不可重试，不应累加重试计数，实际 %d", svc.mineRetries.Load())
	}
}

// ---- §12 Test F：链尾变化只丢弃重试，不烧 PoW、不计入「被拒」 ----

// TestMiningRuntimeDiscardsCandidateWhenTipChanges tests F。
//
// 手法说明：「求解期间链尾变化」本质上是异步事件，为使其可复现，本用例在 mineOnce
// 运行期间**持续重投**链尾变化信号（该信号通道容量为 1、投递为非阻塞，因此重投
// 不会阻塞也不会重复唤醒）。mineOnce 入口会丢弃一次遗留信号，但重投会立即补上，
// 使「进入 select 时信号已在通道中」几乎必然成立。
//
// 用例仍不依赖这个时序巧合：它允许最多 10 次尝试，只要其中**任意一次**落在
// 放弃路径上即通过；而任何一种非预期结果（结构性错误、政策终态）都会立即失败。
func TestMiningRuntimeDiscardsCandidateWhenTipChanges(t *testing.T) {
	rt, _, _ := startTestRuntime(t, false)
	svc := rt.svc

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			svc.notifyTipChanged()
			runtime.Gosched()
		}
	}()

	sawStale := false
	for attempt := 0; attempt < 10 && !sawStale; attempt++ {
		heightBefore := svc.chain.Height()
		acceptedBefore := svc.acceptedBlocks.Load()
		rejectedBefore := svc.rejectedBlocks.Load()

		switch got := mineOnce(svc, nil); got {
		case mineOutcomeStale:
			sawStale = true
			if svc.chain.Height() != heightBefore {
				t.Fatalf("放弃候选区块不应改变链高：%d → %d", heightBefore, svc.chain.Height())
			}
			if svc.acceptedBlocks.Load() != acceptedBefore {
				t.Fatalf("放弃不应计入 acceptedBlocks")
			}
			if svc.rejectedBlocks.Load() != rejectedBefore {
				t.Fatalf("链尾变化属于「放弃」而非「被拒」，不应计入 rejectedBlocks")
			}
			if svc.mineRetries.Load() == 0 {
				t.Fatal("放弃候选区块应累加重试计数")
			}
			if st, reason := svc.miningStateSnapshot(); st == MiningFailed || st == MiningStalled {
				t.Fatalf("链尾变化是合法情形，不得进入终态：状态 = %s（原因 %q）", st, reason)
			}
		case mineOutcomeMined:
			// PoW 抢先命中（概率极低）：链已合法推进，重试即可
		default:
			t.Fatalf("链尾变化场景不应产生该结果：%s", got)
		}
	}
	if !sawStale {
		t.Fatal("10 次尝试均未触发「求解期间链尾变化」的放弃路径，用例未能覆盖该分支")
	}
}

// ---- §12 Test C + Test D：补贴归零边界（真实链上端到端）----

// TestMiningRuntimeAtSubsidyExhaustion 在**真实的高度 1260** 上验证 A1 裁决下的两条行为：
//
//	Test D：Subsidy == 0 且 fees == 0  ⇒  不存在合法候选  ⇒  STALLED，0 次 PoW
//	Test C：Subsidy == 0 且 fees  > 0  ⇒  零补贴区块仍合法  ⇒  MINED，PoW 真实执行
//
// 这两个子用例共用同一条前推到边界的长链（Test D 不改变链状态，因此 Test C 可直接续用）。
//
// 关于长链的构建方式：链状态在**无存储的** Blockchain 上构建后再交给 nodeService。
// 这样做的原因是 FileBlockStore.SaveBlock 每块都做一次 fsync —— 1259 次 fsync 会让
// 本用例的时间由「PoW」变成「磁盘」，而本用例要证明的性质与持久化无关
// （持久化本身由既有的 TestChainPersistsAcrossRestart / TestFullStackRestartPersists 覆盖）。
func TestMiningRuntimeAtSubsidyExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("需要构建高度 1259 的真实链（约 8×10^7 次双 SHA-256），-short 下跳过")
	}

	const boundary = 1260 // Subsidy 恰在此高度归零

	rt, _, _ := startTestRuntime(t, false)
	svc := rt.svc

	genesis, err := rt.chain.BlockByHeight(0)
	if err != nil {
		t.Fatalf("读取创世区块失败: %v", err)
	}
	chain, err := blockchain.NewBlockchainWithGenesis(genesis)
	if err != nil {
		t.Fatalf("基于同一创世构建无存储链失败: %v", err)
	}
	svc.chain = chain

	workers := runtime.NumCPU()
	start := time.Now()
	for chain.Height() < boundary-1 {
		h := chain.Height() + 1
		tip, err := chain.Tip()
		if err != nil {
			t.Fatalf("读取链尾失败: %v", err)
		}
		cb := transaction.NewCoinbaseTx(svc.miner.PubKeyHash(), utxo.Subsidy(h), h)
		cand := block.NewCandidateBlock(tip.Header.Hash(), chain.CurrentBits(),
			[]*transaction.Transaction{cb})
		if found, _ := pow.MineParallel(cand, workers); !found {
			t.Fatalf("高度 %d 求解失败", h)
		}
		if err := chain.AddBlock(cand); err != nil {
			t.Fatalf("高度 %d 上链失败: %v", h, err)
		}
		if h%210 == 0 {
			t.Logf("前推进度：高度 %d，该高度补贴 %d", h, utxo.Subsidy(h))
		}
	}
	t.Logf("链已前推到高度 %d（用时 %s），下一可挖高度 %d 的 Subsidy = %d",
		chain.Height(), time.Since(start).Round(time.Millisecond), boundary, utxo.Subsidy(boundary))

	if got := utxo.Subsidy(boundary); got != 0 {
		t.Fatalf("前提不成立：Subsidy(%d) = %d, want 0", boundary, got)
	}

	t.Run("TestD_补贴耗尽且无手续费_进入STALLED且0次PoW", func(t *testing.T) {
		if svc.pool.Len() != 0 {
			t.Fatalf("本子用例要求交易池为空，实际 %d 笔", svc.pool.Len())
		}
		heightBefore := chain.Height()
		powBefore := svc.powAttempts.Load()
		acceptedBefore := svc.acceptedBlocks.Load()

		got := mineOnce(svc, nil)

		if got != mineOutcomeStalled {
			t.Fatalf("结果 = %s, want STALLED（补贴归零且无手续费时不存在合法候选）", got)
		}
		if delta := svc.powAttempts.Load() - powBefore; delta != 0 {
			t.Fatalf("政策终态不得执行任何 PoW，实际尝试 %d 次（无意义 PoW 未被消除）", delta)
		}
		st, reason := svc.miningStateSnapshot()
		if st != MiningStalled {
			t.Fatalf("挖矿状态 = %s, want STALLED", st)
		}
		if reason != "subsidy-exhausted-no-fee-tx" {
			t.Fatalf("状态原因 = %q, want \"subsidy-exhausted-no-fee-tx\"", reason)
		}
		if chain.Height() != heightBefore {
			t.Fatalf("STALLED 不得改变链高：%d → %d", heightBefore, chain.Height())
		}
		if svc.acceptedBlocks.Load() != acceptedBefore {
			t.Fatalf("STALLED 不得计入 acceptedBlocks")
		}
		if svc.rejectedBlocks.Load() != 0 {
			t.Fatalf("政策终态不是「被拒」，rejectedBlocks = %d, want 0", svc.rejectedBlocks.Load())
		}
	})

	t.Run("TestC_补贴耗尽但手续费为正_fee-only区块可挖出", func(t *testing.T) {
		recipient, err := wallet.NewWallet()
		if err != nil {
			t.Fatalf("生成收款钱包失败: %v", err)
		}
		const amount, fee = uint64(1), uint64(1)
		resp, err := svc.Send(recipient.Address(), amount, fee)
		if err != nil {
			t.Fatalf("提交手续费交易失败（补贴耗尽后节点仍应能付出已成熟的余额）: %v", err)
		}
		if resp.Fee != fee {
			t.Fatalf("实际手续费 = %d, want %d", resp.Fee, fee)
		}
		if svc.pool.Len() != 1 {
			t.Fatalf("交易池大小 = %d, want 1", svc.pool.Len())
		}

		heightBefore := chain.Height()
		powBefore := svc.powAttempts.Load()

		got := mineOnce(svc, nil)

		if got != mineOutcomeMined {
			t.Fatalf("结果 = %s, want MINED（补贴为 0 但有手续费，模板合法）", got)
		}
		if delta := svc.powAttempts.Load() - powBefore; delta == 0 {
			t.Fatal("合法模板必须真实执行 PoW，powAttempts 未增长")
		}
		if svc.pool.Len() != 0 {
			t.Fatalf("打包后交易池应清空，实际 %d 笔", svc.pool.Len())
		}

		height := heightBefore + 1
		if chain.Height() != height {
			t.Fatalf("链高 = %d, want %d", chain.Height(), height)
		}
		b, err := chain.BlockByHeight(height)
		if err != nil {
			t.Fatalf("读取高度 %d 区块失败: %v", height, err)
		}
		if len(b.Transactions) != 2 {
			t.Fatalf("交易数 = %d, want 2（coinbase + 手续费交易）", len(b.Transactions))
		}
		if !b.Transactions[0].IsCoinbase() {
			t.Fatal("区块首笔交易必须是 coinbase")
		}
		// 零补贴区块：coinbase 完全由手续费资助
		if gotTotal := sumOutputs(b.Transactions[0]); gotTotal != fee {
			t.Fatalf("零补贴区块的 coinbase 总额 = %d, want %d（补贴 0 + 手续费 %d）",
				gotTotal, fee, fee)
		}
	})
}

// ---- 工具 ----

// sumOutputs 返回一笔交易全部输出的金额合计。
func sumOutputs(tx *transaction.Transaction) uint64 {
	var total uint64
	for _, o := range tx.Outputs {
		total += o.Value
	}
	return total
}

// pickMatureCoinbaseOutpoint 确定性地挑选一个「已成熟且属于 miner」的 coinbase 输出。
//
// 必须显式排序：UTXO 集合以 map 存储，Go 的 map 迭代顺序是随机的，
// 不排序会让「选到哪一个输出」不可复现，进而让依赖它构造的交易与断言全部不稳定。
func pickMatureCoinbaseOutpoint(t *testing.T, snap *utxo.UTXOSet, miner *wallet.Wallet, height int) (utxo.OutPoint, uint64) {
	t.Helper()
	type cand struct {
		op    utxo.OutPoint
		value uint64
		h     int
	}
	var list []cand
	for op, e := range snap.AllEntries() {
		if e.PubKeyHash != miner.PubKeyHash() || !e.IsCoinbase {
			continue
		}
		if height-e.Height < utxo.CoinbaseMaturity {
			continue
		}
		list = append(list, cand{op: op, value: e.Value, h: e.Height})
	}
	if len(list) == 0 {
		t.Fatal("没有已成熟且属于该矿工的 coinbase 输出")
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].h != list[j].h {
			return list[i].h < list[j].h
		}
		if list[i].op.Hash != list[j].op.Hash {
			return string(list[i].op.Hash[:]) < string(list[j].op.Hash[:])
		}
		return list[i].op.Index < list[j].op.Index
	})
	return list[0].op, list[0].value
}

// signSpend 用 miner 对「消费指定输出、支付到 to、金额为 value」的交易签名。
// 与生产路径使用同一套签名语义（签名消息 = tx.Hash()，并附公钥供验签）。
func signSpend(t *testing.T, miner *wallet.Wallet, op utxo.OutPoint, to [20]byte, value uint64) *transaction.Transaction {
	t.Helper()
	tx := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
		Outputs: []transaction.TxOutput{{Value: value, PubKeyHash: to}},
	}
	sig, err := miner.Sign(tx.Hash())
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	tx.Inputs[0].Signature = sig
	tx.Inputs[0].PubKey = miner.PublicKey
	return tx
}
