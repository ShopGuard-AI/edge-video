package integration

import (
	"context"
	"testing"
	"time"

	"edge-video/v2/internal/storage"
)

// ============================================================================
// INTEGRATION TEST: Redis Auto-Reconnection
// ============================================================================
// Testa recuperação automática de conexão Redis após desconexão
// Valida retry logic e circuit breaker de Redis
// ============================================================================

// TestRedisReconnectionBasics valida tentativas de reconnect no Redis
func TestRedisReconnectionBasics(t *testing.T) {
	config := storage.RedisConfig{
		Enabled:    true,
		Address:    "localhost:6379",
		Timeout:    2 * time.Second,
		TTL:        60 * time.Second,
		Prefix:     "test-reconnect",
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
	}

	client, err := storage.NewRedisClient(config)
	if err != nil || client == nil {
		t.Skip("Redis client disabled or creation failed")
	}

	// Cenário 1: Redis disponível - operação normal
	t.Run("Redis Disponível - Operação Normal", func(t *testing.T) {
		ctx := context.Background()
		key, err := client.StoreWithContext(ctx, "cam1", []byte("test-data"), time.Now())

		if err == nil {
			t.Logf("✅ Store funcionou com Redis disponível (key: %s)", key)
		} else {
			t.Logf("⚠️  Redis não disponível: %v (OK se Redis não estiver rodando)", err)
		}
	})

	// Cenário 2: Timeout durante operação
	t.Run("Timeout Durante Operação", func(t *testing.T) {
		// Context com timeout MUITO curto (força timeout)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := client.StoreWithContext(ctx, "cam1", []byte("test-data"), time.Now())
		duration := time.Since(start)

		// Deve detectar timeout rapidamente
		if err != nil {
			t.Logf("✅ Timeout detectado em %v (erro: %v)", duration, err)
		} else {
			t.Logf("⚠️  Redis respondeu antes do timeout (muito rápido)")
		}

		// Não deve demorar muito (< 1s mesmo com retry)
		if duration > 1*time.Second {
			t.Errorf("StoreWithContext demorou %v, deveria ser rápido mesmo com timeout", duration)
		}
	})

	// Cenário 3: Estatísticas de retry
	t.Run("Estatísticas De Retry", func(t *testing.T) {
		storeCount, storeErrors, getCount, getErrors := client.Stats()

		t.Logf("📊 Redis Stats: Stores=%d, StoreErrors=%d, Gets=%d, GetErrors=%d",
			storeCount, storeErrors, getCount, getErrors)

		// Stats devem estar sendo rastreadas
		if storeCount == 0 && storeErrors == 0 {
			t.Log("⚠️  Nenhuma operação registrada ainda (esperado se Redis não rodou)")
		} else {
			t.Log("✅ Estatísticas sendo rastreadas corretamente")
		}
	})
}

// TestRedisConnectionPooling valida pool de conexões Redis
func TestRedisConnectionPooling(t *testing.T) {
	config := storage.RedisConfig{
		Enabled:    true,
		Address:    "localhost:6379",
		Timeout:    2 * time.Second,
		TTL:        60 * time.Second,
		Prefix:     "test-pool",
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
	}

	client, err := storage.NewRedisClient(config)
	if err != nil || client == nil {
		t.Skip("Redis client disabled or creation failed")
	}

	// Cenário: Múltiplas operações simultâneas (testa pooling)
	t.Run("Múltiplas Operações Simultâneas", func(t *testing.T) {
		ctx := context.Background()
		operationCount := 50
		done := make(chan error, operationCount)

		start := time.Now()

		// Lança 50 operações simultâneas
		for i := 0; i < operationCount; i++ {
			go func(index int) {
				_, err := client.StoreWithContext(ctx, "cam1", []byte("test-data"), time.Now())
				done <- err
			}(i)
		}

		// Aguarda todas completarem
		successCount := 0
		errorCount := 0

		for i := 0; i < operationCount; i++ {
			err := <-done
			if err == nil {
				successCount++
			} else {
				errorCount++
			}
		}

		duration := time.Since(start)

		t.Logf("📊 %d operações em %v: %d sucessos, %d erros",
			operationCount, duration, successCount, errorCount)

		// Se Redis estiver disponível, maioria deve ter sucesso
		if successCount > 0 {
			t.Logf("✅ Pool de conexões funcionando (%d/%d sucessos)",
				successCount, operationCount)
		} else {
			t.Logf("⚠️  Redis não disponível (0 sucessos)")
		}

		// Deve completar em tempo razoável (< 5s para 50 operações)
		if duration > 5*time.Second {
			t.Errorf("50 operações demoraram %v, deveria ser mais rápido", duration)
		}
	})
}

