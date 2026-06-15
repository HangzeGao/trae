// Package common 定义通用密码学抽象:算法、分组模式、Cipher 接口。
package common

import (
	"crypto/cipher"
	"errors"
	"fmt"
)

// Algorithm 标识对称算法族。
type Algorithm uint8

const (
	AlgUnknown Algorithm = 0
	AES        Algorithm = 1
	SM4        Algorithm = 2
)

func (a Algorithm) String() string {
	switch a {
	case AES:
		return "AES"
	case SM4:
		return "SM4"
	default:
		return "UNKNOWN"
	}
}

func ParseAlgorithm(s string) (Algorithm, error) {
	switch s {
	case "AES", "aes":
		return AES, nil
	case "SM4", "sm4":
		return SM4, nil
	default:
		return AlgUnknown, fmt.Errorf("unknown algorithm %q", s)
	}
}

// BlockMode 标识分组模式。
type BlockMode uint8

const (
	ModeUnknown BlockMode = 0
	ECB         BlockMode = 1
	CBC         BlockMode = 2
	CTR         BlockMode = 3
	CFB         BlockMode = 4
	OFB         BlockMode = 5
	GCM         BlockMode = 6
)

func (m BlockMode) String() string {
	switch m {
	case ECB:
		return "ECB"
	case CBC:
		return "CBC"
	case CTR:
		return "CTR"
	case CFB:
		return "CFB"
	case OFB:
		return "OFB"
	case GCM:
		return "GCM"
	default:
		return "UNKNOWN"
	}
}

func ParseBlockMode(s string) (BlockMode, error) {
	switch s {
	case "ECB", "ecb":
		return ECB, nil
	case "CBC", "cbc":
		return CBC, nil
	case "CTR", "ctr":
		return CTR, nil
	case "CFB", "cfb":
		return CFB, nil
	case "OFB", "ofb":
		return OFB, nil
	case "GCM", "gcm":
		return GCM, nil
	default:
		return ModeUnknown, fmt.Errorf("unknown block mode %q", s)
	}
}

// KeyStatus 数据密钥状态。
type KeyStatus uint8

const (
	StatusUnknown KeyStatus = 0
	Active        KeyStatus = 1
	Retired       KeyStatus = 2
	Revoked       KeyStatus = 3
)

func (s KeyStatus) String() string {
	switch s {
	case Active:
		return "ACTIVE"
	case Retired:
		return "RETIRED"
	case Revoked:
		return "REVOKED"
	default:
		return "UNKNOWN"
	}
}

// Errors 业务层错误。
var (
	ErrUnsupported     = errors.New("crypto: unsupported")
	ErrInvalidArgument = errors.New("crypto: invalid argument")
	ErrAuth            = errors.New("crypto: authentication failed")
	ErrNotFound        = errors.New("crypto: not found")
	ErrKeyIDExists     = errors.New("crypto: key id already exists")
	ErrKeyRevoked      = errors.New("crypto: key has been revoked")
)

// Cipher 是算法+模式的统一对外接口。
//  - 对于 AEAD(GCM):Encrypt 返回 ciphertext、iv/nonce、tag;Decrypt 校验 tag。
//  - 对于非 AEAD(CBC/CTR/...):Encrypt 返回 ciphertext、iv(若调用方未传则内部生成);若模式带 HMAC,返回 mac 作为 tag。
//  - 对于 ECB:仅允许 ≤1 块(16 字节)数据,否则返回 ErrInvalidArgument。
type Cipher interface {
	Algorithm() Algorithm
	Mode() BlockMode
	KeyLength() int // 字节
	Encrypt(key, plain, iv, aad []byte) (ciphertext, ivOut, tag []byte, err error)
	Decrypt(key, ciphertext, iv, tag, aad []byte) (plaintext []byte, err error)
}

// AEADLike 暴露 AEAD 接口(若 Cipher 是 GCM 模式)。
type AEADLike interface {
	AEAD() cipher.AEAD
}

// Key 描述一个具体算法/模式的元信息。
type Key struct {
	Alg    Algorithm
	Mode   BlockMode
	Length int // 字节
}

// Supports 描述某 (alg, mode, length) 组合是否被支持。
func Supports(alg Algorithm, mode BlockMode, keyLen int) error {
	if alg != AES && alg != SM4 {
		return fmt.Errorf("%w: algorithm %s", ErrUnsupported, alg)
	}
	if alg == SM4 && keyLen != 16 {
		return fmt.Errorf("%w: SM4 key must be 128-bit, got %d", ErrUnsupported, keyLen*8)
	}
	if alg == AES && keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return fmt.Errorf("%w: AES key must be 128/192/256-bit, got %d", ErrUnsupported, keyLen*8)
	}
	switch mode {
	case ECB:
		// 业务层校验,不在此处拒绝
	case CBC, CTR, CFB, OFB, GCM:
	default:
		return fmt.Errorf("%w: mode %s", ErrUnsupported, mode)
	}
	return nil
}

// ValidateECBPayload ECB 模式只允许 ≤1 块。
func ValidateECBPayload(mode BlockMode, plain []byte) error {
	if mode != ECB {
		return nil
	}
	if len(plain) > 16 {
		return fmt.Errorf("%w: ECB mode refuses payloads >16 bytes (got %d)", ErrInvalidArgument, len(plain))
	}
	return nil
}
