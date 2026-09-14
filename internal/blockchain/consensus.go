package blockchain

// consensus.go 封装 PHASE DIFFICULTY-CONSENSUS-IMPLEMENTATION-1 引入的**难度共识硬分叉**
// 校验逻辑：版本强制、时间戳（旧规则墙钟 / 新规则 MTP）、期望难度（按激活高度分叉）。
//
// 这些校验方法都在「调用方已持锁」的前提下运行（AddBlock 持写锁、ValidateBlock/ValidateTemplate
// 持读锁），因此底层访问链数据走**无锁** unsafeView，避免 sync.RWMutex 重入死锁
// （同一 goroutine 持写锁后再 RLock 会永久阻塞）。
//
// 设计要点：所有难度/时间戳/MTP 计算都委托给 pow 包的纯函数（ComputeExpectedBitsAt /
// MedianTimePastAt），仅通过 ChainView 抽象读取祖先——这样未来 reorg 时可由候选竞争链
// 提供视图，而无需改动共识计算本身（DESIGN-1 §FC-004 的 fork-choice 安全要求）。

import (
	"fmt"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
)

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

// expectedBitsFor 计算高度 height 区块的期望难度位，基于本链（活动链）视图。
// 等价于旧 currentBitsLocked，但按激活高度分叉旧/新规则（见 pow.ComputeExpectedBitsAt）。
func (bc *Blockchain) expectedBitsFor(height int) uint32 {
	want, err := pow.ComputeExpectedBitsAt(unsafeView{bc}, height, bc.activationHeight)
	if err != nil {
		// 正常主链视图不会失败；防御性返回最保守的 MaxTargetBits。
		return pow.MaxTargetBits
	}
	return want
}

// medianTimePast 计算高度 parentHeight 的 MTP（过去中位数时间）。
func (bc *Blockchain) medianTimePast(parentHeight int) int64 {
	return pow.MedianTimePastAt(unsafeView{bc}, parentHeight, bc.activationHeight)
}

// validateVersion 强制硬分叉版本规则：
//   - 激活前：区块版本必须为 LegacyBlockVersion（< NewBlockVersion）
//   - 激活后：区块版本必须为 NewBlockVersion（>= 此值）
func (bc *Blockchain) validateVersion(b *block.Block, height int) error {
	want := pow.VersionForHeight(height, bc.activationHeight)
	if b.Header.Version != want {
		return fmt.Errorf("%w: 区块版本 %d，高度 %d 期望版本 %d", ErrInvalidVersion, b.Header.Version, height, want)
	}
	return nil
}

// validateBits 校验区块难度位等于基于本链计算的期望难度（防止矿工私降/私升难度）。
func (bc *Blockchain) validateBits(b *block.Block, height int) error {
	want := bc.expectedBitsFor(height)
	if b.Header.Bits != want {
		return fmt.Errorf("%w: 区块 %d，共识期望 %d", ErrUnexpectedBits, b.Header.Bits, want)
	}
	return nil
}

// validateTimestamp 按激活状态分叉时间戳规则：
//   - 激活前（旧规则，共识 + 中继策略）：不得早于父块（保证难度跨度单调），
//     且不得大幅超前本地时钟（maxFutureTimestampDrift，墙钟仅作中继拒收，非共识分叉源）。
//   - 激活后（新规则）：必须严格大于父块 MTP（窗口 [max(0,parentH-10), parentH]），
//     彻底剥离墙钟依赖——timewarp 攻击因「不得 ≤ MTP」被天然防御，且墙钟偏差/恶意时钟
//     都不会造成永久共识分叉。
func (bc *Blockchain) validateTimestamp(b *block.Block, tip *block.Block, height int) error {
	if !pow.IsActivationActive(height, bc.activationHeight) {
		if b.Header.Timestamp < tip.Header.Timestamp {
			return fmt.Errorf("%w: 时间戳 %d 早于父块 %d", ErrTimestampOutOfRange, b.Header.Timestamp, tip.Header.Timestamp)
		}
		if b.Header.Timestamp > time.Now().Unix()+maxFutureTimestampDrift {
			return fmt.Errorf("%w: 时间戳 %d 超前本地时钟超过 %d 秒", ErrTimestampOutOfRange, b.Header.Timestamp, maxFutureTimestampDrift)
		}
		return nil
	}
	mtp := bc.medianTimePast(height - 1)
	if b.Header.Timestamp <= mtp {
		return fmt.Errorf("%w: 时间戳 %d 未严格大于父块 MTP %d（post-activation 规则）", ErrTimestampOutOfRange, b.Header.Timestamp, mtp)
	}
	return nil
}

// RequiredVersionFor 返回给定高度区块**必须**使用的版本号（供采矿路径设置候选块版本）。
func (bc *Blockchain) RequiredVersionFor(height int) uint32 {
	return pow.VersionForHeight(height, bc.activationHeight)
}

// MiningTimestamp 返回给定高度候选块应使用的时间戳（采矿策略）：
//   - 激活前：本地当前时间（沿用旧行为）；
//   - 激活后：max(本地当前时间, 父块 MTP + 1)，保证满足 post-activation 的
//     Timestamp > MTP(parent) 共识要求，同时尽量贴近真实时间。
//
// 注意：这是**采矿策略**而非共识——共识只要求 > MTP(parent)，具体取值由矿工决定。
func (bc *Blockchain) MiningTimestamp(height int) int64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	ts := time.Now().Unix()
	if pow.IsActivationActive(height, bc.activationHeight) {
		if mtp := bc.medianTimePast(height - 1); mtp+1 > ts {
			ts = mtp + 1
		}
	}
	return ts
}
