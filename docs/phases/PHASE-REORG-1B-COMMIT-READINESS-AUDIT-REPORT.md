# PHASE REORG-1B-COMMIT-READINESS-AUDIT — FINAL REPORT

**阶段性质**：STRICT READ-ONLY / COMMIT BOUNDARY / REORG INFRASTRUCTURE AUDIT
**执行时间**：2026-09-14
**结论**：**COMMIT READY WITH BOUNDARY CONDITION**

---

## 1. HARD BASELINE

| 项 | 值 | 证据 |
|---|---|---|
| HEAD | `b2724c8d0fe155147341ea0f8474d11d6bcb282a` | `git rev-parse HEAD` |
| HEAD^ | `674c5ad076ed4857a158d6ecd7c73d58e1a61553` | `git rev-parse HEAD^` |
| branch | `main` | `git branch --show-current` |
| `.go` tracked 漂移 | **零** | `git diff --stat` 仅 docs |
| `.go` untracked | **4 个**（全部在 `internal/blocktree/`） | `git status --porcelain -- '*.go'` |
| staged diff | **空** | `git diff --cached --stat` |
| `git diff --check` exit | 2（仅 pre-existing docs 末尾空格，**与 1B 无关**） | 文档问题不在本审计范围 |
| 工作树漂移 | **OUT-OF-BASELINE DRIFT**: 2 个 docs 修改 + 14 个未跟踪 doc 报告（全部为前期阶段产物，**与 1B 无关**，不归本审计处理） | `git status --short` |

> 无新外部漂移；审计范围严格限于 `internal/blocktree/` 内的 4 个 untracked `.go` 文件。

---

## 2. EXACT WORKTREE STATE

| 路径 | 大小 | 状态 |
|---|---|---|
| `internal/blocktree/blocktree.go` | 356 行 | untracked（**1A 创建 + 1B 增量**） |
| `internal/blocktree/blocktree_test.go` | 418 行 | untracked（**1A 创建，1B 未改**） |
| `internal/blocktree/settip.go` | 200 行 | untracked（**1B 新建**） |
| `internal/blocktree/settip_test.go` | 399 行 | untracked（**1B 新建**） |

> `git ls-files internal/blocktree/` 返回空——4 个文件**全部未跟踪**，从未 commit。

---

## 3. 1A / 1B FILE OWNERSHIP

| File | 1A | 1B | Untracked | Modified (vs 1A state) | Commit Recommendation |
|---|---|---|---|---|---|
| `blocktree.go` | ✓ 1A 主体（343 行：BlockNode / WorkOfBits / NewBlockNode / AddBlock / PathToRoot / AncestorAtHeight / IsAncestorOf / BlockTree 原结构 / CheckInvariant / hasCycle / isZeroHash / 错误集 / 文档） | ✓ 1B 增量：包 doc 注释增 ~5 行 + BlockTree doc 增 ~2 行 + 2 字段（`bestTip`/`bestTipWork`）+ 字段 doc 4 行 | ✓ | **是**（1A → 1B：+13 行） | **推荐 1A 提交不含 1B 增量 / 1B 提交含 1B 增量** |
| `blocktree_test.go` | ✓ 1A 全部（TestA–TestJ + 8 负例 + 辅助 h32/hStr/mustAdd/expectedChainCW） | **零**（1B 报告明确「未修改」） | ✓ | **否** | **随 1A 提交**（属于 1A 测试基线） |
| `settip.go` | — | ✓ 1B 全部（200 行：4 错误常量 + SetTip/BestTip/BestTipWork/ActiveHeight/ActiveChain/IsActiveChain/CompareWork/ShouldReorg/tieBreakWinner/ResetTip） | ✓ | — | **随 1B 提交** |
| `settip_test.go` | — | ✓ 1B 全部（T6–T11b 共 8 测试） | ✓ | — | **随 1B 提交** |

### A. 属于 1A 的内容
- `blocktree.go` 的全部 1A 逻辑（已确认 1A 报告 VERDICT=PASS，独立编译/测试全绿）
- `blocktree_test.go` 全部内容（1A 测试矩阵）

### B. 属于 1B 的内容
- `settip.go` 全部（新建）
- `settip_test.go` 全部（新建）
- `blocktree.go` 的 2 个新字段（`bestTip`/`bestTipWork`）+ 字段 doc 4 行 + `BlockTree` struct doc 增 ~2 行 + 包 doc 增 ~5 行（共 +13 行）

