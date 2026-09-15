# PHASE REORG-1H — P2P BRANCH SYNCHRONIZATION / FORK DELIVERY IMPLEMENTATION

你现在进入 **P2PChain REORG-1H**。

## 阶段性质

STRICT CONTROLLED IMPLEMENTATION / P2P CONSENSUS DELIVERY / REORG NETWORK ENABLEMENT

本阶段唯一目标：

> 让真实 P2P 网络能够可靠地把“非当前 canonical tip 的合法 fork block”送达节点，并最终进入 BlockTree / reorg pipeline。

完成后，跨节点 reorg 才具有真实发生的物理条件。

---

# 1. 前置事实

当前本地 HEAD：

`e04d678b4fb33a14ea186999375f6d6dac2a4c4b`

REORG-1A～1G 已完成。

REORG-1G 已确认：

- ChainTree / cumulative work / deterministic fork choice PASS
- common ancestor PASS
- reorg execution PASS
- crash / recovery semantics PASS
- TIP atomic commit model PASS
- mempool boundary PASS
- detached fork idempotency PASS

当前明确阻塞：

> `cmd/node/service.go:154-159` 对 `PrevBlockHash != current tip` 的 block 直接拒绝/丢弃。

因此：

> 当前 P2P 网络无法传播并恢复任意 fork branch，真实跨节点 reorg 不可能发生。

---

# 2. 本阶段严格范围

本阶段只允许解决：

## M1 — Fork Block Detection

当收到：

```text
PrevBlockHash != current canonical tip
```

的合法 block 时：

不得简单视为 invalid。

必须区分：

1. 已知 canonical block
2. 已知 detached/fork block
3. 已知 parent 但非 canonical
4. 未知 parent
5. 真正 invalid block

不得改变现有 consensus validation semantics。

---

# 3. M2 — By-Hash Parent / Branch Retrieval

设计并实现最小必要的：

```text
block hash → request block
```

能力。

要求：

- 不引入 mining pool
- 不引入 Stratum
- 不引入账户系统
- 不引入经济层
- 不改变 fork-choice
- 不改变 cumulative-work 算法
- 不改变 SetTip semantics
- 不改变 persistence commit model

目标只是：

> “我发现一个合法 fork block，但缺 parent/ancestor → 能够请求并取得该 block。”

---

# 4. M3 — Branch Synchronization

必须支持：

```text
received fork block
        ↓
identify missing ancestor
        ↓
request missing block(s)
        ↓
receive ancestor(s)
        ↓
insert into BlockTree
        ↓
validate branch
        ↓
re-evaluate cumulative work
        ↓
ShouldReorg
        ↓
executeReorg
        ↓
SetTip
```

要求：

- parent-first insertion
- 不依赖 map iteration order
- 不允许循环请求
- 不允许无限 ancestor fetch
- 重复 block 请求必须幂等
- 已存在 block 不得重复持久化
- canonical block 不得被错误重写
- detached block 不得污染 canonical height index

---

# 5. M4 — Work-Aware Synchronization

不要简单实现：

> “收到 fork block 就立即 reorg。”

必须继续由现有：

```text
CumulativeWork
ShouldReorg
deterministic tie-break
```

决定是否切换 canonical tip。

P2P 层只能负责：

> delivery / synchronization

不能负责：

> consensus fork-choice

这是强制 architecture boundary。

---

# 6. M5 — Orphan Boundary

本阶段必须明确与 B5 Orphan Pool 的接口。

如果收到：

```text
block B
parent P unknown
```

则不得：

- 永久丢弃
- 无限递归请求
- 无限缓存
- 直接修改 canonical chain

允许：

```text
temporary orphan / pending branch state
        ↓
request parent
        ↓
parent arrives
        ↓
retry validation
```

但是：

> 完整 Orphan Pool policy / bounded retention / eviction / resource policy 属于 B5，不要在本阶段提前扩大范围。

如果当前实现必须提供最小 pending mechanism 才能完成 1H，请明确登记为：

`1H-BRIDGE`

不要偷偷完成整个 B5。

---

# 7. M6 — Network Safety

必须审计：

- request loop
- duplicate requests
- duplicate blocks
- cyclic ancestry
- malicious unknown-parent spam
- excessive ancestor depth
- peer disconnect
- timeout
- malformed response
- block hash mismatch
- parent hash mismatch
- response for wrong request
- concurrent branch delivery

不得引入：

- panic
- uncontrolled recursion
- unbounded memory growth
- unbounded network request loop

---

# 8. M7 — Persistence Boundary

必须严格保持：

```text
P2P receive
    ↓
validate
    ↓
BlockTree insertion
    ↓
reorg decision
    ↓
CommitReorg only when canonical tip changes
```

不得因为收到 detached fork block 就错误地：

```text
change TIP
rewrite canonical blocks
persist UTXO
```

Detached block 可以持久化，但：

> persistence ≠ canonicality

TIP 仍然是 canonical commit authority。

---

# 9. M8 — Test Requirements

至少新增/验证以下测试：

### P2P fork delivery

- fork block received
- fork block accepted into BlockTree
- fork block does not immediately become canonical

### Parent retrieval

