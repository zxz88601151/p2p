# PHASE REORG-1F-REMEDIATION-1

## P2PChain — F-1 Canonical-vs-Detached Reorg Safety + F-2 Failure-Test Remediation

### 阶段性质

STRICT CONTROLLED REMEDIATION

前置阶段：

`PHASE REORG-1F-COMMIT-READINESS-AUDIT-1`

前置结论：

```text
VERDICT: NOT READY
BLOCKING: F-1 P1
SECONDARY: F-2 P1 test-evidence defect
```

---

# 1. HARD STOP RULES

本阶段只允许解决：

```text
F-1
F-2
```

禁止借机处理：

```text
F-3
F-4
F-5
F-6
F-7
BT-1
```

除非某项被证明是 F-1/F-2 修复所绝对必需的，否则不得修改。

本阶段：

- ❌ 不 commit
- ❌ 不 push
- ❌ 不 tag
- ❌ 不 merge
- ❌ 不 rebase
- ❌ 不 amend
- ❌ 不 squash
- ❌ 不触碰生产 datadir
- ❌ 不部署服务器
- ❌ 不生成/替换生产二进制

---

# 2. F-1 ROOT PROBLEM

当前错误逻辑：

Case 2 的 corruption protection 使用：

```text
blockAtHash(hash)
```

但当前 `blockAtHash` 在 v2 store 下可能命中：

```text
canonical block
+
detached / non-canonical block
```

因此：

```text
detached fork block
        ↓
blockAtHash hit
        ↓
被误认为 canonical duplicate
        ↓
reject
```

这违反：

1. canonical duplicate 必须拒绝；
2. detached fork 不得被误杀；
3. 合法 re-delivery 必须保持既有幂等语义。

并且 sync service 对 addBlock error 的处理可能导致：

```text
sync batch abort
        ↓
syncing 未复位
        ↓
后续 requestSync 不再执行
        ↓
node stuck in syncing state
```

---

# 3. F-1 REQUIRED SEMANTICS

必须实现并证明以下四类行为：

### Case A — canonical duplicate corruption

同一个 block hash 已经存在于 canonical chain：

```text
canonical contains H
incoming H
```

结果：

```text
REJECT
```

不得静默接受。

---

### Case B — detached fork re-delivery

block hash 已经存在于 v2 store，但属于：

```text
detached / non-canonical
```

incoming 同一个 fork block：

```text
detached contains H
canonical does NOT contain H
incoming H
```

结果必须保持原有合法幂等语义：

```text
DO NOT reject as canonical corruption
```

不得因为 detached store hit 而误杀。

---

### Case C — new fork block

block 不存在于 canonical chain，也不存在 detached record：

```text
new legitimate fork block
```

必须正常进入既有 fork/reorg 流程。

---

### Case D — sync re-delivery

sync 批次中遇到合法重复 fork block：

```text
duplicate detached block
```

不得因为 F-1 防护造成：

```text
sync batch abort
syncing stuck
```

必须保持既有幂等/同步推进语义。

---

# 4. IMPORTANT ARCHITECTURAL RULE

不要简单地：

```text
blockAtHash → another global store lookup
```

而必须明确表达：

```text
canonical membership
```

与：

```text
store existence
```

是两个不同概念。

优先使用已经存在的 canonical-chain authoritative state。

如果当前代码没有合适的 canonical-only lookup：

1. 先定位现有 `bc.blocks` / canonical index / active chain 数据结构；
2. 选择最小、最明确的 canonical membership 判断；
3. 不新增复杂索引；
4. 不引入新的持久化机制；
5. 不修改 REORG 架构。

如果发现需要新增较大架构：

> STOP，并报告，不得自行扩展范围。

---

# 5. F-1 REGRESSION TESTS

必须新增或补强针对性测试，至少覆盖：

```text
canonical duplicate → reject

detached duplicate → idempotent / not rejected as corruption

new fork block → accepted

re-delivered detached block → no sync-abort behavior
```

其中 detached duplicate 必须真实构造：

```text
canonical chain
    +
detached fork block stored in v2 store
    +
same detached block delivered again
```

不能只测试纯内存路径。

---

# 6. SYNCING STATE REGRESSION

必须检查：

```text
cmd/node/service.go
```

确认 F-1 修复后：

```text
合法 detached duplicate
        ↓
不会产生 addBlock error
        ↓
不会触发 sync batch premature return
        ↓
不会使 syncing 永久保持 true
```

如果必须修改 service.go 才能证明这一点：

> 只允许作为 F-1 必要修复。

不得顺便处理 F-5 TOCTOU。

---

# 7. F-2 TEST EVIDENCE REMEDIATION

当前：

```text
TestReorgResurrectionFailureDoesNotAffectTIP
```

使用：

```text
mempool.New(0)
```

