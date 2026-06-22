// Package crypto implements the Crypto API per design §12.3.
package crypto

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/kvlt/key-vault/internal/api/middleware"
	"github.com/kvlt/key-vault/internal/application/crypto"
	apperrors "github.com/kvlt/key-vault/internal/errorsx"
)

// Handler is the crypto API HTTP handler.
type Handler struct {
	svc *crypto.Service
}

// New constructs a crypto handler.
func New(svc *crypto.Service) *Handler {
	return &Handler{svc: svc}
}

// Routes registers the crypto routes.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /ui/api/v1/crypto/encrypt", h.encrypt)
	mux.HandleFunc("POST /ui/api/v1/crypto/decrypt", h.decrypt)
	mux.HandleFunc("POST /ui/api/v1/data-keys", h.generateDataKey)
}

type encryptReq struct {
	TenantID  string            `json:"tenant_id"`
	KeyID     string            `json:"key_id"`
	Plaintext string            `json:"plaintext"` // base64
	AAD       map[string]string `json:"aad,omitempty"`
	NodeID    string            `json:"node_id,omitempty"`
}

type encryptResp struct {
	KeyID      string `json:"key_id"`
	KeyVersion uint32 `json:"key_version"`
	SuiteID    string `json:"suite_id"`
	Ciphertext string `json:"ciphertext"` // base64 envelope
}

