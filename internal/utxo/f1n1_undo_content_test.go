package utxo

// ════════════════════════════════════════════════════════════════════════════
// PHASE P2PCHAIN — F1-N1
// UNDO CONTRACT TEST DESIGN / LEGACY-V2 REORG COVERAGE AUDIT
//
//   Test F — Undo Content Correctness
//
// SCOPE DISCIPLINE
//   * test-only file. No production `.go` is modified and NO production
//     consumer of undo is wired up (phase spec §7 / §11).
//   * the inverse property is checked by TWO independent oracles:
//       oracle 1 — the production reverse primitive DisconnectBlock
//       oracle 2 — a hand-written inverse (delete Created / restore Spent)
//     Agreement between them is non-trivial evidence that the undo really
//     carries the inverse information, rather than merely "existing".
//   * per-item content is checked too: every Created outpoint must be absent
//     from the pre-state and present in the post-state, and every Spent
//     outpoint must carry the exact pre-state Entry.
//   * determinism + encoding round-trip are checked on identical inputs.
//
// COVERAGE: UTXO creation, UTXO spend, multiple inputs, multiple outputs,
//           coinbase, fee-bearing transaction, mixed transaction block.
// ════════════════════════════════════════════════════════════════════════════

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"testing"

	"p2pchain/internal/transaction"
)

