# PHASE REORG-1F-IMPLEMENTATION-1

## P2PChain — Mempool Resurrection / Disconnected Transaction Re-Add

**阶段性质：STRICT CONTROLLED IMPLEMENTATION / REORG + MEMPOOL INTEGRATION**

前置阶段：

- `REORG-1C-COMMIT-1` — PASS
- `REORG-1C-PUSH-1` — PASS
- `REORG-1D-DESIGN-1` — ALREADY COMPLETE
- `REORG-1F-DESIGN-1` — NOT READY
- `REORG-1F-PRE-IMPLEMENTATION-GATE-1` — READY FOR IMPLEMENTATION

当前 HEAD：

```
369d36e
```

远端：

```
main == 369d36e
```

当前 working tree 中已有 dirty state：

- `docs/DETERMINISTIC-SERIALIZATION-SPEC.md`
- `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md`

这些属于**既有修改**，不得纳入本阶段 commit。

---

# 1. HARD RULES

本阶段允许：

- 修改明确批准的 production code
- 新增明确批准的测试
- 修改必要的 reorg → mempool integration code

本阶段禁止：

- 修改协议参数
- 修改 PoW / Difficulty / Chainwork
- 修改 canonical tip selection 规则
- 修改 UTXO consensus semantics
- 修改 Undo semantics
- 修改 blocktree fork-choice
- 引入 transaction index
- 引入 orphan pool
- 引入 P2P branch sync
- 引入 block-by-hash retrieval
- 引入 RBF
- 引入 ancestor/descendant policy
- 引入 mempool persistence
- 引入 finality / MaxReorgDepth
- 修改 mining template
- 修改 reward model
- Linux production deployment
- Windows mining test
- 生产 datadir 写入
- push
- tag
- merge
- rebase
- amend
- squash

尤其禁止：

```
git add .
git add -A
```

---

# 2. PRE-IMPLEMENTATION BASELINE

实现前重新确认：

- HEAD
- parent
- branch
- remote/main
- working tree
- staging
- untracked files
- pre-existing dirty files

确认：

```
go test ./internal/...
```

以及现有相关测试 baseline。

如果 baseline 与 `REORG-1F-PRE-IMPLEMENTATION-GATE-1` 不一致：

**STOP，不要继续实现。**

---

# 3. IMPLEMENTATION SCOPE

严格限制在以下文件：

### Modify

```
internal/blockchain/blockchain.go
internal/mempool/mempool.go
cmd/node/service.go
```

### Create

```
internal/mempool/reorg_resurrection_test.go
```

除非编译或测试证明必须修改其他文件，否则不得扩展文件范围。

如果必须修改其他文件：

**STOP 并报告原因，等待重新授权。**

---

# 4. ARCHITECTURE — FROZEN

采用：

```
Architecture B
Structured ReorgResult + Service Layer
```

目标：

```
Blockchain
    │
    │ successful reorg
    ↓
ReorgResult
    │
    ↓
Service Layer
    │
    ↓
Mempool Resurrection
```

禁止：

```
Blockchain → Mempool direct dependency
```

Blockchain 层不得 import mempool package。

---

# 5. REORG RESULT

增加结构化 reorg result，至少能够可靠表达：

```
OldTip
NewTip
DisconnectPath
ConnectPath
```

具体字段类型必须复用现有 blocktree/blockchain 类型，不要重复创造新的 block representation。

`DisconnectPath` 必须保持当前已经验证的确定性顺序：

```
old tip → ... → common ancestor
```

`ConnectPath` 保持确定性：

```
common ancestor → ... → new tip
```

---

# 6. IMPORTANT IMPLEMENTATION CONSTRAINT

不要为了提供 `ReorgResult` 而改变：

- fork choice
- chainwork comparison
- common ancestor calculation
- block validation
- UTXO reconstruction
- CommitReorg semantics
- TIP commit ordering

原有 reorg correctness 必须保持不变。

尤其保持：

```
storage CommitReorg
        ↓
canonical state replacement
        ↓
tree.SetTip
```

的既有安全语义。

---

# 7. REORG RESULT LIFECYCLE

优先使用：

```
AddBlock
    ↓
executeReorg
    ↓
return ReorgResult
    ↓
service
```

如果现有调用链无法自然传递结果，才使用已经冻结的 accessor 方案。

如果实现 `LastReorgResult()`：

必须：

- 保证并发安全
- 明确 result 生命周期
- 不允许旧 reorg result 被误用于新的 block acceptance
- 不允许 result 状态影响 consensus
- 不产生 data race

**不得为了 accessor 引入不必要的全局状态。**

---

# 8. MEMPOOL API

实现：

```
Mempool.ReaddDisconnected(...)
```

