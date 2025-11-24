package camera

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"

	"github.com/T3-Labs/edge-video/internal/metadata"
	"github.com/T3-Labs/edge-video/internal/storage"
	"github.com/T3-Labs/edge-video/pkg/buffer"
	"github.com/T3-Labs/edge-video/pkg/circuit"
	"github.com/T3-Labs/edge-video/pkg/logger"
	"github.com/T3-Labs/edge-video/pkg/metrics"
	"github.com/T3-Labs/edge-video/pkg/mq"
	"github.com/T3-Labs/edge-video/pkg/util"
	"github.com/T3-Labs/edge-video/pkg/worker"
	"github.com/go-redis/redis/v8"
	"github.com/streadway/amqp"
)

type Config struct {
	ID  string
	URL string
}

type Capture struct {
	ctx               context.Context
	bufferCtx         context.Context
	bufferCancel      context.CancelFunc
	config            Config
	interval          time.Duration
	compressor        *util.Compressor
	publisher         mq.Publisher
	redisStore        *storage.RedisStore
	metaPublisher     *metadata.Publisher
	workerPool        *worker.Pool
	frameBuffer       *buffer.FrameBuffer
	circuitBreaker    *circuit.Breaker
	persistentCapture *PersistentCapture
	usePersistent     bool
}

func NewCapture(
	ctx context.Context,
	config Config,
	interval time.Duration,
	compressor *util.Compressor,
	publisher mq.Publisher,
	redisStore *storage.RedisStore,
	metaPublisher *metadata.Publisher,
	workerPool *worker.Pool,
	frameBuffer *buffer.FrameBuffer,
	circuitBreaker *circuit.Breaker,
	usePersistent bool,
	persistentBufferSize int,
) *Capture {
	bufferCtx, bufferCancel := context.WithCancel(ctx)

	capture := &Capture{
		ctx:            ctx,
		bufferCtx:      bufferCtx,
		bufferCancel:   bufferCancel,
		config:         config,
		interval:       interval,
		compressor:     compressor,
		publisher:      publisher,
		redisStore:     redisStore,
		metaPublisher:  metaPublisher,
		workerPool:     workerPool,
		frameBuffer:    frameBuffer,
		circuitBreaker: circuitBreaker,
		usePersistent:  usePersistent,
	}

	if usePersistent {
		fps := int(time.Second / interval)
		if fps == 0 {
			fps = 30
		}
		capture.persistentCapture = NewPersistentCapture(ctx, config.ID, config.URL, 5, fps, persistentBufferSize)
	}

	return capture
}

func (c *Capture) Start() {
	go c.bufferDispatcher()

	if c.usePersistent && c.persistentCapture != nil {
		err := c.persistentCapture.Start()
		if err != nil {
			logger.Log.Errorw("Erro ao iniciar captura persistente, usando modo clássico",
				"camera_id", c.config.ID,
				"error", err)
			c.usePersistent = false
		} else {
			go c.persistentCaptureLoop()
			metrics.CameraConnected.WithLabelValues(c.config.ID).Set(1)
			return
		}
	}

	go c.classicCaptureLoop()
	metrics.CameraConnected.WithLabelValues(c.config.ID).Set(1)
}

func (c *Capture) bufferDispatcher() {
	for {
		frame, ok := c.frameBuffer.PopBlocking(c.bufferCtx)
		if !ok {
			if c.bufferCtx.Err() != nil {
				return
			}
			continue
		}

		job := c.newJob(frame)

		if err := c.workerPool.Submit(job); err != nil {
			metrics.FramesDropped.WithLabelValues(c.config.ID, "worker_pool_full").Inc()
			logger.Log.Warnw("Worker pool cheio, processando sincronamente",
				"camera_id", c.config.ID)
			if procErr := job.Process(c.ctx); procErr != nil {
				logger.Log.Errorw("Erro ao processar frame após fallback",
					"camera_id", c.config.ID,
					"error", procErr)
			}
		}

		metrics.BufferSize.WithLabelValues(c.config.ID).Set(float64(c.frameBuffer.Size()))
	}
}

func (c *Capture) persistentCaptureLoop() {
	logger.Log.Infow("Iniciando loop de captura persistente",
		"camera_id", c.config.ID)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			logger.Log.Infow("Parando captura persistente",
				"camera_id", c.config.ID)
			c.persistentCapture.Stop()
			metrics.CameraConnected.WithLabelValues(c.config.ID).Set(0)
			return

		case <-ticker.C:
			frame, ok := c.persistentCapture.GetFrameWithTimeout(c.interval / 2)
			if !ok {
				metrics.FramesDropped.WithLabelValues(c.config.ID, "no_frame_available").Inc()
				continue
			}

			metrics.FrameSizeBytes.WithLabelValues(c.config.ID).Observe(float64(len(frame)))
			c.enqueueFrame(frame, false)
		}
	}
}

func (c *Capture) classicCaptureLoop() {
	logger.Log.Infow("Iniciando loop de captura clássica",
		"camera_id", c.config.ID)

	for {
		select {
		case <-c.ctx.Done():
			logger.Log.Infow("Parando captura clássica",
				"camera_id", c.config.ID)
			metrics.CameraConnected.WithLabelValues(c.config.ID).Set(0)
			return

		default:
		}

		start := time.Now()
		c.captureAndPublish()

		elapsed := time.Since(start)
		sleepTime := c.interval - elapsed
		if sleepTime > 0 {
			time.Sleep(sleepTime)
		}
	}
}

