# PHASE F-6.1-OD — OWNER DECISION CAPTURE & SCOPE FREEZE

**PHASE F-6.1-OD — OWNER DECISION CAPTURE（STRICT READ-ONLY / NO CODE CHANGE / NO GIT MUTATION）**

| 项 | 值 |
|---|---|
| 仓库 | `E:/wakuang/p2pchain` |
| 基线 HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4`（branch `main`） |
| staged | **0** |
| tracked 内容修改（`git diff --name-only`） | **36** |
| porcelain ` M` | 37（含 1 个 CRLF-only 伪差异） |
| untracked（porcelain `??`） | **194** |
| `.gitignore` | **未修改**（sha `f43418cf74ea996cc603976f29badc4c507e6b3ace25f7f619d7d79aefb6567b`） |
| 本阶段性质 | **Owner 决策捕获 + 范围冻结（非开发 / 非 commit / 非 cleanup）** |
| 本阶段禁止 | `git add/commit/push/reset/restore/checkout/clean/stash/rebase/merge/amend/squash`；修改 / 删除 / 移动 生产源码 / 测试 / flaky test / 历史文档 / `.gitignore`；`git add -A` |
| 本阶段允许 | 只读 / 文件分类 / Git object inspection / diff inspection / dependency analysis / report generation / Owner Decision Capture |

---

## §1 — BASELINE（开工基线复验）

F-6.1-OD 开工前，对 F-6.1 收工基线逐项复验，**结果：无漂移（PASS）**。

| 项 | F-6.1 收工记录 | F-6.1-OD 复验 | 判定 |
|---|---|---|---|
| HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` | `4d892be355a38886a83c123faad915c9f3cd62e4` | ✅ 一致 |
| staged | 0 | 0 | ✅ 一致 |
| tracked 内容修改 | 36 | 36 | ✅ 一致 |
| porcelain ` M` | 37 | 37 | ✅ 一致 |
| untracked（`??`） | 194 | 194 | ✅ 一致 |
| `.gitignore` | 未修改 | 未修改（sha `f43418cf…`） | ✅ 一致 |

**F-6.1 三份交付物内容完整性（SHA-256 复算）**：

| 交付物 | 记录 SHA-256 | 复算 SHA-256 | 判定 |
|---|---|---|---|
| `PHASE-F6.1-OWNER-SCOPE-DECISION.md` | `3524b1659baa5d34f0d143e68f6f03e66000314715851a6af43a88c18f26442c` | 同 | ✅ 未漂移 |
| `PHASE-F6.1-FILE-PROVENANCE-MATRIX.md` | `86d20f62e2446811cde9c0452e15745e51c7d49aa8194aab101a4e2bbcf53f4d` | 同 | ✅ 未漂移 |
| `PHASE-F6.1-SECRET-ARTIFACT-INVENTORY.md` | `a4a36f7095a5fabc1a43a029c78e318036ce5911bb6082932f7672870fbfc492` | 同 | ✅ 未漂移 |

**F-6 三份交付物（旁证，非本阶段产物）**：

| 交付物 | SHA-256 |
|---|---|
| `PHASE-F6-GOVERNANCE-CLOSURE-COMMIT-READINESS-AUDIT.md` | `c439c47ffa0c354328e5f8473db597ee07bdc6d0dee09e12c04005ecc4530673` |
| `PHASE-F6-COMMIT-CANDIDATE-MANIFEST.md` | `edf117f51856e9515f6c69ff5b8b79e4672d4cfa4d4216c53ff840e1b3f942d1` |
| `FINDING-REGISTRY-F1-F5.md` | `6c1465e3a57bf402636ae0f532a98bb9c43c2b3f8499f21d7291ce9ad51abe1d` |

> **DRIFT GATE 结论：`PASS` —— 允许进入 OD 处理。未触发 STOP。**

---

## §2 — EVIDENCE INTEGRITY（证据完整性）

本阶段新增 / 复算的证据全部来自**只读命令**（`git rev-parse` / `git log` / `git show` /
`git merge-base --is-ancestor` / `git check-ignore` / `git ls-files` / `git status --porcelain` /
`sha256sum` / `stat` / `find` / `head` / `grep`）。**未读取任何 token 文件内容。**

### 2.1 未跟踪 `.go` 文件全集（13 = 9 源/测试 + 4 GAP-4）

