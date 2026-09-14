package storage

// ════════════════════════════════════════════════════════════════════════════
// REORG-1E · M9 —— 内部测试（package storage）
//
// 本文件覆盖需要访问**内部编码器与内存状态**的测试：M1 帧编解码与全部 fail-stop
// 拒绝分支、M6/M7 真实确定性故障注入（按字节构造崩溃镜像）、M5 TIP 环/回滚，
// 以及 M8 SP-3/SP-3b 的原始帧级反例。
//
// 行为级测试（走公开 API）在 v2_test.go（package storage_test）。
// ════════════════════════════════════════════════════════════════════════════

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
)

// ── 夹具 ───────────────────────────────────────────────────────────────────

// iBlock 构造一枚可挖出的 coinbase-only 区块（确定性：pkh 由 height+tag 派生）。
func iBlock(t *testing.T, prev [32]byte, height int, tag byte) *block.Block {
	t.Helper()
	var pkh [20]byte
	pkh[0] = byte(height)
	pkh[1] = tag
	cb := transaction.NewCoinbaseTx(pkh, utxo.Subsidy(height), height)
	b := block.NewCandidateBlock(prev, pow.MaxTargetBits, []*transaction.Transaction{cb})
	if found, _ := pow.Mine(b); !found {
		t.Fatalf("挖矿失败 height=%d tag=%d", height, tag)
	}
	return b
}

// iChain 构造一条 coinbase-only 链及其逐块真实 undo（走 REORG-1D 的
// ApplyBlockWithUndo，非人造数据）。
func iChain(t *testing.T, n int) ([]*block.Block, []utxo.BlockUndo) {
	t.Helper()
	set := utxo.NewUTXOSet()
	blocks := make([]*block.Block, 0, n)
	undos := make([]utxo.BlockUndo, 0, n)
	var prev [32]byte
	for h := 0; h < n; h++ {
		b := iBlock(t, prev, h, 0x11)
		ns, undo, _, err := utxo.ApplyBlockWithUndo(set, b.Transactions, h)
		if err != nil {
			t.Fatalf("undo 生成失败 height=%d: %v", h, err)
		}
		set = ns
		blocks = append(blocks, b)
		undos = append(undos, undo)
		prev = b.Header.Hash()
	}
	return blocks, undos
}

func iReadLog(t *testing.T, dir string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "blocks.dat"))
	if err != nil {
		t.Fatalf("读取 blocks.dat 失败: %v", err)
	}
	return b
}

func iWriteLog(t *testing.T, dir string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "blocks.dat"), data, 0o600); err != nil {
		t.Fatalf("写 blocks.dat 失败: %v", err)
	}
}

