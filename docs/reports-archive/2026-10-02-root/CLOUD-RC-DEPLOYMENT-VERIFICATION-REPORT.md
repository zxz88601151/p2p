# PHASE P2PCHAIN — RC CLOUD DEPLOYMENT VERIFICATION REPORT
## 腾讯云节点 · v0.9.0-rc1 身份一致性只读验证

- **阶段**：`PHASE-P2PCHAIN-RC-CLOUD-DEPLOYMENT-VERIFICATION-1`
- **目标**：验证腾讯云已部署节点是否与公开发布的 `v0.9.0-rc1` 身份一致
- **性质**：纯只读验证（audit / verification）。**未升级、未替换 binary、未改配置、未删数据、未重启服务**。
- **验证时间**：`2026-10-02 10:50:54 ~ 10:53:00 CST`
- **发布基准**：Version `v0.9.0-rc1` / Git tag `v0.9.0-rc1` / Expected SHA256 `71097357531876b9975998d1d316f0888543a4aea1dcb9456c160df50e6352b1`

---

## 1. Server Identity（服务器身份）

| 项 | 值 |
|----|----|
| Hostname | `VM-0-2-ubuntu` |
| OS | Ubuntu 24.04.4 LTS (Noble Numbat) |
| Kernel | `6.8.0-117-generic` |
| Architecture | x86_64 |
| 公网 IP | `111.229.225.123`（腾讯云 CVM） |
| 当前时间 | 2026-10-02 10:51:07 CST |
| Uptime | 112 days 16:28，load average 0.00/0.00/0.00 |
| SSH | 端口 2222，ed25519 免密，UFW 白名单 |

---

## 2. Running Binary Information（运行二进制信息）

| 项 | 值 |
|----|----|
| Executable path | `/opt/p2pchain/bin/p2pchain` |
| 文件类型 | ELF 64-bit LSB x86-64，statically linked，Go BuildID `oRW7i75j...`，not stripped |
| 文件大小 | 11,921,942 bytes |
| Modification time | 2026-10-01 17:54:49 +0800 |
| VCS revision（内嵌） | `c3cec3f3594f7e11ac135063043f4c8f7c056166` |
| VCS time（内嵌） | 2026-10-01T09:17:34Z |
| VCS modified（内嵌） | `true`（脏工作树构建） |
| 构建模式 | **`-buildvcs=true`（默认）** —— 旧式构建，非 RC 确定性构建 |

**运行进程实例（3 个，共用同一二进制）：**

| 节点 | PID | P2P 监听 | RPC | DataDir | 角色 |
|------|-----|----------|-----|---------|------|
| A（seed 枢纽） | 2872772 | 127.0.0.1:16688 | 127.0.0.1:16689 | /data/p2pchain/blockchain | seed hub |
| B | 2873329 | 127.0.0.1:16690 | 127.0.0.1:16691 | /data/p2pchain/blockchain-b | -seed → A |
| C | 2872762 | 127.0.0.1:16692 | 127.0.0.1:16693 | /data/p2pchain/blockchain-c | -seed → A |

> 确认：当前节点**正在运行**（3 个实例，Ssl 态，均正常监听/响应）。存在 3 个 node 实例（同一 binary，不同 datadir/端口），属预期的 3 节点 loopback 集群，非异常多开。

---

## 3. SHA256 Verification Result（哈希验证结果）

```
实际运行 binary SHA256:
a95df7c9a960fd1896ff4d5b1e2504ea0778fcba87961f6423f8a9e970c5f217

期望 v0.9.0-rc1 SHA256:
71097357531876b9975998d1d316f0888543a4aea1dcb9456c160df50e6352b1

结果: FAIL（不一致）
```

**差异分析（根因已定位，非随机漂移）：**

| 维度 | 云上运行 binary | 公开发布 v0.9.0-rc1 |
|------|----------------|---------------------|
| 构建提交 | `c3cec3f3...`（云上独立维护源） | `2647ba0...`（本地发布源） |
| 构建参数 | `-buildvcs=true`（内嵌 VCS 元数据） | `-buildvcs=false -trimpath` |
| 工作树 | modified=true（脏） | 干净（确定性构建） |
| 文件名 | `p2pchain`（Linux 部署命名） | `node-v0.9.0-rc1.exe`（Windows 发布物） |
| SHA256 | `a95df7c9...` | `71097357...` |

**结论**：云上运行的是**另一条独立维护的源码线**（commit `c3cec3f3`，`vcs.modified=true` 的旧式构建），与本机公开发布的 `v0.9.0-rc1`（`2647ba0`，`-buildvcs=false` 确定性构建）**非同源、非同一二进制**。云上 `/opt/p2pchain/` 内**不存在任何 v0.9.0-rc1 二进制**，也不存在 git 仓库（纯部署目录）。

**按 §1 指令：哈希不一致，已立即停止后续修复动作，仅记录差异，未执行任何状态变更。**

---

## 4. Runtime Configuration（运行时配置）

三个节点完整启动参数（`ps` 实测）：

```
A: /opt/p2pchain/bin/p2pchain -listen 127.0.0.1:16688 -rpc 127.0.0.1:16689
     -datadir /data/p2pchain/blockchain -auth-token-file /opt/p2pchain/secrets/control-token
B: /opt/p2pchain/bin/p2pchain -listen 127.0.0.1:16690 -rpc 127.0.0.1:16691
     -datadir /data/p2pchain/blockchain-b -auth-token-file /opt/p2pchain/secrets/control-token -seed 127.0.0.1:16688
C: /opt/p2pchain/bin/p2pchain -listen 127.0.0.1:16692 -rpc 127.0.0.1:16693
     -datadir /data/p2pchain/blockchain-c -auth-token-file /opt/p2pchain/secrets/control-token -seed 127.0.0.1:16688
```

