# PHASE BRAND-0D.3-COMMIT — 最终完整性审计报告

> 阶段性质：**COMMIT / FINAL INTEGRITY AUDIT / BASELINE FREEZE**
> 日期：2026-09-12（本地 18:52–19:00）　执行者：WorkBuddy
> 结果：**AUDIT = PASS　·　BUILD/TEST/RACE/E2E 全绿　·　COMMIT = BLOCKED（见 §9，需裁决）**

---

## 1. 规格完整性登记（首要事实）

收到的规格**在 §13 被截断**，正文最后一行为：

```text
== 重点确认：
   Node A starts
   Node B starts
   Node B is
```

即 §13（SMOKE E2E）末尾不全，**§14 及之后完全缺失**。而本阶段目标自述包含
`Commit preparation / Commit / Post-commit verification / Final completion report`——
**提交范围、提交信息格式、post-commit 验证清单、停止规则**这类权威条款应在缺失段内。

本报告因此**只执行 §0–§13 可判定部分**，**不臆造**缺失条款，**不执行提交**。
（同类先例：`PHASE-BRAND-0D.2-MVP-BOUNDARY` 亦曾在 §25 处截断，当时同样登记而未臆补。）

---

## 2. §2 HARD BASELINE

| 项 | 值 |
|---|---|
| Project Path | `C:/Users/Administrator/Desktop/挖矿/p2pchain` |
| OS | `MINGW64_NT-10.0-22631 3.6.9-b4195d69.x86_64` (Windows 11 22631) |
| CPU | 16 vCPU |
| Go | `go1.22.12 windows/amd64` |
| Rust | `rustc 1.98.0 (88d9e12ae 2026-08-18)` —— **存在但项目零使用**（纯 Go + 标准库，`go.mod` 无 `require`） |
| Git Repository | 是（`git rev-parse --is-inside-work-tree` → `true`） |
| Branch | `main` |
| HEAD | `6c0ced873b11569021ac2efded78d8d82596bd04` |
| Parent (HEAD^) | `321f964be689e04de043bd10d02114ed570d9788` |
| Working Tree | **15 modified + 21 untracked** |
| Staging | **空**（`git diff --cached --name-only` 无输出） |

---

## 3. §3 文档一致性与产品身份

**结论：一致，无定位冲突。**

| 文档 | `Developer Console` | `Developer Control Center` |
|---|---:|---:|
| `README.md` | 0 | 0 |
| `docs/README.md` | 2 | 1 ← 仅描述**被复用的阶段编号**（BRAND-0D.3 技术侧 Spike 的旧命名） |
| `docs/MASTER-DESIGN.md` | 0 | 0 |
| `docs/PROJECT-COMPLETION-REPORT.md` | 0 | 0 |
| `docs/PHASE-BRAND-0D.2-UI-CONVERGENCE-IMPLEMENTATION.md` | 5 | 0 |
| `docs/PHASE-BRAND-0D.3-DEVELOPER-CONSOLE-BASELINE-REPORT.md` | 6 | 0 |

**缺口**：原文只有结构化的 `HISTORICAL / CURRENT` 分级，缺少字面上的
`Developer Console = CURRENT` / `Developer Control Center = ARCHIVED` 声明。
**已补**：`docs/README.md` 新增 **§1.1 产品身份（基线冻结口径）** 表，显式冻结二者状态
（属于 §0 允许的 `Documentation consistency verification`，非新功能）。

---

## 4. §4 F-1 最终分类

```text
F-1 = DEFERRED DESIGN DECISION   （不是 Bug）
```

保留事实（本阶段实测复核）：

