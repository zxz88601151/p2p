package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
)

func TestF3BExplicitInitializationStateMatrix(t *testing.T) {
	t.Run("empty node fails closed", func(t *testing.T) {
		dir := t.TempDir()
		_, err := newNodeRuntime(nodeConfig{DataDir: dir, ListenAddr: "127.0.0.1:0", RPCAddr: "127.0.0.1:0"})
		if !errors.Is(err, blockchain.ErrUninitializedStore) {
			t.Fatalf("new node error = %v, want ErrUninitializedStore", err)
		}
	})

	t.Run("empty verify fails closed", func(t *testing.T) {
		dir := t.TempDir()
		var out, errOut bytes.Buffer
		code := cmdVerify([]string{"-datadir", dir}, &out, &errOut)
		if code == 0 {
			t.Fatalf("verify exit code = 0, stderr=%q", errOut.String())
		}
	})

	t.Run("empty init creates canonical genesis", func(t *testing.T) {
		dir := t.TempDir()
		var out, errOut bytes.Buffer
		if code := cmdInit([]string{"-datadir", dir}, &out, &errOut); code != 0 {
			t.Fatalf("init exit code = %d stderr=%q", code, errOut.String())
		}
		store, err := storage.OpenFileBlockStoreReadOnly(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		genesis, err := blockchain.VerifyGenesisIdentity(store)
		if err != nil {
			t.Fatalf("verify initialized genesis: %v", err)
		}
		if genesis.Header.Hash() != blockchain.CanonicalGenesisHash {
			t.Fatalf("genesis hash = %s, want %s", genesis.Header.HashHex(), blockchain.CanonicalGenesisHashHex())
		}
	})

	t.Run("matching node and verify continue", func(t *testing.T) {
		dir := t.TempDir()
		initTestDir(t, dir)
		rt, err := newNodeRuntime(nodeConfig{DataDir: dir, ListenAddr: "127.0.0.1:0", RPCAddr: "127.0.0.1:0"})
		if err != nil {
			t.Fatalf("matching node startup: %v", err)
		}
		rt.Close()
		var out, errOut bytes.Buffer
		if code := cmdVerify([]string{"-datadir", dir}, &out, &errOut); code != 0 {
			t.Fatalf("matching verify exit code = %d stderr=%q stdout=%q", code, errOut.String(), out.String())
		}
	})

	t.Run("mismatch node and verify fail closed before replay", func(t *testing.T) {
		dir := t.TempDir()
		writeForeignGenesis(t, dir)
		_, err := newNodeRuntime(nodeConfig{DataDir: dir, ListenAddr: "127.0.0.1:0", RPCAddr: "127.0.0.1:0"})
		if !errors.Is(err, blockchain.ErrGenesisMismatch) {
			t.Fatalf("node mismatch error = %v, want ErrGenesisMismatch", err)
		}
		var out, errOut bytes.Buffer
		code := cmdVerify([]string{"-datadir", dir}, &out, &errOut)
		if code == 0 || !strings.Contains(out.String(), blockchain.ErrGenesisMismatch.Error()) {
			t.Fatalf("verify mismatch code=%d stderr=%q stdout=%q", code, errOut.String(), out.String())
		}
	})

	t.Run("corrupt and zero-byte datasets fail closed", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			data []byte
		}{
			{name: "corrupt", data: []byte("not-a-block-store")},
			{name: "zero-byte", data: nil},
		} {
			t.Run(tc.name, func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "blocks.dat")
				if err := os.WriteFile(path, tc.data, 0o600); err != nil {
					t.Fatal(err)
				}
				_, err := newNodeRuntime(nodeConfig{DataDir: dir, ListenAddr: "127.0.0.1:0", RPCAddr: "127.0.0.1:0"})
				if err == nil {
					t.Fatal("node unexpectedly started")
				}
				var out, errOut bytes.Buffer
				if code := cmdVerify([]string{"-datadir", dir}, &out, &errOut); code == 0 {
					t.Fatalf("verify unexpectedly passed: stdout=%q stderr=%q", out.String(), errOut.String())
				}
			})
		}
	})

	t.Run("init refuses initialized, foreign, and zero-byte datasets", func(t *testing.T) {
		initialized := t.TempDir()
		initTestDir(t, initialized)
		before := fileHash(t, filepath.Join(initialized, "blocks.dat"))
		var out, errOut bytes.Buffer
		if code := cmdInit([]string{"-datadir", initialized}, &out, &errOut); code == 0 {
			t.Fatal("init unexpectedly overwrote initialized dataset")
		}
		if got := fileHash(t, filepath.Join(initialized, "blocks.dat")); got != before {
			t.Fatal("initialized blocks.dat changed after refused init")
		}

		foreign := t.TempDir()
		writeForeignGenesis(t, foreign)
		before = fileHash(t, filepath.Join(foreign, "blocks.dat"))
		out.Reset()
		errOut.Reset()
		if code := cmdInit([]string{"-datadir", foreign}, &out, &errOut); code == 0 {
			t.Fatal("init unexpectedly overwrote foreign dataset")
		}
		if got := fileHash(t, filepath.Join(foreign, "blocks.dat")); got != before {
			t.Fatal("foreign blocks.dat changed after refused init")
		}

		zero := t.TempDir()
		zeroPath := filepath.Join(zero, "blocks.dat")
		if err := os.WriteFile(zeroPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if code := cmdInit([]string{"-datadir", zero}, &out, &errOut); code == 0 {
			t.Fatal("init unexpectedly treated zero-byte dataset as empty")
		}
		info, err := os.Stat(zeroPath)
		if err != nil || info.Size() != 0 {
			t.Fatalf("zero-byte dataset changed: info=%v err=%v", info, err)
		}
	})
}

func initTestDir(t *testing.T, dir string) {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := cmdInit([]string{"-datadir", dir}, &out, &errOut); code != 0 {
		t.Fatalf("init fixture code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

func writeForeignGenesis(t *testing.T, dir string) {
	t.Helper()
	store, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	foreign := block.NewCandidateBlock([32]byte{}, pow.MaxTargetBits, nil)
	foreign.Header.Timestamp++
	if err := store.SaveBlock(foreign); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func fileHash(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
