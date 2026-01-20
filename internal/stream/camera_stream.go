package stream

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"edge-video/v2/internal/health"
	"edge-video/v2/internal/messaging"
	"edge-video/v2/internal/monitoring"
	"edge-video/v2/internal/resilience"
)

// findFFmpegPath procura FFmpeg no diretório local primeiro, depois no PATH
func findFFmpegPath() string {
	// 1. Procura no mesmo diretório do executável
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		localFFmpeg := filepath.Join(exeDir, "ffmpeg.exe")
		if _, err := os.Stat(localFFmpeg); err == nil {
			log.Printf("✓ FFmpeg encontrado localmente: %s", localFFmpeg)
			return localFFmpeg
		}
	}

	// 2. Procura no diretório de trabalho atual
	if _, err := os.Stat("./ffmpeg.exe"); err == nil {
		log.Printf("✓ FFmpeg encontrado no diretório atual: ./ffmpeg.exe")
		return "./ffmpeg.exe"
	}

	// 3. Usa PATH do sistema
	log.Printf("✓ Usando FFmpeg do PATH do sistema")
	return "ffmpeg"
}

// CameraStream usa FFmpeg em modo stream contínuo
// VERSÃO CORRIGIDA: Cada câmera tem seus PRÓPRIOS buffers (sem sync.Pool compartilhado)
type CameraStream struct {
	ID       string
	URL      string
	FPS      int
	Quality  int

	publisher      *messaging.Publisher
	circuitBreaker *resilience.CircuitBreaker      // Circuit breaker para proteção contra falhas
	publishHealth  *health.PublishHealthMonitor    // Monitor de saúde de publish
	metricsServer  *monitoring.MetricsServer       // Servidor de métricas Prometheus
	ctx            context.Context
	cancel         context.CancelFunc

	mu                sync.Mutex
	frameCount        uint64    // Frames publicados
	framesReceived    uint64    // Frames recebidos do FFmpeg
	framesDropped     uint64    // Frames descartados (canal cheio)
	lastFrame         time.Time
	lastFrameReceived time.Time // Último frame do FFmpeg
	running           bool
	retrying          bool      // Flag para evitar múltiplas goroutines de retry

	// CORREÇÃO CRÍTICA: Buffers privados da câmera (não compartilhados!)
	bufferPool chan []byte // Pool LOCAL de buffers (não global!)

	frameChan chan []byte
	cmd       *exec.Cmd

	// Graceful shutdown
	wg sync.WaitGroup // Rastreia goroutines ativas

	// Worker Pool: Semáforo para limitar goroutines de publish concorrentes (máximo 3)
	publishSemaphore chan struct{}
}

// NewCameraStream cria câmera com buffers PRIVADOS e circuit breaker
func NewCameraStream(id, url string, fps, quality int, publisher *messaging.Publisher, cbConfig resilience.CircuitBreakerConfig, metricsServer *monitoring.MetricsServer) *CameraStream {
	ctx, cancel := context.WithCancel(context.Background())

	c := &CameraStream{
		ID:             id,
		URL:            url,
		FPS:            fps,
		Quality:        quality,
		publisher:      publisher,
		circuitBreaker: resilience.NewCircuitBreaker(id, cbConfig),
		publishHealth:  health.NewPublishHealthMonitor(10, 0.8), // Janela: 10, Threshold: 80%
		metricsServer:  metricsServer,
		ctx:            ctx,
		cancel:         cancel,
		frameChan:      make(chan []byte, 5), // Buffer de 5 frames

		// CRÍTICO: Pool LOCAL de buffers (10 buffers dedicados para ESTA câmera)
		bufferPool: make(chan []byte, 10),

		// Worker Pool: Semáforo limitado a 3 goroutines concorrentes
		publishSemaphore: make(chan struct{}, 3),
	}

	// Pre-aloca 10 buffers DEDICADOS para esta câmera
	for i := 0; i < 10; i++ {
		buf := make([]byte, 2*1024*1024) // 2MB cada
		c.bufferPool <- buf
	}

	return c
}

