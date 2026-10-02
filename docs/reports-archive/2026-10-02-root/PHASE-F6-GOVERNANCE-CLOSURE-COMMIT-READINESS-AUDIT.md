# PHASE F-6 — GOVERNANCE CLOSURE & COMMIT READINESS AUDIT

**STRICT READ-ONLY**

| 项 | 值 |
|---|---|
| 仓库 | `E:/wakuang/p2pchain` |
| 审计基线 HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| 分支 | `main` |
| 审计性质 | **STRICT READ-ONLY** — 0 代码改动 / 0 数据改动 / 0 Git 写 / 0 进程启停 |
| 本报告性质 | **自引用文档**：SHA-256 由收工后独立命令计算，登记于文件之外（memory / 交付说明） |
| 交付物 | 本报告 + `FINDING-REGISTRY-F1-F5.md` + `PHASE-F6-COMMIT-CANDIDATE-MANIFEST.md` |

---

## §1 — 执行摘要（EXECUTIVE SUMMARY）

```
PHASE F-6 — VERDICT: NOT READY
HARD STOP: YES
COMMIT EXECUTION: NOT AUTHORIZED (且即使 COMMIT READY 亦须单独授权)
```

**一句话结论**：F-1 → F-5 的**实现本身**是自洽、可复现、无 scope creep 的；但
**「36 个 tracked 修改」不是一个可提交的集合** —— 它**既不足以产出可编译的树**，
**又混入了 2 个无授权记录的外来阶段改动**，并且**规定的 `go test ./...` 实测 FAIL**。

### 1.1 三个决定性事实

| # | 事实 | 证据 |
|---|---|---|
| **1** | **HEAD 的 `internal/blockchain` 测试包无法编译** | pristine HEAD 抽取实测：`go vet` → `undefined: f1n1UTXOSig`，exit 1 |
| **2** | **规定的 `go test ./...` 实测 EXIT 1** | `TestF1N1_C_MixedLegacyV2Reorg` FAIL（未跟踪文件，负载敏感 flaky） |
| **3** | **2 个 tracked 修改无本链授权记录** | `docs/PHASE-P3.1-*.md` 被外来阶段（HEAD `535edb71…`）内容整体替换 |

### 1.2 判定规则触发情况

| 规则 | 触发？ |
|---|---|
| 「G Unknown ⇒ HARD STOP」 | ✅ **触发**（F-6-FINDING-4，2 个 D 类文件） |
| 「数量发生变化 ⇒ STOP」 | ❌ 未触发（HEAD / staged / tracked 修改数**全部与规定一致**） |
| 「不得修改任何文件 / 不得 Git 写」 | ❌ 未违反（见 §14 Zero-Drift Proof） |

---

## §2 — 基线复核（BASELINE RE-VERIFICATION）

F-6 规格要求首先证明：`HEAD == 4d892be`、`staged == 0`、`tracked modifications == 36`。
**实测全部一致：**

| 项 | 要求 | 实测 | 判定 |
|---|---|---|---|
| HEAD | `4d892be` | `4d892be355a38886a83c123faad915c9f3cd62e4` | ✅ |
| 分支 | — | `main` | ✅ |
| staged 文件数 | **0** | **0** | ✅ |
| tracked 内容修改数 | **36** | **36** | ✅ |
| porcelain ` M` 行数 | （37，含 1 stat-only） | **37** | ✅ |
| untracked（porcelain `??`） | （开工 188） | **188** | ✅ |

**⇒ BASELINE PASS。未触发「数量变化 ⇒ STOP」。**

### 2.1 36 个 tracked 修改清单（按 mtime 排序）

见 `PHASE-F6-COMMIT-CANDIDATE-MANIFEST.md` §2.2（逐文件表）。

---

## §3 — 受保护对象复核（PROTECTED OBJECTS）

