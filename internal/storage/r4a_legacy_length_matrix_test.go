package storage

// ════════════════════════════════════════════════════════════════════════════
// REORG-1J-R4A · LEGACY-LENGTH × FORK-POSITION CANONICAL RECOVERY MATRIX
//
// 目标：证明 R3 已确立的 crash / recovery / canonical 不变式**不只在 legacyLen=3
// 成立**，而在不同 legacy 前缀长度与不同 fork position 下均成立。核心是：
//
//   legacy-internal（f = L-2）⇒ 新 canonical v2 区块**真正占据 legacy physical
//   prefix 对应的高度槽位**，而 legacy 字节逐字节不可变；崩溃/撕裂恢复后 canonical
//   状态确定性恢复，并可通过生产路径重新收敛到新 canonical 链。
//
// ── 几何 ────────────────────────────────────────────────────────────────────
//   L      legacy 记录数（高度 0..L-1）
//   k = 4  旧 canonical 链的 v2 段长度（受 tipRingSize=8 约束：k 枚 v2 TIP + 1 枚
//          reorg TIP = 5 ≤ 8，回滚窗口安全）
//   oldTop = L + k - 1          新分支必须比旧链多一枚块（统一 bits ⇒ work 严格更大）
//   newTop = oldTop + 1         n = newTop - f ≤ 6 ⇒ delta 规模与 L 解耦
//
//   fork position（按「新分支首块落在哪个槽位」划分）：
//     legacy-internal  f = L-2  ⇒ f+1 = L-1 < L ⇒ 首块落入 legacy 槽位（需 L ≥ 2）
//     legacy-boundary  f = L-1  ⇒ f+1 = L       ⇒ 首块占首个 v2 槽位（需 L ≥ 1）
//     v2-region        f = L    ⇒ 挂载点为 v2 块（L=0 时 f=0 挂载 v2 创世）
//
// ── 范围锁定（STRICT CONTROLLED TEST-ONLY）──────────────────────────────────
//   production code diff = 0；不修改 G08 / G09；不修改 r3_crash_matrix_test.go；
//   不修 OBS-1J-R3-A（按「先 detached 落盘、最后一枚由 CommitReorg 处理」绕行）；
//   不新增 crash hook。崩溃注入 = 字节级 torn-tail（与 G06/R3 同源）。
//
// ── 与 R3 的代码复用边界 ────────────────────────────────────────────────────
//   可复用（几何无关）：r3Clone / r3DecodeFrames / r3FrameSeq / r3Probe /
//   r3ProbeOf / r3OpenIn / r3Open / r3Opened。
//   不可复用：r3BuildImage / r3WantChain / r3AssertLegal / r3Converge / r3RunCase
//   —— 它们硬编码 R3 几何常量（r3ForkAt / r3ForkTop），而本阶段禁止修改该文件，
//   故按参数化几何重写同名逻辑（r4a*）。
// ════════════════════════════════════════════════════════════════════════════

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/utxo"
)

// ── 几何 ────────────────────────────────────────────────────────────────────

// r4aK 旧 canonical 链的 v2 段长度（见文件头几何说明）。
const r4aK = 4

type r4aPosKind int

const (
	r4aInternal r4aPosKind = iota // legacy-internal
	r4aBoundary                   // legacy-boundary
	r4aV2Region                   // v2-region
)

func (p r4aPosKind) String() string {
	switch p {
	case r4aInternal:
		return "legacy-internal"
	case r4aBoundary:
		return "legacy-boundary"
	default:
		return "v2-region"
	}
}

// r4aCell 一个矩阵格。
type r4aCell struct {
	L   int
	pos r4aPosKind
}

func (c r4aCell) id() string      { return fmt.Sprintf("L%d/%s", c.L, c.pos) }
func (c r4aCell) oldTop() int     { return c.L + r4aK - 1 }
func (c r4aCell) newTop() int     { return c.oldTop() + 1 }
func (c r4aCell) forkPoint() int  { return r4aForkPoint(c) }
func (c r4aCell) newBlocks() int  { return c.newTop() - c.forkPoint() }
func (c r4aCell) legacySlots() int { return c.L }

// r4aForkPoint 返回该格的分叉点高度 f（最后共同祖先）。
func r4aForkPoint(c r4aCell) int {
	switch c.pos {
	case r4aInternal:
		return c.L - 2
	case r4aBoundary:
		return c.L - 1
	default:
		return c.L
	}
}

// r4aCells 15 个适用格（3 个 N/A 不在此列，见 r4aNotApplicable）。
func r4aCells() []r4aCell {
	out := make([]r4aCell, 0, 15)
	for _, L := range []int{0, 1, 2, 3, 8, 64} {
		for _, pos := range []r4aPosKind{r4aInternal, r4aBoundary, r4aV2Region} {
			if r4aIsNA(L, pos) {
				continue
			}
			out = append(out, r4aCell{L: L, pos: pos})
		}
	}
	return out
}

// r4aIsNA 判定该组合是否 NOT APPLICABLE（只记录原因，不为凑矩阵改生产）。
func r4aIsNA(L int, pos r4aPosKind) bool {
	switch {
	case L == 0 && pos == r4aInternal:
		return true // 无 legacy 记录 ⇒ 「v2 块落 legacy 槽位」集合为空
	case L == 0 && pos == r4aBoundary:
		return true // legacy/v2 边界即创世 ⇒ 退化为 v2-region
	case L == 1 && pos == r4aInternal:
		return true // 唯一 legacy 槽位 = 创世；替换创世违反共享创世 + SP-3b
	}
	return false
}

// r4aNAReason 返回 N/A 的形式化原因（供报告与日志引用）。
func r4aNAReason(L int, pos r4aPosKind) string {
	switch {
	case L == 0 && pos == r4aInternal:
		return "L=0 不存在 legacy 记录，『v2 区块落入 legacy 槽位』的集合为空；要制造该形态必须先写入 legacy 记录，与 L=0 矛盾"
	case L == 0 && pos == r4aBoundary:
		return "L=0 时 legacy/v2 边界位于高度 0（创世），该格退化为『挂载 v2 创世分叉』= v2-region，无独立信息量"
	case L == 1 && pos == r4aInternal:
		return "L=1 时唯一 legacy 槽位是高度 0（创世）；reorg 按定义共享创世，且 SP-3b 规定零父哈希仅允许存储为空时 ⇒ 不可构造"
	}
	return ""
}

// ── 旧链缓存（同一 L 只挖一次，三个 fork position 复用字节快照）─────────────

type r4aOld struct {
	L           int
	blocks      []*block.Block // 0..oldTop
	undos       []utxo.BlockUndo
	legacyBytes []byte // legacy 区字节（L=0 时为空）
	pre         []byte // C0 基线：legacy + k 枚 v2 canonical（已重开验证）
	oldTip      [32]byte
	oldHeight   int
}

var r4aOldCache = map[int]*r4aOld{}

