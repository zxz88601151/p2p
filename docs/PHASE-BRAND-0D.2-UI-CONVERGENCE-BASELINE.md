# PHASE BRAND-0D.2 — P2PCHAIN DEVELOPER CONSOLE UI CONVERGENCE
## BASELINE & EXISTING-STATE AUDIT（实现前阻断报告）

> 项目：P2PChain
> 阶段性质（规格自述）：UI PRODUCT CONVERGENCE / EXISTING-CODE IMPLEMENTATION / READ-BEFORE-WRITE
> 本报告覆盖规格 §1（HARD BASELINE）、§2（READ AUTHORITY）、§3（EXISTING UI AUDIT）。
>
> **本阶段结论：**
>
> ```text
> HARD BASELINE              = PASS
> §3 EXISTING UI AUDIT       = NO DESKTOP UI EXISTS（规格前提不成立）
> DATA SOURCE COVERAGE       = 部分成立（约一半指标无真实来源）
> IMPLEMENTATION             = BLOCKED（待 3 项决策 + 规格 §25 补发）
> PRODUCTION CODE CHANGED    = 0 行
> GIT OPERATIONS             = 0 次
> ```

---

# §1 — HARD BASELINE

## 1.1 环境

| 项 | 实测值 |
|---|---|
| Project path | `C:\Users\Administrator\Desktop\挖矿\p2pchain` |
| OS | `MINGW64_NT-10.0-22631` (Windows 11 build 22631), x86_64 |
| CPU | 16 vCPU |
| 内存 | **未采集**（`wmic` 被本机安全策略拦截，不重试） |
| Go | `go1.22.12 windows/amd64` |
| 项目技术栈 | **纯 Go + 标准库，零第三方依赖**（`go.mod` 无 `require` 段） |
| 环境内其他运行时 | Node `v24.19.0`、Rust `rustc 1.98.0`、Python `3.13.14` —— **均存在，但本项目一个都没用** |
| 编译产物 | `node.exe`（8,715,776 bytes） |

## 1.2 仓库

```text
GIT = PRESENT
root    = C:/Users/Administrator/Desktop/挖矿/p2pchain
branch  = main
HEAD    = 6c0ced873b11569021ac2efded78d8d82596bd04
parent  = 321f964be689e04de043bd10d02114ed570d9788
```

`git status --short`：

```text
 M cmd/node/main.go                      ← 并发工作（P3.1），非本阶段
 M docs/RUN-AUDIT-2026-09-12.md          ← 并发工作
 M internal/control/server.go            ← 并发工作
 M internal/storage/datalock.go          ← 并发工作
?? cmd/node/lock_lifecycle_test.go       ← 并发工作
?? docs/PHASE-BRAND-0-FOUNDATION.md
?? docs/PHASE-BRAND-0D-DEVELOPER-DESKTOP.md
?? docs/PHASE-BRAND-0D.1-PRODUCT-VALIDATION.md
?? docs/PHASE-BRAND-0D.2-MVP-BOUNDARY.md
?? docs/PHASE-BRAND-0D.3-TECHNICAL-FOUNDATION.md
?? docs/PHASE-BRAND-1-DISCOVERY.md
?? docs/design/
?? internal/storage/datalock_p3_test.go   ← 并发工作
```

**并发工作已隔离**：本阶段未 restore / clean / reset / checkout / delete / overwrite，且不把并发代码当作能力证据。

## 1.3 构建与测试（实测，非假设）

```text
go build ./...           → PASS (exit 0)
go vet ./...             → PASS (exit 0，零告警)
go test ./... -count=1   → PASS（12/12 包 ok，internal/config 无测试文件）
scripts/smoke-e2e.sh     → 存在（真实双进程 E2E 脚本）
测试函数总数             → 129
```

```text
ok  p2pchain/cmd/node               21.807s     ok  p2pchain/internal/pow          2.146s
ok  p2pchain/internal/block          0.264s     ok  p2pchain/internal/storage      0.478s
ok  p2pchain/internal/blockchain     4.241s     ok  p2pchain/internal/transaction  0.234s
ok  p2pchain/internal/control        0.260s     ok  p2pchain/internal/txbuild      0.264s
ok  p2pchain/internal/mempool        0.243s     ok  p2pchain/internal/utxo         0.264s
ok  p2pchain/internal/p2p            0.709s     ok  p2pchain/internal/wallet       0.364s
```

