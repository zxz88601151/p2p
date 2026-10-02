# PHASE F-6.4 — GENESIS COMMIT CANDIDATE MANIFEST

**PHASE F-6.4 — GENESIS CLOSURE COMMIT BOUNDARY AUDIT**（本文件为 **重做版**）
**Deliverable 2 of 2 ｜ STRICT READ-ONLY ｜ NO GIT MUTATION**

| 项 | 值 |
|---|---|
| 仓库 | `E:/wakuang/p2pchain` |
| 审计基线 HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4`（branch `main`） |
| staged | **0** |
| tracked 内容修改 | **35** |
| untracked（`??`） | **200**（= 198 基线 + 2 份本阶段交付物） |
| 本清单性质 | **只读分析**；**不执行**任何 `git add` / `commit` / `push` |

> 本清单**不是**一条可执行的提交命令。它是「若要提交，必须包含哪些文件、可省略哪些、
> 必须排除哪些、哪些被阻塞」的**逐文件登记**。**最终提交集合须由 Owner 单独授权。**

---

## §1 — 状态口径（STATUS TAXONOMY）

| 状态 | 定义 | 判据 |
|---|---|---|
| **REQUIRED** | 缺失即破坏「可编译 **或** 绿色」 | 实测移除后 `go vet` ≠ 0 **或** `go test` ≠ 0 |
| **OPTIONAL** | 对**编译**可省略（回退 HEAD 版本仍可编译），但省略会**削弱闭合**（语义/测试覆盖） | 实测移除后 `go vet` = 0，但语义已变 |
| **EXCLUDED** | 不属于 Genesis 边界；纳入与否**均安全**，但按治理**必须排除** | 实测加/减 `go vet` 均 = 0；且归属非 F-1～F-5 |
| **BLOCKED** | 边界**必需**，但**当前不可提交**（治理/授权/基建阻塞） | 需 Owner 决策 |

**层级标记**：
`[C]` = 编译必需（compile closure）｜`[G]` = 绿色必需（green suite）｜`[T]` = 测试闭合必需（语义）

---

## §2 — GENESIS CANDIDATE SET（GCS = 33 文件）逐文件表

### 2.1 组 A —— F-3B 生产代码（7，tracked 修改）

| # | 路径 | sha256（前 16） | 状态 | 层级 | 依据 |
|---|---|---|---|---|---|
| A1 | `cmd/node/main.go` | `f22436ba406feb3a` | **REQUIRED** | `[G][T]` | 四态判定：0 字节→CORRUPTED；缺文件→Uninitialized |
| A2 | `cmd/node/cli.go` | `7d9b1c67d4d0012f` | **REQUIRED** | `[G][T]` | `init` 子命令 + identity gate 消费方 |
| A3 | `internal/blockchain/blockchain.go` | `8cf75cb11c5a9e02` | **REQUIRED** | `[C][G][T]` | 移除自动创世；**生产构建**消费方 |
| A4 | `internal/blockchain/verify.go` | `05db33fa2b276a9c` | **REQUIRED** | `[C][G][T]` | `VerifyGenesisIdentity` 消费方（**生产构建**） |
| A5 | `internal/blockchain/genesis.go` | `b6a2c2235a70d4fd` | **REQUIRED** | `[G][T]` | `CanonicalGenesisHash` pin + `Hex()` |
| A6 | `internal/storage/file.go` | `1385fc7433470cd8` | **REQUIRED** | `[G][T]` | `OpenFileBlockStoreStrict`（`repairOnOpen=false`） |
| A7 | `internal/storage/v2.go` | `95e7d9edda30fe33` | **REQUIRED** | `[G][T]` | `loadLog` torn-tail fail-closed |

### 2.2 组 B —— F-3B 测试夹具（22，tracked 修改）

> **共同性质**：`newNodeRuntime` / `NewBlockchainFromStore` **在修改后仍存在**
> （`cmd/node/main.go:74` / `internal/blockchain/blockchain.go:218`），故 HEAD 版本**仍可编译**；
> 但 F-3B 已改变其**语义**（不再自动建创世）⇒ HEAD 版测试**运行期失败**。
> ⇒ **OPTIONAL（编译）**，**REQUIRED（测试闭合 / 绿色）**。

| # | 路径 | sha256（前 16） | 状态 | 层级 | 依据 |
|---|---|---|---|---|---|
| B1 | `cmd/node/datalock_integration_test.go` | `48b72ac80eab229a` | **REQUIRED** | `[G][T]` | `newNodeRuntimeForTest` 消费方 |
| B2 | `cmd/node/fullstack_test.go` | `bc14215436319e48` | **REQUIRED** | `[G][T]` | 同上 |
| B3 | `cmd/node/lock_lifecycle_test.go` | `fbb6e8b07f975ed8` | **REQUIRED** | `[G][T]` | 同上 + `ensureTestDataDir` 接入 |
| B4 | `cmd/node/p2p_branch_test.go` | `21a255f471ce104f` | **REQUIRED** | `[G][T]` | 同上 |
| B5 | `cmd/node/r3_crash_restart_test.go` | `4a25cb3aa8769a2c` | **REQUIRED** | `[G][T]` | gofmt 对齐 |
| B6 | `cmd/node/r4b_dual_reorg_convergence_test.go` | `41879fcd121d851f` | **REQUIRED** | `[G][T]` | gofmt 对齐 |
| B7 | `cmd/node/reset_test.go` | `d74ce70348fc5d80` | **REQUIRED** | `[G][T]` | `NewBlockchainFromStoreForTest` 消费方 |
| B8 | `cmd/node/seed_parallel_test.go` | `0eedc0bc847abf42` | **REQUIRED** | `[G][T]` | `newNodeRuntimeForTest` 消费方 |
| B9 | `cmd/node/stop_lifecycle_test.go` | `08d4a782fed63f94` | **REQUIRED** | `[G][T]` | 新增 `ensureTestDataDir` |
| B10 | `cmd/node/verify_test.go` | `6b55ba166eda6e1d` | **REQUIRED** | `[G][T]` | `NewBlockchainFromStoreForTest` 消费方 |
| B11 | `internal/blockchain/blockchain_test.go` | `503d5ed825aa65b2` | **REQUIRED** | `[G][T]` | 同上（2 处） |
| B12 | `internal/blockchain/c1_hashindex_test.go` | `404ce9e590c23705` | **REQUIRED** | `[G][T]` | 同上 |
| B13 | `internal/blockchain/c1_noninterference_dump_test.go` | `b0968cbe959018c7` | **REQUIRED** | `[G][T]` | import 排序 + 改名 |
| B14 | `internal/blockchain/cd_canonset_test.go` | `de4dc51c3c73319d` | **REQUIRED** | `[G][T]` | 改名 |
| B15 | `internal/blockchain/difficulty_consensus_test.go` | `15237054fb57da34` | **REQUIRED** | `[G][T]` | 改名（2 处） |
| B16 | `internal/blockchain/obs_noninterference_test.go` | `13f6c6ed2650d7fb` | **REQUIRED** | `[G][T]` | 改名（2 处） |
| B17 | `internal/blockchain/reorg_test.go` | `31487b3fd9a05a35` | **REQUIRED** | `[G][T]` | 改名（多处）+ gofmt |
| B18 | `internal/blockchain/replay_persistence_test.go` | `c9b53c99eec68fc2` | **REQUIRED** | `[G][T]` | 改名（2 处） |
| B19 | `internal/mempool/reorg_resurrection_test.go` | `2ebab35845206d08` | **REQUIRED** | `[G][T]` | 改名 |
| B20 | `internal/storage/r3_crash_matrix_test.go` | `158f60f078935283` | **REQUIRED** | `[G][T]` | gofmt 对齐 |
| B21 | `internal/storage/r4a_legacy_length_matrix_test.go` | `7bd1acbd07bd9b64` | **REQUIRED** | `[G][T]` | gofmt 对齐 |
| B22 | `internal/storage/v2_test.go` | `e580439600627420` | **REQUIRED** | `[G][T]` | `NewBlockchainFromStoreForTest` 消费方 |

### 2.3 组 F —— F-3B 新源文件（3，**untracked**）

| # | 路径 | sha256（前 16） | 状态 | 层级 | 依据 |
|---|---|---|---|---|---|
| F-01 | `internal/blockchain/identity.go` | `9aa8fc6e28f8ee58` | **REQUIRED** | `[C][G][T]` | **M2 实测**：缺失 ⇒ `go vet` exit 1，**含生产构建**；定义 6 个符号 |
| F-02 | `cmd/node/f3b_test_helpers_test.go` | `4bd3d1232afc1e53` | **REQUIRED** | `[C][G][T]` | **M3 实测**：缺失 ⇒ `undefined: newNodeRuntimeForTest` |
| F-03 | `cmd/node/f3b_identity_test.go` | `b7a9e5a7b4790716` | **OPTIONAL**（编译）／**REQUIRED**（测试闭合） | `[G][T]` | **M4 实测**：移除后 `go vet` = 0；但承载 F-3B 的 9 子断言验收 |

### 2.4 组 X —— 编译强制的**外来**文件（1，**untracked**）

| # | 路径 | sha256（前 16） | 状态 | 层级 | 依据 |
|---|---|---|---|---|---|
| X-01 | `internal/blockchain/f1n1_reorg_coverage_test.go` | `c4606f09829ccc02` | **REQUIRED + BLOCKED** | `[C][G][T]` | **M1 实测**：缺失 ⇒ `undefined: f1n1UTXOSig`；ownership = **F1-N1**（OD-03）；**Commit Eligibility = BLOCKED**；**含 flaky 用例** |

**GCS 小计**：REQUIRED = 32（A7+B22+F2）＋ F-03（OPTIONAL/编译）＋ X-01（REQUIRED+BLOCKED） = **33**

---

## §3 — SET B（Other Authorized Work，6 文件）逐文件表

| # | 路径 | 状态 | 层级 | 依据 |
|---|---|---|---|---|
| C1 | `cmd/node/r1_boundary_test.go` | **REQUIRED（有益）／INSUFFICIENT／EXCLUDED（Genesis scope）** | `[G]` | **实测**：HEAD 版 `TestR1F4PreParkRejectionReasons` 为 O(N²) ⇒ **138.558s**；C1 版 **0.522s**（≈266×）。**但 `cmd/node` 包总时长仍 ≈575–605s，落在默认 `10m0s` 上限附近（4 次运行 1 PASS / 3 FAIL）** |
| C2 | `internal/explorer/explorer_test.go` | **REQUIRED（绿色）／EXCLUDED（Genesis scope）** | `[G]` | **实测**：HEAD 版断言 301，Go 1.27 返回 307 ⇒ `TestPathBoundaryRawTCP` FAIL；C2 版 ⇒ `internal/explorer` ok |
| C3 | `internal/control/server.go` | **EXCLUDED** | — | CONSOLE-MINE-AUTH-FIX-1；E2 实测 `go vet` = 0 |
| C4 | `internal/control/console_test.go` | **EXCLUDED** | — | 同上 |
| C5 | `internal/control/web/console.html` | **EXCLUDED** | — | 同上 |
| C6 | `internal/control/console_mine_auth_test.go` | **EXCLUDED** | — | 同上；**无阶段报告**（GAP-G-1） |

> **⚠️ C1/C2 的双重身份**：它们**不属** F-1～F-5，却**是绿色必需（C2）/ 有益但不足（C1）**。
> 这是 §14 的核心 scope 冲突来源。

---

## §4 — EXCLUDED（必须排除 / 不属边界）

### 4.1 凭据（🔴 强制排除）

| 路径 | tracked | ignored | 依据 |
|---|---|---|---|
| `audit-run/control-token` | N | **N** | OD-08 永久规则；对 `git add` **可见** |
| `f5-verify/control-token` | N | **N** | 同上 |
| `gui-test/token` | N | **N** | 同上 |

**⇒ 提交禁止 `git add -A` / `git add .` / `git commit -a` / `git add --all`。**

### 4.2 文档

| 路径 / 组 | 状态 | 依据 |
|---|---|---|
| `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` | **EXCLUDED** | D 类；+88/−4 追加 §9.11「PHASE V2.0 补录」；mtime 2026-09-13（**早于本会话**）；**无本链授权记录** |
| `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` | **无动作** | F-6.2 已恢复 ⇒ **= HEAD**（`e9def9aa…`），已离开 modified 集合 |
| 本链报告 F-1..F-5（7）＋ F-6.x（10） | **OUT OF BOUNDARY** | docs 提交另议 |
| `PHASE-REORG-1F..1J`（14） | **EXCLUDED** | 非本链 |
| `docs/` 其余 untracked `.md`（147） | **OUT OF BOUNDARY** | — |

### 4.3 可排除代码（编译无关）

| 路径 | 状态 | 依据 |
|---|---|---|
| `internal/attribution/attribution.go` | **EXCLUDED** | E1 实测；无 tracked 依赖；GAP-4；`ACCEPTED WITH SHA-DRIFT BLOCKER` |
| `internal/attribution/classify.go` | **EXCLUDED** | 同上 |
| `internal/attribution/attribution_test.go` | **EXCLUDED** | 同上 |
| `cmd/coinbase-attribution/main.go` | **EXCLUDED** | 同上 |
| `internal/blockchain/f1n2_reorg_detached_undo_test.go` | **EXCLUDED** | E3 实测 |
| `internal/storage/f1n1_undo_contract_test.go` | **EXCLUDED** | 同上（须与 f1n2 成对排除，§5.4） |
| `internal/storage/f1n2_detached_undo_contract_test.go` | **EXCLUDED** | 同上 |
| `internal/utxo/f1n1_undo_content_test.go` | **EXCLUDED** | 同上 |
| `internal/blockchain/query.go` | **EXCLUDED** | E 类 CRLF-only **零内容差异**（`git diff --quiet` = 0） |

### 4.4 验证产物（不可入库）

`f3b-verify/`、`f4-verify/`、`F-4-lab/`、`f5-verify/`（除 token）、`F4-*`、`F5-*`、`F-1-lab/`、
`audit-run/`、`gui-test/`、`verifier/`、`concept-mvp/`、`.workbuddy/`、`gui/`

---

## §5 — BLOCKED（边界必需但当前不可提交）

| ID | 对象 | 阻塞性质 | 来源 | 解除条件 |
|---|---|---|---|---|
| **BLK-1** | `internal/blockchain/f1n1_reorg_coverage_test.go`（X-01） | **治理 / scope**：编译强制，但 ownership = F1-N1；**Commit Eligibility = BLOCKED**（授权报告在**仓库之外**，F1-N1 未作为独立提交阶段授权） | OD-03 | Owner 裁定（OPT-A/B/C，§6.3） |
| **BLK-2** | `internal/explorer/explorer_test.go`（C2） | **治理 / scope**：绿色必需，但属独立授权工作流，不属 F-1～F-5 | §3 | Owner 裁定混合 scope 或单列提交 |
| **BLK-3** | `TestF1N1_C_MixedLegacyV2Reorg`（在 X-01 内） | **测试稳定性**：负载敏感 flaky，**无法从边界剥离**（编译强制） | F-6-FINDING-3 | Test Stabilization 阶段 |
| **BLK-4** | `cmd/node` 包（测试基建） | **基建**：固有 ≈**571s**，距 `go test` 默认 `10m0s`（600s）上限仅 ≈**29s（4.8%）**；全套并行（`storage` ~300s 同跑）即越界（**实测 1 PASS / 3 FAIL**）；即使含 C1 修复仍越界 | §3 / §13.3 | 提速（代码）或 `-timeout` 策略（基建）——须单独授权 |
| **BLK-5** | 3 个 token 文件 + `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` | **安全 / 治理**：token 未忽略；D 类文档无授权记录 | OD-08 / F-6-FINDING-4 | 显式白名单；Owner 裁定文档归属 |

---

## §6 — 两个候选边界

### 6.1 B-GENESIS（纯 Genesis 边界）

```
B-GENESIS = A(7) + B(22) + F(3) + X(1) = 33 个文件
            = 29 tracked-modified + 4 untracked
