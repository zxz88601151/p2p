# PHASE REORG-1J-DOC-2 — EVIDENCE ARCHIVE / REPORT INTEGRITY CLOSURE AUDIT

- 阶段性质：**STRICT READ-ONLY / EVIDENCE ARCHIVE INTEGRITY / DOCUMENTATION CLOSURE AUDIT**
- 执行日期：2026-09-15
- 基线 HEAD：`e04d678`（未动）
- 生产代码改动：**0**｜测试改动：**0**｜删除/覆盖任何既有文件：**0**
- 本阶段唯一新增文件：本报告
- 扫描范围：`p2pchain/` 仓库（20 份）+ 工作区根 `挖矿\`（16 份）= **36 份** REORG 命名报告

---

## 0. 归档事件记录（2026-09-15 16:08:36 外部覆盖 + 本次重建）

> **本章为重建时追加，原 16:04 版本无此章。**

**事件**：本报告于 16:04 首次写入本路径，大小 **25,664 B**。16:08:36，该文件被**下一阶段 `PHASE REORG-1J-DOC-2-REPAIR` 的 prompt 全文覆盖**，大小变为 **8,810 B**。经核验，覆盖后全文首行为 `# PHASE REORG-1J-DOC-2-REPAIR — EVIDENCE ARCHIVE SAFE REPAIR`，末行为「完成后停止，等待下一授权」，即 **DOC-2-REPAIR 阶段 prompt 原文**。

**意义**：这是 **DOC-2 缺陷根因的当场实证**。机制为——新阶段的 prompt 被写入「上一阶段报告文件」的路径，从而销毁上一阶段报告。这精确解释了本报告 §4 记录的系统性 off-by-one 错位（文件名 = 阶段 X，正文 = 阶段 X+1 的 prompt）。

**处置**：
1. 覆盖后的 8,810 B 内容已作为 prompt 存档保存至 `docs/prompts/REORG-DOC-2-REPAIR.prompt.md`（SHA-256 `d6dd36ad…001db4`），**未删除、未覆盖**；
2. 本文件按 16:04 原文重建，内容与原版一致（仅新增本章）；
3. 16:04 原版的 25,664 B 字节序列**在磁盘上已不可恢复**（快照建立于 16:10，晚于覆盖时刻）。

**结论**：DOC-2 不再是「历史遗留的归档瑕疵」，而是**仍在持续发生的主动破坏**。任何阶段报告写完后必须**立即快照**，不得依赖后续统一归档。

---

## 1. VERDICT

```text
ARCHIVE RECONSTRUCTION REQUIRED
```

原始正文不存在（从未进入版本控制，磁盘上亦无副本），需要从已有 evidence 重建，并**明确标注 `RECONSTRUCTED FROM EVIDENCE`**。

**为什么不是另外三种：**

| 候选 | 否决理由 |
|---|---|
| ARCHIVE CLEAN | 36 份中 **26 份文件名与正文不符**，其中 1 份为空壳 |
| ARCHIVE REPAIR REQUIRED | 该档位前提是「可以通过已有 Git history / 文件证据**安全恢复**」。实测 **Git history 中 REORG 报告提交数 = 0**（见 §5），无 commit、无 rename、无 stash、无悬空 blob ⇒ **不存在可安全恢复的原始版本**，只能重建 |
| ARCHIVE INTEGRITY BLOCKER | Rule C / Rule D：本缺陷**不改变任何技术结论**。R4C 全部结论源自源码 + 测试 + 实测重跑，不依赖报告正文；不存在证据矛盾污染技术链的情形（见 §9 Q1/Q2） |

---

## 2. Executive Summary

1. **DOC-2 的真实规模远大于 R4C 登记的 8 份**。全量扫描 36 份 REORG 命名报告：**10 份正文正确 / 26 份正文错位**（其中 1 份 35 字节空壳）。错位不是偶发，而是**系统性 off-by-one**：文件名声称阶段 X，正文是阶段 X+1 的 prompt。
2. **错位横跨两个阶段族与两个目录**：`p2pchain/`（20 份，16 错）与工作区根 `挖矿\`（16 份，10 错）。工作区根 **不是 git 仓库**，其 16 份报告 100% 在版本控制之外。
3. **没有任何一份 REORG 报告进入过 Git**。35 次提交中曾提交的 `.md` 共 27 个，全部是 `docs/` 下的早期阶段（PHASE-0 / 0.1 / 1A / BRAND / GENESIS / P2.1 / P3.1），**无一份 REORG** ⇒ **Git 恢复路径为零**。
4. **新发现「1I」编号歧义**：存在两个含义不同的 1I —— WBS 原义的 `1I = finality / MaxReorgDepth`（**未执行，DEFERRED**）与后期的 `1I = LEGACY/V2 CANONICAL REORG STORAGE AUDIT`（**执行过，但报告正文被 1J prompt 覆盖**）。后者产物 `PHASE-REORG-1I-LEGACY-V2-CANONICAL-REORG-STORAGE-AUDIT-FINAL-REPORT.md` 正文首行是 `# PHASE REORG-1J — LEGACY PREFIX REORG REMEDIATION`，文件在工作区根。
5. **不影响技术闭环**：GAP-1H-A 仍 CLOSED，R4C 的 14/14 仍成立（Rule C）。DOC-2 是 documentation / evidence archive integrity issue，不是 consensus defect（Rule D）。
6. **不得删除任何错位文件**：它们很可能是这些阶段 prompt 的**唯一现存副本**（Downloads 中仅存 3 份近期 prompt）。修复动作只能是 **rename 归档**，不能 overwrite。
7. **缺陷仍在持续发生**：见 §0 —— 本报告自身在完成 4 分钟后即被下一阶段 prompt 覆盖。

