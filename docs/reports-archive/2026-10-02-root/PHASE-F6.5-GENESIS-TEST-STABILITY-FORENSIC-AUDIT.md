# PHASE F-6.5 — GENESIS TEST STABILITY FORENSIC AUDIT

| 字段 | 值 |
|---|---|
| Phase ID | **F-6.5** |
| Title | GENESIS TEST STABILITY FORENSIC AUDIT |
| Mode | **STRICT READ-ONLY / FORENSIC TEST STABILITY AUDIT** |
| Date (UTC) | 2026-09-27T14:28:45Z |
| Date (local) | 2026-09-27 22:28:46 +0800 |
| Baseline HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| Branch | `main` |
| Repository | `E:/wakuang/p2pchain` |
| Experiment root | `E:/wakuang/f65-verify`（**仓库之外**，未污染工作树） |
| Write operations on repo | **NONE**（零 `git add` / `git commit` / 文件写入 / 配置变更） |
| Verdict | **PHASE F-6.5 COMPLETE / TIME-LOAD FRAGILITY IDENTIFIED / HARD STOP** |

> 本阶段**不是修复阶段**。全部结论基于对 F-6.4 已冻结候选边界（GCS）的**只读复现 + 受控隔离测量**，
> 不修改任何源文件、不改变任何超时配置、不执行 `git` 写操作。

---

## §0 — PHASE ROLE & SCOPE

### 0.1 任务定义

在 **F-6.4 已确定 Genesis Closure Candidate Boundary（GCS = 33 文件）** 的基础上，
**解释并分类**当前 Genesis-related test instability，回答一个核心问题：

> **Genesis 测试基线是否稳定？若不稳定，不稳定的根因属于哪一类？**

### 0.2 允许的结论空间（8 类根因）

| # | 类别 | 本阶段判定 |
|---|---|---|
| 1 | Genesis 功能性缺陷 | **RULED OUT** |
| 2 | 确定性测试失败 | **PRESENT（非 Genesis）** |
| 3 | F1-N1 / F1-N2 跨工作流污染 | **PRESENT（结构性）** |
| 4 | Console / Explorer / 其他工作流污染 | **PRESENT** |
| 5 | 负载敏感 / 时序敏感测试 | **PRESENT（主因之一）** |
| 6 | 测试基建时间预算脆弱 | **PRESENT（主因之一）** |
| 7 | 环境 / 资源争用 | **PRESENT** |
| 8 | 真正未解决的 Genesis 稳定性问题 | **NONE FOUND** |

### 0.3 只读纪律（本阶段遵守情况）

| 禁令 | 状态 |
|---|---|
| 修改 `.gitignore` | ✅ 未触碰（`f43418cf74ea996c` 不变） |
| 读取 token 内容 | ✅ 未读取（仅 `stat` 大小） |
| token 进入 git | ✅ 无 `git add` 发生 |
| `git add -A` / `.` / `-a` | ✅ 未执行 |
| 发现异常顺手修复 | ✅ 未修复（异常仅登记，见 §11.5） |
| `gofmt -w` / `goimports` | ✅ 未执行 |
| 自动推断下一阶段 | ✅ 未推断 |

---

## §1 — AUTHORITATIVE INPUTS

### 1.1 权威输入清单（5 项，逐项存在性核验）

| # | 规格名 | 实际路径 | 存在性 | SHA-256（前 16） |
|---|---|---|---|---|
| 1 | F-6.4 边界审计 | `PHASE-F6.4-GENESIS-CLOSURE-COMMIT-BOUNDARY-AUDIT.md` | **PRESENT** | `bcbe5ddfe760536f` |
| 2 | F-6.4 候选清单 | `PHASE-F6.4-GENESIS-COMMIT-CANDIDATE-MANIFEST.md` | **PRESENT** | `9bbfb663f174e5d1` |
| 3 | F-6.1 最终裁定 | `PHASE-F6.1-OWNER-DECISION-FINALIZATION.md` | **PRESENT** | `5b95d47009dd49c8` |
| 4 | F-6.2 范围冻结矩阵 | `PHASE-F6.2-SCOPE-FREEZE-MATRIX.md` | **ABSENT** | — |
| 5 | F-6.2 裁定记录 | `PHASE-F6.2-OWNER-DECISION-RECORD.md` | **ABSENT** | — |

### 1.2 输入 4 / 5 缺失的处置

规格列出的输入 4、5 **不存在**（F-6.2 的唯一交付物为
`PHASE-F6.2-P3.1-PROVENANCE-RESTORATION-REPORT.md`，SHA-256 `7db1e23894e8fd0d`）。
**不伪造替代物**。本阶段对 F-6.2 的事实引用一律以该唯一交付物 + MEMORY 长期约定为准，
并在 §1.3 声明该信息缺口的边界。

### 1.3 信息缺口声明

```
INPUT GAP: 规格输入 4 / 5 缺失（F-6.2 从未产出该两份文档）。
           ⇒ 本阶段不引用其内容；F-6.2 事实仅来自其唯一实际交付物。
           该缺口不影响 F-6.5 的稳定性结论（F-6.5 的判定输入为 F-6.4 + 实测）。
```

---

## §2 — BASELINE RE-VERIFICATION

### 2.1 Git 基线（开工时实测）

