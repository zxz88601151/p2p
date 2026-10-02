# DEPLOYMENT-RECORD.md — v0.9.0-rc4 腾讯云三节点滚动升级记录

| 项 | 值 |
|----|----|
| 部署时间 | 2026-10-02 15:14 ~ 15:18 CST |
| 目标主机 | 腾讯云 `111.229.225.123`（`VM-0-2-ubuntu`） |
| 升级前版本 | `v0.9.0-rc3`，binary SHA256 `5ab7d64a…`（commit `0c5bc3a`） |
| 升级后版本 | `v0.9.0-rc4`，binary SHA256 `0e1b76cf…`（commit `f1e1771`） |
| 升级顺序 | C → B → A（seed 最后） |
| 破坏性变更 | ❌ 无（钱包/存储/P2P 格式均不变，无前置迁移） |

---

## §1 备份与回滚点

| 项 | 路径 |
|----|------|
| 备份目录 | `/opt/p2pchain/backups/pre-rc4-20261002-151443/` |
| 旧二进制（rc3） | `/opt/p2pchain/bin/p2pchain.old-rc3-5ab7d64a`（SHA256 `5ab7d64a…`） |
| 数据快照 | `blocks.dat.{blockchain,blockchain-b,blockchain-c}` |
| 回滚方式 | `cp -a /opt/p2pchain/bin/p2pchain.old-rc3-5ab7d64a /opt/p2pchain/bin/p2pchain && chmod 755 … && 重启三节点` |

---

## §2 滚动升级执行

| 节点 | 角色 | 旧 PID | 新 PID | 结果 |
|------|------|--------|--------|------|
| C | `-seed→A` | 3088015 | 3116033 | ✅ |
| B | `-seed→A` | 3088990 | 3116520 | ✅ |
| A | seed 枢纽 | 3090241 | 3117174 | ✅ |

启动命令保持与升级前**完全一致**（仅二进制替换），每节点追加 `-wallet-password-file /opt/p2pchain/secrets/wallet-password`。

**执行中遇到并修复的问题**：`scp` 上传的新二进制缺少可执行位，`mv` 替换后 `nohup` 报
`Permission denied`（C 节点首次启动失败）。修复：`chmod 755 /opt/p2pchain/bin/p2pchain` 后重启成功。
后续 B/A 复用已在位的 rc4 二进制，无需重复替换。

---

## §3 升级后验证（全 GREEN）

| 检查项 | 结果 |
|--------|------|
| 三进程真实 exe 哈希（`/proc/<pid>/exe`） | 均 = `0e1b76cfc38e069006f9646716a116b0158bffc00068120fde9272c5be0bdc48` ✅ |
| 链高度 | 三节点均 = **137** ✅ |
| Tip 哈希 | 三节点均 = `0000a54c8e4c310f6cd85daf3ddf36785adad7656342bdd73308b97413842702` ✅ |
| Peer 拓扑 | A↔B、A↔C 星型恢复（A peers=2，B/C peers=1）✅ |
| mining 状态 | 三节点 STOPPED ✅ |
| 日志 ERROR/PANIC | 升级后无真实错误（仅历史 SEND_EXIT 的 `error:""` 空字段）✅ |
| 钱包地址 | A `NQ2aAF45CXSZ27LW29ocjXmxMy5KrHqtj4` 未变 ✅ |
| 口令文件 | `1ba75b77…` 未变 ✅ |

---

## §4 P0-6 / P0-8 在本版本中的实际行为

- **P0-8**：`-rpc` 非回环时默认拒绝启动。云上三节点 RPC 均为 `127.0.0.1`，**不触发**该分支，启动行为无变化。
- **P0-6**：仅修正控制面鉴权文案（mutation 6 端点强制 Bearer / 只读 7 端点依赖回环绑定），无功能影响。

即：本次升级对既有运行行为**零影响**，属「使云端二进制与最新 tag 对齐」的治理动作。
