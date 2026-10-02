# PHASE-P2PCHAIN-RELEASE-CANDIDATE-FREEZE-1 — 发布身份冻结报告

> **阶段**：发布身份准备（只读，禁止 push/tag/deploy/远端修改/code 变更）
> **日期**：2026-10-02
> **治理指令**：Owner 授权「PHASE-P2PCHAIN-RELEASE-CANDIDATE-FREEZE-1」，建立 RC 身份记录，不产生任何 git 变更。

---

## §1 版本身份（VERSION IDENTITY）

### 提交身份

| 字段 | 值 |
|------|----|
| HEAD 短 SHA | `3e6d44a` |
| HEAD 完整 SHA | `3e6d44a248013f814859cf4f5be6feb5bc57f492` |
| 完整 tree SHA | `d5065ee7714bea533674b6d723d7ec11187e47f9` |
| 提交信息 | `docs: synchronize release governance status metadata` |
| 提交者 | `p2pchain-baseline <baseline@p2pchain.local>` |
| 提交时间 | 2026-10-02 09:52:59 +0800 |
| 分支 | `main` |
| 建议版本标签 | `v0.9.0-rc1` |

### 创世参数（`internal/blockchain/genesis.go`）

| 参数 | 值 |
|------|----|
| `GenesisTimestamp` | `1700000000`（2023-11-14T22:13:20Z） |
| `GenesisMinerPubKeyHash` | `[20]byte{0,…,0,1}`（不可花费黑洞地址） |
| `CanonicalGenesisHash` | `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3` |

### 构建产物（Build Artifact）

| 字段 | 值 |
|------|----|
| 二进制 | `node.exe`（12,130,304 字节） |
| SHA-256 | `fb3d2b57dd8adbd02d3c6d501f42a2cfa3c076657694e35756967caf9307b717` |
| Go 版本 | `go1.27.0 windows/amd64` |
| module | `p2pchain`（`go 1.22` 声明，零外部依赖，无 go.sum） |
| 文件系统时间戳 | 2026-10-02 09:29 |

### ⚠️ 版本身份关键披露（时间戳与代码一致性）

- `node.exe` 文件系统时间戳为 **09:29**，而孤儿代码提交 `36f9630` 为 **09:30:30**。
- 但字符串特征核验证明：`node.exe` **已包含孤儿 checkpoint 代码**（含 `orphan_waiting` 2 处、中文日志「孤儿等待检查点」3 处、`ORPH` magic 7 处）。
- **结论**：`node.exe` 是在孤儿代码「工作树已完成、尚未 commit」时构建的，其**代码内容与当前 HEAD（`3e6d44a`）一致**（`3e6d44a` 相对 `36f9630` 仅文档变更，零 `.go` 改动）。
- **建议**：正式发布时**从干净 checkout 重新构建** `node.exe` 并重算 SHA-256，消除「时间戳早于提交」的歧义（见 §4）。

---

## §2 CHANGELOG-RC 内容（审计式，仅准备，不提交）

### v0.9.0-rc1 — Developer Node Release Candidate（非生产加密货币发布）

**新增（feat）**
- `36f9630` 孤儿块耐久与启动恢复：`orphan_checkpoint.go`（`<datadir>/orphan_waiting.bin` 检查点，原子写 temp+fsync+rename + SHA-256 校验 + fail-closed）；`main.go` D1 启动加载 + D2 关机落盘；`service.go` `prepareOrphanRestore` + `consumeRestorePending`。含 6 个测试文件（checkpoint/hook/durability/e2e/recovery_4b2/restore）。

**文档同步（docs）**
- `8410036` 协议真相同步：更新 `PROJECT-AI-CONTEXT.md` + `README.md` 的共识规则集（v1<2000 / v2@2000 / v3@3000 / MaxTargetBits=16 / MaxDifficultyBits=32 / NewRulesetInitialBits=27），新增 `docs/spec/ORPHAN-DURABILITY-SPEC-v1.md` + `STARTUP-RECOVERY-PLAN-1.md`。
- `3e6d44a` 治理状态元数据收口：修正 README + AI-CONTEXT + 2 个 spec 的「未提交/52fb464」陈旧引用为「已提交 36f9630/8410036」。

