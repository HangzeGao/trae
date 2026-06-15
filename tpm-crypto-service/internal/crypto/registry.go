// Package crypto 提供算法注册表与后端选择逻辑。
package crypto

import (
	"fmt"

	"github.com/tpm-crypto/tpm-crypto-service/internal/cpufeat"
	aesc "github.com/tpm-crypto/tpm-crypto-service/internal/crypto/aes"
	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto/common"
	sm4c "github.com/tpm-crypto/tpm-crypto-service/internal/crypto/sm4"
)

// BackendName 标识后端实现。
type BackendName string

const (
	BackendStd     BackendName = "std"
	BackendCircl   BackendName = "circl"
	BackendOpenSSL BackendName = "openssl"
)

// Factory 构造 (alg, mode, keyLen) 对应的 Cipher。
type Factory func(alg common.Algorithm, mode common.BlockMode, keyLen int) (common.Cipher, BackendName, error)

// NewFactory 根据配置 + CPU 特性返回合适的工厂。
// backend 取值: auto | std | circl | openssl。
//   - auto:依据 cpufeat 决定;当前 std 已对 AES-NI / GFNI 自动加速,直接返回 std
//   - circl:暂未实现(预留接口)
//   - openssl:需 -tags=cgo_openssl 构建,此处仅做接口桩
func NewFactory(backend string, feat cpufeat.Features) (Factory, error) {
	switch backend {
	case "auto", "std", "":
		return stdFactory(feat), nil
	case "circl":
		// 当前未实现 circl 路径,直接拒绝以免误用
		return nil, fmt.Errorf("circl backend not implemented in this build")
	case "openssl":
		return opensslFactory(feat)
	default:
		return nil, fmt.Errorf("unknown crypto backend %q", backend)
	}
}

func stdFactory(_ cpufeat.Features) Factory {
	return func(alg common.Algorithm, mode common.BlockMode, keyLen int) (common.Cipher, BackendName, error) {
		if err := common.Supports(alg, mode, keyLen); err != nil {
			return nil, "", err
		}
		switch alg {
		case common.AES:
			c, err := aesc.New(mode, keyLen)
			return c, BackendStd, err
		case common.SM4:
			c, err := sm4c.New(mode)
			return c, BackendStd, err
		}
		return nil, "", fmt.Errorf("%w: alg %s", common.ErrUnsupported, alg)
	}
}

func opensslFactory(_ cpufeat.Features) (Factory, error) {
	return func(alg common.Algorithm, mode common.BlockMode, keyLen int) (common.Cipher, BackendName, error) {
		return nil, "", fmt.Errorf("openssl backend requires -tags=cgo_openssl build (not yet wired in this revision)")
	}, nil
}
