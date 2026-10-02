# MULTI-NODE-CANONICAL-SYNC-VALIDATION-1-REPORT

> **Phase**：MULTI-NODE-CANONICAL-SYNC-VALIDATION-1 — 真实多节点同步与 canonical 链收敛验证
> **Owner authorization**：PHASE P2PCHAIN — MULTI-NODE-CANONICAL-SYNC-VALIDATION-1
> **Date**：2026-10-02
> **Scope**：Real multi-node synchronization & canonical chain validation · No code changes · No consensus changes · No protocol changes
> **HARD STOP（强制）**：No commit · No deployment · No cloud modification · No code changes

---

## 结论速览

**独立节点确实收敛到同一 canonical 链。** 三节点（A/B/C）从干净创世出发，经真实 P2P 握手 + 批量同步，全部收敛到**同高度 12、同 tip `000074bc…`、同 chainwork 851,968**。分叉收敛测试进一步证明：一条低工作量分叉（8 块）在连接高工作量 canonical 链（12 块）后，经 `REORG_REJECT → FORK_VALIDATION → REORG_ATTEMPT → REORG_ACCEPT` 完整链切换，最终收敛。

---

## §0 Baseline

| 项 | 值 |
|---|---|
| `git rev-parse HEAD` | `52fb464af243fbcdfd78bab846b45ea090e918fb` |
| 二进制 `node.exe` SHA-256 | `fb3d2b57dd8adbd02d3c6d501f42a2cfa3c076657694e35756967caf9307b717` |
| 创世身份 `CanonicalGenesisHash` | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| Go 工具链 | `go1.27.0 windows/amd64` |

> ⚠️ **重要发现（关于二进制身份）**：当前 `node.exe`（今日 01:42 构建）**已包含未提交工作树的 §4-B1/B1.5/B2 孤儿检查点代码**——启动日志明确打印「孤儿等待检查点已启用」并调用 `newOrphanCheckpoint`。即本次验证运行的是**含孤儿持久化 + 启动恢复的二进制**，而非纯 `52fb464` 提交基线。这与上一阶段（CONSENSUS-RUNTIME-VALIDATION-1）「孤儿持久化未进入运行时」的结论形成对照——当时是「无活节点」前提下的磁盘态推断；本阶段以实际运行验证确认：**当前本地 `node.exe` 已含 §4-B1/B1.5/B2**（仍属未提交工作树产物）。

---

## §1 Real node startup（干净运行时状态）

| 节点 | datadir | 启动参数 | 初始高度 | 创世 |
|---|---|---|---|---|
| A（矿工） | scratch/a | `-mine -maxblocks 12 -miners 1` | 0 | `00003d97…` |
| B（跟随） | scratch/b | `-seed 127.0.0.1:16601` | 0 | `00003d97…` |
| C（跟随） | scratch/c | `-seed 127.0.0.1:16601` | 0 | `00003d97…` |
| D（分叉测试） | scratch/d | 先隔离 `-mine -maxblocks 8`，再 `-seed A` | 0 | `00003d97…` |

- 全部用 `node init -datadir` 生成干净 canonical 创世，无历史状态。
- A 挖 12 块后自动转全节点模式（`已达到挖矿上限 12 个区块`）。
- 端口隔离：A=16601/16602、B=16603/16604、C=16605/16606、D=16701/16702（避让项目默认端口与云集群）。

---

## §2 Network connection（握手 / peer 发现 / 区块交换）

**B 同步事件链（节选日志）**：

| 事件 | 内容 |
|---|---|
| 握手 | `DISPATCH_EXIT msg_type=handshake peer=127.0.0.1:16601` |
| 发现对端更高 | `RECOVERY_REQUESTED parent=000074bc… max_ancestors=64` |
| 批量拉取 | `SYNC_BATCH_RECEIVED batch_size=12 cursor_before=0 done=true` |
| 逐块上链 | `新区块已上链: 高度=1..12`（12 条） |
| 应用完成 | `SYNC_BATCH_APPLIED applied=12 cursor_after=12 deferred=0 done=true` |
| 分支补齐 | `RECOVERY_RESPONSE block_count=13 found=true` |

- **peer 发现**：A 显示 `对等节点: 2 个`；B/C 各显示 `对等节点: 1 个 [127.0.0.1:16601]`。
- **区块交换**：B/C 各自经 `blocks_resp` 批量拉取 12 块、逐块共识校验后上链，0 缺父待补。

---

## §3 Fork convergence（fork-choice / 累积工作量 / reorg）

构造：D 隔离挖 8 块（`tip=00004f73…`，高度 8，chainwork = 9×2^16 = 589,824）；连接 A（12 块 canonical，chainwork = 13×2^16 = 851,968）。A 的 chainwork 更高 ⇒ D 必须 reorg。

**D 的 reorg 事件链（完整捕获）**：