---

## 3. Canonical Evidence Matrix（§3 / §5）

以**正文内容 + 阶段标题 + 日期 + HEAD + VERDICT + 阶段目标 + 测试结果**交叉判定，不以文件名为准。

| Stage | Expected Report | Actual File | Actual Content | Integrity | Action |
|---|---|---|---|---|---|
| **1G** | FINAL GAP AUDIT | `p2pchain/PHASE-REORG-1G-FINAL-GAP-AUDIT-REPORT.md` (9,315 B) | **REORG-1H prompt** | **WRONG** | rename → prompt 归档；重建 1G（RECONSTRUCTED） |
| **1H** | FINAL REPORT | `p2pchain/PHASE-REORG-1H-FINAL-REPORT.md` (20,747 B) | 真实 1H 报告，15 节 + `## 15. STOP` | **PASS** | 保留不动 |
| **1I** | LEGACY/V2 CANONICAL REORG STORAGE AUDIT | `挖矿\PHASE-REORG-1I-LEGACY-V2-CANONICAL-REORG-STORAGE-AUDIT-FINAL-REPORT.md` (12,020 B) | **REORG-1J prompt**（其「前置报告」段引用 1I 结论 `NOT READY FOR IMPLEMENTATION` 与 GAP-1I-A..E） | **WRONG + 编号歧义** | rename → 1J prompt 归档；1I 正文证据不足，标记 `LOST / REFERENCE-ONLY` |
| **G08** | CONTRACT EXCEPTION FINAL | `p2pchain/PHASE-REORG-1J-G08-CONTRACT-EXCEPTION-FINAL-REPORT.md` (7,083 B) | **G09 prompt** | **WRONG** | rename → prompt 归档；重建 G08（RECONSTRUCTED） |
| **G09** | TRIPWIRE CONVERSION FINAL | `p2pchain/PHASE-REORG-1J-G09-TRIPWIRE-CONVERSION-FINAL-REPORT.md` (**35 B**) | **空壳**，全文仅 `senior‑software‑engineer` | **MISSING** | 重建 G09（RECONSTRUCTED） |
| **R3** | LEGACY-PREFIX CRASH MATRIX FINAL | `p2pchain/PHASE-REORG-1J-R3-LEGACY-PREFIX-CRASH-MATRIX-FINAL-REPORT.md` (4,392 B) | **R4A PRE-GATE prompt**（末行「只有在 PRE-GATE 明确判定 READY 后…」） | **WRONG** | rename → prompt 归档；重建 R3（RECONSTRUCTED） |
| **R4A** | (a) PRE-GATE FINAL ／ (b) FINAL REPORT | (a) `p2pchain/PHASE-REORG-1J-R4A-PRE-GATE-FINAL-REPORT.md` (8,623 B)<br>(b) `p2pchain/docs/PHASE-REORG-1J-R4A-FINAL-REPORT.md` (8,061 B) | (a) **R4A IMPLEMENTATION-1 prompt**<br>(b) **R4B PRE-GATE prompt** | **WRONG ×2** | 两个都 rename → prompt 归档；重建 R4A PRE-GATE 与 R4A FINAL（RECONSTRUCTED） |
| **R4B** | PRE-GATE FINAL | `p2pchain/PHASE-REORG-1J-R4B-PRE-GATE-FINAL-REPORT.md` (25,298 B) | 真实 R4B 报告，16 节 + `**STOP.**` | **PASS** | 保留不动 |
| **R4C** | FINAL CLOSURE AUDIT | `p2pchain/PHASE-REORG-1J-R4C-FINAL-CLOSURE-AUDIT-REPORT.md` (29,148 B) | 真实 R4C 报告，17 节 + 收尾块 + `## STOP` | **PASS** | 保留不动（canonical artifact 已形成） |

