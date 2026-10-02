# PHASE F-6.6 — OWNER COMMIT BOUNDARY DECISION

| 字段 | 值 |
|---|---|
| Phase ID | **F-6.6** |
| Title | OWNER COMMIT BOUNDARY DECISION AUDIT |
| Mode | **STRICT READ-ONLY / DECISION RECORD** |
| Date (local) | 2026-09-28 08:38:36 +0800 |
| Baseline HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| Branch | `main` |
| Repository | `E:/wakuang/p2pchain` |
| Experiment root | `E:/wakuang/f66-verify`（**仓库之外**） |
| Write ops on repo | **NONE**（无 `git add` / `commit` / `push`；无代码/测试修改） |
| Verdict | **PHASE F-6.6 COMPLETE / COMMIT BOUNDARY FROZEN / HARD STOP** |

---

## §0 — PHASE ROLE & SCOPE

### 0.1 任务

基于 **F-6.1（8 项 Owner Decision）/ F-6.3（GAP-4 SHA-drift）/ F-6.5（测试稳定性）** 全部证据，
**冻结 Genesis Closure 最终提交边界**，并产出：
1. `OWNER-COMMIT-BOUNDARY-DECISION.md`（本文件）
2. `COMMIT-CANDIDATE-FINAL-MANIFEST.md`

### 0.2 必须重新验证的 6 项

| # | 项 | 章节 | 结果 |
|---|---|---|---|
| 1 | HEAD baseline | §2 | ✅ PASS |
| 2 | tracked / untracked inventory | §3 | ✅ 完成 |
| 3 | production dependency closure | §4 | ✅ **新结论：8 文件** |
| 4 | test dependency closure | §5 | ✅ **新结论：条件依赖** |
| 5 | ownership classification | §6 | ✅ 完成（UNKNOWN = 0） |
| 6 | secret exclusion | §7 | ✅ PASS |

### 0.3 只读纪律

| 禁令 | 状态 |
|---|---|
| 修改代码 | ✅ 未修改 |
| 修改测试 | ✅ 未修改 |
| `git add` | ✅ 未执行 |
| `commit` / `push` | ✅ 未执行 |
| `.gitignore` 修改 | ✅ 未修改（`f43418cf74ea996c`） |
| token 内容读取 | ✅ 未读取（仅 `stat` 大小） |

---

## §1 — AUTHORITATIVE INPUTS

| # | 文档 | SHA-256（前 16） | 用途 |
|---|---|---|---|
| 1 | `PHASE-F6.1-OWNER-DECISION-FINALIZATION.md` | `5b95d47009dd49c8` | OD-01…OD-08（8 项冻结决策） |
| 2 | `PHASE-F6.1-FILE-PROVENANCE-MATRIX.md` | `86d20f62e2446811` | 逐文件 provenance（E1–E4） |
| 3 | `PHASE-F6.1-SECRET-ARTIFACT-INVENTORY.md` | `a4a36f7095a5fabc` | 秘密/产物清单 |
| 4 | `PHASE-F6.3-GAP4-SHA-DRIFT-FORENSIC-REPORT.md` | `34577f5d732a4c66` | GAP-4 根因（gofmt 事故） |
| 5 | `PHASE-F6.4-GENESIS-CLOSURE-COMMIT-BOUNDARY-AUDIT.md` | `bcbe5ddfe760536f` | 边界定义 + OPT-A/B/C/D |
| 6 | `PHASE-F6.4-GENESIS-COMMIT-CANDIDATE-MANIFEST.md` | `9bbfb663f174e5d1` | GCS(33) 逐文件表 |
| 7 | `PHASE-F6.5-GENESIS-TEST-STABILITY-FORENSIC-AUDIT.md` | `86b6ef8cd4610cd0` | TIME-LOAD FRAGILITY |
| 8 | `PHASE-F6.5-TEST-STABILITY-MATRIX.json` | `c6a3ca925a8f0973` | 机器可读测量 |
| 9 | `PHASE-F6.2-P3.1-PROVENANCE-RESTORATION-REPORT.md` | `7db1e23894e8fd0d` | P3.1 provenance（OD-01 已闭合） |

> **输入缺口（沿用 F-6.5 ANOM-1）**：`PHASE-F6.2-SCOPE-FREEZE-MATRIX.md` /
> `PHASE-F6.2-OWNER-DECISION-RECORD.md` **不存在**，未伪造替代。

