# PHASE BRAND-0D.2 — P2PCHAIN DEVELOPER CONSOLE UI CONVERGENCE

**实现记录 / IMPLEMENTATION REPORT**

| 项目 | 值 |
| --- | --- |
| 阶段 | PHASE BRAND-0D.2（UI CONVERGENCE & IMPLEMENTATION） |
| 前置 | `docs/PHASE-BRAND-0D.2-UI-CONVERGENCE-BASELINE.md`（HARD BASELINE = PASS） |
| 三项裁决 | D-1 Developer Console（本规格） / D-2 零依赖 Go + 内嵌 HTML / D-3 仅允许 bits·difficulty 与日志等级 |
| 生产代码 | 新增 4 文件 + 修改 5 文件 |
| 测试 | 全仓 148 个测试函数，25 个测试文件，全绿 |
| 真实 E2E | PASS=28 / FAIL=0（真实进程 · 真实 PoW · 真实日志） |
| 视觉 QA | 5 个断点 + 离线态 + file:// 回退，无布局破裂 |
| Git 操作 | 0 次 |
| 阶段对齐（2026-09-30） | 见 **§13**——本文件为时点实现记录，原文不回改；现态以追加节记录 |
| 状态 | 实现完成；**§25 起规格缺失，验收标准不完整，故不宣称阶段 DONE** |

---

# §1 交付物清单（文件级）

**新增**

| 文件 | 职责 |
| --- | --- |
| `internal/control/console.go` | `go:embed` 内嵌控制台页面 + `GET /`、`/console` 路由 |
| `internal/control/web/console.html` | Developer Console 单页前端（零依赖、零构建） |
| `internal/control/logs.go` | `LogEntry` / `LogProvider` 契约 + `GET /logs` 处理 |
| `cmd/node/logring.go` | 定长日志环形缓冲（`io.Writer` + `LogProvider`） |
| `internal/control/console_test.go` | 新增 10 个测试函数 |
| `cmd/node/logring_test.go` | 新增 9 个测试函数 |

**修改**

| 文件 | 改动 |
| --- | --- |
| `internal/control/server.go` | `StatusInfo` 增 `bits`/`difficulty`；注册 `/logs` 与 `/` |
| `internal/control/client.go` | 新增 `Client.Logs(tail int)` |
| `cmd/node/nodeapi.go` | `Status()` 填真实 `bits`/`difficulty` + `relativeDifficulty()` |
| `cmd/node/main.go` | `startNode(args, openConsole)` 拆分；接入日志环；`openConsolePage()` |
| `cmd/node/cli.go` | 新增 `ui` 子命令与用法文本 |
| `cmd/node/openurl.go` | 用系统默认浏览器打开页面（平台分支） |

**未新增任何第三方依赖**：`go.mod` 仍无 `require`；页面无 CDN、无构建步骤。

---

# §2 三项裁决的落实情况

| 裁决 | 落实 | 证据 |
| --- | --- | --- |
| **D-1** Developer Console（非 miner 面板） | 页面按 Node / Blockchain / Network / Mining-Hardware / Log 组织，无矿机语义（无算力曲线、无收益、无风扇） | `console.html` 结构 + 视觉 QA 截图 |
| **D-2** 零依赖 Go + 内嵌 HTML | `go:embed` 打包；`node ui` 启动后打开系统浏览器 | `go build ./...` 通过；`TestConsoleServesEmbeddedPage`；`TestConsoleHasNoFabricatedData` 断言无外部资源 |
| **D-3** 仅允许 bits·difficulty 与日志等级 | 后端**只**新增 `/status` 的 `bits`/`difficulty` 与 `/logs`；未动共识、未动 P2P、未动钱包 | `git diff` 仅含 `server.go`/`client.go`/`nodeapi.go`/`main.go`/`cli.go`；`blockchain`/`pow`/`utxo` 零改动 |

