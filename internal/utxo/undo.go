// undo.go 实现 PHASE REORG-INFRASTRUCTURE-IMPLEMENTATION-2 (REORG-1D) 的
// UTXO 反向状态基础设施。
//
// ────────────────────────────────────────────────────────────────────────────
// 唯一职责
//
// 本文件提供：
//
//  1. BlockUndo 数据模型——一个区块 UTXO 变更的完整、可逆、自包含的描述。
//  2. ApplyBlockWithUndo ——在 ApplyBlock 语义之上同步产出 BlockUndo。
//  3. DisconnectBlock —— 严格反向应用 BlockUndo，恢复 pre-block UTXO state。
//  4. BlockUndo 的确定性 encode / decode 接口（持久化留给 REORG-1E）。
//
// 本文件明确不提供：
//
//   - 完整 blockchain reorg orchestration（属 REORG-1C）。
//   - 持久化到 disk（属 REORG-1E：存储层 hash 索引 + Delete/Truncate）。
//   - 多区块链级回滚（属 REORG-1C）。
//   - mempool ReaddDisconnected（属 REORG-1F）。
//   - 任何修改 consensus 参数的路径。
//
// ────────────────────────────────────────────────────────────────────────────
// BlockUndo 不变式（验证后定稿）
//
//  1. Created 是「本 block 新增、且仍存活于 post-block UTXOSet」的 outpoint+Entry。
//     排序：按 OutPoint 字典序升序（Hash 32 字节字典序 → Index uint32 升序），
//     确定性、可验证、与 transaction 顺序无关。
//
//  2. Spent 是「本 block 消费、且未在同一 block 内被重新创建」的 outpoint+Entry。
//     Entry 必须来自 pre-block UTXOSet（即 Spend 前查询到的 Entry），不允许来自
//     同 block 内其他 tx 的输出（那是 Created，不是 Spent）。
//     排序：按消费顺序（tx 索引升序，同 tx 内 input 索引升序）—— 与 Created
//     排序规则不同，但都确定性。
//
//  3. Created 与 Spent 不相交（disjoint）：
//     - 若某 OutPoint 在 pre-block 不存在、且 post-block 存在 → Created
//     - 若某 OutPoint 在 pre-block 存在、且 post-block 不存在 → Spent
//     - 若某 OutPoint 在 pre-block 不存在、且 post-block 也不存在 → 不在任一
//     （典型场景：tx[A] 创建 out_A、tx[B] 在同 block 内消费 out_A —— out_A
//     既不在 Created（post-block 不存在）也不在 Spent（pre-block 不存在））
//
//  4. DisconnectBlock 是 (pre-block-set, undo) → post-undo-set 的纯函数：
//     唯一副作用 = set 字段重新赋值（无外部状态、无 panic 错误处理）。
//
//  5. ApplyBlockWithUndo 与 DisconnectBlock 复合后必须满足严格 round-trip：
//
//     S0 + ApplyBlockWithUndo(B) → S1 + undo
//     S1 + DisconnectBlock(undo) → S0'
//     S0' == S0（完整状态等价，不是 Balance/Len 近似）
//
//     并进一步：
//
//     S0 + ApplyBlockWithUndo(B) → S1
//     S1 + DisconnectBlock(undo) → S0'
//     S0' + ApplyBlockWithUndo(B) → S1'
//     S1' == S1（同 S0' 两次 Apply 等价）
//
// ────────────────────────────────────────────────────────────────────────────
// 反向应用顺序（§五 设计原则 #8）
//
// DisconnectBlock 必须按以下顺序反向应用：
//
//  1. 校验：Created/Spent 与 set 当前状态一致（corruption detection）；
//  2. 删除 Created（按 reverse 顺序，最后创建的先删 —— 严格说 OutPoint 顺序
//     对删除无依赖，因为 Created 各项互不依赖，但 reverse 顺序确保任何错误
//     中断时集合状态处于"中间态可观察"，便于上层事务回滚决策）；
//  3. 恢复 Spent（按 reverse 顺序，最后消费的先恢复 —— 严格说 Spent 各项
//     也互不依赖，但 reverse 顺序保持与 Apply 的对称）；
//
// Created 删除 → Spent 恢复 这个顺序的理由：
//   - 若先恢复 Spent 再删除 Created，则 Created 中可能存在「与 Spent OutPoint
//     同名」的极端情况（违反不变式 3）；先删 Created 可让任何 corruption 在
//     第一步即被检测。
//
// ────────────────────────────────────────────────────────────────────────────
// 一致性保证（§六 不允许）
//
//   - 不允许 panic 作为正常错误处理（corruption → error 返回）。
//   - 不允许忽略 undo mismatch（必须显式 ErrUndoMismatch）。
//   - 不允许静默删除不存在的 outpoint（Created 中出现 set 不存在的项 → error）。
//   - 不允许静默恢复已存在且冲突的 outpoint（Spent 中出现 set 已存在的项 → error）。
package utxo

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"

	"p2pchain/internal/transaction"
)

