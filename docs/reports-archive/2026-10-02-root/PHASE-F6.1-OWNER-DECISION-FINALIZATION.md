# PHASE F-6.1-OWNER-DECISION-FINALIZATION

**PHASE F-6.1-OWNER-DECISION-FINALIZATION**
**OWNER DECISION FINALIZATION ｜ STRICT READ-ONLY ｜ NO CODE CHANGE ｜ NO FILE RESTORE ｜ NO FILE DELETE ｜ NO GIT MUTATION ｜ SCOPE FREEZE**

| 项 | 值 |
|---|---|
| 仓库 | `E:/wakuang/p2pchain` |
| 基线 HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4`（branch `main`） |
| staged | **0** |
| tracked 内容修改（`git diff --name-only`） | **36** |
| porcelain ` M` | 37（含 1 个 CRLF-only 伪差异） |
| untracked（porcelain `??`） | **195** |
| `.gitignore` | **未修改**（sha `f43418cf74ea996cc603976f29badc4c507e6b3ace25f7f619d7d79aefb6567b`） |
| 本阶段性质 | **Owner 决策最终化 + 范围冻结（非开发 / 非 commit / 非 cleanup / 非 restore）** |
| 本阶段禁止 | `git add/commit/reset/restore/checkout/clean/stash/rebase/merge/push/tag`；恢复 P3.1；删除 BRAND-0D.3 orphan；修改 GAP-4 文件；修复 SHA drift；修改 Console 文件；修复 flaky test；修改 `.gitignore`；读取 token 实际内容；移动/复制任何项目文件 |
| 本阶段允许 | 只读 / 证据复验 / 决策记录 / 范围冻结 / report generation |

---

## §1 — BASELINE VERIFICATION（基线验证）

### 1.1 权威输入文件 SHA-256 复验（**全部一致**）

| 权威输入 | 记录 SHA-256 | 本阶段复算 | 判定 |
|---|---|---|---|
| `PHASE-F6.1-OWNER-SCOPE-DECISION.md` | `3524b1659baa5d34f0d143e68f6f03e66000314715851a6af43a88c18f26442c` | 同 | ✅ **MATCH** |
| `PHASE-F6.1-FILE-PROVENANCE-MATRIX.md` | `86d20f62e2446811cde9c0452e15745e51c7d49aa8194aab101a4e2bbcf53f4d` | 同 | ✅ **MATCH** |
| `PHASE-F6.1-SECRET-ARTIFACT-INVENTORY.md` | `a4a36f7095a5fabc1a43a029c78e318036ce5911bb6082932f7672870fbfc492` | 同 | ✅ **MATCH** |
| `PHASE-F6.1-OD-OWNER-DECISION-CAPTURE.md` | `b0c09b06f1c164cd53b0e12603962b328dc8ba607d49e2a3383dd8d67f8d8f32` | 同 | ✅ **MATCH** |

> 四份权威输入**逐字节一致**，无漂移。本阶段决策即建立在**已验证**的证据基线之上。

### 1.2 仓库基线复验

| 项 | F-6.1-OD 收工记录 | 本阶段复验 | 判定 |
|---|---|---|---|
| HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` | `4d892be355a38886a83c123faad915c9f3cd62e4` | ✅ 一致 |
| staged | 0 | 0 | ✅ 一致 |
| tracked 内容修改 | 36 | 36 | ✅ 一致 |
| porcelain ` M` | 37 | 37 | ✅ 一致 |
| untracked（`??`） | 195 | 195 | ✅ 一致 |
| `.gitignore` | 未修改 | 未修改（sha `f43418cf…`） | ✅ 一致 |
| 受保护对象 `node.exe` | `3972843a…c213e1` | `3972843aff90428d…` | ✅ 一致 |
| `run-a/` / `run-b/` | 存在 | 存在 | ✅ 一致 |

**BASELINE VERDICT = PASS。允许进入决策最终化。**

### 1.3 一处事实性更正（记录，非改判）

OD-01 授权文本引用 commit 短哈希为 `535ed7b`。经复验，**唯一**已确认的 P3.1 provenance commit 为：

```
535edb71e7aecbd6e100534bb65b595e665dd26a   （短：535edb7）
2026-09-12 21:14:42 +0800
feat(storage): harden data lock lifecycle
```

`git merge-base --is-ancestor 535edb7 HEAD` → **通过**（该 commit **是 HEAD 的祖先**）。
本报告统一采用 `535edb7` / 全哈希 `535edb71e7aecbd6e100534bb65b595e665dd26a`。**决策内容不受影响。**

---

## §2 — DECISION AUTHORITY（决策权威来源）

本阶段**不产生新证据、不做新推断**，仅将 F-6.1 / F-6.1-OD 已确立的证据**转化为 Owner 最终决策记录**。

| 决策来源 | 角色 | 本阶段动作 |
|---|---|---|
| **Owner（用户）** | **唯一决策权威** | 下达 OD-01 … OD-08 的最终决策（本阶段 §3–§10 逐条记录） |
| `PHASE-F6.1-OWNER-SCOPE-DECISION.md` | 归属裁定准备（SET A/B/C + OD 表） | 被引用，**未修改** |
| `PHASE-F6.1-FILE-PROVENANCE-MATRIX.md` | 逐文件 E1–E4 证据矩阵 | 被引用，**未修改** |
| `PHASE-F6.1-SECRET-ARTIFACT-INVENTORY.md` | 凭据产物清单 | 被引用，**未修改** |
| `PHASE-F6.1-OD-OWNER-DECISION-CAPTURE.md` | 证据捕获（8 项 OD `PENDING`） | 被引用，**未修改** |

