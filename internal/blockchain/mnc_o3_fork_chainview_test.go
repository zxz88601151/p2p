package blockchain

// mnc_o3_fork_chainview_test.go —— MNC-IMPLEMENT-01 §3 O-3 修复的回归测试。
//
// 目标（OD-15 §9 O-3 / IMPLEMENT-01 §8）：`validateForkBlockInner` 的**两个**
// canonical-view 消费者——难度 bits（步 4）与时间戳/MTP（步 5）——都必须改用
// **fork 自身祖先视图**（由 parentNode.PathToRoot() 构造），不得读 canonical bc.blocks。
//
// 覆盖：
//   A. 仅 bits 消费者分歧（MTP 完全一致）→ 隔离 bits；
//   B. 仅 MTP 消费者分歧（bits 完全一致）→ 隔离 MTP；
//   C. 两消费者同时分歧，且 fork 跨越 v2 激活边界 → 组合 + 确定性（连续两次校验，无 sleep）；
//   D. 真实 AddBlock/reorg 路径跨越激活边界（fork-local 校验下被接受）。
//
// 说明：A/B/C 用**合成 canonical 链**（完全掌控难度窗口与 MTP 输入），fork 块为真实挖矿
// 并写入 store(detached) + tree，因此 validateForkBlockInner 的 PoW / Merkle / UTXO replay
// 全部走真实路径。激活高度注入为 11，以在低高度越过 v2 边界而无需真挖 2000 块。

import (
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/blocktree"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
)

const o3GenesisTS int64 = 1700000000 // NewGenesisBlock 的固定时间戳

// synthCanonicalChain 用给定 bits/时间戳重建 canonical 链 0..tip，并同步维护
// bc.blocks / hashIndex / canonicalSet / tree（创世保留 fixture 的 NewGenesisBlock）。
// 目的：让测试完全掌控难度窗口与 MTP 的输入，且不依赖墙钟。
func synthCanonicalChain(t *testing.T, f *reorgTestFixture, tip int,
	bitsFn func(h int) uint32, tsFn func(h int) int64) {
	t.Helper()
	bc := f.chain
	genesis := f.genesis

	tree := blocktree.NewBlockTree()
	if _, err := tree.AddBlock(genesis.Header.Hash(), [32]byte{}, 0,
		genesis.Header.Bits, genesis.Header.Timestamp); err != nil {
		t.Fatalf("tree genesis: %v", err)
	}
	bc.blocks = []*block.Block{genesis}
	bc.hashIndex = map[[32]byte]*block.Block{genesis.Header.Hash(): genesis}
	bc.canonicalSet = map[[32]byte]struct{}{genesis.Header.Hash(): {}}

	prev := genesis.Header.Hash()
	for h := 1; h <= tip; h++ {
		cb := transaction.NewCoinbaseTx([20]byte{0xC0}, utxo.Subsidy(h), h)
		b := &block.Block{
			Header: block.Header{
				Version:       pow.VersionForHeight(h, bc.activationHeight),
				PrevBlockHash: prev,
				Timestamp:     tsFn(h),
				Bits:          bitsFn(h),
				Nonce:         uint64(h),
			},
			Transactions: []*transaction.Transaction{cb},
		}
		b.Header.MerkleRoot = block.ComputeMerkleRoot(b.Transactions)
		prev = b.Header.Hash()

		bc.blocks = append(bc.blocks, b)
		bc.hashIndex[b.Header.Hash()] = b
		bc.canonicalSet[b.Header.Hash()] = struct{}{}
		if _, err := tree.AddBlock(b.Header.Hash(), b.Header.PrevBlockHash, h,
			b.Header.Bits, b.Header.Timestamp); err != nil {
			t.Fatalf("tree h=%d: %v", h, err)
		}
	}
	bc.tree = tree
	if last := bc.blocks[len(bc.blocks)-1]; last != nil {
		if n := tree.LookupNode(last.Header.Hash()); n != nil {
			_ = tree.SetTip(n)
		}
	}

	// 将合成 canonical 块按 v2 路径持久化：SaveBlockDetached（fork 块）要求父块已登记，
	// 且 fork 祖先经 blockAtHash 需可从 store 取回。
	v2s, ok := interface{}(f.store).(reorgStore)
	if !ok {
		t.Fatal("store 不支持 v2 reorg 原语")
	}
	for h := 1; h <= tip; h++ {
		if err := v2s.AppendCanonicalBlock(bc.blocks[h], utxo.BlockUndo{Height: h}); err != nil {
			t.Fatalf("持久化合成 canonical h=%d 失败: %v", h, err)
		}
	}
}