| # | 文件 | 归属线索 | 编译必需 |
|---|---|---|---|
| 1 | `internal/blockchain/identity.go` | 自述 F-3B | ✅ |
| 2 | `cmd/node/f3b_test_helpers_test.go` | 自述 F-3B | ✅ |
| 3 | `cmd/node/f3b_identity_test.go` | 自述 F-3B | — |
| 4 | `internal/blockchain/f1n1_reorg_coverage_test.go` | 自述 **F1-N1** | ✅ |
| 5 | `internal/storage/f1n1_undo_contract_test.go` | 自述 **F1-N1** | — |
| 6 | `internal/utxo/f1n1_undo_content_test.go` | 自述 **F1-N1** | — |
| 7 | `internal/blockchain/f1n2_reorg_detached_undo_test.go` | 自述 **F1-N2** | — |
| 8 | `internal/storage/f1n2_detached_undo_contract_test.go` | 自述 **F1-N2** | — |
| 9 | `internal/control/console_mine_auth_test.go` | 自述 **CONSOLE-MINE-AUTH-FIX-1** | — |
| 10 | `internal/attribution/attribution.go` | 自述 **GAP-4** | — |
| 11 | `internal/attribution/classify.go` | 自述 **GAP-4** | — |
| 12 | `internal/attribution/attribution_test.go` | 自述 **GAP-4** | — |
| 13 | `cmd/coinbase-attribution/main.go` | 自述 **GAP-4** | — |

> 编译必需集合 = **恰好 3 个**：`identity.go`、`f3b_test_helpers_test.go`、`f1n1_reorg_coverage_test.go`。

### 2.2 mtime 聚类分析（E1 证据）

`2026-09-22 14:45` 窗口（14:40–14:50）实际命中 **8 个文件**：

| 文件 | 类别 | git 状态 |
|---|---|---|
| `internal/attribution/attribution.go` | GAP-4 | untracked |
| `internal/attribution/classify.go` | GAP-4 | untracked |
| `internal/attribution/attribution_test.go` | GAP-4 | untracked |
| `cmd/coinbase-attribution/main.go` | GAP-4 | untracked |
| `internal/storage/f1n1_undo_contract_test.go` | **F1-N1** | untracked |
| `internal/obs/obs.go` | — | TRACKED-CLEAN（CRLF-only） |
| `internal/p2p/node_test.go` | — | TRACKED-CLEAN（CRLF-only） |
| `internal/pow/pow.go` | — | TRACKED-CLEAN（CRLF-only） |

**结论（关键）**：该 mtime 窗口**跨越两个互不相关的工作流**（GAP-4 与 F1-N1），
并混入 3 个**内容未变**的 tracked 文件（`git diff --quiet` exit 0，仅行尾差异）。
⇒ **E1（mtime 聚类）单独不足以裁定归属**；必须与 E2/E3/E4 联立。
（此项**强化**了 F-6.1 §5 的「四证据齐备」规则。）

### 2.3 三份 tracked 文件为 CRLF-only 伪差异（复验）

| 文件 | `git diff --quiet` | 判定 |
|---|---|---|
| `internal/obs/obs.go` | exit 0 | 内容等价（CRLF-only） |
| `internal/p2p/node_test.go` | exit 0 | 内容等价（CRLF-only） |
| `internal/pow/pow.go` | exit 0 | 内容等价（CRLF-only） |

> 三者**不属于 36 个 tracked 内容修改**，仅在 mtime 上被本窗口命中。

---

## §3 — OD-01 — `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md`

### FACTS
- 状态：**TRACKED**，且出现在 36 个修改中（+470/−220）。
- mtime：`2026-09-12 21:52:10`（**早于本会话 15 天**）。
- HEAD 版本（`git show HEAD:`）首行：`# PHASE P3.1 — DATA LOCK / LOCK LIFECYCLE VALIDATION, CLOSURE & ISOLATED COMMIT`，**14,923 B**，SHA-256 `e9def9aa1f90e5acec7447c8c2236da4ef7eeb6411ee440e03ec7e0a5746fe61`。
- 工作树版本首行：`# PHASE BRAND-0D.3 — FULL READ-ONLY VALIDATION` / 次行 `# §3–§19 EXECUTION`，**9,373 B**，SHA-256 `98f036fbdd804fa8e525a6d0b08edd0e6eea4213f8fb088ce312f8dd9446cc21`。

### EVIDENCE（本阶段新增）
- `git log --oneline -- docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` → **仅 1 个 commit**：`535edb7 feat(storage): harden data lock lifecycle`。
- `535edb7` 元数据：`535edb71e7aecbd6e100534bb65b595e665dd26a`，`2026-09-12 21:14:42 +0800`。
- `git merge-base --is-ancestor 535edb7 HEAD` → **通过（YES-ANCESTOR）** ⇒ `535edb7` **是 HEAD 的祖先**（该 commit 已在历史中，非外来孤立对象）。
- **仓库内 P3.1 副本数 = 1**（`find -iname '*P3.1*'` 仅命中本文件）⇒ **无第二份 P3.1 归档**。
- **工作树内容 = 孤儿 BRAND-0D.3 报告**：与仓库内 5 份 `PHASE-BRAND-0D.3-*` 文档的 SHA **逐一比对均不匹配**：

