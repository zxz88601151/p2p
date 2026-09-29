# 测试执行契约（TEST EXECUTION CONTRACT）

**状态：ESTABLISHED**（本文件即契约的版本化载体）
**建立阶段：`PHASE MNC-LIM4-TEST-INFRA-FIX`**
**适用对象：本仓库全部测试执行入口（本地 / 未来的 CI）**
**变更政策：本契约的任一参数变更须走治理门禁（见 §8）**

---

## 1. 为什么需要本契约

`go test` 的**每包默认超时是 `10m0s`**，这是 **Go 工具链的默认值**，不由本仓库定义，
也没有出现在本仓库的任何文件里（`grep -rn "timeout"` 在 `*.sh` / `Makefile` / `*.yml` 中命中 **0**）。

本仓库 `cmd/node` 包的全量测试耗时**恰好压在 600s 边界上**：

| 观测 | `cmd/node` | 结果 |
|---|---|---|
| `PHASE-MNC-IMPLEMENT-01` | 583.728s | 通过（余量 +16.3s） |
| `PHASE-MNC-LIM1-COMPAT-E2E` | 584.776s | 通过 |
| `PHASE-MNC-LIM1-LIM3-OWNER-DECISION` | 586.033s | 通过 |
| F-6.5 隔离 ×3 | 586.7 / 590.4 / **597.8s** | 通过（最坏余量 **+2.2s**） |
| `PHASE-MNC-LIMITATIONS-TECHNICAL-DISPOSITION-AUDIT` | **605.295s** | 🔴 已越过 600s（当时以 `-timeout 25m` 运行故未触发） |
| `PHASE MNC-LIM4-TEST-INFRA-FIX`（默认命令复现） | **600.532s** | 🔴 **`FAIL`**（`panic: test timed out after 10m0s`） |

**默认命令下的实测失败输出**（`go test ./... -count=1`，无 `-timeout`）：

```
FAIL	p2pchain/cmd/node	600.532s
ok  	p2pchain/internal/storage	377.793s
ok  	p2pchain/internal/blockchain	113.961s
ok  	p2pchain/internal/mempool	57.953s
ok  	p2pchain/internal/p2p	15.600s
...
FAIL

panic: test timed out after 10m0s
	running tests:
		TestResetAfterGracefulStopNeedsNoForce (0s)
```

**判别性证据**：整份输出中**没有任何 `--- FAIL:` 行**——失败**不是**任何测试的断言失败，
而是**包级预算耗尽**；且被点名测试的耗时显示为 `(0s)`（alarm 在它刚开始时触发）
⇒ 这是「**累计耗时超过默认预算**」，不是「某个测试挂起」，更不是代码缺陷。

> 结论：**默认 `go test ./...` 会对一个完全正常但较慢的测试套件产生假失败（false FAIL）。**
> 这不是覆盖率问题、不是 mock 问题、也不是测试正确性问题，而是**超时契约缺失**问题。

---

## 2. Canonical 命令

```bash
bash scripts/run-tests.sh
```

它等价于（`-count=1` 与 `-timeout` 均为契约的强制部分）：

```bash
go test ./... -count=1 -timeout 25m
```

**验收判定（唯一标准）**：

```
PASS   &&   exit = 0   &&   无 "test timed out" 出现
```

---

## 3. 契约参数

| 参数 | 值 | 强制性 | 依据 |
|---|---|---|---|
| `-timeout` | **`25m`** | **必须显式** | 最慢包 `cmd/node` 实测上界 **605.295s**；Go 默认 `10m`（600s）**不足**。25m = 1500s ≈ **2.48×** 实测上界，可容忍约 **2.4× 更慢**的主机（实测运行间波动已 >20%：`storage` 307.5s→377.8s，+23%）。同时保持**有界**——不设 `0`/无限，真实挂起仍会在有限时间内失败。 |
| `-count=1` | `1` | **必须显式** | Go 测试缓存会把「未变化」的包报成 `(cached)` 而**不真正执行**，使回归证据失真。 |
| 包范围 | `./...` | 默认 | 全仓；`TEST_PKGS` 可覆盖。 |

环境变量覆盖（用于隔离验证，**不改变默认契约**）：`TEST_TIMEOUT`、`TEST_COUNT`、`TEST_PKGS`。

**为什么不是「无超时」**：`-timeout 0` 会让真实挂起变成**无限等待**，破坏 CI 可终止性。
本契约要求「**足够宽 + 有界**」，而非「无界」。

---

## 4. 非 canonical 路径（明确不构成验收证据）

