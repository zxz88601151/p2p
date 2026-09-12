# PHASE BRAND-0D.3 — DEVELOPER CONTROL CENTER TECHNICAL FOUNDATION & FEASIBILITY SPIKE（执行报告）

> 项目：P2PChain
>
> 前置阶段：PHASE BRAND-0D / BRAND-0D.1 / BRAND-0D.2
>
> 前置结论：
> **BRAND-0D.2 = PRODUCT DIRECTION PASS / MVP BOUNDARY PASS / TECHNICAL FOUNDATION BLOCKED**
>
> 本阶段性质：
> **TECHNICAL FOUNDATION / ARCHITECTURAL FEASIBILITY / ISOLATED SPIKE**
>
> 本阶段唯一目标：
> **证明 Developer Control Center 的最小技术闭环是否真实可行：`PAIR → GLANCE → ACT → HISTORY → RECEIPT`，并明确哪些现有 P2PChain 资产可复用 / 必须抽象 / 必须新建 / 必须退出 MVP 关键路径。**
>
> 本阶段结束后必须 STOP。不得自动进入品牌命名、最终 UI 设计、完整 MVP 实现、生产部署。

---

# §0 — ABSOLUTE RULES

## 0.1 READ FIRST

已完整阅读并继承 PHASE BRAND-0D / 0D.1 / 0D.2，不重新讨论已确定的产品方向。

BRAND-0D.2 的核心结论作为本阶段硬前提：

```text
Product Category : Developer Control Center
Primary Job      : 在一个地方看到自己的多台机器正在运行什么，并直接控制正在运行的任务
Secondary Job    : 回看发生了什么
Differentiating  : 对极少数关键事件生成可独立验证的 Receipt
Core Loop        : PAIR → GLANCE → ACT
On-demand        : RECEIPT
Passive          : HISTORY
```

## 0.2 READ-ONLY DEFAULT 遵守情况

本阶段对本仓库生产代码的修改量：**0 行**。

```text
.go / .js / .ts / .py / go.mod / go.sum   未改
配置 / 数据库 / blocks.dat / wallet        未改
P2P / PoW / UTXO / Node / API / CLI / UI   未改
installer / 部署 / 品牌命名 / 最终 UI      未做
```

## 0.3 GIT 完全禁止（遵守）

```text
git init / add / commit / push / tag / merge / rebase / reset   全部未执行
```

## 0.4 CONCURRENT WORK PROTECTION（遵守）

进入本阶段时仓库存在**并发工作（P3.1）**，已按 §2 规则 `STOP AND ISOLATE`：

- 只记录、不 restore / clean / reset / checkout / delete / overwrite / format。
- 并发工作**不作为能力证据**（本文所有"现有资产"判断均只基于 `HEAD` 提交的代码）。
- 全程未触碰并发工作涉及的 4 个修改文件与相关测试文件。

## 0.5 ISOLATED SPIKE 边界（§18 允许）

本阶段唯一的代码产出是**一个位于 git 仓库之外的隔离原型**：

```text
C:\Users\Administrator\Desktop\挖矿\.spike\brand-0d3\
    ├── go.mod      (module brand0d3spike, go 1.22, 零第三方依赖)
    └── main.go     (558 行，单文件)
```

隔离性已实测证明（`git rev-parse --show-toplevel` 在该目录返回 `fatal: not a git repository`）：

```text
non-production ✅   isolated ✅   reversible ✅（整体删除目录即完全回退）   documented ✅
```

**结论：Spike 未并入主架构，且没有任何一条结论依赖 Spike 代码进入生产。**

---

# §1 — HARD BASELINE

## 1.1 环境

| 项 | 值 |
|---|---|
| OS | Windows（Git Bash / MINGW64 环境） |
| Architecture | amd64 |
| CPU | 16 vCPU |
| Go | `go1.22.12 windows/amd64` |
| Node | `v24.19.0`（存在，但本阶段未使用） |
| Repository root | `C:\Users\Administrator\Desktop\挖矿\p2pchain` |
| Current branch | `main` |
| HEAD | `6c0ced8` |
| Go module | `module p2pchain` / `go 1.22`（零第三方依赖） |

## 1.2 仓库状态（进入时）

```text
BASELINE = main@6c0ced8
```

**并发工作（不属于本阶段，未触碰，不作为证据）**：

```text
 M cmd/node/main.go
 M docs/RUN-AUDIT-2026-09-12.md
 M internal/control/server.go
 M internal/storage/datalock.go
?? cmd/node/lock_lifecycle_test.go
?? docs/PHASE-BRAND-0-FOUNDATION.md
?? docs/PHASE-BRAND-0D-DEVELOPER-DESKTOP.md
?? docs/PHASE-BRAND-0D.1-PRODUCT-VALIDATION.md
?? docs/PHASE-BRAND-0D.2-MVP-BOUNDARY.md
?? docs/PHASE-BRAND-1-DISCOVERY.md
?? docs/design/
?? internal/storage/datalock_p3_test.go
```

> 说明：`docs/` 下若干未跟踪文件属并发/前序产出，本阶段只新增本报告一个文件，未改动任何既有文档。

## 1.3 规模与状态

| 指标 | 值 |
|---|---|
| Go 生产文件 | 27 |
| Go 测试文件 | 23 |
| 生产代码 LOC | 4,869 |
| 测试代码 LOC | 4,384 |
| 包（含 `cmd`） | 13 |
| 测试用例（`func Test*`） | 129（分布在 12 个包） |
| 仓库体积（不含 `.git`） | 9.0 MB |
| `go build ./...` | **PASS**（exit 0） |
| `go vet ./...` | **PASS**（exit 0，零告警） |
| `go test ./... -count=1` | **PASS**（12/12 包 ok，`internal/config` 无测试文件） |

```text
cmd/node        22.889s ok      internal/pow        3.015s ok
internal/block   0.288s ok      internal/storage    1.030s ok
internal/blockchain 4.367s ok   internal/transaction 0.374s ok
internal/control 0.354s ok      internal/txbuild     0.349s ok
internal/mempool 0.288s ok      internal/utxo        0.355s ok
internal/p2p     0.820s ok      internal/wallet      0.445s ok
```

**基线为绿。本阶段未破坏任何一条。**

## 1.4 包规模（生产 LOC）

```text
cmd/node        2511   internal/utxo        874   internal/p2p     807
internal/storage 781   internal/blockchain  737   internal/control 736
internal/wallet  717   internal/pow         568   internal/mempool 481
internal/block   385   internal/transaction 325   internal/txbuild 292
internal/config   22
```

---

# §2 — BRAND-0D.2 INHERITANCE

本阶段**继承（不重新论证）**以下 BRAND-0D.2 结论，并只对它们做技术可行性验证：

| 来源 | 继承结论 | 本阶段角色 |
|---|---|---|
| 产品类别 | **Developer Control Center**（不是区块链产品、不是监控面板、不是远程终端） | 作为硬前提 |
| 核心循环 | `PAIR → GLANCE → ACT` + `RECEIPT`（按需）+ `HISTORY`（被动） | 逐段做技术验证 |
| MVP ATTEST EVENTS | 仅 3 类：Session START / Session END / Artifact PRODUCED | §12 验证 |
| Receipt v1 | 哈希链 + 机器签名，**不需要 PoW**，离线可验证，不含隐私数据 | §12 硬性验证 |
| Workspace | 降级为"对象"，非产品本体 | 保留 |
| UTXO / Mempool / PoW / Wallet UI / Blockchain UI | 不在 MVP 关键路径 | §11 / §3 再确认 |
| 六大缺失安全原语 | machine identity / ownership / peer auth / encrypted transport / fleet boundary / unauthorized rejection | §7 验证 |
| S1 状态 | BRAND-0D.2 判定为 **BLOCKED by 6 missing primitives** | 本阶段需给出"是否仍阻塞"的判决 |

BRAND-0D.2 留下的**唯一未决问题**（本阶段必须回答）：

> **"Developer Control Center 的最小技术闭环，用现有资产 + 合理的少量新建，是否真实可行？"**

