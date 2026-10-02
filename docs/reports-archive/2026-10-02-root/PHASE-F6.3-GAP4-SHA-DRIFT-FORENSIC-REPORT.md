# PHASE F-6.3 — GAP-4 SHA-DRIFT FORENSIC RECONCILIATION — REPORT

**PHASE F-6.3 ｜ STRICT READ-ONLY / FORENSIC PROVENANCE AUDIT ｜ NO RESTORATION ｜ NO REPAIR ｜ NO COMMIT**

---

## 1. PHASE IDENTITY

| 项 | 值 |
|---|---|
| Project | **P2PChain** |
| Phase | **F-6.3** |
| Title | **GAP-4 SHA-DRIFT FORENSIC RECONCILIATION** |
| Mode | STRICT READ-ONLY / FORENSIC PROVENANCE AUDIT |
| Previous Phase | PHASE F-6.2 — P3.1 PROVENANCE RESTORATION |
| Previous Verdict | PASS / HARD STOP |
| Baseline HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| 本阶段性质 | **纯取证调查**（非恢复 / 非修复 / 非 commit / 非实现） |
| 唯一目标 | 判定 4 个 GAP-4 SHA-drifted artifact 的 provenance 与状态 |

---

## 2. BASELINE VERIFICATION

| 项 | 规格要求 | 实测 | 判定 |
|---|---|---|---|
| HEAD | `4d892be` | `4d892be355a38886a83c123faad915c9f3cd62e4` | ✅ |
| staged | 0 | 0 | ✅ |
| porcelain modified | 36 | **36** | ✅ |
| untracked | 197 | **197** | ✅ |
| `.gitignore` | `f43418cf…` | `f43418cf74ea996cc603976f29badc4c507e6b3ace25f7f619d7d79aefb6567b` | ✅ |
| `node.exe` | 未变 | `3972843aff90428d…` | ✅ |
| `run-a` / `run-b` | 未变 | 存在 | ✅ |
| F-6.2 report | `7db1e238…e04f0` | `7db1e23894e8fd0d968cb3e335b2bd4de262b67c3833e8a88789063e7a3e04f0` | ✅ **MATCH** |
| P3.1 restored | — | `e9def9aa…fe61`（保持恢复态） | ✅ |

### 2.1 一处规格/实测差异（**已解释，非未授权漂移**）

规格 §1 另列 `tracked = 36`；实测 `git diff --name-only` = **35**。

- 差异 = **恰好 1**，来源已确证：`git diff --name-only | grep PHASE-P3.1` → **ABSENT**。
- 即 **F-6.2 恢复的 P3.1 文档已与 HEAD 逐字节一致，因而不计入 modified**（F-6.2 报告 §11 已记录该预期后果）。
- 规格同列的 `porcelain modified = 36` 与实测 **一致**；`untracked = 197` 亦**一致**。
- ⇒ **属规格陈旧值（应为 35），非仓库未授权漂移**。按 §1「Do not repair the drift」，**未做任何修复**。

**BASELINE VERDICT = PASS（附上述已解释差异）。未触发 STOP。**

---

## 3. GAP-4 AUTHORITATIVE REPORT IDENTITY（候选清点）

### 3.1 候选清点（**不按文件名裁定**）

| # | 候选 | SHA-256 | size | lines | mtime | Git 状态 | 含 4 个记录 SHA？ |
|---|---|---|---|---|---|---|---|
| **C1** | `docs/PHASE-P2PCHAIN-GAP-4-COINBASE-ATTRIBUTION-IMPLEMENTATION-REPLAY-AUDIT-REPORT.md` | `0bf64bf6cd755e471629580079b453da5dad3b00edf89de51e566bfd8bfd66ad` | 8,583 B | 134 | 2026-09-20 20:07:44 | **UNTRACKED** | **是（4/4）** |
| C2 | `docs/PHASE-P2PCHAIN-E1-SOP-GAP4-READINESS-AUDIT-REPORT.md` | `ea171c3f6177bd770458b48345e65c0891200f780ff5cc98a9aa15c92bb05d7a` | 9,940 B | — | 2026-09-20 19:20:48 | UNTRACKED | **否（0/4）** |

### 3.2 权威判定

**C1 为权威 GAP-4 报告**。依据（**基于 provenance 证据，非文件名**）：

