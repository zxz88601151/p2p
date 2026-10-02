# PHASE F-6.1 — SECRET ARTIFACT INVENTORY

**PHASE F-6.1 — OWNER SCOPE RECONCILIATION (STRICT READ-ONLY)**
**Deliverable 3 of 3**

| 项 | 值 |
|---|---|
| 仓库 | `E:/wakuang/p2pchain` |
| 基线 HEAD | `4d892be355a38886a83c123faad915c9f3cd62e4` |
| 性质 | **STRICT READ-ONLY** — `.gitignore` **未修改**；未删除 / 未移动任何文件 |
| 本清单**不含任何凭据值** | 仅登记路径、大小、mtime、跟踪/忽略状态、用途 |

---

## §0 — 结论（TL;DR）

```
SECRET ARTIFACT COUNT = 3
ALL THREE:  tracked=NO   ignored=NO   ⇒  VISIBLE TO `git add`
VERDICT:    NEVER INCLUDE IN `git add`
```

> **任何 `git add -A` / `git add .` / `git commit -a` 都会把 3 个凭据文件暂存。**
> 提交**必须**使用显式路径白名单。

---

## §1 — 凭据产物清单

| # | PATH | tracked? | ignored? | size | mtime | purpose | temporary? | safe to commit? |
|---|---|---|---|---|---|---|---|---|
| **S-1** | `audit-run/control-token` | **NO** | **NO** | 33 B | 2026-09-27 08:51:13 | **AUDIT-FIX-RUN-MINING-1** 阶段本地节点（`-rpc 127.0.0.1:8789`）的 mutation auth token；用于 `node stop` / `mine` 的 Bearer 鉴权 | ✅ 临时（该阶段的一次性运行凭据） | ❌ **NO** |
| **S-2** | `f5-verify/control-token` | **NO** | **NO** | 64 B | 2026-09-27 12:59:02 | **F-5 REAL NODE VALIDATION** 阶段真实节点（`-rpc 127.0.0.1:8901`）的 mutation auth token | ✅ 临时（验证用） | ❌ **NO** |
| **S-3** | `gui-test/token` | **NO** | **NO** | 34 B | 2026-09-27 10:11:58 | **GUI 全功能测试**（`gui/_fulltest.py`）自建节点（`p2pchain/gui-test/`）的 token | ✅ 临时（测试用） | ❌ **NO** |

### 1.1 关键字段说明

| 字段 | 含义 | 判定依据 |
|---|---|---|
| **tracked?** | 是否已在 Git 索引中 | `git ls-files --error-unmatch <path>` |
| **ignored?** | 是否被 `.gitignore` 覆盖 | `git check-ignore -q <path>`（exit 0 = IGNORED） |
| **purpose** | 由 mtime + 所在目录 + 阶段报告推断 | 目录归属 + 阶段记录 |
| **temporary?** | 是否为一次性运行产物 | 与所在验证目录同生命周期 |
| **safe to commit?** | 是否可进入 Git | **全部 NO** |

---

## §2 — `.gitignore` 覆盖分析（只读）

当前 `.gitignore`（**未修改**）：

```
# Binaries
bin/  dist/  *.exe  *.test  *.out
# Coverage / profiles
coverage.out
# Local env / secrets
.env  .env.*
# Editor / OS
.idea/  .vscode/  .DS_Store
# Runtime node data / secrets (GOV-1)
/run-a/  /run-b/  *.dat  *.lock  wallet.json
```

### 2.1 覆盖判定矩阵

| 模式 | 覆盖对象 | 对 token 文件有效？ |
|---|---|---|
| `*.exe` | 二进制 | ❌ 不适用 |
| `*.dat` | 链数据 | ❌ 不适用 |
| `*.lock` | 目录锁 | ❌ 不适用 |
| `wallet.json` | 钱包私钥 | ❌ 不适用 |
| `/run-a/`、`/run-b/` | 取证归档 | ❌ 不适用 |
| `.env`、`.env.*` | 环境变量 | ❌ 不适用 |
| — | **token / secret 模式** | **❌ 完全缺失** |

