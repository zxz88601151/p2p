# PHASE GENESIS-0.1 — P0 PERSISTENCE REMEDIATION

> 项目：P2PChain
> 前置阶段：PHASE GENESIS-0（结论：`PHASE GENESIS-0 = FAIL`，存在 P0 存储缺陷）
> 阶段性质：DEFECT REMEDIATION / REGRESSION / ISOLATED COMMIT
>
> **归档说明**：本报告当时以对话文本交付，未落盘；
> 于 **PHASE BRAND-1.2**（§13 GENESIS DOCUMENTATION）依据当时的原始证据正式写入 `docs/`。
> 内容为历史事实记录，未重新编造实验。

---

## 0. 阶段结论

```text
PHASE GENESIS-0.1 = PASS
提交：90e9fbb  fix(blockchain): prevent persistence during chain replay
父提交：f49b1ca
```

---

## 1. HARD BASELINE

| 项 | 值 |
|---|---|
| HEAD（修复前） | `f49b1ca04234981c4410b168ca9eb2b7cfb82384` |
| HEAD^ | `535edb71e7aecbd6e100534bb65b595e665dd26a` |
| branch | `main` |
| Go / Git | go1.22.12 windows/amd64 / git 2.55.0 |
| working tree | 三个既有变化全程保留：`M docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md`、`?? PHASE-BRAND-0D.3-FRESH-VERIFICATION.md`、`?? PHASE-P2PCHAIN-BASELINE-0.1-REPORT.md` |

---

## 2. 根因复确认

`AddBlock` 的全部生产调用方只有三处，且语义不同：

| 调用点 | 是否需要持久化 |
|---|---|
| 挖矿产出（`cmd/node/main.go` `mineOnce`） | **需要** |
| P2P 收到区块（`cmd/node/service.go` `addBlockAndUpdatePool`） | **需要** |
| 启动回放历史区块（`blockchain.NewBlockchainFromStore` 循环） | **不需要**（区块本来就来自磁盘） |

因此修复方向是**区分入口**，而不是在存储层做静默去重。

---

## 3. 修复方案（Option A，未做 Option B）

把「追加」拆成两个语义明确的入口，共用同一实现：

```text
AddBlock()   → 运行时新区块（挖矿产出 / P2P 收到）：校验 + 入内存 + 落盘
applyBlock() → 启动回放历史区块：校验 + 入内存 + 不落盘
addBlock(persist bool) → 唯一实现
```

回放循环改为调用 `applyBlock`，从而保证：

```text
Replay = read + validate + reconstruct
```

不再是 `read + validate + SaveBlock`。

**为何不做 Option B（在 `SaveBlock` 里加 `if exists { return nil }`）**：
append-only 日志里做静默去重正是规格明令禁止的「用 no-op 掩盖错误」；
Option A 已彻底关闭该路径，B 只会掩盖未来同类缺陷。

---

## 4. 文件变更

| 文件 | 变化 |
|---|---|
| `internal/blockchain/blockchain.go` | 约 +27 / −4（提交统计 31 行改动） |
| `internal/blockchain/replay_persistence_test.go` | 新增 265 行 |

---

## 5. 新增回归测试（4 个）

| 测试 | 覆盖 |
|---|---|
| `TestRestartDoesNotDuplicatePersistedBlocks` | 重启不改 `blocks.dat` 大小；持久化记录数 == 链高 + 1 |
| `TestSecondRestartSucceeds` | 第二次重启成功 |
| `TestRuntimeBlockIsStillPersisted` | 运行时新块仍正常落盘（防止「干脆不 SaveBlock」的伪修复） |
| `TestDuplicatePersistedRecordIsRejected` | 重复记录存储被明确拒绝（负向） |

**反向验证（关键）**：把 `blockchain.go` 还原到 HEAD 版本后，这批测试**必 FAIL**，
且报错是 **`blocks.dat 大小 = 540, want 360`** ——
与 GENESIS-0 实机观测的字节数**完全一致**，单测精确复现了线上现象。

