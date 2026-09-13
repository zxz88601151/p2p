package blockchain_test

// 本文件覆盖 PHASE MINING-REMEDIATION-1 §12 测试矩阵的**验证层**部分，并锁定
// §4 的 CRITICAL INVARIANT：
//
//	ValidateTemplate() 与 ValidateBlock() 必须共用同一套 consensus rules，
//	唯一差异是 ValidateTemplate 跳过 PoW（且只跳过 PoW）。
//
// 为什么这一层必须被单独锁死：本阶段新增的「挖矿前预校验」只有在**共用同一份规则**
// 的前提下才是安全的。若预校验与最终校验是两套实现，二者必然漂移，其结果有两种，
// 且都会造成真实损失：
//
//	① 预校验更宽 → 模板通过预校验、烧完整套 PoW，再被 AddBlock 拒绝（白烧，即本阶段要消灭的行为）；
//	② 预校验更严 → 合法模板被预校验拒绝，挖矿永久停止（活性丧失）。
//
// 因此下列测试不是在「验证功能存在」，而是在**证明两条路径不可分辨**（除 PoW 外）。
//
// 另一个被有意覆盖的点：`skipPoW` 只允许出现在本地挖矿路径上。网络入块路径
// （ValidateBlock / AddBlock / 启动回放 applyBlock）必须仍然完整执行 PoW ——
// 否则等于放弃防伪造区块的 DoS 保护。

