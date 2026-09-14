package storage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"os"

	"p2pchain/internal/block"
	"p2pchain/internal/utxo"
)

// ════════════════════════════════════════════════════════════════════════════
// REORG-1E · M2–M8 —— v2 语义 / 崩溃恢复 / 逻辑删除 / 严格校验
//
// 语义分区（M2 冻结）：
//
//   - **legacy 区** = 日志最前部连续的 `[u32 LE len][block.Encode()]` 记录，
//     被视为 **committed canonical historical prefix**（F 契约）。高度 = 记录序号，
//     哈希 = SHA256(decoded block)。该区**只读不改**：任何字节都不被迁移/重写/截断。
//   - **v2 区** = 首条 v2 帧之后的部分。canonical 状态**只能**由 TIP 记录推进（M5），
//     区块以哈希寻址、可 detached、可逻辑删除。
//
// 单一提交点（M5/F4）：UNDO → BLOCK → TIP，TIP 是唯一 canonical commit point。
//
// 崩溃恢复（M7）：解析 fail-stop；只读路径**严格拒绝**任何损坏；可写路径仅当
// 尾部是「位于 EOF 的截断残片」且能证明其未被提交时才 REPAIR（物理截断残留字节），
// 其余一律 REJECT。完整但语义失效的 TIP 走逻辑 ROLLBACK（不删字节）。
// ════════════════════════════════════════════════════════════════════════════

// ── v2 具名错误 ────────────────────────────────────────────────────────────

var (
	// ErrInvalidBits bits 超出 [1,256]，工作量无法定义。
	ErrInvalidBits = errors.New("bits 超出 [1,256] 范围")
	// ErrDuplicateBlock 区块哈希已存在（M3 重复哈希拒绝）。
	ErrDuplicateBlock = errors.New("区块哈希已存在（重复记录被拒绝）")
	// ErrParentNotFound 父区块不在存储中（M8）。
	ErrParentNotFound = errors.New("父区块不存在")
	// ErrInvalidHeight 区块高度与父高度不连续（M8 / SP-3）。
	ErrInvalidHeight = errors.New("区块高度与父高度不连续（要求 height == parent.Height + 1）")
	// ErrZeroParentHash 零父哈希区块出现在非创世位置（M8 / SP-3b）。
	ErrZeroParentHash = errors.New("零父哈希区块只允许出现在创世位置")
	// ErrLegacyAppendInV2Mode 进入 v2 模式后禁用 legacy 追加口。
	ErrLegacyAppendInV2Mode = errors.New("存储已进入 v2 模式：请使用 AppendCanonicalBlock 或 SaveBlockWithUndo + CommitTip")
	// ErrInterleavedLegacy legacy 记录出现在 v2 记录之后（语义区交错，拒绝）。
	ErrInterleavedLegacy = errors.New("legacy 记录出现在 v2 记录之后：语义区不得交错")
	// ErrTipBlockMissing TIP 引用的区块不存在。
	ErrTipBlockMissing = errors.New("TIP 引用的区块不存在")
	// ErrTipHeightMismatch TIP 声明高度与其引用区块不一致。
	ErrTipHeightMismatch = errors.New("TIP 高度与其引用区块不一致")
	// ErrTipUndoMissing canonical v2 区块缺少 UNDO（违反 I1）。
	ErrTipUndoMissing = errors.New("canonical 区块缺少 UNDO（违反不变量 I1）")
	// ErrTipRingExhausted TIP 恢复候选（最近 K=8 个）全部失效 → 拒绝启动。
	ErrTipRingExhausted = errors.New("TIP 恢复候选耗尽：无法确定唯一 canonical 状态")
	// ErrBlockNotFound 目标区块不存在。
	ErrBlockNotFound = errors.New("区块不存在")
	// ErrLegacyImmutable legacy 区为不可变已提交历史前缀。
	ErrLegacyImmutable = errors.New("legacy 区块属于不可变已提交历史前缀，不可删除")
	// ErrCannotDeleteCanonical 禁止删除 canonical 分支（须用 TruncateFromHeight）。
	ErrCannotDeleteCanonical = errors.New("不得删除 canonical 分支（请使用 TruncateFromHeight）")
	// ErrTruncateOutOfRange 截断高度越界或不需要截断。
	ErrTruncateOutOfRange = errors.New("截断高度超出允许范围")
	// ErrUndoNotFound 未找到该区块的 UNDO。
	ErrUndoNotFound = errors.New("UNDO 不存在")
	// ErrUndoBinding UNDO 与 blockHash / height 三重绑定校验失败。
	ErrUndoBinding = errors.New("UNDO 绑定校验失败（blockHash/height/checksum 不一致）")
	// ErrDuplicateUndo 该区块已存在 UNDO 帧。
	ErrDuplicateUndo = errors.New("UNDO 重复（该区块已有 UNDO）")
	// ErrNotV2Mode 该操作要求存储已处于 v2 语义。
	ErrNotV2Mode = errors.New("存储尚未进入 v2 模式：该操作需要 v2 语义")
)

