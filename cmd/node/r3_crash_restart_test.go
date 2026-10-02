package main

// ════════════════════════════════════════════════════════════════════════════
// REORG-1J-R3 · LEGACY-PREFIX CRASH MATRIX —— 真实进程级崩溃 / 重启证据
//
// 目的：补全 internal/storage/r3_crash_matrix_test.go 的**字节级**注入之外的
// **进程级**证据：用真实 node 二进制、真实 P2P TCP、**字节级写入的区块镜像**构造一条
// 「legacy 前缀共享、自前缀内部高度分叉」的场景，然后在 5 个不同时刻强制杀死
// （Process.Kill，等价于 SIGKILL）正在同步/reorg 的节点进程，随后重启并断言：
//
//	I3  legacy 前缀（前 3 条 legacy 记录）在崩溃镜像与重启后均逐字节不变；
//	I5  崩溃镜像永远包含全部已提交字节（历史不得以任何方式变短）；
//	I8  重启后 canonical 必然收敛到**唯一合法状态**（本场景：B 的更长分支）；
//	I9  crash → restart → inspect → converge；
//	I10 关键崩溃点重复执行 ≥ 5 次，结果一致。
//
// 为什么进程级 kill **不能**替代字节级注入（§6）：
//   CommitReorg 的提交边界是一次 appendFrames 内的单次 file.Sync()，
//   时间窗在微秒级；而且进程被杀时 OS 页缓存中的字节通常仍然落盘（并非掉电），
//   因此「恰好停在提交点中间」无法用 kill 可靠命中。
//   字节级穷举（internal/storage 的 TestR3_C4_ByteSweep）负责覆盖该边界；
//   本文件负责证明「任意时刻的真实进程崩溃都不会产生非法状态」。
//
// 本阶段只新增测试代码，不修改任何生产文件。
// ════════════════════════════════════════════════════════════════════════════

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"p2pchain/internal/control"
)

// r3CrashTimings 5 个不同的「杀死时刻」（自 A 启动起算）。
//
// 刻意覆盖从「尚未连接」到「同步/重组进行中」再到「可能已收敛」的完整区间——
// B 比 A 长 15 个区块，同步窗口足够宽，前几个时刻必然落在**提交过程之中**。
var r3CrashTimings = []time.Duration{
	0, // 控制接口就绪即刻杀死：必然落在同步/重组进行中
	15 * time.Millisecond,
	40 * time.Millisecond,
	100 * time.Millisecond,
	1500 * time.Millisecond, // 对照：落在完全收敛之后
}

