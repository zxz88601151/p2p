# PHASE REORG-1J-R3 — LEGACY-PREFIX CRASH MATRIX FINAL REPORT

```text
RECONSTRUCTED FROM EVIDENCE
```

> **This document is a reconstructed archival artifact. It is NOT the original execution report.**
>
> Original execution report was not recoverable from Git history. This reconstruction is based only on surviving evidence.
>
> 原始报告 `PHASE-REORG-1J-R3-LEGACY-PREFIX-CRASH-MATRIX-FINAL-REPORT.md` 的正文本 REORG-1J-DOC-2 审计时被确认为
> **REORG-1J-R4A PRE-GATE 的 prompt 全文**（系统性 off-by-one 覆盖；末行「只有在 PRE-GATE 明确判定 READY 后…」）；
> 原始 4,392 B 字节序列已不可恢复，该 prompt 副本已 rename 归档至 `docs/prompts/REORG-R4A-PRE-GATE.prompt.md`，
> 原始字节另存于 `docs/_archive_pre-doc2/repo/PHASE-REORG-1J-R3-LEGACY-PREFIX-CRASH-MATRIX-FINAL-REPORT.md`。

- 重建阶段：`REORG-1J-DOC-2-REPAIR`
- 重建日期：2026-09-15
- 重建依据：见 §Evidence Sources（全部可定位到文件 + 行号）

---

## 1. VERDICT

```text
PASS（零生产改动，未提交）
```

证据：`.workbuddy/memory/2026-09-15.md:282` —— 章节标题原文
`## REORG-1J-R3 — LEGACY-PREFIX CRASH MATRIX（VERDICT = PASS，零生产改动，未提交）`。

---

## 2. Stage Identity

| 字段 | 值 | 证据 |
|---|---|---|
| 阶段名 | REORG-1J-R3 — LEGACY-PREFIX CRASH MATRIX | `2026-09-15.md:282` |
| 执行日期 | 2026-09-15 | `2026-09-15.md` 当日日志（位于 1H 09:00–09:45 与 R4A PRE-GATE 13:15–13:40 之间） |
| 执行时间窗 | **EVIDENCE INSUFFICIENT** | 见 §EVIDENCE INSUFFICIENT 字段清单 |
| 基线 HEAD | `e04d678`（未动）；暂存区空 | `2026-09-15.md:284` |
| git 写操作 | 无 commit / push / tag / merge / rebase / amend / squash | `2026-09-15.md:284` |
| 报告原始路径 | `C:/Users/Administrator/Desktop/挖矿/p2pchain/PHASE-REORG-1J-R3-LEGACY-PREFIX-CRASH-MATRIX-FINAL-REPORT.md` | `2026-09-15.md:301` |

---

## 3. Scope

证明 **legacy 前缀存在时的 reorg 崩溃/恢复边界**：定位唯一持久提交点，并用字节级 + 进程级双重注入证明
canonical 恰好在该点跃变一次，且 legacy 前缀物理字节零改变。

---

## 4. §6 提交边界结论（只读分析，已固化进报告）

证据：`2026-09-15.md:286-289`

| 结论 | 内容 |
|---|---|
| 提交点 | `FileBlockStore.appendFrames`（`v2.go:713`）内**唯一一次** `file.Sync()` |
| 提交记录 | **TIP 帧**（恒为帧组末位） |
| 无独立 journal | 无活动链尾文件；blocktree 无持久化（`rebuildTree` 重建）；UTXO 无持久化（回放 0.52 ms/块） |
| canonical 提交 | `commitTipAfterAppend`（`v2api.go:58`），只在 TIP fsync 之后推进视图 |
| 无 crash hook 接缝 | 生产**不存在**任何 crash hook（`cmd/node/testPanicAtStart` 是启动期 panic，与 reorg 无关）⇒ 按 §6 **不得新增钩子**，改用字节级注入 + 进程级 kill |

**本阶段重建时复核**：`internal/storage/v2.go:713` = `func (s *FileBlockStore) appendFrames(...)`，其上一行注释「按序追加若干帧并**只 fsync 一次**（F7/§14 单 fsync 设计）」；`internal/storage/v2api.go:58` = `func (s *FileBlockStore) commitTipAfterAppend(...)`，上一行注释「在 TIP 记录已写入并 fsync 后推进 canonical 状态」。行号与 memory 记录一致。

---

## 5. Tests

**新增文件（仅测试）**：`2026-09-15.md:290`

