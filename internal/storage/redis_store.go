package storage

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisStore handles storing frames in Redis with a configurable TTL.
type RedisStore struct {
	client       *redis.Client
	ttl          time.Duration
	enabled      bool
	keyGenerator *KeyGenerator
	options      *redis.Options
	mu           sync.RWMutex
}

// NewRedisStore creates a new RedisStore.
// vhost é o identificador único do cliente (extraído do AMQP vhost)
func NewRedisStore(addr string, ttlSeconds int, prefix string, vhost string, enabled bool, username string, password string) *RedisStore {
	if !enabled {
		return &RedisStore{enabled: false}
	}

	options := &redis.Options{
		Addr:         addr,
		MaxRetries:   2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
		PoolSize:     64,
		MinIdleConns: 8,
	}

	if username != "" {
		options.Username = username
	}

	if password != "" {
		options.Password = password
	}

	rdb := redis.NewClient(options)

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
		options:      options,
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
	var lastErr error

	for attempt := 0; attempt < 2; attempt++ {
		client := r.getClient()
		if client == nil {
			return "", fmt.Errorf("failed to save frame to redis: client not initialized")
		}

		err := client.Set(ctx, key, data, r.ttl).Err()
		if err == nil {
			return key, nil
		}

		lastErr = err

		if !r.shouldRetry(err) {
			break
		}

		if recErr := r.reconnect(); recErr != nil {
			lastErr = fmt.Errorf("%w; reconnect failed: %v", err, recErr)
			break
		}
	}

	return "", fmt.Errorf("failed to save frame to redis: %w", lastErr)
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

func (r *RedisStore) getClient() *redis.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.client
}

func (r *RedisStore) reconnect() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.client != nil {
		_ = r.client.Close()
	}

	r.client = redis.NewClient(r.options)

	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	return r.client.Ping(pingCtx).Err()
}

func (r *RedisStore) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	if errors.Is(err, net.ErrClosed) {
		return true
	}

	message := err.Error()
	if strings.Contains(message, "broken pipe") {
		return true
	}

	if strings.Contains(message, "connection reset by peer") {
		return true
	}

	if strings.Contains(message, "use of closed network connection") {
		return true
	}

	return false
}
