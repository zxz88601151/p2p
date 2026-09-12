# PHASE BRAND-0D.3 — P2PCHAIN DEVELOPER CONSOLE VALIDATION, HARDENING & PRODUCT BASELINE

**阶段报告 / PHASE BASELINE REPORT**

> 项目：P2PChain（Go 1.22.12，纯标准库、零第三方依赖）
> 阶段性质：VALIDATION / HARDENING / BASELINE（READ-BEFORE-WRITE）
> 报告日期：2026-09-12
> 本阶段**未开发新功能、未做 UI 改版、未修 F-1、未重构 Desktop Shell、未引入任何桌面框架**。

---

## 1. Executive Summary

本阶段对 **P2PChain Developer Console** 做了一次「真实节点 + 真实浏览器」的全链路验证与加固，目标是证明 `UI ↓ API ↓ Node ↓ Blockchain` 每一层都是真实的，而不是一个漂亮的仪表盘。

| 项目 | 结果 |
|---|---|
| 真实浏览器 + 真实节点断言 | **109 / 109 PASS，0 FAIL** |
| Go 测试包（`go test ./...`） | **12 / 12 ok，0 failed** |
| Go 竞态测试（`go test -race ./...`） | **12 / 12 ok，0 failed** |
| 构建 / 静态检查 | `go build ./...` OK / `go vet ./...` 干净 |
| CLI = API = UI 三方一致 | **PASS**（height / tip / bits / difficulty / mempool / peers / mining） |
| stderr = API = UI 日志一致 | **PASS**（末行逐字节相同） |
| 生产路径假数据 | **0 处** |
| UI 轮询 | 2000ms / 轮，每轮 1×`/status` + 1×`/logs`，单飞去重 |
| 进程内存（空闲） | 14,680 KB → 14,740 KB（**+0.4%**） |
| F-1（难度调整失效） | **OPEN / NOT MODIFIED** |
| F-2（节点托管 UI 的冷启动边界） | **DEFERRED / NOT MODIFIED** |
| Git 操作 | **0 次**（未 add / commit / push / tag / merge） |

**本阶段发现并修复了 2 个真实 UI 缺陷**（均属「UI 正确性 / 错误处理」范畴，非新功能）：

- **D-1｜`hidden` 语义被 CSS 覆盖**：`.grid3{display:grid}` 是作者样式，覆盖了浏览器 UA 样式表的 `[hidden]{display:none}`，导致节点离线时 `panels.hidden = true` 但面板**依旧可见**——离线卡片与陈旧指标同时出现。修复：加入 `[hidden]{display:none !important}`（不新增任何视觉样式）。
- **D-2｜读取请求无超时**：`/status`、`/logs` 的 `fetch` 没有超时。一旦接口「连得上但不回」，请求永久挂起；由于轮询采用单飞去重，一次挂起会让之后所有 tick 复用同一个未落定的 Promise，**轮询彻底停摆，UI 永久停留在陈旧的 Online**——正是 §11 禁止的 stale status。修复：读取请求加 4s `AbortController` 超时（`/mine` 出块耗时不确定，**不加**超时）。

**最终决定：PASS**（详见第 20 节）。

---

## 2. HARD BASELINE

| 项 | 值 |
|---|---|
| 项目路径 | `C:\Users\Administrator\Desktop\挖矿\p2pchain` |
| 操作系统 | Windows 11（10.0.22631） |
| CPU | Intel Xeon E5-2680 @ 2.70GHz（16 逻辑核） |
| 内存 | 32,699 MB 总量 / 约 19,792 MB 可用 |
| Go 版本 | `go1.22.12 windows/amd64` |
| Rust 版本 | `rustc 1.98.0` / `cargo 1.98.0`（存在但本阶段未使用） |
| 仓库 | Git 仓库（`git status` 可用，非 ABSENT） |
| 分支 | `main` |
| HEAD | `6c0ced873b11569021ac2efded78d8d82596bd04` |
| HEAD 提交信息 | `docs: PHASE P2.1 执行报告 + MASTER-DESIGN 状态回写` |
| HEAD^ | `321f964be689e04de043bd10d02114ed570d9788` |
| 工作区（`git status --short`） | 7 个已修改 + 20 项未跟踪（清单见第 19 节） |
| 暂存区 | **空**（`git diff --cached` 无输出） |
| 远端 | 无 |

**工具链命令（原样执行，未凭猜测添加）**

```
git status --short
git branch --show-current
git rev-parse HEAD
git rev-parse HEAD^
go build ./...
go vet ./...
go test ./...
go test -race ./...
go build -o <新文件名>.exe ./cmd/node
```

> **工具链坑（本阶段再次确认，重要）**：Windows 下 `go build -o` **不会替换正在运行的 exe**，且可能不报错。上一轮验证中 `p2pchain-0d3.exe`（17:32）与项目根 `node.exe`（00:22）就是两个不同版本的产物。本阶段一律**构建到新文件名**并核对嵌入内容后再运行验证。

**代码规模基线**

| 项 | 数量 |
|---|---|
| 生产 `.go` 文件 | 31 |
| 测试文件 | 25 |
| 测试函数（`func Test…`） | 154 |
| `go.mod` 依赖 | 0（仅 `module p2pchain` + `go 1.22`，无 `require`） |

---

## 3. BRAND-0D.2 Verification

对上一阶段（BRAND-0D.2 UI CONVERGENCE & IMPLEMENTATION）的产物做**基于代码而非报告**的复核：

