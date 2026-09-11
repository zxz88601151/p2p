package main

// 本文件是全栈端到端验收测试：用「进程内真实启动的节点」（同一套 newNodeRuntime 组装路径）
// 走完一条完整业务链：
//
//	启动节点 → 控制接口查询状态 → 挖矿 → coinbase 成熟 → 余额查询
//	→ 构造并签名转账交易 → 入池并广播 → 挖出区块打包该交易
//	→ 收款方余额增加、付款方余额减少 → 重启节点后状态与余额仍一致
//
// 这里不 mock 任何环节：真实验签、真实 PoW、真实规范编解码、真实文件持久化、
// 真实 HTTP 控制接口。因此这条用例通过即代表「功能是通的」，而不是「接口存在」。

import (
	"bytes"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/control"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

// startTestRuntime 在临时目录启动一个完整节点（P2P + 控制接口均为随机端口）。
func startTestRuntime(t *testing.T, mine bool) (*nodeRuntime, *control.Client, string) {
	t.Helper()
	dir := t.TempDir()
	rt, err := newNodeRuntime(nodeConfig{
		ListenAddr: "127.0.0.1:0",
		RPCAddr:    "127.0.0.1:0",
		DataDir:    dir,
		Mine:       mine,
	})
	if err != nil {
		t.Fatalf("启动节点失败: %v", err)
	}
	t.Cleanup(rt.Close)
	return rt, control.NewClient(rt.ctl.Addr()), dir
}

// mineBlocks 连续挖出 n 个区块（走真实挖矿路径，含交易池打包）。
func mineBlocks(t *testing.T, rt *nodeRuntime, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if !mineOnce(rt.svc) {
			t.Fatalf("第 %d 次挖矿被意外中断", i+1)
		}
	}
}

// ---- 用例 ----

// TestFullStackMineAndStatus 启动真实节点：控制接口可用、挖矿后高度与链尾同步推进。
func TestFullStackMineAndStatus(t *testing.T) {
	rt, client, _ := startTestRuntime(t, true)

	st, err := client.Status()
	if err != nil {
		t.Fatalf("查询状态失败: %v", err)
	}
	if st.Height != 0 {
		t.Fatalf("新节点初始高度 = %d, want 0（确定性创世）", st.Height)
	}
	if st.Address == "" {
		t.Fatal("节点未报告钱包地址")
	}
	if err := wallet.ValidateAddress(st.Address); err != nil {
		t.Fatalf("节点报告的钱包地址非法: %v", err)
	}
	if st.Mining {
		t.Fatal("未启动挖矿循环时不应报告正在挖矿")
	}

	mineBlocks(t, rt, 3)

	st2, err := client.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st2.Height != 3 {
		t.Fatalf("挖矿后高度 = %d, want 3", st2.Height)
	}
	tip, err := rt.chain.BlockByHeight(3)
	if err != nil {
		t.Fatal(err)
	}
	if st2.TipHash != tip.Header.HashHex() {
		t.Fatalf("状态接口链尾哈希 %s 与本地链 %s 不一致", st2.TipHash, tip.Header.HashHex())
	}
}

