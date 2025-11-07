# Análise de Performance e Capacidade

## 🎯 Resumo Executivo

**Capacidade Atual Estimada:** 15-30 câmeras simultâneas  
**FPS Configurado:** 30 FPS por câmera  
**Gargalos Identificados:** 8 pontos críticos  
**Melhorias Propostas:** 12 otimizações que podem aumentar para 100+ câmeras

---

## 📊 Análise de Capacidade Atual

### Configuração Atual
```yaml
target_fps: 30  # 30 frames por segundo por câmera
cameras: 5      # 5 câmeras configuradas
```

### Cálculo de Throughput

**Por Câmera (30 FPS):**
- Intervalo entre frames: ~33ms
- Tempo de captura FFmpeg: ~50-100ms (variável)
- Tempo de processamento: ~10-20ms
- **Total por frame: ~100ms**
- **Throughput real: ~10 FPS efetivo**

**Sistema Completo:**
```
5 câmeras × 10 FPS = 50 frames/segundo total
Latência média: 100-200ms por frame
CPU: ~40-60% (5 câmeras)
Memória: ~200-500MB (5 câmeras)
```

### Estimativa de Escalabilidade

| Câmeras | FPS Real | CPU Estimado | RAM Estimada | Status |
|---------|----------|--------------|--------------|--------|
| 5       | 10 FPS   | 40-60%       | 300 MB       | ✅ OK |
| 10      | 8-10 FPS | 70-80%       | 600 MB       | ⚠️ Limite |
| 15      | 6-8 FPS  | 85-95%       | 900 MB       | ❌ Crítico |
| 20+     | <5 FPS   | 100%         | 1.2+ GB      | ❌ Inviável |

**Conclusão:** Com a arquitetura atual, o sistema suporta **15-20 câmeras no máximo** antes de degradação severa.

---

## 🔍 Gargalos Identificados

### 1. ⚠️ Captura Síncrona com FFmpeg (CRÍTICO)

**Problema:**
```go
// pkg/camera/camera.go:68
cmd := exec.CommandContext(c.ctx, "ffmpeg", ...)
err := cmd.Run()  // BLOQUEANTE - aguarda FFmpeg terminar
```

**Impacto:**
- Cada captura bloqueia por 50-100ms
- Processo FFmpeg criado/destruído a cada frame
- Overhead de fork/exec enorme
- CPU 100% com 15+ câmeras

**Solução Proposta:**
```go
// Usar FFmpeg em modo streaming persistente
type PersistentCapture struct {
    cmd    *exec.Cmd
    stdout io.ReadCloser
    stdin  io.WriteCloser
}

func (pc *PersistentCapture) CaptureFrame() ([]byte, error) {
    // FFmpeg já rodando, apenas lê próximo frame
    return pc.readNextFrame()
}
```

**Ganho Esperado:** 3-5x mais câmeras (30-50 câmeras)

---

### 2. ⚠️ Goroutines Ilimitadas (CRÍTICO)

**Problema:**
```go
// pkg/camera/camera.go:111
if c.redisStore.Enabled() {
    go func() {  // Nova goroutine POR FRAME
        // Salva no Redis
        // Publica metadata
    }()
}
```

**Impacto:**
- 5 câmeras × 10 FPS = 50 goroutines/segundo
- 20 câmeras × 10 FPS = 200 goroutines/segundo
- Memory leak potencial
- Overhead de scheduling

**Solução Proposta:**
```go
// Worker pool pattern
type WorkerPool struct {
    jobs    chan FrameJob
    workers int
}

func NewWorkerPool(workers int) *WorkerPool {
    wp := &WorkerPool{
        jobs:    make(chan FrameJob, 1000),
        workers: workers,
    }
    
    for i := 0; i < workers; i++ {
        go wp.worker()
    }
    
    return wp
}

func (wp *WorkerPool) worker() {
    for job := range wp.jobs {
        job.Process()
    }
}
```

**Ganho Esperado:** 2x mais câmeras (30-40 câmeras)

---

### 3. ⚠️ Ausência de Buffer/Queue (ALTO)

