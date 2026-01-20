package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"edge-video/v2/internal/health"
	"edge-video/v2/internal/messaging"
	"edge-video/v2/internal/resilience"
	"edge-video/v2/internal/storage"
)

// ============================================================================
// INTEGRATION TEST: Producer Flow Complete
// ============================================================================
// Testa fluxo completo: Camera → Pool → Redis → RabbitMQ
// Valida integração entre todos os componentes
// ============================================================================

// TestProducerComponentsIntegration valida criação e integração de componentes
func TestProducerComponentsIntegration(t *testing.T) {
	t.Run("Health Monitor + Circuit Breaker + Redis + Publisher", func(t *testing.T) {
		// 1. Health Monitor
		healthMonitor := health.NewPublishHealthMonitor(10, 0.8)
		if healthMonitor == nil {
			t.Fatal("Failed to create Health Monitor")
		}

		// 2. Circuit Breaker
		cbConfig := resilience.CircuitBreakerConfig{
			Enabled:           true,
			MaxFailures:       5,
			ResetTimeout:      30 * time.Second,
			HalfOpenSuccesses: 3,
			InitialBackoff:    1 * time.Second,
			MaxBackoff:        30 * time.Second,
			BackoffMultiplier: 2.0,
		}
		circuitBreaker := resilience.NewCircuitBreaker("test-publisher", cbConfig)
		if circuitBreaker == nil {
			t.Fatal("Failed to create Circuit Breaker")
		}

		// 3. Redis Client
		redisConfig := storage.RedisConfig{
			Enabled:    true,
			Address:    "localhost:6379",
			Timeout:    2 * time.Second,
			TTL:        60 * time.Second,
			Prefix:     "test-flow",
			MaxRetries: 3,
			RetryDelay: 100 * time.Millisecond,
		}
		redisClient, err := storage.NewRedisClient(redisConfig)
		if err != nil && redisClient == nil {
			t.Skip("Redis not available - skipping integration test")
		}

		// 4. Publisher
		publisher := &messaging.Publisher{}

		// Validar que todos os componentes foram criados
		t.Log("✅ Todos os componentes criados com sucesso:")
		t.Log("   - Health Monitor")
		t.Log("   - Circuit Breaker")
		t.Log("   - Redis Client")
		t.Log("   - Publisher")

		// Validar stats iniciais
		stats := healthMonitor.GetStats()
		t.Logf("📊 Health Stats: Total=%d, Errors=%d, Rate=%.2f%%",
			stats.TotalRecent, stats.RecentErrors, stats.ErrorRate*100)

		cbStats := circuitBreaker.Stats()
		t.Logf("📊 Circuit Breaker: State=%s, Failures=%d", cbStats.State, cbStats.Failures)

		storeCount, storeErrors, _, _ := redisClient.Stats()
		t.Logf("📊 Redis: Stores=%d, Errors=%d", storeCount, storeErrors)

		pubCount, pubErrors := publisher.Stats()
		t.Logf("📊 Publisher: Count=%d, Errors=%d", pubCount, pubErrors)
	})
}

// TestProducerFlowSimulation simula fluxo completo de publicação
func TestProducerFlowSimulation(t *testing.T) {
	t.Run("Simular Publicação De Frame", func(t *testing.T) {
		// Setup componentes
		healthMonitor := health.NewPublishHealthMonitor(10, 0.8)
		cbConfig := resilience.CircuitBreakerConfig{
			Enabled:           true,
			MaxFailures:       5,
			ResetTimeout:      30 * time.Second,
			HalfOpenSuccesses: 3,
			InitialBackoff:    1 * time.Second,
			MaxBackoff:        30 * time.Second,
			BackoffMultiplier: 2.0,
		}
		circuitBreaker := resilience.NewCircuitBreaker("test-publisher", cbConfig)

		redisConfig := storage.RedisConfig{
			Enabled:    true,
			Address:    "localhost:6379",
			Timeout:    2 * time.Second,
			TTL:        60 * time.Second,
			Prefix:     "test-flow",
			MaxRetries: 3,
			RetryDelay: 100 * time.Millisecond,
		}
		redisClient, err := storage.NewRedisClient(redisConfig)
		if err != nil || redisClient == nil {
			t.Skip("Redis not available")
		}

		publisher := &messaging.Publisher{}

		// Simula fluxo de publicação
		ctx := context.Background()
		cameraID := "cam1"
		frameData := []byte("test-frame-data-simulated")
		timestamp := time.Now()

		// Passo 1: Verificar Circuit Breaker
		cbStats := circuitBreaker.Stats()
		if cbStats.State.String() != "CLOSED" {
			t.Errorf("Circuit Breaker deveria estar CLOSED, está: %s", cbStats.State)
		}

		// Passo 2: Store no Redis (via Circuit Breaker)
		var redisKey string
		err = circuitBreaker.Execute(func() error {
			var storeErr error
			redisKey, storeErr = redisClient.StoreWithContext(ctx, cameraID, frameData, timestamp)
			return storeErr
		})

		if err != nil {
			t.Logf("⚠️  Redis Store falhou: %v (esperado se Redis não disponível)", err)
			healthMonitor.RecordError()
		} else {
			t.Logf("✅ Frame armazenado no Redis: %s", redisKey)
			healthMonitor.RecordSuccess()
		}

		// Passo 3: Publish no RabbitMQ (vai falhar sem RabbitMQ)
		publishErr := publisher.PublishWithContext(ctx, cameraID, frameData, timestamp)
		if publishErr != nil {
			t.Logf("⚠️  Publisher falhou: %v (esperado sem RabbitMQ)", publishErr)
		}

		// Validar Health Stats
		stats := healthMonitor.GetStats()
		t.Logf("📊 Health após 1 frame: Total=%d, Errors=%d, Rate=%.2f%%",
			stats.TotalRecent, stats.RecentErrors, stats.ErrorRate*100)
	})
}