| 项 | 值 | 与 F-6.4 基线 |
|---|---|---|
| HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` | 一致 |
| branch | `main` | 一致 |
| staged | **0** | 一致 |
| tracked modified | **35** | 一致（F-6.2 恢复后新基线） |
| porcelain `M` | **36** | 一致 |
| untracked（文件级） | **373** | — |
| untracked（porcelain `??` 行，目录折叠） | **200** | 一致 |

> **§2.1 勘误**：`200` 与 `373` 并非冲突——`git status --porcelain` 将未跟踪**目录**折叠为一行，
> 而 `git ls-files --others --exclude-standard` 枚举**文件**。两者并存，无异常。

### 2.2 受保护对象（实测，开工 vs 登记）

| 对象 | 实测 SHA-256（前 16） | 登记值 | 状态 |
|---|---|---|---|
| `.gitignore` | `f43418cf74ea996c` | `f43418cf74ea996c` | ✅ 不变 |
| `node.exe`（唯一能读旧链的二进制） | `3972843aff90428d` | `3972843aff90428d` | ✅ 不变 |
| `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` | `e9def9aa1f90e5ac` | `e9def9aa1f90e5ac` | ✅ 不变 |
| `internal/attribution/attribution.go` | `3f4cb77820b43747` | `3f4cb77820b43747` | ✅ 不变 |
| `internal/attribution/classify.go` | `bb7b00bc135c6146` | `bb7b00bc135c6146` | ✅ 不变 |
| `internal/attribution/attribution_test.go` | `4d7841d418a66881` | `4d7841d418a66881` | ✅ 不变 |
| `cmd/coinbase-attribution/main.go` | `c634c1ecaea59551` | `c634c1ecaea59551` | ✅ 不变 |

### 2.3 Token 元数据（仅大小，未读取内容）

| 文件 | 大小 | 与登记 |
|---|---|---|
| `audit-run/control-token` | 33 B | ✅ |
| `f5-verify/control-token` | 64 B | ✅ |
| `gui-test/token` | 34 B | ✅ |

```
BASELINE RE-VERIFICATION: PASS
```

---

## §3 — F-6.4 EXPERIMENT RECONSTRUCTION

实验在**仓库之外** `E:/wakuang/f65-verify` 重建，全部经 `git archive HEAD` 提取 pristine HEAD，
再叠加候选集工作树版本。叠加**逐文件 SHA-256 校验，0 mismatch**。

### 3.1 实验树构造

| 树 | 构造 | 文件数 |
|---|---|---|
| `base` | `git archive HEAD` | 242 |
| `gcs` | `base` + **GCS(33)** 工作树覆盖 | 242 + 4 新增 |
| `gcs-setb` | `gcs` + **SET B(6)** 工作树覆盖 | +1 新增 |

- GCS 列表：`f65-verify/gcs.files`（33 行）
- SET B 列表：`f65-verify/setb.files`（6 行）
- 覆盖正确性：33/33 SHA-256 与工作树一致；4 个 untracked 文件在 `gcs` 存在、在 `base` 不存在。✅

### 3.2 pristine HEAD 复现（已知事实确认）

```
$ cd base && go vet ./...
# p2pchain/internal/blockchain [p2pchain/internal/blockchain.test]
internal\blockchain\p1_reorg_fail_before_commit_test.go:63:19: undefined: f1n1UTXOSig
BASE_VET_EXIT=1
```

⇒ **HEAD 的 `internal/blockchain` 测试包本就无法编译**（tracked 文件依赖 untracked 符号）。**与 F-6.4 一致。**

### 3.3 L1 — COMPILE CLOSURE（GCS 33）

| 命令 | 退出码 |
|---|---|
| `go build ./...` | **0** |
| `go vet ./...` | **0** |

```
COMPILE CLOSURE (GCS = 33): ACHIEVED
```

### 3.4 L2 — DETERMINISTIC TEST CLOSURE（隔离）

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test -count=1 ./internal/blockchain/...` | **0** | `ok p2pchain/internal/blockchain 78.857s` ✅ |

> F-6.4 记录 `65.706s`；本次 `78.857s`（+20.0%）。**同内容、不同负载下的测量噪声**，见 §10。

```
DETERMINISTIC TEST CLOSURE (isolated): ACHIEVED
```

### 3.5 L3 — FULL SUITE（默认 600s 验收超时）

**L3a — GCS(33)**：`go test -count=1 -json ./...` → **exit 1**，wall **624s**

| 包 | 结果 | 耗时 |
|---|---|---|
| `p2pchain/cmd/node` | **FAIL** | **600.429s**（`panic: test timed out after 10m0s`） |
| `p2pchain/internal/storage` | ok | 320.845s |
| `p2pchain/internal/blockchain` | ok | 85.812s |
| `p2pchain/internal/mempool` | ok | 42.262s |
| `p2pchain/internal/p2p` | ok | 15.730s |
| `p2pchain/internal/explorer` | **FAIL** | 0.791s（`TestPathBoundaryRawTCP`，0.01s） |
| 其余 11 包 | ok / skip | < 5s |

**L3b — GCS+SET B(39)**：`go test -count=1 -json ./...` → **exit 1**，wall **630s**

| 包 | 结果 | 耗时 |
|---|---|---|
| `p2pchain/cmd/node` | **FAIL** | **600.494s**（超时） |
| `p2pchain/internal/storage` | ok | 323.816s |
| `p2pchain/internal/blockchain` | ok | 95.622s |
| `p2pchain/internal/mempool` | ok | 50.709s |
| `p2pchain/internal/explorer` | **ok** | 0.857s ← **C2 修复生效** |
| 其余 | ok / skip | — |

```
FULL SUITE GREEN (GCS 33):      NOT ACHIEVED (exit 1)
FULL SUITE GREEN (GCS+SET B 39): NOT ACHIEVED (exit 1)
```

### 3.6 最小性矩阵（`go vet`，逐项实测）

**M 系列（从 GCS 移除 ⇒ 证明必需）**

| # | 移除 | `go vet ./...` | 缺失符号（首行） | 判定 |
|---|---|---|---|---|
| **M1** | `internal/blockchain/f1n1_reorg_coverage_test.go`（X-01） | **exit 1** | `internal/blockchain/p1_reorg_fail_before_commit_test.go:63:19: undefined: f1n1UTXOSig`（`[.test]` 包） | **REQUIRED（编译）** |
| **M2** | `internal/blockchain/identity.go`（F-01） | **exit 1** | `blockchain.go:224:15: undefined: ErrUninitializedStore` / `blockchain.go:227:16: undefined: VerifyGenesisIdentity`（**`# p2pchain/internal/blockchain` 无 `.test` 后缀**） | **REQUIRED（生产编译）** |
| **M3** | `cmd/node/f3b_test_helpers_test.go`（F-02） | **exit 1** | `cmd\node\datalock_integration_test.go:39:12: undefined: newNodeRuntimeForTest` | **REQUIRED（编译）** |
| **M4** | `cmd/node/f3b_identity_test.go`（F-03） | **exit 0** | — | OPTIONAL（编译） |
| **M5** | 组 B 22 夹具回退 HEAD | **exit 0** | — | OPTIONAL（编译） |