| 0D.2 声称的产物 | 代码复核 | 结论 |
|---|---|---|
| 内嵌单页控制台 | `internal/control/console.go`：`//go:embed web/console.html` + `var consoleHTML []byte`，`/` 与 `/console` 返回 `text/html; charset=utf-8`，非 GET/HEAD 返回 405 + `Allow: GET` | ✅ 存在且生效 |
| 后端最小改动 `bits` / `difficulty` | `internal/control/server.go`：`StatusInfo.Bits uint32 json:"bits"`、`Difficulty float64 json:"difficulty"`；`cmd/node/nodeapi.go`：`Bits: bits`、`Difficulty: relativeDifficulty(bits)` | ✅ 存在且生效 |
| 日志端点 `/logs` | `internal/control/logs.go`：`LogEntry{Time,Level,Component,Message}`、`MaxLogTail = 500`、`defaultLogTail = 100`、无 provider 时返回 `[]`；`cmd/node/logring.go`：`consoleLogCapacity = 600` | ✅ 存在且生效 |
| `node ui` 启动入口 | `cmd/node/cli.go` 子命令分派 + `cmd/node/openurl.go` | ✅ 存在且生效 |
| 无假数据 | 见第 5 节（生产路径 0 处） | ✅ 仍然成立 |

**0D.2 报告中的 F-1 / F-2 判定，本阶段维持**：F-1 = 独立的协议整改项；F-2 = 已知的架构性限制。本阶段对二者**只记录、不修改**（第 15、16 节）。

**文档命名错位（沿用既有事实，本阶段未改名）**：`PHASE-BRAND-0-FOUNDATION.md` 正文标题为 BRAND-1；`PHASE-BRAND-0D-DEVELOPER-DESKTOP.md` 为 BRAND-0D.1；`PHASE-BRAND-0D.1-PRODUCT-VALIDATION.md` 为 BRAND-0D.2；`PHASE-BRAND-0D.2-MVP-BOUNDARY.md` 为 BRAND-0D.3。文件名与正文阶段号不一致属于历史遗留，改名会破坏既有引用，故不在本阶段处理。

---

## 4. Data Contract Audit

**规则**：生产 UI 上出现的每一个字段，必须能追溯到真实代码；否则一律 `Unavailable`，**绝不猜、绝不用 0 冒充**。

### 4.1 最终数据映射表

| UI 字段 | 来源（真实代码） | API | 真实 | Fallback |
|---|---|---|---|---|
| Node 状态（Online/Offline） | 浏览器对 `/status` 的实际请求结果 | `GET /status` | 真实（由请求成功/失败推导，非 API 字段） | 失败态卡片（三态：`Node Offline` / `控制接口错误` / `响应格式非法`） |
| Block Height（`mHeight`/`bHeight`） | `StatusInfo.Height` ← `chain.Height()` | `.height` | 真实 `int` | `Unavailable`（`—`） |
| Chain Tip（`bTip`） | `StatusInfo.TipHash` ← `chain.Tip().Header.HashHex()` | `.tip_hash` | 真实 `string` | `—`，并隐藏「复制」按钮 |
| Latest Block（最新区块时间） | **无来源** | `/status` 无该字段 | ❌ 不存在 | **`Unavailable`（显式声明）** |
| Sync（同步进度） | **无来源**（无 IBD 指标） | `/status` 无该字段 | ❌ 不存在 | **`Unavailable`（显式声明）** |
| Difficulty（`mDiff`/`bDiff`） | `StatusInfo.Difficulty` ← `relativeDifficulty(chain.CurrentBits())` = `2^(bits-16)` | `.difficulty` | 真实 `float64`（派生展示量） | `Unavailable` |
| Bits（`fBits`/`sDiff`/`bDiff`） | `StatusInfo.Bits` ← `chain.CurrentBits()`（共识真值） | `.bits` | 真实 `uint32` | `Unavailable` |
| Peers（`mPeers`/`nPeers`） | `StatusInfo.Peers` ← `net.PeerAddrs()` | `.peers` | 真实 `[]string` | `Unavailable` |
| Mempool（`mPool`） | `StatusInfo.MempoolSize` ← `pool.Len()` | `.mempool_size` | 真实 `int` | `Unavailable` |
| Mining（`mMining`/`sMining`） | `StatusInfo.Mining` ← `mining.Load()` | `.mining` | 真实 `bool` | `Unavailable` |
| Address（`fNode`） | `StatusInfo.Address` ← `miner.Address()` | `.address` | 真实 `string` | `节点未返回钱包地址` |
| Logs（`#log`） | 日志环形缓冲（容量 600）← `log.SetOutput(io.MultiWriter(os.Stderr, ring))` | `GET /logs?tail=N` | 真实 `[]LogEntry` | `暂无日志` |
| Log Level | **关键词推断** `inferLevel(message)`（Go 标准库 `log` 无级别概念） | `.level` | 派生（**非权威**，已在代码注释 + 页脚 + 本报告标注） | — |
| Endpoint（`endpointLabel`） | `location.host`（页面自身地址） | 无（浏览器自身） | 真实 | `同源控制接口` |
| 轮询时间（`lastSync`） | 浏览器本地时间 | 无 | 真实 | — |

### 4.2 结论

