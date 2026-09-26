package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
	prefix string
}

func NewRedisCache(client *redis.Client, prefix string) Cache {
	return &RedisCache{
		client: client,
		prefix: prefix,
	}
}

func (r *RedisCache) prefixKey(key string) string {
	return fmt.Sprintf("%s:%s", r.prefix, key)
}

// Set stores strings and []byte as-is and JSON-encodes anything else.
// (It used to JSON-encode everything, so the already-encoded strings every
// caller passes were stored double-encoded and never decoded back — the
// catalog cache had never produced a single hit.)
func (r *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	var data []byte
	switch v := value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		var err error
		if data, err = json.Marshal(value); err != nil {
			return fmt.Errorf("cache marshal error: %w", err)
		}
	}

	if err := r.client.Set(ctx, r.prefixKey(key), data, ttl).Err(); err != nil {
		return fmt.Errorf("cache set error: %w", err)
	}

	return nil
}

func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, r.prefixKey(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrCacheMiss
		}

		return "", fmt.Errorf("cache get error: %w", err)
	}

	return val, nil
}

func (r *RedisCache) Delete(ctx context.Context, keys ...string) error {
	prefixed := make([]string, len(keys))
	for i, k := range keys {
		prefixed[i] = r.prefixKey(k)
	}

	if err := r.client.Del(ctx, prefixed...).Err(); err != nil {
		return fmt.Errorf("cache delete error: %w", err)
	}

	return nil
}

func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Exists(ctx, r.prefixKey(key)).Result()
	if err != nil {
		return false, fmt.Errorf("cache exist error: %w", err)
	}

	return count > 0, nil
}

func (r *RedisCache) Flush(ctx context.Context) error {
	pattern := fmt.Sprintf("%s:*", r.prefix)

	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("cache flush scan error: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	if err := r.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("cache flush error: %w", err)
	}

	return nil
}

// DeletePrefix uses SCAN (not KEYS, which blocks Redis) to find and delete
// every key under prefix.
func (r *RedisCache) DeletePrefix(ctx context.Context, prefix string) error {
	iter := r.client.Scan(ctx, 0, r.prefixKey(prefix)+"*", 100).Iterator()
	var batch []string
	for iter.Next(ctx) {
		batch = append(batch, iter.Val())
		if len(batch) == 100 {
			if err := r.client.Del(ctx, batch...).Err(); err != nil {
				return fmt.Errorf("cache delete-prefix error: %w", err)
			}
			batch = batch[:0]
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("cache delete-prefix scan error: %w", err)
	}
	if len(batch) > 0 {
		if err := r.client.Del(ctx, batch...).Err(); err != nil {
			return fmt.Errorf("cache delete-prefix error: %w", err)
		}
	}
	return nil
}

func (r *RedisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}
