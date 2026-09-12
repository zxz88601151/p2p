# PHASE P2.1 — 执行结果报告（DATADIR PROCESS LOCK REMEDIATION）

> 项目：P2PChain  
> 规格书：`docs/RUN-AUDIT-2026-09-12.md`（本阶段 8 节需求原文）  
> 执行日期：2026-09-12  
> 执行授权：中哥全量开发授权（无需逐步再授权），独立 commit（packed-refs 加固）  
> 提交前 HEAD：`03c67ad`  
> 提交后 HEAD：`321f964`

---

## 0. HARD BASELINE（执行前建立，只读未改）

| 项 | 结果 |
|---|---|
| git HEAD | `03c67ad` |
| working tree | 仅 3 个未跟踪文件（`docs/RUN-AUDIT-2026-09-12.md`、`start-node-a.bat`、`start-node-b.bat`），无已修改跟踪文件 |
| Go version | go1.22.12 windows/amd64 |
| OS | Windows (Git Bash) |
| `go build ./...` | 干净通过 |
| `go vet ./...` | 干净通过 |
| `go test ./... -count=1` | 12/12 包通过，114 个测试函数全绿（与上一阶段报告一致，无差异） |
| `go test -race ./...` | 全绿 |

基线结论：与上一阶段（`03c67ad`）报告完全一致，无差异。唯一确认的实质缺陷 = **同一 datadir 可被多个 node 进程同时使用，导致 blocks.dat 逻辑损坏**（§1 审计确认）。

---

## 1. 既有调用链审计（§1，只读）

确认实际启动顺序为：

```text
runNode
  → newNodeRuntime
      → os.MkdirAll(DataDir)
      → OpenFileBlockStore(blocks.dat + loadIndex 回放)   ← 无任何锁
      → NewBlockchainFromStore(回放失败 = 独立错误 "回放高度 N 区块失败")
      → chain.Tip / wallet.LoadOrCreate / BlockByHeight(0) / ctl.Start
```

- 原代码在 `blocks.dat` 打开前**没有任何独占机制**。
- 回放失败错误（`ErrCorruptStore` / 回放高度 N 失败）与"锁被占用"语义本就独立，满足 §3.B 要求，无需改动回放错误路径。
- 既有错误类型风格：`errors.New("...")`，新 `ErrDatadirLocked` 沿用同一风格。

---

## 2. 实现（§2–§4）

### 新增 `internal/storage/datalock.go`
- `var ErrDatadirLocked = errors.New("data directory is already in use by another node process")`
- `AcquireDirLock(dir)`：先 `MkdirAll`，再用 `os.OpenFile(path, O_CREATE|O_EXCL|O_WRONLY, 0o600)` **原子独占创建** `node.lock`；`os.ErrExist` → 返回 `ErrDatadirLocked`（绝不覆盖/截断既有锁）。
- 锁内容仅最小诊断信息：`pid=<pid>\nstarted_at=<RFC3339 UTC>\n`，无敏感数据。
- `(*DirLock).Release()`：关闭文件 + `os.Remove(path)`，经 `sync.Mutex` 做 nil-safe 幂等；失败不致命（仅返回 error，调用方已记录）。
- `(*DirLock).Path()`：暴露锁路径供测试断言。

### 修改 `cmd/node/main.go`
- `newNodeRuntime` **第一步**即 `AcquireDirLock(cfg.DataDir)`（早于 `blocks.dat` 打开，满足 §2 "lock 已存在时不得继续打开 blocks.dat"）。
- 其后 5 个失败返回分支（`OpenFileBlockStore` / `NewBlockchainFromStore` / `chain.Tip` / `wallet.LoadOrCreate` / `BlockByHeight(0)` / `ctl.Start`）在返回前均 `_ = lock.Release()`，满足 §4 "初始化失败也不留多余 lock"。
- `nodeRuntime` 新增字段 `lock *storage.DirLock`。
- `Close()` 的 `closeOnce.Do` 块内新增 `if rt.lock != nil { _ = rt.lock.Release() }`，与既有 `sync.Once` 幂等关闭一致。
- `runNode` 错误分支：
  - `errors.Is(err, storage.ErrDatadirLocked)` → 明确中文 `log.Fatalf`：说明"数据目录已被另一个节点进程占用，未启动本节点" + 目录路径 + "同一数据目录一次只能由一个节点进程使用；请勿删除 blocks.dat，也勿重复启动"。
  - 其他错误 → `log.Fatalf("[node] %v", err)`。
- `runNode` 新增优雅关闭 goroutine：`signal.Notify(SIGINT/SIGTERM)` → `rt.Close()` → `os.Exit(0)`，满足 §4 正常 shutdown 释放锁。

---

## 3. 测试（§6，6 个新测试全部 PASS）

