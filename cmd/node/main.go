// cmd/node 是节点的可执行程序入口。
//
// 运行流程：
//  1. 打开（或初始化）持久化区块存储，加载/回放本地区块链
//  2. 生成或加载矿工钱包
//  3. 启动 P2P 监听、连接种子节点
//  4. 矿工模式下进入挖矿循环；全节点模式下仅转发与同步
//
// main.go 只负责组装依赖与解析参数；业务语义在 service.go。
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
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

// runNode 启动一个完整节点（P2P + 可选挖矿），阻塞至进程退出。
func runNode(args []string) {
	fs := flag.NewFlagSet("node", flag.ExitOnError)
	listenAddr := fs.String("listen", ":6688", "本节点监听地址")
	seedAddr := fs.String("seed", "", "种子节点地址，留空表示作为第一个节点启动")
	dataDir := fs.String("datadir", defaultDataDir(), "数据目录（存放区块数据与钱包）")
	mine := fs.Bool("mine", false, "是否启用挖矿")
	_ = fs.Parse(args)

	if err := os.MkdirAll(*dataDir, 0o700); err != nil {
		log.Fatalf("[node] 创建数据目录失败: %v", err)
	}

	// 1. 持久化区块存储 + 链加载（带完整校验回放）
	store, err := storage.OpenFileBlockStore(*dataDir)
	if err != nil {
		log.Fatalf("[node] 打开区块存储失败: %v", err)
	}
	defer func() { _ = store.Close() }()

	chain, err := blockchain.NewBlockchainFromStore(store)
	if err != nil {
		log.Fatalf("[node] 加载区块链失败: %v", err)
	}
	tip, err := chain.Tip()
	if err != nil {
		log.Fatalf("[node] 读取链尾失败: %v", err)
	}
	log.Printf("[node] 本地区块链已就绪: 高度=%d 链尾=%s 数据文件=%s",
		chain.Height(), tip.Header.HashHex(), store.FilePath())

	// 2. 矿工钱包（本地生成并落盘，权限 0600）
	walletPath := filepath.Join(*dataDir, "wallet.json")
	minerWallet, created, err := wallet.LoadOrCreate(walletPath)
	if err != nil {
		log.Fatalf("[node] 加载/创建钱包失败: %v", err)
	}
	if created {
		log.Printf("[node] 已生成新钱包: %s", walletPath)
	}
	log.Printf("[node] 矿工地址=%s", minerWallet.Address())

	// 3. 组装服务并启动 P2P
	pool := mempool.New(MempoolSize)
	svc := newNodeService(chain, pool, minerWallet)

	genesis, err := chain.BlockByHeight(0)
	if err != nil {
		log.Fatalf("[node] 读取创世区块失败: %v", err)
	}
	p2pNode := p2p.NewNode(*listenAddr, nodeID(minerWallet), genesis.Header.HashHex(), svc)
	svc.net = p2pNode
	p2pNode.SetHeightProvider(func() int { return chain.Height() })

	go func() {
		if err := p2pNode.Start(); err != nil {
			log.Fatalf("[p2p] 启动监听失败: %v", err)
		}
	}()
	defer p2pNode.Stop()

	if *seedAddr != "" {
		if err := p2pNode.ConnectToPeer(*seedAddr); err != nil {
			log.Printf("[p2p] 连接种子节点失败: %v", err)
		}
	}

	// 4. 挖矿或纯转发
	if *mine {
		runMiner(svc)
		return
	}
	log.Printf("[node] 以全节点模式运行（未启用挖矿）")
	select {}
}

// runMiner 持续挖矿：组装候选区块 → 挖矿（可被链尾变化中断）→ 上链 → 广播。
func runMiner(svc *nodeService) {
	log.Printf("[miner] 挖矿已启用，地址=%s", svc.miner.Address())
	for {
		if !mineOnce(svc) {
			// 被中断（对端先挖出了区块）：立即重新组装候选区块
			continue
		}
	}
}

// mineOnce 尝试挖出一个区块。返回 false 表示因链尾变化被中断，需要重新组装。
func mineOnce(svc *nodeService) bool {
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

// nodeID 由钱包地址派生，仅用于日志与网络识别。
func nodeID(w *wallet.Wallet) string { return w.Address() }

// defaultDataDir 返回默认数据目录。
func defaultDataDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".p2pchain")
	}
	return ".p2pchain"
}

func main() {
	runNode(os.Args[1:])
}
