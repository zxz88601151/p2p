# PHASE REORG-1J-R4C — FINAL CROSS-EVIDENCE / REORG CLOSURE AUDIT

- 阶段性质：**STRICT READ-ONLY / FINAL CROSS-EVIDENCE / CONSENSUS-SAFETY CLOSURE AUDIT**
- 执行日期：2026-09-15
- 基线 HEAD：`e04d678`（未动）
- 生产代码改动：**0**
- 本阶段唯一新增文件：本报告

---

## 1. VERDICT

```text
REORG-1J CONDITIONALLY CLOSED
R5 READY WITH EXPLICIT DEFERRED ITEMS
```

**判定说明**

| 维度 | 结论 |
|---|---|
| Consensus selection / higher-work reorg | ✅ 已证明（R4B 三档 + G09 R1/R2） |
| P2P trigger / dual-process convergence | ✅ 已证明（真实 OS 进程 + 真实 TCP） |
| Reorg execution / TIP commit semantics | ✅ 已证明（R3 + R4A + R4B 一致） |
| Legacy immutability / canonical height replacement | ✅ 已证明（R4A 存储层 + R4B 生产路径） |
| Crash recovery / restart reconstruction | ✅ 已证明（R3 1,818 + R4A 18,247 + R4B 重启×2） |
| Storage physical/logical separation | ✅ 已证明 |
| Historical compatibility / regression compatibility | ✅ 已证明（G08 复跑 PASS、G09 R1/R2 复跑 PASS、cmd/node 全量 ok 556.554s） |
| Production diff | ✅ R4B 为 0，本阶段亦为 0 |
| **证据归档完整性** | ⚠️ **DOC-2：8 份阶段报告中 5 份正文缺失/错位**（见 §11） |

**为何是 B 而不是 A**：REORG-1J 的**技术闭环**（共识 / 存储 / 崩溃恢复 / P2P 收敛）已经完成且无任何 P0/P1 未决项，R5 不被阻塞。但 REORG-1J 自身的**交付物集合**存在归档缺陷（DOC-2）——5 份以「阶段报告」命名的文件正文实际是下一阶段的 prompt。因此判定为 **CONDITIONALLY CLOSED**，R5 的第一动作必须包含该归档校正。

**为何不是 C**：§9 的 14 项门禁条件逐条成立，无一项失败；不存在共识 / 存储 / 崩溃恢复层面的未决矛盾，也不存在未被覆盖的新生产写入路径。

---

## 2. Executive Summary

1. **REORG-1J 的技术目标已达成**：从「legacy 前缀不可变导致首次 reorg 被拒绝」(GAP-1H-A，P0) 出发，经 G08 解除存储层 Gate2 → R3 证明提交边界 → R4A 证明 legacy-internal 几何 → R4B 证明真实双进程生产路径，证据链**完整且自洽**。GAP-1H-A = CLOSED。
2. **R4B 未引入任何新的持久化提交边界**：R4B 实测走通了两条写入路径（P-A / P-B），二者的帧几何与崩溃状态类**逐一落入 R3 的两个 variant 与 R4A 的几何覆盖**（见 §4.3 逐字节账目）。
3. **首次 v2 激活机制是架构约束而非缺陷**：`v2Mode` 门控使「从未 reorg 的链永远保持 legacy」，这是 legacy 逐字节兼容（SP-3/SP-3b）的直接后果。R4B 证明首次 reorg = v2 激活点在生产路径正确完成（见 §5）。
4. **BT-1 最终分类 = TEST CONSTRUCTION DEFECT**，与生产 `rebuildTree` 无关，不阻塞 R5，但必须显式登记（见 §8）。
5. **新发现 DOC-2（P2）**：阶段报告文件命名与正文系统性错位，R5 提交整理前必须处理（见 §11）。

---

## 3. Evidence Matrix（§1）

