# PHASE P2PCHAIN — CLOUD BINARY PROVENANCE TRACE
## 腾讯云运行 binary 来源追踪报告（只读）

- **阶段**：`PHASE-P2PCHAIN-CLOUD-SOURCE-LINE-TRACE-1`
- **目标**：只读追踪云上运行 binary 的来源，确认 `c3cec3f3` 与 `v0.9.0-rc1`（`2647ba0`）的关系
- **性质**：纯只读追踪。**未停止节点、未替换 binary、未改配置、未改数据、未执行部署**。
- **追踪时间**：`2026-10-02 10:54 ~ 10:58 CST`

---

## 最终结论（先说结果）

### 🟡 **YELLOW — 云上 binary 与 v0.9.0-rc1 同源，但落后 9 个提交、且构建方式不同**

**核心事实（已用多重证据确认）：**

1. **`c3cec3f3` 是本机 git 历史中的一个真实 commit，且是 `2647ba0`（v0.9.0-rc1）的直接祖先。** 两者**同源同仓库同分支**，不存在"独立维护的另一条源码线"。
2. 云上 binary 的代码快照**精确对应 `c3cec3f3`**：含该 commit 引入的 O1 genesis identity guard 代码，不含其后的 `52fb464`（console auth）与 `36f9630`（orphan checkpoint）代码。
3. 云上 binary 落后 RC 发布点 **9 个提交**，其中包含 **2 个功能代码提交**（`52fb464` console mining auth、`36f9630` orphan durability），其余 7 个为纯文档/治理提交。
4. 构建方式差异：云上为 `-trimpath=true -buildvcs=true`（默认，内嵌 VCS 元数据 + `+dirty`），RC 为 `-buildvcs=false -trimpath`（确定性，外部证明）。两者都用了 trimpath，差异仅在 buildvcs。

> **修正上一阶段（VERIFICATION-1）的结论**：上一阶段因未查本机 git 历史，误判云上为"独立源码线"。本阶段确认二者**同源**——云上就是 RC 之前某个中间状态的部署，落后但可追溯。

---

## §0 云部署目录来源审计

| 目录 | 内容 | 结论 |
|------|------|------|
| `/opt/p2pchain/` | bin/backups/logs/secrets + e3~e6 实验脚本（*.py） | 纯部署目录，**无源码、无 go.mod、无 .git** |
| `/opt/p2pchain/bin/` | `p2pchain`（运行）+ `p2pchain.baseline-3e732888`、`p2pchain.baseline-b21c314a`、`p2pchain-o1`、`p2pchain-explorer` | 多代历史构建副本 |
| `/opt/p2pchain/backups/` | etc.tar.gz / iptables / ufw / ssh-root-keys 快照（Oct 1 14:12） | 系统配置备份，与 binary 来源无关 |
| `/opt/p2pchain/logs/` | node.log / node-b*.log / node-c.log | 运行日志，无构建记录 |
| `/root/.p2pchain/` | 空目录 | 无残留 |
| 全盘 `p2pchain*` 目录 | 仅 `/opt/p2pchain`、`/data/p2pchain` | 云上**无源码残留** |

**结论**：云上 binary 是从**本机交叉编译后上传**的（`GOOS=linux`），云上不保留构建上下文。

---

## §1 Git Provenance Tracing（Git 来源追踪）

### 三个 commit 的存在性（本机 git）

| commit | 本机存在？ | 与 2647ba0 关系 | 性质 |
|--------|-----------|----------------|------|
| `c3cec3f3` | ✅ 存在 | **2647ba0 的祖先**（领先 9 提交） | `fix(p2p): reject handshake on genesis hash mismatch (O1 genesis identity guard)` |
| `3e6d44a` | ✅ 存在 | 2647ba0 的祖先（领先 4 提交） | `docs: synchronize release governance status metadata` |
| `2647ba0` | ✅ 存在 | = HEAD = v0.9.0-rc1 tag | RC 发布点 |

### 完整提交链（c3cec3f3 → 2647ba0，共 9 提交）

| # | commit | 类型 | 时间（+0800） | 说明 |
|---|--------|------|--------------|------|
| 0 | `c3cec3f3` | **代码** | 10-01 17:17:34 | O1 genesis identity guard（+22 行 node.go） |
| 1 | `52fb464` | **代码** | 10-01 18:46:22 | console mining auth（server.go 等 3 文件） |
| 2 | `36f9630` | **代码** | 10-02 09:30:30 | orphan durability §4-B1/B1.5/B2（9 文件 +1798 行） |
| 3 | `8410036` | 文档 | 10-02 09:30:44 | 文档真相同步 |
| 4 | `3e6d44a` | 文档 | 10-02 09:52:59 | 发布治理状态元数据同步 |
| 5 | `83914d1` | 文档 | — | RC 证据闭环产物 |
| 6 | `bbbfab7` | 文档 | — | RC 发布证书封存 |
| 7 | `2fe8582` | 文档 | — | RC binary 来源更新 |
| 8 | `a69d24e` | 文档 | — | RC binary 来源对齐定稿 |
| 9 | `2647ba0` | 文档 | 10-02 10:30:55 | 确定性构建模型采纳（external attestation） |

**关键**：`c3cec3f3` 之后有 **3 个代码提交**（`52fb464`、`36f9630`，以及 `c3cec3f3` 本身是代码）。云上 binary 停留在 `c3cec3f3`，**缺少 `52fb464`（console auth）和 `36f9630`（orphan durability）两个功能提交**。

---

## §2 Binary Metadata Audit（二进制元数据审计）

云上运行 binary `/opt/p2pchain/bin/p2pchain` 的完整 `go version -m` 等价元数据（经 strings 提取）：

