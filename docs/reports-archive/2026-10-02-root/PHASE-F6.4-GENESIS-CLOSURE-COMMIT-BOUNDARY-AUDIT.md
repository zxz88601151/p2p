# PHASE F-6.4 — GENESIS CLOSURE COMMIT BOUNDARY AUDIT

**PHASE F-6.4**（本文件为 **重做版** — 按 Owner 授权 `重做 F-6.4` 从 pristine HEAD 重新执行）
**STRICT READ-ONLY ｜ FORENSIC SCOPE AUDIT ｜ COMMIT BOUNDARY DEFINITION**
**NO CODE CHANGE ｜ NO FILE RESTORE ｜ NO FILE DELETE ｜ NO GIT MUTATION**

| 项 | 值 |
|---|---|
| 仓库 | `E:/wakuang/p2pchain` |
| 审计基线 HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4`（branch `main`） |
| staged | **0** |
| tracked 内容修改（`git diff --name-only`） | **35** |
| porcelain ` M` | 36（含 1 个 CRLF-only 伪差异 `internal/blockchain/query.go`） |
| untracked（porcelain `??`） | **200**（= 198 基线 + 2 份本阶段交付物） |
| `.gitignore` | **未修改**（sha `f43418cf74ea996cc603976f29badc4c507e6b3ace25f7f619d7d79aefb6567b`） |
| 本阶段性质 | **只读取证 + 边界定义**；全部实验在**系统临时目录**进行，**仓库零改动** |
| 本阶段禁止 | `git add/commit/reset/restore/checkout/clean/stash/rebase/merge/push/tag`；修改任何被审计文件（含 `.gitignore`）；读取 token 实际内容；启停节点进程；挖矿 |
| 交付物 | 本报告 + `PHASE-F6.4-GENESIS-COMMIT-CANDIDATE-MANIFEST.md` |

> **本报告为自引用文档**：SHA-256 由收工后独立命令计算，登记于文件之外（memory / 交付说明）。

---

## §1 — PHASE IDENTITY

### 1.1 阶段目标

定义 **F-1～F-5 Genesis Closure** 的**最小完整提交边界**（minimal complete commit boundary）：

1. **Complete** = 该文件集合注入 pristine HEAD 后，`go vet ./...` **通过**，且 `go test -count=1 ./...` **通过**；
2. **Minimal** = 集合中**不存在**可移除的成员（移除任一成员即破坏上述任一条件）。

### 1.2 三个必须区分的层级

| 层级 | 判据 | 本报告结论 |
|---|---|---|
| **L1 编译闭合**（compile closure） | `go vet ./...` = 0 且 `go build ./...` = 0 | **GCS 已闭合**（§11.2） |
| **L2 确定性测试闭合**（deterministic test closure） | 隔离运行相关包 `go test` = 0 | **GCS 已闭合**（§11.3） |
| **L3 全套绿色**（green suite） | `go test -count=1 ./...` = 0 | **任何候选边界均未稳定闭合**（§11.4 / §13.3） |

> 规格要求「separate compile closure / deterministic test closure / flaky contamination」——
> 本报告证明这三者**互不等价**，且 **L3 ≠ L1 ∪ L2**（§13.4）。

### 1.3 方法论（复用 F-6.1 §0 的四证据框架）

归属判定使用 **E1 mtime 聚类 / E2 文件头自述 / E3 符号依赖 / E4 授权痕迹**。
**核心规则（OD-03 冻结，长期有效）**：

```
compile dependency ≠ phase ownership
```

E3（symbol dependency）**只证明 dependency**，**永远不能单独证明 ownership**。

### 1.4 重做声明

本阶段按 Owner 授权**从 pristine HEAD 重新执行**（前一次执行的 2 份交付物已被本文件覆盖）。
**全部测量独立重跑**；结果见 §11 / §12 / §13 的「重做复现」列。

---

## §2 — AUTHORITATIVE INPUTS（权威输入）

规格列出 6 项权威输入。**其中 2 项按名称不存在**——按规格要求「list actual paths if names differ」，
下表登记**实际路径**。

### 2.1 权威输入（实际存在，SHA-256 独立复算）

| # | 实际路径 | SHA-256 | 大小 |
|---|---|---|---|
| 1 | `PHASE-F6-GOVERNANCE-CLOSURE-COMMIT-READINESS-AUDIT.md` | `c439c47ffa0c354328e5f8473db597ee07bdc6d0dee09e12c04005ecc4530673` | 24,600 B |
| 2 | `PHASE-F6-COMMIT-CANDIDATE-MANIFEST.md` | `edf117f51856e9515f6c69ff5b8b79e4672d4cfa4d4216c53ff840e1b3f942d1` | 15,724 B |
| 3 | `PHASE-F6.1-OWNER-DECISION-FINALIZATION.md` | `5b95d47009dd49c88aacf890ed0df1f17f2dc99127e9bd583214923221c749cf` | 32,764 B |
| 4 | `PHASE-F6.1-OWNER-SCOPE-DECISION.md` | `3524b1659baa5d34f0d143e68f6f03e66000314715851a6af43a88c18f26442c` | 20,896 B |
| 5 | `PHASE-F6.2-P3.1-PROVENANCE-RESTORATION-REPORT.md` | `7db1e23894e8fd0d968cb3e335b2bd4de262b67c3833e8a88789063e7a3e04f0` | 12,940 B |
| 6 | `PHASE-F6.3-GAP4-SHA-DRIFT-FORENSIC-REPORT.md` | `34577f5d732a4c6644b77a2c869c704d0345f3861b86b95dc0fc03f90eb76a91` | 22,874 B |

### 2.2 规格命名但**不存在**的输入（**不得**以任何文件顶替）

| 规格名称 | `sha256sum` 结果 |
|---|---|
| `PHASE-F6.2-OWNER-DECISION-RECORD.md` | `No such file or directory` |
| `PHASE-F6.2-SCOPE-FREEZE-MATRIX.md` | `No such file or directory` |

**F-6.2 实际只产出 1 份交付物**：`PHASE-F6.2-P3.1-PROVENANCE-RESTORATION-REPORT.md`（上表 #5）。
本阶段**不伪造、不替代**这两个名称。

### 2.3 支持性输入（旁证，非规格列举）

| 路径 | SHA-256 |
|---|---|
| `PHASE-F6.1-FILE-PROVENANCE-MATRIX.md` | `86d20f62e2446811cde9c0452e15745e51c7d49aa8194aab101a4e2bbcf53f4d` |
| `PHASE-F6.1-SECRET-ARTIFACT-INVENTORY.md` | `a4a36f7095a5fabc1a43a029c78e318036ce5911bb6082932f7672870fbfc492` |
| `PHASE-F6.1-OD-OWNER-DECISION-CAPTURE.md` | `b0c09b06f1c164cd53b0e12603962b328dc8ba607d49e2a3383dd8d67f8d8f32` |

---

## §3 — BASELINE VERIFICATION（基线复验）

| 项 | 规格/历史值 | 本阶段实测 | 判定 |
|---|---|---|---|
| HEAD | `4d892be…` | `4d892be355a38886a83c123faad915c9f3cd62e4` | ✅ |
| 分支 | `main` | `main` | ✅ |
| staged | 0 | **0** | ✅ |
| tracked 内容修改 | **36**（F-6/F-6.1 快照） | **35** | ⚠️ 已解释（§3.1） |
| porcelain ` M` | 37 | **36** | ⚠️ 同上 |
| untracked（`??`） | 198 | **200** | ✅（= 198 + 2 交付物） |
| `.gitignore` | `f43418cf…` | `f43418cf74ea996c…` | ✅ 未修改 |
| `git diff --check` | **exit 2**（F-6 §9.3） | **exit 0** | ✅ **已消解**（§3.1） |

### 3.1 一处**预期差异**：36 → 35（F-6.2 的后果，非未授权漂移）

`PHASE F-6.2` **已授权**地将 `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` 逐字节恢复为其历史 blob。
恢复后该文件**等于 HEAD**，因而**离开 modified 集合**：tracked 修改 **36 → 35**、porcelain ` M` **37 → 36**。

**副产品**：F-6 §9.3 的 `git diff --check` = exit 2（trailing whitespace，全部命中该 D 类文档）
**随之自然消解** ⇒ 本阶段实测 **exit 0**。

> **⇒ 基线判定：PASS。差异为 F-6.2 的**已记录预期后果**，非未授权漂移。**

### 3.2 受保护对象（全部未变）

| 对象 | SHA-256 | 判定 |
|---|---|---|
| `node.exe` | `3972843aff90428dd67acb39c6aaa009bd04d42982603617c5dbf6f0e8c213e1` | ✅ |
| `run-a/blocks.dat` | `893e64c12598558ff93793248d363536e1d090a6970c2d12ed632af3eea2fb2f` | ✅ |
| `run-b/blocks.dat` | `5ab67bb4f2787fb11555135671379269818c3fc43fff20057a8743c4f4e1df53` | ✅ |
| `run-a/wallet.json` | `bf577525dcab4f2584812b357b82248435d5a8b85e84cf6cb8218c9d86844a94` | ✅ |
| `run-b/wallet.json` | `762c369374117141fc18a2b687df7e81aed5b8a385f449a8a4a9128a6faa5190` | ✅ |

未导入 / 未签名 / 未复制 / 未输出任何私钥。

### 3.3 上游对象复核

| 对象 | SHA-256 | 判定 |
|---|---|---|
| `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md`（F-6.2 已恢复） | `e9def9aa1f90e5acec7447c8c2236da4ef7eeb6411ee440e03ec7e0a5746fe61` | ✅ 与 F-6.2 登记一致 |
| `internal/attribution/attribution.go` | `3f4cb77820b43747de5354b1f095336e175c1afa4157758223d834d5dac3dadb` | ✅ 与 F-6.3 一致 |
| `internal/attribution/classify.go` | `bb7b00bc135c6146c9ca110f93080b839bd963d99cd78d3eb578ec7a07865060` | ✅ |
| `internal/attribution/attribution_test.go` | `4d7841d418a668818b5c151245045d1a37029e2e77e12d32b779a5b7b77d8b7c` | ✅ |
| `cmd/coinbase-attribution/main.go` | `c634c1ecaea5955118533d035af2d0db2fa8f18365fa579a22c0b7401bfa6f18` | ✅ |

---

## §4 — GENESIS-CANDIDATE-SET（GCS）定义

**GCS = 「F-1～F-5 Genesis Closure 的最小完整提交边界」的候选集合**，由 4 个组构成。

### 4.1 组 A —— F-3B 生产代码（7，tracked 修改）

| # | 路径 | sha256（前 16） | 大小 |
|---|---|---|---|
| A1 | `cmd/node/main.go` | `f22436ba406feb3a` | 36,406 |
| A2 | `cmd/node/cli.go` | `7d9b1c67d4d0012f` | 34,746 |
| A3 | `internal/blockchain/blockchain.go` | `8cf75cb11c5a9e02` | 44,683 |
| A4 | `internal/blockchain/verify.go` | `05db33fa2b276a9c` | 3,450 |
| A5 | `internal/blockchain/genesis.go` | `b6a2c2235a70d4fd` | 2,196 |
| A6 | `internal/storage/file.go` | `1385fc7433470cd8` | 8,275 |
| A7 | `internal/storage/v2.go` | `95e7d9edda30fe33` | 30,856 |

### 4.2 组 B —— F-3B 测试夹具（22，tracked 修改）

| # | 路径 | sha256（前 16） | 大小 |
|---|---|---|---|
| B1 | `cmd/node/datalock_integration_test.go` | `48b72ac80eab229a` | 2,551 |
| B2 | `cmd/node/fullstack_test.go` | `bc14215436319e48` | 17,471 |
| B3 | `cmd/node/lock_lifecycle_test.go` | `fbb6e8b07f975ed8` | 11,274 |
| B4 | `cmd/node/p2p_branch_test.go` | `21a255f471ce104f` | 32,097 |
| B5 | `cmd/node/r3_crash_restart_test.go` | `4a25cb3aa8769a2c` | 7,982 |
| B6 | `cmd/node/r4b_dual_reorg_convergence_test.go` | `41879fcd121d851f` | 23,791 |
| B7 | `cmd/node/reset_test.go` | `d74ce70348fc5d80` | 16,388 |
| B8 | `cmd/node/seed_parallel_test.go` | `0eedc0bc847abf42` | 4,141 |
| B9 | `cmd/node/stop_lifecycle_test.go` | `08d4a782fed63f94` | 20,966 |
| B10 | `cmd/node/verify_test.go` | `6b55ba166eda6e1d` | 11,960 |
| B11 | `internal/blockchain/blockchain_test.go` | `503d5ed825aa65b2` | 19,435 |
| B12 | `internal/blockchain/c1_hashindex_test.go` | `404ce9e590c23705` | 7,319 |
| B13 | `internal/blockchain/c1_noninterference_dump_test.go` | `b0968cbe959018c7` | 3,399 |
| B14 | `internal/blockchain/cd_canonset_test.go` | `de4dc51c3c73319d` | 15,826 |
| B15 | `internal/blockchain/difficulty_consensus_test.go` | `15237054fb57da34` | 25,843 |
| B16 | `internal/blockchain/obs_noninterference_test.go` | `13f6c6ed2650d7fb` | 9,041 |
| B17 | `internal/blockchain/reorg_test.go` | `31487b3fd9a05a35` | 18,074 |
| B18 | `internal/blockchain/replay_persistence_test.go` | `c9b53c99eec68fc2` | 8,968 |
| B19 | `internal/mempool/reorg_resurrection_test.go` | `2ebab35845206d08` | 20,841 |
| B20 | `internal/storage/r3_crash_matrix_test.go` | `158f60f078935283` | 30,965 |
| B21 | `internal/storage/r4a_legacy_length_matrix_test.go` | `7bd1acbd07bd9b64` | 45,856 |
| B22 | `internal/storage/v2_test.go` | `e580439600627420` | 32,067 |

### 4.3 组 F —— F-3B 新源文件（3，**untracked**）

| # | 路径 | sha256（前 16） | 大小 | 自述 |
|---|---|---|---|---|
| F-01 | `internal/blockchain/identity.go` | `9aa8fc6e28f8ee58` | 3,207 | F-3B |
| F-02 | `cmd/node/f3b_test_helpers_test.go` | `4bd3d1232afc1e53` | 763 | F-3B |
| F-03 | `cmd/node/f3b_identity_test.go` | `b7a9e5a7b4790716` | 5,781 | F-3B |

### 4.4 组 X —— 编译强制的**外来**文件（1，**untracked**）

| # | 路径 | sha256（前 16） | 大小 | 自述 ownership |
|---|---|---|---|---|
| X-01 | `internal/blockchain/f1n1_reorg_coverage_test.go` | `c4606f09829ccc02` | 25,981 | **F1-N1**（OD-03） |

### 4.5 GCS 规模

```
GCS = A(7) + B(22) + F(3) + X(1) = 33 个文件
    = 29 tracked-modified + 4 untracked