| Domain | Evidence | Status | Blocker? |
|---|---|---|---|
| Consensus selection | `ShouldReorg`（blocktree）= CompareWork；严格大 → reorg；相等 → tip 哈希大端较大者胜（确定性）。R4B 三档 `newWork > oldWork` 严格成立，全部采纳高 work 分支 | ✅ PROVEN | NO |
| Higher-work fork selection | R4B L=2/8/64：old/new work = 131072/196608、524288/589824、4194304/4259840；`TestBranchLowerWorkForkIsAcceptedButNotCanonical` PASS | ✅ PROVEN | NO |
| P2P trigger | 真实 OS 进程 + 真实 TCP（`-seed`）；A 全程不出块（断言无 `MINING_BLOCK_ACCEPTED`），高度增长只能来自 P2P；双方 `peers > 0` | ✅ PROVEN | NO |
| Reorg execution | 生产路径 `addBlock` Case2 → `tree.AddBlock` → `validateForkBlock` → `ShouldReorg` → `SaveBlockDetached` → `executeReorg` → `CommitReorg` | ✅ PROVEN | NO |
| Legacy immutability | R4A：L=2/3/8/64 legacy 字节零改写；R4B：9 次 `bytes.HasPrefix` 检查 + `LegacyRecordCount()` 恒 L + 物理记录 index L-1 仍解码为 X | ✅ PROVEN | NO |
| Canonical height replacement | R4B：`GetBlockByHeight(L-1).Hash == Y`（v2）而物理记录 index L-1 == X（legacy）；`IsCanonical(X)=false`、`HasBlock(X)=true` | ✅ PROVEN | NO |
| TIP commit semantics | R3 §6 + R4A：提交点 = `appendFrames` 内唯一 `file.Sync()`；提交记录 = TIP 帧且恒末位；delta 内 TIP 数恒 = 1 | ✅ PROVEN | NO |
| Crash recovery | R3：1,818 字节级崩溃点 + 5 次真实进程 kill/restart + 40 次重复；R4A：18,247 字节级崩溃点，每格跃变恰好 1 次且位于 `len(delta)` | ✅ PROVEN | NO |
| Restart reconstruction | R4A：I10 225 次重启 signature 全一致；R4B：重启 ×2 轮，A/B 高度与 tip 全部复现停机前签名，`recovery=REBUILD` | ✅ PROVEN | NO |
| Dual-process convergence | R4B：L=2/8/64 三档，双方高度与 tip 收敛一致 | ✅ PROVEN | NO |
| Cross-node canonical equality | R4B：逐高度（height/hash/prev/bits/work/cumWork）比对 3 / 9 / 65 项 **全等** | ✅ PROVEN | NO |
| Storage physical/logical separation | 1J-G08：`setCanonicalFrom` Gate2 已移除，canonical 由 TIP + 哈希链决定；`TestG08_LegacyImmutableAndTruncateGuard` 本阶段复跑 PASS | ✅ PROVEN | NO |
| Historical compatibility | 纯 legacy 链无 TIP 帧仍可正确加载（`setCanonicalFromLegacyPrefix`）；R4B 中 B 节点全程 100% legacy 且 canonical 正确 | ✅ PROVEN | NO |
| Existing regression compatibility | `go test ./cmd/node/...` **ok 556.554s**；G08 三项 PASS；G09 R1 `TestBranchLegacyPrefixReorgConvergesToLongerBranch` PASS；G09 R2 `TestRealProcessForkReorgConvergesByHash` PASS；1H 基线 `TestRealProcessPairConvergesToSameTip` PASS | ✅ PROVEN | NO |

### 3.1 八阶段审查覆盖确认（§1 要求同时审查 1G / 1H / 1I / G08 / G09 / R3 / R4A / R4B）

| 阶段 | 交付状态 | 本审计中的证据来源 | Blocker? |
|---|---|---|---|
| **REORG-1G** | DELIVERED（PASS W/ GAPS） | GAP-1H-A（P0）立项源；已由 R4A（存储层）+ R4B（真实双进程路径）证闭 | NO |
| **REORG-1H** | DELIVERED（PASS W/ LIMITATIONS） | 基线 `TestRealProcessPairConvergesToSameTip` 本阶段复跑 PASS（22.82 s）；P2P 分支投递语义未再改动 | NO |
| **REORG-1I** | **NOT DELIVERED（无该阶段产物）** | 其设计决策已固化在 `settip.go:26`（永不因深度拒绝更高 work 链）；实现本身 DEFERRED，且 HARD SCOPE LOCK 禁止本阶段提前实现 | NO |
| **REORG-1J-G08** | DELIVERED（契约例外，工作区未提交） | `TestG08_*` ×3 本阶段复跑 PASS；Gate2 已移除，canonical 由 TIP + 哈希链决定 | NO |
| **REORG-1J-G09** | DELIVERED（tripwire 转正，工作区未提交） | G09 R1 `TestBranchLegacyPrefixReorgConvergesToLongerBranch` PASS；R2 `TestRealProcessForkReorgConvergesByHash` PASS（37.85 s） | NO |
| **REORG-1J-R3** | DELIVERED（PASS） | 1,818 字节级崩溃点 + 5 次真实进程 kill/restart + 40 次重复；variant A/B 两种 delta 形状 | NO |
| **REORG-1J-R4A** | DELIVERED（PASS，生产 diff = 0） | 18,247 字节级崩溃点；L∈{2,3,8,64} legacy-internal 几何；legacy 字节零改写 | NO |
| **REORG-1J-R4B** | DELIVERED（PASS，GAP-1H-A = CLOSED） | 真实双进程 + 真实 TCP；L=2/8/64，f=L-2；重启 ×2 轮签名一致 | NO |

