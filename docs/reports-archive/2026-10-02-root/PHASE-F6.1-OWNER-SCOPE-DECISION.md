# PHASE F-6.1 — OWNER SCOPE DECISION

**PHASE F-6.1 — OWNER SCOPE RECONCILIATION (STRICT READ-ONLY)**
**Deliverable 1 of 3**

| 项 | 值 |
|---|---|
| 仓库 | `E:/wakuang/p2pchain` |
| 基线 HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| staged | 0 ｜ tracked 内容修改 36 ｜ porcelain ` M` 37 ｜ untracked（porcelain）191 |
| 本阶段性质 | **归属裁定准备（非开发 / 非 commit / 非 cleanup）** |
| 本阶段禁止 | `git add/commit/push/reset/restore/checkout/clean/stash/rebase/merge/amend/squash`；删除 / 移动 / 修改 `.gitignore` / 生产源码 / 测试 / flaky test / 历史文档 |
| 本阶段允许 | 只读 / 文件分类 / Git object inspection / diff inspection / dependency analysis / report generation / Owner Decision Capture |

---

## §1 — 执行摘要（EXECUTIVE SUMMARY）

```
PHASE F-6.1 COMPLETE
OWNER DECISIONS REQUIRED
HARD STOP
```

本阶段把工作树从

```
36 tracked modifications + 191 untracked + multiple workstreams + unknown historical edits
```

拆解为三个**可命名、可归属、可裁定**的集合：

| 集合 | 含义 | 代码文件数 |
|---|---|---|
| **SET A** | F-1～F-5 Protocol Closure | **32**（29 tracked + 3 untracked） |
| **SET B** | Other Authorized Work（Console / Explorer / r1_boundary 等更早阶段） | **6**（5 tracked + 1 untracked） |
| **SET C** | **UNKNOWN / UNRESOLVED** | **10**（2 tracked + 8 untracked） |

**关键结论**：

1. **36 个 tracked 修改单独存在时，连生产代码都无法编译**（实测 `go vet` exit 1，
   缺失 5 个符号，全部定义在 **untracked** 文件中）。
2. **编译必需的未跟踪集合恰好是 3 个文件**；其中 **1 个（`f1n1_reorg_coverage_test.go`）
   归属 F1-N1，不属于 F-1～F-5** ⇒ 必须由 Owner 裁定（OD-03）。
3. **2 个 tracked 文档修改无任何本链授权记录** ⇒ UNKNOWN（OD-01 / OD-02）。
4. **3 个 token 文件未被 `.gitignore` 覆盖** ⇒ 凭据风险（OD-08）。

**本阶段不替 Owner 做任何决定。全部 8 项 OD 的 `OWNER DECISION = PENDING`。**

---

## §2 — 三个集合

### 2.1 SET A — F-1～F-5 Protocol Closure（32）

**准入条件**（必须全部满足）：为 Genesis Identity / explicit init / fail-closed /
real-node validation 服务，且有 F-3A/F-3B/F-4/F-5 的授权与报告痕迹。

| 组 | 文件 | 数 |
|---|---|---|
| A-生产 | `cmd/node/main.go`、`cmd/node/cli.go`、`internal/blockchain/blockchain.go`、`internal/blockchain/verify.go`、`internal/blockchain/genesis.go`、`internal/storage/file.go`、`internal/storage/v2.go` | 7 |
| B-测试夹具 | `cmd/node/*_test.go`（9）、`internal/blockchain/*_test.go`（7）、`internal/storage/*_test.go`（3）、`internal/mempool/reorg_resurrection_test.go`（1）… 详见矩阵 | 22 |
| F-新增（untracked） | `internal/blockchain/identity.go`、`cmd/node/f3b_test_helpers_test.go`、`cmd/node/f3b_identity_test.go` | 3 |

**授权痕迹**：`PHASE-F3A-GENESIS-IDENTITY-CONTRACT-FREEZE.md`（OD-01..10）、
`PHASE-F3B-IDENTITY-IMPLEMENTATION-EXECUTION.md`、`PHASE-F4-…-REPORT.md`、`PHASE-F5-…-REPORT.md`。

### 2.2 SET B — Other Authorized Work（6 代码 + 大宗文档归档）