但当前语义是：

```text
maxSize <= 0 => unlimited
```

因此测试没有真正制造 resurrection failure。

必须改成真实失败条件。

优先使用：

```text
mempool.New(1)
```

并构造：

```text
至少 2 个可复活 transaction
```

使：

```text
accepted = 1
rejected >= 1
```

然后严格验证：

```text
TIP unchanged / canonical reorg remains successful
```

必须证明：

> mempool resurrection failure cannot roll back or invalidate the canonical reorg.

不要修改 `mempool.New` 的既有容量语义。

---

# 8. DO NOT FIX F-3/F-4/F-5/F-6/F-7

以下全部保持登记状态：

### F-3

`addLocked` / `compositeViewLocked` duplication。

暂不重构。

### F-4

DisconnectBlocks store fallback / silent loss。

暂不改变。

### F-5

`lastReorgResult` TOCTOU。

暂不处理。

### F-6

pre-reorg height。

暂不处理。

### F-7

extendChain error swallowing。

暂不处理。

### BT-1

blocktree historical flaky。

暂不修。

如果修复 F-1 的过程中证明某个问题不可避免地需要触及上述项目：

> STOP 并报告具体依赖关系。

---

# 9. FILE SCOPE

优先限制在：

```text
internal/blockchain/blockchain.go
cmd/node/service.go
internal/mempool/reorg_resurrection_test.go
```

如果 F-1/F-2 确实需要修改：

```text
internal/mempool/mempool.go
```

也必须说明原因。

禁止新增其他 production architecture。

---

# 10. VALIDATION

修复完成后必须执行：

```bash
go test -count=1 ./internal/blockchain/
go test -count=1 ./internal/mempool/
go test -count=1 ./internal/...
```

以及：

```bash
go test -race -count=1 ./internal/blockchain/ ./internal/mempool/
```

以及：

```bash
go vet ./...
```

以及：

```bash
go build ./...
```

以及：

```bash
gofmt -l \
  internal/blockchain/blockchain.go \
  internal/mempool/mempool.go \
  cmd/node/service.go \
  internal/mempool/reorg_resurrection_test.go
```

以及：

```bash
git diff --check
```

---

# 11. TEST MATRIX

必须重新确认：

```text
TestReorgResurrectsDisconnectedTx
TestReorgDoesNotResurrectCoinbase
TestReorgDoesNotResurrectConfirmedTx
TestReorgRejectsInvalidUnderNewUTXO
TestReorgRejectsDoubleSpend
TestReorgResurrectsDependentTxs
TestReorgMultiLevelDependency
TestReorgRepeatedResurrectionNoDuplicate
TestReorgMempoolCapacityRespected
TestReorgResurrectionFailureDoesNotAffectTIP
TestReorgZeroResurrectableTransactions
TestReorgMultipleDisconnectedBlocks
```

另外必须有 F-1 对应的 canonical-vs-detached regression tests。

---

# 12. PRODUCTION SAFETY

确认：

- 所有测试使用 temporary datadir；
- 不连接 `.123`；
- 不连接 `.200`；
- 不部署；
- 不生成生产部署文件；
- 不修改服务器；
- 不修改生产数据。

---

# 13. GIT DISCIPLINE

修复完成后：

```bash
git status --short
git diff --stat
git diff --name-status
```

不得：

```text
git add
git commit
git push
git tag
git merge
git rebase
git amend
git squash
```

必须明确指出：

```text
pre-existing docs
run-a/
run-b/
other untracked artifacts
```

不得混入本阶段。

---

# 14. FINAL REPORT

报告必须回答：

1. F-1 根因是什么？
2. canonical membership 如何与 detached store existence 分离？
3. canonical duplicate 是否仍然拒绝？
4. detached duplicate 是否恢复合法幂等语义？
5. new fork 是否正常？
6. sync batch 是否仍可能因该问题 abort？
7. `syncing` 是否可能因此永久卡死？
8. F-2 是否真正制造了 resurrection failure？
9. TIP 是否在 resurrection failure 下保持成功？
10. 哪些文件发生变化？
11. F-3~F-7 是否保持未处理？
12. BT-1 是否保持未处理？
13. 所有测试/race/vet/build/gofmt/diff-check 是否通过？
14. 是否触碰生产 datadir？

最终只能给出：

```text
VERDICT: PASS
```

或：

```text
VERDICT: NOT READY
```

### 最重要的停止条件

如果修复 F-1 需要改变 consensus architecture、storage model、blocktree semantics 或扩大文件范围：

> **立即 STOP。**

不要为了让测试通过而扩大 REORG-1F。

本阶段完成后仍然：

> **不得 commit。**

下一阶段重新执行：

`PHASE REORG-1F-COMMIT-READINESS-AUDIT-2`