职责仅限：

```
candidate collection
→ coinbase filtering
→ new-chain deduplication
→ validation
→ dependency-aware insertion
```

不得改变现有：

```
Add
Remove
RemoveIncluded
Pending
Has
Fee
All
Len
```

的既有 consensus/policy semantics。

---

# 9. CANDIDATE COLLECTION

从：

```
DisconnectPath
```

获取 block。

严格按照：

```
disconnectPath order
    ↓
block transaction order
```

扫描。

对于每个 block：

```
Transactions[0]
```

视为 coinbase，不进入 candidate set。

扫描：

```
Transactions[1:]
```

作为 resurrection candidates。

必须保持确定性。

不得使用 map iteration order 作为 candidate ordering。

---

# 10. NEW-CHAIN DEDUPLICATION

从：

```
ConnectPath
```

扫描所有 transactions。

构造：

```
newChainTxs map[[32]byte]struct{}
```

对于 disconnected candidate：

```
if txid ∈ newChainTxs:
    skip
```

禁止创建 persistent transaction index。

该 set：

- 每次 reorg 临时创建
- reorg 完成后释放
- 不进入 consensus state
- 不持久化

---

# 11. VALIDATION

resurrection 必须使用：

```
post-reorg canonical UTXO
```

禁止使用 old-chain UTXO。

调用现有：

```
mempool.Add(...)
```

作为最终 admission boundary。

不得复制：

```
ValidateTransaction
```

的逻辑。

不得创建第二套 mempool validation。

目标：

```
new canonical UTXO
        +
existing mempool
        +
already resurrected parents
        ↓
mempool.Add
```

---

# 12. DEPENDENCY RESTORATION

采用：

```
MULTI-PASS
```

算法。

伪逻辑：

```
remaining = candidates

repeat:
    progress = false

    for tx in remaining:
        attempt Add(tx)

        if success:
            remove tx from remaining
            progress = true

    if !progress:
        break
```

要求：

- deterministic iteration
- successful parent can enable child
- unresolved invalid dependency eventually terminates
- no infinite loop
- no duplicate insertion

最大 pass 数必须有理论上限：

```
≤ number of candidates
```

---

# 13. ERROR SEMANTICS

以下情况必须属于正常 resurrection rejection，而不是 reorg failure：

```
ErrKnownTx
ErrConflict
ErrUnknownUTXO
ErrPoolFull
invalid transaction
coinbase
```

原则：

```
candidate rejected
      ≠
reorg failed
```

不得出现：

```
mempool.Add error
      ↓
canonical reorg rollback
```

---

# 14. CAPACITY POLICY

严格沿用现有：

```
maxSize
ErrPoolFull
```

如果 pool 已满：

```
candidate dropped
```

禁止新增：

- eviction
- lowest-fee replacement
- RBF
- fee bumping
- ancestor limit
- descendant limit

---

# 15. COINBASE

双重保护：

### Collection

跳过：

```
Transactions[0]
```

### Admission

即使异常进入：

```
mempool.Add
```

仍由现有：

```
ErrCoinbaseIn
```

拒绝。

不得增加 coinbase maturity bypass。

---

# 16. SERVICE INTEGRATION

在：

```
cmd/node/service.go
```

现有：

```
addBlockAndUpdatePool
```

路径中接入。

目标：

```
new block
   ↓
AddBlock
   ↓
if reorg:
       ReorgResult
          ↓
       resurrect disconnected txs
   ↓
RemoveIncluded(new canonical block)
```

注意：

**必须先完成 canonical reorg，再进行 resurrection。**

不要让 resurrection 参与：

- block validation
- fork choice
- TIP commit
- chainwork comparison

---

# 17. ORDER WITH RemoveIncluded

实现时必须仔细审计：

```
resurrection
```

与：

```
RemoveIncluded
```

的先后关系。

最终必须保证：

> 新 canonical block 中确认的交易不会出现在 mempool。

如果 `RemoveIncluded` 已经处理了 new-tip block，则 resurrection 逻辑不得重新加入这些交易。

如果需要调整调用顺序：

必须通过测试证明：

```
M1 canonical transactions absent
```

始终成立。

不得凭猜测调整。

---

# 18. REQUIRED TESTS

新增：

```
internal/mempool/reorg_resurrection_test.go
```

至少实现以下 12 个测试：

### P0

```
TestReorgResurrectsDisconnectedTx
TestReorgDoesNotResurrectCoinbase
TestReorgDoesNotResurrectConfirmedTx
TestReorgRejectsInvalidUnderNewUTXO
TestReorgRejectsDoubleSpend
```

### P1

