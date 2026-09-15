# PHASE REORG-1C-IMPLEMENTATION-1 — CONTROLLED REORG CONSENSUS IMPLEMENTATION

你现在进入 P2PChain 的：

**STRICT CONTROLLED IMPLEMENTATION / CONSENSUS SAFETY / REORG EXECUTION**

前置设计阶段：

`PHASE REORG-1C-COMMIT-MODEL-DESIGN-1`

前置结论：

**MODEL A — TIP AS SOLE CANONICAL COMMIT POINT**

已经通过设计审计。

---

## HARD RULES

1. 先建立只读 baseline。
2. 不允许 push / tag / merge / rebase / amend / squash。
3. 不允许修改无关模块。
4. 不允许修改既有测试断言来“适配实现”。
5. 不允许引入 Reorg Journal。
6. 不允许持久化 UTXO。
7. 不允许实现 P2P branch sync。
8. 不允许实现 mempool re-add。
9. 不允许实现 MaxReorgDepth。
10. 不允许实现 `/status` fork reporting。
11. 不提前扩展 Mining / Reward / Network 功能。
12. 不改变现有 legacy prefix immutable contract。
13. 不改变 Storage v2 的 TIP canonical semantics。
14. 所有 production datadir 均禁止触碰。
15. 如发现规格冲突、既有测试与设计冲突、或需要扩大 scope，立即 STOP，输出冲突，不自行决定。

---

# PRIMARY OBJECTIVE

实现完整的本地 Reorg execution path：

```text
fork detection
→ common ancestor
→ disconnect old branch
→ apply new branch
→ atomic persistent commit
→ memory canonical-state update
```

必须保证：

```text
old TIP
OR
new valid TIP
```

永远是 restart 后唯一可能的 canonical state。

---

# IMPLEMENTATION ORDER

## M1 — BlockTree / Common Ancestor

实现：

```text
FindCommonAncestor
```

并将 BlockTree 正确接入 Blockchain。

必须支持：

```text
ancestor
old branch
new branch
```

明确产生：

```text
disconnectPath
connectPath
```

要求：

- height 正确
- parent linkage 正确
- hash linkage 正确
- genesis boundary 正确
- 不允许循环
- 不允许跨 fork 错配

---

## M2 — Fork Detection

在 `AddBlock` 路径中识别：

```text
same-chain extension
vs
competing branch
```

不得仅依赖 height。

必须确认：

```text
parent hash
active tip ancestry
blocktree membership
```

---

## M3 — Disconnect / Undo

实现或接通：

```text
DisconnectBlock
```

要求：

- 使用已经持久化的 UNDO
- 不允许 silent UTXO mutation
- 不允许重复 disconnect
- 不允许 disconnect 到 genesis 以下
- 必须验证 undo 与 block 完全匹配

如需要 clone / dry-run，必须先验证 rollback correctness。

---

## M4 — Apply New Branch

使用：

```text
ApplyBlockWithUndo
```

而不是重新创造第二套状态迁移逻辑。

要求：

```text
ancestor UTXO
→ block N
→ block N+1
→ ...
→ new tip
```

每一步必须满足：

```text
valid block
valid UTXO transition
valid undo
continuous height
continuous prevHash
```

---

## M5 — CommitReorg

在：

```text
internal/storage/v2api.go
```

增加最小必要 API：

```text
CommitReorg(...)
```

不要新增 persistent journal。

最终持久化语义：

```text
UNDO existing detached blocks if required
UNDO + BLOCK for newly received blocks
TIP(newTip)
```

使用现有：

```text
appendFrames(...)
```

并保持 TIP 为唯一 canonical commit marker。

---

# CRITICAL SAFETY RULE

不要假设：

> filesystem multi-frame write = transactional atomicity

真正必须证明的是：

```text
No valid new TIP
    →
old TIP remains canonical

Valid new TIP
    →
new canonical chain may be selected
```

因此 recovery correctness 必须依赖：

```text
scanLog
→ interpretScan
→ validateTipCandidate
→ setCanonicalFrom
```

而不是依赖“文件系统一定全部写入或全部不写入”。

---

# M6 — Reorg Orchestration

组合：

```text
detect fork
→ ShouldReorg
→ FindCommonAncestor
→ disconnect old branch
→ apply new branch
→ CommitReorg
→ update in-memory tip
```

必须保证 Blockchain mutex 与 FileBlockStore mutex 的使用顺序明确、固定、无 deadlock。

不得形成：

```text
Blockchain.mu → Storage.mu
```

和其他路径相反顺序获取锁的情况。

---

# M7 — Restart Reconstruction

确认：

```text
NewBlockchainFromStore
```

可以从：

```text
last valid TIP
```

重新构建：

```text
blocktree
canonical path
byHeight
byHash
UTXO
```

并证明：

```text
restart(old-tip-state)
=
old chain + old UTXO

restart(new-tip-state)
=
new chain + new UTXO
```

---

# M8 — Mandatory Crash Tests

至少增加以下测试：

### TEST A — crash before TIP

Expected:

```text
old TIP
old canonical chain
old UTXO after restart
```

### TEST B — new TIP fully persisted

Expected:

```text
new TIP
new canonical chain
new UTXO after restart
```

### TEST C — torn TIP

Expected:

```text
invalid TIP rejected
previous valid TIP selected
```

### TEST D — detached blocks before commit

Expected:

```text
blocks exist
canonical=false
old TIP remains active
```

### TEST E — reorg followed by restart

Expected:

```text
active tip identical
canonical path identical
UTXO identical
```

### TEST F — repeated reorg evaluation after rollback

Expected:

```text
no duplicated UTXO effects
no corrupted state
no duplicate canonical blocks
```

---

# REQUIRED INVARIANTS

必须证明：

### R1

At every crash boundary:

```text
exactly one valid canonical TIP
```

### R2

Detached blocks cannot become canonical without a valid TIP.

### R3

UTXO is exactly derivable from canonical chain.

### R4

A partially persisted reorg cannot create an invalid canonical chain.

### R5

Same persisted storage state produces identical:

```text
TIP
canonical path
UTXO
```

---

# SCOPE LIMIT

本阶段只实现：

```text
local reorg consensus execution
+
persistent atomic canonical commit
+
restart safety
```

明确 DEFER：

```text
P2P branch sync
mempool re-add
MaxReorgDepth
status fork visibility
reorg journal
crash resume
mining fairness
network capacity
reward model
```

---

# STOP CONDITIONS

任何以下情况出现立即 STOP：

- existing test contradicts implementation
- consensus rule ambiguity
- undo semantics ambiguous
- lock ordering ambiguity
- TIP recovery ambiguity
- storage invariant violation
- legacy prefix mutation required
- need to modify unrelated production code
- need to weaken an existing assertion
- need for a new persistent state source
- need for Reorg Journal

STOP 后只输出：

1. conflict
2. affected invariant
3. exact files/functions
4. recommended options
5. safest option

不要自行选择。

---

# FINAL REQUIREMENT

完成后不要直接 commit。

先输出：

**PHASE REORG-1C-IMPLEMENTATION-1 — COMMIT READINESS AUDIT**

必须包含：

- HEAD
- parent
- changed files
- diff statistics
- all tests
- consensus tests
- crash tests
- race tests
- git diff --check
- production datadir untouched proof
- legacy prefix untouched proof
- lock-order audit
- R1–R5 verification
- explicit scope audit
- remaining limitations
- commit recommendation

只有得到下一步明确授权后，才允许进入 commit execution。














