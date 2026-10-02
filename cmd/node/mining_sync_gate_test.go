package main

// MINING-SYNC-GATE 验收测试。
//
// 背景：新数据目录的节点带 `-mine` 启动时，会在追上网络之前按本机链尾自铸
// 大量区块（v1 规则下高度 < 2000 难度钉死 16），随后全部被对端更重的链 reorg
// 丢弃 —— 实测 5 分钟自铸 72 块全部作废。本文件验证「同步门」把这些浪费挡掉，
// 同时**不触碰任何共识规则**。
//
// 覆盖层次：
//   - TestMiningSyncGateDecision：纯函数内核的全部边界（无需构造真实长链）
//   - TestMiningSyncGateLagIsFive：把「容差 = 5 块 / TTL = 10m」显式钉死为契约
//   - TestRecordPeerHeightLazyInit：登记表惰性初始化（防 nil map panic）
//   - TestMiningSyncReadyWiring：登记 → 判定的装配链路 + TTL 自愈
//   - TestMiningSyncGateBlocksAutoMinerUntilCaughtUp：端到端——持续矿工被挡住，
//     对端追平后自动恢复
//   - （已删除）TestMiningSyncGateDoesNotAffectOnDemandMine：断言对象已不存在
//
// 范围（设计 §4，已随按需出块下线改写）：同步门作用于**唯一的挖矿路径** ——
// 持续挖矿循环（runMiner）。原「/mine 按需出块不受 gate 影响」的范围限定条款
// 已随该功能整体下线而失效，对应测试一并删除。

import (
	"math/big"
	"testing"
	"time"
)

// TestMiningSyncGateDecision 覆盖纯函数内核的全部边界。
//
// 用纯函数而非真实链的原因：像「落后 100 块」这类边界，若走真实链需要先挖出
// 100 个区块才能到达该状态，既慢又掩盖了被测逻辑。纯函数让每个边界都是
// 一行输入输出。
func TestMiningSyncGateDecision(t *testing.T) {
	const (
		localH = 100
	)
	localWork := big.NewInt(1000)
	now := time.Now()

	// entry 便捷构造：d 为「距 now 的时长」（负值 = 过去）。
	entry := func(h int, work string, d time.Duration) peerHeightEntry {
		return peerHeightEntry{height: h, work: work, at: now.Add(d)}
	}

	cases := []struct {
		name        string
		localH      int
		localWork   *big.Int
		entries     []peerHeightEntry
		wantReady   bool
		wantPeerMax int
	}{
		{
			name: "无对端 → 放行（单节点/创世/隔离测试网必须能出块）",
			localH: localH, localWork: localWork, entries: nil,
			wantReady: true, wantPeerMax: localH,
		},
		{
			name: "对端落后 → 放行",
			localH: localH, localWork: localWork,
			entries:   []peerHeightEntry{entry(50, "900", 0)},
			wantReady: true, wantPeerMax: localH,
		},
		{
			name: "落后 3 块（< lag=5）→ 放行",
			localH: localH, localWork: localWork,
			entries:   []peerHeightEntry{entry(103, "1100", 0)},
			wantReady: true, wantPeerMax: 103,
		},
		{
			name: "落后恰好 5 块（== lag）→ 放行",
			localH: localH, localWork: localWork,
			entries:   []peerHeightEntry{entry(105, "1100", 0)},
			wantReady: true, wantPeerMax: 105,
		},
		{
			name: "落后 6 块（> lag）→ 阻塞",
			localH: localH, localWork: localWork,
			entries:   []peerHeightEntry{entry(106, "1100", 0)},
			wantReady: false, wantPeerMax: 106,
		},
		{
			name: "落后 100 块 → 阻塞",
			localH: localH, localWork: localWork,
			entries:   []peerHeightEntry{entry(200, "1100", 0)},
			wantReady: false, wantPeerMax: 200,
		},
		{
			name: "低工作量长链（更高但更轻）→ 不阻塞",
			localH: localH, localWork: localWork,
			entries:   []peerHeightEntry{entry(500, "900", 0)},
			wantReady: true, wantPeerMax: localH,
		},
		{
			name: "TTL 过期条目 → 忽略（对端已消失，靠时间自愈）",
			localH: localH, localWork: localWork,
			entries:   []peerHeightEntry{entry(500, "1100", -(miningSyncPeerTTL + time.Minute))},
			wantReady: true, wantPeerMax: localH,
		},
		{
			name: "TTL 未过期 → 生效",
			localH: localH, localWork: localWork,
			entries:   []peerHeightEntry{entry(500, "1100", -(miningSyncPeerTTL - time.Minute))},
			wantReady: false, wantPeerMax: 500,
		},
		{
			name: "多对端：取有效且更重者中的最高（忽略更轻的与过期的）",
			localH: localH, localWork: localWork,
			entries: []peerHeightEntry{
				entry(104, "1100", 0),                              // 更重且在 lag 内 → 计入
				entry(900, "900", 0),                               // 更高但更轻 → 忽略
				entry(800, "1100", -(miningSyncPeerTTL + time.Hour)), // 更重但已过期 → 忽略
			},
			wantReady: true, wantPeerMax: 104,
		},
		{
			name: "多对端：更重者远超 lag → 阻塞",
			localH: localH, localWork: localWork,
			entries: []peerHeightEntry{
				entry(101, "1100", 0),
				entry(400, "1200", 0),
			},
			wantReady: false, wantPeerMax: 400,
		},
		{
			name: "旧节点不填 chain_work → 退化为比高度（对端更高 ⇒ 阻塞）",
			localH: localH, localWork: localWork,
			entries:   []peerHeightEntry{entry(200, "", 0)},
			wantReady: false, wantPeerMax: 200,
		},
		{
			name: "旧节点不填 chain_work 且不更高 → 放行",
			localH: localH, localWork: localWork,
			entries:   []peerHeightEntry{entry(localH, "", 0)},
			wantReady: true, wantPeerMax: localH,
		},
		{
			name: "本地工作量未知（localWork=nil）→ 退化为比高度",
			localH: localH, localWork: nil,
			entries:   []peerHeightEntry{entry(200, "1100", 0)},
			wantReady: false, wantPeerMax: 200,
		},
		{
			name: "对端高度相同但工作量更高 → 计入（work-aware）",
			localH: localH, localWork: localWork,
			entries:   []peerHeightEntry{entry(localH, "1100", 0)},
			wantReady: true, wantPeerMax: localH,
		},
	}

	for _, c := range cases {
		gotReady, gotPeerMax := miningSyncGateDecision(c.localH, c.localWork, c.entries, now)
		if gotReady != c.wantReady || gotPeerMax != c.wantPeerMax {
			t.Errorf("%s: miningSyncGateDecision = (ready=%v, peerMax=%d), want (%v, %d)",
				c.name, gotReady, gotPeerMax, c.wantReady, c.wantPeerMax)
		}
	}
}

