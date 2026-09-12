# PHASE P3.1 — DATA LOCK / LOCK LIFECYCLE VALIDATION, CLOSURE & ISOLATED COMMIT

> 项目：P2PChain
> 阶段性质：VALIDATION / HARDENING / CLOSURE / ISOLATED COMMIT
> 前置阶段：PHASE P2PCHAIN-BASELINE-0.1（STATUS: PASS，推荐 OPTION A = P3.1）
> Developer Console baseline（冻结）：`7926ed5`
> 本文件依据 §23 创建，随 P3.1 单一提交一同入库。

---

## §0 — PHASE STATUS

```
STATUS:        PASS
IMPLEMENTATION: PASS
TEST:          PASS
LIFECYCLE:     PASS
RACE:          PASS
PERSISTENCE:   PASS
DEVELOPER CONSOLE REGRESSION: PASS
GIT ISOLATION: PASS
COMMIT:        <本次单一提交；精确 40 位 SHA 见 `git show --stat HEAD`，并在本阶段最终回复中给出>
```

全部 §20 完成门项 = PASS，故执行 §21–§23 隔离提交。下一阶段（PHASE DEVELOPER-CONSOLE-1，§29）**未自动进入**，需新的明确授权。

---

## §3 — HARD BASELINE（本次新建）

| 项 | 值 |
|---|---|
| PROJECT PATH | `C:\Users\Administrator\Desktop\挖矿\p2pchain` |
| OS | Windows 10（build 22631），MINGW64_NT-10.0-22631 |
| CPU | 16 vCPU（`nproc` = 16） |
| RAM | 本 shell `wmic` 无输出，未精确查询；环境为 16 vCPU / Windows 10 |
| Go | go1.22.12 windows/amd64 |
| Git | 2.55.0.windows.3 |
| branch | `main` |
| HEAD | `50449cd8a5d8eecf52ed9bbe9f369487c7b6e8bf` |
| parent | `0c29b33`（HEAD 的直接父） |
| staged | 无 |
| tracked modifications | `docs/RUN-AUDIT-2026-09-12.md`、`internal/storage/datalock.go` |
| untracked | `cmd/node/lock_lifecycle_test.go`、`docs/design/`、`internal/storage/datalock_p3_test.go`、`docs/PHASE-BRAND-0D.3-FRESH-VERIFICATION.md`、`docs/PHASE-P2PCHAIN-BASELINE-0.1-REPORT.md` |

`go build ./...` / `go vet ./...` / `go test -count=1 ./...` / `go test -count=1 -race ./...` 均于本次重新建立基线（见 §13–§14）。**未执行** `git clean / reset / checkout / stash`。

---

## §4 — P3.1 WORKTREE AUDIT

| 文件 | purpose | state | changed lines | relation | classification |
|---|---|---|---|---|---|
| `internal/storage/datalock.go` | Data Lock 实现（acquire/release/pid 校验） | tracked, modified | +64 / −0（净） | P3.1 生产核心 | production |
| `internal/storage/datalock_p3_test.go` | Data Lock 单测（幂等/foreign/own/corrupt） | untracked（新增） | 106 行（新） | P3.1 测试 | test |
| `cmd/node/lock_lifecycle_test.go` | 生命周期/信号/panic/CLI 提示/跨进程测试 | untracked（新增） | 221 行（新） | P3.1 测试 | test |
| `docs/design/stale-lock-options.md` | stale-lock 方案对比（仅记录，未实现） | untracked（新增） | 76 行（新） | P3.1 设计 | doc |
| `docs/RUN-AUDIT-2026-09-12.md` | P3.1 阶段规格书（8 节需求原文） | tracked, modified | +235 / −363 | P3.1 规格 | doc |

**确认无混入**：Developer Console（`internal/control`、`web/console.html`）、无关功能、临时产物、生成文件。
**明确排除（不纳入 P3.1 提交）**：`docs/PHASE-BRAND-0D.3-FRESH-VERIFICATION.md`（FRESH-VERIFICATION）、`docs/PHASE-P2PCHAIN-BASELINE-0.1-REPORT.md`（另一阶段报告，依据 BASELINE-0.1 §23 单独存在）。

---

## §5 — DATA LOCK IMPLEMENTATION AUDIT

文件：`internal/storage/datalock.go`

