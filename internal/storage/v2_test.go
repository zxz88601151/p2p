package storage_test

// ════════════════════════════════════════════════════════════════════════════
// REORG-1E · M9 —— 行为级测试（package storage_test，走公开 API）
//
// 覆盖 M2 legacy 兼容 / M3 哈希索引与 detached 分支 / M4 持久 UNDO 绑定 /
// M5 TIP 提交点与环 / M6 逻辑删除与回退 / M8 严格校验（API 层）/ §14 性能闸门。
//
// 帧级故障注入与原始帧反例在 v2_internal_test.go。
// ════════════════════════════════════════════════════════════════════════════

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
)

// ── 夹具 ───────────────────────────────────────────────────────────────────

// eBlock 构造一枚可挖出的 coinbase-only 区块；tag 使同高度不同分支的哈希不同。
func eBlock(t *testing.T, prev [32]byte, height int, tag byte) *block.Block {
	t.Helper()
	var pkh [20]byte
	pkh[0], pkh[1] = byte(height), tag
	cb := transaction.NewCoinbaseTx(pkh, utxo.Subsidy(height), height)
	b := block.NewCandidateBlock(prev, pow.MaxTargetBits, []*transaction.Transaction{cb})
	if found, _ := pow.Mine(b); !found {
		t.Fatalf("挖矿失败 height=%d tag=%d", height, tag)
	}
	return b
}

// eChain 返回一条 coinbase-only 链及其逐块真实 undo。
func eChain(t *testing.T, n int) ([]*block.Block, []utxo.BlockUndo) {
	t.Helper()
	set := utxo.NewUTXOSet()
	blocks := make([]*block.Block, 0, n)
	undos := make([]utxo.BlockUndo, 0, n)
	var prev [32]byte
	for h := 0; h < n; h++ {
		b := eBlock(t, prev, h, 0x11)
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

// eUndoOn 返回「在 base 链之上应用 b」的 undo（用于同高度竞争分支）。
func eUndoOn(t *testing.T, base []*block.Block, b *block.Block) utxo.BlockUndo {
	t.Helper()
	set := utxo.NewUTXOSet()
	for i, bb := range base {
		ns, _, _, err := utxo.ApplyBlockWithUndo(set, bb.Transactions, i)
		if err != nil {
			t.Fatalf("base 状态重建失败 height=%d: %v", i, err)
		}
		set = ns
	}
	_, undo, _, err := utxo.ApplyBlockWithUndo(set, b.Transactions, len(base))
	if err != nil {
		t.Fatalf("fork undo 生成失败: %v", err)
	}
	return undo
}

func eRead(t *testing.T, dir string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "blocks.dat"))
	if err != nil {
		t.Fatalf("读取 blocks.dat 失败: %v", err)
	}
	return b
}

func eMustOpenRW(t *testing.T, dir string) *storage.FileBlockStore {
	t.Helper()
	s, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	return s
}

// ── G-02 legacy + v2 兼容 ──────────────────────────────────────────────────