// TestMiningSyncGateLagIsFive 把「容差 = 5 块」这一契约显式钉死：
// 它是「防新块到达时的抖动误判」的唯一旋钮，改动它必须是有意识的决定。
func TestMiningSyncGateLagIsFive(t *testing.T) {
	if miningSyncLagBlocks != 5 {
		t.Fatalf("miningSyncLagBlocks = %d, want 5（容差改变会直接影响正常运行时是否误停挖矿）",
			miningSyncLagBlocks)
	}
	if miningSyncPeerTTL != 10*time.Minute {
		t.Fatalf("miningSyncPeerTTL = %v, want 10m", miningSyncPeerTTL)
	}
}

// TestRecordPeerHeightLazyInit 验证对 nil 登记表写入不会 panic。
//
// 必要性：最小装配（直接 `&nodeService{}` 的用例或未来的轻量装配路径）
// 不会走 newNodeService，登记表为 nil；对 nil map 写入会 panic。
func TestRecordPeerHeightLazyInit(t *testing.T) {
	svc := &nodeService{} // 刻意不走构造函数：peerHeights == nil

	svc.recordPeerHeight("10.0.0.9:16688", 7, "42")

	e, ok := svc.peerHeights["10.0.0.9:16688"]
	if !ok {
		t.Fatal("登记后仍查不到该对端（惰性初始化失效）")
	}
	if e.height != 7 || e.work != "42" {
		t.Fatalf("登记内容 = %+v, want height=7 work=42", e)
	}
	if e.at.IsZero() {
		t.Fatal("登记时刻为零值：TTL 判定会立即把所有条目判为过期")
	}
}

