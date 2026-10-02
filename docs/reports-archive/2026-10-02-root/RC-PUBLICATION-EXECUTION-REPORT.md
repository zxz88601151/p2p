# RC-PUBLICATION-EXECUTION-REPORT.md — 发布执行报告（阻塞）

> **阶段**：PHASE-P2PCHAIN-RC-PUBLICATION-EXECUTION-1（发布 v0.9.0-rc1）
> **日期**：2026-10-02
> **结果**：❌ **发布未执行（remote 不可用，按指令停止）**

---

## 发布目标（确认无误）

| 项 | 值 |
|----|----|
| 版本 | `v0.9.0-rc1` |
| tag | `2647ba0eb23a94a11aa71326e9b0686bbe1b78d8` |
| binary | `node-v0.9.0-rc1.exe` |
| SHA256 | `71097357531876b9975998d1d316f0888543a4aea1dcb9456c160df50e6352b1` |

---

## §0 Remote Final Check（结果：两个远端均不可用）

### gitea（`ssh://git@192.168.3.123:22`）

| 检查项 | 结果 |
|--------|------|
| 主机 ping | ❌ 无响应（0% 收到） |
| SSH 22 端口 | ❌ 不可达 |
| SSH 2222 端口 | ❌ 也不可达 |
| **结论** | 主机离线或网络隔离 |

### github origin（`https://github.com/zxz88601151/p2p`）

| 检查项 | 结果 |
|--------|------|
| 仓库存在性 | ✅ 存在（`zxz88601151/p2p`，public） |
| 仓库内容 | 空（`size: 0`，从未推送） |
| API 访问 | ✅ HTTP 200 |
| git smart HTTP 端点 | ❌ `CONNECT tunnel failed, response 502` |
| **根因** | 系统代理 `http://127.0.0.1:62808` 无法建立到 github 的 CONNECT 隧道 |

---

## 根因诊断

当前系统环境配置了本地代理：

```
HTTP_PROXY  = http://127.0.0.1:62808
HTTPS_PROXY = http://127.0.0.1:62808
```

该代理：
- ✅ 能转发普通 HTTPS 请求（所以 github API 返回 200）
- ❌ 无法建立 git smart HTTP 所需的 CONNECT 隧道（返回 502 Bad Gateway）

git 虽未显式配置 `http.proxy`，但会读取环境变量 `HTTPS_PROXY`，因此 git 的 push/ls-remote 操作被该代理拦截。

---

## 决策：按指令停止，不绕过

Owner 指令明确：**"如 remote 不可用，停止，不绕过。"**

因此本阶段：
- ❌ 未执行 `git push main`
- ❌ 未执行 `git push v0.9.0-rc1`
- ❌ 未执行任何代理绕过（如 `--no-proxy`、临时 unset 环境变量）
- ✅ 本地状态完全未变（tag/HEAD/binary 均保持冻结）

---

## 当前状态

| 项 | 值 |
|----|----|
| HEAD | `2647ba0`（未变） |
| tag `v0.9.0-rc1` | `2647ba0`（仍在本地，未 push） |
| binary SHA256 | `71097357…`（未变） |
| 本地领先远端 | 7 个提交 + 1 个 tag（均未推送） |

---

## 待修复项（需 Owner 决策，不本阶段执行）

| # | 阻塞项 | 建议 |
|---|--------|------|
| 1 | gitea `192.168.3.123` 主机离线/SSH 不可达 | 需恢复 gitea 服务器（开机/启动 sshd/修复网络） |
| 2 | github 被本地代理 502 阻断 | 需修复代理，或明确授权后使用直连 |

---

## HARD STOP

**发布未执行。** 两个远端均不可用，按 Owner 指令停止且不绕过。

本地 RC 资产完好冻结（tag + binary + 文档 + SHA256SUMS），随时可在远端恢复后重新执行发布。

—— PHASE-P2PCHAIN-RC-PUBLICATION-EXECUTION-1（阻塞终止）——
