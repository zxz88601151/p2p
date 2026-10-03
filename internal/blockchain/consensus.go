package blockchain

// consensus.go 封装 PHASE DIFFICULTY-CONSENSUS-IMPLEMENTATION-1 引入的**难度共识硬分叉**
// 校验逻辑：版本强制、时间戳（旧规则墙钟 / 新规则 MTP）、期望难度（按激活高度分叉）。
//
// 这些校验方法都在「调用方已持锁」的前提下运行（AddBlock 持写锁、ValidateBlock/ValidateTemplate
// 持读锁），因此底层访问链数据走**无锁** unsafeView，避免 sync.RWMutex 重入死锁
// （同一 goroutine 持写锁后再 RLock 会永久阻塞）。
//
// 设计要点：所有难度/时间戳/MTP 计算都委托给 pow 包的纯函数（ComputeExpectedBitsAt /
// MedianTimePastAt），仅通过 ChainView 抽象读取祖先。canonical 路径传 unsafeView{bc}
// （祖先即 bc.blocks）；**fork 校验路径传 fork 自身祖先视图**（blockchain.go 的
// forkChainView，由 parentNode.PathToRoot() 构造）——这是 DESIGN-1 §FC-004 的
// fork-choice 安全要求（O-3 修复：bits 与 timestamp/MTP 两个消费者都必须用 fork 视图）。

import (
	"fmt"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
)

// maxActivationTimestampSlack 是 R3（SEC-CLOSE MUST FIX 4）激活后区块时间戳
// 相对父块 MTP 的最大允许超前量（2 小时，与激活前 maxFutureTimestampDrift 的
// 量纲一致）。
//
// 共识语义（post-activation 完整规则，纯链内、无墙钟）：
//
//	MTP(parent) < ts <= MTP(parent) + 7200
//
// 下界（> MTP）防 timewarp；上界（≤ MTP+slack）封顶时间戳膨胀：
// 没有上界时，掌握出块权的攻击者可把候选块时间戳设到任意远未来，人为拉长
// 难度窗口的时间跨度、压低 retarget 后的难度，从而以低成本持续出块。
const maxActivationTimestampSlack int64 = 7200

// unsafeView 在调用方已持锁时提供无锁的 pow.ChainView 实现。
// 它直接读取 bc.blocks 切片，绝不自行加锁。
type unsafeView struct{ bc *Blockchain }

func (v unsafeView) BlockByHeight(h int) (*block.Block, error) {
	return v.bc.blockByHeightUnsafe(h)
}

func (v unsafeView) Height() int {
	return v.bc.heightUnsafe()
}

// blockByHeightUnsafe 无锁读取指定高度区块（调用方必须已持锁）。
func (bc *Blockchain) blockByHeightUnsafe(height int) (*block.Block, error) {
	if height < 0 || height >= len(bc.blocks) {
		return nil, fmt.Errorf("%w: %d（当前高度 %d）", ErrUnknownHeight, height, len(bc.blocks)-1)
	}
	return bc.blocks[height], nil
}

// heightUnsafe 无锁返回当前链高（创世高度 0）。
func (bc *Blockchain) heightUnsafe() int {
	return len(bc.blocks) - 1
}

// ActivationHeight 返回本链的激活高度（共识真值，运行期不变）。
func (bc *Blockchain) ActivationHeight() int {
	return bc.activationHeight
}

// expectedBitsForView 计算高度 height 区块的期望难度位，**基于调用方提供的 ChainView**。
//
// 主链（canonical）路径传 unsafeView{bc}；fork 校验路径传 **fork 自身祖先视图**
// （见 blockchain.go 的 forkChainView）——这正是 pow.ChainView 抽象与 DESIGN-1 §FC-004
// 「reorg 时由候选竞争链提供视图」的设计意图所在。
//
// ruleset 完全由 height 唯一确定（v1/v2/v3，见 pow.ComputeExpectedBitsAt），view 仅提供祖先块。
func (bc *Blockchain) expectedBitsForView(view pow.ChainView, height int) uint32 {
	want, err := pow.ComputeExpectedBitsAt(view, height, bc.activationHeight)
	if err != nil {
		// 正常主链/fork 视图不会失败；防御性返回最保守的 MaxTargetBits。
		return pow.MaxTargetBits
	}
	return want
}