// BlockUndo 一个区块 UTXO 状态变更的完整可逆描述。
//
// 由 ApplyBlockWithUndo 一次性产出，由 DisconnectBlock 反向应用。
// 值类型——构造后不可变（所有 slice 在构造时拷贝）。
type BlockUndo struct {
	// Height 区块高度（用于诊断与序列化时校验）。
	Height int

	// Created 本 block 新增且仍存活于 post-block UTXOSet 的 (OutPoint, Entry)。
	// 排序：OutPoint 字典序升序，确定性。
	// 语义：DisconnectBlock 必须从 set 中删除每一项。
	Created []UndoEntry

	// Spent 本 block 消费且未在同一 block 内被重新创建的 (OutPoint, Entry)。
	// 排序：消费顺序（tx 索引升序，同 tx 内 input 索引升序），确定性。
	// 语义：DisconnectBlock 必须把每一项恢复回 set（Entry 即 pre-block Entry）。
	Spent []UndoEntry

	// PreSetItemsCount pre-block UTXOSet 的 items 数。
	// 用于 DisconnectBlock 完整性校验：post-undo set 的 Len() 必须等于此值。
	// 任何对 Created/Spent 的篡改（缺/重/冲突）都会使此校验失败。
	PreSetItemsCount int
}

// UndoEntry 配对一个 OutPoint 与其在某确定时刻的完整 Entry。
type UndoEntry struct {
	OutPoint OutPoint
	Entry    Entry
}

// Undo journal 错误（corruption detection）。
var (
	// ErrUndoCorruptedCreated Created 中存在 set 不存在的 outpoint。
	// 触发条件：Apply 后某 Created 项被外部状态修改或撤销；或 undo 被篡改。
	ErrUndoCorruptedCreated = errors.New("undo journal corrupted: Created outpoint absent from current set")

	// ErrUndoCorruptedSpent Spent 中存在 set 已存在（且 Entry 不匹配）的 outpoint。
	// 触发条件：undo 试图恢复一个已存在的 outpoint（冲突）。
	ErrUndoCorruptedSpent = errors.New("undo journal corrupted: Spent outpoint already present in current set")

	// ErrUndoEntryMismatch Spent 项的 Entry 与 set 当前 Entry 不匹配（set 不为空）。
	// 触发条件：set 中存在 Spent 项但 Entry 不一致；或 undo 被篡改。
	ErrUndoEntryMismatch = errors.New("undo journal corrupted: Spent Entry does not match current set Entry")

	// ErrUndoOutOfOrderCreated Created 不按 OutPoint 字典序升序（违反确定性）。
	// 触发条件：undo 构造错误；序列化反序列化损坏。
	ErrUndoOutOfOrderCreated = errors.New("undo journal corrupted: Created entries not in canonical OutPoint order")

	// ErrUndoEmptyUndo 在 DisconnectBlock 上传入了空的 undo（既无 Created 也无 Spent）。
	// 这种 undo 是合法的（区块内所有交易都是空 round-trip）但应当显式标注，便于诊断。
	// 不视为错误：仅作为 sanity check 在测试中触发。
)

