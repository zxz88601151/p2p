# PHASE REORG-1J-G09 — TRIPWIRE CONVERSION FINAL REPORT

```text
RECONSTRUCTED FROM EVIDENCE
```

> **This document is a reconstructed archival artifact. It is NOT the original execution report.**
>
> Original execution report was not recoverable from Git history. This reconstruction is based only on surviving evidence.
>
> 本路径在 REORG-1J-DOC-2 审计时的实际内容为一个 **35 字节空壳**（全文仅 `senior‑software‑engineer` + 7 个换行），
> 原始报告正文**从未成功写入**。按 DOC-2 Step 3 规则，该 35 B 原始字节在重建前已完整备份至：
> ① `docs/prompts/REORG-G09-empty-shell.original.txt`（35 B，SHA-256 与本文件重建前一致）；
> ② `docs/_archive_pre-doc2/repo/PHASE-REORG-1J-G09-TRIPWIRE-CONVERSION-FINAL-REPORT.md`（35 B，同 SHA-256）。
> 三者 SHA-256 均为 `25221e1166e0249631ad172c4ff8fbce6f290b34f0cb7b9a1dba69cccc643308`，已逐一验证 PASS。

- 重建阶段：`REORG-1J-DOC-2-REPAIR`
- 重建日期：2026-09-15
- 重建依据：见 §Evidence Sources（全部可定位到文件 + 行号）

---

## 1. VERDICT

```text
DELIVERED（TRIPWIRE CONVERSION）
```

证据：`PHASE-REORG-1J-R4C-FINAL-CLOSURE-AUDIT-REPORT.md` §3.1 行 75 ——
「**REORG-1J-G09** | DELIVERED（tripwire 转正，工作区未提交）| G09 R1 `TestBranchLegacyPrefixReorgConvergesToLongerBranch` PASS；R2 `TestRealProcessForkReorgConvergesByHash` PASS（37.85 s）| NO」。

> 原报告 VERDICT 行原文 **EVIDENCE INSUFFICIENT**（原始正文从未落盘）。

---

## 2. Stage Identity

| 字段 | 值 | 证据 |
|---|---|---|
| 阶段名 | PHASE REORG-1J-G09 — TRIPWIRE CONVERSION | DOC-2 §3 表（G09 行）、§12 canonical filename 表 |
| 执行日期 | 2026-09-15 | `2026-09-15.md` 当日日志 |
| 执行时间窗 | **EVIDENCE INSUFFICIENT** | 见 §EVIDENCE INSUFFICIENT 字段清单 |
| 基线 HEAD | `e04d678` | 阶段链 1G→R4C 全程未发生 commit，`git rev-parse HEAD` 至今仍为 `e04d678`；G09 自身基线记录行未留存 |
| 交付物形态 | 2 条 tripwire（R1 / R2）由「断言缺陷行为」转为「正向验收」 | `cmd/node/p2p_branch_test.go:590`、`cmd/node/p2p_branch_process_test.go:302-314` 注释块 |

---

## 3. Scope

1H 阶段发现的 **GAP-1H-A（P0）** 曾以 2 个 tripwire 固化「缺陷行为」（修复后必失败，应删除/改写而非放宽）。
G09 的任务是把这 2 个 tripwire **转正**为正向 acceptance 用例。

证据链：`.workbuddy/memory/2026-09-15.md:274-276` ——
「已固化 2 个 tripwire（修复后必失败，应删除/改写而非放宽）；测试侧用 `sealV2Mode`（挖一枚低工作量 detached v2 块）绕行，**绕行≠修复**」。

---

## 4. R1 — POSITIVE REORG ACCEPTANCE TRIPWIRE CONVERSION

**位置**：`cmd/node/p2p_branch_test.go`，注释块起始行 **590**（标题 `REORG-1J-G09 · R1 —— POSITIVE REORG ACCEPTANCE TRIPWIRE CONVERSION`），用例 `TestBranchLegacyPrefixReorgConvergesToLongerBranch` 位于行 **629**。

**旧契约（已废止）**（`p2p_branch_test.go:592-598`）：

- 旧用例名：`TestBranchKnownGapLegacyPrefixBlocksFirstReorg`
- 断言的是缺陷行为：legacy 语义下写入的 canonical 块构成不可变历史前缀，`FileBlockStore.setCanonicalFrom` 拒绝让 v2 区块占据高度 `< legacyLen` 的槽位 ⇒ 现实持久化链上的首次 reorg 在存储层被拒绝。
- 旧断言链：B 高度必须恒为 4、链尾必须恒为 `b4`、日志必须出现「legacy 前缀不可变」。

**废止依据**（`p2p_branch_test.go:600-605`）：REORG-1J §1 / §6 / §7 / §11 明确废止「legacy canonical 归属永久不可变化」；canonical ownership 由 **TIP + hash linkage** 决定，与记录物理形态（legacy / v2）解耦；穿越 legacy prefix 的 reorg **必须被允许**。「legacy 前缀不可变」这一拒绝路径已被移除（生产代码中该字符串已不存在）。

