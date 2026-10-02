# PROJECT-AI-CONTEXT.md — P2PChain

> 本文件是**给接手 AI 的项目上下文**。全部内容**仅由本包内的实际源码与仓库既有文档生成**，
> 不补充任何不存在的架构、协议或功能。凡本仓库未实装的能力，一律标注为「未实装 / 明确不做」。
>
> **最高优先级铁律（Owner 明令）：设计状态 ≠ 生产实现状态。**
> 读到「某能力有设计文档 / 有 READINESS 报告 / 有 COMPLIANCE 报告」时，**不等于**它已提交、已收口、已在生产生效。
> 判定基线的唯一顺序是：`git 已提交` → `收口审计阶段复跑全量` → `Owner 显式授权`。见 §17 / §18 / §19。

---

## §1 项目概览

**P2PChain** 是一个用 Go 实现的**确定性、可重放、单二进制**的 Proof-of-Work / UTXO 区块链节点，
定位为「给开发者用的本地区块链节点工具」。

| 项 | 值 |
|---|---|
| Go module | `p2pchain` |
| Go 版本要求 | `go 1.22`（go.mod 声明） |
| 外部依赖 | **零**（刻意只用标准库；`crypto/ecdsa` + `elliptic.P256`） |
| 有无 `go.sum` | **无**（无 require 依赖，故无 go.sum；这是预期状态，不是缺失） |
| 构建 | `go build -o node ./cmd/node` |
| 主要二进制 | `cmd/node`（节点+CLI+控制台）、`cmd/explorer`（只读区块浏览器壳） |
| 辅助客户端 | `gui/` Python(PySide6) 桌面客户端 **P2PChain Studio**（本包仅含源码子集，见 §2 注） |
| 密码学 | ECDSA **P-256**（非 secp256k1）、签名定长 64B（r‖s 各 32B 大端）、地址 = Base58Check(`0x35`‖SHA256(pubkey)[:20]) |
| 网络 | P2P：明文 TCP + 换行分隔 JSON（无 TLS、无对端认证） |
| 控制面 | localhost JSON-HTTP（mutation 端点 Bearer Token 鉴权） |

**核心设计口号（README 原文）**：`Local · Deterministic · Single Binary · Developer-focused · Replayable · Verifiable`

**它的自我定位（README 原文，务必尊重）**：这是一个 **Developer Node** —— 学习/实验项目，**未做安全审计，不得用于任何真实资产场景**。

> ⚠️ **README 与代码存在已知漂移（接手 AI 必读）**：
> `README.md` 的「已知限制」一节仍写着「**分叉处理 / 链重组（reorg）未实现**，收到父哈希不匹配的区块直接拒绝」。
> 这一表述**已经过时**：本仓库已实装 `internal/blocktree`、`blockchain.executeReorg`、`internal/utxo/undo.go`
> 以及多条 reorg / orphan / recovery 阶段。以**源码与阶段报告为准**，README 此处是残留旧文案。
> 同理，`docs/MASTER-DESIGN.md`（早期）写的奖励公式 `50 >> (height/210)` 也已过时，
> 现行公式见 §7（来自 `internal/utxo/apply.go`）。

---

## §2 目录结构

本包保留原始仓库相对路径，未做任何扁平化：

```
p2pchain/
├── cmd/
│   ├── node/            # 节点入口 + CLI + nodeService（P2P Handler 实现）+ 挖矿生命周期 + 孤儿检查点
│   └── explorer/        # 只读 Explorer 独立进程壳（反向代理 control 的 3 个 GET 端点）
├── internal/
│   ├── block/           # 区块结构 / 头哈希 / Merkle / 规范二进制编解码
│   ├── blockchain/      # 链管理、区块校验全序、确定性创世、链身份、只读 verify、reorg 编排
│   ├── blocktree/       # 区块树 / BlockNode / 累积工作量 / fork-choice（ShouldReorg、SetTip）
│   ├── config/          # NodeConfig（当前极简，仅硬编码默认值）
│   ├── control/         # 本地 JSON 控制接口：server + client + 内嵌 Console
│   ├── explorer/        # Explorer 静态 UI（embed）+ 只读路由白名单 + mutation 白名单
│   ├── mempool/         # 交易内存池（组合视图验证 / 重验剔除 / 回池）
│   ├── obs/             # 可观测性事件总线（OBSERVE ≠ CHANGE 铁律）
│   ├── p2p/             # TCP 传输层：握手 / 广播 / 每对端出站队列 / 连接管理
│   ├── pow/             # PoW（双 SHA256）/ 难度调整 / 并行可取消挖矿 / 激活高度规则集
│   ├── storage/         # 区块存储：BlockStore 接口 / 内存版 / blocks.dat 文件版 / v2 帧 / 数据目录独占锁
│   ├── transaction/     # UTXO 交易结构与哈希
│   ├── txbuild/         # 选币 / 找零 / 逐输入签名（解 wallet↔utxo 循环依赖）
│   ├── utxo/            # UTXO 集合、applies、BlockUndo（回滚）
│   └── wallet/          # P-256 密钥、地址(Base58Check)、签名、JSON 持久化(0600)
├── docs/                # 设计文档 + 300+ 份阶段/审计报告（见 MANIFEST 分级）
├── scripts/             # run-tests.sh(canonical 测试契约) / smoke-e2e.sh / git-guard*.sh
├── verifier/            # Python 独立复算验证器（crosscheck / negcheck / 测试向量生成）
├── concept-mvp/         # 概念 MVP（html + 3 份概念/审计报告）
├── gui/                 # ⚠️ 仅源码子集（见下方说明）
├── go.mod
├── .gitignore
├── README.md
├── start-node-a.bat / start-node-b.bat   # 双节点本地启动样例
├── PROJECT-AI-MANIFEST.md
└── PROJECT-AI-CONTEXT.md                 # 本文件
```

> **关于 `gui/`**：原仓库 `gui/` 共 206 MB（含 PyInstaller `build/` 与 `dist/` 产物、Qt 的 `.dll/.pyd/.qm/.pyc`）。
> 本包**只保留纯文本源码**（`*.py / *.spec / *.md`，18 个文件），其余作为**构建产物**整体排除。
> 且此目录在原仓库 `.gitignore` 中属**治理屏蔽路径**（永不入 Git），请把它当作**未入库的辅助客户端**，
> **不要**将其状态误认为主链项目的提交状态。

> **关于 `pkg/`**：本仓库**没有** `pkg/` 目录，全部内部包位于 `internal/`。

---

## §3 核心模块

按 Go 包（含测试）代码行数量化：

