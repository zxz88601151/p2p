# P2PChain 剩余工程 · 总体设计（MASTER DESIGN）

> 授权：中哥于 2026-09-11 授予"从现在到项目完成无需再次授权"的全量开发授权。
> 本文档是剩余全部阶段的设计决策记录（Design Decisions Record），实现以本文档为准。
> 纪律：每阶段 = 实现 + 测试 + `go build/vet/test ./...` 全绿 + 独立 commit（packed-refs 加固）。
> 全程保持定位：学习型 PoW 区块链骨架；标准库零依赖；禁止引入区块链以外的功能。

---

## 现状（HEAD 0d69b30）

- 已有：block（头/双 SHA256/Merkle 单 SHA256）、pow（含 0.1 修复）、transaction（UTXO 模型）、wallet（P-256 签名/验签）、blockchain（仅 PrevHash+PoW 校验）、p2p（换行分隔 JSON、哈希广播桩）、storage（接口 + 内存实现，未接入）、config、cmd/node（单文件骨架）。
- 已知缺口：无 UTXO 状态、无交易校验、无签名接入、无 Coinbase 校验、无 Merkle 重验、无持久化接入、无 mempool、P2P 不传播真实数据、无地址格式、无交易构建、无 CLI。

---

## PHASE 1B — UTXO 与校验（internal/utxo + blockchain 重写）

### 数据结构
- `OutPoint{Hash [32]byte; Index uint32}`：唯一标识一个输出。
- `Entry{Output transaction.TxOutput; Height int; IsCoinbase bool}`：Height 用于 coinbase maturity。
- `UTXOSet`：`map[OutPoint]Entry` + RWMutex。方法：`Add / Get / Spend / Has / Balance / Clone / Len`。
- 原子性：`ApplyBlock` 在 **Clone** 上依次 Apply，全部成功后整体替换——杜绝 partial state transition；无中间状态对外可见（单锁保护）。

### 校验（Validate First → Apply Second）
`ValidateTransaction(tx, set, height) (fee uint64, err)`（纯函数语义，先验证后变更 clone）：
- 结构：非 coinbase 必须有输入（ErrNoInputs）；必须有输出（ErrNoOutputs）；输出 Value>0（ErrZeroOutput）。
- 重复输入检测（ErrDuplicateInput）。
- 输入存在性 + 未花费（clone 顺序消费，二次消费即 ErrUnknownUTXO——覆盖块内双花）。
- Coinbase maturity：花费 coinbase 输出需 `height - Entry.Height >= CoinbaseMaturity`（ErrImmatureCoinbase）。
- 金额：`sum(in) >= sum(out)`（ErrOverspend），fee = 差额。
- 签名：`sha256(input.PubKey)[:20] == 前置输出.PubKeyHash`（ErrPubKeyMismatch），`wallet.Verify(pubkey, sighash, sig)`；malformed pubkey 映射 ErrBadSignature。
- **sighash = tx.Hash()**（序列化天然排除 Signature/PubKey）。安全论证：pubkey 未入 sighash，但 pubkey 被 `PubKeyHash` 锁定比较绑定——替换 pubkey 必然导致 hash 不匹配 + 签名无法通过；输出/输入引用均在 sighash 覆盖内（改输出/输入 → 验签失败）。
- Coinbase 结构校验：唯一输入、空签名/空公钥、输出非空且 Value>0。

### 共识参数（新常量）
- `Subsidy(height) = 50 >> (height/210)`（每 210 块减半，≥64 次后为 0）。
- `CoinbaseMaturity = 10`（测试网取向；比特币为 100，注释说明）。
- `MaxBlockSize = 1 MiB`（以 block 的 JSON 规范序列化长度计，跨节点一致）。

### 区块校验全序（blockchain.ValidateBlock）
1. PrevHash == tip.Hash（ErrInvalidPrevHash）
2. PoW（ErrInvalidPoW）
3. Bits == CurrentBits()（ErrUnexpectedBits）
4. 时间戳：`>= tip.Timestamp` 且 `<= now+7200s`（ErrTimestampOutOfRange）
5. Merkle 重验：`ComputeMerkleRoot(txs) == Header.MerkleRoot`（ErrMerkleMismatch）
6. 大小：JSON 序列化 ≤ 1MiB（ErrBlockTooLarge）
7. 交易：≥1 笔；恰好一笔 coinbase 且在 index 0（ErrFirstTxNotCoinbase / ErrMultipleCoinbase）
8. 状态迁移：clone UTXO 上逐笔 Validate+Apply 累计 fee；coinbase 输出总和 ≤ Subsidy(height)+fees（ErrExcessiveCoinbase）
9. AddBlock = Validate + append + utxo 整体替换（原子）。

