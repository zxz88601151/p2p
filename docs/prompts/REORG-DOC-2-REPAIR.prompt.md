# PHASE REORG-1J-DOC-2-REPAIR — EVIDENCE ARCHIVE SAFE REPAIR

## 阶段性质

**STRICT CONTROLLED DOCUMENTATION / EVIDENCE ARCHIVE REPAIR**

本阶段只处理 `PHASE REORG-1J-DOC-2 — EVIDENCE ARCHIVE / REPORT INTEGRITY CLOSURE AUDIT` 已确认的 documentation / evidence archive 问题。

**绝对禁止触碰 production code、测试代码、consensus、storage、P2P、reorg execution、UTXO、TIP/commit semantics。**

---

# 1. 基线

使用 DOC-2 报告确认的基线：

```text
HEAD = e04d678
```

执行开始后首先重新验证：

```text
git status --short
git rev-parse HEAD
git diff --cached --stat
git diff --stat
```

若 HEAD、工作区或 staging 与 DOC-2 基线存在无法解释的变化：

**STOP。**

---

# 2. 本阶段唯一目标

安全完成 DOC-2 的：

1. 现有报告保护快照；
2. Prompt 文件 rename 归档；
3. G09 35-byte empty shell 安全备份；
4. 重建 6 份有充分 evidence 支撑的 FINAL REPORT。

本阶段：

```text
DO NOT COMMIT
DO NOT PUSH
DO NOT TAG
DO NOT MERGE
DO NOT REBASE
DO NOT AMEND
DO NOT SQUASH
DO NOT DEPLOY
DO NOT RUN WINDOWS MINING
DO NOT MODIFY PRODUCTION CODE
DO NOT MODIFY TEST CODE
```

---

# 3. HARD SAFETY RULES

## Rule A — 不得伪造历史

任何无法由现存 evidence 支撑的内容：

```text
EVIDENCE INSUFFICIENT
```

不得根据经验、上下文或阶段名称自行补写。

所有重建报告必须在正文顶部明确写：

```text
RECONSTRUCTED FROM EVIDENCE
```

并说明：

```text
This document is a reconstructed archival artifact.
It is NOT the original execution report.
```

---

## Rule B — 不得覆盖原文件

任何 rename / replacement 操作之前：

**必须先完成完整快照。**

禁止：

```text
overwrite
delete
truncate
in-place replacement
```

尤其禁止直接把 G09 的 35-byte 文件覆盖成 reconstructed report。

---

# 4. STEP 1 — FULL ARCHIVE SNAPSHOT

在任何 rename / reconstruction 之前，对 DOC-2 识别出的全部 36 份 REORG 文件建立不可变的工作区快照。

至少包含：

```text
relative path
filename
byte size
SHA-256
full original content
```

推荐建立：

```text
docs/_archive_pre-doc2/
```

或等价的独立 tar/archive。

注意：

**快照必须在任何文件移动、rename、覆盖之前完成。**

完成后重新计算并输出 36 份文件的：

```text
path
size
sha256
```

并验证快照中的 SHA-256 与原始文件逐一一致。

若任意一个文件：

```text
missing
size mismatch
hash mismatch
content mismatch
```

立即 STOP。

---

# 5. STEP 2 — PROMPT ARCHIVE

DOC-2 已确认：

```text
25 files = PROMPT-ARCHIVE
```

必须：

**RENAME，不删除。**

统一归档到：

```text
p2pchain/docs/prompts/
```

文件名采用真实正文对应的阶段名称，例如：

```text
REORG-1G.prompt.md
REORG-1H.prompt.md
REORG-G08.prompt.md
REORG-R3.prompt.md
...
```

实际映射必须依据：

```text
正文标题
正文阶段编号
阶段目标
```

而不是仅依据当前错误文件名。

---

# 6. Rename Safety Gate

每一次 rename 前必须确认：

```text
source exists
destination does not exist
source SHA-256 already recorded
source content is preserved in pre-doc2 snapshot
```

rename 后立即验证：

```text
source absent
destination exists
destination SHA-256 == original SHA-256
destination content byte-identical
```

不得允许任何 overwrite。

如果目标文件已经存在：

**STOP，不覆盖。**

---

# 7. STEP 3 — G09 EMPTY SHELL

当前：

```text
PHASE-REORG-1J-G09-TRIPWIRE-CONVERSION-FINAL-REPORT.md
```

大小：

```text
35 bytes
```

首先把原始 35-byte 内容保存为：

```text
docs/prompts/REORG-G09-empty-shell.original.txt
```

该文件必须保留原始字节内容。

然后重新验证：

```text
sha256(original 35-byte file)
sha256(saved empty-shell backup)
```

必须完全一致。

**只有在上述验证 PASS 后，才允许建立 reconstructed G09 report。**

不得让 reconstructed report 覆盖唯一的 35-byte 原始证据而不留下备份。

---

# 8. STEP 4 — RECONSTRUCTED REPORTS

仅允许重建以下 6 份：

```text
1G
G08
G09
R3
R4A-PRE-GATE
R4A-FINAL
```

每份报告必须：

### Header

```text
RECONSTRUCTED FROM EVIDENCE
```

并明确：

