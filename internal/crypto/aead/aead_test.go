package aead

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
)

// helper 生成 n 字节随机密钥，失败时调用 t.Fatal 终止测试。
func mustRandBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("生成随机字节失败: %v", err)
	}
	return b
}

// helper 生成 12 字节随机 nonce。
func mustRandNonce(t *testing.T) []byte {
	t.Helper()
	return mustRandBytes(t, GCMNonceSize)
}

// TestAES256GCMEncryptDecrypt 验证 AES-256-GCM 加密后解密应还原原文。
func TestAES256GCMEncryptDecrypt(t *testing.T) {
	key := mustRandBytes(t, 32)
	nonce := mustRandNonce(t)
	// 测试数据使用 base64 编码的明文
	plaintext, err := base64.StdEncoding.DecodeString("aGVsbG8ga2V5LXZhdWx0IHRlc3Q=")
	if err != nil {
		t.Fatalf("解码测试明文失败: %v", err)
	}
	aad := []byte("associated-data")

	provider, err := New(AlgAES256GCM, key)
	if err != nil {
		t.Fatalf("创建 AES-256-GCM provider 失败: %v", err)
	}

	ciphertext, err := provider.Encrypt(plaintext, aad, nonce)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("密文不应等于明文")
	}

	got, err := provider.Decrypt(ciphertext, aad, nonce)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("解密结果与原文不符: got %q, want %q", got, plaintext)
	}
}

// TestAES256GCMWrongKey 验证错误密钥解密应失败。
func TestAES256GCMWrongKey(t *testing.T) {
	key1 := mustRandBytes(t, 32)
	key2 := mustRandBytes(t, 32)
	nonce := mustRandNonce(t)
	plaintext := []byte("secret message")
	aad := []byte("aad")

	p1, err := New(AlgAES256GCM, key1)
	if err != nil {
		t.Fatalf("创建 provider1 失败: %v", err)
	}
	p2, err := New(AlgAES256GCM, key2)
	if err != nil {
		t.Fatalf("创建 provider2 失败: %v", err)
	}

	ciphertext, err := p1.Encrypt(plaintext, aad, nonce)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	if _, err := p2.Decrypt(ciphertext, aad, nonce); err == nil {
		t.Fatal("使用错误密钥解密应失败，但成功了")
	}
}

// TestAES256GCMTamperedCiphertext 验证篡改密文应解密失败。
func TestAES256GCMTamperedCiphertext(t *testing.T) {
	key := mustRandBytes(t, 32)
	nonce := mustRandNonce(t)
	plaintext := []byte("tamper test")
	aad := []byte("aad")

	provider, err := New(AlgAES256GCM, key)
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	ciphertext, err := provider.Encrypt(plaintext, aad, nonce)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	// 篡改密文的最后一个字节（GCM tag 区域）
	if len(ciphertext) == 0 {
		t.Fatal("密文不应为空")
	}
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)-1] ^= 0xff

	if _, err := provider.Decrypt(tampered, aad, nonce); err == nil {
		t.Fatal("篡改密文后解密应失败，但成功了")
	}
}

// TestAES256GCMWrongAAD 验证错误 AAD 应解密失败。
func TestAES256GCMWrongAAD(t *testing.T) {
	key := mustRandBytes(t, 32)
	nonce := mustRandNonce(t)
	plaintext := []byte("aad test")
	aad := []byte("correct-aad")
	wrongAAD := []byte("wrong-aad")

	provider, err := New(AlgAES256GCM, key)
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	ciphertext, err := provider.Encrypt(plaintext, aad, nonce)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	if _, err := provider.Decrypt(ciphertext, wrongAAD, nonce); err == nil {
		t.Fatal("使用错误 AAD 解密应失败，但成功了")
	}
}

