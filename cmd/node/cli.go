// cli.go 实现离线/在线命令行子命令。
//
// 命令分为两类：
//   - 在线命令（status / balance / utxos / send）：通过本机控制接口与运行中的节点交互，
//     因为只有节点才知道当前链尾、交易池与网络状态；
//   - 离线命令（wallet / printchain）：直接读取数据目录，不需要节点在运行
//     （printchain 以只读方式打开区块文件，因此不影响运行中的节点）。
//
// 设计约束：所有命令都把输出写到调用方注入的 io.Writer 并返回退出码，
// 自身从不调用 os.Exit —— 这样命令行行为可以在测试中被直接断言
// （见 fullstack_test.go 的 CLI 用例），而不是只能靠「起子进程看输出」。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"p2pchain/internal/blockchain"
	"p2pchain/internal/control"
	"p2pchain/internal/storage"
	"p2pchain/internal/wallet"
)

// usageText 命令行用法。
const usageText = `p2pchain 节点与钱包工具

用法:
  node [选项]                 启动节点（默认行为）
  node <子命令> [选项]

子命令:
  ui            启动节点并打开 Developer Console 页面（本机控制台，同 node 选项）
  status        查询运行中节点的状态（高度、链尾、对等节点、交易池、是否挖矿）
  balance       查询地址余额（默认查询节点钱包自身地址）
  utxos         列出地址的未花费输出（UTXO）
  send          用节点钱包向指定地址转账
  mine          按需立即挖出区块（开发/测试用，对标 bitcoind 的 generatetoaddress）
  wallet        查看或创建本地钱包（离线）
  printchain    打印本地区块链（离线，只读）
  verify        只读校验本地区块链（离线，绝不修改任何数据）
  help          显示本帮助

节点选项（node / ui）:
  -listen  :6688              P2P 监听地址
  -rpc     127.0.0.1:6689     控制接口监听地址（仅本机，无鉴权）
  -seed    host:port           种子节点地址（可留空）
  -datadir <目录>              数据目录（默认 ~/.p2pchain）
  -mine                        启用挖矿
  -maxblocks <整数>            挖矿上限区块数，0 表示不限
  -miners  <整数>              并行挖矿 worker 数（默认等于 CPU 核数，1 为单线程）

在线命令选项:
  -rpc     127.0.0.1:6689     节点控制接口地址
  -address <地址>              （balance/utxos）查询的地址，留空表示节点钱包地址

send 选项:
  -to     <地址>               收款地址（必填）
  -amount <整数>               转账金额（必填，最小单位）
  -fee    <整数>               手续费（默认 0）

mine 选项:
  -count  <整数>               立即挖出的区块数量（默认 1）

离线命令选项:
  -datadir <目录>              数据目录（默认 ~/.p2pchain）
  -limit   <整数>              （printchain）只打印最高 N 个区块，0 表示全部
  -tx                          （printchain）同时打印每个区块的交易
  -json                        （printchain / verify）以 JSON 输出

示例:
  node -mine -datadir ~/.p2pchain
  node ui -mine               # 启动节点并打开 Developer Console
  node status
  node balance
  node send -to 1AbC... -amount 10 -fee 1
  node mine -count 11        # 立即出块，使首笔 coinbase 成熟
  node printchain -limit 5 -tx
  node verify -datadir ./data-a        # 只读校验本地链（退出码 0=通过 / 1=不通过）
  node verify -datadir ./data-a -json  # 机器可读报告
`

// cmdFunc 子命令实现：接收参数与输出目标，返回进程退出码。
type cmdFunc func(args []string, stdout, stderr io.Writer) int

// cliCommands 子命令表（help 单独处理，因为不需要参数解析）。
var cliCommands = map[string]cmdFunc{
	"status":     cmdStatus,
	"balance":    cmdBalance,
	"utxos":      cmdUTXOs,
	"send":       cmdSend,
	"mine":       cmdMine,
	"wallet":     cmdWallet,
	"printchain": cmdPrintChain,
	"verify":     cmdVerify,
}

// runCLI 执行子命令并返回退出码。ok=false 表示不是已知子命令。
func runCLI(cmd string, args []string, stdout, stderr io.Writer) (code int, ok bool) {
	if cmd == "help" || cmd == "-h" || cmd == "--help" {
		fmt.Fprint(stdout, usageText)
		return 0, true
	}
	if cmd == "node" {
		runNode(args)
		return 0, true
	}
	if cmd == "ui" {
		runNodeUI(args)
		return 0, true
	}
	fn, found := cliCommands[cmd]
	if !found {
		return 2, false
	}
	return fn(args, stdout, stderr), true
}

