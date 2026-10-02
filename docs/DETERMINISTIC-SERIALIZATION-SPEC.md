# P2PChain Deterministic Serialization Specification

> 版本：V1（PHASE BRAND-1.2）
> 适用提交：`84eb6ce` 及之后（本规范只描述**当前真实实现**，不描述未来格式）
> 目标：让第三方程序员**不阅读 Go 生产源码**，仅凭本文档即可完成
> 数据解析 / 区块哈希计算 / TxID 计算 / Merkle 重算 / 创世区块重建。

---

## 0. 范围与保证

本文规范以下内容：

- 整数与字节串编码
- 区块头（88 字节）与区块哈希
- 交易序列化与 TxID
- Merkle 计算
- 区块规范编码
- `blocks.dat` 记录分帧
- 工作量证明判定
- 创世区块的确定性构造与期望哈希

本文**不**规范（见 §10 已知 GAP）：

- 交易费用策略与选币算法

---

## 9.1 Integer encoding

| 项目 | 规则 |
|---|---|
| 字节序 | **小端（little-endian）** |
| 定长整数 | `uint32` = 4 字节；`uint64` = 8 字节；`int64` = 8 字节（二进制补码） |
| 有符号性 | 仅 `Timestamp` 为 `int64`；其余全部无符号 |
| 变长编码 | **不存在**。所有长度字段都是**定长 `uint32` 长度前缀**，不是 varint |
| 字节串 | `uint32` 长度前缀 + 原始字节；长度为 0 时**只写前缀，不写数据** |
| 定长哈希 | 32 字节原样写入，无长度前缀，无端序转换（按字节序列处理） |
| 20 字节哈希 | 同上，20 字节原样写入 |

---

## 9.2 Block header

区块头固定 **88 字节**，字段顺序即序列化顺序：

| # | 字段 | 类型 | 长度 | 说明 |
|---|---|---|---|---|
| 1 | `Version` | uint32 LE | 4 | 协议版本号，当前实现恒为 **1** |
| 2 | `PrevBlockHash` | [32]byte | 32 | 父区块头哈希；创世为 32 个 `0x00` |
| 3 | `MerkleRoot` | [32]byte | 32 | 本区块交易列表的 Merkle 根（见 §9.5） |
| 4 | `Timestamp` | int64 LE | 8 | Unix 秒 |
| 5 | `Bits` | uint32 LE | 4 | 难度位（见 §9.7） |
| 6 | `Nonce` | uint64 LE | 8 | 工作量证明随机数 |

合计 `4 + 32 + 32 + 8 + 4 + 8 = 88` 字节。

> 注：代码注释中出现的「80 字节」是沿袭比特币的说法，**本链实际为 88 字节**
> （Nonce 为 `uint64` 而非比特币的 `uint32`）。以 88 为准。

**哈希输入** = 上述 88 字节。
**哈希算法** = 双 SHA-256（见 §9.3）。

---

## 9.3 Hash

| 用途 | 算法 | 输入 |
|---|---|---|
| 区块头哈希 | **SHA-256(SHA-256(header88))**，即 double SHA-256 | 88 字节区块头 |
| TxID / Merkle 叶子 | **单次 SHA-256** | 交易的 sighash 序列化（见 §9.4） |
| Merkle 父节点 | **单次 SHA-256** | 左节点 32 字节 ‖ 右节点 32 字节（共 64 字节） |
| 地址校验和 | **SHA-256(SHA-256(x))** 的前 4 字节 | `version ‖ payload` |

> ⚠️ 本链**不是**处处双哈希：区块头用双 SHA-256，交易与 Merkle **只用单 SHA-256**。
> 这是第三方实现最容易出错的一点。

---

## 9.4 Transaction serialization

结构：`Inputs []TxInput` + `Outputs []TxOutput`。

### 9.4.1 规范编码（磁盘 / 网络）

```
uint32 LE  num_inputs
对每个 input:
    32 字节      PrevTxHash
    uint32 LE    OutIndex
    uint32 LE    len(Signature)   ; 后跟 Signature 字节（可为 0 长度）
    uint32 LE    len(PubKey)      ; 后跟 PubKey 字节（可为 0 长度）
uint32 LE  num_outputs
对每个 output:
    uint64 LE    Value
    20 字节      PubKeyHash
```

### 9.4.2 sighash 序列化（仅用于计算 TxID）

```
对每个 input:
    32 字节      PrevTxHash
    uint32 LE    OutIndex
    若本交易为 coinbase:
        uint32 LE   len(Signature) ; 后跟 Signature 字节
对每个 output:
    uint64 LE    Value
    20 字节      PubKeyHash
```

规则：

