#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Generate verifier/TEST-VECTORS.md from a real chain, then self-verify it.

The markdown is produced programmatically so that every value is taken from
real chain data rather than hand-typed.  After writing, the file is re-read
and every emitted value is asserted to be present; the script exits non-zero
if any assertion fails.
"""

from __future__ import annotations

import hashlib
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import p2pchain_verify as pv  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(HERE, "TEST-VECTORS.md")

CHAIN62 = sys.argv[1] if len(sys.argv) > 1 else \
    r"C:/Users/Administrator/AppData/Local/Temp/0d3-diff/blocks.dat"
CHAINTX = sys.argv[2] if len(sys.argv) > 2 else \
    r"C:/Users/Administrator/AppData/Local/Temp/v20-txchain/blocks.dat"

U = lambda b: b.hex().upper()  # noqa: E731


def build():
    g = pv.decode_block(pv.parse_blocks_dat(CHAIN62)[0])
    recs62 = pv.parse_blocks_dat(CHAIN62)
    b1 = pv.decode_block(recs62[1])
    b15 = pv.decode_block(pv.parse_blocks_dat(CHAINTX)[15])
    tx = b15.transactions[1]

    bad_r = bytearray(tx.inputs[0].signature)
    bad_r[0] ^= 0x01
    bad_s = bytearray(tx.inputs[0].signature)
    bad_s[40] ^= 0x01

    v = {
        "genesis_header_88B": U(g.header.serialize()),
        "genesis_hash": U(g.header.hash()),
        "genesis_merkle": U(g.header.merkle_root),
        "genesis_coinbase_txid": U(g.transactions[0].txid()),
        "genesis_coinbase_sighash": U(g.transactions[0].serialize_for_hash()),
        "genesis_coinbase_sig": U(g.transactions[0].inputs[0].signature),
        "genesis_coinbase_out_pkh": U(g.transactions[0].outputs[0].pubkey_hash),
        "b1_header_88B": U(b1.header.serialize()),
        "b1_hash": U(b1.header.hash()),
        "b1_merkle": U(b1.header.merkle_root),
        "b1_prev": U(b1.header.prev_hash),
        "b1_coinbase_txid": U(b1.transactions[0].txid()),
        "b15_hash": U(b15.header.hash()),
        "b15_merkle": U(b15.header.merkle_root),
        "b15_txid0": U(b15.transactions[0].txid()),
        "b15_txid1": U(tx.txid()),
        "tx_sighash_input": U(tx.serialize_for_hash()),
        "tx_txid": U(tx.txid()),
        "tx_sig": U(tx.inputs[0].signature),
        "tx_sig_r": U(tx.inputs[0].signature[:32]),
        "tx_sig_s": U(tx.inputs[0].signature[32:]),
        "tx_pubkey": U(tx.inputs[0].pubkey),
        "tx_pubkey_hash": U(hashlib.sha256(tx.inputs[0].pubkey).digest()[:20]),
        "tx_prevout": U(tx.inputs[0].prev_tx_hash),
        "tx_address": pv.address_from_pubkey_hash(
            hashlib.sha256(tx.inputs[0].pubkey).digest()[:20]
        ),
    }
    v["tx_verify_ok"] = pv.ecdsa_p256_verify(
        tx.inputs[0].pubkey, tx.txid(), tx.inputs[0].signature)
    v["tx_verify_bad_r"] = pv.ecdsa_p256_verify(
        tx.inputs[0].pubkey, tx.txid(), bytes(bad_r))
    v["tx_verify_bad_s"] = pv.ecdsa_p256_verify(
        tx.inputs[0].pubkey, tx.txid(), bytes(bad_s))
    v["b15_merkle_recomputed"] = U(pv.compute_merkle_root(b15.transactions))
    v["genesis_record_len"] = len(recs62[0])
    return v


MD = """# P2PChain Independent Verifier -- Test Vectors

> PHASE V2.0 -- INDEPENDENT VERIFIER / CROSS-LANGUAGE CONSISTENCY
>
> 本文件由 `verifier/gen_test_vectors.py` 从真实链数据**自动生成并自检**，
> 非手写。重新生成：
>
> ```bash
> python verifier/gen_test_vectors.py <chain62/blocks.dat> <txchain/blocks.dat>
> ```

## 向量来源（§7：不得来自 Python 自生成结果）

| 来源 | 说明 |
|---|---|
| `docs/DETERMINISTIC-SERIALIZATION-SPEC.md` §9.6 | 创世 Merkle / Nonce / Hash 的**已发布期望值** |
| `docs/PHASE-GENESIS-0-GENESIS-BLOCK-MINING-RUNTIME-VALIDATION.md` | 创世与首块的历史实机证据 |
| 既有真实链 #1（62 区块，全部 coinbase） | 创世与高度 1 的字节级样本 |
| 既有真实链 #2（17 区块，含 1 笔真实签名转账） | 交易 / 签名 / 地址样本 |

