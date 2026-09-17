// nodeapi.go 实现 internal/control.Node 接口：把控制接口的请求翻译为
// 对本地区块链、交易池与钱包的操作。
//
// 高度语义统一说明：所有「能否花费 / 是否成熟」的判断都以 nextHeight = 链高+1
// 为基准，即「现在构造的交易会被打包进的那一个高度」，与挖矿组装候选区块时
// 使用的高度完全一致，避免余额查询与转账结果对不上。
package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"

	"p2pchain/internal/block"
	"p2pchain/internal/control"
	"p2pchain/internal/pow"
	"p2pchain/internal/txbuild"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

// 编译期断言：节点服务必须实现控制接口所需的全部能力。
var _ control.Node = (*nodeService)(nil)

// nextHeight 返回下一区块的高度。
func (s *nodeService) nextHeight() int { return s.chain.Height() + 1 }

// Status 返回节点运行状态。
func (s *nodeService) Status() (control.StatusInfo, error) {
	tip, err := s.chain.Tip()
	if err != nil {
		return control.StatusInfo{}, err
	}
	bits := s.chain.CurrentBits()
	state, reason := s.miningStateSnapshot()
	return control.StatusInfo{
		Height:      s.chain.Height(),
		TipHash:     tip.Header.HashHex(),
		Peers:       s.net.PeerAddrs(),
		MempoolSize: s.pool.Len(),
		Mining:      s.mining.Load(),
		Address:     s.miner.Address(),
		Bits:        bits,
		Difficulty:  relativeDifficulty(bits),

		// 挖矿运行时语义状态（PHASE MINING-REMEDIATION-1）。
		// MiningState 是权威字段：它区分「真的在对合法模板执行 PoW」（RUNNING）
		// 与「不存在合法候选、未执行任何 PoW」（STALLED）以及「结构性错误」（FAILED）。
		MiningState:    string(state),
		MiningReason:   reason,
		PowAttempts:    s.powAttempts.Load(),
		MiningRetries:  s.mineRetries.Load(),
		AcceptedBlocks: s.acceptedBlocks.Load(),
		RejectedBlocks: s.rejectedBlocks.Load(),
	}, nil
}

// relativeDifficulty 把难度位（前导零位数）换算为「相对最低难度的倍数」。
//
// 依据 pow.BitsToTarget：target = 1 << (256-bits)，而 pow.MaxTargetBits 是最低难度，
// 因此 倍数 = MaxTarget / Target = 2^(bits-MaxTargetBits)。
// 这是纯展示用的派生量，不参与任何共识判断——共识始终以 Bits 为准。
// bits-MaxTargetBits <= 53 时 float64 可精确表示（本项目难度范围远小于此）。
func relativeDifficulty(bits uint32) float64 {
	if bits < pow.MaxTargetBits {
		return 0
	}
	return math.Pow(2, float64(bits-pow.MaxTargetBits))
}

// Balance 查询地址余额。
func (s *nodeService) Balance(address string) (control.BalanceInfo, error) {
	hash, err := wallet.DecodeAddress(address)
	if err != nil {
		return control.BalanceInfo{}, fmt.Errorf("地址非法: %w", err)
	}
	snap := s.chain.UTXOSnapshot()
	height := s.nextHeight()

	count := 0
	for _, e := range snap.AllEntries() {
		if e.PubKeyHash == hash {
			count++
		}
	}
	return control.BalanceInfo{
		Address:   address,
		Spendable: snap.Balance(hash, height, false),
		Total:     snap.Balance(hash, height, true),
		UTXOCount: count,
		Height:    s.chain.Height(),
	}, nil
}

// UTXOs 列出某地址的未花费输出（按高度、OutPoint 排序保证输出稳定）。
func (s *nodeService) UTXOs(address string) ([]control.UTXOInfo, error) {
	hash, err := wallet.DecodeAddress(address)
	if err != nil {
		return nil, fmt.Errorf("地址非法: %w", err)
	}
	snap := s.chain.UTXOSnapshot()
	height := s.nextHeight()

	out := make([]control.UTXOInfo, 0)
	for op, e := range snap.AllEntries() {
		if e.PubKeyHash != hash {
			continue
		}
		mature := !e.IsCoinbase || height-e.Height >= utxo.CoinbaseMaturity
		out = append(out, control.UTXOInfo{
			OutPoint:   fmt.Sprintf("%x:%d", op.Hash, op.Index),
			Value:      e.Value,
			Height:     e.Height,
			IsCoinbase: e.IsCoinbase,
			Mature:     mature,
		})
	}
	sortUTXOs(out)
	return out, nil
}

