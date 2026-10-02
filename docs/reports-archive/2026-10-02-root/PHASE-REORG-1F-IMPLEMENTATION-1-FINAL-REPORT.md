# PHASE-REORG-1F-IMPLEMENTATION-1 — FINAL REPORT

> 阶段：REORG-1F（mempool 复活 / Architecture B）
> 状态：**PASS（含一处回归修复）— 按阶段纪律 STOP，未提交**
> 报告日期：2026-09-14
> 产物根目录：`C:\Users\Administrator\Desktop\挖矿\p2pchain`

---

## 1. VERDICT

**PASS。**

- 12 个 mempool 复活测试 **全部通过**（含 `-race` 与 5× 全新独立运行复验）。
- 阶段进行中发现并修复了一处**由本阶段对 `extendChain` 的修改引入的回归**：损坏存储重复记录静默被接受（`TestDuplicatePersistedRecordIsRejected` 失败）。已定位根因并修复，该测试现已通过（含 `-race`）。
- 全量 `go test ./internal/...` 绿灯；`-race`、`go vet`、`gofmt`、`git diff --check` 均干净。
- Architecture-B 不变量保持：blockchain 包**不** import `p2pchain/internal/mempool`。

**STOP 规则生效**：本阶段**不提交、不推送、不打 tag、不 merge/rebase/amend**。下一阶段 = `REORG-1F-COMMIT-READINESS-AUDIT`。

---

## 2. 阶段定位与范围

| 项 | 内容 |
|---|---|
| 阶段名 | REORG-1F-IMPLEMENTATION-1（mempool 复活，Architecture B） |
| 设计基线 | `PHASE-REORG-1F-DESIGN-1-FINAL-REPORT.md` / `PHASE-REORG-1F-PRE-IMPLEMENTATION-GATE-1-FINAL-REPORT.md`（均已 PASS） |
| 架构选择 | **Architecture B**：`executeReorg` 返回 `ReorgResult`（含 `DisconnectBlocks`），service 层执行复活；区块链层不得依赖 mempool 包 |
| 授权变更文件 | `internal/blockchain/blockchain.go`、`internal/mempool/mempool.go`、`cmd/node/service.go`、`internal/mempool/reorg_resurrection_test.go` |
| 禁止项 | 无 push/tag/merge/rebase/amend；不可触碰生产 datadir；测试须用临时目录 |

---

## 3. HEAD / PARENT（git 真相）

```
HEAD (committed) : 369d36e  feat: integrate canonical chain reorganization   (REORG-1C)
PARENT chain     : 369d36e → 6ed1e83 (REORG-1E storage) → b001f57 (REORG-1B)
工作树状态       : 3 个 .go 已修改 + 1 个 .go 新增（未提交）；HEAD 仍为 369d36e
```

> 注：本阶段工作树相对 `369d36e` 为**未提交**增量。REORG-1E 已于 `6ed1e83` 提交，非本阶段产物。

---

## 4. 交付物与文件变更

| 文件 | 状态 | Diff stat | 职责 |
|---|---|---|---|
| `internal/blockchain/blockchain.go` | M | +83 / −7 | 复用 1C 的 reorg 管线（`ReorgResult`/`AddBlockWithResult`/`executeReorg`/`SaveBlockDetached`）；本阶段新增 `extendChain` 树维护（保证 live reorg 的 BestTip 正确）+ **Case 2 损坏防护**（回归修复） |
| `internal/mempool/mempool.go` | M | +113 / −0 | `ReaddDisconnected(txList, newChainTxs)` 多轮复活：跳过 coinbase、跳过已确认（在 `newChainTxs` 中）、容量上限守卫 |
| `cmd/node/service.go` | M | +32 / −7 | service 层编排：调用 `AddBlockWithResult` 取 `ReorgResult`，对 `DisconnectBlocks` 内交易执行 `mempool.ReaddDisconnected` |
| `internal/mempool/reorg_resurrection_test.go` | ?? (new) | 558 行 | 12 个复活测试（本阶段核心交付） |

**Diff 汇总**：3 个已跟踪文件，221 行新增 / 7 行删除；另 1 个新增测试文件（558 行）。

---

## 5. 12 个复活测试结果（证据优先）

运行：`go test -v -count=1 -run 'TestReorg' ./internal/mempool/`

