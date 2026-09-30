// cmd/node 是节点的可执行程序入口。
//
// 运行流程：
//  1. 打开（或初始化）持久化区块存储，加载/回放本地区块链
//  2. 生成或加载矿工钱包
//  3. 启动 P2P 监听、连接种子节点
//  4. 矿工模式下进入挖矿循环；全节点模式下仅转发与同步
//
// main.go 只负责组装依赖与解析参数；业务语义在 service.go / nodeapi.go，
// 命令行子命令在 cli.go。
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/control"
	"p2pchain/internal/mempool"
	"p2pchain/internal/p2p"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

// MempoolSize 交易池容量上限。
const MempoolSize = 2048

// nodeConfig 节点启动参数。
type nodeConfig struct {
	ListenAddr    string
	RPCAddr       string
	Seeds         []string
	DataDir       string
	Mine          bool
	MaxBlocks     int
	Miners        int    // 并行挖矿 worker 数，<=1 表示单线程
	AuthTokenFile string // mutation 端点 Bearer Token 文件（路径可上命令行，token 本身绝不）
}

// nodeRuntime 一个已启动节点的全部运行时组件。
// 抽出来是为了让端到端测试能以进程内方式启动真实节点（同样的组装路径），
// 而不是把 main 里的逻辑复制一份到测试里——测试必须跑在被测代码上。
type nodeRuntime struct {
	store     *storage.FileBlockStore
	chain     *blockchain.Blockchain
	pool      *mempool.Mempool
	svc       *nodeService
	p2p       *p2p.Node
	ctl       *control.Server
	seeds     []string
	stopPeer  chan struct{}
	lock      *storage.DirLock // 数据目录进程独占锁，Close 时释放
	closeOnce sync.Once
}

