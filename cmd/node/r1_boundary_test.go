package main

// 本文件是 PHASE P2PCHAIN — ORPHAN RESOURCE-BOUNDARY REMEDIATION R-1 的回归测试：
// F-1 每父子块配额（Q=64）· F-3 孤儿按 Header.Hash() 去重 · F-4 入队前的父无关校验 ·
// F-7 既有 obs 事件路径下的 waiting / inflight 观测。
//
// 所有断言都基于**可观察计数**（waiting keys / waiting blocks / parkedHashes 规模 / 链高 / 链尾），
// 不依赖 RSS。

import (
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
)

// ---- 断言辅助（读同一把 svc.mu） ----

func waitingCounts(t *testing.T, svc *nodeService) (keys, blocks, parked int) {
	t.Helper()
	svc.mu.Lock()
	defer svc.mu.Unlock()
	k, b, _ := svc.waitingSnapshotLocked()
	return k, b, len(svc.parkedHashes)
}

func assertWaitingCounts(t *testing.T, svc *nodeService, wantKeys, wantBlocks int, msg string) {
	t.Helper()
	keys, blocks, _ := waitingCounts(t, svc)
	if keys != wantKeys || blocks != wantBlocks {
		t.Fatalf("%s: waiting_keys=%d waiting_blocks=%d, want keys=%d blocks=%d",
			msg, keys, blocks, wantKeys, wantBlocks)
	}
}

// siblingOn 以不同收款地址在同一父块上挖出**不同哈希**的兄弟块（用于配额边界）。
// 若同一秒 + 同地址 + 同高度会产出完全相同的块，那就测不到"第 65 个不同孤儿"；用不同 pkh 保证互不相同。
func siblingOn(t *testing.T, svc *nodeService, parent *block.Block, height int, tag byte) *block.Block {
	t.Helper()
	return mineOnTo(t, svc, parent, height, mkPkh(tag))
}

// mkPkh 构造一个以 tag 为首字节的确定性收款地址（保证不同 tag ⇒ 不同哈希的块）。
func mkPkh(tag byte) [20]byte {
	var pkh [20]byte
	pkh[0] = tag
	for i := 1; i < 20; i++ {
		pkh[i] = byte(i)
	}
	return pkh
}

// ============================================================ F-1

// TestR1F1PerParentQuotaBoundary 验证每个父哈希的子块配额 Q=64 的边界语义。
func TestR1F1PerParentQuotaBoundary(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	// p 不会被投递 ⇒ 其子块全部以 orphan 身份入队（共享同一个父哈希）。
	p := mineOn(t, svc, g, 1)

	// 第 1..64 个不同子块：全部保留（配额未到）。
	for i := 1; i <= maxWaitingChildrenPerParent-1; i++ {
		deliverBroadcast(t, svc, siblingOn(t, svc, p, 2, byte(i)))
	}
	assertWaitingCounts(t, svc, 1, maxWaitingChildrenPerParent-1, "63 个不同子块应全部保留（Q-1 边界）")

	// 第 64 个：仍保留（== Q）。
	deliverBroadcast(t, svc, siblingOn(t, svc, p, 2, byte(maxWaitingChildrenPerParent)))
	assertWaitingCounts(t, svc, 1, maxWaitingChildrenPerParent, "第 64 个子块仍应保留（== Q）")

	// 第 65 个：超过配额 ⇒ 丢弃，计数不增长。
	deliverBroadcast(t, svc, siblingOn(t, svc, p, 2, byte(maxWaitingChildrenPerParent+1)))
	assertWaitingCounts(t, svc, 1, maxWaitingChildrenPerParent, "第 65 个子块应被配额丢弃（计数不变）")

	// Invariant B：总数上限 = 256 × Q（这里是计数界，不表示字节）。
	// 这里只验证"每键不超过 Q"这一实现前提；全局键上限另见 TestR1GlobalKeyCeilingPreserved。
	if got := svc.waitingChildren(p.Header.Hash()); got != maxWaitingChildrenPerParent {
		t.Fatalf("children(p) = %d, want %d", got, maxWaitingChildrenPerParent)
	}

	// 配额丢弃后，父块到达仍能正常释放并级联（B-1 语义不受影响）。
	deliverBroadcast(t, svc, p) // p 的父是 genesis（已知）⇒ 应用 p；随后级联释放 64 个子块。
	// 64 个子块高度均为 2，彼此竞争同一位置，最终只有一个成为 canonical。
	assertHeight(t, svc, 2, "父块到达后应完成解析并推进到高度 2")
	keys, blocks, parked := waitingCounts(t, svc)
	if keys != 0 || blocks != 0 || parked != 0 {
		t.Fatalf("释放后应清空：keys=%d blocks=%d parked=%d", keys, blocks, parked)
	}
}