---

## §2 — BASELINE RE-VERIFICATION

| 项 | 实测 | 与 F-6.5 基线 | 状态 |
|---|---|---|---|
| HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` | 一致 | ✅ |
| branch | `main` | 一致 | ✅ |
| staged | **0** | 一致 | ✅ |
| tracked modified | **35** | 一致 | ✅ |
| porcelain `M` | **36** | 一致 | ✅ |
| untracked（文件级） | **375** | 373 → **375**（+2 = F-6.5 交付物） | ✅ 可解释 |
| untracked（porcelain `??` 行） | **202** | 200 → **202**（+2） | ✅ 可解释 |

### 2.1 受保护对象

| 对象 | 实测 SHA-256（前 16） | 状态 |
|---|---|---|
| `.gitignore` | `f43418cf74ea996c` | ✅ 不变 |
| `node.exe` | `3972843aff90428d` | ✅ 不变 |
| `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` | `e9def9aa1f90e5ac` | ✅ 不变 |
| `internal/attribution/attribution.go` | `3f4cb77820b43747` | ✅ 不变 |
| `internal/attribution/classify.go` | `bb7b00bc135c6146` | ✅ 不变 |
| `internal/attribution/attribution_test.go` | `4d7841d418a66881` | ✅ 不变 |
| `cmd/coinbase-attribution/main.go` | `c634c1ecaea59551` | ✅ 不变 |
| tokens（大小） | 33 / 64 / 34 B | ✅ 不变 |

```
BASELINE RE-VERIFICATION: PASS
```

---

## §3 — TRACKED / UNTRACKED INVENTORY

### 3.1 Tracked modified = **35**（全部为真实内容变更）

按工作流归类：

| 工作流 | 文件数 | 文件 |
|---|---|---|
| **F-3B 生产（组 A）** | **7** | `cmd/node/main.go`、`cmd/node/cli.go`、`internal/blockchain/blockchain.go`、`internal/blockchain/verify.go`、`internal/blockchain/genesis.go`、`internal/storage/file.go`、`internal/storage/v2.go` |
| **F-3B 夹具（组 B）** | **22** | 见 §5.2 表 |
| **SET B — Console** | **3** | `internal/control/server.go`、`internal/control/console_test.go`、`internal/control/web/console.html` |
| **SET B — Explorer** | **1** | `internal/explorer/explorer_test.go` |
| **SET B — R1** | **1** | `cmd/node/r1_boundary_test.go` |
| **D 类文档** | **1** | `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` |
| 合计 | **35** | — |

> **CRLF 伪差异**：`internal/blockchain/query.go` 出现在 porcelain `M`（36）但**不在** `git diff --name-only`（35）
> ⇒ 36 − 35 = 1 个**纯 CRLF 伪修改**。**不构成内容变更**，**不进入提交边界**。

### 3.2 Untracked `*.go` = **13**（全部编译相关候选）

| # | 文件 | 归属 | 备注 |
|---|---|---|---|
| 1 | `internal/blockchain/identity.go` | **F-3B (F-01)** | **生产必需** |
| 2 | `cmd/node/f3b_test_helpers_test.go` | **F-3B (F-02)** | **条件必需**（§5.3） |
| 3 | `cmd/node/f3b_identity_test.go` | **F-3B (F-03)** | 编译可选 / Genesis 测试闭合必需 |
| 4 | `internal/blockchain/f1n1_reorg_coverage_test.go` | **F1-N1 (X-01)** | **编译强制**（§4.3） |
| 5 | `internal/blockchain/f1n2_reorg_detached_undo_test.go` | F1-N2 | 边界外 |
| 6 | `internal/storage/f1n1_undo_contract_test.go` | F1-N1 | 边界外 |
| 7 | `internal/storage/f1n2_detached_undo_contract_test.go` | F1-N2 | 边界外 |
| 8 | `internal/utxo/f1n1_undo_content_test.go` | F1-N1 | 边界外 |
| 9 | `internal/attribution/attribution.go` | **GAP-4** | 边界外 |
| 10 | `internal/attribution/classify.go` | **GAP-4** | 边界外 |
| 11 | `internal/attribution/attribution_test.go` | **GAP-4** | 边界外 |
| 12 | `cmd/coinbase-attribution/main.go` | **GAP-4** | 边界外 |
| 13 | `internal/control/console_mine_auth_test.go` | **SET B (C6)** | 边界外 |

### 3.3 Untracked 非 Go（边界外）

`docs/`(148) · `.workbuddy/`(107) · `gui/`(34) · `F-1-lab/`(16) · `audit-run/`(8) · `verifier/`(6) ·
`concept-mvp/`(4) · `f5-verify/`(3) · `gui-test/`(1) · 仓库根 35 份 `PHASE-*` 报告。
⇒ **全部排除**（§10）。

---

## §4 — PRODUCTION DEPENDENCY CLOSURE ★（F-6.6 新结论）

### 4.1 实验（仓库外 `git archive HEAD` 树）

| 树 | 组成 | `go build ./...` | 判定 |
|---|---|---|---|
| **P0** | `base` + **组 A(7)**（**无** `identity.go`） | **exit 1** | ❌ |
| **P1** | P0 + `internal/blockchain/identity.go` | **exit 0** | ✅ |

**P0 失败详情（★ 生产包，无 `.test` 后缀）**：

```
# p2pchain/internal/blockchain
internal\blockchain\blockchain.go:224:15: undefined: ErrUninitializedStore
internal\blockchain\blockchain.go:227:16: undefined: VerifyGenesisIdentity
internal\blockchain\verify.go:57:16:     undefined: VerifyGenesisIdentity
```

### 4.2 生产编译闭包（PCC）★

```
PRODUCTION COMPILE CLOSURE (PCC) = 组 A(7) + identity.go(1) = 8 文件
```

| # | 文件 | Git | 角色 |
|---|---|---|---|
| 1 | `cmd/node/main.go` | M | 启动门（identity gate 调用方） |
| 2 | `cmd/node/cli.go` | M | `init` / `verify` 子命令 |
| 3 | `internal/blockchain/blockchain.go` | M | `ErrUninitializedStore` 使用点（:224） |
| 4 | `internal/blockchain/verify.go` | M | `VerifyGenesisIdentity` 使用点（:57） |
| 5 | `internal/blockchain/genesis.go` | M | 创世定义 |
| 6 | `internal/storage/file.go` | M | 存储层 |
| 7 | `internal/storage/v2.go` | M | v2 存储 |
| 8 | `internal/blockchain/identity.go` | **??** | **定义** `ErrUninitializedStore` / `VerifyGenesisIdentity` / `InitializeBlockchainStore` |

> **⇒ Genesis 的「生产真相」只有 8 个文件。** 其余 25 个 GCS 文件**全部与生产编译无关**。

### 4.3 生产闭包 ≠ 测试闭包

P1 的 `go vet ./...` 仍 **exit 1**：

```
# p2pchain/internal/blockchain [p2pchain/internal/blockchain.test]
internal\blockchain\p1_reorg_fail_before_commit_test.go:63:19: undefined: f1n1UTXOSig
```

⇒ **tracked、未修改**的 `p1_reorg_fail_before_commit_test.go:63` 强制要求 `f1n1UTXOSig`
（唯一定义在 untracked 的 **X-01**）⇒ **测试编译强制引入 F1-N1 文件**（BLK-1）。

---

## §5 — TEST DEPENDENCY CLOSURE ★（F-6.6 新结论：条件依赖）

### 5.1 增量实验（最小编译闭包）

| 树 | 组成 | `go vet ./...` |
|---|---|---|
| **C1** | base + A(7) + `identity.go` | **exit 1**（`undefined: f1n1UTXOSig`） |
| **C2** | C1 + **X-01** | **exit 0** ✅ |
| C3 | C2 + `f3b_test_helpers_test.go` | exit 0 |
| C4 | C3 + `f3b_identity_test.go` | exit 0 |

```
绝对最小测试编译闭包 (TCC-abs) = A(7) + identity.go + X-01 = 9 文件
```

### 5.2 条件依赖 ★★（F-6.6 关键发现）

`f3b_test_helpers_test.go`（定义 `newNodeRuntimeForTest`）**是否必需取决于组 B 的形态**：

| 组 B 版本 | 引用 `newNodeRuntimeForTest`？ | `f3b_test_helpers_test.go` |
|---|---|---|
| **HEAD 版** | **0 处**（调用 `newNodeRuntime(...)` 直连） | **不需要**（C2 实测 exit 0） |
| **工作树版** | **11 处**（6 个文件） | **必需**（M3 实测 exit 1） |

实测（HEAD vs 工作树，`newNodeRuntimeForTest` 引用计数）：

| 文件 | HEAD | 工作树 |
|---|---|---|
| `cmd/node/datalock_integration_test.go` | 0 | 2 |
| `cmd/node/fullstack_test.go` | 0 | 2 |
| `cmd/node/lock_lifecycle_test.go` | 0 | 4 |
| `cmd/node/seed_parallel_test.go` | 0 | 2 |
| `cmd/node/p2p_branch_test.go` | 0 | 1 |

**⇒ 依赖是条件性的**：

```
若组 B 以工作树形态提交 ⇒ TCC-gcs = A(7) + B(22) + identity.go + f3b_test_helpers_test.go + X-01 = 32 文件
若组 B 保持 HEAD 形态   ⇒ TCC-abs = 9 文件（但 Genesis 夹具语义缺失 ⇒ 不可接受）
```

### 5.3 M 系列复验（`go vet`，仓库外）

| # | 从 GCS 移除 | `go vet` | 首行证据 | 判定 |
|---|---|---|---|---|
| **M1** | `f1n1_reorg_coverage_test.go`（X-01） | **exit 1** | `[.test]` 包：`p1_reorg_fail_before_commit_test.go:63:19: undefined: f1n1UTXOSig` | **REQUIRED** |
| **M2** | `internal/blockchain/identity.go` | **exit 1** | **生产包**（无 `.test`）：`blockchain.go:224/227`、`verify.go:57` | **REQUIRED（生产）** |
| **M3** | `cmd/node/f3b_test_helpers_test.go` | **exit 1** | `datalock_integration_test.go:39:12: undefined: newNodeRuntimeForTest` | **REQUIRED（条件）** |
| **M4** | `cmd/node/f3b_identity_test.go` | **exit 0** | — | OPTIONAL（编译） |

### 5.4 GCS(33) 编译闭合复验

| 命令 | 结果 |
|---|---|
| `go build ./...` | **exit 0** ✅ |
| `go vet ./...` | **exit 0** ✅ |

### 5.5 绿色闭包（沿用 F-6.5）

```
GREEN CLOSURE = GCS(33) + SET B(6) = 39 文件
但 F-6.5 已证：39 文件**仍不能稳定变绿**（cmd/node 隔离余量仅 +2.228s / 0.37%）。
```

---

## §6 — OWNERSHIP CLASSIFICATION

### 6.1 四证据复验

**E1 — mtime 聚类**

| 簇 | 时间 | 文件 |
|---|---|---|
| **F1-N1/N2 残余** | 2026-09-20 07:12–07:55 | `utxo/f1n1_undo_content_test.go`、`storage/f1n2_detached_undo_contract_test.go`、`blockchain/f1n2_reorg_detached_undo_test.go` |
| **GAP-4（gofmt 事故）** | 2026-09-22 14:45:36–38 | 4 GAP-4 + `storage/f1n1_undo_contract_test.go` |
| **SET B C1/C2** | 2026-09-27 08:50–08:54 | `explorer_test.go`、`r1_boundary_test.go` |
| **SET B Console** | 2026-09-27 09:19–09:20 | `console.html`、`console_test.go`、`console_mine_auth_test.go`、`server.go` |
| **F-3B Genesis** | 2026-09-27 10:48–10:59 | A(7) + `identity.go` + B(22) + F-02/F-03 |
| 后段微调 | 2026-09-27 12:25–12:36 | `stop_lifecycle_test.go`、`lock_lifecycle_test.go`、`genesis.go` |

**E2 — 文件头 phase 自述**

| 文件 | 头部自述 |
|---|---|
| `internal/blockchain/f1n1_reorg_coverage_test.go` | `// PHASE P2PCHAIN — F1-N1` |
| `internal/blockchain/f1n2_reorg_detached_undo_test.go` | `// PHASE P2PCHAIN — F1-N2` |
| `internal/storage/f1n1_undo_contract_test.go` | `// PHASE P2PCHAIN — F1-N1` |
| `internal/storage/f1n2_detached_undo_contract_test.go` | `// PHASE P2PCHAIN — F1-N2` |
| `internal/utxo/f1n1_undo_content_test.go` | `// PHASE P2PCHAIN — F1-N1` |
| `cmd/node/r1_boundary_test.go` | `// 本文件是 PHASE P2PCHAIN — ORPHAN RESOURCE-BOUNDARY REMEDIATION R-1 的回归测试` |
| `internal/control/server.go` | `自 PHASE CONTROL-AUTH-1 起…` |
| `internal/control/console_mine_auth_test.go` | `// PHASE CONSOLE-MINE-AUTH-FIX-1：…` |

