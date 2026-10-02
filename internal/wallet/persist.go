// 钱包持久化：P0-4 后私钥只以加密形式落盘（v2 信封），明文 v1 仅保留读取用于迁移。
//
// 持久化协议（PHASE WALLET-PERSISTENCE-HARDENING-1，方案 B1）保持不变：
// 同目录 tmp → 完整写入 → Sync → Close → 原子 rename 替换。
// 加密在原子写之前完成（先加密成字节再走 tmp 路径），crash-safe 语义与
// hookWrite/hookSync/hookRename 失败注入点原样复用。
package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
)

// walletFileJSON v1 明文字段（历史格式，仅迁移读取用；新钱包不再产生）。
type walletFileJSON struct {
	Version int    `json:"version"`
	D       string `json:"d"`
	X       string `json:"x"`
	Y       string `json:"y"`
	PubKey  string `json:"pubkey"`
}

// 钱包文件路径（P0-4：与区块数据分离，persist.go 旧注释的要求）。
const (
	// WalletSecretsDir datadir 下的 secrets 子目录（0700）。
	WalletSecretsDir = "secrets"
	// WalletFileName 钱包文件名。
	WalletFileName = "wallet.json"
)

// DefaultWalletPath 新版加密钱包路径：<datadir>/secrets/wallet.json。
func DefaultWalletPath(dataDir string) string {
	return filepath.Join(dataDir, WalletSecretsDir, WalletFileName)
}

// LegacyWalletPath 旧版明文钱包路径：<datadir>/wallet.json（仅迁移检测用）。
func LegacyWalletPath(dataDir string) string {
	return filepath.Join(dataDir, WalletFileName)
}

// 未导出测试注入点：仅在单元测试中被替换，生产路径语义为零改变。
// PHASE WALLET-PERSISTENCE-HARDENING-1：用于 T4–T7 失败注入。
var (
	hookWrite = func(f *os.File, data []byte) error {
		_, err := f.Write(data)
		return err
	}
	hookSync   = func(f *os.File) error { return f.Sync() }
	hookRename = os.Rename
)

// atomicWriteFile crash-safe 原子写：tmp → 写入 → Sync → Close → rename。
// data 必须已是最终字节（明文 JSON 或加密信封），本函数不关心内容语义。
//
// 明确禁止：delete-before-rename、O_TRUNC 打开目标、rename 失败后的任何非原子 fallback
// （含 Windows sharing violation 时的绕过尝试）——失败即返回错误，方向 fail-stop。
func atomicWriteFile(path string, data []byte) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("创建钱包目录失败: %w", err)
		}
	}
	// tmp 与目标同目录 → 同一 filesystem → rename 原子性成立。
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("创建钱包临时文件失败: %w", err)
	}
	if err := hookWrite(f, data); err != nil {
		f.Close()
		removeTmp(tmp)
		return fmt.Errorf("写入钱包临时文件失败: %w", err)
	}
	if err := hookSync(f); err != nil {
		f.Close()
		removeTmp(tmp)
		return fmt.Errorf("同步钱包临时文件失败: %w", err)
	}
	if err := f.Close(); err != nil {
		removeTmp(tmp)
		return fmt.Errorf("关闭钱包临时文件失败: %w", err)
	}
	if err := hookRename(tmp, path); err != nil {
		removeTmp(tmp)
		return fmt.Errorf("原子替换钱包文件失败: %w", err)
	}
	return nil
}

// removeTmp 失败路径的 best-effort 清理：其自身失败不得覆盖主持久化错误，故忽略返回值。
func removeTmp(tmp string) {
	_ = os.Remove(tmp)
}

// SaveToFile 将钱包以 crash-safe 方式写入指定路径（0600 权限）。
//
// P0-4：保留明文写入仅用于测试与内部兼容；生产路径必须使用 SaveEncryptedToFile。
// 已标记为 deprecated，新代码勿用。
//
// Deprecated: 使用 SaveEncryptedToFile。
func (w *Wallet) SaveToFile(path string) error {
	rec := walletFileJSON{
		Version: 1,
		D:       w.PrivateKey.D.Text(16),
		X:       w.PrivateKey.PublicKey.X.Text(16),
		Y:       w.PrivateKey.PublicKey.Y.Text(16),
		PubKey:  hex.EncodeToString(w.PublicKey),
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(path, data)
}

// SaveEncryptedToFile 加密后以 crash-safe 方式写入（P0-4 生产路径）。
func (w *Wallet) SaveEncryptedToFile(path string, password []byte) error {
	data, err := EncryptWallet(w, password)
	if err != nil {
		return err
	}
	return atomicWriteFile(path, data)
}

// walletFromDBytes 由 32 字节大端 D 重建钱包：ScalarBaseMult 重新派生公钥，
// 不信任任何外部输入的 X/Y（v1/v2 共用同一信任根）。
func walletFromDBytes(dBytes []byte) (*Wallet, error) {
	if len(dBytes) != 32 {
		return nil, fmt.Errorf("私钥长度非法（%d != 32）", len(dBytes))
	}
	d := new(big.Int).SetBytes(dBytes)
	if d.Sign() <= 0 {
		return nil, fmt.Errorf("私钥为零")
	}
	curve := elliptic.P256()
	if d.Cmp(curve.Params().N) >= 0 {
		return nil, fmt.Errorf("私钥超出曲线阶")
	}
	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = curve
	priv.D = d
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())
	if priv.PublicKey.X == nil || priv.PublicKey.Y == nil {
		return nil, fmt.Errorf("私钥无法派生有效公钥")
	}
	expected := elliptic.Marshal(curve, priv.PublicKey.X, priv.PublicKey.Y)
	return &Wallet{PrivateKey: priv, PublicKey: expected}, nil
}

