# P2PChain

一个给开发者用的**本地区块链节点工具**：Go 1.22 实现，**零外部依赖（仅标准库）、单个二进制**，
在本机运行一条确定性可重放的 Proof-of-Work UTXO 链，并提供 CLI、本机 JSON 接口
与一个轻量观测控制台。

```text
Local · Deterministic · Single Binary · Developer-focused · Replayable · Verifiable
```

它的用途是：让你在几秒内**造出任意高度的本地链**、观察共识与 P2P 的真实行为、
用 CLI 构造并广播交易、并在任何时候用 `verify` **只读地**证明本地链的完整与一致。

> ⚠️ 这是**学习/实验项目**，未做安全审计，不要用于任何真实资产场景。

### 今天它是什么

```text
Developer Node
```

即：一个开发者在本机运行和调试的确定性节点 —— 自带完整 UTXO 与密码学交易校验、
按需出块、每次启动自动把整条链重新校验一遍、并对自己的数据目录拥有独占所有权。

**它不是**（这些尚未达到产品门槛，不在当前范围内）：

```text
Verification Runtime · Verifiable Work Runtime · Contribution Network
```

### 已具备的能力

- 确定性本地链（固定创世，所有节点字节级一致）
- UTXO 状态机与密码学交易校验（P-256 签名、公钥哈希锁定、双花/成熟期/金额守恒）
- 重放（启动时对整条链重新执行完整共识校验）
- 持久化（append-only `blocks.dat` + 数据目录独占锁 + 优雅释放）
- P2P 同步（真实区块/交易传播、追赶同步、种子重连）
- 挖矿（可取消、多核并行）
- **离线只读校验（`verify`）**

## 已实现的功能（全部可运行、有测试覆盖）

- **完整 UTXO 模型**：OutPoint 引用、余额、coinbase 成熟期（10 块）、块内/池内双花检测
- **完整交易校验**：结构校验、签名验证（P-256 ECDSA，64 字节定长 r||s 编码）、公钥哈希锁定、金额守恒、手续费计算
- **区块校验全序**：父哈希 → PoW → 难度位 → 时间戳 → Merkle 重验 → 大小限制 → coinbase 位置/数量 → 状态迁移（Validate-First → Apply-Second，clone 上验证后原子替换）
- **确定性创世区块**：固定时间戳 + 固定 coinbase + 串行挖矿，所有节点得到字节级相同的创世
- **工作量证明**：双 SHA256、难度调整（比特币式 4 倍限幅）、**多核并行挖矿**（等差类切分搜索空间）、**可取消挖矿**（链尾变化立即放弃候选区块，不泄漏 goroutine）
- **钱包**：P-256 密钥对、Base58Check 地址（版本字节 0x35）、JSON 落盘（0600 权限）
- **交易构建**：选币（降序累计）→ 构造输出 + 找零 → 逐输入签名
- **内存池（Mempool）**：TxID 去重、组合视图验证（链 UTXO + 池内交易叠加）、出块后重验剔除失效交易
- **持久化**：`blocks.dat` 追加写 + 启动全量回放重建 UTXO；重启不丢链
- **P2P 网络**：TCP + 换行分隔 JSON；握手（交换高度与已知节点）、区块/交易真实传播与中继、追赶同步（分批拉取）、**种子节点断线自动重连**（防孤岛链）
- **控制接口**：localhost JSON API（`/status /balance /utxos /send /mine /block`）
- **CLI 子命令**：`ui / node / status / balance / utxos / send / mine / wallet / printchain / verify / help`

## 快速开始

需要 Go 1.22+：

```bash
# 构建
go build -o node ./cmd/node

# 启动第一个节点（矿工模式，并行挖矿 worker 数 = CPU 核数）
./node -listen :6688 -mine -datadir ./data-a

# 另一个终端：启动第二个节点，连接第一个节点，自动同步
./node -listen :6689 -rpc 127.0.0.1:6690 -seed 127.0.0.1:6688 -datadir ./data-b
```

