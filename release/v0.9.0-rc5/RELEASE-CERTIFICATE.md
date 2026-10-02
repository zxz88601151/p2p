# RELEASE-CERTIFICATE.md — Release Candidate 发布证书（v0.9.0-rc5）

> 本证书为 P2PChain `v0.9.0-rc5` Release Candidate 的**不可变版本身份证明**，
> 记录 tag → commit → source → binary 的完整 provenance 链路。
> **Provenance 模型**：external attestation（二进制不内嵌 VCS，溯源由本证书外部记录）。

---

## 版本身份（Immutable Version Identity）

| 项 | 值 |
|----|----|
| 版本标签（annotated tag） | `v0.9.0-rc5` |
| 定位 | Developer Node Release Candidate（**非生产加密货币发布**） |
| 发布状态 | ✅ 自动矿工同步门 + 按需出块下线完成，已打 annotated tag |
| 打标日期 | 2026-10-02 |
| 前驱版本 | `v0.9.0-rc4`（tag → `f1e1771`，binary `0e1b76cf…`） |
| **破坏性变更** | ⚠️ **有**（本版本含 3 项 breaking change，详见「Release Notes」与「兼容性说明」） |

---

## Release Notes（含 BREAKING CHANGE）

### ⚠️ BREAKING CHANGE 1 — 按需出块（on-demand mining）**全量下线**

以下全部移除，**调用一律返回 `404`**（端点/入口在物理上不存在，路由层判定，
与请求头、是否携带有效 Bearer Token **无关** —— 这是比 `401` 更强的边界）：

| 层面 | 下线内容 |
|------|----------|
| HTTP 路由 | `POST /mine`、`POST /console/mine` |
| 服务层实现 | `nodeService.Mine(count)`、`Node` 接口方法、`MineRequest` / `MineResponse`、`MaxMineCount` |
| 客户端方法 | `control.Client.Mine(count)` |
| CLI | `node mine` 子命令（`cliCommands` 表项 + `cmdMine` + usage 条目 + 选项块） |
| 同源闸门 | `consoleOriginGate` / `isSameOriginRequest`（随其唯一使用者下线而退役） |
| 界面 | Developer Console 出块按钮与脚本；GUI（P2PChain Studio）「按需出块」卡片 |

**迁移路径**：出块只剩两条入口 —— **启动期 `-mine`**（可用 `-maxblocks N` 精确限制产出块数），
或运行期 **`POST /mine/start` / `POST /mine/stop`**（持续挖矿生命周期，Bearer 保护）。

> 注意：`POST /mine/start` 的 `maxBlocks` 为 0（不限），**无法「精确挖 N 块」**。
> 若脚本原先依赖 `mine -count N` 的确定性，须改用 `-mine -maxblocks N` 启动，
> 或改为「`/mine/start` → 轮询到目标条件 → `/mine/stop`」（见 `scripts/smoke-e2e.sh` 的现成实现）。

### ⚠️ BREAKING CHANGE 2 — 控制面契约变更：mutation 6→4，端点 13→11，不再产生 `403`

| 项 | rc4 | rc5 |
|----|-----|-----|
| 端点总数 | 13 | **11**（移除 `/mine`、`/console/mine`） |
| mutation 端点 | 6（`/send` `/mine` `/mine/start` `/mine/stop` `/console/mine` `/stop`） | **4**（`/send` `/mine/start` `/mine/stop` `/stop`） |
| 只读端点 | 7（无鉴权，安全边界为回环绑定） | 7（不变） |
| `403` 状态码 | 由 `/console/mine` 的非同源请求产生 | **控制面不再产生 403**（同源闸门已退役） |

`404` 语义扩展：控制台路径不匹配，**以及**已下线的 `/mine` / `/console/mine`。
规范同步见 `docs/CANONICAL-RPC-SPEC.md`（端点表、§2.2 同源闸门已退役、§6 错误码）。

### ✨ 新增能力 — 自动矿工「同步门」（MINING-SYNC-GATE-1）

- **行为**：本地**明显落后**于网络时，**自动暂停挖矿**；追上后**自动恢复**，无需人工干预。
- **新语义状态**：`mining_state = WAITING_SYNC`（`/status` 可见），CLI 渲染为
  「等待同步（本地落后于网络，暂停自动挖矿）」，Console 与 GUI 同样呈现。
- **日志**：`[miner] 等待同步：本地高度=%d，对端最高=%d，暂停自动挖矿`
  （进入等待时 1 条，之后每 12 次复查 1 条，节流不刷屏）。
- **判定**：容忍 5 块落差（抵消新块传播抖动）；TTL 10 分钟（对端条目靠时间自愈，
  无断开回调）；**只统计「更重」的对端**（复用既有 work-priority 逻辑，
  低难度长链不会误挡）；**无有效对端时放行**（孤立节点/创世必须能出块）。
- **未就绪时**：不组装候选区块、不执行任何 PoW；等待为 stop-aware
  （`POST /mine/stop` 与节点停机可被立即响应）。
