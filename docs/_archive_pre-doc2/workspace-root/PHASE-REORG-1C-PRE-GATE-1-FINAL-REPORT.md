PHASE REORG-1C-COMMIT-MODEL-DESIGN-1

P2PChain — Reorg Canonical Commit Model / Crash Semantics Design Audit

PHASE TYPE:  
STRICT READ-ONLY  
CONSENSUS SAFETY  
ARCHITECTURE DESIGN  
NO IMPLEMENTATION

CURRENT HEAD:  
6ed1e836d1a6ab0f7ee2e3960529362d95226901

PREVIOUS PHASE:  
REORG-1C-PRE-GATE-1

PREVIOUS VERDICT:  
NOT READY

PURPOSE:

Before any REORG AddBlock wiring is implemented, determine and freeze  
the canonical persistence / crash-consistency model for Reorg.

The goal is to prevent implementation of a normal-path-only Reorg  
whose crash semantics are undefined.

==================================================  
HARD RULES
==========

STRICT READ-ONLY.

DO NOT:

- modify Go code
- modify tests
- modify configuration
- modify protocol parameters
- create commits
- push
- tag
- amend
- rebase
- merge
- reset
- touch production datadir
- operate production nodes
- modify AddBlock
- modify SetTip
- modify DisconnectBlock
- modify blocktree
- modify storage implementation
- modify UTXO implementation

You MAY:

- inspect all relevant source
- inspect existing tests
- model state transitions
- inspect storage frame semantics
- inspect UTXO undo semantics
- inspect blocktree semantics
- construct crash matrices
- reason about persistence ordering
- compare alternative architectures

==================================================  
PRIMARY QUESTION
================

Determine which canonical Reorg commit model should be used:

MODEL A:  
Atomic Canonical-TIP Commit

MODEL B:  
Persistent Reorg Journal

Do not choose based on simplicity alone.

The decision must be based on actual current P2PChain  
Storage / UTXO / BlockTree capabilities.

==================================================  
MODEL A AUDIT
=============

Evaluate whether the following model is actually implementable  
with the current Storage v2 design:

old canonical state  
↓  
find common ancestor  
↓  
disconnect old branch in memory  
↓  
apply new branch in memory  
↓  
validate resulting state  
↓  
persist required BLOCK + UNDO records  
↓  
single canonical TIP commit  
↓  
update in-memory active state

Determine whether the TIP frame can act as the sole canonical  
commit marker for an entire Reorg.

Specifically audit:

- multiple disconnected blocks
- multiple newly canonical blocks
- existing detached blocks
- UNDO persistence
- BLOCK persistence
- TIP persistence
- fsync ordering
- partial writes
- restart replay
- duplicate frames
- conflicting frames
- old TIP retention
- new TIP commit
- crash immediately before TIP
- crash immediately after TIP

==================================================  
MODEL B AUDIT
=============

Evaluate whether a persistent REORG journal is actually necessary.

If required, determine minimum protocol/state:

REORG_BEGIN  
REORG_PROGRESS  
REORG_COMMIT

Determine:

- required fields
- old tip
- new tip
- common ancestor
- disconnect list
- apply list
- progress index
- integrity checksum
- restart behavior
- rollback semantics
- resume semantics
- journal cleanup
- crash points

Do NOT implement it.

==================================================  
CRITICAL QUESTION — UTXO
========================

Determine whether UTXO can safely remain memory-only during the  
entire Reorg and be committed only indirectly through canonical  
storage + TIP.

Verify:

- UTXO rebuild from canonical storage
- old branch rollback
- new branch application
- crash before TIP
- crash after TIP
- restart reconstruction

Determine whether UTXO persistence is independently required.

==================================================  
CRITICAL QUESTION — DETACHED BLOCKS
===================================

Analyze this state:

Storage contains:

OLD CANONICAL:  
A → B → C

DETACHED:  
A → X → Y → Z

TIP = C

Now Reorg selects Z.

Analyze every persistence order.

Determine whether:

1. X/Y/Z already existing as detached blocks
2. new UNDO frames
3. new TIP

can safely coexist before canonical commit.

==================================================  
CRASH MATRIX
============

Produce a complete matrix for at least:

1. before disconnect
2. after disconnect
3. after UTXO rollback
4. after first new block apply
5. after complete new branch apply
6. after first persistence write
7. after all BLOCK writes
8. after all UNDO writes
9. immediately before TIP
10. during TIP write
11. immediately after TIP
12. before in-memory tip update
13. after in-memory tip update
14. restart

For each:

- canonical persistent state
- detached persistent state
- UTXO memory state
- restart state
- recovery action
- canonical result
- data loss possibility
- duplicate possibility
- consensus divergence possibility

==================================================  
ATOMICITY REQUIREMENT
=====================

Determine whether the following invariant can be guaranteed:

INVARIANT R1:

At every crash boundary, restart reconstructs exactly one  
canonical chain determined by the last valid committed TIP.

INVARIANT R2:

Detached blocks may exist, but cannot become canonical without  
a valid committed TIP.

INVARIANT R3:

UTXO reconstructed from canonical storage always corresponds  
exactly to the committed canonical TIP.

INVARIANT R4:

A partially persisted Reorg cannot produce a canonical chain  
that was never fully validated.

INVARIANT R5:

The same persisted state produces the same active tip and UTXO  
after restart.

If any invariant cannot currently be guaranteed, identify the  
minimum architectural change required.

==================================================  
IMPORTANT DISTINCTION
=====================

Separate these concepts:

1. crash recovery
2. crash resume
3. crash rollback
4. canonical atomicity
5. operation idempotence

Do not automatically assume that "not crash-resumable" means  
"unsafe".

Determine whether rollback-to-old-TIP is sufficient.

==================================================  
DECISION
========

Return:

MODEL A — ACCEPTED

MODEL B — REQUIRED

MODEL A — ACCEPTED WITH MODIFICATIONS

or

NEITHER — ADDITIONAL STORAGE/CONSENSUS DESIGN REQUIRED

Then explain why.

==================================================  
IMPLEMENTATION CONTRACT
=======================

If MODEL A is selected, produce a precise implementation contract:

- what gets persisted
- what remains memory-only
- exact persistence order
- exact TIP semantics
- exact crash semantics
- exact restart semantics
- required locking/serialization
- required validation boundaries

If MODEL B is selected, produce the minimum journal contract.

Do not implement either model.

==================================================  
FINAL GATE
==========

Answer:

1. Is Reorg canonical state atomic with current Storage v2?
2. Can TIP alone be the canonical commit marker?
3. Can UTXO remain memory-only during Reorg?
4. Can detached blocks safely exist before canonical commit?
5. Is a persistent Reorg journal actually necessary?
6. Is crash-resume necessary, or is crash-rollback sufficient?
7. Can R1–R5 be proven?
8. Is the architecture now sufficiently specified for     
   REORG-1C-IMPLEMENTATION-1?

FINAL VERDICT:

READY FOR IMPLEMENTATION  
or  
NOT READY

No code changes.  
No commits.  
No production operations.  
Do not automatically start implementation after the report.
