func r4aOldChain(t *testing.T, L int) *r4aOld {
	t.Helper()
	if o, ok := r4aOldCache[L]; ok {
		return o
	}
	dir := t.TempDir()
	total := L + r4aK
	blocks, undos := iChain(t, total)

	s, err := OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("L=%d 打开存储失败: %v", L, err)
	}
	for h := 0; h < L; h++ {
		if err := s.SaveBlock(blocks[h]); err != nil {
			t.Fatalf("L=%d legacy 写入 %d 失败: %v", L, h, err)
		}
	}
	var legacyBytes []byte
	if L > 0 {
		legacyBytes = r3Clone(iReadLog(t, dir))
	}
	for h := L; h < total; h++ {
		if err := s.AppendCanonicalBlock(blocks[h], undos[h]); err != nil {
			t.Fatalf("L=%d v2 canonical 追加 %d 失败: %v", L, h, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("L=%d 关闭失败: %v", L, err)
	}

	// 重开一次 ⇒ pre 必须是**可重启解释**的稳定基线
	s2, err := OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("L=%d 重开失败: %v", L, err)
	}
	h, err := s2.Height()
	if err != nil {
		t.Fatalf("L=%d Height 失败: %v", L, err)
	}
	want := L + r4aK - 1
	if h != want {
		t.Fatalf("L=%d 前提不成立：旧链高度 = %d, want %d", L, h, want)
	}
	if got := s2.LegacyRecordCount(); got != L {
		t.Fatalf("L=%d 前提不成立：legacy 记录数 = %d, want %d", L, got, L)
	}
	tip, hasTip := s2.TipHash()
	if !hasTip {
		t.Fatalf("L=%d 前提不成立：旧链无 TIP 提交", L)
	}
	pre := r3Clone(iReadLog(t, dir))
	if err := s2.Close(); err != nil {
		t.Fatalf("L=%d 重开后关闭失败: %v", L, err)
	}

	o := &r4aOld{L: L, blocks: blocks, undos: undos, legacyBytes: legacyBytes,
		pre: pre, oldTip: tip, oldHeight: h}
	r4aOldCache[L] = o
	return o
}

// r4aForkOf 自 main[f] 分叉挖 n 枚块（tag 0x22 ⇒ 与主链同高度不同哈希），
// undo 由 ApplyBlockWithUndo 真实生成（非人造数据）。
func r4aForkOf(t *testing.T, main []*block.Block, f, n int) ([]*block.Block, []utxo.BlockUndo) {
	t.Helper()
	set := utxo.NewUTXOSet()
	for h := 0; h <= f; h++ {
		ns, _, _, err := utxo.ApplyBlockWithUndo(set, main[h].Transactions, h)
		if err != nil {
			t.Fatalf("ancestor 状态重建失败 height=%d: %v", h, err)
		}
		set = ns
	}
	out := make([]*block.Block, 0, n)
	undos := make([]utxo.BlockUndo, 0, n)
	ph := main[f].Header.Hash()
	for h := f + 1; h <= f+n; h++ {
		b := iBlock(t, ph, h, 0x22)
		ns, undo, _, err := utxo.ApplyBlockWithUndo(set, b.Transactions, h)
		if err != nil {
			t.Fatalf("分叉块 undo 生成失败 height=%d: %v", h, err)
		}
		set = ns
		out = append(out, b)
		undos = append(undos, undo)
		ph = b.Header.Hash()
	}
	return out, undos
}

// ── 崩溃镜像 ────────────────────────────────────────────────────────────────

type r4aImg struct {
	cell  r4aCell
	f     int
	n     int
	tag   string // "A-all-detached" / "B-new-tip-block"

	legacyBytes []byte
	pre         []byte
	base        []byte
	delta       []byte
	full        []byte

	oldChain []*block.Block // 0..oldTop
	fork     []*block.Block // f+1..newTop
	forkUndo []utxo.BlockUndo

	oldTip, newTip   [32]byte
	oldHeight        int
	newHeight        int

	storedInBase int
	tipFrameLen  int
	blockFrameL  int
	deltaFrames  []frameInfo
}

// r4aCut 返回「base + delta 前 k 字节」的崩溃镜像。
func (img *r4aImg) r4aCut(k int) []byte {
	if k < 0 {
		k = 0
	}
	if k > len(img.delta) {
		k = len(img.delta)
	}
	return append(r3Clone(img.base), img.delta[:k]...)
}

// r4aWantChain 返回给定链尾对应的合法 canonical 链。
func (img *r4aImg) r4aWantChain(tip [32]byte) []*block.Block {
	if tip == img.newTip {
		chain := append([]*block.Block(nil), img.oldChain[:img.f+1]...)
		return append(chain, img.fork...)
	}
	return img.oldChain
}

var r4aImgCache = map[string]*r4aImg{}

// r4aBuild 构造一个格的完整崩溃镜像（variant A：全 detached；variant B：新链尾未落盘）。
//
// 镜像按 (格, 变体) 缓存——镜像内容是不可变字节与区块切片，跨用例复用可把挖矿
// 成本从「每用例一次」降为「每格一次」。
func r4aBuild(t *testing.T, c r4aCell, newTipIsNew bool) *r4aImg {
	t.Helper()
	key := fmt.Sprintf("%s/%v", c.id(), newTipIsNew)
	if img, ok := r4aImgCache[key]; ok {
		return img
	}
	img := r4aBuildUncached(t, c, newTipIsNew)
	r4aImgCache[key] = img
	return img
}

func r4aBuildUncached(t *testing.T, c r4aCell, newTipIsNew bool) *r4aImg {
	t.Helper()
	old := r4aOldChain(t, c.L)
	f := c.forkPoint()
	n := c.newBlocks()
	if f < 0 {
		t.Fatalf("%s 前提不成立：分叉点 f=%d < 0", c.id(), f)
	}
	fork, forkUndo := r4aForkOf(t, old.blocks, f, n)

	dir := t.TempDir()
	iWriteLog(t, dir, old.pre)
	s, err := OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("%s 打开存储失败: %v", c.id(), err)
	}
	defer func() { _ = s.Close() }()

	img := &r4aImg{
		cell:        c,
		f:           f,
		n:           n,
		legacyBytes: r3Clone(old.legacyBytes),
		pre:         r3Clone(old.pre),
		oldChain:    old.blocks,
		fork:        fork,
		forkUndo:    forkUndo,
		oldTip:      old.oldTip,
		oldHeight:   old.oldHeight,
	}

	stored := n
	if newTipIsNew {
		stored = n - 1
	}
	img.storedInBase = stored
	for i := 0; i < stored; i++ {
		if err := s.SaveBlockDetached(fork[i]); err != nil {
			t.Fatalf("%s detached 写入 fork[%d] 失败: %v", c.id(), i, err)
		}
	}
	img.base = r3Clone(iReadLog(t, dir))

	// 生产同构：executeReorg 只为「已存储但缺 undo」的区块补 undo，
	// 全新区块（若有）随 CommitReorg 一并写入。
	du := make(map[[32]byte]utxo.BlockUndo)
	for i := 0; i < stored; i++ {
		du[fork[i].Header.Hash()] = forkUndo[i]
	}
	var nb []*block.Block
	var nu []utxo.BlockUndo
	if newTipIsNew {
		nb = append(nb, fork[stored])
		nu = append(nu, forkUndo[stored])
	}
	newTipHash := fork[n-1].Header.Hash()
	if err := s.CommitReorg(du, nb, nu, newTipHash); err != nil {
		t.Fatalf("%s CommitReorg 失败: %v", c.id(), err)
	}
	img.newTip = newTipHash
	h, _ := s.Height()
	img.newHeight = h
	if h != c.newTop() {
		t.Fatalf("%s 前提不成立：reorg 后高度 = %d, want %d", c.id(), h, c.newTop())
	}
	if tip, _ := s.TipHash(); tip != newTipHash {
		t.Fatalf("%s 前提不成立：reorg 后链尾 = %x, want %x", c.id(), tip, newTipHash)
	}
	img.full = r3Clone(iReadLog(t, dir))
	if len(img.full) <= len(img.base) {
		t.Fatalf("%s 前提不成立：reorg 未写入任何字节", c.id())
	}
	img.delta = r3Clone(img.full[len(img.base):])

	// delta 帧构成：TIP 数恒 == 1 且位于末位（§11 STOP CONDITION）
	img.deltaFrames = r3DecodeFrames(t, img.delta)
	tipCount := 0
	for i, fr := range img.deltaFrames {
		switch fr.Type {
		case recTypeTip:
			tipCount++
			if i != len(img.deltaFrames)-1 {
				t.Fatalf("%s 前提不成立：TIP 帧不在 delta 末位（位置 %d/%d）", c.id(), i, len(img.deltaFrames))
			}
			img.tipFrameLen = fr.TotalLen
		case recTypeBlock:
			img.blockFrameL = fr.TotalLen
		}
	}
	if tipCount != 1 {
		t.Fatalf("%s 前提不成立：delta 内 TIP 帧数 = %d, want 1", c.id(), tipCount)
	}
	if img.tipFrameLen != frameOverhead+tipPayloadLen {
		t.Fatalf("%s 前提不成立：TIP 帧长度 = %d, want %d", c.id(), img.tipFrameLen, frameOverhead+tipPayloadLen)
	}
	if newTipIsNew {
		img.tag = "B-new-tip-block"
	} else {
		img.tag = "A-all-detached"
	}
	return img
}

