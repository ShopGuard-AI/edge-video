package main

import (
	"fmt"
	"sync"
	"time"
)

// State representa o estado do circuit breaker
type CircuitState int

const (
	// StateClosed - circuito fechado (normal, permite chamadas)
	StateClosed CircuitState = iota
	// StateOpen - circuito aberto (bloqueando chamadas, em backoff)
	StateOpen
	// StateHalfOpen - circuito semi-aberto (tentando recuperar)
	StateHalfOpen
)

// String retorna representação textual do estado
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig configuração do circuit breaker
type CircuitBreakerConfig struct {
	Enabled           bool          `yaml:"enabled"`             // Se está habilitado
	MaxFailures       int           `yaml:"max_failures"`        // Falhas consecutivas antes de abrir
	ResetTimeout      time.Duration `yaml:"reset_timeout"`       // Tempo antes de tentar HALF_OPEN
	HalfOpenSuccesses int           `yaml:"half_open_successes"` // Sucessos necessários em HALF_OPEN para fechar
	InitialBackoff    time.Duration `yaml:"initial_backoff"`     // Backoff inicial
	MaxBackoff        time.Duration `yaml:"max_backoff"`         // Backoff máximo
	BackoffMultiplier float64       `yaml:"backoff_multiplier"`  // Multiplicador do backoff
}

// DefaultCircuitBreakerConfig retorna configuração padrão (conservadora)
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Enabled:           false, // Desabilitado por padrão
		MaxFailures:       5,
		ResetTimeout:      30 * time.Second,
		HalfOpenSuccesses: 3,
		InitialBackoff:    5 * time.Second,
		MaxBackoff:        5 * time.Minute,
		BackoffMultiplier: 2.0,
	}
}

// CircuitBreaker implementa padrão Circuit Breaker com backoff exponencial
type CircuitBreaker struct {
	name   string
	config CircuitBreakerConfig

	mu                sync.RWMutex
	state             CircuitState
	failures          int
	consecutiveSuccesses int
	lastFailureTime   time.Time
	lastStateChange   time.Time
	currentBackoff    time.Duration

	// Estatísticas
	totalCalls    uint64
	totalFailures uint64
	totalSuccesses uint64
	totalRejected uint64
	stateChanges  uint64
}

// NewCircuitBreaker cria novo circuit breaker
func NewCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	// Se backoff não configurado, usa valores padrão
	if config.InitialBackoff == 0 {
		config.InitialBackoff = 5 * time.Second
	}
	if config.MaxBackoff == 0 {
		config.MaxBackoff = 5 * time.Minute
	}
	if config.BackoffMultiplier == 0 {
		config.BackoffMultiplier = 2.0
	}
	if config.HalfOpenSuccesses == 0 {
		config.HalfOpenSuccesses = 3
	}

	return &CircuitBreaker{
		name:            name,
		config:          config,
		state:           StateClosed,
		currentBackoff:  config.InitialBackoff,
		lastStateChange: time.Now(),
	}
}

// Execute executa função com proteção do circuit breaker
func (cb *CircuitBreaker) Execute(fn func() error) error {
	// Se circuit breaker desabilitado, executa direto
	if !cb.config.Enabled {
		return fn()
	}

	// Verifica se pode executar
	if !cb.allowRequest() {
		cb.mu.Lock()
		cb.totalRejected++
		cb.mu.Unlock()
		return fmt.Errorf("circuit breaker [%s] is %s - próxima tentativa em %v",
			cb.name, cb.state, cb.timeUntilRetry())
	}

	// Executa função
	cb.mu.Lock()
	cb.totalCalls++
	cb.mu.Unlock()

	err := fn()

	// Registra resultado
	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

// allowRequest verifica se request pode ser executado
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		// Estado normal, permite tudo
		return true

	case StateOpen:
		// Circuito aberto, verifica se já passou o backoff
		if time.Since(cb.lastFailureTime) >= cb.currentBackoff {
			// Tempo de tentar novamente (vai para HALF_OPEN)
			cb.mu.RUnlock()
			cb.mu.Lock()
			if cb.state == StateOpen { // Double-check
				cb.transitionTo(StateHalfOpen)
			}
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false

	case StateHalfOpen:
		// Em HALF_OPEN, permite requests limitados
		return true

	default:
		return false
	}
}

// onSuccess registra sucesso
func (cb *CircuitBreaker) onSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalSuccesses++

	switch cb.state {
	case StateClosed:
		// Normal, reseta contador de falhas
		cb.failures = 0
		cb.consecutiveSuccesses++

	case StateHalfOpen:
		// Sucesso em HALF_OPEN, conta para fechar
		cb.consecutiveSuccesses++
		cb.failures = 0

		if cb.consecutiveSuccesses >= cb.config.HalfOpenSuccesses {
			// Sucessos suficientes, fecha circuito!
			cb.transitionTo(StateClosed)
			cb.consecutiveSuccesses = 0
			cb.currentBackoff = cb.config.InitialBackoff // Reset backoff
		}
	}
}

