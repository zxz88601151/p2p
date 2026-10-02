# PHASE CB-2 — GENESIS CLOSURE COMMIT READINESS & CONTROLLED COMMIT — REPORT

**Phase ID:** CB-2
**Phase Type:** OWNER-AUTHORIZED CONTROLLED WRITE (32-file controlled commit)
**Object:** Genesis Closure 32-file boundary (A=7 + B=22 + F=3), per F-6.6 `COMMIT-CANDIDATE-FINAL-MANIFEST.md`
**Baseline HEAD (pre-commit):** `dc5590728ef35ab0512ac3609abc2c3f59a731dd`
**New HEAD (post-commit):** `017493817c929d9d2a2cb147e55ea6f7d41f1934`
**Status:** `CB-2 COMPLETE / GENESIS CLOSURE COMMITTED / CB-1 prerequisite COMMITTED / Genesis Closure = CLOSED / F1-N1 stability = OPEN / GAP-4 = DEFERRED / CONSOLE/AUDIT-FIX = DEFERRED / HARD STOP`

---

## §0 PHASE PURPOSE

Execute the Owner-authorized **CB-2 — Genesis Closure Controlled Commit**, limited strictly to the
F-6.6-frozen 32-file Genesis Closure boundary. CB-1 (`internal/blockchain/f1n1_reorg_coverage_test.go`)
was already committed at `dc55907` and **must not re-enter CB-2**.

---

## §1 FROZEN CB-2 BOUNDARY

```
A = 7   Genesis production files
B = 22  Genesis tests / fixtures
F = 3   mandatory dependency files
TOTAL = 32 files
```

Obtained directly from the frozen manifests (no path was guessed):

- **F-6.6 `COMMIT-CANDIDATE-FINAL-MANIFEST.md`** §3 (the operative 32-file list).
- **F-6.4 `PHASE-F6.4-GENESIS-COMMIT-CANDIDATE-MANIFEST.md`** §2 (GCS = 33 = CB-2 32 + X-01) — cross-check.

Both manifests are mutually consistent: F-6.4 GCS(33) = F-6.6 CB-2(32) + X-01(1).

---

## §2 ABSOLUTE EXCLUSIONS (verified absent from CB-2)

`internal/blockchain/f1n1_reorg_coverage_test.go` (committed at `dc55907`) · `MEMORY.md` ·
`PHASE-F6.7-OD03-OWNER-AUTHORIZATION.md` · `PHASE-F6.8-CB1-CONTROLLED-COMMIT-REPORT.md` · F-6.6
deliverables · GAP-4 attribution files · F1-N2 files · CONSOLE files · AUDIT-FIX files · any
token/control-token · any unknown untracked file · any non-Genesis tracked modification.

Prohibited commands (`git add -A` / `.` / `--all` / `git commit -a`) were **not** used. Only
`git add -- <32 explicit manifest paths>` and a single `git commit -m` were executed.

---

## §3 BASELINE VERIFICATION

| Anchor | Expected (F-6.8 exit) | Observed (CB-2 open) | Result |
|---|---|---|---|
| HEAD | `dc5590728ef35ab0512ac3609abc2c3f59a731dd` | `dc5590728ef35ab0512ac3609abc2c3f59a731dd` | ✅ MATCH |
| short | `dc55907` | `dc55907` | ✅ MATCH |
| parent | `4d892be355a38886a83c123faad915c9f3cd62e4` | `4d892be355a38886a83c123faad915c9f3cd62e4` | ✅ MATCH |
| Branch | `main` | `main` | ✅ MATCH |
| Staged | 0 | 0 | ✅ MATCH |
| Tracked modified | 35 | 35 | ✅ MATCH |
| Porcelain modified | 36 | 36 | ✅ MATCH |
| Untracked total | 378 | 378 | ✅ MATCH |
| CB-1 commit SHA | `dc55907` | `dc55907` | ✅ MATCH |
| CB-1 commit file count | 1 | 1 | ✅ MATCH |
| CB-1 committed blob SHA | `c4606f09829ccc02…` | `c4606f09829ccc02f6a057b48566c498a0e3af73338d82b898368643c88e438a` | ✅ MATCH |
| `.gitignore` SHA-256 (prefix) | `f43418cf74ea996c` | `f43418cf74ea996c` | ✅ MATCH |
| `node.exe` SHA-256 (prefix) | `3972843aff90428d` | `3972843aff90428d` | ✅ MATCH |
| Tokens (sizes) | 33 / 64 / 34 B | 33 / 64 / 34 B | ✅ MATCH |
| F-6.8 report SHA-256 (prefix) | `1e8ec8491ca30825` | `1e8ec8491ca30825` | ✅ MATCH |

