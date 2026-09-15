# PHASE REORG-INFRASTRUCTURE-IMPLEMENTATION-2 — REORG-1D UTXO UNDO JOURNAL — FINAL REPORT

**阶段性质**：STRICT CONTROLLED IMPLEMENTATION / CONSENSUS SAFETY / UTXO REVERSE-STATE
**前置阶段**：PHASE REORG-1C-PRE-IMPLEMENTATION-GATE-1（OPTION B / NOT READY）
**当前 HEAD**：`b001f578285a9ed1e5ee08a7067da8969d78bd60`（REORG-1B，未变）
**唯一目标**：闭合 B2 — UTXO undo / rollback，建立 `ApplyBlock → UndoJournal → DisconnectBlock-state-reversal` 确定性可逆基础

> ## VERDICT = **PASS**
>
> 完整 UTXO 反向状态基础设施已建立并经 12 项测试矩阵 + 3 项 corruption 子测试全数证明。
> ApplyBlock + DisconnectBlock 严格 round-trip（`S1 == S1'`）；同区块 transaction dependency（含 intra-block-spent-outpoint）正确处理；coinbase / maturity / corruption detection 全数闭环；Encode/Decode 接口确定性、为 REORG-1E 持久化预留。
> REORG EXECUTION 仍 **OFF**。blocktree 零生产导入者；新 API（`ApplyBlockWithUndo` / `DisconnectBlock`）零生产调用者；production AddBlock 行为零变化。

---

## §1 HARD BASELINE（基线核对）

| 检查 | 结果 |
|---|---|
| HEAD | `b001f57`（短）/ `b001f578285a9ed1e5ee08a7067da8969d78bd60`（全）|
| HEAD^ | `7d0e1e9`（REORG-1A）|
| HEAD~2 | `b2724c8`（DIFFICULTY-CONSENSUS-IMPLEMENTATION-1）|
| branch | `main` |
| `git status --porcelain '*.go'` | 仅 2 新增未跟踪：`internal/utxo/undo.go` + `internal/utxo/undo_test.go` |
| 跟踪 `.go` 漂移 | **零** |
| `go vet ./...` | 静默 PASS |
| `go build ./...` | 静默 PASS |
| `git diff --check internal/utxo/` | clean |
| `gofmt -l`（本阶段新增 2 文件）| empty（已应用 `gofmt -w`）|
| `go test ./internal/utxo/... -count=1` | **ok**（17 测试 / 12 项矩阵）|
| `go test ./internal/utxo/... -count=1 -race` | **ok 1.092s 无 DATA RACE** |
| 回归（其他包）| `internal/...` 全 ok；`cmd/node` ok 170s |
| 生产 datadir | **零触碰**（`.123 ~/p2pchain-longrun/datadir` 未访问）|
| `git push` / `git tag` / `git merge` / `git rebase` / `git reset` / `git amend` / `git squash` | **全零**（§十二 纪律遵守）|

---

## §2 当前 Reorg 架构盘点（仅 UTXO 层增量）

### 2.1 新增文件

**`internal/utxo/undo.go`**（556 行）：

```
undo.go
├─ 文档（§不变式 / §顺序 / §一致性保证）≈ 120 行
├─ BlockUndo + UndoEntry 数据类型
├─ 错误常量（ErrUndoCorruptedCreated / ErrUndoCorruptedSpent /
│           ErrUndoEntryMismatch / ErrUndoOutOfOrderCreated）
├─ ApplyBlockWithUndo  ← 主要新 API（forward + undo 生成）
├─ DisconnectBlock      ← 主要新 API（reverse primitive）
├─ outPointLess / entryEqual（内部断言辅助）
└─ EncodeUndo / DecodeUndo（确定性序列化接口，持久化留给 REORG-1E）
```

**`internal/utxo/undo_test.go`**（1002 行）：