// ── 不变式断言（I1–I8 + I11）────────────────────────────────────────────────

// r4aAssertLegal 校验 I1–I8 与 I11。任一不成立即构成 §11 STOP CONDITION。
//
// 与 R3 的 r3AssertLegal 逻辑同构，但 fork point / 高度按本格几何参数化。
func r4aAssertLegal(t *testing.T, id string, img *r4aImg, o *r3Opened) {
	t.Helper()
	p, s := o.p, o.s

	// I3 · legacy 前缀逐字节不变
	if !bytes.HasPrefix(p.logBytes, img.legacyBytes) {
		t.Fatalf("[%s] I3 违反：legacy 前缀被改写（日志 %d 字节，legacy 前缀 %d 字节）", id, len(p.logBytes), len(img.legacyBytes))
	}
	if p.legacyLen != img.cell.L {
		t.Fatalf("[%s] I3 违反：legacy 记录数 = %d, want %d", id, p.legacyLen, img.cell.L)
	}
	// I11 · legacy 槽位被 v2 块占据**不改变物理记录序列**
	if len(s.v2.legacySeq) != img.cell.L {
		t.Fatalf("[%s] I11 违反：legacySeq 长度 = %d, want %d", id, len(s.v2.legacySeq), img.cell.L)
	}
	for h := 0; h < img.cell.L; h++ {
		if s.v2.legacySeq[h].Header.Hash() != img.oldChain[h].Header.Hash() {
			t.Fatalf("[%s] I11 违反：legacySeq 高度 %d 被改写", id, h)
		}
	}

	// I5 · 无非法截断
	if !bytes.HasPrefix(img.full, p.logBytes) {
		t.Fatalf("[%s] I5 违反：重启后的日志不是完整日志的字节前缀（非法 truncate 或字节改写）", id)
	}
	if !bytes.HasPrefix(p.logBytes, img.pre) {
		t.Fatalf("[%s] I5 违反：已提交字节被删除（C0 基线已不是前缀）", id)
	}
	if p.logSize != int64(len(p.logBytes)) {
		t.Fatalf("[%s] I5 违反：LogSize=%d 与文件长度 %d 不一致", id, p.logSize, len(p.logBytes))
	}

	// I1 · TIP 有效：链尾区块存在且哈希 == TIP
	if p.height < 0 {
		t.Fatalf("[%s] I1 违反：canonical 高度为负（%d）", id, p.height)
	}
	top, err := s.GetBlockByHeight(p.height)
	if err != nil {
		t.Fatalf("[%s] I1 违反：TIP 指向的区块不存在（高度 %d）: %v", id, p.height, err)
	}
	if top.Header.Hash() != p.tip {
		t.Fatalf("[%s] I1 违反：TIP=%x 与高度 %d 的区块哈希 %x 不一致", id, p.tip, p.height, top.Header.Hash())
	}
	// I8 / I2 · 无半 reorg：整条 canonical 链必须完全等于链尾所属分支
	if p.tip != img.oldTip && p.tip != img.newTip {
		t.Fatalf("[%s] I8 违反：出现半 reorg 状态 —— 链尾 %x 既非旧链尾 %x 也非新链尾 %x", id, p.tip, img.oldTip, img.newTip)
	}
	want := img.r4aWantChain(p.tip)
	if p.height != len(want)-1 {
		t.Fatalf("[%s] I2 违反：高度 = %d，与链尾所属分支长度 %d 不符", id, p.height, len(want))
	}
	for h, wb := range want {
		gb, err := s.GetBlockByHeight(h)
		if err != nil {
			t.Fatalf("[%s] I2 违反：canonical 高度 %d 不可读: %v", id, h, err)
		}
		if gb.Header.Hash() != wb.Header.Hash() {
			t.Fatalf("[%s] I8 违反：canonical 高度 %d = %x, want %x（分支混杂）", id, h, gb.Header.Hash(), wb.Header.Hash())
		}
		ok, err := s.IsCanonical(wb.Header.Hash())
		if err != nil || !ok {
			t.Fatalf("[%s] I2 违反：高度 %d 的区块未被标记为 canonical (ok=%v err=%v)", id, h, ok, err)
		}
	}

	// I7 · 无幻影 canonical
	if p.canonN != p.height+1 {
		t.Fatalf("[%s] I7 违反：canonical 记录数 = %d，但 canonical 高度 = %d（应相差 1）", id, p.canonN, p.height)
	}

	// I4 · 已落盘区块仍可查；未落盘者不得出现（无幻影）
	for _, b := range img.oldChain {
		if !s.HasBlock(b.Header.Hash()) {
			t.Fatalf("[%s] I4 违反：旧链区块 %x 丢失", id, b.Header.Hash())
		}
	}
	if bytes.HasPrefix(p.logBytes, img.base) {
		for i := 0; i < img.storedInBase; i++ {
			if !s.HasBlock(img.fork[i].Header.Hash()) {
				t.Fatalf("[%s] I4 违反：已落盘分叉块 fork[%d] %x 丢失", id, i, img.fork[i].Header.Hash())
			}
		}
	} else {
		for i := 0; i < img.storedInBase; i++ {
			if s.HasBlock(img.fork[i].Header.Hash()) {
				t.Fatalf("[%s] I7 违反：分叉块 fork[%d] %x 尚未落盘却出现在存储中（幻影）", id, i, img.fork[i].Header.Hash())
			}
		}
	}
}

