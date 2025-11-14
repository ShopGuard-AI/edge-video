//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc/mgr"
	"golang.org/x/sys/windows/svc/eventlog"
)

func installService() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("Error: Cannot get executable path: %v\n", err)
		return
	}

	m, err := mgr.Connect()
	if err != nil {
		fmt.Printf("Error: Cannot connect to service manager: %v\n", err)
		return
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err == nil {
		s.Close()
		fmt.Printf("Service %s already exists\n", serviceName)
		return
	}

	config := mgr.Config{
		StartType:   mgr.StartAutomatic, // Start automatically with Windows
		DisplayName: "Edge Video Camera Capture Service",
		Description: "Captures frames from RTSP cameras and distributes via RabbitMQ/Redis for real-time processing",
	}

	// Create service
	s, err = m.CreateService(serviceName, exePath, config)
	if err != nil {
		fmt.Printf("Error: Cannot create service: %v\n", err)
		return
	}
	defer s.Close()

	// Setup event log
	err = eventlog.InstallAsEventCreate(serviceName, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		fmt.Printf("Error: Cannot install event log: %v\n", err)
		return
	}

	fmt.Printf("Service %s installed successfully\n", serviceName)
	fmt.Printf("Service executable: %s\n", exePath)
	fmt.Printf("Service will start automatically with Windows\n")
	fmt.Println("Use 'net start EdgeVideoService' or Services.msc to start the service")
}

func uninstallService() {
	m, err := mgr.Connect()
	if err != nil {
		fmt.Printf("Error: Cannot connect to service manager: %v\n", err)
		return
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		fmt.Printf("Service %s not found\n", serviceName)
		return
	}
	defer s.Close()

	// Try to stop service if running
	fmt.Printf("Stopping service %s if running...\n", serviceName)
	s.Control(1) // Stop command
	time.Sleep(3 * time.Second)

	// Delete service
	err = s.Delete()
	if err != nil {
		fmt.Printf("Error: Cannot delete service: %v\n", err)
		return
	}

	// Remove event log
	err = eventlog.Remove(serviceName)
	if err != nil {
		fmt.Printf("Warning: Cannot remove event log: %v\n", err)
	}

	fmt.Printf("Service %s uninstalled successfully\n", serviceName)
}

func startService() {
	m, err := mgr.Connect()
	if err != nil {
		fmt.Printf("Error: Cannot connect to service manager: %v\n", err)
		return
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		fmt.Printf("Error: Service %s not found. Install the service first.\n", serviceName)
		return
	}
	defer s.Close()

	err = s.Start()
	if err != nil {
		fmt.Printf("Error: Cannot start service: %v\n", err)
		return
	}

	fmt.Printf("Service %s started successfully\n", serviceName)
}

func stopService() {
	m, err := mgr.Connect()
	if err != nil {
		fmt.Printf("Error: Cannot connect to service manager: %v\n", err)
		return
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		fmt.Printf("Error: Service %s not found\n", serviceName)
		return
	}
	defer s.Close()

	_, err = s.Control(1) // Stop command
	if err != nil {
		fmt.Printf("Error: Cannot stop service: %v\n", err)
		return
	}

	fmt.Printf("Service %s stop signal sent\n", serviceName)
}

// createConfigFile creates a default config.toml in the installation directory
func createConfigFile(installDir string) error {
	configDir := filepath.Join(installDir, "config")
	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		return fmt.Errorf("cannot create config directory: %v", err)
	}

	configPath := filepath.Join(configDir, "config.toml")
	
	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		return nil // Config already exists, don't overwrite
	}

	configContent := `# Edge Video Configuration File
# Copy this file and modify according to your environment

# FPS for camera capture
target_fps = 30

# Message protocol: "amqp" or "mqtt"
protocol = "amqp"

# AMQP Configuration (RabbitMQ)
[amqp]
amqp_url = "amqp://guest:guest@localhost:5672/"
vhost = "/"
exchange = "cameras"
routing_key_prefix = "camera."

# Performance optimization
[optimization]
max_workers = 20
buffer_size = 200
frame_quality = 5
frame_resolution = "1280x720"
use_persistent = true
circuit_max_failures = 5
circuit_reset_seconds = 60

# Redis storage for frames
[redis]
enabled = false  # Disable by default - configure as needed
address = "localhost:6379"
ttl_seconds = 300
prefix = "frames"

# Metadata publishing
[metadata]
enabled = true
exchange = "camera.metadata"
routing_key = "camera.metadata.event"

# API Registration (optional)
[registration]
enabled = false  # Configure as needed
api_url = "http://localhost:8000/api/register"

# Example camera configuration
# Uncomment and configure according to your cameras
# [[cameras]]
# id = "cam1"
# url = "rtsp://username:password@192.168.1.100:554/stream1"

# [[cameras]]
# id = "cam2" 
# url = "rtsp://username:password@192.168.1.101:554/stream1"
`

	return os.WriteFile(configPath, []byte(configContent), 0644)
}