| 文件 | 阶段 | 授权痕迹 |
|---|---|---|
| `internal/control/server.go` | CONSOLE-MINE-AUTH-FIX-1 | 源码自述；**无报告文件** ⚠️ |
| `internal/control/console_test.go` | CONSOLE-MINE-AUTH-FIX-1 | 同上 ⚠️ |
| `internal/control/web/console.html` | CONSOLE-MINE-AUTH-FIX-1 | 同上 ⚠️ |
| `internal/control/console_mine_auth_test.go`（untracked） | CONSOLE-MINE-AUTH-FIX-1 | 同上 ⚠️ |
| `internal/explorer/explorer_test.go` | **AUDIT-FIX-RUN-MINING-1** | `E:/wakuang/PHASE-AUDIT-FIX-RUN-MINING-1-REPORT.md`（D-2 节） |
| `cmd/node/r1_boundary_test.go` | **AUDIT-FIX-RUN-MINING-1** | 同上报告（明确列出本文件 +12/-4） |

**另有**：`docs/` 下 ~160 份历史阶段文档 / prompt 规格、`PHASE-REORG-1F..1J`（14 份）、
各验证产物目录（`F-1-lab/`、`audit-run/`、`concept-mvp/`、`gui/`、`verifier/`、`.workbuddy/`）
—— 属更早阶段，**均非 F-1～F-5**。

### 2.3 SET C — UNKNOWN / UNRESOLVED（10）

| 文件 | 为何无法可靠归属 |
|---|---|
| `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` | mtime 09-13，**早于本会话**；追加「PHASE V2.0 补录」；**无本链授权记录** |
| `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` | mtime 09-12；工作树内容被 **PHASE BRAND-0D.3** 报告**整体替换**（含外来 HEAD `535edb71`） |
| `internal/blockchain/f1n1_reorg_coverage_test.go` | 自述 **F1-N1**；**但编译必需**（被 tracked 文件引用）⇒ 归属与依赖冲突 |
| `internal/storage/f1n1_undo_contract_test.go` | 自述 F1-N1；**未找到授权/报告** |
| `internal/utxo/f1n1_undo_content_test.go` | 自述 F1-N1；**未找到授权/报告** |
| `internal/blockchain/f1n2_reorg_detached_undo_test.go` | 自述 F1-N2；**未找到授权/报告** |
| `internal/storage/f1n2_detached_undo_contract_test.go` | 自述 F1-N2；**未找到授权/报告** |
| `internal/attribution/attribution.go` | 自述 **GAP-4**；相关报告**本身也未跟踪** |
| `internal/attribution/classify.go` | 同上 |
| `internal/attribution/attribution_test.go` + `cmd/coinbase-attribution/main.go` | 同上（合并计为 GAP-4 组） |

---

## §3 — OWNER DECISION TABLE（核心交付）

| ID | Object | Evidence | Candidate Phase | Decision Required |
|---|---|---|---|---|
| **OD-01** | `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` | historical diff（HEAD 内容 = commit `535edb7` 的 P3.1 报告；工作树内容 = BRAND-0D.3 报告；外来 HEAD `535edb71`；mtime 09-12） | **?** | **Owner** |
| **OD-02** | `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` | historical diff（+88/−4，追加 §9.11「PHASE V2.0 补录」；mtime 09-13） | **?** | **Owner** |
| **OD-03** | `internal/blockchain/f1n1_reorg_coverage_test.go` | **compile dependency**（`f1n1UTXOSig` 被 tracked 的 `p1_reorg_fail_before_commit_test.go:63` 引用；pristine HEAD `go vet` → `undefined: f1n1UTXOSig`） | **?** | **Owner** |
| **OD-04** | `internal/attribution/`（3 文件） | **separate workstream**（自述 GAP-4；无 tracked 引用） | **?** | **Owner** |
| **OD-05** | `cmd/coinbase-attribution/main.go` | **separate workstream**（唯一 importer of `internal/attribution`；无 tracked 反向引用） | **?** | **Owner** |
| **OD-06** | remaining untracked tests（F1-N1 ×2 + F1-N2 ×2） | dependency / 自述 phase；**无授权痕迹** | **?** | **Owner** |
| **OD-07** | C-class files（5 tracked + 1 untracked） | earlier phases（CONSOLE-MINE-AUTH-FIX-1 ×4；AUDIT-FIX-RUN-MINING-1 ×2） | **?** | **Owner** |
| **OD-08** | token files（3） | **secret risk**（tracked=NO, ignored=NO） | **Governance** | **Owner** |

---

## §4 — 逐项 OD 详情

> 每项提供 `FACTS / EVIDENCE / IMPACT / OPTIONS / RECOMMENDED DEFAULT`。
> **`OWNER DECISION = PENDING`**（本阶段不代选）。

