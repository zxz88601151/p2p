# FINDING REGISTRY — F-1 → F-5

**PHASE F-6 — GOVERNANCE CLOSURE & COMMIT READINESS AUDIT (STRICT READ-ONLY)**
**Deliverable 1 of 3**

| 项 | 值 |
|---|---|
| 仓库 | `E:/wakuang/p2pchain` |
| 审计基线 HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| 分支 | `main` |
| 审计性质 | STRICT READ-ONLY（0 代码改动 / 0 数据改动 / 0 Git 写 / 0 进程启停） |
| 报告为自引用文档 | SHA-256 由收工后独立命令计算，登记于文件之外（memory / 交付说明） |

---

## §0 — 目的与口径

本登记册汇总 **F-1 → F-5** 全部阶段中产生的一切 finding，给出：

1. **ID**（阶段内唯一）；
2. **严重度**（BLOCKER / HIGH / MEDIUM / LOW / GOVERNANCE）；
3. **状态**（CLOSED / MITIGATED / OPEN / ACCEPTED-CONSTRAINT / DEFERRED-BY-CONTRACT）；
4. **闭合证据**（哪一步、哪条命令、哪个哈希）；
5. **残留风险**（若有）。

口径说明：

- **CLOSED** = 缺陷已由后续已授权阶段消除，且有可复现实测证据。
- **MITIGATED** = 缺陷的触发路径被契约/机制切断，但底层结构性事实仍存在（已由 Owner 显式接受）。
- **OPEN** = 未修，且不在任何已授权阶段的范围内；须单独授权。
- **ACCEPTED-CONSTRAINT** = 由 Owner 决策显式接受为约束，不是缺陷。
- **DEFERRED-BY-CONTRACT** = 由 Owner 契约明文推迟到未来阶段。

> **本登记册不含任何私钥、token 或凭据值。** 涉及 wallet 的行只登记键名与 SHA-256。

---

## §1 — 受保护对象（跨全部阶段不变的锚点）

审计开工时重新实测，与历史登记**逐字符一致**：

| 对象 | SHA-256 | 说明 |
|---|---|---|
| `node.exe` | `3972843aff90428dd67acb39c6aaa009bd04d42982603617c5dbf6f0e8c213e1` | 2026-09-13 构建；**唯一能读旧链 `run-a`/`run-b` 的二进制** |
| `run-a/blocks.dat` | `893e64c12598558ff93793248d363536e1d090a6970c2d12ed632af3eea2fb2f` | FORENSIC ARCHIVE（0..1259），只读 |
| `run-b/blocks.dat` | `5ab67bb4f2787fb11555135671379269818c3fc43fff20057a8743c4f4e1df53` | FORENSIC ARCHIVE（0..400），只读 |
| `run-a/wallet.json` | `bf577525dcab4f2584812b357b82248435d5a8b85e84cf6cb8218c9d86844a94` | QUARANTINED（只登记哈希） |
| `run-b/wallet.json` | `762c369374117141fc18a2b687df7e81aed5b8a385f449a8a4a9128a6faa5190` | QUARANTINED（只登记哈希） |

---

## §2 — F-1 阶段 finding

### F-1-FINDING-1 — 【HIGH · 共识级】创世分裂：当前源码无法回放任何既有链
- **状态**：**CLOSED**（由 Owner Decision B + F-3B identity gate 闭合）
- **发现**：`run-a`/`run-b` 在受保护 `node.exe`（09-13）下 **PASS**；由 HEAD 新构建的二进制 **FAIL 于高度 0**：
  `创世区块 UTXO 初始化失败: coinbase 输出超过区块奖励加手续费上限: coinbase 输出 50 > 奖励 5 + 手续费 0`
- **根因**：HEAD `4d892be` 同时改了 `internal/utxo/apply.go` 的 `subsidyInitial 50→5` 与
  `subsidyHalvingInterval 210→5_250_000`。这两个常量**同时**驱动 (a) 创世 coinbase（`genesis.go` 调 `utxo.Subsidy(0)`）
  与 (b) 全链每个高度的 `ErrExcessiveCoinbase` 上限 ⇒ **整条历史补贴曲线被替换**，而补贴**无激活门**
  （唯一激活门 `pow.ActivationHeight=2000` 只管难度/时间戳/版本）⇒ 改补贴即改创世。