**Problema:**
- Sem fila de frames pendentes
- Frames descartados se processamento atrasa
- Picos de latência não são absorvidos

**Solução Proposta:**
```go
type FrameBuffer struct {
    frames chan Frame
    size   int
}

func NewFrameBuffer(size int) *FrameBuffer {
    return &FrameBuffer{
        frames: make(chan Frame, size),
        size:   size,
    }
}

func (fb *FrameBuffer) Push(frame Frame) error {
    select {
    case fb.frames <- frame:
        return nil
    default:
        // Buffer cheio, pode descartar frame mais antigo
        <-fb.frames
        fb.frames <- frame
        return ErrBufferFull
    }
}
```

**Ganho Esperado:** Redução de 50% em frames perdidos

---

### 4. ⚠️ Timestamp Hardcoded (MÉDIO)

**Problema:**
```go
// pkg/camera/camera.go:115
width, height := 1280, 720  // TODO: Obter do frame real
```

**Impacto:**
- Metadata imprecisa
- Não detecta mudanças de resolução
- Impossível otimizar por resolução

**Solução Proposta:**
```go
func extractFrameInfo(data []byte) (width, height int, err error) {
    // Usar biblioteca de imagem para detectar dimensões
    img, _, err := image.DecodeConfig(bytes.NewReader(data))
    if err != nil {
        return 0, 0, err
    }
    return img.Width, img.Height, nil
}
```

**Ganho Esperado:** Metadata precisa, otimizações futuras

---

### 5. ⚠️ Logging Excessivo (MÉDIO)

**Problema:**
```go
log.Printf("capturado frame da camera %s (%d bytes)", c.config.ID, len(frameData))
// Log a cada frame = 50+ logs/segundo com 5 câmeras
```

**Impacto:**
- I/O disk intensivo
- CPU desperdiçada em formatação
- Logs gigantes

**Solução Proposta:**
```go
// Usar níveis de log e sampling
if frameCount%100 == 0 {  // Log apenas 1 a cada 100 frames
    logger.Debug("Stats",
        zap.String("camera", c.config.ID),
        zap.Int("frames", frameCount),
        zap.Duration("avg_latency", avgLatency))
}
```

**Ganho Esperado:** 10-15% CPU liberada

---

### 6. ⚠️ Compressão Não Otimizada (MÉDIO)

**Problema:**
```yaml
compression:
  enabled: false  # Desabilitado na config atual
```

**Impacto:**
- Frames JPEG sem otimização
- Tamanho típico: 50-200 KB/frame
- Bandwidth RabbitMQ: 5 câmeras × 10 FPS × 100 KB = 5 MB/s

**Solução Proposta:**
```go
// Ajustar qualidade JPEG dinamicamente
func (c *Capture) optimizeQuality(bandwidth float64) int {
    if bandwidth > 10.0 {  // MB/s
        return 5  // Alta qualidade
    } else if bandwidth > 5.0 {
        return 10  // Média qualidade
    }
    return 15  // Baixa qualidade
}
```

**Ganho Esperado:** 30-50% redução de bandwidth

---

### 7. ⚠️ Redis TTL Fixo (BAIXO)

**Problema:**
```yaml
redis:
  ttl_seconds: 300  # 5 minutos fixo
```

**Impacto:**
- Frames antigos ocupam memória desnecessariamente
- Redis pode ficar saturado com muitas câmeras

**Solução Proposta:**
```go
// TTL dinâmico baseado em uso
func (rs *RedisStore) calculateTTL(cameraID string) time.Duration {
    accessFreq := rs.getAccessFrequency(cameraID)
    
    if accessFreq > 10 {  // Acessos por minuto
        return 10 * time.Minute  // Câmera muito acessada
    } else if accessFreq > 1 {
        return 5 * time.Minute
    }
    return 1 * time.Minute  // Câmera pouco acessada
}
```

**Ganho Esperado:** 40% redução de uso de memória Redis

---

### 8. ⚠️ Ausência de Circuit Breaker (ALTO)

**Problema:**
- Sem proteção contra falhas em cascata
- Uma câmera offline pode afetar outras
- Reconnection storms ao RabbitMQ/Redis

