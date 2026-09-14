// settip.go 实现 REORG-1B 的「树级 tip 基础设施」。
//
// ─────────────────────────────────────────────────────────────────────────────
// REORG-1B 契约边界（必须与 REORG-1C 严格区分，否则触发 §16 STOP #5/#7）：
//
//   - 本文件的 SetTip 是【树级 / 索引级】原语：仅移动 BlockTree.bestTip / bestTipWork
//     两个内存指针，**绝不**触碰 UTXO / mempool / 持久化文件（§9 契约）。
//   - REORG-1C（R3）的 SetTip / ConnectBlock / DisconnectBlock 才是【共识级】链切换：
//     包含 UTXO disconnect/connect、mempool re-add、存储原子提交——属后续阶段，
//     在 internal/blockchain 实现，且必须等 UTXO undo（REORG-1D）/存储 delete（REORG-1E）
//     就绪后才能安全接入。
//   - 因此 1B 的 SetTip 是 1C 的「数据前提」：1C 在完成 UTXO rollback + 新分支 connect
//     后，会调用本 SetTip 完成【索引层】指针切换；1B 本身不触发任何链替换。
//
// ─────────────────────────────────────────────────────────────────────────────
// fork-choice 决策（§E 契约，纯只读比较，不切换）：
//
//	if   candidate.CumulativeWork > active.CumulativeWork:  candidate wins
//	elif candidate.CumulativeWork < active.CumulativeWork:  active wins
//	else:  deterministic tie-break（tip hash 大端较大者胜，§E.2）
//
// 铁律：network arrival order 绝不决定 consensus chain。本比较只依赖本地验证后的
// CumulativeWork（*big.Int）+ 确定性 tie-break。
//
// ─────────────────────────────────────────────────────────────────────────────
// MaxReorgDepth（BG-3，已定稿）：本 SetTip **永不**因深度拒绝更高 work 链——
// 「告警 + 限速，永不拒绝」。深度策略属 REORG-1I，此处不实现、不检查。
package blocktree

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
)

// REORG-1B SetTip 相关可判定错误。
var (
	// ErrTipNotInTree SetTip/CompareWork 的目标节点不在树中。
	ErrTipNotInTree = errors.New("blocktree: tip node not present in tree")
	// ErrNilTip 传入了 nil 节点。
	ErrNilTip = errors.New("blocktree: tip node is nil")
	// ErrNilCandidate ShouldReorg/CompareWork 的候选节点为 nil。
	ErrNilCandidate = errors.New("blocktree: candidate node is nil")
	// ErrNoActiveTip 尚未设置 bestTip（fork-choice 比较无活动基准）。
	ErrNoActiveTip = errors.New("blocktree: no active tip set")
)

// SetTip 将树级活动链尾指针切换到 node。
//
// 契约（REORG-1B §9）：
//
//	SetTip 负责：
//	  • 校验 node 非空且存在于本树（LookupNode）；
//	  • 设置 bestTip = node、bestTipWork = node.CumulativeWork（缓存，O(1) fork-choice）。
//
//	SetTip 明确不负责（属后续阶段）：
//	  • UTXO 回滚 / undo（REORG-1D / R4，FC-008）；
//	  • mempool re-add（REORG-1F / R6，FC-006）；
//	  • 存储层 Delete/Truncate/原子提交（REORG-1E / R5，FC-007）；
//	  • common ancestor 计算 / 分支 disconnect+connect（REORG-1C / R3）；
//	  • MaxReorgDepth 深度策略（REORG-1I / R9，BG-3）；
//	  • 通知矿工/RPC tip 变更（REORG-1C 接入 blockchain.notifyTipChanged）。
//
// 即：本 SetTip 只移动索引指针，是「未通电的开关」。生产 AddBlock 路径不调用它
// （见 REORG OFF proof），因此不会因本方法的存在而自动发生链替换。
//
// 返回 nil 表示指针已切换；返回错误时 bestTip/bestTipWork 保持不变（原子语义）。
func (t *BlockTree) SetTip(node *BlockNode) error {
	if node == nil {
		return ErrNilTip
	}
	// 校验节点确属本树：防止外部传入游离 *BlockNode 造成索引与树脱节。
	if t.LookupNode(node.Hash) != node {
		return fmt.Errorf("%w: hash %x…", ErrTipNotInTree, node.Hash[:4])
	}
	t.bestTip = node
	t.bestTipWork = new(big.Int).Set(node.CumulativeWork)
	return nil
}

// BestTip 返回当前树级活动链尾节点；尚未 SetTip 时返回 nil。
func (t *BlockTree) BestTip() *BlockNode {
	return t.bestTip
}