**归属证据优先级（本阶段重申，不可违反）**：

```
E1 mtime 聚类
E2 文件头 phase 自述
E3 符号依赖
E4 授权痕迹（报告 / prompt / git log）
```

- **E3 只证明 dependency，永远不能单独证明 ownership。**
- **E1 单独不足以裁定**（F-6.1-OD §2.2 已证：`2026-09-22 14:45` 窗口跨 GAP-4 与 F1-N1 两个工作流）。
- **禁止**以「该文件现在能让项目编译」为由归入当前阶段。
- **禁止**以「文件看起来属于某 phase」为由认定授权。

---

## §3 — OD-01 FINAL DECISION

**对象**：`docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md`

### OWNER DECISION
```
ACCEPT REAL P3.1 PROVENANCE.
```

### 理由（Owner 陈述 + 证据支撑）
- `535edb7`（`535edb71e7aecbd6e100534bb65b595e665dd26a`）是 **HEAD 的祖先**（`git merge-base --is-ancestor` 通过）。
- 该 commit 是**唯一已确认的 P3.1 provenance**（`git log -- <path>` → 仅 1 commit）。
- 当前 working-tree 内容是 **orphan BRAND-0D.3**（`# PHASE BRAND-0D.3 — FULL READ-ONLY VALIDATION`，9,373 B，sha `98f036fbdd804fa8e525a6d0b08edd0e6eea4213f8fb088ce312f8dd9446cc21`）。
- 当前内容与**已发现 BRAND-0D.3 archive**（5 份：`-COMMIT-AUDIT` / `-COMMIT-REPORT` / `-DEVELOPER-CONSOLE-BASELINE-REPORT` / `-FRESH-VERIFICATION` / `-TECHNICAL-FOUNDATION`）**均不匹配**。
- ⇒ 当前 working-tree 内容**不能被认定为 P3.1**。

### 处理
- **P3.1 后续进入独立恢复 / 保全阶段**（`P3.1 RESTORATION`）。
- 本阶段：**DO NOT RESTORE. DO NOT DELETE. DO NOT OVERWRITE.**

### 归属判定
| 字段 | 值 |
|---|---|
| Ownership | **P3.1（真实 provenance 已被 Owner 接受）** |
| Governance Status | **RESOLVED（provenance 层面）** |
| Commit Eligibility | **BLOCKED** —— 工作树内容 ≠ P3.1 内容（内容错配） |
| Blocking Condition | P3.1 独立恢复/保全阶段完成（**本阶段不执行**） |
| Next Phase | **P3.1 RESTORATION（独立阶段，须单独授权）** |

---

## §4 — OD-02 FINAL DECISION

**对象**：`docs/DETERMINISTIC-SERIALIZATION-SPEC.md`

### OWNER DECISION
```
ASSIGN TO PHASE V2.0 — INDEPENDENT VERIFIER.
```
**不得归入 F-1～F-5 Protocol Closure。V2.0 lineage 已得到证据支持。**

### 理由（证据支撑）
- `git log -- <path>` → 仅 1 commit：`91dfee6 docs: document developer node and deterministic verification`。
- 工作树追加 §9.11（P-256 signature encoding）/ §9.12（Base58Check），自述「PHASE V2.0 补录」。
- HEAD SHA `33f67a69fe57f1c0098929aa4c2c56004e888fdbadd8df6e6c96d6f3689cd9bb` → 工作树 SHA `5caf38b23f8b842349360b5a96829d42aa388222a481783f8ef90bf2487936f1`。
- 该内容是**实现事实的补录**（曲线 P-256 / 非压缩 SEC1 65B / 签名 r‖s 64B），**与 F-1～F-5 Genesis Identity 无关**。
- **旁证（绑定关系）**：`internal/attribution/attribution.go` 自述「格式权威：`docs/DETERMINISTIC-SERIALIZATION-SPEC.md` §9.1–9.9」⇒ 该文档是 **GAP-4 工作流的依赖**。

### 归属判定
| 字段 | 值 |
|---|---|
| Ownership | **PHASE V2.0 — INDEPENDENT VERIFIER** |
| Governance Status | **RESOLVED** |
| Commit Eligibility | **BLOCKED（本链）** —— 属 V2.0，不随 F-1～F-5 提交 |
| Blocking Condition | 若 OD-04 的 GAP-4 决定纳入，则本文档**必须**同批纳入（格式权威绑定） |
| Next Phase | **V2.0 — INDEPENDENT VERIFIER（归档）** ／ 或随 **GAP-4** 提交 |

---

## §5 — OD-03 FINAL DECISION

**对象**：`internal/blockchain/f1n1_reorg_coverage_test.go`（F-03）

### OWNER DECISION
```
ASSIGN TO F1-N1.
```

### 理由（Owner 陈述 + 证据支撑）
- F1-N1 **有正式治理记录**（`docs/prompts/F1-GAP-CONTRACT.md`）。
- `F1-GAP-CONTRACT` **明确记录该阶段**。
- report SHA **已验证**：`E:/wakuang/.workbuddy/F1-N1-UNDO-CONTRACT-TEST-DESIGN/PHASE-P2PCHAIN-F1-N1-…-FINAL-REPORT.md`
  = `42462d9b2c3e2e41eb5eca52348030aa560bb3b5fda8c52b1d42950dcdde0f3d`（与记录**逐字节一致**）。
