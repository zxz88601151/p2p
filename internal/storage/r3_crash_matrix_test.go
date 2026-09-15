package storage

// ════════════════════════════════════════════════════════════════════════════
// REORG-1J-R3 · LEGACY-PREFIX CRASH MATRIX（C0–C6 / I1–I10）
//
// 目标：证明「canonical 链存在 legacy 前缀、一次 reorg 会把 canonical 切换到分叉
// 分支」时，在关键崩溃/中断边界异常终止**不会**产生：
//   - 非法 canonical 状态（TIP 指向缺失区块 / canonical 高度 ≠ TIP）
//   - 半激活链尾（既不是旧链也不是新链）
//   - 被破坏的 legacy 字节
//   - 非法 truncate（删除已提交字节）
//
// 并且：重启后必然回到**唯一合法状态**（提交点之前 = 旧 canonical，之后 = 新 canonical）。
//
// ── §6 只读分析结论（提交边界）────────────────────────────────────────────
//   - canonical 提交点 = FileBlockStore.appendFrames 内的**唯一一次** file.Sync()；
//     提交记录 = TIP 帧（帧组内最后一枚）。
//   - blocktree 无持久化（rebuildTree 由 canonical 链重建）；UTXO 无持久化（启动回放）；
//     活动链尾无独立文件（= TIP 帧）。不存在 journal / active-tip 独立落盘。
//   - 因此「一次 reorg」的全部持久化副作用 = CommitReorg 写入的**一段连续字节**
//     （下称 delta），且 delta 内**只有一枚 TIP 且位于末位**。
//
// ── 崩溃注入方式（§6：禁止新增 production crash hook）─────────────────────
//   字节级 torn-tail 构造：把「提交前日志 + delta 的任意前缀」写盘后重新打开。
//   与既有 TestG06_CrashInjectionMatrix 同源，零生产代码改动，且能**穷举**提交点
//   内的每一个字节偏移（进程级 kill 无法可靠命中该边界）。
//   进程级终止另见 cmd/node/r3_crash_restart_test.go。
//
// 本阶段只新增测试代码，不修改任何生产文件。
// ════════════════════════════════════════════════════════════════════════════

import (
	"bytes"
	"fmt"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/utxo"
)

// ── 场景规模常量 ────────────────────────────────────────────────────────────

const (
	r3LegacyLen = 3 // legacy 前缀高度 0..2（由 legacy SaveBlock 写出）
	r3OldTop    = 5 // 旧 canonical 链尾高度
	r3ForkTop   = 6 // 新分支链尾高度（比旧链多一枚 ⇒ 工作量更大 ⇒ reorg 合法）
	r3ForkAt    = r3LegacyLen - 1 // 分叉点 = legacy 高度 2
)

// r3Image 一次崩溃注入的完整基线。
//
//	pre   ── C0：分叉分支抵达之前
//	base  ── C1：分叉块已 detached 落盘、reorg 尚未提交（= C2/C3 的崩溃镜像）
//	delta ── C4：CommitReorg 单次 fsync 写入的全部字节
//	full  ── C5/C6：base + delta（完全提交）
type r3Image struct {
	variant string // "A-all-detached" / "B-new-tip-block"

	legacyBytes []byte // legacy 前缀字节（I3 比对基准）
	legacyLen   int

	pre   []byte
	base  []byte
	delta []byte
	full  []byte

	oldChain []*block.Block // main[0..5]
	fork     []*block.Block // 高度 3..6
	forkUndo []utxo.BlockUndo

	oldTip, newTip     [32]byte
	oldHeight, newHt   int

	newTipIsNew   bool // true：新链尾由 CommitReorg 首次写入
	storedInBase  int  // base 中已 detached 落盘的分叉块数
	tipFrameLen   int
	blockFrameLen int
	deltaFrames   []frameInfo
}

// r3Probe 一次「崩溃后重启」的可观测状态快照。
type r3Probe struct {
	height   int
	tip      [32]byte
	hasTip   bool
	legacyLen int
	records  int
	detached int
	dangling int
	recovery string
	logSize  int64
	logBytes []byte
	canonN   int // records 中 canonical=true 的数量（内部视图）
}

func (p *r3Probe) sig() string {
	return fmt.Sprintf("h=%d tip=%x hasTip=%v legacy=%d rec=%d det=%d dang=%d mode=%s size=%d canon=%d",
		p.height, p.tip, p.hasTip, p.legacyLen, p.records, p.detached, p.dangling, p.recovery, p.logSize, p.canonN)
}

