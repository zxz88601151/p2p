package main

// PHASE P2P-SYNC-LIVENESS-MINIMUM-SAFE-FIX-1 的测试集。
//
// 目标：证明「全局 pending chokepoint 已消除、在途条目最终必被释放、
// retry 有界且有 backoff、断连能清理、并发有上限」，并且**正常同步行为不变**。
//
// 设计原则：全部用真实 TCP 连接与真实共识（与 service_test.go 一致），
// 唯一的「桩」是 silentPeer —— 一个只完成握手、永不回应 GetBlocks 的最小对端，
// 用于确定性地复现「请求已发出但响应永不返回」这一真实场景（CASE 7）。
//
// 本阶段不做（保持 DEFER）：心跳 / ping-pong / readTimeout / 协议消息类型 /
// NodeID / known_peers / maxInbound / BroadcastExcept 的任何改动或断言。

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/p2p"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
)

// ---- 测试脚手架 ----

// silentPeer 只完成握手、此后对 GetBlocks 永不回应的最小 P2P 对端。
//
// 它是复现「sync request 成功但 response 超时」的最小充分装置：
// 我方会正常登记在途条目并发出请求，而响应永远不来。
type silentPeer struct {
	addr string
	ln   net.Listener

	mu           sync.Mutex
	conn         net.Conn
	gotGetBlocks chan int // 每次收到的 GetBlocks 起始高度（用于证明 retry 有界）
}

// startSilentPeer 启动一个声明高度为 height 的静默对端。
func startSilentPeer(t *testing.T, height int) *silentPeer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("分配端口失败: %v", err)
	}
	sp := &silentPeer{addr: ln.Addr().String(), ln: ln, gotGetBlocks: make(chan int, 256)}
	t.Cleanup(func() { sp.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		sp.mu.Lock()
		sp.conn = conn
		sp.mu.Unlock()
		defer conn.Close()

		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadBytes('\n')
			if err != nil {
				return
			}
			var msg p2p.Message
			if err := json.Unmarshal(line, &msg); err != nil {
				continue
			}
			switch msg.Type {
			case p2p.MsgHandshake:
				// 回一条握手，使本节点把它记为「已连接且已握手」的对端。
				hs, err := json.Marshal(p2p.HandshakePayload{
					NodeID:      "silent-test-peer",
					ChainHeight: height,
					ListenAddr:  sp.addr,
				})
				if err != nil {
					return
				}
				env, err := json.Marshal(p2p.Message{Type: p2p.MsgHandshake, Payload: hs})
				if err != nil {
					return
				}
				if _, err := conn.Write(append(env, '\n')); err != nil {
					return
				}
			case p2p.MsgGetBlocks:
				var gp p2p.GetBlocksPayload
				_ = json.Unmarshal(msg.Payload, &gp)
				select {
				case sp.gotGetBlocks <- gp.FromHeight:
				default:
				}
			}
			// 其它消息一律不回应 —— 本测试要的就是「不回应」。
		}
	}()
	return sp
}

// Close 关闭连接与监听器（模拟对端消失）。
func (sp *silentPeer) Close() {
	sp.mu.Lock()
	if sp.conn != nil {
		_ = sp.conn.Close()
		sp.conn = nil
	}
	sp.mu.Unlock()
	_ = sp.ln.Close()
}

// newFastSyncService 组装一个「时序被压缩」的服务实例并启动在途调度器。
//
// 压缩的只是 TTL / backoff / 巡检周期三个**时序参数**，
// 业务分支（去重、上限、终态、failover）与生产完全一致。
func newFastSyncService(t *testing.T) *nodeService {
	t.Helper()
	svc := newServiceFor(t, testChain(t))
	svc.syncTTL = 200 * time.Millisecond
	svc.syncRetryBase = 60 * time.Millisecond
	svc.syncSweepInterval = 40 * time.Millisecond
	startService(t, svc) // 装配 svc.net
	svc.startSyncScheduler()
	t.Cleanup(svc.stopSyncScheduler)
	return svc
}

// addBlocks 真实挖出 n 个区块并直接上链（不经网络）。
func addBlocks(t *testing.T, svc *nodeService, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		tip, err := svc.chain.Tip()
		if err != nil {
			t.Fatalf("读取链尾失败: %v", err)
		}
		height := svc.chain.Height() + 1
		cb := transaction.NewCoinbaseTx(svc.miner.PubKeyHash(), utxo.Subsidy(height), height)
		candidate := block.NewCandidateBlock(tip.Header.Hash(), svc.chain.CurrentBits(),
			[]*transaction.Transaction{cb})
		if found, _ := pow.Mine(candidate); !found {
			t.Fatal("挖矿失败")
		}
		if err := svc.chain.AddBlock(candidate); err != nil {
			t.Fatalf("合法区块被拒绝: %v", err)
		}
	}
}

