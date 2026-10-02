// orphan_checkpoint.go 实现孤儿等待检查点（ORPHAN-DURABILITY-SPEC-v1）。
//
// 职责边界（SPEC §1 DATA MODEL + §2 PERSISTENCE POLICY）：
//   - 本文件**只**实现检查点的数据格式、序列化/反序列化、校验、原子写原语；
//   - 它**不**触碰 canonical 链 / UTXO / fork-choice / reorg / mempool / P2P 协议；
//   - 它**不**与 nodeService 的任何运行时状态耦合（挂钩在后续阶段另行接线）。
//
// 数据模型（SPEC §1.1，冻结）：
//
//	文件：<datadir>/orphan_waiting.bin
//	头部（41 字节）：magic "ORPH"(4B) + version 0x01(1B) + entryCount uint32 大端(4B)
//	                + checksum SHA-256(32B，覆盖 version+entryCount+全部 entry 字节，不含 magic)
//	entry（36 + 32×childCount 字节）：parentHash(32B) + childCount uint32 大端(4B) + childHash[] 32B×N
//
// 大小上界（SPEC §1.2）：entryCount ≤ 256，每 entry childCount ≤ 64，总 childHash ≤ 16,384，
// 文件 ≤ 16,384×32 + 41 ≈ 524,329 B。
//
// 原子写（SPEC §2.2）：temp + fsync + os.Rename。
//
// 失败行为（SPEC §2.4）：任何失败只影响孤儿可用性，绝不反向影响 canonical。
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// checkpoint 文件格式常量（SPEC §1.1，冻结）。
const (
	// orphanCPMagic 检查点文件的魔数。
	orphanCPMagic = "ORPH"
	// orphanCPVersion 当前检查点格式版本（SPEC v1）。
	orphanCPVersion = 0x01
	// orphanCPHeaderSize magic(4) + version(1) + entryCount(4) + checksum(32)。
	orphanCPHeaderSize = 4 + 1 + 4 + 32
	// orphanCPEntryHeaderSize parentHash(32) + childCount(4)。
	orphanCPEntryHeaderSize = 32 + 4
	// orphanCPMaxEntryCount 对应 maxWaitingBlocks（不同父哈希个数上限）。
	orphanCPMaxEntryCount = 256
	// orphanCPMaxChildrenPerEntry 对应 maxWaitingChildrenPerParent（每父子块数上限）。
	orphanCPMaxChildrenPerEntry = 64
	// orphanCPFlushInterval 周期 flush 最小间隔（§4-B1.5 D2）：脏状态才写，文件极小，5s 足够。
	orphanCPFlushInterval = 5 * time.Second
)

// orphanCPEntry 是检查点中的一个「父哈希 → 子块哈希列表」条目。
// childHash 保持入队序（SPEC §1.3）；entries 保持插入序（追加序）。
type orphanCPEntry struct {
	parentHash [32]byte
	childHash  [][32]byte
}

// orphanCheckpoint 是孤儿等待检查点的持久化句柄。
//
// 锁纪律（SPEC §5.3 R3）：本类型的 mu 独立于 nodeService.mu。调用方约定：
// nodeService.mu 临界区内**不得**调用本类型的任何加锁方法；markDirty/remove/flush
// 均在 nodeService.mu 之外调用。本文件只提供数据层，不含任何 nodeService 引用。
type orphanCheckpoint struct {
	mu      sync.Mutex
	path    string // <datadir>/orphan_waiting.bin
	tmpPath string // <datadir>/orphan_waiting.bin.tmp

	// entries 是内存投影（插入序），是持久层唯一事实源的镜像。
	entries   []orphanCPEntry
	dirty     bool
	lastFlush time.Time
}

// newOrphanCheckpoint 构造一个指向 <datadir>/orphan_waiting.bin 的检查点句柄。
// 不读盘、不创建文件；调用方随后显式 load 或直接以空态 flush。
func newOrphanCheckpoint(dataDir string) *orphanCheckpoint {
	return &orphanCheckpoint{
		path:    filepath.Join(dataDir, "orphan_waiting.bin"),
		tmpPath: filepath.Join(dataDir, "orphan_waiting.bin.tmp"),
	}
}

