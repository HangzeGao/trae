// Package aes 实现 AES 算法的所有分组模式。
//
//  - GCM:标准 AEAD,直接调用 crypto/aes + cipher.NewGCM
//  - CTR/CFB/OFB:流模式,无需填充
//  - CBC:PKCS#7 填充,且必须配 HMAC(encrypt-then-MAC)
//  - ECB:仅支持 ≤1 块(16B),不推荐,仅供迁移期
package aes

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto/common"
	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto/hmac"
)

type aesCipher struct {
	alg   common.Algorithm
	mode  common.BlockMode
	keyLn int
}

// New 构造一个 AES Cipher,keyLen 单位为字节(16/24/32)。
func New(mode common.BlockMode, keyLen int) (common.Cipher, error) {
	if err := common.Supports(common.AES, mode, keyLen); err != nil {
		return nil, err
	}
	return &aesCipher{alg: common.AES, mode: mode, keyLn: keyLen}, nil
}

func (a *aesCipher) Algorithm() common.Algorithm { return a.alg }
func (a *aesCipher) Mode() common.BlockMode      { return a.mode }
func (a *aesCipher) KeyLength() int              { return a.keyLn }

func (a *aesCipher) newBlock(key []byte) (cipher.Block, error) {
	return aes.NewCipher(key)
}

func (a *aesCipher) Encrypt(key, plain, iv, aad []byte) (ciphertext, ivOut, tag []byte, err error) {
	if err = common.ValidateECBPayload(a.mode, plain); err != nil {
		return nil, nil, nil, err
	}
	switch a.mode {
	case common.GCM:
		return a.encryptGCM(key, plain, iv, aad)
	case common.CTR:
		return a.encryptStream(key, plain, iv, cipher.NewCTR)
	case common.CFB:
		return a.encryptStream(key, plain, iv, cipher.NewCFBEncrypter)
	case common.OFB:
		return a.encryptStream(key, plain, iv, cipher.NewOFB)
	case common.CBC:
		return a.encryptCBCWithMAC(key, plain, iv, aad)
	case common.ECB:
		return a.encryptECB(key, plain)
	}
	return nil, nil, nil, fmt.Errorf("%w: AES mode %s", common.ErrUnsupported, a.mode)
}

func (a *aesCipher) Decrypt(key, ciphertext, iv, tag, aad []byte) ([]byte, error) {
	if err := common.ValidateECBPayload(a.mode, ciphertext); err != nil {
		return nil, err
	}
	switch a.mode {
	case common.GCM:
		return a.decryptGCM(key, ciphertext, iv, tag, aad)
	case common.CTR:
		return a.decryptStream(key, ciphertext, iv, cipher.NewCTR)
	case common.CFB:
		return a.decryptStream(key, ciphertext, iv, cipher.NewCFBDecrypter)
	case common.OFB:
		return a.decryptStream(key, ciphertext, iv, cipher.NewOFB)
	case common.CBC:
		return a.decryptCBCWithMAC(key, ciphertext, iv, tag, aad)
	case common.ECB:
		return a.decryptECB(key, ciphertext)
	}
	return nil, fmt.Errorf("%w: AES mode %s", common.ErrUnsupported, a.mode)
}

// --- GCM ---

