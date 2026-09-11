package wallet_test

import (
	"crypto/sha256"
	"testing"

	"p2pchain/internal/wallet"
)

// TestKeyGeneration 验证能生成非空的密钥对。
func TestKeyGeneration(t *testing.T) {
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("NewWallet error: %v", err)
	}
	if w.PrivateKey == nil {
		t.Fatal("private key is nil")
	}
	if len(w.PublicKey) == 0 {
		t.Fatal("public key is empty")
	}
}

// TestSignAndVerifyValid 验证签名可被同一公钥验签通过。
func TestSignAndVerifyValid(t *testing.T) {
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("NewWallet error: %v", err)
	}
	msg := sha256.Sum256([]byte("hello"))
	sig, err := w.Sign(msg)
	if err != nil {
		t.Fatalf("Sign error: %v", err)
	}
	if len(sig) != 64 {
		t.Fatalf("signature length = %d, want 64", len(sig))
	}
	ok, err := wallet.Verify(w.PublicKey, msg, sig)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if !ok {
		t.Fatal("valid signature failed to verify")
	}
}

// TestVerifyRejectsTamperedMessage 验证篡改消息后验签失败。
func TestVerifyRejectsTamperedMessage(t *testing.T) {
	w, _ := wallet.NewWallet()
	msg := sha256.Sum256([]byte("hello"))
	sig, _ := w.Sign(msg)
	other := sha256.Sum256([]byte("tampered"))
	ok, _ := wallet.Verify(w.PublicKey, other, sig)
	if ok {
		t.Fatal("signature over a different message must not verify")
	}
}

// TestVerifyRejectsWrongKey 验证用错误公钥验签失败。
func TestVerifyRejectsWrongKey(t *testing.T) {
	w1, _ := wallet.NewWallet()
	w2, _ := wallet.NewWallet()
	msg := sha256.Sum256([]byte("hello"))
	sig, _ := w1.Sign(msg)
	ok, _ := wallet.Verify(w2.PublicKey, msg, sig)
	if ok {
		t.Fatal("signature verified with the wrong public key")
	}
}
