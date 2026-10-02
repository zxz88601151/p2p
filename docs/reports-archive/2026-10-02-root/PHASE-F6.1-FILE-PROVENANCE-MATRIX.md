# PHASE F-6.1 — FILE PROVENANCE MATRIX

**PHASE F-6.1 — OWNER SCOPE RECONCILIATION (STRICT READ-ONLY)**
**Deliverable 2 of 3**

| 项 | 值 |
|---|---|
| 仓库 | `E:/wakuang/p2pchain` |
| 基线 HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| staged | 0 |
| tracked 内容修改 | 36 |
| porcelain ` M` | 37（含 1 stat-only） |
| untracked（porcelain `??`） | 191 |
| untracked（展开为文件） | 364 |
| 性质 | **STRICT READ-ONLY** — 0 修改 / 0 删除 / 0 移动 / 0 Git 写 |

---

## §0 — 方法（METHOD）

归属判定使用**四条独立证据**，**不**使用「测试需要」作为归属依据：

| 证据 | 手段 |
|---|---|
| **E1 时间簇** | `stat -c '%y'` — 文件 mtime 聚类 |
| **E2 内容自述** | 文件头部的 phase 自声明（如 `PHASE P2PCHAIN — F1-N1`） |
| **E3 依赖关系** | 声明符号提取 + 跨文件引用（`git ls-files` × `grep`） |
| **E4 授权痕迹** | 报告文件 / prompt 规格 / commit 历史（`git log -- <path>`） |

**归属成立** 需同时满足：`依赖 + phase ownership + 授权 + scope`（F-6.1 §13）。
任一缺失 ⇒ **SET C（UNKNOWN）**。

---

## §1 — 三个集合的定义与规模

| 集合 | 定义 | 文件数 |
|---|---|---|
| **SET A** | F-1～F-5 Protocol Closure（Genesis Identity / explicit init / fail-closed / real-node validation） | 29 tracked + 3 untracked = **32** |
| **SET B** | Other Authorized Work（Console / Explorer / r1_boundary / 其他此前已授权阶段），**不属于** F-1～F-5 | 5 tracked + 1 untracked = **6**（代码）；另有大宗历史文档归档 |
| **SET C** | **UNKNOWN / UNRESOLVED** — 无法可靠证明归属 | **2 tracked + 8 untracked = 10** |
| **（E）** | CRLF/stat 伪差异（零内容差异，不计入 36） | 1 |

---

## §2 — 36 个 tracked 修改：逐文件矩阵

