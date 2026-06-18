// Package jwt 实现 JWT 验证器（第 5.2 节）。
//
// 验证器从配置的 JWK URL 周期性拉取并缓存 JWK Set，使用 RS256 算法
// 校验 JWT 签名，并强制校验 iss/aud/exp/iat/sub/tid 六项必填字段（HA-01）。
package jwt

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/HangzeGao/trae/key-vault/internal/auth/principal"
	"github.com/HangzeGao/trae/key-vault/internal/config"
	"github.com/HangzeGao/trae/key-vault/internal/errors"
)

// Verifier 是 JWT 验证器（第 5.2 节）。
type Verifier struct {
	issuer   string
	audience string
	jwkURL   string
	cacheTTL time.Duration
	maxTTL   time.Duration
	keys     *keyCache // JWK 缓存
}

// New 创建 JWT 验证器。
// 必须提供 JWTIssuer、JWTAudience 与 JWKURL；其余字段为零值时使用安全默认值。
func New(cfg config.AuthConfig) (*Verifier, error) {
	if cfg.JWTIssuer == "" {
		return nil, fmt.Errorf("jwt issuer is required")
	}
	if cfg.JWTAudience == "" {
		return nil, fmt.Errorf("jwt audience is required")
	}
	if cfg.JWKURL == "" {
		return nil, fmt.Errorf("jwk url is required")
	}
	cacheTTL := cfg.JWKCacheTTL
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	maxTTL := cfg.TokenMaxTTL
	if maxTTL <= 0 {
		maxTTL = 15 * time.Minute
	}
	return &Verifier{
		issuer:   cfg.JWTIssuer,
		audience: cfg.JWTAudience,
		jwkURL:   cfg.JWKURL,
		cacheTTL: cacheTTL,
		maxTTL:   maxTTL,
		keys:     newKeyCache(cfg.JWKURL, cacheTTL),
	}, nil
}

// Verify 验证 JWT token，返回 Principal。
// 必须校验六字段（HA-01）：iss, aud, exp, iat, sub, tid。
// exp 不能超过 maxTTL。
func (v *Verifier) Verify(ctx context.Context, tokenStr string) (*principal.Principal, error) {
	if strings.TrimSpace(tokenStr) == "" {
		return nil, errors.AuthFailed(fmt.Errorf("empty token"))
	}

	claims := &customClaims{}
	// 解析并校验签名、iss、aud，并要求 exp 必填
	parsed, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		// 仅允许 RS256
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		key, kerr := v.keys.getKey(ctx, kid)
		if kerr != nil {
			return nil, kerr
		}
		return key, nil
	},
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{"RS256"}),
	)
	if err != nil {
		return nil, errors.AuthFailed(err)
	}
	if parsed == nil || !parsed.Valid {
		return nil, errors.AuthFailed(fmt.Errorf("invalid token"))
	}

	// HA-01：sub 必填
	if claims.Subject == "" {
		return nil, errors.AuthFailed(fmt.Errorf("missing subject (sub)"))
	}
	// HA-01：tid 必填
	if claims.TenantID == "" {
		return nil, errors.AuthFailed(fmt.Errorf("missing tenant id (tid)"))
	}
	// HA-01：iat 必填
	if claims.IssuedAt == nil || claims.IssuedAt.Time.IsZero() {
		return nil, errors.AuthFailed(fmt.Errorf("missing issued at (iat)"))
	}

	p := &principal.Principal{
		TenantID:   claims.TenantID,
		CallerID:   claims.Subject,
		Scopes:     claims.Scopes,
		TokenID:    claims.ID,
		IssuedAt:   claims.IssuedAt.Time,
		ExpiresAt:  claims.ExpiresAt.Time,
		AuthMethod: principal.AuthMethodJWT,
	}

	// 二次校验必填字段与 TTL 约束
	if err := p.Validate(v.issuer, v.audience, v.maxTTL); err != nil {
		return nil, err
	}
	return p, nil
}

// customClaims 自定义 JWT claims，包含标准字段与扩展字段（tid、scopes）。
type customClaims struct {
	TenantID string   `json:"tid"`             // 租户 ID（HA-01 必填）
	Scopes   []string `json:"scopes,omitempty"` // 权限范围
	jwt.RegisteredClaims
}

// keyCache 内部 JWK 缓存，定期从 JWKURL 刷新。
type keyCache struct {
	url    string
	ttl    time.Duration
	mu     sync.RWMutex
	keys   map[string]*rsa.PublicKey
	expiry time.Time
	client *http.Client
}

// newKeyCache 创建 JWK 缓存。
func newKeyCache(url string, ttl time.Duration) *keyCache {
	return &keyCache{
		url:    url,
		ttl:    ttl,
		keys:   make(map[string]*rsa.PublicKey),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// getKey 根据 kid 获取 RSA 公钥，必要时触发刷新。
func (c *keyCache) getKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	if time.Now().Before(c.expiry) {
		if key, ok := c.keys[kid]; ok {
			c.mu.RUnlock()
			return key, nil
		}
		// 缓存未过期但找不到 kid，直接返回错误避免穿透
		if len(c.keys) > 0 {
			c.mu.RUnlock()
			return nil, fmt.Errorf("key with kid %q not found", kid)
		}
	}
	c.mu.RUnlock()

	// 缓存已过期或为空，触发刷新
	if err := c.refresh(ctx); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if key, ok := c.keys[kid]; ok {
		return key, nil
	}
	// kid 为空且缓存中只有一个 key 时，回退使用该 key
	if kid == "" && len(c.keys) == 1 {
		for _, key := range c.keys {
			return key, nil
		}
	}
	return nil, fmt.Errorf("key with kid %q not found", kid)
}

// refresh 从 URL 拉取最新的 JWK Set 并更新缓存。
func (c *keyCache) refresh(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 双重检查：可能在等待锁期间已被其他 goroutine 刷新
	if time.Now().Before(c.expiry) {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return fmt.Errorf("build jwk request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch jwk: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwk fetch failed: status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read jwk body: %w", err)
	}

	var jwks struct {
		Keys []jsonWebKey `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("parse jwk set: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey)
	for _, jwk := range jwks.Keys {
		if jwk.Kty != "RSA" {
			continue
		}
		key, err := jwk.toRSAPublicKey()
		if err != nil {
			// 跳过无法解析的 key
			continue
		}
		newKeys[jwk.Kid] = key
	}
	if len(newKeys) == 0 {
		return fmt.Errorf("no usable RSA keys in jwk set")
	}

	c.keys = newKeys
	c.expiry = time.Now().Add(c.ttl)
	return nil
}

// jsonWebKey 表示 JWK Set 中的一个 key。
type jsonWebKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use,omitempty"`
	Alg string `json:"alg,omitempty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// toRSAPublicKey 将 JWK 转换为 RSA 公钥。
func (jwk jsonWebKey) toRSAPublicKey() (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	// 将 e 转换为 int
	var eInt int
	for _, b := range eBytes {
		eInt = eInt<<8 + int(b)
	}
	if eInt == 0 {
		return nil, fmt.Errorf("invalid exponent")
	}
	n := new(big.Int).SetBytes(nBytes)
	return &rsa.PublicKey{N: n, E: eInt}, nil
}
