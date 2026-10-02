# PHASE P2PCHAIN — REMOTE MINING TEST GATE SPEC
## 开发机器专用远程挖矿测试通道设计规格

- **阶段**：`PHASE-P2PCHAIN-REMOTE-MINING-TEST-GATE-SPEC-1`
- **性质**：纯设计（只设计，不修改代码、不修改云节点、不部署）。
- **目标**：设计一条「开发机器 → 腾讯云节点」的受控远程挖矿测试通道，使开发机器能够安全地触发云上节点的按需出块（on-demand mining）进行测试，同时严格守住生产安全边界。
- **设计时间**：`2026-10-02 11:12 CST`

---

## 0. 文档定位与设计原则

本规格基于对**现有代码的真实审计**（非臆测），提出一条「最小侵入、可撤销、默认拒绝」的远程测试通道设计。设计遵循以下原则：

1. **默认拒绝（fail-closed）**：未显式配置即不开放远程，通道关闭状态与现状完全一致。
2. **复用现有鉴权，不新造密码学**：沿用 `requireAuth` 的 Bearer Token + constant-time 比较 + 固定失败延迟，不引入自研加密。
3. **最小暴露面**：只暴露 `mining` 相关测试端点，不开放 `/send /stop` 等破坏性/资金相关端点。
4. **全链路可审计**：每次远程请求落审计日志，可追溯来源 IP、client id、结果。
5. **可撤销**：通道可一键关闭，关闭后回退到纯 loopback 现状。

---

## §0 当前 mining / console 架构分析（基于源码审计）

### 0.1 现状事实

| 维度 | 现状（已审计确认） |
|------|-------------------|
| RPC 绑定 | 恒 `127.0.0.1`（`DefaultAddr = "127.0.0.1:6689"`），注释明确「只做本机控制，不做远程钱包服务」 |
| 云上实际 | 三节点 RPC 分别绑 `127.0.0.1:16689/16691/16693`，仅 loopback，外部不可达 |
| mutation 鉴权 | `requireAuth`：Bearer Token，`crypto/subtle.ConstantTimeCompare`，固定 `authFailureDelay` 防爆破 |
| token 来源 | `-auth-token-file` 指定文件（云上 `/opt/p2pchain/secrets/control-token`，0600），token 绝不进命令行/环境变量/日志 |
| mutation 端点 | `/send` `/mine` `/mine/start` `/mine/stop` `/console/mine` `/stop` 全部 requireAuth |
| read 端点 | `/status` `/balance` `/utxos` `/block` `/blocks` `/logs` 无需鉴权（loopback 保护） |
| 出块能力 | `handleMine`（按需挖 count 个，`MaxMineCount=1000`）、`handleMineStart/Stop`（持续挖矿） |
| 客户端 | `internal/control/client.go` 已内置 `SetToken` + `Mine` + `Status` 等方法 |
| IP 白名单 | **当前无任何 IP allowlist 机制**（grep 为空） |
| 外部访问路径 | 需 SSH 隧道（`ssh -L 16689:127.0.0.1:16689 ...`）才能触达云上 RPC |

### 0.2 当前「远程挖矿」的事实路径

开发机器目前若要触发云上出块，唯一安全路径是 **SSH 隧道**：

```
开发机器 ──ssh -L 16689:127.0.0.1:16689 root@111.229.225.123──▶ 云上 127.0.0.1:16689
   │                                                                      │
   └── curl -H "Bearer <token>" POST /mine ────────────────────────────────┘
```

- SSH 隧道本身已提供：传输加密、主机认证、用户身份（root 免密 ed25519）、可达性隔离。
- 但**缺乏**：client 身份标识、独立审计日志、细粒度 IP 白名单、独立测试 token。

### 0.3 设计动机（为什么需要专用通道）

| 现状痛点 | 影响 |
|----------|------|
| 复用生产 `control-token` 做测试 | 测试泄漏风险与生产 token 混同，无法区分「测试出块」与「生产操作」 |
| 无 client identity | 审计日志无法区分「哪个开发机器/哪个测试脚本」触发了出块 |
| 无 IP 白名单 | 一旦 RPC 开放非 loopback，任何持有 token 的 IP 均可调用 |
| 无独立审计 | 测试出块混入生产日志，无法单独追踪测试活动 |

---

## §1 白名单模型设计

### 1.1 三层准入模型（纵深防御）

```
┌─────────────────────────────────────────────────────────┐
│ L1 网络层：IP allowlist（仅白名单 IP 可到达 RPC 端口）      │
│ L2 应用层：Bearer Token（constant-time 比对）              │
│ L3 身份层：client identity（可选，用于审计归因）            │
└─────────────────────────────────────────────────────────┘
```

### 1.2 L1 — IP Allowlist