// iSeedLegacy 用 legacy API 建立一条已落盘的单链（模拟 REORG-1E 之前的生产文件）。
func iSeedLegacy(t *testing.T, dir string, n int) []*block.Block {
	t.Helper()
	store, err := OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	blocks, _ := iChain(t, n)
	for i, b := range blocks {
		if err := store.SaveBlock(b); err != nil {
			t.Fatalf("legacy 写入 %d 失败: %v", i, err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	return blocks
}

// ── G-01 记录帧 ────────────────────────────────────────────────────────────

func TestG01_FrameRoundTripAndOverhead(t *testing.T) {
	payload := []byte("hello-p2pchain")
	var hash [32]byte
	copy(hash[:], bytes.Repeat([]byte{0x7a}, 32))

	for _, typ := range []byte{recTypeBlock, recTypeUndo, recTypeTip, recTypeDelete} {
		enc := encodeFrame(typ, 42, hash, payload)
		if got := len(enc) - len(payload); got != 80 {
			t.Fatalf("type=%s 帧开销 = %d, want 80 (F2)", recTypeName(typ), got)
		}
		fr, err := decodeFrame(enc)
		if err != nil {
			t.Fatalf("type=%s 解码失败: %v", recTypeName(typ), err)
		}
		if fr.Type != typ || fr.Height != 42 || fr.BlockHash != hash {
			t.Fatalf("type=%s 往返不一致: %+v", recTypeName(typ), fr)
		}
		if !bytes.Equal(fr.Payload, payload) {
			t.Fatalf("type=%s 载荷不一致", recTypeName(typ))
		}
		if fr.TotalLen != 80+len(payload) {
			t.Fatalf("type=%s TotalLen=%d", recTypeName(typ), fr.TotalLen)
		}
	}
}

// TestG01_MagicIsAboveLegacyLimit 证明 F1 的零迁移兼容：'PCC2' 按小端 u32 必
// 大于 legacy 长度上限，故 v1 读者遇到 v2 记录必然以「非法长度」拒绝。
func TestG01_MagicIsAboveLegacyLimit(t *testing.T) {
	asU32 := binary.LittleEndian.Uint32([]byte(frameMagic))
	if asU32 != 843268944 {
		t.Fatalf("PCC2 小端 u32 = %d, want 843268944", asU32)
	}
	if asU32 <= maxPayloadLen {
		t.Fatalf("魔数 %d 未超过 legacy 长度上限 %d：存在歧义风险（F1 失效）", asU32, maxPayloadLen)
	}
	if hasFrameMagic([]byte("PCC")) {
		t.Fatal("3 字节不应被判为魔数")
	}
	if hasFrameMagic([]byte("PCC3")) {
		t.Fatal("PCC3 不应被判为魔数")
	}
	if !hasFrameMagic([]byte("PCC2")) {
		t.Fatal("PCC2 应被判为魔数")
	}
}

func TestG01_FrameRejectsAllMalformed(t *testing.T) {
	payload := []byte("0123456789")
	var hash [32]byte
	good := encodeFrame(recTypeBlock, 7, hash, payload)
	const n = 10

	tests := []struct {
		name string
		mut  func([]byte) []byte
		want error
	}{
		{"bad-magic", func(b []byte) []byte { m := clone(b); m[0] = 'X'; return m }, ErrBadRecordMagic},
		{"bad-version", func(b []byte) []byte { m := clone(b); m[4] = 3; return m }, ErrUnsupportedRecordVersion},
		{"bad-type", func(b []byte) []byte { m := clone(b); m[5] = 99; return m }, ErrInvalidRecordType},
		{"non-zero-reserved", func(b []byte) []byte { m := clone(b); m[6] = 1; return m }, ErrNonZeroReserved},
		{"zero-payload-len", func(b []byte) []byte {
			m := clone(b)
			binary.LittleEndian.PutUint32(m[8:12], 0)
			return m
		}, ErrInvalidPayloadLen},
		{"huge-payload-len", func(b []byte) []byte {
			m := clone(b)
			binary.LittleEndian.PutUint32(m[8:12], maxPayloadLen+1)
			return m
		}, ErrInvalidPayloadLen},
		{"truncated-header", func(b []byte) []byte { return clone(b)[:frameHeaderSize-1] }, ErrTruncatedHeader},
		{"truncated-payload", func(b []byte) []byte {
			return clone(b)[:frameHeaderSize+n-1]
		}, ErrTruncatedPayload},
		{"missing-checksum", func(b []byte) []byte { return clone(b)[:frameHeaderSize+n] }, ErrMissingChecksum},
		{"checksum-mismatch", func(b []byte) []byte { m := clone(b); m[frameHeaderSize] ^= 0x01; return m }, ErrChecksumMismatch},
		{"header-bit-flip", func(b []byte) []byte { m := clone(b); m[12] ^= 0x01; return m }, ErrChecksumMismatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeFrame(tc.mut(good))
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func clone(b []byte) []byte { return append([]byte(nil), b...) }

// ── G-06 崩溃注入矩阵（真实确定性故障注入，非随机）──────────────────────

// crashImage 是「下一枚 canonical 区块」的完整字节镜像，按固定故障点裁剪即可
// 精确复现任意崩溃时刻的磁盘状态。
//
// blocks / undos 与 base 出自同一次 iChain 调用：测试必须复用同一份区块对象，
// 因为 NewCandidateBlock 的时间戳与 pow.Mine 的 nonce 都不确定，重复生成会得到
// 不同的哈希（父哈希将对不上）。
type crashImage struct {
	base     []byte
	undoF    []byte
	blkF     []byte
	tipF     []byte
	nextHash [32]byte
	blocks   []*block.Block
	undos    []utxo.BlockUndo
}

func iBuildCrashImage(t *testing.T) crashImage {
	t.Helper()
	dir := t.TempDir()
	store, err := OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	blocks, undos := iChain(t, 3) // genesis(0), b1(1), b2(2)
	for i := 0; i < 2; i++ {
		if err := store.AppendCanonicalBlock(blocks[i], undos[i]); err != nil {
			t.Fatalf("基线追加 %d 失败: %v", i, err)
		}
	}
	cw, ok := store.Chainwork()
	if !ok {
		t.Fatal("基线 chainwork 未知")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	base := iReadLog(t, dir)

	b := blocks[2]
	ub, err := utxo.EncodeUndo(undos[2])
	if err != nil {
		t.Fatalf("编码 undo 失败: %v", err)
	}
	w, err := workOfBits(b.Header.Bits)
	if err != nil {
		t.Fatalf("workOfBits 失败: %v", err)
	}
	tp, err := encodeTipPayload(new(big.Int).Add(cw, w), 2)
	if err != nil {
		t.Fatalf("编码 tip 失败: %v", err)
	}
	hash := b.Header.Hash()
	return crashImage{
		base:     base,
		undoF:    encodeFrame(recTypeUndo, 2, hash, ub),
		blkF:     encodeFrame(recTypeBlock, 2, hash, b.Encode()),
		tipF:     encodeFrame(recTypeTip, 2, hash, tp),
		nextHash: hash,
		blocks:   blocks,
		undos:    undos,
	}
}

// TestG06_CrashInjectionMatrix 覆盖 M7 崩溃矩阵 8 个故障点，并断言：
//   - 每个故障点的 canonical 状态**确定**（前一已提交状态，或下一完整提交状态）；
//   - 已提交字节永不被改写（修复后文件 = 基线 + 完整帧前缀）；
//   - 不出现「无法判定的 canonical 状态」。
func TestG06_CrashInjectionMatrix(t *testing.T) {
	img := iBuildCrashImage(t)
	tail := bytes.Join([][]byte{img.undoF, img.blkF, img.tipF}, nil)
	if len(img.tipF) != frameOverhead+tipPayloadLen {
		t.Fatalf("TIP 帧长度 = %d，与预期不符", len(img.tipF))
	}

	type tc struct {
		name         string
		cut          int
		wantHeight   int
		wantDetach   int
		wantDangling int
		wantTorn     bool // 打开时是否存在需修复的截断残片
	}
	cases := []tc{
		{"1-before-undo", 0, 1, 0, 0, false},
		{"2-partial-undo-header", 8, 1, 0, 0, true},
		{"3-partial-undo-payload", frameHeaderSize + 10, 1, 0, 0, true},
		{"4-complete-undo-before-block", len(img.undoF), 1, 0, 1, false},
		{"5-partial-block", len(img.undoF) + 8, 1, 0, 1, true},
		{"6-complete-block-before-tip", len(img.undoF) + len(img.blkF), 1, 1, 0, false},
		{"7-partial-tip", len(img.undoF) + len(img.blkF) + 8, 1, 1, 0, true},
		{"8-complete-tip", len(tail), 2, 0, 0, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			torn := append(clone(img.base), tail[:c.cut]...)

			// (a) 只读路径必须在**未修复的原始残片上**严格拒绝（且不得修改文件）
			dirRO := t.TempDir()
			iWriteLog(t, dirRO, torn)
			before := iReadLog(t, dirRO)
			ro, rerr := OpenFileBlockStoreReadOnly(dirRO)
			if rerr != nil {
				if !c.wantTorn {
					t.Fatalf("只读打开不应失败: %v", rerr)
				}
			} else {
				ro.Close()
				if c.wantTorn {
					t.Fatal("只读打开应拒绝截断残片")
				}
			}
			if after := iReadLog(t, dirRO); !bytes.Equal(before, after) {
				t.Fatal("只读路径修改了数据文件")
			}

			// (b) 可写路径必须判定出确定的 canonical 状态
			dir := t.TempDir()
			iWriteLog(t, dir, torn)
			store, err := OpenFileBlockStore(dir)
			if err != nil {
				t.Fatalf("可写打开失败（canonical 状态必须可判定）: %v", err)
			}
			defer store.Close()

			h, _ := store.Height()
			if h != c.wantHeight {
				t.Fatalf("canonical 高度 = %d, want %d", h, c.wantHeight)
			}
			if got := store.DetachedCount(); got != c.wantDetach {
				t.Fatalf("detached 数 = %d, want %d", got, c.wantDetach)
			}
			if got := store.DanglingUndoCount(); got != c.wantDangling {
				t.Fatalf("悬空 UNDO 数 = %d, want %d", got, c.wantDangling)
			}

			// 完整帧的字节前缀（= 崩溃后仍可信的尾部长度）
			committed := 0
			switch {
			case c.cut >= len(tail):
				committed = len(tail)
			case c.cut >= len(img.undoF)+len(img.blkF):
				committed = len(img.undoF) + len(img.blkF)
			case c.cut >= len(img.undoF):
				committed = len(img.undoF)
			}

			// 已提交字节不得被改写：文件 = 基线 + 完整帧前缀
			want := append(clone(img.base), tail[:committed]...)
			if got := iReadLog(t, dir); !bytes.Equal(got, want) {
				t.Fatalf("磁盘日志与「基线 + 完整帧前缀」不一致：len got=%d want=%d", len(got), len(want))
			}
			if store.LogSize() != int64(len(want)) {
				t.Fatalf("LogSize = %d, want %d", store.LogSize(), len(want))
			}
		})
	}
}

// ── G-07 尾部修复边界 ─────────────────────────────────────────────────────

// TestG07_RepairedLogIsUsable 证明修复后的日志仍可继续追加（不是只读残骸）。
func TestG07_RepairedLogIsUsable(t *testing.T) {
	img := iBuildCrashImage(t)
	dir := t.TempDir()
	// 在 BLOCK 帧中途崩溃
	iWriteLog(t, dir, append(clone(img.base), img.undoF...))
	iWriteLog(t, dir, append(iReadLog(t, dir), img.blkF[:20]...))

	store, err := OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("修复打开失败: %v", err)
	}
	if h, _ := store.Height(); h != 1 {
		t.Fatalf("修复后高度 = %d, want 1", h)
	}
	// 修复后继续提交同一枚区块必须成功（悬空 UNDO 不得阻塞重新追加）
	if err := store.AppendCanonicalBlock(img.blocks[2], img.undos[2]); err != nil {
		t.Fatalf("修复后重新追加失败: %v", err)
	}
	if h, _ := store.Height(); h != 2 {
		t.Fatalf("追加后高度 = %d, want 2", h)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	// 重启后仍为 h=2
	store2, err := OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("重启失败: %v", err)
	}
	defer store2.Close()
	if h, _ := store2.Height(); h != 2 {
		t.Fatalf("重启后高度 = %d, want 2", h)
	}
}

// TestG07_LegacyTruncatedTailIsRejected 纯 legacy 截断尾部**不得修复**：
// 没有 TIP 就无从证明该记录未被提交（M7 规则），必须 REJECT。
func TestG07_LegacyTruncatedTailIsRejected(t *testing.T) {
	dir := t.TempDir()
	iSeedLegacy(t, dir, 2)
	full := iReadLog(t, dir)

	// 截断 3 字节（legacy 记录数据不完整）
	iWriteLog(t, dir, full[:len(full)-3])
	if _, err := OpenFileBlockStore(dir); err == nil {
		t.Fatal("纯 legacy 截断尾部被静默修复（应 REJECT）")
	}
	if _, err := OpenFileBlockStoreReadOnly(dir); err == nil {
		t.Fatal("只读路径未拒绝纯 legacy 截断尾部")
	}
}

// TestG07_LegacyPrefixPlusPartialV2IsRepaired 尾部残片起始即 v2 魔数时，
// 即便此前没有任何完整 v2 记录，也可证明其为「未提交的 v2 写入」→ 允许修复。
func TestG07_LegacyPrefixPlusPartialV2IsRepaired(t *testing.T) {
	dir := t.TempDir()
	iSeedLegacy(t, dir, 2)
	legacyBytes := iReadLog(t, dir)

	var hash [32]byte
	frame := encodeFrame(recTypeBlock, 2, hash, []byte("payload-payload"))
	iWriteLog(t, dir, append(clone(legacyBytes), frame[:frameHeaderSize+3]...))

	store, err := OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("v2 残片应可修复: %v", err)
	}
	defer store.Close()
	if got := iReadLog(t, dir); !bytes.Equal(got, legacyBytes) {
		t.Fatal("修复后 legacy 前缀被改写（违反 I8）")
	}
	if h, _ := store.Height(); h != 1 {
		t.Fatalf("高度 = %d, want 1", h)
	}
}

// TestG07_MalformedFrameNeverTruncated 完整长度但校验和不符的帧属于畸形记录，
// **绝不**通过截断「修复」（否则可能删掉已提交数据）。
func TestG07_MalformedFrameNeverTruncated(t *testing.T) {
	img := iBuildCrashImage(t)
	dir := t.TempDir()
	bad := clone(img.tipF)
	bad[frameHeaderSize] ^= 0x01 // 破坏载荷 → 校验和不符
	logData := append(clone(img.base), img.undoF...)
	logData = append(logData, img.blkF...)
	logData = append(logData, bad...)
	iWriteLog(t, dir, logData)

	before := iReadLog(t, dir)
	if _, err := OpenFileBlockStore(dir); err == nil {
		t.Fatal("畸形 TIP 帧被接受（应 REJECT）")
	}
	if after := iReadLog(t, dir); !bytes.Equal(before, after) {
		t.Fatal("畸形帧被物理截断（不得删除可能已提交的字节）")
	}
}

// ── G-05 TIP 环 / 回滚 ─────────────────────────────────────────────────────

// TestG05_RollbackToPreviousTip 完整有效但语义失效的 TIP → 逻辑 ROLLBACK 到上一 TIP。
func TestG05_RollbackToPreviousTip(t *testing.T) {
	img := iBuildCrashImage(t)

	// (a) TIP 引用不存在的 BLOCK → 回退上一 TIP
	dir := t.TempDir()
	var bogus [32]byte
	copy(bogus[:], bytes.Repeat([]byte{0xAB}, 32))
	tp, _ := encodeTipPayload(big.NewInt(1<<16), 99)
	iWriteLog(t, dir, append(clone(img.base), encodeFrame(recTypeTip, 99, bogus, tp)...))

	store, err := OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("应回滚而非拒绝: %v", err)
	}
	if h, _ := store.Height(); h != 1 {
		t.Fatalf("回滚后高度 = %d, want 1", h)
	}
	if got := store.RecoveryMode(); got != "ROLLBACK" {
		t.Fatalf("RecoveryMode = %q, want ROLLBACK", got)
	}
	store.Close()

	// (b) TIP 引用缺失 UNDO 的区块 → 保守回滚
	dir2 := t.TempDir()
	b2 := img.blocks[2]
	blkF := encodeFrame(recTypeBlock, 2, b2.Header.Hash(), b2.Encode())
	tp2, _ := encodeTipPayload(big.NewInt(1<<16), 2)
	tipF2 := encodeFrame(recTypeTip, 2, b2.Header.Hash(), tp2)
	iWriteLog(t, dir2, append(clone(img.base), append(blkF, tipF2...)...))

	store2, err := OpenFileBlockStore(dir2)
	if err != nil {
		t.Fatalf("缺失 UNDO 的 TIP 应保守回滚而非拒绝: %v", err)
	}
	defer store2.Close()
	if h, _ := store2.Height(); h != 1 {
		t.Fatalf("回滚后高度 = %d, want 1", h)
	}
	if got := store2.RecoveryMode(); got != "ROLLBACK" {
		t.Fatalf("RecoveryMode = %q, want ROLLBACK", got)
	}
	if got := store2.DetachedCount(); got != 1 {
		t.Fatalf("detached = %d, want 1（区块已落盘但未 canonical）", got)
	}
}

// TestG05_TipRingExhaustedRejected 最近 K=8 个 TIP 候选全部失效 → REJECT。
func TestG05_TipRingExhaustedRejected(t *testing.T) {
	img := iBuildCrashImage(t)
	dir := t.TempDir()
	var bogus [32]byte
	copy(bogus[:], bytes.Repeat([]byte{0xCD}, 32))
	tp, _ := encodeTipPayload(big.NewInt(1<<16), 7)
	data := clone(img.base)
	for i := 0; i < tipRingSize; i++ {
		data = append(data, encodeFrame(recTypeTip, 7, bogus, tp)...)
	}
	iWriteLog(t, dir, data)

	_, err := OpenFileBlockStore(dir)
	if !errors.Is(err, ErrTipRingExhausted) {
		t.Fatalf("err = %v, want ErrTipRingExhausted", err)
	}
}

// ── G-09 SP-3 / SP-3b（原始帧级反例）──────────────────────────────────────

// TestG09_SP3_WrongDeclaredHeightRejected 校验和不符与高度错位都必须被拒：
// v2 语义下区块高度由父链**确定性派生**，帧声明高度必须严格等于 parent.Height+1。
func TestG09_SP3_WrongDeclaredHeightRejected(t *testing.T) {
	img := iBuildCrashImage(t) // base 高度 1，下一块应落在高度 2
	dir := t.TempDir()
	b2 := img.blocks[2]
	wrong := encodeFrame(recTypeBlock, 99, b2.Header.Hash(), b2.Encode())
	iWriteLog(t, dir, append(clone(img.base), wrong...))

	if _, err := OpenFileBlockStore(dir); !errors.Is(err, ErrInvalidHeight) {
		t.Fatalf("err = %v, want ErrInvalidHeight（SP-3）", err)
	}
}

// TestG09_SP3b_ZeroParentHashAtNonGenesisRejected 零父哈希区块出现在非创世
// 位置必须被拒（彻底封堵 SP-3b 的绕过路径）。
func TestG09_SP3b_ZeroParentHashAtNonGenesisRejected(t *testing.T) {
	img := iBuildCrashImage(t)
	dir := t.TempDir()
	// 用零父哈希构造一枚「伪创世」区块，声明高度 0
	fake := iBlock(t, [32]byte{}, 0, 0xEE)
	wrong := encodeFrame(recTypeBlock, 0, fake.Header.Hash(), fake.Encode())
	iWriteLog(t, dir, append(clone(img.base), wrong...))

	_, err := OpenFileBlockStore(dir)
	if err == nil {
		t.Fatal("非创世位置的零父哈希区块被接受（SP-3b 未封堵）")
	}
	if !errors.Is(err, ErrZeroParentHash) {
		t.Fatalf("err = %v, want ErrZeroParentHash", err)
	}
}

// TestG09_LegacyAfterV2Rejected 语义区交错（legacy 记录出现在 v2 记录之后）必须被拒。
func TestG09_LegacyAfterV2Rejected(t *testing.T) {
	img := iBuildCrashImage(t)
	dir := t.TempDir()
	extra := iBlock(t, [32]byte{0x5A}, 99, 0x01)
	enc := extra.Encode()
	var lb [4]byte
	binary.LittleEndian.PutUint32(lb[:], uint32(len(enc)))
	iWriteLog(t, dir, append(clone(img.base), append(lb[:], enc...)...))

	if _, err := OpenFileBlockStore(dir); err == nil {
		t.Fatal("v2 之后的 legacy 记录被接受（语义区交错）")
	}
}
