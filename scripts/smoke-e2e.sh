#!/usr/bin/env bash
# 端到端冒烟测试：启动两个真实节点进程，通过 P2P 互联，完成
# 「出块 → 同步 → 转账 → 打包 → 双方余额核对」全流程。
#
# 与 `go test ./cmd/node` 的区别：单元/集成测试在进程内组装节点；
# 本脚本跑的是**编译后的真实二进制**与**两个独立进程**，覆盖
# 进程启动、命令行参数、控制接口、P2P 互联、磁盘持久化等只有真跑才暴露的问题。
#
# 数据目录契约（F-2 / F-3B）：全新目录**不会**被普通 node 启动路径隐式初始化，
# 必须先显式 `node init -datadir <dir>` 创建 canonical Genesis，再 `node start`。
# 本脚本因此对 A/B 两个数据目录各执行一次 init（见下）。
#
# mutation 端点契约（CONTROL-AUTH-1）：`/send /mine/start /mine/stop /stop` 需要
# Bearer Token，本脚本生成一个临时测试 token 文件并以绝对路径传给节点与 CLI（见下）。
#
# 出块路径（ON-DEMAND-MINING-REMOVAL-1）：按需出块（CLI `node mine`、POST /mine、
# POST /console/mine）已**全量下线**。本脚本改用两条现存路径出块：
#   ① 节点 A 以 `-mine -maxblocks 12` 启动：挖满 12 块后自动转入全节点模式，
#      高度可确定地停在 12（这是下线后唯一能「精确出 N 块」的路径）；
#   ② 转账后 `POST /mine/start` → 等交易上链 → `POST /mine/stop` 打包该交易。
# 注意 ② 的出块数量**不可精确控制**：pre-activation 难度钉死 bits=16，实测出块
# 速率约 76 块/秒，停止请求的往返延迟即产生数十块溢出。故 ② 之后的断言一律使用
# **实测高度**（相对断言），不再假定固定高度。
#
# 钱包口令契约（P0-4）：节点启动**必须**提供 0600 口令文件（-wallet-password-file），
# 缺失即 fail-closed 拒绝启动。本脚本在工作目录内生成临时口令文件（见下）。
#
# 用法：bash scripts/smoke-e2e.sh [工作目录]
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="${1:-$(mktemp -d)}"
# 仅当工作目录由本脚本自动创建（未显式传入参数）时，收工才删除它，
# 保证不产生永久测试数据；显式传入的目录视为调用方所有，保留以便排查。
if [ -z "${1:-}" ]; then AUTO_WORK=1; else AUTO_WORK=0; fi
BIN="$WORK/p2pchain-node"
A_DIR="$WORK/node-a"
B_DIR="$WORK/node-b"
# mutation 端点（/send /mine/start /mine/stop /stop）自 CONTROL-AUTH-1 起要求 Bearer Token。
# 本脚本使用**本临时工作目录内**的测试 token（绝不复用任何真实凭据），
# 并以绝对路径同时传给节点（-auth-token-file）与 CLI（-token-file），
# 从而不依赖工作目录下的 secrets/control-token 相对路径。
TOKEN_FILE="$WORK/control-token"
# 钱包口令文件（P0-4）：节点启动必填，缺失即 fail-closed。仅测试凭据，值不打印。
WALLET_PW_FILE="$WORK/wallet-password"

A_P2P=127.0.0.1:16688
A_RPC=127.0.0.1:16689
B_P2P=127.0.0.1:16690
B_RPC=127.0.0.1:16691

PASS=0
FAIL=0
NODE_A_PID=""   # 兜底强杀用；正常路径经 node stop 优雅停止，故通常保持为空
NODE_B_PID=""

log()  { printf '\n\033[1m== %s\033[0m\n' "$*"; }
ok()   { printf '  \033[32mPASS\033[0m %s\n' "$*"; PASS=$((PASS+1)); }
bad()  { printf '  \033[31mFAIL\033[0m %s\n' "$*"; FAIL=$((FAIL+1)); }