// syncInflightLen 在途条目数（取锁读取，测试专用）。
func syncInflightLen(svc *nodeService) int {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return len(svc.syncInflight)
}

// ---- TEST-1 正常单 peer 同步 ----

// TestMSF01NormalSinglePeerSync 正常路径：握手触发同步 → 收到响应 → 在途条目被清空。
func TestMSF01NormalSinglePeerSync(t *testing.T) {
	svcA := newServiceFor(t, testChain(t))
	svcB := newFastSyncService(t)
	nodeA, addrA := startService(t, svcA)
	defer nodeA.Stop()

	addBlocks(t, svcA, 3)
	if err := svcB.net.ConnectToPeer(addrA); err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	waitFor(t, func() bool { return svcB.chain.Height() == 3 }, 10*time.Second, "B 未追平到高度 3")

	// 批完成后在途登记表必须为空（否则就 reintroduce 了永久占用）。
	waitFor(t, func() bool { return syncInflightLen(svcB) == 0 }, 5*time.Second,
		"同步完成后在途登记表非空")
}

// ---- TEST-2 peer 正常断开 ----

// TestMSF02PeerDisconnectReleasesInFlight 对端断开后其条目被释放（不再等 TTL）。
func TestMSF02PeerDisconnectReleasesInFlight(t *testing.T) {
	svc := newFastSyncService(t)
	sp := startSilentPeer(t, 100)

	if err := svc.net.ConnectToPeer(sp.addr); err != nil {
		t.Fatalf("连接静默对端失败: %v", err)
	}
	waitFor(t, func() bool { return syncInflightLen(svc) == 1 }, 5*time.Second, "请求未登记")

	// 对端消失：条目应在下一次巡检被判负（reason=peer_disconnected），不必等 TTL。
	sp.Close()
	waitFor(t, func() bool { return syncInflightLen(svc) == 0 }, 5*time.Second,
		"对端断开后在途条目未被释放")
}

// ---- TEST-3 peer 在 request 后立即失效 ----

// TestMSF03PeerDiesImmediatelyAfterRequest 请求刚发出即对端失效，条目仍须最终释放。
func TestMSF03PeerDiesImmediatelyAfterRequest(t *testing.T) {
	svc := newFastSyncService(t)
	sp := startSilentPeer(t, 100)

	if err := svc.net.ConnectToPeer(sp.addr); err != nil {
		t.Fatalf("连接静默对端失败: %v", err)
	}
	waitFor(t, func() bool { return syncInflightLen(svc) == 1 }, 5*time.Second, "请求未登记")
	sp.Close() // 请求刚发出就失效

	waitFor(t, func() bool { return syncInflightLen(svc) == 0 }, 5*time.Second,
		"请求后立即失效时条目未被最终释放")
}

// ---- TEST-4 SendTo failure ----

// TestMSF04SendToFailureIsNotTightLoop 发送失败：立即判负但**不在读循环内重试**。
func TestMSF04SendToFailureIsNotTightLoop(t *testing.T) {
	svc := newFastSyncService(t)

	// 向一个根本不存在的地址请求：SendTo 立即失败。
	svc.requestSync("198.51.100.1:65530", 1)

	svc.mu.Lock()
	r, ok := svc.syncInflight[1]
	snapshot := syncRequest{}
	if ok {
		snapshot = *r
	}
	svc.mu.Unlock()

	if !ok {
		t.Fatal("发送失败后条目不应被立即释放（应由调度器按 backoff 处理）")
	}
	if snapshot.attempt != 1 {
		t.Fatalf("发送失败后尝试次数应为 1，实际 %d（说明在调用栈内发生了重试）", snapshot.attempt)
	}
	if !snapshot.failed {
		t.Fatal("发送失败后条目应处于 failed 状态")
	}
	if !snapshot.nextRetry.After(time.Now()) {
		t.Fatal("发送失败后必须设置未来的重试时刻（backoff 闸门缺失 ⇒ 可能紧循环）")
	}
}

