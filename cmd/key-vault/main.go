// key-vault 是基于 TPM2 的密钥管理与数据加解密系统。
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/HangzeGao/trae/key-vault/internal/api/admin"
	cryptohandler "github.com/HangzeGao/trae/key-vault/internal/api/crypto"
	"github.com/HangzeGao/trae/key-vault/internal/api/middleware"
	"github.com/HangzeGao/trae/key-vault/internal/application/crypto"
	"github.com/HangzeGao/trae/key-vault/internal/application/keys"
	"github.com/HangzeGao/trae/key-vault/internal/audit"
	"github.com/HangzeGao/trae/key-vault/internal/audit/wal"
	"github.com/HangzeGao/trae/key-vault/internal/auth/hmacsign"
	"github.com/HangzeGao/trae/key-vault/internal/auth/jwt"
	"github.com/HangzeGao/trae/key-vault/internal/config"
	"github.com/HangzeGao/trae/key-vault/internal/observability"
	"github.com/HangzeGao/trae/key-vault/internal/repository/postgres"
	"github.com/HangzeGao/trae/key-vault/internal/resolver/keyresolver"
	"github.com/HangzeGao/trae/key-vault/internal/tpm/provider"
)

// staticSecretGetter 是 P0 简单的 HMAC secret 提供器，从环境变量读取。
type staticSecretGetter struct {
	secret []byte
}

func (g *staticSecretGetter) GetSecret(_ context.Context, _ string) ([]byte, error) {
	return g.secret, nil
}

func main() {
	cfg := config.FromEnv(config.Default())

	observability.Init(cfg.Logging.Level, cfg.Logging.Format)
	log := observability.Get()

	log.Info("starting key-vault service",
		"http_addr", cfg.Server.HTTPAddr,
		"use_swtpm", cfg.TPM.UseSWTPM,
	)

	// 1. 数据库连接与迁移
	db, err := postgres.Connect(cfg.Database)
	if err != nil {
		log.Error("failed to connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	migrator := postgres.New(db)
	if err := migrator.Run(context.Background()); err != nil {
		log.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	log.Info("database migrations completed")

	// 2. TPM Provider 初始化
	tpmProvider, err := provider.New(cfg.TPM, log)
	if err != nil {
		log.Error("failed to init TPM provider", "error", err)
		os.Exit(1)
	}
	if err := tpmProvider.Init(context.Background()); err != nil {
		log.Error("failed to init TPM", "error", err)
		os.Exit(1)
	}
	defer tpmProvider.Close()
	log.Info("TPM provider initialized")

	// 3. 审计 WAL 与 Emitter
	var auditWAL *wal.WAL
	if cfg.Audit.WALDir != "" {
		auditWAL, err = wal.New(cfg.Audit)
		if err != nil {
			log.Error("failed to init audit WAL", "error", err)
			os.Exit(1)
		}
		defer auditWAL.Close()
	}
	auditEmitter := audit.NewEmitter(db, auditWAL, log)
	log.Info("audit emitter initialized")

	// 4. 仓储初始化
	keyRepo := postgres.NewKeyRepository()
	policyRepo := postgres.NewPolicyRepository()

	// 5. Key Resolver 初始化
	resolver := keyresolver.NewResolver(db, keyRepo, tpmProvider, cfg.Crypto, log)
	defer resolver.Close()

	// 启动预取 worker
	prefetchCtx, prefetchCancel := context.WithCancel(context.Background())
	defer prefetchCancel()
	go resolver.StartPrefetchWorker(prefetchCtx)
	log.Info("key resolver initialized")

	// 6. 应用服务初始化
	keyService := keys.NewService(db, keyRepo, policyRepo, tpmProvider, auditEmitter, log)
	cryptoService := crypto.NewService(db, keyRepo, policyRepo, resolver, auditEmitter, log)

	// 7. 认证中间件
	var jwtVerifier *jwt.Verifier
	if cfg.Auth.JWKURL != "" {
		jwtVerifier, err = jwt.New(cfg.Auth)
		if err != nil {
			log.Error("failed to init JWT verifier", "error", err)
			os.Exit(1)
		}
	}

	var hmacVerifier *hmacsign.Verifier
	if hmacSecret := os.Getenv("KV_HMAC_SECRET"); hmacSecret != "" {
		hmacVerifier = hmacsign.New(cfg.Auth, &staticSecretGetter{secret: []byte(hmacSecret)})
	}

	authMiddleware := middleware.NewAuthMiddleware(jwtVerifier, hmacVerifier)

	// 8. HTTP 路由注册
	mux := http.NewServeMux()

	// 健康检查与指标（不需要认证）
	healthHandler := admin.NewHealthHandler(log)
	mux.HandleFunc("/healthz", healthHandler.Liveness)

	readinessChecks := []admin.ReadinessCheck{
		{Name: "database", Check: func(ctx context.Context) error { return db.PingContext(ctx) }},
		{Name: "tpm", Check: func(ctx context.Context) error { return resolver.HealthCheck(ctx) }},
	}
	mux.HandleFunc("/readyz", healthHandler.Readiness(readinessChecks))
	mux.HandleFunc("/metrics", admin.MetricsHandler)

	// 业务 API（需要认证）
	keyHandler := keys.NewHandler(keyService, authMiddleware, log)
	keyHandler.RegisterRoutes(mux)

	cryptoHandler := cryptohandler.NewHandler(cryptoService, authMiddleware, log)
	cryptoHandler.RegisterRoutes(mux)

	// 9. 中间件链
	handler := middleware.RequestID(mux)
	handler = middleware.Logging(log, handler)
	handler = middleware.Recover(log, handler)

	srv := &http.Server{
		Addr:         cfg.Server.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// 10. 启动服务
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	log.Info("server listening", "addr", cfg.Server.HTTPAddr)

	// 11. 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", "error", err)
	}

	log.Info("server exited")
}