- **非 coinbase**：`Signature` 与 `PubKey` **都排除**在 TxID 之外（避免签名自依赖）。
- **coinbase**：`Signature` **参与** TxID 计算。原因是 coinbase 无真实签名，
  该字段用于承载区块高度（见 §9.6），若排除则同金额 coinbase 的 TxID 会互相覆盖。
- `PubKey` **永不**参与 TxID。

### 9.4.3 coinbase 判定

一笔交易为 coinbase，当且仅当：

```
len(Inputs) == 1
  && Inputs[0].PrevTxHash == 32 个 0x00
  && Inputs[0].OutIndex   == 0xFFFFFFFF
```

### 9.4.4 区块内交易布局约束（共识）

- 每区块至少 1 笔交易；
- 第 0 笔必须是 coinbase，且全区块**恰好**一笔 coinbase；
- coinbase 输出总额 ≤ `Subsidy(height) + 本区块手续费总和`；
- 普通交易必须有 ≥1 输入、≥1 输出，输出 `Value > 0`；
- 输入不得重复引用同一 UTXO；`sum(inputs) >= sum(outputs)`，差额为手续费；
- coinbase 输出需 `height - entry.Height >= 10`（成熟期）才可花费。

奖励（历史设计稿，已过期）：早期设计稿曾写 `Subsidy(height) = 50 >> (height / 210)`（每 210 块减半），**非当前实现**，仅保留作历史参考。
当前实现（权威）：`Subsidy(height) = 5 >> (height / 5_250_000)`（每 5,250,000 块减半；halvings ≥ 3 即归零，最后一个非零补贴高度 15,749,999，归零高度 15,750,000），见 `internal/utxo/apply.go` 与 `PROJECT-AI-CONTEXT.md` §7。

---

## 9.5 Merkle calculation

```
若 len(txs) == 0:
    root = SHA256(空字节串)          ; 即 e3b0c442...b855
否则:
    layer = [ TxID(tx) for tx in txs ]     ; 单 SHA-256
    当 len(layer) > 1:
        若 len(layer) 为奇数:
            layer.append(layer[-1])        ; 复制最后一个节点（比特币经典处理）
        next = []
        for i in 0, 2, 4, ...:
            next.append( SHA256(layer[i] || layer[i+1]) )   ; 单 SHA-256，64 字节输入
        layer = next
    root = layer[0]
```

- **叶子哈希** = TxID（单 SHA-256，见 §9.4.2）
- **配对顺序** = 交易在区块中的原始顺序，左 = 下标 i，右 = 下标 i+1
- **奇数处理** = 复制最后一个节点与其自身配对
- **父哈希算法** = 单 SHA-256，输入为左右子节点拼接（64 字节）

---

## 9.6 Genesis

创世区块由以下**固定输入**构造，因此全网字节级一致：

| 字段 | 值 |
|---|---|
| `Version` | `1` |
| `PrevBlockHash` | 32 个 `0x00` |
| `Timestamp` | `1700000000`（2023-11-14T22:13:20Z） |
| `Bits` | `16`（= `pow.MaxTargetBits`） |
| 交易列表 | 恰好 1 笔 coinbase |
| coinbase 输入 | `PrevTxHash` = 32 个 `0x00`，`OutIndex` = `0xFFFFFFFF`，`Signature` = `uint32 LE(0)` → `00 00 00 00`，`PubKey` 为空 |
| coinbase 输出 | `Value` = `Subsidy(0)` = **50**，`PubKeyHash` = 19 个 `0x00` 后接 `0x01`（黑洞地址，无对应私钥，不可花费） |
| `MerkleRoot` | 由该 coinbase 按 §9.5 计算（单叶 = 其 TxID） |
| `Nonce` | 由串行搜索确定：**从 0 递增，取第一个满足 §9.7 的解** |

**期望值（实机实证，多轮独立运行一致）**：

```text
MerkleRoot  = 41a8b12643a3d8ce799941e514b8d82832336915554f5ecccad0a8d4150fbf3e
Nonce       = 230970
Genesis Hash= 0000aca1af720da390380a38dc97d141cfb67afe578a2d31315860a5489db58c
```

创世区块**不做头校验**（视为可信输入），但其交易必须能成功建立初始 UTXO 状态。

---

## 9.7 Proof of Work

```
target(bits) = 1 << (256 - bits)
有效 ⟺ int(header_hash) < target        ; header_hash 按大端整数解释
```