**越界自检**：本阶段**没有**实现 hashrate、温度、功耗、风扇、GPU 枚举、协议版本号、入/出站方向、延迟、同步进度、数据目录暴露中的任何一项——它们在页面上全部渲染为 `Unavailable`。

---

# §3 后端最小改动：为什么是这三处

## 3.1 `bits` / `difficulty`（`internal/control/server.go`、`cmd/node/nodeapi.go`）

```go
// StatusInfo 新增
Bits       uint32  `json:"bits"`        // 共识真值：下一个待挖区块的难度目标
Difficulty float64 `json:"difficulty"`  // 派生展示量：2^(bits - pow.MaxTargetBits)
```

```go
// relativeDifficulty：依据 pow.BitsToTarget 的定义反算倍数
func relativeDifficulty(bits uint32) float64 {
	if bits < pow.MaxTargetBits { return 0 }
	return math.Pow(2, float64(bits-pow.MaxTargetBits))
}
```

- **为什么必要**：难度是开发者控制台的第一屏指标。改动前 `/status` 完全没有难度字段，UI 只能显示 `Unavailable`。
- **为什么是这个形状**：`Bits` 是**共识真值**（`currentBitsLocked()` 的输出，区块校验第 3 步就用它）；`Difficulty` 是**纯展示派生量**，不参与任何判断。二者都在响应里给出，UI 显示派生值、需要时可核对真值。
- **为什么安全**：`relativeDifficulty` 是纯函数，`bits` 是 `uint32` 且有上界（`pow.MaxTargetBits` 语义），不存在 NaN/Inf 路径；`TestRelativeDifficulty` 用 `pow.BitsToTarget` 的真实 `big.Int` 目标值反算交叉验证，防止展示口径与共识口径脱钩。

## 3.2 `GET /logs`（`internal/control/logs.go`、`cmd/node/logring.go`）

- **为什么必要**：控制台运行在浏览器里，**无法读取节点进程的 stdout**。没有 `/logs`，日志面板只能是摆设。
- **为什么用独立接口而不是给 `Node` 加方法**：`LogProvider` 是**可选能力**。若并入 `Node` 接口，所有既有实现（含测试替身）都得改。独立接口让未注入时 `/logs` 返回 `[]` 而不是编译失败或报错。
- **为什么安装日志环不改变终端行为**：`log.SetOutput(io.MultiWriter(os.Stderr, ring))` —— 终端仍收到同一份字节，只是多了一路观察者。`TestInstallLogRingMirrorsLog` 断言安装后仍能捕获，且原文未被改写。

## 3.3 「等级」的诚实边界（重要）

标准库 `log` 的默认输出形如 `2006/01/02 15:04:05 [component] message`，**原生不带等级**。

因此 `LogEntry.Level` 是由正文关键词**推断**的（`inferLevel`），代码注释、页面脚注、本报告三处均已标注为「推断值，非节点原生字段」。这不是妥协包装成功能：页面把它当**分组着色**用，page footer 明确写着 `日志等级（INFO/WARN/ERROR）由日志文本按关键词推断，非节点原生字段`。

---

# §4 前端实现（`internal/control/web/console.html`）

## 4.1 数据映射表（全部字段逐一交代）

| 界面位置 | 数据来源 | 状态 |
| --- | --- | --- |
| Header 节点状态胶囊 | `/status` 可达性 | **真实** |
| Header 端点标签 | `location.host` | **真实**（见 §9 缺陷修复） |
| BLOCK HEIGHT | `height` | **真实** |
| DIFFICULTY | `difficulty` / `bits` | **真实** |
| PEERS | `peers.length` | **真实** |
| MEMPOOL | `mempool_size` | **真实** |
| MINING | `mining` | **真实** |
| Blockchain · Chain Tip | `tip_hash` | **真实**（含复制按钮） |
| Blockchain · Block Time | — | `Unavailable`（`/status` 无该字段） |
| Blockchain · Sync State | — | `Unavailable`（无 IBD 指标） |
| Network · Peers | `peers` | **真实** |
| Network · Inbound/Outbound/Latency/Protocol | — | `Unavailable` |
| Mining · Mining | `mining` | **真实** |
| Mining · On-demand | `POST /mine` | **真实**（可用性提示） |
| Mining · Hashrate/Device/Temperature/Power-Fan | — | `Unavailable` |
| Node Log | `/logs` | **真实**（等级为推断值） |
| Footer · 算法/签名/难度位/Node ID | 固定常量 + `/status` | **真实/常量**（已标注） |
| Footer · 数据目录/协议版本 | — | `Unavailable` |

