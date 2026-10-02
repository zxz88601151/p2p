# PHASE F-3B — PROTOCOL IDENTITY IMPLEMENTATION EXECUTION

**阶段性质**：IMPLEMENTATION EXECUTION（写入阶段 —— 按 F-3A 冻结契约实施）
**前置阶段**：PHASE F-2（契约冻结）→ PHASE F-3（就绪性审计）→ PHASE F-3A（Owner 决策 + 契约冻结）
**当前 HEAD**：`4d892be355a38886a83c123faad915c9f3cd62e4`（未提交，工作树修改）
**本阶段判定**：

```
PHASE F-3B STATUS:
BASELINE:                   PASS
SCOPE COMPLIANCE:           7/7（无扩大）
IMPLEMENTATION:             COMPLETE
REGRESSION:                 PASS（16/16 包）
E2E ACCEPTANCE:             PASS（6/6 真实数据场景）
PROTECTED OBJECTS:          UNCHANGED
HARD STOP:                  YES（→ F-4 须单独授权）
```

---

## §1 BASELINE

| # | 检查项 | 期望 | 实测 | 结论 |
|---|---|---|---|---|
| 1 | HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` | 同 | ✅ |
| 2 | F-3A 契约报告 | `PHASE-F3A-GENESIS-IDENTITY-CONTRACT-FREEZE.md` | 存在 | ✅ |
| 3 | 期望创世哈希（身份锚点） | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` | 同 | ✅ |
| 4 | `node.exe`（受保护，唯一可读旧链的二进制） | `3972843a…c213e1` | 同 | ✅ |
| 5 | `run-a/blocks.dat` | `893e64c1…ea2fb2f` | 同 | ✅ |
| 6 | `run-b/blocks.dat` | `5ab67bb4…e1df53` | 同 | ✅ |
| 7 | `run-a/wallet.json` | `bf577525…6844a94` | 同 | ✅ |
| 8 | `run-b/wallet.json` | `762c3693…aa5190` | 同 | ✅ |

```
BASELINE PASS   ——  未触发 STOP — BASELINE DRIFT
```

> **会话连续性说明**：本阶段的实现代码在**上一次会话中断前**已写入工作树
> （最后写入时刻 10:59），但**未完成验证、未收敛测试、未出报告**。
> 本次会话的任务是：**核验实现是否完整、补齐被中断的回归收敛、执行端到端验收、出报告**。
> 实现代码本身未在本轮被重写（仅补齐回归测试夹具 + 修正 1 处注释事实错误）。

---

## §2 SCOPE COMPLIANCE（F-3A §13.1 的 7 项，逐项核对）

| # | F-3B 冻结范围 | 落地位置 | 状态 |
|---|---|---|---|
| 1 | **Genesis hash pin**（`00003d97…e4a3`） | `internal/blockchain/genesis.go` → `CanonicalGenesisHash` | ✅ |
| 2 | **Canonical identity gate**（单一 primitive，`node`+`verify` 共用） | `internal/blockchain/identity.go` → `VerifyGenesisIdentity` | ✅ |
| 3 | **`GENESIS_MISMATCH` 错误语义落地** | `identity.go` → `ErrGenesisMismatch`（标识符由本阶段决定） | ✅ |
| 4 | **`p2pchain init` 显式初始化命令** | `cmd/node/cli.go` → `cmdInit` | ✅ |
| 5 | **移除 `NewBlockchainFromStore` 自动创世路径** | `internal/blockchain/blockchain.go` | ✅ |
| 6 | **STATE C / D 的 FAIL CLOSED** | `cmd/node/main.go` + `identity.go` + `cli.go` | ✅ |
| 7 | **Regression / compatibility tests** | `cmd/node/f3b_identity_test.go`（新）+ 22 个既有真实子进程用例收敛 | ✅ |

**未扩大至**：P2P ✅ 未改 / wallet ✅ 未改 / GUI ✅ 未改 / 经济模型 ✅ 未改 / 生产部署 ✅ 未执行。

---

## §3 IMPLEMENTATION DETAIL

### 3.1 身份闸门与哨兵错误 —— `internal/blockchain/identity.go`（新增）

```go
ErrUninitializedStore     // 数据目录尚无持久化创世
ErrGenesisMismatch        // = "GENESIS_MISMATCH"：数据集来自另一条链
ErrCorruptGenesisIdentity // 持久化创世无法作为身份读取
ErrAlreadyInitialized     // init 拒绝覆盖既有数据集
ErrCanonicalGenesisDrift  // 运行时创世定义与 pin 不一致（发布级阻断）
```

