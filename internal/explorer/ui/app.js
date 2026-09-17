/* P2PChain Explorer V1 — vanilla JS（零框架、零 CDN）。
   安全铁律：所有来自 API 的动态字符串一律经 textContent 渲染，
   禁止 innerHTML 插入链数据（唯一 innerHTML 用途是静态骨架模板）。
   刷新铁律：全局唯一 2s 定时器 + in-flight 防重入 + 路由切换 abort，
   禁止重复定时器/跑飞请求/卸载后轮询（授权 §7）。 */
"use strict";

/* ---------------- utilities ---------------- */
const $app = document.getElementById("app");
const $netBadge = document.getElementById("net-badge");
const $poll = document.getElementById("poll-indicator");

function el(tag, attrs, ...children) {
  const n = document.createElement(tag);
  if (attrs) for (const [k, v] of Object.entries(attrs)) {
    if (k === "class") n.className = v;
    else if (k === "text") n.textContent = v;
    else if (k.startsWith("on")) n.addEventListener(k.slice(2), v);
    else n.setAttribute(k, v);
  }
  for (const c of children) if (c != null) n.append(c);
  return n;
}
function fmtTime(unix) {
  if (typeof unix !== "number") return "—";
  const d = new Date(unix * 1000);
  return isNaN(d) ? String(unix) : d.toLocaleString() + " (" + unix + ")";
}
function shortHash(h, head = 10, tail = 8) {
  return (h && h.length > head + tail + 1) ? h.slice(0, head) + "…" + h.slice(-tail) : (h || "—");
}
function dashIfUndef(v) { return v === undefined || v === null ? "—" : v; }
function hexBits(bits) { return typeof bits === "number" ? "0x" + bits.toString(16).padStart(8, "0") : "—"; }

/* ---------------- API layer ---------------- */
let consecutiveFail = 0, everSucceeded = false;

async function api(path, opts = {}, signal = null) {
  const res = await fetch("/api" + path, { signal, ...opts });
  if (!res.ok) {
    let body = "";
    try { body = await res.text(); } catch (_) { /* ignore */ }
    const err = new Error("HTTP " + res.status);
    err.status = res.status;
    err.body = body;
    throw err;
  }
  everSucceeded = true;
  consecutiveFail = 0;
  setNetBadge("ok", "API OK");
  return res.json();
}
function apiFail() {
  consecutiveFail++;
  setNetBadge(consecutiveFail >= 2 || !everSucceeded ? "bad" : "stale",
    consecutiveFail >= 2 || !everSucceeded ? "API UNAVAILABLE" : "API STALE");
}
function setNetBadge(cls, text) { $netBadge.className = "net-badge " + cls; $netBadge.textContent = text; }

/* ---------------- polling manager（全局唯一定时器） ---------------- */
const POLL_MS = 2000;
let pollTimer = null;
let currentTick = null;      // 当前路由的轮询函数（null = 不轮询）
let currentRouteKey = "";    // 路由键；变化即 abort 旧请求
let inflight = false;
let currentAbort = null;

function setTicker(routeKey, tick) {
  if (currentAbort) { try { currentAbort.abort(); } catch (_) {} }
  currentAbort = null;
  inflight = false;
  currentRouteKey = routeKey;
  currentTick = tick;
}
function ensureTimer() {
  if (pollTimer) return;
  pollTimer = setInterval(pollTick, POLL_MS);
}
async function pollTick() {
  if (!currentTick || inflight || document.hidden) return;
  inflight = true;
  currentAbort = new AbortController();
  try { await currentTick(currentAbort.signal); }
  finally {
    inflight = false;
    currentAbort = null;
  }
  $poll.textContent = "poll: 2s";
}

/* ---------------- banner / states ---------------- */
function errorBanner(msg, cls = "") {
  return el("div", { class: "banner " + cls, text: msg });
}
function emptyState(msg) { return el("div", { class: "empty", text: msg }); }
function loadingBlock() { return el("div", { class: "loading-block", text: "Loading…" }); }

