# PHASE F-6 — COMMIT CANDIDATE MANIFEST

**PHASE F-6 — GOVERNANCE CLOSURE & COMMIT READINESS AUDIT (STRICT READ-ONLY)**
**Deliverable 2 of 3**

| 项 | 值 |
|---|---|
| 仓库 | `E:/wakuang/p2pchain` |
| 审计基线 HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| 分支 | `main` |
| staged | **0** |
| tracked 内容修改 | **36** |
| porcelain ` M`（含 1 个 stat-only） | **37** |
| untracked（porcelain `??`） | **188**（开工时） |
| 本清单性质 | **只读分析**；不执行任何 `git add` / `commit` / `push` |

> 本清单**不是**一条可执行的提交命令。它是「若要提交，必须提交哪些文件、必须排除哪些文件」的
> 逐文件登记。**最终提交集合须由 Owner 单独授权**（F-6 §Commit Boundary 要求）。

---

## §1 — 分类口径（A–G）

本清单使用的分类是**显式定义**的，便于 Owner 复核与反驳：

| 类 | 定义 | 是否属于 F-3B/F-4/F-5 授权范围 |
|---|---|---|
| **A** | F-3B **生产代码**：identity gate 实现核心 | ✅ 是 |
| **B** | F-3B **测试夹具**：机械改名（`…ForTest`）、gofmt 对齐、`ensureTestDataDir` 新增 | ✅ 是 |
| **C** | 本会话**更早已授权阶段**的改动（CONSOLE-MINE-AUTH-FIX-1；审计修复 explorer/r1） | ⚠️ 相邻（同会话、独立授权） |
| **D** | **早于本会话**的历史工作树改动（2026-09-12/13），与 F-1..F-5 无授权关联 | ❌ 否 |
| **E** | CRLF/stat **伪差异**（零内容差异） | ❌ 否（无内容） |
| **F** | **未跟踪**的 F-3B 新源文件（`identity.go`、`f3b_*_test.go`） | ✅ 是（但不在「36」内） |
| **G** | **无法归属到任何已授权阶段 = UNKNOWN ⇒ HARD STOP** | 🚫 触发 HARD STOP |

**判定规则**：出现任一 **G** ⇒ **HARD STOP**（F-6 规格明文）。

---

## §2 — 36 个 tracked 修改的逐文件分类

来源判定依据：**文件 mtime 聚类**（见 §2.1）+ **逐文件 diff 内容**。

### 2.1 mtime 聚类（客观证据）

| mtime 簇 | 文件数 | 归属 |
|---|---|---|
| `2026-09-12 21:52` / `2026-09-13 09:30` | 2 | **D** — 早于本会话 |
| `2026-09-27 08:50 – 09:20` | 5 | **C** — 审计修复 + Console 修复 |
| `2026-09-27 10:48 – 10:59` | 26 | **A / B** — F-3B 实现 |
| `2026-09-27 12:25 – 12:36` | 3 | **B** — F-3B 会话内收敛（fixture + 注释） |

### 2.2 逐文件表