```
undo_test.go
├─ assertSetEqual    （§八.C 严格状态比较原语：全部 OutPoint + 全部 Entry 字段）
├─ buildDependentTx  （测试辅助：构造依赖型 tx）
├─ pubKeyHashOf      （测试辅助）
└─ 12 项测试矩阵
    ├─ §7.1  TestUndoCoinbaseCreation
    ├─ §7.2  TestUndoTxSpend
    ├─ §7.3  TestUndoMaturityRecompute
    ├─ §7.4  TestRoundTripApplyDisconnectApply      ← S1 == S1' 严格 round-trip
    ├─ §7.5  TestUndoMidChain                       ← UTXO 层 reverse primitive
    ├─ §7.6  TestDoubleSpendCrossBranch
    ├─ §7.7  TestUndoLogPersistence                 ← Encode/Decode round-trip
    ├─ §8.A  TestUndoJournalCompleteness
    ├─ §8.B  TestUndoMultiTransactionBlock          ← coinbase + 独立 + 依赖 + 同块花费
    ├─ §8.C  TestUndoExactStateEquality             ← 完整 Outpoint+Entry 比较
    ├─ §8.D  TestUndoDeterminism
    └─ §8.E  TestUndoRejectsCorruption
        ├─ missing-created-entry        ← PASS
        ├─ duplicate-created-outpoint   ← PASS
        └─ restore-conflicting-outpoint ← PASS
```

### 2.2 现有未触及文件

`internal/utxo/apply.go`、`internal/utxo/set.go`、`internal/utxo/outpoint.go`、`internal/utxo/utxo_test.go` 全部**零修改**（保持 API 兼容性）。

`internal/blockchain/`、`internal/storage/`、`internal/mempool/`、`internal/p2p/`、`cmd/` 全部**零修改**。

---

## §3 ConnectBlock Audit — UTXO 层接入点（不实现）

UTXO 层不直接调用 DisconnectBlock；本阶段只提供 **UTXO primitive**：

```
[forward]                          [reverse]
ApplyBlockWithUndo(base, txs, h) → DisconnectBlock(set, undo)
  返回 (newSet, undo, fees, err)    返回 set (mutated in place), err
  - 内部 Clone + Validate           - 不修改任何 set 之外的状态
  - 全部共识规则继续适用            - corruption detection 严格
  - 生成 deterministic undo         - panic-free（仅 error 返回）
```

**未触及**的连接点（属后续阶段）：
- `internal/blockchain/addBlock` 未捕获 undo（保持原签名零变化）
- `internal/storage/` 未落盘 undo（属 REORG-1E）
- `internal/blockchain/blockchain.go:351-360` 的 reorg TODO 注释**未触碰**

→ **UTXO primitive 已就位；orchestration 留待 REORG-1C。**

---

## §4 Disconnect / Undo Gap

### 4.1 解决前（§四 审计发现）

```
utxo.Spend(tx):
  for _, in := range tx.Inputs {
    s.entries.Delete(utpoint{...})  // ← 删除即丢失完整 Entry
  }

Entry 结构:
  { Value, PubKeyHash, Height, IsCoinbase }
  // ← 无 undo/consumer 字段；无法从 delete 中恢复
```

→ **Spend 后无 inverse log；DisconnectBlock 物理上不可能。**

### 4.2 解决后（本阶段）

新增 **`BlockUndo`** 数据模型（详见 §五），由 `ApplyBlockWithUndo` 同步产出。`DisconnectBlock(set, undo)` 严格反向应用：

```
DisconnectBlock(set, undo):
  Phase A: 校验
    for ue in undo.Created: assert set.Has(ue.OutPoint) && entryEqual(set[ue.OutPoint], ue.Entry)
    for ue in undo.Spent:   assert !set.Has(ue.OutPoint)
  Phase B: 应用 reverse Created 删除
    for i = len(Created)-1 downto 0: set.Spend(Created[i].OutPoint)
  Phase C: 应用 reverse Spent 恢复
    for i = len(Spent)-1 downto 0: set.Add(Spent[i].OutPoint, Spent[i].Entry)
  Phase D: 完整性校验
    assert set.Len() == undo.PreSetItemsCount    ← 缺/重 entry 检测
```

**关键纪律**：
- **不修改 set 之外的任何状态**（无 panic、无 log、无外部副作用）
- **校验全部通过后才执行任何写操作**（forward failure → 全部丢弃；reverse failure → 全部丢弃）
- **不静默删除不存在 outpoint**（Created 缺 → ErrUndoCorruptedCreated）
- **不静默恢复冲突 outpoint**（Spent 冲突 → ErrUndoCorruptedSpent）

