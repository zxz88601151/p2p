# FINAL-PROTOCOL-AUDIT-1-REPORT

> **Phase**：FINAL-PROTOCOL-AUDIT-1 — 最终只读协议正确性评估（多阶段验证收口）
> **Owner authorization**：PHASE P2PCHAIN — FINAL-PROTOCOL-AUDIT-1
> **Date**：2026-10-02
> **Scope**：Final read-only protocol validation consolidation · No code changes · No consensus changes · No protocol changes
> **HARD STOP（强制）**：No commit · No deployment · No cloud modification · No source changes

---

## 0. 审计对象与基线

| 项 | 值 |
|---|---|
| `git rev-parse HEAD` | `52fb464af243fbcdfd78bab846b45ea090e918fb` |
| 二进制 `node.exe` SHA-256 | `fb3d2b57dd8adbd02d3c6d501f42a2cfa3c076657694e35756967caf9307b717` |
| 创世身份 `CanonicalGenesisHash` | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |
| 当前规则集 | v1（链高 < 2000 未激活 v2/v3） |

**本审计聚合的验证阶段（9 份报告）**：

| # | 阶段报告 | 验证内容 |
|---|---|---|
| 1 | `PHASE-P2PCHAIN-ORPHAN-DURABILITY-4B15-COMPLIANCE-REPORT.md` | §4-B1.5 孤儿持久化生产激活 |
| 2 | `PHASE-P2PCHAIN-ORPHAN-DURABILITY-4B2-COMPLIANCE-REPORT.md` | §4-B2 启动恢复接线 |
| 3 | `PHASE-P2PCHAIN-ORPHAN-DURABILITY-E2E-RECOVERY-1-REPORT.md` | 孤儿耐久端到端恢复全链路 |
| 4 | `CONSOLIDATION-2-REPORT.md` | 仓库真相同步 |
| 5 | `DOCUMENT-REMAINDER-CLEANUP-1-REPORT.md` | 残留陈旧文档引用清理 |
| 6 | `CONSENSUS-RUNTIME-VALIDATION-1-REPORT.md` | 运行时共识状态（PoW/难度/chainwork/UTXO） |
| 7 | `MULTI-NODE-CANONICAL-SYNC-VALIDATION-1-REPORT.md` | 多节点同步 + canonical 收敛 |
| 8 | `TIE-BREAK-CONSENSUS-VALIDATION-1-REPORT.md` | 等功 fork-choice 确定性 tie-break |
| 9 | `NODE-RESTART-CONSISTENCY-VALIDATION-1-REPORT.md` | 节点重启一致性 |

---

## 1. Consensus validation（共识正确性）

| 性质 | 验证方式 | 结果 |
|---|---|---|
| **PoW（双 SHA256）** | 独立 Python 重算 665+201+201 块（运行时）+ 多节点实验块 | ✅ **PROVEN**——1067 块全量 `hash < target` 通过，零无效 |
| **难度规则集（v1/v2/v3）** | 源码核对 + 运行时 bits 直读 | ✅ **PROVEN（v1）/ UNPROVEN（v2/v3）**——v1 钉死 16 实证；v2@2000/v3@3000 仅源码冻结、**未达激活高度、未运行时验证** |
| **fork-choice（高功胜）** | 多节点实验：8 块分叉连 12 块 canonical | ✅ **PROVEN**——`REORG_ACCEPT`（8 detach/8 attach）实证高工作量胜 |
| **tie-break（等功较大 tip hash 胜）** | 等功分叉实验：X(`0000a9ae`) vs Y(`00008828`) | ✅ **PROVEN**——观察者确定性选较大 hash，与 `tieBreakWinner` 逐字节一致 |

**结论**：fork-choice 的**全部三条决策分支**（`CompareWork>0` 反超胜 / `<0` 保持 / `==0` 较大 tip hash 胜）均已运行时实证，确定性、可复算。

---

## 2. Network validation（网络正确性）

| 性质 | 验证方式 | 结果 |
|---|---|---|
| **握手（handshake）** | 多节点日志：`握手完成: 对端高度/工作量/链尾` | ✅ **PROVEN**——交换高度、work、tip 正确 |
| **区块同步（block sync）** | `SYNC_BATCH_RECEIVED/APPLIED`（12 块、14 块批量） | ✅ **PROVEN**——批量拉取、逐块校验上链、0 缺父 |
| **多节点收敛** | A/B/C/D 四节点最终同高同 tip 同 work | ✅ **PROVEN**——同 canonical 收敛 |

**结论**：P2P 传输 + 批量同步 + 分支补齐（`block_by_hash_resp`）均真实跑通，多节点确定性收敛。

---

## 3. Persistence validation（持久化正确性）