func (h *Handler) encrypt(w http.ResponseWriter, r *http.Request) {
	p := middleware.PrincipalFromContext(r.Context())
	if p == nil {
		writeErr(w, 401, "AUTH_FAILED", "no principal")
		return
	}
	if !p.HasScope("crypto:encrypt") {
		writeErr(w, 403, "PERMISSION_DENIED", "missing scope crypto:encrypt")
		return
	}
	body := middleware.BodyFromContext(r.Context())
	var req encryptReq
	if err := middleware.DecodeJSONStrict(body, &req); err != nil {
		writeErr(w, 400, "BAD_REQUEST", "invalid json")
		return
	}
	if p.TenantID != "" && req.TenantID != p.TenantID {
		writeErr(w, 403, "PERMISSION_DENIED", "tenant mismatch")
		return
	}
	pt, err := base64.StdEncoding.DecodeString(req.Plaintext)
	if err != nil {
		writeErr(w, 400, "BAD_REQUEST", "plaintext not base64")
		return
	}
	nodeID := req.NodeID
	if nodeID == "" {
		nodeID = p.NodeID
	}
	if nodeID == "" {
		writeErr(w, 400, "BAD_REQUEST", "node_id required")
		return
	}
	res, err := h.svc.Encrypt(r.Context(), crypto.EncryptCommand{
		TenantID:   req.TenantID,
		KeyID:      req.KeyID,
		Plaintext:  pt,
		AAD:        crypto.CallerAADInput{ResourceID: req.AAD["resource_id"], Purpose: req.AAD["purpose"]},
		NodeID:     nodeID,
		PrincipalID: p.ID,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, encryptResp{
		KeyID:      res.KeyID,
		KeyVersion: res.KeyVersion,
		SuiteID:    res.SuiteID,
		Ciphertext: base64.StdEncoding.EncodeToString(res.Ciphertext),
	})
}

type decryptReq struct {
	TenantID   string            `json:"tenant_id"`
	Ciphertext string            `json:"ciphertext"` // base64 envelope
	AAD        map[string]string `json:"aad,omitempty"`
}

type decryptResp struct {
	KeyID      string `json:"key_id"`
	KeyVersion uint32 `json:"key_version"`
	Plaintext  string `json:"plaintext"` // base64
}

func (h *Handler) decrypt(w http.ResponseWriter, r *http.Request) {
	p := middleware.PrincipalFromContext(r.Context())
	if p == nil {
		writeErr(w, 401, "AUTH_FAILED", "no principal")
		return
	}
	if !p.HasScope("crypto:decrypt") {
		writeErr(w, 403, "PERMISSION_DENIED", "missing scope crypto:decrypt")
		return
	}
	body := middleware.BodyFromContext(r.Context())
	var req decryptReq
	if err := middleware.DecodeJSONStrict(body, &req); err != nil {
		writeErr(w, 400, "BAD_REQUEST", "invalid json")
		return
	}
	if p.TenantID != "" && req.TenantID != p.TenantID {
		writeErr(w, 403, "PERMISSION_DENIED", "tenant mismatch")
		return
	}
	ct, err := base64.StdEncoding.DecodeString(req.Ciphertext)
	if err != nil {
		writeErr(w, 400, "BAD_REQUEST", "ciphertext not base64")
		return
	}
	res, err := h.svc.Decrypt(r.Context(), crypto.DecryptCommand{
		TenantID:   req.TenantID,
		Ciphertext: ct,
		AAD:        crypto.CallerAADInput{ResourceID: req.AAD["resource_id"], Purpose: req.AAD["purpose"]},
		PrincipalID: p.ID,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, decryptResp{
		KeyID:      res.KeyID,
		KeyVersion: res.KeyVersion,
		Plaintext:  base64.StdEncoding.EncodeToString(res.Plaintext),
	})
}

type dataKeyReq struct {
	TenantID          string            `json:"tenant_id"`
	KeyID             string            `json:"key_id"`
	Purpose           string            `json:"purpose"`
	TTLSeconds        int               `json:"ttl_seconds,omitempty"`
	EncryptionContext map[string]string `json:"encryption_context,omitempty"`
	Caller            string            `json:"caller"`
}

type dataKeyResp struct {
	KeyID                 string    `json:"key_id"`
	KeyVersion            uint32    `json:"key_version"`
	PlaintextDataKey      string    `json:"plaintext_data_key"`
	WrappedDataKey        string    `json:"wrapped_data_key"`
	SuiteID               string    `json:"suite_id"`
	ClientZeroizeBy       time.Time `json:"client_zeroize_by"`
	EncryptionContextHash string    `json:"encryption_context_hash"`
}

func (h *Handler) generateDataKey(w http.ResponseWriter, r *http.Request) {
	p := middleware.PrincipalFromContext(r.Context())
	if p == nil {
		writeErr(w, 401, "AUTH_FAILED", "no principal")
		return
	}
	// Per HA-10: datakey:generate is a separate scope from crypto:encrypt/decrypt.
	if !p.HasScope("datakey:generate") {
		writeErr(w, 403, "PERMISSION_DENIED", "missing scope datakey:generate")
		return
	}
	body := middleware.BodyFromContext(r.Context())
	var req dataKeyReq
	if err := middleware.DecodeJSONStrict(body, &req); err != nil {
		writeErr(w, 400, "BAD_REQUEST", "invalid json")
		return
	}
	if p.TenantID != "" && req.TenantID != p.TenantID {
		writeErr(w, 403, "PERMISSION_DENIED", "tenant mismatch")
		return
	}
	ttl := time.Duration(req.TTLSeconds) * time.Second
	caller := req.Caller
	if caller == "" {
		caller = "direct"
	}
	res, err := h.svc.GenerateDataKey(r.Context(), crypto.GenerateDataKeyCommand{
		TenantID:          req.TenantID,
		KeyID:             req.KeyID,
		Purpose:           req.Purpose,
		TTL:               ttl,
		EncryptionContext: req.EncryptionContext,
		PrincipalID:       p.ID,
		Caller:            caller,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, dataKeyResp{
		KeyID:                 res.KeyID,
		KeyVersion:            res.KeyVersion,
		PlaintextDataKey:      base64.StdEncoding.EncodeToString(res.PlaintextDataKey),
		WrappedDataKey:        base64.StdEncoding.EncodeToString(res.WrappedDataKey),
		SuiteID:               res.SuiteID,
		ClientZeroizeBy:       res.ClientZeroizeBy,
		EncryptionContextHash: res.EncryptionContextHash,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	b, _ := json.Marshal(v)
	w.Write(b)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	b, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": msg,
		},
	})
	w.Write(b)
}

func writeServiceError(w http.ResponseWriter, err error) {
	code := apperrors.AsCode(err)
	status := apperrors.HTTPStatus(code)
	msg := "request failed"
	if code == apperrors.CodeInternal {
		msg = "internal error"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	b, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"code":      string(code),
			"message":   msg,
			"retryable": apperrors.AsRetryable(err),
		},
	})
	w.Write(b)
}

// strings import workaround.
var _ = strings.TrimSpace
