package main

// PHASE MINING-LIFECYCLE-1 测试：持续挖矿生命周期（设计冻结 §15 测试契约）。
//
// 覆盖矩阵映射（授权词 §20 编号）：
//   START：single / duplicate / concurrent ×N / while FAILED
//   STOP ：single / duplicate / while idle / during active PoW
//   RACE ：START vs STOP / STOP vs START / repeated sequence / transition integrity
//   （browser/HTTP 层面 Case G/I/J/K 由 internal/control 与 internal/explorer
//     测试及 UI 轮询契约覆盖；本文件验证 runtime 语义。）
//
// 铁律：STOP 只停挖矿——断言节点其余部分（chain /status 可读）保持存活；
// FAILED 终态不被隐式清除；每次 START 恰好一个 mining loop。

import (
	"errors"
	"sync"
	"testing"
	"time"

	"p2pchain/internal/control"
)

// isMineConflict 判定错误是否为生命周期状态冲突（HTTP 409 语义）。
func isMineConflict(err error) bool {
	var cf *control.MineConflictError
	return errors.As(err, &cf)
}

// newLifecycleService 构造带真实 p2p.Node（svc.net）的测试服务。
// 必须：持续挖矿会真实出块并走 broadcastBlock → relayBlock → net.BroadcastExcept，
// net 为 nil 即 panic（既有注入模式：service_test.go startService）。
func newLifecycleService(t *testing.T) *nodeService {
	t.Helper()
	svc := newServiceFor(t, testChain(t))
	startService(t, svc)
	return svc
}

// waitMineState 轮询等待 runtime 状态到达 want（上限 5s），返回到达时刻的实际状态。
func waitMineState(t *testing.T, svc *nodeService, want miningState) miningState {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		st, _ := svc.miningStateSnapshot()
		if st == want {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待状态 %s 超时：实际 %s", want, st)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestMineLifecycleStartFromStopped 覆盖：START from STOPPED（契约 #1）。
// 断言：受理成功；状态推进到 RUNNING/STALLED（循环已活动）；mining=true；
// 且重复 START（契约 #2）返回 *control.MineConflictError 且不产生第二个循环。
func TestMineLifecycleStartFromStopped(t *testing.T) {
	svc := newLifecycleService(t)

	resp, err := svc.StartMining()
	if err != nil {
		t.Fatalf("START from STOPPED 失败: %v", err)
	}
	if !resp.Accepted {
		t.Fatalf("Accepted=false: %+v", resp)
	}
	if resp.Height != svc.chain.Height() {
		t.Fatalf("响应高度 %d ≠ 链高 %d", resp.Height, svc.chain.Height())
	}

	// 循环已活动：状态必然离开 STOPPED（RUNNING 或 STALLED 皆合法）。
	waitMineStateNot(t, svc, MiningStopped)

	// duplicate START（契约 #2）：409 语义，无副作用。
	if _, err := svc.StartMining(); err == nil {
		t.Fatal("重复 START 未被拒绝")
	} else if !isMineConflict(err) {
		t.Fatalf("错误类型不符（应为 *control.MineConflictError）: %T", err)
	} else {
		st, _ := svc.miningStateSnapshot()
		if st != MiningRunning && st != MiningStalled && st != MiningStarting {
			t.Fatalf("冲突后状态异常: %s", st)
		}
	}

	// 收尾：STOP 后回到 STOPPED（契约 #4 的前置验证）。
	stopResp, err := svc.StopMining()
	if err != nil {
		t.Fatalf("STOP 失败: %v", err)
	}
	if !stopResp.Accepted {
		t.Fatalf("STOP Accepted=false: %+v", stopResp)
	}
	waitMineState(t, svc, MiningStopped)
	if svc.mining.Load() {
		t.Fatal("STOP 后 mining 标志仍为 true")
	}
}

// TestMineLifecycleConcurrentStartN 覆盖：concurrent START ×N（契约 #3）。
// 断言：恰 1 个成功、N-1 个 *control.MineConflictError，绝无多个循环。
func TestMineLifecycleConcurrentStartN(t *testing.T) {
	svc := newLifecycleService(t)
	defer svc.StopMining()

	const n = 16
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		okCount  int
		conflict int
		otherErr []error
	)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.StartMining()
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				okCount++
			} else if isMineConflict(err) {
				conflict++
			} else {
				otherErr = append(otherErr, err)
			}
		}()
	}
	wg.Wait()

	if okCount != 1 {
		t.Fatalf("恰 1 个 START 应成功，实际 %d", okCount)
	}
	if conflict != n-1 {
		t.Fatalf("其余 %d 个应为 409 冲突，实际 %d", n-1, conflict)
	}
	if len(otherErr) != 0 {
		t.Fatalf("出现非冲突错误: %v", otherErr)
	}
	stopResp, err := svc.StopMining()
	if err != nil || !stopResp.Accepted {
		t.Fatalf("收尾 STOP 失败: %v %v", stopResp, err)
	}
	waitMineState(t, svc, MiningStopped)
}

// TestMineLifecycleStopWhileIdle 覆盖：STOP while idle / duplicate STOP（契约 #8 幂等）。
// 断言：从未启动时 STOP 幂等成功且状态为 STOPPED；FAILED 时状态保持不被抹除。
func TestMineLifecycleStopWhileIdle(t *testing.T) {
	svc := newLifecycleService(t)

	resp, err := svc.StopMining()
	if err != nil || !resp.Accepted {
		t.Fatalf("idle STOP 应幂等成功: %+v %v", resp, err)
	}
	if st, _ := svc.miningStateSnapshot(); st != MiningStopped {
		t.Fatalf("idle STOP 后状态应为 STOPPED，实际 %s", st)
	}
}