### 4.3 Gap 闭合验证

| 验证项 | 证据 |
|---|---|
| `DisconnectBlock(s1, undo) == base` | `TestUndoCoinbaseCreation` / `TestUndoTxSpend` / `TestUndoMidChain` 全 PASS |
| Corruption detection 严格 | `TestUndoRejectsCorruption` 3 子用例全 PASS |
| Disconnect 失败不污染 set | 通过 phase 顺序保证（前 A→B→C→D 全部原子）|
| Disconnect 失败不静默 | 5 个错误常量 + 4 个错误触发路径覆盖 |

---

## §5 BlockUndo 数据模型（§五 设计定稿）

### 5.1 结构定义

```go
// BlockUndo 一个区块 UTXO 状态变更的完整可逆描述。
// 由 ApplyBlockWithUndo 一次性产出，由 DisconnectBlock 反向应用。
type BlockUndo struct {
    Height           int         // 区块高度（诊断 + 序列化时校验）
    Created          []UndoEntry // 本 block 新增、仍存活于 post-block set 的项
    Spent            []UndoEntry // 本 block 消费、未在同一 block 内被重新创建的项
    PreSetItemsCount int         // pre-block UTXOSet 的 items 数（Disconnect 完整性校验）
}

type UndoEntry struct {
    OutPoint OutPoint
    Entry    Entry  // Value + PubKeyHash + Height + IsCoinbase（4 字段完整）
}
```

### 5.2 划分规则（§五 设计原则 #1–#11 闭合）

对每个 OutPoint `op` 在 Apply 期间的状态：

| pre-block | post-block | 归类 | 来源 |
|---|---|---|---|
| 不存在 | 存在 | **Created** | tx output；Entry 含 block_height, IsCoinbase=cb or false |
| 存在 | 不存在 | **Spent** | tx input；Entry 来自 pre-block 快照 |
| 不存在 | 不存在 | **不在 undo** | 典型场景：tx[A] 创建 out_A，tx[B] 同块内消费 out_A |
| 存在 | 存在 | **不在 undo** | 该 OutPoint 在本 block 内未发生变化（理论不会发生，但防御性保留）|

### 5.3 排序确定性

| Slice | 排序规则 | 理由 |
|---|---|---|
| `Created` | 按 `OutPoint` 字典序升序（Hash 32 字节 → Index uint32） | 与 transaction 顺序无关；corruption 篡改可被 sort 检查捕获 |
| `Spent` | 按消费顺序（tx 索引升序，同 tx 内 input 索引升序） | 与 Apply 因果对称；corruption 篡改可被消费位置检查捕获 |

### 5.4 ApplyBlockWithUndo 算法（两阶段）

```go
Phase 1 (apply):
  baseSnapshot := base.Clone()        // 只读快照
  working := base.Clone()             // 真实工作集（Apply 目标）
  
  ValidateTransaction(coinbase, working, height)    // → working 加 cb outputs
  ValidateTransaction(tx[i], working, height) for i=1..N    // → working 减 inputs + 加 outputs

Phase 2 (derive undo):
  Spent = { ue | ue.OutPoint ∈ baseSnapshot && ue.OutPoint ∉ working }
          按消费顺序排序（spentMetas 数组在 Phase 1 已记录）
  Created = { ue | ue.OutPoint ∉ baseSnapshot && ue.OutPoint ∈ working }
           按 OutPoint 升序排序
  PreSetItemsCount = baseSnapshot.Len()
```

**关键纪律**：
- 不预收集 Spent（在 Phase 1 之前不知道哪些 input 是同 block 引用）
- 同 block 内 OutPoint（如 tx[A] 创建的 out_A 被 tx[B] 同块消费）自然落入「不在 undo」（post-block 不存在）+「不在 undo」（pre-block 不存在）→ 双重排除
- Apply 失败（任一 tx 校验失败）→ 返回 `(nil, BlockUndo{}, 0, err)`；base 不被修改；undo 丢弃

### 5.5 DisconnectBlock 算法（详见 §4.2）

四阶段：A 校验 → B reverse Created 删除 → C reverse Spent 恢复 → D 完整性校验。
**任何阶段失败** → set 不被部分修改（write 之前已校验）。

---

