// Package pow 实现工作量证明（Proof of Work）挖矿与难度调整算法。
//
// 挖矿的本质：不断修改区块头中的 Nonce（以及必要时的时间戳/coinbase extra nonce），
// 使区块头哈希值在数值上小于当前难度目标（Target）。由于哈希函数的不可预测性，
// 唯一的办法就是暴力枚举，这也是"工作量"的来源。
package pow

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"sync/atomic"

	"p2pchain/internal/block"
)

const (
	// MaxTargetBits 表示最低难度对应的目标值的前导零位数（可按需调整）。
	// 数值越小，初始难度越低，适合个人 CPU 挖矿测试网。
	//
	// 取 16 是测试网调优：期望需枚举 2^16 ≈ 6.5 万次哈希才能出一个区块，
	// 实测 CPU 约 1.1 M hash/s → 单块 ~60ms，PoW 仍然真实（不是「必中」），
	// 同时让全量回归测试保持秒级。这是本链的共识参数，改动会改变创世区块哈希，
	// 因此一旦有节点长期运行就不可随意调整。
	MaxTargetBits = 16

	// TargetBlockTimeSeconds 期望的平均出块间隔（秒）。
	// 比特币是 600 秒（10分钟），测试阶段可以设置更短，比如 30-60 秒，便于调试。
	TargetBlockTimeSeconds = 60

	// DifficultyAdjustmentInterval 每隔多少个区块重新计算一次难度。
	// 比特币是 2016，个人项目初期建议设置得小一些（如 20~50）以便更快看到难度变化效果。
	DifficultyAdjustmentInterval = 20

	// MaxDifficultyBits 是本链难度（bits）的**浮动上限**。
	//
	// 历史：本链早期以 CPU 毫秒级出块为目标（见 MaxTargetBits 注释），曾把难度**有意钉死**
	// 在最低难度（MaxDifficultyBits == MaxTargetBits == 16），使难度不浮动。
	//
	// 现经 PHASE DIFFICULTY-CONSENSUS-DESIGN-1 + PRE-IMPLEMENTATION-GATE-1 审计，
	// 在**固定激活高度硬分叉**（见下方 ActivationHeight）之后解除钉死：难度可按 AdjustBits
	// 公式在 [1, MaxDifficultyBits] 内真实浮动，上限抬至 32（单块枚举 2^32 次哈希 ≈ 数千秒，
	// 给难度足够的上行空间，又不至于在一两个周期内失控）。
	//
	// 硬分叉语义：height < ActivationHeight 仍走「旧规则」（难度钉死 MaxTargetBits=16、
	// 版本 < NewBlockVersion）；height >= ActivationHeight 才启用本浮动规则。
	// 因此本常量只影响 post-activation 的链，存量（pre-activation）链行为不变。
	MaxDifficultyBits = 32
)

// ---- 难度共识硬分叉激活参数（PHASE DIFFICULTY-CONSENSUS-IMPLEMENTATION-1） ----

const (
	// ActivationHeight 是难度浮动 + 新时间戳/MTP 规则 + 新版本强制生效的**固定激活高度**。
	//
	// 选择 2000 是刻意大于当前生产链高度（约 1275），保证**存量链在激活前的行为与旧节点
	// 逐字节等价**——已落盘的 blocks.dat 回放时不触发任何新规则（旧块全部 bits=16、
	// version=1、时间戳满足旧共识区间），因此无需任何迁移或重挖。
	//
	// 这是结构性硬分叉：旧节点（ActivationHeight 尚未定义/仍钉死 16）在 height >= 2000 处
	// 会拒绝新块（新块 version>=2 且时间戳/难度走新规则），新旧节点确定性分叉。
	// 软分叉不可行（DESIGN-1 已形式化证明），故采用固定高度硬分叉。
	ActivationHeight = 2000

	// LegacyBlockVersion 是激活前区块必须使用的版本号（< NewBlockVersion）。
	LegacyBlockVersion uint32 = 1

	// NewBlockVersion 是激活后区块必须使用的版本号（>= 此值）。
	NewBlockVersion uint32 = 2
)

// BitsToTarget 将压缩格式的难度（Bits）还原为大整数目标值。
// 简化实现：这里假设 bits 直接表示目标值前导零的位数（而不是比特币真实的浮点式编码），
// 便于初期理解和调试；后续可以替换成与比特币兼容的 nBits 编码。
func BitsToTarget(bits uint32) *big.Int {
	target := big.NewInt(1)
	target.Lsh(target, uint(256-bits))
	return target
}