| 项 | 设计 |
|----|------|
| 配置键 | `-test-mining-allowlist`（逗号分隔 CIDR/IP 列表） |
| 默认值 | 空 → 通道关闭（fail-closed，与现状一致） |
| 语义 | 仅当请求 `RemoteAddr` 命中 allowlist 时，才进入 L2 token 校验 |
| 匹配 | 支持单 IP（`115.191.60.241`）与 CIDR（`115.191.60.0/24`） |
| 命中失败 | 直接 403（不进入 token 校验，避免泄露 token 校验时机） |
| 存储 | 运行时内存，启动时解析，非法条目 fail-closed |

> 注意：云上节点 RPC 若仍绑 `127.0.0.1`，则 `RemoteAddr` 恒为 `127.0.0.1`（经 SSH 隧道），IP allowlist 需配合「RPC 绑定地址」策略（见 §5 安全边界）才有意义。设计上支持「绑 loopback + SSH 隧道」与「绑内网 + allowlist」两种部署形态。

### 1.3 L2 — Bearer Token

| 项 | 设计 |
|----|------|
| 配置键 | `-test-mining-token-file`（独立于生产 `-auth-token-file`） |
| 默认值 | 空 → 测试通道关闭 |
| 语义 | 测试通道使用**独立 token**，与生产 `control-token` 分离 |
| 校验 | 完全复用 `requireAuth` 的 constant-time 比较 + `authFailureDelay` |
| 归一化 | 复用 `LoadTokenFile`（trim + 长度区间 + 0600 权限检查） |

### 1.4 L3 — Client Identity（可选，审计归因）

| 项 | 设计 |
|----|------|
| 载体 | 请求头 `X-Mining-Client: <client_id>`（自定义，非标准） |
| 取值 | 开发机器自定义字符串（如 `dev-win-06uf28s`、`ci-runner-01`） |
| 校验 | 不做强校验（仅用于审计归因），长度上限 64 字符 |
| 落日志 | 写入审计日志 `client_id` 字段，便于追溯 |

---

## §2 测试矿工客户端设计

### 2.1 CLI 形态

复用现有 `internal/control/client.go`，新增一个轻量测试客户端（设计为独立命令 `p2pchain-miner-test` 或 `node test-mine` 子命令）：

```
p2pchain-miner-test \
  --rpc 127.0.0.1:16689 \          # 目标节点 RPC（经 SSH 隧道）
  --token-file ./test-token \       # 独立测试 token
  --client-id dev-win-06uf28s \     # 审计归因身份
  --count 3                         # 按需出块数量（1..1000）
```

### 2.2 API 契约（客户端 → 服务端）

| 字段 | 说明 |
|------|------|
| 方法 | `POST /mine`（复用现有端点，不新增端点） |
| 请求头 | `Authorization: Bearer <test-token>` + `X-Mining-Client: <client_id>` |
| 请求体 | `{"count": 3}`（复用现有 `MineRequest`） |

### 2.3 Response 契约

| 场景 | HTTP | 响应体 |
|------|------|--------|
| 成功 | 200 | 复用现有 `MineResponse`（accepted + height 等） |
| IP 不在白名单 | 403 | `{"error":"forbidden"}` |
| token 缺失/错误 | 401 | `{"error":"unauthorized"}`（最小化，不区分原因） |
| count 非法 | 400 | `{"error":"count 必须在 1..1000 之间..."}` |
| 节点忙/冲突 | 409 | 复用现有冲突语义 |

> 客户端实现要点：`SetToken` 后额外 `SetHeader("X-Mining-Client", ...)`；超时 15s（复用现有 `Client`）。

---

## §3 服务端鉴权流程

### 3.1 请求处理流水线

```
POST /mine 请求到达
   │
   ├─ 1. 方法守卫（非 POST → 405，复用现有 requireMethod）
   │
   ├─ 2. 测试通道开关判断：test-mining 是否启用？
   │     ├─ 未启用 → 走现有 requireAuth 路径（生产 token），行为与现状完全一致
   │     └─ 已启用 → 进入专用 test-mining 流水线
   │
   ├─ 3. L1 IP allowlist：RemoteAddr 命中？
   │     ├─ 未命中 → 403（记录审计：ip_denied）
   │     └─ 命中 → 继续
   │
   ├─ 4. L2 token：constant-time 比对 test-token？
   │     ├─ 失败 → 固定延迟 + 401（记录审计：auth_failed）
   │     └─ 成功 → 继续
   │
   ├─ 5. L3 client identity：提取 X-Mining-Client（可选）
   │
   ├─ 6. 调用 handleMine（复用现有出块逻辑）
   │
   └─ 7. 审计日志落盘（见 §4）
```

### 3.2 关键设计决策

| 决策 | 理由 |
|------|------|
| **复用 `/mine` 端点，不新增端点** | 最小侵入；`handleMine` 逻辑（count 校验、冲突处理）完全复用 |
| **测试通道是「增强」而非「替代」** | 未启用时，`/mine` 仍走生产 `requireAuth`，零行为回归 |
| **IP allowlist 在 token 之前** | 先拒不可信 IP，避免对不可信来源暴露 token 校验（时序侧信道缓解） |
| **测试 token 独立于生产 token** | 隔离测试/生产凭据，测试 token 泄漏不影响生产控制面 |