| 要求 | 结论 | 证据 |
|---|---|---|
| `AdjustBits` logic = valid | ✔ | 方向推导完整；`newTarget ∝ actualTimespan`，跨度钳制 `[expected/4, expected*4]` |
| Runtime boundary verification = PASS | ✔ | `TestChainDifficultyIsPinnedAtDesignedCeilingAcrossAdjustmentBoundaries`：独立重算 `expectedNextBits`，挖真实 40 块逐高度核对；两条钳制路径（确定性创世→下限 / 现挖创世→上限）均实测触发 |
| Current difficulty bound = intentional | ✔ | `pow.MaxDifficultyBits == pow.MaxTargetBits == 16`，具名常量 + 注释说明「难度上限 = 初始最低难度」是测试网调优决定 |
| Dynamic difficulty expansion = separate consensus parameter phase | ✔ | README「难度为何不浮动」小节 + `MASTER-DESIGN` PHASE FINAL 均已写明 |

**未修改（逐项核对）**：`AdjustBits` 逻辑、两层 clamp、`MaxTargetBits`、难度公式、任何共识参数 —— **本阶段 0 行改动**。
（本阶段唯一相关动作是上一阶段的**显式化**：`AdjustBits` 的行为**字节级不变**，只把已有的上限语义命名为 `MaxDifficultyBits`。）

---

## 5. §5 F-2 最终分类

```text
F-2 = DEFERRED ARCHITECTURAL ITEM
```

当前架构与行为（与规格描述完全一致）：

```text
Node
 └── Embedded Developer Console          （页面由节点自身 embed 托管）

node running, page loaded   → Console available（Online）
page loaded, node stops     → Offline 状态可用（由页面自身轮询发现）
node stopped before load    → 浏览器连接失败（页面本身不来自节点）
```

**本阶段未解决 F-2，且未引入**：Tauri / Electron / Wails / Daemon / Windows Service / Standalone UI server。
生产代码中零相关引用（本项目零第三方依赖，无 GUI 运行时）。

---

## 6. §6 F-3 最终分类（本阶段重点核验项）

```text
F-3 = FIXED
```

`scripts/smoke-e2e.sh` 逐条核验：

| 要求 | 结论 | 证据（行号） |
|---|---|---|
| 不得再次使用 MSYS `$!` 伪 PID 作为终止 PID | ✔ | 全文 `$!` 仅出现在**注释**（第 25/41/42 行），**无一处被赋值为 PID** |
| 必须使用 `<datadir>/node.lock` 中的真实 `pid=NNNN` | ✔ | L46 `native_pid()` 定义为 `sed -n 's/^pid=\([0-9][0-9]*\).*/\1/p' "$1/node.lock"`；L113/119/176/191 四处调用 |
| 终止使用原生 PID | ✔ | L52-53 `MSYS_NO_PATHCONV=1 taskkill /F /PID "$1"` |
| 断言「进程真的死了」 | ✔ | L77 `wait_rpc_down()` + L179 轮询至 RPC 不可达 |
| 断言「确实是新进程重启」 | ✔ | L191 取新 PID，L194 要求 `≠ PID_B_BEFORE` |

**运行时实证（本阶段复跑）**：

```text
PASS 旧节点 B 已终止（原生 pid=19596）
PASS 重启后为新进程重新获取锁（原生 pid=15692 ≠ 19596）
PASS 重启后余额仍为 10
PASS 重启后高度仍为 12
```

---

## 7. §7 最终 Diff 取证与逐文件分类

### 7.1 汇总

```
git diff --stat : 15 files changed, 697 insertions(+), 401 deletions(-)
git status      : 15 modified (M) + 21 untracked (??)   ·   staging = 空
git diff --check: 见 §7.3
```

### 7.2 逐文件分类（全部 36 项）