- **编译依赖不能改变 ownership**（E3 只证明 dependency）。
- 但 **provenance evidence 足以确认 F1-N1 ownership**（E2 文件头自述 `PHASE P2PCHAIN — F1-N1 …` + E4 报告）。

### 归属判定
| 字段 | 值 |
|---|---|
| Ownership | **F1-N1** |
| Governance Status | **RESOLVED**（**不是** F-1～F-5） |
| Commit Eligibility | **BLOCKED** —— 授权报告位于**仓库之外**；且 F1-N1 尚未作为独立提交阶段授权 |
| Blocking Condition | F1-N1 独立提交阶段授权 + 报告归档决策 |
| Next Phase | **F1-N1（授权与归档）** |

> **CRITICAL**：本文件**编译必需**（被 tracked、未修改的 `p1_reorg_fail_before_commit_test.go:63` 引用），
> 但**编译必需 ≠ 阶段归属**。归属依 E2 + E4，**不依 E3**。

---

## §6 — OD-04 FINAL DECISION

**对象**：`internal/attribution/*`（`attribution.go` / `classify.go` / `attribution_test.go`）

### OWNER DECISION
```
ASSIGN TO GAP-4.
```
**状态必须记录：`ACCEPTED WITH SHA-DRIFT BLOCKER`。**

### 理由（Owner 陈述 + 证据支撑）
- 授权报告存在：`docs/PHASE-P2PCHAIN-GAP-4-COINBASE-ATTRIBUTION-IMPLEMENTATION-REPLAY-AUDIT-REPORT.md`
  （**未跟踪**，mtime `2026-09-20 20:07`，sha `0bf64bf6cd755e471629580079b453da5dad3b00edf89de51e566bfd8bfd66ad`）。
- **但 4 个文件当前 SHA 与报告登记 SHA 全部不一致**（见 §15）。

### 归属判定
| 字段 | 值 |
|---|---|
| Ownership | **GAP-4** |
| Governance Status | **ACCEPTED WITH SHA-DRIFT BLOCKER** |
| Commit Eligibility | **BLOCKED** |
| Blocking Condition | **SHA-DRIFT CLOSURE**（单独阶段完成后方可评估提交资格） |
| Next Phase | **GAP-4 — SHA-DRIFT CLOSURE**（**本阶段不得修复**） |

---

## §7 — OD-05 FINAL DECISION

**对象**：`cmd/coinbase-attribution/main.go`

### OWNER DECISION
```
ASSIGN TO GAP-4.
```
**状态：`ACCEPTED WITH SHA-DRIFT BLOCKER`（与 OD-04 共用同一个 SHA-DRIFT CLOSURE）。**

### 理由（证据支撑）
- 是 `internal/attribution` 的**唯一** importer（`main.go:26`）⇒ 包 + 入口**必须同进同出**。
- 无 tracked 反向引用。
- 同样命中 **SHA drift**：报告 `c0fe645c1962762b6c5672fa1b39acb3c6b2cacd92e826cb4ca2a913f95c703d`
  vs 当前 `c634c1ecaea5955118533d035af2d0db2fa8f18365fa579a22c0b7401bfa6f18`。

### 归属判定
| 字段 | 值 |
|---|---|
| Ownership | **GAP-4** |
| Governance Status | **ACCEPTED WITH SHA-DRIFT BLOCKER** |
| Commit Eligibility | **BLOCKED** |
| Blocking Condition | **SHA-DRIFT CLOSURE**（与 OD-04 同一条件） |
| Next Phase | **GAP-4 — SHA-DRIFT CLOSURE** |

---

## §8 — OD-06 FINAL DECISION

**对象**：F1-N1 ×2（`internal/storage/f1n1_undo_contract_test.go`、`internal/utxo/f1n1_undo_content_test.go`）
＋ F1-N2 ×2（`internal/blockchain/f1n2_reorg_detached_undo_test.go`、`internal/storage/f1n2_detached_undo_contract_test.go`）

### OWNER DECISION
```
FORMALLY ASSIGNED TO F1-N1 / F1-N2.
```

### 理由（Owner 陈述 + 证据支撑）
- `F1-GAP-CONTRACT` **明确登记**该阶段（`docs/prompts/F1-GAP-CONTRACT.md`）。
- 两份 report SHA **已验证**：
  - F1-N1 = `42462d9b2c3e2e41eb5eca52348030aa560bb3b5fda8c52b1d42950dcdde0f3d`
  - F1-N2 = `ddc8e4d7fd7c02920aee930e09de250647e4a8e0d86f73d86dda14681bbe3011`
- **文件数量与报告登记完全对应**（F1-N1 ×2 + F1-N2 ×2 = 4）。
- **provenance evidence 完整**（E2 自述 + E4 报告）。

### 注意
- 外部 `.workbuddy` report **仅作为 provenance evidence**。
- **本阶段不得复制进入 repo**（`E:/wakuang/.workbuddy/` 位于仓库之外）。

### 归属判定
| 字段 | 值 |
|---|---|
| Ownership | **F1-N1（×2）/ F1-N2（×2）** |
| Governance Status | **RESOLVED** |
| Commit Eligibility | **BLOCKED** —— 报告位于仓库之外；F1-N1/N2 尚未作为独立提交阶段授权 |
| Blocking Condition | F1-N1 / F1-N2 独立提交阶段授权 + 报告归档决策 |
| Next Phase | **F1-N1 / F1-N2（授权与归档）** |