## 1.4 当前启动方式 / UI 入口 / 后端入口

**当前"启动方式"（两个 .bat，双击运行）：**

```bat
rem start-node-a.bat  ← 矿工节点
node.exe -listen 127.0.0.1:6688 -rpc 127.0.0.1:6689 -mine -miners 2 -datadir run-a

rem start-node-b.bat  ← 全节点，从 A 播种
node.exe -listen 127.0.0.1:16690 -rpc 127.0.0.1:16691 -seed 127.0.0.1:6688 -datadir run-b
```

> 注意：这是**进程启动脚本**，不是 UI 启动器。

**当前 UI 入口点：**

```text
NONE
```

**后端入口点（真实存在，共两套）：**

1. **CLI**：`node [选项]` 与子命令 `status / balance / utxos / send / mine / wallet / printchain / help`
   节点选项：`-listen -rpc -seed -datadir -mine -maxblocks -miners`
2. **HTTP 控制 API**（`internal/control`，JSON over HTTP，默认 `127.0.0.1:6689`，**零鉴权**）：
   `GET /status` · `GET /balance` · `GET /utxos` · `POST /send` · `POST /mine` · `GET /block`

## 1.5 基线判定

```text
HARD BASELINE = PASS
```

构建与既有测试全绿 → 不触发规格 §1 的 STOP 条件。

---

# §2 — READ AUTHORITY（规格要求区分 AUTHORITATIVE / CURRENT / LEGACY / DEFERRED / ARCHIVED）

| 文档 | 分类 | 结论 |
|---|---|---|
| `DEVELOPMENT_PROMPT.md` | **AUTHORITATIVE（根意图）** | 项目 = 类比特币 P2P PoW 链；定位「技术验证 / 学习性质的测试网络骨架」；交付面 = CLI。**全文未出现任何 UI / Desktop 需求** |
| `MASTER-DESIGN.md` | **AUTHORITATIVE（工程决策记录）** | 「学习型 PoW 区块链骨架；标准库零依赖；**禁止引入区块链以外的功能**」；「明确不做：… TLS/加密传输；代币经济」；**无任何 UI 章节** |
| `PHASE-0-ENGINEERING-BASELINE-REPORT.md` | CURRENT（历史证据） | 工程基线 |
| `PHASE-0.1-GATE-A-R-POW-DIFFICULTY-REMEDIATION-REPORT.md` | CURRENT（历史证据） | PoW 难度修复 |
| `PHASE-1A-CORE-CONSENSUS-UTXO-TRANSACTION-AUDIT.md` | CURRENT（历史证据） | 共识/UTXO 审计 |
| `PHASE-P2.1-EXECUTION-REPORT.md` | CURRENT（历史证据） | datadir 锁 |
| `RUN-AUDIT-2026-09-12.md` | CURRENT（并发工作 P3.1 规格） | 锁生命周期加固 |
| `PHASE-BRAND-0-FOUNDATION.md` | **ARCHIVED** | VERA 命名研究，已被 BRAND-0D 正式废弃，不得作为要求 |
| `PHASE-BRAND-0D-*.md` / `0D.1-*` / `0D.2-*` / `0D.3-*` / `1-*` | CURRENT（产品方向研究） | 见下方 §2.2 命名错位 |
| **本规格（BRAND-0D.2 UI Convergence）** | **CURRENT（本阶段指令）** | 即本次任务授权 |

## 2.1 权威冲突 C-1（必须先裁决）

三份文件给出**三个互斥的产品目标**：

```text
A. MASTER-DESIGN.md（工程权威，2026-09-12）
   → 学习型 PoW 骨架；标准库零依赖；禁止区块链以外功能；无 UI；明确不做 TLS
      ⇒ 隐含结论：不做 Desktop UI

B. PHASE BRAND-0D.1（产品研究，本轮会话结论）
   → 产品类别 = Developer Control Center
     对象 = 机器舰队 / Agent 会话 / Event / Receipt（"for your own machines & agent workloads"）
     明确把 mining UI 列入 NOT-MVP

C. 本规格（§4）
   → 产品类别 = P2PChain Developer Console
     对象 = Node / Blockchain / Network / Validation / Mining / Diagnostics / Logs
     明确 NOT: Crypto Wallet / Web3 Dashboard / Blockchain Explorer / Consumer Crypto App
```