// LoadFromFile 从指定路径读取 v1 明文钱包（仅迁移/测试用；生产加载走 LoadEncryptedFromFile）。
func LoadFromFile(path string) (*Wallet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取钱包文件失败: %w", err)
	}
	var rec walletFileJSON
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("解析钱包文件失败: %w", err)
	}
	if rec.Version != 1 {
		return nil, fmt.Errorf("LoadFromFile 只支持 v1 明文格式（version=%d），加密钱包请用 LoadEncryptedFromFile", rec.Version)
	}
	d := new(big.Int)
	if _, ok := d.SetString(rec.D, 16); !ok {
		return nil, fmt.Errorf("私钥字段非法")
	}
	dBytes := d.Bytes()
	padded := make([]byte, 32)
	if len(dBytes) > 32 {
		return nil, fmt.Errorf("私钥字段非法")
	}
	copy(padded[32-len(dBytes):], dBytes)
	w, err := walletFromDBytes(padded)
	if err != nil {
		return nil, err
	}
	// 若文件中记录了公钥，则做一致性校验（防止文件被篡改）
	if rec.PubKey != "" {
		got, err := hex.DecodeString(rec.PubKey)
		if err != nil {
			return nil, fmt.Errorf("公钥字段非法: %w", err)
		}
		if hex.EncodeToString(w.PublicKey) != hex.EncodeToString(got) {
			return nil, fmt.Errorf("钱包文件公钥与私钥不一致（文件可能被篡改）")
		}
	}
	return w, nil
}

// LoadEncryptedFromFile 从指定路径读取 v2 加密钱包并解密。
func LoadEncryptedFromFile(path string, password []byte) (*Wallet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取钱包文件失败: %w", err)
	}
	ver, err := DetectVersion(data)
	if err != nil {
		return nil, err
	}
	if ver == 1 {
		return nil, ErrLegacyPlaintextWallet
	}
	return DecryptWallet(data, password)
}

// LoadOrCreate 加载或创建钱包（P0-4 语义）：
//
//   - 文件存在且为 v2 → 口令解密加载（口令缺失/错误即失败）；
//   - 文件存在且为 v1 → 返回 ErrLegacyPlaintextWallet（指引 wallet encrypt 迁移）；
//   - 文件不存在且有口令 → 新建**加密**钱包并保存，created=true；
//   - 文件不存在且无口令 → fail-closed 报错（不再静默生成明文钱包）。
func LoadOrCreate(path string, password []byte) (*Wallet, bool, error) {
	if _, err := os.Stat(path); err == nil {
		w, err := LoadEncryptedFromFile(path, password)
		return w, false, err
	}
	if len(password) == 0 {
		return nil, false, fmt.Errorf("%w：新建钱包必须提供口令（--wallet-password-file）", ErrPasswordRequired)
	}
	w, err := NewWallet()
	if err != nil {
		return nil, false, err
	}
	if err := w.SaveEncryptedToFile(path, password); err != nil {
		return nil, false, err
	}
	return w, true, nil
}

// LoadOrCreateForDataDir datadir 感知的加载/创建：新路径 <datadir>/secrets/wallet.json；
// 若新路径不存在但旧路径 <datadir>/wallet.json 存在 v1 文件，返回 ErrLegacyPlaintextWallet。
func LoadOrCreateForDataDir(dataDir string, password []byte) (*Wallet, bool, error) {
	newPath := DefaultWalletPath(dataDir)
	if _, err := os.Stat(newPath); err == nil {
		return LoadOrCreate(newPath, password)
	}
	if data, err := os.ReadFile(LegacyWalletPath(dataDir)); err == nil {
		if ver, verr := DetectVersion(data); verr == nil && ver == 1 {
			return nil, false, fmt.Errorf("%w（旧文件：%s）", ErrLegacyPlaintextWallet, LegacyWalletPath(dataDir))
		}
	}
	return LoadOrCreate(newPath, password)
}