// Send 用节点钱包构造、签名、入池并广播一笔支付交易。
func (s *nodeService) Send(to string, amount, fee uint64) (control.SendResponse, error) {
	toHash, err := wallet.DecodeAddress(to)
	if err != nil {
		return control.SendResponse{}, fmt.Errorf("收款地址非法: %w", err)
	}
	height := s.nextHeight()
	snap := s.chain.UTXOSnapshot()

	tx, actualFee, err := txbuild.BuildTransaction(snap, s.miner, toHash, amount, txbuild.Options{
		Fee:    fee,
		Height: height,
	})
	if err != nil {
		return control.SendResponse{}, err
	}

	// 入池前先过一遍完整校验（签名、双花、金额），避免把无效交易广播出去
	if err := s.pool.Add(snap, tx, height); err != nil {
		return control.SendResponse{}, fmt.Errorf("交易未通过校验，已拒绝广播: %w", err)
	}
	s.broadcastTx(tx)
	log.Printf("[node] 已提交并广播交易: %s → %s 金额=%d 手续费=%d 输入数=%d",
		s.miner.Address(), to, amount, actualFee, len(tx.Inputs))

	return control.SendResponse{
		TxID:     tx.HashHex(),
		Fee:      actualFee,
		Amount:   amount,
		To:       to,
		InputNum: len(tx.Inputs),
	}, nil
}

// BlockHex 返回指定高度区块的规范编码（十六进制）。
func (s *nodeService) BlockHex(height int) (string, error) {
	b, err := s.chain.BlockByHeight(height)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b.Encode()), nil
}

// BlocksPage 实现 control.Node.BlocksPage（PHASE EXPLORER-API-IMPLEMENTATION-1）。
//
// 数据路径：**单次** blockchain.BlocksFrom(from,count) 完成 canonical 主链批量
// 读取（该方法本就是 P2P 同步用的公开分页原语，天然只含 canonical 链），
// 随后在 nodeapi 侧做纯展示派生（hash/难度/体积/交易摘要），零新增内部扫描、
// 零存储触碰、零锁策略改动。
//
// canonical 块的 height 直接来自 BlocksFrom 的索引位置；persisted 恒为 true：
// 内存 canonical 链完全源自 blocks.dat 回放/appendFrames 提交（blocks.dat 唯一真值）。
func (s *nodeService) BlocksPage(from, count int) (control.BlocksPageResult, error) {
	blocks, atTip := s.chain.BlocksFrom(from, count)
	out := make([]control.BlockJSON, 0, len(blocks))
	for i, b := range blocks {
		h := from + i
		out = append(out, blockToExplorerJSON(b, &h, true))
	}
	return control.BlocksPageResult{
		From:      from,
		Requested: count,
		Returned:  len(out),
		AtTip:     atTip,
		Height:    s.chain.Height(),
		Blocks:    out,
	}, nil
}

// BlockJSONByHash 实现 control.Node.BlockJSONByHash（PHASE EXPLORER-API-IMPLEMENTATION-1）。
//
// 数据路径：复用既有 blockchain.BlockByHash（canonical 内存链 → store fallback，
// detached 命中走既有机制，零新扫描算法）+ IsCanonicalHash 判定展示标签。
//
// canonical 块的高度解析：经**单次**公开 BlocksFrom(0, 链高+1) 取回整链指针
// 切片后在 nodeapi 侧匹配（一次临界区；与既有 by-hash 公开路径同为 O(h) 量级，
// 不引入任何 blockchain 内部新路径）。detached 块无公开高度来源 → Height 省略。
func (s *nodeService) BlockJSONByHash(hash [32]byte) (control.BlockJSON, error) {
	b, ok := s.chain.BlockByHash(hash)
	if !ok {
		return control.BlockJSON{}, control.ErrBlockNotFound
	}
	canonical := s.chain.IsCanonicalHash(hash)
	var height *int
	if canonical {
		if h, found := s.canonicalHeightOf(hash); found {
			height = &h
		}
	}
	return blockToExplorerJSON(b, height, canonical), nil
}

// canonicalHeightOf 在 canonical 链上解析区块高度（单次公开 BlocksFrom 读，nodeapi 侧匹配）。
func (s *nodeService) canonicalHeightOf(hash [32]byte) (int, bool) {
	tip := s.chain.Height()
	if tip < 0 {
		return 0, false
	}
	blocks, _ := s.chain.BlocksFrom(0, tip+1)
	for i, b := range blocks {
		if b.Header.Hash() == hash {
			return i, true
		}
	}
	return 0, false
}

