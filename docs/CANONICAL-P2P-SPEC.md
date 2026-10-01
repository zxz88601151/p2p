# P2PChain — CANONICAL P2P PROTOCOL SPEC

> **本文是 P2PChain 网络层协议的规范描述（CANONICAL SPECIFICATION），描述当前可执行源码的行为。**
> 本文**不是**真值源：最高真值为**可执行实现 + 可重复测试**。冲突时以可执行实现为准并登记 `DOCUMENTATION DRIFT`；本文随实现漂移同步修订。

- **建立/治理修正阶段**：初稿由第二写入者于 2026-10-01 01:11 落盘（未提交）；经 `DOCUMENT AUTHORITY RECONCILIATION-1` 审计发现权威缺陷；`DOCUMENT AUTHORITY GOVERNANCE CLOSURE-1` 治理修正（删除"唯一真值源"歧义、明确 L0 终审、修正来源声明）。
- **适用基线**：HEAD `434f8c7c6cc39585c8c31deb35e33b27bef1edb2`
- **主要证据**：`internal/p2p/node.go`（832 行）、`cmd/node/service.go`
- **状态**：**CANONICAL / CURRENT**

---

## §0 定位

本包只负责**传输与连接管理**，不解释共识语义：收到的原始载荷交给上层 Handler（区块验证、交易校验、入池、上链由 `blockchain` / `mempool` 层完成）。

---

## §1 Transport

| 项 | 规则 |
|---|---|
| 传输 | **TCP 长连接** |
| 帧格式 | **每行一个 JSON 消息**，以 `\n` 结尾 |
| 信封 | `Message{ "type": <MessageType>, "payload": <json.RawMessage> }` |
| 区块/交易载荷 | 以**规范编码的十六进制字符串**传输（`block.Encode()` / `tx.Encode()`），与磁盘/共识层使用同一字节表示 |

---

## §2 消息类型（**7 条，不是旧 README 的 5 条**）

| 类型 | 方向 | Payload | 说明 |
|---|---|---|---|
| `handshake` | 双向 | `HandshakePayload` | 建立连接后交换节点 ID/高度/监听地址/genesis/已知节点/链工作量/链尾哈希 |
| `new_block` | 广播 | `BlockPayload{encoded}` | 完整区块；接收端完整校验后上链并中继（排除来源） |
| `new_tx` | 广播 | `TxPayload{encoded}` | 完整交易；入池（组合视图验证）后中继 |
| `get_blocks` | 请求 | `GetBlocksPayload{from_height, count}` | 按高度追赶同步 |
| `blocks_resp` | 响应 | `BlocksRespPayload{encoded_blocks[], done}` | 区块批次 |
| `get_block_by_hash` | 请求 | `GetBlockByHashPayload{hash, max_ancestors}` | **按哈希取块及其祖先**（REORG-1H，孤儿补齐） |
| `block_by_hash_resp` | 响应 | `BlockByHashRespPayload{blocks[], hash, found}` | 按哈希响应 |

> 🔴 旧 README 只列 5 条，遗漏 `get_block_by_hash` 与 `block_by_hash_resp` ⇒ 登记 `DOCUMENTATION DRIFT`。

### 2.1 HandshakePayload

```go
type HandshakePayload struct {
    NodeID      string   `json:"node_id"`
    ChainHeight int      `json:"chain_height"`
    ListenAddr  string   `json:"listen_addr"`
    GenesisHash string   `json:"genesis_hash"` // 快速识别网络不一致
    KnownPeers  []string `json:"known_peers"`  // 节点发现
    ChainWork   string   `json:"chain_work"`   // 累积工作量（十进制串；空 = 未知）
    TipHash     string   `json:"tip_hash"`     // 链尾哈希（hex；空 = 未知）
}
```

