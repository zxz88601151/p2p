# PHASE REORG-1J-R4B-PRE-GATE

## POSITIVE END-TO-END REORG CONVERGENCE / DUAL-PROCESS COMPETITIVE FORK AUDIT

你现在进入 P2PChain 的 REORG-1J-R4B PRE-GATE。

这是一个 **STRICT READ-ONLY / CONSENSUS SAFETY / P2P END-TO-END / CANONICAL CONVERGENCE AUDIT**。

严禁在本阶段修改任何 production code。

---

## 1. 当前已知前置结论

REORG-1J-R4A 已 PASS。

已证明：

- legacy-internal L=2/3/8/64 均可让 v2 canonical block 占据 `h < L` 的 legacy 逻辑槽位；
- legacy physical bytes 全程 byte-identical；
- 18,247 个字节级 crash points 全部通过；
- 每个矩阵格 canonical transition 恰好一次；
- transition 恰好发生于 TIP 完整落盘；
- I9 production-style recovery convergence 全部通过；
- I10 225 次 deterministic restart signature 全部一致；
- I12 同高度双记录解析 deterministic；
- R4A 未修改 production code；
- G08/G09/R3 tripwire 均保持不变。

当前已知 P0 GAP：

GAP-1H-A：

> storage layer 已解除 legacy-region reorg rejection，但尚未证明真实双进程 / P2P competitive fork 场景下，两个节点能够通过 production path 最终收敛到同一个 canonical tip。

---

# 2. 本阶段唯一核心问题

回答：

> 当两个真实 node 进程从共同 legacy prefix 出发，各自产生/接收不同分支，并通过真实 P2P 路径竞争更高-work chain 时，REORG 是否能够完整经过 production consensus/network/storage path，最终让双方 deterministic convergence 到同一个 canonical chain？

---

# 3. 严格范围

本阶段只读。

禁止：

