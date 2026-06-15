// Package audit 提供统一的审计日志写入器。
package audit

import (
	"go.uber.org/zap"
)

// Log 记录一次业务操作;绝不包含明文密钥与业务数据。
func Log(log *zap.Logger, method, keyID, algorithm, mode, caller, result string, extra ...zap.Field) {
	fields := []zap.Field{
		zap.Bool("audit", true),
		zap.String("method", method),
		zap.String("caller", caller),
		zap.String("result", result),
	}
	if keyID != "" {
		fields = append(fields, zap.String("key_id", keyID))
	}
	if algorithm != "" {
		fields = append(fields, zap.String("algorithm", algorithm))
	}
	if mode != "" {
		fields = append(fields, zap.String("mode", mode))
	}
	fields = append(fields, extra...)
	log.Info("audit", fields...)
}
