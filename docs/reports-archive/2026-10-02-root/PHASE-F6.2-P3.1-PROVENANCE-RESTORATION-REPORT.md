# PHASE F-6.2 — P3.1 PROVENANCE RESTORATION & SCOPE-CONTROLLED RECONCILIATION — REPORT

**PHASE F-6.2 ｜ CONTROLLED EXECUTION / NARROW-SCOPE RESTORATION ｜ SCOPE FROZEN ｜ NO COMMIT**

---

## 1. PHASE IDENTITY

| 项 | 值 |
|---|---|
| Project | **P2PChain** |
| Phase | **F-6.2** |
| Title | **P3.1 PROVENANCE RESTORATION & SCOPE-CONTROLLED RECONCILIATION** |
| Mode | CONTROLLED EXECUTION / NARROW-SCOPE RESTORATION |
| Baseline HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| Previous Phase | `PHASE F-6.1-OWNER-DECISION-FINALIZATION` |
| Previous Phase Status | COMPLETE / SCOPE FROZEN / HARD STOP |
| 授权范围 | **仅** reconcile 被污染/误归属的 P3.1 文档 provenance |
| 明确不扩展至 | F1-N1 ｜ F1-N2 ｜ GAP-4 ｜ Console 治理补救 ｜ AUDIT-FIX ｜ token 清理 ｜ flaky-test 修复 ｜ 通用工作树清理 |

---

## 2. OWNER AUTHORIZATION

Owner 在 F-6.1 对 **OD-01** 的决策：

```
ACCEPT REAL P3.1 PROVENANCE
```

已确认的 provenance 来源：

| 字段 | 值 |
|---|---|
| commit（全） | `535edb71e7aecbd6e100534bb65b595e665dd26a` |
| commit（短） | `535edb7` |
| 与 HEAD 关系 | **已验证为 HEAD 的祖先**（`git merge-base --is-ancestor` 通过） |

规范历史 P3.1 文档路径：

```
docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md
```

该路径的**当前工作树内容**此前已被证明为 **orphaned BRAND-0D.3 report**，**不得**视为权威 P3.1 内容。

**本阶段唯一目标**：从已验证的 provenance 来源**恢复规范 P3.1 内容**，同时保持其余全部冻结范围，并证明无任何无关对象被修改。

---

## 3. BASELINE VERIFICATION（开工基线）

| 项 | 要求 | 实测 | 判定 |
|---|---|---|---|
| HEAD | `4d892be` | `4d892be355a38886a83c123faad915c9f3cd62e4` | ✅ |
| staged | 0 | 0 | ✅ |
| tracked modified | 36 | 36 | ✅ |
| porcelain ` M` | 37 | 37 | ✅ |
| untracked | 196 | 196 | ✅ |
| `.gitignore` SHA | `f43418cf…` | `f43418cf74ea996cc603976f29badc4c507e6b3ace25f7f619d7d79aefb6567b` | ✅ |
| `node.exe` | 存在、未动 | `3972843aff90428d…` | ✅ |
| `run-a` / `run-b` | 存在、未动 | 存在 | ✅ |
| F-6.1 finalization report | 存在且匹配 | `PHASE-F6.1-OWNER-DECISION-FINALIZATION.md` = `5b95d47009dd49c88aacf890ed0df1f17f2dc99127e9bd583214923221c749cf` | ✅ **MATCH** |

**BASELINE VERDICT = PASS。未触发 STOP。**

---

## 4. HISTORICAL SOURCE IDENTITY

来源对象：`535edb71e7aecbd6e100534bb65b595e665dd26a:docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md`

| 字段 | 值 |
|---|---|
| Path identity | `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` |
| Commit | `535edb71e7aecbd6e100534bb65b595e665dd26a`（短 `535edb7`） |
| Commit timestamp | `2026-09-12 21:14:42 +0800` |
| Author | `p2pchain-baseline` |
| Subject | `feat(storage): harden data lock lifecycle` |
| Parent commit | `50449cd8a5d8eecf52ed9bbe9f369487c7b6e8bf` |
| **Git blob identity** | **`08b74f1bdce1638f6b51e4c0be42143b9c7cebaa`** |
| HEAD 版本是否相同 | **是**（HEAD 版本 SHA 与 535edb7 版本一致 ⇒ 该路径自 535edb7 后未再被任何 commit 修改） |