func (c *Capture) captureAndPublish() {
	start := time.Now()

	err := c.circuitBreaker.Call(func() error {
		return c.doCapture()
	})

	if err != nil {
		logger.Log.Errorw("Erro na captura com circuit breaker",
			"camera_id", c.config.ID,
			"error", err)
		metrics.FramesDropped.WithLabelValues(c.config.ID, "circuit_breaker_open").Inc()
		return
	}

	metrics.CaptureLatency.WithLabelValues(c.config.ID).Observe(time.Since(start).Seconds())
}

func (c *Capture) doCapture() error {
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
		logger.Log.Errorw("Erro ao capturar frame",
			"camera_id", c.config.ID,
			"error", err,
			"stderr", stderr.String())
		return err
	}

	frameData := stdout.Bytes()
	if len(frameData) == 0 {
		logger.Log.Warnw("Frame vazio capturado",
			"camera_id", c.config.ID)
		return errors.New("frame vazio")
	}

	metrics.FrameSizeBytes.WithLabelValues(c.config.ID).Observe(float64(len(frameData)))

	logger.Log.Debugw("Frame capturado",
		"camera_id", c.config.ID,
		"size_bytes", len(frameData))

	c.enqueueFrame(frameData, true)
	return nil
}

func (c *Capture) enqueueFrame(frameData []byte, copyData bool) {
	if len(frameData) == 0 {
		logger.Log.Warnw("Frame vazio recebido ao enfileirar",
			"camera_id", c.config.ID)
		return
	}

	data := frameData
	if copyData {
		buf := getFrameBuffer(len(frameData))
		copy(buf, frameData)
		data = buf
	}

	frame := buffer.Frame{
		CameraID:  c.config.ID,
		Data:      data,
		Timestamp: time.Now(),
		Release: func() {
			releaseFrameBuffer(data)
		},
	}

	if err := c.frameBuffer.Push(frame); err != nil {
		metrics.FramesDropped.WithLabelValues(c.config.ID, "frame_buffer_full").Inc()
		logger.Log.Warnw("Frame buffer cheio, frame substituído",
			"camera_id", c.config.ID,
			"buffer_size", c.frameBuffer.Capacity())
	}

	metrics.BufferSize.WithLabelValues(c.config.ID).Set(float64(c.frameBuffer.Size()))
}

func (c *Capture) newJob(frame buffer.Frame) *FrameProcessJob {
	return &FrameProcessJob{
		cameraID:      frame.CameraID,
		frameData:     frame.Data,
		timestamp:     frame.Timestamp,
		publisher:     c.publisher,
		redisStore:    c.redisStore,
		metaPublisher: c.metaPublisher,
		release:       frame.Release,
	}
}

type FrameProcessJob struct {
	cameraID      string
	frameData     []byte
	timestamp     time.Time
	publisher     mq.Publisher
	redisStore    *storage.RedisStore
	metaPublisher *metadata.Publisher
	release       func()
}

func (j *FrameProcessJob) GetID() string {
	return j.cameraID + "_" + j.timestamp.Format("20060102150405.000")
}

func (j *FrameProcessJob) Process(ctx context.Context) error {
	defer func() {
		if j.release != nil {
			j.release()
		}
	}()
	start := time.Now()

	err := j.publisher.Publish(ctx, j.cameraID, j.frameData)
	if err != nil {
		logger.Log.Errorw("Erro ao publicar frame",
			"camera_id", j.cameraID,
			"error", err)
		return err
	}

	metrics.PublishLatency.WithLabelValues("amqp").Observe(time.Since(start).Seconds())
	metrics.FramesProcessed.WithLabelValues(j.cameraID).Inc()

	if j.redisStore.Enabled() {
		width, height := 1280, 720

		key, err := j.redisStore.SaveFrame(ctx, j.cameraID, j.timestamp, j.frameData)
		if err != nil {
			if errors.Is(err, redis.ErrClosed) {
				logger.Log.Errorw("Redis store error (connection closed)",
					"camera_id", j.cameraID,
					"error", err)
			} else {
				logger.Log.Errorw("Redis store error",
					"camera_id", j.cameraID,
					"error", err)
			}
			metrics.StorageOperations.WithLabelValues("save_frame", "error").Inc()
			return err
		}

		metrics.StorageOperations.WithLabelValues("save_frame", "success").Inc()

		if j.metaPublisher.Enabled() {
			err = j.metaPublisher.PublishMetadata(j.cameraID, j.timestamp, key, width, height, len(j.frameData), "jpeg")
			if err != nil {
				if amqpErr, ok := err.(*amqp.Error); ok && amqpErr.Code == amqp.ChannelError {
					logger.Log.Errorw("Metadata publish error (channel closed)",
						"camera_id", j.cameraID,
						"error", err)
				} else {
					logger.Log.Errorw("Metadata publish error",
						"camera_id", j.cameraID,
						"error", err)
				}
				return err
			}
		}
	}

	return nil
}