/* ---------------- shared renderers ---------------- */
function blocksTable(blocks, opts = {}) {
  const tbody = el("tbody");
  for (const b of blocks) {
    tbody.append(el("tr", null,
      el("td", null, b.height == null ? el("span", { text: "—", class: "badge detached" })
        : el("a", { href: "#/block/" + b.hash, text: "#" + b.height })),
      el("td", null, el("a", { class: "hash-link", href: "#/block/" + b.hash, text: shortHash(b.hash, 14, 10), title: b.hash })),
      el("td", { text: fmtTime(b.timestamp) }),
      el("td", { class: "num", text: String(b.tx_count) }),
      el("td", { class: "num", text: dashIfUndef(b.size != null ? b.size.toLocaleString() : undefined) }),
      el("td", null, el("span", { class: "badge " + (b.canonical ? "canonical" : "detached"), text: b.canonical ? "CANONICAL" : "DETACHED" }))
    ));
  }
  const table = el("table", { class: "tbl" },
    el("thead", null, el("tr", null,
      el("th", { text: "Height" }), el("th", { text: "Hash" }),
      el("th", { text: "Timestamp" }), el("th", { text: "Txs" }),
      el("th", { text: "Size" }), el("th", { text: "Status" }))),
    tbody);
  return table;
}

function txTable(txs) {
  const tbody = el("tbody");
  for (const t of txs) {
    tbody.append(el("tr", null,
      el("td", { text: shortHash(t.txid, 16, 10), title: t.txid }),
      el("td", null, el("span", { class: "badge " + (t.coinbase ? "era" : ""), text: t.coinbase ? "COINBASE" : "TX" })),
      el("td", { class: "num", text: String(t.input_count) }),
      el("td", { class: "num", text: String(t.output_count) }),
      el("td", { class: "num", text: (t.total_out ?? 0).toLocaleString() }),
      el("td", { class: "num", text: t.fee === undefined ? "—" : String(t.fee),
        title: t.fee === undefined ? "普通交易 fee 需历史输出索引，API 诚实省略（非 0）" : "" })
    ));
  }
  return el("table", { class: "tbl" },
    el("thead", null, el("tr", null,
      el("th", { text: "TxID" }), el("th", { text: "Type" }),
      el("th", { text: "Inputs" }), el("th", { text: "Outputs" }),
      el("th", { text: "Total Out" }), el("th", { text: "Fee" }))),
    tbody);
}

/* ---------------- Route: Overview ---------------- */
async function renderOverview() {
  setTicker("overview", overviewTick);
  $app.replaceChildren(loadingBlock());
  await overviewTick(null, true);
}

async function overviewTick(signal, forceRedraw) {
  let status, page;
  try {
    status = await api("/status", {}, signal);
    const height = typeof status.height === "number" ? status.height : -1;
    const from = Math.max(0, height - 9);
    page = await api("/blocks?from=" + from + "&count=10", {}, signal);
  } catch (e) {
    if (e && e.name === "AbortError") return;
    apiFail();
    if (forceRedraw) $app.replaceChildren(errorBanner("Explorer API unavailable — 无法连接本机节点控制接口。"));
    else flashStale();
    return;
  }
  if (currentRouteKey !== "overview") return;
  drawOverview(status, page);
}

let overviewRoot = null;
function flashStale() {
  if (overviewRoot) overviewRoot.prepend(errorBanner("数据可能过期（连续 2 次以上刷新失败）— 显示的是最后一次成功数据。", "stale"));
}