// expectedBitsFor 计算高度 height 区块的期望难度位，基于本链（活动链）视图。
// 等价于旧 currentBitsLocked，但按 ruleset（v1/v2/v3）分叉（见 pow.ComputeExpectedBitsAt）。
func (bc *Blockchain) expectedBitsFor(height int) uint32 {
	return bc.expectedBitsForView(unsafeView{bc}, height)
}

// medianTimePast 计算高度 parentHeight 的 MTP（过去中位数时间），基于本链（活动链）视图。
func (bc *Blockchain) medianTimePast(parentHeight int) int64 {
	return bc.medianTimePastView(unsafeView{bc}, parentHeight)
}

// medianTimePastView 计算高度 parentHeight 的 MTP，**基于调用方提供的 ChainView**
// （canonical 路径传 unsafeView{bc}，fork 路径传 fork 自身视图）。
func (bc *Blockchain) medianTimePastView(view pow.ChainView, parentHeight int) int64 {
	return pow.MedianTimePastAt(view, parentHeight, bc.activationHeight)
}

// validateVersion 强制硬分叉版本规则（三态，OD-15 §7 冻结，版本号经
// PHASE-P2PCHAIN-V3-CONSENSUS-IMPLEMENTATION-1 修订）：
//   - height < activationHeight：LegacyBlockVersion(1)
//   - activationHeight <= height < 3000：NewBlockVersion(2)
//   - height >= 3000：NewRulesetBlockVersion(4)
func (bc *Blockchain) validateVersion(b *block.Block, height int) error {
	want := pow.VersionForHeight(height, bc.activationHeight)
	if b.Header.Version != want {
		return fmt.Errorf("%w: 区块版本 %d，高度 %d 期望版本 %d", ErrInvalidVersion, b.Header.Version, height, want)
	}
	return nil
}

// validateBits 校验区块难度位等于基于**本链（活动链）**视图计算的期望难度。
// canonical 路径专用；fork 校验路径必须用 validateBitsWithView(forkView, …)（O-3）。
func (bc *Blockchain) validateBits(b *block.Block, height int) error {
	return bc.validateBitsWithView(unsafeView{bc}, b, height)
}

// validateBitsWithView 校验区块难度位等于基于**给定视图**计算的期望难度
// （防止矿工私降/私升难度）。fork 校验路径传入 fork 自身 ChainView（O-3 修复）。
func (bc *Blockchain) validateBitsWithView(view pow.ChainView, b *block.Block, height int) error {
	want := bc.expectedBitsForView(view, height)
	if b.Header.Bits != want {
		return fmt.Errorf("%w: 区块 %d，共识期望 %d", ErrUnexpectedBits, b.Header.Bits, want)
	}
	return nil
}

// validateTimestamp 按激活状态分叉时间戳规则，基于**本链（活动链）**视图。
// canonical 路径专用；fork 校验路径必须用 validateTimestampWithView(forkView, …)（O-3）。
func (bc *Blockchain) validateTimestamp(b *block.Block, tip *block.Block, height int) error {
	return bc.validateTimestampWithView(unsafeView{bc}, b, tip, height)
}