// TestSM4GCMEncryptDecrypt 验证 SM4-GCM 加解密往返。
func TestSM4GCMEncryptDecrypt(t *testing.T) {
	key := mustRandBytes(t, 16)
	nonce := mustRandNonce(t)
	plaintext, err := base64.StdEncoding.DecodeString("U00wIEdDTSB0ZXN0IHBheWxvYWQ=")
	if err != nil {
		t.Fatalf("解码测试明文失败: %v", err)
	}
	aad := []byte("sm4-aad")

	provider, err := New(AlgSM4GCM, key)
	if err != nil {
		t.Fatalf("创建 SM4-GCM provider 失败: %v", err)
	}

	ciphertext, err := provider.Encrypt(plaintext, aad, nonce)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	got, err := provider.Decrypt(ciphertext, aad, nonce)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("解密结果与原文不符: got %q, want %q", got, plaintext)
	}
}

// TestNewInvalidKeySize 验证错误密钥长度应返回错误。
func TestNewInvalidKeySize(t *testing.T) {
	// AES-256-GCM 需要 32 字节，传入 16 字节应失败
	if _, err := New(AlgAES256GCM, mustRandBytes(t, 16)); err == nil {
		t.Fatal("AES-256-GCM 使用 16 字节密钥应返回错误")
	}
	// SM4-GCM 需要 16 字节，传入 32 字节应失败
	if _, err := New(AlgSM4GCM, mustRandBytes(t, 32)); err == nil {
		t.Fatal("SM4-GCM 使用 32 字节密钥应返回错误")
	}
	// 不支持的算法应返回错误
	if _, err := New("AES_128_GCM", mustRandBytes(t, 16)); err == nil {
		t.Fatal("不支持的算法应返回错误")
	}
}

// TestNewInvalidNonce 验证错误 nonce 长度应返回错误。
func TestNewInvalidNonce(t *testing.T) {
	key := mustRandBytes(t, 32)
	provider, err := New(AlgAES256GCM, key)
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	plaintext := []byte("nonce size test")
	aad := []byte("aad")

	// 11 字节 nonce（不足 12）
	shortNonce := mustRandBytes(t, 11)
	if _, err := provider.Encrypt(plaintext, aad, shortNonce); err == nil {
		t.Fatal("使用 11 字节 nonce 加密应返回错误")
	}

	// 13 字节 nonce（超过 12）
	longNonce := mustRandBytes(t, 13)
	if _, err := provider.Encrypt(plaintext, aad, longNonce); err == nil {
		t.Fatal("使用 13 字节 nonce 加密应返回错误")
	}

	// 解密时错误 nonce 也应失败
	ciphertext, err := provider.Encrypt(plaintext, aad, mustRandNonce(t))
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if _, err := provider.Decrypt(ciphertext, aad, shortNonce); err == nil {
		t.Fatal("使用 11 字节 nonce 解密应返回错误")
	}
}

// TestAES256GCMEmptyPlaintext 验证空明文也能正确加解密。
func TestAES256GCMEmptyPlaintext(t *testing.T) {
	key := mustRandBytes(t, 32)
	nonce := mustRandNonce(t)
	provider, err := New(AlgAES256GCM, key)
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	// 空明文加密后应仅包含 GCM tag（16 字节）
	ciphertext, err := provider.Encrypt(nil, nil, nonce)
	if err != nil {
		t.Fatalf("加密空明文失败: %v", err)
	}
	if len(ciphertext) != 16 {
		t.Fatalf("空明文密文长度应为 16（GCM tag），got %d", len(ciphertext))
	}

	got, err := provider.Decrypt(ciphertext, nil, nonce)
	if err != nil {
		t.Fatalf("解密空明文失败: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("解密空明文结果应为空，got %d 字节", len(got))
	}

	// 验证返回的错误是普通 error（非 panic）
	_, err = New(AlgAES256GCM, []byte{1, 2, 3})
	if err == nil {
		t.Fatal("短密钥应返回错误")
	}
	if !errors.Is(err, err) {
		t.Fatal("返回的错误应实现 error 接口")
	}
}