// r3Opened 一次重启的句柄 + 状态 + 所在目录。
type r3Opened struct {
	dir    string
	s      *FileBlockStore
	p      *r3Probe
	closed bool
}

// close 幂等关闭（注册进 t.Cleanup，确保断言失败时也不泄漏文件句柄）。
func (o *r3Opened) close() {
	if o == nil || o.s == nil || o.closed {
		return
	}
	o.closed = true
	_ = o.s.Close()
}

// closeErr 关闭并返回错误（用于「优雅关闭必须成功」的断言）。
func (o *r3Opened) closeErr() error {
	if o == nil || o.s == nil || o.closed {
		return nil
	}
	o.closed = true
	return o.s.Close()
}

func r3Clone(b []byte) []byte { return append([]byte(nil), b...) }

// ── 场景构造 ────────────────────────────────────────────────────────────────

// r3BuildImage 构造「legacy 前缀 + v2 canonical + 自 legacy 高度分叉的长分支」。
//
// newTipIsNew=false（variant A）：分叉块全部先以 detached 落盘，CommitReorg 只补
// UNDO + 写 TIP。newTipIsNew=true（variant B）：新链尾由 CommitReorg 首次写入
// （UNDO + BLOCK + TIP），用于覆盖「新分支连接途中」的崩溃点。
func r3BuildImage(t *testing.T, newTipIsNew bool) *r3Image {
	t.Helper()
	dir := t.TempDir()
	s, err := OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}

	main, mainUndo := iChain(t, r3OldTop+1) // 高度 0..5

	// 1) legacy 前缀（高度 0..2）—— 模拟 REORG-1E 之前的生产文件
	for h := 0; h < r3LegacyLen; h++ {
		if err := s.SaveBlock(main[h]); err != nil {
			t.Fatalf("legacy 写入 %d 失败: %v", h, err)
		}
	}
	img := &r3Image{
		legacyBytes: r3Clone(iReadLog(t, dir)),
		legacyLen:   r3LegacyLen,
		oldChain:    main,
	}

	// 2) v2 canonical（高度 3..5）
	for h := r3LegacyLen; h <= r3OldTop; h++ {
		if err := s.AppendCanonicalBlock(main[h], mainUndo[h]); err != nil {
			t.Fatalf("v2 canonical 追加 %d 失败: %v", h, err)
		}
	}
	img.pre = r3Clone(iReadLog(t, dir)) // C0
	img.oldTip, _ = s.TipHash()
	img.oldHeight, _ = s.Height()

	// 3) 分叉分支：自 **legacy 高度 2** 分叉 ⇒ v2 区块占据 legacy 槽位（GAP-1H-A 场景）
	set := utxo.NewUTXOSet()
	for h := 0; h <= r3ForkAt; h++ {
		ns, _, _, err := utxo.ApplyBlockWithUndo(set, main[h].Transactions, h)
		if err != nil {
			t.Fatalf("ancestor 状态重建失败 height=%d: %v", h, err)
		}
		set = ns
	}
	ph := main[r3ForkAt].Header.Hash()
	for h := r3ForkAt + 1; h <= r3ForkTop; h++ {
		b := iBlock(t, ph, h, 0x22)
		ns, undo, _, err := utxo.ApplyBlockWithUndo(set, b.Transactions, h)
		if err != nil {
			t.Fatalf("分叉块 undo 生成失败 height=%d: %v", h, err)
		}
		set = ns
		img.fork = append(img.fork, b)
		img.forkUndo = append(img.forkUndo, undo)
		ph = b.Header.Hash()
	}

	// 4) 分叉块以 detached 落盘（每次各自 fsync ⇒ 各自是独立的崩溃边界）
	stored := len(img.fork)
	if newTipIsNew {
		stored = len(img.fork) - 1
	}
	img.storedInBase = stored
	img.newTipIsNew = newTipIsNew
	for i := 0; i < stored; i++ {
		if err := s.SaveBlockDetached(img.fork[i]); err != nil {
			t.Fatalf("detached 写入 fork[%d] 失败: %v", i, err)
		}
	}
	img.base = r3Clone(iReadLog(t, dir)) // C1

	// 5) 提交 reorg —— 唯一的持久化副作用
	newTip := img.fork[len(img.fork)-1].Header.Hash()
	detachedUndos := make(map[[32]byte]utxo.BlockUndo)
	for i := 0; i < stored; i++ {
		detachedUndos[img.fork[i].Header.Hash()] = img.forkUndo[i]
	}
	var newBlocks []*block.Block
	var newUndos []utxo.BlockUndo
	if newTipIsNew {
		newBlocks = append(newBlocks, img.fork[stored])
		newUndos = append(newUndos, img.forkUndo[stored])
	}
	if err := s.CommitReorg(detachedUndos, newBlocks, newUndos, newTip); err != nil {
		t.Fatalf("CommitReorg 失败: %v", err)
	}
	img.newTip = newTip
	img.newHt, _ = s.Height()
	if img.newHt != r3ForkTop {
		t.Fatalf("前提不成立：reorg 后高度 = %d, want %d", img.newHt, r3ForkTop)
	}
	if tip, _ := s.TipHash(); tip != newTip {
		t.Fatalf("前提不成立：reorg 后链尾 = %x, want %x", tip, newTip)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}

	img.full = r3Clone(iReadLog(t, dir)) // C5/C6
	img.delta = r3Clone(img.full[len(img.base):])

	// delta 的帧构成 —— C2/C3 的核心证据：提交字节序列内**只有一枚 TIP 且位于末位**
	img.deltaFrames = r3DecodeFrames(t, img.delta)
	if len(img.deltaFrames) == 0 {
		t.Fatal("前提不成立：reorg 未写入任何字节")
	}
	tipCount := 0
	for i, f := range img.deltaFrames {
		switch f.Type {
		case recTypeTip:
			tipCount++
			if i != len(img.deltaFrames)-1 {
				t.Fatalf("前提不成立：TIP 帧不在 delta 末位（位置 %d/%d）", i, len(img.deltaFrames))
			}
			img.tipFrameLen = f.TotalLen
		case recTypeBlock:
			img.blockFrameLen = f.TotalLen
		}
	}
	if tipCount != 1 {
		t.Fatalf("前提不成立：delta 内 TIP 帧数 = %d, want 1（提交点必须唯一）", tipCount)
	}
	if img.tipFrameLen != frameOverhead+tipPayloadLen {
		t.Fatalf("前提不成立：TIP 帧长度 = %d, want %d", img.tipFrameLen, frameOverhead+tipPayloadLen)
	}

	if newTipIsNew {
		img.variant = "B-new-tip-block"
	} else {
		img.variant = "A-all-detached"
	}
	return img
}