| 文件 | 内容 | 本阶段复核位置 |
|---|---|---|
| `internal/storage/r3_crash_matrix_test.go` | C0–C6 + 字节穷举 + I1–I10 | `TestR3_C0_PreReorgCrash`(:526) / `C1`(:539) / `C2C3`(:565) / `C4_CommitBoundary`(:628) / `C4_ByteSweep`(:676) / `C5`(:715) / `C6`(:741) / `I10_KeyCrashPointsRepeatFiveTimes`(:773) |
| `cmd/node/r3_crash_restart_test.go` | 真实进程 kill ×5 | `TestR3_RealProcessCrashRestartLegacyPrefix`(:63) |

**场景**（`2026-09-15.md:291`）：legacy 前缀 0–2（`legacyLen=3`）+ v2 canonical 3–5（旧链尾 h5）+ 自 **legacy 高度 2** 分叉的分支 3–6（新链尾 h6）＝ GAP-1H-A 现实形态。两个 variant：

- **A** = 全 detached
- **B** = 新链尾由 `CommitReorg` 首次写入（UNDO + BLOCK + TIP）

---

## 6. Measured Outputs

### 6.1 delta 形状（`2026-09-15.md:293`）

| variant | delta 大小 | 帧数 | TIP 帧 |
|---|---|---|---|
| A | **780 B** | 5 帧 | 恒 = 1 且位于末位 |
| B | **1036 B** | 6 帧 | 恒 = 1 且位于末位 |

⇒ **C2 / C3「disconnect/connect 途中」无任何中间持久状态，等价于 C1**。

### 6.2 字节穷举（`2026-09-15.md:294`）

| variant | 偏移总数 | 划分 |
|---|---|---|
| A | **781** | 0–779 旧 / 780 新 |
| B | **1037** | 0–1035 旧 / 1036 新 |

⇒ **canonical 恰好跃变一次，发生在 TIP 完整落盘处**。
合计 **1,818 字节级崩溃点**（与 `R4C §3` 行 59 记载「1,818 字节级崩溃点」一致）。

### 6.3 进程级 kill（`2026-09-15.md:295`）

- B = 高度 100、A = 高度 5，共同 legacy 前缀 2；
- 杀点 0 / 15 / 40 / 100 / 1500 ms ⇒ 2–3 轮真正落在未收敛（杀死时高度 83 / 97）；
- 重启后**全部**收敛到高度 100 且链尾 == B，**legacy 前缀零改变、无截断**。

### 6.4 同步速率（`2026-09-15.md:296`）

≈ **2.5 ms/区块**（100 区块 < 200 ms）⇒ 进程级 kill 原理上无法可靠命中微秒级提交窗，**由字节穷举补偿**。

### 6.5 回归闸门（`2026-09-15.md:299`）

| 项 | 结果 |
|---|---|
| R3 存储 | **8/8 PASS（16.5 s）** |
| R3 进程 | **5/5 PASS（131.6 s）** |
| G09 R1 `TestG08_LegacyImmutableAndTruncateGuard` | PASS |
| G09 R2 `TestRealProcessForkReorgConvergesByHash` | PASS（**41.8 s**） |
| `go test ./cmd/node/...` | ok（**356.98 s**） |
| `go test -race ./internal/...` | 全 ok（storage 167 s / blockchain 104 s / mempool 71 s） |
| `go build ./...` / `go vet ./...` | OK |
| `git diff --check -- internal cmd/node` | 干净（全仓库告警只来自长期 dirty 的 `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md`） |

> 注：`2026-09-15.md:299` 将 G09 R1 记为 `TestG08_LegacyImmutableAndTruncateGuard`，与 `2026-09-15.md:373` 及 R4C §3.1 行 75 的 `TestBranchLegacyPrefixReorgConvergesToLongerBranch` 不一致。已在 G09 重建报告中登记为证据矛盾；两种记法下均为 PASS。

### 6.6 BT-1（`2026-09-15.md:300`）

**2 FAIL / 8**（历史 4/8）——确认既有 flaky，本阶段未触碰 blocktree；引用「全量绿灯」前必须排除该包。

---

## 7. I9 幂等性发现

证据：`2026-09-15.md:297`

崩溃残留 UNDO **不阻塞重试**——`executeReorg` 只为「已存储且缺 undo」的区块补 undo，故重复 UNDO 帧字节相同会被 `interpretScan` 去重；
生产路径 `AddBlock` 必先 `SaveBlockDetached`（`blockchain.go:489/498`）⇒ `actualNewBlocks` 恒空。

---

## 8. Newly Registered Observation

证据：`2026-09-15.md:298`

🔎 **OBS-1J-R3-A（P3，未修复）**：`CommitReorg` 无法一次提交「多枚全新区块」的分支（Phase-1 `deriveStrict` 要求父已登记，登记在 Phase 4）
⇒ 传 `[f3,f4]` 且都未存储时报 `ErrParentNotFound`。
**生产不可达**（`AddBlock` 先 detached 落盘）；失败模式 **fail-safe**（不产生半状态）。

