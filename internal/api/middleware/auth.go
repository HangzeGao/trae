// Package middleware 的 auth.go 文件实现认证授权中间件（M4-1）。
//
// 提供 JWT 与 HMAC 两种认证方式的自动选择、基于 scope 的鉴权，
// 以及租户隔离校验。本文件不修改已有的 middleware.go，仅新增 auth 相关能力。
package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/HangzeGao/trae/key-vault/internal/auth/hmacsign"
	"github.com/HangzeGao/trae/key-vault/internal/auth/jwt"
	"github.com/HangzeGao/trae/key-vault/internal/auth/principal"
	"github.com/HangzeGao/trae/key-vault/internal/errors"
)

// tenantPathSegment 是 URL 路径中标识租户 ID 的路径段。
const tenantPathSegment = "tenants"

// AuthMiddleware 认证中间件，支持 JWT 和 HMAC 两种方式。
type AuthMiddleware struct {
	jwtVerifier  *jwt.Verifier
	hmacVerifier *hmacsign.Verifier
}

// NewAuthMiddleware 创建认证中间件。
// jwtv 与 hmacv 均可为 nil，表示不启用对应认证方式；至少需提供一个。
func NewAuthMiddleware(jwtv *jwt.Verifier, hmacv *hmacsign.Verifier) *AuthMiddleware {
	return &AuthMiddleware{
		jwtVerifier:  jwtv,
		hmacVerifier: hmacv,
	}
}

// RequireAuth 要求认证，自动选择 JWT 或 HMAC。
// 选择规则：存在 Authorization: Bearer <token> 时使用 JWT；
// 否则存在 X-Caller-ID 与 X-Signature 时使用 HMAC；
// 均不满足则返回 401。认证成功后将 Principal 存入 context。
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var p *principal.Principal
		var err error

		switch {
		case m.jwtVerifier != nil && hasBearerToken(r):
			p, err = m.handleJWT(ctx, r)
		case m.hmacVerifier != nil && hasHMACHeaders(r):
			p, err = m.handleHMAC(ctx, r)
		default:
			err = errors.AuthFailed(nil)
		}

		if err != nil {
			writeAuthError(w, ctx, err)
			return
		}

		// 将 Principal 与 TenantID 存入 context
		ctx = SetPrincipal(ctx, p)
		if p.TenantID != "" {
			ctx = SetTenantID(ctx, p.TenantID)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireScope 要求特定权限。
// 必须在 RequireAuth 之后使用；若 context 中无 Principal 则返回 401，
// 若 Principal 不持有指定 scope 则返回 403。
func (m *AuthMiddleware) RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			p, ok := getPrincipal(ctx)
			if !ok || p == nil {
				writeAuthError(w, ctx, errors.AuthFailed(nil))
				return
			}
			if !p.HasScope(scope) {
				writeAuthError(w, ctx, errors.PermissionDenied())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireTenantMatch 要求租户匹配（租户隔离）。
// 从 URL 路径中提取 tenant_id，与 Principal 的 TenantID 比对；
// 不一致时返回 403（统一为 PERMISSION_DENIED 以防存在性枚举，HA-11）。
// 若 URL 中未携带 tenant_id，则放行（适用于非租户作用域路由）。
func (m *AuthMiddleware) RequireTenantMatch(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		p, ok := getPrincipal(ctx)
		if !ok || p == nil {
			writeAuthError(w, ctx, errors.AuthFailed(nil))
			return
		}
		urlTenantID := extractTenantID(r.URL.Path)
		if urlTenantID == "" {
			// 路径中无租户段，放行
			next.ServeHTTP(w, r)
			return
		}
		if p.TenantID == "" || p.TenantID != urlTenantID {
			// 跨租户访问统一返回 PERMISSION_DENIED（HA-11）
			writeAuthError(w, ctx, errors.CrossTenantDenied())
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleJWT 处理 JWT 认证。
func (m *AuthMiddleware) handleJWT(ctx context.Context, r *http.Request) (*principal.Principal, error) {
	tokenStr := extractBearerToken(r)
	if tokenStr == "" {
		return nil, errors.AuthFailed(nil)
	}
	return m.jwtVerifier.Verify(ctx, tokenStr)
}

// handleHMAC 处理 HMAC 签名认证。
// 需读取请求体以计算 body_hash，读取后恢复 Body 供下游处理。
func (m *AuthMiddleware) handleHMAC(ctx context.Context, r *http.Request) (*principal.Principal, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, errors.AuthFailed(err)
	}
	// 恢复 Body 供下游 handler 读取
	r.Body = io.NopCloser(bytes.NewReader(body))
	return m.hmacVerifier.VerifyRequest(ctx, r.Method, r.URL.Path, body, r.Header)
}

// hasBearerToken 检查请求是否携带 Bearer token。
func hasBearerToken(r *http.Request) bool {
	return extractBearerToken(r) != ""
}

// extractBearerToken 从 Authorization 头提取 Bearer token。
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}
	return strings.TrimSpace(auth[len(prefix):])
}

// hasHMACHeaders 检查请求是否携带 HMAC 签名所需头。
func hasHMACHeaders(r *http.Request) bool {
	return r.Header.Get(hmacsign.HeaderCallerID) != "" &&
		r.Header.Get(hmacsign.HeaderSignature) != ""
}

// getPrincipal 从 context 提取 *principal.Principal。
func getPrincipal(ctx context.Context) (*principal.Principal, bool) {
	v := GetPrincipal(ctx)
	if v == nil {
		return nil, false
	}
	p, ok := v.(*principal.Principal)
	return p, ok
}

// extractTenantID 从 URL 路径中提取租户 ID。
// 约定路径形如 /api/v1/tenants/{tenant_id}/...，取 tenants 段之后的第一个段。
func extractTenantID(path string) string {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for i, seg := range segments {
		if seg == tenantPathSegment && i+1 < len(segments) {
			return segments[i+1]
		}
	}
	return ""
}

// writeAuthError 写入认证/授权错误响应，附加 request_id。
// 若 err 不是 *errors.Error，则包装为 AuthFailed。
func writeAuthError(w http.ResponseWriter, ctx context.Context, err error) {
	var e *errors.Error
	if err == nil {
		e = errors.AuthFailed(nil)
	} else if ae, ok := err.(*errors.Error); ok {
		e = ae
	} else {
		e = errors.AuthFailed(err)
	}
	errors.WriteJSON(w, e.WithRequestID(GetRequestID(ctx)))
}
