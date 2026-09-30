#!/usr/bin/env bash
# ============================================================================
# scripts/git-guard-test.sh — tests for scripts/git-guard.sh
# ----------------------------------------------------------------------------
# Authority: PHASE-P2PCHAIN-WORKTREE-COMMIT-BOUNDARY-1 §12
#
# Design note: the guard takes an explicit path list via --paths, so these
# tests are PURE-FUNCTION tests — they never stage, unstage, or otherwise
# write to the repository. Zero git write operations in this test suite.
#
# Exit codes: 0 = all cases behaved as expected; 1 = at least one failure.
# ============================================================================
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
GUARD="$HERE/git-guard.sh"

pass=0
fail=0

expect_block() { # expect_block <case-name> <path...>
    local name="$1"; shift
    local out rc
    out="$(bash "$GUARD" --paths "$@" 2>&1)"; rc=$?
    if [ "$rc" -eq 1 ]; then
        pass=$((pass+1))
        printf '  ok   BLOCK  %-34s -> %s\n' "$name" "$(echo "$out" | head -1)"
    else
        fail=$((fail+1))
        printf '  FAIL expect-BLOCK got-rc=%s  %-34s  out=%s\n' "$rc" "$name" "$out"
    fi
}

expect_pass() { # expect_pass <case-name> <path...>
    local name="$1"; shift
    local out rc
    out="$(bash "$GUARD" --paths "$@" 2>&1)"; rc=$?
    if [ "$rc" -eq 0 ]; then
        pass=$((pass+1))
        printf '  ok   PASS   %-34s\n' "$name"
    else
        fail=$((fail+1))
        printf '  FAIL expect-PASS got-rc=%s  %-34s  out=%s\n' "$rc" "$name" "$out"
    fi
}

echo "== git-guard test suite =="
echo
echo "-- §12 Positive: normal project files MUST PASS --"
expect_pass "go source"              internal/control/server.go
expect_pass "go source (cmd)"        cmd/node/main.go
expect_pass "go test file"           internal/utxo/f1n1_undo_content_test.go
expect_pass "verifier python"        verifier/crosscheck.py
expect_pass "verifier test"          verifier/test_p2pchain_verify.py
expect_pass "verifier doc"           verifier/TEST-VECTORS.md
expect_pass "canonical test script"  scripts/run-tests.sh
expect_pass "batch (all above)"      internal/control/server.go cmd/node/main.go verifier/crosscheck.py scripts/run-tests.sh

echo
echo "-- §12 Negative: credentials / keys MUST BLOCK --"
expect_block "control-token (root)"        control-token
expect_block "control-token (audit-run)"   audit-run/control-token
expect_block "control-token (f5-verify)"   f5-verify/control-token
expect_block "token (gui-test)"            gui-test/token
expect_block "token (deep dir)"            some/deep/dir/token
expect_block "token.local"                 token.local
expect_block "wallet.json"                 wallet.json
expect_block "wallet.json (run-b)"         run-b/wallet.json
expect_block ".env"                        .env
expect_block ".env.production"             config/.env.production
expect_block "server.pem"                  certs/server.pem
expect_block "machine.key"                 keys/machine.key

echo
echo "-- §12 Path policy: MUST NOT TRACK dirs MUST BLOCK --"
expect_block ".workbuddy/ memory"          .workbuddy/memory/MEMORY.md
expect_block ".workbuddy/ snapshot"        .workbuddy/snapshots/01-x.md
expect_block "gui/ source"                 gui/main.py
expect_block "gui/ build"                  gui/build/app.spec
expect_block "gui/ dist"                   gui/dist/app/app.py
expect_block "audit-run/ log"              audit-run/node.log
expect_block "f5-verify/ artifact"         f5-verify/vector.json
expect_block "gui-test/ artifact"          gui-test/trace.txt

echo
echo "-- extra: runtime / build artifacts MUST BLOCK --"
expect_block "blocks.dat"                  datadir/blocks.dat
expect_block "node.lock"                   datadir/node.lock
expect_block "node.exe"                    bin/node.exe
expect_block "pkg.test"                    pkg.test
expect_block "cover.out"                   cover.out

echo
echo "-- §12 Path policy: verifier/ MUST PASS --"
expect_pass "verifier (all 6)"     verifier/TEST-VECTORS.md verifier/crosscheck.py verifier/gen_test_vectors.py verifier/negcheck.py verifier/p2pchain_verify.py verifier/test_p2pchain_verify.py

echo
echo "-- usage errors --"
out="$(bash "$GUARD" --paths 2>&1)"; rc=$?
if [ "$rc" -eq 2 ]; then
    pass=$((pass+1)); echo "  ok   rc=2    --paths with no argument"
else
    fail=$((fail+1)); echo "  FAIL expect-rc=2 got-rc=$rc  --paths with no argument"
fi
out="$(bash "$GUARD" --bogus 2>&1)"; rc=$?
if [ "$rc" -eq 2 ]; then
    pass=$((pass+1)); echo "  ok   rc=2    unknown flag"
else
    fail=$((fail+1)); echo "  FAIL expect-rc=2 got-rc=$rc  unknown flag"
fi

echo
total=$((pass+fail))
echo "TEST RESULT: ${pass}/${total} cases OK"
if [ "$fail" -ne 0 ]; then
    echo "TEST RESULT: FAIL (${fail} failing)"
    exit 1
fi
echo "TEST RESULT: PASS"
exit 0