- unknown parent
- request parent
- parent arrives
- child becomes processable

### Multi-block branch

例如：

```text
A
├── B
│   └── C
└── X
    └── Y
        └── Z
```

验证：

```text
X → Y → Z
```

可以通过网络恢复完整 branch。

### Work comparison

验证：

```text
lower-work fork → no reorg

higher-work fork → reorg
```

### Tie-break

相同 cumulative work：

```text
tip hash deterministic winner
```

### Duplicate delivery

同一个 fork block 重复发送：

```text
no panic
no duplicate persistence
no duplicate tree node
no second reorg
```

### Restart

验证：

```text
receive fork
persist detached
restart
recover tree
fork remains usable
```

如果当前 persistence architecture 对 detached recovery 有限制，必须明确记录，而不是伪造 PASS。

---

# 10. Multi-Node Test

优先使用真实独立进程。

至少：

```text
Node A
Node B
```

不要只依赖：

```text
two nodeService objects in one process
```

测试目标：

1. 两节点建立 TCP P2P
2. 两节点共同拥有 canonical chain
3. A/B 各自产生不同 fork
4. 其中一条 branch 获得更高 cumulative work
5. fork blocks 能跨节点传播
6. 接收端恢复缺失 ancestor
7. 接收端进入 BlockTree
8. ShouldReorg=true
9. executeReorg 执行
10. SetTip 完成
11. 两节点最终 canonical tip 收敛

必须证明：

```text
Node A tip == Node B tip
```

并证明：

```text
canonical chain == expected winning branch
```

---

# 11. 不允许扩大范围

本阶段禁止主动实施：

- MaxReorgDepth
- finality
- deep-reorg policy
- pruning
- physical compaction
- mining fairness
- mining pool
- Stratum
- reward model
- Windows mining
- production deployment
- production datadir migration
- Linux production restart
- firewall modification
- server deployment
- REORG-1I
- REORG-1J 完整实现
- B5 完整实现

如果发现这些问题：

> 只记录 GAP，不修复。

---

# 12. BT-1 处理要求

当前已知：

`TestI_RestartLikeReconstruction`

存在 map iteration 导致的 flaky。

本阶段不要为了让报告“全绿”而偷偷修改无关测试。

如果测试运行：

```text
go test ./...
```

再次触发 BT-1：

必须：

1. 明确记录
2. 单独运行该测试确认
3. 证明 production rebuildTree 不依赖该随机顺序
4. 不把 flaky test PASS 伪装成全绿

BT-1 remediation 仍属于独立阶段。

---

# 13. Acceptance Criteria

只有同时满足以下条件才能判定：

`REORG-1H PASS`

### A

非-tip fork block 不再被简单丢弃。

### B

未知 parent 可以触发受控 parent retrieval。

### C

多块 fork branch 可以完整恢复。

### D

BlockTree 正确保存 detached branch。

### E

fork-choice 仍完全由 cumulative work + deterministic tie-break 决定。

### F

higher-work branch 可以触发真实 reorg。

### G

lower-work branch 不触发 reorg。

### H

重复 fork block 幂等。

### I

canonical chain 不被 detached block 污染。

### J

P2P 请求不存在无限循环 / 无限递归。

### K

至少一个真实双进程 multi-node test PASS。

### L

reorg 后两个节点最终 canonical tip 收敛。

### M

现有 reorg / consensus / persistence / service tests 不被破坏。

---

# 14. Required Final Report

最终必须输出：

```text
PHASE REORG-1H — FINAL REPORT

VERDICT:
PASS / PASS WITH LIMITATIONS / NOT READY

HEAD:
<hash>

FILES CHANGED:
<exact list>

COMMITS:
<if authorized>

M1:
PASS/FAIL

M2:
PASS/FAIL

M3:
PASS/FAIL

M4:
PASS/FAIL

M5:
PASS/FAIL

M6:
PASS/FAIL

M7:
PASS/FAIL

M8:
PASS/FAIL

MULTI-NODE:
PASS/FAIL

FORK DELIVERY:
PASS/FAIL

BRANCH RECOVERY:
PASS/FAIL

REORG CONVERGENCE:
PASS/FAIL

CRASH/RESTART:
PASS/FAIL/NOT COVERED

BT-1:
PASS/FLAKY/FAIL

NEW LIMITATIONS:
<exact list>

DEFERRED:
<exact list>

PRODUCTION DEPLOYMENT:
NOT AUTHORIZED

WINDOWS MINING:
NOT AUTHORIZED

NEXT PHASE:
<single recommended phase>
```

## 最重要原则

本阶段不是“让节点更聪明”，而是建立：

> **真实网络 fork delivery → branch reconstruction → consensus fork-choice → reorg convergence**

这一条完整链路。

不要因为本地测试已经通过，就提前宣布真实多节点 reorg 成功。

最终必须区分：

```text
LOCAL REORG PASS
≠
P2P FORK DELIVERY PASS
≠
MULTI-NODE REORG PASS
≠
LINUX TESTNET PASS
≠
WINDOWS MINING PASS
≠
PRODUCTION READY
```

严格执行阶段边界。

**STOP AFTER REPORT。**