**10 项无数据来源的指标全部渲染 `Unavailable`，并带 `title` 说明原因**——没有一处填 0、没有一处占位数字。

## 4.2 实现纪律（可回归断言）

| 约束 | 断言测试 |
| --- | --- |
| 无 `Math.random()`、无 `mockData/fakeData/dummyData` | `TestConsoleHasNoFabricatedData` |
| 无 `cdn./unpkg/jsdelivr/googleapis` 外部资源 | `TestConsoleHasNoFabricatedData` |
| 只走同源相对路径（无绝对地址、无 `XMLHttpRequest`、无 `WebSocket`） | `TestConsoleOnlyUsesSameOriginAPI` |
| 不硬编码控制接口默认地址 | `TestConsoleEndpointLabelNotHardcoded` |

## 4.3 状态机（§15 三态）

| 状态 | 触发 | 表现 |
| --- | --- | --- |
| **Running** | `/status` 200 | 全部真实值；「立即出块」可用 |
| **Offline** | fetch 失败 / 非 2xx | 胶囊转 `Offline`；主按钮禁用；KPI 与面板全部退化为 `—`；出现 `Node Offline` 卡片（真实端点 + 重试 + 启动命令提示）；不残留旧值 |
| **Error** | 交互动作失败（如 `/mine` 409） | 就地显示错误文本，不静默、不清屏 |

---

# §5 宿主与启动

```bash
node ui -mine -datadir <目录>     # 启动节点 + 打开 Developer Console
node  -mine -datadir <目录>       # 等价，但不自动打开浏览器
```

- `runNode` / `runNodeUI` 是同一份 `startNode`，唯一差异是 `openConsole` 布尔——**不存在两套启动路径**。
- 打开浏览器失败**不致命**：打印地址供手动访问（`openBrowser` 无可用浏览器时返回错误并被记录）。
- 无需浏览器也可用：`/` 直接返回页面。

---

# §6 测试证据

## 6.1 新增用例（19 个）

**`internal/control/console_test.go`（10）**
`TestStatusCarriesBitsAndDifficulty`（字段名即前端契约）· `TestLogsEmptyWithoutProvider`（必须是 `[]` 不是 `null`）· `TestLogsProviderNilYieldsEmptyArray` · `TestLogsTailHandling`（默认 100 / 显式 tail / 超限截断 500 / 非法 400 / 空 tail 走默认）· `TestLogsMethodNotAllowed` · `TestConsoleServesEmbeddedPage`（200 + `text/html` + `no-store`）· `TestConsoleRouteBoundaries`（未知路径 404、非 GET 405、HEAD 200）· `TestConsoleHasNoFabricatedData` · `TestConsoleEndpointLabelNotHardcoded` · `TestConsoleOnlyUsesSameOriginAPI`

**`cmd/node/logring_test.go`（9）**
`TestParseLogLine`（7 种形态，含 Windows 行尾与「正文以 `[` 开头但不是组件」）· `TestInferLevel` · `TestLogRingKeepsNewestLines` · `TestLogRingBuffersPartialLines`（跨 `Write` 拼接）· `TestLogRingSkipsBlankLines` · `TestLogRingRecentLogsBounds` · `TestNewLogRingDefaults` · `TestInstallLogRingMirrorsLog` · `TestRelativeDifficulty`（与 `pow.BitsToTarget` 交叉验证）

