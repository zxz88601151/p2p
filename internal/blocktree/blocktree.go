// Package blocktree 实现 REORG-1A 阶段的最小、确定性、可测试的
// BlockNode / Block Tree 内存基础设施。
//
// 本阶段范围（REORG-1A 契约）：
//   - 仅建立区块在内存中的树形索引（Hash → BlockNode）；
//   - 计算并维护「严格工作量」Work(bits)=2^bits 与累积工作量 CumulativeWork（*big.Int）；
//   - 维护双向父子链接、去重、高度一致、父存在、根处理、祖先遍历；
//   - 提供 CheckInvariant 验证整树结构不变式。
//
// 本阶段「不实现」（属后续阶段）：
//   - 共识级链切换执行（ConnectBlock/DisconnectBlock/UTXO rollback）—— REORG-1C / R3；
//   - 持久化（落盘）—— REORG-1E / R5；
//   - BlockUndo / atomic commit / crash recovery；
//   - common ancestor 计算；
//   - MaxReorgDepth 与 P2P reorg 同步；
//   - 任何共识参数、难度或 P2P 行为变更。
//
// REORG-1B 在本包内新增「树级 tip 基础设施」（见 settip.go）：
//   - BlockTree.bestTip / bestTipWork（R1 契约 §C.2-q4：显式 active tip 指针）；
//   - SetTip 仅移动该指针，**绝不触碰 UTXO / mempool / 持久化**（§9 契约）；
//   - 这是 REORG-1C 共识级 SetTip 的数据前提，本身不触发链切换。
//
// 该包是纯内存结构，不依赖任何既有链模块（仅使用标准库），
// 因此天然不接入 cmd/node，reorg 执行保持 OFF。
package blocktree

import (
	"errors"
	"fmt"
	"math/big"
)

// Status 表示节点的「本地校验态」，而非共识态。
// 注意（REORG-1A 契约 §4.3）：fork-choice 不得读取落盘 Status；
// 本字段仅用于本地索引层记录校验结果，不参与任何共识决策。
type Status uint8

const (
	// StatusUnknown 初始/未校验态。
	StatusUnknown Status = iota
	// StatusValid 本地校验通过（仅本地视图，非全网共识确认）。
	StatusValid
	// StatusInvalid 本地校验失败。
	StatusInvalid
)

// 错误集合：AddBlock 与 CheckInvariant 返回的可判定错误。
var (
	// ErrDuplicateHash 哈希已存在于树中。
	ErrDuplicateHash = errors.New("blocktree: duplicate block hash")
	// ErrMissingParent 指定了 parentHash 但父节点不存在（无效父链接）。
	ErrMissingParent = errors.New("blocktree: parent block not found")
	// ErrParentHashMismatch 节点的 ParentHash 与其 Parent.Hash 不一致。
	ErrParentHashMismatch = errors.New("blocktree: parent hash mismatch")
	// ErrInvalidHeight 高度不是父高度 +1（非根）。
	ErrInvalidHeight = errors.New("blocktree: height is not parent.height+1")
	// ErrInvalidRoot 高度为 0 却携带了父引用。
	ErrInvalidRoot = errors.New("blocktree: height 0 must have no parent")
	// ErrSelfParent 区块指向自身为父（hash == parentHash 且 height>0）。
	ErrSelfParent = errors.New("blocktree: block cannot be its own parent")
	// ErrInvalidBits bits 取值非法（0 或 >256 会导致目标溢出/无工作量）。
	ErrInvalidBits = errors.New("blocktree: bits out of range [1,256]")
	// ErrNegativeHeight 高度为负数。
	ErrNegativeHeight = errors.New("blocktree: negative height")
)

// WorkOfBits 返回 bits 对应的「严格工作量」：Work(bits) = 2^bits。
//
// 本链 bits 是「前导零位数」（exponent）：target = 2^(256-bits)（见 pow.BitsToTarget）。
// 期望哈希数 = 2^256 / 2^(256-bits) = 2^bits，精确无舍入，
// 因此 Work(bits)=2^bits 是严格真实工作量。无论难度是否浮动，
// 分支间均可按 Σ 2^bits_i 用 *big.Int 全序精确比较（见 PHASE POW-WORK 审计）。
func WorkOfBits(bits uint32) *big.Int {
	w := big.NewInt(1)
	w.Lsh(w, uint(bits))
	return w
}