---

## §9 — OD-07 FINAL DECISION（拆分处理）

### OD-07-A — `CONSOLE-MINE-AUTH-FIX-1`

**对象**：`internal/control/server.go`、`internal/control/console_test.go`、`internal/control/web/console.html`、`internal/control/console_mine_auth_test.go`

#### OWNER DECISION
```
AUTHORIZED WORKSTREAM
```
**但：`GOVERNANCE EVIDENCE GAP`** —— 未发现正式 phase report。

**证据**：`grep -rl 'CONSOLE-MINE-AUTH-FIX-1' --include='*.md'` → **仅命中本链自身 F-6/F-6.1 报告**（旁证），
**无任何阶段报告**；该字符串仅出现在源码/测试文件与若干 `.exe` 中。

| 字段 | 值 |
|---|---|
| Ownership | **CONSOLE-MINE-AUTH-FIX-1** |
| Governance Status | **AUTHORIZED WORKSTREAM + GOVERNANCE EVIDENCE GAP** |
| Commit Eligibility | **BLOCKED** |
| Blocking Condition | **GOVERNANCE CLOSURE = REQUIRED**（补产阶段报告） |
| Next Phase | **CONSOLE GOVERNANCE CLOSURE** |

### OD-07-B — `AUDIT-FIX-RUN-MINING-1`

**对象**：`internal/explorer/explorer_test.go`、`cmd/node/r1_boundary_test.go`

#### OWNER DECISION
```
AUTHORIZED WORKSTREAM
```
外部 report：`E:/wakuang/PHASE-AUDIT-FIX-RUN-MINING-1-REPORT.md`
（15,317 B，mtime `2026-09-27 09:15`，sha `5e5e8142014e79932682d0ebd4f058b1befdf0464ca9edbdff76f5c0c5ddb507`）。

**当前阶段只记录其 provenance。不得复制、移动、删除。**

| 字段 | 值 |
|---|---|
| Ownership | **AUDIT-FIX-RUN-MINING-1** |
| Governance Status | **AUTHORIZED WORKSTREAM（report 在仓库之外）** |
| Commit Eligibility | **BLOCKED** —— 报告未归档入 repo |
| Blocking Condition | 报告归档决策 + 独立提交阶段授权 |
| Next Phase | **AUDIT-FIX-RUN-MINING-1（归档与提交）** |

---

## §10 — OD-08 FINAL DECISION

**对象**：`audit-run/control-token`、`f5-verify/control-token`、`gui-test/token`

### OWNER DECISION
```
PERMANENTLY EXCLUDED FROM GIT.
```

### 规则（MANDATORY，永久有效）
```
NEVER git add
NEVER git commit
NEVER git restore
NEVER git clean
NEVER print token contents
NEVER migrate token into another artifact.
```

### 证据支撑
- 三者 **tracked = NO，ignored = NO**（`git check-ignore -q` 全部 exit 1）。
- `.gitignore`（sha `f43418cf…`）**无任何 token / secret 模式**。
- 逐文件档案见 `PHASE-F6.1-SECRET-ARTIFACT-INVENTORY.md`（S-1 / S-2 / S-3）。
- 本阶段**未读取任何 token 实际内容**。

### 归属判定
| 字段 | 值 |
|---|---|
| Ownership | **GOVERNANCE（secret artifact）** |
| Governance Status | **PERMANENTLY EXCLUDED FROM GIT** |
| Commit Eligibility | **NONE（永久排除）** |
| Blocking Condition | 不适用（永不提交）；`.gitignore` 加固须单独 Governance 阶段授权 |
| Next Phase | **GOVERNANCE — `.gitignore` secret hardening**（可选，须单独授权） |

---

## §11 — FINAL SCOPE MATRIX（最终范围矩阵）

> 每个对象具备 7 列：**Object ｜ Ownership ｜ Evidence ｜ Governance Status ｜ Commit Eligibility ｜ Blocking Condition ｜ Next Phase**。
> 本矩阵为 **FROZEN**（冻结）。

### 11.1 SET A — F-1～F-5 Protocol Closure（32 代码文件）