## PHASE 2 — 钱包 / 地址 / 交易构建

- `internal/wallet/base58.go`：Base58 + Base58Check（版本字节 0x35，自定义网络）。
- 地址 = Base58Check(0x35 || PubKeyHash20B)；`w.Address()`；`DecodeAddress(addr) ([20]byte, error)`（含 4 字节双 SHA256 校验和验证）。
- `internal/wallet/persist.go`：JSON 文件（D/X/Y hex）落盘 `wallet.dat`，0600 权限；Load。
- `internal/txbuild`（独立包避免 wallet↔utxo 循环依赖）：`BuildTransaction(set, from *wallet.Wallet, to [20]byte, amount, feePerByte)`：
  - 选币：仅 mature 且非未成熟 coinbase 的本地址 UTXO，按金额降序累计至 `amount+fee`；
  - 构造输出（接收方 + 找零，找零>0 才创建）、签名（sighash = tx.Hash()）；
  - 返回 (tx, fee)；余额不足 ErrInsufficientFunds。

## PHASE 3 — Mempool（internal/mempool）

- `map[TxID]tx` + FIFO 序 + RWMutex。
- `Add(tx, baseUTXO)`：TxID 去重；在「baseUTXO clone + 依次应用池内待处理 tx」的组合视图上跑 ValidateTransaction（覆盖块内双花与池内冲突）；ErrKnownTx / ErrConflict。
- `RemoveIncluded(block)`：移除上链交易；剩余交易在新 UTXO 上重验，失效者剔除。
- `Pending(maxCount)`：按 fee 降序返回供挖矿打包；`Fee(tx)`。

## PHASE 4 — 持久化（block codec + storage/file）

- `block.Encode/Decode/Size`：规范二进制（定长头 + varint 计数 + 长度前缀字段），用于大小限制与磁盘格式（替代 1B 的 JSON 尺寸？——保持 1MiB 限制基于 Encode 尺寸，跨节点确定一致）。
- `storage.FileBlockStore`：`blocks.dat` 追加写（长度前缀）+ 启动重建索引（hash→block, height→block）；`SaveBlock/GetBlockByHash/GetBlockByHeight/Height`；互斥保护。MemoryBlockStore 保留供测试。
- `blockchain.NewBlockchainFromStore(store)`：空库→创建确定性创世并落盘；非空→按高度回放重建 UTXO；每次 AddBlock 同步 SaveBlock。

## PHASE 5 — P2P 真实传播（internal/p2p 扩展）

- 保持 TCP + 换行分隔 JSON（简单、帧清晰、stdlib）。消息补全：`MsgGetBlocks{FromHeight}` / `MsgBlocksResp{Blocks []Block}`（上限 500/批）。
- Payload 携带完整 Block/Tx（JSON，[]byte 自动 base64）。
- Node 增强：`BroadcastExcept(msg, except)`（中继防回环）、死连接清理、`PeerCount/PeerAddrs`、握手交换已知 peer 列表（Gossip 发现）、断线重连种子。
- 去重：nodeService 维护 seen set（区块哈希/TxID），避免重复处理与广播风暴。
- 同步：握手发现更高链 → 发 MsgGetBlocks → 分批拉取 → 逐块 ValidateBlock+AddBlock。

## PHASE 6 — 节点组装（cmd/node 重写 + 确定性创世）

- **确定性创世**：`blockchain.NewGenesisBlock()`——固定时间戳、固定 coinbase（黑色洞地址 `0x00..01`）、MaxTargetBits 挖出；全网络同一创世，多节点可同步。
- CLI 子命令：`wallet`（创建/加载钱包到 DataDir）、`getbalance -addr`、`send -wallet -to -amount -fee`、`printchain`、`start -listen -seed -datadir -miner`。
- nodeService：实现全部 Handler（OnNewBlock→验证/上链/更新池/中继、OnNewTx→验证/入池/中继、OnHandshake→必要时同步、OnGetBlocks/OnBlocksResp）；挖矿取消机制（新 tip 到达即重组候选）。
- miner：goroutine 循环——打包池内交易（fee 降序）+ coinbase(subsidy+fees) → Mine(带取消) → AddBlock → 广播。

## PHASE 7 — 文档与收尾

- README（p2pchain/ 与外层同步）：功能清单、构建运行、CLI 用法、协议说明、已知限制。
- 全量回归 + 最终报告 `docs/FULL-IMPLEMENTATION-REPORT.md`。

## 明确不做（超出学习项目边界）
- 分叉/reorg 树状链（保留 TODO 与最长链原则说明，当前单链追加）；RIPEMD160/secp256k1（stdlib 限制，注释说明升级路径）；SPV/轻节点；TLS/加密传输；代币经济。
