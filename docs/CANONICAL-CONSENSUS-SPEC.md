# P2PChain — CANONICAL CONSENSUS SPEC

> **本文是 P2PChain 共识规则的规范描述（CANONICAL SPECIFICATION），描述当前可执行源码的行为。**
> 本文**不是**真值源：最高真值永远是**可执行实现 + 可重复测试**（见 §0）。任何与本文冲突的文档（含 README、历史阶段报告）应以可执行实现为准，并登记为 `DOCUMENTATION DRIFT`；本文随实现漂移同步修订。

- **建立/治理修正阶段**：初稿由第二写入者于 2026-10-01 01:10–01:12 落盘（未提交）；经 `DOCUMENT AUTHORITY RECONCILIATION-1` 审计发现权威缺陷；`DOCUMENT AUTHORITY GOVERNANCE CLOSURE-1` 治理修正（删除"唯一真值源"歧义、明确 L0 终审、修正来源声明）。
- **适用基线**：HEAD `434f8c7c6cc39585c8c31deb35e33b27bef1edb2` ／ tree `70de68b0d128914dde15a01b199f0590795cffe5` ／ 72 commits
- **取证方式**：现场读取源码 + 全量测试实测（`scripts/run-tests.sh` → RC=0，16 包 ok）
- **状态**：**CANONICAL / CURRENT**

---

## §0 真值优先级（CANONICAL TRUTH PRINCIPLE）

当不同层级的陈述冲突时，**不得"平均解释"**，严格按以下优先级取最高者：

| 优先级 | 层 | 说明 |
|---|---|---|
| 1 | **可执行源码行为** | 最高真值 |
| 2 | **可重复测试结果** | 次高，用于锁定源码语义 |
| 3 | 当前协议 / 序列化实现 | |
| 4 | 当前运行实测 | |
| 5 | 当前阶段报告 | |
| 6 | README / 项目介绍 | |
| 7 | 历史说明 | |

⇒ **`SOURCE OF TRUTH = 实际实现`**。低层文档与高层事实冲突时，登记 `DOCUMENTATION DRIFT` 并修正文档，**不改代码来迁就文档**。

---

## §1 Genesis

| 项 | 值 | 证据 |
|---|---|---|
| 固定时间戳 | `GenesisTimestamp = 1700000000`（2023-11-14T22:13:20Z） | `internal/blockchain/genesis.go:14` |
| coinbase 接收方 | `GenesisMinerPubKeyHash = [20]byte{0,…,1}`（无对应私钥 ⇒ **不可花费的黑洞**） | `genesis.go:19` |
| coinbase 金额 | `utxo.Subsidy(0)` = **5** | `genesis.go:39` |
| bits | `pow.MaxTargetBits` = **16** | `genesis.go:40` |
| PrevBlockHash | 零 `[32]byte{}` | `genesis.go:40` |
| 挖矿方式 | **串行** `pow.Mine`（nonce 从 0 递增，返回首个解） | `genesis.go:42` |
| canonical 哈希锚点 | `CanonicalGenesisHash = 0x00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` | `genesis.go:27` |

### 1.1 确定性规则

创世必须**字节级可复现**：固定时间戳 + 固定 coinbase + 固定难度 + **串行**挖矿，四者共同保证所有节点得到同一创世。

> `pow.MineParallel` **不保证**返回最小合法 nonce（谁先命中谁生效）⇒ **任何要求字节级可复现的场合（含创世）禁止使用并行挖矿**。

### 1.2 新目录不再隐式初始化

自 F-3B 起，**空数据目录不会被节点启动路径隐式初始化**。必须先执行 `node init -datadir <dir>` 创建 canonical genesis，再启动节点。启动前统一经过 `VerifyGenesisIdentity`，任何身份不匹配或未初始化状态在 consensus replay 前 **fail closed**。

### 1.3 创世规则

- 创世区块**视为可信**（不做头校验），但其交易必须能成功建立初始 UTXO 状态；
- 创世 coinbase 的成熟期照常适用（10 块）；其输出因无私钥而**永不可花费**；
- 改动任何影响 canonical genesis 的共识参数 ⇒ 必须先经兼容性审计与协议身份决策，**不得静默改变 `CanonicalGenesisHash`**。

---

## §2 Block

### 2.1 Header（`internal/block/block.go:21`）

