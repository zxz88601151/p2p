package main

import (
	"errors"
	"os"
	"path/filepath"

	"p2pchain/internal/blockchain"
	"p2pchain/internal/storage"
)

// mustTestWalletPasswordFile 测试用：生成口令文件，失败即 Fatal（P0-4）。
func mustTestWalletPasswordFile(t interface {
	Fatalf(string, ...interface{})
}, dataDir string,
) string {
	pwPath, err := testWalletPasswordFileForTest(dataDir)
	if err != nil {
		t.Fatalf("生成测试口令文件失败: %v", err)
	}
	return pwPath
}

// testWalletPasswordFileForTest 在 datadir 下生成测试用 0600 口令文件（P0-4）。
// 生产代码永不调用；仅测试脚手架使用。
func testWalletPasswordFileForTest(dataDir string) (string, error) {
	pwPath := filepath.Join(dataDir, "secrets", "test-wallet-password")
	if err := os.MkdirAll(filepath.Dir(pwPath), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(pwPath, []byte("test-wallet-password-0123456789abcdef"), 0o600); err != nil {
		return "", err
	}
	return pwPath, nil
}

// newNodeRuntimeForTest makes fixture setup explicit for legacy tests that
// historically relied on node startup to create genesis implicitly. Production
// code never calls this helper; F-3B tests the real newNodeRuntime separately.
func newNodeRuntimeForTest(cfg nodeConfig) (*nodeRuntime, error) {
	// P0-4：测试默认注入口令文件（生产必填 --wallet-password-file）。
	if cfg.WalletPasswordFile == "" {
		pwPath, err := testWalletPasswordFileForTest(cfg.DataDir)
		if err != nil {
			return nil, err
		}
		cfg.WalletPasswordFile = pwPath
	}
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
