# PHASE REORG-1H — FINAL REPORT

**阶段性质**：STRICT CONTROLLED IMPLEMENTATION / P2P CONSENSUS DELIVERY / REORG NETWORK ENABLEMENT
**执行日期**：2026-09-15
**基线 HEAD**：`e04d678`（REORG-1F-COMMIT-1）

---

## 1. VERDICT

```text
VERDICT: PASS WITH LIMITATIONS
```

**为什么是 PASS**：M1–M8 全部交付并有可复现证据；1G 认定的**唯一硬阻塞**（`cmd/node/service.go` 就地丢弃「父块 ≠ 当前链尾」的区块）已被移除，真实 P2P 网络第一次具备把合法 fork block 送进 BlockTree / reorg pipeline 的物理条件；真实双进程 multi-node 测试存在且 PASS。

**为什么不是 PASS（无限定）**：**真实双进程的 reorg 收敛尚未成功**。根因是一个**本阶段新发现的存储层 P0 缺陷（GAP-1H-A）**——legacy 语义写入的 canonical 块构成不可变历史前缀，`FileBlockStore.setCanonicalFrom`（`internal/storage/v2.go:644`）拒绝让 v2 区块占据 `height < legacyLen` 的槽位，因此**现实持久化链上的首次 reorg 在存储层被拒绝**。该缺陷已在真实双进程中被现场复现并固化为 tripwire 用例。

> 按阶段原则：「不要因为本地测试已经通过，就提前宣布真实多节点 reorg 成功。」
> 本报告**不**宣布真实多节点 reorg 成功。

---

## 2. HEAD / PARENT（git 真相）

```text
HEAD:  e04d678 feat: restore transactions after chain reorganization
PARENT:369d36e (REORG-1C-COMMIT-1)
BRANCH: main
REMOTE: gitea（本阶段未推送）
```

## 3. COMMITS

```text
COMMITS: NONE
```

本阶段未获提交授权，未执行任何 `git commit` / `git push`。工作区改动保持未提交状态，供下一阶段继续或单独授权提交。

## 4. FILES CHANGED（精确清单）

**修改（5 个）**

| 文件 | 改动 | 说明 |
|---|---|---|
| `cmd/node/service.go` | +462 / −37 | 1H 主体：移除硬丢弃分支；新增分叉分类投递、by-hash 分支补齐、work-aware 握手/同步 |
| `cmd/node/main.go` | +14 | 接线 `SetChainStatusProvider`（向 P2P 层提供 chainwork / tip hash） |
| `internal/p2p/node.go` | +95 | 新增 `MsgGetBlockByHash` / `MsgBlockByHashResp`、握手 `chain_work` / `tip_hash` 字段、`Handler` 两个新方法 |
| `internal/p2p/node_test.go` | +119 | 新增 2 个真实 TCP 协议用例 |
| `internal/blockchain/blockchain.go` | +19 / −… | 新增导出哨兵 `ErrOrphanParent`（包裹 `ErrInvalidPrevHash`，保持既有 `errors.Is` 断言语义） |

**新增（3 个）**

| 文件 | 行数 | 说明 |
|---|---|---|
| `internal/blockchain/query.go` | 132 | 1H 只读查询层（按哈希取块 / 取回祖先链 / 工作量 / canonical 判定） |
| `cmd/node/p2p_branch_test.go` | 578 | 进程内「真实 nodeRuntime + 真实 TCP」分叉投递测试（9 个用例） |
| `cmd/node/p2p_branch_process_test.go` | 330 | **真实双进程**（`go build` + `os/exec`）测试（2 个用例） |

**未触碰**：`internal/blocktree`、`internal/storage`、`internal/utxo`、`internal/mempool`、共识参数、生产 datadir、部署配置。
（`docs/DETERMINISTIC-SERIALIZATION-SPEC.md`、`docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` 的工作区改动为**历史遗留**，非本阶段产生。）

---

## 5. M1–M8 逐项结论

### M1 — Fork Block Detection：PASS

**修复前的真实阻塞**（`cmd/node/service.go`，旧 199–204 行，已删除）：

