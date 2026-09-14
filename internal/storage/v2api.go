package storage

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"p2pchain/internal/block"
	"p2pchain/internal/utxo"
)

// ════════════════════════════════════════════════════════════════════════════
// REORG-1E · M3–M8 —— v2 严格 / 可回滚 API
//
// 与 legacy 的 SaveBlock 严格分工（Option A 决议）：
//
//   - SaveBlock        ：纯 legacy 文件上行为**逐字节不变**（既有测试 0 改动、
//                        生产字节 0 变化、I8 天然成立）。一旦该 store 进入 v2 模式
//                        （出现首条 v2 记录），SaveBlock 被**拒绝**并要求改用下述
//                        严格 API——这是「进入 v2 后启用严格校验」的最强形态：
//                        结构上不可能产出缺少 UNDO 的 canonical 区块（I1 由此天然成立）。
//   - SaveBlockDetached / SaveBlockWithUndo / PutUndo
//                      ：写入 v2 BLOCK / UNDO，**不提交** canonical（非 canonical）。
//   - CommitTip        ：唯一 canonical 提交点（M5）。
//   - AppendCanonicalBlock
//                      ：UNDO → BLOCK → TIP，**一次 fsync**（F7/§14 生产原语）。
//   - DeleteBlock / DeleteBranch / TruncateFromHeight
//                      ：仅逻辑删除；**绝不物理删除已提交字节**（I3/I4）。
// ════════════════════════════════════════════════════════════════════════════

// cwBytes 把累积工作量编码为 32 B 大端。
func cwBytes(cw *big.Int) ([32]byte, error) {
	var out [32]byte
	if cw == nil {
		return out, nil
	}
	if cw.Sign() < 0 || cw.BitLen() > 256 {
		return out, fmt.Errorf("%w: bitlen=%d", ErrChainworkOverflow, cw.BitLen())
	}
	cw.FillBytes(out[:])
	return out, nil
}

// registerBlock 把一枚已落盘的 v2 区块登记进统一索引（非 canonical，直到被 TIP 提交）。
func (s *FileBlockStore) registerBlock(b *block.Block, height int, cum *big.Int, offset int64) *blockRecord {
	rec := &blockRecord{
		hash: b.Header.Hash(), height: height, offset: offset, block: b,
		isV2: true, canonical: false, cumWork: cum, parent: b.Header.PrevBlockHash,
	}
	s.v2.records[rec.hash] = rec
	s.v2.v2Blocks++
	return rec
}

// commitTipAfterAppend 在 TIP 记录已写入并 fsync 后推进 canonical 状态。
func (s *FileBlockStore) commitTipAfterAppend(rec *blockRecord, tipOffset int64) error {
	if rec == nil {
		return fmt.Errorf("%w: nil tip", ErrTipBlockMissing)
	}
	if err := s.validateTipCandidate(tipRecord{hash: rec.hash, height: uint32(rec.height)}); err != nil {
		return err
	}
	if err := s.setCanonicalFrom(rec.hash); err != nil {
		return err
	}
	s.v2.v2Mode = true
	s.v2.tipHash, s.v2.tipHeight, s.v2.hasTip = rec.hash, rec.height, true
	s.v2.chainwork = new(big.Int).Set(rec.cumWork)
	cw, err := cwBytes(rec.cumWork)
	if err != nil {
		return err
	}
	s.pushRing(tipRecord{offset: tipOffset, hash: rec.hash, height: uint32(rec.height), cw: cw})
	return nil
}

// currentTipRec 返回当前 canonical 链尾对应的记录。
func (s *FileBlockStore) currentTipRec() (*blockRecord, error) {
	if len(s.byHeight) == 0 {
		return nil, fmt.Errorf("%w: 空链无链尾", ErrBlockNotFound)
	}
	var h [32]byte
	if s.v2.hasTip {
		h = s.v2.tipHash
	} else {
		h = s.byHeight[len(s.byHeight)-1].Header.Hash()
	}
	rec, ok := s.v2.records[h]
	if !ok {
		return nil, fmt.Errorf("%w: %x", ErrTipBlockMissing, h)
	}
	return rec, nil
}