---

### OD-01 — `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md`

**FACTS**
- 状态：**TRACKED**，且出现在 36 个修改中（+470/−220）。
- mtime：`2026-09-12 21:52:10`（**早于本会话 15 天**）。
- HEAD 版本首行：`# PHASE P3.1 — DATA LOCK / LOCK LIFECYCLE VALIDATION, CLOSURE & ISOLATED COMMIT`（14,923 B）。
- 工作树首行：`# PHASE BRAND-0D.3 — FULL READ-ONLY VALIDATION`（9,373 B）。
- 工作树内容包含 `HEAD: 535edb71e7aecbd6e100534bb65b595e665dd26a`。

**EVIDENCE**
- `git log -1 -- docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` → `535edb7 feat(storage): harden data lock lifecycle`（2026-09-12）。
- `535edb7` **是 HEAD 的祖先**（`git merge-base --is-ancestor` 通过）。
- 与未跟踪的 `docs/PHASE-BRAND-0D.3-FRESH-VERIFICATION.md` **内容不同**（`diff -q` → DIFFERENT）。

**IMPACT**
- 若提交：一个名为「PHASE P3.1」的文件将携带 **BRAND-0D.3** 的内容，并把**外来基线 HEAD** 写入本链历史 ⇒ 历史污染。
- 该文件同时是 `git diff --check` **exit 2**（trailing whitespace）的**唯一**来源。

**OPTIONS**
| 选项 | 说明 |
|---|---|
| **O1 排除** | 不纳入任何提交，保留工作树现状（**不改不删**） |
| **O2 还原** | `git checkout --` 还原到 HEAD 内容（**属 Git 写操作，须单独授权**） |
| **O3 纳入** | 承认 BRAND-0D.3 内容，随本链提交（**污染历史**） |
| **O4 另案** | 移交 BRAND-0D.3 阶段处理 |

**RECOMMENDED DEFAULT**：**O1 排除**（本阶段禁止改/删/还原；O2 需单独授权）。

**OWNER DECISION = PENDING**

---

### OD-02 — `docs/DETERMINISTIC-SERIALIZATION-SPEC.md`

**FACTS**
- 状态：**TRACKED**，+88/−4。mtime `2026-09-13 09:30:20`（**早于本会话**）。
- HEAD 11,004 B → 工作树 13,921 B（**追加** §9.11「P-256 signature encoding」，自述「PHASE V2.0 补录」）。

**EVIDENCE**
- `git log -1 -- <path>` → `91dfee6 docs: document developer node and deterministic verification`。
- 该 §9.11 内容是**实现事实的补录**（曲线 P-256 / 非压缩 SEC1 65B / 签名 r‖s 64B），**与 F-1～F-5 无关**。
- 旁证：`internal/attribution/attribution.go` 自述「格式权威：`docs/DETERMINISTIC-SERIALIZATION-SPEC.md` §9.1–9.9」⇒ 该文档是 **GAP-4 工作流的依赖**。

**IMPACT**
- 若提交：把 **V2.0 阶段**的文档补录并入本链历史。
- 若排除：GAP-4 工作流（若未来提交）将**缺少其声明的格式权威**。

**OPTIONS**
| 选项 | 说明 |
|---|---|
| **O1 排除** | 不纳入本链提交 |
| **O2 纳入** | 视为 V2.0 阶段的合法补录，一并提交 |
| **O3 与 GAP-4 绑定** | 若 OD-04 决定纳入 attribution，则本文档**必须**同时纳入（否则 attribution 的格式权威缺失） |

**RECOMMENDED DEFAULT**：**O1 排除**，并**登记 O3 的绑定关系**供 Owner 参考。

**OWNER DECISION = PENDING**

---

### OD-03 — `internal/blockchain/f1n1_reorg_coverage_test.go` ★ 最关键

**FACTS**
- 状态：**UNTRACKED**，25,981 B，mtime `2026-09-27 10:52:35`（与 F-3B 批量改名**同时间戳**）。
- 自述 phase：`PHASE P2PCHAIN — F1-N1 UNDO CONTRACT TEST DESIGN / LEGACY-V2 REORG COVERAGE AUDIT`。
- 定义 `func f1n1UTXOSig(bc *Blockchain) string`（第 134 行）。

