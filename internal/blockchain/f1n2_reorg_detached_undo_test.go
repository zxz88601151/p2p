package blockchain

// ════════════════════════════════════════════════════════════════════════════
// PHASE P2PCHAIN — F1-N2
// DETACHED-V2 UNDO CONTENT / REORG PERSISTENCE CONTRACT AUDIT
//
//   Test G — Detached-v2 UNDO content oracle   (reorg-produced, end-to-end)
//   Test H — Detached-v2 UNDO persistence across restart
//
// SCOPE DISCIPLINE (F1-N2 §1)
//   * test-only file. No production `.go`, storage primitive, reorg
//     implementation, `PutUndo` / `interpretScan` / `CommitReorg` change is made,
//     and `UndoFor` / `DisconnectBlock` are NOT wired into any consumer.
//   * every scenario runs on its own t.TempDir() — isolated storage only.
//   * F1-N1 test files are read/reused but NEVER modified; all new helpers here
//     are prefixed `f1n2` to avoid redeclaration in `package blockchain`.
//
// WHY THIS PHASE EXISTS (the F1-N1 G-3 gap)
//   F1-N1 Test B observed that the v2 reorg persists an UNDO for every block of
//   the adopted branch, but asserted only `HasUndo(...) == true` — i.e. an
//   EXISTENCE MARKER. It never compared the persisted payload against an
//   independently derived oracle. Likewise the pre-existing lineages
//   (`TestG04_UndoPersistenceAndBinding`, `TestG04_SameHeightForkUndoCoexistence`
//   in storage/v2_test.go) exercise only the `AppendCanonicalBlock` and
//   `SaveBlockWithUndo` write paths, and the r3/r4a crash matrices call
//   `CommitReorg(detachedUndos, …)` while asserting only frame composition and
//   counters (see `r3Probe` — it carries no UNDO content field at all).
//   This file closes that gap for the ONE path nothing else covers: the
//   `CommitReorg` **detached-undo** branch, reached end-to-end through
//   `Blockchain.AddBlock` → `executeReorg` → `CommitReorg`.
//
// PRODUCTION FACTS THIS FILE RELIES ON (frozen baseline efe02a7)
//   * `extendChain` writes UNDO on the canonical path via `ApplyBlockWithUndo`
//     + `AppendCanonicalBlock`            (blockchain.go:587-596)
//   * the fork path stores the block with `SaveBlockDetached` — block ONLY,
//     no UNDO                             (blockchain.go:571-576)
//   * `executeReorg` builds `detachedUndos` from connectPath members where
//     `HasBlock(h) && !HasUndo(h)`, taking the value from the freshly computed
//     `newUndos[i]` — never from the stored payload
//                                         (blockchain.go:866-871)
//   * `CommitReorg` writes an UNDO frame for each detached entry in Phase 2
//     and indexes it in Phase 4             (v2api.go:391-395, 412-419)
//   * `UndoFor` re-reads the payload from the FILE via `readFramePayloadAt`
//     (`f.ReadAt`) and validates frame type + frame blockHash + undo.Height
//                                         (v2api.go:737-756, 98-130)
// ════════════════════════════════════════════════════════════════════════════

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/storage"
	"p2pchain/internal/utxo"
)

// ── F1-N2 local helpers ─────────────────────────────────────────────────────