// ── I9 收敛 / I6 确定性 ─────────────────────────────────────────────────────

// r4aConverge 生产式恢复收敛：补存缺失 detached（父先子后）→ CommitReorg → 重启持久。
//
// 绕行 OBS-1J-R3-A：先按父先子把分叉块全部 detached 落盘，再以 (du,nil,nil,newTip)
// 提交，与 Blockchain.executeReorg 的入参选择规则一致。
func r4aConverge(t *testing.T, img *r4aImg, o *r3Opened) {
	t.Helper()
	if o.p.tip == img.newTip {
		return // 已收敛
	}
	topHash := img.fork[len(img.fork)-1].Header.Hash()
	for _, b := range img.fork {
		if o.s.HasBlock(b.Header.Hash()) {
			continue
		}
		if err := o.s.SaveBlockDetached(b); err != nil {
			t.Fatalf("[%s/variant=%s] I9 违反：补存分叉块 %x 失败: %v", img.cell.id(), img.tag, b.Header.Hash(), err)
		}
	}
	du := make(map[[32]byte]utxo.BlockUndo)
	for i, b := range img.fork {
		h := b.Header.Hash()
		if o.s.HasBlock(h) && !o.s.HasUndo(h) {
			du[h] = img.forkUndo[i]
		}
	}
	if err := o.s.CommitReorg(du, nil, nil, topHash); err != nil {
		t.Fatalf("[%s/variant=%s] I9 违反：重启后重试 reorg 失败: %v", img.cell.id(), img.tag, err)
	}
	if h, _ := o.s.Height(); h != img.cell.newTop() {
		t.Fatalf("[%s] I9 违反：重试后高度 = %d, want %d", img.cell.id(), h, img.cell.newTop())
	}
	if tip, _ := o.s.TipHash(); tip != topHash {
		t.Fatalf("[%s] I9 违反：重试后链尾 = %x, want %x", img.cell.id(), tip, topHash)
	}
	if err := o.closeErr(); err != nil {
		t.Fatalf("[%s] 关闭失败: %v", img.cell.id(), err)
	}
	o2, err := r3Open(t, iReadLog(t, o.dir))
	if err != nil {
		t.Fatalf("[%s] I9 违反：收敛后重启失败: %v", img.cell.id(), err)
	}
	defer o2.close()
	if o2.p.height != img.cell.newTop() || o2.p.tip != topHash {
		t.Fatalf("[%s] I9 违反：收敛后重启状态 = (h=%d tip=%x), want (h=%d tip=%x)",
			img.cell.id(), o2.p.height, o2.p.tip, img.cell.newTop(), topHash)
	}
}

// r4aRunCase 崩溃 → 重启 → 断言 → 二次重启比对（I6）→ 收敛（I9）。
func r4aRunCase(t *testing.T, img *r4aImg, id string, log []byte, wantTip [32]byte, wantHeight int) *r3Probe {
	t.Helper()
	o1, err := r3Open(t, log)
	if err != nil {
		t.Fatalf("[%s] 崩溃后重启失败（canonical 状态必须可判定）: %v", id, err)
	}
	r4aAssertLegal(t, id, img, o1)
	if o1.p.tip != wantTip {
		t.Fatalf("[%s] 期望链尾 = %x，实际 = %x", id, wantTip, o1.p.tip)
	}
	if o1.p.height != wantHeight {
		t.Fatalf("[%s] 期望高度 = %d，实际 = %d", id, wantHeight, o1.p.height)
	}
	if err := o1.closeErr(); err != nil {
		t.Fatalf("[%s] 关闭失败: %v", id, err)
	}

	o2, err := r3Open(t, log)
	if err != nil {
		t.Fatalf("[%s] 第二次重启失败: %v", id, err)
	}
	if o1.p.sig() != o2.p.sig() {
		t.Fatalf("[%s] I6 违反：两次重启状态不一致\n  第一次 %s\n  第二次 %s", id, o1.p.sig(), o2.p.sig())
	}
	if !bytes.Equal(o1.p.logBytes, o2.p.logBytes) {
		t.Fatalf("[%s] I6 违反：两次重启后的磁盘字节不一致（%d vs %d）", id, len(o1.p.logBytes), len(o2.p.logBytes))
	}
	r4aConverge(t, img, o2)
	o2.close()
	return o1.p
}

// ── 第一优先级：L=0 探针 ────────────────────────────────────────────────────

// TestR4A_L0_Probe 独立探针：验证「纯 v2 创世 + 无 legacy 前缀」能否在既有
// 契约下合法构造，并完成一次合法 reorg 与崩溃恢复。
//
// 若失败：属于既有 contract limitation ⇒ L0/v2-region 降级 CONTRACT LIMITATION/N/A；
// 若唯一解是修改 production ⇒ 立即 STOP（本阶段禁止改生产）。
func TestR4A_L0_Probe(t *testing.T) {
	c := r4aCell{L: 0, pos: r4aV2Region}
	img := r4aBuild(t, c, false)

	t.Logf("L0 探针：f=%d oldTop=%d newTop=%d n=%d legacy=%d 字节",
		img.f, img.cell.oldTop(), img.cell.newTop(), img.n, len(img.legacyBytes))
	t.Logf("L0 探针：pre=%d base=%d delta=%d full=%d 字节；帧序列=%s",
		len(img.pre), len(img.base), len(img.delta), len(img.full), r3FrameSeq(img.deltaFrames))

	// 1) v2 创世：高度 0 的区块必须是 v2 记录，且没有 legacy 记录
	o0, err := r3Open(t, img.pre)
	if err != nil {
		t.Fatalf("L0 探针：pre 重启失败: %v", err)
	}
	if o0.p.legacyLen != 0 {
		t.Fatalf("L0 探针：legacy 记录数 = %d, want 0", o0.p.legacyLen)
	}
	g, err := o0.s.GetBlockByHeight(0)
	if err != nil {
		t.Fatalf("L0 探针：读取创世失败: %v", err)
	}
	if g.Header.PrevBlockHash != ([32]byte{}) {
		t.Fatalf("L0 探针：创世父哈希非零")
	}
	rec0 := o0.s.v2.records[g.Header.Hash()]
	if rec0 == nil || !rec0.isV2 {
		t.Fatalf("L0 探针：创世不是 v2 记录（isV2=%v）", rec0 != nil && rec0.isV2)
	}
	if !o0.s.HasUndo(g.Header.Hash()) {
		t.Fatalf("L0 探针：v2 创世缺少 UNDO（违反 I1）")
	}
	o0.close()

	// 2) canonical path / TIP / UNDO / 高度一致性
	p := r4aRunCase(t, img, "L0/v2-region/pre", img.pre, img.oldTip, img.oldHeight)
	t.Logf("L0 探针：pre 重启 → %s", p.sig())

	// 3) 提交点字节穷举（该格 delta 较小，直接全穷举）
	swap := t.TempDir()
	prev := img.oldTip
	transitions, transitionAt := 0, -1
	for k := 0; k <= len(img.delta); k++ {
		o, err := r3OpenIn(t, swap, img.r4aCut(k))
		if err != nil {
			t.Fatalf("L0 探针：k=%d 重启失败: %v", k, err)
		}
		r4aAssertLegal(t, fmt.Sprintf("L0/k=%d", k), img, o)
		if k < len(img.delta) {
			if o.p.tip != img.oldTip || o.p.height != c.oldTop() {
				t.Fatalf("L0 探针：k=%d 期望旧 canonical (h=%d tip=%x)，实际 (h=%d tip=%x)",
					k, c.oldTop(), img.oldTip, o.p.height, o.p.tip)
			}
			if o.p.logSize != int64(len(img.base)+r4aCompleteEnd(img, k)) {
				t.Fatalf("L0 探针：k=%d 撕裂残片未被精确移除 LogSize=%d, want %d",
					k, o.p.logSize, len(img.base)+r4aCompleteEnd(img, k))
			}
		} else if o.p.tip != img.newTip || o.p.height != c.newTop() {
			t.Fatalf("L0 探针：k=%d 期望新 canonical (h=%d tip=%x)，实际 (h=%d tip=%x)",
				k, c.newTop(), img.newTip, o.p.height, o.p.tip)
		}
		if o.p.tip != prev {
			transitions++
			transitionAt = k
			prev = o.p.tip
		}
		if err := o.closeErr(); err != nil {
			t.Fatalf("L0 探针：k=%d 关闭失败: %v", k, err)
		}
	}
	if transitions != 1 {
		t.Fatalf("L0 探针：canonical 跃变次数 = %d, want 1", transitions)
	}
	if transitionAt != len(img.delta) {
		t.Fatalf("L0 探针：跃变发生在 k=%d, want %d", transitionAt, len(img.delta))
	}
	t.Logf("L0 探针：字节穷举 %d 个偏移：0..%d 全部保持旧 canonical，%d 处切换到新 canonical",
		len(img.delta)+1, len(img.delta)-1, transitionAt)

	// 4) 完整提交后的 canonical 全链路
	p2 := r4aRunCase(t, img, "L0/v2-region/full", img.full, img.newTip, img.cell.newTop())
	t.Logf("L0 探针：full 重启 → %s", p2.sig())

	t.Logf("L0 探针结论：可合法构造 ⇒ L0/v2-region 保留为适用格（合成边界，生产当前不可达）")
}

