# PHASE REORG-1J-R4A — IMPLEMENTATION-1

## LEGACY-LENGTH × FORK-POSITION CANONICAL RECOVERY MATRIX

基于 `PHASE REORG-1J-R4A-PRE-GATE` FINAL REPORT，正式授权进入 **R4A controlled test-only implementation**。

当前基线：

```text
HEAD = e04d678b4fb33a14ea186999375f6d6dac2a4c4b
branch = main
staging = empty
```

## 一、阶段目标

实现并执行：

```text
internal/storage/r4a_legacy_length_matrix_test.go
```

验证 R4A 定义的：

```text
6 legacy lengths × 3 fork positions
= 15 applicable cells + 3 N/A
```

核心目标是证明：

> 当 reorg fork point 位于 legacy prefix 内部时，v2 canonical block 可以合法占据 legacy height 对应的 logical slot，同时 legacy physical bytes 保持完全 immutable；crash/torn-tail recovery 后 canonical state 必须确定性恢复，并能够通过生产路径重新收敛到新 canonical chain。

本阶段是：

```text
STRICT CONTROLLED TEST-ONLY IMPLEMENTATION
CONSENSUS / STORAGE SAFETY VALIDATION
PRODUCTION CODE DIFF = 0
```

---

# 二、绝对 Scope Lock

本阶段严禁：

1. 修改任何 production storage / consensus code：
   - `internal/storage/*.go` 非 `_test.go`
   - `internal/blockchain/*`
   - `internal/blocktree/*`
   - `internal/p2p/*`
2. 修改 G08 contract exception。
3. 修改 G09 R1 / R2 acceptance semantics。
4. 修改 `internal/storage/r3_crash_matrix_test.go`。
5. 修复 `OBS-1J-R3-A`。
6. 新增 production crash hook、global switch 或 test-only production branch。
7. 修改 SP-3b、UNDO binding、legacy write restriction 或 genesis production semantics。
8. 修改 BT-1。
9. commit / push / tag / merge / rebase / amend / squash。
10. 部署服务器。
11. Windows 实机挖矿。

如果发现任何问题必须先分类并报告，不得自行扩大 scope。

---

# 三、第一优先级：L=0 探针

首先只实现：

```text
L=0 / v2-region
```

这是一个独立 probe。

目标：

```text
OpenFileBlockStore(empty)
→ construct pure-v2 genesis
→ append k=4 canonical blocks
→ construct legal reorg
→ CommitReorg
→ crash-image recovery
→ deterministic reopen
```

验证：

- 是否能够合法构造；
- `Height()`；
- `TipHash()`；
- v2 genesis；
- TIP；
- UNDO；
- canonical path；
- crash recovery；
- signature determinism。

如果 L=0 失败：

### 如果失败原因属于既有 production contract：

立即将：

```text
L0 / v2-region
```

降级为：

```text
CONTRACT LIMITATION / N/A
```

并记录证据。

### 如果唯一解决方法需要修改 production：

立即 STOP。

禁止修改 production 以“修通”L0。

---

# 四、第二优先级：legacy-internal

完成 L0 probe 后，优先执行：

```text
L=2 / legacy-internal
L=3 / legacy-internal
L=8 / legacy-internal
L=64 / legacy-internal
```

这是本阶段最高价值测试。

尤其必须证明：

```text
f = L - 2
```

时：

```text
f+1 <= L-1
```

即新 canonical v2 block 真正落入 legacy physical prefix 对应的高度范围。

---

# 五、必须严格验证 I12

I12 是 R4A 的核心新增 invariant。

不要仅验证：

```text
Height()
TipHash()
```

必须在 `legacy-internal` case 中显式找到至少一个：

```text
h < L
legacyRecord[h]
v2Block[h]
```

并证明：

```text
legacyRecord[h] physical bytes unchanged
v2Block[h] != legacyRecord[h] where expected
```

然后验证 canonical selection：

```text
latest valid TIP
    ↓
TIP block
    ↓
parent-hash traversal
    ↓
canonical ancestor path
```

必须明确：

> 最新有效 TIP 选择 canonical chain 的末端；canonical path 由 TIP 的 parent-hash ancestry 唯一确定。

在同一高度存在 legacy + v2 双记录时：

```text
same blocks.dat
same valid TIP
same startup
```

必须得到：

```text
same Height
same TipHash
same byHeight[h]
same canonical classification
```

重复启动必须逐字节 signature 相同。

---

# 六、矩阵执行

15 个 applicable cells 全部执行。

参数：

```text
k = 4
tipRingSize = 8
```

fork positions：

```text
legacy-internal
legacy-boundary
v2-region
```

legacy lengths：

```text
0
1
2
3
8
64
```

3 个 N/A 必须保持 N/A：