// MaxTarget 返回初始（最低）难度对应的目标值。
func MaxTarget() *big.Int {
	return BitsToTarget(MaxTargetBits)
}

// Validate 校验区块头的哈希是否满足难度目标：hash 数值必须小于 target。
func Validate(h *block.Header) bool {
	target := BitsToTarget(h.Bits)
	hash := h.Hash()
	hashInt := new(big.Int).SetBytes(hash[:])
	return hashInt.Cmp(target) == -1
}

// Mine 串行（单线程）搜索工作量证明解。
//
// 确定性：从 nonce=0 递增、返回第一个满足难度目标的解，因此同输入必然得到
// 同一个 Nonce 与同一个区块哈希。这是「全网络字节级相同的创世区块」的前提，
// 所以创世区块必须走这条路径。
//
// 需要并行加速用 MineParallel；需要在链尾变化时立即放弃用 MineCancelable。
//
// 性能实现（与朴素实现结果完全一致）：
//  1. 头序列化长度固定、Nonce 位于末尾 8 字节——预计算前缀后每次迭代只改写 Nonce；
//  2. 目标值与哈希都用 32 字节大端定长表示，循环内直接做字节序比较，
//     避免每轮迭代都构造 big.Int（原实现的主要分配热点）。
//
// 关键热路径上不做任何堆分配，仅两次 sha256.Sum256（栈上数组）。
func Mine(b *block.Block) (found bool, attempts uint64) {
	return MineCancelable(b, 1, nil)
}

// targetToBytes 把难度目标换算为 32 字节大端定长表示，供循环内做无分配比较。
// bits 过小（目标 ≥ 2^256）时用全 0xff 填充，等价于「任何哈希都满足」，避免 FillBytes panic。
func targetToBytes(target *big.Int) [32]byte {
	var out [32]byte
	if target.Sign() <= 0 {
		return out // 目标为 0：任何哈希都不满足
	}
	if target.BitLen() > 256 {
		for i := range out {
			out[i] = 0xff
		}
		return out
	}
	target.FillBytes(out[:])
	return out
}

// MineParallel 用 workers 个 goroutine 并行搜索 Nonce（不可取消，等价于 MineCancelable(..., nil)）。
//
// 与 Mine 的重要差异：**不保证返回最小的合法 Nonce**，谁先命中谁生效，
// 因此同一次挖矿在不同机器/不同轮次可能得到不同的 Nonce 与区块哈希。
// 任何要求字节级可复现的场合（例如确定性创世区块）必须使用串行的 Mine。
func MineParallel(b *block.Block, workers int) (found bool, attempts uint64) {
	return MineCancelable(b, workers, nil)
}

// MineCancelable 是可取消的挖矿入口：关闭 cancel 通道即可让全部 worker 尽快退出。
//
// 用途：节点的链尾一旦变化，正在求解的候选区块立即作废。若不能取消，
// 并行 worker 会一直算到命中为止——既白烧 CPU，也会泄漏 goroutine。
//
// workers <= 1 时走串行路径（顺序、结果确定）；workers > 1 时按等差类切分搜索空间。
// cancel 为 nil 表示不取消。
func MineCancelable(b *block.Block, workers int, cancel <-chan struct{}) (found bool, attempts uint64) {
	targetBytes := targetToBytes(BitsToTarget(b.Header.Bits))

	if workers <= 1 {
		return mineSequential(b, targetBytes, cancel)
	}

	// 在启动 worker 之前一次性序列化头部，之后 worker 只操作各自的副本，
	// 完全不触碰 b.Header —— 避免「某个 worker 已写入 Nonce、其它 worker 仍在读头部」
	// 造成的数据竞态（race detector 会直接报错）。
	prefix := b.Header.SerializeHeader()
	nonceOff := len(prefix) - 8

	var (
		wg       sync.WaitGroup
		winner   atomic.Bool
		winNonce atomic.Uint64
		tries    atomic.Uint64
	)

	// 搜索空间切分：worker w 只尝试 nonce ∈ {w, w+workers, w+2*workers, ...}，
	// 整体覆盖与串行完全一致（不重复、不遗漏），加速比接近 workers。
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(start uint64) {
			defer wg.Done()
			buf := make([]byte, len(prefix))
			copy(buf, prefix)

			var n uint64
			for nonce := start; !winner.Load(); nonce += uint64(workers) {
				if isCancelled(cancel) {
					break
				}
				binary.LittleEndian.PutUint64(buf[nonceOff:], nonce)
				first := sha256.Sum256(buf)
				hash := sha256.Sum256(first[:])
				n++
				// 同为 32 字节定长时，bytes.Compare 的字典序等价于大整数的数值比较
				if bytes.Compare(hash[:], targetBytes[:]) < 0 {
					// CompareAndSwap 保证只有一个 worker 成为赢家
					if winner.CompareAndSwap(false, true) {
						winNonce.Store(nonce)
					}
					break
				}
			}
			tries.Add(n)
		}(uint64(w))
	}
	wg.Wait()

	if !winner.Load() {
		return false, tries.Load()
	}
	b.Header.Nonce = winNonce.Load()
	return true, tries.Load()
}

