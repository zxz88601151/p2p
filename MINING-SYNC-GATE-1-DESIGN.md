# MINING-SYNC-GATE-1 — 设计说明（自动矿工「同步门」）

> **阶段**：`MINING-SYNC-GATE-1`（同步门）+ `ON-DEMAND-MINING-REMOVAL-1`（按需出块全量下线，同步改写 §4）
> **状态**：**IMPLEMENTED / VERIFIED**（验收证据见 `MINING-SYNC-GATE-1-REPORT.md`）
> **基线**：HEAD `467621769c19f455e10e575dd1213fa6d02ca2c1` + 本阶段改动
> **真值源**：可执行实现 + 可重复测试。本文是设计描述，不是真值源。

---

## §0 问题与目标

### 现象

新数据目录的节点带 `-mine` 启动时，会在**追上网络之前**按本机链尾自铸大量区块：

- v1 难度规则（高度 < `pow.ActivationHeight = 2000`）把 bits 钉死为 16；
- 于是出块几乎瞬时（本机实测 **~76 块/秒**）；
- 实测 5 分钟内自铸 **72 块**，全部被对端更重的链 reorg 丢弃 ⇒ **纯浪费**。

### 目标

让自动矿工在「本地明显落后于网络」时**暂停出块**，追上后**自动恢复**。

### 铁律（本阶段硬约束）

| # | 约束 | 落实方式 |
|---|---|---|
| 1 | **不动共识真值** | 高度 / 难度 / 区块规则零改动；`internal/blockchain`、`internal/pow`、`internal/utxo` 逐字节未改 |
| 2 | **不动存储字节格式** | `internal/storage` 未改 |
| 3 | **不动 P2P 协议面** | 不新增消息类型、不改握手协议；只**读**既有 `HandshakePayload.ChainHeight/ChainWork` |
| 4 | **任何失败先停下报告** | 实施过程中每次失败均停下定位，未采用「修一下继续」 |

---

## §1 对端高度登记表

**位置**：`cmd/node/service.go`

```go
const (
    // 允许落后的块数：本地高度 + 该值 >= 对端最高高度 即视为已追上。
    miningSyncLagBlocks = 5
    // 条目存活时长：无断开回调，条目靠时间自愈。
    miningSyncPeerTTL = 10 * time.Minute
)

type peerHeightEntry struct {
    height int       // 对端握手时自报的链高
    work   string    // 对端握手时自报的累积工作量（十进制；空 = 未知/旧节点）
    at     time.Time // 记录时刻，仅用于 TTL 过期判定
}
```

`nodeService` 新增字段（受既有 `mu` 保护）：

```go
peerHeights map[string]peerHeightEntry   // 键 = 对端地址
```

**写入点**：`OnHandshake` 的**第 (0) 步**，在既有三步（追赶 / 分支发现 / 启动恢复）之前：

```go
s.recordPeerHeight(peerAddr, payload.ChainHeight, payload.ChainWork)
```

- **纯观测**：不改变 `OnHandshake` 任何既有分支的行为。
- **惰性初始化**：`recordPeerHeight` 内对 nil map 兜底（防 panic），不依赖构造顺序。
- **绝不持久化**：对端高度是易失的运行时观测，重启后由下一次握手重建；落盘只会引入又一份需要在崩溃恢复中证明正确性的状态。
- **TTL 自愈**：本节点**没有对端断开回调**，条目只能靠时间过期，避免「已消失的对端永久挡住挖矿」。

---

## §2 判定函数

### 纯函数内核（可穷举测试，不读链、不读全局状态）

```go
func miningSyncGateDecision(localHeight int, localWork *big.Int,
                            entries []peerHeightEntry, now time.Time) (ready bool, peerMax int)
```

判定流程：

