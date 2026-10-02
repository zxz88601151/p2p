# PHASE F-2 — PROTOCOL IDENTITY DECISION & MIGRATION CONTRACT FREEZE

**阶段性质**：STRICT READ-ONLY / NO IMPLEMENTATION / OWNER DECISION CAPTURE / CONTRACT FREEZE
**前置阶段**：PHASE F-1（`F-1 CONFIRMED — PROTOCOL COMPATIBILITY ISSUE`，已 HARD STOP）
**当前 HEAD**：`4d892be355a38886a83c123faad915c9f3cd62e4`
**本阶段判定**：

```
PHASE F-2 STATUS

Protocol Route:          B — NEW CHAIN / NEW GENESIS
Legacy Data:             B-DATA-1 — FORENSIC ARCHIVE
Protocol Identity:       FROZEN
Genesis Identity:        FROZEN
Dataset Contract:        FROZEN
Startup Contract:        FROZEN
Private Key Policy:      QUARANTINED
Implementation:          NOT EXECUTED
F-3:                     READY FOR SEPARATE AUTHORIZATION
HARD STOP:               YES
```

---

## §1 EXECUTIVE SUMMARY

F-1 已确认 HEAD `4d892be` 通过修改 `subsidyInitial (50→5)` 与 `subsidyHalvingInterval (210→5_250_000)`
**事实上定义了一个新的协议身份**；旧数据目录（`run-a`/`run-b`）属于旧身份，
而程序**没有任何机制**识别这一身份差异 —— 旧目录被新二进制打开时只给出**误导性的经济共识错误**
（`ErrExcessiveCoinbase`），而非明确的身份错误。

F-2 完成四件事：

1. **VERIFY** —— 复核 F-1 基线未被扰动（`BASELINE PASS`），并新增 `wallet.json` 为受保护对象；
2. **CAPTURE** —— 捕获项目方明确决策：**Route B（新链 / 新 Genesis）** + **B-DATA-1（旧数据取证归档）**；
3. **FREEZE** —— 冻结 7 项契约：Protocol Identity / Genesis Identity / Dataset / Startup /
   Private Key Quarantine / Future Consensus Change / GUI Policy；
4. **BOUNDARY** —— 划定 F-3 实施边界（不扩大至钱包 / GUI / 经济模型 / 生产部署）。

**本阶段 0 行代码改动、0 数据改动、0 Git 写操作、0 进程启停。**

**核心设计原则（本阶段确立）**：

> **数据目录身份检查必须早于历史区块 consensus replay。**

**F-2 定位的根因（承 F-1）**：授权链（`A-2.3-E` §10 D5-P0-4、`A-2.3-H` §L-1）
早已把"**必须使用全新空 datadir**"列为**硬约束**，却**从未将其实现为任何运行时机制**。
契约层修复目标语义：**`GENESIS_MISMATCH` / FAIL CLOSED**。

---

## §2 F-1 EVIDENCE REFERENCE

本阶段全部结论锚定于 F-1 报告及其证据集，不重新取证、不重新解释。

### §2.1 F-1 报告

| 项 | 值 |
|---|---|
| 路径 | `E:/wakuang/p2pchain/PHASE-F1-GENESIS-SUBSIDY-COMPATIBILITY-AUDIT.md` |
| 大小 | 28,036 B（2026-09-27 10:22） |
| SHA-256 | `d44791da1ffc029ed1f1d8666f732e7705b73dfbb2f4d2c867d0b559074c7ded` |
| 结论 | `F-1 CONFIRMED — PROTOCOL COMPATIBILITY ISSUE` |

### §2.2 F-1 关键事实（本阶段直接引用）