| # | PATH | TYPE | E1 时间簇 | E2 自述 | E3 依赖 | E4 授权 | PHASE | SET | COMMIT CANDIDATE |
|---|---|---|---|---|---|---|---|---|---|
| 1 | `cmd/node/main.go` | 生产 | 09-27 10:59 | F-3B | 调 `ErrCorruptGenesisIdentity`/`ErrUninitializedStore` | F-3A §13 / F-3B 报告 | **F-3B** | **A** | ✅ 是 |
| 2 | `cmd/node/cli.go` | 生产 | 09-27 10:58 | F-3B | 定义 `cmdInit` | F-3A OD-03 | **F-3B** | **A** | ✅ 是 |
| 3 | `internal/blockchain/blockchain.go` | 生产 | 09-27 10:48 | F-3B | 用 `VerifyGenesisIdentity`/`ErrUninitializedStore`（**来自 untracked identity.go**） | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-01**） |
| 4 | `internal/blockchain/verify.go` | 生产 | 09-27 10:48 | F-3B | 同上 | F-3A OD-05 | **F-3B** | **A** | ✅ 是（**依赖 F-01**） |
| 5 | `internal/blockchain/genesis.go` | 生产 | 09-27 12:36 | F-3B | 定义 `CanonicalGenesisHash` | F-3A / F-3B 报告 | **F-3B** | **A** | ✅ 是 |
| 6 | `internal/storage/file.go` | 生产 | 09-27 10:58 | F-3B | 定义 `OpenFileBlockStoreStrict` | F-3B 报告 | **F-3B** | **A** | ✅ 是 |
| 7 | `internal/storage/v2.go` | 生产 | 09-27 10:58 | F-3B | `repairOnOpen` 门 | F-3B 报告 | **F-3B** | **A** | ✅ 是 |
| 8 | `cmd/node/datalock_integration_test.go` | 测试 | 09-27 10:52 | — | 用 `newNodeRuntimeForTest`（**来自 untracked f3b_test_helpers**） | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-02**） |
| 9 | `cmd/node/fullstack_test.go` | 测试 | 09-27 10:52 | — | 同上 | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-02**） |
| 10 | `cmd/node/p2p_branch_test.go` | 测试 | 09-27 10:52 | — | 同上 | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-02**） |
| 11 | `cmd/node/r3_crash_restart_test.go` | 测试 | 09-27 10:52 | — | gofmt 对齐 | F-3B 报告 | **F-3B** | **A** | ✅ 是 |
| 12 | `cmd/node/r4b_dual_reorg_convergence_test.go` | 测试 | 09-27 10:52 | — | gofmt 对齐 | F-3B 报告 | **F-3B** | **A** | ✅ 是 |
| 13 | `cmd/node/reset_test.go` | 测试 | 09-27 10:52 | — | 用 `NewBlockchainFromStoreForTest`（**来自 F-01**） | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-01**） |
| 14 | `cmd/node/seed_parallel_test.go` | 测试 | 09-27 10:52 | — | 用 `newNodeRuntimeForTest`（**F-02**） | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-02**） |
| 15 | `cmd/node/verify_test.go` | 测试 | 09-27 10:52 | — | 用 `NewBlockchainFromStoreForTest`（**F-01**） | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-01**） |
| 16 | `internal/blockchain/blockchain_test.go` | 测试 | 09-27 10:52 | — | 同上（**F-01**） | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-01**） |
| 17 | `internal/blockchain/c1_hashindex_test.go` | 测试 | 09-27 10:52 | — | 同上（**F-01**） | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-01**） |
| 18 | `internal/blockchain/c1_noninterference_dump_test.go` | 测试 | 09-27 10:52 | — | 同上（**F-01**） | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-01**） |
| 19 | `internal/blockchain/cd_canonset_test.go` | 测试 | 09-27 10:52 | — | 同上（**F-01**） | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-01**） |
| 20 | `internal/blockchain/difficulty_consensus_test.go` | 测试 | 09-27 10:52 | — | 同上（**F-01**） | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-01**） |
| 21 | `internal/blockchain/obs_noninterference_test.go` | 测试 | 09-27 10:52 | — | 同上（**F-01**） | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-01**） |
| 22 | `internal/blockchain/reorg_test.go` | 测试 | 09-27 10:52 | — | 同上（**F-01**）+ struct gofmt | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-01**） |
| 23 | `internal/blockchain/replay_persistence_test.go` | 测试 | 09-27 10:52 | — | 同上（**F-01**） | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-01**） |
| 24 | `internal/mempool/reorg_resurrection_test.go` | 测试 | 09-27 10:52 | — | 同上（**F-01**） | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-01**） |
| 25 | `internal/storage/r3_crash_matrix_test.go` | 测试 | 09-27 10:52 | — | gofmt 对齐 | F-3B 报告 | **F-3B** | **A** | ✅ 是 |
| 26 | `internal/storage/r4a_legacy_length_matrix_test.go` | 测试 | 09-27 10:52 | — | gofmt 对齐 | F-3B 报告 | **F-3B** | **A** | ✅ 是 |
| 27 | `internal/storage/v2_test.go` | 测试 | 09-27 10:52 | — | 用 `NewBlockchainFromStoreForTest`（**F-01**） | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-01**） |
| 28 | `cmd/node/stop_lifecycle_test.go` | 测试 | 09-27 12:25 | — | 定义 `ensureTestDataDir`；调 `cmdInit` | F-3B 报告 | **F-3B** | **A** | ✅ 是 |
| 29 | `cmd/node/lock_lifecycle_test.go` | 测试 | 09-27 12:25 | — | 用 `ensureTestDataDir`+`newNodeRuntimeForTest`（**F-02**） | F-3B 报告 | **F-3B** | **A** | ✅ 是（**依赖 F-02**） |
| 30 | `internal/control/server.go` | 生产 | 09-27 09:20 | **CONSOLE-MINE-AUTH-FIX-1**（注释自述） | 定义 `consoleOriginGate` | 仅 session memory；**无报告文件** | **CONSOLE-MINE-AUTH-FIX-1** | **B** | ⚠️ Owner |
| 31 | `internal/control/console_test.go` | 测试 | 09-27 09:19 | 同上 | 正则同步 | 同上 | **CONSOLE-MINE-AUTH-FIX-1** | **B** | ⚠️ Owner |
| 32 | `internal/control/web/console.html` | 前端 | 09-27 09:19 | 同上 | 改用 `/console/mine` | 同上 | **CONSOLE-MINE-AUTH-FIX-1** | **B** | ⚠️ Owner |
| 33 | `internal/explorer/explorer_test.go` | 测试 | 09-27 08:50 | — | 301→307 | **`E:/wakuang/PHASE-AUDIT-FIX-RUN-MINING-1-REPORT.md`**（D-2 节，含本文件 +12/-4） | **AUDIT-FIX-RUN-MINING-1** | **B** | ⚠️ Owner |
| 34 | `cmd/node/r1_boundary_test.go` | 测试 | 09-27 08:54 | — | O(N²) 修复 | **同上报告**（明确列 `cmd/node/r1_boundary_test.go (+12/-4)`） | **AUDIT-FIX-RUN-MINING-1** | **B** | ⚠️ Owner |
| 35 | `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` | 文档 | **09-13 09:30** | 「PHASE V2.0 补录」 | — | **无本链授权记录**；末次 commit `91dfee6` | **UNKNOWN** | **C** | ❌ Owner |
| 36 | `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` | 文档 | **09-12 21:52** | 「PHASE BRAND-0D.3」 | — | **无本链授权记录**；HEAD 内容属 commit `535edb7`，**工作树内容被外来阶段整体替换** | **UNKNOWN** | **C** | ❌ Owner |