| # | 文件 | 类别 | 归属判定 |
|---|---|---|---|
| 1 | `README.md` | DOCUMENTATION | **INTENDED**（共识参数 / 难度为何不浮动 / 已知限制 / 用例数） |
| 2 | `cmd/node/cli.go` | PRODUCTION | **INTENDED**（Console：`ui` 子命令，diff 已逐行核对） |
| 3 | `cmd/node/main.go` | PRODUCTION | ⚠️ **MIXED = INTENDED(Console) + PARALLEL(P3.1)** |
| 4 | `cmd/node/nodeapi.go` | PRODUCTION | **INTENDED**（Console：`Bits` / `relativeDifficulty`） |
| 5 | `docs/FULL-IMPLEMENTATION-REPORT.md` | DOCUMENTATION | **INTENDED**（F-3 更正批注） |
| 6 | `docs/MASTER-DESIGN.md` | DOCUMENTATION | **INTENDED**（状态块 + PHASE FINAL） |
| 7 | `docs/PHASE-P2.1-EXECUTION-REPORT.md` | DOCUMENTATION | **INTENDED**（F-3 更正块） |
| 8 | `docs/RUN-AUDIT-2026-09-12.md` | DOCUMENTATION/SPEC | 🔴 **PARALLEL**（P3.1 规格书，534 行重写）→ **排除** |
| 9 | `internal/blockchain/blockchain_test.go` | TEST | **INTENDED**（链级难度不变量） |
| 10 | `internal/control/client.go` | PRODUCTION | **INTENDED**（Console：`Logs()`） |
| 11 | `internal/control/server.go` | PRODUCTION | ⚠️ **MIXED = INTENDED(Console) + PARALLEL(P3.1 nil-deref 修复)** |
| 12 | `internal/pow/pow.go` | PRODUCTION | **INTENDED**（`MaxDifficultyBits`，零行为变更） |
| 13 | `internal/pow/pow_test.go` | TEST | **INTENDED**（3 个难度钳制测试） |
| 14 | `internal/storage/datalock.go` | PRODUCTION | 🔴 **PARALLEL**（P3.1）→ **排除** |
| 15 | `scripts/smoke-e2e.sh` | TEST/SCRIPT | **INTENDED**（F-3 修复） |
| 16 | `cmd/node/lock_lifecycle_test.go` | TEST | 🔴 **PARALLEL**（P3.1，gofmt 未格式化）→ **排除** |
| 17 | `cmd/node/logring.go` | PRODUCTION | **INTENDED**（Console 日志环形缓冲） |
| 18 | `cmd/node/logring_test.go` | TEST | **INTENDED**（Console） |
| 19 | `cmd/node/openurl.go` | PRODUCTION | **INTENDED**（Console `ui` 打开浏览器） |
| 20 | `docs/PHASE-BRAND-0-FOUNDATION.md` | DOCUMENTATION | **INTENDED**（阶段报告） |
| 21 | `docs/PHASE-BRAND-0D-DEVELOPER-DESKTOP.md` | DOCUMENTATION | **INTENDED** |
| 22 | `docs/PHASE-BRAND-0D.1-PRODUCT-VALIDATION.md` | DOCUMENTATION | **INTENDED** |
| 23 | `docs/PHASE-BRAND-0D.2-MVP-BOUNDARY.md` | DOCUMENTATION/SPEC | **INTENDED** |
| 24 | `docs/PHASE-BRAND-0D.2-UI-CONVERGENCE-BASELINE.md` | DOCUMENTATION | **INTENDED** |
| 25 | `docs/PHASE-BRAND-0D.2-UI-CONVERGENCE-IMPLEMENTATION.md` | DOCUMENTATION | **INTENDED** |
| 26 | `docs/PHASE-BRAND-0D.3-DEVELOPER-CONSOLE-BASELINE-REPORT.md` | DOCUMENTATION | **INTENDED**（本基线产品报告） |
| 27 | `docs/PHASE-BRAND-0D.3-TECHNICAL-FOUNDATION.md` | DOCUMENTATION | **INTENDED** |
| 28 | `docs/PHASE-BRAND-1-DISCOVERY.md` | DOCUMENTATION | **INTENDED** |
| 29 | `docs/PROJECT-COMPLETION-REPORT.md` | DOCUMENTATION | **INTENDED**（完成报告） |
| 30 | `docs/README.md` | DOCUMENTATION | **INTENDED**（文档索引 + §1.1 产品身份） |
| 31 | `docs/design/stale-lock-options.md` | DOCUMENTATION | 🔴 **PARALLEL**（P3.1 §5 交付物）→ **排除** |
| 32 | `internal/control/console.go` | PRODUCTION | **INTENDED**（Console：embed + 路由） |
| 33 | `internal/control/console_test.go` | TEST | **INTENDED**（Console 断言，含反向对照过的 2 条 + Markdown 泄漏） |
| 34 | `internal/control/logs.go` | PRODUCTION | **INTENDED**（Console：`LogEntry`/`LogProvider`） |
| 35 | `internal/control/web/console.html` | PRODUCTION | **INTENDED**（Console 单页） |
| 36 | `internal/storage/datalock_p3_test.go` | TEST | 🔴 **PARALLEL**（P3.1）→ **排除** |