// newNodeRuntime 按配置组装并启动节点（P2P 监听 + 控制接口 + 种子重连）。
// 返回后节点已可接受连接与查询；不启动挖矿（由调用方决定）。
func newNodeRuntime(cfg nodeConfig) (*nodeRuntime, error) {
	// 第一步：独占锁定数据目录。失败（含已被占用）直接返回，绝不打开 blocks.dat。
	lock, err := storage.AcquireDirLock(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	// 防御：若后续初始化中途 panic（而非返回 error），保证本进程持有的锁被释放。
	// 正常/错误返回路径由各分支显式 Release 处理；此处仅兜底 panic 与未覆盖路径，
	// 不吞 panic——defer 释放后 panic 继续向上传播，进程以非零码退出。
	var initDone bool
	defer func() {
		if !initDone {
			_ = lock.Release()
		}
	}()

	// F-3B：0 字节的既有 blocks.dat 按损坏身份处理，不能当作空库自动创世。
	// 不存在 blocks.dat 的目录仍交给统一链加载路径返回未初始化错误；只有显式
	// `init` 命令才允许在全新目录中创建 canonical Genesis。
	blocksPath := filepath.Join(cfg.DataDir, "blocks.dat")
	if info, statErr := os.Stat(blocksPath); statErr == nil && info.Size() == 0 {
		_ = lock.Release()
		return nil, fmt.Errorf("加载区块链失败: %w", blockchain.ErrCorruptGenesisIdentity)
	} else if statErr != nil {
		_ = lock.Release()
		if errors.Is(statErr, os.ErrNotExist) {
			return nil, fmt.Errorf("加载区块链失败: %w", blockchain.ErrUninitializedStore)
		}
		return nil, fmt.Errorf("检查区块数据文件失败: %w", statErr)
	}

	store, err := storage.OpenFileBlockStoreStrict(cfg.DataDir)
	if err != nil {
		_ = lock.Release()
		return nil, fmt.Errorf("打开区块存储失败: %w", err)
	}

	chain, err := blockchain.NewBlockchainFromStore(store)
	if err != nil {
		_ = store.Close()
		_ = lock.Release()
		return nil, fmt.Errorf("加载区块链失败: %w", err)
	}
	tip, err := chain.Tip()
	if err != nil {
		_ = store.Close()
		_ = lock.Release()
		return nil, fmt.Errorf("读取链尾失败: %w", err)
	}
	log.Printf("[node] 本地区块链已就绪: 高度=%d 链尾=%s 数据文件=%s",
		chain.Height(), tip.Header.HashHex(), store.FilePath())

	walletPath := filepath.Join(cfg.DataDir, "wallet.json")
	nodeWallet, created, err := wallet.LoadOrCreate(walletPath)
	if err != nil {
		_ = store.Close()
		_ = lock.Release()
		return nil, fmt.Errorf("加载/创建钱包失败: %w", err)
	}
	if created {
		log.Printf("[node] 已生成新钱包: %s", walletPath)
	}
	log.Printf("[node] 节点钱包地址=%s", nodeWallet.Address())

	pool := mempool.New(MempoolSize)
	svc := newNodeService(chain, pool, nodeWallet)
	svc.miners = cfg.Miners
	if svc.miners < 1 {
		svc.miners = 1
	}

	genesis, err := chain.BlockByHeight(0)
	if err != nil {
		_ = store.Close()
		_ = lock.Release()
		return nil, fmt.Errorf("读取创世区块失败: %w", err)
	}
	p2pNode := p2p.NewNode(cfg.ListenAddr, nodeWallet.Address(), genesis.Header.HashHex(), svc)
	svc.net = p2pNode
	p2pNode.SetHeightProvider(func() int { return chain.Height() })
	// REORG-1H：握手携带链尾累积工作量与链尾哈希，让对端能按「工作量」而非
	// 「高度」判断是否追赶，并发现彼此处于不同分支（work-aware 同步 + 分支发现）。
	p2pNode.SetChainStatusProvider(func() (string, string) {
		w := chain.BestTipWork()
		work := ""
		if w != nil {
			work = w.String()
		}
		tipHash := ""
		if tip, err := chain.Tip(); err == nil {
			tipHash = tip.Header.HashHex()
		}
		return work, tipHash
	})

	// PHASE TEST-INFRASTRUCTURE-REMEDIATION-1：同步完成 P2P 端口绑定。
	// Start 返回即 listener 已就绪（ListenAddr 就绪契约），消除
	// 「newNodeRuntime 返回但监听地址仍是配置占位值（127.0.0.1:0）」的竞态。
	// 绑定失败现在会让节点启动明确失败（此前只记日志，节点会以无监听状态继续运行）。
	if err := p2pNode.Start(); err != nil {
		p2pNode.Stop()
		_ = store.Close()
		_ = lock.Release()
		return nil, fmt.Errorf("启动 P2P 监听失败: %w", err)
	}

	ctl := control.NewServer(svc)
	// PHASE CONTROL-AUTH-1：mutation 端点（/send /mine /stop）启用 Bearer Token。
	// token 文件缺失/无效时 fail-closed：mutation 一律 401，读端点不受影响。
	// 日志只记路径与结论，绝不记 token 内容。
	if tok, tokErr := control.LoadTokenFile(cfg.AuthTokenFile); tokErr != nil {
		log.Printf("[node] 警告：mutation token 不可用（%v）；/send /mine /stop 已禁用（fail-closed），读端点不受影响", tokErr)
	} else {
		ctl.SetAuthToken(tok)
		log.Printf("[node] mutation 端点认证已启用（token 文件: %s）", cfg.AuthTokenFile)
	}
	actualRPC, err := ctl.Start(cfg.RPCAddr)
	if err != nil {
		p2pNode.Stop()
		_ = store.Close()
		_ = lock.Release()
		return nil, fmt.Errorf("启动控制接口失败: %w", err)
	}
	if !isLoopback(actualRPC) {
		log.Printf("[node] 警告：控制接口监听在 %s，非回环地址且无鉴权，请勿暴露到不可信网络", actualRPC)
	}
	log.Printf("[node] 控制接口已启动: %s", actualRPC)

	rt := &nodeRuntime{
		store: store, chain: chain, pool: pool, svc: svc, p2p: p2pNode, ctl: ctl,
		seeds: cfg.Seeds, stopPeer: make(chan struct{}), lock: lock,
	}
	// PHASE P2P-SYNC-LIVENESS-MINIMUM-SAFE-FIX-1：启动批量同步在途调度器。
	//
	// 调度器负责「TTL 到期 / 对端断开 / 发送失败」后的**有界重试与 peer failover**，
	// 它运行在独立 goroutine 上，与任何 peer 的读循环解耦。
	// 未启动时的行为与修复前完全一致（失败即释放，把机会交回下一次握手）。
	svc.startSyncScheduler()
	rt.connectSeeds() // 首次连接
	rt.watchSeeds()   // 断线后自动重连
	initDone = true   // 初始化完成：上面的 defer 释放兜底不再触发
	return rt, nil
}

// connectSeeds 尝试连接全部尚未连接的种子节点。
func (rt *nodeRuntime) connectSeeds() {
	connected := make(map[string]bool)
	for _, a := range rt.p2p.PeerAddrs() {
		connected[a] = true
	}
	for _, seed := range rt.seeds {
		if seed == "" || connected[seed] {
			continue
		}
		if err := rt.p2p.ConnectToPeer(seed); err != nil {
			log.Printf("[p2p] 连接种子节点 %s 失败: %v", seed, err)
		}
	}
}

// watchSeeds 周期性补齐与种子节点的连接。
//
// 为什么要重连：种子节点重启、网络抖动都会让连接消失，而单链 PoW 节点如果
// 掉线后再也不重连，就会永久成为一个「孤岛链」——继续挖自己的分叉。
// 这里用「只补不足」的简单策略：已连接的种子不动，缺失的才重连。
func (rt *nodeRuntime) watchSeeds() {
	if len(rt.seeds) == 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(seedReconnectInterval)
		defer ticker.Stop()
		for {
			select {
			case <-rt.stopPeer:
				return
			case <-ticker.C:
				rt.connectSeeds()
			}
		}
	}()
}

// seedReconnectInterval 种子节点重连检查周期。
const seedReconnectInterval = 5 * time.Second

