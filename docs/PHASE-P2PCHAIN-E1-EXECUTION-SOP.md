# P2PCHAIN E1 EXECUTION SOP（v1.0 FREEZE CANDIDATE，待用户批准后生效）

> **来源**：T2 根因复盘（2026-09-19 15:52）冻结的 guard-file 方案 + 官方样本 `E1-20260919-155634-198`（15:56–16:03）的成功实证。
> **地位**：本 SOP 由「E1 EXECUTION SOP FREEZE + GAP-4 READINESS AUDIT」阶段整理产出，经「SOP v1.0 FREEZE / COMMIT READINESS AUDIT」阶段修订为 Freeze Candidate；**待用户批准后版本升 v1.0 生效**。批准生效后为 E1 及后续所有 cell 执行的唯一流程权威。
> **v1.0-RC1 → v1.0-FC 修订记录**：① §3 断言表增加证据状态列（已实证 ✅ / 设计要求 🔧），严禁将设计要求表述为已验证事实；② §5 补齐 archive verification 操作定义（存在性 + SHA256 + manifest 注册 + 清理授权 + 缺失证据处置）；③ §2 终态条款升级为规范性 MUST（终态写入前必须检查无既有终态行）。
> **证据权威**：路径一律遵循 `docs/PHASE-P2PCHAIN-E1-EVIDENCE-PATH-AUTHORITY.md`。

---

## 1. RUN IDENTITY（SOP-A）

```text
EXECUTION_ID = E1-<UTCDate>-<HHMMSS>-<seq>
             例：E1-20260919-155634-198（沿用已实证格式）
```

