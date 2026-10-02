# PHASE F-5 — REAL NODE VALIDATION REPORT

- **阶段**：PHASE F-5 — REAL NODE VALIDATION
- **性质**：真实节点生命周期验证（受控、隔离、最小规模）
- **对象**：`E:/wakuang/p2pchain`（P2PChain）
- **日期**：2026-09-27
- **前置**：F-1 → F-2 → F-3 → F-3A → F-3B → F-4 全部 PASS
- **判定**：

```
PHASE F-5 — VERDICT: PASS
```

---

## 1. BASELINE

| # | 项 | 实测 |
|---|---|---|
| 1 | HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| 2 | branch | `main` |
| 3 | `git status --porcelain`（`^ M`） | **37** |
| 4 | 真实内容修改（`git diff --name-only`） | **36** |
| 5 | CRLF/stat 伪差异 | **1** —— `internal/blockchain/query.go`（`git diff --quiet` 退出 0） |
| 6 | staged | **0** |
| 7 | untracked（开工） | **186** |
| 8 | `node.exe` SHA-256 | `3972843aff90428dd67acb39c6aaa009bd04d42982603617c5dbf6f0e8c213e1` |
| 9 | F-4 report SHA-256 | `e0e610fe68c9e24ee8bf1035c46d41570bd8730d4c11b46f88653d162a51fbd5` |

**未自行清理任何状态**：无 `git reset` / `checkout` / `clean` / `restore` / `stash`。
第 4/5 项的 36 vs 37 差异与 F-4 结论一致，**保持原样**。

---

## 2. SOURCE HEAD

```
4d892be355a38886a83c123faad915c9f3cd62e4
```

F-5 全程 HEAD 未变。**无 commit / amend / rebase / tag / merge / squash。**

---

## 3. VERIFICATION BINARY

| 项 | 值 |
|---|---|
| 路径 | `f5-verify/f5-node.exe` |
| 构建命令 | `go build -o f5-verify/f5-node.exe ./cmd/node` |
| `go build` | **exit 0** ✅ |
| `go vet ./...` | **exit 0** ✅ |
| 大小 | `12,065,792` B |

**未修改生产 binary**：`node.exe` SHA 全程保持 `3972843a…c213e1`（见 §17）。
F-5 全程使用**独立验证二进制**，从未执行 `node.exe`。

---

## 4. BINARY SHA

```
2a02814af98ee76a47ca24b61308aa16263e9489e0ecdf72ef87df0734db34fb   f5-verify/f5-node.exe
```

> **可复现性证据**：该 SHA 与 F-4 阶段构建的 `f4-verify/f4-node.exe` **逐字节相同**
> （`2a02814a…34fb`），即同一源码两次独立构建产出同一二进制。

---

## 5. FRESH DATADIR

| 项 | 值 |
|---|---|
| 路径 | `E:/wakuang/p2pchain/F5-real-node-validation-1` |
| 开工前存在性 | **不存在**（`ls -d` → exit 2）✅ |
| 创建方式 | `mkdir -p`（本阶段显式创建） |
| 创建后内容 | **空**（仅 `.` / `..`）✅ |

### 5.1 隔离性（**未使用**的目录）

```
❌ production datadir            ❌ run-a / run-b（legacy）
❌ F4-controlled-init-1          ❌ gui-test/
❌ 任何运行中 service 的数据目录  ❌ legacy chain 原始数据
```

**F-4 目录未被 F-5 触碰**：`F4-controlled-init-1/blocks.dat` 收工 SHA 仍为
`e517053e5ed51692a08e80c3c97f4fe79125f3a020e0a50d935250170113f3a0`（未变）。

---

## 6. INIT EVIDENCE

```
$ ./f5-verify/f5-node.exe init -datadir F5-real-node-validation-1
已初始化 canonical Genesis: 00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3

exit = 0   ✅
```

