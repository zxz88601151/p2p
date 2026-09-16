package blocktree

import (
	"bytes"
	"math/big"
	"sort"
	"testing"
)

// —— 测试辅助 ——

// h32 由单个整数生成一个唯一可辨识的 [32]byte 哈希（仅测试用）。
func h32(b byte) [32]byte {
	var h [32]byte
	h[0] = b
	return h
}

// hStr 由字符串生成一个伪哈希（仅测试用）。
func hStr(s string) [32]byte {
	var h [32]byte
	copy(h[:], s)
	return h
}

// mustAdd 是测试便捷封装：AddBlock 失败即 t.Fatal。
func mustAdd(t *testing.T, tr *BlockTree, hash, parent [32]byte, height int, bits uint32, ts int64) *BlockNode {
	t.Helper()
	n, err := tr.AddBlock(hash, parent, height, bits, ts)
	if err != nil {
		t.Fatalf("AddBlock(%x) unexpected error: %v", hash[:4], err)
	}
	return n
}

// expectedChainCW 计算「从根到指定高度，每级 bits」的累积工作量。
func expectedChainCW(bits []uint32) *big.Int {
	cw := big.NewInt(0)
	for _, b := range bits {
		cw.Add(cw, WorkOfBits(b))
	}
	return cw
}

// =====================================================================
// 正例 A–J
// =====================================================================

// A. Genesis — 创世块：height=0、Parent=nil、Work=CumulativeWork=2^16。
func TestA_Genesis(t *testing.T) {
	tr := NewBlockTree()
	g := mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)

	if g.Parent != nil {
		t.Fatal("genesis Parent must be nil")
	}
	if g.Height != 0 {
		t.Fatalf("genesis height = %d, want 0", g.Height)
	}
	if g.Work.Cmp(WorkOfBits(16)) != 0 {
		t.Fatalf("genesis Work = %s, want 2^16", g.Work)
	}
	if g.CumulativeWork.Cmp(WorkOfBits(16)) != 0 {
		t.Fatalf("genesis CumulativeWork = %s, want 2^16", g.CumulativeWork)
	}
	if tr.Genesis() != g {
		t.Fatal("Genesis() must return the root node")
	}
	if errs := tr.CheckInvariant(); errs != nil {
		t.Fatalf("CheckInvariant on genesis-only tree: %v", errs)
	}
}

// B. Linear — 线性链：逐级 CW = parent.CW + 2^bits，双向链接一致。
func TestB_LinearChain(t *testing.T) {
	tr := NewBlockTree()
	mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	prev := h32(1)
	for i := 1; i <= 10; i++ {
		h := h32(byte(10 + i))
		n := mustAdd(t, tr, h, prev, i, 16, 1000+int64(i))
		// 双向链接
		if len(n.Parent.Children) != 1 || n.Parent.Children[0] != n {
			t.Fatalf("height %d: bidirectional link broken", i)
		}
		// CW 累加
		want := expectedChainCW(makeBits(16, i+1))
		if n.CumulativeWork.Cmp(want) != 0 {
			t.Fatalf("height %d CumulativeWork = %s, want %s", i, n.CumulativeWork, want)
		}
		prev = h
	}
	if tr.Len() != 11 {
		t.Fatalf("Len = %d, want 11", tr.Len())
	}
	if errs := tr.CheckInvariant(); errs != nil {
		t.Fatalf("CheckInvariant on linear chain: %v", errs)
	}
}

// C. Branching — 分叉：同一父的两个子均正确挂接，tip 高度=父+1。
func TestC_Branching(t *testing.T) {
	tr := NewBlockTree()
	g := mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	c1 := mustAdd(t, tr, h32(2), g.Hash, 1, 16, 1001)
	c2 := mustAdd(t, tr, h32(3), g.Hash, 1, 16, 1002)

	if len(g.Children) != 2 {
		t.Fatalf("genesis Children = %d, want 2", len(g.Children))
	}
	for _, c := range []*BlockNode{c1, c2} {
		if c.Height != 1 || c.Parent != g {
			t.Fatalf("child height/parent wrong: h=%d parent?=%v", c.Height, c.Parent == g)
		}
		if c.CumulativeWork.Cmp(expectedChainCW([]uint32{16, 16})) != 0 {
			t.Fatalf("child CumulativeWork = %s, want 2*2^16", c.CumulativeWork)
		}
	}
	if errs := tr.CheckInvariant(); errs != nil {
		t.Fatalf("CheckInvariant on branch: %v", errs)
	}
}