## 6.2 全量回归

```text
== gofmt（本次改动文件） ==   仅 cmd/node/main.go（历史遗留行，非本次引入，见 §11）
== go build ./... ==          BUILD_EXIT=0
== go vet ./... ==            VET_EXIT=0
== go test ./... -count=1 ==
ok  cmd/node 22.5s | internal/blockchain 3.98s | internal/pow 2.90s
ok  internal/control 0.35s | internal/block | mempool | p2p | storage
ok  internal/transaction | txbuild | utxo | wallet        （12 包全 ok）
```

```text
测试函数总数    129 → 148   （+19）
测试文件数       23 → 25    （+2）
生产代码文件     27 → 31    （+4）
```

**竞态检查**：`go test -race ./internal/control/ -count=1` → ok；`go test -race ./cmd/node/ -run 'TestLogRing|TestInstallLogRing|TestRelativeDifficulty|TestParseLogLine'` → ok。

---

# §7 真实数据 E2E（PASS=28 / FAIL=0）

**测量方式**：`go build` 产出真实二进制 → 真实子进程 + 真实 `-datadir` + 真实控制接口；断言全部基于真实响应，无任何桩。

真实 `/status`（高度 0，创世后）：

```json
{"height":0,
 "tip_hash":"0000aca1af720da390380a38dc97d141cfb67afe578a2d31315860a5489db58c",
 "peers":[],"mempool_size":0,"mining":false,
 "address":"NVKHRRkxtYZHRotUFLm3AaWbfSP11RU9L4","bits":16,"difficulty":1}
```

关键证据：

| 检查 | 实测 |
| --- | --- |
| `/status` 含全部 UI 依赖字段 | 缺失 `[]` |
| `difficulty` 与 `bits` 自洽 | `bits=16 difficulty=1 expect=1.0` |
| `/logs` 与进程 stderr **完全一致** | 最近 6 行中匹配 **6** 行（无编造） |
| `/mine count=2` | `{"mined":2,"height":2}`；链尾哈希变化 |
| 出块日志进入 `/logs` | 6 → 10 条，含「挖到新区块: 高度=2 …」 |
| `GET /` | 200 · `text/html` · `no-store` · 25,194 字节 |
| 未知路径 | 404 |
| 响应无 `NaN`/`Infinity`/`null` | 通过 |

**难度调整探针（真实出块跨越 20 区块周期）**：连挖至高度 21（出块远快于 60s/块期望间隔），`bits` 仍为 16、`difficulty` 仍为 1 → 见 §10 F-1。

---

# §8 视觉 QA

真实节点（高度 3、12 条真实日志）渲染，无头浏览器逐断点取样：

| 断点 | 布局 | 结果 |
| --- | --- | --- |
| 1024 | KPI 2 列；面板单列堆叠 | 无溢出、无遮挡、无横向滚动 |
| 1280 | KPI 5 列；面板 3 列（Network 占中） | 同上 |
| 1366 | KPI 5 列；面板 3 列 | 同上；链尾哈希换行不溢出 |
| 1440 | 同上 | 同上 |
| 1920 | 同上，留白增大 | 同上 |

**离线态（同源 404 触发）**：胶囊 `Offline`；「立即出块」禁用；KPI 全 `—`（含解释性 hint）；`Node Offline` 卡片显示**真实端点** `127.0.0.1:17790: HTTP 404` + 「重试连接」+ 启动命令提示；所有面板空态；日志面板空。**未残留任何旧值。**

**`file://` 回退**：端点标签显示中性的「同源控制接口」（不编造端口），错误文本 `无法连接控制接口 同源控制接口：Failed to fetch`。