// TestFullStackCoinbaseMaturityAndBalance 余额随 coinbase 成熟而变化（含未成熟区分）。
//
// 注意创世区块的设计：其 coinbase 支付给固定的「黑洞」公钥哈希（无对应私钥），
// 目的是让所有节点得到字节级相同的创世区块。因此节点钱包在创世后余额为 0，
// 必须靠自己挖出的区块获得可花费余额。
func TestFullStackCoinbaseMaturityAndBalance(t *testing.T) {
	rt, client, _ := startTestRuntime(t, true)
	st, err := client.Status()
	if err != nil {
		t.Fatal(err)
	}

	// 1. 创世后：节点钱包没有余额（创世奖励属于黑洞地址）
	bal, err := client.Balance(st.Address)
	if err != nil {
		t.Fatalf("查询余额失败: %v", err)
	}
	if bal.UTXOCount != 0 || bal.Total != 0 || bal.Spendable != 0 {
		t.Fatalf("创世后节点钱包应为空，实际 total=%d spendable=%d utxo=%d",
			bal.Total, bal.Spendable, bal.UTXOCount)
	}

	// 2. 挖出第一个区块：钱包得到一笔未成熟 coinbase
	mineBlocks(t, rt, 1)
	bal1, err := client.Balance(st.Address)
	if err != nil {
		t.Fatal(err)
	}
	if bal1.UTXOCount != 1 {
		t.Fatalf("挖出 1 个区块后 UTXO 数量 = %d, want 1", bal1.UTXOCount)
	}
	if want := utxo.Subsidy(1); bal1.Total != want {
		t.Fatalf("总余额 = %d, want %d", bal1.Total, want)
	}
	if bal1.Spendable != 0 {
		t.Fatalf("高度 1 的 coinbase 尚未成熟，可花费余额应为 0，实际 %d", bal1.Spendable)
	}

	// 3. 再挖 CoinbaseMaturity 个区块，高度 1 的 coinbase 成熟
	mineBlocks(t, rt, utxo.CoinbaseMaturity)

	bal2, err := client.Balance(st.Address)
	if err != nil {
		t.Fatal(err)
	}
	if bal2.Spendable == 0 {
		t.Fatalf("已达成熟高度，可花费余额仍为 0（UTXO 数 %d）", bal2.UTXOCount)
	}
	if bal2.Spendable > bal2.Total {
		t.Fatalf("可花费余额 %d 不应大于总余额 %d", bal2.Spendable, bal2.Total)
	}

	// 4. UTXO 明细：成熟输出金额之和应等于可花费余额，且存在尚未成熟的输出
	list, err := client.UTXOs(st.Address)
	if err != nil {
		t.Fatal(err)
	}
	var matureSum uint64
	var sawImmature bool
	for _, u := range list {
		if u.Mature {
			matureSum += u.Value
		} else {
			sawImmature = true
		}
	}
	if matureSum != bal2.Spendable {
		t.Fatalf("成熟 UTXO 金额之和 = %d, 与可花费余额 %d 不一致", matureSum, bal2.Spendable)
	}
	if !sawImmature {
		t.Fatal("应存在尚未成熟的 coinbase 输出")
	}

	// 5. 成熟度边界必须精确：恰好成熟的那个高度可花费，晚一个高度的不可
	currentHeight := rt.chain.Height()
	maturedHeight := currentHeight + 1 - utxo.CoinbaseMaturity
	for _, u := range list {
		if !u.IsCoinbase {
			continue
		}
		want := u.Height <= maturedHeight
		if u.Mature != want {
			t.Fatalf("高度 %d 的 coinbase 成熟状态 = %v, want %v（当前高度 %d，成熟度 %d）",
				u.Height, u.Mature, want, currentHeight, utxo.CoinbaseMaturity)
		}
	}
}

