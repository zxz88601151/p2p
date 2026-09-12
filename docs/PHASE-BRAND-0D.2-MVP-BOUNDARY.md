# PHASE BRAND-0D.3 — DEVELOPER CONTROL CENTER TECHNICAL FOUNDATION & FEASIBILITY SPIKE

> 项目：P2PChain
>
> 前置阶段：
>
> - PHASE BRAND-0D
> - PHASE BRAND-0D.1
> - PHASE BRAND-0D.2
>
> 前置结论：  
> **BRAND-0D.2 = PRODUCT DIRECTION PASS / MVP BOUNDARY PASS / TECHNICAL FOUNDATION BLOCKED**
>
> 本阶段性质：  
> **TECHNICAL FOUNDATION / ARCHITECTURAL FEASIBILITY / CONTROL-PLANE SPIKE**
>
> 本阶段唯一目标：
>
> **证明 Developer Control Center 的最小技术闭环是否真实可行：**
>
> ```text
> PAIR → GLANCE → ACT → HISTORY → RECEIPT
> ```
>
> 并明确：
>
> ```text
> 哪些现有 P2PChain 资产可以直接复用
> 哪些必须抽象
> 哪些必须新建
> 哪些资产必须退出 MVP 关键路径
> ```
>
> 本阶段结束后必须 STOP。
>
> **不得自动进入品牌命名、最终 UI 设计、完整 MVP 实现、生产部署。**

---

# §0 — ABSOLUTE RULES

## 0.1 READ FIRST

必须完整阅读并继承：

```text
PHASE BRAND-0D
PHASE BRAND-0D.1
PHASE BRAND-0D.2
```

不得重新讨论已经确定的产品方向。

BRAND-0D.2 的核心结论必须作为本阶段硬前提：

```text
Product Category:
Developer Control Center

Primary Job:
在一个地方看到自己的多台机器正在运行什么，并在需要时直接控制这些运行中的任务。

Secondary Job:
回看发生了什么。

Differentiating Job:
对极少数关键事件生成可独立验证的 Receipt。

Core Loop:
PAIR → GLANCE → ACT

On-demand:
RECEIPT

Passive:
HISTORY
```

---

# §1 — READ-ONLY DEFAULT

本阶段默认：

**READ-ONLY。**

禁止修改：

```text
.go
.js
.ts
.py
go.mod
go.sum
配置
数据库
blocks.dat
wallet
P2P
PoW
UTXO
Node
API
CLI
UI
installer
```

除非在本阶段明确出现单独的：

```text
IMPLEMENTATION GATE
```

否则不得修改任何生产代码。

允许：

```text
源码阅读
测试运行
静态分析
架构映射
技术 Spike
临时 throwaway prototype
文档
```

如需要创建临时 prototype：

- 必须放入明确的 spike / temporary 范围
- 不得污染 production architecture
- 必须在报告中明确是否保留
- 不得自动合并进生产代码

---

# §2 — HARD BASELINE

重新建立独立 HARD BASELINE。

记录：

```text
OS
CPU
architecture
Go version
Node version（如存在）
repository root
current branch
HEAD
parent
git status
tracked diff
staged diff
untracked files
project size
package count
test count
build status
vet status
test status
```

必须确认：

```text
BASELINE = main@<actual commit>
```

如果当前存在并发工作：

```text
STOP AND ISOLATE
```

不得：

```text
restore
clean
reset
checkout
delete
overwrite
format
```

并发工作不得作为能力证据。

---

# §3 — EXISTING ARCHITECTURE RE-AUDIT

只针对 Developer Control Center 所需能力重新审计：

```text
Node
P2P
Wallet
Storage
Blockchain
PoW
UTXO
Mempool
CLI
Control API
```

不要重新做全项目审计。

目标是建立：

```text
CURRENT ASSET
      ↓
MVP REQUIRED CAPABILITY
      ↓
REUSE / ADAPT / ABSTRACT / DROP / NEW
```

---

# §4 — RUNTIME FOUNDATION AUDIT

这是本阶段最高优先级。

必须真实回答：

> **一台机器上的 Agent Session 到底如何成为一个可观察、可控制的 Runtime？**

必须搜索并确认：

```text
os/exec
exec.Command
exec.CommandContext
Start
Wait
Process
ProcessState
Kill
Signal
PID
stdout
stderr
pipes
logs
child process
parent process
session
daemon
service
watcher
heartbeat
```

必须输出：

### R1 — Process lifecycle

是否存在：

```text
spawn
start
running detection
exit detection
exit code
stop
kill
restart
```

### R2 — Log lifecycle

是否存在：

