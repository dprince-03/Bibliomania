// Package telemetry wires the three observability signals every service
// emits (Step 37, pulled forward by the microservices split):
//
//   - traces: OpenTelemetry, exported over OTLP/gRPC to Tempo when
//     OTEL_EXPORTER_OTLP_ENDPOINT is set. W3C trace context is always
//     propagated (HTTP headers, gRPC metadata, event envelopes), even when
//     export is off, so a trace stays connected across all eight processes.
//   - logs: log/slog as JSON on stdout, each line stamped with service,
//     trace_id and span_id — Grafana Alloy ships stdout to Loki, and Grafana
//     links a Loki line to its Tempo trace through trace_id.
//   - metrics: Prometheus, scraped from each service's GET /metrics.
package telemetry

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

// Init installs the global tracer provider, propagator and slog logger.
// The returned func flushes pending spans; call it on shutdown.
func Init(ctx context.Context, serviceName, otlpEndpoint string) (func(context.Context) error, error) {
	slog.SetDefault(slog.New(&traceHandler{
		Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
	}).With("service", serviceName))

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	// Schemaless on purpose: resource.Default() carries the SDK's own semconv
	// schema URL, and merging two resources with different schema URLs is an
	// error — so pinning ours broke every service's startup the moment an SDK
	// upgrade moved its schema (1.37 vs 1.41, found on the HA cluster).
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceName(serviceName),
	))
	if err != nil {
		return nil, err
	}

	opts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	if otlpEndpoint != "" {
		exporter, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(otlpEndpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, err
		}
		opts = append(opts, sdktrace.WithBatcher(exporter))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// Tracer returns a named tracer from the global provider.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// traceHandler adds trace_id/span_id to every record logged with a context
// that carries an active span (slog.InfoContext etc).
type traceHandler struct {
	slog.Handler
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithGroup(name)}
}