// f1n2SetSig renders a UTXO set deterministically, so two independently
// constructed sets can be compared entry-for-entry (the utxo package's
// assertSetEqual helper is not reachable from `package blockchain`).
func f1n2SetSig(s *utxo.UTXOSet) string {
	m := s.AllEntries()
	lines := make([]string, 0, len(m))
	for op, e := range m {
		lines = append(lines, fmt.Sprintf("%s|%d|%x|%d|%v",
			op.String(), e.Value, e.PubKeyHash, e.Height, e.IsCoinbase))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// f1n2Replay derives — from scratch and in a test-owned loop — the pre-state,
// post-state and canonical UNDO of path[idx] by replaying path[0..idx].
//
// This is the independent oracle: it never consults the store's undo reader,
// so a retrieved payload can be compared against something derived purely from
// block data and the UTXO transition rules.
func f1n2Replay(t *testing.T, path []*block.Block, idx int) (pre, post *utxo.UTXOSet, undo utxo.BlockUndo) {
	t.Helper()
	set := utxo.NewUTXOSet()
	for i := 0; i < idx; i++ {
		ns, _, err := utxo.ApplyBlock(set, path[i].Transactions, i)
		if err != nil {
			t.Fatalf("F1N2 oracle replay: apply h=%d: %v", i, err)
		}
		set = ns
	}
	pre = set.Clone()
	np, u, _, err := utxo.ApplyBlockWithUndo(pre, path[idx].Transactions, idx)
	if err != nil {
		t.Fatalf("F1N2 oracle replay: undo h=%d: %v", idx, err)
	}
	return pre, np, u
}

// f1n2AssertUndoContent is the core G-3 assertion. It validates the payload
// retrieved through `UndoFor` on four independent axes:
//
//	(1) struct equality vs the replayed oracle
//	(2) byte equality of the encoded payload vs the replayed oracle
//	(3) explicit field semantics derived from the pre/post STATE TRANSITION
//	    itself (entry counts, PreSetItemsCount, per-entry outpoint + Entry
//	    membership and value, Created ordering)
//	(4) semantic inversion: DisconnectBlock(post, retrieved) must restore pre
//
// Axis (3)/(4) are deliberately independent of axis (1)/(2): they would still
// catch a payload that matched a corrupted oracle.
func f1n2AssertUndoContent(t *testing.T, label string, s *storage.FileBlockStore, path []*block.Block, idx int) {
	t.Helper()
	b := path[idx]
	h := b.Header.Hash()

	if !s.HasUndo(h) {
		t.Fatalf("%s: HasUndo(%s)=false, want true", label, hx4(h))
	}
	got, err := s.UndoFor(h)
	if err != nil {
		t.Fatalf("%s: UndoFor(%s): %v", label, hx4(h), err)
	}
	pre, post, want := f1n2Replay(t, path, idx)

	// (1) whole-struct equality against the oracle.
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: retrieved undo != oracle\ngot  =%+v\nwant =%+v", label, got, want)
	}
	// (2) byte-level equality of the canonical encoding.
	gb, err := utxo.EncodeUndo(got)
	if err != nil {
		t.Fatalf("%s: EncodeUndo(got): %v", label, err)
	}
	wb, err := utxo.EncodeUndo(want)
	if err != nil {
		t.Fatalf("%s: EncodeUndo(want): %v", label, err)
	}
	if !bytes.Equal(gb, wb) {
		t.Fatalf("%s: encoded payload differs (got %d B, want %d B)", label, len(gb), len(wb))
	}

	// (3) field semantics derived from the state transition, not the oracle.
	if got.Height != idx {
		t.Fatalf("%s: undo.Height=%d, want %d", label, got.Height, idx)
	}
	if got.PreSetItemsCount != pre.Len() {
		t.Fatalf("%s: PreSetItemsCount=%d, want pre-block Len()=%d",
			label, got.PreSetItemsCount, pre.Len())
	}
	expCreated, expSpent := 0, 0
	for op := range post.AllEntries() {
		if _, in := pre.Get(op); !in {
			expCreated++
		}
	}
	for op := range pre.AllEntries() {
		if _, in := post.Get(op); !in {
			expSpent++
		}
	}
	if len(got.Created) != expCreated {
		t.Fatalf("%s: len(Created)=%d, want %d (derived from the transition)",
			label, len(got.Created), expCreated)
	}
	if len(got.Spent) != expSpent {
		t.Fatalf("%s: len(Spent)=%d, want %d (derived from the transition)",
			label, len(got.Spent), expSpent)
	}

	// Created: absent from pre, present in post, Entry == post's Entry.
	for i, ce := range got.Created {
		if _, in := pre.Get(ce.OutPoint); in {
			t.Fatalf("%s: Created[%d] %s present in the pre-state", label, i, ce.OutPoint)
		}
		pe, in := post.Get(ce.OutPoint)
		if !in {
			t.Fatalf("%s: Created[%d] %s absent from the post-state", label, i, ce.OutPoint)
		}
		if pe != ce.Entry {
			t.Fatalf("%s: Created[%d] %s entry mismatch\n got=%+v\nwant=%+v",
				label, i, ce.OutPoint, ce.Entry, pe)
		}
	}
	// Spent: absent from post, present in pre, Entry == pre's Entry.
	for i, se := range got.Spent {
		if _, in := post.Get(se.OutPoint); in {
			t.Fatalf("%s: Spent[%d] %s still present in the post-state", label, i, se.OutPoint)
		}
		pe, in := pre.Get(se.OutPoint)
		if !in {
			t.Fatalf("%s: Spent[%d] %s absent from the pre-state", label, i, se.OutPoint)
		}
		if pe != se.Entry {
			t.Fatalf("%s: Spent[%d] %s entry mismatch\n got=%+v\nwant=%+v",
				label, i, se.OutPoint, se.Entry, pe)
		}
	}
	// Created must be in strictly ascending OutPoint order — a documented
	// determinism contract of BlockUndo (undo.go:99-105).
	for i := 1; i < len(got.Created); i++ {
		a, c := got.Created[i-1].OutPoint, got.Created[i].OutPoint
		cmp := bytes.Compare(a.Hash[:], c.Hash[:])
		if cmp > 0 || (cmp == 0 && a.Index >= c.Index) {
			t.Fatalf("%s: Created not in strictly ascending OutPoint order at %d (%s then %s)",
				label, i, a, c)
		}
	}

	// (4) semantic inversion, independent of any oracle function.
	restored, err := utxo.DisconnectBlock(post, got)
	if err != nil {
		t.Fatalf("%s: DisconnectBlock(post, retrieved): %v", label, err)
	}
	if f1n2SetSig(restored) != f1n2SetSig(pre) {
		t.Fatalf("%s: DisconnectBlock did not restore the pre-block state exactly", label)
	}
}

// f1n2Topo is the mixed topology used by Test G and Test H.
type f1n2Topo struct {
	gen *block.Block
	// Old branch — canonical before the reorg, displaced afterwards.
	c1, c2 *block.Block
	// Fork branch — wins the reorg. f1/f2 are stored detached WITHOUT undo
	// before the fork becomes heavier, so their UNDO is produced by the
	// `CommitReorg` detached-undo branch, not by the canonical append path.
	f1, f2, f3 *block.Block
}

func (tp *f1n2Topo) oldPath() []*block.Block { return []*block.Block{tp.gen, tp.c1, tp.c2} }
func (tp *f1n2Topo) newPath() []*block.Block { return []*block.Block{tp.gen, tp.f1, tp.f2, tp.f3} }

// f1n2BuildReorg constructs the topology through the REAL reorg mechanism
// (`AddBlock` → `executeReorg` → `CommitReorg`) and returns the still-open
// store, the chain, the topology and the log bytes captured immediately before
// the reorg-triggering block was added.
func f1n2BuildReorg(t *testing.T) (string, *storage.FileBlockStore, *Blockchain, *f1n2Topo, []byte) {
	t.Helper()
	dir := t.TempDir()
	s := f1n1Store(t, dir)

	gen := NewGenesisBlock()
	if err := s.AppendCanonicalBlock(gen, utxo.BlockUndo{Height: 0}); err != nil {
		t.Fatalf("F1N2: AppendCanonicalBlock(genesis): %v", err)
	}
	bc := f1n1ChainFromStore(t, s)

	// ── canonical branch: gen → c1 → c2, UNDO written by the canonical path ──
	c1 := f1n1Mine(t, gen.Header.Hash(), 1, 0xB1)
	if err := bc.AddBlock(c1); err != nil {
		t.Fatalf("F1N2: AddBlock(c1): %v", err)
	}
	// c2 is harvested as a pool MAXIMUM so that f2 (below) can be constructed
	// under it with negligible failure probability — the f2/c2 tie is
	// cross-parent, so it cannot be ordered exactly by construction.
	c2 := f1n1HarvestWinnerAbove(t, c1.Header.Hash(), 2, 8)
	if err := bc.AddBlock(c2); err != nil {
		t.Fatalf("F1N2: AddBlock(c2): %v", err)
	}
	for _, b := range []*block.Block{c1, c2} {
		if !s.HasUndo(b.Header.Hash()) {
			t.Fatalf("F1N2 precondition: canonical %s has no UNDO", hx4(b.Header.Hash()))
		}
	}

	// ── fork branch ──
	// f1/f2 are stored detached WITHOUT undo: the fork is lighter than the
	// canonical chain, so `addBlock` takes the SaveBlockDetached path
	// (blockchain.go:571-576).
	f1 := f1n1Mine(t, gen.Header.Hash(), 1, 0xB5)
	if err := bc.AddBlock(f1); err != nil {
		t.Fatalf("F1N2: AddBlock(f1): %v", err)
	}
	// f2 is harvested to hash BELOW c2 so it deterministically loses the
	// work-tie at equal height and is merely parked (settip.go tie-break).
	f2 := f1n1HarvestLoserBelow(t, f1.Header.Hash(), 2, c2.Header.Hash(), 64)
	if err := bc.AddBlock(f2); err != nil {
		t.Fatalf("F1N2: AddBlock(f2): %v", err)
	}
	if got := f1n1Tip(t, bc); got != c2.Header.Hash() {
		t.Fatalf("F1N2 precondition: the tie-break adopted f2; determinism assumption broke")
	}

	// THE G-3 PRECONDITION: stored, non-canonical, and *without* UNDO.
	for _, b := range []*block.Block{f1, f2} {
		h := b.Header.Hash()
		if !s.HasBlock(h) {
			t.Fatalf("F1N2 precondition: fork %s is not stored", hx4(h))
		}
		if s.HasUndo(h) {
			t.Fatalf("F1N2 precondition: fork %s already has an UNDO", hx4(h))
		}
		if f1n1Canonical(t, s, h) {
			t.Fatalf("F1N2 precondition: fork %s is canonical", hx4(h))
		}
		if _, err := s.UndoFor(h); err == nil {
			t.Fatalf("F1N2 precondition: UndoFor(fork %s) unexpectedly succeeded", hx4(h))
		}
	}

	logBefore, err := os.ReadFile(s.FilePath())
	if err != nil {
		t.Fatalf("F1N2: read log before reorg: %v", err)
	}

	// f3 makes the fork strictly heavier (3 > 2) ⇒ the reorg MUST run, and it
	// must write f1/f2's UNDO through the detached-undo branch.
	f3 := f1n1Mine(t, f2.Header.Hash(), 3, 0xB7)
	if err := bc.AddBlock(f3); err != nil {
		t.Fatalf("F1N2: AddBlock(f3) (reorg): %v", err)
	}

	if got := bc.Height(); got != 3 {
		t.Fatalf("F1N2: Height=%d after the reorg, want 3", got)
	}
	if got := f1n1Tip(t, bc); got != f3.Header.Hash() {
		t.Fatalf("F1N2: tip=%s after the reorg, want %s", hx4(got), hx4(f3.Header.Hash()))
	}
	return dir, s, bc, &f1n2Topo{gen: gen, c1: c1, c2: c2, f1: f1, f2: f2, f3: f3}, logBefore
}

// ════════════════════════════════════════════════════════════════════════════
// Test G — Detached-v2 UNDO content oracle
// ════════════════════════════════════════════════════════════════════════════

func TestF1N2_G_DetachedV2UndoContentOracle(t *testing.T) {
	_, s, _, tp, logBefore := f1n2BuildReorg(t)
	defer func() { _ = s.Close() }()

	// ── G/1: the reorg-produced UNDO now exists for the previously bare forks ──
	for _, b := range []*block.Block{tp.f1, tp.f2} {
		if !s.HasUndo(b.Header.Hash()) {
			t.Fatalf("G/1: reorg did not produce an UNDO for %s", hx4(b.Header.Hash()))
		}
	}
	// f1/f2 are canonical after the winning reorg; c1/c2 are the displaced set.
	for _, b := range []*block.Block{tp.f1, tp.f2, tp.f3} {
		if !f1n1Canonical(t, s, b.Header.Hash()) {
			t.Fatalf("G/1: %s is not canonical after the reorg", hx4(b.Header.Hash()))
		}
	}
	for _, b := range []*block.Block{tp.c1, tp.c2} {
		if f1n1Canonical(t, s, b.Header.Hash()) {
			t.Fatalf("G/1: displaced %s is still canonical", hx4(b.Header.Hash()))
		}
	}

	// ── G/2: CONTENT audit of the reorg-produced (detached-branch) UNDOs ──
	// f1 and f2 reach the store through `detachedUndos`; f3 is the newly
	// persisted block of the same CommitReorg batch. All three are audited.
	//
	// Index 0 (genesis) is excluded on purpose, and this is NOT an easing of
	// the assertion — it is a scope boundary:
	//   * genesis is the common ancestor of EVERY path and is never a member of
	//     connectPath/disconnectPath, so it is never a *reorg-produced* undo,
	//     which is precisely what G-3 is about;
	//   * production never gives genesis an undo frame at all — an empty store
	//     is seeded through the LEGACY path `store.SaveBlock(genesis)`
	//     (blockchain.go:224-227), which writes no UNDO.
	// This fixture (like F1-N1 Test B1 and reorg_test.go:43-45) merely forces v2
	// mode by writing genesis via `AppendCanonicalBlock(gen, BlockUndo{Height:0})`,
	// so genesis here carries a fixture-seeded EMPTY undo that by construction
	// cannot equal a replay of genesis' own state transition. Auditing it would
	// audit the fixture, not the contract. See report OBS-1.
	newPath := tp.newPath()
	for i := 1; i < len(newPath); i++ {
		f1n2AssertUndoContent(t, fmt.Sprintf("G/2[h=%d,detached-path]", i), s, newPath, i)
	}

	// ── G/3: detaching does NOT alter the UNDO of a displaced block ──
	// c1/c2 were canonical (UNDO written by AppendCanonicalBlock). The reorg
	// disconnected them; their payload must be byte-identical to the oracle for
	// their own original chain, and to whatever the store returned before.
	oldPath := tp.oldPath()
	for i := 1; i < len(oldPath); i++ {
		f1n2AssertUndoContent(t, fmt.Sprintf("G/3[h=%d,displaced]", i), s, oldPath, i)
	}
	// Their UNDO must survive the detach unchanged, not merely be re-derivable.
	for _, b := range []*block.Block{tp.c1, tp.c2} {
		if !s.HasUndo(b.Header.Hash()) {
			t.Fatalf("G/3: displaced %s lost its UNDO", hx4(b.Header.Hash()))
		}
	}

	// ── G/4: the reorg is append-only — earlier bytes are untouched ──
	logAfter, err := os.ReadFile(s.FilePath())
	if err != nil {
		t.Fatalf("G/4: read log after reorg: %v", err)
	}
	if !bytes.HasPrefix(logAfter, logBefore) {
		t.Fatalf("G/4: the reorg rewrote previously persisted bytes (log is not append-only)")
	}
	if len(logAfter) <= len(logBefore) {
		t.Fatalf("G/4: log did not grow across the reorg (%d -> %d)", len(logBefore), len(logAfter))
	}

	// ── G/5: the displaced set is exactly {c1,c2} — both v2, both non-canonical ──
	if got := s.DetachedCount(); got != 2 {
		t.Fatalf("G/5: DetachedCount=%d, want 2", got)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Test H — Detached-v2 UNDO persistence across restart
// ════════════════════════════════════════════════════════════════════════════

func TestF1N2_H_DetachedV2UndoPersistenceAcrossRestart(t *testing.T) {
	dir, s, _, tp, _ := f1n2BuildReorg(t)

	newPath := tp.newPath()
	oldPath := tp.oldPath()

	// Snapshot the in-session payloads first: these are the values that must be
	// reproduced after a reopen.
	type wantUndo struct {
		name string
		path []*block.Block
		idx  int
		hash [32]byte
		enc  []byte
	}
	var wants []wantUndo
	add := func(name string, path []*block.Block, idx int) {
		b := path[idx]
		h := b.Header.Hash()
		u, err := s.UndoFor(h)
		if err != nil {
			t.Fatalf("H: in-session UndoFor(%s): %v", name, err)
		}
		enc, err := utxo.EncodeUndo(u)
		if err != nil {
			t.Fatalf("H: EncodeUndo(%s): %v", name, err)
		}
		wants = append(wants, wantUndo{name: name, path: path, idx: idx, hash: h, enc: enc})
	}
	// Index 0 (genesis) is excluded for the same reason as in G/2: the shared
	// root carries a fixture-seeded empty undo and is never reorg-produced.
	for i := 1; i < len(newPath); i++ {
		add(fmt.Sprintf("new[h=%d]", i), newPath, i)
	}
	for i := 1; i < len(oldPath); i++ {
		add(fmt.Sprintf("displaced[h=%d]", i), oldPath, i)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("H: close: %v", err)
	}

	// Two reopen cycles: distinguishes "in-memory existence" from "persisted
	// content" and additionally proves recovery idempotence for this path.
	for cycle := 1; cycle <= 2; cycle++ {
		sN := f1n1Store(t, dir)
		bcN := f1n1ChainFromStore(t, sN)

		// Canonical view reconstructed identically each time.
		if got := sN.RecoveryMode(); got != "REBUILD" {
			t.Fatalf("H[%d]: RecoveryMode=%q, want REBUILD", cycle, got)
		}
		if got := bcN.Height(); got != 3 {
			t.Fatalf("H[%d]: Height=%d, want 3", cycle, got)
		}
		if tip, has := sN.TipHash(); !has || tip != tp.f3.Header.Hash() {
			t.Fatalf("H[%d]: tip=%s has=%v, want %s/true", cycle, hx4(tip), has, hx4(tp.f3.Header.Hash()))
		}
		if got := sN.DanglingUndoCount(); got != 0 {
			t.Fatalf("H[%d]: DanglingUndoCount=%d, want 0", cycle, got)
		}

		// Content: every payload must come back byte-identical to the
		// pre-restart snapshot AND to the independently replayed oracle.
		for _, w := range wants {
			if !sN.HasUndo(w.hash) {
				t.Fatalf("H[%d]: HasUndo(%s)=false after restart", cycle, w.name)
			}
			got, err := sN.UndoFor(w.hash)
			if err != nil {
				t.Fatalf("H[%d]: UndoFor(%s) after restart: %v", cycle, w.name, err)
			}
			enc, err := utxo.EncodeUndo(got)
			if err != nil {
				t.Fatalf("H[%d]: EncodeUndo(%s): %v", cycle, w.name, err)
			}
			if !bytes.Equal(enc, w.enc) {
				t.Fatalf("H[%d]: %s payload changed across restart", cycle, w.name)
			}
			_, _, oracle := f1n2Replay(t, w.path, w.idx)
			ob, err := utxo.EncodeUndo(oracle)
			if err != nil {
				t.Fatalf("H[%d]: EncodeUndo(oracle %s): %v", cycle, w.name, err)
			}
			if !bytes.Equal(enc, ob) {
				t.Fatalf("H[%d]: %s payload != independent oracle after restart", cycle, w.name)
			}
		}

		// Canonical membership classification must be reproduced too.
		for _, b := range []*block.Block{tp.f1, tp.f2, tp.f3} {
			if !f1n1Canonical(t, sN, b.Header.Hash()) {
				t.Fatalf("H[%d]: %s lost canonical membership", cycle, hx4(b.Header.Hash()))
			}
		}
		for _, b := range []*block.Block{tp.c1, tp.c2} {
			if f1n1Canonical(t, sN, b.Header.Hash()) {
				t.Fatalf("H[%d]: displaced %s became canonical again", cycle, hx4(b.Header.Hash()))
			}
		}
		if err := sN.Close(); err != nil {
			t.Fatalf("H[%d]: close: %v", cycle, err)
		}
	}
}