// BestTipWork 返回活动链尾的累积工作量缓存；尚未 SetTip 时返回 nil。
// 该值等于 BestTip().CumulativeWork，仅在 SetTip 时刷新，O(1) 读取。
func (t *BlockTree) BestTipWork() *big.Int {
	return t.bestTipWork
}

// ActiveHeight 返回活动链尾高度；尚未 SetTip 时返回 -1。
func (t *BlockTree) ActiveHeight() int {
	if t.bestTip == nil {
		return -1
	}
	return t.bestTip.Height
}

// ActiveChain 返回从活动链尾到根（含两端）的节点序列；尚未 SetTip 时返回 nil。
// 顺序为 [bestTip, parent, ..., genesis]。
func (t *BlockTree) ActiveChain() []*BlockNode {
	if t.bestTip == nil {
		return nil
	}
	return t.bestTip.PathToRoot()
}

// IsActiveChain 判断 node 是否位于当前活动链上（含链尾与根）。
// 尚未 SetTip 时返回 false（无活动链则无成员）。
// 实现沿 bestTip.PathToRoot() 线性扫描；链长 = height+1，对小链与中等链均足够，
// 不引入额外 map（避免 BT-1 类 map 迭代 nondeterminism）。
func (t *BlockTree) IsActiveChain(node *BlockNode) bool {
	if node == nil || t.bestTip == nil {
		return false
	}
	for _, a := range t.bestTip.PathToRoot() {
		if a == node {
			return true
		}
	}
	return false
}

// CompareWork 返回候选节点相对活动链尾的累积工作量比较结果（§E.1 纯只读比较，不切换）。
//
// 返回值语义（与 math/big.Int.Cmp 一致）：
//
//	>0 : candidate.CumulativeWork > active（候选胜）
//	<0 : candidate.CumulativeWork < active（活动胜）
//	 0 : 相等，需走 tie-break（见 ShouldReorg）
//
// 错误：bestTip 未设置 → ErrNoActiveTip；candidate nil → ErrNilCandidate；
// candidate 不在本树 → ErrTipNotInTree（防止比较游离节点）。
func (t *BlockTree) CompareWork(candidate *BlockNode) (int, error) {
	if t.bestTip == nil {
		return 0, ErrNoActiveTip
	}
	if candidate == nil {
		return 0, ErrNilCandidate
	}
	if t.LookupNode(candidate.Hash) != candidate {
		return 0, fmt.Errorf("%w: hash %x…", ErrTipNotInTree, candidate.Hash[:4])
	}
	return candidate.CumulativeWork.Cmp(t.bestTipWork), nil
}

// ShouldReorg 应用 §E fork-choice 决策，返回是否应当切换到 candidate 及原因。
// 本方法是【纯只读决策】：不切换 bestTip、不触碰任何外部状态。
// 实际切换由调用方（REORG-1C 共识层）在完成 UTXO rollback+connect 后调 SetTip。
//
// 决策规则（冻结于 §E.1 + §E.2）：
//
//	CompareWork > 0 → candidate wins（work 反超）
//	CompareWork < 0 → active wins（work 不足）
//	CompareWork == 0 → tie-break：tip hash 大端较大者胜（确定性，全网可复算）
//
// 返回：
//
//	(true,  reason)  : candidate 应当成为新活动链尾
//	(false, reason)  : 活动链尾保持不变
//	(false, err.Error()) + err : 比较前置失败（无活动 tip / 候选不在树等）
func (t *BlockTree) ShouldReorg(candidate *BlockNode) (bool, string, error) {
	cmp, err := t.CompareWork(candidate)
	if err != nil {
		return false, "", err
	}
	switch {
	case cmp > 0:
		return true, "candidate cumulative work exceeds active", nil
	case cmp < 0:
		return false, "candidate cumulative work below active", nil
	default:
		// tie-break：tip hash 大端较大者胜（§E.2 推荐）。
		// bytes.Compare 对 [32]byte 做字典序比较 = 大端整数比较。
		if tieBreakWinner(t.bestTip, candidate) == candidate {
			return true, "work tie; candidate wins by larger tip hash (deterministic)", nil
		}
		return false, "work tie; active wins by larger tip hash (deterministic)", nil
	}
}

// tieBreakWinner 返回两节点中按「tip hash 大端较大者胜」规则获胜的节点。
// 二者 hash 相同时（同一节点）返回 active（保守，不切换）。
func tieBreakWinner(active, candidate *BlockNode) *BlockNode {
	switch bytes.Compare(candidate.Hash[:], active.Hash[:]) {
	case 1:
		return candidate
	default: // 0（同节点）或 -1（active 更大）
		return active
	}
}

// ResetTip 清空树级 tip 状态（bestTip/bestTipWork 置 nil）。
// 仅供测试与重建场景使用；生产路径不调用。
func (t *BlockTree) ResetTip() {
	t.bestTip = nil
	t.bestTipWork = nil
}
