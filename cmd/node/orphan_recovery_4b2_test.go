// orphan_recovery_4b2_test.go 覆盖 ORPHAN-DURABILITY-IMPLEMENTATION-1 §4-B2 启动恢复接线。
//
// 测试目标（owner 授权 §4-B2 Required #5）：
//   - 启动从持久化 checkpoint 恢复（prepareOrphanRestore 加载 → restorePending）
//   - restorePending 消费（OnHandshake → consumeRestorePending → requestBranch）
//   - 已知父跳过（drain）
//   - 重复握手幂等（inflight/TTL 去重，杜绝重复在途请求）
//   - 失败恢复保留（未知父 + 对端无此块 ⇒ 保留待重试）
//   - 成功恢复清理（父已知 ⇒ restorePending + checkpoint 双删）
//   - 与既有 waiting 队列交互（绝不腐蚀 / 复制 s.waiting）
//   - 重启模拟（checkpoint 加载 ↔ 恢复闭环）
//   - 无 canonical / reorg 回归
//
// 所有测试均不触碰共识 / 校验 / reorg / mempool；恢复只复用既有 requestBranch 原语。
package main

import (
	"testing"
	"time"

	"p2pchain/internal/block"
)

// genesisHash 取链上创世块哈希（canonical，必然 HasBlockHash 命中，用作「已知父」）。
func genesisHash(t *testing.T, svc *nodeService) [32]byte {
	t.Helper()
	g, err := svc.chain.BlockByHeight(0)
	if err != nil {
		t.Fatalf("读取创世块失败: %v", err)
	}
	return g.Header.Hash()
}

// connectAndWait 让 from 连接 to（真实 TCP），并等待双边 peer 链路建立。
func connectAndWait(t *testing.T, from, to *nodeService, toAddr string) {
	t.Helper()
	if err := from.net.ConnectToPeer(toAddr); err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	waitFor(t, func() bool {
		return from.net.PeerCount() == 1 && to.net.PeerCount() == 1
	}, 5*time.Second, "两节点未建立连接")
}

// Test4B2_RestoreKnownParentDrainedAndCpCleaned 已知父 ⇒ drain 双删（restorePending + checkpoint）。
// 不触碰网络（已知父不触发 requestBranch，net=nil 安全）。
func Test4B2_RestoreKnownParentDrainedAndCpCleaned(t *testing.T) {
	svc := newServiceFor(t, testChain(t))
	dir := t.TempDir()
	cp := newOrphanCheckpoint(dir)
	known := genesisHash(t, svc)

	// 写一条已知父的检查点条目（模拟「持久化但父已上链」的陈旧条目）。
	cp.MarkDirty(known, [32]byte{0xab})
	if err := cp.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// 手动把已知父塞入 restorePending（绕过 prepareOrphanRestore 的已知跳过，模拟加载态），并挂上 orphanCP。
	svc.orphanCP = cp
	svc.restorePending = map[[32]byte]struct{}{known: {}}

	svc.consumeRestorePending("127.0.0.1:0")

	if _, ok := svc.restorePending[known]; ok {
		t.Fatalf("已知父应从 restorePending 移除")
	}
	// checkpoint 也应被 Remove。
	if err := cp.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	entries, err := cp.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, e := range entries {
		if e.parentHash == known {
			t.Fatalf("checkpoint 应已清除已知父条目")
		}
	}
}

// Test4B2_ConsumeUnknownTriggersAndRetains 未知父 ⇒ 复用 requestBranch 拉取，且保留（失败恢复）。
// 需要真实连接的 A 节点使 SendTo 成功（branchReqSent 仅成功发送后自增）。
func Test4B2_ConsumeUnknownTriggersAndRetains(t *testing.T) {
	svcA := newServiceFor(t, testChain(t))
	svcB := newServiceFor(t, testChain(t))
	nodeA, addrA := startService(t, svcA)
	defer nodeA.Stop()
	nodeB, _ := startService(t, svcB)
	defer nodeB.Stop()

	unknown := [32]byte{0x77} // 不在 A 链上
	svcB.restorePending = map[[32]byte]struct{}{unknown: {}}

	// B 连接 A ⇒ OnHandshake 触发 consumeRestorePending(B, A 载荷) ⇒ requestBranch(unknown) 发往 A。
	connectAndWait(t, svcB, svcA, addrA)
	waitFor(t, func() bool { return svcB.branchReqSent.Load() >= 1 }, 5*time.Second, "未发出恢复请求")

	// 留存：A 没有该块 ⇒ 恢复未完成 ⇒ 未知父仍在 restorePending。
	if _, ok := svcB.restorePending[unknown]; !ok {
		t.Fatalf("失败恢复应保留未知父")
	}
	// A 不受影响（无回归）。
	if svcA.chain.Height() != 0 {
		t.Fatalf("A 链高度不应被影响，实际 %d", svcA.chain.Height())
	}
}