// TestMSF04bSendToFailureTerminalWithoutPeer 无任何可用对端时，失败条目进入终态释放。
func TestMSF04bSendToFailureTerminalWithoutPeer(t *testing.T) {
	svc := newFastSyncService(t)
	svc.requestSync("198.51.100.1:65530", 1)

	waitFor(t, func() bool { return syncInflightLen(svc) == 0 }, 5*time.Second,
		"无可用对端时失败条目未进入终态释放")
}

// ---- TEST-5 response timeout（TTL） ----

// TestMSF05ResponseTimeoutBoundedRetry 响应超时：TTL 判负 → 有界重试 → 终态释放。
func TestMSF05ResponseTimeoutBoundedRetry(t *testing.T) {
	svc := newFastSyncService(t)
	sp := startSilentPeer(t, 100)

	if err := svc.net.ConnectToPeer(sp.addr); err != nil {
		t.Fatalf("连接静默对端失败: %v", err)
	}
	waitFor(t, func() bool { return syncInflightLen(svc) == 1 }, 5*time.Second, "请求未登记")
	waitFor(t, func() bool { return syncInflightLen(svc) == 0 }, 10*time.Second,
		"响应超时场景下在途条目未被最终释放")

	// 重试必须是**有界**的：请求总数不得超过 maxSyncAttempts。
	got := len(sp.gotGetBlocks)
	if got < 2 {
		t.Fatalf("TTL 到期后应至少重试 1 次（实际收到 %d 次请求），否则等于没有 failover 能力", got)
	}
	if got > maxSyncAttempts {
		t.Fatalf("重试次数越界：收到 %d 次请求 > maxSyncAttempts=%d", got, maxSyncAttempts)
	}
}

// ---- TEST-6 多 peer / 并发上限 ----

// TestMSF06GlobalConcurrencyCap 在途条目数受全局上限约束。
func TestMSF06GlobalConcurrencyCap(t *testing.T) {
	svc := newFastSyncService(t)

	// 白盒预置 maxInflightSync 个在途条目（不同区间），模拟多路并发追赶。
	now := time.Now()
	svc.mu.Lock()
	for i := 0; i < maxInflightSync; i++ {
		from := 10 + i
		svc.syncInflight[from] = &syncRequest{
			from: from, peer: "198.51.100.2:1000", attempt: 1,
			created: now, deadline: now.Add(time.Hour),
		}
	}
	svc.mu.Unlock()

	// 第 maxInflightSync+1 个区间必须被上限拒绝。
	svc.requestSync("198.51.100.2:1000", 99)
	if got := syncInflightLen(svc); got != maxInflightSync {
		t.Fatalf("并发上限失效：在途 %d 条，期望被限制在 %d 条", got, maxInflightSync)
	}
}

// ---- TEST-7 一个坏 peer + 一个健康 peer（failover） ----

// TestMSF07FailoverToHealthyPeer 坏 peer 占住区间后，TTL 到期必须改投健康 peer 并追平。
func TestMSF07FailoverToHealthyPeer(t *testing.T) {
	svcA := newServiceFor(t, testChain(t)) // 健康对端：真实节点，链上有 3 个区块
	svcB := newFastSyncService(t)
	nodeA, addrA := startService(t, svcA)
	defer nodeA.Stop()
	addBlocks(t, svcA, 3)

	// 先接坏 peer（永不回应），它会占用区间 1 的在途槽。
	sp := startSilentPeer(t, 100)
	if err := svcB.net.ConnectToPeer(sp.addr); err != nil {
		t.Fatalf("连接静默对端失败: %v", err)
	}
	waitFor(t, func() bool { return syncInflightLen(svcB) == 1 }, 5*time.Second, "坏 peer 请求未登记")

	// 再接健康 peer：它触发的同步会被去重抑制（区间 1 已在途）。
	if err := svcB.net.ConnectToPeer(addrA); err != nil {
		t.Fatalf("连接健康对端失败: %v", err)
	}

	// 坏 peer 不阻塞全局：TTL 到期后调度器改投健康 peer，B 必须追平到 3。
	waitFor(t, func() bool { return svcB.chain.Height() == 3 }, 15*time.Second,
		"坏 peer 未让出在途槽：B 未能经 failover 追平到高度 3")
	waitFor(t, func() bool { return syncInflightLen(svcB) == 0 }, 5*time.Second,
		"追平后在途登记表非空")
}

// ---- TEST-8 重复相同 range ----

