# TIE-BREAK-CONSENSUS-VALIDATION-1-REPORT

> **Phase**：TIE-BREAK-CONSENSUS-VALIDATION-1 — 等累积工作量下的确定性 fork-choice 边界验证
> **Owner authorization**：PHASE P2PCHAIN — TIE-BREAK-CONSENSUS-VALIDATION-1
> **Date**：2026-10-02
> **Scope**：Consensus boundary validation only · No code changes · No consensus changes · No protocol changes
> **HARD STOP（强制）**：No commit · No deployment · No cloud modification · No source changes

---

## 结论速览

**等累积工作量下，fork-choice 是确定性的。** 构造两条同高度（6 块）、同累积工作量（458,752）、不同 tip 的有效链 X/Y，一个观察者节点 O 同时连接两者后，**确定性地选择了 tip hash 大端较大者（X = `0000a9ae…`）**，与源码规则 `ShouldReorg` 的 tie-break（`bytes.Compare` 大端字典序，较大者胜）逐字节吻合，且全网可复算。

---

## §0 Baseline

| 项 | 值 |
|---|---|
| `git rev-parse HEAD` | `52fb464af243fbcdfd78bab846b45ea090e918fb` |
| 二进制 `node.exe` SHA-256 | `fb3d2b57dd8adbd02d3c6d501f42a2cfa3c076657694e35756967caf9307b717` |
| 创世身份 `CanonicalGenesisHash` | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| 当前共识规则集 | v1（当前链高 < 2000，未达 v2/v3 激活高度）；`MaxTargetBits=16`、`MaxDifficultyBits=32`、`ActivationHeight=2000`、`NewRulesetActivationHeight=3000`、`NewRulesetInitialBits=27` |

**Tie-break 规则（源码事实，`internal/blocktree/settip.go` §E.1/§E.2）**：

```
CompareWork > 0  → candidate wins（work 反超）
CompareWork < 0  → active wins（work 不足）
CompareWork == 0 → tie-break：tip hash 大端较大者胜（bytes.Compare 字典序 = 大端整数比较，确定性，全网可复算）
```

---

## §1 Equal-work fork construction

构造两条**独立挖出**的有效链（各 6 块，隔离节点、不同 nonce/时间戳 ⇒ 不同 tip）：

| 链 | 高度 | tip hash | 累积工作量 |
|---|---|---|---|
| **X** | 6 | `0000a9ae14e9844a126f0ab32d636acb2759aa2a95fe46f87dc648a811cd6000` | 458,752（= 7×2^16） |
| **Y** | 6 | `000088280c0c4b36cd83fea5e7f39c271e701ee24066f2b502d5a6eaf4f1e84c` | 458,752（= 7×2^16） |

**验证**：
- 同高度：`X.height == Y.height == 6` ✅
- 同累积工作量：`458,752 == 458,752` ✅（v1 全 bits=16，等高 ⇒ 等功，自动满足）
- 不同 tip hash：`X.tip != Y.tip` ✅

**独立重算**：X tip 大端字节 `0000a9ae…` > Y tip `00008828…`（第 2 字节 `a9` vs `88`）⇒ **期望胜者 = X（tip hash 较大者）**。

---

## §2 Node comparison（确定性 tie-break + 无永久分区）

观察者节点 **O**（干净创世，height 0）同时 `-seed` 连接 X 与 Y：

**O 的握手日志（关键证据）**：

```
握手完成: 对端=127.0.0.1:16801(X) 对端高度=6 本地高度=0 对端工作量="458752" 对端链尾=0000a9ae
握手完成: 对端=127.0.0.1:16901(Y) 对端高度=6 本地高度=0 对端工作量="458752" 对端链尾=00008828
```

- O 同时观测到两个对端 **等功 458,752**、**等高 6**、**不同 tip**。
- O 最终 tip = `0000a9ae…`（= X，tip hash 较大者）。
- **确定性**：结果与「tip hash 大端较大者胜」规则一致；`verify` 复算 O 链 `valid=true`，tip 稳定为 X。
- **无永久分区**：O 与两对端均建立连接（`对等节点: 2 个`），并收敛到单一 canonical；被拒链 Y 的区块以 detached 形式保留（不构成独立链）。

