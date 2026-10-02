# PHASE P2PCHAIN — ORPHAN-DURABILITY-IMPLEMENTATION-1 / §4-B2 READINESS AUDIT

**Owner authorization:** `ORPHAN-DURABILITY-IMPLEMENTATION-1 / §4-B2 READINESS AUDIT`
**Scope:** Design & implementation-readiness only. **No code changes. No commits.**
**Goal:** Prepare the startup recovery wiring plan built on the completed §4-B1 `restorePending` layer.
**STOP:** Readiness report only. Owner authorization required before §4-B2 implementation.

---

## §0 HARD BASELINE

### 0.1 Git state (confirmed)
- Branch `main`, up to date with `gitea/main`. Last commit `52fb464`.
- **Working-tree modifications (uncommitted):**
  - `cmd/node/service.go` (+50 行) — §4-B1 数据层接线（见 0.2）。
  - `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` (+92 行) — checkpoint 序列化格式规约。
  - `internal/blockchain/query.go` — **无语义改动**，仅为 autocrlf 行尾噪声（`git diff --ignore-all-space` 为空）。**非 drift。**
- **Untracked（§4-B1 新增）：** `cmd/node/orphan_checkpoint.go`、`orphan_checkpoint_test.go`、`orphan_checkpoint_hook_test.go`、`orphan_restore_test.go`，以及大量 `PHASE-*` 报告。
- 结论：§4-B1 全部为**未提交**状态，符合 in-progress 阶段特征。

### 0.2 §4-B1 artifacts confirmed (present & self-consistent)
| 工件 | 位置 | 状态 |
|---|---|---|
| 检查点数据/持久层（load/serialize/parse/markDirty/remove/flush/cleanupTmp） | `cmd/node/orphan_checkpoint.go` | ✅ 完整 |
| `orphanCP` 字段 + 运行时挂钩 `deferOrphan`→`MarkDirty`、`takeWaiting`→`Remove` | `cmd/node/service.go:198,728,1024` | ✅ 已实现（但 prod 中为 no-op，见 0.4） |
| `restorePending` 字段 + `prepareOrphanRestore()` | `cmd/node/service.go:202,1044` | ✅ 已实现（仅填充，不消费） |
| 单元测试（数据层 / 挂钩 no-op / 恢复去重·已知父跳过·损坏 fail-closed·不重建 waiting） | `cmd/node/orphan_*_test.go` | ✅ 覆盖 |
| 序列化格式规约（magic/version/entryCount/checksum/边界） | `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` | ✅ 与代码一致 |

自检：`prepareOrphanRestore` 与代码内注释引用的 SPEC 行为（只填父键集、跳过已知父、fail-closed、不重建 waiting）逐条对齐。

### 0.3 No drift verification
- `service.go` 的 +50 行**仅**为 §4-B1 意图（字段 + 两个挂钩 + `prepareOrphanRestore`），无越界改动、无既有逻辑回退。
- `query.go` 无内容差异 → 无语义 drift。
- ⚠️ **文档漂移风险（非阻断）：** 代码引用的两份权威 spec —— `ORPHAN-DURABILITY-IMPLEMENTATION-SPEC-v1`（`orphan_checkpoint.go:1`）与 `STARTUP-RECOVERY-PLAN-1 §2`（`prepareOrphanRestore` 注释）—— **在仓库中不存在独立文件**。当前以代码注释为事实 spec。建议 §4-B2 落地时随附这两份 spec 文档，避免后续回归无基准。

### 0.4 ⛔ READINESS BLOCKERS（§4-B1 当前在 prod 中为 inert）
1. **`orphanCP` 仅在测试中赋值**（`svc.orphanCP = cp` 全仓只出现在 `*_test.go`）。生产 `newNodeService` 从未赋值 → `svc.orphanCP == nil` → `deferOrphan`/`takeWaiting` 内的 `if s.orphanCP != nil` 恒为假 → **§4-B1 持久层在生产中完全不生效**。
2. **无任何生产 `Flush()` 调用方**（全仓 `grep "orphanCP.Flush|\.Flush()"` 仅命中 `*_test.go`；`shouldFlushLocked` 已定义但无调用者）。即使 `orphanCP` 被赋值，`orphan_waiting.bin` 也**永远不会落盘** → 每次重启 `prepareOrphanRestore` 读不到文件 → `restorePending` 恒空 → 恢复功能 inert。