| # | 测试 | 结果 | 耗时 |
|---|---|---|---|
| 1 | `TestReorgResurrectsDisconnectedTx` | PASS | 2.62s |
| 2 | `TestReorgDoesNotResurrectCoinbase` | PASS | 0.54s |
| 3 | `TestReorgDoesNotResurrectConfirmedTx` | PASS | 2.40s |
| 4 | `TestReorgRejectsInvalidUnderNewUTXO` | PASS | 3.37s |
| 5 | `TestReorgRejectsDoubleSpend` | PASS | 1.07s |
| 6 | `TestReorgResurrectsDependentTxs` | PASS | 2.92s |
| 7 | `TestReorgMultiLevelDependency` | PASS | 2.41s |
| 8 | `TestReorgRepeatedResurrectionNoDuplicate` | PASS | 2.33s |
| 9 | `TestReorgMempoolCapacityRespected` | PASS | 3.07s |
| 10 | `TestReorgResurrectionFailureDoesNotAffectTIP` | PASS | 2.41s |
| 11 | `TestReorgZeroResurrectableTransactions` | PASS | 0.68s |
| 12 | `TestReorgMultipleDisconnectedBlocks` | PASS | 2.09s |

**合计：12 / 12 PASS**（`ok p2pchain/internal/mempool`，单跑 ~26–28s，5× 独立复验一致）。

---

## 6. 验证门（Validation Gate）

| 检查 | 命令 | 结果 |
|---|---|---|
| 全量内部测试 | `go test -count=1 ./internal/...` | **全部 ok**（含 `blocktree` BT-1 本次未触发 flaky） |
| 竞态检测 | `go test -race -count=1 ./internal/blockchain/ ./internal/mempool/` | blockchain ok / mempool ok（无 data race） |
| 静态检查 | `go vet ./...` | exit 0（干净） |
| 格式 | `gofmt -l <4 文件>` | 空（全部已格式化） |
| 差异卫生 | `git diff --check <3 跟踪文件>` | 空（无尾随空白/无错误） |
| 架构不变量 | grep `p2pchain/internal/mempool` in `internal/blockchain` | **无匹配**（Architecture B 保持） |

> 备注：`git status` 对 3 个跟踪文件打印 `LF will be replaced by CRLF` 警告——此为仓库 autocrlf 行尾规范提示，**非内容错误**；`git diff --check` 已确认无尾随空白等实质问题。

---

## 7. 关键发现：损坏存储静默接受回归（已修复）

### 现象
`TestDuplicatePersistedRecordIsRejected`（既存持久化安全测试，位于 `internal/blockchain/replay_persistence_test.go`）在引入本阶段 `extendChain` 修改后由 **PASS 转为 FAIL**（含/不含 `-race` 均失败）：损坏存储（同一区块被追加两次）被静默加载为有效链，违反「损坏数据不得被转换成有效链」契约。

### 根因（经干净 worktree @ HEAD 对照 + 插桩追踪确认）
- 该测试构造：genesis + 块 b（高度 1）+ 手动 `SaveBlock(b)` 重复记录（高度 2，其 `PrevBlockHash = genesis`）。
- 回放至高度 2 时，`currentTip = b`，故 `parentHash(genesis) != tip(b)` → 走 **Case 2（fork 路径）**。
- **HEAD 原行为**：回放期间 blocktree 仅含 genesis（rebuild 在循环后执行），故 `tree.ShouldReorg` 报 `blocktree: no active tip set` → `applyBlock` 返回错误 → 正确拒绝。
- **本阶段修改后**：`extendChain` 在回放期也把块加入 blocktree 并 `SetTip`，导致高度 2 的重复记录命中 `tree.AddBlock → ErrDuplicateHash`，被 `addBlock` 当作幂等 re-delivery **`return nil` 静默接受** → 测试失败。

### 修复（位于 `internal/blockchain/blockchain.go`，授权文件内）
在 `addBlock` 的 **Case 2** 入口新增**显式损坏防护**：若区块哈希已存在于 canonical 链（`bc.blockAtHash` 命中 `bc.blocks`），则显式拒绝（`ErrInvalidPrevHash` 包装），不再落入 `ErrDuplicateHash` 幂等分支。

```go
// 损坏防护：区块哈希若已存在于 canonical 链（bc.blocks），则这是重复持久化记录
//（旧 bug 的存储形态：同一区块被追加两次），必须显式拒绝，绝不能当作幂等 re-delivery 静默吞掉。
if _, err := bc.blockAtHash(b.Header.Hash()); err == nil {
    bh := b.Header.Hash()
    return fmt.Errorf("%w: 区块 %x 已存在于 canonical 链（疑似重复持久化记录）", ErrInvalidPrevHash, bh[:4])
}
```

