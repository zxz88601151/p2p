执行 **PHASE REORG-1E-COMMIT-1**。

严格依据刚刚完成的：

**PHASE REORG-1E-IMPLEMENTATION-1 — FINAL REPORT**

执行模式：

> STRICT CONTROLLED GIT COMMIT EXECUTION

### 授权范围

仅执行当前报告 Option A：

1. 建立 commit 前硬基线；
2. 检查当前 HEAD、branch、working tree、staging；
3. 核对本阶段 M1–M10 实际变更仅属于报告列出的文件；
4. 显式审查 diff，确认：
   - 无生产 datadir 修改；
   - 无 `cmd/` production reorg wiring；
   - 无 `blocktree.SetTip` production integration；
   - 无 `ShouldReorg` / `ApplyBlockWithUndo` / `DisconnectBlock` 等提前接线；
   - 无未授权文件混入；
   - 既有测试断言未被修改；
5. 按报告边界创建 **单一 commit**；
6. commit message 建议：

`feat: implement persistent reorg storage and undo journal`

1. commit 后立即执行 post-commit verification：
   - HEAD 新 commit hash；
   - parent hash；
   - working tree clean；
   - staging empty；
   - commit 文件列表精确核对；
   - `git show --stat`；
   - `git diff --check HEAD^..HEAD`；
   - 关键 storage / UTXO targeted tests；
   - HEAD 稳定性检查；
2. 若发现任何超范围变更、未跟踪异常文件、生产路径接线或测试断言变化，立即 STOP，不自行修复。

### 明确禁止

本阶段绝对不要：

- 修改 BT-1；
- 修改 `internal/blocktree/`；
- 修改既有测试断言；
- 接入 `blocktree.SetTip`；
- 实现 REORG orchestration；
- 修改 `blockchain.AddBlock`；
- 修改 mining；
- 修改 P2P；
- 修改生产 datadir；
- migration；
- push；
- tag；
- merge；
- rebase；
- amend；
- squash；
- 修改生产配置；
- 启动真实服务器部署。

### 最终要求

完成 commit 后：

**STOP。**

不要自动进入 REORG-1C。

输出完整 FINAL REPORT，包括：

- pre-commit baseline
- exact commit hash
- parent hash
- exact files committed
- diff/stat
- tests
- post-commit verification
- scope verification
- production safety verification
- final verdict
- 下一阶段建议

下一阶段是否执行 **REORG-1C-PRE-GATE-1** 必须等待新的明确授权。













