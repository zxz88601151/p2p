# DOCUMENT-REMAINDER-CLEANUP-1-REPORT

> **Phase**：DOCUMENT-REMAINDER-CLEANUP-1 — 文档一致性清理（CONSOLIDATION-2 残留陈旧引用清零）
> **Owner authorization**：PHASE P2PCHAIN — DOCUMENT-REMAINDER-CLEANUP-1
> **Date**：2026-10-02
> **Scope**：Documentation consistency cleanup only · No code changes · No consensus changes · No protocol changes · No feature development
> **HARD STOP（强制）**：No commit · No deployment · No cloud modification · No production changes

---

## §0 HARD BASELINE

| 项 | 值 |
|---|---|
| `git rev-parse HEAD` | `52fb464af243fbcdfd78bab846b45ea090e918fb`（**本阶段未改动**） |
| `git branch --show-current` | `main` |
| 本阶段改动文件 | `docs/DETERMINISTIC-SERIALIZATION-SPEC.md`（M）、`docs/MASTER-DESIGN.md`（M）、`README.md`（M，CONSOLIDATION-2 延续） |
| 代码改动 | ❌ **无**。本阶段 `git diff --stat -- cmd/ internal/` 仅含 §4-B1.5/§4-B2 既存未提交 diff，本阶段零新增代码改动 |
| 对齐基线 | `docs/CANONICAL-CONSENSUS-SPEC.md` §4（难度规则集）、`PROJECT-AI-CONTEXT.md` §7（奖励公式）、`internal/utxo/apply.go`（补贴真值） |

---

## §1 Target 1 — `docs/DETERMINISTIC-SERIALIZATION-SPEC.md`

### §1.1 发现的陈旧引用

| 位置 | 旧陈述（stale） | 性质 |
|---|---|---|
| §9.7（行 201-203） | 「`MaxTargetBits = MaxDifficultyBits = 16`，因此链上可达难度恒为 16」「详见 README「难度为何不浮动」」 | 废弃的固定-16 陈述 + 失效交叉引用 |
| §9.7 调整公式（行 214） | `newBits = clamp(newBits, 1, 16)` | 上限钳制误写为 16（应为 `MaxDifficultyBits=32`） |
| §9.7（行 212） | `若 newTarget > MaxTarget: return 16`（无 floor 语义标注） | 缺 floor=MaxTargetBits 语义说明 |
| §9.4.4（行 138） | `奖励：Subsidy(height) = 50 >> (height / 210)` | 过期奖励公式（经济/共识引用） |

### §1.2 修订内容

- **§9.7 顶部陈述**（行 201-210 重写）：删除「难度恒为 16 / 难度为何不浮动」的旧说法，改为与 CANONICAL §4 对齐的三态规则集描述：
  - `MaxTargetBits=16`（floor，三重角色）/`MaxDifficultyBits=32`（ceiling）；
  - v1(<2000 钉死16) / v2([2000,3000) Ceil 钳[16,32]) / v3(≥3000 Nearest 钳[16,32] + h=3000 注入27)；
  - 「现网恒 16」解释为**尚未到达激活高度**，非设计不可浮动；v2/v3 结构性硬分叉；
  - 保留**历史注记**：明确旧「`MaxTargetBits = MaxDifficultyBits = 16`、难度固定不浮动」已被 CANONICAL §4 判作废。
- **§9.7 调整公式**（行 219、221）：`return 16` 补注 `floor clamp ⇒ MaxTargetBits`；`clamp(newBits, 1, 16)` → `clamp(newBits, 1, MaxDifficultyBits)`（ceiling=32，旧版误写16）。
- **§9.4.4 奖励公式**（行 138）：原 `50 >> (height/210)` 标记「历史设计稿，已过期，非当前实现」，并补入当前权威公式 `5 >> (height/5_250_000)`（每 5,250,000 块减半；halvings≥3 即归零，15,750,000 高度归零）。

> 修订严格**未重写架构**：§9.2/§9.4/§9.5/§9.8 等序列化字节格式、Merkle、创世 Bits=16（创世恒 16，正确）等均未触动；仅修正难度术语、共识引用与过期经济假设，并保留历史信息。

---

## §2 Target 2 — `docs/MASTER-DESIGN.md`

### §2.1 发现的陈旧引用

| 位置 | 旧陈述（stale） | 性质 |
|---|---|---|
| 共识参数（行 45） | `Subsidy(height) = 50 >> (height/210)`（每 210 块减半） | 过期奖励公式 / 经济假设 |
| 明确不做（行 125） | 「分叉/reorg 树状链（…当前单链追加）」列入明确不做 | reorg 已实现 → 过期「不做」陈述（属 CONSOLIDATION-2 发现的残留陈旧引用） |

### §2.2 修订内容

