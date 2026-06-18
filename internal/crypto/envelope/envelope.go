package envelope

import (
	"encoding/binary"
	"fmt"

	"github.com/HangzeGao/trae/key-vault/internal/errors"
)

// 信封魔数与版本。
const (
	// Magic 是信封魔数 "KV01"。
	Magic = "KV01"
	// Version 是信封格式版本。
	Version byte = 1
)

// Suite ID 常量，标识加密算法套件。
const (
	SuiteAES256GCM byte = 0x01 // AES_256_GCM
	SuiteSM4GCM    byte = 0x02 // SM4_128_GCM
)

// 算法名常量，与 internal/domain/key 中的定义保持一致。
const (
	AlgAES256GCM = "AES_256_GCM"
	AlgSM4GCM    = "SM4_128_GCM"
)

// 字段长度限制。
const (
	maxDEKKIDLen   = 0xFFFF   // 2 字节
	maxNonceLen    = 0xFF     // 1 字节
	maxCiphertextLen = 0xFFFFFFFF // 4 字节
	maxAADLen      = 0xFFFFFFFF // 4 字节
)

// 信封各固定字段长度。
const (
	magicLen     = 4
	versionLen   = 1
	suiteIDLen   = 1
	dekKIDLenField = 2
	nonceLenField  = 1
	ctLenField     = 4
	aadLenField    = 4
)

// Parsed 是解析后的信封内容。
type Parsed struct {
	SuiteID    byte   // 算法套件 ID
	DEKKID     string // DEK 的 key ID
	Nonce      []byte // 加密使用的 nonce
	Ciphertext []byte // 密文（含 GCM tag）
	AAD        []byte // 规范化 AAD
}

// SuiteIDFor 将算法名转换为 suite_id。
func SuiteIDFor(algorithm string) (byte, error) {
	switch algorithm {
	case AlgAES256GCM:
		return SuiteAES256GCM, nil
	case AlgSM4GCM:
		return SuiteSM4GCM, nil
	default:
		return 0, fmt.Errorf("envelope: unsupported algorithm: %s", algorithm)
	}
}

// AlgorithmFor 将 suite_id 转换为算法名。
func AlgorithmFor(suiteID byte) (string, error) {
	switch suiteID {
	case SuiteAES256GCM:
		return AlgAES256GCM, nil
	case SuiteSM4GCM:
		return AlgSM4GCM, nil
	default:
		return "", fmt.Errorf("envelope: unsupported suite_id: 0x%02x", suiteID)
	}
}

// Build 构建信封字节串。
//
// 格式: magic(4) || version(1) || suite_id(1) || dek_kid_len(2) || dek_kid ||
//
//	nonce_len(1) || nonce || ct_len(4) || ciphertext || aad_len(4) || aad
//
// 所有长度字段使用大端序编码。
func Build(suiteID byte, dekKID string, nonce, ciphertext, aad []byte) ([]byte, error) {
	dekKIDBytes := []byte(dekKID)
	if len(dekKIDBytes) > maxDEKKIDLen {
		return nil, errors.EnvelopeInvalid(fmt.Errorf("dek_kid too long: %d", len(dekKIDBytes)))
	}
	if len(nonce) > maxNonceLen {
		return nil, errors.EnvelopeInvalid(fmt.Errorf("nonce too long: %d", len(nonce)))
	}
	if len(ciphertext) > maxCiphertextLen {
		return nil, errors.EnvelopeInvalid(fmt.Errorf("ciphertext too long: %d", len(ciphertext)))
	}
	if len(aad) > maxAADLen {
		return nil, errors.EnvelopeInvalid(fmt.Errorf("aad too long: %d", len(aad)))
	}

	total := magicLen + versionLen + suiteIDLen +
		dekKIDLenField + len(dekKIDBytes) +
		nonceLenField + len(nonce) +
		ctLenField + len(ciphertext) +
		aadLenField + len(aad)

	buf := make([]byte, 0, total)

	// magic(4)
	buf = append(buf, Magic...)
	// version(1)
	buf = append(buf, Version)
	// suite_id(1)
	buf = append(buf, suiteID)

	// dek_kid_len(2) || dek_kid
	var kidLen [dekKIDLenField]byte
	binary.BigEndian.PutUint16(kidLen[:], uint16(len(dekKIDBytes)))
	buf = append(buf, kidLen[:]...)
	buf = append(buf, dekKIDBytes...)

	// nonce_len(1) || nonce
	buf = append(buf, byte(len(nonce)))
	buf = append(buf, nonce...)

	// ct_len(4) || ciphertext
	var ctLen [ctLenField]byte
	binary.BigEndian.PutUint32(ctLen[:], uint32(len(ciphertext)))
	buf = append(buf, ctLen[:]...)
	buf = append(buf, ciphertext...)

	// aad_len(4) || aad
	var aadLen [aadLenField]byte
	binary.BigEndian.PutUint32(aadLen[:], uint32(len(aad)))
	buf = append(buf, aadLen[:]...)
	buf = append(buf, aad...)

	return buf, nil
}

