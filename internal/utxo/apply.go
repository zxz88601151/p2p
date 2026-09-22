package utxo

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"p2pchain/internal/transaction"
	"p2pchain/internal/wallet"
)

// 共识参数。
const (
	// CoinbaseMaturity coinbase 输出必须经过的成熟期（块数）后才可被花费。
	// 比特币为 100；本项目为测试网取向取 10，属共识参数，修改即硬分叉。
	CoinbaseMaturity = 10

	// subsidyInitial 初始出块奖励（最小单位）。
	//
	// C1 经济政策（A-2.3-G2 实施，项目方已于 A-2.3-F-R4 §5 冻结并授权 Q6）：
	// 50 → 5。该值属**共识权威**，修改即硬分叉。
	subsidyInitial = 5

	// subsidyHalvingInterval 奖励减半间隔（块数）。
	//
	// C1 经济政策（A-2.3-G2 实施）：210 → 5,250,000。该值属**共识权威**。
	subsidyHalvingInterval = 5_250_000
)

// Subsidy 返回指定高度区块的出块奖励：初始 5，每 5,250,000 块减半。
//
// C1 参数（A-2.3-G2）下的权威语义：halvings = height / subsidyHalvingInterval
// （uint 整除，向下取整）；5 = 0b101 仅 3 位，故第 3 次减半即归零 ——
// 最后一个非零补贴高度为 15,749,999（补贴 1），归零高度为 15,750,000。
// halvings >= 64 的守卫保留为**防御性上界**（对 int 高度实际不可达）。
func Subsidy(height int) uint64 {
	halvings := uint(height / subsidyHalvingInterval)
	if halvings >= 64 {
		return 0
	}
	return subsidyInitial >> halvings
}

// 校验错误（全部为共识拒绝原因，逐一对应攻击矩阵）。
var (
	ErrNoInputs          = errors.New("交易没有输入（非 coinbase 交易必须消费至少一个 UTXO）")
	ErrNoOutputs         = errors.New("交易没有输出")
	ErrZeroOutput        = errors.New("交易输出金额为 0")
	ErrDuplicateInput    = errors.New("交易包含重复输入（同一 OutPoint 被引用两次）")
	ErrUnknownUTXO       = errors.New("输入引用的 UTXO 不存在或已被花费")
	ErrImmatureCoinbase  = errors.New("coinbase 输出尚未度过成熟期，不能花费")
	ErrOverspend         = errors.New("输入总额小于输出总额（超额支出）")
	ErrPubKeyMismatch    = errors.New("输入公钥与该 UTXO 锁定的公钥哈希不匹配")
	ErrBadSignature      = errors.New("输入签名验证失败")
	ErrBadCoinbase       = errors.New("coinbase 交易非法")
	ErrExcessiveCoinbase = errors.New("coinbase 输出超过区块奖励加手续费上限")
	// ErrFeeOverflow 块内手续费累计发生 uint64 回绕（A-2.3-G 算术安全防护）。
	// 手续费是既有 UTXO 的价值转移，理论上不可能达到 2^64 量级；
	// 一旦回绕意味着记账已不可信，按共识拒绝（fail-closed）。
	ErrFeeOverflow = errors.New("块内手续费累计 uint64 回绕")
)

// sighash 返回签名消息哈希。
//
// 决策（PHASE 1A 决策 A4/H）：sighash = tx.Hash()，即 serializeForHash 的 SHA256，
// 该序列化天然排除 Signature 与 PubKey，不存在"签名依赖自身哈希"的循环。
//
// 安全性说明：PubKey 未被 sighash 覆盖，但花费权由「PubKey 的哈希 == UTXO 锁定的
// PubKeyHash」绑定——替换公钥必然导致 PubKeyMismatch；PrevTxHash/OutIndex/全部输出
// 均在 sighash 覆盖内，签名后篡改任何一处都会导致验签失败。
// （后续可升级为 BIP-143 风格的分段 sighash，本阶段保持该简化并已文档化。）
func sighash(tx *transaction.Transaction) [32]byte {
	return tx.Hash()
}

