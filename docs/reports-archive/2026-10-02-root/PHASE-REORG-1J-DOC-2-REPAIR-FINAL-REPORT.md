# PHASE REORG-1J-DOC-2-REPAIR — FINAL REPORT

- 阶段性质：**STRICT CONTROLLED DOCUMENTATION / EVIDENCE ARCHIVE SAFE REPAIR**
- 执行日期：2026-09-15
- 基线 HEAD：`e04d678`（未动）—— 本阶段**零 commit / 零 push / 零 tag / 零 merge / 零 rebase / 零 amend / 零 squash / 零 deploy / 零 Windows mining**
- 生产代码改动：**0**｜测试代码改动：**0**｜共识 / 存储 / P2P 改动：**0**
- 授权范围：DOC-2 Step 1–4（不包含 Step 5 / Step 6）

---

## 1. VERDICT

```text
PASS WITH DOCUMENTATION GAPS
```

**为什么是 PASS WITH DOCUMENTATION GAPS，而不是 PASS 或 BLOCKED：**

| 候选 | 裁定 | 理由 |
|---|---|---|
| PASS | ❌ | 6 份报告是 `RECONSTRUCTED FROM EVIDENCE`，**永久不等于原始执行报告**；1 份（1I）**永久 LOST**；DOC-2 Step 5 / Step 6 按授权**明确未执行**。这些是不可消除的文档缺口 |
| **PASS WITH DOCUMENTATION GAPS** | ✅ | Step 1–4 **全部完成且逐项 hash 验证通过**；生产/测试/共识/存储/P2P diff = 0；Git 安全六项全过；无快照丢失、无哈希不一致、无未授权覆盖 |
| BLOCKED | ❌ | BLOCKED 的触发条件是「快照 / hash / rename / reconstruction / evidence provenance 出现不一致」。实测 **25/25 rename 哈希一致、37/37 快照在位、35 B 空壳双备份哈希一致、6/6 重建报告均有可定位 evidence、生产代码 16/16 文件哈希与阶段起点完全相同** ⇒ 无不一致 |

---

## 2. Baseline

执行开始时的复验输出（与 DOC-2 基线 `e04d678` 一致）：

| 项 | 值 |
|---|---|
| `git rev-parse HEAD` | `e04d678b4fb33a14ea186999375f6d6dac2a4c4b` |
| `git branch --show-current` | `main` |
| `git diff --cached --stat` | **空**（staged = 0） |
| `git diff --stat` | 9 个代码文件（`cmd/` 2 + `internal/` 7）+ 2 个长期 dirty 文档 —— **均为 1H/1J/R3/R4A/R4B 阶段遗留，非本阶段产生** |
| 未跟踪 | 含 `docs/_archive_pre-doc2/`、`docs/prompts/`（**上一轮 DOC-2-REPAIR 中断后的残留产物**） |

**基线指纹（供 §10 零-diff 比对）**：16 个生产/测试代码文件的 SHA-256 于 2026-09-15T17:00:03+08:00 全量采集，阶段结束时逐一复算 —— **16/16 完全相同**。

> ⚠️ **重要状态发现**：本阶段开始时，上一轮 `DOC-2-REPAIR` 已**部分执行**（16:10–16:26）：
> 快照目录 `docs/_archive_pre-doc2/`（37 份 + MANIFEST.tsv）已建立，`docs/prompts/` 已存在 25 份文件，RENAME-LOG 已记录 **23 条** rename。
> 本阶段采取**续做而非重做**：先逐条验证已有产物（见 §9 A），再补齐剩余 2 条 rename 与 Step 3 / Step 4。**未覆盖任何既有文件。**

---

## 3. Snapshot Evidence（Step 1）

### 3.1 快照内容

| 项 | 值 |
|---|---|
| 快照根 | `p2pchain/docs/_archive_pre-doc2/` |
| 结构 | `repo/`（21）+ `workspace-root/`（16）+ `reference-only/`（2）= **39 份** |
| 覆盖 | DOC-2 识别的 **36 份 REORG 报告** + DOC-2 报告自身（+1）+ 1I 源副本与 LOST 登记表 |
| 清单 | `MANIFEST.tsv`（37 条，上一轮生成）+ **`MANIFEST.verified.tsv`（39 条，本阶段重算，不覆盖原文件）** |
| 建立时刻 | 2026-09-15 16:10（**早于任何 rename**），本阶段 17:00 后重算校验 |

> 每份快照条目含：相对路径、文件名、字节大小、SHA-256、**完整原始内容副本**（`repo/` / `workspace-root/` / `reference-only/` 三目录）。

### 3.2 完整性校验结果

