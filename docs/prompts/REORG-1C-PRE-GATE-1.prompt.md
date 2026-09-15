PHASE REORG-1C-PRE-GATE-1

P2PChain — REORG EXECUTION / CONSENSUS INTEGRATION SAFETY PRE-GATE

PHASE TYPE:  
STRICT READ-ONLY  
CONSENSUS SAFETY  
REORG EXECUTION INTEGRATION AUDIT

CURRENT BASELINE:

REORG-1E-IMPLEMENTATION-1 = PASS  
REORG-1E-COMMIT-1 = PASS  
REORG-1E-PUSH-1 = PASS

CURRENT COMMITTED HEAD:  
6ed1e836d1a6ab0f7ee2e3960529362d95226901

REMOTE:  
gitea/main == 6ed1e836d1a6ab0f7ee2e3960529362d95226901

IMPORTANT:  
Storage v2 and UTXO Undo Journal are now committed and pushed,  
but REORG EXECUTION WIRING IS NOT ENABLED.

The purpose of this phase is NOT implementation.

It is to determine whether the existing architecture is actually  
READY for controlled Reorg integration.

==================================================  
HARD RULES
==========

READ-ONLY ONLY.

DO NOT:

- modify .go files
- modify tests
- modify configuration
- modify protocol parameters
- modify documentation
- create commits
- push
- tag
- amend
- rebase
- merge
- reset
- touch production datadir
- operate production nodes
- call SetTip as part of an implementation
- wire Reorg into AddBlock
- enable production Reorg
- alter existing test assertions

You MAY:

- read source
- read tests
- inspect call graphs
- inspect existing storage implementation
- inspect blocktree implementation
- inspect UTXO implementation
- inspect blockchain AddBlock path
- perform safe isolated/read-only reasoning
- run existing tests if they are non-mutating
- construct state-transition models
- construct crash/recovery matrices
- identify blocking gaps

==================================================  
PRIMARY AUDIT PATH
==================

Audit the complete intended path:

AddBlock  
↓  
block/fork detection  
↓  
candidate chain selection  
↓  
cumulative chainwork comparison  
↓  
common ancestor discovery  
↓  
old canonical branch identification  
↓  
DisconnectBlock  
↓  
UTXO Undo Journal  
↓  
new branch validation  
↓  
ApplyBlock / UTXO transition  
↓  
storage canonical persistence  
↓  
active tip update  
↓  
restart reconstruction

Do not assume any step is safe merely because the underlying  
component passed its own unit tests.

==================================================  
AUDIT 1 — STATE MACHINE
=======================

Construct the actual current state machine.

Identify:

- active block
- active tip
- canonical storage tip
- blocktree tip
- UTXO state tip
- persisted tip metadata
- undo journal state

Determine whether each is:

- authoritative
- derived
- cached
- persisted
- reconstructable

Explicitly identify every possible state where:

blocktree tip  
!= storage tip  
!= UTXO tip

could occur.

Classify whether such divergence is:

SAFE / RECOVERABLE / CONSENSUS-CRITICAL.

==================================================  
AUDIT 2 — REORG ORDERING
========================

Determine the only safe ordering for:

1. common ancestor selection
2. disconnect
3. UTXO rollback
4. new branch application
5. canonical storage persistence
6. active tip update

For each ordering decision explain:

- why it is safe
- what happens on crash immediately afterward
- what restart observes
- whether the operation can be retried safely

==================================================  
AUDIT 3 — ATOMICITY / CRASH MATRIX
==================================

Build a crash matrix.

At minimum analyze crashes:

A. before first disconnect  
B. after disconnect journal write  
C. after UTXO rollback  
D. after final old-branch disconnect  
E. after first new-branch application  
F. after partial new-branch application  
G. after canonical block append  
H. before active tip update  
I. after active tip update  
J. immediately before process shutdown

For every point answer:

- persisted state
- in-memory state
- restart behavior
- recovery mechanism
- possibility of permanent divergence
- possibility of duplicate application
- possibility of duplicate disconnect
- consensus safety

==================================================  
AUDIT 4 — UNDO JOURNAL SAFETY
=============================

Verify that Undo records are safely associated with the exact block  
being disconnected.

Check:

