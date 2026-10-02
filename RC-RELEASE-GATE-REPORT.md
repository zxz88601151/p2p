# RC-RELEASE-GATE-REPORT.md — RC 发布门禁报告

> **阶段**：PHASE-P2PCHAIN-RC-RELEASE-EVIDENCE-CLOSURE-1（RC 发布证据链封存）
> **日期**：2026-10-02
> **治理指令**：完成 RC 证据链封存，不修改业务代码/协议/共识逻辑，不删除历史二进制，不打 tag。

---

## §0 HARD BASELINE

| 项 | 值 |
|----|----|
| git HEAD | `3e6d44a248013f814859cf4f5be6feb5bc57f492` |
| 分支 | `main` |
| tree 哈希 | `d5065ee7714bea533674b6d723d7ec11187e47f9` |
| status | 仅 `internal/blockchain/query.go`（CRLF 噪声，无实质改动） |
| RC 二进制 SHA256 | `17e5ce8180f66526ed06826bbe84c74949e8f01b114f2d1895aa2964bfeb0edf` |
| 历史 node.exe SHA256 | `fb3d2b57dd8adbd02d3c6d501f42a2cfa3c076657694e35756967caf9307b717`（保留） |

---

## §1 RC-ARTIFACT-INVENTORY.md ✅

已产出，包含：
- **源码提交**：`3e6d44a`（完整 SHA + tree 哈希 + 三连提交链）
- **二进制产物**：官方 `node-v0.9.0-rc1.exe`（SHA256）+ 历史 `node.exe`（隔离说明）
- **构建参数**：冻结（`3e6d44a` + `-trimpath` + Go1.27.0 windows/amd64）
- **Provenance**：`vcs.revision=3e6d44a` + `vcs.modified=false` + mod 伪版本
- **创世参数**：`GenesisTimestamp` / `CanonicalGenesisHash`
- **排除项**：链数据 / 钱包 / 凭据 / 检查点 / 历史报告

---

## §2 RC Binary Self Audit ✅（全过）

| 校验项 | 结果 |
|--------|------|
| vcs.revision | ✅ `3e6d44a248013f814859cf4f5be6feb5bc57f492` 精确匹配 |
| vcs.modified | ✅ `false`（干净工作树，无 dirty） |
| 可复现重建哈希 | ✅ 独立重跑 `go build -trimpath` 得到**完全一致**的 `17e5ce81…` |

**可复现性结论**：官方 RC 二进制在冻结参数下**确定性可复现**（两次独立构建哈希一致），证明其与源码提交 `3e6d44a` 的绑定是可靠、可验证的。

---

## §3 Release Package Layout ✅

```
release/v0.9.0-rc1/
├── node-v0.9.0-rc1.exe      # 官方 RC 二进制（SHA256 = 17e5ce81…）
├── BINARY-MANIFEST.md       # 二进制清单（构建参数 + VCS 溯源 + 复现命令）
└── ARTIFACT-INVENTORY.md    # 资产清单（源码 + 二进制 + 溯源 + 排除项）
```

---

## §4 最终门禁决策（FINAL GATE）

| 判定维度 | 结果 |
|----------|------|
| 源码边界 | ✅ `3e6d44a`，零业务代码/协议/共识改动 |
| 二进制溯源 | ✅ VCS 干净（`3e6d44a` + `modified=false`） |
| 可复现性 | ✅ 确定性重建哈希一致 |
| 历史二进制保留 | ✅ `node.exe` 未删除 |
| 资产清单 | ✅ `RC-ARTIFACT-INVENTORY.md` 完整 |
| 发布包布局 | ✅ `release/v0.9.0-rc1/` 三件套齐全 |
| **门禁结论** | **✅ 通过（GREEN）** |

### 建议版本标签

`v0.9.0-rc1` — "Developer Node Release Candidate. Not production cryptocurrency release."

---

## HARD STOP

**本阶段终止，等待 Owner 授权。** 已完成的封存产物：

| 文件 | 状态 |
|------|------|
| `RC-ARTIFACT-INVENTORY.md` | 未跟踪（未 git add） |
| `RC-BINARY-MANIFEST.md` | 未跟踪 |
| `release/v0.9.0-rc1/` 三件套 | 未跟踪 |
| `node-v0.9.0-rc1.exe` | 未跟踪 |

**未执行、也将不执行**：`git tag` / `git push` / `deploy` / 业务代码改动 / 历史二进制删除。

等待 Owner 明确授权后，方可执行：打标 `v0.9.0-rc1` 等后续动作。

—— PHASE-P2PCHAIN-RC-RELEASE-EVIDENCE-CLOSURE-1 结束 ——