// Parse 解析信封字节串，返回 Parsed 结构。
// 若格式不合法则返回 EnvelopeInvalid 错误。
func Parse(envelope []byte) (*Parsed, error) {
	// 最小长度：所有固定字段 + 空变长字段
	minLen := magicLen + versionLen + suiteIDLen +
		dekKIDLenField + nonceLenField + ctLenField + aadLenField
	if len(envelope) < minLen {
		return nil, errors.EnvelopeInvalid(fmt.Errorf("envelope too short: got %d, want >= %d", len(envelope), minLen))
	}

	off := 0

	// magic(4)
	if string(envelope[off:off+magicLen]) != Magic {
		return nil, errors.EnvelopeInvalid(fmt.Errorf("invalid magic: %x", envelope[off:off+magicLen]))
	}
	off += magicLen

	// version(1)
	version := envelope[off]
	if version != Version {
		return nil, errors.EnvelopeInvalid(fmt.Errorf("unsupported version: %d", version))
	}
	off += versionLen

	// suite_id(1)
	suiteID := envelope[off]
	off += suiteIDLen

	// dek_kid_len(2) || dek_kid
	kidLen := int(binary.BigEndian.Uint16(envelope[off : off+dekKIDLenField]))
	off += dekKIDLenField
	if off+kidLen > len(envelope) {
		return nil, errors.EnvelopeInvalid(fmt.Errorf("truncated at dek_kid: need %d, have %d", off+kidLen, len(envelope)))
	}
	dekKID := string(envelope[off : off+kidLen])
	off += kidLen

	// nonce_len(1) || nonce
	nonceLen := int(envelope[off])
	off += nonceLenField
	if off+nonceLen > len(envelope) {
		return nil, errors.EnvelopeInvalid(fmt.Errorf("truncated at nonce: need %d, have %d", off+nonceLen, len(envelope)))
	}
	nonce := make([]byte, nonceLen)
	copy(nonce, envelope[off:off+nonceLen])
	off += nonceLen

	// ct_len(4) || ciphertext
	ctLen := int(binary.BigEndian.Uint32(envelope[off : off+ctLenField]))
	off += ctLenField
	if off+ctLen > len(envelope) {
		return nil, errors.EnvelopeInvalid(fmt.Errorf("truncated at ciphertext: need %d, have %d", off+ctLen, len(envelope)))
	}
	ciphertext := make([]byte, ctLen)
	copy(ciphertext, envelope[off:off+ctLen])
	off += ctLen

	// aad_len(4) || aad
	aadLen := int(binary.BigEndian.Uint32(envelope[off : off+aadLenField]))
	off += aadLenField
	if off+aadLen > len(envelope) {
		return nil, errors.EnvelopeInvalid(fmt.Errorf("truncated at aad: need %d, have %d", off+aadLen, len(envelope)))
	}
	aad := make([]byte, aadLen)
	copy(aad, envelope[off:off+aadLen])
	off += aadLen

	return &Parsed{
		SuiteID:    suiteID,
		DEKKID:     dekKID,
		Nonce:      nonce,
		Ciphertext: ciphertext,
		AAD:        aad,
	}, nil
}
