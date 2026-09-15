# PHASE REORG-1J-R4B-PRE-GATE — 正向端到端 reorg 收敛 / 双进程竞争分叉审计

- 阶段：`REORG-1J-R4B-PRE-GATE / POSITIVE END-TO-END REORG CONVERGENCE / DUAL-PROCESS COMPETITIVE FORK AUDIT`
- 日期：2026-09-15
- 仓库：`C:\Users\Administrator\Desktop\挖矿\p2pchain`，HEAD = `e04d678`
- 执行模式：**STRICT READ-ONLY（生产代码零改动）**
- 唯一新增文件：`cmd/node/r4b_dual_reorg_convergence_test.go`（纯测试，无生产分支、无 hook）

---

## 1. VERDICT

```
VERDICT = PASS
GAP-1H-A = CLOSED
```

**判定依据（不是测试数量）**：两个**真实 node 操作系统进程**在**真实 P2P TCP** 上交换**竞争分叉**区块后，沿**生产**代码路径完成了一次真实 reorg，确定性地收敛到同一个 canonical tip；被重组节点的 **legacy 物理前缀逐字节不变**；`L = 2 / 8 / 64` 三档全部成立，**fork 高度 f = L-2，新分支含高度 h = L-1 < L**（v2 块占据 legacy 逻辑槽位）。

> **GAP-1H-A 闭合声明**：GAP-1H-A 的原始命题是「现实持久化链的首次 reorg 必被存储层拒绝」。
> 本阶段证明：在**未修改任何生产代码**的前提下，现实持久化链（100% legacy 前缀）的首次 reorg
> **被生产路径完整走通并完成**。因此 `GAP-1H-A = CLOSED`。
> 1J-G08 已移除存储层 Gate2，R4A 在存储层证闭，R4B 在端到端生产路径证闭——三段证据链至此闭合。

---

## 2. 范围锁定（SCOPE LOCK）

本阶段**未修改**（mtime 硬证据见 §12）：

| 对象 | 状态 |
|---|---|
| `internal/storage/*.go`（生产：`file.go` / `v2.go` / `v2api.go` / `frame.go`） | 未修改 |
| `internal/blockchain/*.go` | 未修改 |
| `internal/blocktree/*.go` | 未修改 |
| `internal/p2p/*.go` | 未修改 |
| `cmd/node/main.go` / `service.go` / `nodeapi.go` | 未修改 |
| 共识规则、难度/bits、区块有效性 | 未修改 |
| G08 / G09 / R3 / R4A 既有产物（含 `TestRealProcessForkReorgConvergesByHash` 正向 tripwire） | 未修改 |
| 新增 crash hook / test-only 生产分支 | **无** |
| legacy 写入语义 / UNDO 绑定 / 创世语义 | 未修改 |
| BT-1 修复 | **未做**（按需排除，见 §11） |
| commit / push / tag / merge / rebase / amend / squash / deploy / Windows 挖矿 | **未执行** |

---

## 3. 生产调用路径（§4：实际 file:line，非测试名推测）

### 3.1 进程装配（`cmd/node/main.go`）

```
newNodeRuntime(cfg nodeConfig)            main.go:73
  ├─ DirLock                              （node.lock 独占）
  ├─ storage.OpenFileBlockStore           main.go:  → loadLog/interpretScan（v2.go:305/359）
  ├─ blockchain.NewBlockchainFromStore    blockchain.go:159（空库→写创世；否则 1..h 回放 + rebuildTree:138）
  ├─ mempool / wallet
  ├─ newNodeService                       → nodeapi.go:38 StatusInfo
  ├─ p2p.NewNode(...).Start()             （-listen）
  ├─ SetHeightProvider / SetChainStatusProvider（ChainWork + TipHash → 握手载荷）
  ├─ control.NewServer + ctl.Start        （-rpc）
  └─ connectSeeds / watchSeeds            （-seed，真实 TCP）
```

