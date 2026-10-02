#!/usr/bin/env bash
# =============================================================================
# scripts/verify-mining-sync-gate.sh
#
# 验收脚本：MINING-SYNC-GATE-1（自动矿工同步门）+ ON-DEMAND-MINING-REMOVAL-1
#          （按需出块全量下线）
#
# 按四项清单逐条核验并打印证据：
#   [1] 同步门代码三处：peerHeights 的**写入 / 过期 / 判定**
#   [2] /mine 残留引用：全仓库 grep（区分「必须为零」与「有意保留」）
#   [3] internal/ 零改动断言：共识 / 存储 / P2P 面相对阶段前基线零改动
#   [4] 测试全绿：专项测试（快）+ 可选 canonical 全量回归（慢）
#
# 用法：
#   bash scripts/verify-mining-sync-gate.sh                 # [1][2][3] + 专项测试
#   bash scripts/verify-mining-sync-gate.sh --full          # 追加 canonical 全量回归
#   bash scripts/verify-mining-sync-gate.sh <baseline-ref>  # 指定阶段前基线（默认见下）
#   bash scripts/verify-mining-sync-gate.sh --help
#
# 退出码：0 = 全部 PASS；1 = 至少一项 FAIL
# 本脚本**只读**：不修改任何文件、不提交、不推送、不启停节点。
# =============================================================================
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# 阶段前基线（本阶段第一个提交的父提交）。可用第一个参数覆盖。
DEFAULT_BASELINE="4676217"
BASELINE=""
WITH_FULL=0

for arg in "$@"; do
  case "$arg" in
    --help|-h)
      sed -n '2,25p' "$0" | sed 's/^# \{0,1\}//'
      exit 0 ;;
    --full) WITH_FULL=1 ;;
    -*) echo "未知参数: $arg（见 --help）" >&2; exit 2 ;;
    *) BASELINE="$arg" ;;
  esac
done
[ -n "$BASELINE" ] || BASELINE="$DEFAULT_BASELINE"

PASS=0
FAIL=0
pass() { printf '  \033[32mPASS\033[0m %s\n' "$*"; PASS=$((PASS+1)); }
bad()  { printf '  \033[31mFAIL\033[0m %s\n' "$*"; FAIL=$((FAIL+1)); }
hdr()  { printf '\n\033[1m== %s\033[0m\n' "$*"; }
info() { printf '       %s\n' "$*"; }

# 定位某函数定义的首行行号（用于打印「三处」并附行号）。
line_of() { # <file> <regex>
  grep -nE "$2" "$1" | head -1 | cut -d: -f1
}
show() { # <file> <from> <to>
  awk -v f="$2" -v t="$3" 'NR>=f && NR<=t {printf "%6d| %s\n", NR, $0}' "$1"
}

echo "P2PChain — MINING-SYNC-GATE-1 / ON-DEMAND-MINING-REMOVAL-1 验收"
echo "HEAD     : $(git rev-parse --short HEAD 2>/dev/null)"
echo "基线     : $BASELINE"
echo "工作树   : $ROOT"

# =============================================================================
hdr "[1] 同步门代码三处（peerHeights 写入 / 过期 / 判定）"
# =============================================================================
SVC="cmd/node/service.go"
MAIN="cmd/node/main.go"

if [ ! -f "$SVC" ] || [ ! -f "$MAIN" ]; then
  bad "找不到 $SVC 或 $MAIN"
