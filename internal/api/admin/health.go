// Package admin 实现管理面 HTTP API。
package admin

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/HangzeGao/trae/key-vault/internal/observability"
)

// HealthHandler 健康检查处理器。
type HealthHandler struct {
	log *observability.Logger
}

func NewHealthHandler(log *observability.Logger) *HealthHandler {
	return &HealthHandler{log: log}
}

// HealthResponse 健康检查响应。
type HealthResponse struct {
	Status  string                 `json:"status"`
	Version string                 `json:"version"`
	Checks  map[string]CheckResult `json:"checks"`
}

type CheckResult struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Liveness 存活检查（K8s liveness probe）。
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HealthResponse{
		Status:  "ok",
		Version: "0.1.0",
	})
}

// Readiness 就绪检查（K8s readiness probe）。
func (h *HealthHandler) Readiness(checks []ReadinessCheck) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results := make(map[string]CheckResult)
		allOK := true
		for _, c := range checks {
			err := c.Check(r.Context())
			if err != nil {
				results[c.Name] = CheckResult{Status: "fail", Error: err.Error()}
				allOK = false
			} else {
				results[c.Name] = CheckResult{Status: "ok"}
			}
		}
		status := "ok"
		code := http.StatusOK
		if !allOK {
			status = "degraded"
			code = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(HealthResponse{
			Status:  status,
			Version: "0.1.0",
			Checks:  results,
		})
	}
}

// ReadinessCheck 是就绪检查项。
type ReadinessCheck struct {
	Name  string
	Check func(ctx context.Context) error
}

// MetricsHandler 暴露指标。
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	snap := observability.MetricsInstance().Snapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snap)
}