### 3.2 P2P 收块 → 共识 → 存储（真实 reorg 全链）

```
p2p 入站
  └─ OnNewBlock        cmd/node/service.go:261   （json → hex → block.DecodeBlock）
   |  OnBlocksResp     cmd/node/service.go:350   （批量同步 / 孤儿处理）
     └─ ingestBlock    cmd/node/service.go:436
        ├─ IsCanonicalHash → 重复，静默返回
        ├─ ErrOrphanParent → deferOrphan + 按哈希请求祖先
        └─ addBlockAndUpdatePool           cmd/node/service.go:659
           └─ Blockchain.AddBlockWithResult blockchain.go:400
              └─ Blockchain.addBlock         blockchain.go:419
                 ├─ Case 1（parent == tip） extendChain  blockchain.go:506
                 │    └─ 门控 blockchain.go:512：
                 │       `if v2s, ok := bc.store.(reorgStore); ok && v2s.V2Mode()`
                 │          true  → utxo.ApplyBlockWithUndo + v2s.AppendCanonicalBlock（v2）
                 │          false → bc.store.SaveBlock(b)                    （legacy，逐字节不变）
                 │    └─ tree.AddBlock + tree.SetTip
                 └─ Case 2（fork）            blockchain.go:~455
                      ├─ canonicalContains 去重
                      ├─ tree.LookupNode(parentHash) 否则 ErrOrphanParent
                      ├─ tree.AddBlock                        blockchain.go:463
                      ├─ validateForkBlock                    blockchain.go:472
                      ├─ ShouldReorg                          blockchain.go:480
                      ├─ SaveBlockDetached（**reorg 前必写**） blockchain.go:489 / 498
                      └─ executeReorg                         blockchain.go:502 → :655
                           ├─ FindCommonAncestor / disconnectPath / connectPath
                           ├─ utxoAtNode(ancestor) 全量 replay（不依赖 legacy UNDO，:688）
                           ├─ utxo.ApplyBlockWithUndo × connectPath
                           └─ v2s.CommitReorg(detachedUndos, actualNewBlocks, …, newTip.Hash)
                                └─ PutUndo… → appendFrames → 唯一一次 file.Sync()
                                   → commitTipAfterAppend（v2api.go:58）→ setCanonicalFrom
```

### 3.3 重启恢复

```
NewBlockchainFromStore  blockchain.go:159
  ├─ store.Height()（v2 TIP 优先，REORG-1J 后不依赖 v2Mode）
  ├─ 1..h 回放 applyBlock（无写回）
  └─ rebuildTree         blockchain.go:138（按 bc.blocks **切片**逐高度 AddBlock）
```

### 3.4 关键机理：`v2Mode` 的鸡生蛋与「首次 v2 帧从哪来」

`v2Mode` **只**在以下点置真：载入时见到 v2 帧（`v2.go:399`）、`SaveBlockDetached`（`v2api.go:165`）、`SaveBlockWithUndo`（:203）、`PutUndo`（:235）、`commitTipAfterAppend`（:68）。

⇒ 一个**全新纯 legacy** 数据目录在挖矿时，`extendChain` 的门控（`blockchain.go:512`）为 false，会**永远走 legacy `SaveBlock`**。
⇒ 因此**现实中第一枚 v2 帧只能来自 fork 分支的 `SaveBlockDetached`**（`blockchain.go:489/498`）。
这正是「现实持久化链 = 100% legacy 前缀」的成因，也正是 R4B 必须证明的场景。

**本阶段的构造天然命中该路径**：A 持 100% legacy 前缀、不出块；v2 帧只能由网络收到的竞争块触发。

---

## 4. 双进程竞争分叉模型（§5）

