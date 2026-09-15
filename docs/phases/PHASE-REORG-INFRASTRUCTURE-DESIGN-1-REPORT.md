# PHASE REORG-INFRASTRUCTURE-DESIGN-1

## REORGANIZATION / CHAIN TREE / CUMULATIVE WORK CONSENSUS ARCHITECTURE AUDIT

> 阶段性质：**严格 READ-ONLY / CONSENSUS ARCHITECTURE DESIGN AUDIT**。
> 本阶段只做代码阅读、现有测试阅读、数学/状态机推导与只读环境观察。
> **未修改任何 `.go` / 测试 / `.md` / 配置 / 共识参数 / 生产 datadir；未重启或 kill 生产节点；未做任何 git 写操作。**
> 本文件是 **Implementation Contract（实现契约）**，不是实现。

---

## A. HARD BASELINE

| 项 | 值 |
|---|---|
| HEAD | `674c5ad076ed4857a158d6ecd7c73d58e1a61553`（`fix: handle signals during mining`） |
| HEAD~1 | `91bfd79dd8fbb32cea6e6d1920b110c79105cc20` |
| branch | `main` |
| `git status --porcelain` | ` M docs/DETERMINISTIC-SERIALIZATION-SPEC.md` · ` M docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` · 13 个 untracked `docs/PHASE-*.md` + `verifier/` |
| `.go` 漂移 | **无**（全部 untracked / modified 均为 docs，无源码变更）→ **漂移检查 PASSED** |
| Go version | 1.22.12（`~/.workbuddy/binaries/go/go/bin`） |
| 与 Phase F / Phase G 关系 | 代码与 Phase F（`PHASE-FORK-CHOICE-DESIGN-1`）、Phase G（`PHASE-POW-WORK-DIFFICULTY-CHAINWORK-SECURITY-AUDIT-1`）基线**逐字节一致**；本次重读 `blockchain.go` / `utxo` / `storage` / `mempool` / `p2p` / `cmd/node/service.go` 确认结论无变化 |
| 生产节点 | `.123` PID **70434**，height=1275，STALLED（accepted 15 / rejected 0），**全程未触碰** |

**基线结论**：Phase H 在已确认的 Phase F（reorg NOT READY，9 项 P1 缺口 FC-001…FC-008）与 Phase G（`PASS WITH DESIGN GAPS — READY FOR REORG-INFRASTRUCTURE-DESIGN-1`，强制前置 G2=解钳前须 `DIFFICULTY-CONSENSUS-DESIGN-1`）之上推进。本阶段目标是把「R1–R9 + L0–L4 + D1–D11」冻结为**可拆 WBS 的设计定稿**。

---

## B. R1–R9 GAP MATRIX（结合当前最新代码重新审计）

> 证据列均指向本次实际重读的代码行。Phase F 的登记项在代码中**显式存在且未变**。