> **A、B、C 无法同时为真。** A 说不做 UI，B 说做"机器/会话"控制中心，C 说做"节点/链"控制台。B 与 C 的对象集几乎不重叠。
>
> §2 明文要求：「不得把旧报告中的设计方案直接当作当前要求」。因此在裁决前，**不得擅自选定其一开工**。

## 2.2 文档命名错位（已核实，影响 §2 权威判定）

逐文件核对首行后发现系统性错位：

```text
PHASE-BRAND-0-FOUNDATION.md            → 内容实为 BRAND-1 规格
PHASE-BRAND-0D-DEVELOPER-DESKTOP.md    → 内容实为 BRAND-0D.1 规格
PHASE-BRAND-0D.1-PRODUCT-VALIDATION.md → 内容实为 BRAND-0D.2 规格（截断于 §21 — KIL）
PHASE-BRAND-0D.2-MVP-BOUNDARY.md       → 内容实为 BRAND-0D.3 规格（1,233 行）
PHASE-BRAND-1-DISCOVERY.md             → BRAND-1 执行报告
```

**并且**：本规格自编号为 `PHASE BRAND-0D.2`，与既有 `PHASE-BRAND-0D.2-MVP-BOUNDARY.md` 编号直接冲突 → 若按编号查找，会读到另一份完全不同的文档。

**建议**：本报告暂以 `PHASE-BRAND-0D.2-UI-CONVERGENCE-BASELINE.md` 命名，避免二次覆盖（需授权后再统一整理命名）。

---

# §3 — EXISTING UI AUDIT

## 3.1 UI Entry

| 检索项 | 结果 |
|---|---|
| Desktop entry | ❌ 无 |
| Main window / App shell / Renderer | ❌ 无 |
| HTML / JS / TS / CSS / Vue / Svelte | ❌ 无 |
| `package.json` | ❌ 无 |
| Electron / Tauri / Wails 引用 | ❌ 无（全仓 grep 零命中） |
| WebView | ❌ 无 |

**p2pchain 文件类型全量统计（`.git` 除外）：**

```text
50  .go        16  .md        2  .bat        1  .sh        1  .mod        1  .exe        1  .gitignore
```

**⇒ 结论：EXISTING DESKTOP UI = 不存在。**

全工作区唯一的 HTML 文件是我上一轮放在**仓库之外**的 `devcontrol-center-prototype.html`（不属于 p2pchain，也不属于本规格的对象）。

## 3.2 UI Components

| 组件 | 是否存在 |
|---|---|
| Header / Navigation / Dashboard / Status | ❌ |
| Mining / Node / Network 面板 | ❌ |
| Logs / Settings / Charts / Tables / Dialogs | ❌ |

**现有的全部"用户可见面"只有两种：**

```text
1. CLI 文本输出（stdout）
2. HTTP JSON 响应（控制 API）
```

## 3.3 前提证伪

规格开篇写的是「将当前 P2PChain Desktop 的**现有 UI**，从原有的泛化/矿工导向界面，**收敛**为 Developer Console」，并声明「本阶段不是重新开发 P2PChain」。

**但：现有 UI 不存在，因此"收敛"这一动作没有对象。** 本阶段若继续，实际性质是「**从零建立** UI」，与 §0 的"不是重新开发"直接矛盾。

> 这条不是我的推断，是文件系统事实：仓库里没有任何一行前端代码。

---

# §4 — DATA SOURCE MATRIX（规格 §3 要求的表）

逐字段追溯到真实来源。**判定规则：没有真实来源的字段一律不得作为真实数据展示（规格 §19 / §3）。**