// readFramePayloadAt 从日志中读出偏移处的一枚帧载荷，并校验类型与区块哈希。
func (s *FileBlockStore) readFramePayloadAt(offset int64, wantType byte, wantHash [32]byte) ([]byte, error) {
	f := s.file
	if f == nil {
		f = s.rfile
	}
	if f == nil {
		return nil, fmt.Errorf("%w: 无可用文件句柄", ErrCorruptStore)
	}
	head := make([]byte, frameHeaderSize)
	if _, err := f.ReadAt(head, offset); err != nil {
		return nil, fmt.Errorf("%w: 读取帧头失败: %v", ErrCorruptStore, err)
	}
	if !hasFrameMagic(head) {
		return nil, fmt.Errorf("%w: %v", ErrCorruptStore, ErrBadRecordMagic)
	}
	n := binary.LittleEndian.Uint32(head[8:12])
	if n == 0 || n > maxPayloadLen {
		return nil, fmt.Errorf("%w: %v", ErrCorruptStore, ErrInvalidPayloadLen)
	}
	full := make([]byte, int64(frameOverhead)+int64(n))
	if _, err := f.ReadAt(full, offset); err != nil {
		return nil, fmt.Errorf("%w: 读取帧体失败: %v", ErrCorruptStore, err)
	}
	fr, err := decodeFrame(full)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorruptStore, err)
	}
	if fr.Type != wantType {
		return nil, fmt.Errorf("%w: 帧类型 %s != %s", ErrCorruptStore, recTypeName(fr.Type), recTypeName(wantType))
	}
	if fr.BlockHash != wantHash {
		return nil, fmt.Errorf("%w: 帧 blockHash 不匹配", ErrCorruptStore)
	}
	return fr.Payload, nil
}

// ── 写入：非 canonical ────────────────────────────────────────────────────

// SaveBlockDetached 写入一枚 v2 BLOCK（父存在即可，允许非规范块），**不提交** canonical。
//
// 键值为「detached branch 必须能跨重启存活」（I3）：区块字节永久保留在日志中，
// 是否 canonical 完全由 TIP 决定。
func (s *FileBlockStore) SaveBlockDetached(b *block.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireWritable(); err != nil {
		return err
	}
	h, cum, err := s.deriveStrict(b)
	if err != nil {
		return err
	}
	hash := b.Header.Hash()
	frame := encodeFrame(recTypeBlock, uint32(h), hash, b.Encode())
	offs, err := s.appendFrames(frame)
	if err != nil {
		return err
	}
	s.v2.v2Mode = true
	s.registerBlock(b, h, cum, offs[0])
	return nil
}

// SaveBlockWithUndo 写入 UNDO + BLOCK（UNDO→BLOCK 顺序），**不提交** canonical。
// 单次 fsync。提交需另行调用 CommitTip。
func (s *FileBlockStore) SaveBlockWithUndo(b *block.Block, undo utxo.BlockUndo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireWritable(); err != nil {
		return err
	}
	h, cum, err := s.deriveStrict(b)
	if err != nil {
		return err
	}
	if undo.Height != h {
		return fmt.Errorf("%w: undo.Height=%d 派生 height=%d", ErrUndoBinding, undo.Height, h)
	}
	if _, dup := s.v2.undoIndex[b.Header.Hash()]; dup {
		return fmt.Errorf("%w: %x", ErrDuplicateUndo, b.Header.Hash())
	}
	ub, err := utxo.EncodeUndo(undo)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUndoBinding, err)
	}
	hash := b.Header.Hash()
	undoFrame := encodeFrame(recTypeUndo, uint32(h), hash, ub)
	blockFrame := encodeFrame(recTypeBlock, uint32(h), hash, b.Encode())
	offs, err := s.appendFrames(undoFrame, blockFrame) // UNDO → BLOCK，单次 fsync
	if err != nil {
		return err
	}
	s.v2.v2Mode = true
	s.registerBlock(b, h, cum, offs[1])
	s.v2.undoIndex[hash] = undoRecord{blockHash: hash, height: uint32(h), offset: offs[0]}
	return nil
}

// PutUndo 为已存在的 v2 区块补写一条 UNDO 帧（UNDO 是可重建的派生缓存，F3）。
func (s *FileBlockStore) PutUndo(blockHash [32]byte, height int, undo utxo.BlockUndo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireWritable(); err != nil {
		return err
	}
	rec, ok := s.v2.records[blockHash]
	if !ok {
		return fmt.Errorf("%w: %x", ErrBlockNotFound, blockHash)
	}
	if rec.height != height || undo.Height != height {
		return fmt.Errorf("%w: rec.height=%d undo.Height=%d want=%d", ErrUndoBinding, rec.height, undo.Height, height)
	}
	if _, dup := s.v2.undoIndex[blockHash]; dup {
		return fmt.Errorf("%w: %x", ErrDuplicateUndo, blockHash)
	}
	ub, err := utxo.EncodeUndo(undo)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUndoBinding, err)
	}
	frame := encodeFrame(recTypeUndo, uint32(height), blockHash, ub)
	offs, err := s.appendFrames(frame)
	if err != nil {
		return err
	}
	s.v2.v2Mode = true
	s.v2.undoIndex[blockHash] = undoRecord{blockHash: blockHash, height: uint32(height), offset: offs[0]}
	return nil
}

