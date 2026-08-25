// Package idempotency contains the Processing-Worker-specific
// implementation of the fiapx-events idempotency.Store interface.
package idempotency

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore implements events/idempotency.Store using SETNX with a TTL.
// Chosen over a dedicated Postgres table so this worker doesn't need its
// own database just for a dedup ledger — it reuses the Redis instance
// already deployed for fiapx-video-service's status cache. A rare
// duplicate video.uploaded delivery that slips past the TTL window
// re-runs ffmpeg once more; the downstream consumers (Video Service,
// Notification Service) are themselves idempotent, so the only cost is
// wasted CPU, never a correctness bug.
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisStore(client *redis.Client, ttl time.Duration) *RedisStore {
	return &RedisStore{client: client, ttl: ttl}
}

// SeenOrRecord returns alreadySeen=true iff another call already won the
// SETNX for this (eventId, consumer) pair within the TTL window.
func (s *RedisStore) SeenOrRecord(ctx context.Context, eventID, consumer string) (bool, error) {
	key := "idempotency:" + consumer + ":" + eventID
	wonRace, err := s.client.SetNX(ctx, key, "1", s.ttl).Result()
	if err != nil {
		return false, err
	}
	return !wonRace, nil
}