### C. 虽然未修改但属于历史工作的文件
- `blocktree_test.go`：1A 创建后从未 commit；属于 1A 历史工作（不是 1B）

### D. 是否应拆成独立 commits

**是，且本审计强烈推荐拆成两 commit**：

| Commit | File Set | 语义 |
|---|---|---|
| **Commit 1: REORG-1A** | `blocktree.go`（不含 bestTip 字段） + `blocktree_test.go` | 「纯 BlockNode/BlockTree 内存基础设施」独立可编译、可测试 |
| **Commit 2: REORG-1B** | `blocktree.go` delta（+2 字段 + doc） + `settip.go` + `settip_test.go` | 「REORG-1A 基础上增量新增树级 tip 状态 + SetTip 原语」 |

**理由**：
1. 1A 是逻辑完整、测试通过、可独立编译的最小单元（不含 SetTip/ShouldReorg/CompareWork 等 1B 概念）
2. 1B 是 1A 之上的严格增量（additive：2 字段 + 新文件）；settip.go 不修改 1A 的任何方法
3. 拆 commit 后 `git log -- blocktree.go` 可清晰看到两阶段的演进
4. 若 1C 需要回退 1B 而保留 1A 基础设施，单独 commit 让 revert 干净
5. **不能合并成单一 commit 的硬性证据**：当前 `blocktree.go` 是「1A 内容 + 1B 增量」的混合状态；若强行一次 commit 全部 4 个文件，commit message 必须同时描述两阶段语义，模糊 1A/1B 边界——违反 §9「不得凭感觉判断」

---

## 4. EXACT DIFF

### 4.1 1B 对 `blocktree.go` 的修改范围（已通过 `grep` 精确定位）

| 行号 | 内容 | 类型 |
|---|---|---|
| 1–13 | 包 doc 注释：描述从「1A 范围」更新为「1A + 1B」联合说明 | doc |
| 18–21 | 包 doc 注释：增 1B 新增项（bestTip 指针 + SetTip 原语） | doc |
| 166–168 | `BlockTree` struct doc 注释：增「REORG-1B 在其上新增树级 tip 状态…」 | doc |
| 171–175 | 字段 doc 注释：4 行说明 bestTip/bestTipWork 语义 + 与 REORG-1C 边界 | doc |
| **176–177** | **2 个新字段**：`bestTip *BlockNode` / `bestTipWork *big.Int` | **code** |

### 4.2 1A 逻辑函数全部零改动（逐项核对）

| 函数 | 行号 | 1B 是否修改 | 证据 |
|---|---|---|---|
| `WorkOfBits` | 68–72 | ✗ | 1B 报告 §11 列「零回归」 |
| `NewBlockNode` | 102–120 | ✗ | 同 |
| `PathToRoot` | 123–131 | ✗ | 同 |
| `AncestorAtHeight` | 135–145 | ✗ | 同 |
| `IsAncestorOf` | 148–158 | ✗ | 同 |
| `BlockTree.AddBlock` | 206–243 | ✗ | 同 |
| `BlockTree.CheckInvariant` | 265–318 | ✗ | 同 |
| `hasCycle` | 321–332 | ✗ | 同 |
| `isZeroHash` | 335–342 | ✗ | 同 |

**代码变更总计**：2 字段 + 13 行 doc 注释。无逻辑方法改动。

### 4.3 `git diff --check` 状态

`git diff --check` exit code 2，但**所有警告均在 `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md`（tracked doc，前阶段产物）**，**与 REORG-1B 的 4 个 untracked `.go` 文件无关**。这是 pre-existing issue，不在 1B 审计范围。

---

## 5. SetTip Audit

`settip.go` 实际符号清单 vs §4 要求清单：