```go
type Header struct {
    Version       uint32   // 协议版本号（共识强制，见 §5.1）
    PrevBlockHash [32]byte // 父块头哈希
    MerkleRoot    [32]byte // 本块交易的 Merkle 根
    Timestamp     int64    // Unix 秒
    Bits          uint32   // 难度位
    Nonce         uint64   // PoW 随机数
}
```

### 2.2 Header 序列化（`block.go:55`）

`SerializeHeader()`：**定长 80 字节**，字段顺序固定：

| 偏移 | 长度 | 字段 | 端序 |
|---|---|---|---|
| 0 | 4 | Version | **小端** |
| 4 | 32 | PrevBlockHash | 原始字节 |
| 36 | 32 | MerkleRoot | 原始字节 |
| 68 | 8 | Timestamp | **小端** |
| 76 | 4 | Bits | **小端** |
| 80 | 8 | Nonce | **小端** |

### 2.3 区块哈希（`block.go:67`）

```
hash = SHA256( SHA256( SerializeHeader() ) )     // 双 SHA-256，32 字节
```

### 2.4 区块规范编码（`internal/block/codec.go:33`）

```
block bytes = SerializeHeader() ‖ u32LE(len(Txs)) ‖ encodeTx(tx)*
```

- 单块交易数上限 `maxTxPerBlock = 100_000`（`codec.go:20`），超限 ⇒ `ErrTxDecodeTooLarge`；
- `Block.Size() = len(Encode())` —— **体积口径 = 规范编码长度**。

### 2.5 Merkle 根（`block.go:81`）

- 叶子 = 各交易的 **TxID**（见 §6.2）；
- 每层奇数个 ⇒ **最后一个元素与自身拼接**（比特币经典处理）；
- 每层合并 = `SHA256( left ‖ right )`（**单 SHA-256**）；
- 空交易列表 ⇒ `SHA256(nil)`。

### 2.6 区块大小

`MaxBlockSize = 1 << 20`（**1 MiB**），按 §2.4 的规范编码长度计算（`internal/blockchain/blockchain.go:57`）。

---

## §3 PoW

| 项 | 规则 | 证据 |
|---|---|---|
| 哈希函数 | 双 SHA-256 | `block.go:68` |
| 目标值 | `target = 2^(256 - bits)` | `pow.BitsToTarget`（`internal/pow/pow.go:147`） |
| 有效性判据 | `hash < target`（**严格小于**） | `pow.Validate`（`pow.go:162`） |
| bits 域 | 合法区块 bits ∈ `[1, 40]`（⊂ 共识域 `[1,256]`） | §5 |
| bits 越界防护 | `bits > 256` ⇒ 返回**零目标**（PoW 恒失败），**不构造大整数**；bits ≤ 256 时输出与加固前逐字节一致 | `pow.go:147-154`（F-4 输入加固） |
| 挖矿入口 | `Mine`（串行·确定性）／`MineParallel`（并行·非确定）／`MineCancelable`（可取消） | `pow.go:183/209/220` |
| 并行切分 | worker `w` 只试 `nonce ∈ {w, w+workers, …}`（等差类，覆盖完整、不重不漏） | `pow.go:242-269` |
| 取消语义 | 关闭 cancel 通道 ⇒ 全部 worker 尽快退出（防 goroutine 泄漏） | `pow.go:220` |

> **位宽边界**：`targetBitWidth = 256`。`256 - bits` 按 uint32 计算；bits > 256 时会回绕成 2^32 量级（曾可构造 512 MiB 中间值，构成远程内存放大面），现以零目标短路。

---

## §4 难度 —— 完整重述（**禁止沿用"难度固定为 16"的旧说法**）

> 旧 README 的「`MaxTargetBits = MaxDifficultyBits = 16`、难度固定不浮动」**与当前源码不符**，已作废。

### 4.1 四个必须分开的概念（不得混为一谈）