**EVIDENCE**
- **consumer（tracked，未修改，在 HEAD 中）**：`internal/blockchain/p1_reorg_fail_before_commit_test.go` 第 **63** 行 `utxoSig: f1n1UTXOSig(bc),`。
- **pristine HEAD 实测**：`go vet ./internal/blockchain/` → `undefined: f1n1UTXOSig`，**exit 1**。
- **provider 自身依赖**：该文件使用 `NewBlockchainFromStoreForTest`（定义于 **untracked** `identity.go`）。
- **实测收敛**：36 tracked 修改 + `identity.go` + `f3b_test_helpers_test.go` + **本文件** ⇒ `go vet ./...` **exit 0**。
- 该文件**同时**承载 flaky 用例 `TestF1N1_C_MixedLegacyV2Reorg`（见 §6）。

**IMPACT**
- **不纳入** ⇒ 提交树**无法编译**（`internal/blockchain` 测试构建失败）。
- **纳入** ⇒ 引入一个 **flaky** 用例，且把一个 **F1-N1**（非 F-1～F-5）文件并入本链历史。

**OPTIONS**
| 选项 | 说明 | 后果 |
|---|---|---|
| **O1 纳入** | 承认其**编译必需**，随本链提交 | 编译通过；但混入 F1-N1 文件 + flaky 用例 |
| **O2 排除** | 严格保持 F-1～F-5 边界 | 提交树**不可编译** ⇒ 必须先处置 `p1_reorg_fail_before_commit_test.go` 的引用 |
| **O3 拆分** | 仅提取 `f1n1UTXOSig` 辅助函数到 F-1～F-5 名下的新文件，其余留在 F1-N1 | 需**改代码**（本阶段禁止；须单独授权） |
| **O4 前移** | 把 F1-N1 作为**前置阶段**先独立提交，再提交 F-1～F-5 | 顺序化，最干净；需 F1-N1 授权补齐 |

**RECOMMENDED DEFAULT**：**O4 前移**（先解决 F1-N1 的授权与提交，再谈 F-1～F-5 提交）。
**理由**：O1 会把未授权工作流混入本链；O2 使树不可编译；O3 需改码（本阶段禁止）。

**OWNER DECISION = PENDING**

---

### OD-04 — `internal/attribution/`（attribution.go / classify.go / attribution_test.go）

**FACTS**
- 状态：**UNTRACKED**，mtime `2026-09-22 14:45:37`（**早于本会话**）。
- 自述：「**独立只读分析工具，不属于生产共识路径**；只读输入字节，fail-stop 报错」。
- 自述格式权威：`docs/DETERMINISTIC-SERIALIZATION-SPEC.md` §9.1–9.9（见 OD-02）。

**EVIDENCE**
- `git grep "internal/attribution"`（仅索引/HEAD）→ **无命中** ⇒ **无任何 tracked 文件引用**。
- 唯一 importer 是 **untracked** 的 `cmd/coinbase-attribution/main.go`（见 OD-05）。
- 相关报告：`docs/PHASE-P2PCHAIN-GAP-4-COINBASE-ATTRIBUTION-IMPLEMENTATION-REPLAY-AUDIT-REPORT.md`（**本身也未跟踪**）。
- `go test ./internal/attribution/` 在**工作树**中 `ok 0.371s`（被 `go test ./...` 执行）。

**IMPACT**
- 属 **SEPARATE WORKSTREAM（GAP-4 币基归因）**，与 Genesis Identity **无关**。
- 若排除：`go test ./...` 将少一个包（不影响编译）；工作树与提交树不等价。
- 若纳入：把 GAP-4 工作流并入本链历史，且**必须**同时纳入 OD-02 的格式权威文档。

**OPTIONS**：**O1 排除**（保持 F-1～F-5 纯净） / **O2 纳入**（连带 OD-02） / **O3 另立 GAP-4 提交**。

**RECOMMENDED DEFAULT**：**O3 另立 GAP-4 提交**（与 OD-02 绑定）。

**OWNER DECISION = PENDING**

---

### OD-05 — `cmd/coinbase-attribution/main.go`

**FACTS**
- 状态：**UNTRACKED**，6,098 B，mtime `2026-09-22 14:45:36`。
- 是 `internal/attribution` 的**唯一** importer（`main.go:26`）。
- 属 GAP-4 工具链的可执行入口。

**EVIDENCE**
- `git ls-files` 中**不存在**该包 ⇒ 若单独提交 `internal/attribution` 而不提交本文件，`go build ./...` 的**包集合**将变化（本包消失）。
- 无 tracked 反向引用。

**IMPACT**：与 OD-04 强绑定（包 + 入口必须同进同出）。