// ValidateCoinbaseStructure 校验 coinbase 交易的结构规则（不含金额上限——金额上限
// 需要区块内手续费总和，属区块链层策略，见 ApplyBlock）。
// height 绑定：Signature 字段必须为 4 字节小端区块高度（BIP34 风格），
// 保证 coinbase TxID 唯一且不可跨高度重放。
func ValidateCoinbaseStructure(tx *transaction.Transaction, height int) error {
	if !tx.IsCoinbase() {
		return fmt.Errorf("%w: 不是合法的 coinbase 形态", ErrBadCoinbase)
	}
	in := tx.Inputs[0]
	if len(in.PubKey) != 0 {
		return fmt.Errorf("%w: coinbase 输入不得携带公钥", ErrBadCoinbase)
	}
	h, ok := transaction.DecodeCoinbaseHeight(in.Signature)
	if !ok {
		return fmt.Errorf("%w: coinbase 高度字段缺失或非法（须为 4 字节小端）", ErrBadCoinbase)
	}
	if h != height {
		return fmt.Errorf("%w: coinbase 声明高度 %d 与所在区块高度 %d 不一致", ErrBadCoinbase, h, height)
	}
	if len(tx.Outputs) == 0 {
		return fmt.Errorf("%w: coinbase 必须至少有一个输出", ErrNoOutputs)
	}
	for _, out := range tx.Outputs {
		if out.Value == 0 {
			return fmt.Errorf("%w: coinbase 输出金额为 0", ErrZeroOutput)
		}
	}
	return nil
}

// ValidateTransaction 在给定集合 set 上校验并应用一笔交易（先验证、后应用）。
//
// 返回值 fee：非 coinbase = 输入总额 - 输出总额；coinbase = 0。
// 验证全部通过后 set 才被修改（新增输出 / 消费输入）；失败时 set 保持原样。
// 区块级原子性由调用方在 Clone 上操作保证（见 ApplyBlock）。
//
// height 为正在处理该交易所在区块的高度（maturity 判定依据）。
func ValidateTransaction(tx *transaction.Transaction, set *UTXOSet, height int) (fee uint64, err error) {
	if tx.IsCoinbase() {
		if err := ValidateCoinbaseStructure(tx, height); err != nil {
			return 0, err
		}
		for i := range tx.Outputs {
			set.Add(outpointAt(tx, i), Entry{
				Value:      tx.Outputs[i].Value,
				PubKeyHash: tx.Outputs[i].PubKeyHash,
				Height:     height,
				IsCoinbase: true,
			})
		}
		return 0, nil
	}

	// ---- 结构校验（验证阶段：只读 + 本地状态，不修改 set）----
	if len(tx.Inputs) == 0 {
		return 0, ErrNoInputs
	}
	if len(tx.Outputs) == 0 {
		return 0, ErrNoOutputs
	}
	for _, out := range tx.Outputs {
		if out.Value == 0 {
			return 0, fmt.Errorf("%w（输出金额 %d）", ErrZeroOutput, out.Value)
		}
	}

	seen := make(map[OutPoint]struct{}, len(tx.Inputs))
	spends := make([]OutPoint, 0, len(tx.Inputs))
	var inTotal uint64
	for _, in := range tx.Inputs {
		op := OutPoint{Hash: in.PrevTxHash, Index: in.OutIndex}

		// 重复输入检测（同一交易内双花同一输出）
		if _, dup := seen[op]; dup {
			return 0, fmt.Errorf("%w: %s", ErrDuplicateInput, op)
		}
		seen[op] = struct{}{}

		// 输入存在性（不存在或已被花费）
		entry, ok := set.Get(op)
		if !ok {
			return 0, fmt.Errorf("%w: %s", ErrUnknownUTXO, op)
		}

		// coinbase 成熟期
		if entry.IsCoinbase && height-entry.Height < CoinbaseMaturity {
			return 0, fmt.Errorf("%w: 输出在高度 %d 产生，当前高度 %d，需 %d",
				ErrImmatureCoinbase, entry.Height, height, CoinbaseMaturity)
		}

		// 花费权：公钥哈希必须与锁定值匹配（覆盖 malformed pubkey 攻击面：
		// 非法公钥无法匹配哈希，即使侥幸匹配也会在验签阶段失败）
		pubHash := sha256.Sum256(in.PubKey)
		var keyHash [20]byte
		copy(keyHash[:], pubHash[:20])
		if keyHash != entry.PubKeyHash {
			return 0, fmt.Errorf("%w: 输入 %s", ErrPubKeyMismatch, op)
		}

		// 签名验证（消息 = sighash，覆盖除 sig/pubkey 外的全部交易内容）
		ok2, err := wallet.Verify(in.PubKey, sighash(tx), in.Signature)
		if err != nil {
			return 0, fmt.Errorf("%w: %v", ErrBadSignature, err)
		}
		if !ok2 {
			return 0, fmt.Errorf("%w: 输入 %s", ErrBadSignature, op)
		}

		// 金额累计（uint64 溢出防护）
		if inTotal+entry.Value < inTotal {
			return 0, ErrOverspend
		}
		inTotal += entry.Value
		spends = append(spends, op)
	}

	var outTotal uint64
	for _, out := range tx.Outputs {
		if outTotal+out.Value < outTotal {
			return 0, ErrOverspend
		}
		outTotal += out.Value
	}
	if inTotal < outTotal {
		return 0, fmt.Errorf("%w: 输入 %d < 输出 %d", ErrOverspend, inTotal, outTotal)
	}
	fee = inTotal - outTotal

	// ---- 验证全部通过，应用状态变更 ----
	for _, op := range spends {
		if _, err := set.Spend(op); err != nil {
			// 理论不可达：验证阶段已确认存在。防御性处理。
			return 0, fmt.Errorf("应用阶段消费 UTXO 失败: %w", err)
		}
	}
	for i := range tx.Outputs {
		set.Add(outpointAt(tx, i), Entry{
			Value:      tx.Outputs[i].Value,
			PubKeyHash: tx.Outputs[i].PubKeyHash,
			Height:     height,
			IsCoinbase: false,
		})
	}
	return fee, nil
}

