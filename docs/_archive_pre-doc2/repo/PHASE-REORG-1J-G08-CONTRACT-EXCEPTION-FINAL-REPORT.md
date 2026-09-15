# PHASE REORG-1J-G09 — POSITIVE REORG ACCEPTANCE TRIPWIRE CONVERSION

你现在进入 P2PChain 的 **REORG-1J-G09**。

## 阶段性质

**STRICT CONTROLLED IMPLEMENTATION**

目标不是扩大 REORG 范围，而是继续完成 REORG-1J §13 已登记的两个测试契约转换：

- R1：`TestBranchKnownGapLegacyPrefixBlocksFirstReorg`
- R2：`TestRealProcessForkBlockTravelsByHash`

本阶段只处理这两个既存 tripwire 的“旧行为断言 → 新 canonical reorg 正向 acceptance test”转换。

---

# 1. 当前基线

G08 exception 已完成：

- G08 contract conflict 已解决
- legacy-prefix canonical reorg 语义已正式接受
- legacy physical bytes immutable 语义仍保持
- G08 已通过 5/5 deterministic runs
- storage 全量测试通过
- `go build ./...` 通过
- `go vet` 通过
- 本阶段 G08 未 commit
- HEAD 仍为 `e04d678`
- 不得 commit / push / tag / merge / rebase / amend / squash

因此：

**不要重新修改 G08。**

---

# 2. 本阶段唯一目标

把 REORG-1J §13 中仍然保留旧语义的两个 tripwire 转换为正向 acceptance tests。

## R1

`TestBranchKnownGapLegacyPrefixBlocksFirstReorg`

旧语义：

> legacy-prefix reorg 应被拒绝。

新语义：

> 当 legacy prefix 中存在旧 canonical block，而更长/更优的 V2 branch 通过 hash linkage 成为合法候选时，reorg 必须成功，并最终使 active TIP/canonical height 正确收敛。

## R2

`TestRealProcessForkBlockTravelsByHash`

旧语义：

> 两个真实进程不应收敛。

新语义：

> 两个真实进程通过 P2P 传播 fork/reorg 所需 block，并基于 hash linkage 完成 canonical convergence，最终双方 active chain tip 必须一致。

---

# 3. 严格修改边界

允许修改：

- 与 R1 对应的 test 文件
- 与 R2 对应的 test 文件
- 必要的阶段测试报告

禁止：

- 修改 `internal/storage/v2.go`
- 修改 `internal/storage/v2api.go`
- 修改 `internal/storage/file.go`
- 修改 consensus production code
- 修改 `internal/blockchain/*`
- 修改 P2P production implementation
- 修改 G08
- 修改既有 flake
- 修改任何与 R1/R2 无关的测试
- 删除测试函数
- 弱化 assertion
- 修改错误字符串
- 新增 shadow-canonical 状态
- 修改 storage layout
- 修改 REORG-1J §1/§6/§7/§8/§11 契约

---

# 4. R1 必须证明的性质

转换 R1 时，不要简单把：

`expected error`

改成：

`expected nil`

必须建立完整 acceptance chain：

1. legacy block 仍然存在；
2. legacy block physical bytes 不发生变化；
3. branch block 已经通过 hash linkage 被识别；
4. `TruncateFromHeight()` 穿越 legacy prefix 不再因为 legacy ownership 而被拒绝；
5. active TIP 更新到正确 branch；
6. canonical height 正确；
7. detached old branch 不再被 `IsCanonical()` 认为 canonical；
8. legacy block 可以通过 hash 查询；
9. legacy block 内容与 reorg 前完全一致；
10. journal / append-only storage 没有发生非法 truncate。

优先使用现有公开 API 和现有测试 helper。

不得为了测试方便引入新的 production API。

---

# 5. R2 必须证明的性质

真实双进程测试必须从“观察不到收敛”转换成“验证收敛”。

至少验证：

