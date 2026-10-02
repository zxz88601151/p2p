# PHASE REORG-1J-G08 — CONTRACT EXCEPTION FINAL REPORT

```text
RECONSTRUCTED FROM EVIDENCE
```

> **This document is a reconstructed archival artifact. It is NOT the original execution report.**
>
> Original execution report was not recoverable from Git history. This reconstruction is based only on surviving evidence.
>
> 原始报告 `PHASE-REORG-1J-G08-CONTRACT-EXCEPTION-FINAL-REPORT.md` 的正文本 REORG-1J-DOC-2 审计时被确认为
> **REORG-1J-G09 的 prompt 全文**（系统性 off-by-one 覆盖）；原始 7,083 B 字节序列已不可恢复，
> 该 prompt 副本已 rename 归档至 `docs/prompts/REORG-G09.prompt.md`，原始字节另存于
> `docs/_archive_pre-doc2/repo/PHASE-REORG-1J-G08-CONTRACT-EXCEPTION-FINAL-REPORT.md`。

- 重建阶段：`REORG-1J-DOC-2-REPAIR`
- 重建日期：2026-09-15
- 重建依据：见 §Evidence Sources（全部可定位到文件 + 行号）

---

## 1. VERDICT

```text
DELIVERED（CONTRACT EXCEPTION）
```

证据：`PHASE-REORG-1J-R4C-FINAL-CLOSURE-AUDIT-REPORT.md` §3.1 行 74 ——
「**REORG-1J-G08** | DELIVERED（契约例外，工作区未提交）| `TestG08_*` ×3 本阶段复跑 PASS；Gate2 已移除，canonical 由 TIP + 哈希链决定 | NO」。

> 注：原始 G08 报告自身的 VERDICT 行文本**未留存**。此处引用的是 R4C 对 G08 交付状态的复核裁定。
> 原报告 VERDICT 原文 = **EVIDENCE INSUFFICIENT**。

---

## 2. Stage Identity

| 字段 | 值 | 证据 |
|---|---|---|
| 阶段名 | PHASE REORG-1J-G08 — CONTRACT EXCEPTION | DOC-2 §3 表（G08 行）、§12 canonical filename 表 |
| 执行日期 | 2026-09-15 | `2026-09-15.md` 当日日志；具体时间窗 **EVIDENCE INSUFFICIENT** |
| 执行时间窗 | **EVIDENCE INSUFFICIENT** | 见 §EVIDENCE INSUFFICIENT 字段清单 |
| 基线 HEAD | `e04d678` | 阶段链 1G→R4C 全程未发生 commit，`git rev-parse HEAD` 至今仍为 `e04d678`；但 G08 自身的基线记录行未留存 |
| 交付性质 | 受控 TEST CONTRACT UPDATE（改测试契约，不改生产） | `internal/storage/v2_test.go:622` 注释块标题 |

---

## 3. Scope

G08 处理 1H 阶段发现的 **GAP-1H-A** 在**存储层测试契约**上的投影：

- `internal/storage/v2.go` 的 `setCanonicalFrom` 原先以 **Gate2** 拒绝让 v2 区块占据 `height < legacyLen` 的槽位；
- 该拒绝行为被一条负向测试断言固化为「永久契约」；
- REORG-1J §1 / §6 / §7 / §11 明确**废止**「legacy canonical 归属永久不可变化」：canonical ownership 由 **TIP + hash linkage** 决定，与记录物理形态（legacy / v2）解耦；
- G08 的任务是在**不变更 REORG 实现范围**的前提下，把已废止的负向断言替换为**更强的正向 acceptance test**，同时**保留并加强**真正的安全性质（legacy storage immutability）。

证据：`internal/storage/v2_test.go:618-641` 注释块（本阶段重建时复核，文件内容可验证）。

---

## 4. Contract Change（受控契约例外）

证据：`internal/storage/v2_test.go:618-641`

**被废止的旧断言**：`TruncateFromHeight(1)` 必须返回 `ErrTruncateOutOfRange`（把 legacy canonical membership 永久不可变化写成冻结契约）。

**必须继续保护的性质（逐条对应断言，原文列出 6 条）**：

1. legacy block 不允许通过 `DeleteBlock` 删除；
2. legacy bytes 不得被 rewrite（reorg 前后逐字节一致）；
3. legacy 数据在 reorg 后仍然物理存在；
4. canonical view 与 detached view 语义正确；
5. reorg 后 active TIP 正确；
6. 重新读取 legacy block 的内容与 reorg 前逐字节一致。

**加强项（原注释明示）**：新增逐字节比对 + 物理存在性 + detached 语义 + restart 一致性。

**实现侧结论**：`setCanonicalFrom` **Gate2 已移除**，canonical 由 TIP + 哈希链决定。
证据：`PHASE-REORG-1J-R4C-FINAL-CLOSURE-AUDIT-REPORT.md` §3 行 63、§3.1 行 74。

**错误类型定性**：该旧断言把**已废止的行为**当作正确行为来固化，属**过时契约**（`internal/storage/v2_test.go:618-641` 原文用词）。

---

## 5. Tests

| 测试用例 | 位置 | 结果 |
|---|---|---|
| `TestG08_LogicalDeleteBranchAndTruncate` | `internal/storage/v2_test.go:495` | PASS |
| `TestG08_DeleteBranch` | `internal/storage/v2_test.go:573` | PASS |
| `TestG08_LegacyImmutableAndTruncateGuard` | `internal/storage/v2_test.go:643` | PASS |

**复跑记录（可定位）**：