// tipRingSize K=8：会话内保留的 TIP 恢复候选数（M5）。
const tipRingSize = 8

// ── 数据模型 ───────────────────────────────────────────────────────────────

// blockRecord 统一索引项：legacy 区块与 v2 BLOCK 记录共用。
type blockRecord struct {
	hash      [32]byte
	height    int
	offset    int64 // 记录（legacy 长度前缀 / v2 帧起始）偏移
	block     *block.Block
	isV2      bool // true = v2 BLOCK 帧；false = legacy 记录
	canonical bool // 是否在 canonical（tip 祖辈）路径上
	deleted   bool // 是否被生效的逻辑删除墓碑标记
	cumWork   *big.Int
	parent    [32]byte
}

// undoRecord v2 UNDO 帧索引项（blockHash 寻址，M4）。
type undoRecord struct {
	blockHash [32]byte
	height    uint32
	offset    int64
}

// tipRecord 一条完整有效 TIP 记录的会话内表示（M5）。
type tipRecord struct {
	offset int64
	hash   [32]byte
	height uint32
	cw     [32]byte
}

// v2State 承载 REORG-1E 引入的全部 v2 语义状态；与 legacy 视图（byHeight/byHash）解耦。
type v2State struct {
	v2Mode    bool
	records   map[[32]byte]*blockRecord // 全部区块（legacy + v2），统一哈希索引
	undoIndex map[[32]byte]undoRecord   // blockHash → UNDO 帧
	tipRing   []tipRecord               // 最近 ≤8 个完整有效 TIP
	tipHash   [32]byte
	tipHeight int
	hasTip    bool
	chainwork *big.Int
	// legacySeq 是 legacy 区的**记录序列**（高度 = 序号，允许重复哈希）。
	// 必须与 records 分开保存：records 以哈希去重，无法表达 legacy 的
	// 「同一区块占据两个高度」这一历史形态（既有 P0 测试即该形态）。
	legacySeq    []*block.Block
	legacyLen    int      // = len(legacySeq)
	legacyCum    *big.Int // legacy 前缀的累积工作量
	v2Blocks     int      // 已落盘的 v2 BLOCK 记录数（物理记录统计用）
	danglingUndo int      // 未被任何 BLOCK 引用的悬空 UNDO 帧数（崩溃残留，已忽略）
	logSize      int64    // 日志物理长度（append 定位用）
	recoveryMode string   // 最近一次加载采用的恢复模式（诊断/证据）
}

func (s *FileBlockStore) initV2() {
	s.v2 = v2State{
		records:   make(map[[32]byte]*blockRecord),
		undoIndex: make(map[[32]byte]undoRecord),
		chainwork: big.NewInt(0),
		legacyCum: big.NewInt(0),
	}
}

// workOfBits 返回 bits 对应的严格工作量 2^bits。
//
// 与 blocktree.WorkOfBits 同式（POW-WORK 审计冻结的权威口径：本链 bits 为前导零
// 位数，期望哈希数 = 2^bits，精确无舍入）。**有意不 import internal/blocktree**：
// blocktree 目前零生产导入者（REORG OFF 三重证明之二），若 storage（生产包）导入
// 它将使该证明失效。此处仅一行算术恒等式，并由 v2_test.go 与 blocktree.WorkOfBits
// 交叉比对锁定二者等价。
func workOfBits(bits uint32) (*big.Int, error) {
	if bits == 0 || bits > 256 {
		return nil, fmt.Errorf("%w: bits=%d", ErrInvalidBits, bits)
	}
	return new(big.Int).Lsh(big.NewInt(1), uint(bits)), nil
}

// ── 扫描（fail-stop，绝不越过损坏点重新同步）───────────────────────────────

type tornKind int

