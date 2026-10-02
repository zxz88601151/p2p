# PHASE REORG-1G — FINAL GAP AUDIT REPORT

```text
RECONSTRUCTED FROM EVIDENCE
```

> **This document is a reconstructed archival artifact. It is NOT the original execution report.**
>
> Original execution report was not recoverable from Git history. This reconstruction is based only on surviving evidence.
>
> 原始报告 `PHASE-REORG-1G-FINAL-GAP-AUDIT-REPORT.md` 的正文本 REORG-1J-DOC-2 审计时被确认为
> **REORG-1H 的 prompt 全文**（系统性 off-by-one 覆盖）；原始 9,315 B 字节序列已不可恢复，
> 该 prompt 副本已 rename 归档至 `docs/prompts/REORG-1H.prompt.md`，原始字节另存于
> `docs/_archive_pre-doc2/repo/PHASE-REORG-1G-FINAL-GAP-AUDIT-REPORT.md`。

- 重建阶段：`REORG-1J-DOC-2-REPAIR`
- 重建日期：2026-09-15
- 重建依据：见 §Evidence Sources（全部可定位到文件 + 行号）
- 基线 HEAD（本阶段执行时）：`e04d678`（未变）

---

## 1. VERDICT

```text
PASS WITH BLOCKING GAPS
```

证据：`.workbuddy/memory/2026-09-15.md:236` —— 章节标题原文
`## 08:16–08:30 PHASE REORG-1G（只读最终 gap 审计 → VERDICT = PASS WITH BLOCKING GAPS）`。

---

## 2. Stage Identity

| 字段 | 值 | 证据 |
|---|---|---|
| 阶段名 | PHASE REORG-1G — FINAL GAP AUDIT | `2026-09-15.md:236` |
| 阶段性质 | 严格 READ-ONLY（零源码修改 / 零 git 写操作 / 零部署重启 / 零 datadir 触碰） | `2026-09-15.md:238` |
| 执行时间窗 | 08:16–08:30（2026-09-15） | `2026-09-15.md:236` |
| 基线 HEAD | `e04d678`（未变），branch `main`，staging 空，remote gitea 未改 | `2026-09-15.md:239` |
| 报告原始路径 | `C:/Users/Administrator/Desktop/挖矿/p2pchain/PHASE-REORG-1G-FINAL-GAP-AUDIT-REPORT.md` | `2026-09-15.md:257` |

---

## 3. Scope

1G 为 **只读最终 gap 审计**，覆盖（证据 `2026-09-15.md:240-255`）：

- blocktree 状态机完整性复核；
- 三层 canonical（`bc.blocks` / `tree.BestTip` / store TIP）维护情况；
- Architecture B 是否仍成立；
- crash 五问 A–E 的共识安全性；
- finality / MaxReorgDepth 实现现状；
- F-3 / F-4 / F-5 / F-6 / F-7 精确定位；
- 生产环境（Linux `.123`）部署版本与运行状态核实。

---

## 4. Audit Findings（可证据支撑部分）

### 4.1 代码审计结论（本地）

证据：`2026-09-15.md:240-241`

| 项 | 结论 |
|---|---|
| blocktree 状态机 | 完整（含 `rebuildTree` 会 `SetTip`） |
| 三层 canonical | `bc.blocks` / `tree.BestTip` / store TIP 均被维护 |
| Architecture B | 仍成立（`grep mempool internal/blockchain` = 空） |
| crash 五问 A–E | 全部 consensus-safe（依赖 MODEL A 单 fsync TIP 提交点，非「测试通过」） |

### 4.2 finality / MaxReorgDepth

`finality / MaxReorgDepth = 零实现`（仅 3 处注释）→ 登记 **CONSENSUS GAP → REORG-1I**，不阻塞本阶段，但长期运行需解决。
证据：`2026-09-15.md:242`。