**统计：9 个受审阶段中 3 PASS / 5 WRONG / 1 MISSING，无一可按文件名直接采信。**

### 3.1 逐阶段 17 问（§4）

记号：`Y`=是 `N`=否 `—`=不适用

**1G** — ①文件名声称 1G ②正文 = 1H prompt ③正文完整（但非本报告） ④是下一阶段 prompt：`Y` ⑤空壳：`N` ⑥真正 FINAL REPORT：`N` ⑦同阶段多版本：`N` ⑧正文被覆盖：`推定 Y`（无法证明） ⑨Git 可恢复：`N`（0 commit） ⑩可从 evidence 重建：`Y`（`.workbuddy/memory/2026-09-15.md:236-258` 含 VERDICT / 阻塞 6 项 / 测试结论） ⑪须标 reconstructed：`Y` ⑫影响技术证据链：`N` ⑬影响 Git commit integrity：`Y`（会把 1H prompt 当 1G 报告提交） ⑭须 rename：`Y` ⑮须 restore：不可 ⑯须 regenerate：`Y`（RECONSTRUCTED） ⑰可保留不动：`N`

**1H** — ①1H ②真实 1H 报告 ③完整（15 节 + STOP） ④`N` ⑤`N` ⑥`Y` ⑦`N` ⑧`N` ⑨`—` ⑩`—` ⑪`N` ⑫`N` ⑬`N` ⑭`N` ⑮`—` ⑯`N` ⑰**`Y`（保留不动）**

**1I** — ①1I（LEGACY/V2 CANONICAL REORG STORAGE AUDIT） ②REORG-1J prompt ③`N`（无 1I 正文） ④`Y` ⑤`N` ⑥`N` ⑦**`Y`（编号歧义：另有 WBS 原义 1I = finality/MaxReorgDepth，未执行）** ⑧`推定 Y` ⑨`N` ⑩**`N`（证据不足：无执行日志，仅 1J prompt 转述结论 `NOT READY FOR IMPLEMENTATION` 与 GAP-1I-A..E 清单）** ⑪`Y`（若将来重建） ⑫`N` ⑬`Y` ⑭`Y` → 1J prompt 归档 ⑮不可 ⑯**建议不重建，标记 `LOST / REFERENCE-ONLY`** ⑰`N`

**G08** — ①G08 ②G09 prompt ③`Y`（作为 prompt 完整） ④`Y` ⑤`N` ⑥`N` ⑦`N` ⑧`推定 Y` ⑨`N` ⑩`Y`（`2026-09-15.md` 行 299/313/325/360/372/373/379；R4C §12 复跑记录 G08 ×3 PASS） ⑪`Y` ⑫`N` ⑬`Y` ⑭`Y` ⑮不可 ⑯`Y`（RECONSTRUCTED） ⑰`N`

**G09** — ①G09 ②空壳（35 B，仅 skill 名） ③`N` ④`N` ⑤**`Y`** ⑥`N` ⑦`N` ⑧`N`（从未写入正文） ⑨`N` ⑩`Y`（`2026-09-15.md` 行 299/313/325/355/360/372/373/379；R4C §12 复跑记录 G09 R1/R2 PASS） ⑪`Y` ⑫`N` ⑬`Y` ⑭`N`（文件名正确，无需 rename） ⑮不可 ⑯`Y`（RECONSTRUCTED，**必须先备份该 35 B**） ⑰`N`

**R3** — ①R3 ②R4A PRE-GATE prompt ③`Y`（作为 prompt） ④`Y` ⑤`N` ⑥`N` ⑦`N` ⑧`推定 Y` ⑨`N` ⑩`Y`（`2026-09-15.md:282-305` R3 专段，含 PASS / 零生产改动 / 未提交） ⑪`Y` ⑫`N` ⑬`Y` ⑭`Y` ⑮不可 ⑯`Y`（RECONSTRUCTED） ⑰`N`

**R4A** — ①R4A PRE-GATE + R4A FINAL（两份） ②(a) R4A IMPLEMENTATION-1 prompt；(b) R4B PRE-GATE prompt ③`Y`（作为 prompt） ④`Y` ⑤`N` ⑥`N` ⑦`Y`（同阶段两份，双双错位） ⑧`推定 Y` ⑨`N` ⑩`Y`（PRE-GATE：`2026-09-15.md:306-330`；IMPLEMENTATION-1：`:331-340`，含 15 适用格 / 18,247 字节点 / I9 15/15 / I10 225 次重启） ⑪`Y` ⑫`N` ⑬`Y` ⑭`Y`（两份都 rename） ⑮不可 ⑯`Y`（RECONSTRUCTED ×2） ⑰`N`