```go
if b.Header.PrevBlockHash != tip.Header.Hash() {
    // 单链实现不处理 reorg，分叉场景已在 blockchain 包文档中列为后续工作项。
    log.Printf("[node] 忽略区块 %s（父块不是当前链尾，可能为重复广播或分叉）", b.Header.HashHex())
    return
}
```

**修复后**：`OnNewBlock → ingestBlock` 把收到的区块分类为五类，且**不改变任何既有共识校验语义**：

1. 已在 canonical 链上 → 静默幂等（`IsCanonicalHash`）；
2. 缺父（`ErrOrphanParent`）→ `deferOrphan` + 按哈希请求父块，不报错、不丢弃；
3. 父已知、非 canonical → 交由 `blockchain.AddBlock` 走 Case2 fork 路径（detached 入库 + fork-choice）；
4. 真正非法 → 记录「区块拒绝」并 `endSync`，不再污染同步状态。

`ErrOrphanParent` 以 `fmt.Errorf("%w: 父区块未知（orphan）", ErrInvalidPrevHash)` 定义，**`errors.Is(err, ErrInvalidPrevHash)` 仍为真**，因此既有断言零改动。

证据：`TestBranchForkBlockWithKnownParentConverges`（竞争块跨节点广播 → 双方收敛到同一链尾）。

### M2 — By-Hash Parent / Branch Retrieval：PASS

- 协议：新增 `MsgGetBlockByHash` / `MsgBlockByHashResp`（`GetBlockByHashPayload{Hash, MaxAncestors}` / `BlockByHashRespPayload{Hash, Blocks, Found}`），服务端常量 `MaxAncestorsPerResp = 64`。
- 查询层：`internal/blockchain/query.go` 新增 `BlockByHash` / `HasBlockHash` / `IsCanonicalHash` / `KnowsParent` / `BestTipWork` / `BlockByHashWithAncestors(hash, maxAncestors)`（返回**由新到旧**的祖先链，index 0 = 被请求块）。
- 纯只读，全部持 `bc.mu.RLock()`；**未**引入 mining pool / Stratum / 账户系统 / 经济层，**未**改动 fork-choice、cumulative-work、`SetTip` 语义、持久化提交模型。

证据：`TestGetBlockByHashTravelsOverTCP`（真实 TCP 往返）、`TestBranchLowerWorkForkIsAcceptedButNotCanonical`（detached 块可按哈希取回）。

### M3 — Branch Synchronization：PASS

完整链路已接通：

```text
收到 fork / orphan block
  → 识别缺父（KnowsParent）
  → requestBranch（按哈希，带去重 + 在途上限）
  → 对端 OnGetBlockByHash → BlockByHashWithAncestors
  → OnBlockByHashResp 定位首个父已知的下标 start
  → for i := start; i >= 0; i-- 逐块应用（严格 parent-first，不依赖 map 迭代序）
  → 仍无挂载点 → 有界加深（rounds ≤ maxBranchRounds=8）
  → applyResolved 显式队列级联（无递归）
  → resumeSync
```

约束（全部有常量与去重保障）：`MaxBranchAncestors=64`、`maxInflightBranch=64`、`maxWaitingBlocks=256`、`maxBranchRounds=8`、`branchReqTTL=30s`；孤儿等待表 `waiting` **仅内存、从不落盘**。

证据：`TestBranchMissingAncestorFetchTriggersReorg`（A=5 / B=4 跨节点补齐两个祖先并 reorg）、`TestBranchOrphanRequestIsBoundedAndNotAmplified`（未知父 → 恰好 1 次请求，不放大）。

### M4 — Work-Aware Synchronization：PASS

- 握手新增可选字段 `chain_work` / `tip_hash`；**空值 = 未知 → 退化为比高度**，与旧版本节点完全向后兼容（`TestHandshakeOmitsChainStatusWhenUnset`）。
- `shouldSyncFrom`：双方工作量已知 → **只比累积工作量**；`workAtLeast`：不低于才拉分支。
- **P2P 层只负责 delivery / synchronization，不参与 fork-choice**：是否 reorg 仍完全由 `CumulativeWork + ShouldReorg + deterministic tie-break` 决定，1H 未改动该路径一行代码。