| BRAND-0D.3 文档 | SHA-256 |
|---|---|
| `docs/PHASE-BRAND-0D.3-COMMIT-AUDIT.md` | `67ead5a04bf8c4a9bd28c480ed9388423f6161330d4de97082cf201bed3d3690` |
| `docs/PHASE-BRAND-0D.3-COMMIT-REPORT.md` | `75400e0dbbbc16b71827a36c7a43d169148ca71f26a25f4755172a07a82926ec` |
| `docs/PHASE-BRAND-0D.3-DEVELOPER-CONSOLE-BASELINE-REPORT.md` | `fe0802acac7cbad71d00c30f0216d7cf37e2578e81caeefd29c2575d65eae3b9` |
| `docs/PHASE-BRAND-0D.3-FRESH-VERIFICATION.md` | `830cb420b2a3e499cb321a7ec0306f3711cd3d6a2280e8ecd3cd795b287fc485` |
| `docs/PHASE-BRAND-0D.3-TECHNICAL-FOUNDATION.md` | `f4fc1dec272256c03e317575cf89b96b6eea6a15f49bad0b296e157cc31c4c45` |

  ⇒ 工作树内容（`98f036fb…`）**不对应任何已知 BRAND-0D.3 归档文件**，属**孤儿内容**。
- 该文件同时是 `git diff --check` **exit 2**（trailing whitespace）的**唯一**来源（F-6 已记录）。

### IMPACT
- 若提交：一个名为「PHASE P3.1」的文件将携带 **BRAND-0D.3** 的内容，并把**外来基线 HEAD `535edb71`** 写入本链历史 ⇒ **历史污染 + 语义错配**。
- 若还原（`git checkout --`）：恢复真实 P3.1 报告，但**属 Git 写操作**，本阶段禁止，须单独授权。
- 若排除：工作树保留该错配内容（**不改不删**），但提交树与工作树不等价。

### OPTIONS
| 选项 | 说明 | 后果 |
|---|---|---|
| **O1 排除** | 不纳入任何提交，保留工作树现状（**不改不删**） | 零副作用；工作树≠提交树 |
| **O2 还原** | 还原到 HEAD 内容（`git checkout --`） | 恢复真实 P3.1；**须单独 Git 写授权** |
| **O3 纳入** | 承认 BRAND-0D.3 内容，随本链提交 | **污染历史**（不推荐） |
| **O4 另案** | 移交 BRAND-0D.3 阶段处理 | 延后裁定 |

### RECOMMENDED DEFAULT
**O1 排除**（本阶段禁止改/删/还原；O2 需单独授权）。

**OWNER DECISION = PENDING ｜ CANDIDATE PHASE = ?**

---

## §4 — OD-02 — `docs/DETERMINISTIC-SERIALIZATION-SPEC.md`

### FACTS
- 状态：**TRACKED**，+88/−4。mtime `2026-09-13 09:30:20`（**早于本会话**）。
- HEAD 11,004 B → 工作树 13,921 B（**追加** §9.11「P-256 signature encoding」）。
- HEAD SHA-256 `33f67a69fe57f1c0098929aa4c2c56004e888fdbadd8df6e6c96d6f3689cd9bb`；工作树 SHA-256 `5caf38b23f8b842349360b5a96829d42aa388222a481783f8ef90bf2487936f1`。

### EVIDENCE
- `git log --oneline -- docs/DETERMINISTIC-SERIALIZATION-SPEC.md` → **仅 1 个 commit**：`91dfee6 docs: document developer node and deterministic verification`。
- §9.11 内容是**实现事实的补录**（曲线 P-256 / 非压缩 SEC1 65B / 签名 r‖s 64B），**与 F-1～F-5 无关**。
- 旁证：`internal/attribution/attribution.go` 自述「格式权威：`docs/DETERMINISTIC-SERIALIZATION-SPEC.md` §9.1–9.9」⇒ 该文档是 **GAP-4 工作流的依赖**。
- 与本链授权痕迹（F-3A/F-3B/F-4/F-5 报告）**无任何交集**。

### IMPACT
- 若提交：把 **V2.0 阶段**的文档补录并入本链历史。
- 若排除：GAP-4 工作流（若未来提交）将**缺少其声明的格式权威**。

### OPTIONS
| 选项 | 说明 |
|---|---|
| **O1 排除** | 不纳入本链提交 |
| **O2 纳入** | 视为 V2.0 阶段的合法补录，一并提交 |
| **O3 与 GAP-4 绑定** | 若 OD-04 决定纳入 attribution，则本文档**必须**同时纳入（否则 attribution 的格式权威缺失） |

### RECOMMENDED DEFAULT
**O1 排除**，并**登记 O3 的绑定关系**供 Owner 参考。

**OWNER DECISION = PENDING ｜ CANDIDATE PHASE = ?**

---

## §5 — OD-03 — `internal/blockchain/f1n1_reorg_coverage_test.go` ★ 最关键

### FACTS
- 状态：**UNTRACKED**，25,981 B，mtime `2026-09-27 10:52:35`（与 F-3B 批量改名**同时间戳**）。
- 自述 phase：`PHASE P2PCHAIN — F1-N1 UNDO CONTRACT TEST DESIGN / LEGACY-V2 REORG COVERAGE AUDIT`。
- 定义 `func f1n1UTXOSig(bc *Blockchain) string`（第 134 行）。

