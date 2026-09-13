# PHASE GENESIS-0 — GENESIS BLOCK & MINING RUNTIME VALIDATION

> 项目：P2PChain
> 阶段性质：READ-ONLY / RUNTIME VALIDATION / INDEPENDENT VERIFICATION
> 当时基线 HEAD：`f49b1ca04234981c4410b168ca9eb2b7cfb82384`（本阶段未产生任何 commit）
>
> **归档说明**：本报告当时以对话文本交付，未落盘；
> 于 **PHASE BRAND-1.2**（§13 GENESIS DOCUMENTATION）依据当时的原始证据正式写入 `docs/`。
> 内容为历史事实记录，未重新编造实验。

---

## 0. 阶段结论

```text
CORE OBJECTIVE 答案：YES — FIRST BLOCK VERIFIED
阶段判定：PHASE GENESIS-0 = FAIL（存在 P0 存储缺陷，见 §7）
```

阶段纪律：未改任何 `.go` / `.md` / `go.mod` / 配置，未做任何 git 写操作；
发现问题按 STOP / RECORD / DEFERRED 处理，未现场修复。

---

## 1. 运行命令与隔离数据目录

- 运行时命令（参数从 `cmd/node/main.go` 实读，非猜测）：

```text
p2pchain-node.exe -datadir <tmp>\data -listen 127.0.0.1:17788 -rpc 127.0.0.1:17789 -mine -maxblocks 1
```

- 临时数据目录（**刻意保留**作为 P0 复现素材）：

```text
C:\Users\Administrator\AppData\Local\Temp\p2pchain-genesis-validation-20260913-001240\data
```

- 项目默认数据目录 `~/.p2pchain`（内含既有状态与 `node.lock`）**全程未触碰**。

---

## 2. 创世区块（空库首次启动自动生成）

| 项 | 值 |
|---|---|
| 高度 | 0 |
| 哈希 | `0000aca1af720da390380a38dc97d141cfb67afe578a2d31315860a5489db58c` |
| PrevBlockHash | 32 字节全 0 |
| Timestamp | `1700000000` |
| Bits | 16 |
| Nonce | `230970` |
| 交易 | 1 笔 coinbase → 黑洞地址哈希 `…0001` |

**确定性实证**：三次完全独立的运行（不同临时目录、不同时间）得到**同一个创世哈希**。

---

## 3. 挖矿与首个 PoW 区块

- `[miner] 挖矿已启用…上限=1 个区块` → `开始挖矿: 高度=1 …难度位=16`
  （挖矿默认关闭，需显式 `-mine`）
- **首个 PoW 区块产出成功**（启动后 < 1 秒）

| 字段 | 值 |
|---|---|
| Height | 1 |
| Hash | `00003046a84740b11ac167ef72ee24d7e21add4a6b7b85b36251e49b13673e06` |
| PrevBlockHash | `0000aca1af720da390380a38dc97d141cfb67afe578a2d31315860a5489db58c` |
| Timestamp | `1789229966` |
| Bits | 16 |
| Nonce | `34349` |
| Difficulty | 1 |
| 交易数 | 1（coinbase） |

> 观察：创世哈希每次相同（确定性）；**首个挖出区块的哈希每次不同**
> （时间戳与 coinbase 收款地址不同）——符合「创世确定性、出块非确定性」的预期。

**矿工与奖励**：reward = **50**，收款 pubkeyhash `1c266037f0a5…61f4`，
即节点钱包 `NNUp5XbJJDPeY7VpVXb9TLzFS4k6DcmT4U`；
UTXO 金额 50、`coinbase=true`、**`mature=false`**（成熟期 10 块）→ 可花费 0 / 总 50。

---

## 4. 独立 PoW 复算（第三方可验证性历史证据）

按源码确定的序列化规则（`pow.go` / `block.go`），用 **Python `hashlib` 独立重算**
（非 Go、非节点自证）：

- 区块头 = `Version u32 LE + PrevBlockHash 32 + MerkleRoot 32 + Timestamp i64 LE + Bits u32 LE + Nonce u64 LE` = **88 字节**
- 哈希 = 双 SHA-256
- **结果：重算哈希与节点观测值逐字节相同**
  （反向确认了 `Version = 1` 与字段顺序 / 端序）
- 前导零 18 ≥ 16；`int(hash) < 1 << 240` 成立（裕度 **5.30×**）

> 这条证据后来成为 PHASE BRAND-1.2 建立公开序列化规范（
> `docs/DETERMINISTIC-SERIALIZATION-SPEC.md`）的依据。

---

## 5. API / CLI 交叉核查

| 来源 | 结果 |
|---|---|
| `GET /status` | height=1，链尾与独立重算一致，bits=16，mining=false |
| `GET /block?height=1` | 返回的原始编码**前 88 字节与 Python 复算的 header hex 完全相同** |
| `GET /balance` / `GET /utxos` | 与 CLI `status / balance / utxos` 数值逐一吻合 |

---

## 6. 优雅停止与数据锁生命周期

- 采用 Windows 优雅关停法（进程组 + `CTRL_BREAK` → Go 映射为 SIGTERM）：
  日志出现 `[node] 收到退出信号，正在关闭（释放数据目录锁）`
- `node.lock` 正常释放。
- **这是本环境首次在真实进程上验证 P3.1 的锁生命周期**（此前 Windows 下无法优雅停止）。

---

## 7. P0 缺陷（本阶段 FAIL 的原因）

### 现象

首次重启后：

- `blocks.dat` 从 **360 → 540 字节**（正好多一条区块记录）
- 磁盘出现两条**哈希相同**的记录（height 1 与 height 2 同为 `00003046a847…`）
- 内存链仍报 `高度=1` → 存储与内存状态不一致

**致命后果：第二次重启永久无法启动**

```text
EXIT=1
[node] 加载区块链失败: 回放高度 2 区块失败（数据可能损坏）:
       添加区块失败: 区块的前置哈希与当前链尾不匹配
```

即：**只要节点挖过区块，第二次重启就永久损坏。**

后续观察：失败的那次重启在报错前**又追加了一条记录**，使文件进一步涨到 **720 字节**（4 条记录）。

### 根因（只读分析，未修）

`internal/blockchain/blockchain.go` 的 `NewBlockchainFromStore` 在**进入回放循环之前**
就赋值了 `bc.store = store`，而循环体内调用 `bc.AddBlock()`，
`AddBlock()` 内部会执行 `store.SaveBlock(b)` ——
于是**每次启动都把已存在的区块再追加落盘一遍**。

佐证：`internal/storage/file.go` 的 `SaveBlock` **只校验父区块存在、不校验区块是否已存在**，
且其 `byHeight` 是 append 顺序、`Height() = len - 1`，所以重复块能占据新高度。

### 为什么之前没被发现（诚实记录）

- 此前做过一次重启，但只查了 `status` / `balance`（都正常）——**没跑 printchain**，因此漏掉；
  本次补齐了 `printchain` 才发现。
- 项目既有的 smoke E2E（15/15）含重启场景也未捕获，说明
  **缺少「重启后校验存储记录数 == 链高 + 1」的断言**。

---

## 8. Git 完整性

- HEAD 前后一致：`f49b1ca042…82384`
- 源码零改动；无新增未跟踪项；无 commit / push / tag
- 三个既有 working tree 变化保持原样

---

## 9. 后续

P0 的修复与回归在 **PHASE GENESIS-0.1** 完成（提交 `90e9fbb`），
见 `docs/PHASE-GENESIS-0.1-P0-PERSISTENCE-REMEDIATION.md`。