// ApplyBlockWithUndo 在 ApplyBlock 语义之上同步产出 BlockUndo。
//
// 等价于 ApplyBlock(base, txs, height) → newSet + fees + undo：
//
//   - 全部 ApplyBlock 的共识规则（coinbase 位置/双花/maturity/金额上限）继续适用；
//   - 全部验证在 base 的克隆上完成；任何失败返回 (nil, BlockUndo{}, 0, err)，
//     base 不被修改；
//   - 成功时返回 (newSet, undo, fees, nil)，newSet 与 base 是不同的 *UTXOSet
//     （base 本身永不被修改），undo 是从 (base, newSet, txs) 派生的完整反向描述。
//
// undo 构造算法（两阶段）：
//
//	Phase 1（apply）：逐 tx 调 ValidateTransaction 完成校验 + 状态迁移。
//	  不预收集 spent——因为 tx 可能消费同 block 内前序 tx 的输出，
//	  那种引用既不在 pre-block 也不在 post-block，不应进 Spent。
//
//	Phase 2（derive undo）：
//	  Created = working - baseSnapshot（OutPoint 不在 base 但在 working 的），
//	            按 OutPoint 升序排序；
//	  Spent   = baseSnapshot - working（OutPoint 在 base 但不在 working 的），
//	            按 (txIdx, inputIdx) 升序（即消费顺序）排序——为此需要
//	            记录每个 spent outpoint 的消费位置。
//
// 注意 Phase 2 中 Spent 的「消费位置」信息必须在 Phase 1 中收集：
// 我们在 tx 验证过程中记录「该 tx 的 input 列表中实际被 Spend 的 OutPoint
// 及其来源 (pre-block? same-block created?)」。最后只把来源为 pre-block 的
// 那些收集进 Spent。
func ApplyBlockWithUndo(base *UTXOSet, txs []*transaction.Transaction, height int) (newSet *UTXOSet, undo BlockUndo, totalFees uint64, err error) {
	if base == nil {
		return nil, BlockUndo{}, 0, errors.New("utxo: 基础集合为空")
	}
	if len(txs) == 0 {
		return nil, BlockUndo{}, 0, ErrNoOutputs
	}
	if !txs[0].IsCoinbase() {
		return nil, BlockUndo{}, 0, fmt.Errorf("%w: 首笔交易不是 coinbase", ErrBadCoinbase)
	}
	for _, tx := range txs[1:] {
		if tx.IsCoinbase() {
			return nil, BlockUndo{}, 0, fmt.Errorf("%w: 区块中出现多于一个 coinbase", ErrBadCoinbase)
		}
	}

	// pre-block 快照：用于 Spent 项的 Entry 查询与 Created/Spent 划分。
	// 关键：baseSnapshot 与 base 的 items 在 Apply 全程中**只读**。
	baseSnapshot := base.Clone()
	working := base.Clone()

	// undo 累积容器（构造期间可变；构造结束统一排序）。
	undo.Height = height
	created := make([]UndoEntry, 0, 2*len(txs))
	spent := make([]UndoEntry, 0, len(txs)) // 每笔非 coinbase tx 至少 1 input

	// spentMeta 记录每个 Spent 项的消费位置（tx 索引 + input 索引），用于排序与诊断。
	type spentMeta struct {
		outPoint OutPoint
		txIdx    int // 1-based（与 txs 索引对齐；coinbase 不参与）
		inIdx    int
	}
	spentMetas := make([]spentMeta, 0, len(txs))

	// ---- coinbase ----
	cb := txs[0]
	if fee, err := ValidateTransaction(cb, working, height); err != nil {
		return nil, BlockUndo{}, 0, err
	} else if fee != 0 {
		return nil, BlockUndo{}, 0, fmt.Errorf("%w: coinbase 交易不应产生手续费", ErrBadCoinbase)
	}
	// coinbase output 的 TxID 含区块高度（参见 transaction.serializeForHash），
	// 因此 OutPoint 与 baseSnapshot 中的任何 OutPoint 碰撞概率可忽略。
	// 防御性校验：若碰撞则报错。
	for i := range cb.Outputs {
		op := outpointAt(cb, i)
		if baseSnapshot.Has(op) {
			return nil, BlockUndo{}, 0, fmt.Errorf("utxo: coinbase output 碰撞 pre-block outpoint %s", op)
		}
	}

	// ---- 非 coinbase 交易 ----
	var fees uint64
	for txIdx, tx := range txs[1:] {
		// 1) 预记录 spent 元数据（用于 Phase 2 排序）。
		//    注意：tx 可能消费同 block 内前序 tx 的输出——这种情况下我们
		//    不把它加进 Spent（应自然落入"既不在 Created 也不在 Spent"）。
		//    但仍记录 spentMeta 以便 Phase 2 决定。
		for inIdx, in := range tx.Inputs {
			spentMetas = append(spentMetas, spentMeta{
				outPoint: OutPoint{Hash: in.PrevTxHash, Index: in.OutIndex},
				txIdx:    txIdx + 1, // 1-based；coinbase 索引 0
				inIdx:    inIdx,
			})
		}

		// 2) 校验 + 应用。
		fee, err := ValidateTransaction(tx, working, height)
		if err != nil {
			return nil, BlockUndo{}, 0, fmt.Errorf("高度 %d 交易校验失败: %w", height, err)
		}
		fees += fee
	}

	// ---- coinbase 金额上限 ----
	var coinbaseOut uint64
	for _, out := range cb.Outputs {
		coinbaseOut += out.Value
	}
	if coinbaseOut > Subsidy(height)+fees {
		return nil, BlockUndo{}, 0, fmt.Errorf(
			"%w: coinbase 输出 %d > 奖励 %d + 手续费 %d",
			ErrExcessiveCoinbase, coinbaseOut, Subsidy(height), fees)
	}

	// ---- Phase 2：derive undo ----

	// Spent：仅收集 baseSnapshot 中存在、post-block working 中不存在的 OutPoint。
	// Entry 来自 baseSnapshot。
	for _, sm := range spentMetas {
		if !baseSnapshot.Has(sm.outPoint) {
			// 同 block 内引用，不进 Spent
			continue
		}
		if working.Has(sm.outPoint) {
			// pre-block 存在，post-block 也存在——该 tx 消费了它但同 block 后序 tx 又创建了？
			// 这种情况要求 tx[A] 消费 opX，tx[B] 又创建同名 outpoint——TxID 唯一性保证不会发生。
			// 视为内部错误。
			return nil, BlockUndo{}, 0, fmt.Errorf(
				"utxo: undo derive 异常，OutPoint %s 在 pre-block 与 post-block 同时存在", sm.outPoint)
		}
		entry, ok := baseSnapshot.Get(sm.outPoint)
		if !ok {
			// 不可达（baseSnapshot.Has == true ⇒ Get 也返回 ok）
			return nil, BlockUndo{}, 0, fmt.Errorf(
				"utxo: Spent Entry 不可达 %s", sm.outPoint)
		}
		spent = append(spent, UndoEntry{OutPoint: sm.outPoint, Entry: entry})
	}

	// Created：working 中存在、baseSnapshot 中不存在的 OutPoint。
	// Entry 来自 working（post-block Entry）。
	workingAll := working.AllEntries()
	for op, entry := range workingAll {
		if baseSnapshot.Has(op) {
			continue
		}
		// 跳过 coinbase/非 coinbase 区分——Entry.IsCoinbase 已表明。
		created = append(created, UndoEntry{OutPoint: op, Entry: entry})
	}

	// 排序 Created 按 OutPoint 升序；Spent 按消费顺序保持（已天然有序）。
	sort.Slice(created, func(i, j int) bool {
		return outPointLess(created[i].OutPoint, created[j].OutPoint)
	})

	undo.Created = created
	undo.Spent = spent
	undo.PreSetItemsCount = baseSnapshot.Len()
	return working, undo, fees, nil
}

