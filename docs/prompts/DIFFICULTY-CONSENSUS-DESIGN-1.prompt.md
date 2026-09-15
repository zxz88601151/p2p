PHASE DIFFICULTY-CONSENSUS-DESIGN-1

DIFFICULTY / MTP / RETARGET / MAX-DIFFICULTY /  
CHAINWORK CONSENSUS DESIGN AUDIT

==================================================  
PHASE TYPE
==========

STRICT READ-ONLY  
CONSENSUS DESIGN / SECURITY AUDIT

本阶段只允许：

- 阅读源码
- 阅读现有测试
- 阅读现有协议文档
- 数学推导
- 构造攻击模型
- disposable / read-only 实验
- consensus state-machine analysis

禁止：

- 修改任何 .go
- 修改任何测试
- 修改任何 .md
- 修改配置
- 修改 genesis
- 修改 difficulty 参数
- 修改生产 datadir
- 重启 / kill 生产节点
- 修改 P2P 行为
- 修改 mining behavior
- git add
- git commit
- git push
- git tag
- amend
- rebase
- squash
- merge

如果发现 baseline .go 漂移：

立即 STOP。

==================================================  
HARD BASELINE
=============

当前基线：

HEAD:  
674c5ad076ed4857a158d6ecd7c73d58e1a61553

HEAD~1:  
91bfd79dd8fbb32cea6e6d1920b110c79105cc20

REORG-1A：  
尚未 commit。

当前新增：

internal/blocktree/blocktree.go  
internal/blocktree/blocktree_test.go

注意：

REORG-1A 当前只是未跟踪的内存基础设施。

不得因为本阶段需要而自动 commit。

==================================================  
PRIMARY QUESTION
================

回答唯一核心问题：

在不破坏现有 P2PChain 共识确定性的前提下，

如何把当前：

bits = 16 hard clamp

演化为：

真正具有 cumulative-work 安全意义的  
动态 difficulty consensus？

必须证明：

不同 difficulty 历史上的：

CumulativeWork  
仍然可以作为唯一 fork-choice work metric。

==================================================  
CURRENT POW MODEL
=================

当前模型：

bits = exponent

target = 2^(256-bits)

Work(bits) = 2^bits

CumulativeWork =  
parent.CumulativeWork + Work(bits)

当前 difficulty 存在：

bits=16 限制 / clamp。

必须确认：

1. 当前 AdjustBits 实际行为
2. 当前 bits 合法范围
3. 当前 retarget 是否存在
4. 当前 timestamp 使用方式
5. 当前 block validation 对 bits 的要求
6. genesis bits
7. consensus-critical difficulty assumptions

不得假设源码行为。

必须逐项引用实际代码位置。

==================================================  
FC-004 ANALYSIS
===============

重点验证：

当前 bits=16 clamp 是否导致：

Work(bits) = constant

从而：

CumulativeWork ≈ block height × constant

如果成立：

必须严格证明。

同时分析：

攻击者是否可以通过：

- timestamp manipulation
- fork-specific difficulty
- private mining
- short-chain retarget
- long-chain retarget
- reorg across difficulty boundary

获得异常 cumulative work。

==================================================  
MTP DESIGN
==========

必须设计并比较至少：

Option A:  
previous block timestamp

Option B:  
Median-Time-Past

Option C:  
bounded median / rolling median

必须分析：

- timestamp manipulation
- future timestamp
- backward timestamp
- reorg determinism
- multi-node convergence
- difficulty retarget interaction

必须给出明确推荐方案。

不要因为 Bitcoin 使用 MTP  
就直接复制 Bitcoin。

必须证明该方案适合当前 P2PChain。

==================================================  
RETARGET WINDOW
===============

必须分析：

- retarget interval
- target block interval
- observed elapsed time
- minimum adjustment
- maximum adjustment
- integer rounding
- boundary behavior

必须考虑：

极短时间出块  
极长时间不出块  
突然算力增加  
突然算力下降  
攻击者短期算力租赁  
私链 / 私挖后重新连接

==================================================  
MAX DIFFICULTY
==============

必须重点分析：

MaxDifficultyBits

包括：

- 是否需要
- 如何定义
- 初始值
- 为什么安全
- 是否 consensus-critical
- 是否需要 hard fork
- 与 genesis bits 的关系
- 与 Work() 数学关系
- 是否可能被滥用制造异常 chainwork

必须区分：

minimum difficulty  
maximum difficulty  
maximum work per block

不得把三个概念混淆。

==================================================  
DIFFICULTY TRANSITION
=====================

必须设计：