```

> **⚠️ 组 X 的边界性质**：X-01 **不是** Genesis 产物，但**被 Genesis 编译强制**（§5）。
> 它同时是 §13 的 flaky 载体、§14 的 scope 冲突焦点。

---

## §5 — GENESIS TEST DEPENDENCY CLOSURE（测试依赖闭合）

Genesis closure 的符号依赖通过 **Go 包级编译单元**成立：同一 package 的
`_test.go` 与生产 `.go` **共享符号空间**，缺符号 ⇒ `undefined`。

### 5.1 三个**编译必需**符号（实测）

| 符号 | 定义位置 | 跟踪状态 | 消费方（实测） |
|---|---|---|---|
| `ErrUninitializedStore` / `VerifyGenesisIdentity` / `NewBlockchainFromStoreForTest` / `InitializeBlockchainStore` / `ErrGenesisMismatch` / `ErrCorruptGenesisIdentity` | `internal/blockchain/identity.go`（F-01） | **UNTRACKED** | `blockchain.go`（**生产**）、`verify.go`（**生产**）、`cmd/node/cli.go`、`cmd/node/main.go` + 12 个测试文件 |
| `newNodeRuntimeForTest` | `cmd/node/f3b_test_helpers_test.go`（F-02） | **UNTRACKED** | `datalock_integration_test.go`、`fullstack_test.go`、`lock_lifecycle_test.go`、`p2p_branch_test.go`、`seed_parallel_test.go` |
| `f1n1UTXOSig` | `internal/blockchain/f1n1_reorg_coverage_test.go`（X-01） | **UNTRACKED** | **`internal/blockchain/p1_reorg_fail_before_commit_test.go:63`（TRACKED、未修改、在 HEAD 中）** |

### 5.2 `f1n1UTXOSig` 证据链（决定性）

| 字段 | 值 |
|---|---|
| consumer | `internal/blockchain/p1_reorg_fail_before_commit_test.go`（**tracked，未修改**，`git cat-file -e HEAD:…` → PRESENT） |
| consumer 位置 | 第 63 行 `utxoSig:        f1n1UTXOSig(bc),`（`func p1SnapshotState(bc *Blockchain) p1ReorgState` 内，**实代码**） |
| provider | `internal/blockchain/f1n1_reorg_coverage_test.go:134` `func f1n1UTXOSig(bc *Blockchain) string {` |
| provider 跟踪状态 | **UNTRACKED**（`git cat-file -e HEAD:…` → `exists on disk, but not in 'HEAD'`） |
| provider 自述 ownership | **F1-N1**（文件头：`PHASE P2PCHAIN — F1-N1 UNDO CONTRACT TEST DESIGN / LEGACY-V2 REORG COVERAGE AUDIT`） |
| provider 的进一步依赖 | 自身第 60 行使用 `NewBlockchainFromStoreForTest` ⇒ **依赖 F-01** |

**⇒ X-01 编译必需，但 ownership = F1-N1（OD-03）。**

### 5.3 反向依赖（无）

`internal/attribution` 的**唯一** importer 是 `cmd/coinbase-attribution/main.go`（**两者皆 untracked**）：
```
$ grep -rn "p2pchain/internal/attribution" --include=*.go .
./cmd/coinbase-attribution/main.go:26:	"p2pchain/internal/attribution"
```
⇒ GAP-4 是**自封闭孤岛**，**无任何 tracked 文件依赖它**（§7）。

### 5.4 未跟踪测试文件之间的**内部**耦合（影响排除策略）

| 定义者（untracked） | 符号 | 被谁（untracked）引用 |
|---|---|---|
| `internal/blockchain/f1n1_reorg_coverage_test.go` | `hx4`, `f1n1Store`, `f1n1ChainFromStore`, `f1n1Tip`, `f1n1Canonical`, `f1n1Mine`, `f1n1MineBelow` | `internal/blockchain/f1n2_reorg_detached_undo_test.go` |
| `internal/storage/f1n1_undo_contract_test.go` | `f1n1OpenRW` | `internal/storage/f1n2_detached_undo_contract_test.go` |

⇒ F1-N2 测试**依赖** F1-N1 测试文件（同包）。**二者必须同进同出**（不得只取其一）。

---

## §6 — F1-N1 / F1-N2 EXCLUSION TEST（排除测试）

### 6.1 待判定对象

| ID | 路径 | 自述 | 结论 |
|---|---|---|---|
| X-01 | `internal/blockchain/f1n1_reorg_coverage_test.go` | F1-N1 | **不可排除**（编译强制，§5.2） |
| — | `internal/blockchain/f1n2_reorg_detached_undo_test.go` | F1-N2 | **可排除**（`go vet` = 0） |
| — | `internal/storage/f1n1_undo_contract_test.go` | F1-N1 | **可排除** |
| — | `internal/storage/f1n2_detached_undo_contract_test.go` | F1-N2 | **可排除**（但依赖上一个，须成对排除） |
| — | `internal/utxo/f1n1_undo_content_test.go` | F1-N1 | **可排除** |

### 6.2 实测（E3 = GCS + 上述 4 个可排除项）

```
$ go vet ./...      → exit 0
```

**⇒ F1-N1/F1-N2 的 4 个非强制文件**（f1n2_reorg / storage×2 / utxo）**可排除**；
**唯独 X-01 不可排除**——它不是「F1-N1 想不想进」的问题，而是**Genesis 编译的硬约束**。

### 6.3 判定

```
F1-N1 / F1-N2 EXCLUSION: PARTIAL
  - 4 个文件: EXCLUDABLE（编译无关）
  - 1 个文件 (X-01): NOT EXCLUDABLE（被 tracked 的 p1_reorg_fail_before_commit_test.go:63 强制）
```

---

## §7 — GAP-4 EXCLUSION TEST

### 7.1 待判定对象（4，全部 untracked）

`internal/attribution/attribution.go`、`internal/attribution/classify.go`、
`internal/attribution/attribution_test.go`、`cmd/coinbase-attribution/main.go`

### 7.2 实测

| 实验 | 命令 | 结果 |
|---|---|---|
| **排除**（GCS 本身未注入） | `go vet ./...` | **exit 0** ✅ |
| **加入**（E1 = GCS + 4 文件） | `go vet ./...` | **exit 0** ✅ |

### 7.3 判定

```
GAP-4 EXCLUSION: YES — FULLY EXCLUDABLE
```

依据：**无 tracked 文件依赖**（§5.3）；包自封闭；加入/移除均不影响 Genesis 编译与测试。

> 附带说明：GAP-4 的 4 个文件带 **SHA-DRIFT BLOCKER**（OD-04 / OD-05，
> `ACCEPTED WITH SHA-DRIFT BLOCKER`），F-6.3 已裁定为 **P2 VERIFIED DESCENDANT**。
> 无论其 provenance 是否闭合，**它都不属于 Genesis 边界**。

---

## §8 — CONSOLE / EXPLORER / AUDIT-FIX CLASSIFICATION（SET B）

**SET B（Other Authorized Work）**= 本会话**更早已独立授权**、**不属于** F-1～F-5 的改动。

### 8.1 SET B 成员（6）

| # | 路径 | 类型 | 阶段自述 | 跟踪 |
|---|---|---|---|---|
| C1 | `cmd/node/r1_boundary_test.go` | tracked 修改 | AUDIT-FIX-RUN-MINING-1 | tracked |
| C2 | `internal/explorer/explorer_test.go` | tracked 修改 | AUDIT-FIX（Go 1.22+ ServeMux） | tracked |
| C3 | `internal/control/server.go` | tracked 修改 | CONSOLE-MINE-AUTH-FIX-1 | tracked |
| C4 | `internal/control/console_test.go` | tracked 修改 | CONSOLE-MINE-AUTH-FIX-1 | tracked |
| C5 | `internal/control/web/console.html` | tracked 修改 | CONSOLE-MINE-AUTH-FIX-1 | tracked |
| C6 | `internal/control/console_mine_auth_test.go` | **untracked** | CONSOLE-MINE-AUTH-FIX-1 | untracked |

### 8.2 ★ 关键发现：**SET B 修复了 explorer，但**不足以**稳定 cmd/node**

#### 8.2.1 C2（explorer）—— 测试**断言**失败（可被 SET B 完全修复）

```
--- FAIL: TestPathBoundaryRawTCP (0.00s)
    explorer_test.go:373: 重复斜杠被清洗: GET /api//status 响应行不含期望状态码 301；实测响应头:
        HTTP/1.1 307 Temporary Redirect
```

HEAD 的 `explorer_test.go` 断言 **301**；当前 Go（go1.27.0）ServeMux 对路径规范化请求返回 **307**。
C2 的改动（`301` → `307`）**彻底修复**该失败（E4 两次运行 `internal/explorer` 均 ok）。

#### 8.2.2 C1（r1_boundary）—— 测试**挂死**（O(N²)），修复效果显著但**不解决 cmd/node 超时**

HEAD 的 `TestR1F4PreParkRejectionReasons` 在内层循环中反复调用 `oversize.Size()`
（`Size() = len(Block.Encode())`，**每次重新序列化整块**），且内层期间 `Outputs` 尚未回写
⇒ 内层退化为「固定做 N 次全块序列化」，总代价 **O(N²)**（1 MiB 块 ≈ 数十 GB 序列化）。
C1 改为「内层只追加（摊还 O(1)），每轮外层仅一次 `Size()` 检查」⇒ **O(N)**。

**实测（重做）**：

| 树 | 单用例 `TestR1F4PreParkRejectionReasons` | 加速比 |
|---|---|---|
| GCS（HEAD 版） | **138.558s** | 1× |
| E4（C1 版） | **0.522s** | **≈266×** |

**但**：`cmd/node` 包**总时长**仍处于 **10m0s 默认超时边界**（§8.3）。

### 8.3 ★★ 重做新发现：**`cmd/node` 本身就在 10 分钟超时边界上**

五次 `cmd/node` 实测：

| 运行 | 树 | 条件 | `cmd/node` 时长 | 结果 |
|---|---|---|---|---|
| **隔离** | GCS+SET B(39) | 单独运行、无并行负载（`-timeout 25m`） | **571.249s** | ✅ ok |
| 重做 E4 #2 | GCS+SET B(39) | 全套并行 | **600.448s** | ❌ 超时 |
| （前次）E4 #1 | GCS+SET B(39) | 全套并行 | 574.591s | ✅ ok |
| 重做 GCS | GCS(33) | 全套并行 | **600.369s** | ❌ 超时 |
| （前次）GCS | GCS(33) | 全套并行 | 600.354s | ❌ 超时 |

**⇒ 结论：`cmd/node` 的固有运行时长 ≈ 571s，而 `go test` 默认包超时为 `10m0s = 600s`
⇒ 余量仅 ≈29s（4.8%）。** 任何并行负载（全套 `./...` 会同时跑 `storage` ~300s）即把它推过上限。

**⇒ 即使加入 SET B（含 C1 修复），L3 绿色也**不可稳定达成**。**

> 超时点在不同运行中落在**不同测试**（`TestR4B_DualProcessCompetitiveForkConverges` /
> `TestResetRefusesWhenStopFailed`）⇒ 证明这是**包级累计时长**问题，而非某个用例挂死。

### 8.4 判定

```
SET B CLASSIFICATION: REQUIRED-FOR-GREEN(explorer) / INSUFFICIENT-FOR-STABLE-GREEN(cmd/node)
                      NOT GENESIS-OWNED
```

- **编译**：SET B **非必需**（GCS 已 `go vet` = 0）；
- **绿色**：C2 **必需**（缺则 explorer FAIL）；C1 **大幅有益但不足**（cmd/node 仍在超时边界）；
- **ownership**：SET B = 独立授权工作流（AUDIT-FIX / CONSOLE），**不属** F-1～F-5。

> **⇒ 这是本阶段的核心 scope 冲突**：Genesis 边界要「complete（绿色）」就必须**混入 SET B**，
> 而混入后**仍不能稳定变绿**。详见 §14。

---

## §9 — DOCUMENT CLASSIFICATION（文档分类）

| 类别 | 路径 | 状态 | 边界判定 |
|---|---|---|---|
| **D 类（历史脏改动）** | `docs/DETERMINISTIC-SERIALIZATION-SPEC.md`（+88/−4，追加 §9.11「PHASE V2.0 补录」，mtime 2026-09-13，**早于本会话**） | tracked 修改 | **EXCLUDE**（无本链授权记录） |
| **已恢复** | `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` | **= HEAD**（F-6.2 恢复，`e9def9aa…`） | 已离开 modified 集合，**无动作** |
| **本链报告（F-1..F-5）** | `PHASE-F1…`/`F2`/`F3`/`F3A`/`F3B`/`F4`/`F5`（7） | untracked | **OUT OF BOUNDARY**（docs 提交另议） |
| **F-6.x 报告（10）** | `FINDING-REGISTRY-F1-F5.md` + `PHASE-F6*`（9） | untracked | **OUT OF BOUNDARY** |
| **历史阶段归档** | `PHASE-REORG-1F..1J`（14） | untracked | **EXCLUDE**（非本链） |
| **`docs/` 其他** | 147 个 untracked `.md` | untracked | **OUT OF BOUNDARY** |

**判定**：**无任何文档属于 Genesis Closure 代码提交边界**。
唯一在 modified 集合中的文档（D 类）**必须排除**。

---

## §10 — SECRET EXCLUSION（凭据排除）

### 10.1 实测

| 路径 | 存在 | tracked | ignored | `git check-ignore` |
|---|---|---|---|---|
| `audit-run/control-token` | Y | **N** | **N** | exit 1（NOT-IGNORED） |
| `f5-verify/control-token` | Y | **N** | **N** | exit 1 |
| `gui-test/token` | Y | **N** | **N** | exit 1 |
| `secrets/control-token` | N | — | — | — |

**⇒ 3 个 token 文件均「未跟踪且未被忽略」⇒ 对 `git add` 可见。**

### 10.2 边界规则（OD-08 冻结，长期有效）

```
NEVER git add -A / git add . / git commit -a / git add --all
ALWAYS explicit path whitelist
NEVER print token contents
NEVER migrate token into another artifact
```

**本阶段未读取任何 token 实际内容**；`.gitignore` **未修改**（`f43418cf…`）。

### 10.3 判定

```
SECRET EXCLUSION: MANDATORY — 3 files (audit-run/, f5-verify/, gui-test/)
```

---

## §11 — MINIMUM CLOSURE EXPERIMENT

**实验环境**：`git archive HEAD | tar -x -C <TEMP>` 得到 **pristine baseline**（242 文件），
然后**仅注入 GCS（33 文件）**。仓库**零改动**。

### 11.1 pristine HEAD 复现（F-6-FINDING-1）

```
$ go build ./...                 → exit 0
$ go vet ./internal/blockchain/  → # p2pchain/internal/blockchain [p2pchain/internal/blockchain.test]
                                   internal\blockchain\p1_reorg_fail_before_commit_test.go:63:19: undefined: f1n1UTXOSig
                                   exit 1
```

⇒ **HEAD 的 `internal/blockchain` 测试包无法编译**（生产构建 `go build ./...` 正常）。

### 11.2 L1 — COMPILE CLOSURE（GCS 注入后）

| 命令 | 退出码 | 判定 |
|---|---|---|
| `go vet ./...` | **0** | ✅ **PASS** |
| `go build ./...` | **0** | ✅ **PASS** |

```
COMPILE CLOSURE (GCS = 33 files): ACHIEVED
```

### 11.3 L2 — DETERMINISTIC TEST CLOSURE（隔离）

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test -count=1 ./internal/blockchain/...` | **0** | `ok p2pchain/internal/blockchain 65.706s` ✅ |

⇒ **隔离运行下 blockchain 包全绿**（含 `TestF1N1_C_MixedLegacyV2Reorg`）。

```
DETERMINISTIC TEST CLOSURE (isolated): ACHIEVED
```

### 11.4 L3 — FULL SUITE（`go test -count=1 ./...`）

```
?   	p2pchain/cmd/explorer	[no test files]
FAIL	p2pchain/cmd/node	600.369s
ok  	p2pchain/internal/block	0.704s
ok  	p2pchain/internal/blockchain	76.238s
ok  	p2pchain/internal/blocktree	0.277s
?   	p2pchain/internal/config	[no test files]
ok  	p2pchain/internal/control	3.771s
FAIL	p2pchain/internal/explorer	0.532s
ok  	p2pchain/internal/mempool	36.231s
ok  	p2pchain/internal/obs	0.300s
ok  	p2pchain/internal/p2p	15.170s
ok  	p2pchain/internal/pow	1.863s
ok  	p2pchain/internal/storage	298.036s
ok  	p2pchain/internal/transaction	0.348s
ok  	p2pchain/internal/txbuild	0.279s
ok  	p2pchain/internal/utxo	0.378s
ok  	p2pchain/internal/wallet	3.915s
FAIL
gcs-full-exit=1
```

**15 ok / 2 FAIL（+2 no-test-files）。**

| 失败包 | 退出信息 | 归因 |
|---|---|---|
| `p2pchain/cmd/node` | `panic: test timed out after 10m0s` | 包级累计时长越 10m 上限（§8.3） |
| `p2pchain/internal/explorer` | `--- FAIL: TestPathBoundaryRawTCP`（301 vs 307） | **缺失 C2** |

> **注意**：`internal/blockchain` **PASS**（76.238s）——`TestF1N1_C_MixedLegacyV2Reorg`
> **本次未复现 flaky**。**单次 PASS 不构成稳定性证据**（§13.4）。

```
FULL SUITE GREEN (GCS = 33 files): NOT ACHIEVED
```

---

## §12 — MINIMALITY TEST（最小性测试）

### 12.1 M 系列 —— **移除**（证明「必需」）

| # | 从 GCS 移除 | `go vet ./...` | 缺失符号 | 判定 |
|---|---|---|---|---|
| **M1** | `internal/blockchain/f1n1_reorg_coverage_test.go`（X-01） | **exit 1** | `undefined: f1n1UTXOSig`（`p1_reorg_fail_before_commit_test.go:63`） | **REQUIRED（编译）** |
| **M2** | `internal/blockchain/identity.go`（F-01） | **exit 1** | `ErrUninitializedStore` / `VerifyGenesisIdentity`（**生产构建**）+ `NewBlockchainFromStoreForTest` | **REQUIRED（编译）** |
| **M3** | `cmd/node/f3b_test_helpers_test.go`（F-02） | **exit 1** | `undefined: newNodeRuntimeForTest` | **REQUIRED（编译）** |
| **M4** | `cmd/node/f3b_identity_test.go`（F-03） | **exit 0** | — | **OPTIONAL（编译）**；REQUIRED（F-3B 测试闭合） |
| **M5** | 组 B 的 22 个夹具（**回退到 HEAD 版本**） | **exit 0** | — | **OPTIONAL（编译）**；REQUIRED（绿色：语义已变） |

**M2 的关键性质**：失败**包含生产构建**（`# p2pchain/internal/blockchain`，**无 `.test` 后缀**）
⇒ F-01 缺失是**生产代码不可编译**，非仅测试问题。

**M5 的机制说明**：`newNodeRuntime`（`cmd/node/main.go:74`）与 `NewBlockchainFromStore`
（`internal/blockchain/blockchain.go:218`）**在修改后仍存在**，故 HEAD 的测试文件**仍可编译**；
但 F-3B 已改变二者**语义**（不再自动建创世）⇒ HEAD 版测试**运行期失败**。
故组 B 是**测试闭合必需**、**编译可选**。

### 12.2 E 系列 —— **加入**（证明「可排除」）

| # | GCS + 加入 | `go vet ./...` | 判定 |
|---|---|---|---|
| **E1** | GAP-4（4 文件） | **exit 0** | ✅ 可排除（加/减皆安全） |
| **E2** | Console 组（C3–C6） | **exit 0** | ✅ 编译安全 |
| **E3** | F1-N1/N2 其余 4 文件 | **exit 0** | ✅ 可排除 |
| **E4** | **SET B 全组**（C1–C6） | **exit 0** | ✅ 编译安全；修复 explorer |

### 12.3 最小性结论

```
MINIMALITY: ESTABLISHED
  - 编译最小集 = { F-01, F-02, X-01 }（3 个 untracked）
  - 组 A/B/F-03 = 编译可选，但「绿色」必需
  - GAP-4 / F1-N1-N2 其余 / docs / secrets = 可排除
  - SET B = 编译可选；C2 为 explorer 绿色必需；C1 大幅有益但不足（§8.3）
```

---

## §13 — 三层分离（COMPILE / DETERMINISTIC / FLAKY）

### 13.1 分离矩阵

| 集合 | L1 编译 | L2 隔离测试 | L3 全套绿色 |
|---|---|---|---|
| pristine HEAD | ✅（仅 build） | ❌（blockchain 测试包不编译） | ❌ |
| **GCS（33）** | ✅ | ✅ | ❌（cmd/node 超时 + explorer FAIL） |
| **GCS + SET B（39）** | ✅ | ✅ | ❌ **不稳定**（explorer 已修复；cmd/node 仍 574.6s~600.4s 越界，§8.3） |

### 13.2 GREEN BOUNDARY（最小绿色集合）

```
MINIMAL GREEN SET (MGS) = GCS(33) + SET B(6) = 39 个文件
```

**但 MGS 也不能稳定变绿**：`internal/explorer` 被 C2 修复，`cmd/node` 仍在 10m 超时边界（§8.3）。
⇒ **MGS 是「最接近绿色」的候选，而非「稳定绿色」。**

### 13.3 L3 不可稳定达成的两个独立原因

| # | 原因 | 证据 | 层级 |
|---|---|---|---|
| **G-1** | `cmd/node` 固有 ≈**571s** vs 默认 `10m0s`（600s）⇒ 余量仅 **≈29s（4.8%）** | 隔离 571.249s（ok）；全套并行 600.369 / 600.448 / 574.591 / 600.354 s | **测试基建**（`-timeout` 或提速） |
| **G-2** | `internal/explorer` HEAD 断言 301 vs Go 1.27 的 307 | E4 修复后 ok；GCS 下 FAIL | **SET B（C2）** |

### 13.4 FLAKY CONTAMINATION

| 项 | 值 |
|---|---|
| 用例 | `TestF1N1_C_MixedLegacyV2Reorg` |
| 位置 | **X-01**（`internal/blockchain/f1n1_reorg_coverage_test.go:560`） |
| 性质 | **负载敏感 flaky**（F-6-FINDING-3）：隔离 PASS / 整包 PASS / 全套并行曾 FAIL |
| 本阶段实测 | GCS 全套中 PASS（76.238s）；E4 全套中 PASS（87.973s） |
| 结构性后果 | **X-01 编译强制** ⇒ **flaky 用例被自动拖入 Genesis 边界**，无法「不带它」 |

> **⇒ 编译闭合（L1）与测试稳定性（L3）在 X-01 上**耦合**：
> 为让树可编译，必须纳入一个**含已知 flaky 用例**的外来文件。

### 13.5 三层分离结论

```
L1 compile closure        : GCS 充分
L2 deterministic closure  : GCS 充分
L3 green suite            : 任何候选边界均未稳定达成（GCS ❌ / MGS 不稳定）
L1 ∪ L2 ≠ L3              : 证明成立（GCS 满足 L1+L2，仍不满足 L3）
```

---

## §14 — COMMIT BOUNDARY DEFINITION（提交边界定义）

### 14.1 两个候选边界

| 边界 | 组成 | 规模 | 编译 | 稳定绿色 | scope 纯度 |
|---|---|---|---|---|---|
| **B-GENESIS**（纯 Genesis） | A(7)+B(22)+F(3)+X(1) = GCS | **33** | ✅ | ❌ | ⚠️ 含 1 外来（X-01） |
| **B-GREEN**（最接近绿色） | GCS + SET B(6) = MGS | **39** | ✅ | ❌（不稳定，§8.3） | ❌ 含 7 外来 |

### 14.2 结构冲突（**核心结论**）

1. **B-GENESIS 不含 X-01 ⇒ 不编译**（M1 实测 `undefined: f1n1UTXOSig`）；
   **含 X-01 ⇒ 违反 OD-03**（`compile dependency ≠ ownership`；X-01 的
   **Commit Eligibility = BLOCKED**，其授权报告位于**仓库之外**，F1-N1 尚未作为独立提交阶段授权）。
2. **B-GENESIS 不绿**：缺 C1/C2 即 `cmd/node` 超时 + `explorer` FAIL；
   **补 C1/C2 ⇒ 变成 B-GREEN，混入 SET B**（独立授权工作流，不属 F-1～F-5），
   **且仍不保证绿色**（`cmd/node` 越界，§8.3）。
3. **flaky 无法剥离**：X-01 编译强制 ⇒ `TestF1N1_C_MixedLegacyV2Reorg` 必随行。

```
⇒ 不存在一个「纯 F-1～F-5 Genesis Closure」的完整可提交边界；
  也不存在一个可稳定变绿的边界（即使跨 scope）。
```

### 14.3 五个**已登记**的前置阻塞（均为**既存**，非本阶段引入）

| ID | 阻塞 | 来源 | 性质 |
|---|---|---|---|
| **BLK-1** | X-01 编译强制但 ownership = F1-N1 且 Commit Eligibility = **BLOCKED** | OD-03 | **治理 / scope** |
| **BLK-2** | 绿色必需 SET B（C2），其 ownership 属独立工作流 | §8.2.1 | **治理 / scope** |
| **BLK-3** | `TestF1N1_C_MixedLegacyV2Reorg` 负载敏感 flaky，无法从边界剥离 | F-6-FINDING-3 | **测试稳定性** |
| **BLK-4** | **`cmd/node` 固有 ≈571s，落在默认 `10m0s`（600s）上限内仅 ≈29s** | §8.3 | **测试基建**（新增） |
| **BLK-5** | 3 个未忽略 token；D 类文档须排除 | OD-08 / F-6-FINDING-4 | **安全 / 治理** |

### 14.4 可执行的边界**形状**（供 Owner 选择，本阶段**不执行**）

| 选项 | 内容 | 代价 |
|---|---|---|
| **OPT-A** | 接受**混合 scope 提交**：MGS(39) 一次性提交（Genesis + X-01 + SET B） | 违反 OD-03 纯度；且仍不能稳定绿 |
| **OPT-B** | **先**授权 F1-N1 独立提交（X-01 落地）**再**提交 Genesis | 需 F1-N1 授权 + 报告归档决策 |
| **OPT-C** | **代码重构**：把 `f1n1UTXOSig`（及 flaky 用例）从 X-01 抽出为 **Genesis-owned 测试夹具** | 需**写入阶段**授权（本阶段只读，不得实施） |
| **OPT-D** | 提交前**先修** flaky + 解决 `cmd/node` 超时（提速或 `-timeout` 策略），再定边界 | 需 Test Stabilization 阶段 |

> 本阶段**只定义边界与冲突**，**不代 Owner 选择**，**不实施任何选项**。

---

## §15 — OUT-OF-SCOPE CONFIRMATION（范围确认）

| 项 | 确认 |
|---|---|
| 本阶段是否修改任何源码 / 测试？ | **否** |
| 本阶段是否修改 `.gitignore`？ | **否**（`f43418cf…` 未变） |
| 本阶段是否恢复 / 删除任何文件？ | **否**（仅覆盖本阶段自己的 2 份交付物） |
| 本阶段是否执行 `git add` / `commit` / `push` / `reset` / `restore` / `checkout` / `clean`？ | **否** |
| 本阶段是否读取 token 实际内容？ | **否**（仅 `ls` / `check-ignore` 元数据） |
| 本阶段是否启停节点 / 挖矿？ | **否**（仅 `go vet` / `go build` / `go test` 在**临时目录**） |
| 本阶段是否把任何文件写入仓库？ | **否**（除 2 份交付物报告） |
| 实验是否污染仓库？ | **否**（`git archive` + 复制到 `$TEMP`，仓库零改动） |

---

## §16 — GIT MUTATION AUDIT & ZERO-DRIFT PROOF

### 16.1 未执行的操作（明文声明）

**未执行**：`git add` / `commit` / `push` / `tag` / `merge` / `rebase` / `amend` / `squash` /
`reset` / `checkout` / `clean` / `restore` / `stash` / `update-index`。
**未修改**任何被审计文件（含 `.gitignore`、任何 `.go`、任何 `.md`）。
**未启停**任何节点进程；**未运行**任何挖矿。

### 16.2 Zero-Drift Proof（开工 vs 收工）

| 项 | 开工 | 收工 | 漂移 |
|---|---|---|---|
| HEAD | `4d892be…` | `4d892be…` | **0** |
| 分支 | `main` | `main` | **0** |
| staged | 0 | 0 | **0** |
| tracked 内容修改 | 35 | 35 | **0** |
| porcelain ` M` | 36 | 36 | **0** |
| untracked（`??`） | 200 | **200** | **0**（2 份交付物为覆盖写，计数不变） |
| `.gitignore` | `f43418cf…` | `f43418cf…` | **0** |
| 受保护对象（5 项 SHA-256） | — | — | **0** |
| P3.1 恢复文档 | `e9def9aa…` | `e9def9aa…` | **0** |
| GAP-4（4 项 SHA-256） | — | — | **0** |
| GCS 33 文件 SHA-256 | — | — | **0**（与首次执行逐字节相同） |

### 16.3 唯一工作树副作用（披露）

- 本阶段交付物为**覆盖写**：`PHASE-F6.4-GENESIS-CLOSURE-COMMIT-BOUNDARY-AUDIT.md`、
  `PHASE-F6.4-GENESIS-COMMIT-CANDIDATE-MANIFEST.md`（untracked 计数**保持 200**）。
- 全部实验在**系统临时目录** `%TEMP%` 进行（`f64pris` / `f64exp` / `f64_m1..m5` / `f64_e1..e4`），
  **仓库零改动**；抽取物与副本**收工前已删除**。

---

## §17 — VERDICT（判定）+ HARD STOP

```
PHASE F-6.4 — GENESIS CLOSURE COMMIT BOUNDARY AUDIT（重做）

BASELINE:                          PASS
AUTHORITATIVE INPUTS:              6 verified (2 spec-named inputs DO NOT EXIST)
COMPILE CLOSURE (GCS 33):          PASS   (go vet ./... = 0, go build ./... = 0)
DETERMINISTIC TEST CLOSURE:        PASS   (internal/blockchain 65.706s ok)
FULL-SUITE GREEN (GCS 33):         FAIL   (cmd/node 600.369s 超时 + explorer FAIL)
FULL-SUITE GREEN (GCS+SET B 39):   FAIL   (不稳定：cmd/node 574.6s~600.4s 越界)
MINIMALITY:                        ESTABLISHED
SET B:                             REQUIRED-FOR-GREEN(explorer) / INSUFFICIENT(cmd/node)
GAP-4 / F1-N1-N2 / docs / secrets: EXCLUDABLE
FLAKY CONTAMINATION:               PRESENT (TestF1N1_C_MixedLegacyV2Reorg, in X-01)

GENESIS-ONLY COMPLETE BOUNDARY:    DOES NOT EXIST
STABLE-GREEN BOUNDARY:             DOES NOT EXIST
SCOPE CONFLICT:                    PRESENT (BLK-1 … BLK-5)

VERDICT:  BLOCKED / OWNER DECISION REQUIRED
HARD STOP:  YES
```

### 17.1 为何是 BLOCKED（而非 COMPLETE）

规格要求定义「**minimal complete** commit boundary」——**complete 必须同时满足可编译与绿色**。
本阶段（重做）证明：

1. **纯 Genesis 边界不编译**（缺 X-01 ⇒ `undefined: f1n1UTXOSig`）；
2. **纳入 X-01 即违反 OD-03**（X-01 ownership = F1-N1，Commit Eligibility = **BLOCKED**）；
3. **纯 Genesis 边界不绿**（缺 C1/C2 ⇒ cmd/node 超时 + explorer FAIL）；
4. **补 C1/C2 即混入 SET B**，且**仍不保证绿色**（`cmd/node` 固有 ≈**571s**，距 `10m0s` 上限仅 ≈**29s**，**实测 1 PASS / 3 FAIL**）。

⇒ 「完整」与「纯 Genesis scope」**互斥**；且「完整」本身**不可稳定达成**。
这**不是**可由 AI 自行消解的工程问题，而是**需要 Owner 裁定的治理问题**（§14.4 四选项）。

### 17.2 重做 vs 首次执行的差异（披露）

| 项 | 首次 | 重做 | 说明 |
|---|---|---|---|
| GCS 编译/隔离 | ✅/✅ | ✅/✅ | 一致 |
| GCS 全套 | exit 1（cmd/node 600.354s + explorer FAIL） | exit 1（600.369s + explorer FAIL） | **一致** |
| E4 全套 | exit 0（cmd/node 574.591s） | **exit 1（cmd/node 600.448s 超时）** | **新增发现：cmd/node 处于超时边界** |
| 最小性 M1–M5/E1–E3 | 同 | 同 | 一致 |
| R1 单用例 | 118.083s → 1.000s | **138.558s → 0.522s** | 量级一致（≈118× / ≈266×） |

**⇒ 重做**强化**了结论：不仅纯 Genesis 边界不存在，**连跨 scope 的「绿色」边界也不稳定**（新增 BLK-4）。

### 17.3 未自动推进

**未自动进入任何下一阶段。** 未 `commit` / `push` / `tag`。
未实施 OPT-A/B/C/D 中的任何一项。未修改 flaky 用例。未修改 `.gitignore`。

### 17.4 交付物

| # | 文件 |
|---|---|
| 1 | `PHASE-F6.4-GENESIS-CLOSURE-COMMIT-BOUNDARY-AUDIT.md`（本报告，17 节） |
| 2 | `PHASE-F6.4-GENESIS-COMMIT-CANDIDATE-MANIFEST.md`（逐文件表 + REQUIRED/OPTIONAL/EXCLUDED/BLOCKED） |

**HARD STOP。**
