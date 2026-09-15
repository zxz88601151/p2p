# PHASE REORG-1F-PRE-IMPLEMENTATION-GATE-1

## P2PChain — Mempool Resurrection / Reorg Integration Pre-Implementation Safety Gate

**阶段性质：STRICT READ-ONLY / CONSENSUS SAFETY / MEMPOOL INTEGRATION GATE**

前置阶段：

- `PHASE REORG-1C-COMMIT-1` — PASS / CLOSED
- `PHASE REORG-1C-PUSH-1` — PASS / REMOTE SYNCED
- `PHASE REORG-1D-DESIGN-1` — ALREADY COMPLETE
- `PHASE REORG-1F-DESIGN-1` — NOT READY / 5 gaps identified

当前已知核心状态：

- canonical reorg execution 已存在并已提交
- UTXO undo infrastructure 已存在并已验证
- mempool 基础 admission / validation / conflict detection 已存在
- 当前 reorg execution 与 mempool 完全没有集成
- disconnected transaction resurrection 尚未实现

---

# 1. HARD RULES

本阶段必须严格遵守：

- 只读审计
- 不修改任何 `.go`
- 不修改测试
- 不修改协议
- 不修改配置
- 不修改生产 datadir
- 不执行生产部署
- 不启动 Linux/Windows mining test
- 不 push
- 不 commit
- 不 tag
- 不 amend
- 不 rebase
- 不 merge
- 不重建 installer
- 不改变现有测试断言

本阶段唯一目标：

> 判断 REORG-1F 是否具备安全进入 implementation 的条件，并冻结正确的 integration boundary。

---

# 2. 必须首先建立硬基线

记录：

- HEAD
- parent
- branch
- local vs remote
- working tree
- staging
- untracked files
- 当前已有未提交文件
- `git diff --check`
- 当前测试基线

明确区分：

1. 本阶段之前已有修改
2. REORG-1F 相关修改

禁止把历史遗留 dirty state 误认为本阶段变更。

---

# 3. 审计 Blockchain → Mempool Integration Boundary

重点检查：

- `executeReorg`
- `addBlock`
- `AddBlock`
- reorg success return path
- service/nodeService
- block acceptance path
- block confirmation后的 `RemoveIncluded`
- mempool 生命周期

回答：

### Q1

reorg 的哪个时刻代表：

> canonical TIP 已经成功提交，新的 canonical UTXO 已经确定？

### Q2

mempool resurrection 是否必须发生在 TIP commit 之后？

### Q3

如果 resurrection 完全失败，canonical reorg 是否仍必须保持成功？

### Q4

当前架构最干净的 integration boundary 是：

A. `executeReorg` 内部直接调用 mempool

B. blockchain 返回 reorg result，由 service layer 执行 resurrection

C. callback / observer

D. 其他

必须给出明确推荐及理由。

---

# 4. 冻结 Consensus / Mempool 边界

必须证明：

```text
Consensus decides canonical chain.
Mempool never decides canonical chain.
```

检查是否存在任何可能导致：

```text
mempool.Add failure
        ↓
reorg rollback
```

或：

```text
mempool state
        ↓
TIP selection
```

如果存在，必须标记 BLOCKER。

目标模型应为：

```text
REORG COMMIT
     │
     ├── success → resurrect candidates
     │                 │
     │                 ├── accepted
     │                 └── rejected
     │
     └── failure → no mempool resurrection
```

---

# 5. 审计 Disconnected Transaction 数据来源

确认 reorg execution 当前实际能够可靠获得：

- disconnect path
- old canonical blocks
- each block's transactions
- transaction ordering
- block ordering

特别确认：

```text
ancestor
   ↓
old branch blocks
   ↓
disconnect order
```

是否确定且可重复。

不要只假设 `disconnectPath` 一定存在。

必须检查真实代码。

---

# 6. 审计 New-Chain Deduplication

必须明确：

> 如何证明某个 disconnected transaction 已经存在于新的 canonical chain？

评估至少两种方式：

### Option A

扫描 new canonical connect path

### Option B

从 canonical chain / transaction index 查询

必须根据当前真实代码选择。

不要新增 transaction index，除非审计证明现有结构完全无法完成需求。

必须冻结：

```text
newCanonicalTxSet
```

的生命周期和构造方式。

---

# 7. 审计 Re-validation 时的 UTXO 状态

这是本阶段最重要的审计之一。

必须明确 resurrection 使用的 UTXO：

```text
old-chain UTXO
```

还是：

```text
new canonical UTXO
```

必须证明只能使用：

> **new canonical UTXO + current mempool composite view**

同时审计：

- `mempool.Add`
- `compositeView`
- `ValidateTransaction`
- UTXO lookup
- mempool conflict detection

确认：

```text
chain UTXO
    +
existing mempool txs
    +
successfully resurrected parent txs
```

能够形成正确 validation view。

---

# 8. 审计 Parent → Child Resurrection

建立明确测试场景：

```text
OLD:

A → B(tx1) → C(tx2)

tx2 spends tx1 output
```

发生：

```text
A → D → E
```

其中：

- tx1 未被新链确认
- tx2 未被新链确认

要求：

```text
tx1
 ↓
mempool
 ↓
tx2
 ↓
mempool
```

不得允许：