矿工节点日志示例：

```
[node] 本地区块链已就绪: 高度=0 链尾=0000aca1af72...
[node] 节点钱包地址=NfUsAq2HVgpJrHwTtAoJNUHznkvmpxooas
[p2p] 节点已启动，监听 127.0.0.1:6688
[miner] 开始挖矿: 高度=1 打包交易=0 手续费=0 难度位=16
[miner] 挖到新区块: 高度=1 哈希=0000d8ef4d8d... 交易数=1
```

## CLI 用法

```bash
# 在线命令（经控制接口查询/操作运行中的节点）
./node status                          # 高度、链尾、对等节点、交易池、是否挖矿
./node balance                         # 节点钱包余额（-address 可查任意地址）
./node utxos                           # 未花费输出列表
./node send -to <地址> -amount 100      # 转账（最小单位整数）
./node mine -count 6                    # 按需出块（开发/测试用）

# 离线命令
./node wallet -datadir ./data-a         # 查看或创建本地钱包
./node printchain -datadir ./data-a     # 打印本地区块链（只读）
./node verify -datadir ./data-a         # 只读校验本地链（退出码 0=通过 / 1=不通过）
```

## 只读校验（verify）

`verify` 以**只读**方式打开 `blocks.dat`，逐块重新执行与节点启动时完全相同的共识校验
（父哈希 → PoW → 难度位 → 时间戳 → Merkle 重验 → 体积 → 交易状态迁移），
输出 `PASS` / `FAIL`；失败时给出**失败高度、区块哈希与具体原因**，而不是泛化的 "invalid"。

```text
$ ./node verify -datadir ./data-a
[verify] 数据目录  : ./data-a
[verify] 已校验区块: 4 个（高度 0 → 3）
[verify] 创世哈希  : 0000aca1af72...db58c
[verify] 链尾哈希  : 0000b60369db...6ce59
[verify] 结果      : PASS（只读回放校验通过，未修改任何数据）
```

保证（设计与测试双重约束）：

- 不写 `blocks.dat`（校验前后的 SHA-256 与文件大小不变，有回归测试覆盖）
- 不获取/创建/删除数据目录锁 —— 因此**可以在节点正在运行时执行**
- 不加载或写入钱包、不产生新区块、不触发挖矿、不启动 P2P、不改变链状态
- 退出码：`0` = 通过；`1` = 未通过或无法执行；`2` = 参数错误

机器可读输出：`./node verify -datadir ./data-a -json`

字节级格式（供第三方独立实现解析/复算）见
[`docs/DETERMINISTIC-SERIALIZATION-SPEC.md`](docs/DETERMINISTIC-SERIALIZATION-SPEC.md)。

## 控制台（Console）

`node ui` 会在启动节点后打开内嵌的 Developer Console。
它是**轻量节点观测面**（状态、链尾、网络、日志 + 按需出块），
**不是**区块浏览器、交易构建器或钱包管理器 —— 这些能力请使用上面的 CLI。

节点启动选项：`-listen`（P2P 监听）、`-rpc`（控制接口，仅本机回环）、`-seed`（种子地址，逗号分隔）、`-datadir`（数据目录）、`-mine`（启用挖矿）、`-maxblocks`（出块上限）、`-miners`（并行 worker 数）。

完整说明运行 `./node help`。

## 控制接口（JSON over HTTP，仅本机）

| 端点 | 方法 | 说明 |
|---|---|---|
| `/status` | GET | 节点状态（高度/链尾/对等/池/挖矿中） |
| `/balance?address=` | GET | 余额（可花费与含未成熟两个口径） |
| `/utxos?address=` | GET | 未花费输出列表 |
| `/send` | POST | 用节点钱包转账 |
| `/mine` | POST | 按需出块 `{"count": N}` |
| `/block?height=N` | GET | 区块十六进制内容 |

