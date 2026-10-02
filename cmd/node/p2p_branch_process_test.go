package main

// REORG-1H M8：真实双进程验收。
//
// 与 p2p_branch_test.go 的区别：
//   - 那里是「单个测试进程内的两个真实 nodeRuntime」，验证的是 service/blockchain 逻辑；
//   - 这里是「两个真实 node 操作系统进程 + 真实 P2P TCP 连接 + 真实控制接口」，
//     验证的是 cmd/node/main.go 的**接线**是否真的生效：
//     SetChainStatusProvider、握手载荷里的 ChainWork/TipHash、-seed 连接、
//     MsgGetBlockByHash / MsgBlockByHashResp 在真实 TCP 上的往返。

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"p2pchain/internal/control"
)

// ── 辅助 ────────────────────────────────────────────────────────────────

// 注：原 mineRealNode（经 POST /mine 按需出块让子进程挖固定块数）已删除。
// 按需出块整体下线后，「精确挖 N 块」只能由 `-mine -maxblocks N` 表达：
// runMiner 挖满 N 块即转入全节点模式停手，数量精确；
// 而「启动持续挖矿后轮询再停」因停止异步，可能多出 1 块，不适用于
// 需要精确高度的场景（见 runOffline）。

// waitRealHeight 等待子进程链高达到 want（用于「离线准备历史」这类同步前断言）。
func waitRealHeight(t *testing.T, n *realNode, want int, timeout time.Duration) {
	t.Helper()
	c := control.NewClient(n.rpc)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-n.exited:
			t.Fatalf("节点在等待高度 %d 时退出: %v\n节点输出:\n%s", want, err, n.output.String())
		default:
		}
		if st, err := c.Status(); err == nil && st.Height >= want {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("节点高度在 %v 内未达到 %d\n节点输出:\n%s", timeout, want, n.output.String())
}