// mineSequential 串行搜索：从 nonce=0 起递增，返回第一个满足目标的解。
// 顺序确定，因此同输入必然得到同一个 Nonce（确定性创世依赖这一点）。
// cancel 每轮检查一次（一次非阻塞 select，相对两次 SHA-256 可忽略）。
func mineSequential(b *block.Block, targetBytes [32]byte, cancel <-chan struct{}) (bool, uint64) {
	prefix := b.Header.SerializeHeader()
	buf := make([]byte, len(prefix))
	copy(buf, prefix)
	nonceOff := len(prefix) - 8

	var nonce uint64
	for {
		if isCancelled(cancel) {
			return false, nonce
		}
		binary.LittleEndian.PutUint64(buf[nonceOff:], nonce)
		first := sha256.Sum256(buf)
		hash := sha256.Sum256(first[:])
		if bytes.Compare(hash[:], targetBytes[:]) < 0 {
			b.Header.Nonce = nonce
			return true, nonce + 1
		}
		nonce++
	}
}

// isCancelled 非阻塞判断取消通道是否已关闭；nil 通道视为永不取消。
func isCancelled(cancel <-chan struct{}) bool {
	if cancel == nil {
		return false
	}
	select {
	case <-cancel:
		return true
	default:
		return false
	}
}

// AdjustBits 根据最近一个难度调整周期实际耗费的时间，计算下一周期的难度（Bits）。
//
// 逻辑与比特币一致：
//   - 若实际用时比期望用时短（矿工太多/算力太强），提高难度（增大 bits）
//   - 若实际用时比期望用时长（矿工太少/算力下降），降低难度（减小 bits）
//
// 为避免难度剧烈波动，比特币将单次调整幅度限制在 4 倍以内，这里同样做了限制。
//
// 输出随后经过两层**有意的**钳制（见 MaxDifficultyBits 的说明）：
//   - 难度下限：target 不得超过 T(MaxTargetBits)，即 bits 不得小于 MaxTargetBits；
//   - 难度上限：bits 不得超过 MaxDifficultyBits（本链 == MaxTargetBits，故难度固定）。
//
// 注意：方向推导（newTarget ∝ actualTimespan）在钳制前在数学上是正确且无分支的，
// 钳制只压缩「链上可达的动态范围」，不改变推导本身。该语义由
// TestAdjustBitsDirection / TestDifficultyAdjustmentBounds 与本包的链级测试共同锁定。
func AdjustBits(currentBits uint32, actualTimespanSeconds int64) uint32 {
	expected := int64(TargetBlockTimeSeconds * DifficultyAdjustmentInterval)

	// 限制调整幅度在 [expected/4, expected*4] 之间，防止极端值造成难度失控
	minTimespan := expected / 4
	maxTimespan := expected * 4
	if actualTimespanSeconds < minTimespan {
		actualTimespanSeconds = minTimespan
	}
	if actualTimespanSeconds > maxTimespan {
		actualTimespanSeconds = maxTimespan
	}

	currentTarget := BitsToTarget(currentBits)
	newTarget := new(big.Int).Mul(currentTarget, big.NewInt(actualTimespanSeconds))
	newTarget.Div(newTarget, big.NewInt(expected))

	// 难度下限钳制：难度不能低于初始最低难度（即 target 不能超过 MaxTarget）。
	if newTarget.Cmp(MaxTarget()) == 1 {
		return MaxTargetBits
	}

	// 将 newTarget 换算回近似的 bits：找到使 2^(256-bits) 最接近 newTarget 的 bits。
	// 逆变换用 257-BitLen：T(b)=2^(256-b) 的 BitLen 恰为 257-b，
	// 因此 257-BitLen 可让 target→bits→target 在 2 的幂处精确还原（均衡态难度不变）；
	// 对一般 target 则向下保守取整（T(bits') ≤ newTarget，永不比计算值更易）。
	// 旧公式 256-BitLen 会在均衡态把 bits 低估 1（如 20→19），造成每周期难度系统性变易。
	newBits := uint32(257 - newTarget.BitLen())
	if newBits < 1 {
		newBits = 1
	}
	// 难度上限钳制：post-activation 时本链 MaxDifficultyBits=32，难度可在 [1,32] 内浮动。
	if newBits > MaxDifficultyBits {
		newBits = MaxDifficultyBits
	}
	return newBits

}

