package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisStore handles storing frames in Redis with a configurable TTL.
type RedisStore struct {
	client  *redis.Client
	ttl     time.Duration
	prefix  string
	enabled bool
}

// NewRedisStore creates a new RedisStore.
func NewRedisStore(addr string, ttlSeconds int, prefix string, enabled bool) *RedisStore {
	if !enabled {
		return &RedisStore{enabled: false}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	return &RedisStore{
		client:  rdb,
		ttl:     time.Duration(ttlSeconds) * time.Second,
		prefix:  prefix,
		enabled: true,
	}
}

// Enabled returns true if the Redis store is enabled.
func (r *RedisStore) Enabled() bool {
	return r.enabled
}

// SaveFrame stores a frame in Redis with the configured TTL.
// The key is constructed as <prefix>:<cameraID>:<timestamp_RFC3339Nano>.
func (r *RedisStore) SaveFrame(ctx context.Context, cameraID string, timestamp time.Time, data []byte) (string, error) {
	if !r.enabled {
		return "", nil
	}

	key := fmt.Sprintf("%s:%s:%s", r.prefix, cameraID, timestamp.Format(time.RFC3339Nano))
	err := r.client.Set(ctx, key, data, r.ttl).Err()
	if err != nil {
		return "", fmt.Errorf("failed to save frame to redis: %w", err)
	}
	return key, nil
}