// buildFork 从 diverge 高度起构造 fork 分支至 tip（真实挖矿），以 detached 形式写入
// store 并加入 tree（**不**经过 AddBlock，避免触发 reorg）。返回按高度索引的节点与区块。
func buildFork(t *testing.T, f *reorgTestFixture, diverge, tip int,
	bitsFn func(h int) uint32, tsFn func(h int) int64) (map[int]*blocktree.BlockNode, map[int]*block.Block) {
	t.Helper()
	bc := f.chain
	parentHash := bc.blocks[diverge-1].Header.Hash()
	nodes := make(map[int]*blocktree.BlockNode, tip-diverge+1)
	blocks := make(map[int]*block.Block, tip-diverge+1)

	for h := diverge; h <= tip; h++ {
		cb := transaction.NewCoinbaseTx([20]byte{0xF0}, utxo.Subsidy(h), h)
		b := block.NewCandidateBlock(parentHash, bitsFn(h), []*transaction.Transaction{cb})
		b.Header.Version = pow.VersionForHeight(h, bc.activationHeight)
		b.Header.Timestamp = tsFn(h)
		if found, _ := pow.Mine(b); !found {
			t.Fatalf("fork 挖矿失败 h=%d", h)
		}
		f.saveDetached(b)
		n, err := bc.tree.AddBlock(b.Header.Hash(), parentHash, h, b.Header.Bits, b.Header.Timestamp)
		if err != nil {
			t.Fatalf("fork tree h=%d: %v", h, err)
		}
		nodes[h] = n
		blocks[h] = b
		parentHash = b.Header.Hash()
	}
	return nodes, blocks
}

// TestO3ForkBitsConsumerUsesForkLocalView 隔离 **bits 消费者**：
// fork 块 19 的 bits=18（时间戳与 canonical 相同 ⇒ MTP 完全一致），
// 使 h=20（周期边界）期望难度由 parent.Bits 决定 → fork 期望 18、canonical 期望 16。
func TestO3ForkBitsConsumerUsesForkLocalView(t *testing.T) {
	f := newReorgFixture(t)
	f.chain.activationHeight = 11
	synthCanonicalChain(t, f, 20,
		func(int) uint32 { return pow.MaxTargetBits },
		func(h int) int64 { return o3GenesisTS + int64(100*h) })
	bc := f.chain

	nodes, blocks := buildFork(t, f, 19, 20,
		func(h int) uint32 { return 18 }, // 块 19 与 20 的 bits 均为 18
		func(h int) int64 {
			if h == 19 {
				return o3GenesisTS + 1900 // 与 canonical ts(19) 相同 ⇒ MTP 一致
			}
			return o3GenesisTS + 1501 // > MTP(19)=1700001500
		})

	view := forkView{bc: bc, node: nodes[19]}

	// MTP 必须一致（本测试隔离 bits，故 MTP 不是分歧点）。
	canonMTP := bc.medianTimePast(19)
	forkMTP := bc.medianTimePastView(view, 19)
	if canonMTP != forkMTP {
		t.Fatalf("前置条件失败：MTP 应一致，canonical=%d fork=%d", canonMTP, forkMTP)
	}
	if canonMTP != o3GenesisTS+1400 {
		t.Fatalf("canonical MTP(19)=%d, want %d", canonMTP, o3GenesisTS+1400)
	}

	// bits 必须分歧：canonical=16（AdjustBits(16,1900)），fork=18（AdjustBits(18,1900)）。
	if got := bc.expectedBitsFor(20); got != 16 {
		t.Fatalf("canonical expectedBits(20)=%d, want 16", got)
	}
	if got := bc.expectedBitsForView(view, 20); got != 18 {
		t.Fatalf("fork expectedBits(20)=%d, want 18（fork-local parent.Bits=18）", got)
	}

	// 决定性断言：fork 块必须被接受（旧 canonical-view 实现会因 bits 16≠18 误拒）。
	if err := bc.validateForkBlock(blocks[20], nodes[19]); err != nil {
		t.Fatalf("fork 块被误拒（O-3 回归）：%v", err)
	}
	// 负向对照：canonical 视图对同一块报错 ⇒ 证明本修复是必要的（非恒真）。
	if err := bc.validateBitsWithView(unsafeView{bc}, blocks[20], 20); err == nil {
		t.Fatal("负向对照失败：canonical 视图不应接受该 fork 块的 bits")
	}
}