- **实证**：`run-a`(0..1259) 与旧规则 `50>>(h/210)` 一致 **1260/1260**、与新规则 **0/1260**，首个分歧高度 **0**；
  `run-b`(0..400) 同样 401/401 vs 0/401。
- **派生**：理论最大供应 旧 **20,370** → 新 **42,000,000**。
- **闭合证据**：F-2 Owner Decision = **B（新链/新 Genesis）**；F-3B 落地 canonical identity gate 后，
  旧链启动不再报误导性 `ErrExcessiveCoinbase`，而是 **`GENESIS_MISMATCH`（失败高度 0）**：
  `GENESIS_MISMATCH: expected 00003d97…e4a3, got 0000aca1…b58c`。
  失败**先于** consensus replay，符合 F-2 §8 Case C 契约。
- **残留**：旧链作为**协议身份**已永久废弃（Owner Decision B），保留为取证归档。**不是缺陷，是决策。**

### F-1-FINDING-2 — 【MEDIUM · 文档】README 经济参数过期
- **状态**：**OPEN**（与 F-1 决策绑定，需单独授权）
- **发现**：README 第 165 行仍写 `50 >> (height/210)`；「难度为何不浮动」整节已过期（实际 `MaxDifficultyBits=32`）。
- **为何未修**：F-1 明文规定「F-1 定案前不宜改写，避免预设结论」。F-3B/F-4/F-5 均未授权文档阶段。
- **建议**：与 F-3B-CONSEQUENCE-1 同批，在 Documentation 阶段一次性更新。

### F-1-FINDING-3 — 【LOW · 治理】创世非不可变常量（无 pin）
- **状态**：**CLOSED**（由 F-3B 落地 pin）
- **发现**：F-1 时 `NewBlockchainFromStore` 在 `Height()<0` 时运行时重新生成创世，无任何 pin。
- **闭合证据**：F-3B 新增 `internal/blockchain/genesis.go` 的 `CanonicalGenesisHash`
  = `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` + `CanonicalGenesisHashHex()`；
  `InitializeBlockchainStore` 创建前自校验 pin，漂移则 `ErrCanonicalGenesisDrift`。

---

## §3 — F-3 阶段 finding（5 个 blocker）

> F-3 判定为 **READY WITH BLOCKERS**。下表给出每个 blocker 在 F-3B 之后的去向。

| ID | 严重度 | 内容 | F-3B 后状态 | 闭合证据 |
|---|---|---|---|---|
| **B-1** | BLOCKER（结构性） | 身份元数据缺失 ⇒ Case A（空库）与 Case D（损坏）不可区分；0 字节/缺失 `blocks.dat` 被当空库 ⇒ 自动创世 | **MITIGATED**（由 OD-01=A + OD-03 缓解） | 判别器改为**显式初始化动作**而非文件内容：`node` 启动永不自动初始化；0 字节按 CORRUPTED fail closed（`cmd/node/main.go`）；仅 `p2pchain init` 可进入 INITIALIZED |
| **B-2** | BLOCKER | 无任何 genesis identity check；非测试 `.go` 创世哈希字面量 = **0** | **CLOSED** | `internal/blockchain/identity.go`（新增）唯一 gate `VerifyGenesisIdentity`；`CanonicalGenesisHash` pin |
| **B-3** | BLOCKER | mismatch 检测方向依赖：仅当磁盘 coinbase > 期望补贴才偶然失败 | **CLOSED** | 身份比较取代经济副作用；`GENESIS_MISMATCH` 与 coinbase 无关 |
| **B-4** | BLOCKER | `verify` 独立旁路（`VerifyStoredChain` 绕过 `NewBlockchainFromStore`），两条生产构造路径都无校验 | **CLOSED** | `verify.go` 与 node 共用同一 gate；F-4/F-5 实测 `verify` 亦报 `GENESIS_MISMATCH` |
| **B-5** | BLOCKER | 初始化是隐式的（无 `--init`，打开空目录即自动生成创世） | **CLOSED** | `cmd/node/cli.go` 新增 `init` 子命令（唯一参数 `-datadir`），自动创世路径移除 |

