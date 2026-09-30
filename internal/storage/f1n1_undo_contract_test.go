package storage

// ════════════════════════════════════════════════════════════════════════════
// PHASE P2PCHAIN — F1-N1
// UNDO CONTRACT TEST DESIGN / LEGACY-V2 REORG COVERAGE AUDIT
//
//   Test D — Legacy Undo Persistence / Restart Binding
//   Test E — Dangling / Orphan UNDO matrix (E1..E5)
//
// SCOPE DISCIPLINE
//   * test-only file. No production `.go`, storage primitive, PutUndo /
//     interpretScan / CommitReorg change is made.
//   * every scenario runs on its own t.TempDir() — isolated storage only.
//   * production runtime and production storage are never touched.
//
// LAYER SEPARATION (required by the phase spec §5 — "不要只检查文件里有字节")
//   L0  frame physically exists (raw bytes + canonical encoding)
//   L1  frame is parsed (the log loads; in-session accounting is exact)
//   L2  frame is bound (undoIndex carries the key)
//   L3  frame is / is not classified dangling (danglingUndo counter)
//   L4  frame is available through the public accessor (HasUndo / UndoFor)
//   L5  frame participates in recovery without disturbing canonical state
// ════════════════════════════════════════════════════════════════════════════

import (
	"bytes"
	"errors"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/utxo"
)

// ── F1-N1 local helpers ─────────────────────────────────────────────────────

// f1n1OpenRW opens an existing (or new) store in dir. The caller owns Close.
func f1n1OpenRW(t *testing.T, dir string) *FileBlockStore {
	t.Helper()
	s, err := OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("F1N1: OpenFileBlockStore(%s): %v", dir, err)
	}
	return s
}

// f1n1ReplayUndos replays blocks from genesis with the real ApplyBlockWithUndo
// and returns the per-height undo. This is a TEST-LAYER ORACLE only — no
// production consumer of undo is wired up (phase spec §7).
func f1n1ReplayUndos(t *testing.T, blocks []*block.Block) []utxo.BlockUndo {
	t.Helper()
	set := utxo.NewUTXOSet()
	undos := make([]utxo.BlockUndo, 0, len(blocks))
	for h, b := range blocks {
		ns, u, _, err := utxo.ApplyBlockWithUndo(set, b.Transactions, h)
		if err != nil {
			t.Fatalf("F1N1: ApplyBlockWithUndo h=%d: %v", h, err)
		}
		set = ns
		undos = append(undos, u)
	}
	return undos
}

// f1n1UndoBytes serialises an undo through the production encoder so content
// equality is byte-exact (deterministic big-endian encoding).
func f1n1UndoBytes(t *testing.T, u utxo.BlockUndo) []byte {
	t.Helper()
	b, err := utxo.EncodeUndo(u)
	if err != nil {
		t.Fatalf("F1N1: EncodeUndo: %v", err)
	}
	return b
}