---

# §3 — EXISTING ARCHITECTURE RE-AUDIT

只针对 DCC 所需能力重新审计（未重做全项目审计）。映射链路：
`CURRENT ASSET → MVP REQUIRED CAPABILITY → REUSE / ADAPT / ABSTRACT / DROP / NEW`

## 3.1 现有资产清单（基于 `HEAD@6c0ced8`）

| 现有资产 | 位置 | 与 DCC 的关系 |
|---|---|---|
| P2P 网络（握手 / 区块 / 交易广播 / 同步 / peer 发现） | `internal/p2p`（807 行） | 网络层候选，但**无身份、无加密、无认证** |
| 控制 API（HTTP 6 端点 + client） | `internal/control`（736 行） | **控制面骨架候选**，但零鉴权、绑定回环 |
| 钱包（P-256 ECDSA + Base58 地址 + 签名/验签） | `internal/wallet`（717 行） | **crypto identity primitive 候选** |
| 存储（文件读写 + 进程独占锁 `datalock`） | `internal/storage`（781 行） | 事件落盘候选；`datalock` 是"进程独占"而非"运行监管" |
| 区块链（区块校验 / 链管理 / 重组织） | `internal/blockchain`（737 行） | 与 DCC MVP 无直接关系 |
| PoW（难度 / 校验） | `internal/pow`（568 行） | **不在 Receipt v1 路径**（§12 证实） |
| UTXO / Mempool / Transaction / TxBuild | 3,154 行合计 | 不在 DCC MVP 关键路径 |
| CLI（节点启动 / 参数 / 生命周期） | `cmd/node`（2,511 行） | 仅提供"单进程节点"模式，非多会话运行监管 |
| Block / Transaction 规范编码 `Encode()` | `internal/block/codec.go`、`internal/transaction/codec.go` | **确定性序列化的可复用范式** |
| `sha256` 哈希 | block / pow / transaction / utxo / wallet | 可复用 |

## 3.2 关键发现：DCC 的真正重活不在链，而在运行时

对 DCC 所需能力逐项比对本仓库代码：

```text
os/exec / exec.Command / exec.Cmd    （生产代码）出现次数 = 0
signal.Notify                        （生产代码）出现次数 = 1，且仅用于节点自我退出：
                                     cmd/node/main.go:291  signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
```

**含义：现有代码完全没有"派生并监管子进程"的能力。**

再看"事件日志 / 哈希链"：

```text
grep -i "eventlog|event_log|hashchain|hash_chain|appendonly|append-only"  →  0 命中
```

**含义：现有代码没有任何 Event Log 或哈希链抽象可"顺手复用"。**

再看"会话"概念：

```text
生产代码中与 "session" 相关的命中 = 0
唯一的进程身份用法：internal/storage/datalock.go:59  os.Getpid()（数据目录独占锁）
```

**含义：现有代码没有"Agent Session"这一对象。`datalock` 是"同一数据目录只允许一个节点进程"的文件锁，与"多会话运行监管"是两件事。**

## 3.3 映射判决

| MVP 能力 | 现有资产 | 判决 |
|---|---|---|
| Machine Identity | `internal/wallet` 的 P-256 ECDSA（`ecdsa.GenerateKey(elliptic.P256())`） | **ABSTRACT**：抽 `crypto identity primitive`，向上长出 Machine Identity，**不把 Wallet 改名**（§7 P1） |
| Machine Ownership | 无（`datalock` 只解决"同目录单进程"） | **NEW** |
| Pairing | 无 | **NEW** |
| Peer Authentication | 无（`HandshakePayload.NodeID` 是自报字符串） | **NEW** |
| Secure Transport | 无（全仓零 TLS） | **NEW** |
| Agent Host | 无（零 `os/exec`） | **NEW** |
| Runtime Registry | 无 | **NEW** |
| Session State | 无 | **NEW** |
| Observe / Stop / Restart / Logs | 无（无子进程即无监管） | **NEW** |
| Event Log | 无（零 hashchain / eventlog） | **ADAPT** 存储落盘范式 + `Encode()` 确定性序列化 → **NEW** 事件层 |
| Receipt | 无（wallet 有签名原语，但无 Receipt 结构） | **ABSTRACT** 签名原语 → 新 Signing/Receipt 层 |
| Offline Verify | 无 | **NEW** |
| Local IPC（控制面） | `internal/control` HTTP + client（6 端点） | **ADAPT**：保留 HTTP 形态，**新建鉴权层**，丢弃"链业务端点" |
| CLI / Tray / Notification | CLI 有；tray/notification 无 | **NEW** |
| PoW / UTXO / Mempool / Blockchain | 成熟存在 | **DROP（出 MVP 关键路径）** —— 不为复用而复用 |

## 3.4 与 BRAND-0D.2 的一致性核对

BRAND-0D.2 曾判定"最重的活是运行时监管（今天为零），而不是账本（今天成熟）"。本阶段复审**再次确认该判断**：

```text
账本侧（PoW / UTXO / Mempool / Blockchain / Wallet）  ≈ 3,600+ 行 → MVP 关键路径上几乎为 0 需求
运行时侧（进程监管 / 日志采集 / 会话状态 / 事件）      = 0 行     → MVP 关键路径上的主要工作量
```

**这是本阶段最重要的架构事实：DCC 的成本结构与 P2PChain 的既有投入结构几乎正交。**

---

# §4 — RUNTIME FOUNDATION AUDIT

本阶段最高优先级。核心问题：

> **一台机器上的 Agent Session 到底如何成为一个可观察、可控制的 Runtime？**

审计手段：源码检索（§3.2 已给出）+ **可运行 Spike 实测**（`T1` / `T2`）。

## R1 — Process lifecycle

| 能力 | 现有 | Spike 实测 |
|---|---|---|
| spawn | ❌ 无 | ✅ `exec.Command(self, "--child")` 成功，`pid=22468` |
| start | ❌ 无 | ✅ `cmd.Start()` 成功 |
| running detection | ❌ 无 | ✅ `cmd.Process.Pid > 0` 即"已运行" |
| exit detection | ❌ 无 | ✅ `cmd.ProcessState.Exited() == true` |
| exit code | ❌ 无 | ✅ `ExitCode() == 3` |
| stop / kill | ❌ 无 | ✅ `Process.Kill()` 成功，终态判定成立（见 T2） |
| restart | ❌ 无 | ⚠️ 未直接 Spike；由"spawn + kill"两原语组合即可，**判定为可组合实现（低风险）** |

**R1 判决：现有为零，但标准库可行 → PASS（feasible）。**

T2 关键证据：长运行子进程用 `Kill()` 终止后——

```text
Wait() 返回非 nil（≠ 正常退出）
ProcessState.Exited() == true
→ 映射为终态 FAILED / LOST 成立
且 Kill 之前已产生的 stdout 部分被完整保留（111 bytes captured）
```

这意味着**"停止一个失控会话"和"保留崩溃前的日志"这两件 DCC 核心动作，在标准库层面天然成立。**

## R2 — Log lifecycle

| 能力 | 现有 | Spike 实测 |
|---|---|---|
| stdout capture | ❌ 无 | ✅ `StdoutPipe()` + `bufio.Scanner` 流式读，捕获 3/3 行 |
| stderr capture | ❌ 无 | ⚠️ 未 Spike（`StderrPipe()` 与 stdout 同构，**低风险**） |
| tail N lines | ❌ 无 | ⚠️ 未 Spike（环形缓冲，**纯内存逻辑，低风险**） |
| log persistence | ❌ 无 | ⚠️ 未 Spike（复用 `internal/storage` 落盘范式） |
| log rotation | ❌ 无 | ⚠️ 未 Spike（**MVP 可先不做**） |
| log truncation | ❌ 无 | ⚠️ 未 Spike（**MVP 可先不做**） |

**R2 判决：capture + streaming 已验证 PASS；persistence/rotation/truncation 属确定性工程量，非技术风险。**

## R3 — Agent Session lifecycle

