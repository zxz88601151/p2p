# OWNER-VERIFICATION-GATE-AUDIT-1-REPORT

**审计对象**：本地仓库 `E:/wakuang/p2pchain`（main = `7cc1fc2`）
**审计时间**：2026-10-02 14:10 CST
**性质**：只读事实核查（read-only）。**未 push、未 merge、未构建、未触碰云节点。**
**结论一句话**：用户请求的两件事，一件**没有对象可做**（不存在 P0-6/P0-8 补丁），一件**没有证据可给**（无 SSH、无原始实测记录，且第 7 项"异地备份"不成立）。

---

## 第一部分：P0-6/P0-8 合并 —— ❌ 不存在该补丁，无物可 push

### 请求
> 把 P0-6/P0-8 合并版 push 到 GitHub，以便逐行验证 4 处 hunk 解冲突结果。

### 核查结果：**该工作区不存在任何 P0-6/P0-8 补丁/分支/提交**

| 核查项 | 命令 | 结果 |
|---|---|---|
| 全部 ref | `git for-each-ref` | 仅 `main` / `gitea/main` / `origin/main` / tag `rc1,rc2,rc3` —— 无 P0-6/P0-8 分支 |
| 全历史提交 | `git log --all --oneline` | 仅 `33e4fcb(P0-1)`、`433d82b(P0-2/P0-3)`、`9575863(P0-4)`、`0c5bc3a(P0-5)`；**无 P0-6、无 P0-8** |
| 全历史文本 | `git grep "P0-6" $(git rev-list --all)` | **0 命中** |
| 工作树文本 | `grep -rn "P0-6\|P0-8"`（全部 .go/.md） | **0 命中** |
| 补丁文件 | `find /e/wakuang -iname "*.patch" -o -iname "*.diff"` | 仅存在无关的 reorg/P1 补丁；**无 P0-6/P0-8 patch** |
| 文档编号分布 | `grep -rhoE "P0-[0-9]+"` | 仅 P0-1…P0-5（P0-4×5、P0-1×4、P0-3×3、P0-2×3、P0-5×1）；**不存在 P0-6/P0-8** |

### 关键点：用户担心的两个"接线"现场实际状态

| 接线 | 现场事实 | 判定 |
|---|---|---|
| P0-4 `--wallet-password-file` | **已在 main 中**：`cmd/node/main.go` L369 注释 + L390 `fs.StringVar(&nf.walletPassFile, "wallet-password-file", ...)`，来自提交 `9575863` | ✅ 已提交，**不存在"被合并弄丢"的风险**（因为根本没有合并） |
| P0-8 `--allow-non-loopback` | `grep -rn "non-loopback\|nonloopback\|NonLoopback" --include=*.go` → **0 命中** | ❌ 该接线在任何地方都不存在 |
| `docs/SECURITY-BOUNDARY.md` L10 | 仍为 **"mutation 5 条"**，无 "6 条" 变体 | ❌ 文档未发生 P0-8 改动 |

### 结论
- **dry-run 的 4 处 hunk 冲突无法在本仓库复现**，因为冲突的两个输入之一（P0-6/P0-8 补丁）不存在于本工作区。
- 因此**没有任何东西可以 push**：本地 `main`(`7cc1fc2`) == `origin/main`(`7cc1fc2`)，工作树仅有 1 个无关改动（`internal/blockchain/query.go`，CRLF 噪音）+ 若干未跟踪报告文件。
- 我不会**伪造一个合并提交**来"交差"。要推进，请提供真实的 P0-6/P0-8 补丁文件（或其所在分支/提交），我才能应用、解冲突并 push 供你逐行验证。

---

## 第二部分：云上升级验证证据 —— ⚠️ 无法提供实测证据（无 SSH），且第 7 项不成立

### 前提澄清
本会话**没有到 111.229.225.123 的 SSH 通道**，因此**无法执行 `md5sum /proc/<pid>/exe`、无法读云上日志、无法列云上文件**。
本工作区里所有关于云的信息，都来自**此前 AI 会话自行撰写的 markdown 报告 + `.workbuddy/memory/2026-10-02.md`**——是**叙述性声明**，不是原始捕获的终端输出。**不能把它们当成"我刚才的实测"。**

### 用户 7 项 vs 工作区记录（记录 ≠ 实测）