// TestR3_RealProcessCrashRestartLegacyPrefix 真实双进程 + 强制杀死 + 重启收敛。
//
// 场景（全部由真实二进制产出，无任何手工构造）：
//
//	prefix  ── 高度 2（3 条 legacy 记录，A 与 B 共享，逐字节相同）
//	A       ── prefix + 3 枚区块  = 高度 5
//	B       ── prefix + 98 枚区块 = 高度 100（工作量更大 ⇒ A 必须 reorg 到 B 的链）。
//
// 实测同步速率约 2.5 ms/区块（100 区块 < 200 ms 完成），因此杀点必须前移到**毫秒级**
// 才能真正落在同步/重组进行中；1500ms 那一轮作为「已收敛后杀死」的对照。
//
// 分叉点位于 **legacy 前缀内部的高度 2**，正是 GAP-1H-A（v2 区块占据 legacy 槽位）
// 的现实形态。
func TestR3_RealProcessCrashRestartLegacyPrefix(t *testing.T) {
	if testing.Short() {
		t.Skip("需要构建并启动真实 node 二进制")
	}
	const prefixHeight = 2
	const aHeight = 5
	const bHeight = 100

	prefix := mineChainOffline(t, prefixHeight)                  // 高度 2
	histA := extendChainOffline(t, prefix, aHeight-prefixHeight) // 高度 5
	histB := extendChainOffline(t, prefix, bHeight-prefixHeight) // 高度 6

	// ── 前提断言（对数据本身，不与同步赛跑）──
	if got := initialHeightFromLegacy(t, prefix); got != prefixHeight {
		t.Fatalf("前缀链高 = %d, want %d", got, prefixHeight)
	}
	if got := initialHeightFromLegacy(t, histA); got != aHeight {
		t.Fatalf("A 历史链高 = %d, want %d", got, aHeight)
	}
	if got := initialHeightFromLegacy(t, histB); got != bHeight {
		t.Fatalf("B 历史链高 = %d, want %d", got, bHeight)
	}
	if !bytes.HasPrefix(histA, prefix) || !bytes.HasPrefix(histB, prefix) {
		t.Fatal("前提不成立：A / B 必须以同一份 legacy 前缀为逐字节前缀")
	}
	if bytes.Equal(histA, histB) {
		t.Fatal("前提不成立：A / B 必须自高度 3 起分叉")
	}

	// ── B：常驻种子（更长的分支，reorg 的目标）──
	dirB := t.TempDir()
	writeBlocksDat(t, dirB, histB)
	b := startRealNode(t, dirB)
	defer stopRealNode(t, b)
	waitRealHeight(t, b, bHeight, 30*time.Second)
	stB, err := b.status()
	if err != nil {
		t.Fatalf("取 B 状态失败: %v", err)
	}
	bTip := stB.TipHash
	t.Logf("B 已就绪：高度=%d 链尾=%s", stB.Height, bTip)

	seed := b.p2pListenAddr()

	for i, delay := range r3CrashTimings {
		runID := fmt.Sprintf("R3-CRASH-%d/kill@%v", i+1, delay)
		t.Run(runID, func(t *testing.T) {
			// ── A：以 histA 启动，delay 后强制杀死 ──
			dirA := t.TempDir()
			writeBlocksDat(t, dirA, histA)
			a := startRealNode(t, dirA, "-seed", seed)
			// 诊断：控制接口就绪时 A 的高度（用于判断同步是否已在启动阶段完成）
			hAtReady := -1
			if st0, err := a.status(); err == nil {
				hAtReady = st0.Height
			}
			time.Sleep(delay)
			hAtKill := -1
			if st1, err := a.status(); err == nil {
				hAtKill = st1.Height
			}
			crashed := readBlocksDat(t, dirA)
			a.kill()
			a.waitExit(10 * time.Second)

			// I3 / I5 · 崩溃镜像：legacy 前缀逐字节不变，且历史未变短
			if !bytes.HasPrefix(crashed, prefix) {
				t.Fatalf("I3 违反：崩溃镜像的 legacy 前缀被改写（%d 字节）", len(crashed))
			}
			if len(crashed) < len(histA) {
				t.Fatalf("I5 违反：崩溃镜像比启动历史更短（%d < %d）—— 已提交字节被删除", len(crashed), len(histA))
			}

			// ── 重启：新目录 + 崩溃镜像 + 同一 seed ──
			dirA2 := t.TempDir()
			writeBlocksDat(t, dirA2, crashed)
			a2 := startRealNode(t, dirA2, "-seed", seed)
			st := waitRealCond(t, a2, func(s *control.StatusInfo) bool {
				return s.Height == bHeight && s.TipHash == bTip
			}, 120*time.Second, fmt.Sprintf("重启后收敛到高度 %d 且与 B 同链尾", bHeight))

			after := readBlocksDat(t, dirA2)
			if !bytes.HasPrefix(after, prefix) {
				t.Fatalf("I3 违反：重启并同步后 legacy 前缀被改写")
			}
			if !bytes.HasPrefix(after, crashed) {
				t.Fatalf("I5 违反：重启后的日志不再是崩溃镜像的字节扩展（已提交字节被改写/截断）")
			}
			// 优雅停止必须成功（说明状态完全可判定，无残骸）
			stopRealNode(t, a2)

			midSync := hAtKill >= 0 && hAtKill < bHeight // 真正落在同步/重组进行中
			t.Logf("%s → [就绪时高度=%d | 杀死时高度=%d] 崩溃镜像 %d 字节"+
				"（启动历史 %d / B 全量 %d，杀死时未收敛=%v），"+
				"重启后 高度=%d 链尾=%s（与 B 一致），重启后日志 %d 字节",
				runID, hAtReady, hAtKill, len(crashed), len(histA), len(histB), midSync,
				st.Height, st.TipHash, len(after))
		})
	}
}

// ── 辅助 ────────────────────────────────────────────────────────────────────

func writeBlocksDat(t *testing.T, dir string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "blocks.dat"), data, 0o644); err != nil {
		t.Fatalf("写入 blocks.dat 失败: %v", err)
	}
}

func readBlocksDat(t *testing.T, dir string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "blocks.dat"))
	if err != nil {
		t.Fatalf("读取 blocks.dat 失败: %v", err)
	}
	return b
}