| 项 | 值 |
|---|---|
| `blocks.dat` exists | ✅ |
| `blocks.dat` size | **180** B |
| `blocks.dat` SHA-256 | `e517053e5ed51692a08e80c3c97f4fe79125f3a020e0a50d935250170113f3a0` |
| mtime | `2026-09-27 12:58:53.082617200 +0800` |
| Genesis = canonical | ✅（`00003d97…e4a3`） |
| 手工创建 blocks.dat | **否** ✅ |

> **确定性证据**：该 `blocks.dat` SHA 与 F-4 阶段独立初始化的产物**逐字节相同**
> （`e517053e…f3a0`）⇒ `init` 在全新目录上的输出是**确定性的**。

---

## 7. GENESIS IDENTITY

| 项 | 值 |
|---|---|
| EXPECTED（F-3A 冻结） | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| ACTUAL（F-5 新链） | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| MATCH | **YES** ✅ |
| LEGACY（对照） | `0000aca1af720da390380a38dc97d141cfb67afe578a2d31315860a5489db58c` |

### 7.1 创世区块完整事实（来自 `printchain`）

| 字段 | 值 |
|---|---|
| hash | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| previous_hash | `0000…0000`（全零） |
| merkle_root | `4d86b378a37c05f8963c781d0db121c901357c28ea7a01a027e79f531c7128ff` |
| timestamp | `1700000000`（= `GenesisTimestamp`） |
| bits | `16`（= `MaxTargetBits`） |
| nonce | `5390` |
| size | `176` B |
| transactions | 1（coinbase） |
| coinbase out[0] | 金额 **5**，收款公钥哈希 `0000…0001`（黑洞） |

---

## 8. PRE-START VERIFY

```
$ ./f5-verify/f5-node.exe verify -datadir F5-real-node-validation-1 -json
{
  "height": 0,
  "blocks_checked": 1,
  "genesis_hash": "00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3",
  "tip_hash":     "00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3",
  "valid": true,
  "fail_height": -1
}
exit = 0   ✅ PASS
```

---

## 9. REAL NODE STARTUP

```
$ ./f5-verify/f5-node.exe \
    -datadir F5-real-node-validation-1 \
    -listen 127.0.0.1:8902 \
    -rpc 127.0.0.1:8901 \
    -auth-token-file f5-verify/control-token \
    -mine -miners 1 -maxblocks 3
```

### 9.1 启动日志（原文）

```
2026/09/27 12:59:09 [node] 本地区块链已就绪: 高度=0 链尾=00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3 数据文件=F5-real-node-validation-1\blocks.dat
2026/09/27 12:59:09 [node] 已生成新钱包: F5-real-node-validation-1\wallet.json
2026/09/27 12:59:09 [node] 节点钱包地址=NXTawzj51MYwXsUdkG5yKpmRYyCmyCZ9v3
2026/09/27 12:59:09 [p2p] 节点已启动，监听 127.0.0.1:8902（nodeID=NXTawzj51MYwXsUdkG5yKpmRYyCmyCZ9v3）
2026/09/27 12:59:09 [node] mutation 端点认证已启用（token 文件: f5-verify/control-token）
2026/09/27 12:59:09 [node] 控制接口已启动: 127.0.0.1:8901
2026/09/27 12:59:09 [miner] 挖矿已启用，地址=NXTawzj51MYwXsUdkG5yKpmRYyCmyCZ9v3 上限=3 个区块
```

### 9.2 运行环境

| 项 | 值 |
|---|---|
| datadir | `F5-real-node-validation-1`（F-5 独立目录）✅ |
| binary | `f5-verify/f5-node.exe`（F-5 验证二进制）✅ |
| 是否 production service | **否** ✅ |
| 是否 GUI harness | **否** ✅ |
| 是否 legacy chain | **否** ✅ |
| 是否 F-4 目录 | **否** ✅ |
| P2P 监听 | `127.0.0.1:8902` |
| 控制接口 | `127.0.0.1:8901` |

**未修改**：difficulty / subsidy / consensus constants / mining algorithm / reward policy
—— 全部使用当前协议既有默认参数（`-miners 1` 仅为**最小规模**，非参数改动）。

---

## 10. NODE PID

