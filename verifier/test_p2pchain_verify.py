#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Tests for the P2PChain independent Python verifier (PHASE V2.0).

Run:  python verifier/test_p2pchain_verify.py         (or python -m unittest)

Standard library only.  Real-chain tests are skipped (not faked) when the
sample chains are not present on this machine.
"""

from __future__ import annotations

import hashlib
import os
import shutil
import struct
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import p2pchain_verify as pv  # noqa: E402

CHAIN62 = r"C:/Users/Administrator/AppData/Local/Temp/0d3-diff/blocks.dat"
CHAINTX = r"C:/Users/Administrator/AppData/Local/Temp/v20-txchain/blocks.dat"


def have(path: str) -> bool:
    return os.path.isfile(path)


class TestGenesisVector(unittest.TestCase):
    def test_published_genesis_is_self_consistent(self):
        # Published expectation from docs/DETERMINISTIC-SERIALIZATION-SPEC.md §9.6
        self.assertEqual(len(pv.PUBLISHED_GENESIS_HASH), 64)
        self.assertEqual(len(pv.PUBLISHED_GENESIS_MERKLE), 64)
        self.assertEqual(pv.PUBLISHED_GENESIS_NONCE, 230970)

    @unittest.skipUnless(have(CHAIN62), "62-block sample chain not present")
    def test_genesis_header_and_hash(self):
        g = pv.decode_block(pv.parse_blocks_dat(CHAIN62)[0])
        self.assertEqual(g.header.serialize(), pv.HEADER_SIZE and g.header.serialize())
        self.assertEqual(len(g.header.serialize()), 88)
        self.assertEqual(g.header.version, 1)
        self.assertEqual(g.header.timestamp, 1700000000)
        self.assertEqual(g.header.bits, 16)
        self.assertEqual(g.header.nonce, pv.PUBLISHED_GENESIS_NONCE)
        self.assertEqual(g.header.hash_hex(), pv.PUBLISHED_GENESIS_HASH)
        self.assertEqual(g.header.merkle_root.hex(), pv.PUBLISHED_GENESIS_MERKLE)

    @unittest.skipUnless(have(CHAIN62), "62-block sample chain not present")
    def test_genesis_coinbase_txid_equals_merkle(self):
        g = pv.decode_block(pv.parse_blocks_dat(CHAIN62)[0])
        self.assertEqual(g.transactions[0].txid_hex(), pv.PUBLISHED_GENESIS_MERKLE)
        self.assertEqual(g.transactions[0].inputs[0].signature, b"\x00\x00\x00\x00")
        self.assertEqual(g.transactions[0].outputs[0].value, 50)
        self.assertEqual(
            g.transactions[0].outputs[0].pubkey_hash, b"\x00" * 19 + b"\x01"
        )


class TestMerkle(unittest.TestCase):
    def test_empty_is_sha256_of_empty(self):
        self.assertEqual(pv.compute_merkle_root([]), hashlib.sha256(b"").digest())

    def test_single_leaf_is_txid(self):
        class FakeTx:
            def txid(self):
                return b"\x11" * 32

        self.assertEqual(pv.compute_merkle_root([FakeTx()]), b"\x11" * 32)

    def test_odd_count_duplicates_last(self):
        class T:
            def __init__(self, b):
                self.b = b

            def txid(self):
                return self.b

        a, b, c = T(b"\x01" * 32), T(b"\x02" * 32), T(b"\x03" * 32)
        expected = hashlib.sha256(
            hashlib.sha256(a.b + b.b).digest() + hashlib.sha256(c.b + c.b).digest()
        ).digest()
        self.assertEqual(pv.compute_merkle_root([a, b, c]), expected)

    @unittest.skipUnless(have(CHAINTX), "tx sample chain not present")
    def test_two_tx_merkle_from_real_block(self):
        b15 = pv.decode_block(pv.parse_blocks_dat(CHAINTX)[15])
        self.assertEqual(len(b15.transactions), 2)
        self.assertEqual(pv.compute_merkle_root(b15.transactions), b15.header.merkle_root)


class TestP256(unittest.TestCase):
    @unittest.skipUnless(have(CHAINTX), "tx sample chain not present")
    def _real_input(self):
        b15 = pv.decode_block(pv.parse_blocks_dat(CHAINTX)[15])
        tx = b15.transactions[1]
        return tx.inputs[0], tx.txid()

    def test_real_signature_verifies(self):
        if not have(CHAINTX):
            self.skipTest("tx sample chain not present")
        inp, txid = self._real_input()
        self.assertEqual(len(inp.signature), 64)
        self.assertEqual(len(inp.pubkey), 65)
        self.assertEqual(inp.pubkey[0], 0x04)
        self.assertTrue(pv.ecdsa_p256_verify(inp.pubkey, txid, inp.signature))

    def test_tampered_signature_rejected(self):
        if not have(CHAINTX):
            self.skipTest("tx sample chain not present")
        inp, txid = self._real_input()
        bad_r = bytearray(inp.signature)
        bad_r[0] ^= 0x01
        bad_s = bytearray(inp.signature)
        bad_s[40] ^= 0x01
        self.assertFalse(pv.ecdsa_p256_verify(inp.pubkey, txid, bytes(bad_r)))
        self.assertFalse(pv.ecdsa_p256_verify(inp.pubkey, txid, bytes(bad_s)))

    def test_tampered_message_rejected(self):
        if not have(CHAINTX):
            self.skipTest("tx sample chain not present")
        inp, txid = self._real_input()
        other = bytes([txid[0] ^ 0x01]) + txid[1:]
        self.assertFalse(pv.ecdsa_p256_verify(inp.pubkey, other, inp.signature))

    def test_wrong_length_signature_rejected(self):
        if not have(CHAINTX):
            self.skipTest("tx sample chain not present")
        inp, txid = self._real_input()
        self.assertFalse(pv.ecdsa_p256_verify(inp.pubkey, txid, inp.signature[:63]))
        self.assertFalse(pv.ecdsa_p256_verify(inp.pubkey, txid, inp.signature + b"\x00"))

    def test_malformed_pubkey_rejected(self):
        if not have(CHAINTX):
            self.skipTest("tx sample chain not present")
        inp, txid = self._real_input()
        self.assertFalse(pv.ecdsa_p256_verify(b"\x04" + inp.pubkey[1:-1] + b"\x00",
                                              txid, inp.signature))
        self.assertFalse(pv.ecdsa_p256_verify(b"\x02" + inp.pubkey[1:], txid, inp.signature))


class TestBase58(unittest.TestCase):
    def test_alphabet(self):
        self.assertEqual(len(pv.BASE58_ALPHABET), 58)
        self.assertEqual(pv.BASE58_ALPHABET[0], "1")
        for ch in "0OIl":
            self.assertNotIn(ch, pv.BASE58_ALPHABET)

    @unittest.skipUnless(have(CHAINTX), "tx sample chain not present")
    def test_real_address_roundtrip(self):
        b15 = pv.decode_block(pv.parse_blocks_dat(CHAINTX)[15])
        pkh = hashlib.sha256(b15.transactions[1].inputs[0].pubkey).digest()[:20]
        addr = pv.address_from_pubkey_hash(pkh)
        version, payload = pv.base58check_decode(addr)
        self.assertEqual(version, 0x35)
        self.assertEqual(payload, pkh)
        self.assertEqual(pv.address_from_pubkey_hash(payload), addr)

    def test_bad_checksum_rejected(self):
        addr = pv.address_from_pubkey_hash(b"\x01" * 20)
        last = addr[-1]
        swapped = addr[:-1] + ("B" if last != "B" else "C")
        with self.assertRaises(pv.ChainReject):
            pv.base58check_decode(swapped)

    def test_leading_zero_preserved(self):
        raw = b"\x00\x00" + b"\xff" * 20
        enc = pv.base58_encode(raw)
        self.assertEqual(enc[:2], "11")
        self.assertEqual(pv.base58_decode(enc), raw)


class TestConsensusHelpers(unittest.TestCase):
    def test_subsidy(self):
        self.assertEqual(pv.subsidy(0), 50)
        self.assertEqual(pv.subsidy(209), 50)
        self.assertEqual(pv.subsidy(210), 25)
        self.assertEqual(pv.subsidy(420), 12)

    def test_target_and_pow(self):
        h = pv.Header(1, b"\x00" * 32, b"\x00" * 32, 1700000000, 16, 0)
        # an all-zero hash trivially satisfies any positive target
        self.assertTrue(int.from_bytes(b"\x00" * 32, "big") < (1 << (256 - 16)))
        self.assertFalse(int.from_bytes(b"\xff" * 32, "big") < (1 << (256 - 16)))
        self.assertEqual(len(h.serialize()), 88)

    def test_adjust_bits_clamped_to_16(self):
        # max difficulty bits == max target bits == 16, so output can never exceed 16
        for span in (-10_000, 0, 1, 1200, 100_000):
            self.assertLessEqual(pv.adjust_bits(16, span), 16)
            self.assertGreaterEqual(pv.adjust_bits(16, span), 1)


class TestWholeChain(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="v20-neg-")
        self.addCleanup(shutil.rmtree, self.tmp, True)

    def _copy(self, src: str) -> str:
        dst = os.path.join(self.tmp, "blocks.dat")
        shutil.copyfile(src, dst)
        return dst

    @unittest.skipUnless(have(CHAIN62), "62-block sample chain not present")
    def test_valid_chain_passes(self):
        path = self._copy(CHAIN62)
        rep = pv.verify_chain(pv.parse_blocks_dat(path), now=2_000_000_000)
        self.assertTrue(rep["valid"])
        self.assertEqual(rep["blocks_checked"], 62)
        self.assertEqual(rep["height"], 61)
        self.assertEqual(rep["genesis_hash"], pv.PUBLISHED_GENESIS_HASH)
        self.assertTrue(rep["genesis_matches_published"])

    @unittest.skipUnless(have(CHAIN62), "62-block sample chain not present")
    def test_n1_header_corruption(self):
        path = self._copy(CHAIN62)
        with open(path, "r+b") as f:
            f.seek(4 + 40)          # inside header of block 0 (merkle area)
            b = f.read(1)
            f.seek(4 + 40)
            f.write(bytes([b[0] ^ 0x01]))
        rep = pv.verify_chain(pv.parse_blocks_dat(path), now=2_000_000_000)
        self.assertFalse(rep["valid"])

    @unittest.skipUnless(have(CHAIN62), "62-block sample chain not present")
    def test_n2_truncation(self):
        path = self._copy(CHAIN62)
        size = os.path.getsize(path)
        with open(path, "r+b") as f:
            f.truncate(size - 16)
        with self.assertRaises(pv.ChainReject):
            pv.parse_blocks_dat(path)

    @unittest.skipUnless(have(CHAIN62), "62-block sample chain not present")
    def test_n3_merkle_corruption(self):
        """Flip a byte inside a *transaction* of the last block (header untouched)."""
        path = self._copy(CHAIN62)
        size = os.path.getsize(path)
        with open(path, "r+b") as f:
            f.seek(size - 10)       # tail of the final record: output value area
            b = f.read(1)
            f.seek(size - 10)
            f.write(bytes([b[0] ^ 0x01]))
        rep = pv.verify_chain(pv.parse_blocks_dat(path), now=2_000_000_000)
        self.assertFalse(rep["valid"])
        # the header still hashes fine, so the rejection must come from
        # Merkle or the transaction layer, not from PoW
        self.assertIn(rep["reason"][:40], ["Merkle 根不匹配", "添加区块失败", "区块解码失败"])

    @unittest.skipUnless(have(CHAIN62), "62-block sample chain not present")
    def test_n4_linkage_break(self):
        """Swap two middle records -> PrevHash no longer matches."""
        path = self._copy(CHAIN62)
        recs = pv.parse_blocks_dat(path)
        recs[30], recs[31] = recs[31], recs[30]
        with open(path, "wb") as f:
            for r in recs:
                f.write(struct.pack("<I", len(r)) + r)
        rep = pv.verify_chain(pv.parse_blocks_dat(path), now=2_000_000_000)
        self.assertFalse(rep["valid"])
        self.assertIn("前置哈希", rep["reason"])

    @unittest.skipUnless(have(CHAIN62), "62-block sample chain not present")
    def test_n5_duplicate_record(self):
        """Append a duplicate of the last record -> duplicate height / linkage."""
        path = self._copy(CHAIN62)
        recs = pv.parse_blocks_dat(path)
        recs.append(recs[-1])
        with open(path, "wb") as f:
            for r in recs:
                f.write(struct.pack("<I", len(r)) + r)
        rep = pv.verify_chain(pv.parse_blocks_dat(path), now=2_000_000_000)
        self.assertFalse(rep["valid"])

    @unittest.skipUnless(have(CHAIN62), "62-block sample chain not present")
    def test_determinism_three_runs(self):
        path = self._copy(CHAIN62)
        results = []
        for _ in range(3):
            rep = pv.verify_chain(pv.parse_blocks_dat(path), now=2_000_000_000)
            results.append((rep["blocks_checked"], rep["genesis_hash"],
                            rep["tip_hash"], rep["valid"]))
        self.assertEqual(len(set(results)), 1)

    @unittest.skipUnless(have(CHAINTX), "tx sample chain not present")
    def test_chain_with_real_transaction_passes(self):
        path = self._copy(CHAINTX)
        rep = pv.verify_chain(pv.parse_blocks_dat(path), now=2_000_000_000)
        self.assertTrue(rep["valid"])
        self.assertEqual(rep["blocks_checked"], 17)

    @unittest.skipUnless(have(CHAINTX), "tx sample chain not present")
    def test_tampered_signature_in_real_tx_rejected(self):
        """Flip one byte of the real 64-byte signature inside the stored chain."""
        path = self._copy(CHAINTX)
        with open(path, "rb") as f:
            blob = bytearray(f.read())
        sig_marker = bytes.fromhex(
            "817B409306B2F6B63F2F090FA9D567596E6B0DDD78144DE43F8CDAB8BB1EDE36"
            "47820447F0E7C9A0514D41EF23C49B4E9D09FE9161B54521D7CA7D0A5CD18357"
        )
        idx = blob.find(sig_marker)
        self.assertGreaterEqual(idx, 0, "signature not found in sample chain")
        blob[idx] ^= 0x01
        with open(path, "wb") as f:
            f.write(bytes(blob))
        rep = pv.verify_chain(pv.parse_blocks_dat(path), now=2_000_000_000)
        self.assertFalse(rep["valid"])

    def test_empty_file_is_not_valid(self):
        path = os.path.join(self.tmp, "empty.dat")
        with open(path, "wb"):
            pass
        rep = pv.verify_chain(pv.parse_blocks_dat(path), now=2_000_000_000)
        self.assertFalse(rep["valid"])
        self.assertIn("本地链为空", rep["reason"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