import (
	"errors"
	"strings"
	"testing"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

// ---- 测试辅助 ----

// newTemplateTestChain 返回一条只含确定性测试创世的链（高度 0），
// 以及一个持有创世奖励的矿工钱包。
func newTemplateTestChain(t *testing.T) (*blockchain.Blockchain, *wallet.Wallet) {
	t.Helper()
	miner := newTestWallet(t)
	bc, err := blockchain.NewBlockchainWithGenesis(mineGenesis(t, miner))
	if err != nil {
		t.Fatalf("初始化链失败: %v", err)
	}
	return bc, miner
}

// mineCandidate 在给定链上求解出一个 PoW 合法的候选区块（不改动链状态）。
// 返回的区块等同于「矿工真实挖出但尚未上链」的区块。
func mineCandidate(t *testing.T, bc *blockchain.Blockchain, txs []*transaction.Transaction) *block.Block {
	t.Helper()
	tip, err := bc.Tip()
	if err != nil {
		t.Fatalf("读取链尾失败: %v", err)
	}
	b := block.NewCandidateBlock(tip.Header.Hash(), bc.CurrentBits(), txs)
	if found, _ := pow.Mine(b); !found {
		t.Fatal("候选区块 PoW 求解失败")
	}
	return b
}

// ---- §4 CRITICAL INVARIANT：两条路径共用同一套规则 ----

// TestValidateTemplateSharesOneRuleSetWithValidateBlock 逐条规则比对两条路径。
//
// 构造方式：**先篡改、后求解 PoW**。这一点是必要的——若先求解再篡改头部，
// 被篡改头部的 PoW 也会失效，ValidateBlock 会在第 2 步（PoW）就短路返回，
// 从而观察不到后续规则的差异；先篡改再求解可保证两条路径都能推进到被判定的那一步。
//
// 断言三项：
//  1. 两条路径都拒绝；
//  2. 两条路径拒绝的**哨兵错误**相同（errors.Is 命中同一个 sentinel）；
//  3. 两条路径的**完整错误文本**逐字节相同 —— 这是「同一份规则、同一顺序」的直接证据。
func TestValidateTemplateSharesOneRuleSetWithValidateBlock(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, bc *blockchain.Blockchain, miner *wallet.Wallet, b *block.Block)
		want   error
	}{
		{
			name: "规则1 前置哈希不匹配",
			mutate: func(_ *testing.T, _ *blockchain.Blockchain, _ *wallet.Wallet, b *block.Block) {
				b.Header.PrevBlockHash = [32]byte{0xAA}
			},
			want: blockchain.ErrInvalidPrevHash,
		},
		{
			name: "规则3 难度位与共识难度不一致",
			mutate: func(_ *testing.T, _ *blockchain.Blockchain, _ *wallet.Wallet, b *block.Block) {
				// 15 比共识（16）**更易**，因此重解 PoW 只需极少尝试；
				// 若写成更难的值，测试会白等——这一点本身就是「难度位必须被校验」的意义。
				b.Header.Bits = 15
			},
			want: blockchain.ErrUnexpectedBits,
		},
		{
			name: "规则4 时间戳早于父块",
			mutate: func(t *testing.T, bc *blockchain.Blockchain, _ *wallet.Wallet, b *block.Block) {
				tip, err := bc.Tip()
				if err != nil {
					t.Fatalf("读取链尾失败: %v", err)
				}
				b.Header.Timestamp = tip.Header.Timestamp - 1
			},
			want: blockchain.ErrTimestampOutOfRange,
		},
		{
			name: "规则4 时间戳大幅超前本地时钟",
			mutate: func(_ *testing.T, _ *blockchain.Blockchain, _ *wallet.Wallet, b *block.Block) {
				// 余量必须**远大于**本用例自身的运行时间：允许漂移是 7200 秒，
				// 而本用例在篡改之后还要解一次 PoW（可能因机器负载耗时至秒级）。
				// 若只超出 1 秒，时钟一走就落回允许区间，用例会随机通过——
				// 这正是「测试必须与运行时长无关」的地方。取 1 天余量即可确定性成立。
				b.Header.Timestamp = time.Now().Unix() + 7200 + 86400
			},
			want: blockchain.ErrTimestampOutOfRange,
		},
		{
			name: "规则5 Merkle 根与交易集合不匹配",
			mutate: func(_ *testing.T, _ *blockchain.Blockchain, _ *wallet.Wallet, b *block.Block) {
				b.Header.MerkleRoot = [32]byte{0x01}
			},
			want: blockchain.ErrMerkleMismatch,
		},
		{
			name: "规则6 区块体积超限",
			mutate: func(_ *testing.T, _ *blockchain.Blockchain, miner *wallet.Wallet, b *block.Block) {
				// 单个 coinbase 携带远超上限的输出数：编码后必然 > MaxBlockSize(1 MiB)。
				// 输出数故意取大值，避免依赖具体编码长度的边界巧合。
				outs := make([]transaction.TxOutput, 45000)
				for i := range outs {
					outs[i] = transaction.TxOutput{Value: 1, PubKeyHash: miner.PubKeyHash()}
				}
				cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), 1, 1)
				cb.Outputs = outs
				b.Transactions = []*transaction.Transaction{cb}
				b.Header.MerkleRoot = block.ComputeMerkleRoot(b.Transactions)
			},
			want: blockchain.ErrBlockTooLarge,
		},
		{
			name: "规则7 交易层 coinbase 每个输出金额不得为 0",
			mutate: func(_ *testing.T, _ *blockchain.Blockchain, miner *wallet.Wallet, b *block.Block) {
				cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), 0, 1)
				b.Transactions = []*transaction.Transaction{cb}
				b.Header.MerkleRoot = block.ComputeMerkleRoot(b.Transactions)
			},
			want: blockchain.ErrBadTxLayout,
		},
		{
			name: "规则7 交易层 coinbase 总额不得超补贴加手续费",
			mutate: func(_ *testing.T, _ *blockchain.Blockchain, miner *wallet.Wallet, b *block.Block) {
				cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(1)+1, 1)
				b.Transactions = []*transaction.Transaction{cb}
				b.Header.MerkleRoot = block.ComputeMerkleRoot(b.Transactions)
			},
			want: blockchain.ErrBadTxLayout,
		},
		{
			name: "规则7 交易层 未成熟 coinbase 不得被花费",
			mutate: func(t *testing.T, bc *blockchain.Blockchain, miner *wallet.Wallet, b *block.Block) {
				snap := bc.UTXOSnapshot()
				var op utxo.OutPoint
				var found bool
				for o, e := range snap.AllEntries() {
					if e.IsCoinbase {
						op, found = o, true
						break
					}
				}
				if !found {
					t.Fatal("创世后应存在一个 coinbase 输出")
				}
				// 高度 1 上花费高度 0 的 coinbase：maturity(10) 尚未满足
				spend := spendFrom(t, op, miner, [20]byte{0x22}, 50)
				cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(1), 1)
				b.Transactions = []*transaction.Transaction{cb, spend}
				b.Header.MerkleRoot = block.ComputeMerkleRoot(b.Transactions)
			},
			want: blockchain.ErrBadTxLayout,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bc, miner := newTemplateTestChain(t)

			txs := []*transaction.Transaction{
				transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(1), 1),
			}
			b := block.NewCandidateBlock(mustTipHash(t, bc), bc.CurrentBits(), txs)
			tc.mutate(t, bc, miner, b)
			if found, _ := pow.Mine(b); !found {
				t.Fatal("篡改后的候选区块 PoW 求解失败")
			}

			tmplErr := bc.ValidateTemplate(b)
			blkErr := bc.ValidateBlock(b)

			if tmplErr == nil {
				t.Fatal("ValidateTemplate 未拒绝该模板（预校验存在规则漏洞）")
			}
			if blkErr == nil {
				t.Fatal("ValidateBlock 未拒绝该区块（入选路径存在规则漏洞）")
			}
			if !errors.Is(tmplErr, tc.want) {
				t.Fatalf("ValidateTemplate 拒绝原因 = %v, want errors.Is(%v)", tmplErr, tc.want)
			}
			if !errors.Is(blkErr, tc.want) {
				t.Fatalf("ValidateBlock 拒绝原因 = %v, want errors.Is(%v)", blkErr, tc.want)
			}
			// 核心断言：两条路径必须给出**逐字相同**的拒绝原因。
			if tmplErr.Error() != blkErr.Error() {
				t.Fatalf("两条路径拒绝原因不一致（说明已是两套规则）:\n  ValidateTemplate = %q\n  ValidateBlock    = %q",
					tmplErr.Error(), blkErr.Error())
			}
			// 该模板不得被接受上链
			if err := bc.AddBlock(b); err == nil {
				t.Fatal("被拒绝的区块竟被 AddBlock 接受")
			}
			if bc.Height() != 0 {
				t.Fatalf("链高 = %d, want 0（不得有任何区块上链）", bc.Height())
			}
		})
	}
}

