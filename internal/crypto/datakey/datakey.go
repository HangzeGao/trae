// Package datakey 提供 DEK（数据加密密钥）的生成、包装/解包与缓存。
//
// DEK 用于实际数据加解密，由 KEK（密钥加密密钥，AES-256-GCM）包装后存储。
// Cache 提供 per-KID 的 DEK 租约缓存，带 TTL 过期检查，避免频繁解包。
package datakey

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/HangzeGao/trae/key-vault/internal/crypto/aead"
	"github.com/HangzeGao/trae/key-vault/internal/errors"
)

// 算法名常量，与 internal/domain/key 中的定义保持一致。
const (
	AlgAES256GCM = "AES_256_GCM"
	AlgSM4GCM    = "SM4_128_GCM"
)

// DEK 密钥长度。
const (
	AES256KeySize = 32 // AES-256-GCM 密钥长度
	SM4KeySize    = 16 // SM4-128-GCM 密钥长度
)

// wrapNonceSize 是 DEK 包装使用的 nonce 长度（GCM 标准 12 字节）。
const wrapNonceSize = 12

// Generate 生成指定算法的随机 DEK。
// AES_256_GCM 生成 32 字节，SM4_128_GCM 生成 16 字节。
// 使用 crypto/rand 保证密码学安全随机性。
func Generate(algorithm string) ([]byte, error) {
	var size int
	switch algorithm {
	case AlgAES256GCM:
		size = AES256KeySize
	case AlgSM4GCM:
		size = SM4KeySize
	default:
		return nil, fmt.Errorf("datakey: unsupported algorithm: %s", algorithm)
	}
	dek := make([]byte, size)
	if _, err := rand.Read(dek); err != nil {
		return nil, fmt.Errorf("datakey: generate DEK: %w", err)
	}
	return dek, nil
}

// Wrap 使用 KEK（AES-256-GCM）包装 DEK。
// 返回格式: nonce(12) || ciphertext（含 GCM tag）。
// KEK 必须为 32 字节（AES-256-GCM 密钥）。
func Wrap(dek []byte, kek []byte) ([]byte, error) {
	provider, err := aead.New(AlgAES256GCM, kek)
	if err != nil {
		return nil, fmt.Errorf("datakey: create KEK provider: %w", err)
	}
	nonce := make([]byte, wrapNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("datakey: generate wrap nonce: %w", err)
	}
	// DEK 包装不绑定额外 AAD，DEK 本身即为加密内容
	ciphertext, err := provider.Encrypt(dek, nil, nonce)
	if err != nil {
		return nil, fmt.Errorf("datakey: wrap DEK: %w", err)
	}
	wrapped := make([]byte, 0, wrapNonceSize+len(ciphertext))
	wrapped = append(wrapped, nonce...)
	wrapped = append(wrapped, ciphertext...)
	return wrapped, nil
}

// Unwrap 使用 KEK（AES-256-GCM）解包 DEK。
// 输入格式: nonce(12) || ciphertext。
// 若认证失败或格式不合法，返回错误。
func Unwrap(wrappedDEK []byte, kek []byte) ([]byte, error) {
	if len(wrappedDEK) < wrapNonceSize {
		return nil, errors.EnvelopeInvalid(fmt.Errorf("datakey: wrapped DEK too short: got %d, want >= %d", len(wrappedDEK), wrapNonceSize))
	}
	provider, err := aead.New(AlgAES256GCM, kek)
	if err != nil {
		return nil, fmt.Errorf("datakey: create KEK provider: %w", err)
	}
	nonce := wrappedDEK[:wrapNonceSize]
	ciphertext := wrappedDEK[wrapNonceSize:]
	plaintext, err := provider.Decrypt(ciphertext, nil, nonce)
	if err != nil {
		return nil, fmt.Errorf("datakey: unwrap DEK: %w", err)
	}
	return plaintext, nil
}

// Lease 是缓存的 DEK 租约。
type Lease struct {
	KID       string    // 关联的 key ID
	DEK       []byte    // 明文 DEK
	ExpiresAt time.Time // 过期时间
}

// Cache 是 per-KID 的 DEK 缓存，带 TTL 过期检查。
// 使用 sync.Map 实现并发安全访问。
type Cache struct {
	leases sync.Map // map[string]*Lease
}

// NewCache 创建 DEK 缓存。
func NewCache() *Cache {
	return &Cache{}
}

// Get 获取指定 KID 的 DEK lease。
// 若不存在或已过期返回 nil, false。
func (c *Cache) Get(kid string) (*Lease, bool) {
	v, ok := c.leases.Load(kid)
	if !ok {
		return nil, false
	}
	lease := v.(*Lease)
	if time.Now().After(lease.ExpiresAt) {
		return nil, false
	}
	return lease, true
}

// Set 设置指定 KID 的 DEK lease，带 TTL。
func (c *Cache) Set(kid string, dek []byte, ttl time.Duration) {
	c.leases.Store(kid, &Lease{
		KID:       kid,
		DEK:       dek,
		ExpiresAt: time.Now().Add(ttl),
	})
}

// Delete 删除指定 KID 的 DEK lease。
func (c *Cache) Delete(kid string) {
	c.leases.Delete(kid)
}

// Len 返回缓存中的 lease 数量（含可能已过期但未清理的条目）。
func (c *Cache) Len() int {
	count := 0
	c.leases.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}