// getBuffer pega buffer do pool LOCAL da câmera
func (c *CameraStream) getBuffer() []byte {
	select {
	case buf := <-c.bufferPool:
		return buf
	default:
		// Pool vazio, aloca novo (só acontece em caso extremo)
		log.Printf("[%s] AVISO: Pool local vazio, alocando novo buffer", c.ID)
		return make([]byte, 2*1024*1024)
	}
}

// putBuffer devolve buffer ao pool LOCAL da câmera
func (c *CameraStream) putBuffer(buf []byte) {
	select {
	case c.bufferPool <- buf:
		// Buffer devolvido com sucesso
	default:
		// Pool cheio, descarta buffer (GC vai liberar)
		// Isso é normal se alocamos buffers extras
	}
}

// Start inicia
func (c *CameraStream) Start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()

	// Inicia goroutines principais com wrapper que gerencia WaitGroup
	c.startFFmpegWithWaitGroup()
	c.startPublishLoopWithWaitGroup()
	c.startHealthMonitorWithWaitGroup()
}

// startFFmpegWithWaitGroup wrapper que gerencia WaitGroup
func (c *CameraStream) startFFmpegWithWaitGroup() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.startFFmpeg()
	}()
}

// startPublishLoopWithWaitGroup wrapper que gerencia WaitGroup
func (c *CameraStream) startPublishLoopWithWaitGroup() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.publishLoop()
	}()
}

// startHealthMonitorWithWaitGroup wrapper que gerencia WaitGroup
func (c *CameraStream) startHealthMonitorWithWaitGroup() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.healthMonitor()
	}()
}

// healthMonitor monitora saúde de publish e loga estatísticas periodicamente
func (c *CameraStream) healthMonitor() {
	log.Printf("[%s] Health monitor iniciado", c.ID)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("[%s] Health monitor parado", c.ID)
			return
		case <-ticker.C:
			stats := c.publishHealth.GetStats()

			// Log de estatísticas
			if stats.TotalRecent > 0 {
				log.Printf("[%s] 📊 Health Stats: Success=%d, Errors=%d, ErrorRate=%.1f%%, Consecutive=%d",
					c.ID,
					stats.RecentSuccesses,
					stats.RecentErrors,
					stats.ErrorRate*100,
					stats.ConsecutiveFails)

				// Alerta se degradado
				if stats.IsDegraded {
					log.Printf("[%s] ⚠️  WARNING: Publish health DEGRADED! ErrorRate=%.1f%% (threshold: 80%%)",
						c.ID, stats.ErrorRate*100)
				}

				// Alerta emergência
				if stats.IsEmergency {
					log.Printf("[%s] 🚨 EMERGENCY: %d consecutive failures! Circuit Breaker will trigger soon.",
						c.ID, stats.ConsecutiveFails)
				}
			}
		}
	}
}

// Stop para e aguarda finalização de todas as goroutines
func (c *CameraStream) Stop() {
	log.Printf("[%s] Iniciando shutdown...", c.ID)

	c.mu.Lock()
	c.running = false
	c.mu.Unlock()

	// Cancela contexto (sinaliza para todas as goroutines pararem)
	c.cancel()

	// Mata processo FFmpeg se existir
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}

	// Aguarda todas as goroutines finalizarem (com timeout de 45 segundos)
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("[%s] ✓ Shutdown completo - todas as goroutines finalizadas", c.ID)
	case <-time.After(45 * time.Second):
		log.Printf("[%s] ⚠️  Timeout (45s) no shutdown - forçando encerramento", c.ID)
	}
}

