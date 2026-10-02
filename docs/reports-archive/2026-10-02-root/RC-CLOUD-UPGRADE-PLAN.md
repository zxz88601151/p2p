# PHASE P2PCHAIN — RC CLOUD UPGRADE PLAN
## 腾讯云节点 v0.9.0-rc1 升级设计方案（只读 + 本地构建，未执行升级）

- **阶段**：`PHASE-P2PCHAIN-RC-CLOUD-UPGRADE-PLAN-1`
- **目标**：设计云上节点从 `c3cec3f3` 升级到 `v0.9.0-rc1`（`2647ba0`）的完整方案
- **性质**：本阶段**仅做冻结快照 + 本地构建 + 影响分析 + 回滚设计**。**未上传、未替换、未重启、未迁移数据**。
- **生成时间**：`2026-10-02 10:57~10:58 CST`

---

## §0 云节点冻结快照（升级前基线）

### 运行实例

| 节点 | PID | 角色 | P2P 监听 | RPC | DataDir |
|------|-----|------|----------|-----|---------|
| A | 2872772 | seed 枢纽 | 127.0.0.1:16688 | 127.0.0.1:16689 | /data/p2pchain/blockchain |
| B | 2873329 | -seed → A | 127.0.0.1:16690 | 127.0.0.1:16691 | /data/p2pchain/blockchain-b |
| C | 2872762 | -seed → A | 127.0.0.1:16692 | 127.0.0.1:16693 | /data/p2pchain/blockchain-c |

### 冻结快照（升级回滚依据）

| 项 | 值 |
|----|----|
| 运行 binary | `/opt/p2pchain/bin/p2pchain` |
| binary SHA256 | `a95df7c9a960fd1896ff4d5b1e2504ea0778fcba87961f6423f8a9e970c5f217` |
| block height | 136（三节点一致） |
| tip hash | `0000e923a6f536b23dc8dc1ebe1e5d4b27d5fce657d7d7507e4d63fed09ed544`（三节点一致） |
| peers | A↔B、A↔C（星型，B/C 各连 A） |
| config (control-token) SHA256 | `654a27379c0d0d3e005bc87db9f5286d8087cf05f1fd633b8d2df266e87bd455` |

### 数据目录（升级前基线）

| DataDir | 大小 | blocks.dat SHA256 | wallet.json SHA256 |
|---------|------|-------------------|--------------------|
| blockchain (A) | 144K | `9035fb101cd7f46c503a1f7d1b4e370eeea0e1f0ca4f3096ab471b0b1a6227d0` | `02f19c1da42dd06e76981c98c0a98721dbda117ddaa7f7c4c6b289258f1e7c32` |
| blockchain-b (B) | 100K | `ea733ecebdda18dd596ca174f68236a8880425a10515ae87118ef6546ca9c97e` | `c36a44b3da9d44ce0582b8dbcceb3b5d2e015e08d29681da362af01aee9a4087` |
| blockchain-c (C) | 56K | `a1c606c2e91f4310d0889c7bdfaf53754051df03c360bd8e72a689cbc7e4d6ee` | `da7342364f2002605acea7c683030f2a5e123f82c3248b2edaabd2eef2fb9701` |

---

## §1 RC Linux Binary 准备

### 构建参数（严格按 RC 发布基准）

| 参数 | 值 |
|------|----|
| 目标版本 | `v0.9.0-rc1` |
| 源码 | `2647ba0eb23a94a11aa71326e9b0686bbe1b78d8`（HEAD = v0.9.0-rc1 tag） |
| GOOS | linux |
| GOARCH | amd64 |
| CGO_ENABLED | 0 |
| buildvcs | false |
| trimpath | true |
| Go 版本 | go1.27.0 |

### 构建产物

| 项 | 值 |
|----|----|
| 文件名 | `p2pchain-linux-amd64-v0.9.0-rc1` |
| 大小 | 11,950,947 bytes |
| SHA256 | `b26181173ba1ce8795a598f89c1fb5ee64f84c7cf5b2fbe0fa77709ad24877bb` |
| 内嵌 vcs.revision | **0 处**（`-buildvcs=false` 生效，与 RC 外部证明模型一致） |
| 确定性复验 | ✅ 两次独立构建 SHA256 完全一致 |

> **说明**：Linux binary SHA256（`b2618117...`）与 Windows 发布的 `71097357...` 天然不同——不同 GOOS 目标的产物哈希必然不同。二者均来自同一 `2647ba0` 源码 + 相同确定性构建参数（`-buildvcs=false -trimpath`），身份一致，仅目标平台不同。

### 产物位置（本机）

```
release/v0.9.0-rc1/p2pchain-linux-amd64-v0.9.0-rc1
release/v0.9.0-rc1/p2pchain-linux-amd64-v0.9.0-rc1.sha256
```

---

## §2 升级影响分析（c3cec3f3 → 2647ba0）

### 代码变更全貌

`c3cec3f3..2647ba0` 共 **11 个 .go 文件**变更（+1889 / −138），全部来自 2 个功能提交：

| 提交 | 文件 | 类型 |
|------|------|------|
| `52fb464` | `internal/control/server.go` + `console_mine_auth_test.go` | console 鉴权收紧 |
| `36f9630` | `cmd/node/main.go` + `service.go` + `orphan_checkpoint.go`（新增）+ 6 个测试 | orphan 崩溃恢复韧性 |

### 逐项兼容性判定