**E3 — 符号依赖**（**只证 dependency，永不能单独证 ownership** — 长期规则）

**E4 — 授权痕迹**：OD-03（X-01 → F1-N1）/ OD-04+OD-05（GAP-4）/ OD-07-A（Console）/ OD-07-B（AUDIT-FIX）/ OD-08（token）

### 6.2 三集合判定

| 集合 | 内容 | 规模 | UNKNOWN |
|---|---|---|---|
| **SET A**（F-1～F-5 Protocol Closure） | GCS(33) = A(7)+B(22)+F(3)+X-01(1) | 33 | — |
| **SET B**（Other Authorized Workstreams） | C1–C6 | 6 | — |
| **SET C**（Unknown） | — | **0** | ✅ 四证据齐备 |

```
OWNERSHIP CLASSIFICATION: COMPLETE — SET C (UNKNOWN) = 0
```

---

## §7 — SECRET EXCLUSION

### 7.1 `.gitignore` 实际覆盖

```
bin/  dist/  *.exe  *.test  *.out  coverage.out  .env  .env.*  .idea/  .vscode/  .DS_Store
/run-a/  /run-b/  *.dat  *.lock  wallet.json
```

**★ 无 token / secret / *.key 模式**（`grep -i "token|secret|\.key|credential"` 仅命中注释与 `.env`）。

