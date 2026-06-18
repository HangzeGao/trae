package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Code 是稳定的错误码标识，客户端应基于此做分支判断。
type Code string

const (
	CodeAuthFailed        Code = "AUTH_FAILED"
	CodePermissionDenied  Code = "PERMISSION_DENIED"
	CodeKeyNotFound       Code = "KEY_NOT_FOUND"
	CodeKeyDisabled       Code = "KEY_DISABLED"
	CodeKeyDestroyed      Code = "KEY_DESTROYED"
	CodePolicyDenied      Code = "POLICY_DENIED"
	CodeAADMismatch       Code = "AAD_MISMATCH"
	CodeEnvelopeInvalid   Code = "ENVELOPE_INVALID"
	CodeNonceExhausted    Code = "NONCE_EXHAUSTED"
	CodeTPMUnavailable    Code = "TPM_UNAVAILABLE"
	CodeDBConflict        Code = "DB_CONFLICT"
	CodeAuditUnavailable  Code = "AUDIT_UNAVAILABLE"
	CodeRateLimited       Code = "RATE_LIMITED"
	CodeInternal          Code = "INTERNAL"
	CodeInvalidRequest    Code = "INVALID_REQUEST"
	CodeIdempotencyConflict Code = "IDEMPOTENCY_CONFLICT"
)

// Error 实现 error 接口，携带稳定 code、HTTP 状态、可重试性和通用 message。
type Error struct {
	Code      Code   `json:"code"`
	Message   string `json:"message"`
	HTTPStatus int   `json:"-"`
	Retryable bool   `json:"retryable"`
	RequestID string `json:"request_id,omitempty"`
	// internalCause 仅供服务端日志使用，不序列化到响应。
	internalCause error `json:"-"`
}

func (e *Error) Error() string {
	if e.internalCause != nil {
		return fmt.Sprintf("%s: %s (cause: %v)", e.Code, e.Message, e.internalCause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.internalCause }

// WithCause 附加内部原因，仅用于日志，不暴露给客户端。
func (e *Error) WithCause(cause error) *Error {
	e.internalCause = cause
	return e
}

// WithRequestID 附加请求 ID。
func (e *Error) WithRequestID(id string) *Error {
	e.RequestID = id
	return e
}

// New 构造一个错误。
func New(code Code, message string, httpStatus int, retryable bool) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Retryable:  retryable,
	}
}

// 预定义错误构造器，对应第 18.1 节错误码表。

func AuthFailed(cause error) *Error {
	return New(CodeAuthFailed, "authentication failed", http.StatusUnauthorized, false).WithCause(cause)
}

func PermissionDenied() *Error {
	return New(CodePermissionDenied, "permission denied", http.StatusForbidden, false)
}

// KeyNotFound 统一返回 PERMISSION_DENIED 以防跨租户存在性枚举（HA-11）。
// 仅在同租户上下文内可区分不存在，由调用方决定是否使用此构造器。
func KeyNotFound() *Error {
	return New(CodeKeyNotFound, "key not found", http.StatusNotFound, false)
}

// CrossTenantDenied 跨租户访问统一返回 PERMISSION_DENIED，不区分不存在与无权限（HA-11）。
func CrossTenantDenied() *Error {
	return New(CodePermissionDenied, "permission denied", http.StatusForbidden, false)
}

func KeyDisabled() *Error {
	return New(CodeKeyDisabled, "key is disabled for this operation", http.StatusConflict, false)
}

func KeyDestroyed() *Error {
	return New(CodeKeyDestroyed, "key is destroyed", http.StatusGone, false)
}

func PolicyDenied(reason string) *Error {
	return New(CodePolicyDenied, "policy denied: "+reason, http.StatusBadRequest, false)
}

func AADMismatch() *Error {
	return New(CodeAADMismatch, "AAD verification failed", http.StatusBadRequest, false)
}

func EnvelopeInvalid(cause error) *Error {
	return New(CodeEnvelopeInvalid, "envelope format invalid", http.StatusBadRequest, false).WithCause(cause)
}

func NonceExhausted() *Error {
	return New(CodeNonceExhausted, "nonce lease exhausted or renewal failed", http.StatusTooManyRequests, true)
}

func TPMUnavailable(cause error) *Error {
	return New(CodeTPMUnavailable, "TPM/vTPM unavailable", http.StatusServiceUnavailable, true).WithCause(cause)
}

func DBConflict(cause error) *Error {
	return New(CodeDBConflict, "concurrent state conflict", http.StatusConflict, true).WithCause(cause)
}

func AuditUnavailable(cause error) *Error {
	return New(CodeAuditUnavailable, "audit system unavailable for high-risk operation", http.StatusServiceUnavailable, true).WithCause(cause)
}

func RateLimited() *Error {
	return New(CodeRateLimited, "rate limit exceeded", http.StatusTooManyRequests, true)
}

func Internal(cause error) *Error {
	return New(CodeInternal, "internal error", http.StatusInternalServerError, false).WithCause(cause)
}

func InvalidRequest(reason string) *Error {
	return New(CodeInvalidRequest, "invalid request: "+reason, http.StatusBadRequest, false)
}

func IdempotencyConflict() *Error {
	return New(CodeIdempotencyConflict, "idempotency key conflict", http.StatusConflict, false)
}

// ErrorResponse 是对外 JSON 错误响应体。
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Retryable bool   `json:"retryable"`
}

// WriteJSON 将错误以 JSON 写入 HTTP 响应，message 使用通用描述（HA-11）。
func WriteJSON(w http.ResponseWriter, e *Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.HTTPStatus)
	resp := ErrorResponse{
		Error: ErrorBody{
			Code:      string(e.Code),
			Message:   e.Message,
			RequestID: e.RequestID,
			Retryable: e.Retryable,
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}
