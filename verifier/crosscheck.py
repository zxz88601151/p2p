#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PHASE V2.0 -- cross-language comparison harness (NOT part of the verifier).

`p2pchain_verify.py` is the independent verifier and never touches Go.
THIS file is a separate comparison harness: it runs the Go node's
`printchain` (read-only) and the Python verifier, then diffs the results
block by block.  It exists only to produce the §8 comparison table.

Usage:
    python crosscheck.py <datadir> [--repo <p2pchain repo root>]
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import p2pchain_verify as pv  # noqa: E402


GO_BLOCK_RE = re.compile(
    r"区块 #(?P<h>\d+)\s*\n"
    r"\s*哈希\s*:\s*(?P<hash>[0-9a-f]{64})\s*\n"
    r"\s*前块哈希\s*:\s*(?P<prev>[0-9a-f]{64})\s*\n"
    r"\s*Merkle 根\s*:\s*(?P<merkle>[0-9a-f]{64})\s*\n"
    r"\s*时间戳\s*:\s*(?P<ts>-?\d+)\s*\n"
    r"\s*难度位\s*:\s*(?P<bits>\d+)\s*\n"
    r"\s*Nonce\s*:\s*(?P<nonce>\d+)\s*\n"
    r"\s*交易数\s*:\s*(?P<ntx>\d+)"
)


def go_printchain(repo: str, datadir: str) -> dict[int, dict]:
    env = dict(os.environ)
    env.setdefault("GOPATH", r"C:/Users/Administrator/.workbuddy/binaries/go/gopath")
    go = r"C:\Users\Administrator\.workbuddy\binaries\go\go\bin\go.exe"
    if not os.path.exists(go):
        go = "go"
    proc = subprocess.run(
        [go, "run", "./cmd/node", "printchain", "-datadir", datadir],
        cwd=repo,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
        env=env,
        timeout=300,
    )
    if proc.returncode != 0:
        raise RuntimeError("go printchain failed: %s%s" % (proc.stdout, proc.stderr))
    out = {}
    for m in GO_BLOCK_RE.finditer(proc.stdout):
        out[int(m.group("h"))] = {
            "hash": m.group("hash"),
            "prev": m.group("prev"),
            "merkle": m.group("merkle"),
            "ts": int(m.group("ts")),
            "bits": int(m.group("bits")),
            "nonce": int(m.group("nonce")),
            "ntx": int(m.group("ntx")),
        }
    return out


def python_blocks(datadir: str) -> dict[int, dict]:
    path = datadir if os.path.isfile(datadir) else os.path.join(datadir, "blocks.dat")
    records = pv.parse_blocks_dat(path)
    blocks = {}
    for height, raw in enumerate(records):
        blk = pv.decode_block(raw)
        h = blk.header
        blocks[height] = {
            "hash": h.hash_hex(),
            "prev": h.prev_hash.hex(),
            "merkle": h.merkle_root.hex(),
            "ts": h.timestamp,
            "bits": h.bits,
            "nonce": h.nonce,
            "ntx": len(blk.transactions),
        }
    return blocks


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("datadir")
    ap.add_argument("--repo", default=os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args()

    go = go_printchain(args.repo, args.datadir)
    py = python_blocks(args.datadir)

    rows = []
    mismatches = 0
    for height in sorted(set(go) | set(py)):
        g, p = go.get(height), py.get(height)
        if g is None or p is None:
            mismatches += 1
            rows.append({"height": height, "result": "MISSING"})
            continue
        same = all(g[k] == p[k] for k in ("hash", "prev", "merkle", "ts", "bits", "nonce", "ntx"))
        if not same:
            mismatches += 1
        rows.append(
            {
                "height": height,
                "go_hash": g["hash"],
                "py_hash": p["hash"],
                "go_merkle": g["merkle"],
                "py_merkle": p["merkle"],
                "go_bits": g["bits"],
                "py_bits": p["bits"],
                "go_nonce": g["nonce"],
                "py_nonce": p["nonce"],
                "go_ntx": g["ntx"],
                "py_ntx": p["ntx"],
                "result": "IDENTICAL" if same else "DIFFERENT",
            }
        )

    summary = {
        "go_blocks": len(go),
        "py_blocks": len(py),
        "compared": len(rows),
        "identical": sum(1 for r in rows if r["result"] == "IDENTICAL"),
        "mismatches": mismatches,
        "rows": rows,
    }
    if args.json:
        print(json.dumps(summary, ensure_ascii=False, indent=2))
    else:
        print("Go 区块数 = %d / Python 区块数 = %d / 比对 %d 块"
              % (summary["go_blocks"], summary["py_blocks"], summary["compared"]))
        print("逐块一致 = %d / 不一致 = %d" % (summary["identical"], summary["mismatches"]))
        for r in rows:
            if r["result"] != "IDENTICAL":
                print("  MISMATCH @%d: %s" % (r["height"], r))
    return 0 if mismatches == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
