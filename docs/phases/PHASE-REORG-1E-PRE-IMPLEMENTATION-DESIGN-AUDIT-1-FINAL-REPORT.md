# PHASE REORG-1E-PRE-IMPLEMENTATION-DESIGN-AUDIT-1 — FINAL REPORT

**P2PChain — Persistent Block / Undo Journal / Active Tip / Storage Reorganization Safety Audit**

- 阶段性质：STRICT READ-ONLY / CONSENSUS SAFETY / STORAGE ARCHITECTURE / CRASH CONSISTENCY / REORG PRE-IMPLEMENTATION GATE
- 执行时间：2026-09-14 23:15–23:35 (GMT+8)
- 报告文件：`PHASE-REORG-1E-PRE-IMPLEMENTATION-DESIGN-AUDIT-1-FINAL-REPORT.md`
- 纪律：**零 .go 修改 / 零测试修改 / 零配置修改 / 零共识参数修改 / 零 git 写 / 零生产数据触碰 / 未启动 reorg / 未调用 SetTip**

---

## §0 VERDICT

```
VERDICT = READY FOR REORG-1E IMPLEMENTATION
```

- 无 BLOCKING-GRADE 未决项：REORG-1E 的最小实现范围在本报告中**已完整冻结**（F1–F8 设计冻结 + M1–M7 必实现清单 + X1–X9 禁实现清单）。
- 3 项**同阶段内必须闭合的缺陷**（不阻塞开工，但不得留到后续阶段）：SP-3、SP-3b、FC-007/B3。
- 1 项**升级登记的既有缺陷**：BT-1（blocktree flaky）本会话复现率由历史 2/6 恶化至 **3/4 FAIL**，`go test ./...` 现大概率红灯 —— 属 REORG-1A 遗留，不属 1E 范围，但会污染一切"全量测试绿灯"证据。
- 1 项**环境观察（非本阶段责任）**：`.123` 生产机上仍驻留 2 个 P2-SIGTERM 阶段的 disposable 挖矿进程（p2a:17890 / p2c:17894）。

---

## §1 HARD BASELINE

### 1.1 Git

| 项 | 值 |
|---|---|
| HEAD | `b001f578285a9ed1e5ee08a7067da8969d78bd60` (REORG-1B) |
| HEAD^ | `7d0e1e94aa562d52354bf7b062831d6d4f2b6b24` (REORG-1A) |
| HEAD~2 | `b2724c8d0fe155147341ea0f8474d11d6bcb282a` (DIFFICULTY-CONSENSUS-IMPLEMENTATION-1) |
| branch | `main` |
| `git log --oneline -3` | `b001f57` blocktree: add tree-level tip state and SetTip primitive (REORG-1B) → `7d0e1e9` blocktree: add BlockNode/BlockTree memory infrastructure (REORG-1A) → `b2724c8` consensus: implement difficulty and MTP rules |
| `git status --porcelain` | **21 项** = 2 M（docs）+ 19 ??（17 docs + 2 utxo .go + run-a/ + run-b/ + verifier/） |
| `git status --porcelain '*.go'` | **仅 2 项**：`?? internal/utxo/undo.go`、`?? internal/utxo/undo_test.go` → **零 .go 漂移（无任何 modified .go）** |
| tracked diff | 仅 docs：`M docs/DETERMINISTIC-SERIALIZATION-SPEC.md`、`M docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` |
| `git diff --check` | **非零退出**：`docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` 大量 trailing whitespace + EOF 空行（**pre-existing，纯 docs，与 .go 无关**） |

**REORG-1D 归属确认**：`internal/utxo/undo.go`(556 行) + `undo_test.go`(1002 行) 均为 **untracked 新增**，零修改既有文件，与 REORG-1D 报告一致。blocktree 4 文件已入库（`7d0e1e9`+`b001f57`），工作树零 `*.go` 未提交修改。

### 1.2 工具链与构建

| 检查 | 结果 |
|---|---|
| Go | `go1.22.12 windows/amd64`（托管于 `C:\Users\Administrator\.workbuddy\binaries\go\go\bin`） |
| module | `module p2pchain`（`go.mod` 前三行确认） |
| `go vet ./...` | **exit 0，静默 PASS** |
| `go build ./...` | **exit 0，静默 PASS** |
| `gofmt -l .` | 5 文件被标记：`cmd/node/lock_lifecycle_test.go`、`cmd/node/main.go`、`internal/blockchain/blockchain.go`、`internal/blockchain/difficulty_consensus_test.go`、`internal/pow/pow.go` |
| gofmt 性质判定 | **非 CRLF 伪影**（实测 4 个候选文件均为 LF-only）。`gofmt -d internal/pow/pow.go` 显示差异为 **Go 1.19+ 文档注释重排**（`//       ·` → `//     ·`）与少量尾部空行。全部为 **pre-existing（工作树未修改的已跟踪文件）**，本阶段零新增漂移。 |

### 1.3 测试基线

| 包 | 结果 |
|---|---|
| `go test -count=1 ./internal/storage/...` | **ok 0.976s** |
| `go test -count=1 ./internal/utxo/...` | **ok 0.125s** |
| `go test -count=1 ./internal/block/...` | **ok 0.250s** |
| `go test -count=1 ./internal/blockchain/...` | **ok 36.504s** |
| `go test -count=1 ./internal/blocktree/...` | **FAIL** — `TestI_RestartLikeReconstruction`（`blocktree_test.go:241: rebuild AddBlock error: blocktree: parent block not found`） |

**BT-1 升级登记（既有缺陷，非本阶段引入）**：`-run TestI_RestartLikeReconstruction` 连跑 4 次 → `FAIL / ok / FAIL / FAIL`（**3/4 失败**）。历史登记为"6 进程复跑 4 过 2 挂"（~33%），本会话恶化为 **75%**。根因（REORG-1B 报告已定位）：测试以 `range map` 顺序重建，而 `AddBlock` 要求父先存在 → map 迭代序随机决定成败。**影响**：`go test ./...` 现大概率红灯，将污染后续所有阶段的"全量测试绿灯"证据链。

### 1.4 生产 datadir 状态（只读采集，零写入）

主机 `192.168.3.123`（hostname `adminjiedian`，Ubuntu，up 1 day 9:53，load 0.14/0.07/0.02）：

| 项 | 值 |
|---|---|
| `~/p2pchain-longrun/datadir/` | `blocks.dat` 237,462 B（mtime Sep 14 15:04）· `node.lock` 42 B · `wallet.json` 388 B |
| `blocks.dat` SHA256 | `c780a57db62cf0b5dc07700cdf261fedc867cdc60f2f4a33eec25569b7832074` |
| `node.lock` | `pid=47785` / `started_at=2026-09-14T03:28:14Z` |
| 进程 | PID 47785 `node -datadir ~/p2pchain-longrun/datadir -listen 0.0.0.0:17880 -rpc 127.0.0.1:17881 -mine` |
| systemd | `p2pchain-node` = **active + enabled** |
| `/status` | `height=1284`，`tip_hash=000067fccbc6cefa5c1e84f689a372538144c37f370c1f5bad9bf51b41333a6d`，`mining_state=STALLED`，`mining_reason=subsidy-exhausted-no-fee-tx`，`bits=16`，`difficulty=1`，`accepted_blocks=24`，`rejected_blocks=0`，`peers=[]`，`mempool_size=0` |
| 二进制 SHA256 | `4d2345b83079f29a8acec95eed4b9587a2b6cf123c3271447e6a8200cf96afec`（= `674c5ad` 构建，与登记一致） |

**与上次登记的差异（必须更新）**：height **1275 → 1284**（+9，仍 STALLED，为 fee-tx 驱动的低速率出块）；其余全部一致。生产链处于 **激活高度 2000 之前**（1284 < 2000）→ 存量链 100% 走 legacy 规则，`bits≡16`、`version≡1`。