```text
Original execution report was not recoverable from Git history.
This reconstruction is based only on surviving evidence.
```

---

# 9. Evidence Provenance

每一份 reconstructed report 必须包含：

```text
Evidence Sources
```

至少列出 DOC-2 已确认的 evidence 来源。

例如：

```text
.workbuddy/memory/2026-09-15.md
R4C final closure audit
relevant source/test files
surviving execution outputs
```

对于可以精确定位的 evidence：

必须记录具体：

```text
file
line/range
section
test/output identifier
```

不得只写：

```text
"based on memory"
```

---

# 10. Reconstruction Content Rule

只恢复能够被证据直接证明的内容：

```text
stage identity
date
baseline HEAD
verdict
scope
tests
measured outputs
blocking gaps
non-actions
stop condition
```

任何证据不足字段统一写：

```text
EVIDENCE INSUFFICIENT
```

禁止：

```text
guessing
backfilling
inventing timestamps
inventing commands
inventing test results
inventing commit hashes
inventing execution details
```

---

# 11. 1I SPECIAL RULE

不得重建：

```text
1I — LEGACY/V2 CANONICAL REORG STORAGE AUDIT
```

将其状态明确登记：

```text
LOST / REFERENCE-ONLY
```

原因：

现存 evidence 不足以形成可信的原始执行报告。

只能保留：

- 1J prompt 对该阶段的引用；
- `NOT READY FOR IMPLEMENTATION`；
- GAP-1I-A..E；
- 其他可以被直接证明的引用信息。

不得把 1J prompt 重新包装成 1I FINAL REPORT。

---

# 12. Canonical Filename Rule

重建后的 6 份报告使用 DOC-2 确认的 canonical filename。

不得为了方便修改 R4C 已经形成的引用关系。

至少包括：

```text
PHASE-REORG-1G-FINAL-GAP-AUDIT-REPORT.md
PHASE-REORG-1J-G08-CONTRACT-EXCEPTION-FINAL-REPORT.md
PHASE-REORG-1J-G09-TRIPWIRE-CONVERSION-FINAL-REPORT.md
PHASE-REORG-1J-R3-LEGACY-PREFIX-CRASH-MATRIX-FINAL-REPORT.md
PHASE-REORG-1J-R4A-PRE-GATE-FINAL-REPORT.md
PHASE-REORG-1J-R4A-FINAL-REPORT.md
```

如果目标路径已经存在：

**STOP，不覆盖。**

---

# 13. DO NOT DO STEP 5 / STEP 6 YET

本阶段暂不执行：

```text
工作区根 16 份报告全部迁移到 p2pchain/docs/phases/
```

以及：

```text
1I 编号重新设计 / 重编号
```

这两项属于下一层 archive governance cleanup。

本阶段优先确保：

**现有证据不再丢失。**

---

# 14. FINAL VALIDATION

完成 Step 1–4 后，必须执行只读验证。

## A. File integrity

验证：

```text
original snapshot hashes
renamed prompt hashes
empty-shell backup hash
```

全部一致。

---

## B. Reconstruction classification

最终明确：

```text
ORIGINAL = 10
PROMPT-ARCHIVE = 25
EMPTY SHELL = 1
RECONSTRUCTED = 6
LOST / REFERENCE-ONLY = 1
```

不得出现数量矛盾。

---

## C. Filename/content audit

重新扫描全部 REORG archive：

```text
filename stage
content stage
classification
```

确保：

```text
original reports
prompt archives
reconstructed reports
lost reference
```

均可明确区分。

---

## D. Production safety

确认：

```text
production code diff = 0
test code diff = 0
consensus diff = 0
storage diff = 0
P2P diff = 0
```

---

## E. Git safety

确认：

```text
HEAD unchanged
no commit
no push
no tag
no merge
no rebase
no amend
no squash
```

---

# 15. STOP CONDITION

本阶段结束时必须 STOP。

不得自动进入：

```text
DOC-2 Step 5
DOC-2 Step 6
R5 Commit Readiness
commit
deployment
Linux production validation
Windows mining
```

必须先输出最终报告。

---

# 16. FINAL REPORT REQUIRED

报告标题：

```text
PHASE REORG-1J-DOC-2-REPAIR — FINAL REPORT
```

必须包含：

1. VERDICT
2. Baseline
3. Snapshot Evidence
4. Rename Mapping
5. G09 Empty-Shell Preservation
6. Reconstructed Reports
7. Evidence Provenance
8. 1I LOST / REFERENCE-ONLY
9. File Integrity Verification
10. Production-Code Zero-Diff Verification
11. Git Safety Verification
12. Remaining Archive Issues
13. Recommended Next Step
14. STOP

最终结论只能从：

```text
PASS
PASS WITH DOCUMENTATION GAPS
BLOCKED
```

中选择。

如果任何快照、hash、rename、reconstruction 或证据 provenance 出现不一致：

**BLOCKED + STOP。**

---

## ABSOLUTE FINAL BOUNDARY

本阶段的正确终点不是“把所有东西整理漂亮”。

正确终点是：

> **先阻止历史证据继续丢失，再建立可审计、可区分 Original / Prompt / Reconstructed / Lost 的证据档案。**

完成后停止，等待下一授权。











