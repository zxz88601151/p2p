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
	// 选择 2000 是刻意大于当前生产链高度（2026-10-02 实测约 137），保证**存量链在激活前的行为与旧节点
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

// ---- FROZEN CONSENSUS PARAMETER CONTRACT（MNC 系列，MNC-OD-14 冻结 / MNC-OD-15-D 补全） ----
//
// 下列常量是 **第二激活边界（ruleset v3）** 的共识真值。它们是**高度锚定**的：
// 规则集完全由区块高度唯一确定，绝不依赖 canonical tip（OD-15 §4 硬性约束）。
//
// 术语纪律（OD-14/OD-15）：FROZEN = YES / IMPLEMENTED = YES（本阶段落地）/ ACTIVATED = NO。
// 本阶段只实现代码，**不激活网络**：生产链高度（2026-10-02 实测约 137）远低于 3000，存量链行为逐字节不变。
const (
	// NewRulesetActivationHeight 是第二激活高度（H_nearest），ruleset v3 的**起点高度**。
	//
	// 冻结值 = 3000（MNC-OD-15-D 正式冻结）。约束核验（OD-15-D §3.1）：
	//   - > 2000（严格后于 v2 边界，不复用 ActivationHeight）；
	//   - > 当前生产链高（2026-10-02 实测约 137），留 2863 块余量；
	//   - 为 DifficultyAdjustmentInterval(20) 的整数倍（3000/20=150），注入后首个周期干净；
	//   - v2 稳定观察区间 [2000,3000) = 1000 块 = 50 周期。
	//
	// 语义：h == 3000 是「新规则起点」而非「旧规则终点」（与 IsActivationActive 的
	// 「激活块自身用新规则」一致）。注入点高度锚定，跨分支确定，无 fork-choice 歧义。
	NewRulesetActivationHeight = 3000

	// NewRulesetInitialBits 是 ruleset v3 的**初始难度位**（独立参数，MNC-OD-15 §4 拆分）。
	//
	// 冻结值 = 27。它与 MaxTargetBits=16 **不是同一个参数**：
	//   - MaxTargetBits=16 继续承担 genesis 难度 / v1·v2 AdjustBits 起点 / difficulty floor 三重角色（零改动）；
	//   - NewRulesetInitialBits=27 只在 h == NewRulesetActivationHeight 处**一次性注入**
	//     （见 ComputeExpectedBitsAt），用于把 v2（Ceil）漂移造成的 overshoot（~28，OD-08）
	//     在切换瞬间校正回 Nearest 稳态（OD-10 实证 27 为稳态收敛点）。
	//
	// 关键：**绝不允许把 MaxTargetBits 改成 27**，也**绝不允许**让 27 外溢到 genesis
	// （genesis 恒 16，由 ComputeExpectedBitsAt 的 height==0 分支保证）。
	NewRulesetInitialBits uint32 = 27

	// NewRulesetBlockVersion 是 ruleset v3 区块**必须**使用的版本号。
	//
	// 冻结值 = 3（常量名源自 OD-15 §191）。版本三态：v1(<2000) / v2([2000,3000)) / v3(>=3000)。
	// 旧节点（v2 二进制）对 v3 区块做**双重拒绝**：
	//   1. VersionForHeight 返回 2 ≠ 3 → ErrInvalidVersion；
	//   2. 即使版本校验被绕过，旧节点用 Ceil 算出的 expected bits ≠ 新块 Nearest bits → ErrUnexpectedBits。
	NewRulesetBlockVersion uint32 = 3
)

// targetBitWidth 是难度目标的位宽：target = 2^(targetBitWidth-bits)。
//
// 它同时定义了「不触发移位回绕」的 bits 上界：位移量按 uint32 计算，
// bits > targetBitWidth 时 targetBitWidth-bits 会回绕成 2^32 量级（见 BitsToTarget）。
const targetBitWidth = 256

// IsBitsInConsensusDomain 报告 bits 是否落在本链共识认可的难度域 [1, 256]。
//
// 该域**不是本阶段新引入的协议边界**，而是补齐既有不变量：
//   - internal/blocktree：`bits == 0 || bits > 256 → ErrInvalidBits`
//     （注释原文：「防止移位溢出 / 零工作量」）；
//   - internal/storage：workOfBits 采用同一域判定（ErrInvalidBits）；
//   - internal/pow：合法区块的 bits 必须等于期望难度（∈ [1, MaxDifficultyBits=32]），
//     且 32 ≤ 256 ⇒ **一切合法区块都落在本域内**。
//
// 用途：作为区块校验中**先于目标构造与 PoW** 的廉价前置闸门，使对端可控的
// bits 在进入任何大整数构造之前被拒绝（PHASE F-4-CONSENSUS-INPUT-HARDENING-REMEDIATION）。
func IsBitsInConsensusDomain(bits uint32) bool {
	return bits >= 1 && bits <= targetBitWidth
}