| 检查 | 结果 |
|---|---|
| 快照文件在位 | **37/37 PASS**（`repo/` 21 + `workspace-root/` 16） |
| 重算 SHA-256 vs `MANIFEST.tsv` | **36/37 一致**；1 条不一致（见下） |
| 现存源文件 vs 快照 | **14/14 PASS**（仓库 6 + 工作区根 8） |
| G09 空壳 35 B 三处比对 | **PASS**（见 §5） |

**唯一的 1 条清单不一致（已定位、已解释、已修正，非数据丢失）**：

```text
repo/PHASE-REORG-1J-DOC-2-EVIDENCE-ARCHIVE-CLOSURE-AUDIT-REPORT.md
  MANIFEST.tsv   : 8,810 B  d6dd36ad…a001db4
  实测（快照/磁盘）: 26,975 B  524c7184…a4658032
```

原因：`MANIFEST.tsv` 生成于 **16:11**，而 DOC-2 报告在 **16:19** 才按 DOC-2 §0 重建为 26,975 B（快照副本同步刷新）。
⇒ **不是快照损坏，是清单过期**。处置：新增 `MANIFEST.verified.tsv`（重算，标注 `UPDATED-vs-MANIFEST.tsv(8810/d6dd36ad,stale)`），**原 `MANIFEST.tsv` 未覆盖、未删除**。
被覆盖前的 8,810 B 版本（= 本阶段 prompt 原文）已单独存档为 `docs/prompts/REORG-DOC-2-REPAIR.prompt.md`（SHA-256 `d6dd36ad…a001db4`，一致）。

---

## 4. Rename Mapping（Step 2）

### 4.1 执行结果

| 项 | 值 |
|---|---|
| 目标数 | **25 份**（DOC-2 §7：`PROMPT-ARCHIVE = 25`） |
| 上一轮已完成 | 23 条（RENAME-LOG.tsv） |
| 本阶段补齐 | **2 条** |
| 归档目录 | `p2pchain/docs/prompts/` |
| 命名依据 | **正文标题 + 正文阶段编号 + 阶段目标**（非原错误文件名） |
| 最终清单 | `docs/_archive_pre-doc2/RENAME-MAP.FINAL.tsv`（25 条 + 表头，含 size / SHA-256 / VERIFY） |
| 校验 | **25/25 `OK`**（dest 存在 + size 一致 + SHA-256 与原始一致） |

### 4.2 本阶段补齐的 2 条

| # | 源路径 | 目标 | size | SHA-256（前 16） | 判定依据 |
|---|---|---|---|---|---|
| 24 | `../PHASE-REORG-INFRASTRUCTURE-PRE-IMPLEMENTATION-GATE-1-REPORT.md` | `docs/prompts/REORG-1A.prompt.md` | 8,758 | `8d014678d0c4ea59` | 正文首行 `PHASE REORG-1A — BLOCKNODE / BLOCK TREE INFRASTRUCTURE IMPLEMENTATION` |
| 25 | `../PHASE-REORG-1I-LEGACY-V2-CANONICAL-REORG-STORAGE-AUDIT-FINAL-REPORT.md` | `docs/prompts/REORG-1J.prompt.md` | 12,020 | `e4cf2dd08a849fcc` | 正文首行 `# PHASE REORG-1J — LEGACY PREFIX REORG REMEDIATION / CANONICAL VIEW UNIFICATION`；**该 rename 的 dest 已由上一轮以复制方式建立（同哈希）** |

### 4.3 Rename Safety Gate 逐条结果

| Gate | 结果 |
|---|---|
| source exists | ✅ 2/2 |
| destination does not exist | ✅ #24 不存在（#25 存在但**字节完全相同**，见下） |
| source SHA-256 已记录 | ✅ 见 `MANIFEST.tsv` 第 37 / 33 行 |
| source content 已在 pre-doc2 快照 | ✅ `workspace-root/` 各 1 份 |
| rename 后 source absent | ✅ 2/2 |
| destination SHA-256 == 原始 | ✅ 2/2 |
| 无 overwrite | ✅（#25 未发生覆盖行为，见下） |

**#25 的特殊处置（不做覆盖、不做删除）**：
上一轮对该文件执行的是**复制**而非移动，导致「文件名声称 1I / 正文为 1J prompt」的错位文件残留于工作区根。
本阶段处置 = 将**源副本移动**（非删除）至 `docs/_archive_pre-doc2/reference-only/PHASE-REORG-1I-LEGACY-V2-CANONICAL-REORG-STORAGE-AUDIT-FINAL-REPORT.md`，
移动前后 SHA-256 均为 `e4cf2dd0…86af3`，与 `docs/prompts/REORG-1J.prompt.md` 逐字节相同。
⇒ **无任何字节销毁，错位文件名已从活动树移除**。

### 4.4 完整映射（25 条，按执行序）

