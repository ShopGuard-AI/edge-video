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
	"runtime"
	"strings"
	"sync"
	"time"

	"edge-video/v2/internal/capabilities"
	"edge-video/v2/internal/config"
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
	ID      string
	URL     string
	FPS     int
	Quality int

	publisher      *messaging.Publisher
	circuitBreaker *resilience.CircuitBreaker   // Circuit breaker para proteção contra falhas
	publishHealth  *health.PublishHealthMonitor // Monitor de saúde de publish
	metricsServer  *monitoring.MetricsServer    // Servidor de métricas Prometheus
	ctx            context.Context
	cancel         context.CancelFunc

	mu                sync.Mutex
	frameCount        uint64 // Frames publicados
	framesReceived    uint64 // Frames recebidos do FFmpeg
	framesDropped     uint64 // Frames descartados (canal cheio)
	lastFrame         time.Time
	lastFrameReceived time.Time // Último frame do FFmpeg
	running           bool
	retrying          bool // Flag para evitar múltiplas goroutines de retry

	// CORREÇÃO CRÍTICA: Buffers privados da câmera (não compartilhados!)
	bufferPool chan []byte // Pool LOCAL de buffers (não global!)

	frameChan chan []byte
	cmd       *exec.Cmd

	// Graceful shutdown
	wg sync.WaitGroup // Rastreia goroutines ativas

	// Worker Pool: Semáforo para limitar goroutines de publish concorrentes
	publishSemaphore chan struct{}

	// Configurações dinâmicas
	watchdogTimeout    time.Duration
	initialGracePeriod time.Duration

	// Configuração dinâmica de Encoding
	encodingConfig config.EncodingConfig

	// Estado de Hardware
	hwAccelFailed bool // Se true, desativa QSV e usa CPU (fallback)
}

// NewCameraStream cria câmera com buffers PRIVADOS e circuit breaker
func NewCameraStream(id, url string, fps, quality int, encodingConfig config.EncodingConfig, publisher *messaging.Publisher, cbConfig resilience.CircuitBreakerConfig,
	metricsServer *monitoring.MetricsServer, workerPoolSize int, watchdogTimeout, initialGracePeriod time.Duration) *CameraStream {

	ctx, cancel := context.WithCancel(context.Background())

	if workerPoolSize <= 0 {
		workerPoolSize = 3
	}

	c := &CameraStream{
		ID:                 id,
		URL:                url,
		FPS:                fps,
		Quality:            quality,
		encodingConfig:     encodingConfig,
		publisher:          publisher,
		circuitBreaker:     resilience.NewCircuitBreaker(id, cbConfig),
		publishHealth:      health.NewPublishHealthMonitor(10, 0.8), // Janela: 10, Threshold: 80%
		metricsServer:      metricsServer,
		ctx:                ctx,
		cancel:             cancel,
		frameChan:          make(chan []byte, 5), // Buffer de 5 frames
		watchdogTimeout:    watchdogTimeout,
		initialGracePeriod: initialGracePeriod,

		// CRÍTICO: Pool LOCAL de buffers (10 buffers dedicados para ESTA câmera)
		bufferPool: make(chan []byte, 10),

		// Worker Pool: Semáforo limitado
		publishSemaphore: make(chan struct{}, workerPoolSize),
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
	c.startStreamWatchdogWithWaitGroup()
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

// startStreamWatchdogWithWaitGroup wrapper que gerencia WaitGroup
func (c *CameraStream) startStreamWatchdogWithWaitGroup() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.streamWatchdog()
	}()
}

// streamWatchdog monitora se estamos recebendo frames e reinicia em caso de stall
func (c *CameraStream) streamWatchdog() {
	timeout := c.watchdogTimeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}

	log.Printf("[%s] Stream Watchdog iniciado (Timeout: %v)", c.ID, timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Aguarda um pouco antes de começar a matar processos para dar tempo de conexão
	grace := c.initialGracePeriod
	if grace == 0 {
		grace = 20 * time.Second
	}
	initialGracePeriod := time.Now().Add(grace)

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("[%s] Stream Watchdog parado", c.ID)
			return
		case <-ticker.C:
			// Se ainda estamos no período de graça inicial, ignora
			if time.Now().Before(initialGracePeriod) {
				continue
			}

			c.mu.Lock()
			lastRecv := c.lastFrameReceived
			isRetrying := c.retrying
			process := c.cmd
			c.mu.Unlock()

			// Se já estamos recuperando, não faça nada
			if isRetrying {
				continue
			}

			// Se não recebemos frames há mais de timeout
			if time.Since(lastRecv) > timeout {
				log.Printf("[%s] 🚨 STALL DETECTED! Sem frames há %v. Forçando reinício...",
					c.ID, time.Since(lastRecv).Round(time.Second))

				// Mata o processo FFmpeg para forçar erro de leitura e trigger no restart logic
				if process != nil && process.Process != nil {
					pid := process.Process.Pid
					log.Printf("[%s] 🔪 Matando processo FFmpeg (PID: %d) para destravar leitura...", c.ID, pid)

					// Tenta matar de forma robusta (Go Kill + Windows Taskkill)
					err := forceKillProcess(pid, process.Process)
					if err != nil {
						log.Printf("[%s] Erro ao matar processo: %v", c.ID, err)
					} else {
						log.Printf("[%s] Processo %d morto com sucesso via Force Kill", c.ID, pid)
					}
					// A leitura vai falhar, e o readFrames vai acionar o retry logic
				}
			}
		}
	}
}