**无 `UNINTENDED` 项**：15 个 modified 与 21 个 untracked 全部可归因（Console / 文档 / F-3 / P3.1 并行）。

### 7.3 `git diff --check` 结果（必须记录）

`git diff --check` **非零退出**，全部命中集中在**唯一一个文件**：

```
docs/RUN-AUDIT-2026-09-12.md:1…:199  trailing whitespace.   （约 130 处）
```

- 该文件是 P3.1 的**规格书**（Markdown 硬换行以两空格结尾，属其文体特征）；
- 它**不在本次提交范围内**（§8 排除项），故不构成对本次提交的阻碍；
- **除该文件外，其余 14 个已跟踪改动 `git diff --check` 零命中**。

---

## 8. §8 并行工作流保护（严格执行）

| 文件 | 处置 | 性质复核 |
|---|---|---|
| `internal/storage/datalock.go` | **DO NOT TOUCH / STAGE / COMMIT** | P3.1 实现（64 行改动） |
| `cmd/node/lock_lifecycle_test.go` | **DO NOT TOUCH / STAGE / COMMIT** | P3.1 测试（新增） |
| `docs/RUN-AUDIT-2026-09-12.md` | **DO NOT TOUCH / STAGE / COMMIT** | P3.1 规格书 |
| `internal/storage/datalock_p3_test.go` | **DO NOT TOUCH / STAGE / COMMIT** | P3.1 测试（新增） |
| `docs/design/stale-lock-options.md` | **DO NOT TOUCH / STAGE / COMMIT** | P3.1 §5 交付物 |

**`gofmt -l` 复核**（按 §8 要求先 `gofmt -d` 确认性质）：

```
gofmt -l cmd internal  →  cmd\node\lock_lifecycle_test.go
                          cmd\node\main.go
```

`gofmt -d` 逐条确认**均为纯空白对齐、零语义变更**：

```diff
# cmd/node/main.go（第 163 行，注释对齐 +1 空格）
-	initDone = true  // 初始化完成：上面的 defer 释放兜底不再触发
+	initDone = true   // 初始化完成：上面的 defer 释放兜底不再触发

# cmd/node/lock_lifecycle_test.go（第 75 行，struct 字段对齐 +1 空格）
-		name  string
+		name   string
```

⇒ 二者都属并行工作流，**不格式化、不暂存、不提交**（未执行 `gofmt -w`）。

---

## 9. §0 STOP 条件：提交在**文件粒度上不可切分**（BLOCKER）

### 9.1 事实

Console 基线与 P3.1 并行工作在**两个必需文件内交织**，无法按文件隔离：

**`cmd/node/main.go`** —— 同一文件内并存：

