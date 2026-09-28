package blockchain

// ════════════════════════════════════════════════════════════════════════════
// PHASE P2PCHAIN — F1-N1
// UNDO CONTRACT TEST DESIGN / LEGACY-V2 REORG COVERAGE AUDIT
//
//   Test A — Legacy Reorg Baseline
//   Test B — V2 Reorg Baseline
//   Test C — Mixed Legacy → V2 Reorg   (highest priority)
//
// SCOPE DISCIPLINE
//   * test-only file. No production `.go`, storage primitive, reorg
//     implementation, PutUndo / interpretScan / CommitReorg change is made,
//     and UndoFor / DisconnectBlock are NOT wired into any consumer.
//   * every scenario runs on its own t.TempDir() — isolated storage only.
//   * production runtime and production storage are never touched.
//   * Test C deliberately EXERCISES the mixed topology instead of avoiding it
//     (phase spec INV-10); the pre-existing reorg fixture does the opposite —
//     see reorg_test.go:42 ("避免 legacy/v2 混用导致 reorg 失败").
// ════════════════════════════════════════════════════════════════════════════

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
)

// ── F1-N1 local helpers ─────────────────────────────────────────────────────

// hx4 renders the first 4 bytes of a hash for readable failure messages.
// It takes the hash BY VALUE on purpose: a method-call result (e.g.
// b.Header.Hash(), type [32]byte) is not addressable, so `b.Header.Hash()[:4]`
// does not compile. Callers must pass the value through this helper instead.
func hx4(h [32]byte) string { return hex.EncodeToString(h[:4]) }

// f1n1Store opens (or creates) the store in dir. Ownership of Close is the
// caller's, because the restart scenarios must close and reopen explicitly.
func f1n1Store(t *testing.T, dir string) *storage.FileBlockStore {
	t.Helper()
	s, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("F1N1: OpenFileBlockStore(%s): %v", dir, err)
	}
	return s
}

func f1n1ChainFromStore(t *testing.T, s *storage.FileBlockStore) *Blockchain {
	t.Helper()
	bc, err := NewBlockchainFromStoreForTest(s)
	if err != nil {
		t.Fatalf("F1N1: NewBlockchainFromStore: %v", err)
	}
	return bc
}

func f1n1Tip(t *testing.T, bc *Blockchain) [32]byte {
	t.Helper()
	tip, err := bc.Tip()
	if err != nil || tip == nil {
		t.Fatalf("F1N1: Tip(): err=%v", err)
	}
	return tip.Header.Hash()
}

func f1n1Canonical(t *testing.T, s *storage.FileBlockStore, hash [32]byte) bool {
	t.Helper()
	can, err := s.IsCanonical(hash)
	if err != nil {
		t.Fatalf("F1N1: IsCanonical(%x): %v", hash[:4], err)
	}
	return can
}

// f1n1Mine mines one coinbase-only block. pkh is derived from height+tag so that
// same-parent siblings are distinct (the C5 technique used elsewhere).
func f1n1Mine(t *testing.T, prev [32]byte, height int, tag byte) *block.Block {
	t.Helper()
	var pkh [20]byte
	pkh[0] = byte(height)
	pkh[1] = tag
	cb := transaction.NewCoinbaseTx(pkh, utxo.Subsidy(height), height)
	b := block.NewCandidateBlock(prev, pow.MaxTargetBits, []*transaction.Transaction{cb})
	if found, _ := pow.Mine(b); !found {
		t.Fatalf("F1N1: mine failed height=%d tag=%d", height, tag)
	}
	return b
}

// f1n1MineAbove mines siblings until one beats threshold on the byte-lexicographic
// (big-endian) comparison used by the deterministic tie-break. Harvesting an
// explicit ordering is what makes the fork-choice outcome of these tests
// deterministic rather than probabilistic.
func f1n1MineAbove(t *testing.T, prev [32]byte, height int, threshold [32]byte) *block.Block {
	t.Helper()
	for i := 0; i < 256; i++ {
		b := f1n1Mine(t, prev, height, byte(i))
		h := b.Header.Hash()
		if bytes.Compare(h[:], threshold[:]) > 0 {
			return b
		}
	}
	t.Fatalf("F1N1: no sibling above threshold after 256 attempts (height=%d)", height)
	return nil
}

