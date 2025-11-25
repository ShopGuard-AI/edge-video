package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/T3-Labs/edge-video/internal/metadata"
	"github.com/T3-Labs/edge-video/internal/storage"
	"github.com/T3-Labs/edge-video/pkg/buffer"
	"github.com/T3-Labs/edge-video/pkg/camera"
	"github.com/T3-Labs/edge-video/pkg/circuit"
	"github.com/T3-Labs/edge-video/pkg/config"
	"github.com/T3-Labs/edge-video/pkg/logger"
	"github.com/T3-Labs/edge-video/pkg/memcontrol"
	"github.com/T3-Labs/edge-video/pkg/metrics"
	"github.com/T3-Labs/edge-video/pkg/mq"
	"github.com/T3-Labs/edge-video/pkg/registration"
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

	// Extrai o vhost da URL AMQP para usar como identificador do cliente
	vhost := cfg.ExtractVhostFromAMQP()

	interval := cfg.GetFrameInterval()
	maxWorkers := cfg.Optimization.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU() * 2
	}

	workerQueueSize := cfg.Optimization.WorkerQueueSize
	if workerQueueSize <= 0 {
		workerQueueSize = cfg.Optimization.BufferSize
		if workerQueueSize <= 0 {
			workerQueueSize = 200
		}
	}

	cameraBufferSize := cfg.Optimization.CameraBufferSize
	if cameraBufferSize <= 0 {
		cameraBufferSize = cfg.Optimization.BufferSize
		if cameraBufferSize <= 0 {
			cameraBufferSize = 200
		}
	}

	persistentBufferSize := cfg.Optimization.PersistentBufSize
	if persistentBufferSize <= 0 {
		persistentBufferSize = cameraBufferSize / 2
		if persistentBufferSize <= 0 {
			persistentBufferSize = 25
		}
	}

	logger.Log.Infow("Configuração carregada",
		"config_file", *configFile,
		"target_fps", cfg.TargetFPS,
		"interval", interval,
		"cameras", len(cfg.Cameras),
		"max_workers", maxWorkers,
		"worker_queue_size", workerQueueSize,
		"camera_buffer_size", cameraBufferSize,
		"persistent_buffer_size", persistentBufferSize,
		"vhost", vhost)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Inicializa o Memory Controller
	var memController *memcontrol.Controller
	if cfg.Memory.Enabled {
		memController = memcontrol.NewController(cfg.Memory.MaxMemoryMB)

		if cfg.Memory.WarningPercent > 0 {
			memConfig := memController.GetConfig()
			memConfig.WarningPercent = cfg.Memory.WarningPercent
			memConfig.CriticalPercent = cfg.Memory.CriticalPercent
			memConfig.EmergencyPercent = cfg.Memory.EmergencyPercent
			memConfig.GCTriggerPercent = cfg.Memory.GCTriggerPercent
			if cfg.Memory.CheckInterval > 0 {
				memConfig.CheckInterval = time.Duration(cfg.Memory.CheckInterval) * time.Second
			}
			memController.UpdateConfig(memConfig)
		}

		memController.Start()
		defer memController.Stop()

		// Registra callbacks para diferentes níveis de memória
		memController.RegisterCallback(memcontrol.MemoryWarning, func(stats memcontrol.MemoryStats) {
			logger.Log.Warnw("Memória em nível de AVISO",
				"usage_percent", fmt.Sprintf("%.2f%%", stats.UsagePercent),
				"alloc_mb", stats.Alloc/1024/1024)
			metrics.MemoryUsagePercent.Set(stats.UsagePercent)
			metrics.MemoryAllocMB.Set(float64(stats.Alloc / 1024 / 1024))
			metrics.MemoryLevel.Set(float64(memcontrol.MemoryWarning))
		})

		memController.RegisterCallback(memcontrol.MemoryCritical, func(stats memcontrol.MemoryStats) {
			logger.Log.Errorw("Memória em nível CRÍTICO - reduzindo velocidade de captura",
				"usage_percent", fmt.Sprintf("%.2f%%", stats.UsagePercent),
				"alloc_mb", stats.Alloc/1024/1024)
			metrics.MemoryUsagePercent.Set(stats.UsagePercent)
			metrics.MemoryAllocMB.Set(float64(stats.Alloc / 1024 / 1024))
			metrics.MemoryLevel.Set(float64(memcontrol.MemoryCritical))
		})

		memController.RegisterCallback(memcontrol.MemoryEmergency, func(stats memcontrol.MemoryStats) {
			logger.Log.Errorw("Memória em EMERGÊNCIA - pausando capturas temporariamente",
				"usage_percent", fmt.Sprintf("%.2f%%", stats.UsagePercent),
				"alloc_mb", stats.Alloc/1024/1024,
				"heap_mb", stats.HeapAlloc/1024/1024)
			metrics.MemoryUsagePercent.Set(stats.UsagePercent)
			metrics.MemoryAllocMB.Set(float64(stats.Alloc / 1024 / 1024))
			metrics.MemoryLevel.Set(float64(memcontrol.MemoryEmergency))
		})

		logger.Log.Infow("Memory Controller ativado",
			"max_memory_mb", cfg.Memory.MaxMemoryMB,
			"warning_percent", cfg.Memory.WarningPercent,
			"critical_percent", cfg.Memory.CriticalPercent,
			"emergency_percent", cfg.Memory.EmergencyPercent)
	}

	// Registra o serviço na API (se habilitado)
	// Tenta registrar com retry automático a cada 1 minuto em caso de falha
	registrationClient := registration.NewClient(cfg.Registration.APIURL, cfg.Registration.Enabled)
	registrationClient.RegisterWithRetry(ctx, cfg, vhost)

	workerPool := worker.NewPool(ctx, maxWorkers, workerQueueSize)
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

	// Cria RedisStore usando o vhost como identificador do cliente
	// Isso garante isolamento entre múltiplas instâncias usando diferentes vhosts
	redisStore := storage.NewRedisStore(
		cfg.Redis.Address,
		cfg.Redis.TTLSeconds,
		cfg.Redis.Prefix,
		vhost,
		cfg.Redis.Enabled,
		cfg.Redis.Username,
		cfg.Redis.Password,
	)
	if redisStore.Enabled() {
		logger.Log.Infow("Redis Store configurado",
			"vhost", vhost,
			"prefix", cfg.Redis.Prefix,
			"ttl_seconds", cfg.Redis.TTLSeconds)
	}

	var metaPublisher *metadata.Publisher
	if amqpPublisher != nil {
		metaPublisher = metadata.NewPublisher(amqpPublisher.GetChannel(), cfg.Metadata.Exchange, cfg.Metadata.RoutingKey, cfg.Metadata.Enabled)
	} else {
		metaPublisher = metadata.NewPublisher(nil, "", "", false)
	}

	// Cria o monitor de câmeras
	cameraMonitor := camera.NewMonitor(ctx, 30*time.Second)
	
	// Configura callbacks para publicar eventos de status
	if metaPublisher.Enabled() {
		cameraMonitor.SetCallbacks(
			// onAllInactive
			func(cameraID string) {
				allStatus := cameraMonitor.GetAllStatus()
				inactiveCount := 0
				for _, status := range allStatus {
					if !status.IsActive {
						inactiveCount++
					}
				}
				
				logger.Log.Errorw("ALERTA CRÍTICO: Nenhuma câmera ativa",
					"total_cameras", len(allStatus),
					"inactive_cameras", inactiveCount)
				
				err := metaPublisher.PublishSystemStatus(
					len(allStatus),
					0,
					inactiveCount,
					"ALERTA: Nenhuma câmera ativa detectada. Sistema em estado crítico.",
				)
				if err != nil {
					logger.Log.Errorw("Erro ao publicar evento de sistema sem câmeras ativas",
						"error", err)
				}
			},
			// onCameraDown
			func(cameraID string) {
				status, exists := cameraMonitor.GetStatus(cameraID)
				if !exists {
					return
				}
				
				logger.Log.Warnw("Câmera ficou inativa",
					"camera_id", cameraID,
					"consecutive_failures", status.ConsecutiveFailures)
				
				err := metaPublisher.PublishCameraStatus(
					cameraID,
					metadata.CameraStateInactive,
					status.ConsecutiveFailures,
					status.LastError,
					"Câmera tornou-se inativa após múltiplas falhas",
				)
				if err != nil {
					logger.Log.Errorw("Erro ao publicar evento de câmera inativa",
						"camera_id", cameraID,
						"error", err)
				}
			},
			// onCameraUp
			func(cameraID string) {
				logger.Log.Infow("Câmera voltou a ficar ativa",
					"camera_id", cameraID)
				
				err := metaPublisher.PublishCameraStatus(
					cameraID,
					metadata.CameraStateActive,
					0,
					nil,
					"Câmera reconectada e operando normalmente",
				)
				if err != nil {
					logger.Log.Errorw("Erro ao publicar evento de câmera ativa",
						"camera_id", cameraID,
						"error", err)
				}
			},
		)
	}
	
	cameraMonitor.Start()

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
		// Registra a câmera no monitor
		cameraMonitor.RegisterCamera(camCfg.ID)
		
		frameBuffer := buffer.NewFrameBuffer(cameraBufferSize)

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
			persistentBufferSize,
			cameraMonitor,
			memController,
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

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		logger.Log.Infow("System stats",
			"pool_stats", stats.String(),
			"memory_alloc_mb", memStats.Alloc/1024/1024,
			"memory_sys_mb", memStats.Sys/1024/1024,
			"num_goroutines", runtime.NumGoroutine())
	}
}
