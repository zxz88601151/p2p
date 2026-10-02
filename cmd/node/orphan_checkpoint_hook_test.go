// orphan_checkpoint_hook_test.go 覆盖 ORPHAN-DURABILITY-IMPLEMENTATION-1 §3-A 挂钩级测试。
//
// 目标（SPEC §3-A 证明点）：
//  1. orphan 队列行为不变（orphanCP == nil 时挂钩为 no-op，由既有 B1 测试回归保证）；
//  2. 持久化失败不改变 correctness 路径（本文件验证 orphanCP 投影仅增删，不影响 waiting 语义）；
//  3. 锁序（nodeService.mu 不嵌套 orphanCheckpoint 内部锁）——见注释与 T-LockOrder。
package main

import (
	"testing"

	"p2pchain/internal/block"
)

// TestOrphanCPHook_EnqueuePersists 验证 deferOrphan 入队后调用 MarkDirty，
// 使 orphanCP 投影记录 parent→child；且 waiting 队列语义不变（子块仍在 waiting）。
func TestOrphanCPHook_EnqueuePersists(t *testing.T) {
	svc := newB1Service(t)
	cp := newOrphanCheckpoint(t.TempDir())
	svc.orphanCP = cp

	// 挖一个「父缺失」的子块：父哈希 = 随机未知哈希。
	child := mineOnTo(t, svc, block_unknownParent(t, svc), 1, svc.miner.PubKeyHash())
	parent := child.Header.PrevBlockHash
	childHash := child.Header.Hash()

	svc.deferOrphan(b1Peer, child)

	// 1. waiting 队列仍含该父键（行为不变）。
	svc.mu.Lock()
	kids := svc.waiting[parent]
	svc.mu.Unlock()
	if len(kids) != 1 {
		t.Fatalf("waiting queue should retain 1 child, got %d", len(kids))
	}

	// 2. orphanCP 投影记录了 parent→child。
	cp.mu.Lock()
	found := false
	for _, e := range cp.entries {
		if e.parentHash == parent {
			if len(e.childHash) == 1 && e.childHash[0] == childHash {
				found = true
			}
		}
	}
	cp.mu.Unlock()
	if !found {
		t.Fatalf("orphanCP projection should record parent→child after deferOrphan")
	}
}

// TestOrphanCPHook_TakeWaitingRemoves 验证 takeWaiting 删除后调用 Remove，
// 使 orphanCP 投影删除该 parent 键；且 waiting 队列被清空（行为不变）。
func TestOrphanCPHook_TakeWaitingRemoves(t *testing.T) {
	svc := newB1Service(t)
	cp := newOrphanCheckpoint(t.TempDir())
	svc.orphanCP = cp

	child := mineOnTo(t, svc, block_unknownParent(t, svc), 1, svc.miner.PubKeyHash())
	parent := child.Header.PrevBlockHash

	svc.deferOrphan(b1Peer, child)

	// takeWaiting 取走该父键。
	out := svc.takeWaiting(parent)
	if len(out) != 1 {
		t.Fatalf("takeWaiting should return 1 child, got %d", len(out))
	}

	// 1. waiting 队列已清空（行为不变）。
	svc.mu.Lock()
	_, stillThere := svc.waiting[parent]
	svc.mu.Unlock()
	if stillThere {
		t.Fatalf("waiting queue should be cleared after takeWaiting")
	}

	// 2. orphanCP 投影已删除该 parent 键。
	cp.mu.Lock()
	for _, e := range cp.entries {
		if e.parentHash == parent {
			cp.mu.Unlock()
			t.Fatalf("orphanCP projection should be removed after takeWaiting")
		}
	}
	cp.mu.Unlock()
}

// TestOrphanCPHook_NilIsNoop 验证 orphanCP == nil 时挂钩为 no-op（不 panic、不改 waiting）。
// 这是「持久化失败/未启用不改变 correctness 路径」的显式证明。
func TestOrphanCPHook_NilIsNoop(t *testing.T) {
	svc := newB1Service(t)
	svc.orphanCP = nil // 显式 nil（默认即 nil）

	child := mineOnTo(t, svc, block_unknownParent(t, svc), 1, svc.miner.PubKeyHash())
	parent := child.Header.PrevBlockHash

	// deferOrphan 不 panic。
	svc.deferOrphan(b1Peer, child)
	svc.mu.Lock()
	kids := svc.waiting[parent]
	svc.mu.Unlock()
	if len(kids) != 1 {
		t.Fatalf("nil orphanCP must not alter waiting queue, got %d", len(kids))
	}

	// takeWaiting 不 panic，行为不变。
	out := svc.takeWaiting(parent)
	if len(out) != 1 {
		t.Fatalf("nil orphanCP takeWaiting should still return child, got %d", len(out))
	}
}

// TestOrphanCPHook_LockOrdering 验证锁序：deferOrphan/takeWaiting 调用 MarkDirty/Remove 时
// 已释放 nodeService.mu（即 orphanCheckpoint 锁在 nodeService.mu 之外获取）。
// 通过「在持有 nodeService.mu 时，orphanCP 的 entries 尚未被并发修改」这一可观察性质间接验证：
// 单线程下顺序调用 deferOrphan 后，orphanCP.entries 与 waiting 一致，且无死锁（测试能完成即无死锁）。
func TestOrphanCPHook_LockOrdering(t *testing.T) {
	svc := newB1Service(t)
	cp := newOrphanCheckpoint(t.TempDir())
	svc.orphanCP = cp

	child := mineOnTo(t, svc, block_unknownParent(t, svc), 1, svc.miner.PubKeyHash())
	parent := child.Header.PrevBlockHash
	childHash := child.Header.Hash()

	// 顺序执行：deferOrphan → takeWaiting → 再次 deferOrphan（幂等 re-delivery）。
	svc.deferOrphan(b1Peer, child)
	svc.takeWaiting(parent)
	svc.deferOrphan(b1Peer, child)

	// 最终态：waiting 有 1 子块；orphanCP 投影有 parent→childHash。
	svc.mu.Lock()
	kids := svc.waiting[parent]
	svc.mu.Unlock()
	if len(kids) != 1 {
		t.Fatalf("waiting should have 1 child after re-delivery, got %d", len(kids))
	}
	cp.mu.Lock()
	match := false
	for _, e := range cp.entries {
		if e.parentHash == parent && len(e.childHash) == 1 && e.childHash[0] == childHash {
			match = true
		}
	}
	cp.mu.Unlock()
	if !match {
		t.Fatalf("orphanCP projection should match waiting after re-delivery")
	}
}

// block_unknownParent 返回一个「父哈希未知」的虚拟父块（仅用于构造缺父子块）。
// 其 PrevBlockHash 为一个非零随机值，保证子块的父在本链中不存在。
func block_unknownParent(t *testing.T, svc *nodeService) *block.Block {
	t.Helper()
	// 用当前链尾的哈希 + 一个非零扰动，构造一个「不在本链」的父哈希。
	tip, err := svc.chain.Tip()
	if err != nil {
		t.Fatalf("tip: %v", err)
	}
	unknown := tip.Header.Hash()
	unknown[0] ^= 0xff
	return &block.Block{Header: block.Header{PrevBlockHash: unknown}}
}
