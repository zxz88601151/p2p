# PHASE P2PCHAIN — REMOTE MINING TEST ARCHITECTURE DECISION
## 远程挖矿测试通道最终架构决策文档

- **阶段**：`PHASE-P2PCHAIN-REMOTE-MINING-TEST-ARCHITECTURE-DECISION-1`
- **性质**：只读设计决策，未改代码、未部署、未修改云节点。
- **基于**：`REMOTE-MINING-TEST-GATE-SPEC-1`
- **决策时间**：`2026-10-02 11:13 CST`

---

## 决策摘要（先说结论）

| 决策项 | 最终选择 |
|--------|---------|
| 接入方案 | **SSH Tunnel（形态 A）为主，IP Allowlist 作为纵深兜底** |
| 权限模型 | 独立测试 token + 只读/只挖矿最小权限（不开放 /send /stop） |
| Token 生命周期 | 独立文件 + 可轮换 + 无过期（短生命周期测试用）+ 可一键撤销 |
| 审计模型 | `TEST_MINING` 独立审计日志 + client_id 归因 |

**总体架构定性**：采用「**保持 loopback + SSH 隧道 + 应用层 IP allowlist 兜底**」的混合方案，**不引入 Gateway 反向代理**（对当前 3 节点 loopback 集群而言过度设计）。

---

## §0 当前 RPC 安全边界确认（源码 + 云上实测）

### 0.1 源码层事实

| 项 | 事实 |
|----|------|
| RPC 绑定 | `-rpc` 参数可配置，默认 `control.DefaultAddr = "127.0.0.1:6689"` |
| 设计注释 | 「只做本机控制，不做远程钱包服务。默认绑定 127.0.0.1」 |
| 潜在暴露面 | `-rpc` 可被改成 `0.0.0.0:<port>`，届时 RPC 对外可达（依赖 UFW 兜底） |
| 鉴权 | mutation 端点 `requireAuth`（Bearer + constant-time + 固定延迟） |
| read 端点 | `/status` 等无鉴权（依赖 loopback 保护） |

### 0.2 云上实测事实

| 项 | 事实 |
|----|------|
| 三节点 RPC | 全部绑 `127.0.0.1:16689/16691/16693`（loopback），外部不可达 ✅ |
| SSH | 端口 2222，`PermitRootLogin prohibit-password`（仅 ed25519 密钥） |
| UFW 2222 白名单 | 仅 `115.191.60.241`、`223.67.152.38` 两个 Owner IP ✅ |
| UFW 其他端口 | 22/80/443/8888 等 `ALLOW Anywhere`（注意：非 RPC 端口） |

### 0.3 边界结论

- **当前 RPC 外部不可达**（loopback + UFW 无 RPC 放行），安全边界清晰。
- **唯一到达路径 = SSH 隧道**（2222 严格白名单 + 密钥认证）。
- **风险点**：若未来误把 `-rpc` 改成 `0.0.0.0`，且 UFW 放行对应端口，则 read 端点（无鉴权）会暴露——这是设计 IP allowlist 兜底的核心动机。

---

## §1 三方案比较：SSH Tunnel / IP Allowlist / Gateway

### 1.1 方案对比矩阵

| 维度 | A. SSH Tunnel | B. IP Allowlist（应用层） | C. Gateway（反向代理） |
|------|--------------|--------------------------|----------------------|
| 传输加密 | ✅ SSH 自带 | ❌ 明文 HTTP（需另加 TLS） | ✅ 可加 TLS |
| 主机认证 | ✅ SSH 密钥 | ❌ 无 | ⚠️ 需自建证书体系 |
| 用户身份 | ✅ root@ed25519 | ❌ 仅 IP | ⚠️ 需认证模块 |
| 可达性隔离 | ✅ 精确（仅隧道端口） | ⚠️ 依赖 RPC 绑定地址 | ✅ 可精细路由 |
| 审计归因 | ⚠️ 仅 SSH 日志 | ⚠️ 仅 IP | ✅ 可加 client 头 |
| 部署复杂度 | 🟢 零代码（现成 ssh） | 🟡 少量代码（allowlist 中间件） | 🔴 新增组件+运维 |
| 当前契合度 | 🟢 已在用（2222 白名单） | 🟡 需新增 | 🔴 过度设计 |
| 生产风险 | 🟢 最小（不改 RPC 绑定） | 🟡 需改 RPC 绑定才生效 | 🔴 新增攻击面 |

