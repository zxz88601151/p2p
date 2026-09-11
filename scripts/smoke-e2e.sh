#!/usr/bin/env bash
# 端到端冒烟测试：启动两个真实节点进程，通过 P2P 互联，完成
# 「出块 → 同步 → 转账 → 打包 → 双方余额核对」全流程。
#
# 与 `go test ./cmd/node` 的区别：单元/集成测试在进程内组装节点；
# 本脚本跑的是**编译后的真实二进制**与**两个独立进程**，覆盖
# 进程启动、命令行参数、控制接口、P2P 互联、磁盘持久化等只有真跑才暴露的问题。
#
# 用法：bash scripts/smoke-e2e.sh [工作目录]
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="${1:-$(mktemp -d)}"
BIN="$WORK/p2pchain-node"
A_DIR="$WORK/node-a"
B_DIR="$WORK/node-b"

A_P2P=127.0.0.1:16688
A_RPC=127.0.0.1:16689
B_P2P=127.0.0.1:16690
B_RPC=127.0.0.1:16691

PASS=0
FAIL=0
NODE_A_PID=""
NODE_B_PID=""

log()  { printf '\n\033[1m== %s\033[0m\n' "$*"; }
ok()   { printf '  \033[32mPASS\033[0m %s\n' "$*"; PASS=$((PASS+1)); }
bad()  { printf '  \033[31mFAIL\033[0m %s\n' "$*"; FAIL=$((FAIL+1)); }

expect_contains() { # <描述> <期望子串> <实际文本>
  if printf '%s' "$3" | grep -qF -- "$2"; then ok "$1"; else
    bad "$1（期望包含: $2）"; printf '       实际输出:\n%s\n' "$3" | sed 's/^/       /'
  fi
}

# force_kill 在 Windows 上可靠终止原生 Go 进程：Git Bash 的 kill 对原生 .exe 发的是
# 非可捕获信号，进程不会真正退出；taskkill /F 调用 TerminateProcess 才是真杀。
force_kill() {
  [ -z "$1" ] && return 0
  if command -v taskkill >/dev/null 2>&1; then
    taskkill /F /PID "$1" >/dev/null 2>&1 || true
  else
    kill -9 "$1" 2>/dev/null || true
  fi
}

cleanup() {
  force_kill "$NODE_A_PID"
  force_kill "$NODE_B_PID"
  return 0
}
trap cleanup EXIT

# 等节点控制接口就绪（最多 ~10 秒）
wait_rpc() { # <rpc地址>
  for _ in $(seq 1 50); do
    if "$BIN" status -rpc "$1" >/dev/null 2>&1; then return 0; fi
    sleep 0.2
  done
  return 1
}

# 等待节点高度达到（或超过）目标值
wait_height() { # <rpc地址> <目标高度>
  for _ in $(seq 1 50); do
    h=$("$BIN" status -rpc "$1" 2>/dev/null | awk '/^高度/{print $NF}')
    [ -n "$h" ] && [ "$h" -ge "$2" ] 2>/dev/null && return 0
    sleep 0.2
  done
  return 1
}

log "构建二进制"
( cd "$ROOT" && go build -o "$BIN" ./cmd/node ) || { echo "构建失败"; exit 1; }
echo "  二进制: $BIN"
echo "  工作目录: $WORK"

log "启动节点 A（P2P $A_P2P / RPC $A_RPC）"
"$BIN" -datadir "$A_DIR" -listen "$A_P2P" -rpc "$A_RPC" >"$WORK/node-a.log" 2>&1 &
NODE_A_PID=$!
wait_rpc "$A_RPC" || { bad "节点 A 未就绪"; tail -20 "$WORK/node-a.log"; exit 1; }
ok "节点 A 已就绪（pid=$NODE_A_PID）"

log "启动节点 B 并指定种子节点 A（P2P $B_P2P / RPC $B_RPC）"
"$BIN" -datadir "$B_DIR" -listen "$B_P2P" -rpc "$B_RPC" -seed "$A_P2P" >"$WORK/node-b.log" 2>&1 &
NODE_B_PID=$!
wait_rpc "$B_RPC" || { bad "节点 B 未就绪"; tail -20 "$WORK/node-b.log"; exit 1; }
ok "节点 B 已就绪（pid=$NODE_B_PID）"

log "两个节点应拥有相同创世区块"
HASH_A=$("$BIN" status -rpc "$A_RPC" | awk '/^链尾哈希/{print $NF}')
HASH_B=$("$BIN" status -rpc "$B_RPC" | awk '/^链尾哈希/{print $NF}')
if [ -n "$HASH_A" ] && [ "$HASH_A" = "$HASH_B" ]; then
  ok "创世区块哈希一致（$HASH_A）"
else
  bad "创世区块哈希不一致: A=$HASH_A B=$HASH_B"
fi