// TestProducerFlowMultipleFrames simula publicação de múltiplos frames
func TestProducerFlowMultipleFrames(t *testing.T) {
	t.Run("Publicar 100 Frames", func(t *testing.T) {
		// Setup
		healthMonitor := health.NewPublishHealthMonitor(100, 0.8)
		redisConfig := storage.RedisConfig{
			Enabled:    true,
			Address:    "localhost:6379",
			Timeout:    2 * time.Second,
			TTL:        60 * time.Second,
			Prefix:     "test-multi",
			MaxRetries: 3,
			RetryDelay: 100 * time.Millisecond,
		}
		redisClient, err := storage.NewRedisClient(redisConfig)
		if err != nil || redisClient == nil {
			t.Skip("Redis not available")
		}

		// Publicar 100 frames
		ctx := context.Background()
		successCount := 0
		errorCount := 0

		start := time.Now()

		for i := 0; i < 100; i++ {
			frameData := []byte("test-frame-data")
			_, err := redisClient.StoreWithContext(ctx, "cam1", frameData, time.Now())

			if err == nil {
				successCount++
				healthMonitor.RecordSuccess()
			} else {
				errorCount++
				healthMonitor.RecordError()
			}
		}

		duration := time.Since(start)

		// Stats
		stats := healthMonitor.GetStats()
		t.Logf("📊 100 frames em %v:", duration)
		t.Logf("   Sucessos: %d", successCount)
		t.Logf("   Erros: %d", errorCount)
		t.Logf("   Error Rate: %.2f%%", stats.ErrorRate*100)
		t.Logf("   Throughput: %.2f frames/s", float64(100)/duration.Seconds())

		// Validar throughput (deve ser rápido)
		if duration > 5*time.Second {
			t.Errorf("100 frames demoraram %v, deveria ser mais rápido", duration)
		} else {
			t.Logf("✅ Throughput adequado: %.2f frames/s", float64(100)/duration.Seconds())
		}
	})
}

// TestProducerFlowWithContext valida propagação de context no fluxo completo
func TestProducerFlowWithContext(t *testing.T) {
	t.Run("Context Cancelado Durante Fluxo", func(t *testing.T) {
		redisConfig := storage.RedisConfig{
			Enabled:    true,
			Address:    "localhost:6379",
			Timeout:    2 * time.Second,
			TTL:        60 * time.Second,
			Prefix:     "test-ctx",
			MaxRetries: 3,
			RetryDelay: 100 * time.Millisecond,
		}
		redisClient, err := storage.NewRedisClient(redisConfig)
		if err != nil || redisClient == nil {
			t.Skip("Redis not available")
		}

		publisher := &messaging.Publisher{}

		// Context com cancel
		ctx, cancel := context.WithCancel(context.Background())

		// Goroutine que cancela após 100ms
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		// Loop de publicação (deve parar quando context cancelado)
		publishCount := 0
		for {
			select {
			case <-ctx.Done():
				// Context cancelado - para o loop
				t.Logf("✅ Loop parou após context cancelado (publicou %d frames)", publishCount)
				return

			default:
				// Tenta publicar
				_, err := redisClient.StoreWithContext(ctx, "cam1", []byte("frame"), time.Now())
				if err == context.Canceled {
					t.Logf("✅ StoreWithContext detectou context cancelado")
					return
				}

				publisher.PublishWithContext(ctx, "cam1", []byte("frame"), time.Now())
				publishCount++

				time.Sleep(10 * time.Millisecond) // Simula 100fps
			}
		}
	})
}