// BitsToTarget 将压缩格式的难度（Bits）还原为大整数目标值。
// 简化实现：这里假设 bits 直接表示目标值前导零的位数（而不是比特币真实的浮点式编码），
// 便于初期理解和调试；后续可以替换成与比特币兼容的 nBits 编码。
//
// F-4 输入加固：位移量由 uint32 算术得出 —— bits > 256 时 `256-bits` 回绕成
// 2^32 量级，big.Int 会为**单次调用**构造 2^32 bit（512 MiB）的中间值
// （实测 155–212 ms/次；≤1 MiB 消息即可触发），构成远程内存放大面。
// 因此在越界时直接返回**零目标**（任何哈希都不满足 ⇒ PoW 恒失败），不构造大整数。
// 对全部合法 bits（共识域 [1,256]）本函数输出与加固前逐字节一致（§7 保真）。
func BitsToTarget(bits uint32) *big.Int {
	if bits > targetBitWidth {
		return new(big.Int)
	}
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

// adjustTargetCore 是难度调整的**共享核心**（OD-15-C §6 O-4：shared core + 双入口）。
//
// 它完成与取整模式无关的三步：
//  1. 幅度 clamp：actualTimespan 夹到 [expected/4, expected*4]（防难度失控）；
//  2. newTarget = currentTarget × actualTimespan / expected（方向推导，无分支）；
//  3. floor clamp：newTarget > MaxTarget() 时报告 floorHit（难度低于下限）。
//
// 取整（Ceil / Nearest）与 ceiling clamp 由两个入口函数分别完成，避免重复 clamp 逻辑。
// 返回值：floorHit=true 时 newTarget 为 nil（调用方须直接返回 MaxTargetBits）。
func adjustTargetCore(currentBits uint32, actualTimespanSeconds int64) (newTarget *big.Int, floorHit bool) {
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
	newTarget = new(big.Int).Mul(currentTarget, big.NewInt(actualTimespanSeconds))
	newTarget.Div(newTarget, big.NewInt(expected))

	// 难度下限钳制（作用于 target，先于任何取整）：难度不能低于初始最低难度。
	if newTarget.Cmp(MaxTarget()) == 1 {
		return nil, true
	}
	return newTarget, false
}

// ceilBitsFromTarget 把难度目标 newTarget 换算为 **Ceil** 语义的 bits。
//
// 逆变换用 257-BitLen：T(b)=2^(256-b) 的 BitLen 恰为 257-b，
// 因此 257-BitLen 可让 target→bits→target 在 2 的幂处精确还原（均衡态难度不变）；
// 对一般 target 则向上（偏难）取整。数学意义 = ceil(256 - log2(newTarget))。
func ceilBitsFromTarget(newTarget *big.Int) uint32 {
	return uint32(257 - newTarget.BitLen())
}

// nearestBitsFromTarget 把难度目标 newTarget 换算为 **Nearest** 语义的 bits
// （round(256 - log2(newTarget))），全程整数运算、**不含任何浮点**（OD-13 §7F / OD-15 §6.4）。
//
// 推导（确定性，可独立复核）：
//
//	设 b_cont = 256 - log2(t)，b0 = ceil(b_cont) = 257 - BitLen(t)（= 现行 Ceil 值）。
//	写 t = m·2^(n-1)，n = BitLen(t)，m ∈ [1,2)，则 b_cont = b0 - log2(m)，log2(m) ∈ [0,1)。
//	Nearest（round-half-up）判据：log2(m) ≤ 0.5 → b0；log2(m) > 0.5 → b0-1。
//	log2(m) ≤ 0.5  ⟺  m ≤ √2  ⟺  m² ≤ 2  ⟺  t² ≤ 2^(2n-1) = 2^(513-2·b0)。
//
// ⇒ **t² ≤ 2^(513-2·b0) 时取 b0，否则取 b0-1。**
//
// 相邻 bits 的几何中点 M = √(T(b0-1)·T(b0)) = 2^(256.5-b0) 是无理数，而 t 恒为精确整数
// （整数×整数÷整数）⇒ t 永不等 M ⇒ **tie 不可达**；为完备性，等号规范为 round-half-up
// （t ≤ M → b0）。
//
// 无溢出/无下溢：floor clamp 在调用前已执行 ⇒ newTarget ≤ 2^240 ⇒ b0 ≥ 16 ⇒ 指数 513-2·b0 ≥ 481 > 0；
// 共识域内 newTarget ≥ 2^222（currentBits ≤ 32 且 timespan ≥ expected/4）⇒ b0 ≤ 34，指数恒安全。
// 故 2^(513-2·b0) 用 big.Int.Lsh 精确构造，t² 亦为精确大整数，比较无浮点误差。
func nearestBitsFromTarget(newTarget *big.Int) uint32 {
	if newTarget.Sign() <= 0 {
		return 0
	}
	b0 := 257 - newTarget.BitLen() // = ceil(b_cont) = 现行 Ceil 值
	exp := 513 - 2*b0
	if exp <= 0 {
		// 防御性：正常共识域（floor clamp 之后）不可达。target 极小 ⇒ 直接取 b0。
		return uint32(b0)
	}
	t2 := new(big.Int).Mul(newTarget, newTarget)      // t²
	mid := new(big.Int).Lsh(big.NewInt(1), uint(exp)) // 2^(513-2·b0)
	if t2.Cmp(mid) <= 0 {
		return uint32(b0)
	}
	return uint32(b0 - 1)
}

// AdjustBits 根据最近一个难度调整周期实际耗费的时间，计算下一周期的难度（Bits）。
//
// 逻辑与比特币一致：
//   - 若实际用时比期望用时短（矿工太多/算力太强），提高难度（增大 bits）
//   - 若实际用时比期望用时长（矿工太少/算力下降），降低难度（减小 bits）
//
// 为避免难度剧烈波动，比特币将单次调整幅度限制在 4 倍以内，这里同样做了限制。
//
// **本函数是 ruleset v1/v2 的 Ceil 取整入口**（OD-15-C §6 O-4：保留 Ceil 入口向后兼容，
// 现有 pow_test.go 的 Ceil 断言逐字节不变）。ruleset v3 使用 AdjustBitsNearest。
//
// 输出随后经过两层**有意的**钳制（见 MaxDifficultyBits 的说明）：
//   - 难度下限：target 不得超过 T(MaxTargetBits)，即 bits 不得小于 MaxTargetBits；
//   - 难度上限：bits 不得超过 MaxDifficultyBits。
//
// 执行顺序（冻结，不可换位）：① floor clamp（target）→ ② 取整（bits）→ ③ ceiling clamp（bits）。
//
// 注意：方向推导（newTarget ∝ actualTimespan）在钳制前在数学上是正确且无分支的，
// 钳制只压缩「链上可达的动态范围」，不改变推导本身。该语义由
// TestAdjustBitsDirection / TestDifficultyAdjustmentBounds 与本包的链级测试共同锁定。
func AdjustBits(currentBits uint32, actualTimespanSeconds int64) uint32 {
	newTarget, floorHit := adjustTargetCore(currentBits, actualTimespanSeconds)
	if floorHit {
		return MaxTargetBits
	}
	newBits := ceilBitsFromTarget(newTarget)
	if newBits < 1 {
		newBits = 1
	}
	// 难度上限钳制：post-activation 时本链 MaxDifficultyBits=32，难度可在 [1,32] 内浮动。
	if newBits > MaxDifficultyBits {
		newBits = MaxDifficultyBits
	}
	return newBits
}

// AdjustBitsNearest 是 ruleset v3（FROZEN CONSENSUS，h >= NewRulesetActivationHeight）的
// **Nearest** 取整入口：与 AdjustBits 共享同一 core（幅度 clamp + newTarget + floor clamp），
// 仅取整步骤不同（Nearest 作用于 b_cont，见 nearestBitsFromTarget）。
//
// 动机（OD-08/OD-10）：Ceil 对小幅扰动系统性 +1（overshoot），Nearest 抑制该偏差。
// 预期行为差异（非冲突）：newTarget ∈ (2^239.5, 2^240.5) 时 Ceil 给 17、Nearest 给 16。
//
// 执行顺序与 AdjustBits 一致（冻结）：① floor clamp → ② Nearest → ③ ceiling clamp。
func AdjustBitsNearest(currentBits uint32, actualTimespanSeconds int64) uint32 {
	newTarget, floorHit := adjustTargetCore(currentBits, actualTimespanSeconds)
	if floorHit {
		return MaxTargetBits
	}
	newBits := nearestBitsFromTarget(newTarget)
	if newBits < 1 {
		newBits = 1
	}
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

// IsActivationActive 判断给定高度是否已处于 v2 共识规则（难度浮动 + MTP 时间戳 + 新版本）。
//
// 约定：激活块自身（height == ActivationHeight）即使用新规则——激活高度是「新规则起点」，
// 而非「旧规则终点」。高度 0（创世）始终视为未激活（创世永远用 MaxTargetBits）。
//
// 注意：本函数**只管 v2 边界（ActivationHeight，默认 2000）**，用于版本 v1/v2 分界与
// MTP 时间戳规则；ruleset v3（Nearest + 注入）的第二边界由 IsNewRulesetActive 单独判定。
func IsActivationActive(height, activationHeight int) bool {
	return height >= activationHeight && height > 0
}

// IsNewRulesetActive 判断给定高度是否已处于 ruleset v3（FROZEN CONSENSUS：Nearest 取整 +
// 新版本位），边界为固定的 NewRulesetActivationHeight（= 3000）。
//
// 高度锚定：完全由 height 决定，与 activationHeight / canonical tip 无关。
// height 0（创世）恒未激活（genesis 永为 MaxTargetBits=16）。
func IsNewRulesetActive(height int) bool {
	return height >= NewRulesetActivationHeight && height > 0
}

// VersionForHeight 返回给定高度区块**必须**使用的版本号（三态，OD-15 §7 冻结）。
//
//	height <  activationHeight              → LegacyBlockVersion    (v1)
//	activationHeight <= height < 3000       → NewBlockVersion       (v2)
//	height >= 3000（且已激活）               → NewRulesetBlockVersion (v3)
//
// 版本互斥构成结构性硬分叉（DESIGN-1 已证软分叉不可行）：旧节点（v2 二进制）对 v3 区块
// 返回 2 ≠ 3 → ErrInvalidVersion 确定性拒绝。
func VersionForHeight(height, activationHeight int) uint32 {
	if IsActivationActive(height, activationHeight) {
		if IsNewRulesetActive(height) {
			return NewRulesetBlockVersion
		}
		return NewBlockVersion
	}
	return LegacyBlockVersion
}

// ComputeExpectedBitsAt 按共识规则独立计算「高度 height 的区块应当使用的难度位」，
// 完全基于 view 提供的候选链自身祖先（绝不依赖外部活动链尾 / canonical tip）。
//
// 规则（三态，冻结于 DESIGN-1 / GATE-1 / MNC-OD-14 / MNC-OD-15-D）：
//   - height == 0：创世，固定 MaxTargetBits(16)。（**genesis 恒 16，注入 27 绝不外溢**）
//   - 0 < height < ActivationHeight（ruleset v1，LEGACY）：难度沿用父块 bits
//     （有效链上恒为 MaxTargetBits=16，由创世归纳保证），即旧链「难度不浮动」语义的精确等价。
//   - ActivationHeight <= height < 3000（ruleset v2，Ceil 浮动）：
//     非周期边界沿用父块 bits；周期边界 AdjustBits（**Ceil**）钳制在 [16,32]。
//   - height == 3000（ruleset v3 起点）：**无条件**返回 NewRulesetInitialBits(27)
//     （一次性注入，先于周期边界判断）——把 v2 的 Ceil overshoot 校正回 Nearest 稳态。
//   - height > 3000（ruleset v3，Nearest 浮动）：
//     非周期边界沿用父块 bits；周期边界 AdjustBitsNearest（**Nearest**）钳制在 [16,32]。
//
// 高度锚定：ruleset 完全由 height 唯一确定，与 canonical tip 无关 ⇒ 历史块回放 / 跨分支
// 校验 / 新节点同步逐高度使用正确 ruleset，**无 retroactive reinterpretation**。
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

	// ruleset v3（FROZEN CONSENSUS）：Nearest 取整 + 一次性注入。
	if IsNewRulesetActive(height) {
		if height == NewRulesetActivationHeight {
			// 一次性注入（无条件，先于周期边界判断）：高度锚定、跨分支确定。
			return NewRulesetInitialBits, nil
		}
		return retargetAtBoundary(view, parent, height, AdjustBitsNearest)
	}

	// ruleset v1（钉死 16）/ v2（Ceil 浮动）由 v2 激活边界分叉。
	if !IsActivationActive(height, activationHeight) {
		return parent.Header.Bits, nil
	}
	return retargetAtBoundary(view, parent, height, AdjustBits)
}

// retargetAtBoundary 在难度调整周期边界处按给定取整函数重算 bits；非边界沿用父块 bits。
// 抽出以避免 v2(Ceil)/v3(Nearest) 两条分支重复周期窗口逻辑（单一事实源）。
func retargetAtBoundary(view ChainView, parent *block.Block, height int, round func(uint32, int64) uint32) (uint32, error) {
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
	return round(parent.Header.Bits, actualTimespan), nil
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
