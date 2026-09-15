# PHASE REORG-1E-IMPLEMENTATION-1 — CONTRACT CONFLICT REPORT

**触发条款**：执行提示词 §0「总原则」——

> 如果发现实现过程中必须修改以下任何冻结项（含 legacy/v2 compatibility semantics、append-only semantics、no-third-party-dependency rule 等）…… 立即 **STOP IMPLEMENTATION** 并输出 `CONTRACT CONFLICT REPORT`。不得自行修改设计继续执行。

**状态**：⛔ **IMPLEMENTATION STOPPED — 零代码写入。**

- HEAD 未变：`b001f578285a9ed1e5ee08a7067da8969d78bd60`
- 本阶段至今**未新增/修改任何 `.go` 文件**（含测试、报告以外的任何产物）
- `git status --porcelain '*.go'` 仍为 REORG-1D 的 2 个 untracked 文件，与基线逐字一致
- 本文件是唯一产出

---

## §1 冲突总述

提示词给出的两条**同时生效、且互斥**的要求：

| 要求来源 | 内容 |
|---|---|
| **M3 / M8**（§5 / §10） | `duplicate block hash 必须拒绝`；`SaveBlock` 强制 `height == parent.Height + 1`，`不得接受任意错误高度` |
| **§1 / §11**（HARD SAFETY BOUNDARY / TEST FOUNDATION） | **禁止**「修改既有测试断言以适应新实现」；**禁止**「删除或降低既有测试覆盖」 |

而 **4 处既有测试断言**恰好把「`SaveBlock` 接受错位/重复记录、且 storage 打开必须成功」钉死为**不可变契约**。二者不可同时满足。

这不是"M3/M8 难实现"，而是**规格自相矛盾**：M3/M8 要修的那段行为，正是既有测试用来断言"正确行为"的那段行为。

> **本报告同时更正上一阶段审计的一处遗漏**：`REORG-1E-PRE-IMPLEMENTATION-DESIGN-AUDIT-1` §8.3 只识别出 `TestFileStoreDetectsCorruption` 这一处潜在冲突，并据此断言"1E 只新增测试、不修改既有断言"**成立**。该结论**错误**——真实的冲突面是 4 处，含 2 处硬冲突。此处如实改写。

---

## §2 冲突 1（BLOCKING）：M8/SP-3 严格高度连续性 ⟂ 既有 P0 回归测试

### 2.1 M8 要求

```text
SP-3 强制：height == parent.Height + 1；不得接受任意错误高度。
```

### 2.2 既有断言（不可修改）

`internal/blockchain/replay_persistence_test.go:238-253`

```go
bc, store := openChain(t, dir)          // genesis 落盘
b := mineBlock(t, bc, miner)            // 追加 block #1（parent = genesis）
// 人为制造旧 bug 的存储形态：同一个区块被追加第二次
if err := store.SaveBlock(b); err != nil {
    t.Fatalf("追加重复记录失败: %v", err)   // ← :245 必须返回 nil
}
...
if got := persistedCount(t, dir); got != 3 {   // ← :251 persistedCount = Height()+1
    t.Fatalf("构造后的持久化记录数 = %d, want 3", got)
}
```

### 2.3 互斥证明

被追加的 `b`：`PrevBlockHash == genesis.Hash()`（父高度 = 0），而存储中**已存在** `b` 本身（高度 1）。

- 若执行 M8：`SaveBlock` 必须拒绝（`height == parent.Height+1` ⇒ 要求落点为 1，但 1 已被占用；SP-3 语义即"位置必须等于父高度+1"）→ **`:245` 必然 `t.Fatalf`**。
- 若通过 `:245`：`SaveBlock` 必须接受"父不是当前链尾"的记录 → **SP-3 未闭合**。

穷举所有"更弱的严格化"均无法同时成立：

| 候选规则 | 该用例结果 | 结论 |
|---|---|---|
| `height == parent.Height + 1`（M8 原文） | 拒绝 | ✗ 破测试 |
| `parent.Height + 1 ≤ recordCount`（弱化） | 通过 | ✗ 不闭合 SP-3（旧块仍可重挂到更高位） |
| `derivedHeight == recordCount` | 拒绝（1 ≠ 2） | ✗ 破测试 |
| 仅校验父哈希存在（= 今日现状） | 通过 | ✗ 正是 SP-3 本身 |

**⇒ 冲突 1 无法在"M8 原文 + 不改既有断言"的前提内解决。**

---