- 生产 UI 字段 **100% 可追溯到真实代码**。
- **2 个字段显式 `Unavailable`**（最新区块时间、同步进度）——宁缺毋假。
- 「Difficulty」是 `bits` 的**派生展示量**，不参与任何共识判断；页面同时展示 `bits=16`，使派生量可被独立核对（`UI diff='×1（bits=16）'`）。

---

## 5. Fake Data Audit

**全局检索范围**：`*.go`（排除 `_test.go`）、`internal/control/web/console.html`。
**检索关键词**：`Math.random` / `random` / `fake` / `mock` / `fixture` / `demo` / `sample` / `placeholder` / `hardcoded`，以及 `fakePeers` / `fakeHashrate` / `fakeBlocks` / `fakeDifficulty` / `fakeSync` / `fakeTransactions` / `fakeNode` / `fakeNetwork`。

| 检索位置 | 命中 | 性质 |
|---|---|---|
| 生产 `*.go` | **1 处**：`internal/control/console.go:18` 注释「无 Math.random、无硬编码"生产状态"」 | 注释（说明性），非数据路径 |
| `console.html` | **1 处**：第 346 行注释「本文件不含 Math.random / 无硬编码运行时数据」 | 注释（说明性），非数据路径 |
| `fakePeers` / `fakeHashrate` / `fakeBlocks` / `fakeDifficulty` / `fakeSync` / `fakeTransactions` / `fakeNode` / `fakeNetwork` | **0** | — |

**测试夹具的边界**：`fakeNode` / `fakeLogs` / `fakeChain` 等全部定义在 `*_test.go` 内，仅通过 `httptest.Server` 或内存桩参与测试，**不进入任何生产代码路径**。
`cmd/node/main.go` 中的 `var testPanicAtStart func()` 是**生产代码中恒为 nil 的测试钩子**（仅测试可赋值），源码中已有注释说明。

**判定：`Production fake-data paths = 0`** → 未触发 §23 「发现生产假数据 → 立即 STOP」条件。

---

## 6. API Contract Audit

**方法**：真实节点启动后，用原始 HTTP 客户端逐项探测（不改动任何 API 语义）。

| 请求 | 状态码 | Content-Type | Allow | 正文 | 判定 |
|---|---|---|---|---|---|
| `GET /` | 200 | `text/html; charset=utf-8` | — | 34,145 B 内嵌控制台 | ✅ |
| `GET /console` | 200 | `text/html; charset=utf-8` | — | 同上（同一页） | ✅ |
| `HEAD /` | 200 | `text/html; charset=utf-8` | — | 0 B | ✅ |
| `GET /status` | 200 | `application/json; charset=utf-8` | — | 206 B | ✅ |
| `GET /logs` | 200 | `application/json; charset=utf-8` | — | JSON 数组 | ✅ |
| `POST /status` | **405** | `application/json; charset=utf-8` | **`GET`** | `{"error":"仅支持 GET"}` | ✅ 只读路由正确拒绝 |
| `GET /nope` | **404** | `text/plain; charset=utf-8` | — | `404 page not found` | ✅ 通配 `/` 未被误用 |
| `POST /mine`（畸形 JSON 体） | **400** | `application/json; charset=utf-8` | — | `{"error":"请求体解析失败: invalid character 'b' …"}` | ✅ 错误态有结构 |
| `GET /logs?tail=0` | 400 | JSON | — | 错误对象 | ✅ 非法参数拒绝 |
| `GET /logs?tail=-5` | 400 | JSON | — | 错误对象 | ✅ |
| `GET /logs?tail=abc` | 400 | JSON | — | 错误对象 | ✅ |
| `GET /logs?tail=999999` | 200 | JSON | — | ≤ 500 条（`MaxLogTail` 截断） | ✅ 上界生效 |

**`/status` JSON 形状与类型**

```
keys      = [address, bits, difficulty, height, mempool_size, mining, peers, tip_hash]
typecheck = height=int, tip_hash=str, peers=list, mining=bool, bits=int, difficulty=int
```

**空态 / 错误态**：无日志来源时 `/logs` 返回 `[]`（而非 `null`）；未知路由 404 且 Content-Type 为 `text/plain`（不伪装成 JSON）——两者均已被 `internal/control` 测试锁定。

**结论：本阶段只做 Bug 修复 / 错误处理 / 契约澄清，未为测试而改动任何 API 语义。**

---

## 7. Node Lifecycle Validation

| 生命周期 | 触发方式 | 观测结果 |
|---|---|---|
| 启动 | `node -listen … -rpc … -datadir …` | 端口就绪后 `/status` 200，页面显示 `Online`，指标为真实值（height=21） |
| 运行中 | 真实挖矿 21 块 | 页面高度随链推进（`0 → 1 → 2`，见 §12），无假状态 |
| 加载后停机 | 真实 `taskkill /F` 杀死进程，端口释放 | 页面在 **4.0 s** 内**自行**转为 `Offline`（上界 = 轮询 2 s + 读超时 4 s）；离线卡片出现、指标面板 `display:none`、数值退化为 `—`、副标题降级为「最后读数」 |
| 重启 | 同一 `datadir` 重新启动 | 200 OK；**链高 2 → 2**（持久化生效），→ 点「重试连接」后 **0.0 s** 回到 `Online` |

**是否伪造状态**：`setOnline(true/false)` 仅在**真实请求结果**分支里被调用（成功路径 / catch 路径），页面中**不存在** `setTimeout(() => Online)` 之类的伪状态推进。**判定：无伪造生命周期。**

