# PHASE BRAND-0D.2 — DEVELOPER CONTROL CENTER MVP VALIDATION & MINIMUM PRODUCT BOUNDARY

> 项目：P2PChain
>
> 前置阶段：
>
> - PHASE BRAND-0D
> - PHASE BRAND-0D.1 — DEVELOPER PRODUCT VALIDATION & PRODUCT CATEGORY CONVERGENCE
>
> 前置结论：
>
> **Developer Control Center = ADOPT**
>
> **Machine = CORE OBJECT**
>
> **Agent Session = CORE / P0**
>
> **Observe + Act = Primary Daily Value**
>
> **Event = Persistent Memory**
>
> **Attest / Verifiable Ledger = Differentiating Infrastructure / Potential Moat**
>
> 阶段性质：
>
> **PRODUCT VALIDATION / MVP BOUNDARY / ARCHITECTURAL VALIDATION**
>
> 本阶段唯一目标：
>
> > **确定一个足够小、但能够证明“Developer Control Center”产品类别真实成立的 MVP 边界，并验证现有 P2PChain 技术资产是否足以支撑这个 MVP。**
>
> 本阶段不是正式产品开发。
>
> 本阶段结束后必须停止。
>
> **不得自动进入品牌命名、UI 美化、完整产品实现或生产部署。**

---

# §0 — ABSOLUTE RULES

## 0.1 READ-FIRST

必须先完整阅读：

- `docs/PHASE-BRAND-0D-DEVELOPER-DESKTOP.md`
- `docs/PHASE-BRAND-0D.1-*`
- 所有与 BRAND / PRODUCT / P2P / Proof / architecture 相关的现有报告

不得重新发明 BRAND-0D / BRAND-0D.1 已经明确完成的结论。

---

# §0.2 NO CODE MODIFICATION BY DEFAULT

本阶段默认：

**READ-ONLY**

禁止：

- 修改 `.go`
- 修改 `.js`
- 修改 `.ts`
- 修改 `go.mod`
- 修改配置
- 修改数据库
- 修改 `blocks.dat`
- 修改 wallet
- 修改 P2P
- 修改 PoW
- 修改 UTXO
- 修改 Node
- 修改 API
- 修改 CLI
- 修改 UI

允许：

- 阅读源码
- 阅读测试
- 运行只读测试
- 建立架构映射
- 建立 MVP 模型
- 建立状态模型
- 建立事件模型
- 建立产品流程
- 编写本阶段报告

**除非本阶段明确出现单独的 implementation gate，否则不得修改生产代码。**

---

# §0.3 GIT

禁止：

- commit
- push
- pull
- merge
- rebase
- reset
- restore
- checkout
- clean
- revert
- stash
- tag

允许：

- status
- log
- show
- diff
- diff --stat
- diff --name-only
- rev-parse
- branch --show-current
- ls-files

---

# §0.4 CONCURRENT WORK PROTECTION

首先检查工作区是否存在并发修改。

特别注意此前已经存在的：

**PHASE P3.1**

不得：

- restore
- clean
- reset
- checkout
- delete
- overwrite
- format
- 自动修复

任何并发修改。

如果存在并发工作：

1. 完整记录
2. 隔离
3. 不依赖未提交代码作为产品能力证据
4. 以指定 Git baseline 为主要技术证据

---

# §1 — HARD BASELINE

建立完整 HARD BASELINE：

记录：

- project path
- OS
- architecture
- Go version
- current branch
- HEAD
- parent commit
- working tree
- staged changes
- untracked files
- package count
- test count
- build result
- vet result

运行：

- `go test ./...`
- `go build ./...`
- `go vet ./...`

测试失败：

**不得修复。**

只记录。

---

# §2 — READ BRAND-0D.1

完整读取 PHASE BRAND-0D.1。

必须明确继承以下结论：

```text
PRODUCT CATEGORY
=
Developer Control Center
```

```text
PRIMARY USER
=
Developer with ≥2 machines
+
long-running workloads
+
no platform team
```

```text
CORE OBJECT
=
Machine
```

```text
P0 OBJECT
=
Agent Session
```

```text
PRIMARY VALUE
=
Observe + Act
```

```text
DIFFERENTIATION
=
Attest
```

```text
INFRASTRUCTURE
=
Verifiable Event Ledger
```

---

# §3 — THE CENTRAL MVP QUESTION

必须回答：

