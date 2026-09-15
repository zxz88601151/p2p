# PHASE REORG-INFRASTRUCTURE-IMPLEMENTATION-1 — REORG-1B FINAL REPORT

**阶段性质**：STRICT CONTROLLED IMPLEMENTATION（REORG-1B — BlockIndex + SetTip Infrastructure）
**执行时间**：2026-09-14
**结论**：**REORG-1B IMPLEMENTATION PASS**

---

## 1. HARD BASELINE

| 项 | 值 | 证据 |
|---|---|---|
| 起始 HEAD | `b2724c8d0fe155147341ea0f8474d11d6bcb282a` | §5 只读基线 |
| parent | `674c5ad076ed4857a158d6ecd7c73d58e1a61553` | `git rev-parse HEAD^` |
| branch | `main` | `git branch --show-current` |
| 起始 tracked `.go` 漂移 | **零** | `git status --porcelain -- '*.go'` 仅 `?? internal/blocktree/{blocktree,blocktree_test}.go`（1A 未跟踪） |
| 终态 tracked `.go` 漂移 | **零** | 仅新增 `?? internal/blocktree/settip.go` + `settip_test.go`；`blocktree.go` 增 2 字段（见 §2） |
| Reorg execution | **OFF** | blocktree **零生产导入者**（见 §9） |
| 生产 datadir / 节点 | 未触碰 | §12 |
| git 写 | **零**（无 commit/push/tag/merge/rebase/amend/squash） | §15 |

---

## 2. Files Changed

| 文件 | 类型 | 变更 |
|---|---|---|
| `internal/blocktree/settip.go` | **NEW** | REORG-1B SetTip 基础设施：`bestTip`/`bestTipWork` 状态管理 + `SetTip`/`BestTip`/`BestTipWork`/`ActiveHeight`/`ActiveChain`/`IsActiveChain`/`CompareWork`/`ShouldReorg`/`tieBreakWinner`/`ResetTip` + 4 错误常量 |
| `internal/blocktree/settip_test.go` | **NEW** | T6–T11 测试矩阵（8 测试） |
| `internal/blocktree/blocktree.go` | **EDIT（1A 文件，最小）** | `BlockTree` 结构体新增 `bestTip *BlockNode` + `bestTipWork *big.Int` 两字段 + 更新包/结构体 doc 注释（标明 1B tip 基础设施位置）；**1A 逻辑零改动**（AddBlock/CheckInvariant/WorkOfBits/NewBlockNode/PathToRoot 等全部不动） |
| `internal/blocktree/blocktree_test.go` | 未改 | 1A 测试原样保留（T1–T5 由其覆盖） |

> **变更边界干净**：所有改动集中在 `internal/blocktree/` 包内。零跨包改动、零 cmd/node 改动、零 consensus/pow/blockchain/utxo/storage/mempool/p2p 改动。

---

## 3. REORG-1B Implementation

### 3.1 与冻结设计契约的一致性（§16 STOP #5/#7 不触发）

设计报告（`PHASE-REORG-INFRASTRUCTURE-DESIGN-1-REPORT.md`）将 tip 基础设施分两层：

| 层 | 归属 | 范围 | 阶段 |
|---|---|---|---|
| **树级 / 索引级**（R1） | `internal/blocktree` | `bestTip`/`bestTipWork` 指针 + `SetTip` 移动指针 | **REORG-1B（本阶段）** |
| **共识级**（R3） | `internal/blockchain` | `SetTip`/`ConnectBlock`/`DisconnectBlock` + UTXO disconnect/connect + mempool re-add + 存储原子提交 | REORG-1C（后续） |

- 设计 §C.2-q4（line 77）：「active tip 如何表示？**显式字段 `bestTip *BlockNode` + `bestTipWork *big.Int`**」→ 本阶段在 `BlockTree` 上落地此两字段。
- 设计 §E（fork-choice 契约）：`candidate.CumulativeWork.Cmp(active)` + tie-break（tip hash 大端较大者胜）→ 本阶段 `CompareWork`/`ShouldReorg` 落地。
- 设计 REORG-1C（line 396）：「R3 `SetTip`/`ConnectBlock`/`DisconnectBlock` 脚手架 + `bestTip` 切换」在 `internal/blockchain` → **本阶段不触碰**。
- §9 契约：`SetTip` 不得隐式修改 UTXO/mempool/持久化 → 本阶段 `SetTip` 仅设两内存指针，**完全合规**。
- STOP #7（SetTip 必须依赖尚未设计的 undo/storage）→ **不触发**：本 SetTip 不依赖任何 undo/storage（那是 1C 的职责）。

