# CLOUD-ROLLING-UPGRADE-MINING-SYNC-GATE-1 — 腾讯云三节点滚动升级记录

| 项 | 值 |
|----|----|
| 部署时间 | 2026-10-02 23:03 ~ 23:09 CST |
| 目标主机 | 腾讯云 `111.229.225.123`（`VM-0-2-ubuntu`），SSH 端口 `2222` |
| 升级前版本 | `v0.9.0-rc4`，binary SHA256 `0e1b76cf…`（commit `f1e1771`） |
| 升级后版本 | `main` 最新，binary SHA256 `460c3c0c…`（commit `cf02dd5`） |
| 内容 | `MINING-SYNC-GATE-1`（自动矿工同步门）+ `ON-DEMAND-MINING-REMOVAL-1`（按需出块全量下线） |
| 升级顺序 | **C → B → A**（seed A 最后） |
| 存储破坏性变更 | ❌ 无（钱包 / 存储 / P2P 格式均不变，数据目录无需迁移） |
| 结果 | ✅ 三节点 exe 哈希 / height / tip 全部一致 |

**构建参数**（沿用仓库冻结配方 `RC-BINARY-MANIFEST.md`）：

```
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -trimpath -o p2pchain ./cmd/node
```

- 可复现性已验证：同树两次构建 → SHA256 完全一致（`460c3c0c…`）。
- `-buildvcs=false` ⇒ 二进制不内嵌 VCS 信息；**因此本次文档提交 `cf02dd5` 与任务简报所述
  `a8df694` 产出的二进制完全相同**（文档不参与编译），不影响部署身份。

---

## §1 备份与回滚点

| 项 | 路径 |
|----|------|
| 备份目录 | `/opt/p2pchain/backups/pre-syncgate-20261002-230358/` |
| 旧二进制（rc4） | `/opt/p2pchain/bin/p2pchain.old-rc4-0e1b76cf`（也在备份目录内） |
| 数据快照（**静默后一致副本**） | `datadir-blockchain{,-b,-c}/`（整目录）+ `blocks.dat.blockchain{,-b,-c}`（扁平，便于比对） |
| 钱包 | `wallet-A.json` / `wallet-B.json` / `wallet-C.json` + `wallet-password` |
| 升级前状态 | `pre-upgrade-status.txt`（三节点 /status 原文） |

**回滚方式**（单台）：

```bash
cp -a /opt/p2pchain/bin/p2pchain.old-rc4-0e1b76cf /opt/p2pchain/bin/p2pchain
chmod 755 /opt/p2pchain/bin/p2pchain
# 优雅停机 → 启动（命令行见 §2）
```

> 因**零存储改动**，正常回滚**只需换回二进制**，无需恢复数据目录；数据目录备份是灾难兜底。

---

## §2 滚动升级执行

| 节点 | 角色 | 旧 PID | 新 PID | 停机时高度 | 结果 |
|------|------|--------|--------|-----------|------|
| C | `-seed→A` | 3116033 | 3239539 | 2379 | ✅ |
| B | `-seed→A` | 3116520 | 3240236 | 2379 | ✅ |
| A | seed 枢纽（**唯一挖矿节点**） | 3117174 | 3240549 | 2380 | ✅ |

**每台执行的步骤**（顺序不可交换）：

1. 优雅停机：`/opt/p2pchain/bin/p2pchain stop -rpc 127.0.0.1:<rpc> -token-file /opt/p2pchain/secrets/control-token`
2. 断言进程退出（`/proc/<pid>` 消失）+ 端口释放（`ss -ltn`）+ `node.lock` 已删除
3. **静默后**对该节点数据目录取一致快照（运行中拷贝可能截断 blocks.dat 尾部）
4. `mv -f` 原子替换二进制（仅 C 需要，见下）
5. 以**与升级前逐字一致**的命令行启动
6. 验证（§3）通过后才做下一台

**二进制替换只做了一次**：`mv p2pchain.new-460c3c0c → p2pchain` 发生在 C 的窗口内。
A/B 重启时在位的已是新二进制（与 rc4 部署记录中的做法一致），无需重复替换。
注意 `mv` 替换路径**不影响**已在运行的进程（它们持有旧 inode），故 A/B 在替换后仍正常服务直至各自重启。

**启动命令行**（与升级前完全一致，仅二进制内容变化）：

```bash
# A（seed，无 -seed，无 -mine）
/opt/p2pchain/bin/p2pchain -listen 127.0.0.1:16688 -rpc 127.0.0.1:16689 \
  -datadir /data/p2pchain/blockchain \
  -auth-token-file /opt/p2pchain/secrets/control-token \
  -wallet-password-file /opt/p2pchain/secrets/wallet-password
# B
... -listen 127.0.0.1:16690 -rpc 127.0.0.1:16691 -datadir /data/p2pchain/blockchain-b ... -seed 127.0.0.1:16688
# C
... -listen 127.0.0.1:16692 -rpc 127.0.0.1:16693 -datadir /data/p2pchain/blockchain-c ... -seed 127.0.0.1:16688
```

启动方式：`cd /opt/p2pchain && setsid nohup <cmd> >> /opt/p2pchain/logs/<log> 2>&1 < /dev/null &`
（`setsid` 使进程完全脱离会话，`PPID=1`，与升级前的游离进程形态一致）

---

## §3 升级后验证（全 GREEN）