`dc55907` is confirmed the **F1-N1 prerequisite commit**, containing exactly **1 file**.
**Baseline verdict: PASS — no drift.**

---

## §4 MANIFEST AUTHORITY CHECK

```
CB2_MANIFEST:
  count        = 32
  unique paths = 32
  source       = F-6.6 §3 (operative) + F-6.4 §2 (cross-check)
```

Each of the 32 entries carries `path` / `ownership` (F-3B = Genesis) / `class` (A/B/F) /
`expected SHA-256` / `role`. No manifest/repository inconsistency was found. No manifest was edited.

| Group | Count | git status |
|---|---|---|
| A — F-3B production | 7 | tracked `M` |
| B — F-3B fixtures | 22 | tracked `M` |
| F — F-3B new sources | 3 | untracked `??` |
| **TOTAL** | **32** | **29 M + 3 ??** |

---

## §5 CB-1 EXCLUSION GATE

```
git ls-tree -r --name-only HEAD | grep -c 'f1n1_reorg_coverage_test.go'  →  1   (in HEAD)
grep -c 'f1n1_reorg_coverage_test.go'  CB2_MANIFEST                      →  0   (not a candidate)
```

**CB-1 ≠ CB-2 — CONFIRMED.** CB-2 handles only the Genesis 32-file closure.

---

## §6 GENESIS OWNERSHIP GATE

All 32 candidates re-verified as **Genesis Closure ownership** (F-3B identity implementation).
Excluded-set contamination scan over the candidate list — all zero:

| Pattern | Hits |
|---|---|
| `attribution` / `coinbase-attribution` (GAP-4) | 0 / 0 |
| `f1n2` (F1-N2) | 0 |
| `f1n1_undo` (F1-N1 residue) | 0 |
| `console` (CONSOLE) | 0 |
| `explorer` / `r1_boundary` (AUDIT-FIX / R1) | 0 / 0 |
| `query.go` (CRLF pseudo-diff) | 0 |
| `control-token` (secret) | 0 |

No F1-N1 / F1-N2 / GAP-4 / CONSOLE / AUDIT-FIX provenance was absorbed into Genesis.

---

## §7 G-L1 — COMPILE CLOSURE

| Gate | Command | Result |
|---|---|---|
| Vet | `go vet ./...` | exit **0** ✅ |
| Build | `go build ./...` | exit **0** ✅ |

Genesis implementation symbols resolve on `Genesis candidate + current HEAD`: `ErrUninitializedStore`,
`VerifyGenesisIdentity`, canonical genesis, `OpenFileBlockStoreStrict`, `node init` lifecycle, verify
identity gate — all resolve (vet/build clean).

Note: `f1n1UTXOSig` is supplied by **CB-1** (already in the parent commit `dc55907`) and is **not** a
CB-2 candidate.

---

## §8 G-L2 — BLOCKCHAIN ISOLATION

```
go test -count=1 ./internal/blockchain/...   →   ok  p2pchain/internal/blockchain  70.458s   (exit 0)
```

Genesis candidate tests/implementation validate independently on `dc55907`, **without** requiring
GAP-4 / F1-N2 / CONSOLE / AUDIT-FIX. No "extra unknown file needed to compile Genesis tests" case
occurred ⇒ no boundary expansion.

---

## §9 G-L2b — GENESIS OWN TESTS

```
go test -count=1 -run 'TestF3BExplicitInitializationStateMatrix' ./cmd/node   →   ok  p2pchain/cmd/node  1.679s   (exit 0)
```

Genesis own-test behaviours covered by the frozen F-4/F-5 acceptance set (fresh init · repeat init
refusal · verify canonical genesis · empty-dir fail-closed · legacy genesis mismatch · legacy init
refusal · 0-byte state handling · real-node genesis identity · genesis linkage · subsidy = 5 ·
restart persistence). **Genesis own tests = PASS.**

---

## §10 G-L3 — FULL SUITE

```
go test -count=1 ./...   →   exit 0   (duration ≈ 10m12s wall)
```