| # | Requirement | Current State | Evidence | Missing | Proposed Design（承接 Phase F D1–D11） | Consensus Impact | Implementation Risk |
|---|---|---|---|---|---|---|---|
| **R1** | 区块索引树 `BlockNode{Hash,Parent,Height,Bits,Timestamp,Work,CumulativeWork,Status}` + `map[hash]*BlockNode` | 仅 `blocks []*block.Block` 线性切片（`Blockchain` 结构体） | `blockchain.go:51` `blocks []*block.Block`；`blockchain.go:344-353` reorg TODO | 树结构、按 hash 索引、children 指针、active tip 指针 | `BlockNode` 独立于完整 Block（§C）；`index map[[32]byte]*BlockNode` + `bestTip` 字段 | 是（链选择的数据前提） | 低：纯内存结构，启动由 `blocks.dat` 重建 |
| **R2** | 累积工作量 `chainwork` 累加器 | `Blockchain` 无 chainwork；`currentBitsLocked()` 钳制恒返回 16 | `blockchain.go:175-192`；`pow.go` `AdjustBits` 双钳制→16 | 每节点 `CumulativeWork *big.Int`；`Work(bits)=2^bits` | §D 契约：递归 `CumulativeWork=parent+Work(bits)`，本地重算，永不信任远端 | 是（fork-choice 核心） | 低：`*big.Int` 累加 |
| **R3** | Best-chain 指针 + `SetTip`/`ConnectBlock`/`DisconnectBlock` | 仅 `addBlock` 单向 append；`utxo` 字段整体替换 | `blockchain.go:313-342`；`bc.blocks=append(...)`；`bc.utxo=newSet` | `SetTip`/`Disconnect`/`Connect`；`bestTip` 切换 | §F 状态机；`SetTip` 成功后复用 `notifyTipChanged()`（D8） | 是（链切换） | 中：原子性 + crash 一致性（§J） |
| **R4** | UTXO undo journal（每块逆操作集）**或** 检查点重放 | `Spend` 仅 `delete` 无逆日志；`ApplyBlock` 返回新集合但无 per-block undo | `utxo/set.go:57-66` `Spend` 只 delete；`utxo/apply.go:218-265` | `BlockUndo`（spent 原条目 + created outpoint） | §G：undo journal（D3），`Disconnect` 精确复原 | 是（状态迁移正确性） | **高（核心难点）**：undo 与 maturity 交互 |
| **R5** | 存储层：索引由「高度数组」改为「hash→offset」+ `SetTip`/`Truncate`/`DeleteBranch` | `FileBlockStore` append-only；`SaveBlock` 强制高度连续 + prev 必须已存在；无 Delete/Truncate | `storage/file.go:119-149` `SaveBlock` 约束；`byHeight []*Block` | hash 索引 + 活跃链回溯；旧块软保留 | §J：B/E 混合（D2：物理不删，活跃链回溯） | 否（存储格式） | 中：数据迁移 + 元数据文件 |
| **R6** | Mempool `ReaddDisconnected(blocks)` + 重验 | 仅 `RemoveIncluded`；无复活 | `mempool/mempool.go:148-177` | `ReaddDisconnected` 把被断开交易重验后回池 | §K/§11：复用 `compositeView` 思路，局部可控 | 否（policy） | 低：新增 1 方法 |
| **R7** | Orphan pool（按 `PrevHash` 索引） | 无；`OnNewBlock` 非 tip-prev 即忽略 | `cmd/node/service.go:154-159` `忽略区块 return` | orphan pool：`prevHash→[]*Block` | §H：本地 policy，PoW 校验后才入池 | 否（policy） | 低：有界容量 + 淘汰 |
| **R8** | P2P：握手带 `CumulativeWork`；按 hash 拉分支 | 握手仅 `ChainHeight`；`BlocksFrom` 高度驱动；`OnBlocksResp` 首不匹配即 return | `p2p/node.go:61-67`；`service.go:201` `BlocksFrom`；`service.go:234-237` return | 握手加 `CumulativeWork`（hint）；`getblocks-by-hash`；`OnBlocksResp` 越过不匹配 | §I：协议字段表 + 兼容性 | 是（协议变更，须版本化） | 中：软兼容（可选字段） |
| **R9** | Reorg 最终性策略（最大深度/超限拒绝/事件指标） | 无 `MaxReorgDepth`、无确认数、`/status` 无 reorg 字段 | `blockchain.go` 无；`control/server.go` 仅 `Bits`/`Difficulty` | `MaxReorgDepth≤100` + 拒绝+告警；确认数 API；RPC 观测字段 | §14/§K/§L：`MaxReorgDepth` 为 policy 安全阀，非 PoW 安全 | 是（深度上限执行） + 否（最终性为 policy） | 低 |

> **R10（Phase F 补充）**：难度钳制带宽再评估 = 独立共识参数阶段（`DIFFICULTY-CONSENSUS-DESIGN-1`），本阶段**只评估其作为强制前置**（见 §20、§M-REORG-1N）。

**审计结论**：Phase F 的 R1–R9 在最新代码中**全部仍为「缺失/退化」**，无一项被意外实现或漂移；缺口性质与登记一致。本阶段为每一项给出**冻结的设计契约**（§C–§J），消除「设计空白」。

---

## C. BLOCK TREE DESIGN

### C.1 结构

```text
BlockNode
├── Hash            [32]byte      // 共识关键：区块头双 SHA256
├── ParentHash      [32]byte      // 共识关键：指向父节点
├── Height          int           // 共识关键
├── Bits            uint32        // 共识关键：本地解出，用于 Work(bits)
├── Timestamp       int64         // 共识关键
├── Work            *big.Int      // 共识关键：Work(Bits)=2^Bits
├── CumulativeWork  *big.Int      // 共识关键：parent.CumulativeWork + Work
├── Parent          *BlockNode    // 索引（由 ParentHash 解析）
├── Children        []*BlockNode  // 索引（多子=分叉）
├── Status          enum{Valid, Orphan, Side}  // 共识关键：validity
└── hasFullBlock    bool          // local-only：完整块是否在 storage
```

`BlockNode` **独立于完整 `Block`**：完整块始终留在 `storage`（append-only `blocks.dat`）；`BlockNode` 是轻量索引/共识视图。

### C.2 十个问题解答

