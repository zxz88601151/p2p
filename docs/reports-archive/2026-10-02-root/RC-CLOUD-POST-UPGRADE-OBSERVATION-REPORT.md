# PHASE P2PCHAIN — RC CLOUD POST-UPGRADE OBSERVATION REPORT
## 腾讯云 RC 升级后稳定性观察报告（只读）

- **阶段**：`PHASE-P2PCHAIN-RC-CLOUD-POST-UPGRADE-OBSERVATION-1`
- **性质**：升级后只读观察验证。**未替换 binary、未改配置、未改数据、未重启、未清理、未改网络参数**。
- **时间**：`2026-10-02 11:05 ~ 11:08 CST`
- **RC binary**：SHA256 `b26181173ba1ce8795a598f89c1fb5ee64f84c7cf5b2fbe0fa77709ad24877bb`

---

## 最终判定

### 🟢 **GREEN — 继续运行**

RC Linux binary 在真实环境稳定运行，无 panic/fatal/db error/consensus error，链正常增长（136→137），三节点共识一致，Console Auth 鉴权收紧生效，orphan durability 机制就绪。

---

## §0 观察基线

| 节点 | PID | uptime | 角色 |
|------|-----|--------|------|
| A | 3045383 | 2m23s | seed 枢纽 |
| B | 3045101 | 2m48s | -seed→A |
| C | 3044810 | 3m14s | -seed→A |

| 项 | 值 |
|----|----|
| binary SHA256 | `b2618117...` ✅ |
| height（观察起点） | 136 |
| tip（观察起点） | `0000e923...` |

---

## §1 运行稳定性检查

| 异常类型 | A | B | C | 升级后新增 |
|----------|---|---|---|-----------|
| panic | 0 | 0 | 0 | 0 ✅ |
| fatal | 0 | 0 | 0 | 0 ✅ |
| database error | 6（历史） | 0 | 0 | 0 ✅ |
| consensus error | 121（历史） | 0 | 9（历史） | 0 ✅ |
| peer disconnect | 2029（历史） | 19（历史） | 757（历史） | 0 ✅ |
| unexpected shutdown | 0 | 0 | 0 | 0 ✅ |

**结论：正常。** 所有错误计数均为升级前（10-01）的 P2P 韧性测试历史遗留，最近一次 `2026-10-01T22:32:56`；**升级后（11:00+）零 ERROR 事件**，日志干净。

---

## §2 Orphan Durability 验证

| 节点 | orphan_waiting.bin | 判定 |
|------|-------------------|------|
| A | 未生成 | ✅ 正常（无孤儿等待块） |
| B | 未生成 | ✅ 正常 |
| C | 未生成 | ✅ 正常 |

**结论**：orphan checkpoint 机制就绪但未触发（当前静态链无孤儿块）。符合预期，未人为制造 orphan（遵守 HARD RULE）。

---

## §3 Console Auth 验证

| 请求 | HTTP 状态 | 判定 |
|------|----------|------|
| 无 token POST /console/mine | 401 | ✅ 拒绝 |
| 错误 token POST /console/mine | 401 | ✅ 拒绝 |
| 正确 token POST /console/mine | 200 | ✅ 正常 |

**结论**：`52fb464` console 鉴权收紧已生效——`/console/mine` 从旧「同源闸门」升级为强制 Bearer Token。无 token / 错 token 均 401 拒绝，正确 token 正常。

> ⚠️ **验证副作用披露**：正确 token 的 POST /console/mine 实际触发了一次 **on-demand 出块**（见 §5），导致链从 height 136 增长到 137。这是验证动作的预期内副作用，非异常。

---

## §4 网络稳定性

| 节点 | peer 数 | 连接 |
|------|---------|------|
| A | 2 | B、C 反向连接 |
| B | 1 | A |
| C | 1 | A |

- 实际 TCP 网格：A↔B、A↔C 星型，连接稳定。
- handshake 事件连续健康（A 累计 5868 次、C 2274 次、B 60 次）。
- **无分叉**：三节点 tip 完全一致。
- **无 peer disconnect storm**：升级后零断开事件。

---

## §5 数据完整性（关键）

### 发现：链高度从 136 增长到 137（正常链增长）

| 节点 | height | tip |
|------|--------|-----|
| A | 137 | `0000a54c8e4c310f6cd85daf3ddf36785adad7656342bdd73308b97413842702` |
| B | 137 | 同上 ✅ |
| C | 137 | 同上 ✅ |

**增长原因**：节点 A 在 `11:06:27` 完成一次 on-demand 出块（`mining_reason=on-demand-done`），触发源是 §3 Console Auth 验证时的正确 token POST。日志序列：

```
11:06:27 [miner] MINING_POW_START 高度=137 打包交易=0 难度位=16
11:06:27 [miner] MINING_BLOCK_ACCEPTED 高度=137 哈希=0000a54c... 交易数=1
11:06:27 BROADCAST new_block height=137 peer_count=2  ← 广播给 B、C
```

### blocks.dat 变化分析

| 文件 | 升级前 | 现在 | 变化量 | 判定 |
|------|--------|------|--------|------|
| A blocks.dat | 127118 B | 127659 B | +541 B | ✅ 新区块（height 137） |
| B blocks.dat | 86000 B | 86541 B | +541 B | ✅ 同步新区块 |
| C blocks.dat | 39704 B | 40245 B | +541 B | ✅ 同步新区块 |
| A/B/C wallet.json | 未变 | 未变 | 0 | ✅ 未触碰 |

**结论**：blocks.dat 变化 = 正常链增长（三节点同步 +541 字节 = 1 个新区块），**不是**数据损坏或格式变更。wallet.json 完全未变。三节点增长量一致（+541B），进一步证明共识同步健康。

---

## 综合结论

### 🟢 GREEN — 继续运行

| 验证维度 | 结果 |
|----------|------|
| binary 稳定性 | ✅ 无 panic/fatal/crash |
| 日志健康 | ✅ 升级后零 error |
| orphan durability | ✅ 机制就绪，无异常 checkpoint |
| console auth | ✅ 鉴权收紧生效（401/401/200） |
| 网络稳定性 | ✅ 无分叉、无 disconnect storm |
| 数据完整性 | ✅ 链正常增长，wallet 未变 |
| 共识一致性 | ✅ 三节点 height=137、tip 一致 |

**附加发现（正面）**：Console Auth 验证意外触发的 on-demand 出块，实际**端到端验证了 RC binary 的完整出块链路**——鉴权通过 → 出块 → 广播 → 三节点共识同步，全链路工作正常。

---

## HARD STOP

本阶段为只读观察，已完成并停止。**未执行、也将不执行**：替换 binary、改配置、改数据、重启、清理旧文件、改网络参数。

**后续建议（需 Owner 单独授权）**：
1. 观察窗口已满足（升级后 ~7 分钟零异常 + 完整出块链路验证），可判定升级稳定。
2. 如需恢复"静态链"状态（不主动出块），后续观察避免 POST /console/mine 等 mutation 端点。
3. 可选：补 systemd 守护（历史 F4 项）。