> 这两点是 §4-B2 能达成其目标的**前置依赖**：§4-B2 不仅是"消费 restorePending"，还包含"让 §4-B1 真正在生产中激活并持久化"。

---

## §1 STARTUP PATH DISCOVERY

追踪 `main.go → nodeService 创建 → checkpoint 初始化 → OnHandshake` 生命周期（行号基于当前工作树）：

```
main()                                        cmd/node/main.go:885
 └─ startNode → newNodeRuntime(cfg)           main.go:364 / 74
     ├─ storage.AcquireDirLock                 main.go:76        (独占数据目录)
     ├─ OpenFileBlockStoreStrict               main.go:105
     ├─ NewBlockchainFromStore                 main.go:111       (chain 就绪)
     ├─ wallet.LoadOrCreate                    main.go:127
     ├─ pool := mempool.New                    main.go:138
     ├─ svc := newNodeService(chain,pool,w)    main.go:139  ──★ 结点创建（§4-B1 字段在此分配，但 orphanCP 仍 nil）
     ├─ p2p.NewNode(...) ; svc.net = ...       main.go:151
     ├─ p2pNode.Start()                        main.go:173       (监听绑定；此后才可能收发)
     ├─ ctl.NewServer(svc) ; ctl.Start         main.go:180
     ├─ svc.startSyncScheduler()               main.go:211
     ├─ rt.connectSeeds()                      main.go:212  ──★ 触发 ConnectToPeer → 握手
     └─ rt.watchSeeds()                        main.go:213       (每 5s 重连 → 重复握手)

P2P 收包 → dispatchInner(MsgHandshake)         internal/p2p/node.go:788
     ├─ O1 创世一致性守卫（异网拒绝，不进业务层）node.go:801
     ├─ learnPeer(...)                          node.go:808
     └─ n.handler.OnHandshake(peerAddr,p)      node.go:812  ──★ 每个对端握手各触发一次（异步、可多次）

OnHandshake                                  cmd/node/service.go:316
     ├─ shouldSyncFrom → requestSync          service.go:323
     └─ 分支发现 → requestBranch(tip)          service.go:330
        （★ §4-B2 在此追加：消费 restorePending）
```

**关键时序约束：**
- `restorePending` 必须在**首个 `OnHandshake` 之前**由 `prepareOrphanRestore` 填充完毕。
- 当前 `newNodeRuntime` 中 `svc` 在 `p2pNode.Start()` 与 `connectSeeds()` **之前**已创建且 chain 已就绪 → 插入 `orphanCP` 赋值 + `prepareOrphanRestore(cp)` 的安全点为 `main.go:139~151` 之间（或 `p2pNode.Start()` 之前任意处）。
- `OnHandshake` 是每个对端独立的、可重复触发的异步回调 → §4-B2 消费逻辑必须**幂等**、可跨多次握手推进。

---

## §2 RESTORE CONSUMPTION DESIGN

### 2.1 restorePending 生命周期
- **填充（启动，单次，单线程）：** `prepareOrphanRestore(cp)` 从 `orphan_waiting.bin` 加载 `父哈希→子哈希` 引用，仅保留「本节点未知」的父哈希（`chain.HasBlockHash` 跳过已知父），构建 `restorePending` map（去重）。
- **消费（握手，并发，幂等）：** 每次 `OnHandshake` 末尾调用 `consumeRestorePending(peerAddr)`：
  - 对每个 `p ∈ restorePending`：
    - 若 `chain.HasBlockHash(p)`（已知，可能经正常 sync 到达）→ **drain**：`delete(restorePending,p)`；若 `orphanCP != nil` 则 `orphanCP.Remove(p)`（checkpoint 同步、幂等）。
    - 否则 → `requestBranch(peerAddr, p)` 重新拉取缺失分支；**保留**在 `restorePending` 中，等待下一轮握手/对端再尝试。
- **终态：** 父变为已知即从集合移除；永不已知则留在集合、随每次握手重发请求（受 `requestBranch` 的 inflight/TTL 去重限流）。

### 2.2 成功 / 失败移除语义
- **成功（父已知）：** 立即从 `restorePending` 与 checkpoint 双删。
- **失败（`requestBranch` → 对端 `Found=false` → 分支丢弃）：** 父**保留**在 `restorePending`，不在此处判定终态；下一握手（不同对端 / 同一对端 TTL 后）重新请求。恢复不依赖单次请求成功。

