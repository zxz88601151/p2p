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
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
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
	ListenAddr string
	RPCAddr    string
	SeedAddr   string
	DataDir    string
	Mine       bool
	MaxBlocks  int
}

// nodeRuntime 一个已启动节点的全部运行时组件。
// 抽出来是为了让端到端测试能以进程内方式启动真实节点（同样的组装路径），
// 而不是把 main 里的逻辑复制一份到测试里——测试必须跑在被测代码上。
type nodeRuntime struct {
	store *storage.FileBlockStore
	chain *blockchain.Blockchain
	pool  *mempool.Mempool
	svc   *nodeService
	p2p   *p2p.Node
	ctl   *control.Server
}

// newNodeRuntime 按配置组装并启动节点（P2P 监听 + 控制接口）。
// 返回后节点已可接受连接与查询；不启动挖矿（由调用方决定）。
func newNodeRuntime(cfg nodeConfig) (*nodeRuntime, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	store, err := storage.OpenFileBlockStore(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("打开区块存储失败: %w", err)
	}

	chain, err := blockchain.NewBlockchainFromStore(store)
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("加载区块链失败: %w", err)
	}
	tip, err := chain.Tip()
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("读取链尾失败: %w", err)
	}
	log.Printf("[node] 本地区块链已就绪: 高度=%d 链尾=%s 数据文件=%s",
		chain.Height(), tip.Header.HashHex(), store.FilePath())

	walletPath := filepath.Join(cfg.DataDir, "wallet.json")
	nodeWallet, created, err := wallet.LoadOrCreate(walletPath)
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("加载/创建钱包失败: %w", err)
	}
	if created {
		log.Printf("[node] 已生成新钱包: %s", walletPath)
	}
	log.Printf("[node] 节点钱包地址=%s", nodeWallet.Address())

	pool := mempool.New(MempoolSize)
	svc := newNodeService(chain, pool, nodeWallet)

	genesis, err := chain.BlockByHeight(0)
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("读取创世区块失败: %w", err)
	}
	p2pNode := p2p.NewNode(cfg.ListenAddr, nodeWallet.Address(), genesis.Header.HashHex(), svc)
	svc.net = p2pNode
	p2pNode.SetHeightProvider(func() int { return chain.Height() })

	go func() {
		if err := p2pNode.Start(); err != nil {
			log.Printf("[p2p] 监听结束: %v", err)
		}
	}()

	ctl := control.NewServer(svc)
	actualRPC, err := ctl.Start(cfg.RPCAddr)
	if err != nil {
		p2pNode.Stop()
		_ = store.Close()
		return nil, fmt.Errorf("启动控制接口失败: %w", err)
	}
	if !isLoopback(actualRPC) {
		log.Printf("[node] 警告：控制接口监听在 %s，非回环地址且无鉴权，请勿暴露到不可信网络", actualRPC)
	}
	log.Printf("[node] 控制接口已启动: %s", actualRPC)

	if cfg.SeedAddr != "" {
		if err := p2pNode.ConnectToPeer(cfg.SeedAddr); err != nil {
			log.Printf("[p2p] 连接种子节点失败: %v", err)
		}
	}

	return &nodeRuntime{store: store, chain: chain, pool: pool, svc: svc, p2p: p2pNode, ctl: ctl}, nil
}

// Close 关闭节点持有的全部资源。
func (rt *nodeRuntime) Close() {
	if rt.ctl != nil {
		_ = rt.ctl.Stop()
	}
	if rt.p2p != nil {
		rt.p2p.Stop()
	}
	if rt.store != nil {
		_ = rt.store.Close()
	}
}

// runNode 启动一个完整节点（P2P + 控制接口 + 可选挖矿），阻塞至进程退出。
func runNode(args []string) {
	fs := flag.NewFlagSet("node", flag.ExitOnError)
	listenAddr := fs.String("listen", ":6688", "本节点监听地址")
	rpcAddr := fs.String("rpc", control.DefaultAddr, "控制接口监听地址（仅本机，无鉴权）")
	seedAddr := fs.String("seed", "", "种子节点地址，留空表示作为第一个节点启动")
	dataDir := fs.String("datadir", defaultDataDir(), "数据目录（存放区块数据与钱包）")
	mine := fs.Bool("mine", false, "是否启用挖矿")
	maxBlocks := fs.Int("maxblocks", 0, "挖矿最多产出多少个区块，0 表示不限（用于测试网可控出块）")
	_ = fs.Parse(args)

	rt, err := newNodeRuntime(nodeConfig{
		ListenAddr: *listenAddr,
		RPCAddr:    *rpcAddr,
		SeedAddr:   *seedAddr,
		DataDir:    *dataDir,
		Mine:       *mine,
		MaxBlocks:  *maxBlocks,
	})
	if err != nil {
		log.Fatalf("[node] %v", err)
	}
	defer rt.Close()

	if *mine {
		runMiner(rt.svc, *maxBlocks)
		return
	}
	log.Printf("[node] 以全节点模式运行（未启用挖矿）；可用 `node status` 查看状态")
	select {}
}

