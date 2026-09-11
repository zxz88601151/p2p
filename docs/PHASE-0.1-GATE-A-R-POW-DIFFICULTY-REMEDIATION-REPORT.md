# PHASE 0.1 — GATE A-R / PoW DIFFICULTY REMEDIATION REPORT

> 项目：P2PChain · 阶段：PHASE 0.1（最小范围 PoW Difficulty Remediation）+ PHASE 0.1-R（Post-Remediation Verification & Closure 合并归档）
> 前置：PHASE 1B Entry Gate 暴露 `TestDifficultyAdjustmentStableAtExpected` 失败（got 19, want 20）
> 本阶段不是 PHASE 1B。未实现 UTXO / Transaction Validation / 任何其他 Consensus 功能。
>
> 归档说明：本文件曾在 PHASE 0.1 与 0.1-R 之间被外部进程覆写为任务书文本（无 git 副本），
> 已按会话内完整内容重写并并入 0.1-R 的验证更新。覆写事件已单独记录。

---

## 1. HARD BASELINE

| 项 | 值 |
| --- | --- |
| project path | `C:\Users\Administrator\Desktop\挖矿\p2pchain` |
| OS / CPU | Windows (MINGW64_NT-10.0-22631) / x86_64 |
| Go version | go1.22.12 windows/amd64（官方 zip SHA256 已校验） |
| Git version | git version 2.55.0.windows.3 |
| branch | `main` |
| HEAD（修改前） | `aacf87dcff66ccf2aa79857fc8a293dd83ceae77` = `aacf87d` ✓（0.1 与 0.1-R 两次基线均确认） |
| git status --short（0.1-R 起点） | `M internal/pow/pow.go`、`M internal/pow/pow_test.go`、`?? docs/PHASE-0.1-...md`、`?? docs/PHASE-1A-...md`（与任务书列举一致 ✓） |
| git diff --cached --stat | 空 ✓ |

## 2. 原始测试失败

```text
--- FAIL: TestDifficultyAdjustmentStableAtExpected (0.00s)
    pow_test.go:69: AdjustBits at expected timespan = 19, want 20
```

触发条件：`AdjustBits(MaxTargetBits=20, expectedTimespan=60s×20=1200s)`。
唯一生产调用方：`internal/blockchain/blockchain.go:66`（`Blockchain.CurrentBits()`），
其结果作为下一区块难度并由 `pow.Validate` 以 `hash < BitsToTarget(Bits)` 强制执行——
因此该缺陷直接弱化共识：每个均衡周期有效 target 系统性变化。

## 3. 数学根因

当前编码（简化压缩格式，非比特币 nBits）：

```text
bits → target :  T(b) = 2^(256-b)
target → bits :  b' = 256 - target.BitLen()   ← 缺陷所在
```

均衡态代入：`T(20) = 2^236`，`BitLen(2^236) = 237`，故 `b' = 256-237 = 19 ≠ 20`。
即 `T ∘ T⁻¹` round-trip 不恒等：`bits=20 → target → bits=19`，且
`T(19) = 2^237 = 2 × T(20)`——**均衡态每个调整周期难度向"变易"方向系统性漂移 2 倍**。
根因：对精确 2 的幂 target，`BitLen(2^k) = k+1`，逆变换应为 `257 - BitLen` 而非 `256 - BitLen`。

## 4. Bits → Target 分析

`BitsToTarget(bits)`：`1 << (256-bits)`（`pow.go` L31-35）。语义：bits = target 前导零位数。
`MaxTargetBits = 20` → `MaxTarget = T(20) = 2^236`（难度下限对应的最大 target）。
本阶段未修改该函数；`Validate`/`Mine` 仅依赖此正向变换，不受缺陷影响。

## 5. Target → Bits 分析