// ── I12 深度签名 ────────────────────────────────────────────────────────────

// r4aDeepSig 覆盖 Height / TipHash / 全部 byHeight 哈希 / legacySeq 及其 canonical
// 归属——用于 I12「同高度双记录」的确定性比对（比 r3Probe.sig 更严格）。
func r4aDeepSig(s *FileBlockStore) string {
	h, _ := s.Height()
	tip, hasTip := s.TipHash()
	out := fmt.Sprintf("h=%d tip=%x hasTip=%v legacy=%d rec=%d det=%d mode=%s size=%d",
		h, tip, hasTip, s.LegacyRecordCount(), s.RecordCount(), s.DetachedCount(), s.RecoveryMode(), s.LogSize())
	for i, b := range s.byHeight {
		out += fmt.Sprintf("|by%d=%x", i, b.Header.Hash())
	}
	for i, lb := range s.v2.legacySeq {
		lh := lb.Header.Hash()
		canon := false
		if r := s.v2.records[lh]; r != nil {
			canon = r.canonical
		}
		out += fmt.Sprintf("|lg%d=%x/canon=%v", i, lh, canon)
	}
	return out
}

// ── 全矩阵：delta 契约 + 字节级 torn-tail 穷举 ──────────────────────────────

// r4aCompleteEnd 返回 delta[:k] 内最后一个**完整帧**的结束偏移（无完整帧则为 0）。
//
// 这是 REPAIR 语义的精确不变量：loadLog 只在存在撕裂残片（tornTruncated）时把文件
// 截断到 committedEnd = 最后一个完整帧末尾；若 k 恰好落在帧边界上，则无残片可截，
// 文件合法保留这些「完整但未提交」的帧（UNDO/BLOCK 不是提交点，TIP 才是）。
// 因此期望 LogSize = len(base) + r4aCompleteEnd(k)，而不是恒等于 len(base)。
func r4aCompleteEnd(img *r4aImg, k int) int {
	end := 0
	for _, fi := range img.deltaFrames {
		if end+fi.TotalLen > k {
			break
		}
		end += fi.TotalLen
	}
	return end
}

// TestR4A_Matrix_ByteSweep 对 15 个适用格逐格执行：
//   - 断言 delta 内 TIP 数 == 1 且位于末位（§11 STOP CONDITION）；
//   - j ∈ [0, len(delta)] 全穷举：j < len(delta) ⇒ 旧 canonical 且
//     LogSize == len(base) + 最后一个完整帧末尾（撕裂残片必被精确移除）；
//     j == len(delta) ⇒ 新 canonical；
//   - canonical 只允许跃变一次，且必须恰好发生在 TIP 帧完整落盘处。
func TestR4A_Matrix_ByteSweep(t *testing.T) {
	for _, c := range r4aCells() {
		c := c
		t.Run(c.id(), func(t *testing.T) {
			img := r4aBuild(t, c, false)
			if img.deltaFrames[len(img.deltaFrames)-1].Type != recTypeTip {
				t.Fatalf("%s delta 末帧不是 TIP", c.id())
			}
			tipStart := len(img.delta) - img.tipFrameLen
			swap := t.TempDir()
			prev := img.oldTip
			transitions, transitionAt := 0, -1
			for k := 0; k <= len(img.delta); k++ {
				o, err := r3OpenIn(t, swap, img.r4aCut(k))
				if err != nil {
					t.Fatalf("%s k=%d 重启失败（canonical 必须可判定）: %v", c.id(), k, err)
				}
				r4aAssertLegal(t, fmt.Sprintf("%s/k=%d", c.id(), k), img, o)
				if k < len(img.delta) {
					if o.p.tip != img.oldTip || o.p.height != c.oldTop() {
						t.Fatalf("%s k=%d：期望旧 canonical (h=%d tip=%x)，实际 (h=%d tip=%x)",
							c.id(), k, c.oldTop(), img.oldTip, o.p.height, o.p.tip)
					}
				if o.p.logSize != int64(len(img.base)+r4aCompleteEnd(img, k)) {
					t.Fatalf("%s k=%d：撕裂残片未被精确移除 LogSize=%d, want %d（base=%d + 完整帧 %d）",
						c.id(), k, o.p.logSize, len(img.base)+r4aCompleteEnd(img, k), len(img.base), r4aCompleteEnd(img, k))
				}
				// TIP 起始偏移必须是完整帧边界（R3 C4 同源断言）
				if k == tipStart && o.p.logSize != int64(len(img.r4aCut(k))) {
					t.Fatalf("%s：TIP 起始偏移 %d 不是完整帧边界（LogSize=%d，镜像=%d）",
						c.id(), tipStart, o.p.logSize, len(img.r4aCut(k)))
				}
			} else {
					if o.p.tip != img.newTip || o.p.height != c.newTop() {
						t.Fatalf("%s k=%d：期望新 canonical (h=%d tip=%x)，实际 (h=%d tip=%x)",
							c.id(), k, c.newTop(), img.newTip, o.p.height, o.p.tip)
					}
				}
				if o.p.tip != prev {
					transitions++
					transitionAt = k
					prev = o.p.tip
				}
				if err := o.closeErr(); err != nil {
					t.Fatalf("%s k=%d 关闭失败: %v", c.id(), k, err)
				}
			}
			if transitions != 1 {
				t.Fatalf("%s canonical 跃变次数 = %d, want 1（提交点必须唯一且确定）", c.id(), transitions)
			}
			if transitionAt != len(img.delta) {
				t.Fatalf("%s 跃变发生在 k=%d, want %d", c.id(), transitionAt, len(img.delta))
			}
			t.Logf("%s f=%d oldTop=%d newTop=%d n=%d | delta=%d 字节（TIP 起始 %d）帧序列=%s | 穷举 %d 个偏移：0..%d 旧 canonical，%d 处切换新 canonical",
				c.id(), img.f, c.oldTop(), c.newTop(), img.n, len(img.delta), tipStart,
				r3FrameSeq(img.deltaFrames), len(img.delta)+1, len(img.delta)-1, transitionAt)
		})
	}
}

