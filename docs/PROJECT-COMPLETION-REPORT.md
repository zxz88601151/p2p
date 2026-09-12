# P2PChain 项目完成报告（PROJECT COMPLETION REPORT）

> 日期：2026-09-12　HEAD：`6c0ced8`　分支：`main`
> 定位：**学习型 PoW 区块链骨架**；Go 1.22.12；**标准库零外部依赖**。
> 结论：**PROJECT STATUS = COMPLETE** —— 功能实现、全量测试、文档三线全部收口；**0 个未决 P0 / 0 个未决 P1**。
> 纪律：本报告只陈述**可复现实证**。每一项「通过」都附命令与实测数字；凡「测量方法被修正」与「断言被放宽」两件事严格分开陈述。

---

## 1. 交付范围与「完成」的定义

本项目**不使用外部定义的完成标准**，而使用仓库自身在 `README.md`「测试」一节写明的门禁：

```bash
go test ./...                 # 单元 + 集成 + 进程内端到端
go test -race ./...           # 数据竞态检测
bash scripts/smoke-e2e.sh     # 双真实节点进程：出块→同步→转账→打包→余额→真重启持久化
```

**完成定义（Definition of Done）**：全部模块实现 + 上述三条门禁全绿 + `go build ./...` / `go vet ./...` 通过 + README/脚本/文档同步更新 + 完成报告落盘。

交付物：`cmd/node`（1 个可执行程序）+ `internal/` 下 **12 个包**：

```
block · transaction · utxo · blockchain · mempool · txbuild
wallet · p2p · storage · pow · control · config
```

---

## 2. 全量验收证据（2026-09-12 复跑）

| 检查 | 命令 | 实测结果 | 结论 |
|---|---|---|---|
| 构建 | `go build ./...` | exit 0，无输出 | ✅ PASS |
| 静态分析 | `go vet ./...` | exit 0，无告警 | ✅ PASS |
| 全量测试 | `go test -count=1 ./...` | **12/12 包 ok，0 FAIL** | ✅ PASS |
| 竞态检测 | `go test -count=1 -race ./...` | **12/12 包 ok，0 DATA RACE** | ✅ PASS |
| 双进程 E2E | `bash scripts/smoke-e2e.sh` | **通过 15 项，失败 0 项** | ✅ PASS |
| 控制台真实浏览器 E2E | `P2PCHAIN_BIN=… phase0d3_validate.py` | **PASS=109 FAIL=0** | ✅ PASS |
| 控制台静态完整性 | `check_console.py` | `RESULT = PASS`（JS 语法 OK，6 类禁用项为 0） | ✅ PASS |
| 格式 | `gofmt -l cmd internal` | 仅 2 个**并行工作流**文件（见 §9），本次改动 **0 个** | ✅ PASS（本改动） |

测试计时（`-count=1`，非并行）：`cmd/node` 23.4s · `internal/blockchain` 13.1s · `internal/pow` 2.6s · 其余各 < 1.3s。
`-race` 下：`cmd/node` 44.0s · `internal/blockchain` 35.0s（真实 TCP 端到端在内）。

---

## 3. 测试用例分布（159 个顶层测试函数）

| 包 | 用例数 | 包 | 用例数 |
|---|---:|---|---:|
| `cmd/node` | 27 | `internal/pow` | 20 |
| `internal/control` | 25 | `internal/wallet` | 17 |
| `internal/utxo` | 16 | `internal/storage` | 12 |
| `internal/blockchain` | 10 | `internal/mempool` | 9 |
| `internal/block` | 8 | `internal/p2p` | 5 |
| `internal/transaction` | 5 | `internal/txbuild` | 5 |
| **合计** | | | **159** |

- 另有 **4 组** `t.Run` 子测试；**1 个**条件 SKIP：`TestWalletFilePermissions`（Windows 上 `0o600` 权限语义不可验证，属历史既有，**非回归**）。
- `internal/config` 无测试文件（纯常量与解析，被上层间接覆盖）。

---

## 4. 本轮（2026-09-12）新增与修复

### 4.1 F-3：E2E「重启持久化」断言空转（**P1，已修复**）

见 §8 —— 本轮最重要的发现。

### 4.2 F-1：难度钳制意图显式化（**P2，按设计关闭**）

见 §6。零行为变更，仅把已有语义**显式命名**并**补齐证据**。

### 4.3 文档诚实化（产品面与控制台）