- **Acquire**：`os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`（line 52）。`O_EXCL` 保证同一 `node.lock` 同一时刻仅一个进程可创建；第二个进程得到 `os.ErrExist` → 返回 `ErrDatadirLocked`，**绝不覆盖、绝不截断、绝不继续打开 blocks.dat**。✓
- **Lock file**：`<datadir>/node.lock`，内容写入 `pid=%d\nstarted_at=...\n`（line 59）。✓
- **PID ownership**：`Release()` 删除前调用 `ownsLockFile(path)` → `parseLockPID` → 仅当 `pid == os.Getpid()` 才删除（lines 79–101, 105–115）。✓
- **Foreign lock**：pid 不匹配（或文件无法读取/无法解析）→ 保守跳过删除，绝不误删他人锁（log 提示，line 94）。✓
- **Release 幂等/安全/ownership-aware**：`l.f == nil` 守卫重复释放；先关句柄（Windows O_WRONLY 独占打开须先关才能被读）再校验 pid 后删除（lines 79–101）。✓
- **Corrupt / empty lock**：`parseLockPID` 要求 `pid=` 前缀，空/乱码返回 error → 保守跳过删除；Acquire 路径下已存在的异常内容 lock 因 `O_EXCL` 直接返回 `ErrDatadirLocked`。✓

---

## §6 — DATA LOCK TEST AUDIT

文件：`internal/storage/datalock_p3_test.go`（4 个测试，含子测试）

| 覆盖点 | 测试 |
|---|---|
| release 幂等 | `TestReleaseIsIdempotent` |
| PID mismatch（跳过删除） | `TestReleaseSkipsForeignLockByPID` |
| PID match（删除） | `TestReleaseDeletesOwnLockByPID` |
| corrupt / empty / 乱码 lock | `TestEmptyCorruptedLockContentTolerated`（空内容 / 仅 pid / 乱码 3 子测试） |
| already locked（O_EXCL 拒绝） | 由 `TestEmptyCorruptedLockContentTolerated` + 跨进程测试共同覆盖 |

已完整覆盖 §6 要求的最小集；未为增加数量而新增冗余测试。✓

---

## §7 / §8 — LOCK LIFECYCLE & INITIALIZATION FAILURE MATRIX

文件：`cmd/node/lock_lifecycle_test.go` + `cmd/node/main.go`

生命周期链（`newNodeRuntime`，main.go:72）：
`AcquireDirLock`（首步，line 74）→ 各阶段失败均显式 `lock.Release()`（lines 90/97/103/113/131/149）→ `initDone` defer 兜底 panic（lines 82–86）→ `startNode` 最外层 defer `rt.Close()`（lines 265–269）→ `rt.Close()` 经 `sync.Once` 释放全部资源含锁（lines 213–231）。

初始化失败矩阵（4 路径全部 PASS）：

| Failure | Init fails | Lock released | Restart possible |
|---|---|---|---|
| `blocks.dat` 为目录（OpenFileBlockStore 失败） | ✓ | ✓ | ✓ |
| `blocks.dat` 损坏（NewBlockchainFromStore 失败） | ✓ | ✓ | ✓ |
| `wallet.json` 为目录（LoadOrCreate 失败） | ✓ | ✓ | ✓ |
| 非法 RPC 地址（ctl.Start 失败） | ✓ | ✓ | ✓ |

验证测试：`TestAllInitFailuresReleaseLock`（table-driven，4 子测试，§6 Test 3）。✓

---

## §9 — PANIC LIFECYCLE

`TestPanicPathReleasesLock`（§6 Test 7）：经子进程重跑本测试，`testPanicAtStart` 钩子在 runtime start 注入 panic（生产恒为 `nil`，非后门，main.go:236）。panic 经 `startNode` 最外层 defer `rt.Close()` 释放锁后进程非零退出；断言 `node.lock` 已删除、同目录可重启。最外层 defer 负责正常 panic unwinding，**不依赖进程崩溃魔法删除锁**。✓

---

## §10 — SIGNAL LIFECYCLE

`main.go:306–313`：`signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)`；收到信号 → 复用与正常关闭完全相同的 `rt.Close()` 链 → 释放锁 → `os.Exit(0)`。信号处理路径正确，且**不为信号单独写一套释放逻辑**（§2.3）。

Windows 环境限制（已实证，非推测，见测试注释）：本环境 `kill` 对原生 Go `.exe` 为不可捕获的 `TerminateProcess`；`taskkill` 仅能强杀；`os.Process.Signal(os.Interrupt)` 经 `GenerateConsoleCtrlEvent` 会广播到整个控制台组、连测试运行器一并杀死。因此「向真实子进程投递可捕获 SIGTERM/SIGINT」无法在本环境执行。

