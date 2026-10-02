# PHASE F-6.6 — COMMIT-CANDIDATE-FINAL-MANIFEST

| 字段 | 值 |
|---|---|
| Phase ID | **F-6.6** |
| Companion | `OWNER-COMMIT-BOUNDARY-DECISION.md`（决定记录） |
| Date (local) | 2026-09-28 08:38:36 +0800 |
| Baseline HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| Branch | `main` |
| Decision | **OPT-B** — CB-1 (F1-N1) → CB-2 (Genesis Closure) |
| Boundary size | **33 文件** |
| Mode | STRICT READ-ONLY（本清单**不执行**提交） |

---

## §1 — BOUNDARY SUMMARY

| 提交 | 名称 | 文件数 | 前置条件 |
|---|---|---|---|
| **CB-1** | F1-N1 (X-01) | **1** | OD-03 解阻 |
| **CB-2** | Genesis Closure = A(7) + B(22) + F(3) | **32** | CB-1 完成 |
| **TOTAL** | **Genesis Closure Commit Boundary** | **33** | — |

**提交顺序强制**：`CB-1 → CB-2`（CB-2 的测试编译依赖 `f1n1UTXOSig`，唯一定义在 CB-1）。

---

## §2 — CB-1：F1-N1（1 文件）

| # | 路径 | SHA-256 | size | git | 归属 | 备注 |
|---|---|---|---|---|---|---|
| 1 | `internal/blockchain/f1n1_reorg_coverage_test.go` | `c4606f09829ccc02f6a057b48566c498a0e3af73338d82b898368643c88e438a` | 25,981 B | `??` | **F1-N1**（OD-03） | 定义 `f1n1UTXOSig` 等夹具；**含时序敏感 flaky 用例** `TestF1N1_C_MixedLegacyV2Reorg` |

> **前置**：Owner 须将 F1-N1 授权为**独立提交阶段**（OD-03 当前为 `BLOCKED`；
> 其授权报告位于仓库之外）。**F-6.6 不执行该授权。**

---

## §3 — CB-2：Genesis Closure（32 文件）

### 3.1 组 A — F-3B 生产（7 文件，tracked modified）

| # | 路径 | SHA-256 | size | git |
|---|---|---|---|---|
| 1 | `cmd/node/main.go` | `f22436ba406feb3abc5ad15b77d5915d46f389375a42732ae3b93bd693ec890b` | 36,406 B | `M` |
| 2 | `cmd/node/cli.go` | `7d9b1c67d4d0012f7aa15256d456ebb4cf9ae867ed9d88a3b9520162e0a6552d` | 34,746 B | `M` |
| 3 | `internal/blockchain/blockchain.go` | `8cf75cb11c5a9e0284add7257400f1ae5b79d9cb8f21ce3feaa27a100345b3ac` | 44,683 B | `M` |
| 4 | `internal/blockchain/verify.go` | `05db33fa2b276a9c40480ab7ef3be1ef920179911c2431944392d6b5450dfbf5` | 3,450 B | `M` |
| 5 | `internal/blockchain/genesis.go` | `b6a2c2235a70d4fd98dc12aca06911c3f7a7a119916601c52cc329a93537362f` | 2,196 B | `M` |
| 6 | `internal/storage/file.go` | `1385fc7433470cd8a5e971d3591f5c9d881e4bfdec08dd78ca21471f96c1699a` | 8,275 B | `M` |
| 7 | `internal/storage/v2.go` | `95e7d9edda30fe3379f5ebb59ea62dcd70a5ee1712e3b089d6a2002fd0b7bc1e` | 30,856 B | `M` |

> **组 A 是 Genesis 的唯一生产变更**（+ `identity.go`）⇒ **生产编译闭包 PCC = 8 文件**（F-6.6 §4.2）。

### 3.2 组 B — F-3B 夹具（22 文件，tracked modified）

