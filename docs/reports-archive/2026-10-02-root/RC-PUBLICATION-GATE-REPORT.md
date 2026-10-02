# RC-PUBLICATION-GATE-REPORT.md — 最终 RC 发布门禁报告

> **阶段**：PHASE-P2PCHAIN-RC-PUBLICATION-AUTHORIZATION-1（发布前最终冻结审计）
> **日期**：2026-10-02
> **治理指令**：不 push、不 deploy、不修改 tag/binary/业务代码，仅完成发布前冻结审计与 dry run。

---

## §0 FINAL-RC-SNAPSHOT.md ✅

已创建最终冻结快照，记录完整冻结状态：

| 项 | 值 |
|----|----|
| tag `v0.9.0-rc1` | `2647ba0eb23a94a11aa71326e9b0686bbe1b78d8`（annotated） |
| 二进制 SHA256 | `71097357531876b9975998d1d316f0888543a4aea1dcb9456c160df50e6352b1` |
| 构建标志 | `-buildvcs=false -trimpath` |
| 可复现性 | 四次独立构建一致 `71097357…` |

---

## §1 最终 Release Package 验证 ✅（全过）

| 校验项 | 结果 |
|--------|------|
| 四件套齐全（binary + manifest + inventory + certificate） | ✅ 全部存在 |
| 二进制 hash 一致（根目录 vs release 目录） | ✅ 均 `71097357…` |
| 文档内容一致（根目录 vs release 副本） | ✅ 3 对全部一致 |
| SHA256 统一 | ✅ 3 个文档均记录 `71097357…` |

**Release package 布局**：
```
release/v0.9.0-rc1/
├── node-v0.9.0-rc1.exe      (SHA256 = 71097357…)
├── BINARY-MANIFEST.md
├── ARTIFACT-INVENTORY.md
├── RELEASE-CERTIFICATE.md
└── SHA256SUMS               (新增，供用户校验)
```

---

## §2 Publication Dry Run ✅（模拟通过）

| 模拟项 | 结果 |
|--------|------|
| Release 页面内容 | ✅ 含下载表、SHA256、复现命令、验证清单 |
| checksum 发布 | ✅ `SHA256SUMS` 生成正确 |
| 用户验证流程 | ✅ 下载 → sha256sum → 比对 → **校验通过** |

**模拟的发布流程**（未来真实发布时复用）：
1. 上传 `node-v0.9.0-rc1.exe` + `SHA256SUMS`
2. 发布页面展示 SHA256 与复现命令
3. 用户下载后 `sha256sum` 比对，或从源码复现构建验证

---

## §3 最终门禁决策（FINAL GATE DECISION）

| 判定维度 | 结果 |
|----------|------|
| tag 冻结 | ✅ `2647ba0`（annotated，不可变） |
| 二进制冻结 | ✅ SHA256 `71097357…` |
| 可复现性 | ✅ 四次独立构建一致（Deterministically Reproducible） |
| release package | ✅ 四件套 + SHA256SUMS 齐全 |
| provenance | ✅ external attestation 模型，无自引用循环 |
| 业务代码 | ✅ 零变更（边界 `3e6d44a`） |
| 敏感材料 | ✅ 无泄露 |
| **门禁结论** | ✅ **通过（GREEN）—— 发布就绪** |

---

## 发布授权状态

**当前处于"等待发布授权"状态。** 一切发布资产已冻结并验证完毕，但**尚未执行任何 push / deploy 动作**。

### 未来发布时需执行（待 Owner 明确授权）

| 动作 | 状态 |
|------|------|
| push 提交 + tag 到远端 | ⏸ 未执行（等待授权） |
| 部署 release 包 | ⏸ 未执行 |
| 发布 Release 页面 | ⏸ 未执行 |

### 发布前仍需注意

1. **远端未就绪**：gitea SSH 不可达（需先修复）、github 空仓库（需首次推送）。
2. **首次推送注意事项**：github 为全新仓库，需 `git push -u origin main` + `git push origin v0.9.0-rc1`（tag 需单独推送）。

---

## HARD STOP

**本阶段终止，等待发布授权。** 已执行：创建冻结快照、验证 release package、publication dry run。**未执行、也将不执行**：push、deploy、tag 修改、binary 修改、业务代码修改。

**RC 状态：Deterministically Reproducible Release Candidate，发布门禁 GREEN，等待 Owner 发布授权。**

—— PHASE-P2PCHAIN-RC-PUBLICATION-AUTHORIZATION-1 结束 ——