// TestValidateTemplateSkipsOnlyThePoWStep 是 skipPoW 开关的**边界证明**。
//
// 光有「两条路径在失败时一致」还不足以证明「唯一差异只是 PoW」：还需要证明
// 存在一类输入，使两条路径**恰好只在这一步上分岔**。本用例即构造该输入：
// 结构完全合法、仅 Nonce 不满足难度目标。
func TestValidateTemplateSkipsOnlyThePoWStep(t *testing.T) {
	bc, miner := newTemplateTestChain(t)

	cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(1), 1)
	b := mineCandidate(t, bc, []*transaction.Transaction{cb})

	// 基线：已求解的合法区块被两条路径同时接受
	if err := bc.ValidateTemplate(b); err != nil {
		t.Fatalf("已求解的合法模板被 ValidateTemplate 拒绝: %v", err)
	}
	if err := bc.ValidateBlock(b); err != nil {
		t.Fatalf("已求解的合法区块被 ValidateBlock 拒绝: %v", err)
	}

	// 只破坏 PoW：把 Nonce 移到第一个不满足难度目标的取值
	goodNonce := b.Header.Nonce
	badNonce := goodNonce
	for i := uint64(1); i <= 64; i++ {
		b.Header.Nonce = goodNonce + i
		if !pow.Validate(&b.Header) {
			badNonce = goodNonce + i
			break
		}
	}
	if badNonce == goodNonce {
		t.Fatal("连续 64 个 Nonce 都满足难度目标，测试前提不成立")
	}
	b.Header.Nonce = badNonce

	if err := bc.ValidateBlock(b); !errors.Is(err, blockchain.ErrInvalidPoW) {
		t.Fatalf("ValidateBlock 未按 PoW 拒绝（网络入选路径的 DoS 保护失效）: %v", err)
	}
	if err := bc.ValidateTemplate(b); err != nil {
		t.Fatalf("ValidateTemplate 不应校验 PoW，却拒绝了该模板: %v", err)
	}

	// 反向确认：除 PoW 外的规则仍然生效（否则就成了「skipPoW 顺手跳过了别的规则」）
	b.Header.PrevBlockHash = [32]byte{0xEE}
	if err := bc.ValidateBlock(b); !errors.Is(err, blockchain.ErrInvalidPrevHash) {
		t.Fatalf("ValidateBlock 在 PoW 已失效时未按规则顺序先报前置哈希错误: %v", err)
	}
	if err := bc.ValidateTemplate(b); !errors.Is(err, blockchain.ErrInvalidPrevHash) {
		t.Fatalf("ValidateTemplate 漏检前置哈希: %v", err)
	}
}

