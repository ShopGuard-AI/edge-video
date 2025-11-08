package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisStore handles storing frames in Redis with a configurable TTL.
type RedisStore struct {
	client       *redis.Client
	ttl          time.Duration
	enabled      bool
	keyGenerator *KeyGenerator
}

// NewRedisStore creates a new RedisStore.
// vhost é o identificador único do cliente (extraído do AMQP vhost)
func NewRedisStore(addr string, ttlSeconds int, prefix string, vhost string, enabled bool) *RedisStore {
	if !enabled {
		return &RedisStore{enabled: false}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// Cria o KeyGenerator com estratégia sequence (recomendado para prevenir colisões)
	keyGen := NewKeyGenerator(KeyGeneratorConfig{
		Strategy: StrategySequence,
		Prefix:   prefix,
		Vhost:    vhost,
	})

	return &RedisStore{
		client:       rdb,
		ttl:          time.Duration(ttlSeconds) * time.Second,
		enabled:      true,
		keyGenerator: keyGen,
	}
}

// Enabled returns true if the Redis store is enabled.
func (r *RedisStore) Enabled() bool {
	return r.enabled
}

// SaveFrame stores a frame in Redis with the configured TTL.
// Usa KeyGenerator para criar chaves únicas prevenindo colisões.
// Formato: {prefix}:{vhost}:{cameraID}:{timestamp}:{sequence}
func (r *RedisStore) SaveFrame(ctx context.Context, cameraID string, timestamp time.Time, data []byte) (string, error) {
	if !r.enabled {
		return "", nil
	}

	key := r.keyGenerator.GenerateKey(cameraID, timestamp)
	err := r.client.Set(ctx, key, data, r.ttl).Err()
	if err != nil {
		return "", fmt.Errorf("failed to save frame to redis: %w", err)
	}
	return key, nil
}

// GetFrame retrieves a frame by its exact key
func (r *RedisStore) GetFrame(ctx context.Context, key string) ([]byte, error) {
	if !r.enabled {
		return nil, fmt.Errorf("redis store is disabled")
	}

	val, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("frame not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get frame: %w", err)
	}
	return val, nil
}

// QueryFrames busca frames usando padrão
// Se cameraID for vazio, busca todos os frames do vhost
func (r *RedisStore) QueryFrames(ctx context.Context, cameraID string) ([]string, error) {
	if !r.enabled {
		return nil, fmt.Errorf("redis store is disabled")
	}

	pattern := r.keyGenerator.QueryPattern(cameraID, "")
	
	var keys []string
	iter := r.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan keys: %w", err)
	}

	return keys, nil
}

// GetVhost retorna o vhost configurado para este store
func (r *RedisStore) GetVhost() string {
	if !r.enabled {
		return ""
	}
	return r.keyGenerator.GetConfig().Vhost
}