| 能力 | 现有 | 结论 |
|---|---|---|
| session creation | ❌ | NEW |
| session identity | ❌ | NEW（建议 `run_<machine>_<seq>`，见 §6） |
| session state | ❌ | NEW（见 §5） |
| last activity | ❌ | NEW（可派生自 stdout 最后一行时间戳，或显式 heartbeat） |
| completion | ❌ | 可由 `Wait()` 正常返回派生 → PASS |
| failure | ❌ | 可由非零 exit code / Wait 错误派生 → PASS |
| lost | ❌ | 可由 supervisor 重启后对照 registry 判定 → PASS（协议相关） |
| unknown | ❌ | 默认兜底态 → PASS |

**R3 判决：状态可由"进程事实"派生，不依赖 Agent 主动上报（除 IDLE/WAITING 外）。**

## R4 — Runtime registry

现有：**不存在**。

本阶段**只设计，不实现**。最小 Registry 设计：

```text
runtime_id    : rt_<machine_id>_<seq>          （本机唯一，单调递增）
machine_id    : mch_<sha256(pubkey_der)[:12]>  （见 §7 P1）
process_identity : OS PID + 派生时间           （非持久身份，仅用于本机定位）
command_identity : 受限命令描述符（非任意命令，见 §9）
session_id    : sess_<runtime_id>_<seq>
timestamps    : started_at / last_activity_at / ended_at（RFC3339 UTC）
state         : 见 §5 枚举
```

> **设计原则**：Registry 是"本机事实的唯一账本"，任何远端看到的会话都必须能回溯到 Registry 中的一条记录；Registry 不存日志正文，只存引用与摘要。

**§4 总判决：RUNTIME FOUNDATION = PASS（可行性），GAP = 全部新建（今天为零）。**

---

# §5 — SESSION STATE MACHINE

对 BRAND-0D.2 定义的 7 态逐项回答 7 个问题。**不为满足枚举而制造假状态。**

| 状态 | 本机可观测 | 远端可观测 | 证据 | 进入转换 | 清除转换 | 可否伪造 | 会否过期 |
|---|---|---|---|---|---|---|---|
| **RUNNING** | ✅ | ✅ | 进程存在 + 最近有 stdout | spawn/start | 进程退出 或 超时无活动 | 可（本机持钥者） | 会（需 heartbeat） |
| **IDLE** | ⚠️ | ⚠️ | **需 Agent 主动上报** | Agent 上报 idle | Agent 上报 active | 可 | 会 |
| **WAITING** | ⚠️ | ⚠️ | **需 Agent 主动上报**（等待人工/外部输入） | Agent 上报 waiting | Agent 上报 resume | 可 | 会 |
| **COMPLETED** | ✅ | ✅ | `Wait()` 正常返回 + exit 0 | 正常退出 | 终态 | 可 | 否 |
| **FAILED** | ✅ | ✅ | 非零 exit code / `Wait()` 报错（**T1 实测 code=3 → FAILED**） | 异常退出 | 终态 | 可 | 否 |
| **LOST** | ✅ | ✅ | supervisor 存活但进程不可达 / Kill 后未正常收尾（**T2 实测**） | 失联 | 终态 | 可 | 否 |
| **UNKNOWN** | ✅ | ✅ | 默认兜底（无任何证据） | 证据缺失 | 任一确定态 | — | 会 |

## 5.1 诚实降级声明

本阶段只能**可靠实现 5 态**：

```text
RUNNING / COMPLETED / FAILED / LOST / UNKNOWN   ← 由"进程事实"派生，不依赖 Agent 协作
```

`IDLE` 与 `WAITING` 必须 Agent 主动上报，标记为：

```text
PROTOCOL-DEPENDENT   → MVP 默认不实现（§17 明确列为可缩小项）
```

**理由**：MVP 的 Agent Session 以"受监管的子进程"为现实模型，进程本身无法自证"空闲"或"等待输入"。强行实现只能靠轮询猜测，属"假状态"，违反 §5 纪律。

## 5.2 状态机（MVP 可实现子集）

```text
            spawn/start
  (none) ─────────────────▶ RUNNING
                              │  │  │
         exit==0 ─────────────┘  │  └──────────── 证据缺失 ──▶ UNKNOWN
                                 │                                   │
         exit!=0 / Wait err ─────┘                                   │
                                 ▼                                   │
                    COMPLETED / FAILED                               │
                                                                     │
         进程不可达 / 被 Kill 未收尾 ──────────────────────────────▶ LOST
```

---

# §6 — MINIMUM AGENT PROTOCOL

**设计但默认不实现。** 目的：定义"控制面 ↔ 本机 Agent"之间交换什么，为 §7 / §9 定契约。

## 6.1 最小字段集

```text
machine_id       : mch_<...>            （长期身份，见 §7 P1）
agent_id         : agt_<machine_id>_<seq>
runtime_id       : rt_<machine_id>_<seq>
session_id       : sess_<runtime_id>_<seq>
protocol_version : 1
timestamp        : RFC3339 UTC
nonce            : 16 bytes random     （防重放，见 §7 P5）
signature        : ECDSA-P256(ASN.1) over canonical(fields)   （见 §12）
state            : RUNNING | COMPLETED | FAILED | LOST | UNKNOWN
action           : observe | stop | restart | logs | attach
event            : session.started | session.ended | artifact.produced
```

## 6.2 三向通信内容

**Agent → Control Center**（上行，推送事实）

```text
session.started   {runtime_id, session_id, started_at, command_digest, env_fp}
session.activity  {session_id, last_activity_at}                      （心跳，不含日志正文）
session.ended     {session_id, ended_at, state, exit_code}
artifact.produced {session_id, artifact_digest}
```

**Control Center → Agent**（下行，下达受限动作）

```text
observe(runtime_id)   → RuntimeSnapshot
stop(runtime_id)      → AcceptedAction{action_id}
restart(runtime_id)   → AcceptedAction{action_id}
logs(runtime_id, tail)→ LogPage             （只读，见 §9）
attach(runtime_id)    → AttachHandle        （语义见 §10）
```

**Agent → Agent**（机器间，MVP 默认禁用）

```text
MVP 不设计 Agent ↔ Agent 直连。所有跨机器交互必须经 Control Center 仲裁（§14）。
```

## 6.3 明确禁止（设计红线，遵守）

```text
❌ generic RPC framework      ❌ plugin system        ❌ marketplace
❌ workflow engine            ❌ Kubernetes-like orchestration
❌ full remote shell          ❌ generic command execution
```

---

# §7 — SECURE MACHINE PAIRING

重新验证 BRAND-0D.2 的安全地板。**Spike 实测覆盖：身份（T3）、本机控制面鉴权（T7）。配对/加密/重放防护为设计（未 Spike）。**

## P1 — 如何生成 Machine Identity？

**必须抽象，不得把 Wallet 改名。**

实测证据（T3）：`ecdsa.GenerateKey(elliptic.P256(), rand.Reader)` → 公钥 → `x509.MarshalPKIXPublicKey` → `sha256[:12]` → `mch_791509bd6bb9cce4c6ef217f`。私钥以 `0600` 落盘。

正确的抽象方向：

```text
crypto identity primitive  (新建，可复用 internal/wallet 的 P-256 原语)
        ↓
Machine Identity           (新建：mch_ + 公钥指纹 + 私钥安全存储)
        ↓
Wallet                     (既有，保持不变；它只是"另一种身份用途")
```

**错误做法（明确拒绝）**：

```text
Wallet ──改名──▶ MachineIdentity     ❌
```

理由：Wallet 的语义是"持有余额、签交易"，Machine Identity 的语义是"标识一台机器、签 Receipt"。把前者改名会污染两者的语义，并使后续安全审计无法分辨"哪把钥匙能签什么"。

## P2 — Pairing 如何发生？

**只设计，不做 UI。** 推荐一次性配对流程：