// Close 关闭节点持有的全部资源。
//
// 幂等：调用方可能先手动 Close（如测试模拟节点重启）再由 t.Cleanup/defer
// 关一次——裸 close(stopPeer) 会在第二次调用时 panic，因此用 sync.Once 保护。
func (rt *nodeRuntime) Close() {
	rt.closeOnce.Do(func() {
		if rt.stopPeer != nil {
			close(rt.stopPeer)
		}
		if rt.svc != nil {
			// 先停同步调度器，再停 P2P：避免关停路径上还在发起新的同步请求。
			rt.svc.stopSyncScheduler()
		}
		if rt.ctl != nil {
			_ = rt.ctl.Stop()
		}
		if rt.p2p != nil {
			rt.p2p.Stop()
		}
		if rt.store != nil {
			_ = rt.store.Close()
		}
		if rt.lock != nil {
			// 不能静默吞掉：释放失败意味着 node.lock 残留，下次启动会被拒，
			// 用户必须知道这件事（STOP-INV-09：异常不得静默报告成功）。
			if err := rt.lock.Release(); err != nil {
				log.Printf("[node] 警告：释放数据目录锁失败，可能需要手动删除 %s: %v",
					rt.lock.Path(), err)
			}
		}
	})
}

// testPanicAtStart 仅供测试注入 panic（生产代码恒为 nil，零副作用，非后门功能）。
// 用于在 runtime start 阶段注入 panic，验证「最外层 defer 在 panic 路径释放锁」的契约
// （PHASE P3.1 §6 Test 7）。任何生产路径都不会设置它。
var testPanicAtStart func()

// runNode 启动一个完整节点（P2P + 控制接口 + 可选挖矿），阻塞至进程退出。
func runNode(args []string) { startNode(args, false) }

// runNodeUI 与 runNode 完全同源，区别只有一个：控制接口就绪后，
// 用系统默认浏览器打开 Developer Console 页面（`node ui` 子命令）。
func runNodeUI(args []string) { startNode(args, true) }

// nodeFlags 节点启动选项（node / ui 共用）的解析结果。
//
// 抽出来是为了让 startNode 与 main 的「子命令预扫描」使用**同一份**选项定义。
// 若两处各写一份，形如 `-datadir X verify` 的组合会在「预扫描认为选项在哪里结束」
// 与「实际解析认为选项在哪里结束」之间产生分歧，P1 会以另一种形式复发。
type nodeFlags struct {
	listen      string
	rpc         string
	seed        string
	dataDir     string
	mine        bool
	maxBlocks   int
	miners      int
	authTokFile string
}

// newNodeFlagSet 创建节点选项集。
//
// errHandling 由调用方决定：
//   - startNode 用 ExitOnError：解析失败直接打印用法并退出（既有行为，不变）；
//   - main 的预扫描用 ContinueOnError：只需要知道「选项在哪里结束」，
//     解析失败时把 argv 原样交回 startNode，由它按既有行为报错。
func newNodeFlagSet(errHandling flag.ErrorHandling) (*flag.FlagSet, *nodeFlags) {
	nf := &nodeFlags{}
	fs := flag.NewFlagSet("node", errHandling)
	fs.StringVar(&nf.listen, "listen", ":6688", "本节点监听地址")
	fs.StringVar(&nf.rpc, "rpc", control.DefaultAddr, "控制接口监听地址（仅本机）")
	// PHASE CONTROL-AUTH-1：mutation 端点 token 文件。相对路径按工作目录解析，
	// 服务端（WorkingDirectory）与 CLI/ExecStop 同目录运行时天然一致。
	// 只传路径，token 本身绝不进命令行/环境变量/日志。
	fs.StringVar(&nf.authTokFile, "auth-token-file", "secrets/control-token",
		"mutation 端点（/send /mine /stop）Bearer Token 文件（0600；相对工作目录；缺省 secrets/control-token）")
	fs.StringVar(&nf.seed, "seed", "", "种子节点地址，多个用逗号分隔；留空表示作为第一个节点启动")
	fs.StringVar(&nf.dataDir, "datadir", defaultDataDir(), "数据目录（存放区块数据与钱包）")
	fs.BoolVar(&nf.mine, "mine", false, "是否启用挖矿")
	fs.IntVar(&nf.maxBlocks, "maxblocks", 0, "挖矿最多产出多少个区块，0 表示不限（用于测试网可控出块）")
	fs.IntVar(&nf.miners, "miners", runtime.NumCPU(), "并行挖矿的 worker 数，1 表示单线程")
	return fs, nf
}

