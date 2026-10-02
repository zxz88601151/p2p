# PHASE F-4 — CONTROLLED NEW-CHAIN INITIALIZATION REPORT

- **阶段**：PHASE F-4 — CONTROLLED NEW-CHAIN INITIALIZATION
- **性质**：CONTROLLED INITIALIZATION / STATE-CREATION（受控初始化 / 状态创建）
- **对象**：`E:/wakuang/p2pchain`（P2PChain）
- **日期**：2026-09-27
- **前置**：F-1 PASS → F-2 PASS（Route B / New Chain）→ F-3 PASS（READY WITH BLOCKERS）→ F-3A PASS（契约冻结）→ **F-3B PASS（协议身份实施）**
- **判定**：

```
PHASE F-4 — VERDICT: PASS
```

---

## 1. PHASE IDENTITY

| 项 | 值 |
|---|---|
| Phase ID | `F-4` |
| 名称 | CONTROLLED NEW-CHAIN INITIALIZATION |
| 类型 | State-creation（唯一被授权的写入 = 在**全新隔离目录**中创建 canonical Genesis） |
| 上游 | F-3B（`PHASE-F3B-IDENTITY-IMPLEMENTATION-EXECUTION.md`, sha `4166d119…0fda5a`） |
| 下游 | F-5 — REAL NODE VALIDATION（**未授权，本阶段不得进入**） |
| 本阶段授权范围 | canonical `init` / 身份验证 / init 拒绝性 / legacy 隔离 / 空目录 fail-closed / 受保护数据完整性 |

---

## 2. BASELINE（Step 1 — 写操作之前）

| # | 检查项 | 实测 |
|---|---|---|
| 1 | HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| 2 | branch | `main` |
| 3 | porcelain（`^ M`） | **37** 条 |
| 4 | 其中**真实内容差异**（`git diff --name-only`） | **36** 条 |
| 5 | 其中**CRLF/stat 伪差异** | **1** 条 —— `internal/blockchain/query.go` |
| 6 | staged（`git diff --cached`） | **0** 条（索引 == HEAD） |
| 7 | untracked | **185** 条 |
| 8 | deleted / renamed / added | **0** |

### 2.1 关于 37 vs 36（已定性，非缺陷）

`internal/blockchain/query.go` 出现在 `git status --porcelain` 中，但：

```
$ git diff --quiet internal/blockchain/query.go   → exit 0   （零内容差异）
$ git update-index --refresh                      → 状态仍为 " M"
```

⇒ 该条目是**行尾（CRLF）规范化层面的 stat 伪差异**，**内容与 HEAD 完全一致**。
真实受跟踪修改 = **36 个文件**（与 F-3B 收工时记录一致：`36 files changed, 957 insertions(+), 407 deletions(-)`）。

> **本阶段未执行任何"整理"操作**：无 `git reset` / `checkout` / `clean` / `restore`。
> 唯一触及索引的命令是 `git update-index --refresh`（仅刷新 stat 缓存，**不改变内容、不改变 ref、不改变索引条目集合**，
> 且执行后 porcelain 输出逐字符不变）。此项在此**主动披露**，以保持 Zero-Drift 证明完整。

### 2.2 Untracked 归类（185 条）

| 类别 | 数量 |
|---|---|
| `docs/`（历史阶段报告与 prompt） | 148 |
| 其它目录（`internal/`、`cmd/`、`verifier/`、`gui/`、`gui-test/`、`concept-mvp/`、`audit-run/`、`F-1-lab/`、`.workbuddy/`） | 17 |
| 仓库根历史报告（`PHASE-*.md`） | 20 |

**F-3B 报告、F-3A/F-3/F-2/F-1 报告均在此列（untracked，未提交）。**

---

## 3. HEAD

```
4d892be355a38886a83c123faad915c9f3cd62e4
```

**F-4 全程 HEAD 未变**（无 commit / amend / rebase / tag）。

---

## 4. WORKING TREE STATE

