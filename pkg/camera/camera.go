package camera

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os/exec"
	"time"

	"github.com/T3-Labs/edge-video/internal/metadata"
	"github.com/T3-Labs/edge-video/internal/storage"
	"github.com/T3-Labs/edge-video/pkg/mq"
	"github.com/T3-Labs/edge-video/pkg/util"
	"github.com/go-redis/redis/v8"
	"github.com/streadway/amqp"
)

type Config struct {
	ID  string
	URL string
}

type Capture struct {
	ctx           context.Context
	config        Config
	interval      time.Duration
	compressor    *util.Compressor
	publisher     mq.Publisher
	redisStore    *storage.RedisStore
	metaPublisher *metadata.Publisher
}

func NewCapture(
	ctx context.Context,
	config Config,
	interval time.Duration,
	compressor *util.Compressor,
	publisher mq.Publisher,
	redisStore *storage.RedisStore,
	metaPublisher *metadata.Publisher,
) *Capture {
	return &Capture{
		ctx:           ctx,
		config:        config,
		interval:      interval,
		compressor:    compressor,
		publisher:     publisher,
		redisStore:    redisStore,
		metaPublisher: metaPublisher,
	}
}

func (c *Capture) Start() {
	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		for {
			select {
			case <-c.ctx.Done():
				log.Printf("parando captura para camera %s", c.config.ID)
				return
			case <-ticker.C:
				c.captureAndPublish()
			}
		}
	}()
}

func (c *Capture) captureAndPublish() {
	cmd := exec.CommandContext(
		c.ctx,
		"ffmpeg",
		"-rtsp_transport", "tcp",
		"-i", c.config.URL,
		"-frames:v", "1",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-q:v", "5",
		"-",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("erro ao capturar frame da câmera %s: %v (stderr: %s)", c.config.ID, err, stderr.String())
		return
	}

	frameData := stdout.Bytes()
	if len(frameData) == 0 {
		log.Printf("frame vazio capturado da câmera %s", c.config.ID)
		return
	}

	log.Printf("capturado frame da camera %s (%d bytes)", c.config.ID, len(frameData))

	// Publicação principal (síncrona ou assíncrona, dependendo da implementação do publisher)
	err = c.publisher.Publish(c.ctx, c.config.ID, frameData)
	if err != nil {
		log.Printf("erro ao publicar frame da câmera %s: %v", c.config.ID, err)
	}

	// Operações assíncronas de Redis e Metadados
	if c.redisStore.Enabled() {
		go func() {
			timestamp := time.Now()
			// TODO: Obter width/height do frame real se possível
			width, height := 1280, 720

			key, err := c.redisStore.SaveFrame(c.ctx, c.config.ID, timestamp, frameData)
			if err != nil {
				// Tratar erro de conexão com Redis (ex: logar)
				if errors.Is(err, redis.ErrClosed) {
					log.Printf("redis store error (connection closed): %v", err)
				} else {
					log.Printf("redis store error: %v", err)
				}
				return
			}

			if c.metaPublisher.Enabled() {
				err = c.metaPublisher.PublishMetadata(c.config.ID, timestamp, key, width, height, len(frameData), "jpeg")
				if err != nil {
					// Tratar erro de conexão com RabbitMQ (ex: logar)
					if amqpErr, ok := err.(*amqp.Error); ok && amqpErr.Code == amqp.ChannelError {
						log.Printf("metadata publish error (channel closed): %v", err)
					} else {
						log.Printf("metadata publish error: %v", err)
					}
				}
			}
		}()
	}
}