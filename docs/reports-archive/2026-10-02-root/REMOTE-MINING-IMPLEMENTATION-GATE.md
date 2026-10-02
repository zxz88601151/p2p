# PHASE P2PCHAIN — REMOTE MINING IMPLEMENTATION GATE
## 远程挖矿测试通道实现边界冻结门禁

- **阶段**：`PHASE-P2PCHAIN-REMOTE-MINING-TEST-IMPLEMENTATION-GATE-1`
- **性质**：实现就绪门禁（只读设计）。**No code changes / No cloud changes / No deployment / No commits。**
- **时间**：`2026-10-02 11:15 CST`
- **前置决策**：`REMOTE-MINING-TEST-ARCHITECTURE-DECISION-1`（D1~D9 已冻结）

---

## §0 HARD BASELINE（实现前基线）

### git 状态

| 项 | 值 |
|----|----|
| HEAD | `2647ba0eb23a94a11aa71326e9b0686bbe1b78d8` |
| v0.9.0-rc1 tag | `2647ba0`（= HEAD，一致） |
| 工作树 .go 改动 | **无**（仅 `query.go` 的 CRLF 换行符噪音，非实质改动） |
| 最近提交 | `2647ba0`（RC 发布）、`a69d24e`、`2fe8582` |

### binary 状态

| 项 | 值 |
|----|----|
| RC binary（本地 Windows） | `node-v0.9.0-rc1.exe` → SHA256 `71097357...` |
| RC binary（Linux 云部署） | `p2pchain-linux-amd64-v0.9.0-rc1` → SHA256 `b2618117...` |
| 构建参数 | `-buildvcs=false -trimpath` GOOS=linux GOARCH=amd64 CGO=0 |

### cloud 状态（腾讯云三节点）

| 节点 | height | tip | peers | mining | binary |
|------|--------|-----|-------|--------|--------|
| A | 137 | `0000a54c8e4c310f6cd8...` | 2 | STOPPED | `b2618117...` |
| B | 137 | 同上 | 1 | STOPPED | `b2618117...` |
| C | 137 | 同上 | 1 | STOPPED | `b2618117...` |

> 生产基线稳定：三节点 RC 共识一致（height=137、tip 一致），mining STOPPED，无异常。

---

## §1 FILE SCOPE FREEZE（文件范围冻结）

### 1.1 允许修改的文件（白名单）

| 文件 | 修改内容 | 性质 |
|------|---------|------|
| `internal/control/server.go` | 新增测试通道鉴权流水线（IP allowlist + 独立 token + 审计钩子） | 控制面 |
| `internal/control/server_test.go` | 新增测试通道单测 | 测试 |
| `cmd/node/main.go` | 新增 `-test-mining-*` 配置项 + 传入 nodeConfig | 入口 |
| `cmd/node/cli.go`（可选） | 测试矿工 CLI 子命令（或独立小工具） | 客户端 |

> 严格限定：仅上述 4 类文件。任何超出范围的文件改动一律 STOP。

### 1.2 禁止修改的文件（红线）

| 类别 | 具体文件/包 | 理由 |
|------|-----------|------|
| 共识逻辑 | `internal/blockchain/*`（除 test） | 涉及区块格式、genesis、难度 |
| 数据格式 | `internal/storage/*`、`blocks.dat` 序列化 | 涉及数据持久化 |
| 网络协议 | `internal/p2p/*`（除 test） | 涉及握手、消息结构 |
| 钱包 | `internal/wallet/*` | 涉及资金 |
| 挖矿核心 | `cmd/node/mining_*.go`、`service.go` 的挖矿逻辑 | 涉及 PoW 算法 |
| 孤儿持久化 | `cmd/node/orphan_checkpoint.go` | 涉及崩溃恢复 |

### 1.3 冻结断言

> 测试通道是**纯控制面增强**：只在 `requireAuth` 之前/之间插入一层鉴权，**不触碰** `handleMine` 的核心出块逻辑、不触碰 `node.Mine`、不触碰任何共识/数据/网络代码。

---

## §2 API CONTRACT FREEZE（API 契约冻结）

### 2.1 测试挖矿端点

| 项 | 冻结值 |
|----|--------|
| 端点 | **复用现有 `/mine`**（不新增端点） |
| 方法 | POST（复用 `requireMethod` 守卫） |
| 请求体 | 复用 `MineRequest{Count int}`，`count ∈ [1, 1000]`（`MaxMineCount`） |
| 响应 | 复用 `MineResponse` |