1. C1 自述为「GAP-4 COINBASE ATTRIBUTION IMPLEMENTATION + TEST / OFFICIAL SAMPLE REPLAY — **FINAL REPORT**」。
2. C1 §2 以表格**逐文件登记 4 个 artifact SHA-256**（候选 C2 为 0/4）。
3. C1 §1 明确列出实际写入范围，恰为 4 个 artifact + 本报告。
4. C2 自述为「SOP FREEZE + GAP-4 … **READINESS AUDIT**」，属**就绪性审计**，非实现报告。

**权威报告 C1 元数据**：

| 字段 | 值 |
|---|---|
| Path | `docs/PHASE-P2PCHAIN-GAP-4-COINBASE-ATTRIBUTION-IMPLEMENTATION-REPLAY-AUDIT-REPORT.md` |
| Git 状态 | **UNTRACKED**（`git ls-files --error-unmatch` 失败） |
| Baseline HEAD（报告自述） | `34decb240a86b173e5af1d27b0d2644bd9a947d0`（`docs: freeze E1 execution SOP v1.0`，parent `8466766`） |
| 报告自述 REPO | **`H:\wakuang\p2pchain`**（≠ 当前 `E:/wakuang/p2pchain`） |
| 报告自述 Date | 2026-09-20 |
| DIFF_SHA（报告自述） | `7f0fc2b55727043892dd43aeca0e0f3af4775685d470ac3cbe4a91d21bbee889` |

---

## 4. GAP-4 REPORT SHA

```
0bf64bf6cd755e471629580079b453da5dad3b00edf89de51e566bfd8bfd66ad
```

（`sha256sum docs/PHASE-P2PCHAIN-GAP-4-COINBASE-ATTRIBUTION-IMPLEMENTATION-REPLAY-AUDIT-REPORT.md`；8,583 B / 134 行；mtime 2026-09-20 20:07:44；**未修改**。）

---

## 5. REPORTED ARTIFACT HASHES（报告登记值）

| 文件 | 报告登记 SHA-256（C1 §2） |
|---|---|
| `internal/attribution/attribution.go` | `783aeaf569e308a858a40506db1ab19f34941922653d7745998c51b642388715` |
| `internal/attribution/classify.go` | `f64f07bb55485eee68e12410a6a49580d66e12d6835e905d2cb15cb8feaf338e` |
| `internal/attribution/attribution_test.go` | `4c4d91fdd9274cb6fa40287101c54c681ab3a357720a7b2b6e40bfc743c8f43b` |
| `cmd/coinbase-attribution/main.go` | `c0fe645c1962762b6c5672fa1b39acb3c6b2cacd92e826cb4ca2a913f95c703d` |

> 报告**未登记**字节大小 / 行数（仅 SHA-256）。

---

## 6. CURRENT ARTIFACT HASHES（独立复验）

| 文件 | 当前工作树 SHA-256 | size | lines | mtime |
|---|---|---|---|---|
| `internal/attribution/attribution.go` | `3f4cb77820b43747de5354b1f095336e175c1afa4157758223d834d5dac3dadb` | 12,709 B | 355 | 2026-09-22 14:45:37 |
| `internal/attribution/classify.go` | `bb7b00bc135c6146c9ca110f93080b839bd963d99cd78d3eb578ec7a07865060` | 6,803 B | 217 | 2026-09-22 14:45:37 |
| `internal/attribution/attribution_test.go` | `4d7841d418a668818b5c151245045d1a37029e2e77e12d32b779a5b7b77d8b7c` | 16,158 B | 459 | 2026-09-22 14:45:37 |
| `cmd/coinbase-attribution/main.go` | `c634c1ecaea5955118533d035af2d0db2fa8f18365fa579a22c0b7401bfa6f18` | 6,098 B | 197 | 2026-09-22 14:45:36 |

**与 F-6.1-OD 记录逐值一致（独立复验通过）。** 4 个当前 SHA 与 4 个记录 SHA **全部不等**。

---

## 7. GIT BLOB IDENTITY MATRIX