| 项 | 开工 | 收工 | 结论 |
|---|---|---|---|
| HEAD | `4d892be` | `4d892be` | 未动 |
| 真实内容修改 | 36 | 36 | 未动 |
| porcelain `^ M` | 37 | 37 | 未动 |
| staged | 0 | 0 | 未动 |
| untracked | 185 | 186 | +1 = **本报告自身**（唯一新增未跟踪条目；F-4 的运行产物被 `.gitignore` 吸收，见 §15.4） |

**本阶段对源码 / 协议 / 配置的写入 = 0 字节。** 未跟踪条目的净增仅为交付物本身。

---

## 5. PROTECTED OBJECTS（Step 2 — READ-ONLY）

### 5.1 开工 / 收工 SHA-256（逐字符一致）

| 对象 | SHA-256 | 开工 | 收工 |
|---|---|---|---|
| `node.exe` | `3972843aff90428dd67acb39c6aaa009bd04d42982603617c5dbf6f0e8c213e1` | ✅ | ✅ 未变 |
| `run-a/blocks.dat` | `893e64c12598558ff93793248d363536e1d090a6970c2d12ed632af3eea2fb2f` | ✅ | ✅ 未变 |
| `run-b/blocks.dat` | `5ab67bb4f2787fb11555135671379269818c3fc43fff20057a8743c4f4e1df53` | ✅ | ✅ 未变 |
| `run-a/wallet.json` | `bf577525dcab4f2584812b357b82248435d5a8b85e84cf6cb8218c9d86844a94` | ✅ | ✅ 未变 |
| `run-b/wallet.json` | `762c369374117141fc18a2b687df7e81aed5b8a385f449a8a4a9128a6faa5190` | ✅ | ✅ 未变 |
| `PHASE-F3B-…-EXECUTION.md` | `4166d119a47ca85a03a2beb9278387071b62098554ecf883834e339f810fda5a` | ✅ | ✅ 未变 |

### 5.2 目录清单（未变）

```
run-a/  blocks.dat 226800 B (Sep 14 23:06) | node.lock 42 B | wallet.json 389 B
run-b/  blocks.dat  72180 B (Sep 14 23:06) | node.lock 42 B | wallet.json 389 B
```

`node.exe` 是**唯一能读取 legacy `run-a` / `run-b` 的二进制**；F-4 期间**未被读取、未被覆盖、未被调用**。

### 5.3 被验证的二进制（新建，非受保护对象）

| 项 | 值 |
|---|---|
| 路径 | `f4-verify/f4-node.exe` |
| 构建 | `go build -o f4-verify/f4-node.exe ./cmd/node`（exit 0） |
| 大小 | `12,065,792` B |
| SHA-256 | `2a02814af98ee76a47ca24b61308aa16263e9489e0ecdf72ef87df0734db34fb` |
| 来源 | HEAD `4d892be` + F-3B 工作树（**即 F-3B 已验证的正式实现**） |

> **未覆盖 `node.exe`**（构建产物另起文件名，遵守项目既有红线）。

---

## 6. INITIALIZATION TARGET（Step 3）

| 项 | 值 |
|---|---|
| 路径 | `E:/wakuang/p2pchain/F4-controlled-init-1` |
| 创建前 | **不存在**（`ls -d` → exit 2, "No such file or directory"） |
| 创建方式 | `mkdir -p`（本阶段显式创建） |
| 创建后内容 | **空**（仅 `.` / `..`） |
| `blocks.dat` | **ABSENT** ✅ |
| `wallet.json` | **ABSENT** ✅ |
| `node.lock` | **ABSENT** ✅ |
| 与既有 node/service 共用 | **否**（全新路径，无任何进程引用） |
| 是否生产目录 | **否** |

**安全性判定**：目标为**全新、明确、隔离、可删除**的初始化目录 ⇒ 满足 §2 HARD SAFETY RULE，**允许执行 `init`**。

### 6.1 隔离性说明（未触碰的对象）

以下对象在 F-4 中**从未被 `init`**：