- `ChainWork` / `TipHash` 为**可选**字段（缺失即未知，零值 = 对端未提供）⇒ 旧版本节点握手不含二者，接收方自动退化为"按高度比较"，**协议向后兼容**；
- `ChainWork` 让同步判据从"谁更高"升级为"**谁的工作量更大**"（work-aware）；
- `TipHash` 让本节点发现"对端在一条我不认识的分支上"，从而按哈希拉分支。

### 2.2 BlockByHashRespPayload 顺序契约

`Blocks` **索引 0 = 被请求的区块**，其后依次是父、祖父……（**由新到旧**）。请求方**从后往前**应用，天然满足"父先于子"的插入约束。

---

## §3 Handshake 与兼容性

| 项 | 规则 |
|---|---|
| 时机 | 连接建立后**必须**在 `handshakeTimeout` 内收到握手 |
| 强制方式（R1-B） | 未完成握手前，**每次读**都以 `handshakeTimeout` 为 deadline；超时即断开。完成后恢复常规 `readTimeout` |
| genesis 校验 | `GenesisHash` 不一致 ⇒ 不兼容，拒绝 |
| 重复对端 | 已注册的地址/节点 ⇒ 拒绝或去重 |
| 未知消息类型 | 忽略并记录（不中断连接） |

---

## §4 Peer lifecycle

| 阶段 | 规则 |
|---|---|
| **外拨 connect** | `net.DialTimeout("tcp", addr, 5s)`；外拨前预检 `len(peers) >= maxPeers` ⇒ 拒绝外拨 |
| **入站 accept** | 受 `maxInbound` / `handshakeQuota` / `maxPeers` 三重限额约束 |
| **握手** | 见 §3 |
| **读写** | 每连接 **reader goroutine + writer goroutine**；`p.mu` 保证同一连接上的写**永不交错** |
| **disconnect** | `maxSendFailures` 连续失败 → `dropPeer`；写超时；读超时；握手超时 |
| **reconnect** | 种子节点断线后由后台协程**周期性补齐连接**（README 记 5 秒） |

### 4.1 连接资源限额（R1-B，SEC-CLOSE MUST FIX 2）

| 常量 | 值 | 作用 |
|---|---|---|
| `maxInbound` | **125** | 入站连接上限（含未完成握手者） |
| `maxPeers` | **128** | 对端总数上限（入站 + 外拨） |
| `handshakeQuota` | **32** | 同时处于"已注册未握手"状态的连接配额 |
| `maxSendFailures` | **3** | 连续发送失败阈值 |

> 目的：堵住无上限 accept 的资源耗尽面（连接洪水 / 慢握手槽位占用 / 日食式连接挤占）。测试按生产值直接验证。

---

## §5 区块传播

1. 收到 `new_block` → 解码 → **完整共识校验**（PoW / 父哈希 / 版本 / bits / 时间戳 / Merkle / 体积 / 交易状态迁移）；
2. 校验通过 → 上链（可能触发 reorg，见 §8）→ `BroadcastExcept(msg, sourcePeer)` **中继并排除来源对端**；
3. 校验失败 → 拒绝，不中继。

---

## §6 交易传播

1. 收到 `new_tx` → 解码 → **组合视图验证**（链 UTXO + 内存池叠加）；
2. 通过 → 入池 → 中继（排除来源）；
3. coinbase 交易**禁止入池**（`ErrCoinbaseIn`）。

---

## §7 同步（Sync）

### 7.1 按高度追赶

| 项 | 值 | 位置 |
|---|---|---|
| 请求批量（请求方） | `MaxSyncBatch = 200` | `cmd/node/service.go:34` |
| 响应上限（服务端裁剪） | `MaxBlocksPerResp = 500` | `internal/p2p/node.go:52` |
| 响应来源 | `Blockchain.BlocksFrom(from, count)` | `blockchain.go:291` |
| 续传 | `done == false`（未到链尾）⇒ 继续请求下一批，逐块校验上链 | |

> 请求方每批请求 **200** 块（旧 README 的"每批 ≤200"在**请求侧**成立）；服务端上限 500 为服务端裁剪值，二者不冲突但须分别标注。

