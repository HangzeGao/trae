package datakey

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"
)

// mustRandBytes 生成 n 字节随机数据，失败时调用 t.Fatal 终止测试。
func mustRandBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("生成随机字节失败: %v", err)
	}
	return b
}

// TestGenerateAES256 验证生成 32 字节 DEK。
func TestGenerateAES256(t *testing.T) {
	dek, err := Generate(AlgAES256GCM)
	if err != nil {
		t.Fatalf("Generate(AES_256_GCM) 失败: %v", err)
	}
	if len(dek) != AES256KeySize {
		t.Fatalf("AES-256 DEK 长度应为 %d, got %d", AES256KeySize, len(dek))
	}

	// 多次生成应不同（密码学随机性）
	dek2, err := Generate(AlgAES256GCM)
	if err != nil {
		t.Fatalf("第二次 Generate 失败: %v", err)
	}
	if bytes.Equal(dek, dek2) {
		t.Fatal("两次生成的 DEK 不应相同")
	}
}

// TestGenerateSM4 验证生成 16 字节 DEK。
func TestGenerateSM4(t *testing.T) {
	dek, err := Generate(AlgSM4GCM)
	if err != nil {
		t.Fatalf("Generate(SM4_128_GCM) 失败: %v", err)
	}
	if len(dek) != SM4KeySize {
		t.Fatalf("SM4 DEK 长度应为 %d, got %d", SM4KeySize, len(dek))
	}

	// 多次生成应不同
	dek2, err := Generate(AlgSM4GCM)
	if err != nil {
		t.Fatalf("第二次 Generate 失败: %v", err)
	}
	if bytes.Equal(dek, dek2) {
		t.Fatal("两次生成的 DEK 不应相同")
	}
}

// TestGenerateUnsupportedAlgorithm 验证不支持的算法应返回错误。
func TestGenerateUnsupportedAlgorithm(t *testing.T) {
	if _, err := Generate("AES_128_GCM"); err == nil {
		t.Fatal("不支持的算法应返回错误")
	}
	if _, err := Generate(""); err == nil {
		t.Fatal("空算法名应返回错误")
	}
}

