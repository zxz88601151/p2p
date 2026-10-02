# PHASE F-6.8 — CB-1 F1-N1 COMMIT READINESS & CONTROLLED COMMIT — REPORT

**Phase ID:** F-6.8
**Phase Type:** OWNER-AUTHORIZED CONTROLLED WRITE (single-file commit)
**Object:** `internal/blockchain/f1n1_reorg_coverage_test.go` (CB-1)
**Baseline HEAD (pre-commit):** `4d892be355a38886a83c123faad915c9f3cd62e4`
**New HEAD (post-commit):** `dc5590728ef35ab0512ac3609abc2c3f59a731dd`
**Status:** `PHASE F-6.8 COMPLETE / CB-1 COMMITTED / OD-03 APPROVED / F1-N1 stability = OPEN / CB-2 = READY TO ENTER NEXT PHASE / HARD STOP`

---

## §0 PHASE PURPOSE

F-6.8 executes the Owner-authorized **CB-1 single-file controlled commit** — the F1-N1
prerequisite commit frozen by F-6.6 (OPT-B) and authorized by F-6.7 (OD-03 = APPROVED / OPTION-B).

Scope of this phase (exactly one file):

```
CB-1
└── internal/blockchain/f1n1_reorg_coverage_test.go
```

Objectives achieved: (1) final Commit Readiness verification; (2) identity/SHA/ownership/dependency
zero-drift confirmation; (3) confirmation that no file other than CB-1 entered the commit;
(4) explicit-path-whitelist staging; (5) single-file commit; (6) full post-commit acceptance;
(7) Genesis Closure 32-file boundary preserved (not committed); (8) F1-N1 stability preserved as
**OPEN**.

---

## §1 HARD BOUNDARY

### §1.1 The only file permitted to be committed

```
internal/blockchain/f1n1_reorg_coverage_test.go
```

### §1.2 Explicitly excluded from CB-1 (verified NOT staged — §7)

Genesis Closure 32 files · MEMORY.md · `PHASE-F6.7-OD03-OWNER-AUTHORIZATION.md` · F-6.6 deliverables ·
GAP-4 attribution files · CONSOLE workstream · AUDIT-FIX workstream · token files · any other
untracked `*.go` · any other tracked modification.

### §1.3 Prohibited commands (none executed)

`git add -A` · `git add .` · `git add --all` · `git commit -a` · `git restore` · `git checkout` ·
`git reset` · `git clean` · `git rebase` · `git merge` · `git amend` · `git squash` · `git push` ·
`git tag` · `gofmt -w`.

**Compliance:** only `git add -- <explicit single path>` and a single `git commit -m "…"` were
executed. No source or test file was modified.

---

## §2 BASELINE RE-VERIFICATION

Baseline re-verified at F-6.8 open against the F-6.7 exit record.