| 性质 | 验证方式 | 结果 |
|---|---|---|
| **孤儿耐久（§4-B1/B1.5）** | E2E 真实 `orphan_waiting.bin` 原子写/校验/加载 | ✅ **PROVEN**（工作树未提交）——14 项耐久+恢复断言全绿 |
| **启动恢复（§4-B2）** | E2E 真实 `prepareOrphanRestore`→`consumeRestorePending`→`requestBranch` | ✅ **PROVEN**（工作树未提交）——父恢复、孤儿消解、链一致 |
| **重启一致性** | A/B/C 停 B→重启 B→补齐 14 块→收敛 | ✅ **PROVEN**（已提交路径）——存储回放 + 重连 + 增量同步，`verify=true` |

> ⚠️ **边界**：孤儿持久化/启动恢复（§4-B1/B1.5/B2）虽已实现并 E2E 验证，但**属未提交工作树**（`orphan_checkpoint.go` 等 8 文件全部 untracked），**不在 `52fb464` 提交基线内**。当前本地 `node.exe`（今日重建）已含该代码（启动日志「孤儿等待检查点已启用」实证），但这是**工作树构建产物**，非提交基线。详见 §7 治理状态。

---

## 4. Reorganization validation（重组正确性）

| 性质 | 验证方式 | 结果 |
|---|---|---|
| **更高工作量 reorg** | 8 块分叉连 12 块 canonical → `REORG_ACCEPT` | ✅ **PROVEN** |
| **更低工作量拒绝** | `REORG_REJECT reason=chainwork_not_won` ×N | ✅ **PROVEN** |
| **等功确定性 tie-break** | X/Y 等功 → 较大 tip hash 胜 | ✅ **PROVEN** |

**结论**：重组三态（高功重接 / 低功拒绝 / 等功确定性）全部实证，无非法 reorg。

---

## 5. Documentation consistency（文档一致性）

| 文档 | 状态 |
|---|---|
| `README.md` | ✅ 已同步（CONSOLIDATION-2 + CLEANUP-1 修正 reorg/难度漂移） |
| `PROJECT-AI-CONTEXT.md` | ✅ 已同步（§7 共识、§10 孤儿、§17 状态区分） |
| `CANONICAL-CONSENSUS-SPEC.md` | ✅ 本就正确（v1/v2/v3 权威定义） |
| `DETERMINISTIC-SERIALIZATION-SPEC.md` | ✅ 已清理（§9.7 难度、§9.4.4 奖励） |
| `MASTER-DESIGN.md` | ✅ 已清理（奖励公式、reorg 历史注记） |
| `docs/spec/ORPHAN-DURABILITY-SPEC-v1.md` | ✅ 已补建（仅记已实现） |
| `docs/spec/STARTUP-RECOVERY-PLAN-1.md` | ✅ 已补建（仅记已实现） |
| 历史 `PHASE-*` 阶段报告 | ⚠️ 部分仍沿用旧难度/README 措辞（登记待后续清理，不影响真值） |

**结论**：核心权威文档（README / PROJECT-AI-CONTEXT / CANONICAL / 序列化 spec / MASTER-DESIGN）已全部对齐；历史阶段报告存在非阻断的旧措辞残留。

---

## 6. Proven vs Unproven properties（性质清单）

### 6.1 ✅ 已证明（PROVEN）性质

| 领域 | 性质 |
|---|---|
| 共识 | 双 SHA256 PoW 有效性；v1 难度钉死 16；fork-choice 三态（高功胜/低功保持/等功较大 tip hash 胜）；累积工作量 = Σ2^bits 精确 |
| 网络 | 握手字段正确；批量同步；分支补齐；多节点确定性收敛 |
| 持久化 | 存储回放恢复；增量同步；重启收敛；孤儿检查点原子写/校验/加载；启动恢复 drain |
| 重组 | 高功重接；低功拒绝；等功确定性 tie-break；无非法 reorg |
| 状态 | UTXO 状态迁移一致（`verify=true`）；区块索引完整；存储帧无尾随 |

### 6.2 ⚠️ 未证明（UNPROVEN）性质

| 领域 | 未证明项 | 原因 |
|---|---|---|
| 共识 | **v2/v3 规则集运行时行为** | 链高 < 2000，未达激活高度；Ceil/Nearest/注入 27 仅源码冻结，未运行时验证 |
| 共识 | **难度浮动真实发生** | 无链越过 2000；`AdjustBits` 浮动路径未在生产链上触发 |
| 持久化 | **孤儿块体恢复** | 仅持久化父键、不持久化块体；依赖父恢复后网络 re-gossip，若块不再被持有则永久丢失 |
| 持久化 | **真实进程 DirLock 重启耦合** | E2E 用进程内同 datadir 模拟，未走 `newNodeRuntime` 的 `DirLock` 获取/释放 |
| 网络 | **sync/branch 并发交付隔离** | 未隔离单路送达路径 |
| 安全 | **威胁模型 / 攻击矩阵** | `SECURITY-BOUNDARY.md`/`LIMITATIONS.md` 仍 SKELETON |

