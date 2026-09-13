// mining_state.go 定义挖矿运行时的**语义状态**与结果分类。
//
// PHASE MINING-REMEDIATION-1 的背景：修复前 mineOnce 返回裸 bool，把三类完全不同
// 的结果（链尾变化 / 收到停止 / 区块被结构性拒绝）压成同一个 false；上层 for 循环
// 于是对任何 false 都「立即以同一高度重试」。在「补贴耗尽且无手续费交易」时，
// 该模板**不存在合法候选**（coinbase 必须有 >0 的输出，而上限为 Subsidy+fees=0），
// 却被反复求解完整 PoW —— 表现为 6 核热循环 + 5 GB/天日志。
//
// 本文件把「运行状态」与「失败原因」显式化：
//
//	STOPPED  未挖矿（未开启 / 已停止 / 已达出块上限）
//	STARTING 已启动，首个模板尚未完成预校验
//	RUNNING  正在对【已通过预校验的】模板求解 PoW（或刚出块，链推进正常）
//	STALLED  政策终态：当前不存在合法候选区块（补贴耗尽且无手续费交易）
//	FAILED   结构性错误：模板不满足共识要求，挖矿已终止（需修复后重启）
//	STOPPING 已收到停止请求，正在取消 worker
//
// 关键区别：**STALLED 不是缺陷，FAILED 是**。二者在修复前都被误报为「运行中」。
package main

import "p2pchain/internal/blockchain"

// miningState 是挖矿运行时的语义状态。
type miningState string

const (
	// MiningStopped 未挖矿。
	MiningStopped miningState = "STOPPED"
	// MiningStarting 已启动，首个模板尚未完成预校验。
	MiningStarting miningState = "STARTING"
	// MiningRunning 正在对已通过预校验的模板求解 PoW。
	MiningRunning miningState = "RUNNING"
	// MiningStalled 政策终态：不存在合法候选区块（不执行 PoW，亦非缺陷）。
	MiningStalled miningState = "STALLED"
	// MiningFailed 结构性错误：挖矿已终止（需修复后重启节点）。
	MiningFailed miningState = "FAILED"
	// MiningStopping 已收到停止请求。
	MiningStopping miningState = "STOPPING"
)

// mineOutcome 是 mineOnce 的分类结果，取代语义重载的裸 bool。
type mineOutcome int

const (
	// mineOutcomeMined 成功挖出并上链一个区块。
	mineOutcomeMined mineOutcome = iota
	// mineOutcomeStale 模板作废：链尾变化，重建模板重试即可（非缺陷）。
	mineOutcomeStale
	// mineOutcomeStop 收到停止请求。
	mineOutcomeStop
	// mineOutcomeStalled 政策终态：不存在合法候选区块（非缺陷）。
	mineOutcomeStalled
	// mineOutcomeStructFail 结构性错误：必须终止挖矿（FAILED）。
	mineOutcomeStructFail
	// mineOutcomeBackoff 其它可重试失败（有界退避后重试，超限升级为 FAILED）。
	mineOutcomeBackoff
)

// String 返回稳定的英文标识，便于测试断言与日志检索。
func (o mineOutcome) String() string {
	switch o {
	case mineOutcomeMined:
		return "MINED"
	case mineOutcomeStale:
		return "STALE"
	case mineOutcomeStop:
		return "STOP"
	case mineOutcomeStalled:
		return "STALLED"
	case mineOutcomeStructFail:
		return "STRUCT_FAIL"
	case mineOutcomeBackoff:
		return "BACKOFF"
	default:
		return "UNKNOWN"
	}
}

// templateFailureClass 是「模板预校验失败」的语义分类，对应提示词 §5 的 A–E。
//
// 之所以把它抽成**纯函数**（不读链、不读全局状态），是为了让「某一类失败应当
// 如何处置」这一决策可被确定性单测覆盖 —— 包括在高度 1260（补贴归零）这类
// 需要构建整条长链才能自然到达的场景，也可直接构造输入验证分类结果。
type templateFailureClass int

const (
	// classTemplateStale 对应 §5 的 B（链尾已变化）与 C（陈旧/竞争）：模板本身
	// 并不非法，重建模板重试即可。**不得**记为结构性缺陷。
	classTemplateStale templateFailureClass = iota
	// classTemplatePolicyTerminal 对应 §5 的 D（政策终态）：该高度**不存在**
	// 任何合法候选区块（补贴与手续费之和为 0）。不是缺陷、不是 FAILED。
	classTemplatePolicyTerminal
	// classTemplateStructural 对应 §5 的 A（结构性失败）：模板不满足共识要求，
	// 重试无法恢复，必须终止挖矿（FAILED）。
	classTemplateStructural
)

// classifyTemplateFailure 把「模板预校验失败 + 该高度 coinbase 可用的奖励总额
// （= Subsidy(height) + fees）」映射为分类。
//
// 判定顺序及其理由（顺序本身是语义的一部分，不可随意交换）：
//
//  1. **stale 优先**：链尾/难度/时间戳已变化时，模板陈旧只是「时机问题」。
//     若此时先判 policy terminal，则每次链尾推进都会把 STALLED 状态刷一遍，
//     状态在 RUNNING↔STALLED 之间抖动，掩盖真正的原因。
//  2. **policy terminal 次之**：totalReward == 0 时，模板必然违反
//     「coinbase 至少一个输出且输出金额不为 0」（⇒ 总额 ≥ 1）与
//     「coinbase 总额 ≤ totalReward」（⇒ 总额 ≤ 0）两条共识规则的合取——
//     该合取在 totalReward == 0 时无解。因此此时预校验必然失败，且
//     失败原因是**可机械判定的政策终态**，而非需要修复的结构性缺陷。
//     这正是「不得把 Subsidy==0 && fees==0 误报为故障」的落点。
//  3. **其余一律结构性**：没有任何其它已知原因，意味着模板构造逻辑
//     与共识规则已经不一致，重试只会持续烧 CPU，必须显式 FAILED。
//
// totalReward 由调用方按共识函数计算传入（**不在此处复算共识结论**，
// 避免出现「第二份共识实现」）。
func classifyTemplateFailure(err error, totalReward uint64) templateFailureClass {
	if err == nil {
		// 防御：无错误即无需分类（调用方不应走到这里）。
		return classTemplateStructural
	}
	if blockchain.IsTemplateStale(err) {
		return classTemplateStale
	}
	if totalReward == 0 {
		return classTemplatePolicyTerminal
	}
	return classTemplateStructural
}

// ---- 语义状态读写（供挖矿循环与 /status 使用）----

// setMineState 更新语义状态与原因，返回「状态是否发生变化」。
//
// 调用方据此决定是否写日志 —— 这是「不产生无限刷屏」的关键：
// **只有状态转移才写日志**，重复处于同一状态不写。日志量因此与状态变化的次数
// 成正比，而与重试/PoW 循环的迭代次数无关（后者才可能是每秒数百次）。
func (s *nodeService) setMineState(st miningState, reason string) bool {
	prev, _ := s.mineState.Load().(miningState)
	s.mineReason.Store(reason)
	if prev == st {
		return false
	}
	s.mineState.Store(st)
	return true
}

// miningStateSnapshot 返回当前的（语义状态, 原因），供 /status 与 CLI 读取。
func (s *nodeService) miningStateSnapshot() (miningState, string) {
	st, _ := s.mineState.Load().(miningState)
	reason, _ := s.mineReason.Load().(string)
	if st == "" {
		st = MiningStopped
	}
	return st, reason
}