const (
	tornNone      tornKind = iota // 无残留
	tornTruncated                 // 文件在记录中途结束（EOF 截断）——潜在可修复
	tornMalformed                 // 内容畸形/校验和不符 —— 一律 REJECT
)

// rawRecord 扫描阶段收集的「完整且通过初步校验」的一条记录。
type rawRecord struct {
	offset int64
	legacy *block.Block // 非 nil = legacy 记录
	frame  frameInfo    // legacy == nil 时为 v2 帧
}

// logScan 一次日志扫描的确定性结果。
type logScan struct {
	records      []rawRecord
	torn         tornKind
	tornOffset   int64
	tornErr      error
	fileSize     int64
	committedEnd int64 // 最后一条完整记录之后的偏移
	hasV2Frame   bool  // 已出现至少一条完整 v2 帧
	sawMagic     bool  // 尾部残片起始即 v2 魔数
}

// frameRead readFrameAt 的结果。
type frameRead struct {
	frame   frameInfo
	total   int
	torn    tornKind
	tornErr error
	fatal   error
}

// readFrameAt 在偏移 pos 处尝试读取一枚 v2 帧。
func readFrameAt(f *os.File, pos, size int64) frameRead {
	remaining := size - pos
	if remaining < frameHeaderSize {
		return frameRead{torn: tornTruncated, tornErr: fmt.Errorf("%w: 仅剩 %d 字节", ErrTruncatedHeader, remaining)}
	}
	head := make([]byte, frameHeaderSize)
	if _, err := f.ReadAt(head, pos); err != nil {
		return frameRead{fatal: fmt.Errorf("读取帧头失败@%d: %w", pos, err)}
	}
	if !hasFrameMagic(head) {
		return frameRead{torn: tornMalformed, tornErr: ErrBadRecordMagic}
	}
	if head[4] != frameVersion {
		return frameRead{torn: tornMalformed, tornErr: fmt.Errorf("%w: got %d", ErrUnsupportedRecordVersion, head[4])}
	}
	if !validRecType(head[5]) {
		return frameRead{torn: tornMalformed, tornErr: fmt.Errorf("%w: %d", ErrInvalidRecordType, head[5])}
	}
	if binary.LittleEndian.Uint16(head[6:8]) != 0 {
		return frameRead{torn: tornMalformed, tornErr: ErrNonZeroReserved}
	}
	n := binary.LittleEndian.Uint32(head[8:12])
	if n == 0 || n > maxPayloadLen {
		return frameRead{torn: tornMalformed, tornErr: fmt.Errorf("%w: %d", ErrInvalidPayloadLen, n)}
	}
	need := int64(frameOverhead) + int64(n)
	if remaining < need {
		if remaining < int64(frameHeaderSize)+int64(n) {
			return frameRead{torn: tornTruncated, tornErr: ErrTruncatedPayload}
		}
		return frameRead{torn: tornTruncated, tornErr: ErrMissingChecksum}
	}
	full := make([]byte, need)
	if _, err := f.ReadAt(full, pos); err != nil {
		return frameRead{fatal: fmt.Errorf("读取帧体失败@%d: %w", pos, err)}
	}
	fr, err := decodeFrame(full)
	if err != nil {
		return frameRead{torn: tornMalformed, tornErr: err}
	}
	return frameRead{frame: fr, total: int(need)}
}

