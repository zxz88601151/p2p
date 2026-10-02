// keystore_test.go P0-4 回归测试：钱包静态加密。
package wallet

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const testPassword = "correct-horse-battery-staple-42"

// TestPBKDF2_KnownVectors PBKDF2-HMAC-SHA256 已知答案向量（RFC 7914 附录 A 风格，
// 用 Python hashlib.pbkdf2_hmac 独立生成，防自证）。
func TestPBKDF2_KnownVectors(t *testing.T) {
	cases := []struct {
		pass, salt string
		iter       int
		dkLen      int
		want       string
	}{
		{"password", "salt", 1, 32,
			"120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"},
		{"password", "salt", 2, 32,
			"ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43"},
		{"passwordPASSWORDpassword", "saltSALTsaltSALTsaltSALTsaltSALTsalt", 4096, 40,
			"348c89dbcbd32b2f32d814b8116e84cf2b17347ebc1800181c4e2a1fb8dd53e1c635518c7dac47e9"},
	}
	for _, c := range cases {
		got := hex.EncodeToString(pbkdf2Key([]byte(c.pass), []byte(c.salt), c.iter, c.dkLen))
		if got != c.want {
			t.Fatalf("PBKDF2(%q, %q, %d) = %s, want %s", c.pass, c.salt, c.iter, got, c.want)
		}
	}
}

// TestEncryptDecryptRoundTrip 加密→解密往返，地址一致，签名可用。
func TestEncryptDecryptRoundTrip(t *testing.T) {
	w, err := NewWallet()
	if err != nil {
		t.Fatal(err)
	}
	data, err := EncryptWallet(w, []byte(testPassword))
	if err != nil {
		t.Fatal(err)
	}
	// 信封里必须没有明文私钥
	if strings.Contains(string(data), hex.EncodeToString(w.PrivateKey.D.Bytes())) {
		t.Fatalf("加密信封泄露明文私钥")
	}
	w2, err := DecryptWallet(data, []byte(testPassword))
	if err != nil {
		t.Fatal(err)
	}
	if w2.Address() != w.Address() {
		t.Fatalf("解密后地址不一致: %s != %s", w2.Address(), w.Address())
	}
	msg := [32]byte{1, 2, 3}
	sig, err := w2.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := Verify(w2.PublicKey, msg, sig)
	if err != nil || !ok {
		t.Fatalf("解密后钱包签名不可用: ok=%v err=%v", ok, err)
	}
}

// TestDecryptWrongPassword 错口令必须失败（GCM 认证）。
func TestDecryptWrongPassword(t *testing.T) {
	w, _ := NewWallet()
	data, _ := EncryptWallet(w, []byte(testPassword))
	if _, err := DecryptWallet(data, []byte("wrong-password-0123456789")); err == nil {
		t.Fatalf("错口令解密必须失败")
	}
}

// TestDecryptTampered 篡改密文/盐/nonce 必须失败。
func TestDecryptTampered(t *testing.T) {
	w, _ := NewWallet()
	data, _ := EncryptWallet(w, []byte(testPassword))
	// 翻转密文最后一个 hex 字符
	tampered := string(data[:len(data)-3]) + "0" + string(data[len(data)-2:])
	if _, err := DecryptWallet([]byte(tampered), []byte(testPassword)); err == nil {
		t.Fatalf("篡改密文后解密必须失败")
	}
}

// TestEncryptSaltRandomness 两次加密同一钱包，盐/nonce/密文必须不同（防确定性泄露）。
func TestEncryptSaltRandomness(t *testing.T) {
	w, _ := NewWallet()
	a, _ := EncryptWallet(w, []byte(testPassword))
	b, _ := EncryptWallet(w, []byte(testPassword))
	if string(a) == string(b) {
		t.Fatalf("两次加密输出完全相同：盐/nonce 未随机化")
	}
}

// TestLoadPasswordFile 口令文件的校验：权限/长度。
func TestLoadPasswordFile(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "pw")
	if err := os.WriteFile(good, []byte(testPassword+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pw, err := LoadPasswordFile(good)
	if err != nil {
		t.Fatal(err)
	}
	if string(pw) != testPassword {
		t.Fatalf("口令归一化错误: %q", pw)
	}
	// 过短
	short := filepath.Join(dir, "short")
	os.WriteFile(short, []byte("abc"), 0o600)
	if _, err := LoadPasswordFile(short); err == nil {
		t.Fatalf("过短口令必须拒绝")
	}
	// 权限过宽（POSIX）：Windows 无 POSIX 权限位语义，跳过（与 keystore.go 的
	// runtime.GOOS != "windows" 守卫保持一致）。
	if runtime.GOOS != "windows" {
		wide := filepath.Join(dir, "wide")
		os.WriteFile(wide, []byte(testPassword), 0o644)
		if _, err := LoadPasswordFile(wide); err == nil {
			t.Fatalf("0644 权限口令文件必须拒绝")
		}
	}
	// 不存在
	if _, err := LoadPasswordFile(filepath.Join(dir, "nope")); err == nil {
		t.Fatalf("缺失口令文件必须报错")
	}
}

// TestEncryptedSaveLoadRoundTrip 加密落盘往返 + 0600 权限。
func TestEncryptedSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets", "wallet.json")
	w, _ := NewWallet()
	if err := w.SaveEncryptedToFile(path, []byte(testPassword)); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(path); err != nil || fi.Mode().Perm()&0o077 != 0 {
			t.Fatalf("加密钱包文件权限必须为 owner-only")
		}
	}
	w2, err := LoadEncryptedFromFile(path, []byte(testPassword))
	if err != nil {
		t.Fatal(err)
	}
	if w2.Address() != w.Address() {
		t.Fatalf("落盘往返地址不一致")
	}
}