`VerifyGenesisIdentity(store)` 是**唯一**的 canonical identity gate：

- `Height() < 0` → `ErrUninitializedStore`
- 读不出创世 → `ErrCorruptGenesisIdentity`（**STATE D**）
- 创世哈希 ≠ `CanonicalGenesisHash` → `ErrGenesisMismatch`（**STATE C**，含 expected/got 双向明文）
- 一致 → 返回创世（**STATE B**）

**不做 consensus replay、绝不写存储。** 与 §7.2 的要求一致：身份失败**不复用**
`ErrCorruptStore` / `ErrDatadirLocked` / `ErrExcessiveCoinbase` / 通用 I/O error。

另含两个显式入口：

- `InitializeBlockchainStore(store)` —— **唯一的创世创建入口**。空库才可调用；
  创建前先自校验 `NewGenesisBlock().Hash() == CanonicalGenesisHash`，不一致即
  `ErrCanonicalGenesisDrift`（把「改补贴即改创世」这一缺陷**变成显式阻断**）。
- `NewBlockchainFromStoreForTest(store)` —— **仅供测试夹具**保留旧便利语义
  （空库时先 `InitializeBlockchainStore`）。生产路径不引用。

### 3.2 创世 pin —— `internal/blockchain/genesis.go`

新增 `CanonicalGenesisHash [32]byte` 与 `CanonicalGenesisHashHex()`。
该值**不是新发明的 Chain ID / Protocol Version**（OD-02 禁令保持），
而是对既有确定性 `NewGenesisBlock()` 输出的**显式固化**，注释中写明
「任何影响 canonical genesis 的共识修改都必须先经兼容性审计与协议身份决策」。

> 顺带修正 1 处**事实错误的注释**：`MaxTargetBits=20 期望约 2^20` → `MaxTargetBits=16 期望约 2^16`
> （`internal/pow/pow.go:29` 实为 `MaxTargetBits = 16`）。纯注释，无行为变化。

### 3.3 移除自动创世 —— `internal/blockchain/blockchain.go`

`NewBlockchainFromStore` 的 `h < 0` 分支由
「`NewGenesisBlock()` + `SaveBlock` + 返回」改为 **`return nil, ErrUninitializedStore`**；
创世读取由 `store.GetBlockByHeight(0)` 改为 **`VerifyGenesisIdentity(store)`**，
即**身份校验先于逐块 replay**（OD-07）。

### 3.4 共用闸门 —— `internal/blockchain/verify.go`

`VerifyStoredChain` 同样先调 `VerifyGenesisIdentity`，失败时
`FailHeight = 0` + 创世哈希 + `GENESIS_MISMATCH` 原因，**不进入 replay**。
⇒ `node` 与 `verify` 两条生产链构造路径**共用同一 primitive**（OD-05 / §6.3）。

### 3.5 存储层：禁止「打开即修复」—— `internal/storage/file.go`

新增 `OpenFileBlockStoreStrict(dir)`（`repairOnOpen=false`）：
打开阶段遇到任何截断/损坏**一律 fail closed，不修改 `blocks.dat`**。
`OpenFileBlockStore` 保留旧的 EOF 截断修复语义供历史调用方使用。
**节点启动与显式 `init` 使用 Strict 版本** —— 保证身份检查之前不发生隐式修复。

### 3.6 启动四态状态机 —— `cmd/node/main.go`

在**已取得目录锁之后、打开存储之前**插入显式判定：

| 磁盘状态 | 行为 | 对应 STATE |
|---|---|---|
| 无 `blocks.dat` | `ErrUninitializedStore`（fail closed，**不自动创世**） | A（须显式 `init`） |
| `blocks.dat` 存在且 **0 字节** | `ErrCorruptGenesisIdentity`（**按 CORRUPTED 处理**，非 EMPTY） | D（C-1 约束） |
| `blocks.dat` 存在且非空 | 交 `VerifyGenesisIdentity` | B / C |

### 3.7 显式初始化命令 —— `cmd/node/cli.go`

新增子命令 **`init`**（`cmdInit`）：