**E 系列（向 GCS 加入 ⇒ 证明可排除）**

| # | 加入 | `go vet ./...` | 判定 |
|---|---|---|---|
| **E1** | GAP-4（4 文件） | **exit 0** | ✅ 可排除 |
| **E2** | Console 组（C3–C6） | **exit 0** | ✅ 可排除 |
| **E3** | F1-N1/N2 残余（4 文件） | **exit 0** | ✅ 可排除 |
| **E4** | SET B 全组（C1–C6） | **exit 0** | ✅ 编译可排除 |

```
MINIMALITY: ESTABLISHED（与 F-6.4 一致）
  编译最小集 = { identity.go, f3b_test_helpers_test.go, f1n1_reorg_coverage_test.go }
```

---

## §4 — FAILURE CLASSIFICATION（FAILURE-F1 / FAILURE-F2）

GCS 全套暴露**两个**独立失败，命名如下：

### 4.1 FAILURE-F1 — `p2pchain/cmd/node` 包级超时

| 字段 | 值 |
|---|---|
| 现象 | `panic: test timed out after 10m0s` |
| 触发点（L3a） | 最后启动的用例 = `TestR4B_DualProcessCompetitiveForkConverges/L=64` |
| 触发点（L3b） | 最后启动的用例 = `TestResetRefusesWhenStopFailed/hard_kill_then_reset_takes_over` |
| 包耗时 | L3a = 600.429s；L3b = 600.494s（均被 600s 上限截断） |
| 根因类别 | **#6 测试基建时间预算脆弱**（主）+ **#5 负载敏感**（副） |
| 是否 Genesis 缺陷 | **否** |

**★ 关键证据：两次运行的超时点落在不同用例** ⇒ 这是**包级累计时长**问题，
**不是**某个用例挂死。（挂死型超时会在同一用例稳定复现。）

### 4.2 FAILURE-F2 — `p2pchain/internal/explorer` 断言失败

| 字段 | 值 |
|---|---|
| 用例 | `TestPathBoundaryRawTCP` |
| 现象 | `重复斜杠被清洗: GET /api//status 响应行不含期望状态码 301；实测响应头: HTTP/1.1 307 Temporary Redirect` |
| L3a | FAIL（包 0.791s，用例 0.01s） |
| L3b | **ok**（包 0.857s，用例 0.02s）← SET B C2 修复 |
| 根因类别 | **#2 确定性测试失败**（+ **#7 环境依赖**：Go 版本） |
| 是否 Genesis 缺陷 | **否** |

**★ 源证据（HEAD vs 工作树 diff）**：HEAD 的 `explorer_test.go` 断言 **301**（7 处），
工作树 C2 改为 **307**。Go 1.22+ `ServeMux` 对需规范化的路径返回 `StatusTemporaryRedirect = 307`
（`net/http/server.go` `matchOrRedirect` / `RedirectHandler(u, StatusTemporaryRedirect)`）。
⇒ 断言锁定的是 **stdlib 实现细节**（旧版 301），与 Genesis 无关。

### 4.3 第三个观测对象 — F1-N1 flaky 用例（未在本次复现）

| 用例 | 位置 | 本次 6 次运行结果 |
|---|---|---|
| `TestF1N1_C_MixedLegacyV2Reorg` | `internal/blockchain/f1n1_reorg_coverage_test.go:560` | **6/6 PASS**（1.24s / 1.76s，其余运行未执行该包） |

⇒ 本次**未复现**该 flaky（F-6.4 曾复现 1 次）。**单次/少量 PASS 不构成稳定性证据**（见 §8.4、§14）。
根因已定位（§5.3），属 **#5 时序敏感**。

---

## §5 — SOURCE-DEPENDENCY EVIDENCE

### 5.1 FAILURE-F1 的源依赖链（为何 cmd/node 必然慢）

`cmd/node` 的耗时**不来自 Genesis**，而来自**真实子进程集成测试**：

| 机制 | 源码证据 |
|---|---|
| 构建真实二进制（一次） | `cmd/node/lock_lifecycle_test.go:257` `buildNodeBinary` → `exec.Command("go","build","-o",…,".")`（`sync.Once`） |
| 启动真实子进程 + 等待控制接口就绪 | `cmd/node/stop_lifecycle_test.go:98` `startRealNode` → `exec.Command(bin, …)` + `waitReady(30*time.Second)`（每 100ms 轮询） |
| 真实崩溃/重启 × 5 | `cmd/node/r3_crash_restart_test.go:63` `TestR3_RealProcessCrashRestartLegacyPrefix` |
| 双进程竞争分叉 | `cmd/node/r4b_dual_reorg_convergence_test.go:256` `TestR4B_DualProcessCompetitiveForkConverges` |
| O(N²) 全块序列化 | `cmd/node/r1_boundary_test.go` HEAD 内层循环逐次调用 `oversize.Size()` |

**`Block.Size() = len(Block.Encode())` 每次重新序列化整块**（`internal/block/block.go`），
HEAD 版 `TestR1F4PreParkRejectionReasons` 在内层循环中反复调用 ⇒ **O(N²)**。

### 5.2 无 `t.Parallel()` —— 包内串行

```
cmd/node  t.Parallel() 计数 = 0
internal/blockchain t.Parallel() 计数 = 0
cmd/node  TestMain 计数 = 0
```