expect_contains() { # <描述> <期望子串> <实际文本>
  if printf '%s' "$3" | grep -qF -- "$2"; then ok "$1"; else
    bad "$1（期望包含: $2）"; printf '       实际输出:\n%s\n' "$3" | sed 's/^/       /'
  fi
}

# 优雅停止节点：走产品自身的 `node stop`（与 SIGINT 相同的关闭链，节点自行
# 释放数据目录锁），而不是 taskkill /F。原因：
#   1. 更符合当前产品契约（cmdStop 就是为脚本化「正常停止」设计的）；
#   2. Windows 上节点以**写句柄**持有 <datadir>/node.lock，MSYS 的 sed/cat
#      读该文件会得到 EBUSY（Device or resource busy），因此不能靠读 node.lock
#      取原生 PID；而 bash 的 $! 在 MSYS 下是伪 PID，taskkill 也不认。
# 停止是否成功以「控制接口不再响应」为准（见 wait_rpc_down），不以请求是否发出为准。
stop_node() { # <rpc地址>
  "$BIN" stop -rpc "$1" -token-file "$TOKEN_FILE" 2>&1
}

force_kill() { # <原生 pid>（兜底强杀；PID 未知时为空操作）
  [ -z "${1:-}" ] && return 0
  if command -v taskkill >/dev/null 2>&1; then
    MSYS_NO_PATHCONV=1 taskkill /F /PID "$1" >/dev/null 2>&1 || true
  else
    kill -9 "$1" 2>/dev/null || true
  fi
}

cleanup() {
  # 优先优雅停止（节点自行释放数据目录锁）；节点已退出时该调用无害失败。
  if [ -x "${BIN:-}" ]; then
    "$BIN" stop -rpc "$A_RPC" -token-file "$TOKEN_FILE" >/dev/null 2>&1 || true
    "$BIN" stop -rpc "$B_RPC" -token-file "$TOKEN_FILE" >/dev/null 2>&1 || true
  fi
  force_kill "$NODE_A_PID"
  force_kill "$NODE_B_PID"
  # 自动创建的工作目录（含二进制、日志、临时链数据）在收工后删除，保证
  # 不产生永久测试数据；显式传入的目录保留给调用方排查。trap 覆盖成功与失败
  # 两条路径，故失败时同样清理。用 POSIX 规范化路径，规避环境的安全删除垫片
  # 把 Windows 路径判为非法而 fail-closed。
  if [ "${AUTO_WORK:-0}" = "1" ] && [ -n "${WORK:-}" ]; then
    local w
    w="$(cygpath -u "$WORK" 2>/dev/null || printf '%s' "$WORK")"
    [ -n "$w" ] && rm -rf "$w" 2>/dev/null || true
  fi
  return 0
}

