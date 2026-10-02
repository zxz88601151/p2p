# PHASE REORG-1J-DOC-2-ARCHIVE-GOVERNANCE — FINAL REPORT

- 阶段性质：**STRICT CONTROLLED DOCUMENTATION GOVERNANCE ONLY**（非代码、非功能、非 R5、非测试增强）
- 执行日期：2026-09-15（17:38–17:45）
- 基线 HEAD：`e04d678`（全程未变）
- 范围：DOC-2-REPAIR 遗留的 **Step 5 + Step 6**
- 生产代码改动：**0**｜测试代码改动：**0**｜覆盖/删除 evidence：**0**

---

## 1. VERDICT

```text
PASS WITH DOCUMENTATION GAPS
```

**裁定依据（§9 两条款的取舍）**：

| 条款 | 是否满足 | 说明 |
|---|---|---|
| 「Step 5/6 全部完成且没有新的 evidence integrity issue」→ PASS | ✅ **满足** | Step 5 = **6/6 MOVE-OK**；Step 6 = 消歧登记完成；**本阶段未发现任何新的 evidence integrity issue**（§8 十一项 STOP 条件逐条检查，全部未触发） |
| 「仍有 documentation-only limitation」→ PASS WITH DOCUMENTATION GAPS | ✅ **同样满足** | REORG-1J 证据体系整体仍存在 **5 项不可消除的 documentation-only limitation**（§10），**非本阶段产生，亦非本阶段可消除** |

⇒ 取**较保守者**：**PASS WITH DOCUMENTATION GAPS**。
理由：若本阶段裁定 PASS，将暗示「证据档案已完全干净」，会使 R5 免于登记 6 份 RECONSTRUCTED / 1 份 LOST / DOC-2 永久损失 —— **这是错误信号**。

**本阶段自身范围内（Step 5 + Step 6）= 完全 PASS，零缺口。**

---

## 2. BASELINE

### 2.1 只读基线（2026-09-15T17:38:42+08:00）

| 项 | 值 |
|---|---|
| `git rev-parse HEAD` | `e04d678b4fb33a14ea186999375f6d6dac2a4c4b` |
| `git branch --show-current` | `main` |
| `git diff --cached --name-only` | **0**（staged 为空，无用户既有 staging 需报告） |
| `git diff --name-only` | 11 个长期 dirty 文件（9 代码 + 2 docs），**均为 1H/1J/R3/R4A/R4B 遗留，本阶段未触碰** |
| `git rev-list --all --count` | 35 |
| 工作区根是否 git 仓库 | **否**（`cd .. && git rev-parse --show-toplevel` → `fatal: not a git repository`） |
| `docs/phases/` 是否存在 | **否**（移动前 `ls -d docs/phases` → No such file or directory） |

### 2.2 上一阶段已确认事实的继承核验

| 事实 | 本阶段复核 |
|---|---|
| A. `docs/_archive_pre-doc2/` 40 份快照 | ✅ 在位 |
| B. `MANIFEST.tsv` 36/37 一致，DOC-2 条目为 stale metadata | ✅ **未修改 `MANIFEST.tsv`**（本阶段零写入该文件） |
| C. rename 25/25 hash verified OK | ✅ 未触碰 |
| D. G09 35 B 三方 hash 一致 | ✅ 复核仍为 `25221e11…43308` ×2 |
| E. 6 份 reconstructed 必须继续标记 | ✅ 6/6 hash 未变，标记未动 |
| F. 1I 维持 LOST / REFERENCE-ONLY，仅 6 项引用 | ✅ 复核 6 项行号，未变 |
| G. evidence discrepancies 原样保留 | ✅ **未修改任何历史 evidence**（未改 `2026-09-15.md`、未改 R4C、未改 DOC-2、未改 REPAIR 报告） |
| H. DOC-2 16:04 原版 25,664 B 永久不可恢复 | ✅ 未尝试任何「恢复」 |
| I. 不得用 `git diff` 判断整个工作区 | ✅ 采用逐文件 SHA-256 |

---

## 3. STEP 5 RESULT

```text
目标：治理工作区根遗留的 6 份 ORIGINAL
动作：MOVE（非 copy、非 delete）→ p2pchain/docs/phases/
结果：6 / 6 MOVE-OK
```