**R4B** — ①R4B ②真实 R4B 报告 ③完整（16 节 + STOP） ④`N` ⑤`N` ⑥`Y` ⑦`N` ⑧`N` ⑨`—` ⑩`—` ⑪`N` ⑫`N` ⑬`N` ⑭`N` ⑮`—` ⑯`N` ⑰**`Y`**

**R4C** — ①R4C ②真实 R4C 报告 ③完整（17 节 + 收尾块 + STOP） ④`N` ⑤`N` ⑥`Y`（canonical archive artifact 已形成） ⑦`N` ⑧`N` ⑨`—` ⑩`—` ⑪`N` ⑫`N` ⑬`N`（但属未跟踪文件，R5 须显式 `git add`） ⑭`N` ⑮`—` ⑯`N` ⑰**`Y`**

---

## 4. File-by-File Archive Audit（§4）

### 4.1 `p2pchain/` 仓库内（20 份）

| # | 文件名 | 文件名声称 | 正文实际 | 判定 |
|---|---|---|---|---|
| 1 | PHASE-REORG-1C-COMMIT-1-FINAL-REPORT.md | 1C-COMMIT-1 | REORG-1C-PUSH-1 prompt | ❌ WRONG |
| 2 | PHASE-REORG-1C-IMPLEMENTATION-1-COMMIT-READINESS-AUDIT.md | 1C-IMPL-1 提交就绪审计 | REORG-1C-COMMIT-1 prompt | ❌ WRONG |
| 3 | PHASE-REORG-1C-PUSH-1-FINAL-REPORT.md | 1C-PUSH-1 | REORG-1D prompt | ❌ WRONG |
| 4 | PHASE-REORG-1D-DESIGN-1-FINAL-REPORT.md | 1D-DESIGN-1 | REORG-1F-DESIGN-1 prompt | ❌ WRONG |
| 5 | PHASE-REORG-1F-DESIGN-1-FINAL-REPORT.md | 1F-DESIGN-1 | 1F-PRE-IMPLEMENTATION-GATE-1 prompt | ❌ WRONG |
| 6 | PHASE-REORG-1F-PRE-IMPLEMENTATION-GATE-1-FINAL-REPORT.md | 1F-PRE-GATE | 1F-IMPLEMENTATION-1 prompt | ❌ WRONG |
| 7 | **PHASE-REORG-1F-IMPLEMENTATION-1-FINAL-REPORT.md** | 1F-IMPL-1 | ✅ 真实报告（含状态/日期/产物根） | ✅ PASS |
| 8 | PHASE-REORG-1F-COMMIT-READINESS-AUDIT-1-FINAL-REPORT.md | 1F 提交就绪审计-1 | 1F-REMEDIATION-1 prompt | ❌ WRONG |
| 9 | PHASE-REORG-1F-COMMIT-READINESS-AUDIT-2-FINAL-REPORT.md | 1F 提交就绪审计-2 | 1F-COMMIT-1 prompt | ❌ WRONG |
| 10 | PHASE-REORG-1F-COMMIT-1-FINAL-REPORT.md | 1F-COMMIT-1 | REORG-1G prompt | ❌ WRONG |
| 11 | PHASE-REORG-1G-FINAL-GAP-AUDIT-REPORT.md | 1G | REORG-1H prompt | ❌ WRONG |
| 12 | **PHASE-REORG-1H-FINAL-REPORT.md** | 1H | ✅ 真实报告 | ✅ PASS |
| 13 | PHASE-REORG-1J-IMPLEMENTATION-1-CONTRACT-CONFLICT-REPORT.md | 1J-IMPL-1 契约冲突 | G08 prompt | ❌ WRONG |
| 14 | PHASE-REORG-1J-G08-CONTRACT-EXCEPTION-FINAL-REPORT.md | G08 | G09 prompt | ❌ WRONG |
| 15 | PHASE-REORG-1J-G09-TRIPWIRE-CONVERSION-FINAL-REPORT.md | G09 | 空壳（35 B） | ❌ MISSING |
| 16 | PHASE-REORG-1J-R3-LEGACY-PREFIX-CRASH-MATRIX-FINAL-REPORT.md | R3 | R4A PRE-GATE prompt | ❌ WRONG |
| 17 | PHASE-REORG-1J-R4A-PRE-GATE-FINAL-REPORT.md | R4A-PRE-GATE | R4A IMPLEMENTATION-1 prompt | ❌ WRONG |
| 18 | docs/PHASE-REORG-1J-R4A-FINAL-REPORT.md | R4A FINAL | R4B PRE-GATE prompt | ❌ WRONG |
| 19 | **PHASE-REORG-1J-R4B-PRE-GATE-FINAL-REPORT.md** | R4B | ✅ 真实报告 | ✅ PASS |
| 20 | **PHASE-REORG-1J-R4C-FINAL-CLOSURE-AUDIT-REPORT.md** | R4C | ✅ 真实报告 | ✅ PASS |

