# RC-BINARY-MANIFEST.md — 官方 RC 二进制清单

> 本清单记录 P2PChain Release Candidate 官方二进制的**可复现构建参数**与**溯源信息**。
> **Provenance 模型**：external attestation（外部证明）——二进制不内嵌 VCS 信息，身份由源码内容 + 本清单的 SHA256 记录共同锚定。
> 用于对外分发时的完整性核对，以及未来灾难恢复时的重建依据。

---

## 官方 RC 二进制

| 项 | 值 |
|----|----|
| 文件名 | `node-v0.9.0-rc1.exe` |
| 大小 | 12,112,384 字节 |
| SHA-256 | `71097357531876b9975998d1d316f0888543a4aea1dcb9456c160df50e6352b1` |
| 版本标签 | `v0.9.0-rc1`（Developer Node Release Candidate，非生产加密货币发布） |

## 冻结构建参数（Reproducibility）

| 参数 | 值 |
|------|----|
| 源码提交（source commit） | `a69d24ef03c85ab6e669b81a4132749178de76d5`（tag `v0.9.0-rc1` target） |
| 业务源码边界 | `3e6d44a248013f814859cf4f5be6feb5bc57f492`（业务代码零差异） |
| Go 版本 | `go1.27.0 windows/amd64` |
| OS / 架构 | `windows / amd64` |
| 构建命令 | `go build -buildvcs=false -trimpath -o node-rc.exe ./cmd/node` |
| 构建标志 | `-buildvcs=false -trimpath` |

## Provenance 模型：External Attestation

**设计决策**：采用 `-buildvcs=false`，二进制**不内嵌** VCS 信息（revision/time/mod 版本）。

**原因**：`-buildvcs=true` 会导致「binary hash ↔ commit hash ↔ manifest SHA256」的自引用循环——二进制内嵌的 `vcs.revision`（commit hash）依赖含 SHA256 记录的文档 tree，而文档记录的 SHA256 又依赖二进制，无法收敛。

**External attestation 模型**：二进制身份 = **纯源码内容**（业务代码 + go.mod）的确定性函数，与文档/tag/commit 哈希完全解耦。溯源关系由**本清单 + RC-RELEASE-CERTIFICATE.md** 在外部记录：

```
tag v0.9.0-rc1 (a69d24e)
  └─ 源码提交 a69d24e（业务代码 = 3e6d44a，零差异）
       └─ go build -buildvcs=false -trimpath
            └─ node-v0.9.0-rc1.exe (SHA256 = 71097357…)
```

## 复现验证命令（Reproduce & Verify）

```bash
# 1. 干净 checkout（tag）
git worktree add --detach /tmp/p2pchain-rc-verify v0.9.0-rc1
cd /tmp/p2pchain-rc-verify

# 2. 冻结参数重建
go build -buildvcs=false -trimpath -o node-rc.exe ./cmd/node

# 3. 核对哈希（期望 71097357…）
sha256sum node-rc.exe
# 期望输出：71097357531876b9975998d1d316f0888543a4aea1dcb9456c160df50e6352b1

# 4. 确认无 VCS 内嵌（external attestation）
go version -m node-rc.exe | grep -c vcs.revision   # 期望 0
```

---

## 历史工件（非 RC，保留供参考，不对外分发）

| 文件 | SHA-256 | 说明 | 状态 |
|------|---------|------|------|
| `node.exe`（开发工件） | `fb3d2b57dd8adbd02d3c6d501f42a2cfa3c076657694e35756967caf9307b717` | 构建于旧提交 `52fb464` + dirty 工作树 | ⚠️ 溯源缺口，仅本地测试 |
| `node-v0.9.0-rc1-historical-3e6d44a.exe`（历史 RC 构建） | `17e5ce8180f66526ed06826bbe84c74949e8f01b114f2d1895aa2964bfeb0edf` | 打 tag 前构建，`vcs.revision=3e6d44a` | ⚠️ 已被确定性构建替换 |

> 说明：
> - `node.exe` 构建于旧提交 `52fb464` + dirty 工作树，溯源缺口。
> - `node-v0.9.0-rc1-historical-3e6d44a.exe` 是打 tag 前的 RC 构建，因 tag 未创建导致 Go VCS 版本推导与最终 tag target 不一致，已由确定性构建（`-buildvcs=false`）替换。

---

## 完整性声明

- 官方 RC 二进制 `node-v0.9.0-rc1.exe` 由干净 checkout（`v0.9.0-rc1` = `a69d24e`，零漂移）+ 冻结参数（`-buildvcs=false -trimpath`）构建。
- **确定性可复现**：三次独立构建（worktree + 两次 clone checkout tag）SHA256 完全一致 `71097357…`。
- **External attestation**：二进制不含 VCS 内嵌，身份纯由源码内容决定，与文档/tag 解耦，从根本上消除自引用循环。
- 此二进制为 **source-only release** 的构建产物，不含任何链数据 / 钱包 / 凭据（均被 gitignore 排除）。