// TestValidateTemplateDoesNotMutateChainState 锁定「预校验是只读操作」。
//
// 这一点是安全的必要条件：预校验发生在**每次挖矿循环迭代**中（频率与重试次数同阶），
// 一旦它污染链状态（例如把模板的交易写入 UTXO 集合），结果是不可逆的数据损坏。
// 实现上依赖 validateBlock 在 UTXO 克隆上做迁移（apply.go 的 working := base.Clone()），
// 本用例是该实现细节的**行为级**守护。
func TestValidateTemplateDoesNotMutateChainState(t *testing.T) {
	bc, miner := newTemplateTestChain(t)

	beforeTip, err := bc.Tip()
	if err != nil {
		t.Fatalf("读取链尾失败: %v", err)
	}
	beforeHash := beforeTip.Header.Hash()
	beforeEntries := len(bc.UTXOSnapshot().AllEntries())

	cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(1), 1)
	tmpl := block.NewCandidateBlock(beforeHash, bc.CurrentBits(), []*transaction.Transaction{cb})
	if err := bc.ValidateTemplate(tmpl); err != nil {
		t.Fatalf("合法模板被预校验拒绝: %v", err)
	}

	// 1. 链高/链尾/UTXO 集合均不变
	if bc.Height() != 0 {
		t.Fatalf("预校验后链高 = %d, want 0", bc.Height())
	}
	afterTip, err := bc.Tip()
	if err != nil {
		t.Fatalf("读取链尾失败: %v", err)
	}
	if afterTip.Header.Hash() != beforeHash {
		t.Fatalf("预校验改变了链尾: %s → %s", beforeHash, afterTip.Header.HashHex())
	}
	if got := len(bc.UTXOSnapshot().AllEntries()); got != beforeEntries {
		t.Fatalf("预校验改变了 UTXO 集合大小: %d → %d", beforeEntries, got)
	}
	// 2. 模板没有被写入链
	if _, err := bc.BlockByHeight(1); err == nil {
		t.Fatal("预校验把未求解的模板写入了链")
	}
	// 3. 未求解的模板仍不能通过正式入选路径（预校验不构成任何 PoW 豁免）
	if err := bc.AddBlock(tmpl); !errors.Is(err, blockchain.ErrInvalidPoW) {
		t.Fatalf("AddBlock 未按 PoW 拒绝未求解模板: %v", err)
	}
	if bc.Height() != 0 {
		t.Fatalf("链高 = %d, want 0", bc.Height())
	}
}

// ---- §12 Test D 的验证层形式：补贴归零后合法候选集合为空 ----

