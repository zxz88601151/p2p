package wallet_test

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"p2pchain/internal/wallet"
)

// ---- Base58 / Base58Check ----

func TestBase58RoundTrip(t *testing.T) {
	cases := [][]byte{
		{},
		{0x00},
		{0x00, 0x00, 0x01},
		{0xFF, 0xFE, 0x7A},
		[]byte("p2pchain"),
	}
	for _, in := range cases {
		enc := wallet.Base58Encode(in)
		dec, err := wallet.Base58Decode(enc)
		if err != nil {
			t.Fatalf("解码 %q 失败: %v", enc, err)
		}
		if len(dec) != len(in) {
			t.Fatalf("round-trip 长度不一致: %v → %q → %v", in, enc, dec)
		}
		for i := range in {
			if in[i] != dec[i] {
				t.Fatalf("round-trip 内容不一致: %v → %q → %v", in, enc, dec)
			}
		}
	}
}

func TestBase58RejectsAmbiguousChars(t *testing.T) {
	for _, s := range []string{"0OIl", "abc0", "l1I"} {
		if _, err := wallet.Base58Decode(s); err == nil {
			t.Fatalf("非法字符未被拒绝: %q", s)
		}
	}
}

// ---- 地址 ----

func TestAddressRoundTrip(t *testing.T) {
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("生成钱包失败: %v", err)
	}
	addr := w.Address()
	if addr == "" {
		t.Fatal("地址为空")
	}
	got, err := wallet.DecodeAddress(addr)
	if err != nil {
		t.Fatalf("解析地址失败: %v", err)
	}
	if got != w.PubKeyHash() {
		t.Fatalf("地址解析结果与公钥哈希不一致: %x vs %x", got, w.PubKeyHash())
	}
	if err := wallet.ValidateAddress(addr); err != nil {
		t.Fatalf("地址校验失败: %v", err)
	}
}

func TestAddressRejectsTampering(t *testing.T) {
	w, _ := wallet.NewWallet()
	addr := w.Address()

	// 篡改最后一个字符（校验和必须发现）
	tampered := addr[:len(addr)-1] + string(boolChar(addr[len(addr)-1]))
	if _, err := wallet.DecodeAddress(tampered); err == nil {
		t.Fatal("被篡改的地址未拒绝")
	}
	// 明显非法输入
	if _, err := wallet.DecodeAddress("not-an-address"); err == nil {
		t.Fatal("非法地址字符串未拒绝")
	}
	if _, err := wallet.DecodeAddress(""); err == nil {
		t.Fatal("空地址未拒绝")
	}
}

func boolChar(c byte) byte {
	if c == 'a' {
		return 'b'
	}
	return 'a'
}

// TestAddressVersionEnforced 版本字节不匹配必须拒绝（例如比特币风格地址）。
func TestAddressVersionEnforced(t *testing.T) {
	w, _ := wallet.NewWallet()
	h := w.PubKeyHash()
	btcStyle := b58check(0x00, h[:])
	if _, err := wallet.DecodeAddress(btcStyle); err == nil {
		t.Fatal("错误版本字节的地址未被拒绝")
	}
}

// b58check 在测试侧独立实现 Base58Check 编码（用于构造版本字节不匹配的地址）。
func b58check(version byte, payload []byte) string {
	checked := append([]byte{version}, payload...)
	first := sha256.Sum256(checked)
	second := sha256.Sum256(first[:])
	return wallet.Base58Encode(append(checked, second[:4]...))
}

// ---- 持久化 ----

func TestWalletSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.dat")

	w1, created, err := wallet.LoadOrCreate(path)
	if err != nil {
		t.Fatalf("创建钱包失败: %v", err)
	}
	if !created {
		t.Fatal("首次调用应创建新钱包")
	}

	w2, created, err := wallet.LoadOrCreate(path)
	if err != nil {
		t.Fatalf("加载钱包失败: %v", err)
	}
	if created {
		t.Fatal("已存在钱包文件时不应重建")
	}
	if w1.Address() != w2.Address() {
		t.Fatalf("重新加载后地址不一致: %s vs %s", w1.Address(), w2.Address())
	}

	// 签名/验签仍可用（密钥正确恢复）
	h := [32]byte{1, 2, 3}
	sig, err := w2.Sign(h)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	ok, err := wallet.Verify(w2.PublicKey, h, sig)
	if err != nil || !ok {
		t.Fatalf("恢复的钱包验签失败: ok=%v err=%v", ok, err)
	}
}

func TestWalletFilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "wallet.dat")
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("生成钱包失败: %v", err)
	}
	if err := w.SaveToFile(path); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat 失败: %v", err)
	}
	// Windows 上权限位语义有限（始终报告 0666），仅在 POSIX 平台严格校验
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不支持 POSIX 权限位语义")
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("钱包文件权限过宽: %v", info.Mode().Perm())
	}
}

func TestLoadFromFileRejectsCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.dat")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := wallet.LoadFromFile(path); err == nil {
		t.Fatal("损坏的钱包文件未报错")
	}
}
