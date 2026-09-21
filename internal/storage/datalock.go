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

// DirLock 数据目录的进程独占锁（R2：内核生命周期锁）。
//
// 锁的权威仲裁是操作系统内核锁：对 <datadir>/node.lock 的句柄获取排他锁
// （Windows LockFileEx / POSIX flock，见 datalock_windows.go / datalock_unix.go）。
// 锁随句柄存活：持有进程以任何方式终止（优雅退出、kill -9 / taskkill /F、崩溃、
// 断电）时锁由内核自动释放。由此保证：
//   - 活持有者在位：任何其他进程获取内核锁失败 → ErrDatadirLocked（互斥）
//   - 持有者死亡：残留的 node.lock 文件不再是屏障，下一个节点启动时检测到
//     内核锁空闲即接管（truncate 复写诊断信息）——无需人工清锁，且全程
//     不使用「PID 是否存活」判定（§4 明确禁止其作为接管依据）
//
// node.lock 文件本身仅承载诊断信息（pid、started_at）与 pid 归属校验（Release
// 时防误删他人锁的防御层），不再是锁语义的载体。接管时以 Truncate+覆写复用
// 同一文件（Unix 上避免 inode 分裂；Windows 无此问题）。
type DirLock struct {
	path string
	f    *os.File
	mu   sync.Mutex // 防并发 Release（节点本身不并发 Release，仅作防御）
}

// acquireRetries 是内核锁仲裁对象漂移（lockOwnerStable 不过）时的重试上限。
// 每次失败的唯一原因是持锁文件被并发接管者删除/重建，现实中常数次即收敛；
// 64 次远超任何合法交错，触顶说明环境异常，放弃并报错。
const acquireRetries = 64

// AcquireDirLock 独占锁定数据目录，准备初始化节点。
//
// 行为：
//   - 目录不存在则创建（0700）
//   - 打开/创建 node.lock（O_RDWR|O_CREATE，残留文件照常打开——是否可接管
//     由内核锁判定，绝不由文件存在性判定）
//   - 内核锁被其他活动进程持有 → 返回 ErrDatadirLocked（不覆盖、不截断、
//     不继续打开 blocks.dat）
//   - 内核锁空闲 → 接管（含首次创建与强杀残留两种情况），Truncate 后写入
//     诊断信息（pid、started_at）
//
// 调用方责任：
//   - 节点关闭时必须调用 (*DirLock).Release 释放
//   - 若初始化中途失败，返回错误前也必须 Release，避免留下本不需要的锁
func AcquireDirLock(dir string) (*DirLock, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	path := filepath.Join(dir, "node.lock")
	for i := 0; i < acquireRetries; i++ {
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
		if err != nil {
			return nil, fmt.Errorf("打开数据目录锁失败: %w", err)
		}
		if err := lockExclusive(f); err != nil {
			// 内核锁被持有：另一活进程在位。互斥语义与旧实现一致，但依据
			// 是内核锁而非文件存在性。
			_ = f.Close()
			return nil, ErrDatadirLocked
		}
		if !lockOwnerStable(f, path) {
			// 锁到的对象与路径当前指向不一致（接管窗口内被并发删除/重建）：
			// 关闭重开，仲裁对象必须与路径一致，否则将出现双持有者。
			_ = f.Close()
			continue
		}
		info := fmt.Sprintf("pid=%d\nstarted_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
		// Truncate 后覆写：首次创建为空文件（无操作）；接管强杀残留时复写
		// stale 内容，保证 Release 的 pid 归属校验读到本进程。
		if err := f.Truncate(0); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("重置锁信息失败: %w", err)
		}
		if _, err := f.WriteAt([]byte(info), 0); err != nil {
			// 写诊断信息失败：关闭句柄释放内核锁，并尽力撤销本次创建的文件，
			// 避免留下一个内容无法识别的死锁；remove 失败仅留残留（下次启动
			// 由内核锁接管），不掩盖主错误。
			_ = f.Close()
			_ = os.Remove(path)
			return nil, fmt.Errorf("写入锁信息失败: %w", err)
		}
		return &DirLock{path: path, f: f}, nil
	}
	return nil, fmt.Errorf("数据目录锁仲裁对象持续漂移，放弃获取: %s", path)
}

// Release 释放数据目录锁（释放内核锁并删除 node.lock）。
//
// 幂等：重复调用不会报错，也不会误删他人重新获取的锁。
//
// PID 校验（P3.1 防御层，保留）：删除前先校验 node.lock 内容记录的 pid 是否等于
// 本进程 pid，仅当匹配时才删除；不匹配（异常信号：本进程崩溃后该文件已被其他
// 进程接管复写）则跳过删除——绝不误删他人的锁。读取或解析失败时保守跳过删除。
// 注意：这是删除的防御闸，不是锁语义的载体——互斥与接管完全由内核锁仲裁，
// 本函数不做、也禁止做「PID 是否存活」的判活恢复（§4）。
//
// 删除顺序按平台（见 datalock_unix.go / datalock_windows.go 的 removeBeforeClose）：
//   - Unix：先删后关——持锁状态下删除目录项，防止「接管者锁住被删 inode +
//     第三方重建同名文件」的并存窗口；
//   - Windows：先关后删——句柄未关闭时文件无法删除（无 FILE_SHARE_DELETE）；
//     关闭后 remove 若失败，唯一原因是接管者已打开该文件（归属他人，本就不
//     应删除），静默接受并记日志。
func (l *DirLock) Release() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil { // 已释放（或从未持有）
		return nil
	}

	if !ownsLockFile(l.path) {
		// 锁内容 pid 不匹配本进程（或无法解析）——异常信号，只关锁不删文件。
		_ = l.f.Close()
		l.f = nil
		log.Printf("[datalock] node.lock 的 pid 不匹配本进程或无法解析，跳过删除（不误删他人锁）: %s", l.path)
		return nil
	}

	if removeBeforeClose() {
		if err := os.Remove(l.path); err != nil {
			_ = l.f.Close()
			l.f = nil
			return err
		}
		_ = l.f.Close() // 释放内核锁
		l.f = nil
		return nil
	}
	_ = l.f.Close() // 释放内核锁
	l.f = nil
	if err := os.Remove(l.path); err != nil {
		// Windows：接管者已打开文件时删除失败 = 文件归属他人，静默接受。
		log.Printf("[datalock] 释放后删除 node.lock 失败（通常为他人已接管）: %s: %v", l.path, err)
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