func TestG02_LegacyPrefixUnchangedAndV2MixedReadable(t *testing.T) {
	dir := t.TempDir()
	blocks, undos := eChain(t, 3)

	// 1) 纯 legacy 文件（模拟 REORG-1E 之前的生产 blocks.dat）
	rw := eMustOpenRW(t, dir)
	if err := rw.SaveBlock(blocks[0]); err != nil {
		t.Fatalf("legacy 写入 0 失败: %v", err)
	}
	if err := rw.SaveBlock(blocks[1]); err != nil {
		t.Fatalf("legacy 写入 1 失败: %v", err)
	}
	if h, _ := rw.Height(); h != 1 {
		t.Fatalf("纯 legacy 高度 = %d, want 1", h)
	}
	if rw.V2Mode() {
		t.Fatal("纯 legacy 文件不应处于 v2 模式")
	}
	if err := rw.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	legacyBytes := eRead(t, dir)

	// 2) 混合：第 3 块走 v2 canonical（UNDO+BLOCK+TIP）
	rw2 := eMustOpenRW(t, dir)
	if err := rw2.AppendCanonicalBlock(blocks[2], undos[2]); err != nil {
		t.Fatalf("v2 追加失败: %v", err)
	}
	if !rw2.V2Mode() {
		t.Fatal("出现 v2 记录后应进入 v2 模式")
	}
	if h, _ := rw2.Height(); h != 2 {
		t.Fatalf("混合后高度 = %d, want 2", h)
	}
	if err := rw2.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}

	after := eRead(t, dir)
	// I8：legacy 前缀逐字节不变
	if !bytes.Equal(after[:len(legacyBytes)], legacyBytes) {
		t.Fatal("legacy 前缀被改写（违反 I8）")
	}
	// F1：v2 记录起点按 legacy 语义读出的长度必然非法 → v1 读者必拒
	raw := binary.LittleEndian.Uint32(after[len(legacyBytes) : len(legacyBytes)+4])
	if raw <= 64<<20 {
		t.Fatalf("v2 记录起点按 legacy 读为 %d，未超出 64 MiB 上限：v1 读者可能误读", raw)
	}

	// 3) 重启后混合文件可正常读取
	ro, err := storage.OpenFileBlockStoreReadOnly(dir)
	if err != nil {
		t.Fatalf("只读打开混合文件失败: %v", err)
	}
	defer ro.Close()
	if h, _ := ro.Height(); h != 2 {
		t.Fatalf("重启后高度 = %d, want 2", h)
	}
	if n := ro.LegacyRecordCount(); n != 2 {
		t.Fatalf("legacy 记录数 = %d, want 2", n)
	}
	// legacy 记录：高度与哈希按「记录序号 + SHA256(decoded block)」解释
	g, err := ro.GetBlockByHeight(0)
	if err != nil {
		t.Fatalf("读取 legacy 创世失败: %v", err)
	}
	if g.Header.Hash() != blocks[0].Header.Hash() {
		t.Fatal("legacy 创世哈希不一致")
	}
	byHash, err := ro.GetBlockByHash(blocks[1].Header.Hash())
	if err != nil {
		t.Fatalf("按哈希读取 legacy 区块失败: %v", err)
	}
	if byHash.Header.Hash() != blocks[1].Header.Hash() {
		t.Fatal("按哈希读取 legacy 区块内容不一致")
	}
	// v2 BLOCK 亦可读取
	v2b, err := ro.GetBlockByHeight(2)
	if err != nil {
		t.Fatalf("读取 v2 区块失败: %v", err)
	}
	if v2b.Header.Hash() != blocks[2].Header.Hash() {
		t.Fatal("v2 区块内容不一致")
	}
	if tip, ok := ro.TipHash(); !ok || tip != blocks[2].Header.Hash() {
		t.Fatal("TIP 未指向 v2 提交的链尾")
	}
}

