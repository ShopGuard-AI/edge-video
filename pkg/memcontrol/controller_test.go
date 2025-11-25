package memcontrol

import (
	"testing"
	"time"
)

func TestNewController(t *testing.T) {
	controller := NewController(1024)
	
	if controller == nil {
		t.Fatal("Controller não deve ser nil")
	}
	
	config := controller.GetConfig()
	if config.MaxMemoryMB != 1024 {
		t.Errorf("MaxMemoryMB esperado: 1024, obtido: %d", config.MaxMemoryMB)
	}
	
	if config.WarningPercent != 60.0 {
		t.Errorf("WarningPercent esperado: 60.0, obtido: %.2f", config.WarningPercent)
	}
}

func TestNewControllerAutoMemory(t *testing.T) {
	controller := NewController(0)
	
	if controller == nil {
		t.Fatal("Controller não deve ser nil")
	}
	
	config := controller.GetConfig()
	if config.MaxMemoryMB < 512 {
		t.Errorf("MaxMemoryMB automático deve ser >= 512, obtido: %d", config.MaxMemoryMB)
	}
}

func TestMemoryLevelString(t *testing.T) {
	tests := []struct {
		level    MemoryLevel
		expected string
	}{
		{MemoryNormal, "NORMAL"},
		{MemoryWarning, "WARNING"},
		{MemoryCritical, "CRITICAL"},
		{MemoryEmergency, "EMERGENCY"},
	}
	
	for _, tt := range tests {
		got := tt.level.String()
		if got != tt.expected {
			t.Errorf("Level.String() = %v, esperado %v", got, tt.expected)
		}
	}
}

func TestDetermineLevel(t *testing.T) {
	controller := NewController(1024)
	
	tests := []struct {
		percent  float64
		expected MemoryLevel
	}{
		{50.0, MemoryNormal},
		{59.9, MemoryNormal},
		{60.0, MemoryWarning},
		{74.9, MemoryWarning},
		{75.0, MemoryCritical},
		{84.9, MemoryCritical},
		{85.0, MemoryEmergency},
		{95.0, MemoryEmergency},
	}
	
	for _, tt := range tests {
		got := controller.determineLevel(tt.percent)
		if got != tt.expected {
			t.Errorf("determineLevel(%.1f) = %v, esperado %v", tt.percent, got, tt.expected)
		}
	}
}

func TestGetThrottleDelay(t *testing.T) {
	controller := NewController(1024)
	
	// Simula diferentes níveis
	controller.mu.Lock()
	controller.currentLevel = MemoryNormal
	controller.mu.Unlock()
	
	delay := controller.GetThrottleDelay("cam1")
	if delay != 0 {
		t.Errorf("Delay em Normal deve ser 0, obtido: %v", delay)
	}
	
	controller.mu.Lock()
	controller.currentLevel = MemoryWarning
	controller.mu.Unlock()
	
	delay = controller.GetThrottleDelay("cam1")
	if delay != 100*time.Millisecond {
		t.Errorf("Delay em Warning deve ser 100ms, obtido: %v", delay)
	}
	
	controller.mu.Lock()
	controller.currentLevel = MemoryCritical
	controller.mu.Unlock()
	
	delay = controller.GetThrottleDelay("cam1")
	if delay != 500*time.Millisecond {
		t.Errorf("Delay em Critical deve ser 500ms, obtido: %v", delay)
	}
	
	controller.mu.Lock()
	controller.currentLevel = MemoryEmergency
	controller.mu.Unlock()
	
	delay = controller.GetThrottleDelay("cam1")
	if delay != 2*time.Second {
		t.Errorf("Delay em Emergency deve ser 2s, obtido: %v", delay)
	}
}

func TestShouldThrottle(t *testing.T) {
	controller := NewController(1024)
	
	controller.mu.Lock()
	controller.currentLevel = MemoryNormal
	controller.mu.Unlock()
	
	if controller.ShouldThrottle() {
		t.Error("Não deve throttle em Normal")
	}
	
	controller.mu.Lock()
	controller.currentLevel = MemoryWarning
	controller.mu.Unlock()
	
	if controller.ShouldThrottle() {
		t.Error("Não deve throttle em Warning")
	}
	
	controller.mu.Lock()
	controller.currentLevel = MemoryCritical
	controller.mu.Unlock()
	
	if !controller.ShouldThrottle() {
		t.Error("Deve throttle em Critical")
	}
	
	controller.mu.Lock()
	controller.currentLevel = MemoryEmergency
	controller.mu.Unlock()
	
	if !controller.ShouldThrottle() {
		t.Error("Deve throttle em Emergency")
	}
}

func TestShouldPause(t *testing.T) {
	controller := NewController(1024)
	
	controller.mu.Lock()
	controller.currentLevel = MemoryNormal
	controller.mu.Unlock()
	
	if controller.ShouldPause() {
		t.Error("Não deve pausar em Normal")
	}
	
	controller.mu.Lock()
	controller.currentLevel = MemoryCritical
	controller.mu.Unlock()
	
	if controller.ShouldPause() {
		t.Error("Não deve pausar em Critical")
	}
	
	controller.mu.Lock()
	controller.currentLevel = MemoryEmergency
	controller.mu.Unlock()
	
	if !controller.ShouldPause() {
		t.Error("Deve pausar em Emergency")
	}
}

func TestRegisterCallback(t *testing.T) {
	controller := NewController(1024)
	
	called := false
	controller.RegisterCallback(MemoryWarning, func(stats MemoryStats) {
		called = true
	})
	
	// Simula mudança de nível
	controller.mu.Lock()
	oldLevel := controller.currentLevel
	controller.currentLevel = MemoryWarning
	stats := MemoryStats{Level: MemoryWarning}
	controller.mu.Unlock()
	
	controller.onLevelChange(oldLevel, MemoryWarning, stats)
	
	// Aguarda callback assíncrono
	time.Sleep(100 * time.Millisecond)
	
	if !called {
		t.Error("Callback não foi chamado")
	}
}

func TestUpdateConfig(t *testing.T) {
	controller := NewController(1024)
	
	newConfig := ThresholdConfig{
		MaxMemoryMB:      2048,
		WarningPercent:   50.0,
		CriticalPercent:  70.0,
		EmergencyPercent: 90.0,
	}
	
	controller.UpdateConfig(newConfig)
	
	config := controller.GetConfig()
	if config.MaxMemoryMB != 2048 {
		t.Errorf("MaxMemoryMB esperado: 2048, obtido: %d", config.MaxMemoryMB)
	}
	
	if config.WarningPercent != 50.0 {
		t.Errorf("WarningPercent esperado: 50.0, obtido: %.2f", config.WarningPercent)
	}
}