**1I 的显式判定**：§1 将 1I 列入审查清单，但 **1I 从未作为阶段执行过，仓库中无对应报告**。其唯一落地物是 `settip.go:26` 的冻结决策（**永不因深度拒绝更高 work 链**），已由本审计复核。该决策与 R3/R4A/R4B 的「更高 work 必被采纳」结论**一致无矛盾**；1I 的完整实现（finality / MaxReorgDepth）属显式 DEFERRED，**不构成 R5 阻塞项**，且本阶段被 HARD SCOPE LOCK 明令禁止提前实现。

---

## 4. R3 ↔ R4A ↔ R4B Consistency Audit（§2）

### 4.1 逻辑链逐段核对

```text
R3    : crash boundary / committed boundary
        （legacyLen=3，fork@2 = legacy-boundary，1,818 字节崩溃点 + 5 次进程 kill）
   ↓  结论：TIP 是唯一 canonical commit boundary
R4A   : canonical recovery across legacy/v2 physical boundary
        （L∈{0,1,2,3,8,64} × {internal,boundary,v2-region}，18,247 字节崩溃点）
   ↓  结论：legacy-internal 亦可恢复，legacy 字节零改写
R4B   : real production P2P reorg
        （L=2/8/64，f=L-2，真实双进程 + 真实 TCP）
   ↓  结论：生产路径可完成首次 reorg 并收敛
restart convergence（R4B 重启 ×2 轮签名一致）
```

**是否存在矛盾：否。** 三段结论在「TIP = 唯一提交点」「legacy 字节不可变」「canonical 由哈希链决定」三条公理上完全一致。

### 4.2 R3 的 crash model 是否覆盖 R4A 的 commit boundary

**是，且方向相反互补**：

| | R3 | R4A |
|---|---|---|
| 崩溃注入 | 字节级 torn-tail 穷举 + 进程级 kill | 字节级 torn-tail 穷举（`j ∈ [0, len(delta)]`） |
| 几何 | legacyLen=3，fork@2（**legacy-boundary**），分叉块 h3..6，**v2 从不占 legacy 槽位** | L∈{0,1,2,3,8,64}，fork ∈ {internal, boundary, v2-region}，**v2 占 legacy 槽位** |
| variant | A-all-detached / B-new-tip-block | k=4，n=4/5/6 |
| delta 形状 | 780 B（UNDO×4 + TIP）/ 1036 B（UNDO×4 + BLOCK + TIP） | 780 / 945 / 1110 B（UNDO×n + TIP） |

⇒ R3 覆盖了「delta 内含 BLOCK 帧」的形状；R4A 覆盖了「delta 内无 BLOCK 帧（块已 detached 预写）」的形状，并把几何维扩展到 legacy 长度与 fork 位置。**两者互补，无缺口，无矛盾。**

### 4.3 R4B 是否进入 R4A 未覆盖的新 production write path —— **否（逐字节账目证明）**

帧尺寸（本审计从实测文件长度反推并经三方交叉验证）：

```text
legacy 记录      = 4 + 176 = 180 B
v2 BLOCK 帧      = 80 + 176 = 256 B
v2 UNDO  帧      = 80 +  85 = 165 B   （coinbase-only undo 载荷恒定 85 B，与 R4A「k=165」观察一致）
v2 TIP   帧      = 80 +  40 = 120 B
```

交叉验证（纯 legacy 的 B 节点）：L=2 → 540 = 3×180；L=8 → 1620 = 9×180；L=64 → 11700 = 65×180 ✅

A 节点 delta 分解（实测）：

| 用例 | A 文件长度 | legacy 前缀 | delta | 分解 | 路径 |
|---|---|---|---|---|---|
| L=2 | 1442 | 360 | **1082** | 256 + (165+120) + (165+256+120) | **P-B** |
| L=8 | 2402 | 1440 | **962** | (256+256) + (165+165+120) | **P-A** |
| L=64 | 12482 | 11520 | **962** | (256+256) + (165+165+120) | **P-A** |
| Path test (L=8) | 2522 | 1440 | **1082** | 256 + (165+120) + (165+256+120) | **P-B** |

两条路径：