1. **BlockNode 是否独立于完整 Block？** 是。完整块不可变地存于 `storage`；`BlockNode` 只持共识字段 + 指针，内存常驻。
2. **parent lookup 如何？** `node.Parent` 指针（启动时由 `ParentHash` 在 `index` 中解析填充）；同时 `index[ParentHash]` 可查。
3. **hash → node 如何索引？** `index map[[32]byte]*BlockNode`（`byHash` 升级版），与 Phase F D2 一致。
4. **active tip 如何表示？** 显式字段 `bestTip *BlockNode` + `bestTipWork *big.Int`；不再靠 `blocks[len-1]` 推断。
5. **competing branches 如何保存？** 同一 `index` 内的多 children 节点即侧链；仅 `bestTip` 为活跃。侧链保留至被策略淘汰（深度/资源），不做物理删除（D2）。
6. **orphan 如何进入 tree？** orphan **不直接成为** `BlockNode`；先入 orphan pool（§H）按 `PrevHash` 索引；父节点到达并链接后，提升为 `BlockNode` 并接入 `Children`。
7. **重启后如何恢复 tree metadata？** 启动扫描 `blocks.dat`（`loadIndex` 已有顺序扫描），逐块建 `BlockNode` + `index` + `Children`；`bestTip` = `CumulativeWork` 最大且 `Status==Valid` 的节点（确定性，tie-break 见 §E）。`CumulativeWork` 由本地 bits **重算**，不读远端。
8. **cumulative work 是否缓存？** 是，每节点缓存（O(1) fork-choice）。
9. **cached work 如何验证？** 连接/校验时断言 `node.CumulativeWork == node.Parent.CumulativeWork + Work(node.Bits)`；启动全量校验一次；RPC/调试可触发局部复算。
10. **是否允许 lazy calculation？** **不允许**在共识关键路径懒惰计算——`CumulativeWork` 必须在 `ConnectBlock` 时立即算出（fork-choice 依赖）；仅 RPC/调试展示允许按需复算。

### C.3 共识关键数据 vs 本地元数据（显式分离）

- **consensus-critical**：`Hash, ParentHash, Height, Bits, Timestamp, Work, CumulativeWork, Status, Parent, Children` —— 全部由本地验证推导，绝不来自网络。
- **local-only metadata**：`hasFullBlock`、到达时间、来源 peer、中继计数、orphan 池成员资格、LRU 时间戳、序列化缓存大小 —— 不参与共识，重启可丢失。

---

## D. CUMULATIVE WORK CONTRACT

### D.1 形式化定义

```text
Work(bits)        = 2^bits                         // 等价 1/target，target=2^(256-bits)（见 pow.BitsToTarget）
Work(bits)        = new(big.Int).Lsh(big.NewInt(1), uint(bits))

CumulativeWork(genesis) = Work(genesis.Bits)       // 创世无父，自含
CumulativeWork(node)    = CumulativeWork(parent) + Work(node.Bits)
```

与 Phase G §2 一致：`chainwork=Σ 2^bits_i`，累加器必须 `*big.Int`。

### D.2 逐项明确

- **genesis**：`CumulativeWork(genesis)=2^16`。创世同 bits=16 ⇒ 无特殊分支（与 Phase G 证明一致）。
- **overflow**：当前 `height×2^16`（h=1259 ⇒ 8.24e7）虽 < 2^63，但**未来难度浮动后** `bits` 可达 32+，`height×2^32` 在 ~2.1e9 块处溢出 64 位 ⇒ **强制 `*big.Int`**。
- **`big.Int`**：所有 `Work` / `CumulativeWork` 运算走 `*big.Int`；比较用 `Cmp`。
- **serialization**：元数据文件（§J）中以大端定长字节存 `CumulativeWork`；RPC 以十进制字符串（或 0x hex）表示。
- **RPC representation**：新增 `tip_work` 字段（`control/server.go` 当前仅有 `Bits`/`Difficulty`，`server.go:79-84`），值为 `bestTip.CumulativeWork.String()`。
- **comparison**：`candidate.CumulativeWork.Cmp(active.CumulativeWork)`；`>0` 候选胜，`<0` 活跃胜，`==0` 走 §E tie-break。
- **persistence**：`CumulativeWork` 随 `BlockNode` 元数据落盘（§J）；**启动时从 genesis 重算全链校验**（不信任落盘值）。
- **restart recovery**：同 §C.7——由 `blocks.dat` 重建并本地重算，落盘 `CumulativeWork` 仅作缓存。
- **cache invalidation**：单链内 `CumulativeWork` 单调且永不改；仅在 **reorg 重连** 时，从 common ancestor 起向前重算受影响分支（§F/§9）。
- **maliciously supplied chainwork 是否必须重新计算？** **必须。** 节点**绝不信任**远端提供的 `CumulativeWork`（§6 原则）。P2P 握手携带的 `CumulativeWork` 仅作「是否需要同步」的 **hint**，真相一律由本地验证后的 `bits` 重算。

---

## E. FORK-CHOICE CONTRACT

### E.1 确定性规则

```text
if   candidate.CumulativeWork > active.CumulativeWork:  candidate wins
elif candidate.CumulativeWork < active.CumulativeWork:  active wins
else:  deterministic tie-break
```

### E.2 tie-break 分析（为何只有一种可选）