### EVIDENCE
- **consumer（tracked，未修改，在 HEAD 中）**：`internal/blockchain/p1_reorg_fail_before_commit_test.go` 第 **63** 行 `utxoSig: f1n1UTXOSig(bc),`。
- **pristine HEAD 实测**：`go vet ./internal/blockchain/` → `undefined: f1n1UTXOSig`，**exit 1**。
- **provider 自身依赖**：该文件使用 `NewBlockchainFromStoreForTest`（定义于 **untracked** `identity.go`）。
- **实测收敛**：36 tracked 修改 + `identity.go` + `f3b_test_helpers_test.go` + **本文件** ⇒ `go vet ./...` **exit 0**。
- 该文件**同时**承载 flaky 用例 `TestF1N1_C_MixedLegacyV2Reorg`（见 §12）。
- **授权痕迹**：F1-N1 报告存在于**仓库之外** `E:/wakuang/.workbuddy/F1-N1-UNDO-CONTRACT-TEST-DESIGN/…-FINAL-REPORT.md`，SHA-256 `42462d9b2c3e2e41eb5eca52348030aa560bb3b5fda8c52b1d42950dcdde0f3d`（**与 `docs/prompts/F1-GAP-CONTRACT.md` 记录逐字节一致**）。

### IMPACT
- **不纳入** ⇒ 提交树**无法编译**（`internal/blockchain` 测试构建失败）。
- **纳入** ⇒ 引入一个 **flaky** 用例，且把一个 **F1-N1**（非 F-1～F-5）文件并入本链历史。

### OPTIONS
| 选项 | 说明 | 后果 |
|---|---|---|
| **O1 纳入** | 承认其**编译必需**，随本链提交 | 编译通过；但混入 F1-N1 文件 + flaky 用例 |
| **O2 排除** | 严格保持 F-1～F-5 边界 | 提交树**不可编译** ⇒ 必须先处置 `p1_reorg_fail_before_commit_test.go` 的引用 |
| **O3 拆分** | 仅提取 `f1n1UTXOSig` 辅助函数到 F-1～F-5 名下的新文件，其余留在 F1-N1 | 需**改代码**（本阶段禁止；须单独授权） |
| **O4 前移** | 把 F1-N1 作为**前置阶段**先独立提交，再提交 F-1～F-5 | 顺序化，最干净；需 F1-N1 授权补齐 |

### RECOMMENDED DEFAULT
**O4 前移**（先解决 F1-N1 的授权与提交，再谈 F-1～F-5 提交）。
**理由**：O1 会把未授权工作流混入本链；O2 使树不可编译；O3 需改码（本阶段禁止）。

> **CRITICAL RULE 提示**：本文件的「编译必需」（E3）**仅证明编译影响**，
> **不构成**其归属 F-1～F-5 的依据。归属依 E2（自述 F1-N1）+ E4（授权报告在 F1-N1 名下）。

**OWNER DECISION = PENDING ｜ CANDIDATE PHASE = ?**

---

## §6 — OD-04 — `internal/attribution/`（attribution.go / classify.go / attribution_test.go）

### FACTS
- 状态：**UNTRACKED**；mtime `2026-09-22 14:45:37`（**早于本会话**）。
- 自述：「**独立只读分析工具，不属于生产共识路径**；只读输入字节，fail-stop 报错」。
- 自述格式权威：`docs/DETERMINISTIC-SERIALIZATION-SPEC.md` §9.1–9.9（见 OD-02）。
- 尺寸：`attribution.go` 12,709 B / `classify.go` 6,803 B / `attribution_test.go` 16,158 B。

### EVIDENCE
- `git grep "internal/attribution"`（仅索引/HEAD）→ **无命中** ⇒ **无任何 tracked 文件引用**。
- 唯一 importer 是 **untracked** 的 `cmd/coinbase-attribution/main.go`（见 OD-05）。
- 相关报告：`docs/PHASE-P2PCHAIN-GAP-4-COINBASE-ATTRIBUTION-IMPLEMENTATION-REPLAY-AUDIT-REPORT.md`（**本身也未跟踪**，mtime `2026-09-20 20:07`，SHA-256 `0bf64bf6cd755e471629580079b453da5dad3b00edf89de51e566bfd8bfd66ad`）。
- `go test ./internal/attribution/` 在**工作树**中 `ok`（被 `go test ./...` 执行）。

### ★ 本阶段新增发现：GAP-4 产物 SHA 与授权报告**不一致**（ALL 4 MISMATCH）

| 文件 | 报告登记 SHA-256（2026-09-20） | 当前工作树 SHA-256 | 判定 |
|---|---|---|---|
| `internal/attribution/attribution.go` | `783aeaf569e308a858a40506db1ab19f34941922653d7745998c51b642388715` | `3f4cb77820b43747de5354b1f095336e175c1afa4157758223d834d5dac3dadb` | ❌ 不符 |
| `internal/attribution/classify.go` | `f64f07bb55485eee68e12410a6a49580d66e12d6835e905d2cb15cb8feaf338e` | `bb7b00bc135c6146c9ca110f93080b839bd963d99cd78d3eb578ec7a07865060` | ❌ 不符 |
| `internal/attribution/attribution_test.go` | `4c4d91fdd9274cb6fa40287101c54c681ab3a357720a7b2b6e40bfc743c8f43b` | `4d7841d418a668818b5c151245045d1a37029e2e77e12d32b779a5b7b77d8b7c` | ❌ 不符 |
| `cmd/coinbase-attribution/main.go` | `c0fe645c1962762b6c5672fa1b39acb3c6b2cacd92e826cb4ca2a913f95c703d` | `c634c1ecaea5955118533d035af2d0db2fa8f18365fa579a22c0b7401bfa6f18` | ❌ 不符 |