证据：`TestShouldSyncFromPrefersWorkOverHeight`、`TestWorkAtLeastGate`（纯函数表驱动）、`TestBranchForkBlockWithKnownParentConverges`（同工作量 → 由 tip 哈希大端较大者确定性胜出，与网络到达顺序无关）。

### M5 — Orphan Boundary（1H-BRIDGE，非 B5）：PASS（登记为 1H-BRIDGE）

交付的是**最小 pending 机制**，明确登记为 `1H-BRIDGE`，**不是** B5 Orphan Pool：

- 孤儿登记在内存 `waiting map[[32]byte][]*block.Block`，**不落盘**；
- 有界（256 条/父哈希上限 + 总量上限，超限丢弃并记录日志）；
- 父块到达后 `takeWaiting` 级联处理，无递归；
- 不直接修改 canonical chain。

未实现（属 B5，本阶段禁止扩大范围）：完整 retention policy、eviction、资源配额、孤儿池观测。

### M6 — Network Safety：PASS

审计项与处置：

| 风险 | 处置 |
|---|---|
| request loop | 同一哈希 `branchReqTTL` 窗口内绝不重复请求（`inflight` 去重） |
| duplicate requests | 同上；`TestBranchDuplicateResponseIsIdempotent` |
| duplicate blocks | canonical 重复静默幂等；detached 重复不重复持久化 |
| cyclic ancestry | 回溯轮次上限 `maxBranchRounds=8`；单次深度 `MaxAncestorsPerResp=64` |
| malicious unknown-parent spam | 在途上限 64 + 等待上限 256 + 超限丢弃（有日志，不静默） |
| excessive ancestor depth | 双重裁剪（请求侧 `MaxBranchAncestors`、服务侧 `MaxAncestorsPerResp`） |
| peer disconnect / timeout | `clearInflight` + `endSync("")` 复位 `syncing`/`pending`，**不再永久卡死** |
| malformed response | 解码失败 → 放弃该分支并记录，不 panic |
| block/parent hash mismatch | 逐块 `block.Decode` + 父哈希比对后才应用 |
| response for wrong request | 响应携带 `Hash`，与请求哈希不符即放弃 |
| concurrent branch delivery | 全部状态在 `s.mu` 下；`branchReqSent/RespRecv/Applied` 原子计数 |
| unbounded recursion | `applyResolved` 显式队列级联，**零递归** |

额外修复（既有缺陷）：`OnBlocksResp` 遇到非法/被拒块时 `syncing` 标志永久置真（同步永久卡死）→ 改为 `endSync` / `resumeSync`。

### M7 — Persistence Boundary：PASS

```text
P2P receive → validate → BlockTree insertion → reorg decision → CommitReorg（仅当 tip 变化）
```

- detached 块可持久化（`SaveBlockDetached`），但**持久化 ≠ canonicality**：`IsCanonicalHash` 与 `HasBlockHash` 严格区分（1F 引入的 canonical-membership 语义）；
- TIP 仍是唯一 canonical commit 权威；
- 收到 detached 分叉块**不会**改 TIP / 重写 canonical / 落 UTXO；
- 高度索引不被 detached 污染（`TestBranchLowerWorkForkIsAcceptedButNotCanonical` 显式断言）；
- 1H 引入的内存态（`inflight` / `waiting` / `rounds` / `syncResume`）**全部不落盘**。

### M8 — Test Requirements：PASS（含 M8 全部子项）