**已知运维事实（非缺陷，记录）**：`taskkill /F` 强杀不会走 `datalock` 的 `Release` 路径，`<datadir>/node.lock` 会残留，节点按设计拒绝再次启动（`O_CREATE|O_EXCL` 独占，绝不覆盖他人锁），并在错误信息中给出运维指引。本阶段按该指引清理残留锁后重启成功。**该锁语义属 `internal/storage/datalock.go`，而该文件当前正由并行工作改动中，本阶段未做任何修改。**

---

## 8. Blockchain Data Validation

**三方一致性（CLI = API = UI）** —— 真实节点、真实链：

| 指标 | CLI（`node status` / `printchain`） | API（`GET /status`） | UI（页面文本） | 一致 |
|---|---|---|---|---|
| Block Height | `高度: 0` | `height: 0` | `mHeight = "0"` | ✅ |
| Chain Tip | 同下哈希 | `tip_hash: 0000aca1af720da39038…` | `bTip = 0000aca1af720da39038…` | ✅ |
| Bits（难度位） | `难度位 : 16` | `bits: 16` | `fBits = "bits=16"` | ✅ |
| Difficulty | （派生量，不经 CLI 输出） | `difficulty: 1` | `bDiff = "×1（bits=16）"` | ✅ |
| Mempool | `交易池: 0 笔待打包` | `mempool_size: 0` | `mPool = "0"` | ✅ |
| Peers | `对等节点: 0 个 []` | `peers: []` | `mPeers = "0"` | ✅ |
| Mining | `挖矿状态: …` | `mining: false` | `mMining = "IDLE"` | ✅ |
| Endpoint | `-rpc 127.0.0.1:17889` | 服务监听同址 | `endpointLabel = 127.0.0.1:17889` | ✅ |

**21 块真实挖矿后的复核**：`height=21`，`bits=16`，`difficulty=1`，`printchain` 22 条记录（含创世）的难度位集合 = `[16]`。

**结论：`CLI value = API value = UI value`，无任何差异。未通过 UI 格式化隐藏差异。**

---

## 9. Log Validation

真实执行「启动 → 挖矿 → 停机 → 重启」后，取同一时刻的三方日志：

| 通道 | 观测 |
|---|---|
| 节点 stderr（落盘） | **10 行** |
| `GET /logs?tail=200` | **10 条** |
| 页面 `#log` | 10 行；`logMeta = "10 行 · 等级为推断值"` |

**末行逐字节一致**：

```
stderr = 挖到新区块: 高度=2 哈希=00009bf32f36c7da6a52b01e7205c66ef35a2fc7b623d…
API    = 挖到新区块: 高度=2 哈希=00009bf32f36c7da6a52b01e7205c66ef35a2fc7b623d…
UI     = 挖到新区块: 高度=2 哈希=00009bf32f36c7da6a52b01e7205c66ef35a2fc7b623d…
```

- UI 组件名 `miner`、等级 `INFO` 均**来自原文解析**，不是页面编造。
- 等级是 `inferLevel` 的**关键词推断值**，页面页脚与代码注释均声明其为非权威。
- **日志未被重新编造。**

---

## 10. Endpoint Validation

| 场景 | 启动方式 | 端口 | 页面端点标签 | 源码残留 | 判定 |
|---|---|---|---|---|---|
| 默认端口 | `node`（**不传** `-rpc`） | `6689` | `127.0.0.1:6689` | 源码中 `127.0.0.1:6689` 命中 **0** | ✅ |
| 主用端口 | `node -rpc 127.0.0.1:17889` | `17889` | `127.0.0.1:17889` | 0 | ✅ |
| 第三个不同端口 | `node -rpc 127.0.0.1:17891` | `17891` | `127.0.0.1:17891` | 0 | ✅ |

- 端点标签实现：`$("endpointLabel").textContent = location.host || "同源控制接口"` → 标签**始终等于页面自身地址**，换端口启动时不会显示错误地址。
- 三个端口下均验证：控制接口可用 → CLI 可读取状态（rc=0）→ 页面 `Online` → 标签等于实际端口。
- **生产路径无 `127.0.0.1:6689` 硬编码残留**（静态检查 0 处 + 运行时逐端口复核 0 处）。

---

## 11. Offline / Error Validation

### 11.1 畸形 / 错误响应（桩服务，7 种模式）

| 模式 | 模拟内容 | 页面状态 | 失败态标题 | 字面 `null`/`undefined`/`NaN`/`[object` |
|---|---|---|---|---|
| `ok` | 正常响应 | `Online` | — | 无（`mHeight=3` 真实值） |
| `shape_invalid` | 各字段为 `null` / 类型错误 | `Offline` | `响应格式非法` | 无 |
| `no_height` | 缺 `height` | `Offline` | `响应格式非法` | 无 |
| `bad_json` | 200 但正文非 JSON | `Offline` | `响应格式非法` | 无 |
| `empty_body` | 200 且正文为空 | `Offline` | `响应格式非法` | 无 |
| `array_body` | 200 但正文是数组 | `Offline` | `响应格式非法` | 无 |
| `http500` | HTTP 500 | `Offline` | `控制接口错误` | 无 |
| **`hang`** | **连得上但永不回**（先 Online 再切挂起） | `Offline`（**5.3 s**，上界 2 s+4 s） | `Node Offline`，详情含「…ms 内无响应」 | 无 |

