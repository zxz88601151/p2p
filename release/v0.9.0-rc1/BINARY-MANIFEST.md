# RC-BINARY-MANIFEST.md — 官方 RC 二进制清单

> 本清单记录 P2PChain Release Candidate 官方二进制的**可复现构建参数**与**溯源信息**。
> 用于对外分发时的完整性核对，以及未来灾难恢复时的重建依据。

---

## 官方 RC 二进制

| 项 | 值 |
|----|----|
| 文件名 | `node-v0.9.0-rc1.exe` |
| 大小 | 12,112,896 字节 |
| SHA-256 | `17e5ce8180f66526ed06826bbe84c74949e8f01b114f2d1895aa2964bfeb0edf` |
| 版本标签 | `v0.9.0-rc1`（Developer Node Release Candidate，非生产加密货币发布） |

## 冻结构建参数（Reproducibility）

| 参数 | 值 |
|------|----|
| 源码提交（source commit） | `3e6d44a248013f814859cf4f5be6feb5bc57f492` |
| tree 哈希 | `d5065ee7714bea533674b6d723d7ec11187e47f9` |
| Go 版本 | `go1.27.0 windows/amd64` |
| OS / 架构 | `windows / amd64` |
| 构建命令 | `go build -trimpath -o node-rc.exe ./cmd/node` |
| 构建标志 | `-trimpath`（去除路径内嵌，保证跨目录可复现） |

## VCS 溯源（内嵌元数据）

| 字段 | 值 |
|------|----|
| `vcs` | `git` |
| `vcs.revision` | `3e6d44a248013f814859cf4f5be6feb5bc57f492` |
| `vcs.time` | `2026-10-02T01:52:59Z` |
| `vcs.modified` | `false`（干净工作树，无未提交改动） |
| `mod` 伪版本 | `v0.0.0-20261002015259-3e6d44a24801`（无 `+dirty` 后缀） |

## 复现验证命令（Reproduce & Verify）

```bash
# 1. 干净 checkout
git worktree add --detach /tmp/p2pchain-rc-verify 3e6d44a248013f814859cf4f5be6feb5bc57f492
cd /tmp/p2pchain-rc-verify

# 2. 冻结参数重建
go build -trimpath -o node-rc.exe ./cmd/node

# 3. 核对哈希（期望 17e5ce81…）
sha256sum node-rc.exe

# 4. 核对溯源
go version -m node-rc.exe | grep -E 'vcs.revision|vcs.modified'
```

---

## 历史开发工件（非 RC，保留供参考，不对外分发）

| 文件 | SHA-256 | VCS 溯源 | 状态 |
|------|---------|----------|------|
| `node.exe`（开发工件） | `fb3d2b57dd8adbd02d3c6d501f42a2cfa3c076657694e35756967caf9307b717` | `vcs.revision=52fb464…` + `modified=true`（dirty） | ⚠️ 溯源缺口，仅本地测试 |

> 说明：`node.exe` 构建于旧提交 `52fb464` + dirty 工作树（孤儿代码已写未提交），VCS 溯源与 RC 边界 `3e6d44a` 不符，故不作为官方 RC 二进制。

---

## 完整性声明

- 官方 RC 二进制 `node-v0.9.0-rc1.exe` 由干净 checkout（`3e6d44a`，零漂移）+ 冻结参数（`-trimpath`）构建。
- VCS 溯源 `vcs.revision=3e6d44a` + `vcs.modified=false` 已核验，与 RC 边界严格一致。
- 此二进制为 **source-only release** 的构建产物，不含任何链数据 / 钱包 / 凭据（均被 gitignore 排除）。