- **CRLF→LF 归一化测试未能消解差异**（归一化后 SHA 与原始 SHA 相同 ⇒ 文件为**纯 LF**，非行尾问题）。
- 时间线：报告 `2026-09-20 20:07` → 产物 `2026-09-22 14:45`（**约 2 天后**）。
- ⇒ **授权报告所审计的产物状态，与当前工作树产物状态不一致**（artifact-vs-report drift）。

### IMPACT
- 属 **SEPARATE WORKSTREAM（GAP-4 币基归因）**，与 Genesis Identity **无关**。
- **治理风险**：授权报告的 SHA 证据**已失效**；若直接提交，将提交**未经报告审计的产物版本**。
- 若排除：`go test ./...` 将少一个包（不影响编译）；工作树与提交树不等价。
- 若纳入：把 GAP-4 工作流并入本链历史，且**必须**同时纳入 OD-02 的格式权威文档。

### OPTIONS
| 选项 | 说明 |
|---|---|
| **O1 排除** | 保持 F-1～F-5 纯净 |
| **O2 纳入** | 连带 OD-02；但**须先重建/复核授权报告 SHA** |
| **O3 另立 GAP-4 提交** | 与 OD-02 绑定；并**须先闭合 SHA drift** |

### RECOMMENDED DEFAULT
**O3 另立 GAP-4 提交**（与 OD-02 绑定），**且以「SHA drift 闭合」为前置条件**。

**OWNER DECISION = PENDING ｜ CANDIDATE PHASE = ?**

---

## §7 — OD-05 — `cmd/coinbase-attribution/main.go`

### FACTS
- 状态：**UNTRACKED**，6,098 B，mtime `2026-09-22 14:45:36`。
- 是 `internal/attribution` 的**唯一** importer（`main.go:26`）。
- 属 GAP-4 工具链的可执行入口。

### EVIDENCE
- `git ls-files` 中**不存在**该包 ⇒ 若单独提交 `internal/attribution` 而不提交本文件，`go build ./...` 的**包集合**将变化（本包消失）。
- 无 tracked 反向引用。
- 同样**命中 OD-04 的 SHA drift**（报告 `c0fe645c…703d` vs 当前 `c634c1ec…6f18`）。

### IMPACT
与 OD-04 强绑定（包 + 入口必须同进同出）。

### OPTIONS
**O1 排除** / **O2 随 OD-04 一起纳入** / **O3 另立 GAP-4 提交**。

### RECOMMENDED DEFAULT
**与 OD-04 保持一致**（同进同出）。

**OWNER DECISION = PENDING ｜ CANDIDATE PHASE = ?**

---

## §8 — OD-06 — remaining untracked tests（F1-N1 ×2 + F1-N2 ×2）

### FACTS

| 文件 | 自述 phase | 尺寸 | mtime |
|---|---|---|---|
| `internal/storage/f1n1_undo_contract_test.go` | F1-N1 | 18,515 B | `2026-09-22 14:45:38` |
| `internal/utxo/f1n1_undo_content_test.go` | F1-N1 | 12,660 B | `2026-09-20 07:12:49` |
| `internal/blockchain/f1n2_reorg_detached_undo_test.go` | F1-N2 | 20,956 B | `2026-09-20 07:55:43` |
| `internal/storage/f1n2_detached_undo_contract_test.go` | F1-N2 | 24,108 B | `2026-09-20 07:52:41` |

### EVIDENCE
- 四者**均无 tracked 引用**（逐个符号提取 + 跨文件 grep 验证）。
- **授权痕迹（本阶段定位到仓库之外）**：
  - F1-N1 报告 `E:/wakuang/.workbuddy/F1-N1-UNDO-CONTRACT-TEST-DESIGN/…-FINAL-REPORT.md`，SHA-256 `42462d9b2c3e2e41eb5eca52348030aa560bb3b5fda8c52b1d42950dcdde0f3d`。
  - F1-N2 报告 `E:/wakuang/.workbuddy/F1-N2-DETACHED-V2-UNDO-CONTRACT-AUDIT/…-FINAL-REPORT.md`，SHA-256 `ddc8e4d7fd7c02920aee930e09de250647e4a8e0d86f73d86dda14681bbe3011`。
  - 两者 SHA 均**与 `docs/prompts/F1-GAP-CONTRACT.md` 的记录逐字节一致**（该记录列出 F1-N1..N4 报告 SHA）。
