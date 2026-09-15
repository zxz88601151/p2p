# PHASE REORG-1D — UTXO DISCONNECT / UNDO INFRASTRUCTURE DESIGN & IMPLEMENTATION

前置阶段：

- REORG-1C-PRE-GATE-1 = CLOSED
- REORG-1C-COMMIT-MODEL-DESIGN-1 = MODEL A
- REORG-1C-IMPLEMENTATION-1 = PASS
- REORG-1C-COMMIT-1 = PASS
- REORG-1C-PUSH-1 = PASS
- Remote main = `369d36ec97f257ed27bf43c3b3d9b78d357ab745`

当前正式进入：

**REORG-1D — UTXO Disconnect / Undo Infrastructure**

## PRIMARY OBJECTIVE

解决此前 REORG-1C-PRE-GATE 中确认的核心 blocker：

**B-1 — production DisconnectBlock / UndoBlock 缺失。**

建立可用于真实 REORG execution 的：

- DisconnectBlock / UndoBlock
- UTXO reverse-state
- deterministic rollback
- old-chain state restoration
- connect/disconnect correctness invariants

## HARD SCOPE

本阶段只处理：

1. Undo data model
2. DisconnectBlock
3. UTXO reverse-state correctness
4. Connect → Disconnect round-trip
5. Crash/restart correctness
6. Deterministic state restoration
7. 必要的 consensus safety tests

## DO NOT IMPLEMENT

禁止提前加入：

- P2P branch sync
- orphan pool
- mempool transaction resurrection
- MaxReorgDepth
- finality
- mining reward redesign
- mining pool / Stratum
- production deployment
- Linux production mining test
- Windows mining test
- unrelated consensus changes

## FIRST STEP

先做严格 READ-ONLY baseline audit：

- 当前 HEAD / remote
- working tree / staging
- 现有 UTXO implementation
- ApplyBlockWithUndo
- DisconnectBlock / UndoBlock 是否已有部分基础
- 当前 reorg execution path
- 当前 storage persistence model
- 现有 tests
- 1C 已实现的 replay fallback

明确区分：

**已有能力 / 缺失能力 / 可以复用 / 必须新增**

不要直接修改代码。

## DESIGN REQUIREMENTS

重点证明：

Old Chain:

A → B → C

Reorg:

A → B → C  
↘ D → E

执行：

1. Disconnect C
2. Restore UTXO/state to B
3. Connect D
4. Connect E
5. New tip = E

必须满足：

- disconnect 后状态等价于 ancestor state
- connect 后状态等价于从 ancestor 全量 replay
- 相同 storage + 相同 chain → 相同 state
- crash/restart 不产生不可验证的 canonical state
- disconnected branch 不被错误标记为 canonical

## IMPORTANT

不要因为 1C 已经 PASS 就扩大范围。

本阶段首先输出：

# PHASE REORG-1D-DESIGN-1 — FINAL REPORT

报告必须包含：

1. Baseline
2. Existing UTXO capabilities
3. Existing undo capabilities
4. Missing safety guarantees
5. Proposed undo model
6. Disconnect state machine
7. Crash semantics
8. Required invariants
9. Test matrix
10. Implementation plan
11. Scope exclusions
12. Verdict

只有完成设计审计并明确 READY 后，才进入受控 implementation。

STOP after the design report.

