package events

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/dprince-03/Bibliomania/internal/telemetry"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoOutbox is the transactional outbox for reading-service, the one
// MongoDB-backed service. Same idea as the SQL outbox (outbox.go): the
// event document is inserted in the same multi-document transaction as the
// session/bookmark write — which is why MongoDB runs as a (single-node)
// replica set in infra/docker, since standalone mongod has no transactions.
type MongoOutbox struct {
	coll *mongo.Collection
}

type mongoOutboxDoc struct {
	ID          string     `bson:"_id"`
	Type        string     `bson:"type"`
	Payload     string     `bson:"payload"`
	CreatedAt   time.Time  `bson:"created_at"`
	PublishedAt *time.Time `bson:"published_at"`
	LeaseUntil  *time.Time `bson:"lease_until"`
}

func NewMongoOutbox(db *mongo.Database) *MongoOutbox {
	return &MongoOutbox{coll: db.Collection("outbox_events")}
}

// EnsureIndexes creates the relay's lookup index and a TTL index that
// removes published events after a week.
func (o *MongoOutbox) EnsureIndexes(ctx context.Context) error {
	_, err := o.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "published_at", Value: 1}, {Key: "created_at", Value: 1}}},
		{Keys: bson.D{{Key: "published_at", Value: 1}}, Options: options.Index().
			SetExpireAfterSeconds(7 * 24 * 3600).
			SetPartialFilterExpression(bson.M{"published_at": bson.M{"$type": "date"}}).
			SetName("ttl_published")},
	})
	return err
}

// Add inserts e; call with the session context of a running transaction.
func (o *MongoOutbox) Add(ctx context.Context, e Event) error {
	payload, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = o.coll.InsertOne(ctx, mongoOutboxDoc{
		ID: e.ID, Type: e.Type, Payload: string(payload), CreatedAt: time.Now().UTC(),
	})
	return err
}

// RunRelay publishes pending events oldest-first until ctx ends. Each
// event is claimed with a 30s lease before publishing, so two replicas
// don't publish the same event concurrently (a duplicate after a crash is
// still possible — consumers dedupe, and JetStream dedupes by message ID).
func (o *MongoOutbox) RunRelay(ctx context.Context, pub Publisher, interval time.Duration) {
	slog.Info("outbox relay started", "store", "mongodb")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for ctx.Err() == nil {
				done, err := o.relayOne(ctx, pub)
				if err != nil {
					slog.Warn("outbox relay failed", "error", err)
					break
				}
				if !done {
					break
				}
			}
		}
	}
}

func (o *MongoOutbox) relayOne(ctx context.Context, pub Publisher) (bool, error) {
	now := time.Now().UTC()
	lease := now.Add(30 * time.Second)

	var doc mongoOutboxDoc
	err := o.coll.FindOneAndUpdate(ctx,
		bson.M{
			"published_at": nil,
			"$or":          bson.A{bson.M{"lease_until": nil}, bson.M{"lease_until": bson.M{"$lt": now}}},
		},
		bson.M{"$set": bson.M{"lease_until": lease}},
		options.FindOneAndUpdate().SetSort(bson.D{{Key: "created_at", Value: 1}}).SetReturnDocument(options.After),
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	e, err := parse([]byte(doc.Payload))
	if err == nil {
		if err = pub.Publish(ctx, e); err != nil {
			telemetry.EventsPublished.WithLabelValues(doc.Type, "error").Inc()
			return false, err
		}
		telemetry.EventsPublished.WithLabelValues(doc.Type, "ok").Inc()
	} else {
		slog.Error("dropping malformed outbox document", "id", doc.ID, "error", err)
	}

	_, err = o.coll.UpdateByID(ctx, doc.ID, bson.M{"$set": bson.M{"published_at": time.Now().UTC()}})
	return err == nil, err
}