- `run-a` / `run-b`（legacy，只读归档）
- `node.exe` 所服务的任何既有数据目录
- `audit-run/`、`gui-test/`、`f3b-verify/`、`F-1-lab/`（历史阶段产物）

---

## 7. EXACT COMMAND（Step 4）

### 7.1 先确认 CLI 契约（**未猜参数**）

```
$ ./f4-verify/f4-node.exe help
...
  init          显式初始化新数据目录并创建 canonical Genesis
...
离线命令选项:
  -datadir <目录>              数据目录（默认 ~/.p2pchain）
```

```
$ ./f4-verify/f4-node.exe init -h
Usage of init:
  -datadir string
    	数据目录 (default "C:\\Users\\Administrator\\.p2pchain")
```

⇒ `init` 的**唯一**参数是 `-datadir`。默认值为 `C:\Users\Administrator\.p2pchain` ——
**本阶段显式覆盖该默认值**，确保不触碰用户默认数据目录。

### 7.2 实际执行

```bash
./f4-verify/f4-node.exe init -datadir F4-controlled-init-1
```

**未使用**：手工写 Genesis / 手工生成 `blocks.dat` / 调用内部私有函数绕过 CLI / 修改 Genesis 参数 / 以旧链副本为输入。

---

## 8. INIT RESULT（Step 4）

```
$ ./f4-verify/f4-node.exe init -datadir F4-controlled-init-1
已初始化 canonical Genesis: 00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3

exit = 0   ✅
```

---

## 9. GENESIS IDENTITY

| 项 | 值 |
|---|---|
| EXPECTED（F-3A/F-3B 冻结） | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| ACTUAL（F-4 新链创世） | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| MATCH | **YES** ✅ |
| LEGACY（对照，非本链） | `0000aca1af720da390380a38dc97d141cfb67afe578a2d31315860a5489db58c` |

⇒ **F-4 创建出来的是 F-3A / F-3B 冻结的 canonical new chain。**

---

## 10. blocks.dat EVIDENCE（Step 5）

### 10.1 目录内容（初始化后）

```
$ ls -la F4-controlled-init-1
drwxr-xr-x  .            Sep 27 12:52
drwxr-xr-x  ..
-rw-r--r--  blocks.dat   180 B   Sep 27 12:52
```

```
$ find F4-controlled-init-1 -type f
F4-controlled-init-1/blocks.dat          ← 唯一文件
```

### 10.2 文件事实

| 项 | 值 |
|---|---|
| 路径 | `F4-controlled-init-1/blocks.dat` |
| 大小 | **180** bytes（非 0 ✅） |
| mtime | `2026-09-27 12:52:49.674084500 +0800` |
| SHA-256 | `e517053e5ed51692a08e80c3c97f4fe79125f3a020e0a50d935250170113f3a0` |

### 10.3 否定性检查（全部通过）

| 检查 | 结果 |
|---|---|
| 出现 legacy chain 数据？ | **否** ✅（创世 = `00003d97…`，非 `0000aca1…`） |
| 出现意外 wallet？ | **否** ✅（`init` 不创建钱包，`wallet.json` ABSENT） |
| 出现额外数据库文件？ | **否** ✅（无 sidecar / metadata / manifest / 索引文件） |
| 出现 `node.lock` 残留？ | **否** ✅（锁已释放） |

> **单文件不变量**：OD-01 = A（继续以 `blocks.dat[0]` 作唯一 identity carrier）在 F-4 得到实证 ——
> 初始化产物**只有 `blocks.dat` 一个文件**，无任何显式身份元数据。B-1 的"被接受的约束"保持成立。

---

## 11. VERIFY RESULT（Step 6）

```
$ ./f4-verify/f4-node.exe verify -datadir F4-controlled-init-1 -json
{
  "data_dir": "F4-controlled-init-1",
  "height": 0,
  "blocks_checked": 1,
  "genesis_hash": "00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3",
  "tip_hash":     "00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3",
  "valid": true,
  "fail_height": -1,
  "fail_hash": "",
  "reason": ""
}

exit = 0   ✅
```