| 候选 tie-break | 能否作 consensus fork-choice | 原因 |
|---|---|---|
| **tip hash 字典序**（推荐） | ✅ | 由区块内容确定性派生，所有节点独立复算得同一结果 |
| first-seen（先到先得） | ❌ | network arrival order 决定共识 ⇒ 网络分区即链分裂 |
| peer ID | ❌ | Sybil 可操控；非确定性 |
| arrival time | ❌ | 时钟/网络抖动 ⇒ 节点间不一致 |
| random | ❌ | 非确定性 ⇒ 永久分叉 |
| local node state | ❌ | 各节点状态不同 ⇒ 永不收敛 |

**推荐**：`tip hash lexicographical ordering` —— 将 `bestTip.Hash()`（`[32]byte`）按大端整数比较，**较大者胜**（或冻结为「较小者胜」，关键是全局唯一且可复算）。与 Phase F D1 一致。

**铁律**：**network arrival order 绝不能决定 consensus chain。** fork-choice 只取决于（本地验证的）`CumulativeWork` + 确定性 tie-break。

---

## F. REORG STATE MACHINE

```text
ACTIVE
  ↓ (收到 prev 不在 tip 的合法块 / 侧链 chainwork 反超)
COMPETING BRANCH DETECTED
  ↓
CANDIDATE VALIDATION      // 逐块 validateBlock（含 PoW/结构/UTXO 克隆迁移）
  ↓ (任一失败 → 丢弃候选，回到 ACTIVE)
WORK COMPARISON          // candidate.CumulativeWork vs active（§E）
  ↓ (≤ → 丢弃候选)
COMMON ANCESTOR          // findCommonAncestor(oldTip, newTip)（§9）
  ↓
DISCONNECT OLD BRANCH    // 标记 oldTip..ancestor 待断开
  ↓
ROLLBACK STATE / UTXO    // 在克隆集合上应用 undo（§G），不改变活跃 utxo
  ↓
CONNECT NEW BRANCH       // 在克隆集合上顺序 ApplyBlock 新分支
  ↓
VERIFY RESULT            // 断言新 utxo 自洽 + chainwork 一致
  ↓ (失败 → 丢弃克隆，活跃态不变)
COMMIT NEW TIP           // 原子替换 bestTip + utxo 指针（§J 原子性）
```

### F.1 每状态契约（节选关键不变量 / crash 行为）

| 状态 | 输入 | 输出 | 不变量 | 失败 | crash 行为 |
|---|---|---|---|---|---|
| CANDIDATE VALIDATION | 候选块序列 | 通过/拒绝 | 在 **克隆** UTXO 上校验，活跃态不变 | 拒绝→回 ACTIVE | 崩溃无副作用（活跃态未动） |
| WORK COMPARISON | 两 chainwork | 胜者 | 用本地重算值 | ≤ → 丢弃 | 同 |
| COMMON ANCESTOR | oldTip,newTip | ancestor 节点 | ancestor 必存在且两链可达 | malformed→丢弃 | 同 |
| DISCONNECT/ROLLBACK | 克隆集 + undo | 回滚后集 | 不改 `bc.utxo`（仅改克隆） | 失败→丢弃克隆 | 活跃态完整 ⇒ 重启即 A |
| CONNECT | 克隆集 + 新分支 | 新集 | `Verify Result` 前不替换活跃 | 失败→丢弃克隆 | 活跃态完整 ⇒ 重启即 A |
| COMMIT NEW TIP | 新 bestTip + 新集 | 指针替换 | **单点原子**（锁内指针 swap + 元数据 fsync） | — | 见 §J 契约 |

### F.2 原子性设计（核心）

> **reorg 过程中任一步失败，节点绝不能留下半回滚 / 半连接状态。**

设计：**先在全克隆上完成 `ROLLBACK+CONNECT+VERIFY`，仅在 `COMMIT NEW TIP` 一步做锁内 `bestTip` / `utxo` 指针原子替换**，且**替换发生在所有新块已落盘、元数据已 fsync 之后**。任意前置步骤失败 → 克隆被丢弃 → 活跃态（A）从未改变 → 重启即得 A。因此「半状态」在内存中不可观测，在磁盘上由 §J 契约约束。

---

## G. UTXO ROLLBACK DESIGN（最高风险之一）

### G.1 当前能力审计

- `UTXOSet.Spend`（`utxo/set.go:57-66`）：仅 `delete`，**无逆日志** → 确认 FC-008。
- `ApplyBlock`（`utxo/apply.go:218-265`）：在 `base.Clone()` 上原子迁移，返回**新集合**；但**不产出 per-block undo**。
- `Entry`（`utxo/set.go:10-15`）已带 `Height` / `IsCoinbase` ⇒ 复原所需「creator 信息」已具备。
- 持久化：UTXO 集合本身不独立落盘，启动由 `blocks.dat` 全量重放（`NewBlockchainFromStore`，`blockchain.go:73-113`）。