// ApplyBlock 在 base 集合的克隆上原子地应用一整个区块的交易列表。
//
// 共识规则：
//   - 交易数 ≥ 1；恰好一笔 coinbase，且必须位于 index 0；
//   - 非 coinbase 交易逐笔 ValidateTransaction（签名/双花/maturity/金额），累计手续费；
//   - coinbase 输出总额 ≤ Subsidy(height) + 全块手续费（ErrExcessiveCoinbase）。
//
// 全部通过后返回新集合与总手续费；任何一步失败返回 (nil, 0, err)，
// 调用方保持原集合不变——保证无 partial state transition。
// 本函数不校验区块头（PrevHash/PoW/Merkle/Bits/时间戳/大小），那些属于 blockchain 层。
func ApplyBlock(base *UTXOSet, txs []*transaction.Transaction, height int) (newSet *UTXOSet, totalFees uint64, err error) {
	if base == nil {
		return nil, 0, errors.New("utxo: 基础集合为空")
	}
	if len(txs) == 0 {
		return nil, 0, ErrNoOutputs
	}
	if !txs[0].IsCoinbase() {
		return nil, 0, fmt.Errorf("%w: 首笔交易不是 coinbase", ErrBadCoinbase)
	}
	for _, tx := range txs[1:] {
		if tx.IsCoinbase() {
			return nil, 0, fmt.Errorf("%w: 区块中出现多于一个 coinbase", ErrBadCoinbase)
		}
	}

	working := base.Clone()

	// coinbase 输出先入集合（它没有真实输入，不影响普通交易验证；
	// 且同块花费因 maturity 必然被拒）
	if fee, err := ValidateTransaction(txs[0], working, height); err != nil {
		return nil, 0, err
	} else if fee != 0 {
		return nil, 0, fmt.Errorf("%w: coinbase 交易不应产生手续费", ErrBadCoinbase)
	}

	var fees uint64
	for _, tx := range txs[1:] {
		fee, err := ValidateTransaction(tx, working, height)
		if err != nil {
			return nil, 0, fmt.Errorf("高度 %d 交易校验失败: %w", height, err)
		}
		// A-2.3-G：手续费累计 uint64 回绕防护（与普通交易 inTotal 累加的
		// `if x+v < x` 先例同型）。回绕会使上限比较失效，必须拒绝。
		if fees+fee < fees {
			return nil, 0, fmt.Errorf("%w: 高度 %d 手续费累计回绕", ErrFeeOverflow, height)
		}
		fees += fee
	}

	// coinbase 金额上限：奖励 + 手续费。
	// A-2.3-G 算术加固：
	//   1) coinbaseOut 累加带回绕防护——2^63 + 2^63 ≡ 0 (mod 2^64) 曾可绕过
	//      上限检查（BLK-A23F-1）；真实合计一旦 ≥ 2^64 必然超过任何可达上限，
	//      按超额拒绝；
	//   2) 上限比较改写为减法形式（coinbaseOut - Subsidy > fees），
	//      从构造上消除 Subsidy(height)+fees 的加法回绕可能性（fail-safe
	//      方向虽不变严，但显式消除回绕路径，且与 fees/coinbaseOut 防护对称）。
	var coinbaseOut uint64
	for _, out := range txs[0].Outputs {
		sum := coinbaseOut + out.Value
		if sum < coinbaseOut {
			return nil, 0, fmt.Errorf(
				"%w: coinbase 输出真实合计 ≥ 2^64（uint64 回绕被拒）",
				ErrExcessiveCoinbase)
		}
		coinbaseOut = sum
	}
	if sub := Subsidy(height); coinbaseOut >= sub {
		if coinbaseOut-sub > fees {
			return nil, 0, fmt.Errorf(
				"%w: coinbase 输出 %d > 奖励 %d + 手续费 %d",
				ErrExcessiveCoinbase, coinbaseOut, sub, fees)
		}
	}

	return working, fees, nil
}
