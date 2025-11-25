package memcontrol

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/T3-Labs/edge-video/pkg/logger"
)

type MemoryLevel int

const (
	MemoryNormal MemoryLevel = iota
	MemoryWarning
	MemoryCritical
	MemoryEmergency
)

func (ml MemoryLevel) String() string {
	switch ml {
	case MemoryNormal:
		return "NORMAL"
	case MemoryWarning:
		return "WARNING"
	case MemoryCritical:
		return "CRITICAL"
	case MemoryEmergency:
		return "EMERGENCY"
	default:
		return "UNKNOWN"
	}
}

type MemoryStats struct {
	Alloc         uint64
	TotalAlloc    uint64
	Sys           uint64
	NumGC         uint32
	HeapAlloc     uint64
	HeapSys       uint64
	HeapInuse     uint64
	StackInuse    uint64
	UsagePercent  float64
	Level         MemoryLevel
	Timestamp     time.Time
}

type ThresholdConfig struct {
	MaxMemoryMB      uint64
	WarningPercent   float64
	CriticalPercent  float64
	EmergencyPercent float64
	CheckInterval    time.Duration
	GCTriggerPercent float64
}

type Controller struct {
	mu              sync.RWMutex
	config          ThresholdConfig
	currentLevel    MemoryLevel
	stats           MemoryStats
	callbacks       map[MemoryLevel][]func(MemoryStats)
	gcInProgress    bool
	lastGC          time.Time
	lastLevelChange time.Time
	ctx             context.Context
	cancel          context.CancelFunc
	throttleMap     map[string]*ThrottleState
	throttleMu      sync.Mutex
}

type ThrottleState struct {
	CurrentDelay time.Duration
	LastUpdate   time.Time
	Paused       bool
}

func NewController(maxMemoryMB uint64) *Controller {
	if maxMemoryMB == 0 {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		systemMem := memStats.Sys / 1024 / 1024
		maxMemoryMB = uint64(float64(systemMem) * 0.75)

		if maxMemoryMB < 512 {
			maxMemoryMB = 512
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	config := ThresholdConfig{
		MaxMemoryMB:      maxMemoryMB,
		WarningPercent:   60.0,
		CriticalPercent:  75.0,
		EmergencyPercent: 85.0,
		CheckInterval:    2 * time.Second,
		GCTriggerPercent: 70.0,
	}

	c := &Controller{
		config:          config,
		currentLevel:    MemoryNormal,
		callbacks:       make(map[MemoryLevel][]func(MemoryStats)),
		lastGC:          time.Now(),
		lastLevelChange: time.Now(),
		ctx:             ctx,
		cancel:          cancel,
		throttleMap:     make(map[string]*ThrottleState),
	}

	if logger.Log != nil {
		logger.Log.Infow("Memory Controller inicializado",
			"max_memory_mb", maxMemoryMB,
			"warning_percent", config.WarningPercent,
			"critical_percent", config.CriticalPercent,
			"emergency_percent", config.EmergencyPercent)
	}

	return c
}

func (c *Controller) Start() {
	go c.monitorLoop()
	if logger.Log != nil {
		logger.Log.Info("Memory Controller iniciado")
	}
}

func (c *Controller) Stop() {
	c.cancel()
	if logger.Log != nil {
		logger.Log.Info("Memory Controller parado")
	}
}

func (c *Controller) monitorLoop() {
	ticker := time.NewTicker(c.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.updateStats()
			c.checkAndAct()
		}
	}
}

func (c *Controller) updateStats() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	allocMB := memStats.Alloc / 1024 / 1024
	usagePercent := (float64(allocMB) / float64(c.config.MaxMemoryMB)) * 100

	c.mu.Lock()
	c.stats = MemoryStats{
		Alloc:        memStats.Alloc,
		TotalAlloc:   memStats.TotalAlloc,
		Sys:          memStats.Sys,
		NumGC:        memStats.NumGC,
		HeapAlloc:    memStats.HeapAlloc,
		HeapSys:      memStats.HeapSys,
		HeapInuse:    memStats.HeapInuse,
		StackInuse:   memStats.StackInuse,
		UsagePercent: usagePercent,
		Level:        c.determineLevel(usagePercent),
		Timestamp:    time.Now(),
	}
	c.mu.Unlock()
}

func (c *Controller) determineLevel(usagePercent float64) MemoryLevel {
	switch {
	case usagePercent >= c.config.EmergencyPercent:
		return MemoryEmergency
	case usagePercent >= c.config.CriticalPercent:
		return MemoryCritical
	case usagePercent >= c.config.WarningPercent:
		return MemoryWarning
	default:
		return MemoryNormal
	}
}

func (c *Controller) checkAndAct() {
	c.mu.Lock()
	stats := c.stats
	oldLevel := c.currentLevel
	newLevel := stats.Level
	c.mu.Unlock()

	if newLevel != oldLevel {
		c.onLevelChange(oldLevel, newLevel, stats)
	}

	switch newLevel {
	case MemoryWarning:
		c.handleWarning(stats)
	case MemoryCritical:
		c.handleCritical(stats)
	case MemoryEmergency:
		c.handleEmergency(stats)
	}
}