> **F-01 / F-02** = §3.1 的两个**强制依赖**（见 §3）。

### 2.1 E 类（stat-only，不计入 36）

| PATH | 证据 | SET |
|---|---|---|
| `internal/blockchain/query.go` | `git diff --quiet` → exit 0；`--numstat` → 空；仅在 CRLF 警告列表出现 | **E**（零内容差异） |

---

## §3 — 未跟踪源文件矩阵（13 个）

### 3.1 强制依赖（被 tracked / 待提交文件引用）

| ID | PATH | TYPE | E1 时间簇 | E2 自述 | E3 依赖（**被谁需要**） | PHASE | SET | COMMIT CANDIDATE |
|---|---|---|---|---|---|---|---|---|
| **F-01** | `internal/blockchain/identity.go` | 生产 | 09-27 10:51 | F-3B | **被 15 个 tracked 修改引用**：`blockchain.go`/`verify.go`（生产）+ 8 个测试 + `cli.go`/`reset_test.go`/`verify_test.go`/`reorg_resurrection_test.go`/`v2_test.go` | **F-3B** | **A** | ✅ 是（**编译必需**） |
| **F-02** | `cmd/node/f3b_test_helpers_test.go` | 测试 | 09-27 10:54 | F-3B | 定义 `newNodeRuntimeForTest`，**被 5 个 tracked 修改引用** | **F-3B** | **A** | ✅ 是（**编译必需**） |
| **F-03** | `internal/blockchain/f1n1_reorg_coverage_test.go` | 测试 | 09-27 10:52 | **PHASE P2PCHAIN — F1-N1** | 定义 `f1n1UTXOSig`，**被 tracked（未修改）的 `p1_reorg_fail_before_commit_test.go:63` 引用**；自身又依赖 F-01 的 `NewBlockchainFromStoreForTest` | **F1-N1** | **C** | ⚠️ **Owner**（见 OD-03） |

> ⚠️ **F-03 是本阶段的核心张力**：它**编译必需**，但**归属 F1-N1（不是 F-1～F-5）**。
> 按 §13 规则，**不得**因「编译需要」就自动归入 F-1～F-5。

### 3.2 无强制依赖的未跟踪源文件

