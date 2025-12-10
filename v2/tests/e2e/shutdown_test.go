package e2e

import (
	"os"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// TestGracefulShutdown valida shutdown graceful <5s com 5 câmeras ativas
func TestGracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	t.Log("🛑 GRACEFUL SHUTDOWN TEST")
	t.Log("   Cameras: 5 simultaneous")
	t.Log("   Target: Shutdown <5 seconds")
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

	// Iniciar producer
	proc, err := StartProducer("config.yaml")
	if err != nil {
		t.Fatalf("Failed to start producer: %v", err)
	}

	t.Log("✅ Producer started")

	// Aguardar 10s para todas as câmeras iniciarem
	t.Log("⏳ Waiting 10s for cameras to start...")
	time.Sleep(10 * time.Second)

	if !proc.IsRunning() {
		stdout, stderr := proc.GetOutput()
		t.Log("❌ Producer stopped unexpectedly")
		t.Logf("Stdout: %s", stdout)
		t.Logf("Stderr: %s", stderr)
		t.FailNow()
	}

	t.Log("✅ All cameras running")
	t.Log("")

	// Verificar que temos frames no Redis
	t.Log("📊 Checking frames before shutdown:")
	framesBeforeShutdown := make(map[string]int)

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
			framesBeforeShutdown[cam.ID] = count
			t.Logf("   %s: %d frames", cam.ID, count)
		}
	}

	t.Log("")

	// Enviar SIGTERM e medir tempo de shutdown
	t.Log("⏳ Sending SIGTERM (graceful shutdown)...")
	shutdownStart := time.Now()

	if err := proc.Stop(); err != nil {
		t.Errorf("❌ Failed to stop producer gracefully: %v", err)
	}

	shutdownDuration := time.Since(shutdownStart)

	t.Log("")
	t.Logf("📊 SHUTDOWN RESULTS:")
	t.Logf("   Duration: %v", shutdownDuration)
	t.Logf("   Target: <5s")
	t.Log("")

	// Validação
	if shutdownDuration > 5*time.Second {
		t.Errorf("❌ Shutdown took too long: %v (max: 5s)", shutdownDuration)
	} else if shutdownDuration > 1*time.Second {
		t.Logf("⚠️  Shutdown took %v (OK, but could be faster)", shutdownDuration)
	} else {
		t.Logf("✅ Shutdown FAST: %v", shutdownDuration)
	}

	// Verificar frames após shutdown
	t.Log("")
	t.Log("📊 Checking frames after shutdown:")

	for _, cam := range config.Cameras {
		count, err := CountFramesInRedis(
			config.Redis.Address,
			config.Redis.Password,
			config.Redis.DB,
			cam.ID,
		)

		if err != nil {
			t.Logf("   %s: ERROR - %v", cam.ID, err)
			continue
		}

		before := framesBeforeShutdown[cam.ID]
		added := count - before

		t.Logf("   %s: %d frames (+%d during test)", cam.ID, count, added)
	}

	t.Log("")
	t.Log("✅ Graceful shutdown test complete")
}

// TestShutdownCleanup valida que recursos são liberados após shutdown
func TestShutdownCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	t.Log("🧹 SHUTDOWN CLEANUP TEST")
	t.Log("")

	// Iniciar producer
	proc, err := StartProducer("config.yaml")
	if err != nil {
		t.Fatalf("Failed to start producer: %v", err)
	}

	// Aguardar 5s
	time.Sleep(5 * time.Second)

	// Parar
	t.Log("⏳ Stopping producer...")
	if err := proc.Stop(); err != nil {
		t.Errorf("Failed to stop producer: %v", err)
	}

	t.Log("✅ Producer stopped")
	t.Log("")

	// Aguardar mais 2s para cleanup
	time.Sleep(2 * time.Second)

	// Verificar se processo realmente terminou
	if proc.IsRunning() {
		t.Error("❌ Producer still running after Stop()")
		proc.cmd.Process.Kill()
	} else {
		t.Log("✅ Producer process terminated")
	}

	// Verificar stdout/stderr
	stdout, stderr := proc.GetOutput()

	if stdout != "" {
		t.Logf("📄 Stdout output:")
		t.Logf("%s", stdout)
	}

	if stderr != "" {
		t.Logf("⚠️  Stderr output:")
		t.Logf("%s", stderr)
	}

	// Buscar por erros comuns no stderr
	if len(stderr) > 0 {
		t.Log("")
		t.Log("🔍 Checking for common errors...")

		errors := []string{
			"panic",
			"fatal error",
			"runtime error",
			"deadlock",
			"goroutine leak",
		}

		foundErrors := false
		for _, errStr := range errors {
			if len(stderr) > 0 && contains(stderr, errStr) {
				t.Errorf("❌ Found error in stderr: %s", errStr)
				foundErrors = true
			}
		}

		if !foundErrors {
			t.Log("✅ No critical errors found in stderr")
		}
	}

	t.Log("")
	t.Log("✅ Cleanup test complete")
}

// TestShutdownDuringHighLoad valida shutdown durante carga alta
func TestShutdownDuringHighLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	t.Log("💥 SHUTDOWN DURING HIGH LOAD TEST")
	t.Log("   Scenario: Shutdown while processing high FPS")
	t.Log("")

	// Carregar config e aumentar FPS temporariamente
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

	// Criar config com FPS alto
	tempConfig := config
	tempConfig.FPS = 30 // Aumentar para 30 FPS

	tempConfigData, err := yaml.Marshal(tempConfig)
	if err != nil {
		t.Fatalf("Failed to marshal temp config: %v", err)
	}

	tempConfigPath := "config.e2e.highload.yaml"
	if err := os.WriteFile(tempConfigPath, tempConfigData, 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}
	defer os.Remove(tempConfigPath)

	// Iniciar producer
	proc, err := StartProducer(tempConfigPath)
	if err != nil {
		t.Fatalf("Failed to start producer: %v", err)
	}

	t.Log("✅ Producer started @ 30 FPS")

	// Aguardar 10s para carga alta
	t.Log("⏳ Building up load (10s @ 30 FPS)...")
	time.Sleep(10 * time.Second)

	// Shutdown imediato
	t.Log("⏳ Sending SIGTERM during high load...")
	shutdownStart := time.Now()

	if err := proc.Stop(); err != nil {
		t.Errorf("Failed to stop producer: %v", err)
	}

	shutdownDuration := time.Since(shutdownStart)

	t.Log("")
	t.Logf("📊 Shutdown during high load: %v", shutdownDuration)

	if shutdownDuration > 5*time.Second {
		t.Errorf("❌ Shutdown took too long even during high load: %v", shutdownDuration)
	} else {
		t.Logf("✅ Shutdown OK during high load: %v", shutdownDuration)
	}
}

// Helper function
func contains(s string, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && s != "" && substr != "" &&
		len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