// f1n1MineBelow is the mirror of f1n1MineAbove: it guarantees the sibling LOSES
// the work-tie, so the competing branch can be parked deterministically.
func f1n1MineBelow(t *testing.T, prev [32]byte, height int, threshold [32]byte) *block.Block {
	t.Helper()
	for i := 0; i < 256; i++ {
		b := f1n1Mine(t, prev, height, byte(0x80+i))
		h := b.Header.Hash()
		if bytes.Compare(h[:], threshold[:]) < 0 {
			return b
		}
	}
	t.Fatalf("F1N1: no sibling below threshold after 256 attempts (height=%d)", height)
	return nil
}

// f1n1UTXOSig returns a stable digest of the canonical UTXO set so that
// "state unchanged" can be asserted without depending on UTXO internals.
func f1n1UTXOSig(bc *Blockchain) string {
	entries := bc.UTXOSnapshot().AllEntries()
	lines := make([]string, 0, len(entries))
	for op, e := range entries {
		lines = append(lines, fmt.Sprintf("%s|%d|%x|%d|%v",
			op.String(), e.Value, e.PubKeyHash, e.Height, e.IsCoinbase))
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

// ════════════════════════════════════════════════════════════════════════════
// Test A — LEGACY REORG BASELINE
//
// Establishes the legacy-only semantics: canonical state, branch state, UTXO,
// block identity, tip, and post-restart state — plus the structural fact that a
// fork event transitions the store out of legacy purity, because parking a
// detached block writes a v2 BLOCK frame (SaveBlockDetached sets v2Mode).
// ════════════════════════════════════════════════════════════════════════════
func TestF1N1_A_LegacyReorgBaseline(t *testing.T) {
	dir := t.TempDir()

	// ── A1: fresh store → genesis is persisted through the LEGACY path ──
	s := f1n1Store(t, dir)
	bc := f1n1ChainFromStore(t, s)

	genHash := f1n1Tip(t, bc)
	if s.V2Mode() {
		t.Fatalf("A1: V2Mode=true after genesis, want false (legacy era)")
	}
	if got := s.LegacyRecordCount(); got != 1 {
		t.Fatalf("A1: LegacyRecordCount=%d, want 1", got)
	}
	if s.HasUndo(genHash) {
		t.Fatalf("A1: genesis carries an UNDO in the legacy era, want none")
	}
	if _, hasTip := s.TipHash(); hasTip {
		t.Fatalf("A1: TipHash hasTip=true, want false (no TIP frame written)")
	}
	if got := s.RecoveryMode(); got != "REBUILD" {
		t.Fatalf("A1: RecoveryMode=%q, want REBUILD", got)
	}

	// ── A2: legacy canonical extension ──
	l1 := f1n1Mine(t, genHash, 1, 0xA1)
	if err := bc.AddBlock(l1); err != nil {
		t.Fatalf("A2: AddBlock(L1): %v", err)
	}
	l2 := f1n1Mine(t, l1.Header.Hash(), 2, 0xA2)
	if err := bc.AddBlock(l2); err != nil {
		t.Fatalf("A2: AddBlock(L2): %v", err)
	}

	if s.V2Mode() {
		t.Fatalf("A2: V2Mode=true after two legacy appends, want false")
	}
	if got := s.LegacyRecordCount(); got != 3 {
		t.Fatalf("A2: LegacyRecordCount=%d, want 3", got)
	}
	if got := s.RecordCount(); got != 3 {
		t.Fatalf("A2: RecordCount=%d, want 3", got)
	}
	if got := s.DetachedCount(); got != 0 {
		t.Fatalf("A2: DetachedCount=%d, want 0", got)
	}

	// canonical state / tip / block identity
	if got := bc.Height(); got != 2 {
		t.Fatalf("A2: Height=%d, want 2", got)
	}
	if got := f1n1Tip(t, bc); got != l2.Header.Hash() {
		t.Fatalf("A2: tip=%x, want L2 %s", got[:4], hx4(l2.Header.Hash()))
	}
	for h, want := range map[int][32]byte{0: genHash, 1: l1.Header.Hash(), 2: l2.Header.Hash()} {
		got, err := bc.BlockByHeight(h)
		if err != nil {
			t.Fatalf("A2: BlockByHeight(%d): %v", h, err)
		}
		if got.Header.Hash() != want {
			t.Fatalf("A2: block identity at h=%d mismatch", h)
		}
		if s.HasUndo(want) {
			t.Fatalf("A2: legacy block h=%d carries an UNDO, want none", h)
		}
		if !f1n1Canonical(t, s, want) {
			t.Fatalf("A2: legacy block h=%d is not canonical", h)
		}
	}

	// resulting UTXO: three coinbase outputs, all immature-but-present
	if n := bc.UTXOSnapshot().Len(); n != 3 {
		t.Fatalf("A2: UTXOSet len=%d, want 3 (one coinbase per height)", n)
	}
	preUTXO := f1n1UTXOSig(bc)

	// ── A3: restart preserves canonical state and UTXO (INV-4) ──
	if err := s.Close(); err != nil {
		t.Fatalf("A3: close: %v", err)
	}
	s2 := f1n1Store(t, dir)
	defer func() { _ = s2.Close() }()
	bc2 := f1n1ChainFromStore(t, s2)

	if got := bc2.Height(); got != 2 {
		t.Fatalf("A3: Height=%d after restart, want 2", got)
	}
	if got := f1n1Tip(t, bc2); got != l2.Header.Hash() {
		t.Fatalf("A3: tip=%x after restart, want L2", got[:4])
	}
	if got := s2.RecoveryMode(); got != "REBUILD" {
		t.Fatalf("A3: RecoveryMode=%q after restart, want REBUILD", got)
	}
	if s2.V2Mode() {
		t.Fatalf("A3: V2Mode=true after restart, want false (still a legacy-only file)")
	}
	if got := s2.LegacyRecordCount(); got != 3 {
		t.Fatalf("A3: LegacyRecordCount=%d after restart, want 3", got)
	}
	if _, hasTip := s2.TipHash(); hasTip {
		t.Fatalf("A3: hasTip=true after restart, want false")
	}
	if got := f1n1UTXOSig(bc2); got != preUTXO {
		t.Fatalf("A3/INV-4: UTXO changed across restart")
	}

	// ── A4: a fork/LOSING sibling event activates v2 storage ──
	// Any fork block must be parked (SaveBlockDetached) before fork choice can
	// be evaluated, and parking writes a v2 BLOCK frame. Therefore a "pure
	// legacy reorg persistence" does not exist as a reachable state: the first
	// fork observed by a legacy-only node leaves legacy purity behind.
	loser := f1n1MineBelow(t, genHash, 1, l1.Header.Hash())
	if err := bc2.AddBlock(loser); err != nil {
		t.Fatalf("A4: AddBlock(losing sibling): %v", err)
	}
	if !s2.V2Mode() {
		t.Fatalf("A4: V2Mode=false after a fork was parked, want true")
	}
	if s2.HasUndo(loser.Header.Hash()) {
		t.Fatalf("A4: parked fork block carries an UNDO, want none")
	}
	if got := s2.DetachedCount(); got != 1 {
		t.Fatalf("A4: DetachedCount=%d, want 1", got)
	}
	if got := bc2.Height(); got != 2 {
		t.Fatalf("A4: Height=%d, want 2 (canonical must not move)", got)
	}
	if got := f1n1Tip(t, bc2); got != l2.Header.Hash() {
		t.Fatalf("A4: canonical tip moved to %x, want L2 unchanged", got[:4])
	}
	if can := f1n1Canonical(t, s2, loser.Header.Hash()); can {
		t.Fatalf("A4: losing sibling became canonical")
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Test B — V2 REORG BASELINE
//
// v2-only control: canonical records, tip, UTXO, undo index, restart, recovery.
// This is the negative control that isolates "legacy" as the discriminating
// variable for the Test C refusal.
// ════════════════════════════════════════════════════════════════════════════
func TestF1N1_B_V2ReorgBaseline(t *testing.T) {
	dir := t.TempDir()
	s := f1n1Store(t, dir)

	// ── B1: genesis written through the V2 path → undo + TIP present ──
	genesis := NewGenesisBlock()
	if err := s.AppendCanonicalBlock(genesis, utxo.BlockUndo{Height: 0}); err != nil {
		t.Fatalf("B1: AppendCanonicalBlock(genesis): %v", err)
	}
	genHash := genesis.Header.Hash()

	if !s.V2Mode() {
		t.Fatalf("B1: V2Mode=false, want true")
	}
	if got := s.LegacyRecordCount(); got != 0 {
		t.Fatalf("B1: LegacyRecordCount=%d, want 0 (v2-only)", got)
	}
	if !s.HasUndo(genHash) {
		t.Fatalf("B1: genesis has no UNDO, want one (AppendCanonicalBlock writes it)")
	}
	if can := f1n1Canonical(t, s, genHash); !can {
		t.Fatalf("B1: genesis is not canonical")
	}

	bc := f1n1ChainFromStore(t, s)
	if got := bc.Height(); got != 0 {
		t.Fatalf("B1: Height=%d, want 0", got)
	}
	if !s.HasUndo(genHash) {
		t.Fatalf("B1: undo index lost across the load")
	}

	// ── B2: canonical v2 extension carries UNDO ──
	v1 := f1n1Mine(t, genHash, 1, 0xB1)
	if err := bc.AddBlock(v1); err != nil {
		t.Fatalf("B2: AddBlock(V1): %v", err)
	}
	v2b := f1n1Mine(t, v1.Header.Hash(), 2, 0xB2)
	if err := bc.AddBlock(v2b); err != nil {
		t.Fatalf("B2: AddBlock(V2): %v", err)
	}

	for i, b := range []*block.Block{v1, v2b} {
		h := b.Header.Hash()
		if !s.HasUndo(h) {
			t.Fatalf("B2: v2 block %x has no UNDO", h[:4])
		}
		u, err := s.UndoFor(h)
		if err != nil {
			t.Fatalf("B2: UndoFor(%x): %v", h[:4], err)
		}
		// undo.Height is bound to the derived block height (1, then 2).
		if want := i + 1; u.Height != want {
			t.Fatalf("B2: undo height=%d, want %d", u.Height, want)
		}
		if can := f1n1Canonical(t, s, h); !can {
			t.Fatalf("B2: v2 block %x is not canonical", h[:4])
		}
	}
	if got := s.LegacyRecordCount(); got != 0 {
		t.Fatalf("B2: LegacyRecordCount=%d, want 0", got)
	}
	if got := s.RecordCount(); got != 3 {
		t.Fatalf("B2: RecordCount=%d, want 3", got)
	}

	// ── B3: strictly heavier fork → reorg must SUCCEED (v2-only control) ──
	f1 := f1n1Mine(t, genHash, 1, 0xB5)
	f2 := f1n1Mine(t, f1.Header.Hash(), 2, 0xB6)
	f3 := f1n1Mine(t, f2.Header.Hash(), 3, 0xB7)
	for _, b := range []*block.Block{f1, f2, f3} {
		if err := bc.AddBlock(b); err != nil {
			t.Fatalf("B3: AddBlock(fork %s): %v", hx4(b.Header.Hash()), err)
		}
	}

	if got := bc.Height(); got != 3 {
		t.Fatalf("B3: Height=%d after reorg, want 3", got)
	}
	if got := f1n1Tip(t, bc); got != f3.Header.Hash() {
		t.Fatalf("B3: tip=%x, want F3 %s (heavier branch must win)", got[:4], hx4(f3.Header.Hash()))
	}
	for _, b := range []*block.Block{f1, f2, f3} {
		h := b.Header.Hash()
		if !s.HasUndo(h) {
			t.Fatalf("B3: reorg did not persist an UNDO for %x", h[:4])
		}
		if can := f1n1Canonical(t, s, h); !can {
			t.Fatalf("B3: %x is not canonical after the reorg", h[:4])
		}
	}
	// VERIFIED CONTRACT (storage/v2api.go:692-703 + storage/v2.go:674):
	// the reorg displaced v1 and v2b from the canonical path. `setCanonicalFrom`
	// flips their `canonical` flag to false but does NOT erase them, and
	// DetachedCount() counts exactly `isV2 && !canonical`. Both displaced blocks
	// are v2 (LegacyRecordCount==0 above), so the retained detached set is 2 —
	// the old chain is preserved for a possible future re-org back.
	if got := s.DetachedCount(); got != 2 {
		t.Fatalf("B3: DetachedCount=%d after the reorg, want 2 (displaced v1,v2b retained)", got)
	}

	// ── B4: restart + recovery ──
	preUTXO := f1n1UTXOSig(bc)
	if err := s.Close(); err != nil {
		t.Fatalf("B4: close: %v", err)
	}
	s2 := f1n1Store(t, dir)
	defer func() { _ = s2.Close() }()
	bc2 := f1n1ChainFromStore(t, s2)

	if got := s2.RecoveryMode(); got != "REBUILD" {
		t.Fatalf("B4: RecoveryMode=%q, want REBUILD (last TIP is itself valid)", got)
	}
	tip, hasTip := s2.TipHash()
	if !hasTip {
		t.Fatalf("B4: hasTip=false, want true (v2 store has TIP frames)")
	}
	if tip != f3.Header.Hash() {
		t.Fatalf("B4: persisted tip=%x, want F3 %s", tip[:4], hx4(f3.Header.Hash()))
	}
	if got := bc2.Height(); got != 3 {
		t.Fatalf("B4: Height=%d after restart, want 3", got)
	}
	if got := f1n1Tip(t, bc2); got != f3.Header.Hash() {
		t.Fatalf("B4: replayed tip=%x, want F3", got[:4])
	}
	if !s2.HasUndo(f3.Header.Hash()) {
		t.Fatalf("B4: undo index not restored for the canonical tip")
	}
	if _, err := s2.UndoFor(f3.Header.Hash()); err != nil {
		t.Fatalf("B4: UndoFor(tip) after restart: %v", err)
	}
	if got := f1n1UTXOSig(bc2); got != preUTXO {
		t.Fatalf("B4/INV-4: UTXO changed across restart")
	}
	if got := s2.LegacyRecordCount(); got != 0 {
		t.Fatalf("B4: LegacyRecordCount=%d after restart, want 0", got)
	}
	if got := s2.DanglingUndoCount(); got != 0 {
		t.Fatalf("B4: DanglingUndoCount=%d, want 0", got)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Test C — MIXED LEGACY → V2 REORG  (highest priority)
//
// Topology, built entirely through production entry points:
//
//	Phase 1  legacy canonical h0 → h1 → h2            (SaveBlock, no UNDO)
//	Phase 2  sibling displacement puts W2 in place of h2
//	           → parking W2 activates v2 storage
//	           → the reorg succeeds because W2 is a v2 record
//	Phase 3  canonical v2 extension  →  canonical = [L0, L1, W2, V3]
//	           i.e. legacy, legacy, v2, v2  — the shape required by the spec
//	Phase 4  competing branch L2 → C3 → C4; L2 is the DISPLACED LEGACY RECORD,
//	         so it re-enters connectPath and CommitReorg Phase-1 refuses to
//	         write an UNDO for it  →  ErrLegacyImmutable
//	Phase 5  restart recovery, then a normal-extension control
//
// NOTE: this test asserts the CURRENT, DOCUMENTED behaviour (F1-F7 / F1-F1) and
// deliberately does not avoid the mixed topology. Per the phase spec this is an
// EXISTING-BEHAVIOR record, not a new defect.
// ════════════════════════════════════════════════════════════════════════════
func TestF1N1_C_MixedLegacyV2Reorg(t *testing.T) {
	dir := t.TempDir()
	s := f1n1Store(t, dir)
	bc := f1n1ChainFromStore(t, s)

	genHash := f1n1Tip(t, bc)

	// ── Phase 1: legacy prefix ──
	l1 := f1n1Mine(t, genHash, 1, 0xC1)
	if err := bc.AddBlock(l1); err != nil {
		t.Fatalf("C/1: AddBlock(L1): %v", err)
	}
	l2 := f1n1Mine(t, l1.Header.Hash(), 2, 0xC2)
	if err := bc.AddBlock(l2); err != nil {
		t.Fatalf("C/1: AddBlock(L2): %v", err)
	}
	if s.V2Mode() {
		t.Fatalf("C/1: V2Mode=true during the legacy phase, want false")
	}
	if got := s.LegacyRecordCount(); got != 3 {
		t.Fatalf("C/1: LegacyRecordCount=%d, want 3", got)
	}
	if got := bc.Height(); got != 2 {
		t.Fatalf("C/1: Height=%d, want 2", got)
	}

	// ── Phase 2: displace L2 with the same-work sibling W2 (tie-break wins) ──
	w2 := f1n1MineAbove(t, l1.Header.Hash(), 2, l2.Header.Hash())
	if err := bc.AddBlock(w2); err != nil {
		t.Fatalf("C/2: AddBlock(W2): %v", err)
	}
	if got := f1n1Tip(t, bc); got != w2.Header.Hash() {
		t.Fatalf("C/2: tie-break did not adopt W2 (tip=%x)", got[:4])
	}
	if !s.V2Mode() {
		t.Fatalf("C/2: V2Mode=false after sibling displacement, want true")
	}
	if got := s.LegacyRecordCount(); got != 3 {
		t.Fatalf("C/2: LegacyRecordCount=%d, want 3 (L2 remains a legacy record)", got)
	}
	if s.HasUndo(l2.Header.Hash()) {
		t.Fatalf("C/2: displaced legacy record carries an UNDO, want none")
	}
	if can := f1n1Canonical(t, s, l2.Header.Hash()); can {
		t.Fatalf("C/2: L2 still canonical after displacement")
	}
	if !s.HasUndo(w2.Header.Hash()) {
		t.Fatalf("C/2: W2 has no UNDO — the v2 reorg should have persisted one")
	}

	// ── Phase 3: canonical v2 extension → mixed canonical chain ──
	v3 := f1n1Mine(t, w2.Header.Hash(), 3, 0xC3)
	if err := bc.AddBlock(v3); err != nil {
		t.Fatalf("C/3: AddBlock(V3): %v", err)
	}
	if got := bc.Height(); got != 3 {
		t.Fatalf("C/3: Height=%d, want 3", got)
	}
	if got := f1n1Tip(t, bc); got != v3.Header.Hash() {
		t.Fatalf("C/3: tip=%x, want V3", got[:4])
	}

	// Mixed topology ESTABLISHED: legacy h0,h1 + v2 h2,h3, plus one detached
	// legacy record (L2). This is precisely what the pre-existing reorg fixture
	// refuses to construct (reorg_test.go:42).
	if !s.V2Mode() {
		t.Fatalf("C/3: V2Mode=false, want true (mixed topology)")
	}
	if got := s.LegacyRecordCount(); got != 3 {
		t.Fatalf("C/3: LegacyRecordCount=%d, want 3", got)
	}
	// VERIFIED CONTRACT (storage/v2api.go:692-703 + storage/v2.go:631,674):
	// DetachedCount() counts ONLY `isV2 && !canonical`. The block displaced here
	// is L2, a LEGACY record, so it can never be counted — legacy bytes are never
	// rewritten and exclusion of a legacy block from the canonical path is
	// expressed solely by flipping its canonical flag (asserted false below).
	// A non-zero expectation here would be a misreading of the API.
	if got := s.DetachedCount(); got != 0 {
		t.Fatalf("C/3: DetachedCount=%d, want 0 (legacy records are excluded by construction)", got)
	}
	for _, h := range []struct {
		name string
		hash [32]byte
		want bool
	}{
		{"L0(legacy)", genHash, true},
		{"L1(legacy)", l1.Header.Hash(), true},
		{"W2(v2)", w2.Header.Hash(), true},
		{"V3(v2)", v3.Header.Hash(), true},
		{"L2(legacy,displaced)", l2.Header.Hash(), false},
	} {
		if got := f1n1Canonical(t, s, h.hash); got != h.want {
			t.Fatalf("C/3: IsCanonical(%s)=%v, want %v", h.name, got, h.want)
		}
	}

	// ── Phase 4: re-attach the LEGACY branch ──
	// C3 is harvested BELOW V3's hash so it loses the work-tie and is merely
	// parked; the reorg then triggers deterministically on C4 (strictly heavier).
	logBefore := s.LogSize()

	c3 := f1n1MineBelow(t, l2.Header.Hash(), 3, v3.Header.Hash())
	if err := bc.AddBlock(c3); err != nil {
		t.Fatalf("C/4: AddBlock(C3): %v", err)
	}
	if got := f1n1Tip(t, bc); got != v3.Header.Hash() {
		t.Fatalf("C/4: C3 won the tie-break; the determinism assumption broke")
	}

	// Preconditions that place L2 into detachedUndos (blockchain.go:866-871):
	//   HasBlock(L2) && !HasUndo(L2), and L2 sits on the candidate branch
	//   strictly above the common ancestor.
	if !s.HasBlock(l2.Header.Hash()) {
		t.Fatalf("C/4 precondition: HasBlock(L2)=false, want true")
	}
	if s.HasUndo(l2.Header.Hash()) {
		t.Fatalf("C/4 precondition: HasUndo(L2)=true, want false")
	}

	c4 := f1n1Mine(t, c3.Header.Hash(), 4, 0xC4)
	err := bc.AddBlock(c4)

	// The branch is strictly heavier (4 > 3), so a reorg is required — but the
	// legacy record L2 on connectPath makes it impossible to persist.
	if err == nil {
		t.Fatalf("C/4: expected the legacy-record refusal, got nil (branch adopted?)")
	}
	if !errors.Is(err, storage.ErrLegacyImmutable) {
		t.Fatalf("C/4: err=%v, want ErrLegacyImmutable", err)
	}

	// Canonical state must be untouched: executeReorg returns before the
	// in-memory canonical replacement (blockchain.go:881-883 vs :906-913).
	if got := bc.Height(); got != 3 {
		t.Fatalf("C/4: Height=%d after the refused reorg, want 3", got)
	}
	if got := f1n1Tip(t, bc); got != v3.Header.Hash() {
		t.Fatalf("C/4: tip=%x after the refused reorg, want V3 unchanged", got[:4])
	}
	if !f1n1Canonical(t, s, v3.Header.Hash()) {
		t.Fatalf("C/4: V3 lost canonical membership")
	}
	if f1n1Canonical(t, s, l2.Header.Hash()) {
		t.Fatalf("C/4: L2 became canonical despite the refusal")
	}
	// The guard still refuses to persist an UNDO for the legacy record.
	if s.HasUndo(l2.Header.Hash()) {
		t.Fatalf("C/4: an UNDO was written for L2 despite the guard")
	}
	// The bytes appended are C3/C4 landing in storage as detached v2 blocks when
	// they were added to the tree (SaveBlockDetached) — NOT output of the reorg
	// itself. The refused reorg wrote nothing: its legacy guard sits in
	// CommitReorg Phase 1, which is pure validation with no side effects on the
	// v2 maps (v2api.go:318-333), and executeReorg returns at blockchain.go:882.
	if got := s.LogSize(); got <= logBefore {
		t.Fatalf("C/4: LogSize=%d did not grow past %d", got, logBefore)
	}
	for _, b := range []*block.Block{c3, c4} {
		h := b.Header.Hash()
		if !s.HasBlock(h) {
			t.Fatalf("C/4: parked block %x is not stored", h[:4])
		}
		if s.HasUndo(h) {
			t.Fatalf("C/4: parked block %x unexpectedly has an UNDO", h[:4])
		}
	}

	// ── Phase 5: restart recovery + invariant guards ──
	if err := s.Close(); err != nil {
		t.Fatalf("C/5: close: %v", err)
	}
	s2 := f1n1Store(t, dir)
	defer func() { _ = s2.Close() }()
	bc2 := f1n1ChainFromStore(t, s2)

	if got := s2.RecoveryMode(); got != "REBUILD" {
		t.Fatalf("C/5: RecoveryMode=%q, want REBUILD", got)
	}
	if got := bc2.Height(); got != 3 {
		t.Fatalf("C/5: Height=%d after restart, want 3 (INV-4)", got)
	}
	if got := f1n1Tip(t, bc2); got != v3.Header.Hash() {
		t.Fatalf("C/5: tip=%x after restart, want V3", got[:4])
	}
	if got := s2.LegacyRecordCount(); got != 3 {
		t.Fatalf("C/5: LegacyRecordCount=%d after restart, want 3", got)
	}
	if s2.HasUndo(l2.Header.Hash()) {
		t.Fatalf("C/5: L2 acquired an UNDO across restart")
	}
	if f1n1Canonical(t, s2, l2.Header.Hash()) {
		t.Fatalf("C/5: L2 is canonical after restart")
	}
	if got := s2.DanglingUndoCount(); got != 0 {
		t.Fatalf("C/5: DanglingUndoCount=%d, want 0", got)
	}

	// INV-10 guard: assert the mixed topology is genuinely present, i.e. this
	// test did not earn a PASS by quietly avoiding the legacy/v2 mix.
	if !(s2.V2Mode() && s2.LegacyRecordCount() > 0 && s2.DetachedCount() >= 1) {
		t.Fatalf("C/INV-10: mixed topology not present (v2=%v legacy=%d detached=%d)",
			s2.V2Mode(), s2.LegacyRecordCount(), s2.DetachedCount())
	}

	// Control: the canonical branch still extends normally after the refusal,
	// so the refusal is specific to the legacy record on the competing path and
	// is not general breakage. (The v2-only analogue that SUCCEEDS is Test B.)
	v4 := f1n1Mine(t, v3.Header.Hash(), 4, 0xC9)
	if err := bc2.AddBlock(v4); err != nil {
		t.Fatalf("C/control: canonical extension after the refusal failed: %v", err)
	}
	if got := f1n1Tip(t, bc2); got != v4.Header.Hash() {
		t.Fatalf("C/control: tip=%x, want V4", got[:4])
	}
	if !s2.HasUndo(v4.Header.Hash()) {
		t.Fatalf("C/control: V4 carries no UNDO")
	}
	fmt.Printf("F1N1-C: mixed legacy/v2 reorg refusal recorded "+
		"(legacy=L2 %s, connectPath=[L2,C3,C4], err=%v)\n", hx4(l2.Header.Hash()), err)
}