### 1.2 决策结论

**✅ 选择「A（SSH Tunnel 为主）+ B（IP Allowlist 兜底）」的混合方案，否决 C（Gateway）。**

**理由**：

1. **A 是当前事实路径**：开发机器已经通过 `ssh -L 16689:127.0.0.1:16689 ...` 触达云上 RPC，零额外组件、零代码改动、复用 2222 严格白名单。
2. **B 是必要的纵深兜底**：防止「未来误改 `-rpc` 为 `0.0.0.0`」时 read 端点无鉴权暴露。allowlist 作为应用层第二道闸，即使 RPC 绑定被误改，仍拒绝非白名单来源。
3. **C 过度设计**：当前仅 3 节点 loopback 集群 + 单开发机器测试场景，引入 Gateway（如 nginx/caddy）会新增组件、证书、运维成本与攻击面，收益不匹配。

### 1.3 混合方案的生效边界

```
形态 A（当前默认）：RPC 绑 127.0.0.1 → SSH 隧道 → allowlist 恒命中 127.0.0.1（无实际过滤作用，但保留）
形态 B（未来误改绑定）：RPC 绑 0.0.0.0 → allowlist 生效，拒绝非白名单 IP
```

> allowlist 在形态 A 下「形同虚设但无害」，在形态 B 下「关键兜底」。这正是纵深防御的价值：**不依赖单一假设**。

---

## §2 权限模型冻结

### 2.1 角色与权限矩阵

| 角色 | 通道 | 允许操作 | 禁止操作 |
|------|------|---------|---------|
| 生产管理员（Owner） | SSH 直接 + 生产 token | `/send` `/mine` `/mine/start` `/mine/stop` `/stop` | — |
| 测试矿工（开发机器） | SSH 隧道 + 测试 token | **仅 `/mine`**（按需出块） | `/send` `/mine/start` `/mine/stop` `/stop` |

### 2.2 冻结的权限边界（不可突破）

| 边界 | 值 |
|------|----|
| 测试通道端点白名单 | 仅 `/mine`（按需出块，`count ∈ [1,1000]`） |
| 测试通道禁端点 | `/send`（资金）、`/stop`（停节点）、`/mine/start`（持续挖矿）、`/mine/stop` |
| 测试 token 作用域 | 仅限测试通道，**不能**用于生产端点 |
| 生产 token 作用域 | 生产端点，**不受**测试通道开关影响 |

### 2.3 权限模型核心原则

> **最小权限 + 双 token 隔离**：测试矿工只拿到「出块」这一项能力，且凭据与生产凭据完全分离。任何越权调用（如尝试 `/send`）在测试通道下直接 403。

---

## §3 Token 生命周期设计

### 3.1 生命周期状态机

```
[生成] → [激活] → [使用] → [轮换/撤销] → [销毁]
```

| 阶段 | 操作 | 说明 |
|------|------|------|
| 生成 | 开发机器本地生成随机 token | `openssl rand -hex 32`（64 字符） |
| 落盘 | 写入云上独立文件 | `-test-mining-token-file /opt/p2pchain/secrets/test-mining-token`（0600） |
| 激活 | 节点启动时 `LoadTokenFile` 加载 | 复用现有归一化 + 长度区间 + 权限检查 |
| 使用 | 测试客户端携带 | `Authorization: Bearer <test-token>` |
| 轮换 | 覆盖 token 文件 | 无需重启（若设计为运行时重读）或需重启（若启动时加载） |
| 撤销 | 清空/删除 token 文件 + 关闭 `-test-mining-enable` | fail-closed，立即失效 |

### 3.2 生命周期决策

| 决策项 | 选择 | 理由 |
|--------|------|------|
| 是否有过期时间 | **无内置过期**（测试 token 短生命周期，靠人工撤销） | 测试通道非长期凭据，引入过期会增加复杂度 |
| 是否运行时重读 | **启动时加载**（与生产 token 一致） | 复用现有 `LoadTokenFile` 语义，避免新增热重载逻辑 |
| 轮换方式 | 覆盖文件 + 重启节点（或仅重启测试通道） | 简单、可预测 |
| 撤销方式 | 删除 token 文件 + `-test-mining-enable=false` + 重启 | 三重保险，fail-closed |