// ── legacy-internal：v2 区块真正占据 legacy 高度槽位 ────────────────────────

// TestR4A_LegacyInternal_V2OccupiesLegacySlot 证明 f = L-2 时：
//   - 至少存在一个 h < L，其 canonical 区块是 **v2 区块**，而 legacy 物理记录是**另一枚**区块；
//   - legacy 字节逐字节不变，legacy 记录仍物理可查但非 canonical；
//   - canonical path 由 TIP 的 parent-hash ancestry 唯一确定。
func TestR4A_LegacyInternal_V2OccupiesLegacySlot(t *testing.T) {
	for _, L := range []int{2, 3, 8, 64} {
		c := r4aCell{L: L, pos: r4aInternal}
		t.Run(c.id(), func(t *testing.T) {
			img := r4aBuild(t, c, false)
			if img.f+1 > L-1 {
				t.Fatalf("%s 前提不成立：f+1=%d 未落入 legacy 槽位（L-1=%d）", c.id(), img.f+1, L-1)
			}
			o, err := r3Open(t, img.full)
			if err != nil {
				t.Fatalf("%s full 重启失败: %v", c.id(), err)
			}
			defer o.close()
			s := o.s

			dual := make([]int, 0, 2)
			for h := 0; h < L; h++ {
				canon := s.byHeight[h]
				legacy := s.v2.legacySeq[h]
				if canon.Header.Hash() == legacy.Header.Hash() {
					continue
				}
				dual = append(dual, h)
				want := img.fork[h-(img.f+1)]
				if canon.Header.Hash() != want.Header.Hash() {
					t.Fatalf("%s I12 违反：高度 %d canonical = %x, want 分叉 v2 块 %x",
						c.id(), h, canon.Header.Hash(), want.Header.Hash())
				}
				if want.Header.PrevBlockHash != s.byHeight[h-1].Header.Hash() && h > 0 {
					t.Fatalf("%s I12 违反：高度 %d 的父哈希链断裂", c.id(), h)
				}
				ok, err := s.IsCanonical(legacy.Header.Hash())
				if err != nil {
					t.Fatalf("%s IsCanonical(legacy@%d) 失败: %v", c.id(), h, err)
				}
				if ok {
					t.Fatalf("%s I12 违反：legacy 记录 @%d 仍被标记为 canonical", c.id(), h)
				}
				if !s.HasBlock(legacy.Header.Hash()) {
					t.Fatalf("%s I12 违反：legacy 记录 @%d 物理丢失", c.id(), h)
				}
			}
			if len(dual) == 0 {
				t.Fatalf("%s I12 违反：f=%d 却没有任何 v2 区块占据 legacy 槽位（L=%d）", c.id(), img.f, L)
			}
			if !bytes.HasPrefix(o.p.logBytes, img.legacyBytes) {
				t.Fatalf("%s I3 违反：legacy 前缀字节被改写", c.id())
			}

			// canonical path 由 TIP 的 parent-hash ancestry 唯一确定
			tip, _ := s.TipHash()
			path := make([][32]byte, 0, s.TipRingLen()+len(s.byHeight)+1)
			cur := s.v2.records[tip]
			for cur != nil {
				path = append(path, cur.hash)
				if cur.height == 0 {
					break
				}
				cur = s.v2.records[cur.parent]
			}
			for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
				path[i], path[j] = path[j], path[i]
			}
			if len(path) != len(s.byHeight) {
				t.Fatalf("%s I12 违反：TIP ancestry 长度 %d != canonical 长度 %d", c.id(), len(path), len(s.byHeight))
			}
			for h, want := range path {
				if s.byHeight[h].Header.Hash() != want {
					t.Fatalf("%s I12 违反：canonical 高度 %d = %x，TIP ancestry 推出 %x", c.id(), h, s.byHeight[h].Header.Hash(), want)
				}
			}
			t.Logf("%s v2 占据 legacy 槽位的高度 = %v（共 %d 个）；legacy 前缀 %d 字节保持不变；TIP ancestry 与 canonical 完全一致",
				c.id(), dual, len(dual), len(img.legacyBytes))
		})
	}
}

// TestR4A_I12_DeterministicReopen 同一份 blocks.dat + 同一有效 TIP + 重复启动
// ⇒ 相同 Height / TipHash / byHeight[h] / canonical 归属（逐字节 signature 相同）。
func TestR4A_I12_DeterministicReopen(t *testing.T) {
	for _, c := range []r4aCell{
		{L: 2, pos: r4aInternal},
		{L: 3, pos: r4aInternal},
		{L: 8, pos: r4aInternal},
		{L: 64, pos: r4aInternal},
		{L: 64, pos: r4aBoundary},
		{L: 64, pos: r4aV2Region},
	} {
		c := c
		t.Run(c.id(), func(t *testing.T) {
			img := r4aBuild(t, c, false)
			dir := t.TempDir()
			base := ""
			baseBytes := []byte(nil)
			for i := 0; i < 5; i++ {
				o, err := r3OpenIn(t, dir, img.full)
				if err != nil {
					t.Fatalf("%s 第 %d 次重启失败: %v", c.id(), i+1, err)
				}
				sig := r4aDeepSig(o.s)
				if i == 0 {
					base, baseBytes = sig, o.p.logBytes
				} else if sig != base {
					t.Fatalf("%s I12 违反：第 %d 次启动签名不一致\n  基准 %s\n  实际 %s", c.id(), i+1, base, sig)
				} else if !bytes.Equal(o.p.logBytes, baseBytes) {
					t.Fatalf("%s I12 违反：第 %d 次启动磁盘字节不一致", c.id(), i+1)
				}
				if err := o.closeErr(); err != nil {
					t.Fatalf("%s 第 %d 次关闭失败: %v", c.id(), i+1, err)
				}
			}
			t.Logf("%s 5 次重复启动深度签名一致：h=%d tip=%x legacy=%d",
				c.id(), img.newHeight, img.newTip, c.L)
		})
	}
}

// ── I9 收敛 / I10 重复 ──────────────────────────────────────────────────────

