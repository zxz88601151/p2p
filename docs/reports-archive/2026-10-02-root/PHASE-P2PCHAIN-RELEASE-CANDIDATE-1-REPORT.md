# PHASE-P2PCHAIN-RELEASE-CANDIDATE-1-REPORT

> **Phase**：PHASE-P2PCHAIN-RELEASE-CANDIDATE-1 — 发布候选准备
> **Owner authorization**：PHASE P2PCHAIN — RELEASE-CANDIDATE-1
> **Date**：2026-10-02
> **Scope**：Release candidate preparation only · No new features · No consensus changes · No protocol changes · No cloud deployment
> **HARD STOP（强制）**：No tag / deployment until Owner review.

---

## 0. 结论速览

发布候选准备完成：孤儿耐久实现与文档真相同步已**分两个语义化提交**入库；从干净 checkout 构建成功；四项强制验证套件**全部通过**。发布基线 = `8410036`。**未打 tag、未部署**（待 Owner review）。

---

## 1. Release baseline

| 项 | 值 |
|---|---|
| **Release baseline hash** | `8410036ed7b610ac6e03d48dce5abdf20ef33643` |
| 前序基线 | `52fb464` → `36f9630` → `8410036` |
| 分支 | `main` |
| 提交 1（孤儿代码） | `36f9630` — `feat(node): orphan durability §4-B1/B1.5/B2 implementation`（9 文件，+1798） |
| 提交 2（文档同步） | `8410036` — `docs: synchronize documentation truth (CONSOLIDATION-2 + CLEANUP-1)`（6 文件，+1065/-27） |

---

## 2. Binary identity

| 二进制 | SHA-256 | 来源 |
|---|---|---|
| 干净 checkout 构建 `node.exe` | `0412640a74b6d32d738a20d92735f471e835b1f20d7545c0ff4b3384583db33b` | `git clone` + `go build -o node.exe ./cmd/node`（BUILD EXIT=0） |
| 工作树旧构建 `node.exe`（对照） | `fb3d2b57…`（前序工作树构建，含孤儿代码，但非本 RC 干净产物） | — |

> **身份裁定**：发布候选二进制 = `0412640a…`（从 `8410036` 干净 checkout 构建）。此前 `fb3d2b57…` 是工作树构建产物，身份已由干净重建取代。

---

## 3. Validation matrix（强制套件，均对干净 checkout 二进制重跑）

| 套件 | 验证内容 | 结果 |
|---|---|---|
| **ORPHAN-DURABILITY-E2E-RECOVERY-1** | 孤儿耐久端到端恢复全链路（孤儿创建→检查点落盘→重启→恢复加载→握手消费→分支请求→父恢复→孤儿消解→链一致） | ✅ **PASS**（`TestE2E_OrphanDurabilityRecoveryLifecycle`，0.26s；完整 `ORPHAN_RESTORE_DRAINED` 生命周期） |
| **MULTI-NODE-CANONICAL-SYNC-VALIDATION-1** | 3 节点握手+批量同步+canonical 收敛 | ✅ **PASS**（A/B/C 全 height=12、tip=`0000a22161…` 一致） |
| **TIE-BREAK-CONSENSUS-VALIDATION-1** | 等功分叉确定性 tie-break（较大 tip hash 胜） | ✅ **PASS**（O 选 X=`0000bd5a…`，较大 hash 胜者） |
| **NODE-RESTART-CONSISTENCY-VALIDATION-1** | 停 B→A/C 推进→重启 B→补齐 14 块→收敛 | ✅ **PASS**（A/B/C 全 height=22 收敛，`应用 14 个区块，缺父待补 0`） |
| **孤儿耐久+恢复测试族（补充）** | E2E + B1.5(21) + B2(7) + checkpoint/hook/restore/durability | ✅ **PASS**（`ok p2pchain/cmd/node 11.517s`） |

**结论**：四项强制套件 + 孤儿测试族，从干净 checkout 全部通过。

---

## 4. 提交内容核对（scope 纪律）

### 4.1 已提交（Owner 授权范围）

| 提交 | 文件 | 说明 |
|---|---|---|
| `36f9630` | `cmd/node/orphan_checkpoint.go` + 7 个 orphan*_test.go + `main.go` + `service.go` | 孤儿耐久实现（§4-B1 数据层 + §4-B1.5 激活 + §4-B2 启动恢复） |
| `8410036` | `README.md`、`PROJECT-AI-CONTEXT.md`、`docs/DETERMINISTIC-SERIALIZATION-SPEC.md`、`docs/MASTER-DESIGN.md`、`docs/spec/ORPHAN-DURABILITY-SPEC-v1.md`、`docs/spec/STARTUP-RECOVERY-PLAN-1.md` | 文档真相同步 |

