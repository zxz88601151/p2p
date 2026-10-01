# P2PChain — CANONICAL RPC SPEC（控制接口）

> **本文是本地 JSON 控制接口的规范描述（CANONICAL SPECIFICATION），描述当前可执行源码的行为。**
> 本文**不是**真值源：最高真值为**可执行实现 + 可重复测试**。旧 README 的「6 个端点 / 无鉴权」**已作废**（以可执行实现为准）。

- **建立/治理修正阶段**：初稿由第二写入者于 2026-10-01 01:12 落盘（未提交）；经 `DOCUMENT AUTHORITY RECONCILIATION-1` 审计发现权威缺陷；`DOCUMENT AUTHORITY GOVERNANCE CLOSURE-1` 治理修正（删除"唯一真值源"歧义、明确 L0 终审、修正来源声明）。
- **适用基线**：HEAD `434f8c7c6cc39585c8c31deb35e33b27bef1edb2`
- **主要证据**：`internal/control/server.go`（路由 321-342）、`console.go`、`logs.go`
- **状态**：**CANONICAL / CURRENT**

---

## §1 通用契约

| 项 | 值 |
|---|---|
| 协议 | HTTP/1.1，JSON（`Content-Type: application/json; charset=utf-8`） |
| 默认绑定 | `127.0.0.1:6689`（`control.DefaultAddr`，`client.go:15`）——**仅本机回环** |
| 覆盖方式 | 节点启动参数 `-rpc <addr>` |
| 非回环告警 | 监听到非回环地址时启动告警 |
| 方法守卫 | `requireMethod` ⇒ 不符时 `405` + `Allow: <method>` |
| 错误格式 | 统一 `{"error": "<原因>"}`，**绝不为泛化的 "invalid"** |
| 并发 | 由 `net/http` 提供；节点侧状态受 `blockchain.mu` / `Server.mu` 保护，快照式读取 |

---

## §2 认证（**FAIL-CLOSED**，旧 README「无鉴权」已作废）

### 2.1 `requireAuth`（mutation 端点）

```
条件：仅对 POST 生效（非 POST 交给既有方法守卫，保持「GET /stop → 405 + Allow: POST」不变）
判定：expected != "" && given != "" && subtle.ConstantTimeCompare(expected, given) == 1
失败：① 可选固定延迟（默认 500ms，防爆破） → ② 401 {"error":"unauthorized"}
```

| 属性 | 值 |
|---|---|
| 令牌来源 | Bearer：`Authorization: Bearer <token>` |
| 比较方式 | `crypto/subtle.ConstantTimeCompare`（**常量时间**，防时序侧信道） |
| **无令牌时行为** | **`FAIL-CLOSED`**：`expected == ""` ⇒ **一律 401**，不放行 |
| 失败延迟 | `defaultAuthFailureDelay = 500ms`（可配；`0` = 关闭） |
| 令牌长度约束 | `minTokenLen = 16`、`maxTokenLen = 1024` |
| 令牌文件权限 | 非 Windows 下要求 **owner-only 0600**，过宽 ⇒ 启动期报错 |

### 2.2 `consoleOriginGate`（`/console/mine`）

```
非 POST → 交给方法守卫
POST 且非同源 → 403 {"error":"forbidden"}
```

同源判定 `isSameOriginRequest`：`Sec-Fetch-Site: same-origin` 或 `Origin` **严格等于** host。

### 2.3 只读端点认证策略（明确）

**GET 只读端点（`/status` `/balance` `/utxos` `/block` `/blocks` `/logs` `/`）不要求 Bearer 令牌。**

这是**有意设计**（观测面免凭据便于本机使用），其安全性依赖**绑定在回环地址**这一前提。
⇒ **若控制接口被绑定到非回环地址，这些端点即无保护地暴露**（见 `docs/SECURITY-BOUNDARY.md`）。

---

## §3 端点总表（**13 条**）

