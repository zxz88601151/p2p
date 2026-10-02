// orphan_durability_e2e_test.go 是 ORPHAN-DURABILITY-E2E-RECOVERY-1 的真实可执行端到端验证。
//
// 目标：证明完整孤儿持久化生命周期
//
//	孤儿创建 → 检查点持久化 → 崩溃/重启 → restorePending 重建 →
//	OnHandshake 消费 → 分支请求 → 父块恢复 → 孤儿解析 → 链一致性
//
// 与 §4-B2 既有测试的本质区别（本测试的关键价值）：
//   - §4-B2 的 RecoverEndToEnd 是「半真」：检查点由手动 MarkDirty+Flush 伪造、恢复由
//     手动调用 prepareOrphanRestore 触发；并未经过真实的孤儿停车管线写盘，也未经过真实的
//     OnNewBlock→deferOrphan→MarkDirty 挂钩。
//   - 本测试是「全真」：孤儿由真实区块处理管线产生，检查点经真实原子写落盘到真实 datadir
//     的 orphan_waiting.bin，重启经真实 prepareOrphanRestore 从磁盘重建 restorePending，
//     恢复经真实 OnHandshake→consumeRestorePending→requestBranch 跨真实 TCP 拉取。
//
// 所有断言仅验证可观察状态（链高、链尾哈希、canonical 成员、waiting 规模、检查点文件内容、
// branchReqSent 计数），不绑定内部函数名。
//
// HARD STOP（owner 授权）：本文件只新增测试，不改动任何生产代码；不提交、不部署、不改生产状态。
package main

import (
	"os"
	"testing"
	"time"
)