---

## 5. HISTORICAL SOURCE SHA-256

```
e9def9aa1f90e5acec7447c8c2236da4ef7eeb6411ee440e03ec7e0a5746fe61
```

（由 `git cat-file blob 08b74f1b… | sha256sum` 独立计算；与 `git show HEAD:<path> | sha256sum` 结果一致。）

---

## 6. HISTORICAL SOURCE SIZE

| 项 | 值 |
|---|---|
| Size | **14,923 bytes** |
| Line count | **310** |
| 首行 | `# PHASE P3.1 — DATA LOCK / LOCK LIFECYCLE VALIDATION, CLOSURE & ISOLATED COMMIT` |

---

## 7. PRE-RESTORATION WORKING-TREE SHA

| 项 | 值 |
|---|---|
| Path | `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` |
| Pre-restoration SHA-256 | `98f036fbdd804fa8e525a6d0b08edd0e6eea4213f8fb088ce312f8dd9446cc21` |
| Size | 9,373 bytes |
| Line count | 560 |
| 首行 | `# PHASE BRAND-0D.3 — FULL READ-ONLY VALIDATION` |

**⇒ 两者（历史源 vs 工作树）内容不同，符合预期。**

### 7.1 预恢复控制点（SCOPE SNAPSHOT）

在恢复前捕获（写入系统临时目录 `/tmp/f62-snapshot`，**仓库零改动**）：

| 快照项 | 规模 |
|---|---|
| `git status --porcelain` | 233 行 |
| `git diff --name-only` | 36 |
| `git ls-files` | 242 |
| `git diff --stat` | `36 files changed, 957 insertions(+), 407 deletions(-)` |
| 哈希清单（36 tracked + 3 mandatory untracked + 4 GAP-4 + `node.exe` + `.gitignore`） | 50 行 |
| **合并摘要（pre-restoration digest）** | **`e6ebbe1a6e8bc59cc6fffa1e213b01fd5e00c17c561b954564c921226b1d2609`** |

---

## 8. RESTORATION OPERATION

**方法**：以 Git 历史对象为权威来源，**单路径**写出；**未**手工重建、**未**改写措辞、**未**归一化行尾、**未**增删章节、**未**合并 BRAND-0D.3 内容。

```bash
git cat-file blob 08b74f1bdce1638f6b51e4c0be42143b9c7cebaa \
  > docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md
```

**未使用**任何广义恢复命令：

```
git restore .            ✗ 未使用
git checkout -- .        ✗ 未使用
git restore --source=…   ✗ 未使用
git reset --hard         ✗ 未使用
```

**仅** `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` 这一个路径被改写。**未**恢复任何目录。

**BRAND-0D.3 orphan 内容**：**未**复制、**未**重命名、**未**归档、**未**嵌入、**未**另建副本、**未**移动 —— 按 §7 要求，本阶段**不**建立新归档系统。

---

## 9. POST-RESTORATION SHA

| 项 | 值 |
|---|---|
| Post-restoration SHA-256 | `e9def9aa1f90e5acec7447c8c2236da4ef7eeb6411ee440e03ec7e0a5746fe61` |
| Size | **14,923 bytes** |
| Line count | **310** |
| 首行 | `# PHASE P3.1 — DATA LOCK / LOCK LIFECYCLE VALIDATION, CLOSURE & ISOLATED COMMIT` |

**⇒ 与历史源 SHA 完全一致。**

---

## 10. BYTE-FOR-BYTE EQUALITY RESULT

| 校验 | 命令 | 结果 |
|---|---|---|
| **内容逐字节比对** | `git cat-file blob 08b74f1b… \| cmp -s - <path>` | **IDENTICAL（exit 0）** ✅ |
| **SHA-256 相等** | `sha256sum` | `e9def9aa…` = `e9def9aa…` ✅ |
| **大小相等** | `stat -c%s` | 14923 = 14923 ✅ |
| **Blob 身份相等** | `git hash-object <path>` | `08b74f1bdce1638f6b51e4c0be42143b9c7cebaa` = 历史 blob ✅ |
| **相对 HEAD 无差异** | `git diff --quiet -- <path>` | exit 0（恢复后与 HEAD 内容一致）✅ |

