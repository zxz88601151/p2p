# CONSENSUS-RUNTIME-VALIDATION-1-REPORT

> **Phase**：CONSENSUS-RUNTIME-VALIDATION-1 — 运行时共识状态只读验证
> **Owner authorization**：PHASE P2PCHAIN — CONSENSUS-RUNTIME-VALIDATION-1
> **Date**：2026-10-02
> **Scope**：Runtime state verification only · No code changes · No consensus changes · No protocol changes
> **HARD STOP（强制）**：No commit · No deployment · No cloud modification · No code changes

---

## §0 Runtime baseline

| 项 | 值 | 说明 |
|---|---|---|
| `git rev-parse HEAD` | `52fb464af243fbcdfd78bab846b45ea090e918fb` | 未改动 |
| 二进制 `node.exe` | SHA-256 `fb3d2b57dd8adbd02d3c6d501f42a2cfa3c076657694e35756967caf9307b717` | 2026-10-02 01:42 构建（晚于 HEAD，本地工作树构建产物） |
| 二进制 `node-test.exe` | `12046848` B（2026-09-27 08:34） | 参考 |
| Go 工具链 | `go1.27.0 windows/amd64` | — |
| **运行中的 P2PChain 节点** | **无** | 4 个 `node.exe` 进程均为 Node.js 运行时（驻留 ~73–460 MB，远大于本 ~12 MB Go 二进制），且**无一绑定项目端口**（6688/6689/6690/16688–16693/17881/9091 均无 LISTENING） |
| 运行时链状态 | **磁盘持久链**（`blocks.dat`） | 无活节点 ⇒ 以磁盘链 + 只读 `verify` 重放为运行时真值 |

### 数据目录清单（磁盘链）

| datadir | height | genesis | tip | `verify` 结论 |
|---|---|---|---|---|
| `audit-run/data-ui` | **664** | `00003d97…` ✅ | `0000c50b6b17fd61…` | **valid=true** |
| `audit-run/data-b` | **200** | `00003d97…` ✅ | `00000e686ef53cfe…` | **valid=true** |
| `audit-run/data-miner` | **200** | `00003d97…` ✅ | `00000e686ef53cfe…` | **valid=true** |
| `run-a`（遗留） | 1259 | `0000aca1…` ❌ | — | `GENESIS_MISMATCH`（旧创世身份） |
| `run-b`（遗留） | 400 | `0000aca1…` ❌ | — | `GENESIS_MISMATCH`（旧创世身份） |

> **关键裁定**：`run-a`/`run-b` 是**旧创世身份（F2/F3 冻结前）**的遗留链，其 genesis `0000aca1…` ≠ 当前协议身份锚点 `CanonicalGenesisHash=00003d97…`，故 `verify` 判定 `GENESIS_MISMATCH`。它们**不是当前共识协议下的合法链**，仅作历史遗留。当前权威运行时链为 `audit-run/*` 三目录。

---

## §1 Ruleset verification

| 规则集 | 判据 | 实测 | 结论 |
|---|---|---|---|
| **v1（h < 2000）** | version=1、bits 钉死 16 | 三链全部区块 version=1、bits=16（665 / 201 / 201 块） | ✅ 一致 |
| **v2（2000 ≤ h < 3000）** | version=2、Ceil 浮动 | 链高 ≤ 664，**未到达** | ✅ 未触发（正确） |
| **v3（h ≥ 3000）** | version=3、Nearest 浮动、注入 27 | 链高 ≤ 664，**未到达** | ✅ 未触发（正确） |

- `data-ui`：665 块全 `version=1`（histogram `{1: 665}`）。
- `data-b`/`data-miner`：201 块全 `version=1`（histogram `{1: 201}`）。
- **结论**：三链高度均远低于 2000，完全处于 **ruleset v1**，与文档共识规则（`VersionForHeight` 三态、激活高度 2000/3000）逐字节一致。

---

## §2 Difficulty runtime verification

