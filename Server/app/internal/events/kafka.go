package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Kafka carries borrow's and payment's events: each producing service owns
// one topic (TopicBorrow, TopicPayment), keyed by aggregate ID so events
// about the same borrow/purchase stay ordered within a partition. Kafka's
// durable, replayable log is the reason these two domains use it.
type Kafka struct {
	client *kgo.Client
	topic  string // produce target; empty for consume-only clients
}

// TopicPartitions caps how many consumers in one group can read a topic in
// parallel (one partition → one consumer). 6 leaves room to scale
// notification-service (and future consumers) past one replica.
const TopicPartitions = 6

// EnsureTopics creates topics (plus their "<topic>.dlq") if missing, and
// grows existing ones to TopicPartitions (partitions can only be added).
// Replication factor comes from KAFKA_REPLICATION_FACTOR via
// SetReplicationFactor (1 for single-broker dev, 3 for the HA cluster).
func EnsureTopics(ctx context.Context, brokers []string, topics ...string) error {
	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		return err
	}
	defer cl.Close()
	adm := kadm.NewClient(cl)

	var all []string
	for _, t := range topics {
		all = append(all, t, t+".dlq")
	}

	var lastErr error
	for attempt := 1; attempt <= 20; attempt++ {
		resp, err := adm.CreateTopics(ctx, TopicPartitions, replicationFactor, nil, all...)
		if err == nil {
			lastErr = nil
			for _, r := range resp {
				if r.Err != nil && !errors.Is(r.Err, kerr.TopicAlreadyExists) {
					lastErr = r.Err
				}
			}
			if lastErr == nil {
				// Older topics were created with 3 partitions — grow them.
				// "Already has N" responses are fine and ignored.
				if _, err := adm.UpdatePartitions(ctx, TopicPartitions, all...); err != nil {
					slog.Warn("could not grow kafka topic partitions", "error", err)
				}
				return nil
			}
		} else {
			lastErr = err
		}
		slog.Warn("kafka not ready, retrying", "attempt", attempt, "error", lastErr)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("creating kafka topics: %w", lastErr)
}

var replicationFactor int16 = 1

// SetReplicationFactor sets the replication factor for topics EnsureTopics
// creates (call before it; values outside 1..32767 are ignored).
func SetReplicationFactor(rf int) {
	if rf >= 1 && rf <= math.MaxInt16 {
		replicationFactor = int16(rf)
	}
}

// NewKafkaPublisher returns a publisher that produces to topic with
// acks=all and idempotent writes (franz-go's default).
func NewKafkaPublisher(brokers []string, topic string) (*Kafka, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
	)
	if err != nil {
		return nil, err
	}
	return &Kafka{client: cl, topic: topic}, nil
}

// Publish produces e synchronously (returns once the broker acks).
func (k *Kafka) Publish(ctx context.Context, e Event) error {
	body, err := jsonBody(e)
	if err != nil {
		return err
	}
	return k.client.ProduceSync(ctx, &kgo.Record{
		Topic: k.topic,
		Key:   []byte(e.Key),
		Value: body,
		Headers: []kgo.RecordHeader{
			{Key: "event-type", Value: []byte(e.Type)},
			{Key: "schema-version", Value: fmt.Appendf(nil, "%d", e.SchemaVersion)},
		},
	}).FirstErr()
}

// Close flushes and closes the client.
func (k *Kafka) Close() {
	k.client.Close()
}

// Ping checks broker reachability — for /health.
func (k *Kafka) Ping(ctx context.Context) error {
	return k.client.Ping(ctx)
}

// ConsumeKafka joins consumer group `group` on topics and feeds records to
// h until ctx ends. Offsets are committed only after a record is handled
// (or dead-lettered to "<topic>.dlq"), giving at-least-once delivery.
func ConsumeKafka(ctx context.Context, brokers []string, group string, topics []string, h Handler) error {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topics...),
		kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return err
	}
	defer cl.Close()
	slog.Info("kafka consumer started", "group", group, "topics", topics)

	for {
		fetches := cl.PollFetches(ctx)
		if ctx.Err() != nil {
			return nil
		}
		fetches.EachError(func(topic string, partition int32, err error) {
			slog.Warn("kafka fetch error", "topic", topic, "partition", partition, "error", err)
		})

		var done []*kgo.Record
		fetches.EachRecord(func(rec *kgo.Record) {
			e, err := parse(rec.Value)
			if err == nil {
				err = process(ctx, rec.Topic, e, h)
			}
			if err != nil {
				dlq := &kgo.Record{Topic: rec.Topic + ".dlq", Key: rec.Key, Value: rec.Value, Headers: rec.Headers}
				if perr := cl.ProduceSync(ctx, dlq).FirstErr(); perr != nil {
					// Can't park it — don't commit, so it's redelivered.
					slog.Error("kafka dead-letter publish failed", "topic", rec.Topic, "error", perr)
					return
				}
			}
			done = append(done, rec)
		})

		if len(done) > 0 {
			if err := cl.CommitRecords(ctx, done...); err != nil && ctx.Err() == nil {
				slog.Warn("kafka offset commit failed", "error", err)
			}
		}
	}
}