### G.2 核心问题

> 当前系统是否保存了足够信息，使任意合法 reorg 可逆向恢复 UTXO state？
> **答：否。** 既无 undo journal，也（在深度 > 链长时）无结构化回退；全量重建仅在 `MaxReorgDepth` 内可行。

### G.3 设计：`BlockUndo`（承接 Phase F D3）

每块附加逆操作集：

```text
BlockUndo
├── spent    []UndoSpent   // 被本块消费的 UTXO：OutPoint + 原 Entry(Value,PubKeyHash,Height,IsCoinbase)
└── created  []OutPoint    // 本块新建的 UTXO（断开时移除）
```

- **Disconnect(block, undo)**：
  1. 移除 `created`（本块新建输出）；
  2. 按 `spent` 把原 `Entry` 重新 `Add` 回集合（精确复原 `Height`/`IsCoinbase`）。
- **确定性**：UTXO 顺序由 block 内 tx 顺序 + tx 内 output index 决定，`UndoSpent` 与 `created` 均按该顺序记录 ⇒ 可逆且确定性。

### G.4 约束

- **禁止**把「重新扫描当前链」作为唯一 rollback 机制（§10 铁律）。全量重建仅作 `MaxReorgDepth` 内兜底。
- **maturity 副作用**：reorg 使 `currentHeight` 回退 ⇒ 已成熟 coinbase 可能重新未成熟（`Balance` 纯函数派生，自动正确，但**用户可见余额回跳**）。须 UI/文档声明「未达最终性余额可变」（Phase F §6.3）。
- **undo 落盘**：`BlockUndo` 随元数据文件（§J）持久化，供 `MaxReorgDepth` 内 O(depth) 回滚，避免每次重启全量重放。

---

## H. ORPHAN DESIGN

```text
unknown parent
  ↓ (校验 PoW + 头结构后)
orphan pool        // key=PrevBlockHash → []*Block
  ↓ (父块到达 / 被拉取)
attach             // 提升为 BlockNode，接入 Children
  ↓
validate           // validateBlock（全序）
  ↓
calculate work     // 本地 CumulativeWork
  ↓
evaluate chain     // fork-choice（§E）
```

### H.1 边界情形

- **eviction**：按数量上限 + LRU + 过期；满时淘汰最旧/最低 work，拒绝新 orphan 入池。
- **duplicate blocks**：`index` + orphan 集按 hash 去重。
- **malicious orphan flood**：orphan **入池前必须先验证 PoW + 头**（廉价），否则直接丢弃；容量上限 + 速率限制。
- **parent out of order**：由 orphan pool 自然吸收，父到即提升。
- **multi-level orphan chains**：每个未知父块各自入池；根父到达后递归提升整链。
- **orphan → active transition**：提升时即跑 fork-choice，可能触发 reorg（§F）。

### H.2 边界声明

**orphan pool 是本地 policy；block validity 是 consensus。** orphan 池只负责「暂存待认领」，绝不改变共识判定。

---

## I. P2P INTEGRATION DESIGN

### I.1 当前行为确认

`OnNewBlock`（`cmd/node/service.go:154-159`）：`if b.Header.PrevBlockHash != tip.Header.Hash() { 忽略区块; return }` —— 即 `PrevBlockHash != tip.Hash()` 时**仅 ignore**（FC-002 / MM-1）。未来**不能**继续如此（竞争块被静默丢弃 ⇒ 永久分叉）。

### I.2 未来设计

```text
receive competing block
  ↓ (PoW + 头校验)
store                 // 入 index（侧链）或 orphan pool（父未知）
  ↓
validate             // validateBlock
  ↓
attach               // 接入 tree
  ↓
calculate cumulative work
  ↓
fork-choice (§E)
  ↓
reorg if necessary (§F)
```

### I.3 协议扩展（字段表，须版本化）

| 消息 | 变更 | 说明 |
|---|---|---|
| `HandshakePayload` | 增 `CumulativeWork string`（hint） | `p2p/node.go:61-67`；仅作同步触发判据，非真相 |
| `GetBlocksPayload` | 增 `FromHash [32]byte`（可选） | 支持按 hash 拉分支，而非仅高度 |
| `MsgGetHeaders` / `MsgInv`（新增） | 分支宣告 | 先交换 hash 再拉块，避免全量 |
| `OnBlocksResp` | **越过首不匹配继续** | 当前 `service.go:234-237` 首不匹配即 return ⇒ FC-003 永久孤岛；须改为「跳过不知节点、继续后续」 |

### I.4 情形分析