log "节点 A 按需出块 11 个（使高度 1 的 coinbase 成熟）"
OUT=$("$BIN" mine -rpc "$A_RPC" -count 11 2>&1)
expect_contains "出块命令返回高度" "当前高度 11" "$OUT"

log "等待节点 B 通过 P2P 同步到高度 11"
if wait_height "$B_RPC" 11; then ok "节点 B 已同步到高度 11"; else
  bad "节点 B 未同步到高度 11"; "$BIN" status -rpc "$B_RPC" | sed 's/^/       /'
  echo "      ---- B 日志尾部 ----"; tail -15 "$WORK/node-b.log" | sed 's/^/      /'
fi

log "节点 A 余额与 UTXO"
BAL_A=$("$BIN" balance -rpc "$A_RPC"); echo "$BAL_A" | sed 's/^/  /'
SPENDABLE_A=$(printf '%s' "$BAL_A" | awk '/^可花费余额/{print $NF}')
if [ -n "$SPENDABLE_A" ] && [ "$SPENDABLE_A" -gt 0 ] 2>/dev/null; then
  ok "节点 A 有可花费余额（$SPENDABLE_A）"
else
  bad "节点 A 可花费余额为 0"
fi
UTXO_A=$("$BIN" utxos -rpc "$A_RPC"); expect_contains "UTXO 列表有表头" "OUTPOINT" "$UTXO_A"

ADDR_B=$("$BIN" wallet -datadir "$B_DIR" -address)
log "节点 B 钱包地址: $ADDR_B"

log "节点 A 向节点 B 转账 10（手续费 1）"
SEND_OUT=$("$BIN" send -rpc "$A_RPC" -to "$ADDR_B" -amount 10 -fee 1 2>&1)
echo "$SEND_OUT" | sed 's/^/  /'
TXID=$(printf '%s' "$SEND_OUT" | awk '/^交易 ID/{print $NF}')
if [ -n "$TXID" ]; then ok "转账已提交（txid=$TXID）"; else bad "转账未返回交易 ID"; fi

log "节点 A 出块 1 个以打包该交易"
"$BIN" mine -rpc "$A_RPC" -count 1 | sed 's/^/  /'

log "等待节点 B 同步打包后的区块"
wait_height "$B_RPC" 12 && ok "节点 B 已同步到高度 12" || bad "节点 B 未同步到高度 12"

log "核对节点 B 地址余额（应精确等于 10）"
BAL_B=$("$BIN" balance -rpc "$B_RPC" -address "$ADDR_B"); echo "$BAL_B" | sed 's/^/  /'
RECV_B=$(printf '%s' "$BAL_B" | awk '/^可花费余额/{print $NF}')
if [ "$RECV_B" = "10" ]; then ok "收款方余额正确（10）"; else bad "收款方余额 = $RECV_B, want 10"; fi

log "核对交易已进入区块（离线只读链数据）"
CHAIN_OUT=$("$BIN" printchain -datadir "$A_DIR" -limit 1 -tx 2>&1)
expect_contains "高度 12 区块包含该交易" "$TXID" "$CHAIN_OUT"

log "重启节点 B，验证持久化（高度与余额保持不变）"
force_kill "$NODE_B_PID"; sleep 1
# Windows 信号模型说明：taskkill /F 对原生 Go 进程是 TerminateProcess（不可捕获），不会触发
# 本节点的 SIGINT 优雅关闭，因此 node.lock 不会被进程自己释放；本工作目录为 mktemp 独占临时
# 目录，此处显式清理锁等价于「进程优雅退出后的释放」——节点锁语义（Close 释放 / 占用拒绝 /
# 内容不变）由 internal/storage 单元测试覆盖，不在此重复。
rm -f "$B_DIR/node.lock"
"$BIN" -datadir "$B_DIR" -listen "$B_P2P" -rpc "$B_RPC" >"$WORK/node-b2.log" 2>&1 &
NODE_B_PID=$!
wait_rpc "$B_RPC" || bad "重启后节点 B 未就绪"
BAL_B2=$("$BIN" balance -rpc "$B_RPC" -address "$ADDR_B")
RECV_B2=$(printf '%s' "$BAL_B2" | awk '/^可花费余额/{print $NF}')
if [ "$RECV_B2" = "10" ]; then ok "重启后余额仍为 10"; else bad "重启后余额 = $RECV_B2, want 10"; fi
H_B2=$("$BIN" status -rpc "$B_RPC" | awk '/^高度/{print $NF}')
if [ "$H_B2" = "12" ]; then ok "重启后高度仍为 12"; else bad "重启后高度 = $H_B2, want 12"; fi

log "结果"
printf '  通过 %d 项，失败 %d 项\n' "$PASS" "$FAIL"
if [ "$FAIL" -gt 0 ]; then
  echo "  日志目录: $WORK"
  exit 1
fi
echo "  全部通过。日志目录: $WORK"