else
  # ---- (1) 写入 ----
  W_FN="$(line_of "$SVC" '^func \(s \*nodeService\) recordPeerHeight')"
  W_CALL="$(line_of "$SVC" 's\.recordPeerHeight\(peerAddr, payload\.ChainHeight, payload\.ChainWork\)')"
  if [ -n "$W_FN" ] && [ -n "$W_CALL" ]; then
    pass "写入点存在：recordPeerHeight 定义 @ service.go:$W_FN，OnHandshake 调用 @ service.go:$W_CALL"
    info "OnHandshake 调用点上下文（应为第 (0) 步，纯观测、在既有三步之前）："
    show "$SVC" $((W_CALL-4)) $((W_CALL+1))
    info "写入实现（受 s.mu 保护 + nil map 惰性初始化 + 记 at=time.Now()）："
    show "$SVC" "$W_FN" $((W_FN+9))
  else
    bad "未找到写入点（recordPeerHeight 定义=$W_FN 调用=$W_CALL）"
  fi

  # ---- (2) 过期 ----
  E_LINE="$(line_of "$SVC" 'now\.Sub\(e\.at\) > miningSyncPeerTTL')"
  E_TTL="$(line_of "$SVC" 'miningSyncPeerTTL = 10 \* time\.Minute')"
  if [ -n "$E_LINE" ] && [ -n "$E_TTL" ]; then
    pass "过期判定存在：TTL 常量 @ service.go:$E_TTL，过期跳过 @ service.go:$E_LINE"
    show "$SVC" "$E_LINE" $((E_LINE+2))
  else
    bad "未找到 TTL 过期判定（常量=$E_TTL 判定=$E_LINE）"
  fi

  # ---- (3) 判定 ----
  D_FN="$(line_of "$SVC" '^func miningSyncGateDecision')"
  R_FN="$(line_of "$SVC" '^func \(s \*nodeService\) miningSyncReady')"
  G_LINE="$(line_of "$MAIN" 'if ready, local, peerMax := svc\.miningSyncReady\(\); !ready')"
  if [ -n "$D_FN" ] && [ -n "$R_FN" ] && [ -n "$G_LINE" ]; then
    pass "判定链路存在：纯函数 @ service.go:$D_FN，装配层 @ service.go:$R_FN，闸门插入 @ main.go:$G_LINE"
    info "纯函数内核（TTL 跳过 / work-priority 复用 / ready 公式）："
    show "$SVC" "$D_FN" $((D_FN+13))
    info "闸门插入点（未就绪 ⇒ WAITING_SYNC + 日志 + stop-aware 等待 + continue）："
    show "$MAIN" $((G_LINE-2)) $((G_LINE+15))
  else
    bad "判定链路不完整（纯函数=$D_FN 装配=$R_FN 闸门=$G_LINE）"
  fi

  # ---- 关键不变量：闸门必须在 mineOnce 之前 ----
  M_LINE="$(line_of "$MAIN" 'switch mineOnce\(svc, stop\)')"
  if [ -n "$G_LINE" ] && [ -n "$M_LINE" ] && [ "$G_LINE" -lt "$M_LINE" ]; then
    pass "闸门位于 mineOnce 之前（main.go:$G_LINE < $M_LINE）—— 未就绪时不会执行任何 PoW"
  else
    bad "闸门未位于 mineOnce 之前（闸门=$G_LINE mineOnce=$M_LINE）"
  fi
fi

# =============================================================================
hdr "[2] /mine 残留引用（全仓库 grep，排除 dist/ 与 reports-archive/）"
# =============================================================================
GREP_EXCL=(--exclude-dir=dist --exclude-dir=reports-archive --exclude-dir=.git
           --exclude-dir=.workbuddy --exclude-dir=.workbuddy-ai
           --exclude-dir=build --exclude-dir=release)

# (2a) 控制面路由：只允许 /mine/start、/mine/stop
ROUTES="$(grep -rn 'HandleFunc("/mine' internal/control/*.go 2>/dev/null)"
if printf '%s\n' "$ROUTES" | grep -q '/mine/start' \
   && printf '%s\n' "$ROUTES" | grep -q '/mine/stop' \
   && ! printf '%s\n' "$ROUTES" | grep -qE 'HandleFunc\("/mine"'; then
  pass "控制面路由仅 /mine/start 与 /mine/stop（无裸 /mine）"
  printf '%s\n' "$ROUTES" | sed 's/^/       /'
else
  bad "控制面路由异常：$ROUTES"
fi