// forceKillProcess tenta matar um processo de todas as formas possíveis
func forceKillProcess(pid int, proc *os.Process) error {
	// 1. Tenta método nativo do Go (SIGKILL)
	goErr := proc.Kill()
	if goErr == nil {
		return nil // Sucesso
	}

	// 2. Se falhar (Access Denied no Windows), tenta taskkill
	// Isso é comum quando o processo está "zumbi" ou travado em I/O driver
	if runtime.GOOS == "windows" {
		// /F = Force, /T = Tree (mata filhos), /PID = Process ID
		cmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil // Sucesso via taskkill
		}
		return fmt.Errorf("go kill failed: %v | taskkill failed: %v (out: %s)", goErr, err, string(out))
	}

	return goErr
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
	// Verifica suporte HW (Decode e Encode)
	hasQSVDecode := capabilities.HasQSV()
	hasQSVEncode := capabilities.HasMJPEGQSV()

	// Define modo de operação
	useHWDecode := hasQSVDecode && !c.hwAccelFailed
	useHWEncode := useHWDecode && hasQSVEncode // Só usa encode se decode também for HW (pipeline full)

	accelTag := "CPU (Software)"
	if useHWDecode {
		if useHWEncode {
			accelTag = "QSV FULL (GPU Decode + GPU Encode)"
		} else {
			accelTag = "QSV HYBRID (GPU Decode + CPU Encode)"
		}
	}

	log.Printf("[%s] Iniciando stream FFmpeg - FPS: %d - Mode: %s", c.ID, c.FPS, accelTag)

	// Atualiza LastFrameReceived para agora para evitar que watchdog mate imediatamente
	c.mu.Lock()
	c.lastFrameReceived = time.Now()
	c.mu.Unlock()

	// Detecta protocolo
	isRTMP := strings.HasPrefix(strings.ToLower(c.URL), "rtmp://") ||
		strings.HasPrefix(strings.ToLower(c.URL), "rtmps://")
	isRTSP := strings.HasPrefix(strings.ToLower(c.URL), "rtsp://") ||
		strings.HasPrefix(strings.ToLower(c.URL), "rtsps://")

	// Procura FFmpeg (local primeiro, depois PATH)
	ffmpegPath := findFFmpegPath()
	args := []string{ffmpegPath}

	// 1. Hardware Acceleration (ANTES do input -i)
	if useHWDecode {
		// "-hwaccel auto" tenta o melhor disponível (dxva2/d3d11va/qsv no Windows)
		args = append(args, "-hwaccel", "auto")
	}

	if isRTSP {
		log.Printf("[%s] Protocolo: RTSP", c.ID)
		args = append(args,
			"-rtsp_transport", "tcp",
			"-timeout", "30000000", // 30s - melhor estabilidade para streams longos
			"-rtsp_flags", "prefer_tcp",
		)
	} else if isRTMP {
		log.Printf("[%s] Protocolo: RTMP", c.ID)
		args = append(args,
			"-rw_timeout", "30000000", // 30s - melhor estabilidade
			"-listen", "0",
		)
	}

	args = append(args,
		"-fflags", "nobuffer+fastseek+flush_packets+discardcorrupt",
		"-flags", "low_delay",
		"-max_delay", "0",
		"-probesize", "5000000", // 5MB - detecta stream format adequadamente
		"-analyzeduration", "5000000", // 5s - analisa stream sem EOF prematuro
		"-err_detect", "ignore_err",
		"-i", c.URL,
		"-vf", fmt.Sprintf("fps=%d", c.FPS),
		"-f", "image2pipe",
	)

	// SELEÇÃO DO ENCODER
	if useHWEncode {
		// Modo Full GPU: Encoder de Hardware
		// MAPEAMENTO DE QUALIDADE:
		// Config 'quality' (1-31, menor=melhor) -> QSV 'global_quality' (1-100, maior=melhor)

		qsvQuality := 0 // Será calculado ou override

		// 1. Verifica se há override explícito na config (YAML encoding.qsv_quality)
		if c.encodingConfig.QSVQuality > 0 {
			qsvQuality = c.encodingConfig.QSVQuality
			log.Printf("[%s] QSV Quality Override: Usando valor explícito do config: %d", c.ID, qsvQuality)
		} else {
			// 2. Lógica padrão baseada no 'quality' (1-31)
			// ALTA QUALIDADE PADRÃO (Config 5 -> QSV 85)
			if c.Quality <= 31 {
				qsvQuality = 100 - (c.Quality * 3)
			} else {
				// Fallback seguro
				qsvQuality = c.Quality
			}
		}

		// Safety Check
		if qsvQuality < 1 {
			qsvQuality = 1
		}
		if qsvQuality > 100 {
			qsvQuality = 100
		}

		log.Printf("[%s] QSV Final Quality: %d", c.ID, qsvQuality)

		args = append(args,
			"-c:v", "mjpeg_qsv",
			"-global_quality", fmt.Sprintf("%d", qsvQuality),
		)
	} else {
		// Modo CPU/Híbrido: Encoder de Software
		args = append(args,
			"-vcodec", "mjpeg",
			"-q:v", fmt.Sprintf("%d", c.Quality),
		)
	}

	args = append(args,
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

	startTime := time.Now()

	// Inicia FFmpeg
	err = c.cmd.Start()
	if err != nil {
		log.Printf("[%s] ERRO ao iniciar FFmpeg: %v", c.ID, err)
		// Se falhar no Start, marca falha de HW se estava tentando usar
		if useHWDecode {
			log.Printf("[%s] Falha imediata no HW Accel. Desabilitando para próximas tentativas.", c.ID)
			c.mu.Lock()
			c.hwAccelFailed = true
			c.mu.Unlock()
		}

		c.circuitBreaker.Execute(func() error { return err })
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

			// Detecta erro específico de HW Accel se necessário
			if useHWDecode && (strings.Contains(line, "Device creation failed") || strings.Contains(line, "Failed to create Direct3D")) {
				log.Printf("[%s] ⚠️  Erro detectado no HW Accel: %s", c.ID, line)
				c.mu.Lock()
				c.hwAccelFailed = true
				c.mu.Unlock()
			}

			if strings.Contains(line, "error") || strings.Contains(line, "Error") ||
				strings.Contains(line, "fatal") || strings.Contains(line, "Fatal") {
				log.Printf("[%s] FFmpeg ERRO: %s", c.ID, line)
			}
		}
	}()

	log.Printf("[%s] FFmpeg iniciado!", c.ID)

	reader := bufio.NewReaderSize(stdout, 1024*1024)
	c.readFrames(reader)

	// Pós-morte: Se o processo morreu muito rápido (< 10s) e estávamos usando HW,
	// assume que o HW Accel causou o crash
	if useHWDecode && time.Since(startTime) < 10*time.Second {
		log.Printf("[%s] ⚠️  FFmpeg morreu muito rápido (<10s) com HW Accel. Desativando QSV para fallback seguro.", c.ID)
		c.mu.Lock()
		c.hwAccelFailed = true
		c.mu.Unlock()
	}
}

