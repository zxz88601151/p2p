package blockchain

// PHASE P2PCHAIN — P1 REORG SILENT-STATE-DIVERGENCE REMEDIATION
//
// 本文件只做一件事：用**确定性 fault injection** 证明 reorg 提交路径在
// 「必需 path 区块无法获取」时，会 **fail before commit** —— 返回错误，
// 且 bc.blocks / bc.utxo / bc.hashIndex / bc.canonicalSet / tree tip /
// lastReorgResult 全部保持原样（不存在 partial state commit）。
//
// 注入手段：真实 store 被一层薄包装，对指定哈希在 GetBlockByHash 上必然失败；
// 同时把该哈希从内存 hashIndex 中移除，使 blockAtHash 必然回退到 storage。
// 全程无 sleep / 无随机 / 无时间竞态 / 无并发 / 无网络依赖。

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
)

// p1InjectErrMarker 是被注入故障的唯一可识别标记。
const p1InjectErrMarker = "注入的存储读取失败"

// p1FailGetStore 包装真实 *storage.FileBlockStore：除 failHash 外全部委派。
//
// 嵌入具体类型（而非 storage.BlockStore 接口）是必要的——只有具体类型才能把
// reorgStore 的 8 个附加方法一并提升，使 bc.store.(reorgStore) 断言仍然成立。
type p1FailGetStore struct {
	*storage.FileBlockStore
	failHash [32]byte
}

func (s *p1FailGetStore) GetBlockByHash(hash [32]byte) (*block.Block, error) {
	if hash == s.failHash {
		return nil, fmt.Errorf("%s（%x…）: %w", p1InjectErrMarker, hash[:4], storage.ErrNotFound)
	}
	return s.FileBlockStore.GetBlockByHash(hash)
}

// p1ReorgState 是 reorg 提交面的可比快照（全部为确定性表示）。
type p1ReorgState struct {
	height         int
	chainHashes    []string
	utxoSig        string
	bestTip        string
	hashIndexKeys  []string
	canonicalKeys  []string
	hasReorgResult bool
}

// p1SnapshotState 读取提交面快照。
//
// 刻意 **不** 在内部加 bc.mu：本测试为单 goroutine，且 f1n1UTXOSig 内部会自行取
// RLock —— 嵌套 RLock 在存在等待写锁时可能死锁，故这里直接读字段。
func p1SnapshotState(bc *Blockchain) p1ReorgState {
	s := p1ReorgState{
		height:         len(bc.blocks) - 1,
		utxoSig:        f1n1UTXOSig(bc),
		hasReorgResult: bc.lastReorgResult != nil,
	}
	for _, b := range bc.blocks {
		s.chainHashes = append(s.chainHashes, b.Header.HashHex())
	}
	for h := range bc.hashIndex {
		s.hashIndexKeys = append(s.hashIndexKeys, hex.EncodeToString(h[:]))
	}
	for h := range bc.canonicalSet {
		s.canonicalKeys = append(s.canonicalKeys, hex.EncodeToString(h[:]))
	}
	sort.Strings(s.hashIndexKeys)
	sort.Strings(s.canonicalKeys)
	if tip := bc.tree.BestTip(); tip != nil {
		s.bestTip = hex.EncodeToString(tip.Hash[:])
	}
	return s
}

// p1AssertUnchanged 断言注入失败后提交面逐项未变。
func p1AssertUnchanged(t *testing.T, before, after p1ReorgState) {
	t.Helper()
	if after.height != before.height {
		t.Errorf("高度被修改: %d → %d", before.height, after.height)
	}
	if strings.Join(after.chainHashes, ",") != strings.Join(before.chainHashes, ",") {
		t.Errorf("bc.blocks 被修改:\n  before=%v\n  after =%v", before.chainHashes, after.chainHashes)
	}
	if after.utxoSig != before.utxoSig {
		t.Errorf("bc.utxo 被修改: %s → %s", before.utxoSig, after.utxoSig)
	}
	if after.bestTip != before.bestTip {
		t.Errorf("tree.BestTip 被修改: %s → %s", before.bestTip, after.bestTip)
	}
	if strings.Join(after.hashIndexKeys, ",") != strings.Join(before.hashIndexKeys, ",") {
		t.Errorf("bc.hashIndex 被修改:\n  before=%v\n  after =%v", before.hashIndexKeys, after.hashIndexKeys)
	}
	if strings.Join(after.canonicalKeys, ",") != strings.Join(before.canonicalKeys, ",") {
		t.Errorf("bc.canonicalSet 被修改:\n  before=%v\n  after =%v", before.canonicalKeys, after.canonicalKeys)
	}
	// 服务层通过 AddBlockWithResult 消费 lastReorgResult（供 mempool 复活）；
	// 失败路径不得留下任何 ReorgResult。
	if after.hasReorgResult {
		t.Error("失败路径留下了 lastReorgResult（服务层会消费到 partial ReorgResult）")
	}
}