// scanLog 顺序扫描日志，遇首个不完整/非法记录即 fail-stop（不尝试越过重同步）。
func scanLog(f *os.File, size int64) (*logScan, error) {
	sc := &logScan{fileSize: size, tornOffset: size, committedEnd: 0}
	var pos int64
	for pos < size {
		remaining := size - pos
		if remaining < 4 {
			sc.torn, sc.tornOffset = tornTruncated, pos
			sc.tornErr = fmt.Errorf("%w: 剩余 %d 字节不足以构成记录头", ErrCorruptStore, remaining)
			return sc, nil
		}
		var head [4]byte
		if _, err := f.ReadAt(head[:], pos); err != nil {
			return nil, fmt.Errorf("读取记录头失败@%d: %w", pos, err)
		}
		if hasFrameMagic(head[:]) {
			sc.sawMagic = true
			fr := readFrameAt(f, pos, size)
			if fr.fatal != nil {
				return nil, fr.fatal
			}
			if fr.torn != tornNone {
				sc.torn, sc.tornOffset, sc.tornErr = fr.torn, pos, fr.tornErr
				return sc, nil
			}
			sc.records = append(sc.records, rawRecord{offset: pos, frame: fr.frame})
			sc.hasV2Frame = true
			pos += int64(fr.total)
			sc.committedEnd = pos
			continue
		}
		// legacy 记录
		n := binary.LittleEndian.Uint32(head[:])
		if n == 0 || n > maxPayloadLen {
			sc.torn, sc.tornOffset = tornMalformed, pos
			sc.tornErr = fmt.Errorf("%w: 非法 legacy 记录长度 %d", ErrCorruptStore, n)
			return sc, nil
		}
		if uint64(remaining) < 4+uint64(n) {
			sc.torn, sc.tornOffset = tornTruncated, pos
			sc.tornErr = fmt.Errorf("%w: legacy 记录数据不完整", ErrCorruptStore)
			return sc, nil
		}
		buf := make([]byte, n)
		if _, err := f.ReadAt(buf, pos+4); err != nil {
			return nil, fmt.Errorf("读取 legacy 记录失败@%d: %w", pos, err)
		}
		b, err := block.DecodeBlock(buf)
		if err != nil {
			sc.torn, sc.tornOffset = tornMalformed, pos
			sc.tornErr = fmt.Errorf("%w: legacy 区块解码失败: %v", ErrCorruptStore, err)
			return sc, nil
		}
		sc.records = append(sc.records, rawRecord{offset: pos, legacy: b})
		pos += 4 + int64(n)
		sc.committedEnd = pos
	}
	return sc, nil
}

// ── 加载 / 恢复 ────────────────────────────────────────────────────────────

// loadLog 扫描并解释日志；执行 M7 的 REJECT / REPAIR / ROLLBACK / REBUILD 决策。
func (s *FileBlockStore) loadLog(f *os.File) error {
	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("读取区块数据文件状态失败: %w", err)
	}
	sc, err := scanLog(f, fi.Size())
	if err != nil {
		return err
	}
	if sc.torn != tornNone {
		if s.readOnly {
			return fmt.Errorf("%w: %v", ErrCorruptStore, sc.tornErr)
		}
		repairable := sc.torn == tornTruncated && (sc.sawMagic || sc.hasV2Frame)
		if !repairable {
			return fmt.Errorf("%w: %v", ErrCorruptStore, sc.tornErr)
		}
		// REPAIR：物理截断未提交尾部（仅限位于 EOF 的截断残片）。
		//
		// 必须用**独立的非 O_APPEND 句柄**执行截断：Windows 上以 O_APPEND 打开的
		// 句柄只持有 FILE_APPEND_DATA，SetEndOfFile 会返回 Access denied。
		// 该句柄只在此处短暂存在，之后由主（append）句柄继续写入。
		tf, terr := os.OpenFile(s.path, os.O_RDWR, 0o600)
		if terr != nil {
			return fmt.Errorf("打开修复句柄失败: %w", terr)
		}
		if terr := tf.Truncate(sc.committedEnd); terr != nil {
			tf.Close()
			return fmt.Errorf("修复未提交尾部失败: %w", terr)
		}
		if terr := tf.Sync(); terr != nil {
			tf.Close()
			return fmt.Errorf("修复未提交尾部刷盘失败: %w", terr)
		}
		if terr := tf.Close(); terr != nil {
			return fmt.Errorf("关闭修复句柄失败: %w", terr)
		}
		sc.fileSize, sc.torn, sc.tornOffset = sc.committedEnd, tornNone, sc.committedEnd
	}
	return s.interpretScan(sc)
}