小计：**4 PASS / 16 WRONG**。

### 4.2 工作区根 `C:\Users\Administrator\Desktop\挖矿\`（16 份，全部在 Git 之外）

| # | 文件名 | 文件名声称 | 正文实际 | 判定 |
|---|---|---|---|---|
| 1 | PHASE-REORG-1A-FINAL-REPORT.md | 1A | **PHASE DIFFICULTY-CONSENSUS-DESIGN-1** prompt（跨阶段族污染） | ❌ WRONG |
| 2 | PHASE-REORG-1B-COMMIT-EXECUTION-2-COMMITS-FINAL-REPORT.md | 1B-COMMIT-EXECUTION | REORG-1C-PRE-IMPLEMENTATION-GATE-1 prompt | ❌ WRONG |
| 3 | **PHASE-REORG-1B-COMMIT-READINESS-AUDIT-REPORT.md** | 1B 提交就绪审计 | ✅ 真实报告（H1 匹配 + 裁定） | ✅ PASS |
| 4 | PHASE-REORG-1C-COMMIT-MODEL-DESIGN-1-FINAL-REPORT.md | 1C-COMMIT-MODEL-DESIGN-1 | REORG-1C-IMPLEMENTATION-1 prompt | ❌ WRONG |
| 5 | PHASE-REORG-1C-PRE-GATE-1-FINAL-REPORT.md | 1C-PRE-GATE-1 | REORG-1C-COMMIT-MODEL-DESIGN-1 prompt | ❌ WRONG |
| 6 | PHASE-REORG-1C-PRE-IMPLEMENTATION-GATE-1-FINAL-REPORT.md | 1C-PRE-IMPL-GATE-1 | REORG-INFRASTRUCTURE-IMPLEMENTATION-2（REORG-1D）prompt | ❌ WRONG |
| 7 | PHASE-REORG-1E-COMMIT-1-FINAL-REPORT.md | 1E-COMMIT-1 | REORG-1E-PUSH-1 prompt | ❌ WRONG |
| 8 | **PHASE-REORG-1E-IMPLEMENTATION-1-CONTRACT-CONFLICT-REPORT.md** | 1E-IMPL-1 契约冲突 | ✅ 真实报告 | ✅ PASS |
| 9 | PHASE-REORG-1E-IMPLEMENTATION-1-FINAL-REPORT.md | 1E-IMPL-1 | 「执行 PHASE REORG-1E-COMMIT-1」prompt | ❌ WRONG |
| 10 | **PHASE-REORG-1E-PRE-IMPLEMENTATION-DESIGN-AUDIT-1-FINAL-REPORT.md** | 1E 设计审计 | ✅ 真实报告（§0 VERDICT） | ✅ PASS |
| 11 | PHASE-REORG-1E-PUSH-1-FINAL-REPORT.md | 1E-PUSH-1 | REORG-1C-PRE-GATE-1 prompt | ❌ WRONG |
| 12 | PHASE-REORG-1I-LEGACY-V2-CANONICAL-REORG-STORAGE-AUDIT-FINAL-REPORT.md | 1I | **REORG-1J prompt** | ❌ WRONG |
| 13 | **PHASE-REORG-INFRASTRUCTURE-DESIGN-1-REPORT.md** | INFRA-DESIGN-1 | ✅ 真实报告（FINAL VERDICT） | ✅ PASS |
| 14 | **PHASE-REORG-INFRASTRUCTURE-IMPLEMENTATION-1-REORG-1B-FINAL-REPORT.md** | INFRA-IMPL-1 / 1B | ✅ 真实报告（裁定） | ✅ PASS |
| 15 | **PHASE-REORG-INFRASTRUCTURE-IMPLEMENTATION-2-REORG-1D-FINAL-REPORT.md** | INFRA-IMPL-2 / 1D | ✅ 真实报告（§16 VERDICT） | ✅ PASS |
| 16 | PHASE-REORG-INFRASTRUCTURE-PRE-IMPLEMENTATION-GATE-1-REPORT.md | INFRA-PRE-IMPL-GATE-1 | REORG-1A prompt | ❌ WRONG |

小计：**6 PASS / 10 WRONG**。

### 4.3 合计