- 修改 internal/storage/*.go production files
- 修改 internal/blockchain/*
- 修改 internal/blocktree/*
- 修改 internal/p2p/*
- 修改 consensus rules
- 修改 difficulty/bits
- 修改 block validity rules
- 修改 G08/G09/R3/R4A
- 新增 production crash hook
- 新增 test-only production branch
- 修改 legacy write semantics
- 修改 UNDO binding
- 修改 genesis semantics
- 修复 BT-1
- commit
- push
- tag
- merge
- rebase
- amend
- squash
- server deployment
- Windows mining deployment

如发现生产代码必须修改才能完成证明：

立即 STOP，并报告：

`PRODUCTION CHANGE REQUIRED`

不得自行修改。

---

# 4. PRE-GATE 必须确认的真实路径

首先只读定位并证明：

```text
P2P receive
    ↓
block validation
    ↓
blocktree insertion
    ↓
fork detection
    ↓
ShouldReorg / equivalent decision
    ↓
DisconnectBlock / undo
    ↓
SaveBlockDetached
    ↓
CommitReorg
    ↓
persistent TIP
    ↓
restart recovery
```

必须给出实际 production call path，而不是仅凭测试名称推测。

---

# 5. 双节点竞争分叉模型

设计最小真实双进程模型。

Node A 与 Node B 必须：

- 使用独立 datadir；
- 使用独立监听端口；
- 使用真实 P2P transport；
- 不共享 blocks.dat；
- 不共享 blocktree；
- 不共享 UTXO state；
- 不使用 mock P2P；
- 不直接调用内部 CommitReorg 代替真实网络路径。

共同起点：

```text
Genesis / legacy prefix
        ↓
common canonical chain
```

随后产生：

```text
common prefix
                  |
          +-------+-------+
          |               |
        Fork A          Fork B
          |               |
        Node A           Node B
```

确保两个分支初始存在真实竞争关系。

---

# 6. Reorg 必须真实发生

不得只验证“两个节点最终都有同一条链”。

必须明确证明：

```text
old canonical tip
        ↓
competing fork observed
        ↓
higher-work chain selected
        ↓
reorg decision
        ↓
old canonical detached
        ↓
new branch attached
        ↓
CommitReorg
        ↓
new canonical tip persisted
```

要求记录：

- old tip hash
- competing tip hash
- fork height
- old chain work
- new chain work
- selected canonical tip
- detached heights
- attached heights
- final persisted TIP
- restart result

---

# 7. Legacy-internal 至少覆盖

至少验证：

```text
L = 2
L = 8
L = 64
```

其中 fork 必须落在：

```text
f = L - 2
```

或等价地确保：

```text
new branch contains h < L
```

必须明确证明：

```text
v2 canonical block occupies legacy logical height
```

而：

```text
legacy physical record remains unchanged
```

---

# 8. 必须证明双节点最终一致

最终要求：

```text
Node A TIP == Node B TIP
```

并进一步验证：

```text
canonical path A == canonical path B
```

逐高度比较：

```text
height
block hash
previous hash
work
```

不能只比较 tip hash。

---

# 9. Physical legacy immutability

在 reorg 前保存：

```text
legacy prefix bytes
legacy record hashes
legacy record count
legacy sequence
```

reorg 后以及 restart 后再次比较。

要求：

```text
legacy bytes == original bytes
legacy record count == original count
legacy sequence hashes == original
```

任何 legacy physical byte mutation：

立即 STOP。

---

# 10. Restart convergence

在 reorg 完成后：

1. 停止 Node A；
2. 停止 Node B；
3. 分别重新启动；
4. 分别重新加载；
5. 获取 canonical signature；
6. 比较两个节点。

必须满足：

```text
restart(Node A).signature
==
restart(Node B).signature
```

并且：

```text
signature after restart
==
signature before shutdown
```

---

# 11. Crash 不在本阶段重新穷举

R4A 已经完成：

18,247 byte-level crash points。

本阶段不要重新复制 R4A 的 byte sweep。

只需要验证：

```text
normal production reorg
+
persistent commit
+
restart
```

如果必须进行 crash test，只允许设计最小的关键提交边界验证，不得扩大成新的 crash matrix，除非 PRE-GATE 明确发现 R4A 覆盖不到的新生产路径。

---

# 12. BT-1 明确排除

不要修复：

```text
BT-1
```

如果普通 blocktree test 仍出现既有：

```text
map iteration order
parent missing
```

类型问题：

单独登记为 existing limitation。

必须证明：

```text
production rebuildTree path
```

与该测试问题是否相关。

不得为了让测试全绿而修改 production code。

---

# 13. 必须建立 STOP 条件

任何以下情况立即 STOP：

1. production reorg path 不存在；
2. P2P 无法触发竞争分叉；
3. higher-work fork 无法被 canonical selection 采用；
4. legacy physical bytes 被改写；
5. canonical chain 出现两个合法 TIP；
6. restart 后 canonical tip 不确定；
7. Node A / Node B 最终不一致；
8. 需要修改 consensus/storage/network production code；
9. 需要修改 G08/G09/R3/R4A；
10. 发现 R4A 的核心结论与真实 production path 不一致。

---

# 14. 输出报告必须回答

最终报告必须逐条回答：

1. 是否存在真实双进程 competitive fork？
2. 是否通过真实 P2P 传播？
3. 是否真实进入 canonical reorg decision？
4. 是否真实执行 detach/attach？
5. 是否真实执行 CommitReorg？
6. fork 是否落入 legacy logical region？
7. v2 block 是否真正占据 `h < L`？
8. legacy physical bytes 是否完全不变？
9. Node A / Node B 是否最终 canonical convergence？
10. restart 后是否保持一致？
11. 是否发现新的 P0/P1 consensus/storage gap？
12. 是否需要修改 production code？
13. BT-1 是否仍然独立？
14. GAP-1H-A 是否可以 CLOSED？
15. 下一阶段最小必要动作是什么？

---

# 15. 最终输出格式

必须输出：

```text
VERDICT = PASS
```

或：

```text
VERDICT = NOT READY
```

或：

```text
VERDICT = STOP
```

并明确：

```text
GAP-1H-A = CLOSED
```

或：

```text
GAP-1H-A = OPEN
```

不得因为测试数量多而自动判定 PASS。

PASS 的唯一标准是：

> 真实双节点竞争分叉经过 production P2P → consensus → blocktree → storage reorg path 后，双方最终 deterministic convergence 到同一个 canonical chain，并证明 legacy physical prefix 完全不可变。

本阶段结束后：

STOP。

不要自动进入 implementation、commit、push、deployment 或 mining test。



