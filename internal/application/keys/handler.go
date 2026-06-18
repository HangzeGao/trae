// Package keys 的 handler.go 实现密钥管理的 HTTP 处理器。
//
// Handler 注册以下路由（均需认证与租户匹配）：
//   - POST   /api/v1/tenants/{tenantID}/keys                      创建密钥（keys:write）
//   - GET    /api/v1/tenants/{tenantID}/keys/{keyID}              查询密钥（keys:read）
//   - POST   /api/v1/tenants/{tenantID}/keys/{keyID}/disable      禁用密钥（keys:write）
//   - POST   /api/v1/tenants/{tenantID}/keys/{keyID}/enable       启用密钥（keys:write）
//   - POST   /api/v1/tenants/{tenantID}/keys/{keyID}/destroy      销毁密钥（keys:write）
//   - POST   /api/v1/tenants/{tenantID}/keys/{keyID}/rotate       轮转密钥（keys:write）
//   - GET    /api/v1/tenants/{tenantID}/keys                      列出密钥（keys:read）
//
// 所有路由通过 RequireAuth -> RequireTenantMatch -> RequireScope 中间件链保护，
// 确保认证、租户隔离与权限校验。请求/响应使用 JSON 格式。
package keys

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/HangzeGao/trae/key-vault/internal/api/middleware"
	"github.com/HangzeGao/trae/key-vault/internal/auth/principal"
	apperrors "github.com/HangzeGao/trae/key-vault/internal/errors"
	"github.com/HangzeGao/trae/key-vault/internal/observability"
)

// 路由路径常量。
const (
	routeBase          = "/api/v1/tenants/{tenantID}/keys"
	routeKey           = "/api/v1/tenants/{tenantID}/keys/{keyID}"
	routeKeyDisable    = "/api/v1/tenants/{tenantID}/keys/{keyID}/disable"
	routeKeyEnable     = "/api/v1/tenants/{tenantID}/keys/{keyID}/enable"
	routeKeyDestroy    = "/api/v1/tenants/{tenantID}/keys/{keyID}/destroy"
	routeKeyRotate     = "/api/v1/tenants/{tenantID}/keys/{keyID}/rotate"
)

// 权限 scope 常量。
const (
	scopeKeysRead  = "keys:read"
	scopeKeysWrite = "keys:write"
)

// Handler 是密钥管理的 HTTP 处理器。
//
// 通过 RegisterRoutes 注册路由，所有路由经认证、租户匹配、scope 校验中间件保护。
type Handler struct {
	service *Service
	auth    *middleware.AuthMiddleware
	log     *observability.Logger
}

// NewHandler 创建一个密钥管理 HTTP 处理器实例。
//
// service 提供密钥生命周期业务逻辑；auth 提供认证授权中间件；
// log 用于结构化日志输出。
func NewHandler(service *Service, auth *middleware.AuthMiddleware, log *observability.Logger) *Handler {
	return &Handler{
		service: service,
		auth:    auth,
		log:     log,
	}
}

// RegisterRoutes 在 mux 上注册密钥管理路由。
//
// 每条路由依次应用 RequireAuth、RequireTenantMatch、RequireScope 中间件，
// 确保认证、租户隔离与权限校验。使用 Go 1.22+ 的方法+路径模式注册。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// 创建密钥：keys:write
	mux.Handle("POST "+routeBase,
		h.wrap(scopeKeysWrite, http.HandlerFunc(h.CreateKey)))

	// 列出密钥：keys:read
	mux.Handle("GET "+routeBase,
		h.wrap(scopeKeysRead, http.HandlerFunc(h.ListKeys)))

	// 查询密钥：keys:read
	mux.Handle("GET "+routeKey,
		h.wrap(scopeKeysRead, http.HandlerFunc(h.GetKey)))

	// 禁用密钥：keys:write
	mux.Handle("POST "+routeKeyDisable,
		h.wrap(scopeKeysWrite, http.HandlerFunc(h.DisableKey)))

	// 启用密钥：keys:write
	mux.Handle("POST "+routeKeyEnable,
		h.wrap(scopeKeysWrite, http.HandlerFunc(h.EnableKey)))

	// 销毁密钥：keys:write
	mux.Handle("POST "+routeKeyDestroy,
		h.wrap(scopeKeysWrite, http.HandlerFunc(h.DestroyKey)))

	// 轮转密钥：keys:write
	mux.Handle("POST "+routeKeyRotate,
		h.wrap(scopeKeysWrite, http.HandlerFunc(h.RotateKey)))
}