**环境观察（只读，未干预）**：同机仍驻留 **2 个 disposable 挖矿进程** —— PID 46521（`~/p2pchain-longrun/p2a`，17890/17891）与 PID 46563（`~/p2pchain-longrun/p2c`，17894/17895），来自 PHASE P2-SIGTERM-REMEDIATION 复现矩阵。生产实例（47785）与它们 datadir 隔离，无冲突；建议后续授权阶段清理。

### 1.5 回放成本实测（本阶段新增量化证据）

以生产 `blocks.dat` 的**只读副本**（本地 `C:\tmp\reorg1e-audit\blocks.dat`，SHA256 与生产一致 `c780a57d…`）运行 `node verify`（只读回放全量共识校验，`OpenFileBlockStoreReadOnly`，不取锁、不写盘）：

| run | 耗时 | 结果 |
|---|---|---|
| 1 | 746 ms | PASS |
| 2 | 670 ms | PASS |
| 3 | 655 ms | PASS |

`[verify] 已校验区块: 1285 个（高度 0 → 1284）`，创世 `0000aca1af720da3…`，链尾 `000067fccbc6cefa…`。

**推导（本报告核心量化依据）**：
- 1285 块 / 237,462 B → **184.8 B/块**（纯 coinbase 单链）
- 全链回放 = **~0.67 s** → **~0.52 ms/块**（含完整 7 步共识校验 + UTXO 重建）
- 线性外推：H=10k → ~5.2 s；H=100k → ~52 s；H=1M → ~8.7 min
- 结论：**"从创世回放重建 UTXO"在 1285 块规模成本可忽略**，但**按 H 线性增长**，不能作为 reorg 的主路径；作为**恢复/修复后备路径**是唯一确定性的权威手段。详见 §4/§7。

### 1.6 工作树残留（只读观察，未触碰）

- `p2pchain/run-a/`（blocks.dat 226,800 B + node.lock + wallet.json，Sep 14 23:05–23:06）
- `p2pchain/run-b/`（blocks.dat 72,180 B + node.lock + wallet.json，Sep 14 23:06）
- `p2pchain/verifier/`（Python 独立验证器：`p2pchain_verify.py` 27,680 B + 3 脚本 + TEST-VECTORS.md）
- `p2pchain/docs/` 17 个 untracked 报告
- `C:\c\`（空目录，非本阶段产生；本阶段因 Windows 路径转换误建 `C:\c\tmp\reorg1e-audit\` 已当场删除，仅余空壳 `C:\c`）
- 本地无 p2pchain 节点进程（178xx 端口无监听），`run-a/run-b` 为惰性遗留。

---

## §2 当前 Storage Architecture 全量审计

### 2.1 真实 storage state machine（逐状态落点）

```
Block creation (cmd/node/main.go:741 / candidate 模板)
    ↓
Validation (blockchain.validateBlock 7 步, blockchain.go:218-257)
    ↓
Block persistence (FileBlockStore.SaveBlock, storage/file.go:119-149)
    ↓
Chain metadata (无独立层 —— 见 §2.2)
    ↓
Active tip (= blocks[len-1], 隐式)
    ↓
Restart (AcquireDirLock → OpenFileBlockStore → loadIndex 全扫描)
    ↓
