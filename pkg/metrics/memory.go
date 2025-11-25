package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MemoryUsagePercent = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "edge_video_memory_usage_percent",
		Help: "Porcentagem de uso de memória atual",
	})

	MemoryAllocMB = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "edge_video_memory_alloc_mb",
		Help: "Memória alocada em megabytes",
	})

	MemoryLevel = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "edge_video_memory_level",
		Help: "Nível de memória atual (0=Normal, 1=Warning, 2=Critical, 3=Emergency)",
	})

	MemoryGCCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "edge_video_memory_gc_total",
		Help: "Número total de coletas de lixo forçadas",
	})

	CameraThrottled = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edge_video_camera_throttled_total",
		Help: "Número de vezes que uma câmera foi desacelerada por memória",
	}, []string{"camera_id"})

	CameraPaused = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edge_video_camera_paused_total",
		Help: "Número de vezes que uma câmera foi pausada por memória",
	}, []string{"camera_id"})
)