交叉佐证：`internal/blocktree/settip.go:26` 原文「MaxReorgDepth（BG-3，已定稿）：本 SetTip **永不**因深度拒绝更高 work 链——『告警 + 限速，永不拒绝』。深度策略属 REORG-1I，此处不实现、不检查。」（本阶段重建时复核，文件内容可验证）。

### 4.3 F-3 / F-4 / F-5 / F-6 / F-7 精确定位

证据：`2026-09-15.md:243`

| 编号 | 位置 | 状态 |
|---|---|---|
| F-3 | `mempool.go:298` / `:317` | OPEN |
| F-4 | `blockchain.go:742-753` | OPEN |
| F-5 | `blockchain.go:390-397` | OPEN |
| F-6 | `blockchain.go:515-521` | OPEN |
| F-7 | `service.go:258` | OPEN |

全部 P2 / P3，零共识影响，本阶段未修。

### 4.4 BT-1

BT-1 本轮**再次触发**（`go test ./...` FAIL）→ 判定 **NON-BLOCKING**（blocktree 索引层；生产 `rebuildTree` 按序插入，不经 map 迭代）；但引用「全量绿灯」前须排除。
证据：`2026-09-15.md:244`。

---

## 5. Tests

证据：`2026-09-15.md:245`

| 项 | 结果 |
|---|---|
| `go test ./...` | 除 BT-1 外全绿 |
| `cmd/node` | **172.9 s PASS** |
| **NOT COVERED** | ① 进程级 `kill -9` crash 注入；② 跨主机多节点；③ 真实 fork 竞争自动化回归 |

---

## 6. Measured Outputs — §10 决定性发现

证据：`2026-09-15.md:246-251`

| 观测 | 实测值 |
|---|---|
| Linux `.123` 运行二进制 SHA | `4d2345b8…` |
| 该二进制 mtime | `2026-09-14 03:26:36 UTC` = `11:26:36 +0800` |
| `674c5ad` 提交时间 | `11:11:05` |
| `b2724c8` 提交时间 | `21:34:17` |
| **结论** | **部署版本 = `674c5ad`，落后 LOCAL HEAD 6 个 commit** |
| 落后的 6 个 commit | `b2724c8`（难度共识）/ `7d0e1e9`（1A）/ `b001f57`（1B）/ `6ed1e83`（1E）/ `369d36e`（1C）/ `e04d678`（1F） |
| 由此导致的缺失能力 | 无 reorg、无难度共识、legacy 存储 |

`.123` `/status` 实测：`height=1302`、`bits=16`、`difficulty=1`、`peers=[]`、
`mining_state=STALLED / subsidy-exhausted-no-fee-tx`、`accepted=42`、`rejected=0`；
`blocks.dat` = 244,968 B；`node.lock` pid=47785 与进程一致。
同机仍驻留 2 个 P2-SIGTERM 遗留挖矿进程（p2a 17890 / p2c 17894）；服务器 `~/p2pchain-longrun` **无 .git**。

`.200:22222` **连接超时不可达** ⇒ 当前无法组建两节点网络。

---

## 7. Stage Readiness

证据：`2026-09-15.md:252`

| Stage | 结论 |
|---|---|
| A（本地） | **READY** |
| B（Linux 多节点） | **NOT READY** |
| C（Windows 挖矿） | **NOT READY** |

---

## 8. Blocking Gaps（B-1 … B-6）

证据：`2026-09-15.md:253-254`

| 编号 | 阻塞项 |
|---|---|
| B-1 | 部署落后 6 commit |
| B-2 | 无 P2P 分支同步（`OnBlock` 丢弃 fork 块 ⇒ 跨节点 reorg 物理不可能） |
| B-3 | 无 orphan pool |
| B-4 | `.200` 不可达 |
| B-5 | 生产链 subsidy 耗尽 |
| B-6 | 无 reorg 可观测（`/status` 零 reorg 字段） |

---

## 9. Next Step（原报告结论）

推荐下一阶段（唯一）：**REORG-1H**（P2P 分支同步 + work-aware handshake + orphan-tolerant intake）—— 不解决则永远测不到 reorg。
部署时机倾向 **OPTION A**（先补齐 1H / B5 / 1J 再部署）；OPTION B 可验证存储迁移但对 reorg 价值 ≈ 0。
证据：`2026-09-15.md:255-256`。