# (2b) 精确字面量 "/mine"：命中必须全部是负例测试或注释
LIT="$(grep -rn '"/mine"' "${GREP_EXCL[@]}" --include='*.go' cmd/ internal/ 2>/dev/null)"
if [ -z "$LIT" ]; then
  pass "Go 源码中精确字面量 \"/mine\"：0 命中"
else
  # 逐行判定：测试文件 / 注释行 视为合法（负例断言与说明）
  ILLEGAL="$(printf '%s\n' "$LIT" | grep -vE '(_test\.go|^[^:]+:[0-9]+:\s*//)' || true)"
  if [ -z "$ILLEGAL" ]; then
    pass "精确字面量 \"/mine\" 命中 $(printf '%s\n' "$LIT" | wc -l | tr -d ' ') 处，全部为负例测试/注释（合法）"
    printf '%s\n' "$LIT" | sed 's/^/       /'
  else
    bad "发现非测试/非注释的 \"/mine\" 引用："; printf '%s\n' "$ILLEGAL" | sed 's/^/       /'
  fi
fi

# (2c) CLI 子命令表：不得再注册 "mine"
CLI="$(grep -rn '"mine"' "${GREP_EXCL[@]}" --include='*.go' cmd/node/ 2>/dev/null | grep -v '_test\.go' || true)"
if [ -z "$CLI" ]; then
  pass "CLI 源码中无 \"mine\" 子命令注册"
else
  # -mine 是启动开关（fs.BoolVar），合法；其余非法
  ILLEGAL="$(printf '%s\n' "$CLI" | grep -v 'fs.BoolVar' || true)"
  if [ -z "$ILLEGAL" ]; then
    pass "CLI 中 \"mine\" 仅作为 -mine 启动开关出现（合法）"
  else
    bad "CLI 中仍有 \"mine\" 子命令注册："; printf '%s\n' "$ILLEGAL" | sed 's/^/       /'
  fi
fi

# (2d) Console 页面：禁用字符串逐条扫描
BANNED=(
  'fetch(API + "/mine"'
  'fetch(API + "/console/mine"'
  'id="mineBtn"'
  'id="mineBtnText"'
  '可用：POST /console/mine'
  '可用：POST /mine count=1'
  'hOnDemand'
)
CH="internal/control/web/console.html"
CH_BAD=0
for s in "${BANNED[@]}"; do
  n="$(grep -cF -- "$s" "$CH" 2>/dev/null || true)"
  [ -z "$n" ] && n=0
  if [ "$n" != "0" ]; then CH_BAD=1; info "命中（应删）: $s -> $n"; fi
done
if [ "$CH_BAD" = "0" ]; then
  pass "Console 页面 ${#BANNED[@]} 条禁用字符串全部为 0 命中"
else
  bad "Console 页面仍残留已下线功能的痕迹"
fi

# (2e) 页面只读锁：整页只允许 1 处 fetch，且无 method: 声明
N_FETCH="$(grep -c 'fetch(' "$CH" 2>/dev/null || true)"
if [ "$N_FETCH" = "1" ] && ! grep -qE '\bmethod[[:space:]]*:' "$CH"; then
  pass "Console 页面只读：仅 1 处 fetch（jget 的读取超时包装），无 method: 声明"
else
  bad "Console 页面只读性被破坏（fetch 数=$N_FETCH，method: 命中=$(grep -cE '\bmethod[[:space:]]*:' "$CH" || true)）"
fi

# =============================================================================
hdr "[3] internal/ 零改动断言（相对阶段前基线 $BASELINE）"
# =============================================================================
if ! git rev-parse --verify --quiet "$BASELINE" >/dev/null; then
  bad "基线 $BASELINE 不存在（用 git log --oneline 找一个阶段前提交并作为参数传入）"
