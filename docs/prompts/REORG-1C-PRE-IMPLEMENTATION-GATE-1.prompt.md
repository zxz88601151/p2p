PHASE REORG-1C-PRE-IMPLEMENTATION-GATE-1

STRICT READ-ONLY / CONSENSUS SAFETY / REORG EXECUTION PRE-GATE

目标：

基于已经完成并本地提交的：

- REORG-1A = 7d0e1e9
- REORG-1B = b001f57

对当前 P2PChain 进行一次严格只读的 Reorg Execution Pre-Implementation Gate Audit。

本阶段不是实现阶段。

禁止修改任何代码、测试、协议、配置、文档或生产环境。  
禁止任何 git 写操作。

==================================================  
HARD RULES
==========

本阶段：

- ZERO code modification
- ZERO test modification
- ZERO documentation modification
- ZERO config modification
- ZERO git add
- ZERO git commit
- ZERO git reset
- ZERO git checkout --worktree mutation
- ZERO git clean
- ZERO git push
- ZERO git tag
- ZERO git merge
- ZERO git rebase
- ZERO git amend
- ZERO cherry-pick
- ZERO production SSH
- ZERO production restart
- ZERO production datadir modification
- ZERO production deployment
- ZERO P2P topology mutation
- ZERO mining mutation
- ZERO blockchain production-path activation

允许：

- git status
- git log
- git show
- git diff
- git diff --cached
- git ls-files
- grep / rg
- go test
- go test -race
- go vet
- go build
- 静态代码阅读
- 只读数学/状态机推导
- 只读测试与历史分析

==================================================  
HARD BASELINE
=============

首先建立并记录：

HEAD  
HEAD^  
branch  
working tree status  
staging status  
tracked diff  
untracked files  
REORG-1A commit SHA  
REORG-1B commit SHA  
parent-child relationship

确认：

HEAD = b001f57  
parent = 7d0e1e9

确认 REORG-1A / 1B 两个 commit 均保持独立。

任何 pre-existing docs/verifier/untracked 状态不得修改。

==================================================  
AUDIT OBJECTIVE
===============

核心问题：

当前 P2PChain 是否已经具备安全实施：

OLD ACTIVE CHAIN  
↓  
find common ancestor  
↓  
disconnect old branch  
↓  
restore UTXO/state  
↓  
connect new branch  
↓  
activate new chain  
↓  
persist state

所需要的全部前置条件？

必须给出：

OPTION A = READY FOR REORG-1C IMPLEMENTATION

或

OPTION B = NOT READY / BLOCKING GAPS

或

OPTION C = READY WITH EXPLICIT PRECONDITIONS

不得因为 REORG-1B 已 PASS 就自动判定 1C READY。

==================================================

1. BLOCKCHAIN / BLOCKTREE BOUNDARY     
   ==================================================

审计：

internal/blocktree/  
internal/blockchain/

回答：

1. BlockTree 当前负责什么？
2. blockchain 当前负责什么？
3. active chain 当前真正由谁定义？
4. bestTip / bestTipWork 是否已经被生产 blockchain 使用？
5. AddBlock 是否自动触发 fork-choice？
6. competing block 到达后是否仍然不会自动 reorg？
7. BlockTree 与 production blockchain 是否存在状态分裂风险？
8. 哪个组件最终应该拥有 active-chain authority？

明确画出当前状态机：

Block received  
→ BlockTree  
→ validation  
→ fork-choice  
→ ???  
→ UTXO  
→ storage

标出当前尚未连接的边界。

==================================================  
2\. CONNECT BLOCK AUDIT
=======================

完整审计当前 ConnectBlock / block application / UTXO mutation 逻辑。

不要只搜索函数名。

追踪：

- transaction application
- input spending
- output creation
- coinbase creation
- fee handling
- transaction validation
- UTXO mutation
- indexes
- metadata
- height-dependent state
- cumulative work
- timestamps / MTP
- any cached chain state

建立：

ConnectBlock mutation inventory

格式：

STATE  
CURRENT MUTATION  
REVERSIBLE?  
UNDO DATA EXISTS?  
OWNER  
CONSENSUS CRITICAL?  
REORG IMPACT

==================================================  
3\. DISCONNECT / UNDO GAP AUDIT
===============================

回答：

当前是否存在真正的 DisconnectBlock？

如果不存在：

明确列出实现它需要恢复的全部状态。

重点：

- spent UTXOs
- newly created outputs
- coinbase outputs
- transaction-level state
- block-level state
- indexes
- cached values
- height
- active tip
- cumulative work
- MTP-related state
- any persistent metadata

判断当前设计是否可以可靠生成 undo information。

禁止在本阶段实现 undo。

==================================================  
4\. COMMON ANCESTOR / REORG PATH
================================

基于 REORG-1B 的：

- ActiveChain
- IsActiveChain
- ShouldReorg
- CompareWork
- tieBreakWinner
- ResetTip

推导真正的 reorg algorithm。

至少分析：

A → B → C → D  
  
E → F → G

以及：

A → B → C → D  
  
E → F

要求明确：

1. common ancestor 如何确定
2. disconnect 顺序
3. connect 顺序
4. active tip 何时改变
5. BlockTree tip 与 blockchain tip 是否必须原子同步
6. 中途失败怎么办

不得实现，只能审计。

==================================================  
5\. PARTIAL FAILURE / ATOMICITY AUDIT
=====================================

这是本阶段最高优先级之一。

模拟：

Case 1:  
Disconnect old block #1 success  
Disconnect old block #2 FAIL

Case 2:  
Disconnect old branch complete  
Connect new block #1 success  
Connect new block #2 FAIL

Case 3:  
Connect new branch complete  
Persist active state FAIL

Case 4:  
Process crash between disconnect/connect