1. `peerMax` 初值 = `localHeight`。
2. 逐条遍历 `entries`：
   - `now.Sub(e.at) > miningSyncPeerTTL` ⇒ **跳过**（条目过期，TTL 自愈）；
   - `!shouldSyncFrom(e.work, e.height, localWork, localHeight)` ⇒ **跳过**
     （**复用既有 work-priority 逻辑**：对端工作量不比我大 ⇒ 它不是我的同步源）；
   - 否则 `peerMax = max(peerMax, e.height)`。
3. `ready = localHeight + miningSyncLagBlocks >= peerMax`。

### 装配层

```go
func (s *nodeService) miningSyncReady() (ready bool, local int, peerMax int)
```

读链高/链尾工作量 → 在锁内**拷贝**登记表（不持锁做判定）→ 交给纯函数。

### 关键边界语义

| 情形 | 判定 | 理由 |
|---|---|---|
| **无有效对端** | `ready = true` | 孤立节点（无种子 / 单机开发链）不应被永久挡住挖矿 |
| 对端 TTL 过期 | 忽略该条 | 靠时间自愈，无需断开回调 |
| 对端链**更长但更轻** | 忽略该条 | 复用 work-priority：低难度长链不是同步源（M4 语义） |
| 落后 100 块 | `ready = false` | 明显落后，暂停挖矿 |
| 落后 3 块（< lag=5） | `ready = true` | 容忍新块传播抖动，避免正常运行时误停挖矿 |
| 与对端等高 | `ready = true` | `shouldSyncFrom` 要求**严格更重**，等高不算同步源 |

---

## §3 闸门插入点

**位置**：`cmd/node/main.go` 的 `runMiner` 主循环，在 `mineOnce` 之前。

```go
if ready, local, peerMax := svc.miningSyncReady(); !ready {
    entered := svc.setMineState(MiningWaitingSync, "behind-network")
    if entered || syncWaitTicks%miningSyncWaitLogEvery == 0 {
        log.Printf("[miner] 等待同步：本地高度=%d，对端最高=%d，暂停自动挖矿", local, peerMax)
    }
    syncWaitTicks++
    // stop-aware 等待：绝不裸 sleep，保证停止请求能被立即响应。
    if sleepOrStop(stop, miningSyncWaitInterval) {
        finish(MiningStopped, "stopped")
        return
    }
    continue
}
syncWaitTicks = 0
```

设计要点：

- **绝不裸 sleep**：等待走 `sleepOrStop(stop, …)`，保证 `POST /mine/stop` 与节点停机路径都能被**立即**响应（这是 `STOP-INV-05` 一类不变量在挖矿循环里的延续）。
- **不执行任何 PoW**：未就绪时循环直接 `continue`，不组装候选区块、不调用 `mineOnce`。
- **状态可观测**：新增 `miningState` 枚举值 `WAITING_SYNC`（`cmd/node/mining_state.go`），`/status` 的 `mining_state` 字段与 CLI `status` 的「挖矿状态」行都如实反映。

---

## §4 范围限定（**已随按需出块下线改写**）

> 本节是 `ON-DEMAND-MINING-REMOVAL-1` 的同步改写结果。原文见本节末「已作废的原文」。

### §4.1 改写后的真值

1. **按需出块已全量下线**：`POST /mine`、`POST /console/mine`、CLI `node mine`、
   `nodeService.Mine`、`control.Client.Mine`、Console 页面按钮、GUI「立即出块」卡片
   全部移除；同源闸门 `consoleOriginGate` / `isSameOriginRequest` 随之退役。
   ⇒ 原条款所要「豁免」的那个入口**在物理上已不存在**，豁免条款随之删除。

2. **同步门作用于全部挖矿入口**。闸门位于 `runMiner` 主循环内部，而 `runMiner`
   是**唯一的挖矿路径**——启动期 `-mine` 与 `POST /mine/start` 都经
   `minerLifecycle.start()` → `runMiner`。因此范围限定退化为一句话：

   > **同步门覆盖本节点的一切挖矿入口：启动期 `-mine` 与 `POST /mine/start`。**