### 7.2 按哈希取分支（REORG-1H）

| 项 | 值 |
|---|---|
| 单次回溯上限 | `MaxAncestorsPerResp = 64` |
| 在途区块上界推导 | `4 × MaxSyncBatch = 800` |
| 孤儿等待条目上界 | `256 × 64 = 16,384` |

**有界的必要性**：孤儿块不知道缺口有多深，若无上限，一个恶意/故障对端可诱导本节点在长链上无限回溯，吃光内存与带宽。64 个祖先足以覆盖"几分钟分区"级别的正常分叉；更深的缺口由请求方**分多轮**补齐（每轮同样有上限与轮次上限）。

---

## §8 Fork / Reorg 交互

| 场景 | 网络行为 |
|---|---|
| 收到已知父的**分叉块** | 加入 blocktree → 基于父分支 UTXO 完整校验 → `ShouldReorg`（累积工作量比较）→ 是则 `executeReorg`，否则存为 **detached** 不切换 canonical |
| 收到**缺父孤儿** | `ErrOrphanParent` → `deferOrphan` 登记等待 + 向来源对端发 `get_block_by_hash` 补齐祖先 → 补齐后级联解析 |
| 对端在未知分支上（由 `TipHash` 发现） | 按哈希拉分支 |
| 重复投递 | 已存在于 canonical ⇒ 显式拒绝（重复持久化防护）；已在 blocktree ⇒ 幂等返回 |

观测事件（供诊断）：`ORPHAN_SEEN` / `ORPHAN_DUPLICATE_IGNORED` / `ORPHAN_REJECTED_PREPARK` / `WAITING_PARENT_ADD` / `RECOVERY_ELIGIBLE` / `RECOVERY_DROPPED` / `FORK_VALIDATION_START` / `FORK_VALIDATION_END`。

**完整 orphan pool 与重播策略属 REORG-1G/B5，明确未做**（见 `docs/LIMITATIONS.md`）。

---

## §9 重复抑制（Duplicate suppression）

| 层 | 机制 |
|---|---|
| 广播回环 | `BroadcastExcept(msg, except)` 排除来源对端 |
| 区块重复 | canonical 链哈希查重（显式拒绝重复持久化）；blocktree `ErrDuplicateHash` ⇒ 幂等 |
| 孤儿重复 | `ORPHAN_DUPLICATE_IGNORED` |
| 交易重复 | 内存池 TxID 去重（`Mempool.Has`） |
| 未知消息类型 | 忽略并记录 |

---

## §10 超时

| 超时 | 值 | 适用 |
|---|---|---|
| `DialTimeout` | **5 s** | 建立 TCP 连接 |
| `handshakeTimeout` | **10 s** | 握手完成前每次读的 deadline |
| `readTimeout` | **60 s** | 握手完成后的读 |
| `writeTimeout` | **10 s** | 单次写（每连接写前设置 deadline） |

---

## §11 消息与资源限制

| 限制 | 值 |
|---|---|
| `MaxMessageSize` | **1 MiB**（与区块体积上限同量级）；写入侧 `len(data)+1 > MaxMessageSize` ⇒ 拒绝；读取侧行长度超限 ⇒ 拒绝 |
| mutation RPC 请求体 | `MaxBytesReader` 4 KiB（`1<<12`） |
| `maxTxPerBlock` | 100,000 |
| `maxInputsPerTx` / `maxOutputsPerTx` | 10,000 / 10,000 |

---

## §12 Backpressure —— **旧 HOL limitation 已关闭，新 limitation 已登记**

### 12.1 旧 limitation：`BroadcastExcept` 同步中继 ~30s HOL blocking

**状态：CLOSED / FIXED（R1-A 已在代码中修复）。**

修复后行为（`internal/p2p/node.go:386-448`）：

