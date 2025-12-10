package e2e

import (
	"os"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// TestStressFiveCameras testa 5 câmeras simultâneas por 5 minutos
func TestStressFiveCameras(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	t.Log("🔥 STRESS TEST: 5 Câmeras Simultâneas @ 15 FPS")
	t.Log("   Duration: 5 minutes")
	t.Log("   Target: <2% frame drop, stable memory, <30% CPU")
	t.Log("")

	// Carregar config
	configPath, err := FindConfigFile("config.yaml")
	if err != nil {
		t.Fatalf("Failed to find config: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Validar que temos 5 câmeras
	if len(config.Cameras) < 5 {
		t.Fatalf("Need 5 cameras configured, got %d", len(config.Cameras))
	}

	t.Logf("📹 Cameras configured:")
	for i := 0; i < 5; i++ {
		t.Logf("   %s: %s", config.Cameras[i].ID, config.Cameras[i].URL[:50]+"...")
	}
	t.Log("")

	// Iniciar resource monitor
	monitor := NewResourceMonitor()
	monitor.Start(2 * time.Second) // Sample a cada 2s
	defer monitor.Stop()

	t.Log("📊 Resource monitoring started (sampling every 2s)")
	t.Log("")

	// Iniciar producer com config completo (5 câmeras)
	proc, err := StartProducer("config.yaml")
	if err != nil {
		t.Fatalf("Failed to start producer: %v", err)
	}
	defer func() {
		t.Log("⏳ Stopping producer...")
		if err := proc.Stop(); err != nil {
			t.Logf("⚠️  Error stopping producer: %v", err)
		}
	}()

	t.Log("✅ Producer started with 5 cameras")
	t.Log("")

	// Aguardar 10s para estabilização
	t.Log("⏳ Waiting 10s for system stabilization...")
	time.Sleep(10 * time.Second)

	// Verificar se todas as câmeras estão rodando
	if !proc.IsRunning() {
		stdout, stderr := proc.GetOutput()
		t.Log("❌ Producer stopped unexpectedly during startup")
		t.Logf("Stdout: %s", stdout)
		t.Logf("Stderr: %s", stderr)
		t.FailNow()
	}

	t.Log("✅ Producer stable after 10s")
	t.Log("")

	// Rodar por 5 minutos
	testDuration := 5 * time.Minute
	t.Logf("🚀 Starting 5-minute stress test...")
	t.Logf("   Start time: %s", time.Now().Format("15:04:05"))
	t.Logf("   End time: %s", time.Now().Add(testDuration).Format("15:04:05"))
	t.Log("")

	// Ticker para status updates a cada 30s
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	testStart := time.Now()
	testEnd := testStart.Add(testDuration)

	updateCount := 0
	for time.Now().Before(testEnd) {
		select {
		case <-ticker.C:
			updateCount++
			elapsed := time.Since(testStart)
			remaining := testEnd.Sub(time.Now())

			// Get resource stats
			stats := monitor.GetStats()

			t.Logf("📊 Update #%d (elapsed: %v, remaining: %v)", updateCount, FormatDuration(elapsed), FormatDuration(remaining))
			t.Logf("   Memory: %.1f MB (min: %.1f, max: %.1f, avg: %.1f)",
				stats.MaxMemoryMB, stats.MinMemoryMB, stats.MaxMemoryMB, stats.AvgMemoryMB)
			t.Logf("   Goroutines: %d", stats.MaxGoroutines)
			t.Logf("   GC Cycles: %d", stats.GCCount)

			// Verificar se producer ainda está rodando
			if !proc.IsRunning() {
				stdout, stderr := proc.GetOutput()
				t.Log("❌ Producer stopped unexpectedly during stress test")
				t.Logf("Stdout: %s", stdout)
				t.Logf("Stderr: %s", stderr)
				t.FailNow()
			}

		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

	t.Log("")
	t.Log("✅ 5-minute stress test completed")
	t.Log("")

	// Parar monitoring
	monitor.Stop()

	// Análise final
	finalStats := monitor.GetStats()

	t.Log("📊 FINAL RESULTS:")
	t.Log("")
	t.Logf("Memory Usage:")
	t.Logf("   Min: %.2f MB", finalStats.MinMemoryMB)
	t.Logf("   Max: %.2f MB", finalStats.MaxMemoryMB)
	t.Logf("   Avg: %.2f MB", finalStats.AvgMemoryMB)
	t.Logf("   Delta: %.2f MB", finalStats.MaxMemoryMB-finalStats.MinMemoryMB)
	t.Log("")

	t.Logf("Goroutines:")
	t.Logf("   Max: %d", finalStats.MaxGoroutines)
	t.Log("")

	t.Logf("GC Activity:")
	t.Logf("   Total GC Cycles: %d", finalStats.GCCount)
	t.Logf("   GC per minute: %.1f", float64(finalStats.GCCount)/5.0)
	t.Log("")

	// Verificar frames no Redis para cada câmera
	t.Log("📹 Frame counts in Redis:")
	totalFrames := 0
	expectedPerCamera := config.FPS * int(testDuration.Seconds())

	for i := 0; i < 5; i++ {
		cameraID := config.Cameras[i].ID
		count, err := CountFramesInRedis(
			config.Redis.Address,
			config.Redis.Password,
			config.Redis.DB,
			cameraID,
		)

		if err != nil {
			t.Logf("   %s: ERROR - %v", cameraID, err)
		} else {
			totalFrames += count
			captureRate := float64(count) / float64(expectedPerCamera) * 100
			t.Logf("   %s: %d frames (%.1f%% of expected %d)",
				cameraID, count, captureRate, expectedPerCamera)
		}
	}

	t.Log("")
	totalExpected := expectedPerCamera * 5
	overallRate := float64(totalFrames) / float64(totalExpected) * 100
	frameDropRate := 100.0 - overallRate

	t.Logf("Overall:")
	t.Logf("   Total captured: %d frames", totalFrames)
	t.Logf("   Total expected: %d frames", totalExpected)
	t.Logf("   Capture rate: %.2f%%", overallRate)
	t.Logf("   Frame drop rate: %.2f%%", frameDropRate)
	t.Log("")

	// Validações
	t.Log("🎯 VALIDATION:")
	t.Log("")

	// 1. Memory should be stable (<500MB)
	if finalStats.MaxMemoryMB > 500 {
		t.Errorf("❌ Memory usage too high: %.2f MB (max: 500 MB)", finalStats.MaxMemoryMB)
	} else {
		t.Logf("✅ Memory usage OK: %.2f MB", finalStats.MaxMemoryMB)
	}

	// 2. Memory delta should be small (no memory leak)
	memoryDelta := finalStats.MaxMemoryMB - finalStats.MinMemoryMB
	if memoryDelta > 200 {
		t.Errorf("❌ Memory delta too large: %.2f MB (possible memory leak)", memoryDelta)
	} else {
		t.Logf("✅ Memory stable: delta %.2f MB", memoryDelta)
	}

	// 3. Frame drop rate should be <2%
	if frameDropRate > 2.0 {
		t.Errorf("❌ Frame drop rate too high: %.2f%% (max: 2%%)", frameDropRate)
	} else {
		t.Logf("✅ Frame drop rate OK: %.2f%%", frameDropRate)
	}

	// 4. Goroutines should be reasonable (<100)
	if finalStats.MaxGoroutines > 100 {
		t.Errorf("❌ Too many goroutines: %d (max: 100)", finalStats.MaxGoroutines)
	} else {
		t.Logf("✅ Goroutine count OK: %d", finalStats.MaxGoroutines)
	}

	t.Log("")
	t.Log("🎉 Stress test validation complete!")
}

// TestStressThreeCamerasShort teste rápido com 3 câmeras por 2 minutos
func TestStressThreeCamerasShort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	t.Log("🔥 SHORT STRESS TEST: 3 Câmeras @ 15 FPS")
	t.Log("   Duration: 2 minutes")
	t.Log("")

	// Carregar config
	configPath, err := FindConfigFile("config.yaml")
	if err != nil {
		t.Fatalf("Failed to find config: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Criar config temporário com 3 câmeras
	if len(config.Cameras) < 3 {
		t.Fatalf("Need 3 cameras configured, got %d", len(config.Cameras))
	}

	tempConfig := config
	tempConfig.Cameras = config.Cameras[:3] // Apenas primeiras 3

	// Salvar config temporário
	tempConfigData, err := yaml.Marshal(tempConfig)
	if err != nil {
		t.Fatalf("Failed to marshal temp config: %v", err)
	}

	tempConfigPath := "config.e2e.3cams.yaml"
	if err := os.WriteFile(tempConfigPath, tempConfigData, 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}
	defer os.Remove(tempConfigPath)

	// Iniciar resource monitor
	monitor := NewResourceMonitor()
	monitor.Start(2 * time.Second)
	defer monitor.Stop()

	// Iniciar producer
	proc, err := StartProducer(tempConfigPath)
	if err != nil {
		t.Fatalf("Failed to start producer: %v", err)
	}
	defer proc.Stop()

	t.Log("✅ Producer started with 3 cameras")
	t.Log("")

	// Aguardar 5s para estabilização
	time.Sleep(5 * time.Second)

	// Rodar por 2 minutos
	testDuration := 2 * time.Minute
	t.Logf("🚀 Running for 2 minutes...")
	time.Sleep(testDuration)

	t.Log("✅ Test completed")
	t.Log("")

	// Stats finais
	stats := monitor.GetStats()

	t.Log("📊 RESULTS:")
	t.Logf("   Memory: %.2f MB (delta: %.2f MB)", stats.MaxMemoryMB, stats.MaxMemoryMB-stats.MinMemoryMB)
	t.Logf("   Goroutines: %d", stats.MaxGoroutines)
	t.Logf("   GC Cycles: %d", stats.GCCount)

	// Contar frames
	totalFrames := 0
	for i := 0; i < 3; i++ {
		count, _ := CountFramesInRedis(
			config.Redis.Address,
			config.Redis.Password,
			config.Redis.DB,
			config.Cameras[i].ID,
		)
		totalFrames += count
		t.Logf("   %s: %d frames", config.Cameras[i].ID, count)
	}

	expectedTotal := config.FPS * int(testDuration.Seconds()) * 3
	captureRate := float64(totalFrames) / float64(expectedTotal) * 100

	t.Logf("")
	t.Logf("Overall: %d/%d frames (%.1f%%)", totalFrames, expectedTotal, captureRate)

	if captureRate < 98.0 {
		t.Errorf("❌ Capture rate too low: %.1f%%", captureRate)
	} else {
		t.Logf("✅ Capture rate OK: %.1f%%", captureRate)
	}
}
