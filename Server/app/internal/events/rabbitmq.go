package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Rabbit is a RabbitMQ connection that re-dials on demand. All events go
// through one durable topic exchange (RabbitExchange), routed by event type;
// each consuming service owns one durable queue bound to the types it
// wants, plus a "<queue>.dlq" for messages its handler gave up on.
type Rabbit struct {
	url       string
	queueType string // "classic" or "quorum"
	mu        sync.Mutex
	conn      *amqp.Connection
	pub       *amqp.Channel
}

const rabbitDLX = RabbitExchange + ".dlx"

// DialRabbit connects (retrying while the broker starts) and declares the
// shared exchanges.
// queueType is "classic" or "quorum". Quorum queues are replicated across
// RabbitMQ cluster nodes (Raft) — the HA choice — and also enforce a
// delivery limit. The type is fixed when a queue is first declared:
// switching an existing broker means deleting (draining) the queues first.
func DialRabbit(ctx context.Context, url, queueType string) (*Rabbit, error) {
	if queueType == "" {
		queueType = "classic"
	}
	r := &Rabbit{url: url, queueType: queueType}
	var err error
	for attempt := 1; attempt <= 20; attempt++ {
		if _, err = r.connection(); err == nil {
			return r, nil
		}
		slog.Warn("rabbitmq not ready, retrying", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, fmt.Errorf("connecting to rabbitmq: %w", err)
}

func (r *Rabbit) connection() (*amqp.Connection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn != nil && !r.conn.IsClosed() {
		return r.conn, nil
	}
	conn, err := amqp.Dial(r.url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	defer ch.Close()
	if err := ch.ExchangeDeclare(RabbitExchange, "topic", true, false, false, false, nil); err != nil {
		conn.Close()
		return nil, err
	}
	if err := ch.ExchangeDeclare(rabbitDLX, "direct", true, false, false, false, nil); err != nil {
		conn.Close()
		return nil, err
	}
	r.conn, r.pub = conn, nil
	return conn, nil
}

func (r *Rabbit) publishChannel() (*amqp.Channel, error) {
	conn, err := r.connection()
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pub != nil && !r.pub.IsClosed() {
		return r.pub, nil
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	if err := ch.Confirm(false); err != nil {
		ch.Close()
		return nil, err
	}
	r.pub = ch
	return ch, nil
}

// Publish sends e to the exchange with routing key e.Type and waits for the
// broker's publisher confirm.
func (r *Rabbit) Publish(ctx context.Context, e Event) error {
	ch, err := r.publishChannel()
	if err != nil {
		return err
	}
	body, err := jsonBody(e)
	if err != nil {
		return err
	}
	confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx, RabbitExchange, e.Type, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    e.ID,
		Timestamp:    e.OccurredAt,
		Type:         e.Type,
		Body:         body,
	})
	if err != nil {
		return err
	}
	acked, err := confirm.WaitContext(ctx)
	if err != nil {
		return err
	}
	if !acked {
		return errors.New("rabbitmq nacked the publish")
	}
	return nil
}

// Consume declares queue (bound to routingKeys) and feeds its messages to h
// until ctx ends, reconnecting after broker restarts. Failed messages are
// dead-lettered to "<queue>.dlq" rather than requeued, so a poison message
// can't block the queue.
func (r *Rabbit) Consume(ctx context.Context, queue string, routingKeys []string, h Handler) {
	for ctx.Err() == nil {
		if err := r.consumeOnce(ctx, queue, routingKeys, h); err != nil && ctx.Err() == nil {
			slog.Warn("rabbitmq consumer stopped, reconnecting", "queue", queue, "error", err)
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
			}
		}
	}
}

func (r *Rabbit) consumeOnce(ctx context.Context, queue string, routingKeys []string, h Handler) error {
	conn, err := r.connection()
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	dlq := queue + ".dlq"
	typeArg := amqp.Table{"x-queue-type": r.queueType}
	if _, err := ch.QueueDeclare(dlq, true, false, false, false, typeArg); err != nil {
		return err
	}
	if err := ch.QueueBind(dlq, queue, rabbitDLX, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(queue, true, false, false, false, amqp.Table{
		"x-queue-type":              r.queueType,
		"x-dead-letter-exchange":    rabbitDLX,
		"x-dead-letter-routing-key": queue,
	}); err != nil {
		return err
	}
	for _, key := range routingKeys {
		if err := ch.QueueBind(queue, key, RabbitExchange, false, nil); err != nil {
			return err
		}
	}
	if err := ch.Qos(10, 0, false); err != nil {
		return err
	}

	deliveries, err := ch.ConsumeWithContext(ctx, queue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	slog.Info("rabbitmq consumer started", "queue", queue, "routing_keys", routingKeys)

	for d := range deliveries {
		e, err := parse(d.Body)
		if err == nil {
			err = process(ctx, queue, e, h)
		}
		if err != nil {
			_ = d.Nack(false, false) // → DLX → <queue>.dlq
			continue
		}
		_ = d.Ack(false)
	}
	return errors.New("delivery channel closed")
}

// Close closes the underlying connection.
func (r *Rabbit) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

// Ping reports whether the connection is up — for /health.
func (r *Rabbit) Ping(context.Context) error {
	_, err := r.connection()
	return err
}