// TestR1F1QuotaIsPerKeyNotGlobal 确认 Q=64 是**每键**配额，不是全局 64（防止误实现成全局上限）。
func TestR1F1QuotaIsPerKeyNotGlobal(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	// 三个不同的（不会被投递的）父块 ⇒ 三个不同的 waiting 键。
	// 注意：必须用不同收款地址（pkh）——同父同高同地址在 determinstic 挖掘下
	// 会产出完全相同的块哈希，测试前提即失效。
	p1 := mineOnTo(t, svc, g, 1, mkPkh(1))
	p2 := mineOnTo(t, svc, g, 1, mkPkh(2))
	p3 := mineOnTo(t, svc, g, 1, mkPkh(3))
	if p1.Header.Hash() == p2.Header.Hash() ||
		p2.Header.Hash() == p3.Header.Hash() ||
		p1.Header.Hash() == p3.Header.Hash() {
		t.Fatal("测试前提不成立：三个父块哈希应互不相同")
	}
	for i, p := range []*block.Block{p1, p2, p3} {
		// 每个键放 10 个不同子块（远小于 Q），总计 30 个。
		for j := 1; j <= 10; j++ {
			deliverBroadcast(t, svc, siblingOn(t, svc, p, 2, byte(i*20+j)))
		}
	}
	// 全局 30 > 64? 否。但每键 10 <= 64。
	assertWaitingCounts(t, svc, 3, 30, "每键 10 个、共 3 个键应全部保留（Q 是每键而非全局）")
}

// ============================================================ F-3

// TestR1F3DuplicateOrphanNotStored 重复投递同一孤儿不得放大存储。
func TestR1F3DuplicateOrphanNotStored(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	p := mineOn(t, svc, g, 1)
	c := mineOn(t, svc, p, 2)

	for i := 0; i < 20; i++ {
		deliverBroadcast(t, svc, c)
	}
	// 20 次重复投递 ⇒ 仍然只有 1 个 waiting 条目、1 个哈希索引。
	assertWaitingCounts(t, svc, 1, 1, "20 次重复投递应只保留 1 个等待条目")
	_, _, parked := waitingCounts(t, svc)
	if parked != 1 {
		t.Fatalf("parkedHashes = %d, want 1", parked)
	}
	// 重复投递也不得触发额外的 by-hash 请求。该性质在结构上由 F-3 保证：
	// dedup 命中后**先于 requestBranch 返回**，重复到达根本不会进入请求路径。
	// 说明：branchReqSent 计数器只在 SendTo 成功后自增（service.go requestBranch 尾部），
	// 本单元 harness 的假对端未连接 ⇒ 计数恒为 0、不可观测；跨进程真实连接下的
	// 请求行为由 p2p_branch_test.go（真实连接）覆盖。故此处不作计数断言。
	// 父块到达后正常释放，索引同步删除（允许之后再次入队）。
	deliverBroadcast(t, svc, p)
	assertHeight(t, svc, 2, "父块到达后子块应被解析")
	_, _, parked = waitingCounts(t, svc)
	if parked != 0 {
		t.Fatalf("释放后 parkedHashes = %d, want 0", parked)
	}
}

