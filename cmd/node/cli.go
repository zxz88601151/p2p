// cli.go 实现离线/在线命令行子命令。
//
// 命令分为两类：
//   - 在线命令（status / balance / utxos / send）：通过本机控制接口与运行中的节点交互，
//     因为只有节点才知道当前链尾、交易池与网络状态；
//   - 离线命令（wallet / printchain / verify / reset）：直接读/写数据目录，不需要节点在运行
//     （printchain 与 verify 以只读方式打开区块文件，因此不影响运行中的节点；
//     reset 相反，它要求数据目录**没有**节点在使用，见 cmdReset）。
//
// 设计约束：所有命令都把输出写到调用方注入的 io.Writer 并返回退出码，
// 自身从不调用 os.Exit —— 这样命令行行为可以在测试中被直接断言
// （见 fullstack_test.go 的 CLI 用例），而不是只能靠「起子进程看输出」。
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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
  stop          请求运行中的节点优雅停止（释放数据目录锁后退出）
  init          显式初始化新数据目录并创建 canonical Genesis
  wallet        查看/创建本地加密钱包（离线；wallet encrypt 做 v1 明文迁移）
  printchain    打印本地区块链（离线，只读）
  verify        只读校验本地区块链（离线，绝不修改任何数据）
  reset         清空本地实验状态（离线，破坏性：删除链/钱包/锁）
  help          显示本帮助

节点选项（node / ui）:
  -listen  :6688              P2P 监听地址
  -rpc     127.0.0.1:6689     控制接口监听地址（默认仅回环；读端点无鉴权，mutation 端点需 Bearer Token）
  -allow-non-loopback          允许控制接口监听非回环地址（危险：读端点无鉴权，确认网络隔离再开）
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

离线命令选项:
  -datadir <目录>              数据目录（默认 ~/.p2pchain）
  -limit   <整数>              （printchain）只打印最高 N 个区块，0 表示全部
  -tx                          （printchain）同时打印每个区块的交易
  -json                        （printchain / verify / reset）以 JSON 输出
  -force                       （reset）跳过确认提示（脚本 / CI 使用）

选项位置（PHASE PRODUCT-DEV-1C.0）:
  子命令与选项的先后顺序**不改变命令语义**，下面两行完全等价：
    node verify -datadir ./data-a
    node -datadir ./data-a verify
  识别不了的子命令一律报错退出（退出码 2），绝不会退化成「启动节点」。

示例:
  node -mine -datadir ~/.p2pchain
  node ui -mine               # 启动节点并打开 Developer Console
  node status
  node balance
  node send -to 1AbC... -amount 10 -fee 1
  node -mine -maxblocks 101 -datadir ./data-a   # 持续挖矿至 101 块，使首笔 coinbase 成熟
  node stop                  # 优雅停止节点（等价于终端里按 Ctrl+C）
  node printchain -limit 5 -tx
  node verify -datadir ./data-a        # 只读校验本地链（退出码 0=通过 / 1=不通过）
  node verify -datadir ./data-a -json  # 机器可读报告
  node reset -datadir ./data-a         # 清空实验状态，回到首次运行状态（需确认）
  node reset -datadir ./data-a -force  # 同上但跳过确认（脚本用）