| 文件 | 当前工作树 blob（`git hash-object`） | HEAD blob | 记录 blob |
|---|---|---|---|
| `internal/attribution/attribution.go` | `9cd9228bca805ab9b4eae91ae499b38fcc1cb2a4` | **不存在**（未在 HEAD） | **不存在**（记录值为 SHA-256，无对应 blob） |
| `internal/attribution/classify.go` | `1e3441fa690966846460b7756c7627313f83e8a5` | **不存在** | **不存在** |
| `internal/attribution/attribution_test.go` | `b53dd993a28d2178caf8c1e069cf51b04ec12ba4` | **不存在** | **不存在** |
| `cmd/coinbase-attribution/main.go` | `eb7146a90c7a615d501d517b5f5865eeceb9c675` | **不存在** | **不存在** |

- 4 个当前 blob SHA-1 **均不在任何 commit/tree**（`git rev-list --all --objects | grep <blob>` → **0**）。
- `git cat-file -e <blob>` → **ABSENT**（4/4）。
- ⇒ 当前内容**仅存在于工作树**，从未被 `git add`/commit。

---

## 8. HISTORICAL COMMIT MATRIX

| 探查 | 结果 |
|---|---|
| `git log --all --oneline -- internal/attribution/` | **空**（无任何 commit 触及） |
| `git log --all --oneline -- cmd/coinbase-attribution/` | **空** |
| 首次出现（commit 引入） | **无**（文件从未被提交） |
| 修改该文件的 commit | **无** |
| 报告基线 commit `34decb24…` | **存在**，且 `git merge-base --is-ancestor 34decb2 HEAD` → **是 HEAD 的祖先** |
| 唯一 ref | `refs/heads/main` → `4d892be` |
| HEAD 祖先链（近 10） | `4d892be → 060333f → fce8b73 → f17978c → 0f337de → ca926c1 → 16a66a5 → e90f38f → 34decb2 → 8466766` |

**⇒ 4 个 artifact 从未进入任何 commit。报告基线 `34decb2` 与本仓历史同源（为 HEAD 祖先）。**

---

## 9. REPORTED-SHA EXISTENCE MATRIX

方法：对 **全部 391 个 blob** 逐个计算内容 SHA-256，与 4 个记录值比对（`git cat-file --batch` 单流）。

| 记录 SHA-256 | 在 Git object DB？ | 在其他仓库证据？ |
|---|---|---|
| `783aeaf5…8715` | **ABSENT**（391/391 无命中） | 仅文本引用：C1 报告 + `PHASE-F6.1-OD-OWNER-DECISION-CAPTURE.md` + `PHASE-F6.1-OWNER-DECISION-FINALIZATION.md` |
| `f64f07bb…338e` | **ABSENT** | 同上 |
| `4c4d91fd…f43b` | **ABSENT** | 同上 |
| `c0fe645c…703d` | **ABSENT** | 同上 |

**结论**：4 个记录 SHA **不对应任何现存 Git 对象**；其对应字节**在仓库内不可恢复**。
（证据包 `H:\wakuang\.workbuddy\GAP4-REPLAY\` 自述存在，但 **H: 盘在本环境不可达** ⇒ 无法从该处取回。）

---

## 10. CURRENT-SHA EXISTENCE MATRIX

| 当前 SHA-256 | 在 Git object DB？ | 属于任何 commit？ | 仅工作树？ | 匹配已知历史 artifact？ |
|---|---|---|---|---|
| `3f4cb778…dadb` | **ABSENT** | 否 | **是** | 否（记录值不同） |
| `bb7b00bc…5060` | **ABSENT** | 否 | **是** | 否 |
| `4d7841d4…8b7c` | **ABSENT** | 否 | **是** | 否 |
| `c634c1ec…6f18` | **ABSENT** | 否 | **是** | 否 |

**⇒ 当前字节仅作为工作树内容存在，未进入任何 Git 对象。**

---

## 11. BYTE-LEVEL COMPARISON

**记录内容 vs 当前内容的字节级 diff —— 无法执行**：记录值对应的字节**不可恢复**（§9），
无法构造 pre-image 进行 `diff`/`cmp`。

因此本阶段改为**对当前文件做独立表征**（只读）：

| 文件 | size | lines | 结尾换行 | BOM | 首 3 字节 | gofmt 合规 |
|---|---|---|---|---|---|---|
| `attribution.go` | 12,709 B | 355 | 有（`0a`） | 无 | `2f2f20`（`// `） | ✅ 合规 |
| `classify.go` | 6,803 B | 217 | 有 | 无 | `2f2f20` | ✅ 合规 |
| `attribution_test.go` | 16,158 B | 459 | 有 | 无 | `706163`（`pac`） | ✅ 合规 |
| `main.go` | 6,098 B | 197 | 有 | 无 | `2f2f20` | ✅ 合规 |

