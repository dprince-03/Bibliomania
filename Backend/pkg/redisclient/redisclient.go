package redisclient

import (
	"bibliotheca/internal/config"
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

func Connect(cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		PoolSize:     10,
		MinIdleConns: 3,
	})

	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %m", err)
	}

	log.Printf("Redis connected successfully !!!")
	return client, nil
}
