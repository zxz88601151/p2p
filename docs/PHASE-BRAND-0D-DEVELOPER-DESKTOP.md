# PHASE BRAND-0D.1 — DEVELOPER PRODUCT VALIDATION & PRODUCT CATEGORY CONVERGENCE

> 项目：P2PChain
>
> 前置阶段：PHASE BRAND-0D — DEVELOPER DESKTOP PRODUCT REPOSITIONING & ARCHITECTURE
>
> 前置结论：
>
> `PHASE BRAND-0D = PARTIAL`
>
> 当前发现：
>
> - Developer 方向成立
> - VERA / Humanitarian 方向正式废弃
> - P2PChain 的技术资产具有 Developer Infrastructure 潜力
> - Verifiable Ledger / Proof 是有价值的技术方向
> - 但目前尚未证明 Verifiable Ledger 应成为整个产品核心
> - Developer Desktop / Developer Workspace / Developer Control Center / Developer Infrastructure 四种产品类别尚未完成收敛
>
> 本阶段性质：
>
> **PRODUCT VALIDATION / CATEGORY CONVERGENCE / READ-ONLY RESEARCH**
>
> 本阶段不是编码阶段。
>
> 本阶段唯一目标：
>
> # 找到“全球开发者真正愿意使用”的产品类别，并确定 Verifiable Ledger / Proof 在产品中的正确位置。
>
> 完成后必须停止。
>
> **不得自动进入品牌命名。**

---

# §0 — ABSOLUTE RULES

## 0.1 READ-ONLY MODE

本阶段原则：

> **不修改任何现有代码。**

禁止：

```text
修改 .go
修改 .js
修改 .ts
修改 go.mod
修改配置
修改测试
修改数据库
修改 blocks.dat
修改 wallet
修改 P2P
修改 PoW
修改 UTXO
修改 Node
修改 API
修改 CLI
修改 UI
```

禁止重构。

禁止性能优化。

禁止安全修复。

禁止产品功能实现。

---

# §0.2 GIT 完全禁止

禁止：

```text
git commit
git push
git pull
git merge
git rebase
git reset
git restore
git checkout
git clean
git revert
git stash
git tag
```

允许只读：

```text
git status
git log
git show
git diff
git diff --stat
git rev-parse HEAD
git branch --show-current
git ls-files
```

---

# §0.3 CONCURRENT WORK PROTECTION

开始前必须执行：

```bash
git status --short
git branch --show-current
git rev-parse HEAD
git diff --stat
git diff --name-only
```