```
TestReorgResurrectsDependentTxs
TestReorgMultiLevelDependency
TestReorgRepeatedResurrectionNoDuplicate
TestReorgMempoolCapacityRespected
TestReorgResurrectionFailureDoesNotAffectTIP
```

### P2

```
TestReorgZeroResurrectableTransactions
TestReorgMultipleDisconnectedBlocks
```

---

# 19. TEST REQUIREMENTS

测试必须分别验证：

### Consensus

```
canonical TIP
canonical chain
UTXO state
```

### Mempool

```
transaction presence
transaction absence
dependency ordering
duplicate behavior
capacity behavior
```

不能只测试：

```
mempool.Len()
```

而不验证 canonical chain。

---

# 20. CRASH / FAILURE TEST

至少一个测试必须证明：

```
resurrection failure
      ↓
canonical TIP remains new tip
```

不得通过人工修改 production state 来模拟。

优先使用一个必然被 mempool rejection 的 candidate。

验证：

```
TIP == new canonical tip
```

同时：

```
invalid candidate NOT in mempool
```

---

# 21. REORG REGRESSION

实现完成后必须重新运行：

```
go test ./internal/blockchain/...
go test ./internal/mempool/...
go test ./internal/storage/...
go test ./internal/utxo/...
```

以及：

```
go test ./internal/...
```

如果已有 blocktree flaky BT-1 再次出现：

必须区分：

```
pre-existing BT-1
```

与：

```
1F regression
```

不得为了让测试全绿而修改 BT-1。

---

# 22. RACE TEST

必须运行与 reorg/mempool 相关的 race tests。

至少覆盖：

```
blockchain
mempool
service integration
```

发现 data race：

**STOP，不能进入 commit。**

---

# 23. STATIC / FORMAT CHECKS

必须执行：

```
gofmt
go vet
git diff --check
```

禁止把 pre-existing docs whitespace warning 误认为本阶段代码问题。

---

# 24. REQUIRED INVARIANTS

实现后必须证明：

### M1

Canonical chain transactions never remain in mempool.

### M2

Disconnected non-coinbase transactions can become resurrection candidates.

### M3

Every candidate is validated against NEW canonical UTXO.

### M4

Mempool conflicts/double-spends are rejected.

### M5

Dependencies are restored safely.

### M6

Mempool never influences canonical tip selection.

### M7

Mempool failure never rolls back canonical reorg.

### M8

Repeated resurrection never duplicates transactions.

### M9

Same chain + same candidate set + same mempool state produces deterministic acceptance.

### M10

Coinbase is never resurrected.

---

# 25. IMPLEMENTATION DIFF BOUNDARY

Expected primary scope:

```
internal/blockchain/blockchain.go
internal/mempool/mempool.go
cmd/node/service.go
internal/mempool/reorg_resurrection_test.go
```

Expected approximate size:

```
~110 production lines
~400 test lines
```

这些只是估算，不是硬性行数目标。

**Correctness > line count.**

不得为了达到行数而增加无必要抽象。

---

# 26. POST-IMPLEMENTATION AUDIT

实现完成后：

1. 查看 `git status`
2. 查看 `git diff --stat`
3. 查看精确 diff
4. 确认只有授权文件发生变化
5. 运行全部测试
6. 运行 race
7. 运行 vet
8. `git diff --check`
9. 检查生产 datadir 未触碰
10. 检查没有新增协议参数
11. 检查没有新增 persistent mempool state
12. 检查没有 blockchain → mempool dependency

---

# 27. COMMIT RULE

本阶段：

**不要 commit。**

实现完成后只生成：

```
PHASE REORG-1F-IMPLEMENTATION-1 — FINAL REPORT
```

然后 STOP。

下一阶段将单独进行：

```
REORG-1F-COMMIT-READINESS-AUDIT
```

不得自动 commit。

不得 push。

---

# 28. FINAL REPORT

报告必须明确：

```
VERDICT
HEAD
PARENT
FILES MODIFIED
FILES CREATED
DIFF STAT
TEST RESULTS
RACE RESULTS
VET
GIT DIFF CHECK
INVARIANTS M1-M10
PRODUCTION DATADIR STATUS
SCOPE COMPLIANCE
KNOWN LIMITATIONS
```

并回答：

```
1. Does resurrection ever affect consensus?
2. Does resurrection use post-reorg UTXO?
3. Are confirmed new-chain transactions excluded?
4. Are coinbase transactions excluded?
5. Are dependent transactions restored safely?
6. Can mempool failure roll back reorg?
7. Is repeated resurrection idempotent?
8. Is behavior deterministic?
```

最终只能给：

```
IMPLEMENTATION PASS
```

或：

```
IMPLEMENTATION BLOCKED
```

完成报告后立即 STOP。






