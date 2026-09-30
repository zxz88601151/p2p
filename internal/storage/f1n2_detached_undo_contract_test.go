package storage

// ════════════════════════════════════════════════════════════════════════════
// PHASE P2PCHAIN — F1-N2
// DETACHED-V2 UNDO CONTENT / REORG PERSISTENCE CONTRACT AUDIT
//
//   Test I — HasUndo vs UndoFor contract
//   Test J — Reorg linkage integrity (frame-level binding)
//   Test K — Negative lookup
//   Test L — Idempotent restart/read
//
// SCOPE DISCIPLINE (F1-N2 §1)
//   * test-only file. No production `.go` modification, no `_test.go` of a
//     pre-existing file modified, no protocol/storage-format/consensus change.
//   * `UndoFor` / `DisconnectBlock` are NOT wired into any consumer.
//   * F1-N1 helpers are REUSED (`f1n1OpenRW`, `iChain`, `iReadLog`, `iWriteLog`,
//     `iBlock`, `encodeFrame`, `decodeFrame`, `recTypeUndo`); new helpers are
//     prefixed `f1n2` so they cannot collide in `package storage`.
//
// WHY THESE TESTS EXIST (the F1-N1 G-3 gap)
//   Existing coverage was audited before writing anything:
//     * storage/v2_test.go `TestG04_UndoPersistenceAndBinding` covers undo
//       content on the **AppendCanonicalBlock** (canonical append) write path,
//       plus ErrUndoNotFound / ErrUndoBinding / ErrDuplicateUndo for a *stored*
//       fresh block.
//     * storage/v2_test.go `TestG04_SameHeightForkUndoCoexistence` covers
//       same-height coexistence on the **SaveBlockWithUndo** (test-called) path.
//     * r3/r4a crash matrices call `CommitReorg(detachedUndos, …)` but assert
//       only frame composition and counters — `r3Probe` (r3_crash_matrix_test.go:304)
//       carries NO undo-content field.
//   ⇒ Nothing validated the payload of an UNDO written by the **CommitReorg
//     detached-undo branch**, and nothing exercised a detached-v2 UNDO whose
//     `Spent` list is non-empty. That is what Test I/J/L add, at the frame level.
//     Test K adds the genuinely uncovered negative variants (never-existing hash;
//     legacy record; and the `ErrUndoNotFound` vs `ErrBlockNotFound` distinction).
//
// PRODUCTION FACTS RELIED ON (frozen baseline efe02a7)
//   * `CommitReorg` Phase 2 emits one UNDO frame per `detachedUndos` entry
//     (v2api.go:391-395) and Phase 4 indexes the true written offset
//     (v2api.go:412-419)
//   * `HasUndo` tests map membership only and returns bool — it never reads the
//     frame                                                (v2api.go:728-733)
//   * `UndoFor` is the only accessor that reads the payload, from the FILE via
//     `readFramePayloadAt` / `f.ReadAt`, validating frame type + frame blockHash
//     + undo.Height vs the index height                (v2api.go:737-756, 98-130)
//   * `interpretScan`'s second pass binds an UNDO only when its blockHash is in
//     `records`, and REJECTS a height-inconsistent UNDO at scan time
//     (v2.go:438-461) — so `HasUndo == true` implies the payload decoded and its
//     height agreed at scan time, while still saying nothing about whether the
//     payload is the *semantically correct* undo for that block.
// ════════════════════════════════════════════════════════════════════════════

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
	"p2pchain/internal/wallet"
)

// ── F1-N2 local helpers ─────────────────────────────────────────────────────

func f1n2Wallet(t *testing.T) *wallet.Wallet {
	t.Helper()
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("F1N2: NewWallet: %v", err)
	}
	return w
}

