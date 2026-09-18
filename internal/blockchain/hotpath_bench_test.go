// PHASE P2PCHAIN — HOTPATH BENCHMARK FOUNDATION
//
// 本文件为 **benchmark-only** 资产，由
// "PHASE P2PCHAIN — HOTPATH BENCHMARK FOUNDATION / PRIORITY AUDIT" 授权创建。
//
// 边界声明（严格遵守授权 §1 / §12）：
//   - 不修改任何 production 实现、consensus 行为、locking、observability、schema；
//   - 不修改 blockchain.go / query.go / findBlockLocked / blockAtHash / hashIndex / canonicalContains；
//   - 只新增测试侧 fixture 与 benchmark 函数；
//   - hashIndex 相关测量为「只读引用测量」与「测试侧 counterfactual」，**不写回 production**。
//
// Fixture 性质：真实 PoW 挖矿构造（bits = pow.MaxTargetBits = 16），
// 经 production `AddBlock` 路径入库（因此 hashIndex / canonicalSet / tree 均按生产规则维护）。
// 存储使用 `MemoryBlockStore`（无磁盘 I/O），detached 块经 `SaveBlock` 直接写入。

package blockchain

import (
	"fmt"
	"sync"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
)

// ---------------------------------------------------------------------------
// sinks：防止编译器消除被测量表达式（benchmark 卫生，§10）
// ---------------------------------------------------------------------------

var (
	benchSinkBlock *block.Block
	benchSinkOK    bool
	benchSinkHash  [32]byte
	benchSinkSlice []*block.Block
	benchSinkSet   *utxo.UTXOSet
)

// benchHeights 为 O(h) / O(h×U) 的 scaling 观测维度（§5.5 / §7）。
var benchHeights = []int{8, 64, 256, 1024}

// cloneHeights 为 clone replay 的观测维度。h=1024 的单次 op 成本过高
// （O(h×U) 且 U≈h），受基准运行时间约束排除，仅以 8/64/256 观察 scaling。
var cloneHeights = []int{8, 64, 256}

// ---------------------------------------------------------------------------
// Fixture（按高度缓存，构造成本不计入计时）
// ---------------------------------------------------------------------------

type benchFixture struct {
	bc     *Blockchain
	store  *storage.MemoryBlockStore
	blocks []*block.Block // canonical，按高度升序（含 genesis）
	// detachedHash 是「已落盘但非 canonical」的区块哈希，用于 store fallback 测量。
	detachedHash [32]byte
}

var (
	benchCacheMu sync.Mutex
	benchCache   = map[int]*benchFixture{}
)

// benchFixtureAt 返回高度 h 的缓存 fixture；首次调用时构造（计时暂停）。
func benchFixtureAt(b *testing.B, h int) *benchFixture {
	b.Helper()
	benchCacheMu.Lock()
	defer benchCacheMu.Unlock()
	if f, ok := benchCache[h]; ok {
		return f
	}

	b.StopTimer()
	defer b.StartTimer()

	genesis := NewGenesisBlock()
	bc, err := NewBlockchainWithGenesis(genesis)
	if err != nil {
		b.Fatalf("bench fixture: new chain: %v", err)
	}
	store := storage.NewMemoryBlockStore()
	bc.store = store
	if err := store.SaveBlock(genesis); err != nil {
		b.Fatalf("bench fixture: save genesis: %v", err)
	}

	blocks := []*block.Block{genesis}
	for i := 0; i < h; i++ {
		tip, err := bc.Tip()
		if err != nil {
			b.Fatalf("bench fixture: tip: %v", err)
		}
		cb := transaction.NewCoinbaseTx([20]byte{1}, utxo.Subsidy(len(bc.blocks)), len(bc.blocks))
		nb := block.NewCandidateBlock(tip.Header.Hash(), pow.MaxTargetBits, []*transaction.Transaction{cb})
		if found, _ := pow.Mine(nb); !found {
			b.Fatalf("bench fixture: mine failed at h=%d", i+1)
		}
		if err := bc.AddBlock(nb); err != nil {
			b.Fatalf("bench fixture: add block h=%d: %v", i+1, err)
		}
		blocks = append(blocks, nb)
	}

	// detached：从 tip 分叉出一枚块，仅写入 store，不进 canonical 链。
	tip := blocks[len(blocks)-1]
	cb := transaction.NewCoinbaseTx([20]byte{9}, utxo.Subsidy(len(bc.blocks)), len(bc.blocks))
	fork := block.NewCandidateBlock(tip.Header.Hash(), pow.MaxTargetBits, []*transaction.Transaction{cb})
	if found, _ := pow.Mine(fork); !found {
		b.Fatalf("bench fixture: mine detached failed")
	}
	if err := store.SaveBlock(fork); err != nil {
		b.Fatalf("bench fixture: save detached: %v", err)
	}

	f := &benchFixture{bc: bc, store: store, blocks: blocks, detachedHash: fork.Header.Hash()}
	benchCache[h] = f
	return f
}

// absentHash 是一个确定不在链上、也不在 store 中的哈希（miss 路径）。
func absentHash() [32]byte {
	var h [32]byte
	for i := range h {
		h[i] = 0xA5
	}
	return h
}

// ---------------------------------------------------------------------------
// §5 A. findBlockLocked —— 线性扫描成本测量
//
// 说明：以下 Raw 变体直接调用 findBlockLocked 而**不持锁**，用于隔离
// 「线性扫描 + Header.Hash() 重算」本身的成本；持锁版本见
// BenchmarkBlockByHash_HitLast（production 调用路径，含 RLock 开销）。
// ---------------------------------------------------------------------------

