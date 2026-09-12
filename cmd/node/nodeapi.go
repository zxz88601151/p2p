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
	return control.StatusInfo{
		Height:      s.chain.Height(),
		TipHash:     tip.Header.HashHex(),
		Peers:       s.net.PeerAddrs(),
		MempoolSize: s.pool.Len(),
		Mining:      s.mining.Load(),
		Address:     s.miner.Address(),
		Bits:        bits,
		Difficulty:  relativeDifficulty(bits),
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

// Mine 按需立即挖出 count 个区块（测试网/开发便利功能）。
//
// 与持续挖矿循环（-mine）互斥：若正在持续挖矿则直接拒绝，
// 否则两个挖矿路径会各自组装候选区块、互相作废，白烧 CPU 且日志混乱。
func (s *nodeService) Mine(count int) (control.MineResponse, error) {
	if s.mining.Load() {
		return control.MineResponse{}, errors.New("节点正在持续挖矿（-mine），请先停用持续挖矿再使用按需出块")
	}
	if count <= 0 {
		return control.MineResponse{}, fmt.Errorf("挖矿数量必须大于 0，实际 %d", count)
	}

	mined := 0
	for i := 0; i < count; i++ {
		if !mineOnce(s) {
			// 被链尾变化中断：说明有对端区块到达，本轮作废，不计入结果
			break
		}
		mined++
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