| 概念 | 当前状态 | 证据 |
|---|---|---|
| **ALGORITHM**（算法是否存在） | **存在**。两套取整入口均已实现：`AdjustBits`（Ceil）、`AdjustBitsNearest`（Nearest），共享 `adjustTargetCore` | `pow.go:416 / 440 / 326` |
| **PARAMETER**（参数值） | `MaxTargetBits=16`、`MaxDifficultyBits=40`、`DifficultyAdjustmentInterval=20`、`TargetBlockTimeSeconds=60`（V1/V2）／`NewRulesetTargetBlockTimeSeconds=300`（V3） | `pow.go:29/52/37/33/132` |
| **ACTIVATION**（是否已激活） | **v2 未激活**（生产链高度 2026-10-02 实测约 137 < 2000）；**v3 未激活**（< 3000）。二者均为 **FROZEN + IMPLEMENTED + ACTIVATED=NO** | `pow.go:81-82` |
| **CURRENT CHAIN STATE**（当前链上实测） | 生产链高度约 **137**（2026-10-02 实测）< 2000 ⇒ **当前链上难度恒为 16** | 生产现态 |

> **核心裁定**：
> `ALGORITHM EXISTS ≠ CONSENSUS CURRENTLY USES VARIABLE DIFFICULTY`。
> 当前恒为 16 是**"尚未到达激活高度"**，**不是**"设计上不可浮动"。

### 4.2 共识常量

| 常量 | 值 | 角色 |
|---|---|---|
| `MaxTargetBits` | **16** | 创世难度 / v1·v2 的 AdjustBits 起点 / **难度下限（floor）** —— 三重角色 |
| `MaxDifficultyBits` | **40** | 难度浮动**上限**（ceiling） |
| `TargetBlockTimeSeconds` | 60 | 期望出块间隔（V1/V2） |
| `NewRulesetTargetBlockTimeSeconds` | **300** | 期望出块间隔（V3） |
| `DifficultyAdjustmentInterval` | 20 | 调整周期（块数） |
| `ActivationHeight` | **2000** | v2 边界（难度浮动 + MTP 时间戳 + 版本 v2） |
| `NewRulesetActivationHeight` | **3000** | v3 边界（Nearest 取整 + 版本 v4） |
| `NewRulesetInitialBits` | **30** | v3 起点一次性注入值 |
| `LegacyBlockVersion` / `NewBlockVersion` / `NewRulesetBlockVersion` | 1 / 2 / 4 | 区块版本三态 |

### 4.3 三态规则（`pow.ComputeExpectedBitsAt`，`pow.go:546`）

规则**完全由区块高度唯一确定**，与 canonical tip 无关（高度锚定，跨分支确定，无 retroactive reinterpretation）。

| 高度区间 | 规则集 | bits 计算 | 版本 |
|---|---|---|---|
| `h == 0` | genesis | **固定 `MaxTargetBits`(16)**（注入 30 **绝不外溢**） | v1 |
| `0 < h < 2000` | **v1（LEGACY）** | **钉死父块 bits**（父块恒 16 ⇒ 恒 16） | v1 (=1) |
| `2000 ≤ h < 3000` | **v2** | 非周期边界沿用父块 bits；周期边界 `AdjustBits`（**Ceil**，target 60s），钳制 `[16, 40]` | v2 (=2) |
| `h == 3000` | **v3 起点** | **无条件返回 30**（一次性注入，先于周期边界判断） | v3 (=4) |
| `h > 3000` | **v3** | 非周期边界沿用父块 bits；周期边界 `AdjustBitsNearest`（**Nearest**，target 300s），钳制 `[16, 40]` | v3 (=4) |

**周期边界**：`h % DifficultyAdjustmentInterval == 0`。

### 4.4 重算公式（retarget）

`adjustTargetCore`（`pow.go:326`）执行与取整无关的三步：

1. **幅度 clamp**：`actualTimespan ← clamp(actualTimespan, expected/4, expected*4)`，其中 `expected = targetBlockTime × 20`（V2：`60 × 20 = 1200` 秒；V3：`300 × 20 = 6000` 秒，由参数化 `adjustTargetCore` 传入）；
2. **方向推导**：`newTarget = currentTarget × actualTimespan / expected`（无分支，整数运算）；
3. **floor clamp**：`newTarget > MaxTarget()` ⇒ `floorHit`，直接返回 `MaxTargetBits`。

随后按规则集取整：

- **Ceil（v2）**：`b0 = 257 - BitLen(newTarget)`（= `ceil(256 - log2(newTarget))`，偏难取整）；
- **Nearest（v3）**：`b0 = 257 - BitLen(t)`；判据 `t² ≤ 2^(513-2·b0)` ⇒ 取 `b0`，否则取 `b0-1`。**全程整数运算、无浮点**；相邻 bits 的几何中点是无理数而 `t` 恒为整数 ⇒ **tie 不可达**（完备性取 round-half-up）。

