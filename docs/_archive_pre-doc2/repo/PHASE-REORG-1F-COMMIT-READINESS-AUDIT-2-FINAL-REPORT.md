# PHASE REORG-1F-COMMIT-1 — CONTROLLED GIT COMMIT EXECUTION

你现在进入：

**P2PChain — REORG-1F Mempool Resurrection / Controlled Commit Execution**

阶段性质：

> **STRICT CONTROLLED GIT COMMIT EXECUTION**

前置阶段：

> `PHASE REORG-1F-COMMIT-READINESS-AUDIT-2`

前置结论：

> **COMMIT READY**

---

## 1. 本阶段唯一目标

将已经通过 `COMMIT-READINESS-AUDIT-2` 的 REORG-1F 实现，以**严格受控、精确文件范围**执行一次 Git commit。

**本阶段只允许 Commit。**

禁止自动进入：

- push
- tag
- merge
- rebase
- amend
- squash
- deployment
- server synchronization
- Linux production deployment
- Windows mining test
- 新功能开发
- REORG-1G
- BT-1 修复
- F-3/F-4/F-5/F-6/F-7 修复

上述任何动作均需要新的独立授权。

---

# 2. Commit 前重新建立硬基线

执行任何 `git add` 前，必须重新检查：

```bash
git status --short
git branch --show-current
git log -1 --oneline
git remote -v
git diff --stat
git diff --cached --stat
```

确认：

- HEAD 仍为 `369d36e` 或 readiness audit-2 所确认的同一基线；
- 当前分支仍为 `main`；
- staging 必须为空；
- 不得出现 readiness audit-2 之后新增的意外 production 修改；
- 不得因为本阶段执行而自动清理、恢复或修改任何用户已有 dirty/untracked 文件。

如果基线与 readiness audit-2 不一致：

> **立即停止，不得 commit。**

---

# 3. 严格冻结 Commit 文件范围

本次 commit **只能包含以下 4 个文件**：

```text
internal/blockchain/blockchain.go
internal/mempool/mempool.go
cmd/node/service.go
internal/mempool/reorg_resurrection_test.go
```

执行：

```bash
git diff -- internal/blockchain/blockchain.go
git diff -- internal/mempool/mempool.go
git diff -- cmd/node/service.go
git diff -- internal/mempool/reorg_resurrection_test.go
```

然后只允许显式：

```bash
git add internal/blockchain/blockchain.go
git add internal/mempool/mempool.go
git add cmd/node/service.go
git add internal/mempool/reorg_resurrection_test.go
```

**绝对禁止：**

```bash
git add .
git add -A
git add --all
```

---

# 4. 明确排除以下内容

以下内容无论当前是否存在，都不得进入本次 commit：

```text
docs/
PHASE*.md
run-a/
run-b/
verifier/
其他 untracked 文件
其他 dirty 文件
任何生产 datadir
blocks.dat
node.lock
wallet.json
```

尤其注意：

> `run-a/`、`run-b/` 以及 `verifier/` 已在 readiness audit 中被明确识别为非 commit 内容。

如果 staged diff 出现任何上述内容：

> **立即停止并取消 staging，禁止继续 commit。**

---

# 5. Staged Diff 最终审计

完成四个文件的 `git add` 后，必须执行：

```bash
git status --short
git diff --cached --stat
git diff --cached --name-only
git diff --cached --check
git diff --cached
```

必须证明：

### 文件集合严格等于：

```text
internal/blockchain/blockchain.go
internal/mempool/mempool.go
cmd/node/service.go
internal/mempool/reorg_resurrection_test.go
```

不得多一个，也不得少一个。

---

# 6. Commit 内容边界

本次 commit 的逻辑必须保持为：

> **REORG-1F Mempool Resurrection**

核心内容：

- Architecture B；
- ReorgResult；
- disconnected transaction resurrection；
- coinbase exclusion；
- canonical-chain deduplication；
- post-reorg mempool revalidation；
- dependency-aware resurrection；
- capacity failure handling；
- service-layer integration；
- detached fork re-delivery idempotency；
- canonical duplicate protection；
- 相关回归测试。

不得借本次 commit 顺便修改：

- consensus rule；
- difficulty；
- PoW；
- monetary model；
- P2P protocol；
- mining algorithm；
- block format；
- storage architecture；
- BT-1；
- F-3/F-4/F-5/F-6/F-7；
- 新的 reorg policy；
- MaxReorgDepth；
- finality；
- mempool eviction policy。

---

# 7. Commit Message

推荐使用：

```text
feat: restore transactions after chain reorganization
```

只执行一次普通 commit：

```bash
git commit -m "feat: restore transactions after chain reorganization"
```

**不得执行：**

```bash
git commit --amend
git commit --fixup
git rebase
git merge
git reset --hard
```

---

# 8. Commit 后立即验证

commit 成功后执行：

```bash
git status --short
git log -1 --oneline
git show --stat --oneline HEAD
git show --name-only --format='' HEAD
git diff HEAD^ HEAD --check
```

确认：

1. commit 成功；
2. HEAD 已移动到新 commit；
3. commit 只包含上述 4 个文件；
4. commit diff-check PASS；
5. 未误提交 docs；
6. 未误提交 run-a；
7. 未误提交 run-b；
8. 未误提交 verifier；
9. 未误提交其他文件；
10. working tree 中原有 dirty/untracked 内容仍保持在 commit 外。

---

# 9. HEAD 稳定性检查

commit 后等待至少约 3 秒，然后：

```bash
git rev-parse HEAD
sleep 3
git rev-parse HEAD
```

两次必须一致。

如不一致：

> **立即停止并报告。**

---

# 10. 禁止 Push

即使 commit 完全成功：

**本阶段不得执行：**

```bash
git push
```

也不得：

- 创建 tag；
- 修改 remote；
- merge；
- rebase；
- amend；
- squash；
- 部署服务器；
- 修改 Linux 节点；
- 启动 Windows 挖矿实验。

---

# 11. 最终报告必须包含

请输出：

## PHASE REORG-1F-COMMIT-1 — FINAL REPORT

至少包含：

### A. Pre-Commit Baseline

- HEAD
- parent
- branch
- remote
- working tree
- staging state

### B. Commit Scope

明确列出实际 commit 的 4 个文件。

### C. Excluded Files

明确确认：

- docs 未提交；
- run-a 未提交；
- run-b 未提交；
- verifier 未提交；
- 其他 dirty/untracked 文件未提交。

### D. Commit Result

- commit hash
- short hash
- commit message
- parent hash
- files changed
- insertions/deletions

### E. Post-Commit Verification

报告：

- `git status`
- `git show`
- `git diff --check`
- HEAD stability

### F. Git Operations Boundary

明确确认：

```text
commit      = EXECUTED
push        = NOT EXECUTED
tag         = NOT EXECUTED
merge       = NOT EXECUTED
rebase      = NOT EXECUTED
amend       = NOT EXECUTED
squash      = NOT EXECUTED
deployment  = NOT EXECUTED
```

### G. Final Verdict

只有在以上全部满足时：

```text
REORG-1F-COMMIT-1 = PASS
```

否则：

```text
REORG-1F-COMMIT-1 = NOT READY / STOPPED
```

---

## 最重要的 HARD RULE

**本阶段只做一次精确的 4 文件 Git Commit。**

不要把“下一步”自动理解为 push。

Commit 完成后必须停下来，等待新的明确授权。
