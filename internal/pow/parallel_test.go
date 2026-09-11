package pow_test

import (
	"testing"
	"time"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
)

// makeCandidate 构造一个用于挖矿测试的候选区块（时间戳固定，使结果可复现）。
func makeCandidate(bits uint32, ts int64) *block.Block {
	cb := transaction.NewCoinbaseTx([20]byte{0x01}, 50, 1)
	b := block.NewCandidateBlock([32]byte{0xAB}, bits, []*transaction.Transaction{cb})
	b.Header.Timestamp = ts
	b.Header.Nonce = 0
	return b
}

// TestMineParallelFindsValidSolution 并行挖矿结果必须通过 PoW 校验。
//
// 注意：这里刻意不断言「nonce 落在某个 worker 的等差类里」——
// 任何非负整数对 workers 取模都必然小于 workers，该断言恒真、无法证伪；
// 搜索空间划分的正确性由 TestMineParallelSingleWorkerEqualsSequential
// （单 worker 等价串行）与 MineCancelable 的覆盖性设计共同保证。
func TestMineParallelFindsValidSolution(t *testing.T) {
	const workers = 4
	b := makeCandidate(pow.MaxTargetBits, 1700000123)

	found, attempts := pow.MineParallel(b, workers)
	if !found {
		t.Fatal("并行挖矿未找到解")
	}
	if attempts == 0 {
		t.Fatal("尝试次数为 0，说明统计未生效")
	}
	if !pow.Validate(&b.Header) {
		t.Fatalf("并行挖出的 nonce=%d 未通过 PoW 校验", b.Header.Nonce)
	}
}

// TestMineParallelSingleWorkerEqualsSequential workers<=1 时退化为串行实现。
func TestMineParallelSingleWorkerEqualsSequential(t *testing.T) {
	a := makeCandidate(pow.MaxTargetBits, 1700000123)
	b := makeCandidate(pow.MaxTargetBits, 1700000123)

	if found, _ := pow.Mine(a); !found {
		t.Fatal("串行挖矿失败")
	}
	if found, _ := pow.MineParallel(b, 1); !found {
		t.Fatal("单 worker 并行挖矿失败")
	}
	if a.Header.Nonce != b.Header.Nonce || a.Header.Hash() != b.Header.Hash() {
		t.Fatalf("单 worker 结果应与串行一致: nonce %d/%d", a.Header.Nonce, b.Header.Nonce)
	}
}

// TestMineDeterministic 串行挖矿必须确定性（同输入 → 同 nonce / 同哈希）。
// 这是确定性创世区块的前提：所有节点必须得到字节级相同的创世。
func TestMineDeterministic(t *testing.T) {
	a := makeCandidate(pow.MaxTargetBits, 1700000999)
	b := makeCandidate(pow.MaxTargetBits, 1700000999)
	if found, _ := pow.Mine(a); !found {
		t.Fatal("挖矿失败")
	}
	if found, _ := pow.Mine(b); !found {
		t.Fatal("挖矿失败")
	}
	if a.Header.Hash() != b.Header.Hash() {
		t.Fatalf("串行挖矿结果不确定: %s vs %s", a.Header.HashHex(), b.Header.HashHex())
	}
}

// TestMineParallelHigherDifficulty 并行挖矿不设迭代上限，更难的目标也必能解出。
func TestMineParallelHigherDifficulty(t *testing.T) {
	// 用更难一点的难度（bits 更大）验证不会提前放弃
	b := makeCandidate(pow.MaxTargetBits+4, 1700000777)
	found, _ := pow.MineParallel(b, 4)
	if !found {
		t.Fatal("并行挖矿提前放弃")
	}
	if !pow.Validate(&b.Header) {
		t.Fatal("并行挖矿结果不满足难度目标")
	}
}

// TestMineCancelableStopsAllWorkers 关闭 cancel 后，全部并行 worker 必须尽快退出。
//
// 背景难度（bits）远超单机可在测试时限内解出的水平，因此「函数返回」
// 只可能由取消触发——若 worker 泄漏（不检查 cancel），测试会超时失败。
func TestMineCancelableStopsAllWorkers(t *testing.T) {
	b := makeCandidate(48, 1700000555) // 2^48：测试时限内不可能挖出

	cancel := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		found, _ := pow.MineCancelable(b, 4, cancel)
		if found {
			t.Error("难度 2^48 不应在测试时限内被解出")
		}
	}()

	time.Sleep(100 * time.Millisecond) // 让 worker 真正跑起来
	close(cancel)

	select {
	case <-done:
		// 全部 worker 已退出
	case <-time.After(3 * time.Second):
		t.Fatal("关闭 cancel 后 3 秒内并行 worker 未全部退出（goroutine 泄漏）")
	}
}

// TestMineCancelableSequentialPathHonorsCancel 串行路径（workers<=1）同样必须响应取消。
func TestMineCancelableSequentialPathHonorsCancel(t *testing.T) {
	b := makeCandidate(48, 1700000666)

	cancel := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		found, _ := pow.MineCancelable(b, 1, cancel)
		if found {
			t.Error("难度 2^48 不应在测试时限内被解出")
		}
	}()

	time.Sleep(100 * time.Millisecond)
	close(cancel)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("关闭 cancel 后 3 秒内串行挖矿未退出")
	}
}

// TestMineCancelableNilCancelEquivalentToPlainMine cancel 为 nil 时行为与不可取消挖矿一致。
func TestMineCancelableNilCancelEquivalentToPlainMine(t *testing.T) {
	b := makeCandidate(pow.MaxTargetBits, 1700000888)
	found, attempts := pow.MineCancelable(b, 1, nil)
	if !found {
		t.Fatal("nil cancel 时串行挖矿失败")
	}
	if attempts == 0 {
		t.Fatal("尝试次数为 0")
	}
	if !pow.Validate(&b.Header) {
		t.Fatal("结果未通过 PoW 校验")
	}
}
