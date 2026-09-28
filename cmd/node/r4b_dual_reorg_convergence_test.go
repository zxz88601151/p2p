package main

// REORG-1J-R4B · POSITIVE END-TO-END REORG CONVERGENCE
// ────────────────────────────────────────────────────────────────────────────
//
// 目标：证明两个**真实 node 操作系统进程**，从同一条 legacy 前缀出发、通过
// 真实 P2P TCP 交换**竞争分支**后，能沿**生产**代码路径
//
//	P2P 收块 → 区块校验 → blocktree 插入 → fork 检测 → ShouldReorg
//	        → DisconnectBlock/undo → SaveBlockDetached → CommitReorg
//	        → 持久 TIP → 重启恢复
//
// 确定性地收敛到同一个 canonical tip，且被重组节点的 **legacy 物理前缀逐字节不变**。
//
// 严格只读：本文件只新增测试，不修改任何生产代码（production diff = 0）。
//
// ── 几何（与 R4A 对齐）──────────────────────────────────────────────────────
//
//	L            = 被重组节点 A 的 legacy 记录条数（高度 0..L-1）
//	prefixB      = A 的历史截断到高度 L-2（L-1 条记录）⇒ A/B 真正的共同前缀
//	B 在 prefixB 之上再挖 2 块：
//	   Y @ 高度 L-1 —— **L-1 < L** ⇒ v2 块占据 legacy 逻辑槽位（R4A 场景）
//	   Z @ 高度 L   —— 累计工作量严格大于 A ⇒ 必然触发 reorg
//	fork 高度 f    = L-2
//
// 覆盖 L = 2 / 8 / 64（§7）。
//
// ── 生产路径真实性保证（§5）─────────────────────────────────────────────────
//
//   - 两个独立数据目录、独立端口、独立 blocks.dat / blocktree / UTXO；
//   - 真实 TCP（-seed）+ 真实二进制子进程，无 mock P2P；
//   - 绝不直接调用 CommitReorg，reorg 只能由网络收到的竞争块触发；
//   - A 全程不出块（无 -mine），其任何高度增长都只能来自 P2P。
// ────────────────────────────────────────────────────────────────────────────

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/blocktree"
	"p2pchain/internal/control"
	"p2pchain/internal/storage"
)

// r4bExtra 是 B 在共同前缀之上额外挖出的区块数（≥2 保证工作量严格更高，
// 且保证新分支一定含一个高度 < L 的块）。
const r4bExtra = 2

// ── blocks.dat 解析辅助（纯 legacy 记录视图）────────────────────────────────

// r4bReadFile 读取数据目录中的 blocks.dat。
func r4bReadFile(t *testing.T, dir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "blocks.dat"))
	if err != nil {
		t.Fatalf("读取 blocks.dat 失败: %v", err)
	}
	return data
}

// r4bLegacyCount 统计**连续 legacy 记录**条数；遇到 v2 帧（长度前缀非法）即停止。
// 用于证明 v2 追加不会改变 legacy 物理记录序列（I11）。
func r4bLegacyCount(data []byte) int {
	off, n := 0, 0
	for off+4 <= len(data) {
		l := int(binary.LittleEndian.Uint32(data[off : off+4]))
		if l <= 0 || off+4+l > len(data) {
			return n
		}
		off += 4 + l
		n++
	}
	return n
}

// r4bLegacyBlock 取第 idx 条 legacy 记录并解码为区块（idx 自 0 = 创世）。
func r4bLegacyBlock(t *testing.T, data []byte, idx int) *block.Block {
	t.Helper()
	off := 0
	for i := 0; i <= idx; i++ {
		if off+4 > len(data) {
			t.Fatalf("legacy 记录不足：需要第 %d 条（仅有 %d 条）", idx, i)
		}
		l := int(binary.LittleEndian.Uint32(data[off : off+4]))
		if l <= 0 || off+4+l > len(data) {
			t.Fatalf("第 %d 条 legacy 记录长度非法: %d（剩余 %d 字节）", i, l, len(data)-off-4)
		}
		if i == idx {
			b, err := block.DecodeBlock(data[off+4 : off+4+l])
			if err != nil {
				t.Fatalf("解码第 %d 条 legacy 记录失败: %v", i, err)
			}
			return b
		}
		off += 4 + l
	}
	t.Fatalf("unreachable: idx=%d", idx)
	return nil
}