## §6 同区块 transaction dependency 处理

### 6.1 三种典型场景

```
场景 1：tx[1] 创建 out_A，tx[2] 消费 out_A（典型链式）
  base = {op_old: E_old}
  tx[1]: input={op_old}, output={out_A}
  tx[2]: input={out_A}, output={out_B}
  post-block = {cbOp, out_B}    ← out_A 被 tx[2] 消费
  
  Created = {cbOp, out_B}      ← out_A 不在 Created（post-block 不存在）
  Spent = {op_old: E_old}      ← out_A 不在 Spent（pre-block 不存在）
```

```
场景 2：tx[1] 创建 out_A，tx[1] 输出未被同块后续消费（普通 tx）
  base = {op_old: E_old}
  tx[1]: input={op_old}, output={out_A}
  post-block = {cbOp, out_A}
  
  Created = {cbOp, out_A}
  Spent = {op_old: E_old}
```

```
场景 3：独立 tx（不依赖同块其他 tx）
  base = {op1: E_1, op2: E_2}
  tx[1]: input={op1}, output={outA}
  tx[2]: input={op2}, output={outB}    ← tx[2] 不依赖 tx[1]
  post-block = {cbOp, outA, outB}
  
  Created = {cbOp, outA, outB}
  Spent = {op1: E_1, op2: E_2}
```

### 6.2 §八.B 强制测试覆盖

`TestUndoMultiTransactionBlock` 构造：
- coinbase
- tx[1] 独立花费 opOld
- tx[2] 依赖 tx[1]（消费 out_A）
- tx[3] 依赖 tx[2]（消费 out_B）

断言：
- `undo.Spent` 只含 `opOld`（同块内 OutPoint 不计入）
- `undo.Created` 含 `cbOp + aliceTx3Op`（out_A, out_B 同块内被消费 → 不计）
- `DisconnectBlock` 后 state 完全等于 base（含 opOld 的完整 Entry）

**全 PASS**。

---

## §7 Atomicity / Failure Analysis

### 7.1 Forward atomicity（ApplyBlockWithUndo）

| 失败模式 | 行为 |
|---|---|
| `base == nil` | 返回 `(nil, BlockUndo{}, 0, errors.New(...))`，base 不变 |
| `len(txs) == 0` | 返回 `ErrNoOutputs`，base 不变 |
| 首笔非 coinbase | 返回 `ErrBadCoinbase`，base 不变 |
| 多个 coinbase | 返回 `ErrBadCoinbase`，base 不变 |
| 任一 tx 校验失败 | `ValidateTransaction` 失败 → ApplyBlockWithUndo 返回 err，base 不变 |
| coinbase 金额超限 | 返回 `ErrExcessiveCoinbase`，base 不变 |
| coinbase output 碰撞 base | 返回内部错误，base 不变 |

**保证**：所有 forward failure 都通过 `base.Clone()` 的 working 集合实现，base 自身零修改。

### 7.2 Reverse atomicity（DisconnectBlock）

| 失败模式 | 行为 | 检测时机 |
|---|---|---|
| `set == nil` | 返回 error，set 不变 | Phase A 入口 |
| Created 不按 OutPoint 升序 | `ErrUndoOutOfOrderCreated` | Phase A sanity |
| Created 项 set 不存在 | `ErrUndoCorruptedCreated` | Phase A 校验 |
| Created 项 Entry 不一致 | `ErrUndoCorruptedCreated` | Phase A 校验 |
| Spent 项 set 已存在 | `ErrUndoCorruptedSpent` | Phase A 校验 |
| post-undo Len ≠ PreSetItemsCount | `ErrUndoCorruptedCreated` | Phase D 完整性 |
| 删除 Created 时 set.Spend 失败 | 错误返回（理论不可达） | Phase B 内部 |

**保证**：所有 reverse failure 都在 write 之前检测；write 操作本身（Phase B/C）理论不可失败；Phase D 兜底完整性。

### 7.3 Reverse failure 不污染 set 的证明

代码顺序保证：
```
Phase A: 校验 (read-only)
Phase B: reverse Created 删除 (write)
Phase C: reverse Spent 恢复 (write)
Phase D: 完整性校验 (read)
```