> `hang` 用例必须是「**先 Online，再切挂起**」：若直接以挂起模式加载页面，页面启动即 `setOnline(false)`，状态本来就是 `Offline`，「0 s 转 Offline」是**空断言**，证明不了超时生效。本阶段的 harness 已按此修正。

### 11.2 真实场景

| 场景 | 结果 |
|---|---|
| Running | `Online`，全部指标为真实值 |
| Node stopped after page loaded | 4.0 s 内自行转为 `Offline`；面板真正隐藏（`display=none`）；数值退化为 `—`；副标题保留「最后读数」；同步标签变「最后尝试」 |
| API unavailable（过载/无响应） | 经读超时降级为失败态，**不长期停留在陈旧 Online** |
| Invalid API response | `响应格式非法` |
| No peers | `mPeers=0`，副标题「无活跃对等节点」（不是 `Unavailable`，因为 `0` 是**真实读数**） |
| Mining inactive | `mMining=IDLE`（真实布尔） |

**全部场景均未把 `null` / `undefined` / `NaN` / 假 `0` 显示为正常状态**（逐场景对 `document.body.innerText` 做字面量扫描，命中 0）。

---

## 12. UI Interaction Validation

| 交互 | 断言 | 实测 |
|---|---|---|
| 立即出块（单击） | 点击后同步读：按钮禁用 + loading 文案 + `/mine` 恰好 1 次 | `disabled=True`、`出块中…`、`n=1` |
| 出块真实推进 | 高度 +1，且 UI 显示与链一致 | `0 → 1` |
| 立即出块（双击） | 只 1 次 `/mine`、只出 1 个块 | `n=1`，高度 `1 → 2` |
| 刷新（连击 3 次） | **确定性测量**：同一 eval 内连点 3 次后立即读计数 | `n=1`（单飞生效）、`disabled=True`、`刷新中…` |
| 刷新（长窗口交叉验证） | 6.2 s 纯轮询基线 vs 同长窗口+刷新，差值 ≤ 1 | 纯轮询=3，刷新后=4 |
| 刷新结束后 | 按钮恢复可用与文案 | `刷新` |
| 重试（节点仍停机） | loading 反馈 + 禁用 + 只 1 次 `/status` | `重试中…`、`disabled=True`、`n=1` |
| 重试（节点已恢复） | 恢复 `Online` 且指标为真实值 | 0.0 s 恢复，`mHeight="2"` |
| 复制（成功路径） | 明确成功反馈 + 自动复位 | `已复制` → 1.2 s 后 `复制` |
| 复制（失败路径） | 显式报错，**不得静默、不得伪成功** | `复制失败：denied`，按钮不显示「已复制」 |

**未发现**重复请求、陈旧状态、假成功。

> **测量方法学修正（重要）**：上一轮 harness 用「固定等待 3.5 s 后读一次」+「固定 2.2 s 窗口计数」的方法，被 2 s 轮询相位污染，一度误报失败。本阶段改为：① 单飞用「同一 eval 内连点 3 次后立即读计数」的**确定性测量**（`click()` 同步执行到首个 `await`，且 `disabled` 按钮不再派发 click 事件）；② 生命周期用「轮询到状态变化并记录延迟」替代固定等待。修正后同一实现稳定通过。

---

## 13. Visual Regression

5 个视口，逐项检查横向溢出与关键区块越界（要求 `scrollWidth ≤ innerWidth`，且各区块右边界不越界）：

| 视口 | `scrollWidth / innerWidth` | 关键区块 | header 与 metrics 重叠 | 截图 |
|---|---|---|---|---|
| 1024×768 | 1009 / 1024 | 均存在、不越界 | 无 | `vr-1024x768.png` |
| 1280×720 | 1265 / 1280 | 均存在、不越界 | 无 | `vr-1280x720.png` |
| 1366×768 | 1351 / 1366 | 均存在、不越界 | 无 | `vr-1366x768.png` |
| 1440×900 | 1440 / 1440 | 均存在、不越界 | 无 | `vr-1440x900.png` |
| 1920×1080 | 1920 / 1920 | 均存在、不越界 | 无 | `vr-1920x1080.png` |

补充截图：`offline-after-load.png`（离线陈旧态）、`recovered.png`（重试恢复后）。

**重点压力项复核**（长哈希 / 长日志 / 大高度 / 大难度值 / 空态 / 错误文案）：Chain Tip 为 64 位十六进制显示不换行溢出；日志行长文本 `white-space:pre-wrap` + `word-break:break-word`；空态与错误文案均在失败态卡片内呈现，不撑破布局。

**未新增任何视觉样式**（D-1 修复只加入 `[hidden]{display:none !important}` 一条不可见的语义兜底规则）。

---

## 14. Performance / Resource Check

| 项 | 实测 | 判定 |
|---|---|---|
| UI 轮询间隔 | **2000 ms**，注册点唯一（`setInterval(tick, 2000)`，`strings.Count == 1`） | ✅ |
| 每轮请求数 | 1×`/status` + 1×`/logs` ≈ 1 req/s/页面 | ✅ |
| 6.2 s 内 `/status` 轮询 | 3 次（理论 3，±1） | ✅ |
| `/logs` 与 `/status` 同步 | `status=3, logs=3` | ✅ |
| 日志环形缓冲上界 | `consoleLogCapacity = 600`（静态断言） | ✅ |
| `/logs` 返回上界 | `?tail=999` → 10 条；`?tail=999999` → ≤ 500（`MaxLogTail` 截断） | ✅ **日志不会无界增长** |
| 进程工作集（空闲期） | 14,680 KB → 14,740 KB（**+0.4%**，阈值 <15%） | ✅ 稳定 |
| 空闲 CPU | 无持续算力占用（未启用 `-mine`） | ✅ |