// TestFullStackTransfer 完整转账闭环：构造 → 签名 → 入池 → 打包上链 → 双方余额变化。
func TestFullStackTransfer(t *testing.T) {
	rt, client, _ := startTestRuntime(t, true)

	// 1. 挖到第一个 coinbase 成熟
	mineBlocks(t, rt, utxo.CoinbaseMaturity)

	status, err := client.Status()
	if err != nil {
		t.Fatal(err)
	}
	sender := status.Address
	beforeSend, err := client.Balance(sender)
	if err != nil {
		t.Fatal(err)
	}
	if beforeSend.Spendable == 0 {
		t.Fatal("付款方无可花费余额，无法进行转账测试")
	}
	beforeList, err := client.UTXOs(sender)
	if err != nil {
		t.Fatal(err)
	}
	beforeSet := indexUTXOs(beforeList)

	// 2. 生成收款方地址（独立钱包，只取地址）
	recipientWallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatal(err)
	}
	recipient := recipientWallet.Address()

	const amount, fee = uint64(7), uint64(1)
	if beforeSend.Spendable < amount+fee {
		t.Fatalf("可花费余额 %d 不足以支付 %d + 手续费 %d", beforeSend.Spendable, amount, fee)
	}

	// 3. 提交转账：节点用自己钱包签名、校验并入池
	resp, err := client.Send(control.SendRequest{To: recipient, Amount: amount, Fee: fee})
	if err != nil {
		t.Fatalf("提交转账失败: %v", err)
	}
	if len(resp.TxID) != 64 {
		t.Fatalf("交易 ID 长度异常: %q", resp.TxID)
	}
	if resp.InputNum == 0 {
		t.Fatal("交易没有输入")
	}

	// 交易已在池中等待打包
	st, _ := client.Status()
	if st.MempoolSize != 1 {
		t.Fatalf("交易池大小 = %d, want 1", st.MempoolSize)
	}

	// 4. 挖一个区块把交易打包上链
	mineBlocks(t, rt, 1)
	packedHeight := rt.chain.Height()

	st2, _ := client.Status()
	if st2.MempoolSize != 0 {
		t.Fatalf("交易已被打包，交易池应为空，实际 %d 笔", st2.MempoolSize)
	}

	// 5. 收款方精确收到 amount
	afterRecv, err := client.Balance(recipient)
	if err != nil {
		t.Fatal(err)
	}
	if afterRecv.Spendable != amount {
		t.Fatalf("收款方可花费余额 = %d, want %d", afterRecv.Spendable, amount)
	}

	// 6. UTXO 级守恒核对（比单纯比较余额更严，因为挖矿会同时改变成熟度）：
	//    消失的输出应恰好是被花费的那个输入；新增的输出应恰好是
	//    本区块的 coinbase 与找零；找零 = 输入额 - amount - fee。
	afterList, err := client.UTXOs(sender)
	if err != nil {
		t.Fatal(err)
	}
	afterSet := indexUTXOs(afterList)

	var consumed []control.UTXOInfo
	for op, u := range beforeSet {
		if _, still := afterSet[op]; !still {
			consumed = append(consumed, u)
		}
	}
	if len(consumed) != 1 {
		t.Fatalf("消失的 UTXO 数量 = %d, want 1（应恰好花费一个输入）", len(consumed))
	}
	spentValue := consumed[0].Value

	var coinbase, change *control.UTXOInfo
	added := 0
	for op, u := range afterSet {
		if _, existed := beforeSet[op]; existed {
			continue
		}
		added++
		u := u
		if u.IsCoinbase {
			coinbase = &u
		} else {
			change = &u
		}
	}
	if added != 2 {
		t.Fatalf("新增的 UTXO 数量 = %d, want 2（本区块 coinbase + 找零）", added)
	}
	if coinbase == nil || change == nil {
		t.Fatalf("新增 UTXO 构成不符：coinbase=%v change=%v", coinbase != nil, change != nil)
	}
	if want := spentValue - amount - fee; change.Value != want {
		t.Fatalf("找零 = %d, want %d（输入 %d - 金额 %d - 手续费 %d）",
			change.Value, want, spentValue, amount, fee)
	}
	if want := utxo.Subsidy(packedHeight) + fee; coinbase.Value != want {
		t.Fatalf("区块 coinbase = %d, want %d（补贴 %d + 手续费 %d）",
			coinbase.Value, want, utxo.Subsidy(packedHeight), fee)
	}

	// 7. 总余额守恒：支出 amount+fee，收入本区块 coinbase（含手续费）
	afterSend, err := client.Balance(sender)
	if err != nil {
		t.Fatal(err)
	}
	wantTotal := beforeSend.Total - amount - fee + coinbase.Value
	if afterSend.Total != wantTotal {
		t.Fatalf("付款方总余额 = %d, want %d", afterSend.Total, wantTotal)
	}

	// 8. 区块里确实包含这笔交易（按规范编码从控制接口取回，重新解码核对 TxID）
	encoded, err := client.BlockHex(packedHeight)
	if err != nil {
		t.Fatalf("取回区块失败: %v", err)
	}
	blk := decodeBlockHex(t, encoded)
	found := false
	for _, tx := range blk.Transactions {
		if tx.HashHex() == resp.TxID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("高度 %d 的区块中未找到交易 %s", packedHeight, resp.TxID)
	}
}

