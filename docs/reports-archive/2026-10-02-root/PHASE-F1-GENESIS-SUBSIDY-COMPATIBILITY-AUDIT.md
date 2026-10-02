继续执行 PHASE F-2。

Owner 已明确确认：

```text
Protocol Route:
B — 正式切换新链 / 新 Genesis

Legacy Data:
B-DATA-1 — 保留为取证归档
```

F-1 最新报告已经再次确认：

- HEAD = `4d892be355a38886a83c123faad915c9f3cd62e4`
- 新 Genesis =  
  `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3`
- 旧 Genesis = `0000aca1…b58c`
- `subsidyInitial`: `50 → 5`
- `subsidyHalvingInterval`: `210 → 5_250_000`
- 旧链历史区块 100% 符合旧规则
- 新 HEAD 规则 100% 拒绝旧链历史
- Path R / New Chain 已有正式授权
- `run-a` / `run-b` 不再属于当前协议身份
- 旧数据必须保留为 forensic archive
- 旧 `wallet.json` 中的明文 P-256 私钥不得用于新链

==================================================

# F-2 CONTRACT FREEZE

==================================================

本阶段仍然：

**STRICT READ-ONLY / NO IMPLEMENTATION**

禁止：

- 修改源码
- 修改 subsidy
- 修改 genesis
- 修改 validation
- 修改 blocks.dat
- 删除 run-a/run-b
- 删除 wallet.json
- 初始化新链
- 重建 node.exe
- 启动生产节点
- git commit
- git push
- tag
- merge
- rebase
- reset

==================================================

# 1. FREEZE CURRENT PROTOCOL IDENTITY

==================================================

根据源码和 F-1 证据，建立：

```text
PROTOCOL_IDENTITY_V1

Chain ID:
[读取实际源码]

Protocol / Consensus Version:
[读取实际项目定义；不得自行创造不存在的版本号]

Genesis Hash:
00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3

Genesis Coinbase:
5

Subsidy Initial:
5

Subsidy Halving Interval:
5,250,000

Maximum Supply:
42,000,000
```

注意：

`MaximumSupply = 42,000,000` 是派生不变量，不得假装它当前是源码 enforcement 常量。

明确记录：

```text
MaximumSupply Enforcement:
NONE
```

==================================================

# 2. FREEZE GENESIS IDENTITY

==================================================

新 Genesis 必须被定义为：

**protocol identity anchor**

要求设计契约：

```text
EXPECTED_GENESIS_HASH
        ↓
ACTUAL_DATASET_GENESIS_HASH
        ↓
MATCH?
   YES → continue
   NO  → FAIL CLOSED
```

但本阶段：

**只冻结契约，不实现。**

目标错误语义：

```text
GENESIS_MISMATCH
```

或者项目现有错误系统中等价的、明确表示：

```text
DATASET_NOT_COMPATIBLE_WITH_PROTOCOL
```

禁止继续进入：

```text
ErrExcessiveCoinbase
```

这种后续验证错误。

核心原则：

> 数据目录身份检查必须早于历史区块 consensus replay。

==================================================

# 3. FREEZE DATASET POLICY

==================================================

当前有效数据：

```text
NEW PROTOCOL
    ↓
NEW DATASET
    ↓
NEW GENESIS
```

旧数据：

```text
run-a
run-b
    ↓
LEGACY PROTOCOL
    ↓
FORENSIC ARCHIVE
```

明确：

```text
Runtime:
FORBIDDEN

Migration:
FORBIDDEN

Automatic Upgrade:
FORBIDDEN

Automatic Deletion:
FORBIDDEN
```

未来如果需要处理旧数据：

必须单独授权。

==================================================

# 4. FREEZE PRIVATE KEY POLICY

==================================================

`run-a/run-b` 中现有：

```text
wallet.json
```

含明文 P-256 私钥。

定义：

```text
PRIVATE KEY STATUS:
QUARANTINED / FORENSIC ARCHIVE ONLY
```

禁止：

- 新链导入
- 新链签名
- 生产使用
- 复制
- 上传
- Git tracking
- 日志输出
- 报告中输出完整私钥