| PATH | TYPE | E1 时间簇 | E2 自述 | E3 依赖 | E4 授权 | PHASE | SET | COMMIT CANDIDATE |
|---|---|---|---|---|---|---|---|---|
| `cmd/node/f3b_identity_test.go` | 测试 | 09-27 10:57 | F-3B | 定义 `TestF3BExplicitInitializationStateMatrix` + `initTestDir`/`writeForeignGenesis`/`fileHash`（**无 tracked 引用**） | F-3B 报告（明列 9 子断言） | **F-3B** | **A** | ✅ 是 |
| `internal/control/console_mine_auth_test.go` | 测试 | 09-27 09:20 | **CONSOLE-MINE-AUTH-FIX-1** | 测 tracked 的 `consoleOriginGate`（**反向依赖，非被依赖**） | 仅 session memory；**无报告文件** | **CONSOLE-MINE-AUTH-FIX-1** | **B** | ⚠️ Owner |
| `internal/attribution/attribution.go` | 生产 | 09-22 14:45 | 「独立只读分析工具，**不属于生产共识路径**」 | **无 tracked 引用** | `docs/PHASE-P2PCHAIN-GAP-4-COINBASE-ATTRIBUTION-…-REPORT.md`（untracked） | **GAP-4** | **C** | ⚠️ Owner（见 OD-04） |
| `internal/attribution/classify.go` | 生产 | 09-22 14:45 | 同上 | 无 tracked 引用 | 同上 | **GAP-4** | **C** | ⚠️ Owner（OD-04） |
| `internal/attribution/attribution_test.go` | 测试 | 09-22 14:45 | 同上 | 无 tracked 引用 | 同上 | **GAP-4** | **C** | ⚠️ Owner（OD-04） |
| `cmd/coinbase-attribution/main.go` | 生产(main) | 09-22 14:45 | GAP-4 工具 | **唯一** import `p2pchain/internal/attribution`；**无 tracked 反向引用** | 同上 | **GAP-4** | **C** | ⚠️ Owner（见 OD-05） |
| `internal/blockchain/f1n2_reorg_detached_undo_test.go` | 测试 | 09-20 07:55 | **PHASE P2PCHAIN — F1-N2** | 无 tracked 引用 | `docs/prompts/F1-GAP-CONTRACT.md` 等 | **F1-N2** | **C** | ⚠️ Owner（见 OD-06） |
| `internal/storage/f1n1_undo_contract_test.go` | 测试 | 09-22 14:45 | **F1-N1** | 无 tracked 引用 | 同 F1-N1 | **F1-N1** | **C** | ⚠️ Owner（OD-06） |
| `internal/storage/f1n2_detached_undo_contract_test.go` | 测试 | 09-20 07:52 | **F1-N2** | 无 tracked 引用 | 同 F1-N2 | **F1-N2** | **C** | ⚠️ Owner（OD-06） |
| `internal/utxo/f1n1_undo_content_test.go` | 测试 | 09-20 07:12 | **F1-N1** | 无 tracked 引用 | 同 F1-N1 | **F1-N1** | **C** | ⚠️ Owner（OD-06） |

---

## §4 — 依赖证据链（DEPENDENCY EVIDENCE）

F-6.1 §5 要求为 `f1n1UTXOSig` 建立结构化证据。完整链条：

| 字段 | 值 |
|---|---|
| **consumer** | `internal/blockchain/p1_reorg_fail_before_commit_test.go`（**TRACKED，未修改，在 HEAD 中**） |
| **consumer 位置** | 第 63 行：`utxoSig: f1n1UTXOSig(bc),`（**实代码**，非注释） |
| **symbol** | `f1n1UTXOSig` |
| **provider** | `internal/blockchain/f1n1_reorg_coverage_test.go` |
| **provider 位置** | 第 134 行 `func f1n1UTXOSig(bc *Blockchain) string {` |
| **provider 跟踪状态** | **UNTRACKED**（`git cat-file -e HEAD:…` → `exists on disk, but not in 'HEAD'`） |
| **why provider is required** | Go 包级编译单元：同一 package 的测试文件共享符号空间；缺 provider ⇒ `undefined: f1n1UTXOSig` |
| **which phase provider belongs to** | **F1-N1**（文件头自述：`PHASE P2PCHAIN — F1-N1 UNDO CONTRACT TEST DESIGN / LEGACY-V2 REORG COVERAGE AUDIT`） |
| **provider 的进一步依赖** | provider 自身使用 `NewBlockchainFromStoreForTest`（定义于 **F-01**，即 untracked `identity.go`） |

### 4.1 实测证据（pristine HEAD 抽取，仓库零改动）

```
$ git archive HEAD | tar -x -C $TMP
$ ( cd $TMP && go vet ./internal/blockchain/ )
internal\blockchain\p1_reorg_fail_before_commit_test.go:63:19: undefined: f1n1UTXOSig
vet-exit=1
```

### 4.2 「36 个 tracked 修改」单独注入的实测（决定性）