// newFlagSet 创建不会自行退出进程的参数集合（解析错误由调用方处理）。
func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

// fail 打印错误并返回退出码 1。
func fail(stderr io.Writer, format string, a ...any) int {
	fmt.Fprintf(stderr, "错误: "+format+"\n", a...)
	return 1
}

// ---- 在线命令 ----

func cmdStatus(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("status", stderr)
	rpc := fs.String("rpc", control.DefaultAddr, "节点控制接口地址")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	st, err := control.NewClient(*rpc).Status()
	if err != nil {
		return fail(stderr, "%v\n（提示：请先用 `node` 启动节点）", err)
	}
	// 输出格式统一为「标签: 值」，不加手工空格对齐——
	// 中文字符在终端里宽度不一，手工对齐既排不齐也让脚本取值变难。
	fmt.Fprintf(stdout, "高度: %d\n", st.Height)
	fmt.Fprintf(stdout, "链尾哈希: %s\n", st.TipHash)
	fmt.Fprintf(stdout, "节点钱包地址: %s\n", st.Address)
	fmt.Fprintf(stdout, "对等节点: %d 个 %v\n", len(st.Peers), st.Peers)
	fmt.Fprintf(stdout, "交易池: %d 笔待打包\n", st.MempoolSize)
	fmt.Fprintf(stdout, "挖矿状态: %s\n", miningText(st.Mining))
	return 0
}

func miningText(mining bool) string {
	if mining {
		return "运行中"
	}
	return "已停止"
}

func cmdBalance(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("balance", stderr)
	rpc := fs.String("rpc", control.DefaultAddr, "节点控制接口地址")
	address := fs.String("address", "", "查询的地址，留空表示节点钱包地址")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	client := control.NewClient(*rpc)
	addr, err := resolveAddress(client, *address)
	if err != nil {
		return fail(stderr, "%v\n（提示：请先用 `node` 启动节点，或显式指定 -address）", err)
	}

	info, err := client.Balance(addr)
	if err != nil {
		return fail(stderr, "%v", err)
	}
	fmt.Fprintf(stdout, "地址: %s\n", info.Address)
	fmt.Fprintf(stdout, "可花费余额: %d\n", info.Spendable)
	fmt.Fprintf(stdout, "总余额(含未成熟): %d\n", info.Total)
	fmt.Fprintf(stdout, "UTXO 数量: %d\n", info.UTXOCount)
	fmt.Fprintf(stdout, "链高度: %d\n", info.Height)
	return 0
}

func cmdUTXOs(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("utxos", stderr)
	rpc := fs.String("rpc", control.DefaultAddr, "节点控制接口地址")
	address := fs.String("address", "", "查询的地址，留空表示节点钱包地址")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	client := control.NewClient(*rpc)
	addr, err := resolveAddress(client, *address)
	if err != nil {
		return fail(stderr, "%v\n（提示：请先用 `node` 启动节点，或显式指定 -address）", err)
	}

	list, err := client.UTXOs(addr)
	if err != nil {
		return fail(stderr, "%v", err)
	}
	if len(list) == 0 {
		fmt.Fprintf(stdout, "地址 %s 没有未花费输出\n", addr)
		return 0
	}
	fmt.Fprintf(stdout, "%-70s %14s %8s %10s %s\n", "OUTPOINT", "金额", "高度", "coinbase", "成熟")
	var total uint64
	for _, u := range list {
		fmt.Fprintf(stdout, "%-70s %14d %8d %10v %v\n", u.OutPoint, u.Value, u.Height, u.IsCoinbase, u.Mature)
		total += u.Value
	}
	fmt.Fprintf(stdout, "合计: %d（共 %d 个输出）\n", total, len(list))
	return 0
}

