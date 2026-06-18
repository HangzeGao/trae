package keyresolver

import (
	"context"

	apperrors "github.com/HangzeGao/trae/key-vault/internal/errors"
)

// HealthCheck 检查 resolver 健康状态。
//
// 尝试一次轻量 TPM 操作（GetNRWKPublicKey），成功则清除降级状态，
// 使后续 DEK cache miss 请求恢复正常解封流程。
// 失败则设置降级状态并返回 TPMUnavailable 错误。
//
// 此方法可被就绪检查（readiness probe）定期调用以实现自动恢复：
// 当 TPM 从瞬时故障中恢复后，健康检查成功会自动清除降级状态。
func (r *Resolver) HealthCheck(ctx context.Context) error {
	// 使用 ResolverTimeout 控制超时，避免健康检查阻塞过久
	timeoutCtx, cancel := context.WithTimeout(ctx, r.config.ResolverTimeout)
	defer cancel()

	if _, err := r.tpm.GetNRWKPublicKey(timeoutCtx); err != nil {
		// TPM 不可用，设置降级状态
		r.degraded.Store(true)
		r.log.Error("resolver health check failed, entering degraded mode",
			"error", err,
		)
		return apperrors.TPMUnavailable(err)
	}

	// 健康检查成功，清除降级状态
	r.degraded.Store(false)
	return nil
}