```text
P-A（单次 CommitReorg 提交多枚 detached 块）
  SaveBlockDetached(Y) → BLOCK 256
  SaveBlockDetached(Z) → BLOCK 256
  CommitReorg          → UNDO 165 + UNDO 165 + TIP 120      （一次 fsync）
  ⇒ delta = 962

P-B（先 reorg 到 Y，再沿新 canonical 追加 Z）
  SaveBlockDetached(Y)      → BLOCK 256
  CommitReorg(→Y)          → UNDO 165 + TIP 120              （一次 fsync）
  AppendCanonicalBlock(Z)   → UNDO 165 + BLOCK 256 + TIP 120  （一次 fsync）
  ⇒ delta = 1082
```

两条路径的选择由**确定性 tie-break**（同工作量时 tip 哈希大端较大者胜）决定，因此对给定数据可复现；R4B 四轮运行中两条均被观测到（2× P-A、2× P-B）。

**覆盖映射**：

| R4B 写入状态 | 对应已被穷举的状态 | 覆盖来源 |
|---|---|---|
| detached BLOCK 已落盘、尚未提交 | R3 **C1/C2/C3**（base 镜像，显式探测） | R3 |
| P-A 提交 delta（UNDO×n + TIP，一次 fsync） | R3 **variant A** delta / **R4A** 全矩阵 delta | R3 + R4A |
| P-B 第一次提交 delta（UNDO + TIP） | 同上（n=1） | R3 + R4A |
| P-B 第二次提交 delta（UNDO + BLOCK + TIP，一次 fsync） | R3 **variant B** delta（UNDO×n + BLOCK + TIP，一次 fsync），n=1 为其特例 | R3 |
| 全部提交后重启 | C5/C6 + I9/I10 | R3 + R4A + R4B |

**结论：R4B 未引入任何新的持久化提交边界类。** 其全部可观测崩溃状态（partial delta / 完整非 TIP 帧无 TIP / partial TIP / 完整提交）均已由 R3 与 R4A 的字节穷举覆盖。

> 已复核 `AppendCanonicalBlock`（`v2api.go:264-296`）：`appendFrames(undoFrame, blockFrame, tipFrame)` **单次 fsync**，非两阶段提交 —— 因此不存在「两个 fsync 组之间的中间态」这一额外状态类。

---

## 5. First-v2 Activation Audit（§4）

```text
pure legacy datadir → extendChain → V2Mode=false → legacy SaveBlock
        → P2P competing fork → SaveBlockDetached → first v2 frame → CommitReorg
```

**事实确认**（本审计代码复核）：

- `extendChain` 门控 `blockchain.go:512`：`if v2s, ok := bc.store.(reorgStore); ok && v2s.V2Mode()` → v2；否则 legacy `SaveBlock`。
- `v2Mode` 仅在以下点置真：`loadLog` 见到 v2 帧（`v2.go:399`）、`SaveBlockDetached`（`v2api.go:165`）、`SaveBlockWithUndo`（:203）、`PutUndo`（:235）、`commitTipAfterAppend`（:68）。
- `v2Mode` 的语义在 `v2.go:111-114` 被明确定义为**物理存储模式标志**，**不是 canonical 权威**。

**判定：ARCHITECTURAL CONSTRAINT（非 defect）**

| 判据 | 结论 |
|---|---|
| 是否符合当前设计 | ✅ 是。该门控是 SP-3/SP-3b「legacy 写入语义逐字节不变」硬约束的直接后果：若 canonical 追加默认写 v2，legacy 格式兼容无法保证 |
| 是否属于 defect | ❌ 否。没有产生错误状态；R4B 证明首次 reorg = v2 激活点在生产路径正确完成（三档 L 全部收敛、legacy 字节零改写、重启签名一致） |
| 是否属于 architectural constraint | ✅ 是。代价 = 「从未 reorg 的链永久保持 legacy」；收益 = legacy 格式零迁移兼容 |
| 是否必须在 R5 前修改 | ❌ 否 |

**登记为 P3 OBSERVATION，并附两条运维提示**（不修改代码）：

1. 生产首次 reorg 是一条**冷路径**：.123 生产节点（height≈1302）至今 0 次 reorg、0 枚 v2 帧，该路径**仅有受控测试验证，无生产验证**。若未来生产发生首次 reorg，应在可观测条件下进行并留存 blocks.dat 快照。
2. `v2Mode` 一旦置真**不可逆**（无降级回 legacy 的路径）。这是设计取舍，不是缺陷，但值得在运维文档中明确。

---

## 6. Real Reorg Proof Audit（§5）

R4B 对假阳性的排除：