| 对象 | 实测 SHA-256 | 与历史登记 | 判定 |
|---|---|---|---|
| `node.exe` | `3972843aff90428dd67acb39c6aaa009bd04d42982603617c5dbf6f0e8c213e1` | 逐字符一致 | ✅ |
| `run-a/blocks.dat` | `893e64c12598558ff93793248d363536e1d090a6970c2d12ed632af3eea2fb2f` | 逐字符一致 | ✅ |
| `run-b/blocks.dat` | `5ab67bb4f2787fb11555135671379269818c3fc43fff20057a8743c4f4e1df53` | 逐字符一致 | ✅ |
| `run-a/wallet.json` | `bf577525dcab4f2584812b357b82248435d5a8b85e84cf6cb8218c9d86844a94` | 逐字符一致 | ✅ |
| `run-b/wallet.json` | `762c369374117141fc18a2b687df7e81aed5b8a385f449a8a4a9128a6faa5190` | 逐字符一致 | ✅ |

**⇒ 受保护对象全部未变。** 未导入 / 未签名 / 未复制 / 未输出任何私钥。

---

## §4 — 报告审计（F-3B / F-4 / F-5）

| 报告 | 大小 | 章节数 | HARD STOP | SHA-256（实测，与报告自述一致） |
|---|---|---|---|---|
| `PHASE-F3B-IDENTITY-IMPLEMENTATION-EXECUTION.md` | 21,141 B | 35 | ✅ | `4166d119a47ca85a03a2beb9278387071b62098554ecf883834e339f810fda5a` |
| `PHASE-F4-CONTROLLED-NEW-CHAIN-INITIALIZATION-REPORT.md` | 24,724 B | 59 | ✅ | `e0e610fe68c9e24ee8bf1035c46d41570bd8730d4c11b46f88653d162a51fbd5` |
| `PHASE-F5-REAL-NODE-VALIDATION-REPORT.md` | 24,717 B | 54 | ✅ | `ecbc24cf6e3aa82f65552dac1147d139f6a31a9d5ce63782e5f15fd7ee8627d6` |

**判定**：三份报告结构完整、含 HARD STOP 段、SHA-256 与报告自述**逐字符一致**。**报告审计 PASS。**

### 4.1 报告声明的**独立复核**（本次实测，非引用）

| 报告声明 | F-6 实测 | 判定 |
|---|---|---|
| F-4：init 产出 `blocks.dat` 180 B，sha `e517053e…13f3a0` | `F4-controlled-init-1/blocks.dat` = **180 B**，sha **`e517053e…13f3a0`** | ✅ |
| F-5：4 块后 `blocks.dat` 720 B，sha `d68c1568…b55d84` | `F5-real-node-validation-1/blocks.dat` = **720 B**，sha **`d68c1568…b55d84`** | ✅ |
| F-4/F-5：legacy 副本 sha = run-a | `F4-legacy-copy-1` / `F5-legacy-copy-1` blocks.dat = **`893e64c1…2fb2f`** | ✅ |
| F-4/F-5：二进制可复现 | `f3b-node.exe` = `f4-node.exe` = `f5-node.exe` = **`2a02814a…34db34fb`**（三份**逐字节相同**） | ✅ |
| F-3B/F-4/F-5：36 文件 +957/−407 | `git diff --stat` → **36 files, 957 insertions, 407 deletions** | ✅ |

**⇒ 全部报告声明获独立复核通过，无虚报。**

---

## §5 — 范围闭合审计（SCOPE CLOSURE）

### 5.1 F-3B 声明的 7 项 scope vs 实际

| # | 声明项 | 落地文件 | 判定 |
|---|---|---|---|
| 1 | Genesis hash pin | `internal/blockchain/genesis.go` | ✅ |
| 2 | canonical identity gate（node+verify 共用） | `internal/blockchain/identity.go`（新）+ `verify.go` + `blockchain.go` | ✅ |
| 3 | `GENESIS_MISMATCH` 落地 | `identity.go`（哨兵） | ✅ |
| 4 | `p2pchain init` 命令 | `cmd/node/cli.go` | ✅ |
| 5 | 移除 `NewBlockchainFromStore` 自动创世 | `blockchain.go` | ✅ |
| 6 | STATE C/D FAIL CLOSED | `cmd/node/main.go` + `storage/file.go` + `storage/v2.go` | ✅ |
| 7 | regression + compatibility tests | `cmd/node/f3b_identity_test.go` + `f3b_test_helpers_test.go` | ✅ |

