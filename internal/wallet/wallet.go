// Package wallet 实现最基础的密钥管理：生成密钥对、派生地址、对交易签名与验签。
//
// 骨架阶段使用标准库的 P-256 椭圆曲线（crypto/elliptic），实现简单、无需外部依赖。
// 真实的比特币系列项目使用 secp256k1 曲线，如果未来要和比特币生态兼容，
// 需要换成 github.com/btcsuite/btcd/btcec/v2。
//
// 地址生成也做了简化：真实比特币是 RIPEMD160(SHA256(pubkey)) 再做 Base58Check 编码，
// 这里先用 SHA256 截断代替 RIPEMD160（标准库没有内置 RIPEMD160），
// 并保留 TODO 说明如何升级为完全兼容版本。
package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"math/big"
)

// Wallet 持有一对密钥：私钥用于签名（必须保密），公钥用于生成地址、供他人验签。
type Wallet struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  []byte // 未压缩格式公钥字节：0x04 || X || Y
}

// SignatureSize 签名字节长度：r、s 各按 32 字节大端定长编码后拼接。
//
// 定长是必须的（曾经的缺陷）：r/s 是大整数，若最高位字节为 0，big.Int.Bytes() 返回的
// 长度会小于 32 甚至只有 31 字节。此时若按「变长拼接 + 从中间切分」来验签，
// 切分位置就会错位，导致约 1/128 的签名随机验不过——表现为极难复现的偶发失败。
const SignatureSize = 64

// ErrBadSignatureLength 签名长度不等于 SignatureSize。
var ErrBadSignatureLength = errors.New("签名长度非法（应为 64 字节定长 r||s）")

// NewWallet 生成一个新的密钥对。
func NewWallet() (*Wallet, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	pub := elliptic.Marshal(elliptic.P256(), priv.PublicKey.X, priv.PublicKey.Y)
	return &Wallet{PrivateKey: priv, PublicKey: pub}, nil
}

// PubKeyHash 计算公钥哈希（地址的核心内容）。取 SHA256(pubkey) 前 20 字节。
// 地址的「版本号 + 校验和 + Base58Check 编码」在 address.go / base58.go 中实现。
//
// TODO: 与比特币完全兼容需改为 RIPEMD160(SHA256(pubkey))（标准库无内置 RIPEMD160）。
func (w *Wallet) PubKeyHash() [20]byte {
	sum := sha256.Sum256(w.PublicKey)
	var hash [20]byte
	copy(hash[:], sum[:20])
	return hash
}

// Sign 对任意消息哈希做 ECDSA 签名，返回定长 64 字节的 r||s。
func (w *Wallet) Sign(msgHash [32]byte) ([]byte, error) {
	r, s, err := ecdsa.Sign(rand.Reader, w.PrivateKey, msgHash[:])
	if err != nil {
		return nil, err
	}
	return encodeSignature(r, s), nil
}

// encodeSignature 把 (r, s) 编码为 64 字节定长字节串（各 32 字节大端，左侧补零）。
func encodeSignature(r, s *big.Int) []byte {
	sig := make([]byte, SignatureSize)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return sig
}

// decodeSignature 从 64 字节定长字节串还原 (r, s)。
func decodeSignature(sig []byte) (r, s *big.Int, err error) {
	if len(sig) != SignatureSize {
		return nil, nil, ErrBadSignatureLength
	}
	return new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:]), nil
}

// Verify 用给定公钥字节验证签名是否有效。
func Verify(pubKeyBytes []byte, msgHash [32]byte, sig []byte) (bool, error) {
	x, y := elliptic.Unmarshal(elliptic.P256(), pubKeyBytes)
	if x == nil {
		return false, errors.New("非法公钥字节，无法解析曲线坐标")
	}
	pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

	r, s, err := decodeSignature(sig)
	if err != nil {
		return false, err
	}
	// r/s 为 0 时签名必然无效，显式拒绝（避免进入 ecdsa.Verify 的边界分支）
	if r.Sign() == 0 || s.Sign() == 0 {
		return false, nil
	}
	return ecdsa.Verify(pub, msgHash[:], r, s), nil
}