| 位置 | 变更 |
|---|---|
| `README.md` 共识参数表 | 新增「难度上限 = `MaxDifficultyBits = MaxTargetBits = 16`」与「难度动态范围 = 固定为 16（上限 = 下限）」两行 |
| `README.md` 新增小节 | 「难度为何不浮动（重要，避免误读为缺陷）」——说明这是**测试网调优决定**，并给出放开浮动所需的独立阶段边界 |
| `README.md` 已知限制 | 新增「难度浮动」条目 |
| `internal/control/web/console.html` | 披露卡新增「难度语义」段落（`Difficulty` 恒为 ×1，**这不是故障**）；`Difficulty` 指标加 `title` 提示；顺带修掉 2 处误写入 HTML 的 Markdown 粗体语法 |
| `internal/control/console_test.go` | 新增 `TestConsoleHasNoMarkdownLeak`（页面不得出现 `**` / `## ` / `- [ ] `），堵住此类回归 |
| `docs/README.md`（新增） | 文档索引：权威/当前/历史/规格/快照五级分级、文件名 ↔ 正文阶段号错位对照表、推荐阅读顺序、维护规则 |

### 4.4 历史报告更正批注（不改写历史）

- `docs/PHASE-P2.1-EXECUTION-REPORT.md` §4 末尾：加 **F-3 更正块**，说明该节「重启段」的 2 条证据当时无效。
- `docs/FULL-IMPLEMENTATION-REPORT.md` §3 表格下方：加 **F-3 更正块** + 数值口径更新说明。

> 原则：历史报告的时点数字**保留不改**（它们是当时的真实快照），但**由后续证据推翻的结论必须就地标注**，否则读者无法分辨。

---

## 5. F-1：难度为何固定为 16（**有意设计，非缺陷**）

### 5.1 规则的推导部分本身是完整的

`pow.AdjustBits(currentBits, actualTimespan)`：

1. `newTarget ∝ actualTimespan` —— 实际跨度**短于**期望 → 目标变小（更难）；**长于**期望 → 目标变大（更易）；
2. `actualTimespan` 先被钳制到 `[expected/4, expected*4]`（单次幅度 ≤ 4 倍）。

方向推导正确，且被 `TestAdjustBitsDirection` 覆盖。

### 5.2 输出随后经过**两层有意钳制**（这才是「不浮动」的原因）

| 钳制 | 触发条件 | 结果 |
|---|---|---|
| **难度下限** | `newTarget > T(MaxTargetBits)` | 回落到 `MaxTargetBits` |
| **难度上限** | `newBits > MaxDifficultyBits`（本链 `== MaxTargetBits`） | 回落到 `MaxDifficultyBits` |

两条路径都把 `bits` 钉在 **16**，因此链上可达难度只有一个点。

### 5.3 两条 runtime 路径都被实测触发过（此前是未验证的理论）

| 路径 | 现场 | 断言 |
|---|---|---|
| **下限路径** | 确定性创世（`GenesisTimestamp = 1700000000`，2023-11-14）→ 高度 20 首次调整时 span ≈ 9×10⁷ s ≫ `maxTimespan`(4800) | 钳制到 16 |
| **上限路径** | 现挖创世（`time.Now()`）→ 首个周期 span 极小 | 钳制到 16 |

`TestChainDifficultyIsPinnedAtDesignedCeilingAcrossAdjustmentBoundaries` 用**独立重算**（`expectedNextBits` 按共识规则从当前 tip 重新推导）而不是复读被测代码的返回值，在高度 20/40 逐点核对 `bc.CurrentBits()`，并对**每一个高度**断言 `tip.Header.Bits == pow.MaxDifficultyBits`（真实挖矿 40 块，约 97k hash/s）。

### 5.4 为什么不能「顺手放开」

本链以 **CPU 毫秒级出块**为目标（实测 ~60 ms/块）。若允许难度按公式自由上升，每周期 `+2 bits`（16→18→20…），约 200 块后单块需枚举 `2^36` 次哈希 → 单块耗时从毫秒级升到**小时级**，学习与回归价值随之消失。

**难度是共识真值**：改动会让老节点拒绝新区块。因此放开浮动必须**重设 clamp 带宽**并把 `MaxDifficultyBits` 抬到预期上限，属于**独立的共识参数阶段**——本轮**明确不做**，只把意图写成常量与文档。

### 5.5 反向对照（证明新断言非空转）

把 `MaxDifficultyBits` 临时改为 `MaxTargetBits + 4`：

```
FAIL 创世后 CurrentBits=16, want 20
```

→ 链级断言确实在对「上限是否起作用」负责，而不是恒真。已还原并复跑通过。