// f1n2SpendBlock builds a coinbase+spend block together with its GENUINE undo,
// derived by `ApplyBlockWithUndo` over a hand-built pre-state that this test
// owns. Because the spending input references a NON-coinbase entry, no maturity
// wait is needed and the spend is a real, signed, consensus-valid transaction —
// so the resulting BlockUndo has a non-empty `Spent` list without resorting to
// a fabricated payload.
func f1n2SpendBlock(t *testing.T, prev [32]byte, height int, tag byte) (b *block.Block, undo utxo.BlockUndo, pre, post *utxo.UTXOSet) {
	t.Helper()
	w := f1n2Wallet(t)

	pre = utxo.NewUTXOSet()
	op := utxo.OutPoint{Hash: [32]byte{0xF1, tag}, Index: 0}
	pre.Add(op, utxo.Entry{Value: 5000, PubKeyHash: w.PubKeyHash(), Height: height - 1})

	spend := &transaction.Transaction{
		Inputs:  []transaction.TxInput{{PrevTxHash: op.Hash, OutIndex: op.Index}},
		Outputs: []transaction.TxOutput{{Value: 4900, PubKeyHash: [20]byte{0x77}}},
	}
	// Mirror the established signing order: the sighash is taken before the
	// signature/pubkey fields are populated.
	sig, err := w.Sign(spend.Hash())
	if err != nil {
		t.Fatalf("F1N2: Sign: %v", err)
	}
	spend.Inputs[0].Signature = sig
	spend.Inputs[0].PubKey = w.PublicKey

	var pkh [20]byte
	pkh[0] = byte(height)
	pkh[1] = tag
	cb := transaction.NewCoinbaseTx(pkh, utxo.Subsidy(height), height)

	b = block.NewCandidateBlock(prev, pow.MaxTargetBits, []*transaction.Transaction{cb, spend})
	if found, _ := pow.Mine(b); !found {
		t.Fatalf("F1N2: mine failed height=%d tag=%d", height, tag)
	}

	np, u, _, err := utxo.ApplyBlockWithUndo(pre, b.Transactions, height)
	if err != nil {
		t.Fatalf("F1N2: ApplyBlockWithUndo: %v", err)
	}
	return b, u, pre, np
}

// f1n2CommitDetachedUndo is the exact persistence shape `executeReorg` uses for
// the detached branch: the block is ALREADY stored (SaveBlockDetached, hence no
// UNDO) and `CommitReorg` is asked to attach the undo in its detached-undo map.
// The value comes from the freshly computed oracle, never from a stored payload.
func f1n2CommitDetachedUndo(t *testing.T, s *FileBlockStore, b *block.Block, u utxo.BlockUndo) {
	t.Helper()
	if err := s.SaveBlockDetached(b); err != nil {
		t.Fatalf("F1N2: SaveBlockDetached: %v", err)
	}
	if s.HasUndo(b.Header.Hash()) {
		t.Fatalf("F1N2 precondition: SaveBlockDetached wrote an UNDO")
	}
	du := map[[32]byte]utxo.BlockUndo{b.Header.Hash(): u}
	if err := s.CommitReorg(du, nil, nil, b.Header.Hash()); err != nil {
		t.Fatalf("F1N2: CommitReorg(detached-undo): %v", err)
	}
}

// f1n2Fixture is the shared store for Tests I/J/L: a canonical v2 base
// (h0, h1) plus a spend-bearing block D whose UNDO can only reach storage
// through the `CommitReorg` detached-undo branch.
type f1n2Fixture struct {
	dir  string
	s    *FileBlockStore
	base []*block.Block
	undo []utxo.BlockUndo
	// D — committed through the detached-undo branch; Spent is NON-EMPTY.
	d     *block.Block
	uD    utxo.BlockUndo
	preD  *utxo.UTXOSet
	postD *utxo.UTXOSet
}

func f1n2NewFixture(t *testing.T) *f1n2Fixture {
	t.Helper()
	dir := t.TempDir()
	base, undos := iChain(t, 3) // h0..h2, coinbase-only, real per-block undos
	s := f1n1OpenRW(t, dir)
	for i := 0; i < 2; i++ {
		if err := s.AppendCanonicalBlock(base[i], undos[i]); err != nil {
			t.Fatalf("F1N2: AppendCanonicalBlock(%d): %v", i, err)
		}
	}
	d, uD, preD, postD := f1n2SpendBlock(t, base[1].Header.Hash(), 2, 0xD1)
	f1n2CommitDetachedUndo(t, s, d, uD)
	return &f1n2Fixture{dir: dir, s: s, base: base, undo: undos, d: d, uD: uD, preD: preD, postD: postD}
}