**Solução Proposta:**
```go
type CircuitBreaker struct {
    maxFailures int
    timeout     time.Duration
    state       State  // Closed, Open, HalfOpen
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    if cb.state == Open {
        if time.Since(cb.lastFailure) > cb.timeout {
            cb.state = HalfOpen
        } else {
            return ErrCircuitOpen
        }
    }
    
    err := fn()
    if err != nil {
        cb.failures++
        if cb.failures >= cb.maxFailures {
            cb.state = Open
            cb.lastFailure = time.Now()
        }
    } else if cb.state == HalfOpen {
        cb.state = Closed
        cb.failures = 0
    }
    
    return err
}
```

**Ganho Esperado:** 99% uptime, resilência a falhas

---

## 🚀 Plano de Otimização Recomendado

### Fase 1: Quick Wins (1-2 dias)

#### 1.1 Implementar Worker Pool
```go
// cmd/edge-video/main.go
workerPool := NewWorkerPool(runtime.NumCPU() * 2)

for _, camCfg := range cfg.Cameras {
    capture := camera.NewCapture(
        ctx,
        camera.Config{ID: camCfg.ID, URL: camCfg.URL},
        interval,
        compressor,
        publisher,
        redisStore,
        metaPublisher,
        workerPool,  // <-- Novo parâmetro
    )
    capture.Start()
}
```

**Resultado:** 2x capacidade (30 câmeras)

#### 1.2 Reduzir Logging
```go
// Usar structured logging com níveis
logger := zap.NewProduction()
defer logger.Sync()

// Apenas erros e warnings em produção
if err != nil {
    logger.Error("capture failed",
        zap.String("camera", c.config.ID),
        zap.Error(err))
}
```

**Resultado:** 10% CPU liberada

#### 1.3 Adicionar Frame Buffer
```go
type Capture struct {
    // ...campos existentes...
    frameBuffer *FrameBuffer
}

func (c *Capture) captureAndPublish() {
    // ...captura frame...
    
    // Enfileira ao invés de processar imediatamente
    c.frameBuffer.Push(Frame{
        CameraID: c.config.ID,
        Data:     frameData,
        Timestamp: time.Now(),
    })
}
```

**Resultado:** 50% menos frames perdidos

---

### Fase 2: Otimizações Médias (3-5 dias)

#### 2.1 FFmpeg Persistente
```go
type PersistentFFmpeg struct {
    cmd       *exec.Cmd
    stdout    *bufio.Reader
    frameChan chan []byte
}

func (pf *PersistentFFmpeg) Start(url string) error {
    pf.cmd = exec.Command("ffmpeg",
        "-rtsp_transport", "tcp",
        "-i", url,
        "-f", "image2pipe",
        "-vcodec", "mjpeg",
        "-q:v", "5",
        "-r", "10",  // 10 FPS fixo
        "-",
    )
    
    stdout, _ := pf.cmd.StdoutPipe()
    pf.stdout = bufio.NewReader(stdout)
    
    go pf.readFrames()
    return pf.cmd.Start()
}

func (pf *PersistentFFmpeg) readFrames() {
    for {
        frame, err := pf.readJPEG()
        if err != nil {
            break
        }
        pf.frameChan <- frame
    }
}
```

**Resultado:** 3-5x capacidade (50-100 câmeras)

#### 2.2 Compressão Adaptativa
```go
func (c *Capture) adaptiveCompress(data []byte) []byte {
    size := len(data)
    
    if size > 200*1024 {  // > 200 KB
        // Alta compressão
        return c.compressor.Compress(data, 9)
    } else if size > 100*1024 {  // > 100 KB
        // Média compressão
        return c.compressor.Compress(data, 5)
    }
    // Sem compressão para frames pequenos
    return data
}
```

**Resultado:** 40% redução bandwidth

#### 2.3 Circuit Breaker
```go
type Capture struct {
    // ...campos existentes...
    circuitBreaker *CircuitBreaker
}

func (c *Capture) captureAndPublish() {
    err := c.circuitBreaker.Call(func() error {
        return c.doCapture()
    })
    
    if err == ErrCircuitOpen {
        log.Printf("circuit open for camera %s, skipping", c.config.ID)
        return
    }
}
```

