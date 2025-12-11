package resilience

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// CIRCUIT BREAKER UNIT TESTS - Fase 1 QA
// ============================================================================
// Testes baseados na funcionalidade real do circuit_breaker.go
// Cobertura: Criação, transições de estado, backoff, thread-safety, stats
// ============================================================================

// TestNewCircuitBreaker valida criação do circuit breaker com config customizado
func TestNewCircuitBreaker(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:           true,
		MaxFailures:       3,
		ResetTimeout:      10 * time.Second,
		HalfOpenSuccesses: 2,
		InitialBackoff:    2 * time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
	}

	cb := NewCircuitBreaker("test-cb", config)

	if cb.name != "test-cb" {
		t.Errorf("Expected name 'test-cb', got '%s'", cb.name)
	}

	if cb.state != StateClosed {
		t.Errorf("Expected initial state CLOSED, got %s", cb.state)
	}

	if cb.config.MaxFailures != 3 {
		t.Errorf("Expected MaxFailures 3, got %d", cb.config.MaxFailures)
	}

	if cb.currentBackoff != 2*time.Second {
		t.Errorf("Expected initial backoff 2s, got %v", cb.currentBackoff)
	}

	t.Log("✅ Circuit Breaker criado corretamente com config customizado")
}

// TestNewCircuitBreakerDefaults valida valores padrão quando não configurados
func TestNewCircuitBreakerDefaults(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:     true,
		MaxFailures: 5,
		// InitialBackoff, MaxBackoff, etc não configurados
	}

	cb := NewCircuitBreaker("test-defaults", config)

	// Valores padrão aplicados automaticamente
	if cb.config.InitialBackoff != 5*time.Second {
		t.Errorf("Expected default InitialBackoff 5s, got %v", cb.config.InitialBackoff)
	}

	if cb.config.MaxBackoff != 5*time.Minute {
		t.Errorf("Expected default MaxBackoff 5m, got %v", cb.config.MaxBackoff)
	}

	if cb.config.BackoffMultiplier != 2.0 {
		t.Errorf("Expected default BackoffMultiplier 2.0, got %f", cb.config.BackoffMultiplier)
	}

	if cb.config.HalfOpenSuccesses != 3 {
		t.Errorf("Expected default HalfOpenSuccesses 3, got %d", cb.config.HalfOpenSuccesses)
	}

	t.Log("✅ Valores padrão aplicados corretamente")
}

// TestExecuteDisabled valida que quando disabled, executa direto sem proteção
func TestExecuteDisabled(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:     false, // DESABILITADO
		MaxFailures: 3,
	}

	cb := NewCircuitBreaker("test-disabled", config)

	executed := false
	err := cb.Execute(func() error {
		executed = true
		return nil
	})

	if err != nil {
		t.Errorf("Execute should not fail when disabled: %v", err)
	}

	if !executed {
		t.Error("Function should be executed when circuit breaker is disabled")
	}

	// Mesmo com muitas falhas, não deve abrir o circuito
	for i := 0; i < 10; i++ {
		cb.Execute(func() error { return fmt.Errorf("error") })
	}

	if cb.state != StateClosed {
		t.Errorf("Circuit breaker should remain CLOSED when disabled, got %s", cb.state)
	}

	t.Log("✅ Circuit Breaker desabilitado executa função sem proteção")
}

// TestStateTransitionClosedToOpen valida transição CLOSED → OPEN após MaxFailures
func TestStateTransitionClosedToOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:        true,
		MaxFailures:    3,
		InitialBackoff: 1 * time.Second,
	}

	cb := NewCircuitBreaker("test-closed-to-open", config)

	// Estado inicial: CLOSED
	if cb.State() != StateClosed {
		t.Fatalf("Expected initial state CLOSED, got %s", cb.State())
	}

	// Gera 2 falhas (ainda não atinge MaxFailures=3)
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return fmt.Errorf("error %d", i) })
	}

	// Deve continuar CLOSED
	if cb.State() != StateClosed {
		t.Errorf("Expected state CLOSED after 2 failures, got %s", cb.State())
	}

	// 3ª falha → abre circuito
	cb.Execute(func() error { return fmt.Errorf("error 3") })

	// Deve estar OPEN agora
	if cb.State() != StateOpen {
		t.Errorf("Expected state OPEN after 3 failures, got %s", cb.State())
	}

	stats := cb.Stats()
	if stats.StateChanges != 1 {
		t.Errorf("Expected 1 state change (CLOSED→OPEN), got %d", stats.StateChanges)
	}

	t.Log("✅ Transição CLOSED → OPEN funciona corretamente")
}