| 项 | 实现 |
|---|---|
| 进程 | 两个**真实 node 操作系统进程**（`exec.Command(bin, -datadir -listen -rpc [-seed])`），非单进程内双 runtime |
| 数据目录 | 各自 `t.TempDir()`，独立 `blocks.dat` / blocktree / UTXO / `node.lock` |
| 端口 | 各自独立空闲端口，listen ≠ rpc |
| 传输 | 真实 TCP，`-seed <B 的 P2P 监听地址>` |
| Mock | **无 mock P2P**，握手/同步/by-hash 请求全部走真实网络栈 |
| 直接调用 CommitReorg | **从不**；reorg 只能由网络收到的竞争块触发 |
| A 是否出块 | **否**（无 `-mine`）。断言 A 日志无 `MINING_BLOCK_ACCEPTED` ⇒ A 的任何高度增长**只能**来自 P2P |
| 共同前缀来源 | 同一份历史**截断**得到（构造铁律），非两次独立挖矿 |

**几何构造**

```
L        = 被重组节点 A 的 legacy 记录条数（高度 0..L-1）
histA    = mineChainOffline(L-1)                  → L 条 legacy 记录，高度 0..L-1
prefixB  = truncateLegacy(histA, L-2)             → L-1 条，高度 0..L-2（共同前缀）
histB    = extendChainOffline(prefixB, 2)         → L+1 条，高度 0..L

共同祖先 P @ 高度 f = L-2
A 的高度 L-1 块 = X（legacy 物理记录 index L-1）
B 的高度 L-1 块 = Y（竞争块，将作为 v2 占据 legacy **逻辑**槽位 L-1）
B 的高度 L   块 = Z（parent = Y）
```

- `L-1 < L` ⇒ **new branch 含 h < L**（§7 要求 ✓）
- `f = L-2`（§7 要求 ✓）
- 累计工作量：B 比 A 多一整块 ⇒ `newWork > oldWork` **严格**成立，不依赖 tie-break（§13「更高 work 分叉未被采纳」不成立）

---

## 5. reorg 真的发生了吗（§6 全字段记录）

`go test -run TestR4B -v ./cmd/node/`，实测 **PASS，173.182s**。以下为**实测输出原文摘录**。

### L = 2

| 字段 | 值 |
|---|---|
| old tip（A，reorg 前） | `h=1 0000965629100d17095c5bb7c8f264d91f0c52f462cba0e56d932aa5822be475` |
| competing tip（B） | `h=2 0000914bfa7a22dae365decafacf45ae142ee3e3f293bae99a1965bc623764ba` |
| fork height f | **0**（共同祖先 = 创世 `0000aca1af72`） |
| old chain work | `131072` |
| new chain work | `196608` |
| selected tip | `h=2 0000914bfa7a22dae365decafacf45ae142ee3e3f293bae99a1965bc623764ba` |
| detached heights | `[1]` |
| attached heights | `[1 2]`（含 h=1 < L=2） |
| A 持久 | `1442 B / legacy=2 / records=4 / TIP帧=0000914bf… / canonicalTip=0000914bf… / recovery=REBUILD` |
| B 持久 | `540 B / legacy=3 / records=3 / 无 TIP 帧（纯 legacy，合法）/ canonicalTip=0000914bf… / recovery=REBUILD` |
| canonical 路径 | 3 项，A == B |

### L = 8

| 字段 | 值 |
|---|---|
| old tip（A） | `h=7 0000379be30ec3f1bc880478800b8877b095c88e28be5b98d5c7e237062769c0` |
| competing tip（B） | `h=8 0000dce51cbaa6c4115441177b3333160fec4b32fbdcff901dac07a6cc003ca5` |
| fork height f | **6**（共同祖先 `00009f6cf214`） |
| old / new chain work | `524288` / `589824` |
| selected tip | `h=8 0000dce51cbaa6c4115441177b3333160fec4b32fbdcff901dac07a6cc003ca5` |
| detached / attached | `[7]` / `[7 8]`（含 h=7 < L=8） |
| A 持久 | `2402 B / legacy=8 / records=10 / TIP帧=0000dce51… / recovery=REBUILD` |
| B 持久 | `1620 B / legacy=9 / records=9 / 无 TIP 帧 / recovery=REBUILD` |
| canonical 路径 | 9 项，A == B |