func cmdSend(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("send", stderr)
	rpc := fs.String("rpc", control.DefaultAddr, "节点控制接口地址")
	to := fs.String("to", "", "收款地址（必填）")
	amount := fs.Uint64("amount", 0, "转账金额（必填，最小单位）")
	fee := fs.Uint64("fee", 0, "手续费（最小单位，默认 0）")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *to == "" {
		return fail(stderr, "必须指定 -to 收款地址")
	}
	if *amount == 0 {
		return fail(stderr, "必须指定大于 0 的 -amount 转账金额")
	}
	if err := wallet.ValidateAddress(*to); err != nil {
		return fail(stderr, "收款地址非法: %v", err)
	}

	resp, err := control.NewClient(*rpc).Send(control.SendRequest{To: *to, Amount: *amount, Fee: *fee})
	if err != nil {
		return fail(stderr, "%v", err)
	}
	fmt.Fprintf(stdout, "交易已提交并广播\n")
	fmt.Fprintf(stdout, "交易 ID: %s\n", resp.TxID)
	fmt.Fprintf(stdout, "收款地址: %s\n", resp.To)
	fmt.Fprintf(stdout, "金额: %d\n", resp.Amount)
	fmt.Fprintf(stdout, "手续费: %d\n", resp.Fee)
	fmt.Fprintf(stdout, "输入数: %d\n", resp.InputNum)
	return 0
}

func cmdMine(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("mine", stderr)
	rpc := fs.String("rpc", control.DefaultAddr, "节点控制接口地址")
	count := fs.Int("count", 1, "立即挖出的区块数量")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *count <= 0 {
		return fail(stderr, "-count 必须大于 0，实际 %d", *count)
	}

	resp, err := control.NewClient(*rpc).Mine(*count)
	if err != nil {
		return fail(stderr, "%v\n（提示：请先用 `node` 启动节点，且不要在 -mine 持续挖矿模式下调用）", err)
	}
	fmt.Fprintf(stdout, "已挖出 %d 个区块，当前高度 %d\n", resp.Mined, resp.Height)
	return 0
}

// resolveAddress 解析查询目标地址：显式给出则直接用，否则取节点钱包地址。
func resolveAddress(client *control.Client, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	st, err := client.Status()
	if err != nil {
		return "", err
	}
	if st.Address == "" {
		return "", errors.New("节点未报告钱包地址，请用 -address 显式指定")
	}
	return st.Address, nil
}

// ---- 离线命令 ----

func cmdWallet(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("wallet", stderr)
	dataDir := fs.String("datadir", defaultDataDir(), "数据目录")
	addressOnly := fs.Bool("address", false, "只输出地址（便于脚本使用）")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	path := filepath.Join(*dataDir, "wallet.json")
	w, created, err := wallet.LoadOrCreate(path)
	if err != nil {
		return fail(stderr, "加载/创建钱包失败: %v", err)
	}
	if *addressOnly {
		fmt.Fprintln(stdout, w.Address())
		return 0
	}
	if created {
		fmt.Fprintf(stdout, "已创建新钱包: %s\n", path)
	} else {
		fmt.Fprintf(stdout, "已加载钱包: %s\n", path)
	}
	fmt.Fprintf(stdout, "地址: %s\n", w.Address())
	return 0
}

// printChainJSON printchain -json 的输出结构。
type printChainJSON struct {
	Height int    `json:"height"`
	Hash   string `json:"hash"`
	Prev   string `json:"prev_hash"`
	Merkle string `json:"merkle_root"`
	Time   int64  `json:"timestamp"`
	Bits   uint32 `json:"bits"`
	Nonce  uint64 `json:"nonce"`
	Txs    int    `json:"tx_count"`
}