- `bits` 语义是**前导零位数**（不是比特币的浮点式 nBits 编码）。
- 难度**已不再是「钉死在 16」**——它按**固定激活高度**分阶段演进（权威定义见 `docs/CANONICAL-CONSENSUS-SPEC.md` §4）：
  - `MaxTargetBits = 16`（创世难度 / v1·v2 起点 / **难度下限 floor** 三重角色）；
  - `MaxDifficultyBits = 32`（激活后**难度浮动上限 ceiling**）；
  - **ruleset v1（h < 2000，LEGACY）**：难度**钉死**父块 bits（= 16）；版本 `1`；
  - **ruleset v2（2000 ≤ h < 3000）**：周期边界 `AdjustBits`（**Ceil**），钳制 `[16, 32]`；版本 `2`；时间戳启用 MTP；
  - **ruleset v3（h ≥ 3000）**：周期边界 `AdjustBitsNearest`（**Nearest**），钳制 `[16, 32]`；`h == 3000` **一次性注入 `NewRulesetInitialBits = 27`**；版本 `3`。
  - **为何现网仍像「难度恒为 16」**：当前生产链高度远低于 2000，仍走 v1 钉死 16 路径——这是**尚未到达激活高度**的预期行为，**不是**「设计上不可浮动」（见 CANONICAL-CONSENSUS-SPEC.md §4.1 裁定）。
  - v2 / v3 是**结构性硬分叉**：旧节点对 v2/v3 块确定性拒绝；改动难度属真值变更，会让老节点拒绝新区块。
- 历史注记：本规格早期版本曾写「`MaxTargetBits = MaxDifficultyBits = 16`、难度固定不浮动」，该说法已被 CANONICAL-CONSENSUS-SPEC.md §4 判为**作废**（与当前源码不符）。

难度调整（供完整重放时校验 `Bits` 字段）：

```
每 DifficultyAdjustmentInterval = 20 个区块调整一次（height % 20 == 0 时）
expected = 60 * 20 = 1200 秒
actual   = clamp(tip.Timestamp - periodStart.Timestamp, expected/4, expected*4)
newTarget = currentTarget * actual / expected
若 newTarget > MaxTarget: return 16              ; floor clamp ⇒ MaxTargetBits（难度下限）
newBits = 257 - newTarget.BitLen()      ; 向下保守取整
newBits = clamp(newBits, 1, MaxDifficultyBits)   ; ceiling clamp ⇒ 32（旧版误写为 16）
```

---

## 9.8 Block canonical encoding

```
88 字节        header（见 §9.2）
uint32 LE      num_transactions
对每个交易     规范编码（见 §9.4.1）
```

区块体积上限校验基于**本编码的字节长度**，上限 `1 MiB`。

---

## 9.9 blocks.dat record framing

数据文件是**追加写**的长度前缀记录序列：

```
uint32 LE   length        ; 其后载荷的字节数
length 字节  block bytes   ; 即 §9.8 的规范编码
```

- 记录按写入顺序排列，**第 N 条记录即高度 N 的区块**（创世为高度 0 / 第 0 条）。
- 没有文件头、没有魔数、没有校验和。
- 校验/回放时逐条读取并重新执行共识校验；附加约束：非创世区块的 `PrevBlockHash`
  必须命中已有记录。

> ⚠️ 本格式**没有自校验字段**：数据被篡改后，只有通过完整重放共识校验才能发现。
> 这正是 `verify` 存在的原因。

---

## 9.10 完整校验顺序（ValidateBlock）

回放 / 接受区块时按以下顺序执行，任一步失败立即拒绝：

1. `PrevBlockHash == 当前链尾哈希`
2. 工作量证明（§9.7）
3. `Bits == 当前共识难度`
4. 时间戳：不早于父块，且不超前本地时钟 7200 秒以上
5. Merkle 重验（§9.5）
6. 体积 ≤ 1 MiB
7. 交易层：coinbase 布局 + 签名 + 双花 + 成熟期 + 金额守恒 + coinbase 上限
   （在克隆的 UTXO 集合上原子迁移，失败不留半迁移状态）

> **重要推论**：区块头参与哈希计算，因此**任何头部篡改都会在第 2 步（PoW）被拦截**；
> 而交易篡改不改变头部哈希，会一路走到第 5 步（Merkle 重验）才被发现。

---

## 9.11 P-256 signature encoding

（PHASE V2.0 补录，原 §10 G1。全部来自当前实现事实，非未来格式。）

### 9.11.1 曲线与密钥

| 项 | 值 |
|---|---|
| 曲线 | **NIST P-256**（secp256r1 / prime256v1） |
| 公钥编码 | **非压缩 SEC1**：`0x04 ‖ X(32B) ‖ Y(32B)`，共 **65 字节** |
| 公钥哈希 | `SHA256(pubkey_bytes)[:20]`（**不是** 比特币的 RIPEMD160(SHA256(...))） |

### 9.11.2 签名编码（规范编码 / 网络传输）