// TestMiningSyncReadyWiring 验证「握手登记 → 同步门判定」的装配链路，
// 并验证 TTL 过期后登记表自动失效（无断开回调场景下的自愈路径）。
func TestMiningSyncReadyWiring(t *testing.T) {
	rt := newBranchRuntime(t, t.TempDir())
	svc := rt.svc

	if h := svc.chain.Height(); h != 0 {
		t.Fatalf("新数据目录节点链高 = %d, want 0（测试前提不成立）", h)
	}

	// (1) 无对端 → 放行。这是「创世/单节点必须能出块」的保证。
	if ready, local, peerMax := svc.miningSyncReady(); !ready || local != 0 || peerMax != 0 {
		t.Fatalf("无对端时 = (ready=%v, local=%d, peerMax=%d), want (true, 0, 0)", ready, local, peerMax)
	}

	// (2) 登记一个远在前方的对端（work 未知 ⇒ 退化为比高度）→ 阻塞。
	svc.recordPeerHeight("10.0.0.1:16688", 100, "")
	ready, local, peerMax := svc.miningSyncReady()
	if ready {
		t.Fatalf("对端高度 100 而本地 %d：同步门应阻塞，实际放行", local)
	}
	if local != 0 || peerMax != 100 {
		t.Fatalf("判定返回 = (local=%d, peerMax=%d), want (0, 100)", local, peerMax)
	}

	// (3) TTL 过期 → 该条目被忽略 → 恢复放行。
	//     直接改写登记表里的时刻（同包测试可访问未导出字段），
	//     避免为了测 TTL 而 sleep 10 分钟。
	svc.mu.Lock()
	svc.peerHeights["10.0.0.1:16688"] = peerHeightEntry{
		height: 100,
		work:   "",
		at:     time.Now().Add(-(miningSyncPeerTTL + time.Minute)),
	}
	svc.mu.Unlock()

	if ready, local, peerMax := svc.miningSyncReady(); !ready {
		t.Fatalf("对端条目已 TTL 过期，同步门应放行，实际阻塞（local=%d peerMax=%d）", local, peerMax)
	}

	// (4) 重新握手会把条目刷新回有效状态 → 再次阻塞。
	//     这是「对端还活着就仍然算数」的保证：TTL 自愈不能误放行活跃对端。
	svc.recordPeerHeight("10.0.0.1:16688", 100, "")
	if ready, _, _ := svc.miningSyncReady(); ready {
		t.Fatal("重新握手后条目应恢复有效并阻塞，实际放行")
	}
}

// TestMiningSyncGateBlocksAutoMinerUntilCaughtUp 是同步门的端到端验收。
//
// 场景（对应设计「集成验证」一节，但用注入对端高度替代第二个真实节点，
// 以获得确定性——真实节点在回环上瞬间同步完，gate 窗口小到无法稳定断言）：
//
//	1. 登记一个远在前方的对端 → 启动持续矿工；
//	2. 断言矿工进入 WAITING_SYNC，且期间【零】出块、链高不推进；
//	3. 让对端「追平」→ 断言自动恢复出块。
func TestMiningSyncGateBlocksAutoMinerUntilCaughtUp(t *testing.T) {
	rt := newBranchRuntime(t, t.TempDir())
	svc := rt.svc

	// 注入前方对端：本地高度 0，对端 100（work 未知 ⇒ 按高度比较）。
	svc.recordPeerHeight("fake-seed:16688", 100, "")

	if err := svc.minerLife.Load().start(0); err != nil {
		t.Fatalf("启动持续挖矿失败: %v", err)
	}
	// 测试收尾：先停挖并等待 goroutine 退出，再让 rt.Close() 关闭存储
	// （Cleanup 为 LIFO：本函数先于 newBranchRuntime 注册的 rt.Close 执行）。
	t.Cleanup(func() { svc.minerLife.Load().beginShutdown() })

	// (2) 必须进入等待同步状态。
	waitFor(t, func() bool {
		st, _ := svc.miningStateSnapshot()
		return st == MiningWaitingSync
	}, 10*time.Second, "矿工未进入 WAITING_SYNC（同步门未生效）")

	// 阻塞期间必须零出块、链高不推进。
	time.Sleep(1500 * time.Millisecond)
	if n := svc.acceptedBlocks.Load(); n != 0 {
		t.Fatalf("等待同步期间仍出块 %d 个（同步门失效）", n)
	}
	if h := svc.chain.Height(); h != 0 {
		t.Fatalf("等待同步期间链高推进到 %d（同步门失效）", h)
	}

	// (3) 对端追平（高度降到本地）→ 门放行 → 自动恢复挖矿。
	svc.recordPeerHeight("fake-seed:16688", 0, "")

	// 注意：链高在 mineOnce 的 AddBlock 阶段就已推进，而 acceptedBlocks 计数器
	// 在其后自增，两者之间存在极短的窗口。因此必须分别「等待」两个条件，
	// 不能在一个瞬间同时断言（否则是测试自身的竞态，而非产品缺陷）。
	waitFor(t, func() bool { return svc.chain.Height() > 0 }, 60*time.Second,
		"对端追平后未恢复自动挖矿（同步门卡死）")
	waitFor(t, func() bool { return svc.acceptedBlocks.Load() > 0 }, 10*time.Second,
		"链高已推进但 acceptedBlocks 始终为 0：出块计数未更新")
}

// 注：原 TestMiningSyncGateDoesNotAffectOnDemandMine 已删除。
// 它断言「POST /mine 按需出块不受同步门约束」，而按需出块已整体下线（端点、
// 客户端、服务层方法、CLI 子命令全部移除），该断言的对象已不存在。
// 同步门现在作用于唯一的挖矿路径：持续挖矿循环 runMiner。
