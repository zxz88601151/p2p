# PHASE F-3 — GENESIS IDENTITY & STARTUP STATE IMPLEMENTATION READINESS AUDIT

**阶段性质**：STRICT READ-ONLY / IMPLEMENTATION-READINESS AUDIT / NO CODE CHANGE / NO BUILD / NO MIGRATION / NO WALLET ACCESS BEYOND STRUCTURAL INSPECTION / NO GIT WRITE
**前置阶段**：PHASE F-2（`PHASE-F2-PROTOCOL-IDENTITY-DECISION-FREEZE.md`，契约已冻结）
**当前 HEAD**：`4d892be355a38886a83c123faad915c9f3cd62e4`
**本阶段判定**：

```
PHASE F-3 STATUS:
BASELINE:                   PASS
VERDICT:                    READY WITH BLOCKERS
BLOCKERS:                   5 (B-1..B-5)
IMPLEMENTATION AUTHORIZED:  NO
HARD STOP:                  YES
```

---

## §1 EXECUTIVE SUMMARY

F-3 审计了"把 F-2 的 Genesis Identity Contract 转化为可实施状态模型"所需的全部前置条件。
结论：**READY WITH BLOCKERS** —— 实施路径明确，但存在 5 个必须先解决的明确 blocker。

**核心事实（全部源码取证）**：

1. **当前不存在任何 genesis identity check。** `NewBlockchainFromStore`（`blockchain.go:217`）读取磁盘创世后
   **直接使用**，从不与期望值比较；全仓非测试 `.go` 中创世哈希字面量命中 = **0**。
2. **Genesis identity 未以任何显式元数据持久化。** 数据目录仅含 `blocks.dat` / `node.lock` / `wallet.json`；
   身份**仅隐式存在于 `blocks.dat` 的第一条记录**内。→ `NOT CURRENTLY PERSISTED AS EXPLICIT IDENTITY METADATA`
3. **Case D（身份损坏）与 Case A（空库）在存储层不可区分。** `Height()` 以 `len(byHeight)-1` 判定；
   若 `blocks.dat` 缺失或为 0 字节 ⇒ 返回 `-1` ⇒ **被当作空库 ⇒ 自动生成并落盘新创世**。
   这**直接违反** F-2 Case D（必须 FAIL CLOSED，不得自动重新初始化）。
4. **初始化是隐式的，不是显式的。** 没有任何 `--init` 标志；只要打开一个空/新目录就会自动生成创世。
   F-2 Case A 要求"**explicit** initialization"，当前实现不满足。
5. **`verify` 是独立旁路。** `cli.go:556` → `VerifyStoredChain` → `NewBlockchainWithGenesis(first)`，
   **完全绕过** `NewBlockchainFromStore`，且源码注释明确"创世区块按既有设计视为可信输入……不新增创世侧校验规则"。
6. **`GENESIS_MISMATCH` 不存在** → `REQUIRED BY F-2 / NOT YET IMPLEMENTED`。
   当前 mismatch 只以**经济错误** `ErrExcessiveCoinbase` 的形式**偶然**暴露。
7. **mismatch 检测是方向依赖的。** 仅当"磁盘创世 coinbase **大于** 期望补贴"时才失败。
   若磁盘创世 coinbase **≤** 期望补贴，`NewBlockchainWithGenesis` 会**成功**，
   mismatch **完全不被察觉**，回放将带着**外来创世**继续。
8. **P2P 握手字段 `genesis_hash` 被发送与解码，但从未被校验。** 全仓无任何 `payload.GenesisHash` 比较。
   注释称"用于快速识别网络不一致"，属**意图而非已实现行为**。
9. **不存在任何 auto-migration / auto-upgrade / auto-delete / auto-overwrite 代码。**（`grep migrat|upgrade|convert` = 0 命中）
10. **`reset` 是显式 + 确认门控的用户命令**，非自动回退 ⇒ 符合 F-2（删除需显式授权）。

**结论**：契约本身可实现，但必须先解决 5 个 blocker（§16），其中 **B-1（身份元数据缺失导致 Case A/D 不可区分）**
是**结构性**的，必须在实施前作出语义决策。

---

## §2 BASELINE VERIFICATION