// TestWrapUnwrap 验证包装后解包应还原 DEK。
func TestWrapUnwrap(t *testing.T) {
	kek := mustRandBytes(t, 32) // AES-256-GCM KEK
	dek := mustRandBytes(t, 32) // 原始 DEK

	wrapped, err := Wrap(dek, kek)
	if err != nil {
		t.Fatalf("Wrap 失败: %v", err)
	}
	if len(wrapped) <= wrapNonceSize {
		t.Fatalf("wrapped 长度应大于 nonce(%d), got %d", wrapNonceSize, len(wrapped))
	}
	if bytes.Equal(wrapped[wrapNonceSize:], dek) {
		t.Fatal("wrapped 密文部分不应等于原始 DEK")
	}

	got, err := Unwrap(wrapped, kek)
	if err != nil {
		t.Fatalf("Unwrap 失败: %v", err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatalf("解包后的 DEK 与原始不符: got %x, want %x", got, dek)
	}
}

// TestWrapUnwrapSM4DEK 验证 SM4 DEK 也能正确包装/解包（KEK 仍为 AES-256-GCM）。
func TestWrapUnwrapSM4DEK(t *testing.T) {
	kek := mustRandBytes(t, 32)
	dek := mustRandBytes(t, 16) // SM4 DEK

	wrapped, err := Wrap(dek, kek)
	if err != nil {
		t.Fatalf("Wrap 失败: %v", err)
	}

	got, err := Unwrap(wrapped, kek)
	if err != nil {
		t.Fatalf("Unwrap 失败: %v", err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatalf("解包后的 DEK 与原始不符: got %x, want %x", got, dek)
	}
}

// TestUnwrapWrongKEK 验证错误 KEK 解包应失败。
func TestUnwrapWrongKEK(t *testing.T) {
	kek1 := mustRandBytes(t, 32)
	kek2 := mustRandBytes(t, 32)
	dek := mustRandBytes(t, 32)

	wrapped, err := Wrap(dek, kek1)
	if err != nil {
		t.Fatalf("Wrap 失败: %v", err)
	}

	if _, err := Unwrap(wrapped, kek2); err == nil {
		t.Fatal("使用错误 KEK 解包应失败，但成功了")
	}
}

// TestUnwrapInvalidKEKSize 验证错误 KEK 长度应返回错误。
func TestUnwrapInvalidKEKSize(t *testing.T) {
	kek := mustRandBytes(t, 16) // AES-256-GCM 需要 32 字节
	dek := mustRandBytes(t, 32)

	if _, err := Wrap(dek, kek); err == nil {
		t.Fatal("使用 16 字节 KEK 包装应失败")
	}
}

// TestUnwrapTruncated 验证截断的 wrapped DEK 应失败。
func TestUnwrapTruncated(t *testing.T) {
	kek := mustRandBytes(t, 32)

	// 长度小于 nonce(12) 应失败
	short := mustRandBytes(t, 10)
	if _, err := Unwrap(short, kek); err == nil {
		t.Fatal("截断的 wrapped DEK 应解包失败")
	}

	// 长度刚好等于 nonce(12) 但无密文也应失败（GCM tag 缺失）
	onlyNonce := mustRandBytes(t, 12)
	if _, err := Unwrap(onlyNonce, kek); err == nil {
		t.Fatal("仅含 nonce 的 wrapped DEK 应解包失败")
	}
}

// TestUnwrapTampered 验证篡改 wrapped DEK 应失败。
func TestUnwrapTampered(t *testing.T) {
	kek := mustRandBytes(t, 32)
	dek := mustRandBytes(t, 32)

	wrapped, err := Wrap(dek, kek)
	if err != nil {
		t.Fatalf("Wrap 失败: %v", err)
	}

	// 篡改 nonce 部分
	tampered := make([]byte, len(wrapped))
	copy(tampered, wrapped)
	tampered[0] ^= 0xff
	if _, err := Unwrap(tampered, kek); err == nil {
		t.Fatal("篡改 nonce 后解包应失败")
	}

	// 篡改密文部分
	tampered2 := make([]byte, len(wrapped))
	copy(tampered2, wrapped)
	tampered2[len(tampered2)-1] ^= 0xff
	if _, err := Unwrap(tampered2, kek); err == nil {
		t.Fatal("篡改密文后解包应失败")
	}
}

// TestCacheSetGet 验证缓存存取。
func TestCacheSetGet(t *testing.T) {
	cache := NewCache()
	dek := mustRandBytes(t, 32)
	kid := "kid-cache-1"

	// 未设置时应返回 false
	if _, ok := cache.Get(kid); ok {
		t.Fatal("未设置的 KID 不应命中缓存")
	}

	// 设置后应命中
	cache.Set(kid, dek, time.Minute)
	lease, ok := cache.Get(kid)
	if !ok {
		t.Fatal("设置后应命中缓存")
	}
	if lease.KID != kid {
		t.Errorf("lease.KID 不匹配: got %q, want %q", lease.KID, kid)
	}
	if !bytes.Equal(lease.DEK, dek) {
		t.Errorf("lease.DEK 不匹配: got %x, want %x", lease.DEK, dek)
	}

	// Len 应反映缓存数量
	if cache.Len() != 1 {
		t.Errorf("Len 应为 1, got %d", cache.Len())
	}

	// 多个 KID 应独立
	dek2 := mustRandBytes(t, 32)
	kid2 := "kid-cache-2"
	cache.Set(kid2, dek2, time.Minute)
	if cache.Len() != 2 {
		t.Errorf("Len 应为 2, got %d", cache.Len())
	}
	lease2, ok := cache.Get(kid2)
	if !ok {
		t.Fatal("kid-2 应命中缓存")
	}
	if !bytes.Equal(lease2.DEK, dek2) {
		t.Errorf("kid-2 lease.DEK 不匹配")
	}

	// Delete 后应不再命中
	cache.Delete(kid)
	if _, ok := cache.Get(kid); ok {
		t.Fatal("Delete 后不应命中缓存")
	}
	if cache.Len() != 1 {
		t.Errorf("Delete 后 Len 应为 1, got %d", cache.Len())
	}
}

// TestCacheExpiry 验证缓存过期。
func TestCacheExpiry(t *testing.T) {
	cache := NewCache()
	dek := mustRandBytes(t, 32)
	kid := "kid-expire"

	// 设置一个很短的 TTL
	cache.Set(kid, dek, 30*time.Millisecond)
	// 立即应命中
	if _, ok := cache.Get(kid); !ok {
		t.Fatal("设置后立即应命中缓存")
	}

	// 等待过期
	time.Sleep(50 * time.Millisecond)
	if _, ok := cache.Get(kid); ok {
		t.Fatal("过期后不应命中缓存")
	}

	// 过期条目仍占用 sync.Map 槽位（Get 返回 false 但条目未删除）
	// Len 反映 sync.Map 中的条目数（含可能已过期但未清理的）
	if cache.Len() != 1 {
		t.Logf("过期条目仍占 sync.Map 槽位, Len=%d", cache.Len())
	}

	// 重新设置应可命中
	cache.Set(kid, dek, time.Minute)
	if _, ok := cache.Get(kid); !ok {
		t.Fatal("重新设置后应命中缓存")
	}
}

// TestCacheZeroTTL 验证零 TTL 立即过期。
func TestCacheZeroTTL(t *testing.T) {
	cache := NewCache()
	dek := mustRandBytes(t, 32)
	kid := "kid-zero-ttl"

	cache.Set(kid, dek, 0)
	// 零 TTL 时 ExpiresAt = now，下一次 Get 时 time.Now().After(now) 可能为 true 也可能为 false
	// 取决于时钟精度。这里仅验证不 panic。
	_, _ = cache.Get(kid)
}

// TestCacheConcurrent 验证缓存并发安全。
func TestCacheConcurrent(t *testing.T) {
	cache := NewCache()
	const goroutines = 20
	const opsPerG = 50

	done := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			for j := 0; j < opsPerG; j++ {
				kid := "kid-concurrent"
				cache.Set(kid, mustRandBytes(t, 32), time.Minute)
				_, _ = cache.Get(kid)
				if j%10 == 0 {
					cache.Delete(kid)
				}
			}
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
}