### L = 64

| 字段 | 值 |
|---|---|
| old tip（A） | `h=63 0000e3726a28e2bf2c50ba700aee64b2b005445dce78a3ef6b92f97f99adcb8d` |
| competing tip（B） | `h=64 00004405aad21645dab02b39c6acb8d7732a14e1ba917e292701aa2bd9fabbf3` |
| fork height f | **62**（共同祖先 `000082a7cafd`） |
| old / new chain work | `4194304` / `4259840` |
| selected tip | `h=64 00004405aad21645dab02b39c6acb8d7732a14e1ba917e292701aa2bd9fabbf3` |
| detached / attached | `[63]` / `[63 64]`（含 h=63 < L=64） |
| A 持久 | `12482 B / legacy=64 / records=66 / TIP帧=00004405a… / recovery=REBUILD` |
| B 持久 | `11700 B / legacy=65 / records=65 / 无 TIP 帧 / recovery=REBUILD` |
| canonical 路径 | 65 项，A == B |

### 独立的生产路径反证（`TestR4B_ProductionReorgPathTraversed`，L=8）

```
A blocks.dat 1440 → 2522 字节（+v2 帧），legacy 前缀 1440 字节逐字节不变，
fork@7 由 X=0000ac4bab8f 翻转为 Y=0000f61cc1d2
```

该用例专门排除「高度相同 = 假阳性」：断言 A 的文件**变长**（确有新帧写入）+ legacy 前缀**逐字节不变** + fork 高度处 canonical 由 X **翻转**为 Y。

---

## 6. §7 legacy-internal 覆盖（L = 2 / 8 / 64，f = L-2）

| L | fork f | v2 占据的 legacy 逻辑槽位 | legacy 物理记录条数（前 → 后 → 重启后） | 结论 |
|---|---|---|---|---|
| 2 | 0 | h=1 | 2 → 2 → 2 | ✅ v2 占槽位，legacy 物理不变 |
| 8 | 6 | h=7 | 8 → 8 → 8 | ✅ |
| 64 | 62 | h=63 | 64 → 64 → 64 | ✅ |

三档全部满足 `f = L-2` 且 `new branch contains h = L-1 < L`。

> 说明：R4A 的合成边界 `L=0`（无 legacy 记录）不在本阶段覆盖——生产 `legacyLen ≥ 1`（创世恒以 legacy 写入，`NewBlockchainFromStore:167`）。
> `L=1` 亦不适用：唯一的 legacy 槽位是创世，替换创世违反共享创世 + SP-3b（`v2.go:544-549` / `deriveStrict` `v2.go:769-772`）。

---

## 7. §8 canonical 路径逐高度一致性（不只是 tip）

断言方式：收敛并优雅停止后，**只读**打开两侧 `blocks.dat`，对 `h = 0..L` 逐高度比对四项签名：

```
height / block hash / previous hash / bits → 单块 work / 累计 cumWork
（work 计算与 fork-choice 同源：blocktree.WorkOfBits，即 ShouldReorg 的 CompareWork 基准）
```

结果：

| L | 比对项数 | 结果 |
|---|---|---|
| 2 | 3 | 全等 |
| 8 | 9 | 全等 |
| 64 | 65 | 全等 |

实测抽样（L=64）：

```
h=0  hash=0000aca1af72 prev=000000000000 bits=16 cumWork=65536
h=1  hash=000002ccd85c prev=0000aca1af72 bits=16 cumWork=131072
h=2  hash=00000da36994 prev=000002ccd85c bits=16 cumWork=196608
...
h=62 hash=000082a7cafd prev=000080b8d5f4 bits=16 cumWork=4128768   ← 共同祖先
h=63 hash=000014628c90 prev=000082a7cafd bits=16 cumWork=4194304   ← v2 竞争块 Y（A 侧）
h=64 hash=00004405aad2 prev=000014628c90 bits=16 cumWork=4259840   ← Z
```

