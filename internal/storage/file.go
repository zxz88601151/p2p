package storage

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"

	"p2pchain/internal/block"
)

// ErrCorruptStore 表示数据文件损坏（截断或解码失败）。
var ErrCorruptStore = errors.New("区块数据文件损坏")

// ErrReadOnlyStore 以只读方式打开的存储上执行了写操作。
var ErrReadOnlyStore = errors.New("存储以只读方式打开，不允许写入")

// FileBlockStore 基于单一追加文件（blocks.dat）的区块存储。
//
// 文件格式：连续的记录，分两种语义区（REORG-1E · M2）：
//
//   - legacy 区：`[u32 LE length][block.Encode()]`（高度 = 记录序号）；
//   - v2 区：REORG-1E 记录帧（见 frame.go），自首条 v2 记录起启用以哈希寻址、
//     TIP 提交、持久 UNDO、detached 分支与逻辑删除。
//
// 索引与区块本体均在打开时通过顺序扫描重建，**永不落盘**（F5）；UTXO 永不落盘，
// 启动从创世回放重建（REORG-1E 审计 §5 实测 0.52 ms/块）。
//
// 并发：所有公开方法受互斥保护。写入为追加语义，天然满足「只增不改」的链特性。
type FileBlockStore struct {
	mu       sync.RWMutex
	dir      string
	path     string
	file     *os.File // 可写句柄（只读打开时为 nil）
	rfile    *os.File // 只读句柄（可写打开时为 nil；供按偏移读取 UNDO 帧）
	readOnly bool

	// legacy / canonical 视图（与 REORG-1E 前的语义逐字节保持一致）
	byHeight []*block.Block   // 高度 → 区块（canonical 链）
	byHash   map[[32]byte]int // 哈希 → 高度

	// v2 语义状态（见 v2.go）
	v2 v2State
}

// OpenFileBlockStore 打开（或创建）目录下的区块存储，并重建索引（可写）。
func OpenFileBlockStore(dir string) (*FileBlockStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	path := filepath.Join(dir, "blocks.dat")
	s := &FileBlockStore{
		dir:    dir,
		path:   path,
		byHash: make(map[[32]byte]int),
	}
	s.initV2()

	// O_RDWR 而非 O_WRONLY：写入仍是 O_APPEND 追加语义，同时允许按偏移回读
	// 已落盘的 UNDO 帧（M4）；除此之外行为与 REORG-1E 之前完全一致。
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("打开区块数据文件失败: %w", err)
	}
	if err := s.loadLog(f); err != nil {
		f.Close()
		return nil, err
	}
	s.file = f
	return s, nil
}

// OpenFileBlockStoreReadOnly 只读打开已有区块存储（不创建文件，不持有写句柄）。
// 供离线命令（如 printchain / verify）在节点运行期间安全读取链数据。
//
// **严格拒绝任何损坏**（M7）：只读路径不做任何修复。
func OpenFileBlockStoreReadOnly(dir string) (*FileBlockStore, error) {
	path := filepath.Join(dir, "blocks.dat")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("区块数据文件不存在（%s）: %w", path, err)
	}
	s := &FileBlockStore{
		dir:      dir,
		path:     path,
		readOnly: true,
		byHash:   make(map[[32]byte]int),
	}
	s.initV2()
	if err := s.loadLog(f); err != nil {
		f.Close()
		return nil, err
	}
	s.rfile = f
	return s, nil
}