- 全新/空目录 → 创建 canonical Genesis，输出哈希，退出码 0
- 目录非空但**缺 `blocks.dat`** → 拒绝（「拒绝将其当作空库初始化」）
- `blocks.dat` **0 字节** → `ErrCorruptGenesisIdentity`，拒绝
- 已初始化 → `ErrAlreadyInitialized`，拒绝
- 外来创世 → `ErrGenesisMismatch`，拒绝
- **任何拒绝路径都不得改写既有 `blocks.dat`**（已实测哈希不变）

该命令**不加载钱包、不启动网络、不挖矿**；运行时 `node` 启动路径**永不**调用它。

---

## §4 VERIFICATION EVIDENCE

### 4.1 构建 / 静态检查

```
$ go build ./...                        → EXIT=0
$ go vet ./internal/blockchain/ ./cmd/node/  → EXIT=0
$ gofmt -l <F-3B 触及文件>              → 空（干净）
```

### 4.2 契约状态矩阵测试 —— `cmd/node/f3b_identity_test.go`

```
$ go test -count=1 -run TestF3B -v ./cmd/node/
--- PASS: TestF3BExplicitInitializationStateMatrix (1.47s)
    --- PASS: .../empty_node_fails_closed
    --- PASS: .../empty_verify_fails_closed
    --- PASS: .../empty_init_creates_canonical_genesis
    --- PASS: .../matching_node_and_verify_continue
    --- PASS: .../mismatch_node_and_verify_fail_closed_before_replay
    --- PASS: .../corrupt_and_zero-byte_datasets_fail_closed/{corrupt,zero-byte}
    --- PASS: .../init_refuses_initialized,_foreign,_and_zero-byte_datasets
ok  p2pchain/cmd/node  1.490s
```

覆盖：STATE A/B/C/D 全部分支 + init 的 4 类拒绝 + 「拒绝后 `blocks.dat` 逐字节不变」。

### 4.3 端到端 CLI 验收（真实二进制，**独立于 `node.exe`**）

产物：`f3b-verify/f3b-node.exe`（`go build -o … ./cmd/node`，12,065,792 B）。
**`node.exe` 未被覆盖**（哈希保持 `3972843a…c213e1`）。

| # | 场景 | 命令 | 实测 | 结论 |
|---|---|---|---|---|
| 1 | 全新目录初始化 | `init -datadir <fresh>` | `已初始化 canonical Genesis: 00003d97…e4a3`，exit 0 | ✅ |
| 2 | 重复初始化 | `init`（同目录） | exit 1，`data directory is already initialized；拒绝覆盖现有数据集` | ✅ |
| 3 | 初始化后校验 | `verify -datadir <fresh>` | PASS，高度 0→0，创世 `00003d97…e4a3` | ✅ |
| 4 | 空目录启动节点 | `node -datadir <empty>` | exit 1，`加载区块链失败: data directory is not initialized` | ✅ |
| 5 | **旧链启动节点** | `node -datadir <legacy-copy>` | exit 1，`加载区块链失败: GENESIS_MISMATCH: expected 00003d97…e4a3, got 0000aca1…b58c` | ✅ |
| 6 | **旧链执行 init** | `init -datadir <legacy-copy>` | exit 1，`GENESIS_MISMATCH …；拒绝覆盖现有数据集` | ✅ |

**关键对照（F-1 缺陷已闭合）**：同一份 `run-a` 数据

- 旧行为：`创世区块 UTXO 初始化失败: coinbase 输出超过区块奖励加手续费上限: coinbase 输出 50 > 奖励 5 + 手续费 0`
  —— 伪装成**经济共识错误**、发生在**高度 0**、且**方向依赖**（F-3 B-3）。
- 新行为：`GENESIS_MISMATCH: expected … got …` —— **显式身份失败**，在 **ApplyBlock 之前**。

### 4.4 兼容性：`printchain` 原始转储路径不受影响

`printchain` **不是** chain-construction 路径（F-3A §6.4 已界定），不经过身份闸门：

```
$ f3b-node.exe printchain -datadir <legacy-copy> -limit 2   → 高度 1259 正常转储，exit 0
$ f3b-node.exe printchain -datadir <fresh>       -limit 2   → 高度 0 正常转储，exit 0
```

### 4.5 只读性证明（旧数据零改写）

```
$ sha256sum f3b-verify/lab-legacy/blocks.dat
893e64c12598558ff93793248d363536e1d090a6970c2d12ed632af3eea2fb2f   ← 与 run-a/blocks.dat 逐字节相同
```