## §3 冲突 2（BLOCKING）：M3 重复哈希拒绝 ⟂ 2 处既有断言

### 3.1 M3 要求

```text
duplicate block hash 必须拒绝
duplicate undo hash 必须拒绝
```

### 3.2 既有断言 A —— 写入侧

`internal/blockchain/replay_persistence_test.go:242-264`

- `:245` 重复区块**必须写入成功**（返回 nil）
- `:251` 写入后 `persistedCount == 3` ⇒ `Height() == 2`
- `:255` `storage.OpenFileBlockStore(dir)` **必须成功**
- `:260` `blockchain.NewBlockchainFromStore(s)` **必须失败**（拒绝点被固定在 **replay**，而非 open/append）

### 3.3 既有断言 B —— 只读校验侧

`cmd/node/verify_test.go:306-333`（N5）

```go
rec := data[last[0]-4 : last[0]+last[1]]   // 复制最后一条完整记录字节
dup := append(data, rec...)                 // 追加重复记录
os.WriteFile(blocksPath, dup, 0o644)
out, code, _ := verifyRaw(t, dir)           // 内部 = OpenFileBlockStoreReadOnly + VerifyStoredChain
if code == 0 { t.Fatalf(...) }              // 必须失败
if !strings.Contains(out, "前置哈希与当前链尾不匹配") {   // ← :330
    t.Errorf("N5 期望具名错误 ErrInvalidPrevHash，实际输出:\n%s", out)
}
```

`:330` 要求：失败原因必须是**回放期**的 `blockchain.ErrInvalidPrevHash`（`blockchain.go:24`：「区块的前置哈希与当前链尾不匹配」）。

**⇒ `OpenFileBlockStoreReadOnly` 必须允许重复记录并成功打开，且重复记录必须进入 canonical 链、在 replay 时才被拒绝。**

### 3.4 互斥证明

M3 提供两个可能的实现位置，二者都撞车：

| 实现位置 | 后果 |
|---|---|
| 在 `SaveBlock`（append）拒绝重复哈希 | 断言 A `:245` 破（必须返回 nil） |
| 在 open/scan 拒绝重复哈希 | 断言 A `:255`、`persistedCount`（只读 open）破；断言 B `:330` 破（错误文本变成 storage 错误，不再是 `ErrInvalidPrevHash`） |

**⇒ 冲突 2 无法在"M3 原文 + 不改既有断言"的前提内解决。**

---

## §4 冲突 3（HIGH，可用"显式解释"消解，但需用户签字）：v2 帧的写入范围 ⟂ legacy 布局断言

### 4.1 事实

`cmd/node/verify_test.go:89-108` 的测试 helper `recordSpans` **断言整份 `blocks.dat` 只由 legacy 记录构成**：

```go
// recordSpans 解析 blocks.dat 的「4 字节小端长度前缀 + 载荷」记录布局
n := int(binary.LittleEndian.Uint32(data[pos : pos+4]))
pos += 4
if n < 0 || pos+n > len(data) {
    t.Fatalf("blocks.dat 记录长度越界: 偏移 %d 长度 %d（文件 %d 字节）", pos-4, n, len(data))
}
```

且 `cmd/node/verify_test.go:274-277`（N3）按 **legacy 载荷布局**计算篡改偏移：

```go
// 载荷布局：头 88 + 交易计数 4 + 输入计数 4 + PrevTxHash 32 + OutIndex 4
//           + sig 长度 4 + sig 4 + pubkey 长度 4 + 输出计数 4 + Value 8 ...
// 偏移 148 落在一笔 coinbase 的输出金额（u64）区域
flipByteAt(t, blocksPath, last[0]+150)
```

`recordSpans` / N1 / N3 / N5 上游的夹具是 `buildChainForVerify` → `chain.AddBlock` → `blockchain.addBlock` → **`store.SaveBlock`**。

### 4.2 冲突

若按 M1 字面推断"**所有**新增区块写入都改用 v2 帧"：

- `'P','C','C','2'` 按 u32 LE 读 = `843,268,944`（这正是 F1 的反误解析设计）→ `pos+n > len(data)` → `recordSpans` 直接 `t.Fatalf` → **N1/N3/N5 全挂**。
- `last[0]+150` 恒定落在 legacy 载荷内；v2 帧下真实载荷起点后移 48 B（帧头）+ 尾部多 32 B（checksum），偏移语义失配。

### 4.3 可消解的解释（需用户确认）

