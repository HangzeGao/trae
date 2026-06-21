// Package admin implements the Management API per design §12.2.
package admin

import (
	"encoding/json"
	"net/http"

	"github.com/kvlt/key-vault/internal/api/middleware"
	"github.com/kvlt/key-vault/internal/application/keys"
	"github.com/kvlt/key-vault/internal/application/nodes"
	"github.com/kvlt/key-vault/internal/auth/principal"
	"github.com/kvlt/key-vault/internal/errorsx"
	"github.com/kvlt/key-vault/internal/repository/models"
)

// Handler is the management API HTTP handler.
type Handler struct {
	keys  *keys.Service
	nodes *nodes.Service
}

// New constructs a management handler.
func New(keys *keys.Service, nodes *nodes.Service) *Handler {
	return &Handler{keys: keys, nodes: nodes}
}

// Routes registers the management routes.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/keys", h.createKey)
	mux.HandleFunc("GET /v1/keys", h.listKeys)
	mux.HandleFunc("GET /v1/keys/{key_id}", h.getKey)
	mux.HandleFunc("POST /v1/keys/{key_id}/enable", h.enableKey)
	mux.HandleFunc("POST /v1/keys/{key_id}/disable", h.disableKey)
	mux.HandleFunc("POST /v1/keys/{key_id}/rotate", h.rotateKey)
	mux.HandleFunc("POST /v1/keys/{key_id}/schedule-destroy", h.scheduleDestroy)
	mux.HandleFunc("POST /v1/nodes/register", h.registerNode)
	mux.HandleFunc("POST /v1/nodes/{node_id}/mark-ready", h.markReady)
	mux.HandleFunc("POST /v1/nodes/{node_id}/revoke", h.revokeNode)
	mux.HandleFunc("GET /v1/nodes/{node_id}", h.getNode)
}

type createKeyReq struct {
	TenantID string            `json:"tenant_id"`
	Name     string            `json:"name"`
	Purpose  string            `json:"purpose"`
	PolicyID string            `json:"policy_id"`
	SuiteID  string            `json:"suite_id"`
	Tags     map[string]string `json:"tags,omitempty"`
}

func (h *Handler) createKey(w http.ResponseWriter, r *http.Request) {
	p := middleware.PrincipalFromContext(r.Context())
	if p == nil {
		writeErr(w, 401, "AUTH_FAILED", "no principal")
		return
	}
	if !p.HasScope("keys:manage") {
		writeErr(w, 403, "PERMISSION_DENIED", "missing scope keys:manage")
		return
	}
	body := middleware.BodyFromContext(r.Context())
	var req createKeyReq
	if err := middleware.DecodeJSONStrict(body, &req); err != nil {
		writeErr(w, 400, "BAD_REQUEST", "invalid json")
		return
	}
	if p.TenantID != "" && req.TenantID != p.TenantID {
		writeErr(w, 403, "PERMISSION_DENIED", "tenant mismatch")
		return
	}
	cmd := keys.CreateKeyCommand{
		TenantID:    req.TenantID,
		Name:        req.Name,
		Purpose:     req.Purpose,
		PolicyID:    req.PolicyID,
		SuiteID:     req.SuiteID,
		Tags:        req.Tags,
		PrincipalID: p.ID,
	}
	dto, err := h.keys.CreateKey(r.Context(), cmd)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 201, dto)
}

func (h *Handler) getKey(w http.ResponseWriter, r *http.Request) {
	p := middleware.PrincipalFromContext(r.Context())
	keyID := r.PathValue("key_id")
	tenantID := tenantFromQueryOrPrincipal(r, p)
	dto, err := h.keys.GetKey(r.Context(), tenantID, keyID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, dto)
}

func (h *Handler) listKeys(w http.ResponseWriter, r *http.Request) {
	p := middleware.PrincipalFromContext(r.Context())
	tenantID := tenantFromQueryOrPrincipal(r, p)
	dtos, err := h.keys.ListKeys(r.Context(), tenantID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"keys": dtos})
}