### 修复正确性论证
- **保留** `extendChain` 的 tree 维护（live 操作 `persist=true` 时仍需，否则 reorg 的 BestTip 基准错乱）—— live reorg 不受影响。
- **合法 fork / re-delivery** 区块不在 canonical 链中，`blockAtHash` 不命中 → 不误杀 → 仍走 `ErrDuplicateHash` 幂等路径。
- 复验：`TestDuplicatePersistedRecordIsRejected` 现 PASS（含/不含 `-race`）；12 个复活测试仍全 PASS；全量 `./internal/...` 仍全 ok。

---

## 8. 测试稳定性修复（reorg_resurrection_test.go 内）

| 问题 | 根因 | 修复 |
|---|---|---|
| 多个「expected reorg」随机失败 | `ShouldReorg` 在工作量平局时按 tip 哈希大端做确定性 tie-break，genesis 分支 fork 若长度 == canonical 高度则 `CumulativeWork` 相等 → 随机触发 | 所有 genesis 分支 fork 延长至 13–15 块（高度 ≥ canonical 11–12）→ `cmp>0` 确定性 reorg |
| `TestReorgMultipleDisconnectedBlocks`：disconnect=12 want 2 | fork 从 genesis 分叉，断开全部 12 个 canonical 块 | 改为从 **h10** 分叉（3 块 h11'..h13'），common ancestor=h10，`DisconnectBlocks` 确定化为 `[h11, h12]`（2 块），`accepted=2` |
| `TestReorgRejectsInvalidUnderNewUTXO`：coinbase 未成熟 | 花费交易置于 fork h9（< maturity 10） | 移至 fork h12（成熟） |
| `TestReorgMempoolCapacityRespected`：accepted=1 want 0 | 误用 `mempool.New(0)`（实现语义为不限容量），却期望拒绝 | 改写为真容量测试：`mempool.New(1)` + 2 个可复活交易 → `accepted=1, rejected=1` |

---

## 9. 生产 datadir 状态

- 全部测试使用 `t.TempDir()`，无对生产 datadir 的写入。
- 本地仓库根 `p2pchain/` 未被测试触碰；远程 `.123` 生产节点（`~/p2pchain-longrun/datadir`）**未触及**（本阶段纯本地代码+测试，无部署）。
- 工作树为未提交增量，未 `go build` 产物外泄至生产路径。

---

## 10. 范围合规（Scope Compliance）

- ✅ 仅 4 个授权文件变更：`blockchain.go`(M)、`mempool.go`(M)、`service.go`(M)、`reorg_resurrection_test.go`(?? new)。
- ✅ 无 config / 第三方依赖 / 文档源码变更混入（docs 下若干 `.md` 为历史未跟踪报告，非本阶段新增）。
- ✅ 无生产 datadir 写入；测试全用临时目录。
- ✅ 无 git 写操作（未 commit/push）。

---

## 11. 已知限制（Known Limitations）

1. **`blockchain.go` 含超出「纯 mempool」冻结范围的改动**：1C 的 reorg 管线（`ReorgResult`/`AddBlockWithResult`/`executeReorg`/`SaveBlockDetached`）与本阶段的 `extendChain` 树维护 + Case 2 损坏防护均落在 `blockchain.go`（授权文件内），但属「为让 reorg 真正可运行所必需」的越界修复，已在 §7 显式记录。
2. **BT-1（blocktree flaky）**：`internal/blocktree` `TestI_RestartLikeReconstruction` 因 map 迭代序随机偶发失败，为**既存**缺陷，与本阶段无关；本次全量运行未触发，但后续引用「全量绿灯」前须显式排除或处理该包。
3. **损坏防护语义边界**：Case 2 损坏防护基于「哈希已存在于 canonical 链」；合法 fork 的 re-delivery（不在 canonical 链）仍走 `ErrDuplicateHash` 幂等，符合预期。
4. **CRLF 行尾警告**：autocrlf 规范化提示，非内容错误，提交阶段按仓库既定策略处理即可。

---

## 12. 下一步 / STOP

- **STOP**：本阶段到此结束，**不得提交**。
- **下一阶段**：`REORG-1F-COMMIT-READINESS-AUDIT`——审计提交就绪度（范围、测试、不变量、回归修复），再决定是否进入 `REORG-1F-COMMIT-1`。
- 如需进入提交阶段，请显式授权；届时建议单条 commit 涵盖 4 个文件，并在消息中注明 §7 的损坏回归修复。

---

*报告结束 — VERDICT: PASS（含回归修复）— STOP，未提交。*
