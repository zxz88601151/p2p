package main

// PHASE MINING-LIFECYCLE-1：持续挖矿生命周期管理器（设计冻结实现）。
//
// 设计冻结依据：.workbuddy/ONE-CLICK-MINING-DESIGN-FREEZE/DESIGN-FREEZE-REPORT。
//
// 关键不变量：
//   - 单飞：mining atomic.Bool 的 CompareAndSwap 保证同一时间最多一个 miner
//     instance；禁止任何 Load→Store 形式的守卫（设计冻结 §3）。
//   - 停挖 ≠ 停节点：minerStopCh 与 node stopCh 完全分离；节点停机路径会
//     同时关闭 minerStopCh（停机 ⇒ 停挖），反向不成立（设计冻结 §4）。
//   - PoW 取消复用既有 mineOnce 的 stop 参数 + pow.MineCancelable 通道
//     （main.go 既有机制，本文件不新增第二套取消抽象）。
//   - FAILED 为终态：拒绝 START 且不自动清除（设计冻结 §3/§5）；
//     不持久化，节点重启后诚实回到 STOPPED（设计冻结 §7）。
//   - STOP 语义：不再开始新的候选区块；在途 PoW 取消、在途 AddBlock 自然
//     完成（块可能照常上链，设计冻结 §4/§10），blocks.dat 一致性由
//     STOP-INV-05 既有保证维护。

import (
	"sync"

	"p2pchain/internal/control"
)

// minerRun 表示一次持续挖矿运行：每个 START 新建一个 stop 通道，
// 配对一个 sync.Once，保证该次运行只被 close 一次（无 double close）。
// done 在 runMiner 返回后关闭，供节点停机路径等待挖矿 goroutine 完全退出，
// 确保 store.Close() 不与在途 AddBlock 并发（STOP-INV-05）。
type minerRun struct {
	stopCh chan struct{}
	once   sync.Once
	done   chan struct{}
}

func (r *minerRun) requestStop() { r.once.Do(func() { close(r.stopCh) }) }

// minerLifecycle 持续挖矿生命周期管理器。
// 由 newNodeService 创建并挂到 nodeService.minerLife（atomic.Pointer），
// 控制面（POST /mine/stop）与节点停机路径共用同一个 Stop 入口。
type minerLifecycle struct {
	svc *nodeService

	mu      sync.Mutex
	current *minerRun // nil = 无运行中的挖矿循环
	closing bool      // 节点停机已启动：拒绝后续 START（冻结竞态 Case D）
}

func newMinerLifecycle(svc *nodeService) *minerLifecycle {
	return &minerLifecycle{svc: svc}
}

// start 启动持续挖矿（单飞）。maxBlocks>0 保留 boot -max-blocks 既有语义。
//
// 返回 *control.MineConflictError 表示状态冲突（已在跑 / FAILED / STOPPING /
// 节点停机中），调用方（control handler）映射为 HTTP 409；CAS 失败亦属冲突，
// 无副作用。
func (m *minerLifecycle) start(maxBlocks int) error {
	s := m.svc

	m.mu.Lock()
	closing := m.closing
	m.mu.Unlock()
	if closing {
		return &control.MineConflictError{Message: "节点正在关闭，拒绝启动挖矿", State: string(MiningStopped)}
	}

	// FAILED 为终态：拒绝 START，不自动清除、不隐式 reset（设计冻结 §5）。
	if st, _ := s.miningStateSnapshot(); st == MiningFailed {
		return &control.MineConflictError{
			Message: "挖矿处于 FAILED（结构性错误），需修复后重启节点",
			State:   string(st),
		}
	}

	// 单飞：CAS false→true；失败 = 已有 miner 在跑（含 STOPPING 窗口与并发 START）。
	if !s.mining.CompareAndSwap(false, true) {
		st, _ := s.miningStateSnapshot()
		return &control.MineConflictError{Message: "挖矿已在运行", State: string(st)}
	}

	m.mu.Lock()
	run := &minerRun{stopCh: make(chan struct{}), done: make(chan struct{})}
	m.current = run
	m.mu.Unlock()

	go func() {
		defer close(run.done)
		runMiner(s, maxBlocks, run.stopCh)
	}()
	return nil
}

// stop 幂等停止挖矿。绝不触碰节点 stopCh / P2P / 存储 / 共识状态。
//
// 状态处理（设计冻结 §4）：
//   - mining=true（RUNNING/STARTING/STALLED 循环存活）：立即落 STOPPING
//     （stop-requested），最终 STOPPED 由 runMiner finish() 收尾；
//   - mining=false（STOPPED / FAILED / STALLED 无循环遗留）：不改状态，
//     FAILED 证据保留，响应如实返回当前状态。
func (m *minerLifecycle) stop() {
	s := m.svc

	if s.mining.Load() {
		if st, _ := s.miningStateSnapshot(); st != MiningFailed {
			s.setMineState(MiningStopping, "stop-requested")
		}
	}

	m.mu.Lock()
	run := m.current
	m.mu.Unlock()
	if run != nil {
		run.requestStop()
	}
}

// beginShutdown 标记节点停机开始（拒绝后续 START），并停止+等待挖矿 goroutine
// 完全退出。必须在 rt.Close()（store 关闭）之前调用，保证不与在途 AddBlock
// 并发（STOP-INV-05）；在途块自然完成（冻结语义 §10）。幂等。
func (m *minerLifecycle) beginShutdown() {
	m.mu.Lock()
	m.closing = true
	m.mu.Unlock()
	m.stop()
	m.wait()
}

// wait 等待当前挖矿运行（若有）完全退出。无可等待运行时立即返回。
func (m *minerLifecycle) wait() {
	m.mu.Lock()
	run := m.current
	m.mu.Unlock()
	if run != nil {
		<-run.done
	}
}
