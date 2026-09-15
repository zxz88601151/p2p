PHASE REORG-1A — BLOCKNODE / BLOCK TREE INFRASTRUCTURE IMPLEMENTATION

阶段性质：  
IMPLEMENTATION + UNIT TEST + READ-ONLY PRODUCTION BOUNDARY

前置：  
PHASE REORG-INFRASTRUCTURE-PRE-IMPLEMENTATION-GATE-1  
VERDICT = READY FOR REORG-1A

重要：  
本阶段仅在用户明确授权后执行。  
未授权前不得修改任何文件。

==================================================  
HARD RULES
==========

1. 本阶段只实现 REORG-1A。
2. 不实现任何后续 REORG 阶段功能。
3. 不改变既有 blocks.dat 格式。
4. 不改变既有共识参数。
5. 不启用 production reorg。
6. reorg feature flag 必须保持 OFF。
7. 不修改生产 datadir。
8. 不重启 / kill 生产节点。
9. 不执行生产 SetTip / Disconnect / Connect。
10. 不修改 difficulty / AdjustBits / float difficulty。
11. 不修改现有 P2P 行为。
12. 不实现 P2P reorg synchronization。
13. 不实现 MaxReorgDepth enforcement。
14. 不实现 common ancestor。
15. 不实现 BlockUndo。
16. 不实现 atomic reorg commit。
17. 不实现 crash recovery。
18. 不实现 fork-choice execution。
19. 不执行 git commit / push / tag / amend / rebase / squash / merge，      
    除非本阶段结束后另行授权。
20. 若发现 baseline .go 漂移，立即 STOP，并报告 BASELINE INVALID。

==================================================  
BASELINE
========

必须首先重新确认：

HEAD:  
674c5ad076ed4857a158d6ecd7c73d58e1a61553

HEAD~1:  
91bfd79dd8fbb32cea6e6d1920b110c79105cc20

branch:  
main

Go:  
go1.22.12 windows/amd64

执行只读 baseline：

git rev-parse HEAD  
git rev-parse HEAD~1  
git status --porcelain  
git status --porcelain '*.go'  
git diff-index --name-only HEAD -- '*.go'  
git diff --check

如果发现任何 .go 漂移：  
立即 STOP。  
不得继续实现。

==================================================  
REORG-1A SCOPE
==============

本阶段目标：

建立最小、确定性、可测试的 BlockNode / Block Tree  
基础设施。

需要支持：

1. BlockNode 基础结构

至少表达：

- Hash
- ParentHash
- Height
- Bits
- Timestamp
- Work
- CumulativeWork
- Parent pointer
- Children collection
- local validation state

其中：

consensus-derived:  
Hash  
ParentHash  
Height  
Bits  
Timestamp  
Work  
CumulativeWork

local-only:  
Parent  
Children

local validation state:  
Status

必须严格遵守：

Status 不是 consensus state。

任何 fork-choice 逻辑不得读取落盘 Status  
作为共识依据。

==================================================  
WORK MODEL
==========

必须严格使用当前已经冻结的 Work 定义：

bits = exponent

target = 2^(256-bits)

Work(bits) = 2^bits

# CumulativeWork(node)

CumulativeWork(parent)  
\+  
Work(node)

使用 *big.Int。

不得重新解释 bits 为 Bitcoin compact nBits。

不得修改现有 pow semantics。

必须添加测试证明：

- genesis work = 2^16
- child cumulative work 正确
- 多代累计正确
- 不同 bits 的 cumulative work 可正确比较
- 不得发生 uint64 overflow
- *big.Int 比较正确

==================================================  
BLOCK TREE
==========

实现最小 Block Tree：

Hash → BlockNode index

支持：

- AddNode
- LookupNode
- Parent linking
- Child linking
- Duplicate hash rejection / idempotent handling
- Height consistency
- Parent existence检查
- Root / genesis handling
- Ancestor traversal基础能力

必须保持：

Parent.children 与 child.Parent 双向一致。

不得出现：

child.Parent != parent  
但 parent.Children 中不存在 child

或者反向不一致。

==================================================  
VALIDATION BOUNDARY
===================

REORG-1A 不得绕过现有 block validation。

如果需要接收 block：

应明确区分：

1. node indexing
2. block validation

不得因为 BlockNode 存在就自动认为 block valid。

Status 只能表达本地验证结果。

==================================================  
FORK-CHOICE BOUNDARY
====================

本阶段不得实现最终 fork-choice。

不得实现：

candidate.work > active.work  
→ SetTip

不得实现 tie-break execution。