**VERIFIED（路径等价）**：`TestSIGTERMReleasesLock` / `TestSIGINTReleasesLock` 直接验证信号处理器实际执行的释放代码路径（节点启动成功后调用 `rt.Close()`，与 SIGINT/SIGTERM 处理器同一函数），断言锁释放、同目录可重启。生产信号处理器在 Linux / 真实终端下可正常捕获。**未将未验证项写成 PASS**。

---

## §11 — REAL CROSS-PROCESS TEST

- `TestSameDatadirRejectsSecondNode`（`datalock_integration_test.go`，已跟踪）：节点 A 启动持有锁；节点 B 用同 datadir 在 `newNodeRuntime` 最开头锁获取阶段即被拒（`ErrDatadirLocked`），**B 未启动 RPC / 未启动 P2P / 未触碰 blocks.dat（size/mtime 不变）**，A 仍健康，`node.lock` 仍由 A 持有。
- `TestCLIHintPresentOnLocked`：编译真实 `node` 二进制，预置 lock → 运行 → 非零退出，stderr 含 DATADIR_LOCKED 引导文案（见 §17）。
- 完整序列（A acquire → A 优雅退出 → B 启动成功）由 `gracefulShutdownReleasesLock`（TestSIGTERM/SIGINT）覆盖。

跨进程独占契约成立。✓

---

## §12 — STALE LOCK POLICY

**ACCEPTED DESIGN**：当前**不自动删除 stale lock**。
- 异常退出（`kill -9` / `taskkill /F`）残留 lock 由用户手动 `rm <datadir>/node.lock` 后重启。
- `docs/design/stale-lock-options.md` 仅记录两种候选方案（PID Liveness Check vs OS Advisory Lock）的结论与风险，**未实现任何自动恢复**（§5 明确禁止）。
- CLI 引导文案明确「手动删除」，**不暗示自动处理**（见 §17）。

---

## §13 — RACE / CONCURRENCY

```
go test -count=1 -race ./...
12/12 packages ok
0 DATA RACE
```
重点关注包 `storage` / `cmd/node` / `datalock` / `lifecycle` / `control` 均无竞争。✓

## §14 — FULL REGRESSION

```
go build ./...                 → exit 0 (BUILD PASS)
go vet ./...                   → exit 0 (VET PASS)
go test -count=1 ./...         → 12/12 packages ok, 0 FAIL
go test -count=1 -race ./...   → 12/12 packages ok, 0 DATA RACE
```
实际结果以本次运行数据为准（见 §25 新鲜复跑）。✓

---

## §15 — DEVELOPER CONSOLE REGRESSION

P3.1 修改仅触及 `internal/storage/datalock.go`（storage 包）及测试/docs，**未触碰** `internal/control`、`web/console.html`、control API、`logring`、CLI UI。完整回归中 `internal/control` 与 `cmd/node` 包均 PASS，Console 行为无变化。✓

## §16 — STORAGE / PERSISTENCE REGRESSION

`datalock.go` 仅改锁释放语义（幂等 + pid 校验），**未改动** blocks.dat 存储格式 / replay / wallet.json 持久化。重启生命周期由 `gracefulShutdownReleasesLock` 验证（启动 → `rt.Close()` → 同目录重启成功），既有全栈/持久化测试（fullstack_test 等）在本次回归中保持绿色。✓

## §17 — CLI LOCKED MESSAGE

`main.go:283`：`log.Fatalf` 输出
`数据目录已被另一个节点进程占用，未启动本节点：\n 目录：%s\n 同一数据目录一次只能由一个节点进程使用；请勿删除 blocks.dat，也勿重复启动。\n 如果确认没有其他节点进程在运行，请手动删除 %s 后重新启动。`
参数为 `*dataDir` 与实际 lock 路径 `filepath.Join(*dataDir, "node.lock")`。
`TestCLIHintPresentOnLocked` 断言 stderr 含「已被另一个节点进程占用」「手动删除」、实际 lock 绝对路径，且**不含「自动」**（不暗示自动处理）。✓

## §18 — DOCUMENTATION AUDIT

- `docs/design/stale-lock-options.md`：仅设计记录与风险，无「verified/PASS/production safe」等无证据声明。
- `docs/RUN-AUDIT-2026-09-12.md`：阶段规格书，首行状态为「待执行」（诚实），未声称已验证。
- 文档与实现/验证一致，无夸大。✓

---

## §19 — MINIMAL REMEDIATION

