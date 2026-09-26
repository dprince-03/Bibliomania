package events

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/dprince-03/Bibliomania/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const handlerAttempts = 3

// process runs h for one consumed event with tracing, metrics and
// in-process retries (1s, 2s backoff). It returns the final error, which
// each adapter turns into a dead-letter. Unsupported schema versions fail
// immediately — retrying can't fix them.
func process(ctx context.Context, topic string, e Event, h Handler) error {
	ctx, span := telemetry.Tracer("events").Start(e.Context(ctx), "consume "+e.Type,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.message.id", e.ID),
			attribute.Int("event.schema_version", e.SchemaVersion),
		),
	)
	defer span.End()

	var err error
	for attempt := 1; attempt <= handlerAttempts; attempt++ {
		if err = h(ctx, e); err == nil {
			telemetry.EventsHandled.WithLabelValues(topic, "ok").Inc()
			return nil
		}
		if errors.Is(err, ErrUnsupportedVersion) || attempt == handlerAttempts {
			break
		}
		telemetry.EventsHandled.WithLabelValues(topic, "retry").Inc()
		slog.WarnContext(ctx, "event handler failed, retrying", "type", e.Type, "id", e.ID, "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}

	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	telemetry.EventsHandled.WithLabelValues(topic, "dead_letter").Inc()
	slog.ErrorContext(ctx, "event handler gave up, dead-lettering", "type", e.Type, "id", e.ID, "error", err)
	return err
}

// Route dispatches by event type, ignoring types with no handler (a
// consumer bound to a broad topic only cares about some events on it).
func Route(handlers map[string]Handler) Handler {
	return func(ctx context.Context, e Event) error {
		if h, ok := handlers[e.Type]; ok {
			return h(ctx, e)
		}
		return nil
	}
}