- **mtime 提示**：`storage/f1n1_undo_contract_test.go` 与 GAP-4 四文件同处 `14:45` 窗口 ⇒
  再次印证 **E1 聚类跨工作流**，不足以单独裁定归属。
- `go test ./...` 在**工作树**中会执行它们（属 F1-N1/F1-N2 包）。

### IMPACT
- 与 OD-03 同族（F1-N1/F1-N2 reorg/undo 工作流）。
- 无编译依赖 ⇒ 可独立裁定。

### OPTIONS
**O1 排除** / **O2 随 OD-03 一并纳入** / **O3 与 OD-03 一并另立 F1-N1/F1-N2 提交**。

### RECOMMENDED DEFAULT
**O3**（与 OD-03 同批处置，保持 F1-N1/F1-N2 工作流完整）。

> **注**：F1-N1/F1-N2 已具备**形式授权（报告）**，但报告位于**仓库之外**（`E:/wakuang/.workbuddy/`）。
> 是否将报告纳入仓库、以及是否视为「本链授权」，**须由 Owner 裁定**。

**OWNER DECISION = PENDING ｜ CANDIDATE PHASE = ?**

---

## §9 — OD-07 — C-class files（5 tracked + 1 untracked）

### FACTS

| 文件 | 阶段 | 授权痕迹 |
|---|---|---|
| `internal/control/server.go` | CONSOLE-MINE-AUTH-FIX-1 | 源码注释自述；**无报告文件** |
| `internal/control/console_test.go` | CONSOLE-MINE-AUTH-FIX-1 | 同上 |
| `internal/control/web/console.html` | CONSOLE-MINE-AUTH-FIX-1 | 同上 |
| `internal/control/console_mine_auth_test.go`（untracked，7,476 B，mtime 09-27 09:20:30） | CONSOLE-MINE-AUTH-FIX-1 | 文件头自述 phase 名 |
| `internal/explorer/explorer_test.go` | **AUDIT-FIX-RUN-MINING-1** | **有报告（仓库之外）** |
| `cmd/node/r1_boundary_test.go` | **AUDIT-FIX-RUN-MINING-1** | **同上报告**（明确列出 +12/−4） |

### EVIDENCE
- Console 组：`grep -rl 'CONSOLE-MINE-AUTH-FIX-1' --include='*.md'` → **仅命中本链自身的 F-6/F-6.1 报告**（旁证），
  **无任何阶段报告 .md** ⇒ **GOVERNANCE GAP：已授权执行但未产出报告**。
  （字符串仅出现在 `server.go` / `console_test.go` / `console_mine_auth_test.go` / `console.html` 及若干 `.exe` 中。）
- AUDIT-FIX 组：报告位于 **仓库之外** `E:/wakuang/PHASE-AUDIT-FIX-RUN-MINING-1-REPORT.md`
  （15,317 B，mtime `2026-09-27 09:15`，SHA-256 `5e5e8142014e79932682d0ebd4f058b1befdf0464ca9edbdff76f5c0c5ddb507`），
  报告正文明确登记两个文件。**本阶段仅确认其存在与指纹，未移动/复制/删除该外部报告。**
- Console 组是**安全边界相邻**改动（引入同源闸门 `POST /console/mine`）。

### IMPACT
- 两组均**不属于 F-1～F-5** ⇒ 不应混入 F-1～F-5 提交。
- Console 组缺报告 ⇒ 若纳入提交，缺少配套治理凭据。

### OPTIONS
| 选项 | 说明 |
|---|---|
| **O1 全部排除** | 保持 F-1～F-5 纯净 |
| **O2 Console 组排除 + AUDIT-FIX 组纳入** | 有报告者纳入 |
| **O3 两组各自另立提交** | Console 需先补报告 |

### RECOMMENDED DEFAULT
**O3**（各自另立提交；Console 组先补报告，因其安全边界相邻）。

**OWNER DECISION = PENDING ｜ CANDIDATE PHASE = ?**

---

## §10 — OD-08 — token files（3）

### FACTS（**仅元数据，未读取内容**）

| 文件 | 尺寸 | mtime | tracked | ignored |
|---|---|---|---|---|
| `audit-run/control-token` | 33 B | `2026-09-27 08:51:13` | NO | **NO** |
| `f5-verify/control-token` | 64 B | `2026-09-27 12:59:02` | NO | **NO** |
| `gui-test/token` | 34 B | `2026-09-27 10:11:58` | NO | **NO** |

### EVIDENCE
- `git check-ignore -q` 对三者**全部 exit 1**（NOT-IGNORED）。
- `.gitignore`（sha `f43418cf…`）**无任何 token/secret 模式**。
- 详见 `PHASE-F6.1-SECRET-ARTIFACT-INVENTORY.md`（F-6.1 交付物 3）。

### IMPACT
- `git add -A` 会**暂存凭据**；一旦 commit+push，凭据**永久进入历史**。
- ⇒ **提交必须走显式路径白名单**，禁用 `git add -A` / `.` / `-a`。