// f1n2AssertSpentBearing asserts that the retrieved D payload is a genuine,
// complete, semantically inverting undo — and that it is NOT vacuous.
func f1n2AssertSpentBearing(t *testing.T, label string, got utxo.BlockUndo, f *f1n2Fixture) {
	t.Helper()
	if !reflect.DeepEqual(got, f.uD) {
		t.Fatalf("%s: payload != genuine oracle\ngot  =%+v\nwant =%+v", label, got, f.uD)
	}
	gb, err := utxo.EncodeUndo(got)
	if err != nil {
		t.Fatalf("%s: EncodeUndo(got): %v", label, err)
	}
	ob, err := utxo.EncodeUndo(f.uD)
	if err != nil {
		t.Fatalf("%s: EncodeUndo(oracle): %v", label, err)
	}
	if !bytes.Equal(gb, ob) {
		t.Fatalf("%s: encoded payload differs from the oracle", label)
	}
	// The fixture must actually exercise the Spent side, otherwise this whole
	// test would silently degrade into the coinbase-only case.
	if len(got.Spent) == 0 {
		t.Fatalf("%s: fixture degenerated — Spent is empty", label)
	}
	if len(got.Created) == 0 {
		t.Fatalf("%s: fixture degenerated — Created is empty", label)
	}
	if got.PreSetItemsCount != f.preD.Len() {
		t.Fatalf("%s: PreSetItemsCount=%d, want %d", label, got.PreSetItemsCount, f.preD.Len())
	}
	// Independent expectation of the entry counts from the state transition.
	expCreated, expSpent := 0, 0
	for op := range f.postD.AllEntries() {
		if _, in := f.preD.Get(op); !in {
			expCreated++
		}
	}
	for op := range f.preD.AllEntries() {
		if _, in := f.postD.Get(op); !in {
			expSpent++
		}
	}
	if len(got.Created) != expCreated || len(got.Spent) != expSpent {
		t.Fatalf("%s: counts Created=%d/%d Spent=%d/%d", label,
			len(got.Created), expCreated, len(got.Spent), expSpent)
	}
	// Per-entry semantics.
	for i, ce := range got.Created {
		if _, in := f.preD.Get(ce.OutPoint); in {
			t.Fatalf("%s: Created[%d] %s in pre-state", label, i, ce.OutPoint)
		}
		pe, in := f.postD.Get(ce.OutPoint)
		if !in || pe != ce.Entry {
			t.Fatalf("%s: Created[%d] %s entry mismatch", label, i, ce.OutPoint)
		}
	}
	for i, se := range got.Spent {
		if _, in := f.postD.Get(se.OutPoint); in {
			t.Fatalf("%s: Spent[%d] %s in post-state", label, i, se.OutPoint)
		}
		pe, in := f.preD.Get(se.OutPoint)
		if !in || pe != se.Entry {
			t.Fatalf("%s: Spent[%d] %s entry mismatch", label, i, se.OutPoint)
		}
	}
	// Semantic inversion — independent of any oracle function.
	restored, err := utxo.DisconnectBlock(f.postD, got)
	if err != nil {
		t.Fatalf("%s: DisconnectBlock: %v", label, err)
	}
	if restored.Len() != f.preD.Len() {
		t.Fatalf("%s: restored Len=%d, want %d", label, restored.Len(), f.preD.Len())
	}
	for op, e := range f.preD.AllEntries() {
		re, in := restored.Get(op)
		if !in || re != e {
			t.Fatalf("%s: restored entry %s mismatch", label, op)
		}
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Test I — HasUndo vs UndoFor contract
// ════════════════════════════════════════════════════════════════════════════

func TestF1N2_I_HasUndoVsUndoForContract(t *testing.T) {
	f := f1n2NewFixture(t)
	defer func() { _ = f.s.Close() }()

	// ── I/1: the detached-undo branch persisted a COMPLETE payload ──
	if !f.s.HasUndo(f.d.Header.Hash()) {
		t.Fatalf("I/1: HasUndo(D)=false, want true")
	}
	gotD, err := f.s.UndoFor(f.d.Header.Hash())
	if err != nil {
		t.Fatalf("I/1: UndoFor(D): %v", err)
	}
	f1n2AssertSpentBearing(t, "I/1[D,detached-undo branch]", gotD, f)

	// ── I/2: HasUndo == false corresponds to a genuine absence of payload ──
	// E is stored detached and never committed: it exists as a block record.
	e, uE, preE, postE := f1n2SpendBlock(t, f.base[1].Header.Hash(), 2, 0xD2)
	if err := f.s.SaveBlockDetached(e); err != nil {
		t.Fatalf("I/2: SaveBlockDetached(E): %v", err)
	}
	eHash := e.Header.Hash()
	if !f.s.HasBlock(eHash) {
		t.Fatalf("I/2: E is not stored")
	}
	if f.s.HasUndo(eHash) {
		t.Fatalf("I/2: HasUndo(E)=true, want false")
	}
	if _, err := f.s.UndoFor(eHash); !errors.Is(err, ErrUndoNotFound) {
		t.Fatalf("I/2: UndoFor(E) err=%v, want ErrUndoNotFound", err)
	}

	// ── I/3: attach a payload to E, then show HasUndo cannot distinguish the
	//          two blocks while UndoFor can ──
	if err := f.s.PutUndo(eHash, 2, uE); err != nil {
		t.Fatalf("I/3: PutUndo(E): %v", err)
	}
	if !f.s.HasUndo(f.d.Header.Hash()) || !f.s.HasUndo(eHash) {
		t.Fatalf("I/3: both D and E must report HasUndo=true")
	}
	gotE, err := f.s.UndoFor(eHash)
	if err != nil {
		t.Fatalf("I/3: UndoFor(E): %v", err)
	}
	// E's own oracle must hold (same independent derivation, own pre-state).
	ef := &f1n2Fixture{preD: preE, postD: postE, uD: uE}
	f1n2AssertSpentBearing(t, "I/3[E,PutUndo]", gotE, ef)

	// Two blocks at the SAME height, both HasUndo=true, DIFFERENT content:
	// ⇒ existence carries no information about content.
	if got := f.s.DetachedCount(); got != 1 {
		t.Fatalf("I/3: DetachedCount=%d, want 1 (E is the only detached v2 block)", got)
	}
	encD, err := utxo.EncodeUndo(gotD)
	if err != nil {
		t.Fatalf("I/3: EncodeUndo(D): %v", err)
	}
	encE, err := utxo.EncodeUndo(gotE)
	if err != nil {
		t.Fatalf("I/3: EncodeUndo(E): %v", err)
	}
	if bytes.Equal(encD, encE) {
		t.Fatalf("I/3: D and E payloads are identical — fixture cannot discriminate")
	}
	// And neither reader ever returns the other's payload.
	if !bytes.Equal(encE, mustEncode(t, uE)) {
		t.Fatalf("I/3: UndoFor(E) does not return E's own payload")
	}
	if !bytes.Equal(encD, mustEncode(t, f.uD)) {
		t.Fatalf("I/3: UndoFor(D) does not return D's own payload")
	}
}

// TestF1N2_Ib_ScanRejectsHeightInconsistentUndo documents WHY `HasUndo == true`
// is a trustworthy existence signal, and simultaneously why it can never be
// accepted in place of a content check.
//
// `interpretScan`'s second pass enforces `undo.Height == frame.Height ==
// rec.height` at scan time (v2.go:455-458). A frame violating it is REJECTED —
// so the store cannot even open. That is the boundary which makes the index a
// reliable existence oracle; the semantic correctness of the undo *for that
// block* is nevertheless only observable through `UndoFor` + an oracle.
func TestF1N2_Ib_ScanRejectsHeightInconsistentUndo(t *testing.T) {
	dir := t.TempDir()
	base, undos := iChain(t, 3)
	s := f1n1OpenRW(t, dir)
	for i := 0; i < 2; i++ {
		if err := s.AppendCanonicalBlock(base[i], undos[i]); err != nil {
			t.Fatalf("F1N2: AppendCanonicalBlock(%d): %v", i, err)
		}
	}
	// D is recorded (so its hash IS in `records`) but has NO undo frame.
	d, uD, _, _ := f1n2SpendBlock(t, base[1].Header.Hash(), 2, 0xD1)
	if err := s.SaveBlockDetached(d); err != nil {
		t.Fatalf("F1N2: SaveBlockDetached: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("F1N2: close: %v", err)
	}

	// Craft: frame header height = 2 (D's true height) but the PAYLOAD claims 99.
	bad := uD
	bad.Height = 99
	pb, err := utxo.EncodeUndo(bad)
	if err != nil {
		t.Fatalf("F1N2: EncodeUndo(bad): %v", err)
	}
	frame := encodeFrame(recTypeUndo, 2, d.Header.Hash(), pb)
	iWriteLog(t, dir, append(iReadLog(t, dir), frame...))

	_, err = OpenFileBlockStore(dir)
	if err == nil {
		t.Fatalf("F1N2: height-inconsistent UNDO was accepted at scan time")
	}
	if !errors.Is(err, ErrUndoBinding) {
		t.Fatalf("F1N2: reopen err=%v, want ErrUndoBinding", err)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Test J — Reorg linkage integrity (frame-level binding)
// ════════════════════════════════════════════════════════════════════════════

func TestF1N2_J_ReorgLinkageIntegrity(t *testing.T) {
	f := f1n2NewFixture(t)
	defer func() { _ = f.s.Close() }()

	// A second same-height sibling with its own payload, so two bindings exist.
	e, uE, _, _ := f1n2SpendBlock(t, f.base[1].Header.Hash(), 2, 0xD2)
	if err := f.s.SaveBlockDetached(e); err != nil {
		t.Fatalf("J: SaveBlockDetached(E): %v", err)
	}
	if err := f.s.PutUndo(e.Header.Hash(), 2, uE); err != nil {
		t.Fatalf("J: PutUndo(E): %v", err)
	}

	raw := iReadLog(t, f.dir)

	type binding struct {
		name string
		hash [32]byte
		undo utxo.BlockUndo
	}
	for _, bd := range []binding{
		{"D(detached-undo branch)", f.d.Header.Hash(), f.uD},
		{"E(PutUndo)", e.Header.Hash(), uE},
	} {
		// (1) the index entry exists and is keyed by THIS block's hash
		ur, ok := f.s.v2.undoIndex[bd.hash]
		if !ok {
			t.Fatalf("J/%s: no undoIndex entry", bd.name)
		}
		if ur.blockHash != bd.hash {
			t.Fatalf("J/%s: index keyed by %x, want %x", bd.name, ur.blockHash[:4], bd.hash[:4])
		}
		if int(ur.height) != 2 {
			t.Fatalf("J/%s: index height=%d, want 2", bd.name, ur.height)
		}
		// (2) the FRAME at that offset is an UNDO frame bound to THIS hash
		if ur.offset <= 0 || int(ur.offset+8) > len(raw) {
			t.Fatalf("J/%s: implausible offset %d", bd.name, ur.offset)
		}
		fr, err := decodeFrame(raw[ur.offset:])
		if err != nil {
			t.Fatalf("J/%s: decodeFrame at %d: %v", bd.name, ur.offset, err)
		}
		if fr.Type != recTypeUndo {
			t.Fatalf("J/%s: frame type=%s, want %s", bd.name, recTypeName(fr.Type), recTypeName(recTypeUndo))
		}
		if fr.BlockHash != bd.hash {
			t.Fatalf("J/%s: frame blockHash=%x, want %x (bindings crossed)",
				bd.name, fr.BlockHash[:4], bd.hash[:4])
		}
		if int(fr.Height) != 2 {
			t.Fatalf("J/%s: frame height=%d, want 2", bd.name, fr.Height)
		}
		// (3) the frame payload is byte-identical to the payload the accessor returns
		enc := mustEncode(t, bd.undo)
		if !bytes.Equal(fr.Payload, enc) {
			t.Fatalf("J/%s: frame payload differs from the authoritative encoding", bd.name)
		}
		got, err := f.s.UndoFor(bd.hash)
		if err != nil {
			t.Fatalf("J/%s: UndoFor: %v", bd.name, err)
		}
		if !bytes.Equal(mustEncode(t, got), enc) {
			t.Fatalf("J/%s: UndoFor payload differs from the frame payload", bd.name)
		}
	}

	// (4) the two bindings are physically distinct and semantically distinct:
	//     no neighbouring lookup can return the other's payload.
	urD := f.s.v2.undoIndex[f.d.Header.Hash()]
	urE := f.s.v2.undoIndex[e.Header.Hash()]
	if urD.offset == urE.offset {
		t.Fatalf("J: D and E share one frame offset %d", urD.offset)
	}
	encD, encE := mustEncode(t, f.uD), mustEncode(t, uE)
	if bytes.Equal(encD, encE) {
		t.Fatalf("J: fixture payloads are identical")
	}
	if bytes.Contains(encD, encE) || bytes.Contains(encE, encD) {
		t.Fatalf("J: one payload contains the other — cross-read would be undetectable")
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Test K — Negative lookup
// ════════════════════════════════════════════════════════════════════════════

func TestF1N2_K_NegativeLookup(t *testing.T) {
	f := f1n2NewFixture(t)
	defer func() { _ = f.s.Close() }()

	// K/1 — a hash that NEVER existed: UndoFor reports ErrUndoNotFound, not
	// ErrBlockNotFound. The two APIs deliberately use different sentinels.
	var absent [32]byte
	if _, err := f.s.UndoFor(absent); !errors.Is(err, ErrUndoNotFound) {
		t.Fatalf("K/1: UndoFor(absent) err=%v, want ErrUndoNotFound", err)
	}
	if _, err := f.s.UndoFor(absent); errors.Is(err, ErrBlockNotFound) {
		t.Fatalf("K/1: UndoFor(absent) surfaced ErrBlockNotFound")
	}
	if f.s.HasUndo(absent) {
		t.Fatalf("K/1: HasUndo(absent)=true")
	}
	// Contrast: the block-oriented accessors DO use ErrBlockNotFound.
	if _, err := f.s.IsCanonical(absent); !errors.Is(err, ErrBlockNotFound) {
		t.Fatalf("K/1: IsCanonical(absent) err=%v, want ErrBlockNotFound", err)
	}

	// K/2 — a stored detached v2 block that never received an UNDO.
	nb, _, _, _ := f1n2SpendBlock(t, f.base[1].Header.Hash(), 2, 0xD9)
	if err := f.s.SaveBlockDetached(nb); err != nil {
		t.Fatalf("K/2: SaveBlockDetached: %v", err)
	}
	if f.s.HasUndo(nb.Header.Hash()) {
		t.Fatalf("K/2: HasUndo=true for a never-committed detached block")
	}
	if _, err := f.s.UndoFor(nb.Header.Hash()); !errors.Is(err, ErrUndoNotFound) {
		t.Fatalf("K/2: err=%v, want ErrUndoNotFound", err)
	}

	// K/3 — a LEGACY record: it is canonical-prefix history with no UNDO.
	ldir := t.TempDir()
	legacy := iSeedLegacy(t, ldir, 2)
	ls := f1n1OpenRW(t, ldir)
	defer func() { _ = ls.Close() }()
	lh := legacy[0].Header.Hash()
	if ls.HasUndo(lh) {
		t.Fatalf("K/3: legacy record reports HasUndo=true")
	}
	if _, err := ls.UndoFor(lh); !errors.Is(err, ErrUndoNotFound) {
		t.Fatalf("K/3: UndoFor(legacy) err=%v, want ErrUndoNotFound", err)
	}

	// K/4 — PutUndo on a hash that is not a record at all.
	if err := f.s.PutUndo(absent, 2, f.uD); !errors.Is(err, ErrBlockNotFound) {
		t.Fatalf("K/4: PutUndo(unknown) err=%v, want ErrBlockNotFound", err)
	}
	// K/5 — duplicate UNDO for a block that already has one.
	if err := f.s.PutUndo(f.d.Header.Hash(), 2, f.uD); !errors.Is(err, ErrDuplicateUndo) {
		t.Fatalf("K/5: PutUndo(dup) err=%v, want ErrDuplicateUndo", err)
	}
	// K/6 — height binding violation on a record that has no UNDO yet.
	if err := f.s.PutUndo(nb.Header.Hash(), 7, f.uD); !errors.Is(err, ErrUndoBinding) {
		t.Fatalf("K/6: PutUndo(wrong height) err=%v, want ErrUndoBinding", err)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Test L — Idempotent restart/read
// ════════════════════════════════════════════════════════════════════════════

func TestF1N2_L_IdempotentRestartRead(t *testing.T) {
	f := f1n2NewFixture(t)

	e, uE, _, _ := f1n2SpendBlock(t, f.base[1].Header.Hash(), 2, 0xD2)
	if err := f.s.SaveBlockDetached(e); err != nil {
		t.Fatalf("L: SaveBlockDetached(E): %v", err)
	}
	if err := f.s.PutUndo(e.Header.Hash(), 2, uE); err != nil {
		t.Fatalf("L: PutUndo(E): %v", err)
	}

	dHash, eHash := f.d.Header.Hash(), e.Header.Hash()
	wantD, wantE := mustEncode(t, f.uD), mustEncode(t, uE)
	tip0, _ := f.s.TipHash()
	ht0, _ := f.s.Height()

	// Repeated reads within one session must be identical.
	if err := f.s.Close(); err != nil {
		t.Fatalf("L: close: %v", err)
	}

	for cycle := 1; cycle <= 2; cycle++ {
		sN := f1n1OpenRW(t, f.dir)

		if got := sN.RecoveryMode(); got != "REBUILD" {
			t.Fatalf("L[%d]: RecoveryMode=%q, want REBUILD", cycle, got)
		}
		if got, _ := sN.Height(); got != ht0 {
			t.Fatalf("L[%d]: Height=%d, want %d", cycle, got, ht0)
		}
		if tip, has := sN.TipHash(); !has || tip != tip0 {
			t.Fatalf("L[%d]: tip drifted", cycle)
		}
		if got := sN.DanglingUndoCount(); got != 0 {
			t.Fatalf("L[%d]: DanglingUndoCount=%d, want 0", cycle, got)
		}
		if !sN.HasUndo(dHash) || !sN.HasUndo(eHash) {
			t.Fatalf("L[%d]: HasUndo lost for D or E", cycle)
		}

		// Two consecutive reads in this cycle, plus comparison to the
		// pre-restart snapshot, must all agree exactly.
		for i := 0; i < 2; i++ {
			gD, err := sN.UndoFor(dHash)
			if err != nil {
				t.Fatalf("L[%d/%d]: UndoFor(D): %v", cycle, i, err)
			}
			gE, err := sN.UndoFor(eHash)
			if err != nil {
				t.Fatalf("L[%d/%d]: UndoFor(E): %v", cycle, i, err)
			}
			if !bytes.Equal(mustEncode(t, gD), wantD) {
				t.Fatalf("L[%d/%d]: D payload drifted", cycle, i)
			}
			if !bytes.Equal(mustEncode(t, gE), wantE) {
				t.Fatalf("L[%d/%d]: E payload drifted", cycle, i)
			}
		}
		if err := sN.Close(); err != nil {
			t.Fatalf("L[%d]: close: %v", cycle, err)
		}
	}
}

// mustEncode is a tiny local wrapper so assertions read cleanly.
func mustEncode(t *testing.T, u utxo.BlockUndo) []byte {
	t.Helper()
	b, err := utxo.EncodeUndo(u)
	if err != nil {
		t.Fatalf("F1N2: EncodeUndo: %v", err)
	}
	return b
}

var _ = fmt.Sprintf // keep fmt available for future diagnostics without churn