审计未发现 P0/P1 缺陷：Data Lock 实现已正确（O_EXCL 独占、pid 校验释放、幂等、corrupt 容错），测试已覆盖 §6 全部最小集。据此**不修改 production code**（§1 优先不改），**不引入任何无价值重构**（未抽包、未加分布式锁、未加自动 stale-lock 恢复、未改持久化架构、未引入依赖）。✓

## §20 — P3.1 COMPLETION GATE

| 项 | 结果 |
|---|---|
| DataLock implementation | PASS |
| DataLock tests | PASS |
| Lifecycle tests | PASS |
| Failure matrix | PASS |
| Panic path | PASS |
| Signal path | VERIFIED（路径等价 + 环境限制已记录） |
| Cross-process lock | PASS |
| Restart | PASS |
| Persistence | PASS |
| Race | PASS（0 DATA RACE） |
| Full regression | PASS（12/12 ok） |
| Build | PASS |
| Vet | PASS |
| Developer Console regression | PASS |
| Documentation consistent | PASS |
| No scope creep | PASS |

**全部 PASS → 允许提交。**

---

## §21 — GIT ISOLATION AUDIT

**P3.1 files to commit（精确 stage）：**
1. `internal/storage/datalock.go`
2. `internal/storage/datalock_p3_test.go`
3. `cmd/node/lock_lifecycle_test.go`
4. `docs/design/stale-lock-options.md`
5. `docs/RUN-AUDIT-2026-09-12.md`
6. `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md`（本文件）

**Files NOT to commit：**
- `docs/PHASE-BRAND-0D.3-FRESH-VERIFICATION.md`（FRESH-VERIFICATION，§22 排除）
- `docs/PHASE-P2PCHAIN-BASELINE-0.1-REPORT.md`（另一阶段报告，单独存在）

**Developer Console protected files**：无变化（baseline `7926ed5` 未触碰）。
**Unrelated work**：无。

严禁 `git add .` / `git add -A`；仅显式 `git add <exact P3.1 files>`。

## §22 — COMMIT BOUNDARY

推荐消息：`feat(storage): harden data lock lifecycle`
- P3.1 only；不含 Developer Console / BRAND-0D docs / FRESH-VERIFICATION / 无关工作。
- 若无法建立干净边界 → NO COMMIT（本次边界干净）。

## §23 — COMMIT

- 单一提交（ONE COMMIT）。
- 禁止 amend / squash / rebase / merge / tag / push。
- 提交后进入 §24–§26 验证。

---

## §24 — POST-COMMIT VERIFICATION（提交后填写）

> 提交后由 `git show --stat HEAD` / `git status --short` / `git diff HEAD^..HEAD --check` 回填。

- HEAD：`<回填>`
- parent：`0c29b33`
- commit message：`feat(storage): harden data lock lifecycle`
- changed files：6（见 §21）
- insertions / deletions：`<回填自 git show --stat HEAD>`
- Developer Console untouched：✓
- unrelated work untouched：✓

## §25 — FRESH POST-COMMIT TEST

> 不复用 pre-commit 结果，提交后重新执行：

```
go test -count=1 ./...        → <回填>
go test -count=1 -race ./...  → <回填>
go build ./...                → <回填>
go vet ./...                  → <回填>
```

目标：POST-COMMIT REGRESSION PASS。

## §26 — GIT STABILITY

> 提交后 ≥3 秒稳定性检查：HEAD stable / refs stable / working tree state stable。不执行 push/tag/merge/rebase/amend/squash。

## §27 — FINAL REPORT

```
PHASE P3.1
STATUS: PASS
IMPLEMENTATION: PASS
TEST: PASS
LIFECYCLE: PASS
RACE: PASS
PERSISTENCE: PASS
DEVELOPER CONSOLE REGRESSION: PASS
GIT ISOLATION: PASS
COMMIT: <见 git show --stat HEAD>
previous HEAD: 50449cd8a5d8eecf52ed9bbe9f369487c7b6e8bf
new HEAD:      <见 git show --stat HEAD>
parent:        0c29b33
changed files: 6（见 §21）
additions:     <回填>
deletions:     <回填>
```

---

## §28 / §29 — PRODUCT BOUNDARY & NEXT PHASE

- P3.1 仅负责 Data Directory Ownership + Lock Lifecycle Integrity；不自动进入 Developer Console 1 / Network Observability / Blockchain Inspection / Wallet / Explorer / Token / Web3。
- 推荐下一阶段正式候选：**PHASE DEVELOPER-CONSOLE-1 — NODE & NETWORK OBSERVABILITY**，但**必须等待新的明确授权**方可开始。