默认监听 `127.0.0.1:6689`，无鉴权——**切勿暴露到不可信网络**（启动时监听到非回环地址会打警告）。

## P2P 协议

TCP 长连接，每行一个 JSON 消息。消息类型：

| 类型 | 方向 | 说明 |
|---|---|---|
| `handshake` | 双向 | 连接建立后交换：节点 ID、服务高度、 genesis 哈希、已知节点列表 |
| `new_block` | 广播 | 完整区块；接收端完整校验（PoW/父哈希/交易/UTXO）后上链并中继（排除来源对端） |
| `new_tx` | 广播 | 完整交易；入内存池（组合视图验证）后中继 |
| `get_blocks` | 请求→响应 | 握手发现对端更高时发起追赶；`{fromHeight}` 起分批拉取（每批 ≤200 块） |
| `blocks_resp` | 响应 | 区块批次；逐块校验上链，未到链尾则继续请求 |

保护措施：消息大小上限、读写超时、死连接清理、重复数据去重。种子节点断线后由后台协程周期性（5 秒）补齐连接。

## 共识参数

| 参数 | 值 | 说明 |
|---|---|---|
| 出块奖励 | `50 >> (height/210)` | 每 210 块减半 |
| Coinbase 成熟期 | 10 块 | 比特币为 100，测试网取向 |
| 区块大小上限 | 1 MiB | 规范二进制序列化长度 |
| 初始难度 / 难度上限 | `MaxTargetBits = MaxDifficultyBits = 16` | 期望约 2^16 ≈ 6.5 万次哈希/块（CPU 毫秒级出块，测试网调优值） |
| 出块目标间隔 | 60 秒 | 难度调整的期望跨度基准 |
| 难度调整周期 | 20 块 | 比特币为 2016 |
| 难度动态范围 | **固定为 16（上限 = 下限）** | 有意设计，见下方「难度为何不浮动」 |
| 地址版本字节 | 0x35 | 自定义网络的 Base58Check 前缀 |
| 签名曲线 | P-256 | 标准库 `crypto/ecdsa`；比特币用 secp256k1 |

### 难度为何不浮动（重要，避免误读为缺陷）

本链的难度调整**推导是完整的**（`pow.AdjustBits`：实际跨度短于期望 → 更难；长于期望 → 更易，
单次幅度限制在 4 倍内），但输出随后经过两层**有意的**钳制：

- **难度下限**：`target` 不得超过 `T(MaxTargetBits)`，即难度不得低于初始值；
- **难度上限**：`bits` 不得超过 `MaxDifficultyBits`（本链 `== MaxTargetBits`）。

因此在本链当前共识参数下，**链上可达难度被固定为 16，难度不会浮动**。这是**测试网调优决定，
不是缺陷**：本链以 CPU 毫秒级出块为目标，实际出块间隔远小于 60 秒；若允许难度自由上升，
每个调整周期会 `+2 bits`（16→18→20…），约 200 块后单块需枚举 `2^36` 次哈希，
单块耗时从毫秒级升到小时级，学习与回归价值随之消失。

需要难度真正浮动时，必须**重设 clamp 带宽**并把 `MaxDifficultyBits` 抬到预期上限——
这属于**独立的共识参数阶段**：难度是共识真值，改动会让老节点拒绝新区块。
相关的单元级与链级证据见 `internal/pow/pow_test.go`、`internal/blockchain/blockchain_test.go`。

## 目录结构

```
p2pchain/
├── cmd/node/            # 节点入口：组装、CLI 子命令、控制接口实现
├── internal/
│   ├── block/           # 区块结构、头哈希、Merkle 树、二进制编解码
│   ├── blockchain/      # 链管理、区块校验全序、确定性创世
│   ├── control/         # 本地 JSON 控制接口（server + client）
│   ├── mempool/         # 交易内存池（组合视图验证）
│   ├── p2p/             # TCP 网络层（握手/广播/同步/重连）
│   ├── pow/             # 工作量证明、并行挖矿、难度调整
│   ├── storage/         # 区块存储（内存版 + 文件版）
│   ├── transaction/     # UTXO 交易结构
│   ├── txbuild/         # 交易构建（选币/找零/签名）
│   ├── utxo/            # UTXO 集合与状态迁移
│   └── wallet/          # 密钥、地址、签名、持久化
├── scripts/
│   ├── run-tests.sh     # canonical 全量测试入口（显式 -timeout 契约）
│   └── smoke-e2e.sh     # 双真实进程端到端冒烟测试
└── docs/                # 设计文档与阶段报告
```

