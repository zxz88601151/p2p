# docs/ 索引（P2PChain）

> 本文件是文档系统的**唯一入口**。先看这里，再按「推荐阅读顺序」进入具体文档。
> 建立原因：早期阶段的**文件名与正文阶段号存在错位**（历史遗留），且有**阶段号被复用**的情况。
> 为不破坏任何既有引用，**不改文件名**，改用本索引建立权威映射。

## 1. 权威性分级

| 级别 | 含义 | 代表文档 |
|---|---|---|
| **AUTHORITATIVE** | 决策依据。冲突时以它为准 | `MASTER-DESIGN.md`、`PROJECT-COMPLETION-REPORT.md` |
| **CURRENT** | 当前有效的事实/报告 | `PHASE-P2.1-EXECUTION-REPORT.md`、`PHASE-BRAND-0D.2-UI-CONVERGENCE-*.md`、`PHASE-BRAND-0D.3-DEVELOPER-CONSOLE-BASELINE-REPORT.md` |
| **HISTORICAL** | 已完成阶段的记录，只作追溯 | `PHASE-0-*.md`、`PHASE-0.1-*.md`、`PHASE-1A-*.md`、`PHASE-BRAND-0-*.md`、`PHASE-BRAND-0D-*.md` |
| **SPEC / TASK-BOOK** | 某阶段的输入任务书（不是结论） | `DEVELOPMENT_PROMPT.md`、`RUN-AUDIT-2026-09-12.md`、`PHASE-BRAND-0D.2-MVP-BOUNDARY.md` |
| **SNAPSHOT（局部过期）** | 对某段历史阶段仍权威，但计数/状态已落后于当前 | `FULL-IMPLEMENTATION-REPORT.md`（PHASE 1B–7 的验收快照；测试计数与状态早于 P2.1 及 BRAND 阶段） |

### 1.1 产品身份（基线冻结口径）

| 名称 | 状态 | 说明 |
|---|---|---|
| **P2PChain Developer Console** | **CURRENT** | 当前产品面：由节点自身 `embed` 托管的单页控制台（Node / Blockchain / Network / Mining / Diagnostics / Logs） |
| Developer Control Center | **ARCHIVED** | 已被取代的方向（BRAND-0D / 0D.1 / 0D.2 阶段）；仅作为**历史阶段名**出现在上述 HISTORICAL 文档中 |

> 二者**不得同时被表述为「当前产品」**。本索引中对 `Developer Control Center` 的全部引用都在描述
> **被复用的阶段编号**与**已归档的方向**，不构成定位冲突（PHASE BRAND-0D.3-COMMIT §3 核验结论：一致）。

## 2. 文件名 ↔ 正文阶段号映射（错位说明）
| 文件名 | 正文标题阶段号 | 是否错位 |
|---|---|---|
| `PHASE-0-ENGINEERING-BASELINE-REPORT.md` | PHASE 0 | 一致 |
| `PHASE-0.1-GATE-A-R-POW-DIFFICULTY-REMEDIATION-REPORT.md` | PHASE 0.1 (+0.1-R) | 一致 |
| `PHASE-1A-CORE-CONSENSUS-UTXO-TRANSACTION-AUDIT.md` | PHASE 1A | 一致 |
| `PHASE-P2.1-EXECUTION-REPORT.md` | PHASE P2.1 | 一致 |
| `PHASE-BRAND-0-FOUNDATION.md` | **BRAND-1** | ⚠️ 错位 |
| `PHASE-BRAND-0D-DEVELOPER-DESKTOP.md` | **BRAND-0D.1** | ⚠️ 错位 |
| `PHASE-BRAND-0D.1-PRODUCT-VALIDATION.md` | **BRAND-0D.2** | ⚠️ 错位 |
| `PHASE-BRAND-0D.2-MVP-BOUNDARY.md` | **BRAND-0D.3**（技术基础 Spike 的**任务书/边界规格**） | ⚠️ 错位 |
| `PHASE-BRAND-0D.2-UI-CONVERGENCE-BASELINE.md` | BRAND-0D.2（UI 收敛·实现前基线） | 一致 |
| `PHASE-BRAND-0D.2-UI-CONVERGENCE-IMPLEMENTATION.md` | BRAND-0D.2（UI 收敛·实现报告） | 一致 |
| `PHASE-BRAND-0D.3-TECHNICAL-FOUNDATION.md` | BRAND-0D.3（技术基础与可行性 Spike·**执行报告**） | 一致 |
| `PHASE-BRAND-0D.3-DEVELOPER-CONSOLE-BASELINE-REPORT.md` | BRAND-0D.3（Developer Console 验证/加固/基线） | 一致（**编号复用**，见下） |
| `PHASE-BRAND-1-DISCOVERY.md` | BRAND-1（品牌发现） | 一致 |