### 4.2 未提交（有意排除，Owner 决策）

| 类别 | 数量 | 排除原因 |
|---|---|---|
| 历史 `PHASE-*` 阶段报告（根级 + docs/） | ~280 | Owner 选择「仅孤儿代码+文档真相同步」，历史报告不在范围 |
| 本会话验证/审计报告（CONSOLIDATION/CLEANUP/运行时/多节点/tie-break/重启/FINAL-AUDIT） | 8 | Owner 未选择「全部验证报告」选项 |
| `internal/blockchain/query.go` | 1（tracked-modified） | CRLF 行尾噪声（`git diff -w` 为空），非实质改动 |
| 治理屏蔽路径（`audit-run/`、`run-a/`、`run-b/`、`gui/`、`f*-verify/` 等） | — | `.gitignore` 屏蔽，含明文私钥/凭据 |
| 3 个凭据文件（`audit-run/control-token`、`f5-verify/control-token`、`gui-test/token`） | 3 | F-6.1 审计铁律「永不 `git add -A`」 |

> **凭据防护确认**：两次提交均用**显式路径白名单** `git add <精确文件>`，`git diff --cached --name-only` 逐次校验，**零凭据文件进入暂存区**（SECRET CHECK = OK）。

---

## 5. Remaining unproven properties（残留未证明项）

| 领域 | 未证明项 | 说明 |
|---|---|---|
| 共识 | v2/v3 规则集运行时行为 | 链高 < 2000，未达激活高度；Ceil/Nearest/注入 27 仅源码冻结 |
| 共识 | 难度浮动真实发生 | 无链越过 2000，`AdjustBits` 浮动路径未生产触发 |
| 持久化 | 孤儿块体恢复 | 仅持久化父键；块体依赖网络 re-gossip，若不再被持有则永久丢失 |
| 持久化 | 真实进程 DirLock 重启耦合 | E2E 用进程内同 datadir 模拟，未走 `newNodeRuntime` DirLock |
| 网络 | sync/branch 并发交付隔离 | 未隔离单路送达 |
| 安全 | 威胁模型/攻击矩阵 | `SECURITY-BOUNDARY.md`/`LIMITATIONS.md` 仍 SKELETON |

---

## 6. Release readiness assessment（发布就绪评估）

### 6.1 相对 FINAL-PROTOCOL-AUDIT-1 的进展

| 前次 P0/P1 项 | 本次状态 |
|---|---|
| **P0：孤儿代码未提交（8 文件 untracked，二进制身份不确定）** | ✅ **已收口**——孤儿代码已提交 `36f9630`，二进制身份由干净 checkout 重建确定为 `0412640a…` |
| P1：孤儿块体不持久化 | ⚠️ 仍存在（容量边界，非本阶段范围） |
| P1：v2/v3 规则集未运行时验证 | ⚠️ 仍存在（需链高越过 2000/3000） |
| P2：安全边界 SKELETON | ⚠️ 仍存在 |

### 6.2 结论

**「提交边界已收口、协议正确性已重新验证」**，但仍有 3 项未覆盖（孤儿块体恢复、v2/v3 运行时、安全边界）。综合判定：

| 维度 | 状态 |
|---|---|
| 提交边界收口 | ✅ 完成（孤儿代码 + 文档同步已入库） |
| 二进制身份确定性 | ✅ 完成（干净 checkout 构建 `0412640a…`） |
| 强制验证套件 | ✅ 4/4 通过 |
| 发布就绪（生产级） | ⛔ **仍不推荐生产发布**——安全边界 SKELETON + v2/v3 未运行时验证 + 孤儿块体不持久化 |
| 作为「Developer Node / 学习实验」版本基线 | ✅ **可作为候选**（协议核心机制已验证正确） |

**发布建议（Owner 决策）**：本 RC 可作为「开发者学习实验」的候选基线（提交边界干净、二进制确定、验证通过）；但**不应**宣称生产可用，需在后续补齐安全边界与 v2/v3 运行时验证。

---

## 7. HARD STOP confirmation

- ✅ **已执行**：2 个语义化提交（Owner 明确授权范围：孤儿代码 + 文档真相同步）。
- ❌ **未执行**：任何 `git tag`、`git push`、部署、云修改——**全部待 Owner review**。
- ✅ **凭据防护**：显式路径白名单，零凭据文件提交。
- ✅ **scope 纪律**：仅提交授权范围（孤儿代码 + 文档同步）；历史报告、验证报告、CRLF 噪声、治理屏蔽路径、凭据文件全部排除。

**待 Owner 下一步**：review 本 RC（基线 `8410036`），决定是否打 tag / 部署。在此之前不做任何 tag / push / deploy。