// TestZeroSubsidyCoinbaseAdmissibleSetIsEmpty 用**生产共识函数**直接证明
// 「Subsidy == 0 且 fees == 0 时不存在任何合法 coinbase」。
//
// 手法：不构造长链，而是把 `height` 作为参数直接交给 `utxo.ApplyBlock`
// （它的签名本就接受高度，是实现里唯一的高度来源）。因此本用例调用的是**真实共识代码**，
// 只是在真实边界高度上调用，从而避开「挖 1260 个区块」的成本，
// 同时不引入任何测试专用的规则副本。
//
// 两条共识规则的合取构成空集：
//
//	规则 α（结构）：coinbase 必须至少有一个输出，且每个输出金额 != 0  ⇒  输出总额 ≥ 1
//	规则 β（上限）：coinbase 输出总额 ≤ Subsidy(height) + fees(height)  ⇒  总额 ≤ 0（当 Subsidy=0, fees=0）
//
// α ∧ β 在 Subsidy(height)+fees == 0 时不可满足。
//
// 边界对照是**决定性**的：完全相同的 coinbase（总额 1）在高度 1259 被接受、
// 在高度 1260 被拒绝 —— 唯一的自变量是补贴归零。这排除了「拒绝其实来自别的原因」。
func TestZeroSubsidyCoinbaseAdmissibleSetIsEmpty(t *testing.T) {
	// 前提：补贴恰在 1259→1260 之间归零（50 = 0b110010，仅 6 位，故第 6 次减半即为 0）
	if got := utxo.Subsidy(1259); got != 1 {
		t.Fatalf("Subsidy(1259) = %d, want 1（最后一个非零补贴高度）", got)
	}
	if got := utxo.Subsidy(1260); got != 0 {
		t.Fatalf("Subsidy(1260) = %d, want 0（补贴归零边界）", got)
	}

	miner := newTestWallet(t)

	// 基础集合：仅含高度 0 的 coinbase（矿工持有 50），供手续费交易消费
	genesisCb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(0), 0)
	base, _, err := utxo.ApplyBlock(utxo.NewUTXOSet(), []*transaction.Transaction{genesisCb}, 0)
	if err != nil {
		t.Fatalf("构造基础 UTXO 集合失败: %v", err)
	}

	var cbOp utxo.OutPoint
	var cbValue uint64
	for op, e := range base.AllEntries() {
		if e.IsCoinbase {
			cbOp, cbValue = op, e.Value
			break
		}
	}
	if cbValue == 0 {
		t.Fatal("基础集合中应存在金额非零的 coinbase 输出")
	}

	t.Run("规则α 总额0被零值输出规则拒绝", func(t *testing.T) {
		// 总额 0 只有一种实现：一个金额为 0 的输出
		zero := transaction.NewCoinbaseTx(miner.PubKeyHash(), 0, 1260)
		_, _, err := utxo.ApplyBlock(base, []*transaction.Transaction{zero}, 1260)
		if err == nil {
			t.Fatal("总额为 0 的 coinbase 被接受（规则α失效）")
		}
		if !errors.Is(err, utxo.ErrZeroOutput) {
			t.Fatalf("拒绝原因 = %v, want ErrZeroOutput", err)
		}
	})

	t.Run("规则β 总额1被上限规则拒绝（高度1260）", func(t *testing.T) {
		// 总额 1 是满足规则α的最小正数
		one := transaction.NewCoinbaseTx(miner.PubKeyHash(), 1, 1260)
		_, _, err := utxo.ApplyBlock(base, []*transaction.Transaction{one}, 1260)
		if err == nil {
			t.Fatal("补贴为 0 且无手续费时，总额 1 的 coinbase 被接受（规则β失效）")
		}
		if !errors.Is(err, utxo.ErrExcessiveCoinbase) {
			t.Fatalf("拒绝原因 = %v, want ErrExcessiveCoinbase", err)
		}
		if !strings.Contains(err.Error(), "奖励 0 + 手续费 0") {
			t.Fatalf("拒绝原因未体现「补贴 0 + 手续费 0」: %v", err)
		}
	})

	t.Run("边界对照 同一个coinbase在高度1259被接受", func(t *testing.T) {
		// 与上一个子用例**完全相同**的 coinbase，只把高度退回 1259（补贴 1）
		one := transaction.NewCoinbaseTx(miner.PubKeyHash(), 1, 1259)
		if _, _, err := utxo.ApplyBlock(base, []*transaction.Transaction{one}, 1259); err != nil {
			t.Fatalf("高度 1259（补贴 1）下总额 1 的 coinbase 应被接受: %v", err)
		}
	})

	t.Run("手续费为正时合法候选重新存在（fee-only 出块）", func(t *testing.T) {
		// 高度 1260：coinbase 总额 = 补贴 0 + 手续费 1 = 1，恰好落在规则β的边界上
		one := transaction.NewCoinbaseTx(miner.PubKeyHash(), 1, 1260)
		// 构造一笔真实手续费交易：消费金额 cbValue，输出 cbValue-1 ⇒ 手续费 1
		spend := spendFrom(t, cbOp, miner, [20]byte{0x33}, cbValue-1)

		set, fees, err := utxo.ApplyBlock(base, []*transaction.Transaction{one, spend}, 1260)
		if err != nil {
			t.Fatalf("补贴为 0 但手续费为 1 时，零补贴区块应合法: %v", err)
		}
		if fees != 1 {
			t.Fatalf("区块手续费合计 = %d, want 1", fees)
		}
		// coinbase 与找零都必须真实进入 UTXO 集合
		if got := len(set.AllEntries()); got != 2 {
			t.Fatalf("应用后 UTXO 条目数 = %d, want 2（coinbase + 找零）", got)
		}
	})
}

