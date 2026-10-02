# RELEASE-CERTIFICATE.md — Release Candidate 发布证书（v0.9.0-rc4）

> 本证书为 P2PChain `v0.9.0-rc4` Release Candidate 的**不可变版本身份证明**，
> 记录 tag → commit → source → binary 的完整 provenance 链路。
> **Provenance 模型**：external attestation（二进制不内嵌 VCS，溯源由本证书外部记录）。

---

## 版本身份（Immutable Version Identity）

| 项 | 值 |
|----|----|
| 版本标签（annotated tag） | `v0.9.0-rc4` |
| 定位 | Developer Node Release Candidate（**非生产加密货币发布**） |
| 发布状态 | ✅ P0-6 / P0-8 安全加固完成，已打 annotated tag |
| 打标日期 | 2026-10-02 |
| 前驱版本 | `v0.9.0-rc3`（tag → `0c5bc3a`） |
| 破坏性变更 | ❌ **无**（本版本仅控制面鉴权加固 + 文档修正，钱包与存储格式不变） |

---

## 本版本变更（相对 v0.9.0-rc3）

| 提交 | 变更项 | 类别 |
|------|--------|------|
| `73463e5` | chore(release): rc3 发布证书与校验和归档 | 发布治理 |
| `7cc1fc2` | docs: 修正过时的生产链高度（1275/1765 → 137）与 `MaxBlockSize` 编码描述 | 文档准确性 |
| `79a58f5` | docs: 修正 README 过时的实现状态声明（移除陈旧 commit hash / 阶段标签） | 文档准确性 |
| `f1e1771` | **P0-6** 鉴权文案修正 + **P0-8** 非回环控制接口 fail-closed | 控制面安全 |

**P0-8 语义**：`-rpc` 指向非回环地址时**默认拒绝启动**（检查位于数据目录加锁前，无副作用），
必须显式 `--allow-non-loopback` 确认；此前仅记一条日志警告仍继续启动。
**P0-6 语义**：修正「非回环地址且无鉴权」误导文案——mutation 6 端点已强制 Bearer，
无鉴权的是 7 个只读端点（其安全边界为回环绑定）。

**业务源码边界**：`0c5bc3a`（rc3 tag 目标）→ 本版本在 rc3 之上新增 3 个语义提交（文档 ×2 + P0-6/P0-8 ×1）。

---

## Provenance 链路（tag → commit → source → binary）

### 1. tag → commit

| 项 | 值 |
|----|----|
| annotated tag | `v0.9.0-rc4`（`git cat-file -t` = `tag`） |
| 指向 commit | `f1e17712c57d7f8dd5f860b018da4a5fc67ae569` |
| tag message | P0-6 鉴权文案修正 + P0-8 非回环控制接口 fail-closed（详见 tag 注解） |

### 2. commit → source（相对 rc3 的增量）

| 项 | 值 |
|----|----|
| rc3 基线（tag target） | `0c5bc3a791e2f16c7569f33f109898a3f3b34057` |
| rc4 最终提交 | `f1e17712c57d7f8dd5f860b018da4a5fc67ae569` |

**rc4 增量提交链（main 分支，4 提交）**：

| 顺序 | SHA | 说明 |
|------|-----|------|
| 1 | `73463e5` | chore(release): add v0.9.0-rc3 release certificate and checksums |
| 2 | `7cc1fc2` | docs: 修正过时的生产链高度与 MaxBlockSize 编码描述 |
| 3 | `79a58f5` | docs: 修正 README 过时的实现状态声明 |
| 4 | `f1e1771` | fix(node): P0-6 鉴权文案修正 + P0-8 非回环控制接口 fail-closed（tag target） |

### 3. source → binary（External Attestation）

| 项 | 值 |
|----|----|
| 官方二进制 | `p2pchain-linux-amd64-v0.9.0-rc4`（Linux 部署产物） |
| SHA-256 | `0e1b76cfc38e069006f9646716a116b0158bffc00068120fde9272c5be0bdc48` |
| VCS 内嵌 | ❌ 无（`-buildvcs=false`） |
| 构建参数 | `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -trimpath -o p2pchain-linux-amd64-v0.9.0-rc4 ./cmd/node` |
| Go 版本 | `go1.27.0`（linux/amd64 交叉编译） |
| Provenance 模型 | **external attestation**（身份由源码内容决定，溯源由本证书外部记录） |
| 确定性复验 | ✅ 两次独立构建 SHA256 完全一致 `0e1b76cf…` |
| 特征串核验 | ✅ 二进制含 `allow-non-loopback` flag 及 P0-8 fail-closed / P0-6 修正文案 |

---

## 兼容性说明（部署前必读）

本版本**无破坏性变更**：

1. **控制面**：云上三节点 RPC 均绑定 `127.0.0.1`（回环），P0-8 的新 fail-closed 逻辑**不改变既有启动行为**，无需新增任何 flag。
2. **钱包**：沿用 rc3 的 v2 加密信封与 `--wallet-password-file` 机制，**无需迁移**。
3. **存储**：`blocks.dat` / v2 帧格式不变，**无需迁移**。
4. **P2P**：协议面不变。

因此升级 = 原子替换二进制 + 重启，无前置迁移步骤。

---

## 不可变性声明（Immutability Declaration）

1. **源码冻结**：rc4 业务源码固定于 `f1e1771`，相对 rc3（`0c5bc3a`）新增 3 个语义提交。
2. **二进制冻结**：官方 Linux 二进制 `p2pchain-linux-amd64-v0.9.0-rc4` 的 SHA-256 固定为 `0e1b76cf…`。
3. **确定性可复现**：冻结参数（`f1e1771` checkout + `-buildvcs=false -trimpath` + Go1.27.0）下，两次独立构建 SHA256 完全一致。
4. **tag 不可变**：annotated tag `v0.9.0-rc4` 已创建，指向 `f1e1771`。

---

## 排除与限制（Exclusions & Limitations）

- **非生产发布**：本 RC 定位为 Developer Node 学习/实验候选，未做安全审计，不得用于真实资产场景。
- **source-only release**：不含链数据 / 钱包 / 凭据 / 孤儿检查点（均被 gitignore 排除）。
- **回归测试**：P0-6/P0-8 新增 3 项回归（fail-closed / 显式放行 / flag 注册）；`go build ./...` / `go vet ./...` 干净。
- **已知测试平台差异**：`keystore_test.go` 的 0600 权限断言在 Windows 上跳过（与生产代码 `runtime.GOOS != "windows"` 守卫一致）。
- **部署状态**：源码与 tag 已 push 至 GitHub `origin/main`；腾讯云三节点滚动升级见部署记录。

---

## 签发（Issuance）

- 签发人：Owner（经 p2pchain-baseline 执行）
- 签发时间：2026-10-02
- 生效范围：本地仓库 `main` 分支 + GitHub `origin/main`

**本证书为 v0.9.0-rc4 的正式不可变身份凭证，随 tag 一并封存。**