```

| 属性 | 实测（重做） |
|---|---|
| `go vet ./...` | **0** ✅ |
| `go build ./...` | **0** ✅ |
| `go test -count=1 ./internal/blockchain/...` | **0** ✅（65.706s） |
| `go test -count=1 ./...` | **1** ❌（`cmd/node` 600.369s 超时 + `internal/explorer` FAIL） |
| scope 纯度 | ⚠️ 含 1 个外来文件（X-01，BLOCKED） |

**⇒ B-GENESIS 可编译，但不完整（不绿），且含一个不可提交文件。**

### 6.2 B-GREEN（最接近绿色的边界）

```
B-GREEN = B-GENESIS + SET B(6) = 39 个文件
```

| 属性 | 实测（重做） |
|---|---|
| `go vet ./...` | **0** ✅ |
| `go test -count=1 ./...` | **1** ❌ —— `cmd/node` **600.448s 超时**（`panic: test timed out after 10m0s`） |
| `internal/explorer` | ✅ ok（C2 修复生效） |
| `cmd/node` 时长 | **隔离 571.249s（ok，`-timeout 25m`）**；全套并行 600.369 / 600.448 / 574.591 / 600.354 s ⇒ **距默认 `10m0s` 上限仅 ≈29s（4.8%），PASS 取决于负载** |
| r1 单用例 | **138.558s → 0.522s**（≈266×） |
| scope 纯度 | ❌ 含 **7** 个外来文件（X-01 + C1..C6） |

```
B-GREEN 最接近绿色，但【不能稳定变绿】：internal/explorer 已修复；
cmd/node 固有时长落在默认 10m 超时边界（BLK-4）。
```

### 6.3 结论

```
不存在「纯 F-1～F-5 Genesis Closure」的完整可提交边界；
也不存在可【稳定】变绿的边界（即使跨 scope）。
```

**⇒ VERDICT: BLOCKED / OWNER DECISION REQUIRED。** 候选方案见主报告 §14.4（OPT-A/B/C/D）。

---

## §7 — 提交强制约束（COMMIT CONSTRAINTS）

```
1. 禁止 git add -A / git add . / git commit -a / git add --all
2. 必须使用显式路径白名单（逐文件列举）
3. 3 个 token 文件永不进入提交（audit-run/ f5-verify/ gui-test/）
4. 禁止读取 / 打印 / 迁移 token 内容
5. .gitignore 未修改（f43418cf…）；加固须单独 Governance 阶段授权
6. 提交集合的最终决定权在 Owner
```

**本清单未执行任何 `git add` / `commit` / `push` / `reset` / `checkout` / `clean` / `restore`。**

---

## §8 — 边界声明

- 本清单**未执行**任何 `git` 写操作。
- 本清单**未修改**任何被审计文件（含 `.gitignore`）。
- 全部实验（`git archive HEAD` 抽取 + 注入 + `go vet`/`go test`）在**系统临时目录**进行，
  **仓库零改动**，抽取物与副本**收工前已删除**。
- 本清单**未读取**任何 token 实际内容。
- 本文件为**重做版**，覆盖前一次执行的同名交付物。

**HARD STOP。**