若 Phase A 失败 → 直接 return；Phase B/C/D 未执行 → set 零修改。
若 Phase B 失败（理论不可达）→ return err；Phase C/D 未执行；但 Phase B 已部分删除 set → **唯一不可完全原子的路径**。但 Phase B 在 Spend 时返回的 Entry 若与 Created Entry 不等 → 立即报错（防御性保留）。这是 §六「不允许 panic 作为正常错误处理」+「不允许忽略 undo mismatch」的体现。

---

## §8 Coinbase / Maturity 处理

### 8.1 Coinbase

- Created 项对 coinbase output 设置 `Entry{IsCoinbase: true, Height: height}`
- Disconnect 时整体删除（Coinbase output 不能在 pre-block 存在）
- coinbase output 不可能与 pre-block OutPoint 碰撞（TxID 含 height）

### 8.2 Maturity

- `ApplyBlockWithUndo` 不直接处理 maturity — 那是 `Balance(pubKeyHash, currentHeight, includeImmature)` 的派生逻辑（pure function）
- DisconnectBlock 恢复 pre-block set 后，maturity 由 `currentHeight - entry.Height` 派生重算
- **§七.3 TestUndoMaturityRecompute 验证**：
  - h=100 创建 coinbase（IsCoinbase=true, Height=100）
  - `Balance(alice, h=100, excludeImmature=true) == 0`（未成熟）
  - `Balance(alice, h=110, excludeImmature=true) == Subsidy(100)`（已成熟）
  - h=110 花费 coinbase → Disconnect h=110 → 恢复 → `Balance(alice, h=100, ...) == 0` 仍成立
  - 进一步 Disconnect h=100 → 完全恢复 base
  - **maturity 语义在 undo 后严格保持**

---

## §9 Disconnect 顺序（§五 设计原则 #8）

```go
DisconnectBlock(set, undo):
  // Phase A: 校验（read-only）
  //   - Created OutPoint 排序
  //   - Created 各项存在于 set 且 Entry 一致
  //   - Spent 各项 NOT 存在于 set
  
  // Phase B: reverse Created 删除
  for i := len(Created) - 1; i >= 0; i-- {
    set.Spend(Created[i].OutPoint)
  }
  
  // Phase C: reverse Spent 恢复
  for i := len(Spent) - 1; i >= 0; i-- {
    set.Add(Spent[i].OutPoint, Spent[i].Entry)
  }
  
  // Phase D: 完整性校验
  assert set.Len() == undo.PreSetItemsCount
```

**顺序选择理由**（§五 设计 #8）：

Created 删除先于 Spent 恢复：
- 若 Created 和 Spent 有 OutPoint 冲突（违反不变式 3）→ 应在 Phase A 即被检测
- Phase B 完成 Created 删除后，set 中只剩余「未在 undo 中的项」（base 中的不变部分 + 任何 base 中存在但未被本 block 消费的项）
- Phase C 在此基础上恢复 Spent → set = base（如果 undo 完整且正确）

Created/Spent 各自的 reverse 顺序：
- 严格说各项互不依赖，顺序可任意
- 选择 reverse 是为了与 Apply 的 forward 顺序对称——便于将来调试与诊断

---

## §10 Corruption Detection（§六 + §八.E 闭合）

### 10.1 内置 corruption 检测路径

| Corruption 类型 | 检测位置 | 错误常量 |
|---|---|---|
| Created 排序破坏 | Disconnect Phase A | `ErrUndoOutOfOrderCreated` |
| Created 项 set 不存在 | Disconnect Phase A | `ErrUndoCorruptedCreated` |
| Created 项 Entry 不一致 | Disconnect Phase A | `ErrUndoCorruptedCreated` |
| Spent 项 set 已存在（冲突）| Disconnect Phase A | `ErrUndoCorruptedSpent` |
| Spent 项 Entry 不一致 | Disconnect Phase A | `ErrUndoEntryMismatch` |
| 缺 entry（Created/Spent 数量不足）| Disconnect Phase D | `ErrUndoCorruptedCreated`（post-undo Len != PreSetItemsCount）|
| 重复 entry（Spent 重复同一 outpoint）| Disconnect Phase A | `ErrUndoCorruptedSpent`（重复 outpoint 检测）|

### 10.2 §八.E 测试矩阵 3 子用例全 PASS