- **duplicate**：hash 去重。
- **competing**：存为侧链，fork-choice 决定。
- **out-of-order**：orphan pool 吸收。
- **malicious peer**：本地全量校验，绝不信任远端 `CumulativeWork`；reorg 频率限速。
- **branch announcement / block request / orphan recovery**：由 `FromHash` + `MsgInv` 支持。

**兼容性**：新增字段为**可选**（旧节点忽略未知字段）→ 软兼容，非硬分叉；但旧节点无法把侧链喂给新节点，故 reorg 跨版本收敛需双方均升级——在 REORG-1H 验收中标注。

---

## J. PERSISTENCE / CRASH SAFETY

### J.1 `FileBlockStore` 审计

当前 append-only（`storage/file.go`）：`byHeight []*Block` + `byHash map`；`SaveBlock` 强制高度连续 + prev 已存在（119-149）；**无 Delete / Truncate**（FC-007）。足以支持 branch storage（块都留着），但**缺活跃链回溯与索引**。

### J.2 方案比较（不替你选，给出推荐）

| 方案 | 描述 | 优劣 | 取舍 |
|---|---|---|---|
| A. full rebuild | 每次启动从创世重放 | 最简；1259 块内可接受 | 随链长线性劣化；启动 O(N) |
| B. metadata index | 独立 `block_index.dat`：每块 hash/parent/height/bits/timestamp/cumulativeWork/status + active tip | 启动 O(1) 恢复 tree；`blocks.dat` 不变 | 需维护元数据一致性 |
| C. journal/WAL | reorg 操作日志 | 精确回放 | 复杂，与 B 重叠 |
| D. atomic snapshot | 周期 UTXO 快照 | 启动快 | 需与 undo 配合 |
| **E. hybrid（推荐）** | **B + 周期快照(D) + `MaxReorgDepth` 内 undo journal(G)** | 启动快 + O(depth) 回滚 + 崩溃可恢复 | 实现量中等 |

**推荐 E**：`blocks.dat` 保持 append-only（保留所有分支，D2）；新增 `block_index.dat`（hash→元数据 + active tip 指针 + cumulativeWork）；`utxo_snapshot.dat` 周期快照（checkpoint 高度）；`undo` 随元数据存最近 `MaxReorgDepth` 块。

### J.3 原子 tip 提交

- `COMMIT NEW TIP`（§F）在锁内：① 写所有新块（已在 `blocks.dat`）；② 更新 `block_index.dat`（含新 `bestTip` + 新节点 cumulativeWork）；③ **fsync 后**原子切换 `bestTip` 指针（写临时文件 + `rename` 或单 int + fsync）。
- 指针切换是 reorg 的**唯一不可逆点**；此前任何崩溃 ⇒ 活跃态（A）完整。

### J.4 REORG CRASH-SAFETY CONTRACT（8 崩溃点）

> 模型：`old chain A` / `new chain B` / `common ancestor X`。
> **承诺：restart 后确定性恢复到 A 或 B，绝不出现第三种非法状态。**

| # | 崩溃点 | 重启恢复 |
|---|---|---|
| 1 | before disconnect | 活跃=A（B 尚未触碰）⇒ A |
| 2 | during disconnect（克隆上） | 活跃=A 未变 ⇒ A |
| 3 | after UTXO rollback（克隆上） | 活跃=A ⇒ A |
| 4 | before new branch connect | 活跃=A ⇒ A |
| 5 | during new branch connect（克隆上） | 活跃=A ⇒ A |
| 6 | after new tip update（内存，未 fsync 元数据） | 元数据仍指向 A ⇒ A |
| 7 | before disk persistence | 元数据=A ⇒ A |
| 8 | after disk persistence | 元数据=B ⇒ **B** |

**契约结论**：因 reorg 在克隆上完成、仅 `COMMIT NEW TIP` 单点原子切换且晚于所有落盘，**8 点崩溃均收敛到 A 或 B**，无第三态。REORG-1J 须以注入式崩溃测试逐点验证（§M）。

---

## K. CONSENSUS VS POLICY

### Consensus-critical（必须确定性、本地验证、全网一致）
- block validity（PoW / 结构 / Merkle / 时间 / 体积）
- parent relationship（`PrevHash` 必须存在于 index 且为 ancestor）
- cumulative work 计算（本地，由 bits）
- fork-choice（max cumulative work + 确定性 tie-break）
- state transition（UTXO apply / disconnect 正确性）
- reorg correctness（common ancestor + 原子 swap）

### Local policy（可因节点而异，不影响共识）
- orphan pool size / eviction
- mempool size / relay strategy
- max cached branches / peer limits
- resource limits（reorg 频率 / 深度预算 / DoS 限速）
- validation cache

### 模糊项澄清
- **`MaxReorgDepth`**：属 **policy 安全阀**，其「拒绝 + 告警」是本地资源保护，**不是 PoW 安全规则**；共识规则始终是「candidate work > active work」。不得宣传为安全性（§14、§L）。
- **checkpoints**：默认 **local policy**（§15）；若强制为不可越过的历史边界则需 **hard fork**（单独阶段）。
- **tie-break**：**consensus**（必须确定性，全网同一结果）。