Recovery (NewBlockchainFromStore 逐块 applyBlock 回放, blockchain.go:88-132)
```

### 2.2 每一个状态实际保存在哪里

| 状态 | 载体 | 落盘？ | 重启后如何恢复 | 代码位置 |
|---|---|---|---|---|
| 区块字节 | `blocks.dat` 追加记录 `[u32 LE len][block.Encode()]` | **YES** | 顺序读 | `storage/file.go:21-27,136-144` |
| 区块高度 | **隐式** = 记录序号（文件中无高度字段） | 派生 | `height := len(s.byHeight)` 重算 | `storage/file.go:112` |
| hash→区块 | `byHash map[[32]byte]int`（**内存**，值为高度） | **NO** | 全文件扫描重建 | `storage/file.go:35,47,114` |
| height→区块 | `byHeight []*block.Block`（**内存，持有全部区块对象**） | **NO** | 全文件扫描 + 全量解码进内存 | `storage/file.go:34,113` |
| 规范链 | `Blockchain.blocks []*block.Block` | 间接（= 全部记录） | 逐块 `applyBlock` 回放 | `blockchain.go:51,120-130` |
| **活动链尾** | `blocks[len-1]`（**隐式，无独立持久化**） | 间接（= 末条记录） | 回放结束即为 tip | `blockchain.go:135-149` |
| UTXO 集合 | `Blockchain.utxo *utxo.UTXOSet` | **NO（永不落盘）** | 从创世全量回放重建 | `blockchain.go:52,65,252` |
| mempool | `mempool.Mempool` | **NO** | 空 | `cmd/node/main.go:122` |
| 钱包 | `wallet.json` | YES（tmp→`fsync`→rename 原子） | 加载 | `wallet/persist.go:45-95` |
| 目录所有权 | `node.lock`（`O_CREATE|O_EXCL`，内容 pid+started_at） | YES | 存在即拒绝启动（**无 stale-lock 自动恢复**） | `storage/datalock.go:47-67` |

### 2.3 五条关键结构性事实

1. **`blocks.dat` 的文件长度就是链高。** 没有独立的"元数据层"，没有独立的"活动链尾"持久化 —— **文件即链，末条记录即链尾**。
2. **索引永不落盘 → 「stale index」在结构上不可能存在。** 这是当前架构的一个**真实优点**，REORG-1E 必须保留这条性质（见 F5）。
3. **纯追加 + 位置即高度 ⇒ 无法表达分叉。** 任何非规范区块既被 `validateBlock` 拒绝（`PrevHash != tip.Hash()` → `ErrInvalidPrevHash`，`blockchain.go:223`），也没有可存放的位置。**FC-001/FC-007 在存储层的直接体现。**
4. **UTXO 永不落盘 ⇒ 崩溃一致性被大幅简化。** 唯一的持久真值是"区块记录序列"；UTXO 永远是**派生量**。代价是 O(H) 启动。
5. **存储层不提供任何完整性校验。** 记录无 magic/version/height/hash/checksum；唯一的防线是 `loadIndex` 的长度合法性（`0 < len ≤ 64 MiB`，`file.go:101-103`）与上层的**回放校验**（`blockchain.go:127-129`：任一块非法即拒绝启动）。

---

## §3 Block Persistence Model（9 问逐条，基于当前真实代码）

| # | 问题 | 结论 | 证据 |
|---|---|---|---|
| 1 | block 是否按 hash 存储？ | **否**。物理布局按写入顺序；hash 仅为**内存索引键**，且**值是高度而非偏移量** | `file.go:35` `byHash map[[32]byte]int`；`file.go:147` `byHash[hash] = expected` |
| 2 | block 是否按 height 存储？ | **隐式是**：记录序号 = 高度。文件中**没有** height 字段 | `file.go:112` `height := len(s.byHeight)` |
| 3 | 是否存在 index？ | **存在，但仅为内存索引**，启动时全量扫描重建（含**所有区块对象进内存**） | `file.go:82-117`；`file.go:113` `byHeight = append(byHeight, b)` |
| 4 | 能否通过 hash 定位任意 historical block？ | **会话内能**（内存 map，O(1)）；**磁盘层不能**（无 hash→offset 持久映射，必须整文件扫描） | `file.go:152-160` |
| 5 | 能否从 tip 向 parent 回溯？ | **能**：`GetBlockByHeight(h-1)` + `Header.PrevBlockHash`，每步 O(1) | `file.go:163-170`；`block.go:23` |
| 6 | 能否删除 detached branch？ | **不能**。无 `Delete*` API（FC-007）；且 detached branch 物理上无处可存 | `grep 'func.*Delete'`：storage 包 0 命中 |
| 7 | 能否 truncate canonical chain？ | **不能**。唯一删除手段是离线命令 `reset`（整文件删除） | `cli.go:578-700`，`cli.go:591` `resetDataFiles = {"blocks.dat","wallet.json"}` |
| 8 | 删除 block 后 metadata 是否同步更新？ | **不适用**（无删除能力）。但 `SaveBlock` 失败路径是**先落盘再改内存**（`blockchain.go:340-348`），方向正确 | `blockchain.go:341-347` |
| 9 | restart 后能否重建这些关系？ | **能**：hash 索引由扫描重建；链式关系由 `applyBlock` 逐块复验（`PrevHash == tip.Hash()`） | `blockchain.go:88-132` |

### 3.1 存储层缺陷登记（本阶段新增，必须由 REORG-1E 闭合）

> **SP-3（P2，隐式契约违反）** — `SaveBlock` 的文档注释声称"高度必须严格递增（等于当前高度 +1）"，但**代码未做任何高度连续性校验**。第 128-134 行只检查父哈希**是否存在**，不检查父高度。因此"把旧区块重挂到任意更高位置"在存储层可被接受，且会被**静默标上错误高度**（`file.go:147` 用 `expected = len(byHeight)` 作为新高度写进 `byHash`）。
> 证据：`file.go:119-134`（注释 vs 实现不一致）。
> 可达性：生产路径不可达（`blockchain.addBlock` 先经 `validateBlock` 强制 `PrevHash == tip.Hash()`）。**但 REORG-1E 将在此基础上扩展，必须先修。**

> **SP-3b（P2，旁路）** — 当 `PrevBlockHash == 零值` 且 `expected > 0` 时，整个父校验被短路（`if PrevBlockHash != zero && expected > 0`），**任何"伪创世"区块都可被无条件追加到链中任意高度**。
> 证据：`file.go:129` 的复合条件。
> 可达性：当前不可达（同一原因）。**REORG-1E 扩展此原语前必须修。**

> **SP-4（P2，= FC-007/B3）** — 存储层**零** Delete/Truncate/Compaction 原语；`os.Remove` 仅出现在 `node.lock` 与 `reset` 全删路径。

> **SP-5（P3，一致性语义弱化）** — 存储层不接受"未验证即存"的区块（无此接口），但也不提供"已存即已验证"的证明：读回时无 hash/checksum 比对，完整性完全依赖上层 replay。见 §10 的 `Detection` 列。

> **SP-6（P3，可扩展性）** — `byHeight` 持有**全部区块对象**（非仅偏移），内存 ≈ O(H × 块大小)。当前 1285 块可忽略；1M 块时该设计不可用。**属 DEFERRED，不属 1E。**

---

## §4 Undo Journal Persistence Design

### 4.1 REORG-1D 资产的精确边界（只读复核）

| 资产 | 位置 | 签名 | 本阶段结论 |
|---|---|---|---|
| `BlockUndo` | `utxo/undo.go:98-116` | `{Height int, Created []UndoEntry, Spent []UndoEntry, PreSetItemsCount int}` | 数据模型完备、可逆、自包含 |
| `UndoEntry` | `utxo/undo.go:119-122` | `{OutPoint OutPoint, Entry Entry}` | 69 B 定长 |
| `ApplyBlockWithUndo` | `utxo/undo.go:174-304` | `(base, txs, height) → (newSet, undo, fees, err)` | 两阶段（apply + derive），两阶段划分正确（同块内创建又消费的 OutPoint 被双重排除） |
| `DisconnectBlock` | `utxo/undo.go:326-390` | `(set, undo) → (set, err)` | 四阶段：校验→逆序删 Created→逆序恢复 Spent→`Len()==PreSetItemsCount` 完整性校验 |
| `EncodeUndo` | `utxo/undo.go:429-454` | `BlockUndo → []byte`（确定性 BE） | **无 magic / 无 version / 无 checksum / 无 blockHash 绑定** |
| `DecodeUndo` | `utxo/undo.go:486-529` | `[]byte → BlockUndo` | 仅结构校验，不校验排序、不校验与 set 的一致性 |
| corruption 常量 | `utxo/undo.go:125-145` | 4 个 `ErrUndoCorrupted*` + `ErrUndoOutOfOrderCreated` | 语义清晰，可复用 |

**EncodeUndo 字节布局（实测代码）**：`Height(i32) | PreSetItemsCount(i32) | nCreated(u32) | Created[n](69 B each) | nSpent(u32) | Spent[n](69 B each)`。
> **DOC-1（P3 文档缺陷）**：第 413-424 行的格式注释字段列表**遗漏 `PreSetItemsCount`**（代码有写、注释没写），但后续长度公式 `8 + 4 + len(C)*69 + 4 + len(S)*69` 恰好正确（前 8 B = 两个 i32）。建议在 1E 实现时顺带修正注释（**不得改动字节布局**）。

### 4.2 REORG-1E 应如何持久化 —— 设计冻结

```
blocks.dat (v2 扩展)     ← 区块字节 + 显式 height + 显式 blockHash + checksum
undo 记录（同一日志内）   ← BlockUndo 包装帧，按 blockHash 寻址
TIP 记录（同一日志内）     ← 提交点（commit point），显式 canonical tip 声明
内存: index（永不落盘）    ← 从上述日志扫描重建
```

**核心决策 D-A：单一交错日志（本报告推荐）**
`blocks.dat` 内交错三类记录：`BLOCK` / `UNDO` / `TIP`。每出一块：`write([UNDO][BLOCK][TIP])` 一次 → **一次 `fsync`**。**fsync 次数与今日完全一致（1/块）**，因此**不引入吞吐回退**（生产实测天花板 36.9 blk/s 由逐块 fsync 决定）。

**备选 D-B（不推荐）**：`undo.dat` + `tip.dat` 分文件 → 每块 3 次 fsync → 吞吐约降至 1/3。仅当将来需要独立 GC 时才值得。

**帧格式冻结（F1/F2）**

| 字段 | 长度 | 说明 |
|---|---|---|
| magic | 4 B | 字节 `'P','C','C','2'`。**选它的理由**：按 LE 读作 u32 = `0x32434350` = 843,268,944 **> 64 MiB**（legacy 上限）⇒ **v1 读到此值必判"非法记录长度"并拒绝，绝不误解析** —— 这是零修改 legacy 前缀即可安全扩展的关键 |
| version | 1 B | 记录帧版本 = `2` |
| type | 1 B | `1=BLOCK`、`2=UNDO`、`3=TIP` |
| reserved | 2 B | 必须为 0（前向兼容） |
| payloadLen | 4 B | payload 字节数 |
| height | 4 B | **显式高度**（消除"位置即高度"的脆弱性） |
| blockHash | 32 B | **显式区块哈希**（寻址主键 / 绑定键） |
| payload | var | BLOCK: `block.Encode()`；UNDO: `utxo.EncodeUndo()` 原样字节；TIP: 见下 |
| checksum | 32 B | `SHA256(magic‖version‖type‖reserved‖payloadLen‖height‖blockHash‖payload)` |

帧头固定 **48 B** + checksum **32 B** = 80 B 开销。

**TIP payload**：`chainwork(u256 BE, 32 B) | tipHeight(4) | reserved(4)` = 40 B。
**legacy 兼容规则（F1）**：文件前段若为 legacy 记录（`[u32 len][payload]`，`len ≤ 64 MiB`），按 v1 语义读为**区块**，其高度 = 序号，其 hash = 解码后重算；隐式提交语义 = "legacy 前缀全部为已提交规范块，其 tip = 最后一条 legacy 块"。首次进入 v2 记录后，规范链完全由 `TIP` 记录驱动。**存量 237,462 B 生产文件因此字节级可读、零迁移、零重写。**

### 4.3 §4 十问逐条回答

| # | 问题 | 冻结答案 |
|---|---|---|
| 1 | undo 按 height 存还是按 block hash 存？ | **按 blockHash 存**。每条 UNDO 记录携带 `blockHash`（帧头）+ `height`（帧头，且必须等于 `undo.Height`，冗余校验）。height **不是唯一键**（同高度多分叉），不可作主键。 |
| 2 | 如何避免同 height 不同 fork 的 undo 冲突？ | 由哈希寻址天然消解：两条同高度但不同哈希的 UNDO 是**两个不同键**，可共存。索引维护 `hash → (blockOffset, undoOffset, height)`。 |
| 3 | reorg 后如何找到旧 branch 的 undo？ | 沿被断开分支**从新 tip 向共同祖先逐块**取 `hash`，逐块命中 UNDO 记录（O(D) 次查找，D = reorg 深度）。共同祖先计算属 REORG-1C。 |
| 4 | restart 后如何恢复 detached branch？ | **前提**：被断开的区块必须物理存在于日志中（由 F1/F2 的哈希寻址提供，1E 使"分叉块可存储"成为可能）。重启后扫描重建完整索引（含非规范块），再由 REORG-1C 的 fork-choice 重判。**若 1E 仅存规范块，则 detached branch 在重启后不可恢复 —— 这是 1E 的范围红线（见 §9/Q4 判定）。** |
| 5 | undo 与 block 是否必须一一绑定？ | **必须，且只能按 hash 绑定**（不能按 height）。绑定强度分三层：①帧头 `blockHash` 存在；②`checksum = SHA256(…‖blockHash‖undoBytes)`；③语义校验 `undo.Height == node.Height`。 |
| 6 | 如何验证 undo 对应正确 block？ | 四层递进：**(a)** 帧 checksum；**(b)** `SHA256(blockHash ‖ EncodeUndo(undo))` 与 checksum 比对（防"错块配错 undo"）；**(c)** `undo.Height == block.Height`；**(d)** 语义级 —— `DisconnectBlock` 的四阶段校验 + `Len()==PreSetItemsCount`；**(e) 最强**（仅可疑时启用）：以该块父哈希为终点的从创世/检查点重放，逐 outpoint 比对（`assertSetEqual` 级严格比较，O(H)）。 |
| 7 | 是否需要 checksum / hash / magic / version？ | **全部需要**：magic（隔离 legacy 与 v2 + 抗误解析）、version（格式演进）、显式 height + blockHash（寻址与绑定）、checksum（自校验、可精确定位损坏记录）。 |
| 8 | corruption 如何被发现？ | 分四层：**记录层**（magic 非法 / version 未知 / reserved 非 0 / checksum 不匹配 / 长度超限 / 截断）→ **解码层**（`block.DecodeBlock` / `DecodeUndo` 结构错误）→ **绑定层**（`recordHash != block.Header.Hash()`、`undo.Height != height`）→ **语义层**（replay 的 7 步校验 / `DisconnectBlock` 的 4 个 `ErrUndoCorrupted*`）。**任何一层失败一律 fail-stop 或显式降级，绝不静默继续。** |
| 9 | partial write 如何被发现？ | ①帧头未满 48 B → 尾部不完整；②`payloadLen` 声明 N 但可读 < N → `io.ReadFull` 失败；③checksum 缺失/不匹配。**结构性限制（必须承认）**：纯追加日志无法区分"记录 A 被截断后记录 B 被追加"与"记录 A 本身损坏"—— 因此恢复策略必须是**前缀严格（prefix-strict）**：日志只承认"到最后一个完整且 checksum 合法的记录"为止，其后一切视为**未提交尾部**。 |
| 10 | crash 在 block write 与 undo write 之间会发生什么？ | **由写入顺序决定，这是本报告最重要的设计决策之一（F4）**。冻结为 **`UNDO → BLOCK → TIP`**：<br>· crash 在 UNDO 后、BLOCK 前 → 悬空 UNDO：**惰性垃圾**，无害（其 blockHash 无对应块，索引不建条目）。<br>· crash 在 BLOCK 后、TIP 前 → 区块已落盘但**无 TIP ⇒ 未提交**：重启后该块**不得成为规范块**，规范链回落到上一条 TIP。<br>· crash 在 TIP 写入中途 → TIP 记录不完整 ⇒ 同上回落。<br>**反向顺序（BLOCK → UNDO）被否决**：会留下"已落盘但永远无法断开"的毒化区块（无法安全回滚）。 |

### 4.4 成本核算（实测数据推导）

| 方案 | 每块写放大 | fsync/块 | 1285 块日志大小 | 1M 块外推 |
|---|---|---|---|---|
| 今日（仅区块） | 184.8 B（实测） | 1 | 237,462 B（实测） | ~185 MB |
| **1E D-A（UNDO+BLOCK+TIP 同一日志）** | ~185 + 133 + 120 = **~438 B** | **1（不变）** | ~563 KB | ~438 MB |
| 1E D-B（三分文件） | 同上 | **3（吞吐降至 ~1/3）** | 同上 | 同上 |

UNDO 体积公式：`8 + 4 + 69×|Created| + 4 + 69×|Spent|` B。纯 coinbase 块：`|Created|=1, |Spent|=0` → **85 B** + 帧 80 B = 165 B（含帧）。含 N 笔普通交易的块：`|Created| ≈ 输出总数`，`|Spent| = N` → 线性增长。

**结论**：D-A 在**零 fsync 回退**的前提下获得完整 reorg 回滚能力，体积代价 < 2.4×。**无需引入任何第三方数据库**（明确否决 `storage/storage.go:67-73` 的 bbolt/leveldb 建议 —— 既违反"禁第三方依赖"硬边界，在本规模也完全无必要）。

---

## §5 Crash Consistency Audit

### 5.1 当前架构的 crash matrix（真实评估，非设计理想）

| Crash Point | 当前实际行为 | 判定 |
|---|---|---|
| before block write | 无状态变化 | **PASS** |
| **during block write** | 文件尾部留下**部分记录** → 下次 `loadIndex` 在 `io.ReadFull` 处短读 → `ErrCorruptStore` → `OpenFileBlockStore` 返回错误 → `newNodeRuntime` 失败 → **节点永久无法启动** | **FAIL（P1）**。无任何修复路径：`verify` 只读报告 FAIL；`reset` 会删除全部数据（含钱包）。 |
| after block write before undo write | **不适用**（今日无 undo 落盘、无独立元数据写） | N/A |
| during undo write | **不适用**（无 undo 落盘） | N/A |
| after undo before active tip | **不适用**（tip 隐式 = 末条记录） | N/A |
| **during active tip update** | **不存在独立的 tip 更新** → 撕裂点只可能是"尾部部分记录" | **结构简单，但语义缺失**：无法表达"落盘但未提交" |
| after active tip update | 无独立更新 ⇒ 无该窗口 | N/A |

### 5.2 判定

> **当前 storage 架构不能安全支持 §5 所要求的模型。**

原因有三，且**互不重叠**：
1. **无提交点（commit point）**：规范链完全由"文件长度"隐式决定，因此不存在"已写入但未提交"这一状态 —— 而 reorg 的本质恰恰需要区分二者（分叉块"已存储"但"非规范"）。
2. **无 undo 落盘**：disconnect 所需的反向信息仅在内存，节点重启即失。
3. **尾部部分写 = 不可恢复**：crash 在写入中途的直接后果是**节点无法启动**，且没有"仅丢弃尾部字节"的手段。

### 5.3 最小必要基础设施（明确"最小"边界）

必须且仅需以下 5 项（全部 stdlib，零第三方依赖）：

| # | 基础设施 | 解决什么 | 复杂度 |
|---|---|---|---|
| I1 | **记录帧（magic/version/type/len/height/blockHash/checksum）** | 损坏可定位、legacy 可共存、哈希可寻址 | 中 |
| I2 | **UNDO 记录（哈希寻址 + 三重绑定）** | disconnect 信息持久化 | 低（复用 `EncodeUndo`） |
| I3 | **TIP 记录 = 提交点** | 显式规范链，区分"已存"与"已提交" | 低 |
| I4 | **尾部未提交记录检测 + 有界尾部修复** | 消除"crash 即砖化" | 中（唯一允许物理截断的场景） |
| I5 | **索引重建（已存在，需扩展为 hash→offset 三元组）** | 永不落盘 ⇒ 永不过期 | 低 |

**明确不引入**：bbolt / leveldb / badger / WAL 库 / 任何依赖。
**论证**：单日志 + 单 fsync 已满足 1E 全部需求；日志规模 <1 MB（当前）/<500 MB（1M 块）；校验和 + 前缀严格语义足以定位并隔离损坏。引入键值库需要重写索引层、改变 datadir 布局、增加依赖，收益为零。

---

## §6 Delete / Truncate Semantics（B3 / FC-007 专章）

### 6.1 语义冻结

> **总原则：追加日志上的所有 reorg 操作都是「逻辑操作」（标记/墓碑/提交点移动）；物理字节截断**只**允许用于「未提交尾部的修复」。**

```text
DeleteBlock(hash)        // 单块逻辑删除
DeleteBranch(fromHash)   // 逻辑删除 fromHash 及其全部后代
TruncateFromHeight(h)    // 逻辑回退规范链到高度 h-1
```

| 原语 | 精确定义 | 前置条件 | 副作用 |
|---|---|---|---|
| `DeleteBlock(hash)` | 将 `hash` 标记为 **dead**（索引 tombstone），并从索引中移除其可达性；**不删除任何字节** | `hash` **不得**位于当前规范链上（须先 `TruncateFromHeight`） | 索引 tombstone；**不**改 TIP |
| `DeleteBranch(fromHash)` | 对 `fromHash` 子树中所有节点执行 `DeleteBlock`（自叶向根或自根向叶均可，**顺序有确定性**） | 子树与规范链不相交 | 索引 tombstone 集合；UNDO 记录**保留**（见下） |
| `TruncateFromHeight(h)` | ①定位高度 h 的规范区块 `b_h`；②对规范链上所有 `height ≥ h` 的块执行 `DeleteBlock`；③追加一条 **TIP 记录**，其 `blockHash = b_{h-1}.Hash`、`chainwork = cumulative(b_{h-1})`、`tipHeight = h-1` | `h ≥ 1`；`h ≤ tipHeight+1`；`b_{h-1}` 必须存在且其 UNDO 必须存在（若需回滚 UTXO） | **唯一的规范链变更入口**；**提交点前移** |

### 6.2 八问逐条回答

| # | 问题 | 冻结答案 |
|---|---|---|
| 1 | 删除什么？ | **逻辑删除该块的索引可达性与（可选）。UNDO 记录是"保留"还是"标记 dead"取决于是否为该块保留复活能力。冻结：标记 dead，不物理删。** |
| 2 | 保留什么？ | 保留全部**物理字节**（区块 + UNDO）；保留**规范链共同祖先及以上**；保留 `wallet.json` 与 `node.lock`。 |
| 3 | index 如何同步？ | 索引**永不落盘**（F5），因此不存在"同步"问题 —— 索引是 TIP 记录 + 日志扫描的**纯函数**。墓碑集合同样由 TIP 记录 + 可达性重算得到。**这是"stale index"被结构性消灭的机制。** |
| 4 | undo 如何同步删除？ | 与区块**同生共死但不物理删**：物理字节保留（append-only），逻辑上随其区块的 tombstone 一起失效。**禁止**"删了 block 却留着可被误用的 undo"：索引层不暴露 dead 块的 undo；任何按 hash 取 undo 的接口必须先校验该 hash 不在 tombstone 集合中。 |
| 5 | active tip 如何同步更新？ | **只能通过追加一条 TIP 记录**（唯一入口）。禁止原地改写 TIP。 |
| 6 | 删除失败如何处理？ | **fail-stop**：部分删除**不允许**提交 —— 墓碑集合先在内存构建并全量校验（全部 hash 存在、均不在新规范链上），全部通过后才追加单条 TIP 记录 + 一次 fsync。fsync 失败 → 返回错误，内存状态回滚到操作前（**绝不留下半删除**）。 |
| 7 | partial delete 如何恢复？ | 由**前缀严格**语义天然覆盖：TIP 记录是原子提交单元（单条记录 + checksum）。crash 在 TIP 前 ⇒ 整个删除操作**未发生**；crash 在 TIP 后 ⇒ 整个操作**已完成**。**不存在中间态**（这正是"提交点"的设计目的）。 |
| 8 | historical branch 是否必须保留？ | **必须保留**（物理），否则 REORG-1H 的 P2P 分支同步与 reorg 后再 reorg 均无数据可用。物理回收（compaction/GC）属 **DEFERRED**，且必须由显式离线命令触发（`--compact`），绝不在节点运行时发生。 |
| 9 | orphan branch 与 detached branch 如何区分？ | **orphan** = 父未知，无法接入树（索引中不建档，仅暂存于有界 orphan 缓存，属 REORG-1G）。**detached** = 父已知、块合法，但**不在当前规范链上**（索引建档，标记 `IsCanonical=false`）。判定方法：从候选块沿 `BlockNode.Parent` 上溯至根，若路径与当前规范链的哈希集合相交于某祖先，则为 detached 而非 orphan。 |

### 6.3 明令禁止的失败模式

> **严禁**实现"只删 block 字节，却留下 stale index / stale undo / stale tip"。本设计中该失败模式在**结构上不可构造**，因为：索引不落盘（无法 stale）、UNDO 不物理删（无法 stale-leak）、TIP 只追加（无法被局部改写）。
>
> **同时严禁**：把 `TruncateFromHeight` 实现成对 `blocks.dat` 的字节级 `Truncate()`。那会在 append-only 文件上造成空洞或错位，并使**所有后续记录的高度全部偏移**（因为 legacy 记录的高度 = 序号）。这是本报告认为最容易被误实现、后果最严重的一条。

---

## §7 Atomicity Boundary

### 7.1 持久化不变式（冻结）

```text
canonical(blockHash)  ⟺
      record(BLOCK, blockHash) 存在  且 checksum 合法  且 recordHash == block.Header.Hash()
  AND record(UNDO,  blockHash) 存在  且 checksum 合法  且 undo.Height == height
  AND blockHash ∈ ancestors( tipMarker.hash )
