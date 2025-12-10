package monitoring

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"sync"
	"time"

	"edge-video/v2/internal/memory"
	"edge-video/v2/internal/resilience"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsServer gerencia exposição de métricas Prometheus
type MetricsServer struct {
	port     int
	autoPort bool // Se true, tenta portas alternativas se ocupadas
	mu       sync.RWMutex
	cameras  map[string]*CameraMetrics
	registry *prometheus.Registry
}

// CameraMetrics armazena métricas por câmera
type CameraMetrics struct {
	cameraID string

	// Contadores
	framesReceived  prometheus.Counter
	framesPublished prometheus.Counter
	framesDropped   prometheus.Counter
	publishErrors   prometheus.Counter

	// Gauges
	fps              prometheus.Gauge
	publishLatency   prometheus.Gauge
	frameSize        prometheus.Gauge
	circuitBreakerState prometheus.Gauge

	// Histogramas
	publishDuration prometheus.Histogram
	frameSizeHist   prometheus.Histogram
}

// Métricas globais do sistema
var (
	metricsServer *MetricsServer
	metricsOnce   sync.Once

	// Publisher Confirms
	publishConfirmsAck = promauto.NewCounter(prometheus.CounterOpts{
		Name: "edge_video_publisher_confirms_ack_total",
		Help: "Total de ACKs recebidos do RabbitMQ",
	})

	publishConfirmsNack = promauto.NewCounter(prometheus.CounterOpts{
		Name: "edge_video_publisher_confirms_nack_total",
		Help: "Total de NACKs recebidos do RabbitMQ",
	})

	// System Metrics
	systemCPU = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "edge_video_system_cpu_percent",
		Help: "Uso de CPU do processo (%)",
	})

	systemRAM = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "edge_video_system_ram_mb",
		Help: "Uso de RAM do processo (MB)",
	})

	systemGoroutines = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "edge_video_system_goroutines",
		Help: "Número total de goroutines ativas",
	})

	systemGCCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "edge_video_system_gc_total",
		Help: "Número total de GC executados",
	})

	// Circuit Breaker
	circuitBreakerOpen = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "edge_video_circuit_breakers_open",
		Help: "Número de circuit breakers no estado OPEN",
	})

	// Memory Controller
	memoryControllerLevel = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "edge_video_memory_controller_level",
		Help: "Nível do memory controller (0=NORMAL, 1=WARNING, 2=CRITICAL, 3=EMERGENCY)",
	})

	memoryControllerGC = promauto.NewCounter(prometheus.CounterOpts{
		Name: "edge_video_memory_controller_gc_total",
		Help: "Número de GCs manuais executados pelo memory controller",
	})

	// Redis Metrics
	redisStoreTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edge_video_redis_store_total",
		Help: "Total de frames armazenados no Redis",
	}, []string{"camera_id", "status"})

	redisStoreDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "edge_video_redis_store_duration_seconds",
		Help:    "Duração do store no Redis",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
	}, []string{"camera_id"})

	redisGetTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edge_video_redis_get_total",
		Help: "Total de GET operations no Redis",
	}, []string{"status"})

	redisConnectionStatus = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "edge_video_redis_connected",
		Help: "Status da conexão Redis (1=connected, 0=disconnected)",
	})

	// Uptime
	uptimeSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "edge_video_uptime_seconds",
		Help: "Tempo de execução do sistema em segundos",
	})
)

// InitMetricsServer inicializa servidor de métricas Prometheus
func InitMetricsServer(port int, autoPort bool) *MetricsServer {
	metricsOnce.Do(func() {
		metricsServer = &MetricsServer{
			port:     port,
			autoPort: autoPort,
			cameras:  make(map[string]*CameraMetrics),
			registry: prometheus.NewRegistry(),
		}

		// Registra métricas globais no registry padrão
		metricsServer.registry.MustRegister(
			publishConfirmsAck,
			publishConfirmsNack,
			systemCPU,
			systemRAM,
			systemGoroutines,
			systemGCCount,
			circuitBreakerOpen,
			memoryControllerLevel,
			memoryControllerGC,
			uptimeSeconds,
		)

		// Inicia servidor HTTP (em background)
		go metricsServer.serve()

		// Inicia collector de métricas do sistema
		go metricsServer.collectSystemMetrics()

		if autoPort {
			log.Printf("📊 Prometheus metrics server iniciando (porta: %d, auto-port: enabled)...", port)
		} else {
			log.Printf("📊 Prometheus metrics server rodando em http://localhost:%d/metrics", port)
		}
	})

	return metricsServer
}

