package blockchain

import (
	"errors"
	"fmt"

	"p2pchain/internal/block"
	"p2pchain/internal/storage"
)

var (
	// ErrUninitializedStore means the data directory has no persisted genesis yet.
	ErrUninitializedStore = errors.New("data directory is not initialized")

	// ErrGenesisMismatch is the identity failure for a dataset from another chain.
	ErrGenesisMismatch = errors.New("GENESIS_MISMATCH")

	// ErrCorruptGenesisIdentity means the persisted genesis cannot be read as identity.
	ErrCorruptGenesisIdentity = errors.New("genesis identity is corrupted")

	// ErrAlreadyInitialized prevents init from overwriting an existing dataset.
	ErrAlreadyInitialized = errors.New("data directory is already initialized")

	// ErrCanonicalGenesisDrift means the runtime genesis definition no longer matches
	// the pinned protocol identity. This is a release-blocking consensus change.
	ErrCanonicalGenesisDrift = errors.New("canonical genesis definition does not match pinned identity")
)

// VerifyGenesisIdentity is the single canonical identity gate for every persisted
// chain-construction path. It performs no consensus replay and never writes storage.
func VerifyGenesisIdentity(store storage.BlockStore) (*block.Block, error) {
	h, err := store.Height()
	if err != nil {
		return nil, fmt.Errorf("read storage height: %w", err)
	}
	if h < 0 {
		return nil, ErrUninitializedStore
	}

	genesis, err := store.GetBlockByHeight(0)
	if err != nil {
		return nil, fmt.Errorf("%w: read stored genesis: %v", ErrCorruptGenesisIdentity, err)
	}
	actual := genesis.Header.Hash()
	if actual != CanonicalGenesisHash {
		return genesis, fmt.Errorf("%w: expected %s, got %s", ErrGenesisMismatch, CanonicalGenesisHashHex(), genesis.Header.HashHex())
	}
	return genesis, nil
}

// NewBlockchainFromStoreForTest preserves the old test fixture convenience without
// reintroducing implicit initialization into production node startup. Production
// callers must use InitializeBlockchainStore explicitly before NewBlockchainFromStore.
func NewBlockchainFromStoreForTest(store storage.BlockStore) (*Blockchain, error) {
	h, err := store.Height()
	if err != nil {
		return nil, err
	}
	if h < 0 {
		if _, err := InitializeBlockchainStore(store); err != nil {
			return nil, err
		}
	}
	return NewBlockchainFromStore(store)
}

// InitializeBlockchainStore explicitly creates and persists the canonical genesis
// in an empty store. It never overwrites an existing or unreadable dataset.
func InitializeBlockchainStore(store storage.BlockStore) (*Blockchain, error) {
	h, err := store.Height()
	if err != nil {
		return nil, fmt.Errorf("read storage height: %w", err)
	}
	if h >= 0 {
		return nil, ErrAlreadyInitialized
	}

	genesis := NewGenesisBlock()
	if genesis.Header.Hash() != CanonicalGenesisHash {
		return nil, fmt.Errorf("%w: expected %s, generated %s", ErrCanonicalGenesisDrift, CanonicalGenesisHashHex(), genesis.Header.HashHex())
	}
	bc, err := NewBlockchainWithGenesis(genesis)
	if err != nil {
		return nil, fmt.Errorf("validate canonical genesis: %w", err)
	}
	if err := store.SaveBlock(genesis); err != nil {
		return nil, fmt.Errorf("persist canonical genesis: %w", err)
	}
	bc.store = store
	return bc, nil
}
