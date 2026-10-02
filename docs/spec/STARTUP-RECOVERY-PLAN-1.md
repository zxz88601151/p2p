# STARTUP-RECOVERY-PLAN-1

> **状态**：本文件是**文档同步产物**（CONSOLIDATION-2，2026-10-02），仅记录 `cmd/node/service.go` 中
> **已经实现**的启动恢复接线（§4-B2：`prepareOrphanRestore` + `consumeRestorePending`）。
> **不含任何新设计、新接口、新能力。** 任何解释若与本文件冲突，以源码为准。
>
> **治理状态**：本 spec 描述的能力位于**未提交工作树**（HEAD = `52fb464` 不含 §4-B2）。
> 见 `PROJECT-AI-CONTEXT.md` §17 关键状态区分。孤儿持久化的数据格式见 `ORPHAN-DURABILITY-SPEC-v1.md`。

---

## §1 Scope & Status

| 项 | 值 |
|---|---|
| 覆盖阶段 | §4-B2（启动恢复接线） |
| 实现位置 | `cmd/node/service.go`（`prepareOrphanRestore`、`consumeRestorePending`）、`cmd/node/main.go`（D1 调用 `prepareOrphanRestore`） |
| 测试 | `orphan_recovery_4b2_test.go`（7 项专项）、`orphan_restore_test.go`、E2E `orphan_durability_e2e_test.go` |
| 提交状态 | ⛔ 未提交、未部署、未做生产变更 |

**目标**：节点崩溃/重启后，凡在停机前已入队、但其父块当时未知的孤儿，其「父键」需在启动时被恢复成一个待拉取集合；
待与对端完成握手后，复用既有的 by-hash 分支拉取（`requestBranch`）把缺失分支补全，从而使孤儿最终能被消解、链保持一致。

---

## §2 Restore Pending Set（`prepareOrphanRestore`）

`prepareOrphanRestore(cp *orphanCheckpoint)` 在节点装配阶段（D1，`p2pNode.Start()` 之前）调用，**仅填充** `s.restorePending`：

```go
func (s *nodeService) prepareOrphanRestore(cp *orphanCheckpoint) {
    s.restorePending = make(map[[32]byte]struct{})
    if cp == nil {
        return // 持久层未启用 → 退化为 memory-only
    }
    entries, err := cp.Load()
    if err != nil {
        // fail-closed：检查点损坏/不兼容 → 空恢复，退化 memory-only，绝不 panic
        log.Printf("[node] 孤儿等待检查点不可用（%v），跳过恢复（退化为内存态）", err)
        return
    }
    for _, e := range entries {
        if s.chain.HasBlockHash(e.parentHash) {
            // 父已知（canonical 或 detached）→ 无需恢复拉取，且不在 waiting，
            // 从持久投影剪枝，避免 checkpoint 累积陈旧条目（下次 flush 即剔除）。
            cp.Remove(e.parentHash)
            continue
        }
        s.restorePending[e.parentHash] = struct{}{} // map 去重
    }
}
```

**语义要点**：

- `restorePending` 是 `map[[32]byte]struct{}`（父键集合），与 `s.waiting`（`map[[32]byte][]*block.Block`）**完全独立的两个结构**。
- 父已知 → 不进入待恢复集，且从检查点投影剪枝（避免陈旧条目永久累积）。
- fail-closed：`cp == nil` 或 `Load` 失败 → `restorePending` 置空，退化为 memory-only，**绝不 panic、绝不反向影响 canonical**。
- 本阶段只**填充** `restorePending`，不消费（消费属 §3，由 `OnHandshake` 触发）。

---

## §3 Consume On Handshake（`consumeRestorePending`）

`consumeRestorePending(peerAddr string)` 在**每次握手末尾**调用，消费待恢复集：