// ---- 难度共识：工作量度量、链视图与激活门控（PHASE DIFFICULTY-CONSENSUS-IMPLEMENTATION-1） ----

// ChainView 是「一条链的只读视图」抽象，使难度/时间戳/MTP 计算既能作用于**主链**
// （*blockchain.Blockchain 天然满足），也能作用于**候选竞争链**（未来 reorg 时由
// BlockTree 提供），从而让 expectedBits / MTP 始终基于「候选区块自身的祖先」而非
// 任意活动链尾——这是 fork-choice 安全的前提（DESIGN-1 §FC-004 闭环要求）。
//
// 注意：必须是按高度 O(1) 随机访问的视图；窗口 [max(0,h-10)..h] 的 MTP 计算依赖此。
type ChainView interface {
	BlockByHeight(height int) (*block.Block, error)
	Height() int
}

// WorkOfBits 返回难度位 bits 对应的**期望工作量（期望 PoW 成本）**。
//
// 本链的 target = 2^(256-bits) 是 2 的整数次幂 ⇒ 满足 target 的哈希期望尝试次数
// 严格等于 2^bits（无取整误差）。因此 Work(bits) = 2^bits 既是真实期望成本，
// 也是 fork-choice 的链工作量度量（CumulativeWork = Σ 2^bits_i）。
//
// 术语：用「期望工作量 / 期望 PoW 成本」而非「保证 2^bits 次哈希」——哈希是随机变量，
// 任何单块的实际尝试次数可能远低于或高于期望值，但**期望**严格为 2^bits，长期累加后
// 期望工作量之差即难度之差，足以作为共识度量的稳定指标。使用 *big.Int 避免 uint64 溢出
// （bits 可达 32 ⇒ 2^32 仍在 uint64 内，但 Σ 跨数千块必然溢出，故全程 big.Int）。
func WorkOfBits(bits uint32) *big.Int {
	w := big.NewInt(1)
	if bits >= 256 {
		// bits>=256 ⇒ target>=2^0=1 ⇒ 任何哈希都满足；赋予极大值代表「零难度」，
		// 但本链 MaxDifficultyBits=32，正常路径不会到达；此处仅防御除零/越界。
		w.Lsh(w, 255)
		return w
	}
	w.Lsh(w, uint(bits))
	return w
}

// IsActivationActive 判断给定高度是否已处于新共识规则（难度浮动 + MTP 时间戳 + 新版本）。
//
// 约定：激活块自身（height == ActivationHeight）即使用新规则——激活高度是「新规则起点」，
// 而非「旧规则终点」。高度 0（创世）始终视为未激活（创世永远用 MaxTargetBits）。
func IsActivationActive(height, activationHeight int) bool {
	return height >= activationHeight && height > 0
}

// VersionForHeight 返回给定高度区块**必须**使用的版本号。
//
// 硬分叉版本强制：激活前必须用 LegacyBlockVersion（< NewBlockVersion），激活后必须
// 用 NewBlockVersion（>= 此值）。二者互斥构成结构性分叉（DESIGN-1 已证软分叉不可行）。
func VersionForHeight(height, activationHeight int) uint32 {
	if IsActivationActive(height, activationHeight) {
		return NewBlockVersion
	}
	return LegacyBlockVersion
}