// CommitTip 追加 TIP 记录——canonical 状态的**唯一**提交点（M5）。
func (s *FileBlockStore) CommitTip(hash [32]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireWritable(); err != nil {
		return err
	}
	rec, ok := s.v2.records[hash]
	if !ok {
		return fmt.Errorf("%w: %x", ErrTipBlockMissing, hash)
	}
	tipFrame, err := s.tipFrameFor(rec)
	if err != nil {
		return err
	}
	offs, err := s.appendFrames(tipFrame)
	if err != nil {
		return err
	}
	return s.commitTipAfterAppend(rec, offs[0])
}

// AppendCanonicalBlock 是 reorg-aware 的生产原语：
// 以 **UNDO → BLOCK → TIP 顺序、一次 fsync** 原子追加一枚 canonical 区块（F4/F7）。
func (s *FileBlockStore) AppendCanonicalBlock(b *block.Block, undo utxo.BlockUndo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireWritable(); err != nil {
		return err
	}
	h, cum, err := s.deriveStrict(b)
	if err != nil {
		return err
	}
	if undo.Height != h {
		return fmt.Errorf("%w: undo.Height=%d 派生 height=%d", ErrUndoBinding, undo.Height, h)
	}
	if _, dup := s.v2.undoIndex[b.Header.Hash()]; dup {
		return fmt.Errorf("%w: %x", ErrDuplicateUndo, b.Header.Hash())
	}
	ub, err := utxo.EncodeUndo(undo)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUndoBinding, err)
	}
	hash := b.Header.Hash()
	undoFrame := encodeFrame(recTypeUndo, uint32(h), hash, ub)
	blockFrame := encodeFrame(recTypeBlock, uint32(h), hash, b.Encode())
	tipPayload, err := encodeTipPayload(cum, h)
	if err != nil {
		return err
	}
	tipFrame := encodeFrame(recTypeTip, uint32(h), hash, tipPayload)
	offs, err := s.appendFrames(undoFrame, blockFrame, tipFrame) // 单次 fsync
	if err != nil {
		return err
	}
	rec := s.registerBlock(b, h, cum, offs[1])
	s.v2.undoIndex[hash] = undoRecord{blockHash: hash, height: uint32(h), offset: offs[0]}
	return s.commitTipAfterAppend(rec, offs[2])
}

// ── 写入：逻辑删除 / 回退（永不物理删除已提交字节）───────────────────────

// DeleteBlock 逻辑删除一枚区块（墓碑 + 追加 TIP）。
//
// 允许对象：detached 的 v2 区块，或 canonical 链尾（删除后 canonical 回退到其父）。
// 禁止：legacy 区块（不可变历史前缀）、canonical 非链尾区块（会孤立其后代，须用
// TruncateFromHeight）。
func (s *FileBlockStore) DeleteBlock(hash [32]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireWritable(); err != nil {
		return err
	}
	rec, ok := s.v2.records[hash]
	if !ok {
		return fmt.Errorf("%w: %x", ErrBlockNotFound, hash)
	}
	if !rec.isV2 {
		return fmt.Errorf("%w: %x", ErrLegacyImmutable, hash)
	}
	if rec.deleted {
		return nil // 幂等
	}
	target, err := s.currentTipRec()
	if err != nil {
		return err
	}
	if rec.canonical {
		if !s.v2.hasTip || rec.hash != s.v2.tipHash {
			return fmt.Errorf("%w: %x", ErrCannotDeleteCanonical, hash)
		}
		p, ok := s.v2.records[rec.parent]
		if !ok {
			return fmt.Errorf("%w: %x", ErrParentNotFound, rec.parent)
		}
		target = p
	}
	delFrame := encodeFrame(recTypeDelete, uint32(rec.height), rec.hash, rec.hash[:])
	tipFrame, err := s.tipFrameFor(target)
	if err != nil {
		return err
	}
	offs, err := s.appendFrames(delFrame, tipFrame) // 墓碑 → TIP，单次 fsync
	if err != nil {
		return err
	}
	rec.deleted = true
	return s.commitTipAfterAppend(target, offs[1])
}