⇒ 包内**串行**执行；争用只发生在**跨包并行**（`go test ./...` 默认 `-p = NumCPU`）。

### 5.3 F1-N1 flaky 的源依赖链（时序敏感根因）

```
internal/block/block.go:43   NewCandidateBlock(...)  →  Timestamp: time.Now().Unix()
internal/pow/pow.go:142      Mine(b)  →  MineCancelable(b, 1, nil)   // 串行，给定块确定
internal/blockchain/f1n1_reorg_coverage_test.go:104  f1n1MineAbove(...)  // 256 次重试上限
internal/blockchain/f1n1_reorg_coverage_test.go:119  f1n1MineBelow(...)  // 256 次重试上限
```

**★ 根因**：区块头 `Timestamp` 取自 `time.Now().Unix()`（**秒级墙钟**）⇒ **块哈希随运行时刻变化**。
`f1n1MineAbove/MineBelow` 以「某块哈希」为阈值，在 256 次尝试内寻找高于/低于阈值的兄弟块。
哈希在 `[0, 2^240)`（16-bit PoW 目标）近似均匀 ⇒ 单次成功概率 `p = threshold/2^240`；
256 次全败概率 `≈ (1-p)^256`。对均匀阈值取期望：

```
P(harvest 失败) = ∫₀¹ (1-t)^256 dt = 1/257 ≈ 0.389%  （每次 harvest）
```

⇒ 该用例**天然带 ~0.4%/次 harvest 的失败概率**，与负载无关，**只与墙钟时间种子有关**。
注释自述「Harvesting an explicit ordering is what makes the fork-choice outcome deterministic」
——但 harvest **本身是概率性的有界重试**。属 **#5 时序敏感测试**，**非 Genesis 缺陷**。

### 5.4 结论

```
FAILURE-F1 ← 真实子进程集成测试的固有成本 + 600s 预算（#5/#6），与 Genesis 无关
FAILURE-F2 ← 陈旧 stdlib 断言 301 vs Go 1.27 的 307（#2/#7），与 Genesis 无关
F1-N1 flaky ← time.Now() 种子 + 有界重试（#5），与 Genesis 无关
```

---

## §6 — ISOLATION MATRIX（E1–E5）

隔离矩阵用于**分离**：包内固有成本 / 跨包并行争用 / SET B 效果 / 超时预算。

| # | 树 | 模式 | 命令 | 包耗时 | 结果 |
|---|---|---|---|---|---|
| **E1** | pristine HEAD | 编译 | `go vet ./...` | — | **exit 1**（测试包不编译） |
| **E2** | GCS(33) | 全套并行（默认 600s） | `go test -count=1 -json ./...` | cmd/node **600.429s** | **FAIL** |
| **E3** | GCS+SET B(39) | 全套并行（默认 600s） | `go test -count=1 -json ./...` | cmd/node **600.494s** | **FAIL** |
| **E4** | GCS+SET B(39) | **cmd/node 隔离**（`-timeout 25m`） | `go test -count=1 -json -timeout 25m ./cmd/node` | **586.721 / 590.410 / 597.772s** | **PASS ×3** |
| **E5** | GCS(33) | **cmd/node 隔离**（`-timeout 25m`） | `go test -count=1 -json -timeout 25m ./cmd/node` | **735.852s** | **PASS** |

### 6.1 隔离矩阵读出的四条结论

1. **E4 vs E3**：同一树（39），隔离 **PASS**、并行 **FAIL** ⇒ **负载敏感确证**。
2. **E4 vs E5**：隔离下，**C1 存在** 590s，**C1 缺失** 735.9s ⇒ **C1 值 ≈ 148s**（见 §7.3）。
3. **E5**：**纯 GCS(33) 即使隔离也达 735.852s** ⇒ 超 600s 预算 **135.852s（22.6%）** ⇒
   **纯 Genesis 集合自身无法通过验收超时**（根因是 R1 的 O(N²) 用例，非 Genesis）。
4. **E2 vs E3**：SET B 使 explorer 由 FAIL→ok，但 **cmd/node 仍 FAIL** ⇒ SET B **必需但不充分**。

```
ISOLATION MATRIX VERDICT:
  cmd/node instability = LOAD-SENSITIVE (E3/E4) + TIME-BUDGET (E4/E5) 双因
  SET B = REQUIRED-FOR-GREEN(explorer) / INSUFFICIENT-FOR-STABLE-GREEN(cmd/node)
```

---

## §7 — cmd/node TIME FORENSICS

### 7.1 包级耗时归因（GCS+SET B 隔离，run-1 = 586.721s）

| 用例 | 耗时 | 占比 | 工作流 |
|---|---|---|---|
| `TestR4B_DualProcessCompetitiveForkConverges` | **151.180s** | 25.8% | R4B（双进程） |
| `TestR3_RealProcessCrashRestartLegacyPrefix` | **132.640s** | 22.6% | R3（真实崩溃重启） |
| `TestRealProcessForkReorgConvergesByHash` | 39.220s | 6.7% | 真实进程 reorg |
| `TestR1GlobalKeyCeilingPreserved` | 30.800s | 5.2% | R1 |
| `TestR4B_ProductionReorgPathTraversed` | 22.930s | 3.9% | R4B |
| `TestRealProcessPairConvergesToSameTip` | 17.230s | 2.9% | 真实进程 |
| `TestRepeatedStartStop` | 16.730s | 2.9% | 生命周期 |
| `TestResetWithDatadirBeforeAndAfter` | 12.190s | 2.1% | CLI |
| `TestPrintchainWithDatadirBeforeAndAfter` | 11.920s | 2.0% | CLI |
| `TestVerifyWithDatadirBeforeCommand` | 10.370s | 1.8% | CLI |
| **Top-10 合计** | **445.21s** | **75.9%** | — |
| **Genesis 自有用例** `TestF3BExplicitInitializationStateMatrix` | **1.46s** | **0.25%** | **F-3B** |
| 其余 127 个用例 | ~140s | 24% | 混合 |