**OPTIONS**：**O1 排除** / **O2 随 OD-04 一起纳入** / **O3 另立 GAP-4 提交**。

**RECOMMENDED DEFAULT**：**与 OD-04 保持一致**（同进同出）。

**OWNER DECISION = PENDING**

---

### OD-06 — remaining untracked tests（F1-N1 ×2 + F1-N2 ×2）

**FACTS**

| 文件 | 自述 phase | mtime |
|---|---|---|
| `internal/storage/f1n1_undo_contract_test.go` | F1-N1 | 09-22 14:45 |
| `internal/utxo/f1n1_undo_content_test.go` | F1-N1 | 09-20 07:12 |
| `internal/blockchain/f1n2_reorg_detached_undo_test.go` | F1-N2 | 09-20 07:55 |
| `internal/storage/f1n2_detached_undo_contract_test.go` | F1-N2 | 09-20 07:52 |

**EVIDENCE**
- 四者**均无 tracked 引用**（逐个符号提取 + 跨文件 grep 验证）。
- 相关 prompt：`docs/prompts/F1-GAP-CONTRACT.md`、`docs/prompts/F1-N4-GAP-CONTRACT-…prompt.md`（**均未跟踪**）。
- 未在仓库内找到 F1-N1 / F1-N2 的**授权报告**。
- `go test ./...` 在**工作树**中会执行它们（属 F1-N1/F1-N2 包）。

**IMPACT**
- 与 OD-03 同族（F1-N1/F1-N2 reorg/undo 工作流）。
- 无编译依赖 ⇒ 可独立裁定。

**OPTIONS**：**O1 排除** / **O2 随 OD-03 一并纳入** / **O3 与 OD-03 一并另立 F1-N1/F1-N2 提交**。

**RECOMMENDED DEFAULT**：**O3**（与 OD-03 同批处置，保持 F1-N1/F1-N2 工作流完整）。

**OWNER DECISION = PENDING**

---

### OD-07 — C-class files（5 tracked + 1 untracked）

**FACTS**

| 文件 | 阶段 | 授权痕迹 |
|---|---|---|
| `internal/control/server.go` | CONSOLE-MINE-AUTH-FIX-1 | 源码注释自述；**无报告文件** |
| `internal/control/console_test.go` | CONSOLE-MINE-AUTH-FIX-1 | 同上 |
| `internal/control/web/console.html` | CONSOLE-MINE-AUTH-FIX-1 | 同上 |
| `internal/control/console_mine_auth_test.go`（untracked） | CONSOLE-MINE-AUTH-FIX-1 | 文件头自述 phase 名 |
| `internal/explorer/explorer_test.go` | **AUDIT-FIX-RUN-MINING-1** | **有报告**：`E:/wakuang/PHASE-AUDIT-FIX-RUN-MINING-1-REPORT.md`（D-2 节） |
| `cmd/node/r1_boundary_test.go` | **AUDIT-FIX-RUN-MINING-1** | **同上报告**（明确列出 +12/-4） |

**EVIDENCE**
- Console 组：`grep -rl 'CONSOLE-MINE-AUTH-FIX-1' p2pchain/` → 仅源码/测试文件命中，**无 .md 报告**。
  ⇒ **GOVERNANCE GAP：已授权执行但未产出报告**。
- AUDIT-FIX 组：报告位于 **仓库之外**（`E:/wakuang/` 根），报告正文明确登记两个文件。
- Console 组是**安全边界相邻**改动（引入同源闸门 `/console/mine`）。

**IMPACT**
- 两组均**不属于 F-1～F-5** ⇒ 不应混入 F-1～F-5 提交。
- Console 组缺报告 ⇒ 若纳入提交，缺少配套治理凭据。

**OPTIONS**
| 选项 | 说明 |
|---|---|
| **O1 全部排除** | 保持 F-1～F-5 纯净 |
| **O2 Console 组排除 + AUDIT-FIX 组纳入** | 有报告者纳入 |
| **O3 两组各自另立提交** | Console 需先补报告 |

**RECOMMENDED DEFAULT**：**O3**（各自另立提交；Console 组先补报告，因其安全边界相邻）。

**OWNER DECISION = PENDING**

---

### OD-08 — token files（3）

**FACTS**：`audit-run/control-token`（33 B）、`f5-verify/control-token`（64 B）、`gui-test/token`（34 B）。
三者 **tracked=NO, ignored=NO**。

**EVIDENCE**：`git check-ignore` 全部 exit 1；`.gitignore` **无任何 token/secret 模式**。
详见 `PHASE-F6.1-SECRET-ARTIFACT-INVENTORY.md`。