- **共识参数（行 45-46 重写）**：旧公式标记「历史设计稿，已过期，非当前实现，保留作历史参考」；新增当前权威公式 `5 >> (height/5_250_000)` 与归零参数（15,749,999 / 15,750,000），引用 `internal/utxo/apply.go` 与 `PROJECT-AI-CONTEXT.md` §7 为权威。
- **明确不做（行 126 注解）**：reorg 条目改为「已于 REORG-1* 阶段实现（`internal/blocktree` + `blockchain.executeReorg` + 孤儿队列）」，明确旧「明确不做」属**历史注记（已过期）**；其余真正未做项（RIPEMD160/secp256k1、SPV、TLS、代币经济）保留。
- 未改动区块校验全序、钱包、交易构建等架构性内容；行 121 提及的「README『难度为何不浮动』小节」为 PHASE 2.1 历史行动记录，按「保留历史信息」原则保留。

---

## §3 Consistency audit

### §3.1 Documentation matches code / canonical ✅

| 文档陈述 | 对齐依据 | 结论 |
|---|---|---|
| `MaxDifficultyBits=32`、`MaxTargetBits=16`、激活 2000/3000、注入 27、版本三态 | `internal/pow/pow.go`（已提交，CANONICAL §4.2） | ✅ 一致 |
| 单次幅度 ≤4×（`clamp(actual, expected/4, expected*4)`） | `adjustTargetCore` / CANONICAL §4.4 | ✅ 一致 |
| v1/v2/v3 三态规则（h 锚定） | `ComputeExpectedBitsAt` / CANONICAL §4.3 | ✅ 一致 |
| 奖励 `5 >> (height/5_250_000)`、归零 15,750,000 | `internal/utxo/apply.go` / PROJECT-AI-CONTEXT §7 | ✅ 一致 |
| reorg 已实现 | `internal/blocktree` / `blockchain.executeReorg`（README/PROJECT-AI-CONTEXT §9/§10 已声明） | ✅ 一致 |

### §3.2 No stale statements remain（授权范围内）✅

- 终检 grep（两文件）：`50 >> | (height/210) | 当前单链追加 | MaxTargetBits = MaxDifficultyBits | 不浮动 | 难度固定 | 固定为 16` **仅命中明确标注为「历史/过期」的保留行**（DETERMINISTIC 行 138、211；MASTER-DESIGN 行 45），无任何现行错误断言。
- CONSOLIDATION-2 §3.4 登记的两处残留（`DETERMINISTIC-SERIALIZATION-SPEC.md:202` 旧难度陈述、MASTER-DESIGN 旧奖励公式）**均已清除**；reorg 残留引用（MASTER-DESIGN 行 125）同步清除。

### §3.3 Out of scope / preserved

- 历史记录（PHASE 2.1 对 README 的诚实化描述、旧公式的「历史设计稿」标注）按「保留历史信息」原则保留，不重写。
- 序列化字节格式、Merkle、创世 Bits=16（正确，非 stale）等均未改动。

---

## §4 Residual risks & recommendations

1. **文档系统性过期**（非本阶段范围）：仓库尚有大量 `docs/PHASE-*` 阶段报告沿用旧难度/README 措辞（如 `PHASE-P2PCHAIN-A2.3-*`、`PHASE-MINING-*` 等）。CANONICAL-CONSENSUS-SPEC.md 已正确，建议后续仅维护 CANONICAL + README + PROJECT-AI-CONTEXT 三处权威源，阶段报告按需标注历史。
2. **单一事实源**：经济参数（补贴）当前权威为 `internal/utxo/apply.go` + PROJECT-AI-CONTEXT §7；建议后续在 CANONICAL-CONSENSUS-SPEC.md 增补经济章节，避免补贴真值在多处发散。
3. **未提交**：本阶段全部改动在工作树，需 Owner 后续授权 commit。

---

## §5 HARD STOP confirmation

- ❌ 未执行任何 `git commit` / `git push`。
- ❌ 未部署到任何环境（含腾讯云三节点 loopback 集群，全程只读、未触碰）。
- ❌ 未修改云/生产配置、未改动任何 consensus / network / wallet / 代码真值（本阶段零代码改动）。
- ❌ 未产生任何生产数据变更（仅编辑 3 份 Markdown 文档）。

**结论**：DOCUMENT-REMAINDER-CLEANUP-1 目标全部达成——CONSOLIDATION-2 发现的残留陈旧文档引用（DETERMINISTIC-SERIALIZATION-SPEC.md 旧难度/奖励陈述、MASTER-DESIGN.md 旧奖励公式与 reorg「不做」陈述）已全部清除并与 CANONICAL-CONSENSUS-SPEC.md / PROJECT-AI-CONTEXT.md 对齐；历史信息按要求保留；终检确认授权范围内无残留陈旧断言。全程遵守 HARD STOP。