**未引入任何性能框架**，仅使用既有 go test / 原生 HTTP / 进程工作集观测。

---

## 15. F-1 Status

**`F-1 STATUS = OPEN`　`F-1 = NOT MODIFIED`**

### 15.1 一手证据（本阶段真实观测，非引用旧报告）

在真实节点上执行 `node mine -count 21`：

| 观测点 | 值 |
|---|---|
| 起始 | `height=0`, `bits=16`, `difficulty=1` |
| `mine -count 21` 返回 | `已挖出 21 个区块，当前高度 21`（rc=0） |
| 21 块后 `/status` | `height=21`, `bits=16`, `difficulty=1` |
| `printchain` 全链 | 22 条记录（含创世），**难度位集合 = `[16]`**（唯一值） |
| 共识常量（源码真值） | `MaxTargetBits=16`、`DifficultyAdjustmentInterval=20`、`TargetBlockTimeSeconds=60` |

**结论**：链高 21 **已跨过难度调整周期 20**，但全链 `bits` 恒为 16、`difficulty` 恒为 1 → **难度调整未生效，确认 F-1 现象可复现且未修复。**

### 15.2 隔离声明

- 本阶段**未修改** `internal/pow/**`（目标计算 / `AdjustBits` 上下界钳制）、**未修改**共识校验、**未修改**区块验证、**未修改**任何数据库结构。
- `git status --short` 中不存在 `internal/pow/` 或共识模块的改动。
- F-1 被登记为**独立的协议整改项**，本阶段只做记录。

### 15.3 F-1 REMEDIATION RECOMMENDATION（**仅写方案，未执行**）

1. **定位**：在 20 块边界处读取 `AdjustBits` 的输入输出（实际耗时 vs `TargetBlockTimeSeconds`）与钳制分支，确认是「实际耗时未达调整阈值」还是「钳制上下界把结果夹回 16」。
2. **最小修复**：只改 `AdjustBits` 的钳制与边界触发条件，不改 `BitsToTarget` 与共识校验路径。
3. **验证**：新增「跨 20 块边界后 `bits` 必须变化」的回归用例（先红后绿），并保留 `bits` 单调性与上下界不变量测试。
4. **风险**：难度改变会影响已构造候选区块的有效性，必须与 `tipChanged` 中断逻辑联合评审。
5. **纪律**：需**单独授权**的 PHASE 才能执行（本阶段 §9 明确禁止修复）。

---

## 16. F-2 Status

**`F-2 STATUS = DEFERRED`　`F-2 = NOT MODIFIED`**

### 16.1 一手证据（三分支）

| 分支 | 操作 | 观测 |
|---|---|---|
| Node running → UI 可用 | 节点运行中打开页面 | `Online`，`mHeight=21`（真实链高） |
| **Node 未启动 → 浏览器连接失败** | 导航到无监听端口 | `title='127.0.0.1'`、正文首行 `无法访问此网站`、`document.getElementById("nodeStateText")` 为 `false` → **页面根本不是来自节点，控制台无法冷启动** |
| Node 加载后停止 → Offline UI 可用 | 先加载再杀进程 | 2.8 s 内转为 `Offline`，`panels.display='none'`，离线详情含真实端点 |

### 16.2 隔离声明

- 本阶段**未引入** Tauri / Electron / Wails / 独立 Web Server / 后台 Daemon / 桌面服务。
- 未改动 UI 托管方式：仍为「零依赖 Go 进程 + `go:embed` 内嵌页面」，UI 由节点自身托管。
- F-2 被登记为**延后的架构性议题**（属于「节点托管 UI」这一既定架构的固有边界）。

### 16.3 说明

F-2 的等价表述是：**本控制台是「节点的一个界面」，不是「独立于节点存在的应用」**。这是产品定位的直接结果（Developer Console 服务于 Node Operator），不是缺陷。若要支持「节点未启动也能看到界面」，必须引入独立宿主——那属于**被明确禁止的范围扩展**。

---

## 17. Tests

### 17.1 Go 测试（既有 + 本阶段新增）

```
go test ./...         → 12/12 包 ok，0 failed
go test -race ./...   → 12/12 包 ok，0 failed
```

| 包 | 测试函数数 | 结果 |
|---|---|---|
| `cmd/node` | 27 | ok（22.3 s；`-race` 44.8 s） |
| `internal/block` | 8 | ok |
| `internal/blockchain` | 9 | ok |
| `internal/control` | 24 | ok |
| `internal/mempool` | 9 | ok |
| `internal/p2p` | 5 | ok |
| `internal/pow` | 17 | ok |
| `internal/storage` | 12 | ok |
| `internal/transaction` | 5 | ok |
| `internal/txbuild` | 5 | ok |
| `internal/utxo` | 16 | ok |
| `internal/wallet` | 17 | ok |
| `internal/config` | 0（无测试文件） | — |
| **合计** | **154 个测试函数 / 25 个测试文件** | **0 failed** |

**本阶段新增 2 个 Go 回归断言**（`internal/control/console_test.go`，该包由 22 → 24）：