经过 `verify` + `node` 启动失败 + `init` 拒绝 + `printchain` 四轮访问后，
旧链副本**未被写入 1 个字节** ⇒ STATE C/D 的
「禁止 replay / migration / reset / delete / overwrite / regenerate / auto-repair」全部满足。

---

## §5 REGRESSION 收敛（本阶段补齐的被中断工作）

### 5.1 问题

移除自动创世后，**22 个真实子进程用例**失败（`go test ./cmd/node/` → FAIL）：

```
TestVerifyWithDatadirBeforeCommand / TestVerifyWithDatadirAfterCommand
TestPrintchainWithDatadirBeforeAndAfter (×2) / TestResetWithDatadirBeforeAndAfter (×2)
TestNodeStartupUnaffected
TestCLIHintPresentOnLocked/stale_lock_file_is_taken_over
TestRealProcessPairConvergesToSameTip / TestRealProcessForkReorgConvergesByHash
TestR3_RealProcessCrashRestartLegacyPrefix
TestR4B_DualProcessCompetitiveForkConverges (×3) / TestR4B_ProductionReorgPathTraversed
TestGracefulStopExitsAndReleasesLock / TestStopPreservesChainAndWallet
TestStopThenRestartNeedsNoForce / TestStopThenVerifyConsistent / TestStopWhileMining
TestStopAfterTransaction / TestStopAfterP2PActivity / TestRepeatedStartStop
TestResetAfterGracefulStopNeedsNoForce / TestResetRefusesWhenStopFailed
```

**统一根因**：这些用例用真实二进制在**空 `t.TempDir()`** 上启动节点
（含离线建链辅助 `mineChainOffline` → `runOffline(t, nil, n)`），
F-3B 后空目录 **fail closed**，节点在就绪前即以 exit 1 退出：
`加载区块链失败: data directory is not initialized`。

> 注：F-3B 的夹具迁移已覆盖**进程内**路径（`newNodeRuntimeForTest` /
> `NewBlockchainFromStoreForTest`），但**真实子进程路径**（`startRealNode`）漏改，
> 即本次补齐的部分。

### 5.2 修复（仅测试夹具，无生产代码改动）

`cmd/node/stop_lifecycle_test.go` 新增 `ensureTestDataDir(t, dir)` 并在
`startRealNode` 入口调用：

- **`blocks.dat` 不存在** → 走**真实 `cmdInit`**（等价 `p2pchain init`）显式初始化
- **`blocks.dat` 已存在** → **原样保留**（canonical / legacy / 外来创世交给被测闸门判定）

这一「条件初始化」是刻意的：它既让空目录用例恢复可用，
又**不掩盖**任何用例自行构造的身份场景（legacy 前缀 / 外来创世仍会被闸门拦下）。

`cmd/node/lock_lifecycle_test.go` 的 `stale_lock_file_is_taken_over` 子用例
（该用例直接 `exec.Command` 起进程，不走 `startRealNode`）同样补 `ensureTestDataDir`
—— 其断言的是「正常启动 + 接管残留锁」语义，必须先把目录显式初始化。

### 5.3 结果

```
$ go test ./cmd/node/                        → ok  p2pchain/cmd/node  569.514s   （22 FAIL → 0 FAIL）
$ go test -timeout 1800s ./...               → 16/16 包 ok，0 FAIL
      cmd/node 574.375s | storage 273.5s | blockchain 67.2s | mempool 31.9s
      p2p 15.2s | control 4.2s | wallet 3.4s | 其余 <1s
$ go test -count=1 ./internal/blockchain/    → ok  57.463s（最终修订版强制非缓存复验）
$ go test -count=1 -run TestF3B ./cmd/node/  → ok  1.490s
```

**16/16 包全 PASS，无 FAIL。**

---

## §6 与 F-3A 冻结契约的逐条对照

