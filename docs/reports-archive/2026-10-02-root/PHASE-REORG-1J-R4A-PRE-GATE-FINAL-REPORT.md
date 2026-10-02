# PHASE REORG-1J-R4A — PRE-GATE FINAL REPORT

```text
RECONSTRUCTED FROM EVIDENCE
```

> **This document is a reconstructed archival artifact. It is NOT the original execution report.**
>
> Original execution report was not recoverable from Git history. This reconstruction is based only on surviving evidence.
>
> 原始报告 `PHASE-REORG-1J-R4A-PRE-GATE-FINAL-REPORT.md` 的正文本 REORG-1J-DOC-2 审计时被确认为
> **REORG-1J-R4A IMPLEMENTATION-1 的 prompt 全文**（系统性 off-by-one 覆盖）；
> 原始 8,623 B 字节序列已不可恢复，该 prompt 副本已 rename 归档至 `docs/prompts/REORG-R4A-IMPLEMENTATION-1.prompt.md`，
> 原始字节另存于 `docs/_archive_pre-doc2/repo/PHASE-REORG-1J-R4A-PRE-GATE-FINAL-REPORT.md`。

- 重建阶段：`REORG-1J-DOC-2-REPAIR`
- 重建日期：2026-09-15
- 重建依据：见 §Evidence Sources（全部可定位到文件 + 行号）

---

## 1. VERDICT

```text
READY
```

证据：`.workbuddy/memory/2026-09-15.md:306` —— 章节标题原文
`## 13:15–13:40 · REORG-1J-R4A PRE-GATE（只读审计，VERDICT = READY）`。

---

## 2. Stage Identity

| 字段 | 值 | 证据 |
|---|---|---|
| 阶段名 | REORG-1J-R4A PRE-GATE | `2026-09-15.md:306` |
| 被审计对象 | `REORG-1J-R4A — LEGACY-LENGTH × FORK-POSITION CANONICAL RECOVERY MATRIX` | `2026-09-15.md:308` |
| 阶段性质 | 严格**只读** PRE-GATE（A–H 八项审计 + 13 节报告）；**未进入实现** | `2026-09-15.md:308` |
| 执行时间窗 | **13:15–13:40**（2026-09-15） | `2026-09-15.md:306` |
| 报告产出 | 新增 `p2pchain/PHASE-REORG-1J-R4A-PRE-GATE-FINAL-REPORT.md` | `2026-09-15.md:310` |
| 文件动作 | 修改既有文件 **0 个**，**生产代码 diff = 0** | `2026-09-15.md:310` |
| 基线 | HEAD = `e04d678`（未变），暂存区空 | `2026-09-15.md:310` |

**diff = 0 的复核方式（原文）**：审计后复核 `git status` 与修改前完全一致，HEAD 仍 `e04d678`，暂存区空（`2026-09-15.md:310`）。

---

## 3. Scope

对 R4A 立项做**前置门禁审计**：确认基线、确认既有阶段产物、通读相关存储源码、确认夹具 100% test-only，
并输出矩阵几何、成本基线与 STOP 条件。**不进入实现**。

---

## 4. Audit Evidence（A–D）

证据：`2026-09-15.md:312-315`

| 项 | 内容 |
|---|---|
| **A / B** | HEAD = `e04d678`、暂存区空、1H + 1J + R3 全部未提交（改 9 / 新增 5 + 1 报告）。R3 实测复跑 **9 项全 PASS（20.276 s）**；G08 R1 `TestG08_LegacyImmutableAndTruncateGuard` **PASS（0.13 s）**；G09 R2 两条 tripwire 文件确认存在（`p2p_branch_test.go:593`、`p2p_branch_process_test.go:314`） |
| **C** | 通读 `v2.go`(359–780)、`v2api.go`(全篇)、`file.go`(1–120)、`blockchain.go`(585–777)，并 `git diff` 核对 1J 三处存储改动 |
| **D** | R4A 夹具 **100% test-only**，全部复用既有 `iBlock` / `iChain` / `iReadLog` / `iWriteLog` 与 R3 的 `r3Probe` / `r3Opened` / `r3AssertLegal` / `r3Converge` |

