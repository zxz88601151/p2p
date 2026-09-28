package blockchain

import (
	"encoding/hex"

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

// CanonicalGenesisHash 是当前协议的链身份锚点。
//
// 它来自当前已存在的确定性 NewGenesisBlock 定义，而不是新的 Chain ID、
// Protocol Version 或独立 metadata。任何影响 canonical genesis 的共识修改
// 都必须先经过兼容性审计与协议身份决策，不能静默改变此值。
var CanonicalGenesisHash = [32]byte{0x00, 0x00, 0x3d, 0x97, 0x72, 0x3c, 0x3c, 0xcc, 0xec, 0x83, 0xa6, 0x64, 0xf5, 0xd2, 0x2d, 0xa6, 0xf6, 0x6d, 0xfa, 0x72, 0xc9, 0xf2, 0x8b, 0x28, 0x6c, 0x74, 0x6f, 0x4b, 0xc4, 0xdc, 0xe4, 0xa3}

// CanonicalGenesisHashHex 返回当前协议的 canonical genesis hash。
func CanonicalGenesisHashHex() string {
	return hex.EncodeToString(CanonicalGenesisHash[:])
}

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
	if found, _ := pow.Mine(g); !found {
		// MaxTargetBits=16 期望约 2^16 次尝试，理论必然可解；防御性 panic 而非静默返回坏创世
		panic("创世区块挖矿失败：难度参数可能被改坏")
	}
	return g
}