| 检查项 | 结果 |
|---|---|
| 只读枚举（文件名/大小/哈希/git 状态/归档对应物/重复/混淆风险） | ✅ 6/6 完成，见 §5 |
| 归档对应物是否存在 | ✅ 6/6 `COUNTERPART-MATCH`（`_archive_pre-doc2/workspace-root/`） |
| 是否存在第三份重复 | ✅ **无**（全 `挖矿\` 扫描每份仅 2 处） |
| 是否与 reconstructed 混淆 | ✅ **无**（6 个文件名与 6 份 RECONSTRUCTED 无一名重合） |
| 目标目录是否已存在 | ✅ **不存在** ⇒ 不可能覆盖已有 evidence |
| 逐份 Gate（source 存在 ∧ dest 不存在） | ✅ 6/6 PASS |
| 移动后 source 状态 | ✅ 6/6 `ABSENT` |
| 移动后 hash / size | ✅ 6/6 不变 |
| 是否发生删除 | ❌ **无删除**（`mv`，字节与哈希守恒） |

**治理产物**：`docs/_archive_pre-doc2/ORIGINAL-REFERENCE-ONLY-GOVERNANCE.md`（含完整矩阵与验证）。

**关键副作用（正向）**：工作区根 `挖矿\` 下 `PHASE-REORG-*.md` 由 **6 → 0**；
6 份 ORIGINAL 现位于 **git 工作树内**（`p2pchain/docs/phases/`），R5 可显式 `git add`
⇒ **关闭 DOC-2 §2 登记的「6 份 ORIGINAL 在版本控制之外」缺陷**。

---

## 4. STEP 6 RESULT

```text
目标：REORG-1I 最终消歧登记
动作：新增独立 governance evidence（不修改既有登记文件）
结果：完成
```

| 项 | 结果 |
|---|---|
| STATUS 维持 | ✅ `LOST / REFERENCE-ONLY` |
| 原始 report 不存在 / 不可恢复 | ✅ 明确记录 |
| 不得把 reconstructed 当作 original | ✅ 明确记录 |
| 6 项 evidence references 复核 | ✅ 6/6 行号逐条复核一致（`REORG-1J.prompt.md:9/13/15-17/31-35/97/151/188/642-646`） |
| references 可用于审计定位 | ✅ 明确记录 |
| 不足以构成原始完整 report | ✅ 明确记录 |
| 两个 1I 含义并存登记 | ✅ ①WBS 原义 `finality/MaxReorgDepth`（NOT EXECUTED，落地物 `settip.go:26`）；②`LEGACY/V2 CANONICAL REORG STORAGE AUDIT`（执行过，报告被覆盖） |
| 是否修改历史报告 | ❌ **无** |
| 是否声称恢复成功 | ❌ **无** |
| 是否发现新的恢复可能 | ❌ **无**（§8 第 10 项未触发） |

**治理产物**：`docs/_archive_pre-doc2/reference-only/REORG-1I-FINAL-DISAMBIGUATION-GOVERNANCE.md`
（**独立新增**，与 REPAIR 阶段的 `REORG-1I-LOST-REFERENCE-ONLY.md` 并存，后者未被修改）。

---

## 5. ORIGINAL → REFERENCE-ONLY MATRIX

| # | 文件名 | size | SHA-256 | 原位置 | 归档位置 | 快照位置 | 分类 |
|---|---|---|---|---|---|---|---|
| 1 | PHASE-REORG-1B-COMMIT-READINESS-AUDIT-REPORT.md | 20,352 | `0805d4a099e1774201649209c91d2997ec64d00a6d710dea6d2f0a2dbe05efed` | `挖矿\` | `docs/phases/` | `_archive_pre-doc2/workspace-root/` | **ORIGINAL** |
| 2 | PHASE-REORG-1E-IMPLEMENTATION-1-CONTRACT-CONFLICT-REPORT.md | 14,986 | `e67250bee9572ad4f26fed784caf1fce7f79ad940ffc3ea6ac644d5862a21771` | `挖矿\` | `docs/phases/` | `_archive_pre-doc2/workspace-root/` | **ORIGINAL** |
| 3 | PHASE-REORG-1E-PRE-IMPLEMENTATION-DESIGN-AUDIT-1-FINAL-REPORT.md | 53,969 | `bfa1ab15675a5634b60ff84c0a423f53e7563c8e9d2c2bb9ad14bbaec17e2e2d` | `挖矿\` | `docs/phases/` | `_archive_pre-doc2/workspace-root/` | **ORIGINAL** |
| 4 | PHASE-REORG-INFRASTRUCTURE-DESIGN-1-REPORT.md | 31,927 | `6cc4a076439a5c9b83d4475147d0be38b3c55293d917f5b47a9286f6fac15736` | `挖矿\` | `docs/phases/` | `_archive_pre-doc2/workspace-root/` | **ORIGINAL** |
| 5 | PHASE-REORG-INFRASTRUCTURE-IMPLEMENTATION-1-REORG-1B-FINAL-REPORT.md | 17,135 | `32de847862d129523a61f405aa8c121e79da6cf0208a88af510d2806acc3b6d9` | `挖矿\` | `docs/phases/` | `_archive_pre-doc2/workspace-root/` | **ORIGINAL** |
| 6 | PHASE-REORG-INFRASTRUCTURE-IMPLEMENTATION-2-REORG-1D-FINAL-REPORT.md | 28,062 | `677117e41c2a782a3755a21b5ea84c82f21f120e9dd4b1b67171af5ef0ddf4de` | `挖矿\` | `docs/phases/` | `_archive_pre-doc2/workspace-root/` | **ORIGINAL** |

**分类说明**：6 份**仍为 ORIGINAL**，未被降级。
「REFERENCE-ONLY / ARCHIVE」在本阶段指**归档治理动作与受控存放位置**，
与 `REORG-1I` 的 `LOST / REFERENCE-ONLY`（因证据不足而不可采信）**语义不同，不得混用**。

**移动前只读枚举补充**：

| 属性 | 值（6/6 一致） |
|---|---|
| git tracked/untracked | **不在任何 git 仓库中**（`挖矿\` 非仓库） |
| archive counterpart | 存在且 byte-identical |
| 重复文件 | 无第三份 |
| 与 reconstructed 混淆 | 无 |

---

## 6. SHA-256 VERIFICATION

### 6.1 Step 5：source == destination（6/6）

| # | 文件 | pre-move（前 12） | post-move（前 12） | size | source | 判定 |
|---|---|---|---|---|---|---|
| 1 | PHASE-REORG-1B-COMMIT-READINESS-AUDIT-REPORT.md | `0805d4a099e1` | `0805d4a099e1` | 20,352 | ABSENT | **MOVE-OK** |
| 2 | PHASE-REORG-1E-IMPLEMENTATION-1-CONTRACT-CONFLICT-REPORT.md | `e67250bee957` | `e67250bee957` | 14,986 | ABSENT | **MOVE-OK** |
| 3 | PHASE-REORG-1E-PRE-IMPLEMENTATION-DESIGN-AUDIT-1-FINAL-REPORT.md | `bfa1ab15675a` | `bfa1ab15675a` | 53,969 | ABSENT | **MOVE-OK** |
| 4 | PHASE-REORG-INFRASTRUCTURE-DESIGN-1-REPORT.md | `6cc4a076439a` | `6cc4a076439a` | 31,927 | ABSENT | **MOVE-OK** |
| 5 | PHASE-REORG-INFRASTRUCTURE-IMPLEMENTATION-1-REORG-1B-FINAL-REPORT.md | `32de847862d1` | `32de847862d1` | 17,135 | ABSENT | **MOVE-OK** |
| 6 | PHASE-REORG-INFRASTRUCTURE-IMPLEMENTATION-2-REORG-1D-FINAL-REPORT.md | `677117e41c2a` | `677117e41c2a` | 28,062 | ABSENT | **MOVE-OK** |

结束时的**三方比对**（destination vs `_archive_pre-doc2` 快照）：**6/6 `TRIPLE-MATCH`**。

### 6.2 受保护文件：阶段开始 == 阶段结束（26/26）

采集 26 个受保护文件（16 生产/测试 + 6 RECONSTRUCTED + 2 G09 空壳 + 2 1I 引用）的 SHA-256，
阶段起止各一次，`diff` 结果为空：

```text
entries: 26
=== 26/26 PROTECTED FILES UNCHANGED (START == END) — PASS ===
```

| 类别 | 数量 | 结果 |
|---|---|---|
| PRODTEST | 16 | 全等 |
| RECON | 6 | 全等 |
| G09SHELL | 2 | 全等（`25221e11…43308`） |
| 1IREF | 2 | 全等（`e4cf2dd0…86af3`） |

---

## 7. REORG-1I FINAL STATUS

```text
REORG-1I : STATUS = LOST / REFERENCE-ONLY
RECOVERY : NOT ACHIEVED — AND MUST NOT BE CLAIMED
BLOCKS R5: NO
```

| 项 | 内容 |
|---|---|
| 原始 report | **不存在 / 不可恢复**，且本阶段**未发现任何新的恢复可能** |
| 不得当作 original | reconstructed material **不得**冒充 1I original |
| 保留内容 | **仅 6 项可定位引用**（见下），可用于审计定位，**不足以构成原始完整 report** |
| 两个 1I 并存 | ①WBS 原义 `finality / MaxReorgDepth`（NOT EXECUTED / DEFERRED，落地物 `settip.go:26`）<br>②`LEGACY/V2 CANONICAL REORG STORAGE AUDIT`（执行过，报告正文被 1J prompt 覆盖） |
| 唯一现存实体 | 12,020 B，SHA-256 `e4cf2dd08a849fcc21facf056ffba42465ebef007e00d0d4faa110a28c086af3`（本阶段复核未变） |
| R5 阻塞性 | **NO**（R4C §3.1 行 73 / 行 80） |

**6 项 evidence references**（本阶段 17:41 逐行复核，全部一致）：

| # | 引用 | 来源 |
|---|---|---|
| 1 | 阶段名 `PHASE REORG-1I — LEGACY/V2 CANONICAL REORG STORAGE AUDIT` | `REORG-1J.prompt.md:9` |
| 2 | `NOT READY FOR IMPLEMENTATION` | `REORG-1J.prompt.md:13` |
| 3 | `OPTION A / CANONICAL VIEW UNIFICATION` | `REORG-1J.prompt.md:15-17` |
| 4 | `GAP-1I-A` — P0 | `REORG-1J.prompt.md:31`（§4 于 `:97`） |
| 5 | `GAP-1I-B` — P1，修复后视为 P0 | `REORG-1J.prompt.md:32`（§5 于 `:151`） |
| 6 | `GAP-1I-C` / `D` / `E` | `REORG-1J.prompt.md:33-35`、`188`、`642-646` |

**R5 必须登记的表述（不得改写）**：
「1I 原始报告已丢失，其结论以 1J prompt 的『前置报告』段为唯一引用来源。」

---

## 8. RECONSTRUCTED REPORT STATUS

6 份报告**在本阶段零修改**（哈希全等，见 §6.2），`RECONSTRUCTED FROM EVIDENCE` 标记与 `EVIDENCE INSUFFICIENT` 清单**原样保留**。

| # | 文件 | size | SHA-256 | 状态 |
|---|---|---|---|---|
| 1 | PHASE-REORG-1G-FINAL-GAP-AUDIT-REPORT.md | 8,969 | `0ea5ca0186a2030dbdbc563ad89e4fa7f2cef2415c4c81c68bfe5f793f3f17bb` | ✅ 未变 |
| 2 | PHASE-REORG-1J-G08-CONTRACT-EXCEPTION-FINAL-REPORT.md | 8,497 | `19a70fe77d74868ea4cc96e0943139e557327e04d220014662c73f938fac331b` | ✅ 未变 |
| 3 | PHASE-REORG-1J-G09-TRIPWIRE-CONVERSION-FINAL-REPORT.md | 11,432 | `89d8d702205d54aa5dc87e79682aad07ae89aa5edc44fcc5b06bf13cd8a2e00c` | ✅ 未变 |
| 4 | PHASE-REORG-1J-R3-LEGACY-PREFIX-CRASH-MATRIX-FINAL-REPORT.md | 10,489 | `3158b66d626c52a5a4dd46cdc2bc6f8d3c5bf25d2d1486318300663bfb14a66f` | ✅ 未变 |
| 5 | PHASE-REORG-1J-R4A-PRE-GATE-FINAL-REPORT.md | 9,220 | `8147383dfa618351874a6826936f24085bcc9d7f4bd6ef9b9349cccc716ac8b4` | ✅ 未变 |
| 6 | docs/PHASE-REORG-1J-R4A-FINAL-REPORT.md | 7,910 | `cbaa9d23d4e0b99c363b16db89413fb61a442bcb87bc97af80cd6659d0cec684` | ✅ 未变 |

⇒ **未被改造为「原始报告」**，与 ORIGINAL 保持可区分。

---

## 9. LOST EVIDENCE REGISTER

| # | 丢失项 | 大小 | 状态 | 可否恢复 |
|---|---|---|---|---|
| 1 | DOC-2 报告 16:04 原版（被 16:08:36 的 REPAIR prompt 覆盖） | 25,664 B | **PERMANENTLY UNRECOVERABLE** | ❌ 不可 |
| 2 | REORG-1I 原始执行报告 | 未知 | **LOST / REFERENCE-ONLY** | ❌ 不可 |
| 3 | 6 份阶段的原始执行报告（1G / G08 / G09 / R3 / R4A-PRE-GATE / R4A-FINAL） | 未知 | 已 RECONSTRUCTED，**永久 ≠ 原始** | ❌ 不可 |
| 4 | G09 原始空壳的**原始路径内容** | 35 B | 已备份 2 份，**未丢失**（`REORG-G09-empty-shell.original.txt` + 快照） | ✅ 已保全 |
| 5 | DOC-2 被覆盖前的 8,810 B 版本 | 8,810 B | 已存档为 `REORG-DOC-2-REPAIR.prompt.md`（`d6dd36ad…a001db4`） | ✅ 已保全 |

**本阶段未新增任何丢失项，也未伪恢复任何一项。**

---

## 10. HISTORICAL EVIDENCE LIMITATIONS

以下 **documentation-only limitation** 全部**非本阶段产生，且非本阶段可消除** —— 这是裁定 `PASS WITH DOCUMENTATION GAPS` 的唯一理由：

| # | Limitation | 性质 |
|---|---|---|
| 1 | 6 份报告为 RECONSTRUCTED，**永久不等于原始执行报告** | 不可消除 |
| 2 | 1I 永久 LOST，仅存 6 项二手引用（来自 1J prompt 转述） | 不可消除 |
| 3 | DOC-2 16:04 原版 25,664 B **PERMANENTLY UNRECOVERABLE** | 不可消除 |
| 4 | 1I 编号歧义的**消歧动作**（②重编号为 `REORG-1J-PRE-GATE`）**仍未执行**，需单独授权 | 需授权 |
| 5 | 工作区根仍有**非 REORG** 阶段报告在 git 之外（DOC-2 §2 记 57 份）—— 超出本阶段授权 | 超出范围 |

**另需注意（非 limitation，属既有已登记事实）**：

| 项 | 状态 |
|---|---|
| evidence 行号漂移 2 处（`p2p_branch_test.go:593`→590/629；`r3_crash_matrix_test.go:172`→168） | **原样保留，未为「统一」而修改历史** |
| evidence 命名矛盾 1 处（G09 R1，`2026-09-15.md:299` vs `:373`） | **原样保留**，canonical 采用 `:373` + R4C §3.1 行 75 |
| `MANIFEST.tsv` DOC-2 条目 stale（8,810 vs 26,975 B） | **原文件未修改**，由 `MANIFEST.verified.tsv` 补充 |

---

## 11. GIT SAFETY RESULT

| 检查 | 阶段开始 | 阶段结束 | 结果 |
|---|---|---|---|
| `git rev-parse HEAD` | `e04d678b4fb33a14ea186999375f6d6dac2a4c4b` | 同 | ✅ 未变 |
| `git diff --cached --name-only` | 0 | **0** | ✅ staged = 0 |
| `git diff --name-only` | 11 个长期 dirty | 同（未新增） | ✅ |
| `git rev-list --all --count` | 35 | 35 | ✅ 无新 commit |
| `git tag` | 0 | 0 | ✅ |
| `git stash list` | 0 | 0 | ✅ |
| `.git/rebase-merge` / `rebase-apply` / `MERGE_HEAD` / `CHERRY_PICK_HEAD` | absent | absent | ✅ 无 merge/rebase 状态 |
| `git add` | — | **未执行** | ✅ |
| `git commit` / `push` / `tag` | — | **未执行** | ✅ |

**本阶段新增未跟踪项（全部未 `git add`）**：
`docs/phases/`（6 份）、`docs/_archive_pre-doc2/ORIGINAL-REFERENCE-ONLY-GOVERNANCE.md`、
`docs/_archive_pre-doc2/reference-only/REORG-1I-FINAL-DISAMBIGUATION-GOVERNANCE.md`、本最终报告。

**用户既有 staging 报告**：无（staged = 0，无需报告亦无需清理）。

---

## 12. PRODUCTION/TEST ZERO-DIFF RESULT

逐文件列出（阶段开始 == 阶段结束）：

| SHA-256 | 文件 |
|---|---|
| `f627b537529c9280272b6837db8a33ffd41a99eb5b1db580afbb41d21fc27aec` | `cmd/node/main.go` |
| `a751dc60901ff46f4b9727e230dd5168ab6a03cf24124e3786896d32cb34f645` | `cmd/node/service.go` |
| `ff03e8aaf3c2660f93c1f9f3f273009d7240be28c72d465b345d18c42def36b4` | `internal/blockchain/blockchain.go` |
| `9656c0667a8ea4d70769db4a197a032b205f3b8b722472af50e348947b86d8db` | `internal/blockchain/query.go` |
| `f33cc50e37e6a51a93b66ad275e7723b47ccfaf8cd2258bca60bfb0b3bde3d5a` | `internal/p2p/node.go` |
| `1cd1a46c563024ef16070ffe511092cbed14d6b5fadbbc485943bb49d3dc09a5` | `internal/p2p/node_test.go` |
| `5fc6d3684d82214c0b087c426963deb88bd0b748c5fab360ca7be4be9008e12b` | `internal/storage/file.go` |
| `603ba1dd86166921cceb1441c588856ab516ecd61a49ec588e6337b005e11678` | `internal/storage/v2.go` |
| `fdfc7bf542613232140b75298a8c04db3a7cfee12adce1f3f82bcabae2578bff` | `internal/storage/v2_test.go` |
| `f16fee42442951650b568038bdf4d8a62f6e869e132982d4bb90cf45c96d6ab5` | `internal/storage/v2api.go` |
| `b1f75ce202f20a2f19962bcffb64d91a4da34b05f7f301f60a70e413973a7da4` | `cmd/node/p2p_branch_process_test.go` |
| `39b6c350149159b67d2d9e486bd50c0d6a0416f7537db8b6922d7dbd13ad7284` | `cmd/node/p2p_branch_test.go` |
| `c5decb8907a0c30492bd9c49c283fa3da63b0ce488aa685f641c8564fb9eecf3` | `cmd/node/r3_crash_restart_test.go` |
| `136b095b5382efaf38d6da0c22d9dea6cdff8345dd77c369541b2240ed29ffdc` | `cmd/node/r4b_dual_reorg_convergence_test.go` |
| `73fbf1ea74259c65e936a0fd2cee620bae2d975ea70c842aac9e649f376b68d6` | `internal/storage/r3_crash_matrix_test.go` |
| `a51bfa925a9827e81fc2e874d06a05966585d8c86ccab3aa7a3eeb301a845f88` | `internal/storage/r4a_legacy_length_matrix_test.go` |

**结果：16/16 生产/测试文件哈希全等 ⇒ diff = 0。**

⇒ consensus / storage / P2P / reorg execution / UTXO / TIP·commit semantics 改动全部为 **0**。

---

## 13. REMAINING DEFERRED ITEMS

| # | 项 | 类型 | 需授权 |
|---|---|---|---|
| 1 | 1I 编号消歧**动作**（②重编号为 `REORG-1J-PRE-GATE` + MEMORY 固化） | DOC | ✅ 需单独授权 |
| 2 | 工作区根非 REORG 阶段报告纳入版本控制（DOC-2 §2 记 57 份） | DOC | ✅ 需单独授权 |
| 3 | R5 提交整理（`git add` 全部报告 + 提交说明登记 6 RECONSTRUCTED / 1 LOST / 5 项 limitation / 全部 DEFERRED） | GIT | ✅ 需授权 |
| 4 | 防复发机制落地（「报告写完即快照」成为强制流程） | 流程 | ✅ |
| 5 | BT-1（`[TEST CONSTRUCTION DEFECT]`，引用全量绿灯须排除 `internal/blocktree`） | 测试 | 已知 DEFERRED |
| 6 | OBS-1J-R3-A（P3） | 代码 | 已知 DEFERRED |
| 7 | R4A-OBS-1（P3，语义澄清） | 代码 | 已知 DEFERRED |
| 8 | finality / MaxReorgDepth / orphan pool / fork 观测 | 功能 | 已知 DEFERRED |
| 9 | 长期 dirty 项（R5 须排除）：`docs/DETERMINISTIC-SERIALIZATION-SPEC.md`、`docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md`、`run-a/`、`run-b/`、`verifier/` | GIT | R5 处理 |

---

## 14. RECOMMENDATION FOR NEXT PHASE

```text
唯一推荐：
REORG-1J-R5 — COMMIT ORGANIZATION