| Package | Result |
|---|---|
| `cmd/node` | ok **593.646s** |
| `internal/storage` | ok 317.601s |
| `internal/blockchain` | ok 85.218s |
| `internal/mempool` | ok 36.756s |
| `internal/p2p` | ok 15.462s |
| `internal/control` | ok 4.695s |
| `internal/wallet` | ok 4.610s |
| `internal/pow` | ok 3.557s |
| `internal/block` | ok 1.141s |
| `internal/explorer` | ok 0.809s |
| `internal/blocktree` | ok 0.465s |
| `internal/transaction` | ok 0.459s |
| `internal/obs` | ok 0.405s |
| `internal/utxo` | ok 0.396s |
| `internal/attribution` | ok 0.378s |
| `internal/txbuild` | ok 0.363s |
| `cmd/coinbase-attribution`, `cmd/explorer`, `internal/config` | no test files |

**All 18 test packages PASS — no FAIL, no timeout.** Per F-6.5, G-L3 full-suite green is **not** a
CB-2 acceptance blocker; it passed this run anyway. **No Genesis-specific regression observed.**
(Note: `cmd/node` 593.646s is load-dependent and sits close to the 600s default limit — recorded as a
standing observation, not a CB-2 blocker.)

---

## §11 KNOWN F1-N1 STABILITY STATUS

Preserved, unchanged:

```
TestF1N1_C_MixedLegacyV2Reorg = OPEN / LOAD-SENSITIVE / WALL-CLOCK-SENSITIVE
```

Not closed by CB-1's commit nor by any single passing run. Unmodified: `block.go:43`
`time.Now().Unix()`, the 256 retry bound, `f1n1MineAbove`, `f1n1MineBelow`. **CB-2 does not resolve
F1-N1 stability.**

---

## §12 CANDIDATE CONSTRUCTION

`EXPECTED_CB2 = 32` (from manifest) vs `ACTUAL_CB2` (from Git state):

```
count            = 32
set equality     = TRUE
git status of 32 = 29 M + 3 ??
```

Absent from candidate (all verified): CB-1 file · token files · `MEMORY.md` · F-6.8 report · GAP-4 ·
F1-N2 · CONSOLE · AUDIT-FIX.

**Inventory accounting (zero unknown files):**
- tracked-M **35** = CB-2 A+B (29) + SET B (5) + D-class doc (1).
- untracked `.go` **12** = CB-2 F (3) + GAP-4 (4) + F1-N2 (2) + F1-N1 residue (2) + CONSOLE (1).

---

## §13 SHA / CONTENT INTEGRITY

All 32 candidate files' current SHA-256 compared against the manifest expected SHA-256:

```
OK = 32   BAD = 0
```

**ALL 32 MATCH.** No "harmless formatting" assumption was needed.

---

## §14 READINESS GATE

| Gate | Criterion | Result |
|---|---|---|
| R1 | baseline | ✅ PASS |
| R2 | manifest authority | ✅ PASS |
| R3 | CB-1 exclusion | ✅ PASS |
| R4 | ownership | ✅ PASS |
| R5 | G-L1 compile closure | ✅ PASS |
| R6 | G-L2 blockchain isolation | ✅ PASS |
| R7 | G-L2b Genesis tests | ✅ PASS |
| R8 | SHA/content integrity | ✅ PASS |
| R9 | candidate set isolation | ✅ PASS |
| R10 | secret exclusion | ✅ PASS |

```
CB-2 COMMIT READY
```

---

## §15 CONTROLLED STAGING

```
git add -- <32 explicit manifest paths>      (exit 0)
```

Verification:

```
git diff --cached --name-only | wc -l   →  32
SET_EQUALITY (cached vs manifest)       →  PASS
git diff --cached --shortstat           →  32 files changed, 605 insertions(+), 170 deletions(-)
git diff --cached --check               →  exit 0
```

`cached files = exactly 32` and `set(cached) == set(CB2_MANIFEST)`. ✅

---

## §16 PRE-COMMIT FINAL GATE

- `git diff --cached --name-only` → **32 files** ✅
- `git diff --cached --check` → **PASS (exit 0)** ✅
- Contamination counts (all **0**): tokens · CB-1 · GAP-4 · F1-N2 · CONSOLE · AUDIT-FIX · MEMORY ·
  `query.go` · D-class doc.