| 轮次 | PID | 会话 | 内存 | 状态 |
|---|---|---|---|---|
| Run 1（挖矿） | **5680** | Console / 1 | 14,116 K | 已优雅停止 |
| Run 2（重启，全节点） | **22056** | Console / 1 | 13,872 K | 已优雅停止 |

**收工时无任何 `f5-node.exe` 残留进程** ✅

端口占用实测（Run 1 期间）：

```
TCP  127.0.0.1:8901  LISTENING  5680
TCP  127.0.0.1:8902  LISTENING  5680
```

---

## 11. MINING EVIDENCE

### 11.1 挖矿日志（真实 PoW）

```
[miner] MINING_POW_START 高度=1 打包交易=0 手续费=0 难度位=16
[miner] MINING_BLOCK_ACCEPTED 高度=1 哈希=0000aa9432d4799f5af8a1408983729ef694346848af87955a6c028f1fda6c16 交易数=1
[miner] MINING_POW_START 高度=2 打包交易=0 手续费=0 难度位=16
[miner] MINING_BLOCK_ACCEPTED 高度=2 哈希=0000ee9a8b6bb90ced31e87926021aab748baaec3a8bc19733c1d48dfc5d17ae 交易数=1
[miner] MINING_POW_START 高度=3 打包交易=0 手续费=0 难度位=16
[miner] MINING_BLOCK_ACCEPTED 高度=3 哈希=00009b2cd932f141add349f381960e44daebb33af0582a623c82ca7f6a6632f9 交易数=1
[miner] 已达到挖矿上限 3 个区块（当前高度 3），转为全节点模式
```

### 11.2 `/status`（运行中）

```json
{"height":3,
 "tip_hash":"00009b2cd932f141add349f381960e44daebb33af0582a623c82ca7f6a6632f9",
 "peers":[], "mempool_size":0, "mining":false,
 "address":"NXTawzj51MYwXsUdkG5yKpmRYyCmyCZ9v3",
 "mining_state":"STOPPED", "mining_reason":"max-blocks-reached",
 "pow_attempts":83946, "mining_retries":0,
 "accepted_blocks":3, "rejected_blocks":0,
 "bits":16, "difficulty":1}
```

| 项 | 值 | 说明 |
|---|---|---|
| `pow_attempts` | **83,946** | 真实 PoW 尝试次数（≈ 3 × 2^16 量级，与 bits=16 期望一致） |
| `accepted_blocks` | 3 | 全部被接受 |
| `rejected_blocks` | **0** | 无拒绝 |
| `mining_retries` | 0 | 无重试 |
| `mining_reason` | `max-blocks-reached` | 到达 `-maxblocks 3` 上限后停止（预期行为） |

---

## 12. BLOCK GROWTH

```
genesis (h=0)
   └─ h=1  0000aa9432d4799f5af8a1408983729ef694346848af87955a6c028f1fda6c16
        └─ h=2  0000ee9a8b6bb90ced31e87926021aab748baaec3a8bc19733c1d48dfc5d17ae
             └─ h=3  00009b2cd932f141add349f381960e44daebb33af0582a623c82ca7f6a6632f9
```

**height 0 → 1 → 2 → 3，真实增长** ✅

### 12.1 链式连接校验（`previous_hash` 逐块比对）