### 3.3 Token 隔离保证

| 保证 | 机制 |
|------|------|
| 测试/生产 token 不混同 | 独立文件、独立配置项、独立内存字段 |
| token 不泄露 | 绝不进命令行/环境变量/日志/审计（只记校验结果） |
| 泄漏可撤销 | 独立 token，轮换不影响生产 control-token |

---

## §4 审计模型设计

### 4.1 审计事件分类

| 事件类型 | 触发 | 记录内容 |
|----------|------|---------|
| `TEST_MINING_ACCEPTED` | 测试出块成功 | client_id, remote_ip, count, height_before/after |
| `TEST_MINING_DENIED_IP` | IP 不在白名单 | remote_ip, client_id |
| `TEST_MINING_DENIED_AUTH` | token 校验失败 | remote_ip（**不记 token**） |
| `TEST_MINING_REJECTED` | 业务拒绝（count 非法等） | remote_ip, reason |

### 4.2 审计字段（冻结）

```json
{
  "event": "TEST_MINING_ACCEPTED",
  "ts": "2026-10-02T11:13:00+08:00",
  "client_id": "dev-win-06uf28s",
  "remote_ip": "127.0.0.1",
  "action": "mine",
  "count": 3,
  "result": "accepted",
  "height_before": 137,
  "height_after": 140,
  "error": ""
}
```

### 4.3 审计模型决策

| 决策项 | 选择 |
|--------|------|
| 落盘位置 | 独立 `test-mining-audit.log`（与运行日志/生产日志分离） |
| 记录粒度 | 每次请求一条（含成功与所有失败类型） |
| 隐私红线 | **绝不记录 token 明文**；只记校验结果 + 来源 + 归因 |
| 与生产审计关系 | 测试通道有独立 `TEST_MINING` 前缀，可与生产日志过滤区分 |
| 保留策略 | append-only，与现有日志轮转对齐 |

---

## §5 最终架构决策文档（综合）

### 5.1 最终架构图

```
┌──────────────┐   SSH 隧道(2222, ed25519)    ┌─────────────────────────────┐
│ 开发机器      │ ───────────────────────────▶ │ 腾讯云节点 A (127.0.0.1)      │
│ (Win/CI)     │                              │  ├─ RPC 127.0.0.1:16689      │
│              │   curl -H "Bearer <test>"    │  │   ├─ IP allowlist (兜底)   │
│ 测试矿工 CLI  │   -H "X-Mining-Client: id"  │  │   ├─ 独立 test-token       │
│              │   POST /mine {count:3}       │  │   └─ TEST_MINING 审计日志  │
└──────────────┘ ───────────────────────────▶ └─────────────────────────────┘
```

### 5.2 冻结的架构决策清单

| # | 决策 | 冻结值 |
|---|------|--------|
| D1 | 接入方案 | SSH Tunnel 为主 + IP Allowlist 兜底，否决 Gateway |
| D2 | RPC 绑定 | **保持 127.0.0.1 不变**（不改为 0.0.0.0） |
| D3 | 端点范围 | 仅 `/mine`（按需出块） |
| D4 | 鉴权 | 独立 test-token（复用 requireAuth 语义） |
| D5 | 身份归因 | `X-Mining-Client` 头（审计用） |
| D6 | IP 白名单 | `-test-mining-allowlist`（默认空=关闭） |
| D7 | Token 生命周期 | 启动加载 + 手动轮换/撤销，无内置过期 |
| D8 | 审计 | 独立 `TEST_MINING` 日志，绝不记 token |
| D9 | 默认状态 | 全部 fail-closed（不配置即零变化） |

### 5.3 风险与缓解

| 风险 | 缓解 |
|------|------|
| 误改 `-rpc` 为 0.0.0.0 | allowlist 兜底（D6）+ UFW 不放行 RPC 端口 |
| 测试 token 泄漏 | 独立凭据 + 可轮换撤销（D7），不影响生产 |
| 测试活动不可追溯 | 审计日志 + client_id（D8） |
| 越权调用 /send | 端点白名单仅 /mine（D3），其他 403 |

---

## HARD STOP

本阶段为最终架构决策（只读设计），已完成。**未实现、未部署、未修改云节点。** 架构决策已冻结（D1~D9），后续实现需 Owner 单独授权，且实现范围严格限定在决策冻结范围内。