### 5.2 禁区检查（F-3A §13.1 明文禁止扩大）

对全部 36 个 tracked 修改逐个路径匹配 `internal/p2p/*`、`internal/wallet/*`、`gui/*`、
`internal/utxo/*`、`cmd/wallet/*`：

**⇒ 命中 0 个。F-3B 未触及 P2P / wallet / GUI / 经济模型 / 生产部署。SCOPE CLOSURE PASS。**

### 5.3 F-4 / F-5 范围

F-4 / F-5 为**受控执行阶段**（仅创建隔离目录 + 出报告），未改代码。
实测 F-4/F-5 产物全部落在隔离目录（`F4-*` / `F5-*` / `f4-verify` / `f5-verify`），
**未触碰** `node.exe` / `run-a` / `run-b`。**SCOPE CLOSURE PASS。**

---

## §6 — 差异分类（A–G）

分类口径见 `PHASE-F6-COMMIT-CANDIDATE-MANIFEST.md` §1。汇总：

| 类 | 含义 | 文件数 |
|---|---|---|
| **A** | F-3B 生产代码 | 7 |
| **B** | F-3B 测试夹具（改名 / gofmt / `ensureTestDataDir`） | 22 |
| **C** | 本会话更早已授权阶段（Console 修复 + 审计修复） | 5 |
| **D** | **早于本会话**的历史工作树改动（无本链授权记录） | **2** |
| **E** | CRLF/stat 伪差异（零内容差异，不计入 36） | 1 |
| **F** | 未跟踪的 F-3B 新源文件 | 3 |
| **G** | **无法归属到已授权阶段 ⇒ UNKNOWN** | **2**（= D 类） |

> **说明**：D 类 2 个文件虽可追溯到「PHASE BRAND-0D.3 / PHASE V2.0」字样，但
> **在本链（F-1..F-6）的授权记录中不存在任何对应的授权条目**，且
> `PHASE-P3.1` 的 HEAD 指向**外来基线** `535edb71…`。
> 按 F-6 规则「无法归属到任何已授权阶段 = G Unknown」，**计为 G ⇒ HARD STOP**。

---

## §7 — 产物清单（ARTIFACT INVENTORY）

见 `PHASE-F6-COMMIT-CANDIDATE-MANIFEST.md` §3。

要点：
- **F-4/F-5 的链数据产物对 Git 不可见**（`*.dat` / `*.exe` / `wallet.json` 被忽略）——**机制性保护有效**；
- **但 `f5-verify/`（含 token + log）与 `audit-run/`、`gui-test/` 对 Git 可见** —— 见 §8。

---

## §8 — 凭据卫生审计（SECRET HYGIENE）

### 8.1 staged 内容

**staged = 0 文件** ⇒ **当前无任何内容被暂存**，无即时泄漏。

### 8.2 tracked 差异中的凭据扫描

对全部 tracked 差异做正则扫描（`BEGIN … PRIVATE` / `"d":` / `private_key` / `secret` /
`control-token` / `bearer <token>`）：

**⇒ 命中全部为注释文本**（讨论 "Bearer Token" 的鉴权语义），**无任何真实凭据值**。

### 8.3 `.gitignore` 覆盖率

```
bin/  dist/  *.exe  *.test  *.out  coverage.out  .env  .env.*  .idea/  .vscode/  .DS_Store
/run-a/  /run-b/  *.dat  *.lock  wallet.json
```

