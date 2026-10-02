# CONSOLIDATION-2-REPORT

> **Phase**：CONSOLIDATION-2 — Repository truth synchronization after completed orphan durability E2E validation
> **Owner authorization**：PHASE P2PCHAIN — CONSOLIDATION-2
> **Date**：2026-10-02
> **Scope**：No feature development · No consensus changes · No network protocol changes · No wallet changes
> **HARD STOP（强制）**：No commit · No deployment · No cloud modification · No production changes

---

## §0 HARD BASELINE

### §0.1 Current HEAD & branch

| 项 | 值 |
|---|---|
| `git rev-parse HEAD` | `52fb464af243fbcdfd78bab846b45ea090e918fb` |
| `git branch --show-current` | `main` |
| Binary（上次构建，仅参考） | `node.exe` 2026-10-02 00:27，哈希 `ade62e87…` |

### §0.2 B1 / B1.5 / B2 / E2E implementation boundary（关键区分）

| 能力 | 提交状态 | 说明 |
|---|---|---|
| §4-B1 数据层 `orphan_checkpoint.go` | ⛔ **未提交**（untracked） | 已实现，`newOrphanCheckpoint` + 原子写 + fail-closed load |
| §4-B1.5 生产激活（D1+D2） | ⛔ **未提交**（modified `main.go`/`service.go`） | D1 初始化 + `FlushIfDirty` 周期落盘 |
| §4-B2 启动恢复接线 | ⛔ **未提交**（modified `service.go`） | `prepareOrphanRestore` + `consumeRestorePending` |
| ORPHAN-DURABILITY-E2E-RECOVERY-1 | ⛔ **未提交**（untracked 测试 + 报告） | 真实网络/磁盘/进程边界全链路，14 项断言全绿 |
| **共识规则集（`internal/pow/pow.go`）** | ✅ **已提交（位于 HEAD 内）** | `ActivationHeight=2000` / `NewRulesetActivationHeight=3000` / `MaxDifficultyBits=32` 等均已在 `git HEAD` 中，**不在本阶段工作树 diff 中** |

> ⚠️ **边界澄清**：本次文档同步里对「共识规则集」的修订（§1）纠正的是**文档相对已提交源码的漂移**——
> `pow.go` 早已提交 v2@2000 / v3@3000 规则，但 `README.md` 与 `PROJECT-AI-CONTEXT.md` §7 仍写着旧的「难度钉死 16」。
> 因此这些文档修订**不涉及任何共识代码改动**，只是让文档追平已提交的真相。这一点与 HARD STOP 完全一致。

### §0.3 Uncommitted changes（基线快照）

- **Modified（tracked）**：`cmd/node/main.go`、`cmd/node/service.go`、`docs/DETERMINISTIC-SERIALIZATION-SPEC.md`、`internal/blockchain/query.go`、`README.md`
- **Untracked（新增）**：`cmd/node/orphan_checkpoint.go`、`cmd/node/orphan_checkpoint_test.go`、`cmd/node/orphan_checkpoint_hook_test.go`、`cmd/node/orphan_durability_test.go`、`cmd/node/orphan_restore_test.go`、`cmd/node/orphan_recovery_4b2_test.go`、`cmd/node/orphan_durability_e2e_test.go`、`docs/PHASE-P2PCHAIN-ORPHAN-DURABILITY-*.md`（含 E2E 报告）、`PROJECT-AI-CONTEXT.md`（本阶段由 `dist/ai-package` 归档复制为仓库根权威副本）、`docs/spec/ORPHAN-DURABILITY-SPEC-v1.md`、`docs/spec/STARTUP-RECOVERY-PLAN-1.md`、`CONSOLIDATION-2-REPORT.md`
- **本阶段新增的文档改动**：`README.md`（共识表 + 难度节 + 功能清单 + 已知限制）、`PROJECT-AI-CONTEXT.md`（§7/§10/§16/§17/§19）、`cmd/node/orphan_checkpoint.go`（1 行注释引用更正）

---

## §1 Documentation truth synchronization

### §1.1 `README.md`

| 位置 | 修改前（stale） | 修改后（truth） |
|---|---|---|
| 共识参数表「初始难度 / 难度上限」 | `MaxTargetBits = MaxDifficultyBits = 16`（上限==下限） | 拆为 `MaxTargetBits=16`（floor）/ `MaxDifficultyBits=32`（post-activation ceiling），并新增 `ActivationHeight=2000`、`NewRulesetActivationHeight=3000`、`NewRulesetInitialBits=27`、版本三态 `1/2/3`、单次幅度 ≤4× |
| 「难度动态范围」 | 固定为 16（上限=下限） | 激活前固定 16；激活后 [16,32] 内按周期浮动 |
| 「难度为何不浮动」节 | 断言难度永远钉死 16 | 改写为「难度规则集：固定激活高度的硬分叉」——v1(<2000 钉死16)/v2([2000,3000) Ceil 浮动)/v3(≥3000 Nearest 浮动 + h=3000 注入27) |
| 「已实现的功能」 | 不含 reorg / 孤儿 / 启动恢复 | 新增 reorg、孤儿持久化与启动恢复、E2E 恢复校验 三条 bullet + 实现状态同步说明（未提交） |
| 「已知限制」 | 含「reorg 未实现」「难度浮动未实现」两条错误陈述 | 删除这两条；改为指向已实现功能，并补充「完整 orphan pool / MaxReorgDepth」等**真实**未做项 |

