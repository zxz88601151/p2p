//go:build windows

package storage

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	modkernel32    = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx = modkernel32.NewProc("LockFileEx")
)

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002
)

// lockByteOffset 是锁区起始偏移：锁在 EOF 之外（诊断信息恒 < 4KB）。
//
// Windows 的 LockFileEx 区域锁是强制锁：任何句柄的 Read/WriteFile 跨入锁区
// 即 ERROR_LOCK_VIOLATION——包括本进程自己读回诊断信息（Release 的
// ownsLockFile → os.ReadFile）。若锁与数据区重叠，Acquire 一成功，锁文件
// 内容就再也读不出来，Release 的归属校验必然失效（实测教训）。
// 锁在 EOF 之外（文件内容恒小于该偏移）既保留互斥与「随句柄死亡自动释放」
// 的全部语义，又使文件数据区永远可读——Windows byte-range lock 的经典技法。
const lockByteOffset = 4096

// lockExclusive 对 f 的宿主文件按字节区间 [lockByteOffset, lockByteOffset+1)
// 获取内核级排他锁（kernel32 LockFileEx，
// LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY = 非阻塞排他）。
//
// 锁随句柄存活：持有进程以任何方式终止（优雅退出、taskkill /F、崩溃、断电）
// 时，锁由内核自动释放——这是 R2（SEC-CLOSE MUST FIX 3）「内核生命周期锁」
// 的核心性质：进程死亡即解锁，残留的锁文件不再是屏障。
//
// 平台事实（Go 1.22.5 syscall_windows.go:366）：os.OpenFile 以
// FILE_SHARE_READ|FILE_SHARE_WRITE 打开（无 FILE_SHARE_DELETE），因此本进程
// 持锁期间任何进程（含本进程）都无法删除该文件——NT 语义下不存在「锁住已删除
// inode」的分裂问题，lockOwnerStable 恒稳定。锁区可越过文件末尾（EOF 之外
// 加锁为 Windows 官方支持的用法）。
func lockExclusive(f *os.File) error {
	ol := new(syscall.Overlapped)
	ol.Offset = lockByteOffset // OffsetHigh=0：锁 EOF 之外 1 字节
	r1, _, errno := procLockFileEx.Call(
		f.Fd(), // HANDLE
		uintptr(lockfileFailImmediately|lockfileExclusiveLock),
		0,    // dwReserved
		1, 0, // nNumberOfBytesToLockLow / High
		uintptr(unsafe.Pointer(ol)),
	)
	if r1 == 0 {
		return errno
	}
	return nil
}

// lockOwnerStable：Windows 句柄即 NT 文件对象本体，同一路径的成功打开必然指向
// 同一对象，不存在「unlink 后重建换 inode」的分裂语义，恒稳定。
func lockOwnerStable(_ *os.File, _ string) bool { return true }

// removeBeforeClose：Windows 上必须「先关后删」——句柄未关闭时文件无法被删除
// （本进程与他人的句柄均无 FILE_SHARE_DELETE）。关闭句柄即释放内核锁；随后的
// remove 若失败，唯一可能是接管者已打开该文件，此时文件归属他人、本就不应
// 删除，由调用方静默接受（记日志）。
func removeBeforeClose() bool { return false }
