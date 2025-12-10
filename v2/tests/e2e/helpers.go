package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	amqp "github.com/rabbitmq/amqp091-go"
)

// ProducerProcess representa um processo do producer em execução
type ProducerProcess struct {
	cmd        *exec.Cmd
	configPath string
	stdout     strings.Builder
	stderr     strings.Builder
	mu         sync.Mutex
}

// ResourceMonitor monitora recursos do sistema
type ResourceMonitor struct {
	samples []ResourceSample
	mu      sync.Mutex
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// ResourceSample representa uma amostra de recursos
type ResourceSample struct {
	Timestamp   time.Time
	MemoryMB    float64
	Goroutines  int
	CPUPercent  float64
	NumGC       uint32
}

// StartProducer inicia o producer com config específico
func StartProducer(configPath string) (*ProducerProcess, error) {
	// Encontrar binário (procurar em múltiplos locais)
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
		return nil, fmt.Errorf("producer binary not found in: %v (run: go build -o bin/producer.exe ./cmd/producer)",
			binaryPaths)
	}

	// Criar comando
	cmd := exec.Command(binaryPath, "-config", configPath)

	proc := &ProducerProcess{
		cmd:        cmd,
		configPath: configPath,
	}

	// Capturar stdout/stderr
	cmd.Stdout = &proc.stdout
	cmd.Stderr = &proc.stderr

	// Iniciar
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start producer: %w", err)
	}

	// Aguardar inicialização (2s)
	time.Sleep(2 * time.Second)

	return proc, nil
}

// Stop para o producer gracefully
func (p *ProducerProcess) Stop() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	// No Windows, SIGTERM não é suportado, usar Kill() diretamente
	if runtime.GOOS == "windows" {
		if err := p.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
		p.cmd.Wait()
		return nil
	}

	// Unix-like: tentar SIGTERM primeiro (graceful shutdown)
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// Se SIGTERM falhar, tentar SIGKILL
		p.cmd.Process.Kill()
		return fmt.Errorf("failed to send SIGTERM, killed process: %w", err)
	}

	// Aguardar até 10s para processo terminar
	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		p.cmd.Process.Kill()
		return fmt.Errorf("producer did not stop gracefully, killed")
	}
}

// GetOutput retorna stdout e stderr capturados
func (p *ProducerProcess) GetOutput() (stdout, stderr string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stdout.String(), p.stderr.String()
}

// IsRunning verifica se processo ainda está rodando
func (p *ProducerProcess) IsRunning() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}

	// No Windows, verificar se processo existe
	if runtime.GOOS == "windows" {
		err := p.cmd.Process.Signal(syscall.Signal(0))
		return err == nil
	}

	// Unix-like
	err := p.cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}

// NewResourceMonitor cria um novo monitor de recursos
func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{
		samples: make([]ResourceSample, 0),
		stopCh:  make(chan struct{}),
	}
}

// Start inicia monitoramento de recursos
func (rm *ResourceMonitor) Start(interval time.Duration) {
	rm.wg.Add(1)
	go func() {
		defer rm.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-rm.stopCh:
				return
			case <-ticker.C:
				rm.collectSample()
			}
		}
	}()
}

// Stop para monitoramento
func (rm *ResourceMonitor) Stop() {
	close(rm.stopCh)
	rm.wg.Wait()
}

// collectSample coleta uma amostra de recursos
func (rm *ResourceMonitor) collectSample() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	sample := ResourceSample{
		Timestamp:   time.Now(),
		MemoryMB:    float64(m.Alloc) / 1024 / 1024,
		Goroutines:  runtime.NumGoroutine(),
		CPUPercent:  0, // TODO: Implementar CPU sampling se necessário
		NumGC:       m.NumGC,
	}

	rm.mu.Lock()
	rm.samples = append(rm.samples, sample)
	rm.mu.Unlock()
}

// GetSamples retorna todas as amostras coletadas
func (rm *ResourceMonitor) GetSamples() []ResourceSample {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Criar cópia
	result := make([]ResourceSample, len(rm.samples))
	copy(result, rm.samples)
	return result
}