### OPTIONS
| 选项 | 说明 |
|---|---|
| **O1 保持现状 + 提交走白名单** | 零副作用；依赖流程纪律 |
| **O2 修改 `.gitignore`** | 须单独授权；会改变基线数字（36→37） |
| **O3 删除 token 文件** | 本阶段禁止 |

### RECOMMENDED DEFAULT
**O1 + 未来 Governance 阶段做 O2**。

**OWNER DECISION = PENDING ｜ CANDIDATE PHASE = GOVERNANCE**

---

## §11 — FINDING-1（保留，未改动）

**FINDING-1（F-6.1 原样保留，本阶段未修改结论）**：

| 实验 | 结果 |
|---|---|
| pristine HEAD（`git archive HEAD`）单独 `go vet ./internal/blockchain/` | **exit 1**（`undefined: f1n1UTXOSig`） |
| 36 tracked 修改**单独** | **exit 1**（缺失 `ErrUninitializedStore` / `VerifyGenesisIdentity` / `NewBlockchainFromStoreForTest` / `f1n1UTXOSig` / `newNodeRuntimeForTest`） |
| 36 tracked + **恰好 3 个**强制未跟踪文件 | **exit 0** |

- 强制未跟踪集合 = `internal/blockchain/identity.go`、`cmd/node/f3b_test_helpers_test.go`、`internal/blockchain/f1n1_reorg_coverage_test.go`。
- **核心区分（长期有效）**：**「编译必需」≠「阶段归属」**。
  `f1n1_reorg_coverage_test.go` 编译必需，但其归属为 **F1-N1**（E2 自述 + E4 授权报告在 F1-N1 名下），
  **不得**因其被 F-1～F-5 编译依赖而自动并入 F-1～F-5。

**本阶段动作：保留原结论，未做任何修改。**

---

## §12 — FLAKY TEST（记录，不修）

| 项 | 值 |
|---|---|
| 用例 | `TestF1N1_C_MixedLegacyV2Reorg` |
| 位置 | `internal/blockchain/f1n1_reorg_coverage_test.go:560` |
| 状态 | **FLAKY / LOAD-SENSITIVE / OPEN** |
| 实测 | 隔离 `-run` 3/3 **PASS**；整包 2/2 **PASS**；**全套 `./...` 并行 1/1 FAIL** |
| 失败信息 | `F1N1: no sibling below threshold after 256 attempts (height=3)` |
| 归因 | 256 次 PoW 尝试预算在整套并行负载（`cmd/node` ~572s + `storage` ~295s）下被打破 |
| 本阶段动作 | **DO NOT FIX**（仅记录） |
| 建议 | 单独设立 **F-6.2 Test Stabilization** |

---

## §13 — CRITICAL RULE COMPLIANCE（归属规则合规声明）

F-6.1 §13 规定：**绝对禁止**以「测试需要，所以应该提交」作为唯一归属依据。
**四证据齐备**才可归属；缺任一 ⇒ SET C。

| 文件 | 是否因「测试需要」归类？ | 实际归属依据 | 结论 |
|---|---|---|---|
| `f1n1_reorg_coverage_test.go` | **否** | E3 仅证明**编译必需**；归属依 E2（F1-N1）+ E4（F1-N1 报告） | **SET C / F1-N1**（待 Owner） |
| `identity.go` | **否** | E2（F-3B）+ E4（F-3B 报告明列）+ E3（被生产代码引用） | SET A |
| `f3b_test_helpers_test.go` | **否** | E2（F-3B）+ E4（F-3B 报告） | SET A |
| `console_mine_auth_test.go` | **否** | E2（自述 phase）+ 仅**反向**依赖；E4 缺报告 | SET B（非 A） |
| `internal/attribution/*` | **否** | E2（GAP-4 自述）+ E4（GAP-4 报告，**但 SHA 已 drift**） | SET C（待 Owner） |

**⇒ 依赖（E3）仅用于证明「编译影响」，从未用于证明「归属」。**
**⇒ E1（mtime 聚类）单独亦不足**（§2.2 证明其跨工作流）。

---

## §14 — ZERO-DRIFT PROOF（开工 vs 收工）

| 项 | 开工 | 收工 | 漂移 |
|---|---|---|---|
| HEAD | `4d892be…` | `4d892be…` | **0** |
| staged | 0 | 0 | **0** |
| tracked 内容修改 | 36 | 36 | **0** |
| porcelain ` M` | 37 | 37 | **0** |
| untracked（`??`） | 194 | **195** | **+1**（= 本阶段 1 份交付物） |
| 受保护对象（`node.exe` / `run-a` / `run-b`） | — | — | **0** |
| `.gitignore` | — | — | **未修改** |

**未执行**：`git add` / `commit` / `push` / `reset` / `restore` / `checkout` / `clean` / `stash` /
`rebase` / `merge` / `amend` / `squash`；**未**删除 / **未**移动任何文件；
**未**读取任何 token 文件内容。
**唯一副作用**：新增 1 份交付物（本文件）。

---

## §15 — SCOPE FREEZE（范围冻结）

本阶段**冻结**以下范围，**任何后续阶段不得自动扩张**：

