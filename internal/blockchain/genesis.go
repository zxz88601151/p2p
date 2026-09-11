package blockchain

import (
	"p2pchain/internal/block"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
)

// GenesisTimestamp 创世区块的固定时间戳（2023-11-14T22:13:20Z）。
const GenesisTimestamp int64 = 1700000000

// GenesisMinerPubKeyHash 创世 coinbase 的接收方（不可花费的"黑洞"地址哈希）。
// 使用固定值以保证全网络生成完全相同的创世区块。
var GenesisMinerPubKeyHash = [20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}

// NewGenesisBlock 生成确定性创世区块：
//   - 固定时间戳（GenesisTimestamp）
//   - 固定 coinbase（发送至 GenesisMinerPubKeyHash）
//   - 固定难度（pow.MaxTargetBits）
//
// 确定性是网络的前提：所有节点必须得到字节级相同的创世区块，否则无法就链达成一致。
// 创世 coinbase 的成熟期照常适用；其输出因无对应私钥而不可花费（黑洞）。
func NewGenesisBlock() *block.Block {
	cb := transaction.NewCoinbaseTx(GenesisMinerPubKeyHash, utxo.Subsidy(0), 0)
	g := block.NewCandidateBlock([32]byte{}, pow.MaxTargetBits, []*transaction.Transaction{cb})
	g.Header.Timestamp = GenesisTimestamp
	if found, _ := pow.Mine(g, 0); !found {
		// MaxTargetBits=20 期望约 2^20 次尝试，理论必然可解；防御性 panic 而非静默返回坏创世
		panic("创世区块挖矿失败：难度参数可能被改坏")
	}
	return g
}