| 假阳性类型 | 排除方式 | 证据 |
|---|---|---|
| height-only false positive | 额外断言「文件变长 + fork 高度处 canonical 由 X **翻转**为 Y」，而非只看高度 | `TestR4B_ProductionReorgPathTraversed`：1440 → 2522 B，fork@7 X→Y |
| same-tip false positive | 前提断言 `X.Hash != Y.Hash`（真实竞争块），且 A 的 old tip ≠ B 的 competing tip | R4B §5 三张表 |
| single-runtime simulation | 两个**操作系统进程**（`exec.Command(bin, -datadir -listen -rpc [-seed])`），各自 `node.lock` | `startRealNode` |
| mocked P2P | 无 mock；真实 TCP `-seed`；双方 `peers > 0` 断言 | R4B 阶段 1 |
| direct CommitReorg invocation | 测试**从不**调用 `CommitReorg`；只能由网络收到的竞争块经 `AddBlock` 触发 | R4B 测试源码 |
| artificial storage mutation | 历史全部由真实二进制产出（`mineChainOffline` / `extendChainOffline`），共同前缀由**同一份历史截断**得到 | R4B 阶段 0 |
| test-only reorg branch | 无；本阶段 production diff = 0 | §12 mtime 硬证据 |

**确认链路成立**：

```text
real process A  ↕  real TCP P2P  ↕  real process B
        → higher-work fork → production reorg → canonical convergence
```

---

## 7. Crash / Recovery Boundary Audit（§6）

**未重跑 R4A 的 18,247 字节扫描**（符合 §6 要求）。只做 cross-reference：

| 问题 | 结论 |
|---|---|
| R3 是否已穷举相关 crash boundary | ✅ 1,818 字节级崩溃点 + 5 次真实进程 kill/restart + 40 次重复；variant A / B 两种 delta 形状均已穷举 |
| R4A 是否覆盖 canonical commit boundary | ✅ 15 适用格 × `j ∈ [0, len(delta)]`，共 13,860 点；外加 VariantB 3,606 点、L0 探针 781 点；每格跃变恰好 1 次 |
| R4B 是否引入新的 persistent commit boundary | ❌ **否**。逐字节账目见 §4.3：两条写入路径的帧几何与崩溃状态类逐一落入 R3 variant A/B 与 R4A 覆盖 |
| R4B 重启是否引入新恢复路径 | ❌ 否。`recovery=REBUILD`（最近 TIP 候选有效即被采纳），三档一致，无 ROLLBACK |

**结论：`R4B introduces no new production write path` —— 已给出生产路径证明（§4.3）。**

---

## 8. BT-1 Final Classification（§7）

```text
BT-1 = [TEST CONSTRUCTION DEFECT]
```

**证明 1 —— 生产 `rebuildTree` 顺序确定**

```go
// internal/blockchain/blockchain.go:138
for i := range bc.blocks {   // bc.blocks 是 []*block.Block，严格按高度 0→h 排列
    _ = bc.tree.AddBlock(...)  // 父节点必已存在
}
```
切片按索引遍历 ⇒ 父必先于子 ⇒ 恒满足 `AddBlock` 的「父先存在」前提。

**证明 2 —— 测试 map 迭代非确定**

```go
// internal/blocktree/blocktree_test.go:229-233
var flat []hdr
for _, n := range orig.nodes {   // orig.nodes 是 map[[32]byte]*BlockNode
    flat = append(flat, hdr{n.Hash, n.ParentHash, n.Height, n.Bits, n.Timestamp})
}
for _, hd := range flat {
    if _, err := rebuilt.AddBlock(...); err != nil {
        t.Fatalf("rebuild AddBlock error: %v", err)   // :241
    }
}
```
Go 的 map 迭代顺序**随机** ⇒ 子节点可能先于父节点被喂给 `AddBlock` ⇒ 随机失败。

**证明 3 —— 实测波动**

本审计复测：`go test -run TestI_RestartLikeReconstruction -count=8 ./internal/blocktree/` → **5 FAIL / 8**。
历史序列：2/8 → 4/8 → 7/8 → 5/8（持续波动，符合 map 迭代随机性，不符合确定性缺陷特征）。

**是否阻塞 R5：否。** 前提：① 显式登记为既有 limitation；② 任何「全量绿灯」声明必须显式排除 `internal/blocktree` 的该用例；③ CI 门禁不得把该包作为阻塞项（否则会随机红）。

**不得修复**（本阶段与 HARD SCOPE LOCK 均禁止）。

---

## 9. Consensus / Storage / P2P Safety Summary（§5–§6 支撑）

