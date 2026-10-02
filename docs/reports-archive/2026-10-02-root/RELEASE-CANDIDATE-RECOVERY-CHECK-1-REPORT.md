# RELEASE-CANDIDATE-RECOVERY-CHECK-1 — 恢复检查报告

> **阶段**：只读验证 + 恢复准备（禁止 code/commit/push/tag/deploy/远端修改）
> **日期**：2026-10-02
> **治理指令**：Owner 授权「PHASE-P2PCHAIN-RELEASE-CANDIDATE-RECOVERY-CHECK-1」，为 Release Candidate 建立恢复检查点信息，不做任何远端变更。

---

## §0 HARD BASELINE

| 项 | 值 |
|----|----|
| HEAD（完整） | `3e6d44a248013f814859cf4f5be6feb5bc57f492` |
| 分支 | `main` |
| 脏状态 | 仅 `internal/blockchain/query.go`（CRLF 噪声，无实质改动）+ 大量未跟踪历史报告 |
| 未推送提交 | **3 个**（`36f9630` → `8410036` → `3e6d44a`） |
| 上游跟踪 | `gitea/main`，本地 `ahead 3` |

---

## §1 发布资产清单（RELEASE ASSET INVENTORY）

### 源码（16 个 Go 包，全部已跟踪）
`cmd/node`、`cmd/explorer`、`internal/{block, blockchain, blocktree, config, control, explorer, mempool, obs, p2p, pow, storage, transaction, txbuild, utxo, wallet}`

### 规范文档（spec，全部齐全 ✅）
| 类别 | 文件 |
|------|------|
| 共识规范 | `docs/CANONICAL-CONSENSUS-SPEC.md` |
| P2P 规范 | `docs/CANONICAL-P2P-SPEC.md` |
| RPC 规范 | `docs/CANONICAL-RPC-SPEC.md` |
| 确定性序列化 | `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` |
| 主设计 | `docs/MASTER-DESIGN.md` |
| 孤儿耐久 | `docs/spec/ORPHAN-DURABILITY-SPEC-v1.md` |
| 启动恢复 | `docs/spec/STARTUP-RECOVERY-PLAN-1.md` |
| 限制/安全边界 | `docs/LIMITATIONS.md`、`docs/SECURITY-BOUNDARY.md` |

### 创世 / 链身份资产（`internal/blockchain/genesis.go`）
| 参数 | 值 |
|------|----|
| `GenesisTimestamp` | `1700000000`（2023-11-14T22:13:20Z） |
| `GenesisMinerPubKeyHash` | `[20]byte{0,…,0,1}`（黑洞地址） |
| `CanonicalGenesisHash` | `00003d97 723c3ccc ec83a664 f5d22da6 …`（32 字节链身份锚点） |

### 数据库 / 运行时资产
- 存储 = 追加日志 `<datadir>/blocks.dat`（legacy `[u32 LE len][block.Encode()]` 格式）
- 孤儿检查点 = `<datadir>/orphan_waiting.bin`（magic `"ORPH"` + version + entryCount + SHA256）
- **运行时数据（`run-a/`、`run-b/`、`*.dat`、`wallet.json`）均被 `.gitignore` 排除，永不入 Git** —— 这是正确的发布边界，但意味着**恢复点不包含链数据，只有代码**。

### 构建物
- 二进制 `node.exe`（12.1 MB，2026-10-02 09:29 构建）**未跟踪**，恢复时需 `go build -o node ./cmd/node` 重建。
- 构建脚本：`scripts/{git-guard.sh, git-guard-test.sh, run-tests.sh, smoke-e2e.sh}`
- `go.mod` 存在，`go.sum` 不存在（零依赖，预期状态）。

### ⚠️ 缺失的发布组件（IDENTIFIED GAPS）