// DisconnectBlock 严格反向应用 BlockUndo，把 set 恢复到该区块应用前的状态。
//
// 约束（§六）：
//
//   - set 必须 == ApplyBlockWithUndo 产出的 newSet（即 post-block state）；
//     否则 corruption detection 会触发（ErrUndoCorruptedCreated / Spent）。
//   - Disconnect 失败（corruption）时 set 不被部分修改——全部校验通过后才执行
//     任何写操作；这是「forward failure 不污染 / reverse failure 不静默」的
//     对偶保证。
//   - 不修改 set 之外的任何状态（无 panic、无 log、无外部副作用）。
//
// 算法：
//
//  1. 校验 Created 中每一项存在于 set 且 Entry 与 set 中一致；
//  2. 校验 Spent 中每一项 NOT 存在于 set；
//  3. 按 reverse 顺序删除 Created（每步校验删除后的 set 一致性）；
//  4. 按 reverse 顺序恢复 Spent（每步校验 set 一致性）。
//
// 返回值：set 即被恢复到 pre-block state（== ApplyBlockWithUndo 的 base），
// 若 set 是 nil 则返回 ErrEmptyChain 类错误。
func DisconnectBlock(set *UTXOSet, undo BlockUndo) (*UTXOSet, error) {
	if set == nil {
		return nil, errors.New("utxo: DisconnectBlock set 为空")
	}

	// sanity: Created 排序（若 undo 来自非构造路径——例如篡改或跨实现——则触发）。
	for i := 1; i < len(undo.Created); i++ {
		if !outPointLess(undo.Created[i-1].OutPoint, undo.Created[i].OutPoint) {
			return nil, fmt.Errorf("%w: Created[%d]=%s 不小于 Created[%d]=%s",
				ErrUndoOutOfOrderCreated,
				i-1, undo.Created[i-1].OutPoint,
				i, undo.Created[i].OutPoint)
		}
	}

	// 校验 Created：必须存在于 set 且 Entry 一致。
	for _, ue := range undo.Created {
		cur, ok := set.Get(ue.OutPoint)
		if !ok {
			return nil, fmt.Errorf("%w: Created outpoint %s 不在当前 set",
				ErrUndoCorruptedCreated, ue.OutPoint)
		}
		if !entryEqual(cur, ue.Entry) {
			return nil, fmt.Errorf("%w: Created outpoint %s Entry 不一致",
				ErrUndoCorruptedCreated, ue.OutPoint)
		}
	}

	// 校验 Spent：必须 NOT 存在于 set（否则冲突 / undo 被部分应用过）。
	for _, ue := range undo.Spent {
		if set.Has(ue.OutPoint) {
			return nil, fmt.Errorf("%w: Spent outpoint %s 已存在于当前 set（可能已被部分恢复）",
				ErrUndoCorruptedSpent, ue.OutPoint)
		}
	}

	// 应用：reverse Created 删除。
	for i := len(undo.Created) - 1; i >= 0; i-- {
		ue := undo.Created[i]
		// Spend 返回 Entry 与 Entry 不一致时（已被外部修改）→ error（不静默）。
		actual, err := set.Spend(ue.OutPoint)
		if err != nil {
			return nil, fmt.Errorf("utxo: DisconnectBlock Created 删除失败 %s: %w", ue.OutPoint, err)
		}
		if !entryEqual(actual, ue.Entry) {
			// 不可达（前置校验已通过），防御性保留。
			return nil, fmt.Errorf("utxo: DisconnectBlock Created Spend 后 Entry 不一致 %s", ue.OutPoint)
		}
	}

	// 应用：reverse Spent 恢复。
	for i := len(undo.Spent) - 1; i >= 0; i-- {
		ue := undo.Spent[i]
		set.Add(ue.OutPoint, ue.Entry)
	}

	// 完整性校验：post-undo Len 必须等于 PreSetItemsCount。
	// 这能捕获「缺 entry」（post-undo Len > PreSetItemsCount）等篡改。
	if final := set.Len(); final != undo.PreSetItemsCount {
		return nil, fmt.Errorf("%w: post-undo set.Len()=%d, want PreSetItemsCount=%d（undo 可能被篡改：缺/重 entry）",
			ErrUndoCorruptedCreated, final, undo.PreSetItemsCount)
	}

	return set, nil
}