**要点**：A 与 B 的**物理形态不同**（A = legacy 前缀 + v2 帧；B = 纯 legacy），但 canonical **路径签名完全相同**。这证明 canonical 由 TIP + 哈希链决定，与记录的物理容器无关（1J-G08 语义）。

---

## 8. §9 legacy 物理不可变性

对每个 L、每个检查点（reorg 后、重启第 1 轮后、重启第 2 轮后）执行：

1. `bytes.HasPrefix(after, histA)` —— legacy 前缀**逐字节**仍是启动前写入的那段字节；
2. `r4bLegacyCount(after) == L` —— legacy 记录条数不变；
3. `store.LegacyRecordCount() == L` —— 存储层 legacy 长度不变（I11）；
4. `store.RecordCount()` **不减**（A：L → L+2）；
5. `r4bLegacyBlock(after, L-1).Hash == X.Hash` —— 该槽位**物理字节**仍是原来的 X，未被 v2 覆盖；
6. `store.HasBlock(X.Hash) == true` 且 `store.IsCanonical(X.Hash) == false` —— 被取代块**逻辑墓碑**而非物理删除；
7. `store.GetBlockByHeight(L-1).Hash == Y.Hash` —— 同一**逻辑**高度 canonical 已是 v2 块 Y。

⚠️ **过程中的一次真实失败（已修正，非生产问题）**：首轮实现把「纯 legacy 链无 TIP 帧」误判为损坏（`st.TipHash()` 返回 `ok=false`）。B 的链 100% legacy，**无 TIP 帧是合法形态**（canonical = legacy 记录序列）。已改为用 `GetBlockByHeight(height)` 取 canonical 链尾，并额外断言 **A 必须有 TIP 帧**（证明 `CommitReorg` 真的落了提交点）、**B 必须无 v2 帧**。

**结果：三档 L × 三个检查点，legacy 物理前缀零改写。**

---

## 9. §10 重启收敛

流程：收敛 → 优雅停止（走真实 `cmdStop` 代码路径）→ 用**同一数据目录**重启 → 比对签名 → 再停止 → **重复 2 轮**。

| L | 重启轮次 | A 重启后 (height, tip) | B 重启后 (height, tip) | 与停机前一致 |
|---|---|---|---|---|
| 2 | 1, 2 | (2, `0000914bfa…`) | (2, `0000914bfa…`) | ✅ 两轮均一致 |
| 8 | 1, 2 | (8, `0000dce51c…`) | (8, `0000dce51c…`) | ✅ |
| 64 | 1, 2 | (64, `00004405aa…`) | (64, `00004405aa…`) | ✅ |

每轮重启后再次执行 §8 的全部离线硬校验（legacy 前缀逐字节、legacy 条数、canonical 路径、`VerifyStoredChain` 通过、`recovery=REBUILD`）。

> `recovery=REBUILD` 语义（`v2.go:465-479`）：最近一个 TIP 候选有效即被选中；只有需要回退到更早 TIP 时才标记 `ROLLBACK`。
> 本阶段三档全部为 `REBUILD` ⇒ **没有任何一次重启需要回退**，与「提交点唯一且末位」一致。

---

## 10. §11 崩溃矩阵范围

**未重跑 R4A 的 18,247 字节崩溃扫描**。本阶段只做：正常生产 reorg + 持久提交 + 重启恢复。
R4A 已穷举覆盖提交边界；本阶段未引入任何新的生产写入路径（生产 diff = 0），因此不存在 R4A 未覆盖的新提交边界。

---

## 11. §12 BT-1 独立性与生产 `rebuildTree` 的关系

### 11.1 结论：**无关**。BT-1 是**测试构造缺陷**，不是生产缺陷。