// ---------------------------------------------------------------------------
// 序列化（SPEC §1.1）
// ---------------------------------------------------------------------------

// serializeLocked 把 entries 序列化为完整文件字节（含 magic/version/entryCount/checksum）。
// 调用方须持 mu。序列化是确定性的：entries 顺序 = 追加序，childHash 顺序 = 入队序。
func (c *orphanCheckpoint) serializeLocked() []byte {
	buf := make([]byte, 0, orphanCPHeaderSize+len(c.entries)*orphanCPEntryHeaderSize)
	buf = append(buf, orphanCPMagic...)
	buf = append(buf, orphanCPVersion)

	// entryCount 占位（4B，稍后回填）。
	entryCountOff := len(buf)
	buf = append(buf, 0, 0, 0, 0)

	// checksum 占位（32B，稍后回填）。checksum 覆盖 version..entries 全部字节（不含 magic）。
	checksumOff := len(buf)
	buf = append(buf, make([]byte, 32)...)

	for i := range c.entries {
		e := &c.entries[i]
		buf = append(buf, e.parentHash[:]...)
		var cnt [4]byte
		binary.BigEndian.PutUint32(cnt[:], uint32(len(e.childHash)))
		buf = append(buf, cnt[:]...)
		for j := range e.childHash {
			buf = append(buf, e.childHash[j][:]...)
		}
	}

	// 回填 entryCount。
	binary.BigEndian.PutUint32(buf[entryCountOff:entryCountOff+4], uint32(len(c.entries)))

	// checksum 覆盖 version+entryCount+全部 entry 字节，即 buf[4:] 中**排除** checksum 字段自身
	// （checksumOff..checksumOff+32）。不含 magic 前 4 字节、不含 checksum 字段。
	sum := sha256.Sum256(append(append([]byte{}, buf[4:checksumOff]...), buf[checksumOff+32:]...))
	copy(buf[checksumOff:checksumOff+32], sum[:])
	return buf
}

// ---------------------------------------------------------------------------
// 反序列化 + 校验（SPEC §1.2 + §3.2）
// ---------------------------------------------------------------------------

// orphanCheckpointLoad 的校验错误（fail-closed：任何错误都令整个文件被丢弃）。
var (
	errOrphanCPBadMagic   = errors.New("orphan checkpoint: bad magic")
	errOrphanCPBadVersion = errors.New("orphan checkpoint: unsupported version")
	errOrphanCPChecksum   = errors.New("orphan checkpoint: checksum mismatch")
	errOrphanCPTruncated  = errors.New("orphan checkpoint: truncated")
	errOrphanCPBounds     = errors.New("orphan checkpoint: bounds exceeded")
)

// loadLocked 从磁盘读取并校验检查点文件，返回恢复出的父键集（parentHash 列表，插入序）
// 与每个父键对应的子块哈希列表。调用方须持 mu（load 不写 c 的字段，但统一走同一锁保证路径一致）。
//
// 失败行为（SPEC §3.2）：文件不存在返回空（不报错）；magic/version/checksum/截断/越界一律
// 返回错误，由调用方「丢弃文件、空启动」，**绝不 panic**。
func (c *orphanCheckpoint) loadLocked() ([]orphanCPEntry, error) {
	data, err := os.ReadFile(c.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil // 无检查点 = 空（SPEC §3.2）
		}
		return nil, fmt.Errorf("orphan checkpoint: read: %w", err)
	}
	entries, err := parseOrphanCheckpoint(data)
	if err != nil {
		return nil, err
	}
	// §4-B1.5 D2 修正：把磁盘投影灌入内存唯一事实源，使后续 MarkDirty/Remove/Flush
	// 在已加载条目的基础上增量合并，而非从空态覆盖（否则首次 Flush 会清空已持久的恢复集）。
	c.entries = entries
	c.dirty = false
	return entries, nil
}