func (c *Controller) onLevelChange(old, new MemoryLevel, stats MemoryStats) {
	c.mu.Lock()
	c.currentLevel = new
	c.lastLevelChange = time.Now()
	c.mu.Unlock()

	if logger.Log != nil {
		logger.Log.Warnw("Nível de memória alterado",
			"old_level", old,
			"new_level", new,
			"usage_percent", fmt.Sprintf("%.2f%%", stats.UsagePercent),
			"alloc_mb", stats.Alloc/1024/1024,
			"heap_mb", stats.HeapAlloc/1024/1024)
	}

	c.notifyCallbacks(new, stats)
}

func (c *Controller) handleWarning(stats MemoryStats) {
	if c.shouldTriggerGC(stats) {
		c.triggerGC("warning level")
	}
}

func (c *Controller) handleCritical(stats MemoryStats) {
	if logger.Log != nil {
		logger.Log.Warnw("Memória em nível CRÍTICO - ativando medidas de contenção",
			"usage_percent", fmt.Sprintf("%.2f%%", stats.UsagePercent),
			"alloc_mb", stats.Alloc/1024/1024)
	}

	c.triggerGC("critical level")
	debug.FreeOSMemory()
}

func (c *Controller) handleEmergency(stats MemoryStats) {
	if logger.Log != nil {
		logger.Log.Errorw("Memória em nível EMERGÊNCIA - sistema em risco de travamento",
			"usage_percent", fmt.Sprintf("%.2f%%", stats.UsagePercent),
			"alloc_mb", stats.Alloc/1024/1024,
			"heap_mb", stats.HeapAlloc/1024/1024)
	}

	c.triggerGC("emergency level")
	debug.FreeOSMemory()

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	debug.FreeOSMemory()
}

func (c *Controller) shouldTriggerGC(stats MemoryStats) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.gcInProgress {
		return false
	}

	if time.Since(c.lastGC) < 5*time.Second {
		return false
	}

	return stats.UsagePercent >= c.config.GCTriggerPercent
}

func (c *Controller) triggerGC(reason string) {
	c.mu.Lock()
	if c.gcInProgress {
		c.mu.Unlock()
		return
	}
	c.gcInProgress = true
	c.lastGC = time.Now()
	c.mu.Unlock()

	go func() {
		defer func() {
			c.mu.Lock()
			c.gcInProgress = false
			c.mu.Unlock()
		}()

		if logger.Log != nil {
			logger.Log.Infow("Forçando coleta de lixo", "reason", reason)
		}
		start := time.Now()
		runtime.GC()
		duration := time.Since(start)
		if logger.Log != nil {
			logger.Log.Infow("Coleta de lixo concluída", "duration", duration)
		}
	}()
}

func (c *Controller) GetStats() MemoryStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

func (c *Controller) GetLevel() MemoryLevel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentLevel
}

func (c *Controller) ShouldThrottle() bool {
	level := c.GetLevel()
	return level >= MemoryCritical
}

func (c *Controller) ShouldPause() bool {
	level := c.GetLevel()
	return level >= MemoryEmergency
}

func (c *Controller) GetThrottleDelay(cameraID string) time.Duration {
	c.throttleMu.Lock()
	defer c.throttleMu.Unlock()

	state, exists := c.throttleMap[cameraID]
	if !exists {
		state = &ThrottleState{
			CurrentDelay: 0,
			LastUpdate:   time.Now(),
			Paused:       false,
		}
		c.throttleMap[cameraID] = state
	}

	level := c.GetLevel()

	switch level {
	case MemoryNormal:
		state.CurrentDelay = 0
		state.Paused = false
	case MemoryWarning:
		state.CurrentDelay = 100 * time.Millisecond
		state.Paused = false
	case MemoryCritical:
		state.CurrentDelay = 500 * time.Millisecond
		state.Paused = false
	case MemoryEmergency:
		state.CurrentDelay = 2 * time.Second
		state.Paused = true
	}

	state.LastUpdate = time.Now()
	return state.CurrentDelay
}

func (c *Controller) RegisterCallback(level MemoryLevel, callback func(MemoryStats)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callbacks[level] = append(c.callbacks[level], callback)
}

func (c *Controller) notifyCallbacks(level MemoryLevel, stats MemoryStats) {
	c.mu.RLock()
	callbacks := append([]func(MemoryStats){}, c.callbacks[level]...)
	c.mu.RUnlock()

	for _, cb := range callbacks {
		go cb(stats)
	}
}

func (c *Controller) ForceGC() {
	c.triggerGC("manual trigger")
}

func (c *Controller) GetConfig() ThresholdConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

func (c *Controller) UpdateConfig(config ThresholdConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
	if logger.Log != nil {
		logger.Log.Infow("Configuração de memória atualizada",
			"max_memory_mb", config.MaxMemoryMB,
			"warning_percent", config.WarningPercent,
			"critical_percent", config.CriticalPercent)
	}
}