| 检查项 | 结果 |
|--------|------|
| 三进程真实 exe 哈希（`/proc/<pid>/exe`） | 唯一值 **1** = `460c3c0c50eb2a6355af594e61da450a322cbe4938d84b326f28a0d14f52c3cb` ✅ |
| 链高度 | 唯一值 **1** = **2381** ✅ |
| Tip 哈希 | 唯一值 **1** = `000000010f0fa80fca55f26b827105f8ca79a8ee31acb3478b8175b40627a1b7` ✅ |
| Peer 拓扑 | A peers=2（B、C），B/C peers=1（A）—— 星型恢复 ✅ |
| 端口 | 16688-16693 全部由新 PID 持有 ✅ |
| 日志 ERROR/PANIC | 三节点均**无**真实错误 ✅ |
| `POST /mine` | **404**（无 token / 有效 token 均 404）✅ |
| `POST /console/mine` | **404** ✅ |
| `POST /mine/start` / `/mine/stop` | 200（生命周期端点存活）✅ |
| A 挖矿状态 | `mining_state=RUNNING`、`mining_reason=pow`、已产出区块（accepted_blocks≥1）✅ |
| B / C 挖矿状态 | `STOPPED` ✅ |
| 同步门日志 | 0 条「等待同步」（未被误触发）✅ |
| 钱包地址 | A `NQ2aAF45CXSZ27LW29ocjXmxMy5KrHqtj4` 未变 ✅ |

---

## §4 与任务简报的偏差（**已与 Owner 确认**）

| # | 简报假设 | 实际情况 | 处置 |
|---|---|---|---|
| D1 | 「systemd 已含 `--wallet-password-file`，保持不变」 | **云上没有任何 systemd 单元**；三节点是 `nohup` 游离进程（`PPID=1`）。`-wallet-password-file` 确实在命令行里，但不由 systemd 提供 | 按 Owner 决策**沿用既有 nohup 方式**，本次不动机制。⚠️ **遗留风险：主机重启后三节点不会自动恢复**（见 §6） |
| D2 | 未提及挖矿 | A 的启动命令**没有** `-mine`，但它在挖矿（`RUNNING`）—— 说明是历史某次 `POST /mine/start` 留下的运行时状态。挖矿状态**不持久化**，重启后必然回到 `STOPPED` | 按 Owner 决策**恢复 A 的挖矿**：A 重启并验证通过后重新 `POST /mine/start`。恢复后 `mining_state=RUNNING`，且**未触发同步门**（B/C 高度不高于 A，不构成更重同步源） |
| D3 | 「当前 HEAD=a8df694」 | 实际 HEAD 已是 `cf02dd5`（Owner 随后要求的文档提交） | 从 **main 最新（`cf02dd5`）** 构建。因 `-buildvcs=false`，二者二进制**完全相同**，不影响部署身份 |
| D4 | 未提及 | 任务执行中一度用 `pgrep -f "<datadir 模式>"` 取 PID，**匹配到了自己的远端 `bash -c`**（自匹配陷阱），导致首次读到的 `/proc/<pid>/exe` 是 bash 的哈希 | 改用 `pgrep -x p2pchain`（精确进程名）后取 PID 再按 cmdline 过滤；已复核三节点 exe 哈希 |

---

## §5 `POST /mine` 调用方审计（简报「注意」项）

| 项 | 结果 |
|---|---|
| 引用已下线的 CLI 子命令 `mine -count` 的脚本 | **24 个**，全部位于 `/root/`，均为历史实验驱动（`deep1/deep2/exp1-6/fork1/r1j_*/r1k_*/r1l_*/txc1-5`），最后修改时间 **2026-10-01** |
| 引用 `/mine` 或 `/console/mine` 端点的文件 | **1 个**：`/opt/p2pchain/e6_rpc.py`（E6 阶段控制面鉴权探针，历史审计脚本） |
| **定时任务 / 调度** | **0** —— root crontab 为空；`/etc/cron.d` 4 条均为系统项（`e2scrub_all`/`sgagenttask`/`sysstat`/`yunjing`）；systemd timer 无 p2p/mine 相关 |
| **正在运行的调用进程** | **0** |
| 引用 `-mine` **启动开关**的文件 | 1 个：`/root/mining-soak-1h.v2.sh` —— 该开关**仍然有效**，无需改动 |

**结论**：**无任何调度或自动化调用方**，不存在静默失败风险。24 个实验脚本与 1 个探针为
**历史时点产物**（与仓库内 `docs/PHASE-*.md` 同类），本次**未改写、未删除**，以保留实验可复现性。
若后续需要重新运行它们，须先把 `mine -count N` 迁移为 `-mine -maxblocks N`（启动期）或
`POST /mine/start` + `/mine/stop`（运行期）。

---

## §6 遗留事项（**本次未处理，需 Owner 决策**）

| # | 事项 | 风险 | 建议 |
|---|---|---|---|
| L1 | **三节点无 systemd 管理** | **主机重启后三节点不会自动恢复**，链停止服务，且无告警 | 单独立项：为三节点建 systemd 单元（`Restart=on-failure`、`WantedBy=multi-user.target`），把 §2 的命令行固化进单元文件 |
| L2 | A 的挖矿是**运行时状态**（非 `-mine` 启动开关） | 任何重启（含本次、含意外崩溃）都会停掉挖矿，需人工重新 `POST /mine/start`；且**无持久化** | 若希望 A 恒定挖矿，应把 `-mine` 写进启动命令行（那样重启即自动挖矿）；但需注意同步门会在落后时自动暂停——这正是本阶段引入的行为 |
| L3 | 24 个历史实验脚本引用已下线的 `mine` 子命令 | 无（未被调度） | 需要时再迁移或归档；本次刻意保留 |
| L4 | 口令文件备份与钱包**同机**（`pre-syncgate-…/wallet-password`） | 单点故障：主机整体失效则备份同时失效 | 异地备份（此前已登记过同类问题） |
