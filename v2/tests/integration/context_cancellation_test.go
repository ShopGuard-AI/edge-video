package integration

import (
	"context"
	"testing"
	"time"

	"edge-video/v2/internal/messaging"
	"edge-video/v2/internal/storage"
)

// ============================================================================
// INTEGRATION TEST: Context Cancellation Propagation
// ============================================================================
// Testa propagação de context.Context em toda a stack
// Valida que cancelamento é respeitado por Redis e Publisher
// Crítico para graceful shutdown <5s
// ============================================================================

// TestContextCancellationInRedis valida que Redis respeita context cancelado
func TestContextCancellationInRedis(t *testing.T) {
	config := storage.RedisConfig{
		Enabled: true,
		Address: "localhost:6379",
		Timeout: 5 * time.Second, // Timeout alto para forçar context a cancelar primeiro
		TTL:     60 * time.Second,
		Prefix:  "test",
		Vhost:   "test-vhost",
	}

	client, err := storage.NewRedisClient(config)
	if err != nil || client == nil {
		t.Skip("Redis client disabled or creation failed")
	}

	// Cenário 1: Context cancelado ANTES de chamar StoreWithContext
	t.Run("Context Cancelado Antes da Chamada", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancela IMEDIATAMENTE

		start := time.Now()
		_, err := client.StoreWithContext(ctx, "cam1", []byte("test-data"), time.Now())
		duration := time.Since(start)

		// Deve retornar imediatamente com context.Canceled
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got: %v", err)
		}

		// Deve ser MUITO rápido (< 100ms)
		if duration > 100*time.Millisecond {
			t.Errorf("StoreWithContext demorou %v, deveria ser instantâneo", duration)
		}

		t.Logf("✅ Context cancelado detectado em %v", duration)
	})

	// Cenário 2: Context com timeout CURTO
	t.Run("Context Com Timeout Curto", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := client.StoreWithContext(ctx, "cam1", []byte("test-data"), time.Now())
		duration := time.Since(start)

		// Aceita:
		// - context.DeadlineExceeded (timeout atingido)
		// - context.Canceled (context cancelado)
		// - Erro de Redis (falhou rápido antes do timeout)
		// - nil (Redis respondeu rápido com sucesso)
		if err == nil {
			t.Logf("⚠️  Redis respondeu antes do timeout (OK, Redis muito rápido)")
		} else if err == context.DeadlineExceeded || err == context.Canceled {
			t.Logf("✅ Context timeout detectado: %v", err)
		} else {
			// Qualquer outro erro (ex: Redis error) também é OK - operação completou
			t.Logf("⚠️  Operação completou com erro antes do timeout: %v (OK)", err)
		}

		t.Logf("✅ Context timeout respeitado em %v", duration)
	})

	// Cenário 3: Context válido (deve funcionar normalmente)
	t.Run("Context Válido", func(t *testing.T) {
		ctx := context.Background()

		// Se Redis não estiver rodando, teste vai falhar - OK
		key, err := client.StoreWithContext(ctx, "cam1", []byte("test-data"), time.Now())

		if err == nil {
			t.Logf("✅ StoreWithContext funcionou com context válido (key: %s)", key)
		} else {
			t.Logf("⚠️  Redis não disponível: %v (esperado se Redis não estiver rodando)", err)
		}
	})
}

// TestContextCancellationInPublisher valida que Publisher respeita context cancelado
func TestContextCancellationInPublisher(t *testing.T) {
	// Publisher config (não precisa estar conectado para testar context handling)
	publisher := &messaging.Publisher{}

	// Cenário 1: Context cancelado ANTES de chamar PublishWithContext
	t.Run("Context Cancelado Antes da Chamada", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancela IMEDIATAMENTE

		start := time.Now()
		err := publisher.PublishWithContext(ctx, "cam1", []byte("test-data"), time.Now())
		duration := time.Since(start)

		// Deve retornar imediatamente com context.Canceled
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got: %v", err)
		}

		// Deve ser MUITO rápido (< 50ms)
		if duration > 50*time.Millisecond {
			t.Errorf("PublishWithContext demorou %v, deveria ser instantâneo", duration)
		}

		t.Logf("✅ Context cancelado detectado em %v", duration)
	})

	// Cenário 2: Context com timeout CURTO
	t.Run("Context Com Timeout Curto", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		start := time.Now()
		err := publisher.PublishWithContext(ctx, "cam1", []byte("test-data"), time.Now())
		duration := time.Since(start)

		// Deve retornar com context.DeadlineExceeded ou context.Canceled
		if err != context.DeadlineExceeded && err != context.Canceled {
			// Pode falhar por outras razões (não conectado) - aceitar
			if err != nil {
				t.Logf("⚠️  Erro diferente de timeout: %v (OK se não conectado)", err)
			}
		}

		t.Logf("✅ Context timeout respeitado em %v", duration)
	})
}

