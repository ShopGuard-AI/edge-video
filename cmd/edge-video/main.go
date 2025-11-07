package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/T3-Labs/edge-video/internal/metadata"
	"github.com/T3-Labs/edge-video/internal/storage"
	"github.com/T3-Labs/edge-video/pkg/buffer"
	"github.com/T3-Labs/edge-video/pkg/camera"
	"github.com/T3-Labs/edge-video/pkg/circuit"
	"github.com/T3-Labs/edge-video/pkg/config"
	"github.com/T3-Labs/edge-video/pkg/logger"
	"github.com/T3-Labs/edge-video/pkg/metrics"
	"github.com/T3-Labs/edge-video/pkg/mq"
	"github.com/T3-Labs/edge-video/pkg/util"
	"github.com/T3-Labs/edge-video/pkg/worker"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Parse command line flags
	configFile := flag.String("config", "config.toml", "Caminho para o arquivo de configuração")
	flag.Parse()

	err := logger.InitLogger(false)
	if err != nil {
		log.Fatalf("erro ao inicializar logger: %v", err)
	}
	defer logger.Sync()
	
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		logger.Log.Fatalw("Erro ao carregar config", "error", err, "config_file", *configFile)
	}

	interval := cfg.GetFrameInterval()
	logger.Log.Infow("Configuração carregada",
		"config_file", *configFile,
		"target_fps", cfg.TargetFPS,
		"interval", interval,
		"cameras", len(cfg.Cameras),
		"max_workers", cfg.Optimization.MaxWorkers,
		"buffer_size", cfg.Optimization.BufferSize)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerPool := worker.NewPool(ctx, cfg.Optimization.MaxWorkers, cfg.Optimization.BufferSize)
	defer workerPool.Close()

	var publisher mq.Publisher
	var amqpPublisher *mq.AMQPPublisher
	if cfg.Protocol == "mqtt" {
		p, err := mq.NewMQTTPublisher(cfg.MQTT.Broker, cfg.MQTT.TopicPrefix)
		if err != nil {
			logger.Log.Fatalw("Erro ao criar mqtt publisher", "error", err)
		}
		publisher = p
	} else {
		p, err := mq.NewAMQPPublisher(cfg.AMQP.AmqpURL, cfg.AMQP.Exchange, cfg.AMQP.RoutingKeyPrefix)
		if err != nil {
			logger.Log.Fatalw("Erro ao criar amqp publisher", "error", err)
		}
		publisher = p
		amqpPublisher = p
	}
	defer publisher.Close()

	redisStore := storage.NewRedisStore(cfg.Redis.Address, cfg.Redis.TTLSeconds, cfg.Redis.Prefix, cfg.Redis.Enabled)

	var metaPublisher *metadata.Publisher
	if amqpPublisher != nil {
		metaPublisher = metadata.NewPublisher(amqpPublisher.GetChannel(), cfg.Metadata.Exchange, cfg.Metadata.RoutingKey, cfg.Metadata.Enabled)
	} else {
		metaPublisher = metadata.NewPublisher(nil, "", "", false)
	}

	var compressor *util.Compressor
	if cfg.Compression.Enabled {
		comp, err := util.NewCompressor(cfg.Compression.Level)
		if err != nil {
			logger.Log.Fatalw("Erro ao criar compressor", "error", err)
		}
		compressor = comp
	}

	go startMetricsServer(":9090")

	go monitorSystem(workerPool)

	for _, camCfg := range cfg.Cameras {
		frameBuffer := buffer.NewFrameBuffer(cfg.Optimization.BufferSize)
		
		resetTimeout := time.Duration(cfg.Optimization.CircuitResetSec) * time.Second
		if resetTimeout == 0 {
			resetTimeout = 60 * time.Second
		}
		
		maxFailures := int64(cfg.Optimization.CircuitMaxFailures)
		if maxFailures == 0 {
			maxFailures = 5
		}
		
		circuitBreaker := circuit.NewBreaker(camCfg.ID, maxFailures, resetTimeout)
		
		capture := camera.NewCapture(
			ctx,
			camera.Config{ID: camCfg.ID, URL: camCfg.URL},
			interval,
			compressor,
			publisher,
			redisStore,
			metaPublisher,
			workerPool,
			frameBuffer,
			circuitBreaker,
			cfg.Optimization.UsePersistent,
		)

		capture.Start()
		
		logger.Log.Infow("Câmera iniciada",
			"camera_id", camCfg.ID,
			"camera_name", camCfg.Name,
			"use_persistent", cfg.Optimization.UsePersistent)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	
	logger.Log.Info("Recebido sinal de finalização, encerrando...")
	cancel()
	
	time.Sleep(2 * time.Second)
	
	logger.Log.Info("Aplicação finalizada")
}

func startMetricsServer(addr string) {
	http.Handle("/metrics", promhttp.Handler())
	
	logger.Log.Infow("Servidor de métricas iniciado", "address", addr)
	
	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Log.Errorw("Erro no servidor de métricas", "error", err)
	}
}

func monitorSystem(pool *worker.Pool) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		stats := pool.Stats()
		
		metrics.WorkerPoolQueueSize.WithLabelValues("main").Set(float64(stats.QueueSize))
		metrics.WorkerPoolProcessing.WithLabelValues("main").Set(float64(stats.Processing))
		
		logger.Log.Infow("System stats",
			"pool_stats", stats.String())
	}
}