// r4bAllBlocks 解码全部连续 legacy 记录。
func r4bAllBlocks(t *testing.T, data []byte) []*block.Block {
	t.Helper()
	n := r4bLegacyCount(data)
	out := make([]*block.Block, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, r4bLegacyBlock(t, data, i))
	}
	return out
}

func r4bHex(h [32]byte) string { return fmt.Sprintf("%x", h) }

// r4bCumWork 计算给定区块序列（自创世起）的累计工作量。
// 与 fork-choice 同源：使用 blocktree.WorkOfBits（ShouldReorg 的 CompareWork 基准）。
func r4bCumWork(blocks []*block.Block) *big.Int {
	c := new(big.Int)
	for _, b := range blocks {
		c.Add(c, blocktree.WorkOfBits(b.Header.Bits))
	}
	return c
}

// ── 只读存储事实 ────────────────────────────────────────────────────────────

func r4bOpen(t *testing.T, dir string) *storage.FileBlockStore {
	t.Helper()
	st, err := storage.OpenFileBlockStoreReadOnly(dir)
	if err != nil {
		t.Fatalf("只读打开 %s 失败: %v", dir, err)
	}
	return st
}

type r4bStoreFacts struct {
	height    int
	tipHash   string // 持久 TIP 帧指向的哈希；纯 legacy 链无 TIP 帧 ⇒ 空
	canonTip  string // canonical 视图的高度 height 处块哈希（legacy / v2 均适用）
	hasTip    bool
	legacyLen int
	records   int
	logSize   int64
	recovery  string
	chainwork string
}

func r4bFacts(t *testing.T, dir string) r4bStoreFacts {
	t.Helper()
	st := r4bOpen(t, dir)
	defer func() { _ = st.Close() }()
	h, err := st.Height()
	if err != nil {
		t.Fatalf("读取高度失败: %v", err)
	}
	// 纯 legacy 链没有 TIP 帧（canonical = legacy 记录序列），这是**合法**形态，
	// 不得误判为损坏；此时 canonical 链尾由 canonical 视图本身给出。
	tip, hasTip := st.TipHash()
	canonTip := ""
	if bh, err := st.GetBlockByHeight(h); err == nil {
		canonTip = r4bHex(bh.Header.Hash())
	}
	cw, _ := st.Chainwork()
	th := ""
	if hasTip {
		th = r4bHex(tip)
	}
	return r4bStoreFacts{
		height:    h,
		tipHash:   th,
		canonTip:  canonTip,
		hasTip:    hasTip,
		legacyLen: st.LegacyRecordCount(),
		records:   st.RecordCount(),
		logSize:   st.LogSize(),
		recovery:  st.RecoveryMode(),
		chainwork: cw.String(),
	}
}

// r4bHInfo 是 canonical 路径上单个高度的签名（§8：逐高度四项比对）。
type r4bHInfo struct {
	height int
	hash   string
	prev   string
	bits   uint32
	work   string
	cum    string
}

// r4bCanonPath 取某个数据目录 canonical 路径 0..h 的逐高度签名。
func r4bCanonPath(t *testing.T, dir string, h int) []r4bHInfo {
	t.Helper()
	st := r4bOpen(t, dir)
	defer func() { _ = st.Close() }()
	out := make([]r4bHInfo, 0, h+1)
	cum := new(big.Int)
	for i := 0; i <= h; i++ {
		b, err := st.GetBlockByHeight(i)
		if err != nil {
			t.Fatalf("读取 %s 高度 %d 失败: %v", dir, i, err)
		}
		w := blocktree.WorkOfBits(b.Header.Bits)
		cum.Add(cum, w)
		out = append(out, r4bHInfo{
			height: i,
			hash:   r4bHex(b.Header.Hash()),
			prev:   r4bHex(b.Header.PrevBlockHash),
			bits:   b.Header.Bits,
			work:   w.String(),
			cum:    new(big.Int).Set(cum).String(),
		})
	}
	return out
}