| # | 文件 | ± | 类 | 归属 / 依据 |
|---|---|---|---|---|
| 1 | `cmd/node/main.go` | +16/−1 | **A** | 四态判定：0 字节 → CORRUPTED；缺文件 → Uninitialized；改用 `OpenFileBlockStoreStrict` |
| 2 | `cmd/node/cli.go` | +77/−0 | **A** | 新增 `init` 子命令 + 帮助文本（唯一参数 `-datadir`） |
| 3 | `internal/blockchain/blockchain.go` | +7/−16 | **A** | 移除自动创世（`h<0 → ErrUninitializedStore`）；改用 gate |
| 4 | `internal/blockchain/verify.go` | +7/−9 | **A** | `VerifyStoredChain` 共用同一 gate |
| 5 | `internal/blockchain/genesis.go` | +15/−1 | **A** | `CanonicalGenesisHash` pin + `Hex()`；注释 `20→16` |
| 6 | `internal/storage/file.go` | +23/−9 | **A** | `OpenFileBlockStoreStrict`（`repairOnOpen=false`） |
| 7 | `internal/storage/v2.go` | +1/−1 | **A** | `loadLog` torn-tail：`readOnly \|\| !repairOnOpen` → fail closed |
| 8 | `cmd/node/datalock_integration_test.go` | +2/−2 | **B** | `newNodeRuntime` → `newNodeRuntimeForTest` |
| 9 | `cmd/node/fullstack_test.go` | +2/−2 | **B** | 同上 |
| 10 | `cmd/node/p2p_branch_test.go` | +1/−1 | **B** | 同上 |
| 11 | `cmd/node/r3_crash_restart_test.go` | +4/−4 | **B** | gofmt 对齐（注释列） |
| 12 | `cmd/node/r4b_dual_reorg_convergence_test.go` | +2/−2 | **B** | gofmt 对齐 |
| 13 | `cmd/node/reset_test.go` | +5/−5 | **B** | `NewBlockchainFromStore` → `…ForTest` |
| 14 | `cmd/node/seed_parallel_test.go` | +2/−2 | **B** | `newNodeRuntime` → `…ForTest` |
| 15 | `cmd/node/verify_test.go` | +1/−1 | **B** | `NewBlockchainFromStore` → `…ForTest` |
| 16 | `internal/blockchain/blockchain_test.go` | +2/−2 | **B** | 同上（2 处） |
| 17 | `internal/blockchain/c1_hashindex_test.go` | +1/−1 | **B** | 同上 |
| 18 | `internal/blockchain/c1_noninterference_dump_test.go` | +2/−2 | **B** | import 排序 + 改名 |
| 19 | `internal/blockchain/cd_canonset_test.go` | +1/−1 | **B** | 改名 |
| 20 | `internal/blockchain/difficulty_consensus_test.go` | +2/−2 | **B** | 改名（2 处） |
| 21 | `internal/blockchain/obs_noninterference_test.go` | +2/−2 | **B** | 改名（2 处） |
| 22 | `internal/blockchain/reorg_test.go` | +58/−58 | **B** | 改名（多处）+ struct gofmt 对齐 |
| 23 | `internal/blockchain/replay_persistence_test.go` | +2/−2 | **B** | 改名（2 处） |
| 24 | `internal/mempool/reorg_resurrection_test.go` | +1/−1 | **B** | 改名 |
| 25 | `internal/storage/r3_crash_matrix_test.go` | +16/−16 | **B** | gofmt 对齐 |
| 26 | `internal/storage/r4a_legacy_length_matrix_test.go` | +22/−22 | **B** | gofmt 对齐 |
| 27 | `internal/storage/v2_test.go` | +1/−1 | **B** | 改名 |
| 28 | `cmd/node/stop_lifecycle_test.go` | +25/−0 | **B** | 新增 `ensureTestDataDir` + 接入 `startRealNode` |
| 29 | `cmd/node/lock_lifecycle_test.go` | +7/−4 | **B** | 改名 + `ensureTestDataDir` 接入 `stale_lock_file_is_taken_over` |
| 30 | `internal/control/server.go` | +59/−0 | **C** | CONSOLE-MINE-AUTH-FIX-1：`/console/mine` + `consoleOriginGate` |
| 31 | `internal/control/console_test.go` | +5/−2 | **C** | 正则同步为 `/console/mine` |
| 32 | `internal/control/web/console.html` | +6/−2 | **C** | 改用 `/console/mine`；修正可用性文案 |
| 33 | `internal/explorer/explorer_test.go` | +12/−7 | **C** | 301 → 307（Go 1.22+ ServeMux 行为） |
| 34 | `cmd/node/r1_boundary_test.go` | +10/−2 | **C** | O(N²) 挂死修复（`Size()` 内层调用消除） |
| 35 | `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` | +88/−4 | **D** | 追加 §9.11（「PHASE V2.0 补录」）；**无本链授权记录** |
| 36 | `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` | +470/−220 | **D** | 内容被**整体替换**为「PHASE BRAND-0D.3」报告，含**外来 HEAD `535edb71…`**；**无本链授权记录** |