`

// cmdFunc 子命令实现：接收参数与输出目标，返回进程退出码。
type cmdFunc func(args []string, stdout, stderr io.Writer) int

// cliCommands 子命令表（help 单独处理，因为不需要参数解析）。
var cliCommands = map[string]cmdFunc{
	"status":     cmdStatus,
	"balance":    cmdBalance,
	"utxos":      cmdUTXOs,
	"send":       cmdSend,
	"stop":       cmdStop,
	"init":       cmdInit,
	"wallet":     cmdWallet,
	"printchain": cmdPrintChain,
	"verify":     cmdVerify,
	"reset":      cmdReset,
}

// cliStdin 是 reset 确认提示的输入源。生产恒为 os.Stdin，
// 测试可临时替换以断言「输入 yes / 输入其它 / 非终端」三条分支。
var cliStdin io.Reader = os.Stdin

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

// cmdInit 显式初始化一个全新的数据目录。
//
// 该命令只创建 canonical Genesis，不加载钱包、不启动网络、不挖矿。
// 普通 node 启动路径不会调用它，也不会在空库上隐式初始化。
func cmdInit(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("init", stderr)
	dataDir := fs.String("datadir", defaultDataDir(), "数据目录")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	blocksPath := filepath.Join(*dataDir, "blocks.dat")
	dirInfo, dirErr := os.Stat(*dataDir)
	freshDir := false
	if dirErr != nil {
		if !errors.Is(dirErr, os.ErrNotExist) {
			return fail(stderr, "检查数据目录失败: %v", dirErr)
		}
		freshDir = true
	} else if !dirInfo.IsDir() {
		return fail(stderr, "数据路径不是目录: %s", *dataDir)
	} else {
		entries, err := os.ReadDir(*dataDir)
		if err != nil {
			return fail(stderr, "读取数据目录失败: %v", err)
		}
		if len(entries) == 0 {
			freshDir = true
		}
	}

	if !freshDir {
		info, err := os.Stat(blocksPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fail(stderr, "数据目录非空但缺少 blocks.dat；拒绝将其当作空库初始化")
			}
			return fail(stderr, "检查 blocks.dat 失败: %v", err)
		}
		if info.Size() == 0 {
			return fail(stderr, "%v: %s", blockchain.ErrCorruptGenesisIdentity, blocksPath)
		}
	}

	lock, err := storage.AcquireDirLock(*dataDir)
	if err != nil {
		return fail(stderr, "锁定数据目录失败: %v", err)
	}
	defer func() { _ = lock.Release() }()

	store, err := storage.OpenFileBlockStoreStrict(*dataDir)
	if err != nil {
		return fail(stderr, "打开区块存储失败: %v", err)
	}
	defer func() { _ = store.Close() }()

	if freshDir {
		if _, err := blockchain.InitializeBlockchainStore(store); err != nil {
			return fail(stderr, "初始化 canonical Genesis 失败: %v", err)
		}
		fmt.Fprintf(stdout, "已初始化 canonical Genesis: %s\n", blockchain.CanonicalGenesisHashHex())
		return 0
	}

	genesis, err := blockchain.VerifyGenesisIdentity(store)
	if err != nil {
		if genesis != nil {
			return fail(stderr, "%v；拒绝覆盖现有数据集", err)
		}
		return fail(stderr, "%v；拒绝覆盖现有数据集", err)
	}
	_ = genesis
	return fail(stderr, "%v；拒绝覆盖现有数据集", blockchain.ErrAlreadyInitialized)
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
	fmt.Fprintf(stdout, "挖矿状态: %s\n", miningStateText(*st))
	// 仅在异常状态（停滞/失败）时补充原因：这正是开发者区分
	// 「政策终态（补贴耗尽，不是故障）」与「结构性错误（需要修复）」所需的最小信息。
	// 正常状态下不额外输出，保持既有输出格式不变。
	if st.MiningState == string(MiningStalled) || st.MiningState == string(MiningFailed) ||
		st.MiningState == string(MiningWaitingSync) {
		fmt.Fprintf(stdout, "挖矿原因: %s\n", st.MiningReason)
	}
	return 0
}

// miningStateText 渲染挖矿状态。
//
// 优先使用语义状态 MiningState（PHASE MINING-REMEDIATION-1）：它区分
// 「正在对合法模板求解 PoW」（运行中）与「不存在合法候选、未执行任何 PoW」（已停滞）
// 以及「结构性错误」（失败）。修复前只有一个布尔值，停摆期间会误报「运行中」。
//
// 当服务端未给出 MiningState 时（例如旧版节点或第三方实现）退化为按
// Mining 布尔值渲染，保持向后兼容。
func miningStateText(st control.StatusInfo) string {
	switch st.MiningState {
	case string(MiningRunning):
		return "运行中"
	case string(MiningStarting):
		return "启动中"
	case string(MiningWaitingSync):
		return "等待同步（本地落后于网络，暂停自动挖矿）"
	case string(MiningStalled):
		return "已停滞（不存在合法候选区块，未执行 PoW）"
	case string(MiningFailed):
		return "失败（模板结构性错误，挖矿已终止）"
	case string(MiningStopping):
		return "停止中"
	case string(MiningStopped):
		return "已停止"
	}
	return miningText(st.Mining)
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
	tokenFile := fs.String("token-file", "secrets/control-token", "mutation token 文件（0600；相对工作目录）")
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
	tok, err := control.LoadTokenFile(*tokenFile)
	if err != nil {
		return fail(stderr, "%v", err)
	}

	client := control.NewClient(*rpc)
	client.SetToken(tok)
	resp, err := client.Send(control.SendRequest{To: *to, Amount: *amount, Fee: *fee})
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

// 注：`node mine` 子命令（按需出块 CLI 入口）已随 POST /mine 端点整体下线。
// 需要让节点出块的场景改用持续挖矿：`node -mine -maxblocks <N>`（启动即挖、挖满退出），
// 或对运行中的节点 `POST /mine/start` + 轮询 + `POST /mine/stop`。

// stopWaitTimeout 发出停止请求后，等待节点真正退出的最长时限。
// 覆盖「挖矿节点需要放弃当前候选区块后再退出」的正常耗时，
// 超时则说明停止请求可能未被处理，必须如实报错而不是假装成功。
const stopWaitTimeout = 15 * time.Second

// stopPollInterval 等待节点退出时的探活间隔。
const stopPollInterval = 100 * time.Millisecond

// cmdStop 请求运行中的节点优雅停止（PHASE PRODUCT-DEV-1B）。
//
// 为什么需要它：在 Windows 下，后台/脚本启动的节点进程无法被投递可捕获的
// 控制台信号（Ctrl+C 只能广播到整个控制台组，强杀又不会走清理链），
// 于是「正常停止」长期只能靠 taskkill /F —— 那会留下 node.lock 残留。
// 本命令走既有本机控制接口，让节点自己走一遍与 SIGINT 完全相同的关闭链。
//
// 判定成功的标准不是「请求发出去了」，而是「节点真的退出了」：
// 控制接口不再响应才算停止完成（STOP-INV-01）。
func cmdStop(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("stop", stderr)
	rpc := fs.String("rpc", control.DefaultAddr, "节点控制接口地址")
	tokenFile := fs.String("token-file", "secrets/control-token", "mutation token 文件（0600；相对工作目录）")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	client := control.NewClient(*rpc)
	// 先探活：节点不在时给出与其它在线命令一致的可操作提示。
	if _, err := client.Status(); err != nil {
		return fail(stderr, "%v\n（提示：请先用 `node` 启动节点）", err)
	}
	// PHASE CONTROL-AUTH-1：/stop 为 mutation 端点，需要 Bearer Token。
	tok, err := control.LoadTokenFile(*tokenFile)
	if err != nil {
		return fail(stderr, "%v", err)
	}
	client.SetToken(tok)

	// 节点可能在写出响应前就关闭了连接——这是「已经在退出了」的正常表现，
	// 因此不把它当失败；最终结论以「节点是否真的退出」为准。
	if _, err := client.Stop(); err != nil {
		fmt.Fprintf(stderr, "提示: 停止请求的响应未完整收到（%v），继续确认节点是否已退出...\n", err)
	} else {
		fmt.Fprintf(stdout, "已发送停止请求到 %s\n", *rpc)
	}

	deadline := time.Now().Add(stopWaitTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(stopPollInterval)
		if _, err := client.Status(); err != nil {
			fmt.Fprintf(stdout, "节点已停止（控制接口 %s 不再响应；数据目录锁已释放）\n", *rpc)
			return 0
		}
	}
	return fail(stderr, "节点在 %v 内未退出，停止请求可能未被处理；可检查节点日志或改用 Ctrl+C",
		stopWaitTimeout)
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
	// 子命令：wallet encrypt（v1 明文 → v2 加密迁移）
	if len(args) > 0 && args[0] == "encrypt" {
		return cmdWalletEncrypt(args[1:], stdout, stderr)
	}
	fs := newFlagSet("wallet", stderr)
	dataDir := fs.String("datadir", defaultDataDir(), "数据目录")
	addressOnly := fs.Bool("address", false, "只输出地址（便于脚本使用）")
	pwFile := fs.String("password-file", "", "钱包口令文件（0600；P0-4 唯一口令来源）")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// --address 无需口令：v2 信封的公钥为明文（地址本就是公开身份）。
	if *addressOnly {
		addr, err := wallet.LoadAddressOnly(wallet.DefaultWalletPath(*dataDir))
		if err != nil {
			return fail(stderr, "读取地址失败: %v", err)
		}
		fmt.Fprintln(stdout, addr)
		return 0
	}
	if *pwFile == "" {
		return fail(stderr, "需要口令文件：请提供 --password-file <0600 口令文件>")
	}
	password, err := wallet.LoadPasswordFile(*pwFile)
	if err != nil {
		return fail(stderr, "口令文件无效: %v", err)
	}
	defer wallet.ZeroBytes(password)
	w, created, err := wallet.LoadOrCreateForDataDir(*dataDir, password)
	if err != nil {
		return fail(stderr, "加载/创建钱包失败: %v", err)
	}
	if created {
		fmt.Fprintf(stdout, "已创建新加密钱包: %s\n", wallet.DefaultWalletPath(*dataDir))
	} else {
		fmt.Fprintf(stdout, "已加载钱包: %s\n", wallet.DefaultWalletPath(*dataDir))
	}
	fmt.Fprintf(stdout, "地址: %s\n", w.Address())
	return 0
}

// cmdWalletEncrypt v1 明文钱包 → v2 加密钱包的迁移命令（P0-4）。
//
// 流程：读取旧路径 v1 → 口令加密 → 原子写入新路径 → 删除旧明文文件。
// 拒绝覆盖已存在的新路径钱包；迁移成功后旧明文文件被删除（不留明文残留）。
func cmdWalletEncrypt(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("wallet encrypt", stderr)
	dataDir := fs.String("datadir", defaultDataDir(), "数据目录")
	pwFile := fs.String("password-file", "", "钱包口令文件（0600；P0-4 唯一口令来源）")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *pwFile == "" {
		return fail(stderr, "需要口令文件：请提供 --password-file <0600 口令文件>")
	}
	legacyPath := wallet.LegacyWalletPath(*dataDir)
	newPath := wallet.DefaultWalletPath(*dataDir)
	if _, err := os.Stat(newPath); err == nil {
		return fail(stderr, "已存在加密钱包 %s，拒绝覆盖", newPath)
	}
	legacyData, err := os.ReadFile(legacyPath)
	if err != nil {
		return fail(stderr, "未发现旧版明文钱包 %s: %v", legacyPath, err)
	}
	if ver, verr := wallet.DetectVersion(legacyData); verr != nil || ver != 1 {
		return fail(stderr, "旧钱包文件不是 v1 明文格式，无需迁移")
	}
	w, err := wallet.LoadFromFile(legacyPath)
	if err != nil {
		return fail(stderr, "读取旧钱包失败: %v", err)
	}
	password, err := wallet.LoadPasswordFile(*pwFile)
	if err != nil {
		return fail(stderr, "口令文件无效: %v", err)
	}
	defer wallet.ZeroBytes(password)
	if err := w.SaveEncryptedToFile(newPath, password); err != nil {
		return fail(stderr, "写入加密钱包失败: %v", err)
	}
	// 迁移成功后删除旧明文文件：P0-4 的目标就是消灭明文私钥残留。
	if err := os.Remove(legacyPath); err != nil {
		return fail(stderr, "加密钱包已写入 %s，但删除旧明文文件失败（请手动删除 %s）: %v", newPath, legacyPath, err)
	}
	fmt.Fprintf(stdout, "迁移完成：%s → %s（v2 加密）；旧明文文件已删除\n", legacyPath, newPath)
	fmt.Fprintf(stdout, "地址: %s\n请妥善备份口令文件，口令丢失则钱包无法恢复\n", w.Address())
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

// ---- 离线命令：reset（PHASE PRODUCT-DEV-1A）----

// resetDataFiles 一个数据目录中的全部**持久化状态产物**（§2 B/C）。
//
// 切片顺序即删除顺序，且顺序是有意义的：blocks.dat 排第一。
// 若节点仍在运行，Windows 会因文件被占用而在第一个文件上直接失败，
// 从而实现「一个都没删就失败」，而不是删掉钱包之后才失败（RESET-INV-07）。
//
// node.lock 不在此列：它由 DirLock.Release 删除（持有锁时不能删自己打开的
// 文件），或在 --force 处理陈旧锁时单独删除。
//
// 这里只列真正会落盘的文件。UTXO / mempool / 网络状态纯内存，日志只在内存
// ring 中，没有配置文件（internal/config 是未被引用的死包），因此无需清理。
// P0-4：钱包已迁移到 <datadir>/secrets/wallet.json；旧版 <datadir>/wallet.json
// 仍列入清理（迁移残留/回滚场景）。删除顺序的意义不变（见 resetDataFiles 注释）。
var resetDataFiles = []string{"blocks.dat", "wallet.json", filepath.Join("secrets", "wallet.json")}

// resetReport 是 reset 的结果报告（-json 输出，字段风格与 verify 一致）。
type resetReport struct {
	DataDir string   `json:"data_dir"`
	Removed []string `json:"removed"` // 本次实际删除的文件
	Absent  []string `json:"absent"`  // 本就不存在（已干净）
	Failed  []string `json:"failed"`  // 删除失败
	Errors  []string `json:"errors"`  // 失败的具体原因
	Clean   bool     `json:"clean"`   // 目录已无任何已知状态文件
	Aborted bool     `json:"aborted"` // 首个文件失败后中止，剩余文件未被处理
}

// cmdReset 清空一个数据目录的实验状态，使其回到「首次运行」状态（§3 Option C）。
//
// 语义：删除 blocks.dat + wallet.json + node.lock。下次启动节点时会自动生成
// 确定性创世区块与新钱包 —— 这就是一个可重复实验的干净起点。
//
// 不保留钱包（§3 否决 Option B）：链已删除而保留一个余额归零、UTXO 不存在的
// 钱包是误导性状态；且 coinbase 归属矿工地址，保留旧钱包会让新链与「真正首次
// 运行」的链不一致，反而削弱可复现性。需要长期保留身份请用 -datadir 分目录。
//
// 安全模型（§4）：
//   - 默认需要交互确认；--force 跳过（脚本 / CI 使用）；
//   - 非终端且无 --force 时直接失败，避免无人看管的静默破坏；
//   - 先尝试获取数据目录独占锁。取不到说明存在 node.lock：
//     无 --force 时中止，有 --force 时按陈旧锁处理；
//   - 任一文件删除失败 => 退出码非 0 并列出失败项，绝不静默部分成功。
func cmdReset(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("reset", stderr)
	dataDir := fs.String("datadir", defaultDataDir(), "数据目录")
	force := fs.Bool("force", false, "跳过确认提示（脚本 / CI 使用）")
	asJSON := fs.Bool("json", false, "以 JSON 输出（机器可读）")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// 1) 占用探测：能否取得数据目录独占锁。
	//    先记录 node.lock 是否已存在：探测本身会创建它，
	//    否则任何干净的目录都会被报告成「删掉了 node.lock」。
	lockPath := filepath.Join(*dataDir, "node.lock")
	lockExisted := true
	if _, err := os.Stat(lockPath); err != nil {
		lockExisted = false
	}
	lock, lockErr := storage.AcquireDirLock(*dataDir)
	heldLock := lockErr == nil
	if lockErr != nil && !errors.Is(lockErr, storage.ErrDatadirLocked) {
		return fail(stderr, "探测数据目录失败: %v", lockErr)
	}
	if heldLock {
		// 持有锁期间其它节点进程无法启动；结束时 Release 会删掉 node.lock。
		defer func() { _ = lock.Release() }()
	} else if !*force {
		return fail(stderr,
			"数据目录已被占用（存在 node.lock），未删除任何文件：\n"+
				"  目录：%s\n"+
				"  请先停止使用该目录的节点进程后重试；\n"+
				"  若确认没有节点在运行（崩溃残留的陈旧锁），请追加 --force。",
			*dataDir)
	}

	// 2) 确认（默认需要，--force 跳过）
	if !*force {
		pending := make([]string, 0, len(resetDataFiles)+1)
		for _, name := range resetDataFiles {
			if _, err := os.Stat(filepath.Join(*dataDir, name)); err == nil {
				pending = append(pending, name)
			}
		}
		if !heldLock {
			pending = append(pending, "node.lock")
		}
		if len(pending) > 0 && !confirmReset(stdout, stderr, *dataDir, pending) {
			fmt.Fprintln(stderr, "已取消，未删除任何文件。")
			return 1
		}
	}

	// 3) 删除（顺序敏感，见 resetDataFiles 注释）
	//
	// 失败即中止（RESET-INV-07）：第一个文件删除失败就停止，不再处理剩余文件。
	// 「blocks.dat 排第一」只保证链最先被尝试；若失败后继续删 wallet.json，
	// 就会留下「链还在、钱包没了」的不可恢复状态 —— 这比什么都不删更糟。
	rep := resetReport{
		DataDir: *dataDir,
		Removed: []string{},
		Absent:  []string{},
		Failed:  []string{},
		Errors:  []string{},
	}
	var aborted bool
	for _, name := range resetDataFiles {
		path := filepath.Join(*dataDir, name)
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				rep.Absent = append(rep.Absent, name)
				continue
			}
			rep.Failed = append(rep.Failed, name)
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", name, err))
			aborted = true
			break
		}
		if err := os.Remove(path); err != nil {
			rep.Failed = append(rep.Failed, name)
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", name, err))
			aborted = true
			break
		}
		rep.Removed = append(rep.Removed, name)
	}
	if aborted {
		// 中止：保留现场。若本次探测自己创建了 node.lock，defer 的 Release 会删掉它，
		// 使目录回到操作前的样子；用户持有的陈旧锁则原样保留。
	} else if heldLock {
		// Windows 下无法删除自己仍打开的文件，交由锁自身的释放逻辑处理。
		if err := lock.Release(); err != nil {
			rep.Failed = append(rep.Failed, "node.lock")
			rep.Errors = append(rep.Errors, fmt.Sprintf("node.lock: %v", err))
		} else if lockExisted {
			rep.Removed = append(rep.Removed, "node.lock")
		} else {
			// 锁是本次探测自己创建的，不属于「用户的状态」，不计入已删除。
			rep.Absent = append(rep.Absent, "node.lock")
		}
	} else {
		resetOne(&rep, lockPath, "node.lock")
	}

	// P0-4：secrets 目录若已空（钱包文件已删），一并清理，避免空目录残留
	// 导致后续 init 误判"目录非空"。
	if !aborted {
		secretsDir := filepath.Join(*dataDir, wallet.WalletSecretsDir)
		if entries, err := os.ReadDir(secretsDir); err == nil && len(entries) == 0 {
			_ = os.Remove(secretsDir)
		}
	}

	// 4) 复核：目录里是否还有任何已知状态文件（RESET-INV-01/02/03）
	rep.Aborted = aborted
	rep.Clean = len(rep.Failed) == 0
	for _, name := range append(append([]string{}, resetDataFiles...), "node.lock") {
		if _, err := os.Stat(filepath.Join(*dataDir, name)); err == nil {
			rep.Clean = false
		}
	}

	if *asJSON {
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return fail(stderr, "序列化 reset 报告失败: %v", err)
		}
		fmt.Fprintln(stdout, string(data))
		if !rep.Clean {
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "[reset] 数据目录  : %s\n", rep.DataDir)
	fmt.Fprintf(stdout, "[reset] 已删除    : %s\n", joinOrNone(rep.Removed))
	fmt.Fprintf(stdout, "[reset] 本不存在  : %s\n", joinOrNone(rep.Absent))
	if !rep.Clean {
		fmt.Fprintf(stdout, "[reset] 结果      : FAIL\n")
		fmt.Fprintf(stdout, "[reset] 删除失败  : %s\n", strings.Join(rep.Failed, ", "))
		for _, e := range rep.Errors {
			fmt.Fprintf(stdout, "[reset] 原因      : %s\n", e)
		}
		if aborted {
			fmt.Fprintf(stdout, "[reset] 说明      : 已在第一个失败处中止，剩余文件未被处理（数据安全优先）\n")
		}
		fmt.Fprintln(stderr, "错误: reset 未完全成功，数据目录可能处于不一致状态，请检查后重试。")
		return 1
	}
	if len(rep.Removed) == 0 {
		fmt.Fprintf(stdout, "[reset] 结果      : OK（数据目录已是干净状态，无需删除）\n")
		return 0
	}
	fmt.Fprintf(stdout,
		"[reset] 结果      : OK（已回到首次运行状态；下次启动将重新生成确定性创世与新钱包）\n")
	return 0
}

// resetOne 删除单个文件并把结果记入报告。
func resetOne(rep *resetReport, path, name string) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			rep.Absent = append(rep.Absent, name)
			return
		}
		rep.Failed = append(rep.Failed, name)
		rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", name, err))
		return
	}
	if err := os.Remove(path); err != nil {
		rep.Failed = append(rep.Failed, name)
		rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", name, err))
		return
	}
	rep.Removed = append(rep.Removed, name)
}

// confirmReset 在终端上请求确认。非终端（脚本 / CI）无法交互确认时返回 false。
func confirmReset(stdout, stderr io.Writer, dataDir string, pending []string) bool {
	if !isTerminalFn(cliStdin) {
		fmt.Fprintf(stderr,
			"错误: reset 是破坏性操作，需要确认；当前标准输入不是终端，无法交互确认。\n"+
				"  若确认要清空 %s，请追加 --force。\n", dataDir)
		return false
	}
	fmt.Fprintf(stdout, "即将清空数据目录: %s\n将删除: %s\n", dataDir, strings.Join(pending, ", "))
	fmt.Fprint(stdout, "此操作不可撤销。输入 yes 继续: ")
	line, err := bufio.NewReader(cliStdin).ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		fmt.Fprintln(stdout)
		return false
	}
	return strings.TrimSpace(line) == "yes"
}

// isTerminalFn 判断输入是否来自终端。抽成变量只为让测试能覆盖
// 「终端 + 输入 yes」与「终端 + 输入其它」两条分支；生产恒为 isTerminal。
var isTerminalFn = isTerminal

// isTerminal 判断 r 是否是字符设备（终端 / 控制台）。
func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// joinOrNone 把文件列表拼成可读串，空列表显示为「（无）」。
func joinOrNone(list []string) string {
	if len(list) == 0 {
		return "（无）"
	}
	return strings.Join(list, ", ")
}