前置条件（R5 提交说明必须显式登记）：
  ① 6 份 RECONSTRUCTED FROM EVIDENCE 报告 —— 不得记为原始报告
  ② 1 份 LOST（REORG-1I）—— 不得记为已交付
  ③ 5 项 documentation-only limitation（§10）
  ④ 9 项 remaining deferred items（§13）
  ⑤ 必须排除长期 dirty：docs/DETERMINISTIC-SERIALIZATION-SPEC.md、
     docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md、run-a/、run-b/、verifier/
  ⑥ 本次新增需显式 git add：
     - p2pchain/docs/phases/（6 份 ORIGINAL，本次已移入 git 工作树）
     - p2pchain/docs/prompts/（25 PROMPT-ARCHIVE + 1 阶段 prompt + 1 空壳备份）
     - p2pchain/docs/_archive_pre-doc2/（快照与治理索引）
     - 6 份 RECONSTRUCTED 报告
     - 3 份 governance / final report

不推荐：本阶段不自动进入任何代码/功能/部署阶段。
```

**技术闭环不受影响（Rule C / Rule D 复核）**：本阶段为纯 documentation governance，
未触碰 consensus / storage / P2P / reorg execution / UTXO / TIP·commit semantics。
R4C 的 `CONDITIONALLY CLOSED / R5 READY` 与 `GAP-1H-A = CLOSED` **继续成立**。

---

## 15. STOP

```text
STEP 5  ORIGINAL GOVERNED     : 6 / 6 MOVE-OK (hash preserved)
STEP 6  1I DISAMBIGUATION     : DONE (LOST / REFERENCE-ONLY maintained)
FILES OVERWRITTEN             : 0
FILES DELETED                 : 0
HISTORICAL EVIDENCE MODIFIED  : 0
MANIFEST.tsv MODIFIED         : NO
RECONSTRUCTED REPORTS MODIFIED: 0 / 6
PRODUCTION/TEST CODE DIFF     : 0 (16/16 hashes identical)
CONSENSUS / STORAGE / P2P DIFF: 0 / 0 / 0
COMMIT                        : NO
PUSH                          : NO
TAG                           : NO
MERGE / REBASE / AMEND / SQUASH : NO
DEPLOY                        : NO
WINDOWS MINING                : NO
```

**本阶段到此结束。**

不自动进入 R5、不修改 R5、不实施任何 reorg code、不 commit、不 push、不 tag、不部署、
不处理任何 deferred items。只提交本最终报告，等待下一条明确授权。

> 按 DOC-2 §8 Step 8 防复发要求，本文件写入后**立即建立独立快照**至
> `docs/_archive_pre-doc2/repo/PHASE-REORG-1J-DOC-2-ARCHIVE-GOVERNANCE-FINAL-REPORT.md`。

---

## 附：本阶段新增文件清单（全部未 git add）

| 路径 | 类型 |
|---|---|
| `p2pchain/docs/phases/`（6 份 ORIGINAL，MOVE 而来） | 归档治理 |
| `docs/_archive_pre-doc2/ORIGINAL-REFERENCE-ONLY-GOVERNANCE.md` | Step 5 治理索引 |
| `docs/_archive_pre-doc2/reference-only/REORG-1I-FINAL-DISAMBIGUATION-GOVERNANCE.md` | Step 6 治理证据 |
| `PHASE-REORG-1J-DOC-2-ARCHIVE-GOVERNANCE-FINAL-REPORT.md` | 本文件 |