- `broadcastExcept` **只做序列化 + 每对端非阻塞入队**；
- 实际写出由该连接**唯一的 `peerWriter` goroutine** 承担（FIFO，经 `p.mu` 与直接写互斥）；
- 入队为 `select { case p.outQ <- m: default: drop }` ⇒ **队满即对该对端丢弃本条，绝不阻塞调用方**（尤其挖矿临界区）与其他健康对端；
- 调用方（含 `commitMinedBlock → broadcastBlock`）**永不被慢对端的 TCP 写阻塞**。

⇒ **不得再作为 OPEN limitation 登记**。遗留：登记该 limitation 的历史报告需加"已于 R1-A 修复"批注。

### 12.2 新 limitation：**队列满 → 消息被丢弃 → 无重传**

| 字段 | 内容 |
|---|---|
| **LTM-ID** | **LTM-001** |
| **Severity** | MEDIUM |
| **Trigger** | 某对端的出站队列（`outboundQueueCap = 256`）被填满；典型触发 = 该对端消费过慢（其 writer 正被 `writeTimeout` 卡住）或链路拥塞 |
| **Impact** | 该对端**静默丢失**本次广播的区块/交易。它不会立刻知道缺失，只能依赖后续 sync / 孤儿恢复机制补齐 ⇒ 高丢包率下同步延迟不可预测，可能出现短暂的链视图分歧 |
| **Current behavior** | 非阻塞入队失败 ⇒ `p.queueDrops++` + 发射 `OUTQ_OVERFLOW` 观测事件（含 `queue_cap`）并打印日志；**不重传、不告警给上层、不触发补拉** |
| **Mitigation** | ① 受影响面被限制在**单个慢对端**（不扩散到其他对端、不阻塞调用方）；② `writeTimeout` 与 `maxSendFailures` 最终会断开持续异常的对端；③ 被丢弃的区块可由该对端后续 sync 或对端主动广播补齐 |
| **Future direction** | 候选方向（**均需独立授权**）：溢出后标记"该对端落后"并主动触发定向补拉；按消息类型分级丢弃（区块优先于交易）；队列水位可观测化并暴露到 Console |
| **修复授权** | **必须独立授权。本阶段不修复。** |

---

## §13 安全与健壮性（P2P 层现状）

| 项 | 现状 |
|---|---|
| 消息大小上限 | ✅ 有 |
| 读写/握手超时 | ✅ 有 |
| 连接数限额 | ✅ 有（`maxInbound` / `handshakeQuota` / `maxPeers`） |
| 死连接清理 | ✅ 有（`maxSendFailures`、超时） |
| 广播回环抑制 | ✅ 有 |
| 重复数据去重 | ✅ 有 |
| **传输加密** | ❌ 无（明文 TCP） |
| **对端认证** | ❌ 无 |
| **peer scoring / ban / isolation** | ❌ 无（LTM-002） |
| **P2P 协议版本 / 能力协商** | ❌ 无独立版本字段（LTM-003） |
| 节点发现 | 仅种子 + 握手交换已知节点（无 DHT/DNS seed） |

> 完整安全边界见 `docs/SECURITY-BOUNDARY.md`；完整限制登记见 `docs/LIMITATIONS.md`。

---

## §14 与旧文档的 DRIFT 登记

| 旧声明 | 本文真值 | DRIFT |
|---|---|---|
| 消息类型 5 条 | **7 条**（含 `get_block_by_hash` / `block_by_hash_resp`） | **CONFLICT** |
| "每批 ≤200 块" | 请求侧 `MaxSyncBatch=200` ✅；服务端上限 `MaxBlocksPerResp=500`；另有按哈希回溯上限 64 | **STALE**（不完整） |
| P3 HOL blocking（~30s）仍为 OPEN | **CLOSED / FIXED**（R1-A）；新限制为 LTM-001 | **CONFLICT**（已登记修复） |
| P2P 保护措施仅"大小/超时/死连接/去重" | 另有连接限额、出站队列背压、孤儿补齐 | **STALE** |