只允许为后续 fork-choice 提供数据基础。

tie-break 已冻结为：

tip hash 按大端整数比较，  
较大者胜。

但本阶段不要实现完整 fork-choice。

==================================================  
DIFFICULTY BOUNDARY
===================

当前 reorg infrastructure 按 bits=16 基线实现。

不得修改：

AdjustBits  
difficulty retarget  
floating difficulty  
difficulty consensus rules

REORG-1N / DIFFICULTY-CONSENSUS-DESIGN-1  
独立推进。

==================================================  
PERSISTENCE BOUNDARY
====================

本阶段不得实现：

block_index.dat  
utxo_snapshot.dat  
undo persistence  
commitSeq  
atomic rename  
directory fsync  
crash recovery

这些属于后续阶段。

BlockNode 当前可以作为内存基础设施。

如果需要 persistence interface，  
只允许定义最小边界，不得接入生产持久化。

==================================================  
PRODUCTION SAFETY
=================

必须确认：

reorg execution feature flag = OFF

不得：

- 修改生产配置
- 修改生产 datadir
- 重启生产节点
- kill 生产节点
- SetTip production chain
- Disconnect production blocks
- Connect alternative production branch

所有测试必须使用：

unit test  
disposable test data  
isolated temporary state

==================================================  
TEST REQUIREMENTS
=================

至少覆盖：

A. Genesis

- genesis node
- parent nil
- height 0
- cumulative work = Work(genesis)

B. Linear chain

genesis  
→ A  
→ B  
→ C

验证：

height  
parent  
children  
cumulative work

C. Branching

genesis  
→ A  
→ B1

genesis  
→ A  
→ B2

验证：

B1/B2 都正确挂载。

D. Multi-level branch

genesis  
→ A  
→ B  
→ C1  
↘ C2  
↘ D2

验证：

所有 parent / children 正确。

E. Duplicate hash

重复加入相同 hash：

不得产生两个 BlockNode。

F. Missing parent

孤立节点不得错误连接到不存在 parent。

G. Work comparison

不同 bits：

Work  
CumulativeWork  
必须严格正确。

H. Big.Int safety

构造足够长的 chain：

验证不存在 fixed-width overflow。

I. Restart-like reconstruction

在纯内存 disposable 环境中：

按照相同 block sequence 重建 BlockTree，

必须得到相同：

Hash  
Height  
Parent  
Children  
Work  
CumulativeWork

J. Determinism

相同输入序列：

必须产生完全一致的 tree topology  
和 cumulative work。

==================================================  
REQUIRED NEGATIVE TESTS
=======================

必须测试至少：

1. parent height mismatch
2. duplicate hash
3. invalid parent linkage
4. inconsistent cumulative work
5. invalid bits
6. self-parent
7. cyclic parent relation（如果架构允许外部注入）
8. nil / malformed node

不得为了通过测试而放宽共识条件。

==================================================  
IMPLEMENTATION CONSTRAINT
=========================

尽可能只修改：

REORG-1A 所需最小源码文件  
\+  
REORG-1A 对应测试文件

禁止顺手重构其他模块。

禁止格式化整个仓库导致无关 diff。

禁止修改无关 docs。

禁止修改 production config。

==================================================  
VALIDATION
==========

实现后必须执行：

1. targeted REORG-1A tests
2. existing Go test suite
3. go vet（若项目当前基线支持）
4. git diff --check
5. git status
6. .go diff inventory
7. feature flag OFF verification

必须报告：

- 修改文件精确列表
- 每个文件 diff 行数
- 测试数量
- 测试结果
- 是否有旧测试回归
- 是否存在无关 diff
- production boundary 是否保持完整
- fork-choice 是否仍未启用
- reorg execution 是否仍关闭

==================================================  
FINAL GATE
==========

本阶段结束时不得自动进入 REORG-1B。

输出：

PHASE REORG-1A FINAL REPORT

必须明确：

VERDICT =  
PASS  
或  
PASS WITH GAPS  
或  
BLOCKED

并分别列出：

1. Baseline
2. Files Changed
3. BlockNode Contract
4. Tree Invariants
5. Work Invariants
6. Tests
7. Negative Tests
8. Production Safety
9. Known Limitations
10. Deferred REORG Components
11. Recommended Next Phase

如果 PASS：

最后必须明确：

REORG-1A IMPLEMENTATION COMPLETE.  
REORG EXECUTION REMAINS DISABLED.  
NO GIT WRITE PERFORMED.  
WAITING FOR NEXT EXPLICIT AUTHORIZATION.











