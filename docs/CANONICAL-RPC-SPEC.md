# P2PChain — CANONICAL RPC SPEC（控制接口）

> **本文是本地 JSON 控制接口的规范描述（CANONICAL SPECIFICATION），描述当前可执行源码的行为。**
> 本文**不是**真值源：最高真值为**可执行实现 + 可重复测试**。旧 README 的「6 个端点 / 无鉴权」**已作废**（以可执行实现为准）。

- **建立/治理修正阶段**：初稿由第二写入者于 2026-10-01 01:12 落盘（未提交）；经 `DOCUMENT AUTHORITY RECONCILIATION-1` 审计发现权威缺陷；`DOCUMENT AUTHORITY GOVERNANCE CLOSURE-1` 治理修正（删除"唯一真值源"歧义、明确 L0 终审、修正来源声明）。
- **最近修订**：`ON-DEMAND-MINING-REMOVAL-1` —— 按需出块（`POST /mine`、`POST /console/mine`）**整体下线**（路由、服务层、客户端方法、CLI 子命令、Console/GUI 入口全部移除），同源闸门 `consoleOriginGate` / `isSameOriginRequest` 随之退役。端点总数 13 → **11**；mutation 6 → **4**；`403` 码不再由控制面产生。
- **适用基线**：上一版基线 HEAD `434f8c7c6cc39585c8c31deb35e33b27bef1edb2`；本次修订对应基线 HEAD `467621769c19f455e10e575dd1213fa6d02ca2c1` **加**上述下线改动
- **主要证据**：`internal/control/server.go`（路由 309-331）、`console.go`、`logs.go`
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

### 2.2 ~~`consoleOriginGate`~~（**已退役**）

原 `consoleOriginGate` / `isSameOriginRequest`（基于 `Sec-Fetch-Site` / `Origin` 的「同源闸门」）在
`PHASE RPC-CONTROL-PLANE-AUTH-HARDENING-1`（E6-C-1）已被判定**可被非浏览器客户端伪造绕过**
（curl / python 可任意设置这些请求头）而退役；其唯一使用者 `POST /console/mine` 又已在
`ON-DEMAND-MINING-REMOVAL-1` 整体下线。**当前控制面不存在任何依赖同源判定的端点**，
故 `403` 不再由控制面产生。

### 2.3 只读端点认证策略（明确）

**GET 只读端点（`/status` `/balance` `/utxos` `/block` `/blocks` `/logs` `/`）不要求 Bearer 令牌。**

这是**有意设计**（观测面免凭据便于本机使用），其安全性依赖**绑定在回环地址**这一前提。
⇒ **若控制接口被绑定到非回环地址，这些端点即无保护地暴露**（见 `docs/SECURITY-BOUNDARY.md`）。

---

## §3 端点总表（**11 条**）

| # | 端点 | 方法 | 类型 | 认证 | 说明 |
|---|---|---|---|---|---|
| 1 | `/status` | GET | read-only | 无 | 节点状态 |
| 2 | `/balance` | GET | read-only | 无 | 余额（可花费 / 含未成熟） |
| 3 | `/utxos` | GET | read-only | 无 | UTXO 列表 |
| 4 | `/send` | POST | **mutation** | **Bearer** | 用节点钱包转账 |
| 5 | `/mine/start` | POST | **mutation** | **Bearer** | 启动持续挖矿 |
| 6 | `/mine/stop` | POST | **mutation** | **Bearer** | 停止挖矿（**绝不停节点**） |
| 7 | `/block` | GET | read-only | 无 | 区块（按 `height=` 或 `hash=`） |
| 8 | `/blocks` | GET | read-only | 无 | 区块分页 |
| 9 | `/logs` | GET | read-only | 无 | 日志尾部 |
| 10 | `/stop` | POST | **mutation** | **Bearer** | 优雅停止节点 |
| 11 | `/`（`/console`） | GET/HEAD | read-only | 无 | Developer Console 页面（embed HTML） |

> **已下线（不存在该路由）**：`POST /mine`、`POST /console/mine`。二者一律 `404`
> （命中 `handleConsole` 的「路径非 `/` 且非 `/console` ⇒ 404」分支），与请求头、是否
> 携带有效 Bearer 令牌**无关** —— 这是比 `401` 更强的边界（判定发生在路由层）。
> 出块只能由**启动期 `-mine`** 或 `POST /mine/start` 驱动。

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

