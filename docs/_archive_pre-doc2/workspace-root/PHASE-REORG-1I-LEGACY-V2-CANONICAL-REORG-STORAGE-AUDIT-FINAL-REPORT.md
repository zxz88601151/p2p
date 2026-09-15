# PHASE REORG-1J — LEGACY PREFIX REORG REMEDIATION / CANONICAL VIEW UNIFICATION

## 阶段性质

STRICT CONTROLLED IMPLEMENTATION / CONSENSUS SAFETY / STORAGE SEMANTICS / REORG EXECUTION

## 前置报告

`PHASE REORG-1I — LEGACY/V2 CANONICAL REORG STORAGE AUDIT`

审计结论：

`NOT READY FOR IMPLEMENTATION`

推荐方向：

`OPTION A / CANONICAL VIEW UNIFICATION`

本阶段不得执行 OPTION B 的 legacy→v2 生产迁移。

---

# 1. 唯一目标

本阶段只允许解决 REORG-1I 已确认并登记的问题，使：

> **纯 legacy chain 可以安全执行穿越 legacy prefix 的 canonical reorg，同时保持 TIP 唯一提交权威、restart determinism、legacy physical immutability、crash safety 与 backward compatibility。**

必须解决：

- `GAP-1I-A` — P0
- `GAP-1I-B` — P1，修复后视为 P0
- `GAP-1I-C` — 两道 legacy prefix 闸门必须统一处理
- 为 `GAP-1I-D` 补齐 legacy-prefix crash evidence
- 为 `GAP-1I-E` 修正 tripwire，使其验证行为而不是脆弱错误字符串

---

# 2. HARD SAFETY RULES

本阶段严格禁止：

- 不得迁移 legacy 文件
- 不得重写 legacy record
- 不得 seek/truncate/overwrite legacy bytes
- 不得执行 legacy→v2 production migration
- 不得修改 production datadir
- 不得部署 Linux production node
- 不得启动 Windows production mining test
- 不得 push
- 不得 tag
- 不得 merge
- 不得 rebase
- 不得 amend
- 不得 squash
- 不得修改既有测试以降低断言强度
- 不得删除测试来制造 PASS
- 不得通过改变错误字符串来制造 tripwire PASS
- 不得引入 blockchain/blocktree 依赖到 storage 层
- 不得顺便实现 mining pool、Stratum、reward、network capacity 等无关功能

所有实现必须先在临时/dev/test datadir 验证。

---

# 3. PRIMARY DESIGN INVARIANT

必须建立以下唯一语义：

```text
TIP = 唯一 canonical commit authority

canonical view = deterministic function(TIP, block hash linkage)

legacy/v2 = physical storage representation

v2Mode != canonical authority
```

特别禁止：

```text
v2Mode == canonical state
```

不得再出现：

```text
SaveBlockDetached()
    -> v2Mode = true
    -> Height() changes
    -> canonical view remains stale
```

---

# 4. GAP-1I-A — REQUIRED FIX

修复：

```text
SaveBlockDetached()
```

导致：

```text
v2Mode = true
tipHeight = -1
Height() = -1
```

而 legacy `byHeight` 仍然存在的状态。

要求：

1. Detached BLOCK 写入不得改变 canonical TIP。
2. Detached BLOCK 写入不得改变 canonical height。
3. Detached BLOCK 写入不得清空或伪装 canonical view。
4. Detached BLOCK 写入后：

```text
Height()
TipHash()
BlockByHeight()
CurrentBits()
chain-work view
```

必须继续描述旧 canonical chain。

1. detached block 必须仅存在于 detached/block index。
2. restart 前后 canonical view 必须完全一致。
3. `v2Mode` 可以描述 storage mode，但不得改变 canonical state。

新增 deterministic regression test：

```text
legacy chain
    -> SaveBlockDetached(fork block)
    -> Height unchanged
    -> TipHash unchanged
    -> canonical blocks unchanged
    -> detached block exists
    -> restart
    -> all above remain identical
```

---

# 5. GAP-1I-B — REQUIRED FIX

修复：

```text
CommitReorg()
detachedUndos
undoIndex.offset = 0
```

不得使用常量 offset。

要求：

- detached block 已存在但 undo 尚不存在时；
- CommitReorg 必须能够正确定位新写入的 UNDO frame；
- `UndoFor(hash)` 必须从真实 frame offset 读取；
- restart 后 undo index 必须重新建立；
- 不允许通过重新执行 PutUndo 来规避问题；
- 不允许改变 `CommitReorg` 的 atomic commit semantics。

