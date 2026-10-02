# RC-PUBLICATION-RETRY-REPORT.md — 发布重试报告（再次阻塞）

> **阶段**：PHASE-P2PCHAIN-RC-PUBLICATION-RETRY-1（重试发布 v0.9.0-rc1）
> **日期**：2026-10-02
> **结果**：❌ **发布仍未执行（remote 未恢复，按指令停止）**

---

## 前置条件检查结果

前置条件声明：**"remote 已恢复"** —— 但实际检查发现 **两个远端均未恢复**。

| 远端 | 状态 | 详情 |
|------|------|------|
| gitea `192.168.3.123:22` | ❌ 未恢复 | SSH 22 不可达，主机 ping 无响应 |
| github `p2p` git transport | ❌ 未恢复 | 仍报 `CONNECT tunnel failed, response 502` |

---

## §0 Remote Final Re-check 详细结果

| 检查项 | 结果 |
|--------|------|
| 本地 tag | `2647ba0`（未变，冻结正常） |
| 本地 HEAD | `2647ba0`（未变） |
| binary SHA256 | `71097357…`（未变） |
| gitea SSH 22 | ❌ 不可达 |
| github API | ✅ HTTP 200（API 正常，但 API ≠ git transport） |
| github git smart HTTP | ❌ `CONNECT tunnel failed, response 502` |
| 代理环境变量 | 仍为 `http://127.0.0.1:62808`（未变化） |

---

## 根因（与上一阶段完全一致，未解决）

1. **gitea**：`192.168.3.123` 主机离线或网络隔离（ping 无响应，SSH 22/2222 均不可达）。
2. **github**：本地代理 `http://127.0.0.1:62808` 无法建立到 github 的 CONNECT 隧道，导致 git smart HTTP 传输返回 502。

**关键区分**：github API（普通 HTTPS 请求，代理放行）可达 ≠ github git 传输（需 CONNECT 隧道，代理阻断）可达。发布 push 依赖的是后者，因此**实际不可用**。

---

## 决策：按指令停止

Owner 指令明确：**"如 remote 再次不可用，停止。"**

因此本阶段：
- ❌ 未执行 `git push main`
- ❌ 未执行 `git push v0.9.0-rc1` tag
- ❌ 未执行任何绕过手段
- ✅ 本地状态完全未变

---

## 当前状态（与上一阶段结束时完全一致）

| 项 | 值 |
|----|----|
| HEAD = tag `v0.9.0-rc1` | `2647ba0`（未变） |
| binary SHA256 | `71097357…`（未变） |
| 未推送 | 7 提交 + 1 tag |
| 本地 RC 资产 | 完好冻结 |

---

## 明确的阻塞项（需 Owner 实际解决后，发布才能继续）

| # | 阻塞项 | 具体解决动作 |
|---|--------|-------------|
| 1 | gitea 服务器离线 | 恢复 `192.168.3.123` 主机（开机 / 启动 sshd / 修复网络 / 确认端口） |
| 2 | 本地代理阻断 github git 传输 | 修复 `127.0.0.1:62808` 代理的 CONNECT 隧道能力，或更换网络环境 |

**注意**：以上两项是**环境/基础设施问题**，非代码问题。在它们真正解决之前，重复重试只会得到相同结果。

---

## HARD STOP

**发布仍未执行。** 前置条件"remote 已恢复"未满足，两个远端仍不可用，按 Owner 指令停止且不绕过。

本地 RC 资产（tag + binary + 文档 + SHA256SUMS）持续完好冻结，一旦远端真正恢复，即可立即完成发布，无需重新构建或修改任何资产。

—— PHASE-P2PCHAIN-RC-PUBLICATION-RETRY-1（再次阻塞终止）——