```text
stdout capture
stderr capture
tail N lines
log persistence
log rotation
log truncation
```

### R3 — Agent Session lifecycle

是否存在：

```text
session creation
session identity
session state
last activity
completion
failure
lost
unknown
```

### R4 — Runtime registry

是否存在：

```text
runtime_id
machine_id
process identity
command identity
session identity
timestamps
state
```

如果不存在：

> 明确设计最小 Registry，而不是立即实现。

---

# §5 — SESSION STATE MACHINE SPIKE

必须重新验证 BRAND-0D.2 定义的：

```text
RUNNING
IDLE
WAITING
COMPLETED
FAILED
LOST
UNKNOWN
```

对每一个状态回答：

```text
Can it be observed locally?
Can it be observed remotely?
What evidence proves it?
What transition creates it?
What transition clears it?
Can it be forged?
Can it become stale?
```

特别注意：

**不要为了满足枚举而制造假的状态。**

如果当前只能可靠实现：

```text
RUNNING
COMPLETED
FAILED
LOST
UNKNOWN
```

必须诚实记录。

`IDLE` / `WAITING` 如果需要 Agent Protocol 主动上报，则明确标记为：

```text
PROTOCOL-DEPENDENT
```

---

# §6 — MINIMUM AGENT PROTOCOL

设计但默认不实现一个最小 Agent Protocol。

必须定义最少：

```text
Machine Identity
Agent Identity
Runtime Identity
Session Identity
Protocol Version
Timestamp
Nonce
Signature
State
Action
Event
```

至少回答：

```text
Agent → Control Center
Control Center → Agent
Agent → Agent
```

各自传什么。

禁止设计：

```text
generic RPC framework
plugin system
marketplace
workflow engine
Kubernetes-like orchestration
full remote shell
generic command execution
```

---

# §7 — SECURE MACHINE PAIRING

重新验证 BRAND-0D.2 的安全地板。

至少设计：

```text
Machine Identity
Machine Ownership
Pairing
Peer Authentication
Authorization
Fleet Boundary
Unauthorized Peer Rejection
Replay Protection
```

必须回答：

### P1

如何生成 Machine Identity？

优先研究现有：

```text
P-256
```

但：

> **禁止简单把 Wallet 对象改名为 MachineIdentity。**

必须判断是否应该抽象：

```text
crypto identity primitive
        ↓
Machine Identity
        ↓
Wallet
```

而不是：

```text
Wallet
   ↓
改名
```

### P2

Pairing 如何发生？

例如：

```text
QR
one-time code
challenge-response
fingerprint
```

本阶段只设计，不做完整 UI。

### P3

如何证明：

> “这台机器属于我的 Fleet？”

### P4

陌生机器如何被拒绝？

### P5

如何防 replay？

### P6

Remote Control 是否必须加密？

必须重点验证：

```text
Stop
Restart
Logs
Attach
```

的安全需求。

**最终 MVP 不允许出现未经认证的远程控制。**

建议默认安全底线：

```text
Identity        MUST
Pairing         MUST
Authentication  MUST
Authorization   MUST
Encryption      MUST
Fleet Boundary  MUST
Replay Defense  MUST
```

本阶段如果发现某项可以合理降级，必须给出明确威胁模型，而不能直接标记 OPTIONAL。

---

# §8 — P2P REUSE DECISION

不要因为项目原本存在 P2P 就强行复用。

必须回答：

> **现有 P2P 是否值得成为 Developer Control Center 的网络层？**

至少比较：

```text
Option A:
直接改造现有 P2P

Option B:
保留现有 P2P transport，重建 identity/auth layer

Option C:
只复用 crypto primitives，重新建立 Control Transport

Option D:
MVP 暂时单机 + 后续再引入跨机器 transport
```

评价维度：

```text
Security
Complexity
Migration Cost
Testability
Maintainability
Product Fit
Privacy
Future Expansion
```

最终只能推荐一个：

```text
RECOMMENDED OPTION
```

并解释为什么。

---

# §9 — CONTROL PLANE SPIKE

建立最小控制模型：

```text
Observe
Stop
Restart
Logs
Attach
```

对每一个动作定义：

```text
Request
Authorization
Target
Execution
Result
Timeout
Failure
Event
Audit
```

特别禁止：

```text
arbitrary PID
arbitrary command
arbitrary shell
arbitrary executable path
```

必须证明：

```text
Control Action
      ↓
Known Runtime
      ↓
Authorized Machine
      ↓
Authenticated Agent
      ↓
Execution
      ↓
Event
```

---

# §10 — ATTACH 的重新定义

必须特别审查：