| # | 事实 | F-1 依据 |
|---|---|---|
| E-1 | HEAD = `4d892be355a38886a83c123faad915c9f3cd62e4` | §2.1 |
| E-2 | 新 Genesis = `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` | §5 / E-9 |
| E-3 | 旧 Genesis = `0000aca1af720da390380a38dc97d141cfb67afe578a2d31315860a5489db58c` | §3.2 |
| E-4 | `subsidyInitial: 50 → 5`；`subsidyHalvingInterval: 210 → 5_250_000` | §7 |
| E-5 | `run-a` 旧规则匹配 **1260/1260**，新规则匹配 **0/1260**，首个分歧高度 **0** | §6 |
| E-6 | `run-b` 旧规则匹配 **401/401**，新规则匹配 **0/401**，首个分歧高度 **0** | §6 |
| E-7 | 受保护 `node.exe`（`3972843a…c213e1`）对两目录 **PASS**；HEAD 二进制 **FAIL** | §3.3 |
| E-8 | 失败语义 = `创世区块 UTXO 初始化失败: coinbase 输出超过区块奖励加手续费上限: coinbase 输出 50 > 奖励 5 + 手续费 0` | §3.2 |
| E-9 | 创世非不可变常量；无 hash pin；运行时派生 | §5 |
| E-10 | `NO ACTIVATION GATE FOUND`（就 subsidy 而言） | §8 |
| E-11 | GUI 零经济硬编码 | §10 |
| E-12 | `MaximumSupply` 常量在源码中**不存在**；enforcement = NONE | §7 |

### §2.3 F-1 与 F-2 的连续性

F-1 的 §12「Recommended Next Decision Gate」明确要求下一阶段**首先执行 OWNER PROTOCOL DECISION**。
F-2 即该决策门，并已在本阶段闭合（§3）。

---

## §3 OWNER PROTOCOL DECISION

**项目方于本阶段明确确认（决策捕获，非 AI 代选）：**

```
Protocol Route:
    B — 正式切换新链 / 新 Genesis

旧 run-a / run-b 不再属于当前协议身份。
旧数据必须保留为 forensic archive。
旧 wallet.json 中的明文 P-256 私钥不得用于新链。
```

### §3.1 决策的技术含义（转述，不含评价）

| 维度 | Model B 下的技术影响 |
|---|---|
| 技术影响 | 接受 `5 / 5_250_000` 与新创世 `00003d97…e4a3` 为唯一权威；旧链数据被有意废弃 |
| 实施成本 | 参数已是目标值；需**补一层启动期身份校验**（当前缺失） |
| 兼容性 | `run-a`/`run-b` 永久不可验证（**设计意图**）；新链从新创世重新开始 |
| 数据处理 | 旧数据按 B-DATA-1 保留为取证归档（只读） |
| 长期维护 | 单一 consensus 语义（无高度分支、无双语义并存） |
| 安全边界 | 需新增 FAIL-CLOSED 身份校验；旧密钥须隔离 |

### §3.2 与既有授权链的一致性

```
Decision        : A-2.3-E §11  Chosen Path = R（新链 / 新 Genesis），2026-09-22
Authorization   : A-2.3-F-R4 §5.2  Q1–Q5 CONFIRMED / Q6 AUTHORIZED
                  （Q5 = subsidyInitial 5 / subsidyHalvingInterval 5,250,000）
Implementation  : A-2.3-G2 → HEAD 4d892be（apply.go 两常量）
Observed        : A-2.3-H §L-1  COND-A23F-2..6 仍 OPEN，含 "fresh empty datadir" 部署规格
Unresolved      : 该硬约束从未落地为运行时机制 ⇒ 无 GENESIS_MISMATCH 检测
```

**Model B 与 `Path R` 方向一致，与 Q5 冻结参数一致。** 本阶段**不重新解释**任何已确定的 Owner Decision。

### §3.3 治理可追溯性观察（记录，不改判）

共识权威常量改动落在提交 `4d892be`（提交信息 `test: harden economic consensus edge cases`，
`test:` 前缀）中，而非 `feat!` / `BREAKING CHANGE` 提交。该改动**有正式授权**，
故**不构成越权**；但提交语义未标注破坏性，对后续读者构成可追溯性风险。
**该风险的处理方式见 §11（Future Consensus Change Contract）。**

---

## §4 LEGACY DATA DECISION