同时登记未来专项：

```text
WALLET / KEY MATERIAL DISPOSITION AUDIT
```

该专项单独授权。

本阶段不得销毁私钥文件。

==================================================

# 5. FREEZE STARTUP CONTRACT

==================================================

定义未来节点启动行为：

### Case A

```text
empty/new dataset
+
explicit new-chain initialization
```

允许初始化新的 Genesis。

### Case B

```text
existing dataset
+
Genesis == expected Genesis
```

允许进入正常验证。

### Case C

```text
existing dataset
+
Genesis != expected Genesis
```

必须：

```text
FAIL CLOSED
```

不得：

- 自动迁移
- 自动重置
- 自动删除
- 自动覆盖
- 自动重新生成 Genesis
- 尝试继续 replay

### Case D

```text
existing dataset
+
Genesis identity unavailable / corrupt
```

同样：

```text
FAIL CLOSED
```

==================================================

# 6. FREEZE FUTURE CONSENSUS CHANGE CONTRACT

==================================================

这是本次 F-1 最重要的长期补强。

以后任何修改：

- Subsidy
- Halving interval
- Coinbase rules
- Difficulty
- Timestamp rules
- Block version
- Transaction validation
- Consensus constants

不得只修改一个 `.go` 常量然后直接运行旧数据。

未来必须经过：

```text
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

把这一点正式写入 F-2 Contract。

==================================================

# 7. FREEZE GUI POLICY

==================================================

F-1 已确认 GUI：

```text
NO HARD-CODED ECONOMIC CONSENSUS
```

因此：

**本阶段不修改 GUI。**

只登记未来可考虑展示：

```text
Chain ID
Protocol Version
Genesis Hash
Network Identity
```

如果实现，必须读取节点权威 `/status`，不得 GUI 自己计算共识参数。

==================================================

# 8. CREATE IMPLEMENTATION PHASE BOUNDARY

==================================================

定义下一阶段：

```text
PHASE F-3 — PROTOCOL IDENTITY IMPLEMENTATION
```

F-3 只能在单独授权后开始。

F-3 的候选范围：

1. Genesis identity pin
2. Startup dataset identity check
3. Explicit genesis mismatch error
4. Fail-closed behavior
5. New-chain initialization contract
6. Legacy dataset rejection
7. Regression tests
8. Protocol identity RPC/status exposure
9. Compatibility replay tests

注意：

F-3 不得自动扩大为钱包、GUI、经济模型或生产部署阶段。

==================================================

# 9. GENERATE FINAL F-2 REPORT

==================================================

生成：

```text
E:/wakuang/p2pchain/PHASE-F2-PROTOCOL-IDENTITY-DECISION-FREEZE.md
```

必须包含：

1. Executive Summary
2. F-1 Evidence Reference
3. Owner Protocol Decision
4. Legacy Data Decision
5. Frozen Protocol Identity
6. Genesis Identity Contract
7. Dataset Identity Contract
8. Startup Contract
9. Legacy Archive Contract
10. Private Key Quarantine Contract
11. Future Consensus Change Contract
12. GUI Policy
13. F-3 Scope
14. Forbidden Actions
15. Evidence Manifest
16. SHA-256

==================================================

# 10. FINAL VERIFICATION

==================================================

完成报告后重新确认：

```text
HEAD unchanged
tracked diff unchanged
node.exe unchanged
run-a/blocks.dat unchanged
run-b/blocks.dat unchanged
wallet.json unchanged
no build
no new binary
no deletion
no migration
no Git write
```

如果任一项变化：

```text
STOP — F-2 READ-ONLY GUARANTEE VIOLATED
```

==================================================

# FINAL OUTPUT

==================================================

只输出：

```text
PHASE F-2 STATUS

Protocol Route:
B — NEW CHAIN / NEW GENESIS

Legacy Data:
B-DATA-1 — FORENSIC ARCHIVE

Protocol Identity:
FROZEN

Genesis Identity:
FROZEN

Dataset Contract:
FROZEN

