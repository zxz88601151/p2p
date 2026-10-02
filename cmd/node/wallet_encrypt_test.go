// wallet_encrypt_test.go P0-4 回归测试（cmd 层）：wallet encrypt 迁移命令与节点启动 fail-closed。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"p2pchain/internal/wallet"
)

const testWalletPW = "test-wallet-password-0123456789"

func writeTestPWFile(t *testing.T, dir string) string {
	t.Helper()
	pwPath := filepath.Join(dir, "pw")
	if err := os.WriteFile(pwPath, []byte(testWalletPW), 0o600); err != nil {
		t.Fatal(err)
	}
	return pwPath
}

func seedLegacyV1(t *testing.T, dir string) {
	t.Helper()
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.SaveToFile(wallet.LegacyWalletPath(dir)); err != nil {
		t.Fatal(err)
	}
}

// TestWalletEncryptMigration v1 明文 → v2 加密迁移：新文件为 v2、旧文件删除、地址一致。
func TestWalletEncryptMigration(t *testing.T) {
	dir := t.TempDir()
	seedLegacyV1(t, dir)
	pwFile := writeTestPWFile(t, dir)

	// 先记下旧钱包地址（v1 可直接读）
	oldW, err := wallet.LoadFromFile(wallet.LegacyWalletPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	oldAddr := oldW.Address()

	var out, errBuf bytes.Buffer
	code := cmdWallet([]string{"encrypt", "-datadir", dir, "-password-file", pwFile}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("wallet encrypt 失败: code=%d out=%s err=%s", code, out.String(), errBuf.String())
	}
	// 旧明文文件必须删除
	if _, err := os.Stat(wallet.LegacyWalletPath(dir)); err == nil {
		t.Fatalf("迁移后旧明文文件必须删除")
	}
	// 新文件为 v2 且地址一致
	newW, err := wallet.LoadEncryptedFromFile(wallet.DefaultWalletPath(dir), []byte(testWalletPW))
	if err != nil {
		t.Fatalf("读取迁移后钱包失败: %v", err)
	}
	if newW.Address() != oldAddr {
		t.Fatalf("迁移前后地址不一致: %s != %s", newW.Address(), oldAddr)
	}
}

// TestWalletEncryptRefusesOverwrite 已存在加密钱包时拒绝覆盖。
func TestWalletEncryptRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	seedLegacyV1(t, dir)
	pwFile := writeTestPWFile(t, dir)

	var out, errBuf bytes.Buffer
	if code := cmdWallet([]string{"encrypt", "-datadir", dir, "-password-file", pwFile}, &out, &errBuf); code != 0 {
		t.Fatalf("首次迁移应成功: %s", errBuf.String())
	}
	// 再放一个 v1 回去，尝试二次迁移
	seedLegacyV1(t, dir)
	if code := cmdWallet([]string{"encrypt", "-datadir", dir, "-password-file", pwFile}, &out, &errBuf); code == 0 {
		t.Fatalf("已存在加密钱包时必须拒绝覆盖")
	}
}

// TestWalletEncryptRequiresPasswordFile 无口令文件时拒绝。
func TestWalletEncryptRequiresPasswordFile(t *testing.T) {
	dir := t.TempDir()
	seedLegacyV1(t, dir)
	var out, errBuf bytes.Buffer
	if code := cmdWallet([]string{"encrypt", "-datadir", dir}, &out, &errBuf); code == 0 {
		t.Fatalf("无口令文件时迁移必须拒绝")
	}
}

// TestWalletAddressWithoutPassword --address 无需口令。
func TestWalletAddressWithoutPassword(t *testing.T) {
	dir := t.TempDir()
	seedLegacyV1(t, dir)
	pwFile := writeTestPWFile(t, dir)
	var out, errBuf bytes.Buffer
	if code := cmdWallet([]string{"encrypt", "-datadir", dir, "-password-file", pwFile}, &out, &errBuf); code != 0 {
		t.Fatalf("迁移失败: %s", errBuf.String())
	}
	out.Reset()
	if code := cmdWallet([]string{"-datadir", dir, "-address"}, &out, &errBuf); code != 0 {
		t.Fatalf("--address 应无需口令: %s", errBuf.String())
	}
	if strings.TrimSpace(out.String()) == "" {
		t.Fatalf("--address 应输出地址")
	}
}

// TestNodeRequiresWalletPassword 节点启动无口令文件时 fail-closed。
func TestNodeRequiresWalletPassword(t *testing.T) {
	dir := t.TempDir()
	if code := cmdInit([]string{"-datadir", dir}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("init 失败")
	}
	_, err := newNodeRuntime(nodeConfig{
		DataDir:    dir,
		ListenAddr: "127.0.0.1:0",
		RPCAddr:    "127.0.0.1:0",
		// WalletPasswordFile 缺失
	})
	if err == nil {
		t.Fatalf("无钱包口令文件时节点必须拒绝启动（fail-closed）")
	}
	if !strings.Contains(err.Error(), "口令") {
		t.Fatalf("错误信息应指出口令问题，实际: %v", err)
	}
}

// TestNodeRejectsLegacyPlaintextWallet 旧版明文钱包存在时节点拒绝启动并指引迁移。
func TestNodeRejectsLegacyPlaintextWallet(t *testing.T) {
	dir := t.TempDir()
	if code := cmdInit([]string{"-datadir", dir}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("init 失败")
	}
	seedLegacyV1(t, dir)
	pwFile := writeTestPWFile(t, dir)
	_, err := newNodeRuntime(nodeConfig{
		DataDir:            dir,
		ListenAddr:         "127.0.0.1:0",
		RPCAddr:            "127.0.0.1:0",
		WalletPasswordFile: pwFile,
	})
	if err == nil {
		t.Fatalf("旧版明文钱包存在时必须拒绝启动")
	}
	if !strings.Contains(err.Error(), "encrypt") {
		t.Fatalf("错误信息应指引 wallet encrypt 迁移，实际: %v", err)
	}
}
