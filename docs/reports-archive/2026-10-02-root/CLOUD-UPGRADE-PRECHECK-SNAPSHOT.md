# PHASE P2PCHAIN — RC CLOUD UPGRADE PRECHECK SNAPSHOT
## 云节点升级前基线快照（只读）

- **阶段**：`PHASE-P2PCHAIN-RC-CLOUD-UPGRADE-EXECUTION-READINESS-1` §0
- **性质**：只读快照，未做任何状态变更。
- **时间**：`2026-10-02 11:00 CST`

---

## 三节点运行实例

| 节点 | PID | 角色 | P2P 监听 | RPC | DataDir |
|------|-----|------|----------|-----|---------|
| A | 2872772 | seed 枢纽 | 127.0.0.1:16688 | 127.0.0.1:16689 | /data/p2pchain/blockchain |
| B | 2873329 | -seed → A | 127.0.0.1:16690 | 127.0.0.1:16691 | /data/p2pchain/blockchain-b |
| C | 2872762 | -seed → A | 127.0.0.1:16692 | 127.0.0.1:16693 | /data/p2pchain/blockchain-c |

## 当前 binary

| 项 | 值 |
|----|----|
| 路径 | `/opt/p2pchain/bin/p2pchain` |
| SHA256 | `a95df7c9a960fd1896ff4d5b1e2504ea0778fcba87961f6423f8a9e970c5f217` |
| vcs.revision | `c3cec3f3594f7e11ac135063043f4c8f7c056166` |
| vcs.modified | `true` |
| 大小 | 11,921,942 bytes |

## 链状态（三节点一致）

| 项 | 值 |
|----|----|
| height | 136 |
| tip hash | `0000e923a6f536b23dc8dc1ebe1e5d4b27d5fce657d7d7507e4d63fed09ed544` |
| peers | A↔B、A↔C（星型，B/C 各连 A） |
| mining | STOPPED (init) |

## DataDir 状态

| DataDir | 大小 | blocks.dat | wallet.json |
|---------|------|-----------|-------------|
| blockchain (A) | 144K | `9035fb10...6227d0` | `02f19c1d...1e7c32` |
| blockchain-b (B) | 100K | `ea733ece...ca9c97e` | `c36a44b3...a9a4087` |
| blockchain-c (C) | 56K | `a1c606c2...7e4d6ee` | `da734236...2fb9701` |

## 磁盘空间

| 挂载点 | 大小 | 已用 | 可用 | 使用率 |
|--------|------|------|------|--------|
| / (含 /opt /data) | 40G | 8.5G | 30G | 23% |

> 磁盘空间充足，备份 3 个 datadir（合计 ~300K）+ binary 无压力。

## 服务启动方式

**manual / nohup（无 systemd 单元）**

- 三进程 PPID=1（init），典型 nohup 后台启动特征。
- `systemctl list-units` 无 p2p/node 相关 service；`/etc/systemd/system/` 无 p2p unit。
- **影响**：升级/回滚重启需手动 `nohup <binary> <args> >> <log> 2>&1 &` 恢复；崩溃不自动拉起（历史已知项 F4）。

## 日志重定向

| 节点 | 日志文件 |
|------|---------|
| A | `/opt/p2pchain/logs/node.log` |
| B | `/opt/p2pchain/logs/node-b.log` |
| C | `/opt/p2pchain/logs/node-c.log` |
