# PHASE REORG-1F-DESIGN-1 — MEMPOOL RE-ADD / DISCONNECTED TRANSACTION AUDIT

前置状态：

- REORG-1C = CLOSED
- Commit = `369d36ec97f257ed27bf43c3b3d9b78d357ab745`
- Remote main synchronized
- REORG-1D = ALREADY IMPLEMENTED / PASS
- `BlockUndo` / `DisconnectBlock` 已存在并经过 UTXO 层测试验证
- 当前 `executeReorg` 使用 full replay；不得为了本阶段强制改成 incremental disconnect

## OBJECTIVE

审计当前 P2PChain 在 chain reorganization 后对：

**disconnected transactions / mempool resurrection**

的处理能力。

当前已知 limitation：

旧 canonical branch 上被 disconnect、但在新 canonical branch 中没有被确认的普通交易，目前不会自动重新进入 mempool。

## STRICT READ-ONLY

本阶段：

- 只读代码
- 只读测试
- 只读架构
- 不修改 production code
- 不修改 tests
- 不修改 consensus rules
- 不修改 mempool behavior
- 不 commit
- 不 push
- 不部署
- 不运行生产节点

## FIRST AUDIT

完整定位：

1. 当前 mempool implementation
2. transaction admission path
3. transaction validation path
4. transaction dependency handling
5. transaction removal on block confirmation
6. transaction removal on invalidation
7. reorg execution path
8. disconnected branch transaction availability
9. transaction identity / deduplication
10. coinbase handling
11. fee/policy validation
12. current mempool tests

必须明确：

- 哪些交易可以 resurrection
- 哪些交易绝对不能 resurrection
- 当前代码是否保存 disconnected transactions
- 如果没有保存，是否能够从 detached blocks deterministic recovery
- transaction dependency ordering
- parent-before-child restoration requirements

## REQUIRED REORG SCENARIO

至少分析：

```text
Common ancestor A

Old canonical:

A → B → C

C contains:
  tx1
  tx2 depends on tx1
  coinbase

New canonical:

A → B → D → E
```

要求设计能够正确区分：

- tx1 未被新链确认 → candidate for re-add
- tx2 未被新链确认 → candidate for re-add after dependency handling
- transaction already confirmed in D/E → MUST NOT re-add
- coinbase → MUST NOT re-add
- double-spending transaction → MUST NOT enter mempool
- invalid transaction under new UTXO → MUST reject
- duplicate transaction → MUST reject

## CONSENSUS / POLICY BOUNDARY

明确禁止：

mempool resurrection 改变 consensus validity。

必须区分：

CONSENSUS:

- transaction validity
- UTXO validity
- double spend
- coinbase rules

POLICY:

- fee
- relay policy
- mempool limits
- replacement policy
- ancestor/descendant limits

## CRASH SEMANTICS

分析：

1. reorg before mempool resurrection
2. reorg after canonical TIP commit
3. crash before re-add
4. crash during re-add
5. restart

原则：

**mempool is reconstructible / non-consensus state**

不得因为 mempool recovery failure 导致 canonical chain invalid。

## REQUIRED DESIGN INVARIANTS

至少定义：

M1 — Canonical chain transactions MUST NOT remain resurrected in mempool.

M2 — Disconnected non-coinbase transactions MAY become resurrection candidates.

M3 — Invalid transactions MUST be rejected.

M4 — Double-spending transactions MUST be rejected.

M5 — Transaction dependencies MUST be restored safely.

M6 — Mempool state MUST NOT affect consensus canonical-tip selection.

M7 — Mempool failure MUST NOT corrupt canonical chain.

M8 — Repeated reorg evaluation MUST NOT duplicate mempool entries.

M9 — Same canonical chain + same candidate transactions MUST produce deterministic acceptance behavior subject to explicitly documented policy.

## IMPORTANT SCOPE LIMIT

不要实现：

- orphan pool
- P2P branch sync
- by-hash block retrieval
- mining template redesign
- finality
- MaxReorgDepth
- reward redesign
- production deployment
- Linux mining test
- Windows mining test

这些属于后续阶段。

## FINAL REPORT

输出：

# PHASE REORG-1F-DESIGN-1 — FINAL REPORT

必须包含：

1. Current mempool architecture
2. Current reorg interaction
3. Disconnected transaction lifecycle
4. Existing capabilities
5. Missing capabilities
6. Consensus/policy boundary
7. Dependency model
8. Coinbase handling
9. Double-spend handling
10. Crash semantics
11. Required invariants
12. Test matrix
13. Proposed implementation architecture
14. Scope exclusions
15. Risks
16. Verdict

最终只能给出：

READY FOR IMPLEMENTATION

或

NOT READY

完成报告后 STOP。





