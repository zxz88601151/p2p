// Base58 与 Base58Check 编码（标准库实现，无外部依赖）。
//
// Base58 是比特币地址采用的编码：去除易混淆字符（0/O/I/l）的 Base58 字母表。
// Base58Check = 版本字节 || 载荷 || 前 4 字节校验和（SHA256(SHA256(payload))），
// 能检测地址的抄写错误。
package wallet

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"math/big"
)

// AddressVersion p2pchain 地址版本字节（自定义网络，区别于比特币 0x00/0x6f）。
const AddressVersion byte = 0x35

var (
	ErrInvalidAddress    = errors.New("地址格式非法")
	ErrBadAddressVersion = errors.New("地址版本字节不匹配")
	ErrBadChecksum       = errors.New("地址校验和不匹配")
)

const b58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// Base58Encode 将字节流编码为 Base58 字符串（保留前导零 → '1'）。
func Base58Encode(input []byte) string {
	x := new(big.Int).SetBytes(input)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := new(big.Int)

	var out []byte
	for x.Cmp(zero) > 0 {
		x.DivMod(x, base, mod)
		out = append(out, b58Alphabet[mod.Int64()])
	}
	for _, b := range input {
		if b != 0x00 {
			break
		}
		out = append(out, b58Alphabet[0]) // '1'
	}
	// 反转
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}

// Base58Decode 解码 Base58 字符串（返回原始字节，保留前导零）。
func Base58Decode(s string) ([]byte, error) {
	x := big.NewInt(0)
	base := big.NewInt(58)
	for _, c := range s {
		idx := -1
		for i, a := range b58Alphabet {
			if a == c {
				idx = i
				break
			}
		}
		if idx < 0 {
			return nil, ErrInvalidAddress
		}
		x.Mul(x, base)
		x.Add(x, big.NewInt(int64(idx)))
	}
	decoded := x.Bytes()
	// 前导 '1' → 前导 0x00
	nZeros := 0
	for _, c := range s {
		if c == '1' {
			nZeros++
		} else {
			break
		}
	}
	return append(make([]byte, nZeros), decoded...), nil
}

// base58CheckEncode 版本字节 + 载荷 + 4 字节校验和。
func base58CheckEncode(version byte, payload []byte) string {
	checked := append([]byte{version}, payload...)
	first := sha256.Sum256(checked)
	second := sha256.Sum256(first[:])
	checked = append(checked, second[:4]...)
	return Base58Encode(checked)
}

// base58CheckDecode 解码并校验版本字节与校验和，返回版本字节与载荷。
func base58CheckDecode(s string) (byte, []byte, error) {
	raw, err := Base58Decode(s)
	if err != nil {
		return 0, nil, err
	}
	if len(raw) < 6 { // 1 版本 + 最少 1 载荷 + 4 校验和
		return 0, nil, ErrInvalidAddress
	}
	version := raw[0]
	payload := raw[1 : len(raw)-4]
	checksum := raw[len(raw)-4:]

	first := sha256.Sum256(raw[:len(raw)-4])
	second := sha256.Sum256(first[:])
	if !bytes.Equal(second[:4], checksum) {
		return 0, nil, ErrBadChecksum
	}
	return version, payload, nil
}