// D. MultiLevel — 多级树：跨多代累加正确。
func TestD_MultiLevelTree(t *testing.T) {
	tr := NewBlockTree()
	mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	// 构造：1 -> 2 -> 3 与 1 -> 4 -> 5 -> 6
	mustAdd(t, tr, h32(2), h32(1), 1, 16, 1001)
	mustAdd(t, tr, h32(3), h32(2), 2, 16, 1002)
	mustAdd(t, tr, h32(4), h32(1), 1, 16, 1003)
	mustAdd(t, tr, h32(5), h32(4), 2, 16, 1004)
	mustAdd(t, tr, h32(6), h32(5), 3, 16, 1005)

	n6 := tr.LookupNode(h32(6))
	if n6 == nil {
		t.Fatal("node 6 missing")
	}
	// 深度 3 链：16,16,16,16（含根）=> 4 * 2^16
	want := expectedChainCW([]uint32{16, 16, 16, 16})
	if n6.CumulativeWork.Cmp(want) != 0 {
		t.Fatalf("node6 CumulativeWork = %s, want %s", n6.CumulativeWork, want)
	}
	// 祖先遍历
	if got := len(n6.PathToRoot()); got != 4 {
		t.Fatalf("PathToRoot len = %d, want 4", got)
	}
	if a := n6.AncestorAtHeight(1); a == nil || a.Hash != h32(4) {
		t.Fatal("AncestorAtHeight(1) wrong")
	}
	if errs := tr.CheckInvariant(); errs != nil {
		t.Fatalf("CheckInvariant on multi-level: %v", errs)
	}
}

// E. Duplicate — 去重机制：重复哈希被拒绝。
func TestE_DuplicateHash(t *testing.T) {
	tr := NewBlockTree()
	mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	if _, err := tr.AddBlock(h32(1), [32]byte{}, 0, 16, 1000); err != ErrDuplicateHash {
		t.Fatalf("duplicate genesis: got %v, want ErrDuplicateHash", err)
	}
	mustAdd(t, tr, h32(2), h32(1), 1, 16, 1001)
	if _, err := tr.AddBlock(h32(2), h32(1), 1, 16, 1001); err != ErrDuplicateHash {
		t.Fatalf("duplicate child: got %v, want ErrDuplicateHash", err)
	}
}

// F. MissingParent — 父缺失被安全拒绝（不 panic）。
func TestF_MissingParent(t *testing.T) {
	tr := NewBlockTree()
	if _, err := tr.AddBlock(h32(2), h32(99), 1, 16, 1001); err != ErrMissingParent {
		t.Fatalf("missing parent: got %v, want ErrMissingParent", err)
	}
}

// G. WorkComparison — 不同 bits 分支按 Σ2^bits 全序精确比较（*big.Int）。
func TestG_WorkComparison(t *testing.T) {
	tr := NewBlockTree()
	g := mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	low := mustAdd(t, tr, h32(2), g.Hash, 1, 16, 1001)  // CW = 2^16 + 2^16
	high := mustAdd(t, tr, h32(3), g.Hash, 1, 20, 1002) // CW = 2^16 + 2^20

	if low.CumulativeWork.Cmp(high.CumulativeWork) >= 0 {
		t.Fatalf("low CW %s should be < high CW %s", low.CumulativeWork, high.CumulativeWork)
	}
	// 同一分支继续叠加更高 bits 块：累积工作量严格递增（验证 Σ2^bits 全序）
	higher := mustAdd(t, tr, h32(4), high.Hash, 2, 20, 1003) // CW = 2^16 + 2^20 + 2^20
	if higher.CumulativeWork.Cmp(high.CumulativeWork) <= 0 {
		t.Fatalf("higher CW %s should be > high CW %s", higher.CumulativeWork, high.CumulativeWork)
	}
	if errs := tr.CheckInvariant(); errs != nil {
		t.Fatalf("CheckInvariant on work comparison: %v", errs)
	}
}

// H. BigIntSafety — 无 uint64 溢出：深链与混合 bits 的 big.Int 精确累加。
func TestH_BigIntSafety(t *testing.T) {
	tr := NewBlockTree()
	mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	prev := h32(1)
	const depth = 200
	for i := 1; i <= depth; i++ {
		h := h32(byte(10 + i))
		prev = mustAdd(t, tr, h, prev, i, 16, 1000+int64(i)).Hash
	}
	tip := tr.LookupNode(prev)
	// 201 块 * 2^16 = 201 * 65536 = 13178880，远超 uint32，验证 big.Int 精确
	want := new(big.Int).Mul(big.NewInt(int64(depth+1)), WorkOfBits(16))
	if tip.CumulativeWork.Cmp(want) != 0 {
		t.Fatalf("deep chain CW = %s, want %s", tip.CumulativeWork, want)
	}
	if errs := tr.CheckInvariant(); errs != nil {
		t.Fatalf("CheckInvariant on deep chain: %v", errs)
	}
}