- **不变量**：这是**纯策略层**闸门 —— 不动共识真值（高度/难度/出块规则零改动）、
  不动存储字节格式、不动 P2P 协议面（不新增消息类型、不改握手协议）。
- **已知局限**：对端**更重但同步持续失败**时会持续压制挖矿；单个对端
  **谎报极高高度**即可压制挖矿（握手自报值未经验证）。二者均为 **v1 有意接受**，
  详见 `MINING-SYNC-GATE-1-DESIGN.md` §6 与 `MINING-SYNC-GATE-1-REPORT.md` §6。

---

## 本版本变更（相对 v0.9.0-rc4）

| 提交 | 变更项 | 类别 |
|------|--------|------|
| `f6fb1b2` | chore(release): add v0.9.0-rc4 release certificate and checksums | 发布治理 |
| `689502f` | chore(docs): archive 80 root-level untracked reports | 发布治理 |
| `4676217` | docs(release): add v0.9.0-rc4 cloud rolling upgrade record | 发布治理 |
| `9a5c582` | **feat(mining)**: 自动矿工同步门 + 按需出块全量下线（**业务源码主提交**，19 个 .go） | 功能 |
| `a8df694` | **fix(mining)**: 清扫下线残留引用（含退化为空断言的负向测试改写）+ 新增验收复核脚本（7 个 .go） | 修复/治理 |
| `cf02dd5` | docs(mining): 登记「握手高度谎报」攻击向量为同步门 v1 已知局限 | 文档 |
| `ecf9a55` | docs(cloud): 记录 rc4 → main 三节点滚动升级（**tag target**） | 文档 |

**业务源码边界**：`a8df694`（rc5 中**最后一个改动 .go 的提交**；其后的 `cf02dd5`、`ecf9a55`
均为纯文档提交，不参与编译）。

---

## Provenance 链路（tag → commit → source → binary）

### 1. tag → commit

| 项 | 值 |
|----|----|
| annotated tag | `v0.9.0-rc5`（`git cat-file -t` = `tag`） |
| 指向 commit | `ecf9a55e321ad2ea15c4b17edff6b89b33aed29e` |
| tag message | 自动矿工同步门 + 按需出块全量下线（详见 tag 注解） |
| 业务源码边界 | `a8df69462bfb56e39ab64fec036a488f5954abd6` |

> 与 rc4 一致：**tag 指向代码提交**，本证书与校验和在该 tag **之后**提交。
> 这正是 external attestation 模型的预期形态 —— 二进制身份由**源码内容**决定，
> 溯源由本证书在**外部**记录，不要求 tag 树内包含证书本身。

### 2. commit → source（相对 rc4 的增量）

| 项 | 值 |
|----|----|
| rc4 基线（tag target） | `f1e17712c57d7f8dd5f860b018da4a5fc67ae569` |
| rc5 tag target | `ecf9a55e321ad2ea15c4b17edff6b89b33aed29e` |

**rc5 增量提交链（main 分支，7 提交）**：

| 顺序 | SHA | 说明 | .go 文件数 |
|------|-----|------|-----------|
| 1 | `f6fb1b2` | rc4 发布证书与校验和 | 0 |
| 2 | `689502f` | 归档 80 个根目录未跟踪报告 | 0 |
| 3 | `4676217` | rc4 云上滚动升级记录 | 0 |
| 4 | `9a5c582` | **同步门 + 按需出块下线**（业务源码主提交） | 19 |
| 5 | `a8df694` | 下线残留清扫 + 验收复核脚本 | 7 |
| 6 | `cf02dd5` | 登记握手高度谎报攻击向量 | 0 |
| 7 | `ecf9a55` | rc5 云上滚动升级记录（tag target） | 0 |

### 3. source → binary（External Attestation）

| 项 | 值 |
|----|----|
| 官方二进制 | `p2pchain-linux-amd64-v0.9.0-rc5`（Linux 部署产物） |
| 大小 | 12,038,561 字节 |
| SHA-256 | `460c3c0c50eb2a6355af594e61da450a322cbe4938d84b326f28a0d14f52c3cb` |
| VCS 内嵌 | ❌ 无（`-buildvcs=false`） |
| 构建参数 | `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -trimpath -o p2pchain-linux-amd64-v0.9.0-rc5 ./cmd/node` |
| Go 版本 | `go1.27.0`（linux/amd64 交叉编译，宿主 windows/amd64） |
| 链接形态 | 静态（`statically linked`，与 rc4 一致） |
| Provenance 模型 | **external attestation**（身份由源码内容决定，溯源由本证书外部记录） |
| 确定性复验 | ✅ 两次独立构建 SHA256 完全一致 `460c3c0c…` |

**二进制特征串核验**（与 rc4 对照，证明功能确实变更）：

| 特征串 | rc4 | rc5 | 含义 |
|--------|-----|-----|------|
| `handleMineStart` / `handleMineStop` | 有 | 有 | 持续挖矿生命周期保留 |
| 裸 `handleMine`（按需出块 handler） | **有** | **0** | ✅ 按需出块已移除 |
| `MaxMineCount` | 有 | **0** | ✅ 随 `/mine` 一并移除 |
| `WAITING_SYNC` | **0** | 3 | ✅ 新增同步门状态 |
| `等待同步：本地高度` | **0** | 1 | ✅ 新增同步门日志 |
| `behind-network` | **0** | 1 | ✅ 新增同步门原因字段 |