### 3.2 SetTip 契约（§9 严格边界）

```text
SetTip 负责：
  • 校验 node 非空且存在于本树（LookupNode）；
  • 设置 bestTip = node、bestTipWork = node.CumulativeWork（缓存，O(1) fork-choice）；
  • 失败时 bestTip/bestTipWork 保持不变（原子语义）。

SetTip 明确不负责（属后续阶段）：
  • UTXO 回滚 / undo（REORG-1D / R4，FC-008）；
  • mempool re-add（REORG-1F / R6，FC-006）；
  • 存储层 Delete/Truncate/原子提交（REORG-1E / R5，FC-007）；
  • common ancestor 计算 / 分支 disconnect+connect（REORG-1C / R3）；
  • MaxReorgDepth 深度策略（REORG-1I / R9，BG-3）；
  • 通知矿工/RPC tip 变更（REORG-1C 接入 blockchain.notifyTipChanged）。
```

> 本 SetTip 是「未通电的开关」：数据结构层指针就位，但生产 AddBlock 路径不调用它（见 §9），因此不会因本方法的存在而自动发生链替换。

---

## 4. BlockIndex Model

`BlockTree`（REORG-1A + 1B 增量）现已确定性表达设计 §C 要求的全部字段：

| 设计要求（§7） | 实现 | 位置 |
|---|---|---|
| BlockHash | `BlockNode.Hash [32]byte` | blocktree.go:83 |
| ParentHash | `BlockNode.ParentHash [32]byte` | blocktree.go:84 |
| Height | `BlockNode.Height int` | blocktree.go:85 |
| Header（共识字段） | `Bits`+`Timestamp`（Header 的共识关键子集；完整 Block 留 storage，BlockNode 独立于完整 Block，§C.2-q1） | blocktree.go:86-87 |
| CumulativeWork | `BlockNode.CumulativeWork *big.Int` | blocktree.go:89 |
| Children/descendant | `BlockNode.Children []*BlockNode` + `Parent *BlockNode`（双向链接） | blocktree.go:92-93 |
| **Main-chain membership**（1B 新增） | `BlockTree.bestTip` + `IsActiveChain()` | settip.go |
| Block exists vs Block is on active chain | `LookupNode(hash)`（存在）≠ `IsActiveChain(node)`（活动链成员）—— **显式分离**，不再用 `PrevHash==tip` 作存在性判据 | settip.go |

### 「Block exists」与「Block is on active chain」的显式区分（§7 关键要求）

- `tr.Has(hash)` / `tr.LookupNode(hash)` → 块是否存在于索引（含侧链）。
- `tr.IsActiveChain(node)` → 块是否在 `bestTip.PathToRoot()` 上。
- 二者独立：侧链块 `Has==true` 但 `IsActiveChain==false`。这正是未来 reorg 能暂存竞争分支的前提（§10 拓扑）。

---

## 5. SetTip Contract

已在 §3.2 给出。补充 fork-choice 决策（§E，纯只读比较，不切换）：

```text
ShouldReorg(candidate):
  cmp = candidate.CumulativeWork.Cmp(bestTipWork)
  cmp > 0 → true（work 反超）
  cmp < 0 → false（work 不足）
  cmp == 0 → tie-break：bytes.Compare(candidate.Hash, bestTip.Hash) > 0 → candidate 胜
```

- `CompareWork` 只读比较，返回 Cmp 结果 + 错误（无活动 tip / 候选不在树）。
- `ShouldReorg` 应用 §E 决策，**不切换** bestTip；实际切换由调用方（1C）在完成 UTXO rollback+connect 后显式调 `SetTip`。
- **铁律**：network arrival order 绝不决定 consensus chain（§E.2）—— tie-break 只用 tip hash 字典序。

---

## 6. Cumulative Work Behavior