// DeleteBranch 逻辑删除 fromHash 及其全部 v2 后代（仅允许 detached 分支）。
// 返回被本次标记删除的区块数（已删除者不计）。
func (s *FileBlockStore) DeleteBranch(fromHash [32]byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireWritable(); err != nil {
		return 0, err
	}
	root, ok := s.v2.records[fromHash]
	if !ok {
		return 0, fmt.Errorf("%w: %x", ErrBlockNotFound, fromHash)
	}
	if !root.isV2 {
		return 0, fmt.Errorf("%w: %x", ErrLegacyImmutable, fromHash)
	}
	if root.canonical {
		return 0, fmt.Errorf("%w: %x", ErrCannotDeleteCanonical, fromHash)
	}
	// BFS 收集分支（v2 记录数受日志体积约束）
	set := map[[32]byte]*blockRecord{root.hash: root}
	queue := []*blockRecord{root}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, r := range s.v2.records {
			if !r.isV2 || r.parent != cur.hash {
				continue
			}
			if _, seen := set[r.hash]; !seen {
				set[r.hash] = r
				queue = append(queue, r)
			}
		}
	}
	// 按高度升序（确定性）
	list := make([]*blockRecord, 0, len(set))
	for _, r := range set {
		list = append(list, r)
	}
	for i := 1; i < len(list); i++ {
		key := list[i]
		j := i - 1
		for j >= 0 && list[j].height > key.height {
			list[j+1] = list[j]
			j--
		}
		list[j+1] = key
	}
	var frames [][]byte
	fresh := make([]*blockRecord, 0, len(list))
	for _, r := range list {
		if r.deleted {
			continue
		}
		frames = append(frames, encodeFrame(recTypeDelete, uint32(r.height), r.hash, r.hash[:]))
		fresh = append(fresh, r)
	}
	if len(fresh) == 0 {
		return 0, nil
	}
	target, err := s.currentTipRec()
	if err != nil {
		return 0, err
	}
	tipFrame, err := s.tipFrameFor(target)
	if err != nil {
		return 0, err
	}
	frames = append(frames, tipFrame)
	offs, err := s.appendFrames(frames...) // 墓碑批次 → TIP，单次 fsync
	if err != nil {
		return 0, err
	}
	for _, r := range fresh {
		r.deleted = true
	}
	if err := s.commitTipAfterAppend(target, offs[len(offs)-1]); err != nil {
		return len(fresh), err
	}
	return len(fresh), nil
}

// TruncateFromHeight 逻辑回退 canonical 链：保留高度 [0, h-1]，新区块高度为 h-1。
//
// 流程（M6）：构建待删除 canonical 集合 → 完整验证 → 构建新 canonical 状态 →
// append TIP → 单次 fsync → commit。
//
// **绝不使用 os.Truncate 处理已提交 canonical 数据**：被回退的区块字节永久保留，
// 仅因不再位于新 tip 的祖辈路径而转为 detached（I3/I4）。
func (s *FileBlockStore) TruncateFromHeight(h int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireWritable(); err != nil {
		return err
	}
	if !s.v2.v2Mode {
		return fmt.Errorf("%w: TruncateFromHeight", ErrNotV2Mode)
	}
	top := len(s.byHeight) - 1
	if top < 0 {
		return fmt.Errorf("%w: 空链", ErrTruncateOutOfRange)
	}
	if h < 1 || h > top {
		return fmt.Errorf("%w: h=%d, canonical tip=%d（须 1 ≤ h ≤ tip）", ErrTruncateOutOfRange, h, top)
	}
	// 不得截入 legacy 不可变前缀
	if s.v2.legacyLen > 0 && h < s.v2.legacyLen {
		return fmt.Errorf("%w: h=%d 会截入 legacy 前缀（末尾高度 %d）", ErrTruncateOutOfRange, h, s.v2.legacyLen-1)
	}
	// 1) 构建并完整验证待删除集合
	for i := h; i <= top; i++ {
		hash := s.byHeight[i].Header.Hash()
		if _, ok := s.v2.records[hash]; !ok {
			return fmt.Errorf("%w: 待删除高度 %d 记录缺失 %x", ErrBlockNotFound, i, hash)
		}
	}
	// 2) 新 canonical 状态 = 高度 h-1 的区块
	newTipBlock := s.byHeight[h-1]
	newTip, ok := s.v2.records[newTipBlock.Header.Hash()]
	if !ok {
		return fmt.Errorf("%w: %x", ErrTipBlockMissing, newTipBlock.Header.Hash())
	}
	tipFrame, err := s.tipFrameFor(newTip)
	if err != nil {
		return err
	}
	// 3) 追加 TIP 并单次 fsync（不物理删除任何字节）
	offs, err := s.appendFrames(tipFrame)
	if err != nil {
		return err
	}
	return s.commitTipAfterAppend(newTip, offs[0])
}

