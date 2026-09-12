package storage

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrDatadirLocked 表示数据目录已被另一个节点进程占用。
//
// 与「数据损坏 / 回放失败」语义必须独立：本错误意味着根本不应启动第二个实例，
// 而回放失败意味着链数据本身有问题——二者解法不同（前者拒绝启动、后者需修复数据），
// 因此调用方不得把它们混成同一种错误文案。
var ErrDatadirLocked = errors.New("data directory is already in use by another node process")

// DirLock 数据目录的进程独占锁。
//
// 通过 <datadir>/node.lock 以 O_CREATE|O_EXCL 独占创建实现：同一目录同一时刻只允许一个
// 节点进程持有。本阶段不实现 stale-lock 的自动恢复（§5 明确禁止），仅保证：
//   - 正常退出释放锁
//   - lock 已存在时明确拒绝启动（绝不覆盖、绝不截断、绝不继续打开 blocks.dat）
//
// 未来扩展点：异常退出残留的 lock 可加一个「检测 lock 内记录的 PID 是否已存活」的
// recover 流程，但需显式授权，不在本阶段范围内。
type DirLock struct {
	path string
	f    *os.File
	mu   sync.Mutex // 防并发 Release（节点本身不并发 Release，仅作防御）
}

// AcquireDirLock 独占锁定数据目录，准备初始化节点。
//
// 行为：
//   - 目录不存在则创建（0700）
//   - node.lock 不存在：成功创建并写入最小诊断信息（pid、started_at）
//   - node.lock 已存在：返回 ErrDatadirLocked（绝不覆盖已有 lock、绝不截断）
//
// 调用方责任：
//   - 节点关闭时必须调用 (*DirLock).Release 释放
//   - 若初始化中途失败，返回错误前也必须 Release，避免留下本不需要的锁
func AcquireDirLock(dir string) (*DirLock, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	path := filepath.Join(dir, "node.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, ErrDatadirLocked
		}
		return nil, fmt.Errorf("创建数据目录锁失败: %w", err)
	}
	info := fmt.Sprintf("pid=%d\nstarted_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	if _, err := f.WriteString(info); err != nil {
		// 写诊断信息失败：撤销本次创建的 lock，避免留下一个无法识别的死锁
		f.Close()
		os.Remove(path)
		return nil, fmt.Errorf("写入锁信息失败: %w", err)
	}
	return &DirLock{path: path, f: f}, nil
}

// Release 释放数据目录锁（关闭并删除 node.lock）。
//
// 幂等：重复调用不会报错，也不会误删他人重新获取的锁。
//
// PID 校验（§2.2 / §5）：删除前先校验 node.lock 内容记录的 pid 是否等于本进程 pid，
// 仅当匹配时才删除；不匹配（说明本进程崩溃后该同名锁已被另一个进程重新持有）则静默跳过，
// 绝不误删他人的锁。读取或解析失败时保守跳过删除——绝不删除无法确认归属的锁。
//
// 必须仅由成功持有该锁的进程调用。本阶段不实现 stale-lock 自动恢复，异常退出残留的
// lock 不会被静默删除；此处的 pid 校验是防御手段，不得被用作「检测后自动删除」的通道。
func (l *DirLock) Release() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil { // 已释放（或从未持有）
		return nil
	}
	// 先关闭文件句柄：Windows 下以 O_WRONLY 独占打开的文件在句柄未关闭时无法被读取。
	_ = l.f.Close()
	l.f = nil

	if !ownsLockFile(l.path) {
		// 锁内容 pid 不匹配本进程（或无法解析）——这是「他人/Crash 残留」信号，跳过删除。
		log.Printf("[datalock] node.lock 的 pid 不匹配本进程或无法解析，跳过删除（不误删他人锁）: %s", l.path)
		return nil
	}
	if err := os.Remove(l.path); err != nil {
		return err
	}
	return nil
}

// ownsLockFile 判断 path 处的 node.lock 是否由本进程持有（内容记录的 pid == 本进程 pid）。
// 文件不存在 / 无法读取 / 内容无法解析 pid 时均返回 false（保守：不确认归属就不删）。
func ownsLockFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	pid, err := parseLockPID(data)
	if err != nil {
		return false
	}
	return pid == os.Getpid()
}

// parseLockPID 从 node.lock 内容（形如 "pid=N\nstarted_at=...\n"）解析 pid 字段。
// 缺少 pid= 前缀、首行非整数、空内容均视为解析失败。
func parseLockPID(data []byte) (int, error) {
	s := string(data)
	if !strings.HasPrefix(s, "pid=") {
		return 0, fmt.Errorf("lock 内容缺少 pid= 前缀")
	}
	rest := s[len("pid="):]
	line := rest
	if i := strings.IndexByte(rest, '\n'); i >= 0 {
		line = rest[:i]
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, fmt.Errorf("lock 内容 pid 字段为空")
	}
	return strconv.Atoi(line)
}

// Path 返回锁文件路径（诊断/日志用）。
func (l *DirLock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}