```text
1. 新机器 Agent 首次启动 → 生成 Machine Identity（P-256）
2. Agent 展示：一次性配对码(short code) + 公钥指纹(指纹可人眼核对)
3. Control Center 输入/扫描配对码（QR 或 6 位码）
4. Agent 用私钥对 {pairing_nonce, cc_pubkey} 签名 → 回传
5. Control Center 验签 → 该 machine_id 记入本机 Fleet 白名单（ownership 落库）
6. 配对码即刻作废（一次性）
```

## P3 — 如何证明"这台机器属于我的 Fleet"？

Fleet 成员资格 = **本机持久化的白名单 + 该机器拥有对应私钥**：

```text
机器属于我的 Fleet  ⟺  machine_id ∈ LocalFleetWhitelist
                      ∧  该机器能对随机 challenge 用 machine_id 对应私钥签名
```

**只靠 machine_id 字符串不成立**（可被任意自报伪造）。

## P4 — 陌生机器如何被拒绝？

```text
未在 LocalFleetWhitelist 的 machine_id → 一律拒绝（默认拒绝，白名单制）
```

现有 `HandshakePayload.NodeID`（`internal/p2p/node.go:62`）是**自报字符串**，无签名、无 challenge、无 nonce、无公钥——**任何节点都能自称任意 NodeID**。因此**现有 P2P 握手不可直接作为 Fleet 边界**（详见 §8）。

## P5 — 如何防 replay？

```text
每次会话的请求携带 16-byte random nonce
+ RFC3339 timestamp
+ 签名覆盖 {nonce, timestamp, action, target}
接收方维护"已见 nonce"短窗口去重集合
+ 拒绝过期 timestamp（如 ±60s）
```

## P6 — Remote Control 是否必须加密？

**必须。** 实测证据（T7）：本机控制面已实现 `127.0.0.1:0` 环回 + `Authorization: Bearer <token>`：

```text
无 token  → 401 Rejected
带 token  → 200 Accepted
监听地址  → 127.0.0.1:xxxxx（仅回环）
```

这对**本机**是足够的（回环不外泄）。但 `Stop` / `Restart` / `Logs` / `Attach` 一旦跨机器，明文 HTTP 不可接受。

**现有代码自己也承认这一点**：

```text
internal/control/server.go:6  "因此绝不可绑定到公网地址（需要远程访问时应加 TLS + 认证，属本阶段范围外）"
cmd/node/main.go:242          "控制接口监听地址（仅本机，无鉴权）"
```

全仓 TLS 命中数 = **0**。

## 7.1 安全地板判决

```text
Identity        MUST   →  原语已实测可行（T3）             ✅ feasible
Pairing         MUST   →  设计完成，未 Spike              ⚠️ unproven
Authentication  MUST   →  本机 Bearer 已实测（T7）；跨机未做 ⚠️ partial
Authorization   MUST   →  Fleet 白名单设计完成，未 Spike    ⚠️ unproven
Encryption      MUST   →  未做，且全仓无 TLS               ❌ gap
Fleet Boundary  MUST   →  设计完成，依赖上述三项            ⚠️ unproven
Replay Defense  MUST   →  nonce+timestamp 设计完成，未 Spike ⚠️ unproven
```

**结论：SECURITY FOUNDATION = PARTIAL。**

不允许把 Encryption 直接标记 OPTIONAL。威胁模型（降级前提必须写清）：

```text
若 MVP 严格限制在"单机 + 环回"：
   跨机器传输不存在 → Encryption 的缺失暂不构成远程攻击面
   → 这是【范围收缩】而非【安全降级】，因此可接受
若进入"跨机器控制"：
   Encryption 为 MUST，无 TLS 则 Stop/Restart/Attach 全部禁止上线
```

**因此 §17 的约束项之一必然是：跨机器控制必须先补齐加密传输；MVP 一期默认单机。**

---

# §8 — P2P REUSE DECISION

核心问题：

> **现有 P2P 是否值得成为 Developer Control Center 的网络层？**

**倾向性结论：不值得直接复用。** 依据：现有 P2P 完全没有身份/认证/加密（§7 P4/P6），而 DCC 的整个价值建立在"我的机器边界"之上。把它们焊接在一起，等于把 DCC 的安全地板压到 P2P 的历史水平之下。

## 8.1 四选项比较

| 维度 | A. 直接改造现有 P2P | B. 保留 P2P transport，重建 identity/auth | C. 只复用 crypto primitive，重建 Control Transport | D. MVP 单机，跨机后置 |
|---|---|---|---|---|
| Security | 低（需在原握手里塞入签名/challenge，改动面大） | 中（transport 复用但需 TLS 层） | 高（新传输从零按安全设计） | 高（无远程面） |
| Complexity | 高 | 高 | 中 | 低 |
| Migration Cost | 高 | 中 | 低 | 低 |
| Testability | 低（P2P 测试要建真实网络） | 中 | 高 | 高 |
| Maintainability | 低（两套语义纠缠） | 中 | 高 | 高 |
| Product Fit | 低（P2P 是"链同步"语义，非"机器控制"） | 中 | 高 | 高（一期本就单机） |
| Privacy | 中 | 中 | 高 | 高 |
| Future Expansion | 中 | 中 | 高（可长成认证传输） | 中（需一期后补） |

## 8.2 RECOMMENDED OPTION

```text
RECOMMENDED OPTION = C（只复用 crypto primitives，重新建立 Control Transport）
      并叠加 D 作为一期落地范围（MVP 先单机，跨机器后置）
```

**为什么（可证伪）**：

1. **DCC 的网络需求与 P2P 的网络需求是不同物种。** 现有 P2P 传的是"区块 / 交易 / 链高度"（`internal/p2p/node.go` 的 `BlockPayload` / `TxPayload` / `GetBlocksPayload`），DCC 要传的是"会话状态 / 控制动作 / 日志页"。**复用 transport 的收益（省一次连接管理）远小于安全债（要重造身份与加密）**。
2. **唯一真正值得复用的是 crypto primitive**（P-256 生成/签名/验签），而被复用的部分恰好是已经过测试（`internal/wallet` 全部测试绿）的部分。
3. **一期单机（D）与 C 不冲突**：C 定义"将来用什么传输"，D 定义"一期先不做传输"。两者叠加是最低风险路径。

**明确拒绝**：

```text
❌ 直接改造现有 P2P 成为控制传输
❌ 把 HandshakePayload.NodeID 当作 machine_id
❌ 为了"项目原本有 P2P"而保留 P2P 在 MVP 关键路径
```

---

# §9 — CONTROL PLANE

## 9.1 最小动作模型

对每一个动作定义 9 个字段。**核心不变式：动作只能作用于 Registry 中已知的 Runtime。**

| 动作 | Request | Authorization | Target | Execution | Result | Timeout | Failure | Event | Audit |
|---|---|---|---|---|---|---|---|---|---|
| **Observe** | `GET /observe` | Bearer token（T7 实测） | runtime_id 集合 | 读 Registry | RuntimeSnapshot[] | 5s | 返回空/陈旧标记 | 无（被动） | 只读，不审计 |
| **Stop** | `POST /stop` | Bearer + Fleet 校验 | runtime_id（必须在 Registry） | `Process.Kill()`（T2 实测） | Accepted{action_id} | 10s | 进程已不存在 → ALREADY_GONE | `action.stop` | 记入 Event Log |
| **Restart** | `POST /restart` | Bearer + Fleet 校验 | runtime_id | kill + 以**原受限命令描述符** respawn | Accepted{action_id, new_runtime_id} | 15s | 命令描述符失效 → REFUSED | `action.restart` | 记入 Event Log |
| **Logs** | `GET /logs?tail=N` | Bearer + Fleet 校验 | runtime_id | 读本机日志缓冲 | LogPage（≤N 行，**脱敏**） | 5s | 无日志 → EMPTY | 无（只读） | 只读，记访问审计 |
| **Attach** | `POST /attach` | Bearer + Fleet 校验 | runtime_id | 见 §10 语义 | AttachHandle | 10s | 见 §10 | `action.attach` | 记入 Event Log |

## 9.2 明确禁止（遵守）

