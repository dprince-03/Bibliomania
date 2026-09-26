package notification

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisSentLog keeps "email sent for event X" markers for a week — longer
// than any broker would plausibly redeliver.
type RedisSentLog struct {
	rdb *redis.Client
}

func NewRedisSentLog(rdb *redis.Client) *RedisSentLog {
	return &RedisSentLog{rdb: rdb}
}

const sentTTL = 7 * 24 * time.Hour

func key(eventID string) string { return "notification:sent:" + eventID }

func (l *RedisSentLog) WasSent(ctx context.Context, eventID string) (bool, error) {
	n, err := l.rdb.Exists(ctx, key(eventID)).Result()
	return n > 0, err
}

func (l *RedisSentLog) MarkSent(ctx context.Context, eventID string) error {
	return l.rdb.Set(ctx, key(eventID), 1, sentTTL).Err()
}