- `TestConsoleHiddenAttributeIsEffective`：对所有通过 `$("x").hidden` 切换可见性的元素，解析其 class 是否声明了 `display`；若声明则要求存在 `[hidden]{display:none !important}` 兜底。**这是对 D-1 的永久性防线**（因为只读 `.hidden` DOM 属性的检查是**空断言**）。
- `TestConsoleReadRequestsHaveTimeout`：要求 `jget` 具备 `AbortController` + `REQ_TIMEOUT_MS` + `ctl.abort()` + `clearTimeout` + `AbortError` 分支；并**反向断言** `/mine` 不得携带该读超时信号（出块耗时不确定）。

**反向控制（非空断言证明）**：临时移除修复后二者均 **FAIL**，且报错指名具体原因——

```
移除 [hidden] 兜底规则 →
FAIL: 以下 class 声明了 display，会覆盖 UA 的 [hidden]{display:none}…: [grid3]
      修复：在样式表中加入 [hidden]{display:none !important}

移除 AbortController →
FAIL: jget 缺少读取超时防护（接口挂起会让单飞轮询永久停摆）: "AbortController"
```

### 17.2 真实浏览器 + 真实节点 harness

**109 / 109 PASS，0 FAIL**（分节：A 畸形/错误响应 25 项、B 三方一致 14 项、C 交互加固 14 项、D 日志一致 5 项、E 资源 3 项、F 3 秒可理解性 1 项、G 视觉回归 15 项、H 生命周期 16 项、I 端点 10 项、其余为 A2/F 等专项）。

**无 skipped 用例**。失败项：**无**。

> 测试装置本身的可信度也做了修正：曾出现 3 处「空断言 / 被相位污染」的测量（面板隐藏只读 DOM 属性、连击刷新固定窗口差值、挂起用例未先置 Online），均已改为确定性测量后重跑通过——**不是把断言放宽，而是把测量方法改对**。

---

## 18. Build

| 命令 | 结果 |
|---|---|
| `go build ./...` | OK（无输出） |
| `go vet ./...` | OK（无输出） |
| `go build -o <新文件名>.exe ./cmd/node` | OK（8,903,680 B），并核对嵌入内容含 `REQ_TIMEOUT_MS` 与 `[hidden]{display:none !important}` |
| `gofmt -l cmd internal` | 仅剩 2 个**非本阶段改动**的文件 |

**`gofmt -l` 残留说明（均为既有 / 并行工作，本阶段未触碰）**：

- `cmd/node/main.go` —— 既有格式差异（`initDone = true` 附近对齐），非本阶段引入。
- `cmd/node/lock_lifecycle_test.go` —— 未跟踪文件，属并行进行中的 datalock 工作。

本阶段自己改动的 `internal/control/console_test.go` 曾被 `gofmt` 指出文件末多余空行，**已修正**。

**未通过猜测添加任何工具链**；所有命令取自项目实际环境。

---

## 19. Git Status

**本阶段执行了 0 次 Git 写操作**：未 `add`、未 `commit`、未 `push`、未 `tag`、未 `merge`、未 `rebase`、未 `amend`、未 `squash`。

```
分支        : main
HEAD        : 6c0ced873b11569021ac2efded78d8d82596bd04
HEAD^       : 321f964be689e04de043bd10d02114ed570d9788
HEAD 信息   : docs: PHASE P2.1 执行报告 + MASTER-DESIGN 状态回写
暂存区      : 空
远端        : 无
```

`git status --short`（与阶段开始时**完全一致**）：

```
 M cmd/node/cli.go
 M cmd/node/main.go
 M cmd/node/nodeapi.go
 M docs/RUN-AUDIT-2026-09-12.md
 M internal/control/client.go
 M internal/control/server.go
 M internal/storage/datalock.go
?? cmd/node/lock_lifecycle_test.go
?? cmd/node/logring.go
?? cmd/node/logring_test.go
?? cmd/node/openurl.go
?? docs/PHASE-BRAND-0-FOUNDATION.md
?? docs/PHASE-BRAND-0D-DEVELOPER-DESKTOP.md
?? docs/PHASE-BRAND-0D.1-PRODUCT-VALIDATION.md
?? docs/PHASE-BRAND-0D.2-MVP-BOUNDARY.md
?? docs/PHASE-BRAND-0D.2-UI-CONVERGENCE-BASELINE.md
?? docs/PHASE-BRAND-0D.2-UI-CONVERGENCE-IMPLEMENTATION.md
?? docs/PHASE-BRAND-0D.3-TECHNICAL-FOUNDATION.md
?? docs/PHASE-BRAND-1-DISCOVERY.md
?? docs/design/
?? internal/control/console.go
?? internal/control/console_test.go
?? internal/control/logs.go
?? internal/control/web/
?? internal/storage/datalock_p3_test.go
```

**本阶段实际改动的文件（全部位于未跟踪路径内，未触碰任何已跟踪文件）**：

| 文件 | 改动 | 性质 |
|---|---|---|
| `internal/control/web/console.html` | +`[hidden]{display:none !important}`（D-1）；`jget` 增加 4s 读超时（D-2） | UI 正确性 / 错误处理 |
| `internal/control/console_test.go` | +2 个回归断言；修正文件末空行（gofmt） | 测试 |
| `docs/PHASE-BRAND-0D.3-DEVELOPER-CONSOLE-BASELINE-REPORT.md` | 本报告 | 文档 |

> 默认 **NO COMMIT**。如后续需要落库，需由 `PHASE BRAND-0D.3-COMMIT` 单独授权。

---

## 20. Final Decision

# ✅ PASS