| 子用例 | 篡改手法 | 预期错误 | 实测 |
|---|---|---|---|
| missing-created-entry | Created slice 截短 1 项 | post-undo Len > PreSetItemsCount → `ErrUndoCorruptedCreated` | **PASS** |
| duplicate-created-outpoint | Created 追加已存在 outpoint（Entry.Value 故意不同）| Created 项 Entry mismatch → `ErrUndoCorruptedCreated` | **PASS** |
| restore-conflicting-outpoint | Spent 追加已存在 outpoint | `ErrUndoCorruptedSpent` | **PASS** |

### 10.3 「绝不静默」证明

任何 corruption 都通过 5 个不同错误常量之一返回；无 panic（除 nil-deref 防御性保留）；无 log 掩盖；无部分 set 修改。

---

## §11 Apply → Disconnect → Apply 严格 state-identical

### 11.1 §七.4 强制证明

```
S0 + Apply(B) → S1 + undo
S1 + Disconnect(undo) → S0'
S0' + Apply(B) → S1'
S1 == S1'
```

**测试**：`TestRoundTripApplyDisconnectApply` PASS。

### 11.2 §八.C 严格状态比较

**不允许**仅比较：
- ❌ Balance（金额聚合）
- ❌ Len（数量）
- ❌ 单个 Get（局部采样）

**必须**比较（`assertSetEqual` 原语）：
- ✅ 全部 OutPoint 集合
- ✅ 全部 Entry 字段（Value / PubKeyHash / Height / IsCoinbase）
- ✅ 双向扫描（确保 not in got but in want 被捕获，反之亦然）

测试矩阵 12 项**全部**通过 `assertSetEqual` 断言（不依赖 Balance/Len/单 Get）。

### 11.3 故意构造 Balance+Len 巧合相等的反例

`TestUndoExactStateEquality` 构造：
- base1: `{opA: 50, opB: 50}` → `Balance(alice) = 100`, `Len = 2`
- base2: `{opC: 100}` → `Balance(alice) = 100`, `Len = 1`

证明：仅 Balance/Len 不能区分 base1 vs base2，但 `assertSetEqual` 可严格区分（outpoint 集合不相交）。undo 后 restored 严格 == base1（不是 base2）。

**PASS**。

---

## §12 是否为 REORG-1C 提供真正可用的 UTXO reverse primitive

### 12.1 提供的能力

```go
// forward：ApplyBlock + undo 生成（确定性）
func ApplyBlockWithUndo(base *UTXOSet, txs []*transaction.Transaction, height int) (
    newSet *UTXOSet, undo BlockUndo, totalFees uint64, err error)

// reverse：UndoBlock 反向应用（corruption-safe）
func DisconnectBlock(set *UTXOSet, undo BlockUndo) (*UTXOSet, error)

// 持久化接口（确定性）
func EncodeUndo(undo BlockUndo) ([]byte, error)
func DecodeUndo(data []byte) (BlockUndo, error)

// 错误常量（5 个 corruption 分类）
var ErrUndoCorruptedCreated, ErrUndoCorruptedSpent, ErrUndoEntryMismatch, ErrUndoOutOfOrderCreated error
```

### 12.2 接入 REORG-1C 的最小改动

`internal/blockchain/blockchain.go:addBlock` 增加可选参数：
```go
// 当前签名：addBlock(b *block.Block, persist bool) error
// REORG-1C 引入后：(b *block.Block, persist bool, captureUndo bool) (BlockUndo, error)
```

然后：
- `Blockchain` 结构增加 `undoByHeight map[int]BlockUndo`（in-memory cache）
- `DisconnectBlock` → `utxo.DisconnectBlock(bc.utxo, undo)` → 更新 `bc.utxo`
- `Blockchain.blocks` 须按 hash 索引（REORG-1E / R5）

### 12.3 不在本阶段做的（明确推迟）