// TestR4A_I9_Converge 每个适用格：崩溃于提交点前 ⇒ 重启后按生产路径重放 reorg
// （父先子后 detached 落盘 → CommitReorg(du,nil,nil,newTip)）⇒ 收敛后的新 canonical
// 必须再次重启后仍然持久。
func TestR4A_I9_Converge(t *testing.T) {
	for _, c := range r4aCells() {
		c := c
		t.Run(c.id(), func(t *testing.T) {
			img := r4aBuild(t, c, false)
			// 提交点前的两个代表性崩溃点：尚未写入 / 只差最后一字节
			for _, k := range []int{0, len(img.delta) - 1} {
				id := fmt.Sprintf("%s/converge-k=%d", c.id(), k)
				r4aRunCase(t, img, id, img.r4aCut(k), img.oldTip, img.cell.oldTop())
			}
			// 已提交：收敛应直接命中（幂等）
			r4aRunCase(t, img, fmt.Sprintf("%s/converge-full", c.id()), img.full, img.newTip, img.cell.newTop())
			t.Logf("%s I9 收敛通过：k=0 / k=%d / full 三处均收敛到 (h=%d tip=%x)",
				c.id(), len(img.delta)-1, img.cell.newTop(), img.newTip)
		})
	}
}

// TestR4A_I10_RepeatFive j ∈ {0, len(delta)-1, len(delta)} × 5 次 ⇒ signature 完全一致。
func TestR4A_I10_RepeatFive(t *testing.T) {
	for _, c := range r4aCells() {
		c := c
		t.Run(c.id(), func(t *testing.T) {
			img := r4aBuild(t, c, false)
			dir := t.TempDir()
			for _, k := range []int{0, len(img.delta) - 1, len(img.delta)} {
				wantTip, wantH := img.oldTip, img.cell.oldTop()
				if k == len(img.delta) {
					wantTip, wantH = img.newTip, img.cell.newTop()
				}
				base := ""
				for i := 0; i < 5; i++ {
					o, err := r3OpenIn(t, dir, img.r4aCut(k))
					if err != nil {
						t.Fatalf("%s k=%d 第 %d 次重启失败: %v", c.id(), k, i+1, err)
					}
					r4aAssertLegal(t, fmt.Sprintf("%s/k=%d/run%d", c.id(), k, i+1), img, o)
					if o.p.tip != wantTip || o.p.height != wantH {
						t.Fatalf("%s k=%d 第 %d 次状态 = (h=%d tip=%x), want (h=%d tip=%x)",
							c.id(), k, i+1, o.p.height, o.p.tip, wantH, wantTip)
					}
					if i == 0 {
						base = o.p.sig()
					} else if o.p.sig() != base {
						t.Fatalf("%s k=%d I10 违反：第 %d 次 signature 不一致\n  基准 %s\n  实际 %s",
							c.id(), k, i+1, base, o.p.sig())
					}
					if err := o.closeErr(); err != nil {
						t.Fatalf("%s k=%d 关闭失败: %v", c.id(), k, err)
					}
				}
				t.Logf("%s k=%d 5 次重复执行状态完全一致：%s", c.id(), k, base)
			}
		})
	}
}

// ── Variant B（仅 L=3 三格，绕行 OBS-1J-R3-A）───────────────────────────────

// TestR4A_VariantB_L3 「最后一枚新块由 CommitReorg 路径写入」的变体。
// 绕行方式：其余分叉块先 detached 落盘，只有链尾一枚作为 newBlocks 传入。
func TestR4A_VariantB_L3(t *testing.T) {
	for _, pos := range []r4aPosKind{r4aInternal, r4aBoundary, r4aV2Region} {
		c := r4aCell{L: 3, pos: pos}
		t.Run(c.id(), func(t *testing.T) {
			img := r4aBuild(t, c, true)
			if img.tag != "B-new-tip-block" {
				t.Fatalf("%s 变体标记 = %s, want B-new-tip-block", c.id(), img.tag)
			}
			swap := t.TempDir()
			prev := img.oldTip
			transitions, transitionAt := 0, -1
			for k := 0; k <= len(img.delta); k++ {
				o, err := r3OpenIn(t, swap, img.r4aCut(k))
				if err != nil {
					t.Fatalf("%s/B k=%d 重启失败: %v", c.id(), k, err)
				}
				r4aAssertLegal(t, fmt.Sprintf("%s/B/k=%d", c.id(), k), img, o)
				if o.p.tip != prev {
					transitions++
					transitionAt = k
					prev = o.p.tip
				}
				if err := o.closeErr(); err != nil {
					t.Fatalf("%s/B k=%d 关闭失败: %v", c.id(), k, err)
				}
			}
			if transitions != 1 || transitionAt != len(img.delta) {
				t.Fatalf("%s/B canonical 跃变 = %d 次 @%d, want 1 次 @%d", c.id(), transitions, transitionAt, len(img.delta))
			}
			r4aRunCase(t, img, fmt.Sprintf("%s/B/converge", c.id()), img.r4aCut(0), img.oldTip, img.cell.oldTop())
			t.Logf("%s/B delta=%d 字节 帧序列=%s | 穷举 %d 个偏移：跃变 1 次 @%d",
				c.id(), len(img.delta), r3FrameSeq(img.deltaFrames), len(img.delta)+1, transitionAt)
		})
	}
}

// ── N/A 三格：只登记，不构造 ────────────────────────────────────────────────

func TestR4A_NotApplicableCells(t *testing.T) {
	cases := []struct {
		L   int
		pos r4aPosKind
	}{
		{0, r4aInternal},
		{0, r4aBoundary},
		{1, r4aInternal},
	}
	for _, cs := range cases {
		if !r4aIsNA(cs.L, cs.pos) {
			t.Fatalf("L%d/%s 应判定为 N/A", cs.L, cs.pos)
		}
		t.Logf("N/A 保留：L%d/%s —— %s", cs.L, cs.pos, r4aNAReason(cs.L, cs.pos))
	}
	if got := len(r4aCells()); got != 15 {
		t.Fatalf("适用格数量 = %d, want 15", got)
	}
}

// ── A1 · TruncateFromHeight 跨入 legacy 区 ──────────────────────────────────

