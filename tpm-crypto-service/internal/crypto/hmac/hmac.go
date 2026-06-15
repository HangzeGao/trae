// Package hmac 提供 CBC/CFB 等非 AEAD 模式的 encrypt-then-MAC 完整性保护。
package hmac

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
)

// DeriveSubKeys 从基础 key 派生出 K_enc / K_mac。
// 使用 HKDF 思路:HMAC-SHA256 的链式派生,固定 info 以保证确定性。
func DeriveSubKeys(baseKey []byte) (kEnc, kMac []byte) {
	mac := hmac.New(sha256.New, baseKey)
	mac.Write([]byte("tpm-crypto/enc/v1"))
	kEnc = mac.Sum(nil)
	mac.Reset()
	mac.Write([]byte("tpm-crypto/mac/v1"))
	kMac = mac.Sum(nil)
	return
}

// Compute 计算 message 的 HMAC-SHA256(用 K_mac)。
func Compute(kMac, message []byte) []byte {
	mac := hmac.New(sha256.New, kMac)
	mac.Write(message)
	return mac.Sum(nil)
}

// Verify 恒定时间比较;长度不匹配时立即返回 false。
func Verify(kMac, message, expected []byte) error {
	got := Compute(kMac, message)
	if subtle.ConstantTimeCompare(got, expected) != 1 {
		return errors.New("hmac: verification failed")
	}
	return nil
}
