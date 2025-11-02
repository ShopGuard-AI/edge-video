package camera

import (
	"context"
	"log"
	"time"

	"edge_guard_ai/internal/mq"
	"edge_guard_ai/internal/util"
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
	// Simula a captura de um frame da camera
	frameData := []byte("frame_data_from_" + c.config.ID)
	log.Printf("capturado frame da camera %s", c.config.ID)

	var dataToPublish []byte
	var err error

	if c.compressor != nil {
		dataToPublish, err = c.compressor.Compress(frameData)
		if err != nil {
			log.Printf("erro ao comprimir frame: %v", err)
			return
		}
	} else {
		dataToPublish = frameData
	}

	err = c.publisher.Publish(c.ctx, c.config.ID, dataToPublish)
	if err != nil {
		log.Printf("erro ao publicar frame: %v", err)
	}
}