```go
func (s *nodeService) consumeRestorePending(peerAddr string) {
    s.mu.Lock()
    if len(s.restorePending) == 0 {
        s.mu.Unlock()
        return // 快速路径：无待恢复项
    }
    pending := make([][32]byte, 0, len(s.restorePending))
    for p := range s.restorePending {
        pending = append(pending, p)
    }
    s.mu.Unlock() // 复制键集后释放锁，避免持 s.mu 做网络 I/O（防锁序嵌套）

    triggered := 0
    for _, p := range pending {
        if s.chain.HasBlockHash(p) {
            // 父已知（经正常 sync / 本次分支到达）→ drain：双删 restorePending + checkpoint
            s.mu.Lock()
            delete(s.restorePending, p)
            s.mu.Unlock()
            if s.orphanCP != nil {
                s.orphanCP.Remove(p) // 幂等；仅在父已知时执行，绝不误删仍存活的 waiting 条目
            }
            obs.Emit("ORPHAN_RESTORE_DRAINED", "parent", hashHex32(p), "peer", peerAddr)
            continue
        }
        // 父仍未知 → 复用既有 by-hash 分支拉取（inflight/TTL/rounds 去重限流）
        s.requestBranch(peerAddr, p)
        triggered++
    }
    if triggered > 0 {
        obs.Emit("ORPHAN_RESTORE_TRIGGERED", "count", uint64(triggered), "peer", peerAddr)
    }
}
```

**语义契约（对齐 §4-B2 READINESS AUDIT §2/§4.2）**：

- **仅消费「本节点仍未知」的父哈希**，复用既有 `requestBranch`（inflight / `branchReqTTL` / `maxBranchRounds` 去重限流）重新拉取缺失分支。
- **父已知 → 立即双删 drain**：`restorePending` + 检查点 `Remove`（`Remove` 幂等，且**仅在父已知时**执行，绝不误删 live waiting）。
- **父未知且已请求 → 保留**在集合中，等待下一轮握手/对端再尝试（隐式有界重试：inflight/TTL/rounds 压制成环）。
- **不伪造块指针、不旁路孤儿准入、不腐蚀 `s.waiting`**。
- **仅 `OnHandshake` 调用**，幂等、可跨多次握手推进。

**锁纪律**：`restorePending` 读写全程 `s.mu` 保护；`chain.HasBlockHash` 与 `requestBranch` 在 `s.mu` 外调用
（`requestBranch` 内部自锁 `s.mu`，不可嵌套）；`orphanCP` 内部自锁。

---

## §4 Branch Request Reuse（`requestBranch` 原语）

`consumeRestorePending` 复用既有的 by-hash 分支拉取原语 `requestBranch(peerAddr, hash)`，其约束来自
`cmd/node` 孤儿/同步常量（见 `PROJECT-AI-CONTEXT.md` §10）：

| 常量 | 值 | 语义 |
|---|---:|---|
| `maxInflightBranch` | 64 | 在途 by-hash 请求上限 |
| `branchReqTTL` | 30 s | 同一哈希请求去重窗口 |
| `maxBranchRounds` | 8 | 同一孤儿链回溯轮次上限 |
| `MaxBranchAncestors` | 64 | 单次请求期望回溯祖先数 |

机制复用保证：启动恢复分支拉取与常规孤儿分支发现走**同一套**在途/限流/轮次压制，不引入第二套分支协议。

---

## §5 Invariants

1. **两 map 独立**：`restorePending`（父键集）与 `s.waiting`（孤儿队列）互不替代；检查点只镜像父键关系。
2. **双删仅在父已知时**：`Remove` 只删已确认到达的父键，绝不误删仍存活的 `waiting` 条目。
3. **fail-closed**：检查点损坏/不可读 → 空恢复、memory-only、绝不 panic。
4. **无伪造、无旁路**：不构造块指针、不绕过孤儿准入校验、不破坏 `s.waiting` 既有语义。
5. **可重入幂等**：跨多次握手推进；`Remove`/`drain` 幂等；隐式有界重试由 inflight/TTL/rounds 压制为环。
6. **时机正确**：`prepareOrphanRestore` 在 `p2pNode.Start()` 之前完成，保证首个 `OnHandshake` 触发前 `restorePending` 已就绪。

---

## §6 Evidence

- 专项：`orphan_recovery_4b2_test.go`（7/7 PASS）、`orphan_restore_test.go`。
- 端到端：`docs/PHASE-P2PCHAIN-ORPHAN-DURABILITY-E2E-RECOVERY-1-REPORT.md`
  「恢复加载 → 握手消费 → 分支请求 → 父块到达 → 孤儿消解 → 链一致」全链路，14 项断言全绿。
- 合规：`docs/PHASE-P2PCHAIN-ORPHAN-DURABILITY-4B2-COMPLIANCE-REPORT.md`。