func TestG02_MalformedInputsRejected(t *testing.T) {
	t.Run("garbage", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "blocks.dat"), []byte("not-a-valid-block-file"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := storage.OpenFileBlockStore(dir); err == nil {
			t.Fatal("垃圾数据未拒绝")
		}
	})
	t.Run("bad-magic-v2", func(t *testing.T) {
		dir := t.TempDir()
		blocks, undos := eChain(t, 2)
		s := eMustOpenRW(t, dir)
		if err := s.AppendCanonicalBlock(blocks[0], undos[0]); err != nil {
			t.Fatal(err)
		}
		s.Close()
		data := eRead(t, dir)
		data[0] = 'X' // 破坏 v2 魔数
		if err := os.WriteFile(filepath.Join(dir, "blocks.dat"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := storage.OpenFileBlockStore(dir); err == nil {
			t.Fatal("坏魔数未拒绝")
		}
	})
}

// ── G-03 detached 分支 ─────────────────────────────────────────────────────

func TestG03_DetachedBranchSurvivesRestartAndForkSwitch(t *testing.T) {
	dir := t.TempDir()
	blocks, undos := eChain(t, 2) // genesis, b1
	s := eMustOpenRW(t, dir)
	if err := s.AppendCanonicalBlock(blocks[0], undos[0]); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendCanonicalBlock(blocks[1], undos[1]); err != nil {
		t.Fatal(err)
	}

	// 两条同高度（2）竞争分支
	fa := eBlock(t, blocks[1].Header.Hash(), 2, 0xA)
	fb := eBlock(t, blocks[1].Header.Hash(), 2, 0xB)
	if err := s.AppendCanonicalBlock(fa, eUndoOn(t, blocks, fa)); err != nil {
		t.Fatalf("追加分支 A 失败: %v", err)
	}
	if err := s.SaveBlockWithUndo(fb, eUndoOn(t, blocks, fb)); err != nil {
		t.Fatalf("追加分支 B（detached）失败: %v", err)
	}

	if ok, _ := s.IsCanonical(fa.Header.Hash()); !ok {
		t.Fatal("分支 A 应 canonical")
	}
	if ok, _ := s.IsCanonical(fb.Header.Hash()); ok {
		t.Fatal("分支 B 不应 canonical")
	}
	if n := s.DetachedCount(); n != 1 {
		t.Fatalf("detached 数 = %d, want 1", n)
	}
	if !s.HasBlock(fb.Header.Hash()) {
		t.Fatal("detached 区块应物理存在")
	}
	sizeBefore := int64(len(eRead(t, dir)))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// 重启后 detached 分支必须存活（I3）
	s2 := eMustOpenRW(t, dir)
	if !s2.HasBlock(fb.Header.Hash()) {
		t.Fatal("重启后 detached 分支丢失（违反 I3）")
	}
	if n := s2.DetachedCount(); n != 1 {
		t.Fatalf("重启后 detached 数 = %d, want 1", n)
	}
	if ok, _ := s2.IsCanonical(fb.Header.Hash()); ok {
		t.Fatal("重启后 detached 分支不应变成 canonical")
	}
	if h, _ := s2.Height(); h != 2 {
		t.Fatalf("重启后高度 = %d, want 2", h)
	}
	// 物理字节保留：文件只增不减
	if got := int64(len(eRead(t, dir))); got < sizeBefore {
		t.Fatalf("文件缩小：%d < %d（违反 I4）", got, sizeBefore)
	}

	// TIP 驱动的分叉切换：提交分支 B
	if err := s2.CommitTip(fb.Header.Hash()); err != nil {
		t.Fatalf("切换到分支 B 失败: %v", err)
	}
	if tip, _ := s2.TipHash(); tip != fb.Header.Hash() {
		t.Fatal("TIP 未切到分支 B")
	}
	if ok, _ := s2.IsCanonical(fa.Header.Hash()); ok {
		t.Fatal("切换后分支 A 应变为 detached")
	}
	// 重复哈希必须拒绝（M3）
	if err := s2.SaveBlockDetached(fa); !errors.Is(err, storage.ErrDuplicateBlock) {
		t.Fatalf("重复区块未被拒绝: %v", err)
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}

	s3 := eMustOpenRW(t, dir)
	defer s3.Close()
	if tip, ok := s3.TipHash(); !ok || tip != fb.Header.Hash() {
		t.Fatal("重启后 TIP 未保持分支 B")
	}
	if n := s3.DetachedCount(); n != 1 {
		t.Fatalf("重启后 detached 数 = %d, want 1", n)
	}
}

// ── G-04 持久 UNDO 与绑定 ──────────────────────────────────────────────────

func TestG04_UndoPersistenceAndBinding(t *testing.T) {
	dir := t.TempDir()
	blocks, undos := eChain(t, 3)
	s := eMustOpenRW(t, dir)
	for i := 0; i < 3; i++ {
		if err := s.AppendCanonicalBlock(blocks[i], undos[i]); err != nil {
			t.Fatalf("追加 %d 失败: %v", i, err)
		}
	}
	// 会话内读取
	got, err := s.UndoFor(blocks[1].Header.Hash())
	if err != nil {
		t.Fatalf("UndoFor 失败: %v", err)
	}
	if !reflect.DeepEqual(got, undos[1]) {
		t.Fatal("会话内 UNDO 与写入值不一致")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// 重启后 encode → persist → restart → decode 全链路一致
	s2 := eMustOpenRW(t, dir)
	defer s2.Close()
	for i := 0; i < 3; i++ {
		got, err := s2.UndoFor(blocks[i].Header.Hash())
		if err != nil {
			t.Fatalf("重启后 UndoFor(%d) 失败: %v", i, err)
		}
		if !reflect.DeepEqual(got, undos[i]) {
			t.Fatalf("重启后 UNDO(%d) 与写入值不一致", i)
		}
	}
	if !s2.HasUndo(blocks[0].Header.Hash()) {
		t.Fatal("HasUndo 应为 true")
	}

	// 缺失 UNDO
	fresh := eBlock(t, blocks[2].Header.Hash(), 3, 0x3)
	if err := s2.SaveBlockDetached(fresh); err != nil {
		t.Fatalf("写入无 UNDO 区块失败: %v", err)
	}
	if s2.HasUndo(fresh.Header.Hash()) {
		t.Fatal("新建 detached 区块不应有 UNDO")
	}
	if _, err := s2.UndoFor(fresh.Header.Hash()); !errors.Is(err, storage.ErrUndoNotFound) {
		t.Fatalf("err = %v, want ErrUndoNotFound", err)
	}

	// 错误高度
	if err := s2.PutUndo(fresh.Header.Hash(), 99, undos[2]); !errors.Is(err, storage.ErrUndoBinding) {
		t.Fatalf("错误高度未被拒绝: %v", err)
	}
	// 重复 UNDO
	if err := s2.PutUndo(blocks[0].Header.Hash(), 0, undos[0]); !errors.Is(err, storage.ErrDuplicateUndo) {
		t.Fatalf("重复 UNDO 未被拒绝: %v", err)
	}
}

// TestG04_SameHeightForkUndoCoexistence 同高度不同分支的 UNDO 可共存（M4）。
func TestG04_SameHeightForkUndoCoexistence(t *testing.T) {
	dir := t.TempDir()
	blocks, undos := eChain(t, 2)
	s := eMustOpenRW(t, dir)
	if err := s.AppendCanonicalBlock(blocks[0], undos[0]); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendCanonicalBlock(blocks[1], undos[1]); err != nil {
		t.Fatal(err)
	}
	fa := eBlock(t, blocks[1].Header.Hash(), 2, 0xC1)
	fb := eBlock(t, blocks[1].Header.Hash(), 2, 0xC2)
	ua, ub := eUndoOn(t, blocks, fa), eUndoOn(t, blocks, fb)
	if err := s.SaveBlockWithUndo(fa, ua); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveBlockWithUndo(fb, ub); err != nil {
		t.Fatal(err)
	}
	if err := s.CommitTip(fa.Header.Hash()); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2 := eMustOpenRW(t, dir)
	defer s2.Close()
	ga, err := s2.UndoFor(fa.Header.Hash())
	if err != nil {
		t.Fatalf("分支 A UNDO 读取失败: %v", err)
	}
	gb, err := s2.UndoFor(fb.Header.Hash())
	if err != nil {
		t.Fatalf("分支 B UNDO 读取失败: %v", err)
	}
	if !reflect.DeepEqual(ga, ua) || !reflect.DeepEqual(gb, ub) {
		t.Fatal("同高度分支 UNDO 无法共存或内容不一致")
	}
	if reflect.DeepEqual(ga, gb) {
		t.Fatal("不同分支的 UNDO 不应相同（测试夹具失效）")
	}
}

// TestG04_CorruptedUndoRejected 损坏的 UNDO 永不被使用：校验和不符必须 REJECT。
func TestG04_CorruptedUndoRejected(t *testing.T) {
	dir := t.TempDir()
	blocks, undos := eChain(t, 2)
	s := eMustOpenRW(t, dir)
	if err := s.AppendCanonicalBlock(blocks[0], undos[0]); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendCanonicalBlock(blocks[1], undos[1]); err != nil {
		t.Fatal(err)
	}
	s.Close()

	data := eRead(t, dir)
	// 在第一枚 UNDO 帧的载荷区翻一位（帧头 48 + 魔数定位）
	off := bytes.Index(data, []byte("PCC2"))
	if off < 0 {
		t.Fatal("未找到 v2 帧")
	}
	data[off+50] ^= 0x01
	if err := os.WriteFile(filepath.Join(dir, "blocks.dat"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.OpenFileBlockStore(dir); err == nil {
		t.Fatal("损坏的 UNDO 帧未被拒绝")
	}
}

// ── G-05 TIP 提交点 / 环 ───────────────────────────────────────────────────

func TestG05_TipCommitPointAndRing(t *testing.T) {
	dir := t.TempDir()
	const n = 12
	blocks, undos := eChain(t, n)
	s := eMustOpenRW(t, dir)
	for i := 0; i < n; i++ {
		if err := s.AppendCanonicalBlock(blocks[i], undos[i]); err != nil {
			t.Fatalf("追加 %d 失败: %v", i, err)
		}
	}
	if h, _ := s.Height(); h != n-1 {
		t.Fatalf("高度 = %d, want %d", h, n-1)
	}
	if got := s.TipRingLen(); got != 8 {
		t.Fatalf("TIP 环长度 = %d, want 8（K=8）", got)
	}
	cw, ok := s.Chainwork()
	if !ok {
		t.Fatal("chainwork 未知")
	}
	// 期望累积工作量 = n * 2^bits（Work(bits)=2^bits，与 blocktree.WorkOfBits 同式）
	want := new(big.Int).Lsh(big.NewInt(n), uint(pow.MaxTargetBits))
	if cw.Cmp(want) != 0 {
		t.Fatalf("累积工作量 = %s, want %s（%d*2^%d）", cw.String(), want.String(), n, pow.MaxTargetBits)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// 重启：从最后一个有效 TIP 恢复
	s2 := eMustOpenRW(t, dir)
	defer s2.Close()
	if h, _ := s2.Height(); h != n-1 {
		t.Fatalf("重启后高度 = %d, want %d", h, n-1)
	}
	if tip, ok := s2.TipHash(); !ok || tip != blocks[n-1].Header.Hash() {
		t.Fatal("重启后 TIP 未恢复到最后一个有效 TIP")
	}
	if got := s2.TipRingLen(); got != 8 {
		t.Fatalf("重启后 TIP 环长度 = %d, want 8", got)
	}
	cw2, _ := s2.Chainwork()
	if cw2.Cmp(cw) != 0 {
		t.Fatalf("重启后 chainwork = %s, want %s", cw2.String(), cw.String())
	}
}

// ── G-08 删除 / 分支删除 / 回退 ────────────────────────────────────────────

func TestG08_LogicalDeleteBranchAndTruncate(t *testing.T) {
	dir := t.TempDir()
	blocks, undos := eChain(t, 4) // 高度 0..3
	s := eMustOpenRW(t, dir)
	for i := 0; i < 4; i++ {
		if err := s.AppendCanonicalBlock(blocks[i], undos[i]); err != nil {
			t.Fatalf("追加 %d 失败: %v", i, err)
		}
	}

	// TruncateFromHeight(2)：保留 0,1；2,3 转为 detached（字节保留）
	sizeBefore := int64(len(eRead(t, dir)))
	if err := s.TruncateFromHeight(2); err != nil {
		t.Fatalf("截断失败: %v", err)
	}
	if h, _ := s.Height(); h != 1 {
		t.Fatalf("截断后高度 = %d, want 1", h)
	}
	if !s.HasBlock(blocks[3].Header.Hash()) {
		t.Fatal("被回退区块的字节被删除（违反 I4）")
	}
	if ok, _ := s.IsCanonical(blocks[2].Header.Hash()); ok {
		t.Fatal("被回退区块仍 canonical")
	}
	if got := int64(len(eRead(t, dir))); got <= sizeBefore {
		t.Fatalf("日志未增长（应只追加 TIP）：%d <= %d", got, sizeBefore)
	}
	// 回退越界与触入 legacy
	if err := s.TruncateFromHeight(0); !errors.Is(err, storage.ErrTruncateOutOfRange) {
		t.Fatalf("h=0 未被拒绝: %v", err)
	}
	if err := s.TruncateFromHeight(99); !errors.Is(err, storage.ErrTruncateOutOfRange) {
		t.Fatalf("h 越界未被拒绝: %v", err)
	}

	// DeleteBlock：canonical 非链尾 → 拒绝
	if err := s.DeleteBlock(blocks[0].Header.Hash()); !errors.Is(err, storage.ErrCannotDeleteCanonical) {
		t.Fatalf("canonical 祖先删除未被拒绝: %v", err)
	}
	// DeleteBlock：detached → 允许
	if err := s.DeleteBlock(blocks[3].Header.Hash()); err != nil {
		t.Fatalf("detached 删除失败: %v", err)
	}
	if del, _ := s.IsDeleted(blocks[3].Header.Hash()); !del {
		t.Fatal("逻辑删除未生效")
	}
	if _, err := s.GetBlockByHash(blocks[3].Header.Hash()); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("被逻辑删除的区块仍可经 GetBlockByHash 取到: %v", err)
	}
	// DeleteBlock：canonical 链尾 → 回退到父
	if err := s.DeleteBlock(blocks[1].Header.Hash()); err != nil {
		t.Fatalf("删除 canonical 链尾失败: %v", err)
	}
	if h, _ := s.Height(); h != 0 {
		t.Fatalf("删除链尾后高度 = %d, want 0", h)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// 重启后：canonical=0，被删/被回退区块的字节全部保留
	s2 := eMustOpenRW(t, dir)
	defer s2.Close()
	if h, _ := s2.Height(); h != 0 {
		t.Fatalf("重启后高度 = %d, want 0", h)
	}
	if n := s2.RecordCount(); n != 4 {
		t.Fatalf("物理记录数 = %d, want 4（逻辑删除不减少记录）", n)
	}
	if s2.HasBlock(blocks[3].Header.Hash()) {
		t.Fatal("被逻辑删除的区块仍可经 HasBlock 命中")
	}
	if del, _ := s2.IsDeleted(blocks[1].Header.Hash()); !del {
		t.Fatal("重启后墓碑未保持")
	}
}

// TestG08_DeleteBranch 分支删除只允许 detached 分支。
func TestG08_DeleteBranch(t *testing.T) {
	dir := t.TempDir()
	blocks, undos := eChain(t, 2)
	s := eMustOpenRW(t, dir)
	defer s.Close()
	for i := 0; i < 2; i++ {
		if err := s.AppendCanonicalBlock(blocks[i], undos[i]); err != nil {
			t.Fatal(err)
		}
	}
	// 构造 detached 分支 f1 -> f2
	f1 := eBlock(t, blocks[1].Header.Hash(), 2, 0xD1)
	f2 := eBlock(t, f1.Header.Hash(), 3, 0xD2)
	if err := s.SaveBlockDetached(f1); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveBlockDetached(f2); err != nil {
		t.Fatal(err)
	}
	if n := s.DetachedCount(); n != 2 {
		t.Fatalf("detached 数 = %d, want 2", n)
	}
	// canonical 分支不得删除
	if _, err := s.DeleteBranch(blocks[1].Header.Hash()); !errors.Is(err, storage.ErrCannotDeleteCanonical) {
		t.Fatalf("canonical 分支删除未被拒绝: %v", err)
	}
	n, err := s.DeleteBranch(f1.Header.Hash())
	if err != nil {
		t.Fatalf("分支删除失败: %v", err)
	}
	if n != 2 {
		t.Fatalf("删除区块数 = %d, want 2", n)
	}
	if del, _ := s.IsDeleted(f1.Header.Hash()); !del {
		t.Fatal("分支根未标记删除")
	}
	if del, _ := s.IsDeleted(f2.Header.Hash()); !del {
		t.Fatal("分支后代未标记删除")
	}
	// 幂等
	if n2, err := s.DeleteBranch(f1.Header.Hash()); err != nil || n2 != 0 {
		t.Fatalf("重复分支删除应幂等返回 0, got n=%d err=%v", n2, err)
	}
}

// TestG08_LegacyImmutableAndTruncateGuard legacy 区不可删、不可被 TIP 截断。
func TestG08_LegacyImmutableAndTruncateGuard(t *testing.T) {
	dir := t.TempDir()
	blocks, undos := eChain(t, 3)
	// legacy 前缀 = 高度 0,1
	rw := eMustOpenRW(t, dir)
	if err := rw.SaveBlock(blocks[0]); err != nil {
		t.Fatal(err)
	}
	if err := rw.SaveBlock(blocks[1]); err != nil {
		t.Fatal(err)
	}
	if err := rw.Close(); err != nil {
		t.Fatal(err)
	}
	// v2 追加高度 2
	s := eMustOpenRW(t, dir)
	if err := s.AppendCanonicalBlock(blocks[2], undos[2]); err != nil {
		t.Fatal(err)
	}
	// legacy 区块不可删
	if err := s.DeleteBlock(blocks[0].Header.Hash()); !errors.Is(err, storage.ErrLegacyImmutable) {
		t.Fatalf("legacy 区块删除未被拒绝: %v", err)
	}
	// 截断到 legacy 末尾（h=2 → 保留 0,1）允许
	if err := s.TruncateFromHeight(2); err != nil {
		t.Fatalf("截断到 legacy 末尾应允许: %v", err)
	}
	if h, _ := s.Height(); h != 1 {
		t.Fatalf("高度 = %d, want 1", h)
	}
	// 再往 legacy 内部截断必须拒绝
	if err := s.TruncateFromHeight(1); !errors.Is(err, storage.ErrTruncateOutOfRange) {
		t.Fatalf("截入 legacy 前缀未被拒绝: %v", err)
	}
	defer s.Close()
}

// ── G-09 API 层严格校验（SP-3 / SP-3b / 模式升级）────────────────────────

func TestG09_StrictAppendAPI(t *testing.T) {
	dir := t.TempDir()
	blocks, undos := eChain(t, 2)
	s := eMustOpenRW(t, dir)
	if err := s.AppendCanonicalBlock(blocks[0], undos[0]); err != nil {
		t.Fatal(err)
	}
	// SP-3b：非创世位置的零父哈希
	phony := eBlock(t, [32]byte{}, 0, 0xEF)
	if err := s.SaveBlockDetached(phony); !errors.Is(err, storage.ErrZeroParentHash) {
		t.Fatalf("零父哈希区块未被拒绝: %v", err)
	}
	// 未知父
	orphan := eBlock(t, [32]byte{0x42}, 9, 0x01)
	if err := s.SaveBlockDetached(orphan); !errors.Is(err, storage.ErrParentNotFound) {
		t.Fatalf("未知父未被拒绝: %v", err)
	}
	// 重复哈希
	if err := s.SaveBlockDetached(blocks[0]); !errors.Is(err, storage.ErrDuplicateBlock) {
		t.Fatalf("重复哈希未被拒绝: %v", err)
	}
	// 进入 v2 模式后 legacy 追加口必须被拒（I1：canonical 区块不得缺 UNDO）
	if err := s.SaveBlock(blocks[1]); !errors.Is(err, storage.ErrLegacyAppendInV2Mode) {
		t.Fatalf("v2 模式下 legacy 追加未被拒绝: %v", err)
	}
	// UNDO 高度不匹配
	if err := s.AppendCanonicalBlock(blocks[1], undos[0]); !errors.Is(err, storage.ErrUndoBinding) {
		t.Fatalf("UNDO 高度错配未被拒绝: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestOptionC_ProductionPathRejectsMismatchedParentBeforeStorage 证明：生产追加
// 路径 blockchain.addBlock 在**触碰存储之前**已强制 PrevHash == tip.Hash()，因此
// SP-3/SP-3b 在生产路径上不可达（把 latent 缺陷固化为可验证边界）。
func TestOptionC_ProductionPathRejectsMismatchedParentBeforeStorage(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	defer store.Close()
	chain, err := blockchain.NewBlockchainFromStore(store)
	if err != nil {
		t.Fatalf("加载链失败: %v", err)
	}
	genesis, err := chain.Tip()
	if err != nil {
		t.Fatalf("读取创世失败: %v", err)
	}

	appendNext := func(prev [32]byte, h int, tag byte) *block.Block {
		t.Helper()
		var pkh [20]byte
		pkh[0], pkh[1] = byte(h), tag
		cb := transaction.NewCoinbaseTx(pkh, utxo.Subsidy(h), h)
		b := block.NewCandidateBlock(prev, pow.MaxTargetBits, []*transaction.Transaction{cb})
		if found, _ := pow.Mine(b); !found {
			t.Fatal("挖矿失败")
		}
		if err := chain.AddBlock(b); err != nil {
			t.Fatalf("追加高度 %d 失败: %v", h, err)
		}
		return b
	}
	b1 := appendNext(genesis.Header.Hash(), 1, 0x01)
	b2 := appendNext(b1.Header.Hash(), 2, 0x02)

	hBefore, _ := store.Height()
	recBefore := 0
	if ro, err := storage.OpenFileBlockStoreReadOnly(dir); err == nil {
		recBefore, _ = ro.Height()
		ro.Close()
	}

	// 构造 parent 指向创世（非链尾）的区块：试图让存储接受「错位重挂」
	stale := func() *block.Block {
		var pkh [20]byte
		pkh[0], pkh[1] = 3, 0x77
		cb := transaction.NewCoinbaseTx(pkh, utxo.Subsidy(3), 3)
		b := block.NewCandidateBlock(genesis.Header.Hash(), pow.MaxTargetBits, []*transaction.Transaction{cb})
		if found, _ := pow.Mine(b); !found {
			t.Fatal("挖矿失败")
		}
		return b
	}()
	if err := chain.AddBlock(stale); err == nil {
		t.Fatal("错位重挂区块被生产路径接受（SP-3 在生产路径可达！）")
	} else if !errors.Is(err, blockchain.ErrInvalidPrevHash) {
		t.Fatalf("err = %v, want ErrInvalidPrevHash（共识层拦截，而非存储层）", err)
	}

	// 存储层未被写入：高度与记录数不变
	if h, _ := store.Height(); h != hBefore {
		t.Fatalf("存储高度被改动: %d → %d", hBefore, h)
	}
	ro2, err := storage.OpenFileBlockStoreReadOnly(dir)
	if err != nil {
		t.Fatalf("只读打开失败: %v", err)
	}
	defer ro2.Close()
	hAfter, _ := ro2.Height()
	if hAfter != recBefore {
		t.Fatalf("持久化记录数被改动: %d → %d", recBefore, hAfter)
	}
	if hAfter != 2 {
		t.Fatalf("链高 = %d, want 2", hAfter)
	}
	_ = b2
}

// ── G-10 性能 / 字节账（§14 单 fsync 闸门）────────────────────────────────

func TestG10_SingleFsyncByteAccountAndThroughput(t *testing.T) {
	const n = 200

	// (a) legacy 基线吞吐
	dirL := t.TempDir()
	lb, _ := eChain(t, n)
	sL := eMustOpenRW(t, dirL)
	startL := time.Now()
	for i := 0; i < n; i++ {
		if err := sL.SaveBlock(lb[i]); err != nil {
			t.Fatalf("legacy 写入 %d 失败: %v", i, err)
		}
	}
	durL := time.Since(startL)
	legacyBytes := int64(len(eRead(t, dirL)))
	sL.Close()

	// (b) 1E canonical：UNDO + BLOCK + TIP 单次 fsync
	dirV := t.TempDir()
	vb, vu := eChain(t, n)
	sV := eMustOpenRW(t, dirV)
	startV := time.Now()
	for i := 0; i < n; i++ {
		if err := sV.AppendCanonicalBlock(vb[i], vu[i]); err != nil {
			t.Fatalf("canonical 追加 %d 失败: %v", i, err)
		}
	}
	durV := time.Since(startV)
	v2Bytes := int64(len(eRead(t, dirV)))
	sV.Close()

	// 字节账必须精确（G-10 的可判定部分）：
	// 每块 = UNDO 帧(80+undo) + BLOCK 帧(80+block) + TIP 帧(80+40)
	ub, err := utxo.EncodeUndo(vu[0])
	if err != nil {
		t.Fatal(err)
	}
	perBlock := int64(3*frameOverheadForTest() + tipPayloadLenForTest() + len(ub) + len(vb[0].Encode()))
	if want := perBlock * n; v2Bytes != want {
		t.Fatalf("v2 日志 %d 字节, want %d（每块 %d）", v2Bytes, want, perBlock)
	}

	thrL := float64(n) / durL.Seconds()
	thrV := float64(n) / durV.Seconds()
	t.Logf("G-10 性能账：legacy=%d B 记录/%.0f blk/s | 1E canonical=%d B 记录/%.0f blk/s",
		legacyBytes, thrL, v2Bytes, thrV)
	t.Logf("G-10 字节账：区块=%d B, undo=%d B, 每块日志增长=%d B（legacy=%d B）",
		len(vb[0].Encode()), len(ub), perBlock, len(lb[0].Encode())+4)

	// 结构性闸门：canonical 路径每块仍只 fsync 一次，吞吐不得退化到 1/3 以下
	// （若退化为 3 fsync/block，机械盘上会跌到约 1/3）。
	if thrV < thrL/3.0 {
		t.Fatalf("1E canonical 吞吐 %.1f blk/s 相对 legacy %.1f blk/s 退化超过 3×（疑似多次 fsync）", thrV, thrL)
	}
}

// frameOverheadForTest / tipPayloadLenForTest 与内部 frame.go 的常量同值
// （外部测试包不可见内部常量）。若内部布局漂移，G-10 的字节账断言会立即报警。
func frameOverheadForTest() int { return 80 }
func tipPayloadLenForTest() int { return 40 }