旧规则  
→ 新规则

的边界。

必须明确：

- activation height
- fork height
- version signaling 是否需要
- genesis / historical blocks 是否保持兼容
- activation 前后 bits 验证规则
- activation 边界的 timestamp / MTP
- activation 边界的 chainwork
- reorg crossing activation boundary

必须证明：

同一合法区块历史，  
所有节点可以确定性计算相同：

bits  
Work  
CumulativeWork

==================================================  
CHAINWORK SECURITY
==================

必须重新进行：

POW → Difficulty → Work → Chainwork → Fork Choice

完整数学验证。

重点证明：

如果：

bits_1 != bits_2

那么：

Σ 2^bits_i

仍然是正确的 cumulative work。

分析：

是否存在某种 difficulty path：

较低真实安全成本  
却产生较高 cumulative work。

如果存在：

必须 STOP 并登记 BLOCKING GAP。

==================================================  
REORG INTERACTION
=================

必须分析：

difficulty adjustment + reorg

组合状态机。

至少分析：

A:  
active chain

B:  
candidate chain

C:  
candidate crosses retarget boundary

D:  
candidate has different timestamp history

E:  
candidate has different difficulty history

F:  
candidate has higher cumulative work

必须证明：

fork-choice 只依赖：

locally validated blocks  
\+  
locally computed chainwork  
\+  
frozen tie-break

远端宣称的 difficulty / cumulative work  
永远不能直接进入 consensus decision。

==================================================  
ATTACK MODEL
============

至少建立：

1. timestamp attack
2. selfish mining difficulty attack
3. private-chain difficulty attack
4. low-work/high-chainwork illusion attack
5. retarget boundary attack
6. reorg-across-retarget attack
7. oscillating difficulty attack
8. sudden hash-rate spike
9. sudden hash-rate collapse
10. stale-node / old-rule attack

每项必须给：

攻击条件  
攻击步骤  
成功条件  
当前系统结果  
新设计结果  
是否存在 consensus divergence

==================================================  
DETERMINISM
===========

必须证明：

同一历史区块序列：

任何节点独立计算得到完全一致：

ExpectedBits  
Target  
Work  
CumulativeWork

不能依赖：

- wall clock
- local configuration
- peer-reported work
- peer-reported difficulty
- local policy

==================================================  
POLICY VS CONSENSUS
===================

明确分类：

CONSENSUS-CRITICAL：

- bits validity
- target calculation
- MTP
- retarget
- max/min difficulty
- Work
- CumulativeWork

POLICY ONLY：

- warning
- rate limit
- deep reorg alert
- mining pause
- local monitoring

必须确认：

不同节点 policy  
不会产生不同 consensus chain。

==================================================  
HARD FORK REQUIREMENT
=====================

必须回答：

是否必须 hard fork？

如果需要：

为什么？

如果不需要：

为什么？

不得模糊回答。

必须给：

Option A  
Option B  
Option C

并选出唯一推荐方案。

==================================================  
REQUIRED OUTPUT
===============

最终生成：

PHASE DIFFICULTY-CONSENSUS-DESIGN-1 FINAL REPORT

结构：

1. HARD BASELINE
2. CURRENT DIFFICULTY MODEL
3. FC-004 FORMAL PROOF
4. MTP ANALYSIS
5. RETARGET OPTIONS
6. MAX DIFFICULTY ANALYSIS
7. CHAINWORK SECURITY ANALYSIS
8. RETARGET ATTACK ANALYSIS
9. REORG INTERACTION
10. DETERMINISM PROOF
11. POLICY VS CONSENSUS
12. HARD FORK ANALYSIS
13. FINAL CONSENSUS CONTRACT
14. IMPLEMENTATION WBS
15. BLOCKING GAPS
16. NON-BLOCKING GAPS
17. RECOMMENDED NEXT PHASE

==================================================  
FINAL VERDICT
=============

只能输出：

PASS

PASS WITH CONSENSUS GAPS

或

BLOCKED

如果 PASS：

必须明确：

DIFFICULTY-CONSENSUS-DESIGN-1 COMPLETE.

如果存在任何无法证明：

“不同节点得到相同 difficulty / work / chainwork”

则：

BLOCKED。

==================================================  
IMPORTANT
=========

本阶段只做设计。

不得实现 difficulty。

不得解除 bits=16 clamp。

不得修改 pow.go。

不得修改 consensus parameters。

不得启用新的 difficulty。

不得进入生产。

不得自动启动 REORG-1B。

REORG-1B 必须等待本阶段完成并获得新的明确授权。