| 包 | LOC（含测试） | 职责一句话 |
|---|---:|---|
| `cmd/node` | 14,235 | 节点装配、CLI、`nodeService`（全部 P2P Handler）、挖矿生命周期、孤儿检查点 |
| `internal/blockchain` | 7,603 | 区块校验全序、链状态机、**reorg 编排**、创世、链身份、只读 verify |
| `internal/storage` | 7,603 | blocks.dat 追加日志、**v2 记录帧**（UNDO/BLOCK/TIP/DELETE）、崩溃恢复、datadir 独占锁 |
| `internal/utxo` | 3,173 | UTXO 集合、`ApplyBlockWithUndo`、`DisconnectBlock`（回滚原语） |
| `internal/control` | 2,892 | 本机 JSON 控制接口 + client |
| `internal/p2p` | 1,623 | TCP 传输、握手、每对端出站队列、广播去重 |
| `internal/mempool` | 1,653 | 内存池、共享校验视图、出块剔除、reorg 回池 |
| `internal/pow` | 1,519 | PoW、难度调整、并行/可取消挖矿、规则集版本化 |
| `internal/blocktree` | 1,410 | 内存区块树、累积工作量、fork-choice（纯索引层） |
| `internal/explorer` | 1,004 | Explorer 静态 UI + 只读/受控 mutation 白名单代理 |
| `internal/wallet` | 992 | P-256 密钥/地址/签名/持久化 |
| `internal/obs` | 612 | JSONL 事件总线（有界通道，绝不阻塞业务） |
| `internal/block` | 387 | 区块结构与规范二进制 |
| `internal/transaction` | 326 | 交易结构与 sighash |
| `internal/txbuild` | 293 | 选币/找零/签名 |
| `cmd/explorer` | 57 | Explorer 进程入口 |
| `internal/config` | 22 | 极简 NodeConfig |
| **合计** | **45,404** | （非测试 14,907 + 测试 30,497） |

**关键依赖方向（不成环）**：`cmd/*` → 所有 internal 包；`internal/txbuild` 依赖 `wallet+utxo+transaction`（专门抽出以解环）；`internal/p2p` **不 import** `blockchain/mempool/utxo`（只声明 `Handler` 接口由上层实现）；`internal/explorer` **不 import** `blockchain/storage/wallet`（只经 HTTP 访问 control）；`internal/control` **不依赖** 业务包（只声明 `Node` 接口）。

---

## §4 区块链数据流

```
                         ┌──────────────── 外部世界 ────────────────┐
   网络 P2P ────────────►│ p2p.Node.dispatch → Handler.On*         │
   CLI/RPC ─────────────►│ control.Server → Node 接口实现           │
   本地挖矿 ────────────►│ runMiner → mineOnce → nodeService        │
                         └───────────────┬─────────────────────────┘
                                         │ 统一收敛到 nodeService 的到达面
                                         ▼
        cmd/node.ingestBlock / addBlockAndUpdatePool  (服务 hook)
                                         │
                    ┌────────────────────▼────────────────────┐
                    │ blockchain.AddBlockWithResult           │
                    │  ├─ validateBlock(Validate-First)       │  ← 全序共识校验，见 §6
                    │  ├─ 分支判定：父 == tip ? extendChain    │
                    │  │              : executeReorg          │
                    │  └─ utxo.Set（原子替换）+ storage 持久化  │
                    └───────────┬─────────────────┬───────────┘
                                ▼                 ▼
                     mempool.RemoveIncluded   storage(v2 log)
                     / ReaddDisconnected      UNDO→BLOCK→TIP
```

**观测旁路**：全链路另有 `internal/obs` 的 JSONL 事件（OBSERVE ≠ CHANGE：不持锁、不改返回值、不 panic、可丢弃）。
事件写入由 `P2PCHAIN_OBS`（开关）与 `P2PCHAIN_OBS_FILE`（目标文件）控制。

---

## §5 交易生命周期

1. **构造**：`internal/txbuild.BuildTransaction(set, wallet, to, amount, feePerByte)`
   - 只选**本地址且已成熟**的 UTXO，按金额**降序累计**至 `amount+fee`；
   - 输出 = 收款方 + 找零（找零 > `DustThreshold` 才建输出，否则并入手续费）；
   - 逐输入签名，**sighash = tx.Hash()**（序列化天然排除 Signature/PubKey 字段）。
2. **入池**：`mempool.Add(base, tx, height)`。关键设计是**共享校验视图**：`base.Clone()` + 按插入序依次 Apply 池内交易后再校验新交易
   —— 因此「池内双花」与「链式交易（子花父）」都能判定，且单笔校验只与输入数相关、与池大小无关。
   - 超过容量时按 `admitWithEvictionLocked` 驱逐（含零费率驱逐语义，有专项测试）。
3. **打包**：`mempool.Pending(max)` 按 **fee 降序**返回；受 `MaxBlockTxs=500` 与 1 MiB 区块上限双重约束。
4. **上链**：在 clone 出来的 UTXO 视图上 `ValidateTransaction` → `ApplyBlockWithUndo`（同时产出 `BlockUndo`）。
5. **出块后**：`mempool.RemoveIncluded` 剔除已打包交易，**剩余交易在新 UTXO 上重验**，失效者剔除。
6. **reorg 时**：`mempool.ReaddDisconnected(disconnectBlocks, newBase, height, newChainTxs)` 把被回滚链上的交易重新评估回池。
7. **回滚**：被 disconnect 的区块用 `utxo.DisconnectBlock(undo)` 严格反向恢复。

**sighash 覆盖性论证（源码注释原文要点）**：pubkey 未进 sighash，但 pubkey 必须与前置输出的 `PubKeyHash` 相等才能解锁；
替换 pubkey 必然导致该等式失败且签名无法通过；所有输入引用与输出值都在 sighash 覆盖内。

---

## §6 区块生命周期

`blockchain.validateBlock` 的**校验全序**（顺序本身是共识的一部分）：

1. `PrevHash` 与父块关系（单链追加时须 == tip.Hash；否则走 fork 路径）→ `ErrInvalidPrevHash`
2. PoW（`pow.Validate`，双 SHA256 < target）→ `ErrInvalidPoW`
3. `Bits` == 本链推导的期望值 → `ErrUnexpectedBits`
4. 时间戳：`>= tip.Timestamp` 且 `<= now + 7200s`（maxActivationTimestampSlack），并经 MTP 钳制 → `ErrTimestampOutOfRange`
5. Merkle 重验 → `ErrMerkleMismatch`
6. 体积 ≤ 1 MiB（按**规范二进制**长度计）→ `ErrBlockTooLarge`
7. 交易结构：≥1 笔、恰好 1 笔 coinbase 且位于 index 0 → `ErrFirstTxNotCoinbase / ErrMultipleCoinbase`
8. 状态迁移：在 **clone** 上逐笔 Validate+Apply 累计 fee；coinbase 输出总额 ≤ `Subsidy(height) + fees` → `ErrExcessiveCoinbase`
9. **Apply**：全部成功后整体替换 UTXO 集合（Validate-First → Apply-Second，**无中间状态对外可见**）

