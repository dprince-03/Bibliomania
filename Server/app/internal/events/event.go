// Package events is the async half of inter-service communication: the
// event envelope every broker carries, the shared event-type contract,
// a transactional outbox for SQL-backed services, and publisher/consumer
// adapters for the three brokers (RabbitMQ, Kafka, NATS JetStream).
//
// Which broker carries what is fixed per producing service (see
// Server/app/docs/plan.md → "Microservices split"): auth/user/catalog →
// RabbitMQ, borrow/payment → Kafka, reading → NATS. One business flow never
// crosses brokers; a service may consume from several.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Event is the envelope on the wire, identical on every broker.
//
// Versioning: SchemaVersion is the version of Data's shape for this Type
// (see CurrentVersions). Additive, backwards-compatible changes (a new
// optional field) keep the version. A breaking change bumps it; the
// producer then publishes both versions until every consumer handles the
// new one, and consumers reject versions they don't know (see Decode) —
// sending the message to the dead-letter path instead of misreading it.
type Event struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	SchemaVersion int    `json:"schema_version"`
	Source        string `json:"source"`
	// Key orders related events (Kafka partition key); for RabbitMQ/NATS
	// it's informational. Usually the aggregate's ID.
	Key        string    `json:"key,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
	// W3C trace context, so a consumer's span joins the producer's trace
	// even across the outbox's asynchronous relay.
	Trace map[string]string `json:"trace,omitempty"`
	Data  json.RawMessage   `json:"data"`
}

// ErrUnsupportedVersion marks an event this consumer can't safely read.
var ErrUnsupportedVersion = errors.New("unsupported event schema version")

// New builds an event of the given type at its current schema version,
// capturing the trace context from ctx.
func New(ctx context.Context, eventType, source, key string, data any) (Event, error) {
	version, ok := CurrentVersions[eventType]
	if !ok {
		return Event{}, fmt.Errorf("unknown event type %q — add it to CurrentVersions", eventType)
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return Event{}, fmt.Errorf("marshal %s payload: %w", eventType, err)
	}

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	return Event{
		ID:            uuid.NewString(),
		Type:          eventType,
		SchemaVersion: version,
		Source:        source,
		Key:           key,
		OccurredAt:    time.Now().UTC(),
		Trace:         carrier,
		Data:          raw,
	}, nil
}

// Decode unmarshals e.Data into dst after checking the schema version is
// one this consumer supports.
func (e Event) Decode(dst any, supportedVersions ...int) error {
	if len(supportedVersions) == 0 {
		supportedVersions = []int{CurrentVersions[e.Type]}
	}
	if !slices.Contains(supportedVersions, e.SchemaVersion) {
		return fmt.Errorf("%w: %s v%d (supported: %v)", ErrUnsupportedVersion, e.Type, e.SchemaVersion, supportedVersions)
	}
	return json.Unmarshal(e.Data, dst)
}

// Context returns ctx carrying the producer's trace context from e.
func (e Event) Context(ctx context.Context) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(e.Trace))
}

func parse(body []byte) (Event, error) {
	var e Event
	if err := json.Unmarshal(body, &e); err != nil {
		return Event{}, fmt.Errorf("malformed event envelope: %w", err)
	}
	return e, nil
}

// Publisher hands an event to a broker. Implementations must only return
// nil once the broker has durably accepted it (publisher confirms, Kafka
// acks, JetStream PubAck) — the outbox relay relies on that to mark rows
// published.
type Publisher interface {
	Publish(ctx context.Context, e Event) error
}

// Handler processes one consumed event. Returning an error triggers the
// consumer's retry → dead-letter path; handlers must be idempotent, since
// every broker here delivers at-least-once.
type Handler func(ctx context.Context, e Event) error

func jsonBody(e Event) ([]byte, error) {
	return json.Marshal(e)
}