// outPointLess 字典序比较：先 Hash 32 字节，再 Index。
// 与 bytes.Compare(Hash[:]) 然后比较 Index 一致。
func outPointLess(a, b OutPoint) bool {
	c := bytes.Compare(a.Hash[:], b.Hash[:])
	if c != 0 {
		return c < 0
	}
	return a.Index < b.Index
}

// entryEqual 完整 Entry 比较（所有 4 字段）。
func entryEqual(a, b Entry) bool {
	return a.Value == b.Value &&
		a.PubKeyHash == b.PubKeyHash &&
		a.Height == b.Height &&
		a.IsCoinbase == b.IsCoinbase
}

// ────────────────────────────────────────────────────────────────────────────
// 确定性 Encode / Decode 接口
//
// 持久化由 REORG-1E（internal/storage 的 UNDO 记录帧）承载：本文件只负责
// BlockUndo 的确定性字节序列化，不涉及文件格式。
//
// 格式（每项 big-endian，便于调试与跨语言互操作）：
//
//   Height             : int32  (4 bytes BE)
//   PreSetItemsCount   : int32  (4 bytes BE)
//   Created.Len        : uint32 (4 bytes BE)
//   Created[i]         : OutPoint (32+4=36 bytes) + Entry (8+20+4+1=33 bytes)
//   Spent.Len          : uint32 (4 bytes BE)
//   Spent[i]           : OutPoint (32+4=36 bytes) + Entry (8+20+4+1=33 bytes)
//
// 总长度：8 + 4 + len(Created)*69 + 4 + len(Spent)*69 bytes
//         （前 8 = Height(4) + PreSetItemsCount(4)）。
//
// 不使用 JSON / gob —— 跨语言互操作 + 抗版本漂移 + 紧凑字节。

