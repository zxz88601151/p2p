# PHASE REORG-1G — FINAL REORG CONSENSUS / RECOVERY / DEPLOYMENT GAP AUDIT

## 阶段性质

**STRICT READ-ONLY / CONSENSUS SAFETY / REORG FINAL GAP AUDIT**

本阶段只允许：

- 阅读代码
- 阅读现有测试
- 阅读已有 REORG 报告
- 阅读 Git 历史
- 只读检查本地环境
- 如确有必要，只读观察当前 Linux/远端部署状态

**严禁：**

- 修改任何 `.go` / `.js` / `.json` / 配置文件
- `git add`
- `git commit`
- `git push`
- `git reset`
- `git checkout`
- `git restore`
- `git clean`
- `git rebase`
- `git merge`
- 修改服务器生产环境
- 重启生产节点
- 修改生产 datadir
- 修改数据库/blocks.dat/UTXO 状态
- Windows 挖矿测试
- 新增功能实现

---

# 1. Git Baseline

首先建立并报告：

- HEAD
- parent
- branch
- remote
- working tree
- staging
- untracked files
- 最近 10 个 commit

特别确认：

`e04d678b4fb33a14ea186999375f6d6dac2a4c4b`

是否仍为当前 HEAD。

确认：

```text
e04d678
feat: restore transactions after chain reorganization
```

如果 HEAD 已变化，立即 STOP 并报告，不得自行处理。

---

# 2. REORG-1 完整阶段链核对

读取并核对当前已经完成的：

- REORG-1A
- REORG-1B
- REORG-1C
- REORG-1D
- REORG-1E
- REORG-1F
- REORG-1F-COMMIT-READINESS-AUDIT-1
- REORG-1F-COMMIT-READINESS-AUDIT-2
- REORG-1F-COMMIT-1

建立：

```text
REORG-1 STATUS MATRIX
```

每一项必须标记：

- PASS
- PASS WITH LIMITATION
- DEFERRED
- BLOCKED
- NOT AUDITED

禁止根据名称猜测状态，必须根据实际报告内容确认。

---

# 3. Reorg Consensus Safety Audit

对当前 HEAD 的实际代码重新审计：

## 3.1 Chain Tree

确认：

- block tree
- active tip
- cumulative work
- fork tracking
- canonical chain determination

是否形成完整一致的状态机。

## 3.2 Reorg Execution

确认：

```text
old canonical tip
        ↓
find fork point
        ↓
disconnect old chain
        ↓
restore UTXO
        ↓
connect new chain
        ↓
update canonical tip
        ↓
persist canonical state
        ↓
revalidate / resurrect mempool
```

是否存在：

- partial state
- stale tip
- stale UTXO
- stale block index
- duplicate persistence
- detached/canonical ambiguity
- crash window

---

# 4. ReorgResult / Mempool Boundary Audit

重点确认：

`ReorgResult`

是否真正保持：

```text
Blockchain → produces deterministic reorg result
Service → consumes result
Mempool → independent subsystem
```

确认 blockchain package 是否仍然没有反向依赖 mempool。

重点审计：

- Disconnect
- Connect
- coinbase exclusion
- canonical deduplication
- dependency-aware resurrection
- capacity failure
- validation failure
- idempotency

判断是否存在任何“测试通过但架构边界实际上被破坏”的情况。

---

# 5. Crash / Recovery Audit

这是本阶段最高优先级之一。

重新检查 REORG-1C / 1E / 1F 已定义的 crash semantics。

逐项回答：

### A

如果：

```text
disconnect completed
connect partially completed
process crashes
```

重启后能否得到确定状态？

### B

如果：

```text
block persisted
tip update not persisted
```

会发生什么？

### C

如果：

```text
tip persisted
UTXO journal incomplete
```

会发生什么？

### D

如果：

```text
mempool resurrection completed
```

之后 crash，重启是否会重复造成状态问题？

### E

如果：

```text
detached fork block
```

被重复送入系统，是否仍然幂等？

明确区分：

```text
consensus-safe
recoverable
idempotent
durable
```

不要把“测试通过”直接等同于“crash-safe”。

---

# 6. F-3 / F-4 / F-5 / F-6 / F-7 Audit

逐项重新定位：

- F-3
- F-4
- F-5
- F-6
- F-7

对于每项给出：

```text
Current status
Exact location
Severity
Consensus impact
Whether implementation exists
Whether tests exist
Whether additional audit is required
```

禁止自行修复。

---

# 7. BT-1 Audit

重新确认 BT-1：

