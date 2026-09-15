# PHASE REORG-1C-COMMIT-1 — CONTROLLED COMMIT EXECUTION

严格执行 REORG-1C-COMMIT-1。

当前前置审计：

- REORG-1C-IMPLEMENTATION-1 = READY FOR COMMIT
- M1–M8 implementation complete
- build PASS
- vet PASS
- blockchain/storage/utxo tests PASS
- reorg crash/restart/invariant tests PASS
- race test PASS
- production datadir untouched
- legacy storage path preserved
- known BT-1 blocktree flaky test is pre-existing and MUST NOT be modified

## HARD SCOPE

本次只允许提交以下 4 个 REORG-1C 文件：

1. internal/blockchain/blockchain.go
2. internal/blocktree/blocktree.go
3. internal/storage/v2api.go
4. internal/blockchain/reorg_test.go

明确禁止将以下预-existing uncommitted files 纳入 commit：

- docs/DETERMINISTIC-SERIALIZATION-SPEC.md
- docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md

## PRE-COMMIT GATE

先执行只读基线：

- git status --short
- git diff --stat
- git diff --name-only
- git diff -- internal/blockchain/blockchain.go internal/blocktree/blocktree.go internal/storage/v2api.go internal/blockchain/reorg_test.go
- git diff -- docs/DETERMINISTIC-SERIALIZATION-SPEC.md docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md
- git diff --check

确认：

- 只有上述 4 个目标文件属于 1C
- 两个 docs 文件保持 unstaged / uncommitted
- 不允许通过 git add . 或 git add -A 扩大范围

## COMMIT

显式 git add 仅以下 4 个文件：

internal/blockchain/blockchain.go  
internal/blocktree/blocktree.go  
internal/storage/v2api.go  
internal/blockchain/reorg_test.go

然后检查：

- git diff --cached --name-only
- git diff --cached --stat
- git diff --cached --check

确认 staging 中恰好只有 4 个目标文件后执行：

git commit -m "feat: integrate canonical chain reorganization"

## POST-COMMIT VERIFICATION

commit 成功后立即执行：

1. git status --short
2. git diff --cached --check
3. git diff --check
4. git log -1 --oneline
5. git show --stat --oneline HEAD
6. git diff HEAD^..HEAD --check
7. git status --porcelain
8. HEAD stability check，至少等待 3 秒确认 HEAD 未变化

验证：

- commit 只包含 4 个目标文件
- 两个 pre-existing docs 文件仍然保持原状态
- working tree 中不存在由本阶段意外产生的其他修改
- staging 必须为空
- commit 成功后不得自动进行任何其他 Git 操作

## ABSOLUTE PROHIBITIONS

本阶段禁止：

- git push
- git tag
- git merge
- git rebase
- git amend
- git squash
- 修改 BT-1
- 修改测试断言以消除 BT-1
- 修改生产 datadir
- 修改服务器
- 部署
- P2P branch sync
- orphan pool
- mempool re-add
- mining template rebuild
- UTXO persistence
- MaxReorgDepth / finality
- 任何未授权的新功能

## FINAL REPORT

输出：

# PHASE REORG-1C-COMMIT-1 — FINAL REPORT

必须包含：

1. Pre-commit baseline
2. Exact staged files
3. Exact commit hash
4. Parent commit
5. Commit message
6. Commit diffstat
7. Post-commit verification
8. Working tree / staging state
9. Confirmation that both pre-existing docs remained untouched by this commit
10. Confirmation that no push/tag/merge/rebase/amend/squash occurred
11. BT-1 remains documented as pre-existing and unchanged
12. Final verdict

最后必须明确：

`REORG-1C-COMMIT-1 = PASS` 或 `FAIL`

完成后 STOP，不得自动进入下一阶段。