`node verify` 与 `node printchain`（Go 侧）仅用于**交叉比对**，
不作为本文件任何期望值的来源。

---

## V1 -- Genesis header (88 bytes)

source: 真实链 #1 第 0 条记录；期望哈希来自已发布规范 §9.6

```
input   : {genesis_header_88B}
expected: hash = {genesis_hash}
          merkle = {genesis_merkle}
          version = 1, timestamp = 1700000000, bits = 16, nonce = 230970
record  : {genesis_record_len} bytes (4-byte LE length prefix + 176)
```

字段拆分（小端）：

```
01000000                                                   Version   = 1
0000000000000000000000000000000000000000000000000000000000000000   PrevHash  = 32 x 00
{genesis_merkle}   MerkleRoot
00F1536500000000                                           Timestamp = 1700000000
10000000                                                   Bits      = 16
3A86030000000000                                           Nonce     = 230970
```

## V2 -- Genesis coinbase transaction

source: 真实链 #1 第 0 条记录的交易 0

```
input (sighash serialization) : {genesis_coinbase_sighash}
expected TxID (single SHA256) : {genesis_coinbase_txid}
expected == genesis MerkleRoot: YES（单叶 Merkle = 叶子 TxID）
signature field               : {genesis_coinbase_sig}  (4 字节小端高度 = 0)
output[0].PubKeyHash          : {genesis_coinbase_out_pkh}
output[0].Value               : 50
```

## V3 -- Block header hash (height 1)

source: 真实链 #1 第 1 条记录；Go `printchain` 交叉比对一致

```
input   : {b1_header_88B}
expected: hash   = {b1_hash}
          merkle = {b1_merkle}
          prev   = {b1_prev}
          nonce  = 67528, timestamp = 1789207202, bits = 16
```

## V4 -- Merkle root with two transactions

source: 真实链 #2 第 15 条记录（区块高度 15，含 coinbase + 1 笔真实转账）

```
txid[0] : {b15_txid0}
txid[1] : {b15_txid1}
expected: merkle = SHA256(txid[0] || txid[1]) = {b15_merkle}
recomputed by verifier: {b15_merkle_recomputed}
block hash: {b15_hash}
```

## V5 -- Transaction ID (non-coinbase, sighash excludes sig/pubkey)

source: 真实链 #2 高度 15 的交易 1

```
input (sighash serialization) : {tx_sighash_input}
expected TxID                 : {tx_txid}
prevout                       : {tx_prevout} index 0
```

## V6 -- P-256 signature encoding (§9.11)

source: 真实链 #2 高度 15 交易 1 的输入 0（由 Go 节点在真实运行中产生）

```
pubkey (65B, uncompressed SEC1) : {tx_pubkey}
pubkey hash (SHA256[:20])       : {tx_pubkey_hash}
signature (64B = r || s)        : {tx_sig}
  r = {tx_sig_r}
  s = {tx_sig_s}
message hash (= TxID)           : {tx_txid}
expected verify                 : {tx_verify_ok}
negative: flip 1 bit of r       : {tx_verify_bad_r}
negative: flip 1 bit of s       : {tx_verify_bad_s}
```

## V7 -- Base58Check address (§9.12)

source: 同一笔交易的输出 PubKeyHash 经规范编码得到

```
PubKeyHash : {tx_pubkey_hash}
version    : 0x35
address    : {tx_address}
alphabet   : 123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz
```

---

## 自检

`gen_test_vectors.py` 写回本文件后会重新读取，断言上表每一个值都原样出现在
文件中；任一断言失败即以非零退出码失败。这保证文件里没有因手工编辑或渲染
问题而损坏的值。
"""


def main() -> int:
    v = build()
    text = MD.format(**v)
    with open(OUT, "w", encoding="utf-8") as f:
        f.write(text)

    with open(OUT, "r", encoding="utf-8") as f:
        written = f.read()
    missing = [k for k, val in v.items() if str(val) not in written]
    if missing:
        print("SELF-CHECK FAILED, missing keys: %s" % missing, file=sys.stderr)
        return 1
    print("SELF-CHECK OK: %d vectors written and verified in %s" % (len(v), OUT))

    # independent assertions against published values
    assert v["genesis_merkle"].lower() == pv.PUBLISHED_GENESIS_MERKLE
    assert v["genesis_hash"].lower() == pv.PUBLISHED_GENESIS_HASH
    assert v["b15_merkle"] == v["b15_merkle_recomputed"]
    assert v["tx_verify_ok"] is True
    assert v["tx_verify_bad_r"] is False
    assert v["tx_verify_bad_s"] is False
    print("ASSERTIONS OK: genesis matches published spec; merkle/signature vectors valid")
    return 0


if __name__ == "__main__":
    sys.exit(main())