- `gofmt -l internal/attribution/ cmd/coinbase-attribution/` → **无输出（4/4 gofmt-clean）**。
- **不得**因「能编译」而把差异判定为合法（§8 要求）——本阶段仅做 provenance 分析。
- 未发现「添加/删除区域、函数、常量、import」的任何可观测证据（因无 pre-image，**不作断言**）。

---

## 12. CRLF / LF ANALYSIS

**假设检验：`current SHA ≠ recorded SHA` 能否仅由 CRLF↔LF 解释？**

| 文件 | `\r` 字节数 | `\n` 字节数 | 判定 |
|---|---|---|---|
| `attribution.go` | **0** | 355 | **纯 LF** |
| `classify.go` | **0** | 217 | **纯 LF** |
| `attribution_test.go` | **0** | 459 | **纯 LF** |
| `main.go` | **0** | 197 | **纯 LF** |

- 4 个文件**均无任何 CR 字节**（`tr -cd '\r' | wc -c` = 0）⇒ **纯 LF**。
- **无 BOM**；**末字节为 `0a`**（结尾换行存在）。
- ⇒ **当前内容不含 CRLF**。若记录内容亦为 LF，则 **SHA 差异不能由 CRLF↔LF 解释**。
- **但**：记录内容不可恢复，**无法独立确认其行尾形态**。`gofmt` 输出恒为 LF，
  故若事故前为 CRLF，事故会**顺带**产生行尾归一化 ⇒ 漂移**可能**同时含「排版」与「行尾」两个分量。
- **明确声明**：**无任何证据支持「仅 CRLF 差异」**；差异属**实质性（非行尾）**内容差异
  （与 §14 的 gofmt 排版归一化一致）。
- 未做任何归一化（§15 禁令）。

---

## 13. TIMESTAMP ANALYSIS

`2026-09-22 14:45` 窗口复核（`find -newermt`）：

| 文件 | mtime | 类别 |
|---|---|---|
| `cmd/coinbase-attribution/main.go` | 14:45:36 | GAP-4 |
| `internal/attribution/attribution.go` | 14:45:37 | GAP-4 |
| `internal/attribution/attribution_test.go` | 14:45:37 | GAP-4 |
| `internal/attribution/classify.go` | 14:45:37 | GAP-4 |
| `internal/storage/f1n1_undo_contract_test.go` | 14:45:38 | F1-N1 |
| `internal/blockchain/f1n1_reorg_coverage_test.go` | 14:45:37 | F1-N1 |
| `internal/obs/obs.go` | 14:45 | tracked-CLEAN（CRLF-only） |
| `internal/p2p/node_test.go` | 14:45 | tracked-CLEAN（CRLF-only） |
| `internal/pow/pow.go` | 14:45 | tracked-CLEAN（CRLF-only） |

**本阶段证据升级**：该时间簇**不再是弱相关**，而是被 **A-2.3-G-CR §7 的事故披露精确解释**：
一次 `go fmt ./...`（实际带 `-w`）在 **14:45:36–38** 重写了 **17 个 `.go` 文件**
（11 tracked + 6 untracked）。**6 个 untracked = 4 个 GAP-4 artifact + 2 个 F1-N1 测试**（§14）。

**结论**：
- mtime **在本案中提供了有效佐证**（与事故披露的窗口逐秒吻合）。
- **但仍须显式声明**：**filesystem timestamp correlation is not equivalent to provenance.**
  时间戳**只作佐证**，归属依据是**事故披露文本 + 后续三阶段复验**（§14），**非** mtime 本身。

---

## 14. CROSS-WORKSTREAM PROVENANCE ANALYSIS

### 14.1 关键证据链（本阶段新发现）

**证据 E-a — A-2.3-G-CR §7「审计过程事故与还原（完整披露）」**
（`docs/PHASE-P2PCHAIN-A2.3-G-COMMIT-READINESS-AUDIT-REPORT.md:296-321`）：