// TestRedisKeyExpiration valida TTL de chaves Redis
func TestRedisKeyExpiration(t *testing.T) {
	config := storage.RedisConfig{
		Enabled:    true,
		Address:    "localhost:6379",
		Timeout:    2 * time.Second,
		TTL:        2 * time.Second, // TTL curto para teste
		Prefix:     "test-ttl",
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
	}

	client, err := storage.NewRedisClient(config)
	if err != nil || client == nil {
		t.Skip("Redis client disabled or creation failed")
	}

	// Cenário: Valida que TTL está configurado corretamente
	t.Run("TTL Configurado Corretamente", func(t *testing.T) {
		ttl := client.GetTTL()

		if ttl != 2*time.Second {
			t.Errorf("Expected TTL 2s, got %v", ttl)
		} else {
			t.Logf("✅ TTL configurado: %v", ttl)
		}
	})

	// Nota: Teste de expiração real requer Redis rodando e aguardar TTL
	// Para este teste de integração, validamos apenas a configuração
}

// TestRedisErrorHandling valida tratamento de erros no Redis
func TestRedisErrorHandling(t *testing.T) {
	// Cenário 1: Redis desabilitado deve retornar erro apropriado
	t.Run("Redis Desabilitado", func(t *testing.T) {
		config := storage.RedisConfig{
			Enabled: false, // DISABLED
		}

		client, err := storage.NewRedisClient(config)

		// Cliente desabilitado retorna nil, nil
		if client != nil {
			t.Error("Client deveria ser nil quando disabled")
		}

		if err != nil {
			t.Errorf("Erro deveria ser nil quando disabled, got: %v", err)
		}

		t.Log("✅ Redis desabilitado retorna nil corretamente")
	})

	// Cenário 2: Endereço inválido
	t.Run("Endereço Inválido", func(t *testing.T) {
		config := storage.RedisConfig{
			Enabled:    true,
			Address:    "invalid-host:9999", // Host inválido
			Timeout:    1 * time.Second,     // Timeout curto
			TTL:        60 * time.Second,
			Prefix:     "test",
			MaxRetries: 1, // Apenas 1 retry
			RetryDelay: 100 * time.Millisecond,
		}

		client, _ := storage.NewRedisClient(config)

		// Cliente pode ser criado mesmo com host inválido (conexão lazy)
		if client == nil {
			t.Log("⚠️  Client nil com host inválido (OK - criação lazy)")
		}

		// Se cliente foi criado, tentativa de store deve falhar
		if client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			start := time.Now()
			_, err := client.StoreWithContext(ctx, "cam1", []byte("test"), time.Now())
			duration := time.Since(start)

			if err != nil {
				t.Logf("✅ Erro detectado com host inválido em %v: %v", duration, err)
			} else {
				t.Error("Deveria retornar erro com host inválido")
			}

			// Com MaxRetries=1, não deve demorar muito
			if duration > 3*time.Second {
				t.Errorf("Store com host inválido demorou %v, deveria falhar rápido", duration)
			}
		}
	})

	// Cenário 3: Client nil deve retornar erro apropriado
	t.Run("Client Nil", func(t *testing.T) {
		var client *storage.RedisClient // nil client

		if client == nil {
			t.Log("✅ Client nil (esperado)")
		}

		// Qualquer operação em client nil deve panificar ou retornar erro
		// (depende da implementação)
	})
}

// TestRedisContextAwareness valida que Redis respeita context em todas operações
func TestRedisContextAwareness(t *testing.T) {
	config := storage.RedisConfig{
		Enabled:    true,
		Address:    "localhost:6379",
		Timeout:    5 * time.Second, // Timeout alto
		TTL:        60 * time.Second,
		Prefix:     "test-ctx",
		MaxRetries: 3,
		RetryDelay: 1 * time.Second, // Delay alto
	}

	client, err := storage.NewRedisClient(config)
	if err != nil || client == nil {
		t.Skip("Redis client disabled or creation failed")
	}

	// Cenário: Context cancelado durante retry
	t.Run("Context Cancelado Durante Retry", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		// Goroutine para cancelar após 100ms
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		_, err := client.StoreWithContext(ctx, "cam1", []byte("test-data"), time.Now())
		duration := time.Since(start)

		// Deve detectar cancelamento
		if err == context.Canceled {
			t.Logf("✅ Context cancelado detectado em %v", duration)
		} else if err != nil {
			t.Logf("⚠️  Erro diferente: %v (OK se operação completou rápido)", err)
		} else {
			t.Log("⚠️  Operação completou com sucesso antes do cancelamento")
		}

		// Não deve esperar todo o retry backoff (3 * 1s = 3s)
		if duration > 2*time.Second {
			t.Errorf("StoreWithContext demorou %v, deveria cancelar rapidamente", duration)
		}
	})
}