else
  ALL_INT="$(git diff --name-only "$BASELINE" -- internal/ 2>/dev/null)"
  OFF_CONTROL="$(printf '%s\n' "$ALL_INT" | grep -v '^internal/control/' | grep -v '^$' || true)"
  if [ -z "$OFF_CONTROL" ]; then
    pass "internal/ 改动全部落在 internal/control/（控制面）"
    printf '%s\n' "$ALL_INT" | sed 's/^/       /'
  else
    bad "internal/ 出现控制面以外的改动（违反铁律）："; printf '%s\n' "$OFF_CONTROL" | sed 's/^/       /'
  fi

  # 铁律明确点名的包：必须逐字节零改动
  for pkg in blockchain pow utxo storage p2p blocktree mempool transaction block wallet config; do
    hits="$(git diff --name-only "$BASELINE" -- "internal/$pkg/" 2>/dev/null)"
    if [ -z "$hits" ]; then
      pass "internal/$pkg/ 零改动"
    else
      bad "internal/$pkg/ 有改动（违反铁律）："; printf '%s\n' "$hits" | sed 's/^/       /'
    fi
  done

  # 存储字节格式：*.dat 读写路径所在包 + 序列化规范文档
  if git diff --name-only "$BASELINE" -- internal/storage/ docs/DETERMINISTIC-SERIALIZATION-SPEC.md 2>/dev/null | grep -q .; then
    bad "存储/序列化面有改动（违反铁律）"
  else
    pass "存储字节格式面（internal/storage + 序列化规范）零改动"
  fi
fi

# =============================================================================
hdr "[4] 测试"
# =============================================================================
echo "  --- 专项测试（同步门 + 下线回归锁）---"
SPECIAL='TestMiningSyncGate|TestRecordPeerHeightLazyInit|TestMiningSyncReadyWiring|TestOnDemandMineEndpointsRemoved|TestConsolePageHasNoMiningButton|TestMineEndpointRemoved|TestMineContractRemoved|TestCLIMineSubcommandRemoved|TestConsoleReadRequestsHaveTimeout'
go test ./cmd/node/ ./internal/control/ -run "$SPECIAL" -count=1 -timeout 5m > /tmp/verify-special.log 2>&1
SPECIAL_RC=$?
tail -4 /tmp/verify-special.log | sed 's/^/       /'
if [ "$SPECIAL_RC" = "0" ] && ! grep -qE '^(FAIL|--- FAIL)' /tmp/verify-special.log; then
  pass "专项测试全绿（同步门 5 用例 + 下线回归锁 5 用例 + Console 只读锁）"
else
  bad "专项测试存在失败（exit=$SPECIAL_RC）"
fi

echo "  --- 构建与静态检查 ---"
BUILD_OUT="$(go build ./... 2>&1)"; BUILD_RC=$?
if [ "$BUILD_RC" = "0" ]; then
  pass "go build ./... exit 0"
else
  bad "go build 失败"; printf '%s\n' "$BUILD_OUT" | head -10 | sed 's/^/       /'
fi
VET_OUT="$(go vet ./... 2>&1)"; VET_RC=$?
if [ "$VET_RC" = "0" ]; then
  pass "go vet ./... exit 0"
else
  bad "go vet 失败"; printf '%s\n' "$VET_OUT" | head -10 | sed 's/^/       /'
fi

if [ "$WITH_FULL" = "1" ]; then
  echo "  --- canonical 全量回归（scripts/run-tests.sh，约 13 分钟）---"
  bash scripts/run-tests.sh > /tmp/verify-full.log 2>&1
  FULL_RC=$?
  tail -20 /tmp/verify-full.log | sed 's/^/       /'
  if [ "$FULL_RC" = "0" ] && ! grep -qE '^(FAIL|--- FAIL)' /tmp/verify-full.log; then
    pass "全量回归 0 FAIL（exit 0）"
  else
    bad "全量回归存在失败（exit=$FULL_RC）"
  fi
else
  info "跳过 canonical 全量回归（加 --full 执行；约 13 分钟）"
fi

# =============================================================================
hdr "结果"
printf '  PASS %d 项，FAIL %d 项\n' "$PASS" "$FAIL"
if [ "$FAIL" -gt 0 ]; then
  echo "  验收未通过。"
  exit 1
fi
echo "  全部通过。"
