package blockchain_test

// PHASE MINING-2-I0-OBSERVABILITY-1 · §10 Test D：非干扰确定性对照。
//
// 设计：一次性挖出全部区块（块对象即「冻结输入」），然后把同一批块对象按同一
// 顺序在 obs on / off 两种状态下、各自全新的存储目录中重放，逐项比较：
//
//	block hash / AddBlock·ValidateBlock 结果（错误文本）/ Height / Tip。
//
// 为什么用「重放」而非两次独立挖矿：wallet.NewWallet 随机、候选块时间戳取
// time.Now() —— 两次独立挖矿不可能产出相同区块，逐字节对比无从谈起。
// 重放共享同一批 *block.Block 指针，两态输入逐字节相同，任何行为差异都只能
// 来自观测层本身 —— 这正是本测试要证明不存在的东西（OBSERVE ≠ CHANGE）。
//
// 为什么用 FileBlockStore 而非内存链（NewBlockchainWithGenesis）：
// REORG 场景中 fork 链的后续块校验需要 utxoAtNode 从创世重放父分支，detached
// 块必须可经 store.GetBlockByHash 取回；内存链 store=nil 且 MemoryBlockStore
// 不实现 reorgStore，fork 链第二块起必然校验失败（本测试初版实测命中）。
// SaveBlockDetached 首次调用即激活 v2 语义，无需预置 v2 状态。
//
// 断言面：
//  1. 两态 12 步结果逐项一致（含 3 个 REORG_REJECT、1 个 REORG_ACCEPT、
//     4 条非法块拒绝路径、reorg 后链延续）；
//  2. on 态事件流非空且含 REORG_* / FORK_VALIDATION_*（插桩活跃性证明，
//     排除「两边都静默所以相等」的假阳性）;
//  3. off 态零输出。
//
// 注意 SetOutputForTest 必须先于 EnableForTest：writerLoop 启动时一次性
// 快照输出目标（本测试初版实测：后设 buf 会全部打到 stderr）。

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/obs"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

// mineObsCandidate 只挖不投递：产出满足 PoW 的候选块，由调用方决定 add / validate。
func mineObsCandidate(t *testing.T, parent [32]byte, bits uint32, height int, miner *wallet.Wallet) *block.Block {
	t.Helper()
	cb := transaction.NewCoinbaseTx(miner.PubKeyHash(), utxo.Subsidy(height), height)
	cand := block.NewCandidateBlock(parent, bits, []*transaction.Transaction{cb})
	if found, _ := pow.Mine(cand); !found {
		t.Fatalf("高度 %d 候选块挖矿失败", height)
	}
	return cand
}

type obsOp struct {
	label  string
	action string // "add" | "validate"
	blk    *block.Block
}

type obsStep struct {
	label, action, hash8, errStr, tip8 string
	height                             int
}

func (s obsStep) equal(o obsStep) bool {
	return s.label == o.label && s.action == o.action && s.hash8 == o.hash8 &&
		s.errStr == o.errStr && s.tip8 == o.tip8 && s.height == o.height
}

// buildScenario 基于确定性创世挖出全部块（一次性，对象冻结）：
// 主链 b1..b3 → 等长 fork f1..f3（REORG_REJECT）→ 反超块 f4（REORG_ACCEPT）
// → 4 条非法校验路径 → reorg 后延续块 good5。
func buildScenario(t *testing.T, genesisHash [32]byte, bits uint32) []obsOp {
	t.Helper()
	minerA := newTestWallet(t)
	minerB := newTestWallet(t)

	b1 := mineObsCandidate(t, genesisHash, bits, 1, minerA)
	b2 := mineObsCandidate(t, b1.Header.Hash(), bits, 2, minerA)
	b3 := mineObsCandidate(t, b2.Header.Hash(), bits, 3, minerA)

	f1 := mineObsCandidate(t, genesisHash, bits, 1, minerB)
	f2 := mineObsCandidate(t, f1.Header.Hash(), bits, 2, minerB)
	f3 := mineObsCandidate(t, f2.Header.Hash(), bits, 3, minerB)
	f4 := mineObsCandidate(t, f3.Header.Hash(), bits, 4, minerB)

	// reorg 后 tip = f4。以下四个校验块均以 f4 为名义父块。
	badPrev := block.NewCandidateBlock([32]byte{0xEE}, bits, // 父不在树（校验先于 PoW，无需挖）
		[]*transaction.Transaction{transaction.NewCoinbaseTx(minerA.PubKeyHash(), utxo.Subsidy(5), 5)})
	wrongBits := mineObsCandidate(t, f4.Header.Hash(), bits-1, 5, minerA) // PoW 有效但 bits 不符共识
	oldTs := mineObsCandidate(t, f4.Header.Hash(), bits, 5, minerA)
	oldTs.Header.Timestamp = f4.Header.Timestamp - 1 // 改头后重挖，保持 PoW 有效
	if found, _ := pow.Mine(oldTs); !found {
		t.Fatal("oldTs 重挖失败")
	}
	tampered := mineObsCandidate(t, f4.Header.Hash(), bits, 5, minerA)
	tampered.Transactions[0].Outputs[0].Value = 999 // 头未重算 → Merkle 不匹配
	good5 := mineObsCandidate(t, f4.Header.Hash(), bits, 5, minerA)

	return []obsOp{
		{"b1@main", "add", b1},
		{"b2@main", "add", b2},
		{"b3@main", "add", b3},
		{"f1@fork-equal", "add", f1}, // 等长竞争 → REORG_REJECT
		{"f2@fork-equal", "add", f2},
		{"f3@fork-equal", "add", f3},
		{"f4@fork-win", "add", f4}, // 反超 → REORG_ACCEPT
		{"badPrev", "validate", badPrev},
		{"wrongBits", "validate", wrongBits},
		{"oldTimestamp", "validate", oldTs},
		{"tamperedMerkle", "validate", tampered},
		{"good5@post-reorg", "add", good5},
	}
}