- `PHASE-REORG-1J-R4C-FINAL-CLOSURE-AUDIT-REPORT.md` §3.1 行 74：`TestG08_*` **×3 本阶段复跑 PASS**；
- 同上 §3 行 65（Existing regression compatibility）：**G08 三项 PASS**；
- 同上 §3 行 63：`TestG08_LegacyImmutableAndTruncateGuard` **本阶段复跑 PASS**；
- `.workbuddy/memory/2026-09-15.md:313`（R4A PRE-GATE 段）：`G08 R1 TestG08_LegacyImmutableAndTruncateGuard` **PASS (0.13 s)**；
- `.workbuddy/memory/2026-09-15.md:373`（R4C 段证据复核）：**G08 ×3 PASS**。

---

## 6. Measured Outputs

| 项 | 值 | 证据 |
|---|---|---|
| G08 测试数量 | 3 项（`TestG08_*`） | `internal/storage/v2_test.go` 函数清单：`LogicalDeleteBranchAndTruncate` / `DeleteBranch` / `LegacyImmutableAndTruncateGuard` |
| R4A PRE-GATE 复跑耗时 | 0.13 s | `2026-09-15.md:313` |
| 生产代码 diff | 0（工作区未提交） | R4C §3.1 行 74 明示「工作区未提交」 |

---

## 7. Non-Actions

- ❌ 未变更 REORG 实现范围（仅改测试契约）——`internal/storage/v2_test.go:618-641` 原文
- ❌ 未削弱 legacy storage immutability（保留并加强）
- ❌ 未 commit / push（`R4C §3.1` 行 74：「工作区未提交」）
- ❌ 未修改生产代码（本阶段无生产 diff 记录；见 §EVIDENCE INSUFFICIENT）

---

## 8. Stop Condition

原报告 STOP 块文本 **EVIDENCE INSUFFICIENT**。

可证据支撑的等价事实：G08 在 REORG-1J 阶段链中已结束，其后继阶段 G09（tripwire 转正）已执行并被 R4C 复核为 DELIVERED（`R4C §3.1` 行 75）。

---

## Evidence Sources

| # | 来源 | 精确定位 | 用途 |
|---|---|---|---|
| 1 | `internal/storage/v2_test.go` | 行 **618–641**（`REORG-1J-G08-CONTRACT-EXCEPTION —— 受控 TEST CONTRACT UPDATE` 注释块） | 契约例外的原因、被废止的旧断言、继续保护的 6 条性质、加强项 |
| 2 | `internal/storage/v2_test.go` | 行 **495**、**573**、**643** | 三个 `TestG08_*` 用例的精确位置 |
| 3 | `PHASE-REORG-1J-R4C-FINAL-CLOSURE-AUDIT-REPORT.md` | §3 行 **63**、行 **65**；§3.1 行 **74** | G08 交付状态、Gate2 已移除、×3 复跑 PASS |
| 4 | `.workbuddy/memory/2026-09-15.md` | 行 **313**（R4A PRE-GATE 段） | `TestG08_LegacyImmutableAndTruncateGuard` PASS (0.13 s) |
| 5 | `.workbuddy/memory/2026-09-15.md` | 行 **373**（R4C 段证据复核） | G08 ×3 PASS |
| 6 | `.workbuddy/memory/2026-09-15.md` | 行 **325**、**360** | R4A PRE-GATE STOP 条件与 R4C 14 条硬锁均含「不改 G08·G09」⇒ G08 产物存在且被锁定 |
| 7 | `PHASE-REORG-1J-DOC-2-EVIDENCE-ARCHIVE-CLOSURE-AUDIT-REPORT.md` | §3 表（G08 行）、§3.1（G08 十七问）、§4.1 第 14 行、§5.1、§7 | 原始报告被 G09 prompt 覆盖的归档事实与重建授权 |
| 8 | `docs/_archive_pre-doc2/repo/PHASE-REORG-1J-G08-CONTRACT-EXCEPTION-FINAL-REPORT.md` | 全文件 7,083 B，SHA-256 `0f937bf3ba30dca9fe699e95cd6d455b3adb3ab22c5636f282cb2304a5a24307` | 被覆盖后残留正文（= G09 prompt）的不可变快照 |

**文件系统事实（非阶段记录，仅作时间参照）**：
`docs/prompts/REORG-1J.prompt.md` mtime = `2026-09-15 10:08`；
`docs/prompts/REORG-G09.prompt.md` mtime = `2026-09-15 11:03`。
两者均为**覆盖后的 prompt 文件**，不等同于 G08 原报告的写入时间。

---

## EVIDENCE INSUFFICIENT 字段清单

| 字段 | 状态 |
|---|---|
| 原报告 VERDICT 行原文 | EVIDENCE INSUFFICIENT |
| 执行时间窗（起止时刻） | EVIDENCE INSUFFICIENT |
| 原报告字节大小 | EVIDENCE INSUFFICIENT（仅知残留 prompt 副本 7,083 B） |
| 原报告章节结构 | EVIDENCE INSUFFICIENT |
| 生产代码 diff 的显式记录 | EVIDENCE INSUFFICIENT（仅由 R4C「工作区未提交」间接支撑） |
| 原报告 STOP 块文本 | EVIDENCE INSUFFICIENT |
| Gate2 移除的具体代码位置 / diff | EVIDENCE INSUFFICIENT（仅存 R4C 结论性表述） |

---

## STOP

```text
本文件为 RECONSTRUCTED FROM EVIDENCE 归档产物。
不得作为原始执行报告引用。
```