**已知限制（不宣称生产就绪）**
- 孤儿块体不持久化（仅 checkpoint 投影）；安全边界骨架 SKELETON；P2P 无 TLS/对端认证；完整孤儿 pool 重播策略未做。

---

## §3 链状态策略（CHAIN STATE POLICY）

### 判定：**A. 源码发布（source-only release）**

| 依据 | 说明 |
|------|------|
| 运行时数据被 gitignore | `run-a/`、`run-b/`、`*.dat`、`*.lock`、`wallet.json`、`orphan_waiting.bin` 全部**不入 Git** |
| HEAD 树仅含源码+文档 | 无任何 `blocks.dat` / genesis 序列化产物 / 链数据快照 |
| 创世块非预置文件 | genesis 由 `NewGenesisBlock()` 确定性生成，非预置二进制快照 |

**结论**：RC 是**纯源码 + 文档**发布，**不含链状态**。任何节点需从源码 `go build` 后自行从创世块开始挖/同步。

### 快照边界（若未来需 B 类可恢复链状态发布，需补充）

| 边界项 | 当前状态 | 若走 B 类需导出 |
|--------|----------|----------------|
| 创世块 | ✅ 确定性生成（代码内） | 可选导出 `NewGenesisBlock().Encode()` 字节 |
| 链数据 `blocks.dat` | ❌ 未入 Git（各 run-* 目录） | 需单独备份目标高度的 `blocks.dat` |
| 孤儿检查点 | ❌ 未入 Git | 需单独备份 `orphan_waiting.bin` |
| 钱包私钥 | ❌ 故意排除（安全铁律） | 需线下安全交接，**永不入 Git** |

---

## §4 构建复现计划（BUILD REPRODUCTION PLAN）

### 目标
从干净 checkout 重建 `node.exe`，使 SHA-256 可复现核对，消除当前时间戳歧义。

### 复现步骤（待 Owner 授权后执行，本阶段不执行）

```bash
# 1. 干净 checkout（分离 HEAD，避免工作树污染）
git clone . /tmp/p2pchain-rc-build
cd /tmp/p2pchain-rc-build
git checkout 3e6d44a248013f814859cf4f5be6feb5bc57f492

# 2. 构建
go build -o node.exe ./cmd/node

# 3. 复现核对
sha256sum node.exe
# 期望：与当前 node.exe 的 SHA-256 对比
# 当前值 = fb3d2b57dd8adbd02d3c6d501f42a2cfa3c076657694e35756967caf9307b717

# 4. 功能自检
./node.exe -version 2>&1 || ./node.exe status -rpc 127.0.0.1:6690
```

### 注意事项
- Go 版本锁定 `go1.27.0 windows/amd64`（不同 Go 版本可能产生不同二进制，影响 SHA-256 复现）。
- 若 SHA-256 不一致，需排查：Go 版本、GOOS/GOARCH、构建目录路径（Go 二进制可能内嵌绝对路径，`go build` 用相对/不同路径会影响哈希）。
- **可复现性的强保证**：`go build` 默认开启 `-trimpath` 前的路径内嵌，正式复现需统一构建目录或用 `-trimpath`。

---

## 结论

| 项 | 结果 |
|----|------|
| 版本身份 | ✅ 已记录（HEAD/tree/创世/二进制 SHA 全锚点） |
| CHANGELOG | ✅ 审计式内容已准备（不提交） |
| 链状态策略 | ✅ 判定为 **A. source-only release** |
| 构建复现计划 | ✅ 已提供流程（不执行） |
| 关键披露 | node.exe 时间戳早于孤儿代码提交，但内容与 HEAD 一致，建议正式发布前干净重建 |

**本阶段零变更**：无 code 改动、无 commit、无 push、无 tag、无 deploy、无远端修改。

—— PHASE-P2PCHAIN-RELEASE-CANDIDATE-FREEZE-1 结束 ——
