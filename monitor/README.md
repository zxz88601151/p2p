# P2PChain V3 Activation Window 本地监控

只读监控生产链 V3 共识激活过渡（2999 → 3000 → 3001）的本地 daemon。

## 安全边界（HARD SAFETY）

本监控**只读**，绝不干预生产：

- 不启动/停止 miner，不重启节点，不修改配置/源码/数据库/链数据
- 不手工推进高度，不 rollback / reset，不 deploy / commit / push
- 唯一允许的生产动作：**READ**

监控程序崩溃不会影响生产节点（无任何写路径）。

## 文件

| 路径 | 说明 |
|---|---|
| `v3monitor.py` | 核心监控（单次 / daemon / mock 三模式） |
| `evidence/snapshots/` | 每轮快照（时间戳命名，不可覆盖） |
| `evidence/transition/` | 2999/3000/3001 逐块证据（存在则比较差异，不静默覆盖） |
| `evidence/history/` | history.jsonl 追加式历史 |
| `evidence/alerts/` | INFO / WARNING / CRITICAL 分级告警 |

## 用法

```bash
cd monitor
python v3monitor.py once                  # 单次采集
python v3monitor.py daemon --interval 60  # 持续监控
python v3monitor.py mock                  # 状态机回归测试
```

## 数据源（只读）

经 SSH（111.229.225.123:2222, root）到生产主机本地回环，读三节点控制 RPC：

- `GET /status`（三节点 A=16689 / B=16691 / C=16693）
- `GET /blocks?from=<h>&count=1`（区块明文字段：hash/previous_hash/version/bits/difficulty/nonce/merkle_root/timestamp/consensus_era）
- `sha256sum /opt/p2pchain/bin/p2pchain`（binary provenance）

## 状态机

```
height < 2990            -> LOW_FREQUENCY
2990 <= height < 2999    -> HIGH_FREQUENCY
height >= 2999           -> TRANSITION_WINDOW（捕获 2999/3000/3001 逐块证据）
height > 3001 且证据不全 -> TRANSITION_EVIDENCE_INCOMPLETE（INVESTIGATION REQUIRED）
```

终态三选一：`IN PROGRESS` / `INVESTIGATION REQUIRED` / `PASS_CANDIDATE`。

`PASS_CANDIDATE` 仅当 2999=V2、3000=V3(bits=30)、3001=V3 全部捕获且正确链接、三节点一致、
binary provenance 正确、无未解释异常时才进入；**仍非最终 release PASS**。

## 冻结真值

```
SOURCE_COMMIT          = 4cc474ca2e35f34ec7c867b729f883333b78b1f1
RELEASE_TAG            = v0.9.0-rc6
EXPECTED_BINARY_SHA256 = 9641d88a2da854306a6643256c9796fafa5a7ce119dab1697b9a74e33860a5e4
ACTIVATION_HEIGHT      = 3000
NEW_RULESET_INITIAL_BITS = 30
NEW_RULESET_BLOCK_VERSION = 4
```