挖矿侧另有 `ValidateTemplate`（比 `ValidateBlock` 宽松，用于校验尚未成形的候选模板）。

`AddBlock` / `AddBlockWithResult` → `extendChain`（父即 tip，直接追加）或 `executeReorg`（见 §9）。

---

## §7 共识路径

| 参数 | 现值 | 出处 |
|---|---|---|
| PoW | 双 SHA256 | `internal/pow/Validate` |
| 初始/下限难度（`MaxTargetBits`） | `16` | 创世与 v1/v2/v3 难度下限（floor）；激活前链上可达难度恒为 16 |
| 难度上限（`MaxDifficultyBits`） | `32` | 激活后（height ≥ 2000）难度浮动上界 |
| 难度 v2 激活高度（`ActivationHeight`） | `2000` | 自此启用：难度浮动（Ceil）+ MTP 时间戳 + 区块版本强制 v2 |
| 规则集 v3 激活高度（`NewRulesetActivationHeight`） | `3000` | 自此改用 Nearest 取整 + 版本强制 v3；`h==3000` 一次性注入 `NewRulesetInitialBits=27` |
| 区块版本三态 | `1`(<2000) / `2`([2000,3000)) / `3`(≥3000) | `VersionForHeight`；旧节点对 v2/v3 块确定性拒绝（固定高度硬分叉） |
| 出块间隔目标 | 60 s | 难度调整期望跨度基准 |
| 难度调整周期 | **20 块**（比特币为 2016） | `pow.AdjustBits` |
| 调整限幅 | 单次 ≤ 4 倍 | `pow.AdjustBits` |
| 出块奖励 | `Subsidy(height) = 5 >> (height / 5_250_000)` | `internal/utxo/apply.go` |
| 归零条件 | halvings ≥ 3 即归零；最后一个非零补贴高度 15,749,999，归零高度 15,750,000 | `internal/utxo/apply.go` 注释 |
| Coinbase 成熟期 | 10 块 | README 共识参数表 |
| 区块大小上限 | 1 MiB | `blockchain.MaxBlockSize` |
| 地址版本字节 | 0x35 | `internal/wallet/base58.go` |
| **确定性创世时间戳** | `1700000000` | `internal/blockchain/genesis.go` |
| 创世 coinbase 接收方 | 黑洞地址哈希 `[20]byte{...01}` | `genesis.go` |
| **Canonical Genesis Hash** | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` | `genesis.go` `CanonicalGenesisHash` |

**⚠️ 难度规则集：固定激活高度的硬分叉（必须理解，否则会误判为缺陷）**：
难度**已不再是「钉死在 16」**——按**固定激活高度**分阶段演进（由 `internal/pow` 的 `IsActivationActive` /
`IsNewRulesetActive` / `VersionForHeight` / `ComputeExpectedBitsAt` 锁定，完全由高度决定，与 canonical tip 无关）：

- **ruleset v1（height < 2000，LEGACY）**：难度**钉死**在父块 bits（= `MaxTargetBits = 16`），区块版本必须为 `1`；存量链在激活前逐字节不变。
- **ruleset v2（2000 ≤ height < 3000）**：难度**浮动**，采用 `AdjustBits`（**Ceil** 取整），钳制在 [`16`, `32`]；区块版本强制 `2`；时间戳启用 MTP 规则。
- **ruleset v3（height ≥ 3000）**：改用 `AdjustBitsNearest`（**Nearest** 取整，抑制 Ceil 系统性 +1 overshoot），钳制区间同上；`h == 3000` **一次性注入** `NewRulesetInitialBits = 27`；区块版本强制 `3`。

两套取整共享同一 core，均经「floor clamp(`16`) → 取整 → ceiling clamp(`32`)」，**单次幅度 ≤ 4 倍**（[expected/4, expected×4]）。
**为何现网仍像「不浮动」**：当前生产链高远低于 2000，仍走 v1 钉死 16 路径——这是预期行为；越过高即浮动。
v2/v3 是**结构性硬分叉**（旧节点确定性拒绝 v2/v3 块），改动难度属真值变更，会让老节点拒绝新区块。

**规则集版本化 / 激活**：`internal/pow` 提供 `IsActivationActive(height, activationHeight)`（`height >= activationHeight && height > 0`）、
`IsNewRulesetActive`（`height >= 3000 && height > 0`）、`VersionForHeight`、`ComputeExpectedBitsAt`、`MedianTimePastAt`、`ChainCumulativeWork`（经 `pow.ChainView` 接口），
并配套 `blockchain.validateVersion / validateBits / validateTimestamp` 的 view 版本。`NewBlockchainWithGenesisAndActivation` 可注入激活高度。
**当前冻结值（已提交共识真值，`internal/pow/pow.go` 位于 `git HEAD` 内，非工作树未提交改动）**：
`ActivationHeight = 2000`、`NewRulesetActivationHeight = 3000`、`NewRulesetInitialBits = 27`、`MaxDifficultyBits = 32`、
`NewBlockVersion = 2`、`NewRulesetBlockVersion = 3`、`MaxTargetBits = 16`、`TargetBlockTimeSeconds = 60`、`DifficultyAdjustmentInterval = 20`。

---

## §8 P2P 网络路径

传输：**TCP 长连接 + 每条消息一行 JSON**，信封为 `Message{Type, Payload}`。
区块/交易在 Payload 内以**规范二进制的十六进制串**传输（与磁盘/共识层同一字节表示，避免结构演进导致不一致）。

| 消息类型 | 方向 | 说明 |
|---|---|---|
| `handshake` | 双向 | 交换节点 ID / 服务高度 / **genesis 哈希** / 链尾 / 已知节点列表 |
| `new_block` | 广播 | 完整区块；接收方全量校验后上链并中继（排除来源对端） |
| `new_tx` | 广播 | 完整交易；入池后中继 |
| `get_blocks` | 请求→响应 | 批量追赶：`get_blocks{fromHeight}` → `blocks_resp`（分批，≤ MaxSyncBatch=200 / MaxBlocksPerResp=500） |
| `get_block_by_hash` | 请求→响应 | **分支补齐**：按哈希取该块及其至多 `MaxAncestorsPerResp=64` 个祖先 |
| `block_by_hash_resp` | 响应 | 祖先链；逐块走既有 `ingestBlock` 全量共识校验 |

边界常量：`MaxMessageSize = 1 MiB`、`MaxBlocksPerResp = 500`、`MaxAncestorsPerResp = 64`、
`MaxSyncBatch = 200`（cmd 层）、读写超时、发送失败阈值后断链清理、`BroadcastExcept(msg, except)` 防中继回环。

**每对端有界出站队列（R1-A）**：`Broadcast` 只做非阻塞入队，慢对端只能溢出自己的队列（丢帧），
**绝不阻塞调用方**（尤其不阻塞挖矿临界区）与其他健康对端。

**同步活性 registry（PHASE P2P-SYNC-LIVENESS-MINIMUM-SAFE-FIX-1）**：批量同步在途登记表取代了原来的「全局单计数器」，
每个区间起点条目带归属 peer / 尝试次数 / deadline / backoff，常量见 §10。

**seed 重连**：`watchSeeds` 每 5 s 补齐不足的种子连接。

---

## §9 fork / reorg

**层次分工（务必分清，历史上多次被混淆）**：

| 层 | 包 | 职责 |
|---|---|---|
| **索引层** | `internal/blocktree` | 内存区块树、`BlockNode`、**累积工作量**、`FindCommonAncestor`、`CompareWork`、`ShouldReorg`、`SetTip`（**只移动指针，绝不触碰 UTXO / mempool / 持久化**） |
| **回滚原语层** | `internal/utxo/undo.go` | `BlockUndo` 数据模型 + `ApplyBlockWithUndo` + `DisconnectBlock`（严格反向） |
| **存储层** | `internal/storage` v2 | hash 寻址、detached 区、UNDO 持久化、逻辑删除、崩溃恢复 |
| **共识编排层** | `internal/blockchain.executeReorg` | 真正的 chain switch：disconnect → apply → persist |

**fork-choice（§E 契约，纯只读比较，不切换）**：

```
if   candidate.CumulativeWork > active.CumulativeWork:  candidate wins
elif candidate.CumulativeWork < active.CumulativeWork:  active wins
else:  确定性 tie-break：tip hash 大端较大者胜
```

**铁律：网络到达顺序绝不决定 consensus chain。**
`Work(bits) = 2^bits`（「严格工作量」），累积量用 `*big.Int`。

**MaxReorgDepth（BG-3 已定稿）**：`blocktree.SetTip` **永不**因深度拒绝更高 work 链 ——「告警 + 限速，永不拒绝」。
深度策略属 REORG-1I，**未实现、不检查**。

`blockchain.executeReorg(newTip, persist)` 流程（概括自源码）：共同祖先 → 依次 `DisconnectBlock`(用存储里的 UNDO) 回滚旧分支 → 依次 validate+apply 新分支 → 更新 UTXO / blocktree.tip → 按 `persist` 提交到存储 → mempool 回池。

---

## §10 orphan / recovery

孤儿与恢复是本项目**近年审计密度最高**的区域，请优先阅读 §17–§19 的状态说明。

**内存孤儿队列（cmd/node/nodeService）**：

| 常量 | 值 | 语义 |
|---|---:|---|
| `maxWaitingBlocks` | 256 | **不同父哈希**的键数上限 |
| `maxWaitingChildrenPerParent` | 64 | 每父哈希下子块数上限（R-1 修复，此前无界）→ 总条目上界 256×64 = 16,384 |
| `maxInflightBranch` | 64 | 在途 by-hash 请求上限 |
| `branchReqTTL` | 30 s | 同一哈希请求去重窗口 |
| `maxBranchRounds` | 8 | 同一孤儿链回溯轮次上限 |
| `MaxBranchAncestors` | 64 | 单次请求期望回溯祖先数（服务端再按 `MaxAncestorsPerResp` 裁剪） |
| `maxInflightSync` | 4 | 批量同步在途区间数上限 |
| `defaultSyncReqTTL` | 30 s | 批量同步单次尝试窗口 |
| `maxSyncAttempts` | 3 | 总尝试次数（**有终态**） |
| `defaultSyncRetryBase` | 2 s | 指数退避基数 |
| `defaultSyncSweepInterval` | 2 s | 唯一执行重试的巡检 goroutine 周期 |

**孤儿入队前置守卫（不再无条件入队）**：`preParkRejectReason` 在入队前做**父无关**的廉价校验，顺序为
`size → bits 共识域 → merkle → pow`，任一失败即拒绝（不占用孤儿保留资源）。
重复到达由 `parkedHashes` 去重（先廉价快查，再与插入同临界区权威复查）。

**ORPHAN-DURABILITY-IMPLEMENTATION-1（最近的一条主线）**，三段式：

- **§4-B1 数据层**：`cmd/node/orphan_checkpoint.go` —— 文件 `<datadir>/orphan_waiting.bin`
  格式：magic `"ORPH"`(4B) + version(1B) + entryCount(u32 BE) + checksum(SHA-256 32B)；
  entry = parentHash(32B) + childCount(u32 BE) + childHash×N。
  上界 entryCount ≤ 256、每 entry childCount ≤ 64，文件 ≤ ~524 KB。原子写 = temp + fsync + rename。
  **失败语义 fail-closed**：任何失败只影响孤儿可用性，**绝不反向影响 canonical**。
- **§4-B1.5**：把 §4-B1 在**生产中真正激活**——`cmd/node/main.go` 的 D1（`newOrphanCheckpoint(cfg.DataDir)` → `CleanupTmp()` → `svc.orphanCP = orphanCP` → `prepareOrphanRestore(orphanCP)`，**在 `p2pNode.Start()` 之前**）+ D2（`runSyncSweep` 经 `orphanCP.FlushIfDirty(5s)` 周期落盘）。**已实现并验证**（含 §4-B1.5 合规报告与全量测试），仍处**未提交工作树**（见 §17）。
- **§4-B2**：启动恢复接线（**已实现 + E2E 验证**）—— `OnHandshake` → `consumeRestorePending(peerAddr)` → 复用 `requestBranch` 重拉缺失分支。
  语义：父已知 → 双删 drain（`restorePending` + checkpoint，`Remove` 幂等，且**仅在父已知时**执行，绝不误删 live waiting）；
  父未知 → `requestBranch` 并**保留**在集合中（隐式有界重试，由 inflight/TTL/rounds 压制）。
  不变量：不伪造块指针、不旁路孤儿准入、不腐蚀 `s.waiting`、`restorePending` 与 `s.waiting` 是两个独立 map。
  端到端证据：`docs/PHASE-P2PCHAIN-ORPHAN-DURABILITY-E2E-RECOVERY-1-REPORT.md`（真实网络/磁盘/进程边界全链路验证，14 项 durability+recovery 断言全绿）。

---

## §11 storage / database

**无数据库依赖，无 migration 目录。** 存储 = 单个追加日志文件 `<datadir>/blocks.dat`。

**语义分区（REORG-1E · M2 冻结）**：

- **legacy 区**：日志最前部连续的 `[u32 LE len][block.Encode()]`，高度 = 记录序号。被视为
  **committed canonical historical prefix**，**只读不改、永不迁移/重写/截断**。
- **v2 区**：自首条 v2 帧起。canonical 状态**只能**由 TIP 记录推进；区块以哈希寻址、可 detached、可逻辑删除。

**v2 记录帧（frame.go，总开销固定 80 B）**：

| 偏移 | 长度 | 字段 |
|---:|---:|---|
| 0 | 4 | magic `'P','C','C','2'` |
| 4 | 1 | version (=2) |
| 5 | 1 | type：1=BLOCK 2=UNDO 3=TIP 4=DELETE |
| 6 | 2 | reserved（必须 0，LE） |
| 8 | 4 | payloadLen（LE） |
| 12 | 4 | height（LE） |
| 16 | 32 | blockHash（双 SHA-256） |
| 48 | n | payload |
| 48+n | 32 | checksum = SHA256(头‖payload) |

**零迁移兼容的结构性保证**：magic `'PCC2'` 按小端 u32 读作 **843,268,944**，**大于** legacy 记录长度上限 64 MiB，
因此 legacy 读者遇到 v2 帧必然以「非法记录长度」fail-stop —— v2 帧不可能被旧读者误当合法记录。

**单一提交点（M5/F4）**：`UNDO → BLOCK → TIP`，**TIP 是唯一 canonical commit point**。
所有解析路径 **fail-stop**：任何字段非法立即返回具名错误，绝不跳过/猜测/重新同步。

**API 分工（Option A 决议）**：
`SaveBlock`（legacy 语义，逐字节不变；一旦进入 v2 模式即被拒绝）/
`SaveBlockDetached` / `SaveBlockWithUndo` / `PutUndo`（写 v2，**不提交** canonical）/
`CommitTip`（唯一提交点）/ `AppendCanonicalBlock`（UNDO→BLOCK→TIP，**一次 fsync**）/
`DeleteBlock` / `DeleteBranch` / `TruncateFromHeight`（**仅逻辑删除，绝不物理删除已提交字节**）。

**崩溃恢复（M7）**：只读路径**严格拒绝**任何损坏；可写路径**仅当**尾部是「位于 EOF 的截断残片」且能证明其未被提交时才 REPAIR（物理截断残留字节）；
完整但语义失效的 TIP 走**逻辑 ROLLBACK**（不删字节）。

**索引与 UTXO 永不落盘**：打开时顺序扫描重建；UTXO 从创世回放重建。

**数据目录独占锁（PHASE 2.1 / P3.1）**：`<datadir>/node.lock` 经 `O_CREATE|O_EXCL|O_WRONLY` **原子独占**创建（绝不覆盖/截断既有锁），
内容仅 `pid + started_at`；`newNodeRuntime` **第一步**取锁（早于打开 blocks.dat）；`Close()` 经 `sync.Once` 释放。
**§5 严格禁止**基于 PID 的 stale-lock 自动删除。

---

## §12 RPC / API

**节点控制接口（`internal/control`，默认 `127.0.0.1:6689`）**：

| 端点 | 方法 | 鉴权 | 说明 |
|---|---|---|---|
| `/status` | GET | 无 | 高度/链尾/对等/池/挖矿中 |
| `/balance?address=` | GET | 无 | 可花费 + 含未成熟两个口径 |
| `/utxos?address=` | GET | 无 | UTXO 列表 |
| `/block?height=N` `/block?hash=` / `/blocks` | GET | 无 | 区块（`/blocks` 分页 `MaxBlocksPerPage=100`） |
| `/logs?tail=` | GET | 无 | 最近日志（`MaxLogTail=500`） |
| `/console` | GET | 无 | 内嵌 Console 页面 |
| `/send` | POST | **Bearer** | 用节点钱包转账 |
| `/mine` `/mine/start` `/mine/stop` | POST | **Bearer** | 按需出块（并行会有 `MineConflictError`） |
| `/stop` | POST | **Bearer** | 远程停止（未启用时 `ErrStopUnsupported`） |

鉴权（`PHASE CONTROL-AUTH-1`）：`requireAuth` 中间件，**未配置 token 时 fail-closed（一律 401）**，
失败统一带固定延迟。**本质边界是回环绑定** —— 切勿暴露到不可信网络。

**Explorer（`cmd/explorer` + `internal/explorer`）**：`浏览器 → Explorer(127.0.0.1:9091) → control(127.0.0.1:17881) → Node`。
严格`"explorer is a shell"`：只做 (1) embed 的静态 UI，(2) **显式只读路由白名单**转发三个 GET 端点。
**不提供通用 `/api/*` 反向代理**；未命中白名单的请求一律在 Explorer 本地拒绝（upstreamHits == 0）。
mutation 白名单仅 `POST /api/mine/start`、`/api/mine/stop`；token 只存在于 Explorer 进程内，**浏览器零接触**。

**CLI（`cmd/node/cli.go`）**：`status / balance / utxos / send / mine / stop / init / wallet / printchain / verify / reset`（另有上层入口 `ui` / `node`）。

**离线只读校验 `verify`**：以只读方式打开 `blocks.dat`，逐块重跑**与启动时完全相同**的共识校验，
输出带**失败高度 / 哈希 / 具体原因**（绝不是泛化的 "invalid"），退出码 `0=通过 / 1=不通过或无法执行 / 2=参数错误`。
保证（有回归测试）：不改 blocks.dat（SHA-256 与大小不变）、**不获取/创建/删除锁（因此可在节点运行时执行）**、不加载钱包、不起 P2P。
机器可读输出：`verify -json`。

---

## §13 wallet

- 曲线 **P-256**（`crypto/ecdsa` + `elliptic`）；**非** secp256k1，`internal/wallet/wallet.go` 注释给出升级路径。
- 地址：`Base58Check(0x35 ‖ SHA256(pubkey)[:20])`。用 SHA256 截断代替 RIPEMD160（stdlib 无 RIPEMD160）。
- **签名必须定长 64 字节**（r‖s 各 32 B 大端）—— 这是**已修复的历史缺陷**：
  `big.Int.Bytes()` 在最高位为 0 时会短于 32 B，若按「变长拼接 + 中间切分」验签则切分错位，
  表现为约 **1/128 概率的偶发验签失败**。现由 `SignatureSize` + `ErrBadSignatureLength` 锁定。
- 持久化：`<datadir>/wallet.json`，字段 `{version, d, x, y, pubkey}`，权限 **0600**。
  **源码注释明确：这是学习用途的明文存储，生产必须用 argon2/scrypt 派生密钥加密后再落盘**。
- ⚠️本审计包**不含任何 wallet.json 实例**（含私钥 `d` 的文件已在打包阶段按运行时数据排除）。

---

## §14 genesis / config

- **无 `config/` 目录、无 config.example、无 migration/schema 目录、无 Makefile / Dockerfile / docker-compose / CI 配置。**
  这是仓库的真实状态 —— 不要假设它们存在。
- 配置现状：`internal/config` 仅一个极简 `NodeConfig{...}` + `DefaultConfig()`（22 行）。
  实际运行参数全部是 `cmd/node` 的命令行 flag：`-listen -rpc -seed -datadir -mine -maxblocks -miners`。
- **genesis 不是配置文件驱动的**：由 `internal/blockchain.NewGenesisBlock()` 代码生成
  （固定时间戳 `1700000000` + 固定 coinbase 到黑洞地址 + MaxTargetBits 串行挖出），从而保证全网字节级一致。
- **链身份（$PHASE F2/F3A 冻结的产物）**：`CanonicalGenesisHash` 常量 = 当前协议身份锚点，
  `VerifyGenesisIdentity(store)` 是**所有持久化建链路径的唯一 canonical 身份闸门**（不做共识回放、不写存储）。
  改造任何影响 canonical genesis 的东西，**必须先过兼容性审计与协议身份决策**，不能静默改值。
- 启动样例：`start-node-a.bat` / `start-node-b.bat`（Windows 双节点本地起链）。

---

## §15 测试体系

| 维度 | 数量 | 说明 |
|---|---:|---|
| 测试文件（`*_test.go`） | **93** | 占总 Go 文件 143 的 65% |
| `Test*/Benchmark*` 函数 | **615** | 全仓 |
| 测试代码行数 | 30,497 | 非测试代码 14,907 |

分布（顶层用例数）：`cmd/node` 187、`internal/blockchain` 101、`internal/storage` 68、`internal/control` 54、
`internal/utxo` 33、`internal/blocktree` 26、`internal/wallet` 25、`internal/mempool` 24、`internal/explorer` 19、
`internal/p2p` 13、`internal/obs` 11、`internal/block` 8、`internal/transaction` 5、`internal/txbuild` 5、`internal/pow` 36。

**⚠️ 必须用 canonical 入口跑（否则会遇到假失败）**：

```bash
bash scripts/run-tests.sh        # == go test ./... -count=1 -timeout 25m
bash scripts/smoke-e2e.sh        # 双真实节点进程端到端（15 项）
```

原因（`docs/TEST-EXECUTION-CONTRACT.md`）：`go test` 的**每包默认超时是 10m**，
而 `cmd/node` 全量实测 **583.5–605.3 s**（真实 `go build` 子进程 + 真实 node 子进程 + 真实 RPC 挖矿 + 真实 TCP P2P），
默认值会以 `panic: test timed out after 10m0s` 造成**无任何断言失败的假失败**（已实测复现）。
`-count=1` 是必须的：否则测试缓存会把包报成 `(cached)` 而并未真正执行，回归证据失真。

已覆盖族（节选）：UTXO 状态机全错误路径、签名定长编码回归、难度调整边界、创世确定性、重启持久化、
真实 TCP 传播/同步/拒绝无效块、种子重连、并行挖矿逐块共识校验、控制接口错误映射、
reorg/fork-choice/undo（`f1n1_*` `f1n2_*` `r4b_*`）、storage 崩溃矩阵（`r3_crash_matrix`）、
孤儿耐久（`orphan_*`）、§4-B2 恢复（`orphan_recovery_4b2_test.go`，7 项）。

**独立复算验证器（`verifier/`，Python）**：`p2pchain_verify.py` / `crosscheck.py` / `negcheck.py` / `gen_test_vectors.py`
+ `TEST-VECTORS.md` + `test_p2pchain_verify.py` —— 允许第三方**不读 Go 源码**独立复算哈希/序列化/创世。

**确定性序列化规范**：`docs/DETERMINISTIC-SERIALIZATION-SPEC.md`（第三方可实现解析/复算/创世重建）+ orphan checkpoint 格式规约。

---

## §16 当前已知问题

1. **README「已知限制」与代码漂移**：已于 **CONSOLIDATION-2** 在本工作树中修复——README 现声明 reorg / 难度浮动 / 孤儿持久化 / 启动恢复 为已实现（并附未提交状态同步说明）。**修复仍处未提交状态**（见 §17），以源码与阶段报告为准。
2. **被引用但缺失的 spec 文档**：已于 **CONSOLIDATION-2** 创建 `docs/spec/ORPHAN-DURABILITY-SPEC-v1.md` 与 `docs/spec/STARTUP-RECOVERY-PLAN-1.md`（仅记录已实现行为，无新设计）。
   `cmd/node/orphan_checkpoint.go` 顶部注释原引用名 `ORPHAN-DURABILITY-IMPLEMENTATION-SPEC-v1` 已同步更正为 `ORPHAN-DURABILITY-SPEC-v1`（工作树未提交）。
3. **P2P 出站队列满即静默丢帧**、**无重传**（LTM-001，`docs/LIMITATIONS.md` 登记）。
4. **无 peer scoring / ban / isolation**（LTM-002），**无独立 P2P 协议版本/能力协商字段**（LTM-003）。
5. **完整 orphan pool 与重播策略明确未做**（REORG-1G/B5）：孤儿**仅限内存 + checkpoint 投影**，
   不做定时重播/评分/封禁，深度也很浅（见 §10 各项上限）。
6. **安全边界骨架未完成**：`docs/SECURITY-BOUNDARY.md` 与 `docs/LIMITATIONS.md` 均自述 **SKELETON / 待补全**，
   完整威胁模型、攻击矩阵、缓解清单 **尚未产出**。
   P2P 明文无 TLS、无对端认证；控制面无 TLS、只读端点无鉴权（依赖回环绑定）。
7. **钱包明文落盘**（学习用途，见 §13）。
8. **难度固定**（见 §7，属**有意设计**，不要当缺陷修）。
9. **§4-B2 残余风险 R1–R4**（见 §17/§19 引用的合规报告）：依赖对端持有所缺分支、崩溃窗口内新孤儿可能未落盘、
   握手遍历开销、与既有的追赶/分支发现并存。均判为 P2–P3，**无阻断性风险**。

---

## §17 当前审计阶段

**主线：`ORPHAN-DURABILITY-IMPLEMENTATION-1`**（孤儿持久性与启动恢复）。

```
REORG-1A → 1B → 1C → 1D → 1E → 1F → 1G → 1H → 1J        （reorg 基础设施主线）
   │        │     │     │     │
   │        │     │     │     └─ v2 记录帧 / UNDO / 逻辑删除 / 崩溃恢复
   │        │     │     └─────── UTXO BlockUndo / DisconnectBlock
   │        │     └───────────── 共识级 reorg 编排（executeReorg）
   │        └─────────────────── 树级 tip 基础设施 / fork-choice
   └──────────────────────────── BlockTree / BlockNode / 累积工作量