```text
tx2 first → reject permanently
```

必须判断：

- multiple-pass 是否足够
- 是否需要 dependency graph
- 是否需要 deterministic candidate ordering

1F 第一版优先保持简单。

---

# 9. 特别审计“跨候选依赖”

检查：

```text
tx1 → tx2 → tx3
```

以及：

```text
tx1 invalid
tx2 depends on tx1
tx3 depends on tx2
```

预期：

```text
tx1 rejected
tx2 rejected
tx3 rejected
```

而不是：

```text
tx1 rejected
tx2 accidentally accepted
```

同时检查：

```text
tx1 accepted
tx2 accepted
tx3 accepted
```

是否具有确定性。

---

# 10. Coinbase Boundary

明确：

```text
disconnected block
 ├── coinbase
 ├── tx1
 ├── tx2
 └── ...
```

只有：

```text
tx1 / tx2 / ...
```

可以成为 resurrection candidate。

coinbase 永远不能进入 mempool。

同时确认：

> 不应为了 resurrection 引入 coinbase maturity 的特殊 mempool bypass。

---

# 11. Capacity / Fee Policy Boundary

特别审计：

- `maxSize`
- `Pending`
- fee ordering
- current pool-full behavior

不要在本阶段设计新的：

- eviction policy
- RBF
- ancestor limits
- descendant limits
- fee replacement

如果 resurrection 遇到：

```text
mempool full
```

只冻结当前已有 policy 的真实行为。

不要自行发明“最低手续费 eviction”。

---

# 12. Failure / Crash Semantics

必须验证设计允许：

```text
TIP commit
   ↓
process crash
   ↓
mempool resurrection never happens
```

且：

```text
canonical chain remains correct
```

因此必须明确：

> resurrection 不是 consensus atomic transaction。

禁止设计：

```text
TIP + mempool
```

作为一个必须原子提交的状态。

---

# 13. Repeated Reorg / Idempotency

设计并审计：

```text
same reorg
same disconnected tx
```

重复触发时：

```text
ErrKnownTx
```

或等价 duplicate protection 必须阻止重复。

还必须检查：

```text
tx confirmed on new chain
```

时不得 resurrect。

---

# 14. Integration Architecture Decision

最终必须在以下架构中选一个：

### A

`executeReorg()` 直接依赖 mempool

### B

`executeReorg()` 返回结构化 reorg result：

```text
ReorgResult {
    OldTip
    NewTip
    DisconnectPath
    ConnectPath
}
```

然后由上层 service 执行：

```text
ReorgResult
    ↓
mempool resurrection
```

### C

callback / observer

### D

其他

优先考虑：

> **B：blockchain consensus layer 不直接依赖 mempool，service layer 在 reorg commit 成功后处理 resurrection。**

但必须根据真实代码审计确认，而不是机械接受。

---

# 15. Required Invariants

冻结以下 invariants：

### M1

Canonical chain transactions are never left in mempool.

### M2

Disconnected non-coinbase transactions may become resurrection candidates.

### M3

Every resurrection candidate is validated against NEW canonical UTXO.

### M4

Mempool conflicts / double spends are rejected.

### M5

Dependencies are restored parent-before-child.

### M6

Mempool state never influences canonical tip selection.

### M7

Mempool failure never rolls back or corrupts canonical reorg.

### M8

Repeated resurrection never creates duplicate txids.

### M9

Same canonical chain + same candidate set + same existing mempool state produces deterministic acceptance, subject to existing policy.

### M10

Coinbase transactions are never resurrected.

---

# 16. Required Test Architecture Review

不实现测试，只审计测试设计。

至少冻结：

1. basic resurrection
2. coinbase exclusion
3. confirmed-on-new-chain exclusion
4. invalid-under-new-UTXO rejection
5. double-spend rejection
6. parent → child resurrection
7. multi-level dependency
8. repeated resurrection / duplicate protection
9. mempool capacity behavior
10. resurrection failure does not affect canonical TIP
11. reorg with zero resurrectable transactions
12. multiple disconnected blocks with deterministic candidate ordering

同时确认：

> 测试必须验证 canonical chain correctness 与 mempool state separately。

---

# 17. Important Scope Control

本阶段明确禁止顺手实现：

- orphan pool
- P2P branch synchronization
- block-by-hash retrieval
- mining template redesign
- mempool persistence
- RBF
- ancestor/descendant policy
- finality
- MaxReorgDepth
- reward redesign
- Mining Pool / Stratum
- Linux deployment
- Windows mining
- production testing

---

# 18. Final Verdict

最终只能给出以下之一：

```text
READY FOR IMPLEMENTATION
```

或：

```text
NOT READY
```

如果 READY，必须明确冻结：

1. integration boundary
2. data flow
3. disconnected transaction source
4. new-chain deduplication mechanism
5. new-UTXO validation point
6. dependency restoration strategy
7. failure semantics
8. capacity-policy behavior
9. test matrix
10. exact implementation scope

如果 NOT READY：

- 明确 blocker
- 明确为什么不能进入实现
- 给出下一步最小审计阶段
- STOP

---

# 19. Final Rule

本阶段完成后立即 STOP。

禁止自动进入 implementation。

即使所有审计 PASS，也必须等待新的显式授权后才能实施。


