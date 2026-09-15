执行：

PHASE REORG-INFRASTRUCTURE-IMPLEMENTATION-2 — REORG-1D UTXO UNDO JOURNAL

阶段性质：  
STRICT CONTROLLED IMPLEMENTATION / CONSENSUS SAFETY / UTXO REVERSE-STATE

前置阶段：  
PHASE REORG-1C-PRE-IMPLEMENTATION-GATE-1

前置结论：  
OPTION B — NOT READY — BLOCKING GAPS

当前 HEAD：  
b001f578285a9ed1e5ee08a7067da8969d78bd60  
短哈希：b001f57

==================================================  
一、唯一目标
======

本阶段只解决：

B2 — UTXO undo / rollback

目标是建立确定性的：

ApplyBlock → UndoJournal → DisconnectBlock-state-reversal

但本阶段 NÃO 实现完整 Reorg orchestration。

必须证明：

S0  
→ ApplyBlock(B)  
→ S1 + Undo(B)  
→ DisconnectBlock(B, Undo(B))  
→ S0

并进一步证明：

S0  
→ ApplyBlock(B)  
→ S1  
→ DisconnectBlock(B, Undo(B))  
→ S0  
→ ApplyBlock(B)  
→ S1'

最终：

S1 == S1'

其中比较必须是完整 UTXO state equivalence，  
不能只比较 Balance 或 Len。

==================================================  
二、严格范围
======

允许修改：

- internal/utxo/
- 与 UTXO undo 集成直接必要的 internal/blockchain/ 调用点
- 对应 tests
- 本阶段报告

禁止修改：

- internal/storage/ 的 Reorg storage implementation
- internal/p2p/
- internal/mempool/ 的 ReaddDisconnected
- orphan pool
- production Reorg wiring
- blocktree fork-choice semantics
- Difficulty / MTP / PoW / chainwork 参数
- 已冻结 consensus parameters
- production datadir

禁止提前实现：

- 完整 ConnectBlock orchestration
- 完整 DisconnectBlock orchestration
- production SetTip
- Reorg transaction coordinator
- P2P branch synchronization
- MaxReorgDepth enforcement
- /status reorg observability

如果发现上述能力需要修改才能完成本阶段，  
立即 STOP 并报告，而不是扩大范围。

==================================================  
三、先建立 HARD BASELINE
===================

执行并记录：

git rev-parse HEAD  
git status --short  
git status --porcelain '*.go'  
git diff --check  
go vet ./...  
go build ./...  
go test ./...

确认：

HEAD == b001f57  
生产 datadir 未触碰  
工作树初始状态记录完整

任何基线异常立即 STOP。

==================================================  
四、先审计现有 UTXO 模型
===============

只读确认：

- Entry
- UTXOSet
- Add
- Get
- Spend
- Clone
- ApplyBlock
- coinbase handling
- maturity handling
- transaction ordering
- intra-block transaction dependency

明确记录：

当前 ApplyBlock 如何改变 UTXO state；  
哪些信息在 Spend 后会永久丢失；  
Undo Journal 必须保存哪些信息才能完整恢复。

不要先写代码再决定数据模型。

==================================================  
五、Undo Journal 数据模型
===================

设计一个最小、确定性、可持久化友好的 BlockUndo / UndoJournal。

原则：

1. 必须保存被 Spend 删除的完整原始 UTXO Entry。
2. 必须记录本 Block 创建的所有 Outpoint。
3. Disconnect 时必须能够仅依赖：
   - prior UTXO state
   - block
   - UndoJournal       
     完整恢复 block 执行前状态。
4. Undo 数据不能依赖已经消失的临时内存状态。
5. 不允许通过重新执行整个历史链实现 undo。
6. 不允许保存整个 UTXO snapshot 作为 block undo。
7. 顺序必须确定性。
8. Disconnect 必须按照正确逆序恢复状态。
9. coinbase creation 必须能够被反向删除。
10. 普通 transaction spend 必须能够恢复完整 Entry。
11. 同区块 transaction dependency 必须正确处理。

==================================================  
六、核心实现
======

实现：

ApplyBlock + UndoJournal generation

以及：

DisconnectBlock / ReverseApply

但如果当前架构中完整 DisconnectBlock 会自然跨入  
REORG-1C orchestration，  
则只实现 UTXO 层 reverse primitive，  
不要提前建立完整 blockchain reorg coordinator。

推荐抽象方向：

ApplyBlock(...) -> (newSet, undo, error)

以及：

DisconnectBlock(...) -> (restoredSet, error)

具体 API 名称必须根据当前代码结构决定，  
不要机械照抄名称。

要求：

- forward failure 不污染原 state
- reverse failure 不产生静默部分状态
- 不允许 panic 作为正常错误处理
- 不允许忽略 undo mismatch
- 不允许静默删除不存在的 outpoint
- 不允许静默恢复已存在且冲突的 outpoint

==================================================  
七、必须测试的矩阵
=========

至少实现并通过：

1. TestUndoCoinbaseCreation

验证：  
Apply coinbase  
→ Disconnect  
→ coinbase output 完全消失

1. TestUndoTxSpend