**已记录的限制（非缺陷，属架构必然）**：页面与节点同源同进程 → **节点完全停机时全新加载页面得到的是浏览器错误页**，页面内置的 Offline 卡片只在「页面已加载后节点才断开（2s 轮询失败）」时可见。详见 §10 F-2。

---

# §9 缺陷与修复（本阶段自查发现）

**D-1：端点标签硬编码 —— 已修复并加测试锁定**

- **现象**：页头地址写死为 `127.0.0.1:6689`（`id="endpointLabel"` 只被读、从不被写）。节点换端口启动时页面显示**错误地址**；而该标签唯一被使用的地方正是离线错误提示 —— 用户会照着错地址排查。
- **性质**：显示一个「看起来像真数据」的错值，比显示空值更有害（违反 §19 精神）。
- **修复**：静态文本改为中性占位；启动时 `$("endpointLabel").textContent = location.host || "同源控制接口"`。
- **锁定**：`TestConsoleEndpointLabelNotHardcoded` 断言页面**不含** `control.DefaultAddr`，且**必须**引用 `location.host`。
- **验证**：修复后 1920/1280/1366/1024 截图均显示真实 `127.0.0.1:17789`，离线态显示 `127.0.0.1:17790`。

> 该方法在实现中途还暴露出一个更值得记录的工程事实：`go build -o` 的目标二进制**正在运行时不会被替换**（Windows 文件占用），而当时的构建命令被整条后台化，导致「构建看起来成功、实际跑的还是旧二进制」。最终采用「先停进程 → 构建 → grep 校验内嵌字符串 → 再启动」的顺序消除了这一假阳性。详见 §11 工具链注意事项。

---

# §10 发现（F 系列）

| 编号 | 发现 | 等级 | 处置 |
| --- | --- | --- | --- |
| **F-1** | **难度调整实际不生效：`bits` 恒为 16，`difficulty` 恒为 1。** `pow.AdjustBits` 的上行路径被 `if newBits > MaxTargetBits { newBits = MaxTargetBits }` 截断，而 `MaxTargetBits` 同时又是「最低难度」；下行的 `newTarget > MaxTarget()` 分支同样返回 `MaxTargetBits`。两侧钳位把结果夹在 16，**快速连挖 21 个区块（远快于 60s/块）后 `bits` 实测仍为 16** | P2 | **未修改**（越界）。UI 显示的是真实值，且 KPI hint 明示 `bits=16 · 相对最低难度`。建议单独立项裁决：这属于「难度机制未生效」而不是「显示错误」 |
| **F-2** | 控制台与节点同源进程：节点停机后**无法重新加载页面** | P3 | 架构必然，已在 §8 如实记录。若需「节点全停也能看到离线页」需引入独立静态宿主，属产品决策 |
| **F-3** | 日志等级为关键词推断，非节点原生字段 | P3 | 三处显式标注（代码注释 / 页面脚注 / 本报告），未伪装成原生能力 |
| **F-4** | `cmd/node/main.go` 存在历史 gofmt 差异（`initDone = true` 注释对齐），非本次引入 | P3 | **未触碰**（属并发工作文件，避免污染无关 diff） |

---

# §11 未完成项 / 阻塞项（诚实清单）

1. **规格 §25 起缺失**：原文在 `### Data` 处截断。本阶段因此**无法核对完整验收标准**，故状态记为「实现完成」而非「阶段 DONE」。补发后可直接对照验收，本报告结构已按「可追加」组织。
2. **`RUN-AUDIT-2026-09-12.md` / `internal/storage/datalock.go` 的改动非本阶段产出**（并发工作），本次未触碰。
3. **BRAND-0D §7.5 / 0D.2 K-1..K-7 用户访谈**仍未执行 → 页面信息架构未经真实用户验证。
4. **文档命名错位**仍未裁决（`PHASE-BRAND-0-D-FOUNDATION.md` 是 BRAND-1 规格；0D.1/0D.2/0D.3 文件名与内容错位一格）。
5. **权限边界**：控制接口**零鉴权**且绑定回环。控制台是同一接口的又一个消费者，**没有引入新的暴露面**，但也没有解决鉴权问题——远程访问仍需 TLS + 认证（本阶段范围外）。
6. **P3.1 仍未提交**；本阶段 **0 次 Git 操作**。