func TestR4A_A1_TruncateCrossingLegacy(t *testing.T) {
	for _, L := range []int{8, 64} {
		t.Run(fmt.Sprintf("L%d", L), func(t *testing.T) {
			old := r4aOldChain(t, L)
			for _, h := range []int{1, 3, L - 1} {
				dir := t.TempDir()
				iWriteLog(t, dir, old.pre)
				s, err := OpenFileBlockStore(dir)
				if err != nil {
					t.Fatalf("L%d h=%d 打开失败: %v", L, h, err)
				}
				rcBefore := s.RecordCount()
				if err := s.TruncateFromHeight(h); err != nil {
					s.Close()
					t.Fatalf("L%d TruncateFromHeight(%d) 失败: %v", L, h, err)
				}
				hh, _ := s.Height()
				tip, _ := s.TipHash()
				rcAfter := s.RecordCount()
				if hh != h-1 {
					s.Close()
					t.Fatalf("L%d h=%d：回退后高度 = %d, want %d", L, h, hh, h-1)
				}
				if tip != old.blocks[h-1].Header.Hash() {
					s.Close()
					t.Fatalf("L%d h=%d：回退后链尾 = %x, want %x", L, h, tip, old.blocks[h-1].Header.Hash())
				}
				if rcAfter != rcBefore {
					s.Close()
					t.Fatalf("L%d h=%d：物理记录数从 %d 变为 %d（不得物理删除）", L, h, rcBefore, rcAfter)
				}
				if s.LegacyRecordCount() != L {
					s.Close()
					t.Fatalf("L%d h=%d：legacy 记录数 = %d, want %d", L, h, s.LegacyRecordCount(), L)
				}
				// 被回退的区块仍物理可查
				if !s.HasBlock(old.blocks[old.oldHeight].Header.Hash()) {
					s.Close()
					t.Fatalf("L%d h=%d：被回退的链尾区块物理丢失", L, h)
				}
				after := iReadLog(t, dir)
				if !bytes.HasPrefix(after, old.legacyBytes) {
					s.Close()
					t.Fatalf("L%d h=%d：legacy 字节被改写", L, h)
				}
				if err := s.Close(); err != nil {
					t.Fatalf("L%d h=%d 关闭失败: %v", L, h, err)
				}
				// 重启后状态保持
				s2, err := OpenFileBlockStore(dir)
				if err != nil {
					t.Fatalf("L%d h=%d 重启失败: %v", L, h, err)
				}
				if h2, _ := s2.Height(); h2 != h-1 {
					s2.Close()
					t.Fatalf("L%d h=%d：重启后高度 = %d, want %d", L, h, h2, h-1)
				}
				if t2, _ := s2.TipHash(); t2 != old.blocks[h-1].Header.Hash() {
					s2.Close()
					t.Fatalf("L%d h=%d：重启后链尾不一致", L, h)
				}
				if err := s2.Close(); err != nil {
					t.Fatalf("L%d h=%d 重启后关闭失败: %v", L, h, err)
				}
				nh := old.blocks[h-1].Header.Hash()
				t.Logf("A1/L%d h=%d → 高度=%d 链尾=%x… legacy=%d 记录数=%d（不减）回退区块仍可查",
					L, h, h-1, nh, L, rcAfter)
			}
		})
	}
}

// ── A2 · legacy 单字节损坏 ⇒ REJECT + 无物理改写 ────────────────────────────

func TestR4A_A2_LegacyByteCorruption(t *testing.T) {
	L := 8
	old := r4aOldChain(t, L)
	n0 := binary.LittleEndian.Uint32(old.pre[0:4])

	for _, tc := range []struct {
		name string
		off  int
	}{
		{"首条 legacy 记录长度前缀", 0},
		{"第二条 legacy 记录长度前缀", int(4 + n0)},
	} {
		dir := t.TempDir()
		bad := r3Clone(old.pre)
		bad[tc.off] = 0x00 // 长度低字节置 0 ⇒ n == 0 ⇒ 非法 legacy 记录长度
		iWriteLog(t, dir, bad)
		s, err := OpenFileBlockStore(dir)
		if err == nil {
			s.Close()
			t.Fatalf("A2/%s：损坏的 legacy 文件被接受（必须 REJECT）", tc.name)
		}
		if !errors.Is(err, ErrCorruptStore) {
			t.Fatalf("A2/%s：错误 = %v, want ErrCorruptStore", tc.name, err)
		}
		got := iReadLog(t, dir)
		if !bytes.Equal(got, bad) {
			t.Fatalf("A2/%s：文件被物理改写（%d → %d 字节）", tc.name, len(bad), len(got))
		}
		t.Logf("A2/%s（偏移 %d）→ REJECT ErrCorruptStore，文件 %d 字节零改写", tc.name, tc.off, len(got))
	}
}

// ── A3 · v2 撕裂 ⇒ REPAIR；纯 legacy 撕裂 ⇒ REJECT ──────────────────────────

var r4aLegacyOnlyCache = map[int][]byte{}

func r4aLegacyOnly(t *testing.T, L int) []byte {
	t.Helper()
	if b, ok := r4aLegacyOnlyCache[L]; ok {
		return b
	}
	old := r4aOldChain(t, L) // 复用已挖出的块，避免重复挖矿
	dir := t.TempDir()
	s, err := OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("legacy-only L=%d 打开失败: %v", L, err)
	}
	for h := 0; h < L; h++ {
		if err := s.SaveBlock(old.blocks[h]); err != nil {
			t.Fatalf("legacy-only L=%d 写入 %d 失败: %v", L, h, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("legacy-only L=%d 关闭失败: %v", L, err)
	}
	b := r3Clone(iReadLog(t, dir))
	r4aLegacyOnlyCache[L] = b
	return b
}

func TestR4A_A3_TearSemantics(t *testing.T) {
	L := 8

	// (a) v2 撕裂：base 之后写入 1 字节残片 ⇒ 必须 REPAIR 到已提交边界
	t.Run("v2-tear-REPAIR", func(t *testing.T) {
		img := r4aBuild(t, r4aCell{L: L, pos: r4aV2Region}, false)
		o, err := r3Open(t, append(r3Clone(img.base), img.delta[0]))
		if err != nil {
			t.Fatalf("A3/v2 撕裂：重启失败: %v", err)
		}
		defer o.close()
		if o.p.logSize != int64(len(img.base)) {
			t.Fatalf("A3/v2 撕裂：LogSize=%d, want %d（未修复到已提交边界）", o.p.logSize, len(img.base))
		}
		if o.p.height != img.cell.oldTop() || o.p.tip != img.oldTip {
			t.Fatalf("A3/v2 撕裂：恢复状态 = (h=%d tip=%x), want 旧 canonical", o.p.height, o.p.tip)
		}
		if o.p.recovery != "REBUILD" {
			t.Fatalf("A3/v2 撕裂：恢复模式 = %s, want REBUILD", o.p.recovery)
		}
		t.Logf("A3/v2 撕裂 → REPAIR 到 %d 字节（= 已提交边界），恢复模式=%s，canonical 保持旧链 (h=%d)",
			o.p.logSize, o.p.recovery, o.p.height)
	})

	// (b) 纯 legacy 撕裂 ⇒ 必须 REJECT（fail-stop，绝不修复）
	t.Run("legacy-tear-REJECT", func(t *testing.T) {
		full := r4aLegacyOnly(t, L)
		dir := t.TempDir()
		torn := r3Clone(full[:len(full)-3])
		iWriteLog(t, dir, torn)
		s, err := OpenFileBlockStore(dir)
		if err == nil {
			s.Close()
			t.Fatalf("A3/legacy 撕裂：被接受（必须 REJECT）")
		}
		if !errors.Is(err, ErrCorruptStore) {
			t.Fatalf("A3/legacy 撕裂：错误 = %v, want ErrCorruptStore", err)
		}
		if got := iReadLog(t, dir); !bytes.Equal(got, torn) {
			t.Fatalf("A3/legacy 撕裂：文件被物理改写（%d → %d）", len(torn), len(got))
		}
		t.Logf("A3/legacy 撕裂（%d 字节 → %d 字节）→ REJECT ErrCorruptStore，零物理改写",
			len(full), len(torn))
	})
}
