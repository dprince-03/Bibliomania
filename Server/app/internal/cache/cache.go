package cache

import (
	"context"
	"time"
)

type Cache interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, keys ...string) error
	// DeletePrefix removes every key starting with prefix — how list/search
	// caches (one key per page/filter combination) are invalidated.
	DeletePrefix(ctx context.Context, prefix string) error
	Ping(ctx context.Context) error
	Exists(ctx context.Context, key string) (bool, error)
	Flush(ctx context.Context) error
}