// TestR1F3DuplicateAfterRejection 被 F-4 拒收的候选重复到达后仍为 0 条目（无隐藏增长）。
func TestR1F3DuplicateAfterRejection(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	p := mineOn(t, svc, g, 1)
	bad := mineOn(t, svc, p, 2)
	bad.Header.Nonce++

	for i := 0; i < 5; i++ {
		deliverBroadcast(t, svc, bad)
	}
	assertWaitingCounts(t, svc, 0, 0, "被拒收的候选重复到达仍应为 0 条目")
	// F-4 拒收路径先于 requestBranch 返回 ⇒ 被拒候选根本不进入请求路径。
	// （branchReqSent 在本 harness 不可观测，理由同 TestR1F3DuplicateOrphanNotStored 的注释。）
}

// ============================================================ F-4

// TestR1F4PreParkRejectionReasons 逐个验证授权的四项父无关校验，并确认合法孤儿通过。
func TestR1F4PreParkRejectionReasons(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	p := mineOn(t, svc, g, 1)
	good := mineOn(t, svc, p, 2)

	// 1) 体积：追加输出直到规范序列化超过 MaxBlockSize。
	if got := preParkRejectReason(good); got != "" {
		t.Fatalf("合法孤儿的 preParkRejectReason = %q, want \"\"", got)
	}
	oversize := mineOn(t, svc, p, 2)
	oversize.Transactions[0].Outputs = append(oversize.Transactions[0].Outputs, transaction.TxOutput{Value: 1})
	// 性能约束（R1-F4 修复）：Block.Size() = len(Block.Encode())，而 Encode() 每次都会
	// 重新分配并序列化**整块**，代价 O(块字节数)。原实现在内层循环里逐次调用 Size()，
	// 但内层期间 oversize.Transactions[0].Outputs 尚未回写（回写在循环之后），
	// 因此内层条件恒取同一个陈旧值 ⇒ 内层实际退化为「固定做 appendN 次全块序列化」，
	// 总代价 O(N²)（1 MiB 块 ≈ 数十 GB 序列化 + GC 抖动），使该用例挂死数分钟，
	// 并连带拖死整个 `go test ./...`。
	// 现改为：内层只做追加（摊还 O(1)），每轮外层仅做一次 Size() 检查；
	// 输出数几何倍增，快速越过阈值，总代价 O(N)。最终块与修复前逐字节一致。
	for oversize.Size() <= blockchain.MaxBlockSize {
		outs := oversize.Transactions[0].Outputs
		// 每轮翻倍，快速越过 1 MiB 阈值（n 在进入内层时一次性求值）。
		for i, n := 0, len(outs); i < n; i++ {
			outs = append(outs, transaction.TxOutput{Value: 1})
		}
		oversize.Transactions[0].Outputs = outs
	}
	if got := preParkRejectReason(oversize); got != "size" {
		t.Fatalf("超大区块的 preParkRejectReason = %q, want \"size\"", got)
	}

	// 2) bits 共识域。
	badBits := mineOn(t, svc, p, 2)
	badBits.Header.Bits = 0 // 越出 [1, targetBitWidth]
	if got := preParkRejectReason(badBits); got != "bits" {
		t.Fatalf("越界 bits 的 preParkRejectReason = %q, want \"bits\"", got)
	}

	// 3) 默克尔不一致。
	badMerkle := mineOn(t, svc, p, 2)
	badMerkle.Header.MerkleRoot[0] ^= 0xFF

	if got := preParkRejectReason(badMerkle); got != "merkle" {
		t.Fatalf("默克尔不一致的 preParkRejectReason = %q, want \"merkle\"", got)
	}

	// 4) PoW 不成立。
	badPoW := mineOn(t, svc, p, 2)
	badPoW.Header.Nonce++
	if got := preParkRejectReason(badPoW); got != "pow" {
		t.Fatalf("PoW 不成立的 preParkRejectReason = %q, want \"pow\"", got)
	}
}

