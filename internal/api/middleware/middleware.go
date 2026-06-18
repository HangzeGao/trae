// Package middleware 提供 HTTP 中间件。
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"runtime/debug"

	"github.com/HangzeGao/trae/key-vault/internal/errors"
	"github.com/HangzeGao/trae/key-vault/internal/observability"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	tenantIDKey  contextKey = "tenant_id"
	principalKey contextKey = "principal"
)

// RequestID 为每个请求生成唯一 ID。
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			b := make([]byte, 16)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set("X-Request-ID", id)
		ctx := observability.WithRequestID(r.Context(), id)
		ctx = context.WithValue(ctx, requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Recover 捕获 panic，返回 500，并记录堆栈（HA-03：panic 时零化敏感缓冲区由各组件负责）。
func Recover(log *observability.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic recovered",
					"error", rec,
					"request_id", observability.RequestIDFromContext(r.Context()),
					"stack", string(debug.Stack()),
				)
				errors.WriteJSON(w, errors.Internal(nil).WithRequestID(observability.RequestIDFromContext(r.Context())))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Logging 记录请求日志。
func Logging(log *observability.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"request_id", observability.RequestIDFromContext(r.Context()),
		)
		next.ServeHTTP(w, r)
	})
}

// GetRequestID 从 context 提取 request ID。
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// SetTenantID 在 context 中设置租户 ID。
func SetTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// GetTenantID 从 context 提取租户 ID。
func GetTenantID(ctx context.Context) string {
	if v, ok := ctx.Value(tenantIDKey).(string); ok {
		return v
	}
	return ""
}

// SetPrincipal 在 context 中设置调用方主体。
func SetPrincipal(ctx context.Context, p interface{}) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// GetPrincipal 从 context 提取调用方主体。
func GetPrincipal(ctx context.Context) interface{} {
	return ctx.Value(principalKey)
}