| 要求 | 实际 | 行号 | 状态 |
|---|---|---|---|
| `bestTip` 字段 | ✓ | blocktree.go:176 | PASS |
| `bestTipWork` 字段 | ✓ | blocktree.go:177 | PASS |
| `SetTip` | ✓ | settip.go:69 | PASS |
| `BestTip` | ✓ | settip.go:83 | PASS |
| `BestTipWork` | ✓ | settip.go:89 | PASS |
| `ActiveHeight` | ✓ | settip.go:94 | PASS |
| `ActiveChain` | ✓ | settip.go:103 | PASS |
| `IsActiveChain` | ✓ | settip.go:114 | PASS |
| `CompareWork` | ✓ | settip.go:136 | PASS |
| `ShouldReorg` | ✓ | settip.go:164 | PASS |
| `tieBreakWinner` | ✓ | settip.go:186 | PASS |
| `ResetTip` | ✓ | settip.go:197 | PASS |

### SetTip 不执行清单（§4 要求确认）

| 项 | 是否执行 | 证据 |
|---|---|---|
| UTXO disconnect | ✗ | settip.go 全文无 `utxo`/`UTXO`/`Spend`/`Disconnect` 等 |
| UTXO connect | ✗ | 同 |
| mempool mutation | ✗ | settip.go 全文无 `mempool`/`ReaddDisconnected` |
| block storage mutation | ✗ | settip.go 全文无 `storage`/`SaveBlock`/`Delete`/`Truncate` |
| persistent tip mutation | ✗ | settip.go 不写盘（`BlockTree` 无任何 I/O 方法） |
| P2P mutation | ✗ | settip.go 全文无 `p2p`/`Broadcast`/`Send` |
| consensus activation | ✗ | settip.go 仅 `Cmp`/指针写入 |

> **SetTip 仅 `t.bestTip = node; t.bestTipWork = new(big.Int).Set(node.CumulativeWork)`**（settip.go:77-78）。**两次内存指针写入**。除此之外无任何外部状态变更。

---

## 6. REORG-OFF Audit

### 6.1 `BlockTree.AddBlock` 不调 SetTip/ShouldReorg（§5 要求）

```
grep "SetTip|ShouldReorg|CompareWork" internal/blocktree/blocktree.go
→ 5 处匹配，**全部在注释中**（行 20, 21, 166, 173, 175），零代码调用
```

### 6.2 `BlockTree.AddBlock` 不自动 fork 选择 / 链替换

`AddBlock`（blocktree.go:206-243）只做：
- 校验 height/bits/重复/根/父存在/自引用/高度一致
- 调 `NewBlockNode` 计算 Work/CumulativeWork
- 追加 `parent.Children` 与 `t.nodes[hash]`

**无任何 SetTip/ShouldReorg/BestTip 调用**——AddBlock 是纯插入函数。

### 6.3 production code 不 import `internal/blocktree`

```
grep -rln '"p2pchain/internal/blocktree"' .   →  [空]
```

`blocktree` 包**零生产导入者**。生产路径（cmd/node, blockchain, p2p, mempool, storage, utxo, wallet, transaction, control）**完全隔离**。

### 6.4 验证

```text
REORG EXECUTION = OFF   ✓  PASS
```

---

## 7. Consensus Semantics Audit

### 7.1 CumulativeWork 类型与计算（§6 要求）

| 检查项 | 实际 | 证据 |
|---|---|---|
| 类型必须是 `*big.Int` | ✓ | blocktree.go:89, 93-94, 177；settip.go:78, 89, 146 |
| 计算必须是 `parent.CumulativeWork + WorkOfBits(Bits)` | ✓ | blocktree.go:108-111 `cw := new(big.Int).Set(work); if parent != nil { cw.Add(cw, parent.CumulativeWork) }` |
| **不得按 height 计算** | ✓ | blocktree.go:269-274 CheckInvariant 明文：CumulativeWork = Parent.CW + Work |
| **不得 uint64/int64/float 累计** | ✓ | grep 无 `uint64.*Cumulative\|int64.*Cumulative\|float.*Cumulative` |
| **不得退回 height 推导** | ✓ | blocktree.go:172-177 bestTipWork 仅缓存 `node.CumulativeWork`，**不**存 height |

### 7.2 Fork choice（§6 要求）

| 规则 | 实际 | 证据 |
|---|---|---|
| cumulative work higher → winner | ✓ | settip.go:165-167 `cmp > 0 → true` |
| equal work → deterministic tip-hash tie-break | ✓ | settip.go:170-176 tieBreakWinner: `bytes.Compare(candidate.Hash, active.Hash)` |
| arrival order 不决定 consensus | ✓ | settip.go:22-23 铁律；tieBreakWinner 仅用 hash 字节序，**不读时间戳/peer/到达序** |