| # | 端点 | 方法 | 类型 | 认证 | 说明 |
|---|---|---|---|---|---|
| 1 | `/status` | GET | read-only | 无 | 节点状态 |
| 2 | `/balance` | GET | read-only | 无 | 余额（可花费 / 含未成熟） |
| 3 | `/utxos` | GET | read-only | 无 | UTXO 列表 |
| 4 | `/send` | POST | **mutation** | **Bearer** | 用节点钱包转账 |
| 5 | `/mine` | POST | **mutation** | **Bearer** | 按需出块（同步） |
| 6 | `/mine/start` | POST | **mutation** | **Bearer** | 启动持续挖矿 |
| 7 | `/mine/stop` | POST | **mutation** | **Bearer** | 停止挖矿（**绝不停节点**） |
| 8 | `/console/mine` | POST | **mutation** | **同源闸门** | Console 页面出块入口 |
| 9 | `/block` | GET | read-only | 无 | 区块（按 `height=` 或 `hash=`） |
| 10 | `/blocks` | GET | read-only | 无 | 区块分页 |
| 11 | `/logs` | GET | read-only | 无 | 日志尾部 |
| 12 | `/stop` | POST | **mutation** | **Bearer** | 优雅停止节点 |
| 13 | `/`（`/console`） | GET/HEAD | read-only | 无 | Developer Console 页面（embed HTML） |

---

## §4 端点明细

### 4.1 `GET /status`

- 无参数。
- 响应：高度、链尾哈希、节点钱包地址、对等节点列表、内存池大小、挖矿状态（`MiningState`）；仅在异常状态（停滞/失败）时附带 `MiningReason`。
- 错误：`500` 节点状态获取失败。

### 4.2 `GET /balance?address=<addr>`

- `address` **必填**，缺失 ⇒ `400`（`ErrAddressRequired`）。
- 响应：`address`、`spendable`、`total`（含未成熟）、`utxo_count`、`height`。
- 错误：`400` 地址非法或查询失败。

### 4.3 `GET /utxos?address=<addr>`

- `address` **必填**，缺失 ⇒ `400`。
- 响应：数组（无 UTXO 时返回 **空数组**而非 `null`）；元素 = `UTXOInfo{outpoint, value, height, is_coinbase, mature}`。
- 错误：`400`。

### 4.4 `POST /send`

- 认证：**Bearer（fail-closed）**。
- 请求体（`MaxBytesReader` 4 KiB）：`{"to": string, "amount": uint64, "fee": uint64}`。
- 响应 `200`：`{"txid","fee","amount","to","input_num"}`。
- 错误：`400` 请求体解析失败 / 参数非法；`401` 未授权；`500`。

### 4.5 `POST /mine`

- 认证：**Bearer**。
- 请求体**可省略**（默认 `Count = 1`）：`{"count": int}`。
- 约束：`count ∈ [1, MaxMineCount]`，`MaxMineCount = 1000`。
- 响应 `200`：`{"mined","height"}`。

### 4.6 `POST /mine/start`

- 认证：**Bearer**。
- 请求体：仅接受空体或 `{}`（`DisallowUnknownFields`，4 KiB 上限）。
- 响应：`200` 启动结果；**`409 Conflict`** 当处于冲突状态（响应体含 `error` 与 `state`）。

### 4.7 `POST /mine/stop`

- 认证：**Bearer**。
- 语义：**仅停止挖矿，绝不停节点**。
- 幂等：从未启动 / 已停止 / 重复 STOP 一律 `200 Accepted=true`；`FAILED` 状态下保持 `FAILED`（**不抹掉失败证据**）。
- 响应返回**受理状态**（`STOPPING`）；最终 `STOPPED` 由挖矿循环收尾后经 `/status` 可见。

### 4.8 `POST /console/mine`

- 认证：**同源闸门**（非 Bearer）。非同源 POST ⇒ `403`。
- 其余语义同 `/mine`。

### 4.9 `GET /block`

