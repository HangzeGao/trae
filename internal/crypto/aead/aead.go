// Package aead wraps AES-GCM and SM4-GCM behind a uniform AEAD interface.
// Per design §8.4, basic algorithms must come from mature libraries;
// AES uses Go standard library; SM4 uses github.com/emmansun/gmsm.
package aead

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"

	"github.com/emmansun/gmsm/sm4"
)

// SuiteID identifies an algorithm suite. Values are stable and part of
// the Envelope v1 wire format (design §8.3, §8.1).
type SuiteID uint16

const (
	SuiteAES256GCM         SuiteID = 0x0001
	SuiteSM4GCM            SuiteID = 0x0002
	SuiteAES256CBCHMACSHA256 SuiteID = 0x0003 // decrypt-only in P0
	SuiteSM4CBCHMACSM3     SuiteID = 0x0004 // decrypt-only in P0
)

func (s SuiteID) String() string {
	switch s {
	case SuiteAES256GCM:
		return "AES_256_GCM"
	case SuiteSM4GCM:
		return "SM4_GCM"
	case SuiteAES256CBCHMACSHA256:
		return "AES_256_CBC_HMAC_SHA256"
	case SuiteSM4CBCHMACSM3:
		return "SM4_CBC_HMAC_SM3"
	default:
		return fmt.Sprintf("SUITE_0x%04X", uint16(s))
	}
}

// KeyBits returns the key length in bits for the suite.
func (s SuiteID) KeyBits() int {
	switch s {
	case SuiteAES256GCM, SuiteAES256CBCHMACSHA256:
		return 256
	case SuiteSM4GCM, SuiteSM4CBCHMACSM3:
		return 128
	default:
		return 0
	}
}

// KeyBytes returns the key length in bytes.
func (s SuiteID) KeyBytes() int { return s.KeyBits() / 8 }

// NonceLen returns the GCM nonce length (12 bytes for both AES-GCM and SM4-GCM).
func (s SuiteID) NonceLen() int {
	switch s {
	case SuiteAES256GCM, SuiteSM4GCM:
		return 12
	case SuiteAES256CBCHMACSHA256, SuiteSM4CBCHMACSM3:
		return 16 // IV
	}
	return 0
}

// TagLen returns the AEAD tag length.
func (s SuiteID) TagLen() int {
	switch s {
	case SuiteAES256GCM, SuiteSM4GCM:
		return 16
	case SuiteAES256CBCHMACSHA256:
		return 32
	case SuiteSM4CBCHMACSM3:
		return 32
	}
	return 0
}

// AEAD is the uniform interface for AEAD ciphers used by the envelope.
type AEAD interface {
	Encrypt(plaintext, nonce, aad []byte) (ciphertext, tag []byte)
	Decrypt(ciphertext, tag, nonce, aad []byte) ([]byte, error)
	NonceSize() int
	TagSize() int
}

// New constructs an AEAD for the given suite and key.
func New(suite SuiteID, key []byte) (AEAD, error) {
	if len(key) != suite.KeyBytes() {
		return nil, fmt.Errorf("aead: key length %d does not match suite %s (%d)",
			len(key), suite, suite.KeyBytes())
	}
	switch suite {
	case SuiteAES256GCM:
		b, err := aes.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("aead: aes new: %w", err)
		}
		g, err := cipher.NewGCMWithNonceSize(b, 12)
		if err != nil {
			return nil, fmt.Errorf("aead: gcm new: %w", err)
		}
		return &gcmAEAD{g: g}, nil
	case SuiteSM4GCM:
		b, err := sm4.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("aead: sm4 new: %w", err)
		}
		g, err := cipher.NewGCMWithNonceSize(b, 12)
		if err != nil {
			return nil, fmt.Errorf("aead: sm4 gcm new: %w", err)
		}
		return &gcmAEAD{g: g}, nil
	default:
		return nil, fmt.Errorf("aead: suite %s not supported for new encryption in P0", suite)
	}
}

type gcmAEAD struct {
	g cipher.AEAD
}

func (a *gcmAEAD) Encrypt(plaintext, nonce, aad []byte) (ciphertext, tag []byte) {
	// Use Seal with dst=nil to get ciphertext||tag, then split.
	out := a.g.Seal(nil, nonce, plaintext, aad)
	tagLen := a.g.Overhead()
	if len(out) < tagLen {
		// Should never happen.
		panic("aead: seal output shorter than tag")
	}
	return out[:len(out)-tagLen], out[len(out)-tagLen:]
}

func (a *gcmAEAD) Decrypt(ciphertext, tag, nonce, aad []byte) ([]byte, error) {
	if len(tag) != a.g.Overhead() {
		return nil, fmt.Errorf("aead: tag length mismatch")
	}
	combined := make([]byte, 0, len(ciphertext)+len(tag))
	combined = append(combined, ciphertext...)
	combined = append(combined, tag...)
	return a.g.Open(nil, nonce, combined, aad)
}

func (a *gcmAEAD) NonceSize() int { return a.g.NonceSize() }
func (a *gcmAEAD) TagSize() int   { return a.g.Overhead() }
