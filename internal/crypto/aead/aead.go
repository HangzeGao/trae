// Package aead 提供 AEAD（认证加密）加解密抽象与实现。
//
// 当前支持两种算法套件：
//   - AES_256_GCM：使用 crypto/aes + crypto/cipher，密钥 32 字节
//   - SM4_128_GCM：使用 github.com/emmansun/gmsm/sm4，密钥 16 字节
//
// 两种套件均使用 12 字节 nonce（GCM 标准）。
package aead

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"

	"github.com/emmansun/gmsm/sm4"
)

// 算法名常量，与 internal/domain/key 中的定义保持一致。
const (
	AlgAES256GCM = "AES_256_GCM"
	AlgSM4GCM    = "SM4_128_GCM"
)

// GCMNonceSize 是 GCM 标准要求的 nonce 长度（12 字节）。
const GCMNonceSize = 12

// 密钥长度常量。
const (
	aes256KeySize = 32
	sm4KeySize    = 16
)

// Provider 是 AEAD 加解密提供者接口。
type Provider interface {
	// Encrypt 使用给定 nonce 和 aad 加密 plaintext，返回密文（含认证 tag）。
	Encrypt(plaintext, aad, nonce []byte) (ciphertext []byte, err error)
	// Decrypt 使用给定 nonce 和 aad 解密 ciphertext，返回明文。
	// 若认证失败则返回错误。
	Decrypt(ciphertext, aad, nonce []byte) (plaintext []byte, err error)
}

// aesGCMProvider 基于 AES-GCM 的实现。
type aesGCMProvider struct {
	aead cipher.AEAD
}

// Encrypt 使用 AES-GCM 加密。
func (p *aesGCMProvider) Encrypt(plaintext, aad, nonce []byte) ([]byte, error) {
	if len(nonce) != GCMNonceSize {
		return nil, fmt.Errorf("aead: invalid nonce size: got %d, want %d", len(nonce), GCMNonceSize)
	}
	return p.aead.Seal(nil, nonce, plaintext, aad), nil
}

// Decrypt 使用 AES-GCM 解密。
func (p *aesGCMProvider) Decrypt(ciphertext, aad, nonce []byte) ([]byte, error) {
	if len(nonce) != GCMNonceSize {
		return nil, fmt.Errorf("aead: invalid nonce size: got %d, want %d", len(nonce), GCMNonceSize)
	}
	return p.aead.Open(nil, nonce, ciphertext, aad)
}

// sm4GCMProvider 基于 SM4-GCM 的实现。
type sm4GCMProvider struct {
	aead cipher.AEAD
}

// Encrypt 使用 SM4-GCM 加密。
func (p *sm4GCMProvider) Encrypt(plaintext, aad, nonce []byte) ([]byte, error) {
	if len(nonce) != GCMNonceSize {
		return nil, fmt.Errorf("aead: invalid nonce size: got %d, want %d", len(nonce), GCMNonceSize)
	}
	return p.aead.Seal(nil, nonce, plaintext, aad), nil
}

// Decrypt 使用 SM4-GCM 解密。
func (p *sm4GCMProvider) Decrypt(ciphertext, aad, nonce []byte) ([]byte, error) {
	if len(nonce) != GCMNonceSize {
		return nil, fmt.Errorf("aead: invalid nonce size: got %d, want %d", len(nonce), GCMNonceSize)
	}
	return p.aead.Open(nil, nonce, ciphertext, aad)
}

// New 根据算法名构造 AEAD Provider。
// AES_256_GCM 需要 32 字节密钥；SM4_128_GCM 需要 16 字节密钥。
func New(algorithm string, key []byte) (Provider, error) {
	switch algorithm {
	case AlgAES256GCM:
		if len(key) != aes256KeySize {
			return nil, fmt.Errorf("aead: AES-256-GCM key must be %d bytes, got %d", aes256KeySize, len(key))
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("aead: create AES cipher: %w", err)
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("aead: create AES-GCM: %w", err)
		}
		return &aesGCMProvider{aead: gcm}, nil
	case AlgSM4GCM:
		if len(key) != sm4KeySize {
			return nil, fmt.Errorf("aead: SM4-GCM key must be %d bytes, got %d", sm4KeySize, len(key))
		}
		block, err := sm4.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("aead: create SM4 cipher: %w", err)
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("aead: create SM4-GCM: %w", err)
		}
		return &sm4GCMProvider{aead: gcm}, nil
	default:
		return nil, fmt.Errorf("aead: unsupported algorithm: %s", algorithm)
	}
}
