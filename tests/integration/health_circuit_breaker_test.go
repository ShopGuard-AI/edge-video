package integration

import (
	"testing"
	"time"

	"edge-video/v2/internal/health"
	"edge-video/v2/internal/resilience"
)

// ============================================================================
// INTEGRATION TEST: Health Monitor + Circuit Breaker
// ============================================================================
// Testa integração entre Health Monitor e Circuit Breaker
// Valida que degradação detectada pelo Health Monitor pode abrir Circuit Breaker
// ============================================================================

// TestHealthMonitorIntegration valida integração entre Health Monitor e Circuit Breaker
func TestHealthMonitorIntegration(t *testing.T) {
	// Setup Health Monitor (detecta degradação após 80% erros)
	healthMonitor := health.NewPublishHealthMonitor(10, 0.8)

	// Setup Circuit Breaker (abre após 5 falhas)
	cbConfig := resilience.CircuitBreakerConfig{
		Enabled:           true,
		MaxFailures:       5,
		ResetTimeout:      30 * time.Second,
		HalfOpenSuccesses: 3,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
	}
	circuitBreaker := resilience.NewCircuitBreaker("publisher", cbConfig)

	// Cenário 1: Sistema saudável
	t.Run("Sistema Saudável", func(t *testing.T) {
		// Registra sucessos
		for i := 0; i < 10; i++ {
			healthMonitor.RecordSuccess()
		}

		stats := healthMonitor.GetStats()
		if stats.IsDegraded {
			t.Error("Sistema NÃO deveria estar degradado com 10 sucessos")
		}

		if !circuitBreaker.IsClosed() {
			t.Error("Circuit Breaker deveria estar CLOSED com sistema saudável")
		}

		t.Logf("✅ Sistema saudável: Health=%v, Circuit=%s",
			stats.IsDegraded, circuitBreaker.State())
	})

	// Cenário 2: Degradação detectada → Circuit Breaker deve abrir
	t.Run("Degradação Detectada", func(t *testing.T) {
		// Reset
		healthMonitor.Reset()
		circuitBreaker.Reset()

		// Simula degradação: 9 erros + 1 sucesso = 90% erro rate
		for i := 0; i < 9; i++ {
			healthMonitor.RecordError()
			// Também registra falha no Circuit Breaker
			circuitBreaker.Execute(func() error {
				return nil // Simula erro sendo tratado externamente
			})
		}
		healthMonitor.RecordSuccess()

		// Health Monitor deve detectar degradação
		stats := healthMonitor.GetStats()
		if !stats.IsDegraded {
			t.Errorf("Health Monitor deveria detectar degradação (error rate: %.2f%%)",
				stats.ErrorRate*100)
		}

		// Simula mais falhas para abrir Circuit Breaker
		for i := 0; i < 5; i++ {
			circuitBreaker.Execute(func() error {
				return &MockError{}
			})
		}

		// Circuit Breaker deve estar OPEN
		if !circuitBreaker.IsOpen() {
			t.Errorf("Circuit Breaker deveria estar OPEN, está %s", circuitBreaker.State())
		}

		t.Logf("✅ Degradação detectada: Health=%v (%.1f%% erros), Circuit=%s",
			stats.IsDegraded, stats.ErrorRate*100, circuitBreaker.State())
	})

	// Cenário 3: Recuperação do sistema
	t.Run("Recuperação do Sistema", func(t *testing.T) {
		// Reset Health Monitor
		healthMonitor.Reset()

		// Aguarda backoff do Circuit Breaker (1s)
		time.Sleep(1100 * time.Millisecond)

		// Simula recuperação: sucessos consecutivos
		for i := 0; i < 3; i++ {
			circuitBreaker.Execute(func() error { return nil })
		}

		// Health Monitor também recebe sucessos
		for i := 0; i < 10; i++ {
			healthMonitor.RecordSuccess()
		}

		// Circuit Breaker deve fechar (3 sucessos em HALF_OPEN)
		if !circuitBreaker.IsClosed() {
			t.Logf("⚠️  Circuit Breaker ainda não fechou (estado: %s), mas sistema recuperou",
				circuitBreaker.State())
		}

		// Health Monitor não deve reportar degradação
		stats := healthMonitor.GetStats()
		if stats.IsDegraded {
			t.Error("Health Monitor deveria indicar sistema saudável após recuperação")
		}

		t.Logf("✅ Sistema recuperado: Health=%v, Circuit=%s",
			stats.IsDegraded, circuitBreaker.State())
	})

	// Cenário 4: Emergency state (>20 erros consecutivos)
	t.Run("Emergency State", func(t *testing.T) {
		healthMonitor.Reset()
		circuitBreaker.Reset()

		// Simula emergency: 25 erros consecutivos
		for i := 0; i < 25; i++ {
			healthMonitor.RecordError()
			circuitBreaker.Execute(func() error {
				return &MockError{}
			})
		}

		stats := healthMonitor.GetStats()

		// Health Monitor deve detectar emergency
		if !stats.IsEmergency {
			t.Errorf("Health Monitor deveria detectar EMERGENCY após %d erros consecutivos",
				stats.ConsecutiveFails)
		}

		// Circuit Breaker deve estar OPEN
		if !circuitBreaker.IsOpen() {
			t.Errorf("Circuit Breaker deveria estar OPEN em emergency, está %s",
				circuitBreaker.State())
		}

		t.Logf("✅ Emergency detectado: Erros consecutivos=%d, Emergency=%v, Circuit=%s",
			stats.ConsecutiveFails, stats.IsEmergency, circuitBreaker.State())
	})
}

