// Package cpufeat 检测 CPU 指令集特性,用于在启动时选择最优算法实现。
package cpufeat

import "golang.org/x/sys/cpu"

// Features 描述本进程可见的 SIMD 加密指令集。
// 注意:Go 标准库仅暴露 AVX-512 编码的 GFNI/VAES/VPCLMULQDQ 字段;
// 早期非 AVX-512 版本的 GFNI 等在实际硬件中并不存在,因此使用 AVX-512 系列字段作判断。
type Features struct {
	AESNI     bool
	GFNI      bool // HasAVX512GFNI
	AVX2      bool
	AVX512F   bool
	SSE41     bool
	VAES      bool // HasAVX512VAES
	VPCLMUL   bool // HasAVX512VPCLMULQDQ
	PCLMULQDQ bool
}

// Map 用于日志/指标输出。
func (f Features) Map() map[string]bool {
	return map[string]bool{
		"aesni":     f.AESNI,
		"gfni":      f.GFNI,
		"avx2":      f.AVX2,
		"avx512f":   f.AVX512F,
		"sse4_1":    f.SSE41,
		"vaes":      f.VAES,
		"vpclmul":   f.VPCLMUL,
		"pclmulqdq": f.PCLMULQDQ,
	}
}

// Detect 读取 runtime CPU 特性。
func Detect() Features {
	x := cpu.X86
	return Features{
		AESNI:     x.HasAES,
		GFNI:      x.HasAVX512GFNI,
		AVX2:      x.HasAVX2,
		AVX512F:   x.HasAVX512F,
		SSE41:     x.HasSSE41,
		VAES:      x.HasAVX512VAES,
		VPCLMUL:   x.HasAVX512VPCLMULQDQ,
		PCLMULQDQ: x.HasPCLMULQDQ,
	}
}

// HasAESGCMFastPath 判定是否具备硬件 AES-GCM 加速。
func (f Features) HasAESGCMFastPath() bool {
	// AES-NI 走 CLMUL 即可;VAES + VPCLMUL 是 AVX-512 增强路径,均视为具备快速路径。
	return (f.AESNI && f.PCLMULQDQ) || (f.VAES && f.VPCLMUL)
}

// HasSM4FastPath 判定是否具备 SM4 加速(GFNI 是 SM4 高效实现的必要指令)。
func (f Features) HasSM4FastPath() bool {
	return f.GFNI
}