| 检查项 | 结论 | 证据 |
|---|---|---|
| 累加器类型 | `*big.Int`（**未退回 uint64/int64/float**） | blocktree.go:89, settip.go `SetTip` |
| 计算依据 | `parent.CumulativeWork + WorkOfBits(Bits)`（**不基于 height**） | blocktree.go:104-107 `NewBlockNode` |
| overflow 安全 | `bits` 可达 32+ ⇒ 跨千块必溢出 64 位 ⇒ 强制 `*big.Int` | 设计 §D.2 |
| 比较语义 | `Cmp`（>0 候选胜 / <0 活跃胜 / =0 tie-break） | settip.go `CompareWork`/`ShouldReorg` |
| 相等 work 行为 | tie-break：tip hash 大端较大者胜（确定性，全网可复算） | settip.go `tieBreakWinner` |
| **同高度/不同 work 比较** | **PASS** — T9：B→C(bits16) vs B→D→E(bits16) work 不同可比较 | TestT9 |
| **低高度/高 work 表达** | **PASS** — T6：S3(h3,bits32) work >> L4(h4,bits16) work | TestT6 |
| **不同难度分支累计** | **PASS** — T6 双分支（bits16/bits32）各自正确累计 | TestT6 |

### §6 关键证明：chainwork ≠ height（解钳后的核心价值）

T6 构造共享 genesis 的双分支：
- LONG：G→L1→L2→L3→L4（每块 bits=16，height=4），`CW(L4)=5×2^16=327680`
- SHORT：G→S1→S2→S3（每块 bits=32，height=3），`CW(S3)=2^16+3×2^32≈1.29×10^10`

**S3(height=3) 的 CumulativeWork >> L4(height=4)** → 证明解钳后 chainwork 不再 ≡ height×const，低高度链可具更高 work，fork-choice 必须用 work 而非高度。`SetTip(L4)` 后 `ShouldReorg(S3)==true`。

---

## 7. Active-Chain Semantics

| 操作 | 行为 | 测试 |
|---|---|---|
| `SetTip(node)` | 设 bestTip=node、bestTipWork=CW(node)；校验 node 在树；失败原子不变 | TestT7 |
| `BestTip()` | 返回活动链尾（未设置→nil） | TestT7 |
| `ActiveHeight()` | 活动链尾高度（未设置→-1） | TestT7 |
| `ActiveChain()` | `[bestTip,...,genesis]` 路径（未设置→nil） | TestT8 |
| `IsActiveChain(node)` | node 是否在 bestTip.PathToRoot() 上 | TestT8 |
| `ResetTip()` | 清空 tip 状态（仅供测试/重建） | TestT8/T11b |

**活动链成员判定**沿 `bestTip.PathToRoot()` 线性扫描（链长=height+1），**不引入额外 map**——刻意规避 BT-1 类 map 迭代 nondeterminism。

---

## 8. Competing-Branch Behavior

T9 构造 §10 拓扑（fork at B）：

```text
A(h0) → B(h1) → C(h2)     [活动分支]
            \
              D(h2) → E(h3)  [侧链]
```

- `LookupNode(C/D/E)` 均 ≠ nil → **两条分支并存，侧链未被丢弃**（§10 要求）。
- `SetTip(C)` → 活动链尾=C。
- `ShouldReorg(E)` → true（E work=4×2^16 > C work=3×2^16），**但 bestTip 仍是 C**（决策与执行分离）。
- `IsActiveChain(D/E)` = false（侧链不在活动链）。

---

## 9. REORG OFF Proof

### 9.1 数据结构层：AddBlock 不自动切换 tip

`TestT10_ReorgOff_NoAutoSwitchOnAddBlock`：
- `SetTip(C)`（低 work 活动链尾）→ 加入侧链 `high`(bits=32, work>>C)。
- **断言**：`AddBlock(high)` 后 `BestTip()` 仍是 C——**未自动 SetTip**。
- 即便 `ShouldReorg(high)==true`，bestTip 仍是 C（决策≠执行）。
- 只有显式 `SetTip(high)` 才切换。

`BlockTree.AddBlock`（1A，本阶段未改）只做「插入+链接+CW 计算」，**无任何 SetTip/ShouldReorg 调用**。

### 9.2 生产路径层：blocktree 零生产导入者

```text
grep -rn "blocktree" cmd/ internal/blockchain/ internal/p2p/ internal/mempool/
         internal/storage/ internal/utxo/ internal/wallet/ internal/transaction/
         internal/control/   →  [空]
grep -rln '"p2pchain/internal/blocktree"' .   →  [空]
```