### 2.3 重试语义
- 复用既有 by-hash 拉取机：`requestBranch` 自带 `maxInflightBranch` 上限、`branchReqTTL` 窗口去重、`maxBranchRounds` 有界回溯。
- 不在 §4-B2 引入新重试状态机。重试是"**留在集合直到已知 + 每次握手重发**"的隐式重试，天然有界（inflight/TTL/rounds）。
- 限流：`restorePending` 非空时每次握手有效请求频率 ≤ 每 `branchReqTTL`(30s) 每父一次；`watchSeeds` 每 5s 重连仅放大到"每个种子握手各一次"，仍被 TTL 去重压制。

### 2.4 重复保护
- `restorePending` 为 map → 父键天然去重，消费不重复。
- `requestBranch` 按哈希 `branchReqTTL` 去重 → 无重复在途请求。
- 拉回的区块走既有 `ingestBlock → addBlockAndUpdatePool` 全量共识校验，叠加 `IsCanonicalHash`/`KnowsParent`/detached 存储去重 → 无重复上链。
- checkpoint 层 `markDirtyLocked` 对相同 `childHash` 去重；`Remove` 幂等。

### 2.5 与既有 orphan 队列（s.waiting / parkedHashes）的交互
- **不重建 `s.waiting`、不伪造块指针**（§4-B1 不变式）。恢复只重新请求"缺失父的分支"，而非把子块塞回内存队列。
- 分支到达后，若子块仍被网络重传，会经**既有** `deferOrphan → s.waiting` 路径正常再入队并再经 `MarkDirty` 持久化 → 无双计、无冲突。
- `restorePending` 与 `s.waiting` 是**两个独立 map**，唯一共享可变资源是 checkpoint 文件（通过 `orphanCP`），其 `Remove`/`MarkDirty` 均幂等/去重，互不腐蚀。
- 仅当父确为已知（`HasBlockHash`）时才 `orphanCP.Remove(p)`，故不会误删仍存活的 `waiting` 条目。

---

## §3 HANDSHAKE SAFETY ANALYSIS

| 不变量 | 证明 | 结论 |
|---|---|---|
| **无共识变更** | 恢复路径仅调用 `requestBranch`（网络 I/O）与读 `HasBlockHash`；**绝不**调用 `AddBlock`/`AddBlockWithResult`/`validateBlock`。拉回区块走既有 `ingestBlock` 全量共识流水线。 | ✅ |
| **无重复区块插入** | 拉回块经 `IsCanonicalHash`/`KnowsParent`/detached 存储去重；`requestBranch` inflight 去重防重复请求。 | ✅ |
| **无 waiting 腐蚀** | `restorePending` 独立于 `s.waiting`；`orphanCP.Remove(p)` 仅在 `HasBlockHash(p)` 为真时执行（该父不可能还挂有 live waiting）；`MarkDirty` 去重。 | ✅ |
| **无 reorg 回归** | 恢复只补缺分支、交正常 fork-choice 裁决；不强制 reorg、不改 best-tip 选择；reorg 代码路径零改动。 | ✅ |

**Fail-closed 总证：** checkpoint 损坏/缺失/`orphanCP==nil` → `prepareOrphanRestore` 置空 `restorePending` → 节点行为**完全等同今日**（纯内存态），不 panic、不反向影响 canonical。任何 `orphanCP` 调用均在 `s.mu` 外、独立自锁，不引入锁序风险。

---

## §4 IMPLEMENTATION PLAN（仅指定，不实现）

### 4.0 前置依赖（必做，否则 §4-B2 为 inert）
> 来自 §0.4 的两个 blocker。若不补，恢复功能永远空转。

**(D1) 激活 `orphanCP`（生产赋值）** — `cmd/node/main.go` `newNodeRuntime`：
- 在 `svc` 创建后、`p2pNode.Start()` 之前：
  - `cp := newOrphanCheckpoint(cfg.DataDir)`
  - `cp.CleanupTmp()`（清除崩溃遗留 `.tmp`）
  - `svc.orphanCP = cp`
  - `svc.prepareOrphanRestore(cp)`（**首个握手前**填充 `restorePending`）

**(D2) Flush 驱动（持久化落盘）** — `cmd/node/main.go` + 复用 `syncSchedulerLoop`：
- 关机落盘：`nodeRuntime.Close()` 中、`store.Close()` 之前加 best-effort `if svc.orphanCP != nil { _ = svc.orphanCP.Flush() }`（nil-safe）。
- 周期落盘（推荐，降低崩溃丢失）：在 `runSyncSweep`（`service.go:1300`）末尾，若 `orphanCP != nil` 且 `shouldFlushLocked` 满足，则 `orphanCP.Flush()`。复用既有 2s 巡检 goroutine，无新 goroutine。