### 7.2 三个未忽略 token（OD-08）

| 文件 | tracked | ignored | size | 历史提交数 |
|---|---|---|---|---|
| `audit-run/control-token` | **NO** | **NO** | 33 B | **0** |
| `f5-verify/control-token` | **NO** | **NO** | 64 B | **0** |
| `gui-test/token` | **NO** | **NO** | 34 B | **0** |

**⇒ 排除依据是 OD-08 的路径白名单策略，而非 `.gitignore`。**
⇒ **永久禁令**：禁止 `git add -A` / `git add .` / `git add -a` / `git commit -a`；
**必须显式路径白名单**。

### 7.3 其他秘密面

| 项 | 状态 |
|---|---|
| `run-a/wallet.json` / `run-b/wallet.json`（389 B，QUARANTINED） | ✅ 被 `wallet.json` 模式忽略 |
| 候选边界内高危模式扫描（`BEGIN * PRIVATE KEY` / `password=` / `api_key=`） | ✅ **0 命中** |
| token 出现在 index（staged） | ✅ **0** |
| token 出现在任何 commit | ✅ **0** |

```
SECRET EXCLUSION: VERIFIED — 0 secret in boundary, 0 token in git.
```

---

## §8 — OPTION RE-EVALUATION（结合 F-6.5 证据）