| 解释 | 内容 | 对既有测试 | 对 I8 |
|---|---|---|---|
| **I-1（推荐）** | `SaveBlock`（= 生产 `blockchain.addBlock` 路径）**继续写 legacy 帧**，字节布局与今日**逐字节相同**；v2 帧只由**新增的哈希寻址/可回滚 API** 使用（`SaveBlockDetached` / `SaveBlockWithUndo` / `PutUndo` / `CommitTip` / DELETE） | ✅ 全部不动 | ✅ 天然满足（生产字节零变化，甚至更好：新写的生产块仍可被旧二进制读取） |
| I-2（字面） | 所有写入改 v2 帧 | ❌ 破 N1/N3/N5 | 需 M2 保证仅"既有前缀"不变 |

**I-1 不违反任何冻结项**：M1 只说"实现该帧格式"，未规定 `SaveBlock` 必须使用它；M2 明确要求 legacy 兼容且必须测试"v2 BLOCK 可正常读取"——I-1 下 v2 BLOCK 由新 API 产出并被读取，测试 5 仍可满足。M2 的"首次遇到 v2 record 后进入 v2 semantics"在 I-1 下解释为：**v2 记录（UNDO/TIP/DELETE）与哈希寻址语义自首个 v2 记录起启用**；因 magic 判别无歧义，legacy 帧在文件中任意位置混合都是安全的（该点须书面确认）。

---

## §5 其余既有断言（已核对：**无冲突**，可原样保留）

| 位置 | 断言 | 新设计下的结果 |
|---|---|---|
| `internal/storage/file_test.go:39-45` | genesis/#1 顺序落盘，`Height()==1` | ✅ 通过（canonical 推导在纯 legacy 文件下 = 今日语义） |
| `internal/storage/file_test.go:48-52` | 未知父区块必须被拒（`bogus` 的 PrevHash=`{0x99,…}` 非零） | ✅ 通过 |
| `internal/storage/file_test.go:86-106` | 单记录文件截断 3 B → open 必须失败 | ✅ 通过（0 条完整记录 ⇒ 无合法回退点 ⇒ REJECT；**只读与写模式都拒绝**） |
| `internal/storage/file_test.go:110-159` | 只读打开/读/写入返回 `ErrReadOnlyStore`/不存在目录报错 | ✅ 通过 |
| `internal/blockchain/replay_persistence_test.go:87-232` | 回放不写回、两次重启、新区块仍落盘、记录数==高度+1 | ✅ 通过（I-1 下记录数语义不变） |
| `cmd/node/verify_test.go:128-287`（N1–N3） | 合法链 PASS；单字节篡改 → FAIL 且命中具名错误 | ✅ 通过（I-1 下布局不变） |
| `cmd/node/verify_test.go:289-304`（N4） | 截断 16 B → verify FAIL | ✅ 通过 |
| `cmd/node/lock_lifecycle_test.go:84` | `blocks.dat` 写垃圾 → 节点启动失败 | ✅ 通过 |
| `internal/storage/datalock*_test.go`（9 项） | 锁语义 | ✅ 不涉及 |

---

## §6 影响面分析

| 维度 | 评估 |
|---|---|
| **直接影响** | `SaveBlock` 的 3 条候选收紧规则（重复哈希 / 高度连续性 / 零父哈希）与 4 处既有断言 |
| **间接影响** | M3 索引构建（duplicate 检测位置）、M7 恢复模式（拒绝点从 replay 前移到 open，错误文本与语义层级变化）、M9 测试矩阵（G-03/G-05/G-08/G-09）、`cmd/node/verify` 的对外错误契约（`ChainVerifyReport.Reason` 文本） |
| **数据** | 生产 `blocks.dat` 不受影响（本阶段只读）；无论选哪个选项，**已落盘 legacy 字节均不得变动**（I8） |
| **API/UI** | `verify` 命令的失败原因文本是对外契约（`verify_test.go:330` 断言具体中文文案）；若拒绝点前移，运维可见的诊断文案会变 |
| **测试** | 选项 B 需改动 4 处断言 → 触及 §1 禁令，必须显式授权 |
| **共识** | **零影响**：全部改动限于本地持久化层；`validateBlock` / 难度 / MTP / PoW / 版本规则零改动；且 `blockchain.addBlock`（`blockchain.go:332-349`）在调用 `SaveBlock` 前**已强制** `PrevHash == tip.Hash()`，故 SP-3/SP-3b 在**生产路径上当前不可达**（latent-only） |

---