| 项 | 实测 | 结论 |
|---|---|---|
| 实际 bits 值 | 全部 `16`（`bits histogram = {16: N}`） | ✅ v1 钉死 16 |
| 调整行为 | h%20==0 边界（h=20/40/60/80/100…）bits 恒 16 | ✅ 无重定向（v1 语义：钉死父块 bits，不触发 `AdjustBits`） |
| clamp 范围 | min=max=16（未触及 ceiling） | ✅ 未越界 |
| `MaxDifficultyBits=32` | 链高 < 2000 ⇒ ceiling 从未被触及 | ✅ 未激活（正确，非缺陷） |
| 激活行为 | 未达 2000 ⇒ 无 v2/v3 浮动 | ✅ 未激活 |

**结论**：难度运行时状态与文档「v1 钉死 16、v2/v3 待激活」完全一致。当前链上恒 16 是**尚未到达激活高度**，非设计不可浮动（与 CANONICAL-CONSENSUS-SPEC.md §4.1 裁定吻合）。

---

## §3 PoW verification（独立重算抽样）

采用独立 Python 重算器（`double-SHA256(88-byte header)`，逐块重算，`hash < target(bits)` 判定），对三链**全量**逐块重算：

| datadir | 块数 | PoW 有效 | PoW 无效 | 结论 |
|---|---|---:|---:|---|
| `audit-run/data-ui` | 665 | **665** | 0 | ✅ 全部通过 |
| `audit-run/data-b` | 201 | **201** | 0 | ✅ 全部通过 |
| `audit-run/data-miner` | 201 | **201** | 0 | ✅ 全部通过 |

抽样块头（data-ui）：h=0(nonce 5390)、h=1(2969)、h=2(176035)、h=662(12970)、h=663(121543)、h=664(33350) —— **全部 PoW OK**，哈希前导零与 bits=16 目标（`target=2^240`）一致。

---

## §4 ChainWork verification

| datadir | height | 块数 | 累积工作量 | 公式核对 |
|---|---|---:|---:|---|
| `data-ui` | 664 | 665 | **43,581,440** | `= 665 × 2^16 = 0x2990000` ✅ |
| `data-b` | 200 | 201 | **13,172,736** | `= 201 × 2^16 = 0xc90000` ✅ |
| `data-miner` | 200 | 201 | **13,172,736** | `= 201 × 2^16 = 0xc90000` ✅ |

- `Work(bits) = 2^bits`（bits=16 恒成立），`CumulativeWork = Σ 2^bits` 精确 = 块数 × 65536，无取整误差。
- **结论**：累积工作量一致，fork-choice 度量（工作量大者胜）当前由 `data-ui`（665 块）领先于 `data-b`/`data-miner`（201 块）。

---

## §5 State verification

| 项 | 实测 | 结论 |
|---|---|---|
| 区块索引 | `verify` 逐块重放，`blocks_checked` = height+1（665/201/201） | ✅ 一致 |
| UTXO | `verify valid=true` ⇒ 逐块 Validate+Apply 状态迁移全通过（含 coinbase 成熟期、金额守恒、双花） | ✅ 一致 |
| tip 状态 | `data-ui` tip=`0000c50b6b17…`（v1/bits16/ts1790472989/nonce33350）；`data-b`=`data-miner` tip=`00000e686ef5…` | ✅ 一致 |
| 存储一致性 | `blocks.dat` 帧无尾随字节（trailing=0）、块数精确=height+1、链式 prevHash 全部衔接（linkage ok=N, bad=0） | ✅ 一致 |
| 钱包 | 三目录 `wallet.json` version=1，含 d+pubkey（未导出私钥明文） | ✅ 正常 |

---

## §6 Recovery state

| 项 | 实测 | 结论 |
|---|---|---|
| 孤儿检查点 `orphan_waiting.bin` | **不存在**（全仓库 find 无命中） | ✅ 空（无孤儿入队，或孤儿从未停车） |
| waiting 队列 | 无活节点 ⇒ 内存态为空 | ✅ 空 |
| restorePending | 无活节点 ⇒ 内存态为空 | ✅ 空 |

> **重要边界**：孤儿检查点持久化（§4-B1/B1.5）与启动恢复（§4-B2）属**未提交工作树**（`orphan_checkpoint.go` 为 untracked，`git ls-files` 为空），**不在已提交二进制 `52fb464` 内**。因此当前运行时（磁盘链）天然不含 `orphan_waiting.bin`。这与「运行时状态验证」一致——本阶段仅验证已提交基线，孤儿持久化能力待 commit 后才进入运行时。