### F-3-FINDING-1 — 【GOVERNANCE】P2P `genesis_hash` 只广播不校验
- **状态**：**DEFERRED-BY-CONTRACT**（OD-08）
- **发现**：`grep '\.GenesisHash\s*(==|!=)'` → **NO COMPARISON FOUND**；注释「用于快速识别网络不一致」= 意图未实现。
- **契约**：F-3A OD-08 = P2P 校验 **DEFERRED TO P2P DISCOVERY / RESILIENCE PHASE**。**P2P 重设计 OUT OF SCOPE。**

### F-3-FINDING-2 — 【GOVERNANCE】`wallet.LoadOrCreate` 是一条隐式初始化路径
- **状态**：**OPEN**（OUT OF SCOPE，须单独授权）
- **发现**：`main.go` 缺钱包时**自动创建** `wallet.json`（与「显式初始化」精神相邻）。
- **为何未修**：F-3A §13.1 明文禁止 F-3B 扩大到 wallet。

---

## §4 — F-3B 阶段 finding

### F-3B-CONSEQUENCE-1 — 【MEDIUM · 回归】GUI 夹具在 F-3B 后必然 fail closed
- **状态**：**OPEN**（**故意未修**，由契约禁止）
- **发现**：`gui/_fulltest.py` 的 `reset_dirs()` 删目录后在空目录起节点；F-3B 之后空目录必然
  fail closed 为 `ErrUninitializedStore`。GUI 夹具需先显式 `init`。
- **为何未修**：F-3A §13.1 明文禁止 F-3B 扩大到 GUI。F-4 §13 再次明令 **DO NOT FIX**。
- **精确修法（已登记，一行）**：在 `reset_dirs()` 重建目录后、启动节点前，调用
  `node init -datadir <dir>`（或等价 `cmdInit`）。
- **建议归属**：GUI 阶段（须单独授权）。

### F-3B-FINDING-1 — 【LOW · 注释】`genesis.go` 注释误写难度位
- **状态**：**CLOSED**
- **发现**：`genesis.go` 原注释写 `MaxTargetBits=20`，实际 `MaxTargetBits=16`（`internal/pow/pow.go:29`）。
- **闭合证据**：F-3B 修正为 16（**纯注释**，无共识影响；已计入 36 个 tracked 修改之一）。

---

## §5 — F-4 阶段 finding

F-4 **未暴露新的代码/共识缺陷**。判定 `PHASE F-4 COMPLETE / VERDICT: PASS`，9 项判定条件全过。

### F-4-OBSERVATION-1 — 【GOVERNANCE】`blocks.dat` 在仓库范围内被 Git 忽略
- **状态**：**ACCEPTED-CONSTRAINT**（机制性保护，非缺陷）
- **发现**：`.gitignore` 含 `*.dat` / `*.exe` / `*.lock` / `wallet.json` / `/run-a/` / `/run-b/`
  ⇒ 新链数据**机制上不可能被误提交**。
- **副产品**：F-4 产物（`F4-*/blocks.dat`、`f4-verify/*.exe`）**不出现在 `git status`**
  ⇒ 报告逐项登记其 SHA-256 作为唯一可追溯凭据。
- **F-6 复核**：本条在 F-6 中被**证伪其充分性** —— 见 F-6-FINDING-2（`*.dat`/`*.exe` 被忽略，
  **但 token 文件不被忽略**）。

### F-4-FINDING-1 — 【LOW · 诊断】CRLF/stat 伪差异
- **状态**：**ACCEPTED-CONSTRAINT**（已文档化，未清理）
- **发现**：`git status` 报 `internal/blockchain/query.go` 为 modified，但 `git diff --quiet` 退出 **0**
  ⇒ **零内容差异**，纯 CRLF/stat 伪差异。故「37 M」中真实内容修改只有 **36**。
- **判定方法（复现）**：`git status --porcelain | grep -c '^ M'` vs `git diff --name-only | wc -l`。
- **为何未清理**：清理属 Git 写操作，任何阶段均未授权；且清理会改变受保护基线数字。

---

