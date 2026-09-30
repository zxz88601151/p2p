#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PHASE V2.0 §9 -- negative cross-validation harness (NOT part of the verifier).

Copies the sample chain into throwaway temp directories, corrupts each copy in
a different way, and runs BOTH verifiers against every copy:

    Go      -> `node verify -datadir <dir>`
    Python  -> `p2pchain_verify.py <dir>`

The original sample chain is never modified.

Usage:  python verifier/negcheck.py [blocks.dat] [--repo <repo root>]
"""

from __future__ import annotations

import os
import shutil
import struct
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import p2pchain_verify as pv  # noqa: E402

CHAIN62 = r"C:/Users/Administrator/AppData/Local/Temp/0d3-diff/blocks.dat"
GO = r"C:\Users\Administrator\.workbuddy\binaries\go\go\bin\go.exe"


def go_verify(repo: str, datadir: str) -> tuple[int, str]:
    env = dict(os.environ)
    env.setdefault("GOPATH", r"C:/Users/Administrator/.workbuddy/binaries/go/gopath")
    proc = subprocess.run(
        [GO, "run", "./cmd/node", "verify", "-datadir", datadir],
        cwd=repo, capture_output=True, text=True,
        encoding="utf-8", errors="replace", env=env, timeout=600,
    )
    return proc.returncode, (proc.stdout + proc.stderr)


def py_verify(datadir: str) -> tuple[int, str]:
    proc = subprocess.run(
        [sys.executable, os.path.join(HERE, "p2pchain_verify.py"), datadir],
        capture_output=True, text=True, encoding="utf-8", errors="replace", timeout=600,
    )
    return proc.returncode, (proc.stdout + proc.stderr)


def write_records(path: str, recs: list[bytes]) -> None:
    with open(path, "wb") as f:
        for r in recs:
            f.write(struct.pack("<I", len(r)) + r)


def flip(path: str, offset: int) -> None:
    with open(path, "r+b") as f:
        f.seek(offset)
        b = f.read(1)
        f.seek(offset)
        f.write(bytes([b[0] ^ 0x01]))


def truncate(path: str, n: int) -> None:
    with open(path, "r+b") as f:
        f.truncate(os.path.getsize(path) - n)


def main() -> int:
    src = sys.argv[1] if len(sys.argv) > 1 else CHAIN62
    repo = os.path.dirname(HERE)
    if not os.path.isfile(src):
        print("sample chain not found: %s" % src, file=sys.stderr)
        return 2

    work = tempfile.mkdtemp(prefix="v20-negcheck-")
    cases: list[tuple[str, str]] = []      # (name, dir)

    def newdir(name: str) -> str:
        d = os.path.join(work, name)
        os.makedirs(d)
        shutil.copyfile(src, os.path.join(d, "blocks.dat"))
        cases.append((name, d))
        return d

    # baseline
    newdir("T0-clean")

    # N1 header corruption (inside block 0 header)
    d = newdir("N1-header")
    flip(os.path.join(d, "blocks.dat"), 4 + 40)

    # N2 truncation
    d = newdir("N2-truncated")
    truncate(os.path.join(d, "blocks.dat"), 16)

    # N3 transaction corruption (header untouched -> must be caught by Merkle)
    d = newdir("N3-tx")
    p = os.path.join(d, "blocks.dat")
    flip(p, os.path.getsize(p) - 10)

    # N4 linkage break (swap two adjacent middle records)
    d = newdir("N4-linkage")
    p = os.path.join(d, "blocks.dat")
    recs = pv.parse_blocks_dat(p)
    recs[30], recs[31] = recs[31], recs[30]
    write_records(p, recs)

    # N5 invalid chain state (duplicate last record)
    d = newdir("N5-duplicate")
    p = os.path.join(d, "blocks.dat")
    recs = pv.parse_blocks_dat(p)
    recs.append(recs[-1])
    write_records(p, recs)

    print("%-14s %-12s %-12s %s" % ("CASE", "GO", "PYTHON", "AGREE"))
    print("-" * 56)
    ok = True
    for name, d in cases:
        g_code, _ = go_verify(repo, d)
        p_code, _ = py_verify(d)
        g = "PASS" if g_code == 0 else ("USAGE" if g_code == 2 else "FAIL")
        p = "PASS" if p_code == 0 else ("USAGE" if p_code == 2 else "FAIL")
        agree = g == p
        expect = "PASS" if name == "T0-clean" else "FAIL"
        consistent = (g == expect) and (p == expect)
        if not (agree and consistent):
            ok = False
        print("%-14s %-12s %-12s %s" % (
            name, g, p, "YES" if agree else "NO",
        ), "" if consistent else "  <-- UNEXPECTED (expect %s)" % expect)
    print("-" * 56)
    print("workdir:", work)
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
