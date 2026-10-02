# RC-BINARY-MANIFEST.md — 官方 RC 二进制清单

> 本清单记录 P2PChain Release Candidate 官方二进制的**可复现构建参数**与**溯源信息**。
> 用于对外分发时的完整性核对，以及未来灾难恢复时的重建依据。

---

## 官方 RC 二进制

| 项 | 值 |
|----|----|
| 文件名 | `node-v0.9.0-rc1.exe` |
| 大小 | 12,112,896 字节 |
| SHA-256 | `90f86e9a4c4d465e1c1ea78b4930e9077532ed124a074b102ae65f254030efb6` |
| 版本标签 | `v0.9.0-rc1`（Developer Node Release Candidate，非生产加密货币发布） |

## 冻结构建参数（Reproducibility）

| 参数 | 值 |
|------|----|
| 源码提交（source commit） | `bbbfab7760259ef275afcad2980c6e7b3ca12773`（tag `v0.9.0-rc1` target） |
| tree 哈希 | `0de5660266a111e55defa1dca4a69d6c9182dff9` |
| Go 版本 | `go1.27.0 windows/amd64` |
| OS / 架构 | `windows / amd64` |
| 构建命令 | `go build -buildvcs=true -trimpath -o node-rc.exe ./cmd/node` |
| 构建标志 | `-buildvcs=true -trimpath`（保留 VCS 溯源 + 去除路径内嵌，保证跨目录可复现） |

## VCS 溯源（内嵌元数据）

| 字段 | 值 |
|------|----|
| `vcs` | `git` |
| `vcs.revision` | `bbbfab7760259ef275afcad2980c6e7b3ca12773`（**== tag target**） |
| `vcs.time` | `2026-10-02T02:13:38Z` |
| `vcs.modified` | `false`（干净工作树，无未提交改动） |
| `mod` 版本 | `v0.9.0-rc1`（tag 名，无 `+dirty` 后缀） |

## 复现验证命令（Reproduce & Verify）

```bash
# 1. 干净 checkout（tag，保证 vcs.revision 推导 = tag target）
git worktree add --detach /tmp/p2pchain-rc-verify v0.9.0-rc1
cd /tmp/p2pchain-rc-verify

# 2. 冻结参数重建
go build -buildvcs=true -trimpath -o node-rc.exe ./cmd/node

# 3. 核对哈希（期望 90f86e9a…）
sha256sum node-rc.exe

# 4. 核对溯源（revision 必须 = bbbfab7）
go version -m node-rc.exe | grep -E 'vcs.revision|vcs.modified'
```

---

## 历史工件（非 RC，保留供参考，不对外分发）

| 文件 | SHA-256 | VCS 溯源 | 状态 |
|------|---------|----------|------|
| `node.exe`（开发工件） | `fb3d2b57dd8adbd02d3c6d501f42a2cfa3c076657694e35756967caf9307b717` | `vcs.revision=52fb464…` + `modified=true`（dirty） | ⚠️ 溯源缺口，仅本地测试 |
| `node-v0.9.0-rc1-historical-3e6d44a.exe`（历史 RC 构建） | `17e5ce8180f66526ed06826bbe84c74949e8f01b114f2d1895aa2964bfeb0edf` | `vcs.revision=3e6d44a…` + `modified=false` | ⚠️ 打 tag 前构建，revision 与 tag target 不符 |

> 说明：
> - `node.exe` 构建于旧提交 `52fb464` + dirty 工作树，溯源缺口。
> - `node-v0.9.0-rc1-historical-3e6d44a.exe` 是打 tag 前的 RC 构建（`vcs.revision=3e6d44a`），因 tag 未创建导致 Go VCS 版本推导与最终 tag target 不一致，已由 provenance 对齐修复阶段替换。

---

## 完整性声明

- 官方 RC 二进制 `node-v0.9.0-rc1.exe` 由干净 checkout（`v0.9.0-rc1` = `bbbfab7`，零漂移）+ 冻结参数（`-buildvcs=true -trimpath`）构建。
- VCS 溯源 `vcs.revision=bbbfab7` + `vcs.modified=false` 已核验，**与 tag target 严格一致**（provenance 对齐）。
- 可复现性：独立 clone + checkout tag + 重建，哈希 `90f86e9a…` 完全一致（Deterministically Reproducible）。
- 此二进制为 **source-only release** 的构建产物，不含任何链数据 / 钱包 / 凭据（均被 gitignore 排除）。