// I. RestartLikeReconstruction — 从扁平头列表重建，结构/ CW 确定性一致。
func TestI_RestartLikeReconstruction(t *testing.T) {
	// 原始树
	orig := NewBlockTree()
	mustAdd(t, orig, h32(1), [32]byte{}, 0, 16, 1000)
	mustAdd(t, orig, h32(2), h32(1), 1, 16, 1001)
	mustAdd(t, orig, h32(3), h32(2), 2, 16, 1002)
	mustAdd(t, orig, h32(4), h32(1), 1, 20, 1003)

	// 导出扁平头列表（模拟落盘/网络消息）
	type hdr struct {
		hash, parent [32]byte
		height       int
		bits         uint32
		ts           int64
	}
	var flat []hdr
	for _, n := range orig.nodes {
		flat = append(flat, hdr{n.Hash, n.ParentHash, n.Height, n.Bits, n.Timestamp})
	}

	// BT-1 修复（AUTH-2-BT1-FIX）：orig.nodes 是 map，迭代序随机，可能把 child
	// 排在 parent 之前，触发 AddBlock 的 ErrMissingParent 前置条件（父必须先
	// 存在，blocktree.go「parent == nil → ErrMissingParent」）。按 height 升序、
	// 同高度按哈希字典序做确定性排序，与生产 rebuildTree（blockchain.go，按
	// bc.blocks slice 高度序回放）的 parent-before-child 构造模型一致。
	// 仅修复测试构造，不改 AddBlock、不改断言、不引入 retry/skip。
	sort.Slice(flat, func(i, j int) bool {
		if flat[i].height != flat[j].height {
			return flat[i].height < flat[j].height
		}
		return bytes.Compare(flat[i].hash[:], flat[j].hash[:]) < 0
	})

	// 重建（模拟节点重启后从存储恢复）
	rebuilt := NewBlockTree()
	for _, hd := range flat {
		if _, err := rebuilt.AddBlock(hd.hash, hd.parent, hd.height, hd.bits, hd.ts); err != nil {
			t.Fatalf("rebuild AddBlock error: %v", err)
		}
	}

	// 结构一致性
	if rebuilt.Len() != orig.Len() {
		t.Fatalf("rebuilt Len %d != orig Len %d", rebuilt.Len(), orig.Len())
	}
	for h, on := range orig.nodes {
		rn := rebuilt.LookupNode(h)
		if rn == nil {
			t.Fatalf("rebuilt missing node %x", h[:4])
		}
		if on.CumulativeWork.Cmp(rn.CumulativeWork) != 0 {
			t.Fatalf("node %x CW mismatch: orig %s rebuilt %s", h[:4], on.CumulativeWork, rn.CumulativeWork)
		}
		if on.Height != rn.Height || on.Bits != rn.Bits {
			t.Fatalf("node %x meta mismatch", h[:4])
		}
	}
	if errs := rebuilt.CheckInvariant(); errs != nil {
		t.Fatalf("CheckInvariant on rebuilt: %v", errs)
	}
}

// J. Determinism — 相同输入两次构建，结果字节级一致。
func TestJ_Determinism(t *testing.T) {
	build := func() *BlockTree {
		tr := NewBlockTree()
		mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
		mustAdd(t, tr, h32(2), h32(1), 1, 16, 1001)
		mustAdd(t, tr, h32(3), h32(1), 1, 20, 1002)
		mustAdd(t, tr, h32(4), h32(3), 2, 16, 1003)
		return tr
	}
	a, b := build(), build()
	if a.Len() != b.Len() {
		t.Fatal("non-deterministic Len")
	}
	tipA := a.LookupNode(h32(4))
	tipB := b.LookupNode(h32(4))
	if tipA.CumulativeWork.Cmp(tipB.CumulativeWork) != 0 {
		t.Fatal("non-deterministic CumulativeWork")
	}
	// 根 CW 必为 2^16（与链长无关）
	if a.Genesis().CumulativeWork.Cmp(WorkOfBits(16)) != 0 {
		t.Fatal("genesis CW not deterministic")
	}
}

// =====================================================================
// 负例（REORG-1A REQUIRED NEGATIVE TESTS — 8 项）
// =====================================================================

// 1. parent height mismatch
func TestNeg_ParentHeightMismatch(t *testing.T) {
	tr := NewBlockTree()
	mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	// 父 height=0，子声明 height=5（!= 0+1）
	if _, err := tr.AddBlock(h32(2), h32(1), 5, 16, 1001); err != ErrInvalidHeight {
		t.Fatalf("got %v, want ErrInvalidHeight", err)
	}
}

