package model

// Algorithm 表示对称加密算法
type Algorithm string

const (
	AlgorithmAES128 Algorithm = "AES-128"
	AlgorithmAES192 Algorithm = "AES-192"
	AlgorithmAES256 Algorithm = "AES-256"
	AlgorithmSM4    Algorithm = "SM4"
)

// Mode 表示分组密码模式
type Mode string

const (
	ModeGCM Mode = "GCM"
	ModeCBC Mode = "CBC"
	ModeCTR Mode = "CTR"
	ModeCFB Mode = "CFB"
	ModeOFB Mode = "OFB"
)

// SupportedAlgorithms 返回所有支持的算法
func SupportedAlgorithms() []Algorithm {
	return []Algorithm{AlgorithmAES128, AlgorithmAES192, AlgorithmAES256, AlgorithmSM4}
}

// SupportedModes 返回所有支持的模式
func SupportedModes() []Mode {
	return []Mode{ModeGCM, ModeCBC, ModeCTR, ModeCFB, ModeOFB}
}

// KeyLength 返回算法对应的密钥长度（字节）
func (a Algorithm) KeyLength() int {
	switch a {
	case AlgorithmAES128:
		return 16
	case AlgorithmAES192:
		return 24
	case AlgorithmAES256:
		return 32
	case AlgorithmSM4:
		return 16
	default:
		return 0
	}
}

// IsValid 检查算法是否受支持
func (a Algorithm) IsValid() bool {
	for _, alg := range SupportedAlgorithms() {
		if a == alg {
			return true
		}
	}
	return false
}

// IsValid 检查模式是否受支持
func (m Mode) IsValid() bool {
	for _, mode := range SupportedModes() {
		if m == mode {
			return true
		}
	}
	return false
}