> **事件**：审计执行 `go fmt ./...`（本意为"列出不合规文件"）时，该命令实际带 `-w` 语义，**重写了 17 个 .go 文件**，
> 其中 11 个为 tracked 且相对 HEAD 干净……**残留（不可通过 git 还原，格式化-only，不进入 commit）**：
> 以下 **6 个 untracked** legacy 文件在 **14:45:36–38** 被 gofmt 重写：
> `cmd/coinbase-attribution/main.go` / `internal/attribution/attribution.go` /
> `internal/attribution/attribution_test.go` / `internal/attribution/classify.go` /
> `internal/blockchain/f1n1_reorg_coverage_test.go` / `internal/storage/f1n1_undo_contract_test.go`
>
> **影响评估**：gofmt 仅做**空白/排版归一化，不改变语义**；`go build ./...` 与 `go vet` 均通过；
> 这些文件均为 untracked，不在本次 commit 的 3 文件范围内。

（tracked 侧 11 文件已用**显式清单** `git checkout HEAD -- <11 files>` 还原；untracked 侧 6 文件**不可经 git 还原**，留作残留。）

**证据 E-b — A-2.3-H §8**（`docs/PHASE-P2PCHAIN-A2.3-H-…-AUDIT-REPORT.md:264`）：

> **6 个 legacy untracked `.go` 文件（A-2.3-G-CR gofmt 事故残留）**：mtime 全为 `2026-09-22 14:45:3x`、
> sha 分别 `c634c1ecaea5 / 3f4cb77820b4 / 4d7841d418a6 / bb7b00bc135c / bd936d3a8146 / 543b188316e1`
> ⇒ **本阶段零触碰**，仍为 **formatting-only**、不在任何 commit 内。

**证据 E-c — A-2.3-F-R1 §14.4**（`docs/PHASE-P2PCHAIN-A2.3-F-R1-…-RERUN-REPORT.md:663-677`）：
同 6 文件，mtime `14:45:3x`，sha256 与 A-2.3-H §8 **逐值一致**；**未被修改、未被 staging、未进入任何 commit**；
⇒ **未恢复、未删除、未重新格式化**。

### 14.2 归属分类（§11 五类）

| 类别 | 是否成立 | 证据 |
|---|---|---|
| A. 精确等于 GAP-4 记录 artifact | **否** | SHA 全不等（§5 vs §6） |
| **B. 后世的合法 descendant** | **是（附注）** | E-a 事故披露 + E-b/E-c 复验；gofmt 排版归一化 |
| C. 来自另一工作流的内容 | **否** | 无证据显示内容来自其他工作流；文件身份连续 |
| D. 手工重建文件 | **否** | 无证据；事故披露指向 gofmt 机械重写 |
| E. 未知 provenance 的 modified descendant | **否** | provenance **已知**（E-a） |
| F. 无法解析 | **否** | — |

**⇒ 归类 = B（后世 descendant）**。
**附注（诚实披露）**：E-a 原文称该事件为「**事故**」——即该变换**非本意**（工具越界写入）。
但：(i) 变换机制（gofmt）**确定且语义中性**；(ii) 结果被**后续三个阶段明文披露、复验并保留**。
因此按 §12 术语，它是**有据可查的 descendant**，而非「异源内容」。

### 14.3 未推定事项（§11 要求）

- **不推断意图**，**不推测肇事者**；仅以中性 provenance 术语陈述。
- **不**将「mtime 相同」当作 authorship 证据。

---

## 15. PER-FILE DISPOSITION

对每个文件，按 §12 五选一（**不**强行 P2/P3，须有证据）：

| # | 文件 | 记录 SHA | 当前 SHA | **初步裁定** |
|---|---|---|---|---|
| 1 | `internal/attribution/attribution.go` | `783aeaf5…` | `3f4cb778…` | **P2 — VERIFIED DESCENDANT** |
| 2 | `internal/attribution/classify.go` | `f64f07bb…` | `bb7b00bc…` | **P2 — VERIFIED DESCENDANT** |
| 3 | `internal/attribution/attribution_test.go` | `4c4d91fd…` | `4d7841d4…` | **P2 — VERIFIED DESCENDANT** |
| 4 | `cmd/coinbase-attribution/main.go` | `c0fe645c…` | `c634c1ec…` | **P2 — VERIFIED DESCENDANT** |

