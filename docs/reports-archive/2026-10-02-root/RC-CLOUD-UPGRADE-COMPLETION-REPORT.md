# PHASE P2PCHAIN — RC CLOUD UPGRADE COMPLETION REPORT
## 腾讯云三节点 RC binary 滚动升级完成报告

- **阶段**：`PHASE-P2PCHAIN-RC-CLOUD-UPGRADE-EXECUTION-1`
- **性质**：实际变更阶段（滚动升级），已完成并验证通过。
- **时间**：`2026-10-02 11:01 ~ 11:04 CST`
- **RC binary**：SHA256 `b26181173ba1ce8795a598f89c1fb5ee64f84c7cf5b2fbe0fa77709ad24877bb`（tag v0.9.0-rc1 / commit 2647ba0）

---

## 升级结果总览：✅ 成功

三节点已全部从旧版本 `c3cec3f3`（binary `a95df7c9...`）滚动升级到 RC（binary `b2618117...`），链状态、数据完整性、网络网格全部保持健康。

---

## 升级过程记录（before / after）

### 节点 C（先升级，非 seed）

| 项 | before | after |
|----|--------|-------|
| PID | 2872762 | 3044810 |
| binary SHA256 | `a95df7c9...` | `b2618117...` ✅ |
| height | 136 | 136 ✅ |
| tip | `0000e923...` | `0000e923...` ✅ |
| peers | 1（A） | 1（A）✅ |
| 备份 | — | `/opt/p2pchain/backups/pre-rc1-20261002-110227/` |

### 节点 B（其次）

| 项 | before | after |
|----|--------|-------|
| PID | 2873329 | 3045101 |
| binary SHA256 | `a95df7c9...` | `b2618117...` ✅ |
| height | 136 | 136 ✅（== A） |
| tip | `0000e923...` | `0000e923...` ✅ |
| peers | 1（A） | 1（A）✅ |
| 备份 | — | `/opt/p2pchain/backups/pre-rc1-20261002-110315/` |

### 节点 A（最后，seed 枢纽）

| 项 | before | after |
|----|--------|-------|
| PID | 2872772 | 3045383 |
| binary SHA256 | `a95df7c9...` | `b2618117...` ✅ |
| height | 136 | 136 ✅ |
| tip | `0000e923...` | `0000e923...` ✅ |
| peers | 2（B、C） | 2（B、C）✅ 恢复 |
| 备份 | — | `/opt/p2pchain/backups/pre-rc1-20261002-110339/` |

---

## 关键执行发现（重要，供后续运维参考）

### ⚠️ 三节点共享同一 binary 文件导致的 `Text file busy`

- 初次用 `cp` 原地覆盖 `/opt/p2pchain/bin/p2pchain` 时触发 `Text file busy`，因为 A、B 仍在运行该文件。
- **解决**：改用 `mv` 原子替换（先 `mv p2pchain → p2pchain.old-c3cec3f3`，再 `mv RC → p2pchain`）。
- **原理**：`mv` 仅替换目录项（inode 指针），不影响正在运行的进程持有的旧 inode；新进程启动时自然打开新 inode。
- **结果**：旧 binary 保留为 `/opt/p2pchain/bin/p2pchain.old-c3cec3f3`（`a95df7c9...`），回滚可用。

### 升级顺序验证

- 严格 C → B → A 顺序执行，每节点独立备份 + 停止 + 恢复 + 验证，无并行升级。
- 每节点验证通过后才进入下一节点，未触发任何失败停止。

---

## 最终验证（§4）

### Binary 身份（三节点一致）

| 项 | 值 | 结果 |
|----|----|------|
| SHA256 | `b26181173ba1ce8795a598f89c1fb5ee64f84c7cf5b2fbe0fa77709ad24877bb` | ✅ |
| vcs.revision | 空（无内嵌） | ✅ buildvcs=false |
| trimpath | true | ✅ |
| GOOS | linux | ✅ |
| GOARCH | amd64 | ✅ |
| CGO_ENABLED | 0 | ✅ |
| go 版本 | go1.27.0 | ✅ |
| 三进程 exe | 全部 → `/opt/p2pchain/bin/p2pchain` | ✅ |

### 链状态

| 节点 | height | tip | mining | peers |
|------|--------|-----|--------|-------|
| A | 136 | `0000e923...` | STOPPED | 2（B、C） |
| B | 136 | `0000e923...` | STOPPED | 1（A） |
| C | 136 | `0000e923...` | STOPPED | 1（A） |

- 三节点 height、tip 完全一致，共识健康。
- height 未变化（136），符合静态链预期；升级未触发 reorg / 分裂。

### 数据完整性（关键）

| 文件 | 升级前 | 升级后 | 结果 |
|------|--------|--------|------|
| A blocks.dat | `9035fb10...` | `9035fb10...` | ✅ 未变 |
| B blocks.dat | `ea733ece...` | `ea733ece...` | ✅ 未变 |
| C blocks.dat | `a1c606c2...` | `a1c606c2...` | ✅ 未变 |
| A wallet.json | `02f19c1d...` | `02f19c1d...` | ✅ 未变 |
| B wallet.json | `c36a44b3...` | `c36a44b3...` | ✅ 未变 |
| C wallet.json | `da734236...` | `da734236...` | ✅ 未变 |

> **升级全程未触碰任何数据文件**（blocks.dat / wallet.json hash 与升级前完全一致），无迁移、无 reindex、无 reset。

---

## 回滚路径（保留完整）

| 项 | 位置 |
|----|------|
| 旧 binary | `/opt/p2pchain/bin/p2pchain.old-c3cec3f3`（`a95df7c9...`） |
| C 数据备份 | `/opt/p2pchain/backups/pre-rc1-20261002-110227/` |
| B 数据备份 | `/opt/p2pchain/backups/pre-rc1-20261002-110315/` |
| A 数据备份 | `/opt/p2pchain/backups/pre-rc1-20261002-110339/` |

**回滚命令**（如需）：`mv /opt/p2pchain/bin/p2pchain.old-c3cec3f3 /opt/p2pchain/bin/p2pchain` + 重启三节点（启动命令见 `UPGRADE-EXECUTION-BASELINE.md`）。

---

## 最终判定

### 🟢 **GREEN — 升级成功完成**

三节点全部运行 RC binary（`b2618117...`），链状态一致（height=136、tip `0000e923...`）、数据完整（hash 未变）、网络网格健康（A↔B、A↔C）。无任何失败或回滚触发。

---

## 后续建议（需 Owner 单独授权，非本阶段范围）

1. **补 systemd 守护**：当前仍为 manual/nohup，崩溃不自动拉起（历史 F4 项）。
2. **健康观察**：建议观察一段时间确认 RC 版 orphan durability / console auth 在生产行为正常。
3. **清理旧 binary 副本**：`p2pchain.baseline-3e732888`、`p2pchain.baseline-b21c314a`、`p2pchain-o1`、`p2pchain.old-c3cec3f3` 是否保留由 Owner 决定。
