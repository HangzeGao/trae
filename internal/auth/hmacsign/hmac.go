// Package hmacsign 实现 HMAC 签名验证器（第 5.3 节）。
//
// 验证器基于共享密钥对请求进行 HMAC-SHA256 签名校验，强制校验
// caller_id、timestamp、nonce、method、path、body_hash 六项（HA-02），
// 并通过 timestamp skew 与 nonce 防重放窗口抵御重放攻击。
package hmacsign

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HangzeGao/trae/key-vault/internal/auth/principal"
	"github.com/HangzeGao/trae/key-vault/internal/config"
	"github.com/HangzeGao/trae/key-vault/internal/errors"
)

// 请求头名称常量。
const (
	HeaderCallerID = "X-Caller-ID"
	HeaderTimestamp = "X-Timestamp"
	HeaderNonce    = "X-Nonce"
	HeaderSignature = "X-Signature"
)

// SecretGetter 获取指定 caller 的 HMAC secret。
type SecretGetter interface {
	GetSecret(ctx context.Context, callerID string) ([]byte, error)
}

// CallerInfoGetter 是可选接口，用于获取调用方关联的租户 ID 与权限范围。
// 若 SecretGetter 实现了此接口，验证器将使用它来填充 Principal 的 TenantID 与 Scopes。
type CallerInfoGetter interface {
	GetCallerInfo(ctx context.Context, callerID string) (tenantID string, scopes []string, err error)
}

// Verifier 是 HMAC 签名验证器（第 5.3 节）。
type Verifier struct {
	timestampSkew time.Duration
	nonceWindow   time.Duration
	nonceStore    *nonceStore // 防重放窗口
	secretGetter  SecretGetter
}

// New 创建 HMAC 验证器。
// timestampSkew 与 nonceWindow 为零值时使用安全默认值（300s / 600s）。
func New(cfg config.AuthConfig, getter SecretGetter) *Verifier {
	skew := cfg.TimestampSkew
	if skew <= 0 {
		skew = 300 * time.Second
	}
	window := cfg.NonceWindow
	if window <= 0 {
		window = 600 * time.Second
	}
	// 约束：nonce_window 至少为 2 倍 skew，否则无法覆盖时钟漂移
	if window < 2*skew {
		window = 2 * skew
	}
	v := &Verifier{
		timestampSkew: skew,
		nonceWindow:   window,
		nonceStore:    newNonceStore(window),
		secretGetter:  getter,
	}
	// 启动定期清理过期 nonce 的后台 goroutine
	go v.nonceStore.cleanupLoop(context.Background(), window/2)
	return v
}

