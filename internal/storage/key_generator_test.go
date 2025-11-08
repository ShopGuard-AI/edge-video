package storage

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewKeyGenerator(t *testing.T) {
	tests := []struct {
		name     string
		config   KeyGeneratorConfig
		wantVhost string
		wantStrategy KeyStrategy
	}{
		{
			name: "configuração completa",
			config: KeyGeneratorConfig{
				Strategy: StrategySequence,
				Prefix:   "frames",
				Vhost:    "myvhost",
			},
			wantVhost: "myvhost",
			wantStrategy: StrategySequence,
		},
		{
			name: "usa defaults quando não especificado",
			config: KeyGeneratorConfig{
				Prefix: "frames",
			},
			wantVhost: "default",
			wantStrategy: StrategySequence,
		},
		{
			name: "vhost customizado",
			config: KeyGeneratorConfig{
				Strategy: StrategyUUID,
				Prefix:   "test",
				Vhost:    "client-123",
			},
			wantVhost: "client-123",
			wantStrategy: StrategyUUID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kg := NewKeyGenerator(tt.config)
			
			if kg.config.Vhost != tt.wantVhost {
				t.Errorf("NewKeyGenerator() vhost = %v, want %v", kg.config.Vhost, tt.wantVhost)
			}
			
			if kg.config.Strategy != tt.wantStrategy {
				t.Errorf("NewKeyGenerator() strategy = %v, want %v", kg.config.Strategy, tt.wantStrategy)
			}
		})
	}
}

func TestGenerateKey_WithVhost(t *testing.T) {
	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	
	tests := []struct {
		name       string
		config     KeyGeneratorConfig
		cameraID   string
		wantPrefix string
		wantVhost  string
	}{
		{
			name: "vhost personalizado",
			config: KeyGeneratorConfig{
				Strategy: StrategySequence,
				Prefix:   "frames",
				Vhost:    "client-a",
			},
			cameraID:   "cam1",
			wantPrefix: "client-a:frames:cam1:",
			wantVhost:  "client-a",
		},
		{
			name: "vhost diferente",
			config: KeyGeneratorConfig{
				Strategy: StrategySequence,
				Prefix:   "frames",
				Vhost:    "client-b",
			},
			cameraID:   "cam1",
			wantPrefix: "client-b:frames:cam1:",
			wantVhost:  "client-b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kg := NewKeyGenerator(tt.config)
			key := kg.GenerateKey(tt.cameraID, timestamp)

			if !strings.HasPrefix(key, tt.wantPrefix) {
				t.Errorf("GenerateKey() = %v, want prefix %v", key, tt.wantPrefix)
			}

			// Verifica se o vhost está no lugar correto (primeiro componente)
			parts := strings.Split(key, ":")
			if len(parts) < 3 {
				t.Fatalf("chave inválida: %v", key)
			}
			if parts[0] != tt.wantVhost {
				t.Errorf("vhost na chave = %v, want %v", parts[0], tt.wantVhost)
			}
		})
	}
}

func TestGenerateKey_Strategies(t *testing.T) {
	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	vhost := "test-vhost"
	
	tests := []struct {
		name     string
		strategy KeyStrategy
		validate func(string) bool
	}{
		{
			name:     "basic strategy",
			strategy: StrategyBasic,
			validate: func(key string) bool {
				// vhost:prefix:camera:unix_timestamp
				parts := strings.Split(key, ":")
				if len(parts) != 4 {
					return false
				}
				// Verifica se timestamp é numérico
				_, err := fmt.Sscanf(parts[3], "%d", new(int64))
				return err == nil
			},
		},
		{
			name:     "sequence strategy",
			strategy: StrategySequence,
			validate: func(key string) bool {
				// vhost:prefix:camera:unix_timestamp:seq (5 dígitos no final)
				parts := strings.Split(key, ":")
				if len(parts) != 5 {
					return false
				}
				lastPart := parts[4]
				return len(lastPart) == 5 && strings.HasPrefix(key, "test-vhost:frames:cam1:")
			},
		},
		{
			name:     "uuid strategy",
			strategy: StrategyUUID,
			validate: func(key string) bool {
				// vhost:prefix:camera:unix_timestamp:uuid (8 caracteres no final)
				parts := strings.Split(key, ":")
				if len(parts) != 5 {
					return false
				}
				lastPart := parts[4]
				return len(lastPart) == 8 && strings.HasPrefix(key, "test-vhost:frames:cam1:")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kg := NewKeyGenerator(KeyGeneratorConfig{
				Strategy: tt.strategy,
				Prefix:   "frames",
				Vhost:    vhost,
			})
			
			key := kg.GenerateKey("cam1", timestamp)
			
			if !tt.validate(key) {
				t.Errorf("GenerateKey() = %v, validação falhou", key)
			}
		})
	}
}

func TestGenerateKey_NoCollisions(t *testing.T) {
	kg := NewKeyGenerator(KeyGeneratorConfig{
		Strategy: StrategySequence,
		Prefix:   "frames",
		Vhost:    "test",
	})

	timestamp := time.Now()
	keys := make(map[string]bool)
	
	// Gera 1000 chaves no mesmo timestamp
	for i := 0; i < 1000; i++ {
		key := kg.GenerateKey("cam1", timestamp)
		if keys[key] {
			t.Fatalf("Colisão detectada na chave: %s", key)
		}
		keys[key] = true
	}
}