**项目方于本阶段明确确认（决策捕获，非 AI 代选）：**

```
Legacy Data Policy:
    B-DATA-1 — 保留为 forensic archive
```

| 项 | 内容 |
|---|---|
| 策略 | **B-DATA-1 — 保留为取证归档** |
| 允许 | 原地保留、只读引用 |
| 禁止 | 移动、重命名、删除、修改、清理、覆盖 |
| 授权状态 | **未经后续单独授权不得删除** |
| 双重身份 | (a) F-1 原始取证证据；(b) 旧链协议身份的唯一现存样本 |

**AI 未自动删除任何内容。** 同样**禁止自动清理**：`node.exe`、audit binary、lab artifacts、
`blocks.dat`、keys、reports —— 本阶段均**未触碰**。

---

## §5 FROZEN PROTOCOL IDENTITY

### §5.1 PROTOCOL_IDENTITY_V1（冻结）

> 所有数值**来自源码读取**。**不存在**的定义一律标记 `NOT DEFINED IN SOURCE`，
> **不得自行创造不存在的版本号**（本阶段严格遵守）。

```
PROTOCOL_IDENTITY_V1

Protocol Name:               p2pchain                    [go.mod: module p2pchain]
Protocol / Consensus Version: NOT DEFINED IN SOURCE       ← 源码中无此概念
Chain ID (显式):              NOT DEFINED IN SOURCE       ← 源码中无此概念
Chain ID (事实/网络标识):      创世哈希（P2P 握手域）
Genesis Hash:                00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3
Genesis Coinbase:            5
Subsidy Initial:             5
Subsidy Halving Interval:    5,250,000
Subsidy 归零高度:             15,750,000   （最后非零 15,749,999）
Maximum Supply (派生):        42,000,000
MaximumSupply Enforcement:   NONE                        ← 源码常量不存在
```

### §5.2 完整参数表（源码级）

| 项 | 值 | 来源 |
|---|---|---|
| `subsidyInitial` | `5` | `internal/utxo/apply.go` |
| `subsidyHalvingInterval` | `5_250_000` | `internal/utxo/apply.go` |
| `CoinbaseMaturity` | `10` | `internal/utxo/apply.go` |
| `MaxTargetBits` | `16` | `internal/pow/pow.go` |
| `TargetBlockTimeSeconds` | `60` | `internal/pow/pow.go` |
| `DifficultyAdjustmentInterval` | `20` | `internal/pow/pow.go` |
| `MaxDifficultyBits` | `32` | `internal/pow/pow.go` |
| `ActivationHeight` | `2000` | `internal/pow/pow.go` |
| `LegacyBlockVersion` | `1` | `internal/pow/pow.go` |
| `NewBlockVersion` | `2` | `internal/pow/pow.go` |
| `GenesisTimestamp` | `1700000000` | `internal/blockchain/genesis.go` |
| `GenesisMinerPubKeyHash` | `19×0x00 + 0x01` | `internal/blockchain/genesis.go` |
| `frameVersion` / `frameMagic` | `2` / `PCC2` | `internal/storage/frame.go` |
| P2P handshake field | `genesis_hash`（"用于快速识别网络不一致"） | `internal/p2p/node.go` |

### §5.3 激活参数范围（重要澄清）

`ActivationHeight = 2000` **仅**覆盖：难度浮动 + 时间戳/MTP 规则 + 区块版本强制。
**不覆盖 subsidy** —— F-1 §8 已确证 `NO ACTIVATION GATE FOUND`（就 subsidy 而言）。
契约明确登记此范围，防止后续误以为"把补贴挂到 2000 就能兼容历史"。

### §5.4 Maximum Supply 语义（冻结）

```
MaximumSupply = 42,000,000
Role          = read-only derived invariant / accounting reference
Enforcement   = NONE
```

推导（可独立复算）：
`5×5,250,000 + 2×5,250,000 + 1×5,250,000 = 26,250,000 + 10,500,000 + 5,250,000 = 42,000,000`
与 `A-2.3-F-R4` §10 冻结值一致。**不得假装它当前是源码 enforcement 常量。**