// r4bVerify 校验「legacy 物理前缀不可变 + 持久 TIP 正确」这一组硬断言。
func r4bVerify(t *testing.T, tag, dir string, wantHeight, wantLegacy int, wantTip string, legacyPrefix []byte) r4bStoreFacts {
	t.Helper()
	data := r4bReadFile(t, dir)
	if !bytes.HasPrefix(data, legacyPrefix) {
		t.Fatalf("%s：legacy 物理前缀被改写（文件 %d 字节，前缀 %d 字节不再匹配）", tag, len(data), len(legacyPrefix))
	}
	if got := r4bLegacyCount(data); got != wantLegacy {
		t.Fatalf("%s：legacy 记录条数 = %d, want %d", tag, got, wantLegacy)
	}
	f := r4bFacts(t, dir)
	if f.height != wantHeight {
		t.Fatalf("%s：持久高度 = %d, want %d", tag, f.height, wantHeight)
	}
	if f.canonTip != wantTip {
		t.Fatalf("%s：canonical 链尾 = %s, want %s", tag, f.canonTip, wantTip)
	}
	if f.legacyLen != wantLegacy {
		t.Fatalf("%s：LegacyRecordCount = %d, want %d（v2 占据 legacy 槽位不得改变 legacy 物理序列）", tag, f.legacyLen, wantLegacy)
	}
	if !verifyDir(t, dir) {
		t.Fatalf("%s：离线校验未通过（数据损坏）", tag)
	}
	return f
}

// ── 主用例 ──────────────────────────────────────────────────────────────────

// TestR4B_DualProcessCompetitiveForkConverges （§5–§10 正向端到端收敛证明）
//
// 两个真实进程、独立数据目录、真实 TCP；A 持 legacy 链（L 条记录），
// B 持共同前缀 L-1 条 + 2 枚新块。A 必须经网络完成 reorg 并与 B 收敛到同一 tip。
func TestR4B_DualProcessCompetitiveForkConverges(t *testing.T) {
	if testing.Short() {
		t.Skip("需要构建并启动真实 node 二进制")
	}
	for _, L := range []int{2, 8, 64} {
		L := L
		t.Run(fmt.Sprintf("L=%d", L), func(t *testing.T) {
			r4bRunCase(t, L)
		})
	}
}