| # | 路径 | SHA-256 | size | git |
|---|---|---|---|---|
| 8 | `cmd/node/datalock_integration_test.go` | `48b72ac80eab229a19b65522eacfffc9fa4b0d10234c17945757ac9bdb95fb6d` | 2,551 B | `M` |
| 9 | `cmd/node/fullstack_test.go` | `bc14215436319e4897913d375fe22fd2ff9dc91b2904d7c362c4bc9d8328307d` | 17,471 B | `M` |
| 10 | `cmd/node/lock_lifecycle_test.go` | `fbb6e8b07f975ed810f91bc59363c091b5997db6f052203bc177481db04891ae` | 11,274 B | `M` |
| 11 | `cmd/node/p2p_branch_test.go` | `21a255f471ce104f866ab75bb34768be8a221f392fbc47615e9db13bf36dfb05` | 32,097 B | `M` |
| 12 | `cmd/node/r3_crash_restart_test.go` | `4a25cb3aa8769a2c4acd40fc59dfa16d9828aac3dcd027a9ca2f29e7ed580966` | 7,982 B | `M` |
| 13 | `cmd/node/r4b_dual_reorg_convergence_test.go` | `41879fcd121d851f5eaa5b987430d070867ca718741ab50795ef520a34829dbc` | 23,791 B | `M` |
| 14 | `cmd/node/reset_test.go` | `d74ce70348fc5d80525e7c6555e8cd8459085fc14949f17ea281234b193f4d44` | 16,388 B | `M` |
| 15 | `cmd/node/seed_parallel_test.go` | `0eedc0bc847abf420bc329a939aaa59b456554109a61b109957dccb4f67c1455` | 4,141 B | `M` |
| 16 | `cmd/node/stop_lifecycle_test.go` | `08d4a782fed63f9424c5fcf7b4b0c2fc0fc2fc5fbc3011a92d931db80faadb2f` | 20,966 B | `M` |
| 17 | `cmd/node/verify_test.go` | `6b55ba166eda6e1d233f1e09606ba99ef705cf26ee2628ddb66f2d46f4cef755` | 11,960 B | `M` |
| 18 | `internal/blockchain/blockchain_test.go` | `503d5ed825aa65b23636b051cbfb7b51e92abc70535362e58beda5d64a92c71e` | 19,435 B | `M` |
| 19 | `internal/blockchain/c1_hashindex_test.go` | `404ce9e590c237059bb73372c84e077511086286249342e03df8b551b620001e` | 7,319 B | `M` |
| 20 | `internal/blockchain/c1_noninterference_dump_test.go` | `b0968cbe959018c7ab4c59c61c1a92dc478317382a8546dc84d43b5a8371073d` | 3,399 B | `M` |
| 21 | `internal/blockchain/cd_canonset_test.go` | `de4dc51c3c73319d04c47cc2e8e31d6befc768a8d75e05eb555e000538ab3159` | 15,826 B | `M` |
| 22 | `internal/blockchain/difficulty_consensus_test.go` | `15237054fb57da34d5d6b54c007381414c7fb36d1d6f23c7fb951c8c408ce6ae` | 25,843 B | `M` |
| 23 | `internal/blockchain/obs_noninterference_test.go` | `13f6c6ed2650d7fba1787d5fe94456781cc07cd1955410829385ac4987a0cd72` | 9,041 B | `M` |
| 24 | `internal/blockchain/reorg_test.go` | `31487b3fd9a05a359ad9dd7eb3c8e9662fe12505f2d8c4f18dc028eb3b7f3898` | 18,074 B | `M` |
| 25 | `internal/blockchain/replay_persistence_test.go` | `c9b53c99eec68fc25df12ef67d3b0f6ffd1f6634ecec5aa08a492dd3e1253917` | 8,968 B | `M` |
| 26 | `internal/mempool/reorg_resurrection_test.go` | `2ebab35845206d08ad7f517ac8b2bff13fea946181a5f02df3ce2578abfa4c86` | 20,841 B | `M` |
| 27 | `internal/storage/r3_crash_matrix_test.go` | `158f60f0789352831ed2eabe5ae45508a58fcb8a7d46224c90b2b4f5a6937569` | 30,965 B | `M` |
| 28 | `internal/storage/r4a_legacy_length_matrix_test.go` | `7bd1acbd07bd9b6465fdf6ba28826a6ffa3fe9770aaf3381e68382fe9c1e1ce0` | 45,856 B | `M` |
| 29 | `internal/storage/v2_test.go` | `e580439600627420deaca1a109195cbbc8c22d6ecf7da4d493d35711fd4eae54` | 32,067 B | `M` |