---

## §6 GENESIS IDENTITY CONTRACT

### §6.1 新 Genesis 的地位

新 Genesis `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3`
被定义为 **protocol identity anchor**。

### §6.2 冻结的身份校验流程（设计，本阶段不实现）

```
EXPECTED_GENESIS_HASH
        ↓
ACTUAL_DATASET_GENESIS_HASH
        ↓
     MATCH?
   YES → continue
   NO  → FAIL CLOSED
```

### §6.3 目标错误语义

```
GENESIS_MISMATCH
```

或项目现有错误体系中等价的、明确表示
`DATASET_NOT_COMPATIBLE_WITH_PROTOCOL` 的语义。

**现有错误体系参照**（`Err*` sentinel 约定）：
`ErrDatadirLocked`（进程冲突）、`ErrExcessiveCoinbase`（经济校验）、
`ErrInvalidVersion`（版本）、`ErrUnsupportedRecordVersion`（记录版本）。
契约要求新增一个**独立** sentinel（如 `ErrGenesisMismatch`），与上述**并列而不混用**。

### §6.4 禁止路径

```
禁止继续进入：  ErrExcessiveCoinbase
                这种后续验证错误
```

### §6.5 核心原则（冻结）

> **数据目录身份检查必须早于历史区块 consensus replay。**

### §6.6 新 Genesis 完整身份（来自 A-2.3-E，交叉验证通过）

| 项 | 值 |
|---|---|
| MerkleRoot | `4d86b378…28ff`（A-2.3-E §D1 记录） |
| Nonce | `5390` |
| Hash | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| 旧创世对照 | MerkleRoot `41a8b126…bf3e` / Nonce `230970` / Hash `0000aca1…b58c` |

**交叉验证**：F-1 实测探针（`F-1-lab/evidence/head-genesis-probe.log`）产出的新创世哈希
与 A-2.3-E §D1 的**预测值逐字符一致** —— 独立路径互相印证。

---

## §7 DATASET IDENTITY CONTRACT

### §7.1 有效数据

```
NEW PROTOCOL
    ↓
NEW DATASET
    ↓
NEW GENESIS
```

### §7.2 旧数据

```
run-a
run-b
    ↓
LEGACY PROTOCOL
    ↓
FORENSIC ARCHIVE
```

### §7.3 冻结禁令

```
Runtime:              FORBIDDEN
Migration:            FORBIDDEN
Automatic Upgrade:    FORBIDDEN
Automatic Deletion:   FORBIDDEN
```

未来如需处理旧数据：**必须单独授权。**

### §7.4 数据集身份事实表

| 项 | 值 |
|---|---|
| Valid Genesis | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| Valid Chain 判据 | 块高 0 哈希 == Valid Genesis |
| Legacy Dataset A | `run-a/` 高度 0..1259，链上创世 `0000aca1…b58c` |
| Legacy Dataset B | `run-b/` 高度 0..400，链上创世 `0000aca1…b58c` |
| Git 可见性 | `.gitignore:22-23` 忽略 `/run-a/`、`/run-b/` |

---

## §8 STARTUP CONTRACT

未来节点启动行为（冻结）：

### Case A

```
empty/new dataset
    +
explicit new-chain initialization
```
→ **允许**初始化新的 Genesis。

### Case B

```
existing dataset
    +
Genesis == expected Genesis
```
→ **允许**进入正常验证。

### Case C

```
existing dataset
    +
Genesis != expected Genesis
```
→ **必须 FAIL CLOSED**。
不得：自动迁移 / 自动重置 / 自动删除 / 自动覆盖 / 自动重新生成 Genesis / 尝试继续 replay。

### Case D

```
existing dataset
    +
Genesis identity unavailable / corrupt
```
→ **同样 FAIL CLOSED**。

### §8.1 当前启动序列（只读证据，本阶段未改）