| 候选公式 | 2 的幂 round-trip | 一般 target 语义 | 判定 |
| --- | --- | --- | --- |
| `256 - BitLen`（旧） | ✗（b → b-1） | `T(b') = 2^L > t`：向"变易"方向取整 | 缺陷 |
| `257 - BitLen`（新） | ✓（b → b 精确还原） | `T(b') = 2^(L-1) ≤ t`：向下保守取整，**永不比计算值更易** | 采用 |

## 6. Equilibrium Property

性质：`actualTimespan == expectedTimespan ⟹ AdjustBits(b, expected) == b`（在 clamp 允许范围内）。
- `bits = 20, timespan = 1200s`：修复前返回 19（漂移），修复后返回 20 ✓（`TestDifficultyAdjustmentStableAtExpected` 通过）。
- 一般 bits ∈ [1,20] 的 target→bits round-trip 由 `TestTargetBitsRoundTripAllSupportedBits` 全覆盖验证 ✓。
- 注：`AdjustBits` 层面均衡不漂移仅对 `bits = MaxTargetBits` 可观测（bits < 20 时 `newTarget > MaxTarget` 被难度下限 clamp 返回 20，属既有语义），由 `TestAdjustBitsRoundTripEquilibrium` 验证。
- 单调性未破坏：`higher bits ↔ smaller target` 语义不变（`TestBitsToTargetMonotonic` 通过）。

## 7. 修复方案

`internal/pow/pow.go` L101（含注释更新，逻辑 1 行）：

```diff
-	newBits := uint32(256 - newTarget.BitLen())
+	newBits := uint32(257 - newTarget.BitLen())
```

未采用"盲改"：8 项边界核查全部通过后才采用（2 的幂精确还原 / 非幂次保守向下取整 /
上界 MaxTarget clamp 不受影响 / 下界 `newBits<1` guard 不可达但保留 / 天花板 clamp 语义不变 / 单调性保持）。

## 8. 为什么不是测试错误

测试断言的语义是 PoW 难度调整的定义性性质：**实际出块时间恰好等于期望时间时，难度不应改变**。
若接受 `got=19` 为正确行为，等于接受"哈希率完全符合预期时难度也必须每周期减半（变易 2 倍）"，
这与 `AdjustBits` 自身文档（L71-77："与比特币一致……实际用时比期望用时短才提高难度"）及
`newTarget = currentTarget × actual/expected` 的比例语义直接矛盾（均衡时比例 = 1，target 不应变）。
故：production logic defect → fix production code。既有 6 个测试一行未改。

## 9. 修改文件

| 文件 | 变更 |
| --- | --- |
| `internal/pow/pow.go` | 逆变换 1 行修复 + 注释说明 |
| `internal/pow/pow_test.go` | 纯新增 4 个测试（3 个 PHASE 0.1 + 1 个 0.1-R），既有测试零改动 |
| `docs/PHASE-0.1-GATE-A-R-POW-DIFFICULTY-REMEDIATION-REPORT.md` | 本报告 |

## 10. Diff Scope

```text
 internal/pow/pow.go      |  8 +++--
 internal/pow/pow_test.go | 89 +++++++++++++++++++++++++++++++++++++++++++++++
 2 files changed, 95 insertions(+), 2 deletions(-)
```

核验：无 UTXO / blockchain / wallet / P2P / storage / config 变更。

## 11. Boundary Tests

- `TestTargetBitsRoundTripAllSupportedBits`：bits 1→20 全量 `T(b) → 257-BitLen(T(b)) = b` 精确还原（2 的幂边界）。
- `TestTargetBitsConservativeRounding`（PHASE 0.1-R 补充）：非 2 的幂 target（`0.75×T(b)` → `b+1`、`1.5×T(b)` → `b`）
  满足保守不变量 `T(bits') ≤ t < T(bits'-1)`（向下取整、永不比计算值更易），bits 1→20 全量覆盖。
  前提确认：`bits=21` 作为 target 表示合法（既有 `TestBitsToTargetMonotonic` 即使用 `T(21)`；
  AdjustBits 的天花板 clamp 是输出策略，不限制表示合法性）。

