# P2PChain — V3 CONSENSUS SPEC（FINAL / FROZEN）

> **Status: FINAL / FROZEN**
> 本文是 V3 共识规则的**冻结规格**（specification artifact），由 `PHASE-P2PCHAIN-V3-CONSENSUS-SPEC-FINALIZE-1` 产出。
> 本文**尚未实施**：当前生产链仍运行 V2 规则（height < 3000）。本文的 canonical 提升（写入 `docs/CANONICAL-CONSENSUS-SPEC.md`）需单独 Owner 授权，详见阶段报告 §13。

---

## V3 冻结参数

```text
Version                      = V3
ActivationHeight             = 3000
TargetBlockTimeSeconds       = 300
DifficultyAdjustmentInterval = 20
InitialBits (NewRulesetInitialBits) = 30
MaxDifficultyBits            = 40
Rounding                     = Nearest
BlockVersion (NewRulesetBlockVersion) = 4
```

---

## §1 TARGET / DIFFICULTY MODEL

```text
target(bits) = 2^(256 - bits)
```

`bits` 是**难度指数 / 前导零位数**表示，**不是** Bitcoin compact nBits 编码。

```text
共识域: 1 <= bits <= 256
MaxDifficultyBits = 40   (共识 policy ceiling，位于表示域 [1,256] 之内，不重定义 256-bit target 空间)
```

| bits | target | 验证 |
|---|---|---|
| 30 | `2^226` | ✓ |
| 40 | `2^216` | ✓ |

```text
Work(bits) = 2^bits
CumulativeWork = Σ Work(bits_i)   （big.Int 实现，bits≤40 时 2^40 仍在 uint64，Σ 用 big.Int 无溢出）
```

---

## §2 NEAREST RETARGET RULE（确定性，逐字可实现）

操作顺序（**冻结，不可换位**）：

```text
1. previous bits        = parent.Header.Bits
2. expected timespan    = TargetBlockTimeSeconds × DifficultyAdjustmentInterval = 300 × 20 = 6000s
3. actual timespan      = parent.Timestamp − periodStart.Timestamp
                          （periodStartHeight = height − 20，clamp ≥ 0）
4. lower clamp          = expected / 4 = 1500s（整数除法 floor）
5. upper clamp          = expected × 4 = 24000s
   actual = clamp(actual, lower, upper)
6. target calculation   = currentTarget × actual / expected
                          （big.Int 整数乘法后整数除法，floor）
7. floor clamp (target) : 若 newTarget > T(MaxTargetBits=16) ⇒ bits = 16
8. Nearest rounding     : b0 = 257 − BitLen(t)
                          若 t² ≤ 2^(513 − 2·b0) ⇒ bits = b0
                          否则                    ⇒ bits = b0 − 1
9. MaxDifficultyBits ceiling : 若 bits > 40 ⇒ bits = 40
10. final bits
```

### 2.1 Nearest 数学规则

```text
round(256 − log2(t))，round-half-up
等价整数判据（无浮点、无溢出）：
    b0 = 257 − BitLen(t)
    if t² ≤ 2^(513 − 2·b0): return b0
    else:                   return b0 − 1
```

- 相邻 bits 几何中点 `M = 2^(256.5 − b0)` 是无理数，`t` 恒为精确整数 ⇒ **tie 不可达**；等号规范为 round-half-up（取 b0）。
- 全程整数运算，任何语言（Go/Python/Rust/…）用任意精度整数实现同一判据得到**完全一致**的 bits。

---

## §3 ACTIVATION BOUNDARY（逐块语义）

| Height | Ruleset | Target | Initial/Current Bits | Rounding | MaxBits | Version |
|---|---|---|---|---|---|---|
| 2999 | V2 | 60s | 继承（V2 漂移） | Ceil | 40（共享 ceiling；V2 时代历史常量为 32） | 2 |
| **3000** | **V3（首块）** | **300s** | **注入 30（一次性，非 V2 重算）** | Nearest | 40 | 4 |
| 3001 | V3 | 300s | 继承 30 | Nearest | 40 | 4 |
| 3019 | V3 | 300s | 继承 | Nearest | 40 | 4 |
| **3020** | **V3（首个重算边界）** | **300s** | Nearest 重算 | Nearest | 40 | 4 |