### §1.2 `PROJECT-AI-CONTEXT.md`

| 节 | 修改要点 |
|---|---|
| §7 共识路径 | 共识参数表难度行拆为 floor/ceiling + 激活高度 + 版本三态；「难度为何不浮动」改写为「难度规则集：固定激活高度的硬分叉」；「规则集版本化/激活」段补入冻结值并标注「已提交共识真值」 |
| §10 orphan/recovery | §4-B1.5 标注 D1/D2 已实现；§4-B2 标注已实现+E2E 验证，并链接 E2E 报告 |
| §16 当前已知问题 | 第 1 条（README 漂移）：标注已于 CONSOLIDATION-2 修复（未提交）；第 2 条（缺失 spec）：标注已创建两份 spec 并更正代码引用名 |
| §17 当前审计阶段 | 最近产物表新增 E2E 报告行；Owner 铁律段落补充「E2E 已在未提交工作树中全链路验证（14 项断言全绿）」 |
| §19 当前未完成工作 | 第 3 条（补齐两份 spec）：✅ 已完成（CONSOLIDATION-2 创建，未提交）；第 13 条（修复 README 漂移）：✅ 已修（工作树编辑，未提交） |

### §1.3 关键事实依据（已逐行核对源码）

- `internal/pow/pow.go`：`MaxTargetBits=16`、`MaxDifficultyBits=32`、`ActivationHeight=2000`、`LegacyBlockVersion=1`、`NewBlockVersion=2`、`NewRulesetActivationHeight=3000`、`NewRulesetInitialBits=27`、`NewRulesetBlockVersion=3`；`IsActivationActive(h,ah)=h>=ah && h>0`；`IsNewRulesetActive(h)=h>=3000 && h>0`；`VersionForHeight` 三态；`ComputeExpectedBitsAt` 的分段规则（h=0→16；0<h<2000→父 bits 钉死；2000≤h<3000→`AdjustBits` Ceil 钳[16,32]；h=3000→27；h>3000→`AdjustBitsNearest` Nearest 钳[16,32]）；`AdjustBits`/`AdjustBitsNearest` 共享 core，4× clamp。
- `cmd/node/orphan_checkpoint.go`：文件格式 `ORPH`+`0x01`+entryCount(u32BE)+SHA256(32B)；entry=parent(32)+childCount(u32BE)+childHash[]；entryCount≤256、childCount≤64；temp+fsync+rename 原子写；fail-closed load。
- `cmd/node/main.go`（D1）：`newOrphanCheckpoint`→`CleanupTmp`→`svc.orphanCP=…`→`prepareOrphanRestore`，**在 `p2pNode.Start()` 之前**。
- `cmd/node/service.go`：`prepareOrphanRestore`（填 `restorePending`、剪枝已知父、fail-closed）、`consumeRestorePending`（握手末尾消费、双删 drain、未知则 `requestBranch`、独立于 `s.waiting`）。

---

## §2 Specification consolidation

两份 spec 仅记录**已实现**行为，无新设计：

| 文件 | 覆盖 | 内容 |
|---|---|---|
| `docs/spec/ORPHAN-DURABILITY-SPEC-v1.md` | §4-B1 + §4-B1.5 | 数据模型（文件格式/大小上界）、序列化、反序列化与校验（fail-closed）、内存投影操作、原子写、生产激活 D1+D2、不变量、证据 |
| `docs/spec/STARTUP-RECOVERY-PLAN-1.md` | §4-B2 | `prepareOrphanRestore`（恢复集填充）、`consumeRestorePending`（握手消费/双删 drain/复用 `requestBranch`）、`requestBranch` 复用原语与限流常量、不变量、证据 |

两份文件均显式标注「仅记录已实现行为，不含新设计」与「未发现于已提交 HEAD，位于未提交工作树」的治理状态，与 `PROJECT-AI-CONTEXT.md` §17 关键状态区分一致。

---

## §3 Final consistency audit

### §3.1 Documentation matches code ✅

| 文档陈述 | 源码事实 | 结论 |
|---|---|---|
| README/§7：v2@2000、v3@3000、ceiling=32、floor=16、版本三态 | `pow.go` 常量与方法 | ✅ 一致 |
| README/§7：单次幅度 ≤4× | `adjustTargetCore` 把 span 夹到 [expected/4, expected×4] | ✅ 一致 |
| ORPHAN-DURABILITY-SPEC-v1：文件格式 `ORPH`/`0x01`/checksum/边界 | `orphan_checkpoint.go` | ✅ 一致 |
| STARTUP-RECOVERY-PLAN-1：D1 在 `Start()` 前、双删 drain、复用 `requestBranch` | `main.go` + `service.go` | ✅ 一致 |
| PROJECT-AI-CONTEXT §7：冻结值为「已提交共识真值」 | `pow.go` 在 `git HEAD` | ✅ 一致（pow.go 不在工作树 diff） |

