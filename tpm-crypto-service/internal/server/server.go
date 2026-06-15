// Package server 暴露 gRPC 与 HTTP(Gateway)双协议端点。
package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/tpm-crypto/tpm-crypto-service/api/gen/go"
	tpmcrypto "github.com/tpm-crypto/tpm-crypto-service/api/gen/go"
	"github.com/tpm-crypto/tpm-crypto-service/internal/config"
	"github.com/tpm-crypto/tpm-crypto-service/internal/obs"
	"github.com/tpm-crypto/tpm-crypto-service/internal/service"
)

// GRPCServer 包装 grpc.Server 并提供 graceful shutdown。
type GRPCServer struct {
	srv    *grpc.Server
	lis    net.Listener
	addr   string
	log    *zap.Logger
	closed atomic.Bool
}

// NewGRPC 构造并启动 gRPC 服务。
func NewGRPC(cfg *config.Config, log *zap.Logger, impl *service.Service) (*GRPCServer, error) {
	lis, err := net.Listen("tcp", cfg.Server.GRPCAddr)
	if err != nil {
		return nil, err
	}
	creds := grpc.Creds(insecure.NewCredentials())
	if cfg.Server.TLS.Enabled {
		creds = grpc.Creds(credentials.NewServerTLSFromCert(&dummyCert{}))
	}
	srv := grpc.NewServer(
		creds,
		grpc.UnaryInterceptor(unaryInterceptor(log)),
	)
	tpmcrypto.RegisterCryptoServiceServer(srv, &grpcImpl{svc: impl})
	return &GRPCServer{srv: srv, lis: lis, addr: lis.Addr().String(), log: log}, nil
}

// Addr 返回实际监听地址(:0 调试时有用)。
func (g *GRPCServer) Addr() string { return g.addr }

// Serve 阻塞直到 ctx 取消或 listener 出错。
func (g *GRPCServer) Serve(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() { errCh <- g.srv.Serve(g.lis) }()
	select {
	case <-ctx.Done():
		g.log.Info("grpc: shutdown signal received")
		g.srv.GracefulStop()
		return nil
	case err := <-errCh:
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return err
	}
}

// HTTPGateway 包装 http.Server 并提供 graceful shutdown。
type HTTPGateway struct {
	srv    *http.Server
	addr   string
	log    *zap.Logger
	closed atomic.Bool
}

// NewHTTP 构造并启动 HTTP Gateway(:8080,含 /healthz、/metrics)。
func NewHTTP(ctx context.Context, cfg *config.Config, log *zap.Logger, impl *service.Service, m *obs.Metrics) (*HTTPGateway, error) {
	rmux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.HTTPProtoMarshaler{
			MarshalOptions:   protojson.MarshalOptions{UseProtoNames: true},
			UnmarshalOptions: protojson.UnmarshalOptions{DiscardUnknown: true},
		}),
	)
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if err := tpmcrypto.RegisterCryptoServiceHandlerFromEndpoint(ctx, rmux, "localhost"+cfg.Server.GRPCAddr, opts); err != nil {
		return nil, err
	}
	root := http.NewServeMux()
	root.HandleFunc("/healthz", healthzHandler(impl))
	root.Handle("/metrics", obs.MetricsHandler(m.Registry()))
	root.Handle("/", rmux)

	srv := &http.Server{
		Addr:              cfg.Server.HTTPAddr,
		Handler:           withLog(h2c.NewHandler(root, &http2.Server{}), log),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return &HTTPGateway{srv: srv, addr: cfg.Server.HTTPAddr, log: log}, nil
}

// Addr 返回实际监听地址。
func (h *HTTPGateway) Addr() string { return h.addr }

// Serve 阻塞直到 ctx 取消或 listener 出错。
func (h *HTTPGateway) Serve(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() { errCh <- h.srv.ListenAndServe() }()
	select {
	case <-ctx.Done():
		h.log.Info("http: shutdown signal received")
		sctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return h.srv.Shutdown(sctx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func withLog(next http.Handler, log *zap.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug("http", zap.String("method", r.Method), zap.String("path", r.URL.Path))
		next.ServeHTTP(w, r)
	})
}

func healthzHandler(impl *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		h := impl.HealthCheck(nil)
		ok := h.TPMReady && h.ARKReady && h.KSReady
		w.Header().Set("content-type", "application/json")
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"NOT_SERVING"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"SERVING"}`))
	}
}

func unaryInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		t0 := time.Now()
		resp, err := handler(ctx, req)
		dur := time.Since(t0)
		if err != nil {
			st, _ := status.FromError(err)
			log.Warn("grpc.err",
				zap.String("method", info.FullMethod),
				zap.String("code", st.Code().String()),
				zap.String("msg", st.Message()),
				zap.Duration("dur", dur))
			return resp, err
		}
		log.Debug("grpc.ok", zap.String("method", info.FullMethod), zap.Duration("dur", dur))
		return resp, nil
	}
}

// dummyCert 仅为占位,真实部署应配置 cfg.Server.TLS.{cert,key,ca}。
type dummyCert struct{}

func (d *dummyCert) Certificate() [][]byte { return nil }

// 为 gRPC service 避免 unused import 警告:健康检查 enum
var _ = codes.OK
var _ = tpmcrypto.HealthCheckResponse_SERVING