最后 **ceiling clamp**：`newBits > MaxDifficultyBits ⇒ 40`。

**执行顺序（冻结，不可换位）**：① floor clamp（target）→ ② 取整（bits）→ ③ ceiling clamp（bits）。

### 4.5 工作量度量（fork-choice 基础）

- `WorkOfBits(bits) = 2^bits`（`pow.go:478`）。target 是 2 的整数次幂 ⇒ 满足 target 的期望尝试次数**严格**等于 `2^bits`；
- `CumulativeWork = Σ 2^bits`，全程 `big.Int`（跨数千块必然越 uint64）。

### 4.6 难度参数禁令（写死，不得违反）

1. **禁止**把 `MaxTargetBits` 改成 27（它承担创世难度 / v1·v2 起点 / floor 三重角色）；
2. **禁止**让 `NewRulesetInitialBits=30` 外溢到 genesis（genesis 恒 16，由 `ComputeExpectedBitsAt` 的 `h==0` 分支保证）；
3. 改动任何难度参数 ⇒ **共识真值变更** ⇒ 旧节点拒绝新块 ⇒ 属独立授权阶段。

---

## §5 版本与时间戳

### 5.1 版本（硬分叉强制）

`pow.VersionForHeight(h, activationHeight)` 返回该高度**必须**使用的版本号。区块版本与期望值不符 ⇒ `ErrInvalidVersion`。

> 旧节点对 v3 区块构成**双重拒绝**：① `VersionForHeight` 返回 2 ≠ 3 ⇒ `ErrInvalidVersion`；② 即使绕过版本校验，旧节点用 Ceil 算出的 expected bits ≠ 新块 Nearest bits ⇒ `ErrUnexpectedBits`。

### 5.2 时间戳（`blockchain.validateTimestamp`）

| 状态 | 规则 |
|---|---|
| **pre-activation（h < 2000）** | `Timestamp ≥ 父块 Timestamp` **且** `Timestamp ≤ 本地时钟 + maxFutureTimestampDrift` |
| **post-activation（h ≥ 2000）** | `Timestamp > MTP(h-1)` **且** `Timestamp ≤ MTP(h-1) + 7200` |

**MTP（Median Time Past）**定义（`pow.MedianTimePastAt`，`pow.go:599`）：

- 窗口 `[max(0, h-10), h]`（**至多 11 个区块**；必须是 `h-10` 而非 `h-11`——后者构成 12 块窗口，会在非单调时间戳时令不同节点算出不同 MTP，造成永久分叉）；
- 升序排序后取下标 `k = n/2`（上半中位数，与比特币一致）；
- post-activation 规则**不依赖任何墙钟** ⇒ 墙钟偏差/恶意时钟不会造成永久共识分叉。

---

## §6 Coinbase 与奖励

| 项 | 规则 | 证据 |
|---|---|---|
| 奖励公式 | **`Subsidy(h) = 5 >> (h / 5_250_000)`** | `internal/utxo/apply.go:36` |
| 初始奖励 | **5**（`subsidyInitial = 5`）—— **不是 50** | `apply.go:22` |
| 减半间隔 | **5,250,000** 块（**不是 210**） | `apply.go:27` |
| 归零 | `5 = 0b101` 仅 3 位 ⇒ 第 3 次减半归零；最后非零高度 **15,749,999**（补贴 1），归零高度 **15,750,000** | `apply.go:32-34` |
| 防御上界 | `halvings >= 64 ⇒ 0`（对 int 高度实际不可达） | `apply.go:38` |
| 成熟期 | `CoinbaseMaturity = 10` 块 | `apply.go:16` |
| 高度编码 | 区块高度以 **4 字节小端**写入 coinbase 输入的 `Signature` 字段（BIP34 风格） | `transaction.go:63` |
| coinbase 上限 | `coinbaseOut ≤ Subsidy(h) + 全块手续费`；以**减法形式**比较（`coinbaseOut - Subsidy > fees` ⇒ 拒绝），从构造上消除加法回绕 | `apply.go:277-290` |
| 位置/数量 | coinbase 必须为**首笔且仅有一笔** | `apply.go` |

> 🔴 **旧 README 的 `50 >> (height/210)` 已作废**，属 `DOCUMENTATION DRIFT`（见审计 R-01）。

---

## §7 Transactions

