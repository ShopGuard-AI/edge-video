package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"
)

// CameraStream usa FFmpeg em modo stream contínuo
type CameraStream struct {
	ID       string
	URL      string
	FPS      int
	Quality  int

	publisher *Publisher
	ctx       context.Context
	cancel    context.CancelFunc

	mu           sync.Mutex
	frameCount   uint64
	lastFrame    time.Time
	running      bool

	frameChan chan []byte
	cmd       *exec.Cmd
}

// NewCameraStream cria câmera com stream contínuo
func NewCameraStream(id, url string, fps, quality int, publisher *Publisher) *CameraStream {
	ctx, cancel := context.WithCancel(context.Background())

	return &CameraStream{
		ID:        id,
		URL:       url,
		FPS:       fps,
		Quality:   quality,
		publisher: publisher,
		ctx:       ctx,
		cancel:    cancel,
		frameChan: make(chan []byte, 1), // Buffer de apenas 1 frame
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

	go c.startFFmpeg()
	go c.publishLoop()
}

// Stop para
func (c *CameraStream) Stop() {
	c.cancel()
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
}

// startFFmpeg inicia FFmpeg e lê frames
func (c *CameraStream) startFFmpeg() {
	log.Printf("[%s] Iniciando stream FFmpeg - FPS: %d", c.ID, c.FPS)

	c.cmd = exec.CommandContext(c.ctx,
		"ffmpeg",
		"-loglevel", "error",
		"-rtsp_transport", "tcp",
		"-fflags", "nobuffer",           // Remove buffering
		"-flags", "low_delay",           // Baixa latência
		"-max_delay", "0",               // Sem delay
		"-i", c.URL,
		"-vf", fmt.Sprintf("fps=%d", c.FPS), // FORÇA FPS exato
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-q:v", fmt.Sprintf("%d", c.Quality),
		"-",
	)

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		log.Printf("[%s] ERRO ao criar pipe: %v", c.ID, err)
		return
	}

	err = c.cmd.Start()
	if err != nil {
		log.Printf("[%s] ERRO ao iniciar FFmpeg: %v", c.ID, err)
		return
	}

	log.Printf("[%s] FFmpeg iniciado!", c.ID)

	// Lê frames com buffer maior para melhor performance
	reader := bufio.NewReaderSize(stdout, 1024*1024) // 1MB buffer
	c.readFrames(reader)
}

// readFrames lê frames do stdout do FFmpeg
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
			}
			return
		}

		frameBuffer.WriteByte(b)

		// Detecta fim do JPEG
		if frameBuffer.Len() >= 2 {
			tail := frameBuffer.Bytes()[frameBuffer.Len()-2:]
			if bytes.Equal(tail, jpegEOI) {
				frameData := make([]byte, frameBuffer.Len())
				copy(frameData, frameBuffer.Bytes())

				// Valida JPEG
				if bytes.HasPrefix(frameData, jpegSOI) {
					select {
					case c.frameChan <- frameData:
					default:
						// Canal cheio, descarta frame antigo
					}
				}

				frameBuffer.Reset()
			}
		}
	}
}

// publishLoop publica frames em intervalos controlados
func (c *CameraStream) publishLoop() {
	log.Printf("[%s] Iniciando loop de publicação", c.ID)

	interval := time.Second / time.Duration(c.FPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("[%s] Publicação parada", c.ID)
			return

		case <-ticker.C:
			// Pega frame mais recente, descartando antigos
			var frame []byte
			select {
			case frame = <-c.frameChan:
				// Descarta frames acumulados
				for len(c.frameChan) > 0 {
					<-c.frameChan
				}
			default:
				continue // Sem frame disponível
			}

			start := time.Now()

			// Incrementa contador
			c.mu.Lock()
			c.frameCount++
			frameNum := c.frameCount
			c.lastFrame = start
			c.mu.Unlock()

			// Publica
			err := c.publisher.Publish(c.ID, frame, start)
			if err != nil {
				log.Printf("[%s] ERRO ao publicar frame #%d: %v", c.ID, frameNum, err)
				continue
			}

			publishDuration := time.Since(start)

			// Log a cada 30 frames
			if frameNum%30 == 0 {
				log.Printf("[%s] Frame #%d - Publicação: %v, Tamanho: %d bytes",
					c.ID, frameNum, publishDuration, len(frame))
			}
		}
	}
}

// Stats retorna estatísticas
func (c *CameraStream) Stats() (uint64, time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.frameCount, c.lastFrame
}