// startNode 是 runNode / runNodeUI 的共同实现。
func startNode(args []string, openConsole bool) {
	fs, nf := newNodeFlagSet(flag.ExitOnError)
	_ = fs.Parse(args)

	// 最外层 defer（cmd 层级，§2.1）：无论正常返回、收到信号、还是初始化/运行期 panic，
	// 只要 rt 已成功构建，就通过 rt.Close() 释放全部资源（含数据目录锁，幂等 + pid 校验）。
	// rt 为 nil 时（newNodeRuntime 在返回前已自行释放锁并报错）此处不动作。
	// 在组装节点之前接入日志环形缓冲：这样从「本地区块链已就绪」起的全部日志
	// 都能被控制台读到。终端输出行为完全不变（仍是同一份字节写到 stderr）。
	logRing := installLogRing()

	var rt *nodeRuntime
	defer func() {
		if rt != nil {
			rt.Close()
		}
	}()

	rt2, err := newNodeRuntime(nodeConfig{
		ListenAddr:    nf.listen,
		RPCAddr:       nf.rpc,
		Seeds:         splitSeeds(nf.seed),
		DataDir:       nf.dataDir,
		Mine:          nf.mine,
		MaxBlocks:     nf.maxBlocks,
		Miners:        nf.miners,
		AuthTokenFile: nf.authTokFile,
	})
	if err != nil {
		// 数据目录被另一节点占用时给出明确、可执行的用户级错误（与「数据损坏」区分）。
		if errors.Is(err, storage.ErrDatadirLocked) {
			log.Fatalf("[node] 数据目录已被另一个节点进程占用，未启动本节点：\n  目录：%s\n  同一数据目录一次只能由一个节点进程使用；请勿删除 blocks.dat，也勿重复启动。\n  如果确认没有其他节点进程在运行，请手动删除 %s 后重新启动。",
				nf.dataDir, filepath.Join(nf.dataDir, "node.lock"))
		}
		log.Fatalf("[node] %v", err)
	}
	rt = rt2
	// 把日志来源交给控制接口，使 GET /logs 能返回真实日志（未接入时返回空数组）。
	rt.ctl.SetLogProvider(logRing)

	if openConsole {
		openConsolePage(rt.ctl.Addr())
	}

	// 测试钩子（生产恒为 nil，零副作用，非后门功能）：用于在 runtime start 注入 panic，
	// 验证「最外层 defer 在 panic 路径释放锁」的契约（PHASE P3.1 §6 Test 7）。
	if testPanicAtStart != nil {
		testPanicAtStart()
	}

	// ---- 停止出口（PHASE PRODUCT-DEV-1B §3）----
	//
	// stopCh 是节点唯一的「停止请求」出口，有两个来源：
	//   1. OS 信号 SIGINT / SIGTERM（真实终端按 Ctrl+C、POSIX 的 kill）；
	//   2. 控制接口 POST /stop（可编程停止，Windows 后台进程也能停）。
	// 两者都只做一件事：关闭 stopCh。真正的关闭由下面的同一条路径执行，
	// 因此「被信号停」与「被命令停」的清理行为完全一致。
	//
	// 关键改变：旧实现在信号 goroutine 里直接 rt.Close() + os.Exit(0)，
	// 这会与主流程并发——若信号恰好在 AddBlock 写盘期间到达，store.Close()
	// 会与写入并发执行，存在损坏 blocks.dat 的风险（STOP-INV-05）。
	// 现在改为单出口：主流程自己走到 return，由最外层 defer 执行 rt.Close()。
	var stopOnce sync.Once
	stopCh := make(chan struct{})
	requestStop := func() { stopOnce.Do(func() { close(stopCh) }) }
	rt.ctl.SetStopHook(requestStop)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	if nf.mine {
		// PHASE MINING-LIFECYCLE-1：-mine boot 语义不变（按既有 flag 拉起持续挖矿），
		// 但挖矿 goroutine 的停止源改为生命周期管理器的 minerStopCh（与 node stopCh
		// 分离；节点停机时 main 在 rt.Close() 前先停挖并等待退出——「先 minerStop
		// 后既有关闭链」的冻结次序）。挖矿改后台运行后，主 goroutine 与全节点路径
		// 共用同一 shutdown 等待；原信号桥接 goroutine 不再需要（主 select 直接
		// 消费 sigCh，P2-SIGTERM 修复语义保持：信号必然触发关闭）。
		if err := rt.svc.minerLife.Load().start(nf.maxBlocks); err != nil {
			// boot 时状态恒为 STOPPED，此处仅防御性分支（理论上不可达）。
			log.Printf("[node] 启动挖矿失败: %v", err)
			return
		}
		select {
		case <-sigCh:
			log.Printf("[node] 收到退出信号，正在关闭（释放数据目录锁）...")
		case <-stopCh:
			log.Printf("[node] 收到停止请求，正在关闭（释放数据目录锁）...")
		}
		// 先停挖并等待挖矿 goroutine 退出，再由最外层 defer 执行 rt.Close()
		// （store 关闭不与在途 AddBlock 并发，STOP-INV-05；在途块自然完成——
		// MINING-LIFECYCLE 设计冻结 §10 语义）。
		rt.svc.minerLife.Load().beginShutdown()
		log.Printf("[node] 正在关闭（释放数据目录锁）...")
		return
	}
	log.Printf("[node] 以全节点模式运行（未启用挖矿）；可用 `node stop` 停止节点")
	select {
	case <-sigCh:
		log.Printf("[node] 收到退出信号，正在关闭（释放数据目录锁）...")
	case <-stopCh:
		log.Printf("[node] 收到停止请求，正在关闭（释放数据目录锁）...")
	}
	// PHASE MINING-LIFECYCLE-1：runtime START 的挖矿循环可能仍在后台运行
	// （控制面 POST /mine/start）。停机关闭链开始前同样先停挖并等待退出，
	// 保证 rt.Close() 与 AddBlock 互斥（STOP-INV-05）。幂等：未挖矿时为零开销。
	rt.svc.minerLife.Load().beginShutdown()
	// 落到这里后由最外层 defer 执行 rt.Close()：
	// 控制接口 → P2P → 区块存储 → 数据目录锁，顺序固定且幂等。
}