---

## §4 审计日志设计

### 4.1 审计事件结构（JSON，独立于运行日志）

```json
{
  "event": "TEST_MINING",
  "ts": "2026-10-02T11:12:00.000+08:00",
  "client_id": "dev-win-06uf28s",
  "remote_ip": "127.0.0.1",
  "action": "mine",
  "count": 3,
  "result": "accepted",          // accepted | ip_denied | auth_failed | rejected
  "height_before": 137,
  "height_after": 140,
  "error": ""
}
```

### 4.2 审计字段说明

| 字段 | 说明 |
|------|------|
| `event` | 固定 `TEST_MINING`，与生产日志区分 |
| `client_id` | 来自 `X-Mining-Client` 头（归因身份） |
| `remote_ip` | `r.RemoteAddr`（经隧道时显示为 127.0.0.1，需配合隧道源记录） |
| `action` | `mine` / `mine-start` / `mine-stop` |
| `result` | 四态：`accepted`（成功）/ `ip_denied`（403）/ `auth_failed`（401）/ `rejected`（业务拒绝） |
| `height_before/after` | 出块前后高度，便于追踪测试影响 |

### 4.3 落盘策略

| 项 | 设计 |
|----|------|
| 文件 | `<datadir>/test-mining-audit.log` 或复用 `/opt/p2pchain/logs/test-mining.log` |
| 格式 | 每行一条 JSON（append-only），与现有 `logring`/日志体系解耦 |
| 隐私 | **绝不记录 token 内容**（只记 token 校验结果） |
| 轮转 | 复用现有日志轮转或按大小切分（不在此规格展开） |

---

## §5 安全边界

### 5.1 明确「不做什么」（防止 scope creep）

| 禁止 | 理由 |
|------|------|
| ❌ 不开放 `/send`（转账） | 资金操作绝不暴露远程，测试通道仅限 mining |
| ❌ 不开放 `/stop`（停节点） | 破坏性动作保持仅本机 + 生产 token |
| ❌ 不开放 `/mine/start`（持续挖矿）到远程 | 持续挖矿影响面大，测试通道仅允许按需出块 `/mine` |
| ❌ 不改生产 token 语义 | `control-token` 完全独立，测试 token 不覆盖它 |
| ❌ 不做「无 token 出块」 | 任何远程出块都必须过 token（fail-closed） |

### 5.2 部署形态与暴露面权衡

| 形态 | 暴露面 | 适用 |
|------|--------|------|
| A. 绑 loopback + SSH 隧道（推荐） | 最小，RPC 不出主机 | 单开发机器临时测试 |
| B. 绑内网 + IP allowlist | 中等，仅白名单 IP 可达 | 多机内网测试 |
| C. 绑公网 + allowlist + token | 大，需 TLS + 强鉴权 | **不推荐**，除非加反向代理/TLS |

> 本规格**默认推荐形态 A**：保持 RPC 绑 127.0.0.1，测试机器经 SSH 隧道触达，IP allowlist 作为纵深防御的第二道闸（即使未来误改绑定，allowlist 仍兜底）。

### 5.3 威胁模型与缓解

| 威胁 | 缓解 |
|------|------|
| token 泄漏 | 独立测试 token，可随时轮换；constant-time 比较防时序侧信道 |
| 非白名单 IP 调用 | L1 allowlist 先拒（403），不进入 token 校验 |
| 爆破 token | 复用 `authFailureDelay` 固定延迟，无锁定/黑名单（与现有契约一致） |
| 审计缺失 | 每次请求落 `TEST_MINING` 审计事件，含 client_id + 来源 |
| 测试影响生产 | 仅开放按需出块，不开放转账/停节点；height 变化可追踪 |

### 5.4 配置汇总（新增，均默认关闭）

| 配置项 | 类型 | 默认 | 说明 |
|--------|------|------|------|
| `-test-mining-enable` | bool | false | 测试通道总开关 |
| `-test-mining-allowlist` | string | "" | IP/CIDR 白名单，空=关闭 |
| `-test-mining-token-file` | string | "" | 独立测试 token 文件，空=关闭 |

---

## 6. 落地前置条件（后续实现阶段需 Owner 授权）

本规格为纯设计。若后续进入实现阶段，需明确：

1. **实现范围**：仅 `internal/control/server.go` + `cmd/node/main.go` 配置项 + 客户端小工具，不改共识/区块/钱包逻辑。
2. **默认关闭**：新增配置全部 fail-closed，现有生产节点不传新参数即行为零变化。
3. **独立测试**：需补充 allowlist 匹配、token 隔离、审计日志的单测。
4. **回归验证**：验证「未启用测试通道时，`/mine` 行为与现状完全一致」。

---

## HARD STOP

本阶段为纯设计，已完成。**未修改任何代码、未修改云节点、未部署。** 后续实现需 Owner 单独授权。