```
path            p2pchain/cmd/node
mod             p2pchain  v0.0.0-20261001091734-c3cec3f3594f+dirty
build           -buildmode=exe
build           -compiler=gc
build           -trimpath=true
build           CGO_ENABLED=0
build           GOARCH=amd64
build           GOOS=linux
build           GOAMD64=v1
build           vcs=git
build           vcs.revision=c3cec3f3594f7e11ac135063043f4c8f7c056166
build           vcs.time=2026-10-01T09:17:34Z
build           vcs.modified=true
go              go1.27.0
```

| 字段 | 值 | 解读 |
|------|----|------|
| module version | `v0.0.0-20261001091734-c3cec3f3594f+dirty` | 伪版本时间戳 `20261001091734` = commit `c3cec3f3` 提交时刻（UTC 09:17:34） |
| vcs.revision | `c3cec3f3594f...` | 构建基线 = c3cec3f3 |
| vcs.modified | `true` | 构建时工作树有未提交修改（+dirty） |
| buildvcs | **true**（内嵌了 VCS 元数据） | 默认构建，非 RC 的 `-buildvcs=false` |
| trimpath | **true** | 已用 trimpath |
| GOOS/GOARCH | linux/amd64 | Linux 交叉编译 |
| go 版本 | go1.27.0 | 与本机一致 |

---

## §3 Deployment Timeline Reconstruction（部署时间线还原）

| 时间（+0800） | 事件 |
|--------------|------|
| 10-01 17:17:34 | 本机提交 `c3cec3f3`（O1 genesis identity guard） |
| 10-01 17:54:49 | **云上 binary `p2pchain` 生成**（mtime，此时工作树含 c3cec3f3 + 少量未提交修改 → `+dirty`） |
| 10-01 18:46:22 | 本机提交 `52fb464`（console mining auth）— **云上 binary 之后**，故不含 |
| 10-02 09:30:30 | 本机提交 `36f9630`（orphan durability）— 云上不含 |
| 10-02 09:30~10:30 | 本机提交 7 个文档/治理提交 |
| 10-02 10:30:55 | 本机提交 `2647ba0` = v0.9.0-rc1 tag |

**构建方式判定**：
- 云上 binary 构建于 **10-01 17:54**，即 `c3cec3f3` 提交后约 37 分钟。
- 构建参数：`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 -trimpath=true`（buildvcs 默认开启）。
- 工作树带 `+dirty`（`vcs.modified=true`），说明构建时还有未提交的本地改动，但**代码特征审计确认编译产物精确对应 c3cec3f3 逻辑**（含 O1 guard、不含 console auth、不含 orphan checkpoint）。

### 代码特征交叉验证（决定性证据）

| 特征符号 | 对应提交 | 云上 binary | 结论 |
|----------|---------|------------|------|
| `dropPeerByAddr` / `GENESIS_MISMATCH` | `c3cec3f3` | ✅ 存在 | 含 O1 guard |
| `consoleOriginGate` | `52fb464` | ❌ 不存在 | 缺 console auth |
| `newOrphanCheckpoint` / `orphanCheckpoint` | `36f9630` | ❌ 不存在 | 缺 orphan checkpoint |
| `deferOrphan`（旧基础 orphan 逻辑） | 更早 commit | ✅ 存在 | 基础 orphan 逻辑已有 |

---

## 关系判定与最终状态

### 关系：**同源，云上 = RC 前 9 个提交的中间部署**

- `c3cec3f3` 与 `2647ba0` **同仓库、同 main 分支、同一条提交链**。
- 云上 binary 代码 = `c3cec3f3`，落后 RC 发布点 **9 提交**，其中 **2 个是功能代码提交**（console mining auth、orphan durability）。
- 云上 binary 含 `-trimpath=true`（与 RC 一致），但用了 `-buildvcs=true`（与 RC 的 `-buildvcs=false` 不同）。

### 最终状态：🟡 **YELLOW**

**判定理由**：
- 节点运行正常、共识健康（上一阶段已确认 height=136 三节点一致）。
- 云上 binary **与 RC 同源但落后**（缺 2 个功能提交 + 构建参数 buildvcs 差异），**不是** v0.9.0-rc1 本身。
- 非 RED（已确认非异源、非损坏、非异常数据）；非 GREEN（代码与 RC 不完全一致，缺 console auth 与 orphan durability 两个已提交的安全/韧性修复）。

---

## HARD STOP

本阶段为纯只读追踪，已完成并停止。**未执行、也将不执行**：停止节点、替换 binary、改配置、改数据、部署。

**云上 binary 缺的 2 个功能提交（对 RC 而言是实质差距）：**

| 缺失提交 | 功能 | 影响面 |
|----------|------|--------|
| `52fb464` | console mining endpoint 鉴权 | 控制台挖矿接口无鉴权（安全） |
| `36f9630` | orphan durability（§4-B1/B1.5/B2） | 孤儿块崩溃恢复韧性（数据安全） |

**待 Owner 决策（需单独授权）：**

| 选项 | 动作 | 说明 |
|------|------|------|
| 1 | 云上部署 RC（`2647ba0`）对应 Linux binary，替换 + 重启 3 节点 | 补齐 2 个功能提交 + 确定性构建；需快照回滚预案 |
| 2 | 维持现状，接受云上为「RC 前哨预览环境」 | 明确落后 9 提交、缺 2 个安全/韧性修复，需评估风险 |
| 3 | 仅补齐 2 个功能提交（升级到 `36f9630`），暂不追 RC 文档提交 | 最小功能对齐，仍非 RC 身份 |
