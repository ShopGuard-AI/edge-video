package e2e

import (
	"os"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// TestSoakOneHour executa teste de 1 hora para detectar memory/goroutine leaks
func TestSoakOneHour(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping soak test in short mode")
	}

	t.Log("⏰ SOAK TEST: 1 Hour Continuous Operation")
	t.Log("   Cameras: 5 simultaneous @ 15 FPS")
	t.Log("   Duration: 60 minutes")
	t.Log("   Monitoring: Memory, Goroutines, GC, Frames")
	t.Log("")
	t.Log("⚠️  This test will take 1 hour to complete!")
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

	// Iniciar resource monitor
	monitor := NewResourceMonitor()
	monitor.Start(10 * time.Second) // Sample a cada 10s
	defer monitor.Stop()

	t.Log("📊 Resource monitoring started (sampling every 10s)")
	t.Log("")

	// Iniciar producer
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

	t.Log("✅ Producer started")
	t.Log("")

	// Aguardar 30s para estabilização
	t.Log("⏳ Waiting 30s for system stabilization...")
	time.Sleep(30 * time.Second)

	if !proc.IsRunning() {
		stdout, stderr := proc.GetOutput()
		t.Log("❌ Producer stopped unexpectedly during startup")
		t.Logf("Stdout: %s", stdout)
		t.Logf("Stderr: %s", stderr)
		t.FailNow()
	}

	t.Log("✅ System stable, starting soak test...")
	t.Log("")

	// Teste de 1 hora
	testDuration := 60 * time.Minute
	testStart := time.Now()
	testEnd := testStart.Add(testDuration)

	t.Logf("🚀 Soak test running...")
	t.Logf("   Start: %s", testStart.Format("15:04:05"))
	t.Logf("   End:   %s", testEnd.Format("15:04:05"))
	t.Log("")

	// Updates a cada 5 minutos
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	checkpointCount := 0
	lastFrameCounts := make(map[string]int)

	for time.Now().Before(testEnd) {
		select {
		case <-ticker.C:
			checkpointCount++
			elapsed := time.Since(testStart)
			remaining := testEnd.Sub(time.Now())

			// Resource stats
			stats := monitor.GetStats()

			t.Log("═══════════════════════════════════════════════")
			t.Logf("📊 CHECKPOINT #%d", checkpointCount)
			t.Logf("   Elapsed: %v | Remaining: %v", FormatDuration(elapsed), FormatDuration(remaining))
			t.Log("")

			t.Logf("💾 Memory Usage:")
			t.Logf("   Current: %.2f MB", stats.MaxMemoryMB)
			t.Logf("   Average: %.2f MB", stats.AvgMemoryMB)
			t.Logf("   Range: %.2f - %.2f MB", stats.MinMemoryMB, stats.MaxMemoryMB)
			t.Logf("   Delta: %.2f MB", stats.MaxMemoryMB-stats.MinMemoryMB)
			t.Log("")

			t.Logf("🔄 Goroutines:")
			t.Logf("   Max: %d", stats.MaxGoroutines)
			t.Log("")

			t.Logf("🗑️  GC Activity:")
			t.Logf("   Total Cycles: %d", stats.GCCount)
			t.Logf("   Rate: %.1f GC/min", float64(stats.GCCount)/(elapsed.Minutes()))
			t.Log("")

			// Frame counts
			t.Logf("📹 Frames in Redis:")
			currentFrameCounts := make(map[string]int)
			totalFrames := 0

			for _, cam := range config.Cameras {
				count, err := CountFramesInRedis(
					config.Redis.Address,
					config.Redis.Password,
					config.Redis.DB,
					cam.ID,
				)

				if err != nil {
					t.Logf("   %s: ERROR - %v", cam.ID, err)
				} else {
					currentFrameCounts[cam.ID] = count
					totalFrames += count

					lastCount := lastFrameCounts[cam.ID]
					delta := count - lastCount

					t.Logf("   %s: %d frames (+%d since last checkpoint)", cam.ID, count, delta)
				}
			}

			lastFrameCounts = currentFrameCounts

			t.Logf("   TOTAL: %d frames", totalFrames)
			t.Log("")

			// Verificar se producer ainda está rodando
			if !proc.IsRunning() {
				stdout, stderr := proc.GetOutput()
				t.Log("❌ Producer stopped unexpectedly during soak test!")
				t.Logf("Stdout: %s", stdout)
				t.Logf("Stderr: %s", stderr)
				t.FailNow()
			}

			t.Log("✅ Producer still running")
			t.Log("═══════════════════════════════════════════════")
			t.Log("")

		default:
			time.Sleep(1 * time.Second)
		}
	}

	t.Log("")
	t.Log("✅ 1-hour soak test completed!")
	t.Log("")

	// Parar monitoring
	monitor.Stop()

	// Análise final
	finalStats := monitor.GetStats()

	t.Log("═══════════════════════════════════════════════")
	t.Log("📊 FINAL SOAK TEST RESULTS")
	t.Log("═══════════════════════════════════════════════")
	t.Log("")

	t.Log("💾 Memory Analysis:")
	t.Logf("   Min: %.2f MB", finalStats.MinMemoryMB)
	t.Logf("   Max: %.2f MB", finalStats.MaxMemoryMB)
	t.Logf("   Avg: %.2f MB", finalStats.AvgMemoryMB)
	t.Logf("   Delta: %.2f MB", finalStats.MaxMemoryMB-finalStats.MinMemoryMB)
	t.Logf("   Samples: %d", finalStats.SampleCount)
	t.Log("")

	t.Log("🔄 Goroutine Analysis:")
	t.Logf("   Max: %d", finalStats.MaxGoroutines)
	t.Log("")

	t.Log("🗑️  GC Analysis:")
	t.Logf("   Total Cycles: %d", finalStats.GCCount)
	t.Logf("   Avg per minute: %.1f", float64(finalStats.GCCount)/60.0)
	t.Log("")

	// Frame analysis
	t.Log("📹 Final Frame Counts:")
	totalExpected := config.FPS * 3600 * 5 // 15 FPS × 3600s × 5 cameras
	totalCaptured := 0

	for _, cam := range config.Cameras {
		count, err := CountFramesInRedis(
			config.Redis.Address,
			config.Redis.Password,
			config.Redis.DB,
			cam.ID,
		)

		if err != nil {
			t.Logf("   %s: ERROR - %v", cam.ID, err)
		} else {
			totalCaptured += count
			expectedPerCam := config.FPS * 3600
			rate := float64(count) / float64(expectedPerCam) * 100
			t.Logf("   %s: %d frames (%.1f%%)", cam.ID, count, rate)
		}
	}

	overallRate := float64(totalCaptured) / float64(totalExpected) * 100
	t.Log("")
	t.Logf("Overall:")
	t.Logf("   Total Captured: %d frames", totalCaptured)
	t.Logf("   Total Expected: %d frames", totalExpected)
	t.Logf("   Capture Rate: %.2f%%", overallRate)
	t.Logf("   Frame Drop: %.2f%%", 100.0-overallRate)
	t.Log("")

	// Validações
	t.Log("🎯 VALIDATION RESULTS:")
	t.Log("")

	passedAll := true

	// 1. Memory leak check
	memoryGrowth := finalStats.MaxMemoryMB - finalStats.MinMemoryMB
	if memoryGrowth > 100 {
		t.Errorf("❌ MEMORY LEAK DETECTED: %.2f MB growth", memoryGrowth)
		passedAll = false
	} else {
		t.Logf("✅ No memory leak: %.2f MB growth", memoryGrowth)
	}

	// 2. Memory stability
	if finalStats.MaxMemoryMB > 600 {
		t.Errorf("❌ Memory usage too high: %.2f MB", finalStats.MaxMemoryMB)
		passedAll = false
	} else {
		t.Logf("✅ Memory usage acceptable: %.2f MB", finalStats.MaxMemoryMB)
	}

	// 3. Goroutine leak check
	if finalStats.MaxGoroutines > 150 {
		t.Errorf("❌ GOROUTINE LEAK DETECTED: %d goroutines", finalStats.MaxGoroutines)
		passedAll = false
	} else {
		t.Logf("✅ No goroutine leak: %d goroutines", finalStats.MaxGoroutines)
	}

	// 4. Capture rate
	if overallRate < 95.0 {
		t.Errorf("❌ Capture rate too low: %.2f%%", overallRate)
		passedAll = false
	} else {
		t.Logf("✅ Capture rate acceptable: %.2f%%", overallRate)
	}

	// 5. GC frequency
	gcPerMinute := float64(finalStats.GCCount) / 60.0
	if gcPerMinute > 10.0 {
		t.Errorf("❌ GC too frequent: %.1f GC/min", gcPerMinute)
		passedAll = false
	} else {
		t.Logf("✅ GC frequency acceptable: %.1f GC/min", gcPerMinute)
	}

	t.Log("")
	if passedAll {
		t.Log("🎉 ALL VALIDATIONS PASSED!")
		t.Log("   System is STABLE for 1-hour continuous operation")
	} else {
		t.Log("⚠️  SOME VALIDATIONS FAILED")
		t.Log("   Review results above for details")
	}

	t.Log("")
	t.Log("═══════════════════════════════════════════════")
}