| 路径 | 状态 | 理由 |
|---|---|---|
| `go test ./...`（无 `-timeout`） | ❌ **不可用** | 继承 Go 默认 600s ⇒ 已实测 `FAIL`（§1） |
| `go test -short ./cmd/node` | ❌ **不可作为验收** | `cmd/node` 有 **5 处** `testing.Short()` 逃逸口（`p2p_branch_process_test.go:225/315`、`r3_crash_restart_test.go:64`、`r4b_dual_reorg_convergence_test.go:257/534`）⇒ **会跳过真实子进程/真实挖矿类测试，降低覆盖率**。仅可用于本地快速迭代。 |
| `GOFLAGS=-timeout=25m go test ./...` | ⚠️ 等价但**非 canonical** | 实测（go1.27.0）`GOFLAGS` 中的 `-timeout` **会被 `go test` 采纳**，且非测试命令（`build`/`vet`/`list`）会静默忽略它。但 `GOFLAGS` 是**环境状态、不随仓库版本化**、且不可见地影响该 shell 内所有 `go` 命令 ⇒ 不适合作为审计契约。 |
| 删除 / 跳过慢测试 | ❌ **禁止** | 见 §5 |

---

## 5. 契约禁止事项（任何「修复超时」的方案都不得触犯）

1. **不得**降低测试覆盖率。
2. **不得**删除真实 mining（真实 `pow.Mine` / `RPC Mine(n)`）。
3. **不得**删除真实 RPC（真实控制接口调用）。
4. **不得**删除真实 `go build` 子进程（`buildNodeBinary`）与真实 node 子进程。
5. **不得**把真实 E2E 改成 mock。
6. **不得**通过 `t.Skip` / `-short` / 缩短断言等待来「解决」超时。
7. **不得**为了让测试变快而修改任何 **production** 代码（共识 / 协议 / P2P / 挖矿 / 难度 / RPC / DB）。

> 本契约只调整**执行预算**，不调整**被测内容**。

---

## 6. Local / CI 一致性

- **本仓库当前不存在 CI 配置**（无 `.github/` / `.gitea/` / `.gitlab-ci.yml` / `.circleci/` / `Makefile`）。
  ⇒ **本地契约即唯一契约**；`scripts/run-tests.sh` 被设计为**同时**可直接用于本地与未来的 CI
  （纯 bash、无外部依赖、退出码透传、契约参数打印到 stdout 以便审计）。
- 未来引入 CI 时，**必须**调用同一入口（或使用同一 `-timeout` / `-count` 参数），
  不得另立一套隐式默认值。

---

## 7. 已知交互（记录，不在本契约范围内修改）

| 交互 | 说明 |
|---|---|
| `internal/storage/datalock_kernel_test.go:81` | 内核锁测试的 **helper 子进程**以 `exec.Command(os.Args[0], "-test.run=...")` 启动，**未显式传 `-test.timeout`** ⇒ 该 helper 继承 **Go 默认 10m**，与父进程的 `-timeout` **相互独立**。当前 helper 的 `time.Sleep(10*time.Minute)` 只是「等待父进程强杀」的屏障，父进程在秒级内强杀它 ⇒ 默认值不会触发。**属潜在耦合，已记录；本阶段未修改**（修改它需单独授权，且当前无实际风险）。 |
| `go test -race` | 需要 **cgo + C 工具链**。本机实测：`cgo: C compiler "gcc" not found` ⇒ **`-race` 在本环境不可用**。使用 `-race` 的环境须自行确保 gcc 可用，并同样附加 `-timeout`。 |

---

## 8. 变更政策

本契约的以下任一变更**必须**走治理门禁（不得在普通阶段顺手改）：

- `-timeout` 的默认值（`25m`）；
- `-count` 的默认值（`1`）；
- canonical 入口路径（`scripts/run-tests.sh`）；
- §5 禁止事项清单。

变更须附带：**新的实测耗时证据** + **余量重新计算** + **回归验证**。

---

## 9. 证据溯源

| 项 | 出处 |
|---|---|
| LIM-4 技术处置审计（`DISPOSITION = FIX`，仅测试基础设施） | `E:/wakuang/PHASE-MNC-LIMITATIONS-TECHNICAL-DISPOSITION-AUDIT.md`（sha `e14a4f8c…`） |
| 默认命令失败复现 | `PHASE MNC-LIM4-TEST-INFRA-FIX` STEP 1（`go test ./... -count=1` ⇒ `FAIL cmd/node 600.532s`，`exit 1`） |
| 本契约实施与验证 | `PHASE MNC-LIM4-TEST-INFRA-FIX` STEP 3–5 |