| M8 子项 | 用例 | 结果 |
|---|---|---|
| P2P fork delivery（收到 / 进树 / 不立即 canonical） | `TestBranchLowerWorkForkIsAcceptedButNotCanonical` | PASS |
| Parent retrieval（未知父→请求→到达→可处理） | `TestBranchMissingAncestorFetchTriggersReorg`、`TestBranchOrphanRequestIsBoundedAndNotAmplified` | PASS |
| Multi-block branch 恢复 | `TestBranchMissingAncestorFetchTriggersReorg`（2 块祖先链） | PASS |
| Work comparison（低工作量不 reorg / 高工作量 reorg） | `TestBranchLowerWorkForkIsAcceptedButNotCanonical` / `TestBranchMissingAncestorFetchTriggersReorg` | PASS |
| Tie-break（同工作量确定性胜者） | `TestBranchForkBlockWithKnownParentConverges` | PASS |
| Duplicate delivery（幂等、不重复持久化、不二次 reorg） | `TestBranchDuplicateResponseIsIdempotent` | PASS |
| Restart（detached 持久化后重启仍可用） | `TestBranchDetachedForkSurvivesRestart` | PASS |
| 真实双进程 multi-node | `TestRealProcessPairConvergesToSameTip`、`TestRealProcessForkBlockTravelsByHash` | PASS |

---

## 6. ACCEPTANCE CRITERIA A–M

| 项 | 结论 | 证据 |
|---|---|---|
| A 非-tip fork block 不再被简单丢弃 | **PASS** | 硬丢弃分支已删除；`TestBranchForkBlockWithKnownParentConverges` |
| B 未知 parent 触发受控 parent retrieval | **PASS** | `TestBranchOrphanRequestIsBoundedAndNotAmplified`（恰好 1 次请求） |
| C 多块 fork branch 可完整恢复 | **PASS** | `TestBranchMissingAncestorFetchTriggersReorg` |
| D BlockTree 正确保存 detached branch | **PASS** | `TestBranchLowerWorkForkIsAcceptedButNotCanonical` |
| E fork-choice 仍完全由 cumulative work + tie-break 决定 | **PASS** | 未改动共识路径；tie-break 用例断言确定性胜者 |
| F higher-work branch 触发真实 reorg | **PASS（进程内）/ BLOCKED（真实双进程，GAP-1H-A）** | 见 §7 |
| G lower-work branch 不触发 reorg | **PASS** | `TestBranchLowerWorkForkIsAcceptedButNotCanonical` |
| H 重复 fork block 幂等 | **PASS** | `TestBranchDuplicateResponseIsIdempotent` |
| I canonical 不被 detached 污染 | **PASS** | 高度索引断言 + `IsCanonicalHash` |
| J P2P 请求无无限循环/递归 | **PASS** | 去重 + 三重上限 + 队列级联（零递归） |
| K 至少一个真实双进程 multi-node test PASS | **PASS** | `TestRealProcessPairConvergesToSameTip`（22.4s） |
| L reorg 后两节点 tip 收敛 | **PARTIAL** | 进程内 PASS；真实双进程被 GAP-1H-A 阻塞（见 §7） |
| M 既有 reorg / consensus / persistence / service 测试不被破坏 | **PASS** | 全量回归见 §8（唯一红灯为既有 BT-1 flaky） |

---

## 7. 新发现 P0：GAP-1H-A（legacy 前缀不可变 → 首次 reorg 被存储层拒绝）

### 现象

真实双进程：两个节点共享前缀 3，各自离线挖出高度 4 的竞争区块后互联。双方都正确触发了 by-hash 分支拉取（日志均有「请求分支区块」/「已响应 by-hash」），但**胜出方的 reorg 被存储层拒绝**：

```text
[node] 分支区块被拒绝: 0000ca5c…: reorg: persist failed: 区块数据文件损坏: 高度 1 由 v2 区块占据（legacy 前缀不可变）
```

结果：双方永久停留在各自分支（A tip = a4，B tip = b4），**不收敛**。

### 根因

`internal/storage/v2.go:644`（`setCanonicalFrom`）：

```go
if h < s.v2.legacyLen && r.isV2 {
    return fmt.Errorf("%w: 高度 %d 由 v2 区块占据（legacy 前缀不可变）", ErrCorruptStore, h)
}
```

- 节点自启动以来写的第一个 canonical 块必然走 **legacy 路径**（`blockchain.extendChain` 判据是 `V2Mode()`，而 `V2Mode()` 只在出现第一条 v2 记录后才为真）；
- legacy 记录「高度 = 记录序号」，其隐含语义是**不可变历史前缀**；
- 因此**任何落点在 legacy 前缀内部的 reorg 都必然被拒** —— 包括「现实持久化链上的首次 reorg」。