| 高度 | `previous_hash` | 与前一区块 hash 一致？ |
|---|---|---|
| 0 | `0000…0000` | ✅（创世） |
| 1 | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` | ✅ = genesis |
| 2 | `0000aa9432d4799f5af8a1408983729ef694346848af87955a6c028f1fda6c16` | ✅ = h1 |
| 3 | `0000ee9a8b6bb90ced31e87926021aab748baaec3a8bc19733c1d48dfc5d17ae` | ✅ = h2 |

### 12.2 全链区块字段（`printchain -limit 4 -tx`）

| h | hash | prev | timestamp | bits | nonce | txs | coinbase 金额 | coinbase 收款 |
|---|---|---|---|---|---|---|---|---|
| 0 | `00003d97…e4a3` | `0000…0000` | 1700000000 | 16 | 5390 | 1 | **5** | `0000…0001`（黑洞） |
| 1 | `0000aa94…6c16` | `00003d97…e4a3` | 1790485149 | 16 | 60341 | 1 | **5** | `7ea421b6…2e51` |
| 2 | `0000ee9a…17ae` | `0000aa94…6c16` | 1790485149 | 16 | 20964 | 1 | **5** | `7ea421b6…2e51` |
| 3 | `00009b2c…32f9` | `0000ee9a…17ae` | 1790485149 | 16 | 2638 | 1 | **5** | `7ea421b6…2e51` |

`consensus_era` = `pre-hardfork`（height < `ActivationHeight` 2000）。

---

## 13. STOP EVIDENCE

```
$ ./f5-verify/f5-node.exe stop -rpc 127.0.0.1:8901 -token-file f5-verify/control-token
已发送停止请求到 127.0.0.1:8901
节点已停止（控制接口 127.0.0.1:8901 不再响应；数据目录锁已释放）

exit = 0   ✅
```

节点侧日志：

```
2026/09/27 12:59:31 [node] 收到停止请求，正在关闭（释放数据目录锁）...
2026/09/27 12:59:31 [node] 正在关闭（释放数据目录锁）...
```

| 检查 | 结果 |
|---|---|
| exit code | **0** ✅ |
| 进程消失 | ✅（`tasklist` 无 `f5-node.exe`） |
| 端口释放 | ✅（仅 `TIME_WAIT`，无 `LISTENING`） |
| `node.lock` 释放 | ✅ **ABSENT** |
| `blocks.dat` size | **720** B（= 4 × 180） |
| `blocks.dat` SHA-256 | `d68c1568119b773b9fbfb37fb4b2580cf39e0a7487dd7af879f01445c8b55d84` |
| height | **3** |

**停止后离线 verify**：

```json
{"height":3,"blocks_checked":4,
 "genesis_hash":"00003d97…e4a3",
 "tip_hash":"00009b2c…32f9",
 "valid":true,"fail_height":-1}
exit = 0   ✅ PASS
```

---

## 14. RESTART EVIDENCE

```
$ ./f5-verify/f5-node.exe -datadir F5-real-node-validation-1 \
    -listen 127.0.0.1:8902 -rpc 127.0.0.1:8901 \
    -auth-token-file f5-verify/control-token          # 未启用挖矿
```

### 14.1 重启日志（原文）

```
2026/09/27 12:59:52 [node] 本地区块链已就绪: 高度=3 链尾=00009b2cd932f141add349f381960e44daebb33af0582a623c82ca7f6a6632f9 数据文件=F5-real-node-validation-1\blocks.dat
2026/09/27 12:59:52 [node] 节点钱包地址=NXTawzj51MYwXsUdkG5yKpmRYyCmyCZ9v3
2026/09/27 12:59:52 [p2p] 节点已启动，监听 127.0.0.1:8902（nodeID=NXTawzj51MYwXsUdkG5yKpmRYyCmyCZ9v3）
2026/09/27 12:59:52 [node] 控制接口已启动: 127.0.0.1:8901
2026/09/27 12:59:52 [node] 以全节点模式运行（未启用挖矿）；可用 `node stop` 停止节点
```

| 检查 | 结果 |
|---|---|
| startup | **PASS** ✅ |
| 高度恢复 | **3**（= 停止前）✅ |
| 链尾恢复 | `00009b2c…32f9`（= 停止前 tip）✅ |
| 钱包 | **加载既有**（同地址 `NXTawzj51…`；日志**无**「已生成新钱包」）✅ |
| 挖矿 | 未启用（预期）✅ |

### 14.2 重启后 `/status`

```json
{"height":3,
 "tip_hash":"00009b2cd932f141add349f381960e44daebb33af0582a623c82ca7f6a6632f9",
 "address":"NXTawzj51MYwXsUdkG5yKpmRYyCmyCZ9v3",
 "mining_state":"STOPPED","mining_reason":"init",
 "pow_attempts":0,"accepted_blocks":0,"rejected_blocks":0,
 "bits":16,"difficulty":1}