必须新增测试：

```text
SaveBlockDetached(block)
CommitReorg(detachedUndo)
UndoFor(blockHash)
restart
UndoFor(blockHash)
```

两次均必须成功且 payload 完全一致。

---

# 6. LEGACY PREFIX GATE REMEDIATION

必须同时处理 REORG-1I 已确认的两道闸门：

### Gate 1

当前：

```text
tip.height < legacyLen - 1
```

导致拒绝。

必须重新定义为符合新的 canonical model。

### Gate 2

当前：

```text
legacy height slot occupied by v2
```

导致拒绝。

必须允许：

```text
v2 canonical block
    at height < legacyLen
```

前提是：

```text
TIP
→ previous hash
→ ...
→ genesis
```

形成完整、有效、可验证的 canonical hash chain。

不得简单删除安全检查。

必须用新的 deterministic chain validation 替代旧的 physical-format-based restriction。

---

# 7. LEGACY PHYSICAL IMMUTABILITY

无论 canonical reorg 如何发生：

```text
legacy bytes MUST remain byte-for-byte unchanged
```

旧 legacy block：

```text
physical record remains
```

但：

```text
canonical ownership
```

可以由新的 TIP/hash-chain view 决定。

因此必须严格区分：

```text
physical record
```

与：

```text
canonical membership
```

不得通过复制/覆盖 legacy record 实现 replacement。

---

# 8. CANONICAL VIEW MODEL

实现必须明确回答：

```text
TIP 如何决定 canonical chain？

每一个 canonical height 如何从 hash linkage 推导？

legacy block 如何在新 TIP 下变成 non-canonical？

旧 canonical legacy block 如何继续保留为 detached/available block？

byHeight 如何与 TIP 保持一致？

byHash 如何处理同一 block 多次 physical occurrence？
```

尤其必须消除：

```text
legacySeq
byHash
byHeight
tipHeight
v2Mode
```

之间目前存在的隐式矛盾。

不得通过增加另一个 global boolean 解决问题。

---

# 9. TIP UNIQUENESS

必须保持：

```text
BLOCK write != canonical commit
UNDO write != canonical commit
DETACHED write != canonical commit

TIP commit == canonical commit
```

只有成功提交 TIP 后：

```text
canonical view
```

才允许发生变化。

因此必须验证：

### BLOCK only

```text
canonical unchanged
```

### BLOCK + UNDO

```text
canonical unchanged
```

### BLOCK + UNDO + crash

```text
old canonical recoverable
```

### BLOCK + UNDO + TIP

```text
new canonical
```

---

# 10. RESTART DETERMINISM

必须证明：

```text
runtime canonical view
==
restart reconstructed canonical view
```

至少验证：

```text
Height
TipHash
TipWork
BlockByHeight
canonical hash mapping
detached block availability
undo availability
```

禁止出现：

```text
before restart:
Height = X

after restart:
Height = Y
```

除非磁盘上已经存在合法的新 TIP。

---

# 11. REQUIRED LEGACY REORG MATRIX

必须新增 deterministic matrix：

```text
legacyLen = 8 / 32 / 128
```

至少覆盖：

```text
fork = 1
fork = legacyLen/2
fork = legacyLen-2
fork = legacyLen-1
fork = legacyLen
fork = legacyLen+1
```

每个 case 必须验证：

```text
Commit accepted/rejected
old TIP
new TIP
Height
canonical hashes
detached availability
restart result
```

特别必须证明：

```text
fork < legacyLen
```

现在可以成功执行真正的 legacy-prefix reorg。

---

# 12. REQUIRED CRASH MATRIX

现有 G06 不能直接视为 legacy-prefix evidence。

必须新增 legacy-prefix 基线。

至少覆盖：

```text
before BLOCK
after BLOCK before UNDO
after UNDO before TIP
after BLOCK+UNDO before TIP
after TIP
partial UNDO header
partial UNDO payload
partial BLOCK frame
dangling detached BLOCK
restart after every state
repeated restart
```

必须明确区分：

```text
complete frame
partial frame
missing fsync evidence
```

不得把无法注入的真实 power-loss semantics 写成 PASS。

仍然保持：

```text
MISSING EVIDENCE != PASS
```

---

# 13. TRIPWIRE TRANSFORMATION

禁止降低原 tripwire 的测试强度。

将：

```text
TestBranchKnownGapLegacyPrefixBlocksFirstReorg
```

转换为正向 acceptance test。

建议名称：