// parseOrphanCheckpoint 解析并校验一段检查点字节。纯函数，供 load 与单测直接使用。
func parseOrphanCheckpoint(data []byte) ([]orphanCPEntry, error) {
	if len(data) < orphanCPHeaderSize {
		return nil, errOrphanCPTruncated
	}
	if string(data[0:4]) != orphanCPMagic {
		return nil, errOrphanCPBadMagic
	}
	if data[4] != orphanCPVersion {
		return nil, fmt.Errorf("%w: 0x%02x", errOrphanCPBadVersion, data[4])
	}
	entryCount := binary.BigEndian.Uint32(data[5:9])
	if entryCount > orphanCPMaxEntryCount {
		return nil, fmt.Errorf("%w: entryCount=%d", errOrphanCPBounds, entryCount)
	}

	// checksum 覆盖 data[4:41]（version+entryCount）与 data[41:]（entries），
	// 即 data[4:] 中**排除** checksum 字段自身（data[9:41]）。
	want := sha256.Sum256(append(append([]byte{}, data[4:9]...), data[41:]...))
	var got [32]byte
	copy(got[:], data[9:9+32])
	if got != want {
		return nil, errOrphanCPChecksum
	}

	entries := make([]orphanCPEntry, 0, entryCount)
	off := orphanCPHeaderSize
	for i := uint32(0); i < entryCount; i++ {
		if off+orphanCPEntryHeaderSize > len(data) {
			return nil, errOrphanCPTruncated
		}
		var e orphanCPEntry
		copy(e.parentHash[:], data[off:off+32])
		childCount := binary.BigEndian.Uint32(data[off+32 : off+36])
		off += orphanCPEntryHeaderSize

		// SPEC §3.2：childCount==0 或 >64 丢弃该 entry（其余保留）。
		if childCount == 0 || childCount > orphanCPMaxChildrenPerEntry {
			// 跳过该 entry 的 childHash 区块，继续解析后续 entry。
			if uint64(off)+uint64(childCount)*32 > uint64(len(data)) {
				return nil, errOrphanCPTruncated
			}
			off += int(childCount) * 32
			continue
		}
		if uint64(off)+uint64(childCount)*32 > uint64(len(data)) {
			return nil, errOrphanCPTruncated
		}

		// SPEC §3.2：parentHash 零值丢弃该 entry。
		if e.parentHash == ([32]byte{}) {
			off += int(childCount) * 32
			continue
		}

		e.childHash = make([][32]byte, 0, childCount)
		for j := uint32(0); j < childCount; j++ {
			var ch [32]byte
			copy(ch[:], data[off:off+32])
			off += 32
			e.childHash = append(e.childHash, ch)
		}
		entries = append(entries, e)
	}

	// 尾随字节（若有）容忍忽略（前向兼容占位；SPEC 未禁止）。
	return entries, nil
}

// ---------------------------------------------------------------------------
// 内存投影更新（SPEC §2.1 写触发；本数据层只提供投影操作，不触发磁盘 I/O）
// ---------------------------------------------------------------------------

// markDirtyLocked 记录「某 parent 键新增了一个 childHash」。
// 若 parent 已存在则追加 childHash；否则新建 entry（追加序）。置 dirty。
// 调用方须持 mu。
func (c *orphanCheckpoint) markDirtyLocked(parentHash, childHash [32]byte) {
	for i := range c.entries {
		if c.entries[i].parentHash == parentHash {
			// 去重（对齐 parkedHashes 语义：同一 childHash 不重复记录）。
			for _, ch := range c.entries[i].childHash {
				if ch == childHash {
					return
				}
			}
			c.entries[i].childHash = append(c.entries[i].childHash, childHash)
			c.dirty = true
			return
		}
	}
	c.entries = append(c.entries, orphanCPEntry{parentHash: parentHash, childHash: [][32]byte{childHash}})
	c.dirty = true
}

// removeLocked 删除某 parent 键的 entry（父到达）。不存在则无副作用（SPEC §5.3 T8 幂等）。
// 调用方须持 mu。
func (c *orphanCheckpoint) removeLocked(parentHash [32]byte) {
	for i := range c.entries {
		if c.entries[i].parentHash == parentHash {
			c.entries = append(c.entries[:i], c.entries[i+1:]...)
			c.dirty = true
			return
		}
	}
}