F-6.4 提出 OPT-A/B/C/D；F-6.5 补充了「**不稳定性的量化**」。逐项重估：

| 选项 | 内容 | F-6.5 后的重估 | 可采纳？ |
|---|---|---|---|
| **OPT-A** | 混合提交 MGS(39) | **被 F-6.1 §4.4 明令禁止**（「**不得**因编译依赖并入 F-1～F-5」）；且 F-6.5 证 39 文件**仍不稳定**（0.37% 余量）⇒ 混合**换不来绿色** | ❌ **否决** |
| **OPT-B** | 先授权 F1-N1 独立提交，再提交 Genesis | **唯一同时满足「scope 纯」+「编译闭合」且**无需改代码**的选项**；F-6.5 未产生反对证据 | ✅ **采纳** |
| **OPT-C** | 重构：把 `f1n1UTXOSig` 抽为 Genesis-owned 夹具 | 需**写入阶段**授权；且仍须改 X-01（消除重复定义）⇒ **仍触 F1-N1 治理**，未减少依赖 | ⏸ 记为**长期改进项** |
| **OPT-D** | 先修 flaky + 解决 cmd/node 超时 | 针对**稳定性**（F-6.5 已证为**非 Genesis** 问题），**不解决 scope 冲突** | ⏸ 记为**独立后续阶段** |