func BenchmarkFindBlockLockedRaw_HitFirst(b *testing.B) {
	for _, h := range benchHeights {
		b.Run(fmt.Sprintf("h=%d", h), func(b *testing.B) {
			f := benchFixtureAt(b, h)
			target := f.blocks[0].Header.Hash() // 最佳情况：首个元素命中
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchSinkBlock, benchSinkOK = f.bc.findBlockLocked(target)
			}
		})
	}
}

func BenchmarkFindBlockLockedRaw_HitLast(b *testing.B) {
	for _, h := range benchHeights {
		b.Run(fmt.Sprintf("h=%d", h), func(b *testing.B) {
			f := benchFixtureAt(b, h)
			target := f.blocks[len(f.blocks)-1].Header.Hash() // 最坏情况：全链扫描后命中
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchSinkBlock, benchSinkOK = f.bc.findBlockLocked(target)
			}
		})
	}
}

func BenchmarkFindBlockLockedRaw_MissAbsent(b *testing.B) {
	for _, h := range benchHeights {
		b.Run(fmt.Sprintf("h=%d", h), func(b *testing.B) {
			f := benchFixtureAt(b, h)
			target := absentHash() // 全链扫描 + store 未命中
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchSinkBlock, benchSinkOK = f.bc.findBlockLocked(target)
			}
		})
	}
}

func BenchmarkFindBlockLockedRaw_DetachedFallback(b *testing.B) {
	for _, h := range benchHeights {
		b.Run(fmt.Sprintf("h=%d", h), func(b *testing.B) {
			f := benchFixtureAt(b, h)
			target := f.detachedHash // 全链扫描 miss → store fallback 命中
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchSinkBlock, benchSinkOK = f.bc.findBlockLocked(target)
			}
		})
	}
}

// BenchmarkHeaderHash 隔离单次 Header.Hash() 成本（= 线性扫描每迭代的核心成本）。
// 结构事实：Header.Hash() = sha256(sha256(SerializeHeader()))，**无记忆化**，
// 故 findBlockLocked 每次迭代均重算。
func BenchmarkHeaderHash(b *testing.B) {
	f := benchFixtureAt(b, 8)
	hdr := f.blocks[len(f.blocks)-1].Header
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSinkHash = hdr.Hash()
	}
}

// BenchmarkBlockByHash_HitLast 测量 production 调用路径（RLock + findBlockLocked）。
func BenchmarkBlockByHash_HitLast(b *testing.B) {
	for _, h := range benchHeights {
		b.Run(fmt.Sprintf("h=%d", h), func(b *testing.B) {
			f := benchFixtureAt(b, h)
			target := f.blocks[len(f.blocks)-1].Header.Hash()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchSinkBlock, benchSinkOK = f.bc.BlockByHash(target)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// §6 HASH-INDEX COUNTERFACTUAL（测试侧，只读，不写回 production）
//
// 两个变体：
//   - BenchmarkHashIndexRawLookup：只读 production bc.hashIndex 的 map 查找（不经任何函数）；
//   - BenchmarkBlockAtHash_HitLast：production O(1) 路径 blockAtHash
//     ⚠️ 该函数内部执行 obsBlockLookups.Add(1)（I0/§9 计数器）；
//        本 benchmark 会使其递增，但仅为进程内计数，**不改变语义/schema**。
// ---------------------------------------------------------------------------

func BenchmarkHashIndexRawLookup(b *testing.B) {
	for _, h := range benchHeights {
		b.Run(fmt.Sprintf("h=%d", h), func(b *testing.B) {
			f := benchFixtureAt(b, h)
			target := f.blocks[len(f.blocks)-1].Header.Hash()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchSinkBlock = f.bc.hashIndex[target]
			}
		})
	}
}

func BenchmarkBlockAtHash_HitLast(b *testing.B) {
	for _, h := range benchHeights {
		b.Run(fmt.Sprintf("h=%d", h), func(b *testing.B) {
			f := benchFixtureAt(b, h)
			target := f.blocks[len(f.blocks)-1].Header.Hash()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				blk, err := f.bc.blockAtHash(target)
				if err != nil {
					b.Fatalf("blockAtHash: %v", err)
				}
				benchSinkBlock = blk
			}
		})
	}
}

// ---------------------------------------------------------------------------
// §5.8 P2P-like workload：BlockByHashWithAncestors（OnGetBlockByHash 服务路径）
// 该函数在祖先回溯循环中重复调用 findBlockLocked ⇒ 单次请求 O(n·h)。
// ---------------------------------------------------------------------------

func BenchmarkBlockByHashWithAncestors(b *testing.B) {
	for _, h := range benchHeights {
		n := h
		if n > 64 {
			n = 64 // 与 production MaxBranchAncestors=64 对齐
		}
		b.Run(fmt.Sprintf("h=%d_n=%d", h, n), func(b *testing.B) {
			f := benchFixtureAt(b, h)
			target := f.blocks[len(f.blocks)-1].Header.Hash()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchSinkSlice = f.bc.BlockByHashWithAncestors(target, n)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// §7 CLONE REPLAY：utxoAtNode（O(h × U)）
// 目标不是证明某个数字，而是观察 scaling / allocation / CPU time。
// ---------------------------------------------------------------------------

func BenchmarkUtxoAtNode_Tip(b *testing.B) {
	for _, h := range cloneHeights {
		b.Run(fmt.Sprintf("h=%d", h), func(b *testing.B) {
			f := benchFixtureAt(b, h)
			node := f.bc.tree.BestTip()
			if node == nil {
				b.Fatal("bench: nil best tip")
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				set, err := f.bc.utxoAtNode(node)
				if err != nil {
					b.Fatalf("utxoAtNode: %v", err)
				}
				benchSinkSet = set
			}
		})
	}
}