**新契约（正向 acceptance test）**（`p2p_branch_test.go:607-611`）：
当 legacy prefix 中存在旧 canonical 块，而更长 / 更优的 V2 branch 通过 hash linkage 成为合法候选时，reorg 必须成功，并最终使 active TIP / canonical height 正确收敛；同时 legacy 的物理不可变性仍必须成立。

**被替换的更强不变量（acceptance chain 1–10）**（`p2p_branch_test.go:613-628` 原文列点）：

1. legacy block 仍然存在（`HasBlockHash`）
2. legacy block physical bytes 未变化（日志前缀逐字节不变）
3. branch block 通过 hash linkage 被识别（`BlockByHash` 可取回）
4. reorg 落点穿越 legacy prefix 不再因 legacy ownership 被拒绝
5. active TIP 更新到正确 branch（== `a5`）
6. canonical height 正确（== 5）
7. detached old branch 不再被视为 canonical（`IsCanonicalHash(b4) == false`）
8. legacy block 仍可通过 hash 查询
9. legacy block 内容与 reorg 前完全一致（`Encode()` 逐字节相等）
10. append-only 存储未发生非法 truncate（文件长度只增不减）

**关键前提**：用例刻意**不做** v2 密封——完全复刻「现实持久化链的首次 reorg」场景；若 `a.store.V2Mode() || b.store.V2Mode()` 为真则 `t.Fatal`（`p2p_branch_test.go:630-633`）。

---

## 5. R2 — REAL-PROCESS FORK REORG CONVERGENCE TRIPWIRE

**位置**：`cmd/node/p2p_branch_process_test.go`，用例 `TestRealProcessForkReorgConvergesByHash` 位于行 **314**，注释块 **300-313**。

**必须分别证明的四段链（不得以「收到 block」冒充「完成 reorg」）**（`p2p_branch_process_test.go:302-309` 原文）：

1. **transport**：双方都在真实 TCP 上发出 by-hash 请求、并被对端响应；
2. **storage acceptance**：对端可按哈希找到该分叉块（不因 legacy-prefix ownership 被拒绝，且日志中无任何 legacy 前缀拒绝痕迹）；
3. **validation + canonical selection**：胜出方的 canonical tip 切换到确定性 tie-break 的胜者（同工作量下 tip 哈希较大者）；
4. **convergence**：双方 canonical tip 相同、高度相同、且均未崩溃、数据未损坏、都能正常 shutdown。

**场景**：A 与 B 拥有共同前缀 3，各自离线挖出高度 4 的竞争区块（a4 / b4，因钱包不同必然不同）。双方高度同为 4、工作量相同 ⇒ 由确定性 tie-break（tip 哈希大端较大者）决定胜者，**与网络到达顺序无关**（`p2p_branch_process_test.go:311-313`）。

---

## 6. Tests & Measured Outputs

| Tripwire | 用例 | 位置 | 结果 | 证据 |
|---|---|---|---|---|
| G09 R1 | `TestBranchLegacyPrefixReorgConvergesToLongerBranch` | `cmd/node/p2p_branch_test.go:629` | **PASS** | R4C §3.1 行 75；`2026-09-15.md:373` |
| G09 R2 | `TestRealProcessForkReorgConvergesByHash` | `cmd/node/p2p_branch_process_test.go:314` | **PASS** | R4C §3.1 行 75 |

**耗时记录（可定位）**：

| 复跑场合 | 数值 | 证据 |
|---|---|---|
| R3 回归闸门 | R2 **41.8 s** | `2026-09-15.md:299` |
| R4C 证据复核 | R2 **37.85 s** | `2026-09-15.md:373`；R4C §3.1 行 75 |

**存在性确认**：R4A PRE-GATE 段记录「G09 R2 两条 tripwire 文件确认存在（`p2p_branch_test.go:593`、`p2p_branch_process_test.go:314`）」（`2026-09-15.md:313`）。
> 行号漂移说明：本阶段重建时复核，`p2p_branch_test.go` 中 G09 R1 注释块现位于行 **590**、函数位于行 **629**；
> memory 记录的 `593` 为 2026-09-15 13:xx 时的旧行号。**两者指向同一用例**，差异源于其后文件被追加内容。

**证据矛盾（诚实记录，不掩盖）**：

| 来源 | 对 G09 R1 的命名 |
|---|---|
| `2026-09-15.md:299` | 记为 `TestG08_LegacyImmutableAndTruncateGuard` |
| `2026-09-15.md:373` / R4C §3.1 行 75 | 记为 `TestBranchLegacyPrefixReorgConvergesToLongerBranch` |