// TestSoakTenMinutes teste soak mais curto (10 minutos) para desenvolvimento
func TestSoakTenMinutes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping soak test in short mode")
	}

	t.Log("⏰ SOAK TEST: 10 Minutes")
	t.Log("   Cameras: 5 simultaneous @ 15 FPS")
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

	// Resource monitor
	monitor := NewResourceMonitor()
	monitor.Start(5 * time.Second)
	defer monitor.Stop()

	// Iniciar producer
	proc, err := StartProducer("config.yaml")
	if err != nil {
		t.Fatalf("Failed to start producer: %v", err)
	}
	defer proc.Stop()

	t.Log("✅ Producer started")
	time.Sleep(5 * time.Second)

	// Rodar por 10 minutos
	testDuration := 10 * time.Minute
	t.Logf("🚀 Running for 10 minutes...")
	t.Logf("   End time: %s", time.Now().Add(testDuration).Format("15:04:05"))
	t.Log("")

	time.Sleep(testDuration)

	t.Log("✅ 10-minute test completed")
	t.Log("")

	// Stats
	stats := monitor.GetStats()

	t.Log("📊 RESULTS:")
	t.Logf("   Memory: %.2f MB (delta: %.2f MB)", stats.MaxMemoryMB, stats.MaxMemoryMB-stats.MinMemoryMB)
	t.Logf("   Goroutines: %d", stats.MaxGoroutines)
	t.Logf("   GC Cycles: %d (%.1f/min)", stats.GCCount, float64(stats.GCCount)/10.0)

	// Frames
	totalFrames := 0
	for _, cam := range config.Cameras {
		count, _ := CountFramesInRedis(
			config.Redis.Address,
			config.Redis.Password,
			config.Redis.DB,
			cam.ID,
		)
		totalFrames += count
	}

	expectedTotal := config.FPS * 600 * 5 // 15 FPS × 600s × 5 cameras
	rate := float64(totalFrames) / float64(expectedTotal) * 100

	t.Logf("   Frames: %d/%d (%.1f%%)", totalFrames, expectedTotal, rate)

	// Validations
	memoryGrowth := stats.MaxMemoryMB - stats.MinMemoryMB
	if memoryGrowth > 50 {
		t.Errorf("❌ Possible memory leak: %.2f MB growth", memoryGrowth)
	} else {
		t.Logf("✅ Memory stable: %.2f MB growth", memoryGrowth)
	}

	if rate < 95.0 {
		t.Errorf("❌ Low capture rate: %.1f%%", rate)
	} else {
		t.Logf("✅ Capture rate OK: %.1f%%", rate)
	}
}
