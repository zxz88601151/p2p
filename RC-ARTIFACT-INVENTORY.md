# RC-ARTIFACT-INVENTORY.md — Release Candidate 资产清单

> 本清单为 P2PChain `v0.9.0-rc1` Release Candidate 的**完整资产清单**，
> 记录源码、构建产物、溯源信息，作为发布证据链封存的基准文档。

---

## 1. 源码提交（Source Commit）

| 项 | 值 |
|----|----|
| HEAD 完整 SHA | `3e6d44a248013f814859cf4f5be6feb5bc57f492` |
| HEAD 短 SHA | `3e6d44a` |
| tree 哈希 | `d5065ee7714bea533674b6d723d7ec11187e47f9` |
| 分支 | `main` |
| 提交信息 | `docs: synchronize release governance status metadata` |
| 提交时间 | 2026-10-02 09:52:59 +0800 |

### RC 三连提交（main 分支）

| 顺序 | SHA | 说明 |
|------|-----|------|
| 1 | `36f9630` | feat(node): orphan durability §4-B1/B1.5/B2 implementation |
| 2 | `8410036` | docs: synchronize documentation truth |
| 3 | `3e6d44a` | docs: synchronize release governance status metadata |

---

## 2. 二进制产物（Binary Artifact）

### 官方 RC 二进制

| 项 | 值 |
|----|----|
| 文件名 | `node-v0.9.0-rc1.exe` |
| 大小 | 12,112,896 字节 |
| SHA-256 | `17e5ce8180f66526ed06826bbe84c74949e8f01b114f2d1895aa2964bfeb0edf` |
| 版本标签 | `v0.9.0-rc1` |

### 历史开发工件（保留，非 RC）

| 文件名 | SHA-256 | 状态 |
|--------|---------|------|
| `node.exe` | `fb3d2b57dd8adbd02d3c6d501f42a2cfa3c076657694e35756967caf9307b717` | 开发工件，溯源缺口，仅本地测试 |

---

## 3. 构建参数（Build Parameters，冻结）

| 参数 | 值 |
|------|----|
| 源码提交 | `3e6d44a248013f814859cf4f5be6feb5bc57f492` |
| tree 哈希 | `d5065ee7714bea533674b6d723d7ec11187e47f9` |
| Go 版本 | `go1.27.0 windows/amd64` |
| OS / 架构 | `windows / amd64` |
| 构建命令 | `go build -trimpath -o node-rc.exe ./cmd/node` |
| 构建标志 | `-trimpath` |

---

## 4. Provenance 溯源信息

| 字段 | 值 |
|------|----|
| `vcs` | `git` |
| `vcs.revision` | `3e6d44a248013f814859cf4f5be6feb5bc57f492` |
| `vcs.time` | `2026-10-02T01:52:59Z` |
| `vcs.modified` | `false`（干净工作树） |
| `mod` 伪版本 | `v0.0.0-20261002015259-3e6d44a24801`（无 `+dirty`） |

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
| vcs.revision 匹配 `3e6d44a` | ✅ |
| vcs.modified = false | ✅ |
| 可复现重建哈希 | ✅ 独立重跑 `17e5ce81…` 完全一致 |
| 历史二进制保留 | ✅ node.exe 未删除 |

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