| 路径 | `git check-ignore` | 判定 |
|---|---|---|
| `run-a/wallet.json` | IGNORED | ✅ |
| `run-b/wallet.json` | IGNORED | ✅ |
| `node.lock` | IGNORED | ✅ |
| `*.dat` / `*.exe` | IGNORED | ✅ |
| `secrets/control-token` | **NOT-IGNORED** | 🔴 |
| `f5-verify/control-token` | **NOT-IGNORED** | 🔴 |
| `audit-run/control-token` | **NOT-IGNORED** | 🔴 |
| `gui-test/token` | **NOT-IGNORED** | 🔴 |

**⇒ 仓库内存在 3 个未被忽略的 token 文件（实测）。任何 `git add -A` / `git add .` /
`git commit -a` 都会暂存凭据。**（F-6-FINDING-2；扩展 F-5-FINDING-1）

---

## §9 — 规定测试（REQUIRED TESTS）

F-6 规格允许且仅允许运行：`go test ./...`、`go vet ./...`、`go build`、`git diff --check`。

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go build ./...` | **0** | ✅ PASS（全模块编译通过） |
| `go vet ./...` | **0** | ✅ PASS（无 vet 告警） |
| `go test -count=1 ./...` | **1** | ❌ **FAIL**（1 个包 FAIL，见 §9.1） |
| `git diff --check` | **2** | ❌ **FAIL**（trailing whitespace，全部落在 D 类文档，见 §9.2） |
| `git diff --check --cached` | **0** | ✅ PASS（staged 为空） |

> 说明：`go test` 使用 `-count=1` 以**绕过缓存**、取得真实结果（规格意图为真实测试运行）。

### 9.1 `go test -count=1 ./...` 结果明细

```
ok  	p2pchain/cmd/node	572.224s
ok  	p2pchain/internal/attribution	0.371s
ok  	p2pchain/internal/block	0.297s
FAIL	p2pchain/internal/blockchain	108.798s
ok  	p2pchain/internal/blocktree	0.279s
ok  	p2pchain/internal/control	3.845s
ok  	p2pchain/internal/explorer	0.486s
ok  	p2pchain/internal/mempool	34.542s
ok  	p2pchain/internal/obs	0.433s
ok  	p2pchain/internal/p2p	15.137s
ok  	p2pchain/internal/pow	3.881s
ok  	p2pchain/internal/storage	294.974s
ok  	p2pchain/internal/transaction	0.280s
ok  	p2pchain/internal/txbuild	0.276s
ok  	p2pchain/internal/utxo	0.400s
ok  	p2pchain/internal/wallet	3.437s
FAIL
test-exit=1
```

**15 ok / 1 FAIL（+2 no-test-files）。**

失败详情：

```
--- FAIL: TestF1N1_C_MixedLegacyV2Reorg (39.13s)
    f1n1_reorg_coverage_test.go:560: F1N1: no sibling below threshold after 256 attempts (height=3)