```
[verify] 结果      : PASS（只读回放校验通过，未修改任何数据）
```

| 项 | 要求 | 实测 | 结论 |
|---|---|---|---|
| valid | true | true | ✅ |
| exit | 0 | 0 | ✅ |
| genesis_hash | canonical | canonical | ✅ |
| fail_height | -1 | -1 | ✅ |

---

## 12. RE-INIT REFUSAL（Step 7 — 幂等性 / 拒绝性）

```
$ ./f4-verify/f4-node.exe init -datadir F4-controlled-init-1
错误: data directory is already initialized；拒绝覆盖现有数据集

exit = 1   ✅（必须拒绝）
```

### 12.1 前后对比（关键证据）

| 项 | BEFORE | AFTER | 结论 |
|---|---|---|---|
| `blocks.dat` SHA-256 | `e517053e5ed51692a08e80c3c97f4fe79125f3a020e0a50d935250170113f3a0` | **同** | **IDENTICAL** ✅ |
| `blocks.dat` size | 180 | 180 | 未变 ✅ |
| `blocks.dat` mtime | `2026-09-27 12:52:49.674084500` | **同** | 未变 ✅ |
| 文件清单 | `{blocks.dat}` | `{blocks.dat}` | 未变 ✅ |

> 目录自身的 mtime 由 `12:52` → `12:53`：这是 `init` 为取锁而**创建并随即删除** `node.lock` 的**瞬时副作用**，
> 无任何持久化条目增加/减少/改写（文件清单逐条一致）。

### 12.2 结论

> **`init` 是一次性显式初始化，而不是危险的覆盖式初始化。** ✅

---

## 13. LEGACY ISOLATION（Step 8）

### 13.1 输入：**只读副本**（非生产原始目录）

```
$ cp run-a/blocks.dat F4-legacy-copy-1/blocks.dat
```

| 项 | 值 |
|---|---|
| 副本路径 | `F4-legacy-copy-1/blocks.dat` |
| 大小 | `226800` bytes |
| SHA-256（基线） | `893e64c12598558ff93793248d363536e1d090a6970c2d12ed632af3eea2fb2f` |

### 13.2 三个探针

| # | 操作 | 命令 | 结果 | 退出码 |
|---|---|---|---|---|
| 8a | `verify` | `verify -datadir F4-legacy-copy-1` | FAIL @ height 0，`GENESIS_MISMATCH: expected 00003d97…e4a3, got 0000aca1…b58c` | 1 ✅ |
| 8b | node 启动 | `-datadir F4-legacy-copy-1 -listen 127.0.0.1:0 -rpc 127.0.0.1:0` | `加载区块链失败: GENESIS_MISMATCH: …` | 1 ✅ |
| 8c | `init` | `init -datadir F4-legacy-copy-1` | `GENESIS_MISMATCH: …；拒绝覆盖现有数据集` | 1 ✅ |

**要求满足**：全部路径均以 `GENESIS_MISMATCH` 明确失败，**且失败高度 = 0**（identity 检查**先于** replay）。

### 13.3 只读性（无 repair / rewrite / migrate / overwrite）

| 项 | BEFORE | AFTER | 结论 |
|---|---|---|---|
| 副本 SHA-256 | `893e64c1…ea2fb2f` | `893e64c12598558ff93793248d363536e1d090a6970c2d12ed632af3eea2fb2f` | **IDENTICAL** ✅ |
| 副本 size | 226800 | 226800 | 未变 ✅ |
| 副本 mtime | `2026-09-27 12:53:19.350009500` | **同** | 未变 ✅ |
| 生产 `run-a/blocks.dat` | `893e64c1…ea2fb2f` | `893e64c1…ea2fb2f` | 未变 ✅ |
| 生产 `run-b/blocks.dat` | `5ab67bb4…e1df53` | `5ab67bb4…e1df53` | 未变 ✅ |

