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
	"math/big"
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

	// 难度不能低于初始最低难度（即 target 不能超过 MaxTarget）
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
	if newBits > MaxTargetBits {
		newBits = MaxTargetBits
	}
	return newBits
}