// openConsolePage 打印并尝试打开 Developer Console 页面（`node ui`）。
//
// 失败不致命：即使系统没有可用浏览器，用户仍可按打印出的地址手动访问。
func openConsolePage(addr string) {
	if addr == "" {
		return
	}
	url := "http://" + addr + "/"
	log.Printf("[ui] Developer Console 已就绪: %s", url)
	if err := openBrowser(url); err != nil {
		log.Printf("[ui] 自动打开浏览器失败，请手动访问上面的地址: %v", err)
	}
}

// splitSeeds 解析逗号分隔的种子地址列表，忽略空项与空白。
func splitSeeds(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// isLoopback 判断监听地址是否为本机回环（控制接口的默认安全边界）。
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return host == "localhost" || strings.HasPrefix(host, "127.") || host == "::1"
}

// 挖矿重试/退避参数（PHASE MINING-REMEDIATION-1）。
//
// 设计意图：修复前 mineOnce 返回裸 bool，上层对任何失败都「立即以同一高度重试」，
// 在「补贴耗尽且无手续费交易」时退化为 6 核热循环 + 5 GB/天日志（P1-MINING-001）。
// 下列参数把「重试」变成有界、有分类、有可观测性的行为。
const (
	// stallRecheckInterval 是 STALLED（政策终态）下的复查间隔。
	// STALLED 不执行任何 PoW，只按该间隔重新评估「是否已出现合法候选」
	// （例如带手续费交易已到达，或链尾已推进）。每秒一次的唤醒开销可忽略，
	// 既不会忙等，也不会错过外部事件。
	stallRecheckInterval = time.Second
	// retryBackoffInitial / retryBackoffMax 是「无法归类的可重试失败」的有界指数退避。
	retryBackoffInitial = 50 * time.Millisecond
	retryBackoffMax     = 2 * time.Second
	// maxConsecutiveOperationalFailures 是连续不可归类失败的升级阈值：
	// 超过即进入 FAILED —— 保证「不得无限循环」。
	maxConsecutiveOperationalFailures = 12
	// maxConsecutiveStaleRetries 是连续「链尾变化」重试的守护阈值。
	// 正常的分叉竞争远达不到该值；达到说明存在未预见的循环，转入有界退避。
	// 注意：**不升级为 FAILED** —— 链尾变化本身是合法情形，不是缺陷。
	maxConsecutiveStaleRetries = 1000
)

// runMiner 持续挖矿：组装候选区块 → **模板预校验** → 挖矿（可被链尾变化中断）→ 上链 → 广播。
// maxBlocks > 0 时挖满该数量后停止挖矿，转为普通全节点继续运行（直到收到停止请求）。
//
// stop 是停止请求通道：收到后不再组装新的候选区块，函数返回，
// 由调用方走统一的关闭链。已经在途的 AddBlock 会自然跑完——
// 绝不在写盘中途中断，这是 STOP-INV-05（stop 不损坏 blocks.dat）的保证。
//
// 状态机（PHASE MINING-REMEDIATION-1）：循环不再以「worker goroutine 是否存在」
// 判定运行状态，而是把 mineOnce 的**分类结果**映射为显式语义状态
// （见 mining_state.go）。因此「无意义 PoW 热循环」在结构上不可能再出现：
// 任何不可挖的情形都会落到 STALLED（政策终态）或 FAILED（结构性错误），二者都不执行 PoW。
func runMiner(svc *nodeService, maxBlocks int, stop <-chan struct{}) {
	svc.mining.Store(true)
	svc.startHeight.Store(int64(svc.chain.Height()))
	svc.setMineState(MiningStarting, "enabled")
	log.Printf("[miner] 挖矿已启用，地址=%s 上限=%s", svc.miner.Address(), blocksLimitText(maxBlocks))

	mined := 0
	consecutiveStale := 0
	consecutiveOperational := 0

	// finish 统一收尾：关闭挖矿标志并落到终态（不干扰已在途的写盘）。
	finish := func(state miningState, reason string) {
		svc.mining.Store(false)
		svc.setMineState(state, reason)
	}

	for {
		select {
		case <-stop:
			finish(MiningStopped, "stopped")
			return
		default:
		}

		if maxBlocks > 0 && mined >= maxBlocks {
			log.Printf("[miner] 已达到挖矿上限 %d 个区块（当前高度 %d），转为全节点模式", maxBlocks, svc.chain.Height())
			finish(MiningStopped, "max-blocks-reached")
			<-stop // 继续运行直到被要求停止
			return
		}

		switch mineOnce(svc, stop) {
		case mineOutcomeMined:
			mined++
			consecutiveStale, consecutiveOperational = 0, 0

		case mineOutcomeStalled:
			consecutiveStale, consecutiveOperational = 0, 0
			// 政策终态：不执行 PoW、不视为故障。
			// 等待「停止请求 / 链尾变化 / 复查超时」，以便带手续费交易到达或链尾推进后重新评估。
			if waitStalledRecheck(svc, stop, stallRecheckInterval) {
				finish(MiningStopped, "stopped")
				return
			}

		case mineOutcomeStop:
			finish(MiningStopped, "stopped")
			return

		case mineOutcomeStructFail:
			// 结构性错误无法通过重试恢复：终止挖矿（不自动重启），保持进程以全节点语义运行。
			log.Printf("[miner] 挖矿已终止（FAILED）：模板结构性错误，需修复后重启节点")
			finish(MiningFailed, "structural-error")
			<-stop
			return

		case mineOutcomeStale:
			consecutiveOperational = 0
			consecutiveStale++
			// 链尾变化属合法重试：立即重建模板（预校验已挡住陈旧模板，故不烧 PoW）。
			if consecutiveStale > maxConsecutiveStaleRetries {
				consecutiveStale = 0
				if sleepOrStop(stop, retryBackoffMax) {
					finish(MiningStopped, "stopped")
					return
				}
			}

		case mineOutcomeBackoff:
			consecutiveStale = 0
			consecutiveOperational++
			if consecutiveOperational > maxConsecutiveOperationalFailures {
				log.Printf("[miner] 连续 %d 次不可归类的挖矿失败，挖矿终止（FAILED）", consecutiveOperational)
				finish(MiningFailed, "operational-failures-exceeded")
				<-stop
				return
			}
			if sleepOrStop(stop, backoffDuration(consecutiveOperational)) {
				finish(MiningStopped, "stopped")
				return
			}
		}
	}
}

