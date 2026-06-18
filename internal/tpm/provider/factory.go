package provider

import (
	"fmt"

	"github.com/HangzeGao/trae/key-vault/internal/config"
	"github.com/HangzeGao/trae/key-vault/internal/observability"
)

// New 根据 config 创建合适的 Provider。
//
// 当前 P0 阶段：
//   - UseSWTPM=true 时返回 SoftwareProvider（用于集成测试与开发）
//   - UseSWTPM=false 时同样返回 SoftwareProvider（P1 阶段替换为真实 TPM Provider）
//
// P1 阶段可扩展为：当 UseSWTPM=false 时返回基于 google/go-tpm 的真实
// TPM Provider，通过 cfg.DevicePath 打开 /dev/tpm0 或 tpmrm 设备。
func New(cfg config.TPMConfig, log *observability.Logger) (Provider, error) {
	if cfg.NRWKHandle == 0 {
		return nil, fmt.Errorf("provider: NRWKHandle must not be zero")
	}
	if log != nil {
		log.Info("creating TPM provider",
			"use_swtpm", cfg.UseSWTPM,
			"nrwk_handle", fmt.Sprintf("0x%08x", cfg.NRWKHandle),
			"ak_handle", fmt.Sprintf("0x%08x", cfg.AKHandle),
		)
	}
	return NewSoftwareProvider(cfg, log), nil
}