---

## 5. Two Key Findings

证据：`2026-09-15.md:317-319`

### 5.1 R3 覆盖面澄清

R3 `r3ForkAt = legacyLen-1 = 2`、分叉块高度 3..6 ⇒ 实测的是 **legacy-boundary**；
`r3_crash_matrix_test.go:172` 注释「v2 区块占据 legacy 槽位」**与代码事实不符**。
⇒ **深度回卷（legacy-internal）至今零覆盖**，是 R4A 立项首要理由。

> 行号说明：本阶段重建时复核，该场景注释现位于 `internal/storage/r3_crash_matrix_test.go:168`（原文「3) 分叉分支：自 **legacy 高度 2** 分叉 ⇒ v2 区块占据 legacy 槽位（GAP-1H-A 场景）」），
> 与 memory 记录的 `:172` 相差 4 行，指向同一处注释。

### 5.2 深度回卷 = 真实生产可达（不是合成玩具）

1. `settip.go:26` 永不因深度拒绝更高 work 链；
2. `blockchain.go:688-690` `_ = disconnectPath`，回滚走「从 ancestor 全量 replay」⇒ **不依赖 legacy 区块的 UNDO**；
3. `v2.go:607-619` I1 只对 `isV2` 区块要求 UNDO，**legacy 豁免**。

**本阶段重建时复核**：`internal/blocktree/settip.go:26` 原文「MaxReorgDepth（BG-3，已定稿）：本 SetTip **永不**因深度拒绝更高 work 链——『告警 + 限速，永不拒绝』。深度策略属 REORG-1I，此处不实现、不检查。」——与第 1 条一致。

---

## 6. Matrix Conclusion

证据：`2026-09-15.md:321`

| 项 | 值 |
|---|---|
| L 取值 | `{0, 1, 2, 3, 8, 64}` |
| fork position | `{internal, boundary, v2-region}` |
| 组合数 | **15 适用 / 3 N/A** |
| N/A 原因 | L0-internal 无 legacy 记录；L0-boundary 退化为 v2-region；L1-internal 只能替换创世，违反共享创世 + SP-3b |
| 几何 | `k = 4`（受 `tipRingSize = 8` 约束）、`f = L-2 / L-1 / L`、`newTop = oldTop + 1`、`n ≤ 6` ⇒ **成本与 L 解耦** |
| 新增不变量 | **I11**（legacy 槽位被占不改变物理记录）、**I12**（同高度双记录解析确定性） |

**N/A 处理原则（原文）**：N/A **只记原因，不为凑矩阵改生产**。

---

## 7. Cost Baseline（本阶段实测）

证据：`2026-09-15.md:323`

| 项 | 实测 |
|---|---|
| 每崩溃点 | **3.3 ms** |
| 每挖矿块 | **0.16 s** |
| storage 单包总耗时 | ≈ **2.5 min**（含 15 格字节全穷举） |
| 结论 | **无需抽样降级** |

---

## 8. Stop Conditions（9 条）

证据：`2026-09-15.md:325`

已列 **9 条** STOP 条件，原文列举：

1. 双重 canonical
2. legacy 字节被改
3. 非法物理截断
4. signature 不一致
5. 需改生产
6. 需改 G08 · G09
7. delta 内 TIP ≠ 1 或不在末位
8. 只能证明进程活着
9. L=0 构造需改生产则该格降级 N/A

> 原报告逐字文本 **EVIDENCE INSUFFICIENT**；上表为 memory 记录的条目归纳。

---

## 9. Next Step

证据：`2026-09-15.md:327`

**NEXT（不自动执行，需授权）**：`REORG-1J-R4A — IMPLEMENTATION`。

实现顺序：L=0 探针 → legacy-internal（L=2/3/8/64）→ 其余 11 格 → L3 变体 B → Truncate/REPAIR 附加用例 → 回归 → STOP 不提交。
之后可选 `REORG-1J-R4B — LEGACY-INTERNAL REAL-PROCESS CONVERGENCE`。

---

## 10. Non-Actions

