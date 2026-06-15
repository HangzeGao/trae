// Package sm4 实现国密 SM4 算法的所有分组模式,与 AES 接口一致。
//
// 实现要点:
//   - GCM:调用 gmsm.Sm4GCM,解密时本地校验 tag(因 gmsm Sm4GCM 不自带校验)
//   - CTR/CFB/OFB:基于 cipher.Block 接口构造流加密器
//   - CBC:与 AES 相同,使用 PKCS#7 填充 + encrypt-then-MAC
//   - ECB:仅支持 ≤1 块(16B)
package sm4

import (
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/tjfoc/gmsm/sm4"

	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto/common"
	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto/hmac"
)

type sm4Cipher struct {
	mode common.BlockMode
}

func New(mode common.BlockMode) (common.Cipher, error) {
	if err := common.Supports(common.SM4, mode, 16); err != nil {
		return nil, err
	}
	return &sm4Cipher{mode: mode}, nil
}

func (s *sm4Cipher) Algorithm() common.Algorithm { return common.SM4 }
func (s *sm4Cipher) Mode() common.BlockMode      { return s.mode }
func (s *sm4Cipher) KeyLength() int              { return 16 }

func (s *sm4Cipher) newBlock(key []byte) (cipher.Block, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("%w: SM4 key must be 16 bytes", common.ErrInvalidArgument)
	}
	return sm4.NewCipher(key)
}

func (s *sm4Cipher) Encrypt(key, plain, iv, aad []byte) (ciphertext, ivOut, tag []byte, err error) {
	if err = common.ValidateECBPayload(s.mode, plain); err != nil {
		return nil, nil, nil, err
	}
	switch s.mode {
	case common.GCM:
		return s.encryptGCM(key, plain, iv, aad)
	case common.CTR:
		return s.encryptStream(key, plain, iv, cipher.NewCTR)
	case common.CFB:
		return s.encryptStream(key, plain, iv, cipher.NewCFBEncrypter)
	case common.OFB:
		return s.encryptStream(key, plain, iv, cipher.NewOFB)
	case common.CBC:
		return s.encryptCBCWithMAC(key, plain, iv, aad)
	case common.ECB:
		return s.encryptECB(key, plain)
	}
	return nil, nil, nil, fmt.Errorf("%w: SM4 mode %s", common.ErrUnsupported, s.mode)
}

func (s *sm4Cipher) Decrypt(key, ciphertext, iv, tag, aad []byte) ([]byte, error) {
	if err := common.ValidateECBPayload(s.mode, ciphertext); err != nil {
		return nil, err
	}
	switch s.mode {
	case common.GCM:
		return s.decryptGCM(key, ciphertext, iv, tag, aad)
	case common.CTR:
		return s.decryptStream(key, ciphertext, iv, cipher.NewCTR)
	case common.CFB:
		return s.decryptStream(key, ciphertext, iv, cipher.NewCFBDecrypter)
	case common.OFB:
		return s.decryptStream(key, ciphertext, iv, cipher.NewOFB)
	case common.CBC:
		return s.decryptCBCWithMAC(key, ciphertext, iv, tag, aad)
	case common.ECB:
		return s.decryptECB(key, ciphertext)
	}
	return nil, fmt.Errorf("%w: SM4 mode %s", common.ErrUnsupported, s.mode)
}

func (s *sm4Cipher) encryptGCM(key, plain, iv, aad []byte) (ciphertext, ivOut, tag []byte, err error) {
	if len(iv) == 0 {
		iv = make([]byte, 12)
		if _, err = io.ReadFull(rand.Reader, iv); err != nil {
			return nil, nil, nil, err
		}
	}
	if len(iv) != 12 {
		return nil, nil, nil, fmt.Errorf("%w: SM4-GCM IV must be 12 bytes", common.ErrInvalidArgument)
	}
	ct, t, err := sm4.Sm4GCM(key, iv, plain, aad, true)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: sm4 gcm: %v", common.ErrUnsupported, err)
	}
	return ct, iv, t, nil
}

func (s *sm4Cipher) decryptGCM(key, ciphertext, iv, tag, aad []byte) ([]byte, error) {
	if len(iv) != 12 {
		return nil, fmt.Errorf("%w: SM4-GCM IV must be 12 bytes", common.ErrInvalidArgument)
	}
	if len(tag) != 16 {
		return nil, fmt.Errorf("%w: SM4-GCM tag must be 16 bytes", common.ErrInvalidArgument)
	}
	// 注意:gmsm 的 Sm4GCM decrypt 路径只接受 C(不含 tag),自行计算 _T。
	pt, computedTag, err := sm4.Sm4GCM(key, iv, ciphertext, aad, false)
	if err != nil {
		return nil, fmt.Errorf("%w: sm4 gcm: %v", common.ErrAuth, err)
	}
	if !constantTimeEqual(computedTag, tag) {
		for i := range pt {
			pt[i] = 0
		}
		return nil, fmt.Errorf("%w: sm4 gcm tag mismatch", common.ErrAuth)
	}
	return pt, nil
}

func constantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

type streamFactory func(block cipher.Block, iv []byte) cipher.Stream

func (s *sm4Cipher) encryptStream(key, plain, iv []byte, sf streamFactory) (ciphertext, ivOut, tag []byte, err error) {
	block, err := s.newBlock(key)
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
	st := sf(block, iv)
	ct := make([]byte, len(plain))
	st.XORKeyStream(ct, plain)
	return ct, iv, nil, nil
}

func (s *sm4Cipher) decryptStream(key, ciphertext, iv []byte, sf streamFactory) ([]byte, error) {
	block, err := s.newBlock(key)
	if err != nil {
		return nil, err
	}
	if len(iv) != block.BlockSize() {
		return nil, fmt.Errorf("%w: iv size must be %d", common.ErrInvalidArgument, block.BlockSize())
	}
	st := sf(block, iv)
	pt := make([]byte, len(ciphertext))
	st.XORKeyStream(pt, ciphertext)
	return pt, nil
}

func (s *sm4Cipher) encryptCBCWithMAC(key, plain, iv, aad []byte) (ciphertext, ivOut, tag []byte, err error) {
	block, err := s.newBlock(key)
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
	tagInput := append(append(append([]byte{}, iv...), ct...), aad...)
	return ct, iv, hmac.Compute(kMac, tagInput), nil
}

func (s *sm4Cipher) decryptCBCWithMAC(key, ciphertext, iv, tag, aad []byte) ([]byte, error) {
	block, err := s.newBlock(key)
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
	tagInput := append(append(append([]byte{}, iv...), ciphertext...), aad...)
	if err := hmac.Verify(kMac, tagInput, tag); err != nil {
		return nil, fmt.Errorf("%w: %v", common.ErrAuth, err)
	}
	pt := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(pt, ciphertext)
	return pkcs7Unpad(pt, block.BlockSize())
}

func (s *sm4Cipher) encryptECB(key, plain []byte) (ciphertext, ivOut, tag []byte, err error) {
	block, err := s.newBlock(key)
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

func (s *sm4Cipher) decryptECB(key, ciphertext []byte) ([]byte, error) {
	block, err := s.newBlock(key)
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