// BlockNode 是区块在内存树中的节点，仅承载可被确定性重建的字段。
//
// 字段分类（REORG-1A 契约）：
//   - 共识派生：Hash / ParentHash / Height / Bits / Timestamp / Work / CumulativeWork
//     （完全由区块头推导，不依赖任何外部可变状态）；
//   - 本地索引：Parent（父指针）/ Children（子指针切片）；
//   - 本地校验态：Status（非共识态，fork-choice 不得依赖）。
type BlockNode struct {
	// —— 共识派生字段（确定性可重建）——
	Hash           [32]byte // 区块头双 SHA-256 哈希
	ParentHash     [32]byte // 父区块哈希（创世为零值）
	Height         int      // 区块高度（创世 = 0）
	Bits           uint32   // 难度目标前导零位数
	Timestamp      int64    // 出块时间戳（Unix 秒）
	Work           *big.Int // = WorkOfBits(Bits)，严格工作量
	CumulativeWork *big.Int // = parent.CumulativeWork + Work（根为 2^Bits）

	// —— 本地索引（内存指针，不落盘）——
	Parent   *BlockNode   // 父节点指针（根为 nil）
	Children []*BlockNode // 子节点指针切片

	// —— 本地校验态（非共识态）——
	Status Status
}

// NewBlockNode 构造一个节点并完成 Work / CumulativeWork 计算。
// 当 parent 为 nil（根/创世）时，CumulativeWork = Work。
// 调用方负责保证入参的一致性（AddBlock 已做校验）。
func NewBlockNode(hash, parentHash [32]byte, height int, bits uint32, timestamp int64, parent *BlockNode) *BlockNode {
	work := WorkOfBits(bits)
	cw := new(big.Int).Set(work)
	if parent != nil {
		cw.Add(cw, parent.CumulativeWork)
	}
	return &BlockNode{
		Hash:           hash,
		ParentHash:     parentHash,
		Height:         height,
		Bits:           bits,
		Timestamp:      timestamp,
		Work:           work,
		CumulativeWork: cw,
		Parent:         parent,
		Children:       nil,
		Status:         StatusUnknown,
	}
}

// PathToRoot 返回从本节点沿 Parent 指针到根（含本节点与根）的节点序列。
func (n *BlockNode) PathToRoot() []*BlockNode {
	var path []*BlockNode
	cur := n
	for cur != nil {
		path = append(path, cur)
		cur = cur.Parent
	}
	return path
}

// AncestorAtHeight 返回本节点在指定高度 h 的祖先（含自身）；
// h 越界（<0 或 >Height）返回 nil。common ancestor 计算属后续阶段，此处仅提供单链祖先。
func (n *BlockNode) AncestorAtHeight(h int) *BlockNode {
	if h < 0 || h > n.Height {
		return nil
	}
	for _, a := range n.PathToRoot() {
		if a.Height == h {
			return a
		}
	}
	return nil
}

// IsAncestorOf 判断 n 是否为 other 的祖先（含自身）。
func (n *BlockNode) IsAncestorOf(other *BlockNode) bool {
	if other == nil {
		return false
	}
	for _, a := range other.PathToRoot() {
		if a == n {
			return true
		}
	}
	return false
}

// BlockTree 是 BlockNode 的内存索引，提供 O(1) 查找、双向链接与不变式检查。
// REORG-1B 在其上新增树级 tip 状态（bestTip/bestTipWork）与 SetTip 原语（见 settip.go）。
// 本包不持久化、不实现共识级链切换（Connect/Disconnect/UTXO rollback 属 REORG-1C）。
type BlockTree struct {
	nodes map[[32]byte]*BlockNode

	// —— REORG-1B 树级 tip 状态（R1 契约 §C.2-q4：显式 active tip 指针）——
	// bestTip 是当前「活动链尾」的索引指针；bestTipWork 是其 CumulativeWork 的缓存（O(1) fork-choice）。
	// 二者仅由 SetTip 维护；fork-choice 决策走 ShouldReorg（只读比较，不切换）。
	// 注意：本字段是「索引层」状态，不等于「共识层」活动链（共识层活动链仍由 blockchain.blocks[len-1] 表达，
	// 直到 REORG-1C 把 SetTip 接入 blockchain 并完成 UTXO disconnect/connect）。
	bestTip     *BlockNode
	bestTipWork *big.Int
}

// NewBlockTree 创建一棵空树。
func NewBlockTree() *BlockTree {
	return &BlockTree{nodes: make(map[[32]byte]*BlockNode)}
}

// Len 返回树中节点总数。
func (t *BlockTree) Len() int { return len(t.nodes) }

// LookupNode 按哈希返回节点，不存在返回 nil。
func (t *BlockTree) LookupNode(hash [32]byte) *BlockNode {
	return t.nodes[hash]
}

// Has 判断哈希是否已存在。
func (t *BlockTree) Has(hash [32]byte) bool {
	_, ok := t.nodes[hash]
	return ok
}

// Genesis 返回树中高度为 0 的根节点；若存在多个根或多树，返回任意一个根（本阶段单链/单树假设）。
func (t *BlockTree) Genesis() *BlockNode {
	for _, n := range t.nodes {
		if n.Parent == nil {
			return n
		}
	}
	return nil
}