// GetStats retorna estatísticas resumidas
func (rm *ResourceMonitor) GetStats() ResourceStats {
	samples := rm.GetSamples()

	if len(samples) == 0 {
		return ResourceStats{}
	}

	stats := ResourceStats{
		SampleCount: len(samples),
	}

	// Calcular min/max/avg de memória
	var totalMemory float64
	stats.MinMemoryMB = samples[0].MemoryMB
	stats.MaxMemoryMB = samples[0].MemoryMB

	for _, s := range samples {
		totalMemory += s.MemoryMB
		if s.MemoryMB < stats.MinMemoryMB {
			stats.MinMemoryMB = s.MemoryMB
		}
		if s.MemoryMB > stats.MaxMemoryMB {
			stats.MaxMemoryMB = s.MemoryMB
		}
		if s.Goroutines > stats.MaxGoroutines {
			stats.MaxGoroutines = s.Goroutines
		}
	}

	stats.AvgMemoryMB = totalMemory / float64(len(samples))

	// GC count
	if len(samples) > 0 {
		stats.GCCount = int(samples[len(samples)-1].NumGC - samples[0].NumGC)
	}

	return stats
}

// ResourceStats representa estatísticas de recursos
type ResourceStats struct {
	SampleCount    int
	MinMemoryMB    float64
	MaxMemoryMB    float64
	AvgMemoryMB    float64
	MaxGoroutines  int
	GCCount        int
}

// TestRedisConnection testa conexão com Redis
func TestRedisConnection(address, password string, db int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
		DB:       db,
	})
	defer client.Close()

	// Ping
	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	return nil
}

// TestRabbitMQConnection testa conexão com RabbitMQ
func TestRabbitMQConnection(url string) error {
	conn, err := amqp.Dial(url)
	if err != nil {
		return fmt.Errorf("rabbitmq connection failed: %w", err)
	}
	defer conn.Close()

	// Criar channel de teste
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("rabbitmq channel creation failed: %w", err)
	}
	defer ch.Close()

	return nil
}

// CountFramesInRedis conta frames de uma câmera no Redis
func CountFramesInRedis(address, password string, db int, cameraID string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
		DB:       db,
	})
	defer client.Close()

	// Pattern: frames:cam1:*
	pattern := fmt.Sprintf("frames:%s:*", cameraID)

	keys, err := client.Keys(ctx, pattern).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get keys: %w", err)
	}

	return len(keys), nil
}

// GetLatestFrameFromRedis obtém o frame mais recente de uma câmera
func GetLatestFrameFromRedis(address, password string, db int, cameraID string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
		DB:       db,
	})
	defer client.Close()

	// Pattern: frames:cam1:*
	pattern := fmt.Sprintf("frames:%s:*", cameraID)

	keys, err := client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no frames found for camera %s", cameraID)
	}

	// Pegar última chave (maior timestamp)
	latestKey := keys[len(keys)-1]

	data, err := client.Get(ctx, latestKey).Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get frame data: %w", err)
	}

	return data, nil
}

// WaitForCondition aguarda até que uma condição seja verdadeira
func WaitForCondition(timeout time.Duration, checkInterval time.Duration, condition func() bool) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if condition() {
			return nil
		}
		time.Sleep(checkInterval)
	}

	return fmt.Errorf("condition not met within timeout %v", timeout)
}

// FormatDuration formata duração de forma legível
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

// FormatBytes formata bytes de forma legível
func FormatBytes(bytes float64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%.0fB", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fKB", bytes/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB", bytes/1024/1024)
	}
	return fmt.Sprintf("%.2fGB", bytes/1024/1024/1024)
}

// FindConfigFile encontra config.yaml procurando em múltiplos locais
func FindConfigFile(filename string) (string, error) {
	configPaths := []string{
		filename,
		"../../" + filename,
		"../" + filename,
	}

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("config file not found in: %v", configPaths)
}