// TestR1F4LegitimateOrphanStillParked 合法孤儿（父缺失）仍然入队——F-4 不得误杀。
func TestR1F4LegitimateOrphanStillParked(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	p := mineOn(t, svc, g, 1)
	c := mineOn(t, svc, p, 2)

	deliverBroadcast(t, svc, c) // c 的父 p 未到达 ⇒ 合法孤儿
	assertWaitingCounts(t, svc, 1, 1, "合法孤儿（父缺失）仍应入队")
	assertParked(t, svc, p.Header.Hash(), 1, "合法孤儿应挂在父哈希下")

	// 父到达后由 B-1 级联释放。
	deliverBroadcast(t, svc, p)
	assertHeight(t, svc, 2, "父到达后应完成解析")
	assertWaitingEmpty(t, svc, "解析后应清空")
}

// ============================================================ 释放 / 级联 / 全局键上限

// TestR1ParkedHashPrunedOnRelease 释放时哈希索引必须同步删除（不残留、不重复计数）。
func TestR1ParkedHashPrunedOnRelease(t *testing.T) {
	svc := newB1Service(t)
	g, err := svc.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	a := mineOn(t, svc, g, 1)
	b := mineOn(t, svc, a, 2)
	c := mineOn(t, svc, b, 3)

	// 乱序：c、b 先后入队（各自父未知），再补齐 a ⇒ 一次级联放两级。
	deliverBroadcast(t, svc, c)
	deliverBroadcast(t, svc, b)
	assertWaitingCounts(t, svc, 2, 2, "乱序送达后应有 2 个键、2 个条目")
	deliverBroadcast(t, svc, a)
	assertHeight(t, svc, 3, "补齐根缺口后应解析到高度 3")
	_, _, parked := waitingCounts(t, svc)
	if parked != 0 {
		t.Fatalf("释放后 parkedHashes = %d, want 0", parked)
	}
}

// TestR1GlobalKeyCeilingPreserved 既有 256 个不同父哈希的全局上限保持不变（Q 不得被误实现成全局上限）。
func TestR1GlobalKeyCeilingPreserved(t *testing.T) {
	producer := newB1Service(t)
	g, err := producer.chain.Tip()
	if err != nil {
		t.Fatal(err)
	}
	// 造一条 258 块的链，用它的第 2..258 块作为"257 个父哈希互不相同"的孤儿候选。
	chain := make([]*block.Block, 0, 258)
	parent := g
	for i := 1; i <= 258; i++ {
		nb := mineOn(t, producer, parent, i)
		chain = append(chain, nb)
		parent = nb
	}
	if len(chain) != 258 {
		t.Fatalf("chain len = %d, want 258", len(chain))
	}
	svc := newB1Service(t)
	// 注入第 2..258 块（父分别是第 1..257 块，互不重复 ⇒ 257 个不同父键，超过 256 上限。
	for i := 1; i < len(chain); i++ { // chain[1] == 第 2 块
		deliverBroadcast(t, svc, chain[i])
	}
	// 既有全局键上限：256 个键被保留，超出的被丢弃。
	_ = utxo.Subsidy(1)
	keys, blocks, parked := waitingCounts(t, svc)
	if keys != maxWaitingBlocks {
		t.Fatalf("waiting keys = %d, want %d（既有 256 键上限必须保持不变）", keys, maxWaitingBlocks)
	}
	if blocks != maxWaitingBlocks {
		t.Fatalf("waiting blocks = %d, want %d", blocks, maxWaitingBlocks)
	}
	if parked != maxWaitingBlocks {
		t.Fatalf("parkedHashes = %d, want %d", parked, maxWaitingBlocks)
	}
}
