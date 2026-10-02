# ORPHAN-DURABILITY-SPEC-v1

> **状态**：本文件是**文档同步产物**（CONSOLIDATION-2，2026-10-02），仅记录 `cmd/node/orphan_checkpoint.go`
> 与 `cmd/node/main.go` 中**已经实现**的行为（§4-B1 数据层 + §4-B1.5 生产激活）。
> **不含任何新设计、新接口、新能力。** 任何对孤儿持久化行为的解释若与本文件冲突，以源码为准。
>
> **治理状态**：本 spec 描述的孤儿耐久实现（§4-B1/§4-B1.5/§4-B2）**已提交**于 `36f9630`（feat(node): orphan durability），协议真相同步**已提交**于 `8410036`（docs: synchronize documentation truth），当前 HEAD = `8410036`。**尚未打 tag、尚未部署、尚未做生产发布**。
> 见 `PROJECT-AI-CONTEXT.md` §17 关键状态区分。

---

## §1 Scope & Status

| 项 | 值 |
|---|---|
| 覆盖阶段 | §4-B1（数据层）、§4-B1.5（生产激活 D1+D2） |
| 实现位置 | `cmd/node/orphan_checkpoint.go`、`cmd/node/main.go`（D1 接线） |
| 测试 | `orphan_checkpoint_test.go`、`orphan_checkpoint_hook_test.go`、`orphan_durability_test.go`、`orphan_restore_test.go`、E2E `orphan_durability_e2e_test.go` |
| 提交状态 | ⛔ 未提交、未部署、未做生产变更 |

**职责边界**：本文件只规定检查点的**数据格式、序列化/反序列化、校验、原子写原语**，以及它在生产中的**生命周期接线**。
它**不**触碰 canonical 链 / UTXO / fork-choice / reorg / mempool / P2P 协议；它**不**与 `nodeService` 运行时状态耦合
（启动恢复的消费逻辑见 `STARTUP-RECOVERY-PLAN-1.md`）。

---

## §2 Data Model

**文件**：`<datadir>/orphan_waiting.bin`

检查点是「父哈希 → 子块哈希列表」的**持久化投影**，是孤儿等待队列 `s.waiting`
（`map[[32]byte][]*block.Block`，纯内存）的**跨重启存活镜像**。

**头部（41 字节）**：

| 偏移 | 长度 | 字段 |
|---:|---:|---|
| 0 | 4 | magic `"ORPH"`（ASCII） |
| 4 | 1 | version `0x01` |
| 5 | 4 | entryCount `uint32` 大端 |
| 9 | 32 | checksum `SHA-256`（覆盖 version + entryCount + 全部 entry 字节，**不含** magic 与 checksum 自身） |

**entry（36 + 32×childCount 字节）**：

| 偏移 | 长度 | 字段 |
|---:|---:|---|
| 0 | 32 | parentHash（`[32]byte`，32 字节块哈希） |
| 32 | 4 | childCount `uint32` 大端 |
| 36 | 32×N | childHash[]（每子块 32 字节，保持入队序） |

**大小上界（§1.2）**：entryCount ≤ 256，每 entry childCount ≤ 64，总 childHash ≤ 16,384，
文件 ≤ 16,384×32 + 41 ≈ 524,329 B。

**确定性**：序列化顺序 = entry 追加序（= 父键到达序），childHash 顺序 = 入队序。相同投影字节级一致。

---

## §3 Serialization

`serializeLocked()`（`orphanCheckpoint` 内部，调用方须持 `mu`）：

1. 写 `ORPH` + `0x01`；
2. 预留 entryCount（4B，回填）；预留 checksum（32B，回填）；
3. 逐 entry 写 `parentHash` + `childCount` + `childHash[]`；
4. 回填 entryCount（`len(entries)`）；
5. 计算 `SHA256(version ‖ entryCount ‖ entries)`（即 `buf[4:checksumOff]` 拼接 `buf[checksumOff+32:]`），
   回填 checksum。

---

## §4 Deserialization & Validation（fail-closed）

`parseOrphanCheckpoint(data)`（纯函数）：

1. `len(data) < 41` → `errOrphanCPTruncated`；
2. `data[0:4] != "ORPH"` → `errOrphanCPBadMagic`；
3. `data[4] != 0x01` → `errOrphanCPBadVersion`；
4. `entryCount > 256` → `errOrphanCPBounds`；
5. 重算 checksum 与 `data[9:41]` 比对不一致 → `errOrphanCPChecksum`；
6. 逐 entry 解析；越界 → `errOrphanCPTruncated`；
7. **容错丢弃**：`childCount == 0` 或 `> 64` 的 entry 跳过（其余保留）；`parentHash` 为零值的 entry 跳过。

`loadLocked()`：文件不存在（`os.ErrNotExist`）→ 返回空（无错误）；其余错误 → 返回错误，
由调用方「丢弃文件、空启动」，**绝不 panic**、绝不反向影响 canonical。