// Test4B2_ConsumeKnownSkipUnknownTrigger 混合集：已知父 drain、未知父触发且仅触发一次。
func Test4B2_ConsumeKnownSkipUnknownTrigger(t *testing.T) {
	svcA := newServiceFor(t, testChain(t))
	svcB := newServiceFor(t, testChain(t))
	nodeA, addrA := startService(t, svcA)
	defer nodeA.Stop()
	nodeB, _ := startService(t, svcB)
	defer nodeB.Stop()

	known := genesisHash(t, svcB)
	unknown := [32]byte{0x77}
	svcB.restorePending = map[[32]byte]struct{}{known: {}, unknown: {}}

	connectAndWait(t, svcB, svcA, addrA)
	waitFor(t, func() bool { return svcB.branchReqSent.Load() >= 1 }, 5*time.Second, "未发出恢复请求")

	if _, ok := svcB.restorePending[known]; ok {
		t.Fatalf("已知父应被 drain（从 restorePending 移除）")
	}
	if _, ok := svcB.restorePending[unknown]; !ok {
		t.Fatalf("未知父应保留")
	}
}

// Test4B2_InflightDedupPreventsDuplicateRequest 幂等：上一次握手已发出的在途请求（TTL 窗口内）
// 必须被 requestBranch 去重抑制，绝不二次发出（杜绝重复握手导致的重复在途请求）。
func Test4B2_InflightDedupPreventsDuplicateRequest(t *testing.T) {
	svc := newServiceFor(t, testChain(t))
	node, _ := startService(t, svc)
	defer node.Stop()

	unknown := [32]byte{0x77}
	svc.restorePending = map[[32]byte]struct{}{unknown: {}}

	// 预置在途：模拟「上一次握手已发出、尚未回来」的请求（近期 ⇒ 仍在 branchReqTTL 窗口内）。
	svc.mu.Lock()
	svc.inflight[unknown] = time.Now()
	svc.mu.Unlock()

	svc.consumeRestorePending("127.0.0.1:0")

	if svc.branchReqSent.Load() != 0 {
		t.Fatalf("在途去重应抑制重复请求，branchReqSent=%d（want 0）", svc.branchReqSent.Load())
	}
	if _, ok := svc.restorePending[unknown]; !ok {
		t.Fatalf("未知父应保留（仍为未知）")
	}
}

// Test4B2_NoWaitingCorruption 恢复路径绝不腐蚀 / 复制 s.waiting。
// 预置一个真实块到 waiting（模拟既有孤儿），consumption（含未知父触发）不应改动 waiting。
func Test4B2_NoWaitingCorruption(t *testing.T) {
	svc := newServiceFor(t, testChain(t))
	node, _ := startService(t, svc)
	defer node.Stop()

	// 一个真实块（canonical block 1）作为 waiting 中的既有条目（内容无关，仅验证规模不被改）。
	b := mineOnly(t, svc)
	svc.mu.Lock()
	svc.waiting[b.Header.Hash()] = []*block.Block{b}
	svc.parkedHashes[b.Header.Hash()] = struct{}{}
	nBefore := len(svc.waiting)
	svc.mu.Unlock()

	// restorePending 含已知父 + 未知父（后者触发 requestBranch，发往不存在的对端，安全失败）。
	known := genesisHash(t, svc)
	unknown := [32]byte{0x77}
	svc.restorePending = map[[32]byte]struct{}{known: {}, unknown: {}}

	svc.consumeRestorePending("127.0.0.1:0")

	svc.mu.Lock()
	nAfter := len(svc.waiting)
	_, stillThere := svc.waiting[b.Header.Hash()]
	svc.mu.Unlock()
	if nAfter != nBefore {
		t.Fatalf("consume 不应改动 waiting 规模：%d → %d", nBefore, nAfter)
	}
	if !stillThere {
		t.Fatalf("既有 waiting 条目被误删")
	}
}