> `Attach Session` 到底是什么意思？

不要直接假设是：

```text
remote terminal
```

分别评估：

```text
A. reconnect to existing interactive session
B. view session output
C. resume agent session
D. open terminal
E. remote shell
```

必须选择最小 MVP 语义。

如果真正的 Attach 需要：

```text
PTY
terminal multiplexing
remote shell
interactive transport
```

则必须诚实评估其成本。

如果它过大：

> 可以重新定义 MVP 的 Attach，而不是为了满足原报告机械实现一个远程终端。

---

# §11 — HISTORY / EVENT FOUNDATION

重新验证：

```text
Event
↓
append
↓
integrity
↓
query
↓
timeline
```

必须明确：

```text
event_id
machine_id
runtime_id
session_id
actor_id
event_type
timestamp
summary
integrity
```

禁止：

```text
full file content
source code
secret
environment secret
private configuration
raw sensitive payload
```

必须验证：

> Storage + Blockchain 的哪些部分真正可以抽象成 Event Log。

重点判断：

```text
是否需要 Blockchain？
是否需要 UTXO？
是否需要 Mempool？
是否需要 PoW？
```

本阶段不得因为已有代码存在而保留。

---

# §12 — RECEIPT FEASIBILITY

仅设计最小 Receipt。

只验证 BRAND-0D.2 已确定的三类：

```text
Agent Session
Environment Fingerprint
Artifact
```

必须回答：

```text
What is signed?
What is hashed?
What is exported?
What is verified offline?
What private data is excluded?
```

Receipt 必须满足：

```text
Offline verification
No network
No private key required by verifier
No raw source content
No secret
No wallet balance
No blockchain UI
```

重点回答：

> **是否可以不用 PoW 就实现第一版 Receipt？**

如果可以：

```text
Hash chain + signature
```

是否已经足够？

如果不够：

明确指出为什么。

---

# §13 — DESKTOP SHELL FEASIBILITY

只做技术可行性审计。

评估：

```text
Windows tray
startup
background agent
local IPC
notifications
single-window Control Center
```

重点回答：

```text
Control Center
        ↓
Local Agent
```

应该使用：

```text
HTTP localhost
Named Pipe
Unix socket equivalent
Windows named pipe
other IPC
```

不得现在制作最终 UI。

只选择：

```text
RECOMMENDED IPC
```

并说明原因。

---

# §14 — LOCAL-FIRST ARCHITECTURE

验证以下结构是否成立：

```text
Control Center
                       │
                  Local IPC
                       │
                  Machine Agent
                       │
          ┌────────────┼────────────┐
          ↓            ↓            ↓
      Runtime      Event Log     Identity
          │
          ↓
      Agent Session
```

跨机器：

```text
Machine A Agent
       │
 authenticated transport
       │
Machine B Agent
```

必须明确：

> UI 不直接控制远程机器。

必须经过：

```text
Control Center
      ↓
Authorized Agent
      ↓
Runtime
```

---

# §15 — MVP TECHNICAL FEASIBILITY MATRIX

最终建立：

| Capability       | Existing Asset | Gap | Complexity | Security Risk | MVP |
| ---------------- | -------------- | --- | ---------- | ------------- | --- |
| Machine Identity |                |     |            |               |     |
| Pairing          |                |     |            |               |     |
| Agent Host       |                |     |            |               |     |
| Runtime Registry |                |     |            |               |     |
| Session State    |                |     |            |               |     |
| Observe          |                |     |            |               |     |
| Stop             |                |     |            |               |     |
| Restart          |                |     |            |               |     |
| Logs             |                |     |            |               |     |
| Attach           |                |     |            |               |     |
| Event Log        |                |     |            |               |     |
| History          |                |     |            |               |     |
| Receipt          |                |     |            |               |     |
| Offline Verify   |                |     |            |               |     |
| Tray             |                |     |            |               |     |
| Notification     |                |     |            |               |     |
| Secure Transport |                |     |            |               |     |

---

# §16 — THE MOST IMPORTANT DECISION

必须最终回答：

> **Developer Control Center MVP 是否值得继续开发？**

只允许三种结论：

```text
GO
GO WITH CONSTRAINTS
KILL
```

并且必须给出可证伪理由。

---

# §17 — GO / NO-GO CONDITIONS

## GO

只有满足：

```text
Machine Identity feasible
Pairing feasible
Secure transport feasible
Agent Host feasible
Runtime lifecycle feasible
Observe feasible
Stop feasible
Restart feasible
Logs feasible
History feasible
Receipt feasible
```

才允许：

```text
GO
```

---

## GO WITH CONSTRAINTS

