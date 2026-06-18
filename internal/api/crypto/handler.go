// Package crypto 实现数据面加解密 HTTP API（M3-2）。
//
// Handler 注册以下路由（均需认证与租户匹配）：
//   - POST /api/v1/tenants/{tenantID}/crypto/encrypt  加密（crypto:encrypt）
//   - POST /api/v1/tenants/{tenantID}/crypto/decrypt  解密（crypto:decrypt）
//   - POST /api/v1/tenants/{tenantID}/crypto/datakey  生成数据密钥（datakey:generate）
//
// 所有路由通过 RequireAuth -> RequireTenantMatch -> RequireScope 中间件链保护，
// 确保认证、租户隔离与权限校验。请求/响应使用 JSON 格式，
// plaintext/aad/envelope 使用 base64 编码。
package crypto

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"

	cryptosvc "github.com/HangzeGao/trae/key-vault/internal/application/crypto"
	"github.com/HangzeGao/trae/key-vault/internal/api/middleware"
	"github.com/HangzeGao/trae/key-vault/internal/auth/principal"
	apperrors "github.com/HangzeGao/trae/key-vault/internal/errors"
	"github.com/HangzeGao/trae/key-vault/internal/observability"
)

// 路由路径常量。
const (
	routeEncrypt = "/api/v1/tenants/{tenantID}/crypto/encrypt"
	routeDecrypt = "/api/v1/tenants/{tenantID}/crypto/decrypt"
	routeDataKey = "/api/v1/tenants/{tenantID}/crypto/datakey"
)

// 权限 scope 常量。
const (
	scopeCryptoEncrypt = "crypto:encrypt"
	scopeCryptoDecrypt = "crypto:decrypt"
	scopeDataKeyGen    = "datakey:generate"
)

// Handler 是加解密 API 的 HTTP 处理器。
//
// 通过 RegisterRoutes 注册路由，所有路由经认证、租户匹配、scope 校验中间件保护。
type Handler struct {
	service *cryptosvc.Service
	auth    *middleware.AuthMiddleware
	log     *observability.Logger
}

// NewHandler 创建一个加解密 API HTTP 处理器实例。
//
// service 提供加解密业务逻辑；auth 提供认证授权中间件；
// log 用于结构化日志输出。
func NewHandler(service *cryptosvc.Service, auth *middleware.AuthMiddleware, log *observability.Logger) *Handler {
	return &Handler{
		service: service,
		auth:    auth,
		log:     log,
	}
}

// RegisterRoutes 在 mux 上注册加解密路由。
//
// 每条路由依次应用 RequireAuth、RequireTenantMatch、RequireScope 中间件，
// 确保认证、租户隔离与权限校验。使用 Go 1.22+ 的方法+路径模式注册。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// 加密：crypto:encrypt
	mux.Handle("POST "+routeEncrypt,
		h.wrap(scopeCryptoEncrypt, http.HandlerFunc(h.Encrypt)))

	// 解密：crypto:decrypt
	mux.Handle("POST "+routeDecrypt,
		h.wrap(scopeCryptoDecrypt, http.HandlerFunc(h.Decrypt)))

	// 生成数据密钥：datakey:generate
	mux.Handle("POST "+routeDataKey,
		h.wrap(scopeDataKeyGen, http.HandlerFunc(h.GenerateDataKey)))
}

// wrap 包装中间件链：RequireAuth -> RequireTenantMatch -> RequireScope -> handler。
//
// 返回最终包装后的 http.Handler，确保认证、租户隔离与权限校验依次执行。
func (h *Handler) wrap(scope string, handler http.Handler) http.Handler {
	return h.auth.RequireAuth(
		h.auth.RequireTenantMatch(
			h.auth.RequireScope(scope)(handler)))
}

// encryptRequest 加密请求体。
type encryptRequest struct {
	KeyID     string `json:"key_id"`
	Version   int    `json:"version"`
	Plaintext string `json:"plaintext"` // base64 编码
	AAD       string `json:"aad"`       // base64 编码，可选
}

// encryptResponse 加密响应体。
type encryptResponse struct {
	Envelope  string `json:"envelope"` // base64 编码
	KID       string `json:"kid"`
	Algorithm string `json:"algorithm"`
}

// decryptRequest 解密请求体。
type decryptRequest struct {
	Envelope string `json:"envelope"` // base64 编码
	AAD      string `json:"aad"`      // base64 编码，可选
}

// decryptResponse 解密响应体。
type decryptResponse struct {
	Plaintext string `json:"plaintext"` // base64 编码
	KID       string `json:"kid"`
	Algorithm string `json:"algorithm"`
}

// generateDataKeyRequest 生成数据密钥请求体。
type generateDataKeyRequest struct {
	KeyID     string `json:"key_id"`
	Algorithm string `json:"algorithm"`
}

// generateDataKeyResponse 生成数据密钥响应体。
type generateDataKeyResponse struct {
	PlaintextDEK string `json:"plaintext_dek"` // base64 编码
	WrappedDEK   string `json:"wrapped_dek"`   // base64 编码
	KID          string `json:"kid"`
}

