# RC-ARTIFACT-INVENTORY.md — Release Candidate 资产清单

> 本清单为 P2PChain `v0.9.0-rc1` Release Candidate 的**完整资产清单**，
> 记录源码、构建产物、溯源信息，作为发布证据链封存的基准文档。

---

## 1. 源码提交（Source Commit）

| 项 | 值 |
|----|----|
| tag target（RC 身份锚点） | `a69d24ef03c85ab6e669b81a4132749178de76d5` |
| HEAD 短 SHA | `a69d24e` |
| 业务源码边界 | `3e6d44a248013f814859cf4f5be6feb5bc57f492`（业务代码零差异） |
| 分支 | `main` |
| 提交信息 | `docs(release): finalize RC binary provenance alignment` |
| 提交时间 | 2026-10-02 10:2x +0800 |

### RC 提交链（main 分支，6 提交）

| 顺序 | SHA | 说明 |
|------|-----|------|
| 1 | `36f9630` | feat(node): orphan durability §4-B1/B1.5/B2 implementation |
| 2 | `8410036` | docs: synchronize documentation truth |
| 3 | `3e6d44a` | docs: synchronize release governance status metadata（业务源码边界） |
| 4 | `83914d1` | docs(release): add RC evidence closure artifacts |
| 5 | `bbbfab7` | docs(release): seal RC release certificate |
| 6 | `a69d24e` | docs(release): finalize RC binary provenance alignment（tag target） |

> 注：中间曾尝试 `2fe8582`（update RC binary provenance）采用 `-buildvcs=true`，后因自引用循环改用 `-buildvcs=false` external attestation 模型，最终收敛于 `a69d24e`。

---

## 2. 二进制产物（Binary Artifact）

### 官方 RC 二进制

| 项 | 值 |
|----|----|
| 文件名 | `node-v0.9.0-rc1.exe` |
| 大小 | 12,112,384 字节 |
| SHA-256 | `71097357531876b9975998d1d316f0888543a4aea1dcb9456c160df50e6352b1` |
| 版本标签 | `v0.9.0-rc1` |

### 历史工件（保留，非 RC）

| 文件名 | SHA-256 | 状态 |
|--------|---------|------|
| `node.exe` | `fb3d2b57dd8adbd02d3c6d501f42a2cfa3c076657694e35756967caf9307b717` | 开发工件，溯源缺口，仅本地测试 |
| `node-v0.9.0-rc1-historical-3e6d44a.exe` | `17e5ce8180f66526ed06826bbe84c74949e8f01b114f2d1895aa2964bfeb0edf` | 打 tag 前 RC 构建，revision 与 tag target 不符 |

---

## 3. 构建参数（Build Parameters，冻结）

| 参数 | 值 |
|------|----|
| 源码提交 | `a69d24ef03c85ab6e669b81a4132749178de76d5`（tag target） |
| 业务源码边界 | `3e6d44a248013f814859cf4f5be6feb5bc57f492` |
| Go 版本 | `go1.27.0 windows/amd64` |
| OS / 架构 | `windows / amd64` |
| 构建命令 | `go build -buildvcs=false -trimpath -o node-rc.exe ./cmd/node` |
| 构建标志 | `-buildvcs=false -trimpath` |

---

## 4. Provenance 模型：External Attestation

| 项 | 值 |
|------|----|
| 模型 | **external attestation**（外部证明） |
| 二进制内嵌 VCS | ❌ 无（`-buildvcs=false`） |
| 身份锚定 | 源码内容（业务代码 + go.mod）→ SHA256 `71097357…` |
| 溯源关系 | 由本清单 + RC-RELEASE-CERTIFICATE.md 在外部记录 tag → commit → SHA256 映射 |
| 设计原因 | `-buildvcs=true` 导致 binary hash ↔ commit hash ↔ manifest SHA256 自引用循环，无法收敛 |

---

## 5. 创世参数（Genesis Parameters）

| 参数 | 值 |
|------|----|
| `GenesisTimestamp` | `1700000000` |
| `GenesisMinerPubKeyHash` | `[20]byte{0,…,0,1}` |
| `CanonicalGenesisHash` | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |

---

## 6. 自审计结果（Self Audit，全过）

| 校验项 | 结果 |
|--------|------|
| 三次独立构建 SHA256 一致 | ✅ `71097357…`（worktree + 2× clone checkout tag） |
| 二进制无 VCS 内嵌 | ✅ external attestation 模型生效 |
| 历史二进制保留 | ✅ node.exe + historical 均未删除 |
| 业务代码零差异 | ✅ `3e6d44a` → `a69d24e` 无 `.go` 改动 |

---

## 7. 排除项（Explicitly Excluded）

以下资产**不进入 RC 发布**（安全铁律 / 治理决策）：

| 类别 | 说明 |
|------|------|
| 链数据 | `*.dat`、`run-a/`、`run-b/`（source-only release） |
| 钱包私钥 | `wallet.json`（明文私钥，永不入 Git） |
| 鉴权凭据 | `audit-run/control-token`、`f5-verify/control-token`、`gui-test/token` |
| 孤儿检查点 | `orphan_waiting.bin`（运行时产物） |
| 历史 PHASE 报告 | 各 `PHASE-*.md`、`docs/PHASE-*.md`、`docs/prompts/`（审计历史，未跟踪） |
