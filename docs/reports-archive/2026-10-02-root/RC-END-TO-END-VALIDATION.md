# PHASE P2PCHAIN — RC END-TO-END VALIDATION
## 腾讯云 v0.9.0-rc1 端到端验证记录

- **阶段**：`PHASE-P2PCHAIN-RC-PRODUCTION-BASELINE-FREEZE-1` §3
- **性质**：只读验证记录，未触发新的 mining（本节仅记录已发生的验证事实）。
- **时间**：`2026-10-02 11:08 CST`

---

## 验证总览

RC 升级从 c3cec3f3 → 2647ba0（v0.9.0-rc1）的完整链路，已在真实腾讯云三节点环境完成端到端验证。以下记录各功能模块的验证结果。

---

## §3.1 Console Auth 验证（`52fb464` 功能）

| 请求场景 | HTTP 结果 | 判定 |
|----------|----------|------|
| 无 token POST `/console/mine` | 401 | ✅ 拒绝 |
| 错误 token POST `/console/mine` | 401 | ✅ 拒绝 |
| 正确 token POST `/console/mine` | 200 | ✅ 正常 |

**结论**：console 挖矿端点从旧「同源闸门」（`consoleOriginGate`）收紧为强制 Bearer Token（`requireAuth`），鉴权生效。CLI/第三方脚本的 `/mine` + Bearer Token 语义不变。

---

## §3.2 Mining → Broadcast → Sync 链路验证

正确 token 触发 on-demand 出块后，完整链路在真实环境验证通过：

```
鉴权通过(200)
   ↓
[11:06:27] MINING_POW_START 高度=137 打包交易=0 难度位=16
   ↓
[11:06:27] MINING_BLOCK_ACCEPTED 高度=137 哈希=0000a54c... 交易数=1
   ↓
[11:06:27] BROADCAST new_block height=137 peer_count=2（→ B、C）
   ↓
B、C 同步新区块，三节点 height 136→137，tip 一致 0000a54c...
```

| 链路环节 | 证据 | 判定 |
|----------|------|------|
| 鉴权 | 正确 token 200 / 错 token 401 | ✅ |
| 出块 | `MINING_BLOCK_ACCEPTED height=137` | ✅ |
| 广播 | `BROADCAST new_block peer_count=2` | ✅ |
| 同步 | 三节点 height=137、tip 一致 | ✅ |
| 共识 | 三节点 tip `0000a54c...` 完全一致 | ✅ |

**结论**：RC binary 的出块、广播、共识同步全链路在真实环境工作正常。

---

## §3.3 Orphan Durability 验证（`36f9630` 功能）

| 检查项 | 结果 | 判定 |
|--------|------|------|
| orphan_waiting.bin 生成 | 三节点均未生成 | ✅ 正常（无孤儿块） |
| checkpoint magic | 未触发（无需检查） | ✅ |
| 异常 checkpoint | 无 | ✅ |

**结论**：orphan durability 机制（`orphan_checkpoint.go`）已随 RC binary 部署并就绪，但当前静态链无孤儿块，机制未触发属正常状态。（未人为制造 orphan，遵守 HARD RULE。）

---

## §3.4 Genesis Identity Guard 验证（`c3cec3f3` 功能，升级前已含）

| 检查项 | 结果 | 判定 |
|--------|------|------|
| 创世块 identity | `00003d97...` 无漂移 | ✅ |
| O1 guard 拒绝事件 | 升级后握手零 genesis mismatch 拒绝 | ✅ 正常 |
| 三节点创世一致 | 均从同一创世哈希派生 | ✅ |

**结论**：O1 genesis identity guard 正常，三节点同网同源，无跨网异源风险。

---

## 端到端验证总结

| 功能模块 | 对应提交 | 验证结果 |
|----------|---------|---------|
| Genesis identity guard | c3cec3f3 | ✅ 正常 |
| Console mining auth | 52fb464 | ✅ 收紧生效（401/401/200） |
| Orphan durability | 36f9630 | ✅ 机制就绪 |
| 确定性构建（buildvcs=false） | 2647ba0 | ✅ 无 vcs 内嵌 |
| 出块+广播+同步 | 全链路 | ✅ 真实环境验证通过 |

### 🟢 **端到端验证完整通过**

v0.9.0-rc1 在腾讯云三节点真实运行环境中，从 binary 身份、功能模块到完整出块链路全部验证通过，可作为生产候选基线。