// interpretScan 把扫描结果解释为确定性的内存状态（I5：index = deterministic(log scan)）。
//
// **两遍解释**（关键）：
//
//	第一遍：legacy 前缀 / v2 BLOCK / TIP / DELETE 按文件顺序处理（父先于子，
//	        故高度可由父链确定性派生）；UNDO 帧先收集不处理。
//	第二遍：处理 UNDO 帧——此时全部 BLOCK 已登记，绑定校验可完成。
//
// 之所以分两遍：canonical 写入顺序是 UNDO → BLOCK → TIP，崩溃可能在「UNDO 完整、
// BLOCK 缺失」处停下（M7 crash matrix #4）。这类 **dangling UNDO 属于未提交记录，
// 必须被容忍（忽略），不得导致启动失败**；而单遍处理会因「UNDO 引用的区块尚不存在」
// 误判为损坏。反之，若 UNDO 对应区块在整份日志中都不存在，则是悬空记录 → 忽略。
func (s *FileBlockStore) interpretScan(sc *logScan) error {
	s.v2.logSize = sc.committedEnd

	type rawUndo struct {
		fr     frameInfo
		offset int64
	}

	// —— 第一遍 ——
	var tips []tipRecord
	var pendingUndo []rawUndo
	var pendingDel [][32]byte
	committedDel := make(map[[32]byte]bool)
	seenV2 := false

	for _, rr := range sc.records {
		if rr.legacy != nil {
			if seenV2 {
				return fmt.Errorf("%w: 偏移 %d", ErrCorruptStore, rr.offset)
			}
			b := rr.legacy
			h := len(s.v2.legacySeq)
			hash := b.Header.Hash()
			w, err := workOfBits(b.Header.Bits)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrCorruptStore, err)
			}
			cum := new(big.Int).Add(s.v2.legacyCum, w)
			s.v2.legacyCum = cum
			// legacy 语义：不允许拒绝重复哈希（高度 = 记录序号；安全性由启动 replay 拒绝提供）
			s.v2.legacySeq = append(s.v2.legacySeq, b)
			s.v2.legacyLen = len(s.v2.legacySeq)
			s.v2.records[hash] = &blockRecord{
				hash: hash, height: h, offset: rr.offset, block: b,
				isV2: false, canonical: true, cumWork: cum, parent: b.Header.PrevBlockHash,
			}
			continue
		}

		seenV2 = true
		s.v2.v2Mode = true
		fr := rr.frame
		switch fr.Type {
		case recTypeBlock:
			if err := s.ingestV2Block(fr, rr.offset); err != nil {
				return err
			}
		case recTypeUndo:
			pendingUndo = append(pendingUndo, rawUndo{fr: fr, offset: rr.offset})
		case recTypeDelete:
			if len(fr.Payload) != 32 {
				return fmt.Errorf("%w: %v", ErrCorruptStore, ErrDeletePayloadLen)
			}
			var h [32]byte
			copy(h[:], fr.Payload)
			pendingDel = append(pendingDel, h)
		case recTypeTip:
			if _, err := decodeTipPayload(fr.Payload); err != nil {
				return fmt.Errorf("%w: %v", ErrCorruptStore, err)
			}
			// TIP 使此前同批 DELETE 生效（提交点语义）
			for _, d := range pendingDel {
				committedDel[d] = true
			}
			pendingDel = nil
			tips = append(tips, tipRecord{offset: rr.offset, hash: fr.BlockHash, height: decodedHeight(fr.Payload), cw: decodedCw(fr.Payload)})
		default:
			return fmt.Errorf("%w: %v", ErrCorruptStore, ErrInvalidRecordType)
		}
	}

	// —— 第二遍：UNDO 绑定（悬空 UNDO 被忽略，不导致失败）——
	//
	// 重复 UNDO 的处理：崩溃重试会**合法地**产生两份字节相同的 UNDO（UNDO 已写、
	// BLOCK/TIP 未写 → 重启修复 → 重试同一枚区块再写一遍 UNDO + BLOCK + TIP）。
	// 因此「同一 payload」视为幂等去重；只有**同一区块出现两份不一致的 UNDO**才是
	// 真正的歧义/损坏，必须 REJECT（这正是 M3「duplicate undo hash 必须拒绝」的
	// 实质：索引中绝不能出现无法判定的 UNDO）。
	seenUndo := make(map[[32]byte][]byte)
	for _, ru := range pendingUndo {
		rec, ok := s.v2.records[ru.fr.BlockHash]
		if !ok {
			s.v2.danglingUndo++ // 未提交的悬空 UNDO（崩溃于 UNDO 与 BLOCK 之间）
			continue
		}
		if prev, dup := seenUndo[ru.fr.BlockHash]; dup {
			if !bytes.Equal(prev, ru.fr.Payload) {
				return fmt.Errorf("%w: %w: %x（同一区块存在两份不一致的 UNDO）",
					ErrCorruptStore, ErrDuplicateUndo, ru.fr.BlockHash)
			}
			continue
		}
		undo, err := utxo.DecodeUndo(ru.fr.Payload)
		if err != nil {
			return fmt.Errorf("%w: UNDO 解码失败: %v", ErrCorruptStore, err)
		}
		if undo.Height != int(ru.fr.Height) || undo.Height != rec.height {
			return fmt.Errorf("%w: %w: undo.Height=%d 帧 height=%d 区块 height=%d",
				ErrCorruptStore, ErrUndoBinding, undo.Height, ru.fr.Height, rec.height)
		}
		s.v2.undoIndex[ru.fr.BlockHash] = undoRecord{blockHash: ru.fr.BlockHash, height: ru.fr.Height, offset: ru.offset}
		seenUndo[ru.fr.BlockHash] = ru.fr.Payload
	}

	// —— 第三遍：确定 canonical tip（REBUILD / ROLLBACK / REJECT）——
	mode := "REBUILD"
	if len(tips) > 0 {
		window := tips
		if len(window) > tipRingSize {
			window = window[len(window)-tipRingSize:] // 只保留最近 K=8 个恢复候选（M5）
		}
		var chosen *tipRecord
		for i := len(window) - 1; i >= 0; i-- {
			if err := s.validateTipCandidate(window[i]); err == nil {
				chosen = &window[i]
				break
			}
		}
		if chosen == nil {
			// 最近 K 个候选全部失效 → REJECT（不猜测）
			return fmt.Errorf("%w（考察 %d 个）", ErrTipRingExhausted, len(window))
		}
		if chosen != &window[len(window)-1] {
			mode = "ROLLBACK"
		}
		if err := s.setCanonicalFrom(chosen.hash); err != nil {
			return err
		}
		s.v2.tipHash = chosen.hash
		s.v2.tipHeight = int(chosen.height)
		s.v2.hasTip = true
		s.v2.chainwork = new(big.Int).SetBytes(chosen.cw[:])
		s.v2.tipRing = append([]tipRecord(nil), window...)
	} else {
		// 无 TIP：canonical = legacy 前缀（REBUILD 出「已提交历史前缀」）
		if err := s.setCanonicalFromLegacyPrefix(); err != nil {
			return err
		}
		if s.v2.legacyLen > 0 {
			s.v2.tipHeight = s.v2.legacyLen - 1
			s.v2.tipHash = s.byHeight[len(s.byHeight)-1].Header.Hash()
			s.v2.hasTip = false
		} else {
			s.v2.tipHeight = -1
			s.v2.hasTip = false
		}
		s.v2.chainwork = new(big.Int).Set(s.v2.legacyCum)
	}

	// 未到期（无后续 TIP 提交）的墓碑不生效 —— 崩溃中途的删除被回滚
	// （committedDel 只在遇到 TIP 时由 pendingDel 转入，故残留的 pendingDel 天然被丢弃）
	for h := range committedDel {
		if rec, ok := s.v2.records[h]; ok {
			rec.deleted = true
		}
	}
	s.v2.recoveryMode = mode
	return nil
}