function drawOverview(status, page) {
  const height = status.height;
  const tip = status.tip_hash;

  // -- stats row --
  const stats = el("div", { class: "grid stats" },
    el("div", { class: "panel" },
      el("div", { class: "stat-label", text: "Chain Height" }),
      el("div", { class: "stat-value", text: typeof height === "number" ? String(height) : "—" }),
      el("div", { class: "stat-sub", text: "tip " + shortHash(tip, 14, 10), title: tip || "" })),
    el("div", { class: "panel" },
      el("div", { class: "stat-label", text: "Mempool" }),
      el("div", { class: "stat-value", text: String(status.mempool_size ?? "—") }),
      el("div", { class: "stat-sub", text: status.mempool_size === 0 ? "empty（正常态）" : "pending txs" })),
    el("div", { class: "panel" },
      el("div", { class: "stat-label", text: "Network" }),
      el("div", { class: "stat-value", text: (status.peers && status.peers.length) ? String(status.peers.length) : "0" }),
      el("div", { class: "stat-sub", text: (status.peers && status.peers.length) ? "peers connected" : "standalone（无 peer，非错误）" })),
    el("div", { class: "panel" },
      el("div", { class: "stat-label", text: "Wallet Address" }),
      el("div", { class: "stat-value", style: "font-size:13px", text: shortHash(status.address || "—", 16, 12), title: status.address || "" }),
      el("div", { class: "stat-sub", text: "node wallet" }))
  );

  // -- mining panel（只读状态呈现；F-1/P1：Explorer 不提供任何挖矿控制）--
  const ms = status.mining_state || "UNKNOWN";
  const reasonLine = status.mining_reason ? " · " + status.mining_reason : "";
  const miningPanel = el("div", { class: "panel" },
    el("h2", { text: "Mining（只读状态）" }),
    el("div", { class: "mine-row" },
      el("span", { class: "state-chip " + ms, text: ms }),
      el("span", { class: "mine-msg", text: "continuous(-mine): " + (status.mining ? "ON" : "OFF") + " · pow_attempts: " + (status.pow_attempts ?? "—") + reasonLine })),
    el("div", { class: "stat-sub", style: "margin-top:8px", text: "Explorer 为只读浏览器，不提供按需出块；挖矿操作属于节点控制面（17881），不在本界面。" }));

  // -- recent blocks --
  const recent = (!page.blocks || page.blocks.length === 0)
    ? emptyState("暂无区块（空链态）")
    : blocksTable(page.blocks);

  const root = el("div", null, stats,
    el("div", { class: "grid two", style: "margin-top:14px" }, miningPanel,
      el("div", { class: "panel" }, el("h2", { text: "Node" }),
        el("dl", { class: "kv" },
          el("dt", { text: "accepted_blocks" }), el("dd", { text: String(status.accepted_blocks ?? "—") }),
          el("dt", { text: "rejected_blocks" }), el("dd", { text: String(status.rejected_blocks ?? "—") }),
          el("dt", { text: "mining_retries" }), el("dd", { text: String(status.mining_retries ?? "—") })))),
    el("div", { class: "panel", style: "margin-top:14px" },
      el("h2", null, document.createTextNode("Recent Blocks（最近 10 块，2s 轮询）"),
        el("a", { href: "#/blocks", text: " → 全部区块", style: "float:right;font-size:11px" })),
      recent));
  overviewRoot = root;
  $app.replaceChildren(root);
}

/* ---------------- Route: Blocks ---------------- */
// （F-1/P1：挖矿调用函数与 POST /api/mine 请求已随 Mine 按钮一并移除；
//   Explorer UI 现在只构造 GET /api/status、/api/blocks、/api/block。）
function renderBlocks(fromParam) {
  const routeKey = "blocks:" + fromParam;
  setTicker(routeKey, (signal) => blocksTick(signal, fromParam));
  $app.replaceChildren(loadingBlock());
  blocksTick(null, fromParam, true);
}

async function blocksTick(signal, fromParam, forceRedraw) {
  const COUNT = 20;
  const from = Math.max(0, parseInt(fromParam, 10) || 0);
  let page;
  try {
    page = await api("/blocks?from=" + from + "&count=" + COUNT, {}, signal);
  } catch (e) {
    if (e && e.name === "AbortError") return;
    apiFail();
    if (forceRedraw) $app.replaceChildren(errorBanner("Explorer API unavailable — 无法获取区块列表。"));
    else flashBlocksStale();
    return;
  }
  if (currentRouteKey !== "blocks:" + fromParam) return;
  drawBlocks(page, from, COUNT);
}

