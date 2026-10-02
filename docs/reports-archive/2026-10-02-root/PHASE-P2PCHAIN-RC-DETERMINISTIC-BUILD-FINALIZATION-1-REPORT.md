# PHASE-P2PCHAIN-RC-DETERMINISTIC-BUILD-FINALIZATION-1 — 确定性构建最终化报告

> **阶段**：确定性 RC 构建模型最终化（`-trimpath -buildvcs=false`，external attestation）
> **日期**：2026-10-02
> **治理指令**：解决 `-buildvcs=true` 下 binary hash ↔ commit hash ↔ manifest SHA256 的自引用循环。

---

## §0 HARD BASELINE

| 项 | 值 |
|----|----|
| HEAD（起始） | `a69d24ef03c85ab6e669b81a4132749178de76d5` |
| tag target（起始） | `2fe8582`（落后 HEAD 一个提交） |
| 当前官方 binary | `300fe7bb…`（vcs.revision=2fe8582，已不一致） |
| 历史 binary | `node.exe`（fb3d2b57）、`historical-3e6d44a`（17e5ce81） |

---

## §1 从 tag checkout 构建最终 binary ✅

| 项 | 值 |
|----|----|
| 构建命令 | `go build -buildvcs=false -trimpath -o node-det-final.exe ./cmd/node` |
| VCS 内嵌 | ❌ 无（`vcs.revision` 计数 = 0） |
| SHA-256 | `71097357531876b9975998d1d316f0888543a4aea1dcb9456c160df50e6352b1` |
| 大小 | 12,112,384 字节 |

---

## §2 两次独立 rebuild 验证 ✅（实际三次）

| 构建来源 | SHA-256 |
|----------|---------|
| worktree 首次构建 | `71097357…` |
| worktree rebuild #1 | `71097357…` |
| 独立 clone checkout tag rebuild #2 | `71097357…` |

**结论**：三次独立构建 SHA256 **完全一致**，确定性可复现达成。

---

## §3 更新文档为 External Attestation 模型 ✅

| 文档 | 变更 |
|------|------|
| `RC-BINARY-MANIFEST.md` | 重写为 external attestation 模型，记录 `-buildvcs=false` 决策理由 |
| `RC-ARTIFACT-INVENTORY.md` | 更新 SHA256、tag target、构建参数、provenance 模型 |
| `RC-RELEASE-CERTIFICATE.md` | 更新为 external attestation，SHA256=71097357… |

**Provenance 模型**（从 VCS 内嵌 → 外部证明）：

```
tag v0.9.0-rc1 (2647ba0)
  └─ 源码提交 2647ba0（业务代码 = 3e6d44a，零差异）
       └─ go build -buildvcs=false -trimpath
            └─ node-v0.9.0-rc1.exe (SHA256 = 71097357…)
```

---

## §4 Final deterministic build commit ✅

新提交 `2647ba0` — `docs(release): adopt deterministic build model (external attestation)`（6 文档，144 增/126 删）

---

## §5 最终 clone checkout 验证 ✅（关键闭环）

| 验证项 | 结果 |
|--------|------|
| tag 更新指向 `2647ba0` | ✅ tag target == HEAD == 2647ba0 |
| 最终 clone checkout + 重建 SHA256 | ✅ `71097357…`（**未再变化**） |
| 自引用循环 | ✅ **已根除**（tag/文档更新后二进制 SHA256 保持稳定） |

**核心成果**：`-buildvcs=false` 让二进制身份只依赖业务代码，与文档/tag/revision 完全解耦。即使后续 tag 更新、文档更新，二进制 SHA256 依然稳定在 `71097357…`，从根本上消除了循环。

---

## 最终收敛状态

| 项 | 值 |
|----|----|
| HEAD | `2647ba0eb23a94a11aa71326e9b0686bbe1b78d8` |
| tag `v0.9.0-rc1` | `2647ba0eb23a94a11aa71326e9b0686bbe1b78d8`（annotated） |
| 官方 binary SHA256 | `71097357531876b9975998d1d316f0888543a4aea1dcb9456c160df50e6352b1` |
| 构建模型 | `-buildvcs=false -trimpath`（external attestation） |
| 业务源码边界 | `3e6d44a`（零差异） |
| 历史 binary | 全部保留（node.exe + historical-3e6d44a） |

### RC 最终提交链（main 分支）

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

## HARD STOP

**本阶段终止，不 push。** 已执行：确定性构建、可复现验证、文档更新、提交、tag 更新。**未执行、也将不执行**：push、deploy、源码修改、历史 binary 删除。

**RC 现已达到：Deterministically Reproducible Release Candidate** —— tag → commit → source → binary 全链路一致，且二进制身份确定性可复现，无自引用循环。

—— PHASE-P2PCHAIN-RC-DETERMINISTIC-BUILD-FINALIZATION-1 结束 ——