// indexUTXOs 把 UTXO 列表按 OutPoint 建索引，便于做集合差。
func indexUTXOs(list []control.UTXOInfo) map[string]control.UTXOInfo {
	m := make(map[string]control.UTXOInfo, len(list))
	for _, u := range list {
		m[u.OutPoint] = u
	}
	return m
}

// TestFullStackRestartPersists 重启后高度、链尾与余额保持一致（真实文件持久化）。
func TestFullStackRestartPersists(t *testing.T) {
	rt, client, dir := startTestRuntime(t, true)
	mineBlocks(t, rt, 4)

	before, err := client.Status()
	if err != nil {
		t.Fatal(err)
	}
	balBefore, err := client.Balance(before.Address)
	if err != nil {
		t.Fatal(err)
	}
	rt.Close() // 关闭节点，释放文件与端口

	// 用同一数据目录重新启动
	rt2, err := newNodeRuntime(nodeConfig{
		ListenAddr: "127.0.0.1:0",
		RPCAddr:    "127.0.0.1:0",
		DataDir:    dir,
	})
	if err != nil {
		t.Fatalf("重启节点失败: %v", err)
	}
	t.Cleanup(rt2.Close)
	client2 := control.NewClient(rt2.ctl.Addr())

	after, err := client2.Status()
	if err != nil {
		t.Fatal(err)
	}
	if after.Height != before.Height {
		t.Fatalf("重启后高度 = %d, want %d", after.Height, before.Height)
	}
	if after.TipHash != before.TipHash {
		t.Fatalf("重启后链尾 = %s, want %s", after.TipHash, before.TipHash)
	}
	if after.Address != before.Address {
		t.Fatalf("重启后钱包地址变化: %s → %s", before.Address, after.Address)
	}

	balAfter, err := client2.Balance(after.Address)
	if err != nil {
		t.Fatal(err)
	}
	if balAfter.Total != balBefore.Total || balAfter.Spendable != balBefore.Spendable {
		t.Fatalf("重启后余额变化: total %d→%d, spendable %d→%d",
			balBefore.Total, balAfter.Total, balBefore.Spendable, balAfter.Spendable)
	}
}

// TestFullStackRejectsBadRequests 控制接口对非法输入必须明确报错，不能静默成功。
func TestFullStackRejectsBadRequests(t *testing.T) {
	rt, client, _ := startTestRuntime(t, true)

	if _, err := client.Balance("not-a-real-address"); err == nil {
		t.Fatal("非法地址查询余额应报错")
	}
	if _, err := client.UTXOs("not-a-real-address"); err == nil {
		t.Fatal("非法地址查询 UTXO 应报错")
	}
	// 收款地址非法
	if _, err := client.Send(control.SendRequest{To: "bad", Amount: 1}); err == nil {
		t.Fatal("非法收款地址应报错")
	}
	// 余额不足（新节点 coinbase 未成熟）
	st, _ := client.Status()
	if _, err := client.Send(control.SendRequest{To: st.Address, Amount: 1 << 40}); err == nil {
		t.Fatal("余额不足应报错")
	}
	// 超出实际高度的区块查询
	if _, err := client.BlockHex(9999); err == nil {
		t.Fatal("查询不存在的区块高度应报错")
	}
	// 原始 HTTP：GET /send 应 405
	resp, err := http.Get("http://" + rt.ctl.Addr() + "/send")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /send 状态码 = %d, want 405", resp.StatusCode)
	}
}