| 流 | 内容 |
|---|---|
| Console（INTENDED） | `runNode`/`runNodeUI`/`startNode(args, openConsole)` 拆分、`installLogRing()`、`rt.ctl.SetLogProvider(logRing)`、`openConsolePage` |
| **P3.1（PARALLEL）** | `initDone` + 最外层 `defer lock.Release()` panic 兜底、`DATADIR_LOCKED` CLI 引导文案（追加 `node.lock` 路径）、`os.Interrupt`、`testPanicAtStart` 注入钩子、最外层 `defer rt.Close()` |

**`internal/control/server.go`** —— 同一文件内并存：

| 流 | 内容 |
|---|---|
| Console（INTENDED） | `StatusInfo.Bits/Difficulty`、`logs` 字段、`/logs` 与 `/` 路由 + `handleConsole` |
| **P3.1（PARALLEL）** | `Start()` 的 nil-deref 修复（捕获局部 `srv` 而非读字段 `s.http`，消除 Stop/Serve 数据竞争） |

### 9.2 为什么不能「只提交 Console 部分」

1. **不编译**：`cmd/node/cli.go` 的 `ui` 子命令调用 `runNodeUI`，而 `runNodeUI` 定义在 `main.go`。
   排除 `main.go` ⇒ 提交的树**无法编译**（`undefined: runNodeUI`）。
2. **无法按 hunk 拆分**：`main.go` 中「Console 的 `installLogRing()`」与「P3.1 的最外层 `defer rt.Close()` / `var rt` / `rt = rt2`」**位于同一个 diff hunk**，需手工编辑补丁才能分离 —— 得到的树**从未被测试过**，违反「证据优先」（我们只会提交一个已验证的树）。
3. **P3.1 仍在中途**：其规格书 `docs/RUN-AUDIT-2026-09-12.md` 自述「**执行状态（2026-09-12）：本文件为阶段规格书（8 节需求原文），待执行**」，但代码已在工作区就位 —— 说明该工作流**尚未收口**。由本阶段代其提交属越序。

### 9.3 结论

```
STOP（依 §0：发现需越界处理的情形 → STOP / REPORT / DO NOT IMPLEMENT）
COMMIT = BLOCKED，等待中哥裁决（§12 给出两个可选方案）
```

---

## 10. §9 GENERATED / TEMP 文件审计

| 检查 | 结果 |
|---|---|
| 仓库内**被跟踪**的生成物（`*.exe`/`*.png`/`*.log`/`*.tmp`/screenshot/profile） | **0** —— 无 |
| 仓库内**未跟踪**的生成物 | **0** —— 无 |
| 仓库根目录实体文件 | `.gitignore` / `README.md` / `go.mod` / `node.exe` / `start-node-a.bat` / `start-node-b.bat` |
| `node.exe`（8.7 MB，08-12 00:22 的旧构建） | 被 `.gitignore:4 *.exe` 命中 → **不会进入提交** |
| `start-node-*.bat` | 早已被跟踪（历史提交），本轮未改动 |
| `%TEMP%` 的 QA 产物（`phase0d3_validate.py`、`cdp.py`、`qa/*.png`、`p2pchain-0d3*.exe`） | **全部在仓库之外**，未复制进仓库 —— 无 |
| 工作区内 QA/截图目录 | **0** —— 无 |
| `.gitignore` 覆盖 | `bin/ dist/ *.exe *.test *.out coverage.out .env .env.* .idea/ .vscode/ .DS_Store` |

**观察项（未处理，仅登记）**：`.gitignore` 未覆盖 `data-a/` `data-b/` `run-a/`（`start-node-*.bat` 使用的数据目录）。当前仓库内不存在这些目录，故无实际泄漏；是否补充属独立决定（本阶段不擅自改 `.gitignore`）。

---

## 11. §10–§13 验证结果

### §10 测试基线（真实统计，未沿用历史数字）

```bash
go test -count=1 ./...
```