> §4-B1.5 D2 修正：`loadLocked` 在成功解析后把磁盘投影**灌入内存唯一事实源** `c.entries`，
> 使后续 `MarkDirty`/`Remove`/`Flush` 在已加载条目基础上增量合并，而非从空态覆盖（否则首次 Flush 会清空已持久恢复集）。

---

## §5 In-memory Projection Ops

| 方法 | 语义 | 幂等性 |
|---|---|---|
| `markDirtyLocked(parent, child)` | 父已存在则追加 childHash（去重）；否则新建 entry（追加序）；置 `dirty` | 子哈希去重 |
| `removeLocked(parent)` | 删除该父键 entry；不存在无副作用 | ✅ 幂等（T8） |
| `shouldFlushLocked(now, nDirty, interval)` | `dirty` 为真 且（`nDirty ≥ 64` 或距上次 flush ≥ interval）才返回 true | — |

锁纪律：`orphanCheckpoint.mu` **独立于** `nodeService.mu`。`nodeService.mu` 临界区内**不得**调用本类型任何加锁方法；
`markDirty`/`remove`/`flush` 均在 `nodeService.mu` 之外调用。

---

## §6 Atomic Write（Flush）

`flushLocked()`（temp + fsync + rename）：

1. 序列化 → `data`；
2. `OpenFile(tmpPath, O_WRONLY|O_CREATE|O_TRUNC, 0600)`；
3. 写全部字节；
4. `tmp.Sync()`（**仅 flush 时 fsync**）；
5. `tmp.Close()`；
6. `os.Rename(tmpPath, path)`（原子替换）；
7. 清 `dirty`、更新 `lastFlush`。

失败语义（§2.4）：任何失败只影响孤儿可用性，**绝不反向影响 canonical**，调用方仅 warning，不退化为错误，孤儿功能继续。

`CleanupTmp()`：启动时移除可能残留的 `.tmp`（崩溃半写遗留），幂等。

---

## §7 §4-B1.5 Production Activation

孤儿持久化在生产中真正生效需两个接线点（D1 + D2），均在 `cmd/node/main.go` 节点装配阶段：

**D1 — 初始化与启动加载（必须在 `p2pNode.Start()` 之前）**：

```go
orphanCP := newOrphanCheckpoint(cfg.DataDir)   // 指向 <datadir>/orphan_waiting.bin
orphanCP.CleanupTmp()                          // 清除崩溃遗留 .tmp
svc.orphanCP = orphanCP                        // 注入 svc（nil = 持久层恒为 no-op）
svc.prepareOrphanRestore(orphanCP)             // 启动加载：缺失/corrupt 均 fail-closed
log.Printf("[node] 孤儿等待检查点已启用: %s", orphanCP.Path())
```

**D2 — 周期落盘生命周期**：`runSyncSweep`（唯一执行重试的巡检 goroutine）经
`orphanCP.FlushIfDirty(orphanCPFlushInterval=5s)` 节流写盘。`FlushIfDirty` 语义：

- `dirty` 为假 → 跳过（返回 `false, nil`）；
- 距上次成功 flush 不足 5s（且非首次）→ 跳过，避免无谓写盘；
- 否则原子写盘（temp+fsync+rename），返回 `true`。

> 两个接线点缺一不可：D1 未注入 `orphanCP` 则持久层恒为 no-op；D2 无 `Flush` 调用方则文件永不落盘。
> 二者在 §4-B1.5 合规审计中均为 blocker，现已补齐。

---

## §8 Invariants & Safety

1. **失败封闭（fail-closed）**：任何检查点失败只影响孤儿可用性，绝不反向影响 canonical 链 / UTXO / fork-choice。
2. **独立锁**：`orphanCheckpoint.mu` 不持 `nodeService.mu` 临界区；无锁序嵌套。
3. **投影一致性**：`s.waiting`（内存孤儿队列）与 `orphanCP`（磁盘投影）是两个独立结构；
   检查点只镜像「父键 → 子哈希」关系，不持有块体、不替代 `s.waiting`。
4. **原子替换**：写入走 temp+fsync+rename，崩溃窗口至多残留 `.tmp`（下次启动 `CleanupTmp` 清除）。
5. **大小有界**：entryCount ≤ 256、每 entry childCount ≤ 64，文件 ≤ ~524 KB，防滥用。

---

## §9 Evidence

- 单元测试：`orphan_checkpoint_test.go`（序列化/反序列化/校验/原子写/边界）、`orphan_checkpoint_hook_test.go`（与 `s.waiting` 挂钩）。
- 耐久/恢复：`orphan_durability_test.go`、`orphan_restore_test.go`。
- 端到端：详见 `docs/PHASE-P2PCHAIN-ORPHAN-DURABILITY-E2E-RECOVERY-1-REPORT.md`（真实网络/磁盘/进程边界全链路，14 项断言全绿）。
- 合规：`docs/PHASE-P2PCHAIN-ORPHAN-DURABILITY-4B15-COMPLIANCE-REPORT.md`、`docs/PHASE-P2PCHAIN-ORPHAN-DURABILITY-4B2-COMPLIANCE-REPORT.md`。