let blocksRoot = null;
function flashBlocksStale() {
  if (blocksRoot) blocksRoot.prepend(errorBanner("数据可能过期 — 显示最后一次成功数据。", "stale"));
}

function drawBlocks(page, from, count) {
  const root = el("div", null);
  root.append(el("div", { class: "panel" },
    el("h2", { text: "Blocks（canonical 主链，高度升序）" }),
    page.blocks && page.blocks.length
      ? blocksTable(page.blocks)
      : emptyState("该区间无区块（from 超过链尾为合法空区间）")));

  const maxFrom = Math.max(0, page.height - count + 1);
  const newerFrom = Math.min(from + count, maxFrom);
  const olderFrom = Math.max(0, from - count);
  const pager = el("div", { class: "pager" },
    el("a", { class: "btn", href: "#/blocks/" + olderFrom, text: "← Older" }),
    el("a", {
      class: "btn", href: "#/blocks/" + newerFrom,
      style: (page.at_tip || from + count > page.height) ? "pointer-events:none;opacity:.45" : ""
    }, "Newer →"),
    el("span", { class: "info", text: "from " + from + " · returned " + page.returned + " · chain height " + page.height }),
    page.at_tip ? el("span", { class: "at-tip", text: "AT TIP" }) : null);
  root.append(pager);
  blocksRoot = root;
  $app.replaceChildren(root);
}

/* ---------------- Route: Block Detail ---------------- */
function isValidHash(h) { return /^[0-9a-fA-F]{64}$/.test(h); }

function renderBlockDetail(hash) {
  setTicker("block:" + hash, null); // 详情页不自动轮询（冻结），提供手动刷新
  $app.replaceChildren(loadingBlock());
  if (!isValidHash(hash)) {
    $app.replaceChildren(errorBanner("Invalid block hash — 需要 64 个十六进制字符。"),
      el("a", { class: "btn", href: "#/blocks", text: "← Back to Blocks" }));
    return;
  }
  blockDetailTick(null, hash, true);
}

async function blockDetailTick(signal, hash, forceRedraw) {
  let b;
  try {
    b = await api("/block?hash=" + encodeURIComponent(hash), {}, signal);
  } catch (e) {
    if (e && e.name === "AbortError") return;
    apiFail();
    if (forceRedraw) {
      if (e.status === 404) {
        $app.replaceChildren(errorBanner("Block not found — 该哈希既不在主链也不在 detached 集。"),
          el("a", { class: "btn", href: "#/blocks", text: "← Back to Blocks" }));
      } else {
        $app.replaceChildren(errorBanner("Explorer API unavailable" + (e.status ? " (HTTP " + e.status + ")" : "") + "。"),
          el("a", { class: "btn", href: "#/blocks", text: "← Back to Blocks" }));
      }
    }
    return;
  }
  if (currentRouteKey !== "block:" + hash) return;
  drawBlockDetail(b, hash);
}