⇒ 三个探针（含**以可写方式打开存储**的 node 启动路径）**零字节改写** legacy 数据。

> **对照（F-1 缺陷闭合的最终形态）**：同一份 legacy 数据
> - 旧：`创世区块 UTXO 初始化失败: coinbase 输出 50 > 奖励 5 + 手续费 0`（伪装成经济共识错误、方向依赖）
> - 新：`GENESIS_MISMATCH: expected 00003d97…, got 0000aca1…`（**显式身份失败**）

---

## 14. EMPTY-DIRECTORY FAIL-CLOSED（Step 9）

```
$ mkdir -p F4-empty-dir-1        # 空目录，只执行 node startup，不执行 init
$ ls -la F4-empty-dir-1          # 仅 . / ..
$ ./f4-verify/f4-node.exe -datadir F4-empty-dir-1 -listen 127.0.0.1:0 -rpc 127.0.0.1:0
2026/09/27 12:53:33 [node] 加载区块链失败: data directory is not initialized

exit = 1   ✅
```

| 检查 | 结果 |
|---|---|
| 自动创建 `blocks.dat`？ | **否** ✅（post-state 仍为空） |
| 自动生成 Genesis？ | **否** ✅ |
| 偷偷初始化？ | **否** ✅ |
| 进入 replay？ | **否** ✅（错误发生在身份/初始化判定阶段，早于 ApplyBlock） |
| 冻结的等价 fail-closed 行为 | `ErrUninitializedStore`（`data directory is not initialized`）✅ |

### 14.1 补充证据 A —— 0 字节 `blocks.dat`（STATE D / C-1）

```
$ : > F4-zero-byte-1/blocks.dat          # 0 字节
$ ./f4-verify/f4-node.exe -datadir F4-zero-byte-1 -listen 127.0.0.1:0 -rpc 127.0.0.1:0
2026/09/27 12:53:41 [node] 加载区块链失败: genesis identity is corrupted      exit=1  ✅
$ ./f4-verify/f4-node.exe init -datadir F4-zero-byte-1
错误: genesis identity is corrupted: F4-zero-byte-1\blocks.dat                exit=1  ✅
```

**0 字节文件仍为 0 字节**（未被当作空库、未被改写）✅ —— 符合 F-3A §8.4 的 C-1 约束。

### 14.2 补充证据 B —— F-3B 契约测试在**本修订版**上复跑

```
$ go test -count=1 -run TestF3B ./cmd/node/
ok  	p2pchain/cmd/node	1.669s      ✅
```

⇒ 被 F-4 验证的实现，正是 F-3B 已验证的同一实现（**无隐藏偏差**）。

---

## 15. SHA-256 EVIDENCE

### 15.1 本阶段产物

