# PRE-RELEASE-GATE-1 — 最终门禁报告

> **阶段**：Release Candidate 提交前预检门禁（只读审计，禁止任何 git 变更）
> **日期**：2026-10-02
> **治理指令**：Owner 授权「PRE-RELEASE-GATE-1」，§0→§6 顺序执行，报告后 HARD STOP。
> **门禁结论**：**✅ 通过（GREEN）** — 推荐打标 `v0.9.0-rc1`
>
> **推荐标签**：`v0.9.0-rc1` — "Developer Node Release Candidate. Not production cryptocurrency release."

---

## ⚠️ 两项必须如实披露的门禁偏差

在预检中发现两处与「预检门禁」框架假设不符的客观事实，均在 Owner 既有授权范围内，但必须显式记录，不得隐瞒：

| # | 偏差 | 事实 | 处置 |
|---|------|------|------|
| G-1 | **冻结边界已提前提交** | 本阶段定位为「在任何 git commit **之前**冻结 RC 边界」，但 RC 两个提交（孤儿代码 `36f9630` + 文档真相同步 `8410036`）已在**上一阶段 PHASE-P2PCHAIN-RELEASE-CANDIDATE-1** 由 Owner 明确授权完成并落在 `main` 上。 | 本报告作为「事后复核门禁」，对已提交边界做只读核验，结论仍有效；不重复提交。 |
| G-2 | **README 状态声明陈旧** | `README.md:59` 仍写 `HEAD = 52fb464`、正文「尚未提交、未部署」，与实际 `HEAD = 8410036` 且已提交矛盾。 | 归类为 **D（DEFERRED）**，需在后续文档同步阶段修正；不阻塞 RC，但必须记录。 |

---

## §0 硬基线（HARD BASELINE）

| 项 | 值 | 状态 |
|----|----|----|
| HEAD（完整 SHA） | `8410036ed7b610ac6e03d48dce5abdf20ef33643` | ✅ |
| 分支 | `main` | ✅ |
| 暂存区（`git diff --cached`） | **空**（0 文件） | ✅ 无凭据被暂存 |
| 已跟踪未暂存 | 仅 `internal/blockchain/query.go` | ✅ 纯 CRLF 噪声（`--ignore-cr-at-eol` 零差异） |
| 未跟踪文件 | 221 个 | ✅ 全部为历史 PHASE 报告 / prompts / 实验目录，按 Owner 范围明确排除 |
| 基线污染 / 漂移 | 无 | ✅ |

**基线结论**：工作树干净，无凭据污染，无 RC 相关未提交改动。

---

## §1 发布范围分类（RELEASE SCOPE CLASSIFICATION）