| UI Data | Current Source | Real? | Safe to Display |
|---|---|---|---|
| Node online/offline | 控制 API `/status` 可达性 + 进程存在 | ✅ | ✅ |
| Block height | `/status.height` | ✅ | ✅ |
| Chain tip hash | `/status.tip_hash` | ✅ | ✅（monospace + Copy） |
| Peers | `/status.peers[]`（地址字符串数组） | ✅ | ✅ |
| Peers 数量 | `len(peers)`（派生） | ✅ | ✅ |
| Inbound / Outbound | —— 无该维度 | ❌ | ❌ **只能 Unavailable** |
| Peer latency | —— 无采集 | ❌ | ❌ **只能 Unavailable** |
| Mempool size | `/status.mempool_size` | ✅ | ✅ |
| Mining on/off | `/status.mining`（bool） | ✅ | ✅ |
| Node wallet address | `/status.address` / `p2p` nodeID（二者同源） | ✅ | ✅ |
| Balance (spendable/total/utxo_count) | `/balance` | ✅ | ✅ |
| UTXO 列表 | `/utxos` | ✅ | ✅ |
| Block by height | `/block?height=`（hex） | ✅ | ✅ |
| Difficulty / Bits | `chain.CurrentBits()` / `Header.Bits` 存在，但**仅 CLI `printchain` 离线可见**，**未暴露到 API** | ⚠️ | ⚠️ 需最小后端补充才可显示 |
| **Hashrate** | —— 全仓零命中（`DEVELOPMENT_PROMPT` 任务卡 6 曾要求，未交付） | ❌ | ❌ **只能 Unavailable** |
| **Sync %** | —— 无该概念（无 IBD 进度指标） | ❌ | ❌ **只能 Unavailable** |
| **Temperature / Power / Fan / GPU 设备** | —— 全仓零命中 | ❌ | ❌ **只能 Unavailable** |
| **Network name（MAINNET/TESTNET）** | —— 无网络标识字段（只有确定性 genesis） | ❌ | ❌ **只能 Unavailable** |
| Logs | 真实 stdout；`log.Printf("[node] …")` / `[p2p]` / `[miner]` | ⚠️ | ⚠️ 有 component 前缀、有默认时间戳，**无 level** |
| Protocol version | —— p2p 消息无版本字段 | ❌ | ❌ **只能 Unavailable** |
| Data directory | CLI `-datadir` 实际值 | ✅ | ✅ |

## 4.1 覆盖度

```text
✅ 有真实来源、可直接展示      ：Node状态 / Height / TipHash / Peers / Peers数 /
                                Mempool / Mining / Address / Balance / UTXO / Block / Datadir   （12 项）
⚠️ 存在但未暴露，需最小后端补充：Difficulty(Bits) / 日志等级化                                  （2 项）
❌ 无任何来源，只能 Unavailable：Hashrate / Sync% / Inbound-Outbound / Latency /
                                Temperature / Power / Fan / Device / Network名 / Protocol版本    （10 项）
```

**⇒ 规格 §9（含 Hashrate、Sync）、§11（含 Inbound/Outbound/Latency）、§12（含 Temperature/Power/Fan/Device）四块示例中，超过半数指标在本项目里根本没有数据源。** 按 §19 与 §3 的硬规则，这些只能渲染为 "Unavailable / Not implemented"，不得用随机数或硬编码伪装。

---

# §5 — 规格自检：缺陷与张力清单

| 编号 | 缺陷 | 影响 |
|---|---|---|
| **S-1** | 规格在 **§25 — ACCEPTANCE CRITERIA** 的 `### Data` 处**被截断** | §25 的 Data 段与可能的 §26+ 未知；验收标准不完整，无法据以判定 PASS |
| **S-2** | 编号冲突：本规格自编号 `BRAND-0D.2`，与既有 `PHASE-BRAND-0D.2-MVP-BOUNDARY.md` 相撞（且后者内容实为 0D.3 规格） | 按编号检索会读到错误文档 |
| **S-3** | 产品类别与 `BRAND-0D.1`（Developer Control Center）互斥；与 `MASTER-DESIGN`（无 UI、零依赖）存在张力 | 无法在未裁决的情况下开工 |
| **S-4** | §20 允许「UI components / CSS / HTML / renderer」，但项目**当前不存在 renderer**；且 §1 要求发现 Electron/Tauri 栈 —— 实际为零依赖纯 Go。引入任一前端外壳都会**新增依赖**，与 `MASTER-DESIGN` 的"标准库零依赖"纪律冲突，属需要显式授权的事项 | 技术栈未定，实现无法起手 |
| **S-5** | §22 要求 `aria-label` / 对比度 / 状态不只依赖颜色，但未定义目标平台与无障碍标准级别 | 验收口径不明（次要） |