// SaveBlock 追加保存区块（legacy 语义）。
//
// 契约（REORG-1E Option A 决议，**逐字节不变**）：
//   - 在**纯 legacy 文件**上：行为与 REORG-1E 之前完全一致——写 `[u32 LE len][block.Encode()]`、
//     单次 fsync、高度 = 记录序号、父哈希必须存在（或为创世）。既有的 P0 回归测试
//     （重复记录必须写入成功 / 只读打开必须成功 / replay 阶段才拒绝）即由该路径保证。
//   - 一旦该 store 进入 **v2 模式**（出现首条 v2 记录）：SaveBlock 被拒绝，调用方必须
//     改用 AppendCanonicalBlock 或 SaveBlockWithUndo + CommitTip。理由：v2 语义下
//     canonical 区块必须带 UNDO（I1），而本 API 无从提供 UNDO——拒绝比静默产出
//     违反 I1 的 canonical 区块更安全。
func (s *FileBlockStore) SaveBlock(b *block.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file == nil {
		return ErrReadOnlyStore
	}
	if s.v2.v2Mode {
		return ErrLegacyAppendInV2Mode
	}

	expected := len(s.byHeight)
	if b.Header.PrevBlockHash != ([32]byte{}) && expected > 0 {
		// 非创世：父哈希必须命中已有区块，且高度连续
		if _, ok := s.byHash[b.Header.PrevBlockHash]; !ok {
			return fmt.Errorf("父区块 %x 不存在", b.Header.PrevBlockHash)
		}
	}

	encoded := b.Encode()
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(encoded)))
	record := append(lenBuf[:], encoded...)
	offset := s.v2.logSize
	if _, err := s.file.Write(record); err != nil {
		return fmt.Errorf("写入区块失败: %w", err)
	}
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("刷盘失败: %w", err)
	}

	hash := b.Header.Hash()
	s.byHeight = append(s.byHeight, b)
	s.byHash[hash] = expected

	// 统一索引 / legacy 记录序列 / 累积工作量的会话内记账（不改变任何对外语义）
	var cum *big.Int
	if w, werr := workOfBits(b.Header.Bits); werr == nil {
		s.v2.legacyCum = new(big.Int).Add(s.v2.legacyCum, w)
		cum = new(big.Int).Set(s.v2.legacyCum)
	} else {
		cum = new(big.Int).Set(s.v2.legacyCum)
	}
	s.v2.legacySeq = append(s.v2.legacySeq, b)
	s.v2.legacyLen = len(s.v2.legacySeq)
	s.v2.records[hash] = &blockRecord{
		hash: hash, height: expected, offset: offset, block: b,
		isV2: false, canonical: true, cumWork: cum, parent: b.Header.PrevBlockHash,
	}
	s.v2.logSize += int64(len(record))
	return nil
}

// GetBlockByHash 按哈希查询区块（含 v2 detached 分支；不含被逻辑删除者）。
func (s *FileBlockStore) GetBlockByHash(hash [32]byte) (*block.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.v2.v2Mode {
		if rec, ok := s.v2.records[hash]; ok && !rec.deleted {
			return rec.block, nil
		}
	}
	h, ok := s.byHash[hash]
	if !ok {
		return nil, ErrNotFound
	}
	return s.byHeight[h], nil
}

// GetBlockByHeight 按高度查询 canonical 链上的区块。
func (s *FileBlockStore) GetBlockByHeight(height int) (*block.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if height < 0 || height >= len(s.byHeight) {
		return nil, ErrNotFound
	}
	return s.byHeight[height], nil
}

// Height 返回当前 canonical 链尾高度；空库返回 -1。
//
// 纯 legacy 文件下等同于「记录数 - 1」（与 REORG-1E 之前一致）；
// v2 模式下等于 TIP 提交的 canonical 高度。
func (s *FileBlockStore) Height() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.v2.v2Mode {
		return s.v2.tipHeight, nil
	}
	return len(s.byHeight) - 1, nil
}

// Close 关闭底层文件句柄。
func (s *FileBlockStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var firstErr error
	if s.file != nil {
		if err := s.file.Close(); err != nil {
			firstErr = err
		}
		s.file = nil
	}
	if s.rfile != nil {
		if err := s.rfile.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		s.rfile = nil
	}
	return firstErr
}

// FilePath 返回数据文件路径（日志/诊断用）。
func (s *FileBlockStore) FilePath() string { return s.path }

// 编译期接口断言。
var _ BlockStore = (*FileBlockStore)(nil)