3. **不再有任何绕过路径**：控制面不存在第二个出块入口，也就不存在「从旁路绕过闸门」
   的可能。这同时收窄了控制面的 mutation 面（mutation 6 条 → 4 条）。

### §4.2 为什么不再需要例外条款

原例外有两条理由，逐条复核：

| 原理由 | 复核结论 |
|---|---|
| (a) `POST /mine` 是**显式运维动作**，应当优先于自动策略 | **有意否决**（见下） |
| (b) 门控它会**破坏冒烟测试** | **已消解**：`scripts/smoke-e2e.sh` 改用 `-mine -maxblocks 12` 精确出块 + `POST /mine/start` 打包交易，实测 **20/20 通过** |

**对理由 (a) 的否决依据**：`POST /mine/start` 的语义就是「开始挖矿」，而本特性的全部目的
正是「本地落后于网络时不要挖矿」。在落后状态下响应「开始挖矿」会立刻产生**与本特性要
消除的完全相同的浪费**（自铸区块随后被 reorg 丢弃）。运维意图**不会被静默吞掉**：

- 日志：`[miner] 等待同步：本地高度=%d，对端最高=%d，暂停自动挖矿`；
- `/status`：`mining_state = "WAITING_SYNC"`，`mining_reason = "behind-network"`。

**若 Owner 日后需要「强制挖矿」逃生门**，应作为**独立控制面特性**另行立项
（例如为 `POST /mine/start` 增加 `force` 参数），本阶段不实现——以免重新打开
被本次下线收窄的 mutation 面，也避免让一个安全特性自带后门。

### §4.3 已作废的原文（保留以便审计）

> **§4 范围限定（原文，已作废）**
> 只门控 `-mine` 自动矿工；`/mine` RPC 按需出块**不得**被门控
> （显式运维动作 / 开发测试工具；门控它会破坏冒烟测试）。

---

## §5 可观测性

| 通道 | 内容 |
|---|---|
| 日志 | `[miner] 等待同步：本地高度=%d，对端最高=%d，暂停自动挖矿`（进入等待时 1 条 + 每 `miningSyncWaitLogEvery=12` 次复查 1 条，节流不刷屏） |
| `/status` | `mining_state = "WAITING_SYNC"`、`mining_reason = "behind-network"` |
| CLI `status` | 「挖矿状态」行渲染为「等待同步（本地落后于网络，暂停自动挖矿）」 |
| Developer Console | 挖矿行渲染为「等待同步：本地落后，暂停自动挖矿」；`miningHint` 提示「自动挖矿：等待同步…」 |
| GUI（P2PChain Studio） | `mining_state` 经 `theme.mining_state_color` 渲染为 WARN 色 |

---

## §6 已知局限（v1 接受）

### 6.1 对端更重但同步持续失败 ⇒ 持续压制

> 若对端**更重但同步持续失败**（例如网络隔离、对端长期不响应），同步门会**持续挡住挖矿**。
>
> - 这是 **v1 有意接受**的取舍：宁可不出块，也不产生必然被丢弃的区块。
> - 可观测性保证「为什么没出块」不会被误报为矿工故障（§5）。
> - 后续版本可增加「同步放弃后放行」策略（例如：对端连续 N 次同步失败 ⇒ 降级放行）。
>   该策略需要额外的失败计数与判定，属独立阶段。

该局限已写入代码注释（`cmd/node/service.go` 的常量块与 `miningSyncReady` 文档注释）。

### 6.2 握手高度谎报 ⇒ **单个**对端即可压制自动挖矿

**向量**：`peerHeights` 登记的是对端在**握手时自报**的 `ChainHeight` / `ChainWork`
（`p2p.HandshakePayload` 字段），本节点**不做任何验证** —— 既无法证明该高度对应的区块
真实存在，也无法证明其累积工作量真实。