// VerifyRequest 验证 HMAC 签名请求。
// 必须校验六项（HA-02）：caller_id, timestamp, nonce, method, path, body_hash。
// 签名格式: HMAC-SHA256(secret, caller_id || timestamp || nonce || method || path || body_hash)。
// 从请求头提取: X-Caller-ID, X-Timestamp, X-Nonce, X-Signature。
// body_hash = SHA256(request_body)。
func (v *Verifier) VerifyRequest(ctx context.Context, method, path string, body []byte, headers http.Header) (*principal.Principal, error) {
	// HA-02：caller_id 必填
	callerID := strings.TrimSpace(headers.Get(HeaderCallerID))
	if callerID == "" {
		return nil, errors.AuthFailed(fmt.Errorf("missing caller id"))
	}
	// HA-02：timestamp 必填
	timestampStr := strings.TrimSpace(headers.Get(HeaderTimestamp))
	if timestampStr == "" {
		return nil, errors.AuthFailed(fmt.Errorf("missing timestamp"))
	}
	// HA-02：nonce 必填
	nonce := strings.TrimSpace(headers.Get(HeaderNonce))
	if nonce == "" {
		return nil, errors.AuthFailed(fmt.Errorf("missing nonce"))
	}
	// HA-02：method 必填（由调用方传入，校验非空）
	if method == "" {
		return nil, errors.AuthFailed(fmt.Errorf("missing method"))
	}
	// HA-02：path 必填（由调用方传入，校验非空）
	if path == "" {
		return nil, errors.AuthFailed(fmt.Errorf("missing path"))
	}
	// 签名头必填
	signatureHex := strings.TrimSpace(headers.Get(HeaderSignature))
	if signatureHex == "" {
		return nil, errors.AuthFailed(fmt.Errorf("missing signature"))
	}

	// HA-02：timestamp 必须在 skew 范围内
	ts, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return nil, errors.AuthFailed(fmt.Errorf("invalid timestamp: %w", err))
	}
	now := time.Now()
	reqTime := time.Unix(ts, 0)
	diff := now.Sub(reqTime)
	if diff < 0 {
		diff = -diff
	}
	if diff > v.timestampSkew {
		return nil, errors.AuthFailed(fmt.Errorf("timestamp skew %s exceeds %s", diff, v.timestampSkew))
	}

	// HA-02：nonce 在 nonce_window 内不能重复
	if !v.nonceStore.checkAndStore(callerID, nonce, now) {
		return nil, errors.AuthFailed(fmt.Errorf("nonce replay detected"))
	}

	// 获取 caller 的 HMAC secret
	secret, err := v.secretGetter.GetSecret(ctx, callerID)
	if err != nil {
		return nil, errors.AuthFailed(fmt.Errorf("get secret: %w", err))
	}
	if len(secret) == 0 {
		return nil, errors.AuthFailed(fmt.Errorf("empty secret for caller %s", callerID))
	}

	// HA-02：body_hash = SHA256(request_body)
	bodyHash := sha256.Sum256(body)
	bodyHashHex := hex.EncodeToString(bodyHash[:])

	// 构造待签名字符串：caller_id || timestamp || nonce || method || path || body_hash
	// 使用换行符分隔以避免字段歧义
	message := strings.Join([]string{callerID, timestampStr, nonce, method, path, bodyHashHex}, "\n")

	// 计算预期签名: HMAC-SHA256(secret, message)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(message))
	expectedMAC := mac.Sum(nil)

	// 解析客户端签名
	clientMAC, err := hex.DecodeString(signatureHex)
	if err != nil {
		return nil, errors.AuthFailed(fmt.Errorf("invalid signature encoding: %w", err))
	}
	// 常量时间比较，防止时序攻击
	if !hmac.Equal(expectedMAC, clientMAC) {
		return nil, errors.AuthFailed(fmt.Errorf("signature mismatch"))
	}

	// 构造 Principal
	p := &principal.Principal{
		CallerID:   callerID,
		TokenID:    nonce, // 使用 nonce 作为请求 ID 用于审计
		IssuedAt:   reqTime,
		ExpiresAt:  reqTime.Add(v.nonceWindow),
		AuthMethod: principal.AuthMethodHMAC,
	}

	// 若 SecretGetter 实现了 CallerInfoGetter，填充租户与权限信息
	if infoGetter, ok := v.secretGetter.(CallerInfoGetter); ok {
		tenantID, scopes, err := infoGetter.GetCallerInfo(ctx, callerID)
		if err != nil {
			return nil, errors.AuthFailed(fmt.Errorf("get caller info: %w", err))
		}
		p.TenantID = tenantID
		p.Scopes = scopes
	}

	return p, nil
}

// nonceStore 防重放存储，记录 nonce_window 内已使用的 nonce。
type nonceStore struct {
	window time.Duration
	mu     sync.Mutex
	seen   map[string]time.Time // nonce -> 过期时间
}

// newNonceStore 创建 nonce 防重放存储。
func newNonceStore(window time.Duration) *nonceStore {
	return &nonceStore{
		window: window,
		seen:   make(map[string]time.Time),
	}
}

// checkAndStore 检查 nonce 是否已存在；若不存在则存入并返回 true，否则返回 false。
// nonce 的 key 由 callerID 与 nonce 组合，避免不同 caller 使用相同 nonce 时误判。
func (s *nonceStore) checkAndStore(callerID, nonce string, now time.Time) bool {
	key := callerID + ":" + nonce
	expiry := now.Add(s.window)
	s.mu.Lock()
	defer s.mu.Unlock()
	if exp, ok := s.seen[key]; ok {
		// 仍在窗口内：重放
		if now.Before(exp) {
			return false
		}
	}
	s.seen[key] = expiry
	return true
}

// cleanupLoop 定期清理过期的 nonce。
func (s *nonceStore) cleanupLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = s.window / 2
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.cleanup(now)
		}
	}
}

// cleanup 删除已过期的 nonce。
func (s *nonceStore) cleanup(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, exp := range s.seen {
		if !now.Before(exp) {
			delete(s.seen, k)
		}
	}
}
