// 钱包静态加密（P0-4）：口令派生密钥 + AES-256-GCM 加密私钥落盘。
//
// 设计约束：
//   - 零第三方依赖：KDF 用手写 PBKDF2-HMAC-SHA256（RFC 8018 §5.2，约 30 行，
//     简单到可审计；argon2 手写不安全，x/crypto 则打破仓库零依赖纪律）。
//   - 口令唯一来源 = 口令文件（0600 权限校验），绝不进命令行/环境变量/日志。
//   - 公钥保持明文：地址本就是公开身份（P2P 握手广播），读地址无需口令。
//   - 加密在原子写之前完成：先加密成字节，再走 persist.go 的 tmp→fsync→rename
//     协议，crash-safe 语义与失败注入测试原样复用。
package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// 加密参数。
const (
	// KeystoreVersion 加密信封版本（v1 为旧明文格式，见 persist.go）。
	KeystoreVersion = 2
	// KDFName KDF 算法标识（写入信封，未来可升级）。
	KDFName = "pbkdf2-hmac-sha256"
	// KDFIterations PBKDF2 迭代次数（OWASP 2023 对 PBKDF2-HMAC-SHA256 的推荐值）。
	KDFIterations = 600000
	// SaltLen 盐长度（字节）。
	SaltLen = 16
	// encryptedKeyLen 派生密钥长度 = AES-256。
	encryptedKeyLen = 32
	// minPasswordLen 口令最小长度（归一化后）。
	minPasswordLen = 12
	// maxPasswordLen 口令最大长度（防 DoS 性大口令）。
	maxPasswordLen = 4096
)

// keystoreKDF 信封中的 KDF 参数。
type keystoreKDF struct {
	Name       string `json:"name"`
	Iterations int    `json:"iterations"`
	Salt       string `json:"salt"`
}

// keystoreJSON v2 加密信封的磁盘格式。
type keystoreJSON struct {
	Version    int         `json:"version"`
	KDF        keystoreKDF `json:"kdf"`
	Nonce      string      `json:"nonce"`
	Ciphertext string      `json:"ciphertext"`
	PubKey     string      `json:"pubkey"`
}

var (
	// ErrLegacyPlaintextWallet 旧版 v1 明文钱包：拒绝加载，需 wallet encrypt 迁移。
	ErrLegacyPlaintextWallet = errors.New("检测到旧版明文钱包（v1），已拒绝加载；请运行 `p2pchain wallet encrypt --password-file <口令文件>` 迁移为加密钱包")
	// ErrWrongPasswordOrCorrupt 口令错误或文件损坏（GCM 认证失败，两者不可区分）。
	ErrWrongPasswordOrCorrupt = errors.New("钱包口令错误或文件已损坏")
	// ErrPasswordRequired 需要口令但未提供。
	ErrPasswordRequired = errors.New("加密钱包需要口令：请提供 --wallet-password-file")
)

// pbkdf2Key PBKDF2 密钥派生（RFC 8018 §5.2），PRF = HMAC-SHA256。
func pbkdf2Key(password, salt []byte, iter, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen
	out := make([]byte, 0, numBlocks*hashLen)
	var ctr [4]byte
	for block := 1; block <= numBlocks; block++ {
		binary.BigEndian.PutUint32(ctr[:], uint32(block))
		prf.Reset()
		prf.Write(salt)
		prf.Write(ctr[:])
		u := prf.Sum(nil)
		t := make([]byte, len(u))
		copy(t, u)
		for i := 1; i < iter; i++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
		ZeroBytes(t)
		ZeroBytes(u)
	}
	return out[:keyLen]
}

// ZeroBytes 覆盖敏感字节（best-effort，Go GC 语义下不保证绝对擦除，见设计文档 D4-6）。
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// LoadPasswordFile 从口令文件读取钱包口令（P0-4 唯一口令来源）。
//
// 校验：文件存在、归一化后长度 [12, 4096]、POSIX 下权限必须为 owner-only（0600）。
// 口令本身绝不进日志/错误详情——错误只描述校验结论。
func LoadPasswordFile(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取口令文件 %s 失败: %w", path, err)
	}
	pw := []byte(strings.TrimSpace(string(raw)))
	ZeroBytes(raw)
	if n := len(pw); n < minPasswordLen {
		ZeroBytes(pw)
		return nil, fmt.Errorf("口令文件 %s 内容过短（归一化后 %d 字符，要求 >= %d）", path, n, minPasswordLen)
	} else if n > maxPasswordLen {
		ZeroBytes(pw)
		return nil, fmt.Errorf("口令文件 %s 内容过长（归一化后 %d 字符，要求 <= %d）", path, n, maxPasswordLen)
	}
	if runtime.GOOS != "windows" {
		if fi, statErr := os.Stat(path); statErr == nil {
			if perm := fi.Mode().Perm(); perm&0o077 != 0 {
				ZeroBytes(pw)
				return nil, fmt.Errorf("口令文件 %s 权限过宽（%04o，要求 owner-only 0600）", path, perm)
			}
		}
	}
	return pw, nil
}

