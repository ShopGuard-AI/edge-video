package camera

import (
	"context"
	"log"
	"time"

	"edge_guard_ai/internal/mq"
	"edge_guard_ai/internal/util"
	
	"gocv.io/x/gocv"
)

type CameraConfig struct {
	ID  string
	URL string
}

type Capture struct {
	ctx        context.Context
	config     CameraConfig
	interval   time.Duration
	compressor *util.Compressor
	publisher  mq.Publisher
}

func NewCapture(ctx context.Context, config CameraConfig, interval time.Duration, compressor *util.Compressor, publisher mq.Publisher) *Capture {
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
	webcam, err := gocv.OpenVideoCapture(c.config.URL)
	if err != nil {
		log.Printf("erro ao abrir câmera %s: %v", c.config.ID, err)
		return
	}
	defer webcam.Close()

	img := gocv.NewMat()
	defer img.Close()

	if ok := webcam.Read(&img); !ok || img.Empty() {
		log.Printf("erro ao capturar frame da câmera %s", c.config.ID)
		return
	}

	buf, err := gocv.IMEncode(".jpg", img)
	if err != nil {
		log.Printf("erro ao codificar JPEG: %v", err)
		return
	}

	// Publica na fila sem compressão
	err = c.publisher.Publish(c.ctx, c.config.ID, buf)
	if err != nil {
		log.Printf("erro ao publicar frame: %v", err)
	}
}