```
$ git archive HEAD | tar -x -C $TMP
$ for f in $(git diff --name-only); do cp "$f" "$TMP/$f"; done   # 仅 36 个 tracked 修改
$ ( cd $TMP && go vet ./... )
# p2pchain/internal/blockchain
internal\blockchain\blockchain.go:224:15: undefined: ErrUninitializedStore
internal\blockchain\blockchain.go:227:16: undefined: VerifyGenesisIdentity
internal\blockchain\verify.go:57:16:     undefined: VerifyGenesisIdentity
internal\blockchain\c1_hashindex_test.go:157:14: undefined: NewBlockchainFromStoreForTest
internal\blockchain\cd_canonset_test.go:108:14:   undefined: NewBlockchainFromStoreForTest
internal\blockchain\p1_reorg_fail_before_commit_test.go:63:19: undefined: f1n1UTXOSig
internal\blockchain\reorg_test.go:154:17:         undefined: NewBlockchainFromStoreForTest
... 
vet-exit=1
```

**去重后的全部缺失符号（4 个）**：

| 缺失符号 | 提供者 | 提供者跟踪状态 |
|---|---|---|
| `ErrUninitializedStore` | `internal/blockchain/identity.go` | **UNTRACKED** |
| `VerifyGenesisIdentity` | `internal/blockchain/identity.go` | **UNTRACKED** |
| `NewBlockchainFromStoreForTest` | `internal/blockchain/identity.go` | **UNTRACKED** |
| `f1n1UTXOSig` | `internal/blockchain/f1n1_reorg_coverage_test.go` | **UNTRACKED** |

> **注意**：失败**包含生产构建**（`# p2pchain/internal/blockchain`，无 `.test` 后缀）
> ⇒ 不是「仅测试编译」问题，而是**生产代码不可编译**。

### 4.3 `cmd/node` 的独立缺失符号

（在上一步清除 `internal/blockchain` 的阻塞后单独实测）

```
$ ( cd $TMP && go vet ./cmd/node/ )
vet.exe: cmd\node\datalock_integration_test.go:39:12: undefined: newNodeRuntimeForTest
vet-exit=1
```

| 缺失符号 | 提供者 | 提供者跟踪状态 |
|---|---|---|
| `newNodeRuntimeForTest` | `cmd/node/f3b_test_helpers_test.go` | **UNTRACKED** |

### 4.4 最小必要集合的实测收敛

| 注入内容 | `go vet ./...` |
|---|---|
| 仅 36 个 tracked 修改 | **exit 1** |
| 36 + `identity.go` + `f3b_test_helpers_test.go` + `f1n1_reorg_coverage_test.go` | **exit 0** ✅ |
| 再 + `f3b_identity_test.go` + `console_mine_auth_test.go` | **exit 0** ✅ |

**⇒ 编译必需的未跟踪集合 = { F-01, F-02, F-03 }**（3 个）。
其中 **F-01、F-02 属 F-3B（SET A）**；**F-03 属 F1-N1（SET C）** ⇒ 须 Owner 裁定（OD-03）。

---

## §5 — 未跟踪文档 / 产物（分组）

### 5.1 本链报告（SET A）

| 文件 | PHASE |
|---|---|
| `PHASE-F1-GENESIS-SUBSIDY-COMPATIBILITY-AUDIT.md` | F-1 |
| `PHASE-F2-PROTOCOL-IDENTITY-DECISION-FREEZE.md` | F-2 |
| `PHASE-F3-GENESIS-IDENTITY-STARTUP-READINESS-AUDIT.md` | F-3 |
| `PHASE-F3A-GENESIS-IDENTITY-CONTRACT-FREEZE.md` | F-3A |
| `PHASE-F3B-IDENTITY-IMPLEMENTATION-EXECUTION.md` | F-3B |
| `PHASE-F4-CONTROLLED-NEW-CHAIN-INITIALIZATION-REPORT.md` | F-4 |
| `PHASE-F5-REAL-NODE-VALIDATION-REPORT.md` | F-5 |
| `FINDING-REGISTRY-F1-F5.md` · `PHASE-F6-*.md` ×2 | F-6 |

### 5.2 历史阶段归档（SET B — 非本链）