// f1n1SetSig returns a deterministic digest of a UTXO set, so "state unchanged"
// can be asserted without depending on set internals.
func f1n1SetSig(s *UTXOSet) string {
	entries := s.AllEntries()
	lines := make([]string, 0, len(entries))
	for op, e := range entries {
		lines = append(lines, fmt.Sprintf("%s|%d|%x|%d|%v",
			op.String(), e.Value, e.PubKeyHash, e.Height, e.IsCoinbase))
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

// f1n1ManualInverse applies the BlockUndo contract by hand — delete every
// Created outpoint, then restore every Spent outpoint — WITHOUT calling
// DisconnectBlock. It is the independent oracle for Test F.
func f1n1ManualInverse(t *testing.T, post *UTXOSet, u BlockUndo) *UTXOSet {
	t.Helper()
	inv := post.Clone()
	for _, ce := range u.Created {
		if _, err := inv.Spend(ce.OutPoint); err != nil {
			t.Fatalf("F1N1: manual inverse: Created %s absent from the post-state: %v", ce.OutPoint, err)
		}
	}
	for _, se := range u.Spent {
		inv.Add(se.OutPoint, se.Entry)
	}
	return inv
}

func f1n1Encode(t *testing.T, u BlockUndo) []byte {
	t.Helper()
	b, err := EncodeUndo(u)
	if err != nil {
		t.Fatalf("F1N1: EncodeUndo: %v", err)
	}
	return b
}

func TestF1N1_F_UndoContentCorrectness(t *testing.T) {
	type f1n1Case struct {
		name        string
		height      int
		build       func(t *testing.T) (*UTXOSet, []*transaction.Transaction)
		wantFees    uint64
		wantCreated int
		wantSpent   int
	}

	cases := []f1n1Case{
		// ── UTXO creation only: a coinbase-only block ──
		{
			name:   "coinbase_only_creation",
			height: 1,
			build: func(t *testing.T) (*UTXOSet, []*transaction.Transaction) {
				alice := newTestWallet(t)
				pre := NewUTXOSet()
				cb := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1), 1)
				return pre, []*transaction.Transaction{cb}
			},
			wantFees: 0, wantCreated: 1, wantSpent: 0,
		},
		// ── single input → single output, fee-bearing ──
		{
			name:   "single_input_single_output_fee",
			height: 1,
			build: func(t *testing.T) (*UTXOSet, []*transaction.Transaction) {
				alice := newTestWallet(t)
				bob := newTestWallet(t)
				pre := NewUTXOSet()
				op := fund(t, pre, [32]byte{0xA1}, 0, alice, 100, 0)
				tx := &transaction.Transaction{
					Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
					Outputs: []transaction.TxOutput{{Value: 90, PubKeyHash: bob.PubKeyHash()}},
				}
				signTx(t, tx, alice)
				cb := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+10, 1)
				return pre, []*transaction.Transaction{cb, tx}
			},
			wantFees: 10, wantCreated: 2, wantSpent: 1,
		},
		// ── multiple inputs in one transaction ──
		{
			name:   "multiple_inputs",
			height: 1,
			build: func(t *testing.T) (*UTXOSet, []*transaction.Transaction) {
				alice := newTestWallet(t)
				bob := newTestWallet(t)
				pre := NewUTXOSet()
				op1 := fund(t, pre, [32]byte{0xB1}, 0, alice, 100, 0)
				op2 := fund(t, pre, [32]byte{0xB2}, 1, alice, 100, 0)
				tx := &transaction.Transaction{
					Inputs: []transaction.TxInput{
						{PrevTxHash: op1.Hash, OutIndex: op1.Index},
						{PrevTxHash: op2.Hash, OutIndex: op2.Index},
					},
					Outputs: []transaction.TxOutput{{Value: 150, PubKeyHash: bob.PubKeyHash()}},
				}
				signTx(t, tx, alice)
				cb := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+50, 1)
				return pre, []*transaction.Transaction{cb, tx}
			},
			wantFees: 50, wantCreated: 2, wantSpent: 2,
		},
		// ── multiple outputs (payment + third party) ──
		{
			name:   "multiple_outputs",
			height: 1,
			build: func(t *testing.T) (*UTXOSet, []*transaction.Transaction) {
				alice := newTestWallet(t)
				bob := newTestWallet(t)
				carol := newTestWallet(t)
				pre := NewUTXOSet()
				op := fund(t, pre, [32]byte{0xC1}, 0, alice, 100, 0)
				tx := &transaction.Transaction{
					Inputs: []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
					Outputs: []transaction.TxOutput{
						{Value: 40, PubKeyHash: bob.PubKeyHash()},
						{Value: 50, PubKeyHash: carol.PubKeyHash()},
					},
				}
				signTx(t, tx, alice)
				cb := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+10, 1)
				return pre, []*transaction.Transaction{cb, tx}
			},
			wantFees: 10, wantCreated: 3, wantSpent: 1,
		},
		// ── fee accumulation across two independent fee-bearing transactions ──
		{
			name:   "fee_bearing_two_txs",
			height: 1,
			build: func(t *testing.T) (*UTXOSet, []*transaction.Transaction) {
				alice := newTestWallet(t)
				bob := newTestWallet(t)
				pre := NewUTXOSet()
				op1 := fund(t, pre, [32]byte{0xD1}, 0, alice, 100, 0)
				op2 := fund(t, pre, [32]byte{0xD2}, 0, alice, 100, 0)
				tx1 := &transaction.Transaction{
					Inputs:  []transaction.TxInput{{PrevTxHash: op1.Hash, OutIndex: op1.Index}},
					Outputs: []transaction.TxOutput{{Value: 60, PubKeyHash: bob.PubKeyHash()}},
				}
				signTx(t, tx1, alice)
				tx2 := &transaction.Transaction{
					Inputs:  []transaction.TxInput{{PrevTxHash: op2.Hash, OutIndex: op2.Index}},
					Outputs: []transaction.TxOutput{{Value: 70, PubKeyHash: bob.PubKeyHash()}},
				}
				signTx(t, tx2, alice)
				cb := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+70, 1)
				return pre, []*transaction.Transaction{cb, tx1, tx2}
			},
			wantFees: 70, wantCreated: 3, wantSpent: 2,
		},
		// ── mixed transaction block: coinbase + multi-input/multi-output txs ──
		{
			name:   "mixed_transaction_block",
			height: 1,
			build: func(t *testing.T) (*UTXOSet, []*transaction.Transaction) {
				alice := newTestWallet(t)
				bob := newTestWallet(t)
				carol := newTestWallet(t)
				pre := NewUTXOSet()
				op1 := fund(t, pre, [32]byte{0xE1}, 0, alice, 100, 0)
				op2 := fund(t, pre, [32]byte{0xE2}, 1, alice, 80, 0)
				op3 := fund(t, pre, [32]byte{0xE3}, 0, bob, 60, 0)

				txA := &transaction.Transaction{
					Inputs: []transaction.TxInput{
						{PrevTxHash: op1.Hash, OutIndex: op1.Index},
						{PrevTxHash: op2.Hash, OutIndex: op2.Index},
					},
					Outputs: []transaction.TxOutput{
						{Value: 120, PubKeyHash: bob.PubKeyHash()},
						{Value: 50, PubKeyHash: alice.PubKeyHash()},
					},
				}
				signTx(t, txA, alice)

				txB := &transaction.Transaction{
					Inputs: []transaction.TxInput{{PrevTxHash: op3.Hash, OutIndex: op3.Index}},
					Outputs: []transaction.TxOutput{
						{Value: 30, PubKeyHash: carol.PubKeyHash()},
						{Value: 20, PubKeyHash: bob.PubKeyHash()},
					},
				}
				signTx(t, txB, bob)

				cb := transaction.NewCoinbaseTx(alice.PubKeyHash(), Subsidy(1)+20, 1)
				return pre, []*transaction.Transaction{cb, txA, txB}
			},
			wantFees: 20, wantCreated: 5, wantSpent: 3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pre, txs := tc.build(t)
			preSig := f1n1SetSig(pre)
			preLen := pre.Len()

			post, undo, fees, err := ApplyBlockWithUndo(pre, txs, tc.height)
			if err != nil {
				t.Fatalf("ApplyBlockWithUndo: %v", err)
			}

			// The base set must not be mutated (ApplyBlockWithUndo purity).
			if f1n1SetSig(pre) != preSig || pre.Len() != preLen {
				t.Fatalf("ApplyBlockWithUndo mutated its base set")
			}

			// ── structural expectations ──
			if fees != tc.wantFees {
				t.Fatalf("fees=%d, want %d", fees, tc.wantFees)
			}
			if undo.Height != tc.height {
				t.Fatalf("undo.Height=%d, want %d", undo.Height, tc.height)
			}
			if undo.PreSetItemsCount != preLen {
				t.Fatalf("undo.PreSetItemsCount=%d, want pre-state Len()=%d", undo.PreSetItemsCount, preLen)
			}
			if len(undo.Created) != tc.wantCreated {
				t.Fatalf("len(Created)=%d, want %d", len(undo.Created), tc.wantCreated)
			}
			if len(undo.Spent) != tc.wantSpent {
				t.Fatalf("len(Spent)=%d, want %d", len(undo.Spent), tc.wantSpent)
			}

			// ── per-item inverse content ──
			preEntries := pre.AllEntries()
			postEntries := post.AllEntries()

			for _, ce := range undo.Created {
				if _, inPre := preEntries[ce.OutPoint]; inPre {
					t.Fatalf("Created %s is present in the pre-state: not a creation", ce.OutPoint)
				}
				got, inPost := postEntries[ce.OutPoint]
				if !inPost {
					t.Fatalf("Created %s is absent from the post-state", ce.OutPoint)
				}
				if got != ce.Entry {
					t.Fatalf("Created %s Entry mismatch: undo=%+v post=%+v", ce.OutPoint, ce.Entry, got)
				}
			}
			for _, se := range undo.Spent {
				if got, inPost := postEntries[se.OutPoint]; inPost {
					t.Fatalf("Spent %s is still present in the post-state: %+v", se.OutPoint, got)
				}
				want, inPre := preEntries[se.OutPoint]
				if !inPre {
					t.Fatalf("Spent %s is absent from the pre-state", se.OutPoint)
				}
				if want != se.Entry {
					t.Fatalf("Spent %s does not carry the pre-state Entry: undo=%+v pre=%+v",
						se.OutPoint, se.Entry, want)
				}
			}

			// ── Oracle 1: the production reverse primitive ──
			postSnapshotSig := f1n1SetSig(post)
			restored, err := DisconnectBlock(post.Clone(), undo)
			if err != nil {
				t.Fatalf("oracle1: DisconnectBlock: %v", err)
			}
			if got := f1n1SetSig(restored); got != preSig {
				t.Fatalf("oracle1: DisconnectBlock did not restore the pre-state (%s != %s)", got, preSig)
			}
			assertSetEqual(t, "F1N1 oracle1 (DisconnectBlock)", restored, pre)

			// ── Oracle 2: an independent hand-written inverse ──
			manual := f1n1ManualInverse(t, post, undo)
			if got := f1n1SetSig(manual); got != preSig {
				t.Fatalf("oracle2: manual inverse did not restore the pre-state (%s != %s)", got, preSig)
			}

			// Neither oracle may disturb the post-state.
			if got := f1n1SetSig(post); got != postSnapshotSig {
				t.Fatalf("a reverse oracle mutated the post-state")
			}

			// ── determinism on identical inputs ──
			_, undo2, fees2, err := ApplyBlockWithUndo(pre.Clone(), txs, tc.height)
			if err != nil {
				t.Fatalf("ApplyBlockWithUndo (repeat): %v", err)
			}
			if fees2 != fees {
				t.Fatalf("fees not reproducible: %d vs %d", fees2, fees)
			}
			if !bytes.Equal(f1n1Encode(t, undo), f1n1Encode(t, undo2)) {
				t.Fatalf("undo is NOT deterministic for identical (base, txs, height)")
			}

			// ── encoding round-trip ──
			enc := f1n1Encode(t, undo)
			dec, err := DecodeUndo(enc)
			if err != nil {
				t.Fatalf("DecodeUndo: %v", err)
			}
			if !bytes.Equal(f1n1Encode(t, dec), enc) {
				t.Fatalf("undo encode/decode round-trip mismatch")
			}
			// The decoded undo must still invert the post-state.
			restored2, err := DisconnectBlock(post.Clone(), dec)
			if err != nil {
				t.Fatalf("oracle1 (decoded undo): DisconnectBlock: %v", err)
			}
			if got := f1n1SetSig(restored2); got != preSig {
				t.Fatalf("oracle1 (decoded undo): did not restore the pre-state")
			}

			t.Logf("F1N1-F %-30s pre=%d post=%d created=%d spent=%d fees=%d undoBytes=%d",
				tc.name, preLen, post.Len(), len(undo.Created), len(undo.Spent), fees, len(enc))
		})
	}
}