### 2.2 鉴权模型（冻结）

| 层 | 机制 | 失败响应 |
|----|------|---------|
| L1 IP allowlist | `-test-mining-allowlist`（CIDR/IP），`RemoteAddr` 匹配 | 403 `{"error":"forbidden"}` |
| L2 token | 独立 `-test-mining-token-file`，constant-time 比较 + `authFailureDelay` | 401 `{"error":"unauthorized"}` |
| L3 identity | `X-Mining-Client` 头（≤64 字符，仅审计归因） | 不拒绝，仅记录 |

### 2.3 授权边界（冻结）

| 端点 | 测试通道 | 生产通道 |
|------|---------|---------|
| `/mine` | ✅ 允许（测试 token + allowlist） | ✅ 允许（生产 token） |
| `/send` | ❌ **禁止** | ✅ 生产 token |
| `/stop` | ❌ **禁止** | ✅ 生产 token |
| `/mine/start` | ❌ **禁止** | ✅ 生产 token |
| `/mine/stop` | ❌ **禁止** | ✅ 生产 token |

> 关键约束：**测试通道只放行 `/mine`**，其他 mutation 端点即使带测试 token 也 403。测试 token 无法访问生产端点。

### 2.4 通道切换语义（冻结）

```
-test-mining-enable=false（默认）：
    /mine 走生产 requireAuth（行为与现状完全一致，零回归）

-test-mining-enable=true：
    /mine 走测试流水线（L1→L2→L3→handleMine）
    /send /stop /mine/start /mine/stop 仍走生产 requireAuth（不受影响）
```

---

## §3 TEST CLIENT DESIGN（测试客户端设计）

### 3.1 CLI 接口（冻结）

```
p2pchain test-mine \
  --rpc 127.0.0.1:16689 \          # 目标 RPC（经 SSH 隧道）
  --token-file ./test-mining-token \ # 独立测试 token 文件
  --client-id dev-win-06uf28s \      # 审计归因身份（可选）
  --count 3                          # 出块数量（1..1000）
```

| 参数 | 必填 | 默认 | 说明 |
|------|------|------|------|
| `--rpc` | 是 | 无 | 目标节点 RPC 地址 |
| `--token-file` | 是 | 无 | 测试 token 文件（读取后只在内存，不落日志） |
| `--client-id` | 否 | 空 | 审计归因身份 |
| `--count` | 否 | 1 | 按需出块数量 |

### 3.2 请求生命周期（冻结）

```
1. 读取 --token-file（trim + 长度校验，复用 LoadTokenFile 语义）
2. 构建 POST /mine，body {"count": N}
3. 设置头：Authorization: Bearer <token>；X-Mining-Client: <client-id>
4. 发送（超时 15s，复用现有 Client）
5. 解析响应：
   - 200 → 打印 height/出块结果
   - 403 → "IP 不在白名单"
   - 401 → "token 无效"
   - 400 → "count 非法"
   - 409 → "节点忙"
6. 退出码：0 成功 / 非 0 失败（便于脚本判断）
```

### 3.3 安全限制（冻结）

| 限制 | 值 |
|------|----|
| count 上限 | 1000（`MaxMineCount`，服务端强制） |
| 客户端超时 | 15s（复用现有 `Client`） |
| token 存储 | 仅内存，绝不写日志/环境变量/命令行回显 |
| 无持续挖矿 | 测试客户端**不支持** `/mine/start`（仅按需出块） |

---

## §4 SECURITY TEST PLAN（安全测试计划）

### 4.1 测试矩阵

| # | 测试项 | 预期结果 |
|---|--------|---------|
| T1 | 无 token 调用 `/mine`（测试通道启用） | 401 unauthorized |
| T2 | 错误 token 调用 `/mine` | 401 unauthorized |
| T3 | 正确测试 token + 非白名单 IP | 403 forbidden |
| T4 | 正确测试 token + 白名单 IP | 200 出块成功 |
| T5 | 测试 token 调用 `/send` | 403（端点白名单拒绝） |
| T6 | 测试 token 调用 `/stop` | 403 |
| T7 | 测试 token 调用 `/mine/start` | 403 |
| T8 | 缺失 X-Mining-Client | 出块成功，但审计记录 client_id 为空 |
| T9 | 超长 X-Mining-Client（>64） | 截断或拒绝（记录审计） |
| T10 | 测试通道关闭（enable=false） | `/mine` 走生产 requireAuth，零回归 |
| T11 | 生产 token 在测试通道启用下 | 仍正常（生产端点不受影响） |