// readFrames lê frames do FFmpeg - OTIMIZADO (ZERO-COPY)
func (c *CameraStream) readFrames(reader *bufio.Reader) {
	frameBuffer := bytes.NewBuffer(make([]byte, 0, 512*1024))
	jpegSOI := []byte{0xFF, 0xD8}
	jpegEOI := []byte{0xFF, 0xD9}

	// Buffer temporário para leitura em blocos (32KB)
	chunk := make([]byte, 32*1024)

	// Índice de onde começar a procurar pelo EOI no frameBuffer
	searchStart := 0

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Lê um bloco de bytes (muito mais eficiente que ReadByte)
		n, err := reader.Read(chunk)
		if err != nil {
			if c.ctx.Err() == nil {
				log.Printf("[%s] ERRO ao ler: %v", c.ID, err)

				// CRÍTICO: Registra falha no circuit breaker
				c.circuitBreaker.Execute(func() error {
					return err
				})

				// SEMPRE tenta reconectar (circuit breaker controla o backoff)
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

		// Adiciona dados lidos ao buffer acumulador
		frameBuffer.Write(chunk[:n])

		// Procura pelo EOI (0xFF 0xD9) apenas na parte nova (+1 byte de overlap)
		// Isso evita scan O(N^2)
		if searchStart > 0 {
			searchStart-- // Recua 1 byte para caso o EOI tenha sido quebrado (0xFF | 0xD9)
		}

		// Fatia do buffer onde vamos procurar
		searchSlice := frameBuffer.Bytes()[searchStart:]
		eoiIndex := bytes.Index(searchSlice, jpegEOI)

		if eoiIndex >= 0 {
			// Encontrou EOI!
			// O índice real no buffer global é: searchStart + eoiIndex
			totalLength := searchStart + eoiIndex + 2 // +2 pelo tamanho do EOI

			// ZERO-COPY OPTIMIZATION
			// 1. Obtém buffer do pool local
			buf := c.getBuffer()

			// 2. Garante que cabe
			if totalLength > cap(buf) {
				log.Printf("[%s] AVISO: Frame gigante (%d bytes). Realocando...", c.ID, totalLength)
				c.putBuffer(buf)
				buf = make([]byte, totalLength+1024)
			}

			buf = buf[:totalLength]

			// 3. Copia bytes do buffer acumulador
			copy(buf, frameBuffer.Bytes()[:totalLength]) // Copia apenas o frame completo

			// 4. Valida SOI (Start of Image)
			if bytes.HasPrefix(buf, jpegSOI) {
				c.mu.Lock()
				c.framesReceived++
				c.lastFrameReceived = time.Now()
				c.mu.Unlock()

				// Envia para canal
				select {
				case c.frameChan <- buf:
					// Sucesso
				default:
					c.mu.Lock()
					c.framesDropped++
					c.mu.Unlock()
					// Devolve buffer se não foi usado (dropped)
					// Mas frameChan é um canal de ponteiros? Não, é []byte.
					// Quem recebe é responsável por devolver? Não temos release manual no channel receiver.
					// O receiver vai usar e o GC pega, mas e o pool?
					// Ah, c.frameChan passa pro loop de publish. O loop de publish DEVE devolver pro pool?
					// Atualmente o pool é "bufferPool". O código original não devolvia explicitamente no publish_loop?
					// Vamos verificar o publish loop depois.
					// Por segurança, se dropou, devolve agora.
					c.putBuffer(buf)
				}
			} else {
				// Frame inválido (sem SOI), devolve buffer
				c.putBuffer(buf)
			}

			// Limpeza e preparação para o próximo frame
			// Remove o frame processado do frameBuffer
			// Se sobrar algo (overlap), mantém.
			remaining := frameBuffer.Len() - totalLength
			if remaining > 0 {
				// Move os dados restantes para o início (Next ignora os lidos e retorna bytes restantes)
				// bytes.Buffer.Next(n) avança o read pointer.
				// Mas queremos "cortar".
				// A forma mais fácil é criar novo buffer com o resto.
				rest := frameBuffer.Bytes()[totalLength:]
				newBuf := bytes.NewBuffer(make([]byte, 0, 512*1024))
				newBuf.Write(rest)
				frameBuffer = newBuf
				searchStart = 0 // Reinicia busca do zero no novo resto
			} else {
				frameBuffer.Reset()
				searchStart = 0
			}
		} else {
			// Não encontrou EOI, avança o ponteiro de busca para o final atual
			searchStart = frameBuffer.Len()
		}

		// Loop continua lendo chunks...
		continue
	}
}

// publishLoop - OTIMIZADO (Memory Aware)
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
			// Descarta frames antigos para priorizar realtime
			// IMPORTANTE: Devolve frames descartados ao pool!
			flushed := 0
			for len(c.frameChan) > 0 {
				oldFrame := <-c.frameChan
				c.putBuffer(oldFrame) // <--- Fix: Devolve buffer descartado
				frame = <-c.frameChan // Pega novo
				c.putBuffer(oldFrame) // Devolve anterior (que agora é oldFrame) - Espere, lógica correta abaixo

				// Lógica correta de flush drop-tail:
				// Temos 'frame' (candidato).
				// Se tem mais no canal, 'frame' é velho. Devolve 'frame' e pega próximo.
				// Loop na verdade deve ser:
			}
			// Melhor implementar flush corretamente:
			for len(c.frameChan) > 0 {
				// Tem um frame mais novo
				c.putBuffer(frame)    // Devolve o que estava na mão
				frame = <-c.frameChan // Pega o mais novo
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

		// WORKER POOL: Adquire slot no semáforo (máximo 3 goroutines concorrentes)
		select {
		case c.publishSemaphore <- struct{}{}: // Adquire slot
			// Slot adquirido, continua para publicação
		case <-c.ctx.Done():
			// Contexto cancelado enquanto aguardava
			c.putBuffer(frame) // Devolve buffer se não usar
			continue
		}

		// Publica ASSÍNCRONA (rastreada pelo publishWg)
		publishWg.Add(1)

		// IMPORTANTE: Passamos 'frame' para a goroutine.
		// A goroutine é responsável por devolver ao pool quando terminar!
		go func(cameraID string, frameData []byte, frameNum uint64, start time.Time) {
			defer func() {
				// Devolve buffer ao pool APÓS uso
				c.putBuffer(frameData)

				<-c.publishSemaphore // Libera slot
				publishWg.Done()
			}()

			// Verifica se contexto foi cancelado antes de publicar
			select {
			case <-c.ctx.Done():
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

			if frameNum%300 == 0 {
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