```

### 14.3 重启后创世身份（**直接证据**，非"启动成功即认为正确"）

```
$ curl -s "http://127.0.0.1:8901/blocks?from=0&count=1"
{"height":0,"hash":"00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3",
 "previous_hash":"0000000000000000000000000000000000000000000000000000000000000000",
 "timestamp":1700000000,"bits":16,"nonce":5390,
 "merkle_root":"4d86b378a37c05f8963c781d0db121c901357c28ea7a01a027e79f531c7128ff", ...}
```

---

## 15. PERSISTENCE EVIDENCE

### 15.1 重启完整性（before vs after）

| 项 | 停止前 | 重启后 | 结论 |
|---|---|---|---|
| height | **3** | **3** | **相等** ✅ |
| tip_hash | `00009b2c…32f9` | `00009b2c…32f9` | **相等** ✅ |
| Genesis hash | `00003d97…e4a3` | `00003d97…e4a3` | **相等** ✅ |
| wallet 地址 | `NXTawzj51…` | `NXTawzj51…` | **相等** ✅ |

### 15.2 `blocks.dat` 无预期外重写（关键）

| 时点 | SHA-256 | size | mtime |
|---|---|---|---|
| 第一次 stop 后 | `d68c1568119b773b9fbfb37fb4b2580cf39e0a7487dd7af879f01445c8b55d84` | 720 | `12:59:09.240554800` |
| 重启运行期间 | `d68c1568…5d84` | 720 | `12:59:09.240554800` |
| 第二次 stop 后 | `d68c1568…5d84` | 720 | `12:59:09.240554800` |

⇒ **SHA / size / mtime 三者全同**：重启 + 再次停止**完全未改写** `blocks.dat`（只读回放）✅

### 15.3 全生命周期后最终 verify

```json
{"height":3,"blocks_checked":4,
 "genesis_hash":"00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3",
 "tip_hash":"00009b2cd932f141add349f381960e44daebb33af0582a623c82ca7f6a6632f9",
 "valid":true,"fail_height":-1,"fail_hash":"","reason":""}
exit = 0   ✅ PASS
```

---

## 16. NEGATIVE LEGACY TEST

**输入 = legacy chain COPY（非 production 原始数据）**

```
$ cp run-a/blocks.dat F5-legacy-copy-1/blocks.dat
```

| 项 | 值 |
|---|---|
| 路径 | `F5-legacy-copy-1/blocks.dat` |
| size | `226800` B |
| SHA-256（基线） | `893e64c12598558ff93793248d363536e1d090a6970c2d12ed632af3eea2fb2f` |

### 16.1 真实 node 启动（负向）

```
$ ./f5-verify/f5-node.exe -datadir F5-legacy-copy-1 -listen 127.0.0.1:0 -rpc 127.0.0.1:0
2026/09/27 13:00:19 [node] 加载区块链失败: GENESIS_MISMATCH: expected 00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3, got 0000aca1af720da390380a38dc97d141cfb67afe578a2d31315860a5489db58c

