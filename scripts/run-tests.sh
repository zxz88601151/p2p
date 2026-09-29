#!/usr/bin/env bash
# =============================================================================
# P2PChain — canonical 全量测试入口（LIM-4 F1：版本化 -timeout 契约）
# =============================================================================
#
# 为什么需要本脚本（背景见 docs/TEST-EXECUTION-CONTRACT.md）：
#
#   `go test` 的**每包默认超时是 10m0s**（Go 工具链默认值，不由本仓库定义）。
#   本仓库的 `cmd/node` 全量实测耗时 **583.5s ～ 605.3s**（真实 go build 子进程 +
#   真实 node 子进程 + 真实 RPC 挖矿 + 真实 TCP P2P + 轮询等待，138 个测试）。
#   ⇒ 默认值恰好压在边界上，在负载波动时以
#         panic: test timed out after 10m0s
#     把**正常但较慢**的测试误判为失败（已实测复现）。
#
#   本脚本把「显式 timeout」固化为仓库内的**可审计、可重复**契约：
#   全量测试不会因 Go 默认 600s 被错误判定为失败。
#
# 本脚本**不改变**测试内容：不加 -short、不跳过任何测试、不减覆盖率、
# 不 mock 真实挖矿/RPC/build、不触碰任何 production 代码。
# =============================================================================

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# ---- canonical test execution contract（改动需走治理门禁）--------------------
# TEST_TIMEOUT: 每包测试超时。**必须显式**。
#   依据：最慢包 cmd/node 实测上界 605.295s（Go 默认 600s 不足）；
#         25m = 1500s ≈ 2.48× 实测上界，可容忍约 2.4× 更慢的主机；
#         同时保持**有界**（不设 0/无限），真实挂起仍会在有限时间内失败。
TEST_TIMEOUT="${TEST_TIMEOUT:-25m}"
# TEST_COUNT: 固定为 1。Go 测试缓存会把「未变化」的包报成 (cached) 而**并不真正执行**，
#   使回归证据失真；-count=1 强制真实执行。
TEST_COUNT="${TEST_COUNT:-1}"
# TEST_PKGS: 测试范围。默认全仓；可用空格分隔的包列表覆盖。
TEST_PKGS="${TEST_PKGS:-./...}"
# ----------------------------------------------------------------------------

if [[ " $* " == *" -short "* ]]; then
  echo "[contract] ⚠️  检测到 -short：它会**跳过**真实子进程/真实挖矿类测试（降低覆盖率）。" >&2
  echo "[contract] ⚠️  -short 不是 canonical 路径，仅用于本地快速迭代；不得作为验收证据。" >&2
fi

# 分词是有意的：TEST_PKGS 允许是空格分隔的多个包。
# shellcheck disable=SC2206
pkgs=(${TEST_PKGS})

echo "[contract] go        : $(go version)"
echo "[contract] packages  : ${TEST_PKGS}"
echo "[contract] count     : ${TEST_COUNT}   (禁用测试缓存，强制真实执行)"
echo "[contract] timeout   : ${TEST_TIMEOUT}   (每包；Go 默认 10m 对本仓库不足)"
echo "[contract] command   : go test ${TEST_PKGS} -count=${TEST_COUNT} -timeout ${TEST_TIMEOUT} $*"
echo "[contract] contract  : docs/TEST-EXECUTION-CONTRACT.md"
echo

exec go test "${pkgs[@]}" -count="${TEST_COUNT}" -timeout="${TEST_TIMEOUT}" "$@"