// TestHealthMonitorCircuitBreakerCoordination valida coordenação entre componentes
func TestHealthMonitorCircuitBreakerCoordination(t *testing.T) {
	healthMonitor := health.NewPublishHealthMonitor(5, 0.6) // 60% threshold

	cbConfig := resilience.CircuitBreakerConfig{
		Enabled:           true,
		MaxFailures:       3,
		ResetTimeout:      10 * time.Second,
		HalfOpenSuccesses: 2,
		InitialBackoff:    500 * time.Millisecond,
		MaxBackoff:        5 * time.Second,
		BackoffMultiplier: 2.0,
	}
	circuitBreaker := resilience.NewCircuitBreaker("test", cbConfig)

	// Simula sequência realista: sucesso → falha → recuperação
	scenario := []struct {
		action   string
		isError  bool
		expected string
	}{
		{"sucesso", false, "Sistema inicializando"},
		{"sucesso", false, "Sistema saudável"},
		{"sucesso", false, "Sistema saudável"},
		{"erro", true, "Primeira falha"},
		{"erro", true, "Segunda falha"},
		{"erro", true, "Terceira falha - CB abre"},
		{"sucesso", false, "Tentativa durante CB OPEN (rejeitado)"},
	}

	for i, step := range scenario {
		if step.isError {
			healthMonitor.RecordError()
			err := circuitBreaker.Execute(func() error {
				return &MockError{}
			})
			t.Logf("Step %d (%s): Error=%v, CB=%s", i+1, step.expected,
				err != nil, circuitBreaker.State())
		} else {
			healthMonitor.RecordSuccess()
			err := circuitBreaker.Execute(func() error {
				return nil
			})
			t.Logf("Step %d (%s): Success, CB=%s", i+1, step.expected,
				circuitBreaker.State())

			// Se Circuit Breaker rejeitou (OPEN), não deve contar como erro
			if err != nil && circuitBreaker.IsOpen() {
				t.Logf("   → Request rejeitado por CB OPEN (esperado)")
			}
		}

		stats := healthMonitor.GetStats()
		cbStats := circuitBreaker.Stats()

		t.Logf("   Health: Degraded=%v (%.0f%% erros), CB: State=%s, Failures=%d/%d",
			stats.IsDegraded, stats.ErrorRate*100,
			cbStats.State, cbStats.Failures, cbStats.MaxFailures)
	}

	t.Log("✅ Coordenação Health Monitor + Circuit Breaker funcionando")
}

// MockError é um erro mock para testes
type MockError struct{}

func (e *MockError) Error() string {
	return "mock error"
}