---

## L. SECURITY ATTACK MATRIX

> ⚠️ **reorg infrastructure 本身 ≠ 修复 FC-004。** bits=16 下 `chainwork ≡ height`，reorg 正确 **不** 等于 PoW 历史安全已解决。

| Attack | Current | After Reorg | After Difficulty Float |
|---|---|---|---|
| **cheap long chain** | bits16 ⇒ 单核 ~60ms/块，可重写整链（FC-004） | 同（仍 bits16，chainwork≡height） | 仅当 `DIFFICULTY-CONSENSUS-DESIGN-1` 完成（MTP+新 MaxDifficultyBits）才安全 |
| **low-work branch** | 被 ignore（prev≠tip） | 被拒（work<active） | 被拒 |
| **deep reorg** | 不可能（无 reorg） | 可能，但 `MaxReorgDepth` 上限；bits16 仍廉价造深链 | 安全（难度抬升后深链成本高） |
| **orphan flood** | 无池，直接丢 | 有界 orphan 池 + 入池前 PoW 校验 | 同 |
| **timestamp manipulation** | 钳制（`maxFutureTimestampDrift=7200`、须 ≥ 父） | 同 + fork-choice 不依赖时间 | 需 MTP（超出本阶段，属前置） |
| **equal-work branch** | 不出现（单链） | tie-break 确定（tip hash） | 同 |
| **malicious peer** | 受限（ignore） | 本地全校验 + reorg 限速；绝不信任远端 work | 同 |
| **crash during reorg** | N/A | §J 契约 ⇒ 恢复 A 或 B | 同 |

---

## M. IMPLEMENTATION PLAN（REORG-1A … REORG-1N）

> 每阶段：scope / files / consensus impact / tests / production risk / rollback / hard fork。承接 Phase F W1–W10 依赖。

| ID | Scope | Files（likely） | Consensus Impact | Tests | Prod Risk | Rollback | Hard Fork |
|---|---|---|---|---|---|---|---|
| **REORG-1A** | R1 `BlockNode` + `index` + `Children` + `bestTip`（启动由 `blocks.dat` 重建） | `internal/blockchain` | 否（仅索引） | 树构建/链接/重启重建单测 | 低 | 回退=恢复 `blocks []*Block` 字段 | 否 |
| **REORG-1B** | R2 `Work(bits)`+`CumulativeWork` 契约（`*big.Int`，本地重算） | `internal/blockchain`+`pow` | **是**（定义 work） | work 计算/overflow/genesis/缓存校验 | 低 | 回退=移除字段 | 否（纯 infra，bits 仍 16） |
| **REORG-1C** | R3 `SetTip`/`ConnectBlock`/`DisconnectBlock` 脚手架 + `bestTip` 切换 | `internal/blockchain` | **是**（tip 选择） | 单元：切换/回退 | 中 | 回退=单链 append | 否 |
| **REORG-1D** | R4 `BlockUndo` + `Disconnect`/`Connect`（G.3） | `internal/utxo`+`blockchain` | **是**（状态迁移） | 断开精确复原 / 往返一致 / maturity 回退 | **高** | 回退=禁用 reorg（单链） | 否 |
| **REORG-1E** | R5 存储 hash 索引 + `block_index.dat` + active tip 持久化（J.2-E） | `internal/storage` | 否（格式） | 加载重建 / 迁移旧 `blocks.dat` 可读 | 中（数据迁移） | 回退=旧 append 模式 | 否（文件格式） |
| **REORG-1F** | R6 `Mempool.ReaddDisconnected` + 重验 | `internal/mempool` | 否（policy） | 复活 + 重验 + 顺序敏感 | 低 | 回退=仅 RemoveIncluded | 否 |
| **REORG-1G** | R7 orphan pool（H） | `internal/blockchain`（或新包） | 否（policy） | 提升/淘汰/洪泛/多级 | 低 | 回退=入池即丢 | 否 |
| **REORG-1H** | R8 P2P `CumulativeWork` hint + `FromHash` 拉分支 + `OnBlocksResp` 越过不匹配 | `internal/p2p`+`cmd/node` | **是（协议变更，须版本化）** | 跨分支同步 / 越过不匹配 / 兼容旧节点 | 中 | 回退=旧协议路径 | 否（软兼容） |
| **REORG-1I** | R9 链选择 + tie-break + `MaxReorgDepth` + 最终性 API（§14/E） | `internal/blockchain`+`control` | **是**（fork-choice）/ `MaxReorgDepth` 为 policy | A1–A8（含平局一致、超深拒绝） | 中 | 回退=最长链+无深度限 | 否 |
| **REORG-1J** | §F 状态机 + 原子 tip 提交 + §J 崩溃契约 | `internal/blockchain`+`storage` | **是**（原子性） | **8 点注入式崩溃测试**（§J.4） | 高 | 回退=禁用 reorg | 否 |
| **REORG-1K** | D9 RPC 观测：`tip_work`/`reorg_count`/`reorg_depth_last`/`reorg_depth_max`/`side_chain_blocks`/`orphan_pool_size` | `control/server.go`+`cmd/node` | 否 | `/status` 字段单测 + 可探测分叉 | 低 | 回退=移除字段 | 否 |
| **REORG-1L** | 集成 + 双机收敛实验（A5）：复现 Phase E E1 ⇒ 两侧收敛同 `tip_hash`+`reorg_count≥1`+重连自愈 | disposable harness（同 `p2pchain-blockrate-capacity-harness`） | 验证 | 双节点确定性收敛 | 中（实验） | 实验环境，无生产影响 | 否 |
| **REORG-1M** | 钱包 reorg 感知缺口文档（Phase F：钱包 READY 但缺历史/确认/重发） | docs/policy | 否 | — | 低 | — | 否 |
| **REORG-1N** | **并行轨道**：难度带宽再评估（R10/D10/FC-004）→ 独立阶段 `DIFFICULTY-CONSENSUS-DESIGN-1`（MTP + 新 `MaxDifficultyBits` + hard fork） | `internal/pow`+共识参数 | **是（硬分叉）** | 单独阶段交付 | 高 | 硬分叉回滚 | **是** |