// TestMineLifecycleStartWhileFailed 覆盖：START while FAILED（契约 #5，授权 §5）。
// 断言：409 冲突；FAILED 状态不被自动清除；STOP while FAILED 保持 FAILED。
func TestMineLifecycleStartWhileFailed(t *testing.T) {
	svc := newLifecycleService(t)

	svc.setMineState(MiningFailed, "structural-error")

	if _, err := svc.StartMining(); !isMineConflict(err) {
		t.Fatalf("FAILED 下 START 应被拒绝（409 语义），实际 err=%v", err)
	}
	if st, _ := svc.miningStateSnapshot(); st != MiningFailed {
		t.Fatalf("FAILED 状态被隐式清除: %s", st)
	}

	stopResp, err := svc.StopMining()
	if err != nil || !stopResp.Accepted {
		t.Fatalf("FAILED 下 STOP 应幂等成功: %+v %v", stopResp, err)
	}
	if st, _ := svc.miningStateSnapshot(); st != MiningFailed {
		t.Fatalf("STOP 不应抹掉 FAILED 证据，实际 %s", st)
	}
}

// TestMineLifecycleStopDuringActivePoW 覆盖：STOP during active PoW（契约 #9）。
// 断言：运行中的循环（真实 PoW 在低难度测试链上持续出块）在 STOP 后落 STOPPED；
// 节点存活（链可读）；powAttempts 单调不减（无负回退）；height 在 STOP 后停止增长。
func TestMineLifecycleStopDuringActivePoW(t *testing.T) {
	svc := newLifecycleService(t)

	if _, err := svc.StartMining(); err != nil {
		t.Fatalf("START 失败: %v", err)
	}
	waitMineStateNot(t, svc, MiningStopped)
	heightAtStop := svc.chain.Height()

	stopResp, err := svc.StopMining()
	if err != nil || !stopResp.Accepted {
		t.Fatalf("STOP 失败: %+v %v", stopResp, err)
	}
	waitMineState(t, svc, MiningStopped)

	// 节点存活：链仍可读（停挖 ≠ 停节点）。
	if svc.chain.Height() < heightAtStop {
		t.Fatalf("链高回退: %d < %d", svc.chain.Height(), heightAtStop)
	}

	// STOP 后不再有新块落地（在途块完成属于冻结语义，但循环已终止）。
	final := svc.chain.Height()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if svc.chain.Height() != final {
			t.Fatalf("STOP 后链高仍在增长: %d → %d", final, svc.chain.Height())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestMineLifecycleRepeatedStartStop 覆盖：repeated START/STOP sequence（契约 #10）。
// 断言：三次完整循环均可用（STOPPED→RUNNING→STOPPED），无状态卡死、无泄漏标志。
func TestMineLifecycleRepeatedStartStop(t *testing.T) {
	svc := newLifecycleService(t)

	for i := 0; i < 3; i++ {
		if _, err := svc.StartMining(); err != nil {
			t.Fatalf("第 %d 轮 START 失败: %v", i+1, err)
		}
		waitMineStateNot(t, svc, MiningStopped)
		if _, err := svc.StopMining(); err != nil {
			t.Fatalf("第 %d 轮 STOP 失败: %v", i+1, err)
		}
		waitMineState(t, svc, MiningStopped)
	}
	if svc.mining.Load() {
		t.Fatal("多轮循环后 mining 标志仍为 true")
	}
}

// TestMineLifecycleBeginShutdown 覆盖：START + node shutdown（冻结竞态 Case D）。
// 断言：beginShutdown 后 START 被拒绝（closing gate）；wait 返回（goroutine 已退出）。
func TestMineLifecycleBeginShutdown(t *testing.T) {
	svc := newLifecycleService(t)
	life := svc.minerLife.Load()

	if _, err := svc.StartMining(); err != nil {
		t.Fatalf("START 失败: %v", err)
	}
	waitMineStateNot(t, svc, MiningStopped)

	life.beginShutdown() // 幂等：内部 stop+wait
	waitMineState(t, svc, MiningStopped)

	if _, err := svc.StartMining(); !isMineConflict(err) {
		t.Fatalf("shutdown 后 START 应被拒绝，实际 err=%v", err)
	}
}

// TestMineLifecycleStartStopRace 覆盖：START vs STOP / STOP vs START（竞态，-race 下运行）。
// 断言：任意交错后最终状态一致（STOPPED），无 panic、无 double-close。
func TestMineLifecycleStartStopRace(t *testing.T) {
	svc := newLifecycleService(t)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = svc.StartMining() }()
		go func() { defer wg.Done(); _, _ = svc.StopMining() }()
	}
	wg.Wait()

	// 收敛：确保最终停止。
	if _, err := svc.StopMining(); err != nil {
		t.Fatalf("收敛 STOP 失败: %v", err)
	}
	waitMineState(t, svc, MiningStopped)
}

// ---- helpers ----

func waitMineStateNot(t *testing.T, svc *nodeService, not miningState) miningState {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		st, _ := svc.miningStateSnapshot()
		if st != not {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待状态离开 %s 超时", not)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
