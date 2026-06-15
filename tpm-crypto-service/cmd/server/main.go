// Command server 是 tpm-crypto-service 的统一入口:加载配置 → 初始化 TPM/Keystore/Crypto
// → 启动 gRPC 与 HTTP Gateway → 监听信号优雅关停。
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	"go.uber.org/zap"

	"github.com/tpm-crypto/tpm-crypto-service/internal/config"
	"github.com/tpm-crypto/tpm-crypto-service/internal/cpufeat"
	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto"
	"github.com/tpm-crypto/tpm-crypto-service/internal/keystore"
	"github.com/tpm-crypto/tpm-crypto-service/internal/obs"
	"github.com/tpm-crypto/tpm-crypto-service/internal/server"
	"github.com/tpm-crypto/tpm-crypto-service/internal/service"
	"github.com/tpm-crypto/tpm-crypto-service/internal/tpm"
)

func main() {
	fs := flag.NewFlagSet("tpm-crypto-service", flag.ExitOnError)
	config.BindFlags(fs)
	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "parse flags:", err)
		os.Exit(2)
	}

	cfg, err := config.Load(fs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(2)
	}

	// 1. CPU 特性自检 → 启动日志/指标
	feat := cpufeat.Detect()
	cpuFeatMap := feat.Map()
	fmt.Fprintf(os.Stdout, "cpu_features=%v\n", cpuFeatMap)

	// 2. 初始化指标
	metrics := obs.NewMetrics()
	for name, ok := range cpuFeatMap {
		metrics.SetCPUFeature(name, ok)
	}

	// 3. 初始化 logger
	log, err := obs.NewLogger(cfg.Log.Level, cfg.Log.Encoding, cfg.Log.Output,
		"tpm-crypto-service", hostnameOrPID(), tpmMode(cfg), cpuFeatMap)
	if err != nil {
		fmt.Fprintln(os.Stderr, "init logger:", err)
		os.Exit(2)
	}
	defer func() { _ = log.Sync() }()

	t0 := time.Now()
	log.Info("starting tpm-crypto-service",
		zap.String("grpc", cfg.Server.GRPCAddr),
		zap.String("http", cfg.Server.HTTPAddr),
		zap.String("deploy_mode", cfg.Deploy.Mode),
		zap.String("crypto_backend", cfg.Crypto.Backend))

	// 4. Keystore
	ks, err := keystore.Open(cfg.Keystore.Backend, cfg.Keystore.Path, cfg.Keystore.Endpoints)
	if err != nil {
		log.Fatal("open keystore", zap.Error(err))
	}
	defer func() { _ = ks.Close() }()

	// 5. TPM / SRK
	mode := "physical"
	if cfg.TPM.Simulator {
		mode = "simulator"
	}
	srk, err := tpm.EnsureSRK(mode, cfg.TPM.SealedARK)
	if err != nil {
		log.Fatal("ensure SRK", zap.Error(err))
	}
	ark, err := bootstrapARK(srk, cfg.TPM.SealedARK)
	if err != nil {
		log.Fatal("bootstrap ARK", zap.Error(err))
	}
	log.Info("ark unsealed", zap.Int("len", len(ark)), zap.String("mode", mode))

	// 6. Crypto factory
	factory, err := crypto.NewFactory(cfg.Crypto.Backend, feat)
	if err != nil {
		log.Fatal("init crypto factory", zap.Error(err))
	}
	metrics.SetBackend(cfg.Crypto.Backend)

	// 7. Service / Servers
	svc := &service.Service{
		KS:      ks,
		SRK:     srk,
		Factory: factory,
		ARK:     ark,
		Log:     log,
		Metrics: metrics,
		TPMMode: mode,
		Caller:  hostnameOrPID(),
	}
	gsrv, err := server.NewGRPC(cfg, log, svc)
	if err != nil {
		log.Fatal("new grpc", zap.Error(err))
	}
	rootCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	hsrv, err := server.NewHTTP(rootCtx, cfg, log, svc, metrics)
	if err != nil {
		log.Fatal("new http", zap.Error(err))
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := gsrv.Serve(rootCtx); err != nil {
			log.Error("grpc serve", zap.Error(err))
		}
	}()
	go func() {
		defer wg.Done()
		if err := hsrv.Serve(rootCtx); err != nil {
			log.Error("http serve", zap.Error(err))
		}
	}()

	metrics.ServiceStartDur.Observe(time.Since(t0).Seconds())
	metrics.Health.WithLabelValues("tpm").Set(1)
	metrics.Health.WithLabelValues("ark").Set(1)
	metrics.Health.WithLabelValues("ks").Set(1)

	<-rootCtx.Done()
	log.Info("shutdown initiated")
	wg.Wait()
	log.Info("bye")
}

// bootstrapARK 加载或创建 ARK:先尝试从 SealedARK 文件恢复,失败则生成新的 32B 随机并 Seal。
func bootstrapARK(srk *tpm.SRK, sealedPath string) ([]byte, error) {
	if sealedPath == "" {
		return nil, errors.New("tpm.sealed_ark_path required")
	}
	if s, err := tpm.LoadSealedARK(sealedPath); err == nil {
		ark, err := srk.UnsealARK(s)
		if err != nil {
			return nil, fmt.Errorf("unseal existing ARK: %w", err)
		}
		return ark, nil
	}
	ark := make([]byte, 32)
	if _, err := rand.Read(ark); err != nil {
		return nil, err
	}
	s, err := srk.SealARK(ark, []int{0, 1, 2, 3, 4, 5, 6, 7})
	if err != nil {
		return nil, fmt.Errorf("seal ARK: %w", err)
	}
	if err := tpm.SaveSealedARK(sealedPath, s); err != nil {
		return nil, fmt.Errorf("persist sealed ARK: %w", err)
	}
	return ark, nil
}

func tpmMode(cfg *config.Config) string {
	if cfg.TPM.Simulator {
		return "simulator"
	}
	return "physical"
}

func hostnameOrPID() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return fmt.Sprintf("pid-%d", os.Getpid())
	}
	return h
}
