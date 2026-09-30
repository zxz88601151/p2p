#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""P2PChain independent verifier (Python 3, standard library only).

PHASE V2.0 -- INDEPENDENT VERIFIER / CROSS-LANGUAGE CONSISTENCY.

Purpose
-------
Re-implement the whole-chain verification rules **from the public
Deterministic Serialization Specification only**, then run them against a
real `blocks.dat` and compare the result with the Go implementation
(`node verify`).

Independence constraints (§2 of the phase prompt)
-------------------------------------------------
  * Does NOT import, exec, wrap or shell out to any Go code.
  * Does NOT copy Go production source.
  * Does NOT read or trust any output produced by `node verify`.
  * Does NOT hardcode expected block hashes or a hardcoded PASS.
  * The only inputs are: the specification document + `blocks.dat`.

Usage
-----
    python p2pchain_verify.py <datadir | blocks.dat> [--json] [--now UNIX_TS]

Exit codes
----------
    0  chain verified (PASS)
    1  chain rejected / cannot be verified (FAIL)
    2  usage error
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import struct
import sys
import time

# --------------------------------------------------------------------------
# Consensus constants (§9.x of the specification)
# --------------------------------------------------------------------------

HEADER_SIZE = 88
BLOCK_SIZE_LIMIT = 1 << 20                 # 1 MiB
MAX_RECORD_SIZE = 64 << 20                 # storage-level defensive cap
MAX_TX_PER_BLOCK = 100_000
MAX_IN_PER_TX = 10_000
MAX_OUT_PER_TX = 10_000
MAX_SCRIPT_LEN = 10_000

COINBASE_MATURITY = 10
SUBSIDY_INITIAL = 50
SUBSIDY_HALVING_INTERVAL = 210

MAX_TARGET_BITS = 16
MAX_DIFFICULTY_BITS = 16
TARGET_BLOCK_TIME_SECONDS = 60
DIFFICULTY_ADJUSTMENT_INTERVAL = 20
MAX_FUTURE_TIMESTAMP_DRIFT = 7200

COINBASE_OUT_INDEX = 0xFFFFFFFF
SIGNATURE_SIZE = 64
ADDRESS_VERSION = 0x35
BASE58_ALPHABET = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

# Published genesis expectation (docs/DETERMINISTIC-SERIALIZATION-SPEC.md §9.6
# and the GENESIS-0 report).  Used for an *additional* named check only --
# it is never used as a shortcut for validation.
PUBLISHED_GENESIS_HASH = (
    "0000aca1af720da390380a38dc97d141cfb67afe578a2d31315860a5489db58c"
)
PUBLISHED_GENESIS_MERKLE = (
    "41a8b12643a3d8ce799941e514b8d82832336915554f5ecccad0a8d4150fbf3e"
)
PUBLISHED_GENESIS_NONCE = 230970


class ChainReject(Exception):
    """Raised when the chain does not satisfy a consensus rule."""


# --------------------------------------------------------------------------
# Primitives
# --------------------------------------------------------------------------

def sha256d(b: bytes) -> bytes:
    return hashlib.sha256(hashlib.sha256(b).digest()).digest()


def sha256(b: bytes) -> bytes:
    return hashlib.sha256(b).digest()


class Reader:
    """Bounds-checked big-endian-free little-endian reader."""

    def __init__(self, data: bytes):
        self.d = data
        self.i = 0

    def _need(self, n: int) -> None:
        if self.i + n > len(self.d):
            raise ChainReject("数据不完整: unexpected EOF")

    def u32(self) -> int:
        self._need(4)
        (v,) = struct.unpack_from("<I", self.d, self.i)
        self.i += 4
        return v

    def u64(self) -> int:
        self._need(8)
        (v,) = struct.unpack_from("<Q", self.d, self.i)
        self.i += 8
        return v

    def i64(self) -> int:
        self._need(8)
        (v,) = struct.unpack_from("<q", self.d, self.i)
        self.i += 8
        return v

    def take(self, n: int) -> bytes:
        self._need(n)
        out = self.d[self.i:self.i + n]
        self.i += n
        return out

    def remaining(self) -> int:
        return len(self.d) - self.i


