package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	FramesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "edge_video_frames_processed_total",
			Help: "Total de frames processados por câmera",
		},
		[]string{"camera_id"},
	)
	
	FramesDropped = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "edge_video_frames_dropped_total",
			Help: "Total de frames descartados por câmera",
		},
		[]string{"camera_id", "reason"},
	)
	
	CaptureLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "edge_video_capture_latency_seconds",
			Help:    "Latência de captura de frames",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"camera_id"},
	)
	
	WorkerPoolQueueSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "edge_video_worker_pool_queue_size",
			Help: "Tamanho atual da fila do worker pool",
		},
		[]string{"pool_name"},
	)
	
	WorkerPoolProcessing = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "edge_video_worker_pool_processing",
			Help: "Número de jobs em processamento",
		},
		[]string{"pool_name"},
	)
	
	BufferSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "edge_video_buffer_size",
			Help: "Tamanho atual do buffer de frames",
		},
		[]string{"camera_id"},
	)
	
	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "edge_video_circuit_breaker_state",
			Help: "Estado do circuit breaker (0=closed, 1=open, 2=half-open)",
		},
		[]string{"breaker_name"},
	)
	
	CameraConnected = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "edge_video_camera_connected",
			Help: "Status de conexão da câmera (0=desconectada, 1=conectada)",
		},
		[]string{"camera_id"},
	)
	
	PublishLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "edge_video_publish_latency_seconds",
			Help:    "Latência de publicação de mensagens",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"publisher_type"},
	)
	
	StorageOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "edge_video_storage_operations_total",
			Help: "Total de operações de armazenamento",
		},
		[]string{"operation", "status"},
	)
	
	FrameSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "edge_video_frame_size_bytes",
			Help:    "Tamanho dos frames em bytes",
			Buckets: []float64{1024, 5120, 10240, 51200, 102400, 512000, 1048576},
		},
		[]string{"camera_id"},
	)
)