| # | Object | Ownership | Evidence | Governance Status | Commit Eligibility | Blocking Condition | Next Phase |
|---|---|---|---|---|---|---|---|
| A-1 | `cmd/node/main.go` | F-3B | E2/E3/E4（F-3A §13 / F-3B 报告） | RESOLVED | ✅ ELIGIBLE | — | F-6.3 Commit |
| A-2 | `cmd/node/cli.go` | F-3B | E2/E3/E4（F-3A OD-03） | RESOLVED | ✅ ELIGIBLE | — | F-6.3 Commit |
| A-3 | `internal/blockchain/blockchain.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-01（编译必需） | F-6.3 Commit |
| A-4 | `internal/blockchain/verify.go` | F-3B | E2/E3/E4（F-3A OD-05） | RESOLVED | ✅ ELIGIBLE | 依赖 F-01 | F-6.3 Commit |
| A-5 | `internal/blockchain/genesis.go` | F-3B | E2/E4 | RESOLVED | ✅ ELIGIBLE | — | F-6.3 Commit |
| A-6 | `internal/storage/file.go` | F-3B | E2/E4 | RESOLVED | ✅ ELIGIBLE | — | F-6.3 Commit |
| A-7 | `internal/storage/v2.go` | F-3B | E2/E4 | RESOLVED | ✅ ELIGIBLE | — | F-6.3 Commit |
| A-8 | `cmd/node/datalock_integration_test.go` | F-3B | E2/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-02 | F-6.3 Commit |
| A-9 | `cmd/node/fullstack_test.go` | F-3B | E2/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-02 | F-6.3 Commit |
| A-10 | `cmd/node/p2p_branch_test.go` | F-3B | E2/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-02 | F-6.3 Commit |
| A-11 | `cmd/node/r3_crash_restart_test.go` | F-3B | E2/E4 | RESOLVED | ✅ ELIGIBLE | — | F-6.3 Commit |
| A-12 | `cmd/node/r4b_dual_reorg_convergence_test.go` | F-3B | E2/E4 | RESOLVED | ✅ ELIGIBLE | — | F-6.3 Commit |
| A-13 | `cmd/node/reset_test.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-01 | F-6.3 Commit |
| A-14 | `cmd/node/seed_parallel_test.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-02 | F-6.3 Commit |
| A-15 | `cmd/node/verify_test.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-01 | F-6.3 Commit |
| A-16 | `internal/blockchain/blockchain_test.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-01 | F-6.3 Commit |
| A-17 | `internal/blockchain/c1_hashindex_test.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-01 | F-6.3 Commit |
| A-18 | `internal/blockchain/c1_noninterference_dump_test.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-01 | F-6.3 Commit |
| A-19 | `internal/blockchain/cd_canonset_test.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-01 | F-6.3 Commit |
| A-20 | `internal/blockchain/difficulty_consensus_test.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-01 | F-6.3 Commit |
| A-21 | `internal/blockchain/obs_noninterference_test.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-01 | F-6.3 Commit |
| A-22 | `internal/blockchain/reorg_test.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-01 | F-6.3 Commit |
| A-23 | `internal/blockchain/replay_persistence_test.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-01 | F-6.3 Commit |
| A-24 | `internal/mempool/reorg_resurrection_test.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-01 | F-6.3 Commit |
| A-25 | `internal/storage/r3_crash_matrix_test.go` | F-3B | E2/E4 | RESOLVED | ✅ ELIGIBLE | — | F-6.3 Commit |
| A-26 | `internal/storage/r4a_legacy_length_matrix_test.go` | F-3B | E2/E4 | RESOLVED | ✅ ELIGIBLE | — | F-6.3 Commit |
| A-27 | `internal/storage/v2_test.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-01 | F-6.3 Commit |
| A-28 | `cmd/node/stop_lifecycle_test.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | — | F-6.3 Commit |
| A-29 | `cmd/node/lock_lifecycle_test.go` | F-3B | E2/E3/E4 | RESOLVED | ✅ ELIGIBLE | 依赖 F-02 | F-6.3 Commit |
| A-30 | `internal/blockchain/identity.go`（untracked, **F-01**） | F-3B | E2/E3/E4（被 15 个 tracked 引用） | RESOLVED | ✅ ELIGIBLE（**编译必需**） | — | F-6.3 Commit |
| A-31 | `cmd/node/f3b_test_helpers_test.go`（untracked, **F-02**） | F-3B | E2/E3/E4（被 5 个 tracked 引用） | RESOLVED | ✅ ELIGIBLE（**编译必需**） | — | F-6.3 Commit |
| A-32 | `cmd/node/f3b_identity_test.go`（untracked） | F-3B | E2/E4（F-3B 报告明列 9 子断言） | RESOLVED | ✅ ELIGIBLE | — | F-6.3 Commit |

### 11.2 SET B — Authorized Other Workstreams（6 代码文件）

| # | Object | Ownership | Evidence | Governance Status | Commit Eligibility | Blocking Condition | Next Phase |
|---|---|---|---|---|---|---|---|
| B-1 | `internal/control/server.go` | CONSOLE-MINE-AUTH-FIX-1 | E2（注释自述） | AUTHORIZED + **GOVERNANCE EVIDENCE GAP** | ⚠️ BLOCKED | 补产阶段报告 | CONSOLE GOVERNANCE CLOSURE |
| B-2 | `internal/control/console_test.go` | CONSOLE-MINE-AUTH-FIX-1 | E2 | AUTHORIZED + **GOVERNANCE EVIDENCE GAP** | ⚠️ BLOCKED | 补产阶段报告 | CONSOLE GOVERNANCE CLOSURE |
| B-3 | `internal/control/web/console.html` | CONSOLE-MINE-AUTH-FIX-1 | E2 | AUTHORIZED + **GOVERNANCE EVIDENCE GAP** | ⚠️ BLOCKED | 补产阶段报告 | CONSOLE GOVERNANCE CLOSURE |
| B-4 | `internal/control/console_mine_auth_test.go`（untracked） | CONSOLE-MINE-AUTH-FIX-1 | E2（文件头自述） | AUTHORIZED + **GOVERNANCE EVIDENCE GAP** | ⚠️ BLOCKED | 补产阶段报告 | CONSOLE GOVERNANCE CLOSURE |
| B-5 | `internal/explorer/explorer_test.go` | AUDIT-FIX-RUN-MINING-1 | E4（外部报告 D-2 节，`5e5e8142…`） | AUTHORIZED（report 在仓库之外） | ⚠️ BLOCKED | 报告归档 + 提交授权 | AUDIT-FIX 归档与提交 |
| B-6 | `cmd/node/r1_boundary_test.go` | AUDIT-FIX-RUN-MINING-1 | E4（外部报告，`+12/-4`） | AUTHORIZED（report 在仓库之外） | ⚠️ BLOCKED | 报告归档 + 提交授权 | AUDIT-FIX 归档与提交 |

### 11.3 SET C — Unknown / Unresolved（**本阶段已全部裁定**）

| # | Object | Ownership | Evidence | Governance Status | Commit Eligibility | Blocking Condition | Next Phase |
|---|---|---|---|---|---|---|---|
| C-1 | `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` | **P3.1（真实 provenance 已接受）** | E4（`535edb7`，HEAD 祖先，唯一 commit） | RESOLVED（provenance） | ❌ BLOCKED | 工作树内容错配（orphan BRAND-0D.3） | P3.1 RESTORATION |
| C-2 | `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` | **PHASE V2.0 — INDEPENDENT VERIFIER** | E2（「PHASE V2.0 补录」）+ E4（`91dfee6`） | RESOLVED | ❌ BLOCKED（本链） | 属 V2.0；若 GAP-4 纳入则须同批 | V2.0 归档 ／ GAP-4 |
| C-3 | `internal/blockchain/f1n1_reorg_coverage_test.go` | **F1-N1** | E2（自述 F1-N1）+ E4（报告 `42462d9b…`） | RESOLVED | ❌ BLOCKED | 报告在仓库之外；F1-N1 未授权提交 | F1-N1 授权与归档 |
| C-4 | `internal/storage/f1n1_undo_contract_test.go` | **F1-N1** | E2 + E4 | RESOLVED | ❌ BLOCKED | 同上 | F1-N1 授权与归档 |
| C-5 | `internal/utxo/f1n1_undo_content_test.go` | **F1-N1** | E2 + E4 | RESOLVED | ❌ BLOCKED | 同上 | F1-N1 授权与归档 |
| C-6 | `internal/blockchain/f1n2_reorg_detached_undo_test.go` | **F1-N2** | E2 + E4（报告 `ddc8e4d7…`） | RESOLVED | ❌ BLOCKED | 同上 | F1-N2 授权与归档 |
| C-7 | `internal/storage/f1n2_detached_undo_contract_test.go` | **F1-N2** | E2 + E4 | RESOLVED | ❌ BLOCKED | 同上 | F1-N2 授权与归档 |
| C-8 | `internal/attribution/attribution.go` | **GAP-4** | E2 + E4（报告 untracked） | **ACCEPTED WITH SHA-DRIFT BLOCKER** | ❌ BLOCKED | SHA-DRIFT CLOSURE | GAP-4 SHA-DRIFT CLOSURE |
| C-9 | `internal/attribution/classify.go` | **GAP-4** | E2 + E4 | **ACCEPTED WITH SHA-DRIFT BLOCKER** | ❌ BLOCKED | SHA-DRIFT CLOSURE | GAP-4 SHA-DRIFT CLOSURE |
| C-10 | `internal/attribution/attribution_test.go` | **GAP-4** | E2 + E4 | **ACCEPTED WITH SHA-DRIFT BLOCKER** | ❌ BLOCKED | SHA-DRIFT CLOSURE | GAP-4 SHA-DRIFT CLOSURE |
| C-11 | `cmd/coinbase-attribution/main.go` | **GAP-4** | E3（唯一 importer）+ E4 | **ACCEPTED WITH SHA-DRIFT BLOCKER** | ❌ BLOCKED | SHA-DRIFT CLOSURE | GAP-4 SHA-DRIFT CLOSURE |

> **SET C 冻结结果**：原「UNKNOWN / UNRESOLVED」的 10 个代码对象 + 2 个文档**全部获得明确 ownership**。
> **残余 UNKNOWN = 0**。但其中 **11 个对象带 BLOCKING CONDITION**（非 UNKNOWN，而是 **BLOCKED**）。
> ⇒ **UNKNOWN 已消除；BLOCKED 成为主要治理状态。**

### 11.4 其他对象

| # | Object | Ownership | Evidence | Governance Status | Commit Eligibility | Blocking Condition | Next Phase |
|---|---|---|---|---|---|---|---|
| E-1 | `internal/blockchain/query.go` | —（stat-only） | E1（`git diff --quiet` exit 0） | RESOLVED（零内容差异） | N/A（无内容修改） | — | — |
| S-1 | `audit-run/control-token` | GOVERNANCE（secret） | 见 SECRET-ARTIFACT-INVENTORY | **PERMANENTLY EXCLUDED FROM GIT** | ❌ NONE（永久） | — | GOVERNANCE（可选） |
| S-2 | `f5-verify/control-token` | GOVERNANCE（secret） | 同上 | **PERMANENTLY EXCLUDED FROM GIT** | ❌ NONE（永久） | — | GOVERNANCE（可选） |
| S-3 | `gui-test/token` | GOVERNANCE（secret） | 同上 | **PERMANENTLY EXCLUDED FROM GIT** | ❌ NONE（永久） | — | GOVERNANCE（可选） |

### 11.5 规模校验

```
SET A 代码：29 tracked + 3 untracked = 32
SET B 代码： 5 tracked + 1 untracked =  6
SET C 代码： 2 tracked + 9 untracked = 11   （F-03 + F1-N1/N2 ×4 + GAP-4 ×4）
             ------------------------------