// r3DecodeFrames 顺序解析一段字节中的全部 v2 帧。
func r3DecodeFrames(t *testing.T, data []byte) []frameInfo {
	t.Helper()
	out := make([]frameInfo, 0, 8)
	for pos := 0; pos < len(data); {
		fi, err := decodeFrame(data[pos:])
		if err != nil {
			t.Fatalf("delta 在偏移 %d 处无法解析为帧: %v", pos, err)
		}
		out = append(out, fi)
		pos += fi.TotalLen
	}
	return out
}

// r3Cut 返回「base + delta 前 k 字节」的崩溃镜像。
func (img *r3Image) r3Cut(k int) []byte {
	if k < 0 {
		k = 0
	}
	if k > len(img.delta) {
		k = len(img.delta)
	}
	return append(r3Clone(img.base), img.delta[:k]...)
}

// r3WantChain 返回给定链尾对应的合法 canonical 链。
func (img *r3Image) r3WantChain(tip [32]byte) []*block.Block {
	if tip == img.newTip {
		chain := append([]*block.Block(nil), img.oldChain[:r3ForkAt+1]...)
		return append(chain, img.fork...)
	}
	return img.oldChain
}

// ── 重启 ────────────────────────────────────────────────────────────────────

func r3ProbeOf(s *FileBlockStore) *r3Probe {
	p := &r3Probe{}
	p.height, _ = s.Height()
	p.tip, p.hasTip = s.TipHash()
	p.legacyLen = s.LegacyRecordCount()
	p.records = s.RecordCount()
	p.detached = s.DetachedCount()
	p.dangling = s.DanglingUndoCount()
	p.recovery = s.RecoveryMode()
	p.logSize = s.LogSize()
	n := 0
	for _, r := range s.v2.records {
		if r.canonical {
			n++
		}
	}
	p.canonN = n
	return p
}

