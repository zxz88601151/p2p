# PHASE REORG-1J-G08-CONTRACT-EXCEPTION — CONTROLLED TEST SEMANTIC UPDATE

你现在处理的是：

`PHASE REORG-1J-IMPLEMENTATION-1` 停工报告中的唯一已确认 CONTRACT CONFLICT。

基线：

- HEAD = `e04d678`
- REORG-1H 改动当前仍未提交
- 当前停工原因：`TestG08_LegacyImmutableAndTruncateGuard` 的旧断言要求 `TruncateFromHeight(1)` 在 `legacyLen=2` 时返回 `ErrTruncateOutOfRange`，而 REORG-1J §1 / §6 / §11 明确要求支持穿越 legacy prefix 的 canonical reorg。
- 该冲突已经经过实现层规避分析，确认不能通过分支、global boolean、shadow-canonical 或错误字符串等方式合法规避。

## 本次授权

明确授权：

**仅对 G08 的“legacy prefix 不得被 TIP 回退穿越”这一旧断言进行语义更新。**

本次授权对应：

`Option 1 — G08 定点正向语义改写`

这是一次受控的 TEST CONTRACT UPDATE，不是扩大 REORG 实现范围。

---

# 绝对边界

## 允许

只允许修改：

`internal/storage/v2_test.go`

中的：

`TestG08_LegacyImmutableAndTruncateGuard`

仅限于与：

`TruncateFromHeight(1)`

以及 legacy-prefix canonical reorg 语义直接冲突的断言。

必须保留并继续验证：

1. legacy block 不允许通过 `DeleteBlock` 删除；
2. legacy bytes 不得被 rewrite；
3. legacy 数据在 reorg 后仍然物理存在；
4. canonical view 与 detached view 语义正确；
5. reorg 后 active TIP 正确；
6. 重新读取 legacy block 的内容必须与 reorg 前逐字节一致。

如有必要，可以把原来的“必须拒绝 TruncateFromHeight(1)”改成：

**正向 acceptance assertion：**

当合法 reorg 穿越 legacy prefix 时：

- canonical TIP 可以低于 `legacyLen`；
- 被旧链排除的 legacy block 不得被物理删除；
- legacy block 必须保持 byte-identical；
- detached 状态必须可确定性验证；
- restart 后 canonical view 必须保持一致。

---

# 严格禁止

不得：

- 修改任何 production code；
- 修改 REORG-1J contract；
- 修改 §1 / §6 / §7 / §8 / §11；
- 修改任何其他 test；
- 删除其他 G08 assertions；
- 修改错误字符串制造 PASS；
- 增加 global boolean；
- 增加 shadow-canonical；
- 修改 legacy storage layout；
- 修改 CommitReorg / setCanonicalFrom / validateTipCandidate；
- 修改 1H 已完成的 production implementation；
- push；
- tag；
- merge；
- rebase；
- amend；
- squash；
- commit。

本阶段只允许完成：

**G08 test semantic update + validation。**

---

# 重要原则

不要为了让测试通过而削弱测试。

目标不是：

`old FAIL → PASS`

而是：

`old obsolete negative assertion → stronger positive acceptance test`

因此必须证明：

> 原 G08 所保护的真正安全性质仍然存在，只是把“legacy 不得离开 canonical”这一已经被 REORG-1J 明确废止的行为从冻结断言中移除。

真正需要继续保护的是：

**legacy storage immutability，而不是 legacy canonical membership 永久不可变化。**

---

# 执行前必须先做

1. 输出当前 HEAD；
2. 输出 working tree；
3. 输出 staging；
4. 确认 REORG-1H 当前未提交改动；
5. 精确定位 G08；
6. 精确显示修改前相关断言；
7. 确认本次只会修改 G08；
8. 不执行 git reset / checkout / clean。

---

# 修改后必须验证

至少执行：

1. storage package 全量测试；
2. G08 定向测试；
3. 与 REORG-1J 相关的 canonical/reorg tests；
4. legacy immutability tests；
5. crash/restart canonical-view tests（若当前测试套件已有）；
6. `git diff --check`；
7. `git diff --stat`；
8. `git diff -- internal/storage/v2_test.go`。

必须证明：

- 只有允许范围内的 G08 test 被修改；
- production code zero diff；
- 其他 test zero diff；
- 原有 REORG-1H production changes 未被覆盖或回退；
- 所有相关测试 PASS。

---

# 输出要求

完成后不要继续 REORG-1J implementation。

只生成：

`PHASE REORG-1J-G08-CONTRACT-EXCEPTION — FINAL REPORT`

报告必须包含：

1. Before/After contract；
2. 精确修改位置；
3. 修改理由；
4. 保留的不变量；
5. 新增 acceptance assertions；
6. Test results；
7. Production-code diff = 0；
8. Other-test diff = 0；
9. Git status；
10. 是否满足进入下一阶段的条件。

最终 VERDICT 只能是：

`PASS — G08 CONTRACT CONFLICT RESOLVED`

或：

`NOT READY`

如果发现任何超范围修改，立即 STOP。

**不要 commit。不要 push。不要继续实现 REORG-1J。**