### 4.5 `POST /mine/start`

- 认证：**Bearer**。
- 请求体：仅接受空体或 `{}`（`DisallowUnknownFields`，4 KiB 上限）。
- 响应：`200` 启动结果；**`409 Conflict`** 当处于冲突状态（响应体含 `error` 与 `state`）。

### 4.6 `POST /mine/stop`

- 认证：**Bearer**。
- 语义：**仅停止挖矿，绝不停节点**。
- 幂等：从未启动 / 已停止 / 重复 STOP 一律 `200 Accepted=true`；`FAILED` 状态下保持 `FAILED`（**不抹掉失败证据**）。
- 响应返回**受理状态**（`STOPPING`）；最终 `STOPPED` 由挖矿循环收尾后经 `/status` 可见。

### 4.7 `GET /block`

- 参数**二选一且互斥**：
  - `height=<int>` ⇒ 既有契约，返回 hex 编码区块；
  - `hash=<64 hex>` ⇒ 按哈希查询。
- 同时提供 ⇒ `400`（`ErrAmbiguousBlockQuery`）。
- `hash` 长度 ≠ 64 或非 hex ⇒ `400`。

### 4.8 `GET /blocks?from=&count=`

- `from` / `count` **均为必填整数**。
- 约束：`from >= 0`；`count ∈ [1, MaxBlocksPerPage]`，`MaxBlocksPerPage = 100`。
- 违规 ⇒ `400`（含实际值，便于定位）。

### 4.9 `GET /logs?tail=`

- `tail` 可选，默认 `defaultLogTail = 100`，上限 `MaxLogTail = 500`（超限**静默裁剪**至 500）。
- `tail` 非正整数 ⇒ `400`。
- 无日志来源时返回**空数组**（诚实空态，**不编造内容**）。

### 4.10 `POST /stop`

- 认证：**Bearer**。
- 无 `stopHook` ⇒ **`501 Not Implemented`**（`ErrStopUnsupported`）。
- 有 hook ⇒ `200 {"accepted":true,"message":...}`；节点随后释放数据目录锁并退出。

### 4.11 `GET /`（`/console`）

- 路径非 `/` 且非 `/console` ⇒ `404`。
- 方法非 GET/HEAD ⇒ `405` + `Allow: GET`。
- 返回 `go:embed` 的 `web/console.html`（`Content-Type: text/html; charset=utf-8`，`Cache-Control: no-store`）。

---

## §5 mutation 语义汇总

| 端点 | 幂等 | 副作用 |
|---|---|---|
| `/send` | 否 | 构造交易、入池、广播 |
| `/mine/start` | 否（冲突 ⇒ 409） | 启动持续挖矿 |
| `/mine/stop` | **是** | 停止挖矿（不停节点） |
| `/stop` | 否 | 停止节点并释放锁 |

> 已下线的 `POST /mine`（同步挖 N 个区块，`MaxMineCount = 1000`）与 `POST /console/mine`
> 不再属于 mutation 面；控制面可被滥用的出块入口相应减少一个。

---

## §6 错误码约定

| 码 | 出现场景 |
|---|---|
| `400` | 参数缺失/非法、请求体解析失败、地址非法、哈希非法、分页越界 |
| `401` | mutation 端点未通过 Bearer 认证（**fail-closed**） |
| `404` | 控制台路径不匹配（含已下线的 `/mine`、`/console/mine`） |
| `405` | 方法不符（附 `Allow` 头） |
| `409` | `/mine/start` 状态冲突 |
| `500` | 节点侧错误（状态/余额/UTXO 获取失败等） |
| `501` | `/stop` 无停止钩子 |

> `403` 已从控制面消失：其唯一来源（同源闸门）随 `/console/mine` 下线而退役（见 §2.2）。

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
| 6 个端点 | **11 个**（原 13 个；`/mine` 与 `/console/mine` 于 `ON-DEMAND-MINING-REMOVAL-1` 下线） | **CONFLICT**（R-06） |
| "无鉴权" | mutation **4 条** Bearer **fail-closed**；只读 7 条无鉴权（依赖回环绑定）；同源闸门已退役 | **CONFLICT（安全语义反向）**（R-05） |
| `/send` `/mine` 无速率/批量约束 | 请求体 4 KiB、`MaxBlocksPerPage=100`；`MaxMineCount` 随 `/mine` 下线而删除 | **STALE** |