// RegisterCamera registra métricas para uma câmera
func (ms *MetricsServer) RegisterCamera(cameraID string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, exists := ms.cameras[cameraID]; exists {
		return // Já registrada
	}

	cm := &CameraMetrics{
		cameraID: cameraID,

		framesReceived: promauto.NewCounter(prometheus.CounterOpts{
			Name: "edge_video_frames_received_total",
			Help: "Total de frames recebidos da câmera",
			ConstLabels: prometheus.Labels{
				"camera_id": cameraID,
			},
		}),

		framesPublished: promauto.NewCounter(prometheus.CounterOpts{
			Name: "edge_video_frames_published_total",
			Help: "Total de frames publicados no RabbitMQ",
			ConstLabels: prometheus.Labels{
				"camera_id": cameraID,
			},
		}),

		framesDropped: promauto.NewCounter(prometheus.CounterOpts{
			Name: "edge_video_frames_dropped_total",
			Help: "Total de frames descartados (buffer cheio)",
			ConstLabels: prometheus.Labels{
				"camera_id": cameraID,
			},
		}),

		publishErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "edge_video_publish_errors_total",
			Help: "Total de erros ao publicar frames",
			ConstLabels: prometheus.Labels{
				"camera_id": cameraID,
			},
		}),

		fps: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "edge_video_camera_fps",
			Help: "FPS real da câmera",
			ConstLabels: prometheus.Labels{
				"camera_id": cameraID,
			},
		}),

		publishLatency: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "edge_video_publish_latency_ms",
			Help: "Latência de publicação em milissegundos",
			ConstLabels: prometheus.Labels{
				"camera_id": cameraID,
			},
		}),

		frameSize: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "edge_video_frame_size_bytes",
			Help: "Tamanho do último frame em bytes",
			ConstLabels: prometheus.Labels{
				"camera_id": cameraID,
			},
		}),

		circuitBreakerState: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "edge_video_circuit_breaker_state",
			Help: "Estado do circuit breaker (0=CLOSED, 1=OPEN, 2=HALF_OPEN)",
			ConstLabels: prometheus.Labels{
				"camera_id": cameraID,
			},
		}),

		publishDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name: "edge_video_publish_duration_seconds",
			Help: "Histograma de duração de publicação",
			ConstLabels: prometheus.Labels{
				"camera_id": cameraID,
			},
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms até ~1s
		}),

		frameSizeHist: promauto.NewHistogram(prometheus.HistogramOpts{
			Name: "edge_video_frame_size_bytes_histogram",
			Help: "Histograma de tamanhos de frames",
			ConstLabels: prometheus.Labels{
				"camera_id": cameraID,
			},
			Buckets: prometheus.ExponentialBuckets(10000, 2, 10), // 10KB até ~10MB
		}),
	}

	ms.cameras[cameraID] = cm
	log.Printf("📊 Métricas registradas para câmera: %s", cameraID)
}

// TrackFrameReceived registra recebimento de frame
func (ms *MetricsServer) TrackFrameReceived(cameraID string, frameSize int) {
	ms.mu.RLock()
	cm, ok := ms.cameras[cameraID]
	ms.mu.RUnlock()

	if !ok {
		return
	}

	cm.framesReceived.Inc()
	cm.frameSize.Set(float64(frameSize))
	cm.frameSizeHist.Observe(float64(frameSize))
}

// TrackFramePublished registra publicação de frame
func (ms *MetricsServer) TrackFramePublished(cameraID string, duration time.Duration, success bool) {
	ms.mu.RLock()
	cm, ok := ms.cameras[cameraID]
	ms.mu.RUnlock()

	if !ok {
		return
	}

	if success {
		cm.framesPublished.Inc()
	} else {
		cm.publishErrors.Inc()
	}

	latencyMs := float64(duration.Microseconds()) / 1000.0
	cm.publishLatency.Set(latencyMs)
	cm.publishDuration.Observe(duration.Seconds())
}

// TrackFrameDropped registra descarte de frame
func (ms *MetricsServer) TrackFrameDropped(cameraID string) {
	ms.mu.RLock()
	cm, ok := ms.cameras[cameraID]
	ms.mu.RUnlock()

	if !ok {
		return
	}

	cm.framesDropped.Inc()
}

// UpdateFPS atualiza FPS da câmera
func (ms *MetricsServer) UpdateFPS(cameraID string, fps float64) {
	ms.mu.RLock()
	cm, ok := ms.cameras[cameraID]
	ms.mu.RUnlock()

	if !ok {
		return
	}

	cm.fps.Set(fps)
}