func r4bRunCase(t *testing.T, L int) {
	t.Helper()

	// ── 阶段 0：构造共同前缀与竞争分支（全部由真实二进制产出）─────────────
	histA := mineChainOffline(t, L-1)                 // 高度 0..L-1 ⇒ L 条 legacy 记录
	prefixB := truncateLegacy(t, histA, L-2)          // 高度 0..L-2 ⇒ L-1 条（共同前缀）
	histB := extendChainOffline(t, prefixB, r4bExtra) // 高度 0..L

	if got := r4bLegacyCount(histA); got != L {
		t.Fatalf("A 历史 legacy 记录条数 = %d, want %d", got, L)
	}
	if got := r4bLegacyCount(prefixB); got != L-1 {
		t.Fatalf("共同前缀 legacy 记录条数 = %d, want %d", got, L-1)
	}
	if got := r4bLegacyCount(histB); got != L+1 {
		t.Fatalf("B 历史 legacy 记录条数 = %d, want %d", got, L+1)
	}
	if !bytes.HasPrefix(histA, prefixB) || !bytes.HasPrefix(histB, prefixB) {
		t.Fatal("前提不成立：两份历史必须共享同一份逐字节前缀")
	}

	blocksA := r4bAllBlocks(t, histA) // 0..L-1
	blocksB := r4bAllBlocks(t, histB) // 0..L

	// 竞争分叉结构（f = L-2）
	P := blocksA[L-2] // 共同祖先 @ L-2
	X := blocksA[L-1] // A 的高度 L-1 块（legacy 物理记录 index L-1）
	Y := blocksB[L-1] // B 的竞争块 @ L-1（< L ⇒ 将占据 legacy 逻辑槽位）
	Z := blocksB[L]   // B 的高度 L 块
	if r4bHex(X.Header.Hash()) == r4bHex(Y.Header.Hash()) {
		t.Fatal("前提不成立：高度 L-1 的两枚块必须不同（真实竞争分叉）")
	}
	if r4bHex(blocksB[L-2].Header.Hash()) != r4bHex(P.Header.Hash()) {
		t.Fatal("前提不成立：高度 L-2 不是共同祖先")
	}
	if r4bHex(Y.Header.PrevBlockHash) != r4bHex(P.Header.Hash()) ||
		r4bHex(X.Header.PrevBlockHash) != r4bHex(P.Header.Hash()) {
		t.Fatal("前提不成立：两枚竞争块必须同父")
	}
	if r4bHex(Z.Header.PrevBlockHash) != r4bHex(Y.Header.Hash()) {
		t.Fatal("前提不成立：B 的高度 L 块必须链接到竞争块 Y")
	}

	oldWork := new(big.Int).Set(r4bCumWork(blocksA))
	newWork := new(big.Int).Set(r4bCumWork(blocksB))
	if newWork.Cmp(oldWork) <= 0 {
		t.Fatalf("前提不成立：新分支累计工作量 %s 未严格超过旧链 %s", newWork, oldWork)
	}

	// ── 阶段 1：真实双进程（真实 TCP，A 以 B 为种子）────────────────────────
	//
	// 关键点：A 的「reorg 前 old tip」必须在**建立 P2P 连接之前**捕获。
	// 握手日志出现的那一刻同步就可能已经完成，因此这里先把 A 单独启动一次
	// 取 pre-state（该次启动不挖矿、不写盘，随后优雅停止），再以 -seed 重启。
	dirA, dirB := t.TempDir(), t.TempDir()
	for dir, data := range map[string][]byte{dirA: histA, dirB: histB} {
		if err := os.WriteFile(filepath.Join(dir, "blocks.dat"), data, 0o644); err != nil {
			t.Fatalf("写入 blocks.dat 失败: %v", err)
		}
	}

	a0 := startRealNode(t, dirA)
	waitRealHeight(t, a0, L-1, 30*time.Second)
	stA0, err := a0.status()
	if err != nil {
		t.Fatalf("取 A 初始状态失败: %v\n输出:\n%s", err, a0.output.String())
	}
	oldTipA := stA0.TipHash
	if stA0.Height != L-1 || oldTipA != r4bHex(X.Header.Hash()) {
		t.Fatalf("A 初始状态 = (h=%d, tip=%s), want (h=%d, tip=%s)",
			stA0.Height, oldTipA, L-1, r4bHex(X.Header.Hash()))
	}
	stopRealNode(t, a0)
	if !bytes.Equal(r4bReadFile(t, dirA), histA) {
		t.Fatal("A 在「仅加载历史」阶段修改了 blocks.dat（启动期写盘不应发生）")
	}

	b := startRealNode(t, dirB)
	waitRealHeight(t, b, L, 30*time.Second)
	stB0, err := b.status()
	if err != nil {
		t.Fatalf("取 B 初始状态失败: %v\n输出:\n%s", err, b.output.String())
	}
	competingTipB := stB0.TipHash
	if stB0.Height != L || competingTipB != r4bHex(Z.Header.Hash()) {
		t.Fatalf("B 初始状态 = (h=%d, tip=%s), want (h=%d, tip=%s)",
			stB0.Height, competingTipB, L, r4bHex(Z.Header.Hash()))
	}

	a := startRealNode(t, dirA, "-seed", b.p2pListenAddr())
	// 握手证据：双方都应看到带工作量字段的握手日志
	waitRealLog(t, a, "对端工作量=", 30*time.Second)
	waitRealLog(t, b, "对端工作量=", 30*time.Second)
	// 真实连接证据
	waitRealCond(t, a, func(s *control.StatusInfo) bool { return len(s.Peers) > 0 }, 30*time.Second, "A 建立真实 P2P 连接")
	waitRealCond(t, b, func(s *control.StatusInfo) bool { return len(s.Peers) > 0 }, 30*time.Second, "B 建立真实 P2P 连接")

	// ── 阶段 2：等待经网络触发的 reorg 收敛 ───────────────────────────────
	wantTip := r4bHex(Z.Header.Hash())
	deadline := time.Now().Add(90 * time.Second)
	var fa, fb *control.StatusInfo
	for time.Now().Before(deadline) {
		select {
		case err := <-a.exited:
			t.Fatalf("节点 A 在收敛等待期退出: %v\n输出:\n%s", err, a.output.String())
		case err := <-b.exited:
			t.Fatalf("节点 B 在收敛等待期退出: %v\n输出:\n%s", err, b.output.String())
		default:
		}
		sa, ea := a.status()
		sb, eb := b.status()
		if ea == nil && eb == nil && sa.Height == L && sb.Height == L &&
			sa.TipHash == wantTip && sb.TipHash == wantTip {
			fa, fb = sa, sb
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if fa == nil || fb == nil {
		sa, _ := a.status()
		sb, _ := b.status()
		t.Fatalf("R4B：双进程未在 90s 内收敛\nA: h=%v tip=%v\nB: h=%v tip=%v\nwant h=%d tip=%s\n"+
			"A 输出:\n%s\nB 输出:\n%s", sa.Height, sa.TipHash, sb.Height, sb.TipHash, L, wantTip,
			a.output.String(), b.output.String())
	}

	// A 未出块 ⇒ 高度增长只能来自 P2P（§5）
	if strings.Contains(a.output.String(), "MINING_BLOCK_ACCEPTED") {
		t.Fatalf("A 不应自行出块：本次收敛必须完全来自 P2P 投递\n%s", a.output.String())
	}
	// 不得出现已废止的 legacy 前缀拒绝路径（§13）
	if strings.Contains(a.output.String(), "legacy 前缀不可变") ||
		strings.Contains(b.output.String(), "legacy 前缀不可变") {
		t.Fatalf("仍观察到 legacy 前缀拒绝痕迹\nA:\n%s\nB:\n%s", a.output.String(), b.output.String())
	}

	// ── 阶段 3：优雅停止并做离线硬校验 ────────────────────────────────────
	stopRealNode(t, a)
	stopRealNode(t, b)

	fA := r4bVerify(t, "A", a.dir, L, L, wantTip, histA)
	fB := r4bVerify(t, "B", b.dir, L, L+1, wantTip, histB)
	// A 必须真的落了一枚持久 TIP 帧（CommitReorg 的唯一提交点，R3 定稿）
	if !fA.hasTip {
		t.Fatalf("A 收敛后 blocks.dat 中无 TIP 帧：reorg 未产生持久 canonical 提交（%v）", fA)
	}
	// B 全程无 v2 写入（其链 100% legacy）⇒ 无 TIP 帧是**合法**形态，不得判为失败。
	if fB.hasTip {
		t.Fatalf("B 不应产生 v2 帧（其链应仍为纯 legacy）：%v", fB)
	}

	// §7 核心：v2 竞争块必须占据 A 的 legacy 逻辑高度 L-1，
	// 而该槽位的 legacy **物理**记录仍逐字节是 X。
	stA := r4bOpen(t, a.dir)
	atFork, err := stA.GetBlockByHeight(L - 1)
	if err != nil {
		t.Fatalf("读取 A 高度 %d 失败: %v", L-1, err)
	}
	if r4bHex(atFork.Header.Hash()) != r4bHex(Y.Header.Hash()) {
		t.Fatalf("A 的高度 %d canonical 块 = %s, want 竞争块 %s（v2 未占据 legacy 逻辑槽位）",
			L-1, r4bHex(atFork.Header.Hash()), r4bHex(Y.Header.Hash()))
	}
	canonX, err := stA.IsCanonical(X.Header.Hash())
	if err != nil {
		t.Fatalf("查询 X canonical 归属失败: %v", err)
	}
	if canonX {
		t.Fatalf("被取代的 legacy 块 %s 仍标记为 canonical", r4bHex(X.Header.Hash())[:12])
	}
	if !stA.HasBlock(X.Header.Hash()) {
		t.Fatalf("被取代的 legacy 块 %s 不应被物理删除（逻辑墓碑语义）", r4bHex(X.Header.Hash())[:12])
	}
	_ = stA.Close()

	physX := r4bLegacyBlock(t, r4bReadFile(t, a.dir), L-1)
	if r4bHex(physX.Header.Hash()) != r4bHex(X.Header.Hash()) {
		t.Fatalf("legacy 物理记录 index %d 被改写：%s != %s",
			L-1, r4bHex(physX.Header.Hash()), r4bHex(X.Header.Hash()))
	}

	// §8：canonical 路径逐高度四项签名必须完全一致
	pathA := r4bCanonPath(t, a.dir, L)
	pathB := r4bCanonPath(t, b.dir, L)
	for i := range pathA {
		if pathA[i] != pathB[i] {
			t.Fatalf("canonical 路径在高度 %d 分叉：\n A = %+v\n B = %+v", i, pathA[i], pathB[i])
		}
	}

	// §6：detached / attached 高度
	var detached, attached []int
	for i := 0; i <= L; i++ {
		if i < len(blocksA) && r4bHex(blocksA[i].Header.Hash()) != pathA[i].hash {
			detached = append(detached, i)
			attached = append(attached, i)
		} else if i >= len(blocksA) {
			attached = append(attached, i)
		}
	}
	if len(detached) == 0 || len(attached) == 0 {
		t.Fatalf("未观测到 reorg 拓扑变化：detached=%v attached=%v", detached, attached)
	}

	// ── 阶段 4：重启收敛（§10）───────────────────────────────────────────
	for round := 1; round <= 2; round++ {
		a2 := startRealNode(t, a.dir)
		waitRealHeight(t, a2, L, 30*time.Second)
		sa2, err := a2.status()
		if err != nil {
			t.Fatalf("重启第 %d 轮取 A 状态失败: %v\n输出:\n%s", round, err, a2.output.String())
		}
		b2 := startRealNode(t, b.dir)
		waitRealHeight(t, b2, L, 30*time.Second)
		sb2, err := b2.status()
		if err != nil {
			t.Fatalf("重启第 %d 轮取 B 状态失败: %v\n输出:\n%s", round, err, b2.output.String())
		}
		if sa2.Height != L || sb2.Height != L || sa2.TipHash != wantTip || sb2.TipHash != wantTip {
			t.Fatalf("重启第 %d 轮未复现收敛签名：A=(h=%d,tip=%s) B=(h=%d,tip=%s) want (h=%d,tip=%s)",
				round, sa2.Height, sa2.TipHash, sb2.Height, sb2.TipHash, L, wantTip)
		}
		stopRealNode(t, a2)
		stopRealNode(t, b2)

		// 重启后 legacy 物理前缀与持久 TIP 仍不得变化
		gA := r4bVerify(t, fmt.Sprintf("A/重启%d", round), a.dir, L, L, wantTip, histA)
		gB := r4bVerify(t, fmt.Sprintf("B/重启%d", round), b.dir, L, L+1, wantTip, histB)
		if gA.canonTip != fA.canonTip || gA.height != fA.height || gA.legacyLen != fA.legacyLen {
			t.Fatalf("重启第 %d 轮后 A 持久签名漂移：%v → %v", round, fA, gA)
		}
		if gB.canonTip != fB.canonTip || gB.height != fB.height || gB.legacyLen != fB.legacyLen {
			t.Fatalf("重启第 %d 轮后 B 持久签名漂移：%v → %v", round, fB, gB)
		}
	}

	// ── 证据输出（§6 要求的全部字段）─────────────────────────────────────
	t.Logf("R4B L=%d 收敛证据", L)
	t.Logf("  old tip（A，reorg 前）      = h=%d %s", L-1, oldTipA)
	t.Logf("  competing tip（B）          = h=%d %s", L, competingTipB)
	t.Logf("  fork height f               = %d（共同祖先 %s）", L-2, r4bHex(P.Header.Hash())[:12])
	t.Logf("  old chain work              = %s", oldWork)
	t.Logf("  new chain work              = %s", newWork)
	t.Logf("  selected tip                = h=%d %s", L, wantTip)
	t.Logf("  detached heights            = %v（A 原 canonical 块离开）", detached)
	t.Logf("  attached heights            = %v（含高度 %d < L=%d ⇒ v2 占 legacy 逻辑槽位）", attached, L-1, L)
	t.Logf("  A 持久：%d 字节 / legacy=%d 条 / records=%d / TIP帧=%s / canonicalTip=%s / recovery=%s",
		fA.logSize, fA.legacyLen, fA.records, fA.tipHash, fA.canonTip, fA.recovery)
	t.Logf("  B 持久：%d 字节 / legacy=%d 条 / records=%d / TIP帧=%s / canonicalTip=%s / recovery=%s",
		fB.logSize, fB.legacyLen, fB.records, fB.tipHash, fB.canonTip, fB.recovery)
	t.Logf("  canonical 路径（%d 项）A == B：%v", len(pathA), true)
	for i := 0; i <= L; i++ {
		if i < 3 || i >= L-2 {
			t.Logf("    h=%d hash=%s prev=%s bits=%d cumWork=%s",
				pathA[i].height, pathA[i].hash[:12], pathA[i].prev[:12], pathA[i].bits, pathA[i].cum)
		}
	}
}

// TestR4B_ProductionReorgPathTraversed 断言 reorg 真的走了生产路径，
// 而不是「高度相同 = 没发生 reorg」这种假阳性。
//
// 判据（全部基于离线只读事实，不依赖日志措辞）：
//  1. A 的 blocks.dat 在收敛后**变长**（v2 帧被追加）——说明确实发生了持久化写；
//  2. A 的 legacy 记录条数不变、前缀逐字节不变；
//  3. A 的 canonical 在 fork 高度处由 X 翻转为 Y。
func TestR4B_ProductionReorgPathTraversed(t *testing.T) {
	if testing.Short() {
		t.Skip("需要构建并启动真实 node 二进制")
	}
	const L = 8
	histA := mineChainOffline(t, L-1)
	prefixB := truncateLegacy(t, histA, L-2)
	histB := extendChainOffline(t, prefixB, r4bExtra)

	blocksA := r4bAllBlocks(t, histA)
	blocksB := r4bAllBlocks(t, histB)
	X, Y, Z := blocksA[L-1], blocksB[L-1], blocksB[L]

	a, b := startPairWithHistory(t, histA, histB)
	wantTip := r4bHex(Z.Header.Hash())
	deadline := time.Now().Add(90 * time.Second)
	converged := false
	for time.Now().Before(deadline) {
		sa, ea := a.status()
		sb, eb := b.status()
		if ea == nil && eb == nil && sa.Height == L && sb.Height == L && sa.TipHash == wantTip && sb.TipHash == wantTip {
			converged = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !converged {
		sa, _ := a.status()
		sb, _ := b.status()
		t.Fatalf("未收敛：A=(h=%d,tip=%s) B=(h=%d,tip=%s)\nA:\n%s\nB:\n%s",
			sa.Height, sa.TipHash, sb.Height, sb.TipHash, a.output.String(), b.output.String())
	}
	stopRealNode(t, a)
	stopRealNode(t, b)

	after := r4bReadFile(t, a.dir)
	if len(after) <= len(histA) {
		t.Fatalf("A 的 blocks.dat 未增长（%d → %d）：未发生任何持久化 reorg 写入", len(histA), len(after))
	}
	if !bytes.HasPrefix(after, histA) {
		t.Fatal("A 的 legacy 物理前缀被改写")
	}
	if got := r4bLegacyCount(after); got != L {
		t.Fatalf("A legacy 记录条数 = %d, want %d", got, L)
	}
	st := r4bOpen(t, a.dir)
	defer func() { _ = st.Close() }()
	got, err := st.GetBlockByHeight(L - 1)
	if err != nil {
		t.Fatalf("读取 A 高度 %d 失败: %v", L-1, err)
	}
	if r4bHex(got.Header.Hash()) != r4bHex(Y.Header.Hash()) {
		t.Fatalf("fork 高度 canonical 未翻转：got=%s want=%s（X=%s）",
			r4bHex(got.Header.Hash()), r4bHex(Y.Header.Hash()), r4bHex(X.Header.Hash()))
	}
	t.Logf("R4B 生产路径证据：A blocks.dat %d → %d 字节（+v2 帧），legacy 前缀 %d 字节逐字节不变，fork@%d 由 X=%s 翻转为 Y=%s",
		len(histA), len(after), len(histA), L-1, r4bHex(X.Header.Hash())[:12], r4bHex(Y.Header.Hash())[:12])
}
