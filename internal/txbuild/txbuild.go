// Package txbuild 负责「从钱包构造并签名交易」——即钱包与 UTXO 状态之间的桥接层。
//
// 独立成包的原因（依赖方向）：
//
//	utxo    → 依赖 wallet（验签）与 transaction
//	txbuild → 依赖 wallet + utxo + transaction
//
// 若把选币与签名放进 wallet 包，则形成 wallet ↔ utxo 循环依赖。
package txbuild

import (
	"errors"
	"fmt"
	"sort"

	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

var (
	ErrInsufficientFunds = errors.New("余额不足（含手续费）")
	ErrDustChange        = errors.New("找零金额过小")
)

// DustThreshold 低于该金额的找零并入手续费（避免产生极小额输出）。
const DustThreshold = 1

// Options 交易构造参数。
type Options struct {
	// Fee 目标手续费（绝对金额，最小单位）。
	Fee uint64
	// Height 该交易计划被打包的高度（用于 coinbase maturity 过滤）。
	Height int
	// IncludeImmature 是否允许选用未成熟的 coinbase 输出（仅供测试与特殊场景）。
	IncludeImmature bool
}

// SelectResult 选币结果。
type SelectResult struct {
	Inputs     []transaction.TxInput
	TotalIn    uint64
	Change     uint64
	Fee        uint64
	SelectedOP []utxo.OutPoint
}

// SelectUTXOs 按金额降序贪心选币，直到满足 amount+fee。
// 只选取属于 from 的、可花费（非未成熟 coinbase）的输出。
func SelectUTXOs(set *utxo.UTXOSet, from *wallet.Wallet, amount uint64, opt Options) (*SelectResult, error) {
	type cand struct {
		op utxo.OutPoint
		e  utxo.Entry
	}
	owner := from.PubKeyHash()
	var cands []cand
	for op, e := range set.AllEntries() {
		if e.PubKeyHash != owner {
			continue
		}
		if !opt.IncludeImmature && e.IsCoinbase && opt.Height-e.Height < utxo.CoinbaseMaturity {
			continue
		}
		cands = append(cands, cand{op, e})
	}
	// 金额降序（减少输入数量），同额时按 OutPoint 稳定排序保证确定性
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].e.Value != cands[j].e.Value {
			return cands[i].e.Value > cands[j].e.Value
		}
		if cands[i].op.Hash != cands[j].op.Hash {
			return string(cands[i].op.Hash[:]) < string(cands[j].op.Hash[:])
		}
		return cands[i].op.Index < cands[j].op.Index
	})

	target := amount + opt.Fee
	if target < amount {
		return nil, ErrInsufficientFunds // 溢出
	}

	res := &SelectResult{Fee: opt.Fee}
	for _, c := range cands {
		if res.TotalIn >= target {
			break
		}
		res.Inputs = append(res.Inputs, transaction.TxInput{PrevTxHash: c.op.Hash, OutIndex: c.op.Index})
		res.SelectedOP = append(res.SelectedOP, c.op)
		res.TotalIn += c.e.Value
	}
	if res.TotalIn < target {
		return nil, fmt.Errorf("%w: 可用 %d, 需要 %d（含手续费 %d）",
			ErrInsufficientFunds, res.TotalIn, target, opt.Fee)
	}
	res.Change = res.TotalIn - target
	return res, nil
}

// BuildTransaction 构造并签名一笔支付交易。
//
// 规则：
//   - 选币覆盖 amount + Fee；
//   - 找零 = 输入总额 - amount - Fee，小于 DustThreshold 时并入手续费（不产生找零输出）；
//   - 每个输入用 from 私钥签名（sighash = tx.Hash()，与 utxo 校验一致）。
func BuildTransaction(set *utxo.UTXOSet, from *wallet.Wallet, to [20]byte, amount uint64, opt Options) (*transaction.Transaction, uint64, error) {
	if amount == 0 {
		return nil, 0, errors.New("转账金额必须大于 0")
	}
	sel, err := SelectUTXOs(set, from, amount, opt)
	if err != nil {
		return nil, 0, err
	}

	tx := &transaction.Transaction{Inputs: sel.Inputs}
	tx.Outputs = append(tx.Outputs, transaction.TxOutput{Value: amount, PubKeyHash: to})

	fee := sel.Fee
	if sel.Change > 0 {
		if sel.Change < DustThreshold {
			fee += sel.Change // 并入手续费
		} else {
			tx.Outputs = append(tx.Outputs, transaction.TxOutput{
				Value:      sel.Change,
				PubKeyHash: from.PubKeyHash(),
			})
		}
	}

	// 签名：sighash 不包含签名本身，因此可以先填全部输入引用与输出再统一签名
	h := tx.Hash()
	for i := range tx.Inputs {
		sig, err := from.Sign(h)
		if err != nil {
			return nil, 0, fmt.Errorf("签名失败: %w", err)
		}
		tx.Inputs[i].Signature = sig
		tx.Inputs[i].PubKey = from.PublicKey
	}
	return tx, fee, nil
}

// SpendableBalance 返回某钱包当前可花费余额（排除未成熟 coinbase）。
func SpendableBalance(set *utxo.UTXOSet, w *wallet.Wallet, height int) uint64 {
	return set.Balance(w.PubKeyHash(), height, false)
}

// TotalBalance 返回某钱包总余额（含未成熟 coinbase）。
func TotalBalance(set *utxo.UTXOSet, w *wallet.Wallet, height int) uint64 {
	return set.Balance(w.PubKeyHash(), height, true)
}
