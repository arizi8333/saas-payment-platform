package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/saas-payment-platform/backend/internal/config"
)

// NewRedisClient creates a new Redis client configured from the provided RedisConfig.
func NewRedisClient(cfg *config.RedisConfig) (*redis.Client, error) {
	slog.Info("connecting to Redis",
		"host", cfg.Host,
		"port", cfg.Port,
		"db", cfg.DB,
	)

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	slog.Info("Redis connection established")

	return client, nil
}

// RedisHealthCheck pings the Redis server and returns an error if unreachable.
func RedisHealthCheck(client *redis.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	return nil
}
