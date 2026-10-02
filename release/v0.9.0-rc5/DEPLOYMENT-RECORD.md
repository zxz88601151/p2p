# DEPLOYMENT-RECORD.md — v0.9.0-rc5 腾讯云三节点滚动升级记录

> **完整记录见根目录**：`CLOUD-ROLLING-UPGRADE-MINING-SYNC-GATE-1.md`
> 本文件是发布目录内的**摘要视图**（与 rc4 目录结构保持一致），
> 不含独立事实；若两处出现不一致，**以根目录完整记录为准**。

---

## 摘要

| 项 | 值 |
|----|----|
| 部署时间 | 2026-10-02 23:03 ~ 23:09 CST |
| 目标主机 | 腾讯云 `111.229.225.123`（`VM-0-2-ubuntu`），SSH 端口 `2222` |
| 升级前版本 | `v0.9.0-rc4`，binary `0e1b76cf…`（tag → `f1e1771`） |
| 升级后版本 | `v0.9.0-rc5`，binary `460c3c0c…`（tag → `ecf9a55`） |
| 升级顺序 | **C → B → A**（seed 最后） |
| 存储破坏性变更 | ❌ 无（钱包 / 存储 / P2P 格式均不变，数据目录无需迁移） |
| 结果 | ✅ 三节点 exe 哈希 / height / tip 全部一致 |

---

## 备份与回滚点

| 项 | 路径 |
|----|------|
| 备份目录 | `/opt/p2pchain/backups/pre-syncgate-20261002-230358/` |
| 旧二进制（rc4） | `/opt/p2pchain/bin/p2pchain.old-rc4-0e1b76cf` |
| 数据快照 | `datadir-blockchain{,-b,-c}/`（整目录，停机后静默副本）+ `blocks.dat.blockchain{,-b,-c}` |
| 钱包与口令 | `wallet-A/B/C.json` + `wallet-password` |
| 回滚方式 | `cp -a /opt/p2pchain/bin/p2pchain.old-rc4-0e1b76cf /opt/p2pchain/bin/p2pchain && chmod 755 … && 重启` |

> 因**零存储改动**，正常回滚**只需换回二进制**，无需恢复数据目录。

---

## 逐台执行

| 节点 | 角色 | 旧 PID | 新 PID | 停机时高度 | 结果 |
|------|------|--------|--------|-----------|------|
| C | `-seed→A` | 3116033 | 3239539 | 2379 | ✅ |
| B | `-seed→A` | 3116520 | 3240236 | 2379 | ✅ |
| A | seed 枢纽（**唯一挖矿节点**） | 3117174 | 3240549 | 2380 | ✅ |

二进制替换只做一次（`mv` 发生在 C 的窗口内），B/A 重启时在位已是新版
（与 rc4 部署记录做法一致）。

---

## 升级后验证（全 GREEN）

| 检查项 | 结果 |
|--------|------|
| 三进程 `/proc/<pid>/exe` 哈希 | 唯一值 **1** = `460c3c0c…` ✅ |
| 链高度 | 唯一值 **1**（升级完成时 2381，随后随挖矿增长）✅ |
| Tip 哈希 | 唯一值 **1** ✅ |
| Peer 拓扑 | A peers=2、B/C peers=1（星型恢复）✅ |
| 日志 ERROR/PANIC | 三节点均无 ✅ |
| `POST /mine` / `/console/mine` | **404**（无 token 与有效 token 均 404）✅ |
| `POST /mine/start` / `/mine/stop` | 200（生命周期端点存活）✅ |
| A 挖矿 | `mining_state=RUNNING`，已恢复并正常出块 ✅ |
| B / C 挖矿 | `STOPPED` ✅ |
| 同步门误触发 | 0 条「等待同步」日志 ✅ |

---

## 与简报的两处偏差（已与 Owner 确认）

| # | 简报假设 | 实际 | 处置 |
|---|---|---|---|
| D1 | 三节点由 systemd 管理 | **云上无任何 systemd 单元**，三节点是 `nohup` 游离进程（`PPID=1`） | 按 Owner 决策沿用既有 nohup 方式；⚠️ **主机重启后不自动恢复**（遗留风险 L1） |
| D2 | 未提及挖矿状态 | A 的启动命令**无** `-mine`，但其挖矿来自历史 `POST /mine/start`（运行时状态，**不持久化**） | 按 Owner 决策在 A 重启后重新 `POST /mine/start` 恢复挖矿 |

---

## `POST /mine` 调用方审计

| 项 | 结果 |
|----|------|
| 引用已下线 `mine -count` 的历史实验脚本 | 24 个（`/root/*.sh`，最后修改 2026-10-01） |
| 引用 `/console/mine` 的历史探针 | 1 个（`/opt/p2pchain/e6_rpc.py`） |
| **定时任务 / 调度** | **0**（root crontab 空；`/etc/cron.d` 4 条均为系统项；无 p2p/mine 的 systemd timer） |
| **运行中调用** | **0** |

**结论**：无静默失败风险。历史产物**刻意保留未改写**（与 `docs/PHASE-*.md` 同类，
保留实验可复现性）；重跑前须迁移到 `-mine -maxblocks N` 或 `POST /mine/start`。
