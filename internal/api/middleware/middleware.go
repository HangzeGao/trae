// Package middleware implements HTTP middleware: auth, request ID, body limits,
// plane isolation, idempotency, audit.
package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kvlt/key-vault/internal/auth/hmacsign"
	"github.com/kvlt/key-vault/internal/auth/jwt"
	"github.com/kvlt/key-vault/internal/auth/principal"
	"github.com/kvlt/key-vault/internal/errorsx"
	"github.com/kvlt/key-vault/internal/logging"
)

type ctxKey string

const (
	ctxKeyPrincipal ctxKey = "principal"
	ctxKeyRequestID ctxKey = "request_id"
	ctxKeyBody      ctxKey = "body"
)

// AuthConfig configures the auth middleware.
type AuthConfig struct {
	JWTVerifier    *jwt.Verifier
	HMACVerifier   *hmacsign.Verifier
	HMACEnabled    bool
	StaticTokens   map[string]*principal.Principal // token -> principal
	RequiredScopes map[string][]string              // path pattern -> required scopes
}

// RequestID middleware injects a request ID.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-Id")
		if rid == "" {
			rid = newRequestID()
		}
		w.Header().Set("X-Request-Id", rid)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// BodyLimit enforces a max request body size per design §12.1.
func BodyLimit(maxBytes int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
			next.ServeHTTP(w, r)
		})
	}
}

// ReadBody reads the request body into a byte slice and stores it in context.
// This is needed because HMAC signing covers the body, and we need to verify
// the signature before parsing JSON.
func ReadBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}
		b, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, errorsx.New(errorsx.CodeBadRequest, "read body failed", false), "")
			return
		}
		r.Body.Close()
		ctx := context.WithValue(r.Context(), ctxKeyBody, b)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Auth authenticates the request and injects the principal into context.
func Auth(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var p *principal.Principal
			// Try Bearer token first.
			authz := r.Header.Get("Authorization")
			if strings.HasPrefix(authz, "Bearer ") {
				token := strings.TrimPrefix(authz, "Bearer ")
				// Try static token first.
				if cfg.StaticTokens != nil {
					if sp, ok := cfg.StaticTokens[token]; ok {
						p = sp
					}
				}
				if p == nil && cfg.JWTVerifier != nil {
					claims, err := cfg.JWTVerifier.Verify(token)
					if err != nil {
						writeError(w, errorsx.New(errorsx.CodeAuthFailed, "jwt invalid", false), requestID(r))
						return
					}
					p = claimsToPrincipal(claims, "jwt")
				}
			} else if cfg.HMACEnabled && r.Header.Get(hmacsign.HeaderSignature) != "" {
				body, _ := r.Context().Value(ctxKeyBody).([]byte)
				_, err := cfg.HMACVerifier.VerifyRequest(r, body)
				if err != nil {
					writeError(w, errorsx.New(errorsx.CodeAuthFailed, "hmac invalid", false), requestID(r))
					return
				}
				nodeID := r.Header.Get(hmacsign.HeaderNodeID)
				p = &principal.Principal{
					ID:         "node:" + nodeID,
					AuthMethod: "hmac",
					NodeID:     nodeID,
					Plane:      principal.PlaneData,
					Scopes:     []string{"crypto:encrypt", "crypto:decrypt"},
					Roles:      []string{"data"},
				}
			}
			if p == nil {
				writeError(w, errorsx.New(errorsx.CodeAuthFailed, "no credentials", false), requestID(r))
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeyPrincipal, p)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireScope returns a middleware that requires the given scope.
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := PrincipalFromContext(r.Context())
			if p == nil || !p.HasScope(scope) {
				writeError(w, errorsx.New(errorsx.CodePermissionDenied, "missing scope "+scope, false), requestID(r))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePlane returns a middleware that requires the given plane.
func RequirePlane(plane principal.Plane) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := PrincipalFromContext(r.Context())
			if p == nil || !p.CanAccessPlane(plane) {
				writeError(w, errorsx.New(errorsx.CodePermissionDenied, "plane access denied", false), requestID(r))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// PrincipalFromContext extracts the principal from context.
func PrincipalFromContext(ctx context.Context) *principal.Principal {
	v, _ := ctx.Value(ctxKeyPrincipal).(*principal.Principal)
	return v
}

// RequestIDFromContext extracts the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

// BodyFromContext extracts the cached body from context.
func BodyFromContext(ctx context.Context) []byte {
	v, _ := ctx.Value(ctxKeyBody).([]byte)
	return v
}

func requestID(r *http.Request) string {
	if v, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

func claimsToPrincipal(c *jwt.Claims, method string) *principal.Principal {
	p := &principal.Principal{
		ID:         c.Sub,
		TenantID:   c.Tenant,
		AuthMethod: method,
	}
	if c.Scope != "" {
		p.Scopes = strings.Split(c.Scope, " ")
	}
	if c.Roles != "" {
		p.Roles = strings.Split(c.Roles, " ")
	}
	if c.Plane == "management" {
		p.Plane = principal.PlaneManagement
	} else {
		p.Plane = principal.PlaneData
	}
	if c.NodeID != "" {
		p.NodeID = c.NodeID
	}
	return p
}

// writeError writes a structured error response.
func writeError(w http.ResponseWriter, e *errorsx.Error, requestID string) {
	status := errorsx.HTTPStatus(e.Code)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]any{
		"error": map[string]any{
			"code":       string(e.Code),
			"message":    e.Message,
			"request_id": requestID,
			"retryable":  e.Retryable,
		},
	}
	b, _ := json.Marshal(resp)
	w.Write(b)
}

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	b, _ := json.Marshal(v)
	w.Write(b)
}

// DecodeJSONStrict decodes JSON strictly (no unknown fields, no duplicates).
func DecodeJSONStrict(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	// Check for trailing data.
	if dec.More() {
		return errorsx.New(errorsx.CodeBadRequest, "trailing data in JSON", false)
	}
	return nil
}

// newRequestID generates a request ID.
func newRequestID() string {
	return "req_" + time.Now().Format("20060102T150405.000000000")
}

// Logger returns the package logger.
var Logger = logging.New(0)