- 参数**二选一且互斥**：
  - `height=<int>` ⇒ 既有契约，返回 hex 编码区块；
  - `hash=<64 hex>` ⇒ 按哈希查询。
- 同时提供 ⇒ `400`（`ErrAmbiguousBlockQuery`）。
- `hash` 长度 ≠ 64 或非 hex ⇒ `400`。

### 4.10 `GET /blocks?from=&count=`

- `from` / `count` **均为必填整数**。
- 约束：`from >= 0`；`count ∈ [1, MaxBlocksPerPage]`，`MaxBlocksPerPage = 100`。
- 违规 ⇒ `400`（含实际值，便于定位）。

### 4.11 `GET /logs?tail=`

- `tail` 可选，默认 `defaultLogTail = 100`，上限 `MaxLogTail = 500`（超限**静默裁剪**至 500）。
- `tail` 非正整数 ⇒ `400`。
- 无日志来源时返回**空数组**（诚实空态，**不编造内容**）。

### 4.12 `POST /stop`

- 认证：**Bearer**。
- 无 `stopHook` ⇒ **`501 Not Implemented`**（`ErrStopUnsupported`）。
- 有 hook ⇒ `200 {"accepted":true,"message":...}`；节点随后释放数据目录锁并退出。

### 4.13 `GET /`（`/console`）

- 路径非 `/` 且非 `/console` ⇒ `404`。
- 方法非 GET/HEAD ⇒ `405` + `Allow: GET`。
- 返回 `go:embed` 的 `web/console.html`（`Content-Type: text/html; charset=utf-8`，`Cache-Control: no-store`）。

---

## §5 mutation 语义汇总

| 端点 | 幂等 | 副作用 |
|---|---|---|
| `/send` | 否 | 构造交易、入池、广播 |
| `/mine` | 否 | 同步挖 N 个区块 |
| `/mine/start` | 否（冲突 ⇒ 409） | 启动持续挖矿 |
| `/mine/stop` | **是** | 停止挖矿（不停节点） |
| `/console/mine` | 否 | 同 `/mine` |
| `/stop` | 否 | 停止节点并释放锁 |

---

## §6 错误码约定

| 码 | 出现场景 |
|---|---|
| `400` | 参数缺失/非法、请求体解析失败、地址非法、哈希非法、分页越界 |
| `401` | mutation 端点未通过 Bearer 认证（**fail-closed**） |
| `403` | `/console/mine` 非同源 |
| `404` | 控制台路径不匹配 |
| `405` | 方法不符（附 `Allow` 头） |
| `409` | `/mine/start` 状态冲突 |
| `500` | 节点侧错误（状态/余额/UTXO 获取失败等） |
| `501` | `/stop` 无停止钩子 |

---

## §7 安全边界（RPC 层）

- **默认仅回环**。任何非回环绑定都会使 7 个无鉴权只读端点直接暴露。
- **无 TLS**：明文 HTTP。
- mutation 端点受 Bearer 保护，但**令牌在网络上明文传输**（除非置于 TLS 后）。
- 请求体有 4 KiB 上限；分页、批量、日志条数均有上限。
- ⇒ **SAFE：** localhost / 受信任 LAN 开发实验。**NOT SAFE：** 公网 / 敌对网络 / 真实资产。详见 `docs/SECURITY-BOUNDARY.md`。

---

## §8 与旧文档的 DRIFT 登记

| 旧声明 | 本文真值 | DRIFT |
|---|---|---|
| 6 个端点 | **13 个** | **CONFLICT**（R-06） |
| "无鉴权" | mutation 5 条 Bearer **fail-closed**；`/console/mine` 同源闸门；只读 7 条无鉴权（依赖回环绑定） | **CONFLICT（安全语义反向）**（R-05） |
| `/send` `/mine` 无速率/批量约束 | `MaxMineCount=1000`、请求体 4 KiB、`MaxBlocksPerPage=100` | **STALE** |