// TestMSF08DuplicateRangeSuppressed 同一区间重复请求被去重抑制（不得产生请求风暴）。
func TestMSF08DuplicateRangeSuppressed(t *testing.T) {
	svc := newFastSyncService(t)
	sp := startSilentPeer(t, 100)
	if err := svc.net.ConnectToPeer(sp.addr); err != nil {
		t.Fatalf("连接静默对端失败: %v", err)
	}
	waitFor(t, func() bool { return syncInflightLen(svc) == 1 }, 5*time.Second, "首次请求未登记")

	for i := 0; i < 5; i++ {
		svc.requestSync(sp.addr, 1)
	}
	if got := syncInflightLen(svc); got != 1 {
		t.Fatalf("同一区间重复请求未被去重：在途 %d 条，期望 1 条", got)
	}
	// 首次登记之外的 5 次调用不得产生任何网络请求。
	if got := len(sp.gotGetBlocks); got != 1 {
		t.Fatalf("重复请求产生了请求风暴：对端收到 %d 次 GetBlocks，期望 1 次", got)
	}
}

// ---- TEST-9 连续 reconnect / churn ----

// TestMSF09ReconnectChurnNoLeak 反复重连不得让在途登记表无限增长。
func TestMSF09ReconnectChurnNoLeak(t *testing.T) {
	svc := newFastSyncService(t)
	for i := 0; i < 6; i++ {
		sp := startSilentPeer(t, 100)
		if err := svc.net.ConnectToPeer(sp.addr); err != nil {
			t.Fatalf("第 %d 轮连接失败: %v", i, err)
		}
		waitFor(t, func() bool { return syncInflightLen(svc) >= 1 }, 5*time.Second,
			"请求未登记")
		sp.Close() // 模拟 churn：连接立刻断开
		if got := syncInflightLen(svc); got > maxInflightSync {
			t.Fatalf("churn 期间在途条目越界: %d > %d", got, maxInflightSync)
		}
	}
	waitFor(t, func() bool { return syncInflightLen(svc) == 0 }, 10*time.Second,
		"churn 结束后在途登记表未清空")
}

// ---- TEST-10 不存在永久 pending ----

// TestMSF10NoPermanentPending 在全部对端都不回应的极端情况下，登记表必须最终清空。
func TestMSF10NoPermanentPending(t *testing.T) {
	svc := newFastSyncService(t)
	sp := startSilentPeer(t, 100)
	if err := svc.net.ConnectToPeer(sp.addr); err != nil {
		t.Fatalf("连接静默对端失败: %v", err)
	}
	waitFor(t, func() bool { return syncInflightLen(svc) == 1 }, 5*time.Second, "请求未登记")

	// 远长于 TTL + 全部 backoff + 巡检周期之和。
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if syncInflightLen(svc) == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("出现永久 pending：10s 后在途仍有 %d 条", syncInflightLen(svc))
}

// ---- 强制回归测试 ----

// TestMSFPermanentPendingRegression 专项回归：
//
//	请求登记 → 对端永不响应 → TTL 到期 → 在途必须释放 → 后续 sync 重新获得调度机会。
//
// 这是修复前 CASE 7（一个僵死 peer 使 pending 永久停留在 1）的直接反例。
func TestMSFPermanentPendingRegression(t *testing.T) {
	svc := newFastSyncService(t)
	sp := startSilentPeer(t, 100)
	if err := svc.net.ConnectToPeer(sp.addr); err != nil {
		t.Fatalf("连接静默对端失败: %v", err)
	}
	waitFor(t, func() bool { return syncInflightLen(svc) == 1 }, 5*time.Second, "请求未登记")

	// (1) TTL 到期后在途必须释放。
	waitFor(t, func() bool { return syncInflightLen(svc) == 0 }, 10*time.Second,
		"PERMANENT-PENDING：TTL 到期后在途条目未释放")

	// (2) 释放后同一区间必须能重新登记 —— 这正是修复前做不到的事。
	svc.requestSync(sp.addr, 1)
	svc.mu.Lock()
	_, reacquired := svc.syncInflight[1]
	attempt := 0
	if reacquired {
		attempt = svc.syncInflight[1].attempt
	}
	svc.mu.Unlock()
	if !reacquired {
		t.Fatal("PERMANENT-PENDING：释放后同一区间无法重新登记（chokepoint 未消除）")
	}
	if attempt != 1 {
		t.Fatalf("重新登记的尝试次数应重置为 1，实际 %d", attempt)
	}

	// (3) 新的一轮同样必须最终释放（不得出现第二轮永久占用）。
	waitFor(t, func() bool { return syncInflightLen(svc) == 0 }, 10*time.Second,
		"PERMANENT-PENDING：第二轮在途条目未释放")
}

