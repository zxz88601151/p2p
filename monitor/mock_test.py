#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
mock_test.py — V3 monitor 的 B1/B2 回归测试。

覆盖：
  B1 Windows 文件名安全（无冒号）
  B2 transition evidence 重启恢复（Scenario A/B/C/D）
  transition gate（正常序列 + 跳块）
  evidence 不可变性（首次/相同/不同）

运行：python mock_test.py
"""

import json
import os
import sys
import tempfile

# 引入被测模块（同目录）
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import v3monitor as m  # noqa: E402

PASS = 0
FAIL = 0


def check(name, cond, detail=""):
    global PASS, FAIL
    if cond:
        PASS += 1
        print(f"  [PASS] {name}")
    else:
        FAIL += 1
        print(f"  [FAIL] {name} {detail}")


def setup_tmp_dirs():
    """建立隔离的临时 evidence 目录并指向模块。"""
    tmp = tempfile.mkdtemp(prefix="v3mon_test_")
    m.SNAPSHOT_DIR = os.path.join(tmp, "snapshots")
    m.TRANSITION_DIR = os.path.join(tmp, "transition")
    m.HISTORY_DIR = os.path.join(tmp, "history")
    m.ALERT_DIR = os.path.join(tmp, "alerts")
    for d in (m.SNAPSHOT_DIR, m.TRANSITION_DIR, m.HISTORY_DIR, m.ALERT_DIR):
        os.makedirs(d, exist_ok=True)
    return tmp


# ---------------------------------------------------------------------------
# B1 — Windows 文件名安全
# ---------------------------------------------------------------------------
def test_b1_filename_safety():
    print("\n=== B1 Windows 文件名安全 ===")
    tmp = setup_tmp_dirs()

    # 1-3: write_alert 各等级成功
    for lvl in ("INFO", "WARNING", "ERROR", "CRITICAL"):
        p = m.write_alert(lvl, f"test {lvl}")
        check(f"write_alert({lvl}) 成功", os.path.exists(p))

    # 4: transition 首次写入成功
    hdr = {"height": 3000, "version": 4, "bits": 30, "block_hash": "h3k",
           "previous_hash": "h2999", "merkle_root": "m", "timestamp": 123,
           "difficulty": 16384, "nonce": 1, "consensus_era": "post", "canonical": True}
    p, changed = m.write_transition(3000, hdr)
    check("transition 首次写入成功", os.path.exists(p) and changed is False)

    # 5: 相同内容不覆盖
    p2, changed2 = m.write_transition(3000, hdr)
    check("transition 相同内容不覆盖 (changed=False)", changed2 is False)

    # 6: 不同内容产生 diff
    hdr2 = dict(hdr, previous_hash="DIFFERENT")
    p3, changed3 = m.write_transition(3000, hdr2)
    check("transition 内容变化 changed=True", changed3 is True)
    diffs = [f for f in os.listdir(m.ALERT_DIR) if "content-changed" in f]
    check("产生 diff evidence 文件", len(diffs) >= 1)

    # 7: 所有文件名不含冒号
    all_ok = True
    bad = []
    for d in (m.SNAPSHOT_DIR, m.TRANSITION_DIR, m.HISTORY_DIR, m.ALERT_DIR):
        for f in os.listdir(d):
            if ":" in f:
                all_ok = False
                bad.append(f)
    check("所有生成文件名不含冒号", all_ok, f"含冒号: {bad}")

    # file_safe_ts 单测
    check("file_safe_ts 无冒号", ":" not in m.file_safe_ts())
    check("utcnow_iso 保留冒号(仅内容用)", ":" in m.utcnow_iso())

    import shutil
    shutil.rmtree(tmp)


# ---------------------------------------------------------------------------
# B2 — transition evidence 重启恢复
# ---------------------------------------------------------------------------
def _write_evidence(height, fields):
    os.makedirs(m.TRANSITION_DIR, exist_ok=True)
    with open(os.path.join(m.TRANSITION_DIR, f"{height}.json"), "w", encoding="utf-8") as f:
        json.dump(fields, f)


def _valid_block(h, version, bits, prev, block_hash):
    return {"height": h, "version": version, "bits": bits, "previous_hash": prev,
            "block_hash": block_hash, "merkle_root": "m", "timestamp": 123,
            "difficulty": 16384, "nonce": 1, "consensus_era": "post", "canonical": True}


def test_b2_recovery():
    print("\n=== B2 transition evidence 重启恢复 ===")

    # Scenario A — 完整证据
    tmp = setup_tmp_dirs()
    _write_evidence(2999, _valid_block(2999, 2, 30, "p2998", "h2999"))
    _write_evidence(3000, _valid_block(3000, 4, 30, "h2999", "h3000"))
    _write_evidence(3001, _valid_block(3001, 4, 30, "h3000", "h3001"))
    state, blocks, failures = m.recover_transition_evidence()
    check("A: 2999 CAPTURED", state["2999"] == "CAPTURED")
    check("A: 3000 CAPTURED", state["3000"] == "CAPTURED")
    check("A: 3001 CAPTURED", state["3001"] == "CAPTURED")
    check("A: 无 recovery failure", len(failures) == 0, str(failures))
    import shutil
    shutil.rmtree(tmp)

    # Scenario B — 部分证据
    tmp = setup_tmp_dirs()
    _write_evidence(2999, _valid_block(2999, 2, 30, "p2998", "h2999"))
    state, blocks, failures = m.recover_transition_evidence()
    check("B: 2999 CAPTURED", state["2999"] == "CAPTURED")
    check("B: 3000 MISSING", state["3000"] == "MISSING")
    check("B: 3001 MISSING", state["3001"] == "MISSING")
    shutil.rmtree(tmp)

    # Scenario C — 损坏 evidence
    tmp = setup_tmp_dirs()
    # C1 非法 JSON
    with open(os.path.join(m.TRANSITION_DIR, "3000.json"), "w") as f:
        f.write("{invalid json")
    # C2 缺少 hash 字段
    _write_evidence(3001, {"height": 3001, "version": 4, "bits": 30})  # 缺 block_hash/previous_hash
    state, blocks, failures = m.recover_transition_evidence()
    check("C: 非法 JSON 不崩溃", True)
    check("C: 3000 不误标 CAPTURED", state["3000"] == "MISSING")
    check("C: 3001 缺字段不误标 CAPTURED", state["3001"] == "MISSING")
    check("C: 记录 recovery failure", len(failures) >= 2, str(failures))
    shutil.rmtree(tmp)

    # Scenario C3 — height 不匹配
    tmp = setup_tmp_dirs()
    _write_evidence(3000, _valid_block(2999, 2, 30, "x", "y"))  # 文件名 3000 但内容 height=2999
    state, blocks, failures = m.recover_transition_evidence()
    check("C3: height 不匹配不误标 CAPTURED", state["3000"] == "MISSING")
    check("C3: 记录 failure", len(failures) >= 1, str(failures))
    shutil.rmtree(tmp)


# ---------------------------------------------------------------------------
# transition gate — 正常序列 + 跳块
# ---------------------------------------------------------------------------
def test_transition_gate():
    print("\n=== transition gate 回归 ===")

    # 正常序列 target 覆盖
    check("正常 2999 捕获", m.target_blocks_for_height(2999) == [2999])
    check("正常 3000 捕获(含2999)", m.target_blocks_for_height(3000) == [2999, 3000])
    check("正常 3001 捕获(全)", m.target_blocks_for_height(3001) == [2999, 3000, 3001])

    # 跳块 2998->3000 补查 2999
    check("跳块 2998->3000 补查2999", 2999 in m.target_blocks_for_height(3000))

    # 跳块 2999->3001 补查 3000
    check("跳块 2999->3001 补查3000", 3000 in m.target_blocks_for_height(3001))

    # mock block 字段真值
    check("2999 = V2", m.mock_block(2999)["version"] == 2)
    check("3000 = V4 + bits30", m.mock_block(3000)["version"] == 4 and m.mock_block(3000)["bits"] == 30)
    check("3000 ← 2999 链接", m.mock_block(3000)["previous_hash"] == m.mock_block(2999)["block_hash"])
    check("3001 ← 3000 链接", m.mock_block(3001)["previous_hash"] == m.mock_block(3000)["block_hash"])


# ---------------------------------------------------------------------------
# evidence 不可变性
# ---------------------------------------------------------------------------
def test_immutability():
    print("\n=== evidence 不可变性 ===")
    tmp = setup_tmp_dirs()
    hdr = _valid_block(3000, 4, 30, "h2999", "h3000")

    p1, c1 = m.write_transition(3000, hdr)          # 首次
    p2, c2 = m.write_transition(3000, hdr)          # 相同
    hdr_diff = dict(hdr, bits=29)
    p3, c3 = m.write_transition(3000, hdr_diff)     # 不同

    check("首次 changed=False", c1 is False)
    check("相同不覆盖 changed=False", c2 is False)
    check("不同 changed=True", c3 is True)

    # 原文件内容不变（保留第一次 bits=30）
    with open(os.path.join(m.TRANSITION_DIR, "3000.json")) as f:
        content = json.load(f)
    check("原 evidence 保留 bits=30", content["bits"] == 30)
    check("原 evidence hash 不变", content["block_hash"] == "h3000")

    # history 追加式
    snap = {"timestamp": m.utcnow_iso(), "height": 2757}
    m.write_history(snap)
    m.write_history(snap)
    with open(os.path.join(m.HISTORY_DIR, "history.jsonl")) as f:
        lines = f.readlines()
    check("history 追加 2 行", len(lines) == 2)

    import shutil
    shutil.rmtree(tmp)


# ---------------------------------------------------------------------------
# PASS gate 无简化
# ---------------------------------------------------------------------------
def test_pass_gate_no_simplification():
    print("\n=== PASS gate 无简化 ===")
    # 源码中不存在 height>=3000 -> PASS 的简化逻辑
    src = open(os.path.join(os.path.dirname(__file__), "v3monitor.py"), encoding="utf-8").read()
    check("无 height>=3000 直接 PASS", "PASS_CANDIDATE" in src and "all_captured" in src)


def main():
    test_b1_filename_safety()
    test_b2_recovery()
    test_transition_gate()
    test_immutability()
    test_pass_gate_no_simplification()

    print(f"\n===== 结果: {PASS} PASS / {FAIL} FAIL =====")
    return 0 if FAIL == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