---

## 9. Non-Actions

- ❌ 零生产代码改动（阶段标题明示「零生产改动」，`2026-09-15.md:282`）
- ❌ 未新增 crash hook（§6 明令不得新增，`2026-09-15.md:289`）
- ❌ 未 commit / push / tag / merge / rebase / amend / squash（`2026-09-15.md:284`）
- ❌ 未修 BT-1（`2026-09-15.md:300`）
- ❌ 未修 OBS-1J-R3-A（P3，登记未修，`2026-09-15.md:298`）

---

## 10. Stop Condition

```text
STOP.
建议下一阶段（不自动执行）：REORG-1J-R4 — LEGACY-LENGTH MATRIX。
```

证据：`2026-09-15.md:302`。

---

## Evidence Sources

| # | 来源 | 精确定位 | 用途 |
|---|---|---|---|
| 1 | `.workbuddy/memory/2026-09-15.md` | 行 **282–302**（章节「REORG-1J-R3 — LEGACY-PREFIX CRASH MATRIX」全段） | 阶段身份、基线、提交边界、场景、全部实测、I9、OBS-1J-R3-A、回归闸门、BT-1、STOP |
| 2 | `internal/storage/v2.go` | 行 **712–713** | 提交点 `appendFrames` 单 fsync |
| 3 | `internal/storage/v2api.go` | 行 **57–58** | `commitTipAfterAppend` |
| 4 | `internal/storage/r3_crash_matrix_test.go` | 行 **526 / 539 / 565 / 628 / 676 / 715 / 741 / 773** | C0–C6 + 字节穷举 + I10 用例位置 |
| 5 | `internal/storage/r3_crash_matrix_test.go` | 行 **168** | 场景注释「自 legacy 高度 2 分叉 ⇒ v2 区块占据 legacy 槽位（GAP-1H-A 场景）」 |
| 6 | `cmd/node/r3_crash_restart_test.go` | 行 **63** | 真实进程 kill 用例 |
| 7 | `PHASE-REORG-1J-R4C-FINAL-CLOSURE-AUDIT-REPORT.md` | §3 行 **59**；§3.1 行 **76**；§4.2 表 | 「1,818 字节级崩溃点 + 5 次真实进程 kill/restart + 40 次重复」；R3 与 R4A 的互补关系；delta 形状 780/1036 B |
| 8 | `PHASE-REORG-1J-DOC-2-EVIDENCE-ARCHIVE-CLOSURE-AUDIT-REPORT.md` | §3 表（R3 行）、§3.1（R3 十七问）、§4.1 第 16 行、§5.1、§7 | 原始报告被 R4A PRE-GATE prompt 覆盖的归档事实与重建授权 |
| 9 | `docs/_archive_pre-doc2/repo/PHASE-REORG-1J-R3-LEGACY-PREFIX-CRASH-MATRIX-FINAL-REPORT.md` | 全文件 4,392 B，SHA-256 `af6bace6647c93d43f7e53b94fb4991848c86ac63ba1c82a1404a2afcfed6102` | 被覆盖后残留正文的不可变快照 |

**文件系统事实（非阶段记录）**：`docs/prompts/REORG-R4A-PRE-GATE.prompt.md` mtime = `2026-09-15 13:21`
—— 该时刻为**覆盖 R3 报告路径**的时间，不等同于 R3 阶段执行时间窗。

---

## EVIDENCE INSUFFICIENT 字段清单

| 字段 | 状态 |
|---|---|
| 执行时间窗（起止时刻） | EVIDENCE INSUFFICIENT |
| 原报告章节结构与字节大小 | EVIDENCE INSUFFICIENT（仅知残留 prompt 副本 4,392 B） |
| C0–C6 各用例的逐条断言 | EVIDENCE INSUFFICIENT（仅存用例名与 C2/C3 等价 C1 的结论） |
| I1–I8 不变量的完整定义 | EVIDENCE INSUFFICIENT |
| delta 的确切帧构成（A=5 帧 / B=6 帧各自的帧类型序列） | EVIDENCE INSUFFICIENT（仅知 R4C §4.2 记载 A = UNDO×4 + TIP、B = UNDO×4 + BLOCK + TIP） |
| 5 次进程 kill 的逐次输出 | EVIDENCE INSUFFICIENT |
| 原报告 STOP 块逐字文本 | EVIDENCE INSUFFICIENT（仅存语义） |

---

## STOP

```text
本文件为 RECONSTRUCTED FROM EVIDENCE 归档产物。
不得作为原始执行报告引用。
```