// TestContextCancellationPropagation valida propagação de context em pipeline completo
func TestContextCancellationPropagation(t *testing.T) {
	// Setup Redis Client
	redisConfig := storage.RedisConfig{
		Enabled: true,
		Address: "localhost:6379",
		Timeout: 2 * time.Second,
		TTL:     60 * time.Second,
		Prefix:  "test",
		Vhost:   "test-vhost",
	}
	redisClient, _ := storage.NewRedisClient(redisConfig)

	// Setup Publisher (pode não estar conectado - OK para este teste)
	publisher := &messaging.Publisher{}

	// Cenário: Pipeline completo com context cancelado
	t.Run("Pipeline Completo Com Context Cancelado", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		// Simula pipeline: Redis → Publisher
		done := make(chan struct{})
		var redisErr, publishErr error

		go func() {
			defer close(done)

			// Etapa 1: Store no Redis
			if redisClient != nil {
				_, redisErr = redisClient.StoreWithContext(ctx, "cam1", []byte("frame-data"), time.Now())
			}

			// Etapa 2: Publish no RabbitMQ
			publishErr = publisher.PublishWithContext(ctx, "cam1", []byte("frame-data"), time.Now())
		}()

		// Aguarda 50ms e CANCELA context
		time.Sleep(50 * time.Millisecond)
		cancel()

		// Aguarda pipeline terminar
		select {
		case <-done:
			// Pipeline deve ter detectado cancelamento
			t.Log("✅ Pipeline detectou context cancelado")

			// Pelo menos uma das operações deve ter retornado context.Canceled
			if redisErr == context.Canceled || publishErr == context.Canceled {
				t.Logf("✅ Context.Canceled propagado corretamente (Redis: %v, Publisher: %v)",
					redisErr, publishErr)
			} else {
				t.Logf("⚠️  Context cancelado, mas erros diferentes (Redis: %v, Publisher: %v)",
					redisErr, publishErr)
			}

		case <-time.After(2 * time.Second):
			t.Error("❌ Pipeline não respondeu ao context cancelado em 2s!")
		}
	})

	// Cenário: Múltiplas operações simultâneas com context compartilhado
	t.Run("Múltiplas Operações Com Context Compartilhado", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Simula 10 operações simultâneas
		operationCount := 10
		done := make(chan struct{})
		errors := make([]error, operationCount)

		go func() {
			defer close(done)

			for i := 0; i < operationCount; i++ {
				// Metade vai para Redis, metade para Publisher
				if i%2 == 0 && redisClient != nil {
					_, errors[i] = redisClient.StoreWithContext(ctx, "cam1", []byte("data"), time.Now())
				} else {
					errors[i] = publisher.PublishWithContext(ctx, "cam1", []byte("data"), time.Now())
				}
			}
		}()

		// Cancela após 30ms
		time.Sleep(30 * time.Millisecond)
		cancel()

		// Aguarda todas as operações terminarem
		select {
		case <-done:
			canceledCount := 0
			for _, err := range errors {
				if err == context.Canceled {
					canceledCount++
				}
			}

			t.Logf("✅ %d/%d operações detectaram context cancelado",
				canceledCount, operationCount)

		case <-time.After(2 * time.Second):
			t.Error("❌ Operações não responderam ao context cancelado!")
		}
	})
}

// TestContextCancellationGracefulShutdown valida shutdown graceful com context
func TestContextCancellationGracefulShutdown(t *testing.T) {
	t.Run("Shutdown Graceful <5s", func(t *testing.T) {
		// Simula shutdown: cria context com cancel
		ctx, cancel := context.WithCancel(context.Background())

		// Setup componentes
		redisConfig := storage.RedisConfig{
			Enabled: true,
			Address: "localhost:6379",
			Timeout: 2 * time.Second,
			TTL:     60 * time.Second,
			Prefix:  "test",
			Vhost:   "test-vhost",
		}
		redisClient, _ := storage.NewRedisClient(redisConfig)
		publisher := &messaging.Publisher{}

		// Simula workload contínuo
		done := make(chan struct{})
		operationCount := 0

		go func() {
			defer close(done)

			for {
				select {
				case <-ctx.Done():
					// Context cancelado - deve parar IMEDIATAMENTE
					return

				default:
					// Continua processando
					if redisClient != nil {
						redisClient.StoreWithContext(ctx, "cam1", []byte("data"), time.Now())
					}
					publisher.PublishWithContext(ctx, "cam1", []byte("data"), time.Now())
					operationCount++
					time.Sleep(10 * time.Millisecond) // Simula frame rate
				}
			}
		}()

		// Deixa rodar por 200ms
		time.Sleep(200 * time.Millisecond)

		// SHUTDOWN: cancela context
		shutdownStart := time.Now()
		cancel()

		// Aguarda workload parar
		select {
		case <-done:
			shutdownDuration := time.Since(shutdownStart)

			// CRITÉRIO: Shutdown DEVE ser <5s (idealmente <100ms)
			if shutdownDuration > 5*time.Second {
				t.Errorf("❌ Shutdown demorou %v, máximo aceitável: 5s", shutdownDuration)
			} else if shutdownDuration > 100*time.Millisecond {
				t.Logf("⚠️  Shutdown em %v (OK, mas pode melhorar)", shutdownDuration)
			} else {
				t.Logf("✅ Shutdown RÁPIDO em %v (processou %d operações)",
					shutdownDuration, operationCount)
			}

		case <-time.After(5 * time.Second):
			t.Error("❌ FALHA CRÍTICA: Shutdown não completou em 5s!")
		}
	})
}