// ── 只读查询 ──────────────────────────────────────────────────────────────

// V2Mode 报告存储是否已进入 v2 语义（出现首条 v2 记录后为 true）。
func (s *FileBlockStore) V2Mode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.v2.v2Mode
}

// LegacyRecordCount 返回 legacy 记录数（高度 0..n-1）。
func (s *FileBlockStore) LegacyRecordCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.v2.legacyLen
}

// RecoveryMode 返回最近一次加载采用的恢复模式（REBUILD / ROLLBACK）。
func (s *FileBlockStore) RecoveryMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.v2.recoveryMode
}

// DanglingUndoCount 返回扫描中遇到并忽略的悬空 UNDO 帧数（崩溃残留证据）。
func (s *FileBlockStore) DanglingUndoCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.v2.danglingUndo
}

// TipRingLen 返回会话内保留的 TIP 恢复候选数（上限 K=8）。
func (s *FileBlockStore) TipRingLen() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.v2.tipRing)
}

// LogSize 返回日志的物理字节长度（append 定位基准）。
func (s *FileBlockStore) LogSize() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.v2.logSize
}

// TipHash 返回 canonical 链尾哈希；hasTip 报告是否由 TIP 记录提交。
func (s *FileBlockStore) TipHash() ([32]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.v2.tipHash, s.v2.hasTip
}

// Chainwork 返回 canonical 链的累积工作量（*big.Int）；ok=false 表示尚未确定。
func (s *FileBlockStore) Chainwork() (*big.Int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.v2.chainwork == nil {
		return nil, false
	}
	return new(big.Int).Set(s.v2.chainwork), true
}

// HasBlock 报告哈希是否存在于存储（含 detached；不含被逻辑删除者）。
func (s *FileBlockStore) HasBlock(hash [32]byte) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.v2.records[hash]
	return ok && !rec.deleted
}

// RecordCount 返回物理区块记录数（legacy 记录数 + v2 BLOCK 记录数，含 detached
// 与被逻辑删除者——逻辑删除不减少物理记录，见 I4）。
func (s *FileBlockStore) RecordCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.v2.legacyLen + s.v2.v2Blocks
}

// DetachedCount 返回非 canonical 的 v2 区块数（detached branch 规模）。
func (s *FileBlockStore) DetachedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, r := range s.v2.records {
		if r.isV2 && !r.canonical {
			n++
		}
	}
	return n
}

// IsCanonical 报告区块是否位于 canonical 路径（含 legacy 前缀）。
func (s *FileBlockStore) IsCanonical(hash [32]byte) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.v2.records[hash]
	if !ok {
		return false, fmt.Errorf("%w: %x", ErrBlockNotFound, hash)
	}
	return rec.canonical, nil
}

// IsDeleted 报告区块是否被生效的逻辑删除墓碑标记。
func (s *FileBlockStore) IsDeleted(hash [32]byte) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.v2.records[hash]
	if !ok {
		return false, fmt.Errorf("%w: %x", ErrBlockNotFound, hash)
	}
	return rec.deleted, nil
}

// HasUndo 报告是否存在该区块的 UNDO 帧。
func (s *FileBlockStore) HasUndo(hash [32]byte) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.v2.undoIndex[hash]
	return ok
}

// UndoFor 读取并校验该区块的 UNDO（M4 三重绑定：帧 blockHash / 帧 height / undo.Height）。
// 损坏的 UNDO **永不**被返回。
func (s *FileBlockStore) UndoFor(hash [32]byte) (utxo.BlockUndo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ur, ok := s.v2.undoIndex[hash]
	if !ok {
		return utxo.BlockUndo{}, fmt.Errorf("%w: %x", ErrUndoNotFound, hash)
	}
	payload, err := s.readFramePayloadAt(ur.offset, recTypeUndo, hash)
	if err != nil {
		return utxo.BlockUndo{}, err
	}
	undo, err := utxo.DecodeUndo(payload)
	if err != nil {
		return utxo.BlockUndo{}, fmt.Errorf("%w: %v", ErrUndoBinding, err)
	}
	if undo.Height != int(ur.height) {
		return utxo.BlockUndo{}, fmt.Errorf("%w: undo.Height=%d 帧 height=%d", ErrUndoBinding, undo.Height, ur.height)
	}
	return undo, nil
}