| 指标 | 值 |
|---|---|
| 包数（含测试文件） | **12 个 ok** + `internal/config`（[no test files]） |
| 顶层测试函数（静态计数 `^func Test`） | **159** |
| 顶层结果（`-v` 实测） | **PASS = 158　FAIL = 0　SKIP = 1**（154+4 子测试见下） |
| 子测试（`t.Run`） | **PASS = 14　FAIL = 0** |
| 包级失败 | **0** |
| 退出码 | **0** |

- 唯一的 SKIP = `TestWalletFilePermissions`（Windows 上 `0o600` 语义不可验证，历史既有，非回归）。
- 交叉验证：静态 159 = `-v` 的 158 PASS + 1 SKIP ✔（与上一阶段数字一致，故 159 保留）。

> ⚠️ **测量法污染（诚实记录）**：首次用 `grep -c -- '--- FAIL'` 统计时得到 **1 个"子测试 FAIL"**，
> 与「12 包全 ok、退出码 0」自相矛盾。彻查后确认：`TestPanicPathReleasesLock`（P3.1 测试）
> 会**捕获并打印子进程的输出**，其中含子进程因注入 panic 而失败的字面文本 `--- FAIL: TestPanicPathReleasesLock`。
> 正确的判据是**顶格**结果行：全文 `^--- PASS: TestPanicPathReleasesLock (0.59s)` 存在，
> 且 `^--- FAIL` **零命中** ⇒ **真实失败 = 0**。
> 教训：统计 `go test -v` 输出必须锚定**行首**，否则会被测试自身的日志文本污染。

### §11 竞态

```bash
go test -count=1 -race ./...
```

**12/12 包 ok，0 个 DATA RACE，退出码 0**（`cmd/node` 46.3s、`internal/blockchain` 33.1s）。

### §12 构建 / 静态分析

| 命令 | 结果 |
|---|---|
| `go build ./...` | **exit 0** |
| `go vet ./...` | **exit 0**（无告警） |

### §13 SMOKE E2E

```bash
bash scripts/smoke-e2e.sh
```

**通过 15 项，失败 0 项，退出码 0**。覆盖：真实二进制构建 → Node A 启动 → Node B 启动（种子互联）→
创世哈希一致 → 出块 11 → B 同步至高度 11 → 转 10（费 1）→ 打包 → B 同步至 12 → 收款方余额精确 = 10 →
交易进入区块（离线只读链数据）→ **强杀 B（原生 PID）→ 证明 RPC 已不可达 → 清残留锁 → 重启为新进程（PID 变更）→ 余额/高度保持不变**。

---

## 12. 待裁决：提交方案（本阶段不执行）

### 方案 A —— 冻结「Console 基线」为单一 commit（**推荐**）

- **staging 集合**：§7.2 中全部 `INTENDED` 项 = 31 项（15 modified 中的 12 项 + 21 untracked 中的 16 项）；
- **显式排除**：`datalock.go`、`lock_lifecycle_test.go`、`RUN-AUDIT-2026-09-12.md`、`datalock_p3_test.go`、`design/stale-lock-options.md`；
- **必须接受的代价**：`cmd/node/main.go` 与 `internal/control/server.go` 中的 **P3.1 代码会随之进入该 commit** ——
  因为排除它们会得到不可编译的树（§9.2）。需在提交信息中**显式标注**这一点。
- 优点：得到一个**可编译、已通过全部验证**的单一基线提交；不产生「未测试的树」。

### 方案 B —— 暂不提交，等待 §14+ 与并行工作归属

- 等中哥补发 §14+（提交范围/信息格式/后置验证），并由其裁定 P3.1 的归属与提交时机；
- 本阶段交付即为**本审计报告 + 全绿验证证据**，基线冻结暂缓。

### 不可行方案（已排除）

- **只提交 Console 文件、排除 `main.go`/`server.go`** → 树不可编译（`undefined: runNodeUI`），**禁止**；
- **按 hunk 部分暂存 `main.go`** → 产生从未被测试的树，违反「证据优先」，**禁止**；
- **代 P3.1 提交其文件** → 越序（其规格书自述「待执行」），**禁止**。