- block hash binding
- parent hash binding
- height binding
- transaction/input identity
- output identity
- ordering
- corruption detection
- truncation detection
- duplicate application protection
- wrong-block Undo rejection
- restart behavior

Pay special attention to:

"valid Undo data for block A accidentally being accepted for block B"

==================================================  
AUDIT 5 — STORAGE SAFETY
========================

Audit Storage v2:

- framing
- CRC/checksum
- sequence
- block identity
- canonical interpretation
- recovery
- partial frame
- torn write
- trailing garbage
- duplicate block
- conflicting block
- restart scan
- active tip reconstruction

Determine whether Storage v2 is sufficient for canonical Reorg persistence  
or whether additional integration metadata is required.

==================================================  
AUDIT 6 — BLOCKTREE / CHAINWORK
===============================

Verify:

- competing branch selection
- cumulative chainwork
- stale branch handling
- equal-work handling
- common ancestor
- descendant traversal
- active tip transition
- restart reconstruction

Check that the selected branch cannot differ between:

- in-memory execution
- restart reconstruction
- another node

==================================================  
AUDIT 7 — ADD BLOCK INTEGRATION
===============================

Read the actual AddBlock implementation.

Determine exactly where Reorg logic would need to enter.

Do NOT modify it.

Produce an explicit proposed integration boundary:

CURRENT:

AddBlock  
→ existing behavior

TARGET:

AddBlock  
→ block validation  
→ blocktree update  
→ fork detection  
→ chainwork decision  
→ optional Reorg execution  
→ canonical state commit

Identify all places where introducing this path could change  
existing consensus behavior.

==================================================  
AUDIT 8 — IDEMPOTENCE
=====================

Analyze:

Disconnect  
→ crash  
→ restart  
→ retry

and:

Apply  
→ crash  
→ restart  
→ retry

Determine whether each operation is:

IDEMPOTENT  
NON-IDEMPOTENT BUT RECOVERABLE  
UNSAFE

Do not assume retries are safe.

==================================================  
AUDIT 9 — CONSENSUS SPLIT-BRAIN
===============================

Determine whether two nodes receiving the same competing branches  
could produce different:

- selected tip
- UTXO state
- canonical storage
- restart result

because of:

- ordering
- persistence timing
- incomplete metadata
- recovery behavior
- chainwork calculation
- blocktree reconstruction

Any such possibility is a BLOCKING GAP.

==================================================  
AUDIT 10 — TEST COVERAGE
========================

Map existing tests to:

- fork detection
- chainwork selection
- common ancestor
- disconnect
- undo
- storage persistence
- crash recovery
- restart
- competing branches
- duplicate blocks
- corrupted undo
- corrupted storage
- partial writes
- full Reorg integration

Clearly distinguish:

PROVEN  
PARTIALLY PROVEN  
NOT PROVEN

Do not treat unit-test coverage of isolated components as proof  
of integration safety.

==================================================  
MANDATORY FINAL QUESTIONS
=========================

Answer explicitly:

1. Can AddBlock safely invoke Reorg today?     
   YES / NO
2. Is the current Storage v2 sufficient for canonical Reorg persistence?     
   YES / NO / CONDITIONAL
3. Is the UTXO Undo Journal sufficient for production Reorg execution?     
   YES / NO / CONDITIONAL
4. Is crash-safe Reorg execution proven?     
   YES / NO
5. Is restart-safe Reorg convergence proven?     
   YES / NO
6. Is multi-node deterministic Reorg behavior proven?     
   YES / NO
7. Is there any possible blocktree/storage/UTXO split-brain?     
   YES / NO
8. Is another implementation phase required before AddBlock wiring?     
   YES / NO

==================================================  
FINAL VERDICT
=============

Return exactly one:

READY

NOT READY

BLOCKED

If NOT READY or BLOCKED:

List every blocking gap.

Separate:

P0 — Consensus safety blocker  
P1 — Reorg correctness blocker  
P2 — Recovery / durability gap  
P3 — Test / observability gap

For each gap provide:

- evidence
- affected component
- failure scenario
- required remediation
- whether implementation is required
- whether another design audit is required

IMPORTANT:

Do not implement any remediation in this phase.

The purpose of this phase is to determine whether the architecture  
is safe enough to authorize the next controlled implementation phase.

Do not automatically begin the next phase after the report.