// decodedHeight / decodedCw 是 decodeTipPayload 已校验后的便捷提取（避免调用方重复解码）。
func decodedHeight(payload []byte) uint32 { return binary.LittleEndian.Uint32(payload[32:36]) }

func decodedCw(payload []byte) [32]byte {
	var cw [32]byte
	copy(cw[:], payload[0:32])
	return cw
}

// ingestV2Block 处理一条 v2 BLOCK 帧：解码、绑定、严格高度派生（M8）。
func (s *FileBlockStore) ingestV2Block(fr frameInfo, offset int64) error {
	b, err := block.DecodeBlock(fr.Payload)
	if err != nil {
		return fmt.Errorf("%w: v2 区块解码失败: %v", ErrCorruptStore, err)
	}
	hash := b.Header.Hash()
	if hash != fr.BlockHash {
		return fmt.Errorf("%w: 帧 blockHash %x != 区块哈希 %x", ErrCorruptStore, fr.BlockHash, hash)
	}
	if _, dup := s.v2.records[hash]; dup {
		return fmt.Errorf("%w: %w: %x", ErrCorruptStore, ErrDuplicateBlock, hash)
	}
	parentHash := b.Header.PrevBlockHash
	var height int
	parentCum := big.NewInt(0)
	if parentHash == ([32]byte{}) {
		// SP-3b 封堵：零父哈希仅允许创世位置（无任何既有区块）
		if len(s.v2.records) != 0 {
			return fmt.Errorf("%w: %w: 已有 %d 个区块", ErrCorruptStore, ErrZeroParentHash, len(s.v2.records))
		}
		height = 0
	} else {
		p, ok := s.v2.records[parentHash]
		if !ok {
			return fmt.Errorf("%w: %w: %x", ErrCorruptStore, ErrParentNotFound, parentHash)
		}
		height = p.height + 1
		parentCum = p.cumWork
	}
	// SP-3：帧声明高度必须严格 == 父高度 + 1
	if int(fr.Height) != height {
		return fmt.Errorf("%w: %w: 帧 height=%d 派生 height=%d", ErrCorruptStore, ErrInvalidHeight, fr.Height, height)
	}
	w, err := workOfBits(b.Header.Bits)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCorruptStore, err)
	}
	s.v2.records[hash] = &blockRecord{
		hash: hash, height: height, offset: offset, block: b,
		isV2: true, canonical: false, cumWork: new(big.Int).Add(parentCum, w), parent: parentHash,
	}
	s.v2.v2Blocks++
	return nil
}