```

### 7.2 真值层级（唯一权威，消灭"两个 canonical state"）

| 层 | 角色 | 权威性 |
|---|---|---|
| **日志中的区块记录** | 「存在哪些区块」的**唯一权威** | 权威 |
| **从创世回放** | 「由区块集合派生出什么 UTXO 状态」的**唯一权威**（实测 0.52 ms/块） | 权威 |
| **UNDO 记录** | **派生缓存**：把"回退一块"从 O(H) 降为 O(1)。缺失/损坏 → 可用回放重建 | 非权威（可重建） |
| **TIP 记录** | 「最后一次提交的规范链决策」的**持久陈述** | 权威（就"当时决策"而言） |
| **索引（内存）** | TIP + 日志扫描的**纯函数** | 永不落盘 ⇒ 永不为真值来源 |
| **`blocktree`（内存树 + SetTip）** | REORG-1C 的 fork-choice 工作区 | 非持久 |

**不可存在两个无法判定真值的 canonical state** —— 本设计通过"**TIP 记录是唯一提交点**"实现：任何时刻，"规范链"= 最后一条**完整且 checksum 合法**的 TIP 记录所指向的祖先链。冲突时（例如 TIP 指向的块缺失）→ **不做猜测**，按 §8 的确定性规则降级或 fail-stop。

### 7.3 选择「回放为权威 + UNDO 为缓存」的理由（关键论证）

因为 **UTXO 集合永不落盘**（§2.2），任何"部分完成的 UTXO 变更"都不可能出现在磁盘上 —— 崩溃后 UTXO 必然从日志完整重建。这把一整类崩溃一致性难题（半提交的 UTXO/索引/元数据）**结构性消除**。代价是 O(H) 启动（实测 0.67 s @1285 块），并给出一个**免费的、确定性的、可独立实现（Python verifier 已在库）的修复后备**。

因此 1E 的正确分工是：
- **主路径**：UNDO 日志 ⇒ reorg 断开 O(D)。
- **后备路径**：从创世/检查点回放 ⇒ 当 UNDO 缺失/损坏/不一致时**重建**，而非使用不可信数据。
- **两条路径必须都实现并有测试**（后备路径的测试可直接复用 `verify` 的只读回放机制）。

---

## §8 Restart Recovery

### 8.1 九场景判定矩阵（设计，不实现）

| 场景 | 检测方式 | startup 应做 | 类别 |
|---|---|---|---|
| clean shutdown | 尾部完整、TIP 存在且自洽 | 正常加载 | **no-op** |
| unclean shutdown（尾部完整） | 同上（TIP 未丢） | 正常加载 + 日志记录"非正常关闭" | **no-op** |
| **partial block**（尾部不完整） | 帧头/载荷短读、checksum 缺失 | 若**严格位于文件尾**且**不被任何 TIP 引用** → 丢弃该尾部记录（唯一允许的物理截断）并告警；否则 fail-stop | **repair（有界）** |
| **partial undo** | 同上 | 丢弃该 UNDO；其对应块因无 UNDO 而**不可提交** | **repair（有界）** |
| **partial tip** | TIP 记录不可读/checksum 不匹配 | 回落到**上一条**完整 TIP | **rollback（有界）** |
| **stale index** | —— | **结构上不可能**（索引永不落盘） | **N/A** |
| **stale tip** | TIP 指向的 hash 无 BLOCK/UNDO | ①回落到 ring 中上一条 TIP（建议保留 K=8 条历史 TIP）；②若 ring 耗尽 → **拒绝启动**并提示 `verify` / 显式重建 | **rollback → reject** |
| **missing undo**（规范链上某块无 UNDO） | 索引发现 `canonical(h)` 但 `UNDO(h)` 缺失 | ①回落 TIP 至该块之前；②或（显式选项）从创世**回放重建**并重写缺失 UNDO。**默认取 ①（保守）** | **rollback（默认）** |
| **corrupt undo**（checksum 不匹配） | 帧 checksum | 等同 missing undo。**永不使用损坏的 UNDO** | **rollback（默认）** |
| **extra orphan block**（尾部有合法但不规范/父未知的块） | 接入树失败或 `IsCanonical=false` | **保留**（可能是合法分叉）；仅不在规范链上 | **no-op（保留）** |

### 8.2 三种模式的边界（必须显式区分，不得混淆）

| 模式 | 触发条件 | 允许的动作 | 禁止的动作 |
|---|---|---|---|
| **REJECT**（fail-stop） | 损坏不在尾部、或自洽性无法用确定性规则裁决、或 TIP ring 耗尽 | 打印精确诊断（记录类型 / 偏移 / hash / 原因）+ 非零退出 | 禁止"猜一个 tip 继续跑" |
| **REPAIR**（有界） | **仅**未提交尾部（严格位于 EOF 且未被任何 TIP 引用） | 丢弃尾部记录（可物理截断至最后一个合法记录边界）；记录日志 | 禁止修改任何**已提交**字节 |
| **ROLLBACK**（有界） | TIP 不可用 / 规范链缺 UNDO | 沿 TIP ring 回退；或按显式选项走回放重建 | 禁止静默回退而不告警 |
| **REBUILD** | 显式操作者动作（例如未来 `--reindex`） | 从创世回放重建全部派生状态并重写 UNDO | 禁止在未授权时自动触发 |

### 8.3 与既有测试的冲突处理（重要）

`internal/storage/file_test.go:86` 的 `TestFileStoreDetectsCorruption` 断言：**尾部截断 3 字节后 `OpenFileBlockStore` 必须返回错误**。引入尾部修复后，若让**写模式**自动修复，该测试的**断言语义会被削弱**（违反"不为测试改测试"与"禁止降低断言"）。

**冻结规则（消解冲突，不改测试）**：
- **只读路径**（`OpenFileBlockStoreReadOnly`，被 `printchain` / `verify` 使用）**保持严格拒绝** —— 只读校验器**永远不得**修复或容忍损坏。→ 现有断言**原样成立**。
- **写模式**的自动修复**仅**作用于"严格位于 EOF 的未提交尾部"，且是**新增的独立测试**的职责。
- 因此 1E 完成后，`TestFileStoreDetectsCorruption` **不需要任何修改**；1E 只**新增**测试（见 §11）。

---

## §9 Reorg Compatibility

### 9.1 目标流程 vs 1E 能力映射

| 流程步 | 归属阶段 | 1E 是否可安全支撑 | 说明 |
|---|---|---|---|
| `OldTip` | 1C | ✅（读 TIP 记录） | 1E 提供显式 tip |
| `Find Fork`（共同祖先） | 1C | ✅（1A 的 `AncestorAtHeight` / `PathToRoot`） | 1E 提供按 hash 取块 |
| `Disconnect Old Blocks` | 1C | ✅（1D 的 `DisconnectBlock`） | —— |
| `Restore UTXO via Undo` | 1C | ✅（前提：UNDO **持久化**存在） | **1E 的核心交付** |
| `Delete/Detach old canonical storage` | **1E** | ✅（§6 语义） | —— |
| `Connect New Branch` | 1C | ✅（`ApplyBlockWithUndo`） | —— |
| `Persist Blocks + Undo` | **1E** | ✅（§4 设计） | —— |
| `Update ActiveTip` | 1C 决策 / 1E 持久化 | ✅（TIP 记录追加） | —— |
| （重入）`restart during reorg` | **1E** | ✅（§8 前缀严格 + TIP 提交点） | 崩溃 ⇒ 要么"整个 reorg 未发生"，要么"已完成" |

### 9.2 特别检查项

| 检查 | 结论 |
|---|---|
| old branch undo retrieval | ✅ 可行（hash 寻址），**前提**：old branch 的区块与 UNDO 都已物理落盘。 |
| new branch block retrieval | ✅ 可行（hash 寻址 + 显式 height）。**前提**：new branch 的块已存储 —— 这要求 **1E 的日志必须能承载非规范块**（见 Q4 红线）。 |
| fork height | 由 1C 从共同祖先计算；1E 只提供 `height` 显式字段，使"同一高度多个块"可共存。 |
| deletion boundary | ✅ §6 已冻结（逻辑删除 + TIP 前移；物理截断仅限未提交尾）。 |
| active tip transition | ✅ 单次 TIP 记录追加 = 原子提交。 |
| restart during reorg | ✅ 决定性：TIP 记录是唯一提交点 ⇒ **不存在"半 reorg"**。 |

### 9.3 ⚠️ 范围红线（必须在 1E 开工前确认，否则 1E 无意义）

> **Q4 判定**：若 REORG-1E **只**持久化规范链上的区块（不承载分叉块），则：
> ① `DeleteBranch` 无对象可删（分叉块从未被存储）；
> ② 重启后 detached branch 不可恢复；
> ③ `Find Fork` 之后"Connect New Branch"所需的新分支块只能来自 P2P 重新拉取（依赖 REORG-1H），reorg 在离线/对端不可用时**无法完成**。
>
> **本报告冻结结论**：**1E 的日志格式必须天然支持非规范块**（这对哈希寻址 + 显式 height 来说是"顺带获得"的能力：只要父存在即可写入）。1E **不**负责"何时写入分叉块"（那属 1C 决策 / 1H 网络），但**必须不阻碍**它。若实现时把 `SaveBlock` 限制为"只能接当前 tip"，则将重新制造 FC-001。

---

## §10 Security / Corruption Model

| # | 损坏类型 | Detection | Failure mode | Recovery mode | Consensus impact |
|---|---|---|---|---|---|
| 1 | missing block | 索引/TIP 引用某 hash 但无 BLOCK 记录 | 规范链不完整 | **ROLLBACK**：TIP 前移；或 REBUILD | **无**（本地） |
| 2 | missing undo | 规范块存在但无 UNDO 记录 | 无法 disconnect | **ROLLBACK** 或 REBUILD（默认保守回退） | **无** |
| 3 | wrong undo（错块配错 undo） | ①`SHA256(blockHash‖undo)` ≠ checksum；②`undo.Height ≠ height`；③`DisconnectBlock` 的 `ErrUndoCorrupted*` | 若未被检出将导致 UTXO 错误回滚 | 拒绝使用 → 等同 missing → ROLLBACK/REBUILD | **无**（本地 UTXO 污染不上链；且后续块的 PoW/签名校验会立即暴露） |
| 4 | wrong block hash（记录 hash ≠ 实际块 hash） | 帧头 `blockHash` vs `block.Header.Hash()` | 索引键错位 | 拒绝该记录 → REJECT（非尾部）或 REPAIR（尾部） | **无** |
| 5 | wrong height（记录 height ≠ 实际链位置） | `height ≠ parent.Height + 1`；replay `PrevHash == tip.Hash()` | 高度错位 | REJECT | **无** |
| 6 | wrong parent（`PrevBlockHash` 不匹配） | replay `ErrInvalidPrevHash`（`blockchain.go:223`） | 链断裂 | REJECT（或按 TIP 回退） | **无** |
| 7 | duplicate index | 同一 hash 出现两次 → 写入时 `ErrDuplicateHash` 语义；扫描时断言 | 索引歧义 | 拒绝启动（REJECT），提示重复偏移 | **无** |
| 8 | stale index | **结构上不可能**（索引永不落盘，每次启动重建） | —— | N/A | **无** |
| 9 | malformed undo | `DecodeUndo` 结构错误 | 无法解析 | 等同 missing → ROLLBACK/REBUILD | **无** |
| 10 | truncated undo | 帧长/短读/checksum | 同 9 | 同 9 | **无** |
| 11 | checksum mismatch | 帧 checksum | 记录不可信 | 尾部→REPAIR；非尾部→REJECT | **无** |
| 12 | active tip → 不存在的块 | TIP 引用的 hash 无 BLOCK/UNDO | 规范链悬空 | **ROLLBACK**（TIP ring K=8）；ring 耗尽→REJECT | **无** |
| 13 | **torn write（尾部部分写）** | 短读 / checksum | **今日：节点永久无法启动（P1）** | 1E：**REPAIR**（丢弃未提交尾） | **无** |
| 14 | 记录长度字段被篡改 | 长度越界（`0 < len ≤ 64 MiB`）或解码失败 | 误解析 | REJECT | **无** |

### 10.1 共识影响总判定（关键）

> **REORG-1E 是纯本地、非共识阶段。** 存储格式与恢复策略影响的是**本节点能否读到自己的区块数据**，不影响：
> - 区块有效性判定（`validateBlock` 7 步零改动）；
> - 难度/MTP/版本规则（`consensus.go` 零改动）；
> - PoW 目标（`pow.go` 零改动）；
> - 与对端的消息格式（`p2p` 零改动）。
>
> 任何 1E 缺陷的最坏后果是**本节点本地数据丢失 / 拒绝启动 / 需要重建**，**不可能造成网络共识分叉**。这一判定是 §12 允许 1E 独立于 1C/1H 先行实施的**唯一正当理由**。

### 10.2 攻击面（本地威胁模型）

| 威胁 | 现状 | 1E 后 |
|---|---|---|
| 本地磁盘位翻转 | 依赖 replay 检出（O(H)） | **记录级 checksum 即时检出 + 精确定位** |
| 恶意/错误软件改写 datadir | 同上 | 同上（checksum + blockHash 双绑） |
| 磁盘满 / 写入中断 | **节点砖化** | 有界尾部修复 |
| 时钟回拨导致的重放 | 不影响（本地记录） | 不影响 |

---

## §11 Tests Gap Audit（只读检查）

### 11.1 现有覆盖

| 领域 | 现有测试 | 判定 |
|---|---|---|
| block persistence | `storage/file_test.go:26 TestFileStoreSaveLoad`（保存/重开/重建索引/父不存在拒绝/越界） | **PASS** |
| index | 同上（`GetBlockByHash` / `GetBlockByHeight` 双向） | **PASS（部分）** |
| restart | `storage/file_test.go:26`（重开）；`blockchain/replay_persistence_test.go` 4 项：`TestRestartDoesNotDuplicatePersistedBlocks` / `TestSecondRestartSucceeds` / `TestRuntimeBlockIsStillPersisted` / `TestDuplicatePersistedRecordIsRejected` | **PASS** |
| corruption | `storage/file_test.go:86 TestFileStoreDetectsCorruption`（尾部截断 3 B） | **PASS（仅 1 场景）** |
| read-only | `storage/file_test.go:110 TestFileStoreReadOnly` | **PASS** |
| datalock | `storage/datalock_test.go` 5 + `datalock_p3_test.go` 4 = 9 项 | **PASS** |
| truncate | 无（能力不存在） | **BLOCKER（= 1E 范围）** |
| delete | 无（能力不存在） | **BLOCKER（= 1E 范围）** |
| undo persistence | 无（1D 仅测 Encode/Decode 内存 round-trip） | **BLOCKER（= 1E 范围）** |
| crash consistency | 无（无 crash 注入测试） | **BLOCKER（= 1E 范围）** |
| fork branch storage | 无 | **BLOCKER（= 1E 范围）** |
| active tip | 无（tip 隐式） | **BLOCKER（= 1E 范围）** |

### 11.2 缺口清单（判定 + 归属）

| # | 缺口 | 判定 | 归属 |
|---|---|---|---|
| G-01 | 记录帧 magic/version/checksum 的解析与拒绝 | **GAP → 1E MUST** | 1E |
| G-02 | legacy 前缀 + v2 记录混合日志的正确读取（含"v1 读 v2 必拒"的负例） | **GAP → 1E MUST** | 1E |
| G-03 | 非规范块可存储 + 索引可区分 canonical / detached | **GAP → 1E MUST** | 1E |
| G-04 | UNDO 落盘 + 三重绑定校验（含 wrong-undo 负例） | **GAP → 1E MUST** | 1E |
| G-05 | TIP 记录追加 / ring 回退 / 耗尽拒绝 | **GAP → 1E MUST** | 1E |
| G-06 | 崩溃注入矩阵（在 8 个写入点各截断一次） | **GAP → 1E MUST** | 1E |
| G-07 | 尾部修复：只修未提交尾、绝不改已提交字节、只读路径仍严格拒绝 | **GAP → 1E MUST** | 1E |
| G-08 | `DeleteBlock` / `DeleteBranch` / `TruncateFromHeight` 语义 + 拒绝删规范块 | **GAP → 1E MUST** | 1E |
| G-09 | `SaveBlock` 高度连续性强制（SP-3）+ 零父哈希旁路封堵（SP-3b） | **GAP → 1E MUST** | 1E |
| G-10 | 单 fsync/块 的性能回归测试（不得低于当前 36.9 blk/s 量级） | **GAP → 1E SHOULD** | 1E |
| G-11 | `byHeight` 全量对象内存占用（SP-6） | **DEFERRED** | 后续 |
| G-12 | 物理 compaction / GC | **DEFERRED** | 后续（需显式离线命令） |
| G-13 | `internal/config` 死包清理 | **DEFERRED** | 无关本阶段 |
| G-14 | `docs/PHASE-P3.1-DATA-LOCK-CLOSURE-REPORT.md` trailing whitespace | **DEFERRED** | 无关本阶段 |
| G-15 | `blocktree TestI_RestartLikeReconstruction` flaky（**BT-1**） | **GAP（既有，非 1E 引入）** | 建议独立 `BT-1-REMEDIATION-1` |

> **测试纪律声明**：本阶段**未修改任何测试**，**未降低任何断言**。G-07 与 `TestFileStoreDetectsCorruption` 的潜在冲突已由 §8.3 的"只读路径严格拒绝"规则消解 —— 1E 只**新增**测试，不修改既有断言。

---

## §12 Implementation Scope（最小实施范围）

### 12.1 MUST IMPLEMENT IN REORG-1E

| # | 项 | 说明 | 预估规模 |
|---|---|---|---|
| **M1** | **记录帧 v2（I1）** | `magic=PCC2 / version / type / reserved / payloadLen / height / blockHash / payload / SHA256 checksum`；解析器 + 编码器 + 全部拒绝分支 | ~250 行 + ~300 行测试 |
| **M2** | **legacy 前缀兼容读取** | legacy 记录按 v1 语义读为区块（height=序号、hash 重算）；混合日志顺序扫描；**v1 读者遇 v2 magic 必拒**（负例测试） | 合入 M1 |
| **M3** | **哈希寻址索引（I5）** | `hash → {blockOff, undoOff, height, canonical}`；永不落盘；启动由扫描重建；重复 hash 检测 | ~150 行 |
| **M4** | **UNDO 持久化（I2）** | 包装 `utxo.EncodeUndo`（**零改动 1D 字节**）；三重绑定（帧 blockHash / checksum 含 blockHash / `undo.Height == height`） | ~150 行 |
| **M5** | **TIP 记录 = 提交点（I3）** | 追加式 TIP；K=8 ring；`TruncateFromHeight` 的唯一落点；ring 耗尽 → 拒绝启动 | ~120 行 |
| **M6** | **Delete / Branch / Truncate 原语（§6）** | 逻辑删除 + 墓碑 + 校验"不得删规范块" + 单次 TIP 提交 | ~200 行 |
| **M7** | **崩溃恢复（I4 + §8）** | 前缀严格扫描；未提交尾检测；有界尾部修复；missing/corrupt UNDO 的保守回退；**只读路径保持严格拒绝** | ~200 行 |
| **M8** | **SP-3 / SP-3b 修复** | `SaveBlock` 强制 `height == parent.Height+1`；封堵零父哈希旁路 | ~20 行 |
| **M9** | **测试（§11 G-01…G-10）** | 新增，**不修改任何既有断言** | ~900+ 行 |
| **M10** | **DOC-1 修正** | `undo.go` 格式注释补 `PreSetItemsCount`（**仅注释**） | 2 行 |

**MUST IMPLEMENT 的显式边界**：全部改动限定在 `internal/storage/`（+ `internal/utxo/undo.go` 的注释）+ 对应测试。**不接入** `cmd/node`（除必要的只读诊断外），**不接入** `blockchain`。

### 12.2 MUST NOT IMPLEMENT

| # | 禁止项 | 理由 |
|---|---|---|
| X1 | full reorg orchestration（Connect/Disconnect 编排、common ancestor 驱动） | REORG-1C |
| X2 | `blocktree.SetTip` 生产接入 / 共识级 SetTip | REORG-1C（1B 仅为树级指针） |
| X3 | mempool readd（ReaddDisconnected） | REORG-1F（FC-006） |
| X4 | orphan pool | REORG-1G（FC-002） |
| X5 | P2P work-aware handshake / by-hash 分支拉取 | REORG-1H（FC-003） |
| X6 | finality / MaxReorgDepth 执行 | REORG-1I（FC-005 / BG-3） |
| X7 | `/status` fork/orphan 可观测 | REORG-1J |
| X8 | mining 路径任何变更 | 无关联 |
| X9 | **任何共识参数变更**（难度 / MTP / 版本 / PoW / coinbase / maturity） | 硬边界 |
| X10 | 引入第三方依赖（bbolt/leveldb/badger/WAL 库） | 硬边界 + 无必要（§5.3） |
| X11 | 物理 compaction / GC / 字节级 `Truncate` 已提交数据 | DEFERRED；易造成高度错位 |
| X12 | 触碰生产 datadir / 启动 reorg / 调用 SetTip | HARD STOP |

---

## §13 Required Final Verdict

```
VERDICT = READY FOR REORG-1E IMPLEMENTATION
```

**判定依据**：
1. **设计已完整冻结**：F1–F8（帧格式 / 文件布局 / 哈希寻址 / 写入顺序 / 索引不落盘 / 校验层级 / 恢复模式 / 零依赖）+ §6 三个原语语义 + §7 不变式 + §8 九场景矩阵 + §12 M1–M10 / X1–X12。
2. **无 BLOCKING-GRADE 未决项**：不存在"必须先由其他阶段关闭的强制前置"（对比 REORG-1C 时的 B1–B7）。1E 所需的全部前置资产已就绪：1A（BlockNode/BlockTree）、1B（树级 SetTip）、1D（BlockUndo + Encode/Decode + DisconnectBlock）。
3. **1E 是纯本地非共识阶段**（§10.1）：格式/恢复错误不会造成网络分叉 ⇒ 可独立于 1C/1H 先行。
4. **关键冲突已消解**：§8.3（只读路径严格拒绝 ⇒ `TestFileStoreDetectsCorruption` 断言无需修改）；§9.3（1E 必须天然支持非规范块，否则重造 FC-001 —— 已冻结为设计红线）。
5. **成本已量化**：单 fsync/块（无吞吐回退）、日志增幅 <2.4×（~563 KB @1285 块）、启动回放 0.67 s（实测）。

**同阶段内必须闭合的缺陷（非阻断开工）**：
- **SP-3**：`SaveBlock` 未强制高度连续性（文档 vs 实现不一致）→ M8
- **SP-3b**：零 `PrevBlockHash` 旁路全部父校验 → M8
- **FC-007 / B3**：存储无 Delete/Truncate → M6（本阶段核心交付）

**升级登记（既有，非 1E 范围）**：
- **BT-1（P2）**：`blocktree.TestI_RestartLikeReconstruction` 本会话复现率 **3/4 FAIL**（历史 2/6），`go test ./...` 现大概率红灯。建议独立 `BT-1-REMEDIATION-1`（按 height 排序确定化重建顺序）。**任何后续阶段引用"全量测试绿灯"前必须先解决或显式排除该包。**

**环境观察（非阻断）**：
- `.123` 上 2 个 disposable 挖矿进程（p2a:17890 / p2c:17894，来自 P2-SIGTERM 复现矩阵）仍驻留，建议后续授权阶段清理。
- 本地 `C:\c\` 空目录（Windows 路径转换误建残留，非本阶段产生）。

---

## §14 Next-Step Decision

**本阶段不自动执行实现。** 审计结束后，只有以下两种选择之一：

| 选项 | 内容 | 前置条件 |
|---|---|---|
| **A（推荐）** | `PHASE REORG-1E-IMPLEMENTATION-1` —— 按 §12 M1–M10 实现存储帧 / 哈希索引 / UNDO 落盘 / TIP 提交点 / Delete-Truncate / 崩溃恢复 + 全量新增测试 | 需用户显式授权；建议先落 F 冻结为独立设计文档（可选，非必需 —— 本报告 §4–§12 已构成完整契约） |
| **B** | 先补 `BT-1-REMEDIATION-1`（恢复全量测试绿灯）再进 1E | 需用户显式授权 |

**不建议**：跳过 1E 直接进 REORG-1C —— 1C 的 `DisconnectBlock` 需要一个**持久**的 undo 来源，否则重启即失去回滚能力（§9.2 / Q5）。

**之后**：1E 完成后 → `REORG-1F`(mempool readd) → `REORG-1G`(orphan) → `REORG-1H`(P2P by-hash) → `REORG-1I`(finality/MaxReorgDepth, BG-3 语义) → `REORG-1J`(观测) → 最后才是 `REORG-1C`（共识级链切换编排）。

---

## §15 纪律执行声明

| 约束 | 执行情况 |
|---|---|
| 修改 `.go` / 测试 / 配置 / 协议参数 / consensus / blockchain orchestration / mempool / P2P | **零**（`git status --porcelain '*.go'` = 仅 2 个 pre-existing untracked） |
| 启动 production reorg / 调用 production `SetTip` | **零** |
| 删除/truncate 生产数据 / 触碰生产 datadir | **零写入**（仅只读 `ls`/`sha256sum`/`cat`/`curl /status` + `blocks.dat` 只读复制到本地临时目录） |
| `git add` / commit / push / tag / merge / rebase / amend / squash / reset | **零** |
| 本地构建与测试 | 允许且已执行（`go build -o C:\tmp\...`、`go test`、`node verify` 于**本地副本**） |
| 临时产物 | `C:\tmp\reorg1e-audit\`（本地副本 + 审计用二进制 + verify 输出）；误建的 `C:\c\tmp\...` 已当场清除 |

---

**报告结束 — STOP AFTER REPORT**