// TestLoadOrCreateSemantics LoadOrCreate 的 P0-4 语义矩阵。
func TestLoadOrCreateSemantics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "w.json")

	// 1. 无文件 + 无口令 → fail-closed
	if _, _, err := LoadOrCreate(path, nil); err == nil {
		t.Fatalf("无口令新建必须拒绝（fail-closed）")
	}
	// 2. 无文件 + 有口令 → 新建加密钱包
	w, created, err := LoadOrCreate(path, []byte(testPassword))
	if err != nil || !created {
		t.Fatalf("有口令新建应成功: created=%v err=%v", created, err)
	}
	ver := mustDetectVersion(t, path)
	if ver != 2 {
		t.Fatalf("新建钱包应为 v2 加密格式，实际 v%d", ver)
	}
	// 3. v2 + 正确口令 → 加载
	w2, created, err := LoadOrCreate(path, []byte(testPassword))
	if err != nil || created || w2.Address() != w.Address() {
		t.Fatalf("v2 加载失败: created=%v err=%v", created, err)
	}
	// 4. v2 + 错口令 → 失败
	if _, _, err := LoadOrCreate(path, []byte("wrong-password-0123456789")); err == nil {
		t.Fatalf("错口令加载必须失败")
	}
	// 5. v1 明文 → ErrLegacyPlaintextWallet
	v1path := filepath.Join(dir, "v1.json")
	w3, _ := NewWallet()
	if err := w3.SaveToFile(v1path); err != nil { // 明文旧格式
		t.Fatal(err)
	}
	if _, _, err := LoadOrCreate(v1path, []byte(testPassword)); !isLegacyErr(err) {
		t.Fatalf("v1 文件应返回 ErrLegacyPlaintextWallet，实际: %v", err)
	}
}

// TestLoadOrCreateForDataDir datadir 感知：旧路径 v1 → 指引迁移。
func TestLoadOrCreateForDataDir(t *testing.T) {
	dir := t.TempDir()
	legacy := LegacyWalletPath(dir)
	w, _ := NewWallet()
	if err := w.SaveToFile(legacy); err != nil {
		t.Fatal(err)
	}
	_, _, err := LoadOrCreateForDataDir(dir, []byte(testPassword))
	if !isLegacyErr(err) {
		t.Fatalf("旧路径 v1 应指引迁移，实际: %v", err)
	}
	if !strings.Contains(err.Error(), legacy) {
		t.Fatalf("错误信息应包含旧文件路径，实际: %v", err)
	}
}

// TestLoadAddressOnly 无口令读地址（v2 公钥明文）。
func TestLoadAddressOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "w.json")
	w, _ := NewWallet()
	if err := w.SaveEncryptedToFile(path, []byte(testPassword)); err != nil {
		t.Fatal(err)
	}
	addr, err := LoadAddressOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	if addr != w.Address() {
		t.Fatalf("无口令读地址不一致: %s != %s", addr, w.Address())
	}
	// v1 拒绝
	v1path := filepath.Join(dir, "v1.json")
	w.SaveToFile(v1path)
	if _, err := LoadAddressOnly(v1path); !isLegacyErr(err) {
		t.Fatalf("v1 无口令读地址应指引迁移，实际: %v", err)
	}
}

// TestEncryptedAtomicWriteFailure 加密保存仍走原子写协议：hook 注入失败不留半写。
func TestEncryptedAtomicWriteFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "w.json")
	w, _ := NewWallet()
	if err := w.SaveEncryptedToFile(path, []byte(testPassword)); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)

	oldHook := hookWrite
	hookWrite = func(f *os.File, data []byte) error {
		return os.ErrInvalid // 注入写失败
	}
	defer func() { hookWrite = oldHook }()
	w2, _ := NewWallet()
	if err := w2.SaveEncryptedToFile(path, []byte(testPassword)); err == nil {
		t.Fatalf("注入写失败应返回错误")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatalf("原子写失败后旧文件必须字节级不变")
	}
}

func mustDetectVersion(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ver, err := DetectVersion(data)
	if err != nil {
		t.Fatal(err)
	}
	return ver
}

func isLegacyErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "明文钱包")
}
