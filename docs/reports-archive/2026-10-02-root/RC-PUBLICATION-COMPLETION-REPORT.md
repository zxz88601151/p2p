# RC-PUBLICATION-COMPLETION-REPORT.md — 发布完成报告

> **阶段**：PHASE-P2PCHAIN-RC-PUBLICATION-RETRY-1 → 发布完成
> **日期**：2026-10-02
> **结果**：✅ **发布成功**

---

## 发布目标（已达成）

| 项 | 值 |
|----|----|
| 版本 | `v0.9.0-rc1` |
| tag | `2647ba0eb23a94a11aa71326e9b0686bbe1b78d8` |
| binary | `node-v0.9.0-rc1.exe` |
| SHA256 | `71097357531876b9975998d1d316f0888543a4aea1dcb9456c160df50e6352b1` |
| 目标远端 | github `zxz88601151/p2p` |

---

## 发布动作（全部成功）

| 步骤 | 结果 |
|------|------|
| §1 push main | ✅ `[new branch] main -> main`，建立上游跟踪 |
| §2 push annotated tag | ✅ `[new tag] v0.9.0-rc1 -> v0.9.0-rc1` |
| §3 remote verification | ✅ 全部通过 |

---

## 远端验证详情

| 验证项 | 远端状态 |
|--------|----------|
| `refs/heads/main` | ✅ `2647ba0eb23a94a11aa71326e9b0686bbe1b78d8` |
| `refs/tags/v0.9.0-rc1`（tag 对象） | ✅ `6800723c2ab829ee30efdf4d0d424916c6e9f95c`（annotated） |
| `refs/tags/v0.9.0-rc1^{}`（解引用） | ✅ `2647ba0eb23a94a11aa71326e9b0686bbe1b78d8`（tag target 正确） |
| GitHub API `pushed_at` | ✅ `2026-10-02T02:44:24Z` |
| GitHub API tag 列表 | ✅ `v0.9.0-rc1` → sha `2647ba0…` |

---

## 发布内容

### 提交链（7 提交，已全部推送）

```
36f9630  feat(node): orphan durability
8410036  docs: synchronize documentation truth
3e6d44a  docs: synchronize release governance status metadata  ← 业务源码边界
83914d1  docs(release): add RC evidence closure artifacts
bbbfab7  docs(release): seal RC release certificate
a69d24e  docs(release): finalize RC binary provenance alignment
2647ba0  docs(release): adopt deterministic build model  ← tag v0.9.0-rc1
```

### 发布二进制

| 项 | 值 |
|----|----|
| 文件名 | `node-v0.9.0-rc1.exe` |
| SHA-256 | `71097357531876b9975998d1d316f0888543a4aea1dcb9456c160df50e6352b1` |
| 构建模型 | `-buildvcs=false -trimpath`（external attestation，确定性可复现） |

---

## 发布定位声明

**v0.9.0-rc1 = Developer Node Release Candidate（非生产加密货币发布）**

- 学习/实验候选版本，未做安全审计，不得用于真实资产场景。
- source-only release，不含链数据 / 钱包 / 凭据。

---

## 发布完成状态

| 项 | 状态 |
|----|------|
| github main | ✅ 已推送 |
| github tag v0.9.0-rc1 | ✅ 已推送（annotated） |
| gitea | ⏸ 未推送（SSH 仍离线，可选后续补推） |
| 二进制下载 | ⏸ 二进制 `node-v0.9.0-rc1.exe` 未上传到 GitHub Release（需手动创建 Release 并附二进制 + SHA256SUMS） |

---

## 后续可选动作（需 Owner 另行授权）

1. **创建 GitHub Release**：在 github 网页创建 v0.9.0-rc1 Release，附 `node-v0.9.0-rc1.exe` + `SHA256SUMS`。
2. **补推 gitea**：待 gitea `192.168.3.123` 恢复后，可补推 `git push gitea main` + `git push gitea v0.9.0-rc1`。

---

## 结论

**v0.9.0-rc1 已成功发布到 github `zxz88601151/p2p`**，tag 与提交全部就位，tag target 与二进制 SHA256 与冻结快照完全一致。

—— 发布完成 ——
