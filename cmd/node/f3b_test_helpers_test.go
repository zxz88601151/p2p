package main

import (
	"errors"

	"p2pchain/internal/blockchain"
	"p2pchain/internal/storage"
)

// newNodeRuntimeForTest makes fixture setup explicit for legacy tests that
// historically relied on node startup to create genesis implicitly. Production
// code never calls this helper; F-3B tests the real newNodeRuntime separately.
func newNodeRuntimeForTest(cfg nodeConfig) (*nodeRuntime, error) {
	store, err := storage.OpenFileBlockStore(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	if _, err := blockchain.InitializeBlockchainStore(store); err != nil {
		_ = store.Close()
		if !errors.Is(err, blockchain.ErrAlreadyInitialized) {
			return nil, err
		}
	} else if err := store.Close(); err != nil {
		return nil, err
	}
	return newNodeRuntime(cfg)
}