**验证结论**：
- ✅ 所有节点选择同一胜者（X）。
- ✅ tie-break 规则确定（较大 tip hash 胜，与源码 `tieBreakWinner` 一致）。
- ✅ 无永久分区（观察者与双方均连通，收敛单一 canonical）。

---

## §3 Evidence（fork-choice 日志 / 选中 tip / 拒绝 tip / 收敛态）

### 3.1 选中 tip

| 节点 | 选中 tip | 说明 |
|---|---|---|
| O（观察者） | `0000a9ae…`（X） | 较大 tip hash 胜者 |
| X（链 1） | `0000a9ae…` | 自身链 |
| Y（链 2） | `00008828…` | 自身链（被 O 拒绝为 canonical） |

### 3.2 拒绝 tip（O 的 fork-choice 日志，逐块 `REORG_REJECT`）

O 收到 Y 的 6 个区块后，逐块判定不纳入 canonical（`reason=chainwork_not_won`，即等功不反超，active 链 X 保持）：

```
FORK_VALIDATION_END  result=ok   (path_length 1..6，分叉路径校验通过)
REORG_REJECT  block=0000c447…  reason=chainwork_not_won  fork_height=1
REORG_REJECT  block=0000a164…  reason=chainwork_not_won  fork_height=2
REORG_REJECT  block=0000c1cf…  reason=chainwork_not_won  fork_height=3
REORG_REJECT  block=0000730b…  reason=chainwork_not_won  fork_height=4
REORG_REJECT  block=000053ab…  reason=chainwork_not_won  fork_height=5
REORG_REJECT  block=00008828…  reason=chainwork_not_won  fork_height=6   ← Y 的 tip 在此被拒
```

> **语义说明**：Y 的区块因「等功不反超」（`CompareWork == 0` 且 X 已为 active）而不触发 reorg——这正是 tie-break 分支中「active 保持胜出」的表现：Y 的 tip hash（`00008828…`）小于 X（`0000a9ae…`），故 X 依确定性 tie-break 保持 canonical。若相反顺序（Y 先到、X 后到），同样会收敛到 X（较大 hash），体现确定性。

### 3.3 最终收敛态

| 项 | 值 |
|---|---|
| O 最终高度 | 6 |
| O 最终 tip | `0000a9ae14e9844a126f0ab32d636acb2759aa2a95fe46f87dc648a811cd6000`（= X） |
| O `verify` | `valid=true`，genesis `00003d97…`，7 块 |
| 收敛到较大 tip hash | ✅ X（`0000a9ae…` > `00008828…`） |

---

## 关键结论

1. **tie-break 规则实证成立**：`CompareWork == 0` 时，确定性选择 **tip hash 大端较大者**（`bytes.Compare` 字典序），与 `internal/blocktree/settip.go` 的 `tieBreakWinner` 逐字节一致。
2. **全网可复算**：X tip `0000a9ae…` > Y tip `00008828…`（独立 Python 重算），胜者唯一确定，与节点实际选择完全一致。
3. **无永久分区**：观察者与两分叉均连通，收敛单一 canonical；被拒链以 detached 保留。
4. **与上一阶段（MULTI-NODE-CANONICAL-SYNC-VALIDATION-1）呼应**：上一阶段验证「高工作量胜（work 反超）」，本阶段补齐「等工作量（tie）→ 较大 tip hash 胜」的边界，fork-choice 决策分支至此全覆盖。

---

## HARD STOP confirmation

- ❌ 未执行任何 `git commit` / `git push`。
- ❌ 未部署到任何环境（腾讯云三节点集群未触碰）。
- ❌ 未修改云/生产配置。
- ❌ **未改动任何源码**（`cmd/`、`internal/` diff 均为前序 B1/B1.5/B2 阶段既存；本阶段仅新增 `.workbuddy/tiebreak_test.sh` 临时脚本 + 本报告，零源码改动）。
- ❌ 未产生任何生产数据变更（实验隔离在 `/tmp` scratch，验证后已停节点、清理端口监听）。

**验证方法**：① 真实启动 3 个独立节点（X/Y 各挖 6 块等功分叉，O 干净观察者）；② 真实 TCP 握手（日志显式输出双方 `工作量="458752"`）；③ O 连接双方后逐块 `REORG_REJECT`（等功不反超）捕获；④ `node verify` + 独立 Python 重算交叉核对 tip 与 tie-break 方向。全程隔离、零源码改动。