// TestO3ForkMTPConsumerUsesForkLocalView 隔离 **时间戳/MTP 消费者**：
// fork 全部 bits=16（与 canonical 相同 ⇒ bits 无差异），但 fork 时间戳使 MTP 分歧。
func TestO3ForkMTPConsumerUsesForkLocalView(t *testing.T) {
	f := newReorgFixture(t)
	f.chain.activationHeight = 11
	synthCanonicalChain(t, f, 21,
		func(int) uint32 { return pow.MaxTargetBits },
		func(h int) int64 { return o3GenesisTS + int64(100*h) })
	bc := f.chain

	nodes, blocks := buildFork(t, f, 10, 21,
		func(int) uint32 { return pow.MaxTargetBits },
		func(h int) int64 {
			if h == 21 {
				return o3GenesisTS + 501
			}
			return o3GenesisTS + 500
		})

	view := forkView{bc: bc, node: nodes[20]}

	// bits 必须一致（h=21 非周期边界 → 均沿用父块 bits=16）。
	canonBits := bc.expectedBitsFor(21)
	forkBits := bc.expectedBitsForView(view, 21)
	if canonBits != forkBits || canonBits != 16 {
		t.Fatalf("前置条件失败：bits 应一致且为 16，canonical=%d fork=%d", canonBits, forkBits)
	}

	// MTP 必须分歧：canonical=1700001600（窗口 [11,20]），fork=1700000500（fork 块 11..20 同值）。
	if got := bc.medianTimePast(20); got != o3GenesisTS+1500 {
		t.Fatalf("canonical MTP(20)=%d, want %d", got, o3GenesisTS+1500)
	}
	if got := bc.medianTimePastView(view, 20); got != o3GenesisTS+500 {
		t.Fatalf("fork MTP(20)=%d, want %d（fork-local 时间戳）", got, o3GenesisTS+500)
	}

	// 决定性断言：fork 块必须被接受（旧 canonical-view 实现会因 ts < canonical MTP 误拒）。
	if err := bc.validateForkBlock(blocks[21], nodes[20]); err != nil {
		t.Fatalf("fork 块被误拒（O-3 回归）：%v", err)
	}
	// 负向对照：canonical 视图对同一块报错 ⇒ 证明本修复是必要的（非恒真）。
	if err := bc.validateTimestampWithView(unsafeView{bc}, blocks[21], blocks[20], 21); err == nil {
		t.Fatal("负向对照失败：canonical 视图不应接受该 fork 块的 MTP")
	}
}

// TestO3ForkBothConsumersCombined 两个消费者同时分歧，且 fork 跨越 v2 激活边界（11）。
// 连续两次校验必须得到相同结果（确定性，无 sleep）。
func TestO3ForkBothConsumersCombined(t *testing.T) {
	f := newReorgFixture(t)
	f.chain.activationHeight = 11
	synthCanonicalChain(t, f, 20,
		func(int) uint32 { return pow.MaxTargetBits },
		func(h int) int64 { return o3GenesisTS + int64(100*h) })
	bc := f.chain

	// fork 从高度 9 起（跨越激活边界 11）：块 9..19 bits=16、ts 全 1700000300；
	// 块 20 bits=18。bits 与 MTP 同时与 canonical 分歧。
	nodes, blocks := buildFork(t, f, 9, 20,
		func(h int) uint32 {
			if h == 20 {
				return 18
			}
			return pow.MaxTargetBits
		},
		func(h int) int64 {
			if h == 20 {
				return o3GenesisTS + 401
			}
			return o3GenesisTS + 300
		})

	view := forkView{bc: bc, node: nodes[19]}

	if got := bc.expectedBitsFor(20); got != 16 {
		t.Fatalf("canonical expectedBits(20)=%d, want 16", got)
	}
	if got := bc.expectedBitsForView(view, 20); got != 18 {
		t.Fatalf("fork expectedBits(20)=%d, want 18", got)
	}
	if got := bc.medianTimePast(19); got != o3GenesisTS+1400 {
		t.Fatalf("canonical MTP(19)=%d, want %d", got, o3GenesisTS+1400)
	}
	if got := bc.medianTimePastView(view, 19); got != o3GenesisTS+300 {
		t.Fatalf("fork MTP(19)=%d, want %d", got, o3GenesisTS+300)
	}

	// 确定性：连续两次结果必须一致（无墙钟/无 sleep/无隐藏状态）。
	for i := 1; i <= 2; i++ {
		if err := bc.validateForkBlock(blocks[20], nodes[19]); err != nil {
			t.Fatalf("第 %d 次 fork 校验被拒: %v（fork-local 视图应接受）", i, err)
		}
	}
	// 负向对照：canonical 视图对 bits 与 MTP 均报错 ⇒ 两个消费者都必须修复（非恒真）。
	if err := bc.validateBitsWithView(unsafeView{bc}, blocks[20], 20); err == nil {
		t.Fatal("负向对照失败：canonical 视图不应接受该 fork 块的 bits")
	}
	if err := bc.validateTimestampWithView(unsafeView{bc}, blocks[20], blocks[19], 20); err == nil {
		t.Fatal("负向对照失败：canonical 视图不应接受该 fork 块的 MTP")
	}
}