**为什么既有 `TestChainPersistsAcrossRestart` 抓不到这个 P0**：
它只断言 Height / TipHash / Balance —— 重复落盘后这三项**依然全部正确**
（`store.Height()` 在写入前读取），且它从不重开第三次。
缺的断言是：**文件大小、记录数、第二次重启**。

---

## 6. 全量回归

| 项 | 结果 |
|---|---|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test -count=1 ./...` | **12 包全 ok，0 FAIL** |
| `go test -race ./...` | **全 ok，RACE_EXIT=0** |
| `scripts/smoke-e2e.sh` | **15/15 PASS** |

**已知既有 flaky（与本阶段无关，未修）**：全量并发跑时
`internal/p2p` 的 `TestBroadcastBlockAndRelayExcept` 偶发失败（单独跑 5/5 PASS）。
判定无关的依据：`go list -deps -test ./internal/p2p | grep blockchain` = **0**，
p2p 测试二进制根本不含被修改的包。

**另一条教训**：verbose 全量跑时 `TestPanicPathReleasesLock` 的 `--- FAIL:` 行
是**子进程回显**（该测试会 `exec` 自己）并嵌套在父进程 `t.Logf` 里，
naive grep 会误判为失败；真实父进程结果是 PASS。

---

## 7. 实机验证（全新临时目录，未使用 `~/.p2pchain`）

| 观察项 | 数值 |
|---|---|
| Genesis hash | `0000aca1af720da390380a38dc97d141cfb67afe578a2d31315860a5489db58c` |
| Block #1 hash | `0000975dd06358610687afb8dda63fd50ba93ebb5a072af7663f68ddd6af497f` |
| Height | 1 → 2（第 3 次重启后又挖了 1 块） |
| 持久化记录数 | 2 → 3，恒等于**链高 + 1** |
| `blocks.dat` 大小 | 360 → **360**（重启 #1）→ **360**（重启 #2）→ 540（挖新块后）→ **540**（重启 #3） |
| 重启 #1 / #2 / #3 | **全部成功**（#2 正是修复前 `EXIT=1` 的那一步） |
| 锁生命周期 | PID **13552 → 11776 → 4960 → 2868**，每次先释放再获取 |

五次关停全部使用 Windows 优雅关停法（CTRL_BREAK → SIGTERM），
**未用 `taskkill /F` 充当证据**。

**Before / After 对照**

```text
Before：360 → 540（重复回放）→ 720（失败重启又追加一条）→ 第二次重启 EXIT=1
After ：360 → 360 → 360，三次连续重启均成功；size 只因「挖出新块」才增长
```

**已损坏数据不做自动修复 / 迁移**（超出本阶段范围）：
GENESIS-0 遗留目录用修复后的二进制启动，仍是明确报错，而非静默通过。

---

## 8. 提交与 Git 纪律

```text
90e9fbb  fix(blockchain): prevent persistence during chain replay
parent   f49b1ca
files    2 files changed, 292 insertions(+), 4 deletions(-)
         internal/blockchain/blockchain.go              |  31 ++-
         internal/blockchain/replay_persistence_test.go | 265 +++
```

- 显式 `git add` 两个文件，**未用 `add .` / `add -A`**；三个既有 docs 未被 stage / commit
- HEAD 稳定性：立即与间隔 4 秒两次取值均为 `90e9fbb`
- 未执行 push / tag / merge / rebase / amend / squash / reset / clean
- 补充事实：`packed-refs` 仍显示 `50449cd`（本环境既有的 loose-ref 优先级怪象，
  loose ref 已正确指向 `90e9fbb`），未伪造其内容

---

## 9. 对后续阶段的意义

本次修复顺带确立了一条可复用的产品级不变量：

```text
Replay = read + validate + reconstruct（绝不写回）
Runtime append = validate + persist
```

这条不变量后来成为 **PHASE BRAND-1.2** 中 `verify`（只读回放校验）
能够成立的技术前提 —— 因为回放路径已经保证「不为了校验而修改被校验的数据」。