// startFFmpeg inicia FFmpeg e lê frames (IDÊNTICO ao original)
func (c *CameraStream) startFFmpeg() {
	log.Printf("[%s] Iniciando stream FFmpeg - FPS: %d", c.ID, c.FPS)

	// Detecta protocolo
	isRTMP := strings.HasPrefix(strings.ToLower(c.URL), "rtmp://") ||
	          strings.HasPrefix(strings.ToLower(c.URL), "rtmps://")
	isRTSP := strings.HasPrefix(strings.ToLower(c.URL), "rtsp://") ||
	          strings.HasPrefix(strings.ToLower(c.URL), "rtsps://")

	// Procura FFmpeg (local primeiro, depois PATH)
	ffmpegPath := findFFmpegPath()
	args := []string{ffmpegPath}

	if isRTSP {
		log.Printf("[%s] Protocolo: RTSP", c.ID)
		args = append(args,
			"-rtsp_transport", "tcp",
			"-timeout", "30000000",     // 30s - melhor estabilidade para streams longos
			"-rtsp_flags", "prefer_tcp",
		)
	} else if isRTMP {
		log.Printf("[%s] Protocolo: RTMP", c.ID)
		args = append(args,
			"-rw_timeout", "30000000",  // 30s - melhor estabilidade
			"-listen", "0",
		)
	}

	args = append(args,
		"-fflags", "nobuffer+fastseek+flush_packets+discardcorrupt",
		"-flags", "low_delay",
		"-max_delay", "0",
		"-probesize", "5000000",        // 5MB - detecta stream format adequadamente
		"-analyzeduration", "5000000",  // 5s - analisa stream sem EOF prematuro
		"-err_detect", "ignore_err",
		"-i", c.URL,
		"-vf", fmt.Sprintf("fps=%d", c.FPS),
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-q:v", fmt.Sprintf("%d", c.Quality),
		"-pkt_size", "2097152",
		"-max_muxing_queue_size", "1024",
		"-threads", "1",
		"-",
	)

	c.cmd = exec.CommandContext(c.ctx, args[0], args[1:]...)

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		log.Printf("[%s] ERRO ao criar pipe: %v", c.ID, err)
		return
	}

	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		log.Printf("[%s] ERRO ao criar stderr pipe: %v", c.ID, err)
		return
	}

	// Inicia FFmpeg (SEM circuit breaker aqui - só monitora stream reads)
	err = c.cmd.Start()
	if err != nil {
		log.Printf("[%s] ERRO ao iniciar FFmpeg: %v", c.ID, err)

		// Registra falha no circuit breaker
		c.circuitBreaker.Execute(func() error {
			return err
		})

		// Agenda retry (circuit breaker controla backoff)
		// Mas apenas se já não há retry em andamento
		c.mu.Lock()
		if !c.retrying {
			c.retrying = true
			c.mu.Unlock()
			go c.retryFFmpegWithBackoff()
		} else {
			c.mu.Unlock()
		}
		return
	}

	// Monitora stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "error") || strings.Contains(line, "Error") ||
			   strings.Contains(line, "fatal") || strings.Contains(line, "Fatal") {
				log.Printf("[%s] FFmpeg ERRO: %s", c.ID, line)
			}
		}
	}()

	log.Printf("[%s] FFmpeg iniciado!", c.ID)

	reader := bufio.NewReaderSize(stdout, 1024*1024)
	c.readFrames(reader)
}

// readFrames lê frames do FFmpeg - VERSÃO CORRIGIDA
func (c *CameraStream) readFrames(reader *bufio.Reader) {
	frameBuffer := bytes.NewBuffer(make([]byte, 0, 512*1024))

	jpegSOI := []byte{0xFF, 0xD8}
	jpegEOI := []byte{0xFF, 0xD9}

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		b, err := reader.ReadByte()
		if err != nil {
			if c.ctx.Err() == nil {
				log.Printf("[%s] ERRO ao ler: %v", c.ID, err)

				// CRÍTICO: Registra falha no circuit breaker
				c.circuitBreaker.Execute(func() error {
					return err
				})

				// SEMPRE tenta reconectar (circuit breaker controla o backoff)
				// Mas apenas se já não há retry em andamento
				c.mu.Lock()
				if !c.retrying {
					c.retrying = true
					c.mu.Unlock()
					go c.retryFFmpegWithBackoff()
				} else {
					c.mu.Unlock()
				}
			}
			return
		}

		frameBuffer.WriteByte(b)

		// Detecta fim do JPEG
		if frameBuffer.Len() >= 2 {
			tail := frameBuffer.Bytes()[frameBuffer.Len()-2:]
			if bytes.Equal(tail, jpegEOI) {
				// CORREÇÃO CRÍTICA: Usa buffer do pool LOCAL (não global!)
				buf := c.getBuffer()
				frameSize := frameBuffer.Len()

				if frameSize > len(buf) {
					log.Printf("[%s] ERRO: Frame %d bytes > buffer %d bytes", c.ID, frameSize, len(buf))
					c.putBuffer(buf)
					frameBuffer.Reset()
					continue
				}

				// CORREÇÃO CRÍTICA: FAZ CÓPIA IMEDIATA para um novo slice
				// NÃO envia o buffer do pool para o channel!
				frameCopy := make([]byte, frameSize)
				copy(frameCopy, frameBuffer.Bytes())

				// DEVOLVE buffer IMEDIATAMENTE ao pool local
				c.putBuffer(buf)

				// Valida JPEG
				if bytes.HasPrefix(frameCopy, jpegSOI) && bytes.HasSuffix(frameCopy, jpegEOI) {
					c.mu.Lock()
					c.framesReceived++
					c.lastFrameReceived = time.Now()
					c.mu.Unlock()

					// Registra frame recebido no Prometheus
					if c.metricsServer != nil {
						c.metricsServer.TrackFrameReceived(c.ID, frameSize)
					}

					// Envia CÓPIA para o channel (não o buffer do pool!)
					select {
					case c.frameChan <- frameCopy:
						// Frame enviado
					default:
						// Canal cheio, descarta (GC vai liberar frameCopy)
						c.mu.Lock()
						c.framesDropped++
						c.mu.Unlock()

						// Registra frame descartado no Prometheus
						if c.metricsServer != nil {
							c.metricsServer.TrackFrameDropped(c.ID)
						}
					}
				}

				frameBuffer.Reset()
			}
		}
	}
}