**小计**：A=7 · B=22 · C=5 · D=2 · **E=1（不计入 36）** · **G=0**

### 2.3 E 类（stat-only，不计入 36）

| 文件 | 证据 |
|---|---|
| `internal/blockchain/query.go` | `git diff --quiet` → exit **0**；`git diff --numstat` → **空**；仅在 CRLF 警告列表出现 ⇒ **零内容差异** |

---

## §3 — 未跟踪文件分类（F / 其他）

### 3.1 F 类 —— **F-3B 新源文件（必须纳入提交才能编译/测试）**

| 文件 | mtime | 说明 |
|---|---|---|
| `internal/blockchain/identity.go` | 09-27 10:51 | canonical identity gate（`VerifyGenesisIdentity` / `InitializeBlockchainStore` / `NewBlockchainFromStoreForTest` / 5 个哨兵错误） |
| `cmd/node/f3b_test_helpers_test.go` | 09-27 10:54 | F-3B 测试夹具 |
| `cmd/node/f3b_identity_test.go` | 09-27 10:57 | `TestF3BExplicitInitializationStateMatrix`（9 子断言） |

### 3.2 相邻 —— Console 修复的新测试

| 文件 | mtime | 说明 |
|---|---|---|
| `internal/control/console_mine_auth_test.go` | 09-27 09:20 | CONSOLE-MINE-AUTH-FIX-1 的 7 个用例 |

### 3.3 **未被跟踪但被编译/测试的完整工作流**（早于本会话）

| 文件 | mtime | 说明 |
|---|---|---|
| `internal/attribution/attribution.go` | 09-22 14:45 | 归因包 |
| `internal/attribution/classify.go` | 09-22 14:45 | 归因分类 |
| `internal/attribution/attribution_test.go` | 09-22 14:45 | 归因测试 |
| `cmd/coinbase-attribution/main.go` | 09-22 14:45 | **唯一** importer（`p2pchain/internal/attribution`） |
| `internal/utxo/f1n1_undo_content_test.go` | 09-20 07:12 | F1-N1 undo 内容契约测试 |
| `internal/storage/f1n2_detached_undo_contract_test.go` | 09-20 07:52 | F1-N2 detached undo 契约测试 |
| `internal/blockchain/f1n2_reorg_detached_undo_test.go` | 09-20 07:55 | F1-N2 reorg detached undo 测试 |
| `internal/storage/f1n1_undo_contract_test.go` | 09-22 14:45 | F1-N1 undo 契约测试 |
| `internal/blockchain/f1n1_reorg_coverage_test.go` | 09-27 10:52 | F1-N1 legacy/v2 reorg 覆盖测试 —— **见 §4（关键依赖）** |

> ⚠️ 这 9 个文件**不在「36 个 tracked 修改」内**，但 `go build ./...` / `go test ./...` **会编译/执行它们**。
> 若提交时遗漏，工作树与提交树**不等价**：`cmd/coinbase-attribution` 包与 F1-N1/F1-N2 测试将**静默消失**。

### 3.4 报告类未跟踪文件（`PHASE-*.md`）

`PHASE-F1` / `F2` / `F3` / `F3A` / `F3B` / `F4` / `F5` 共 **7** 份本链报告；
另有 `PHASE-REORG-1F..1J` 系列 **14** 份（**早于本会话、非本链**）。

### 3.5 产物类未跟踪目录