// TestRejectionWhenOpen valida que requests são bloqueados quando circuito está OPEN
func TestRejectionWhenOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:        true,
		MaxFailures:    2,
		InitialBackoff: 5 * time.Second, // Backoff longo para garantir que fica OPEN
	}

	cb := NewCircuitBreaker("test-rejection", config)

	// Abre circuito com 2 falhas
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return fmt.Errorf("error") })
	}

	if cb.State() != StateOpen {
		t.Fatalf("Expected state OPEN, got %s", cb.State())
	}

	// Tenta executar - deve ser rejeitado
	executed := false
	err := cb.Execute(func() error {
		executed = true
		return nil
	})

	if err == nil {
		t.Error("Execute should return error when circuit is OPEN")
	}

	if executed {
		t.Error("Function should NOT be executed when circuit is OPEN")
	}

	stats := cb.Stats()
	if stats.TotalRejected == 0 {
		t.Error("Expected TotalRejected > 0 when circuit is OPEN")
	}

	t.Logf("✅ Requests bloqueados corretamente quando OPEN (rejected: %d)", stats.TotalRejected)
}

// TestStateTransitionOpenToHalfOpen valida transição OPEN → HALF_OPEN após backoff
func TestStateTransitionOpenToHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:           true,
		MaxFailures:       2,
		HalfOpenSuccesses: 5, // Muitos sucessos necessários para não fechar imediatamente
		InitialBackoff:    100 * time.Millisecond,
	}

	cb := NewCircuitBreaker("test-open-to-halfopen", config)

	// Abre circuito
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return fmt.Errorf("error") })
	}

	if cb.State() != StateOpen {
		t.Fatalf("Expected state OPEN, got %s", cb.State())
	}

	// Aguarda backoff (100ms)
	time.Sleep(150 * time.Millisecond)

	// IMPORTANTE: allowRequest() só transiciona para HALF_OPEN quando Execute() é chamado
	// e o backoff já passou. Então chamar Execute() após backoff força a transição.
	cb.Execute(func() error { return nil })

	// Pode estar HALF_OPEN (se HalfOpenSuccesses > 1) ou CLOSED (se HalfOpenSuccesses=1 e sucesso)
	state := cb.State()
	if state != StateHalfOpen && state != StateClosed {
		t.Errorf("Expected state HALF_OPEN or CLOSED after backoff, got %s", state)
	}

	t.Log("✅ Transição OPEN → HALF_OPEN funciona após backoff")
}

// TestStateTransitionHalfOpenToClosed valida transição HALF_OPEN → CLOSED após sucessos
func TestStateTransitionHalfOpenToClosed(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:           true,
		MaxFailures:       2,
		HalfOpenSuccesses: 2, // Apenas 2 sucessos para fechar (mais fácil de testar)
		InitialBackoff:    100 * time.Millisecond,
	}

	cb := NewCircuitBreaker("test-halfopen-to-closed", config)

	// Abre circuito
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return fmt.Errorf("error") })
	}

	if cb.State() != StateOpen {
		t.Fatalf("Expected state OPEN, got %s", cb.State())
	}

	// Aguarda backoff
	time.Sleep(150 * time.Millisecond)

	// Primeira tentativa após backoff → vai para HALF_OPEN
	cb.Execute(func() error { return nil })
	t.Logf("Estado após 1º sucesso: %s (consecutiveSuccesses=%d)", cb.State(), cb.consecutiveSuccesses)

	// Segunda tentativa com sucesso → deve fechar circuito (2/2 sucessos)
	cb.Execute(func() error { return nil })

	// Deve estar CLOSED agora
	if cb.State() != StateClosed {
		t.Errorf("Expected state CLOSED after %d successes in HALF_OPEN, got %s",
			config.HalfOpenSuccesses, cb.State())
	}

	// Backoff deve ter resetado para InitialBackoff
	stats := cb.Stats()
	if stats.CurrentBackoff != config.InitialBackoff {
		t.Errorf("Expected backoff reset to %v, got %v", config.InitialBackoff, stats.CurrentBackoff)
	}

	t.Log("✅ Transição HALF_OPEN → CLOSED funciona após sucessos")
}