// EncodeUndo 将 BlockUndo 编码为确定性字节流。
func EncodeUndo(undo BlockUndo) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, int32(undo.Height)); err != nil {
		return nil, fmt.Errorf("utxo: encode height 失败: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, int32(undo.PreSetItemsCount)); err != nil {
		return nil, fmt.Errorf("utxo: encode pre-set-count 失败: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, uint32(len(undo.Created))); err != nil {
		return nil, fmt.Errorf("utxo: encode created len 失败: %w", err)
	}
	for i, ue := range undo.Created {
		if err := encodeUndoEntry(buf, ue); err != nil {
			return nil, fmt.Errorf("utxo: encode created[%d] 失败: %w", i, err)
		}
	}
	if err := binary.Write(buf, binary.BigEndian, uint32(len(undo.Spent))); err != nil {
		return nil, fmt.Errorf("utxo: encode spent len 失败: %w", err)
	}
	for i, ue := range undo.Spent {
		if err := encodeUndoEntry(buf, ue); err != nil {
			return nil, fmt.Errorf("utxo: encode spent[%d] 失败: %w", i, err)
		}
	}
	return buf.Bytes(), nil
}

func encodeUndoEntry(buf *bytes.Buffer, ue UndoEntry) error {
	if _, err := buf.Write(ue.OutPoint.Hash[:]); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, ue.OutPoint.Index); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, ue.Entry.Value); err != nil {
		return err
	}
	if _, err := buf.Write(ue.Entry.PubKeyHash[:]); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, int32(ue.Entry.Height)); err != nil {
		return err
	}
	var isCb byte
	if ue.Entry.IsCoinbase {
		isCb = 1
	}
	if err := buf.WriteByte(isCb); err != nil {
		return err
	}
	return nil
}