### 7.1 结构（`internal/transaction/transaction.go`）

```go
type TxInput struct {
    PrevTxHash [32]byte // 被引用的交易哈希
    OutIndex   uint32   // 输出索引
    Signature  []byte   // 签名（coinbase 时为高度编码）
    PubKey     []byte   // 公钥（非压缩 SEC1，65B）
}
type TxOutput struct {
    Value      uint64   // 金额（最小单位整数）
    PubKeyHash [20]byte // 接收方公钥哈希
}
type Transaction struct { Inputs []TxInput; Outputs []TxOutput }
```

### 7.2 规范编码（`transaction/codec.go:25`）

```
u32LE(len(Inputs))
for each input:  PrevTxHash(32) ‖ u32LE(OutIndex) ‖ u32LE(len(Sig)) ‖ Sig ‖ u32LE(len(PubKey)) ‖ PubKey
u32LE(len(Outputs))
for each output: u64LE(Value) ‖ PubKeyHash(20)
```

解码上限：`maxInputsPerTx = 10_000`、`maxOutputsPerTx = 10_000`。

### 7.3 TxID（`transaction.go:88/107`）

```
TxID = SHA256( serializeForHash(tx) )      // 单 SHA-256
```

`serializeForHash` **排除** `Signature` 与 `PubKey`（否则签名依赖自身哈希，形成循环）；
**例外**：coinbase 的 `Signature` 承载区块高度，**必须参与** TxID 计算（否则同金额 coinbase 的 TxID 相同，在 UTXO 集合中相互覆盖）。

### 7.4 签名

| 项 | 规则 |
|---|---|
| 曲线 | **NIST P-256**（secp256r1 / prime256v1），标准库 `crypto/ecdsa` |
| 公钥编码 | **非压缩 SEC1**：`0x04 ‖ X(32B) ‖ Y(32B)` = **65 字节** |
| 公钥哈希 | `SHA256(pubkey_bytes)[:20]`（**不是**比特币的 `RIPEMD160(SHA256(...))`） |
| 签名编码 | **64 字节定长 `r ‖ s`**，各 32 字节**大端**、左侧补零 |
| 定长是共识 | 变长拼接后"从中间切分"解析会错位，约 1/128 的签名会随机验不过 ⇒ **必须按 32+32 定长切分** |
| sighash | `msgHash = TxID`（`serializeForHash` 天然排除 Signature/PubKey ⇒ 无循环） |
| 验签流程 | ① 公钥 65B 且首字节 `0x04`，X/Y 在曲线上；② 签名长度必须 == 64；③ `r == 0` 或 `s == 0` ⇒ 拒绝；④ ECDSA 验签 |
| PubKey 绑定 | sighash **不覆盖** PubKey，但花费权由 `SHA256(PubKey)[:20] == UTXO 锁定的 PubKeyHash` 另行绑定 ⇒ 替换公钥必然 `ErrPubKeyMismatch` |

> 规范编码与验签流程的完整字节级描述见 `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` §9.11（**注：该文件当前含未提交的 +88/−4 修改，状态 BLOCKED**）。

### 7.5 地址

```
address = Base58Check( 0x35 ‖ SHA256(pubkey)[:20] )
Base58Check = version ‖ payload ‖ SHA256(SHA256(version‖payload))[:4]
```

- 版本字节 `AddressVersion = 0x35`（`internal/wallet/base58.go:16`）；
- 字母表去除易混淆字符 `0/O/I/l`；
- 解码校验：字符合法性 + 校验和 + 版本字节。

### 7.6 金额守恒与手续费

- `fee = Σin − Σout`（逐笔与逐块两个层面）；
- **输入总额 < 输出总额 ⇒ `ErrOverspend`**；
- 块内手续费累计发生 uint64 回绕 ⇒ **`ErrFeeOverflow`，fail-closed**（手续费是既有 UTXO 的价值转移，理论不可能达 2^64 量级 ⇒ 一旦回绕即记账不可信）；
- **未定义最低费率，未定义 dust 规则**（当前共识）。

---

## §8 UTXO

### 8.1 状态表示

`OutPoint = (PrevTxHash, OutIndex)` → `(Value, PubKeyHash, IsCoinbase, Height)`。`Height` 用于 coinbase maturity 判定（`internal/utxo/set.go:13`）。

### 8.2 状态迁移（`utxo.ApplyBlock`）