tracked 合计：29 + 5 + 2 = 36 ✅
untracked 源文件合计：3 + 1 + 9 = 13 ✅
token：3（永久排除 Git）
E 类：1（stat-only，不计入 36）
```

---

## §12 — COMPILE DEPENDENCY BOUNDARY（编译依赖边界）

### FINDING-1 FINAL STATUS（永久保留）

```
36 tracked modifications alone
  → go vet FAIL

36 tracked + exactly 3 mandatory untracked
  → go vet PASS
```

**mandatory compile dependencies（3 个）**：

| ID | 文件 | Ownership（OD 裁定） |
|---|---|---|
| **F-01** | `internal/blockchain/identity.go` | **F-3B（SET A）** |
| **F-02** | `cmd/node/f3b_test_helpers_test.go` | **F-3B（SET A）** |
| **F-03** | `internal/blockchain/f1n1_reorg_coverage_test.go` | **F1-N1（非 SET A）** |

### 规则（进入 Scope Governance，永久有效）

> **compile dependency ≠ phase ownership**

- **禁止**因「该文件现在能让项目编译」而将其归入当前阶段。
- E3（symbol dependency）**只证明 dependency**，**永远不能单独证明 ownership**。
- F-03 **编译必需**，但其 ownership = **F1-N1**（OD-03），**不得**因编译依赖并入 F-1～F-5。
- 缺失符号（去重后 5 个）全部定义在 **untracked** 文件中：
  `ErrUninitializedStore` / `VerifyGenesisIdentity` / `NewBlockchainFromStoreForTest`（← F-01）、
  `f1n1UTXOSig`（← F-03）、`newNodeRuntimeForTest`（← F-02）。
- 失败**包含生产构建**（`# p2pchain/internal/blockchain`，无 `.test` 后缀）⇒ 非仅测试问题。