---

## 8. Test Results

| 测试 | 命令 | 结果 |
|---|---|---|
| `go vet ./...` | `go vet ./...` | **VET_0**（exit 0，零警告） |
| `go build ./...` | `go build ./...` | **BUILD_0**（exit 0） |
| `TestT6_LowerHeightHigherWork` | `go test -run TestT6 ./internal/blocktree/` | **PASS** |
| `TestT7_SetTipContract` | 同 | **PASS** |
| `TestT8_ActiveChainMembership` | 同 | **PASS** |
| `TestT9_CompetingBranch` | 同 | **PASS** |
| `TestT10_ReorgOff_NoAutoSwitchOnAddBlock` | 同 | **PASS** |
| `TestT10b_TieBreakDeterministic` | 同 | **PASS** |
| `TestT11_RestartReconstructionDeterministic` | 同 | **PASS** |
| `TestT11b_NoPersistenceByDesign` | 同 | **PASS** |
| Race 检测（T6–T11b） | `go test -race -run 'TestT6\|...\|TestT11b' ./internal/blocktree/` | **ok**，无 DATA RACE |
| 1A 测试矩阵（TestA–J + 8 负例，除 TestI） | `go test ./internal/blocktree/`（3 次跑） | 全 PASS（**TestI 间歇 FAIL = BT-1，见 §9**） |

---

## 9. BT-1 Isolation

### 9.1 BT-1 现象（已记录）

`TestI_RestartLikeReconstruction`（blocktree_test.go:217）间歇失败，根因 Go map iteration nondeterminism：
- 1A `TestI` 使用 `for _, n := range orig.nodes`（line 233）和 `for h, on := range orig.nodes`（line 249）迭代 map
- `AddBlock` 要求父先存在 → map 迭代序随机 → 重放顺序不确定 → 偶发失败
- 6 进程复跑 4 PASS / 2 FAIL（已记录于 PHASE DIFFICULTY-CONSENSUS-IMPLEMENTATION-1）

### 9.2 BT-1 与 1B 隔离证据

| 检查 | 结果 |
|---|---|
| 1B `settip.go` 是否使用 `range ... nodes` 迭代 map | ✗（grep 零匹配） |
| 1B `settip_test.go` 是否使用 `range ... nodes` 迭代 map | ✗（grep 零匹配） |
| 1B `T11` 重启重建测试是否迭代 map | ✗（line 333-344 使用 `for _, r := range replay`——`replay` 是**固定顺序 struct 切片**，非 map） |
| 1B SetTip 是否触及 AddBlock 的 map | ✗（SetTip 仅写 2 个内存指针） |
| 1B CumulativeWork 是否改变 | ✗（1A `NewBlockNode` 计算逻辑零改动） |
| 1B tie-break 是否引入新的 nondeterminism | ✗（`bytes.Compare` 是确定性字节比较） |

**结论**：BT-1 是 **1A pre-existing 问题**，**非 1B 引入**，**非 SetTip/cumulative work/tie-break 引入**。**不属本审计范围，不修复**。

后续处理登记：`BT-1-REMEDIATION-1`（建议按 height 排序确定化重放列表——属独立阶段）。

---

## 10. Commit Boundary Decision

### 三选项分析

#### Option A（仅提交 1B 严格文件）
> 「只提交严格属于 1B 的文件」

**不可行**：`blocktree.go` 当前状态是「1A 主体 + 1B 增量」的混合文件，无法在不拆 1A 的前提下「只提交 1B 部分」。`settip.go` 依赖 `BlockTree.bestTip` 字段，该字段仅在 1B 增量中定义——若仅提交 1B 而无 1A 基础，`settip.go` 无法编译。

#### Option B（先 1A commit，再 1B commit）✓ **推荐**
> 「REORG-1A 尚未形成独立 commit，必须先完成 1A commit，再进行 1B commit」

**证据强支持**：
1. `blocktree.go` 的 1A 部分（不含 bestTip 字段）独立编译 + 全部 1A 测试 PASS（1A 报告已验证）
2. `blocktree.go` 的 1B 增量（+2 字段 + doc）是纯 additive，可作为 1A 之上的明确 delta
3. `blocktree_test.go` 1B 未修改（明确证据：`git diff` 无任何 1A→1B 内容变化；1B 报告明确「未修改」），属于 1A 工作
4. 拆 commit 后 `git log -- blocktree.go` 可清晰看到两阶段演进
5. 若 1C 需要仅回退 1B 而保留 1A 基础设施，单独 commit 让 `git revert` 干净

