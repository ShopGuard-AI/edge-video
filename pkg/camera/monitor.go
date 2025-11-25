package camera

import (
	"context"
	"sync"
	"time"

	"github.com/T3-Labs/edge-video/pkg/logger"
	"github.com/T3-Labs/edge-video/pkg/metrics"
)

type CameraStatus struct {
	CameraID             string
	IsActive             bool
	LastSuccessfulCapture time.Time
	ConsecutiveFailures  int
	LastError            error
}

type Monitor struct {
	ctx      context.Context
	mu       sync.RWMutex
	cameras  map[string]*CameraStatus
	interval time.Duration
	
	onAllInactive func()
	onCameraDown  func(cameraID string)
	onCameraUp    func(cameraID string)
}

func NewMonitor(ctx context.Context, interval time.Duration) *Monitor {
	return &Monitor{
		ctx:      ctx,
		cameras:  make(map[string]*CameraStatus),
		interval: interval,
	}
}

func (m *Monitor) SetCallbacks(onAllInactive, onCameraDown, onCameraUp func(string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if onAllInactive != nil {
		m.onAllInactive = func() { onAllInactive("") }
	}
	m.onCameraDown = onCameraDown
	m.onCameraUp = onCameraUp
}

func (m *Monitor) RegisterCamera(cameraID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.cameras[cameraID] = &CameraStatus{
		CameraID:             cameraID,
		IsActive:             false,
		LastSuccessfulCapture: time.Time{},
		ConsecutiveFailures:  0,
	}
	
	logger.Log.Infow("Câmera registrada no monitor",
		"camera_id", cameraID)
}

func (m *Monitor) RecordSuccess(cameraID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	status, exists := m.cameras[cameraID]
	if !exists {
		return
	}
	
	wasInactive := !status.IsActive
	
	status.LastSuccessfulCapture = time.Now()
	status.ConsecutiveFailures = 0
	status.IsActive = true
	status.LastError = nil
	
	metrics.LastSuccessfulCapture.WithLabelValues(cameraID).Set(float64(time.Now().Unix()))
	metrics.CameraConnected.WithLabelValues(cameraID).Set(1)
	
	if wasInactive && m.onCameraUp != nil {
		go m.onCameraUp(cameraID)
	}
	
	m.updateActiveCamerasCount()
}

func (m *Monitor) RecordFailure(cameraID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	status, exists := m.cameras[cameraID]
	if !exists {
		return
	}
	
	wasActive := status.IsActive
	status.ConsecutiveFailures++
	status.LastError = err
	
	if status.ConsecutiveFailures >= 3 {
		status.IsActive = false
		metrics.CameraConnected.WithLabelValues(cameraID).Set(0)
		
		if wasActive && m.onCameraDown != nil {
			go m.onCameraDown(cameraID)
		}
	}
	
	m.updateActiveCamerasCount()
	
	if m.countActiveCameras() == 0 && m.onAllInactive != nil {
		go m.onAllInactive()
	}
}

func (m *Monitor) GetStatus(cameraID string) (CameraStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	status, exists := m.cameras[cameraID]
	if !exists {
		return CameraStatus{}, false
	}
	
	return *status, true
}

func (m *Monitor) GetAllStatus() map[string]CameraStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[string]CameraStatus, len(m.cameras))
	for id, status := range m.cameras {
		result[id] = *status
	}
	
	return result
}

func (m *Monitor) countActiveCameras() int {
	count := 0
	for _, status := range m.cameras {
		if status.IsActive {
			count++
		}
	}
	return count
}

func (m *Monitor) updateActiveCamerasCount() {
	count := m.countActiveCameras()
	metrics.ActiveCamerasCount.Set(float64(count))
}

func (m *Monitor) Start() {
	go m.monitorLoop()
}

func (m *Monitor) monitorLoop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			logger.Log.Info("Monitor de câmeras encerrado")
			return
			
		case <-ticker.C:
			m.checkCameraHealth()
		}
	}
}

func (m *Monitor) checkCameraHealth() {
	m.mu.RLock()
	allStatus := make([]CameraStatus, 0, len(m.cameras))
	for _, status := range m.cameras {
		allStatus = append(allStatus, *status)
	}
	m.mu.RUnlock()
	
	activeCount := 0
	inactiveCount := 0
	
	for _, status := range allStatus {
		if status.IsActive {
			activeCount++
			
			timeSinceCapture := time.Since(status.LastSuccessfulCapture)
			if timeSinceCapture > 5*time.Minute {
				logger.Log.Warnw("Câmera ativa mas sem capturas recentes",
					"camera_id", status.CameraID,
					"time_since_capture", timeSinceCapture.String())
			}
		} else {
			inactiveCount++
		}
	}
	
	totalCameras := len(allStatus)
	
	if totalCameras > 0 {
		if activeCount == 0 {
			logger.Log.Errorw("ALERTA: Nenhuma câmera ativa detectada",
				"total_cameras", totalCameras,
				"inactive_count", inactiveCount)
		} else if inactiveCount > 0 {
			logger.Log.Warnw("Algumas câmeras estão inativas",
				"active_count", activeCount,
				"inactive_count", inactiveCount,
				"total_cameras", totalCameras)
		}
	}
	
	logger.Log.Debugw("Status de câmeras",
		"active", activeCount,
		"inactive", inactiveCount,
		"total", totalCameras)
}