### Consensus Safety
- fork-choice 规则在 1H 之后**未再改动**：CumulativeWork + `ShouldReorg` + 确定性 tie-break。
- `settip.go:26` 固化：**永不因深度拒绝更高 work 链**（1I 设计决策，实现 DEFERRED）。
- 回滚走「从 common ancestor 全量 replay」（`blockchain.go:688` `_ = disconnectPath`），**不依赖 legacy 区块的 UNDO**；I1 对 legacy 豁免 UNDO（`v2.go:607-619`）。

### Storage Safety
- legacy **字节**永不变；legacy **槽位**可被 v2 逻辑占据（1J-G08）。
- 被取代块保留为物理历史记录（逻辑墓碑），`HasBlock(X)=true` 且 `IsCanonical(X)=false`。
- 物理截断只修「未提交尾部」；跨已提交 legacy 的 truncate 被拒（G08 R1 复跑 PASS）。

### P2P Safety
- by-hash 祖先拉取、孤儿仅内存、级联零递归、非法块 `endSync`（1H 已交付）。
- 低 work 分叉被接受但不成 canonical（`TestBranchLowerWorkForkIsAcceptedButNotCanonical` PASS）。

---

## 10. Legacy Immutability & Restart Convergence（§3 / §10）

### 10.1 §3 七项子断言（R4B 全部以代码断言固化）

| 子断言 | 实现 | 结果 |
|---|---|---|
| legacy physical prefix 不变 | `bytes.HasPrefix(after, histA)` × 3 检查点 × 3 档 | ✅ 9/9 |
| legacy record count 不变 | `r4bLegacyCount(after)` + `store.LegacyRecordCount()` | ✅ 恒为 L |
| v2 frame 可逻辑占据 legacy height | `GetBlockByHeight(L-1).Hash == Y.Hash` | ✅ |
| old block 保留为 physical historical record | `store.HasBlock(X.Hash) == true` | ✅ |
| old block 不再 canonical | `store.IsCanonical(X.Hash) == false` | ✅ |
| new block 成为 canonical | `GetBlockByHeight(L-1) == Y`、`GetBlockByHeight(L) == Z` | ✅ |
| restart 后该关系保持 | 重启 ×2 轮后重新执行全部断言 | ✅ |

### 10.2 明确回答 §3 的核心问题

> **是否已经真实证明「v2 logical canonical block 可以替换 legacy physical slot，而不改写 legacy bytes」？**

**是，已真实证明。** 两层证据：

1. **存储层**（R4A）：L=2/3/8/64，v2 占据槽位 h = L-1，18,247 字节级崩溃点全部通过，legacy 字节零改写。
2. **真实生产路径**（R4B）：L=2/8/64，两个真实 OS 进程经真实 TCP 完成竞争分叉 reorg；A 的 legacy 物理前缀 9 次逐字节比对不变；legacy 条数不变；同高度 legacy 记录（X，非 canonical）与 v2 块（Y，canonical）并存；重启 ×2 轮后关系完全保持。

### 10.3 Restart Convergence

| L | 重启轮次 | 重启后 (height, tip) A / B | 与停机前一致 |
|---|---|---|---|
| 2 | 1, 2 | (2, `0000914bfa…`) / (2, `0000914bfa…`) | ✅ |
| 8 | 1, 2 | (8, `0000dce51c…`) / (8, `0000dce51c…`) | ✅ |
| 64 | 1, 2 | (64, `00004405aa…`) / (64, `00004405aa…`) | ✅ |

`recovery=REBUILD` 三档一致 ⇒ 无一次重启需要回退到更早 TIP。

---

## 11. Remaining Issues Classification（§8）