| # | 原文件名（文件名声称） | 归档为（正文真实阶段） |
|---|---|---|
| 1 | PHASE-REORG-1C-COMMIT-1-FINAL-REPORT.md | `REORG-1C-PUSH-1.prompt.md` |
| 2 | PHASE-REORG-1C-IMPLEMENTATION-1-COMMIT-READINESS-AUDIT.md | `REORG-1C-COMMIT-1.prompt.md` |
| 3 | PHASE-REORG-1C-PUSH-1-FINAL-REPORT.md | `REORG-1D.prompt.md` |
| 4 | PHASE-REORG-1D-DESIGN-1-FINAL-REPORT.md | `REORG-1F-DESIGN-1.prompt.md` |
| 5 | PHASE-REORG-1F-DESIGN-1-FINAL-REPORT.md | `REORG-1F-PRE-IMPLEMENTATION-GATE-1.prompt.md` |
| 6 | PHASE-REORG-1F-PRE-IMPLEMENTATION-GATE-1-FINAL-REPORT.md | `REORG-1F-IMPLEMENTATION-1.prompt.md` |
| 7 | PHASE-REORG-1F-COMMIT-READINESS-AUDIT-1-FINAL-REPORT.md | `REORG-1F-REMEDIATION-1.prompt.md` |
| 8 | PHASE-REORG-1F-COMMIT-READINESS-AUDIT-2-FINAL-REPORT.md | `REORG-1F-COMMIT-1.prompt.md` |
| 9 | PHASE-REORG-1F-COMMIT-1-FINAL-REPORT.md | `REORG-1G.prompt.md` |
| 10 | PHASE-REORG-1G-FINAL-GAP-AUDIT-REPORT.md | `REORG-1H.prompt.md` |
| 11 | PHASE-REORG-1J-IMPLEMENTATION-1-CONTRACT-CONFLICT-REPORT.md | `REORG-G08.prompt.md` |
| 12 | PHASE-REORG-1J-G08-CONTRACT-EXCEPTION-FINAL-REPORT.md | `REORG-G09.prompt.md` |
| 13 | PHASE-REORG-1J-R3-LEGACY-PREFIX-CRASH-MATRIX-FINAL-REPORT.md | `REORG-R4A-PRE-GATE.prompt.md` |
| 14 | PHASE-REORG-1J-R4A-PRE-GATE-FINAL-REPORT.md | `REORG-R4A-IMPLEMENTATION-1.prompt.md` |
| 15 | docs/PHASE-REORG-1J-R4A-FINAL-REPORT.md | `REORG-R4B-PRE-GATE.prompt.md` |
| 16 | ../PHASE-REORG-1A-FINAL-REPORT.md | `DIFFICULTY-CONSENSUS-DESIGN-1.prompt.md`（跨阶段族污染） |
| 17 | ../PHASE-REORG-1B-COMMIT-EXECUTION-2-COMMITS-FINAL-REPORT.md | `REORG-1C-PRE-IMPLEMENTATION-GATE-1.prompt.md` |
| 18 | ../PHASE-REORG-1C-COMMIT-MODEL-DESIGN-1-FINAL-REPORT.md | `REORG-1C-IMPLEMENTATION-1.prompt.md` |
| 19 | ../PHASE-REORG-1C-PRE-GATE-1-FINAL-REPORT.md | `REORG-1C-COMMIT-MODEL-DESIGN-1.prompt.md` |
| 20 | ../PHASE-REORG-1C-PRE-IMPLEMENTATION-GATE-1-FINAL-REPORT.md | `REORG-INFRASTRUCTURE-IMPLEMENTATION-2-1D.prompt.md` |
| 21 | ../PHASE-REORG-1E-COMMIT-1-FINAL-REPORT.md | `REORG-1E-PUSH-1.prompt.md` |
| 22 | ../PHASE-REORG-1E-IMPLEMENTATION-1-FINAL-REPORT.md | `REORG-1E-COMMIT-1.prompt.md` |
| 23 | ../PHASE-REORG-1E-PUSH-1-FINAL-REPORT.md | `REORG-1C-PRE-GATE-1.prompt.md` |
| 24 | ../PHASE-REORG-INFRASTRUCTURE-PRE-IMPLEMENTATION-GATE-1-REPORT.md | `REORG-1A.prompt.md` |
| 25 | ../PHASE-REORG-1I-…-STORAGE-AUDIT-FINAL-REPORT.md | `REORG-1J.prompt.md` |

---

## 5. G09 Empty-Shell Preservation（Step 3）

| 项 | 值 |
|---|---|
| 原始文件 | `PHASE-REORG-1J-G09-TRIPWIRE-CONVERSION-FINAL-REPORT.md` |
| 原始大小 | **35 B**（内容 = `senior‑software‑engineer` + 7 个换行，UTF-8 非 ASCII 连字符） |
| 原始 SHA-256 | `25221e1166e0249631ad172c4ff8fbce6f290b34f0cb7b9a1dba69cccc643308` |