// TestMSFPeerDisconnectCleanupRegression 专项回归：
//
//	两个不同 peer 各持一个在途区间 → 其中一个断开 → 只有它的条目被清理，
//	另一个不受影响。
func TestMSFPeerDisconnectCleanupRegression(t *testing.T) {
	svc := newFastSyncService(t)
	spA := startSilentPeer(t, 100)
	if err := svc.net.ConnectToPeer(spA.addr); err != nil {
		t.Fatalf("连接静默对端失败: %v", err)
	}
	waitFor(t, func() bool { return syncInflightLen(svc) == 1 }, 5*time.Second, "请求未登记")

	// 接入第二个（同样静默但**保持连接**的）对端，并让它持有另一个区间。
	spB := startSilentPeer(t, 200)
	if err := svc.net.ConnectToPeer(spB.addr); err != nil {
		t.Fatalf("连接第二个静默对端失败: %v", err)
	}
	waitFor(t, func() bool {
		for _, a := range svc.net.PeerAddrs() {
			if a == spB.addr {
				return true
			}
		}
		return false
	}, 5*time.Second, "第二个对端未建立连接")

	svc.requestSync(spB.addr, 77)
	// 把该条目的 TTL 拉长，使「断连清理」成为本断言窗口内唯一的判负来源
	//（只考察隔离性：spA 断开不得波及 spB 持有的条目）。
	svc.mu.Lock()
	rB, okB := svc.syncInflight[77]
	if okB {
		rB.deadline = time.Now().Add(time.Hour)
	}
	svc.mu.Unlock()
	if !okB {
		t.Fatal("第二个对端的在途条目未登记")
	}

	// 断开 spA：只应清理它持有的区间 1。
	spA.Close()
	waitFor(t, func() bool {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		_, stillA := svc.syncInflight[1]
		return !stillA
	}, 5*time.Second, "对端断开后其条目未被清理")

	svc.mu.Lock()
	_, stillOther := svc.syncInflight[77]
	svc.mu.Unlock()
	if !stillOther {
		t.Fatal("PEER-DISCONNECT-CLEANUP：清理越界，影响了其他对端的在途条目")
	}
	// 更强的隔离断言：条目的**归属**也未被篡改。
	snap := svc.syncInflightSnapshot()
	if r, ok := snap[77]; !ok || r.peer != spB.addr {
		t.Fatalf("PEER-DISCONNECT-CLEANUP：条目归属被篡改，期望 %s，实际快照 %+v",
			spB.addr, snap[77])
	}

	// 收尾：恢复短 TTL，spB 持有的条目也必须最终释放（不得永久占位）。
	svc.mu.Lock()
	if r, ok := svc.syncInflight[77]; ok {
		r.deadline = time.Now()
	}
	svc.mu.Unlock()
	waitFor(t, func() bool { return syncInflightLen(svc) == 0 }, 10*time.Second,
		"另一对端条目未最终释放")
}

// ---- 行为保持（不启动调度器 ⇒ 与修复前一致） ----

// TestMSFNoSchedulerKeepsLegacyBehavior 调度器未启动时，发送失败即立即释放，
// 行为与修复前 `pending = 0` 完全一致（保证不引入新的隐式重试语义）。
func TestMSFNoSchedulerKeepsLegacyBehavior(t *testing.T) {
	svc := newServiceFor(t, testChain(t))
	startService(t, svc) // 装配 svc.net，但不启动调度器

	svc.requestSync("198.51.100.1:65530", 1) // 必然发送失败
	if got := syncInflightLen(svc); got != 0 {
		t.Fatalf("未启动调度器时失败应立即释放（旧行为），实际在途 %d 条", got)
	}
}

// TestMSFBackoffIsMonotonic 退避必须随尝试次数单调增长（杜绝紧循环的量化证明）。
func TestMSFBackoffIsMonotonic(t *testing.T) {
	svc := newServiceFor(t, testChain(t))
	prev := time.Duration(0)
	for attempt := 1; attempt <= maxSyncAttempts; attempt++ {
		d := svc.backoffFor(attempt)
		if d <= prev {
			t.Fatalf("退避非单调：attempt=%d 得到 %v，前值 %v", attempt, d, prev)
		}
		prev = d
	}
}
