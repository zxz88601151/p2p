// Package utxo 实现 UTXO（未花费交易输出）集合与共识状态迁移。
//
// 职责边界：
//   - 本包只负责「状态」与「状态迁移」：给定已通过验证的交易，原子地更新 UTXO 集合；
//   - 交易的签名验证依赖 wallet 包（Verify），金额/结构校验在本包完成；
//   - 共识策略（coinbase 位置、金额上限、难度）由 blockchain 包基于本包的
//     ValidateTransaction / ApplyBlock 组合实现。
//
// 核心纪律：Validate First → Apply Second。先在 UTXO 集合的克隆上完成全部验证，
// 全部通过后才允许整体替换真实状态——任何一步失败都不会留下半迁移状态。
package utxo

import (
	"fmt"
)

// OutPoint 唯一标识一个未花费输出：某笔交易（TxID）的第 Index 个输出。
type OutPoint struct {
	Hash  [32]byte // 被引用交易的 TxID
	Index uint32   // 输出在该交易 Outputs 中的下标
}

// String 返回十六进制可读形式，用于日志与调试。
func (op OutPoint) String() string {
	return fmt.Sprintf("%x:%d", op.Hash, op.Index)
}
