package storage

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"edge-video/v2/internal/monitoring"

	"github.com/redis/go-redis/v9"
)

// RedisClient gerencia conexão e operações com Redis
type RedisClient struct {
	client *redis.Client
	config RedisConfig

	// Stats
	mu          sync.Mutex
	storeCount  uint64
	storeErrors uint64
	getCount    uint64
	getErrors   uint64
}

// RedisConfig define configuração do Redis
type RedisConfig struct {
	Enabled    bool          `yaml:"enabled"`
	Address    string        `yaml:"address"`
	Password   string        `yaml:"password"`
	DB         int           `yaml:"db"`
	Vhost      string        `yaml:"vhost"`     // VHost do RabbitMQ (prefixo da chave Redis)
	Prefix     string        `yaml:"prefix"`
	TTL        time.Duration `yaml:"ttl"`
	MaxRetries int           `yaml:"max_retries"`
	RetryDelay time.Duration `yaml:"retry_delay"`
}

// NewRedisClient cria novo cliente Redis
func NewRedisClient(config RedisConfig) (*RedisClient, error) {
	if !config.Enabled {
		log.Println("Redis DESABILITADO (config.redis.enabled = false)")
		return nil, nil
	}

	log.Printf("Conectando ao Redis: %s (DB: %d)...", config.Address, config.DB)

	rdb := redis.NewClient(&redis.Options{
		Addr:     config.Address,
		Password: config.Password,
		DB:       config.DB,
	})

	// Testa conexão
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	rc := &RedisClient{
		client: rdb,
		config: config,
	}

	log.Printf("✓ Redis conectado: %s (DB: %d, TTL: %v, Prefix: %s)",
		config.Address, config.DB, config.TTL, config.Prefix)

	// Atualiza métrica de conexão
	monitoring.UpdateRedisConnectionStatus(true)

	return rc, nil
}

// Store armazena frame no Redis
func (r *RedisClient) Store(cameraID string, frameData []byte, timestamp time.Time) (string, error) {
	if r == nil {
		return "", fmt.Errorf("redis client disabled")
	}

	start := time.Now()

	// Gera chave: supercarlao_rj_mercado:frames:cam1:1701878400123456789
	// Formato: vhost:prefix:camera:timestamp
	var key string
	if r.config.Vhost != "" {
		key = fmt.Sprintf("%s:%s:%s:%d", r.config.Vhost, r.config.Prefix, cameraID, timestamp.UnixNano())
	} else {
		// Fallback: sem vhost (compatibilidade com config antigo)
		key = fmt.Sprintf("%s:%s:%d", r.config.Prefix, cameraID, timestamp.UnixNano())
	}

	// Store com retry
	var lastErr error
	for i := 0; i < r.config.MaxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := r.client.Set(ctx, key, frameData, r.config.TTL).Err()
		cancel()

		if err == nil {
			// Sucesso
			duration := time.Since(start)

			r.mu.Lock()
			r.storeCount++
			r.mu.Unlock()

			// Registra métrica Prometheus
			monitoring.TrackRedisStore(cameraID, duration, true)

			if r.storeCount%100 == 0 {
				log.Printf("[Redis] Stored %d frames, Last: %s (%v, %d bytes)",
					r.storeCount, key, duration, len(frameData))
			}

			return key, nil
		}

		lastErr = err

		if i < r.config.MaxRetries-1 {
			time.Sleep(r.config.RetryDelay)
		}
	}

	// Falhou após retries
	duration := time.Since(start)
	r.mu.Lock()
	r.storeErrors++
	r.mu.Unlock()

	// Registra métrica Prometheus de erro
	monitoring.TrackRedisStore(cameraID, duration, false)

	return "", fmt.Errorf("redis store failed after %d attempts: %w", r.config.MaxRetries, lastErr)
}

// Get recupera frame do Redis
func (r *RedisClient) Get(key string) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("redis client disabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	r.mu.Lock()
	r.getCount++
	r.mu.Unlock()

	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		r.mu.Lock()
		r.getErrors++
		r.mu.Unlock()

		// Registra métrica Prometheus de erro
		monitoring.TrackRedisGet(false)

		if err == redis.Nil {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, fmt.Errorf("redis get failed: %w", err)
	}

	// Registra métrica Prometheus de sucesso
	monitoring.TrackRedisGet(true)

	return data, nil
}

// Stats retorna estatísticas do Redis
func (r *RedisClient) Stats() (storeCount, storeErrors, getCount, getErrors uint64) {
	if r == nil {
		return 0, 0, 0, 0
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.storeCount, r.storeErrors, r.getCount, r.getErrors
}

// Close fecha conexão Redis
func (r *RedisClient) Close() error {
	if r != nil && r.client != nil {
		return r.client.Close()
	}
	return nil
}

// IsEnabled retorna se Redis está habilitado
func (r *RedisClient) IsEnabled() bool {
	return r != nil
}

// GetTTL retorna o TTL configurado
func (r *RedisClient) GetTTL() time.Duration {
	if r == nil {
		return 0
	}
	return r.config.TTL
}