func (h *Handler) enableKey(w http.ResponseWriter, r *http.Request) {
	p := middleware.PrincipalFromContext(r.Context())
	keyID := r.PathValue("key_id")
	tenantID := tenantFromQueryOrPrincipal(r, p)
	if err := h.keys.EnableKey(r.Context(), tenantID, keyID, p.ID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) disableKey(w http.ResponseWriter, r *http.Request) {
	p := middleware.PrincipalFromContext(r.Context())
	keyID := r.PathValue("key_id")
	tenantID := tenantFromQueryOrPrincipal(r, p)
	if err := h.keys.DisableKey(r.Context(), tenantID, keyID, p.ID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) rotateKey(w http.ResponseWriter, r *http.Request) {
	p := middleware.PrincipalFromContext(r.Context())
	keyID := r.PathValue("key_id")
	tenantID := tenantFromQueryOrPrincipal(r, p)
	dto, err := h.keys.RotateKey(r.Context(), keys.RotateKeyCommand{
		TenantID: tenantID, KeyID: keyID, PrincipalID: p.ID,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, dto)
}

func (h *Handler) scheduleDestroy(w http.ResponseWriter, r *http.Request) {
	p := middleware.PrincipalFromContext(r.Context())
	keyID := r.PathValue("key_id")
	tenantID := tenantFromQueryOrPrincipal(r, p)
	if err := h.keys.ScheduleDestroy(r.Context(), tenantID, keyID, p.ID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(204)
}

type registerNodeReq struct {
	NodeID   string `json:"node_id"`
	Role     string `json:"role"`
	Baseline struct {
		SELinuxStatus  string `json:"selinux_status"`
		KernelVersion  string `json:"kernel_version"`
		VirtPlatform   string `json:"virt_platform"`
		TPM2TSSVersion string `json:"tpm2_tss_version"`
		SwtpmIsolated  bool   `json:"swtpm_isolated"`
	} `json:"baseline"`
}

func (h *Handler) registerNode(w http.ResponseWriter, r *http.Request) {
	p := middleware.PrincipalFromContext(r.Context())
	if !p.HasScope("nodes:manage") {
		writeErr(w, 403, "PERMISSION_DENIED", "missing scope nodes:manage")
		return
	}
	body := middleware.BodyFromContext(r.Context())
	var req registerNodeReq
	if err := middleware.DecodeJSONStrict(body, &req); err != nil {
		writeErr(w, 400, "BAD_REQUEST", "invalid json")
		return
	}
	dto, err := h.nodes.Register(r.Context(), nodes.RegisterCommand{
		NodeID: req.NodeID,
		Role:   req.Role,
		Baseline: models.NodeBaseline{
			SELinuxStatus:  req.Baseline.SELinuxStatus,
			KernelVersion:  req.Baseline.KernelVersion,
			VirtPlatform:   req.Baseline.VirtPlatform,
			TPM2TSSVersion: req.Baseline.TPM2TSSVersion,
			SwtpmIsolated:  req.Baseline.SwtpmIsolated,
		},
		PrincipalID: p.ID,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 201, dto)
}

func (h *Handler) markReady(w http.ResponseWriter, r *http.Request) {
	p := middleware.PrincipalFromContext(r.Context())
	nodeID := r.PathValue("node_id")
	dto, err := h.nodes.MarkReady(r.Context(), nodeID, p.ID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, dto)
}

func (h *Handler) revokeNode(w http.ResponseWriter, r *http.Request) {
	p := middleware.PrincipalFromContext(r.Context())
	nodeID := r.PathValue("node_id")
	if err := h.nodes.Revoke(r.Context(), nodeID, p.ID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) getNode(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("node_id")
	dto, err := h.nodes.Get(r.Context(), nodeID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, 200, dto)
}

func tenantFromQueryOrPrincipal(r *http.Request, p *principal.Principal) string {
	if p != nil && p.TenantID != "" {
		return p.TenantID
	}
	return r.URL.Query().Get("tenant_id")
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
	code := errorsx.AsCode(err)
	status := errorsx.HTTPStatus(code)
	msg := "request failed"
	if code == errorsx.CodeInternal {
		msg = "internal error"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	b, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"code":      string(code),
			"message":   msg,
			"retryable": errorsx.AsRetryable(err),
		},
	})
	w.Write(b)
}
