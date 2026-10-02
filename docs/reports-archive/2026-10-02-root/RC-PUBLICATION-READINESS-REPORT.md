# RC-PUBLICATION-READINESS-REPORT.md — RC 远端发布就绪报告

> **阶段**：PHASE-P2PCHAIN-RC-REMOTE-PUBLICATION-READINESS-1（发布前审计，不 push 不 deploy）
> **日期**：2026-10-02
> **治理指令**：在不 push、不 deploy 的前提下，完成 RC 远端发布前审计。

---

## §0 Remote State Audit

| 项 | 值 |
|----|----|
| git remote | `gitea`（ssh://192.168.3.123:22，SSH 不可达）、`origin`（github p2p，空仓库） |
| branch tracking | `main` → `gitea/main`，本地 **ahead 5**（5 个提交未推送） |
| local tag | `v0.9.0-rc1`（annotated，指向 `bbbfab7760259ef275afcad2980c6e7b3ca12773`） |
| HEAD | `bbbfab7760259ef275afcad2980c6e7b3ca12773` |

**本地领先 gitea 的 5 个提交**（未推送）：
`36f9630` → `8410036` → `3e6d44a` → `83914d1` → `bbbfab7`

---

## §1 Secret / Sensitive Scan ✅

| 扫描项 | 结果 |
|--------|------|
| 私钥 / 助记词 / 明文密钥 | ✅ 零命中（仅 `internal/wallet/*.go` 的 ECDSA 代码逻辑，非真实密钥） |
| 明文 token / 密码 | ✅ 零真实凭据（命中的 3 处均为"凭据被 gitignore 排除"的文档说明文字） |
| release artifacts 目录 | ✅ 仅 3 文件（exe + 2 文档），无敏感文件 |
| 3 个凭据文件 | ✅ 均未跟踪，被 `.gitignore` 目录规则保护 |

**结论**：无敏感材料进入 Git 历史或 release 目录。

---

## §2 Git Integrity Audit ✅

```
git fsck --full  →  exit=0（无损坏）
```

6 个 dangling 对象（均无害）：
- `9b4c2eb` 旧 tag（被 `--force` 替换的前一版 v0.9.0-rc1）
- 其余为临时 commit/blob（实验残留）

**结论**：仓库完整性良好，无损坏对象。

---

## §3 External Checkout Simulation ⚠️（发现 1 项 provenance 缺口）

### 验证结果

| 校验项 | 结果 |
|--------|------|
| clone → checkout v0.9.0-rc1 | ✅ 成功，HEAD = `bbbfab7` |
| certificate 存在 | ✅ `RC-RELEASE-CERTIFICATE.md` |
| manifest 存在 | ✅ `RC-BINARY-MANIFEST.md` + `RC-ARTIFACT-INVENTORY.md` |
| clone 中二进制 | ✅ 无（`.exe` 被 gitignore，source-only 预期） |
| **重建二进制哈希核对** | ❌ **不一致（`90f86e9a…` vs 清单 `17e5ce81…`）** |

### 关键发现：tag 改变了二进制可复现性（provenance gap）

| 二进制 | vcs.revision | mod 伪版本 |
|--------|-------------|-----------|
| 官方 `node-v0.9.0-rc1.exe` | `3e6d44a…` | `v0.0.0-20261002015259-3e6d44a24801` |
| clone 重建 | `bbbfab7…` | `v0.9.0-rc1` |

**根因**：官方二进制构建于 `3e6d44a`（当时 tag 尚未创建），Go 的 VCS 版本推导得到伪版本 `v0.0.0-...+3e6d44a`。**打 tag 后**，从 clone checkout tag 重建时，Go 推导出 `v0.9.0-rc1`，且 `vcs.revision` 变为 tag 指向的 `bbbfab7`。tag 的存在本身改变了二进制内嵌元数据 → 哈希不可复现。

### 影响评估

| 维度 | 结论 |
|------|------|
| 业务代码一致性 | ✅ **零差异**（`3e6d44a`..`bbbfab7` 之间无任何 `.go` 改动，仅 2 个文档提交） |
| 功能等价性 | ✅ 官方二进制功能 = tag 代码功能（完全一致） |
| provenance 严格性 | ⚠️ 官方二进制 `vcs.revision=3e6d44a` ≠ tag 指向 `bbbfab7` |

### 判定：**B 类（可解释的二进制差异）**

非 A（完全可复现）：因 tag 改变 VCS 元数据，哈希无法跨"打 tag 前后"复现。
非 C（无解缺口）：根因明确（tag 影响 mod 版本推导），且业务代码零差异。

---

## §4 发布就绪性结论

| 判定维度 | 结果 |
|----------|------|
| 远端状态 | ⚠️ gitea SSH 不可达、github 空仓库（推前需修复） |
| 敏感材料 | ✅ 无泄露 |
| git 完整性 | ✅ 无损坏 |
| 外部 checkout | ✅ 文档齐全，二进制可重建但哈希受 tag 影响 |
| **发布就绪性** | ⚠️ **有条件就绪（CONDITIONAL READY）** |

### 发布前的已知限制（必须随发布披露）

1. **二进制哈希与 tag 的 provenance 不一致**：官方 `node-v0.9.0-rc1.exe` 内嵌 `vcs.revision=3e6d44a`，而 tag `v0.9.0-rc1` 指向 `bbbfab7`。业务代码零差异，但严格溯源上存在提交哈希层面的错位。
2. **修复选项**（若需严格一致，需 Owner 授权重新构建）：
   - 方案 A：用 `-buildvcs=false` 重新构建，消除 VCS 元数据差异，使二进制哈希独立于 tag 状态。
   - 方案 B：接受当前状态（业务代码一致），在发布说明中明确标注"二进制构建于源码边界 3e6d44a"。
3. **远端未就绪**：gitea SSH 不可达需先修复；github 空仓库需首次推送。

---

## HARD STOP

**本阶段终止，等待发布授权。** 已执行：远端状态审计、敏感扫描、git fsck、外部 checkout 模拟。**未执行、也将不执行**：push、deploy、源码修改、tag 修改、release artifact 修改。

—— PHASE-P2PCHAIN-RC-REMOTE-PUBLICATION-READINESS-1 结束 ——