**IMPACT**：`git add -A` 会**暂存凭据**；一旦 commit+push，凭据**永久进入历史**。

**OPTIONS**：**O1 保持现状 + 提交走白名单** / **O2 修改 `.gitignore`**（须单独授权；会改变基线数字 36→37） / **O3 删除 token 文件**（本阶段禁止）。

**RECOMMENDED DEFAULT**：**O1 + 未来 Governance 阶段做 O2**。

**OWNER DECISION = PENDING**（Candidate Phase: **Governance**）

---

## §5 — Critical Rule 合规声明

F-6.1 §13 规定：**绝对禁止**以「测试需要，所以应该提交」作为唯一归属依据。

本阶段的合规做法：

| 文件 | 是否因「测试需要」而归类？ | 实际归属依据 |
|---|---|---|
| `f1n1_reorg_coverage_test.go` | **否** | 依赖（E3）证明其**编译必需**，但**归属**仍依自述（E2 = F1-N1）+ 授权缺口（E4 = 缺失）⇒ **SET C** |
| `identity.go` | **否** | 自述 F-3B（E2）+ F-3B 报告明列（E4）+ 被生产代码引用（E3）⇒ SET A |
| `f3b_test_helpers_test.go` | **否** | 自述 F-3B（E2）+ F-3B 报告（E4）⇒ SET A |
| `console_mine_auth_test.go` | **否** | 自述 phase（E2）+ 仅**反向**依赖 ⇒ SET B（非 A） |

**⇒ 依赖（E3）仅用于证明「编译影响」，从未用于证明「归属」。**
归属一律要求 `依赖 + phase ownership + 授权 + scope` 四者齐备；缺任一 ⇒ SET C。

---

## §6 — Flaky Test（记录，不修）

| 项 | 值 |
|---|---|
| 用例 | `TestF1N1_C_MixedLegacyV2Reorg` |
| 位置 | `internal/blockchain/f1n1_reorg_coverage_test.go:560` |
| 状态 | **FLAKY / LOAD-SENSITIVE / OPEN** |
| 实测 | 隔离 `-run` 3/3 **PASS**；整包 2/2 **PASS**；**全套 `./...` 并行 1/1 FAIL** |
| 失败信息 | `F1N1: no sibling below threshold after 256 attempts (height=3)` |
| 归因 | 256 次 PoW 尝试预算在整套并行负载（`cmd/node` 572s + `storage` 295s）下被打破 |
| 本阶段动作 | **DO NOT FIX**（仅记录） |
| 建议 | 单独设立 **F-6.2 Test Stabilization** |

---

## §7 — Zero-Drift Proof

| 项 | 开工 | 收工 | 漂移 |
|---|---|---|---|
| HEAD | `4d892be…` | `4d892be…` | **0** |
| staged | 0 | 0 | **0** |
| tracked 内容修改 | 36 | 36 | **0** |
| porcelain ` M` | 37 | 37 | **0** |
| untracked（porcelain `??`） | 191 | **194** | **+3**（= 本阶段 3 份交付物） |
| 受保护对象（node.exe / run-a / run-b） | — | — | **0** |
| `.gitignore` | — | — | **未修改** |

**未执行**：`git add` / `commit` / `push` / `reset` / `restore` / `checkout` / `clean` / `stash` /
`rebase` / `merge` / `amend` / `squash`；**未**删除 / **未**移动任何文件。
**唯一副作用**：新增 3 份交付物；`git archive` 抽取仅写入系统临时目录（**已删除**）。

---

## §8 — 交付物

| # | 文件 |
|---|---|
| 1 | `PHASE-F6.1-OWNER-SCOPE-DECISION.md`（本文件） |
| 2 | `PHASE-F6.1-FILE-PROVENANCE-MATRIX.md` |
| 3 | `PHASE-F6.1-SECRET-ARTIFACT-INVENTORY.md` |

---

## §9 — 最终判定（FINAL VERDICT）

```
F-6.1 COMPLETE
OWNER DECISIONS REQUIRED
HARD STOP
```

- **8 项 Owner Decision 全部 `PENDING`**，本阶段**未代选任何一项**。
- **未 commit**（本阶段禁止；且 F-6 已判定 NOT READY）。
- **下一阶段不得自动推断**：F-6.2（Test Stabilization）与 F-6.3（Commit Readiness）
  是否执行，须由 Owner 基于本阶段决策结果单独裁定。

**HARD STOP。**
