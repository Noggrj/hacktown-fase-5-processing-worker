// Package cache wraps the go-redis client used to cache each user's
// video status listing (GET /videos). Postgres remains the source of
// truth — a cache miss or Redis outage falls back to a normal query,
// it never blocks the request.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewClient(addr string) (*redis.Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("REDIS_ADDR is empty")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return client, nil
}

func Healthy(ctx context.Context, client *redis.Client) error {
	if client == nil {
		return fmt.Errorf("redis client not initialized")
	}
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return client.Ping(c).Err()
}