// validateTimestampWithView 按激活状态分叉时间戳规则，**MTP 由给定视图提供**：
//   - 激活前（旧规则，共识 + 中继策略）：不得早于父块（保证难度跨度单调），
//     且不得大幅超前本地时钟（maxFutureTimestampDrift，墙钟仅作中继拒收，非共识分叉源）。
//   - 激活后（新规则，R3 后的完整规则）：MTP(parent) < ts <= MTP(parent)+7200。
//     下界必须严格大于父块 MTP（窗口 [max(0,parentH-10), parentH]），彻底剥离
//     墙钟依赖——timewarp 攻击因「不得 ≤ MTP」被天然防御，且墙钟偏差/恶意时钟
//     都不会造成永久共识分叉；上界封顶时间戳膨胀，防持有出块权者以远未来时间戳
//     拉长难度时间跨度、压低 retarget 难度（见 maxActivationTimestampSlack）。
//
// view 参数是 O-3 修复的关键：canonical 路径传 unsafeView{bc}（其祖先即 bc.blocks），
// fork 路径传 **fork 自身祖先视图**——否则 fork 块的 MTP 会用 canonical 链的祖先计算，
// 与 fork 自身历史不符（跨 ruleset 边界 reorg 时必然误判）。
func (bc *Blockchain) validateTimestampWithView(view pow.ChainView, b *block.Block, tip *block.Block, height int) error {
	if !pow.IsActivationActive(height, bc.activationHeight) {
		if b.Header.Timestamp < tip.Header.Timestamp {
			return fmt.Errorf("%w: 时间戳 %d 早于父块 %d", ErrTimestampOutOfRange, b.Header.Timestamp, tip.Header.Timestamp)
		}
		if b.Header.Timestamp > time.Now().Unix()+maxFutureTimestampDrift {
			return fmt.Errorf("%w: 时间戳 %d 超前本地时钟超过 %d 秒", ErrTimestampOutOfRange, b.Header.Timestamp, maxFutureTimestampDrift)
		}
		return nil
	}
	mtp := bc.medianTimePastView(view, height-1)
	if b.Header.Timestamp <= mtp {
		return fmt.Errorf("%w: 时间戳 %d 未严格大于父块 MTP %d（post-activation 规则）", ErrTimestampOutOfRange, b.Header.Timestamp, mtp)
	}
	if b.Header.Timestamp > mtp+maxActivationTimestampSlack {
		return fmt.Errorf("%w: 时间戳 %d 超过父块 MTP+%d 上界 %d（post-activation 规则）",
			ErrTimestampOutOfRange, b.Header.Timestamp, maxActivationTimestampSlack, mtp+maxActivationTimestampSlack)
	}
	return nil
}

// RequiredVersionFor 返回给定高度区块**必须**使用的版本号（供采矿路径设置候选块版本）。
func (bc *Blockchain) RequiredVersionFor(height int) uint32 {
	return pow.VersionForHeight(height, bc.activationHeight)
}

// clampMiningTimestamp 将候选块时间戳夹到共识合法窗口 [mtp+1, mtp+maxActivationTimestampSlack]
// 内（采矿策略，非共识）：
//   - now < mtp+1：链短暂超前（MTP 追上来了）→ 取 mtp+1（下界）；
//   - mtp+1 <= now <= mtp+slack：取 now（正常路径，贴近真实时间）；
//   - now > mtp+slack：链停滞超过 2 小时后恢复（MTP 远落后于墙钟）→ 取
//     mtp+slack（上界）。没有这一夹取，恢复挖矿的模板会因自己的新上界被
//     validateTimestamp 拒绝——链停滞 >2h 后永远无法再出块（死锁）。
func clampMiningTimestamp(now, mtp int64) int64 {
	ts := now
	if ts < mtp+1 {
		ts = mtp + 1
	}
	if ts > mtp+maxActivationTimestampSlack {
		ts = mtp + maxActivationTimestampSlack
	}
	return ts
}

// MiningTimestamp 返回给定高度候选块应使用的时间戳（采矿策略）：
//   - 激活前：本地当前时间（沿用旧行为）；
//   - 激活后：clamp(本地当前时间, [MTP(parent)+1, MTP(parent)+7200])，
//     保证模板恒满足 post-activation 共识窗口（> MTP 且 ≤ MTP+7200），
//     同时尽量贴近真实时间。
//
// 注意：这是**采矿策略**而非共识——共识窗口由 validateTimestamp 强制，
// 窗口内的具体取值由矿工决定。
func (bc *Blockchain) MiningTimestamp(height int) int64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	ts := time.Now().Unix()
	if pow.IsActivationActive(height, bc.activationHeight) {
		ts = clampMiningTimestamp(ts, bc.medianTimePast(height-1))
	}
	return ts
}