func (a *aesCipher) encryptGCM(key, plain, iv, aad []byte) (ciphertext, ivOut, tag []byte, err error) {
	block, err := a.newBlock(key)
	if err != nil {
		return nil, nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(iv) == 0 {
		iv = make([]byte, gcm.NonceSize())
		if _, err = io.ReadFull(rand.Reader, iv); err != nil {
			return nil, nil, nil, err
		}
	}
	combined := gcm.Seal(nil, iv, plain, aad)
	if len(combined) < gcm.Overhead() {
		return nil, nil, nil, fmt.Errorf("%w: gcm output too short", common.ErrInvalidArgument)
	}
	tagStart := len(combined) - gcm.Overhead()
	return combined[:tagStart], iv, combined[tagStart:], nil
}

func (a *aesCipher) decryptGCM(key, ciphertext, iv, tag, aad []byte) ([]byte, error) {
	block, err := a.newBlock(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(iv) != gcm.NonceSize() {
		return nil, fmt.Errorf("%w: gcm nonce size mismatch", common.ErrInvalidArgument)
	}
	combined := make([]byte, 0, len(ciphertext)+len(tag))
	combined = append(combined, ciphertext...)
	combined = append(combined, tag...)
	pt, err := gcm.Open(nil, iv, combined, aad)
	if err != nil {
		return nil, fmt.Errorf("%w: gcm open: %v", common.ErrAuth, err)
	}
	return pt, nil
}

// --- Stream (CTR / CFB / OFB) ---

type streamFactory func(block cipher.Block, iv []byte) cipher.Stream

func (a *aesCipher) encryptStream(key, plain, iv []byte, sf streamFactory) (ciphertext, ivOut, tag []byte, err error) {
	block, err := a.newBlock(key)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(iv) == 0 {
		iv = make([]byte, block.BlockSize())
		if _, err = io.ReadFull(rand.Reader, iv); err != nil {
			return nil, nil, nil, err
		}
	} else if len(iv) != block.BlockSize() {
		return nil, nil, nil, fmt.Errorf("%w: iv size must be %d", common.ErrInvalidArgument, block.BlockSize())
	}
	s := sf(block, iv)
	ct := make([]byte, len(plain))
	s.XORKeyStream(ct, plain)
	return ct, iv, nil, nil
}

func (a *aesCipher) decryptStream(key, ciphertext, iv []byte, sf streamFactory) ([]byte, error) {
	block, err := a.newBlock(key)
	if err != nil {
		return nil, err
	}
	if len(iv) != block.BlockSize() {
		return nil, fmt.Errorf("%w: iv size must be %d", common.ErrInvalidArgument, block.BlockSize())
	}
	s := sf(block, iv)
	pt := make([]byte, len(ciphertext))
	s.XORKeyStream(pt, ciphertext)
	return pt, nil
}

// --- CBC + HMAC(encrypt-then-MAC) ---

func (a *aesCipher) encryptCBCWithMAC(key, plain, iv, aad []byte) (ciphertext, ivOut, tag []byte, err error) {
	block, err := a.newBlock(key)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(iv) == 0 {
		iv = make([]byte, block.BlockSize())
		if _, err = io.ReadFull(rand.Reader, iv); err != nil {
			return nil, nil, nil, err
		}
	} else if len(iv) != block.BlockSize() {
		return nil, nil, nil, fmt.Errorf("%w: cbc iv must be %d bytes", common.ErrInvalidArgument, block.BlockSize())
	}
	_, kMac := hmac.DeriveSubKeys(key)
	padded := pkcs7Pad(plain, block.BlockSize())
	ct := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, padded)
	tagInput := make([]byte, 0, len(iv)+len(ct)+len(aad))
	tagInput = append(tagInput, iv...)
	tagInput = append(tagInput, ct...)
	tagInput = append(tagInput, aad...)
	return ct, iv, hmac.Compute(kMac, tagInput), nil
}

func (a *aesCipher) decryptCBCWithMAC(key, ciphertext, iv, tag, aad []byte) ([]byte, error) {
	block, err := a.newBlock(key)
	if err != nil {
		return nil, err
	}
	if len(iv) != block.BlockSize() {
		return nil, fmt.Errorf("%w: cbc iv must be %d bytes", common.ErrInvalidArgument, block.BlockSize())
	}
	if len(ciphertext)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("%w: cbc ciphertext not aligned", common.ErrInvalidArgument)
	}
	_, kMac := hmac.DeriveSubKeys(key)
	tagInput := make([]byte, 0, len(iv)+len(ciphertext)+len(aad))
	tagInput = append(tagInput, iv...)
	tagInput = append(tagInput, ciphertext...)
	tagInput = append(tagInput, aad...)
	if err := hmac.Verify(kMac, tagInput, tag); err != nil {
		return nil, fmt.Errorf("%w: %v", common.ErrAuth, err)
	}
	pt := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(pt, ciphertext)
	return pkcs7Unpad(pt, block.BlockSize())
}

// --- ECB(严格限制 ≤1 块) ---

func (a *aesCipher) encryptECB(key, plain []byte) (ciphertext, ivOut, tag []byte, err error) {
	block, err := a.newBlock(key)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(plain) > block.BlockSize() {
		return nil, nil, nil, fmt.Errorf("%w: ECB >1 block refused", common.ErrInvalidArgument)
	}
	pad := pkcs7Pad(plain, block.BlockSize())
	ct := make([]byte, block.BlockSize())
	block.Encrypt(ct, pad)
	return ct, nil, nil, nil
}

func (a *aesCipher) decryptECB(key, ciphertext []byte) ([]byte, error) {
	block, err := a.newBlock(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) != block.BlockSize() {
		return nil, fmt.Errorf("%w: ECB ciphertext must be exactly 1 block", common.ErrInvalidArgument)
	}
	pt := make([]byte, block.BlockSize())
	block.Decrypt(pt, ciphertext)
	return pkcs7Unpad(pt, block.BlockSize())
}

// --- PKCS#7 ---

func pkcs7Pad(b []byte, blockSize int) []byte {
	pad := blockSize - len(b)%blockSize
	out := make([]byte, len(b)+pad)
	copy(out, b)
	for i := len(b); i < len(out); i++ {
		out[i] = byte(pad)
	}
	return out
}

func pkcs7Unpad(b []byte, blockSize int) ([]byte, error) {
	if len(b) == 0 || len(b)%blockSize != 0 {
		return nil, fmt.Errorf("%w: invalid padded length", common.ErrInvalidArgument)
	}
	pad := int(b[len(b)-1])
	if pad <= 0 || pad > blockSize {
		return nil, fmt.Errorf("%w: invalid pad value", common.ErrInvalidArgument)
	}
	for i := len(b) - pad; i < len(b); i++ {
		if int(b[i]) != pad {
			return nil, fmt.Errorf("%w: pad mismatch", common.ErrInvalidArgument)
		}
	}
	return b[:len(b)-pad], nil
}