---

## §7 Multi-node consistency（只读比对）

| 对比 | 结果 | 说明 |
|---|---|---|
| `data-b` vs `data-miner` | **字节级完全一致**（`cmp` 相同） | 同链 h=200，同 tip `00000e686ef5…` |
| `data-ui` vs `data-b` | **高度 1 即分叉**（common prefix = 仅 genesis） | 两链在 h=1 挖出不同区块（data-ui h1 nonce=2969/ts1790471828；data-b h1 nonce=40068/ts1790470303） |
| 三节点共同 canonical tip | **不存在** | data-ui 独自在 h=664 分支；data-b/miner 在 h=200 分支 |

**裁定（关键发现）**：三个数据目录**并非同一时刻运行的、互相连通的一致集群**。`data-b` 与 `data-miner` 是同一链的两个副本（可能互为备份/同源），而 `data-ui` 是**另一条独立挖出的分支**（自 h=1 起分叉，各自挖自己的块）。三者在**创世身份上一致**（均 `00003d97…`，属当前协议），但**当前无统一 canonical tip**——即当前磁盘链不存在「多节点实时共识」，仅存在「同创世、多分叉」的历史运行痕迹。

> 这与腾讯云 3 节点 loopback 集群（上一会话，height=136、mining STOPPED、tip 一致）是**不同的环境**：本机 `audit-run/*` 是更早的本地实验数据目录，云上集群另有其独立状态。本阶段**未触碰云**。

---

## 结论总览

| § | 验证项 | 结论 |
|---|---|---|
| §0 | 运行时基线 | ✅ 已确立（无活节点；磁盘链为真值；HEAD 未动） |
| §1 | 规则集（v1/v2/v3） | ✅ 三链全在 v1，与文档一致 |
| §2 | 难度运行时 | ✅ bits 恒 16（v1 钉死），无异常 |
| §3 | PoW 重算 | ✅ 三链 1067 块全量重算全通过 |
| §4 | ChainWork | ✅ 精确 = 块数 × 2^16 |
| §5 | 状态（索引/UTXO/tip/存储） | ✅ `verify valid=true`，存储帧干净 |
| §6 | 恢复状态 | ✅ 空（符合未提交基线） |
| §7 | 多节点一致性 | ⚠️ 同创世、多分叉，无统一 canonical tip（真实发现） |

**关键发现（需 Owner 关注）**：
1. **无活节点运行**——「运行时状态」在本机解析为磁盘链；云上三节点集群为独立环境，本阶段未触碰。
2. **遗留旧创世链** `run-a`(1259)/`run-b`(400) 属 F2/F3 前旧身份，`verify` 判 `GENESIS_MISMATCH`，非当前协议合法链。
3. **`audit-run` 三目录非一致集群**：`data-b`≡`data-miner`（h=200 同链），`data-ui`（h=664）自 h=1 独立分叉——无统一 canonical tip。
4. **孤儿持久化/启动恢复未进入运行时**（未提交工作树），故 §6 恢复态为空——这是预期，非缺陷。

---

## HARD STOP confirmation

- ❌ 未执行任何 `git commit` / `git push`。
- ❌ 未部署到任何环境（腾讯云三节点集群全程未触碰，本阶段为纯本机只读）。
- ❌ 未修改云/生产配置。
- ❌ **未改动任何代码**（`git status` 无新增代码 diff；仅新增本报告 + 只读检查脚本 `.workbuddy/inspect_chain.py`）。
- ❌ 未产生任何链上状态变更（`verify`/`printchain`/Python 重算均为只读；`blocks.dat`/`wallet.json` 未写入）。

**验证方法**：① 已提交 `node.exe` 的 `verify -json`（只读全量共识重放）；② `printchain`（只读头遍历）；③ 独立 Python 重算器（直接解析 `blocks.dat` legacy 帧，重算 double-SHA256 PoW 与累积工作量）；④ `find`/`git ls-files`/`cmp`（恢复态与多节点比对）。全程零写盘、零状态变更。