| # | 用户要验证 | 工作区记录（来源：memory §rc3 / RC-* 报告） | 证据强度 |
|---|---|---|---|
| 1 | 三节点钱包地址 迁移前=迁移后 | 声称"逐字一致"：A `NQ2aAF45CXSZ27LW29ocjXmxMy5KrHqtj4`、B `NWmgVrHgSop1sHX3kd6Yin4cXzS5fKP9pB`、C `NT8k32xTErQCEvg37cz8bfJsEMLVXkD3e8` | ⚠️ 仅叙述，无 before/after 原始捕获 |
| 2 | 三进程 exe hash = `5ab7d64a…` | `release/v0.9.0-rc3/SHA256SUMS` 确认 rc3 二进制 = `5ab7d64a…`；memory 称三节点均运行它 | ⚠️ 二进制哈希可信；但**无 `/proc/<pid>/exe` 原始输出**。另注：`5ab7d64a` 是 **SHA-256**，用 `md5sum` 校验会得到**不同的**摘要，命令与期望值本身不自洽 |
| 3 | 当前 height 与对端连接数 | 最近记录：height **137**、peers 星型 A↔B/A↔C | ⚠️ 仅叙述 |
| 4 | 日志有无 ERROR/PANIC | 声称 13:42 后零 ERROR/PANIC | ⚠️ 无原始日志片段归档 |
| 5 | 旧明文 wallet.json 三处已删 | 声称"旧明文 wallet.json 消失" | ⚠️ 无原始 `ls/find` 输出 |
| 6 | 新 `secrets/wallet.json` 三处存在 | 声称存在 | ⚠️ 无原始 `ls/find` 输出 |
| 7 | **口令文件已异地备份**（最关键） | 唯一记录：`/opt/p2pchain/backups/pre-rc3-20261002-134119/wallet-password` | ❌ **同机备份，非异地**。`grep "异地\|off-site\|offsite"` → **0 命中** |

### 第 7 项专项判定（最关键）
- 记录中的口令备份路径 **`/opt/p2pchain/backups/…` 与被加密的钱包在**同一台主机、同一文件系统**。
- **全仓库无任何"异地/off-site"备份记录。**
- 后果：**主机/磁盘一旦损坏，钱包口令不可恢复 → 三节点钱包全部不可解密。** 这是当前最严重的未闭合风险，**不满足**你"异地备份"的验证要求。

### 实际部署的是哪个二进制？
- 记录显示云上部署的是 **rc3 = `5ab7d64a…`（tag `v0.9.0-rc3` → commit `0c5bc3a`）**，其内容为 **P0-1…P0-5**。
- **不含任何 P0-6/P0-8**（因为不存在）。因此"云上跑了含 P0-6/P0-8 的本地合并版"这一担忧，按记录**不成立**——但**我无法用实测证明**，只能引用记录。
- ⚠️ 工作区报告之间存在**未完全对齐**的时间线：`RC-CLOUD-UPGRADE-COMPLETION-REPORT.md`(11:04) 称升级到 `b2618117`(rc1) 且"未触碰 wallet.json"，而 memory 后续记录 rc2→rc3 + 钱包迁移。二者是时间序列，但**报告文件与 memory 未做统一**，进一步说明这些是叙述而非可审计的原始证据。

### 结论
- 7 项中 **1–6 项无实测证据**（只有 AI 自述），**第 7 项明确不成立**。
- 我不会把自述当成实测交付。要真正闭环，需二选一：
  - **(A)** 你提供/确认 SSH 通道，我执行**只读**命令并把**原始输出**贴给你；
  - **(B)** 你自己跑下面这套只读命令（我给出命令范式）。

### 自验命令范式（只读，可直接复制）
```bash
# 1) 三节点钱包地址
for d in blockchain blockchain-b blockchain-c; do
  p2pchain wallet address --datadir /data/p2pchain/$d --password-file /opt/p2pchain/secrets/wallet-password; done
# 2) 三进程 exe 真实摘要（注意用 sha256，与 5ab7d64a 对齐）
for p in $(pgrep -f '/opt/p2pchain/bin/p2pchain'); do sha256sum /proc/$p/exe; done
# 3) height + peers（RPC 只读）
for r in 16689 16691 16693; do curl -s 127.0.0.1:$r/status; echo; done
# 4) 日志 ERROR/PANIC
grep -iE 'error|panic|fatal' /opt/p2pchain/logs/node*.log | tail -50
# 5) 旧明文钱包是否已删（应为空）
find /data/p2pchain -maxdepth 3 -name wallet.json -path '*/blockchain*'
# 6) 新 secrets 钱包是否存在
ls -l /data/p2pchain/*/secrets/wallet.json
# 7) 口令备份：先确认是否异地（若仅 /opt/... 则为同机）
ls -l /opt/p2pchain/backups/*/wallet-password; # 异地需另有一份在本机之外
```

---

## 总判定

| 事项 | 判定 |
|---|---|
| P0-6/P0-8 合并可 push | ❌ **无该补丁，无可 push 对象**（需你先提供真实补丁） |
| 云升级 7 项证据 | ❌ **无法提供实测**（无 SSH）；1–6 仅有 AI 自述；**7 异地备份不成立** |
| 云节点是否跑了含 P0-6/P0-8 的二进制 | 按记录**否**（跑的是 rc3/5ab7d64a = P0-1…P0-5） |

**给 Owner 的行动项**：① 提供 P0-6/P0-8 真实补丁；② 提供 SSH 通道或自行执行上述只读命令；③ **立即为钱包口令做一份异地备份**（当前为单点故障）。