| | 数据来源 | 顺序 |
|---|---|---|
| BT-1（`blocktree_test.go:216-241`） | `for _, n := range orig.nodes` —— **`map` 迭代** | **不确定**，可能子先于父 |
| 生产 `rebuildTree`（`blockchain.go:138`） | `for i := range bc.blocks` —— **切片按高度序** | **确定**，父必先于子 |

`AddBlock` 要求父节点已存在。BT-1 用 map 迭代喂给它 ⇒ 随机失败。生产路径遍历 `bc.blocks`（`[]*block.Block`，严格 0→h）⇒ 恒满足父先存在。

### 11.2 实测 flaky 率（本阶段复测）

```
go test -run TestI_RestartLikeReconstruction -count=8 ./internal/blocktree/
→ 5 FAIL / 8
```

历史序列：2/8 → 4/8 → 7/8 → **5/8**，持续波动，符合 map 迭代随机性。

### 11.3 排除声明

引用「全量绿灯」时必须**显式排除** `internal/blocktree` 的 `TestI_RestartLikeReconstruction`。
本阶段所有结论**不依赖**该包；R4B 的重启确定性证据来自**真实进程**的 `/status` 与离线只读存储事实。

---

## 12. 回归门禁与「生产 diff = 0」硬证据

### 12.1 mtime 硬证据

| 文件 | mtime |
|---|---|
| `internal/blocktree/blocktree.go` | 2026-09-15 **01:07:45** |
| `internal/blockchain/blockchain.go` | 2026-09-15 **08:43:53** |
| `cmd/node/main.go` | 2026-09-15 **08:49:52** |
| `cmd/node/service.go` | 2026-09-15 **08:50:20** |
| `internal/p2p/node.go` | 2026-09-15 **08:46:21** |
| `internal/storage/v2.go` | 2026-09-15 **10:10:19** |
| `internal/storage/file.go` | 2026-09-15 **10:11:04** |
| `internal/storage/v2api.go` | 2026-09-15 **10:12:14** |
| — R4A PRE-GATE 报告 | 2026-09-15 **13:56:25** |
| — R4A 最终报告 | 2026-09-15 **14:43:55** |
| — **R4B 新增测试文件** | 2026-09-15 **15:10:17** |

**所有生产文件的 mtime 均早于本阶段新增文件** ⇒ 本阶段 production diff = 0。

### 12.2 测试门禁

| 命令 | 结果 |
|---|---|
| `go test -run TestR4B -v ./cmd/node/` | **PASS，173.182s**（L=2 52.77s / L=8 48.66s / L=64 49.47s / 路径反证 22.09s） |
| `go test ./cmd/node/...`（全量，含 G09 R2 正向 tripwire） | **ok 556.554s**（全绿） |
| `go build -buildvcs=false ./...` | **BUILD_OK** |
| `go vet ./cmd/node/ ./internal/storage/` | **VET_OK** |
| `go test -run TestI_RestartLikeReconstruction -count=8 ./internal/blocktree/` | **5 FAIL / 8**（BT-1，已排除，见 §11） |
| `git status --porcelain \| grep -c "^ M"` | **11** = 改动 9（既有 1H/1J 工作区改动）+ 2 个长期 dirty 文档；**本阶段新增 0 处修改** |

### 12.3 全量 `cmd/node` 回归

```
go test -buildvcs=false -timeout 40m ./cmd/node/...
→ ok  p2pchain/cmd/node  556.554s
```

全量通过，其中包含：

- `TestRealProcessPairConvergesToSameTip`（1H 双进程基线）
- `TestRealProcessForkReorgConvergesByHash`（1J-G09 R2 正向 tripwire，**未改动**，仍 PASS）
- 本阶段新增 `TestR4B_*`（4 个子用例/测试）
- 全部 STOP 生命周期用例

---

## 13. §13 STOP 条件自检