# --------------------------------------------------------------------------
# P-256 (NIST prime256v1) -- pure Python ECDSA verification (§9.11)
# --------------------------------------------------------------------------

P256_P = 0xffffffff00000001000000000000000000000000ffffffffffffffffffffffff
P256_A = P256_P - 3
P256_B = 0x5ac635d8aa3a93e7b3ebbd55769886bc651d06b0cc53b0f63bce3c3e27d2604b
P256_GX = 0x6b17d1f2e12c4247f8bce6e563a440f277037d812deb33a0f4a13945d898c296
P256_GY = 0x4fe342e2fe1a7f9b8ee7eb4a7c0f9e162bce33576b315ececbb6406837bf51f5
P256_N = 0xffffffff00000000ffffffffffffffffbce6faada7179e84f3b9cac2fc632551


def _inv_mod(x: int, m: int) -> int:
    return pow(x, -1, m)  # Python 3.8+


def _point_add(p1, p2):
    if p1 is None:
        return p2
    if p2 is None:
        return p1
    x1, y1 = p1
    x2, y2 = p2
    if x1 == x2:
        if (y1 + y2) % P256_P == 0:
            return None
        return _point_double(p1)
    lam = ((y2 - y1) * _inv_mod(x2 - x1, P256_P)) % P256_P
    x3 = (lam * lam - x1 - x2) % P256_P
    y3 = (lam * (x1 - x3) - y1) % P256_P
    return (x3, y3)


def _point_double(p):
    if p is None:
        return None
    x, y = p
    if y % P256_P == 0:
        return None
    lam = ((3 * x * x + P256_A) * _inv_mod(2 * y, P256_P)) % P256_P
    x3 = (lam * lam - 2 * x) % P256_P
    y3 = (lam * (x - x3) - y) % P256_P
    return (x3, y3)


def _scalar_mul(k: int, p):
    if k % P256_N == 0 or p is None:
        return None
    if k < 0:
        return _scalar_mul(-k, (p[0], (-p[1]) % P256_P))
    result = None
    addend = p
    while k:
        if k & 1:
            result = _point_add(result, addend)
        addend = _point_double(addend)
        k >>= 1
    return result


def _is_on_curve(p) -> bool:
    if p is None:
        return False
    x, y = p
    if not (0 <= x < P256_P and 0 <= y < P256_P):
        return False
    return (y * y - (x * x * x + P256_A * x + P256_B)) % P256_P == 0


def ecdsa_p256_verify(pubkey_bytes: bytes, msg_hash: bytes, sig: bytes) -> bool:
    """Verify a 64-byte fixed-width r||s P-256 signature (§9.11)."""
    if len(pubkey_bytes) != 65 or pubkey_bytes[0] != 0x04:
        return False
    x = int.from_bytes(pubkey_bytes[1:33], "big")
    y = int.from_bytes(pubkey_bytes[33:65], "big")
    q = (x, y)
    if not _is_on_curve(q):
        return False

    if len(sig) != SIGNATURE_SIZE:
        return False
    r = int.from_bytes(sig[:32], "big")
    s = int.from_bytes(sig[32:], "big")
    if r == 0 or s == 0:
        return False
    if r >= P256_N or s >= P256_N:
        return False

    # hashToInt: order bit length == 256 == len(msg_hash)*8, so no truncation.
    z = int.from_bytes(msg_hash, "big")

    w = _inv_mod(s, P256_N)
    u1 = (z * w) % P256_N
    u2 = (r * w) % P256_N
    point = _point_add(_scalar_mul(u1, (P256_GX, P256_GY)), _scalar_mul(u2, q))
    if point is None:
        return False
    return point[0] % P256_N == r


# --------------------------------------------------------------------------
# Base58 / Base58Check (§9.12)
# --------------------------------------------------------------------------

def base58_encode(raw: bytes) -> str:
    n = int.from_bytes(raw, "big")
    out = []
    while n > 0:
        n, rem = divmod(n, 58)
        out.append(BASE58_ALPHABET[rem])
    for b in raw:                      # leading 0x00 -> leading '1'
        if b != 0x00:
            break
        out.append(BASE58_ALPHABET[0])
    return "".join(reversed(out))