验证：  
原 UTXO  
→ spend  
→ disconnect  
→ 原 Entry 完整恢复

1. TestUndoMaturityRecompute

验证：  
currentHeight 改变后，  
coinbase maturity 语义保持正确。

1. TestRoundTripApplyDisconnectApply

必须证明：

S1 == S1'

使用完整 UTXO state comparison。

1. TestUndoMidChain

验证中间 block undo 所需前置条件；  
如果完整链级 mid-chain disconnect 属于 REORG-1C，  
则在 UTXO 层明确验证 reverse primitive，  
不得提前实现完整 reorg。

1. TestDoubleSpendCrossBranch

验证两个 branch 使用同一原始 UTXO 时，  
undo A 不会破坏 branch B 所需状态。

1. TestUndoLogPersistence

如果当前阶段无法安全进入 storage persistence，  
则不要实现 storage；  
但必须明确设计：  
UndoJournal 可以被 deterministic encode/decode，  
并在报告中记录为何 persistence implementation 留给 REORG-1E。

==================================================  
八、强制新增的高级测试
===========

增加：

TestUndoJournalCompleteness

要求：  
UndoJournal 本身足够恢复 block 前 state。

TestUndoMultiTransactionBlock

一个 block 至少包含：

- coinbase
- independent spend
- dependent transaction
- transaction-created-output 被同区块后续 tx 消费

然后完整 reverse。

TestUndoExactStateEquality

不得只比较：  
Balance  
Len  
单个 Get

必须比较：  
全部 Outpoint  
全部 Entry 字段  
确定性排序后的完整集合

TestUndoDeterminism

相同：  
prior state + block

必须产生完全相同的 UndoJournal。

TestUndoRejectsCorruption

人为篡改 undo：

- 缺少 entry
- 重复 created outpoint
- 恢复冲突 outpoint

必须明确失败，  
不能静默继续。

==================================================  
九、共识安全要求
========

本阶段绝不改变：

- Difficulty
- MTP
- PoW
- chainwork
- version
- block validation order
- consensus constants

Undo 必须是 consensus-state-preserving reverse operation。

禁止通过：

- skip validation
- skipPoW
- special test-only production path
- height hack
- balance shortcut

规避真实状态恢复。

==================================================  
十、Reorg OFF 必须保持
================

完成本阶段后：

REORG EXECUTION 必须仍然 OFF。

确认：

- blocktree.SetTip 仍无生产调用者
- ShouldReorg 仍无生产调用者
- AddBlock 不自动切换 bestTip
- 当前生产 OnNewBlock 不因本阶段而获得完整 reorg 能力

如果本阶段导致 production reorg path 意外开启：

STOP。

==================================================  
十一、完整回归
=======

最终必须执行：

go test ./...  
go vet ./...  
go build ./...  
git diff --check

并记录：

- 新增测试数量
- 全部测试结果
- 是否存在 flaky
- 是否有 pre-existing failure
- 是否有 race / concurrency caveat

如果适用，额外运行：

go test -race ./...

==================================================  
十二、Git 边界
=========

本阶段执行过程中：

允许：

- 修改代码
- 修改测试
- 修改本阶段报告

禁止：

- git push
- git tag
- git merge
- git rebase
- git reset
- git amend
- git squash

除非本阶段结束后我明确授权，  
不要自动 commit。

如果实施阶段被明确要求 commit，  
必须单独进入 commit-readiness / commit-execution。

==================================================  
十三、最终报告必须回答
===========

1. Undo Journal 最终数据模型是什么？
2. 为什么它能够完整恢复原 UTXO state？
3. 如何处理同区块 transaction dependency？
4. 如何处理 coinbase？
5. 如何处理 maturity？
6. Disconnect 顺序是什么？
7. Undo corruption 如何被检测？
8. Apply → Disconnect → Apply 是否严格 state-identical？
9. 是否存在任何 production reorg path 被意外打开？
10. 是否为 REORG-1C 提供了真正可用的 UTXO reverse primitive？
11. 哪些内容明确留给 REORG-1E / 1F / 1G / 1H / 1C？
12. 是否仍满足当前 consensus safety boundary？

最终给出：

VERDICT =  
PASS  
或  
PASS WITH LIMITATIONS  
或  
BLOCKED

并列出：

- implemented
- tested
- deferred
- remaining blockers
- exact recommended next phase

==================================================  
HARD STOP
=========

任何以下情况立即 STOP：

- 发现 UTXO 模型不足以保证 deterministic undo
- 需要修改 consensus parameters
- 需要修改 storage 才能完成核心 UTXO reverse
- 需要提前接入 production Reorg
- 发现已有状态不能安全恢复
- 测试只能通过放宽验证规则
- go test ./... 出现无法解释的 regression
- production datadir 被触碰
- 需要扩大到 REORG-1C 范围

不要为了得到 PASS 而降低验收标准。

目标不是“实现一个 undo 函数”。

目标是：

建立一个经过测试证明的、  
确定性的、  
可逆的、  
不会破坏现有 consensus state 的  
UTXO reverse-state foundation，

作为未来 REORG-1C 的硬前置。

STOP AFTER FINAL REPORT。