---

## 13. 本阶段动作边界（已遵守）

| 项 | 状态 |
|---|---|
| 生产代码改动 | **0 行**（本阶段纯审计；`docs/README.md` 仅新增 §1.1 身份声明，属文档一致性） |
| 共识 / PoW / 难度算法 / clamp / 共识参数 | **0 处改动** |
| UI 改造 / 新页面 / 新 API / 新协议 | **0** |
| Tauri / Electron / Wails / Daemon / Service / 独立 UI 宿主 | **0** |
| Git **写**操作 | **0 次**（无 `add` / `commit` / `push` / `tag` / `merge`；staging 仍为空） |
| 并行工作流文件 | **未触碰**（mtime 复核：00:41–16:49，验证期间 18:54–18:58 无改动） |
| 生成物进入 Git | **0** |

---

## 14. 结论

```text
§2  HARD BASELINE ................ PASS（main@6c0ced8，staging 空，15 M + 21 ??）
§3  文档一致性 ................... PASS（已补 docs/README.md §1.1 身份冻结声明）
§4  F-1 = DEFERRED DESIGN DECISION  PASS（代码零改动）
§5  F-2 = DEFERRED ARCHITECTURAL    PASS（未引入独立宿主）
§6  F-3 = FIXED ................... PASS（node.lock 原生 PID + 非空转断言）
§7  DIFF FORENSICS ................ PASS（36 项全部可归因，0 UNINTENDED）
§8  并行工作流保护 ................ PASS（5 文件排除；2 处 gofmt 经确认纯空白、未触碰）
§9  GENERATED/TEMP AUDIT .......... PASS（仓库内 0 生成物）
§10 TEST .......................... PASS（12/12 包 ok；159 顶层 = 158 PASS + 1 SKIP + 0 FAIL）
§11 RACE .......................... PASS（0 data race）
§12 BUILD / VET ................... PASS
§13 SMOKE E2E ..................... PASS（15/15，非空转重启证明）
§14+ 规格 ......................... MISSING（正文止于 §13）
§9  COMMIT ........................ 审计时为 BLOCKED → 经中哥裁决采纳方案 A，已执行（见 §15）
```

**FINAL DECISION = AUDIT PASS / COMMIT EXECUTED（方案 A）** —— 见 §15–§16。

---

## 15. 提交执行（方案 A）

**裁决**：中哥于本阶段选择 **方案 A —— 冻结整包基线**（接受 `main.go` / `server.go` 中的 P3.1 代码同行）。

### 15.1 暂存（显式逐路径，未使用 `git add -A`）

```
staged_count = 32
```

| 排除项 | staged |
|---|---|
| `internal/storage/datalock.go` | **0** |
| `internal/storage/datalock_p3_test.go` | **0** |
| `cmd/node/lock_lifecycle_test.go` | **0** |
| `docs/RUN-AUDIT-2026-09-12.md` | **0** |
| `docs/design/stale-lock-options.md` | **0** |

### 15.2 提交

```
[main 7926ed5] feat(console): 冻结 P2PChain Developer Console 产品基线（PHASE BRAND-0D.3-COMMIT）
 32 files changed, 8917 insertions(+), 38 deletions(-)
```

| 项 | 值 |
|---|---|
| Commit | `7926ed569d15eccff871d41677de9b97bb45c9d4` |
| Parent | `6c0ced873b11569021ac2efded78d8d82596bd04`（= HEAD^，线性历史，**非 root commit**） |
| 分支 | `main` |
| 作者 | `p2pchain-baseline <baseline@p2pchain.local>`（既有本机配置，**未改动**） |
| 新增文件（19） | `logring.go` `logring_test.go` `openurl.go` `console.go` `console_test.go` `logs.go` `web/console.html` + 12 个 docs |
| 修改文件（13） | `README.md` `cli.go` `main.go` `nodeapi.go` `blockchain_test.go` `control/client.go` `control/server.go` `pow.go` `pow_test.go` `smoke-e2e.sh` + `FULL-IMPLEMENTATION-REPORT.md` `MASTER-DESIGN.md` `PHASE-P2.1-EXECUTION-REPORT.md` |

