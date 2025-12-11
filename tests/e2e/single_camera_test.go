package e2e

import (
	"os"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// TestSingleCameraRTMP testa cam1 (RTMP) isoladamente
func TestSingleCameraRTMP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Carregar config original
	configPath, err := FindConfigFile("config.yaml")
	if err != nil {
		t.Fatalf("Failed to find config: %v", err)
	}

	originalConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(originalConfig, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Criar config temporário com apenas cam1
	tempConfig := config
	tempConfig.Cameras = config.Cameras[:1] // Mantém apenas primeira câmera com TODA a config

	// Salvar config temporário
	tempConfigData, err := yaml.Marshal(tempConfig)
	if err != nil {
		t.Fatalf("Failed to marshal temp config: %v", err)
	}

	tempConfigPath := "config.e2e.cam1.yaml"
	if err := os.WriteFile(tempConfigPath, tempConfigData, 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}
	defer os.Remove(tempConfigPath)

	t.Logf("📹 Testing cam1 (RTMP - Mercado Autônomo)")
	t.Logf("   Config: %s", tempConfigPath)
	t.Logf("   Duration: 30 seconds")
	t.Log("")

	// Iniciar producer
	proc, err := StartProducer(tempConfigPath)
	if err != nil {
		t.Fatalf("Failed to start producer: %v", err)
	}
	defer func() {
		if err := proc.Stop(); err != nil {
			t.Logf("⚠️  Error stopping producer: %v", err)
		}
	}()

	t.Log("✅ Producer started")

	// Aguardar 5s para garantir inicialização
	time.Sleep(5 * time.Second)

	// Verificar se processo ainda está rodando
	if !proc.IsRunning() {
		stdout, stderr := proc.GetOutput()
		t.Logf("❌ Producer stopped unexpectedly")
		t.Logf("Stdout: %s", stdout)
		t.Logf("Stderr: %s", stderr)
		t.FailNow()
	}

	t.Log("✅ Producer running after 5s")

	// Aguardar 30s de captura
	t.Log("⏳ Capturing frames for 30 seconds...")
	time.Sleep(30 * time.Second)

	// Verificar frames no Redis
	frameCount, err := CountFramesInRedis(
		config.Redis.Address,
		config.Redis.Password,
		config.Redis.DB,
		"cam1",
	)

	if err != nil {
		t.Errorf("❌ Failed to count frames in Redis: %v", err)
	}

	// Com FPS=15, em 30s devemos ter ~450 frames
	// TTL=120s, então todos devem estar no Redis
	expectedFrames := config.FPS * 30
	minExpected := int(float64(expectedFrames) * 0.90) // 90% do esperado

	t.Logf("📊 Frames captured:")
	t.Logf("   Total in Redis: %d frames", frameCount)
	t.Logf("   Expected: ~%d frames (FPS=%d × 30s)", expectedFrames, config.FPS)
	t.Logf("   Min acceptable: %d frames (90%%)", minExpected)

	if frameCount < minExpected {
		t.Errorf("❌ Too few frames captured: %d < %d", frameCount, minExpected)
	} else {
		t.Logf("✅ Frame count OK: %d frames", frameCount)
	}

	// Obter último frame
	latestFrame, err := GetLatestFrameFromRedis(
		config.Redis.Address,
		config.Redis.Password,
		config.Redis.DB,
		"cam1",
	)

	if err != nil {
		t.Errorf("❌ Failed to get latest frame: %v", err)
	} else {
		t.Logf("✅ Latest frame retrieved: %d bytes", len(latestFrame))

		// Validar que é JPEG (começa com FF D8)
		if len(latestFrame) > 2 && latestFrame[0] == 0xFF && latestFrame[1] == 0xD8 {
			t.Log("✅ Frame is valid JPEG")
		} else {
			t.Error("❌ Frame is not valid JPEG")
		}
	}

	// Parar producer gracefully
	t.Log("⏳ Stopping producer...")
	stopStart := time.Now()
	if err := proc.Stop(); err != nil {
		t.Errorf("❌ Failed to stop producer gracefully: %v", err)
	} else {
		stopDuration := time.Since(stopStart)
		t.Logf("✅ Producer stopped gracefully in %v", stopDuration)

		if stopDuration > 5*time.Second {
			t.Errorf("❌ Shutdown took too long: %v (max: 5s)", stopDuration)
		}
	}
}

// TestSingleCameraRTSP testa cam2 (RTSP) isoladamente
func TestSingleCameraRTSP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Carregar config original
	configPath, err := FindConfigFile("config.yaml")
	if err != nil {
		t.Fatalf("Failed to find config: %v", err)
	}

	originalConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(originalConfig, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Criar config temporário com apenas cam2
	if len(config.Cameras) < 2 {
		t.Skip("cam2 not configured")
	}

	tempConfig := config
	tempConfig.Cameras = config.Cameras[1:2] // Mantém apenas segunda câmera com TODA a config

	// Salvar config temporário
	tempConfigData, err := yaml.Marshal(tempConfig)
	if err != nil {
		t.Fatalf("Failed to marshal temp config: %v", err)
	}

	tempConfigPath := "config.e2e.cam2.yaml"
	if err := os.WriteFile(tempConfigPath, tempConfigData, 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}
	defer os.Remove(tempConfigPath)

	t.Logf("📹 Testing cam2 (RTSP - Pix Force Canal 1)")
	t.Logf("   Config: %s", tempConfigPath)
	t.Logf("   Duration: 30 seconds")
	t.Log("")

	// Iniciar producer
	proc, err := StartProducer(tempConfigPath)
	if err != nil {
		t.Fatalf("Failed to start producer: %v", err)
	}
	defer func() {
		if err := proc.Stop(); err != nil {
			t.Logf("⚠️  Error stopping producer: %v", err)
		}
	}()

	t.Log("✅ Producer started")

	// Aguardar 5s para garantir inicialização
	time.Sleep(5 * time.Second)

	// Verificar se processo ainda está rodando
	if !proc.IsRunning() {
		stdout, stderr := proc.GetOutput()
		t.Logf("❌ Producer stopped unexpectedly")
		t.Logf("Stdout: %s", stdout)
		t.Logf("Stderr: %s", stderr)
		t.FailNow()
	}

	t.Log("✅ Producer running after 5s")

	// Aguardar 30s de captura
	t.Log("⏳ Capturing frames for 30 seconds...")
	time.Sleep(30 * time.Second)

	// Verificar frames no Redis
	frameCount, err := CountFramesInRedis(
		config.Redis.Address,
		config.Redis.Password,
		config.Redis.DB,
		"cam2",
	)

	if err != nil {
		t.Errorf("❌ Failed to count frames in Redis: %v", err)
	}

	// Com FPS=15, em 30s devemos ter ~450 frames
	expectedFrames := config.FPS * 30
	minExpected := int(float64(expectedFrames) * 0.90) // 90% do esperado

	t.Logf("📊 Frames captured:")
	t.Logf("   Total in Redis: %d frames", frameCount)
	t.Logf("   Expected: ~%d frames (FPS=%d × 30s)", expectedFrames, config.FPS)
	t.Logf("   Min acceptable: %d frames (90%%)", minExpected)

	if frameCount < minExpected {
		t.Errorf("❌ Too few frames captured: %d < %d", frameCount, minExpected)
	} else {
		t.Logf("✅ Frame count OK: %d frames", frameCount)
	}

	// Parar producer gracefully
	t.Log("⏳ Stopping producer...")
	stopStart := time.Now()
	if err := proc.Stop(); err != nil {
		t.Errorf("❌ Failed to stop producer gracefully: %v", err)
	} else {
		stopDuration := time.Since(stopStart)
		t.Logf("✅ Producer stopped gracefully in %v", stopDuration)

		if stopDuration > 5*time.Second {
			t.Errorf("❌ Shutdown took too long: %v (max: 5s)", stopDuration)
		}
	}
}

// TestSingleCameraFrameQuality valida qualidade dos frames capturados
func TestSingleCameraFrameQuality(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

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

	// Testar qualidade de frame de cada câmera
	cameras := []string{"cam1", "cam2"}

	for _, camID := range cameras {
		t.Run(camID, func(t *testing.T) {
			frame, err := GetLatestFrameFromRedis(
				config.Redis.Address,
				config.Redis.Password,
				config.Redis.DB,
				camID,
			)

			if err != nil {
				t.Skipf("No frames found for %s: %v", camID, err)
			}

			// Validar tamanho mínimo (10KB)
			if len(frame) < 10*1024 {
				t.Errorf("❌ Frame too small: %d bytes", len(frame))
			} else {
				t.Logf("✅ Frame size: %s", FormatBytes(float64(len(frame))))
			}

			// Validar JPEG header
			if len(frame) < 2 || frame[0] != 0xFF || frame[1] != 0xD8 {
				t.Error("❌ Invalid JPEG header")
			} else {
				t.Log("✅ Valid JPEG header")
			}

			// Validar JPEG trailer
			if len(frame) < 2 || frame[len(frame)-2] != 0xFF || frame[len(frame)-1] != 0xD9 {
				t.Error("❌ Invalid JPEG trailer")
			} else {
				t.Log("✅ Valid JPEG trailer")
			}
		})
	}
}