def base58_decode(s: str) -> bytes:
    n = 0
    for c in s:
        idx = BASE58_ALPHABET.find(c)
        if idx < 0:
            raise ChainReject("Base58 含非法字符")
        n = n * 58 + idx
    body = n.to_bytes((n.bit_length() + 7) // 8, "big") if n else b""
    zeros = 0
    for c in s:
        if c == "1":
            zeros += 1
        else:
            break
    return b"\x00" * zeros + body


def base58check_decode(addr: str) -> tuple[int, bytes]:
    raw = base58_decode(addr)
    if len(raw) < 6:
        raise ChainReject("地址长度非法")
    version = raw[0]
    payload = raw[1:-4]
    if sha256d(raw[:-4])[:4] != raw[-4:]:
        raise ChainReject("地址校验和不匹配")
    return version, payload


def address_from_pubkey_hash(pkh: bytes) -> str:
    if len(pkh) != 20:
        raise ChainReject("PubKeyHash 必须为 20 字节")
    checked = bytes([ADDRESS_VERSION]) + pkh
    return base58_encode(checked + sha256d(checked)[:4])


# --------------------------------------------------------------------------
# Data model
# --------------------------------------------------------------------------

class TxInput:
    __slots__ = ("prev_tx_hash", "out_index", "signature", "pubkey")

    def __init__(self, prev_tx_hash, out_index, signature, pubkey):
        self.prev_tx_hash = prev_tx_hash
        self.out_index = out_index
        self.signature = signature
        self.pubkey = pubkey


class TxOutput:
    __slots__ = ("value", "pubkey_hash")

    def __init__(self, value, pubkey_hash):
        self.value = value
        self.pubkey_hash = pubkey_hash


class Transaction:
    __slots__ = ("inputs", "outputs", "_raw")

    def __init__(self, inputs, outputs, raw):
        self.inputs = inputs
        self.outputs = outputs
        self._raw = raw

    @property
    def size(self) -> int:
        return len(self._raw)

    def is_coinbase(self) -> bool:
        return (
            len(self.inputs) == 1
            and self.inputs[0].prev_tx_hash == b"\x00" * 32
            and self.inputs[0].out_index == COINBASE_OUT_INDEX
        )

    def serialize_for_hash(self) -> bytes:
        """§9.4.2 sighash serialization."""
        buf = bytearray()
        is_cb = self.is_coinbase()
        for inp in self.inputs:
            buf += inp.prev_tx_hash
            buf += struct.pack("<I", inp.out_index)
            if is_cb:
                buf += struct.pack("<I", len(inp.signature))
                buf += inp.signature
        for out in self.outputs:
            buf += struct.pack("<Q", out.value)
            buf += out.pubkey_hash
        return bytes(buf)

    def txid(self) -> bytes:
        """§9.3 -- single SHA-256."""
        return sha256(self.serialize_for_hash())

    def txid_hex(self) -> str:
        return self.txid().hex()


class Header:
    __slots__ = ("version", "prev_hash", "merkle_root", "timestamp", "bits", "nonce")

    def __init__(self, version, prev_hash, merkle_root, timestamp, bits, nonce):
        self.version = version
        self.prev_hash = prev_hash
        self.merkle_root = merkle_root
        self.timestamp = timestamp
        self.bits = bits
        self.nonce = nonce

    def serialize(self) -> bytes:
        """§9.2 -- 88 bytes, all little-endian."""
        return (
            struct.pack("<I", self.version)
            + self.prev_hash
            + self.merkle_root
            + struct.pack("<q", self.timestamp)
            + struct.pack("<I", self.bits)
            + struct.pack("<Q", self.nonce)
        )

    def hash(self) -> bytes:
        return sha256d(self.serialize())

    def hash_hex(self) -> str:
        return self.hash().hex()


class Block:
    __slots__ = ("header", "transactions", "_raw")

    def __init__(self, header, transactions, raw):
        self.header = header
        self.transactions = transactions
        self._raw = raw

    @property
    def size(self) -> int:
        return len(self._raw)


# --------------------------------------------------------------------------
# Decoding (§9.8 / §9.9)
# --------------------------------------------------------------------------

def decode_transaction(r: Reader) -> Transaction:
    n_in = r.u32()
    if n_in > MAX_IN_PER_TX:
        raise ChainReject("交易输入数超限")
    if n_in == 0:
        raise ChainReject("交易输入数为 0")
    inputs = []
    for _ in range(n_in):
        prev = r.take(32)
        idx = r.u32()
        sig_len = r.u32()
        if sig_len > MAX_SCRIPT_LEN:
            raise ChainReject("签名字段超限")
        sig = r.take(sig_len)
        pk_len = r.u32()
        if pk_len > MAX_SCRIPT_LEN:
            raise ChainReject("公钥字段超限")
        pk = r.take(pk_len)
        inputs.append(TxInput(prev, idx, sig, pk))

    n_out = r.u32()
    if n_out > MAX_OUT_PER_TX:
        raise ChainReject("交易输出数超限")
    if n_out == 0:
        raise ChainReject("交易输出数为 0")
    outputs = []
    for _ in range(n_out):
        value = r.u64()
        pkh = r.take(20)
        outputs.append(TxOutput(value, pkh))

    return Transaction(inputs, outputs, b"")


def decode_block(data: bytes) -> Block:
    start = 0
    r = Reader(data)
    version = r.u32()
    prev_hash = r.take(32)
    merkle_root = r.take(32)
    timestamp = r.i64()
    bits = r.u32()
    nonce = r.u64()
    header = Header(version, prev_hash, merkle_root, timestamp, bits, nonce)

    n_tx = r.u32()
    if n_tx > MAX_TX_PER_BLOCK:
        raise ChainReject("区块交易数超限")
    if n_tx == 0:
        raise ChainReject("区块交易数为 0")
    txs = []
    for i in range(n_tx):
        tx_start = r.i
        tx = decode_transaction(r)
        tx._raw = data[tx_start:r.i]
        txs.append(tx)
    if r.remaining() != 0:
        raise ChainReject("区块数据尾部有 %d 字节多余内容" % r.remaining())
    assert start == 0
    return Block(header, txs, data)


def parse_blocks_dat(path: str) -> list[bytes]:
    """§9.9 -- u32 LE length prefix + block bytes, no magic, no checksum."""
    with open(path, "rb") as f:
        blob = f.read()
    records = []
    r = Reader(blob)
    while r.remaining() > 0:
        length = r.u32()
        if length == 0 or length > MAX_RECORD_SIZE:
            raise ChainReject("非法记录长度 %d" % length)
        records.append(r.take(length))
    return records


# --------------------------------------------------------------------------
# Consensus helpers
# --------------------------------------------------------------------------

def compute_merkle_root(txs: list[Transaction]) -> bytes:
    """§9.5 -- single SHA-256, odd node duplicated."""
    if not txs:
        return sha256(b"")
    layer = [tx.txid() for tx in txs]
    while len(layer) > 1:
        if len(layer) % 2 == 1:
            layer.append(layer[-1])
        layer = [sha256(layer[i] + layer[i + 1]) for i in range(0, len(layer), 2)]
    return layer[0]


def subsidy(height: int) -> int:
    halvings = height // SUBSIDY_HALVING_INTERVAL
    if halvings >= 64:
        return 0
    return SUBSIDY_INITIAL >> halvings


def adjust_bits(current_bits: int, actual_timespan: int) -> int:
    """§9.7 -- clamp / rescale / convert back, clamp [1, 16]."""
    expected = TARGET_BLOCK_TIME_SECONDS * DIFFICULTY_ADJUSTMENT_INTERVAL
    min_span, max_span = expected // 4, expected * 4
    actual_timespan = max(min_span, min(max_span, actual_timespan))
    new_target = ((1 << (256 - current_bits)) * actual_timespan) // expected
    if new_target > (1 << (256 - MAX_TARGET_BITS)):
        return MAX_TARGET_BITS
    new_bits = 257 - new_target.bit_length()
    if new_bits < 1:
        new_bits = 1
    if new_bits > MAX_DIFFICULTY_BITS:
        new_bits = MAX_DIFFICULTY_BITS
    return new_bits


def expected_bits(blocks: list[Block], new_height: int) -> int:
    """Bits required for the block about to be appended at `new_height`."""
    tip_height = new_height - 1
    if tip_height < 0:
        return MAX_TARGET_BITS
    tip = blocks[tip_height].header
    if tip_height == 0 or tip_height % DIFFICULTY_ADJUSTMENT_INTERVAL != 0:
        return tip.bits
    start = max(0, tip_height - DIFFICULTY_ADJUSTMENT_INTERVAL)
    actual = tip.timestamp - blocks[start].header.timestamp
    return adjust_bits(tip.bits, actual)


def validate_pow(header: Header) -> bool:
    target = 1 << (256 - header.bits)
    return int.from_bytes(header.hash(), "big") < target


# --------------------------------------------------------------------------
# UTXO replay (§9.4.4 / §9.10 step 7)
# --------------------------------------------------------------------------

class UTXOSet:
    def __init__(self):
        self.entries: dict[tuple[bytes, int], tuple[int, bytes, int, bool]] = {}

    def clone(self) -> "UTXOSet":
        new = UTXOSet()
        new.entries = dict(self.entries)
        return new

    def add(self, key, value, pkh, height, is_coinbase) -> None:
        self.entries[key] = (value, pkh, height, is_coinbase)

    def get(self, key):
        return self.entries.get(key)

    def spend(self, key) -> None:
        self.entries.pop(key, None)


def validate_coinbase_structure(tx: Transaction, height: int) -> None:
    if not tx.is_coinbase():
        raise ChainReject("不是合法的 coinbase 形态")
    inp = tx.inputs[0]
    if len(inp.pubkey) != 0:
        raise ChainReject("coinbase 输入不得携带公钥")
    if len(inp.signature) != 4:
        raise ChainReject("coinbase 高度字段缺失或非法（须为 4 字节小端）")
    declared = struct.unpack("<I", inp.signature)[0]
    if declared != height:
        raise ChainReject(
            "coinbase 声明高度 %d 与所在区块高度 %d 不一致" % (declared, height)
        )
    if len(tx.outputs) == 0:
        raise ChainReject("coinbase 必须至少有一个输出")
    for out in tx.outputs:
        if out.value == 0:
            raise ChainReject("coinbase 输出金额为 0")


def validate_transaction(tx: Transaction, working: UTXOSet, height: int) -> int:
    """Returns the fee. Raises ChainReject on any consensus violation."""
    if tx.is_coinbase():
        validate_coinbase_structure(tx, height)
        for i, out in enumerate(tx.outputs):
            working.add((tx.txid(), i), out.value, out.pubkey_hash, height, True)
        return 0

    if len(tx.inputs) == 0:
        raise ChainReject("交易没有输入")
    if len(tx.outputs) == 0:
        raise ChainReject("交易没有输出")
    for out in tx.outputs:
        if out.value == 0:
            raise ChainReject("交易输出金额为 0")

    seen = set()
    spends = []
    in_total = 0
    txid = tx.txid()
    for inp in tx.inputs:
        key = (inp.prev_tx_hash, inp.out_index)
        if key in seen:
            raise ChainReject("交易包含重复输入")
        seen.add(key)

        entry = working.get(key)
        if entry is None:
            raise ChainReject(
                "输入引用的 UTXO 不存在或已被花费: %s:%d"
                % (inp.prev_tx_hash.hex()[:16], inp.out_index)
            )
        value, pkh, entry_height, is_coinbase = entry

        if is_coinbase and height - entry_height < COINBASE_MATURITY:
            raise ChainReject(
                "输出在高度 %d 产生，当前高度 %d，需 %d"
                % (entry_height, height, COINBASE_MATURITY)
            )

        if sha256(inp.pubkey)[:20] != pkh:
            raise ChainReject("输入公钥与该 UTXO 锁定的公钥哈希不匹配")

        if not ecdsa_p256_verify(inp.pubkey, txid, inp.signature):
            raise ChainReject("输入签名验证失败")

        in_total += value
        spends.append(key)

    out_total = sum(o.value for o in tx.outputs)
    if in_total < out_total:
        raise ChainReject("输入总额小于输出总额（超额支出）")

    for key in spends:
        working.spend(key)
    for i, out in enumerate(tx.outputs):
        working.add((txid, i), out.value, out.pubkey_hash, height, False)
    return in_total - out_total


def apply_block(base: UTXOSet, txs: list[Transaction], height: int) -> tuple[UTXOSet, int]:
    if len(txs) == 0:
        raise ChainReject("区块没有交易")
    if not txs[0].is_coinbase():
        raise ChainReject("首笔交易不是 coinbase")
    for tx in txs[1:]:
        if tx.is_coinbase():
            raise ChainReject("区块中出现多于一个 coinbase")

    working = base.clone()
    if validate_transaction(txs[0], working, height) != 0:
        raise ChainReject("coinbase 交易不应产生手续费")

    fees = 0
    for tx in txs[1:]:
        fees += validate_transaction(tx, working, height)

    coinbase_out = sum(o.value for o in txs[0].outputs)
    if coinbase_out > subsidy(height) + fees:
        raise ChainReject(
            "coinbase 输出 %d > 奖励 %d + 手续费 %d"
            % (coinbase_out, subsidy(height), fees)
        )
    return working, fees


# --------------------------------------------------------------------------
# Whole-chain verification
# --------------------------------------------------------------------------

def verify_chain(records: list[bytes], now: int) -> dict:
    blocks: list[Block] = []
    report = {
        "blocks_checked": 0,
        "height": -1,
        "genesis_hash": "",
        "tip_hash": "",
        "valid": False,
        "fail_height": -1,
        "fail_hash": "",
        "reason": "",
        "genesis_matches_published": False,
        "per_block": [],
    }

    def reject(height: int, block_hash: str, reason: str) -> dict:
        report["fail_height"] = height
        report["fail_hash"] = block_hash
        report["reason"] = reason
        report["valid"] = False
        return report

    if not records:
        report["reason"] = "本地链为空（数据目录中没有区块可校验）"
        return report

    # ---- genesis: applied without header validation (trusted input) ----
    try:
        genesis = decode_block(records[0])
    except ChainReject as exc:
        return reject(0, "", "区块解码失败: %s" % exc)
    try:
        utxo_set, _ = apply_block(UTXOSet(), genesis.transactions, 0)
    except ChainReject as exc:
        return reject(0, genesis.header.hash_hex(), "创世区块 UTXO 初始化失败: %s" % exc)
    blocks.append(genesis)
    report["genesis_hash"] = genesis.header.hash_hex()
    report["blocks_checked"] = 1
    report["genesis_matches_published"] = (
        report["genesis_hash"] == PUBLISHED_GENESIS_HASH
    )
    report["per_block"].append(
        {
            "height": 0,
            "hash": genesis.header.hash_hex(),
            "merkle": genesis.header.merkle_root.hex(),
            "bits": genesis.header.bits,
            "nonce": genesis.header.nonce,
            "txs": len(genesis.transactions),
            "utxo": "ok",
            "result": "ok",
        }
    )

    # ---- heights 1..N ----
    for height in range(1, len(records)):
        try:
            blk = decode_block(records[height])
        except ChainReject as exc:
            return reject(height, "", "区块解码失败: %s" % exc)
        h = blk.header
        h_hex = h.hash_hex()
        entry = {
            "height": height,
            "hash": h_hex,
            "merkle": h.merkle_root.hex(),
            "bits": h.bits,
            "nonce": h.nonce,
            "txs": len(blk.transactions),
            "utxo": "-",
            "result": "-",
        }
        report["per_block"].append(entry)

        # 1. PrevHash
        if h.prev_hash != blocks[height - 1].header.hash():
            entry["result"] = "prev_hash"
            return reject(height, h_hex, "区块的前置哈希与当前链尾不匹配")
        # 2. PoW
        if not validate_pow(h):
            entry["result"] = "pow"
            return reject(height, h_hex, "工作量证明无效")
        # 3. Bits
        want_bits = expected_bits(blocks, height)
        if h.bits != want_bits:
            entry["result"] = "bits"
            return reject(
                height, h_hex, "难度位与共识难度不符: 区块 %d，共识 %d" % (h.bits, want_bits)
            )
        # 4. Timestamp
        parent_ts = blocks[height - 1].header.timestamp
        if h.timestamp < parent_ts:
            entry["result"] = "timestamp"
            return reject(
                height, h_hex, "时间戳 %d 早于父块 %d" % (h.timestamp, parent_ts)
            )
        if h.timestamp > now + MAX_FUTURE_TIMESTAMP_DRIFT:
            entry["result"] = "timestamp"
            return reject(
                height,
                h_hex,
                "时间戳 %d 超前本地时钟超过 %d 秒" % (h.timestamp, MAX_FUTURE_TIMESTAMP_DRIFT),
            )
        # 5. Merkle
        if compute_merkle_root(blk.transactions) != h.merkle_root:
            entry["result"] = "merkle"
            return reject(height, h_hex, "Merkle 根不匹配")
        # 6. Size
        if blk.size > BLOCK_SIZE_LIMIT:
            entry["result"] = "size"
            return reject(height, h_hex, "区块体积 %d > %d" % (blk.size, BLOCK_SIZE_LIMIT))
        # 7. Transaction + UTXO transition
        try:
            utxo_set, _ = apply_block(utxo_set, blk.transactions, height)
        except ChainReject as exc:
            entry["utxo"] = "reject"
            entry["result"] = "tx"
            return reject(height, h_hex, "添加区块失败: %s" % exc)

        entry["utxo"] = "ok"
        entry["result"] = "ok"
        blocks.append(blk)
        report["blocks_checked"] += 1

    report["height"] = len(blocks) - 1
    report["tip_hash"] = blocks[-1].header.hash_hex()
    report["valid"] = True
    return report


# --------------------------------------------------------------------------
# CLI
# --------------------------------------------------------------------------

def resolve_datadir(target: str) -> str:
    if os.path.isdir(target):
        candidate = os.path.join(target, "blocks.dat")
        if not os.path.isfile(candidate):
            raise SystemExit("错误: 区块数据文件不存在（%s）" % candidate)
        return candidate
    if os.path.isfile(target):
        return target
    raise SystemExit("错误: 路径不存在: %s" % target)


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("target", nargs="?")
    parser.add_argument("--json", action="store_true")
    parser.add_argument("--now", type=int, default=None)
    parser.add_argument("--per-block", action="store_true")
    parser.add_argument("-h", "--help", action="store_true")
    try:
        args = parser.parse_args(argv)
    except SystemExit:
        return 2
    if args.help or not args.target:
        print(__doc__)
        return 2

    path = resolve_datadir(args.target)
    now = args.now if args.now is not None else int(time.time())
    try:
        records = parse_blocks_dat(path)
    except (ChainReject, OSError) as exc:
        print("错误: 打开区块数据失败: %s" % exc, file=sys.stderr)
        return 1

    report = verify_chain(records, now)
    report["data_dir"] = path
    if not args.per_block:
        report.pop("per_block", None)

    if args.json:
        print(json.dumps(report, ensure_ascii=False, indent=2))
        return 0 if report["valid"] else 1

    print("[verify-py] 数据文件  : %s" % report["data_dir"])
    if report["valid"]:
        print(
            "[verify-py] 已校验区块: %d 个（高度 0 → %d）"
            % (report["blocks_checked"], report["height"])
        )
        print("[verify-py] 创世哈希  : %s" % report["genesis_hash"])
        print("[verify-py] 链尾哈希  : %s" % report["tip_hash"])
        print(
            "[verify-py] 创世匹配  : %s"
            % ("YES" if report["genesis_matches_published"] else "NO")
        )
        print("[verify-py] 结果      : PASS（独立 Python 实现校验通过）")
        return 0
    print("[verify-py] 结果      : FAIL")
    if report["fail_height"] >= 0:
        print("[verify-py] 失败高度  : %d" % report["fail_height"])
    if report["fail_hash"]:
        print("[verify-py] 失败区块  : %s" % report["fail_hash"])
    print("[verify-py] 原因      : %s" % report["reason"])
    return 1


if __name__ == "__main__":
    sys.exit(main())