**后果**：**单个**恶意（或有缺陷的）对端只要谎报一个极高高度，就能使
`miningSyncGateDecision` 判定「本地落后」，从而**压制本节点的自动挖矿**：
`ready = localHeight + miningSyncLagBlocks >= peerMax` 在 `peerMax` 被抬到极高时恒为 `false`。
条目 TTL 为 `miningSyncPeerTTL = 10 * time.Minute`，而攻击者**断线重连即刷新条目**，
因此可长期维持压制（每次重连都重新登记一条谎报条目）。

**为什么 v1 有意接受**：

- 当前部署形态是 **3 节点可信内网**（自建、无匿名入网、无对手方动机），风险可控；
- 攻击面是**可用性**（拒绝出块），**不是正确性**：同步门是纯策略层闸门，不触碰高度/难度/
  出块规则；被压制期间**不会产生任何非法区块**，也不会污染共识状态或存储；
- 压制**可观测**：日志 `[miner] 等待同步：本地高度=%d，对端最高=%d，暂停自动挖矿` 与
  `/status` 的 `mining_state = WAITING_SYNC` 会直接暴露被谎报的「对端最高」值，
  运维可据此识别异常（该值远超全网合理高度）；
- 与 §6.1 同源：二者都是「宁可不出块，也不产生必然被丢弃的区块」这一取舍的代价。

**后续加固方向（二选一，单独立项；本阶段不实现）**：

| 方案 | 内容 | 代价 / 待决问题 |
|---|---|---|
| **A. 多 peer 佐证** | 仅当 **≥2 个独立对端**都报告更高（且更重）时才 gate；单对端谎报不足以压制 | 需先定义「独立」（IP / 网段 / 节点 ID 维度）；并处理「全网只有 1 个真实对端」时是否退化为不 gate |
| **B. 纳入 P0-5 失信记分** | 把「高度/工作量谎报」纳入既有 P0-5 记分体系，按最高档 `scoreInvalidBlock = 50` 处理：**记分 + 立即断开**；累计达 `banThreshold = 100` 即封禁 `banDuration = 10 分钟` | **前置条件**：必须实现「声明 vs 可交付」校验（真正去同步、确认对端交付不出所声称的高度才可判谎报）。否则「谎报」与「诚实但暂时不可达」在观测上不可区分，贸然记分会**误封诚实对端** |

> **注**：方案 B 的「声明 vs 可交付」校验是**必要条件而非可选项** —— 缺了它，
> 记分规则会把网络抖动误判为恶意。

**登记位置说明**：本局限（§6.2）目前**只在本设计文档登记**，**尚未写入代码注释**
（本阶段限定「只改文档、不碰代码」）。建议随方案 A 或 B 任一落地时一并补入
`cmd/node/service.go` 的 `peerHeights` 字段注释与 `miningSyncReady` 文档注释。

---

## §7 不变量清单（供回归锁定）

| # | 不变量 | 锁定方式 |
|---|---|---|
| I1 | 无有效对端 ⇒ 允许挖矿 | `TestMiningSyncGateDecision` 表驱动 |
| I2 | TTL 过期条目被忽略 | 同上 + `TestMiningSyncReadyWiring` |
| I3 | 低工作量长链不阻断挖矿 | 同上 |
| I4 | 落后 100 块 ⇒ 阻断 | 同上 |
| I5 | 落后 3 块（< lag=5）⇒ 放行 | 同上 |
| I6 | 登记表惰性初始化不 panic | `TestRecordPeerHeightLazyInit` |
| I7 | 端到端：落后时 0 个 `MINING_BLOCK_ACCEPTED`，追平后恢复出块 | `TestMiningSyncGateBlocksAutoMinerUntilCaughtUp` |
| I8 | `POST /mine`、`POST /console/mine` 恒 `404`（与请求头/令牌无关） | `internal/control` 的 `TestOnDemandMineEndpointsRemoved` |
| I9 | 控制台页面不残留出块按钮/脚本 | `TestConsolePageHasNoMiningButton` |
| I10 | CLI 无 `mine` 子命令 | `TestCLIMineSubcommandRemoved` |
| I11 | 共识/存储/P2P 面无改动 | `git status` 复核（见验收报告） |