```

### 9.2 失败归因（复现实验）

| 运行方式 | 结果 |
|---|---|
| `go test -count=1 -run TestF1N1_C_MixedLegacyV2Reorg ./internal/blockchain/` ×3 | **ok 3/3**（1.8s / 2.6s / 1.0s） |
| `go test -count=1 ./internal/blockchain/`（整包）×2 | **ok 2/2**（66.0s / 64.3s） |
| `go test -count=1 ./...`（全套并行） | **FAIL 1/1**（39.13s，`no sibling below threshold after 256 attempts`） |

**⇒ 判定：`TestF1N1_C_MixedLegacyV2Reorg` 是「负载敏感的 flaky test」。**
该用例在 256 次尝试内搜索一个低于难度阈值的 sibling 块；在整套 `./...` 并行负载
（`cmd/node` 572s + `storage` 295s 同时占用 CPU）下，PoW 尝试预算/时间假设被打破。

**关键属性**：该 flaky 用例位于**未跟踪文件** `internal/blockchain/f1n1_reorg_coverage_test.go`
（F1-N1 工作流，**非 F-3B 产物**）。

### 9.3 `git diff --check` 明细

全部命中 `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md`（**D 类**）：
trailing whitespace × ~130 行 + `new blank line at EOF`。
**剔除 D 类文件后，该检查自然通过。**

---

## §10 — 关键发现（FINDINGS）

### F-6-FINDING-1 — 【BLOCKER · 完整性】「36 个 tracked 修改」不足以产出可编译的树
- **严重度**：**BLOCKER**
- **事实链**：
  1. `internal/blockchain/p1_reorg_fail_before_commit_test.go` 是 **tracked 且未修改**的文件
     （`git cat-file -e HEAD:…` → **PRESENT in HEAD**）。
  2. 该文件第 63 行**作为代码**调用 `f1n1UTXOSig(bc)`。
  3. `f1n1UTXOSig` 的**唯一**定义在 `internal/blockchain/f1n1_reorg_coverage_test.go:134`。
  4. 该定义文件**不在 HEAD**（`git cat-file -e` → `exists on disk, but not in 'HEAD'`）。
- **实测证据**（`git archive HEAD` 抽取到系统临时目录，仓库零改动）：
  ```
  $ go vet ./internal/blockchain/
  internal\blockchain\p1_reorg_fail_before_commit_test.go:63:19: undefined: f1n1UTXOSig
  vet-exit=1
  ```
  （`go build ./...` 于 pristine HEAD → exit 0；破坏**仅在测试编译单元**。）
- **修复的最小必要集合**（实测，注入后 `go vet ./internal/blockchain/` → exit 0）：
  `internal/blockchain/identity.go`（未跟踪，F 类）
  + `internal/blockchain/f1n1_reorg_coverage_test.go`（未跟踪）
  + `blockchain.go` / `genesis.go` / `verify.go`（A 类）
- **影响**：任何「只提交 36 个 tracked 修改」的方案都会产出一棵 `go test ./...`
  **无法编译**的树。提交候选集**必须**扩充（见 MANIFEST §6.1）。
- **性质**：这是**既存于 HEAD 的缺陷**（早于 F-3B），非 F-3B 引入。

### F-6-FINDING-2 — 【HIGH · 安全】未跟踪凭据文件未被 `.gitignore` 覆盖
- **严重度**：**HIGH**
- **事实**：`git check-ignore` 对 `audit-run/control-token`、`f5-verify/control-token`、
  `gui-test/token` 均返回 **exit 1（NOT-IGNORED）**。
- **影响**：任何 `git add -A` / `git add .` / `git commit -a` 都会**暂存凭据**。
- **关系**：扩展 F-5-FINDING-1（原仅登记 F-5 一处；F-6 实测为 **3 处**）。
- **建议**：提交必须使用**显式路径白名单**；`.gitignore` 增补 `secrets/` 与 token 模式
  （须单独授权，因会改变受跟踪修改数）。

### F-6-FINDING-3 — 【HIGH · 测试】规定的 `go test ./...` 实测 FAIL（负载敏感 flaky）
- **严重度**：**HIGH**
- **事实**：`go test -count=1 ./...` → **EXIT 1**；`TestF1N1_C_MixedLegacyV2Reorg` FAIL。
- **归因**：隔离 3/3 PASS、整包 2/2 PASS、全套并行 1/1 FAIL ⇒ **负载敏感 flaky**。
- **位置**：**未跟踪文件** `internal/blockchain/f1n1_reorg_coverage_test.go`（F1-N1 工作流）。
- **与 F-5 报告的关系**：F-5 曾记录 `go test ./...` 16/16 PASS；本次在**同一工作树**上 FAIL
  ⇒ 该用例**本就不稳定**，F-5 的 PASS 是**运气**而非稳定保证。
- **影响**：F-6 的「规定测试」**未通过**；CI 可用性受损。

### F-6-FINDING-4 — 【MEDIUM · 治理】2 个 tracked 修改无本链授权记录（G Unknown）
- **严重度**：**MEDIUM**（但**触发 HARD STOP 规则**）
- **对象**：
  1. `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md`（+470/−220）
     —— 工作树内容被**整体替换**为「PHASE BRAND-0D.3」报告，内含**外来 HEAD `535edb71…`**
     （≠ 本链 `4d892be`）。mtime **2026-09-12 21:52**（**早于本会话**）。
  2. `docs/DETERMINISTIC-SERIALIZATION-SPEC.md`（+88/−4）
     —— 追加 §9.11「PHASE V2.0 补录」。mtime **2026-09-13 09:30**（**早于本会话**）。
- **判定**：在本链 F-1..F-6 的授权记录中**无任何对应授权条目** ⇒ **G Unknown ⇒ HARD STOP**。
- **影响**：若被提交，会把**外来阶段内容**并入本链历史。

### F-6-FINDING-5 — 【MEDIUM · 完整性】未跟踪但被编译/测试的完整工作流
- **严重度**：**MEDIUM**
- **对象**（9 个文件）：`internal/attribution/{attribution,classify,attribution_test}.go`、
  `cmd/coinbase-attribution/main.go`、`internal/{utxo,storage,blockchain}/f1n1_*.go`、
  `internal/{storage,blockchain}/f1n2_*.go`
- **事实**：`go build ./...` / `go test ./...` **会编译/执行它们**（`cmd/coinbase-attribution` 包
  是 `internal/attribution` 的**唯一** importer），但它们**未被跟踪**。
- **影响**：提交树与工作树**不等价**；遗漏提交 ⇒ attribution 包与 F1-N1/F1-N2 测试**静默消失**。

### F-6-FINDING-6 — 【LOW】`git diff --check` 失败
- **严重度**：**LOW**
- **事实**：`git diff --check` → **EXIT 2**，全部命中 D 类文档的 trailing whitespace。
- **影响**：剔除 D 类后自然消解。

### F-6-OBSERVATION-1 — 【INFO】F-4 的「Git 忽略」结论需修正
- F-4 曾结论「新链数据**机制上不可能被误提交**」。该结论对 `*.dat`/`*.exe`/`wallet.json` **成立**，
  但 F-6 证明其**不充分**：**token 文件不在忽略范围内**（F-6-FINDING-2）。

---

## §11 — 提交边界分析（COMMIT BOUNDARY）

| 边界 | 内容 | 判定 |
|---|---|---|
| **必须包含** | A(7) + B(22) + F(3) + `f1n1_reorg_coverage_test.go`(1) = **33 文件** | 否则不可编译（FINDING-1） |
| **须 Owner 决策** | C(5) + `console_mine_auth_test.go`(1) = 6；未跟踪工作流 9；报告 10 | 相邻授权 / 范围判断 |
| **必须排除** | D(2) 文档、E(1) stat-only、3 个凭据目录、REORG-1F..1J 系列、全部验证产物 | 越界 / 安全 / 非本链 |

**强制约束**：提交**禁止**使用 `git add -A` / `git add .` / `git commit -a`。

详见 `PHASE-F6-COMMIT-CANDIDATE-MANIFEST.md` §6。

---

## §12 — 判定（VERDICT）

```
PHASE F-6 — GOVERNANCE CLOSURE & COMMIT READINESS AUDIT

