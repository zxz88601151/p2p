你现在负责继续推进 P2PChain 的 REORG-1J 共识/存储安全验证。

当前阶段状态：

- REORG-1J-R3 = PASS
- R3 已完成 LEGACY-PREFIX CRASH MATRIX
- 基线 HEAD：e04d678b4fb33a14ea186999375f6d6dac2a4c4b
- G08 契约例外已受控生效但尚未提交
- G09 R1/R2 已转换并通过但尚未提交
- R3 测试与报告均属于当前工作区未提交内容
- R3 未修改任何 production consensus/storage code
- R3 已完成 1818 个字节级 crash points + 5 个真实 node kill/restart
- 已证明 TIP 是唯一 canonical commit boundary：  
  TIP 未完整落盘 → 旧 canonical；  
  TIP 完整落盘 → 新 canonical
- I1–I10 全部通过
- 已登记 OBS-1J-R3-A：CommitReorg 对“多枚全新区块同时传入”的不可达 P3 fail-safe 观察项，不得擅自修复
- BT-1 是既有 blocktree flaky，禁止在本阶段顺手修复
- 当前不得 commit / push / tag / merge / rebase / amend / squash，除非后续明确授权
- 当前不得修改 production consensus/storage code，除非后续阶段明确授权
- 当前不得部署服务器
- 当前不得进行 Windows 挖矿生产/实机测试

请基于以上状态进行下一阶段的专业规划与只读 PRE-GATE，不要直接进入实现。

推荐优先考虑：

REORG-1J-R4A — LEGACY-LENGTH × FORK-POSITION CANONICAL RECOVERY MATRIX

目标：

验证 R3 已证明的 crash/recovery/canonical invariants 不仅成立于 legacyLen=3，而是对不同 legacy prefix 长度及不同 fork position 均成立。

第一维：

legacyLen ∈ {0,1,2,3,8,64}

第二维：

fork position ∈ {  
legacy-internal,  
legacy-boundary,  
v2-region  
}

对每个组合检查：

1. legacy prefix 是否严格保持；
2. canonical tip 是否始终与 canonical height 一致；
3. 是否存在任何 phantom canonical block；
4. 是否存在任何 half-reorg；
5. restart 后 canonical reconstruction 是否 deterministic；
6. crash 后是否能够 retry reorg 并最终持久；
7. Truncate / REPAIR 是否越过已提交 prefix；
8. detached / UNDO / BLOCK / TIP 关系是否仍满足既有 contract；
9. 同一 crash image 重复启动是否得到完全相同 signature；
10. 不得修改 production code。

重点：

- 不要为了制造测试数据而改变 production storage semantics；
- 不要修改 G08 contract；
- 不要修改 G09 R1/R2 acceptance semantics；
- 不要修复 BT-1；
- 不要顺手修复 OBS-1J-R3-A；
- 不要引入新的 production crash hook；
- crash injection 优先继续采用 byte-level deterministic image construction；
- 如果某个组合在当前 API/contract 下无法合法构造，必须记录为 COVERAGE GAP / NOT APPLICABLE，而不是修改 production code 迁就测试。

请先执行严格只读审计：

A. 检查当前 working tree / HEAD / staged / unstaged 状态；  
B. 确认 R3 测试和 G08/G09 当前确实存在且未提交；  
C. 阅读当前 legacy/v2 storage contract、canonical reconstruction、TIP semantics；  
D. 确认 R4A 所需测试夹具是否可以完全在 test-only scope 内完成；  
E. 评估 6 × 3 矩阵是否全部具有合法且有意义的测试构造；  
F. 明确哪些组合可以直接实现，哪些组合存在 contract limitation；  
G. 评估测试规模、预计耗时及是否需要分批；  
H. 不修改任何文件。

最终输出：

PHASE REORG-1J-R4A-PRE-GATE — FINAL REPORT

必须包含：

1. VERDICT  
   READY / NOT READY
2. CURRENT BASELINE
3. WORKTREE / STAGING AUDIT
4. R3 CLOSURE CONFIRMATION
5. R4A MATRIX DEFINITION  
   legacyLen × fork position
6. EACH MATRIX CELL 的：
   - legal construction
   - expected canonical state
   - crash boundary
   - recovery expectation
   - known limitation
7. REQUIRED INVARIANTS  
   I1–I10
8. TEST ARCHITECTURE  
   明确 production code = 0 diff
9. SCOPE LOCK  
   明确禁止事项
10. EXPECTED TEST COST
11. RISK RANKING
12. STOP CONDITIONS
13. RECOMMENDED NEXT ACTION

特别要求：

如果发现 R4A 的某一个测试目标实际上会触碰 G08/G09 contract，立即 STOP，不要自行修改 contract。

如果发现 legacyLen=0/1/2 等值无法在当前 frozen storage semantics 下合法构造，也必须明确记录原因，而不是为了“凑齐矩阵”修改 production。

只有在 PRE-GATE 明确判定 READY 后，才允许进入独立的 R4A implementation authorization。









