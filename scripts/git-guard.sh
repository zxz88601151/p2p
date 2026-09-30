#!/usr/bin/env bash
# ============================================================================
# scripts/git-guard.sh — COMMIT BOUNDARY GUARD
# ----------------------------------------------------------------------------
# Authority:
#   PHASE-P2PCHAIN-GOVERNANCE-DECISION-ADJUDICATION-1  §3 (MF-2) / §4 (D-WT)
#   PHASE-P2PCHAIN-WORKTREE-COMMIT-BOUNDARY-1          §7-§9
#
# Purpose:
#   Assert that the STAGED file list contains no forbidden path.
#   This is the machine-enforced layer of the Explicit Path Commit Policy.
#
# Usage:
#   bash scripts/git-guard.sh                    # check current staged list
#   bash scripts/git-guard.sh --paths p1 p2 ...  # check an explicit list
#                                                #   (dry-run / test mode)
#   bash scripts/git-guard.sh --help
#
# Exit codes:
#   0  = PASS   (no staged path hits a blocked rule)
#   1  = BLOCK  (at least one staged path hits a blocked rule)
#   2  = usage error
#
# HARD RULES (WORKTREE-COMMIT-BOUNDARY-1 §9) — this guard NEVER:
#   deletes files / resets / unstages / edits .gitignore / commits / pushes
#   / cleans / touches the user worktree. It only prints PASS or BLOCK and
#   names the offending staged path and the rule it hit.
#
# NOTE (*.log): intentionally NOT blocked here — pending Owner decision OD-8.
# ============================================================================
set -u

GUARD_PROGNAME="${0##*/}"

usage() {
    cat <<'USAGE'
Usage:
  bash scripts/git-guard.sh                    check current staged list
  bash scripts/git-guard.sh --paths p1 p2 ...  check an explicit path list
  bash scripts/git-guard.sh --help             show this help

Exit codes: 0 = PASS, 1 = BLOCK, 2 = usage error
USAGE
}

# --- one path -> prints rule on violation, returns 1; else returns 0 --------
check_path() {
    local p="$1"
    local b="${p##*/}"

    # -- credentials (exact basename, any directory) ------------------------
    case "$b" in
        control-token|token|token.local)
            echo "rule=credential-filename ($b) [MF-1/.gitignore:29-30]"
            return 1 ;;
    esac

    # -- wallet / env --------------------------------------------------------
    if [ "$b" = "wallet.json" ]; then
        echo "rule=wallet-filename (wallet.json) [.gitignore:26]"
        return 1
    fi
    if [ "$b" = ".env" ]; then
        echo "rule=env-file (.env) [.gitignore:12]"
        return 1
    fi
    case "$b" in
        .env.*) echo "rule=env-file-pattern (.env.*) [.gitignore:13]"
                return 1 ;;
    esac

    # -- private key material ------------------------------------------------
    case "$b" in
        *.pem|*.key)
            echo "rule=private-key-suffix ($b) [MF-2 blocked paths]"
            return 1 ;;
    esac

    # -- runtime data --------------------------------------------------------
    case "$b" in
        *.dat|*.lock)
            echo "rule=runtime-data-suffix ($b) [.gitignore:24-25]"
            return 1 ;;
    esac

    # -- build artifacts -----------------------------------------------------
    case "$b" in
        *.exe|*.test|*.out)
            echo "rule=build-artifact-suffix ($b) [.gitignore:4-6]"
            return 1 ;;
    esac

    # -- governance MUST-NOT-TRACK directories (root-anchored) ---------------
    case "$p" in
        .workbuddy/*)
            echo "rule=policy (.workbuddy/ MUST NOT TRACK) [D-WT §4.3]"
            return 1 ;;
        gui/*)
            echo "rule=policy (gui/ MUST NOT TRACK — external auxiliary tooling) [D-WT §4.5]"
            return 1 ;;
        audit-run/*|f5-verify/*|gui-test/*)
            echo "rule=policy (${p%%/*}/ MUST NOT TRACK — local runtime data) [D-WT §4.4 B9-B11]"
            return 1 ;;
        run-a/*|run-b/*)
            echo "rule=policy (${p%%/*}/ MUST NOT TRACK — runtime wallet dirs) [.gitignore:22-23]"
            return 1 ;;
        bin/*|dist/*)
            echo "rule=policy (${p%%/*}/ MUST NOT TRACK — build output) [.gitignore:2-3]"
            return 1 ;;
    esac

    return 0
}

# --- collect input ----------------------------------------------------------
paths=()
if [ "$#" -eq 0 ]; then
    mapfile -t paths < <(git diff --cached --name-only 2>/dev/null)
elif [ "$1" = "--paths" ]; then
    shift
    [ "$#" -eq 0 ] && { echo "$GUARD_PROGNAME: --paths needs at least one path" >&2; exit 2; }
    paths=("$@")
elif [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    usage; exit 0
else
    echo "$GUARD_PROGNAME: unknown argument: $1" >&2
    usage >&2
    exit 2
fi

# --- evaluate ----------------------------------------------------------------
if [ "${#paths[@]}" -eq 0 ]; then
    echo "GUARD: PASS (0 paths checked — nothing staged)"
    exit 0
fi

blocked=0
for p in "${paths[@]}"; do
    rule="$(check_path "$p")"
    if [ -n "$rule" ]; then
        echo "GUARD: BLOCK  $p  ($rule)"
        blocked=1
    fi
done

if [ "$blocked" -eq 1 ]; then
    echo "GUARD RESULT: BLOCK"
    exit 1
fi
echo "GUARD RESULT: PASS (${#paths[@]} paths checked)"
exit 0