func TestGenerateKey_ConcurrentAccess(t *testing.T) {
	kg := NewKeyGenerator(KeyGeneratorConfig{
		Strategy: StrategySequence,
		Prefix:   "frames",
		Vhost:    "concurrent-test",
	})

	timestamp := time.Now()
	keys := make(map[string]bool)
	mu := sync.Mutex{}
	
	var wg sync.WaitGroup
	numGoroutines := 100
	keysPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < keysPerGoroutine; j++ {
				key := kg.GenerateKey("cam1", timestamp)
				mu.Lock()
				if keys[key] {
					t.Errorf("Colisão detectada em acesso concurrent: %s", key)
				}
				keys[key] = true
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	expectedKeys := numGoroutines * keysPerGoroutine
	if len(keys) != expectedKeys {
		t.Errorf("Esperado %d chaves únicas, obteve %d", expectedKeys, len(keys))
	}
}

func TestParseKey(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		wantVhost string
		wantError bool
	}{
		{
			name:      "chave válida com sequence",
			key:       "myvhost:frames:cam1:1705317000000000000:00001",
			wantVhost: "myvhost",
			wantError: false,
		},
		{
			name:      "chave válida básica",
			key:       "client-a:frames:cam2:1705317000000000000",
			wantVhost: "client-a",
			wantError: false,
		},
		{
			name:      "supermercado example",
			key:       "supermercado_vhost:frames:cam4:1731024000123456789:00001",
			wantVhost: "supermercado_vhost",
			wantError: false,
		},
		{
			name:      "chave inválida - poucos componentes",
			key:       "frames:cam1",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kg := NewKeyGenerator(KeyGeneratorConfig{
				Strategy: StrategySequence,
				Prefix:   "frames",
				Vhost:    "test",
			})

			components, err := kg.ParseKey(tt.key)
			
			if tt.wantError {
				if err == nil {
					t.Error("ParseKey() esperava erro, obteve nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseKey() erro inesperado: %v", err)
				return
			}

			if components.Vhost != tt.wantVhost {
				t.Errorf("ParseKey() vhost = %v, want %v", components.Vhost, tt.wantVhost)
			}
		})
	}
}

func TestQueryPattern(t *testing.T) {
	tests := []struct {
		name     string
		vhost    string
		cameraID string
		want     string
	}{
		{
			name:     "todos os frames de um vhost",
			vhost:    "client-a",
			cameraID: "",
			want:     "client-a:frames:*",
		},
		{
			name:     "frames de uma câmera específica",
			vhost:    "client-b",
			cameraID: "cam1",
			want:     "client-b:frames:cam1:*",
		},
		{
			name:     "usa vhost configurado se não especificado",
			vhost:    "",
			cameraID: "cam2",
			want:     "default-vhost:frames:cam2:*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configVhost := "default-vhost"
			if tt.vhost != "" {
				configVhost = tt.vhost
			}

			kg := NewKeyGenerator(KeyGeneratorConfig{
				Strategy: StrategySequence,
				Prefix:   "frames",
				Vhost:    configVhost,
			})

			got := kg.QueryPattern(tt.cameraID, tt.vhost)
			if got != tt.want {
				t.Errorf("QueryPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVhostIsolation(t *testing.T) {
	// Simula dois clientes diferentes
	kg1 := NewKeyGenerator(KeyGeneratorConfig{
		Strategy: StrategySequence,
		Prefix:   "frames",
		Vhost:    "client-a",
	})

	kg2 := NewKeyGenerator(KeyGeneratorConfig{
		Strategy: StrategySequence,
		Prefix:   "frames",
		Vhost:    "client-b",
	})

	timestamp := time.Now()
	
	// Mesma câmera, mesmo timestamp
	key1 := kg1.GenerateKey("cam1", timestamp)
	key2 := kg2.GenerateKey("cam1", timestamp)

	// As chaves devem ser diferentes devido ao vhost diferente
	if key1 == key2 {
		t.Error("Chaves de diferentes vhosts não devem ser iguais")
	}

	// Verifica que os vhosts estão corretos nas chaves (primeiro componente agora)
	parts1 := strings.Split(key1, ":")
	parts2 := strings.Split(key2, ":")

	if parts1[0] != "client-a" {
		t.Errorf("key1 vhost = %v, want client-a", parts1[0])
	}

	if parts2[0] != "client-b" {
		t.Errorf("key2 vhost = %v, want client-b", parts2[0])
	}
}

// Benchmarks
func BenchmarkGenerateKey_Sequence(b *testing.B) {
	kg := NewKeyGenerator(KeyGeneratorConfig{
		Strategy: StrategySequence,
		Prefix:   "frames",
		Vhost:    "bench",
	})
	timestamp := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kg.GenerateKey("cam1", timestamp)
	}
}

func BenchmarkGenerateKey_UUID(b *testing.B) {
	kg := NewKeyGenerator(KeyGeneratorConfig{
		Strategy: StrategyUUID,
		Prefix:   "frames",
		Vhost:    "bench",
	})
	timestamp := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kg.GenerateKey("cam1", timestamp)
	}
}

func BenchmarkGenerateKey_Concurrent(b *testing.B) {
	kg := NewKeyGenerator(KeyGeneratorConfig{
		Strategy: StrategySequence,
		Prefix:   "frames",
		Vhost:    "bench",
	})
	timestamp := time.Now()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			kg.GenerateKey("cam1", timestamp)
		}
	})
}

func ExampleKeyGenerator_GenerateKey() {
	kg := NewKeyGenerator(KeyGeneratorConfig{
		Strategy: StrategySequence,
		Prefix:   "frames",
		Vhost:    "client-production",
	})

	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	key := kg.GenerateKey("cam1", timestamp)

	// A chave inclui o vhost para isolamento entre clientes
	fmt.Println("Formato da chave:", strings.Split(key, ":")[0:3])
	// Output: Formato da chave: [client-production frames cam1]
}
