// Command key-vault runs the P0 key-vault service.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kvlt/key-vault/internal/api/admin"
	cryptoapi "github.com/kvlt/key-vault/internal/api/crypto"
	"github.com/kvlt/key-vault/internal/api/server"
	"github.com/kvlt/key-vault/internal/bootstrap"
	"github.com/kvlt/key-vault/internal/config"
	"github.com/kvlt/key-vault/internal/logging"
)

func main() {
	cfgPath := flag.String("config", "", "path to YAML config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	}
	logger := logging.New(level)
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := bootstrap.Build(ctx, cfg)
	if err != nil {
		slog.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}

	adminH := admin.New(app.KeyService, app.NodeService)
	cryptoH := cryptoapi.New(app.CryptoService)

	srv := server.New(server.Deps{
		Cfg:           cfg,
		AdminHandler:  adminH,
		CryptoHandler: cryptoH,
		JWTVerifier:   app.JWTVerifier,
		HMACVerifier:  app.HMACVerifier,
		StaticTokens:  app.StaticTokens,
	})

	// Signal handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("shutting down")
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		_ = srv.Shutdown(shutCtx)
		cancel()
	}()

	slog.Info("key-vault starting", "addr", cfg.Server.HTTPListenAddr)
	if err := srv.Start(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
