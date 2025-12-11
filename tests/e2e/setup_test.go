package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v3"
)

// Config representa a estrutura do config.yaml
type Config struct {
	FPS     int `yaml:"fps"`
	Quality int `yaml:"quality"`

	AMQP struct {
		URL               string `yaml:"url"`
		PrefetchCount     int    `yaml:"prefetch_count"`
		PublisherConfirms bool   `yaml:"publisher_confirms"`
	} `yaml:"amqp"`

	CircuitBreaker struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"circuit_breaker"`

	Redis struct {
		Enabled  bool   `yaml:"enabled"`
		Address  string `yaml:"address"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`

	Cameras []struct {
		ID  string `yaml:"id"`
		URL string `yaml:"url"`
	} `yaml:"cameras"`
}

// TestEnvironmentSetup valida que o ambiente está pronto para testes E2E
func TestEnvironmentSetup(t *testing.T) {
	t.Run("Binary Exists", func(t *testing.T) {
		binaryPaths := []string{
			"bin/producer.exe",
			"../../bin/producer.exe",
			"../bin/producer.exe",
		}
		if runtime.GOOS != "windows" {
			binaryPaths = []string{
				"bin/producer",
				"../../bin/producer",
				"../bin/producer",
			}
		}

		var binaryPath string
		for _, path := range binaryPaths {
			if _, err := os.Stat(path); err == nil {
				binaryPath = path
				break
			}
		}

		if binaryPath == "" {
			t.Fatalf("❌ Producer binary not found in: %v\n"+
				"   Run: go build -o bin/producer.exe ./cmd/producer", binaryPaths)
		}

		t.Logf("✅ Producer binary found: %s", binaryPath)
	})

	t.Run("Config File Exists", func(t *testing.T) {
		configPath, err := FindConfigFile("config.yaml")
		if err != nil {
			t.Fatalf("❌ Config file not found: %v", err)
		}

		t.Logf("✅ Config file found: %s", configPath)
	})

	t.Run("Config File Valid", func(t *testing.T) {
		configPath, err := FindConfigFile("config.yaml")
		if err != nil {
			t.Fatalf("❌ Config file not found: %v", err)
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("❌ Failed to read config: %v", err)
		}

		var config Config
		if err := yaml.Unmarshal(data, &config); err != nil {
			t.Fatalf("❌ Failed to parse config: %v", err)
		}

		// Validar campos obrigatórios
		if config.FPS == 0 {
			t.Error("❌ FPS not configured")
		} else {
			t.Logf("✅ FPS configured: %d", config.FPS)
		}

		if config.AMQP.URL == "" {
			t.Error("❌ AMQP URL not configured")
		} else {
			t.Logf("✅ AMQP URL configured")
		}

		if !config.Redis.Enabled {
			t.Error("❌ Redis not enabled (required for E2E tests)")
		} else {
			t.Logf("✅ Redis enabled")
		}

		if len(config.Cameras) == 0 {
			t.Error("❌ No cameras configured")
		} else {
			t.Logf("✅ %d cameras configured", len(config.Cameras))
		}
	})

	t.Run("Redis Connection", func(t *testing.T) {
		// Ler config para obter credenciais
		configPath, err := FindConfigFile("config.yaml")
		if err != nil {
			t.Skip("Config file not found, skipping Redis test")
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Skip("Config file not found, skipping Redis test")
		}

		var config Config
		if err := yaml.Unmarshal(data, &config); err != nil {
			t.Skip("Failed to parse config, skipping Redis test")
		}

		if !config.Redis.Enabled {
			t.Skip("Redis not enabled in config")
		}

		// Testar conexão
		err = TestRedisConnection(config.Redis.Address, config.Redis.Password, config.Redis.DB)
		if err != nil {
			t.Errorf("❌ Redis connection failed: %v", err)
		} else {
			t.Logf("✅ Redis connection successful: %s", config.Redis.Address)
		}
	})

	t.Run("RabbitMQ Connection", func(t *testing.T) {
		// Ler config
		configPath, err := FindConfigFile("config.yaml")
		if err != nil {
			t.Skip("Config file not found, skipping RabbitMQ test")
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Skip("Config file not found, skipping RabbitMQ test")
		}

		var config Config
		if err := yaml.Unmarshal(data, &config); err != nil {
			t.Skip("Failed to parse config, skipping RabbitMQ test")
		}

		// Testar conexão
		err = TestRabbitMQConnection(config.AMQP.URL)
		if err != nil {
			t.Errorf("❌ RabbitMQ connection failed: %v", err)
		} else {
			t.Logf("✅ RabbitMQ connection successful")
		}
	})

	t.Run("Working Directory", func(t *testing.T) {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get working directory: %v", err)
		}

		// Deve estar em v2/tests/e2e ou v2/
		baseName := filepath.Base(wd)
		if baseName != "e2e" && baseName != "v2" {
			t.Logf("⚠️  Working directory: %s", wd)
			t.Log("⚠️  Tests should run from v2/ directory: cd v2 && go test ./tests/e2e")
		} else {
			t.Logf("✅ Working directory OK: %s", wd)
		}
	})

	t.Run("System Resources", func(t *testing.T) {
		// Verificar memória disponível
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		t.Logf("📊 System Resources:")
		t.Logf("   Goroutines: %d", runtime.NumGoroutine())
		t.Logf("   Memory Alloc: %.2f MB", float64(m.Alloc)/1024/1024)
		t.Logf("   Memory Total: %.2f MB", float64(m.TotalAlloc)/1024/1024)
		t.Logf("   GC Cycles: %d", m.NumGC)
		t.Logf("   CPUs: %d", runtime.NumCPU())

		t.Log("✅ System resources check complete")
	})
}

// TestConfigLoadCameras valida que todas as câmeras estão configuradas corretamente
func TestConfigLoadCameras(t *testing.T) {
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

	expectedCameras := []string{"cam1", "cam2", "cam3", "cam4", "cam5"}

	for _, expectedID := range expectedCameras {
		t.Run(fmt.Sprintf("Camera_%s", expectedID), func(t *testing.T) {
			found := false
			var cameraURL string

			for _, cam := range config.Cameras {
				if cam.ID == expectedID {
					found = true
					cameraURL = cam.URL
					break
				}
			}

			if !found {
				t.Errorf("❌ Camera %s not found in config", expectedID)
			} else {
				protocol := "UNKNOWN"
				if len(cameraURL) > 0 {
					if cameraURL[:4] == "rtmp" {
						protocol = "RTMP"
					} else if cameraURL[:4] == "rtsp" {
						protocol = "RTSP"
					}
				}

				t.Logf("✅ Camera %s configured (%s)", expectedID, protocol)
			}
		})
	}
}