| 目录 | Git 可见性 | 内容 | 风险 |
|---|---|---|---|
| `f3b-verify/` | 部分（`*.exe` 被忽略；空 lab 目录） | `f3b-node.exe` | 无 |
| `f4-verify/` | **不可见**（仅 `*.exe`） | `f4-node.exe` | 无 |
| `F4-controlled-init-1/` | **不可见**（仅 `*.dat`） | `blocks.dat` 180 B | 无 |
| `F4-legacy-copy-1/` | **不可见**（仅 `*.dat`） | `blocks.dat` 226800 B | 无 |
| `F4-empty-dir-1/` | 空目录 | — | 无 |
| `f5-verify/` | **可见** | `control-token`(**凭据**) + `f5-node.exe` + 2 个 `.log` | 🔴 **见 §5** |
| `F5-real-node-validation-1/` | **不可见**（`*.dat` + `wallet.json`） | `blocks.dat` 720 B + `wallet.json` | 低（测试钱包） |
| `F5-legacy-copy-1/` | **不可见**（仅 `*.dat`） | `blocks.dat` | 无 |
| `audit-run/` | **可见** | `control-token`(**凭据**) + 日志 + data 目录 | 🔴 **见 §5** |
| `gui-test/` | **可见** | `token`(**凭据**) | 🔴 **见 §5** |

---

## §4 — 关键依赖分析：**「36 个 tracked 修改」不足以产出可编译的树**

### 4.1 事实链

1. `internal/blockchain/p1_reorg_fail_before_commit_test.go` 是 **tracked 且未修改** 的文件；
   `git cat-file -e HEAD:internal/blockchain/p1_reorg_fail_before_commit_test.go` → **PRESENT in HEAD**。
2. 该文件第 63 行**作为代码**调用 `f1n1UTXOSig(bc)`。
3. `f1n1UTXOSig` 的**唯一**定义在 `internal/blockchain/f1n1_reorg_coverage_test.go:134`。
4. 该定义文件 **不在 HEAD**：`git cat-file -e HEAD:internal/blockchain/f1n1_reorg_coverage_test.go`
   → `fatal: path … exists on disk, but not in 'HEAD'`。

### 4.2 实测（pristine HEAD 抽取，`git archive HEAD`，仓库零改动）

```
$ git archive HEAD | tar -x -C <tmp>
$ go vet ./internal/blockchain/
# p2pchain/internal/blockchain [p2pchain/internal/blockchain.test]
internal\blockchain\p1_reorg_fail_before_commit_test.go:63:19: undefined: f1n1UTXOSig
vet-exit=1
```

⇒ **HEAD 的 `internal/blockchain` 测试包无法编译。**
（非测试构建 `go build ./...` 于 pristine HEAD → **exit 0**；破坏**仅在测试编译单元**。）

### 4.3 修复的**最小必要集合**（实测）

注入以下文件后 `go vet ./internal/blockchain/` → **exit 0**：

- `internal/blockchain/identity.go`（**F 类，未跟踪**）
- `internal/blockchain/f1n1_reorg_coverage_test.go`（**未跟踪**）
- `internal/blockchain/blockchain.go` / `genesis.go` / `verify.go`（A 类，tracked 修改）

> 中间态证据：仅注入 `f1n1_reorg_coverage_test.go` 时，下一个错误为
> `f1n1_reorg_coverage_test.go:60:13: undefined: NewBlockchainFromStoreForTest`
> ⇒ 该未跟踪测试文件**又依赖** F-3B 的 `identity.go`。

### 4.4 结论

**任何「只提交 36 个 tracked 修改」的方案都会产出一棵 `go test ./...` 无法编译的树。**
提交候选集**必须**包含 §3.1（F 类）与 `internal/blockchain/f1n1_reorg_coverage_test.go`。

---

## §5 — 提交边界与风险（Commit Boundary）

### 5.1 🔴 凭据泄漏风险（HIGH）

`.gitignore` 覆盖 `*.exe` / `*.dat` / `*.lock` / `wallet.json` / `/run-a/` / `/run-b/`，**未覆盖 token**。
实测 `git check-ignore`：

| 路径 | 结果 |
|---|---|
| `secrets/control-token` | **NOT-IGNORED** |
| `f5-verify/control-token` | **NOT-IGNORED** |
| `audit-run/control-token` | **NOT-IGNORED** |
| `gui-test/token` | **NOT-IGNORED** |
| `run-a/wallet.json` | IGNORED ✅ |
| `node.lock` | IGNORED ✅ |