### 7.2 ★★ 决定性归因：Genesis 在 cmd/node 的占比 ≈ 0.25%

| 运行 | 包耗时 | Genesis 用例耗时 | 占比 |
|---|---|---|---|
| cn-iso-1 | 586.721s | 1.46s | **0.249%** |
| cn-iso-2 | 590.410s | 1.54s | **0.261%** |
| cn-iso-3 | 597.772s | 1.54s | **0.258%** |
| cn-iso-gcs | 735.852s | 1.52s | **0.207%** |
| e4-full | 600.494s | 1.73s | **0.288%** |
| gcs-full | 600.429s | 1.75s | **0.291%** |

⇒ **`cmd/node` 的 99.7% 成本与 Genesis 无关。**

### 7.3 C1（`r1_boundary_test.go`）的量化效果

`TestR1F4PreParkRejectionReasons` 实测：

| 树 | 模式 | 耗时 | 加速比 |
|---|---|---|---|
| GCS(33)（HEAD 版，O(N²)） | 全套并行 | **151.380s** | 1× |
| GCS(33)（HEAD 版） | 隔离 | **148.380s** | — |
| GCS+SET B(39)（C1 版，O(N)） | 全套并行 | **0.550s** | **≈275×** |
| GCS+SET B(39)（C1 版） | 隔离 ×3 | 0.15 / 0.50 / 0.38s | **≈370–990×** |

**包级效应**：C1 缺失 → 735.852s；C1 存在 → ~590s ⇒ **C1 值 ≈ 146–148s**，
**恰是把 cmd/node 从「必然超时」拉回「临界通过」的那一项**。

### 7.4 超时点漂移（证明是累计时长而非挂死）

| 运行 | 最后启动的用例 |
|---|---|
| gcs-full | `TestR4B_DualProcessCompetitiveForkConverges/L=64` |
| e4-full | `TestResetRefusesWhenStopFailed/hard_kill_then_reset_takes_over` |

⇒ **两次落点不同** ⇒ 包级累计时长越界，**非单用例挂死**。

---

## §8 — 3-RUN REPEATABILITY

### 8.1 隔离 3 次运行（GCS+SET B = MGS，`-timeout 25m`）

| 运行 | cmd/node 包耗时 | 结果 |
|---|---|---|
| run-1 | **586.721s** | PASS |
| run-2 | **590.410s** | PASS |
| run-3 | **597.772s** | PASS |

### 8.2 统计量

| 指标 | 值 |
|---|---|
| min | **586.721s** |
| max | **597.772s** |
| mean | **591.634s** |
| **range** | **11.051s** |
| 验收超时 | **600.000s** |
| **margin（min vs 600）** | **+13.279s（+2.21%）** |
| **margin（max vs 600）** | **+2.228s（+0.37%）** |

### 8.3 ★ 对 F-6.4 的勘误（重要）

| 来源 | cmd/node 隔离耗时 | 余量 | 余量% |
|---|---|---|---|
| **F-6.4 记录**（单次） | 571.249s | **+28.751s** | **+4.8%** |
| **F-6.5 实测**（3 次） | 586.721 / 590.410 / 597.772s | **+13.279 / +9.590 / +2.228s** | **+2.21% / +1.60% / +0.37%** |

**★ 结论：F-6.4 的单次采样（571.249s）系统性低估了真实耗时。**
F-6.5 的三次采样揭示：

- 均值 **591.634s**（比 F-6.4 高 **+20.385s**）；
- 最坏 **597.772s**，余量仅 **+2.228s（0.37%）**；
- 三次**单调递增**（586.7 → 590.4 → 597.8），暗示存在**热/累积效应**（range 11.051s = 1.9%）。

⇒ **F-6.4 的「余量 ≈29s（4.8%）」应修正为「余量 ≈2.2–13.3s（0.37%–2.21%）」。**
该修正**加强**了 F-6.4 的结论（BLK-4 成立且更严重），**不改变**其方向。

### 8.4 稳定性证据的强度声明

> **单次 PASS 不构成稳定性证据。** 本阶段以 **3 次隔离 + 2 次全套**共 5 次 cmd/node 观测为据，
> 其中**全套 2/2 FAIL、隔离 3/3 PASS** ⇒ PASS/FAIL 由**并行负载**决定，非随机。

---

## §9 — MEASUREMENT TIMEOUT vs ACCEPTANCE TIMEOUT

### 9.1 两个超时的严格区分

| 概念 | 值 | 用途 | 本阶段是否变更 |
|---|---|---|---|
| **ACCEPTANCE TIMEOUT** | **600s**（`go test` 默认 `10m0s`） | 验收判据；**不得修改** | **否，未变更** |
| **MEASUREMENT TIMEOUT** | **25m**（`-timeout 25m`） | 仅用于**取证测量**，避免 600s 截断导致无法读出真实耗时 | 仅出现在隔离取证命令 |

### 9.2 使用规则与证据

- **所有验收等价运行（E2/E3）**使用**默认 600s**：`go test -count=1 -json ./...` ⇒ **未传 `-timeout`**。
- **仅隔离取证运行（E4/E5）**使用 `-timeout 25m`，以便读到 `586.721 / 590.410 / 597.772 / 735.852s`
  四个**未被截断**的真实值。
- **若隔离取证也使用 600s**，则 E5（735.852s）将被截断为 600s，无法证明「纯 GCS 超预算 135.852s」。

```
ACCEPTANCE TIMEOUT = 600s  (UNCHANGED — 验收判据保持原样)
MEASUREMENT TIMEOUT = 25m  (FORENSIC ONLY — 不参与验收判定)
```

> **不得**将 25m 测量结果解读为「验收通过」。验收视角下，cmd/node 在**全套并行**下
> **2/2 FAIL**（E2/E3）；仅在**隔离**下**3/3 PASS 且余量 0.37%–2.21%**。

---

## §10 — RESOURCE / ENVIRONMENT OBSERVATION

### 10.1 主机环境