### 8.1 决定性判据

1. **F-6.1 §4.4 永久规则**：「F-03 编译必需，但其 ownership = F1-N1（OD-03），**不得**因编译依赖并入 F-1～F-5。」
   ⇒ **OPT-A 被永久规则否决**。
2. **F-6.6 §4.2 新结论**：Genesis 生产闭包仅 **8 文件** ⇒ Genesis 的真实变更与 25 个夹具文件可**解耦**。
3. **F-6.6 §5.2 新结论**：`f3b_test_helpers_test.go` 为**条件依赖** ⇒ 组 B 的提交形态决定闭包形状。
4. **F-6.5 §16**：不稳定性 **100% 非 Genesis** ⇒ **「全套绿色」不应作为 Genesis 提交的前置条件**。

---

## §9 — ★ FROZEN DECISION（冻结决定）

### 9.1 决定主体

```
╔══════════════════════════════════════════════════════════════════════════════╗
║  DECISION: OPT-B  —  F1-N1 FIRST, THEN GENESIS CLOSURE                       ║
║  FINAL GENESIS CLOSURE COMMIT BOUNDARY = 33 文件 (= GCS)                     ║
║  SEQUENCED AS TWO COMMITS: CB-1 (F1-N1, 1 文件) → CB-2 (Genesis, 32 文件)     ║
╚══════════════════════════════════════════════════════════════════════════════╝
```

### 9.2 边界构成

| 提交 | 内容 | 文件数 | 前置条件 |
|---|---|---|---|
| **CB-1** | **F1-N1**：`internal/blockchain/f1n1_reorg_coverage_test.go`（X-01） | **1** | **OD-03 解阻**（Owner 将 F1-N1 授权为独立提交阶段） |
| **CB-2** | **Genesis Closure**：组 A(7) + 组 B(22) + 组 F(3) | **32** | CB-1 完成（编译闭合需要 `f1n1UTXOSig`） |
| **合计** | **Genesis Closure Commit Boundary** | **33** | — |

### 9.3 决定条款（D-1 … D-8）

| ID | 条款 |
|---|---|
| **D-1** | 最终 Genesis Closure 提交边界 = **33 文件（GCS）**，拆为 **CB-1(F1-N1, 1) + CB-2(Genesis, 32)**。 |
| **D-2** | **CB-1 必须先于 CB-2**：`p1_reorg_fail_before_commit_test.go:63`（tracked、未修改）编译强制要求 `f1n1UTXOSig`。 |
| **D-3** | **SET B(6) 排除**于本边界，须作为**独立工作流提交**（C1 属 R1、C2 属 AUDIT-FIX、C3–C6 属 CONSOLE）。 |
| **D-4** | **GAP-4(4) 排除**；其 `ACCEPTED WITH SHA-DRIFT BLOCKER`（OD-04/OD-05）**仍然有效**。 |
| **D-5** | **F1-N1/N2 残余(4) 排除**（`blockchain/f1n2_*`、`storage/f1n1_*`、`storage/f1n2_*`、`utxo/f1n1_*`）。 |
| **D-6** | **「全套绿色」不作为 CB-2 的前置条件**；CB-2 的验收判据 = **L1 编译闭合 + L2 Genesis-scoped 确定性测试**（F-6.5 证据：不稳定性 100% 非 Genesis）。 |
| **D-7** | `.gitignore` token 加固**延后**（会改变基线 35→36）⇒ 须独立 Governance 授权。 |
| **D-8** | **本阶段不执行任何提交**；`git add` / `commit` / `push` 须由**独立 COMMIT 阶段**按显式路径白名单执行。 |

### 9.4 边界形状图

```
pristine HEAD (4d892be)
        │
        ├── CB-1  F1-N1  ──────────────► 1 文件  (X-01)
        │
        └── CB-2  Genesis Closure ─────► 32 文件
                  ├─ A  F-3B 生产      7
                  ├─ B  F-3B 夹具     22
                  └─ F  F-3B 新源      3   (identity.go, f3b_test_helpers_test.go, f3b_identity_test.go)
                  ────────────────────────────
                  TOTAL (CB-1 ∪ CB-2)  33  =  GCS

EXCLUDED:  SET B(6) · GAP-4(4) · F1-N1/N2 残余(4) · docs · secrets · 实验产物
```

---

## §10 — EXCLUSION LEDGER