// r3OpenIn 在指定目录（复用或新建）以给定崩溃镜像重启存储。
func r3OpenIn(t *testing.T, dir string, log []byte) (*r3Opened, error) {
	t.Helper()
	iWriteLog(t, dir, log)
	s, err := OpenFileBlockStore(dir)
	if err != nil {
		return nil, err
	}
	o := &r3Opened{dir: dir, s: s, p: r3ProbeOf(s)}
	o.p.logBytes = iReadLog(t, dir)
	t.Cleanup(o.close) // 断言失败时也必须释放句柄，否则 Windows 上 TempDir 清理会残留
	return o, nil
}

func r3Open(t *testing.T, log []byte) (*r3Opened, error) {
	return r3OpenIn(t, t.TempDir(), log)
}

// ── 不变式断言 I1–I8 ────────────────────────────────────────────────────────

// r3AssertLegal 校验 I1–I8。任一不成立即构成 §8 STOP CONDITION。
func r3AssertLegal(t *testing.T, id string, img *r3Image, o *r3Opened) {
	t.Helper()
	p, s := o.p, o.s

	// I3 · legacy 前缀逐字节不变
	if !bytes.HasPrefix(p.logBytes, img.legacyBytes) {
		t.Fatalf("[%s] I3 违反：legacy 前缀被改写（日志 %d 字节，legacy 前缀 %d 字节）", id, len(p.logBytes), len(img.legacyBytes))
	}
	if p.legacyLen != img.legacyLen {
		t.Fatalf("[%s] I3 违反：legacy 记录数 = %d, want %d", id, p.legacyLen, img.legacyLen)
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

	// I8 · 无半 reorg：canonical tip 必须是两个合法值之一，且整条链完全等于该分支
	if p.tip != img.oldTip && p.tip != img.newTip {
		t.Fatalf("[%s] I8 违反：出现半 reorg 状态 —— 链尾 %x 既非旧链尾 %x 也非新链尾 %x", id, p.tip, img.oldTip, img.newTip)
	}
	want := img.r3WantChain(p.tip)
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

	// I7 · 无幻影 canonical：canonical 记录数 == canonical 高度 + 1
	if p.canonN != p.height+1 {
		t.Fatalf("[%s] I7 违反：canonical 记录数 = %d，但 canonical 高度 = %d（应相差 1）", id, p.canonN, p.height)
	}

	// I4 · 已落盘区块仍可按哈希找到（detached 亦不得丢失）
	for _, b := range img.oldChain {
		if !s.HasBlock(b.Header.Hash()) {
			t.Fatalf("[%s] I4 违反：旧链区块 %x 丢失", id, b.Header.Hash())
		}
	}
	// fork 区块：崩溃镜像若已包含 base（分叉块已落盘），则必须全部存活；
	// 若尚未包含（C0 / 提交点之前且分叉块未落盘），则必须**确实不存在**（无幻影）。
	if bytes.HasPrefix(p.logBytes, img.base) {
		for i := 0; i < img.storedInBase; i++ {
			if !s.HasBlock(img.fork[i].Header.Hash()) {
				t.Fatalf("[%s] I4 违反：已落盘分叉块 fork[%d] %x 丢失", id, i, img.fork[i].Header.Hash())
			}
		}
	} else {
		for i := 0; i < img.storedInBase; i++ {
			if s.HasBlock(img.fork[i].Header.Hash()) {
				t.Fatalf("[%s] I7 违反：分叉块 fork[%d] %x 尚未落盘却出现在存储中（幻影状态）", id, i, img.fork[i].Header.Hash())
			}
		}
	}
}

// r3Converge I9 —— 重启后重放/重试 reorg，必须收敛到新 canonical，且收敛后再次重启仍持久。
//
// 重试时按 blockchain.executeReorg 的选择规则构造入参（只为「已存储但缺 undo」的
// 区块补 undo），因此崩溃残留的 UNDO 不会触发 ErrDuplicateUndo —— 这正是生产重试
// 路径的幂等性来源。
func r3Converge(t *testing.T, img *r3Image, o *r3Opened) {
	t.Helper()
	if o.p.tip == img.newTip {
		return // 已收敛
	}
	top := img.fork[len(img.fork)-1]
	topHash := top.Header.Hash()

	// 第 1 步：按父先子后补存缺失的分叉块（detached）。
	// 这与生产路径完全一致 —— Blockchain.AddBlock 在 executeReorg 之前必定先
	// SaveBlockDetached，因此 executeReorg 的 actualNewBlocks 恒为空。
	for _, b := range img.fork {
		if o.s.HasBlock(b.Header.Hash()) {
			continue
		}
		if err := o.s.SaveBlockDetached(b); err != nil {
			t.Fatalf("[converge/variant=%s] I9 违反：补存分叉块 %x 失败: %v", img.variant, b.Header.Hash(), err)
		}
	}

	// 第 2 步：按 executeReorg 的选择规则提交（只为「已存储但缺 undo」的区块补 undo）。
	// 崩溃残留的 UNDO 因此不会触发 ErrDuplicateUndo —— 这是生产重试路径的幂等性来源。
	du := make(map[[32]byte]utxo.BlockUndo)
	for i, b := range img.fork {
		h := b.Header.Hash()
		if o.s.HasBlock(h) && !o.s.HasUndo(h) {
			du[h] = img.forkUndo[i]
		}
	}
	if err := o.s.CommitReorg(du, nil, nil, topHash); err != nil {
		t.Fatalf("[%s/variant=%s] I9 违反：重启后重试 reorg 失败: %v", "converge", img.variant, err)
	}
	if h, _ := o.s.Height(); h != r3ForkTop {
		t.Fatalf("I9 违反：重试后高度 = %d, want %d", h, r3ForkTop)
	}
	if tip, _ := o.s.TipHash(); tip != topHash {
		t.Fatalf("I9 违反：重试后链尾 = %x, want %x", tip, topHash)
	}
	if err := o.closeErr(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	// 收敛状态必须持久：再重启一次
	o2, err := r3Open(t, iReadLog(t, o.dir))
	if err != nil {
		t.Fatalf("I9 违反：收敛后重启失败: %v", err)
	}
	defer o2.close()
	if o2.p.height != r3ForkTop || o2.p.tip != topHash {
		t.Fatalf("I9 违反：收敛后重启状态 = (h=%d tip=%x), want (h=%d tip=%x)", o2.p.height, o2.p.tip, r3ForkTop, topHash)
	}
}

// r3RunCase 执行一次完整的「崩溃 → 重启 → 检查 → 收敛」并校验确定性（I6）。
func r3RunCase(t *testing.T, img *r3Image, id string, log []byte, wantTip [32]byte, wantHeight int) *r3Probe {
	t.Helper()

	o1, err := r3Open(t, log)
	if err != nil {
		t.Fatalf("[%s] 崩溃后重启失败（canonical 状态必须可判定）: %v", id, err)
	}
	r3AssertLegal(t, id, img, o1)
	if o1.p.tip != wantTip {
		t.Fatalf("[%s] 期望链尾 = %x，实际 = %x", id, wantTip, o1.p.tip)
	}
	if o1.p.height != wantHeight {
		t.Fatalf("[%s] 期望高度 = %d，实际 = %d", id, wantHeight, o1.p.height)
	}
	if err := o1.closeErr(); err != nil {
		t.Fatalf("[%s] 关闭失败: %v", id, err)
	}

	// I6 · 确定性：同一崩溃镜像独立重启两次必须完全一致
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

	// I9 · 收敛
	r3Converge(t, img, o2)
	o2.close()
	return o1.p
}

// ── C0 · reorg 之前 ─────────────────────────────────────────────────────────

func TestR3_C0_PreReorgCrash(t *testing.T) {
	img := r3BuildImage(t, false)
	p := r3RunCase(t, img, "C0/"+img.variant, img.pre, img.oldTip, r3OldTop)
	if p.detached != 0 {
		t.Fatalf("C0：尚无任何分叉块，detached = %d, want 0", p.detached)
	}
	if p.legacyLen != r3LegacyLen {
		t.Fatalf("C0：legacy 记录数 = %d, want %d", p.legacyLen, r3LegacyLen)
	}
}

// ── C1 · 分叉分支已抵达、尚未切换 ───────────────────────────────────────────

func TestR3_C1_BranchReceivedBeforeSwitch(t *testing.T) {
	for _, newTipIsNew := range []bool{false, true} {
		img := r3BuildImage(t, newTipIsNew)
		id := "C1/" + img.variant
		p := r3RunCase(t, img, id, img.base, img.oldTip, r3OldTop)
		if p.detached != img.storedInBase {
			t.Fatalf("%s：detached = %d, want %d（分叉块必须存活为非 canonical）", id, p.detached, img.storedInBase)
		}
		if p.dangling != 0 {
			t.Fatalf("%s：悬空 UNDO = %d, want 0", id, p.dangling)
		}
		t.Logf("%s → 高度=%d 链尾=%x detached=%d legacy=%d 恢复模式=%s",
			id, p.height, p.tip[:4], p.detached, p.legacyLen, p.recovery)
	}
}

// ── C2 / C3 · disconnect / connect 途中：无持久副作用 ───────────────────────

// TestR3_C2C3_NoDurableIntermediateState 证明 reorg 的 disconnect（回滚旧分支）与
// connect（应用新分支）阶段**不存在任何中间持久状态**：
//
//  1. 一次 reorg 的全部磁盘副作用 = CommitReorg 写入的连续字节 delta；
//  2. delta 内只有一枚 TIP，且位于末位 ⇒ 提交点唯一；
//  3. delta 的**任何真前缀**都不会产生新 canonical（由 C4 字节穷举证明）。
//
// 因此「崩溃于 disconnect/connect 途中」等价于「崩溃于提交点之前」= C1 状态。
func TestR3_C2C3_NoDurableIntermediateState(t *testing.T) {
	for _, newTipIsNew := range []bool{false, true} {
		img := r3BuildImage(t, newTipIsNew)
		id := "C2C3/" + img.variant

		// (1) delta 之外无任何写入：base 与 full 只在尾部相差 delta
		if !bytes.HasPrefix(img.full, img.base) {
			t.Fatalf("%s：full 不是 base 的字节扩展（存在非预期的额外写入/改写）", id)
		}
		if len(img.full) != len(img.base)+len(img.delta) {
			t.Fatalf("%s：长度关系不成立 %d != %d + %d", id, len(img.full), len(img.base), len(img.delta))
		}
		// (2) 提交点唯一：恰好一枚 TIP，且是最后一枚帧（构造期已断言，此处复核）
		tips := 0
		for i, f := range img.deltaFrames {
			if f.Type == recTypeTip {
				tips++
				if i != len(img.deltaFrames)-1 {
					t.Fatalf("%s：TIP 帧不在末位", id)
				}
			}
		}
		if tips != 1 {
			t.Fatalf("%s：delta 内 TIP 帧数 = %d, want 1", id, tips)
		}
		// (3) 崩溃于「提交点起始处」（k=0）必须完全等价于 C1
		p0 := r3ProbeOfCrash(t, img, 0)
		p1 := r3ProbeOfCrash(t, img, 0)
		if p0.sig() != p1.sig() {
			t.Fatalf("%s：k=0 崩溃状态不确定", id)
		}
		if p0.tip != img.oldTip || p0.height != r3OldTop {
			t.Fatalf("%s：提交点之前的崩溃未回到旧 canonical（tip=%x h=%d）", id, p0.tip, p0.height)
		}
		t.Logf("%s delta=%d 字节，帧序列=%s", id, len(img.delta), r3FrameSeq(img.deltaFrames))
	}
}

func r3FrameSeq(fs []frameInfo) string {
	out := ""
	for i, f := range fs {
		if i > 0 {
			out += " → "
		}
		out += fmt.Sprintf("%s(h=%d,%x…)", recTypeName(f.Type), f.Height, f.BlockHash[:3])
	}
	return out
}

// r3ProbeOfCrash 在一次性目录中以 delta 前 k 字节重启并返回探针（不保留句柄）。
func r3ProbeOfCrash(t *testing.T, img *r3Image, k int) *r3Probe {
	t.Helper()
	o, err := r3Open(t, img.r3Cut(k))
	if err != nil {
		t.Fatalf("崩溃镜像 k=%d 重启失败: %v", k, err)
	}
	defer o.close()
	r3AssertLegal(t, fmt.Sprintf("k=%d", k), img, o)
	return o.p
}

// ── C4 · 提交点周围（最高优先级）───────────────────────────────────────────

func TestR3_C4_CommitBoundary(t *testing.T) {
	for _, newTipIsNew := range []bool{false, true} {
		img := r3BuildImage(t, newTipIsNew)
		L := len(img.delta)
		tipStart := L - img.tipFrameLen // TIP 帧起始偏移
		blkStart := tipStart            // variant A：TIP 之前全是 UNDO
		if img.newTipIsNew {
			blkStart = tipStart - img.blockFrameLen // BLOCK(f6) 起始
		}

		type tc struct {
			name string
			k    int
			want [32]byte
			h    int
			note string
		}
		cases := []tc{
			{"C4-0-提交点起始", 0, img.oldTip, r3OldTop, "提交尚未写入任何字节"},
			{"C4-1-首帧仅1字节", 1, img.oldTip, r3OldTop, "torn（须 REPAIR）"},
			{"C4-2-首帧部分载荷", frameHeaderSize + 10, img.oldTip, r3OldTop, "torn（须 REPAIR）"},
			{"C4-3-TIP之前-全部前置帧已提交", tipStart, img.oldTip, r3OldTop, "**最关键**：UNDO/BLOCK 齐全、TIP 未写"},
			{"C4-4-TIP帧头不完整", tipStart + 8, img.oldTip, r3OldTop, "torn（须 REPAIR）"},
			{"C4-5-TIP载荷不完整", tipStart + frameHeaderSize + 10, img.oldTip, r3OldTop, "torn（须 REPAIR）"},
			{"C4-6-缺最后1字节", L - 1, img.oldTip, r3OldTop, "torn（须 REPAIR）"},
			{"C4-7-完整提交", L, img.newTip, r3ForkTop, "提交完成"},
		}
		if img.newTipIsNew {
			cases = append(cases, tc{"C4-3b-新链尾BLOCK未写", blkStart, img.oldTip, r3OldTop, "connect 途中：BLOCK(f6) 未落盘"})
		}

		for _, c := range cases {
			id := fmt.Sprintf("%s/%s", c.name, img.variant)
			p := r3RunCase(t, img, id, img.r3Cut(c.k), c.want, c.h)
			// C4-3 必须落在**完整帧边界**上（无 REPAIR ⇒ 日志长度 == 镜像长度）
			if c.k == tipStart && p.logSize != int64(len(img.r3Cut(c.k))) {
				t.Fatalf("%s：TIP 起始偏移 %d 不是完整帧边界（LogSize=%d，镜像=%d）",
					id, tipStart, p.logSize, len(img.r3Cut(c.k)))
			}
			t.Logf("%s → 高度=%d 链尾=%x legacy=%d 恢复模式=%s 日志=%d 字节 | %s",
				id, p.height, p.tip[:4], p.legacyLen, p.recovery, p.logSize, c.note)
		}
	}
}

// TestR3_C4_ByteSweep 穷举提交点内的**每一个字节偏移**：
//   - 任何偏移都不得产生非法状态（I1–I8）；
//   - canonical 只允许发生**一次**跃变（旧 → 新），且必须恰好发生在 TIP 帧完整落盘处。
func TestR3_C4_ByteSweep(t *testing.T) {
	for _, newTipIsNew := range []bool{false, true} {
		img := r3BuildImage(t, newTipIsNew)
		swapDir := t.TempDir()
		prev := img.oldTip
		transitions := 0
		transitionAt := -1
		for k := 0; k <= len(img.delta); k++ {
			o, err := r3OpenIn(t, swapDir, img.r3Cut(k))
			if err != nil {
				t.Fatalf("[%s] k=%d 重启失败（canonical 必须可判定）: %v", img.variant, k, err)
			}
			r3AssertLegal(t, fmt.Sprintf("%s/k=%d", img.variant, k), img, o)
			if o.p.tip != prev {
				transitions++
				transitionAt = k
				prev = o.p.tip
			}
			if err := o.closeErr(); err != nil {
				t.Fatalf("k=%d 关闭失败: %v", k, err)
			}
		}
		if transitions != 1 {
			t.Fatalf("[%s] canonical 跃变次数 = %d, want 1（提交点必须唯一且确定）", img.variant, transitions)
		}
		if transitionAt != len(img.delta) {
			t.Fatalf("[%s] canonical 跃变发生在 k=%d, want %d（只有 TIP 完整落盘才可切换）",
				img.variant, transitionAt, len(img.delta))
		}
		if prev != img.newTip {
			t.Fatalf("[%s] 穷举结束后 canonical = %x, want 新链尾 %x", img.variant, prev, img.newTip)
		}
		t.Logf("[%s] 字节穷举 %d 个偏移：0..%d 全部保持旧 canonical，%d 处切换到新 canonical（= TIP 帧完整落盘）",
			img.variant, len(img.delta)+1, len(img.delta)-1, transitionAt)
	}
}

// ── C5 / C6 ────────────────────────────────────────────────────────────────

func TestR3_C5_AfterSwitchBeforeShutdown(t *testing.T) {
	for _, newTipIsNew := range []bool{false, true} {
		img := r3BuildImage(t, newTipIsNew)
		id := "C5/" + img.variant
		p := r3RunCase(t, img, id, img.full, img.newTip, r3ForkTop)
		if p.recovery != "REBUILD" {
			t.Fatalf("%s：恢复模式 = %s, want REBUILD（不应发生回滚）", id, p.recovery)
		}
		// 切换后：被排除的旧链 v2 区块必须仍存在（转 detached，不得丢失）
		chk, err := r3Open(t, img.full)
		if err != nil {
			t.Fatalf("%s：复核打开失败: %v", id, err)
		}
		for h := r3LegacyLen; h <= r3OldTop; h++ {
			if !chk.s.HasBlock(img.oldChain[h].Header.Hash()) {
				t.Fatalf("%s：被排除的旧 canonical 区块（高度 %d）丢失", id, h)
			}
		}
		if err := chk.closeErr(); err != nil {
			t.Fatalf("%s：关闭失败: %v", id, err)
		}
		t.Logf("%s → 高度=%d 链尾=%x detached=%d legacy=%d 恢复模式=%s",
			id, p.height, p.tip[:4], p.detached, p.legacyLen, p.recovery)
	}
}

func TestR3_C6_FullyCompletedAndGracefulClose(t *testing.T) {
	for _, newTipIsNew := range []bool{false, true} {
		img := r3BuildImage(t, newTipIsNew)
		id := "C6/" + img.variant
		o, err := r3Open(t, img.full)
		if err != nil {
			t.Fatalf("%s：打开失败: %v", id, err)
		}
		r3AssertLegal(t, id, img, o)
		if o.p.height != r3ForkTop || o.p.tip != img.newTip {
			t.Fatalf("%s：状态 = (h=%d tip=%x), want (h=%d tip=%x)", id, o.p.height, o.p.tip, r3ForkTop, img.newTip)
		}
		if err := o.closeErr(); err != nil {
			t.Fatalf("%s：优雅关闭失败: %v", id, err)
		}
		// 优雅关闭后重启：状态不变，字节不变
		o2, err := r3Open(t, iReadLog(t, o.dir))
		if err != nil {
			t.Fatalf("%s：重启失败: %v", id, err)
		}
		defer o2.close()
		if o2.p.sig() != o.p.sig() {
			t.Fatalf("%s：优雅关闭前后状态不一致\n  前 %s\n  后 %s", id, o.p.sig(), o2.p.sig())
		}
		if !bytes.Equal(o2.p.logBytes, o.p.logBytes) {
			t.Fatalf("%s：优雅关闭后字节被改写", id)
		}
	}
}

// ── I10 · 关键崩溃点重复执行 ≥ 5 次 ─────────────────────────────────────────

func TestR3_I10_KeyCrashPointsRepeatFiveTimes(t *testing.T) {
	const runs = 5
	for _, newTipIsNew := range []bool{false, true} {
		img := r3BuildImage(t, newTipIsNew)
		L := len(img.delta)
		tipStart := L - img.tipFrameLen

		key := []struct {
			name string
			k    int
			want [32]byte
			h    int
		}{
			{"C1-分支已抵达未切换", -1, img.oldTip, r3OldTop},                 // k=-1 → 用 base
			{"C4-3-TIP之前", tipStart, img.oldTip, r3OldTop},
			{"C4-7-完整提交", L, img.newTip, r3ForkTop},
			{"C5-已切换未关闭", L, img.newTip, r3ForkTop},
		}
		for _, cse := range key {
			var first *r3Probe
			for i := 0; i < runs; i++ {
				log := img.base
				if cse.k >= 0 {
					log = img.r3Cut(cse.k)
				}
				p := r3RunCase(t, img, fmt.Sprintf("%s/%s/run%d", cse.name, img.variant, i+1), log, cse.want, cse.h)
				if first == nil {
					first = p
					continue
				}
				if p.sig() != first.sig() {
					t.Fatalf("[%s/%s] I10 违反：第 %d 次运行与前次不一致\n  首轮 %s\n  本轮 %s",
						cse.name, img.variant, i+1, first.sig(), p.sig())
				}
			}
			t.Logf("[%s/%s] %d 次重复执行状态完全一致：%s", cse.name, img.variant, runs, first.sig())
		}
	}
}