```
AcquireDirLock(cfg.DataDir)          cmd/node/main.go:76   ← 纯进程锁，零身份语义
OpenFileBlockStore(cfg.DataDir)      main.go:90
NewBlockchainFromStore(store)        main.go:96 → blockchain.go:217
    h = store.Height()
    if h < 0:                        ← Case A（空库）
        genesis = NewGenesisBlock(); SaveBlock(genesis)
        NewBlockchainWithGenesis(genesis)
    else:                            ← Case B/C/D 未区分
        first = store.GetBlockByHeight(0)     ← 读盘即用，不比较
        NewBlockchainWithGenesis(first)       ← 在此抛误导性错误
```

**缺口**：`else` 分支**未区分** Case B / C / D —— 这正是契约要求 F-3 填补之处。

---

## §9 LEGACY ARCHIVE CONTRACT

| 项 | 内容 |
|---|---|
| 归档对象 | `run-a/`（0..1259）、`run-b/`（0..400），含 `blocks.dat` / `wallet.json` / `node.lock` |
| 归档模式 | 原地保留（in-place），只读引用 |
| 允许操作 | 读取（审计/取证） |
| 禁止操作 | 移动、重命名、删除、修改、覆盖、清理 |
| 授权状态 | 未经后续单独授权不得删除 |
| 与 Valid Chain 关系 | **不属于** Valid Chain；不得被任何运行时路径加载为当前链 |
| 运行时可见性 | Runtime: **FORBIDDEN** |

**归档的价值**：既是 F-1 原始取证证据，也是旧链协议身份的**唯一现存样本**。
销毁将同时丧失两者 ⇒ 默认保留。

---

## §10 PRIVATE KEY QUARANTINE CONTRACT

### §10.1 现状（只读证据）

`run-a/wallet.json` 与 `run-b/wallet.json`（各 389 B）为 JSON，键为：
`version`、`d`、`x`、`y`、`pubkey`。
其中 **`d` 为 32 字节明文 P-256 私钥标量**（64 hex 字符，**本报告不输出其值**）。

### §10.2 冻结状态

```
PRIVATE KEY STATUS:
    QUARANTINED / FORENSIC ARCHIVE ONLY
```

### §10.3 冻结禁令

- ❌ 新链导入
- ❌ 新链签名
- ❌ 生产使用
- ❌ 复制
- ❌ 上传
- ❌ Git tracking
- ❌ 日志输出
- ❌ 报告中输出完整私钥

### §10.4 登记的未来专项

```
WALLET / KEY MATERIAL DISPOSITION AUDIT
```

该专项**单独授权**。**本阶段不得销毁私钥文件。**

### §10.5 本阶段的合规行为

本阶段仅读取 `wallet.json` 的**键名**（值已脱敏为长度），并计算其 **SHA-256**（用于只读证明）。
**未输出、未复制、未上传任何私钥材料。**

---

## §11 FUTURE CONSENSUS CHANGE CONTRACT

> **这是本次 F-1 最重要的长期补强。**

### §11.1 问题

F-1 的成因是：**只修改一个 `.go` 常量然后直接运行旧数据**。当前流程对此**无任何门禁**。

### §11.2 冻结的强制流程

未来任何修改下列任一项，**不得**只改常量即运行旧数据：

- Subsidy
- Halving interval
- Coinbase rules
- Difficulty
- Timestamp rules
- Block version
- Transaction validation
- Consensus constants

必须经过：

```
CONSENSUS CHANGE
        ↓
COMPATIBILITY IMPACT AUDIT
        ↓
PROTOCOL IDENTITY DECISION
        ↓
GENESIS / ACTIVATION CONTRACT
        ↓
OWNER AUTHORIZATION
        ↓
IMPLEMENTATION
        ↓
REPLAY TEST
        ↓
RELEASE
```

### §11.3 契约效力

本流程**正式写入 F-2 Contract**，作为未来共识变更的**强制门禁**。
其目的：确保任何共识变更都**显式地**回答"链身份是否改变"，并**显式地**处理历史数据兼容性，
而非像 F-1 那样由运行时以误导性错误暴露。