---

## 6. F-2：节点托管 UI 的冷启动边界（**DEFERRED**）

**等价表述**：本控制台是「节点的一个界面」，不是「独立于节点存在的应用」。节点未启动时无法加载 UI（页面由节点自身 `embed` 并托管）。

- **性质**：产品定位的直接结果（Console 服务于 Node Operator），**不是缺陷**；
- **若要消除**：必须引入独立宿主进程 —— 那属于**被明确禁止的范围扩展**；
- **判定**：`F-2 STATUS = DEFERRED` / `NOT MODIFIED`。控制面对该边界的职责是**如实呈现**（离线时给出 `Node Offline` 卡片、真实端点、可重试），该职责已履行并有 109 项浏览器实证。

---

## 7. F-3：E2E「重启持久化」断言空转（**P1，已修复**）

### 7.1 现象

`scripts/smoke-e2e.sh` 报告 13/13 通过，其中两条是「重启后余额仍为 10」「重启后高度仍为 12」。表面上证明了**进程级**重启持久化。

### 7.2 根因

脚本用 `taskkill /F /PID "$!"` 终止节点。但 **Git Bash 的 `$!` 是 MSYS 伪 PID，不是原生 Windows PID**。本机实测：

```
bash $!       = 310            →  taskkill /F /PID 310  →  错误: 没有找到进程 "310"   （进程仍存活）
node.lock pid = 8936           →  taskkill /F /PID 8936 →  成功: 已终止 PID 为 8936 的进程
```

即：**进程从未被杀掉**。随后的「重启」节点因 `node.lock` 仍被占用而**正确地拒绝启动**，而 `wait_rpc`/`balance`/`height` 全部由**同一个存活进程**应答 → 断言恒真。

### 7.3 影响面（诚实界定）

| 受影响 | 说明 |
|---|---|
| `scripts/smoke-e2e.sh` 的 2 条重启断言 | 空转（已修） |
| `docs/PHASE-P2.1-EXECUTION-REPORT.md` §4「重启段」表述 | 已加更正批注 |
| `docs/FULL-IMPLEMENTATION-REPORT.md` §3 表格 | 已加更正批注 |

| **不受影响** | 说明 |
|---|---|
| 控制台浏览器 harness 的生命周期组 | 它用 Python `subprocess.Popen.pid` —— **该值就是原生 PID**，强杀是真杀（故障现场也印证了：残留 `node.lock` 导致重启被拒，正是「杀成功了」的表现） |
| 节点级重启/回放持久化 | 由 `internal/blockchain` 启动回放、`cmd/node` 重启类测试在**进程内**真实覆盖 |
| 其余 11 条 smoke 断言 | 只涉及单次运行中的节点，与 kill 无关 |

### 7.4 修复

PID 改从**被验证对象自身**写入的 `<datadir>/node.lock`（`pid=NNNN`）取得 —— 这是唯一权威来源；并新增两条**证明「确实重启了」**的断言：

1. `旧节点 B 已终止` —— 轮询至 RPC **不再响应**（证明进程真死，而不是「答的是旧进程」）；
2. `重启后为新进程重新获取锁` —— 新原生 PID **与旧值不同**。

### 7.5 证据（修复后真实复跑）

```
PASS 旧节点 B 已终止（原生 pid=19264）
PASS 重启后为新进程重新获取锁（原生 pid=17780 ≠ 19264）
PASS 重启后余额仍为 10
PASS 重启后高度仍为 12
== 结果  通过 15 项，失败 0 项
```

### 7.6 反向对照（证明新断言非空转）

把 `native_pid` 篡改为返回一个打不中的伪 PID（复现原缺陷形态）：

```
FAIL 旧节点 B 未被终止，重启持久化断言将无效
FAIL 重启后原生 pid 未变化（1），重启断言可能空转
PASS 重启后余额仍为 10        ← 旧的余额断言依旧「通过」——正是空转的铁证
== 结果  通过 13 项，失败 2 项
```

### 7.7 附带修掉的两个环境陷阱

- `unlock()` 用 `cygpath -u` 规范化路径：`mktemp -d` 在本机返回**含反斜杠的 Windows 路径**，直接拼给 `rm` 会被环境的安全删除垫片误判为非法路径而 **fail-closed**（静默不删）。此前 smoke 日志里的 `[safe-delete] SAFE_DELETE_FAIL_CLOSED` 即由此而来。
- 脚本内 `force_kill` 的错误注释（原文宣称 `taskkill /F` 是「真杀」却没说 PID 从哪来）已改正，并写明「**绝不能用 `$!` 当 PID**」。