**本阶段工具链注意事项（供后续复用）**

- `go build -o <目标>` 在目标 exe 正在运行时**不会替换**该文件；构建前后必须校验（例如 grep 内嵌字符串），否则会误判「新代码已生效」。
- Git Bash 下 `taskkill //F //PID` 会被原样传递而报「无效参数」，需 `MSYS_NO_PATHCONV=1 taskkill /F /PID <pid>`。
- 写文件到工作目录**之外**会被沙箱静默丢弃（退出码仍为 0）；临时产物请写 `%TEMP%`。
- 无头 Chrome 连续多次调用需为每次指定独立 `--user-data-dir`，否则并发实例争用同一 profile 会导致截图静默失败。

---

# §12 阶段状态

```text
HARD BASELINE              = PASS（继承 0D.2 基线）
D-1 / D-2 / D-3 裁决        = 已落实
BACKEND MINIMAL CHANGE     = 完成（仅 bits/difficulty + /logs，共识零改动）
CONSOLE UI                 = 完成（零依赖 · 内嵌 · 诚实空态）
LAUNCH HOST (node ui)      = 完成
UNIT / REGRESSION TESTS    = 148 个测试函数全绿；build / vet / race 通过
REAL-DATA E2E              = PASS=28 / FAIL=0
VISUAL QA                  = 5 断点 + 离线态 + file:// 回退，通过
DEFECT FOUND & FIXED       = 1（端点硬编码，已加测试锁定）
OPEN FINDINGS              = F-1(P2) / F-2(P3) / F-3(P3) / F-4(P3)
GIT OPERATIONS             = 0

IMPLEMENTATION             = COMPLETE
ACCEPTANCE                 = INCOMPLETE（规格 §25 起缺失）
PHASE STATUS               = 不宣称 DONE
STOP                       = YES
```

**下一步待用户裁决**：① 补发规格 §25 起的验收标准；② F-1 难度调整是否单独立项；③ 文档命名错位是否授权修正。

---

# §13 — 阶段对齐（2026-09-30 追加）

> **文件性质**：本文件是 BRAND-0D.2 实现阶段的**时点记录**（含「GIT OPERATIONS = 0」等当时事实）。后续阶段产生的新事实以本节 **追加**，**不回改原文**。

## 13.1 本阶段交付物现态核验（实测）

| 交付物 | 是否在 HEAD `154de22…` 中 | 证据 |
|---|---|---|
| `internal/control/console.go` | ✅ | `git cat-file -e HEAD:…` 通过 |
| `internal/control/web/console.html` | ✅（35,351 B；本阶段记录 25,194 B，后续迭代增长） | 同上 |
| `internal/control/logs.go` | ✅ | `git ls-files` |
| `cmd/node/logring.go` / `logring_test.go` | ✅ | `git ls-files` |
| `cmd/node/openurl.go` | ✅ | `git ls-files` |
| `cmd/node/cli.go` 的 `ui` 子命令 | ✅ | `cli.go:136` `if cmd == "ui"` |

**构建核验**：`go build ./...` → exit 0（Go `go1.27.0 windows/amd64`，当前工作副本 `E:/wakuang/p2pchain`）。

## 13.2 本阶段之后的界面演进（不属于本阶段产出，但同属「界面说明」范围）

