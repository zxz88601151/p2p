# RC-RELEASE-CERTIFICATE.md — Release Candidate 发布证书（v0.9.0-rc2）

> 本证书为 P2PChain `v0.9.0-rc2` Release Candidate 的**不可变版本身份证明**，
> 记录 tag → commit → source → binary 的完整 provenance 链路。
> **Provenance 模型**：external attestation（二进制不内嵌 VCS，溯源由本证书外部记录）。

---

## 版本身份（Immutable Version Identity）

| 项 | 值 |
|----|----|
| 版本标签（annotated tag） | `v0.9.0-rc2` |
| 定位 | Developer Node Release Candidate（**非生产加密货币发布**） |
| 发布状态 | ✅ P0 安全修复完成，已打 annotated tag |
| 打标日期 | 2026-10-02 |
| 前驱版本 | `v0.9.0-rc1`（tag → `2647ba0`） |

---

## 本版本变更（相对 v0.9.0-rc1）

| 提交 | 修复项 | 类别 |
|------|--------|------|
| `33e4fcb` | P0-1 共识绕过：fork 块入树前做头部 PoW/难度/时间戳/Merkle/体积预检 | 共识安全 |
| `433d82b` | P0-2 分发异步化 + P0-3 SendTo 非阻塞（读循环不再被慢业务/慢对端阻塞） | 网络层阻塞 |

**业务源码边界**：`2647ba0`（rc1 tag 目标）→ 本版本在 rc1 之上新增 2 个修复提交。

---

## Provenance 链路（tag → commit → source → binary）

### 1. tag → commit

| 项 | 值 |
|----|----|
| annotated tag | `v0.9.0-rc2`（`git cat-file -t` = `tag`） |
| 指向 commit | `433d82b69d1a98cc1ef00e53a401cdf3571b5a49` |
| tag message | P0 安全修复：P0-1 共识绕过（fork 块入树前头部预检）+ P0-2/P0-3 P2P 阻塞（分发异步化、SendTo 走出站队列） |

### 2. commit → source（相对 rc1 的增量）

| 项 | 值 |
|----|----|
| rc1 基线（tag target） | `2647ba0eb23a94a11aa71326e9b0686bbe1b78d8` |
| rc2 最终提交 | `433d82b69d1a98cc1ef00e53a401cdf3571b5a49` |

**rc2 增量提交链（main 分支，2 提交）**：

| 顺序 | SHA | 说明 |
|------|-----|------|
| 1 | `33e4fcb` | fix(blockchain): pre-validate fork block header PoW（P0-1） |
| 2 | `433d82b` | fix(p2p): async dispatch + non-blocking SendTo（P0-2/P0-3，tag target） |

### 3. source → binary（External Attestation）

| 项 | 值 |
|----|----|
| 官方二进制 | `p2pchain-linux-amd64-p0fix`（Linux 部署产物） |
| SHA-256 | `b48f0b1d35900cb6bb4f03912fcf7c6f13820428f1b6ea1941ee0e3f5c52cee4` |
| VCS 内嵌 | ❌ 无（`-buildvcs=false`） |
| 构建参数 | `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -trimpath -o p2pchain-linux-amd64-p0fix ./cmd/node` |
| Go 版本 | `go1.27.0`（linux/amd64 交叉编译） |
| Provenance 模型 | **external attestation**（身份由源码内容决定，溯源由本证书外部记录） |
| 确定性复验 | ✅ 两次独立构建 SHA256 完全一致 `b48f0b1d...` |

---

## 不可变性声明（Immutability Declaration）

1. **源码冻结**：rc2 业务源码固定于 `433d82b`，相对 rc1（`2647ba0`）新增 2 个修复提交（P0-1 共识、P0-2/P0-3 网络）。
2. **二进制冻结**：官方 Linux 二进制 `p2pchain-linux-amd64-p0fix` 的 SHA-256 固定为 `b48f0b1d…`。
3. **确定性可复现**：冻结参数（`433d82b` checkout + `-buildvcs=false -trimpath` + Go1.27.0）下，两次独立构建 SHA256 完全一致。
4. **tag 不可变**：annotated tag `v0.9.0-rc2` 已创建，指向 `433d82b`。

---

## 关于 rc1 证书的历史矛盾（如实注明，不改写历史）

按仓库 RC 流程要求，此处**如实记录** `release/v0.9.0-rc1/RELEASE-CERTIFICATE.md` 中随历史演进产生的矛盾，**不追溯改写该历史文件**：

| rc1 证书中的陈述 | 当前实际 | 矛盾说明 |
|------------------|----------|----------|
| "指向 commit `a69d24e`" | `v0.9.0-rc1` 现指向 `2647ba0` | rc1 证书签发后，经 PROVENANCE-REALIGNMENT / DETERMINISTIC-BUILD-FINALIZATION 阶段，tag 重新打标至 `2647ba0`（采纳 `-buildvcs=false` 确定性构建） |
| "未 push（本地唯一）" | rc1 已 push 至 GitHub（`zxz88601151/p2p`） | 证书签发时确为本地唯一，随后经 PUBLICATION-EXECUTION 阶段推送 |
| "tag target = `a69d24e`" | tag target = `2647ba0` | 同上，tag 移动是 provenance realignment 的既定结果 |

> 结论：rc1 证书反映了**其签发时点**的真实状态；上述差异是后续 provenance realignment / publication 阶段的合法演进结果，而非数据损坏。本 rc2 证书以当前 `git` 实际状态为准。

---

## 排除与限制（Exclusions & Limitations）

- **非生产发布**：本 RC 定位为 Developer Node 学习/实验候选，未做安全审计，不得用于真实资产场景。
- **source-only release**：不含链数据 / 钱包 / 凭据 / 孤儿检查点（均被 gitignore 排除）。
- **部署状态**：本 rc2 binary 已部署至腾讯云三节点（滚动升级完成，验证 GREEN）；源码已 push 至 GitHub `origin/main`。
- **tag 推送状态**：截至本证书签发，`v0.9.0-rc2` tag 已本地创建；推送 GitHub 时因本地代理 `127.0.0.1:55611` 对 github CONNECT 隧道返回 502（且本机无法直连 github.com:443）而暂未推送成功，待代理恢复后补推。

---

## 签发（Issuance）

- 签发人：Owner（经 p2pchain-baseline 执行）
- 签发时间：2026-10-02
- 生效范围：本地仓库 `main` 分支 + 腾讯云三节点部署

**本证书为 v0.9.0-rc2 的正式不可变身份凭证，随 tag 一并封存。**