```
64 字节定长 = r ‖ s
  r = 32 字节，大端（big-endian），左侧补零至定长
  s = 32 字节，大端，左侧补零至定长
```

> ⚠️ **定长是共识的一部分，不是序列化偏好**。r/s 是大整数，若最高位字节为 0，
> 变长拼接后按「从中间切分」解析会错位，约 1/128 的签名会随机验不过。
> 因此实现**必须**按 32+32 定长切分，不得按实际字节长度推断。

### 9.11.3 签名消息（sighash）

```
msgHash = TxID = SHA256(serializeForHash(tx))      ; 单 SHA-256，见 §9.4.2
```

即：签名的消息哈希**就是该交易的 TxID**。由于 `serializeForHash` 天然排除
`Signature` 与 `PubKey`，不存在「签名依赖自身哈希」的循环。

`PubKey` 不被 sighash 覆盖，但花费权由「`SHA256(PubKey)[:20] ==` 该 UTXO 锁定的
`PubKeyHash`」另行绑定——替换公钥必然导致 PubKeyMismatch。

### 9.11.4 验签流程（非 coinbase 交易逐输入执行）

```
1. 解析公钥：65 字节，首字节必须为 0x04，X/Y 必须在曲线上；否则拒绝
2. 解析签名：长度必须 == 64；r = int(sig[:32])，s = int(sig[32:])
3. r == 0 或 s == 0 → 拒绝
4. ECDSA 验签：curve=P-256，hash=msgHash（TxID），(r, s)
5. 不通过 → 拒绝该交易
```

---

## 9.12 Base58Check address

（PHASE V2.0 补录，原 §10 G2。全部来自当前实现事实。）

### 9.12.1 字母表

```
"123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
```

即**比特币标准 Base58 字母表**（去除 `0 O I l`）。索引 0 = `'1'`。

### 9.12.2 编码

```
payload   = PubKeyHash（20 字节，见 §9.11.1）
checked   = version(1B) ‖ payload(20B)                  ; version = 0x35
checksum  = SHA256(SHA256(checked))[:4]                 ; 双 SHA-256 的前 4 字节
raw       = checked ‖ checksum                          ; 共 25 字节
address   = Base58Encode(raw)
```

### 9.12.3 Base58 编解码规则

- 整体按**大整数** base-58 处理（`raw` 以大端解释）；
- 前导 `0x00` 字节 → 每个对应一个前导字符 `'1'`；
- 编码结果 = 前导 `'1'`（来自前导零字节）在后、**base-58 数字部分在前**，
  即实现上先生成数字部分（低位在前），再把前导 `'1'` 追加到末尾，**最后整体反转**；
- 解码为逆过程：前导 `'1'` 个数 = 前导 `0x00` 个数。

### 9.12.4 地址长度

`0x35` 版本下 25 字节 raw 的 Base58 结果通常为 **34 字符**，例如
`NfUsAq2HVgpJrHwTtAoJNUHznkvmpxooas`（以 `N` 开头是 `0x35` 版本字节的自然结果，
**不是**协议要求——不要把它当作校验规则）。

---

## 10. 已知 GAP（第三方仅凭本文档仍无法确定之处）

| # | GAP | 影响 | 需要的补充 |
|---|---|---|---|
| ~~G1~~ | ~~P-256 签名编码与验签流程~~ | — | **已于 PHASE V2.0 补录，见 §9.11** |
| ~~G2~~ | ~~Base58 字母表与 Base58Check 编解码~~ | — | **已于 PHASE V2.0 补录，见 §9.12** |
| G3 | UTXO 集合的内存表示与克隆语义 | 无法独立复现状态迁移的**中间**状态 | 只需结果一致，一般不影响验证 |
| G4 | 选币与费率策略 | 无法独立复现节点构造的交易 | 不影响「给定区块是否合法」的判定 |
| G5 | 难度调整的浮点/整数边界细节 | 极端参数下可能差 1 bit | 已给出公式；极端值需参照实现 |

> §10 要求：记录 GAP，**不猜、不偷偷补规则**。上表即本版规范的全部已知不足。
> 本次未能新跑一次完整的第三方独立复算（执行被环境审批拦截），
> 因此第三方可复现性目前依据 **GENESIS-0 的 Python 独立复算历史证据**：
> 该轮用 Python `hashlib` 按上述规则重算创世与首块头部哈希，与节点输出**逐字节相同**。

---

## 11. 与 `verify` 的关系

`node verify` 就是本文档 §9.9 + §9.10 的可执行实现：
它以只读方式读取 `blocks.dat`，逐条重新执行 §9.10 的全部校验，
给出 `PASS` / `FAIL`（含失败高度、区块哈希、具体原因），**不修改任何字节**。
