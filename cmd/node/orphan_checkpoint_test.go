// orphan_checkpoint_test.go 覆盖 SPEC-v1 §5.3 单测矩阵 T1–T9。
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// newTestCP 在临时目录创建一个检查点句柄（不落盘）。
func newTestCP(t *testing.T) (*orphanCheckpoint, string) {
	t.Helper()
	dir := t.TempDir()
	return newOrphanCheckpoint(dir), dir
}

// T1 — round-trip：markDirty + flush + load ⇒ 父键/子哈希逐字节一致。
func TestOrphanCP_T1_RoundTrip(t *testing.T) {
	c, _ := newTestCP(t)

	parentA := [32]byte{0x01}
	parentB := [32]byte{0x02}
	child1 := [32]byte{0xaa}
	child2 := [32]byte{0xbb}
	child3 := [32]byte{0xcc}

	c.MarkDirty(parentA, child1)
	c.MarkDirty(parentA, child2) // 同 parent 追加
	c.MarkDirty(parentB, child3)

	if err := c.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	entries, err := c.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	if entries[0].parentHash != parentA || entries[1].parentHash != parentB {
		t.Fatalf("entry order wrong: %x, %x", entries[0].parentHash, entries[1].parentHash)
	}
	if len(entries[0].childHash) != 2 || entries[0].childHash[0] != child1 || entries[0].childHash[1] != child2 {
		t.Fatalf("parentA children wrong: %x", entries[0].childHash)
	}
	if len(entries[1].childHash) != 1 || entries[1].childHash[0] != child3 {
		t.Fatalf("parentB children wrong: %x", entries[1].childHash)
	}
}