| 事件 | 关键字段 |
|---|---|
| `REORG_REJECT` ×4 | `reason=chainwork_not_won`（D 的分叉块高度 5–8 逐块被拒） |
| `FORK_VALIDATION_START/END` | `result=ok`（分叉路径校验通过） |
| `REORG_ATTEMPT` | `old_tip=00004f73…(D) → new_tip=0000c9de…(A)` |
| `REORG_ACCEPT` | `attached_count=8, detached_count=8`（换入 A 的 8 块、卸下 D 的 8 块） |
| 收敛 | D 最终高度 12、tip `000074bc…`（= A） |

**验证结论**：
- **fork-choice**：`Work(bits)=2^bits`，累积工作量大者胜（A 851,968 > D 589,824 ⇒ A 胜）。
- **累积工作量比较**：`REORG_REJECT reason=chainwork_not_won` 显式体现「低工作量分叉块不纳入 canonical」。
- **reorg 行为**：disconnect（D 的 8 块）→ attach（A 的 8 块）→ 更新 tip，全程原子。

---

## §4 Final consistency（全部节点收敛）

| 节点 | height | tip hash | chainwork | 状态 |
|---|---|---:|---:|---|
| A | 12 | `000074bcc1d5cc724974bc6e162df3ddd6681e0c46dc840913f8810b9d7adda0` | 851,968 | 全节点 |
| B | 12 | 同上 | 851,968 | 全节点 |
| C | 12 | 同上 | 851,968 | 全节点 |
| D | 12 | 同上 | 851,968 | reorg 后收敛 |

- **同高度**：12 ✅
- **同 tip hash**：`000074bc…` ✅
- **同 chainwork**：851,968（= 13 块 × 2^16）✅
- **同状态**：`node verify` 三链均 `valid=true`（UTXO 状态迁移一致）✅

> chainwork 精确性：13 块（含创世）× `2^16` = 851,968，与独立 Python 重算一致。

---

## §5 Legacy chain classification（历史遗留 datadir 归类）

| datadir | 状态 | 归类 |
|---|---|---|
| `audit-run/data-ui`（h=664） | `verify valid=true`，genesis `00003d97…` | 当前协议历史实验链 |
| `audit-run/data-b` / `data-miner`（h=200） | `verify valid=true`，字节级一致 | 当前协议历史实验链 |
| `run-a`（h=1259）/ `run-b`（h=400） | `verify GENESIS_MISMATCH`（旧 genesis `0000aca1…`） | **历史遗留（旧创世身份，F2/F3 冻结前），非当前协议合法链** |
| 本次 scratch/a,b,c,d | 干净创世，验证后已停 | 临时验证产物 |

**裁定**：`run-a`/`run-b` 属旧创世身份的**历史遗留**，仅作历史参考，不得视为当前协议链；`audit-run/*` 属当前身份的历史实验链（上一阶段已确认非互联一致集群）；本次 scratch 为临时验证产物，验证完成后已关闭。

---

## 关键发现汇总

1. **多节点收敛实证成立**：3 独立节点 + 1 分叉节点，全部收敛到同一 canonical（高度/tip/chainwork 一致）。
2. **fork-choice + reorg 全链路实证**：`REORG_REJECT(chainwork_not_won) → FORK_VALIDATION(ok) → REORG_ATTEMPT → REORG_ACCEPT(8 attach/8 detach)`，累积工作量大者胜。
3. **当前 `node.exe` 二进制含未提交 §4-B1/B1.5/B2**（启动日志「孤儿等待检查点已启用」实证）——这是工作树构建产物，非 `52fb464` 提交基线；孤儿持久化/启动恢复代码已随二进制进入运行时，但仍未 git commit。
4. **遗留旧创世链** `run-a`/`run-b` 归历史。

---

## HARD STOP confirmation

- ❌ 未执行任何 `git commit` / `git push`。
- ❌ 未部署到任何环境（腾讯云三节点集群未触碰）。
- ❌ 未修改云/生产配置。
- ❌ **未改动任何代码**（本阶段仅新增 `.workbuddy/mnsv_experiment.sh`、`.workbuddy/mnsv_fork_test.sh` 两个临时脚本 + 本报告；`git status` 中 `cmd/`、`internal/` 的 diff 均为前序 B1/B1.5/B2 阶段既存，本阶段零新增）。
- ❌ 未产生任何生产数据变更（实验仅在隔离 scratch 目录，验证后已停节点并清理端口监听）。

**验证方法**：① 已提交工作树二进制 `node.exe`（含未提交 B1/B1.5/B2 代码）真实启动 3+1 节点；② 真实 TCP P2P 握手/批量同步/分支补齐；③ 真实分叉 → reorg 全链路；④ `node status`/`verify`/独立 Python 重算交叉核对 chainwork 与 tip。全程隔离、零污染主仓库状态。