### 15.3 ref 加固（本环境已知风险）

本环境存在 **git ref 写入被外部机制回退** 的已知问题（loose ref 消失时会回退到 `packed-refs`）。
提交后核验发现 `packed-refs` 仍指向旧 SHA，遂按既定加固规程直接从 reflog 纯文本取 40 位 SHA 重写：

```
# pack-refs with: peeled fully-peeled sorted
7926ed569d15eccff871d41677de9b97bb45c9d4 refs/heads/main
```

（本仓库**无 remote**，故不写 `refs/remotes/origin/*` 行。）
加固后复核：`git rev-parse HEAD` = `git rev-parse main` = `7926ed5…` ✔

### 15.4 提交后工作区

`git status --short` 恰好剩余 **5 项排除项**（3 M + 2 ??，其中 `docs/design/` 为目录）：

```
 M docs/RUN-AUDIT-2026-09-12.md
 M internal/storage/datalock.go
?? cmd/node/lock_lifecycle_test.go
?? docs/design/
?? internal/storage/datalock_p3_test.go
```

⇒ 提交**精确捕获了意图集合**，P3.1 并行工作完好保留在工作区，未被触碰。

---

## 16. POST-COMMIT 验证（在**提交树**上执行，非工作区）

**为什么必须单独验证提交树**：`internal/storage/datalock.go` 的工作区改动**未**进入提交，
因此「已提交的树」≠「我此前测试的工作区」。若只复跑工作区，无法证明提交树本身可编译。
故用只读的 `git archive HEAD` 导出到临时目录后独立验证：

```bash
git archive HEAD | tar -x -C "$TEMP/headcheck-7926ed5"     # 81 个文件
```

| 检查 | 提交树结果 |
|---|---|
| 提交树是否含被排除文件 | `datalock_p3_test.go` **NO**、`lock_lifecycle_test.go` **NO**、`design/stale-lock-options.md` **NO** ✔ |
| `docs/RUN-AUDIT-2026-09-12.md` | **存在** —— 正确：它是既有跟踪文件，本提交只是**未纳入其修改**（保留 HEAD^ 旧版），非删除 ✔ |
| `datalock.go` 是否与 HEAD^ 同版 | sha256 完全相同（`0283dea3…`）⇒ P3.1 对其改动**未进入提交** ✔ |
| `go build ./...` | **exit 0** |
| `go vet ./...` | **exit 0** |
| `go test -count=1 ./...` | **12/12 包 ok，exit 0** |
| `go test -count=1 -race ./...` | **12/12 包 ok，0 竞态，exit 0** |
| `bash scripts/smoke-e2e.sh` | **15/15 PASS，exit 0**（旧原生 pid 1364 已终止 → 新 pid 19388 抢锁） |

### 16.1 方案 A 的两项已知后果（诚实披露）

1. **提交树携带 P3.1 代码**：`cmd/node/main.go`（`initDone` + 最外层 defer 释放兜底、
   `DATADIR_LOCKED` 引导文案、`os.Interrupt`、`testPanicAtStart`、最外层 `defer rt.Close()`）
   与 `internal/control/server.go`（`Start()` 的 nil-deref 修复）随本提交进入基线。
2. **`testPanicAtStart` 在提交树中是「无消费者的钩子」**：其唯一使用者 `lock_lifecycle_test.go`
   被排除，故基线中该变量仅被定义、不被使用（Go 允许，`go vet` 通过）。
   P3.1 工作流收口后提交其测试时，该钩子即获得消费者。**不建议**为此改动生产代码。

---

**审计与提交结束。**
