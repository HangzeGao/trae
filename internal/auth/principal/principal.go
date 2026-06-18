// Package principal 定义已认证的调用方主体模型（第 5.1 节）。
//
// Principal 在认证中间件中由 JWT 或 HMAC 验证器构造，并通过 context
// 在请求处理链中传递，用于后续的鉴权与租户隔离校验。
package principal

import (
	"fmt"
	"time"

	"github.com/HangzeGao/trae/key-vault/internal/errors"
)

// Scope 常量定义系统支持的全部权限范围。
const (
	// ScopeKeysRead 允许读取密钥元数据。
	ScopeKeysRead = "keys:read"
	// ScopeKeysWrite 允许创建、更新、轮换或销毁密钥。
	ScopeKeysWrite = "keys:write"
	// ScopeCryptoEncrypt 允许调用加密接口。
	ScopeCryptoEncrypt = "crypto:encrypt"
	// ScopeCryptoDecrypt 允许调用解密接口。
	ScopeCryptoDecrypt = "crypto:decrypt"
	// ScopeDataKeyGenerate 允许生成数据密钥（DEK）。
	ScopeDataKeyGenerate = "datakey:generate"
	// ScopeAdmin 拥有全部管理权限。
	ScopeAdmin = "admin"
)

// AuthMethodJWT 表示通过 JWT 完成认证。
const AuthMethodJWT = "jwt"

// AuthMethodHMAC 表示通过 HMAC 签名完成认证。
const AuthMethodHMAC = "hmac"

// Principal 表示已认证的调用方主体（第 5.1 节）。
type Principal struct {
	// TenantID 是调用方所属租户 ID。
	TenantID string
	// CallerID 是节点 ID 或服务 ID。
	CallerID string
	// Scopes 是授予该主体的权限范围列表。
	Scopes []string
	// TokenID 是 JWT 的 jti 或 HMAC 请求 ID，用于审计与防重放。
	TokenID string
	// IssuedAt 是 token 签发时间。
	IssuedAt time.Time
	// ExpiresAt 是 token 过期时间。
	ExpiresAt time.Time
	// AuthMethod 取值为 "jwt" 或 "hmac"。
	AuthMethod string
}

// HasScope 检查是否拥有指定权限。
func (p *Principal) HasScope(scope string) bool {
	if p == nil {
		return false
	}
	for _, s := range p.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasAnyScope 检查是否拥有任一权限。
func (p *Principal) HasAnyScope(scopes ...string) bool {
	if p == nil {
		return false
	}
	for _, want := range scopes {
		for _, got := range p.Scopes {
			if got == want {
				return true
			}
		}
	}
	return false
}

// IsExpired 检查 token 是否过期。
func (p *Principal) IsExpired() bool {
	if p == nil {
		return true
	}
	if p.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(p.ExpiresAt)
}

// Validate 校验 JWT 必填字段（HA-01）：iss, aud, exp, iat, sub, tid。
// iss 与 aud 由调用方传入（JWT 验证器已通过 jwt.WithIssuer/WithAudience
// 完成实际比对），此处对 principal 持有的 tid/sub/iat/exp 做必填与时效校验，
// 并确保 exp - iat 不超过 maxTTL。
func (p *Principal) Validate(issuer, audience string, maxTTL time.Duration) error {
	if p == nil {
		return errors.AuthFailed(fmt.Errorf("principal is nil"))
	}
	// 防御性校验：调用方必须提供非空的预期 iss/aud
	if issuer == "" {
		return errors.AuthFailed(fmt.Errorf("missing expected issuer"))
	}
	if audience == "" {
		return errors.AuthFailed(fmt.Errorf("missing expected audience"))
	}
	// HA-01：tid 必须存在
	if p.TenantID == "" {
		return errors.AuthFailed(fmt.Errorf("missing tenant id (tid)"))
	}
	// HA-01：sub 必须存在
	if p.CallerID == "" {
		return errors.AuthFailed(fmt.Errorf("missing subject (sub)"))
	}
	// HA-01：iat 必须存在
	if p.IssuedAt.IsZero() {
		return errors.AuthFailed(fmt.Errorf("missing issued at (iat)"))
	}
	// HA-01：exp 必须存在
	if p.ExpiresAt.IsZero() {
		return errors.AuthFailed(fmt.Errorf("missing expires at (exp)"))
	}
	// 校验 exp - iat <= maxTTL
	if maxTTL > 0 {
		ttl := p.ExpiresAt.Sub(p.IssuedAt)
		if ttl <= 0 {
			return errors.AuthFailed(fmt.Errorf("token ttl must be positive"))
		}
		if ttl > maxTTL {
			return errors.AuthFailed(fmt.Errorf("token ttl %s exceeds max ttl %s", ttl, maxTTL))
		}
	}
	// 校验未过期
	if p.IsExpired() {
		return errors.AuthFailed(fmt.Errorf("token expired"))
	}
	return nil
}