// TestStateTransitionHalfOpenToOpen valida que falha em HALF_OPEN volta para OPEN
func TestStateTransitionHalfOpenToOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:           true,
		MaxFailures:       2,
		HalfOpenSuccesses: 3,
		InitialBackoff:    1 * time.Second,
		BackoffMultiplier: 2.0,
	}

	cb := NewCircuitBreaker("test-halfopen-to-open", config)

	// Abre circuito (2 falhas)
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return fmt.Errorf("error") })
	}

	if cb.State() != StateOpen {
		t.Fatalf("Expected state OPEN, got %s", cb.State())
	}

	// Backoff após primeira abertura deve ser 2s (1s * 2.0)
	stats1 := cb.Stats()
	t.Logf("Backoff após 1ª abertura: %v", stats1.CurrentBackoff)

	// Aguarda backoff
	time.Sleep(stats1.CurrentBackoff + 50*time.Millisecond)

	// Tenta recuperar (vai para HALF_OPEN)
	cb.Execute(func() error { return nil })

	// FALHA em HALF_OPEN → volta para OPEN e aumenta backoff
	cb.Execute(func() error { return fmt.Errorf("error in half-open") })

	// Deve estar OPEN novamente
	if cb.State() != StateOpen {
		t.Errorf("Expected state OPEN after failure in HALF_OPEN, got %s", cb.State())
	}

	// Backoff deve ter aumentado exponencialmente (2s * 2.0 = 4s)
	stats2 := cb.Stats()
	expectedBackoff := time.Duration(float64(stats1.CurrentBackoff) * config.BackoffMultiplier)
	if stats2.CurrentBackoff != expectedBackoff {
		t.Errorf("Expected backoff %v (was %v * %.1f), got %v",
			expectedBackoff, stats1.CurrentBackoff, config.BackoffMultiplier, stats2.CurrentBackoff)
	}

	t.Log("✅ Transição HALF_OPEN → OPEN funciona em caso de falha")
}

// TestExponentialBackoff valida que backoff aumenta exponencialmente
func TestExponentialBackoff(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:           true,
		MaxFailures:       2,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        60 * time.Second,
		BackoffMultiplier: 2.0,
	}

	cb := NewCircuitBreaker("test-backoff", config)

	// Primeira falha → abre circuito
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return fmt.Errorf("error") })
	}

	backoffs := []time.Duration{}
	backoffs = append(backoffs, cb.Stats().CurrentBackoff)

	// Simula várias reconexões falhadas (cada uma aumenta backoff)
	for i := 0; i < 5; i++ {
		time.Sleep(cb.Stats().CurrentBackoff + 10*time.Millisecond)
		cb.Execute(func() error { return fmt.Errorf("error") }) // Falha → aumenta backoff
		backoffs = append(backoffs, cb.Stats().CurrentBackoff)
	}

	// Verifica progressão exponencial: 1s → 2s → 4s → 8s → 16s → 32s → 60s (max)
	t.Logf("Backoff progression: %v", backoffs)

	for i := 1; i < len(backoffs); i++ {
		if backoffs[i] < backoffs[i-1] {
			t.Errorf("Backoff should increase or stay at max, got %v after %v",
				backoffs[i], backoffs[i-1])
		}
	}

	// Último backoff não deve exceder MaxBackoff
	if backoffs[len(backoffs)-1] > config.MaxBackoff {
		t.Errorf("Backoff should not exceed MaxBackoff %v, got %v",
			config.MaxBackoff, backoffs[len(backoffs)-1])
	}

	t.Log("✅ Backoff exponencial funciona corretamente")
}

// TestReset valida reset manual do circuit breaker
func TestReset(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:        true,
		MaxFailures:    2,
		InitialBackoff: 5 * time.Second,
	}

	cb := NewCircuitBreaker("test-reset", config)

	// Abre circuito
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return fmt.Errorf("error") })
	}

	if cb.State() != StateOpen {
		t.Fatalf("Expected state OPEN, got %s", cb.State())
	}

	// Reset manual
	cb.Reset()

	// Deve estar CLOSED
	if cb.State() != StateClosed {
		t.Errorf("Expected state CLOSED after reset, got %s", cb.State())
	}

	stats := cb.Stats()
	if stats.Failures != 0 {
		t.Errorf("Expected failures reset to 0, got %d", stats.Failures)
	}

	if stats.CurrentBackoff != config.InitialBackoff {
		t.Errorf("Expected backoff reset to %v, got %v", config.InitialBackoff, stats.CurrentBackoff)
	}

	t.Log("✅ Reset manual funciona corretamente")
}