**备份（在写入重建报告之前完成）**：

| 副本 | 路径 | 大小 | SHA-256 | 验证 |
|---|---|---|---|---|
| ① 专用备份 | `docs/prompts/REORG-G09-empty-shell.original.txt` | 35 | `25221e11…43308` | ✅ 与原始一致 |
| ② 快照副本 | `docs/_archive_pre-doc2/repo/PHASE-REORG-1J-G09-TRIPWIRE-CONVERSION-FINAL-REPORT.md` | 35 | `25221e11…43308` | ✅ 与原始一致 |

**Gate 结果**：`sha256(原始 35B) == sha256(备份①) == sha256(快照②)` → **三方一致 PASS**，Gate 通过后才写入重建报告。

⇒ **唯一的 35 B 原始证据未被覆盖而消失**：现有 2 份独立副本，均在 git 之外的受控归档目录内。

---

## 6. Reconstructed Reports（Step 4）

| # | Canonical Filename | 位置 | 大小 | 顶部标注 |
|---|---|---|---|---|
| 1 | `PHASE-REORG-1G-FINAL-GAP-AUDIT-REPORT.md` | 仓库根 | 8,969 B | `RECONSTRUCTED FROM EVIDENCE` |
| 2 | `PHASE-REORG-1J-G08-CONTRACT-EXCEPTION-FINAL-REPORT.md` | 仓库根 | 8,497 B | `RECONSTRUCTED FROM EVIDENCE` |
| 3 | `PHASE-REORG-1J-G09-TRIPWIRE-CONVERSION-FINAL-REPORT.md` | 仓库根 | 11,432 B | `RECONSTRUCTED FROM EVIDENCE` |
| 4 | `PHASE-REORG-1J-R3-LEGACY-PREFIX-CRASH-MATRIX-FINAL-REPORT.md` | 仓库根 | 10,489 B | `RECONSTRUCTED FROM EVIDENCE` |
| 5 | `PHASE-REORG-1J-R4A-PRE-GATE-FINAL-REPORT.md` | 仓库根 | 9,220 B | `RECONSTRUCTED FROM EVIDENCE` |
| 6 | `PHASE-REORG-1J-R4A-FINAL-REPORT.md` | `docs/` | 7,910 B | `RECONSTRUCTED FROM EVIDENCE` |

**6/6 均含强制声明**（逐文件核验）：

```text
RECONSTRUCTED FROM EVIDENCE
This document is a reconstructed archival artifact. It is NOT the original execution report.
Original execution report was not recoverable from Git history.
This reconstruction is based only on surviving evidence.
```

**6/6 均含 `EVIDENCE INSUFFICIENT` 字段清单**，对无法证实的内容一律留白，禁止填充。

**写入前路径 Gate**：5 个目标路径在 rename 后**已空缺**（可直接写入）；第 3 项（G09）路径原为 35 B 空壳，按 §5 备份通过后才写入。**无一处覆盖既有报告正文。**

---

## 7. Evidence Provenance

### 7.1 主证据源

| 源 | 类型 | 用途 |
|---|---|---|
| `.workbuddy/memory/2026-09-15.md` | 当日工作日志（395 行） | 6 份重建报告的**主要事实来源**，逐阶段精确到行号区间 |
| `PHASE-REORG-1J-R4C-FINAL-CLOSURE-AUDIT-REPORT.md` | ORIGINAL 报告（29,148 B） | 八阶段交付状态复核（§3 / §3.1）、R3↔R4A↔R4B 一致性（§4）、字节账目（§4.3） |
| `PHASE-REORG-1J-DOC-2-EVIDENCE-ARCHIVE-CLOSURE-AUDIT-REPORT.md` | ORIGINAL 报告（26,975 B） | 36 份归档审计矩阵、重建授权、canonical filename |
| `PHASE-REORG-1J-R4B-PRE-GATE-FINAL-REPORT.md` | ORIGINAL 报告（25,298 B） | 阶段链上下文 |
| `PHASE-REORG-1H-FINAL-REPORT.md` | ORIGINAL 报告（20,747 B） | GAP-1H-A 与 2 个 tripwire 的立项来源 |
| 源码 / 测试文件 | 可验证代码 | 行号级交叉佐证（见 7.2） |
| `docs/_archive_pre-doc2/` | 快照 | 被覆盖正文的不可变副本 |

### 7.2 逐报告 evidence 定位（摘要；完整版见各报告 §Evidence Sources）