#### Option C（混合提交）
> 「经过证据证明，可以把某些 1A 未提交文件与 1B 一起提交」

技术上可行（单次 commit 4 文件），但**语义损失**：
- 1A 与 1B 是逻辑独立、阶段独立的两个 unit，合并 commit 模糊了阶段边界
- §9「不得凭感觉判断」+「优先 COMMIT READY WITH BOUNDARY CONDITION」指向保守拆分

### 裁定

**采用 Option B：拆成两个 commit**。

| Commit | 文件集 | Commit Message 草案 |
|---|---|---|
| **Commit 1: REORG-1A** | `blocktree.go`（不含 bestTip 字段）+ `blocktree_test.go` | `blocktree: add BlockNode/BlockTree memory infrastructure (REORG-1A)` |
| **Commit 2: REORG-1B** | `blocktree.go` delta（+2 字段 + doc）+ `settip.go` + `settip_test.go` | `blocktree: add tree-level tip state and SetTip primitive (REORG-1B)` |

---

## 11. Recommended Commit File Set

### Commit 1: REORG-1A（先提交，1A 阶段追溯）

```text
internal/blocktree/blocktree.go          (1A 主体，不含 bestTip/bestTipWork)
internal/blocktree/blocktree_test.go     (1A 全部 TestA–J + 8 负例 + helpers)
```

**前提**：commit 时 `blocktree.go` 必须只含 1A 内容（即临时移除 bestTip/bestTipWork 字段）——这需要 git 工作流技巧：可以用 `git add -p` 暂存 1A 部分，或 commit 后再单独补 commit 2。

### Commit 2: REORG-1B（在 Commit 1 之上增量提交）

```text
internal/blocktree/blocktree.go          (delta: +2 字段 bestTip/bestTipWork + doc)
internal/blocktree/settip.go             (NEW)
internal/blocktree/settip_test.go        (NEW)
```

### 执行要求

- 两 commit 必须**显式分离**（禁止 `git commit -a` 自动扫描，需 `git add <file>` 精确指定）
- 不得使用 `--amend` 合并两 commit
- 建议 commit message 严格遵循上述草案
- 推荐用 `git rev-parse HEAD` 在两 commit 之间记录父 SHA，便于追溯

---

## 12. Excluded Files

以下文件**不属于 REORG-1B commit**，明确排除：

| 文件/目录 | 排除原因 |
|---|---|
| `docs/*.md`（2 modified + 14 untracked） | pre-existing 文档工作，前阶段产物，与 1B 无关；1B 报告独立维护在工作树根的 `挖矿/` 目录 |
| `verifier/`（untracked 目录） | 与 blocktree 平行的另一个实验性目录（PHASE VERIFIER-*）；未与 1B 协调 |
| 任何 `cmd/node/*.go` / `internal/blockchain/*.go` / `internal/p2p/*.go` / 等 | **REORG OFF**：blocktree 零生产导入者；生产路径未触碰 |
| 任何难度/MTP/PoW/validateBlock 相关文件 | §16 STOP #1/#2 不触发：1B 未改 consensus rule |

---

## 13. Production Safety

| 项 | 状态 |
|---|---|
| SSH / 部署 / 生产节点重启 / datadir 变更 | ✗ 零 |
| Production touch | ✗ 零 |
| Git write（add/commit/push/tag/merge/rebase/amend/checkout/restore/clean/stash/cherry-pick/reset） | ✗ 零（本阶段全程只读） |
| Working tree / index / refs 变更 | ✗ 零 |
| Staged changes | ✗ 零（`git diff --cached --stat` 空） |
| Commit created | ✗ 零 |
| Push / tag | ✗ 零 |
| Out-of-baseline drift | 仅 pre-existing docs（明确登记，非 1B 范围，不修改） |

---

## 14. FINAL VERDICT

# **COMMIT READY WITH BOUNDARY CONDITION**

### 边界条件 1：Commit 必须拆为两个（REORG-1A + REORG-1B），不得合并