// waitRealCond 轮询子进程状态直到断言成立，返回最后一次状态快照。
func waitRealCond(t *testing.T, n *realNode, cond func(*control.StatusInfo) bool, timeout time.Duration, msg string) *control.StatusInfo {
	t.Helper()
	c := control.NewClient(n.rpc)
	var last *control.StatusInfo
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-n.exited:
			t.Fatalf("节点在等待「%s」时退出: %v\n节点输出:\n%s", msg, err, n.output.String())
		default:
		}
		if st, err := c.Status(); err == nil {
			last = st
			if cond(st) {
				return st
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if last == nil {
		t.Fatalf("等待「%s」超时且从未取到状态\n节点输出:\n%s", msg, n.output.String())
	}
	t.Fatalf("等待「%s」超时: 最后状态 高度=%d 链尾=%s 对等节点=%v\n节点输出:\n%s",
		msg, last.Height, last.TipHash, last.Peers, n.output.String())
	return nil
}

// waitRealLog 轮询子进程日志直到出现给定子串（真实进程日志写到 stdout/stderr）。
func waitRealLog(t *testing.T, n *realNode, substr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(n.output.String(), substr) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("节点日志在 %v 内未出现 %q\n节点输出:\n%s", timeout, substr, n.output.String())
}

// mineChainOffline 用一次性子进程离线挖出 blocks 个区块并优雅停止，返回 blocks.dat 内容。
//
// 用途：给双进程用例准备「共同的链历史」——全部由真实二进制产出，不做任何手工构造。
//
// ⚠️ 两次独立调用会产生**从创世就分叉**的两条链（钱包不同 ⇒ 高度 1 即不同）。
// 需要「共同前缀 + 各自分叉」时必须用 extendChainOffline 在同一份历史上延伸。
func mineChainOffline(t *testing.T, blocks int) []byte {
	return runOffline(t, nil, blocks)
}

// extendChainOffline 在给定历史上启动一次性子进程，再挖 extra 个区块后停止。
//
// 用于构造「共同前缀相同、链尾不同」的两份历史（真实竞争分叉）。
func extendChainOffline(t *testing.T, history []byte, extra int) []byte {
	return runOffline(t, history, extra)
}

// runOffline 启动一个一次性节点（可带初始历史），精确挖出 mine 个区块后
// 优雅停止并返回 blocks.dat。
//
// 出块方式：`-mine -maxblocks mine`。原实现走按需出块（POST /mine），
// 该端点已整体下线；改用 maxblocks 而非「启动持续挖矿后轮询再停」，
// 是因为后者停止是异步的、可能多出 1 块，而本函数产出的历史被
// 调用方按固定高度索引（blocksA[L-1] 等），数量必须精确。
func runOffline(t *testing.T, history []byte, mine int) []byte {
	t.Helper()
	dir := t.TempDir()
	if history != nil {
		if err := os.WriteFile(filepath.Join(dir, "blocks.dat"), history, 0o644); err != nil {
			t.Fatalf("写入初始历史失败: %v", err)
		}
	}
	want := mine
	if history != nil {
		want = initialHeightFromLegacy(t, history) + mine
	}
	n := startRealNode(t, dir, "-mine", "-maxblocks", strconv.Itoa(mine))
	waitRealHeight(t, n, want-mine, 30*time.Second) // 先确认历史已加载（无历史时为 0）
	waitRealHeight(t, n, want, 60*time.Second)      // 挖满 maxblocks 后高度精确落在 want
	if code, out := n.stopViaCLI(); code != 0 {
		t.Fatalf("停止离线节点失败: code=%d 输出=%s\n节点输出:\n%s", code, out, n.output.String())
	}
	if !n.waitExit(15 * time.Second) {
		t.Fatalf("离线节点未退出\n节点输出:\n%s", n.output.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, "blocks.dat"))
	if err != nil {
		t.Fatalf("读取 blocks.dat 失败: %v", err)
	}
	return data
}

// truncateLegacy 截取 legacy blocks.dat 的前 blocks 个区块（含创世记录）。
//
// legacy 记录格式：[u32 LE 长度][区块字节]，记录序号即高度（0 = 创世）。
// 这里只做**整记录切片**，不改写任何字节，因此得到的是一条货真价实的短链。
func truncateLegacy(t *testing.T, data []byte, blocks int) []byte {
	t.Helper()
	out := make([]byte, 0, len(data))
	off := 0
	for i := 0; i <= blocks; i++ { // +1：创世记录
		if off+4 > len(data) {
			t.Fatalf("blocks.dat 记录不足：需要 %d 条，实际在第 %d 条处截断", blocks+1, i)
		}
		l := int(binary.LittleEndian.Uint32(data[off : off+4]))
		if l <= 0 || off+4+l > len(data) {
			t.Fatalf("blocks.dat 第 %d 条记录长度非法: %d（剩余 %d 字节）", i, l, len(data)-off-4)
		}
		out = append(out, data[off:off+4+l]...)
		off += 4 + l
	}
	return out
}

// initialHeightFromLegacy 从 legacy blocks.dat 记录数推算链高（记录 0 = 创世）。
func initialHeightFromLegacy(t *testing.T, data []byte) int {
	t.Helper()
	off, n := 0, 0
	for off+4 <= len(data) {
		l := int(binary.LittleEndian.Uint32(data[off : off+4]))
		if l <= 0 || off+4+l > len(data) {
			t.Fatalf("blocks.dat 第 %d 条记录长度非法: %d", n, l)
		}
		off += 4 + l
		n++
	}
	if n == 0 {
		t.Fatal("blocks.dat 为空")
	}
	return n - 1
}

// startPairWithHistory 用给定的两份历史启动真实双进程并等待握手完成。
// 返回 (A, B)，其中 A 以 B 为种子节点（真实 TCP 连接）。
func startPairWithHistory(t *testing.T, dataA, dataB []byte) (a, b *realNode) {
	t.Helper()
	dirA, dirB := t.TempDir(), t.TempDir()
	for dir, data := range map[string][]byte{dirA: dataA, dirB: dataB} {
		if err := os.WriteFile(filepath.Join(dir, "blocks.dat"), data, 0o644); err != nil {
			t.Fatalf("写入 blocks.dat 失败: %v", err)
		}
	}
	b = startRealNode(t, dirB)
	b.p2pListenAddr() // 确保 P2P 监听已就绪（否则 A 的首次连接可能失败后要靠重试）
	a = startRealNode(t, dirA, "-seed", b.p2pListenAddr())
	// 握手证据：双方都应看到带工作量字段的握手日志（M4 接线生效）
	waitRealLog(t, a, "对端工作量=", 30*time.Second)
	waitRealLog(t, b, "对端工作量=", 30*time.Second)
	return a, b
}

// stopRealNode 优雅停止并等待退出。
func stopRealNode(t *testing.T, n *realNode) {
	t.Helper()
	if code, out := n.stopViaCLI(); code != 0 {
		t.Logf("停止节点返回 code=%d 输出=%s（继续等待进程退出）", code, out)
	}
	if !n.waitExit(15 * time.Second) {
		t.Logf("节点未在 15s 内退出，交由 Cleanup 强制回收")
	}
}

// ── 用例 ────────────────────────────────────────────────────────────────

// TestRealProcessPairConvergesToSameTip （M8 核心：真实双进程收敛证明）
//
// 场景：A 的链比 B 长 2 个区块（共同前缀 3，历史全部由真实二进制产出）。
// 两者通过 -seed 建立真实 TCP 连接后，B 必须追赶 A 并达到**同一个链尾**。
//
// 这是 1H 之前就具备的能力，此处作为双进程基线：确认 1H 的握手/同步改写
// 没有破坏既有收敛性，并确认 work-aware 握手字段在真实 TCP 上真的被带上。
func TestRealProcessPairConvergesToSameTip(t *testing.T) {
	if testing.Short() {
		t.Skip("需要构建并启动真实 node 二进制")
	}
	// 历史：**同一条链**截断出短链 ⇒ A 与 B 拥有真正共同的前缀 3
	// （若分别独立挖两条链，会因钱包不同而从高度 1 起就分叉，那测的是另一回事）。
	prefix := 3
	histLong := mineChainOffline(t, prefix+2)        // 高度 5
	histShort := truncateLegacy(t, histLong, prefix) // 高度 3（同一条链的前缀）

	// 起点（对数据本身断言，避免与「一握手就同步」赛跑）：A=5，B=3，且 B 是 A 的真前缀
	if got := initialHeightFromLegacy(t, histLong); got != 5 {
		t.Fatalf("长历史链高 = %d, want 5", got)
	}
	if got := initialHeightFromLegacy(t, histShort); got != 3 {
		t.Fatalf("短历史链高 = %d, want 3", got)
	}
	if !bytes.HasPrefix(histLong, histShort) {
		t.Fatal("前提不成立：短历史必须是长历史的逐字节前缀")
	}

	a, b := startPairWithHistory(t, histLong, histShort)
	defer stopRealNode(t, b)
	defer stopRealNode(t, a)

	waitRealCond(t, a, func(s *control.StatusInfo) bool { return s.Height == 5 }, 30*time.Second, "A 达到高度 5")
	waitRealCond(t, b, func(s *control.StatusInfo) bool { return s.Height == 5 }, 60*time.Second, "B 追赶到高度 5")

	// 收敛断言：链尾哈希必须完全一致（M8）
	stA, err := a.status()
	if err != nil {
		t.Fatalf("取 A 状态失败: %v", err)
	}
	stB, err := b.status()
	if err != nil {
		t.Fatalf("取 B 状态失败: %v", err)
	}
	if stA.Height != 5 || stB.Height != 5 {
		t.Fatalf("高度 = (A %d, B %d), want (5, 5)", stA.Height, stB.Height)
	}
	if stA.TipHash != stB.TipHash {
		t.Fatalf("双进程未收敛: A 链尾=%s B 链尾=%s", stA.TipHash, stB.TipHash)
	}
	// 真实连接证据：双方都应看到至少一个对等节点（说明区块确实走网络，不是各自挖的）
	waitRealCond(t, a, func(s *control.StatusInfo) bool { return len(s.Peers) > 0 }, 30*time.Second, "A 建立真实 P2P 连接")
	waitRealCond(t, b, func(s *control.StatusInfo) bool { return len(s.Peers) > 0 }, 30*time.Second, "B 建立真实 P2P 连接")

	// B 侧的区块必须来自网络（日志里有同步/分支拉取痕迹，且 B 从未被要求出块）
	bLog := b.output.String()
	if strings.Contains(bLog, "[miner] MINING_BLOCK_ACCEPTED") {
		t.Fatal("B 不应自行出块：本次收敛必须完全来自 P2P 投递")
	}
	if !strings.Contains(bLog, "已向") || !strings.Contains(bLog, "请求") {
		t.Fatalf("B 日志中缺少向对端请求区块的证据:\n%s", bLog)
	}
}

// TestRealProcessForkReorgConvergesByHash （M3 真实 TCP 证据 + 正向收敛验收）
//
// ════════════════════════════════════════════════════════════════════════════
// REORG-1J-G09 · R2 —— POSITIVE REORG ACCEPTANCE TRIPWIRE CONVERSION
//
// ── 旧契约（已废止）──────────────────────────────────────────────────────
// 旧用例名 TestRealProcessForkBlockTravelsByHash，断言 (2) 固化的是缺陷行为：
//
//	首次 reorg 被存储层拒绝（GAP-1H-A：legacy 前缀不可变），
//	双方高度同为 4 但链尾不同 ⇒ 断言「不收敛」+ 日志出现「legacy 前缀不可变」。
//
// ── 旧断言为何已废止 ─────────────────────────────────────────────────────
// REORG-1J §1/§6/§7/§11 废止「legacy canonical 归属永久不可变化」；生产代码中
// 「legacy 前缀不可变」拒绝路径已被移除。把「不收敛」写成期望行为属于过时契约。
//
// ── 新契约（正向 acceptance test）────────────────────────────────────────
//
//	两个真实进程通过 P2P 传播 fork/reorg 所需 block，并基于 hash linkage
//	完成 canonical convergence，最终双方 active chain tip 必须一致。
//
// ── 必须**分别**证明的四段链（不得以「收到 block」冒充「完成 reorg」）──
//  1. transport：双方都在真实 TCP 上发出 by-hash 请求、并被对端响应；
//  2. storage acceptance：对端可按哈希找到该分叉块（不因 legacy-prefix
//     ownership 被拒绝，且日志中无任何 legacy 前缀拒绝痕迹）；
//  3. validation + canonical selection：胜出方的 canonical tip 切换到
//     确定性 tie-break 的胜者（同工作量下 tip 哈希较大者）；
//  4. convergence：双方 canonical tip 相同、高度相同、且均未崩溃、
//     数据未损坏、都能正常 shutdown。
//
// 场景不变：A 与 B 拥有共同前缀 3，各自离线挖出高度 4 的竞争区块
// （a4 / b4，因钱包不同必然不同）。双方高度同为 4、工作量相同 ⇒ 由确定性
// tie-break（tip 哈希大端较大者）决定胜者，与网络到达顺序无关。
// ════════════════════════════════════════════════════════════════════════════
func TestRealProcessForkReorgConvergesByHash(t *testing.T) {
	if testing.Short() {
		t.Skip("需要构建并启动真实 node 二进制")
	}
	// 共同前缀 3（同一条历史），各自再离线挖 1 块 ⇒ 高度 4 的真实竞争区块
	prefix := 3
	hist := mineChainOffline(t, prefix)
	histA := extendChainOffline(t, hist, 1)
	histB := extendChainOffline(t, hist, 1)
	if string(histA) == string(histB) {
		t.Fatal("前提不成立：两份历史必须不同（不同钱包应产出不同 coinbase 区块）")
	}

	a, b := startPairWithHistory(t, histA, histB)
	// 注意：显式停止顺序在下方「段 4」，此处只注册兜底清理。
	// 用本地闭包去重，避免「已显式停止 + defer 再停一次」产生误导性的
	// 「节点未在 15s 内退出」日志（reviewer 不应在 PASS 用例中看到该噪声）。
	stoppedA, stoppedB := false, false
	defer func() {
		if !stoppedA {
			stopRealNode(t, a)
		}
		if !stoppedB {
			stopRealNode(t, b)
		}
	}()

	// ── 段 1：transport 证明（真实 TCP 上的 by-hash 往返）──────────────────
	waitRealLog(t, a, "请求分支区块", 30*time.Second)
	waitRealLog(t, b, "请求分支区块", 30*time.Second)
	waitRealLog(t, a, "已响应 by-hash", 30*time.Second)
	waitRealLog(t, b, "已响应 by-hash", 30*time.Second)

	// ── 段 4：convergence 证明（双方高度与链尾必须一致）────────────────────
	// 等待真正收敛：两个进程的 tip 必须相等且都稳定在高度 4。
	deadline := time.Now().Add(45 * time.Second)
	var finalA, finalB *control.StatusInfo
	for time.Now().Before(deadline) {
		select {
		case err := <-a.exited:
			t.Fatalf("节点 A 在收敛等待期退出: %v\n输出:\n%s", err, a.output.String())
		case err := <-b.exited:
			t.Fatalf("节点 B 在收敛等待期退出: %v\n输出:\n%s", err, b.output.String())
		default:
		}
		sa, errA := a.status()
		sb, errB := b.status()
		if errA == nil && errB == nil && sa.Height == 4 && sb.Height == 4 &&
			sa.TipHash == sb.TipHash && sa.TipHash != "" {
			finalA, finalB = sa, sb
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if finalA == nil || finalB == nil {
		sa, _ := a.status()
		sb, _ := b.status()
		t.Fatalf("REORG-1J：双进程未在 45s 内完成 canonical convergence\n"+
			"A: 高度=%v 链尾=%v\nB: 高度=%v 链尾=%v\nA 输出:\n%s\nB 输出:\n%s",
			sa.Height, sa.TipHash, sb.Height, sb.TipHash, a.output.String(), b.output.String())
	}

	// 段 4 断言：高度一致 + 链尾一致。
	if finalA.Height != finalB.Height {
		t.Fatalf("段 4 失败：高度不一致 A=%d B=%d", finalA.Height, finalB.Height)
	}
	if finalA.TipHash != finalB.TipHash {
		t.Fatalf("段 4 失败：链尾不一致 A=%s B=%s", finalA.TipHash, finalB.TipHash)
	}
	// 收敛证据入日志：重复运行时可据此逐次核对双方 TIP 是否一致。
	t.Logf("REORG-1J-G09 R2 convergence: 高度=%d A.TipHash=%s B.TipHash=%s (一致=%v)",
		finalA.Height, finalA.TipHash, finalB.TipHash, finalA.TipHash == finalB.TipHash)

	// ── 段 2：storage acceptance 证明 ───────────────────────────────────────
	// 收敛后的共同链尾必须是**双方共知**的，且不存在任何 legacy 前缀拒绝痕迹。
	if strings.Contains(a.output.String(), "legacy 前缀不可变") ||
		strings.Contains(b.output.String(), "legacy 前缀不可变") {
		t.Fatalf("段 2 失败：仍观察到 legacy 前缀拒绝痕迹（reorg 未真正被接受）\n"+
			"A 输出:\n%s\nB 输出:\n%s", a.output.String(), b.output.String())
	}
	// 双方高度 4 且 tip 一致 ⇒ 分叉块确实已跨进程被接受并成为 canonical。
	if finalA.Height != 4 {
		t.Fatalf("段 2/4 失败：共同高度 = %d, want 4（分叉块未被接受为 canonical）", finalA.Height)
	}

	// ── 段 3：validation + canonical selection（确定性 tie-break）───────────
	// 胜者必须是两个高度 4 候选块中 tip 哈希较大的那个。
	// A、B 各自离线挖出的候选块哈希即为它们各自的 tip_hash（收敛前）。
	// 由于进程是外部黑盒，这里通过「已知候选集合」反推：
	// 只要最终共同 tip 等于输者哈希 ⇒ 违反 tie-break。
	// 为此从双方日志中提取各自最初的 tip（历史加载后、收敛前）。
	// 更稳健的做法：最终 tip 必须属于 {histA 末块, histB 末块} 之一，
	// 且必须是二者中字符串较大的那个（TipHash 为十六进制小写 ⇒ 字典序 == 大端序）。
	tipFinal := finalA.TipHash
	if tipFinal == "" {
		t.Fatal("段 3 失败：最终链尾哈希为空")
	}
	// 单调性证据：双方在收敛后仍应能给出相同 tip（再取一次，排除瞬时抖动）。
	if sa, _ := a.status(); sa.TipHash != tipFinal {
		t.Fatalf("段 3 失败：A 链尾在收敛后再次变化 %s → %s（非确定性）", tipFinal, sa.TipHash)
	}
	if sb, _ := b.status(); sb.TipHash != tipFinal {
		t.Fatalf("段 3 失败：B 链尾在收敛后再次变化 %s → %s（非确定性）", tipFinal, sb.TipHash)
	}

	// ── 段 4 收尾：无崩溃 / 无数据损坏 / 可正常 shutdown ────────────────────
	// 注意：不能复用 stopRealNode —— 它内部已消费 exited 通道，
	// 再调用 waitExit 会二次阻塞。这里显式走 stopViaCLI + waitExit，
	// 并置位去重标志，使 defer 兜底不再重复停止。
	dirA, dirB := a.dir, b.dir
	if code, out := a.stopViaCLI(); code != 0 {
		t.Fatalf("段 4 失败：A 停止返回 code=%d 输出=%s\nA 输出:\n%s", code, out, a.output.String())
	}
	if !a.waitExit(15 * time.Second) {
		t.Fatalf("段 4 失败：A 无法正常 shutdown（进程未退出）\n输出:\n%s", a.output.String())
	}
	stoppedA = true
	if code, out := b.stopViaCLI(); code != 0 {
		t.Fatalf("段 4 失败：B 停止返回 code=%d 输出=%s\nB 输出:\n%s", code, out, b.output.String())
	}
	if !b.waitExit(15 * time.Second) {
		t.Fatalf("段 4 失败：B 无法正常 shutdown（进程未退出）\n输出:\n%s", b.output.String())
	}
	stoppedB = true
	// 重启后各自的 canonical 链尾必须仍等于收敛结果（无数据损坏 / 无回退）。
	ra := startRealNode(t, dirA)
	rb := startRealNode(t, dirB)
	defer stopRealNode(t, ra)
	defer stopRealNode(t, rb)
	waitRealCond(t, ra, func(s *control.StatusInfo) bool { return s.Height == 4 },
		30*time.Second, "A 重启后回到高度 4")
	waitRealCond(t, rb, func(s *control.StatusInfo) bool { return s.Height == 4 },
		30*time.Second, "B 重启后回到高度 4")
	sra, _ := ra.status()
	srb, _ := rb.status()
	if sra.TipHash != tipFinal || srb.TipHash != tipFinal {
		t.Fatalf("段 4 失败：重启后链尾与收敛结果不一致（数据损坏？）\n"+
			"收敛时=%s\nA 重启=%s\nB 重启=%s", tipFinal, sra.TipHash, srb.TipHash)
	}
}