// validateTipCandidate 校验一枚 TIP 候选是否可用于确定 canonical 状态（I1）。
func (s *FileBlockStore) validateTipCandidate(t tipRecord) error {
	rec, ok := s.v2.records[t.hash]
	if !ok {
		return fmt.Errorf("%w: %x", ErrTipBlockMissing, t.hash)
	}
	if int(t.height) != rec.height {
		return fmt.Errorf("%w: tip=%d 区块=%d", ErrTipHeightMismatch, t.height, rec.height)
	}
	// legacy 前缀不可被 TIP 截断（M2：legacy = 已提交历史前缀）
	if s.v2.legacyLen > 0 && rec.height < s.v2.legacyLen-1 {
		return fmt.Errorf("%w: tip 高度 %d 低于 legacy 前缀末尾 %d", ErrTipHeightMismatch, rec.height, s.v2.legacyLen-1)
	}
	cur := rec
	for cur.height > 0 {
		p, ok := s.v2.records[cur.parent]
		if !ok {
			return fmt.Errorf("%w: %x", ErrParentNotFound, cur.parent)
		}
		if p.height != cur.height-1 {
			return fmt.Errorf("%w: %d -> %d", ErrInvalidHeight, cur.height, p.height)
		}
		cur = p
	}
	if cur.parent != ([32]byte{}) {
		return fmt.Errorf("%w: 高度 0 的父哈希非零", ErrZeroParentHash)
	}
	// I1：canonical 路径上的每个 **v2** 区块都必须有 UNDO（legacy 区块由 replay 契约承担）
	cur = rec
	for {
		if cur.isV2 {
			if _, ok := s.v2.undoIndex[cur.hash]; !ok {
				return fmt.Errorf("%w: %x (height=%d)", ErrTipUndoMissing, cur.hash, cur.height)
			}
		}
		if cur.height == 0 {
			break
		}
		cur = s.v2.records[cur.parent]
	}
	return nil
}

// setCanonicalFrom 依据给定 tip 重建 canonical 视图（byHeight/byHash + canonical 标记）。
func (s *FileBlockStore) setCanonicalFrom(tipHash [32]byte) error {
	rec, ok := s.v2.records[tipHash]
	if !ok {
		return fmt.Errorf("%w: %x", ErrTipBlockMissing, tipHash)
	}
	path := make([]*blockRecord, 0, rec.height+1)
	cur := rec
	for {
		path = append(path, cur)
		if cur.height == 0 {
			break
		}
		p, ok := s.v2.records[cur.parent]
		if !ok {
			return fmt.Errorf("%w: %x", ErrParentNotFound, cur.parent)
		}
		if p.height != cur.height-1 {
			return fmt.Errorf("%w: %d -> %d", ErrInvalidHeight, cur.height, p.height)
		}
		cur = p
	}
	// 反转成 低→高
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	// legacy 前缀必须逐高度哈希一致（legacy 不可变）
	byHeight := make([]*block.Block, 0, len(path))
	byHash := make(map[[32]byte]int, len(path))
	for h, r := range path {
		if r.height != h {
			return fmt.Errorf("%w: 路径高度不连续（位置 %d 高度 %d）", ErrCorruptStore, h, r.height)
		}
		if h < s.v2.legacyLen && r.isV2 {
			return fmt.Errorf("%w: 高度 %d 由 v2 区块占据（legacy 前缀不可变）", ErrCorruptStore, h)
		}
		byHeight = append(byHeight, r.block)
		byHash[r.hash] = h
	}
	for _, r := range s.v2.records {
		r.canonical = false
	}
	for _, r := range path {
		r.canonical = true
	}
	s.byHeight = byHeight
	s.byHash = byHash
	return nil
}

