// Package server runs one service process: its HTTP server (REST,
// /health, /metrics), its optional gRPC server, and its background workers
// (outbox relay, broker consumers, sweepers) — all stopped together on
// SIGINT/SIGTERM with a graceful drain.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dprince-03/Bibliomania/internal/config"
	"github.com/dprince-03/Bibliomania/internal/middleware"
	"github.com/dprince-03/Bibliomania/internal/telemetry"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"
)

// Worker is a long-running background task; it must return when ctx ends.
type Worker func(ctx context.Context)

type Service struct {
	Config  *config.Config
	Handler http.Handler // the service's mux (routes only — Run adds the standard middleware)
	GRPC    *grpc.Server // nil for HTTP-only services
	Workers []Worker
}

// Middleware is the standard chain every service's HTTP handler gets,
// outermost first. Services sit behind the gateway, so CORS, security
// headers and rate limiting live there, not here.
func Middleware(serviceName string, mux http.Handler) http.Handler {
	return middleware.Chain(
		mux,
		func(h http.Handler) http.Handler {
			return otelhttp.NewHandler(h, serviceName, otelhttp.WithSpanNameFormatter(
				func(_ string, r *http.Request) string { return r.Method + " " + r.URL.Path },
			))
		},
		middleware.RequestID,
		middleware.Logger,
		middleware.Recovery,
		telemetry.HTTPMetrics, // innermost — see its doc comment
	)
}

// Run serves until a shutdown signal, then drains everything.
func Run(ctx context.Context, svc Service) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := svc.Config
	httpSrv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      Middleware(cfg.ServiceName, svc.Handler),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 2)
	go func() {
		slog.Info("HTTP listening", "port", cfg.ServerPort)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http: %w", err)
		}
	}()

	if svc.GRPC != nil {
		lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
		if err != nil {
			return fmt.Errorf("grpc listen: %w", err)
		}
		go func() {
			slog.Info("gRPC listening", "port", cfg.GRPCPort)
			if err := svc.GRPC.Serve(lis); err != nil {
				errCh <- fmt.Errorf("grpc: %w", err)
			}
		}()
	}

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	var wg sync.WaitGroup
	for _, w := range svc.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w(workerCtx)
		}()
	}

	var runErr error
	select {
	case <-ctx.Done():
		slog.Info("shutting down gracefully")
	case runErr = <-errCh:
		slog.Error("server failed, shutting down", "error", runErr)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced HTTP shutdown", "error", err)
	}
	if svc.GRPC != nil {
		svc.GRPC.GracefulStop()
	}
	cancelWorkers()
	wg.Wait()

	slog.Info("stopped cleanly")
	return runErr
}

// Must exits the process with a structured log line if err is non-nil —
// startup failures only (config, database, broker connections).
func Must(err error, msg string) {
	if err != nil {
		slog.Error(msg, "error", err)
		os.Exit(1)
	}
}

// Ops registers the endpoints every service exposes besides its API:
//
//   - GET /health — dependency health (database, brokers; the gateway's
//     covers every service). Kubernetes readiness for the services.
//   - GET /livez — "this process is up", no dependency checks. Kubernetes
//     liveness everywhere, and the gateway's readiness: taking the gateway
//     out of rotation because one service behind it is down would turn a
//     partial outage into a total one.
//   - GET /metrics — Prometheus.
func Ops(mux *http.ServeMux, healthHandler http.HandlerFunc) {
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("GET /metrics", telemetry.MetricsHandler())
}