**P2 依据（每文件相同）**：
- 当前字节 ≠ 记录字节（排除 P1）。
- **有据可查的变换链**：`GAP-4 artifact（记录 SHA）` → `A-2.3-G-CR gofmt 事故（14:45:36–38，明文披露该 4 文件）` → `当前字节（SHA 由 A-2.3-H §8 / A-2.3-F-R1 §14.4 复验，零后续触碰）`。
- 变换机制**确定**（gofmt 排版归一化），且当前文件 **gofmt-clean**（§11）与之自洽。
- 排除 P3（内容身份连续，非异源工作流）、P4（漂移**已可归因**，非「无法调和」）。

**P2 的边界条件（须一并记录）**：
- 该链是**文档链**（三份阶段报告），**非 Git-commit 链**（文件从未提交）。
- **pre-incident 字节不可恢复** ⇒「formatting-only」为**文件级旁证 + 明文断言**，**非字节级复算**。
- 「intentional descendant」中的 *intentional* 仅适用于**保留/披露**（有意），**不**适用于触发事件（事故）。
- 记录 artifact 本身的字节**永久缺失**（P5 属性）——见 §19。

---

## 16. GLOBAL GAP-4 VERDICT

依据 §17 判定规则：

| 全局结论 | 适用？ | 理由 |
|---|---|---|
| **GAP-4 PROVENANCE RECONCILED** | **✅ 采用** | 4 个 artifact **均有可辩护、有证据支撑的 provenance 链**（E-a 事故披露 + E-b/E-c 复验），漂移**已被完整归因** |
| GAP-4 PARTIALLY RECONCILED | ✗ | 4 文件状态**一致**（非「部分已证、部分未决」） |
| GAP-4 SHA-DRIFT UNRESOLVED | ✗ | 无 artifact 属「无法 provenance 调和」 |
| BLOCKED | ✗ | 基线完整性通过，调查已完成且未越界 |

```
GAP-4 PROVENANCE RECONCILED
```

**说明**：判定基于 **provenance 证据**（三份独立阶段报告 + 一致 SHA/mtime + gofmt-clean 旁证），
**非**基于「当前文件能编译」（§17 明确禁止）。SHA 值不同**不构成 FAIL**（§17 明确禁止）。
**残留验证深度限制见 §19。**

---

## 17. OUT-OF-SCOPE CONFIRMATION

| 冻结边界 | 本阶段动作 | 判定 |
|---|---|---|
| **P3.1**（`docs/PHASE-P3.1-…md`） | **未触碰**；仍为恢复态 `e9def9aa…` | ✅ FROZEN |
| **F1-N1 / F1-N2** | **未修改 / 未暂存** | ✅ FROZEN |
| **Console** | **未修改** | ✅ FROZEN |
| **AUDIT-FIX** | **未执行** | ✅ FROZEN |
| **Flaky test** `TestF1N1_C_MixedLegacyV2Reorg` | **未修改**；保持 `OPEN / DO NOT FIX` | ✅ FROZEN |
| **Secret artifacts**（3 token） | **未读取 / 未修改 / 未暂存 / 未移动** | ✅ FROZEN |
| 调查范围 | **仅** 4 个 GAP-4 文件 | ✅ 未扩张 |
| 代码改动 | **零**（未编辑/恢复/删除/移动/重命名/复制/归一化/重建） | ✅ STRICT READ-ONLY |

---

## 18. GIT MUTATION AUDIT

```
git add / commit / push / reset / clean / stash / checkout / merge / rebase / amend / squash / tag
→ 全部 NOT EXECUTED
```

- 本阶段**唯一**写入 = 本报告文件（`PHASE-F6.3-GAP4-SHA-DRIFT-FORENSIC-REPORT.md`，新建）。
- 临时脚本 `f63_scan.py`、`/tmp/f63-*` 写入**系统临时目录**，**仓库零改动**。
- 4 个 GAP-4 文件在本阶段前后**逐字节不变**（见 §21 收工核验）。

---

## 19. LIMITATIONS