| 类别 | 文件 | 数量 | 理由 |
|---|---|---|---|
| **SET B** | `cmd/node/r1_boundary_test.go`、`internal/explorer/explorer_test.go`、`internal/control/server.go`、`internal/control/console_test.go`、`internal/control/web/console.html`、`internal/control/console_mine_auth_test.go` | 6 | 独立工作流（R1 / AUDIT-FIX / CONSOLE）；**非 F-1～F-5** |
| **GAP-4** | `internal/attribution/{attribution,classify,attribution_test}.go`、`cmd/coinbase-attribution/main.go` | 4 | OD-04/OD-05 SHA-DRIFT BLOCKER 未闭合 |
| **F1-N1/N2 残余** | `internal/blockchain/f1n2_reorg_detached_undo_test.go`、`internal/storage/f1n1_undo_contract_test.go`、`internal/storage/f1n2_detached_undo_contract_test.go`、`internal/utxo/f1n1_undo_content_test.go` | 4 | OD-03/OD-06：报告在仓库之外，须独立归档 |
| **D 类文档** | `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` | 1 | OD-02：须经独立验证者（PHASE V2.0） |
| **CRLF 伪差异** | `internal/blockchain/query.go` | 1 | 无内容变更 |
| **Secrets** | `audit-run/control-token`、`f5-verify/control-token`、`gui-test/token` | 3 | **OD-08 永久排除** |
| **实验产物** | `audit-run/`、`f5-verify/`、`gui-test/`、`F-1-lab/`、`concept-mvp/`、`verifier/` | — | 非源 |
| **工具/工作区** | `.workbuddy/`、`gui/` | — | 非源 |
| **历史报告** | 仓库根 35 份 `PHASE-*`（含 F-1…F-6.5） | 35 | 须走独立文档提交决策 |
| **`docs/` 其余** | 147 份 | 147 | 同上 |

---

## §11 — RESIDUAL BLOCKERS（本决定**未**解决者）

| ID | 阻塞 | 状态 | 需要的动作 |
|---|---|---|---|
| **BLK-1** | X-01 编译强制但 Commit Eligibility = BLOCKED（OD-03） | **本决定以 D-1/D-2 解除**（条件：Owner 授权 F1-N1 独立提交） | **Owner 批准 OD-03 解阻** |
| **BLK-2** | 绿色必需 SET B（C2） | **本决定以 D-3/D-6 隔离**（不并入 Genesis） | Owner 决定 SET B 独立提交时序 |
| **BLK-3** | `TestF1N1_C_MixedLegacyV2Reorg` 时序敏感 flaky（F-6.5 根因：`time.Now()` 种子 + 有界重试，P≈1/257） | **未解决**（随 X-01 进入 CB-1） | flaky stabilization 阶段（OPT-D） |
| **BLK-4** | `cmd/node` 隔离余量仅 **+2.228s（0.37%）**（F-6.5 三测 586.7/590.4/597.8s） | **未解决** | Test Stabilization（提速或超时策略） |
| **BLK-5** | 3 个未忽略 token + `.gitignore` 无 token 模式 | **由 D-7 + OD-08 白名单策略管控** | 可选 Governance 加固 |
| **BLK-6** | 权威输入 4/5 缺失（F-6.2 未产出） | **登记** | Owner 决定是否补产 |

> **★ 本决定只冻结「提交边界与顺序」，不声称边界已可稳定变绿。**
> BLK-3 / BLK-4 属**测试基建**，须由独立阶段处理（D-6 已将其与提交边界解耦）。

---

## §12 — GATE MODEL（验收闸门模型，替代「全套绿色」）

| 闸门 | 判据 | 本边界状态 |
|---|---|---|
| **G-L1 编译闭合** | `go build ./...` = 0 且 `go vet ./...` = 0 | ✅ **满足**（§5.4） |
| **G-L2 Genesis 确定性测试** | `go test -count=1 ./internal/blockchain/...` = 0 | ✅ **满足**（F-6.5：78.857s ok） |
| **G-L2b Genesis 自有测试** | `TestF3BExplicitInitializationStateMatrix` 确定性 PASS | ✅ **满足**（F-6.5：6/6 PASS，S0） |
| **G-L3 全套绿色** | `go test -count=1 ./...` = 0 | ❌ **不满足**（**非 Genesis**：F-6.5 §16） |