```
historical source SHA  =  restored working-tree SHA   ✅
historical size        =  working-tree size           ✅
historical bytes       =  working-tree bytes          ✅
historical blob id     =  working-tree blob id        ✅
```

**PROVENANCE RESTORATION = PASS**

---

## 11. NON-TARGET DRIFT AUDIT

恢复后，将完整仓库状态与**预恢复快照**逐项比对：

| 对象 | 恢复前 | 恢复后 | 判定 |
|---|---|---|---|
| HEAD | `4d892be…` | `4d892be…` | ✅ 未变 |
| staged | 0 | 0 | ✅ 未变 |
| **tracked modified** | **36** | **35** | ✅ **仅 -1**（= P3.1 文档恢复为 HEAD 内容后不再计为 modified） |
| **porcelain ` M`** | **37** | **36** | ✅ **仅 -1**（同上） |
| untracked（`??`） | 196 | **196** | ✅ 未变 |
| `git ls-files` | 242 | 242 | ✅ **IDENTICAL** |
| `git diff --name-only` 集合差异 | — | 仅移除 `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` | ✅ 唯一差异 |
| 其余 35 个 tracked 文件 SHA | — | 全部相同 | ✅ 未变 |
| 3 个强制编译依赖 untracked SHA | — | 全部相同 | ✅ 未变 |
| 4 个 GAP-4 冻结文件 SHA | — | 全部相同 | ✅ 未变 |
| `node.exe` SHA | `3972843a…` | `3972843a…` | ✅ 未变 |
| `.gitignore` SHA | `f43418cf…` | `f43418cf…` | ✅ 未变 |
| P3.1 是否仍在 `git status` 中 | 是（modified） | **ABSENT** | ✅ 预期 |

**哈希清单 pre vs post 的 `diff` 输出**：**仅**一行差异 ——
```
< 98f036fbdd804fa8e525a6d0b08edd0e6eea4213f8fb088ce312f8dd9446cc21 *docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md
```
（即被恢复的旧工作树内容行消失；**无其他行变化**。）

**⇒ NON-TARGET DRIFT = NONE。唯一被修改的对象 = 授权的 P3.1 文档。**

---

## 12. GIT MUTATION AUDIT

本阶段**未执行**任何 Git 写操作：

```
git add        ✗ 未执行
git commit     ✗ 未执行
git push       ✗ 未执行
git reset      ✗ 未执行
git clean      ✗ 未执行
git stash      ✗ 未执行
git checkout   ✗ 未执行
git merge      ✗ 未执行
git rebase     ✗ 未执行
git amend      ✗ 未执行
git squash     ✗ 未执行
git tag        ✗ 未执行
```

**唯一写操作**：以 `git cat-file blob`（**只读** Git 对象读取）为源，将**一个**工作树文件写出。
**未**暂存、**未**提交、**未**推送。恢复后的文件**保持为工作树变更**（对 HEAD 而言现已无内容差异）。

---

## 13. PROTECTED-OBJECT AUDIT

| 受保护对象 | 恢复前 | 恢复后 | 判定 |
|---|---|---|---|
| `node.exe` | `3972843aff90428d…` | `3972843aff90428d…` | ✅ 未变 |
| `run-a/` | 存在 | 存在 | ✅ 未动 |
| `run-b/` | 存在 | 存在 | ✅ 未动 |

---

## 14. SECRET-ARTIFACT AUDIT

| 文件 | 状态 | 判定 |
|---|---|---|
| `audit-run/control-token` | 存在、tracked=NO、ignored=NO | ✅ 未读 / 未暂存 / 未改 / 未移 / 未删 / 未重命名 |
| `f5-verify/control-token` | 存在、tracked=NO、ignored=NO | ✅ 同上 |
| `gui-test/token` | 存在、tracked=NO、ignored=NO | ✅ 同上 |

- **未读取**任何 token 实际内容（仅 `-f` 存在性检查 + `git check-ignore`）。
- **未**修改 `.gitignore`（sha 仍 `f43418cf…`）。
- 继续禁止：`git add .` / `git add -A` / `git add -u` / `git commit -a`。

---