F-1 → F-2 → F-3 → F-4 → F-5 → F6 / F6.1 … F6.8 → CB1 → CB2   （创世一致性与治理收口）

→ ORPHAN-DURABILITY-IMPLEMENTATION-1:  §4-B1 → §4-B1.5 → §4-B2   （当前位置）
```

最近三份相关产物（均在本包内）：

| 日期 | 文档 | 核心结论 |
|---|---|---|
| 2026-10-01 | `PHASE-ORPHAN-DURABILITY-4B-READINESS-AUDIT.md` | §4-B1 数据层完成且自洽；**§4-B1 生产激活未完成**（2 个 blocker：D1 未给 `orphanCP` 赋值 → 持久层恒为 no-op；D2 无 `Flush` 调用方 → 文件永不落盘）；§4-B2 **设计就绪、尚未实施**，需 Owner 授权 |
| 2026-10-02 | `docs/PHASE-P2PCHAIN-ORPHAN-DURABILITY-4B15-COMPLIANCE-REPORT.md` | §4-B1.5 实现完成、21 项测试全绿、vet 干净、**⛔ 未提交**；当时标注「§4-B2 仍未授权」 |
| 2026-10-02 | `docs/PHASE-P2PCHAIN-ORPHAN-DURABILITY-4B2-COMPLIANCE-REPORT.md` | §4-B2 实现完成、7/7 专项 PASS、相关回归族全绿、gofmt/vet/build 干净、**⛔ 未提交 / 未部署 / 未做生产变更** |
| 2026-10-02 | `docs/PHASE-P2PCHAIN-ORPHAN-DURABILITY-E2E-RECOVERY-1-REPORT.md` | ORPHAN-DURABILITY-E2E-RECOVERY-1 完成：真实网络/磁盘/进程边界全链路验证，14 项 durability+recovery 断言全绿、0 失败；残余风险（孤儿块体非持久、drain 依赖下次握手、in-process 重启未走 DirLock、sync/branch 并发未隔离）已登记，**⛔ 未提交 / 未部署 / 未做生产变更** |

### 🔴 关键状态区分（接手 AI 必须先看这一段）

- **已提交基线（`git HEAD`）**：**不含** §4-B1 / §4-B1.5 / §4-B2 的任何代码 —— 三者目前全部是**工作树未提交**状态。
- **当前工作树**：**同时包含** §4-B1 + §4-B1.5 + §4-B2 的实现代码与测试。
- **本次打包时实测（2026-10-02）**：
  - `go build ./...` ✅ 通过
  - `go vet ./...` ✅ 干净
  - `go test ./... -count=1 -timeout 25m` ✅ **全量通过**（`cmd/node` 587.313s、`cmd/explorer` 无测试；
    `internal` 15 包全 ok，其中 `internal/storage` 347.5s、`internal/blockchain` 97.7s、`internal/mempool` 41.0s、`internal/p2p` 15.3s）
- **Owner 铁律（原话精神的强制表述）**：
  > **§4-B1 / §4-B1.5 的数据层与生命周期已实现；§4-B2 的 startup recovery wiring 已在未提交工作树中写完；
  > ORPHAN-DURABILITY-E2E-RECOVERY-1 已在未提交工作树中以真实网络/磁盘/进程边界全链路验证（14 项断言全绿）。
  > 但以上全部仍处于「实现完成 + 验证完成但未提交、未收口、未部署」状态。
  > 不得把设计状态、READINESS/COMPLIANCE/E2E 报告结论、或「未提交的工作树实现」误认为生产实现状态。**
- 因此正确的默认口径是：**按「未实装」对待**，除非同时满足：① 已 `git commit`；② 对应收口审计阶段已复跑全量；③ Owner 显式授权。
  （注：上述实测全绿只能证明「当前工作树可编译、可通过全量测试」，**不能**替代 ①②③。）

---

## §18 当前禁止修改区域

以下为各阶段报告反复重申的 **FORBIDDEN / HARD STOP** 边界（改动前必须先取得 Owner 授权）：

1. **共识真值**：`internal/blockchain` 的区块校验全序、`Subsidy`、`CoinbaseMaturity`、`MaxBlockSize`、
   `CanonicalGenesisHash`、`GenesisTimestamp`、**难度 clamp 带宽与 `MaxDifficultyBits`** —— 任何改动都会让老节点拒绝新区块。
2. **`internal/pow` 的 `IsBitsInConsensusDomain` / 规则集版本化 / 激活高度语义**。
3. **存储字节格式**：legacy `[u32 LE len][block.Encode()]` 与 v2 帧布局（magic/version/type/字段宽度/checksum）—— 一旦写出的字节格式变化即不可逆。
4. **P2P 协议面**：`MessageType` 字符串、`HandshakePayload` 字段、RPC schema —— 无版本协商能力，改动即断裂兼容。
5. ** legacy 区不可变原则**：M2 冻结，任何字节都不被迁移/重写/截断。
6. **`s.waiting` / `parkedHashes` 数据结构**与 `deferOrphan` / `takeWaiting` 的既有语义（ORPHAN-DURABILITY 各阶段保持零改动，只加外围）。
7. **治理屏蔽路径**：`/.workbuddy/`、`/gui/`、`/audit-run/`、`/f5-verify/`、`/gui-test/`、以及含明文私钥的 `/run-a/`、`/run-b/` —— MUST NOT TRACK，**永不入 Git**（见 `.gitignore`）。
8. **不在默认 Range 内的能力（明确 DEFER）**：readTimeout/heartbeat/ping-pong/TCP keepalive/协议消息类型/握手能力声明/NodeID/known_peers/maxInbound/BroadcastExcept/connection layer 重构。
9. **禁止引入区块链以外的功能**（MASTER-DESIGN 纪律：`标准库零依赖`）。

---

## §19 当前未完成工作

| # | 事项 | 状态 |
|---|---|---|
| 1 | **§4-B2 提交入库 + 收口审计复跑全量**（含 `cmd/node` 全量套件，非仅相关回归族） | ⛔ **未完成**。现有证据只覆盖 `-run 'Test4B2\|TestB1T\|TestBranch\|TestMSF\|TestNodeService\|TestOrphan\|TestR1F\|TestR4B\|TestRealProcess\|TestShouldSyncFrom'` 子集 |
| 2 | §4-B1 / §4-B1.5 / §4-B2 三者的 **git commit** | ⛔ **未完成**（全部工作树态） |
| 3 | 补齐两份缺失 spec：`docs/spec/ORPHAN-DURABILITY-SPEC-v1.md`、`docs/spec/STARTUP-RECOVERY-PLAN-1.md` | ✅ **已完成（CONSOLIDATION-2 创建，未提交）** |
| 4 | **完整 orphan pool（REORG-1G/B5）**：定时重播 / 评分 / 封禁 / 更深内存池 | 📌 明确未做（DEFER） |
| 5 | **MaxReorgDepth 深度策略（REORG-1I）** | 📌 未实现、不检查（BG-3） |
| 6 | P2P 传输安全：TLS / 对端认证 / peer scoring / ban / isolation / 协议版本协商 | 📌 明确未做 |
| 7 | `docs/SECURITY-BOUNDARY.md` / `docs/LIMITATIONS.md` 补全为正式文档 | 📌 SKELETON |
| 8 | 完整威胁模型与攻击矩阵 | 📌 未产出 |
| 9 | secp256k1 / RIPEMD160 迁移（现为 P-256 + SHA256 截断） | 📌 仅注释留有升级路径 |
| 10 | 钱包私钥加密落盘（现为明文 0600 JSON） | 📌 未做 |
| 11 | 难度浮动放开（属独立共识参数阶段） | 📌 有意不放开 |
| 12 | SPV / 轻节点 / 代币经济模型 | 📌 明确不做 |
| 13 | 修复 README 与代码的漂移（reorg 已实现、难度规则集过期） | ✅ **已修（CONSOLIDATION-2 工作树编辑 README + PROJECT-AI-CONTEXT，未提交）** |

---

## §20 推荐 AI 阅读顺序

> 目标：**先建立「什么是真」的判断基准，再看实现细节。** 顺序刻意把「状态认定」放在最前。

**第 0 步 —— 先读状态，避免把设计当实现**
1. `PROJECT-AI-CONTEXT.md`（本文件）§17 的关键状态区分 + §18 禁改区 + §19 未完成清单
2. `PROJECT-AI-MANIFEST.md`（结构、统计、排除了什么、脱敏了什么）

**第 1 步 —— 建立产品与环境的真实边界**
3. `README.md`（能力清单、共识参数表、CLI、P2P 消息表。**注意与代码的漂移**）
4. `go.mod`（确认零依赖 —— 这决定了你能引入什么）

**第 2 步 —— 权威规格（SPEC 层，冲突时以此为准）**
5. `docs/CANONICAL-CONSENSUS-SPEC.md`
6. `docs/CANONICAL-P2P-SPEC.md`
7. `docs/CANONICAL-RPC-SPEC.md`
8. `docs/DETERMINISTIC-SERIALIZATION-SPEC.md`（字节级格式；第三方复算的唯一入口）
9. `docs/MASTER-DESIGN.md`（设计决策记录，**注意早期部分已过时**）→ `docs/README.md`（文档索引与权威分级）
10. `docs/PROJECT-COMPLETION-REPORT.md`（全量验收证据 + 未决项分级）

**第 3 步 —— 读代码：按数据流而不是按字母序**
11. `internal/block/block.go` + `codec.go` → `internal/transaction/*` → `internal/utxo/set.go`（数据结构底座）
12. `internal/utxo/apply.go` + `undo.go`（状态迁移与回滚原语）
13. `internal/pow/pow.go`（PoW + 难度 + 规则集版本化）
14. `internal/blockchain/blockchain.go` → `consensus.go` → `genesis.go` → `identity.go` → `verify.go`（共识心脏）
15. `internal/blocktree/blocktree.go` + `settip.go`（fork-choice 的**纯索引层**原语）
16. `internal/storage/storage.go` → `file.go` → `frame.go` → `v2.go` → `v2api.go` → `datalock*.go`（字节格式与崩溃语义）
17. `internal/mempool/mempool.go`（共享校验视图是本项目最绕的一处）
18. `internal/wallet/*` + `internal/txbuild/*`（密钥与交易构造）
19. `internal/p2p/node.go`（传输层；注意它不 import 任何业务包）
20. `cmd/node/service.go` ← **重点**：全部到达面、`syncInflight` registry、孤儿队列、`requestBranch`、§4-B2 `consumeRestorePending`
21. `cmd/node/main.go`（生命周期：取锁 → 开存储 → 建链 → 钱包 → 池 → svc → P2P → control → scheduler → 连种子）
22. `cmd/node/orphan_checkpoint.go`（孤儿持久层，当前主线）
23. `internal/control/*` → `internal/explorer/*` → `cmd/explorer/*`（对外接口与白名单边界）
24. `internal/obs/*`（观测旁路；读它才能真正读懂日志与事件名）

**第 4 步 —— 如果你被要求做安全/共识审计，按此顺序回 correspond 包**
25. 审计 **consensus**：重读 14 → 13 → 12，并对照 `internal/blockchain/*_test.go` 的 `f1n1_*` `f1n2_*` `p1_*` `template_validation`
26. 审计 **fork/reorg**：重读 15 → 16(`v2`) → 14(`executeReorg`) → `internal/blockchain/f1n1_reorg_coverage_test.go` / `mnc_o3_fork_chainview_test.go`
27. 审计 **orphan/recovery**：重读 20(`deferOrphan`/`takeWaiting`/`requestBranch`) → 22 → `cmd/node/orphan_*_test.go`
28. 审计 **durability**：重读 16 全部，重点 `internal/storage/r3_crash_matrix_test.go` / `r4a_legacy_length_matrix_test.go` / `f1n1_undo_contract_test.go`
29. 审计 **storage/protocol 字节兼容性**：`docs/DETERMINISTIC-SERIALIZATION-SPEC.md` ↔ `internal/storage/frame.go` ↔ `verifier/`（独立复算）

**第 5 步 —— 动手前**
30. 跑 `bash scripts/run-tests.sh`（**必须**，不要用裸 `go test ./...`，见 §15 的假失败陷阱）
31. 改动前请回头重读 **§18 禁止修改区域**，尤其是「共识真值」与「存储字节格式」两类不可逆改动。