- ❌ **未进入实现**（`2026-09-15.md:308`）
- ❌ 修改既有文件 **0 个**，生产代码 diff = **0**（`2026-09-15.md:310`）
- ❌ 无 git 写操作（HEAD / 暂存区复核一致）
- ❌ 未改 G08 · G09（STOP 条件第 6 条）
- ❌ 未为凑矩阵改生产（N/A 只记原因）
- 附：本阶段另做 `MEMORY.md` 从 11.8 KB 精简重写为 v5（合并重复项、补 R3/R4A 真值与 §5/§9 新章节）（`2026-09-15.md:329`）

---

## 11. Stop Condition

原报告 STOP 块逐字文本 **EVIDENCE INSUFFICIENT**。
可证据支撑的事实：PRE-GATE 以 `VERDICT = READY` 结束，并以「NEXT 不自动执行，需授权」收尾（`2026-09-15.md:327`）。

---

## Evidence Sources

| # | 来源 | 精确定位 | 用途 |
|---|---|---|---|
| 1 | `.workbuddy/memory/2026-09-15.md` | 行 **306–329**（章节「13:15–13:40 · REORG-1J-R4A PRE-GATE」全段） | 阶段身份、时间窗、VERDICT、A–D 审计证据、两个关键新发现、矩阵结论、成本基线、9 条 STOP、NEXT、memory 维护 |
| 2 | `internal/blocktree/settip.go` | 行 **26** | 「永不因深度拒绝更高 work 链」的代码层佐证 |
| 3 | `internal/storage/r3_crash_matrix_test.go` | 行 **168** | R3 分叉场景注释（memory 记录为 `:172`，行号漂移见 §5.1） |
| 4 | `.workbuddy/memory/2026-09-15.md` | 行 **282–302**（R3 段） | A/B 项中的「R3 实测复跑 9 项全 PASS」上下文 |
| 5 | `PHASE-REORG-1J-DOC-2-EVIDENCE-ARCHIVE-CLOSURE-AUDIT-REPORT.md` | §3 表（R4A 双行）、§3.1（R4A 十七问）、§4.1 第 17–18 行、§5.1、§7 | 原始报告被 R4A IMPLEMENTATION-1 prompt 覆盖的归档事实与重建授权 |
| 6 | `PHASE-REORG-1J-R4C-FINAL-CLOSURE-AUDIT-REPORT.md` | §3.1 行 **77**（R4A 复核行） | R4A 交付状态与 18,247 崩溃点的下游确认 |
| 7 | `docs/_archive_pre-doc2/repo/PHASE-REORG-1J-R4A-PRE-GATE-FINAL-REPORT.md` | 全文件 8,623 B，SHA-256 `ed7bb754e5db8c3f6c335edeaf2a021db99d9eb3a9e5fb831049250e4aca1785` | 被覆盖后残留正文的不可变快照 |

**文件系统事实（非阶段记录）**：`docs/prompts/REORG-R4A-IMPLEMENTATION-1.prompt.md` mtime = `2026-09-15 13:56`
—— 该时刻为**覆盖本 PRE-GATE 报告路径**的时间，晚于 PRE-GATE 执行窗 13:15–13:40，与 off-by-one 覆盖机制一致。

---

## EVIDENCE INSUFFICIENT 字段清单

| 字段 | 状态 |
|---|---|
| 原报告的 13 节结构与逐节标题 | EVIDENCE INSUFFICIENT（仅知「A–H 八项审计 + 13 节报告」） |
| A–H 八项审计中 E / F / G / H 的内容 | EVIDENCE INSUFFICIENT（memory 仅记录 A/B/C/D） |
| 9 条 STOP 条件的逐字文本 | EVIDENCE INSUFFICIENT |
| 原报告字节大小 | EVIDENCE INSUFFICIENT（仅知残留 prompt 副本 8,623 B） |
| 原报告 STOP 块逐字文本 | EVIDENCE INSUFFICIENT |
| `blockchain.go:585-777` / `v2.go:359-780` 通读的具体发现 | EVIDENCE INSUFFICIENT |

---

## STOP

```text
本文件为 RECONSTRUCTED FROM EVIDENCE 归档产物。
不得作为原始执行报告引用。
```