function drawBlockDetail(b, hash) {
  const root = el("div", null);

  const nav = el("div", { class: "detail-nav" },
    el("a", { class: "btn", href: "#/blocks", text: "← Back to Blocks" }),
    el("button", { class: "btn", text: "↻ Refresh", onclick: () => blockDetailTick(null, hash, true) }));

  const h = b.height;
  if (typeof h === "number") {
    const prevBtn = h > 0 ? el("button", {
      class: "btn", text: "← Previous (#" + (h - 1) + ")",
      onclick: () => jumpByHeight(h - 1, nav)
    }) : el("button", { class: "btn", text: "← Previous", disabled: "", title: "genesis 无前置块" });
    const nextBtn = el("button", {
      class: "btn", text: "Next (#" + (h + 1) + ") →",
      onclick: () => jumpByHeight(h + 1, nav)
    });
    nav.append(prevBtn, nextBtn);
  } else {
    nav.append(el("span", { class: "mine-msg conflict", text: "Previous/Next 不可用：detached 块无真实高度来源（API 诚实省略，不猜测）。" }));
  }
  root.append(nav);

  root.append(el("div", { class: "panel" },
    el("h2", null, document.createTextNode("Block " + (typeof h === "number" ? "#" + h : "")),
      el("span", { class: "badge " + (b.canonical ? "canonical" : "detached"), text: b.canonical ? "CANONICAL" : "DETACHED" }),
      el("span", { class: "badge era", text: "era: " + b.consensus_era })),
    el("dl", { class: "kv" },
      el("dt", { text: "height" }), el("dd", { text: h == null ? "—（detached，不猜测）" : String(h) }),
      el("dt", { text: "hash" }), el("dd", { text: b.hash }),
      el("dt", { text: "previous_hash" }), el("dd", null,
        b.previous_hash && b.previous_hash !== "0000000000000000000000000000000000000000000000000000000000000000"
          ? el("a", { class: "hash-link", href: "#/block/" + b.previous_hash, text: b.previous_hash })
          : document.createTextNode(b.previous_hash + "（genesis）")),
      el("dt", { text: "timestamp" }), el("dd", { text: fmtTime(b.timestamp) }),
      el("dt", { text: "version" }), el("dd", { text: String(b.version) }),
      el("dt", { text: "bits" }), el("dd", { text: hexBits(b.bits) + " (" + b.bits + ")" }),
      el("dt", { text: "difficulty" }), el("dd", { text: dashIfUndef(b.difficulty) }),
      el("dt", { text: "nonce" }), el("dd", { text: String(b.nonce) }),
      el("dt", { text: "merkle_root" }), el("dd", { text: b.merkle_root }),
      el("dt", { text: "size" }), el("dd", { text: b.size != null ? b.size.toLocaleString() + " B" : "—" }),
      el("dt", { text: "tx_count" }), el("dd", { text: String(b.tx_count) }),
      el("dt", { text: "persisted" }), el("dd", { text: String(b.persisted) }))));

  const txPanel = el("div", { class: "panel", style: "margin-top:14px" },
    el("h2", { text: "Transactions（" + (b.transactions ? b.transactions.length : 0) + "）" }));
  if (!b.transactions || b.transactions.length === 0) {
    txPanel.append(emptyState("该区块无交易（空块为合法态）"));
  } else {
    txPanel.append(txTable(b.transactions),
      el("div", { class: "stat-sub", style: "margin-top:8px", text: "tx 详情页（/tx/:txid）= DEFERRED（API 无按 txid 端点）" }));
  }
  root.append(txPanel);
  $app.replaceChildren(root);
}

// 通过真实 API（/blocks 批量端点）安全推导相邻块 hash 后跳转；无需 backend 修改。
async function jumpByHeight(targetH, nav) {
  try {
    const page = await api("/blocks?from=" + targetH + "&count=1");
    if (page.blocks && page.blocks.length) {
      location.hash = "#/block/" + page.blocks[0].hash;
    } else {
      alert("已到链尾（height " + page.height + "），无更高区块。");
    }
  } catch (e) {
    alert("跳转失败：Explorer API unavailable。");
  }
}

/* ---------------- Router ---------------- */
function route() {
  const hash = location.hash.replace(/^#/, "");
  document.querySelectorAll(".nav a").forEach(a => a.classList.remove("active"));
  if (hash === "" || hash === "/") {
    document.querySelector('[data-nav="overview"]').classList.add("active");
    renderOverview();
  } else if (hash === "/blocks" || hash.startsWith("/blocks/")) {
    document.querySelector('[data-nav="blocks"]').classList.add("active");
    const from = hash.split("/")[2] || "0";
    renderBlocks(from);
  } else if (hash.startsWith("/block/")) {
    renderBlockDetail(hash.slice("/block/".length));
  } else {
    // malformed route：优雅呈现，不出现 undefined/blank
    setTicker("unknown", null);
    $app.replaceChildren(
      errorBanner("Unknown route: " + (hash || "(empty)")),
      el("a", { class: "btn", href: "#/", text: "← Overview" }));
  }
}

window.addEventListener("hashchange", route);
setNetBadge("stale", "CONNECTING…");
ensureTimer();
route();