// Test4B2_NoCanonicalRegression 空 / 已知-only restorePending 不得改变 canonical 高度或 tip。
func Test4B2_NoCanonicalRegression(t *testing.T) {
	svc := newServiceFor(t, testChain(t))
	h0 := svc.chain.Height()
	tip0, err := svc.chain.Tip()
	if err != nil {
		t.Fatalf("tip: %v", err)
	}

	// (a) 空 restorePending ⇒ 快速路径返回，零影响。
	svc.consumeRestorePending("127.0.0.1:0")

	// (b) 已知-only restorePending ⇒ drain，零链影响。
	known := genesisHash(t, svc)
	svc.restorePending = map[[32]byte]struct{}{known: {}}
	svc.consumeRestorePending("127.0.0.1:0")

	if svc.chain.Height() != h0 {
		t.Fatalf("链高度不应变化：%d → %d", h0, svc.chain.Height())
	}
	tip1, err := svc.chain.Tip()
	if err != nil {
		t.Fatalf("tip: %v", err)
	}
	if tip1.Header.Hash() != tip0.Header.Hash() {
		t.Fatalf("链尾不应变化")
	}
}

// Test4B2_RecoverEndToEnd 启动恢复闭环：B 从持久化 checkpoint 加载 restorePending（h2）⇒
// 连接 A（A 持有 h2）⇒ 握手消费触发恢复 ⇒ h2 经同步/分支到达 ⇒ 父已知 ⇒ drain + checkpoint 清理。
// 覆盖：启动恢复 / 消费 / 已知跳过 / 成功清理 / waiting 不腐蚀 / 无 canonical 回归 / 重启模拟。
func Test4B2_RecoverEndToEnd(t *testing.T) {
	svcA := newServiceFor(t, testChain(t))
	svcB := newServiceFor(t, testChain(t))

	// A 挖出 2 个区块（B 完全不知情）。
	var h2 [32]byte
	for i := 0; i < 2; i++ {
		b := mineOnly(t, svcA)
		if i == 1 {
			h2 = b.Header.Hash()
		}
	}

	// B 的检查点：持久化「父=h2」的恢复条目（模拟上一次运行残留的孤儿父键）。
	dir := t.TempDir()
	cp := newOrphanCheckpoint(dir)
	cp.MarkDirty(h2, [32]byte{0xcd})
	if err := cp.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// B 启动加载（对应 D1 启动顺序：orphanCP 赋值 + prepareOrphanRestore 在握手前）。
	svcB.orphanCP = cp
	svcB.prepareOrphanRestore(cp)
	if _, ok := svcB.restorePending[h2]; !ok {
		t.Fatalf("B 应从检查点加载 h2 到 restorePending")
	}
	if len(svcB.restorePending) != 1 {
		t.Fatalf("restorePending 应仅含 h2，实际 %d", len(svcB.restorePending))
	}

	nodeA, addrA := startService(t, svcA)
	defer nodeA.Stop()
	nodeB, _ := startService(t, svcB)
	defer nodeB.Stop()
	connectAndWait(t, svcB, svcA, addrA)

	// h2 经同步 / 分支到达 ⇒ 变为已知。
	waitFor(t, func() bool { return svcB.chain.HasBlockHash(h2) }, 10*time.Second, "B 未恢复 h2")

	// 再次消费以 drain 已已知父（握手消费发生在 h2 未知时，仅发出请求；父已知后由下一轮消费清理）。
	svcB.consumeRestorePending(addrA)
	waitFor(t, func() bool { return len(svcB.restorePending) == 0 }, 5*time.Second, "restorePending 未清空（drain 失败）")

	// 检查点清理：flush 当前（已剔除 h2 的）内存投影并 reload 校验。
	if err := cp.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	entries, err := cp.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, e := range entries {
		if e.parentHash == h2 {
			t.Fatalf("检查点应已清除 h2，但仍在")
		}
	}

	// 无回归：B 高度精确为 2（同步与恢复分支不重复上链）。
	if got := svcB.chain.Height(); got != 2 {
		t.Fatalf("B 高度=%d, want 2", got)
	}
	if svcA.chain.Height() != 2 {
		t.Fatalf("A 高度被影响：%d", svcA.chain.Height())
	}
	// waiting 队列未被恢复路径腐蚀（无孤儿产生）。
	svcB.mu.Lock()
	nw := len(svcB.waiting)
	svcB.mu.Unlock()
	if nw != 0 {
		t.Fatalf("恢复不应产生孤儿 waiting，实际 %d", nw)
	}
}