**关键语义**：
- `height == 3000` 块接收 `InitialBits = 30` 与 `Version = 4`，**不经过** V2 retarget 计算（一次性 activation injection，先于周期边界判断）。
- `height == 3020` 是 V3 首个真实重算点：`periodStartHeight = 3000`，`actualTimespan = ts(3019) − ts(3000)`。
- 首个 V3 epoch 边界 = 3020（`(3000+20)`，非 3000）。

---

## §4 V2 → V3 HARD-FORK SEMANTICS

```text
V2 block: version=2, Ceil 取整, Target=60s
V3 block: version=4, Nearest 取整, Target=300s
（MaxDifficultyBits=40 为 V2/V3 共享 ceiling 常量；V2 历史可达域 ≤30 未触顶，
  "MaxBits=32" 是 V2 时代的旧常量值，非独立 per-ruleset rule）
```

- **版本互斥**：`VersionForHeight(h)` 三态，h≥3000 返回 4；旧 V2 节点（二进制版本三态 1/2/3）对 Version=4 区块返回期望 3 ≠ 4 ⇒ `ErrInvalidVersion` 确定性拒绝。
- **双重拒绝**：即使版本校验被绕过，旧 V2 节点用 Ceil 算出的 expected bits ≠ 新块 Nearest bits ⇒ `ErrUnexpectedBits`。
- **V2 节点**：在 height≥3000 处确定性分叉，无法继续跟随 V3 链（**hard fork**）。
- **V3 节点**：拒绝 post-activation 的 V2 区块（version≠4 或 bits 不符 V3 规则）。
- **fork 边界**：精确在 height=3000。

---

## §5 REORG / ORPHAN / RECOVERY

```text
ruleset 完全由候选分支自身的 height 决定（IsNewRulesetActive(height)），
绝不依赖当前 canonical tip ⇒ 校验确定性。
```

| 场景 | 语义 |
|---|---|
| reorg before 3000 | 全部按 V2 规则，用分支自身历史算 expected bits |
| reorg crossing 3000 | 分支自身 3000 块按 V3（注入 30），3000 之前按 V2；用分支自身父块/历史 |
| reorg after 3000 | 全部按 V3，分支自身 Nearest 重算 |
| orphaned V3 block | 按 V3 规则验证后因工作量不足被丢弃 |
| restart around 3000 | 逐高度回放，height 决定 ruleset，无 canonical tip 依赖 |
| restart around 3020 | 3020 用分支自身 3000 作为 periodStart 重算 |
| 难度历史不同的分支 | fork-choice 用最大累积工作量 Σ 2^bits 比较，跨 ruleset 一致 |

**关键**：reorg 后难度计算使用**分支自身的 parent/history**（`forkChainView`），从不读 canonical 链祖先。

---

## §6 VERSION SEMANTICS

`Version = 4`（NewRulesetBlockVersion）：

- **consensus-critical**：是——`validateVersion` 强制 `b.Header.Version == VersionForHeight(h)`，不符即 `ErrInvalidVersion`。
- **用于 block rejection**：是——版本不符确定性拒绝。
- **非仅 informational**：版本三态（1/2/4）构成硬分叉的强制标记。
- **与 V2 节点交互**：V2 节点对 Version=4 返回期望 3 ≠ 4，拒绝。
- **activation 语义**：version 由 height activation 决定（`VersionForHeight`），version 本身不是 ruleset 判定源，ruleset 判定只读 height。

---

## §7 DETERMINISTIC TEST VECTORS

> 可直接转为实现阶段单测。`T(b)=2^(256-b)`，`H` 为任意固定算力。

### Vector A — V2 最后一块（height 2999）

```text
input:   height=2999, parent=2998(V2)
expected bits:    parent.Bits（非边界，继承）
expected version: 2
expected target:  2^(256 − parent.Bits)
expected epoch:   非边界，无重算
expected result:  接受（V2 规则）
```

### Vector B — height 3000（首个 V3 块）

```text
input:   height=3000
expected bits:    30（一次性注入）
expected version: 4
expected target:  2^226
expected epoch:   注入，不重算
expected result:  接受（V3 首块）
```

### Vector C — height 3001

```text
input:   height=3001, parent=3000(bits=30)
expected bits:    30（继承）
expected version: 4
expected target:  2^226
expected epoch:   非边界
expected result:  接受
```

### Vector D — height 3019

```text
input:   height=3019
expected bits:    继承父块
expected version: 4
expected target:  2^(256 − bits)
expected epoch:   非边界
expected result:  接受
```