1. **控制面扩容**：路由由本阶段的 8 条增至 **13 条**（新增 `/blocks`、`/mine/start`、`/mine/stop`、`/console/mine`，根路径 `/` 提供 Console 页面）。
2. **鉴权落地**（PHASE CONTROL-AUTH-1）：mutation 端点改由 `requireAuth` 保护，Bearer Token + `crypto/subtle` 常量时间比较，未配置时 fail-closed。
3. **控制台出块通道**（PHASE CONSOLE-MINE-AUTH-FIX-1）：新增 `/console/mine`，用 `consoleOriginGate`（`Sec-Fetch-Site` / `Origin` 同源判定）替代凭据——浏览器零凭据红线保持，CLI 侧 `/mine` + Bearer 语义不变。
4. **第二个 UI 面：Explorer V1**（独立阶段产出）：`internal/explorer/ui/`（29,563 B，`go:embed`）+ `cmd/explorer` 独立二进制，默认 `127.0.0.1:9091` → 上游 control `http://127.0.0.1:17881`；只读白名单 3 条 + mutation 白名单 2 条，token 服务端注入，白名单外 `/api/*` 本地 404、**上游零接触**；零外部资源。
5. **页面迭代**：`console.html` 由 25,194 B 增至 35,351 B；`unavail()` 空态渲染路径保留（19 处调用点），10 项无源指标仍为 `Unavailable`。

## 13.3 仓库与治理现态（影响「文档如何被保存/传播」）

```text
HEAD        = 154de22b661f98f6f09e96e4b5dcaa2d26f3b683   （main，71 commits）
gitea       = ssh://git@192.168.3.123:22/zxzjxx/wakuang.git        ← 154de22…
.200 镜像    = http://192.168.3.200:3000/zxzjxx/wakungzuixin.git    ← 154de22…（ls-remote 实测）
origin      = https://github.com/zxz88601151/p2p                    ← 可达但 PUBLIC 且空仓
```

- **显式路径提交策略（MF-2）**：永久禁用 `git add .` / `-A` / `--all`；由 `scripts/git-guard.sh`（151 行，纯 staged-list 断言）+ `scripts/git-guard-test.sh`（36/36 PASS，纯函数、零 Git 写入）机器强制。
- **`.gitignore`**（39 行 / 679 B）：已追加治理阻断路径 `/.workbuddy/` `/gui/` `/audit-run/` `/f5-verify/` `/gui-test/` 与凭据文件名（`control-token` / `token` / `token.local`）。
- 本文件的「GIT OPERATIONS = 0」是**本阶段时点事实**；后续把包括本文件在内的 19 条路径纳入版本库的提交（`154de22…`）由**独立授权阶段**完成，不是本阶段产出。

## 13.4 仍未闭合项（承接 §11 / §12）

| 项 | 状态 | 备注 |
|---|---|---|
| 规格 §25 起补发 | **未闭合** | ⇒ `ACCEPTANCE = INCOMPLETE`，阶段**不宣称 DONE** |
| F-1 难度调整未生效（`bits` 恒 16） | **未闭合** | 需单独立项裁决，属共识域 |
| 文档命名错位（0D / 0D.1 / 0D.2 / 0D.3） | **未闭合** | 需授权后统一整理 |
| 控制面 LAN/公网暴露（TLS + 认证） | **未闭合** | 本机回环绑定前提下的独立项 |
| `internal/blockchain/query.go` stat 异常 | **保持原样，禁止触碰** | `git status` 显示 ` M` 但 `git diff` 为 0 行（`i/lf w/lf` 全等），属 stat 缓存假象；`checkout`/`add` 会“修好”它从而湮灭证据 |
| `docs/DETERMINISTIC-SERIALIZATION-SPEC.md`（A4） | **BLOCKED，未暂存/未提交** | 待 D-DOC 文档权威裁决后处置 |

## 13.5 阅读指引

- 想看**当时为什么 BLOCKED** → 读 `PHASE-BRAND-0D.2-UI-CONVERGENCE-BASELINE.md` 原文 §1–§8。
- 想看**本阶段实现了什么** → 读本文件 §1–§12。
- 想看**当前阶段的实际现态** → 读本文件 §13 与 BASELINE 的 §0。
