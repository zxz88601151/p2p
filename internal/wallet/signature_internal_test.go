package wallet

// 本文件为包内测试（package wallet），用于直接验证签名编解码这一内部不变量。
// 这是对「ECDSA 签名变长拼接导致偶发验签失败」缺陷的确定性回归测试：
// 外部测试无法稳定构造「r 或 s 带前导零字节」的场景，只能靠概率命中，
// 因此必须从内部直接喂入带前导零的 r/s 来锁定行为。

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"math/big"
	"testing"
)

// TestEncodeSignatureFixedWidth 带前导零的 r/s 也必须编码为定长 64 字节。
func TestEncodeSignatureFixedWidth(t *testing.T) {
	cases := []struct {
		name string
		r, s *big.Int
	}{
		{"r 带 1 个前导零字节", new(big.Int).SetBytes(append([]byte{0x00}, bytes.Repeat([]byte{0xff}, 31)...)), big.NewInt(1)},
		{"s 带 1 个前导零字节", big.NewInt(1), new(big.Int).SetBytes(append([]byte{0x00}, bytes.Repeat([]byte{0xfe}, 31)...))},
		{"r/s 均极小（大量前导零）", big.NewInt(1), big.NewInt(2)},
		{"r/s 均为零", big.NewInt(0), big.NewInt(0)},
		{"r/s 满 32 字节", new(big.Int).SetBytes(bytes.Repeat([]byte{0xff}, 32)), new(big.Int).SetBytes(bytes.Repeat([]byte{0x01}, 32))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sig := encodeSignature(c.r, c.s)
			if len(sig) != SignatureSize {
				t.Fatalf("编码长度 = %d, want %d", len(sig), SignatureSize)
			}
			gotR, gotS, err := decodeSignature(sig)
			if err != nil {
				t.Fatalf("解码失败: %v", err)
			}
			if gotR.Cmp(c.r) != 0 || gotS.Cmp(c.s) != 0 {
				t.Fatalf("往返不一致: r=%v/%v s=%v/%v", gotR, c.r, gotS, c.s)
			}
		})
	}
}

// TestDecodeSignatureRejectsVariableLength 变长（历史实现遗留的 63/65 字节）签名必须被拒绝，
// 而不是被「从中间切分」后静默误判。
func TestDecodeSignatureRejectsVariableLength(t *testing.T) {
	for _, n := range []int{0, 31, 62, 63, 65, 128} {
		if _, _, err := decodeSignature(make([]byte, n)); err != ErrBadSignatureLength {
			t.Fatalf("长度 %d 未返回 ErrBadSignatureLength，实际: %v", n, err)
		}
	}
}

// TestVerifyRejectsBadLengthAndZeroRS Verify 对非法长度与零值 r/s 必须返回 false/错误，绝不 panic。
func TestVerifyRejectsBadLengthAndZeroRS(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub := elliptic.Marshal(elliptic.P256(), priv.PublicKey.X, priv.PublicKey.Y)
	var msg [32]byte

	if ok, err := Verify(pub, msg, make([]byte, 63)); ok || err != ErrBadSignatureLength {
		t.Fatalf("63 字节签名未被拒绝: ok=%v err=%v", ok, err)
	}
	if ok, err := Verify(pub, msg, make([]byte, SignatureSize)); ok || err != nil {
		t.Fatalf("全零签名应验签失败且无错误: ok=%v err=%v", ok, err)
	}
}

// TestSignAlwaysFixedWidthAndVerifiable 签名长度恒为 64 字节，且全部可验签通过。
// 注意：这是不变量断言（新实现下必然 100% 成立），不是靠概率碰运气；
// 出现任何一次长度不为 64 即说明定长编码被破坏。
func TestSignAlwaysFixedWidthAndVerifiable(t *testing.T) {
	w, err := NewWallet()
	if err != nil {
		t.Fatal(err)
	}
	const rounds = 512
	for i := 0; i < rounds; i++ {
		var h [32]byte
		h[0] = byte(i)
		h[31] = byte(i >> 8)
		sig, err := w.Sign(h)
		if err != nil {
			t.Fatalf("第 %d 次签名失败: %v", i, err)
		}
		if len(sig) != SignatureSize {
			t.Fatalf("第 %d 次签名长度 = %d, want %d（定长编码被破坏）", i, len(sig), SignatureSize)
		}
		ok, err := Verify(w.PublicKey, h, sig)
		if err != nil || !ok {
			t.Fatalf("第 %d 次验签失败: ok=%v err=%v", i, ok, err)
		}
	}
}

// TestVerifyRejectsTamperedMessage 篡改消息哈希后验签必须失败。
func TestVerifyRejectsTamperedMessage(t *testing.T) {
	w, err := NewWallet()
	if err != nil {
		t.Fatal(err)
	}
	var h [32]byte
	sig, err := w.Sign(h)
	if err != nil {
		t.Fatal(err)
	}
	h[0] ^= 0x01
	if ok, _ := Verify(w.PublicKey, h, sig); ok {
		t.Fatal("篡改消息后验签仍然通过")
	}
}
