package camera

import (
	"bytes"
	"context"
	"log"
	"os/exec"
	"time"

	"github.com/T3-Labs/edge-video/pkg/mq"
	"github.com/T3-Labs/edge-video/pkg/util"
)

type Config struct {
	ID  string
	URL string
}

type Capture struct {
	ctx        context.Context
	config     Config
	interval   time.Duration
	compressor *util.Compressor
	publisher  mq.Publisher
}

func NewCapture(ctx context.Context, config Config, interval time.Duration, compressor *util.Compressor, publisher mq.Publisher) *Capture {
	return &Capture{
		ctx:        ctx,
		config:     config,
		interval:   interval,
		compressor: compressor,
		publisher:  publisher,
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

	err = c.publisher.Publish(c.ctx, c.config.ID, frameData)
	if err != nil {
		log.Printf("erro ao publicar frame da câmera %s: %v", c.config.ID, err)
	}
}