// backoffDuration 返回第 n 次连续失败的有界指数退避时长（上限 retryBackoffMax）。
func backoffDuration(n int) time.Duration {
	d := retryBackoffInitial
	for i := 1; i < n; i++ {
		d *= 2
		if d >= retryBackoffMax {
			return retryBackoffMax
		}
	}
	if d > retryBackoffMax {
		return retryBackoffMax
	}
	return d
}

// sleepOrStop 睡眠 d；若期间收到停止请求则立即返回 true。
func sleepOrStop(stop <-chan struct{}, d time.Duration) bool {
	if d <= 0 {
		return false
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-stop:
		return true
	case <-t.C:
		return false
	}
}

// waitStalledRecheck 在 STALLED（政策终态）下等待「停止请求 / 链尾变化 / 复查超时」三者之一。
// 返回 true 表示应停止挖矿。
//
// 作用：在**不执行任何 PoW** 的前提下，既不会忙等（每秒一次唤醒），
// 也能被「链尾变化」立即唤醒（例如对端区块到达）。
func waitStalledRecheck(svc *nodeService, stop <-chan struct{}, d time.Duration) (stopRequested bool) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-stop:
		return true
	case <-svc.tipChanged:
		return false
	case <-t.C:
		return false
	}
}

// blocksLimitText 把挖矿上限格式化为可读文本。
func blocksLimitText(max int) string {
	if max <= 0 {
		return "不限"
	}
	return fmt.Sprintf("%d 个区块", max)
}