// publishLoop - VERSÃO CORRIGIDA (muito mais simples!)
func (c *CameraStream) publishLoop() {
	log.Printf("[%s] Iniciando loop de publicação", c.ID)

	interval := time.Second / time.Duration(c.FPS)
	lastPublish := time.Now()

	// WaitGroup para rastrear goroutines de publish
	var publishWg sync.WaitGroup

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("[%s] 🛑 Contexto cancelado, aguardando goroutines de publish finalizarem...", c.ID)

			// Aguarda todas as goroutines de publish finalizarem (com timeout maior)
			done := make(chan struct{})
			go func() {
				publishWg.Wait()
				close(done)
			}()

			// Monitor de progresso a cada 5 segundos
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-done:
					log.Printf("[%s] ✓ Todas as goroutines de publish finalizadas", c.ID)
					log.Printf("[%s] ✓ Publicação parada", c.ID)
					return
				case <-time.After(40 * time.Second):
					log.Printf("[%s] ⚠️  Timeout (40s) aguardando goroutines de publish - forçando shutdown", c.ID)
					log.Printf("[%s] Publicação parada", c.ID)
					return
				case <-ticker.C:
					log.Printf("[%s] ⏳ Ainda aguardando goroutines de publish finalizarem...", c.ID)
				}
			}
		default:
		}

		elapsed := time.Since(lastPublish)
		if elapsed < interval {
			time.Sleep(interval - elapsed)
		}

		// Pega frame mais recente
		var frame []byte
		select {
		case frame = <-c.frameChan:
			// Descarta frames antigos
			flushed := 0
			for len(c.frameChan) > 0 {
				frame = <-c.frameChan // Sobrescreve com mais recente
				flushed++
			}
			if flushed > 10 {
				log.Printf("[%s] Latest Frame Policy: descartou %d frames antigos", c.ID, flushed)
			}
		default:
			continue
		}

		lastPublish = time.Now()
		start := time.Now()

		c.mu.Lock()
		c.frameCount++
		frameNum := c.frameCount
		c.lastFrame = start
		c.mu.Unlock()

		// CORREÇÃO CRÍTICA: NÃO precisa mais de frameCopy aqui!
		// O frame JÁ É UMA CÓPIA INDEPENDENTE feita na linha 245

		// WORKER POOL: Adquire slot no semáforo (máximo 3 goroutines concorrentes)
		// Isso previne criação ilimitada de goroutines quando Redis está lento
		select {
		case c.publishSemaphore <- struct{}{}: // Adquire slot
			// Slot adquirido, continua para publicação
		case <-c.ctx.Done():
			// Contexto cancelado enquanto aguardava slot
			continue
		}

		// Publica ASSÍNCRONA (rastreada pelo publishWg)
		publishWg.Add(1)
		go func(cameraID string, frameData []byte, frameNum uint64, start time.Time) {
			// CRÍTICO: Libera slot do semáforo quando goroutine terminar
			defer func() {
				<-c.publishSemaphore // Libera slot
				publishWg.Done()
			}()

			// Verifica se contexto foi cancelado antes de publicar
			select {
			case <-c.ctx.Done():
				// Contexto cancelado, não publica
				return
			default:
			}

			// ✅ USA PUBLISHWITHCONTEXT para respeitar cancelamento imediato!
			err := c.publisher.PublishWithContext(c.ctx, cameraID, frameData, start)
			publishDuration := time.Since(start)
			monitoring.TrackPublish(publishDuration)

			// Registra publicação no Prometheus
			if c.metricsServer != nil {
				c.metricsServer.TrackFramePublished(cameraID, publishDuration, err == nil)
			}

			if frameNum%30 == 0 {
				log.Printf("[%s] Frame #%d - Publicação: %v, Tamanho: %d bytes",
					cameraID, frameNum, publishDuration, len(frameData))
			}

			if err != nil {
				// Só loga erro se NÃO foi devido ao shutdown
				select {
				case <-c.ctx.Done():
					// Shutdown em andamento, ignora erro
					return
				default:
					log.Printf("[%s] ERRO ao publicar frame #%d: %v", cameraID, frameNum, err)

					// ✅ HEALTH MONITOR: Registra erro
					c.publishHealth.RecordError()

					// 🔥 CIRCUIT BREAKER HÍBRIDO: Se publish está degradado, registra falha
					if c.publishHealth.IsDegraded() {
						log.Printf("[%s] ⚠️  Publish health DEGRADED (80%%+ erros), acionando Circuit Breaker", cameraID)
						c.circuitBreaker.Execute(func() error {
							return fmt.Errorf("publish health degraded")
						})
					}
				}
			} else {
				// ✅ HEALTH MONITOR: Registra sucesso
				c.publishHealth.RecordSuccess()
			}
		}(c.ID, frame, frameNum, start)
	}
}

