//go:build !windows

package storage

import (
	"os"
	"syscall"
)

// lockExclusive 尝试对 f 的宿主文件获取非阻塞内核级排他咨询锁（POSIX flock）。
//
// 锁随打开的文件描述存活：持有进程以任何方式终止（优雅退出、kill -9、崩溃、
// 断电后的进程回收）时，锁由内核自动释放——这是 R2（SEC-CLOSE MUST FIX 3）
// 「内核生命周期锁」的核心性质：进程死亡即解锁，残留的锁文件不再是屏障。
//
// 返回 nil = 持锁成功；非 nil = 锁被其他活动进程持有（或调用失败，调用方
// 统一按「目录被占用」处理）。
func lockExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// lockOwnerStable 校验 f 当前锁定的文件仍是 path 指向的文件（fstat/stat 的
// inode 比对）。Unix 上「unlink 后 O_CREATE 重建」会产生新 inode：若无此校验，
// 接管窗口内可能有两个进程各自锁住不同 inode 却都以为自己持有 node.lock
// （锁仲裁对象分裂）。path 已不存在（stat 失败）同样视为不稳定。
func lockOwnerStable(f *os.File, path string) bool {
	var fs, ss syscall.Stat_t
	if err := syscall.Fstat(int(f.Fd()), &fs); err != nil {
		return false
	}
	if err := syscall.Stat(path, &ss); err != nil {
		return false
	}
	return fs.Ino == ss.Ino
}

// removeBeforeClose：Unix 上 Release 必须「先删后关」——持锁状态下先删除
// 目录项、再关闭句柄释放内核锁。若反过来（先关后删），在「关闭」与「删除」
// 之间会出现窗口：接管者 flock 成功（锁已释放）但其打开的 inode 随后被本进程
// 删除，第三方再 O_CREATE 重建同名文件并锁住新 inode——两个进程同时自认为
// 持有数据目录锁。「先删后关」保证删除只发生在本进程仍持锁（他人必然拿不到
// 该 inode 的锁）之时，配合 Acquire 侧的 lockOwnerStable 复核闭环消除该窗口。
func removeBeforeClose() bool { return true }