| 维度 | 是否变化 | 详情 |
|------|---------|------|
| **database format** | ❌ 未变 | `internal/blockchain/*` 零改动；`blocks.dat` 序列化格式不变 |
| **genesis** | ❌ 未变 | genesis 参数/创世块哈希无变化（`internal/blockchain` 无 diff） |
| **chain compatibility** | ✅ 完全兼容 | 链数据格式不变，可原地读取现有 height=136 链 |
| **wallet compatibility** | ❌ 未变 | `wallet.json` 格式零改动 |
| **network protocol** | ❌ 未变 | `internal/p2p/*` 零改动；handshake 消息结构不变（O1 guard 已在 c3cec3f3 内含） |
| **RPC protocol** | ⚠️ 收紧（向后兼容） | `/console/mine` 从「同源闸门」改为强制 Bearer Token；`/mine` 语义不变 |

### 新增磁盘文件（唯一的结构性变化）

`36f9630` 引入 `<datadir>/orphan_waiting.bin` 检查点文件：

- **新增文件**，不影响现有 `blocks.dat` / `wallet.json` 读取。
- 头部 41 字节：magic `"ORPH"`(4B) + version `0x01`(1B) + entryCount(4B) + checksum(32B)。
- 原子写（temp + fsync + rename），仅在孤儿等待块非空时写入。
- 升级后首次运行若无非空孤儿等待集，该文件不产生。

### 升级性质判定

> **兼容性升级（non-breaking）**。无数据库迁移、无链重置、无 reindex、无 wallet 迁移。升级 = 替换 binary + 重启，数据目录原样复用。

---

## §3 回滚方案设计

### 3.1 备份清单（升级前必须完成）

| 备份对象 | 源路径 | 备份到 | 方法 |
|----------|--------|--------|------|
| 运行 binary | `/opt/p2pchain/bin/p2pchain` | `/opt/p2pchain/bin/p2pchain.pre-rc1` | `cp -p` |
| config | `/opt/p2pchain/secrets/control-token` | 原样保留（不替换） | — |
| A blocks.dat | `/data/p2pchain/blockchain/blocks.dat` | `/opt/p2pchain/backups/pre-rc1-blockchain-blocks.dat` | `cp -p` |
| B blocks.dat | `/data/p2pchain/blockchain-b/blocks.dat` | `/opt/p2pchain/backups/pre-rc1-blockchain-b-blocks.dat` | `cp -p` |
| C blocks.dat | `/data/p2pchain/blockchain-c/blocks.dat` | `/opt/p2pchain/backups/pre-rc1-blockchain-c-blocks.dat` | `cp -p` |
| A/B/C wallet.json | `/data/p2pchain/*/wallet.json` | `/opt/p2pchain/backups/pre-rc1-*.wallet.json` | `cp -p` |

> 备份完成后逐项 `sha256sum` 校验，与 §0 冻结快照哈希比对一致方可继续升级。

### 3.2 回滚触发条件（rollback trigger）

满足**任一**即触发回滚：

| 触发条件 | 判定依据 |
|----------|---------|
| 启动失败 | RC binary 启动即 crash / panic / exit≠0 |
| 链数据无法加载 | 启动后 height≠136 或 blocks.dat 读取报错 |
| tip 哈希漂移 | 任一节点 tip ≠ `0000e923...ed544` |
| 共识分裂 | 三节点 height/tip 不一致超过观察窗口 |
| RPC 异常 | `/status`、`/block` 端点 500 / 超时 |
| orphan checkpoint 异常 | `orphan_waiting.bin` 损坏 / 解析失败（checkpoint fail-closed 报错） |
| 观察期性能劣化 | 连续错误日志 / handshake 失败 / peer 频繁断开 |

### 3.3 回滚步骤（触发后执行）

1. `kill <PID>` 三个 RC 进程。
2. 恢复 binary：`cp -p /opt/p2pchain/bin/p2pchain.pre-rc1 /opt/p2pchain/bin/p2pchain`。
3. 恢复数据（若 chain 被 RC 写入污染）：从 `/opt/p2pchain/backups/pre-rc1-*` 还原 blocks.dat + wallet.json。
4. 按原参数重启三节点（顺序：A → B → C）。
5. 验证 height=136、tip=`0000e923...` 恢复一致。

### 3.4 升级执行顺序（供后续授权参考，本阶段未执行）

1. 停止 C → 替换 → 重启 → 验证。
2. 停止 B → 替换 → 重启 → 验证。
3. 停止 A（seed）→ 替换 → 重启 → 验证。
4. 三节点全量一致性复核（height=136、tip 一致、peer 网格恢复）。

---

## §4 最终状态

### 本阶段产出

- `release/v0.9.0-rc1/p2pchain-linux-amd64-v0.9.0-rc1`（+ `.sha256`）
- 本方案文档 `RC-CLOUD-UPGRADE-PLAN.md`

### 升级可行性结论

🟢 **GREEN — 升级可行，且为兼容性升级（non-breaking）**

- 数据库格式、genesis、wallet、network protocol 均无破坏性变化。
- 唯一结构性变化为新增 `orphan_waiting.bin`（崩溃恢复检查点），不影响现有数据。
- 已准备好确定性构建的 RC Linux binary（SHA256 `b2618117...`，确定性复验通过）。
- 回滚方案完备（binary + blocks.dat + wallet.json 全量备份 + 明确触发条件）。

---

## HARD STOP

本阶段为设计阶段，已完成并停止。**未执行、也将不执行**（未经 Owner 单独授权）：

- ❌ 上传 binary 到云
- ❌ 替换 `/opt/p2pchain/bin/p2pchain`
- ❌ restart 节点
- ❌ 数据迁移

**待 Owner 授权后，方可进入升级执行阶段（下一步骤清单见 §3.4）。**