## 15. FROZEN-SCOPE CONFIRMATION

| 冻结范围 | 本阶段动作 | 判定 |
|---|---|---|
| **GAP-4**（`internal/attribution/*`、`cmd/coinbase-attribution/main.go`） | 未修改 / 未归一化 / 未重写 / 未重建；**未**尝试 SHA-drift 调和 | ✅ FROZEN |
| **F1-N1 / F1-N2**（含 `internal/blockchain/f1n1_reorg_coverage_test.go`） | 未修改 / 未暂存 | ✅ FROZEN |
| **Console** | 未修改任何 Console 文件；**未**修复 governance-evidence gap | ✅ FROZEN |
| **AUDIT-FIX** | **未**执行 / **未**复现；外部报告仅作 provenance evidence | ✅ FROZEN |
| **Flaky test** `TestF1N1_C_MixedLegacyV2Reorg` | **未修改**；状态保持 `OPEN / DO NOT FIX` | ✅ FROZEN |
| **Secret artifacts**（3 token） | 未读 / 未暂存 / 未改 / 未移 / 未删 / 未重命名 | ✅ FROZEN |

**⇒ 本阶段严格限制在 P3.1 provenance restoration，未混入任何其他工作流。**

---

## 16. LIMITATIONS

1. **仅文档恢复**：本阶段仅恢复**文档字节**。P3.1 文档的**后续提交资格**仍由 Owner 单独裁定；
   恢复后的文件当前**等价于 HEAD 内容**（`git diff --quiet` exit 0），因此**不会**在 `git status` 中显示为修改。
2. **orphan 内容未保留**：BRAND-0D.3 orphan 内容按 §7 要求**未**归档/未复制。
   其内容不再存在于工作树该路径（原始 SHA `98f036fb…` 已登记于本报告，可追溯）。
3. **零编译影响**：恢复的 `.md` 文档**未被任何 `.go` 文件引用**（`grep --include='*.go'` 无命中）
   ⇒ 对源码编译**零影响**；本阶段**未**以 build/test 为由修改任何无关文件。
4. **无独立验证者**：恢复等价性由**同一会话**内的 `cmp` / `sha256sum` / `git hash-object` 证明，
   未引入独立第三方验证（属本链一贯的 self-attested 证据模式）。
5. **报告自引用 SHA**：本报告 SHA 由**外部**命令计算（见 §16.1），报告正文**不内嵌**自身 SHA，避免自引用矛盾。

### 16.1 Final Report SHA-256（external record）

```
PHASE-F6.2-P3.1-PROVENANCE-RESTORATION-REPORT.md
SHA-256 = （见交付说明 / memory 的外部登记，本文件内不内嵌以避免自引用矛盾）
```

---

## 17. FINAL VERDICT

依据 §17 判定规则逐项核验：

| 判定条件 | 结果 |
|---|---|
| baseline 有效 | ✅ PASS |
| historical source 独立验证 | ✅ PASS（blob `08b74f1b…` + SHA `e9def9aa…`） |
| P3.1 恢复逐字节一致 | ✅ PASS（`cmp` IDENTICAL） |
| 无无关文件变更 | ✅ PASS（唯一变更 = 授权 P3.1 文档） |
| 受保护对象未变 | ✅ PASS |
| token 未触碰 | ✅ PASS |
| 无 Git 写操作 | ✅ PASS |

```
PHASE F-6.2 — VERDICT: PASS
```

**PASS**（非 PASS WITH LIMITATIONS、非 BLOCKED）。

---

## 18. NEXT-PHASE HARD STOP

```
PHASE F-6.2 COMPLETE
HARD STOP — WAITING FOR OWNER AUTHORIZATION
```

**不得自动进入**：

- GAP-4 SHA-drift closure
- F1-N1
- F1-N2
- Console governance remediation
- AUDIT-FIX
- compile dependency integration
- flaky-test stabilization
- Git commit
- cleanup
- release
- production operations

**最终原则**：

```
RESTORE PROVENANCE FIRST.
DO NOT MIX WORKSTREAMS.
DO NOT REPAIR UNAUTHORIZED SCOPE.
DO NOT COMMIT.
DO NOT AUTO-ADVANCE.
```

**HARD STOP。**
