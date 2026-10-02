# RC-RELEASE-CERTIFICATE.md — Release Candidate 发布证书

> 本证书为 P2PChain `v0.9.0-rc1` Release Candidate 的**不可变版本身份证明**，
> 记录 tag → commit → source → binary 的完整 provenance 链路。
> **Provenance 模型**：external attestation（二进制不内嵌 VCS，溯源由本证书外部记录）。

---

## 版本身份（Immutable Version Identity）

| 项 | 值 |
|----|----|
| 版本标签（annotated tag） | `v0.9.0-rc1` |
| 定位 | Developer Node Release Candidate（**非生产加密货币发布**） |
| 发布状态 | ✅ 确定性构建完成，已打 annotated tag |
| 打标日期 | 2026-10-02 |

---

## Provenance 链路（tag → commit → source → binary）

### 1. tag → commit

| 项 | 值 |
|----|----|
| annotated tag | `v0.9.0-rc1`（`git cat-file -t` = `tag`） |
| 指向 commit | `a69d24ef03c85ab6e669b81a4132749178de76d5` |

### 2. commit → source（业务源码边界）

| 项 | 值 |
|----|----|
| 业务源码最终提交 | `3e6d44a248013f814859cf4f5be6feb5bc57f492` |
| 祖先关系 | ✅ `3e6d44a` 是 `v0.9.0-rc1` 的祖先（业务代码零差异） |

**RC 提交链（main 分支，6 提交）**：

| 顺序 | SHA | 说明 |
|------|-----|------|
| 1 | `36f9630` | feat(node): orphan durability |
| 2 | `8410036` | docs: synchronize documentation truth |
| 3 | `3e6d44a` | docs: synchronize release governance status metadata（业务源码边界） |
| 4 | `83914d1` | docs(release): add RC evidence closure artifacts |
| 5 | `bbbfab7` | docs(release): seal RC release certificate |
| 6 | `a69d24e` | docs(release): finalize RC binary provenance alignment（tag target） |

### 3. source → binary（External Attestation）

| 项 | 值 |
|----|----|
| 官方二进制 | `node-v0.9.0-rc1.exe` |
| SHA-256 | `71097357531876b9975998d1d316f0888543a4aea1dcb9456c160df50e6352b1` |
| VCS 内嵌 | ❌ 无（`-buildvcs=false`） |
| 构建参数 | `go build -buildvcs=false -trimpath -o node-rc.exe ./cmd/node` |
| Go 版本 | `go1.27.0 windows/amd64` |
| Provenance 模型 | **external attestation**（身份由源码内容决定，溯源由本证书外部记录） |

---

## 不可变性声明（Immutability Declaration）

1. **源码冻结**：业务源码边界固定于 `3e6d44a`，此后未做任何源码/协议/共识修改（`3e6d44a` → `a69d24e` 仅文档提交，业务代码零差异）。
2. **二进制冻结**：官方 RC 二进制 `node-v0.9.0-rc1.exe` 的 SHA-256 固定为 `71097357…`。
3. **确定性可复现**：冻结参数（`v0.9.0-rc1` checkout + `-buildvcs=false -trimpath` + Go1.27.0）下，三次独立构建（worktree + 2× clone checkout tag）SHA256 完全一致 `71097357…`。
4. **tag 不可变**：annotated tag `v0.9.0-rc1` 已创建，指向 `a69d24e`，未 push（本地唯一）。

---

## 排除与限制（Exclusions & Limitations）

- **非生产发布**：本 RC 定位为 Developer Node 学习/实验候选，未做安全审计，不得用于真实资产场景。
- **source-only release**：不含链数据 / 钱包 / 凭据 / 孤儿检查点（均被 gitignore 排除）。
- **未 push**：tag 与提交目前仅存在于本地 `main`，尚未推送任何远端。
- **未 deploy**：未做任何部署。

---

## 签发（Issuance）

- 签发人：Owner（经 p2pchain-baseline 执行）
- 签发时间：2026-10-02
- 生效范围：本地仓库 `main` 分支

**本证书为 v0.9.0-rc1 的正式不可变身份凭证，随 tag 一并封存。**
