package health

import (
	"sync"
	"time"
)

// PublishHealthMonitor monitora a saúde das operações de publish
// Detecta quando uma câmera está degradada (muitos erros de publish)
type PublishHealthMonitor struct {
	mu                sync.Mutex
	recentErrors      []time.Time // Últimos N erros
	recentSuccesses   []time.Time // Últimos N sucessos
	consecutiveFails  int
	lastSuccess       time.Time
	windowSize        int     // Janela de análise (padrão: 10)
	degradedThreshold float64 // Threshold (padrão: 0.8 = 80% erros)
}

// NewPublishHealthMonitor cria um novo monitor de saúde
func NewPublishHealthMonitor(windowSize int, degradedThreshold float64) *PublishHealthMonitor {
	if windowSize <= 0 {
		windowSize = 10 // Default: janela de 10 publishes
	}
	if degradedThreshold <= 0 || degradedThreshold > 1.0 {
		degradedThreshold = 0.8 // Default: 80% de erros = degraded
	}

	return &PublishHealthMonitor{
		recentErrors:      make([]time.Time, 0, windowSize),
		recentSuccesses:   make([]time.Time, 0, windowSize),
		consecutiveFails:  0,
		lastSuccess:       time.Now(),
		windowSize:        windowSize,
		degradedThreshold: degradedThreshold,
	}
}

// RecordError registra um erro de publish
func (p *PublishHealthMonitor) RecordError() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.recentErrors = append(p.recentErrors, time.Now())
	if len(p.recentErrors) > p.windowSize {
		p.recentErrors = p.recentErrors[1:]
	}
	p.consecutiveFails++
}

// RecordSuccess registra um sucesso de publish
func (p *PublishHealthMonitor) RecordSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.recentSuccesses = append(p.recentSuccesses, time.Now())
	if len(p.recentSuccesses) > p.windowSize {
		p.recentSuccesses = p.recentSuccesses[1:]
	}
	p.consecutiveFails = 0
	p.lastSuccess = time.Now()
}

// IsDegraded verifica se o estado de publish está degradado
// Retorna true se:
// - 80%+ de erros nos últimos N publishes
// OU
// - Sem sucesso há 30+ segundos
func (p *PublishHealthMonitor) IsDegraded() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Calcula taxa de erro nos últimos N publishes
	total := len(p.recentErrors) + len(p.recentSuccesses)
	if total < p.windowSize {
		return false // Dados insuficientes, não considera degraded
	}

	errorRate := float64(len(p.recentErrors)) / float64(total)

	// Degraded se:
	// - 80%+ de erros nos últimos N publishes
	// OU
	// - Sem sucesso há 30 segundos
	return errorRate >= p.degradedThreshold ||
		time.Since(p.lastSuccess) > 30*time.Second
}

// IsEmergency verifica se está em estado de emergência
// Retorna true se houve muitos failures consecutivos (>20)
func (p *PublishHealthMonitor) IsEmergency() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.consecutiveFails > 20
}

// GetStats retorna estatísticas atuais
func (p *PublishHealthMonitor) GetStats() HealthStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	total := len(p.recentErrors) + len(p.recentSuccesses)
	errorRate := 0.0
	if total > 0 {
		errorRate = float64(len(p.recentErrors)) / float64(total)
	}

	return HealthStats{
		TotalRecent:      total,
		RecentErrors:     len(p.recentErrors),
		RecentSuccesses:  len(p.recentSuccesses),
		ConsecutiveFails: p.consecutiveFails,
		ErrorRate:        errorRate,
		LastSuccess:      p.lastSuccess,
		IsDegraded:       p.IsDegraded(),
		IsEmergency:      p.IsEmergency(),
	}
}

// HealthStats representa estatísticas de saúde
type HealthStats struct {
	TotalRecent      int
	RecentErrors     int
	RecentSuccesses  int
	ConsecutiveFails int
	ErrorRate        float64
	LastSuccess      time.Time
	IsDegraded       bool
	IsEmergency      bool
}

// Reset limpa todas as estatísticas
func (p *PublishHealthMonitor) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.recentErrors = make([]time.Time, 0, p.windowSize)
	p.recentSuccesses = make([]time.Time, 0, p.windowSize)
	p.consecutiveFails = 0
	p.lastSuccess = time.Now()
}