- 当前定义
- 当前状态
- 是否阻塞 REORG production confidence
- 是否阻塞 Linux testnet
- 是否阻塞 Windows mining
- 是否必须在 REORG-1 完成前解决

必须给出明确：

```text
BLOCKING / NON-BLOCKING
```

---

# 8. MaxReorgDepth / Finality Audit

确认当前协议是否已经明确：

- 最大 reorg depth
- finality semantics
- deep reorg handling
- old fork retention
- storage growth
- resource exhaustion

如果尚未定义：

不得实现。

只登记为：

```text
CONSENSUS GAP
```

并判断它是否阻塞当前阶段。

---

# 9. Test Coverage Audit

重新执行现有测试，但严格只读。

至少覆盖：

- blockchain tests
- reorg tests
- mempool tests
- service tests
- persistence tests
- consensus tests
- integration tests

报告：

```text
PASS
FAIL
SKIP
FLAKY
NOT COVERED
```

特别检查是否存在：

- 只有 unit test
- 没有 integration test
- 没有 restart test
- 没有 crash recovery test
- 没有 multi-node test

---

# 10. Local HEAD vs Linux Deployment Gap

这是本阶段另一个最高优先级。

**不要假设 Linux 服务器已经运行当前 HEAD。**

只读确认：

```text
LOCAL HEAD
    ↓
latest production/test binary
    ↓
Linux deployed binary
    ↓
Linux running process
    ↓
actual chain height
    ↓
actual protocol behavior
```

如果服务器可以安全只读检查，则只读取：

- binary version/hash
- process command line
- git revision（如存在）
- binary timestamp
- node height
- peer count
- difficulty/bits
- chainwork（如可读取）
- RPC health
- blocks.dat metadata（只读）
- logs

禁止：

- deploy
- restart
- rebuild
- copy binary
- modify config
- modify datadir

最终明确回答：

```text
Linux server is:
[CURRENT HEAD]
[OLDER COMMIT]
[UNKNOWN]
```

如果是 UNKNOWN，不得假设是最新代码。

---

# 11. Windows Mining Readiness

本阶段不运行 Windows miner。

只审计当前状态是否已经具备进入 Windows mining test 的条件。

分别判断：

### Consensus

是否 READY？

### Difficulty

是否 READY？

### Chainwork

是否 READY？

### Reorg

是否 READY？

### P2P

是否 READY？

### Persistence

是否 READY？

### Mining

是否 READY？

### Observability

是否 READY？

形成：

```text
WINDOWS MINING READINESS MATRIX
```

最终给出：

```text
READY
READY WITH CONDITIONS
NOT READY
```

---

# 12. Real-World Test Readiness

明确区分三个阶段：

## Stage A — Local deterministic tests

状态：

`READY / NOT READY`

## Stage B — Linux multi-node test

状态：

`READY / NOT READY`

## Stage C — Windows miner + Linux node real mining

状态：

`READY / NOT READY`

禁止因为 Stage A PASS 就自动推导 B/C PASS。

---

# 13. Deployment Timing Decision

结合目前项目历史状态，特别注意：

**上一次 Linux 服务器真实测试之后，本地代码已经进行了大量 consensus / difficulty / reorg 更新。**

因此必须判断：

```text
是否现在就应该部署？
```

提供两个明确选项：

### OPTION A — Continue local consensus completion first

### OPTION B — Freeze current HEAD and deploy to Linux for controlled integration test

说明每个选项的：

- 风险
- 收益
- 测试价值
- 对后续 REORG / difficulty / mining 的影响

不得直接执行其中任何一个。

---

# 14. Required Final Verdict

最终只能选择：

```text
REORG-1G VERDICT:
PASS
PASS WITH BLOCKING GAPS
NOT READY
```

并明确列出：

## Blocking

真正阻塞后续真实节点/挖矿测试的问题。

## Non-blocking

可以后续处理的问题。

## Deferred

明确允许后移的问题。

## Recommended Next Phase

只推荐一个下一阶段。

禁止自动进入下一阶段。

---

# HARD RULE

本阶段最重要的不是证明“代码已经很好”。

而是回答：

> **当前 e04d678 HEAD 到底距离“真实 Linux 节点 + Windows 挖矿 + 多节点 reorg 实测”还有多少真实工程距离？**

尤其不要混淆：

```text
Local tests PASS
≠
Linux deployed
≠
Linux running current HEAD
≠
Multi-node consensus verified
≠
Windows mining verified
≠
Production ready
```

最终报告必须把这五层严格分开。

**STOP AFTER REPORT.**