## §7 解决方案选项（请择一，我立即继续）

### 选项 A —— 把 M3/M8 的严格化限定在 v2 / 可回滚路径（**推荐**）

- `SaveBlock`：保持 legacy 帧 + 今日校验行为**逐字节不变**（既有测试 0 改动、生产 0 变化、I8 天然成立）
- 新增严格路径：`SaveBlockDetached` / `SaveBlockWithUndo` 强制 **父存在 + `height == parent.Height+1` + 重复哈希拒绝 + 零父哈希封堵**（仅允许创世位置）
- 另加**模式升级**：一旦该 store 进入 v2 模式（出现首条 v2 记录），`SaveBlock` **也**启用严格校验（此时既有测试仍全绿，因为它们操作的是纯 legacy 文件）
- **代价**：SP-3/SP-3b 状态需重新表述为 **"v2/可回滚路径 CLOSED；纯 legacy 文件保持 documented 序列语义（安全性由 replay 拒绝提供，既有 N5/重复记录测试即该保证）"**
- **零测试改动、零 git 越界、零共识风险**，可立即开工

### 选项 B —— 保持 M3/M8 字面（`SaveBlock` 也严格），修改 4 处既有断言

- 需改动：`replay_persistence_test.go:245 / :251 / :255-260`、`verify_test.go:322 / :330`
- **直接违反 §1 禁令**（"不得修改既有测试断言以适应新实现"）→ 需你**显式授权豁免**
- 语义变化：GENESIS-0 的 P0 回归测试从"replay 拒绝损坏数据"改为"open 拒绝损坏数据"（其核心命题"损坏不得被静默转成有效链"仍成立，但**验证层级变了**，且 `verify` 的对外错误文案改变）
- 收益：SP-3/SP-3b **在源头完全闭合**，损坏更早被发现
- 风险：动了 P0 回归测试，是本阶段唯一会触及"既有覆盖"的操作

### 选项 C —— 选项 A + 追加"严格化开关"，保留 B 的收益但不改断言

- 即选项 A，并在**纯 legacy 文件**上额外提供一条**新增**测试，证明"生产路径已由 `blockchain.addBlock` 保证 `PrevHash == tip.Hash()`，故 SP-3/SP-3b 不可达"
- 不闭合 legacy 路径的 latent 缺陷，但把它固化为**可验证的边界条件**而不是未记录的风险

---

## §8 我的判断（非机械停工，而是给出最优解）

**推荐 选项 A（含 C 的加固）。** 理由：

1. **M2 已经把两种语义分开**：legacy 区 = "committed canonical historical prefix + `height = record sequence`"；v2 区 = 哈希寻址语义。把 M3/M8 的严格化落在 v2 区，是**对 M2 的贯彻**，而非对 M3/M8 的削减。
2. **SP-3/SP-3b 在生产路径上不可达**（`blockchain.addBlock` 已强制 `PrevHash == tip.Hash()`），其后果仅是"存储层原语不设防"；而 1C 将使用的是**新 API**，A 使新 API 严格 ⇒ 真实风险面被完全覆盖。
3. **I8 在 A 下从"需要小心维护"变成"结构上不可违反"**：`SaveBlock` 继续写 legacy 帧 ⇒ 生产字节零改动，连"新写的生产块仍可被旧二进制读取"这一额外兼容性都保留。
4. **代价最小且诚实**：不改一条既有断言、不动一个 P0 测试、不引入任何越界修改；代价只是把 SP-3/SP-3b 的闭合范围写清楚。
5. **选项 B 的唯一优势**（更早发现损坏）在本项目当前阶段价值有限：损坏的最终裁决权本来就在 replay（既有 N5/`TestDuplicatePersistedRecordIsRejected` 就是该设计的证据），而 A 下 v2 路径已有 checksum + 显式 hash 绑定，早发现能力并未丧失。

---

## §9 纪律声明

| 约束 | 执行 |
|---|---|
| 修改 `.go` / 测试 / 配置 / 共识参数 / blockchain / mempool / P2P / mining | **零**（本阶段至今无任何代码写入） |
| `git add` / commit / push / tag / merge / rebase / amend / squash / reset | **零** |
| 触碰 production datadir / 启动 reorg / 调用 SetTip | **零** |
| 引入第三方依赖 | **零** |
| 唯一产出 | 本报告文件 |

**IMPLEMENTATION STOPPED。等待 §7 选项授权后立即继续（选项 A 预计可直接进入 M1…M10 全量实现）。**