// isLoopback 判断监听地址是否为本机回环（控制接口的默认安全边界）。
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return host == "localhost" || strings.HasPrefix(host, "127.") || host == "::1"
}

// runMiner 持续挖矿：组装候选区块 → 挖矿（可被链尾变化中断）→ 上链 → 广播。
// maxBlocks > 0 时挖满该数量后停止挖矿，转为普通全节点继续运行。
func runMiner(svc *nodeService, maxBlocks int) {
	svc.mining.Store(true)
	log.Printf("[miner] 挖矿已启用，地址=%s 上限=%s", svc.miner.Address(), blocksLimitText(maxBlocks))
	mined := 0
	for {
		if maxBlocks > 0 && mined >= maxBlocks {
			svc.mining.Store(false)
			log.Printf("[miner] 已达到挖矿上限 %d 个区块（当前高度 %d），转为全节点模式", maxBlocks, svc.chain.Height())
			select {}
		}
		if mineOnce(svc) {
			mined++
		}
	}
}

// blocksLimitText 把挖矿上限格式化为可读文本。
func blocksLimitText(max int) string {
	if max <= 0 {
		return "不限"
	}
	return fmt.Sprintf("%d 个区块", max)
}

// mineOnce 尝试挖出一个区块。返回 false 表示因链尾变化被中断，需要重新组装。
//
// 全程持有 mineMu：持续挖矿循环与「按需出块」两类入口必须串行，
// 否则同一高度会有两个候选区块在求解，先出块的会白烧 CPU。
func mineOnce(svc *nodeService) bool {
	svc.mineMu.Lock()
	defer svc.mineMu.Unlock()

	// 先丢弃上一轮遗留的中断信号：否则刚组装完候选区块就会被过期信号打断
	// （表现为「刚读完链尾就立刻放弃」，虽然功能正确但白白浪费一轮）。
	drainTipChanged(svc)

	tip, err := svc.chain.Tip()
	if err != nil {
		log.Printf("[miner] 读取链尾失败: %v", err)
		time.Sleep(time.Second)
		return false
	}
	height := svc.chain.Height() + 1

	pending := svc.pool.Pending(MaxBlockTxs)
	fees := svc.pool.TotalFees(pending)

	coinbase := transaction.NewCoinbaseTx(svc.miner.PubKeyHash(), utxo.Subsidy(height)+fees, height)
	txs := make([]*transaction.Transaction, 0, len(pending)+1)
	txs = append(txs, coinbase)
	txs = append(txs, pending...)

	candidate := block.NewCandidateBlock(tip.Header.Hash(), svc.chain.CurrentBits(), txs)
	log.Printf("[miner] 开始挖矿: 高度=%d 打包交易=%d 手续费=%d 难度位=%d",
		height, len(pending), fees, candidate.Header.Bits)

	// 挖矿在工作 goroutine 中进行，主 goroutine 监听「链尾变化」信号并可即时放弃本轮，
	// 避免长时间阻塞在 Pow 计算里、持有过期候选区块。
	done := make(chan struct{})
	go func() {
		defer close(done)
		pow.Mine(candidate, 0) // 0 表示不设迭代上限，直到找到解
	}()

	select {
	case <-svc.tipChanged:
		log.Printf("[miner] 链尾已变化，放弃当前候选区块（高度=%d）", height)
		// 丢弃本轮结果：候选区块的父块已不是链尾，即使挖到也无法上链
		go func() { <-done }()
		return false
	case <-done:
	}

	if err := svc.chain.AddBlock(candidate); err != nil {
		log.Printf("[miner] 挖到的区块被拒绝（链尾可能已变化）: %v", err)
		return false
	}
	svc.pool.RemoveIncluded(candidate, svc.chain.UTXOSnapshot(), height)
	log.Printf("[miner] 挖到新区块: 高度=%d 哈希=%s 交易数=%d",
		svc.chain.Height(), candidate.Header.HashHex(), len(candidate.Transactions))
	svc.broadcastBlock(candidate)
	return true
}

// defaultDataDir 返回默认数据目录。
func defaultDataDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".p2pchain")
	}
	return ".p2pchain"
}

func main() {
	// 首参数为非选项时按子命令处理：识别不了就报错退出，
	// 避免把拼错的子命令当成「启动节点」而误导用户。
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		code, ok := runCLI(os.Args[1], os.Args[2:], os.Stdout, os.Stderr)
		if !ok {
			fmt.Fprintf(os.Stderr, "错误: 未知子命令 %q\n\n%s", os.Args[1], usageText)
			os.Exit(2)
		}
		os.Exit(code)
	}
	runNode(os.Args[1:])
}