# 轮询直到 RPC 不再响应（证明进程确实已终止，而非「断言答的是旧进程」）
wait_rpc_down() { # <rpc地址>
  for _ in $(seq 1 30); do
    "$BIN" status -rpc "$1" >/dev/null 2>&1 || return 0
    sleep 0.2
  done
  return 1
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

# 调用挖矿生命周期端点（POST /mine/start | /mine/stop）。按需出块已全量下线，
# 这是脚本侧唯一可用的程序化出块入口（等价于 `-mine` 启动开关）。
mine_rpc() { # <rpc地址> <start|stop>
  curl -fsS -X POST \
    -H "Authorization: Bearer $(cat "$TOKEN_FILE")" \
    "http://$1/mine/$2" >/dev/null 2>&1
}

# 持续挖矿直到链上出现指定交易（或超时），随后停止挖矿。
#
# 为什么以「交易上链」而非「达到高度 N」为终止条件：pre-activation 难度钉死
# bits=16，实测出块速率约 76 块/秒，而停止请求的往返本身就要数十至数百毫秒，
# 无法「挖到高度 N 后精确停手」。但 /mine/start 后的**第一个**区块必然包含当时
# 交易池中的待打包交易，故以交易上链作为终止条件是确定性的；溢出块数由调用方
# 读取实测高度后做相对断言。
mine_until_tx() { # <rpc地址> <datadir> <txid>
  local rpc="$1" dir="$2" txid="$3" found=1
  mine_rpc "$rpc" start || return 1
  for _ in $(seq 1 300); do
    if "$BIN" printchain -datadir "$dir" -limit 200 -tx 2>/dev/null | grep -qF -- "$txid"; then
      found=0; break
    fi
    sleep 0.2
  done
  mine_rpc "$rpc" stop || true
  # 等挖矿循环收尾（STOPPING → STOPPED），使高度稳定后再返回，避免后续读到半途值。
  for _ in $(seq 1 50); do
    "$BIN" status -rpc "$rpc" 2>/dev/null | grep -q '挖矿状态: 已停止' && break
    sleep 0.2
  done
  return "$found"
}

log "构建二进制"
( cd "$ROOT" && go build -o "$BIN" ./cmd/node ) || { echo "构建失败"; exit 1; }
echo "  二进制: $BIN"
echo "  工作目录: $WORK"

log "生成测试用 mutation token 与钱包口令文件（仅限本临时工作目录）"
# 归一化后长度须落在 [16, 1024]；仅测试凭据，值不打印。
printf '%s\n' 'p2pchain-smoke-e2e-token-0123456789abcdef' > "$TOKEN_FILE" || { echo "写入 token 文件失败"; exit 1; }
# P0-4：节点启动必须提供 0600 钱包口令文件（-wallet-password-file），缺失 fail-closed。
printf '%s\n' 'p2pchain-smoke-e2e-wallet-password-0123456789' > "$WALLET_PW_FILE" || { echo "写入口令文件失败"; exit 1; }
# 0600 仅供非 Windows 平台满足 owner-only 校验；Windows 不表达权限位，失败可忽略。
chmod 600 "$TOKEN_FILE" 2>/dev/null || true
chmod 600 "$WALLET_PW_FILE" 2>/dev/null || true
echo "  token 文件: $TOKEN_FILE"
echo "  口令文件: $WALLET_PW_FILE"

log "初始化节点 A 数据目录（node init 创建 canonical Genesis）"
# 全新数据目录必须先 init：普通启动路径对空库 fail-closed（F-2 / F-3B）。
if "$BIN" init -datadir "$A_DIR" >"$WORK/init-a.log" 2>&1; then
  ok "节点 A 数据目录已初始化（canonical Genesis）"
else
  bad "节点 A 数据目录初始化失败"; sed 's/^/       /' "$WORK/init-a.log"; exit 1
fi

log "启动节点 A 并挖出 12 个区块（-mine -maxblocks 12；P2P $A_P2P / RPC $A_RPC）"
# 出块上限由 -maxblocks 精确约束：挖满 12 块后自动转入全节点模式（不再出块），
# 因此高度可确定地停在 12。这是按需出块下线后唯一能「精确出 N 块」的路径。
"$BIN" -datadir "$A_DIR" -listen "$A_P2P" -rpc "$A_RPC" -auth-token-file "$TOKEN_FILE" \
  -wallet-password-file "$WALLET_PW_FILE" -mine -maxblocks 12 >"$WORK/node-a.log" 2>&1 &
wait_rpc "$A_RPC" || { bad "节点 A 未就绪"; tail -20 "$WORK/node-a.log"; exit 1; }
ok "节点 A 已就绪（控制接口 $A_RPC）"

log "初始化节点 B 数据目录（node init 创建 canonical Genesis）"
if "$BIN" init -datadir "$B_DIR" >"$WORK/init-b.log" 2>&1; then
  ok "节点 B 数据目录已初始化（canonical Genesis）"
else
  bad "节点 B 数据目录初始化失败"; sed 's/^/       /' "$WORK/init-b.log"; exit 1
fi

log "启动节点 B 并指定种子节点 A（P2P $B_P2P / RPC $B_RPC）"
"$BIN" -datadir "$B_DIR" -listen "$B_P2P" -rpc "$B_RPC" -seed "$A_P2P" -auth-token-file "$TOKEN_FILE" \
  -wallet-password-file "$WALLET_PW_FILE" >"$WORK/node-b.log" 2>&1 &
wait_rpc "$B_RPC" || { bad "节点 B 未就绪"; tail -20 "$WORK/node-b.log"; exit 1; }
ok "节点 B 已就绪（控制接口 $B_RPC）"

log "两个节点应拥有相同创世区块"
HASH_A=$("$BIN" status -rpc "$A_RPC" | awk '/^链尾哈希/{print $NF}')
HASH_B=$("$BIN" status -rpc "$B_RPC" | awk '/^链尾哈希/{print $NF}')
if [ -n "$HASH_A" ] && [ "$HASH_A" = "$HASH_B" ]; then
  ok "创世区块哈希一致（$HASH_A）"
else
  bad "创世区块哈希不一致: A=$HASH_A B=$HASH_B"
fi

log "等待节点 A 挖满 12 块（使 3 个 coinbase 成熟：可花费余额 15 ≥ 转账 10 + 手续费 1）"
# 当前共识经济：Subsidy=5/块、CoinbaseMaturity=10 ⇒ 高度 12 时高度 1/2/3 的 coinbase 成熟（15）。
# 仅成熟 1 个（5）不足以支付 10+1 的转账，故出块数按当前经济参数取值。
if wait_height "$A_RPC" 12; then ok "节点 A 已达挖矿上限高度 12"; else
  bad "节点 A 未达高度 12"; "$BIN" status -rpc "$A_RPC" | sed 's/^/       /'
  echo "      ---- A 日志尾部 ----"; tail -15 "$WORK/node-a.log" | sed 's/^/      /'
fi
H_A12=$("$BIN" status -rpc "$A_RPC" | awk '/^高度/{print $NF}')
if [ "$H_A12" = "12" ]; then ok "节点 A 精确停在高度 12（-maxblocks 12 上限生效）"
else bad "节点 A 高度 = $H_A12, want 12（-maxblocks 上限未生效？）"; fi

log "等待节点 B 通过 P2P 同步到高度 12"
if wait_height "$B_RPC" 12; then ok "节点 B 已同步到高度 12"; else
  bad "节点 B 未同步到高度 12"; "$BIN" status -rpc "$B_RPC" | sed 's/^/       /'
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
SEND_OUT=$("$BIN" send -rpc "$A_RPC" -to "$ADDR_B" -amount 10 -fee 1 -token-file "$TOKEN_FILE" 2>&1)
echo "$SEND_OUT" | sed 's/^/  /'
TXID=$(printf '%s' "$SEND_OUT" | awk '/^交易 ID/{print $NF}')
if [ -n "$TXID" ]; then ok "转账已提交（txid=$TXID）"; else bad "转账未返回交易 ID"; fi

log "节点 A 持续挖矿以打包该交易（POST /mine/start → 交易上链 → POST /mine/stop）"
if mine_until_tx "$A_RPC" "$A_DIR" "$TXID"; then
  ok "交易已进入区块（A 链上出现该交易）"
else
  bad "交易未在时限内进入区块"; "$BIN" status -rpc "$A_RPC" | sed 's/^/       /'
fi

# 出块数量不可精确控制（见脚本头「出块路径」），故用 A 的实测高度做相对断言。
H_FINAL=$("$BIN" status -rpc "$A_RPC" | awk '/^高度/{print $NF}')
log "等待节点 B 同步到节点 A 的实测高度 $H_FINAL"
wait_height "$B_RPC" "$H_FINAL" && ok "节点 B 已同步到高度 $H_FINAL" || bad "节点 B 未同步到高度 $H_FINAL"

log "核对节点 B 地址余额（应精确等于 10）"
BAL_B=$("$BIN" balance -rpc "$B_RPC" -address "$ADDR_B"); echo "$BAL_B" | sed 's/^/  /'
RECV_B=$(printf '%s' "$BAL_B" | awk '/^可花费余额/{print $NF}')
if [ "$RECV_B" = "10" ]; then ok "收款方余额正确（10）"; else bad "收款方余额 = $RECV_B, want 10"; fi

log "核对交易已进入区块（离线只读链数据）"
# 交易必然落在 /mine/start 后的第一个区块；但溢出块数不确定，故扫描全部区块。
CHAIN_OUT=$("$BIN" printchain -datadir "$A_DIR" -limit 0 -tx 2>&1)
expect_contains "链上包含该交易" "$TXID" "$CHAIN_OUT"

log "优雅停止节点 B 后重启，验证持久化（高度与余额保持不变）"
# 用产品自身的 `node stop` 走与 SIGINT 相同的关闭链：节点自行释放数据目录锁。
# 这既符合当前产品契约，也避免读取被节点持有的 node.lock（Windows 上会 EBUSY）。
stop_node "$B_RPC" | sed 's/^/  /'
# 先证明旧进程真的死了，否则下面的「重启后仍有余额」只是在问旧进程，断言空转。
if wait_rpc_down "$B_RPC"; then
  ok "旧节点 B 已优雅停止（控制接口不再响应）"
else
  bad "旧节点 B 未被停止，重启持久化断言将无效"
fi
# 优雅停止会释放并删除 node.lock：文件消失即证明旧进程已释放数据目录锁。
if [ ! -e "$B_DIR/node.lock" ]; then
  ok "旧节点 B 已释放数据目录锁（node.lock 已删除）"
else
  bad "旧节点 B 未释放数据目录锁（node.lock 仍存在）"
fi
"$BIN" -datadir "$B_DIR" -listen "$B_P2P" -rpc "$B_RPC" -auth-token-file "$TOKEN_FILE" \
  -wallet-password-file "$WALLET_PW_FILE" >"$WORK/node-b2.log" 2>&1 &
wait_rpc "$B_RPC" || bad "重启后节点 B 未就绪"
# 重启成功即证明旧进程确已退出：否则新进程会因数据目录锁仍被占用而 fail-closed，
# 控制接口不会就绪。node.lock 重新出现则证明新进程重新获取了锁。
if [ -e "$B_DIR/node.lock" ]; then
  ok "重启后新进程重新获取数据目录锁（node.lock 已重建）"
else
  bad "重启后未重新获取数据目录锁（node.lock 缺失）"
fi
BAL_B2=$("$BIN" balance -rpc "$B_RPC" -address "$ADDR_B")
RECV_B2=$(printf '%s' "$BAL_B2" | awk '/^可花费余额/{print $NF}')
if [ "$RECV_B2" = "10" ]; then ok "重启后余额仍为 10"; else bad "重启后余额 = $RECV_B2, want 10"; fi
H_B2=$("$BIN" status -rpc "$B_RPC" | awk '/^高度/{print $NF}')
if [ "$H_B2" = "$H_FINAL" ]; then ok "重启后高度仍为 $H_FINAL"; else bad "重启后高度 = $H_B2, want $H_FINAL"; fi

log "结果"
printf '  通过 %d 项，失败 %d 项\n' "$PASS" "$FAIL"
# 自动创建的工作目录会被 cleanup 删除；显式传入的目录保留并打印路径。
if [ "${AUTO_WORK:-0}" != "1" ]; then echo "  日志目录: $WORK"; fi
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
echo "  全部通过。"