| 契约项 | 要求 | 实现 | 结论 |
|---|---|---|---|
| OD-01 = A | 继续以 `blocks.dat[0]` 作唯一 identity carrier | 未引入任何 metadata / sidecar / manifest | ✅ |
| OD-02 | 禁止发明 Chain ID / Protocol Version / Network ID | 全仓**未新增**这些标识符；`CanonicalGenesisHash` 是既有创世输出的固化 | ✅ |
| OD-03 | 禁止自动创世 + 命令名 `p2pchain init` | `init` 子命令；`node` 路径 `ErrUninitializedStore` | ✅ |
| OD-04 | 四态状态机 | A/B/C/D 全部实测（§4.2） | ✅ |
| OD-05 | 单一 canonical identity gate | `VerifyGenesisIdentity`，`node`+`verify` 共用 | ✅ |
| OD-06 | `GENESIS_MISMATCH` 语义冻结 | `ErrGenesisMismatch = errors.New("GENESIS_MISMATCH")` | ✅ |
| OD-07 | identity 校验先于 consensus replay | 两路径均在 replay 前返回；失败高度 = 0 | ✅ |
| OD-08 | P2P 校验 DEFERRED | **未改 P2P**（`grep .GenesisHash ==/!=` 仍无比较） | ✅ |
| OD-09 | LEGACY → FORENSIC ARCHIVE | `run-a`/`run-b` 原地未动（哈希一致）；无迁移/删除 | ✅ |
| OD-10 | WALLET QUARANTINED | 未导入/签名/复制/修改任何 `wallet.json` | ✅ |
| C-1 | 0 字节 `blocks.dat` 按 CORRUPTED 处理 | `main.go` + `cmdInit` 双路径 fail closed | ✅ |

---

## §7 后果登记与后续项（**不在 F-3B 范围内**）

### 7.1 【必须跟进】GUI 全功能测试夹具需适配新启动契约

- **事实**：`gui/_fulltest.py` 第 76 行 `reset_dirs()` 会删除 `gui-test/data-a`、`data-b`，
  随后第 2 节在**空目录**上启动节点；F-3B 后该启动**必然 fail closed**。
- **性质**：这是**测试夹具**对契约变更的适配（与 §5 的 Go 夹具同类），**不是 GUI 功能缺陷**。
- **为何不在本阶段修**：F-3A §13.1 明文规定「F-3B **不得扩大至** … GUI …」。
- **精确修法**（供后续阶段/授权后一行落地）：在 `_fulltest.py` 启动节点前，
  对每个自建数据目录先执行一次 `run_cli(BIN, ["init", "-datadir", DATA_A])`（幂等，已初始化时返回非 0 可忽略）。
- **登记**：`F3B-CONSEQUENCE-1`。

### 7.2 【沿用 F-3A】P2P `genesis_hash` 仍只广播不校验

`OD-08 = DEFERRED TO P2P DISCOVERY / RESILIENCE PHASE`。本阶段**未触碰** P2P。
在 P2P 阶段落地前，「网络身份」仍是**声明性**而非**强制性**的（F-3A §11.1 残留风险，不阻断 F-3B）。

### 7.3 【沿用 F-1/F-3A】README 经济参数描述仍过期

`README.md` 第 165 行 `50 >> (height/210)` 与实际 `5 / 5_250_000` 不符；
「难度为何不浮动」整节已过期（实际 `MaxDifficultyBits = 32`）。
属文档缺陷，与 F-1 决策绑定，**建议随 F-4 一并更新**。

### 7.4 【环境】`audit-run/`、`f3b-verify/`、`gui-test/`、`F-1-lab/` 为可清理的验证产物

均为未跟踪目录，不含受保护对象。`f3b-verify/f3b-node.exe` 是本阶段验收用二进制。

---

## §8 受保护对象（本阶段结束时核验）

| 对象 | SHA-256 | 状态 |
|---|---|---|
| `node.exe` | `3972843aff90428dd67acb39c6aaa009bd04d42982603617c5dbf6f0e8c213e1` | 未变 |
| `run-a/blocks.dat` | `893e64c12598558ff93793248d363536e1d090a6970c2d12ed632af3eea2fb2f` | 未变 |
| `run-b/blocks.dat` | `5ab67bb4f2787fb11555135671379269818c3fc43fff20057a8743c4f4e1df53` | 未变 |
| `run-a/wallet.json` | `bf577525dcab4f2584812b357b82248435d5a8b85e84cf6cb8218c9d86844a94` | 未变 |
| `run-b/wallet.json` | `762c369374117141fc18a2b687df7e81aed5b8a385f449a8a4a9128a6faa5190` | 未变 |

**未执行**：commit / push / tag / merge / rebase / 任何 Git 写操作。
**未执行**：对 `run-a` / `run-b` 的任何写访问（仅读取 `blocks.dat` 并复制到隔离目录）。
**未执行**：任何 wallet 操作、P2P 修改、生产部署。