### 20.1 判定依据（逐条对齐 §24 的 PASS 条件）

| PASS 条件 | 结论 | 证据 |
|---|---|---|
| 所有既有测试 PASS | ✅ | `go test ./...` 12/12 ok，0 failed（154 个测试函数） |
| 新增测试 PASS | ✅ | `internal/control` 22 → 24（+2），全过；且经反向控制证明非空断言 |
| race PASS | ✅ | `go test -race ./...` 12/12 ok，0 failed（`cmd/node` 44.8 s） |
| build PASS | ✅ | `go build ./...` OK；`go vet ./...` 干净 |
| 真实数据映射 PASS | ✅ | 第 4 节全字段可追溯；2 字段显式 `Unavailable` |
| 无生产假数据 | ✅ | 第 5 节：`Production fake-data paths = 0` |
| UI 交互 PASS | ✅ | 第 12 节：单飞 / 禁用 / loading / 成功与失败反馈均通过 |
| 视觉回归 PASS | ✅ | 第 13 节：5 视口无溢出、无重叠 |
| F-1 已隔离 | ✅ | `OPEN / NOT MODIFIED`，第 15 节 |
| F-2 已隔离 | ✅ | `DEFERRED / NOT MODIFIED`，第 16 节 |
| 无范围扩散 | ✅ | 未新增页面 / 钱包 / Token / Explorer / 交易市场 / 数据库 / 协议 / 共识 / 桌面框架 |

**未触发任何 §23 STOP 条件**：无既有回归、无生产假数据、无 API 不一致、无 CLI-API-UI 不一致、无协议副作用、未改共识、未改 PoW、无数据库迁移、无架构扩张、无桌面框架扩张。

### 20.2 已修复的真实缺陷（本阶段价值）

| 编号 | 缺陷 | 影响 | 修复 |
|---|---|---|---|
| **D-1** | `.grid3{display:grid}` 覆盖 UA `[hidden]{display:none}` | **离线态视觉错误**：节点离线时面板仍可见，与离线卡片并存 | 加 `[hidden]{display:none !important}` |
| **D-2** | `/status`、`/logs` 无读超时 | **stale status**：接口「连得上不回」时单飞轮询永久停摆，UI 永久停留陈旧 Online | 4s `AbortController` 超时（`/mine` 不加） |

两个缺陷均由**真实浏览器 E2E** 发现（D-1 由「只看 DOM 属性」的空断言掩盖，D-2 由「固定等待」的测量方式掩盖），修复后均已用**反向控制**固化为永久回归断言。

### 20.3 已知非阻塞限制（明确记录，不降低本阶段判定）

1. **F-1 = OPEN**：难度调整未生效（跨过周期 20 后 `bits` 恒为 16）。这是**产品的协议层缺陷**，但被 §9 显式划为**独立整改项**，且 §24 的 PASS 条件明确包含「F-1 已隔离」——故不 downgrade。整改方案见 15.3，**未执行**。
2. **F-2 = DEFERRED**：节点未启动时无法加载 UI（页面由节点自身托管）。§10 与 §24 同样将其列为可隔离项。**未引入任何独立宿主**。
3. **强杀后 `node.lock` 残留**：`datalock` 的设计行为（`O_CREATE|O_EXCL`，绝不覆盖他人锁），错误信息自带运维指引。**非缺陷**，且相关文件由并行工作改动中，本阶段未触碰。
4. **文档文件名与正文阶段号错位**：历史遗留，改名会破坏既有引用，未处理。

> 上述第 1、2 项之所以不使判定变为 PARTIAL：§24 把「F-1 已隔离」「F-2 已隔离」**写入 PASS 条件本身**，即规范预期二者与 PASS 相容；且本阶段的验收目标是「控制面可信」，而控制面对 F-1 的职责是**如实呈现**（页面同时给出 `bits=16` 与派生难度 `×1`，可被独立核对），该职责已完整履行。若中哥希望按更严格的读法把产品级已知缺陷整体计入 PARTIAL，可在下一阶段指令中覆盖此判定——本报告的原始证据已完整保留，判定可无损重算。

### 20.4 产品基线确认

**`P2PChain Developer Console Baseline`**

| 维度 | 基线 |
|---|---|
| Product Category | **Developer Console** |
| Primary Role | **Developer / Node Operator** |
| Architecture | **Go Node + Embedded HTML Console**（零第三方依赖，`go:embed`，由节点自身托管） |
| Data | **仅真实后端 / API 数据**；不可得则 `Unavailable` |
| Visual | **Quiet / Technical / Dense / Inspectable**（本轮未新增任何视觉样式） |
| Protocol Changes | **NONE** |
| Economic Layer | **NONE** |
| Wallet Expansion | **NONE** |
| 轮询基线 | 2000 ms，每轮 1×`/status` + 1×`/logs` |
| 日志上界 | 环形缓冲 600，`/logs` 返回 ≤ `MaxLogTail`(500) |
| 读取超时 | 4000 ms（仅 `/status`、`/logs`；`/mine` 不加） |
| 失败态语义 | 三态：`Node Offline` / `控制接口错误` / `响应格式非法` |

**一句话：让 P2PChain Developer Console 成为一个可信的工程控制面，而不是一个漂亮的区块链仪表盘。**

---

**阶段结束。按 §25 立即停止——不自动修复 F-1 / F-2，不新增界面、页面、钱包、Token、Explorer、网络功能；任何后续推进需新的阶段授权。**