⇒ 任何 `git add -A` / `git add .` / `git commit -a` 都会**暂存 3 个凭据文件**。
**提交必须使用显式路径白名单，禁止 `-A` / `.` / `-a`。**

### 5.2 🔴 越界修改风险（MEDIUM）

D 类 2 个文件是**早于本会话**的历史工作树改动，其中
`docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` 的内容已被**整体替换**为另一阶段
（PHASE BRAND-0D.3，HEAD `535edb71…`）的报告 ⇒ 若被提交，会把**外来阶段内容**并入本链历史。

### 5.3 ⚠️ 未跟踪工作流丢失风险（MEDIUM）

§3.3 的 9 个文件（attribution 包 + F1-N1/F1-N2 测试）被编译/测试但未跟踪。
遗漏提交 ⇒ 提交树与工作树不等价（`cmd/coinbase-attribution` 包消失、F1-N1/F1-N2 测试消失）。

### 5.4 ⚠️ 空白字符检查失败（LOW）

`git diff --check` → **exit 2**，全部命中 `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md`
（trailing whitespace × ~130 + EOF 空行）。即 **D 类文件**。剔除 D 类后该项自然消解。

### 5.5 🔴 规定测试失败（HIGH）

`go test -count=1 ./...` → **EXIT 1**。失败用例 `TestF1N1_C_MixedLegacyV2Reorg`
位于**未跟踪文件** `internal/blockchain/f1n1_reorg_coverage_test.go`，
实测为**负载敏感 flaky**（隔离 3/3 PASS、整包 2/2 PASS、全套并行 1/1 FAIL）。

**悖论**：该文件因 §4 的编译依赖而**必须**纳入提交（否则树不可编译），
但纳入后又带入一个**不稳定测试**。⇒ 提交前须先修复或隔离该用例（见主报告 §12.2 前置条件 4）。

---

## §6 — 建议的提交候选集（**仅登记，不执行**）

### 6.1 必须包含（INCLUDE）

| 组 | 内容 | 数量 |
|---|---|---|
| A | F-3B 生产代码（§2.2 #1–7） | 7 |
| B | F-3B 测试夹具（§2.2 #8–29） | 22 |
| F | F-3B 新源文件（§3.1） | 3 |
| — | `internal/blockchain/f1n1_reorg_coverage_test.go`（编译依赖，§4） | 1 |

### 6.2 须 Owner 决策（DECIDE）

| 组 | 内容 | 数量 | 备注 |
|---|---|---|---|
| C | Console 修复 + 审计修复（§2.2 #30–34 + §3.2） | 6 | 同会话、独立授权；**安全边界相邻**，建议单独审核 |
| — | §3.3 未跟踪工作流（attribution + F1-N1/F1-N2 测试） | 9 | 是否纳入本链提交，须 Owner 决定 |
| — | 报告（F-1..F-5 共 7 份 + 本 F-6 三件套） | 10 | 建议单独 docs 提交 |

### 6.3 必须排除（EXCLUDE）

| 内容 | 理由 |
|---|---|
| `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` | D 类，无本链授权记录 |
| `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` | D 类，内容被外来阶段替换 |
| `internal/blockchain/query.go` | E 类，零内容差异 |
| `audit-run/`、`f5-verify/`、`gui-test/` | 🔴 含未忽略凭据 |
| `PHASE-REORG-1F..1J` 系列 14 份报告 | 非本链 |
| `f3b-verify/`、`f4-verify/`、`F4-*`、`F5-*` | 验证产物，不应入库 |

---

## §7 — 本清单的边界声明

- 本清单**未执行**任何 `git add` / `commit` / `push` / `reset` / `checkout` / `clean` / `restore` /
  `rebase` / `merge` / `amend` / `squash`。
- 本清单**未修改**任何被审计文件（含 `.gitignore`）。
- `git archive` 抽取仅写入**系统临时目录**（`$TEMP`），**仓库零改动**，抽取物已删除。
- 提交集合的**最终决定权在 Owner**；本清单只提供分类与依赖证据。

**HARD STOP。**