> **组 B 的作用**：F-3B 改变了 `newNodeRuntime` / `NewBlockchainFromStore` 的**语义**
> （不再自动建创世）⇒ HEAD 版夹具在**运行期**失败。故组 B 是**测试闭合必需**、**编译可选**（M5 exit 0）。

### 3.3 组 F — F-3B 新源（3 文件，untracked）

| # | 路径 | SHA-256 | size | git | 依赖角色 |
|---|---|---|---|---|---|
| 30 | `internal/blockchain/identity.go` | `9aa8fc6e28f8ee580749e3cb0b20f31c650f32071f2a28b9b0ac3cb5a61e29d6` | 3,207 B | `??` | **生产必需**：定义 `ErrUninitializedStore` / `VerifyGenesisIdentity` / `InitializeBlockchainStore` |
| 31 | `cmd/node/f3b_test_helpers_test.go` | `4bd3d1232afc1e53b27022d46d67b41315ec7a50aeb1e0ca23159b29806be5e4` | 763 B | `??` | **条件必需**：定义 `newNodeRuntimeForTest`（组 B 工作树形态引用 11 处） |
| 32 | `cmd/node/f3b_identity_test.go` | `b7a9e5a7b4790716190b85f7262968e1d5ade8cc0fbee1212778a42cb9cfe4b1` | 5,781 B | `??` | 编译可选；**Genesis 测试闭合必需**（`TestF3BExplicitInitializationStateMatrix`） |

---

## §4 — COMBINED BOUNDARY（33 文件）

| 分组 | 数量 | tracked `M` | untracked `??` |
|---|---|---|---|
| CB-1（X-01） | 1 | 0 | 1 |
| CB-2 · 组 A | 7 | 7 | 0 |
| CB-2 · 组 B | 22 | 22 | 0 |
| CB-2 · 组 F | 3 | 0 | 3 |
| **合计** | **33** | **29** | **4** |

> **29 tracked + 4 untracked = 33**（与 F-6.4 GCS 构成一致）。

---

## §5 — EXCLUDED（排除清单）

### 5.1 SET B（6 文件）— 独立工作流，须各自提交

| # | 路径 | SHA-256 | size | git | 归属 |
|---|---|---|---|---|---|
| E-1 | `cmd/node/r1_boundary_test.go` | `ada6a89f0cc9a55af0a0a215292a82f3124fd1472a52909d704dff8098d23061` | 13,034 B | `M` | R1 |
| E-2 | `internal/explorer/explorer_test.go` | `cd1d421695cb1a1adcb97f5488ae276f5ee3f70306990b111911e4a355b48ed3` | 19,709 B | `M` | AUDIT-FIX |
| E-3 | `internal/control/server.go` | `ab84dcf0d9d69d34bcdc7672be12e94c83d533ef0191a7de00abf226413ea355` | 32,059 B | `M` | CONSOLE |
| E-4 | `internal/control/console_test.go` | `10cb3d652c811553b3da2a42fa15551df0d3d6d9e45670ffc2f6a5f349f82120` | 20,473 B | `M` | CONSOLE |
| E-5 | `internal/control/web/console.html` | `dd8eba2d372bcdd73deee9ee190d591599a3615d5e8990f6d840d6340c966b6e` | 35,351 B | `M` | CONSOLE |
| E-6 | `internal/control/console_mine_auth_test.go` | `b59a275e0fc31da46408ae139a9d45a85d6df7b76593375a77059b6efb54467b` | 7,476 B | `??` | CONSOLE |

### 5.2 其他排除

