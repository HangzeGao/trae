// Package server wires the HTTP server with all middleware and routes.
package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/kvlt/key-vault/internal/api/admin"
	cryptoapi "github.com/kvlt/key-vault/internal/api/crypto"
	"github.com/kvlt/key-vault/internal/api/middleware"
	"github.com/kvlt/key-vault/internal/auth/hmacsign"
	"github.com/kvlt/key-vault/internal/auth/jwt"
	"github.com/kvlt/key-vault/internal/auth/principal"
	"github.com/kvlt/key-vault/internal/config"
	"github.com/kvlt/key-vault/internal/web"
)

// Server is the HTTP server.
type Server struct {
	http *http.Server
}

// Deps bundles the server dependencies.
type Deps struct {
	Cfg          *config.Config
	AdminHandler *admin.Handler
	CryptoHandler *cryptoapi.Handler
	JWTVerifier  *jwt.Verifier
	HMACVerifier *hmacsign.Verifier
	StaticTokens map[string]*principal.Principal
}

// New constructs a server with all middleware applied.
func New(deps Deps) *Server {
	mux := http.NewServeMux()

	// Register route handlers.
	deps.AdminHandler.Routes(mux)
	deps.CryptoHandler.Routes(mux)
	// Health check.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	})
	// Embedded frontend SPA.
	mux.Handle("GET /ui/", http.StripPrefix("/ui", web.Handler()))

	// Auth config.
	authCfg := middleware.AuthConfig{
		JWTVerifier:  deps.JWTVerifier,
		HMACVerifier: deps.HMACVerifier,
		HMACEnabled:  deps.Cfg.Auth.HMACEnabled,
		StaticTokens: deps.StaticTokens,
	}

	// Build the middleware chain. Order (outer -> inner):
	//   RequestID -> BodyLimit -> ReadBody -> Auth -> mux
	// ReadBody MUST run before Auth so the body is in context for HMAC verification.
	var handler http.Handler = mux
	authMiddleware := middleware.Auth(authCfg)
	// Wrap auth so healthz and /ui/ bypass authentication.
	authWrapper := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" || strings.HasPrefix(r.URL.Path, "/ui/") {
				h.ServeHTTP(w, r)
				return
			}
			authMiddleware(h).ServeHTTP(w, r)
		})
	}
	handler = authWrapper(handler)
	handler = middleware.ReadBody(handler)
	handler = middleware.BodyLimit(deps.Cfg.Server.MaxRequestBody)(handler)
	handler = middleware.RequestID(handler)

	srv := &http.Server{
		Addr:         deps.Cfg.Server.HTTPListenAddr,
		Handler:      handler,
		ReadTimeout:  deps.Cfg.Server.ReadTimeout,
		WriteTimeout: deps.Cfg.Server.WriteTimeout,
	}
	return &Server{http: srv}
}

// Start begins listening.
func (s *Server) Start() error {
	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

// HTTPServer returns the underlying *http.Server (for testing).
func (s *Server) HTTPServer() *http.Server { return s.http }

// unused import guard
var _ = time.Second