1. fork block 可以通过 P2P 到达另一进程；
2. 对端可以通过 hash 找到该 block；
3. 对端不会因为 legacy-prefix ownership 而拒绝合法 reorg；
4. reorg 后双方 canonical tip 相同；
5. canonical height 相同；
6. active chain 的 block hash 序列一致；
7. old branch 被正确标记为 detached；
8. 无进程 crash；
9. 无数据损坏；
10. 双进程最终都能够正常 shutdown。

不要把“收到 block”误认为“完成 reorg”。

必须分别证明：

**transport → storage acceptance → validation → canonical selection → convergence**

---

# 6. Tripwire 转换原则

不要删除旧测试。

不要降低测试强度。

应该：

**旧 negative tripwire → positive acceptance test**

如果旧测试名称已经表达完全相反的语义，应：

- 优先保留测试函数的主体连续性；
- 必要时进行语义正确的重命名；
- 在报告中明确记录：
  - old contract
  - new contract
  - why old assertion was obsolete
  - what stronger invariant replaced it

任何测试删除必须 STOP 并请求额外授权。

---

# 7. 验证要求

修改后至少执行：

### R1

- 单测至少 5 次
- `-race`
- 相关 storage/blockchain package test

### R2

- 双进程测试至少 5 次
- 不允许只跑一次
- 必须记录每次最终 TIP
- 必须确认双方 TIP hash 一致

### 全局

- `go test ./internal/...`
- `go build ./...`
- `go vet ./...`
- `git diff --check`

如果出现已有 flake：

必须单独归因，不得为了让本阶段变绿修改 flake。

---

# 8. Git 安全边界

本阶段：

**严禁：**

- git commit
- git push
- git tag
- git merge
- git rebase
- git amend
- git squash

完成测试后立即 STOP。

---

# 9. 最终报告必须回答

输出完整 FINAL REPORT，并明确：

### A. R1

- old assertion
- new assertion
- exact changed file/function
- acceptance invariants
- repeated-run result

### B. R2

- old assertion
- new assertion
- exact changed file/function
- transport proof
- reorg proof
- convergence proof
- repeated-run result

### C. Scope

- production files modified?
- unrelated tests modified?
- G08 modified?
- REORG contract modified?
- new APIs?
- new globals?
- storage layout changed?

全部给出 YES/NO + evidence。

### D. Regression

将所有 FAIL 分类：

- 本阶段引入
- 预期 tripwire
- 已知 flake
- 环境问题

不得模糊归类。

### E. Git

明确报告：

- HEAD
- staging
- working tree
- commit 是否执行
- push 是否执行

---

# 10. STOP CONDITION

以下任一情况立即停止，不自行扩大范围：

1. R1 需要修改 production consensus/storage code；
2. R2 需要修改 P2P production implementation；
3. 发现 REORG-1J contract 与现有代码再次冲突；
4. 需要修改 G08；
5. 需要修改 R1/R2 之外的测试；
6. 发现 canonical ownership / TIP / hash linkage 设计仍存在未解决歧义；
7. 双进程测试无法证明最终 canonical convergence；
8. 出现新的 P0/P1 consensus safety issue；
9. 需要 commit 才能继续；
10. 需要部署服务器或生产节点才能继续。

一旦触发：

**立即 STOP，并生成 CONTRACT-CONFLICT / BLOCKING-GAP REPORT，不自行解决。**

---

# 11. 本阶段最终目标

不是“测试全部变绿”。

真正目标是建立以下证据链：

**legacy physical immutability**  
+  
**hash-linked branch validity**  
+  
**legacy-prefix reorg acceptance**  
+  
**canonical TIP transition**  
+  
**P2P block propagation**  
+  
**multi-process canonical convergence**

只有这条链成立，才能继续进入 REORG-1J 后续的 crash matrix、legacyLen matrix、production-scale simulation 和最终 deployment/readiness audit。

执行完成后：

**STOP。不要 commit。不要 push。不要进入 R3–R6。**