// mineCanonicalActivated 在当前 canonical 链尾后真实挖一个合法块（版本/时间戳按规则设置）。
func mineCanonicalActivated(t *testing.T, bc *Blockchain) *block.Block {
	t.Helper()
	tip, err := bc.Tip()
	if err != nil {
		t.Fatalf("读取链尾失败: %v", err)
	}
	h := len(bc.blocks)
	cb := transaction.NewCoinbaseTx([20]byte{0xA0}, utxo.Subsidy(h), h)
	b := block.NewCandidateBlock(tip.Header.Hash(), bc.CurrentBits(), []*transaction.Transaction{cb})
	b.Header.Version = bc.RequiredVersionFor(h)
	b.Header.Timestamp = bc.MiningTimestamp(h)
	if found, _ := pow.Mine(b); !found {
		t.Fatal("canonical 挖矿失败")
	}
	if err := bc.AddBlock(b); err != nil {
		t.Fatalf("canonical AddBlock h=%d 失败: %v", h, err)
	}
	return b
}

// TestO3ReorgCrossingActivationBoundaryAccepted 走真实 AddBlock/reorg 路径：
// canonical 0..11，fork 从 9 起延伸至 16（17 块 > 12 块 ⇒ 触发 reorg）。
// fork 的每一块都经 validateForkBlock（fork-local 视图）校验，最终被接受为新链尾。
//
// 时间戳敏感说明（§5）：fork 时间戳取「父块 ts + 1」，对 canonical 墙钟时间戳存在依赖，
// 但断言不依赖具体数值（仅要求 reorg 被接受），故不受墙钟漂移影响。
func TestO3ReorgCrossingActivationBoundaryAccepted(t *testing.T) {
	f := newReorgFixture(t)
	f.chain.activationHeight = 11
	bc := f.chain

	for h := 1; h <= 11; h++ {
		mineCanonicalActivated(t, bc)
	}
	if len(bc.blocks) != 12 {
		t.Fatalf("canonical 高度应为 11（12 块），实际 %d", len(bc.blocks))
	}

	// fork 从高度 9 起延伸至 16；逐块 ts = 父 ts + 1。
	prevTs := bc.blocks[8].Header.Timestamp
	prevHash := bc.blocks[8].Header.Hash()
	var forkBlocks []*block.Block
	for h := 9; h <= 16; h++ {
		cb := transaction.NewCoinbaseTx([20]byte{0xF0}, utxo.Subsidy(h), h)
		b := block.NewCandidateBlock(prevHash, pow.MaxTargetBits, []*transaction.Transaction{cb})
		b.Header.Version = bc.RequiredVersionFor(h)
		b.Header.Timestamp = prevTs + 1
		if found, _ := pow.Mine(b); !found {
			t.Fatalf("fork 挖矿失败 h=%d", h)
		}
		forkBlocks = append(forkBlocks, b)
		prevHash = b.Header.Hash()
		prevTs = b.Header.Timestamp
	}

	// 逐个 AddBlock：前面的块仅登记为 fork 候选，最后一块因累积工作量更大而触发 reorg。
	for i, b := range forkBlocks {
		if err := bc.AddBlock(b); err != nil {
			t.Fatalf("fork AddBlock #%d (h=%d) 被拒: %v", i, i+9, err)
		}
	}

	tip, err := bc.Tip()
	if err != nil {
		t.Fatalf("读取链尾失败: %v", err)
	}
	want := forkBlocks[len(forkBlocks)-1].Header.Hash()
	got := tip.Header.Hash()
	if got != want {
		t.Fatalf("reorg 未接受 fork 分支：tip=%x want=%x", got[:8], want[:8])
	}
	if bc.Height() != 16 {
		t.Fatalf("reorg 后高度应为 16，实际 %d", bc.Height())
	}
}