---

## 10. Non-Actions

证据：`2026-09-15.md:238`（严格 READ-ONLY）、`:243`（F-3..F-7 未修）、`:244`（BT-1 未修）、`:258`（STOP）

- ❌ 零源码修改
- ❌ 零 git 写操作（无 commit / push / tag / merge / rebase）
- ❌ 零部署、零重启、零生产 datadir 触碰
- ❌ 未修 F-3 / F-4 / F-5 / F-6 / F-7
- ❌ 未修 BT-1
- ❌ 未实现 finality / MaxReorgDepth（登记为 REORG-1I）

---

## 11. Stop Condition

```text
STOP.
未进入下一阶段，等待授权。
```

证据：`2026-09-15.md:258`。

---

## Evidence Sources

| # | 来源 | 精确定位 | 用途 |
|---|---|---|---|
| 1 | `.workbuddy/memory/2026-09-15.md` | 行 **236–258**（章节「08:16–08:30 PHASE REORG-1G」全段） | 阶段身份、时间窗、基线、VERDICT、审计结论、F-3..F-7、测试结果、§10 决定性发现、B-1..B-6、NEXT、STOP |
| 2 | `.workbuddy/memory/2026-09-15.md` | 行 **227**、**234** | 基线 `e04d678` 的来源与阶段链位置 |
| 3 | `internal/blocktree/settip.go` | 行 **26** | finality / MaxReorgDepth 零实现的代码层交叉佐证 |
| 4 | `PHASE-REORG-1J-R4C-FINAL-CLOSURE-AUDIT-REPORT.md` | §3.1 行 **71**（八阶段审查覆盖确认） | 「REORG-1G = DELIVERED（PASS W/ GAPS）；GAP-1H-A（P0）立项源」的二次确认 |
| 5 | `PHASE-REORG-1J-DOC-2-EVIDENCE-ARCHIVE-CLOSURE-AUDIT-REPORT.md` | §3 表（1G 行）、§3.1（1G 十七问）、§4.1 第 11 行、§5.1、§7 | 原始报告被 1H prompt 覆盖的归档事实与重建授权 |
| 6 | `docs/_archive_pre-doc2/repo/PHASE-REORG-1G-FINAL-GAP-AUDIT-REPORT.md` | 全文件 9,315 B，SHA-256 `f23c2c38e4fce745498cdbe3892de777b252088fcbc4ce2b08e6518d81fc0b79` | 被覆盖后残留正文（= REORG-1H prompt）的不可变快照 |
| 7 | `docs/prompts/REORG-1H.prompt.md` | 全文件（与 #6 同哈希） | 覆盖 1G 报告路径的 prompt 归档副本 |

**未采用来源**：无。

---

## EVIDENCE INSUFFICIENT 字段清单

以下字段在现存证据中**无直接记录**，按 Rule A 不填充、不推测：

| 字段 | 状态 |
|---|---|
| 原报告的完整章节结构（节号 / 标题） | EVIDENCE INSUFFICIENT |
| 原报告的字节大小（重建前） | EVIDENCE INSUFFICIENT（仅知残留 prompt 副本 9,315 B） |
| F-3 / F-4 / F-5 / F-6 / F-7 的完整描述文本 | EVIDENCE INSUFFICIENT（仅存位置与 OPEN / P2 / P3 判定） |
| crash 五问 A–E 的逐问内容 | EVIDENCE INSUFFICIENT |
| §10 之外各节的实测命令原文 | EVIDENCE INSUFFICIENT |
| Architecture B 的定义原文 | EVIDENCE INSUFFICIENT |
| 3 处 finality 注释的具体位置 | EVIDENCE INSUFFICIENT |

---

## STOP

```text
本文件为 RECONSTRUCTED FROM EVIDENCE 归档产物。
不得作为原始执行报告引用。
```