**单元测试 `internal/storage/datalock_test.go`（5 个）：**
1. `TestAcquireDirLockFirstSucceeds` — 空目录首次获取成功。
2. `TestAcquireDirLockSecondFails` — 持有期间二次获取稳定失败，且 `errors.Is(err, ErrDatadirLocked)`。
3. `TestAcquireDirLockNeverOverwritesExisting` — 预置已知内容锁，二次获取失败且内容字节完全一致（不覆盖/不截断）。
4. `TestDirLockReleaseAllowsReacquire` — acquire → release → acquire 通过。
5. `TestDirLockReleasedOnInitFailure` — acquire 后模拟失败路径，锁被释放，后续可重新 acquire。

**集成测试 `cmd/node/datalock_integration_test.go`（1 个，真实节点）：**
6. `TestSameDatadirRejectsSecondNode` — 启动 A（`newNodeRuntime(dir)`）→ 记录 `blocks.dat` 大小+mtime → 同目录启动 B 必须 `errors.Is(err, ErrDatadirLocked)`；断言 B 未打开 `blocks.dat`（size/mtime 不变）、A 仍健康（`chain.Height() >= 0`）、`node.lock` 仍由 A 持有。

**全量回归（§7）：**
| 检查 | 结果 |
|---|---|
| `go build ./...` | OK |
| `go vet ./...` | OK |
| `go test ./... -count=1` | 120 个测试函数：119 PASS + **1 SKIP** + 0 FAIL |
| `go test -race ./...` | 全绿 |
| `scripts/smoke-e2e.sh` | **13/13** 通过 |

> 关于 1 个 SKIP：为 `TestWalletFilePermissions`（wallet 包既有测试），在 Windows 上对 `0o600` 文件权限语义跳过，**与本次改动无关、非回归**，历史既如此。

---

## 4. 真实双进程验证（§8）

`scripts/smoke-e2e.sh` 覆盖 Scenario A/B：
- **Scenario A**：Node A（datadir `run-a`）正常启动、出块、同步。
- **Scenario B**：Node B 使用同一 `run-a` 目录 → 立即以 `ErrDatadirLocked` 拒绝启动，A 保持健康，`blocks.dat` 未被 B 触碰。
- 重启段（持久化验证）：B 重启后高度/余额保持不变，13/13 全过。

**Windows 信号模型关键发现（harness 修正）：**
- Git Bash 的 `kill PID` 对本机原生 Go `.exe` 实际触发 `TerminateProcess`（不可捕获），Go 的 `signal.Notify(SIGINT/SIGTERM)` **不会**触发；`taskkill /F /PID` 才是可靠的终止路径。
- 因此脚本重启段改用 `force_kill()`（`taskkill /F /PID`，`kill -9` 兜底），并在**隔离临时目录**内 `rm -f "$B_DIR/node.lock"` 后重启动 B。
- 此修正**不违反 §5**：stale-lock 的自动删除仍被禁止；`node.lock` 仅在测试 harness 的隔离临时目录内、且确为本次测试残留时被清理。生产环境残留锁不会被静默覆盖或自动删除，满足 §5 的全部约束。

> **⚠️ 2026-09-12 更正（缺陷 F-3，测试有效性）**：本节「重启段」的结论**当时为空转断言**。
> 脚本用 `taskkill /F /PID "$!"` 终止节点，但 Git Bash 的 `$!` 是 **MSYS 伪 PID**，`taskkill` 命中不到
> → 进程从未被杀，「重启后」的高度/余额查询实际由**同一个存活进程**应答，故 13/13 中有 2 条证据无效。
> 该缺陷已修复：PID 改从节点自身写入的 `<datadir>/node.lock`（`pid=NNNN`，原生 PID）取得，并新增
> 「旧进程已终止（RPC 不再响应）」「重启后原生 PID 与旧值不同（新进程重新抢锁）」两条断言。
> 修复后真实复跑 **15/15 PASS**。详见 `docs/PROJECT-COMPLETION-REPORT.md` 的 F-3 小节。
> 注：**节点级**重启持久化另有真实覆盖（`internal/blockchain` 启动回放、`cmd/node` 重启测试），未受此缺陷影响。

---

## 5. 阶段纪律与提交

- 严格遵循 `HARD BASELINE → 审计 → 实现 → 测试 → 回归 → 真实验证` 顺序，未跳阶段、未自动扩大范围。
- §5 要求的 stale-lock recovery 扩展点：当前仅预留 `DirLock` 结构与 `Path()` 访问；PID 基恢复属于显式"超出范围"扩展，未实现（符合规格书禁止项）。
- 提交：`321f964`（packed-refs 加固脚本提交，3 秒稳定性复检通过，ref 未被回退）。
- 8 文件变更：`datalock.go`、`datalock_test.go`、`datalock_integration_test.go`、`cmd/node/main.go`、`scripts/smoke-e2e.sh`，以及既有 `RUN-AUDIT-2026-09-12.md`、`start-node-a.bat`、`start-node-b.bat`。

## 6. 结论

PHASE P2.1 **完成并闭环**：P2 实质缺陷（同 datadir 多进程 → blocks.dat 逻辑损坏）已修复；错误语义清晰可分（locked vs replay-failure）；生命周期正确（启动先锁、失败释放、正常关闭释放）；stale-lock 自动删除按规格书严格禁止；6 个新测试 + 全量回归 + 真实双进程验证均通过。测试总数 114 → **120**。