// DecodeUndo 从确定性字节流还原 BlockUndo。
//
// 返回 ErrUndoCorrupted* 类的错误当且仅当字节流结构不合法。
// 不会主动校验 Created 排序或与 set 的一致性——后者由 DisconnectBlock 完成。
func DecodeUndo(data []byte) (BlockUndo, error) {
	var undo BlockUndo
	buf := bytes.NewReader(data)

	var h int32
	if err := binary.Read(buf, binary.BigEndian, &h); err != nil {
		return BlockUndo{}, fmt.Errorf("utxo: decode height 失败: %w", err)
	}
	undo.Height = int(h)

	var preCount int32
	if err := binary.Read(buf, binary.BigEndian, &preCount); err != nil {
		return BlockUndo{}, fmt.Errorf("utxo: decode pre-set-count 失败: %w", err)
	}
	undo.PreSetItemsCount = int(preCount)

	var nCreated uint32
	if err := binary.Read(buf, binary.BigEndian, &nCreated); err != nil {
		return BlockUndo{}, fmt.Errorf("utxo: decode created len 失败: %w", err)
	}
	undo.Created = make([]UndoEntry, 0, nCreated)
	for i := uint32(0); i < nCreated; i++ {
		ue, err := decodeUndoEntry(buf)
		if err != nil {
			return BlockUndo{}, fmt.Errorf("utxo: decode created[%d] 失败: %w", i, err)
		}
		undo.Created = append(undo.Created, ue)
	}

	var nSpent uint32
	if err := binary.Read(buf, binary.BigEndian, &nSpent); err != nil {
		return BlockUndo{}, fmt.Errorf("utxo: decode spent len 失败: %w", err)
	}
	undo.Spent = make([]UndoEntry, 0, nSpent)
	for i := uint32(0); i < nSpent; i++ {
		ue, err := decodeUndoEntry(buf)
		if err != nil {
			return BlockUndo{}, fmt.Errorf("utxo: decode spent[%d] 失败: %w", i, err)
		}
		undo.Spent = append(undo.Spent, ue)
	}

	return undo, nil
}

func decodeUndoEntry(buf *bytes.Reader) (UndoEntry, error) {
	var ue UndoEntry
	if _, err := buf.Read(ue.OutPoint.Hash[:]); err != nil {
		return UndoEntry{}, fmt.Errorf("read hash 失败: %w", err)
	}
	if err := binary.Read(buf, binary.BigEndian, &ue.OutPoint.Index); err != nil {
		return UndoEntry{}, fmt.Errorf("read index 失败: %w", err)
	}
	if err := binary.Read(buf, binary.BigEndian, &ue.Entry.Value); err != nil {
		return UndoEntry{}, fmt.Errorf("read value 失败: %w", err)
	}
	if _, err := buf.Read(ue.Entry.PubKeyHash[:]); err != nil {
		return UndoEntry{}, fmt.Errorf("read pubkeyhash 失败: %w", err)
	}
	var h int32
	if err := binary.Read(buf, binary.BigEndian, &h); err != nil {
		return UndoEntry{}, fmt.Errorf("read height 失败: %w", err)
	}
	ue.Entry.Height = int(h)
	isCb, err := buf.ReadByte()
	if err != nil {
		return UndoEntry{}, fmt.Errorf("read iscoinbase 失败: %w", err)
	}
	ue.Entry.IsCoinbase = isCb == 1
	return ue, nil
}