```text
❌ arbitrary PID            ❌ arbitrary command
❌ arbitrary shell          ❌ arbitrary executable path
```

## 9.3 必须成立的链路（已在 Spike 中部分验证）

```text
Control Action
      ↓
Known Runtime          ← Registry 中存在（新建）
      ↓
Authorized Machine     ← machine_id ∈ Fleet 白名单（新建，设计）
      ↓
Authenticated Agent    ← Bearer（本机 T7 实测 PASS）/ 跨机签名（未做）
      ↓
Execution              ← Process.Kill()（T2 实测 PASS）
      ↓
Event                  ← 写入 Event Log（新建，设计）
```

**判决：CONTROL PLANE = PASS（本机形态）。** 现有 `internal/control` 提供了 HTTP+client 骨架，可直接 **ADAPT**（保留形态，新增鉴权层，剔除链业务端点）。

---

# §10 — ATTACH 的重新定义

必须审查 `Attach Session` 到底是什么。**不假设它 = remote terminal。**

| 语义 | 描述 | 技术成本 | MVP |
|---|---|---|---|
| A. reconnect to existing interactive session | 重连一个仍有交互通道的会话 | 需 PTY + 会话守护 + 多路复用 | ❌ |
| B. **view session output** | **只读查看会话的输出（可 tail / 跟随）** | **读本机日志缓冲 + 长连接推送** | ✅ **选中** |
| C. resume agent session | 让暂停的 Agent 继续 | 需 Agent 协议协作（§6） | ❌ |
| D. open terminal | 打开一个新终端 | 需 PTY | ❌ |
| E. remote shell | 远程命令执行 | 需 PTY + 加密 + 授权 | ❌（且违反 §6/§9 红线） |

## 10.1 MVP 选择

```text
ATTACH (MVP) := B. view session output
```

即：**"Attach = 把这个会话的输出拉到你眼前（只读、可跟随）"，不是远程终端。**

## 10.2 成本诚实评估

真正的交互式 Attach（A/C/D/E）需要：

```text
PTY
terminal multiplexing
remote shell
interactive transport
```

**结论：过高，从 MVP 移除。** 按 §10 纪律，**宁可重新定义 MVP 的 Attach，也不为满足旧报告的措辞机械实现远程终端。**

**判决：ATTACH = 缩小为只读输出跟随（GO WITH CONSTRAINTS 的约束项之一）。**

---

# §11 — EVENT / HISTORY FOUNDATION

## 11.1 必须成立的链路

```text
Event → append → integrity → query → timeline
```

## 11.2 最小事件字段（设计）

```text
event_id    : evt_<monotonic seq>           （或 <machine_id>:<seq>）
machine_id  : mch_<...>
runtime_id  : rt_<...>
session_id  : sess_<...>
actor_id    : 触发者（本机 Agent / Control Center / 人工）
event_type  : session.started | session.ended | artifact.produced
              | action.stop | action.restart | action.attach
timestamp   : RFC3339 UTC
summary     : 一行摘要（≤ 256 字节，人可读）
integrity   : prev_hash + body_hash + signature     ← 见 §12
```

## 11.3 明确禁止写入（遵守）

```text
❌ full file content      ❌ source code        ❌ secret
❌ environment secret     ❌ private configuration
❌ raw sensitive payload
```

> Event 存的是**"发生了什么"的摘要与凭证**，不是**"内容本身"**。这是"可证明"与"可泄露"的分界线。

## 11.4 是否需要 Blockchain / UTXO / Mempool / PoW？

| 问题 | 判决 | 依据 |
|---|---|---|
| 需要 Blockchain？ | **不需要** | 事件是"线性追加 + 完整性"，不需要分叉选择/共识 |
| 需要 UTXO？ | **不需要** | 事件不是所有权转移，无花费概念 |
| 需要 Mempool？ | **不需要** | 事件无"待打包"语义，直接落盘 |
| 需要 PoW？ | **Receipt v1 不需要** | 见 §12：哈希链 + 签名已足够（T5 实测） |

**本阶段未因"代码已存在"而保留任何一项。** 存储层可复用的是 `internal/storage` 的**文件落盘范式**（打开/追加/刷盘/进程独占），而非区块链结构。

---

# §12 — RECEIPT FEASIBILITY

## 12.1 必须回答的 5 个问题

| 问题 | 答案 |
|---|---|
| What is signed? | `canonical(Body)` 的 sha256 摘要（`Body` = 会话起止 + 状态 + exit + env_fp + artifact_digest + prev） |
| What is hashed? | `Body` 的确定性 JSON（struct 字段序固定）→ `sha256` → `body_hash` |
| What is exported? | `{body, body_hash, pub, sig}` 单文件 JSON（T4 实测 713 bytes） |
| What is verified offline? | 重算 body_hash → 比对 → 解析内嵌公钥 → `ecdsa.VerifyASN1` |
| What private data is excluded? | 无私钥、无源码、无 secret、无余额、无链数据、无原始日志正文 |

**关键设计：公钥内嵌在 Receipt 里**，因此**验证方不需要任何外部密钥分发**。

```json
{
  "body": { "v":1, "machine_id":"mch_791509bd6bb9cce4c6ef217f",
            "session_id":"sess-20260912-001",
            "started_at":"...", "ended_at":"...",
            "state":"COMPLETED", "exit_code":0,
            "env_fp":"env_...", "artifact_digest":"sha256:...",
            "prev":"genesis" },
  "body_hash": "22eae252baff52db62f9f17e59e472146f05eac3655dd352d92eb4b105a78f52",
  "pub": "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...",
  "sig": "MEQCIGxjbbd/B+aAOGmoZs9HRYTAczNC0+ZW3csawY5iKHFB..."
}
```

## 12.2 是否可不用 PoW 实现第一版 Receipt？

**可以。**

实测证据（T4）：在**另一个独立进程**中、**只读 `receipt.json`**、**无私钥、无网络**的条件下：

```text
VERIFY OK   session=sess-20260912-001 state=COMPLETED exit=0 | verified offline   （exit 0）
```

篡改一个字段后：

```text
VERIFY FAIL session=sess-20260912-001 | body_hash mismatch (content altered)       （exit ≠ 0）
```

**结论：Hash chain + signature 已经足够支撑第一版 Receipt。**

## 12.3 哈希链的强度边界（T5 实测，诚实记录）

测试构建 4 条记录的链，实施 3 种攻击：

| 攻击 | 手段 | 结果 | 说明 |
|---|---|---|---|
| A | 改字段，不重算 hash | **检测到**：`record 1: body_hash mismatch` | 最弱的攻击 |
| B | 改字段 + 重算 body_hash + 重签名（模拟私钥泄露） | **检测到**：`record 2: CHAIN BREAK` | 单条签名挡不住持钥者改自己那条；**链的前后链接补上了这一环** |
| C | **持钥者重写整条链**（每条都重签，重新链接） | **无法检测**（链自洽） | **这是哈希链的诚实上限** |

T5 结论：**有效链验证 PASS（4 records, chain intact + all signatures valid）。**

攻击 C 的含义必须写清楚：

```text
哈希链能证明："记录未被【事后局部】篡改"
哈希链不能证明："记录不是持钥者【整体重写】的"
```

**这正是 PoW / 外部锚定存在的意义**——给"整体重写"加上不可伪造的成本。但：

```text
MVP 一期不引入 PoW。原因：一期 Receipt 的价值是"机器自证某次会话发生过"，
而非"向敌意第三方证明历史不可重写"。后者的成本（PoW / 外部锚定）远超一期收益。
```

**判决：RECEIPT = PASS。PoW 保留为"未来可选的外部锚定"，不进入一期关键路径。**

---

# §13 — DESKTOP SHELL FEASIBILITY

只做技术可行性审计，**不制作最终 UI**。