// runScenario 在指定观测状态下、全新存储目录中重放全部操作。
// 返回逐步结果与事件流原文（off 态 buf 仅作零输出哨兵，返回空串）。
func runScenario(t *testing.T, ops []obsOp, obsOn bool) ([]obsStep, string) {
	t.Helper()
	var buf bytes.Buffer
	obs.Disable() // 幂等停掉既有 writerLoop（可能持有 stderr 快照）
	if obsOn {
		obs.SetOutputForTest(&buf) // 必须先于 EnableForTest
		obs.EnableForTest()
	} else {
		obs.SetOutputForTest(&buf)
	}

	store, err := storage.OpenFileBlockStore(t.TempDir())
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	// 空库自动创建确定性创世（固定时间戳），两态的 genesis 逐字节一致。
	bc, err := blockchain.NewBlockchainFromStore(store)
	if err != nil {
		t.Fatalf("从存储加载链失败: %v", err)
	}
	genesisHash := mustTip(t, bc).Header.Hash()
	if genesisHash != mustHeight(t, bc, 0).Header.Hash() {
		t.Fatal("空库加载后链尾应为创世")
	}

	steps := make([]obsStep, 0, len(ops))
	for _, op := range ops {
		var err error
		if op.action == "add" {
			err = bc.AddBlock(op.blk)
		} else {
			err = bc.ValidateBlock(op.blk)
		}
		errStr := "nil"
		if err != nil {
			errStr = err.Error()
		}
		tip := mustTip(t, bc)
		blkHash := op.blk.Header.Hash()
		tipHash := tip.Header.Hash()
		steps = append(steps, obsStep{
			label:  op.label,
			action: op.action,
			hash8:  fmt.Sprintf("%x", blkHash[:4]),
			errStr: errStr,
			height: bc.Height(),
			tip8:   fmt.Sprintf("%x", tipHash[:4]),
		})
	}

	// flush 屏障：Disable 等待 writerLoop 排空通道并退出（happens-before via doneCh），
	// 保证 buf 内容完整可见；同时把状态归位为 off。
	obs.Disable()

	// 活跃性/静默哨兵：on 态必须产生事件；off 态必须零输出。
	if obsOn && buf.Len() == 0 {
		t.Fatal("obs on 态零事件输出 —— 插桩未生效，对照失效")
	}
	if !obsOn && buf.Len() != 0 {
		t.Fatalf("obs off 态不应产生任何输出，实际前 200 字节: %.200s", buf.String())
	}
	return steps, buf.String()
}

// assertEventPresent 校验 on 态事件流包含关键事件名（活跃性细查）。
func assertEventPresent(t *testing.T, events string, names ...string) {
	t.Helper()
	for _, n := range names {
		if !strings.Contains(events, n) {
			t.Fatalf("on 态事件流缺少 %s（插桩活跃性不足）", n)
		}
	}
}

// TestObsNonInterferenceDeterministic §10 Test D 主入口。
func TestObsNonInterferenceDeterministic(t *testing.T) {
	t.Cleanup(func() { obs.Disable() }) // 收尾还原，不污染同进程后续测试

	bits := uint32(pow.MaxTargetBits)
	store0, err := storage.OpenFileBlockStore(t.TempDir())
	if err != nil {
		t.Fatalf("打开探针存储失败: %v", err)
	}
	bc0, err := blockchain.NewBlockchainFromStore(store0)
	if err != nil {
		t.Fatalf("探针链加载失败: %v", err)
	}
	genesisHash := mustTip(t, bc0).Header.Hash()
	_ = store0.Close()

	ops := buildScenario(t, genesisHash, bits)

	onSteps, onEvents := runScenario(t, ops, true)
	assertEventPresent(t, onEvents, "REORG_REJECT", "REORG_ACCEPT", "FORK_VALIDATION_START", "FORK_VALIDATION_END")
	offSteps, _ := runScenario(t, ops, false)

	if len(onSteps) != len(offSteps) {
		t.Fatalf("步骤数不一致: on=%d off=%d", len(onSteps), len(offSteps))
	}
	for i := range onSteps {
		if !onSteps[i].equal(offSteps[i]) {
			t.Fatalf("第 %d 步（%s / %s）行为漂移 —— 违反 OBSERVE≠CHANGE:\n  on : hash=%s err=%q height=%d tip=%s\n  off: hash=%s err=%q height=%d tip=%s",
				i, onSteps[i].label, onSteps[i].action,
				onSteps[i].hash8, onSteps[i].errStr, onSteps[i].height, onSteps[i].tip8,
				offSteps[i].hash8, offSteps[i].errStr, offSteps[i].height, offSteps[i].tip8)
		}
	}

	for _, s := range onSteps {
		t.Logf("%-16s %-8s h=%d err=%s", s.label, s.action, s.height, s.errStr)
	}
}