> **What is the smallest product that makes a developer say: “I want this running on my machines”?**

不得回答：

- because it uses blockchain
- because it has PoW
- because it has P2P
- because it has cryptographic proof
- because it is decentralized
- because it is a desktop app

这些全部不是安装理由。

必须围绕：

```text
My machines
My running work
My agents
My state
My control
My history
```

建立答案。

---

# §4 — DEFINE THE MVP JOB

建立：

## PRIMARY JOB

一个开发者可以：

> **在一个地方看到自己的多台机器正在运行什么，并在需要时直接控制这些运行中的任务。**

## SECONDARY JOB

> **当事情发生异常或完成后，开发者可以回看发生了什么。**

## DIFFERENTIATING JOB

> **对于关键事件，开发者可以获得一个不泄露原始内容、但可以被独立验证的 Receipt。**

必须明确：

```text
Primary Job
≠
Secondary Job
≠
Differentiating Job
```

---

# §5 — MVP OBJECT MODEL

验证以下对象：

```text
Machine
Process
Service
Agent Session
Event
Action
Receipt
Identity
Workspace
```

逐一回答：

- 是否 MVP 必需
- 是否 CORE
- 是否 SUPPORTING
- 是否 FUTURE
- 是否 DROP

特别要求：

**不得因为现有代码存在某个对象，就自动将其纳入 MVP。**

---

# §6 — MACHINE-FIRST MODEL

必须建立 Machine 第一性模型：

```text
Machine
├── Identity
├── Connectivity
├── Availability
├── Health
├── Processes
├── Services
├── Agent Sessions
├── Recent Events
└── Available Actions
```

回答：

> 为什么 Machine 是产品第一性对象，而不是 Node / Server / Workspace？

必须确保用户完全不需要理解：

- blockchain node
- wallet
- UTXO
- mempool
- PoW
- mining

---

# §7 — AGENT SESSION MODEL

必须重点设计：

```text
Agent Session
├── Machine
├── Started At
├── Current State
├── Runtime
├── Last Activity
├── Current Task
├── Process
├── Exit Status
├── Recent Events
└── Actions
```

至少验证以下状态：

```text
RUNNING
IDLE
WAITING
COMPLETED
FAILED
LOST
UNKNOWN
```

重点回答：

> **开发者离开机器 30 分钟以后回来，最想知道什么？**

不要从 UI 功能出发。

从信息需求出发。

---

# §8 — OBSERVE MVP

定义最小 Observe：

开发者打开 Control Center 后：

**10 秒以内必须知道：**

1. 有多少机器
2. 哪些机器在线
3. 哪些机器异常
4. 哪些 Agent 正在运行
5. 哪些任务刚刚完成
6. 是否有需要人工处理的事件

禁止：

- 大型 dashboard
- 复杂 analytics
- Grafana clone
- metrics wall
- 100 个指标
- 复杂图表

原则：

> **Daily-glance > Dashboard exploration**

---

# §9 — ACT MVP

必须确定最少控制能力。

候选：

```text
Open terminal
Start process
Stop process
Restart process
Open project
Reconnect session
View logs
```

逐项判断：

- P0
- P1
- FUTURE
- DROP

特别验证：

> 如果没有 ACT，本产品是否只是一个监控面板？

如果答案为 YES：

**ACT 必须属于 MVP。**

---

# §10 — EVENT MVP

建立 Event 最小模型：

```text
Event
├── id
├── timestamp
├── machine_id
├── actor
├── type
├── summary
├── severity
├── metadata
└── integrity
```

明确区分：

```text
EVENT
=
What happened
```

与：

```text
PROOF
=
Can I independently establish that it happened
```

禁止：

**Proof everything.**

---

# §11 — ATTEST MVP

这是本阶段最重要的验证之一。

必须重新质疑：

> **Proof 是否真的应该进入 MVP？**

逐项判断：

```text
Build
Test
Commit
Artifact
Release
Deploy
Config Change
Environment
Machine State
Agent Session
```

对每个事件回答：

1. 是否值得证明
2. 是否需要身份
3. 是否只需要 hash
4. 是否必须留在本机
5. 是否需要第三方验证

最终输出：

```text
MVP ATTEST EVENTS
```

最多允许选择：

**3 类。**

禁止建立：

**Universal Proof Engine**

---

# §12 — REMOVE-BLOCKCHAIN TEST

重新进行思想实验：