| Item | 分类 | 是否必须在 R5 前处理 | 说明 |
|---|---|---|---|
| **DOC-2（本阶段新发现）**：阶段报告文件命名与正文系统性错位 | **P2 NON-BLOCKING** | **是（R5 第一动作）** | 8 份报告中仅 2 份正文正确。详见 §11.1 |
| **BT-1** | **P2 NON-BLOCKING** | 否（但必须在 R5 交付说明中显式登记） | TEST CONSTRUCTION DEFECT；修复需另授权 |
| **OBS-1J-R3-A**：`CommitReorg` 无法一次提交多枚全新块 | **P3 OBSERVATION** | 否 | 生产不可达（`AddBlock` 恒先 `SaveBlockDetached` ⇒ `actualNewBlocks` 恒空），R4B 再次确认；fail-safe |
| **R4A-OBS-1**：REPAIR 只截断撕裂残片 | **P3 OBSERVATION** | 否 | 语义澄清非缺陷；「已提交边界」= 最后一个完整帧末尾 |
| **first-v2 activation** | **P3 OBSERVATION** | 否 | 架构约束非缺陷；附 2 条运维提示（§5） |
| **finality / MaxReorgDepth（1I）** | **DEFERRED** | 否 | 设计已固化（`settip.go:26` 永不因深度拒更高 work）；实现未交付，且本阶段禁止提前实现 |
| **orphan pool（B5）** | **DEFERRED** | 否 | 1H 只做最小 `1H-BRIDGE` pending；完整实现属后续阶段 |
| **fork observability（B8）** | **DEFERRED** | 否 | `/status` 无 fork 观测字段；属后续阶段 |
| **F-3..F-7**（mempool 重复 validation / DisconnectBlocks 静默丢弃 / AddBlockWithResult TOCTOU / pre-reorg height / extendChain 吞 tree 错误） | **P2/P3 OPEN** | 否 | 既有登记项，均不在 reorg 正确性主链路上 |
| **SP-3 / SP-3b**（legacy 语义由下游保证） | **P2 CLOSED (v2 路径)** | 否 | |
| **DOC-1**（`utxo/undo.go:413-424` 注释漏 `PreSetItemsCount`） | **P3 OBSERVATION** | 否 | |
| **Deferred 集合**（D2/D3、P3-a/b、SYNC-001/002、P2P-001、OBS-001..003、B1b、R5、G-11..G-15、物理 compaction/GC、`internal/config` 死包） | **DEFERRED** | 否 | |

### 11.1 DOC-2 明细（本阶段新发现）

| 文件名 | 实际内容 | 期望内容 |
|---|---|---|
| `PHASE-REORG-1G-FINAL-GAP-AUDIT-REPORT.md` | 1H 阶段 prompt | 1G gap audit 报告 |
| `PHASE-REORG-1J-G08-CONTRACT-EXCEPTION-FINAL-REPORT.md` | G09 阶段 prompt | G08 契约例外报告 |
| `PHASE-REORG-1J-G09-TRIPWIRE-CONVERSION-FINAL-REPORT.md` | 7 行空壳（仅 skill 名） | G09 tripwire 转正报告 |
| `PHASE-REORG-1J-R3-LEGACY-PREFIX-CRASH-MATRIX-FINAL-REPORT.md` | R4A PRE-GATE prompt | R3 崩溃矩阵报告 |
| `PHASE-REORG-1J-R4A-PRE-GATE-FINAL-REPORT.md` | R4A IMPLEMENTATION-1 prompt | R4A PRE-GATE 报告 |
| `docs/PHASE-REORG-1J-R4A-FINAL-REPORT.md` | R4B PRE-GATE prompt | R4A 最终报告 |
| `PHASE-REORG-1H-FINAL-REPORT.md` | ✅ 真实 1H 报告 | — |
| `PHASE-REORG-1J-R4B-PRE-GATE-FINAL-REPORT.md` | ✅ 真实 R4B 报告 | — |

影响：**不触发任何共识 / 存储 / 崩溃恢复矛盾**（本审计的全部结论均从**源码 + 测试 + 实测输出**重新取得，未依赖这些报告正文）。但 R5 若直接提交，会把下一阶段的 prompt 当作上一阶段的报告纳入仓库，属于必须整理的交付缺陷。

---

## 12. R5 Readiness Gate（§9）

| # | 条件 | 结论 |
|---|---|---|
| 1 | no P0 | ✅ GAP-1H-A 已 CLOSED（R4B）；无其他 P0 |
| 2 | no P1 | ✅ 无 P1 未决项 |
| 3 | no unresolved consensus contradiction | ✅ G08/G09/1H/R3/R4A/R4B 在 fork-choice 上完全一致 |
| 4 | no unresolved storage contradiction | ✅ legacy 字节不可变 + 逻辑槽位可替换，三层证据一致 |
| 5 | no unresolved crash-recovery contradiction | ✅ TIP 唯一提交点，1,818 + 18,247 点一致 |
| 6 | no new production write path uncovered | ✅ §4.3 逐字节账目证明 |
| 7 | R3/R4A/R4B evidence consistent | ✅ §4 |
| 8 | G08/G09 acceptance remains intact | ✅ 本阶段复跑：G08 三项 PASS；G09 R1 PASS；G09 R2 PASS（37.85s） |
| 9 | legacy immutability proven | ✅ §10 |
| 10 | higher-work reorg proven | ✅ §3 |
| 11 | dual-process convergence proven | ✅ §3 |
| 12 | restart convergence proven | ✅ §10.3 |
| 13 | production diff remains zero for R4B | ✅ §13 |
| 14 | all remaining issues explicitly classified | ✅ §11 |

```text
R5 READY
```

---