```text
TestBranchLegacyPrefixReorgSucceeds
```

必须断言：

- reorg succeeds
- expected height
- expected tip hash
- expected canonical branch
- old branch remains accessible as detached/non-canonical
- restart produces identical result

同时修复：

```text
TestRealProcessForkBlockTravelsByHash
```

使其成为真实双进程 convergence test。

至少断言：

```text
node A TipHash == node B TipHash
node A Height == node B Height
```

禁止只断言日志字符串。

---

# 14. TWO-PROCESS TEST

必须新增真实双进程测试：

```text
Node A
Node B

legacy/common chain
        ↓
fork
        ↓
different branches
        ↓
P2P propagation
        ↓
reorg
        ↓
same canonical TIP
```

必须验证：

```text
block travels by hash
undo available
reorg accepted
both nodes converge
restart both nodes
both remain converged
```

不得使用 mock 替代实际 storage/reorg path。

---

# 15. PRODUCTION-CHAIN SIMULATION

使用与生产 legacy chain 等价的模拟规模：

```text
legacyLen ≈ 1285
```

但：

```text
NO production datadir
```

必须验证：

```text
fork near genesis
fork near middle
fork near tip
fork at legacyLen
fork beyond legacyLen
```

重点确认：

```text
memory usage
runtime
canonical rebuild
restart time
undo reconstruction
storage integrity
```

不得因此修改 production configuration。

---

# 16. REQUIRED TEST COMMANDS

至少执行：

```text
go test ./internal/storage/...
go test ./internal/blockchain/...
go test ./cmd/node/...
```

按 REORG-1I 建议：

```text
DO NOT use ./internal/blocktree/... as a PASS gate
```

BT-1 单独统计。

同时运行所有相关 integration / process / P2P tests。

---

# 17. GIT DISCIPLINE

本阶段实施完成后：

默认：

```text
DO NOT COMMIT
```

先生成：

```text
PHASE REORG-1J — FINAL REPORT
```

报告必须包含：

- exact files changed
- exact functions changed
- diff statistics
- all tests
- legacy matrix
- crash matrix
- two-process convergence
- production-scale simulation
- GAP-1I-A closure evidence
- GAP-1I-B closure evidence
- GAP-1I-C closure evidence
- GAP-1I-D closure evidence
- GAP-1I-E closure evidence
- restart determinism
- git status
- forbidden-operation audit

只有明确达到：

```text
COMMIT READY
```

才允许进入后续 commit phase。

---

# 18. ABSOLUTE STOP CONDITIONS

任何以下情况出现时立即停止实现并报告：

1. 发现 TIP 仍不是唯一 canonical authority。
2. 需要修改 legacy bytes。
3. 需要 migration。
4. 需要修改 block serialization/hash。
5. 需要修改 consensus validation semantics。
6. 发现 canonical view 无法从 TIP/hash chain deterministic reconstruction。
7. crash recovery 出现不同结果。
8. restart 前后 canonical state 不一致。
9. detached block 写入仍影响 Height/Tip。
10. `UndoFor()` 仍可能读取 offset 0。
11. 必须修改 frozen production tests 才能通过。
12. 需要修改 blocktree 才能修复 storage contract。
13. 需要触碰 production datadir。
14. 发现新的 P0/P1 storage-consensus defect。

若发现新的 P0/P1：

```text
STOP IMPLEMENTATION
RETURN TO READ-ONLY AUDIT
REGISTER GAP
DO NOT PATCH SPECULATIVELY
```

---

# 19. FINAL VERDICT REQUIREMENT

最终只能输出以下之一：

```text
PASS
```

或：

```text
PASS WITH DEFERRED GAP
```

或：

```text
NOT READY
```

或：

```text
BLOCKED — NEW P0/P1 DISCOVERED
```

不得为了进入下一阶段而强行判 PASS。

---

# 20. MOST IMPORTANT PRINCIPLE

本阶段不是：

> “把 legacy reorg 的两个错误条件删掉。”

而是：

> **把 canonical ownership 从 physical legacy/v2 representation 中彻底解耦，并证明 canonical state 可以由 TIP + hash linkage 唯一、可重建、可崩溃恢复地确定。**

如果实现过程中发现 OPTION C 的结构性 canonical-view unification 比 OPTION A 的局部 patch 更安全，应优先选择安全性，而不是最小 diff。

但：

```text
不得扩大到 migration
不得扩大到生产部署
不得扩大到 mining
不得扩大到 network capacity
```

完成实现后停止，等待下一阶段授权。