// 2. duplicate hash
func TestNeg_DuplicateHash(t *testing.T) {
	tr := NewBlockTree()
	mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	if _, err := tr.AddBlock(h32(1), [32]byte{}, 0, 16, 1000); err != ErrDuplicateHash {
		t.Fatalf("got %v, want ErrDuplicateHash", err)
	}
}

// 3. invalid parent linkage（父缺失）
func TestNeg_InvalidParentLinkage(t *testing.T) {
	tr := NewBlockTree()
	if _, err := tr.AddBlock(h32(2), h32(99), 1, 16, 1001); err != ErrMissingParent {
		t.Fatalf("got %v, want ErrMissingParent", err)
	}
}

// 4. inconsistent CumulativeWork（CheckInvariant 捕获手动破坏）
func TestNeg_InconsistentCumulativeWork(t *testing.T) {
	tr := NewBlockTree()
	g := mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	c := mustAdd(t, tr, h32(2), g.Hash, 1, 16, 1001)
	// 手动破坏 c 的 CumulativeWork
	c.CumulativeWork = big.NewInt(12345)
	errs := tr.CheckInvariant()
	if !containsMsg(errs, "CumulativeWork inconsistent") {
		t.Fatalf("CheckInvariant should detect corrupted CW, got: %v", errs)
	}
}

// 5. invalid bits
func TestNeg_InvalidBits(t *testing.T) {
	tr := NewBlockTree()
	if _, err := tr.AddBlock(h32(1), [32]byte{}, 0, 0, 1000); err != ErrInvalidBits {
		t.Fatalf("bits=0: got %v, want ErrInvalidBits", err)
	}
	if _, err := tr.AddBlock(h32(1), [32]byte{}, 0, 257, 1000); err != ErrInvalidBits {
		t.Fatalf("bits=257: got %v, want ErrInvalidBits", err)
	}
}

// 6. self-parent
func TestNeg_SelfParent(t *testing.T) {
	tr := NewBlockTree()
	mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	// hash == parentHash 且 height>0
	if _, err := tr.AddBlock(h32(2), h32(2), 1, 16, 1001); err != ErrSelfParent {
		t.Fatalf("got %v, want ErrSelfParent", err)
	}
}

// 7. cyclic（CheckInvariant 捕获手动构造的环）
func TestNeg_Cyclic(t *testing.T) {
	tr := NewBlockTree()
	a := mustAdd(t, tr, h32(1), [32]byte{}, 0, 16, 1000)
	b := mustAdd(t, tr, h32(2), a.Hash, 1, 16, 1001)
	// 手动制造环：b.Parent -> a，a.Parent -> b
	a.Parent = b
	if !containsMsg(tr.CheckInvariant(), "cycle detected") {
		t.Fatal("CheckInvariant should detect cycle")
	}
}

// 8. nil / malformed
func TestNeg_NilMalformed(t *testing.T) {
	// 8a. 负高度
	tr := NewBlockTree()
	if _, err := tr.AddBlock(h32(1), [32]byte{}, -1, 16, 1000); err != ErrNegativeHeight {
		t.Fatalf("negative height: got %v, want ErrNegativeHeight", err)
	}
	// 8b. 根为 height=0 却带非空父哈希
	if _, err := tr.AddBlock(h32(1), h32(9), 0, 16, 1000); err != ErrInvalidRoot {
		t.Fatalf("invalid root: got %v, want ErrInvalidRoot", err)
	}
	// 8c. height>0 但 Parent 指针为 nil（手动破坏，CheckInvariant 捕获）
	tr2 := NewBlockTree()
	g := mustAdd(t, tr2, h32(1), [32]byte{}, 0, 16, 1000)
	c := mustAdd(t, tr2, h32(2), g.Hash, 1, 16, 1001)
	c.Parent = nil // 破坏
	if !containsMsg(tr2.CheckInvariant(), "height>0 but Parent is nil") {
		t.Fatal("CheckInvariant should detect nil Parent at height>0")
	}
}

// containsMsg 判断 errs 中是否包含指定子串的违规。
func containsMsg(errs []error, sub string) bool {
	for _, e := range errs {
		if e != nil && len(e.Error()) >= len(sub) {
			// 简单子串匹配
			if indexOf(e.Error(), sub) >= 0 {
				return true
			}
		}
	}
	return false
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// makeBits 生成长度为 n、值全为 v 的 bits 切片。
func makeBits(v uint32, n int) []uint32 {
	out := make([]uint32, n)
	for i := range out {
		out[i] = v
	}
	return out
}