| 配置项 | 值 | 说明 |
|--------|----|------|
| chain id | 无显式 `-chain` 参数 | 使用默认/创世内建 chain id |
| genesis | 无显式 genesis 文件参数 | 内建创世块（见 §5 创世哈希） |
| network 参数 | `-listen 127.0.0.1:<port>` | 全部 loopback，外部不可达 |
| RPC | `-rpc 127.0.0.1:<port>` | 全部 loopback + Bearer token 鉴权 |
| data dir | `/data/p2pchain/blockchain[-b/-c]` | 三个独立 datadir |
| auth-token | `/opt/p2pchain/secrets/control-token`（64 bytes，0600） | 权限正确 |
| 守护 | 无 systemd 单元，进程 PPID=1 | 崩溃不自动重启（历史已知项 F4） |

> 运行时配置与上一轮健康检查报告一致，未见偏离；但需注意：**无版本/chain id 显式参数**，节点身份完全由二进制内建决定——而该二进制非 RC 二进制。

---

## 5. Chain State（链状态）

| 节点 | height | tip_hash | mining | bits/difficulty | peers |
|------|--------|----------|--------|-----------------|-------|
| A | 136 | `0000e923a6f536b23dc8dc1ebe1e5d4b27d5fce657d7d7507e4d63fed09ed544` | STOPPED (init) | 16 / 1 | 2（B、C） |
| B | 136 | 同上 | STOPPED (init) | 16 / 1 | 1（A） |
| C | 136 | 同上 | STOPPED (init) | 16 / 1 | 1（A） |

- 三节点 **height=136、tip_hash 完全一致** → 处于稳定共识态。
- 创世块（height=0）哈希 `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3`，与历史审计一致，无漂移。
- 余额（A）：spendable=172 / total=177 / utxo_count=34，状态自洽。
- 全部 `mining=STOPPED / reason=init`，`accepted/rejected=0` → 静态链，未挖矿（符合试点验证预期）。

---

## 6. Health Status（健康状态）

| 检查项 | 结果 |
|--------|------|
| 进程存活 | ✅ 3 节点全部 Ssl 态，PPID=1 |
| 端口监听 | ✅ 6 端口（16688–16693）全部 LISTEN |
| RPC 响应 | ✅ 三节点 `/status` 均正常返回 |
| P2P 网格 | ✅ 星型 A↔B、A↔C，握手连续无错 |
| 共识一致性 | ✅ 三节点 height/tip 完全一致 |
| crash loop | ❌ 无 |
| database error | ❌ 无 |
| consensus error | ❌ 无（无 reorg/orphan/分裂记录） |
| 数据完整性 | ✅ blocks.dat + node.lock + wallet.json 齐全，无损坏迹象 |

日志健康：`/opt/p2pchain/logs/*.log` 中 ERROR 字段均为空字符串（`"error":""`），事件为正常 `handshake`/`get_blocks`/`blocks_resp` 等，无 panic/fatal/reorg/consensus 异常。

---

## 7. Final Decision（最终判定）

### 🔴 **RED — 发现运行版本异常**

**判定依据**：

1. **运行 binary 与 v0.9.0-rc1 身份不一致（核心）**：
   - 实际 SHA256 `a95df7c9...` ≠ 期望 `71097357...`。
   - 实际为 `-buildvcs=true` 旧构建，内嵌 `vcs.revision=c3cec3f3`、`vcs.modified=true`；RC 发布为 `-buildvcs=false` 确定性构建，源 `2647ba0`。
   - 云上不存在任何 v0.9.0-rc1 二进制，无 git 仓库可追溯。

2. **节点本身运行正常**：3 节点共识一致、P2P 网格健康、无 crash/error/consensus 异常、数据目录完整。

> 即：**「节点运行健康」但「版本身份与 RC 不一致」**。按决策矩阵，命中「运行版本异常」→ RED（尽管数据与运行态本身无风险，身份差异属版本异常范畴，须如实标注）。

---

## HARD STOP

本阶段为**纯只读验证**，已完成并停止。**未执行、也将不执行**（未经 Owner 单独授权）：

- ❌ binary 替换
- ❌ restart
- ❌ upgrade
- ❌ config 修改
- ❌ 数据修复
- ❌ rollback

**待 Owner 决策的后续选项（需单独授权）：**

| 选项 | 动作 | 性质 |
|------|------|------|
| 1 | 将 `node-v0.9.0-rc1`（Linux 交叉编译产物）部署到云并替换 `/opt/p2pchain/bin/p2pchain` + 重启 3 节点 | 升级（breaking，需快照回滚预案） |
| 2 | 维持现状，接受云上为「独立预研环境」，与公开发布 RC 明确隔离 | 无状态变更 |
| 3 | 在云上重新构建 RC 对应源码（需先将 `2647ba0` 源码同步到云）再替换 | 升级 + 源码同步 |

> 说明：云上运行源码线 `c3cec3f3` 与本机发布源 `2647ba0` 的关系（是否为同一仓库不同分支、是否已合并 RC 改动）需 Owner 进一步澄清后方可判断选项 1/3 的正确路径。