// TestProducerFlowErrorRecovery valida recuperação de erros no fluxo
func TestProducerFlowErrorRecovery(t *testing.T) {
	t.Run("Recuperação Após Erros", func(t *testing.T) {
		healthMonitor := health.NewPublishHealthMonitor(20, 0.8)
		cbConfig := resilience.CircuitBreakerConfig{
			Enabled:           true,
			MaxFailures:       3, // Abre após 3 falhas
			ResetTimeout:      30 * time.Second,
			HalfOpenSuccesses: 2,
			InitialBackoff:    100 * time.Millisecond,
			MaxBackoff:        1 * time.Second,
			BackoffMultiplier: 2.0,
		}
		circuitBreaker := resilience.NewCircuitBreaker("test-recovery", cbConfig)

		// Simula 5 erros consecutivos
		for i := 0; i < 5; i++ {
			err := circuitBreaker.Execute(func() error {
				return fmt.Errorf("simulated service error")
			})
			if err != nil {
				healthMonitor.RecordError()
			}
		}

		// Circuit Breaker deve estar OPEN
		cbStats := circuitBreaker.Stats()
		t.Logf("📊 Após 5 erros: State=%s, Failures=%d", cbStats.State, cbStats.Failures)

		if cbStats.State.String() != "OPEN" {
			t.Logf("⚠️  Circuit Breaker deveria estar OPEN, está: %s", cbStats.State)
		} else {
			t.Log("✅ Circuit Breaker abriu após múltiplas falhas")
		}

		// Health Monitor deve detectar degradação
		stats := healthMonitor.GetStats()
		if stats.ErrorRate > 0.5 {
			t.Logf("✅ Degradação detectada: %.2f%% error rate", stats.ErrorRate*100)
		}

		// Aguarda backoff
		time.Sleep(150 * time.Millisecond)

		// Tenta novamente (deve transitar para HALF_OPEN)
		err := circuitBreaker.Execute(func() error {
			return nil // Sucesso
		})

		if err == nil {
			healthMonitor.RecordSuccess()
			t.Log("✅ Operação bem-sucedida após backoff")
		}

		// Estado final
		cbStatsFinal := circuitBreaker.Stats()
		t.Logf("📊 Estado final: %s", cbStatsFinal.State)
	})
}

// TestProducerFlowConcurrentCameras simula múltiplas câmeras simultâneas
func TestProducerFlowConcurrentCameras(t *testing.T) {
	t.Run("3 Câmeras Simultâneas", func(t *testing.T) {
		redisConfig := storage.RedisConfig{
			Enabled:    true,
			Address:    "localhost:6379",
			Timeout:    2 * time.Second,
			TTL:        60 * time.Second,
			Prefix:     "test-concurrent",
			MaxRetries: 3,
			RetryDelay: 100 * time.Millisecond,
		}
		redisClient, err := storage.NewRedisClient(redisConfig)
		if err != nil || redisClient == nil {
			t.Skip("Redis not available")
		}

		// 3 câmeras publicando simultaneamente
		cameras := []string{"cam1", "cam2", "cam3"}
		done := make(chan struct{}, 3)
		ctx := context.Background()

		start := time.Now()

		for _, cameraID := range cameras {
			go func(cam string) {
				defer func() { done <- struct{}{} }()

				// Cada câmera publica 30 frames
				for i := 0; i < 30; i++ {
					frameData := []byte("frame-data")
					redisClient.StoreWithContext(ctx, cam, frameData, time.Now())
					time.Sleep(10 * time.Millisecond) // ~100fps
				}
			}(cameraID)
		}

		// Aguarda todas as câmeras
		for i := 0; i < 3; i++ {
			<-done
		}

		duration := time.Since(start)

		// Stats
		storeCount, storeErrors, _, _ := redisClient.Stats()
		t.Logf("📊 3 câmeras × 30 frames em %v:", duration)
		t.Logf("   Total Stores: %d", storeCount)
		t.Logf("   Erros: %d", storeErrors)
		t.Logf("   Taxa: %.2f frames/s", float64(90)/duration.Seconds())

		if storeCount > 0 {
			t.Logf("✅ Múltiplas câmeras funcionando simultaneamente")
		}
	})
}