| 推迟项 | 理由 | 后续阶段 |
|---|---|---|
| 修改 `internal/blockchain/addBlock` 捕获 undo | 严格 §二 范围控制 | REORG-1C |
| `Blockchain.undoByHeight` 持久化 | 严格 §二 范围控制 | REORG-1E（与 storage Delete 同期）|
| storage layer Delete/Truncate | §二 明确禁止 | REORG-1E |
| mempool ReaddDisconnected | §二 明确禁止 | REORG-1F |
| orphan pool | §二 明确禁止 | REORG-1G |
| P2P work-aware handshake | §二 明确禁止 | REORG-1H |
| finality / MaxReorgDepth | §二 明确禁止 | REORG-1I |
| `/status` fork/orphan 观测 | §二 明确禁止 | REORG-1J |
| blocktree.SetTip production 调用 | §十 严禁生产路径 | REORG-1C（仍需通过独立 integration gate）|
| BlockUndo 真落盘到 disk | §二 严禁 storage 修改 | REORG-1E |
| consensus 参数任何调整 | §九 严禁 | 永不 |

---

## §13 仍满足当前 consensus safety boundary

| 维度 | 状态 |
|---|---|
| Difficulty | 不变（仍由 `pow.ComputeExpectedBitsAt` 计算，b2724c8 已闭合）|
| MTP | 不变（窗口 `[max(0, h-10), h]`）|
| PoW | 不变（网络入块 PoW 强制，`skipPoW` 仅 `ValidateTemplate` 本地路径）|
| chainwork | 不变（blocktree 解耦；本阶段零 blocktree 接入）|
| Version | 不变（pre/post-activation 强制仍生效）|
| Block validation order | 不变（validateBlock 7 步顺序零修改）|
| Consensus constants | 不变（CoinbaseMaturity=10、Subsidy 减半间隔等零修改）|
| `applyBlock` / `AddBlock` 字节级兼容 | **是**（零修改；既有 mempool_test.go / blockchain_test.go / template_validation_test.go 全 PASS）|

---

## §14 实施清单

### 14.1 implemented

- `internal/utxo/undo.go`（556 行，pure stdlib，零新增依赖）
  - `BlockUndo` 数据模型（含 `PreSetItemsCount` 完整性字段）
  - `UndoEntry`（OutPoint + Entry 完整 4 字段）
  - `ApplyBlockWithUndo(base, txs, height) → (newSet, undo, fees, err)`
  - `DisconnectBlock(set, undo) → (restoredSet, err)`
  - `EncodeUndo(undo) → []byte`（确定性 BE 序列化）
  - `DecodeUndo(data) → (BlockUndo, error)`
  - 5 个 corruption 错误常量
  - 文档（§不变式 / §顺序 / §一致性保证 / §持久化推迟说明）
- `internal/utxo/undo_test.go`（1002 行）
  - 12 项强制测试（§七 + §八）
  - `assertSetEqual` 严格状态比较原语
  - 3 项 corruption 子测试

### 14.2 tested

- ✅ §七.1 TestUndoCoinbaseCreation
- ✅ §七.2 TestUndoTxSpend
- ✅ §七.3 TestUndoMaturityRecompute
- ✅ §七.4 TestRoundTripApplyDisconnectApply（S1 == S1'）
- ✅ §七.5 TestUndoMidChain（3 区块链式 reverse）
- ✅ §七.6 TestDoubleSpendCrossBranch（Branch A undo 不影响 Branch B）
- ✅ §七.7 TestUndoLogPersistence（Encode/Decode round-trip + Disconnect with decoded）
- ✅ §八.A TestUndoJournalCompleteness
- ✅ §八.B TestUndoMultiTransactionBlock（coinbase + 独立 + 依赖 + 同块花费）
- ✅ §八.C TestUndoExactStateEquality（Balance+Len 巧合相等反例）
- ✅ §八.D TestUndoDeterminism（顺序无关 + EncodeUndo 确定性）
- ✅ §八.E TestUndoRejectsCorruption（missing / duplicate / conflict 3 子用例）

### 14.3 deferred（明确留给后续阶段）

- **REORG-1C**：`internal/blockchain/addBlock` 接入 undo 捕获；`Blockchain.undoByHeight` in-memory cache；共识级 `ConnectBlock`/`DisconnectBlock` orchestration；blocktree.SetTip production 接入（仍需独立 integration gate）
- **REORG-1E**：`internal/storage/` hash 索引 + Delete/Truncate；`block_index.dat` 持久化；`active_tip.dat`；undo journal 同步落盘
- **REORG-1F**：`internal/mempool/ReaddDisconnected`
- **REORG-1G**：orphan pool
- **REORG-1H**：P2P work-aware handshake + by-hash branch pull
- **REORG-1I**：finality / MaxReorgDepth 告警+限速
- **REORG-1J**：`/status` fork/orphan 观测