// ---------------------------------------------------------------------------
// 原子写（SPEC §2.2/§2.3）
// ---------------------------------------------------------------------------

// flushLocked 将当前投影序列化并原子写盘（temp + fsync + rename）。调用方须持 mu。
// 返回 error 表示写失败（SPEC §2.4：调用方仅 warning，不退化为错误，孤儿功能继续）。
func (c *orphanCheckpoint) flushLocked() error {
	data := c.serializeLocked()

	// 写临时文件。
	tmp, err := os.OpenFile(c.tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("orphan checkpoint: open tmp: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("orphan checkpoint: write tmp: %w", err)
	}
	if err := tmp.Sync(); err != nil { // fsync（SPEC §2.3：仅 flush 时）
		_ = tmp.Close()
		return fmt.Errorf("orphan checkpoint: fsync tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("orphan checkpoint: close tmp: %w", err)
	}
	if err := os.Rename(c.tmpPath, c.path); err != nil {
		return fmt.Errorf("orphan checkpoint: rename: %w", err)
	}
	c.dirty = false
	c.lastFlush = time.Now()
	return nil
}

// cleanupTmpLocked 清理可能残留的 .tmp（崩溃半写遗留）。调用方须持 mu。
// 幂等：不存在则无副作用。仅供启动时调用。
func (c *orphanCheckpoint) cleanupTmpLocked() {
	_ = os.Remove(c.tmpPath)
}

// shouldFlushLocked 判定是否满足批写触发条件（SPEC §2.1：dirty 变更 ≥ N 或距上次 ≥ T）。
// 本数据层提供判定，flush 时机由后续 service 接线决定。
// 调用方须持 mu。
func (c *orphanCheckpoint) shouldFlushLocked(now time.Time, nDirty int, interval time.Duration) bool {
	if !c.dirty {
		return false
	}
	if nDirty >= 64 {
		return true
	}
	if !c.lastFlush.IsZero() && now.Sub(c.lastFlush) >= interval {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// 对外加锁包装（供后续 service 接线使用；本 §2 阶段可被单测直接覆盖）
// ---------------------------------------------------------------------------

// Load 返回恢复出的 entries（父键 + 子哈希列表）。文件不存在返回空。
func (c *orphanCheckpoint) Load() ([]orphanCPEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loadLocked()
}

// MarkDirty 记录 parent 新增 childHash。
func (c *orphanCheckpoint) MarkDirty(parentHash, childHash [32]byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.markDirtyLocked(parentHash, childHash)
}

// Remove 删除 parent 键。
func (c *orphanCheckpoint) Remove(parentHash [32]byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeLocked(parentHash)
}

// Flush 原子写盘。
func (c *orphanCheckpoint) Flush() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.flushLocked()
}

// FlushIfDirty 在脏状态下按最小间隔节流落盘（§4-B1.5 D2 周期触发入口）。
//
// 语义：
//   - dirty 为假 → 跳过（返回 false, nil），不写盘；
//   - 距上次成功 flush 不足 interval（且非首次：lastFlush 非零）→ 跳过，避免无谓写盘；
//   - 否则原子写盘（temp+fsync+rename），返回 true。
//
// 调用方须持 mu（由本包装负责）；error 仅 warning，不退化为错误（孤儿功能继续）。
func (c *orphanCheckpoint) FlushIfDirty(interval time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty {
		return false, nil
	}
	if !c.lastFlush.IsZero() && time.Since(c.lastFlush) < interval {
		return false, nil
	}
	if err := c.flushLocked(); err != nil {
		return false, err
	}
	return true, nil
}

// CleanupTmp 清理 .tmp。
func (c *orphanCheckpoint) CleanupTmp() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cleanupTmpLocked()
}

// Path 返回正式文件路径（供观测/诊断）。
func (c *orphanCheckpoint) Path() string { return c.path }