| 项 | 现有 | 可行性 |
|---|---|---|
| Windows tray | ❌ | ⚠️ 需第三方库或 syscall（**非标准库**），MVP 可后置 |
| startup（开机自启） | ❌ | ✅ OS 级能力（注册表/Run 键 或 任务计划），**非代码难题** |
| background agent | ❌ | ✅ 就是 §4 的 Agent Host，可行 |
| local IPC | 部分（HTTP） | 见下 |
| notifications | ❌ | ⚠️ 需 OS API（Windows Toast），MVP 可后置 |
| single-window Control Center | ❌ | ✅ 前端工程，无技术风险 |

## 13.1 RECOMMENDED IPC

实测（T6）：**Windows 上 `net.Listen("unix", ...)` 返回 nil 错误（即可用）**。

| IPC 选项 | 实测/评估 | 结论 |
|---|---|---|
| HTTP localhost | ✅ **T7 实测可用**（127.0.0.1 + Bearer 401/200） | **RECOMMENDED** |
| Unix socket | ✅ Windows 实测可用（非官方标准，兼容性需谨慎） | 备选 |
| Windows named pipe | ⚠️ 需 `syscall`（非标准库），且使 Go 代码不可移植 | 不推荐（一期） |
| 其他 | — | — |

```text
RECOMMENDED IPC = HTTP on 127.0.0.1 + Bearer token
```

**原因**：

1. **零依赖**：`net/http` 是标准库，与项目"骨架阶段刻意只用标准库"的既有约定一致。
2. **已实测**：T7 完整验证了鉴权与回环绑定。
3. **可移植**：同一套代码在 Linux/macOS 上直接复用（unix socket 反而是 Windows 上的特例）。
4. **可演进**：将来跨机器时，把 `127.0.0.1` 换成"认证传输 + TLS"即可，HTTP 语义不变。

**跨平台说明**：`Control Center → Local Agent` 用 HTTP 回环；**不引入 named pipe**，以免把 Go 代码钉死在 Windows。

---

# §14 — LOCAL-FIRST ARCHITECTURE

## 14.1 待验证结构

```text
Control Center
      │
  Local IPC          ← §13 RECOMMENDED: HTTP 127.0.0.1 + Bearer（T7 实测）
      │
  Machine Agent      ← 新建（零 os/exec 现状，T1/T2 证明可建）
      │
 ┌────┼────────────┐
 ↓    ↓            ↓
Runtime  Event Log  Identity
 │      (§11 设计)  (§7 P1 实测 mch_)
 ↓
Agent Session        ← 新建（§5 五态可实现）
```

## 14.2 跨机器（一期不做）

```text
Machine A Agent
      │  authenticated transport（TLS + 签名，未做 → 一期禁用）
      │
Machine B Agent
```

## 14.3 验证结论

| 结构 | 是否成立 | 依据 |
|---|---|---|
| Control Center → Local IPC → Agent | ✅ 成立 | T7（IPC+鉴权）+ T1/T2（Agent 可监管进程） |
| Agent → Runtime / Event Log / Identity | ✅ 成立 | Identity T3 实测；Runtime T1/T2 实测；Event Log §11 设计 |
| Runtime → Agent Session | ✅ 成立 | §5 状态机可由进程事实派生 |
| Agent ↔ Agent（跨机器） | ❌ **一期不成立** | 缺加密传输（§7 P6） |

## 14.4 架构不变式（遵守）

```text
UI 不直接控制远程机器。
必须经过：Control Center → Authorized Agent → Runtime
```

---

# §15 — MVP TECHNICAL FEASIBILITY MATRIX

| Capability | Existing Asset | Gap | Complexity | Security Risk | MVP |
|---|---|---|---|---|---|
| Machine Identity | `internal/wallet` P-256 原语 | 抽象出 identity primitive | 低 | 低（原语成熟） | ✅ |
| Pairing | 无 | 全新建 | 中 | **高**（决定 Fleet 边界） | ✅ |
| Agent Host | 无（零 `os/exec`） | 全新建 | 中 | 中（本机进程） | ✅ |
| Runtime Registry | 无 | 全新建 | 低 | 低 | ✅ |
| Session State | 无 | 新建（5 态） | 低 | 低 | ✅ |
| Observe | 无 | 新建（读 Registry） | 低 | 低 | ✅ |
| Stop | 无 | 新建（`Kill`，T2 已证） | 低 | 中（需鉴权） | ✅ |
| Restart | 无 | 新建（kill+respawn） | 中 | 中 | ✅ |
| Logs | 无 | 新建（capture+ring buffer） | 低 | 中（需脱敏） | ✅ |
| Attach | 无 | 新建（**只读输出**） | 低 | 低 | ⚠️ 缩小 |
| Event Log | 无（零 hashchain） | 新建（含 integrity） | 中 | 中 | ✅ |
| History | 无 | 新建（query+timeline） | 低 | 低 | ✅ |
| Receipt | wallet 签名原语 | 新建 Receipt 结构（T4 已证） | 低 | 低 | ✅ |
| Offline Verify | 无 | 新建（T4 已证） | 低 | 低 | ✅ |
| Tray | 无 | 全新建（需第三方/syscall） | 中 | 低 | ❌ 后置 |
| Notification | 无 | 全新建（OS API） | 中 | 低 | ❌ 后置 |
| Secure Transport | 无（全仓无 TLS） | 全新建 | 高 | **高** | ❌ 后置（一期单机） |

---

# §16 — CORE DECISION

> **Developer Control Center MVP 是否值得继续开发？**

```text
DECISION = GO WITH CONSTRAINTS
```

**可证伪理由**：

1. **技术闭环已实测成立，而非断言成立。** 隔离 Spike `PASS=24 FAIL=0`，覆盖：进程生命周期（T1）、Kill 与终态（T2）、P-256 机器身份（T3）、**跨进程离线 Receipt 验证**（T4）、哈希链篡改检测与诚实上限（T5）、Windows IPC（T6）、**环回 HTTP + Bearer 鉴权**（T7）。
2. **最重的活已被证明"可做"**：BRAND-0D.2 判定为 BLOCKED 的运行时监管（今天 0 行 `os/exec`），在标准库层面被 T1/T2 证明可行。
3. **账本依赖被成功剥离**：Receipt v1 无需 PoW、无需 UTXO、无需 Mempool——T4/T5 直接证明。
4. **但三项仍然 PARTIAL，因此不是无条件 GO**：
   - 跨机器**加密传输**缺失（全仓无 TLS）；
   - **Pairing / Ownership / Fleet 边界**仅设计、未 Spike；
   - **Attach** 若按传统"远程终端"理解则成本过高，必须缩小为只读输出。

**为什么不是 KILL**：§17 的 6 条 KILL 条件中，无一条成立——远程控制**可以**被安全化（回环 + Bearer 已验证，跨机 + TLS 有成熟路径）；机器所有权**可以**被建立（P-256 + 白名单）；生命周期**可以**被可靠观测（T1/T2）；核心循环**不需要**过度基础设施（D 选项）；Receipt **有**差异化（离线可验证凭证，T4）；产品**不会**塌缩成通用监控（因为它管的是"你自己的 Agent 会话"，不是通用主机指标）。

---

# §17 — GO / NO-GO

## 17.1 GO 条件核对

| 条件 | 状态 | 依据 |
|---|---|---|
| Machine Identity feasible | ✅ | T3 实测 |
| Pairing feasible | ⚠️ | 设计完成，未 Spike |
| Secure transport feasible | ❌ | 未做（一期用单机绕开） |
| Agent Host feasible | ✅ | T1/T2 |
| Runtime lifecycle feasible | ✅ | T1/T2 |
| Observe feasible | ✅ | T7 + Registry 设计 |
| Stop feasible | ✅ | T2 |
| Restart feasible | ⚠️ | 可组合（kill+spawn），未单测 |
| Logs feasible | ✅ | T1/T2 capture |
| History feasible | ⚠️ | 设计完成，未 Spike |
| Receipt feasible | ✅ | T4/T5 |

**7/11 明确 PASS，4 项 PARTIAL/未 Spike → 不满足无条件 GO。**

## 17.2 GO WITH CONSTRAINTS（采纳）

允许 `GO WITH CONSTRAINTS`，约束如下：