## 13. Production Diff Verification（本阶段 + R4B）

| 检查 | 结果 |
|---|---|
| `git log --oneline -1` | `e04d678`（未动） |
| `git status --porcelain \| grep -c "^ M"` | **11**（既有 9 个代码改动 + 2 个长期 dirty 文档），本阶段新增 0 |
| `git status --porcelain \| grep -c "^??"` | 43（含本阶段新增 1 个报告） |
| 生产文件 mtime | `blocktree.go` 01:07、`blockchain.go` 08:43、`main.go` 08:49、`service.go` 08:50、`p2p/node.go` 08:46、`v2.go` 10:10、`file.go` 10:11、`v2api.go` 10:12 —— **全部早于 R4A/R4B 阶段产物** |
| `go build -buildvcs=false ./...` | BUILD_OK |
| `go vet ./cmd/node/ ./internal/storage/` | VET_OK |

---

## 14. Explicit Non-Actions（§13）

本阶段**未做**以下任何一件事：

- ❌ 未修改任何 production code
- ❌ 未修改既有测试断言
- ❌ 未新增 production hook / test-only production branch
- ❌ 未修改 G08 / G09 / R3 / R4A / R4B 已验证行为
- ❌ 未修复 BT-1
- ❌ 未修改 R3-A（OBS-1J-R3-A）
- ❌ 未修改 R4A-OBS-1
- ❌ 未提前实现 finality / MaxReorgDepth / orphan pool / fork observability
- ❌ 未重跑 R4A 的 18,247 字节崩溃扫描
- ❌ 未 commit / push / tag / merge / rebase / amend / squash
- ❌ 未 deploy（Windows 或 Linux 生产）
- ❌ 未进行 Windows mining

### 14.1 报告章节 vs §11 十四项要求对照

| # | §11 要求 | 本报告位置 |
|---|---|---|
| 1 | VERDICT | §1 |
| 2 | Executive Summary | §2 |
| 3 | Evidence Matrix | §3（14 域）+ §3.1（八阶段覆盖） |
| 4 | R3 ↔ R4A ↔ R4B Consistency | §4（4.1 逻辑链 / 4.2 crash model / 4.3 逐字节账目） |
| 5 | Consensus Safety | §9 Consensus Safety |
| 6 | Storage Safety | §9 Storage Safety |
| 7 | Crash Recovery Safety | §7 |
| 8 | P2P / Dual Process Safety | §9 P2P Safety + §6 Real Reorg Proof Audit |
| 9 | Legacy Immutability | §10.1（七项子断言）+ §10.2（核心问题明确回答） |
| 10 | Restart Convergence | §10.3 |
| 11 | Remaining Issues Classification | §11 + §11.1（DOC-2 明细） |
| 12 | R5 Readiness Gate | §12（14/14 → R5 READY） |
| 13 | Explicit Non-Actions | §14 |
| 14 | Final Decision | §15 |

---

## 15. Final Decision（§14）

```text
REORG-1J CONDITIONALLY CLOSED
R5 READY WITH EXPLICIT DEFERRED ITEMS
```

**技术闭环（可冻结 production implementation）**：共识选择、更高 work reorg、P2P 触发、reorg 执行、legacy 不可变、canonical 高度替换、TIP 提交语义、崩溃恢复、重启重建、双进程收敛、跨节点 canonical 相等、物理/逻辑分离、历史兼容、回归兼容 —— **14/14 全部 PROVEN，无 P0/P1，无矛盾，无新写入路径。**

**条件（CONDITIONALLY 的唯一来源）**：DOC-2 证据归档缺陷（P2）。R5 的第一动作必须完成报告归档校正（重命名 / 补正文），否则提交物将包含错位文件。

**下一步授权（唯一推荐）**：

```text
REORG-1J-R5 — COMMIT ORGANIZATION
范围：① DOC-2 报告归档校正；② 未提交改动的分组与提交（必须排除长期 dirty 文件：
docs/DETERMINISTIC-SERIALIZATION-SPEC.md、docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md、
run-a/、run-b/、verifier/）；③ 提交说明中显式登记 BT-1 与全部 DEFERRED 项。
```

---

## 16. Required Closing Block

```text
PRODUCTION CODE CHANGES:
0

COMMIT:
NO

PUSH:
NO

TAG:
NO

DEPLOY:
NO

WINDOWS MINING:
NO

NEXT AUTHORIZATION:
REORG-1J-R5 — COMMIT ORGANIZATION
```

---

## STOP

本审计到此结束。不进入 R5，不修复任何问题，不提交任何代码，不部署节点，不进行 Windows 挖矿测试。