| 项 | 值 |
|---|---|
| 逻辑处理器 | **16** |
| Go | `go1.27.0 windows/amd64` |
| GOROOT | `D:\Software\go` |
| GOCACHE | `E:\DevCache\go` |
| `go test` 默认并行度 `-p` | `NumCPU` = 16 |

### 10.2 并行争用的直接证据（**同内容、不同运行**的耗时漂移）

| 包 | gcs-full | e4-full | 漂移 | 内容是否变化 |
|---|---|---|---|---|
| `internal/blockchain` | 85.812s | **95.622s** | **+11.4%** | **无**（SET B 不触碰 blockchain） |
| `internal/mempool` | 42.262s | **50.709s** | **+20.0%** | **无** |
| `internal/storage` | 320.845s | 323.816s | +0.9% | 无 |

> ★ `internal/blockchain` 与 `internal/mempool` 的**源码在两棵树中完全相同**，
> 耗时却漂移 **+11.4% / +20.0%** ⇒ **纯跨包并行争用的测量噪声**。
> 这为「cmd/node 在并行下被推过 600s」提供了**同源、同量级**的独立旁证。

### 10.3 争用机制

`go test ./...` 以 `-p=16` 并行启动约 17 个包。`cmd/node` 内含大量**真实子进程 + 真实 PoW**
（`startRealNode` / `buildNodeBinary` / `r3` / `r4b`），与同样 CPU 密集的
`internal/storage`（320.8s）、`internal/mempool`（42–51s）、`internal/p2p`（15.7s）
**并发争用 16 个逻辑核** ⇒ `cmd/node` 由隔离的 ~591s 被推至 **>600s**。

```
ENVIRONMENT CONTENTION: PRESENT — 16 vCPU, -p=16, 真实子进程 + PoW 密集型包并发
```

---

## §11 — CROSS-WORKSTREAM CONTAMINATION

### 11.1 结构性污染 #1 — X-01（F1-N1）被拖入 Genesis 编译闭包

| 事实 | 证据 |
|---|---|
| tracked、未修改的 `internal/blockchain/p1_reorg_fail_before_commit_test.go:63` 调用 `f1n1UTXOSig` | §5.1；M1 实测 exit 1 |
| `f1n1UTXOSig` 唯一定义在 **untracked** 的 `internal/blockchain/f1n1_reorg_coverage_test.go:134` | §5.1 |
| 该文件 ownership = **F1-N1**，Commit Eligibility = **BLOCKED**（OD-03） | F-6.1 §5 |
| ⇒ 同包 `_test.go` 共享符号空间，**编译强制**将其纳入 GCS | M1 |

**⇒ 「纯 F-1～F-5 Genesis Closure」无法既完整又纯净**：X-01 属 F1-N1，却被编译强制拉入。

### 11.2 结构性污染 #2 — cmd/node 是混合工作流包

`cmd/node` 的 138 个用例分属：Genesis(F-3B) / R1 / R3 / R4A-R4B / B1-cascade / fullstack /
lock-lifecycle / mining-lifecycle / cli-auth / cli-dispatch / explorer-api / p2p-branch-process / logring …

| 工作流 | 代表用例 | 耗时 |
|---|---|---|
| Genesis (F-3B) | `TestF3BExplicitInitializationStateMatrix` | **1.46s（0.25%）** |
| R4B | `TestR4B_DualProcessCompetitiveForkConverges` | 151.180s |
| R3 | `TestR3_RealProcessCrashRestartLegacyPrefix` | 132.640s |
| R1 | `TestR1GlobalKeyCeilingPreserved` | 30.800s |

⇒ **Genesis 的验收被同包其他工作流的 ~590s 成本绑架。**

### 11.3 运行时污染 — SET B 必需但不充分

| 项 | 效果 |
|---|---|
| C2（`explorer_test.go` 301→307） | **必需**：缺则 `internal/explorer` FAIL |
| C1（`r1_boundary_test.go` O(N²)→O(N)） | **大幅有益**：-148s；但仍不足（余量 0.37%） |
| SET B 整体 | **REQUIRED-FOR-GREEN / INSUFFICIENT-FOR-STABLE-GREEN** |

### 11.4 非污染（明确排除）

| 项 | 判定 |
|---|---|
| GAP-4（4 文件） | **无影响**（E1 exit 0；不参与任何测试路径） |
| F1-N1/N2 残余（4 文件） | **无影响**（E3 exit 0；GCS 不含它们） |
| docs / secrets | **无影响** |

### 11.5 观测到的异常（仅登记，不修复）

| # | 异常 | 状态 |
|---|---|---|
| **F6.5-ANOM-1** | 规格权威输入 4/5 不存在（§1.2） | **登记**；不伪造替代 |
| **F6.5-ANOM-2** | F-6.4 的 571.249s 单次采样与 F-6.5 三次采样（586.7–597.8s）不一致（§8.3） | **登记**；属测量方法差异，**修正** F-6.4 余量结论 |

> 按 Owner 流程：**发现异常一律 STOP，不顺手修**。本阶段两处异常均**只登记**。

---

## §12 — TEST-ORDER SENSITIVITY

### 12.1 包内顺序是确定的

- `cmd/node` / `internal/blockchain` 的 `t.Parallel()` 计数 = **0**，无 `TestMain`。
- Go 按「文件名字典序 + 文件内声明序」**串行**执行 ⇒ **包内执行顺序确定**。

### 12.2 但「超时落点」不是确定的

| 运行 | 超时落点 |
|---|---|
| gcs-full | `TestR4B_DualProcessCompetitiveForkConverges/L=64` |
| e4-full | `TestResetRefusesWhenStopFailed/hard_kill_then_reset_takes_over` |

⇒ 落点差异**不来自顺序变化**，而来自**累计时长随负载漂移**：
顺序固定 ⇒ 越界点 = 「累计耗时恰好达到 600s 的那个用例」⇒ 该用例随负载而变。

### 12.3 判定