1. **记录字节不可恢复**：4 个记录 SHA（`783aeaf5…` 等）**不对应任何 Git 对象**（391 blob 全扫无命中），
   自述证据包 `H:\wakuang\.workbuddy\GAP4-REPLAY\` 在本环境**不可达**。
   ⇒ **pre-incident 字节永久缺失**（P5 属性）；无法做字节级 diff，无法独立复算「formatting-only」。
2. **文档链 ≠ Git-commit 链**：4 文件从未被提交（§8），故 P2 的链为**三份阶段报告**的文档链，
   而非 Git 历史链。
3. **「formatting-only」为断言 + 旁证**：由 A-2.3-G-CR §7 明文断言、A-2.3-H §8 / A-2.3-F-R1 §14.4 复验，
   并有「当前文件 gofmt-clean」旁证；**未经字节级独立复算**。
4. **09-20 → 09-22 14:45 区间无逐字节连续性证据**：报告记录于 2026-09-20 20:07，事故发生于 2026-09-22 14:45。
   二者之间**无其它已记录的修改**，故推断 pre-incident = GAP-4 记录态；该推断**依赖「无其它已记录修改」**，
   而非直接字节证明。
5. **行尾形态不确定**：当前为纯 LF（§12）；pre-incident 行尾形态**未知**，gofmt 会归一化为 LF
   ⇒ 漂移**可能**同时含排版与行尾分量。
6. **单一验证者**：全部核验在**同一会话**内完成（self-attested），未引入独立第三方。
7. **报告自引用 SHA**：本报告 SHA 由**外部**命令计算（§19.1），正文**不内嵌**自身 SHA。

### 19.1 Final Report SHA-256（external record）

```
PHASE-F6.3-GAP4-SHA-DRIFT-FORENSIC-REPORT.md
SHA-256 = （见交付说明 / memory 的外部登记；本文件内不内嵌以避免自引用矛盾）
```

---

## 20. OWNER DECISION REQUIRED

本阶段**只判定 provenance，不裁定处置**。以下事项**须 Owner 显式决策**（§13 明令本阶段**零 mutation**）：

| OD 候选 | 事项 | 本阶段给出的输入 |
|---|---|---|
| **F-6.3-OD-1** | 是否**接受**当前 4 个 artifact（GAP-4 descendant）为可提交内容 | provenance = **P2（有据 descendant）**；提交资格取决于 Owner 对「formatting-only 仅为文档断言」的接受度 |
| **F-6.3-OD-2** | 是否**要求**先取得 pre-incident 字节（例如从 `H:` 证据包或备份）再做字节级确认 | 当前**不可达**；若 Owner 可提供，可升级为字节级验证 |
| **F-6.3-OD-3** | 是否在提交时**保留**本次格式化噪声，或另行处理 | A-2.3-G-CR §7 已披露「diff 中将包含本次格式化噪声」 |
| **F-6.3-OD-4** | GAP-4 授权报告（C1）**是否入库** | C1 目前 **UNTRACKED** |

**本阶段不预设任何处置。**

---

## 21. HARD STOP

```
PHASE F-6.3 COMPLETE
VERDICT: GAP-4 PROVENANCE RECONCILED
HARD STOP — OWNER DECISION REQUIRED
```

**收工核验（§18）**：

| 检查 | 结果 |
|---|---|
| 报告 SHA 稳定（写入→读回→复算） | ✅ 零漂移（§19.1 外部登记） |
| 仓库状态重跑 | HEAD `4d892be` / staged 0 / porcelain M 36 / untracked 198（+1 报告） |
| 无源/测试文件变更 | ✅ |
| P3.1 保持恢复态 | ✅ `e9def9aa…` |
| 4 个 GAP-4 文件逐字节不变 | ✅ SHA 与 §6 相同 |
| token 未触碰 | ✅ 仍存在、未读、未暂存 |
| 无 Git 写操作 | ✅ |

**不得自动**进入：GAP-4 restoration / 接受或拒绝当前文件 / commit / push / clean / F1-N1 / F1-N2。
**下一步必须由新的显式 Owner Decision 决定。**

```
F-6.2 RESTORED CANONICAL PROVENANCE.
F-6.3 DETERMINED THE GAP-4 DRIFT IS RECONCILABLE (documentary chain).
EVIDENCE FIRST. NO RESTORATION. NO ASSUMPTIONS. NO AUTO-ADVANCE.
```

**HARD STOP。**