**理由**（§10 Option B 证据已列）：
- 1A 逻辑完整可独立编译/测试，1B 是严格 additive 增量
- `blocktree_test.go` 1B 未修改，属于 1A
- `blocktree.go` 含 1A 主体 + 1B 增量两个逻辑阶段，混合 commit 模糊阶段边界
- 拆 commit 让 `git revert` 1B 而保留 1A 基础设施成为可能

### 边界条件 2：建议 commit 前对 4 个 blocktree 文件跑 `gofmt -w`（仅 1B 范围）

**理由**：
- `gofmt -l internal/blocktree/` 标记 `settip.go` 与 `settip_test.go`（注释中使用 `•` 字符，gofmt 会替换为 `-`）
- `gofmt -l internal/` 同时标记 3 个 pre-existing tracked 文件（`blockchain.go`/`difficulty_consensus_test.go`/`pow.go`）——这是**项目范围**的 gofmt diff，与 1B 无关
- 1B 范围建议：`gofmt -w internal/blocktree/settip.go internal/blocktree/settip_test.go`（或 `gofmt -w internal/blocktree/` 全包 4 文件）
- 3 个 pre-existing tracked 文件**不在 1B commit 范围**，不动

> **重要**：gofmt 不影响编译/vet/测试正确性（均已 PASS），仅为风格一致性。**用户可选择是否在 commit 前跑 gofmt**——属偏好，非硬阻断。

### 裁定理由汇总

| 门 | 结果 |
|---|---|
| §1 HARD BASELINE | PASS（HEAD=b2724c8，零新漂移） |
| §2 EXACT WORKTREE STATE | PASS（4 untracked .go 全在 blocktree） |
| §3 1A/1B FILE OWNERSHIP | PASS（边界明确） |
| §4 EXACT DIFF | PASS（仅 doc + 2 字段，无逻辑方法改动） |
| §5 SetTip AUDIT | PASS（12 符号齐 + 7 项 SetTip 不执行清单全清） |
| §6 REORG-OFF | PASS（AddBlock 不调 SetTip + 零生产导入） |
| §7 Consensus Semantics | PASS（*big.Int + parent.CW+Work + tip-hash tie-break） |
| §8 Tests | PASS（vet/build/test/race 全绿） |
| §9 BT-1 Isolation | PASS（1B 零 map 迭代，BT-1=1A pre-existing） |
| §10 Commit Boundary | OPTION B（拆两 commit，证据充分） |
| §11 Commit File Set | 推荐集已明 |
| §12 Excluded Files | 明确 |
| §13 Production Safety | 100% 合规 |

**不通过的唯一场景**：用户要求合并为单一 commit。若用户坚持，则需在 commit message 中显式标注「包含 REORG-1A 主体 + REORG-1B 增量」两阶段语义——但本审计强烈不推荐。

---

## 15. Exact Next Authorized Phase

```text
COMMIT READINESS AUDIT COMPLETE → WAIT FOR EXPLICIT AUTHORIZATION
```

下一步仅允许用户显式授权：

### 选项 A（推荐）：`REORG-1B-COMMIT-EXECUTION-2-COMMITS`
执行两 commit（先 1A 后 1B），保留完整阶段边界。

### 选项 B：`REORG-1B-COMMIT-EXECUTION-1-COMMIT`
合并为单一 commit（4 文件一起），commit message 显式标注「1A + 1B 联合」。

### 不可选

- 不得跳过 `REORG-1B-COMMIT-EXECUTION` 直接进入 `REORG-1C`——commit 是后续阶段的可追溯基础
- 不得修改 1B 已完成代码——本审计全程只读
- 不得触生产节点 / datadir / 推送
- 不得自动执行——等待显式授权

---

## 附：纪律声明

- **STRICT READ-ONLY**：本审计阶段零 `.go` / 配置 / 测试 / 协议 / 文档修改
- **零 git 写**：无 add/commit/push/tag/merge/rebase/amend/checkout/restore/clean/stash/cherry-pick/reset
- **零 staged changes created**：`git diff --cached --stat` 空
- **零 commit created / 零 ref changed / 零 push**
- **零 production touch / 零 SSH / 零部署 / 零 datadir 变更**
- **未自动进入 commit 执行**，等待用户显式授权

**报告产出**：`PHASE-REORG-1B-COMMIT-READINESS-AUDIT-REPORT.md`（本文件）