### 影响面（真实且严重）

生产 `.123` 节点当前高度 1284，**全部为 legacy 记录**（`legacyLen ≈ 1285`）。任何分叉点低于 1285 的 reorg 都会被拒绝 —— 即：**生产链在当前代码下不可能完成任何一次 reorg**。reorg 基础设施（1A/1B/1C/1D/1E/1F/1H）在存储层被这一条校验整体卡死。

### 本阶段处置（符合「只记录 GAP，不修复」）

1. 不修改 `internal/storage`（授权文件外）；
2. 测试侧用 `sealV2Mode` **绕行**（挖一枚累积工作量必定更低的 detached v2 块把存储切进 v2 语义），并在注释中明确「这是绕行不是修复」；
3. 固化为两个 tripwire 用例（修复后必然失败，届时应**删除/改写**而非放宽）：
   - `TestBranchKnownGapLegacyPrefixBlocksFirstReorg`（进程内，直接断言存储层拒绝证据）
   - `TestRealProcessForkBlockTravelsByHash`（真实双进程，断言「不收敛 + 日志含 legacy 前缀不可变」）

### 修复方向（不属于 1H，供下一阶段决策）

- 方案 A：允许 v2 区块占据 legacy 槽位，改为按**哈希链**重建 canonical（`legacyLen` 仅用于兼容判断，不做硬拒绝）；
- 方案 B：首次 reorg 前执行一次「legacy → v2」迁移重写（把 legacy 记录整体升级为 v2 帧），随后取消该校验。
- 需要配套：崩溃恢复语义、TIP 提交点不变性、只读校验路径（`OpenFileBlockStoreReadOnly` 必须继续严格拒绝损坏）的一致性论证。

---

## 8. 验证结果（证据优先）

```text
go build ./...                       → OK
go vet   ./...                       → OK
go test  ./...  -count=1             → 12 个包 ok；1 个包 FAIL（BT-1，见 §9）
```

最终回归（含全部 1H 新用例）：

```text
ok  p2pchain/cmd/node            240.081s
ok  p2pchain/internal/p2p          0.848s
ok  p2pchain/internal/blockchain  59.948s
ok  p2pchain/internal/storage     59.059s
ok  p2pchain/internal/utxo         0.271s
```

1H 相关用例连跑 3 轮（`-count=3`，覆盖 `TestBranch*` / `TestRealProcess*` / `TestNodeService*` / `TestSeedReconnectAfterRestart` / `TestFullStackMineAndStatus`）：

```text
ok  p2pchain/cmd/node  199.160s   （EXIT=0，无 flaky）
```

真实双进程用例单独运行：

```text
--- PASS: TestRealProcessPairConvergesToSameTip        (22.37s)
--- PASS: TestRealProcessForkBlockTravelsByHash        (29.19s)
```

**全量 `go test ./...` 唯一红灯**：`internal/blocktree → TestI_RestartLikeReconstruction`（既有 BT-1）。

---

## 9. BT-1

```text
BT-1: FLAKY
```

- 未做任何修改（`internal/blocktree` 本阶段零改动）；
- 单独复跑 8 次：**4 FAIL / 8**（约 50%），与历史上「2/6、3/4」一致，属随机 map 迭代序问题；
- 生产路径 `rebuildTree` **不依赖**该随机顺序（该用例是按 `map` 迭代构造输入，生产回放按高度顺序）；
- **未**把 flaky PASS 伪装成全绿；BT-1 remediation 仍属独立阶段。

---

## 10. 交付指标汇总