**blocktree 包不被任何生产代码导入**。因此：
- `blockchain.AddBlock` 仍是单链 append（`PrevHash==tip`，blockchain.go:223），语义不变。
- 矿工路径（`cmd/node` mineOnce → AddBlock）不变。
- P2P 接收路径（`OnNewBlock`/`OnBlocksResp` → AddBlock）不变。
- 竞争块仍静默丢弃（FC-002，reorg 未启用）。

**REORG EXECUTION = OFF 已证明。**

---

## 10. Tests

### 新增测试（settip_test.go，8 项全 PASS）

| 测试 | 覆盖 | 结果 |
|---|---|---|
| `TestT6_LowerHeightHigherWork` | T6 低高度高 work + 不同难度分支累计 | PASS |
| `TestT7_SetTipContract` | T7 SetTip 契约（nil/游离/合法/原子） | PASS |
| `TestT8_ActiveChainMembership` | T8 活动链成员判定 + ActiveChain 序 + ResetTip | PASS |
| `TestT9_CompetingBranch` | T9 竞争分支并存 + 决策≠执行 | PASS |
| `TestT10_ReorgOff_NoAutoSwitchOnAddBlock` | T10 AddBlock 不自动切换 tip | PASS |
| `TestT10b_TieBreakDeterministic` | tie-break 确定性（hash 大端较大者胜） | PASS |
| `TestT11_RestartReconstructionDeterministic` | T11 重启重建确定性（固定序切片，避开 BT-1） | PASS |
| `TestT11b_NoPersistenceByDesign` | T11 文档化：本阶段不持久化 | PASS |

### 1A 测试回归（blocktree_test.go，未改）

| 测试 | 结果 |
|---|---|
| TestA–TestH, TestJ + 8 负例 | **全 PASS** |
| `TestI_RestartLikeReconstruction` | **间歇 FAIL（BT-1，已知，非本阶段引入）** |

> T1–T5（Index insertion / Parent lookup / Height / Cumulative work / 不同难度分支比较）由 1A 的 TestA–TestJ 覆盖，本文件不重复。

---

## 11. Existing Test Regression

| 包 | 结果 |
|---|---|
| `internal/blockchain` | ok（cached） |
| `internal/pow` | ok（cached） |
| `internal/utxo` | ok（cached） |
| `internal/block` | ok（cached） |
| `internal/transaction` | ok（cached） |
| `internal/blocktree`（除 TestI） | 全 PASS |
| `go vet ./...` | **VET_0** |
| `go build ./...` | **BUILD_0** |
| `go test -race`（blocktree SetTip 测试） | **ok，无 DATA RACE** |

**零回归**：所有共识相关包测试不变（cached = 与 b2724c8 一致）。blocktree 包唯一间歇失败的是 1A 的 TestI（BT-1，map 迭代序，与 1B 改动无关——1B 的 SetTip 不触及 AddBlock 的 map 迭代；T11 刻意用固定序切片规避）。

---

## 12. Production Safety

- **零 SSH / 零部署 / 零生产 datadir 触碰 / 零生产节点重启**
- 所有实验在仓库内 `internal/blocktree/`（纯内存单测）
- blocktree 零生产导入者（§9.2）
- 无 production migration、无 production activation
- Reorg execution 保持 OFF

---

## 13. Known Limitations

1. **BT-1（未修，out-of-scope）**：`TestI_RestartLikeReconstruction` 间歇失败（Go map 迭代序 + AddBlock 要求父先存在）。1B 的 `T11` 用固定序切片规避了同一问题。建议后续 `BT-1-REMEDIATION-1`（按 height 排序确定化重放列表）。
2. **SetTip 未通电**：本 SetTip 是树级原语，未被任何生产路径调用。需 REORG-1C 在完成 UTXO rollback+connect 后接入 `blockchain`。
3. **无持久化**：BlockTree 是纯内存结构；`bestTip`/`bestTipWork` 重启丢失。持久化（`block_index.dat` + active tip 指针）属 REORG-1E（R5）。重启重建（设计 §C.7）从 `blocks.dat` 顺序扫描重放 + `SetTip(max-CW-Valid-node)`，本阶段 T11 已证明重建确定性。
4. **无 MaxReorgDepth 检查**：BG-3 已定稿为「告警+限速，永不拒绝更高 work 链」；深度策略属 REORG-1I，本 SetTip **永不**因深度拒绝（§9 契约）。
5. **tie-break 方向已冻结**：tip hash 大端较大者胜（§E.2 推荐）。若未来要改为较小者胜，须作为独立共识参数设计阶段（影响全网一致性，不可随意改）。