// onFailure registra falha
func (cb *CircuitBreaker) onFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalFailures++
	cb.failures++
	cb.consecutiveSuccesses = 0
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		// Verifica se atingiu limite de falhas
		if cb.failures >= cb.config.MaxFailures {
			// Abre circuito!
			cb.transitionTo(StateOpen)
			cb.increaseBackoff()
		}

	case StateHalfOpen:
		// Falha em HALF_OPEN, volta para OPEN
		cb.transitionTo(StateOpen)
		cb.increaseBackoff()
	}
}

// transitionTo muda estado do circuito
func (cb *CircuitBreaker) transitionTo(newState CircuitState) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()
	cb.stateChanges++

	// Log de mudança de estado
	switch newState {
	case StateOpen:
		fmt.Printf("🔴 Circuit Breaker [%s]: %s → %s (falhas: %d, backoff: %v)\n",
			cb.name, oldState, newState, cb.failures, cb.currentBackoff)
	case StateHalfOpen:
		fmt.Printf("🟡 Circuit Breaker [%s]: %s → %s (tentando recuperar...)\n",
			cb.name, oldState, newState)
	case StateClosed:
		fmt.Printf("🟢 Circuit Breaker [%s]: %s → %s (recuperado! backoff resetado)\n",
			cb.name, oldState, newState)
	}

	// Atualiza métricas Prometheus (se metricsServer disponível)
	if metricsServer != nil {
		metricsServer.UpdateCircuitBreakerState(cb.name, newState)
	}
}

// increaseBackoff aumenta backoff exponencialmente
func (cb *CircuitBreaker) increaseBackoff() {
	cb.currentBackoff = time.Duration(float64(cb.currentBackoff) * cb.config.BackoffMultiplier)
	if cb.currentBackoff > cb.config.MaxBackoff {
		cb.currentBackoff = cb.config.MaxBackoff
	}
}

// timeUntilRetry retorna tempo até próxima tentativa
func (cb *CircuitBreaker) timeUntilRetry() time.Duration {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state != StateOpen {
		return 0
	}

	elapsed := time.Since(cb.lastFailureTime)
	remaining := cb.currentBackoff - elapsed

	if remaining < 0 {
		return 0
	}

	return remaining
}

// State retorna estado atual
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// IsOpen retorna se circuito está aberto
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.State() == StateOpen
}

// IsClosed retorna se circuito está fechado
func (cb *CircuitBreaker) IsClosed() bool {
	return cb.State() == StateClosed
}

// Reset força reset do circuit breaker para estado CLOSED
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.consecutiveSuccesses = 0
	cb.currentBackoff = cb.config.InitialBackoff
	cb.lastStateChange = time.Now()

	fmt.Printf("🔄 Circuit Breaker [%s]: RESET manual para CLOSED\n", cb.name)
}

// Stats retorna estatísticas do circuit breaker
type CircuitBreakerStats struct {
	Name                 string
	Enabled              bool
	State                CircuitState
	Failures             int
	ConsecutiveSuccesses int
	CurrentBackoff       time.Duration
	TimeUntilRetry       time.Duration
	LastFailureTime      time.Time
	LastStateChange      time.Time

	// Estatísticas totais
	TotalCalls     uint64
	TotalSuccesses uint64
	TotalFailures  uint64
	TotalRejected  uint64
	StateChanges   uint64

	// Configuração
	MaxFailures       int
	HalfOpenSuccesses int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
}

// Stats retorna estatísticas completas
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		Name:                 cb.name,
		Enabled:              cb.config.Enabled,
		State:                cb.state,
		Failures:             cb.failures,
		ConsecutiveSuccesses: cb.consecutiveSuccesses,
		CurrentBackoff:       cb.currentBackoff,
		TimeUntilRetry:       cb.timeUntilRetry(),
		LastFailureTime:      cb.lastFailureTime,
		LastStateChange:      cb.lastStateChange,
		TotalCalls:           cb.totalCalls,
		TotalSuccesses:       cb.totalSuccesses,
		TotalFailures:        cb.totalFailures,
		TotalRejected:        cb.totalRejected,
		StateChanges:         cb.stateChanges,
		MaxFailures:          cb.config.MaxFailures,
		HalfOpenSuccesses:    cb.config.HalfOpenSuccesses,
		InitialBackoff:       cb.config.InitialBackoff,
		MaxBackoff:           cb.config.MaxBackoff,
	}
}

// String retorna representação textual das estatísticas
func (stats CircuitBreakerStats) String() string {
	if !stats.Enabled {
		return fmt.Sprintf("CircuitBreaker[%s]: DISABLED", stats.Name)
	}

	return fmt.Sprintf("CircuitBreaker[%s]: %s | Failures: %d/%d | Successes: %d | "+
		"Backoff: %v | Calls: %d (✓%d ✗%d 🚫%d) | Changes: %d",
		stats.Name,
		stats.State,
		stats.Failures,
		stats.MaxFailures,
		stats.ConsecutiveSuccesses,
		stats.CurrentBackoff,
		stats.TotalCalls,
		stats.TotalSuccesses,
		stats.TotalFailures,
		stats.TotalRejected,
		stats.StateChanges,
	)
}