### 阶段号「BRAND-0D.3」被复用的两个含义

1. **技术侧**：`PHASE-BRAND-0D.2-MVP-BOUNDARY.md`（任务书）+ `PHASE-BRAND-0D.3-TECHNICAL-FOUNDATION.md`（执行报告）
   —— 主题是 *Developer Control Center 的技术基础与可行性 Spike*。
2. **产品侧**：`PHASE-BRAND-0D.3-DEVELOPER-CONSOLE-BASELINE-REPORT.md`
   —— 主题是 *Developer Console 的验证、加固与产品基线*（本阶段）。

两者**内容不同、不存在取代关系**：前者论证「能不能做」，后者交付「已做成什么、可信到什么程度」。

## 3. 推荐阅读顺序

**想快速了解这个项目：**

1. `../README.md` — 功能清单、构建运行、CLI、共识参数、已知限制
2. `PROJECT-COMPLETION-REPORT.md` — 项目完成状态、全量验收证据、未决项分级
3. `MASTER-DESIGN.md` — 剩余工程的总体设计决策记录

**想理解实现细节：**

4. `PHASE-1A-CORE-CONSENSUS-UTXO-TRANSACTION-AUDIT.md` — 共识基础（UTXO/交易）审计
5. `PHASE-0.1-GATE-A-R-POW-DIFFICULTY-REMEDIATION-REPORT.md` — 难度算法与其 clamp 语义（**读难度相关代码前必读**）

**想追溯产品方向：**

6. `PHASE-BRAND-1-DISCOVERY.md` → `PHASE-BRAND-0D.1-PRODUCT-VALIDATION.md` → `PHASE-BRAND-0D.2-MVP-BOUNDARY.md`
7. `PHASE-BRAND-0D.2-UI-CONVERGENCE-{BASELINE,IMPLEMENTATION}.md` → `PHASE-BRAND-0D.3-DEVELOPER-CONSOLE-BASELINE-REPORT.md`

## 4. 各文档一句话摘要

| 文档 | 一句话 |
|---|---|
| `../README.md` | 项目门面：能力、用法、共识参数、已知限制 |
| `PROJECT-COMPLETION-REPORT.md` | 项目完成报告：全量测试证据 + 未决项分级 + 完成判定 |
| `MASTER-DESIGN.md` | 剩余工程的设计决策记录（实现以此为准） |
| `DEVELOPMENT_PROMPT.md` | 原始开发任务书（模块卡片） |
| `PHASE-0-ENGINEERING-BASELINE-REPORT.md` | 把未验证快照建立为可开发基线（不含新功能） |
| `PHASE-0.1-GATE-A-R-POW-DIFFICULTY-REMEDIATION-REPORT.md` | 修均衡态难度漂移的 off-by-one；记录双层 clamp 语义与 caveat |
| `PHASE-1A-CORE-CONSENSUS-UTXO-TRANSACTION-AUDIT.md` | UTXO/交易/共识的严格只读审计 |
| `PHASE-P2.1-EXECUTION-REPORT.md` | datadir 进程独占锁缺陷修复与闭环 |
| `RUN-AUDIT-2026-09-12.md` | P2（数据目录并发）缺陷的规格书 |
| `FULL-IMPLEMENTATION-REPORT.md` | PHASE 1B–7 的全量实现验收快照（**其中测试计数与状态早于 P2.1 及 BRAND 阶段**；最新状态看 `PROJECT-COMPLETION-REPORT.md`） |
| `PHASE-BRAND-*.md` | 产品/品牌/控制台各阶段报告（见上表映射） |
| `design/stale-lock-options.md` | stale-lock 自动恢复候选方案对比（**仅记录，明确未实现**） |

## 5. 维护约定

- 新增阶段报告：文件名使用**正文的真实阶段号**；若不可避免复用编号，必须在本索引登记并写明区别。
- 阶段报告一旦被后续阶段取代，在本索引的「权威性分级」中降级为 HISTORICAL，**不删除**（保留可追溯性）。
- **历史结论被后续证据推翻时：不改写历史数字，就地加「⚠️ 更正」批注并指向新证据。**
  先例：`PHASE-P2.1-EXECUTION-REPORT.md` / `FULL-IMPLEMENTATION-REPORT.md` 中 `smoke-e2e.sh` 的「重启持久化」
  结论，因缺陷 **F-3**（`$!` 是 MSYS 伪 PID → 进程未被杀 → 断言空转）被证伪，已在原文加更正块，
  时点数字保留为快照。完整分析见 `PROJECT-COMPLETION-REPORT.md` §7。
- 索引与 `README.md` 的「文档」一节的条目必须同步。
