package mempool_test

import (
	"strconv"
	"testing"

	"p2pchain/internal/mempool"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

// benchFixture 构造 n 笔独立、已签名、fee>0 的交易及其 UTXO 基础集。
// 全部构造须在计时区外完成（BenchmarkAdmission 用 b.StopTimer 包住本函数）。
func benchFixture(b *testing.B, n int) (*utxo.UTXOSet, []*transaction.Transaction) {
	b.Helper()
	w, err := wallet.NewWallet()
	if err != nil {
		b.Fatalf("生成钱包失败: %v", err)
	}
	base := utxo.NewUTXOSet()
	txs := make([]*transaction.Transaction, 0, n)
	for i := 0; i < n; i++ {
		var txid [32]byte
		txid[0] = byte(i)
		txid[1] = byte(i >> 8)
		txid[2] = byte(i >> 16)
		op := utxo.OutPoint{Hash: txid}
		base.Add(op, utxo.Entry{Value: 100, PubKeyHash: w.PubKeyHash(), Height: 0})

		tx := &transaction.Transaction{
			Inputs:  []transaction.TxInput{{PrevTxHash: txid, OutIndex: 0}},
			Outputs: []transaction.TxOutput{{Value: 90, PubKeyHash: w.PubKeyHash()}}, // fee 10
		}
		sig, err := w.Sign(tx.Hash())
		if err != nil {
			b.Fatalf("签名失败: %v", err)
		}
		tx.Inputs[0].Signature = sig
		tx.Inputs[0].PubKey = w.PublicKey
		txs = append(txs, tx)
	}
	return base, txs
}

// BenchmarkAdmission 批量入池基准：向空池逐笔 Add n 笔独立 fee>0 交易。
// 旧实现（composite view 每笔重建）为 O(N²)；R4 共享视图后为 O(N)。
// 运行方式：go test ./internal/mempool/ -bench BenchmarkAdmission -benchtime=1x
func BenchmarkAdmission(b *testing.B) {
	for _, n := range []int{50, 100, 500, 1000, 2000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				base, txs := benchFixture(b, n)
				pool := mempool.New(0) // 容量不限：纯 admission 复杂度，不掺驱逐
				b.StartTimer()

				for _, tx := range txs {
					if err := pool.Add(base, tx, 1); err != nil {
						b.Fatalf("第 %d 轮批量入池失败: %v", i, err)
					}
				}
				if got := pool.Len(); got != n {
					b.Fatalf("池长度 = %d, want %d", got, n)
				}
			}
		})
	}
}