func cmdPrintChain(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("printchain", stderr)
	dataDir := fs.String("datadir", defaultDataDir(), "数据目录")
	limit := fs.Int("limit", 0, "只打印最高 N 个区块，0 表示全部")
	withTx := fs.Bool("tx", false, "同时打印每个区块的交易")
	asJSON := fs.Bool("json", false, "以 JSON 输出")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// 只读打开：运行中的节点仍可继续追加区块，双方互不干扰
	store, err := storage.OpenFileBlockStoreReadOnly(*dataDir)
	if err != nil {
		return fail(stderr, "打开区块数据失败: %v", err)
	}
	defer func() { _ = store.Close() }()

	height, err := store.Height()
	if err != nil {
		return fail(stderr, "读取高度失败: %v", err)
	}
	if height < 0 {
		fmt.Fprintf(stdout, "本地链为空（数据目录: %s）\n", *dataDir)
		return 0
	}

	from := 0
	if *limit > 0 && height-*limit+1 > 0 {
		from = height - *limit + 1
	}

	if !*asJSON {
		fmt.Fprintf(stdout, "本地区块链: 高度 %d（数据目录 %s）\n", height, *dataDir)
	}
	for h := from; h <= height; h++ {
		b, err := store.GetBlockByHeight(h)
		if err != nil {
			return fail(stderr, "读取高度 %d 区块失败: %v", h, err)
		}
		if *asJSON {
			data, err := json.Marshal(printChainJSON{
				Height: h,
				Hash:   b.Header.HashHex(),
				Prev:   fmt.Sprintf("%x", b.Header.PrevBlockHash),
				Merkle: fmt.Sprintf("%x", b.Header.MerkleRoot),
				Time:   b.Header.Timestamp,
				Bits:   b.Header.Bits,
				Nonce:  b.Header.Nonce,
				Txs:    len(b.Transactions),
			})
			if err != nil {
				return fail(stderr, "序列化区块 %d 失败: %v", h, err)
			}
			fmt.Fprintln(stdout, string(data))
			continue
		}

		fmt.Fprintf(stdout, "\n区块 #%d\n", h)
		fmt.Fprintf(stdout, "  哈希      : %s\n", b.Header.HashHex())
		fmt.Fprintf(stdout, "  前块哈希  : %x\n", b.Header.PrevBlockHash)
		fmt.Fprintf(stdout, "  Merkle 根 : %x\n", b.Header.MerkleRoot)
		fmt.Fprintf(stdout, "  时间戳    : %d\n", b.Header.Timestamp)
		fmt.Fprintf(stdout, "  难度位    : %d\n", b.Header.Bits)
		fmt.Fprintf(stdout, "  Nonce     : %d\n", b.Header.Nonce)
		fmt.Fprintf(stdout, "  交易数    : %d\n", len(b.Transactions))
		if *withTx {
			for i, tx := range b.Transactions {
				kind := "普通"
				if tx.IsCoinbase() {
					kind = "coinbase"
				}
				fmt.Fprintf(stdout, "    [%d] %s (%s) 输入=%d 输出=%d\n",
					i, tx.HashHex(), kind, len(tx.Inputs), len(tx.Outputs))
				for j, out := range tx.Outputs {
					fmt.Fprintf(stdout, "        out[%d] 金额=%d 收款公钥哈希=%x\n", j, out.Value, out.PubKeyHash)
				}
			}
		}
	}
	return 0
}

// ---- 离线只读校验 ----

// cmdVerify 对本地链执行一次只读的确定性回放校验。
//
// 产品契约（PHASE BRAND-1.2 §3）：
//   - 以**只读**方式打开 blocks.dat（与 printchain 同一机制），绝不写回、绝不取锁；
//   - 逐块重新执行与节点启动时完全相同的共识校验（复用 blockchain.VerifyStoredChain）；
//   - 结论只有 PASS / FAIL 两种，FAIL 时必须给出失败高度、区块哈希与具体原因；
//   - 退出码：0 = 校验通过；1 = 校验未通过或无法执行；2 = 命令行参数错误。
func cmdVerify(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("verify", stderr)
	dataDir := fs.String("datadir", defaultDataDir(), "数据目录")
	asJSON := fs.Bool("json", false, "以 JSON 输出（机器可读）")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	store, err := storage.OpenFileBlockStoreReadOnly(*dataDir)
	if err != nil {
		return fail(stderr, "打开区块数据失败: %v", err)
	}
	defer func() { _ = store.Close() }()

	rep, err := blockchain.VerifyStoredChain(store)
	if err != nil {
		return fail(stderr, "执行校验失败: %v", err)
	}
	rep.DataDir = *dataDir

	if *asJSON {
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return fail(stderr, "序列化校验报告失败: %v", err)
		}
		fmt.Fprintln(stdout, string(data))
		if !rep.Valid {
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "[verify] 数据目录  : %s\n", rep.DataDir)
	if rep.Valid {
		fmt.Fprintf(stdout, "[verify] 已校验区块: %d 个（高度 0 → %d）\n", rep.Blocks, rep.Height)
		fmt.Fprintf(stdout, "[verify] 创世哈希  : %s\n", rep.GenesisHash)
		fmt.Fprintf(stdout, "[verify] 链尾哈希  : %s\n", rep.TipHash)
		fmt.Fprintf(stdout, "[verify] 结果      : PASS（只读回放校验通过，未修改任何数据）\n")
		return 0
	}
	fmt.Fprintf(stdout, "[verify] 结果      : FAIL\n")
	if rep.FailHeight >= 0 {
		fmt.Fprintf(stdout, "[verify] 失败高度  : %d\n", rep.FailHeight)
	}
	if rep.FailHash != "" {
		fmt.Fprintf(stdout, "[verify] 失败区块  : %s\n", rep.FailHash)
	}
	fmt.Fprintf(stdout, "[verify] 原因      : %s\n", rep.Reason)
	return 1
}