// ComputeExpectedBitsAt 按共识规则独立计算「高度 height 的区块应当使用的难度位」，
// 完全基于 view 提供的候选链自身祖先（绝不依赖外部活动链尾）。
//
// 规则（冻结于 DESIGN-1 / GATE-1）：
//   - height == 0：创世，固定 MaxTargetBits。
//   - height < ActivationHeight（旧规则）：难度钉死在父块 bits（= MaxTargetBits），
//     此即旧链「难度不浮动」语义的精确等价（旧链无论是否周期边界，结果恒为 16）。
//   - height >= ActivationHeight（新规则）：
//       · 非周期边界（height % DifficultyAdjustmentInterval != 0）：沿用父块 bits；
//       · 周期边界：以 [height-Interval, height-1] 的实际时间跨度调用 AdjustBits，
//         结果钳制在 [1, MaxDifficultyBits]（现 32）内浮动。
//
// 返回的错误仅在 view 无法提供所需祖先块时产生（如 height-1 越界），正常路径恒为 nil。
func ComputeExpectedBitsAt(view ChainView, height, activationHeight int) (uint32, error) {
	if height == 0 {
		return MaxTargetBits, nil
	}
	parent, err := view.BlockByHeight(height - 1)
	if err != nil {
		return 0, fmt.Errorf("计算期望难度：读取父块（高度 %d）失败: %w", height-1, err)
	}
	if !IsActivationActive(height, activationHeight) {
		// 旧规则：钉死在父块难度（= MaxTargetBits）。等价于旧 currentBitsLocked 的全部分支。
		return parent.Header.Bits, nil
	}
	if height%DifficultyAdjustmentInterval != 0 {
		return parent.Header.Bits, nil
	}
	periodStartHeight := height - DifficultyAdjustmentInterval
	if periodStartHeight < 0 {
		periodStartHeight = 0
	}
	periodStart, err := view.BlockByHeight(periodStartHeight)
	if err != nil {
		return 0, fmt.Errorf("计算期望难度：读取周期起点（高度 %d）失败: %w", periodStartHeight, err)
	}
	actualTimespan := parent.Header.Timestamp - periodStart.Header.Timestamp
	return AdjustBits(parent.Header.Bits, actualTimespan), nil
}

// MedianTimePastAt 计算高度 h 的「过去中位数时间」（MTP）。
//
// 定义（冻结于 DESIGN-1）：窗口为 [max(0, h-10), h] 共至多 11 个区块的时间戳，
// 取中位数；median 下标 k = n/2（Go sort 升序后 n/2 为「上半中位数」，偶数个时偏上，
// 与比特币一致）。h < 0 视为空视图返回 0（调用方不应传入）。
//
// 共识用途（仅 post-activation）：新块时间戳必须满足 Timestamp > MTP(h-1)，
// 从而（1）保证时间戳单调非减（MTP(h-1) >= 父块时间戳），（2）天然防御 timewarp 攻击
// （矿工无法把时间戳设到低于父链最近 11 块的中位数之下），（3）**不依赖任何墙钟**，
// 因此墙钟偏差/恶意时钟都不会造成永久共识分叉。
func MedianTimePastAt(view ChainView, h, activationHeight int) int64 {
	if h < 0 {
		return 0
	}
	// 冻结设计（DESIGN-1）：窗口为 [max(0, h-10), h] 共至多 11 个区块（比特币等价 MTP）。
	// 注意：必须是 h-10，而非 h-11——后者会构成 12 块窗口，是偏离冻结设计的共识参数错误，
	// 会在任何窗口内存在非单调时间戳时令不同节点计算出不同的 MTP，造成永久分叉。
	start := h - 10
	if start < 0 {
		start = 0
	}
	n := h - start + 1
	ts := make([]int64, 0, n)
	for i := start; i <= h; i++ {
		b, err := view.BlockByHeight(i)
		if err != nil {
			// 视图缺口：以 0 填充（正常主链视图不会到达此处；防御性）。
			ts = append(ts, 0)
			continue
		}
		ts = append(ts, b.Header.Timestamp)
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i] < ts[j] })
	k := len(ts) / 2
	return ts[k]
}

// ChainCumulativeWork 返回从创世（高度 0）累积到 height（含）的总**期望工作量**，
// 即 fork-choice 的链工作量度量：CumulativeWork = Σ_{i=0}^{height} WorkOfBits(bits_i)。
//
// 使用 *big.Int 累加，跨数千块不会溢出（单个 2^bits 在 bits=32 时为 2^32 ≈ 4e9，
// 仍落 uint64，但 Σ 必然越界，故全程 big.Int）。本函数是 reorg 时「最大累积工作量」
// 判据的权威数据源（与 BlockTree.CumulativeWork 同源，DESIGN-1 §FC-004 闭环）。
func ChainCumulativeWork(view ChainView, height int) (*big.Int, error) {
	if height < 0 {
		return big.NewInt(0), nil
	}
	total := big.NewInt(0)
	for i := 0; i <= height; i++ {
		b, err := view.BlockByHeight(i)
		if err != nil {
			return nil, fmt.Errorf("累积工作量：读取高度 %d 失败: %w", i, err)
		}
		total.Add(total, WorkOfBits(b.Header.Bits))
	}
	return total, nil
}