Startup Contract:
FROZEN

Private Key Policy:
QUARANTINED

Implementation:
NOT EXECUTED

F-3:
READY FOR SEPARATE AUTHORIZATION

HARD STOP:
YES
```

严格遵守：

**VERIFY → FREEZE → REPORT → STOP**



### §8.2 补贴的激活门审计结论

对 `Subsidy` 与 `activation` 的交叉检索：

- `grep -rn "Subsidy" internal/utxo/apply.go` → 仅函数定义与 `ApplyBlock` 调用，**无 activation 判断**；
- `Subsidy()` 函数体仅有 `height / subsidyHalvingInterval` 与 `halvings >= 64` 防御上界；
- `ApplyBlock` 中 `sub := Subsidy(height)` **无高度门控分支**。

**`NO ACTIVATION GATE FOUND`**（就补贴而言）。

即：补贴规则是**全局生效、无条件生效**的。`ActivationHeight = 2000` **不能**用来保护历史补贴——事实上 `run-b` 的失败点在高 0，`run-a` 也停在高度 0，**在到达 2000 之前就已经全线失败**。即便把激活高度调到 1259 之后也**无法**拯救 `run-a`，因为失败发生在高度 0。

> 本节严格按 §6 要求执行：**只记录"未发现激活门"，不添加任何激活门。**

---

## §9 THREE PROTOCOL MODELS（三种协议模型，仅技术分析，不选择）

> 按 §7 要求：**只进行技术分析，不选择方案**。以下模型并列陈述，各自的兼容性/安全性/工作量特征如下。

### Model A — 创世固定 + 历史规则保留（Genesis Fixed + Historical Rule Preserved）

**定义**：保持历史补贴规则 `50 / 210` 作为"已存在链"的权威规则；新区块参数变化**不回溯**。

- **兼容性**：`run-a`/`run-b` **可继续验证**（若同时保留旧创世派生）。
- **矛盾**：与已冻结的 C1 政策（`5 / 5_250_000`、`MaximumSupply=42,000,000`）**直接冲突**。若 A 生效，C1 政策被废止或降级为"未来链"参数。
- **技术障碍**：单一二进制无法同时拥有两套"当前"参数而不引入链身份判别（需要 Model C 的机制）。
- **工作量**：中等（需回滚 `4d892be` 的两常量或引入判别）。
- **治理代价**：与 `A-2.3-F-R4`（Q5 `CONFIRMED`）及 `A-2.3-G2`（已实施）**相矛盾**，需项目方**重新决策**。

### Model B — 全共识重置（Full Consensus Reset）

**定义**：接受 C1 参数为新链的**唯一**权威；旧链数据被**正式废弃**；新链从新创世 `00003d97…` 重新开始。

- **兼容性**：旧 `run-a`/`run-b` **永久不可验证**（这是**设计意图**）。
- **前置条件**：新数据目录（空 datadir）；旧数据目录需**显式归档/删除**。
- **当前缺口**：**该"废弃"没有运行时可执行的表达**——HEAD 二进制面对旧数据目录给出的是**误导性的"coinbase 超限"错误**，而非"数据目录属于旧链，请使用新数据目录"。用户体验与可运维性存在缺口。
- **与既有决策的关系**：与 `A-2.3-E` 已选择的 `Path R（新链 / 新 Genesis）`**方向一致**。
- **工作量**：低（参数已是目标值），但**需要**补一层"链身份不匹配"的显式检测/提示才能闭合。
- **风险**：旧数据中含**明文私钥**（GOV-1），归档需遵循既有密钥边界治理。

### Model C — 版本化 / 激活式共识（Versioned / Activated Consensus）

**定义**：为共识参数引入**链身份或版本维度**（如创世哈希锚定 + 参数集版本），使**同一二进制**能够：识别链身份 → 选择对应参数集 → 对历史链用历史规则验证，对新链用新规则。

- **兼容性**：可同时支持旧链与新链。
- **技术障碍**：需要引入"参数集选择"机制；`Subsidy` 目前是**全局纯函数**，需改为**参数集感知**；且需与 `ActivationHeight` 现有语义（高度维）协调（现为**高度门**，Model C 需要的是**链身份门**，两者正交）。
- **风险**：引入链身份判别本身是新的共识攻击面（链身份如何被信任地确定？创世哈希如何被 pin 而不破坏"运行时生成"设计？）。
- **工作量**：高（触及 `apply.go` / `genesis.go` / `blockchain.go` / `verify.go` 与序列化层）。
- **治理代价**：属于**新协议设计**，超出当前 C1 实施 scope，需独立阶段与独立授权。

### §9.1 三模型对照（速览）

| 维度 | A 保留历史 | B 全重置 | C 版本化 |
|---|---|---|---|
| 旧链可验证 | ✅ | ❌（有意） | ✅ |
| 符合已冻结 C1 政策 | ❌ 冲突 | ✅ | ✅（并存） |
| 触及共识代码量 | 中 | 低（已达成） | 高 |
| 需项目方重新决策 | ✅ 需要 | 否（方向已定） | ✅ 需要 |
| 引入新攻击面 | 否 | 否 | ✅ 是 |
| 与 `Path R` 关系 | 违背 | 一致 | 扩展/超越 |

**本审计不对三模型排序，不推荐。** 选择属 §12 的 OWNER PROTOCOL DECISION。

---

## §10 GUI IMPACT AUDIT（GUI 影响审计，仅记录，不修改）

**审计对象**：`E:/wakuang/p2pchain/gui/p2pchain_studio/`（10 模块）

**结论：GUI 不存在任何硬编码的经济/共识常量。**

检索范围与结果：

| 检索项 | 命中 | 判定 |
|---|---|---|
| `subsidy` / `Subsidy` / `halving` / `Halving` | 0 | ✅ 无 |
| `genesis` / `Genesis` / 创世哈希字面量 | 0 | ✅ 无 |
| `5_250_000` / `5250000` / `21000000` / `42000000` / `15750000` | 0 | ✅ 无 |
| `210` / `2100万` | 0（经济语境） | ✅ 无 |
| `coinbase` | 仅作**布尔标志显示**（`is_coinbase` → "是/否"） | ✅ 非经济常量 |
| `difficulty` / `bits` / `height` | 全部来自 `GET /status` 响应 | ✅ 非硬编码 |

**GUI 的设计原则（源码原文，`main.py:171-172`）**：

> 共识真值一律来自节点（`/status` 的 `height` / `bits` / `difficulty` / `mining_state`）；**界面不自行推导任何共识量**。

**影响评估**：由于 GUI 是**纯节点数据消费者**（所有共识量均从 `/status`、`/blocks`、`/utxo`、`/block` 读取），

- 若采用 **Model B**（新链），GUI **无需改动**即可工作（连接新数据目录的节点即可）；
- 若采用 **Model A/C**，GUI 同样**无需改动**（它从不假设参数值）。

**唯一的 GUI 相关观察（记录，不修改）**：GUI 的"验证"工具页调用 `verify` 子命令；当用户对旧数据目录运行时，将透传节点返回的**误导性错误**（"coinbase 超限"）。这属于**节点侧错误语义**问题，非 GUI 缺陷。

> 本节严格按 §8 要求执行：**只记录，不修改 GUI。**

---

## §11 RISK CLASSIFICATION（风险分级）

| 级别 | 风险项 | 说明 |
|---|---|---|
| **P0** | **旧链数据在 HEAD 上完全不可验证** | `run-a`/`run-b` 0% 通过；任何依赖历史数据的操作（`verify`、重放、审计）全部失败 |
| **P0** | **链身份静默变更** | 创世哈希 `0000aca1…` → `00003d97…`；无 pin、无检测、无提示；系统不会告知"你换了链" |
| **P1** | **误导性错误语义** | 面对旧链，节点报"coinbase 超过上限"，而非"数据目录不属于本链"；误导运维与排障方向 |
| **P1** | **无迁移/拒绝机制** | `Path R` 的"废弃旧链"意图**未落地**为任何运行时行为；旧数据目录处于"存在但不可用"的悬空状态 |
| **P1** | **治理可追溯性缺口** | 共识权威常量的改动落在 `test:` 前缀提交（`4d892be`）中，而非 `feat!`/`BREAKING` 提交；虽有 `A-2.3-F-R4`/`A-2.3-G2` 授权，但**提交语义未标注破坏性**，易被后续读者误判 |
| **P2** | **历史含明文私钥** | `run-a`/`run-b` 按 GOV-1 含明文 P-256 私钥；任何归档/清理动作须遵循既有密钥边界治理 |
| **P2** | **测试覆盖缺口** | `A-2.3-G2` 记录 `G2-NB-1`（`TestMiningRuntimeAtSubsidyExhaustion`）被显式 `t.Skip`（需推进 15,749,999 块）；补贴耗尽路径**未被测试** |

**未发现**的风险（明确记录，避免过度归因）：
- 未发现算术溢出回归（`060333f` 已加固，`ApplyBlock` 使用减法形式）；
- 未发现未授权改动（C1 有正式授权链）；
- 未发现 `node.exe`/链数据被本次审计破坏（§2.3 只读证明）。

---

## §12 RECOMMENDED NEXT DECISION GATE（建议的下一决策门）

**下一步必须先执行：OWNER PROTOCOL DECISION（项目方协议决策）。**

本审计**不建议**任何技术方案，仅建议决策**顺序**：

1. **项目方在 Model A / B / C 之间作出选择**（或提出 Model D）；
2. 若选 **B（全重置）**：需决定旧数据目录（`run-a`/`run-b`，含明文私钥）的**归档/删除/保留**策略，并决定是否要求节点提供**显式链身份不匹配提示**；
3. 若选 **A（保留历史）**：需明确 C1 政策（`A-2.3-F-R4` Q5）如何被**正式修订**，以及 `4d892be` 的处置；
4. 若选 **C（版本化）**：需开启**独立的协议设计阶段**（本审计 scope 之外）；
5. 无论选哪个：需决定 `MaximumSupply` 派生不变量（42,000,000）在新旧链并存时的语义。

**在 OWNER PROTOCOL DECISION 落定之前，不得进入任何实施阶段。**

---

## §13 EXPLICIT NON-ACTIONS（明确未执行的动作）

本审计**全程未执行**下列任何一项：

- ❌ 未修改 `internal/utxo/apply.go` 的任何常量或逻辑
- ❌ 未修改 `internal/blockchain/genesis.go`（创世构建）
- ❌ 未修改任何共识校验代码（`apply.go` / `consensus.go` / `pow.go`）
- ❌ 未添加或修改任何**激活门**（§8 明确"不自行添加"）
- ❌ 未修改 `node.exe`（受保护基线）
- ❌ 未修改 `run-a/blocks.dat` / `run-b/blocks.dat`（只读，PRE==POST）
- ❌ 未修改 GUI（`gui/p2pchain_studio/` 零改动）
- ❌ 未重新构建二进制（实验室二进制为**复制**既有产物，非重编译）
- ❌ 未覆盖、未重置任何数据目录
- ❌ 未执行 `git commit` / `git push` / `git tag`
- ❌ 未执行任何部署动作
- ❌ 未对 `MaximumSupply` 添加常量或 enforcement（与 Q1/Q3 冻结一致）
- ❌ 未回滚 `4d892be`

**唯一新增物**：`F-1-lab/`（实验室产物）与本报告 `PHASE-F1-GENESIS-SUBSIDY-COMPATIBILITY-AUDIT.md`。

---

## §14 EVIDENCE MANIFEST（证据清单）

| # | 文件 | 内容 | 关键值 |
|---|---|---|---|
| E-1 | `F-1-lab/evidence/readonly-proof.txt` | 审计前后哈希（只读证明） | PRE==POST，run-a/run-b 四值相同 |
| E-2 | `F-1-lab/evidence/head-verify-run-a.json` | HEAD 验证 run-a | `valid:false`, `fail_height:0` |
| E-3 | `F-1-lab/evidence/head-verify-run-b.json` | HEAD 验证 run-b | `valid:false`, `fail_height:0` |
| E-4 | `F-1-lab/evidence/old-verify-run-a.json` | 对照验证 run-a | `valid:true`, `blocks_checked:1260` |
| E-5 | `F-1-lab/evidence/old-verify-run-b.json` | 对照验证 run-b | `valid:true`, `blocks_checked:401` |
| E-6 | `F-1-lab/evidence/run-a-printchain.txt` | run-a 全链导出（13,861 行） | 实证补贴表来源 |
| E-7 | `F-1-lab/evidence/run-b-printchain.txt` | run-b 全链导出（4,412 行） | 实证补贴表来源 |
| E-8 | `F-1-lab/evidence/subsidy-schedule.txt` | 补贴段解析与新旧规则对照 | 0/1260 与 0/401 一致新规则 |
| E-9 | `F-1-lab/evidence/head-genesis-probe.log` | 新创世生成 | `00003d97723c3c…dce4a3` |
| E-10 | `F-1-lab/analyze_subsidy.py` | 补贴解析脚本（只读输入） | — |
| E-11 | `F-1-lab/node-head.exe` | HEAD 二进制副本 | sha `5b41ff9e…dde6894c` |
| E-12 | `git show 4d892be -- internal/utxo/apply.go` | 参数变更 diff | `50→5`, `210→5_250_000` |
| E-13 | `git log -L 18,28:internal/utxo/apply.go` | 常量引入史 | `aa0e6e1` 引入旧值 |

**基线对象哈希（受保护，未变）**：

| 对象 | SHA-256 |
|---|---|
| `node.exe` | `3972843aff90428dd67acb39c6aaa009bd04d42982603617c5dbf6f0e8c213e1` |
| `run-a/blocks.dat` | `893e64c12598558ff93793248d363536e1d090a6970c2d12ed632af3eea2fb2f` |
| `run-b/blocks.dat` | `5ab67bb4f2787fb11555135671379269818c3fc43fff20057a8743c4f4e1df53` |
| `F-1-lab/node-head.exe` | `5b41ff9e0be5bd2b34c0fe94b3a65b770a2feb668e5d467db2cb72a1dde6894c` |

---

## §15 FINAL CLASSIFICATION

```
F-1 CONFIRMED — PROTOCOL COMPATIBILITY ISSUE
```

**判据**（全部为事实性，不含推断）：

1. **可复现**：HEAD 二进制对 `run-a`/`run-b` 均 EXIT=1，`fail_height = 0`，对照组 PASS（§3）。
2. **根因明确**：`subsidyInitial 50→5` 与 `subsidyHalvingInterval 210→5_250_000` 同时驱动创世 coinbase 与全链 coinbase 上限（§4、§7）。
3. **非局部**：1260/1260 与 401/401 高度全部违反新规则，首个分歧高度 = 0（§6）。
4. **创世非不可变**：创世运行时派生、无 pin，共识参数变化即改变链身份（§5）。
5. **无激活门**：补贴规则全局生效，`NO ACTIVATION GATE FOUND`（§8）。
6. **GUI 无关**：GUI 零硬编码共识常量，问题纯在节点侧（§10）。

**明确排除的其它状态**：
- 非 `F-1 CLOSED`（问题未消除）；
- 非 `F-1 INCONCLUSIVE`（证据充分且自洽）；
- 非 `FIXED`（**本阶段未修复，且按 §12 HARD STOP 不得修复**）。

---

## §16 HARD STOP

**本审计到此终止。**

- 未修复任何问题；
- 未修改任何代码；
- 未选择任何协议模型；
- 未重建、未覆盖、未重置、未提交、未推送、未部署。

**下一阶段的第一项动作必须是 `OWNER PROTOCOL DECISION`。** 在项目方作出协议决策之前，任何对补贴常量、创世、激活门、校验逻辑、节点二进制或 GUI 的改动**一律不得开始**。

---

*报告结束。本报告为只读取证产物，不构成任何修复或实施授权。*
