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
	PoolSize   int           `yaml:"pool_size"` // Pool de conexões
	Vhost      string        `yaml:"vhost"`     // VHost do RabbitMQ (prefixo da chave Redis)
	Prefix     string        `yaml:"prefix"`
	TTL        time.Duration `yaml:"ttl"`
	MaxRetries int           `yaml:"max_retries"`
	RetryDelay time.Duration `yaml:"retry_delay"`
	Timeout    time.Duration `yaml:"timeout"` // Timeout por operação (Store/Get)
}

// NewRedisClient cria novo cliente Redis
func NewRedisClient(config RedisConfig) (*RedisClient, error) {
	if !config.Enabled {
		log.Println("Redis DESABILITADO (config.redis.enabled = false)")
		return nil, nil
	}

	// Aplica defaults para valores não configurados
	if config.MaxRetries == 0 {
		config.MaxRetries = 3 // Default: 3 tentativas
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 100 * time.Millisecond // Default: 100ms
	}
	if config.Timeout == 0 {
		config.Timeout = 2 * time.Second // Default: 2s
	}
	if config.TTL == 0 {
		config.TTL = 120 * time.Second // Default: 120s (2 minutos)
	}
	if config.Prefix == "" {
		config.Prefix = "frames" // Default: "frames"
	}

	log.Printf("Conectando ao Redis: %s (DB: %d)...", config.Address, config.DB)

	rdb := redis.NewClient(&redis.Options{
		Addr:     config.Address,
		Password: config.Password,
		DB:       config.DB,
		PoolSize: config.PoolSize, // Usa valor configurado (0 = default 10*CPU)
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

	// Timeout configurável (default 2s se não configurado)
	timeout := r.config.Timeout
	if timeout == 0 {
		timeout = 2 * time.Second // Default: 2s (melhor que 5s hardcoded)
	}

	// Store com retry
	var lastErr error
	for i := 0; i < r.config.MaxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
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

// StoreWithContext armazena frame no Redis respeitando contexto do caller
// Se o contexto for cancelado, retorna imediatamente
func (r *RedisClient) StoreWithContext(ctx context.Context, cameraID string, frameData []byte, timestamp time.Time) (string, error) {
	if r == nil {
		return "", fmt.Errorf("redis client disabled")
	}

	// Verifica se contexto já foi cancelado antes de começar
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
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

	// Timeout configurável (default 2s se não configurado)
	timeout := r.config.Timeout
	if timeout == 0 {
		timeout = 2 * time.Second // Default: 2s
	}

	// Store com retry (respeitando contexto do caller)
	var lastErr error
	for i := 0; i < r.config.MaxRetries; i++ {
		// Verifica se contexto foi cancelado antes de retry
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// Cria timeout context derivado do contexto do caller
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		err := r.client.Set(timeoutCtx, key, frameData, r.config.TTL).Err()
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

		// Se contexto foi cancelado, não tenta retry
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		// Aguarda retry delay (interruptível por context)
		if i < r.config.MaxRetries-1 {
			select {
			case <-time.After(r.config.RetryDelay):
				// Continue para próximo retry
			case <-ctx.Done():
				return "", ctx.Err()
			}
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

	// Timeout configurável (default 2s se não configurado)
	timeout := r.config.Timeout
	if timeout == 0 {
		timeout = 2 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