```text
L0 / legacy-internal
L0 / legacy-boundary
L1 / legacy-internal
```

不得为了凑满 18 格修改 production semantics。

---

# 七、Variant B

Variant B 只允许：

```text
L=3
```

三个 fork-position cells。

并严格绕行：

```text
OBS-1J-R3-A
```

即：

```text
先 detached 落盘已有新分支块
最后一枚新块由 CommitReorg 路径处理
```

禁止修改 OBS-1J-R3-A。

---

# 八、Crash Matrix

每个 applicable cell：

```text
delta = full[len(base):]
```

必须首先解码并断言：

```text
TIP count == 1
TIP is final frame
```

然后：

```text
j ∈ [0, len(delta)]
```

执行 byte-level torn-tail sweep。

对每一个：

```text
j < len(delta)
```

必须验证 recovery 后：

```text
Height() == oldTop
TipHash() == oldTip
LogSize() == len(base)
canonical == old canonical
```

而：

```text
j == len(delta)
```

必须得到：

```text
Height() == newTop
TipHash() == newTip
canonical == new canonical
```

不得添加 crash hook。

---

# 九、I9 / I10

每个 applicable cell 必须执行生产式 recovery convergence：

```text
crash
→ reopen
→ identify missing detached branch blocks
→ SaveBlockDetached(parent-first)
→ CommitReorg(detachedUndos, nil, nil, newTip)
→ close
→ reopen
```

最终必须证明：

```text
new canonical survives restart
```

I10：

```text
j = 0
j = len(delta)-1
j = len(delta)
```

每个点重复 5 次。

signature 必须完全一致。

---

# 十、附加用例

执行：

```text
A1: TruncateFromHeight() crossing legacy region
A2: legacy single-byte corruption
A3: legacy tear vs v2 tear
```

特别验证：

### A1

logical height 可以回退，但：

```text
legacy physical bytes unchanged
RecordCount does not physically shrink
restart preserves state
rolled-back block remains physically queryable
```

### A2

必须：

```text
REJECT
ErrCorruptStore
no physical mutation
no truncation
```

### A3

必须保持既有语义：

```text
v2 tear → REPAIR
pure legacy tear → REJECT
```

不得因为测试方便改变该语义。

---

# 十一、硬 STOP Conditions

出现任意一个立即停止整个阶段：

```text
canonical count != Height()+1
TIP height != Height()
legacy prefix bytes changed
LegacyRecordCount != L
committed bytes physically removed
same crash image produces different signature
I12 nondeterminism
TIP count != 1
TIP not final frame
需要修改 production code
需要修改 G08/G09
需要修 OBS-1J-R3-A
需要修改既有测试
无法证明 canonical state，只能证明 process alive
```

唯一例外：

```text
L=0
```

如果其失败原因是既有 contract limitation 且唯一 workaround 是 production change，则：

```text
L0 = N/A / CONTRACT LIMITATION
```

不要修改 production。

---

# 十二、回归门

完成 R4A matrix 后执行：

```text
go test ./internal/storage/
go test -race ./internal/...
go test ./cmd/node/...
go build ./...
go vet ./...
git diff --check
```

引用“全绿”时：

```text
internal/blocktree
```

必须显式排除 BT-1 已知 flake，并准确记录其状态。

不得为了让回归“全绿”修改 BT-1。

---

# 十三、最终文件

本阶段允许新增：

```text
internal/storage/r4a_legacy_length_matrix_test.go
docs/PHASE-REORG-1J-R4A-FINAL-REPORT.md
```

如果报告文件已存在，不得覆盖其他阶段内容，需先停止并报告。

生产代码最终必须：

```text
production diff = 0
```

---

# 十四、最终报告必须回答

1. 15 个 applicable cells 是否全部 PASS？
2. 3 个 N/A 是否保持原定义？
3. L0 是否可构造？
4. legacy-internal 是否全部通过？
5. 是否真实证明 v2 canonical block 占据 legacy height？
6. legacy physical prefix 是否逐字节 immutable？
7. I12 是否通过？
8. 每个 delta 是否恰好一个 TIP 且 TIP 位于末位？
9. byte-level recovery 是否通过？
10. I9 convergence 是否通过？
11. I10 deterministic repetition 是否通过？
12. A1/A2/A3 是否通过？
13. 是否出现任何 production semantic defect？
14. 是否触发任何 STOP condition？
15. working tree / staging 最终状态是什么？

最终输出：

```text
PHASE REORG-1J-R4A — FINAL REPORT
VERDICT = PASS / FAIL / STOP / CONTRACT LIMITATION
```

本阶段结束后：

```text
STOP
```

**不要 commit。**  
**不要 push。**  
**不要 tag。**  
**不要部署。**  
**不要自动进入下一阶段。**