// wrap 包装中间件链：RequireAuth -> RequireTenantMatch -> RequireScope -> handler。
//
// 返回最终包装后的 http.Handler，确保认证、租户隔离与权限校验依次执行。
func (h *Handler) wrap(scope string, handler http.Handler) http.Handler {
	return h.auth.RequireAuth(
		h.auth.RequireTenantMatch(
			h.auth.RequireScope(scope)(handler)))
}

// createKeyRequest 创建密钥请求体。
type createKeyRequest struct {
	Name             string   `json:"name"`
	Algorithm        string   `json:"algorithm"`
	Purpose          string   `json:"purpose"`
	RequireAAD       bool     `json:"require_aad"`
	MaxPlaintextSize int64    `json:"max_plaintext_size"`
	AllowedCallers   []string `json:"allowed_callers"`
	IdempotencyKey   string   `json:"idempotency_key"`
}

// createKeyResponse 创建密钥响应体。
type createKeyResponse struct {
	KeyID          string `json:"key_id"`
	KID            string `json:"kid"`
	Algorithm      string `json:"algorithm"`
	Status         string `json:"status"`
	CurrentVersion int    `json:"current_version"`
	CreatedAt      string `json:"created_at"`
}

// keyInfoResponse 密钥信息响应体。
type keyInfoResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Algorithm      string `json:"algorithm"`
	Purpose        string `json:"purpose"`
	Status         string `json:"status"`
	CurrentVersion int    `json:"current_version"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// rotateKeyRequest 轮转密钥请求体。
type rotateKeyRequest struct {
	Reason string `json:"reason"`
}

// rotateKeyResponse 轮转密钥响应体。
type rotateKeyResponse struct {
	NewVersion int    `json:"new_version"`
	KID        string `json:"kid"`
	RotatedAt  string `json:"rotated_at"`
}

// listKeysResponse 列出密钥响应体。
type listKeysResponse struct {
	Keys []keyInfoResponse `json:"keys"`
}

// CreateKey 处理 POST /api/v1/tenants/{tenantID}/keys 请求。
//
// 从请求体解析创建参数，调用 Service.CreateKey 创建密钥，
// 返回 201 Created 与密钥信息。需要 keys:write scope。
func (h *Handler) CreateKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.PathValue("tenantID")
	requestID := middleware.GetRequestID(ctx)
	actor := actorFromContext(ctx)

	var body createKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.InvalidRequest("invalid json body").WithRequestID(requestID))
		return
	}

	if body.Name == "" || body.Algorithm == "" || body.Purpose == "" {
		writeError(w, apperrors.InvalidRequest("name, algorithm, purpose are required").WithRequestID(requestID))
		return
	}

	req := CreateKeyRequest{
		TenantID:         tenantID,
		Name:             body.Name,
		Algorithm:        body.Algorithm,
		Purpose:          body.Purpose,
		CreatedBy:        actor,
		RequestID:        requestID,
		IdempotencyKey:   body.IdempotencyKey,
		RequireAAD:       body.RequireAAD,
		MaxPlaintextSize: body.MaxPlaintextSize,
		AllowedCallers:   body.AllowedCallers,
	}

	result, err := h.service.CreateKey(ctx, req)
	if err != nil {
		writeError(w, withRequestID(err, requestID))
		return
	}

	resp := createKeyResponse{
		KeyID:          result.KeyID,
		KID:            result.KID,
		Algorithm:      result.Algorithm,
		Status:         result.Status,
		CurrentVersion: result.CurrentVersion,
		CreatedAt:      result.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	writeJSON(w, http.StatusCreated, resp)
}

// GetKey 处理 GET /api/v1/tenants/{tenantID}/keys/{keyID} 请求。
//
// 从路径参数提取租户 ID 与密钥 ID，调用 Service.GetKey 查询密钥信息。
// 需要 keys:read scope。
func (h *Handler) GetKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.PathValue("tenantID")
	keyID := r.PathValue("keyID")
	requestID := middleware.GetRequestID(ctx)

	info, err := h.service.GetKey(ctx, tenantID, keyID)
	if err != nil {
		writeError(w, withRequestID(err, requestID))
		return
	}

	resp := toKeyInfoResponse(info)
	writeJSON(w, http.StatusOK, resp)
}

// DisableKey 处理 POST /api/v1/tenants/{tenantID}/keys/{keyID}/disable 请求。
//
// 调用 Service.DisableKey 禁用密钥，成功返回 204 No Content。
// 需要 keys:write scope。
func (h *Handler) DisableKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.PathValue("tenantID")
	keyID := r.PathValue("keyID")
	requestID := middleware.GetRequestID(ctx)
	actor := actorFromContext(ctx)

	if err := h.service.DisableKey(ctx, tenantID, keyID, actor, requestID); err != nil {
		writeError(w, withRequestID(err, requestID))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// EnableKey 处理 POST /api/v1/tenants/{tenantID}/keys/{keyID}/enable 请求。
//
// 调用 Service.EnableKey 启用密钥，成功返回 204 No Content。
// 需要 keys:write scope。
func (h *Handler) EnableKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.PathValue("tenantID")
	keyID := r.PathValue("keyID")
	requestID := middleware.GetRequestID(ctx)
	actor := actorFromContext(ctx)

	if err := h.service.EnableKey(ctx, tenantID, keyID, actor, requestID); err != nil {
		writeError(w, withRequestID(err, requestID))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DestroyKey 处理 POST /api/v1/tenants/{tenantID}/keys/{keyID}/destroy 请求。
//
// 调用 Service.DestroyKey 销毁密钥（高风险操作），成功返回 204 No Content。
// 需要 keys:write scope。
func (h *Handler) DestroyKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.PathValue("tenantID")
	keyID := r.PathValue("keyID")
	requestID := middleware.GetRequestID(ctx)
	actor := actorFromContext(ctx)

	if err := h.service.DestroyKey(ctx, tenantID, keyID, actor, requestID); err != nil {
		writeError(w, withRequestID(err, requestID))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RotateKey 处理 POST /api/v1/tenants/{tenantID}/keys/{keyID}/rotate 请求。
//
// 从请求体解析轮转原因，调用 Service.RotateKey 轮转密钥（高风险操作），
// 返回 200 OK 与新版本信息。需要 keys:write scope。
func (h *Handler) RotateKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.PathValue("tenantID")
	keyID := r.PathValue("keyID")
	requestID := middleware.GetRequestID(ctx)
	actor := actorFromContext(ctx)

	var body rotateKeyRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, apperrors.InvalidRequest("invalid json body").WithRequestID(requestID))
			return
		}
	}

	result, err := h.service.RotateKey(ctx, tenantID, keyID, actor, body.Reason, requestID)
	if err != nil {
		writeError(w, withRequestID(err, requestID))
		return
	}

	resp := rotateKeyResponse{
		NewVersion: result.NewVersion,
		KID:        result.KID,
		RotatedAt:  result.RotatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	writeJSON(w, http.StatusOK, resp)
}

// ListKeys 处理 GET /api/v1/tenants/{tenantID}/keys 请求。
//
// 从查询参数解析 limit/offset 分页参数，调用 Service.ListKeys 列出密钥。
// 需要 keys:read scope。
func (h *Handler) ListKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := r.PathValue("tenantID")
	requestID := middleware.GetRequestID(ctx)

	limit := parseQueryInt(r, "limit", 0)
	offset := parseQueryInt(r, "offset", 0)

	list, err := h.service.ListKeys(ctx, tenantID, limit, offset)
	if err != nil {
		writeError(w, withRequestID(err, requestID))
		return
	}

	keys := make([]keyInfoResponse, 0, len(list))
	for _, info := range list {
		keys = append(keys, toKeyInfoResponse(info))
	}
	writeJSON(w, http.StatusOK, listKeysResponse{Keys: keys})
}

// actorFromContext 从 context 中提取调用方标识（Principal.CallerID）。
func actorFromContext(ctx context.Context) string {
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

// toKeyInfoResponse 将 KeyInfo 转换为 HTTP 响应体。
func toKeyInfoResponse(info *KeyInfo) keyInfoResponse {
	return keyInfoResponse{
		ID:             info.ID,
		Name:           info.Name,
		Algorithm:      info.Algorithm,
		Purpose:        info.Purpose,
		Status:         info.Status,
		CurrentVersion: info.CurrentVersion,
		CreatedAt:      info.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      info.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
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

// parseQueryInt 从查询参数解析整数值，缺失或无效时返回默认值。
func parseQueryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