| # | STOP 条件 | 结论 |
|---|---|---|
| 1 | 不存在生产 reorg 路径 | ❌ 不成立。§3 给出完整 file:line 生产路径，§5 实测三档真实触发 |
| 2 | P2P 无法触发竞争分叉 | ❌ 不成立。真实 TCP + `-seed`，A 全程不出块，高度增长全部来自 P2P |
| 3 | 更高 work 的分叉未被采纳 | ❌ 不成立。`newWork > oldWork` 严格成立，三档均收敛到 B 的分支 |
| 4 | legacy 字节被改写 | ❌ 不成立。§8 九次检查全部逐字节一致 |
| 5 | 出现两个合法 TIP | ❌ 不成立。A/B canonical 路径 3/9/65 项全等，A 的 TIP 帧唯一 |
| 6 | 重启后 tip 不确定 | ❌ 不成立。§9 两轮重启签名完全一致 |
| 7 | A / B 最终结果不一致 | ❌ 不成立。高度与 tip 全等，且逐高度路径全等 |
| 8 | 需要修改生产代码 | ❌ 不成立。生产 diff = 0（§12.1） |
| 9 | 需要改动 G08 / G09 / R3 / R4A | ❌ 不成立。一处未动；`TestRealProcessForkReorgConvergesByHash` 保持正向 tripwire 原样 |
| 10 | R4A 核心结论与真实生产路径不一致 | ❌ 不成立。R4A「v2 可占 legacy 槽位而 legacy 字节不变」在本阶段真实路径完全复现（h=L-1 占槽、legacy 条数不变） |

**无 STOP 触发。**

---

## 14. §14 十五问

| # | 问题 | 回答 |
|---|---|---|
| 1 | 生产 reorg 全链路调用路径是什么？ | §3：`OnNewBlock/OnBlocksResp`(service.go:261/350) → `ingestBlock`(:436) → `addBlockAndUpdatePool`(:659) → `AddBlockWithResult`(blockchain.go:400) → `addBlock`(:419) → Case2 → `tree.AddBlock`(:463)/`validateForkBlock`(:472)/`ShouldReorg`(:480)/`SaveBlockDetached`(:489,498)/`executeReorg`(:502,655) → `CommitReorg` → `appendFrames` 唯一 fsync → `commitTipAfterAppend`(v2api.go:58) → `setCanonicalFrom` |
| 2 | 两个真实进程能否经真实 P2P 触发竞争分叉？ | 能。独立 datadir/端口，真实 TCP `-seed`；A 不出块（断言无 `MINING_BLOCK_ACCEPTED`），高度增长全部来自网络 |
| 3 | reorg 是否真的发生，而非「高度相同」假阳性？ | 是。`TestR4B_ProductionReorgPathTraversed` 证明 A 文件 1440→2522 B（新帧写入）+ legacy 前缀逐字节不变 + fork@7 由 X 翻转为 Y |
| 4 | fork 高度与 legacy 长度的关系？ | `f = L-2`，新分支含 `h = L-1 < L`。L=2→f=0；L=8→f=6；L=64→f=62 |
| 5 | 更高 work 的分叉是否被采纳？ | 是。old/new work：131072/196608、524288/589824、4194304/4259840；三档均采纳 B 分支 |
| 6 | old tip / competing tip / selected tip 的具体值？ | 见 §5 三张表，均为实测 64-hex |
| 7 | detached / attached 高度？ | L=2: `[1]`/`[1 2]`；L=8: `[7]`/`[7 8]`；L=64: `[63]`/`[63 64]` |
| 8 | 最终持久 TIP 是什么，TIP 是否唯一？ | A 的 TIP 帧 = 选中 tip（三档一致），`recovery=REBUILD` 表明无需回退到更早 TIP；B 纯 legacy 无 TIP 帧（合法） |
| 9 | legacy 物理前缀是否逐字节不变？ | 是。reorg 后 + 重启 ×2 共九次检查全部 `bytes.HasPrefix(after, histA)` 成立，`legacy` 条数不变，`RecordCount()` 不减 |
| 10 | legacy 记录条数是否变化？ | 否。A 恒为 L（2/8/64）；B 恒为 L+1。v2 占槽位不改变 legacy 物理序列（I11） |
| 11 | 两节点 canonical 路径是否逐高度一致？ | 是。3 / 9 / 65 项逐高度比对 height·hash·prev·bits·work·cumWork 全等，尽管物理形态不同 |
| 12 | 重启后是否收敛且与停机前一致？ | 是。两轮重启，A/B 高度与 tip 均等于停机前值，且二次离线校验全通过 |
| 13 | 是否需要生产代码修改？ | **否**。production diff = 0，mtime 硬证据见 §12.1 |
| 14 | BT-1 是否与生产 `rebuildTree` 相关？ | **无关**。BT-1 用 **map** 迭代构造输入（顺序随机）；生产 `rebuildTree`(blockchain.go:138) 遍历 **slice** `bc.blocks` 按高度序，父必先于子。本阶段复测 5 FAIL/8 |
| 15 | R4A 核心结论与真实生产路径是否一致？GAP-1H-A 状态？ | **一致**。R4A 的「v2 可占 legacy 槽位、legacy 字节不变」在真实双进程路径完整复现。**`GAP-1H-A = CLOSED`** |