## 测试

**canonical 测试入口**（版本化的 timeout 契约）：

```bash
bash scripts/run-tests.sh   # == go test ./... -count=1 -timeout 25m
```

> ⚠️ **必须显式 `-timeout`**：`go test` 的**每包默认超时是 10m**，而本仓库 `cmd/node`
> 全量实测耗时 **583.5s ～ 605.3s**（真实 `go build` 子进程 + 真实 node 子进程 + 真实 RPC 挖矿
> + 真实 TCP P2P，138 个测试）⇒ 默认值会以 `panic: test timed out after 10m0s` 产生**假失败**
> （已实测复现，无任何 `--- FAIL:` 断言失败）。详见 `docs/TEST-EXECUTION-CONTRACT.md`。

底层等价命令：

```bash
go test ./... -count=1 -timeout 25m        # 单元 + 集成 + 进程内端到端（171 个顶层用例）
go test -race ./... -count=1 -timeout 25m  # 数据竞态检测（需 CGO + gcc；本机无 gcc 时不可用）
bash scripts/smoke-e2e.sh   # 双真实节点进程：出块→同步→转账→打包→余额→真重启持久化（15 项）
```

覆盖要点：UTXO 状态机全错误路径、签名定长编码回归、难度调整边界、创世确定性、重启持久化、真实 TCP 传播/同步/拒绝无效块、种子重连、并行挖矿逐块共识校验、控制接口错误映射。

## 已知限制（明确不做）

以下能力超出学习项目边界，**有意未实现**：

- **分叉处理 / 链重组（reorg）**：单链追加式，收到父哈希不匹配的区块直接拒绝。设计原则上遵循最长链，但未实现树状链与重组。
- **难度浮动**：难度按设计固定在最低难度（上限 = 下限 = `16`），详见「难度为何不浮动」。放开浮动属独立共识参数阶段。
- **secp256k1 / RIPEMD160**：标准库限制，用 P-256 + SHA256 截断 20 字节替代；升级路径见代码注释。
- **SPV / 轻节点**：所有节点都是全节点。
- **TLS / 加密传输 / 对等认证**：P2P 明文传输，仅适合本地/可信网络。
- **代币经济模型**：无预挖、无分配，仅原生出块奖励。

## 文档

- `docs/README.md` — **文档索引**：文件名 ↔ 正文阶段号映射、权威/历史分级、推荐阅读顺序（先看这个）
- `docs/MASTER-DESIGN.md` — 剩余工程的总体设计决策记录
- `docs/PROJECT-COMPLETION-REPORT.md` — 项目完成报告（全量验收证据 + 未决项分级）
- `docs/DEVELOPMENT_PROMPT.md` — 模块任务卡片（原始开发提示词）
- `docs/FULL-IMPLEMENTATION-REPORT.md` — 全量实现报告（含各阶段验收证据）
- `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` — **确定性序列化规范**（第三方可不读源码实现解析/哈希/创世重建）
- `docs/PHASE-GENESIS-0-GENESIS-BLOCK-MINING-RUNTIME-VALIDATION.md` — 创世与挖矿实机验证（含独立复算证据）
- `docs/PHASE-GENESIS-0.1-P0-PERSISTENCE-REMEDIATION.md` — P0 持久化缺陷修复与回归
- `docs/PHASE-*.md` — 各阶段工程报告（含 `PHASE-BRAND-*` 产品/控制台阶段）
