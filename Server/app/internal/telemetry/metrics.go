package telemetry

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// RED metrics (rate, errors, duration) for both transports. The `service`
// label isn't set here — Prometheus adds it from the scrape target
// (infra/observability/prometheus/prometheus.yml), so every service shares
// the same metric names and one Grafana dashboard covers all of them.
var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "HTTP requests by method, route pattern and status code.",
	}, []string{"method", "route", "status"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency by method and route pattern.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})

	grpcRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_server_handled_total",
		Help: "gRPC calls handled, by full method and status code.",
	}, []string{"method", "code"})

	grpcDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_server_handling_seconds",
		Help:    "gRPC handler latency by full method.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"})

	// EventsHandled counts broker messages consumed, by topic and outcome
	// ("ok", "retry", "dead_letter"). Exported for internal/events.
	EventsHandled = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "events_handled_total",
		Help: "Broker messages consumed, by topic/routing key and outcome.",
	}, []string{"topic", "outcome"})

	// EventsPublished counts events handed to a broker, by topic and
	// outcome ("ok" or "error").
	EventsPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "events_published_total",
		Help: "Events published to a broker, by topic/routing key and outcome.",
	}, []string{"topic", "outcome"})
)

// MetricsHandler serves GET /metrics.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Flush keeps streaming responses (file downloads, the gateway's reverse
// proxy) working through this wrapper.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}

// HTTPMetrics must be the innermost middleware, directly around the
// ServeMux: the mux writes the matched pattern onto the *http.Request it
// receives, and any middleware between here and the mux that calls
// r.WithContext would hand the mux a copy, hiding the pattern from us.
// Keying on the pattern (not the raw path) keeps label cardinality bounded.
func HTTPMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		httpRequests.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
		httpDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

// GRPCMetrics is a unary server interceptor recording the same RED metrics
// for gRPC.
func GRPCMetrics(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	grpcRequests.WithLabelValues(info.FullMethod, status.Code(err).String()).Inc()
	grpcDuration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
	return resp, err
}