**Validate-First → Apply-Second**：在 UTXO 集合的**克隆**上做状态迁移，全部成功后**原子替换**；失败不留半迁移状态。

逐笔规则（非 coinbase 交易）：

| 检查 | 错误 |
|---|---|
| 至少一个输入 | `ErrNoInputs` |
| 至少一个输出 / 输出非零 | `ErrNoOutputs` / `ErrZeroOutput` |
| 无重复输入（同一 OutPoint 引用两次） | `ErrDuplicateInput` |
| 引用的 UTXO 存在且未花费 | `ErrUnknownUTXO`（⇒ 双花在此被拒） |
| coinbase 输出已过成熟期 | `ErrImmatureCoinbase`（`height - entry.Height < 10`） |
| 输入总额 ≥ 输出总额 | `ErrOverspend` |
| 公钥哈希匹配 | `ErrPubKeyMismatch` |
| 签名有效 | `ErrBadSignature` |

块级规则：coinbase 位置/数量 + 上限（§6）+ 手续费不回绕。

### 8.3 回滚支持

`utxo.BlockUndo` / `EncodeUndo` 支持 disconnect；`ApplyBlockWithUndo` 在 canonical append（v2 模式）时生成 undo。这是 reorg 的基础。

---

## §9 区块校验全序（`blockchain.validateBlock`，`blockchain.go:347`）

任何一步失败立即拒绝，**顺序不得调整**（对网络入块而言，先验 PoW 是必要的防 DoS 设计）：

| # | 步骤 | 失败错误 |
|---|---|---|
| 1 | PrevHash 等于当前 tip 哈希 | `ErrInvalidPrevHash` |
| 1.5 | 版本号 == `VersionForHeight(h)` | `ErrInvalidVersion` |
| 1.6 | bits ∈ 共识域 `[1,256]`（**廉价闸门，不构造任何目标值**） | `ErrUnexpectedBits` |
| 2 | PoW（可跳过，见下） | `ErrInvalidPoW` |
| 3 | bits == 本链计算的期望难度 | `ErrUnexpectedBits` |
| 4 | 时间戳（§5.2） | `ErrTimestampOutOfRange` |
| 5 | Merkle 重验 | `ErrMerkleMismatch` |
| 6 | 体积 ≤ 1 MiB | `ErrBlockTooLarge` |
| 7 | coinbase 布局 + 签名 + 双花 + maturity + 金额 + coinbase 上限 | `ErrBadTxLayout` / 原始错误 |

**`skipPoW`**（`ValidateTemplate`）：仅用于**本地挖矿模板预校验**，跳过第 2 步，其余步骤与顺序完全不变。**绝不允许用于来自网络的区块**（跳过 PoW 即放弃防伪造区块的 DoS 保护）。

---

## §10 Chain Selection（**reorg 已实现**）

> 🔴 **旧 README 的「未实现树状链与重组 / 单链追加式 / 收到父哈希不匹配的区块直接拒绝」已作废**，与当前源码**完全反向**（审计 R-04）。

### 10.1 区块树

`internal/blocktree` 维护 `BlockNode`（Hash / ParentHash / Height / Bits / Timestamp / Work / CumulativeWork / children），支持 `LookupNode`、`PathToRoot`、`AncestorAtHeight`、`IsAncestorOf`、`FindCommonAncestor`、`CheckInvariant`。

### 10.2 Fork-choice（`ShouldReorg`，`blocktree/settip.go:164`）

**纯只读决策**，不切换 tip、不改外部状态：

```
CompareWork(candidate) > 0  → candidate 胜（累积工作量反超）
CompareWork(candidate) < 0  → 活动链尾胜（工作量不足）
CompareWork(candidate) == 0 → tie-break：tip hash 大端较大者胜（bytes.Compare，确定性、全网可复算）
```

度量 = `CumulativeWork = Σ 2^bits`（§4.5）。规则冻结于 `settip.go` §E.1 / §E.2。

### 10.3 重组执行

| 组件 | 说明 |
|---|---|
| `blockchain.executeReorg(newTip, persist)` | disconnect 旧分支（应用 UNDO）→ apply 新分支 → 持久化 → 更新内存 |
| `ReorgResult` | 记录断开/连接分支的区块与 UNDO，供上层做 mempool resurrection |
| `AddBlockWithResult` | 与 `AddBlock` 相同，额外返回 reorg 结果 |
| `CommitReorg`（storage 侧） | UNDO + 新块 + 新 UNDO + 新 tipHash 的提交接口 |
| 前置条件 | 触发 reorg 的块**必须先持久化**（否则 `blockAtHash` 找不到它）；失败必须显式返回，绝不静默 |