1. **冻结对象 = F-6.1 定义的 SET A / SET B / SET C**（36 tracked + 13 untracked .go + 3 token + 历史文档）。
2. **冻结期间禁止**：修改任何源码 / 测试 / flaky test / 历史文档 / `.gitignore`；
   任何 `git add/commit/push/...`；任何删除 / 移动 / 还原 / 覆盖。
3. **未决事项（全部挂起，等待 Owner）**：
   - OD-01 … OD-08 **全部 `PENDING`**。
   - **新增**：OD-04/OD-05 的 **GAP-4 artifact-vs-report SHA drift**（4 文件全不符）。
4. **不得**以「编译必需」为由自动把 `f1n1_reorg_coverage_test.go` 并入 F-1～F-5。
5. **不得**以「mtime 相同」为由自动跨工作流归属（§2.2 已证伪）。

---

## §16 — NEXT AUTHORIZED PHASE CANDIDATES（下一阶段候选，**不得自动推断**）

| 候选阶段 | 触发条件 | 说明 |
|---|---|---|
| **F-6.2 — Test Stabilization** | Owner 决定处理 flaky `TestF1N1_C_MixedLegacyV2Reorg` | 本阶段**不修** |
| **F-6.3 — Commit Readiness / Commit Execution** | Owner 逐项裁定 OD-01..OD-08 后 | 本阶段**不 commit** |
| **GOVERNANCE — `.gitignore` secret hardening** | Owner 批准 OD-08 的 O2 | 会改变基线数字（36→37） |
| **GAP-4 — SHA drift closure** | Owner 选择 OD-04/OD-05 的 O2/O3 | **新增前置条件**：先闭合报告与产物 SHA 不一致 |
| **F1-N1 / F1-N2 — 授权与归档** | Owner 选择 OD-03/OD-06 的 O3/O4 | 报告位于仓库之外，须决定是否纳入 |

> **以上均为「候选」，本阶段不做任何选择，也不得由后续会话自动推断。**

---

## §17 — OWNER DECISION TABLE（最终，全部 `PENDING`）

| ID | Object | Evidence（本阶段复验） | Candidate Phase | OWNER DECISION |
|---|---|---|---|---|
| **OD-01** | `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` | 仅 1 commit（`535edb7`，HEAD 祖先）；仓库内仅 1 份 P3.1 副本；工作树内容 = 孤儿 BRAND-0D.3（`98f036fb…` 不匹配任何 BRAND 归档）；`git diff --check` exit 2 唯一来源 | **?** | **PENDING ｜ OWNER** |
| **OD-02** | `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` | 仅 1 commit（`91dfee6`）；追加 §9.11（V2.0）；GAP-4 的格式权威 | **?** | **PENDING ｜ OWNER** |
| **OD-03** | `internal/blockchain/f1n1_reorg_coverage_test.go` | 编译必需（`f1n1UTXOSig`）；归属 F1-N1（报告 `42462d9b…`，与 F1-GAP-CONTRACT 一致） | **?** | **PENDING ｜ OWNER** |
| **OD-04** | `internal/attribution/`（3 文件） | 无 tracked 引用；GAP-4 报告存在但**4 文件 SHA 全不符**（artifact-vs-report drift） | **?** | **PENDING ｜ OWNER** |
| **OD-05** | `cmd/coinbase-attribution/main.go` | 唯一 importer；同样 **SHA drift** | **?** | **PENDING ｜ OWNER** |
| **OD-06** | untracked tests（F1-N1 ×2 + F1-N2 ×2） | 无 tracked 引用；F1-N1 报告 `42462d9b…` / F1-N2 报告 `ddc8e4d7…`（均在仓库之外，SHA 与记录一致） | **?** | **PENDING ｜ OWNER** |
| **OD-07** | C-class files（5 tracked + 1 untracked） | Console 组 **GOVERNANCE GAP（无报告）**；AUDIT-FIX 组报告在仓库之外（`5e5e8142…`） | **?** | **PENDING ｜ OWNER** |
| **OD-08** | token files（3） | tracked=NO, ignored=NO；`.gitignore` 无 secret 模式 | **GOVERNANCE** | **PENDING ｜ OWNER** |

---

## §18 — FINAL VERDICT（最终判定）

```
PHASE F-6.1-OD COMPLETE
OWNER DECISIONS CAPTURED
PENDING
HARD STOP
```

- **Baseline 复验 PASS**（无漂移；F-6.1 三份交付物 SHA 复算一致）。
- **8 项 Owner Decision 全部 `PENDING`**，本阶段**未代选任何一项**。
- **本阶段新增发现**：OD-04/OD-05 的 **GAP-4 artifact-vs-report SHA drift**（4 文件全不符）。
- **本阶段未修改**任何源码 / 测试 / flaky test / 历史文档 / `.gitignore`；**未 commit**。
- **禁止自动进入** F-6.2 / F-6.3 / stabilization / cleanup / commit / merge / archive /
  delete / restore / 任何源码修改。

**HARD STOP。**