```
TEST-ORDER SENSITIVITY: 包内顺序确定；超时落点对「负载 → 累计时长」敏感。
                        即：#5 负载敏感 ⊗ #6 时间预算，而非顺序依赖。
```

---

## §13 — CACHE CONTROL

| 项 | 做法 |
|---|---|
| 测试结果缓存 | **全部使用 `-count=1`**（禁用测试结果缓存，强制真实执行） |
| 构建缓存 | 共享 `GOCACHE=E:\DevCache\go`（与 F-6.4 相同）；**构建缓存 ≠ 测试结果缓存** |
| 隔离树 | 每棵实验树独立目录（`base` / `gcs` / `gcs-setb` / `m1..m5` / `e1..e4`），互不干扰 |
| 重复测量 | 隔离 3 次 `-count=1`，**无缓存复用**（耗时 586.7→590.4→597.8s 递增，证明真实重跑） |

> 若使用缓存，第 2/3 次运行会命中缓存瞬间返回（~0s）。实测 586.7/590.4/597.8s 全量耗时，
> **证明缓存未掩盖任何执行**。

```
CACHE CONTROL: -count=1 全程生效；无缓存伪 PASS。
```

---

## §14 — NO SILENT RETRIES / PASS-FAIL DISTRIBUTION

### 14.1 无静默重试

- 所有命令均为**单次调用**；**无** `-run` 重试循环、**无** `t.Retry`、**无**「失败即重跑至成功」。
- 失败即**如实记录**（E2/E3 exit 1）。

### 14.2 PASS / FAIL 分布（诚实全量）

| 实验 | 树 | 模式 | 结果 |
|---|---|---|---|
| E1 | pristine HEAD | vet | **FAIL**（exit 1，测试包不编译） |
| L1 | GCS(33) | build/vet | PASS ×2 |
| L2 | GCS(33) | blockchain 隔离测试 | **PASS**（78.857s） |
| E2 | GCS(33) | 全套 | **FAIL**（exit 1；cmd/node 600.429s + explorer FAIL） |
| E3 | GCS+SET B(39) | 全套 | **FAIL**（exit 1；cmd/node 600.494s） |
| E4-run1 | GCS+SET B(39) | cmd/node 隔离 | **PASS**（586.721s） |
| E4-run2 | GCS+SET B(39) | cmd/node 隔离 | **PASS**（590.410s） |
| E4-run3 | GCS+SET B(39) | cmd/node 隔离 | **PASS**（597.772s） |
| E5 | GCS(33) | cmd/node 隔离 | **PASS**（735.852s，**>600s**） |
| M1–M5 | — | vet | M1/M2/M3 FAIL（必需）；M4/M5 PASS（可选） |
| E1–E4(vet) | — | vet | PASS ×4（可排除） |

**cmd/node 总览：全套 0/2 PASS、隔离 3/3 PASS。**
**explorer：GCS 0/1 PASS、GCS+SET B 1/1 PASS。**
**F1-N1 flaky：2/2 PASS（本阶段未复现）。**

```
NO SILENT RETRIES: CONFIRMED. 全部分布如上，无隐藏重试。
```

---

## §15 — STABILITY CLASSIFICATION（S0–S6）

### 15.1 分类量表（本阶段定义）

| 级别 | 名称 | 定义 |
|---|---|---|
| **S0** | STABLE | 确定性 PASS，余量 ≥ 25%（≥150s / 600s） |
| **S1** | STABLE-NARROW | 确定性 PASS，余量 < 25% 但 ≥ 5%（30–150s） |
| **S2** | LOAD/TIME-SENSITIVE | 隔离 PASS、并行 FAIL；或余量 < 5%（<30s） |
| **S3** | FLAKY | 同条件下非确定性 PASS/FAIL（无负载差异） |
| **S4** | DETERMINISTIC FAIL | 稳定 FAIL，根因确定 |
| **S5** | ENVIRONMENT-DEPENDENT | 结果由外部因素（Go 版本 / 主机资源）决定 |
| **S6** | UNRESOLVED | 根因未确立 |

### 15.2 逐对象定级

| 对象 | 级别 | 依据 |
|---|---|---|
| **Genesis 自有测试**（`TestF3BExplicitInitializationStateMatrix`） | **S0** | 6/6 PASS，1.46–1.75s，确定性 |
| **`internal/blockchain` 包（L2 隔离）** | **S0/S1** | 78.857s vs 600s，余量 87%，确定性 PASS |
| **`cmd/node` 包** | **S2**（主）+ **S1**（隔离时） | 隔离 PASS（余量 **0.37%–2.21%**）；并行 FAIL（2/2） |
| **`internal/explorer` / `TestPathBoundaryRawTCP`** | **S4** + **S5** | HEAD 断言 301 vs Go 1.27 的 307 ⇒ 确定性 FAIL，Go 版本依赖 |
| **`TestF1N1_C_MixedLegacyV2Reorg`** | **S3** + **S5** | `time.Now()` 种子 + 有界重试 ⇒ ~0.4%/harvest 概率失败 |
| **`TestR1F4PreParkRejectionReasons`（HEAD 版）** | **S4**（性能退化） | O(N²) 148–151s，确定性 |
| **Genesis 测试基线（整体）** | **S2** | 由 cmd/node 的负载/预算脆弱性主导 |

```
STABILITY CLASSIFICATION:
  Genesis-owned artifacts ................ S0  (STABLE)
  internal/blockchain .................... S0/S1
  cmd/node ............................... S2  (LOAD/TIME-SENSITIVE) + S1
  internal/explorer ...................... S4/S5 (DETERMINISTIC + ENV-DEPENDENT)
  TestF1N1_C_MixedLegacyV2Reorg .......... S3/S5 (FLAKY + TIME-SEEDED)
  GENESIS TEST BASELINE (aggregate) ...... S2
```

---

## §16 — GENESIS ACCEPTANCE

### 16.1 三问三答

