# PHASE P2PCHAIN — RC CLOUD UPGRADE EXECUTION GATE
## 云节点 RC 升级执行前最终门禁报告

- **阶段**：`PHASE-P2PCHAIN-RC-CLOUD-UPGRADE-EXECUTION-READINESS-1` §5
- **性质**：只读门禁检查，未上传、未替换、未重启、未改配置、未改数据、未创建远端备份。
- **时间**：`2026-10-02 11:00 CST`

---

## 门禁检查总表

| # | 检查项 | 结果 |
|---|--------|------|
| G1 | 三节点 PID 已记录 | ✅ |
| G2 | 当前 binary SHA256 已记录 | ✅ `a95df7c9...` |
| G3 | 当前 binary build info 已记录 | ✅ vcs.revision=c3cec3f3 |
| G4 | height/tip/peers 已记录 | ✅ 136 / 0000e923 / 星型 |
| G5 | datadir 状态已记录 | ✅ 三目录 hash 固化 |
| G6 | 磁盘空间充足 | ✅ 30G 可用（23%） |
| G7 | 服务启动方式已确认 | ✅ manual/nohup（无 systemd） |
| G8 | 替换权限具备 | ✅ root（uid=0） |
| G9 | backup 目录可写 | ✅ root drwxr-xr-x |
| G10 | RC binary SHA256 匹配 | ✅ `b2618117...` |
| G11 | RC binary 架构正确 | ✅ ELF 64-bit x86-64 |
| G12 | RC binary 构建参数正确 | ✅ linux/amd64/CGO=0/trimpath/buildvcs=false |
| G13 | 原 binary 仍存在（回滚可用） | ✅ + 3 个历史副本 |
| G14 | 回滚命令明确 | ✅ 已从 ps 反推 |
| G15 | 升级顺序已定义 | ✅ C→B→A |

---

## §1 权限与路径（汇总）

| 项 | 值 | 判定 |
|----|----|------|
| 运行用户 | root（uid=0, gid=0） | ✅ 具备替换权限 |
| binary 权限 | `-rwxr-xr-x root:root` | ✅ |
| bin 目录 | `drwxr-xr-x root:root` | ✅ 可写 |
| backup 目录 | `drwxr-xr-x root:root` | ✅ 可写 |
| logs 目录 | `drwxr-xr-x root:root` | ✅ 可写 |
| secrets 目录 | `drwx------ root:root` | ✅ 权限收紧（token 保护） |
| data 目录 | `drwx------ root:root` | ✅ 权限收紧 |
| systemd 单元 | 无（manual/nohup） | ⚠️ 需手动重启 |

## §2 RC binary（汇总）

✅ 全部 PASS（详见 `RC-BINARY-DEPLOY-CHECK.md`）：
- SHA256 `b2618117...` 匹配，ELF 64-bit x86-64，GOOS=linux，CGO=0，buildvcs=false，确定性复验通过。

## §3 升级顺序（最终确认）

**C → B → A**（seed 枢纽最后），每节点升级后强制验证 6 项：

1. binary hash = RC hash `b2618117...`
2. process restart success（PID 存活）
3. RPC health（`/status` 返回 200）
4. height unchanged（= 136）
5. tip hash unchanged（= `0000e923...`）
6. peers recovery（B/C 重新连回 A）

> **失败自动停止**：任一节点任一验证项失败 → 立即停止后续节点升级，进入回滚评估。

## §4 回滚路径（汇总）

| 项 | 判定 |
|----|------|
| 原 binary 存在 | ✅ `/opt/p2pchain/bin/p2pchain`（升级前会 `cp -p` 为 `.pre-rc1`） |
| 历史 binary 副本 | ✅ baseline-3e732888 / baseline-b21c314a / o1 |
| backup 目录可写 | ✅ |
| 回滚命令明确 | ✅ 恢复 binary + 恢复 blocks.dat/wallet.json + nohup 重启 |
| 回滚后启动方式 | ✅ `nohup <binary> <原参数> >> <log> 2>&1 &` |

---

## 最终判定

### 🟢 **GREEN — 可以进入实际升级**

**依据**：全部 15 项门禁检查 PASS。升级执行路径、权限、依赖、回滚条件均已满足：

- 权限：root 全权，bin/backup/logs 目录可写。
- 依赖：RC Linux binary 已就绪且验证通过；Go 运行时静态链接（CGO=0）无系统库依赖。
- 回滚：原 binary + 历史副本 + 全量数据备份方案 + 明确触发条件齐备。
- 唯一需注意点（非阻断）：服务为 manual/nohup，升级/回滚均需手动重启，无自动守护。

---

## HARD STOP

本阶段为门禁检查，已完成并停止。**未执行、也将不执行**（未经 Owner 明确授权）：

- ❌ 上传 RC binary
- ❌ 备份（创建远端备份文件）
- ❌ 替换 `/opt/p2pchain/bin/p2pchain`
- ❌ 重启节点
- ❌ 节点验证

**下一步**：等待 Owner 明确授权进入 `PHASE-P2PCHAIN-RC-CLOUD-UPGRADE-EXECUTION-1` 后，才允许执行：上传 RC binary → 备份 → 替换 → 重启 → 节点验证（按 C→B→A 顺序）。