// AddBlock 将一个区块头插入树，建立双向链接并补全 CumulativeWork。
// 这是 REORG-1A 唯一的「写」入口，所有结构性校验在此完成。
//
// 校验（负例来源）：
//   - height<0 → ErrNegativeHeight
//   - bits 不在 [1,256] → ErrInvalidBits（防止移位溢出 / 零工作量）
//   - 重复哈希 → ErrDuplicateHash
//   - height==0 但携带非空 parentHash → ErrInvalidRoot
//   - height>0 但 parentHash 不在树中 → ErrMissingParent（无效父链接）
//   - hash == parentHash（且 height>0） → ErrSelfParent
//   - height != parent.Height+1 → ErrInvalidHeight（父高度不匹配）
func (t *BlockTree) AddBlock(hash, parentHash [32]byte, height int, bits uint32, timestamp int64) (*BlockNode, error) {
	if height < 0 {
		return nil, ErrNegativeHeight
	}
	if bits == 0 || bits > 256 {
		return nil, ErrInvalidBits
	}
	if _, dup := t.nodes[hash]; dup {
		return nil, ErrDuplicateHash
	}

	if height == 0 {
		if !isZeroHash(parentHash) {
			return nil, ErrInvalidRoot
		}
		node := NewBlockNode(hash, parentHash, height, bits, timestamp, nil)
		t.nodes[hash] = node
		return node, nil
	}

	// 自引用（hash == parentHash）在合法链中不可能出现：其 parent 永远无法先存在，
	// 故必须在「父存在」与「重复」检查之前判定，否则会被 ErrMissingParent 掩盖。
	if hash == parentHash {
		return nil, ErrSelfParent
	}
	parent := t.nodes[parentHash]
	if parent == nil {
		return nil, ErrMissingParent
	}
	if height != parent.Height+1 {
		return nil, ErrInvalidHeight
	}

	node := NewBlockNode(hash, parentHash, height, bits, timestamp, parent)
	parent.Children = append(parent.Children, node) // 双向链接
	t.nodes[hash] = node
	return node, nil
}

// InvariantError 描述 CheckInvariant 发现的一条具体违规。
type InvariantError struct {
	Node [32]byte
	Msg  string
}

func (e *InvariantError) Error() string {
	return fmt.Sprintf("blocktree: invariant violation at %x…: %s", e.Node[:4], e.Msg)
}

// CheckInvariant 验证整棵树的确定性结构不变式（REORG-1A 结构契约）：
//  1. 每个 height>0 的节点必须有非 nil 的 Parent，且 ParentHash == Parent.Hash；
//  2. 每个 height>0 的节点 Height == Parent.Height + 1；
//  3. 每个节点的 Work == WorkOfBits(Bits)；
//  4. 每个 height>0 的节点 CumulativeWork == Parent.CumulativeWork + Work；
//     根节点 CumulativeWork == Work；
//  5. 双向链接一致：node ∈ Parent.Children；
//  6. 无环（沿 Parent 上行步数不超过节点总数）。
//
// 返回所有违规（可为多条），无违规返回 nil。
func (t *BlockTree) CheckInvariant() []error {
	var errs []error
	total := len(t.nodes)

	for hash, n := range t.nodes {
		// 1
		if n.Height > 0 {
			if n.Parent == nil {
				errs = append(errs, &InvariantError{hash, "height>0 but Parent is nil"})
			} else {
				if n.ParentHash != n.Parent.Hash {
					errs = append(errs, &InvariantError{hash, "ParentHash != Parent.Hash"})
				}
				if n.Height != n.Parent.Height+1 {
					errs = append(errs, &InvariantError{hash, "Height != Parent.Height+1"})
				}
				// 5 双向链接
				linked := false
				for _, c := range n.Parent.Children {
					if c == n {
						linked = true
						break
					}
				}
				if !linked {
					errs = append(errs, &InvariantError{hash, "node not present in Parent.Children"})
				}
			}
		}

		// 3
		expectedWork := WorkOfBits(n.Bits)
		if n.Work == nil || n.Work.Cmp(expectedWork) != 0 {
			errs = append(errs, &InvariantError{hash, "Work != WorkOfBits(Bits)"})
		}

		// 4
		var expectedCW *big.Int
		if n.Parent == nil {
			expectedCW = new(big.Int).Set(n.Work)
		} else {
			expectedCW = new(big.Int).Add(n.Parent.CumulativeWork, n.Work)
		}
		if n.CumulativeWork == nil || n.CumulativeWork.Cmp(expectedCW) != 0 {
			errs = append(errs, &InvariantError{hash, "CumulativeWork inconsistent"})
		}

		// 6 无环（沿 Parent 上行不超过 total 步）
		if hasCycle(n, total) {
			errs = append(errs, &InvariantError{hash, "cycle detected in Parent chain"})
		}
	}
	return errs
}

// hasCycle 沿 Parent 上行，若步数超过 total 说明存在环。
func hasCycle(n *BlockNode, total int) bool {
	steps := 0
	cur := n
	for cur != nil {
		steps++
		if steps > total {
			return true
		}
		cur = cur.Parent
	}
	return false
}

// isZeroHash 判断 32 字节哈希是否全零（创世父哈希约定）。
func isZeroHash(h [32]byte) bool {
	for _, b := range h {
		if b != 0 {
			return false
		}
	}
	return true
}