// ════════════════════════════════════════════════════════════════════════════
// Test D — LEGACY UNDO PERSISTENCE / RESTART BINDING
//
// Builds a legacy-only store, appends an UNDO frame keyed by a LEGACY block
// hash through PutUndo (the R-C kernel), and then asserts the six layers above
// both in-session and across two restarts.
//
// This is the test that turns F1-F4 from a code reading into observed evidence.
// ════════════════════════════════════════════════════════════════════════════
func TestF1N1_D_LegacyUndoPersistenceRestartBinding(t *testing.T) {
	dir := t.TempDir()

	// ── Step 1: legacy-only store (legacy prefix h0..h2, written by SaveBlock) ──
	blocks := iSeedLegacy(t, dir, 3)
	legacyHash := blocks[2].Header.Hash()
	undos := f1n1ReplayUndos(t, blocks)

	beforeBytes := iReadLog(t, dir)
	legacyEnd := int64(len(beforeBytes))

	// ── Step 2: pre-conditions ──
	s := f1n1OpenRW(t, dir)
	if s.V2Mode() {
		t.Fatalf("D precondition: V2Mode=true, want false")
	}
	if got := s.LegacyRecordCount(); got != 3 {
		t.Fatalf("D precondition: LegacyRecordCount=%d, want 3", got)
	}
	if s.HasUndo(legacyHash) {
		t.Fatalf("D precondition: HasUndo(legacy)=true, want false")
	}
	if _, err := s.UndoFor(legacyHash); !errors.Is(err, ErrUndoNotFound) {
		t.Fatalf("D precondition: UndoFor(legacy) err=%v, want ErrUndoNotFound", err)
	}
	if got := s.LogSize(); got != legacyEnd {
		t.Fatalf("D precondition: LogSize=%d, len(log)=%d", got, legacyEnd)
	}
	if _, ok := s.v2.undoIndex[legacyHash]; ok {
		t.Fatalf("D precondition: undoIndex is already keyed by the legacy hash")
	}

	// ── Step 3: sidecar write — PutUndo on a LEGACY hash ──
	// No legacy byte is rewritten: the frame is appended at the log end.
	if err := s.PutUndo(legacyHash, 2, undos[2]); err != nil {
		t.Fatalf("D: PutUndo(legacy hash) failed: %v", err)
	}

	afterBytes := iReadLog(t, dir)

	// L0 — the frame physically exists and is keyed by the legacy block hash.
	if int64(len(afterBytes)) <= legacyEnd {
		t.Fatalf("D/L0: log did not grow (%d -> %d)", legacyEnd, len(afterBytes))
	}
	tail := afterBytes[legacyEnd:]
	fr, err := decodeFrame(tail)
	if err != nil {
		t.Fatalf("D/L0: decodeFrame(tail): %v", err)
	}
	if fr.Type != recTypeUndo {
		t.Fatalf("D/L0: tail frame type=%s, want %s", recTypeName(fr.Type), recTypeName(recTypeUndo))
	}
	if fr.BlockHash != legacyHash {
		t.Fatalf("D/L0: tail frame keyed by %x, want legacy %x", fr.BlockHash[:4], legacyHash[:4])
	}
	// L0b — byte-identical to a freshly encoded frame.
	if want := encodeFrame(recTypeUndo, 2, legacyHash, f1n1UndoBytes(t, undos[2])); !bytes.Equal(tail, want) {
		t.Fatalf("D/L0b: appended UNDO frame differs from the canonical encoding")
	}
	// Legacy region untouched (append-only boundary).
	if !bytes.Equal(afterBytes[:legacyEnd], beforeBytes) {
		t.Fatalf("D: legacy region bytes changed after PutUndo")
	}

	// L1 — parsed: the in-session accounting matches the file length.
	if got := s.LogSize(); got != int64(len(afterBytes)) {
		t.Fatalf("D/L1: LogSize=%d, len(log)=%d", got, len(afterBytes))
	}
	// L2 — bound into the live undo index.
	if _, ok := s.v2.undoIndex[legacyHash]; !ok {
		t.Fatalf("D/L2: undoIndex not keyed by the legacy hash after PutUndo")
	}
	// L3 — explicitly NOT dangling (PutUndo is an in-session bind, not a scan).
	if got := s.v2.danglingUndo; got != 0 {
		t.Fatalf("D/L3: danglingUndo=%d in session, want 0", got)
	}
	// L4 — available through the public accessor.
	if !s.HasUndo(legacyHash) {
		t.Fatalf("D/L4: HasUndo(legacy)=false after PutUndo")
	}
	gotU, err := s.UndoFor(legacyHash)
	if err != nil {
		t.Fatalf("D/L4: UndoFor(legacy) failed: %v", err)
	}
	if !bytes.Equal(f1n1UndoBytes(t, gotU), f1n1UndoBytes(t, undos[2])) {
		t.Fatalf("D/L4: UndoFor(legacy) content differs from the oracle undo")
	}
	if got := s.LegacyRecordCount(); got != 3 {
		t.Fatalf("D: LegacyRecordCount=%d after PutUndo, want 3", got)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("D: close s: %v", err)
	}

	// ── Step 4: RESTART — the F1-F4 core assertion ──
	s2 := f1n1OpenRW(t, dir)

	// L2/L4 survive: the second pass of interpretScan binds an UNDO frame
	// whenever its key is present in `records` — and a LEGACY record is present
	// there, so the frame is bound rather than discarded as dangling.
	if !s2.HasUndo(legacyHash) {
		t.Fatalf("D/F1-F4 VIOLATED: HasUndo(legacy)=false after restart")
	}
	gotU2, err := s2.UndoFor(legacyHash)
	if err != nil {
		t.Fatalf("D/F1-F4: UndoFor(legacy) failed after restart: %v", err)
	}
	if !bytes.Equal(f1n1UndoBytes(t, gotU2), f1n1UndoBytes(t, undos[2])) {
		t.Fatalf("D/F1-F4: undo content not preserved across restart")
	}
	// L3 after restart — must not be classified dangling.
	if got := s2.DanglingUndoCount(); got != 0 {
		t.Fatalf("D/F1-F4 VIOLATED: DanglingUndoCount=%d after restart, want 0", got)
	}
	// L5 — recovery succeeds and canonical state is unchanged (INV-4).
	if got := s2.RecoveryMode(); got != "REBUILD" {
		t.Fatalf("D/L5: RecoveryMode=%q, want REBUILD", got)
	}
	if h, err := s2.Height(); err != nil || h != 2 {
		t.Fatalf("D/L5: canonical Height=%d err=%v after restart, want 2", h, err)
	}
	if _, hasTip := s2.TipHash(); hasTip {
		t.Fatalf("D/L5: TipHash reports hasTip=true, want false (PutUndo writes no TIP)")
	}
	if got := s2.LegacyRecordCount(); got != 3 {
		t.Fatalf("D/L5: LegacyRecordCount=%d after restart, want 3", got)
	}
	bytesRestart := iReadLog(t, dir)
	if !bytes.Equal(bytesRestart[:legacyEnd], beforeBytes) {
		t.Fatalf("D/L5: legacy region bytes changed across restart")
	}
	if err := s2.Close(); err != nil {
		t.Fatalf("D: close s2: %v", err)
	}

	// ── Step 5: repeated restart idempotence (INV-9) ──
	s3 := f1n1OpenRW(t, dir)
	defer func() { _ = s3.Close() }()
	if !s3.HasUndo(legacyHash) {
		t.Fatalf("D/INV-9: HasUndo(legacy)=false after the 2nd restart")
	}
	if got := s3.DanglingUndoCount(); got != 0 {
		t.Fatalf("D/INV-9: DanglingUndoCount=%d after the 2nd restart", got)
	}
	if got := s3.LegacyRecordCount(); got != 3 {
		t.Fatalf("D/INV-9: LegacyRecordCount=%d after the 2nd restart", got)
	}
	if got := s3.LogSize(); got != int64(len(bytesRestart)) {
		t.Fatalf("D/INV-9: LogSize mutated on repeated restart (%d -> %d)", len(bytesRestart), got)
	}
	if h, err := s3.Height(); err != nil || h != 2 {
		t.Fatalf("D/INV-9: Height=%d err=%v after the 2nd restart", h, err)
	}
	if _, err := s3.UndoFor(legacyHash); err != nil {
		t.Fatalf("D/INV-9: UndoFor(legacy) failed after the 2nd restart: %v", err)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Test E — DANGLING / ORPHAN UNDO MATRIX
//
//	E1  UNDO keyed by a hash with NO block record            → dangling
//	E2  UNDO keyed by an existing but NON-CANONICAL block    → bound, inert
//	E3  UNDO keyed by a LEGACY record                        → bound, inert
//	E4  UNDO keyed by a v2 record                            → bound, canonical
//	E5  every case re-checked after restart
//
// For each case we record: scanned / bound / dangling / survives restart /
// affects canonical state.
// ════════════════════════════════════════════════════════════════════════════
func TestF1N1_E_DanglingOrphanUndoMatrix(t *testing.T) {
	// ── E1: UNDO keyed by an UNKNOWN block hash ──
	t.Run("E1_unknown_block_hash", func(t *testing.T) {
		dir := t.TempDir()
		blocks := iSeedLegacy(t, dir, 2)
		undos := f1n1ReplayUndos(t, blocks)
		base := iReadLog(t, dir)

		var unknown [32]byte
		copy(unknown[:], bytes.Repeat([]byte{0xEE}, 32))

		injected := append(append([]byte(nil), base...),
			encodeFrame(recTypeUndo, 5, unknown, f1n1UndoBytes(t, undos[1]))...)
		iWriteLog(t, dir, injected)

		// Control: the same legacy prefix without the injected frame.
		ctrlDir := t.TempDir()
		iWriteLog(t, ctrlDir, base)
		ctrl := f1n1OpenRW(t, ctrlDir)
		ctrlHeight, ctrlErr := ctrl.Height()
		if ctrlErr != nil {
			t.Fatalf("E1: control Height: %v", ctrlErr)
		}
		ctrlTip, ctrlHasTip := ctrl.TipHash()
		ctrlMode := ctrl.RecoveryMode()
		ctrlLegacy := ctrl.LegacyRecordCount()
		if err := ctrl.Close(); err != nil {
			t.Fatalf("E1: control close: %v", err)
		}

		s := f1n1OpenRW(t, dir)
		// scanned: the log loads at all (a corrupt frame would have failed here)
		if s.HasBlock(unknown) {
			t.Fatalf("E1: HasBlock(unknown)=true, want false")
		}
		// NOT bound
		if s.HasUndo(unknown) {
			t.Fatalf("E1: HasUndo(unknown)=true — an unknown-key UNDO must NOT be bound")
		}
		// classified dangling
		if got := s.DanglingUndoCount(); got != 1 {
			t.Fatalf("E1: DanglingUndoCount=%d, want 1", got)
		}
		// INV-3: a dangling UNDO must not change canonical state.
		h, err := s.Height()
		if err != nil {
			t.Fatalf("E1: Height: %v", err)
		}
		tip, hasTip := s.TipHash()
		if h != ctrlHeight || hasTip != ctrlHasTip || tip != ctrlTip {
			t.Fatalf("E1/INV-3: canonical drifted (h=%d/%d tip=%x/%x hasTip=%v/%v)",
				h, ctrlHeight, tip[:4], ctrlTip[:4], hasTip, ctrlHasTip)
		}
		if got := s.RecoveryMode(); got != ctrlMode {
			t.Fatalf("E1: RecoveryMode=%q, control=%q", got, ctrlMode)
		}
		if got := s.LegacyRecordCount(); got != ctrlLegacy {
			t.Fatalf("E1: LegacyRecordCount=%d, control=%d", got, ctrlLegacy)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("E1: close: %v", err)
		}

		// ── E5 (E1): re-check after restart ──
		s2 := f1n1OpenRW(t, dir)
		defer func() { _ = s2.Close() }()
		if s2.HasUndo(unknown) {
			t.Fatalf("E5/E1: HasUndo(unknown)=true after restart")
		}
		if got := s2.DanglingUndoCount(); got != 1 {
			t.Fatalf("E5/E1: DanglingUndoCount=%d after restart, want 1 (recomputed on each scan)", got)
		}
		if h2, err := s2.Height(); err != nil || h2 != ctrlHeight {
			t.Fatalf("E5/E1: Height=%d err=%v, want %d", h2, err, ctrlHeight)
		}
		if tip2, hasTip2 := s2.TipHash(); hasTip2 != ctrlHasTip || tip2 != ctrlTip {
			t.Fatalf("E5/E1: canonical tip drifted after restart")
		}
	})

	// ── E2: UNDO keyed by an existing but NON-CANONICAL block ──
	t.Run("E2_non_canonical_block", func(t *testing.T) {
		dir := t.TempDir()
		blocks := iSeedLegacy(t, dir, 2) // legacy h0, h1

		// A detached v2 fork block: parent = h1, height 2, never canonical.
		fork := iBlock(t, blocks[1].Header.Hash(), 2, 0x77)
		forkHash := fork.Header.Hash()
		forkUndo := f1n1ReplayUndos(t,
			append(append([]*block.Block(nil), blocks...), fork))[2]

		s := f1n1OpenRW(t, dir)
		if err := s.SaveBlockDetached(fork); err != nil {
			t.Fatalf("E2: SaveBlockDetached: %v", err)
		}
		if can, err := s.IsCanonical(forkHash); err != nil || can {
			t.Fatalf("E2 precondition: IsCanonical(fork)=%v err=%v, want false", can, err)
		}
		if err := s.PutUndo(forkHash, 2, forkUndo); err != nil {
			t.Fatalf("E2: PutUndo(detached v2 hash): %v", err)
		}
		if !s.HasUndo(forkHash) {
			t.Fatalf("E2: HasUndo=false after PutUndo, want true (record exists)")
		}
		if got := s.DanglingUndoCount(); got != 0 {
			t.Fatalf("E2: DanglingUndoCount=%d, want 0 (the record exists)", got)
		}
		// INV-3: binding an UNDO must not promote the block to canonical.
		if can, err := s.IsCanonical(forkHash); err != nil || can {
			t.Fatalf("E2/INV-3: IsCanonical(fork)=%v, want false", can)
		}
		if h, err := s.Height(); err != nil || h != 1 {
			t.Fatalf("E2/INV-3: Height=%d err=%v, want 1", h, err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("E2: close: %v", err)
		}

		// ── E5 (E2) ──
		s2 := f1n1OpenRW(t, dir)
		defer func() { _ = s2.Close() }()
		if !s2.HasUndo(forkHash) {
			t.Fatalf("E5/E2: HasUndo=false after restart, want true")
		}
		if got := s2.DanglingUndoCount(); got != 0 {
			t.Fatalf("E5/E2: DanglingUndoCount=%d after restart, want 0", got)
		}
		if can, err := s2.IsCanonical(forkHash); err != nil || can {
			t.Fatalf("E5/E2: detached fork became canonical (can=%v err=%v)", can, err)
		}
		if h, err := s2.Height(); err != nil || h != 1 {
			t.Fatalf("E5/E2: Height=%d err=%v after restart, want 1", h, err)
		}
	})

	// ── E3: UNDO keyed by a LEGACY record ──
	t.Run("E3_legacy_record", func(t *testing.T) {
		dir := t.TempDir()
		blocks := iSeedLegacy(t, dir, 3)
		undos := f1n1ReplayUndos(t, blocks)
		h2 := blocks[2].Header.Hash()

		s := f1n1OpenRW(t, dir)
		if err := s.PutUndo(h2, 2, undos[2]); err != nil {
			t.Fatalf("E3: PutUndo(legacy): %v", err)
		}
		if !s.HasUndo(h2) {
			t.Fatalf("E3: not bound")
		}
		if got := s.DanglingUndoCount(); got != 0 {
			t.Fatalf("E3: DanglingUndoCount=%d, want 0", got)
		}
		if can, err := s.IsCanonical(h2); err != nil || !can {
			t.Fatalf("E3: canonical membership changed (can=%v err=%v)", can, err)
		}
		if h, err := s.Height(); err != nil || h != 2 {
			t.Fatalf("E3: Height=%d err=%v, want 2", h, err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("E3: close: %v", err)
		}

		// ── E5 (E3) ──
		s2 := f1n1OpenRW(t, dir)
		defer func() { _ = s2.Close() }()
		if !s2.HasUndo(h2) {
			t.Fatalf("E5/E3: not bound after restart")
		}
		if got := s2.DanglingUndoCount(); got != 0 {
			t.Fatalf("E5/E3: DanglingUndoCount=%d after restart, want 0", got)
		}
		if can, err := s2.IsCanonical(h2); err != nil || !can {
			t.Fatalf("E5/E3: canonical membership lost")
		}
	})

	// ── E4: UNDO keyed by a v2 record ──
	t.Run("E4_v2_record", func(t *testing.T) {
		dir := t.TempDir()
		gen := iSeedLegacy(t, dir, 1) // legacy h0 only

		b1 := iBlock(t, gen[0].Header.Hash(), 1, 0x88)
		b1Hash := b1.Header.Hash()
		u := f1n1ReplayUndos(t, []*block.Block{gen[0], b1})

		s := f1n1OpenRW(t, dir)
		if err := s.AppendCanonicalBlock(b1, u[1]); err != nil {
			t.Fatalf("E4: AppendCanonicalBlock: %v", err)
		}
		if !s.HasUndo(b1Hash) {
			t.Fatalf("E4: v2 block has no UNDO")
		}
		if got := s.DanglingUndoCount(); got != 0 {
			t.Fatalf("E4: DanglingUndoCount=%d, want 0", got)
		}
		if can, err := s.IsCanonical(b1Hash); err != nil || !can {
			t.Fatalf("E4: v2 block not canonical (can=%v err=%v)", can, err)
		}
		if tip, hasTip := s.TipHash(); !hasTip || tip != b1Hash {
			t.Fatalf("E4: TipHash=%x/%v, want %x/true", tip[:4], hasTip, b1Hash[:4])
		}
		if h, err := s.Height(); err != nil || h != 1 {
			t.Fatalf("E4: Height=%d err=%v, want 1", h, err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("E4: close: %v", err)
		}

		// ── E5 (E4) ──
		s2 := f1n1OpenRW(t, dir)
		defer func() { _ = s2.Close() }()
		if !s2.HasUndo(b1Hash) {
			t.Fatalf("E5/E4: not bound after restart")
		}
		if got := s2.DanglingUndoCount(); got != 0 {
			t.Fatalf("E5/E4: DanglingUndoCount=%d after restart, want 0", got)
		}
		if tip, hasTip := s2.TipHash(); !hasTip || tip != b1Hash {
			t.Fatalf("E5/E4: TIP lost across restart")
		}
		if h, err := s2.Height(); err != nil || h != 1 {
			t.Fatalf("E5/E4: Height=%d err=%v after restart, want 1", h, err)
		}
		if _, err := s2.UndoFor(b1Hash); err != nil {
			t.Fatalf("E5/E4: UndoFor after restart: %v", err)
		}
	})
}