| Anchor | Expected (F-6.7) | Observed (F-6.8 open) | Result |
|---|---|---|---|
| HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` | `4d892be355a38886a83c123faad915c9f3cd62e4` | ✅ MATCH |
| Branch | `main` | `main` | ✅ MATCH |
| Staged | 0 | 0 | ✅ MATCH |
| Tracked modified | 35 | 35 | ✅ MATCH |
| Porcelain modified (`^ M`) | 36 | 36 | ✅ MATCH |
| CB-1 SHA-256 | `c4606f09829ccc02…` | `c4606f09829ccc02f6a057b48566c498a0e3af73338d82b898368643c88e438a` | ✅ MATCH |
| CB-1 size | 25,981 B | 25,981 B | ✅ MATCH |
| CB-1 mtime | `2026-09-27 10:52:35` | `2026-09-27 10:52:35` | ✅ MATCH |
| `node.exe` SHA-256 (prefix) | `3972843aff90428d` | `3972843aff90428d` | ✅ MATCH |
| `.gitignore` SHA-256 (prefix) | `f43418cf74ea996c` | `f43418cf74ea996c` | ✅ MATCH |
| Tokens (sizes) | 33 / 64 / 34 B | 33 / 64 / 34 B | ✅ MATCH |
| F-6.7 report SHA-256 (prefix) | `6d0667390b6047b7` | `6d0667390b6047b7` | ✅ MATCH |

**Baseline verdict: PASS — no drift.** No STOP triggered.

---

## §3 CB-1 OBJECT IDENTITY

| Property | Value |
|---|---|
| Path | `internal/blockchain/f1n1_reorg_coverage_test.go` |
| Git status (pre-commit) | **UNTRACKED** (`??`) — `git ls-files` = not known to git |
| Provenance | **F1-N1** |
| Ownership | **F1-N1** |
| SHA-256 | `c4606f09829ccc02f6a057b48566c498a0e3af73338d82b898368643c88e438a` |
| Size | 25,981 B (678 lines added on commit) |

Confirmed facts:

- Header self-declares `PHASE P2PCHAIN — F1-N1` and states it is a **test-only file** ("No production
  `.go`, storage primitive, reorg implementation … change is made").
- Defines `func f1n1UTXOSig(bc *Blockchain) string` — **exactly one definition** in the whole repo
  (`grep -rn "func f1n1UTXOSig"` → 1 hit, line 134).
- Consumed by `internal/blockchain/p1_reorg_fail_before_commit_test.go:63` (`utxoSig: f1n1UTXOSig(bc),`)
  — a **tracked, clean** file ⇒ CB-1 is compile-mandatory for `internal/blockchain`.
- **No duplicate / alternative implementation exists** (`find` for `*reorg_coverage*` → 1 file).
- SHA unchanged vs F-6.7.

---

## §4 OWNERSHIP / SCOPE RECHECK

- `OD-03 = APPROVED / OPTION-B` ✅
- `CB-1 = AUTHORIZED` ✅
- `CB-2 = WAITING FOR CB-1` ✅

Governance nature of this commit:

```
F1-N1 prerequisite commit
NOT Genesis Closure commit
```

CB-1's ownership was **not** redefined on the basis of it being a Genesis compile dependency
(rule: `compile dependency ≠ phase ownership`, MEMORY §9).

---

## §5 STABILITY PRESERVATION

Confirmed **unmodified**:

- `internal/block/block.go:43` → `Timestamp: time.Now().Unix(),` (present, file clean / not modified).
- 256-attempt bounded retry → `f1n1MineAbove` (line 104, `for i := 0; i < 256; i++` at 106, `t.Fatalf`
  "no sibling above threshold after 256 attempts" at 113); `f1n1MineBelow` (line 119, loop at 121,
  `t.Fatalf` "no sibling below threshold after 256 attempts" at 128).
- reorg semantics · mixed Legacy/V2 test semantics — all inside CB-1, whose SHA is byte-identical to
  F-6.7 (`c4606f09…`), therefore unchanged.

`TestF1N1_C_MixedLegacyV2Reorg` (line 459) status **preserved verbatim**:

```
TestF1N1_C_MixedLegacyV2Reorg = OPEN / LOAD-SENSITIVE / WALL-CLOCK-SENSITIVE
```

No repair was performed. No test logic was modified to "make it pass".

---

## §6 PRE-COMMIT TEST / BUILD GATE

| Gate | Command | Result |
|---|---|---|
| Compile (package) | `go vet ./internal/blockchain` | exit **0** ✅ |
| Build (all) | `go build ./...` | exit **0** ✅ |
| Test (CB-1 package) | `go test ./internal/blockchain -count=1 -timeout 25m` | **ok** `p2pchain/internal/blockchain` **74.808s**, exit **0** ✅ |

Observation: the run **PASSED**; `TestF1N1_C_MixedLegacyV2Reorg` did **not** fail on this execution.
Per §5 / F-6.5, **a single PASS is not stability evidence** and does **not** close the item.

```
KNOWN F1-N1 STABILITY LIMITATION — recorded, NOT repaired.
```

Readiness proceeded on the basis of the CB-1 commit contract, **not** by treating the flaky item as
resolved.

---

## §7 CANDIDATE SET ASSERTION

Pre-`git add` state:

- `git diff --cached --name-only` → **empty** (staged = 0).
- `git status --porcelain -- internal/blockchain/f1n1_reorg_coverage_test.go` → `?? internal/blockchain/f1n1_reorg_coverage_test.go`.

```
CB-1 candidate = internal/blockchain/f1n1_reorg_coverage_test.go
```

Everything else remained unstaged / untracked / untouched. Verified NOT in candidate:

| Check | Result |
|---|---|
| MEMORY.md in repo | **absent** (lives outside repo at `E:/wakuang/.workbuddy-ai/memory/`) |
| token files staged | 0 |
| token files tracked | 0 |
| Genesis candidate files staged (`identity.go`, `f3b_*`) | 0 |
| GAP-4 attribution staged (`attribution*`, `coinbase-attribution`) | 0 |
| F1-N2 files staged | 0 |
| total staged | **0** |

Full untracked `*.go` inventory (13 files) — **all** remained untracked except CB-1:
`cmd/coinbase-attribution/main.go`, `cmd/node/f3b_identity_test.go`, `cmd/node/f3b_test_helpers_test.go`,
`internal/attribution/attribution.go`, `internal/attribution/attribution_test.go`,
`internal/attribution/classify.go`, `internal/blockchain/f1n1_reorg_coverage_test.go`,
`internal/blockchain/f1n2_reorg_detached_undo_test.go`, `internal/blockchain/identity.go`,
`internal/control/console_mine_auth_test.go`, `internal/storage/f1n1_undo_contract_test.go`,
`internal/storage/f1n2_detached_undo_contract_test.go`, `internal/utxo/f1n1_undo_content_test.go`.

---

## §8 READINESS DECISION

| Gate | Criterion | Result |
|---|---|---|
| R1 | baseline PASS | ✅ |
| R2 | object identity PASS | ✅ |
| R3 | ownership PASS | ✅ |
| R4 | provenance PASS | ✅ |
| R5 | scope PASS | ✅ |
| R6 | stability preservation PASS | ✅ |
| R7 | candidate isolation PASS | ✅ |
| R8 | required validation PASS | ✅ |
| R9 | secret exclusion PASS | ✅ |

```
COMMIT READY
```

No scope expansion was performed.

---

## §9 CONTROLLED STAGING

```
git add -- internal/blockchain/f1n1_reorg_coverage_test.go    (exit 0)
```

Verification immediately after staging:

```
git diff --cached --name-only   →  internal/blockchain/f1n1_reorg_coverage_test.go   (count = 1)
git diff --cached --stat        →  1 file changed, 678 insertions(+)
git diff --cached --check       →  exit 0 (no whitespace errors)
```

Exactly **1 file**. ✅

Staged blob SHA (pre-commit) = `c4606f09829ccc02f6a057b48566c498a0e3af73338d82b898368643c88e438a`
== working-tree SHA (byte-identical; no CRLF corruption on stage).

---

## §10 COMMIT

Final re-confirmation before commit: `git diff --cached --name-only` → exactly
`internal/blockchain/f1n1_reorg_coverage_test.go` (count = 1).

Single commit executed:

```
git commit -m "test: add F1-N1 reorg coverage prerequisite"
```

Output:

```
[main dc55907] test: add F1-N1 reorg coverage prerequisite
 1 file changed, 678 insertions(+)
 create mode 100644 internal/blockchain/f1n1_reorg_coverage_test.go