| 组 | 规模 | 判定 |
|---|---|---|
| `docs/PHASE-P2PCHAIN-*.md`（A-2.3 / A-2.5 / CUTOVER / E1 / HOST-QA / WATCHER / FAIRNESS / SECURITY 等） | ~120 份 | 更早已授权阶段 |
| `docs/prompts/*.prompt.md` + `F1-GAP-CONTRACT.md` | ~40 份 | 阶段规格/契约 |
| `PHASE-REORG-1F…1J-*.md`（仓库根） | 14 份 | REORG 系列 |
| `docs/PHASE-BRAND-0D.3-FRESH-VERIFICATION.md` | 1 份 | BRAND-0D.3 |
| `docs/canary-*.md` / `canary.txt` | 3 份 | 金丝雀标记 |

### 5.3 产物目录（非源码）

| 目录 | Git 可见性 | SET | COMMIT CANDIDATE |
|---|---|---|---|
| `F-1-lab/` | 可见 | B | ❌ 排除（实验产物） |
| `audit-run/` | **可见（含 token）** | B | ❌ 排除（**含凭据**） |
| `concept-mvp/` | 可见 | B | ❌ 排除 |
| `f5-verify/` | **可见（含 token）** | A | ❌ 排除（**含凭据**） |
| `gui-test/` | **可见（含 token）** | B | ❌ 排除（**含凭据**） |
| `gui/` | 可见 | B | ❌ 排除（GUI 未授权） |
| `verifier/` | 可见 | B | ❌ 排除 |
| `.workbuddy/` | 可见 | B | ❌ 排除（工作快照） |
| `f3b-verify/` / `f4-verify/` / `F4-*` / `F5-*` | 部分/不可见 | A | ❌ 排除（验证产物） |

---

## §6 — 集合归属汇总

| SET | 代码文件 | 说明 |
|---|---|---|
| **A** | 29 tracked（#1–29）+ 3 untracked（F-01, F-02, F-03 中 F-01/F-02）… 见下注 | F-3B Protocol Closure |
| **B** | 5 tracked（#30–34）+ 1 untracked（`console_mine_auth_test.go`） | Console / Explorer / r1_boundary |
| **C** | 2 tracked（#35–36）+ 8 untracked（F-03 + attribution ×4 + F1-N1/N2 ×3） | **UNKNOWN / UNRESOLVED** |

> **注**：SET A 的 3 个 untracked = `identity.go`(F-01)、`f3b_test_helpers_test.go`(F-02)、`f3b_identity_test.go`。
> **F-03 不属 SET A**（它是 F1-N1 文件，仅因编译依赖被牵连）。

### 6.1 规模校验

```
SET A 代码：29 tracked + 3 untracked = 32
SET B 代码： 5 tracked + 1 untracked =  6
SET C 代码： 2 tracked + 8 untracked = 10   （其中 F-03 同时是编译必需）
             ------------------------------
tracked 合计：29 + 5 + 2 = 36 ✅（与 baseline 一致）
untracked 源文件合计：3 + 1 + 8 = 12 … + F-03 = 13 ✅（与 §3 的 13 个一致）
```

> `F-03` 在 §3.1 单列（强制依赖），在集合上归 **C**。

---

## §7 — 与 F-6 结论的关系

| F-6 finding | F-6.1 的处置 |
|---|---|
| F-6-FINDING-1（编译不足） | **强化**：不止测试，**生产代码**亦不可编译；缺失符号 **4 + 1 = 5 个**，跨 3 个 untracked 文件 |
| F-6-FINDING-2（token 未忽略） | 转 §`PHASE-F6.1-SECRET-ARTIFACT-INVENTORY.md` |
| F-6-FINDING-4（G Unknown） | **精确定位**为 #35/#36 两个文档（OD-01 / OD-02） |
| F-6-FINDING-5（未跟踪工作流） | **拆解**为 GAP-4（attribution）+ F1-N1 + F1-N2 三个独立工作流（OD-04/05/06） |

---

## §8 — 边界声明

- 本矩阵**未**修改 / 删除 / 移动任何文件；**未**执行任何 Git 写操作。
- `git archive` 抽取仅写入**系统临时目录**（`$TEMP`），**仓库零改动**，抽取物**已删除**。
- 全部归属结论均标注证据（E1–E4）；**未使用「测试需要」作为归属依据**。
- 任何归属不成立的文件一律标 **SET C / UNKNOWN**，**最终裁定权在 Owner**。

**HARD STOP。**