| 类别 | 文件 | 数量 | 依据 |
|---|---|---|---|
| GAP-4 | `internal/attribution/{attribution,classify,attribution_test}.go`、`cmd/coinbase-attribution/main.go` | 4 | OD-04/OD-05 SHA-DRIFT BLOCKER |
| F1-N1/N2 残余 | `internal/blockchain/f1n2_reorg_detached_undo_test.go`、`internal/storage/f1n1_undo_contract_test.go`、`internal/storage/f1n2_detached_undo_contract_test.go`、`internal/utxo/f1n1_undo_content_test.go` | 4 | OD-03/OD-06（报告在仓库之外） |
| D 类文档 | `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` | 1 | OD-02（须独立验证者） |
| CRLF 伪差异 | `internal/blockchain/query.go` | 1 | 无内容变更 |
| Secrets | `audit-run/control-token`、`f5-verify/control-token`、`gui-test/token` | 3 | **OD-08 永久排除** |
| 历史报告 / docs | 仓库根 35 份 `PHASE-*` + `docs/` 147 份 | 182 | 须独立文档提交决策 |
| 工具 / 工作区 | `.workbuddy/`、`gui/`、`F-1-lab/`、`concept-mvp/`、`verifier/`、`audit-run/`、`f5-verify/`、`gui-test/` | — | 非源 |

---

## §6 — COMMIT CONSTRAINTS（提交约束，OD-08）

```
★ 绝对禁止：git add -A  /  git add .  /  git add -a  /  git commit -a
★ 必须：显式路径白名单（逐文件 add），或 git add -- <explicit paths...>
★ 理由：3 个未忽略 token（audit-run/control-token、f5-verify/control-token、gui-test/token）
        不在 .gitignore 中，任何广义 add 都会将其纳入暂存区。
```

| 约束 | 内容 |
|---|---|
| C-1 | 仅可 `git add -- <§2/§3 表中逐条路径>`；**禁止**任何广义 add |
| C-2 | CB-1 必须先于 CB-2 提交 |
| C-3 | 提交后须验证：`git ls-files --stage | grep -i token` 为**空** |
| C-4 | 提交后须验证：3 个 token 仍 `tracked=NO` 且 `commits=0` |
| C-5 | 提交后须验证：`internal/blockchain/query.go` **未**进入暂存（CRLF 伪差异） |
| C-6 | `.gitignore` **不得**在本边界内修改 |

---

## §7 — VERIFICATION COMMANDS（边界验收，供 COMMIT 阶段复用）

```bash
# G-L1 编译闭合（在含 CB-1 ∪ CB-2 的工作树上）
go build ./... && go vet ./...                       # 期望 exit 0

# G-L2 Genesis 确定性测试（隔离）
go test -count=1 ./internal/blockchain/...           # 期望 exit 0

# G-L2b Genesis 自有测试
go test -count=1 -run 'TestF3BExplicitInitializationStateMatrix' ./cmd/node   # 期望 exit 0

# G-L3 全套（跟踪项，不阻断 CB-2 — D-6）
go test -count=1 ./...                               # 已知 exit 1（非 Genesis；F-6.5 §16）
```

---

## §8 — SHA REGISTRY（汇总）

| 分组 | 文件数 | SHA-256 已登记 |
|---|---|---|
| CB-1 | 1 | ✅ |
| CB-2 · A | 7 | ✅ |
| CB-2 · B | 22 | ✅ |
| CB-2 · F | 3 | ✅ |
| **边界合计** | **33** | ✅ **全部登记** |
| SET B（排除） | 6 | ✅ |
| GAP-4（排除） | 4 | ✅ |

---

## §9 — STATUS

```
MANIFEST STATUS: FINAL / FROZEN
  Boundary      = 33 文件 (CB-1: 1, CB-2: 32)
  Excluded      = SET B(6) + GAP-4(4) + F1-N1/N2 残余(4) + docs + secrets + 实验产物
  Execution     = NOT PERFORMED (本清单不执行任何 git 写操作)
  Prerequisite  = Owner 解阻 OD-03 (F1-N1 独立提交授权)
  HARD STOP     = after decision record
```

*PHASE F-6.6 — COMMIT-CANDIDATE-FINAL-MANIFEST — 自引用文档，SHA-256 于收工后由独立命令计算并登记于文件之外。*