exit = 1   ✅
```

| 要求 | 实测 | 结论 |
|---|---|---|
| fail closed | exit 1，无服务启动 | ✅ |
| 失败高度 = 0 | 错误为**创世身份**失败，replay 未开始 | ✅ |
| 不进入 replay | ✅（身份检查先于 ApplyBlock） | ✅ |
| legacy `blocks.dat` SHA 不变 | `893e64c1…ea2fb2f`（size 226800 / mtime 未变） | ✅ |
| 未使用 production legacy data | 使用**副本** ✅ | ✅ |

### 16.2 对照（F-1 缺陷的最终闭合形态）

| | 旧行为 | F-5 实测 |
|---|---|---|
| 错误 | `创世区块 UTXO 初始化失败: coinbase 输出 50 > 奖励 5 + 手续费 0` | `GENESIS_MISMATCH: expected 00003d97…, got 0000aca1…` |
| 语义 | 伪装成**经济共识**错误、**方向依赖** | **显式身份失败** |
| 时点 | 高度 0 | **replay 之前** |

---

## 17. PRODUCTION PROTECTION

### 17.1 开工 vs 收工 SHA-256（全部一致）

| 对象 | SHA-256 | 结论 |
|---|---|---|
| `node.exe` | `3972843aff90428dd67acb39c6aaa009bd04d42982603617c5dbf6f0e8c213e1` | **未变** ✅ |
| `run-a/blocks.dat` | `893e64c12598558ff93793248d363536e1d090a6970c2d12ed632af3eea2fb2f` | **未变** ✅ |
| `run-b/blocks.dat` | `5ab67bb4f2787fb11555135671379269818c3fc43fff20057a8743c4f4e1df53` | **未变** ✅ |
| `run-a/wallet.json` | `bf577525dcab4f2584812b357b82248435d5a8b85e84cf6cb8218c9d86844a94` | **未变** ✅ |
| `run-b/wallet.json` | `762c369374117141fc18a2b687df7e81aed5b8a385f449a8a4a9128a6faa5190` | **未变** ✅ |
| `PHASE-F4-…-REPORT.md` | `e0e610fe68c9e24ee8bf1035c46d41570bd8730d4c11b46f88653d162a51fbd5` | **未变** ✅ |

### 17.2 其它完整性

| 项 | 开工 | 收工 | 结论 |
|---|---|---|---|
| HEAD | `4d892be` | `4d892be` | 未动 ✅ |
| 真实内容修改 | 36 | 36 | 未动 ✅ |
| porcelain `^ M` | 37 | 37 | 未动 ✅ |
| staged | 0 | 0 | 未动 ✅ |
| untracked | 186 | **188**（+2 = `f5-verify/` 与本报告） | 仅新增验证产物与交付物 |
| `F4-controlled-init-1/blocks.dat` | `e517053e…f3a0` | `e517053e…f3a0` | 未动 ✅ |

**任何变化均未发生 ⇒ 未触发 HARD STOP。**

---

## 18. SHA EVIDENCE

### 18.1 本阶段产物

| 对象 | SHA-256 |
|---|---|
| `f5-verify/f5-node.exe`（12,065,792 B） | `2a02814af98ee76a47ca24b61308aa16263e9489e0ecdf72ef87df0734db34fb` |
| `F5-real-node-validation-1/blocks.dat`（init 后，180 B） | `e517053e5ed51692a08e80c3c97f4fe79125f3a020e0a50d935250170113f3a0` |
| `F5-real-node-validation-1/blocks.dat`（挖矿后，720 B） | `d68c1568119b773b9fbfb37fb4b2580cf39e0a7487dd7af879f01445c8b55d84` |
| `F5-legacy-copy-1/blocks.dat`（226,800 B） | `893e64c12598558ff93793248d363536e1d090a6970c2d12ed632af3eea2fb2f` |
| 本报告 | 见 §20.2 自引用说明 |

### 18.2 上游产物（未变）

| 产物 | SHA-256 |
|---|---|
| F-1 report | `d44791da1ffc029ed1f1d8666f732e7705b73dfbb2f4d2c867d0b559074c7ded` |
| F-2 report | `001c48655c7ff693809420240d254abdfa109f8272cf9c0282ccbc20023bbdb9` |
| F-3 report | `bbced9a80d8cb778e529be5c38defb478ef42679587c9b148375dbeb8f0aaa6f` |
| F-3A report | `bccdfb17d8cbe0d8ed01f72f800f7025a7a8bf39c3f3692f52d220fbcf7159c3` |
| F-3B report | `4166d119a47ca85a03a2beb9278387071b62098554ecf883834e339f810fda5a` |
| F-4 report | `e0e610fe68c9e24ee8bf1035c46d41570bd8730d4c11b46f88653d162a51fbd5` |

### 18.3 基线指纹

| 常量 | 值 |
|---|---|
| HEAD COMMIT | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| CANONICAL GENESIS HASH | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| LEGACY GENESIS HASH | `0000aca1af720da390380a38dc97d141cfb67afe578a2d31315860a5489db58c` |
| F-5 BINARY | `2a02814af98ee76a47ca24b61308aa16263e9489e0ecdf72ef87df0734db34fb` |

---

## 19. KNOWN LIMITATIONS

### 19.1 【Finding · 低-中】mutation auth token 文件未被 `.gitignore` 覆盖

| 项 | 内容 |
|---|---|
| Finding | `F-5-FINDING-1` |
| 事实 | `f5-verify/control-token`（mutation 端点 Bearer Token）**不被 `.gitignore` 忽略**；`git check-ignore` exit 1。且节点**默认** token 路径 `secrets/control-token` 同样未被忽略 |
| 触发条件 | 仅当有人执行 `git add -A` / `git add f5-verify` 时才会进入暂存区 |
| 现有防护 | `.gitignore` 已覆盖 `wallet.json` / `*.lock` / `*.dat` / `*.exe`，但**未覆盖 token 文件**（GOV-1 边界未含此项） |
| 实际风险 | **低**（token 仅对本机回环控制接口有效；节点已停止；文件为 0600 意图、untracked） |
| 本阶段处置 | **不修改**（改 `.gitignore` 会使受跟踪修改数由 36 变为 37，违反 F-5 基线与"不得自行清理"约束） |
| 建议阶段 | 专门的 Governance / Git 边界阶段（与 `F3B-CONSEQUENCE-1` 同批处理） |
| 建议修法 | `.gitignore` 增加 `secrets/`、`*control-token*`、`*-token`（需 Owner 授权） |

> **旁证**：F-4 的 `f4-verify/` 目录对 `git status` 不可见（其内容仅 `*.exe`，被忽略）；
> F-5 的 `f5-verify/` 因含 `control-token` 与 `*.log` 而**可见**，这正是该 Finding 被暴露的原因。

### 19.2 其它限制（设计使然，非缺陷）

| # | 限制 | 说明 |
|---|---|---|
| 1 | 单节点 | F-5 明确排除 P2P multi-node / chain sync / fork-reorg（§16） |
| 2 | 挖矿规模极小 | 3 个区块、`-miners 1`；**不是**压力测试 |
| 3 | 难度恒为 1 | `bits=16`，因 height < `ActivationHeight=2000`，难度调整未激活（预期） |
| 4 | coinbase 未成熟 | 高度 3 < `CoinbaseMaturity=10`，故未验证成熟后可花费性（超出 F-5 范围） |
| 5 | 钱包由节点启动创建 | `wallet.json`（388 B）由 node `LoadOrCreate` 生成，**非** `init` 产物（与 F-4 一致） |
| 6 | 无 mempool/交易 | 全部区块仅含 coinbase，`mempool_size=0`（超出 F-5 范围） |
| 7 | 时间戳为当前时间 | 块 1–3 的 timestamp = 1790485149（挖矿时刻），非创世固定值（预期） |

### 19.3 未暴露任何需 STOP 的问题

F-5 期间**未发现** subsidy 问题 / genesis 参数问题 / persistence 问题 / mining 问题 / node lifecycle 问题。
唯一登记项为 §19.1 的**治理类** Finding（非协议、非实现缺陷）。

---

## 20. VERDICT

### 20.1 PASS 判定逐条

| # | 条件 | 实测 | 结论 |
|---|---|---|---|
| 1 | BUILD | `go build` exit 0，`go vet ./...` exit 0 | ✅ PASS |
| 2 | INIT | exit 0，`blocks.dat` 180 B，genesis canonical | ✅ PASS |
| 3 | VERIFY | valid=true，exit 0（启动前 / 停止后 / 全生命周期后 各一次） | ✅ PASS |
| 4 | REAL NODE START | PID 5680，双端口 LISTENING，日志正常 | ✅ PASS |
| 5 | GENESIS IDENTITY | 运行中 `/blocks` 返回 block0 = `00003d97…e4a3`（直接证据） | ✅ PASS |
| 6 | REAL BLOCK PRODUCTION | 真实 PoW（83,946 attempts），h0→h3，accepted 3 / rejected 0 | ✅ PASS |
| 7 | STOP | exit 0，进程退出，端口释放，锁释放，`node.lock` 无残留 | ✅ PASS |
| 8 | RESTART | 同 binary + 同 datadir 启动成功 | ✅ PASS |
| 9 | PERSISTENCE | height 3→3，tip 相同，genesis 相同，`blocks.dat` SHA/size/mtime 全同 | ✅ PASS |
| 10 | LEGACY FAIL-CLOSED | `GENESIS_MISMATCH`，exit 1，height 0，无 replay，副本 SHA 不变 | ✅ PASS |
| 11 | PRODUCTION PROTECTION | 6/6 SHA 未变；HEAD / 36 修改 / staged 0 未变 | ✅ PASS |

```
PHASE F-5 — VERDICT: PASS
```

### 20.2 报告自引用说明

本报告为**自引用文档**，其 SHA-256 **不能自含**。收工后由独立命令计算并登记于文件之外：

```
最终摘要 = sha256sum PHASE-F5-REAL-NODE-VALIDATION-REPORT.md
```

---

## 21. NEXT-PHASE RECOMMENDATION

### 必须做

1. **Owner 单独授权下一阶段** —— F-5 不自动推进。
2. **决定新链数据目录的正式归属**：`F5-real-node-validation-1` 是**验证用**目录；
   正式链目录需明确指定（F-4/F-5 已证明 `init` 在任意全新目录上确定性产出同一创世）。
3. **提交组织**：36 个受跟踪修改 + F-1..F-5 六份报告仍在工作树未提交；
   须走专门 COMMIT 阶段（F-5 禁止 commit）。

### 应后置

| 项 | 建议阶段 |
|---|---|
| `F-5-FINDING-1`（token 文件未忽略）+ `F3B-CONSEQUENCE-1`（GUI 夹具） | Governance / Git 边界阶段（同批） |
| README 经济参数过期（第 165 行 `50 >> (height/210)`；"难度为何不浮动"整节） | 文档阶段 |
| P2P `genesis_hash` 校验（OD-08 DEFERRED） | P2P DISCOVERY / RESILIENCE PHASE |
| 难度调整激活验证（height ≥ 2000） | 长链 / 容量阶段 |

### 可选

- 把 F-4 + F-5 的验收流程固化为**可重复脚本**（当前均为一次性人工执行）；
- 将创世 180 B 与 720 B 基线哈希纳入回归哨兵。

### 明确不做

```
❌ 不自动进入下一阶段
❌ 不启动 P2P multi-node / chain sync / fork-reorg
❌ 不做 mempool / fee policy / mining fairness / network capacity / security audit
❌ 不做 wallet implementation / GUI migration / production deployment
❌ 不修改协议 / Genesis / 经济参数
❌ 不 commit / push
```

### 下一阶段是否保持原计划

**原计划成立，且 F-5 已为其扫清前置。**
F-5 已用真实证据闭合以下链条：

```
CANONICAL GENESIS (00003d97…e4a3)
        ↓
EXPLICIT INIT (exit 0, 180 B, 确定性)
        ↓
IDENTITY VERIFICATION (verify PASS ×3)
        ↓
REAL NODE (PID 5680, 8901/8902)
        ↓
REAL BLOCK (h1/h2/h3, PoW 83,946 attempts, coinbase 5)
        ↓
PERSISTENCE (720 B, SHA d68c1568…)
        ↓
RESTART (PID 22056, height 3, tip 同)
        ↓
SAME CHAIN IDENTITY (genesis 00003d97…e4a3 不变)
```

⇒ 后续阶段（P2P 发现/韧性、容量模型、提交治理等）可按原计划推进，**但须 Owner 逐项单独授权**。

---

## HARD STOP

```
PHASE F-5 COMPLETE
VERDICT: PASS

NEXT PHASE AUTHORIZATION REQUIRED
```

**不要自动进入下一阶段。**

---

*报告结束。本报告为 F-5 真实节点验证阶段的执行产物，记录基线、命令、证据与判定，不构成任何后续阶段的授权。*