**与已部署产物一致**：该 SHA256 与腾讯云三节点当前运行的二进制
（`/opt/p2pchain/bin/p2pchain`，`/proc/<pid>/exe` 核验）**完全相同**。

---

## 兼容性说明（部署前必读）

| 面 | 是否破坏 | 说明 |
|----|---------|------|
| **控制面** | ✅ **破坏** | `POST /mine`、`POST /console/mine` 恒 404；mutation 6→4，端点 13→11；不再产生 403。**任何调用方须先迁移**（云上审计结果：零调度调用方，见部署记录 §5） |
| **CLI** | ✅ **破坏** | `node mine` 子命令已移除（不再被识别，退出码 2）。改用 `-mine -maxblocks N` |
| **存储** | ❌ 无 | `blocks.dat` / v2 帧格式不变，**无需迁移** |
| **钱包** | ❌ 无 | 沿用 rc3 的 v2 加密信封与 `--wallet-password-file` 机制，**无需迁移** |
| **P2P** | ❌ 无 | 协议面不变（同步门只**读**既有握手字段，不新增消息类型） |
| **共识** | ❌ 无 | 高度 / 难度 / 出块规则零改动 |

**部署影响评估**：

- 升级 = 原子替换二进制 + 优雅停机/重启，**无前置迁移步骤**。
- **挖矿状态不持久化**：若节点靠运行期 `POST /mine/start` 挖矿（而非启动期 `-mine`），
  重启后必然回到 `STOPPED`，**需人工重新 `POST /mine/start`**（本次云上 A 节点即此情形）。
- **同步门会改变既有行为**：此前「新节点带 `-mine` 启动即自铸区块」不再成立 ——
  落后于网络时将**暂停挖矿**直到追平。这是**本版本的核心意图**，但运维需知晓
  「为什么不出块」的答案是 `WAITING_SYNC` 而非故障。

---

## 不可变性声明（Immutability Declaration）

1. **源码冻结**：rc5 业务源码边界固定于 `a8df694`，tag 目标为 `ecf9a55`。
2. **二进制冻结**：官方 Linux 二进制 `p2pchain-linux-amd64-v0.9.0-rc5` 的 SHA-256
   固定为 `460c3c0c50eb2a6355af594e61da450a322cbe4938d84b326f28a0d14f52c3cb`。
3. **确定性可复现**：冻结参数（`ecf9a55` checkout + `-buildvcs=false -trimpath` +
   Go1.27.0 + `GOOS=linux GOARCH=amd64 CGO_ENABLED=0`）下，两次独立构建 SHA256 完全一致。
4. **tag 不可变**：annotated tag `v0.9.0-rc5` 已创建并指向 `ecf9a55`；
   **旧 tag（rc1~rc4）未做任何改动**。

---

## 排除与限制（Exclusions & Limitations）

- **非生产发布**：本 RC 定位为 Developer Node 学习/实验候选，未做安全审计，不得用于真实资产场景。
- **source-only release**：不含链数据 / 钱包 / 凭据 / 孤儿检查点（均被 gitignore 排除）。
- **回归测试**：`scripts/run-tests.sh`（`-count=1 -timeout 25m`）→ **15/15 包 ok、0 FAIL**
  （`cmd/node` 775.743s）；`scripts/verify-mining-sync-gate.sh` → **25/25 PASS**；
  `scripts/smoke-e2e.sh`（双真实进程）→ **20/20**；`go build ./...` / `go vet ./...` 干净。
- **已知局限（v1 接受）**：
  1. 对端更重但同步持续失败 ⇒ 同步门持续压制挖矿；
  2. 单个对端谎报极高握手高度 ⇒ 可压制挖矿（TTL 10m，重连即刷新）。
     加固方向（多 peer 佐证 / 纳入 P0-5 失信记分）**单独立项，本版本未实现**。
- **未覆盖验证**：GUI 运行时测试（`gui/_fulltest.py`、`_smoke.py`）因环境缺 PySide6 未执行，
  GUI 侧仅静态验证（语法 + 引用审计）。`/gui/` 为治理禁止跟踪目录，GUI 改动不进 Git。
- **部署状态**：源码与 tag 已 push 至 GitHub `origin/main` 与 `v0.9.0-rc5`；
  腾讯云三节点滚动升级（C→B→A）已完成并核验一致，见 `DEPLOYMENT-RECORD.md` 及
  根目录 `CLOUD-ROLLING-UPGRADE-MINING-SYNC-GATE-1.md`。

---

## 签发（Issuance）

- 签发人：Owner（经 p2pchain-baseline 执行）
- 签发时间：2026-10-02
- 生效范围：本地仓库 `main` 分支 + GitHub `origin/main` + tag `v0.9.0-rc5`

**本证书为 v0.9.0-rc5 的正式不可变身份凭证，随 tag 一并封存。**