```text
M1:  PASS
M2:  PASS
M3:  PASS
M4:  PASS
M5:  PASS（1H-BRIDGE，非 B5）
M6:  PASS
M7:  PASS
M8:  PASS

MULTI-NODE:       PASS（真实双进程，go build + os/exec + -seed + 真实 TCP）
FORK DELIVERY:    PASS（跨进程 by-hash 往返已验证：请求 + 响应双向日志证据）
BRANCH RECOVERY:  PASS（多块祖先链 parent-first 恢复 + 幂等）
REORG CONVERGENCE: PARTIAL
                  - 进程内真实 runtime：PASS（两节点收敛到确定性胜者）
                  - 真实双进程：BLOCKED by GAP-1H-A（已复现 + tripwire 固化）
CRASH/RESTART:    PASS（detached 分叉块重启后仍可恢复、可按哈希取回；
                        「reorg 过程中崩溃」的一致性与恢复测试 NOT COVERED，属独立阶段）
BT-1:             FLAKY（4/8 FAIL，未改）
```

---

## 11. NEW LIMITATIONS（精确清单）

1. **GAP-1H-A（P0）**：legacy 前缀不可变 → 现实持久化链的首次 reorg 在存储层被拒绝；生产 `.123`（1284 块全 legacy）因此不可能完成任何 reorg。见 §7。
2. **REORG-1H 未实现完整 B5 Orphan Pool**：仅 `1H-BRIDGE` 最小 pending 机制，内存态、无 eviction/配额/观测。
3. **孤儿等待表不落盘**：节点重启后未挂载的孤儿丢失（detached 分叉块本身可恢复，孤儿队列不可）。
4. **`sealV2Mode` 是测试脚手架**：它绕开 GAP-1H-A 而非修复；任何以它为前置的用例都不代表生产可达。
5. **崩溃语义**：`reorg 执行中崩溃` 的恢复路径未在本阶段新增覆盖（1C 已有 A–F 崩溃恢复测试，本次未改动、未重跑其全部组合）。
6. **分支深度**：单次回溯上限 64、加深轮次上限 8 —— 深度超过 8×64 的历史分叉无法一次性补齐（设计上的防风暴约束，非缺陷，但需登记）。
7. **older-peer 兼容**：对端不填 `chain_work` 时退化为比高度（有意设计，行为与 1H 前一致；此时 work-aware 不生效）。

## 12. DEFERRED（精确清单）

- **GAP-1H-A 修复**（建议作为独立存储层阶段，方案 A/B 见 §7）；
- `REORG-1I`（finality / MaxReorgDepth / deep-reorg policy）；
- `REORG-1J`（`/status` fork 观测）完整实现；
- **B5** Orphan Pool 完整实现（含 retention / eviction / 资源配额）；
- **B4** mempool re-add（已由 1F 完成，其余部分）、**B6** by-hash 拉分支的 P2P 层**优化**（批量、流水线、并行对端）；
- **BT-1** remediation（`TestI_RestartLikeReconstruction` 确定化）；
- 既有登记项（未授权不得修）：SP-3 / SP-3b（v2 路径已闭合）、DOC-1、MM-1..8、FC-001..009、SYNC-001/002、P2P-001、OBS-001..003、G-11..G-15、物理 compaction/GC、`internal/config` 死包；
- Windows 挖矿、生产部署、生产 datadir 迁移、Linux 生产重启、防火墙变更 —— 均未授权。

## 13. 授权边界声明

```text
PRODUCTION DEPLOYMENT: NOT AUTHORIZED（本阶段零部署动作）

WINDOWS MINING:        NOT AUTHORIZED（本阶段未在本机启动任何生产挖矿）
```

远程主机（`.123` / `.200`）在本阶段全程未连接、未触碰；生产 datadir 未被读写。

## 14. NEXT PHASE

```text
NEXT PHASE: REORG-1H-GAP-1H-A-REMEDIATION-1
```

**唯一目标**：修复「legacy 前缀不可变导致首次 reorg 被存储层拒绝」，使现实持久化链（生产 1284 块全 legacy）具备完成 reorg 的能力；交付后必须删除/改写两个 tripwire 用例（不得放宽断言），并以**真实双进程**用例证明「竞争分叉后两节点 tip 收敛」。

在此之前，reorg 全链路（1A/1B/1C/1D/1E/1F/1H）在生产环境仍处于「基础设施完备但无法触发」状态。

---

## 15. STOP

本阶段到此为止：未提交、未推送、未扩大范围、未触碰生产。