// TestReorg_FailBeforeCommitOnMissingPathBlock 是 P1 的确定性回归测试。
//
// 场景：canonical = genesis → h1 → h2；竞争者 genesis → a1 → a2 → a3（更长）。
// 注入：让 disconnectPath 成员 h1 在 blockAtHash 上必然失败。
//   - 修复前：该读取发生在 canonical 状态已提交之后，错误被 `_` 丢弃、块被静默跳过
//     ⇒ executeReorg 返回 nil，而 bc.blocks 已被截断、utxo 仍完整 ⇒ 三方静默分歧。
//   - 修复后：解析与校验全部前移到提交之前 ⇒ 返回错误且提交面逐项未变。
func TestReorg_FailBeforeCommitOnMissingPathBlock(t *testing.T) {
	f := newReorgFixture(t)
	_ = f.mineNext(pow.MaxTargetBits) // h1
	_ = f.mineNext(pow.MaxTargetBits) // h2
	if len(f.chain.blocks) != 3 {
		t.Fatalf("前置链高度异常: len(blocks)=%d, want 3", len(f.chain.blocks))
	}
	h1 := f.chain.blocks[1]

	// 竞争者：genesis → a1 → a2 → a3（更长 ⇒ 必然进入 reorg 路径）。
	a1 := f.mineFork(f.genesis.Header.Hash(), 1, pow.MaxTargetBits)
	a2 := f.mineFork(a1.Header.Hash(), 2, pow.MaxTargetBits)
	a3 := f.mineFork(a2.Header.Hash(), 3, pow.MaxTargetBits)
	f.saveDetached(a1)
	f.saveDetached(a2)
	f.saveDetached(a3)

	nodeA3 := f.chain.tree.LookupNode(a3.Header.Hash())
	if nodeA3 == nil {
		_, _ = f.chain.tree.AddBlock(a1.Header.Hash(), f.genesis.Header.Hash(), 1, pow.MaxTargetBits, a1.Header.Timestamp)
		_, _ = f.chain.tree.AddBlock(a2.Header.Hash(), a1.Header.Hash(), 2, pow.MaxTargetBits, a2.Header.Timestamp)
		nodeA3, _ = f.chain.tree.AddBlock(a3.Header.Hash(), a2.Header.Hash(), 3, pow.MaxTargetBits, a3.Header.Timestamp)
	}
	if nodeA3 == nil {
		t.Fatal("fork tip 未能进入 blocktree")
	}

	// 注入目标必须是 disconnectPath 成员（oldTip h2 → h1 → ancestor genesis）：
	// genesis 同时位于 ancestorPath 且被 utxoAtNode 读取，注入它会先命中那条路径。
	injHash := h1.Header.Hash()
	delete(f.chain.hashIndex, injHash) // 强制 blockAtHash 回退到 storage
	f.chain.store = &p1FailGetStore{FileBlockStore: f.store, failHash: injHash}

	// 注入后、调用前取基线（此后 executeReorg 不得再改变任何一项）。
	before := p1SnapshotState(f.chain)

	err := f.chain.executeReorg(nodeA3, true)

	if err == nil {
		t.Fatal("executeReorg 在必需 path 区块缺失时必须返回错误；返回 nil 即为静默状态分歧")
	}
	if !strings.Contains(err.Error(), p1InjectErrMarker) {
		t.Fatalf("错误未包含注入原因（%v）: %v", p1InjectErrMarker, err)
	}

	p1AssertUnchanged(t, before, p1SnapshotState(f.chain))

	// 复核：canonical 链尾仍是旧链 h2，且旧链 h1 仍在 bc.blocks 中（未被截断）。
	tip, tipErr := f.chain.Tip()
	if tipErr != nil {
		t.Fatalf("读取链尾失败: %v", tipErr)
	}
	if got, want := tip.Header.HashHex(), f.chain.blocks[2].Header.HashHex(); got != want {
		t.Fatalf("链尾被修改: got=%s want=%s（失败必须不改动 canonical 链）", got, want)
	}
}