// ---- §12 Test F 的验证层形式：陈旧模板必须被识别为「可重试」而非「非法」 ----

// TestValidateTemplateReportsStaleTemplateAfterTipAdvances 证明「模板所依据的链尾
// 被替换」这一情形被稳定地归入 ErrInvalidPrevHash，从而使上层可以只做「重建 + 重试」，
// 而不必（也不允许）把它误判为结构性缺陷并终止挖矿。
func TestValidateTemplateReportsStaleTemplateAfterTipAdvances(t *testing.T) {
	bc, miner := newTemplateTestChain(t)

	// 在高度 1 上组装一个合法模板（但**不做 PoW**）
	stale := block.NewCandidateBlock(mustTipHash(t, bc), bc.CurrentBits(), []*transaction.Transaction{
		transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(1), 1),
	})
	if err := bc.ValidateTemplate(stale); err != nil {
		t.Fatalf("陈旧化之前的模板应当是合法的: %v", err)
	}

	// 链尾推进（等价于拿到一个对端区块）
	mineBlock(t, bc, miner)
	if bc.Height() != 1 {
		t.Fatalf("链高 = %d, want 1", bc.Height())
	}

	err := bc.ValidateTemplate(stale)
	if err == nil {
		t.Fatal("链尾已变化，旧模板竟仍被判定合法（会导致基于错误父块求解 PoW）")
	}
	if !errors.Is(err, blockchain.ErrInvalidPrevHash) {
		t.Fatalf("拒绝原因 = %v, want ErrInvalidPrevHash", err)
	}
	if !blockchain.IsTemplateStale(err) {
		t.Fatalf("该失败必须被识别为「可重试（陈旧）」，实际 IsTemplateStale=false: %v", err)
	}
}

// TestValidateTemplateReportsStaleWhenNewTipTimestampLeads 覆盖陈旧判定的第二条路径：
// 模板的 PrevHash 未被替换，但新链尾的时间戳已前移到模板时间戳之后。
// 这在真实网络中就是「对端区块领先本节点数十秒」的常态，必须同样归入可重试。
func TestValidateTemplateReportsStaleWhenNewTipTimestampLeads(t *testing.T) {
	bc, miner := newTemplateTestChain(t)

	// 造一个时间戳领先本地时钟 1 小时的「高度 1」区块。
	//
	// 余量取 1 小时（而不是几秒）同样是**为了与运行时长解耦**：本用例在构造该区块
	// 之后还要解一次 PoW，随后才构造模板；只要这段时间不超过 1 小时，
	// 「模板时间戳 < 链尾时间戳」就确定成立。上限仍须小于允许漂移 7200 秒，
	// 否则 AddBlock 会先以「超前太多」拒绝它。
	lead := block.NewCandidateBlock(mustTipHash(t, bc), bc.CurrentBits(), []*transaction.Transaction{
		transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(1), 1),
	})
	lead.Header.Timestamp = time.Now().Unix() + 3600
	if found, _ := pow.Mine(lead); !found {
		t.Fatal("领先时间戳区块 PoW 求解失败")
	}
	if err := bc.AddBlock(lead); err != nil {
		t.Fatalf("领先时间戳的合法区块被拒绝: %v", err)
	}

	// 模板按「当前时间」构造 —— 其时间戳必然早于新链尾
	tmpl := block.NewCandidateBlock(mustTipHash(t, bc), bc.CurrentBits(), []*transaction.Transaction{
		transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(2), 2),
	})
	err := bc.ValidateTemplate(tmpl)
	if err == nil {
		t.Fatal("时间戳落后于新链尾的模板竟被判定合法")
	}
	if !errors.Is(err, blockchain.ErrTimestampOutOfRange) {
		t.Fatalf("拒绝原因 = %v, want ErrTimestampOutOfRange", err)
	}
	if !blockchain.IsTemplateStale(err) {
		t.Fatalf("该失败必须被识别为「可重试（陈旧）」: %v", err)
	}
}