---

## 8. 未决项分级与「明确不做」

### 8.1 未决项

| ID | 内容 | 级别 | 状态 |
|---|---|---|---|
| — | 无 P0 | — | — |
| — | 无 P1 | — | — |
| F-1 | 难度被钳制在 16 | P2 | **按设计关闭**（§5）：意图已显式化 + 双路径 runtime 证据 + 反向对照 |
| F-2 | 节点托管 UI 冷启动边界 | P3 | **DEFERRED**（§6）：消除它=被禁止的范围扩展 |
| H-1 | `gofmt -l` 报 2 文件：`cmd/node/main.go`、`cmd/node/lock_lifecycle_test.go` | P3 | **非本次范围**：属并行 lock-lifecycle 工作流（`initDone`/`defer` 释放兜底）。差异经 `gofmt -d` 核实为**纯对齐空白各 1 处**，零语义变更；按约束不触碰 |

### 8.2 明确不做（超出学习项目边界）

分叉处理 / 链重组（reorg）· **难度浮动** · secp256k1 / RIPEMD160（stdlib 限制，以 P-256 + SHA256 截断 20 字节替代）· SPV / 轻节点 · TLS / 对等认证 · 代币经济模型 · 独立于节点存在的 UI 宿主（F-2 的消除方案）。

以上均已在代码注释与 README 中说明升级路径。

---

## 9. 测试方法学与诚实性声明

### 9.1 反向对照清单（本轮全部执行并通过）

| # | 被验证的断言 | 故意破坏 | 观察到的失败 |
|---|---|---|---|
| 1 | 链上难度恒为上限 | `MaxDifficultyBits = MaxTargetBits + 4` | `FAIL 创世后 CurrentBits=16, want 20` |
| 2 | 页面无 Markdown 泄漏 | 页面重新写入 `**` | `FAIL 页面出现 2 处 Markdown 粗体语法 '**'` |
| 3 | 重启持久化断言有效 | 让 `native_pid` 返回打不中的 PID | `FAIL 旧节点 B 未被终止` + `FAIL 重启后原生 pid 未变化（1）` |
| 4 | `[hidden]` 真正隐藏 | 删除 `[hidden]{display:none !important}` | `FAIL … [grid3]`（计算样式仍为 `grid`） |
| 5 | 读请求有超时 | 移除 `AbortController` | `FAIL … "AbortController"` 缺失 |

### 9.2 「测量方法被修正」≠「断言被放宽」

- **修工具**（正当）：`tasklist /FO CSV` 末列带引号（`"12,524 K"`）导致 `$` 锚定正则失效 → 改按 `","` 切分；`subprocess.run(text=True)` 读中文输出抛 `UnicodeDecodeError` → 改 `encoding="utf-8"`；单飞刷新用「同一 eval 内三次点击」替代固定时间窗增量（消除 2 s 轮询污染）。
- **改断言**（禁止）：本轮**从未**下调任何断言阈值、删除任何测试、或修改任何预期值来「让它变绿」。
- **本轮唯一一次断言口径变化**是**加强**：smoke 从「13 项」变为「15 项」，新增的 2 项把空转断言变成真断言（§7）。

### 9.3 空转断言的通用识别手法（本轮教训）

1. **凡是「杀进程后状态不变」的断言**，必须独立证明**进程真的死了**（RPC 不可达 / PID 消失），否则断言可能答的是旧进程。
2. **PID 必须来自被验证对象自身**（`node.lock` 的 `pid=`、Python 的 `Popen.pid`），**不能**来自 shell 的 `$!`（MSYS 伪 PID）。
3. 断言的 `detail` 文案随 PASS 一起打印时，不要写失败态措辞——本轮已把 `check()` 的一处误导性 detail 修正为分条件文案。

---

## 10. 一键复现

```bash
cd p2pchain
export PATH="/c/Users/Administrator/.workbuddy/binaries/go/go/bin:$PATH"

go build ./...                          # ✅ exit 0
go vet ./...                            # ✅ exit 0
go test -count=1 ./...                  # ✅ 12/12 包 ok
go test -count=1 -race ./...            # ✅ 12/12 包 ok，无竞态
bash scripts/smoke-e2e.sh               # ✅ 15/15（双真实进程 + 真重启）

# 控制台（真实浏览器）——需先构建二进制并指定：
go build -o /tmp/p2pchain-final.exe ./cmd/node
P2PCHAIN_BIN=/tmp/p2pchain-final.exe python phase0d3_validate.py   # ✅ 109/109
python check_console.py                                            # ✅ RESULT = PASS
```