| 报告 | 关键 evidence 定位 |
|---|---|
| **1G** | `2026-09-15.md:236-258`（阶段标题 / VERDICT=PASS WITH BLOCKING GAPS / 阻塞 B-1..B-6 / §10 部署落后 6 commit / Stage A-C）；`internal/blocktree/settip.go:26`；R4C §3.1 行 71 |
| **G08** | `internal/storage/v2_test.go:618-641`（CONTRACT EXCEPTION 注释块）、`:495` / `:573` / `:643`（3 个 `TestG08_*`）；R4C §3 行 63、65，§3.1 行 74；`2026-09-15.md:313`、`:373` |
| **G09** | `cmd/node/p2p_branch_test.go:588-633`（R1 注释块 + 10 条 acceptance chain + 用例行 629）；`cmd/node/p2p_branch_process_test.go:300-314`（R2 四段链 + 用例行 314）；R4C §3.1 行 75；`2026-09-15.md:299`、`:313`、`:355`、`:373` |
| **R3** | `2026-09-15.md:282-302`（全段）；`internal/storage/v2.go:713`（`appendFrames` 单 fsync）；`internal/storage/v2api.go:58`（`commitTipAfterAppend`）；`r3_crash_matrix_test.go` 8 个用例行号；`cmd/node/r3_crash_restart_test.go:63`；R4C §3 行 59、§3.1 行 76、§4.2 |
| **R4A PRE-GATE** | `2026-09-15.md:306-330`（全段：A–D 审计 / 两个关键新发现 / 15 适用 3 N/A / 成本基线 / 9 条 STOP）；`settip.go:26`；`r3_crash_matrix_test.go:168`；R4C §3.1 行 77 |
| **R4A FINAL** | `2026-09-15.md:331-339`（全段：根因 k=165 / 18,247 崩溃点 / I9 15-15 / I10 225 次 / I12 6×5 / 回归 / 生产 diff 硬证据）；`r4a_legacy_length_matrix_test.go` 11 个用例行号 + 行 26 约束；`v2.go:305/337`；R4C §3.1 行 77、§4.2 |

### 7.3 本阶段复核中发现的 evidence 行号漂移（诚实登记）

| 记录位置 | 记录值 | 本阶段实测 | 判定 |
|---|---|---|---|
| `2026-09-15.md:313` | `p2p_branch_test.go:593` | 注释块行 **590**、函数行 **629** | 同一用例，行号漂移（后续追加内容所致） |
| `2026-09-15.md:318` | `r3_crash_matrix_test.go:172` | 注释行 **168** | 同一注释，行号漂移 4 行 |

### 7.4 本阶段复核中发现的 evidence 命名矛盾（诚实登记）

| 来源 | 对 G09 R1 的命名 |
|---|---|
| `2026-09-15.md:299` | `TestG08_LegacyImmutableAndTruncateGuard` |
| `2026-09-15.md:373` + R4C §3.1 行 75 | `TestBranchLegacyPrefixReorgConvergesToLongerBranch` |

裁定：以 `373` + R4C §3.1 行 75（两个互相独立的来源，且一致）为准；`299` 系记录笔误。
`2026-09-15.md:313` 亦将 `TestG08_LegacyImmutableAndTruncateGuard` 归入 **G08 R1**，与该裁定自洽。
**矛盾不影响结论**（两种记法下 R1 均为 PASS）。已同步写入 G09 与 R3 重建报告。

---

## 8. 1I — LOST / REFERENCE-ONLY

```text
STATUS : LOST / REFERENCE-ONLY
ACTION : NOT RECONSTRUCTED (BY RULE A)
```

| 项 | 内容 |
|---|---|
| 登记文件 | `docs/_archive_pre-doc2/reference-only/REORG-1I-LOST-REFERENCE-ONLY.md` |
| 不重建理由 | 除 1J prompt「前置报告」段转述外，**无执行日志 / 无测试产物 / 无 commit / 无实测输出** |
| **1I 编号歧义（必须并存登记）** | ① WBS 原义 `1I = finality / MaxReorgDepth` —— **未执行，DEFERRED**（唯一落地物 `settip.go:26`）<br>② 后期 `1I = LEGACY/V2 CANONICAL REORG STORAGE AUDIT` —— **执行过，报告正文被 1J prompt 覆盖** |
| 允许保留的引用（6 项，全部可定位） | 阶段名（`REORG-1J.prompt.md:9`）、`NOT READY FOR IMPLEMENTATION`（`:13`）、`OPTION A / CANONICAL VIEW UNIFICATION`（`:15-17`）、`GAP-1I-A`（`:31`，§4 于 `:97`）、`GAP-1I-B`（`:32`，§5 于 `:151`）、`GAP-1I-C/D/E`（`:33-35`，closure 清单 `:642-646`） |
| 源副本 | `docs/_archive_pre-doc2/reference-only/PHASE-REORG-1I-…-FINAL-REPORT.md`（12,020 B，`e4cf2dd0…86af3`，与 `REORG-1J.prompt.md` 逐字节相同） |
| **明确禁止** | ❌ 不得把 1J prompt 重新包装成 1I FINAL REPORT<br>❌ 不得补写执行过程 / 测试结果 / 时间戳<br>❌ 不得在 R5 把 1I 记为「已交付」 |
| R5 必须登记的表述 | 「1I 原始报告已丢失，其结论以 1J prompt 的『前置报告』段为唯一引用来源。」 |