> 如果产品首页完全不出现 Blockchain，是否仍然成立？

必须验证：

```text
Observe
Act
Agent Sessions
Events
History
Receipts
Independent Verification
```

全部是否成立。

如果任何核心能力必须依赖用户理解 blockchain：

**STOP AND REPORT PRODUCT FAILURE**

---

# §13 — REMOVE-PROOF TEST

反向进行：

> 如果完全删除 Proof，产品还值得安装吗？

必须给出：

```text
YES / NO / PARTIAL
```

如果：

**NO**

说明产品仍然过度依赖 BRAND-0D 原始技术驱动。

如果：

**YES**

说明：

```text
Observe + Act
=
Product
```

```text
Attest
=
Differentiation
```

这是理想结构。

---

# §14 — EXISTING ASSET FIT

建立：

| Existing Asset | MVP Role | KEEP / ADAPT / HIDE / DEFER / REMOVE |
| -------------- | -------- | ------------------------------------ |
| Node           |          |                                      |
| P2P            |          |                                      |
| Storage        |          |                                      |
| Blockchain     |          |                                      |
| PoW            |          |                                      |
| Wallet         |          |                                      |
| UTXO           |          |                                      |
| Mempool        |          |                                      |
| CLI            |          |                                      |
| Control API    |          |                                      |

特别检查：

> **MVP 是否必须依赖 UTXO？**

如果答案为 NO：

不得为了复用旧代码而强行保留在产品模型中。

---

# §15 — P2P BOUNDARY

必须明确：

本产品不是：

```text
public blockchain
```

不是：

```text
permissionless network
```

不是：

```text
crypto network
```

而是：

```text
MY MACHINE FLEET
```

建立：

```text
Fleet
├── Machine A
├── Machine B
├── Machine C
└── Machine D
```

验证：

- machine identity
- machine ownership
- peer authentication
- encrypted transport
- fleet boundary
- unauthorized peer rejection

注意：

**本阶段只做设计与缺口识别，不实现。**

---

# §16 — COMPETITOR BOUNDARY

重新检查 MVP 是否滑向：

- VS Code
- Cursor
- Docker Desktop
- Portainer
- Grafana
- Tailscale
- Termius
- GitHub
- Uptime Kuma

建立：

```text
We are NOT:
X
X
X
```

同时建立：

```text
We ARE:
________
```

要求：

一句话能够表达。

---

# §17 — MINIMUM PRODUCT LOOP

重新定义最终 MVP Loop。

不得机械复制：

```text
JOIN → OBSERVE → ACT → ATTEST
```

必须根据本阶段真实结果判断是否仍成立。

最终输出：

```text
MVP CORE LOOP
```

并证明每一步为什么存在。

---

# §18 — 10-MINUTE VALUE TEST

模拟一个真实开发者：

```text
09:00
开始工作

09:30
启动 Agent

10:30
离开电脑

12:00
回来

12:01
打开 Control Center
```

必须回答：

**12:01 他需要看到什么？**

然后：

```text
12:02
发现 Agent failed

12:03
他做什么？

12:05
任务重新运行

12:30
任务完成

12:31
他如何知道任务结果可信？
```

形成完整真实工作流。

---

# §19 — MVP FEATURE CUT

必须建立三个列表：

## MUST HAVE

最多：

**7 项**

## SHOULD HAVE

最多：

**7 项**

## NOT MVP

不限。

特别把以下项目逐项处理：

- blockchain UI
- wallet UI
- mining UI
- UTXO
- mempool
- token
- payments
- social
- collaboration
- marketplace
- AI chat
- analytics
- container management
- Kubernetes
- full CI/CD
- remote desktop

原则：

> **如果不是证明核心产品价值所必需，就不进入 MVP。**

---

# §20 — MVP SUCCESS CRITERIA

建立可证伪标准。

至少包括：

### S1

开发者能在一个界面看到 ≥2 台机器。

### S2

开发者能看到 Agent Session 状态。

### S3

开发者能识别异常。

### S4

开发者能执行至少一个有价值的控制动作。

### S5

关键事件可以形成 Receipt。

### S6

Receipt 可以离线独立验证。

### S7

原始敏感内容不离开本机。

### S8

用户完全不知道 Blockchain，也能理解产品。

### S9

用户不需要理解密码学，也能使用产品。

### S10

移除 Proof 后，产品仍然具有明显日常价值。

---

# §21 — KIL