### 6.3 📌 已知限制（DEFER / 明确不做）

| 限制 | 状态 |
|---|---|
| 完整 orphan pool（定时重播/评分/封禁） | 明确未做（REORG-1G/B5） |
| MaxReorgDepth 深度策略 | 未实现、不检查（REORG-1I/BG-3） |
| P2P 传输安全（TLS/对等认证/peer scoring/ban/协议版本协商） | 明确未做（LTM-002/003） |
| 安全边界/威胁模型正式文档 | SKELETON |
| secp256k1 / RIPEMD160 迁移 | 仅注释留升级路径（现 P-256 + SHA256 截断） |
| 钱包明文落盘 | 学习用途（0600 JSON） |
| SPV / 轻节点 / 代币经济模型 | 明确不做 |
| readTimeout/heartbeat/keepalive 等传输加固 | 明确 DEFER |

---

## 7. Release readiness assessment（发布就绪评估）

### 7.1 结论：**「已实现」与「已发布」之间仍存在未收口的提交边界，尚不具备发布条件。**

**已具备（可支撑「developer node 学习/实验」定位）**：
- 共识（PoW/v1 难度/fork-choice/tie-break）真实运行实证成立，确定性、可复算。
- 网络同步 + 多节点收敛 + 重启一致性，全链路真实跑通。
- 文档真相已对齐（README/PROJECT-AI-CONTEXT/CANONICAL/spec）。
- `go build`/`go vet`/全量测试绿（前序阶段实测 587s 全绿）。

**阻断发布的未收口项（按严重度）**：

| 严重度 | 项 | 说明 |
|---|---|---|
| **P0** | §4-B1/B1.5/B2（孤儿持久化+启动恢复）**未提交** | 8 个文件 untracked，仅存在于工作树与今日重建的二进制；发布物是否含该代码取决于构建时的 working tree，**身份不确定** |
| **P1** | 孤儿块体不持久化 | 真实网络下若块不再被对端持有则永久丢失（容量边界） |
| **P1** | v2/v3 规则集未运行时验证 | 激活高度 2000/3000 未达，浮动难度/Nearest/注入 27 仅源码冻结 |
| **P2** | 安全边界/威胁模型 SKELETON | 无攻击矩阵、无 peer scoring/ban |
| **P2** | P2P 明文无认证 | 仅适合本机/可信网络 |

**发布建议**（Owner 决策，非本审计自动执行）：
1. **先收口提交边界**：明确发布物基于 `52fb464`（不含孤儿代码）还是工作树（含 B1/B1.5/B2）——当前二者 SHA 不同（`node.exe` 为工作树构建，含孤儿代码）。
2. 孤儿代码若纳入发布，需补齐「真实进程 DirLock 重启」「孤儿块体可达性」两项边界验证。
3. 补全安全边界文档（威胁模型/攻击矩阵）或明确标注「未审计、不可用于真实资产」。
4. 维持「Developer Node / 学习实验」定位，**不得**宣称生产可用。

---

## 8. 最终裁定

| 维度 | 裁定 |
|---|---|
| 共识正确性 | ✅ 已实证（PoW/难度 v1/fork-choice/tie-break 全分支覆盖） |
| 网络正确性 | ✅ 已实证（握手/同步/收敛） |
| 持久化正确性 | ✅ 已实证（重启一致性 + 孤儿耐久 E2E，后者未提交） |
| 重组正确性 | ✅ 已实证（高功/低功/等功三态） |
| 文档一致性 | ✅ 已对齐（权威文档 + 两份补建 spec） |
| **发布就绪** | ⛔ **未就绪**（提交边界未收口 + 安全边界 SKELETON + v2/v3 未运行时验证） |

**一句话结论**：协议核心机制（共识/fork-choice/同步/持久化/重组/文档）经真实运行验证均**正确且确定性成立**，但项目仍处于「实现完成、验证完成、**未提交收口**」状态，且安全边界与 v2/v3 规则集存在未覆盖项——**判定为「协议正确性已验证」而非「发布就绪」**。

---

## HARD STOP confirmation

- ❌ 未执行任何 `git commit` / `git push`。
- ❌ 未部署到任何环境（腾讯云三节点集群未触碰）。
- ❌ 未修改云/生产配置。
- ❌ **未改动任何源码**（`cmd/`、`internal/` diff 均为前序 B1/B1.5/B2 阶段既存；本阶段为纯只读聚合，零源码、零实验、零状态变更）。
- ❌ 未产生任何生产数据变更。

**本审计方法**：纯聚合 9 份已完成验证报告 + 只读核对源码事实（`internal/blocktree/settip.go` 的 tie-break、`internal/pow/pow.go` 的激活常量、孤儿 E2E 残留风险、`git ls-files` 确认 untracked），无任何新实验、无任何写入。