BASELINE:            PASS   (HEAD 4d892be / staged 0 / tracked 36 — 全部一致)
REPORT AUDIT:        PASS   (F-3B/F-4/F-5 三份报告 + 全部声明获独立复核)
SCOPE CLOSURE:       PASS   (F-3B 7/7 落地，0 禁区命中，F-4/F-5 未越界)
PROTECTED OBJECTS:   PASS   (5 对象 SHA-256 全部未变)
REQUIRED TESTS:      FAIL   (go test ./... → EXIT 1；git diff --check → EXIT 2)
DIFF CLASSIFICATION: G UNKNOWN PRESENT  (D 类 2 文件无本链授权记录)
SECRET HYGIENE:      RISK   (3 个未跟踪 token 文件未被忽略)
COMMIT CANDIDATE:    INSUFFICIENT  (36 个 tracked 修改无法产出可编译的树)

VERDICT:  NOT READY
HARD STOP:  YES
```

### 12.1 为何是 NOT READY（而非 COMMIT READY WITH EXCLUSIONS）

1. **G Unknown 已触发**（F-6-FINDING-4）：按 F-6 规格，「无法归属到已授权阶段」= HARD STOP。
   「WITH EXCLUSIONS」可以剔除文件，**但剔除 G 类文件并不能解释它们为何存在**
   —— 这是**治理缺口**，须 Owner 裁定其归属。
2. **提交集合结构性不足**（F-6-FINDING-1）：不是「多几个文件要不要排除」的问题，
   而是**缺文件就无法编译**。这需要**扩充**提交集，超出「36 个 tracked 修改」的既定范围。
3. **规定测试未通过**（F-6-FINDING-3）：`go test ./...` FAIL。

### 12.2 达到 COMMIT READY 的前置条件（须 Owner 逐项授权）

| # | 前置条件 | 类型 |
|---|---|---|
| 1 | 裁定 D 类 2 文档的归属（纳入 / 排除 / 还原） | 治理 |
| 2 | 授权将 `internal/blockchain/f1n1_reorg_coverage_test.go` 纳入提交（**编译必需**） | 治理 |
| 3 | 裁定 §3.3 的 9 个未跟踪文件是否纳入 | 治理 |
| 4 | 修复 / 隔离 `TestF1N1_C_MixedLegacyV2Reorg` 的负载敏感性 | 代码 |
| 5 | `.gitignore` 增补 token 模式（或确认提交走白名单） | 安全 |
| 6 | 裁定 C 类 6 个文件的提交归属 | 治理 |

---

## §13 — 治理与流程观察

- **F-1 → F-5 的「契约 → 授权 → 实施 → 验证 → 报告 → HARD STOP」流程执行良好**：
  报告自述数字经独立复核**零虚报**；受保护对象**零漂移**；scope **零 creep**。
- **但「提交就绪性」此前从未被独立审计**：F-3B/F-4/F-5 均只证明「**实现正确**」，
  从未证明「**工作树可提交**」。F-6 首次揭示二者**不等价**（FINDING-1/4/5）。
- **F-5 的 `go test ./... PASS` 具有误导性**：同一工作树上 F-6 实测 FAIL（FINDING-3）
  ⇒ 单次 PASS **不构成稳定性证据**。
- **本阶段未产生任何新的实现缺陷** —— 全部 6 个 finding 均为**治理 / 完整性 / 测试稳定性**类。

---

## §14 — Zero-Drift Proof（开工 vs 收工）

| 项 | 开工 | 收工 | 漂移 |
|---|---|---|---|
| HEAD | `4d892be…` | `4d892be…` | **0** |
| 分支 | `main` | `main` | **0** |
| staged | 0 | 0 | **0** |
| tracked 内容修改 | 36 | 36 | **0** |
| porcelain ` M` | 37 | 37 | **0** |
| untracked（porcelain `??`） | 188 | **191** | **+3**（= 本阶段 3 份交付物） |
| 受保护对象（5 项 SHA-256） | — | — | **0** |
| `.gitignore` | — | — | **未修改** |

### 14.1 未执行的操作（明文声明）

**未执行**：`git add` / `commit` / `push` / `tag` / `merge` / `rebase` / `amend` / `squash` /
`reset` / `checkout` / `clean` / `restore` / `stash` / `update-index`。
**未修改**任何被审计文件（含 `.gitignore`、任何 `.go`、任何 `.md`）。
**未启停**任何节点进程；**未运行**任何挖矿。

### 14.2 唯一的工作树副作用（披露）

- 创建 3 份交付物：`FINDING-REGISTRY-F1-F5.md`、`PHASE-F6-COMMIT-CANDIDATE-MANIFEST.md`、
  本报告（untracked 188 → 191，**+3**）。
- 为捕获测试输出临时创建 `f6-test-output.txt`（untracked），**收工前已删除**（不计入 191）。
- 为验证 FINDING-1，使用 `git archive HEAD` 抽取到**系统临时目录**（`$TEMP`），
  **仓库零改动**，抽取物**已删除**。

**⇒ 除 3 份交付物外，仓库零漂移。**

---

## §15 — 复现命令（REPRODUCTION）

```bash
# 基线
git rev-parse HEAD                                   # 4d892be355a38886a83c123faad915c9f3cd62e4
git diff --cached --name-only | wc -l                # 0
git diff --name-only | wc -l                         # 36
git status --porcelain | grep -c '^ M'               # 37
git status --porcelain | grep -c '^??'               # 188 (开工)