// TestIsTemplateStaleClassifiesOnlyRetryableCauses 锁定分类边界。
//
// 这个分类的**代价是不对称的**，因此必须逐个锁定：
//   - 把可重试误判为结构性 ⇒ 挖矿被永久终止（活性丧失）；
//   - 把结构性误判为可重试 ⇒ 无意义 PoW 无限重试（正是本阶段要消灭的行为）。
//
// 因此「哪些错误属于可重试」是一份**白名单**，而非默认行为。
func TestIsTemplateStaleClassifiesOnlyRetryableCauses(t *testing.T) {
	retryable := []struct {
		name string
		err  error
	}{
		{"链尾已被替换", blockchain.ErrInvalidPrevHash},
		{"时间戳越界", blockchain.ErrTimestampOutOfRange},
		{"共识难度位变化", blockchain.ErrUnexpectedBits},
	}
	for _, tc := range retryable {
		if !blockchain.IsTemplateStale(tc.err) {
			t.Fatalf("%s（%v）应被判定为可重试", tc.name, tc.err)
		}
	}

	notRetryable := []struct {
		name string
		err  error
	}{
		{"Merkle 不匹配", blockchain.ErrMerkleMismatch},
		{"PoW 未满足（模板路径本应看不到）", blockchain.ErrInvalidPoW},
		{"交易布局非法", blockchain.ErrBadTxLayout},
		{"体积超限", blockchain.ErrBlockTooLarge},
		{"未知错误", errors.New("某种未分类的模板错误")},
		{"nil", nil},
	}
	for _, tc := range notRetryable {
		if blockchain.IsTemplateStale(tc.err) {
			t.Fatalf("%s（%v）不应被判定为可重试（会导致无意义 PoW 无限重试）", tc.name, tc.err)
		}
	}
}

// TestNetworkEntryPointsStillEnforcePoW 是 skipPoW 的**越权使用防护**回归。
//
// 本阶段引入了「跳过 PoW 的校验入口」。它一旦被接入网络入块路径，就等于
// 允许任何人凭一个 88 字节的头部结构造区块（放弃防伪造区块的 DoS 保护）。
// 本用例把三条正式入口全部钉死：ValidateBlock / AddBlock / 启动回放。
func TestNetworkEntryPointsStillEnforcePoW(t *testing.T) {
	bc, miner := newTemplateTestChain(t)

	unmined := block.NewCandidateBlock(mustTipHash(t, bc), bc.CurrentBits(), []*transaction.Transaction{
		transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(1), 1),
	})

	if err := bc.ValidateBlock(unmined); !errors.Is(err, blockchain.ErrInvalidPoW) {
		t.Fatalf("ValidateBlock 接受未求解区块: %v", err)
	}
	if err := bc.AddBlock(unmined); !errors.Is(err, blockchain.ErrInvalidPoW) {
		t.Fatalf("AddBlock 接受未求解区块: %v", err)
	}
	// 启动回放路径：用「已求解但结构非法」的区块确认回放同样不做 PoW 豁免
	mined := mineCandidate(t, bc, []*transaction.Transaction{
		transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(1), 1),
	})
	mined.Header.PrevBlockHash = [32]byte{0x7F}
	if err := bc.ValidateBlock(mined); !errors.Is(err, blockchain.ErrInvalidPrevHash) {
		t.Fatalf("ValidateBlock 在 PoW 合法时未按规则顺序拒绝错误父块: %v", err)
	}
	if bc.Height() != 0 {
		t.Fatalf("链高 = %d, want 0", bc.Height())
	}
}

// ---- 工具 ----

// mustTipHash 返回当前链尾哈希（失败即终止用例）。
func mustTipHash(t *testing.T, bc *blockchain.Blockchain) [32]byte {
	t.Helper()
	tip, err := bc.Tip()
	if err != nil {
		t.Fatalf("读取链尾失败: %v", err)
	}
	return tip.Header.Hash()
}