### M.1 依赖与门控
- 1A/1B → 1C → 1D/1E/1I → 1J（原子性收口）→ 1F/1G/1H/1K（周边）→ 1L（验收）→ 1M（文档）。
- **1N 与 1A–1M 并行推进但独立交付**；1N 是「难度浮动」的强制前置（G2），**不是 reorg 实现的阻塞项**。
- **建议门控**：REORG-1L 双机收敛实验通过，方宣布生产可用。

---

## FINAL VERDICT

```text
FINAL VERDICT:
PASS WITH DESIGN GAPS

READY FOR:
REORG-INFRASTRUCTURE-IMPLEMENTATION-1

BLOCKERS:
（无阻塞 REORG-INFRASTRUCTURE-IMPLEMENTATION-1 本身启动的项）
  - 注：本设计在 bits=16 下完全可构建（chainwork≡height ⇒ fork-choice 退化为最长链，
    但架构已是一般模型，未来仅 bits→Work(bits) 变化，无需改 fork-choice 架构）。

MANDATORY PRECONDITIONS:
1. DIFFICULTY-CONSENSUS-DESIGN-1（MTP + 新 MaxDifficultyBits + 硬分叉，即 REORG-1N / Phase G G2）
   必须在「任何 difficulty float 启用」之前完成。reorg 基础设施本身不修复 FC-004；
   若未来在「仍高度制 fork-choice / 未解钳」状态下浮动难度 → 升级为
   BLOCKED — CONSENSUS PARAMETER DESIGN REQUIRED。
2. 全部文档 / RPC / 状态展示须持续声明「reorg 正确 ≠ PoW 历史安全已解决」
   （bits=16 下 chainwork≡height，历史重写成本仍 ~60ms/块单核），不得过度宣称。
3. 实现阶段须以 REORG-1L 双机收敛实验作为生产安全门控；
   并以 REORG-1J 的 8 点注入式崩溃测试证明 CRASH-SAFETY CONTRACT。

设计缺口（DESIGN GAPS，须下游/并行关闭，非实现阻塞）：
- FC-004 难度钳制（REORG-1N，独立硬分叉阶段）。
- 钱包 reorg 历史/确认/重发感知（REORG-1M，文档级技术债，不阻塞 reorg）。
- timestamp Median-Time-Past（MTP）属 REORG-1N 前置，本阶段仅要求「fork-choice 不依赖时间」已满足。
```

### 一句话定论
本阶段把 R1–R9 + L0–L4 + D1–D11 冻结为**可拆 WBS 的实现契约**：一般化 `Block → Work(bits) → CumulativeWork → Deterministic Fork-Choice → Chain Tree → Common Ancestor → State Rollback → Atomic Tip Commit` 模型已闭合、与未来难度浮动兼容、crash 一致性可证明。在 **bits=16 下可安全进入 `REORG-INFRASTRUCTURE-IMPLEMENTATION-1`**；但 **PoW 历史安全（FC-004）必须由强制前置 `DIFFICULTY-CONSENSUS-DESIGN-1` 独立关闭**，reorg 基础设施不得被误读为安全修复。

---
*HARD STOP：本阶段为 READ-ONLY 设计审计。未实现、未 commit、未 push、未启动下一阶段。交付物仅为本实现契约。*
