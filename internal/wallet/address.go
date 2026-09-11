// 地址派生与解析。
package wallet

import (
	"fmt"
)

// Address 返回钱包的 Base58Check 地址：Base58Check(0x35 || SHA256(pubkey)[:20])。
func (w *Wallet) Address() string {
	h := w.PubKeyHash()
	return base58CheckEncode(AddressVersion, h[:])
}

// AddressFromPubKeyHash 由公钥哈希构造地址字符串。
func AddressFromPubKeyHash(pubKeyHash [20]byte) string {
	return base58CheckEncode(AddressVersion, pubKeyHash[:])
}

// DecodeAddress 解析 Base58Check 地址为公钥哈希。
// 校验：Base58 字符合法性、校验和、版本字节。
func DecodeAddress(addr string) ([20]byte, error) {
	var hash [20]byte
	version, payload, err := base58CheckDecode(addr)
	if err != nil {
		return hash, err
	}
	if version != AddressVersion {
		return hash, fmt.Errorf("%w: got 0x%02x want 0x%02x", ErrBadAddressVersion, version, AddressVersion)
	}
	if len(payload) != 20 {
		return hash, fmt.Errorf("%w: 载荷长度 %d != 20", ErrInvalidAddress, len(payload))
	}
	copy(hash[:], payload)
	return hash, nil
}

// ValidateAddress 仅校验地址形态（不解析）。
func ValidateAddress(addr string) error {
	_, err := DecodeAddress(addr)
	return err
}