- **创建时机**：执行脚本启动后**第一动作**（先于任何节点启动）。
- **唯一性**：guard 文件存在即拒绝同 ID 重入（§2）；seq 取随机 3 位十进制防撞。
- **绑定**：EXECUTION_ID 写入 guard 文件、写入门-guard 目录名、注入两节点启动参数记录、写死至 evidence root 目录名：
  `H:\wakuang-evidence\E1\runs\<EXECUTION_ID>\`
- **传播**：所有 log 快照 / blocks.dat 副本 / canonical 序列 / attribution 输出文件名前缀 = EXECUTION_ID。

## 2. SINGLE INSTANCE GUARD（SOP-B）

沿用已实证的 `.e1-exec-guard` 追加式 guard 文件（实物 schema 见官方样本），字段：

```text
execution_id=…          # §1
created=<UTC timestamp>
baseline_console=<commit>
baseline_p2pchain=<commit>   # 必须 = efe02a7（SOP-D anchor）或用户显式批准的新 anchor
experiment=<cell 标签>
state=RUNNING | COMPLETED | FAILED | ABORTED
completed=<UTC timestamp>    # 仅终态写
result=<结论标签>
```

规则：
1. **启动前检查**：guard 已存在且含任意 `state=` 行 ⇒ **拒绝启动**（no silent retry；由人工确认 stale 后方可归档重开）。
2. **stale 语义**：guard 存在但无终态行且 `created` 距今 > 24h ⇒ 标记 STALE，仅可由人工显式确认后写 `state=ABORTED` 并另起 run；**禁止自动覆盖**。
3. **crash recovery**：节点崩溃 ⇒ 写 `state=FAILED`（含原因），evidence 保留，run 作废；不得复用同一 EXECUTION_ID 重跑。
4. **终态唯一性（规范性 MUST）**：写入任何终态行（COMPLETED / FAILED / ABORTED）**之前必须检查 guard 中不存在既有终态行**；已含终态的 guard 禁止再追加任何 state 行（官方样本 guard 曾出现两条 COMPLETED 记录——16:03:09 / 16:03:47，属追加式实现缺陷，v1.0 起明确禁止）。每个 run 恰好一个终态，三选一。

## 3. STARTUP ASSERTIONS（SOP-C，全部通过才允许 /mine/start）

按序断言，任一失败 ⇒ 写 `state=ABORTED` 并停止：

| # | 断言 | 证据状态 |
|---|---|---|
| 1 | 端口空闲：两侧 P2P/RPC 端口 `netstat`/`ss` 零监听 | ✅ 已实证（E0 cleanup 纪律 + 官方样本） |
| 2 | fresh datadir：两侧 datadir 目录不存在或为空（blocks.dat 缺席） | ✅ 已实证（官方样本 fresh datadirs gate） |
| 3 | fresh genesis：B 先启后 `/api/nodes` h=0 且 tip=`0000aca1af72…` | ✅ 已实证（两次 E1 均 B h=0/tip 0000aca1） |
| 4 | evidence destination：`H:\wakuang-evidence\E1\runs\<ID>\` 已创建且所在盘 free ≥ 2×预期日志量 | 🔧 设计要求（未实证；官方样本证据散于 E:/G: 教训） |
| 5 | expected node/miner count：恰 2 节点、2 矿工；remote B 无第三者 P2P 互连（peers 恰为对端） | ✅ 双侧互联已实证；peers 数断言 🔧 设计要求 |
| 6 | binaries/scripts：node 二进制 SHA256 = 冻结基线；脚本版本号匹配 | 🔧 设计要求（SHA 记录已实践，断言未程序化） |
| 7 | EXECUTION_ID + UTC timestamp 已写入 guard 与 evidence root | ✅ 已实证（guard 实物 + 报告三处传播） |
| 8 | cell 配置快照（tc 规则/时长/预期速率）已写入 `manifests\` | 🔧 设计要求（tc 规则记录曾实践，manifests 目录化为新增） |

> **标注纪律**：✅ = empirically demonstrated（有实物/报告证据）；🔧 = SOP-required but not yet experimentally demonstrated（设计要求，执行时首次实证）。**禁止把 🔧 项表述为已验证事实。**

## 4. COMPLETION CONTRACT（SOP-D）

- **COMPLETED** 仅当：窗口正常结束 + 停机干净（端口释放）+ 两侧 blocks.dat/node.log 已落 evidence root + 清理完成。**COMPLETED ≠ execution-valid ≠ E1 closure-valid**——它只表示「流程完整、证据已归档」。
- **FAILED**：节点崩溃 / 断言中途失败 / 证据落盘失败。
- **ABORTED**：人工终止 / 断言前置失败。
- **execution-valid**（如 fresh genesis、单窗、无中途干预）与 **E1 closure-valid**（需 per-miner coinbase 归因 + 全部 §6 Gate）由**分析阶段**另行裁定，写入 run 报告，绝不写入 guard。

## 5. EVIDENCE CONTRACT（SOP-E）

可追溯链（EXECUTION_ID 贯穿）：

```text
run(guard+manifests) → node logs（双端全量）→ blocks.dat（双端）
→ snapshots（/api/nodes 窗口内快照）→ canonical hash sequence
→ attribution evidence（GAP-4 parser 输出，含 blocks.dat SHA256 输入绑定）
```

- 大文件本体入 `runs\<ID>\`，SHA256 入 `manifests\`（与冻结计划 §13 一致）。
- **双端 blocks.dat 均为强制归档件**（教训：官方样本 lin 侧 blocks.dat 因清理纪律未归档，见审计报告 §5）。
- **Archive Verification（清理前置门，全部通过才允许任何清理）**：
  1. **存在性核验**：`runs\<ID>\` 下逐文件 `ls` 对账 manifest 清单（缺一即 FAIL）；
  2. **完整性核验**：逐文件 SHA256 实测并写入 `manifests\SHA256SUMS.txt`，`sha256sum -c` 全 OK；
  3. **注册核验**：EXECUTION_ID 与 guard 终态行、manifest、run 报告三方一致；
  4. **清理授权**：上述 1–3 全 PASS 后方可在本 run 范围内执行清理；跨 run/共享目录清理一律另立授权；
  5. **缺失证据处置**：任何清单件缺失或校验失败 ⇒ 本 run 禁止清理 + `state=FAILED`（若未终态）+ 缺失登记进 run 报告（官方样本 lin 侧 blocks.dat UNLOCATED 即为此类先例）。

## 6. ACCOUNTING INTEGRATION（衔接 SOP-B 冻结口径）

- `D_log`：stderr 全量捕获（`P2PCHAIN_OBS_FILE` 或重定向），文本匹配 `MINING_BLOCK_ACCEPTED`（发射点 `cmd/node/main.go:795`，仅本地挖矿）。
- `C_storage/S_storage`：GAP-4 parser 输出（blocks.dat + coinbase PubKeyHash），**禁止**以 printchain 现跑列表作为最终归因。
- 闭环实验目标：`D_log(i) == C_storage(i) + S_storage(i)`（per-miner，coinbase 过滤后）。

---

*v1.0 FREEZE CANDIDATE · 2026-09-20 · 待用户批准（批准即 v1.0 生效）· 修订须用户授权*
