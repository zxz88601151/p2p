// orphan_durability_test.go 覆盖 §4-B1.5 DURABILITY ACTIVATION 的 checkpoint 生命周期验收。
//
// 范围（与授权一致）：
//   - D1 生产 node 生命周期内 orphanCheckpoint 初始化 + 启动加载；
//   - D2 生产 Flush 生命周期（关机边界落盘 + 周期节流）；
//   - 缺失文件 / corrupt 文件 fail-closed；
//   - 对 canonical 链零影响。
// 明确不涉及：OnHandshake 变更、网络行为、块请求/恢复、共识路径、reorg/fork-choice。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestOrphanDurability_RuntimeInit 验证 D1：生产 newNodeRuntime 装配 orphanCP，
// 缺失检查点文件时不报错、restorePending 为空、且不创建孤儿文件。
func TestOrphanDurability_RuntimeInit(t *testing.T) {
	dir := t.TempDir()
	if code := cmdInit([]string{"-datadir", dir}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("init failed: code=%d", code)
	}
	rt, err := newNodeRuntime(nodeConfig{DataDir: dir, ListenAddr: "127.0.0.1:0", RPCAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("newNodeRuntime: %v", err)
	}
	defer rt.Close()

	if rt.svc.orphanCP == nil {
		t.Fatalf("orphanCP must be initialized in production runtime (D1)")
	}
	if len(rt.svc.restorePending) != 0 {
		t.Fatalf("fresh node should have empty restorePending, got %d", len(rt.svc.restorePending))
	}
	if _, statErr := os.Stat(rt.svc.orphanCP.Path()); !os.IsNotExist(statErr) {
		t.Fatalf("orphan_waiting.bin should not exist for fresh node (no orphans yet)")
	}
}

// TestOrphanDurability_RuntimeInitLoadsCheckpoint 验证 D1 启动加载：预置有效检查点后，
// 节点启动把未知父载入 restorePending（fail-closed 的反面：有效文件被加载）。
func TestOrphanDurability_RuntimeInitLoadsCheckpoint(t *testing.T) {
	dir := t.TempDir()
	if code := cmdInit([]string{"-datadir", dir}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("init failed")
	}
	parent := [32]byte{0xab}
	child := [32]byte{0xcd}
	seedCP := newOrphanCheckpoint(dir)
	seedCP.MarkDirty(parent, child)
	if err := seedCP.Flush(); err != nil {
		t.Fatalf("seed flush: %v", err)
	}

	rt, err := newNodeRuntime(nodeConfig{DataDir: dir, ListenAddr: "127.0.0.1:0", RPCAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("newNodeRuntime: %v", err)
	}
	defer rt.Close()

	if rt.svc.orphanCP == nil {
		t.Fatalf("orphanCP must be initialized")
	}
	if _, ok := rt.svc.restorePending[parent]; !ok {
		t.Fatalf("restorePending must contain pre-seeded unknown parent")
	}
	if len(rt.svc.restorePending) != 1 {
		t.Fatalf("restorePending should be exactly 1, got %d", len(rt.svc.restorePending))
	}
}

// TestOrphanDurability_PersistenceAcrossRestart 验证 D2：停车一个孤儿→干净关机落盘→
// 重启加载到同一检查点（persistence across restart）。
func TestOrphanDurability_PersistenceAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	if code := cmdInit([]string{"-datadir", dir}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("init failed")
	}
	parent := [32]byte{0xab}
	child := [32]byte{0xcd}

	// 第一次启动：模拟 deferOrphan 的持久投影（直接 MarkDirty），干净关机触发 D2 落盘。
	rt1, err := newNodeRuntime(nodeConfig{DataDir: dir, ListenAddr: "127.0.0.1:0", RPCAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("newNodeRuntime#1: %v", err)
	}
	rt1.svc.orphanCP.MarkDirty(parent, child)
	rt1.Close() // 关机 flush

	// 重启：应加载到同一检查点。
	rt2, err := newNodeRuntime(nodeConfig{DataDir: dir, ListenAddr: "127.0.0.1:0", RPCAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("newNodeRuntime#2: %v", err)
	}
	defer rt2.Close()

	if _, ok := rt2.svc.restorePending[parent]; !ok {
		t.Fatalf("restart must reload parent into restorePending")
	}
	// 检查点文件可被独立句柄解析出该 entry（证明落盘真实发生）。
	reload := newOrphanCheckpoint(dir)
	entries, err := reload.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(entries) != 1 || entries[0].parentHash != parent ||
		len(entries[0].childHash) != 1 || entries[0].childHash[0] != child {
		t.Fatalf("checkpoint file mismatch after restart: %+v", entries)
	}
}

// TestOrphanDurability_CorruptCheckpointFailClosed 验证 corrupt 检查点 fail-closed：
// 节点仍正常启动、restorePending 为空、canonical 链保持创世（高度 0，tip 有效）。
func TestOrphanDurability_CorruptCheckpointFailClosed(t *testing.T) {
	dir := t.TempDir()
	if code := cmdInit([]string{"-datadir", dir}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("init failed")
	}
	cpPath := filepath.Join(dir, "orphan_waiting.bin")
	if err := os.WriteFile(cpPath, []byte("this-is-not-a-valid-orphan-checkpoint"), 0o600); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	rt, err := newNodeRuntime(nodeConfig{DataDir: dir, ListenAddr: "127.0.0.1:0", RPCAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("newNodeRuntime must not fail on corrupt checkpoint: %v", err)
	}
	defer rt.Close()

	if rt.svc.orphanCP == nil {
		t.Fatalf("orphanCP must be initialized even with corrupt checkpoint")
	}
	if len(rt.svc.restorePending) != 0 {
		t.Fatalf("corrupt checkpoint must yield empty restorePending (fail-closed), got %d", len(rt.svc.restorePending))
	}
	h := rt.svc.chain.Height()
	if h != 0 {
		t.Fatalf("canonical height must remain 0 after corrupt checkpoint, got %d", h)
	}
	tip, err := rt.svc.chain.Tip()
	if err != nil {
		t.Fatalf("tip: %v", err)
	}
	if tip.Header.Hash() == ([32]byte{}) {
		t.Fatalf("tip must be valid genesis, got zero hash")
	}
}

// TestOrphanDurability_NoCanonicalImpact 在 svc 层隔离验证：检查点 MarkDirty/Flush/Remove
// 绝不触碰 canonical 链（高度与链尾不变）。
func TestOrphanDurability_NoCanonicalImpact(t *testing.T) {
	svc := newB1Service(t)
	cp := newOrphanCheckpoint(t.TempDir())
	svc.orphanCP = cp

	h0 := svc.chain.Height()
	if h0 < 0 {
		t.Fatalf("height0 invalid")
	}
	tip0, err := svc.chain.Tip()
	if err != nil {
		t.Fatalf("tip0: %v", err)
	}

	cp.MarkDirty([32]byte{0xab}, [32]byte{0xcd})
	cp.MarkDirty([32]byte{0xab}, [32]byte{0xef})
	if err := cp.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	cp.Remove([32]byte{0xab})
	if err := cp.Flush(); err != nil {
		t.Fatalf("flush2: %v", err)
	}

	h1 := svc.chain.Height()
	tip1, _ := svc.chain.Tip()
	if h1 != h0 {
		t.Fatalf("chain height changed by checkpoint ops: %d -> %d", h0, h1)
	}
	if tip1.Header.Hash() != tip0.Header.Hash() {
		t.Fatalf("chain tip changed by checkpoint ops")
	}
}

// --- 数据层回归守卫（锁定 §4-B1.5 D2 的 load 投影修正，防止回退为「首次 Flush 清空恢复集」） ---

// TestOrphanCP_LoadHydratesProjection 验证 Load 把磁盘投影灌入内存唯一事实源，
// 使得后续 Flush 在已加载条目上增量合并，而非从空态覆盖。
func TestOrphanCP_LoadHydratesProjection(t *testing.T) {
	c, _ := newTestCP(t)
	c.MarkDirty([32]byte{0x01}, [32]byte{0xaa})
	if err := c.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	// 新句柄从同一文件路径加载。
	c2 := newOrphanCheckpoint(filepath.Dir(c.Path()))
	if _, err := c2.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	c2.mu.Lock()
	defer c2.mu.Unlock()
	if len(c2.entries) != 1 {
		t.Fatalf("loaded projection should have 1 entry, got %d", len(c2.entries))
	}
	if c2.entries[0].parentHash != ([32]byte{0x01}) {
		t.Fatalf("loaded projection wrong parent: %x", c2.entries[0].parentHash)
	}
}

// TestOrphanCP_FlushIfDirtyThrottle 验证 FlushIfDirty 的脏状态/节流语义。
func TestOrphanCP_FlushIfDirtyThrottle(t *testing.T) {
	c, _ := newTestCP(t)
	// 未脏 → 不写。
	if wrote, err := c.FlushIfDirty(1 << 30); wrote || err != nil {
		t.Fatalf("clean checkpoint must not flush: wrote=%v err=%v", wrote, err)
	}
	// 脏 + 首次（lastFlush 为零）→ 立即写。
	c.MarkDirty([32]byte{0x01}, [32]byte{0xaa})
	if wrote, err := c.FlushIfDirty(1 << 30); !wrote || err != nil {
		t.Fatalf("dirty first flush must write: wrote=%v err=%v", wrote, err)
	}
	// 再次变脏但间隔未到（1e9s）→ 不写（节流）。
	c.MarkDirty([32]byte{0x02}, [32]byte{0xbb})
	if wrote, _ := c.FlushIfDirty(1 << 30); wrote {
		t.Fatalf("must throttle within interval")
	}
}