| 对象 | SHA-256 |
|---|---|
| `F4-controlled-init-1/blocks.dat`（新链创世，180 B） | `e517053e5ed51692a08e80c3c97f4fe79125f3a020e0a50d935250170113f3a0` |
| `f4-verify/f4-node.exe`（被验证二进制，12,065,792 B） | `2a02814af98ee76a47ca24b61308aa16263e9489e0ecdf72ef87df0734db34fb` |
| `F4-legacy-copy-1/blocks.dat`（legacy 只读副本） | `893e64c12598558ff93793248d363536e1d090a6970c2d12ed632af3eea2fb2f` |
| `F4-zero-byte-1/blocks.dat`（0 字节探针） | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`（空文件标准值） |
| `F4-empty-dir-1/`（空目录探针） | 无文件（N/A） |
| 本报告 | 见 §19 自引用说明（收工后独立计算） |

### 15.2 上游产物（未变）

| 产物 | SHA-256 |
|---|---|
| `PHASE-F1-GENESIS-SUBSIDY-COMPATIBILITY-AUDIT.md` | `d44791da1ffc029ed1f1d8666f732e7705b73dfbb2f4d2c867d0b559074c7ded` |
| `PHASE-F2-PROTOCOL-IDENTITY-DECISION-FREEZE.md` | `001c48655c7ff693809420240d254abdfa109f8272cf9c0282ccbc20023bbdb9` |
| `PHASE-F3-GENESIS-IDENTITY-STARTUP-READINESS-AUDIT.md` | `bbced9a80d8cb778e529be5c38defb478ef42679587c9b148375dbeb8f0aaa6f` |
| `PHASE-F3A-GENESIS-IDENTITY-CONTRACT-FREEZE.md` | `bccdfb17d8cbe0d8ed01f72f800f7025a7a8bf39c3f3692f52d220fbcf7159c3` |
| `PHASE-F3B-IDENTITY-IMPLEMENTATION-EXECUTION.md` | `4166d119a47ca85a03a2beb9278387071b62098554ecf883834e339f810fda5a` |

### 15.3 基线指纹

| 常量 | 值 |
|---|---|
| HEAD COMMIT | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| CANONICAL GENESIS HASH | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| LEGACY GENESIS HASH | `0000aca1af720da390380a38dc97d141cfb67afe578a2d31315860a5489db58c` |
| INIT COMMAND | `init -datadir <dir>` |

### 15.4 重要治理发现：`blocks.dat` 与产物对 Git **不可见**

仓库 `.gitignore` 含：

```
*.exe
*.dat
*.lock
wallet.json
/run-a/
/run-b/
```

⇒ **`blocks.dat` 在仓库范围内被 Git 忽略**，因此：

- F-4 的初始化产物（`F4-*/blocks.dat`、`f4-verify/f4-node.exe`）**不出现在 `git status`**（untracked 计数保持 185 不变）；
- 新链数据**在机制上不可能被误提交**（GOV-1 治理意图的直接体现）；
- 副产品：F-4 的产物目录对 Git 是"隐形"的，本报告因此**逐项登记其 SHA-256** 作为唯一可追溯凭据。

---

## 16. NO-WRITE EVIDENCE

### 16.1 Zero-Drift Proof（开工 vs 收工）

| 项 | 开工 | 收工 | 结论 |
|---|---|---|---|
| HEAD | `4d892be` | `4d892be` | **未动** |
| 真实内容修改 | 36 | 36 | **未动** |
| porcelain `^ M` | 37 | 37 | **未动** |
| staged | 0 | 0 | **未动** |
| untracked | 185 | 186 | **+1 = 本报告**（交付物；运行产物被 `.gitignore` 吸收） |
| `node.exe` | `3972843a…` | `3972843a…` | **未动** |
| `run-a/blocks.dat` | `893e64c1…` | `893e64c1…` | **未动** |
| `run-b/blocks.dat` | `5ab67bb4…` | `5ab67bb4…` | **未动** |
| `run-a/wallet.json` | `bf577525…` | `bf577525…` | **未动** |
| `run-b/wallet.json` | `762c3693…` | `762c3693…` | **未动** |
| 源码 / 常量 / Genesis 参数 | 未改 | 未改 | **未动** |
| 协议规则 / subsidy / difficulty / mining / P2P / wallet / GUI | 未改 | 未改 | **未动** |

### 16.2 唯一写入（已授权范围内）

| 写入对象 | 性质 | 是否在授权范围 |
|---|---|---|
| `F4-controlled-init-1/blocks.dat`（新建 180 B） | canonical `init` 的**预期产物** | ✅ 是 |
| `F4-legacy-copy-1/blocks.dat`（副本，未改写） | 只读副本，内容零变化 | ✅ 是 |
| `F4-empty-dir-1/`（空目录） | 探针目录，无文件 | ✅ 是 |
| `F4-zero-byte-1/blocks.dat`（0 B） | 探针文件，未改写 | ✅ 是 |
| `f4-verify/f4-node.exe`（构建产物） | 验证用二进制 | ✅ 是 |
| 本报告 `.md` | 交付物 | ✅ 是 |

### 16.3 明确**未执行**的写入

```
❌ commit        ❌ push          ❌ tag           ❌ merge
❌ rebase        ❌ amend         ❌ squash        ❌ reset
❌ checkout      ❌ clean         ❌ restore       ❌ stash
❌ 对 run-a / run-b 的任何写操作
❌ 对 node.exe 的覆盖
❌ 删除 / 迁移 / 修复任何 legacy 数据
❌ 修改 GUI / wallet / P2P / 经济参数
```

> **披露**：`git update-index --refresh` 执行过一次（刷新 stat 缓存）。该命令**不属于**上列禁止集合，
> 且执行后 porcelain 输出逐字符不变（详见 §2.1）。在此主动记录以保持证明完整。

---

## 17. KNOWN DOWNSTREAM CONSEQUENCE — F3B-CONSEQUENCE-1

### 17.1 登记（**本阶段不修**）

| 项 | 内容 |
|---|---|
| ID | `F3B-CONSEQUENCE-1` |
| 对象 | `gui/_fulltest.py`（GUI 全功能测试夹具，**非** GUI 产品代码） |
| 触发条件 | `reset_dirs()`（第 76 行）删除 `gui-test/data-a`、`data-b` → 第 2 节随后在**空目录**上 `startRealNode` |
| 预期失败原因 | 空目录 ⇒ `加载区块链失败: data directory is not initialized`（exit 1，节点在就绪前退出） |
| 根因 | 该夹具沿用 F-3B 之前的「空目录启动即自动创世」生命周期假设 |
| 本阶段处置 | **DO NOT FIX**（依 F-4 §13 与 F-3A §13.1「F-3B 不得扩大至 GUI」） |
| 是否修改 GUI | **否** ✅ |
| 是否修改 F-3B 冻结实现 | **否** ✅ |

### 17.2 精确修法（留待后续授权）

在 `_fulltest.py` 启动节点前，对每个自建数据目录先执行一次幂等初始化：

```python
run_cli(BIN, ["init", "-datadir", DATA_A])   # 已初始化时返回非 0，可忽略
```

**性质**：这是**测试夹具**对契约变更的适配，与 F-3B 中已完成的 Go 夹具迁移（`ensureTestDataDir`）同类，
**不是 GUI 功能缺陷**。

---

## 18. SCOPE COMPLIANCE

### 18.1 F-4 已完成（§12 要求）

| 能力 | 状态 | 证据 |
|---|---|---|
| canonical init | ✅ PASS | §7–§10 |
| identity verification | ✅ PASS | §11 |
| init refusal | ✅ PASS | §12 |
| legacy isolation | ✅ PASS | §13 |
| empty-dir fail-closed | ✅ PASS | §14 |
| protected-data integrity | ✅ PASS | §5, §16 |

### 18.2 F-4 **不负责**（本阶段未执行）

```
real node production startup  ❌ 未执行（未在 F4-controlled-init-1 上启动节点）
P2P                           ❌ 未改
mining                        ❌ 未改
block propagation             ❌ 未涉及
chain sync                    ❌ 未涉及
wallet                        ❌ 未改（init 不创建钱包）
GUI                           ❌ 未改（F3B-CONSEQUENCE-1 仅登记）
production service            ❌ 未部署
```

### 18.3 禁止项逐条核对

| 禁止项 | 是否发生 |
|---|---|
| 修改协议规则 | ❌ 未发生 |
| 修改 Genesis 参数 | ❌ 未发生 |
| 修改 subsidy / difficulty / mining / P2P / wallet / GUI | ❌ 未发生 |
| 修改旧链数据 | ❌ 未发生（SHA 逐字符一致） |
| 覆盖任何既有 `blocks.dat` | ❌ 未发生 |
| 删除任何旧链数据 | ❌ 未发生 |
| 自动迁移 / 自动修复 legacy chain | ❌ 未发生 |
| 进入 F-5 真实节点运行验证 | ❌ 未发生 |
| commit / push / tag / merge / rebase / amend / squash | ❌ 未发生 |

---

## 19. VERDICT

### 19.1 判定规则逐条

| # | 条件 | 实测 | 结论 |
|---|---|---|---|
| 1 | canonical init = PASS | exit 0，创世 = canonical | ✅ |
| 2 | verify = PASS | valid=true，exit 0 | ✅ |
| 3 | second init refused = PASS | exit 1，`already initialized` | ✅ |
| 4 | legacy mismatch = PASS | 三路径均 `GENESIS_MISMATCH` | ✅ |
| 5 | legacy data unchanged = PASS | SHA 逐字符一致（副本 + 生产） | ✅ |
| 6 | empty-dir does not auto-init = PASS | `ErrUninitializedStore`，目录仍空 | ✅ |
| 7 | protected objects unchanged = PASS | 6/6 SHA 未变 | ✅ |
| 8 | no protocol modification = PASS | 36 修改文件数未变，零源码写入 | ✅ |
| 9 | no GUI modification = PASS | `gui/` 未触碰 | ✅ |

```
PHASE F-4 — VERDICT: PASS
```

### 19.2 报告自引用说明

本报告为**自引用文档**，其 SHA-256 **不能自含**。收工后由独立命令计算并登记于文件之外：

```
最终摘要 = sha256sum PHASE-F4-CONTROLLED-NEW-CHAIN-INITIALIZATION-REPORT.md
```

---

## 20. NEXT-PHASE RECOMMENDATION

### 20.1 必须做（进入 F-5 之前）

1. **Owner 单独授权 F-5** —— F-4 不自动推进。
2. **明确 F-5 的数据目录策略**：是在 `F4-controlled-init-1` 上继续，还是另建正式新链目录
   （F-4 已证明 `init` 可在任意全新目录中复现 canonical 身份）。
3. **确认 F-5 的运行边界**：P2P 监听 / RPC 端口须避开被占用端口（本机 7688 被 `vpn07Core.exe` 占用）。

### 20.2 应后置

| 项 | 建议阶段 |
|---|---|
| `F3B-CONSEQUENCE-1`（GUI 夹具适配，一行） | 可并入任一后续阶段（**需单独授权**，因涉及 GUI 目录） |
| README 经济参数过期（第 165 行 `50 >> (height/210)`；"难度为何不浮动"整节） | 随 F-5 或专门文档阶段 |
| P2P `genesis_hash` 校验（OD-08 DEFERRED） | `P2P DISCOVERY / RESILIENCE PHASE` |
| 36 个已修改文件 + F-1..F-4 报告的提交组织 | 专门 COMMIT 阶段（F-4 禁止 commit） |

### 20.3 可选

- 把 F-4 的 6 项验收固化为**可重复脚本**（`f4-verify/` 当前为一次性人工执行）；
- 将 `blocks.dat` 的 180 B 基线哈希纳入后续阶段的回归哨兵。

### 20.4 明确不做

```
❌ 不自动进入 F-5
❌ 不启动生产节点 / 不部署
❌ 不修改协议 / Genesis / 经济参数
❌ 不修改 GUI / wallet / P2P
❌ 不 commit / push
```

### 20.5 F-5 计划是否仍成立

**成立，且前置条件已全部满足。** F-4 已证明：
新链可在受控隔离目录中被**确定性创建**（创世 `00003d97…e4a3`）、**可校验**、**不可被二次初始化覆盖**、
**与 legacy 严格隔离**、**空目录不会隐式创世**。
⇒ F-5（真实节点验证：双节点 P2P + 挖矿 + 重启持久化）可按原计划进行，**但须 Owner 单独授权**。

---

## HARD STOP

```
PHASE F-4 COMPLETE
VERDICT: PASS

F-5 AUTHORIZATION REQUIRED
```

**绝对不要自动进入 F-5。**

---

*报告结束。本报告为 F-4 受控初始化阶段的执行产物，记录基线、命令、证据与判定，不构成 F-5 或任何后续阶段的授权。*