### Vector E — height 3020（首个 V3 重算）

```text
input:   height=3020, parent=3019(bits=30), periodStart=3000
         actualTimespan = ts(3019) − ts(3000)
expected bits:    Nearest(clamp(actual) → newTarget)
expected version: 4
expected target:  2^(256 − newBits)
expected epoch:   periodStart=3000
expected result:  接受（V3 首重算）
```

### Vector F — Nearest 取整边界

```text
input:   newTarget = 精确 2 的幂 2^226（bits=30 的中点对称情况）
expected: b0 = 257 − 226 = 31；t² = 2^452，mid = 2^(513−62) = 2^451
         t² > mid ⇒ bits = 30
（通用：t ≤ 2^(256.5−b0) 取 b0，否则 b0−1；tie 规范 round-half-up 取 b0）
```

### Vector G — lower clamp

```text
input:   actualTimespan < 1500s（如 500s）
expected: clamp 到 1500s 后计算 newTarget，再 Nearest
```

### Vector H — upper clamp

```text
input:   actualTimespan > 24000s（如 100000s）
expected: clamp 到 24000s 后计算 newTarget，再 Nearest
```

### Vector I — MaxBits=40 ceiling

```text
input:   新算力导致 Nearest 输出 bits > 40（如 41）
expected: ceiling clamp ⇒ bits = 40
```

### Vector J — reorg across activation

```text
input:   竞争分支 2999'(V2) → 3000'(V3)
expected: 3000' 用分支自身规则 = V3 注入 30；若 3000' 用 V2 规则产出的 bits ≠ 30 ⇒ ErrUnexpectedBits
```

---

## §8 INITIAL BITS VALIDATION

```text
InitialBits = 30 的数学基础：
    expected block time = 2^30 / hashrate
    平衡算力 hashrate = 2^30 / 300 ≈ 3.579 MH/s
```

| bits | expected time @ 3.579 MH/s |
|---|---|
| 27 | ≈ 37.5s |
| 28 | ≈ 75s |
| 29 | ≈ 150s |
| **30** | **≈ 300s** |
| 31 | ≈ 600s |

> 这是**协议初始化标定**（calibration），不是声称真实网络恰好有 3.579 MH/s 算力。

---

## §9 MAXBITS=40 JUSTIFICATION

```text
bits 30 → ~3.579 MH/s 平衡（300s 目标）
bits 40 → ~3.665 GH/s 平衡
headroom = 2^40 / 2^30 = 2^10 = 1024×
```

- `MaxBits=32` 在 300s 目标下平衡算力约 `2^32/300 = 14.3 MH/s`，算力超此即结构性坍缩。
- `MaxBits=40` 提供 1024× 前向头寸，覆盖到 3.665 GH/s。
- **这是前向容量设计**，不声称当前生产链存在 MaxBits 故障（当前链 bits≈30，远未触顶）。

---

## §10 MONITOR / OBSERVABILITY CONTRACT（观察约定，不改 monitor）

V3 激活后，`cmd/difficulty-monitor` 应观察：

```text
current height
current bits
current version（应为 4）
expected target interval = 300s
actual interval
difficulty boundary（3000, 3020, 3040, ...）
retarget result
health state
```

**首个重要边界**：3000、3020、3040、3060、…（每 20 块）。

> monitor 观察规则**不能改变 consensus**。当前 monitor 使用 `targetBlockTimeSeconds=60`（V1/V2 观察目标）与 `maxDifficultyBits=40`（V3 冻结规格，`cmd/difficulty-monitor/main.go`）。V3 激活后 monitor 的观察目标需按 300s 呈现（观察参数，不改变 consensus，也不等同于 V3 activation 本身）。

---

## §11 CANONICAL PROMOTION 说明

本文为独立冻结规格。**未**写入 `docs/CANONICAL-CONSENSUS-SPEC.md`，原因：

1. `CANONICAL-CONSENSUS-SPEC.md` 治理契约明确"**本文随实现漂移同步修订**"、"SOURCE OF TRUTH = 实际实现"，即文档须**跟随实现**而非领先实现。
2. 当前实现仍为 V2/旧 V3（27/32/3/60s），直接改写会使文档"声称未实现的共识"，构成 unauthorized governance mutation。
3. 因此 canonical 提升需在 `PHASE-P2PCHAIN-V3-CONSENSUS-IMPLEMENTATION-1` 完成后、随实现同步修订，需单独 Owner 授权。
