# ORIGINAL → REFERENCE-ONLY / ARCHIVE GOVERNANCE INDEX

| 元数据字段 | 值 |
|---|---|
| **phase** | `REORG-1J-DOC-2-ARCHIVE-GOVERNANCE`（Step 5） |
| **execution date** | 2026-09-15 |
| **baseline HEAD** | `e04d678b4fb33a14ea186999375f6d6dac2a4c4b`（本阶段全程未变） |
| **scope** | 工作区根 `C:\Users\Administrator\Desktop\挖矿\` 遗留的 **6 份 ORIGINAL** REORG 报告的 archive / reference-only 治理 |
| **source evidence** | 本阶段 17:38 只读基线枚举（文件名 / SHA-256 / size / git 状态 / 归档对应物 / 重复扫描 / 与 reconstructed 混淆风险），逐项见 §3–§6 |
| **classification** | `ORIGINAL`（DOC-2 §7 定义：正文 = 文件名声称阶段，可直接采信） |
| **governance decision** | **MOVE** 至 `p2pchain/docs/phases/`（非 copy、非 delete），source hash == destination hash 逐份验证 |
| **limitations** | 见 §8 |

---

## 1. Governance Decision

```text
DECISION : MOVE (not copy, not delete)
FROM     : C:\Users\Administrator\Desktop\挖矿\<name>
TO       : C:\Users\Administrator\Desktop\挖矿\p2pchain\docs\phases\<name>
RESULT   : 6 / 6 MOVE-OK（source hash == destination hash，source ABSENT，size 不变）
```

**决策理由**：

1. 工作区根 `挖矿\` **不是 git 仓库**（本阶段实测 `git rev-parse --show-toplevel` → `fatal: not a git repository`）。
   这 6 份是 REORG-1J 证据链中**仅存的、仍在版本控制之外**的 ORIGINAL（DOC-2 §2 第 2 条、§8 Step 5）。
2. 移入 `p2pchain/docs/phases/` 后进入 git 工作树，R5 可显式 `git add` ⇒ 关闭「零版本化」缺陷。
3. `docs/phases/` 在移动前**不存在**（实测 `ls -d docs/phases` → No such file or directory）⇒ **不可能覆盖任何已有 evidence**。
4. 每份文件在 `docs/_archive_pre-doc2/workspace-root/` 已存在 **byte-identical 快照**（6/6 `COUNTERPART-MATCH`）⇒ 移动后证据仍保有 **2 份**独立副本，原始位置与内容均有不可变记录。
5. 选择 MOVE 而非 COPY：单一定位，避免制造「两份 canonical」的歧义（§3 原则 5）。

**未违反的 §3 原则**：

| 原则 | 合规 |
|---|---|
| 1. 原始 evidence 优先保留 | ✅ 移动后仍有 2 份（phases + 快照），内容字节不变 |
| 2. 不覆盖已有 archive | ✅ 目标目录不存在；`_archive_pre-doc2/` 未被触碰 |
| 3. 不改变 hash | ✅ 6/6 source == destination |
| 4. 不改变内容 | ✅ size 逐份不变 |
| 5. 不允许制造新的 canonical original | ✅ 同名同内容重定位，未新建任何文件版本 |
| 6. 不允许把 reconstructed 冒充 original | ✅ 本阶段未触碰任何 reconstructed 报告 |
| 7. 移动须记录 source == destination hash | ✅ 见 §4 |
| 8. 复制须明确记录 copy 而非 move | ✅ 不适用（本次为 move，已明确记录） |
| 9. 无法安全归档须 STOP | ✅ 6/6 安全完成，无一例需 STOP |

---

## 2. Read-Only Baseline Enumeration（移动前）

| # | 文件名 | size | git 状态 | 归档对应物是否存在 | 是否与 reconstructed 同名 |
|---|---|---|---|---|---|
| 1 | PHASE-REORG-1B-COMMIT-READINESS-AUDIT-REPORT.md | 20,352 | **NOT IN ANY GIT REPO**（`挖矿\` 非仓库） | ✅ `workspace-root/` 同名，`COUNTERPART-MATCH` | ❌ 否 |
| 2 | PHASE-REORG-1E-IMPLEMENTATION-1-CONTRACT-CONFLICT-REPORT.md | 14,986 | 同上 | ✅ MATCH | ❌ 否 |
| 3 | PHASE-REORG-1E-PRE-IMPLEMENTATION-DESIGN-AUDIT-1-FINAL-REPORT.md | 53,969 | 同上 | ✅ MATCH | ❌ 否 |
| 4 | PHASE-REORG-INFRASTRUCTURE-DESIGN-1-REPORT.md | 31,927 | 同上 | ✅ MATCH | ❌ 否 |
| 5 | PHASE-REORG-INFRASTRUCTURE-IMPLEMENTATION-1-REORG-1B-FINAL-REPORT.md | 17,135 | 同上 | ✅ MATCH | ❌ 否 |
| 6 | PHASE-REORG-INFRASTRUCTURE-IMPLEMENTATION-2-REORG-1D-FINAL-REPORT.md | 28,062 | 同上 | ✅ MATCH | ❌ 否 |

**重复扫描**：`find . -name <each>` 在整个 `挖矿\` 下仅命中 **2 处/份**（工作区根 + `workspace-root/` 快照），**无第三份重复**。

**与 reconstructed 混淆风险**：6 份文件名与 6 份 RECONSTRUCTED 报告
（`1G` / `G08` / `G09` / `R3` / `R4A-PRE-GATE` / `R4A-FINAL`）**无一名重合** ⇒ 无混淆风险。

---

## 3. ORIGINAL → REFERENCE-ONLY / ARCHIVE MATRIX

| # | 文件名 | size | SHA-256 | 原位置 | 归档位置 | 快照位置 | 分类 |
|---|---|---|---|---|---|---|---|
| 1 | PHASE-REORG-1B-COMMIT-READINESS-AUDIT-REPORT.md | 20,352 | `0805d4a099e1774201649209c91d2997ec64d00a6d710dea6d2f0a2dbe05efed` | `挖矿\` | `p2pchain/docs/phases/` | `_archive_pre-doc2/workspace-root/` | **ORIGINAL** |
| 2 | PHASE-REORG-1E-IMPLEMENTATION-1-CONTRACT-CONFLICT-REPORT.md | 14,986 | `e67250bee9572ad4f26fed784caf1fce7f79ad940ffc3ea6ac644d5862a21771` | `挖矿\` | `p2pchain/docs/phases/` | `_archive_pre-doc2/workspace-root/` | **ORIGINAL** |
| 3 | PHASE-REORG-1E-PRE-IMPLEMENTATION-DESIGN-AUDIT-1-FINAL-REPORT.md | 53,969 | `bfa1ab15675a5634b60ff84c0a423f53e7563c8e9d2c2bb9ad14bbaec17e2e2d` | `挖矿\` | `p2pchain/docs/phases/` | `_archive_pre-doc2/workspace-root/` | **ORIGINAL** |
| 4 | PHASE-REORG-INFRASTRUCTURE-DESIGN-1-REPORT.md | 31,927 | `6cc4a076439a5c9b83d4475147d0be38b3c55293d917f5b47a9286f6fac15736` | `挖矿\` | `p2pchain/docs/phases/` | `_archive_pre-doc2/workspace-root/` | **ORIGINAL** |
| 5 | PHASE-REORG-INFRASTRUCTURE-IMPLEMENTATION-1-REORG-1B-FINAL-REPORT.md | 17,135 | `32de847862d129523a61f405aa8c121e79da6cf0208a88af510d2806acc3b6d9` | `挖矿\` | `p2pchain/docs/phases/` | `_archive_pre-doc2/workspace-root/` | **ORIGINAL** |
| 6 | PHASE-REORG-INFRASTRUCTURE-IMPLEMENTATION-2-REORG-1D-FINAL-REPORT.md | 28,062 | `677117e41c2a782a3755a21b5ea84c82f21f120e9dd4b1b67171af5ef0ddf4de` | `挖矿\` | `p2pchain/docs/phases/` | `_archive_pre-doc2/workspace-root/` | **ORIGINAL** |

**分类一致性**：6 份仍为 `ORIGINAL`，**未被降级为 REFERENCE-ONLY，也未被升级/改造**。
「REFERENCE-ONLY / ARCHIVE」在本阶段指的是**归档治理动作与受控存放位置**，不是分类降级 ——
这与 `REORG-1I` 的 `LOST / REFERENCE-ONLY`（因证据不足而不可采信）**语义不同，不得混用**。

---

## 4. Move Verification（source hash == destination hash）

| # | 文件 | pre-move SHA-256（前 12） | post-move SHA-256（前 12） | size | source 移动后 | 判定 |
|---|---|---|---|---|---|---|
| 1 | PHASE-REORG-1B-COMMIT-READINESS-AUDIT-REPORT.md | `0805d4a099e1` | `0805d4a099e1` | 20,352 | ABSENT | **MOVE-OK** |
| 2 | PHASE-REORG-1E-IMPLEMENTATION-1-CONTRACT-CONFLICT-REPORT.md | `e67250bee957` | `e67250bee957` | 14,986 | ABSENT | **MOVE-OK** |
| 3 | PHASE-REORG-1E-PRE-IMPLEMENTATION-DESIGN-AUDIT-1-FINAL-REPORT.md | `bfa1ab15675a` | `bfa1ab15675a` | 53,969 | ABSENT | **MOVE-OK** |
| 4 | PHASE-REORG-INFRASTRUCTURE-DESIGN-1-REPORT.md | `6cc4a076439a` | `6cc4a076439a` | 31,927 | ABSENT | **MOVE-OK** |
| 5 | PHASE-REORG-INFRASTRUCTURE-IMPLEMENTATION-1-REORG-1B-FINAL-REPORT.md | `32de847862d1` | `32de847862d1` | 17,135 | ABSENT | **MOVE-OK** |
| 6 | PHASE-REORG-INFRASTRUCTURE-IMPLEMENTATION-2-REORG-1D-FINAL-REPORT.md | `677117e41c2a` | `677117e41c2a` | 28,062 | ABSENT | **MOVE-OK** |

**逐份 Gate**：`source exists = Y` ∧ `destination not exists = Y`（移动前）→ `mv` → `source ABSENT` ∧ `destination exists` ∧ `hash 不变` ∧ `size 不变`。
**结果：6/6 PASS，0 例 GATE-FAIL，0 例 MOVE-FAIL。**

---

## 5. 治理后的工作区根状态

| 项 | 移动前 | 移动后 |
|---|---|---|
| `挖矿\` 下 `PHASE-REORG-*.md` | **6** | **0** |
| `p2pchain/docs/phases/` | 不存在 | **6** |
| `_archive_pre-doc2/workspace-root/` | 16（不变） | 16（不变） |

⇒ 工作区根已无 REORG 命名文件；全部 ORIGINAL 现位于 git 工作树内（`p2pchain/docs/phases/`）+ 不可变快照内。

---

## 6. R5 影响

| 项 | 说明 |
|---|---|
| 是否可直接 `git add` | ✅ `p2pchain/docs/phases/*.md` 位于 git 工作树内，R5 可显式逐份 `git add` |
| `.gitignore` 是否屏蔽 | DOC-2 §5 已验证 `git check-ignore` 未命中（`docs/` 下报告不被忽略） |
| 是否需要改任何报告内容 | ❌ 不需要，内容字节未变 |
| 是否制造新的 canonical original | ❌ 没有 |

---

## 7. Source Evidence（本阶段采集）

| 证据 | 内容 |
|---|---|
| 基线时间 | 2026-09-15T17:38:42+08:00 |
| `git rev-parse HEAD` | `e04d678b4fb33a14ea186999375f6d6dac2a4c4b` |
| `git diff --cached --name-only` | **0**（无 staged） |
| `cd .. && git rev-parse --show-toplevel` | `fatal: not a git repository` ⇒ **工作区根非 git 仓库** |
| `ls -d docs/phases` | `No such file or directory`（移动前） |
| 6 份文件 SHA-256 / size | §3 |
| 归档对应物比对 | 6/6 `COUNTERPART-MATCH` |
| 重复扫描 | 每份仅 2 处，无第三份 |
| 移动验证 | §4，6/6 `MOVE-OK` |

---

## 8. Limitations

1. 本阶段**未**把工作区根其余非 REORG 阶段报告纳入治理（DOC-2 §2 提到工作区根共 57 份阶段报告在 git 外）—— 超出本阶段授权。
2. 本阶段**未**执行 `git add`，6 份文件仍为 **untracked**；版本化要等 R5 显式提交。
3. 移动后 `挖矿\` 下不再有任何 `PHASE-REORG-*.md`；若外部脚本/文档硬编码了原路径，需按 §3 矩阵更新引用。
4. 本索引只记录**治理动作与哈希**，不重新解读任何报告正文；不得据本文件推断报告内容。
5. 本文件为 **documentation governance metadata**，不得作为任何阶段的技术证据使用。