---

## 11. 文档索引

**入口：`docs/README.md`**（分级与命名错位对照表）。核心：

| 文档 | 作用 |
|---|---|
| `README.md`（仓库根） | 功能清单、构建运行、CLI、控制接口、P2P 协议、**共识参数**、已知限制 |
| `docs/PROJECT-COMPLETION-REPORT.md` | **本文档**：完成判定 + 全量证据 + 未决项分级 |
| `docs/MASTER-DESIGN.md` | 剩余工程的设计决策记录（含状态块） |
| `docs/FULL-IMPLEMENTATION-REPORT.md` | 各阶段实现细节（2026-09-11 时点快照 + F-3 更正） |
| `docs/PHASE-*.md` | 各阶段工程报告（含 `PHASE-BRAND-*` 产品/控制台阶段） |
| `docs/design/stale-lock-options.md` | stale-lock 方案对比（**仅记录，不实现**） |

---

## 12. 版本控制状态

> **以下为「收口阶段」的时点快照；其后 `PHASE BRAND-0D.3-COMMIT` 已完成基线冻结提交（见本节末）。**

| 项 | 值（收口阶段时点） |
|---|---|
| 分支 | `main` |
| HEAD | `6c0ced873b11569021ac2efded78d8d82596bd04` |
| 暂存区 | 空 |
| 收口阶段 Git **写**操作 | **0**（无 `add` / `commit` / `push` / `tag` / `merge`） |

> 收口阶段不改动版本历史，改动全部留在工作区。历史提交纪律（每阶段独立 `commit` + packed-refs 加固）见 `MASTER-DESIGN.md`。

### 12.1 基线冻结提交（PHASE BRAND-0D.3-COMMIT）

| 项 | 值 |
|---|---|
| Commit | **`7926ed569d15eccff871d41677de9b97bb45c9d4`** |
| 信息 | `feat(console): 冻结 P2PChain Developer Console 产品基线（PHASE BRAND-0D.3-COMMIT）` |
| Parent | `6c0ced8`（线性历史，非 root commit） |
| 规模 | 32 files changed, **8917 insertions(+), 38 deletions(-)** |
| 组成 | 13 修改 + 19 新增（Console 实现/测试/页面 6 + 文档 13） |
| 排除（未纳入） | `datalock.go`、`datalock_p3_test.go`、`lock_lifecycle_test.go`、`RUN-AUDIT-2026-09-12.md`、`design/stale-lock-options.md`（P3.1 并行工作） |
| ref 加固 | `.git/packed-refs` 同步指向 `7926ed5`（防 loose ref 丢失回退） |
| POST-COMMIT 验证 | 在**提交树**（`git archive HEAD`）上独立复跑：build/vet exit 0；`go test ./...` 12/12 ok；`-race` 12/12 ok / 0 竞态；`smoke-e2e.sh` 15/15 |

> 详尽的提交前审计（逐文件分类、STOP 判定证据）与提交后验证见 **`docs/PHASE-BRAND-0D.3-COMMIT-AUDIT.md`**。
> 已知同行项：`cmd/node/main.go` 与 `internal/control/server.go` 中的 P3.1 代码随该提交进入基线（因与 Console 改动同文件同 hunk、按文件不可切分），已在提交信息中显式声明。

---

## 13. 最终判定

| 维度 | 判定 |
|---|---|
| 功能实现 | ✅ 全部模块完成，可运行、可转账、可多节点共识同步 |
| `go build` / `go vet` | ✅ PASS |
| `go test ./...` | ✅ 12/12 包，159 个顶层用例，0 FAIL |
| `go test -race ./...` | ✅ 12/12 包，0 DATA RACE |
| 双进程 E2E | ✅ 15/15（**真重启**，PID 变更可证） |
| 控制台浏览器 E2E | ✅ 109/109 |
| 未决 P0 / P1 | ✅ 0 / 0 |
| 文档 | ✅ README + 索引 + 设计 + 各阶段报告 + 完成报告齐全，含更正批注 |
| 边界 | ✅ 未引入任何区块链以外的功能；未触碰并行工作流文件 |

### **`PROJECT STATUS = COMPLETE`**

学习型 PoW 区块链骨架已按仓库自身门禁交付完毕，测试全绿，证据可复现，未决项已如实分级。

---

**报告结束。**