// T2 — 空/无文件：load ⇒ 空，不报错。
func TestOrphanCP_T2_NoFile(t *testing.T) {
	c, _ := newTestCP(t)
	entries, err := c.Load()
	if err != nil {
		t.Fatalf("load on missing file should be nil error, got %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

// T3 — 损坏 checksum：load ⇒ 丢弃，不 panic。
func TestOrphanCP_T3_CorruptChecksum(t *testing.T) {
	c, _ := newTestCP(t)
	c.MarkDirty([32]byte{0x01}, [32]byte{0xaa})
	if err := c.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// 翻转文件最后一个字节（位于 checksum 覆盖区内），制造 checksum 不匹配。
	data, _ := os.ReadFile(c.Path())
	data[len(data)-1] ^= 0xff
	if err := os.WriteFile(c.Path(), data, 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	_, err := c.Load()
	if err == nil {
		t.Fatalf("expected checksum error, got nil")
	}
	// 直接调用 parse 验证是 checksum 错误而非其他。
	if _, perr := parseOrphanCheckpoint(data); perr != errOrphanCPChecksum {
		t.Fatalf("expected errOrphanCPChecksum, got %v", perr)
	}
}

// T4 — version≠1：load ⇒ 忽略（丢弃，返回错误）。
func TestOrphanCP_T4_BadVersion(t *testing.T) {
	c, _ := newTestCP(t)
	c.MarkDirty([32]byte{0x01}, [32]byte{0xaa})
	if err := c.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	data, _ := os.ReadFile(c.Path())
	data[4] = 0x02 // 改 version
	if err := os.WriteFile(c.Path(), data, 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if _, err := c.Load(); err == nil {
		t.Fatalf("expected version error, got nil")
	}
}

// T5 — entryCount>256：load ⇒ 丢弃全文件（返回 bounds 错误）。
func TestOrphanCP_T5_EntryCountOverflow(t *testing.T) {
	// 手工构造 entryCount=257 的字节（checksum 需重算以命中 bounds 检查路径）。
	buf := make([]byte, 0)
	buf = append(buf, orphanCPMagic...)
	buf = append(buf, orphanCPVersion)
	var cnt [4]byte
	binary.BigEndian.PutUint32(cnt[:], 257)
	buf = append(buf, cnt[:]...)
	buf = append(buf, make([]byte, 32)...) // checksum 占位
	sum := sha256.Sum256(append(append([]byte{}, buf[4:9]...), buf[41:]...))
	copy(buf[9:9+32], sum[:])

	if _, err := parseOrphanCheckpoint(buf); err == nil {
		t.Fatalf("expected bounds error for entryCount=257, got nil")
	}
}

// T6 — childCount>64：load ⇒ 丢弃该 entry（其余保留）。
func TestOrphanCP_T6_ChildCountOverflow(t *testing.T) {
	// 构造两个 entry：第一个 childCount=65（越界，应丢弃），第二个合法（应保留）。
	buf := make([]byte, 0)
	buf = append(buf, orphanCPMagic...)
	buf = append(buf, orphanCPVersion)
	var cnt [4]byte
	binary.BigEndian.PutUint32(cnt[:], 2) // 2 entries
	buf = append(buf, cnt[:]...)
	buf = append(buf, make([]byte, 32)...) // checksum 占位

	// entry 1：parent=0x01，childCount=65
	buf = append(buf, bytes32(0x01)...)
	binary.BigEndian.PutUint32(cnt[:], 65)
	buf = append(buf, cnt[:]...)
	for i := 0; i < 65; i++ {
		buf = append(buf, bytes32(byte(0xa0+i))...)
	}

	// entry 2：parent=0x02，childCount=1（合法）
	buf = append(buf, bytes32(0x02)...)
	binary.BigEndian.PutUint32(cnt[:], 1)
	buf = append(buf, cnt[:]...)
	buf = append(buf, bytes32(0xbb)...)

	sum := sha256.Sum256(append(append([]byte{}, buf[4:9]...), buf[41:]...))
	copy(buf[9:9+32], sum[:])

	entries, err := parseOrphanCheckpoint(buf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 surviving entry, got %d", len(entries))
	}
	if entries[0].parentHash != [32]byte{0x02} {
		t.Fatalf("surviving entry should be parent=0x02, got %x", entries[0].parentHash)
	}
}

// T7 — parentHash 零值：load ⇒ 丢弃该 entry。
func TestOrphanCP_T7_ZeroParentHash(t *testing.T) {
	buf := make([]byte, 0)
	buf = append(buf, orphanCPMagic...)
	buf = append(buf, orphanCPVersion)
	var cnt [4]byte
	binary.BigEndian.PutUint32(cnt[:], 2)
	buf = append(buf, cnt[:]...)
	buf = append(buf, make([]byte, 32)...)

	// entry 1：parent=零值（丢弃）
	buf = append(buf, make([]byte, 32)...)
	binary.BigEndian.PutUint32(cnt[:], 1)
	buf = append(buf, cnt[:]...)
	buf = append(buf, bytes32(0xaa)...)

	// entry 2：parent=0x03（保留）
	buf = append(buf, bytes32(0x03)...)
	binary.BigEndian.PutUint32(cnt[:], 1)
	buf = append(buf, cnt[:]...)
	buf = append(buf, bytes32(0xbb)...)

	sum := sha256.Sum256(append(append([]byte{}, buf[4:9]...), buf[41:]...))
	copy(buf[9:9+32], sum[:])

	entries, err := parseOrphanCheckpoint(buf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(entries) != 1 || entries[0].parentHash != [32]byte{0x03} {
		t.Fatalf("expected only parent=0x03 entry, got %d entries", len(entries))
	}
}

// T8 — 删除幂等：remove(不存在) 无副作用。
func TestOrphanCP_T8_DeleteIdempotent(t *testing.T) {
	c, _ := newTestCP(t)
	c.MarkDirty([32]byte{0x01}, [32]byte{0xaa})

	// 检查内存投影（Load 会读盘，这里直接检查锁内 entries）。
	c.mu.Lock()
	if len(c.entries) != 1 {
		c.mu.Unlock()
		t.Fatalf("setup: %d entries", len(c.entries))
	}
	c.removeLocked([32]byte{0xff}) // 不存在 → 无副作用
	if len(c.entries) != 1 {
		c.mu.Unlock()
		t.Fatalf("remove(nonexistent) should be no-op, got %d entries", len(c.entries))
	}
	c.removeLocked([32]byte{0x01}) // 存在 → 删除
	if len(c.entries) != 0 {
		c.mu.Unlock()
		t.Fatalf("remove(existing) should clear, got %d entries", len(c.entries))
	}
	c.mu.Unlock()
}

// T9 — 原子写：flush 后无 .tmp 残留；正式文件可被重新 load。
func TestOrphanCP_T9_AtomicWrite(t *testing.T) {
	c, dir := newTestCP(t)
	c.MarkDirty([32]byte{0x01}, [32]byte{0xaa})
	if err := c.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	tmpPath := filepath.Join(dir, "orphan_waiting.bin.tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf(".tmp should not exist after successful rename, stat err=%v", err)
	}
	// 正式文件应存在且可解析。
	if _, err := os.Stat(c.Path()); err != nil {
		t.Fatalf("formal file missing: %v", err)
	}
	entries, err := c.Load()
	if err != nil || len(entries) != 1 {
		t.Fatalf("reload after flush: err=%v entries=%d", err, len(entries))
	}
}

// T9b — CleanupTmp 清理残留 .tmp。
func TestOrphanCP_T9b_CleanupTmp(t *testing.T) {
	c, dir := newTestCP(t)
	tmpPath := filepath.Join(dir, "orphan_waiting.bin.tmp")
	if err := os.WriteFile(tmpPath, []byte("garbage"), 0o600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	c.CleanupTmp()
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf(".tmp should be removed, stat err=%v", err)
	}
}

// bytes32 构造一个以 b 为首字节、其余为零的 [32]byte。
func bytes32(b byte) []byte {
	out := make([]byte, 32)
	out[0] = b
	return out
}