### 4.1 文件与函数改动清单
| 文件 | 函数 | 行为变更 |
|---|---|---|
| `cmd/node/main.go` | `newNodeRuntime` | 插入 D1（赋值 `orphanCP` + `CleanupTmp` + `prepareOrphanRestore`）。 |
| `cmd/node/main.go` | `nodeRuntime.Close` | 插入 D2 关机 `Flush()`（nil-safe）。 |
| `cmd/node/service.go` | `OnHandshake` | 末尾追加 `s.consumeRestorePending(peerAddr)`。 |
| `cmd/node/service.go` | 新增 `consumeRestorePending(peerAddr)` | 见 4.2。 |
| `cmd/node/service.go` | `runSyncSweep` | 末尾追加 D2 周期 flush（可选但推荐）。 |
| `cmd/node/service.go` | `orphan_checkpoint.go`（无改动） | 复用 `Flush`/`Remove`/`MarkDirty`。 |

### 4.2 新增 `consumeRestorePending(peerAddr)` 精确行为（伪代码级）
```
consumeRestorePending(peerAddr):
  s.mu.Lock()
  if len(s.restorePending) == 0 { s.mu.Unlock(); return }      // 快速路径
  pending := snapshot keys(s.restorePending)                    // 复制出，释放锁
  s.mu.Unlock()
  for p in pending:
    if s.chain.HasBlockHash(p):                                 // 已知（含正常 sync 到达）
      s.mu.Lock(); delete(s.restorePending, p); s.mu.Unlock()
      if s.orphanCP != nil { s.orphanCP.Remove(p) }            // checkpoint 同步、幂等
      obs.Emit("ORPHAN_RESTORE_DRAINED", parent=p)
      continue
    s.requestBranch(peerAddr, p)                               // 重拉缺失分支；保留至已知
  obs.Emit("ORPHAN_RESTORE_TRIGGERED", count=len(pending), peer=peerAddr)
```
- **锁纪律：** `restorePending` 读写全程 `s.mu` 保护（启动填充为单线程、握手消费并发）；`chain.HasBlockHash` 与 `requestBranch` 在 `s.mu` **外**调用（与既有 `deferOrphan` 一致）；`orphanCP` 内部自锁，不嵌套 `s.mu`。
- **效率可选：** `requestBranch` 前可再判一次 `HasBlockHash` 省一次已知块的请求；非正确性必需（既有去重已兜底）。

### 4.3 不变更清单（明确边界）
- 共识规则 / `blockchain.*` / `AddBlock` / reorg / mempool / UTXO：零改动。
- `s.waiting`/`parkedHashes` 数据结构与 `deferOrphan`/`takeWaiting` 既有逻辑：零改动。
- P2P 协议、`HandshakePayload` 字段、RPC schema：零改动。
- `orphan_checkpoint.go` 数据层：零改动（仅消费方调用既有 API）。

### 4.4 建议回归测试（§4-B2 实现后）
- `TestRestore_ConsumeOnHandshake`：预置 checkpoint + 协作对端，`OnHandshake` 触发 `requestBranch` 且父到达后从 `restorePending` 移除。
- `TestRestore_FailClosed`：损坏 checkpoint → 节点仍正常启动、`restorePending` 空。
- `TestRestore_DuplicateProtection`：同一对端两次握手 → 每父仅一次在途 `requestBranch`（inflight 去重）。
- `TestRestore_NoConsensusMutation`：spy 确认恢复路径从不调用 `AddBlock`。

---

## READINESS VERDICT

- **§4-B1 数据层：完成且自洽**（代码 + 单测 + 序列化规约一致）。
- **§4-B1 生产激活：未完成** → 两个 blocker（D1 赋值 `orphanCP`、D2 flush 驱动）必须在 §4-B2 内一并落地，否则恢复功能 inert。
- **§4-B2 设计：就绪**，安全分析四项不变量均可证明，复用既有 by-hash 原语、无新增共识/协议面。
- **文档漂移（非阻断）：** 两份被引用 spec 文档不在仓库，建议 §4-B2 随附。

**➡️ OWNER AUTHORIZATION REQUIRED before §4-B2 implementation.**
（建议授权范围：4.0 D1+D2 + 4.1/4.2 消费接线；4.3 边界与 4.4 测试一并纳入。）