**Resultado:** Sistema resiliente a falhas

---

### Fase 3: Arquitetura Avançada (1-2 semanas)

#### 3.1 Distributed Processing
```go
// Separar captura de processamento
type CaptureService struct {
    cameras []*Camera
    queue   *DistributedQueue  // Redis Streams ou Kafka
}

type ProcessingService struct {
    queue     *DistributedQueue
    workers   []*Worker
}

// Permite escalar horizontalmente:
// - 1 instância de CaptureService
// - N instâncias de ProcessingService
```

**Resultado:** 200+ câmeras com múltiplos nós

#### 3.2 Metrics e Observabilidade
```go
// Prometheus metrics
var (
    framesProcessed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "frames_processed_total",
        },
        []string{"camera_id"},
    )
    
    captureLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "capture_latency_seconds",
        },
        []string{"camera_id"},
    )
)
```

**Resultado:** Visibilidade total, auto-scaling informado

#### 3.3 GPU Acceleration (Opcional)
```go
// Usar GPU para decode/encode se disponível
import "github.com/giorgisio/goav/avcodec"

type GPUDecoder struct {
    codec *avcodec.Codec
}

func (gd *GPUDecoder) DecodeFrame(data []byte) (*Frame, error) {
    // Decode em GPU usando NVDEC/QuickSync
    return gd.decode(data)
}
```

**Resultado:** 500+ câmeras com GPU dedicada

---

## 📈 Roadmap de Capacidade

### Hoje (Baseline)
```
Arquitetura: Atual
Câmeras: 15-20
FPS Real: 6-10
CPU: 100%
Status: ⚠️ Limite técnico
```

### Após Fase 1 (Quick Wins)
```
Melhorias: Worker Pool + Buffer + Less Logging
Câmeras: 30-40
FPS Real: 8-10
CPU: 80%
Status: ✅ Produção estável
Esforço: 2 dias
```

### Após Fase 2 (Otimizações)
```
Melhorias: FFmpeg Persistente + Circuit Breaker
Câmeras: 50-100
FPS Real: 8-10
CPU: 70%
Status: ✅ Alta capacidade
Esforço: 1 semana
```

### Após Fase 3 (Arquitetura Avançada)
```
Melhorias: Distributed + GPU
Câmeras: 200+
FPS Real: 10-30
CPU: 60% (distribuído)
Status: ✅ Enterprise grade
Esforço: 2 semanas
```

---

## 🎯 Recomendações Imediatas

### Para Produção Hoje:
1. **Reduzir FPS para 5-10:** Mais realista e sustentável
2. **Implementar Worker Pool:** 2 dias de trabalho, 2x capacidade
3. **Adicionar Monitoring:** Prometheus + Grafana

### Para Escalar (Próximos 30 dias):
1. **FFmpeg Persistente:** Maior impacto na capacidade
2. **Circuit Breaker:** Essencial para produção
3. **Frame Buffer:** Reduz perda de frames

### Para Long-Term:
1. **Arquitetura Distribuída:** Se precisar 100+ câmeras
2. **GPU Acceleration:** Para casos extremos (500+ câmeras)
3. **Edge Computing:** Processar localmente antes de enviar

---

## 📊 Benchmarks Sugeridos

```bash
# Teste de carga com 1 câmera
go test -bench=BenchmarkSingleCamera -benchtime=60s

# Teste de carga com N câmeras
go test -bench=BenchmarkMultipleCamera -benchtime=60s

# Profile de CPU
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Profile de memória
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof
```

---

## 🔗 Referências

- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [FFmpeg Streaming](https://trac.ffmpeg.org/wiki/StreamingGuide)
- [Worker Pool Pattern](https://gobyexample.com/worker-pools)
- [Circuit Breaker Pattern](https://martinfowler.com/bliki/CircuitBreaker.html)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/)

---

**Última Atualização:** 2025-11-07  
**Autor:** Análise Técnica de Performance