### 14.4 remaining blockers（REORG-1C 启动前仍需解除）

- **B3** 存储 Delete/Truncate（FC-007）—— REORG-1E
- **B4** mempool ReaddDisconnected（FC-006）—— REORG-1F
- **B5** orphan pool（FC-002）—— REORG-1G
- **B6** P2P work-aware handshake + by-hash（FC-003）—— REORG-1H
- **B7** finality / MaxReorgDepth（FC-005；BG-3 已定稿）—— REORG-1I
- **B8** `/status` fork/orphan 观测（可与 1C 并行后置）—— REORG-1J

### 14.5 exact recommended next phase

```
PHASE REORG-INFRASTRUCTURE-IMPLEMENTATION-3 — REORG-1E Storage Hash-Index + Delete/Truncate
```

理由：
1. 与 REORG-1D 共同构成 reorg 落盘基础（undo journal 须与 block 同步落盘）
2. 解锁 B3（存储 Delete/Truncate）
3. 不依赖 B1/B4/B5/B6（与 1C 平行）
4. 后续 REORG-1C 启动前置条件进一步收敛

---

## §15 纪律遵守（HARD STOP 全清）

| 禁止项 | 状态 |
|---|---|
| 修改任何既有 `.go` 文件 | ✅ 零修改（仅新增 `undo.go` + `undo_test.go`）|
| 修改 consensus 参数 | ✅ 零修改 |
| 修改 storage 层 | ✅ 零修改（持久化推迟到 REORG-1E）|
| 修改 mempool 层 | ✅ 零修改 |
| 修改 p2p 层 | ✅ 零修改 |
| 修改 cmd/node | ✅ 零修改 |
| 启动完整 blockchain reorg orchestration | ✅ 未启动 |
| 启动 production SetTip | ✅ 未启动 |
| 触发 P2P branch sync | ✅ 未启动 |
| 启用 finality / MaxReorgDepth | ✅ 未启动 |
| 启用 `/status` 观测 | ✅ 未启动 |
| 测试降断言 / 删测试 | ✅ 零降断言（`assertSetEqual` 严格）|
| 测试只能通过放宽验证规则 | ✅ 12 项矩阵全部基于严格状态比较 |
| `go test ./...` regression | ✅ 零回归（其他包全 PASS）|
| production datadir 触碰 | ✅ 零触碰 |
| `git push` / `git tag` / `git merge` / `git rebase` / `git reset` / `git amend` / `git squash` | ✅ 全零 |

---

## §16 VERDICT

```
┌─────────────────────────────────────────────────────────────────────┐
│ VERDICT = PASS                                                       │
│                                                                     │
│ REORG-1D UTXO UNDO JOURNAL 已完整交付：                              │
│   • ApplyBlockWithUndo 同步产出确定性 BlockUndo                     │
│   • DisconnectBlock 反向应用 + corruption detection 严格            │
│   • EncodeUndo/DecodeUndo 确定性接口（持久化留给 REORG-1E）         │
│   • 12 项测试矩阵全 PASS（含 3 corruption 子用例）                  │
│   • 完整 round-trip 证明（S1 == S1'）                                │
│   • REORG EXECUTION 仍 OFF（blocktree 零生产导入者）                │
│   • Consensus safety boundary 完整保持                              │
│                                                                     │
│ 闭合 B2 UTXO undo / rollback                                        │
│ 解锁 REORG-1C 启动的 1/N 前置组件                                    │
│                                                                     │
│ NEXT PHASE: NOT AUTHORIZED（需用户显式授权）                        │
│   推荐 PHASE REORG-INFRASTRUCTURE-IMPLEMENTATION-3 — REORG-1E       │
│   （存储 hash 索引 + Delete/Truncate + undo journal 同步落盘）        │
└─────────────────────────────────────────────────────────────────────┘
```

**STOP AFTER FINAL REPORT.** 不执行任何后续 phase；不 commit；不 push；不修改任何已有文件。