**证据实测（pristine HEAD 抽取，仓库零改动）**：

| 注入内容 | `go vet ./...` |
|---|---|
| 仅 36 个 tracked 修改 | **exit 1** |
| 36 + F-01 + F-02 + F-03 | **exit 0** ✅ |

---

## §13 — SECRET ARTIFACT BOUNDARY（凭据边界）

**边界结论**：

```
SECRET ARTIFACT COUNT = 3
ALL THREE:  tracked = NO   ignored = NO   ⇒  VISIBLE TO `git add`
```

| 文件 | 大小 | mtime | 用途 |
|---|---|---|---|
| `audit-run/control-token` | 33 B | 2026-09-27 08:51:13 | AUDIT-FIX-RUN-MINING-1 本地节点 mutation auth token |
| `f5-verify/control-token` | 64 B | 2026-09-27 12:59:02 | F-5 REAL NODE VALIDATION 节点 token |
| `gui-test/token` | 34 B | 2026-09-27 10:11:58 | GUI 全功能测试节点 token |

**永久规则（OD-08 冻结）**：

```
NEVER git add / commit / restore / clean
NEVER print token contents
NEVER migrate token into another artifact
```

- **提交必须使用显式路径白名单**，**禁止** `git add -A` / `git add .` / `git commit -a` / `git add --all`。
- `.gitignore` **本阶段未修改**（sha `f43418cf…`）；加固须**单独 Governance 阶段授权**
  （注：修改会使 tracked 修改数 36 → 37，破坏 F-5/F-6/F-6.1 基线）。
- 本阶段**未读取任何 token 实际内容**。

---

## §14 — FLAKY TEST BOUNDARY（flaky 测试边界）

| 项 | 值 |
|---|---|
| 用例 | `TestF1N1_C_MixedLegacyV2Reorg` |
| 位置 | `internal/blockchain/f1n1_reorg_coverage_test.go:560` |
| 状态 | **OPEN / FLAKY / LOAD-SENSITIVE** |
| 实测 | 隔离 `-run` 3/3 PASS；整包 2/2 PASS；**全套 `./...` 并行 1/1 FAIL** |
| 失败信息 | `F1N1: no sibling below threshold after 256 attempts (height=3)` |
| 本阶段动作 | **DO NOT FIX**（仅记录） |
| 边界 | 修复须**单独阶段授权**（候选：F-6.2 Test Stabilization） |

> 该用例位于 **F1-N1** 文件（OD-03）⇒ 其稳定性问题**不属** F-1～F-5。

---

## §15 — SHA-DRIFT BLOCKERS（SHA 漂移阻塞项）

### ★ GAP-4 artifact-vs-report SHA drift（未修，**提交前置条件**）

授权报告：`docs/PHASE-P2PCHAIN-GAP-4-COINBASE-ATTRIBUTION-IMPLEMENTATION-REPLAY-AUDIT-REPORT.md`
（**未跟踪**，mtime `2026-09-20 20:07`，sha `0bf64bf6cd755e471629580079b453da5dad3b00edf89de51e566bfd8bfd66ad`）

| 文件 | 报告登记 SHA（2026-09-20） | 当前工作树 SHA | 判定 |
|---|---|---|---|
| `internal/attribution/attribution.go` | `783aeaf569e308a858a40506db1ab19f34941922653d7745998c51b642388715` | `3f4cb77820b43747de5354b1f095336e175c1afa4157758223d834d5dac3dadb` | ❌ **MISMATCH** |
| `internal/attribution/classify.go` | `f64f07bb55485eee68e12410a6a49580d66e12d6835e905d2cb15cb8feaf338e` | `bb7b00bc135c6146c9ca110f93080b839bd963d99cd78d3eb578ec7a07865060` | ❌ **MISMATCH** |
| `internal/attribution/attribution_test.go` | `4c4d91fdd9274cb6fa40287101c54c681ab3a357720a7b2b6e40bfc743c8f43b` | `4d7841d418a668818b5c151245045d1a37029e2e77e12d32b779a5b7b77d8b7c` | ❌ **MISMATCH** |
| `cmd/coinbase-attribution/main.go` | `c0fe645c1962762b6c5672fa1b39acb3c6b2cacd92e826cb4ca2a913f95c703d` | `c634c1ecaea5955118533d035af2d0db2fa8f18365fa579a22c0b7401bfa6f18` | ❌ **MISMATCH** |