// mineOnce 尝试挖出一个区块，返回**分类结果**（见 mineOutcome）。
//
// 关键顺序（PHASE MINING-REMEDIATION-1）：
//
//	构造模板 → 模板预校验（跳过 PoW 的完整共识校验） → PoW → 上链
//
// 预校验保证「无效模板 ⇒ 0 次 PoW 尝试」，而不是先烧完整套 PoW 再被拒绝。
// 修复前本函数返回裸 bool，把「链尾变化 / 收到停止 / 结构性拒绝」压成同一个 false，
// 上层因而对所有 false 都立即重试 —— 那正是无意义 PoW 热循环（P1-MINING-001）的来源。
//
// 全程持有 mineMu：持续挖矿循环与「按需出块」两类入口必须串行，
// 否则同一高度会有两个候选区块在求解，先出块的会白烧 CPU。
//
// stop 为 nil 时该中断源不存在（按需出块场景），nil channel 在 select 中永不就绪。
// 停止只取消「求解过程」，不取消已经开始的写盘：求解成功后的 AddBlock 照常跑完。
func mineOnce(svc *nodeService, stop <-chan struct{}) mineOutcome {
	svc.mineMu.Lock()
	defer svc.mineMu.Unlock()

	// 先丢弃上一轮遗留的中断信号：否则刚组装完候选区块就会被过期信号打断
	// （表现为「刚读完链尾就立刻放弃」，虽然功能正确但白白浪费一轮）。
	drainTipChanged(svc)

	tip, err := svc.chain.Tip()
	if err != nil {
		log.Printf("[miner] 读取链尾失败: %v", err)
		svc.setMineState(MiningRunning, "tip-read-failed")
		return mineOutcomeBackoff
	}
	height := svc.chain.Height() + 1

	pending := svc.pool.Pending(MaxBlockTxs)
	fees := svc.pool.TotalFees(pending)

	subsidy := utxo.Subsidy(height)
	coinbase := transaction.NewCoinbaseTx(svc.miner.PubKeyHash(), subsidy+fees, height)
	txs := make([]*transaction.Transaction, 0, len(pending)+1)
	txs = append(txs, coinbase)
	txs = append(txs, pending...)

	candidate := block.NewCandidateBlock(tip.Header.Hash(), svc.chain.CurrentBits(), txs)

	// 难度共识硬分叉（PHASE DIFFICULTY-CONSENSUS-IMPLEMENTATION-1）：
	// 候选块版本与时间戳必须与该高度激活的规则一致，否则 validateTemplate/AddBlock
	// 会以 ErrInvalidVersion / ErrTimestampOutOfRange 拒绝（硬分叉强制）。
	candidate.Header.Version = svc.chain.RequiredVersionFor(height)
	candidate.Header.Timestamp = svc.chain.MiningTimestamp(height)

	// ---- 模板预校验：跳过 PoW，其余共识规则全部执行 ----
	//
	// 与 AddBlock 使用**同一份**规则（blockchain.ValidateTemplate → validateBlock(b, true)），
	// 不存在「两套验证规则」的漂移风险；这里刻意不自行复算共识结论，
	// 而是把「模板是否合法」的判定完全交给共享校验器。
	if err := svc.chain.ValidateTemplate(candidate); err != nil {
		// 分类决策完全交给纯函数（见 mining_state.go 的 classifyTemplateFailure），
		// 本处只负责把分类落成状态、日志与返回值。
		switch classifyTemplateFailure(err, subsidy+fees) {
		case classTemplateStale:
			// B/C 类：链/链尾已变化（模板本身并不非法）—— 重建模板重试即可。
			svc.mineRetries.Add(1)
			svc.setMineState(MiningRunning, "template-refresh")
			log.Printf("[miner] MINING_TEMPLATE_REFRESH 高度=%d 原因=%v（链/链尾已变化，重建模板后重试，不执行 PoW）", height, err)
			return mineOutcomeStale

		case classTemplatePolicyTerminal:
			// D 类：政策终态（A1 裁决：Subsidy 可达 0）。
			//
			// 该高度**不存在任何合法候选区块**，由两条既有共识规则合取而得：
			//   ① coinbase 必须至少有一个输出且每个输出金额 != 0  ⇒  总额 >= 1
			//      （utxo.ValidateCoinbaseStructure，apply.go:81-88）
			//   ② coinbase 输出总额 <= Subsidy(height)+fees          ⇒  总额 <= 0
			//      （utxo.ApplyBlock，apply.go:258）
			// ①与②互斥 ⇒ 不可满足。这是**政策终态，不是缺陷**：
			// 不执行 PoW，状态记为 STALLED；一旦有带手续费交易到达即自动恢复。
			//
			// 注意与 subsidy==0 && fees>0 严格区分：后者 coinbase = fees > 0，仍可正常出块。
			if svc.setMineState(MiningStalled, "subsidy-exhausted-no-fee-tx") {
				log.Printf("[miner] MINING_STALLED 高度=%d 补贴=%d 手续费=%d 原因=subsidy-exhausted-no-fee-tx：该高度不存在合法候选区块，不执行 PoW",
					height, subsidy, fees)
			}
			return mineOutcomeStalled

		default:
			// A 类（classTemplateStructural）：结构性错误 —— 无法通过重试恢复，
			// 挖矿终止（FAILED）。default 而非具名 case 是为了让「出现未分类的
			// 新分类值」也落到最保守的处置上（宁可停机，也不无限烧 PoW）。
			svc.setMineState(MiningFailed, "structural-error")
			log.Printf("[miner] MINING_TEMPLATE_REJECTED 高度=%d 原因=%v（结构性错误：模板不满足共识要求，不执行 PoW）", height, err)
			return mineOutcomeStructFail
		}
	}

	svc.setMineState(MiningRunning, "pow")
	log.Printf("[miner] MINING_POW_START 高度=%d 打包交易=%d 手续费=%d 难度位=%d",
		height, len(pending), fees, candidate.Header.Bits)

	// 挖矿在工作 goroutine 中进行，主 goroutine 监听「链尾变化」信号；
	// 一旦链尾变化就关闭 cancel 让 worker 立即退出并等待收尾——
	// 不能只丢下不管：并行 worker 会一直算到命中，既白烧 CPU 也会泄漏 goroutine。
	done := make(chan struct{})
	cancel := make(chan struct{})
	var (
		found    bool
		attempts uint64
	)
	go func() {
		defer close(done)
		// 返回值必须被接收：attempts 是「PoW 尝试次数」可观测性的来源；
		// found 用于区分「命中」与「被取消」。
		found, attempts = pow.MineCancelable(candidate, svc.miners, cancel)
	}()

	select {
	case <-svc.tipChanged:
		close(cancel)
		<-done // 等 worker 全部退出，确保没有后台 goroutine 继续持有这个候选区块
		svc.powAttempts.Add(attempts)
		svc.mineRetries.Add(1)
		svc.setMineState(MiningRunning, "tip-changed-retry")
		log.Printf("[miner] MINING_TEMPLATE_STALE 高度=%d PoW尝试=%d 原因=链尾已变化，放弃当前候选区块", height, attempts)
		return mineOutcomeStale
	case <-stop:
		svc.setMineState(MiningStopping, "stop-requested")
		close(cancel)
		<-done
		svc.powAttempts.Add(attempts)
		log.Printf("[miner] 收到停止请求，放弃当前候选区块（高度=%d）", height)
		return mineOutcomeStop
	case <-done:
		svc.powAttempts.Add(attempts)
		if !found {
			// 理论不可达：未发出取消信号时求解必然以命中结束。防御性处理。
			log.Printf("[miner] 求解未命中且未收到中断信号（高度=%d），按可重试失败处理", height)
			svc.mineRetries.Add(1)
			return mineOutcomeBackoff
		}
	}

	if err := svc.chain.AddBlock(candidate); err != nil {
		// 不变式：预校验已通过且 PoW 已命中 ⇒ 此处失败只可能是链尾在求解期间被替换。
		// 因此不再输出「链尾可能已变化」这种模糊措辞，而是明确归类。
		svc.rejectedBlocks.Add(1)
		svc.mineRetries.Add(1)
		svc.setMineState(MiningRunning, "block-orphaned-retry")
		log.Printf("[miner] MINING_TEMPLATE_STALE 高度=%d 原因=链尾在求解期间变化（该区块作废）: %v", height, err)
		return mineOutcomeStale
	}
	// PHASE E-IMPLEMENTATION-A（O-02/B-1）：上链后的收尾（交易池同步 / 计数 / 广播 /
	// 统一解析入口级联）统一由 commitMinedBlock 承担，本地挖矿不再绕过解析契约。
	svc.commitMinedBlock(candidate, height)
	svc.setMineState(MiningRunning, "mined")
	log.Printf("[miner] MINING_BLOCK_ACCEPTED 高度=%d 哈希=%s 交易数=%d",
		svc.chain.Height(), candidate.Header.HashHex(), len(candidate.Transactions))
	return mineOutcomeMined
}