// Stats retorna estatísticas
func (c *CameraStream) Stats() (uint64, time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.frameCount, c.lastFrame
}

// DetailedStats retorna estatísticas detalhadas
func (c *CameraStream) DetailedStats() (uint64, uint64, uint64, time.Time, time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.frameCount, c.framesReceived, c.framesDropped, c.lastFrame, c.lastFrameReceived
}

// retryFFmpegWithBackoff tenta reconectar FFmpeg respeitando circuit breaker
// IMPORTANTE: Assume que c.retrying já foi setado para true ANTES de chamar esta função
func (c *CameraStream) retryFFmpegWithBackoff() {
	defer func() {
		c.mu.Lock()
		c.retrying = false
		c.mu.Unlock()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Verifica estado do circuit breaker
		stats := c.circuitBreaker.Stats()

		if stats.State == resilience.StateOpen {
			// Aguarda backoff
			if stats.TimeUntilRetry > 0 {
				log.Printf("[%s] Circuit breaker OPEN - aguardando %v antes de retry...",
					c.ID, stats.TimeUntilRetry)
				time.Sleep(stats.TimeUntilRetry)
			}
			continue
		}

		// Mata processo FFmpeg anterior (se existir) antes de reconectar
		c.mu.Lock()
		if c.cmd != nil && c.cmd.Process != nil {
			log.Printf("[%s] Matando processo FFmpeg anterior (PID: %d) antes de reconectar...", c.ID, c.cmd.Process.Pid)
			c.cmd.Process.Kill()
			c.cmd = nil
		}
		c.mu.Unlock()

		// Tenta reconectar usando wrapper que gerencia WaitGroup
		log.Printf("[%s] Tentando reconectar FFmpeg (estado: %s)...", c.ID, stats.State)
		c.startFFmpegWithWaitGroup()
		return
	}
}

// GetCircuitBreakerStats retorna estatísticas do circuit breaker
func (c *CameraStream) GetCircuitBreakerStats() resilience.CircuitBreakerStats {
	return c.circuitBreaker.Stats()
}