## §6 — F-5 阶段 finding

### F-5-FINDING-1 — 【MEDIUM · 治理/安全】mutation auth token 文件未被 `.gitignore` 覆盖
- **状态**：**OPEN**（F-6 复核后**升级关注度**）
- **发现**：`git check-ignore` 对 `f5-verify/control-token` 与默认路径 `secrets/control-token`
  均 **exit 1（NOT-IGNORED）**。现有 `.gitignore` 覆盖 `wallet.json`/`*.lock`/`*.dat`/`*.exe`，**未含 token**。
- **F-6 复核（扩大范围）**：不止 F-5 一处。实测仓库内存在 **3 个** 未被忽略的 token 文件：
  `audit-run/control-token`、`f5-verify/control-token`、`gui-test/token`
  ⇒ 任何 `git add -A` / `git add .` 都会**暂存凭据**。见 **F-6-FINDING-2**。
- **为何未修**：改 `.gitignore` 会使受跟踪修改数 36→37，违反 F-5 基线与「不得自行清理」。
- **建议**：与 F-3B-CONSEQUENCE-1 同批，在 Governance/Git 边界阶段加入 `secrets/` 与 token 模式。

### F-5-FINDING-2 — 【INFO】F-4 产物对 Git 不可见
- **状态**：**CLOSED / 已解释**
- **发现**：F-4 的 `f4-verify/` 对 git 不可见（只含 `*.exe`）；F-5 的 `f5-verify/` 因含 token+log 而可见。
- **意义**：**正是 `f5-verify/` 的可见性暴露了 F-5-FINDING-1** —— 即 `.gitignore` 的保护对
  「纯二进制/纯数据」目录有效，对「混入 token/log 的目录」失效。

---

## §7 — 汇总矩阵

| ID | 阶段 | 严重度 | 状态 |
|---|---|---|---|
| F-1-FINDING-1 | F-1 | HIGH（共识级） | **CLOSED** |
| F-1-FINDING-2 | F-1 | MEDIUM（文档） | **OPEN** |
| F-1-FINDING-3 | F-1 | LOW（治理） | **CLOSED** |
| B-1 | F-3 | BLOCKER | **MITIGATED** |
| B-2 | F-3 | BLOCKER | **CLOSED** |
| B-3 | F-3 | BLOCKER | **CLOSED** |
| B-4 | F-3 | BLOCKER | **CLOSED** |
| B-5 | F-3 | BLOCKER | **CLOSED** |
| F-3-FINDING-1 | F-3 | GOVERNANCE | **DEFERRED-BY-CONTRACT** |
| F-3-FINDING-2 | F-3 | GOVERNANCE | **OPEN** |
| F-3B-CONSEQUENCE-1 | F-3B | MEDIUM（回归） | **OPEN** |
| F-3B-FINDING-1 | F-3B | LOW（注释） | **CLOSED** |
| F-4-OBSERVATION-1 | F-4 | GOVERNANCE | **ACCEPTED-CONSTRAINT** |
| F-4-FINDING-1 | F-4 | LOW（诊断） | **ACCEPTED-CONSTRAINT** |
| F-5-FINDING-1 | F-5 | MEDIUM（安全） | **OPEN** |
| F-5-FINDING-2 | F-5 | INFO | **CLOSED** |

**统计**：CLOSED 6 · MITIGATED 1 · ACCEPTED-CONSTRAINT 2 · DEFERRED-BY-CONTRACT 1 · **OPEN 4** ·
**BLOCKER 残留 0**。

**F-1 → F-5 范围内不存在未闭合的 BLOCKER。** 4 个 OPEN 项全部属于
「文档 / GUI 夹具 / 凭据卫生 / wallet 初始化」——均**超出 F-1..F-5 授权边界**，须单独授权阶段处理。

---

## §8 — 与 F-6 的接口

本登记册**不**包含 F-6 自身产生的新 finding。F-6 新 finding（含 1 个 **BLOCKER 级**）登记于：

- `PHASE-F6-GOVERNANCE-CLOSURE-COMMIT-READINESS-AUDIT.md` §Findings

**HARD STOP。** 本登记册为只读审计产物，不授权任何后续写入。