| 位置 | 文件数 | PASS | WRONG | 在 Git 内 |
|---|---|---|---|---|
| `p2pchain/`（含 `docs/`） | 20 | 4 | 16 | **0** |
| 工作区根 `挖矿\` | 16 | 6 | 10 | **0**（该目录非 git 仓库） |
| **合计** | **36** | **10** | **26** | **0** |

---

## 5. Git History Recovery Findings（§6）

**结论：Git 恢复路径为零。不是「被覆盖后找不到」，而是「从未进入版本控制」。**

| 检查项 | 命令 | 结果 |
|---|---|---|
| REORG 报告是否被追踪 | `git ls-files <file>` | 全部 **0 行**（UNTRACKED） |
| REORG 报告提交次数 | `git log --oneline --all -- <file>` | 全部 **0** |
| 是否曾有任何 REORG 报告被提交 | `git log --all --name-only -- '*.md' \| sort -u` | 27 个 `.md`，**全部为 `docs/` 下早期阶段**（PHASE-0 / 0.1 / 1A / BRAND-* / GENESIS-* / P2.1 / P3.1 / PROJECT-COMPLETION / README / RUN-AUDIT / design） |
| 是否被 .gitignore 屏蔽 | `git check-ignore -v <file>` | **否**（退出码 1，无匹配） ⇒ 只是从未 `git add` |
| stash | `git stash list` | 空 |
| 分支 | `git branch -a` | 仅 `main` + `remotes/gitea/main`，无备份分支 |
| 悬空对象 | `git fsck --unreachable --dangling` | 仅 `4b825dc`（= 空 tree 常量），**无任何悬空 blob** |
| rename history | `git log --follow` | 无（文件从未被追踪） |
| 总提交数 | `git rev-list --all --count` | 35 |

补充证据：1C 提交 `369d36e` 与 1D/1E 提交 `6ed1e83` 的 `--stat` **只含代码与测试，不含任何报告** ⇒ 历史惯例即为「提交代码、不提交报告」。

### 5.1 推荐恢复来源（只推荐，不执行）

| 目标 | Git 可恢复 | 推荐恢复/重建来源 |
|---|---|---|
| 1G | ❌ | `.workbuddy/memory/2026-09-15.md:236-258`（含 VERDICT=PASS WITH BLOCKING GAPS、阻塞 6 项 B-1..B-6、测试结论、`.123` 部署落后 6 commit 的决定性发现） |
| 1I（LEGACY/V2 审计） | ❌ | **证据不足**。唯一现存信息是 1J prompt 的转述（结论 `NOT READY FOR IMPLEMENTATION`、GAP-1I-A..E 清单）。**建议标记 `LOST / REFERENCE-ONLY`，不要重建** |
| G08 | ❌ | `2026-09-15.md` 行 299/313/325/360/372/373/379；R4C §12 复跑记录（G08 ×3 PASS） |
| G09 | ❌ | `2026-09-15.md` 行 299/313/325/355/360/372/373/379；R4C §12 复跑记录（R1/R2 PASS，R2 = 37.85 s） |
| R3 | ❌ | `2026-09-15.md:282-305`（R3 专段：PASS、1,818 字节崩溃点、5 次进程 kill、40 次重复、零生产改动） |
| R4A PRE-GATE | ❌ | `2026-09-15.md:306-330`（VERDICT=READY） |
| R4A FINAL | ❌ | `2026-09-15.md:331-340`（15 适用格 / 18,247 字节点 / I9 15/15 / I10 225 次重启 / I12 30 次签名一致） |

**规则约束**：以上重建产物首部必须标注 `RECONSTRUCTED FROM EVIDENCE` 并列出证据源行号（Rule A / Rule B），不得伪装成原始执行报告。

---

## 6. Missing / Misnamed / Overwritten Reports（§6）

### 6.1 MISSING（正文不存在，需重建或标记丢失）

`1G` · `1I`(LEGACY/V2) · `G08` · `G09`（空壳） · `R3` · `R4A-PRE-GATE` · `R4A-FINAL` —— 共 **7 份**（R4A 占 2 份）。

### 6.2 MISNAMED（文件名声称阶段 ≠ 正文阶段）

**26 份**（见 §4.1 / §4.2 全部 ❌ 行）。其中跨阶段族污染 1 例：

- `挖矿\PHASE-REORG-1A-FINAL-REPORT.md` → 正文是 `PHASE DIFFICULTY-CONSENSUS-DESIGN-1`（**不是 REORG 阶段族**）

### 6.3 OVERWRITTEN（推定，无法证明）

26 份错位文件的正文**推定**曾被下一阶段 prompt 覆盖（off-by-one 规律高度一致）。但因文件从未进入 Git，**无法证明原始正文曾存在及其内容**。本报告将此项登记为「推定」，不作事实断言。

> **补记（16:08）**：§0 记录了一次**当场实证**的覆盖事件，证明该机制确实存在且仍在发生。因此 §6.3 的「推定」在机制层面已获得直接支撑 —— 但具体到每一份历史文件，仍无法证明其原始正文内容。

### 6.4 可保留不动（10 份）

`1F-IMPLEMENTATION-1` · `1H` · `R4B` · `R4C`（仓库内 4 份）
`1B-COMMIT-READINESS-AUDIT` · `1E-IMPLEMENTATION-1-CONTRACT-CONFLICT` · `1E-PRE-IMPLEMENTATION-DESIGN-AUDIT-1` · `INFRA-DESIGN-1` · `INFRA-IMPLEMENTATION-1(1B)` · `INFRA-IMPLEMENTATION-2(1D)`（工作区根 6 份）

---

## 7. Original-vs-Reconstructed Classification（§7）

| 类别 | 数量 | 文件 |
|---|---|---|
| **ORIGINAL**（正文 = 文件名声称阶段，可直接采信） | **10** | 见 §6.4 |
| **PROMPT-ARCHIVE**（正文是下一阶段 prompt，须 rename 归档，**不得删除**） | **25** | 26 份错位文件中除 G09 空壳外的 25 份 |
| **EMPTY SHELL**（无正文） | **1** | `PHASE-REORG-1J-G09-TRIPWIRE-CONVERSION-FINAL-REPORT.md`（35 B） |
| **RECONSTRUCTED FROM EVIDENCE**（须新建，明确标注） | **6** | 1G · G08 · G09 · R3 · R4A-PRE-GATE · R4A-FINAL |
| **LOST / REFERENCE-ONLY**（证据不足，不重建） | **1** | 1I（LEGACY/V2 CANONICAL REORG STORAGE AUDIT） |

**为什么 1I 不重建**：除 1J prompt 的转述外无任何执行日志、无测试产物、无 commit、无实测输出。按 Rule A（不得伪造历史报告），只能标记 `LOST / REFERENCE-ONLY`，并在 R5 的交付说明中显式登记「1I 原始报告已丢失，其结论以 1J prompt 的『前置报告』段为唯一引用来源」。

---

## 8. Recommended DOC-2 Repair Plan（§8）

> **本节为计划输出，DOC-2 审计阶段未执行任何一步（修复由后续 REPAIR 阶段执行）。**

### Step 1 — 冻结并保护现有文件（最高优先）

- 对 36 份文件建立只读快照（复制到 `docs/_archive_pre-doc2/` 或打 tar），**在任何 rename 之前**完成。
- 理由：这些文件是多数阶段 prompt 的唯一副本；一旦覆盖即永久丢失。

### Step 2 — rename 归档 prompt（25 份）

- 新建 `p2pchain/docs/prompts/`，把 25 份「正文为 prompt」的文件 **rename** 为 `REORG-<真实阶段>.prompt.md`。
- **禁止 overwrite**，**禁止删除**。

### Step 3 — 处理 G09 空壳（1 份）

- 先把 35 B 原文另存为 `docs/prompts/REORG-G09-empty-shell.original.txt`，再写入 RECONSTRUCTED 报告。

### Step 4 — 重建 6 份报告（RECONSTRUCTED）

- 文件名沿用 canonical 名（保持与 R4C 引用一致），首部强制标注 `RECONSTRUCTED FROM EVIDENCE`。
- 内容只写证据能支撑的部分；证据不支持的栏位写 `EVIDENCE INSUFFICIENT`，**不得填充推测**。

### Step 5 — 统一归档位置（需单独授权）

- 把工作区根 16 份 REORG 报告移入 `p2pchain/docs/phases/`。
- **必须先解决**：工作区根 `挖矿\` 不是 git 仓库，其中的 57 份阶段报告（含 REORG 16 份）**全部在版本控制之外**。

### Step 6 — 消除 1I 编号歧义（需单独授权）

- 明确两个 1I：`1I-finality/MaxReorgDepth`（WBS 原义，**未执行**，DEFERRED）与 `1I-LEGACY/V2 CANONICAL REORG STORAGE AUDIT`（执行过，**报告丢失**）。
- 建议后者重编号为 `REORG-1J-PRE-GATE`（其结论正是 1J 的前置），并在 MEMORY 中固化该消歧。

### Step 7 — R5 提交前的归档门禁

- `.gitignore` **未屏蔽**报告（已验证）⇒ 无需改 ignore，但 R5 必须**显式 `git add`** 每一份报告。
- 提交说明显式登记：BT-1、DOC-2 修复范围、6 份 RECONSTRUCTED 报告、1 份 LOST、以及全部 DEFERRED 项。
- 仍须排除长期 dirty：`docs/DETERMINISTIC-SERIALIZATION-SPEC.md`、`docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md`、`run-a/`、`run-b/`、`verifier/`。

### Step 8 — 防止复发

- 阶段报告写完后**必须回读前 20 行**确认不是下一阶段 prompt（已写入 `p2pchain-phase-audit` 技能 §8.4）。
- 建议每个阶段结束即 `git add` 报告文件，避免再次出现「0 commit 报告」。
- **（16:08 补记，最高优先）阶段报告写完后必须立即建立独立快照**。下一阶段 prompt 会写入上一阶段报告路径并销毁它，回读前 20 行只能发现、不能阻止。

---

## 9. R5 Impact Assessment（§10）

### Q1 — REORG-1J 技术闭环是否仍然成立？

**是。** R4C 的 14 个证据域全部由**源码 + 测试代码 + 实测重跑**支撑，不依赖任何报告正文。本阶段复核的三份真实报告（1H / R4B / R4C）均完整且自洽。按 **Rule C**，DOC-2 不得使 R4C 的技术结论被重新判定为技术失败。

### Q2 — GAP-1H-A 是否仍然 CLOSED？

**是。** 闭合证据 = `cmd/node/r4b_dual_reorg_convergence_test.go` + 实测输出（L=2/8/64，legacy 前缀字节不变，双节点 canonical 逐高度全等，重启 ×2 轮签名一致）+ 源码路径（`blockchain.go:512` / `v2api.go` / `v2.go:713`）。三者均独立于报告正文。

### Q3 — DOC-2 是否唯一需要处理的交付归档问题？

**否。** 除报告错位外，同属交付归档问题的还有：
1. **证据归档位置分裂**：36 份报告分布在 `p2pchain/`（20）与工作区根（16），后者不在任何 git 仓库中；
2. **报告零版本化**：35 次提交、27 个 `.md`，**无一 REORG 报告**；
3. **1I 编号歧义**：两个不同含义的 1I 并存；
4. **G09 空壳**：35 B。

以上**均非 consensus / storage / P2P 缺陷**（Rule D）。

### Q4 — 是否可以在 DOC-2 修复后进入 Commit Readiness Audit？

**可以**，前提是完成 Step 1–Step 4（保护快照 / rename prompt 归档 / 空壳备份 / 6 份 RECONSTRUCTED）。Step 5–Step 6（统一归档位置、1I 消歧）建议在同一轮完成，但技术上不阻塞 Commit Readiness Audit。

### Q5 — 本阶段是否允许 commit？

```text
NO
```

---

## 10. Explicit Non-Actions（§10）

- ❌ 未修改任何 production code / 测试代码 / 测试断言
- ❌ 未修改 consensus / storage / P2P 实现、`v2Mode`、reorg execution、TIP / commit semantics
- ❌ 未修复 BT-1 / OBS-1J-R3-A / R4A-OBS-1
- ❌ 未实现 finality / MaxReorgDepth / orphan pool / fork observability
- ❌ 未重跑 R3 / R4A / R4B 大规模实验（Rule F）
- ❌ 未删除任何历史证据、未覆盖任何现有报告正文
- ❌ 未 rename / 未 restore / 未重建任何文件（修复计划只输出，不执行）
- ❌ 未 commit / push / tag / merge / rebase / amend / squash
- ❌ 未 deploy（Windows 或 Linux 生产）、未进行 Windows mining

---

## 11. Final Decision（§11）

```text
ARCHIVE RECONSTRUCTION REQUIRED
```

**规模**：36 份受审 → 10 ORIGINAL / 25 PROMPT-ARCHIVE / 1 EMPTY SHELL；需新建 6 份 RECONSTRUCTED、标记 1 份 LOST。

**技术闭环不受影响**：R4C 的 `CONDITIONALLY CLOSED / R5 READY` 与 GAP-1H-A = CLOSED **继续成立**（Rule C / Rule D）。DOC-2 是 documentation / evidence archive integrity issue，不是 consensus defect。

**建议下一步授权（唯一）**：

```text
REORG-1J-DOC-2-REPAIR — EVIDENCE ARCHIVE SAFE REPAIR
范围：
① Step 1：36 份（+1 新增）全量不可变快照 + SHA-256 清单
② Step 2：25 份 prompt rename 归档到 docs/prompts/（不删除、不覆盖）
③ Step 3：G09 35 B 空壳安全备份
④ Step 4：重建 6 份 RECONSTRUCTED FROM EVIDENCE 报告
⑤ 明确 1I = LOST / REFERENCE-ONLY，不重建
之后候选：REORG-1J-R5 — COMMIT ORGANIZATION
```

---

## 12. STOP

```text
PRODUCTION CODE CHANGES : 0
FILES RENAMED           : 0
FILES OVERWRITTEN       : 0
FILES DELETED           : 0
REPORTS RECONSTRUCTED   : 0
COMMIT                  : NO
PUSH                    : NO
TAG                     : NO
DEPLOY                  : NO
WINDOWS MINING          : NO
```

本审计到此结束。不执行修复计划，不重建任何报告，不提交任何代码，不部署节点，不进行 Windows 挖矿测试。等待下一步授权。
