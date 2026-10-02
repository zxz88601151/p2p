# README-RELEASE-STATUS-FIX-1 — 最终报告

> **阶段**：文档唯一修正（README.md only，HARD STOP）
> **日期**：2026-10-02
> **治理指令**：Owner 授权「README-RELEASE-STATUS-FIX-1」，修正 PRE-RELEASE-GATE-1 的 D 类文档不一致。禁止任何 code/consensus/protocol/tests/dependency 变更。

---

## §0 基线（BASELINE）

| 项 | 值 |
|----|----|
| HEAD | `8410036ed7b610ac6e03d48dce5abdf20ef33643` |
| README.md 修正前状态 | 无未提交改动（干净） |
| 目标陈旧引用 | 第 59 行：`HEAD = 52fb464` + 「尚未提交、未部署、未做生产变更」 |

---

## §1 精确 diff（EXACT DIFF）

**唯一改动**：`README.md` 第 59 行（单行替换）。

**删除**（陈旧）：
> 但截至本文档同步（HEAD = `52fb464`）**尚未提交、未部署、未做生产变更**

**替换为**（当前事实）：
> **孤儿耐久与启动恢复代码已提交**（`36f9630` feat(node): orphan durability §4-B1/B1.5/B2），**协议真相同步文档已提交**（`8410036` docs: synchronize documentation truth），当前 HEAD = `8410036`。**尚未打 tag、尚未部署、尚未做生产变更**。

完整 diff 见附录（单文件、单行、零代码变更）。

---

## §2 markdown 一致性检查（CONSISTENCY CHECK）

| 检查项 | README.md 结果 | 说明 |
|--------|----------------|------|
| `52fb464` 残留 | ✅ 无 | README 内已完全清除 |
| 「尚未提交」残留 | ✅ 无 | 已改为「已提交」+「尚未打 tag/部署」 |
| 「工作树」残留 | ✅ 无 | README 内已无「工作树」表述 |
| Developer Node RC 定位 | ✅ 保留 | 第 19 行 `Developer Node` 未动 |
| 生产就绪免责声明 | ✅ 正确 | 「尚未打 tag、尚未部署、尚未做生产变更」明确保留，不宣称生产就绪 |
| 提交 SHA 引用准确 | ✅ 准确 | `36f9630`（孤儿代码）、`8410036`（文档）与 `git log` 一致 |

**README.md 一致性结论**：✅ 通过，目标 D 类不一致已消除。

---

## ⚠️ 范围外残留发现（必须披露，未修改）

在 §2 全仓库扫描中，发现 **README.md 之外**仍有 `52fb464` / 「未提交工作树」陈旧引用，但**均不在本阶段授权范围（README.md only）内**，故未做任何修改：

| 文件 | 跟踪状态 | 陈旧内容 | 处置 |
|------|----------|----------|------|
| `docs/spec/ORPHAN-DURABILITY-SPEC-v1.md:7` | **TRACKED（已提交于 8410036）** | 「本 spec 描述的能力位于**未提交工作树**（HEAD = `52fb464`…）」 | 范围外，未改，待 Owner 授权 |
| `docs/spec/STARTUP-RECOVERY-PLAN-1.md:7` | **TRACKED（已提交于 8410036）** | 「本 spec 描述的能力位于**未提交工作树**（HEAD = `52fb464`…）」 | 范围外，未改，待 Owner 授权 |
| `docs/PHASE-P2PCHAIN-ORPHAN-DURABILITY-E2E-RECOVERY-1-REPORT.md`（3 处） | untracked（历史报告，永不提交） | 阶段报告固化当时的 `HEAD=52fb464` | 属历史快照，无需改 |

**关键提示**：前两个 spec 文件的「治理状态」行存在**自我矛盾**——它们声称"位于未提交工作树（HEAD=52fb464）"，但实际上正是通过 `8410036` 提交进入 Git 的。这属于新的 D 类不一致，但**超出本阶段 README.md-only 授权**，需要 Owner 单独授权后才能修正。

---

## §3 最终结论（FINAL）

| 项 | 结果 |
|----|------|
| 授权范围遵守 | ✅ 仅改 README.md，零代码/共识/协议/测试/依赖变更 |
| 目标 D 类不一致（README:59） | ✅ 已消除 |
| README 一致性 | ✅ 通过 |
| 新增发现 | 2 个已提交 spec 文档的「治理状态」仍陈旧（范围外） |

**建议后续阶段**（待 Owner 授权，不在本阶段执行）：
- `SPEC-GOVERNANCE-STATUS-FIX-1`：修正 `docs/spec/ORPHAN-DURABILITY-SPEC-v1.md` 与 `docs/spec/STARTUP-RECOVERY-PLAN-1.md` 的「治理状态」行，改为「已提交（`8410036`）」。

---

## HARD STOP

**本阶段终止。** 未执行、也将不执行：`git add` / `git commit` / `git tag`。

README.md 修正已落地到工作树（未提交），等待 Owner 明确授权后再决定是否提交。

—— README-RELEASE-STATUS-FIX-1 结束 ——