允许：

```text
GO WITH CONSTRAINTS
```

例如：

```text
Attach 被缩小
WAITING 延后
Receipt 仅支持 Artifact
跨机器先限制 2 台
```

但：

```text
PAIR
IDENTITY
AUTHENTICATION
CONTROL
```

不得缺失。

---

## KILL

如果出现：

```text
Remote control cannot be secured
Machine ownership cannot be established
Agent lifecycle cannot be reliably observed
Core loop requires excessive infrastructure
Receipt provides no meaningful differentiation
Product collapses into generic monitoring
```

则：

```text
STOP
RETURN TO BRAND-0D.1
```

不得通过增加功能掩盖问题。

---

# §18 — IMPLEMENTATION GATE

本阶段默认：

```text
NO PRODUCTION IMPLEMENTATION
```

只有当 Technical Spike 发现某一个极小原型对于回答架构问题不可避免时，才允许建立：

```text
ISOLATED SPIKE
```

例如：

```text
temporary local process supervisor
temporary P-256 signing test
temporary pairing handshake
temporary local IPC proof
temporary receipt verification proof
```

所有 Spike 必须：

```text
isolated
documented
non-production
reversible
```

不得自动并入主架构。

---

# §19 — FINAL ARCHITECTURE DECISION

最终必须画出一个最小架构：

```text
┌────────────────────┐
                │  Developer Control │
                │      Center        │
                └─────────┬──────────┘
                          │
                     Local IPC
                          │
                ┌─────────▼──────────┐
                │   Machine Agent    │
                └────┬─────┬─────┬───┘
                     │     │     │
                     ↓     ↓     ↓
                 Runtime  Event Identity
                     │     Log
                     ↓
                Agent Session
                     │
                     │ secure transport
                     ↓
              Other Machines
```

必须根据真实审计结果修改这个图。

不得为了符合模板而保留不成立的组件。

---

# §20 — FINAL REPORT REQUIREMENTS

报告必须至少包含：

```text
§0 Absolute Rules
§1 Hard Baseline
§2 BRAND-0D.2 Inheritance
§3 Existing Architecture Re-audit
§4 Runtime Foundation Audit
§5 Session State Machine
§6 Minimum Agent Protocol
§7 Secure Machine Pairing
§8 P2P Reuse Decision
§9 Control Plane
§10 Attach Definition
§11 Event / History Foundation
§12 Receipt Feasibility
§13 Desktop Shell Feasibility
§14 Local-first Architecture
§15 Technical Feasibility Matrix
§16 Core Decision
§17 GO / NO-GO
§18 Implementation Gate
§19 Final Architecture
§20 Next Phase Gate
```

---

# §21 — FINAL DECISION FORMAT

报告最后必须明确：

```text
PHASE BRAND-0D.3 RESULT

PRODUCT:
Developer Control Center

PRODUCT DIRECTION:
PASS / FAIL

TECHNICAL FEASIBILITY:
PASS / PARTIAL / FAIL

SECURITY FOUNDATION:
PASS / PARTIAL / FAIL

RUNTIME FOUNDATION:
PASS / PARTIAL / FAIL

CONTROL PLANE:
PASS / PARTIAL / FAIL

RECEIPT:
PASS / PARTIAL / FAIL

P2P REUSE:
KEEP / ADAPT / REPLACE / DEFER

RECOMMENDATION:
GO / GO WITH CONSTRAINTS / KILL

NEXT PHASE:
<exact phase name>

IMPLEMENTATION:
NONE / SPIKE ONLY / AUTHORIZED

STOP:
YES
```

**本阶段完成后必须停止。**

不得：

```text
命名
品牌设计
最终 UI
完整 MVP
Production
Installer
Deployment
Push
Tag
Merge
Rebase
```

---

# §22 — 最重要的产品纪律

整个阶段必须始终记住：

> **我们不是在给 P2PChain 找用途。**
>
> **我们是在验证 Developer Control Center 是否值得存在，然后判断 P2PChain 哪些技术资产值得留下。**

如果某个 P2PChain 原有能力：

```text
UTXO
Mempool
PoW
Mining
Wallet UI
Blockchain UI
```

对产品没有必要：

> **不要为了复用资产而复用。**

如果某个全新的能力：

```text
Agent Host
Runtime Supervisor
Pairing
Authentication
Control Plane
```

是产品成立的必要条件：

> **必须承认它是新产品基础设施。**

最终目标不是：

```text
把区块链变成开发者工具
```

而是：

```text

做出一个真正有安装理由的
Developer Control Center

并让原有 P2PChain 技术
只在真正有价值的地方留下。
```


