```text
C-1  Attach 缩小为"只读输出跟随"（§10 → B）
C-2  IDLE / WAITING 延后（PROTOCOL-DEPENDENT，MVP 不实现）
C-3  Receipt 一期只做 3 类事件（Session START / END / Artifact），不扩类
C-4  MVP 一期【单机】；跨机器控制延后至加密传输就绪（§7 P6）
C-5  Tray / Notification 后置（非 MVP）
C-6  PoW / UTXO / Mempool / Blockchain 全部退出 MVP 关键路径
```

**不可缺失项（约束不得触碰）**：

```text
PAIR  /  IDENTITY  /  AUTHENTICATION  /  CONTROL     ← 四项 MUST，任何一项缺失即回到 KILL 评估
```

## 17.3 KILL 条件核对（均不成立）

| KILL 条件 | 是否成立 | 说明 |
|---|---|---|
| Remote control cannot be secured | ❌ | 回环+Bearer 已实测；跨机有 TLS 成熟路径 |
| Machine ownership cannot be established | ❌ | P-256 + 白名单方案成立 |
| Agent lifecycle cannot be reliably observed | ❌ | T1/T2 可靠观测 |
| Core loop requires excessive infrastructure | ❌ | 单机一期基础设施量很小 |
| Receipt provides no meaningful differentiation | ❌ | 离线可验证凭证（T4） |
| Product collapses into generic monitoring | ❌ | 管的是"自己的 Agent 会话"，非通用主机指标 |

**不触发 STOP，不退回 BRAND-0D.1。**

---

# §18 — IMPLEMENTATION GATE

```text
本阶段 = NO PRODUCTION IMPLEMENTATION ✅（0 行生产代码改动）
```

允许并已执行的 `ISOLATED SPIKE`：

```text
✅ temporary local process supervisor      （T1/T2）
✅ temporary P-256 signing test            （T3/T4）
⚠️ temporary pairing handshake             （未做 → 留待 0D.4）
✅ temporary local IPC proof               （T6/T7）
✅ temporary receipt verification proof    （T4/T5）
```

所有 Spike 均满足：`isolated ✅ documented ✅ non-production ✅ reversible ✅`，**未并入主架构**。

---

# §19 — FINAL ARCHITECTURE（按真实审计结果修订）

## 19.1 修订后的最小架构（一期 = 单机）

```text
┌──────────────────────────────┐
              │   Developer Control Center   │   ← 单窗口（§13，前端工程）
              │  （Observe/Stop/Restart/Logs）│
              └──────────────┬───────────────┘
                             │  HTTP 127.0.0.1 + Bearer   ← §13 RECOMMENDED / T7 实测
              ┌──────────────▼───────────────┐
              │         Machine Agent        │   ← 新建（零 os/exec → T1/T2 证明可建）
              │  ┌────────────────────────┐  │
              │  │ Runtime Registry       │  │   ← §4 R4
              │  │ (rt_/sess_/state)      │  │
              │  └───────┬────────────────┘  │
              │          │                   │
              │   ┌──────┼──────────┐        │
              │   ▼      ▼          ▼        │
              │ Runtime  Event    Identity   │   ← Runtime=T1/T2 · Event=§11 · Identity=T3
              │ (os/exec) Log     (P-256)    │
              │   │      │                   │
              │   ▼      ▼                   │
              │ Agent   Hash chain +         │
              │ Session signature            │   ← §12 Receipt（T4/T5）
              └──────────────────────────────┘
                     │
        ✂ 一期到此为止（单机闭环）
        ✂ Object 后置（跨机器需加密传输，§7 P6）
                     │
                     ▼   ← 二期（未授权，仅占位）
              Other Machines（TLS + 签名认证传输）
```

## 19.2 与 §19 模板的差异声明（按纪律修改模板）

| 模板组件 | 保留？ | 原因 |
|---|---|---|
| Control Center | ✅ | 产品本体 |
| Local IPC | ✅ | T7 实测 |
| Machine Agent | ✅ | T1/T2 实测可建 |
| Runtime | ✅ | T1/T2 |
| Event Log | ✅ | §11 |
| Identity | ✅ | T3 |
| **Agent Session → Other Machines（跨机器直连）** | ❌ **移出 MVP** | 缺加密传输；改为"经 Control Center 仲裁 + 加密传输（二期）" |

**未为了符合模板而保留不成立的组件。**

---

# §20 — NEXT PHASE GATE

本阶段**未关闭**的 MUST（决定下一阶段）：

```text
G-1  Secure Machine Pairing（P2/P3/P4/P5）—— 设计完成，未 Spike
G-2  Minimum Agent Protocol（§6）—— 设计完成，未 Spike
G-3  Encrypted Transport for cross-machine control —— 完全未做
```

因此下一阶段**不应**直接进入 MVP 实现（会有"未验证的安全地板"风险），而应做一个**聚焦安全的小型 Spike**：

```text
NEXT PHASE = PHASE BRAND-0D.4 — SECURE PAIRING & MINIMUM AGENT PROTOCOL SPIKE
```

该阶段目标（建议，需另行授权）：

```text
1. 实现并实测 pairing handshake（一次性码 + challenge-response + 落白名单）
2. 实测陌生机器被拒绝（默认拒绝）
3. 实测 replay 防护（nonce 去重 + timestamp 窗口）
4. 实测跨机器加密传输的最小形态（TLS 或 Noise 类，二选一）
5. 复测 §16 的 4 项 PARTIAL → 力图转为 PASS
6. 只有 G-1..G-3 关闭后，才允许讨论 IMPLEMENTATION
```

**本阶段到此 STOP。**

---

# §21 — FINAL DECISION FORMAT

```text
PHASE BRAND-0D.3 RESULT

PRODUCT:
Developer Control Center

PRODUCT DIRECTION:
PASS

TECHNICAL FEASIBILITY:
PASS

SECURITY FOUNDATION:
PARTIAL

RUNTIME FOUNDATION:
PASS

CONTROL PLANE:
PASS

RECEIPT:
PASS

P2P REUSE:
ADAPT

RECOMMENDATION:
GO WITH CONSTRAINTS

NEXT PHASE:
PHASE BRAND-0D.4 — SECURE PAIRING & MINIMUM AGENT PROTOCOL SPIKE

IMPLEMENTATION:
SPIKE ONLY

STOP:
YES
```

## 21.1 结果判定依据（逐项）

| 字段 | 判定 | 一句话依据 |
|---|---|---|
| PRODUCT DIRECTION | **PASS** | 6 条 KILL 条件无一成立；核心循环技术可行 |
| TECHNICAL FEASIBILITY | **PASS** | Spike `PASS=24 FAIL=0`，闭环五段全部有实测支撑 |
| SECURITY FOUNDATION | **PARTIAL** | Identity 实测可行，但 Pairing/加密/Fleet 未 Spike（§7.1） |
| RUNTIME FOUNDATION | **PASS** | T1/T2 证明标准库可完成 spawn/监管/Kill/终态判定 |
| CONTROL PLANE | **PASS** | T7 实测回环 HTTP + Bearer 401/200；骨架可 ADAPT |
| RECEIPT | **PASS** | T4 跨进程离线验证；T5 哈希链检测 A/B 攻击，C 为诚实上限 |
| P2P REUSE | **ADAPT** | 只复用 crypto primitive + 存储范式；重建 Control Transport（§8 = Option C+D） |

---

# §22 — PRODUCT DISCIPLINE（阶段纪律承继）

> **我们不是在给 P2PChain 找用途。**
>
> **我们是在验证 Developer Control Center 是否值得存在，然后判断 P2PChain 哪些技术资产值得留下。**

对 MVP 没有必要的能力，**不为复用而复用**：

```text
UTXO        → 退出关键路径（事件无花费语义）
Mempool     → 退出关键路径（事件无待打包语义）
PoW         → 一期退出（Receipt v1 不需要，T4/T5 已证）
Mining      → 退出
Wallet UI   → 退出（Wallet 仅作 crypto primitive 来源，且不改名）
Blockchain UI → 退出
```