---

## 14. Remaining REORG-1C+ Dependencies

| 组件 | 状态 | 阻断项 |
|---|---|---|
| **REORG-1C**（R3 共识级 SetTip/Connect/Disconnect） | 未实现 | 须在 `internal/blockchain` 接入本 1B SetTip + 完成 UTXO disconnect/connect；依赖 1D/1E |
| **REORG-1D**（R4 UTXO undo） | 未实现 | `utxo.Set.Spend` 只 delete（FC-008）；须加 undo log 或检查点重放 |
| **REORG-1E**（R5 存储 hash 索引 + Delete/Truncate + block_index.dat） | 未实现 | `FileBlockStore` append-only（FC-007） |
| **REORG-1F**（R6 mempool ReaddDisconnected） | 未实现 | FC-006 |
| **REORG-1G**（orphan pool） | 未实现 | FC-002 |
| **REORG-1H**（P2P 按 hash 拉分支 + work-aware handshake） | 未实现 | FC-003 |
| **REORG-1I**（R9 最终性/MaxReorgDepth） | 未实现 | FC-005（BG-3 已定稿） |
| **REORG-1J**（/status fork/orphan 可观测） | 未实现 | FC-002 |

> 本阶段（1B）为 1C 提供了**数据前提**（bestTip 指针 + ShouldReorg 决策 + tie-break），但**不解除** 1C–1J 任一阻断项。生产 reorg 启用须待 1C–1I 全部就绪 + 独立 Integration Audit。

---

## 15. Commit Readiness Recommendation

### 裁定

```text
REORG-1B IMPLEMENTATION PASS
```

### 证据摘要

| 门 | 结果 |
|---|---|
| §3 设计契约一致性（不与 R1–R9 冲突） | PASS（树级 vs 共识级分层，§16 STOP #5/#7 不触发） |
| §4 BlockIndex 表达能力（含竞争分支） | PASS |
| §5 SetTip 契约（不触 UTXO/mempool/存储） | PASS |
| §6 CumulativeWork（*big.Int、不基于 height、低高度高 work） | PASS |
| §7 Active-chain 语义 | PASS |
| §8 竞争分支并存 | PASS |
| §9 REORG OFF（数据结构层 + 生产隔离） | PASS（零生产导入者） |
| §10 测试 T6–T11 | PASS（8/8） |
| §11 既有测试回归 | PASS（零回归；TestI=BT-1 已知非本阶段） |
| §12 生产安全 | PASS（零触碰） |
| go vet/build | PASS |
| go test -race | PASS（无 DATA RACE） |

### Commit 建议

**建议进入 `REORG-1B-COMMIT-READINESS-AUDIT`**（需用户显式授权）。建议提交范围：
- `internal/blocktree/settip.go`（NEW）
- `internal/blocktree/settip_test.go`（NEW）
- `internal/blocktree/blocktree.go`（EDIT：2 字段 + doc）
- `internal/blocktree/blocktree_test.go`（1A，随包提交）

> 不自动 commit；不自动进入 REORG-1C；不启用 Reorg。停在：
> `IMPLEMENTATION COMPLETE → COMMIT READINESS AUDIT → WAIT FOR EXPLICIT AUTHORIZATION`

---

## 附：纪律声明

- **零 git 写**（无 commit/push/tag/merge/rebase/amend/squash）
- **零生产触碰**（无 SSH / 无部署 / 无 datadir 变更 / 无节点重启）
- **未激活 Reorg**、**未接入生产路径**、**未实施 1C+ 任一组件**
- **未改 consensus/difficulty/MTP/PoW/validateBlock**（§16 STOP #1/#2 不触发）
- **未自动进入下一阶段**，等待用户显式授权

**报告产出**：`PHASE-REORG-INFRASTRUCTURE-IMPLEMENTATION-1-REORG-1B-FINAL-REPORT.md`（本文件）
