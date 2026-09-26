// Package mongoclient connects reading-service to MongoDB.
package mongoclient

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Connect opens a client and pings the primary, retrying for ~30s while
// the replica set is still electing one.
func Connect(ctx context.Context, url string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(url))
	if err != nil {
		return nil, fmt.Errorf("creating mongo client: %w", err)
	}

	for attempt := 1; attempt <= 15; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err = client.Ping(pingCtx, readpref.Primary())
		cancel()
		if err == nil {
			slog.Info("database connected", "driver", "mongodb")
			return client, nil
		}
		slog.Warn("database not ready, retrying", "driver", "mongodb", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, fmt.Errorf("connecting to mongodb: %w", err)
}