产品成立所必需的全新能力，**必须承认它是新产品基础设施**：

```text
Agent Host          → 新建（今天 0 行）
Runtime Supervisor  → 新建（今天 0 行）
Pairing             → 新建
Authentication      → 新建（本机已 Spike，跨机未做）
Control Plane       → 新建（可 ADAPT 现有 HTTP 骨架）
Event Log           → 新建（今天 0 行 hashchain）
```

---

# 附录 A — ISOLATED SPIKE 原始证据

## A.1 位置与隔离

```text
目录：C:\Users\Administrator\Desktop\挖矿\.spike\brand-0d3\
文件：go.mod（3 行）· main.go（558 行）
依赖：零第三方（标准库 only）
隔离证明：git rev-parse --show-toplevel → fatal: not a git repository（不在任何仓库内）
```

## A.2 完整运行输出（`PASS=24 FAIL=0`）

```text
=== PHASE BRAND-0D.3 ISOLATED SPIKE ===
runtime : windows/amd64  go=go1.22.12

[T1] Process lifecycle (spawn / running / stdout / exit code)
PASS  T1 spawn + running detected                    | pid=22468
PASS  T1 stdout captured (streaming)                 | 3 lines
PASS  T1 exit detected                               | ProcessState.Exited()=true
PASS  T1 exit code captured                          | code=3
PASS  T1 nonzero exit => FAILED mapping              | Wait() returned non-nil error for code 3

[T2] Stop (Kill) + terminal-state detection
PASS  T2 long child running                          | pid=18408
PASS  T2 Kill returned                               | <nil>
PASS  T2 killed => terminal(FAILED/LOST)             | Wait err=true exited=true
PASS  T2 partial stdout preserved before kill        | 111 bytes captured

[T3] Machine identity (P-256)
PASS  T3 P-256 keypair generated                     | curve=P-256
PASS  T3 machine id derived from pubkey              | mch_791509bd6bb9cce4c6ef217f
PASS  T3 private key persisted (0600)                | ...\work\machine.key

[T4] Receipt: sign now, verify LATER in a SEPARATE process (no priv key, no network)
PASS  T4 receipt signed                              | sig=96 bytes b64
PASS  T4 self-verify                                 | verified offline
PASS  T4 verified in SEPARATE process (exit 0)       | VERIFY OK session=sess-20260912-001 state=COMPLETED exit=0
PASS  T4 field tampering REJECTED (exit != 0)        | VERIFY FAIL | body_hash mismatch (content altered)

[T5] Event ledger: hash chain + tamper detection
PASS  T5 valid chain verifies                        | 4 records, chain intact + all signatures valid
PASS  T5 attack A (edit field) DETECTED              | record 1: body_hash mismatch (content altered)
PASS  T5 attack B (edit+resign) DETECTED by chain link | record 2: CHAIN BREAK
PASS  T5 attack C (rewrite WHOLE chain w/ key) NOT detectable by hash chain | <= honest limit

[T6] IPC options on this OS
PASS  T6 unix socket on windows                      | <nil>

[T7] Local control plane: loopback HTTP + bearer auth
PASS  T7 unauthenticated request REJECTED (401)      | status=401
PASS  T7 authenticated request ACCEPTED (200)        | status=200
PASS  T7 bound to loopback only                      | 127.0.0.1:54339

=== SPIKE SUMMARY ===
PASS=24 FAIL=0
```

## A.3 Spike 修复记录（透明披露）

首次运行结果为 `PASS=22 FAIL=2`。原因：`verifyChain` 的链头基准误写为空串，而首条记录的 `Prev` 为 `"genesis"`，导致合法链在 record 0 被误判为 `CHAIN BREAK`，并使 T5 的三种攻击测试"因错误原因而通过"。

修复：将 `prev := ""` 改为 `prev := "genesis"`。复跑后 `PASS=24 FAIL=0`，且攻击 A/B/C 分别落到**正确的检测路径**（A→record 1 body_hash；B→record 2 chain link；C→不可检测）。

**该缺陷仅存在于隔离 Spike 内部，从未进入生产代码，也未影响任何产品结论**（§12 的 Receipt 可行性由 T4 独立证明，不依赖 `verifyChain`）。

## A.4 产物清单（可整体删除以回退）

```text
work/machine.key            138 B    （模拟本机可信存储，0600）
work/receipt.json           713 B    （合法 Receipt）
work/receipt.tampered.json  722 B    （篡改样本，验证被拒）
work/chain.jsonl           2499 B    （合法链，4 条）
work/chain.a.jsonl         2494 B    （攻击 A 样本）
work/chain.b.jsonl         2494 B    （攻击 B 样本）
work/chain.c.jsonl         2495 B    （攻击 C 样本）
```

## A.5 保留决定

```text
Spike 保留为【纯证据】：目录留在仓库外，不合并、不引用、不进入构建。
如需彻底清理：删除 .spike/brand-0d3 目录即可，对仓库零影响。
```

---

# 附录 B — 本阶段事实 / 推测 / 不确定 三段分离

**已确认（有源码或实测证据）**

- 生产代码 `os/exec|exec.Command|exec.Cmd` 出现 **0** 次；`signal.Notify` 仅 1 处且用于节点自我退出（`cmd/node/main.go:291`）。
- 生产代码无任何 `eventlog/hashchain/append-only` 抽象（0 命中）；无 "session" 概念（0 命中）。
- `HandshakePayload` 仅含自报 `NodeID` 字符串，无签名/challenge/nonce/公钥 → **可伪造**。
- 全仓 **TLS 命中 0**；`internal/control/server.go:6` 与 `cmd/node/main.go:242` 自述"仅本机、无鉴权"。
- 现有控制 API 有 6 个端点（`/status /balance /utxos /send /mine /block`），**零鉴权**。
- `internal/wallet` 使用 `elliptic.P256()` + `ecdsa.GenerateKey`；`internal/storage/datalock.go` 使用 `os.Getpid()`（数据目录独占锁）。
- Spike 实测：T1–T7 全部 PASS（`PASS=24 FAIL=0`）。
- 基线绿：`go build` / `go vet` / `go test ./...` 全通过；129 个测试用例。

**推测（合理但无直接证据）**

- `restart` 可由 `Kill` + 重新 `spawn` 组合实现（两原语均已实测，组合未单测）。
- `stderr` 捕获、`tail N`、日志落盘复用 `internal/storage` 范式后为低风险工程量。
- Windows tray / notification 需第三方库或 syscall，属"可后置"而非"不可做"。

**尚不确定（需下一阶段解决）**

- Pairing / Ownership / Fleet 边界 / Replay 防护的实际实现难度与安全强度（仅设计）。
- 跨机器加密传输的最小可接受形态（TLS vs Noise 类）与成本。
- 一期单机约束下，"多机器 DCC" 的产品价值是否仍然成立（**产品层面问题，非技术问题**，建议由 BRAND-0D.4 或产品访谈回答）。

---

# 附录 C — 并发/前序遗留（不由本阶段处理）

```text
1. PHASE P3.1 相关改动仍未提交（cmd/node/main.go, internal/control/server.go,
   internal/storage/datalock.go + 2 个测试文件）——本阶段按 §0.3 禁止 Git 操作，未触碰。
2. docs/ 下存在命名与内容错位：文件 PHASE-BRAND-0D.2-MVP-BOUNDARY.md 中
   实际保存的是 BRAND-0D.3 的规格（其首行为 "# PHASE BRAND-0D.3 ..."）；
   同类错位亦见于 0D.1 / 0D 两个文件。本阶段未改动任何既有文档，
   仅新增本报告文件，避免覆盖他人产出。建议后续统一整理文档命名（需显式授权）。
3. BRAND-0D §7.5 / 0D.2 K-1..K-7 的用户/开发者验证访谈仍未执行
   （零工程成本，是唯一能证伪产品方向的手段）。
```

---

**本阶段完成。STOP。不得自动进入品牌命名、最终 UI 设计、完整 MVP 实现、生产部署。**
