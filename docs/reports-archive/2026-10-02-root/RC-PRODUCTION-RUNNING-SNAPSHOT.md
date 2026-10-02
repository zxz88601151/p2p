# PHASE P2PCHAIN — RC PRODUCTION RUNNING SNAPSHOT
## 腾讯云三节点 v0.9.0-rc1 生产候选运行基线快照

- **阶段**：`PHASE-P2PCHAIN-RC-PRODUCTION-BASELINE-FREEZE-1` §0~§2
- **性质**：只读冻结，未重启、未改文件、未触发 mining、未清理旧 binary。
- **冻结时间**：`2026-10-02 11:08 CST`
- **RC binary**：SHA256 `b26181173ba1ce8795a598f89c1fb5ee64f84c7cf5b2fbe0fa77709ad24877bb`

---

## §0 三节点运行快照

| 节点 | PID | uptime | 角色 | P2P | RPC |
|------|-----|--------|------|-----|-----|
| A | 3045383 | 4m44s | seed 枢纽 | 16688 | 16689 |
| B | 3045101 | 5m08s | -seed→A | 16690 | 16691 |
| C | 3044810 | 5m34s | -seed→A | 16692 | 16693 |

### Binary 身份（三节点共享同一文件）

| 项 | 值 |
|----|----|
| SHA256 | `b2618117...` ✅ |
| go 版本 | go1.27.0 |
| trimpath | true |
| CGO_ENABLED | 0 |
| GOOS / GOARCH | linux / amd64 |
| buildvcs | false（无 vcs.revision 内嵌） |

---

## §1 链状态冻结

### 三节点共识状态

| 节点 | height | tip hash | difficulty | mining | accepted | mempool |
|------|--------|----------|-----------|--------|----------|---------|
| A | 137 | `0000a54c8e4c310f6cd85daf3ddf36785adad7656342bdd73308b97413842702` | 1 (bits=16) | STOPPED (on-demand-done) | 1 | 0 |
| B | 137 | 同上 ✅ | 1 | STOPPED (init) | 0 | 0 |
| C | 137 | 同上 ✅ | 1 | STOPPED (init) | 0 | 0 |

### Tip 区块（height=137）字段

| 字段 | 值 |
|------|----|
| version | 1 |
| previous_hash | `0000e923a6f536b23dc8dc1ebe1e5d4b27d5fce657d7d7507e4d63fed09ed544`（= height 136 tip） |
| merkle_root | `175f6f39c0912911af0f784735d40ee4e90b5dc1075fa33045c7113cf5e27cee` |
| timestamp | 1790910387 = **2026-10-02 03:06:27 UTC**（= 北京 11:06:27，出块时刻） |
| bits | 16（difficulty=1） |
| nonce | 170678 |

### 创世块（height=0）identity

- previous_hash = `0000...0000`（全零，创世块正确特征）
- merkle_root 前缀 `4d86b378...`，与历史审计创世 merkle `4d86b378a37c05f8963c781d0db121c9...` 一致 ✅
- 创世哈希 `00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3`（历史审计基准）无漂移

### tx 状态

- mempool = 0（三节点一致，无待打包交易）
- tip 区块交易数 = 1（coinbase 出块奖励，符合 on-demand 出块预期）

---

## §2 网络拓扑冻结

### Peer 拓扑

| 节点 | peer 数 | 连接对象 |
|------|---------|---------|
| A | 2 | B（127.0.0.1:55750）、C（127.0.0.1:57166）反向连接 |
| B | 1 | A（127.0.0.1:16688） |
| C | 1 | A（127.0.0.1:16688） |

### 实际 TCP 网格（星型）

```
127.0.0.1:16688 (A) → 127.0.0.1:55750 (B)
127.0.0.1:16688 (A) → 127.0.0.1:57166 (C)
127.0.0.1:55750 (B) → 127.0.0.1:16688 (A)
127.0.0.1:57166 (C) → 127.0.0.1:16688 (A)
```

### Handshake 状态

| 节点 | 升级后 handshake 次数 |
|------|----------------------|
| A | 51 |
| B | 18 |
| C | 24 |

- 握手连续健康，无 genesis mismatch 拒绝事件。

### Genesis Identity 状态

- O1 genesis identity guard 正常：升级后握手日志无「创世块不一致」拒绝记录。
- 三节点创世块 identity 一致（均从同一创世哈希派生），无跨网异源风险。

---

## 冻结结论

🟢 **运行基线已冻结**。三节点以 v0.9.0-rc1（`b2618117...`）稳定运行，链在 height=137 共识一致，星型拓扑健康，genesis identity 无漂移，无异常。