### 4.2 特权分离测试（关键）

| 验证 | 方法 |
|------|------|
| 测试 token ≠ 生产 token | 测试 token 无法访问 `/send /stop /mine/start` |
| 生产 token 不受测试通道影响 | enable=true 时生产 token 行为不变 |
| 审计不泄露 token | 审计日志无 token 明文 |

### 4.3 回归测试（关键）

| 验证 | 方法 |
|------|------|
| 零回归 | enable=false（默认）时，所有现有 mutation/read 端点行为与实现前完全一致 |
| 现有测试通过 | `go test ./internal/control/ ./cmd/node/` 全绿 |

---

## §5 IMPLEMENTATION CHECKLIST（实现清单）

### 5.1 精确文件 + 函数（冻结，不得扩展）

| 文件 | 函数/改动 | 说明 |
|------|----------|------|
| `internal/control/server.go` | `Server` struct 新增字段：`testMiningEnabled bool`、`testMiningAllowlist []*net.IPNet`、`testMiningToken string`、`testMiningAudit io.Writer` | 状态字段 |
| | 新增 `SetTestMining(enabled bool, allowlist string, token string, audit io.Writer) error` | 配置注入 |
| | 新增 `testMiningGate(next http.HandlerFunc) http.HandlerFunc` | 测试通道流水线（L1→L2→L3） |
| | `Handler()` 中：`/mine` 注册改为条件包装（enable 时用 testMiningGate） | 路由接线 |
| | 新增 `parseAllowlist(string) ([]*net.IPNet, error)` | IP/CIDR 解析 |
| | 新增 `writeTestMiningAudit(...)` | 审计落盘 |
| `internal/control/server_test.go` | 新增 T1~T11 对应测试 | 安全测试 |
| `cmd/node/main.go` | `nodeFlags` 新增 `testMiningEnable bool`、`testMiningAllowlist string`、`testMiningTokenFile string` | 配置项 |
| | `newNodeFlagSet` 注册 3 个 flag | flag 定义 |
| | `nodeConfig` 新增对应字段 | 传递 |
| | `startNode` 读取配置并调用 `SetTestMining` | 装配 |
| `cmd/node/cli.go` | 新增 `test-mine` 子命令（或独立工具） | 客户端 |

### 5.2 禁止清单（no scope expansion）

| 禁止 | 说明 |
|------|------|
| ❌ 修改 `handleMine` 核心逻辑 | 只包一层，不改内部 |
| ❌ 修改 `node.Mine` / PoW 算法 | 挖矿核心不动 |
| ❌ 修改共识/区块/genesis | 红线 |
| ❌ 修改 `blocks.dat` 序列化 | 红线 |
| ❌ 修改 p2p 握手/消息 | 红线 |
| ❌ 修改 wallet | 红线 |
| ❌ 新增独立 Gateway/反向代理 | 否决（架构决策 D1） |
| ❌ 新增独立端口/独立 HTTP server | 复用现有 RPC listener |
| ❌ 引入第三方依赖 | 仅标准库（net、crypto/subtle、encoding/json） |
| ❌ 改动 `orphan_checkpoint.go` | 红线 |

### 5.3 实现顺序（建议，冻结）

```
1. parseAllowlist + SetTestMining（纯函数，先测）
2. testMiningGate 流水线（L1→L2→L3）
3. Handler() 路由接线（enable 条件）
4. main.go 配置项装配
5. test-mine CLI
6. 安全测试 T1~T11 + 回归测试
```

---

## 门禁结论

### 🟢 实现边界已冻结，可进入实现（待 Owner 二次授权）

- 文件范围、API 契约、鉴权模型、授权边界、客户端设计、安全测试、实现清单**全部冻结**。
- 红线明确：纯控制面增强，不触碰共识/数据/网络/钱包/挖矿核心。
- 默认 fail-closed：`-test-mining-enable=false` 时行为零回归。

---

## HARD STOP

本阶段为实现前门禁（只读设计），已完成。**No coding / No commits / No deployment / No cloud changes。** 后续进入实际实现（写代码）需 Owner 单独授权，且实现严格限定在 §5 冻结清单内。