---

## 15. 交付清单与残留

### 交付物

| 文件 | 说明 |
|---|---|
| `cmd/node/r4b_dual_reorg_convergence_test.go` | 本阶段唯一新增文件（纯测试，~590 行） |
| `PHASE-REORG-1J-R4B-PRE-GATE-FINAL-REPORT.md` | 本报告 |

### 新增测试资产

- `TestR4B_DualProcessCompetitiveForkConverges`（子用例 `L=2` / `L=8` / `L=64`）
- `TestR4B_ProductionReorgPathTraversed`（生产路径反证，排除假阳性）

### 残留 / 观察（未修，未授权）

1. **BT-1（P2）**：`internal/blocktree` `TestI_RestartLikeReconstruction` 仍 flaky（本阶段 5/8）。测试构造缺陷，非生产缺陷。
2. **OBS-1J-R3-A（P3）**：本阶段再次确认其**生产不可达**——A 收到的 Y、Z 都先经 `SaveBlockDetached` 落盘，`actualNewBlocks` 恒空，`CommitReorg` 未走多新块路径。
3. **R4A-OBS-1（P3）**：REPAIR 只截断撕裂残片；「已提交边界」应定义为「最后一个完整帧末尾」。与本阶段无冲突。
4. **本阶段新增发现（P3，登记）**：`extendChain` 的 `V2Mode()` 门控（`blockchain.go:512`）意味着**全新纯 legacy 数据目录挖矿永不进入 v2 模式**，第一枚 v2 帧只能来自 fork 的 detached 写入。这解释了「生产链为何 100% legacy」，也让「首次 reorg」成为唯一能激活 v2 的路径——不构成缺陷，但值得在后续 1I/B7 设计中注意。
5. **本阶段新增铁律（供后续阶段复用）**：
   - `startPairWithHistory` 返回时同步**可能已完成**，捕获「reorg 前 old tip」必须先**单独启动**一次取 pre-state；
   - 纯 legacy 链**无 TIP 帧是合法形态**，不得判为损坏。

---

## 16. STOP

```
VERDICT = PASS
GAP-1H-A = CLOSED
```

**本阶段到此结束。未执行任何 commit / push / tag / merge / rebase / amend / squash / deploy / Windows 挖矿生产测试。**

后续（均需**单独授权**）候选：

1. `REORG-1J-R5` —— 提交与推送整理（1H/1J 各阶段仍未提交；注意排除长期 dirty 文件）；
2. `1I` —— finality / MaxReorgDepth（冻结 = 告警 + 限速，永不拒绝更高 work 链）；
3. `B5` —— orphan pool 完整实现；`B8` —— `/status` fork 观测。

**STOP.**