---

## 9. File Integrity Verification

### A. 文件完整性

| 检查 | 结果 |
|---|---|
| 快照文件在位 | **39/39 PASS** |
| 快照哈希 vs `MANIFEST.tsv` | **36/37 一致** + 1 条已定位的清单过期（§3.2，非数据丢失） |
| 现存源文件 vs 快照 | **14/14 PASS** |
| rename 目标哈希 vs 原始 | **25/25 PASS**（`RENAME-MAP.FINAL.tsv` 逐条 `OK`） |
| rename 源已消失 | **25/25**（其中 5 个 canonical 路径随后被 Step 4 的 `RECONSTRUCTED` 报告合法重建，非残留） |
| G09 空壳备份 | **PASS**（原始 / 备份① / 快照② 三方哈希一致） |
| 未授权覆盖 | **0**（全程无 overwrite：每个 rename 前均验证 dest 不存在） |

### B. 重建分类计数

```text
ORIGINAL            = 10
PROMPT-ARCHIVE      = 25
EMPTY SHELL         =  1
RECONSTRUCTED       =  6
LOST / REFERENCE-ONLY = 1
────────────────────────
合计（DOC-2 受审口径）= 43 条目 / 36 原始文件
```

**附（不属 DOC-2 口径）**：`docs/prompts/REORG-DOC-2-REPAIR.prompt.md`（本阶段 prompt 被覆盖进 DOC-2 报告路径后的存档，8,810 B，`d6dd36ad…a001db4`）。

**逐类实测**：

| 类别 | 实测 | 明细 |
|---|---|---|
| ORIGINAL | **10** | 仓库 4：`1F-IMPLEMENTATION-1` / `1H` / `R4B` / `R4C`；工作区根 6：`1B-COMMIT-READINESS-AUDIT` / `1E-IMPL-1-CONTRACT-CONFLICT` / `1E-PRE-IMPL-DESIGN-AUDIT-1` / `INFRA-DESIGN-1` / `INFRA-IMPL-1(1B)` / `INFRA-IMPL-2(1D)` |
| PROMPT-ARCHIVE | **25** | `docs/prompts/*.prompt.md` 剔除 `REORG-DOC-2-REPAIR.prompt.md` 后为 **25**（目录共 27 项：25 + 1 阶段 prompt + 1 空壳备份 `.txt`） |
| EMPTY SHELL | **1** | `docs/prompts/REORG-G09-empty-shell.original.txt`（35 B） |
| RECONSTRUCTED | **6** | 见 §6，6/6 首行标注正确 |
| LOST | **1** | 1I（见 §8） |

**数量矛盾检查**：✅ 无矛盾（10 + 25 + 1 = 36 = DOC-2 受审文件总数；RECONSTRUCTED 6 与 LOST 1 为新建/登记条目，不占原始 36 份）。

### C. Filename / Content Audit

| 类别 | 文件名声称 | 正文实际 | 可区分 |
|---|---|---|---|
| ORIGINAL（10） | = 阶段 X | = 阶段 X 报告 | ✅ 一致 |
| PROMPT-ARCHIVE（25） | 阶段 X | 阶段 X+1 的 prompt | ✅ 已 rename 为 `REORG-<真实阶段>.prompt.md`，文件名与正文一致 |
| EMPTY SHELL（1） | G09 | 35 B skill 名 | ✅ 以 `.original.txt` 独立保存，与重建报告分离 |
| RECONSTRUCTED（6） | = 阶段 X | = 阶段 X 重建报告 + 顶部 `RECONSTRUCTED FROM EVIDENCE` | ✅ 顶部标注 + 文件头声明，与 ORIGINAL 明确可区分 |
| LOST（1） | 1I | — | ✅ 独立登记表，无同名报告 |

实测复核：6 份重建报告首行分别为
`# PHASE REORG-1G — FINAL GAP AUDIT REPORT` / `# PHASE REORG-1J-G08 — CONTRACT EXCEPTION FINAL REPORT` /
`# PHASE REORG-1J-G09 — TRIPWIRE CONVERSION FINAL REPORT` / `# PHASE REORG-1J-R3 — LEGACY-PREFIX CRASH MATRIX FINAL REPORT` /
`# PHASE REORG-1J-R4A — PRE-GATE FINAL REPORT` / `# PHASE REORG-1J-R4A — IMPLEMENTATION-1 FINAL REPORT`
—— **全部与 canonical filename 声称的阶段一致**，未重演 off-by-one。