| 类别 | 定义 | 文件 |
|------|------|------|
| **A — MUST COMMIT** | RC 核心：孤儿耐久 + 文档真相同步 | 已提交于 `36f9630`（9 文件）+ `8410036`（6 文件），共 15 文件 |
| **B — RELEASE ATTACHMENT ONLY** | 随发布附带的审计/验证报告 | 本报告 + 历史 PHASE 报告（不提交入 Git） |
| **C — NEVER COMMIT** | 凭据/私钥/明文 token | 3 凭据文件 + wallet.json + *.dat/*.lock（已被 `.gitignore` 保护） |
| **D — DEFERRED** | 需后续修正项 | README.md:59 陈旧状态声明（G-2） |

### A 类明细（已提交，冻结边界）

**Commit `36f9630` feat(node): orphan durability §4-B1/B1.5/B2 implementation**（9 文件，+1798 行）
- `cmd/node/main.go`（D1/D2 孤儿生命周期接线）
- `cmd/node/orphan_checkpoint.go`（核心 checkpoint 实现）
- `cmd/node/service.go`（prepareOrphanRestore / consumeRestorePending / MarkDirty / Remove）
- `cmd/node/orphan_checkpoint_test.go`、`orphan_checkpoint_hook_test.go`、`orphan_durability_test.go`、`orphan_durability_e2e_test.go`、`orphan_recovery_4b2_test.go`、`orphan_restore_test.go`（6 个测试）

**Commit `8410036` docs: synchronize documentation truth**（6 文件，+1065/-27）
- `PROJECT-AI-CONTEXT.md`、`README.md`
- `docs/DETERMINISTIC-SERIALIZATION-SPEC.md`、`docs/MASTER-DESIGN.md`
- `docs/spec/ORPHAN-DURABILITY-SPEC-v1.md`、`docs/spec/STARTUP-RECOVERY-PLAN-1.md`

---

## §2 凭据与密钥边界审计（CREDENTIAL BOUNDARY）

| 检查项 | 结果 |
|--------|------|
| 已提交树含凭据文件 | ❌ 无（`git ls-files \| grep -iE 'control-token\|token.local\|wallet.json\|secret\|\.dat$\|\.lock$'` 零命中） |
| 3 凭据文件跟踪状态 | `audit-run/control-token`、`f5-verify/control-token`、`gui-test/token` 均 **not tracked** |
| `.gitignore` 覆盖 | 目录级精确规则（第 37-39 行）保护 3 凭据；`wallet.json`/`*.dat`/`*.lock`/`control-token`/`token`/`token.local` 全覆盖 |
| 明文私钥模式扫描 | `git grep` 匹配到的 `PrivateKey` 均为 `internal/wallet/*.go` 的 ECDSA 序列化/签名逻辑，**无明文私钥字面量** |
| 助记词/seed phrase/BEGIN PRIVATE KEY | 零命中 |

**凭据结论**：✅ 安全。无任何凭据进入 Git 历史，`.gitignore` 覆盖完整，3 凭据文件未被跟踪、未被暂存、未被删除、未被修改。

---

## §3 代码 / 文档一致性检查（TRUTH SYNC）

| 真值项 | 源码 | 文档 | 一致 |
|--------|------|------|------|
| `MaxTargetBits` | `16`（`internal/pow/pow.go:29`） | README/AI-CONTEXT 均 `16` | ✅ |
| `MaxDifficultyBits` | `32`（pow.go:52） | 均 `32` | ✅ |
| `ActivationHeight`（v2） | `2000`（pow.go:67） | 均 `2000` | ✅ |
| `NewRulesetActivationHeight`（v3） | `3000`（pow.go:94） | 均 `3000` | ✅ |
| `NewRulesetInitialBits` | `27`（pow.go:106） | 均 `27` | ✅ |
| `TargetBlockTimeSeconds` | `60` | 均 `60` | ✅ |
| `DifficultyAdjustmentInterval` | `20` | 均 `20` | ✅ |
| 版本三态 1/2/3 | `VersionForHeight` | 均 `<2000=1 / [2000,3000)=2 / ≥3000=3` | ✅ |
| 孤儿 checkpoint（magic ORPH/version 0x01/u32BE/SHA256/≤256 条目） | `orphan_checkpoint.go` | `PROJECT-AI-CONTEXT.md:314` | ✅ |
| 启动恢复（prepareOrphanRestore + consumeRestorePending） | `service.go:1049/1084` | `STARTUP-RECOVERY-PLAN-1.md` | ✅ |
| 关机落盘 D2（FlushIfDirty） | `main.go:283` | spec | ✅ |
| 分叉选择（CompareWork + 等功 tie-break 取较大 tip hash） | `internal/blocktree/settip.go` | README | ✅ |

**唯一不一致项**：README.md:59 的 `HEAD = 52fb464` + 「尚未提交」声明（归类 D，见 G-2）。

---

## §4 测试就绪审查（VALIDATION MATRIX）

| 验证项 | 证据 | 状态 |
|--------|------|------|
| 孤儿/恢复专项测试 | `go test ./cmd/node/ -run 'Orphan\|orphan\|Restore\|Checkpoint'` → **ok 15.6s** | ✅ PROVEN |
| PoW 共识包 | `go test ./internal/pow/` → **ok 5.8s** | ✅ PROVEN |
| 分叉选择包 | `go test ./internal/blocktree/` → **ok 0.2s** | ✅ PROVEN |
| 区块链核心包 | `go test ./internal/blockchain/` → **ok 84.2s** | ✅ PROVEN |
| gofmt | 孤儿实现 3 文件 → 零差异 | ✅ PROVEN |
| go vet | `./cmd/node/` → 无告警 | ✅ PROVEN |
| 多节点真实同步 / 分叉收敛 / 等功 tie-break / 重启一致性 | 前序 4 阶段报告（MULTI-NODE / TIE-BREAK / NODE-RESTART / FINAL-PROTOCOL-AUDIT）全部 PASS | ✅ PROVEN |
| 生产级加密货币就绪性 | 未在范围内 | ⚠️ NOT IN SCOPE |

**就绪性结论**：Developer Node RC 就绪（GREEN）。**不宣称生产加密货币就绪**——这是 Developer Node Release Candidate，非生产加密货币发布。

---

## §5 提交计划（提案，未执行）

> 以下仅为提案。RC 边界已在上一阶段提交，本阶段**不重复提交**；D 类修正（README 状态声明）留待后续文档同步阶段。

| Commit | 类型 | 范围 | 状态 |
|--------|------|------|------|
| 1 | `feat(consensus): add orphan durability and startup recovery` | `36f9630` 9 文件 | ✅ 已提交（历史） |
| 2 | `docs: synchronize protocol truth and specifications` | `8410036` 6 文件 | ✅ 已提交（历史） |
| 3（可选） | `docs(audit): add release audit index` | 发布审计索引 | 🔲 未执行（Owner 决定） |

**D 类修正提案（后续阶段）**：更新 README.md:59 为 `HEAD = 8410036` + 「已提交」事实。

---

## §6 最终门禁决策（FINAL GATE DECISION）

| 判定维度 | 结果 |
|----------|------|
| 基线干净 | ✅ |
| 范围分类完整 | ✅ |
| 凭据安全 | ✅ |
| 代码/文档一致 | ✅（1 项 D 类陈旧声明除外） |
| 测试就绪 | ✅（全 PROVEN，生产就绪不宣称） |
| **门禁结论** | **✅ 通过（GREEN）** |
| **推荐标签** | **`v0.9.0-rc1`** |

**标签说明文案**（打标时使用）：
> `v0.9.0-rc1` — "Developer Node Release Candidate. Not production cryptocurrency release."

---

## HARD STOP

**报告已生成。本阶段终止。** 未执行、也将不执行：`git add` / `git commit` / `git tag` / `git push` / `deploy`。

**等待 Owner 明确授权后**，方可执行：
1. （可选）修正 README.md:59 陈旧声明（D 类）
2. （可选）打标 `v0.9.0-rc1`（附上述文案）

—— PRE-RELEASE-GATE-1 结束 ——