| # | 检查项 | 期望 | 实测 | 结论 |
|---|---|---|---|---|
| 1 | HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` | 同 | ✅ |
| 2 | F-2 报告存在 | `PHASE-F2-PROTOCOL-IDENTITY-DECISION-FREEZE.md` | 存在 | ✅ |
| 3 | F-2 报告 SHA-256 | `001c48655c7ff693809420240d254abdfa109f8272cf9c0282ccbc20023bbdb9` | 同 | ✅ |
| 4 | tracked diff | 7 | 7 | ✅ |
| 5 | `node.exe` | `3972843a…c213e1` | 同 | ✅ |
| 6 | `run-a/blocks.dat` | `893e64c1…ea2fb2f` | 同 | ✅ |
| 7 | `run-b/blocks.dat` | `5ab67bb4…e1df53` | 同 | ✅ |
| 8 | `run-a/wallet.json` | `bf577525…6844a94` | 同 | ✅ |
| 9 | `run-b/wallet.json` | `762c3693…aa5190` | 同 | ✅ |

```
BASELINE PASS   ——  未触发 STOP — F-3 BASELINE DRIFT
```

---

## §3 F-2 CONTRACT INHERITANCE

本阶段**严格继承** F-2 已冻结内容，**不重新解释、不修改、不扩展**：

| F-2 契约 | 继承内容 |
|---|---|
| Protocol Identity | `PROTOCOL_IDENTITY_V1`；name `p2pchain`；**Protocol Version = `NOT DEFINED IN SOURCE`**；**Explicit Chain ID = `NOT DEFINED IN SOURCE`**；事实网络身份 = Genesis Hash |
| Genesis Identity | `EXPECTED_GENESIS_HASH` vs `ACTUAL_DATASET_GENESIS_HASH` → MATCH? YES→continue / NO→**FAIL CLOSED**；目标语义 **`GENESIS_MISMATCH`** |
| Dataset | NEW PROTOCOL ↔ NEW DATASET ↔ NEW GENESIS；Legacy = FORENSIC ARCHIVE；`Runtime/Migration/Automatic Upgrade/Automatic Deletion` = FORBIDDEN |
| Startup | Case A（空库+**显式**初始化→ALLOW）、B（匹配→VERIFY→CONTINUE）、C（不匹配→FAIL CLOSED）、D（损坏→FAIL CLOSED，不得当空库） |
| Private Key | `QUARANTINED / FORENSIC ARCHIVE ONLY` |
| GUI | 本阶段不修改 |

**本阶段未自行创造任何新的 Protocol Version 或 Chain ID。** 事实网络身份维持 = Genesis Hash。

---

## §4 CURRENT GENESIS IDENTITY ARCHITECTURE

### §4.1 唯一生产构造入口

全仓**非测试**代码中，能构造 `Blockchain` 的路径**只有 2 条**：

| 路径 | 文件:行 | 存储打开 | 链构造 | 身份校验 |
|---|---|---|---|---|
| **node 运行时** | `cmd/node/main.go:90` → `:96` | `OpenFileBlockStore`（可写） | `NewBlockchainFromStore` | **无** |
| **verify 命令** | `cmd/node/cli.go:556` | `OpenFileBlockStoreReadOnly` | `VerifyStoredChain` → `NewBlockchainWithGenesis(first)` | **无** |

`printchain`（`cli.go:468`）只读打印，**不构造链**。
`internal/control`（RPC）、`internal/mempool`（挖矿）**不打开存储、不构造链**（纯消费者）。
⇒ **不存在 RPC/P2P/mining 侧的独立构造入口。**

### §4.2 创世生成

```
internal/blockchain/genesis.go:24
    func NewGenesisBlock() *block.Block {
        cb := transaction.NewCoinbaseTx(GenesisMinerPubKeyHash, utxo.Subsidy(0), 0)
        g := block.NewCandidateBlock([32]byte{}, pow.MaxTargetBits, []*transaction.Transaction{cb})
        g.Header.Timestamp = GenesisTimestamp
        ...
    }
```

创世是**运行时派生**的（`Subsidy(0)` × `MaxTargetBits` × `GenesisTimestamp` × `GenesisMinerPubKeyHash`），
**非硬编码常量**，**无 hash pin**。

---

## §5 GENESIS STORAGE AUDIT

### §5.1 数据目录实际内容

```
run-a/          run-b/
  blocks.dat      blocks.dat
  node.lock       node.lock
  wallet.json     wallet.json