### §3.2 Tests match claims ✅

| 声明 | 证据 |
|---|---|
| §4-B1.5 实现 + 21 项测试全绿、vet 干净、未提交 | `docs/PHASE-P2PCHAIN-ORPHAN-DURABILITY-4B15-COMPLIANCE-REPORT.md` |
| §4-B2 7/7 专项 PASS、相关回归族全绿、未提交 | `docs/PHASE-P2PCHAIN-ORPHAN-DURABILITY-4B2-COMPLIANCE-REPORT.md` |
| E2E 真实网络/磁盘/进程边界全链路、14 项断言全绿、0 失败 | `docs/PHASE-P2PCHAIN-ORPHAN-DURABILITY-E2E-RECOVERY-1-REPORT.md` |
| 本阶段文档/注释改动未破坏编译 | `go build ./cmd/node` → exit 0；`go vet ./cmd/node` → exit 0（本次复跑确认） |

### §3.3 No stale statements remain（已清除的陈旧陈述）

| 陈旧陈述（修正前） | 位置 | 修正动作 |
|---|---|---|
| 「reorg 未实现，单链追加式」 | README 已知限制 | 删除；reorg 列入已实现功能 |
| 「难度固定为 16（上限=下限）」 | README 共识表/难度节 + PROJECT-AI-CONTEXT §7 | 改写为 v1/v2/v3 规则集 |
| 「`MaxTargetBits = MaxDifficultyBits = 16`」 | README/PROJECT-AI-CONTEXT/旧归档 | 拆为 floor=16 / ceiling=32 |
| 「被引用但不存在的 spec 文档」 | PROJECT-AI-CONTEXT §16.2 | 已创建 `docs/spec/ORPHAN-DURABILITY-SPEC-v1.md` 与 `STARTUP-RECOVERY-PLAN-1.md` |
| `orphan_checkpoint.go:1` 引用名 `ORPHAN-DURABILITY-IMPLEMENTATION-SPEC-v1` | 代码注释 | 更正为 `ORPHAN-DURABILITY-SPEC-v1`（与创建文件同名） |

### §3.4 Residual staleness（超出本阶段授权范围，登记待跟进）

| 残留陈旧项 | 位置 | 不在本阶段范围的原因 | 建议 |
|---|---|---|---|
| 「本链 `MaxTargetBits = MaxDifficultyBits = 16`…链上可达难度恒为 16」 | `docs/DETERMINISTIC-SERIALIZATION-SPEC.md:202` | 未被授权编辑（非 README / PROJECT-AI-CONTEXT / 两份 spec） | 后续阶段单独修订该序列化规范 |
| `docs/MASTER-DESIGN.md` 旧奖励公式 `50 >> (height/210)` | 早期设计文档 | 同上；README 当前奖励公式已正确（`5 >> (height/5_250_000)`） | 后续阶段修订或标注作废 |
| `docs/CANONICAL-CONSENSUS-SPEC.md` 已正确 | — | 无需改动（已声明旧 README 作废） | 无需动作 |

> 说明：`docs/CANONICAL-CONSENSUS-SPEC.md`（行 136/375/455）**已**正确描述 v1/v2/v3 与 2000/3000，并明确旧 README 陈述「已作废」。本次 README / PROJECT-AI-CONTEXT 修订即与这一 canonical 真相重新对齐。

---

## §4 Residual risks & recommendations

1. **孤儿块体非持久**（已知，E2E 报告登记）：检查点只持久化「父键 → 子哈希」，块体依赖 re-gossip / 分支拉取恢复；崩溃窗口内新孤儿可能未落盘。
2. **drain 依赖下次握手**：`consumeRestorePending` 仅在 `OnHandshake` 触发；若长期无对端握手，恢复集保留。
3. **in-process 同 datadir 重启未走 DirLock**：E2E 用独立临时目录模拟重启，未实际触发 `newNodeRuntime` 的 datadir 独占锁路径。
4. **sync/branch 并发交付未隔离**：E2E 未覆盖同步流与分支流并发到达同一父块的交错。
5. **文档未提交**：所有文档/注释同步均在未提交工作树，需 Owner 后续授权 commit 才会进入基线。

---

## §5 HARD STOP confirmation

- ❌ 未执行任何 `git commit` / `git push`。
- ❌ 未部署到任何环境（含腾讯云 3 节点 loopback 集群，其高度 136、mining STOPPED，本次只读、未触碰）。
- ❌ 未修改云/生产配置、未改动任何 consensus / network / wallet 代码真值（`pow.go` 仅被读取、未被编辑）。
- ❌ 未产生任何生产数据变更（仅编辑文档与 1 行代码注释）。

**结论**：CONSOLIDATION-2 目标全部达成——仓库文档真相已与已实现（含已提交共识与未提交 B1/B1.5/B2/E2E）状态同步，两份缺失 spec 已补建，最终一致性审计通过（docs↔code、tests↔claims 一致，无残留陈旧陈述于授权范围内）。所有产出均停留在工作树，遵守 HARD STOP。