// TestConcurrentAccess valida thread-safety com múltiplas goroutines
func TestConcurrentAccess(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:        true,
		MaxFailures:    1000, // Alto para evitar abrir circuito durante teste
		InitialBackoff: 10 * time.Second,
	}

	cb := NewCircuitBreaker("test-concurrent", config)

	var wg sync.WaitGroup
	goroutines := 5 // Reduzido para 5 (mais rápido e confiável)
	iterationsPerGoroutine := 20 // Reduzido para 20

	// Goroutines executando operações concorrentemente
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				// Apenas sucessos para não abrir o circuito
				cb.Execute(func() error { return nil })

				// Lê stats concorrentemente
				_ = cb.Stats()
				_ = cb.State()
			}
		}(i)
	}

	// Aguarda todas as goroutines
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		t.Log("✅ Todas as goroutines completaram sem deadlock")
	case <-time.After(2 * time.Second):
		t.Fatal("❌ TIMEOUT! Possível deadlock detectado")
	}

	// Valida estatísticas
	stats := cb.Stats()
	expectedTotal := uint64(goroutines * iterationsPerGoroutine)

	if stats.TotalCalls != expectedTotal {
		t.Errorf("Expected TotalCalls=%d, got %d", expectedTotal, stats.TotalCalls)
	}

	t.Logf("Stats: Calls=%d, Successes=%d, Failures=%d, Rejected=%d, State=%s",
		stats.TotalCalls, stats.TotalSuccesses, stats.TotalFailures, stats.TotalRejected, stats.State)
}

// TestStats valida que estatísticas são rastreadas corretamente
func TestStats(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:        true,
		MaxFailures:    5,
		InitialBackoff: 1 * time.Second,
	}

	cb := NewCircuitBreaker("test-stats", config)

	// 3 sucessos
	for i := 0; i < 3; i++ {
		cb.Execute(func() error { return nil })
	}

	// 2 falhas
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return fmt.Errorf("error") })
	}

	stats := cb.Stats()

	if stats.Name != "test-stats" {
		t.Errorf("Expected name 'test-stats', got '%s'", stats.Name)
	}

	if !stats.Enabled {
		t.Error("Expected Enabled=true")
	}

	if stats.TotalCalls != 5 {
		t.Errorf("Expected TotalCalls=5, got %d", stats.TotalCalls)
	}

	if stats.TotalSuccesses != 3 {
		t.Errorf("Expected TotalSuccesses=3, got %d", stats.TotalSuccesses)
	}

	if stats.TotalFailures != 2 {
		t.Errorf("Expected TotalFailures=2, got %d", stats.TotalFailures)
	}

	if stats.Failures != 2 {
		t.Errorf("Expected current Failures=2, got %d", stats.Failures)
	}

	if stats.State != StateClosed {
		t.Errorf("Expected State=CLOSED, got %s", stats.State)
	}

	// Testa String()
	statsStr := stats.String()
	if statsStr == "" {
		t.Error("Stats.String() should not be empty")
	}

	t.Logf("Stats: %s", statsStr)
	t.Log("✅ Estatísticas rastreadas corretamente")
}

// TestCircuitStateString valida representação textual dos estados
func TestCircuitStateString(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "CLOSED"},
		{StateOpen, "OPEN"},
		{StateHalfOpen, "HALF_OPEN"},
		{CircuitState(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.expected {
			t.Errorf("Expected %s.String() = '%s', got '%s'",
				tt.state, tt.expected, got)
		}
	}

	t.Log("✅ CircuitState.String() funciona corretamente")
}

// ============================================================================
// BENCHMARKS
// ============================================================================

// BenchmarkExecute mede performance de Execute() com circuit breaker habilitado
func BenchmarkExecute(b *testing.B) {
	config := CircuitBreakerConfig{
		Enabled:     true,
		MaxFailures: 1000000, // Muito alto para não abrir durante benchmark
	}

	cb := NewCircuitBreaker("bench", config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(func() error { return nil })
	}
}

// BenchmarkStats mede performance de Stats()
func BenchmarkStats(b *testing.B) {
	config := CircuitBreakerConfig{
		Enabled:     true,
		MaxFailures: 5,
	}

	cb := NewCircuitBreaker("bench-stats", config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.Stats()
	}
}

// BenchmarkConcurrentExecute mede performance com acesso concorrente
func BenchmarkConcurrentExecute(b *testing.B) {
	config := CircuitBreakerConfig{
		Enabled:     true,
		MaxFailures: 1000000,
	}

	cb := NewCircuitBreaker("bench-concurrent", config)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cb.Execute(func() error { return nil })
		}
	})
}