### 10.4 分叉与孤儿

`addBlock` 三路分发：

| Case | 条件 | 行为 |
|---|---|---|
| 1 直接延长 | `parentHash == currentTip.Hash()` | `extendChain`（全序校验 + 持久化） |
| 2 Fork | 父在树中但非 tip | 加入 blocktree → `validateForkBlock`（基于**父分支的 UTXO** 做完整共识校验）→ `ShouldReorg` → 是则 `executeReorg`，否则**保存为 detached 但不切换 canonical** |
| 3 Orphan | 父不在树中 | 返回 **`ErrOrphanParent`**（`ErrInvalidPrevHash` 的特化） |

**重复持久化防护**：区块哈希若已存在于 canonical 链 ⇒ 显式拒绝（疑似重复记录），**不得**当作幂等 re-delivery 静默吞掉。判据**只**查 canonical 内存链（`canonicalContains`），**不得**用 `blockAtHash`（后者会回退到 store 索引，误伤已落盘的合法 fork 块）。

**孤儿处理**：`ErrOrphanParent` → `deferOrphan` 登记等待 + 向来源对端发起 `get_block_by_hash` 补齐祖先（详见 `docs/CANONICAL-P2P-SPEC.md`）。完整 orphan pool 属 REORG-1G/B5，**明确未做**。

### 10.5 无效分支规则

- fork 校验失败 ⇒ 以 `ErrInvalidPrevHash` 包装返回（保持既有断言兼容，`errors.Is(err, ErrInvalidPrevHash)` 仍为 true），**附带真实失败原因**；
- 确定性分叉拒绝：旧节点对 v3 区块**双重拒绝**（§5.1）；
- reorg 前持久化失败 ⇒ 显式返回错误，绝不进入半切换状态。

---

## §11 共识参数总表

| 参数 | 值 | 位置 |
|---|---|---|
| 区块大小上限 | 1 MiB | `blockchain.go:57` |
| coinbase 成熟期 | 10 | `utxo/apply.go:16` |
| 初始奖励 | 5 | `utxo/apply.go:22` |
| 减半间隔 | 5,250,000 | `utxo/apply.go:27` |
| 难度下限 / 创世难度 | bits 16 | `pow.go:29` |
| 难度上限 | bits 40 | `pow.go:52` |
| 目标出块间隔 | 60 s（V1/V2）／300 s（V3） | `pow.go:33/132` |
| 调整周期 | 20 块 | `pow.go:37` |
| 幅度 clamp | [1/4×, 4×] | `pow.go:330-337` |
| v2 激活高度 | 2000 | `pow.go:67` |
| v3 激活高度 | 3000 | `pow.go:94` |
| v3 注入 bits | 30 | `pow.go:106` |
| 区块版本 | 1 / 2 / 4 | `pow.go:70/73/114` |
| MTP 窗口 | `[max(0,h-10), h]` | `pow.go:606` |
| post-activation 时间戳上界 | MTP(h-1) + 7200 | `consensus.go` |
| 地址版本字节 | 0x35 | `wallet/base58.go:16` |
| 签名曲线 | P-256 | 标准库 |

---

## §12 与旧文档的 DRIFT 登记

| 旧声明 | 本文真值 | DRIFT |
|---|---|---|
| 奖励 `50 >> (h/210)` | `5 >> (h/5,250,000)` | **RESOLVED**（R-01，已于 GOVERNANCE CLOSURE-1 修正 README.md:165） |
| `MaxDifficultyBits = 16`、难度固定不浮动 | `MaxTargetBits=16` / `MaxDifficultyBits=40` + 三态规则集，未激活 | **CONFLICT**（R-02/R-03） |
| 未实现 reorg / 单链追加式 | blocktree + ShouldReorg + executeReorg + orphan 保留 | **CONFLICT**（R-04） |
| `internal/blockchain/blockchain.go:9` 包注释"reorg 仍为单链追加实现" | 同文件 `executeReorg` 已实现 | **RESOLVED**（R-20，已于 GOVERNANCE CLOSURE-1 修正 blockchain.go 包注释） |