**对 §3.3 治理可追溯性风险的回应**：该流程的第 3 步（PROTOCOL IDENTITY DECISION）
与第 4 步（GENESIS / ACTIVATION CONTRACT）正是 F-1 缺失的环节。

---

## §12 GUI POLICY

F-1 已确认 GUI：

```
NO HARD-CODED ECONOMIC CONSENSUS
```

**因此本阶段不修改 GUI**（已遵守：`gui/p2pchain_studio/` 零改动）。

**只登记未来可考虑展示**：

| 建议展示项 | 理由 |
|---|---|
| Chain ID | 用户可确认连的是哪条链 |
| Protocol Version | 升级可见性 |
| Genesis Hash | 链身份唯一可信指纹 |
| Network Identity | 多网络共存时的区分 |

**实现约束（若未来实现）**：必须读取节点权威 `/status`，
**不得 GUI 自己计算共识参数**。

---

## §13 F-3 SCOPE

### §13.1 阶段定义

```
PHASE F-3 — PROTOCOL IDENTITY IMPLEMENTATION
```

**F-3 只能在单独授权后开始。** 本阶段不得实施 F-3 的任何内容。

### §13.2 F-3 候选范围（9 项）

1. **Genesis identity pin** —— 在生产代码固化 `00003d97…e4a3` 为期望创世
2. **Startup dataset identity check** —— 启动期比较磁盘创世与期望创世
3. **Explicit genesis mismatch error** —— 新增独立 sentinel（如 `ErrGenesisMismatch`）
4. **Fail-closed behavior** —— 不匹配即拒绝启动，不进入 replay
5. **New-chain initialization contract** —— 空 datadir 生成新创世的正式契约
6. **Legacy dataset rejection** —— 旧数据集在运行时被显式拒绝（而非误报经济错误）
7. **Regression tests** —— 覆盖"错误创世 ⇒ `GENESIS_MISMATCH`"而非 `ErrExcessiveCoinbase`
8. **Protocol identity RPC/status exposure** —— `/status` 暴露 `genesis_hash`（+ 可选 version/chain_id）
9. **Compatibility replay tests** —— 空库/非空库/损坏库三分支 + 新旧身份判定

### §13.3 F-3 边界约束（禁止扩大）

```
F-3 不得自动扩大为：
    ❌ 钱包阶段
    ❌ GUI 阶段
    ❌ 经济模型阶段
    ❌ 生产部署阶段
```

后续阶段（示意，名称可依审计结果调整）：
`F-3 → F-4（NEW CHAIN CONTROLLED INITIALIZATION）→ F-5（REAL NODE VALIDATION）`

---

## §14 FORBIDDEN ACTIONS

本阶段（及 HARD STOP 之后，直至 F-3 单独授权）**明确禁止**：

**代码层**
- ❌ 修改源码 / 修改 subsidy / 修改 genesis / 修改 validation
- ❌ 修改启动路径 / 添加 activation gate / 修改 GUI

**数据层**
- ❌ 修改 / 删除 `run-a` / `run-b`
- ❌ 修改 `blocks.dat` / 删除 `wallet.json`
- ❌ 初始化新链 / 启动生产节点 / 修改生产数据

**产物层**
- ❌ 重建 / 覆盖 `node.exe`
- ❌ 清理 audit binary / lab artifacts / reports

**Git 层**
- ❌ `commit` / `push` / `tag` / `merge` / `rebase` / `reset`

**协议层**
- ❌ 任何真实链迁移 / cutover / 部署

---

## §15 EVIDENCE MANIFEST

### §15.1 本阶段产物

| # | 文件 | 说明 |
|---|---|---|
| F2-1 | `PHASE-F2-PROTOCOL-IDENTITY-DECISION-FREEZE.md` | 本报告（唯一新增交付物） |

**本阶段未新增任何其它文件；未修改任何既有文件。**

### §15.2 受保护对象 SHA-256（开工与收工一致）