---

# §6 — 决策请求（实现前必须裁决）

## D-1 产品权威（最高优先级）

```text
选项 A：本规格为准 —— 做 P2PChain Developer Console（Node/链/网络/挖矿/日志）
选项 B：BRAND-0D.1 为准 —— 做 Developer Control Center（机器/Agent 会话），本规格归档
选项 C：两者并存但分层 —— Console 是"节点面"，DCC 是"机器与会话面"，共用一套设计语言
```

## D-2 UI 宿主与技术栈

项目现状是**纯 Go 零依赖**，没有前端运行时。可选：

```text
选项 A（与现有纪律一致）：零依赖 —— Go 内置 HTTP 服务 + 内嵌单页 HTML，
                          经 127.0.0.1 复用现有控制 API；浏览器/系统 WebView 打开
                          （新增 `node ui` 子命令；不引入任何第三方依赖）
选项 B：Tauri（Rust 1.98.0 已具备）—— 真桌面窗口，但引入 Rust 工具链 + 前端依赖，
                          打破"标准库零依赖"，且超出 §20 默认禁止的后端/架构范围
选项 C：先只产出静态 HTML 视觉稿（不入仓库、不接线），仅作设计确认用
```

## D-3 最小后端补充授权（§20 允许，需最小化 + 记录契约）

```text
是否允许把以下两项以最小改动暴露到现有控制 API？
  a) difficulty / bits（chain.CurrentBits() / Header.Bits 已存在，仅未暴露）
  b) 日志等级化（当前 log.Printf 无 level，需约定或轻量解析）
```

---

# §7 — 诚实可建范围（若 D-1=A 且 D-2=A）

仅列出**有真实来源**的部分，作为裁决后的实现基线：

```text
Header              节点在线/离线（由 /status 可达性推导）
Primary Control     现有能力只有 Mining 开关（-mine / POST /mine）；
                    不存在 Node Start/Stop API → 不得假装存在（规格 §8）
Core Metrics        Block Height · Peers 数 · Mempool · Mining 状态          [4/5 可实现]
                    Hashrate / Sync → Unavailable                          [不可实现]
Blockchain Panel    Height · Tip Hash(mono+Copy) · 按高度取块 · 难度⚠️        [3/4 + 1 需授权]
Network Panel       Peers 地址列表                                            [1/6 可实现]
                    Inbound/Outbound/Latency/Protocol/ConnectionState → Unavailable
Mining/Hardware     Mining on-off                                             [1/7 可实现]
                    Hashrate/Temperature/Power/Fan/Device → Unavailable
Wallet-side         Address · Balance(spendable,total,utxo_count) · UTXO      [全可实现]
Log Panel           真实 stdout（component 前缀 ✅ / 时间戳 ✅ / level ❌）    [可用，需约定]
Footer              datadir · listen · rpc · genesis · address                [5/8 可实现]
                    Protocol Version → Unavailable
```

**Unavailable 状态必须按 §15 设计为语义诚实的空态，而不是 0 / 0 / 0 / 0。**

---

# §8 — 阶段状态

```text
HARD BASELINE              = PASS
EXISTING UI AUDIT          = NO UI EXISTS（前提不成立，已证伪）
DATA SOURCE AUDIT          = 完成（12 可用 / 2 需授权 / 10 无来源）
AUTHORITY CLASSIFICATION   = 完成（发现互斥冲突 C-1）
PRODUCTION CODE CHANGED    = 0 行
GIT OPERATIONS             = 0 次
IMPLEMENTATION             = BLOCKED

BLOCKED BY:
  D-1 产品权威裁决
  D-2 UI 宿主/技术栈裁决
  D-3 最小后端补充授权
  S-1 规格 §25 起补发
```

**未写任何生产代码，未做任何 Git 操作。**

---

**本报告到此为止。等待 D-1 / D-2 / D-3 裁决与 §25 补发后，方可进入 IMPLEMENTATION。**
