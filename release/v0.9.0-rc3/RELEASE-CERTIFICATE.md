# RELEASE-CERTIFICATE.md — Release Candidate 发布证书（v0.9.0-rc3）

> 本证书为 P2PChain `v0.9.0-rc3` Release Candidate 的**不可变版本身份证明**，
> 记录 tag → commit → source → binary 的完整 provenance 链路。
> **Provenance 模型**：external attestation（二进制不内嵌 VCS，溯源由本证书外部记录）。

---

## 版本身份（Immutable Version Identity）

| 项 | 值 |
|----|----|
| 版本标签（annotated tag） | `v0.9.0-rc3` |
| 定位 | Developer Node Release Candidate（**非生产加密货币发布**） |
| 发布状态 | ✅ P0-4 / P0-5 安全修复完成，已打 annotated tag |
| 打标日期 | 2026-10-02 |
| 前驱版本 | `v0.9.0-rc2`（tag → `433d82b`） |
| 破坏性变更 | ⚠️ **P0-4 为破坏性变更**：钱包从明文迁移到加密，旧 `wallet.json` 必须手动 `wallet encrypt` 迁移 |

---

## 本版本变更（相对 v0.9.0-rc2）

| 提交 | 修复项 | 类别 |
|------|--------|------|
| `9575863` | P0-4 钱包静态加密（PBKDF2-HMAC-SHA256 60 万轮 + AES-256-GCM，v2 信封，口令 fail-closed） | 密钥安全（破坏性） |
| `0c5bc3a` | P0-5 Eclipse 缓解（槽位划分、不良行为记分封禁、种子优先通道、地址本硬化） | 网络抗攻击 |

**业务源码边界**：`433d82b`（rc2 tag 目标）→ 本版本在 rc2 之上新增 2 个语义提交（P0-4、P0-5）。

---

## Provenance 链路（tag → commit → source → binary）

### 1. tag → commit

| 项 | 值 |
|----|----|
| annotated tag | `v0.9.0-rc3`（`git cat-file -t` = `tag`） |
| 指向 commit | `0c5bc3a791e2f16c7569f33f109898a3f3b34057` |
| tag message | P0-4 钱包静态加密 + P0-5 Eclipse 缓解（详见 tag 注解） |

### 2. commit → source（相对 rc2 的增量）

| 项 | 值 |
|----|----|
| rc2 基线（tag target） | `433d82b69d1a98cc1ef00e53a401cdf3571b5a49` |
| rc3 最终提交 | `0c5bc3a791e2f16c7569f33f109898a3f3b34057` |

**rc3 增量提交链（main 分支，2 提交）**：

| 顺序 | SHA | 说明 |
|------|-----|------|
| 1 | `9575863` | feat(wallet): P0-4 钱包静态加密（PBKDF2-HMAC-SHA256 + AES-256-GCM） |
| 2 | `0c5bc3a` | feat(p2p): P0-5 Eclipse 缓解（槽位划分 + 记分封禁 + 种子优先，tag target） |

### 3. source → binary（External Attestation）

| 项 | 值 |
|----|----|
| 官方二进制 | `p2pchain-linux-amd64-v0.9.0-rc3`（Linux 部署产物） |
| SHA-256 | `5ab7d64a8d6814e77aa473e73b39ca8b31c03fe5d478653cb1c8e3bdcac3cee9` |
| VCS 内嵌 | ❌ 无（`-buildvcs=false`） |
| 构建参数 | `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -trimpath -o p2pchain-linux-amd64-v0.9.0-rc3 ./cmd/node` |
| Go 版本 | `go1.27.0`（linux/amd64 交叉编译） |
| Provenance 模型 | **external attestation**（身份由源码内容决定，溯源由本证书外部记录） |
| 确定性复验 | ✅ 两次独立构建 SHA256 完全一致 `5ab7d64a…` |

---

## 破坏性变更说明（P0-4 钱包加密，部署前必读）

P0-4 将钱包私钥从明文（v1）迁移到静态加密（v2）。**升级到 rc3 时需手动迁移**：

1. **口令文件**：节点启动命令必须新增 `--wallet-password-file <0600 口令文件>`；缺失则 **fail-closed 拒绝启动**。
2. **旧钱包迁移**：旧明文 `wallet.json` 必须手动执行 `p2pchain wallet encrypt --datadir <dir> --password-file <pw>`，成功后才可启动节点。
3. **路径变更**：新钱包位于 `<datadir>/secrets/wallet.json`（`secrets` 目录 0700）。
4. **systemd 部署**：需通过 `LoadCredential` 或挂载 0600 secret 提供口令文件；启动命令加 `--wallet-password-file`。
5. **无口令读地址**：`wallet --address` 无需口令（v2 信封公钥为明文）。

---

## 不可变性声明（Immutability Declaration）

1. **源码冻结**：rc3 业务源码固定于 `0c5bc3a`，相对 rc2（`433d82b`）新增 2 个语义提交（P0-4 钱包、P0-5 网络）。
2. **二进制冻结**：官方 Linux 二进制 `p2pchain-linux-amd64-v0.9.0-rc3` 的 SHA-256 固定为 `5ab7d64a…`。
3. **确定性可复现**：冻结参数（`0c5bc3a` checkout + `-buildvcs=false -trimpath` + Go1.27.0）下，两次独立构建 SHA256 完全一致。
4. **tag 不可变**：annotated tag `v0.9.0-rc3` 已创建，指向 `0c5bc3a`。

---

## 排除与限制（Exclusions & Limitations）

- **非生产发布**：本 RC 定位为 Developer Node 学习/实验候选，未做安全审计，不得用于真实资产场景。
- **source-only release**：不含链数据 / 钱包 / 凭据 / 孤儿检查点（均被 gitignore 排除）。
- **回归测试**：新增 24 项回归测试全过（wallet 10 + p2p 8 + cmd 6）；反向对照确认旧代码漏洞行为。
- **已知测试平台差异**：`keystore_test.go` 的 0600 权限断言在 Windows 上跳过（与生产代码 `runtime.GOOS != "windows"` 守卫一致）；cmd/node 全量测试存在既有子进程 kill 超时（与本次补丁无关）。
- **部署状态**：源码与 tag 已 push 至 GitHub `origin/main`；腾讯云三节点升级待后续部署阶段执行（P0-4 破坏性变更需先迁移钱包）。

---

## 签发（Issuance）

- 签发人：Owner（经 p2pchain-baseline 执行）
- 签发时间：2026-10-02
- 生效范围：本地仓库 `main` 分支 + GitHub `origin/main`

**本证书为 v0.9.0-rc3 的正式不可变身份凭证，随 tag 一并封存。**