- Staged blob SHA spot-check == manifest: `identity.go` `9aa8fc6e…`, `f3b_identity_test.go`
  `b7a9e5a7…`, `main.go` `f22436ba…`. ✅

---

## §17 CONTROLLED COMMIT

```
git commit -m "feat: close genesis identity and new-chain boundary"
```

Output:

```
[main 0174938] feat: close genesis identity and new-chain boundary
 32 files changed, 605 insertions(+), 170 deletions(-)
 create mode 100644 cmd/node/f3b_identity_test.go
 create mode 100644 cmd/node/f3b_test_helpers_test.go
 create mode 100644 internal/blockchain/identity.go
```

Only this one CB-2 commit was executed. No amend / squash / rebase / merge / push / tag.

---

## §18 POST-COMMIT VERIFICATION

### §18.1 HEAD

```
HEAD    = 017493817c929d9d2a2cb147e55ea6f7d41f1934
parent  = dc5590728ef35ab0512ac3609abc2c3f59a731dd
subject = feat: close genesis identity and new-chain boundary
author  = p2pchain-baseline <baseline@p2pchain.local>
date    = Mon Sep 28 09:45:10 2026 +0800
```

### §18.2 Commit file count

```
git diff-tree --no-commit-id --name-only -r HEAD | wc -l  →  32
```

### §18.3 Exact set equality

```
COMMITTED_SET == CB2_MANIFEST : PASS
committed blob SHA vs manifest (all 32): ok = 32, bad = 0
```

### §18.4 CB-1 ancestry

```
git merge-base --is-ancestor dc55907 HEAD  →  PASS
```

`dc55907` (CB-1) remains in the ancestry of the new Genesis commit.

### §18.5 Working tree

- staged = **0** ✅
- tracked-M = **6** = SET B (5) + D-class doc (1) — exactly the non-CB-2 remainder ✅
- porcelain-M = **7** = 6 + `query.go` (CRLF pseudo-diff) ✅
- untracked `.go` = **9** (12 − 3 committed) ✅
- No `restore` / `clean` / `reset` was used to "clean" the tree; all non-CB-2 modifications remain
  untouched.

### §18.6 Token exclusion

```
tokens in commit = 0      tokens tracked = 0      tokens staged = 0
```

### §18.7 HEAD stability

`git rev-parse HEAD` read, then re-read after a 4 s delay → identical `0174938…`. HEAD stable. ✅

---

## §19 FINAL STATE IF PASS

```
CB-2 COMPLETE
GENESIS CLOSURE COMMITTED
CB-1 prerequisite COMMITTED
Genesis Closure = CLOSED
F1-N1 stability = OPEN
GAP-4 = DEFERRED
CONSOLE/AUDIT-FIX = DEFERRED
HARD STOP
```

New Git baseline:

```
GENESIS_CLOSURE_HEAD = 017493817c929d9d2a2cb147e55ea6f7d41f1934
PARENT               = dc5590728ef35ab0512ac3609abc2c3f59a731dd
FILES                = 32
```

Ancestry: `0174938 (Genesis Closure)` ← `dc55907 (CB-1 F1-N1)` ← `4d892be (prior baseline)`.

---

## §20 IMPORTANT — DO NOT AUTO-ADVANCE

CB-2 success does **not** auto-enter: F1-N1 stability remediation · GAP-4 closure · CONSOLE cleanup ·
AUDIT-FIX · G3/E0. A new Gate is formed; the next phase is decided by the Owner from this report.

---

## FINAL SUCCESS CONDITION

```
dc55907
   │
   ├── CB-1
   │   └── f1n1_reorg_coverage_test.go
   │
   └── CB-2
       └── exactly 32 Genesis Closure files
              ↓
        exact manifest equality  (COMMITTED_SET == CB2_MANIFEST)
              ↓
        post-commit verification  (32/32 blob SHA match, CB-1 ancestry, staged=0, tokens excluded)
              ↓
          HARD STOP
```

The "Genesis Closure 32 files" boundary was **not** expanded into "all current working-tree
modifications" — the 6 tracked + 9 untracked non-CB-2 artifacts remain untouched. No anomaly
occurred; no STOP was triggered.

---

*End of PHASE CB-2 — GENESIS CLOSURE COMMIT READINESS & CONTROLLED COMMIT.*