| 对象 | SHA-256 |
|---|---|
| `node.exe` | `3972843aff90428dd67acb39c6aaa009bd04d42982603617c5dbf6f0e8c213e1` |
| `run-a/blocks.dat` | `893e64c12598558ff93793248d363536e1d090a6970c2d12ed632af3eea2fb2f` |
| `run-b/blocks.dat` | `5ab67bb4f2787fb11555135671379269818c3fc43fff20057a8743c4f4e1df53` |
| `run-a/wallet.json` | `bf577525dcab4f2584812b357b82248435d5a8b85e84cf6cb8218c9d86844a94` |
| `run-b/wallet.json` | `762c369374117141fc18a2b687df7e81aed5b8a385f449a8a4a9128a6faa5190` |
| `run-a/node.lock` | `775a86ce9abd81a513b0cb820deba93351131dae1c8de1a6c6c6191075b62238` |
| `run-b/node.lock` | `277d3781c5195704809e4ed89d8223d7a80fc2c28da7313107e60674761bc3de` |
| `F-1-lab/node-head.exe` | `5b41ff9e0be5bd2b34c0fe94b3a65b770a2feb668e5d467db2cb72a1dde6894c` |
| `PHASE-F1-…-AUDIT.md` | `d44791da1ffc029ed1f1d8666f732e7705b73dfbb2f4d2c867d0b559074c7ded` |

### §15.3 源码基线（只读引用，未修改）

| 文件 | 引用内容 |
|---|---|
| `internal/utxo/apply.go` | `subsidyInitial=5`、`subsidyHalvingInterval=5_250_000`、`Subsidy()`、`CoinbaseMaturity=10`、`ErrExcessiveCoinbase` |
| `internal/blockchain/genesis.go` | `NewGenesisBlock()`、`GenesisTimestamp`、`GenesisMinerPubKeyHash` |
| `internal/blockchain/blockchain.go` | `NewBlockchainFromStore`(:217)、`NewBlockchainWithGenesis`(:137)、`ErrInvalidVersion` |
| `internal/pow/pow.go` | `MaxTargetBits=16`、`TargetBlockTimeSeconds=60`、`DifficultyAdjustmentInterval=20`、`MaxDifficultyBits=32`、`ActivationHeight=2000`、`LegacyBlockVersion=1`、`NewBlockVersion=2` |
| `internal/storage/datalock.go` | `AcquireDirLock`(:61)、`ErrDatadirLocked`（纯进程锁） |
| `internal/storage/frame.go` | `frameVersion=2`、`frameMagic="PCC2"` |
| `internal/p2p/node.go` | `NewNode(listenAddr, nodeID, genesisHash, handler)`(:219)、`GenesisHash json:"genesis_hash"`(:97) |
| `cmd/node/main.go` | `:76/:90/:96` 启动序列；`:136` 创世哈希作 P2P 握手域 |
| `cmd/node/verify_test.go` | `:145` 创世哈希硬断言（**测试**，非生产 pin） |
| `go.mod` | `module p2pchain`；`go 1.22` |

### §15.4 治理文档引用

`A-2.3-E`（Chosen Path = R）、`A-2.3-F-R4`（Q1–Q6）、`A-2.3-G2`（C1 实施）、
`A-2.3-H`（post-commit 审计，`COND-A23F-2..6` OPEN）、`F-1`（前置报告）。

### §15.5 Git 基线

```
HEAD      = 4d892be355a38886a83c123faad915c9f3cd62e4
parent    = 060333f  fix: harden UTXO consensus arithmetic against uint64 overflow
branch    = main
tracked   = 7 项修改（与 F-1 逐字一致）
untracked = 178 项 + 本报告
```

---

## §16 SHA-256

### §16.1 本报告

| 项 | 值 |
|---|---|
| 文件 | `E:/wakuang/p2pchain/PHASE-F2-PROTOCOL-IDENTITY-DECISION-FREEZE.md` |
| SHA-256 | 见 §16.3（报告自哈希不可自含，故由独立命令在收工后计算并登记） |