// defaultDataDir 返回默认数据目录。
func defaultDataDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".p2pchain")
	}
	return ".p2pchain"
}

// splitCommandArgs 把 argv 拆成「子命令 + 该子命令的参数」。
//
// 返回 cmd == "" 表示本次应当启动节点（args 原样返回）。
//
// 为什么需要它（PHASE PRODUCT-DEV-1C.0，P1）：
//
//	node -datadir X verify
//
// 过去 main 只看 argv[0] 是否以 "-" 开头：这里是 "-datadir"，于是直接 runNode；
// flag 包解析到 "verify" 时停止并把它当作位置参数忽略掉，结果「看起来是离线
// 只读校验」的命令实际启动了完整节点——建链、建钱包、取数据目录锁、监听控制
// 接口与 P2P 端口，并永久阻塞。这同时违背了 cmdVerify 自述的
// 「只读、绝不写回、绝不取锁」契约。
//
// 约定（§4）：**参数排列不能改变 command identity。**
// 只要 argv 中存在子命令（且它不位于某个选项的值位置），就必须执行该子命令；
// 位于它之前的、显式设置过的选项，按原值搬到子命令之后。
// 组合本身无意义时（例如 node -mine verify）由子命令自己的 flag 集报 usage
// 错误并退出码 2——绝不静默退化成「启动节点」（INV-05）。
func splitCommandArgs(argv []string) (cmd string, args []string) {
	if len(argv) == 0 {
		return "", argv
	}
	// 既有形式：子命令就在首位，零变更地走原路径。
	if !strings.HasPrefix(argv[0], "-") {
		return argv[0], argv[1:]
	}
	// 选项在前：用与 startNode 完全相同的选项集先解析一遍。flag 包会在第一个
	// 非选项 token 处停下，Args() 即「选项结束之后的剩余位置参数」。
	// 预扫描不产生任何副作用：不读数据目录、不建文件、不监听端口。
	fs, _ := newNodeFlagSet(flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(argv); err != nil {
		return "", argv // 解析失败：交回 startNode，按既有行为打印用法并退出
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return "", argv // 只有选项，没有位置参数：就是启动节点（INV-06）
	}
	// 把显式设置过的选项搬到子命令之后。
	// 统一写 -name=value：布尔选项写成 "-mine true" 会被 flag 包当成两个 token
	// （-mine 后面跟一个位置参数），而 -name=value 对所有类型都成立。
	prefix := make([]string, 0, 4)
	fs.Visit(func(f *flag.Flag) {
		prefix = append(prefix, "-"+f.Name+"="+f.Value.String())
	})
	out := make([]string, 0, len(prefix)+len(rest)-1)
	out = append(out, prefix...)
	out = append(out, rest[1:]...)
	return rest[0], out
}

func main() {
	argv := os.Args[1:]
	cmd, args := splitCommandArgs(argv)
	if cmd != "" {
		code, ok := runCLI(cmd, args, os.Stdout, os.Stderr)
		if !ok {
			fmt.Fprintf(os.Stderr, "错误: 未知子命令 %q\n\n%s", cmd, usageText)
			os.Exit(2)
		}
		os.Exit(code)
	}
	runNode(argv)
}