```

**不存在**任何 metadata / sidecar / manifest / identity 文件。

### §5.2 显式身份元数据检索

```
grep -rniE "metadata|sidecar|manifest|identity" internal/storage/*.go   （非测试）
    → 0 命中
```

### §5.3 结论

```
NOT CURRENTLY PERSISTED AS EXPLICIT IDENTITY METADATA
```

**Genesis identity 唯一载体 = `blocks.dat` 的第一条记录（height 0）。**
⇒ 一旦该记录丢失/不可读/被截断为 0，**身份即丢失**，且**无法与"全新空目录"区分**。

> 本审计**未自行设计实现**。

### §5.4 存储层的"空"判定（关键）

```
internal/storage/file.go:196
    func (s *FileBlockStore) Height() (int, error) {
        if s.v2.hasTip { return s.v2.tipHeight, nil }
        return len(s.byHeight) - 1, nil        // 空 ⇒ -1
    }
```

`byHeight` 由 `loadLog → scanLog → interpretScan` 从文件扫描重建。
**0 字节文件**与**全新文件**扫描结果相同（无记录）⇒ 二者都返回 `-1`。

---

## §6 GENESIS SOURCE-OF-TRUTH AUDIT

### §6.1 来源链

```
Genesis Definition            → genesis.go（Subsidy(0) / MaxTargetBits / GenesisTimestamp / GenesisMinerPubKeyHash）
        ↓
Canonical Genesis             → NewGenesisBlock()（运行时派生，唯一生成器）
        ↓
Persisted Genesis Identity    → blocks.dat 第一条记录（无独立元数据）
        ↓
Startup Verification          → 不存在（§8）
        ↓
P2P Handshake genesis_hash    → chain.BlockByHeight(0).HashHex()（main.go:136）
```

### §6.2 是否存在多个独立来源？

| 来源 | 存在？ | 说明 |
|---|---|---|
| hardcoded genesis（生产） | ❌ 不存在 | 非测试 `.go` 命中 0 |
| runtime-generated genesis | ✅ | `NewGenesisBlock()`，唯一生产生成器 |
| stored genesis | ✅ | `blocks.dat[0]`，由生成器派生 |
| handshake genesis | ✅ | 取自 `chain.BlockByHeight(0)` = **同一来源** |
| CLI genesis | ❌ | CLI 无独立创世定义 |
| config genesis | ❌ | 无配置项定义创世 |
| **hardcoded genesis（测试）** | ✅ | `cmd/node/verify_test.go:145` 硬断言 `00003d97…e4a3` |

### §6.3 判定

**生产侧：单一来源（SINGLE SOURCE）。** 未发现 `MULTIPLE IDENTITY SOURCES`。

**但存在一个结构性特征（drift 风险）**：身份是**派生量**而非**锚定常量**。
任何影响 `Subsidy(0)` / `MaxTargetBits` / `GenesisTimestamp` / `GenesisMinerPubKeyHash` 的改动
都会**静默改变**创世哈希与链身份 —— 这正是 F-1 的成因。

**测试侧存在一个独立的身份断言**（`verify_test.go:145`），它是仓库中**唯一**的创世哈希字面量。
F-3 的"genesis pin"本质上是把**已在测试中存在的锚定**提升到生产代码。

---

## §7 STARTUP STATE MACHINE AUDIT

### §7.1 实际启动序列（源码取证，`cmd/node/main.go`）

```
:76   AcquireDirLock(cfg.DataDir)              ← 进程独占锁（无身份语义）
:90   OpenFileBlockStore(cfg.DataDir)          ← 打开/创建 blocks.dat；可能自动截断 torn 尾
:96   NewBlockchainFromStore(store)            ← 创世读取 + 回放（见 §8）
:102  chain.Tip()
:108  log "本地区块链已就绪: 高度=%d 链尾=%s"
:112  wallet.LoadOrCreate(walletPath)          ← 若缺失则自动创建钱包（隐式初始化）
:123  mempool.New(...)
:130  chain.BlockByHeight(0) → genesis
:136  p2p.NewNode(..., genesis.Header.HashHex(), ...)
      → 控制接口启动
```

### §7.2 四个 Case 的当前行为

| Case | F-2 要求 | 当前行为 | 符合？ |
|---|---|---|---|
| **A** 空库 + **显式**初始化 | ALLOW（显式） | `Height()<0` ⇒ **自动** `NewGenesisBlock()` + `SaveBlock`；**无显式标志** | ⚠️ 隐式，非显式 |
| **B** 存在 + 创世匹配 | VERIFY → CONTINUE | 读盘创世 → `NewBlockchainWithGenesis` → 回放 | ✅（但无显式 VERIFY 步骤） |
| **C** 存在 + 创世不匹配 | **FAIL CLOSED** | 仅当磁盘 coinbase > 期望补贴时，**偶然**以 `ErrExcessiveCoinbase` 失败 | ⚠️ 方向依赖 + 错误语义错误 |
| **D** 身份损坏/不可用 | **FAIL CLOSED**，**不得当空库** | 不可读 ⇒ `ErrCorruptStore`（✓）；**但缺失/0 字节 ⇒ 当作空库 ⇒ 自动重建创世（✗）** | ❌ **违反** |

### §7.3 回放与身份检查的相对顺序

```
NewBlockchainFromStore:
    h := store.Height()
    if h < 0 { ... }                       ← Case A：自动初始化
    first := store.GetBlockByHeight(0)
    bc := NewBlockchainWithGenesis(first)  ← 创世 UTXO 初始化（偶然失败点）
    for i := 1..h { bc.applyBlock(b) }     ← 回放
```

**不存在独立的"身份检查"阶段。** 回放发生在创世 UTXO 初始化**之后**；
而那个偶然失败点**不是**身份检查，只是经济校验的副产物。

⇒ F-2 要求的"**数据目录身份检查必须早于历史区块 consensus replay**"当前**无法被评估为满足** —— 因为该阶段**根本不存在**。

---

## §8 `NewBlockchainFromStore` AUDIT

### §8.1 完整逻辑（`internal/blockchain/blockchain.go:217-259`）

```go
func NewBlockchainFromStore(store storage.BlockStore) (*Blockchain, error) {
    h, err := store.Height()
    if err != nil { return nil, fmt.Errorf("读取存储高度失败: %w", err) }

    if h < 0 {                                          // ← Case A 判定
        genesis := NewGenesisBlock()
        if err := store.SaveBlock(genesis); err != nil { // ← 自动落盘
            return nil, fmt.Errorf("创世区块落盘失败: %w", err)
        }
        bc, err := NewBlockchainWithGenesis(genesis)
        ...
        return bc, nil
    }

    first, err := store.GetBlockByHeight(0)
    if err != nil { return nil, fmt.Errorf("读取创世区块失败: %w", err) }
    bc, err := NewBlockchainWithGenesis(first)           // ← 无身份比较
    if err != nil { return nil, err }                    // ← Case C 在此偶然暴露
    bc.store = store
    for i := 1; i <= h; i++ { ... bc.applyBlock(b) ... } // ← 回放
    bc.rebuildTree()
    ...
}
```

### §8.2 九个问题的精确回答

| # | 问题 | 回答 | 证据 |
|---|---|---|---|
| 1 | 如何判断 datastore 是否存在？ | `store.Height()`；`h < 0` 视为空 | `blockchain.go:218/223`；`file.go:196` |
| 2 | 如何读取 genesis？ | `store.GetBlockByHeight(0)` | `blockchain.go:236` |
| 3 | 是否已存在 genesis identity check？ | **否** | 无任何比较；全仓非测试创世哈希字面量 = 0 |
| 4 | mismatch 会发生什么？ | 仅当磁盘 coinbase > 期望补贴时，在 `NewBlockchainWithGenesis` 内以 `ErrExcessiveCoinbase` 失败（包装为"创世区块 UTXO 初始化失败"）。**否则 mismatch 完全不被察觉** | `blockchain.go:240`；`apply.go` |
| 5 | corrupted identity 会发生什么？ | (a) 文件不可读/可检测损坏 ⇒ `ErrCorruptStore` 或"读取创世区块失败"（✓ FAIL CLOSED）；(b) **文件缺失/0 字节 ⇒ 当作空库 ⇒ 自动生成并落盘新创世（✗）** | `file.go:196`；`blockchain.go:223-227` |
| 6 | empty datastore 会发生什么？ | 自动 `NewGenesisBlock()` + `SaveBlock` —— **隐式**，无显式标志 | `blockchain.go:223-227` |
| 7 | replay 在 identity check 前还是后？ | **不存在 identity check**。回放在 `NewBlockchainWithGenesis` 之后 | `blockchain.go:240/245` |
| 8 | 错误是否会被上层吞掉？ | **不会**：`main.go:97` 包装后返回；`newNodeRuntime` 返回 error ⇒ 进程退出。但**错误文案误导** | `main.go:96-101` |
| 9 | CLI/RPC/P2P 是否可能绕过？ | **CLI 是**：`verify`（`cli.go:556`）走 `VerifyStoredChain` 独立路径。RPC/P2P/mining **不能**（纯消费者） | `cli.go:556`；`verify.go:50` |

### §8.3 方向依赖漏洞（重要）

mismatch 之所以"能被发现"，**纯粹因为**旧链 coinbase（50）> 新补贴（5）。
若某数据集创世的 coinbase **≤** 期望 `Subsidy(0)`，则 `NewBlockchainWithGenesis` **成功**，
**身份不匹配被完全忽略**，随后回放将在一棵**外来创世**的树上进行。

⇒ 当前行为**不是**身份校验，只是**经济校验的偶然副作用**。

---

## §9 FAIL-CLOSED AUDIT

逐项检索可能的 fallback：

| 行为 | 文件:函数:行 | 当前行为 | F-2 是否允许 | 判定 |
|---|---|---|---|---|
| reset | `cli.go:640 cmdReset` | **显式**命令 + 确认门控（`--force` 可跳过） | 允许（需显式授权） | ✅ 非自动回退 |
| recreate genesis | `blockchain.go:224` | `h<0` 时**自动**生成并落盘 | ❌ Case A 要求"显式" | ⚠️ 隐式 |
| delete | `cli.go:716/798 os.Remove` | 仅 `reset` 显式触发（`blocks.dat`/`wallet.json`/`node.lock`） | 允许（显式） | ✅ |
| overwrite | — | 无 | — | ✅ 不存在 |
| migrate | — | 无（`grep migrat` = 0） | FORBIDDEN | ✅ 不存在 |
| replay from genesis | `blockchain.go:245` | 存在（正常回放） | 允许（Case B） | ✅ |
| generate new genesis | `blockchain.go:224` | 见上 | ⚠️ | ⚠️ |
| **ignore error / continue startup** | — | 未发现 | FORBIDDEN | ✅ 不存在 |
| **auto repair（存储层）** | `v2.go:305 loadLog` | **仅**对"位于 EOF 的截断残片"且文件已含 magic/v2 帧时，**截断未提交尾部** | 未直接禁止（存储层语义，非身份） | ⚠️ 记录 |
| **corrupt-as-empty** | `file.go:196` | **0 字节/缺失 ⇒ `-1` ⇒ 当作空库 ⇒ 自动重建** | ❌ **违反 Case D** | ❌ **BLOCKER** |

**违反 F-2 Contract 的路径**：`corrupt-as-empty`（Case D → Case A 混淆）。
**需登记的自动行为**：存储层 torn-tail 截断修复（非身份相关，但属"auto repair"）。

---

## §10 P2P HANDSHAKE CONSISTENCY

### §10.1 GenesisHash 的来源

```
main.go:130   genesis, _ := chain.BlockByHeight(0)
main.go:136   p2p.NewNode(cfg.ListenAddr, nodeWallet.Address(), genesis.Header.HashHex(), svc)
node.go:219   func NewNode(listenAddr, nodeID, genesisHash string, handler Handler) *Node
node.go:697   GenesisHash: n.genesisHash          ← 出站握手填充
```

### §10.2 一致性判定

```
P2P GenesisHash  ==  Local Canonical Genesis Hash ?
```

**是 —— 二者同源**（均取自 `chain.BlockByHeight(0)`，而该创世来自 `blocks.dat[0]`）。
**未发现 P2P 与本地 canonical genesis 来自不同来源。**

### §10.3 关键缺口：字段被广播但从不校验

```
grep -rniE "\.GenesisHash\s*(==|!=)" internal/p2p/*.go cmd/node/*.go   （非测试）
    → >>> NO COMPARISON FOUND <<<
```

- 出站：`node.go:697` 填充 `GenesisHash`
- 入站：`node.go:779-790 dispatchInner` 解码 `HandshakePayload`，**只用 `ListenAddr` / `KnownPeers`**
- 消费者：`service.go:219 OnHandshake` 只用 `ChainHeight` / `ChainWork` / `TipHash`，**未使用 `GenesisHash`**

⇒ `genesis_hash` 字段的注释"**用于快速识别网络不一致**"描述的是**意图**，
**当前代码中不存在对应的校验行为**。

**严重性**：这是 F-2 §6 所设想的"创世哈希 = 网络身份锚点"的**未实现部分**。
**但 P2P 架构重设计属 F-3 §13 Scope Firewall 的 OUT OF SCOPE 项** ⇒ 仅登记，不提出实施。

---

## §11 STARTUP ORDERING

### §11.1 实际顺序

```
process start
    ↓
config（flag 解析）
    ↓
datastore open          main.go:90   （OpenFileBlockStore，可能自动截断 torn 尾）
    ↓
genesis identity        ✗ 不存在
    ↓
genesis verification    ✗ 不存在（仅经济校验偶然暴露）
    ↓
consensus replay        blockchain.go:245（在 NewBlockchainWithGenesis 之后）
    ↓
RPC                     （控制接口，main.go 之后）
    ↓
P2P                     main.go:136
    ↓
mining                  （由调用方决定，见 main.go:74 注释）
```

### §11.2 判定

**"Genesis identity verification 是否早于 consensus replay？"**

严格回答：**该阶段不存在，因此无法判定为"早于"。**

- 若把"创世 UTXO 初始化失败"当作事实上的检查点 ⇒ 它**确实**在回放之前（`blockchain.go:240` 先于 `:245`）。
- 但它是**经济校验的副产物**，且**方向依赖**（§8.3），**不是**身份校验。

⇒ 按 §7 要求标记：**`F-3 IMPLEMENTATION BLOCKER`** —— 缺少**显式的**、**在回放之前**的身份校验阶段。

---

## §12 ENTRY-POINT COVERAGE

### §12.1 全量入口枚举

| 入口 | 类型 | 是否构造链 | 路径 | 是否受身份契约约束 |
|---|---|---|---|---|
| `node`（无子命令） | 运行时 | ✅ | `main.go:96` → `NewBlockchainFromStore` | ❌ 无校验 |
| `node verify` | CLI | ✅ | `cli.go:556` → `VerifyStoredChain` → `NewBlockchainWithGenesis` | ❌ **旁路** |
| `node printchain` | CLI | ❌（仅打印） | `cli.go:468`（只读） | N/A |
| `node reset` | CLI | ❌（删除文件） | `cli.go:640` | N/A（显式） |
| `node status/balance/utxos/send/mine/stop/wallet` | CLI | ❌ | RPC 客户端 | N/A |
| RPC 控制接口 | HTTP | ❌ | `internal/control`（纯消费者） | N/A |
| P2P | 网络 | ❌ | `internal/p2p`（纯消费者） | N/A |
| mining | 内部 | ❌ | `internal/mempool` + `chain` | N/A |
| tests | 测试 | ✅ | 35 处 `NewBlockchainFromStore`（临时目录） | N/A（非生产） |

### §12.2 旁路识别

```
入口 A（node）    → 走 NewBlockchainFromStore        → 无校验（非"verified path"）
入口 B（verify）  → 走 VerifyStoredChain 独立路径     → BYPASS
入口 C            → 不存在
```

**BYPS 结论**：存在 **1 个旁路**（`verify`）。
RPC / P2P / mining / tests **不构成旁路**（不构造链，或仅测试用）。

⇒ F-2 的 Startup Identity Contract 若要生效，**必须同时覆盖 `NewBlockchainFromStore` 与 `VerifyStoredChain` 两条路径**。
当前**两条都没有**校验。

---

## §13 DATASET BOUNDARY

F-2 冻结边界：

```
NEW PROTOCOL ↔ NEW DATASET ↔ NEW GENESIS
```

### §13.1 当前代码是否尊重该边界？

| 检查项 | 当前行为 | 是否尊重边界 |
|---|---|---|
| legacy `blocks.dat` 自动读取 | **是** —— `NewBlockchainFromStore` 无条件读取任意目录的 `blocks.dat[0]` | ❌ |
| legacy replay | **是** —— 无条件进入回放循环 | ❌（F-1 即此后果） |
| automatic migration | 不存在 | ✅ |
| automatic upgrade | 不存在（`v2Mode` 仅在写入时进入，读 legacy 不升级） | ✅ |
| automatic deletion | 不存在（仅 `reset` 显式） | ✅ |
| automatic overwrite | 不存在 | ✅ |

### §13.2 判定

**新协议 ↔ 新数据集的边界未被尊重。** 新二进制会**无条件尝试加载并回放**任何数据目录
（包括 legacy 数据集），仅在"创世 coinbase 超过新补贴"时才失败 —— 且以**误导性经济错误**呈现。

**本审计未执行任何实际迁移。**

---

## §14 WALLET QUARANTINE STATUS

### §14.1 结构性检查（仅允许的范围）

| 项 | 值 |
|---|---|
| filename | `run-a/wallet.json`、`run-b/wallet.json` |
| size | 389 B each |
| schema keys | `version`, `d`, `x`, `y`, `pubkey` |
| version field | 存在（int） |
| **presence of private scalar** | **是** —— `d` 为 32 字节（64 hex）明文 P-256 标量 |
| file hash | `run-a` = `bf577525dcab4f2584812b357b82248435d5a8b85e84cf6cb8218c9d86844a94`<br>`run-b` = `762c369374117141fc18a2b687df7e81aed5b8a385f449a8a4a9128a6faa5190` |

### §14.2 合规声明

- ❌ **未输出** `d` / 任何私钥值
- ❌ **未导入**、**未签名**、**未使用**、**未复制**、**未修改**、**未接入新链**、**未生成新 wallet**
- ✅ 状态维持：`QUARANTINED / FORENSIC ARCHIVE ONLY`
- ✅ 本阶段**未重新读取** wallet 内容（沿用 F-2 已登记的结构事实）

### §14.3 附加观察（记录，非本阶段范围）

`cmd/node/main.go:112 wallet.LoadOrCreate(walletPath)` —— 若数据目录中 `wallet.json` 缺失，
节点会**自动创建**一个新钱包。这是一条**隐式初始化路径**（针对节点自身钱包），
与 §7 Case A 的"隐式初始化"同属一类。属 `DEFERRED / OUT OF SCOPE`（钱包工作）。

---

## §15 IMPLEMENTATION READINESS MATRIX

| Area | Current State | F-2 Contract | Gap | Severity | Required Action |
|---|---|---|---|---|---|
| **Genesis storage** | 仅隐式存于 `blocks.dat[0]`；无元数据 | 身份须可校验 | `NOT CURRENTLY PERSISTED AS EXPLICIT IDENTITY METADATA` | **P0** | 决策：身份如何被可靠取得/区分 |
| **Genesis verification** | **不存在** | EXPECTED vs ACTUAL → MATCH? | 全无校验 | **P0** | 引入 pin + 比较 |
| **Empty datastore** | `Height()<0` ⇒ 自动生成创世（隐式） | Case A 要求**显式**初始化 | 隐式 vs 显式 | **P1** | 引入显式初始化语义 |
| **Matching datastore** | 读盘 → 构造 → 回放（无显式 VERIFY） | Case B: VERIFY → CONTINUE | 缺显式 VERIFY 步骤 | **P1** | 补显式校验步骤 |
| **Mismatch datastore** | 仅方向依赖地偶然失败（`ErrExcessiveCoinbase`） | Case C: **FAIL CLOSED** + `GENESIS_MISMATCH` | 方向依赖 + 错误语义错误 | **P0** | 引入独立身份校验与错误 |
| **Corrupted identity** | 不可读⇒fail（✓）；**缺失/0字节⇒当空库⇒自动重建（✗）** | Case D: FAIL CLOSED，**不得当空库** | Case D→A 混淆 | **P0** | 区分"空"与"身份丢失" |
| **Replay ordering** | 无独立身份阶段；回放在构造之后 | 身份检查**必须早于** replay | 阶段不存在 | **P0** | 在 replay 前插入校验 |
| **Error taxonomy** | `GENESIS_MISMATCH` **不存在** | 需独立 sentinel | 缺失 | **P1** | `REQUIRED BY F-2 / NOT YET IMPLEMENTED` |
| **P2P handshake** | 字段发送+解码，**从不校验** | 创世哈希=网络身份锚点 | 意图未实现 | **P2** | 记录；**P2P 重设计 OUT OF SCOPE** |
| **CLI entry** | `verify` 独立旁路，无校验 | 全部入口受身份契约约束 | 1 个旁路 | **P1** | 统一 `verify` 路径 |
| **RPC entry** | 纯消费者，不构造链 | 无额外要求 | 无 | — | 无需动作 |
| **Mining entry** | 纯消费者，不构造链 | 无额外要求 | 无 | — | 无需动作 |
| **Legacy dataset** | 无条件自动读取 + 回放 | Runtime: FORBIDDEN | 边界未尊重 | **P0** | 运行时显式拒绝 legacy |

---

## §16 BLOCKERS / RISKS

### BLOCKERS（必须先解决）

| ID | Blocker | 依据 | 为何是 blocker |
|---|---|---|---|
| **B-1** | **身份元数据缺失 ⇒ Case A/D 不可区分** | §5.3；`file.go:196` | 存储层无法区分"空目录"与"身份丢失"。F-2 Case D 要求 FAIL CLOSED 且不得当空库 —— **无法在当前语义下实现**，须先作**语义决策** |
| **B-2** | **无任何 genesis identity check** | §8.2 Q3 | 契约的核心（EXPECTED vs ACTUAL）**完全不存在** |
| **B-3** | **mismatch 检测方向依赖** | §8.3 | 仅在"磁盘 coinbase > 期望补贴"时失败；反向 mismatch **静默通过** |
| **B-4** | **`verify` 独立旁路** | §12.2 | 契约须覆盖两条路径；`VerifyStoredChain` 注释明确"不新增创世侧校验规则" |
| **B-5** | **初始化是隐式的** | §7.2 Case A | F-2 要求 explicit initialization；当前无任何显式标志 |

### RISKS（记录，非阻断）

| ID | Risk | 说明 |
|---|---|---|
| R-1 | P2P `genesis_hash` 从不校验 | 意图未实现；**OUT OF SCOPE**（P2P 重设计） |
| R-2 | 存储层 torn-tail 自动截断修复 | `v2.go:305`；仅限 EOF 截断残片且文件含 magic/v2 帧；非身份相关 |
| R-3 | `wallet.LoadOrCreate` 隐式创建钱包 | `main.go:112`；**OUT OF SCOPE**（钱包工作） |
| R-4 | 身份为**派生量**而非锚定常量 | §6.3；未来任何共识参数改动会再次静默改变链身份 |
| R-5 | `verify` 与 `node` 的创世语义不一致 | 前者"视为可信输入"，后者"建立初始 UTXO"—— 两处对同一创世的态度不同 |

### 未发现的（明确记录，避免过度归因）

- ✅ 未发现 auto-migration / auto-upgrade / auto-convert
- ✅ 未发现 ignore-error / continue-startup
- ✅ 未发现 RPC / P2P / mining 侧独立构造入口
- ✅ 未发现 MULTIPLE IDENTITY SOURCES（生产侧）

---

## §17 SCOPE FIREWALL

以下内容**本阶段明确不处理**，如发现只登记 `DEFERRED / OUT OF SCOPE`：

| 项 | 状态 |
|---|---|
| MaximumSupply enforcement | `DEFERRED / OUT OF SCOPE` |
| subsidy redesign | `DEFERRED / OUT OF SCOPE` |
| mining fairness / reward model / fee policy | `DEFERRED / OUT OF SCOPE` |
| wallet implementation | `DEFERRED / OUT OF SCOPE`（§14.3 仅登记） |
| GUI | `DEFERRED / OUT OF SCOPE` |
| explorer | `DEFERRED / OUT OF SCOPE` |
| production deployment | `DEFERRED / OUT OF SCOPE` |
| Chain ID redesign | `DEFERRED / OUT OF SCOPE` |
| Protocol Version creation | `DEFERRED / OUT OF SCOPE` |
| **P2P architecture redesign** | `DEFERRED / OUT OF SCOPE`（§10.3 / R-1 仅登记） |
| consensus parameter changes | `DEFERRED / OUT OF SCOPE` |

**未顺手修复任何一项。**

---

## §18 FINAL VERDICT

```
READY WITH BLOCKERS
```

**判定理由**：

| 判据 | 状态 |
|---|---|
| 实施路径是否明确？ | ✅ 是 —— 需在 `NewBlockchainFromStore` 与 `VerifyStoredChain` 的创世读取处插入 pin 比较 |
| 是否存在必须先解决的明确 blocker？ | ✅ **是，5 个（B-1..B-5）** |
| 协议/存储/启动语义是否足以安全实施？ | ⚠️ **不足以直接实施** —— B-1（身份元数据缺失导致 Case A/D 不可区分）须先作语义决策 |

**为何不是 `READY`**：契约的核心（身份校验）**完全不存在**，且 mismatch 检测**方向依赖**、
初始化**隐式**、存在 **1 个旁路**、Case D **会被误判为空库并自动重建**。

**为何不是 `NOT READY`**：所需能力**不缺失基元** —— 存储层已有 `ErrCorruptStore` 等
分类错误、已有唯一构造入口、已有测试侧身份断言。B-1 虽是结构性语义缺口，
但可在 F-3 范围内通过**语义决策 + 显式实现**闭合，无需新的存储格式或协议层重设计。

**禁止以"看起来没问题"作为 READY 理由** —— 本判定**未**使用任何此类理由。

---

## §19 EVIDENCE INDEX

### §19.1 源码取证点

| 文件:行 | 内容 |
|---|---|
| `internal/blockchain/blockchain.go:217-259` | `NewBlockchainFromStore` 全文 |
| `internal/blockchain/blockchain.go:135-159` | `NewBlockchainWithGenesis`（"创世视为可信"注释） |
| `internal/blockchain/blockchain.go:183-193` | `NewBlockchainWithGenesisAndActivation`（仅测试） |
| `internal/blockchain/genesis.go:24-30` | `NewGenesisBlock()` 派生创世 |
| `internal/blockchain/verify.go:50-90` | `VerifyStoredChain`（:68 注释"不新增创世侧校验规则"） |
| `internal/storage/file.go:50-99` | `OpenFileBlockStore` / `...ReadOnly` |
| `internal/storage/file.go:196-203` | `Height()`（空 ⇒ -1） |
| `internal/storage/v2.go:305-345` | `loadLog`（torn-tail 自动截断修复） |
| `internal/storage/v2.go:396-400` | `v2Mode = true` 仅在文件已含 v2 帧时 |
| `internal/storage/datalock.go:61` | `AcquireDirLock`（纯进程锁） |
| `internal/p2p/node.go:93-101` | `HandshakePayload`（含 `GenesisHash`） |
| `internal/p2p/node.go:219-230` | `NewNode(...genesisHash...)` |
| `internal/p2p/node.go:693-706` | 出站握手填充 `GenesisHash` |
| `internal/p2p/node.go:779-790` | 入站握手解码（**不校验 GenesisHash**） |
| `cmd/node/service.go:219-237` | `OnHandshake`（**不使用 GenesisHash**） |
| `cmd/node/main.go:76-136` | 启动序列 |
| `cmd/node/cli.go:468` / `:556` | `printchain` / `verify` 存储打开 |
| `cmd/node/cli.go:640-760` | `cmdReset`（显式 + 确认门控） |
| `cmd/node/cli.go:612` | `resetDataFiles = {"blocks.dat","wallet.json"}` |
| `cmd/node/verify_test.go:145` | 唯一创世哈希字面量（测试） |
| `go.mod` | `module p2pchain`；`go 1.22` |

### §19.2 检索结论（否定性证据）

| 检索 | 结果 |
|---|---|
| `grep -rniE "chainid\|networkid\|protocolversion" --include=*.go`（非测试） | 0 命中 |
| `grep -rn "0000aca1\|00003d97" --include=*.go`（非测试） | **0 命中** |
| `grep -rn "GENESIS_MISMATCH\|ErrGenesisMismatch"` | **NOT FOUND** |
| `grep -rniE "\.GenesisHash\s*(==\|!=)" internal/p2p cmd/node` | **NO COMPARISON FOUND** |
| `grep -rniE "migrat\|upgrade\|convert" internal/ cmd/`（非测试） | 0 命中 |
| `grep -rniE "metadata\|sidecar\|manifest\|identity" internal/storage`（非测试） | 0 命中 |
| `NewBlockchainFromStore` 生产调用点 | **1**（`main.go:96`） |
| 生产链构造路径总数 | **2**（`node` / `verify`） |

### §19.3 受保护对象（未变）

| 对象 | SHA-256 |
|---|---|
| `node.exe` | `3972843aff90428dd67acb39c6aaa009bd04d42982603617c5dbf6f0e8c213e1` |
| `run-a/blocks.dat` | `893e64c12598558ff93793248d363536e1d090a6970c2d12ed632af3eea2fb2f` |
| `run-b/blocks.dat` | `5ab67bb4f2787fb11555135671379269818c3fc43fff20057a8743c4f4e1df53` |
| `run-a/wallet.json` | `bf577525dcab4f2584812b357b82248435d5a8b85e84cf6cb8218c9d86844a94` |
| `run-b/wallet.json` | `762c369374117141fc18a2b687df7e81aed5b8a385f449a8a4a9128a6faa5190` |
| `PHASE-F2-…-FREEZE.md` | `001c48655c7ff693809420240d254abdfa109f8272cf9c0282ccbc20023bbdb9` |
| `PHASE-F1-…-AUDIT.md` | `d44791da1ffc029ed1f1d8666f732e7705b73dfbb2f4d2c867d0b559074c7ded` |

### §19.4 Zero-Drift Proof

| 项 | 开工 | 收工 | 结论 |
|---|---|---|---|
| HEAD | `4d892be` | `4d892be` | 未动 |
| tracked diff | 7 | 7 | 未动 |
| `node.exe` / `blocks.dat` ×2 / `wallet.json` ×2 | 见 §19.3 | 同 | 未动 |
| 源码 / 常量 / Genesis | 未改 | 未改 | 未动 |
| Git 写操作 | 无 | 无 | 未执行 |
| 构建 / 新二进制 | 无 | 无 | 未执行 |
| 迁移 / 删除 / 挖矿 | 无 | 无 | 未执行 |
| wallet 内容读取 | 仅结构（沿用 F-2） | 同 | 合规 |

---

## §20 REPORT INTEGRITY / SHA-256

### §20.1 自引用说明

本报告为**自引用文档**，其 SHA-256 **不能自含**（写入摘要即改变摘要）。
最终摘要由独立命令在收工后计算，登记于**文件之外**（交付说明 + workspace memory）。

```
最终摘要 = sha256sum PHASE-F3-GENESIS-IDENTITY-STARTUP-READINESS-AUDIT.md
计算时点 = 收工（本报告最后一次写入之后）
可复现性 = 持有本文件者直接运行上述命令即可独立验证
```

### §20.2 上游产物摘要（可直接独立验证）

| 产物 | SHA-256 |
|---|---|
| `PHASE-F1-GENESIS-SUBSIDY-COMPATIBILITY-AUDIT.md` | `d44791da1ffc029ed1f1d8666f732e7705b73dfbb2f4d2c867d0b559074c7ded` |
| `PHASE-F2-PROTOCOL-IDENTITY-DECISION-FREEZE.md` | `001c48655c7ff693809420240d254abdfa109f8272cf9c0282ccbc20023bbdb9` |

### §20.3 基线指纹

| 常量 | 值 |
|---|---|
| HEAD COMMIT | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| NEW GENESIS HASH | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| OLD GENESIS HASH | `0000aca1af720da390380a38dc97d141cfb67afe578a2d31315860a5489db58c` |

---

## HARD STOP

本阶段到此终止。**未执行**任何：

- ❌ 修改代码 / commit / build
- ❌ 迁移 / 删除数据 / 挖矿
- ❌ 触碰生产环境 / 实施 F-3
- ❌ 进入钱包工作 / 进入 GUI 工作

**只有 Owner 单独授权后，才允许进入 F-3 Implementation Execution。**

---

## FINAL OUTPUT

```
PHASE F-3 STATUS:
BASELINE:                   PASS
VERDICT:                    READY WITH BLOCKERS
BLOCKERS:                   B-1 身份元数据缺失（Case A/D 不可区分）
                            B-2 无任何 genesis identity check
                            B-3 mismatch 检测方向依赖
                            B-4 verify 独立旁路
                            B-5 初始化隐式（非显式）
IMPLEMENTATION AUTHORIZED:  NO
HARD STOP:                  YES
```

---

*报告结束。本报告为只读实施就绪性审计产物，不构成任何实施授权。*