// TestFullStackCLICommands CLI 子命令的输出契约（进程内调用，断言真实输出）。
// 覆盖离线命令（wallet/printchain）与在线命令（status/balance/utxos/send），
// 使用真实数据目录与真实控制接口。
func TestFullStackCLICommands(t *testing.T) {
	rt, _, dir := startTestRuntime(t, true)
	mineBlocks(t, rt, 3)
	rpcAddr := rt.ctl.Addr()

	// wallet -address：应输出一个合法地址
	out, code := execCLI(t, "wallet", "-datadir", dir, "-address")
	if code != 0 {
		t.Fatalf("wallet 退出码 = %d, 输出: %s", code, out)
	}
	addr := strings.TrimSpace(out)
	if err := wallet.ValidateAddress(addr); err != nil {
		t.Fatalf("wallet -address 输出非法地址 %q: %v", addr, err)
	}

	// status：应包含高度 3
	out, code = execCLI(t, "status", "-rpc", rpcAddr)
	if code != 0 {
		t.Fatalf("status 退出码 = %d, 输出: %s", code, out)
	}
	if !strings.Contains(out, "高度") || !strings.Contains(out, "3") {
		t.Fatalf("status 输出未包含高度 3:\n%s", out)
	}

	// balance：应报告节点地址
	out, code = execCLI(t, "balance", "-rpc", rpcAddr)
	if code != 0 {
		t.Fatalf("balance 退出码 = %d, 输出: %s", code, out)
	}
	if !strings.Contains(out, addr) {
		t.Fatalf("balance 输出未包含节点地址 %s:\n%s", addr, out)
	}

	// utxos -address：应列出输出（含表头）
	out, code = execCLI(t, "utxos", "-rpc", rpcAddr, "-address", addr)
	if code != 0 {
		t.Fatalf("utxos 退出码 = %d, 输出: %s", code, out)
	}
	if !strings.Contains(out, "OUTPOINT") {
		t.Fatalf("utxos 输出缺少表头:\n%s", out)
	}

	// printchain：离线读取，limit=2 只应包含高度 3 与 2
	out, code = execCLI(t, "printchain", "-datadir", dir, "-limit", "2", "-json")
	if code != 0 {
		t.Fatalf("printchain 退出码 = %d, 输出: %s", code, out)
	}
	if !strings.Contains(out, `"height":3`) || !strings.Contains(out, `"height":2`) {
		t.Fatalf("printchain -json 输出不符合预期:\n%s", out)
	}
	if strings.Contains(out, `"height":1`) {
		t.Fatalf("printchain -limit 2 不应包含高度 1:\n%s", out)
	}

	// send：参数缺失应报错退出（不发起请求）
	out, code = execCLI(t, "send", "-rpc", rpcAddr)
	if code == 0 {
		t.Fatal("send 缺少 -to/-amount 时应返回非零退出码")
	}
	if !strings.Contains(out, "收款地址") {
		t.Fatalf("send 错误提示不符合预期:\n%s", out)
	}

	// sizeable：非法收款地址应被拒绝
	out, code = execCLI(t, "send", "-rpc", rpcAddr, "-to", "bad-address", "-amount", "1")
	if code == 0 {
		t.Fatal("send 非法收款地址应返回非零退出码")
	}
	if !strings.Contains(out, "非法") {
		t.Fatalf("send 非法地址错误提示不符合预期:\n%s", out)
	}

	// 未知子命令应返回退出码 2 且 ok=false
	if code, ok := runCLI("definitely-not-a-command", nil, io.Discard, io.Discard); ok || code == 0 {
		t.Fatalf("未知子命令应返回 ok=false 且非零退出码，实际 code=%d ok=%v", code, ok)
	}

	// 节点未运行（错误端口）时在线命令应给出可读错误
	out, code = execCLI(t, "status", "-rpc", "127.0.0.1:1")
	if code == 0 {
		t.Fatal("无法连接节点时应返回非零退出码")
	}
	if !strings.Contains(out, "提示") {
		t.Fatalf("连接失败的错误提示缺少引导信息:\n%s", out)
	}
}

// execCLI 进程内执行子命令并合并 stdout/stderr，返回输出与退出码。
func execCLI(t *testing.T, cmd string, args ...string) (string, int) {
	t.Helper()
	var buf bytes.Buffer
	code, ok := runCLI(cmd, args, &buf, &buf)
	if !ok {
		t.Fatalf("子命令 %q 未被识别", cmd)
	}
	return buf.String(), code
}

// decodeBlockHex 把控制接口返回的区块编码十六进制解码为区块（复用真实编解码）。
func decodeBlockHex(t *testing.T, encoded string) *block.Block {
	t.Helper()
	raw, err := hex.DecodeString(encoded)
	if err != nil {
		t.Fatalf("区块编码非十六进制: %v", err)
	}
	b, err := block.DecodeBlock(raw)
	if err != nil {
		t.Fatalf("区块解码失败: %v", err)
	}
	return b
}