// UpdateCircuitBreakerState atualiza estado do circuit breaker
func (ms *MetricsServer) UpdateCircuitBreakerState(cameraID string, state resilience.CircuitState) {
	ms.mu.RLock()
	cm, ok := ms.cameras[cameraID]
	ms.mu.RUnlock()

	if !ok {
		return
	}

	// 0=CLOSED, 1=OPEN, 2=HALF_OPEN
	stateValue := 0.0
	switch state {
	case resilience.StateOpen:
		stateValue = 1.0
	case resilience.StateHalfOpen:
		stateValue = 2.0
	}

	cm.circuitBreakerState.Set(stateValue)
}

// collectSystemMetrics coleta métricas do sistema periodicamente
func (ms *MetricsServer) collectSystemMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()

	for range ticker.C {
		// Uptime
		uptimeSeconds.Set(time.Since(startTime).Seconds())

		// Goroutines
		systemGoroutines.Set(float64(runtime.NumGoroutine()))

		// Memória
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		systemRAM.Set(float64(m.Alloc) / 1024 / 1024) // MB
		systemGCCount.Add(float64(m.NumGC))
	}
}

// serve inicia servidor HTTP para métricas
func (ms *MetricsServer) serve() {
	http.Handle("/metrics", promhttp.Handler())

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	// Tenta iniciar servidor com retry se autoPort habilitado
	port := ms.port
	maxRetries := 10

	for i := 0; i < maxRetries; i++ {
		addr := fmt.Sprintf(":%d", port)

		// Log de sucesso ANTES de bloquear (ListenAndServe bloqueia a goroutine)
		if i > 0 {
			log.Printf("✓ Metrics server usando porta alternativa: http://localhost:%d/metrics", port)
		} else if !ms.autoPort {
			// Primeira tentativa sem autoPort - já logado em InitMetricsServer
		} else {
			log.Printf("✓ Metrics server rodando em http://localhost:%d/metrics", port)
		}

		err := http.ListenAndServe(addr, nil)

		if err == nil {
			// Sucesso!
			return
		}

		// Se erro e autoPort desabilitado, loga erro e retorna
		if !ms.autoPort {
			log.Printf("❌ Erro ao iniciar metrics server na porta %d: %v", port, err)
			return
		}

		// Se autoPort habilitado, tenta próxima porta
		log.Printf("⚠️  Porta %d ocupada, tentando porta %d...", port, port+1)
		port++

		// Atualiza porta no struct para outros métodos saberem qual porta foi usada
		ms.mu.Lock()
		ms.port = port
		ms.mu.Unlock()
	}

	log.Printf("❌ Falha ao iniciar metrics server após %d tentativas", maxRetries)
}

// Funções auxiliares para atualizar métricas globais

// TrackPublishConfirm atualiza métricas de Publisher Confirms
func TrackPublishConfirm(ack bool) {
	if ack {
		publishConfirmsAck.Inc()
	} else {
		publishConfirmsNack.Inc()
	}

	// Também atualiza profiling stats
	trackPublishConfirmProfile(ack)
}

// UpdateSystemCPU atualiza métrica de CPU
func UpdateSystemCPU(percent float64) {
	systemCPU.Set(percent)
}

// UpdateCircuitBreakersOpen atualiza número de circuit breakers OPEN
func UpdateCircuitBreakersOpen(count int) {
	circuitBreakerOpen.Set(float64(count))
}

// TrackRedisStore registra operação de store no Redis
func TrackRedisStore(cameraID string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "error"
	}

	redisStoreTotal.WithLabelValues(cameraID, status).Inc()
	redisStoreDuration.WithLabelValues(cameraID).Observe(duration.Seconds())
}

// TrackRedisGet registra operação de GET no Redis
func TrackRedisGet(success bool) {
	status := "success"
	if !success {
		status = "error"
	}

	redisGetTotal.WithLabelValues(status).Inc()
}

// UpdateRedisConnectionStatus atualiza status da conexão Redis
func UpdateRedisConnectionStatus(connected bool) {
	if connected {
		redisConnectionStatus.Set(1)
	} else {
		redisConnectionStatus.Set(0)
	}
}

// UpdateMemoryControllerMetrics atualiza métricas do memory controller
func UpdateMemoryControllerMetrics(level memory.MemoryLevel, manualGCs uint64) {
	// Converte level para número: NORMAL=0, WARNING=1, CRITICAL=2, EMERGENCY=3
	levelValue := 0.0
	switch level {
	case memory.MemoryWarning:
		levelValue = 1.0
	case memory.MemoryCritical:
		levelValue = 2.0
	case memory.MemoryEmergency:
		levelValue = 3.0
	}
	memoryControllerLevel.Set(levelValue)

	// GCs manuais (incrementa apenas novos GCs)
	memoryControllerGC.Add(float64(manualGCs))
}