Case 5:  
Process crash after UTXO mutation but before active tip persistence

Case 6:  
Restart after interrupted reorg

对每一种情况回答：

- 当前实现能否恢复？
- 是否会出现 UTXO 与 active chain 不一致？
- 是否会出现 storage 与 memory 不一致？
- 是否会出现 BlockTree 与 blockchain 不一致？
- 是否可能产生永久错误状态？
- 是否需要 rollback transaction / journal / undo stack / atomic commit？

==================================================  
6\. STORAGE AUDIT
=================

审计：

internal/storage/

明确：

- block persistence
- active-chain persistence
- block index persistence
- UTXO persistence
- delete capability
- rewrite capability
- crash recovery
- fsync semantics
- atomicity guarantees

特别确认：

“切换 active chain”是否仅需要内存更新，  
还是必须同时修改持久化状态。

不要修改 storage。

==================================================  
7\. UTXO REORG SAFETY AUDIT
===========================

审计当前 UTXO implementation。

建立：

OLD CHAIN STATE  
→ disconnect mutations  
→ COMMON ANCESTOR STATE  
→ connect mutations  
→ NEW CHAIN STATE

重点确认：

- spent output restore
- created output removal
- coinbase handling
- transaction ordering
- duplicate outputs
- same-tx dependencies
- fees
- rollback

必须明确当前 UTXO 层距离支持安全 reorg 还缺什么。

==================================================  
8\. MEMPOOL / ORPHAN IMPACT
===========================

只读审计：

internal/mempool/  
orphan-related code  
transaction admission path

回答：

reorg 后：

- old-chain transactions 是否重新进入 mempool？
- new-chain transactions 是否从 mempool 删除？
- invalidated transactions 如何处理？
- orphan blocks 是否受 active chain 改变影响？

注意：

本阶段不要实现 mempool re-add。

只判断它是否是 1C blocker，  
还是应该延后到 integration phase。

==================================================  
9\. P2P / PRODUCTION PATH AUDIT
===============================

确认：

当前 BlockTree reorg logic 是否已经被：

- P2P
- blockchain
- miner
- production node

调用。

尤其确认：

是否存在任何隐式调用 SetTip / ShouldReorg 的 production path。

最终必须证明：

REORG EXECUTION CURRENTLY OFF

==================================================  
10\. RESTART / RECOVERY AUDIT
=============================

审计：

RestartLikeReconstruction  
chain loading  
block loading  
UTXO reconstruction  
active tip reconstruction

特别处理已知：

BT-1 = pre-existing map iteration nondeterminism

不要修复 BT-1。

只判断：

BT-1 是否会影响未来 reorg recovery correctness。

如果影响，明确等级：

BLOCKING / NON-BLOCKING / INDEPENDENT REMEDIATION

==================================================  
11\. CONSENSUS SAFETY MATRIX
============================

建立完整矩阵：

Capability  
Current State  
Required for Reorg  
Status  
Blocking?

至少包括：

- fork-choice
- cumulative work
- common ancestor
- disconnect
- undo
- connect
- UTXO rollback
- storage rollback
- active tip update
- persistence
- crash recovery
- restart recovery
- mempool reconciliation
- orphan reconciliation
- P2P propagation
- miner interaction

==================================================  
12\. IMPLEMENTATION BOUNDARY
============================

最终明确：

REORG-1C 应该实现什么？

REORG-1D 应该实现什么？

REORG-1E 应该实现什么？

哪些内容必须禁止进入 1C？

尤其防止一次 commit 同时修改：

blocktree  
blockchain  
UTXO  
storage  
mempool  
P2P

如果发现阶段划分需要调整，提出最小调整方案。

==================================================  
13\. TEST PLAN GAP
==================

设计未来真正实施前必须存在的测试：

- lower-height higher-work reorg
- multi-block reorg
- competing branch
- common ancestor
- disconnect correctness
- reconnect correctness
- UTXO equivalence
- failed disconnect rollback
- failed connect rollback
- restart after reorg
- persistence consistency
- deterministic recovery
- concurrent block arrival
- production path remains isolated

不要实现。

只给测试设计和优先级。

==================================================  
14\. FINAL GATE
===============

最终只允许以下三个结论之一：

OPTION A  
READY FOR REORG-1C IMPLEMENTATION

OPTION B  
NOT READY — BLOCKING GAPS

OPTION C  
READY WITH EXPLICIT PRECONDITIONS

如果选择 B 或 C：

必须列出：

BLOCKER ID  
DESCRIPTION  
WHY BLOCKING  
REQUIRED PHASE  
MINIMUM FIX  
FORBIDDEN SCOPE

==================================================  
15\. NEXT PHASE RECOMMENDATION
==============================

给出唯一推荐下一阶段。

不要列一堆可选功能。

原则：

- 不提前堆功能
- 不自动实现
- 不修改 production path
- 不修 BT-1，除非证明其为 1C blocker
- 不提前加入 mempool/orphan/P2P reorg
- 不提前加入 mining changes
- 不提前加入 consensus unrelated changes

==================================================  
FINAL REPORT REQUIREMENTS
=========================

报告必须包含：

1. Executive Summary
2. Hard Baseline
3. Current Reorg Architecture
4. ConnectBlock Audit
5. Disconnect / Undo Gap
6. Common Ancestor Analysis
7. Atomicity / Failure Analysis
8. Storage Audit
9. UTXO Audit
10. Mempool / Orphan Audit
11. P2P / Production Isolation
12. Restart / Recovery
13. Consensus Safety Matrix
14. Blocking Gaps
15. Implementation Boundary
16. Required Test Plan
17. Final Verdict
18. Exact Next Phase Prompt

最终不得修改任何文件。

最终不得执行任何后续 phase。

STOP AFTER REPORT.