- **CRLF→LF 归一化不能消解差异**（文件为**纯 LF**）。
- 时间线：报告 `2026-09-20 20:07` → 产物 `2026-09-22 14:45`（**约 2 天后**）。
- ⇒ 报告所审计的产物状态**与当前工作树不一致**。
- **本阶段不得修复**（OD-04 / OD-05：`ACCEPTED WITH SHA-DRIFT BLOCKER`）。
- **提交资格 = BLOCKED**，直至单独完成 **SHA-DRIFT CLOSURE**。

---

## §16 — GOVERNANCE GAPS（治理缺口）

| ID | 缺口 | 证据 | 处置 |
|---|---|---|---|
| **GAP-G-1** | `CONSOLE-MINE-AUTH-FIX-1` **无阶段报告** | `grep -rl 'CONSOLE-MINE-AUTH-FIX-1' --include='*.md'` → 仅本链自身 F-6/F-6.1 报告 | **GOVERNANCE CLOSURE = REQUIRED** |
| **GAP-G-2** | `AUDIT-FIX-RUN-MINING-1` 报告在**仓库之外** | `E:/wakuang/PHASE-AUDIT-FIX-RUN-MINING-1-REPORT.md`（`5e5e8142…`） | 归档决策（**不得复制/移动/删除**） |
| **GAP-G-3** | F1-N1 / F1-N2 报告在**仓库之外** | `E:/wakuang/.workbuddy/F1-N1-…`（`42462d9b…`）/ `F1-N2-…`（`ddc8e4d7…`） | 归档决策（**不得复制进 repo**） |
| **GAP-G-4** | GAP-4 授权报告**本身未跟踪** + **SHA drift** | 见 §15 | SHA-DRIFT CLOSURE |
| **GAP-G-5** | `.gitignore` **无 secret/token 模式** | 3 token 未忽略 | GOVERNANCE hardening（可选，须单独授权） |
| **GAP-G-6** | `docs/PHASE-P3.1-…md` 工作树内容为 **orphan**（错配） | 与 5 份 BRAND-0D.3 归档均不匹配 | P3.1 RESTORATION（独立阶段） |

---

## §17 — AUTHORIZED NEXT PHASES（已授权下一阶段）

> **本阶段仅登记候选，不自动进入任何一项。**

| 候选阶段 | 触发条件 | 前置阻塞 |
|---|---|---|
| **P3.1 RESTORATION** | OD-01 决策（真实 provenance 已接受） | 内容错配；须独立恢复/保全阶段 |
| **GAP-4 — SHA-DRIFT CLOSURE** | OD-04 / OD-05 决策 | 4 文件 SHA 与报告全不符 |
| **F1-N1 / F1-N2 — 授权与归档** | OD-03 / OD-06 决策 | 报告在仓库之外 |
| **CONSOLE GOVERNANCE CLOSURE** | OD-07-A 决策 | 缺阶段报告 |
| **AUDIT-FIX-RUN-MINING-1 — 归档与提交** | OD-07-B 决策 | 报告在仓库之外 |
| **GOVERNANCE — `.gitignore` secret hardening** | OD-08 决策（可选） | 会改变基线（36→37） |
| **F-6.2 — Test Stabilization** | Owner 决定处理 flaky test | `TestF1N1_C_MixedLegacyV2Reorg` |
| **F-6.3 — Commit Readiness / Execution** | Owner 逐项解除 BLOCKED 后 | 需先完成上述前置 |

---

## §18 — HARD STOP

```
PHASE F-6.1-OWNER-DECISION-FINALIZATION COMPLETE
SCOPE FROZEN
HARD STOP
```

- **8 项 Owner Decision 已全部最终化**（OD-01 … OD-08，见 §3–§10）。
- **SET A / SET B / SET C 已正式冻结**；**FINAL SCOPE MATRIX 已生成**（§11）。
- **残余 UNKNOWN = 0**；主要治理状态转为 **BLOCKED**（含 SHA-DRIFT / GOVERNANCE / 归档类阻塞）。
- **未修改**任何源码 / 测试 / flaky test / 历史文档 / `.gitignore`。
- **未**恢复 P3.1；**未**删除 BRAND-0D.3 orphan；**未**修改 GAP-4 文件；**未**修复 SHA drift；
  **未**修改 Console 文件；**未**修复 flaky test；**未**读取 token 内容；**未**移动/复制任何项目文件。
- **未**执行任何 Git 写操作（add / commit / reset / restore / checkout / clean / stash / rebase / merge / push / tag）。

**禁止自动进入**：F-6.2 ｜ F-6.3 ｜ SHA-DRIFT CLOSURE ｜ P3.1 RESTORATION ｜
CONSOLE GOVERNANCE CLOSURE ｜ FLAKY TEST FIX ｜ COMMIT ｜ 任何源码修改。

**等待下一阶段 Owner 授权。**

**HARD STOP。**