// setCanonicalFromLegacyPrefix 把 canonical 视图设为 legacy 前缀（无 TIP 时的 REBUILD 结果）。
//
// legacy 视图按**记录序列**还原（含历史重复记录形态），而非按哈希去重。
func (s *FileBlockStore) setCanonicalFromLegacyPrefix() error {
	byHeight := make([]*block.Block, len(s.v2.legacySeq))
	copy(byHeight, s.v2.legacySeq)
	byHash := make(map[[32]byte]int, len(byHeight))
	for h, b := range byHeight {
		byHash[b.Header.Hash()] = h // 后出现的同哈希记录覆盖（与今日 legacy 语义一致）
	}
	s.byHeight = byHeight
	s.byHash = byHash
	for _, r := range s.v2.records {
		r.canonical = !r.isV2
	}
	return nil
}

// ── 写入原语（F7：单 fsync / 区块）─────────────────────────────────────────

func (s *FileBlockStore) requireWritable() error {
	if s.file == nil {
		return ErrReadOnlyStore
	}
	return nil
}

// appendFrames 按序追加若干帧并**只 fsync 一次**（F7/§14 单 fsync 设计）。
func (s *FileBlockStore) appendFrames(frames ...[]byte) ([]int64, error) {
	if s.file == nil {
		return nil, ErrReadOnlyStore
	}
	offsets := make([]int64, 0, len(frames))
	pos := s.v2.logSize
	for _, fr := range frames {
		offsets = append(offsets, pos)
		if _, err := s.file.Write(fr); err != nil {
			return nil, fmt.Errorf("写入记录失败: %w", err)
		}
		pos += int64(len(fr))
	}
	if err := s.file.Sync(); err != nil {
		return nil, fmt.Errorf("刷盘失败: %w", err)
	}
	s.v2.logSize = pos
	return offsets, nil
}

// tipFrameFor 构造把 canonical tip 推进到 rec 的 TIP 帧。
func (s *FileBlockStore) tipFrameFor(rec *blockRecord) ([]byte, error) {
	if rec == nil {
		return nil, fmt.Errorf("%w: nil 记录", ErrTipBlockMissing)
	}
	payload, err := encodeTipPayload(rec.cumWork, rec.height)
	if err != nil {
		return nil, err
	}
	return encodeFrame(recTypeTip, uint32(rec.height), rec.hash, payload), nil
}

// pushRing 把一条 TIP 记入会话内恢复候选环（M5：保留最近 K=8 个）。
func (s *FileBlockStore) pushRing(t tipRecord) {
	s.v2.tipRing = append(s.v2.tipRing, t)
	if len(s.v2.tipRing) > tipRingSize {
		s.v2.tipRing = s.v2.tipRing[len(s.v2.tipRing)-tipRingSize:]
	}
}

// deriveStrict 对新区块执行 M8 严格校验，返回派生高度与累积工作量。
//
// 规则（SP-3 + SP-3b 的 v2 路径闭合）：
//   - 重复区块哈希 → ErrDuplicateBlock；
//   - 零父哈希 → 仅当存储中没有任何区块（创世位置），否则 ErrZeroParentHash；
//   - 非零父哈希 → 父必须存在（ErrParentNotFound），且 height == parent.Height + 1（ErrInvalidHeight）。
func (s *FileBlockStore) deriveStrict(b *block.Block) (int, *big.Int, error) {
	hash := b.Header.Hash()
	if _, dup := s.v2.records[hash]; dup {
		return 0, nil, fmt.Errorf("%w: %x", ErrDuplicateBlock, hash)
	}
	w, err := workOfBits(b.Header.Bits)
	if err != nil {
		return 0, nil, err
	}
	parentHash := b.Header.PrevBlockHash
	if parentHash == ([32]byte{}) {
		if len(s.v2.records) != 0 {
			return 0, nil, fmt.Errorf("%w: 存储中已有 %d 个区块", ErrZeroParentHash, len(s.v2.records))
		}
		return 0, w, nil
	}
	p, ok := s.v2.records[parentHash]
	if !ok {
		return 0, nil, fmt.Errorf("%w: %x", ErrParentNotFound, parentHash)
	}
	return p.height + 1, new(big.Int).Add(p.cumWork, w), nil
}