// EncryptWallet 将钱包私钥加密为 v2 信封 JSON 字节（不写盘）。
func EncryptWallet(w *Wallet, password []byte) ([]byte, error) {
	if w == nil || w.PrivateKey == nil {
		return nil, errors.New("钱包为空")
	}
	if len(password) == 0 {
		return nil, ErrPasswordRequired
	}
	salt := make([]byte, SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("生成盐失败: %w", err)
	}
	key := pbkdf2Key(password, salt, KDFIterations, encryptedKeyLen)
	defer ZeroBytes(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("生成 nonce 失败: %w", err)
	}
	// D 定长 32 字节大端（P-256 阶长度），前补零。
	plain := make([]byte, 32)
	dBytes := w.PrivateKey.D.Bytes()
	if len(dBytes) > 32 {
		return nil, errors.New("私钥长度异常")
	}
	copy(plain[32-len(dBytes):], dBytes)
	ciphertext := gcm.Seal(nil, nonce, plain, nil)
	ZeroBytes(plain)

	rec := keystoreJSON{
		Version: KeystoreVersion,
		KDF: keystoreKDF{
			Name:       KDFName,
			Iterations: KDFIterations,
			Salt:       hex.EncodeToString(salt),
		},
		Nonce:      hex.EncodeToString(nonce),
		Ciphertext: hex.EncodeToString(ciphertext),
		PubKey:     hex.EncodeToString(w.PublicKey),
	}
	return json.MarshalIndent(rec, "", "  ")
}

// DecryptWallet 解密 v2 信封 JSON 字节，还原钱包（含公钥一致性校验）。
func DecryptWallet(data, password []byte) (*Wallet, error) {
	if len(password) == 0 {
		return nil, ErrPasswordRequired
	}
	var rec keystoreJSON
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("解析加密钱包失败: %w", err)
	}
	if rec.Version != KeystoreVersion {
		return nil, fmt.Errorf("加密钱包版本 %d 不受支持（要求 %d）", rec.Version, KeystoreVersion)
	}
	if rec.KDF.Name != KDFName {
		return nil, fmt.Errorf("不支持的 KDF %q", rec.KDF.Name)
	}
	if rec.KDF.Iterations < 100000 {
		return nil, fmt.Errorf("KDF 迭代次数过低（%d），拒绝解密", rec.KDF.Iterations)
	}
	salt, err := hex.DecodeString(rec.KDF.Salt)
	if err != nil || len(salt) != SaltLen {
		return nil, fmt.Errorf("盐字段非法")
	}
	nonce, err := hex.DecodeString(rec.Nonce)
	if err != nil {
		return nil, fmt.Errorf("nonce 字段非法: %w", err)
	}
	ciphertext, err := hex.DecodeString(rec.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("密文字段非法: %w", err)
	}
	key := pbkdf2Key(password, salt, rec.KDF.Iterations, encryptedKeyLen)
	defer ZeroBytes(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrWrongPasswordOrCorrupt
	}
	defer ZeroBytes(plain)
	if len(plain) != 32 {
		return nil, ErrWrongPasswordOrCorrupt
	}
	// 用解密出的 D 重建钱包（复用 v1 的派生+校验逻辑，保证同一信任根）。
	w, err := walletFromDBytes(plain)
	if err != nil {
		return nil, err
	}
	if rec.PubKey != "" && hex.EncodeToString(w.PublicKey) != strings.ToLower(rec.PubKey) {
		return nil, fmt.Errorf("钱包文件公钥与私钥不一致（文件可能被篡改）")
	}
	return w, nil
}

// DetectVersion 嗅探钱包文件的版本号（1=旧明文，2=加密信封）。
func DetectVersion(data []byte) (int, error) {
	var probe struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return 0, fmt.Errorf("解析钱包文件失败: %w", err)
	}
	if probe.Version != 1 && probe.Version != KeystoreVersion {
		return 0, fmt.Errorf("不支持的钱包版本 %d", probe.Version)
	}
	return probe.Version, nil
}

// LoadAddressOnly 无口令读取钱包地址（v2 信封的公钥为明文；v1 拒绝并指引迁移）。
func LoadAddressOnly(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取钱包文件失败: %w", err)
	}
	ver, err := DetectVersion(data)
	if err != nil {
		return "", err
	}
	if ver == 1 {
		return "", ErrLegacyPlaintextWallet
	}
	var rec keystoreJSON
	if err := json.Unmarshal(data, &rec); err != nil {
		return "", fmt.Errorf("解析加密钱包失败: %w", err)
	}
	pub, err := hex.DecodeString(rec.PubKey)
	if err != nil || len(pub) == 0 {
		return "", fmt.Errorf("公钥字段非法")
	}
	w := &Wallet{PublicKey: pub}
	return w.Address(), nil
}