---

## §9 FILES TOUCHED BY F-3B

### 生产代码（7）

| 文件 | 变更 |
|---|---|
| `internal/blockchain/identity.go` | **新增** —— gate + 5 个哨兵错误 + `InitializeBlockchainStore` + 测试夹具入口 |
| `internal/blockchain/genesis.go` | `CanonicalGenesisHash` pin + `CanonicalGenesisHashHex()`（+1 注释事实修正） |
| `internal/blockchain/blockchain.go` | 移除自动创世；`h<0 → ErrUninitializedStore`；改用 gate |
| `internal/blockchain/verify.go` | 改用同一 canonical gate |
| `internal/storage/file.go` | `OpenFileBlockStoreStrict`（`repairOnOpen=false`） |
| `cmd/node/main.go` | 启动四态判定（缺文件 / 0 字节 / 身份校验） |
| `cmd/node/cli.go` | `init` 子命令 + usage 行 |

### 测试代码

| 文件 | 变更 |
|---|---|
| `cmd/node/f3b_identity_test.go` | **新增** —— 状态矩阵（9 个子断言） |
| `cmd/node/f3b_test_helpers_test.go` | **新增** —— 进程内夹具显式初始化辅助 |
| `cmd/node/stop_lifecycle_test.go` | **本轮补齐** —— `ensureTestDataDir` + 接入 `startRealNode` |
| `cmd/node/lock_lifecycle_test.go` | **本轮补齐** —— 残留锁子用例显式初始化 |
| 其余约 24 个 `cmd/node` / `internal/*` 测试文件 | 夹具迁移至 `newNodeRuntimeForTest` / `NewBlockchainFromStoreForTest` |

---

## §10 SHA-256 INTEGRITY

本报告为**自引用文档**，SHA-256 **不能自含**。收工后由独立命令计算，
登记于文件之外（交付说明 + workspace memory）：

```
最终摘要 = sha256sum PHASE-F3B-IDENTITY-IMPLEMENTATION-EXECUTION.md
```

### 基线指纹

| 常量 | 值 |
|---|---|
| HEAD COMMIT | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| CANONICAL GENESIS HASH | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| LEGACY GENESIS HASH | `0000aca1af720da390380a38dc97d141cfb67afe578a2d31315860a5489db58c` |
| INIT COMMAND | `p2pchain init`（`-datadir`） |
| MISMATCH ERROR | `GENESIS_MISMATCH` |

---

## HARD STOP

本阶段**到此终止**。**未执行**：

- ❌ commit / push / tag / merge / rebase
- ❌ 对 `run-a` / `run-b` / `node.exe` 的任何写操作
- ❌ wallet 操作 / P2P 修改 / GUI 修改 / 经济参数修改 / 生产部署

**后续阶段须 Owner 单独授权**：

- **`F-4 — NEW CHAIN CONTROLLED INITIALIZATION`**（新链受控初始化，产出正式新数据目录）
- **`F-5 — REAL NODE VERIFICATION`**（真实节点验证：双节点 P2P + 挖矿 + 重启持久化）
- `F3B-CONSEQUENCE-1`（GUI 夹具适配，可并入任一后续阶段）

---

## FINAL OUTPUT

```
PHASE F-3B STATUS:
BASELINE:                   PASS
SCOPE COMPLIANCE:           7/7（无扩大）
IMPLEMENTATION:             COMPLETE
  - genesis pin             ✅ 00003d97…e4a3
  - canonical identity gate ✅ VerifyGenesisIdentity（node + verify 共用）
  - GENESIS_MISMATCH        ✅ ErrGenesisMismatch
  - p2pchain init           ✅ cmdInit
  - auto-genesis removed    ✅ ErrUninitializedStore
  - STATE C/D FAIL CLOSED   ✅
  - regression tests        ✅
REGRESSION:                 PASS（16/16 包，cmd/node 574.4s）
E2E ACCEPTANCE:             PASS（6/6 真实数据场景）
PROTECTED OBJECTS:          UNCHANGED
FOLLOW-UPS:                 F3B-CONSEQUENCE-1（GUI 夹具）/ README 经济参数 / P2P 身份校验（DEFERRED）
HARD STOP:                  YES
```

---

*报告结束。本报告为 F-3B 实施执行产物，记录已落地的代码变更、验证证据与残留后果，不构成后续阶段的授权。*