// blockToExplorerJSON 把 *block.Block 派生为 Explorer JSON 视图（纯展示，无共识参与）。
//
// height 为 nil 时（detached 块）省略字段；canonical=false 的块经 by-hash 命中，
// 必然已落盘（store fallback 命中即落盘证据）→ persisted 恒为 true。
// fee 仅对 coinbase 诚实给出 0；普通交易无历史输出索引 → 省略（见 control.TxJSON 注释）。
func blockToExplorerJSON(b *block.Block, height *int, canonical bool) control.BlockJSON {
	txs := make([]control.TxJSON, 0, len(b.Transactions))
	for _, tx := range b.Transactions {
		var totalOut uint64
		for _, o := range tx.Outputs {
			totalOut += o.Value
		}
		tj := control.TxJSON{
			TxID:        tx.HashHex(),
			Coinbase:    tx.IsCoinbase(),
			InputCount:  len(tx.Inputs),
			OutputCount: len(tx.Outputs),
			TotalOut:    totalOut,
		}
		if tj.Coinbase {
			zero := int64(0)
			tj.Fee = &zero
		}
		txs = append(txs, tj)
	}

	era := "pre-hardfork"
	if height != nil && *height >= pow.ActivationHeight {
		era = "post-hardfork"
	} else if height == nil {
		// 高度未知（detached）：era 无法从高度判定 → 不猜，按 pre-hardfork 之外
		// 的第三态不引入新枚举，保守省略判据（见实施报告 known limitations）。
		era = "unknown"
	}

	bj := control.BlockJSON{
		Hash:         b.Header.HashHex(),
		PreviousHash: hex.EncodeToString(b.Header.PrevBlockHash[:]),
		Timestamp:    b.Header.Timestamp,
		Version:      b.Header.Version,
		Bits:         b.Header.Bits,
		Difficulty:   relativeDifficulty(b.Header.Bits),
		Nonce:        b.Header.Nonce,
		MerkleRoot:   hex.EncodeToString(b.Header.MerkleRoot[:]),
		Size:         b.Size(),
		TxCount:      len(b.Transactions),
		Canonical:    canonical,
		Persisted:    true,
		Transactions: txs,
		ConsensusEra: era,
	}
	bj.Height = height
	return bj
}

// Mine 按需立即挖出 count 个区块（测试网/开发便利功能）。
//
// 与持续挖矿循环（-mine）互斥：若正在持续挖矿则直接拒绝，
// 否则两个挖矿路径会各自组装候选区块、互相作废，白烧 CPU 且日志混乱。
//
// 退出条件（PHASE MINING-REMEDIATION-1）：只要本轮不是「成功出块」
// （链尾变化 / 政策终态 / 结构性错误 / 可重试失败）即停止本轮，
// 不再把「被拒绝」误当作「被链尾变化中断」。
func (s *nodeService) Mine(count int) (control.MineResponse, error) {
	if s.mining.Load() {
		return control.MineResponse{}, errors.New("节点正在持续挖矿（-mine），请先停用持续挖矿再使用按需出块")
	}
	if count <= 0 {
		return control.MineResponse{}, fmt.Errorf("挖矿数量必须大于 0，实际 %d", count)
	}

	s.setMineState(MiningStarting, "on-demand")
	mined := 0
	last := mineOutcomeStop
	for i := 0; i < count; i++ {
		last = mineOnce(s, nil)
		if last != mineOutcomeMined {
			break
		}
		mined++
	}
	// 终态：结构性错误与政策终态保留其状态（便于 /status 直接暴露原因），
	// 其余情形回到 STOPPED（按需出块已结束）。
	switch last {
	case mineOutcomeStructFail:
		// 保留 FAILED
	case mineOutcomeStalled:
		// 保留 STALLED
	default:
		s.setMineState(MiningStopped, "on-demand-done")
	}
	return control.MineResponse{Mined: mined, Height: s.chain.Height()}, nil
}

// sortUTXOs 按高度、OutPoint 排序，使接口输出稳定（便于人工核对与脚本处理）。
func sortUTXOs(list []control.UTXOInfo) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].Height != list[j].Height {
			return list[i].Height < list[j].Height
		}
		return list[i].OutPoint < list[j].OutPoint
	})
}
