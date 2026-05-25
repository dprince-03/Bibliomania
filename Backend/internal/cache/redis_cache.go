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

func (r *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal error: %w", err)
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