```
GATE MODEL (F-6.6):
  CB-2 验收 = G-L1 ∧ G-L2 ∧ G-L2b          → SATISFIED
  G-L3 = 独立跟踪项，不阻断 CB-2（D-6）
```

---

## §13 — ANOMALY REGISTRY（仅登记，未修复）

| ID | 异常 | 处置 |
|---|---|---|
| **F6.6-ANOM-1** | `internal/blockchain/f1n1_reorg_coverage_test.go` 的 **mtime 被重盖**：F-6.3 记录 `2026-09-22 14:45:37`，实测 `2026-09-27 10:52:35`；**但 SHA-256 = `c4606f09829ccc02…` 与 F-6.4 登记逐值一致** ⇒ **内容零漂移**，仅 mtime 变更 | **登记**；因 E1 仅为佐证证据（F-6.3 §242），**不影响 ownership 判定**；建议 Owner 知悉 |
| **F6.6-ANOM-2** | 权威输入 4/5 缺失（沿用 F-6.5 ANOM-1） | **登记**；未伪造 |
| **F6.6-ANOM-3** | F-6.4 单次 571.249s vs F-6.5 三次 586.7–597.8s（沿用 F-6.5 ANOM-2） | **登记**；余量已下修为 +2.2～13.3s |

> 按 Owner 流程：**发现异常一律 STOP，不顺手修**。以上三项**均只登记**。

---

## §14 — ZERO-DRIFT PROOF

| 对象 | 开工 | 收工 | 漂移 |
|---|---|---|---|
| HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` | 同 | **0** |
| staged | 0 | 0 | **0** |
| tracked modified | 35 | 35 | **0** |
| porcelain `M` | 36 | 36 | **0** |
| `.gitignore` | `f43418cf74ea996c` | 同 | **0** |
| `node.exe` | `3972843aff90428d` | 同 | **0** |
| P3.1 doc | `e9def9aa1f90e5ac` | 同 | **0** |
| GAP-4 ×4 | `3f4cb778…`/`bb7b00bc…`/`4d7841d4…`/`c634c1ec…` | 同 | **0** |
| tokens（大小） | 33/64/34 B | 同 | **0** |
| `run-a/` `run-b/` | 只读 | 只读 | **0** |

| 写操作 | 数量 |
|---|---|
| 仓库内文件写入（源代码/测试） | **0** |
| `git add` / `commit` / `push` | **0** |
| 实验产物位置 | `E:/wakuang/f66-verify`（仓库之外） |

**本阶段授权交付物**（写入仓库根）：
`OWNER-COMMIT-BOUNDARY-DECISION.md`、`COMMIT-CANDIDATE-FINAL-MANIFEST.md`。

```
ZERO-DRIFT: PROVEN
```

---

## §15 — FINAL GATE

```
╔══════════════════════════════════════════════════════════════════════════════╗
║  PHASE F-6.6 COMPLETE / COMMIT BOUNDARY FROZEN / HARD STOP                   ║
╚══════════════════════════════════════════════════════════════════════════════╝
```

| 项 | 值 |
|---|---|
| Status | **COMPLETE** |
| Decision | **OPT-B — F1-N1 FIRST, THEN GENESIS CLOSURE** |
| Final boundary | **33 文件** = CB-1(1) + CB-2(32) |
| Production closure (new) | **8 文件** |
| Compile closure | **33 文件**（条件：组 B 工作树形态） |
| Gate model | **L1 ∧ L2 ∧ L2b**（**不含** L3 全套绿色） |
| Residual blockers | BLK-3 / BLK-4（测试基建，**非**边界阻断）；BLK-6（输入缺口） |
| 下一阶段 | **不自动推断**；提交须独立 COMMIT 阶段授权 |

### HARD STOP 声明

> 本阶段**到此为止**。
> **未**修改任何代码或测试；**未**执行 `git add` / `commit` / `push`；
> **未**实施 OPT-A/B/C/D 中的任何一项（本阶段仅**记录决定**）。
> 后续动作（F1-N1 授权、COMMIT 执行、Test Stabilization、GAP-4 closure、文档提交决策）
> 须由 **Owner 单独授权**。

---

*PHASE F-6.6 — OWNER COMMIT BOUNDARY DECISION — 本报告为自引用文档，
SHA-256 于收工后由独立命令计算并登记于文件之外（memory / 交付说明）。*