| # | 缺失项 | 影响 | 建议 |
|---|--------|------|------|
| G1 | **版本号文件**（无 `VERSION` / 无 tag） | 无法从代码二进制识别版本 | 打标 `v0.9.0-rc1` 时一并补 |
| G2 | **Release 清单/CHANGELOG**（无 `CHANGELOG.md`） | 发布说明缺汇总 | 若走正式发布需补 |
| G3 | **链数据快照**（无 genesis 区块序列化产物） | 恢复点只有代码，无链数据 | 如需可复现网络，需单独导出 genesis 块 |
| G4 | **鉴权凭据文件**（`audit-run/control-token` 等 3 个） | 故意不入 Git（安全铁律） | 发布时需单独线下交接，**绝不提交** |

---

## §2 远端恢复计划（REMOTE RECOVERY PLAN，仅文档，不连接）

### 当前远端状态（只读快照）

| 远端 | URL | 状态 | 诊断 |
|------|-----|------|------|
| `gitea` | `ssh://git@192.168.3.123:22/zxzjxx/wakuang.git` | ❌ **SSH 22 端口不可达**（主机 ping 通） | 根因待查：SSH 服务未起 / 端口变更 / 防火墙 |
| `origin` | `https://github.com/zxz88601151/p2p` | ⚠️ **仓库为空**（`ls-remote --heads` 无分支） | 若推 github 属全新初始推送 |

### 未来安全恢复/推送流程（待 Owner 授权后执行）

1. **gitea 路径**：先修复 `192.168.3.123` 的 SSH 服务（检查 `sshd` 是否运行、端口是否 22、防火墙规则），恢复后 `git push gitea main`。
2. **github 路径**：确认认证（HTTPS token 或 SSH key）→ 首次推送 `git push -u origin main`（建立上游跟踪）。
3. **恢复点校验**：推送后 `git ls-remote` 核对远端 HEAD == `3e6d44a248013f814859cf4f5be6feb5bc57f492`。

---

## §3 恢复点设计（RESTORE POINT DESIGN，仅信息，不打 tag）

### 本地 RC 恢复检查点（锚点哈希）

| 锚点 | 完整 SHA |
|------|----------|
| RC 最终提交 | `3e6d44a248013f814859cf4f5be6feb5bc57f492` |
| 文档真相同步 | `8410036ed7b610ac6e03d48dce5abdf20ef33643` |
| 孤儿耐久实现 | `36f963056ea06ccfd2d9b075c84a1a510dcd0ff0` |
| RC 完整 tree | `d5065ee7714bea533674b6d723d7ec11187e47f9` |

### 恢复操作（供未来灾难恢复，不现在执行）

```bash
# 从任意位置恢复到 RC 边界
git fetch <remote>
git checkout 3e6d44a248013f814859cf4f5be6feb5bc57f492   # 或 git reset --hard 该 SHA
go build -o node ./cmd/node                               # 重建二进制（node.exe 未跟踪）
```

### 关键提醒

- **恢复点 = 代码 + 文档，不含链数据**（`*.dat`/`wallet.json`/`orphan_waiting.bin` 均被 gitignore，需从原主机另行备份）。
- **未打 tag**：`3e6d44a` 是唯一 RC 锚点，一旦本地 reflog 丢失且未推送，此提交仅存在于本地 `.git`。
- **建议**：即使暂不 tag/push，也应至少保留本地 `.git` 目录完整，避免 `git gc` 误回收未引用的 RC 提交。

---

## 结论（CONCLUSION）

| 项 | 结果 |
|----|------|
| HEAD 基线 | ✅ `3e6d44a`，3 个未推送提交 |
| 发布资产 | ✅ 源码 16 包 + 7 个规范 + 创世参数 + 构建脚本齐全 |
| 缺失组件 | ⚠️ G1 版本号 / G2 CHANGELOG / G3 链数据快照 / G4 凭据（故意排除） |
| 远端状态 | gitea SSH 不可达、github 空仓库 |
| 恢复点 | ✅ 已记录 3 个提交 + 1 个 tree 的完整 SHA |

**本阶段未执行任何变更**：无 code 改动、无 commit、无 push、无 tag、无 deploy、无远端修改。

—— PHASE-P2PCHAIN-RELEASE-CANDIDATE-RECOVERY-CHECK-1 结束 ——