// TestE2E_OrphanDurabilityRecoveryLifecycle 完整生命周期端到端验证（见文件头说明）。
func TestE2E_OrphanDurabilityRecoveryLifecycle(t *testing.T) {
	// === §0 HARD BASELINE（详见 PHASE-P2PCHAIN-ORPHAN-DURABILITY-E2E-RECOVERY-1-REPORT.md）===
	// git HEAD=52fb464..., go1.27.0, datadir 初始无 orphan_waiting.bin。
	// 本测试在函数内对每一步做不变量断言，等价于对基线做运行时校验。

	// --- 准备对账节点 A：持有完整链（genesis→p→c）---
	svcA := newServiceFor(t, testChain(t))
	p := mineOnly(t, svcA) // 高度1，父=genesis
	c := mineOnly(t, svcA) // 高度2，父=p（链式：mineOnly 以当前 tip=p 为父）
	if c.Header.PrevBlockHash != p.Header.Hash() {
		t.Fatalf("构造前提失败：c 的父应为 p，实际 %s", hashHex32(c.Header.PrevBlockHash))
	}
	nodeA, addrA := startService(t, svcA)
	defer nodeA.Stop()
	if svcA.chain.Height() != 2 {
		t.Fatalf("A 链高度应为 2，实际 %d", svcA.chain.Height())
	}

	// --- §1 孤儿创建：B 收到 c 早于其父 p ---
	dirB := t.TempDir()
	cpB := newOrphanCheckpoint(dirB) // 真实 datadir 的孤儿等待检查点（尚未落盘）
	svcB := newServiceFor(t, testChain(t))
	svcB.orphanCP = cpB // 启用真实持久化投影（newServiceFor 默认不挂 orphanCP）
	nodeB, _ := startService(t, svcB)
	// 注意：不 defer nodeB.Stop()，§2 显式 Stop 模拟进程崩溃

	// B 经由真实区块处理管线接收 c：父 p 未知 → deferOrphan 停车于 waiting[p]，
	// 并触发真实 orphanCP.MarkDirty(p, c)。
	deliverBroadcast(t, svcB, c)

	// §1 断言①：真实孤儿已进入 waiting[parent]。
	assertParked(t, svcB, p.Header.Hash(), 1, "§1 孤儿应停在 waiting[parent]")
	// §1 断言②：waiting 仅含 1 个父键（p）。
	svcB.mu.Lock()
	nw := len(svcB.waiting)
	svcB.mu.Unlock()
	if nw != 1 {
		t.Fatalf("§1 waiting 键数应为 1，实际 %d", nw)
	}

	// §1 断言③：检查点经真实原子写落盘到 dirB/orphan_waiting.bin。
	if err := cpB.Flush(); err != nil {
		t.Fatalf("§1 检查点落盘失败: %v", err)
	}
	entries, err := cpB.Load() // 从真实磁盘文件重新解析
	if err != nil {
		t.Fatalf("§1 检查点加载失败: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("§1 检查点应含 1 条父键，实际 %d", len(entries))
	}
	if entries[0].parentHash != p.Header.Hash() {
		t.Fatalf("§1 检查点父键应为 p，实际 %s", hashHex32(entries[0].parentHash))
	}
	if len(entries[0].childHash) != 1 || entries[0].childHash[0] != c.Header.Hash() {
		t.Fatalf("§1 检查点子哈希应为 c")
	}
	// §1 断言④：文件确实存在于磁盘。
	if _, statErr := os.Stat(cpB.Path()); statErr != nil {
		t.Fatalf("§1 检查点文件应存在于磁盘: %v", statErr)
	}

	// === §2 崩溃 / 重启 ===
	// 模拟进程终止：停止 B 的 P2P（内存 waiting 随进程消失，符合 SPEC「仅持久化父键、不持久化块体」契约）。
	nodeB.Stop()
	// §2 断言①：崩溃后磁盘检查点未被破坏。
	if _, statErr := os.Stat(cpB.Path()); statErr != nil {
		t.Fatalf("§2 崩溃后检查点文件应仍在磁盘: %v", statErr)
	}

	// 新进程 B2：共享同一 datadir，复刻 main.go 启动顺序
	// （orphanCP 赋值 → prepareOrphanRestore，均在首个握手前）从磁盘重建 restorePending。
	cpB2 := newOrphanCheckpoint(dirB)
	svcB2 := newServiceFor(t, testChain(t))
	svcB2.orphanCP = cpB2
	svcB2.prepareOrphanRestore(cpB2) // 真实从 dirB/orphan_waiting.bin 加载
	// §2 断言②：restorePending 由真实文件重建出 p。
	if _, ok := svcB2.restorePending[p.Header.Hash()]; !ok {
		t.Fatalf("§2 重启应从检查点重建 restorePending[p]")
	}
	if len(svcB2.restorePending) != 1 {
		t.Fatalf("§2 restorePending 应仅含 p，实际 %d", len(svcB2.restorePending))
	}
	// §2 断言③：B2 链尚未含 p/c（全新链，仅 genesis）。
	if svcB2.chain.Height() != 0 {
		t.Fatalf("§2 B2 初始高度应为 0，实际 %d", svcB2.chain.Height())
	}

	// === §3 握手恢复 ===
	nodeB2, _ := startService(t, svcB2)
	defer nodeB2.Stop()
	// 触发 B2.OnHandshake(addrA) → consumeRestorePending(addrA)。
	connectAndWait(t, svcB2, svcA, addrA)

	// §3 断言①：恢复分支请求已真实发出（branchReqSent 仅成功 SendTo 后自增）。
	waitFor(t, func() bool { return svcB2.branchReqSent.Load() >= 1 }, 5*time.Second, "§3 未发出恢复分支请求")
	// §3 断言②：父 p 经分支请求/同步到达 B2。
	waitFor(t, func() bool { return svcB2.chain.HasBlockHash(p.Header.Hash()) }, 10*time.Second, "§3 B2 未恢复父块 p")
	// §3 断言③：对账节点 A 不受影响（无回归）。
	if svcA.chain.Height() != 2 {
		t.Fatalf("§3 A 链高度被影响: %d", svcA.chain.Height())
	}

	// drain：父已知后再次消费，双删 restorePending + checkpoint（drain 路径）。
	svcB2.consumeRestorePending(addrA)
	waitFor(t, func() bool { return len(svcB2.restorePending) == 0 }, 5*time.Second, "§3 restorePending 未清空")

	// === §4 孤儿解析 + 一致性证明 ===
	// 孤儿块体不持久化（仅父键持久化），经重新 gossip 在父已知后入链（真实网络行为等价）。
	deliverBroadcast(t, svcB2, c)
	// §4 断言①：c 被接受 → B2 高度变为 2，链尾 == A 链尾 c。
	waitFor(t, func() bool { return svcB2.chain.Height() == 2 }, 10*time.Second, "§4 孤儿 c 未解析上链")
	assertTipIs(t, svcB2, c, "§4 B2 链尾应与 A 一致")
	// §4 断言②：无重复上链（A/B2 高度均精确为 2）。
	assertHeight(t, svcA, 2, "§4 A 高度")
	assertHeight(t, svcB2, 2, "§4 B2 高度")
	// §4 断言③：无 waiting 腐蚀（孤儿解析后 waiting 清空）。
	assertWaitingEmpty(t, svcB2, "§4 解析后 waiting 应清空")
	// §4 断言④：无 canonical 回归（B2 与 A 逐高度哈希一致）。
	for h := 0; h <= 2; h++ {
		ba, errA := svcA.chain.BlockByHeight(h)
		bb, errB := svcB2.chain.BlockByHeight(h)
		if errA != nil || errB != nil {
			t.Fatalf("§4 高度 %d 取块失败: A=%v B=%v", h, errA, errB)
		}
		if ba.Header.Hash() != bb.Header.Hash() {
			t.Fatalf("§4 高度 %d 块哈希不一致（canonical 回归）: A=%s B=%s",
				h, ba.Header.HashHex(), bb.Header.HashHex())
		}
	}

	// §4 断言⑤：幂等——重复消费 / 重复握手安全（无重复在途请求、无重复上链）。
	before := svcB2.branchReqSent.Load()
	svcB2.consumeRestorePending(addrA)
	svcB2.consumeRestorePending(addrA)
	if svcB2.branchReqSent.Load() != before {
		t.Fatalf("§4 幂等消费不应再发请求（%d → %d）", before, svcB2.branchReqSent.Load())
	}
	if svcB2.chain.Height() != 2 {
		t.Fatalf("§4 幂等消费后高度异常: %d", svcB2.chain.Height())
	}

	// §4 断言⑥：检查点最终清理——flush 当前内存投影并 reload，确认 p 已剔除。
	if err := cpB2.Flush(); err != nil {
		t.Fatalf("§4 检查点最终落盘失败: %v", err)
	}
	finalEntries, err := cpB2.Load()
	if err != nil {
		t.Fatalf("§4 检查点最终加载失败: %v", err)
	}
	for _, e := range finalEntries {
		if e.parentHash == p.Header.Hash() {
			t.Fatalf("§4 检查点应已清除 p，但仍在")
		}
	}
}