// Encrypt 处理 POST /api/v1/tenants/{tenantID}/crypto/encrypt 请求。
//
// 从请求体解析加密参数，调用 Service.Encrypt 加密数据，
// 返回 200 OK 与密文信封。需要 crypto:encrypt scope。
func (h *Handler) Encrypt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.PathValue("tenantID")
	requestID := middleware.GetRequestID(ctx)
	callerID := callerFromContext(ctx)

	var body encryptRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.InvalidRequest("invalid json body").WithRequestID(requestID))
		return
	}

	if body.KeyID == "" {
		writeError(w, apperrors.InvalidRequest("key_id is required").WithRequestID(requestID))
		return
	}

	plaintext, err := base64.StdEncoding.DecodeString(body.Plaintext)
	if err != nil {
		writeError(w, apperrors.InvalidRequest("invalid base64 plaintext").WithRequestID(requestID))
		return
	}

	var aad []byte
	if body.AAD != "" {
		aad, err = base64.StdEncoding.DecodeString(body.AAD)
		if err != nil {
			writeError(w, apperrors.InvalidRequest("invalid base64 aad").WithRequestID(requestID))
			return
		}
	}

	req := cryptosvc.EncryptRequest{
		TenantID:  tenantID,
		KeyID:     body.KeyID,
		Version:   body.Version,
		Plaintext: plaintext,
		AAD:       aad,
		CallerID:  callerID,
		RequestID: requestID,
	}

	resp, err := h.service.Encrypt(ctx, req)
	if err != nil {
		writeError(w, withRequestID(err, requestID))
		return
	}

	writeJSON(w, http.StatusOK, encryptResponse{
		Envelope:  base64.StdEncoding.EncodeToString(resp.Envelope),
		KID:       resp.KID,
		Algorithm: resp.Algorithm,
	})
}

// Decrypt 处理 POST /api/v1/tenants/{tenantID}/crypto/decrypt 请求。
//
// 从请求体解析密文信封，调用 Service.Decrypt 解密数据，
// 返回 200 OK 与明文。需要 crypto:decrypt scope。
func (h *Handler) Decrypt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.PathValue("tenantID")
	requestID := middleware.GetRequestID(ctx)
	callerID := callerFromContext(ctx)

	var body decryptRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.InvalidRequest("invalid json body").WithRequestID(requestID))
		return
	}

	if body.Envelope == "" {
		writeError(w, apperrors.InvalidRequest("envelope is required").WithRequestID(requestID))
		return
	}

	envelopeBytes, err := base64.StdEncoding.DecodeString(body.Envelope)
	if err != nil {
		writeError(w, apperrors.InvalidRequest("invalid base64 envelope").WithRequestID(requestID))
		return
	}

	var aad []byte
	if body.AAD != "" {
		aad, err = base64.StdEncoding.DecodeString(body.AAD)
		if err != nil {
			writeError(w, apperrors.InvalidRequest("invalid base64 aad").WithRequestID(requestID))
			return
		}
	}

	req := cryptosvc.DecryptRequest{
		TenantID:  tenantID,
		Envelope:  envelopeBytes,
		AAD:       aad,
		CallerID:  callerID,
		RequestID: requestID,
	}

	resp, err := h.service.Decrypt(ctx, req)
	if err != nil {
		writeError(w, withRequestID(err, requestID))
		return
	}

	writeJSON(w, http.StatusOK, decryptResponse{
		Plaintext: base64.StdEncoding.EncodeToString(resp.Plaintext),
		KID:       resp.KID,
		Algorithm: resp.Algorithm,
	})
}

// GenerateDataKey 处理 POST /api/v1/tenants/{tenantID}/crypto/datakey 请求。
//
// 从请求体解析生成参数，调用 Service.GenerateDataKey 生成数据密钥，
// 返回 200 OK 与明文 DEK、包装 DEK。需要 datakey:generate scope。
func (h *Handler) GenerateDataKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.PathValue("tenantID")
	requestID := middleware.GetRequestID(ctx)
	callerID := callerFromContext(ctx)

	var body generateDataKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.InvalidRequest("invalid json body").WithRequestID(requestID))
		return
	}

	if body.KeyID == "" {
		writeError(w, apperrors.InvalidRequest("key_id is required").WithRequestID(requestID))
		return
	}

	req := cryptosvc.GenerateDataKeyRequest{
		TenantID:  tenantID,
		KeyID:     body.KeyID,
		Algorithm: body.Algorithm,
		CallerID:  callerID,
		RequestID: requestID,
	}

	resp, err := h.service.GenerateDataKey(ctx, req)
	if err != nil {
		writeError(w, withRequestID(err, requestID))
		return
	}

	writeJSON(w, http.StatusOK, generateDataKeyResponse{
		PlaintextDEK: base64.StdEncoding.EncodeToString(resp.PlaintextDEK),
		WrappedDEK:   base64.StdEncoding.EncodeToString(resp.WrappedDEK),
		KID:          resp.KID,
	})
}

// callerFromContext 从 context 中提取调用方标识（Principal.CallerID）。
func callerFromContext(ctx context.Context) string {
	v := middleware.GetPrincipal(ctx)
	if v == nil {
		return ""
	}
	p, ok := v.(*principal.Principal)
	if !ok || p == nil {
		return ""
	}
	return p.CallerID
}

// writeJSON 将响应以 JSON 写入 HTTP 响应。
func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeError 将错误以 JSON 写入 HTTP 响应。
func writeError(w http.ResponseWriter, err error) {
	if ae, ok := err.(*apperrors.Error); ok {
		apperrors.WriteJSON(w, ae)
		return
	}
	apperrors.WriteJSON(w, apperrors.Internal(err))
}

// withRequestID 为错误附加 request_id（若错误为 *apperrors.Error）。
func withRequestID(err error, requestID string) error {
	if ae, ok := err.(*apperrors.Error); ok {
		return ae.WithRequestID(requestID)
	}
	return err
}