```

No other Git write operation was performed.

---

## §11 POST-COMMIT VERIFICATION

### §11.1 HEAD

```
HEAD    = dc5590728ef35ab0512ac3609abc2c3f59a731dd
parent  = 4d892be355a38886a83c123faad915c9f3cd62e4
subject = test: add F1-N1 reorg coverage prerequisite
author  = p2pchain-baseline <baseline@p2pchain.local>
date    = Mon Sep 28 09:25:58 2026 +0800
```

### §11.2 Commit file set

```
git show --stat --oneline HEAD  →  1 file changed, 678 insertions(+)
git diff-tree --no-commit-id --name-only -r HEAD  →  internal/blockchain/f1n1_reorg_coverage_test.go   (count = 1)
```

Exactly **1 file**, and only `internal/blockchain/f1n1_reorg_coverage_test.go`. ✅

### §11.3 Working tree

- staged = **0** ✅
- tracked-M = **35** (unchanged) ✅
- porcelain-M = **36** (unchanged) ✅
- porcelain total = 241 → **240** (CB-1 removed from untracked) ✅
- CB-1 no longer untracked (status query → 0 lines) ✅
- All other pre-existing modifications/untracked artifacts were **not** cleaned or modified. ✅

### §11.4 Token exclusion

| Check | Result |
|---|---|
| tokens tracked by new HEAD | **0** |
| tokens in new commit | **0** |
| tokens staged | **0** |

### §11.5 Candidate SHA

```
committed blob SHA (git cat-file blob HEAD:<path>) = c4606f09829ccc02f6a057b48566c498a0e3af73338d82b898368643c88e438a
pre-commit working-tree SHA                        = c4606f09829ccc02f6a057b48566c498a0e3af73338d82b898368643c88e438a
```

**Byte-identical.** ✅

### §11.6 HEAD stability

`git rev-parse HEAD` read 3× in immediate succession + 1 delayed read (+4 s) → identical
`dc5590728ef35ab0512ac3609abc2c3f59a731dd` every time. HEAD did not change. ✅

---

## §12 GOVERNANCE STATE AFTER SUCCESS

```
F-6.8 = COMPLETE
CB-1 = COMMITTED
OD-03 = APPROVED
F1-N1 prerequisite = COMMITTED
F1-N1 stability = OPEN
CB-2 = READY TO ENTER NEXT PHASE
Genesis Closure = NOT YET COMMITTED
HARD STOP
```

CB-1 commit record:

```
COMMIT SHA: dc5590728ef35ab0512ac3609abc2c3f59a731dd
PARENT:     4d892be355a38886a83c123faad915c9f3cd62e4
FILES:      internal/blockchain/f1n1_reorg_coverage_test.go  (1 file, 678 insertions)
SHA-256:    c4606f09829ccc02f6a057b48566c498a0e3af73338d82b898368643c88e438a
```

---

## §13 NEXT-PHASE BOUNDARY

F-6.8 does **not** auto-execute CB-2. The next phase must be separately authorized:

```
CB-2 — GENESIS CLOSURE CONTROLLED COMMIT
```

CB-2 is fixed as the F-6.6-frozen set:

```
A(7) + B(22) + F(3) = 32 files
```

with `CB-1 prerequisite already committed` (satisfied by this phase). Only then may CB-2 readiness be
re-run. CB-2 acceptance gate (F-6.6): `G-L1 ∧ G-L2 ∧ G-L2b`; `G-L3` (full-suite green) remains an
independent tracking item and does not block CB-2.

Residual items carried forward (unchanged): BLK-3 flaky (`TestF1N1_C_MixedLegacyV2Reorg`, still OPEN)
· BLK-4 `cmd/node` acceptance margin +2.228 s (0.37 %) · BLK-6 authoritative inputs 4/5 absent ·
F6.6-ANOM-1 mtime re-stamp (content zero-drift).

---

## FINAL HARD STOP

Success definition met:

```
ONE OWNER-AUTHORIZED F1-N1 FILE
        ↓
ONE EXPLICITLY STAGED FILE
        ↓
ONE COMMIT
        ↓
ONE-FILE COMMIT VERIFIED
        ↓
NO OTHER WORKSTREAM MOVED
        ↓
F1-N1 STABILITY REMAINS OPEN
        ↓
HARD STOP
```

No scope expansion, candidate contamination, baseline drift, secret contamination, or unauthorized
source modification occurred. This report itself is a **new untracked file** and was **not** committed.

---

*End of PHASE F-6.8 — CB-1 F1-N1 COMMIT READINESS & CONTROLLED COMMIT.*