## 12. Monotonicity Tests

- `TestBitsToTargetMonotonic`（既有，未改）：PASS ✓
- `TestDifficultyAdjustmentBounds`（既有，未改）：PASS ✓
- `TestAdjustBitsDirection`：短/均衡/长三种时间跨度下输出均保持 `MaxTargetBits`，
  固化既有双层 clamp 语义（天花板：bits ≤ 20；下限：target ≤ MaxTarget），并记录修复前
  均衡态返回 19 正是绕过该不变量的缺陷。

## 13. Full Build

```text
$ go build ./...
BUILD = PASS
```

## 14. Full Vet

```text
$ go vet ./...
VET = PASS
```

## 15. Full Test

```text
$ go test ./... -count=1  (-v 逐项统计)
ok  p2pchain/internal/block        (5 tests)
ok  p2pchain/internal/pow          (10 tests = 既有 6 + 新增 4)
ok  p2pchain/internal/transaction  (4 tests)
ok  p2pchain/internal/wallet       (4 tests)
TEST = 23/23 PASS（0 failed, 0 skipped）
```

注：任务书预估"16/16"为保守值；实测修复前基线为 18/19 PASS + 1 FAIL（pow 6 项），
本阶段累计新增 4 项边界/方向测试后为 23/23。以 verbose 实测计数为准。

## 16. Git Status（提交前）

```text
 M internal/pow/pow.go
 M internal/pow/pow_test.go
?? docs/PHASE-0.1-GATE-A-R-POW-DIFFICULTY-REMEDIATION-REPORT.md
?? docs/PHASE-1A-CORE-CONSENSUS-UTXO-TRANSACTION-AUDIT.md
HEAD = aacf87d
```

## 17. Remaining Caveats

1. **难度调整动态范围被 clamp 压缩（既有设计，本阶段不改）**：在 `MaxTargetBits=20` 且双层 clamp 下，
   从链可达 bits=20 出发任何时间跨度结果均为 20。
2. **adjustment direction（0.1-R 校准表述）**：
   mathematically correct before clamp（`newTarget ∝ actualTimespan`，构造性保证）；
   effective runtime direction is constrained by existing MaxTargetBits clamp——
   当前 difficulty adjustment 的实际动态范围被 clamp 设计压缩为单点 20，
   **未通过 runtime 验证完整"难度方向"**。若未来需要难度真正浮动，
   需重新设计 clamp 带宽（属独立共识参数阶段）。
3. Go 工具链为本机受管安装（`C:\Users\Administrator\.workbuddy\binaries\go\go\bin`），未写入系统 PATH。
4. 本报告曾被外部进程覆写为任务书文本后重写（见文首归档说明）。

## 18. Final Verdict

```text
VERDICT = PASS
```

- go build ./... = PASS
- go vet ./... = PASS
- go test ./... = PASS（23/23）
- equilibrium = correct（20→20，无漂移）
- round-trip = correct（bits 1→20 精确还原 + 非幂次保守不变量）
- monotonicity = correct
- adjustment direction = mathematically correct before clamp; effective runtime
  direction is constrained by existing MaxTargetBits clamp（动态范围压缩为单点，
  已如实记录，未宣称完整方向经 runtime 验证）
- scope = clean（仅 internal/pow/ 与本报告）

---

## §15 GATE A RE-EVALUATION

```text
Go >= 1.22                                PASS (go1.22.12)
go build ./...                            PASS
go vet ./...                              PASS
go test ./...                             PASS (23/23)
P2PChain zero unrelated changes           PASS
→ GATE A = PASS
READY FOR GATE B
```

## ABSOLUTE STOP

```text
PHASE 0.1 / 0.1-R = COMPLETE
GATE A = PASS
PHASE 1B = NOT STARTED
```