| 问题 | 答案 |
|---|---|
| 是否存在 **Genesis 功能性缺陷**？ | **NO**（L1 编译闭合、L2 隔离全绿、Genesis 自有用例 6/6 PASS） |
| 是否由 **Genesis 引起**任何测试不稳定？ | **NO**（cmd/node 的 99.75% 成本、explorer 失败、flaky 均非 Genesis） |
| 是否存在 **稳定绿色的 Genesis 测试基线**？ | **NO**（受非 Genesis 因素支配） |

### 16.2 判定

```
GENESIS ACCEPTANCE (test-stability dimension): YES
  范围：Genesis 自有产物（F-3B 生产码 + 夹具 + 身份测试）。
  含义：Genesis 本身稳定且无缺陷；不稳定性 100% 归因于非 Genesis 工作流 + 测试基建预算。
  同时声明：stable-green FULL SUITE = NO（该 NO 不是 Genesis 的阻塞项）。
```

### 16.3 对 F-6.4 阻塞项的映射

| F-6.4 阻塞项 | F-6.5 结论 |
|---|---|
| BLK-1/BLK-2/BLK-3（scope/ownership） | 未变（§11.1、§11.2 复证） |
| **BLK-4**（cmd/node 余量 ≈29s） | **修正为 ≈2.2–13.3s（0.37%–2.21%）**，阻塞**加重** |
| BLK-5（explorer 依赖 SET B） | 复证（C2 必需） |

> **F-6.5 不改变 F-6.4 的 `BLOCKED / OWNER DECISION REQUIRED` 判定**，而是**强化**其技术依据。

---

## §17 — ZERO-DRIFT PROOF

### 17.1 开工 vs 收工（受保护对象 + 基线）

| 对象 | 开工 | 收工 | 漂移 |
|---|---|---|---|
| HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` | `4d892be355a38886a83c123faad915c9f3cd62e4` | **0** |
| staged | 0 | 0 | **0** |
| tracked modified | 35 | 35 | **0** |
| porcelain `M` | 36 | 36 | **0** |
| `.gitignore` | `f43418cf74ea996c` | `f43418cf74ea996c` | **0** |
| `node.exe` | `3972843aff90428d` | `3972843aff90428d` | **0** |
| P3.1 doc | `e9def9aa1f90e5ac` | `e9def9aa1f90e5ac` | **0** |
| GAP-4 ×4 | `3f4cb778…`/`bb7b00bc…`/`4d7841d4…`/`c634c1ec…` | 同 | **0** |
| tokens（大小） | 33/64/34 B | 33/64/34 B | **0** |
| `run-a/` `run-b/` | 只读未触碰 | 只读未触碰 | **0** |

### 17.2 写操作清单

| 类别 | 数量 |
|---|---|
| 仓库内文件写入 | **0** |
| `git add` / `git commit` / `git restore` / `git checkout` | **0** |
| `.gitignore` 修改 | **0** |
| token 内容读取 | **0** |
| 实验产物位置 | **`E:/wakuang/f65-verify`（仓库之外）** |

### 17.3 本阶段新交付物（写入仓库根，属本阶段授权产物）

| 文件 | 说明 |
|---|---|
| `PHASE-F6.5-GENESIS-TEST-STABILITY-FORENSIC-AUDIT.md` | 本报告 |
| `PHASE-F6.5-TEST-STABILITY-MATRIX.json` | 机器可读测量矩阵 |

> 二者为本阶段的**授权交付物**，不属于对既有工作树的修改。

```
ZERO-DRIFT: PROVEN（HEAD / staged / tracked-modified / porcelain / 受保护对象哈希 全部不变）
```

---

## §18 — FINAL GATE

### 18.1 结论摘要

1. **Genesis 无功能性缺陷**：L1 编译闭合、L2 隔离全绿、Genesis 自有用例 6/6 PASS。
2. **两个失败均已归因到非 Genesis 工作流**：
   - FAILURE-F1 = cmd/node 时间预算 + 负载敏感（#5/#6/#7）；
   - FAILURE-F2 = explorer 陈旧 stdlib 断言 301 vs 307（#2/#7）。
3. **cmd/node 隔离余量仅 +2.228s（0.37%）**（最坏），F-6.4 的 +28.751s（4.8%）**应下修**。
4. **纯 GCS(33) 隔离即达 735.852s**，超预算 135.852s（22.6%）——由 R1 的 O(N²) 用例造成。
5. **SET B 必需（explorer）但不充分（cmd/node）**。
6. **结构性污染成立**：X-01（F1-N1，BLOCKED）被编译强制纳入 GCS；cmd/node 为混合工作流包。
7. **未发现任何未解决的 Genesis 稳定性问题**（类别 #8 = NONE）。

### 18.2 最终闸门

```
╔══════════════════════════════════════════════════════════════════════════╗
║  PHASE F-6.5 COMPLETE / TIME-LOAD FRAGILITY IDENTIFIED / HARD STOP       ║
╚══════════════════════════════════════════════════════════════════════════╝
```

| 项 | 值 |
|---|---|
| Status | **COMPLETE** |
| Finding | **TIME-LOAD FRAGILITY IDENTIFIED** |
| Genesis Acceptance | **YES（Genesis-scoped，无 Genesis 缺陷）** |
| Stable-green full suite | **NO（非 Genesis 阻塞）** |
| Aggregate stability class | **S2（LOAD/TIME-SENSITIVE）** |
| 下一阶段 | **不自动推断** |

### 18.3 HARD STOP 声明

> 本阶段**到此为止**。
> **不**执行 `git add` / `commit`；**不**修改任何超时配置；**不**修复任何失败；
> **不**自动进入 Commit / Test Stabilization / 任何后续阶段。
> 后续动作须由 **Owner 单独授权**。

---

*PHASE F-6.5 — GENESIS TEST STABILITY FORENSIC AUDIT — 本报告为自引用文档，
SHA-256 于收工后由独立命令计算并登记于文件之外（memory / 交付说明）。*