`2026-09-15.md:313` 将 `TestG08_LegacyImmutableAndTruncateGuard` 归入 **G08 R1**。
⇒ 判定：`299` 行的 R1 命名系记录时的笔误；以 `373` / R4C §3.1 行 75（二者互相独立且一致）为准，即 **G09 R1 = `TestBranchLegacyPrefixReorgConvergesToLongerBranch`**。
**该矛盾不影响结论**：两种记法下 R1 均为 PASS。

---

## 7. Non-Actions

- ❌ 未放宽断言（tripwire 转正为**更强**不变量，非放宽）：`p2p_branch_test.go:613-628` 的 10 条 acceptance chain
- ❌ 未修改生产代码（本阶段无生产 diff 记录；见 §EVIDENCE INSUFFICIENT）
- ❌ 未 commit / push（R4C §3.1 行 75：「工作区未提交」）
- ❌ 后续阶段锁定：R4A PRE-GATE STOP 条件与 R4C 14 条硬锁均含「不改 G08·G09」（`2026-09-15.md:325`、`:360`）

---

## 8. Stop Condition

原报告 STOP 块文本 **EVIDENCE INSUFFICIENT**。

可证据支撑的等价事实：G09 在 REORG-1J 阶段链中已结束；其 R2 在 R4B 阶段复跑时「未改动仍 PASS」（`2026-09-15.md:355`），并被 R4C 复核为 DELIVERED。

---

## Evidence Sources

| # | 来源 | 精确定位 | 用途 |
|---|---|---|---|
| 1 | `cmd/node/p2p_branch_test.go` | 行 **588–633**（G09 R1 注释块 + `TestBranchLegacyPrefixReorgConvergesToLongerBranch` 起始） | R1 旧契约 / 废止依据 / 新契约 / 10 条 acceptance chain / 关键前提 |
| 2 | `cmd/node/p2p_branch_process_test.go` | 行 **300–314**（R2 注释块 + `TestRealProcessForkReorgConvergesByHash`） | R2 四段链、场景、tie-break 语义 |
| 3 | `PHASE-REORG-1J-R4C-FINAL-CLOSURE-AUDIT-REPORT.md` | §3 行 **65**；§3.1 行 **75** | G09 交付状态、R1 / R2 PASS、R2 = 37.85 s |
| 4 | `.workbuddy/memory/2026-09-15.md` | 行 **274–276** | 1H 固化 2 个 tripwire 的原始动机（GAP-1H-A） |
| 5 | `.workbuddy/memory/2026-09-15.md` | 行 **299**（R3 回归闸门） | R2 PASS 41.8 s；R1 命名分歧来源 |
| 6 | `.workbuddy/memory/2026-09-15.md` | 行 **313**（R4A PRE-GATE） | 两条 tripwire 文件存在性确认（旧行号 593 / 314） |
| 7 | `.workbuddy/memory/2026-09-15.md` | 行 **355**（R4B） | 「1J-G09 R2 正向 tripwire 未改动仍 PASS」 |
| 8 | `.workbuddy/memory/2026-09-15.md` | 行 **373**（R4C 证据复核） | G09 R1 / R2 PASS，R2 = 37.85 s |
| 9 | `docs/prompts/REORG-G09-empty-shell.original.txt` | 全文件 **35 B**，SHA-256 `25221e11…43308` | 重建前本路径的唯一原始字节备份（DOC-2 Step 3 强制要求） |
| 10 | `PHASE-REORG-1J-DOC-2-EVIDENCE-ARCHIVE-CLOSURE-AUDIT-REPORT.md` | §3 表（G09 行）、§3.1（G09 十七问）、§4.1 第 15 行、§5.1、§7 | 空壳事实与重建授权 |

**文件系统事实（非阶段记录）**：本路径重建前 mtime = `2026-09-15 11:58`，大小 35 B。
该 mtime 只证明空壳的写入时刻，不等同于 G09 阶段的执行时间窗。

---

## EVIDENCE INSUFFICIENT 字段清单

| 字段 | 状态 |
|---|---|
| 原报告 VERDICT 行原文 | EVIDENCE INSUFFICIENT（正文从未落盘） |
| 原报告章节结构 / 字节大小 | EVIDENCE INSUFFICIENT |
| 执行时间窗 | EVIDENCE INSUFFICIENT |
| 生产代码 diff 的显式记录 | EVIDENCE INSUFFICIENT |
| 原报告 STOP 块文本 | EVIDENCE INSUFFICIENT |
| R1 / R2 的完整运行日志 | EVIDENCE INSUFFICIENT（仅存 PASS 结论与耗时） |
| `sealV2Mode` 绕行在 G09 中的处置方式 | EVIDENCE INSUFFICIENT |

---

## STOP

```text
本文件为 RECONSTRUCTED FROM EVIDENCE 归档产物。
不得作为原始执行报告引用。
原始 35 字节空壳已备份，见文件头。
```
