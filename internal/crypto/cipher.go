package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/tpm2-encryption-system/internal/model"
)

// Cipher 封装了对称加解密操作
type Cipher struct {
	algorithm model.Algorithm
	mode      model.Mode
	key       []byte
}

// NewCipher 根据算法和模式创建 Cipher 实例
func NewCipher(algorithm model.Algorithm, mode model.Mode, key []byte) (*Cipher, error) {
	if !algorithm.IsValid() {
		return nil, model.ErrInvalidAlgorithm
	}
	if !mode.IsValid() {
		return nil, model.ErrInvalidMode
	}
	if len(key) != algorithm.KeyLength() {
		return nil, model.ErrKeyLengthMismatch
	}
	return &Cipher{
		algorithm: algorithm,
		mode:      mode,
		key:       key,
	}, nil
}

// Encrypt 加密数据，返回包含 IV、Tag、密文的 CiphertextFormat
func (c *Cipher) Encrypt(plaintext []byte) (*model.CiphertextFormat, error) {
	iv, err := c.generateIV()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrEncryptFailed, err)
	}

	block, err := c.createBlock()
	if err != nil {
		return nil, err
	}

	ct := &model.CiphertextFormat{
		Algorithm: c.algorithm,
		Mode:      c.mode,
		IV:        iv,
	}

	switch c.mode {
	case model.ModeGCM:
		data, tag, err := c.encryptGCM(block, iv, plaintext)
		if err != nil {
			return nil, err
		}
		ct.Data = data
		ct.Tag = tag
	case model.ModeCBC:
		ct.Data, err = c.encryptCBC(block, iv, plaintext)
		if err != nil {
			return nil, err
		}
	case model.ModeCTR:
		ct.Data, err = c.encryptCTR(block, iv, plaintext)
		if err != nil {
			return nil, err
		}
	case model.ModeCFB:
		ct.Data, err = c.encryptCFB(block, iv, plaintext)
		if err != nil {
			return nil, err
		}
	case model.ModeOFB:
		ct.Data, err = c.encryptOFB(block, iv, plaintext)
		if err != nil {
			return nil, err
		}
	default:
		return nil, model.ErrInvalidMode
	}

	return ct, nil
}

// Decrypt 解密 CiphertextFormat 中的数据
func (c *Cipher) Decrypt(ct *model.CiphertextFormat) ([]byte, error) {
	block, err := c.createBlock()
	if err != nil {
		return nil, err
	}

	switch ct.Mode {
	case model.ModeGCM:
		return c.decryptGCM(block, ct.IV, ct.Data, ct.Tag)
	case model.ModeCBC:
		return c.decryptCBC(block, ct.IV, ct.Data)
	case model.ModeCTR:
		return c.decryptCTR(block, ct.IV, ct.Data)
	case model.ModeCFB:
		return c.decryptCFB(block, ct.IV, ct.Data)
	case model.ModeOFB:
		return c.decryptOFB(block, ct.IV, ct.Data)
	default:
		return nil, model.ErrInvalidMode
	}
}

// createBlock 根据算法创建 cipher.Block
func (c *Cipher) createBlock() (cipher.Block, error) {
	switch c.algorithm {
	case model.AlgorithmAES128, model.AlgorithmAES192, model.AlgorithmAES256:
		block, err := aes.NewCipher(c.key)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", model.ErrEncryptFailed, err)
		}
		return block, nil
	case model.AlgorithmSM4:
		block, err := NewSM4Cipher(c.key)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", model.ErrEncryptFailed, err)
		}
		return block, nil
	default:
		return nil, model.ErrInvalidAlgorithm
	}
}

// generateIV 生成随机 IV/Nonce
func (c *Cipher) generateIV() ([]byte, error) {
	ivSize := c.ivSize()
	iv := make([]byte, ivSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	return iv, nil
}

// ivSize 返回当前模式所需的 IV 长度
func (c *Cipher) ivSize() int {
	switch c.mode {
	case model.ModeGCM:
		return 12 // GCM 标准 nonce 长度
	default:
		return 16 // AES/SM4 块大小
	}
}

// PKCS#7 填充
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := make([]byte, padding)
	for i := range padtext {
		padtext[i] = byte(padding)
	}
	return append(data, padtext...)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, model.ErrDecryptFailed
	}
	if len(data)%blockSize != 0 {
		return nil, model.ErrDecryptFailed
	}
	padding := int(data[len(data)-1])
	if padding > blockSize || padding == 0 {
		return nil, model.ErrDecryptFailed
	}
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, model.ErrDecryptFailed
		}
	}
	return data[:len(data)-padding], nil
}

// GCM 模式
func (c *Cipher) encryptGCM(block cipher.Block, nonce, plaintext []byte) ([]byte, []byte, error) {
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", model.ErrEncryptFailed, err)
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, nil)
	// Seal 返回 ciphertext || tag，需要分离
	tagSize := aead.Overhead()
	if len(ciphertext) < tagSize {
		return nil, nil, model.ErrEncryptFailed
	}
	data := ciphertext[:len(ciphertext)-tagSize]
	tag := ciphertext[len(ciphertext)-tagSize:]
	return data, tag, nil
}

func (c *Cipher) decryptGCM(block cipher.Block, nonce, ciphertext, tag []byte) ([]byte, error) {
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrDecryptFailed, err)
	}
	// 重新组合 ciphertext || tag
	combined := make([]byte, 0, len(ciphertext)+len(tag))
	combined = append(combined, ciphertext...)
	combined = append(combined, tag...)
	plaintext, err := aead.Open(nil, nonce, combined, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrAuthFailed, err)
	}
	return plaintext, nil
}

// CBC 模式
func (c *Cipher) encryptCBC(block cipher.Block, iv, plaintext []byte) ([]byte, error) {
	padded := pkcs7Pad(plaintext, block.BlockSize())
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)
	return ciphertext, nil
}

func (c *Cipher) decryptCBC(block cipher.Block, iv, ciphertext []byte) ([]byte, error) {
	if len(ciphertext)%block.BlockSize() != 0 {
		return nil, model.ErrDecryptFailed
	}
	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)
	return pkcs7Unpad(plaintext, block.BlockSize())
}

// CTR 模式
func (c *Cipher) encryptCTR(block cipher.Block, iv, plaintext []byte) ([]byte, error) {
	ciphertext := make([]byte, len(plaintext))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext, plaintext)
	return ciphertext, nil
}

func (c *Cipher) decryptCTR(block cipher.Block, iv, ciphertext []byte) ([]byte, error) {
	return c.encryptCTR(block, iv, ciphertext) // CTR 模式加解密相同
}

// CFB 模式
func (c *Cipher) encryptCFB(block cipher.Block, iv, plaintext []byte) ([]byte, error) {
	ciphertext := make([]byte, len(plaintext))
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext, plaintext)
	return ciphertext, nil
}

func (c *Cipher) decryptCFB(block cipher.Block, iv, ciphertext []byte) ([]byte, error) {
	plaintext := make([]byte, len(ciphertext))
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(plaintext, ciphertext)
	return plaintext, nil
}

// OFB 模式
func (c *Cipher) encryptOFB(block cipher.Block, iv, plaintext []byte) ([]byte, error) {
	ciphertext := make([]byte, len(plaintext))
	stream := cipher.NewOFB(block, iv)
	stream.XORKeyStream(ciphertext, plaintext)
	return ciphertext, nil
}

func (c *Cipher) decryptOFB(block cipher.Block, iv, ciphertext []byte) ([]byte, error) {
	return c.encryptOFB(block, iv, ciphertext) // OFB 模式加解密相同
}