### D. Production Safety

| 检查 | 结果 |
|---|---|
| 16 个生产/测试代码文件 SHA-256（前后比对） | **16/16 完全相同** |
| `git diff --stat -- cmd internal` | 9 文件 / +844 / −70 —— **与阶段起点逐项一致**（`main.go` 14 / `service.go` 462 / `blockchain.go` 19 / `node.go` 95 / `node_test.go` 119 / `file.go` 8 / `v2.go` 44 / `v2_test.go` 98 / `v2api.go` 55），**本阶段新增 0 行** |
| consensus diff | **0** |
| storage diff | **0** |
| P2P diff | **0** |
| reorg execution / UTXO / TIP·commit semantics | **0**（未触碰） |

### E. Git Safety

| 检查 | 结果 |
|---|---|
| HEAD | `e04d678b4fb33a14ea186999375f6d6dac2a4c4b`（**未变**） |
| branch | `main`（未变） |
| commit | **NO**（`git rev-list --all --count` = **35**，与 DOC-2 基线一致） |
| reflog 末条 | `e04d678 commit: feat: restore transactions after chain reorganization`（**无新条目**） |
| staged | **0** |
| push | **NO**（remote `gitea` 未触碰） |
| tag | **NO**（`git tag` = 0） |
| merge / rebase / amend / squash | **NO**（`.git/rebase-merge` / `rebase-apply` / `MERGE_HEAD` / `CHERRY_PICK_HEAD` / `BISECT_LOG` 全部 absent） |
| stash | 0（未新增） |
| deploy / Windows mining | **NO** |

---

## 10. Production-Code Zero-Diff Verification

见 §9 D。硬证据：

```text
[2026-09-15T17:00:03+08:00] 采集 16 个生产/测试文件 SHA-256 → /tmp/prod-baseline.txt
[阶段结束]                  重算同一 16 个文件        → /tmp/prod-after.txt
diff → 空 ⇒ 16/16 完全相同
```

覆盖：`cmd/node/main.go`、`cmd/node/service.go`、`internal/blockchain/blockchain.go`、`internal/blockchain/query.go`、
`internal/p2p/node.go`、`internal/p2p/node_test.go`、`internal/storage/file.go`、`internal/storage/v2.go`、
`internal/storage/v2_test.go`、`internal/storage/v2api.go`、
`cmd/node/p2p_branch_process_test.go`、`cmd/node/p2p_branch_test.go`、`cmd/node/r3_crash_restart_test.go`、
`cmd/node/r4b_dual_reorg_convergence_test.go`、`internal/storage/r3_crash_matrix_test.go`、`internal/storage/r4a_legacy_length_matrix_test.go`。

---

## 11. Git Safety Verification

见 §9 E。**结论：本阶段对 Git 的全部写操作为 0。**
唯一变化是新增未跟踪文件（快照目录 / prompts 目录 / 6 份重建报告 / 1 份 LOST 登记表 / 本最终报告），**全部未 `git add`**。

---

## 12. Remaining Archive Issues

| # | 问题 | 严重度 | 状态 |
|---|---|---|---|
| 1 | **Step 5 未执行**：工作区根 `挖矿\` 仍有 6 份 ORIGINAL REORG 报告在 **git 之外**（该目录非 git 仓库） | P2 | **按授权不执行**，需单独授权 |
| 2 | **Step 6 未执行**：1I 编号歧义未消歧（建议 ②重编号为 `REORG-1J-PRE-GATE`） | P2 | **按授权不执行**，已登记于 §8 |
| 3 | DOC-2 报告 16:04 原版 **25,664 B 字节序列永久不可恢复**（快照建立于 16:10，晚于 16:08:36 的覆盖） | P2 | 不可恢复，已记录 |
| 4 | 6 份报告为 RECONSTRUCTED，**永久不等于原始执行报告** | P2 | 不可恢复，已标注 |
| 5 | 1 份（1I）永久 LOST | P2 | 不可恢复，已登记 |
| 6 | `MANIFEST.tsv` DOC-2 条目过期（8,810 B vs 26,975 B） | P3 | 已由 `MANIFEST.verified.tsv` 修正；原文件保留未动 |
| 7 | evidence 行号漂移 2 处（`:593`→`:590/:629`、`:172`→`:168`） | P3 | 已登记于 §7.3 |
| 8 | evidence 命名矛盾 1 处（G09 R1，`2026-09-15.md:299` vs `:373`） | P3 | 已裁定并登记于 §7.4 |
| 9 | DOC-2 报告自身 §4.1 小计与表体计数不一致（小计「4 PASS / 16 WRONG」，表体实为 15 WRONG + 1 MISSING） | P3 | **未修改 DOC-2 报告**（超出本阶段授权） |
| 10 | **防复发机制未落地**：阶段报告写完后立即快照尚未成为强制流程 | P1 | DOC-2 §8 Step 8 已有建议；需下一层治理落实 |

---

## 13. Recommended Next Step

```text
唯一推荐：
REORG-1J-DOC-2-ARCHIVE-GOVERNANCE（DOC-2 Step 5 + Step 6，需单独授权）
  ① 工作区根 16 份 REORG 报告迁移至 p2pchain/docs/phases/（解决 6 份 ORIGINAL 仍在 git 之外）
  ② 1I 编号消歧：②重编号为 REORG-1J-PRE-GATE，并在 MEMORY 固化
  ③ 落地「报告写完即快照」的强制流程