### 2.2 实测（`git check-ignore`）

| 探测路径 | 结果 |
|---|---|
| `audit-run/control-token` | **NOT-IGNORED**（exit 1） |
| `f5-verify/control-token` | **NOT-IGNORED**（exit 1） |
| `gui-test/token` | **NOT-IGNORED**（exit 1） |
| `secrets/control-token`（节点默认路径） | **NOT-IGNORED**（exit 1） |
| `run-a/wallet.json` | IGNORED ✅ |
| `node.lock` | IGNORED ✅ |

> **⇒ 保护缺口**：`.gitignore` 对「钱包 / 链数据 / 二进制」有效，对
> **「mutation auth token」无效**。而 token 恰恰是**唯一能触发 mutation 操作**
> （`/mine`、`/stop`、`/send`）的凭据。

---

## §3 — 风险分级

| 风险 | 说明 | 等级 |
|---|---|---|
| **误提交** | `git add -A` 会暂存 3 个 token ⇒ 凭据进入 Git 历史 | 🔴 **HIGH** |
| **不可逆** | 一旦 commit 并 push，凭据进入历史；`git rm` 无法从历史中移除 | 🔴 **HIGH** |
| **时效性缓解** | 三者均为**本地回环、一次性**运行凭据，且对应节点进程已停止 | 🟡 MEDIUM |
| **`secrets/` 默认路径** | 节点默认 `secrets/control-token` 同样未被忽略 ⇒ 未来真实运行会再次暴露 | 🟠 MEDIUM-HIGH |

---

## §4 — 强制规则（MANDATORY RULES）

1. **`NEVER INCLUDE IN git add`** —— 3 个文件**永不**进入任何 `git add`，除非未来有明确 Owner 授权。
2. **禁止批量暂存**：**禁止** `git add -A` / `git add .` / `git commit -a` / `git add --all`。
3. **必须显式白名单**：提交时逐路径指定文件。
4. **提交前预检**（建议流程，非本阶段执行）：
   ```
   git diff --cached --name-only | grep -Ei 'token|secret|\.key|\.pem|\.env' && echo "ABORT" || echo "OK"
   ```
5. **`.gitignore` 修复须单独授权** —— 本阶段 **DO NOT MODIFY**。
   （注：修改 `.gitignore` 会使 tracked 修改数 36 → 37，破坏 F-5/F-6/F-6.1 基线。）

---

## §5 — 与既往 finding 的关系

| ID | 阶段 | 内容 | F-6.1 处置 |
|---|---|---|---|
| F-5-FINDING-1 | F-5 | 发现 `f5-verify/control-token` 未被忽略（**1 处**） | 证实 |
| F-6-FINDING-2 | F-6 | 实测 **3 处** token 未忽略 | 本清单逐文件登记 |
| **F-6.1-S-1…S-3** | **F-6.1** | **逐文件建档 + 强制规则** | **本清单** |

---

## §6 — Owner 决策接口

本清单对应 **OD-08**（见 `PHASE-F6.1-OWNER-SCOPE-DECISION.md`）。

```
OD-08  token files  |  secret risk  |  Candidate Phase: Governance  |  OWNER DECISION = PENDING
```

**建议默认（RECOMMENDED DEFAULT）**：保持现状（不改 `.gitignore`、不删除文件），
在**未来的 Governance 阶段**统一处理（`.gitignore` 增补 + 历史泄漏审计）。

---

## §7 — 边界声明

- 本清单**未**读取、**未**输出任何 token 值（仅大小 / mtime / 状态）。
- 本清单**未**修改 `.gitignore`，**未**删除 / **未**移动任何文件。
- 本清单**未**执行任何 Git 写操作。

**HARD STOP。**
