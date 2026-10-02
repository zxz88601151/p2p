// orphan_restore_test.go 覆盖 ORPHAN-DURABILITY-IMPLEMENTATION-1 §4-B1 恢复状态层测试。
//
// 测试目标（§4-B1 Requirements）：
//   - restorePending 去重
//   - 已在链（canonical）父键跳过
//   - 空检查点行为
//   - 损坏检查点行为（fail-closed）
//   - nil 检查点行为
//   - 不重建 waiting、不伪造块指针（prepareOrphanRestore 只填 restorePending）
package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOrphanRestore_Dedup 验证 restorePending 对重复父键去重（map 语义）。
func TestOrphanRestore_Dedup(t *testing.T) {
	svc := newServiceFor(t, testChain(t))
	cp := newOrphanCheckpoint(t.TempDir())

	// 同一 parent 两个 child（MarkDirty 同键追加），再一个独立 parent。
	p1 := [32]byte{0x01}
	p2 := [32]byte{0x02}
	cp.MarkDirty(p1, [32]byte{0xaa})
	cp.MarkDirty(p1, [32]byte{0xbb}) // 同 p1 追加
	cp.MarkDirty(p2, [32]byte{0xcc})
	if err := cp.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	svc.prepareOrphanRestore(cp)

	if len(svc.restorePending) != 2 {
		t.Fatalf("restorePending 应含 2 个去重父键，实际 %d", len(svc.restorePending))
	}
	if _, ok := svc.restorePending[p1]; !ok {
		t.Fatalf("p1 应在 restorePending 中")
	}
	if _, ok := svc.restorePending[p2]; !ok {
		t.Fatalf("p2 应在 restorePending 中")
	}
}

// TestOrphanRestore_SkipKnownParent 验证已在 canonical 链的父键被跳过。
func TestOrphanRestore_SkipKnownParent(t *testing.T) {
	svc := newServiceFor(t, testChain(t))
	cp := newOrphanCheckpoint(t.TempDir())

	// 取创世块哈希（canonical，HasBlockHash 应命中）。
	genesis, err := svc.chain.BlockByHeight(0)
	if err != nil {
		t.Fatalf("genesis: %v", err)
	}
	known := genesis.Header.Hash()
	unknown := [32]byte{0x77}

	cp.MarkDirty(known, [32]byte{0xaa})
	cp.MarkDirty(unknown, [32]byte{0xbb})
	if err := cp.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	svc.prepareOrphanRestore(cp)

	if _, ok := svc.restorePending[known]; ok {
		t.Fatalf("已在 canonical 的父键不应出现在 restorePending")
	}
	if _, ok := svc.restorePending[unknown]; !ok {
		t.Fatalf("未知父键应出现在 restorePending")
	}
	if len(svc.restorePending) != 1 {
		t.Fatalf("restorePending 应仅含 1 个未知父键，实际 %d", len(svc.restorePending))
	}
}

// TestOrphanRestore_EmptyCheckpoint 验证无文件/空检查点 ⇒ restorePending 为空，不报错。
func TestOrphanRestore_EmptyCheckpoint(t *testing.T) {
	svc := newServiceFor(t, testChain(t))
	cp := newOrphanCheckpoint(t.TempDir()) // 不写任何文件

	svc.prepareOrphanRestore(cp)

	if len(svc.restorePending) != 0 {
		t.Fatalf("空检查点应得空 restorePending，实际 %d", len(svc.restorePending))
	}
}

// TestOrphanRestore_CorruptCheckpoint 验证损坏检查点 ⇒ fail-closed，restorePending 为空，不 panic。
func TestOrphanRestore_CorruptCheckpoint(t *testing.T) {
	svc := newServiceFor(t, testChain(t))
	dir := t.TempDir()
	cp := newOrphanCheckpoint(dir)

	// 写一个损坏文件（garbage 字节）。
	if err := os.WriteFile(filepath.Join(dir, "orphan_waiting.bin"), []byte("garbage-not-a-checkpoint"), 0o600); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	// 不 panic，restorePending 为空。
	svc.prepareOrphanRestore(cp)

	if len(svc.restorePending) != 0 {
		t.Fatalf("损坏检查点应得空 restorePending，实际 %d", len(svc.restorePending))
	}
}

// TestOrphanRestore_NilCheckpoint 验证 nil 检查点 ⇒ restorePending 为空，不 panic。
func TestOrphanRestore_NilCheckpoint(t *testing.T) {
	svc := newServiceFor(t, testChain(t))
	svc.prepareOrphanRestore(nil)
	if len(svc.restorePending) != 0 {
		t.Fatalf("nil 检查点应得空 restorePending，实际 %d", len(svc.restorePending))
	}
}

// TestOrphanRestore_NoWaitingReconstruction 验证 prepareOrphanRestore 不重建 s.waiting、
// 不伪造块指针（核心不变量：恢复只填 restorePending，不动 waiting）。
func TestOrphanRestore_NoWaitingReconstruction(t *testing.T) {
	svc := newServiceFor(t, testChain(t))
	cp := newOrphanCheckpoint(t.TempDir())
	cp.MarkDirty([32]byte{0x01}, [32]byte{0xaa})
	cp.MarkDirty([32]byte{0x02}, [32]byte{0xbb})
	if err := cp.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	svc.prepareOrphanRestore(cp)

	svc.mu.Lock()
	nWaiting := len(svc.waiting)
	svc.mu.Unlock()
	if nWaiting != 0 {
		t.Fatalf("prepareOrphanRestore 不得重建 s.waiting，实际 %d 键", nWaiting)
	}
	if len(svc.restorePending) != 2 {
		t.Fatalf("restorePending 应含 2 父键，实际 %d", len(svc.restorePending))
	}
}