# 受保护对象
sha256sum node.exe run-a/blocks.dat run-b/blocks.dat

# 规定测试
go build ./...                                       # exit 0
go vet ./...                                         # exit 0
go test -count=1 ./...                               # exit 1  (FAIL: TestF1N1_C_MixedLegacyV2Reorg)
git diff --check                                     # exit 2  (trailing whitespace, D 类文档)

# FINDING-1 复现（仓库零改动）
TD=$(mktemp -d) && git archive HEAD | tar -x -C "$TD"
( cd "$TD" && go vet ./internal/blockchain/ )        # undefined: f1n1UTXOSig  → exit 1
rm -rf "$TD"

# FINDING-2 复现
git check-ignore -q f5-verify/control-token ; echo $?   # 1  (NOT-IGNORED)
git check-ignore -q audit-run/control-token ; echo $?   # 1
git check-ignore -q gui-test/token          ; echo $?   # 1

# FINDING-3 复现
go test -count=1 -run TestF1N1_C_MixedLegacyV2Reorg ./internal/blockchain/   # ok
go test -count=1 ./internal/blockchain/                                     # ok
go test -count=1 ./...                                                      # FAIL (under load)

# FINDING-4 复现
git cat-file -e HEAD:internal/blockchain/f1n1_reorg_coverage_test.go   # fatal: not in HEAD
grep -n 'BRAND-0D.3\|535edb71' docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md | head
```

---

## §16 — 局限与未覆盖（LIMITATIONS）

- 本审计**未**验证 F-3B/F-4/F-5 的**功能正确性**（那是各阶段自身的职责，已由各自报告与
  F-6 §4.1 的抽样复核覆盖）。
- 本审计**未**重跑 F-4/F-5 的真实节点生命周期（F-6 为只读阶段，**不启停进程**）。
- 本审计**未**判定 D 类文档的**应然归属** —— 那是 Owner 的治理裁定，非 AI 可代选。
- `TestF1N1_C_MixedLegacyV2Reorg` 的负载敏感性**已定性**（flaky），但**未定量**
  （未测不同并行度下的失败率 —— 属测试工程范畴，超出 F-6 只读边界）。

---

## §17 — 交付物

| # | 文件 | 内容 |
|---|---|---|
| 1 | `FINDING-REGISTRY-F1-F5.md` | F-1..F-5 全部 finding 的 ID / 严重度 / 状态 / 闭合证据 |
| 2 | `PHASE-F6-COMMIT-CANDIDATE-MANIFEST.md` | A–G 分类口径 + 逐文件分类 + 依赖分析 + 提交边界 |
| 3 | `PHASE-F6-GOVERNANCE-CLOSURE-COMMIT-READINESS-AUDIT.md` | 本报告 |

---

## §18 — 后续（NEXT STEPS）

**未自动进入任何下一阶段。** 未 commit / push / tag。

F-6 判定 **NOT READY**，须 Owner 就 §12.2 的 6 项前置条件逐项裁定并单独授权后，
方可形成 **COMMIT EXECUTION** 阶段（该阶段亦须单独授权）。

**HARD STOP。**
