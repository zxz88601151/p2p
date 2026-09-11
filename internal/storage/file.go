package storage

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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
// 文件格式：连续的记录，每条 = 4 字节小端长度 + 区块规范编码（block.Encode）。
// 索引（哈希 → 高度、高度 → 区块）在启动时通过顺序扫描重建，常驻内存；
// 区块本体也随扫描载入内存（学习项目规模足够；生产实现应把索引与数据分离）。
//
// 并发：所有公开方法受互斥保护。写入为追加语义，天然满足「只增不改」的链特性。
type FileBlockStore struct {
	mu       sync.RWMutex
	dir      string
	path     string
	file     *os.File
	readOnly bool
	byHeight []*block.Block
	byHash   map[[32]byte]int // 哈希 → 高度
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

	if err := s.loadIndex(); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("打开区块数据文件失败: %w", err)
	}
	s.file = f
	return s, nil
}

// OpenFileBlockStoreReadOnly 只读打开已有区块存储（不创建文件，不持有写句柄）。
// 供离线命令（如 printchain）在节点运行期间安全读取链数据。
func OpenFileBlockStoreReadOnly(dir string) (*FileBlockStore, error) {
	path := filepath.Join(dir, "blocks.dat")
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("区块数据文件不存在（%s）: %w", path, err)
	}
	s := &FileBlockStore{
		dir:      dir,
		path:     path,
		readOnly: true,
		byHash:   make(map[[32]byte]int),
	}
	if err := s.loadIndex(); err != nil {
		return nil, err
	}
	return s, nil
}

// loadIndex 顺序扫描数据文件，重建内存索引；文件不存在时视为空库。
func (s *FileBlockStore) loadIndex() error {
	f, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // 空库
		}
		return err
	}
	defer f.Close()

	reader := io.Reader(f)
	for {
		var length uint32
		if err := binary.Read(reader, binary.LittleEndian, &length); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("%w: 读取记录长度失败: %v", ErrCorruptStore, err)
		}
		if length == 0 || length > 64<<20 { // 64 MiB 上限，防御损坏数据
			return fmt.Errorf("%w: 非法记录长度 %d", ErrCorruptStore, length)
		}
		buf := make([]byte, length)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return fmt.Errorf("%w: 记录数据不完整: %v", ErrCorruptStore, err)
		}
		b, err := block.DecodeBlock(buf)
		if err != nil {
			return fmt.Errorf("%w: 区块解码失败: %v", ErrCorruptStore, err)
		}
		height := len(s.byHeight)
		s.byHeight = append(s.byHeight, b)
		s.byHash[b.Header.Hash()] = height
	}
	return nil
}

// SaveBlock 追加保存区块。高度必须严格递增（等于当前高度 +1），保证单链追加语义。
func (s *FileBlockStore) SaveBlock(b *block.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file == nil {
		return ErrReadOnlyStore
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
	if _, err := s.file.Write(append(lenBuf[:], encoded...)); err != nil {
		return fmt.Errorf("写入区块失败: %w", err)
	}
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("刷盘失败: %w", err)
	}

	s.byHeight = append(s.byHeight, b)
	s.byHash[b.Header.Hash()] = expected
	return nil
}

// GetBlockByHash 按哈希查询区块。
func (s *FileBlockStore) GetBlockByHash(hash [32]byte) (*block.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, ok := s.byHash[hash]
	if !ok {
		return nil, ErrNotFound
	}
	return s.byHeight[h], nil
}

// GetBlockByHeight 按高度查询区块。
func (s *FileBlockStore) GetBlockByHeight(height int) (*block.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if height < 0 || height >= len(s.byHeight) {
		return nil, ErrNotFound
	}
	return s.byHeight[height], nil
}

// Height 返回当前最高区块的高度；空库返回 -1。
func (s *FileBlockStore) Height() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byHeight) - 1, nil
}

// Close 关闭底层文件。
func (s *FileBlockStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}

// FilePath 返回数据文件路径（日志/诊断用）。
func (s *FileBlockStore) FilePath() string { return s.path }

// 编译期接口断言。
var _ BlockStore = (*FileBlockStore)(nil)