可并行（不阻塞）：
REORG-1J-R5 — COMMIT ORGANIZATION
  前提：R5 提交说明必须显式登记
    - 6 份 RECONSTRUCTED FROM EVIDENCE 报告（不得记为原始报告）
    - 1 份 LOST（1I）
    - 本阶段遗留 10 项归档问题
    - 全部 DEFERRED 项（BT-1 / OBS-1J-R3-A / R4A-OBS-1 / finality·MaxReorgDepth / orphan pool / fork 观测）
    - 仍须排除长期 dirty：docs/DETERMINISTIC-SERIALIZATION-SPEC.md、
      docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md、run-a/、run-b/、verifier/
```

**技术闭环不受影响（Rule C / Rule D 复核）**：本阶段为纯 documentation / evidence archive 操作，
未触碰 consensus / storage / P2P / reorg execution / UTXO / TIP·commit semantics。
R4C 的 `CONDITIONALLY CLOSED / R5 READY` 与 `GAP-1H-A = CLOSED` **继续成立**。

---

## 14. STOP

```text
SNAPSHOT FILES                : 39 (repo 21 + ws-root 16 + reference-only 2)
PROMPT RENAMES                : 25 / 25 HASH-VERIFIED
FILES OVERWRITTEN (UNAUTH)    : 0
FILES DELETED                 : 0
EMPTY-SHELL BACKUPS           : 2 (35 B, hash-identical)
REPORTS RECONSTRUCTED         : 6
REPORTS MARKED LOST           : 1 (1I)
PRODUCTION CODE DIFF          : 0 (16/16 file hashes identical)
TEST CODE DIFF                : 0
CONSENSUS / STORAGE / P2P DIFF: 0 / 0 / 0
COMMIT                        : NO
PUSH                          : NO
TAG                           : NO
MERGE / REBASE / AMEND / SQUASH : NO
DEPLOY                        : NO
WINDOWS MINING                : NO
```

本阶段到此结束。

**不进入** DOC-2 Step 5 / Step 6、不进入 REORG-1J-R5、不 commit、不 push、不部署、不跑 Windows 挖矿。
等待下一授权。

> **本文件按 DOC-2 §8 Step 8 要求，写入后立即建立独立快照**（见 `docs/_archive_pre-doc2/`），
> 以防下一阶段 prompt 覆盖本报告路径 —— 该破坏机制已在 DOC-2 §0 被当场实证。

---

## 附：本阶段新增/改动文件清单（全部未 git add）

| 路径 | 类型 |
|---|---|
| `docs/_archive_pre-doc2/RENAME-MAP.FINAL.tsv` | 新建（25 条 rename 最终清单） |
| `docs/_archive_pre-doc2/MANIFEST.verified.tsv` | 新建（39 条重算清单，不覆盖 MANIFEST.tsv） |
| `docs/_archive_pre-doc2/reference-only/` | 新建目录（1I 源副本 + LOST 登记表） |
| `docs/prompts/REORG-1A.prompt.md` | rename 来源 |
| `docs/prompts/REORG-G09-empty-shell.original.txt` | 新建（35 B 空壳备份） |
| `PHASE-REORG-1G-FINAL-GAP-AUDIT-REPORT.md` | 新建（RECONSTRUCTED） |
| `PHASE-REORG-1J-G08-CONTRACT-EXCEPTION-FINAL-REPORT.md` | 新建（RECONSTRUCTED） |
| `PHASE-REORG-1J-G09-TRIPWIRE-CONVERSION-FINAL-REPORT.md` | 35 B 空壳 → RECONSTRUCTED（备份先行） |
| `PHASE-REORG-1J-R3-LEGACY-PREFIX-CRASH-MATRIX-FINAL-REPORT.md` | 新建（RECONSTRUCTED） |
| `PHASE-REORG-1J-R4A-PRE-GATE-FINAL-REPORT.md` | 新建（RECONSTRUCTED） |
| `docs/PHASE-REORG-1J-R4A-FINAL-REPORT.md` | 新建（RECONSTRUCTED） |
| `PHASE-REORG-1J-DOC-2-REPAIR-FINAL-REPORT.md` | 本文件 |
