// 钱包持久化：将密钥对以 JSON 形式落盘。
//
// 安全说明（重要）：此处为学习用途的明文 JSON 存储（权限 0600）。
// 生产实现必须使用口令派生密钥（如 argon2/scrypt）加密私钥后再落盘，
// 且不得与区块数据放在同一目录。相关设计要点见 storage 包的 TODO。
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

// walletFileJSON 私钥文件的磁盘格式（D/X/Y 十六进制）。
type walletFileJSON struct {
	Version int    `json:"version"`
	D       string `json:"d"`
	X       string `json:"x"`
	Y       string `json:"y"`
	PubKey  string `json:"pubkey"`
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

// SaveToFile 将钱包以 crash-safe 方式写入指定路径（0600 权限，目录不存在时自动创建）。
//
// 持久化协议（PHASE WALLET-PERSISTENCE-HARDENING-1，方案 B1）：
// 同目录 tmp → 完整写入 → Sync → Close → 原子 rename 替换。
// 保证 wallet.json 在任意 crash 边界下只能是「完整旧文件、完整新文件或不存在」，
// 绝不会成为半写/损坏 JSON（W1–W7 crash matrix，见阶段审计报告）。
//
// 明确禁止：delete-before-rename、O_TRUNC 打开目标、rename 失败后的任何非原子 fallback
// （含 Windows sharing violation 时的绕过尝试）——失败即返回错误，方向 fail-stop。
func (w *Wallet) SaveToFile(path string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("创建钱包目录失败: %w", err)
		}
	}
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

// LoadFromFile 从指定路径读取钱包并恢复密钥对，同时校验曲线点合法性。
func LoadFromFile(path string) (*Wallet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取钱包文件失败: %w", err)
	}
	var rec walletFileJSON
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("解析钱包文件失败: %w", err)
	}

	d := new(big.Int)
	if _, ok := d.SetString(rec.D, 16); !ok {
		return nil, fmt.Errorf("私钥字段非法")
	}
	curve := elliptic.P256()
	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = curve
	priv.D = d
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())
	if priv.PublicKey.X == nil || priv.PublicKey.Y == nil {
		return nil, fmt.Errorf("私钥无法派生有效公钥")
	}
	// 若文件中记录了公钥，则做一致性校验（防止文件被篡改）
	expected := elliptic.Marshal(curve, priv.PublicKey.X, priv.PublicKey.Y)
	if rec.PubKey != "" {
		got, err := hex.DecodeString(rec.PubKey)
		if err != nil {
			return nil, fmt.Errorf("公钥字段非法: %w", err)
		}
		if hex.EncodeToString(expected) != hex.EncodeToString(got) {
			return nil, fmt.Errorf("钱包文件公钥与私钥不一致（文件可能被篡改）")
		}
	}
	return &Wallet{PrivateKey: priv, PublicKey: expected}, nil
}

// LoadOrCreate 若文件存在则加载，否则生成新钱包并保存。
// 返回的钱包与 created 标记（true 表示本次新建）。
func LoadOrCreate(path string) (*Wallet, bool, error) {
	if _, err := os.Stat(path); err == nil {
		w, err := LoadFromFile(path)
		return w, false, err
	}
	w, err := NewWallet()
	if err != nil {
		return nil, false, err
	}
	if err := w.SaveToFile(path); err != nil {
		return nil, false, err
	}
	return w, true, nil
}