### §16.2 关键身份常量（指纹）

| 常量 | 值 |
|---|---|
| NEW GENESIS HASH | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| OLD GENESIS HASH | `0000aca1af720da390380a38dc97d141cfb67afe578a2d31315860a5489db58c` |
| HEAD COMMIT | `4d892be355a38886a83c123faad915c9f3cd62e4` |

### §16.3 收工后独立计算的 SHA-256 登记

```
PHASE-F2-PROTOCOL-IDENTITY-DECISION-FREEZE.md
    首版（16 节前身）    = 9cd837b36651db689d373e8494667bc6b448137c7779ab9840113bfb4f67225b
    本修订版（16 节契约） = 见 §16.5（自引用文档不能自含摘要）
```

### §16.4 Zero-Drift Proof

| 项 | 开工 | 收工 | 结论 |
|---|---|---|---|
| HEAD | `4d892be` | `4d892be` | 未动 |
| tracked diff | 7 | 7 | 未动 |
| `node.exe` SHA256 | `3972843a…c213e1` | 同 | 未动 |
| `run-a/blocks.dat` | `893e64c1…ea2fb2f` | 同 | 未动 |
| `run-b/blocks.dat` | `5ab67bb4…e1df53` | 同 | 未动 |
| `run-a/wallet.json` | `bf577525…6844a94` | 同 | 未动 |
| `run-b/wallet.json` | `762c3693…aa5190` | 同 | 未动 |
| GUI | 未改 | 未改 | 未动 |
| 源码 / 常量 / Genesis | 未改 | 未改 | 未动 |
| Git 写操作 | 无 | 无 | 未执行 |
| 进程启动/终止 | 无 | 无 | 未执行 |
| 构建 / 新二进制 | 无 | 无 | 未执行 |
| 删除 / 迁移 | 无 | 无 | 未执行 |

**本阶段 0 行代码改动。**

### §16.5 最终交付摘要（自引用说明）

本报告为**自引用文档**，其 SHA-256 **不能自含**（写入摘要即改变摘要）。
故采用如下约定：

```
最终摘要 = sha256sum PHASE-F2-PROTOCOL-IDENTITY-DECISION-FREEZE.md
计算时点 = 收工（本报告最后一次写入之后）
登记位置 = 交付说明（聊天回复）+ workspace memory 2026-09-27.md
可复现性 = 任何持有本文件者直接运行上述命令即可独立验证
```

**为何不嵌入本文件**：若将摘要写入正文，则写入动作本身改变文件内容 ⇒ 摘要立即失效。
将其登记在**文件之外**（交付说明 + memory）是唯一可被独立复现且不自相矛盾的方案。

**首版摘要**（`9cd837b3…f67225b`）为**另一份历史产物**，可直接 `sha256sum` 独立验证。

---

## HARD STOP

本阶段到此终止。

即使 Owner 已确认 **B**：

- ❌ 不改 subsidy
- ❌ 不改 genesis
- ❌ 不改 validation
- ❌ 不改 startup
- ❌ 不删除旧数据
- ❌ 不销毁私钥
- ❌ 不重建 binary
- ❌ 不初始化新链
- ❌ 不 commit

**必须等 F-3 单独授权。**

---

## FINAL OUTPUT

```
PHASE F-2 STATUS

Protocol Route:          B — NEW CHAIN / NEW GENESIS
Legacy Data:             B-DATA-1 — FORENSIC ARCHIVE
Protocol Identity:       FROZEN
Genesis Identity:        FROZEN
Dataset Contract:        FROZEN
Startup Contract:        FROZEN
Private Key Policy:      QUARANTINED
Implementation:          NOT EXECUTED
F-3:                     READY FOR SEPARATE AUTHORIZATION
HARD STOP:               YES
```

---

*报告结束。本报告为只读取证 + 决策捕获 + 契约冻结产物，不构成任何实施授权。*
*Owner Decision 已被记录并冻结为契约，**不得**被自动转换为代码修改。*
