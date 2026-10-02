# FINAL-RC-SNAPSHOT.md — Release Candidate 最终冻结快照

> 本快照为 P2PChain `v0.9.0-rc1` 发布前**最终冻结状态**的权威记录。
> 生成于发布授权审计阶段，冻结后不再变更。

---

## 冻结身份（Frozen Identity）

| 项 | 值 |
|----|----|
| 版本标签（annotated tag） | `v0.9.0-rc1` |
| tag 完整 SHA | `2647ba0eb23a94a11aa71326e9b0686bbe1b78d8` |
| tag 类型 | `tag`（annotated） |
| 定位 | Developer Node Release Candidate（**非生产加密货币发布**） |

---

## 冻结二进制（Frozen Binary）

| 项 | 值 |
|----|----|
| 文件名 | `node-v0.9.0-rc1.exe` |
| SHA-256 | `71097357531876b9975998d1d316f0888543a4aea1dcb9456c160df50e6352b1` |
| 大小 | 12,112,384 字节 |

---

## 冻结构建参数（Frozen Build Flags）

| 参数 | 值 |
|------|----|
| 构建命令 | `go build -buildvcs=false -trimpath -o node-rc.exe ./cmd/node` |
| 构建标志 | `-buildvcs=false -trimpath` |
| Go 版本 | `go1.27.0 windows/amd64` |
| OS / 架构 | `windows / amd64` |
| Provenance 模型 | external attestation（二进制不内嵌 VCS） |

---

## 可复现性结果（Reproducibility Result）

| 构建来源 | SHA-256 | 一致性 |
|----------|---------|--------|
| worktree 首次构建 | `71097357…` | ✅ |
| worktree rebuild | `71097357…` | ✅ |
| 独立 clone #1 checkout tag | `71097357…` | ✅ |
| 独立 clone #2 checkout tag（最终验证） | `71097357…` | ✅ |

**结论**：✅ **Deterministically Reproducible** —— 四次独立构建 SHA256 完全一致。

---

## 业务源码边界（Source Boundary）

| 项 | 值 |
|----|----|
| 业务源码最终提交 | `3e6d44a248013f814859cf4f5be6feb5bc57f492` |
| 业务代码差异 | `3e6d44a` → `2647ba0` 之间 `.go` 文件**零差异**（仅文档提交） |

---

## RC 提交链（7 提交）

```
36f9630  feat(node): orphan durability
8410036  docs: synchronize documentation truth
3e6d44a  docs: synchronize release governance status metadata  ← 业务源码边界
83914d1  docs(release): add RC evidence closure artifacts
bbbfab7  docs(release): seal RC release certificate
a69d24e  docs(release): finalize RC binary provenance alignment
2647ba0  docs(release): adopt deterministic build model  ← tag v0.9.0-rc1
```

---

## 发布限制声明（Publication Constraints）

- **非生产发布**：Developer Node 学习/实验候选，未做安全审计，不得用于真实资产场景。
- **source-only release**：不含链数据 / 钱包 / 凭据 / 孤儿检查点。
- **未 push / 未 deploy**：tag 与提交目前仅存在于本地 `main`。

---

*本快照冻结于 2026-10-02，随发布授权审计一并封存。*
