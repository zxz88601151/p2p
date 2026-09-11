# P2PChain 全量实现报告（FULL IMPLEMENTATION REPORT）

> 日期：2026-09-11　HEAD：`e16abd0`
> 定位：学习型 PoW 区块链骨架；Go 1.22，零外部依赖（仅标准库）。
> 本报告覆盖 PHASE 1B → PHASE 7 的全部实现内容与验收证据，是项目的最终交付文档。

---

## 1. 项目总览

从「内存链 + 哈希广播桩」的骨架出发，补全了一条**真实可跑**的 UTXO 区块链：完整交易校验、持久化、多节点真实传播与同步、钱包转账、并行挖矿、控制接口与 CLI。

**最终规模**：12 个包 + 1 个可执行程序，约 8,500 行 Go（含测试）；**114 个测试用例**全部通过；`-race` 全绿；双真实进程冒烟测试 13/13 通过。

```
block → transaction → utxo → blockchain → mempool → txbuild
                                           ↑
        wallet ─────────────────────────────┘（签名/地址/持久化）
        p2p（传播/同步）→ cmd/node（组装）→ control（本地 API）→ cli
```

## 2. 各阶段实现与验收

### PHASE 1B — UTXO 与校验（commit `aa0e6e1`）

- `internal/utxo`：`OutPoint{Hash, Index}` 唯一标识输出；`Entry` 携带 Height（coinbase maturity）与 IsCoinbase；`UTXOSet` = map + RWMutex；`ApplyBlock` 在 clone 上依次应用、全部成功后整体替换——无 partial state transition。
- 交易校验（Validate-First → Apply-Second）：结构（非 coinbase 必须有输入/必须有输出/输出值 > 0）、重复输入、块内双花（clone 顺序消费）、coinbase 成熟期（10 块）、金额守恒（fee = in − out）、公钥哈希锁定 + P-256 验签（sighash = tx.Hash()）、coinbase 结构约束。
- 区块校验九步全序：PrevHash → PoW → Bits → 时间戳（≥ tip 且 ≤ now+7200s）→ Merkle 重验 → 大小（1 MiB）→ coinbase 位置唯一 → clone 上状态迁移 + coinbase 总额 ≤ Subsidy+fees → 原子上链。
- 共识参数：`Subsidy(h) = 50 >> (h/210)`、`CoinbaseMaturity = 10`、`MaxBlockSize = 1 MiB`。
- 验收：`internal/utxo` 16 用例 + `internal/blockchain` 9 用例全绿（含全部错误路径逐一触发）。

### PHASE 2 — 钱包 / 地址 / 交易构建（commit `ca4ca08`）

- `internal/wallet`：Base58Check（版本字节 0x35，双 SHA256 四字节校验和）；`DecodeAddress` 校验失败返回错误；JSON 持久化（0600）。
- `internal/txbuild`（独立包，打破 wallet↔utxo 循环依赖）：选币（仅 mature 本地址 UTXO，降序累计）→ 接收方输出 + 找零（>0 才创建）→ 逐输入签名；余额不足返回 `ErrInsufficientFunds`。
- 验收：wallet 17 用例 + txbuild 5 用例。
- **后续 P1 修复（commit `f8fefc1`）**：签名编码从变长 `append(r,s)` 改为 **64 字节定长 r||s（`big.Int.FillBytes`）**——原实现在 r/s 首字节为 0 时长度 < 32，中点切分会错位，导致约 1/128 概率验签失败（race 检测下偶发复现的 mempool 测试抖动根因）。附带确定性回归测试。

### PHASE 3 — Mempool（commit `ca4ca08`）

- `map[TxID]tx` + FIFO + RWMutex；`Add` 在「链 UTXO clone + 池内待处理交易依次应用」的组合视图上跑完整验证（覆盖池内双花/冲突）；`RemoveIncluded` 出块后在新 UTXO 上重验剔除失效交易；`Pending(n)` 按 fee 降序供打包。
- 验收：mempool 9 用例（含池内冲突、双花、重验剔除）。

### PHASE 4 — 持久化（commit `33dbf24`）

- `block.Encode/Decode`：规范二进制（定长头 + varint 计数 + 长度前缀字段），跨节点字节级一致。
- `storage.FileBlockStore`：`blocks.dat` 追加写（长度前缀帧），启动重建索引并全量回放（每块完整校验）重建 UTXO；只读模式（`OpenFileBlockStoreReadOnly`）供离线 CLI。
- `NewBlockchainFromStore`：空库 → 挖确定性创世并落盘；非空 → 回放。
- 验收：storage 3 用例 + blockchain 重启回放用例。

### PHASE 5 — P2P 真实传播（commit `58b05e8`）

- 消息补全：`handshake`（交换高度/genesis 哈希/已知节点列表）、`new_block`/`new_tx`（携带完整区块/交易）、`get_blocks`/`blocks_resp`（分批 ≤200）。
- `BroadcastExcept` 中继防回环；消息大小上限、读写超时、死连接清理。
- `cmd/node/nodeService` 全 Handler 接线：OnNewBlock → 解码 → 完整校验 → 上链 → 清池 → 中继；OnNewTx → 入池 → 中继；OnHandshake → 发现更高链则发起追赶同步；OnBlocksResp → 逐块校验上链、未到链尾继续拉取。
- **测试工程决策**：`Serve(ln)` 监听器注入模式——测试先创建监听器再注入，端口就绪成为确定性事实，消除「dial 探测」竞态。
- **数据竞态修复**：`SetHeightProvider`/`currentHeight` 互斥化（`-race` 抓出）。
- 验收：p2p 5 用例 + nodeService 4 个真实 TCP 端到端用例（传播/追赶/拒绝无效块/重复幂等）。

