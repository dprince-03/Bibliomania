package events

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NATS carries reading's events: high-frequency, low-stakes-per-message
// progress updates, which is NATS's design point. JetStream (not core NATS)
// so user-service's counters still converge if it was down when a
// "book completed" event fired.
type NATS struct {
	conn *nats.Conn
	js   jetstream.JetStream
}

// DialNATS connects (retrying while the server starts) and makes sure the
// READING stream exists.
func DialNATS(ctx context.Context, url string, replicas int) (*NATS, error) {
	var conn *nats.Conn
	var err error
	for attempt := 1; attempt <= 20; attempt++ {
		conn, err = nats.Connect(url, nats.MaxReconnects(-1), nats.ReconnectWait(2*time.Second))
		if err == nil {
			break
		}
		slog.Warn("nats not ready, retrying", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	if err != nil {
		return nil, fmt.Errorf("connecting to nats: %w", err)
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if _, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     StreamReading,
		Subjects: []string{"reading.>"},
		Storage:  jetstream.FileStorage,
		MaxAge:   7 * 24 * time.Hour,
		// JetStream dedupes on the Nats-Msg-Id header within this window,
		// so a retried publish of the same event ID isn't stored twice.
		Duplicates: 2 * time.Minute,
		// 3 on the HA cluster: every message is stored on three servers.
		Replicas: max(replicas, 1),
	}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("creating %s stream: %w", StreamReading, err)
	}
	return &NATS{conn: conn, js: js}, nil
}

// Publish stores e on the stream under subject e.Type and waits for the
// PubAck.
func (n *NATS) Publish(ctx context.Context, e Event) error {
	body, err := jsonBody(e)
	if err != nil {
		return err
	}
	_, err = n.js.Publish(ctx, e.Type, body, jetstream.WithMsgID(e.ID))
	return err
}

// Consume attaches durable consumer `durable` (filtered to subjects) and
// feeds messages to h until ctx ends. A message whose handler gives up is
// terminated (JetStream has no DLQ; the failure is logged and counted in
// events_handled_total{outcome="dead_letter"}).
func (n *NATS) Consume(ctx context.Context, durable string, subjects []string, h Handler) error {
	cons, err := n.js.CreateOrUpdateConsumer(ctx, StreamReading, jetstream.ConsumerConfig{
		Durable:        durable,
		FilterSubjects: subjects,
		AckPolicy:      jetstream.AckExplicitPolicy,
		AckWait:        30 * time.Second,
		MaxDeliver:     5,
	})
	if err != nil {
		return fmt.Errorf("creating consumer %s: %w", durable, err)
	}

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		e, err := parse(msg.Data())
		if err == nil {
			err = process(ctx, msg.Subject(), e, h)
		}
		if err != nil {
			_ = msg.Term()
			return
		}
		_ = msg.Ack()
	})
	if err != nil {
		return err
	}
	slog.Info("nats consumer started", "durable", durable, "subjects", subjects)
	<-ctx.Done()
	cc.Stop()
	return nil
}

// Close drains and closes the connection.
func (n *NATS) Close() {
	_ = n.conn.Drain()
}

// Ping reports whether the connection is up — for /health.
func (n *NATS) Ping(context.Context) error {
	if !n.conn.IsConnected() {
		return fmt.Errorf("nats: %s", n.conn.Status())
	}
	return nil
}
