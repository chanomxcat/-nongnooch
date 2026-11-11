// Package server runs the HTTP + gRPC endpoints with health checks.
package server

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"nongnooch/platform/pkg/chassis/config"
)

// Server bundles the HTTP mux and gRPC server of a service.
type Server struct {
	Mux  *http.ServeMux
	GRPC *grpc.Server

	cfg    config.Base
	db     *sql.DB
	health *health.Server
}

// New builds a server with /healthz, /readyz and the gRPC health service registered.
func New(cfg config.Base, db *sql.DB) *Server {
	s := &Server{
		Mux:    http.NewServeMux(),
		GRPC:   grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler())),
		cfg:    cfg,
		db:     db,
		health: health.NewServer(),
	}
	healthpb.RegisterHealthServer(s.GRPC, s.health)
	s.Mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	s.Mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.db.PingContext(ctx); err != nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	return s
}

// Run serves until ctx is cancelled or SIGTERM/SIGINT, then shuts down gracefully (15s).
func (s *Server) Run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	hl, err := net.Listen("tcp", s.cfg.HTTPAddr)
	if err != nil {
		return err
	}
	gl, err := net.Listen("tcp", s.cfg.GRPCAddr)
	if err != nil {
		hl.Close()
		return err
	}
	hs := &http.Server{Handler: otelhttp.NewHandler(s.Mux, s.cfg.ServiceName), ReadHeaderTimeout: 10 * time.Second}
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	errc := make(chan error, 2)
	go func() { errc <- hs.Serve(hl) }()
	go func() { errc <- s.GRPC.Serve(gl) }()
	slog.Info("server started", "http", s.cfg.HTTPAddr, "grpc", s.cfg.GRPCAddr)

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errc:
	}
	s.health.Shutdown()
	sctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = hs.Shutdown(sctx)
	done := make(chan struct{})
	go func() { s.GRPC.GracefulStop(); close(done) }()
	select {
	case <-done:
	case <-sctx.Done():
		s.GRPC.Stop()
	}
	if errors.Is(runErr, http.ErrServerClosed) {
		runErr = nil
	}
	return runErr
}