### PHASE 6 — 节点组装 / CLI / 控制接口（commit `25bc1e9`）

- `internal/control`：localhost JSON API（`/status /balance /utxos /send /mine /block`），`Node` 接口解耦业务；错误映射（400/404/405/409）。
- `cmd/node`：`nodeConfig`/`nodeRuntime`（测试与生产同一组装路径）；`runNode` 选项（`-listen/-rpc/-seed/-datadir/-mine/-maxblocks/-miners`）；CLI 子命令（`status/balance/utxos/send/mine/wallet/printchain/help`）——全部写注入的 io.Writer + 返回退出码，测试可直接调用。
- `scripts/smoke-e2e.sh`：双真实进程出块→同步→转账→打包→双方余额→重启持久化，13/13。
- 验收：cmd/node 6 个全栈用例（含 coinbase 成熟期边界、UTXO 级金额守恒断言、重启持久化、CLI 全命令）。

### PHASE 7 — 并行挖矿 / 种子重连 / 收尾（commit `e16abd0` + 本文档）

- `pow.MineCancelable`：多 worker 等差类切分搜索空间（不重复不遗漏）；cancel 关闭后全部 worker 尽快退出；串行路径保持确定性（创世依赖）；热路径零堆分配（target 预转 32 字节定长 + `bytes.Compare`，实测 ~60ms/块）。
- 节点挖矿循环：候选区块在工作 goroutine 求解，主 goroutine 监听链尾变化信号，变化即 close(cancel) 并等待收尾——不白烧 CPU、不泄漏 goroutine。
- 种子节点周期性「只补不足」重连（5s）：防孤岛链。
- **`nodeRuntime.Close` 幂等化（sync.Once）**：修复测试模拟重启时 double-close panic——真实缺陷，非测试问题。
- 测试清理：删除两处恒真无法证伪的 `nonce%workers >= workers` 断言；补取消语义（并行/串行路径 goroutine 泄漏检测）、单/多 worker 等价性、高难度用例。
- 验收：pow 17 用例；`TestSeedReconnectAfterRestart`（种子重启后自动重连）、`TestParallelMiningProducesValidBlock`（4 worker 挖 6 块逐块 PoW + 链连续性校验）。

## 3. 最终回归证据

| 检查 | 结果 |
|---|---|
| `go build ./...` | ✅ 通过 |
| `go vet ./...` | ✅ 通过 |
| `go test ./... -count=1` | ✅ **114/114**（12 包全 ok） |
| `go test -race ./... -count=1` | ✅ 全绿（cmd/node 31.6s，含真实 TCP 端到端） |
| `bash scripts/smoke-e2e.sh` | ✅ **13/13**（双真实节点进程全流程） |

各包用例分布：cmd/node 12 · block 8 · blockchain 9 · control 8 · mempool 9 · p2p 5 · pow 17 · storage 3 · transaction 5 · txbuild 5 · utxo 16 · wallet 17。

## 4. 提交历史

```
e16abd0 feat(pow): 并行挖矿与可取消挖矿；种子节点断线重连；Close 幂等化
25bc1e9 feat(cli): 本地控制接口、命令行子命令与端到端冒烟脚本
58b05e8 feat(p2p): 真实区块/交易传播、追赶同步与节点业务接线
f8fefc1 fix(wallet): 签名改用 64 字节定长 r||s 编码，修复 ECDSA 偶发验签失败
33dbf24 feat: block codec, file-backed block store and persistent chain loading
ca4ca08 feat: base58check addresses, wallet persistence, tx builder and mempool
aa0e6e1 feat: utxo state machine with full transaction and block validation
0d69b30 docs: add phase 1a consensus audit report
5e513be fix: correct difficulty target-to-bits conversion
aacf87d docs: add phase 0 engineering baseline report
0862f06 chore: establish p2pchain engineering baseline
```

## 5. 工程决策记录（关键取舍）

1. **Validate-First → Apply-Second**：一切状态变更先在 clone 上完整验证，成功后原子替换。杜绝部分应用导致的中间状态。
2. **sighash = tx.Hash()**：序列化天然排除签名/公钥字段；公钥经 PubKeyHash 锁定比较绑定到前置输出，篡改任何关键字段都会导致验签失败或哈希不匹配。
3. **监听器注入**（`Serve(ln)`）：把「端口就绪」从时序概率变成确定事实，是消除网络测试抖动的根本手段。
4. **串行路径保留**：并行挖矿不保证最小 nonce；确定性创世（全网络字节级相同）必须走串行路径。
5. **单链不 reorg**：超出学习边界，明确不做（见下节）。
6. **难度位 16**：测试网调优（期望 2^16 次哈希/块，CPU 毫秒级出块），是共识参数——有节点长期运行后不可改（会改变创世哈希）。

## 6. 明确不做（边界）

分叉/reorg 树状链 · secp256k1/RIPEMD160（stdlib 限制，用 P-256 + SHA256 截断替代）· SPV 轻节点 · TLS/对等认证 · 代币经济模型。升级路径已在代码注释中说明。

## 7. 后续可选方向（若继续演进）

- reorg：树状区块索引 + 累计工作量比较 + UTXO 快照回滚
- 真实 nBits 压缩编码（比特币兼容）
- peer 发现加密（噪声协议）与节点黑名单
- 交易费市场（按字节费率而非固定 fee）
