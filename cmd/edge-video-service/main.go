package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"

	"github.com/T3-Labs/edge-video/internal/storage"
	"github.com/T3-Labs/edge-video/pkg/config"
	"github.com/T3-Labs/edge-video/pkg/registration"
	"github.com/T3-Labs/edge-video/pkg/worker"
)

const serviceName = "EdgeVideoService"

var version = "dev" // Set during build with -ldflags

type edgeVideoService struct {
	ctx    context.Context
	cancel context.CancelFunc
	elog   debug.Log
}

func main() {
	// Check if running as Windows service
	inService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("Failed to determine if running as service: %v", err)
	}

	if inService {
		runService(serviceName, false)
		return
	}

	// Running in console mode for development/testing
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			installService()
		case "uninstall":
			uninstallService()
		case "start":
			startService()
		case "stop":
			stopService()
		case "console":
			runService(serviceName, true)
		case "version":
			fmt.Printf("Edge Video Service v%s\n", version)
		default:
			fmt.Printf("Usage: %s [install|uninstall|start|stop|console|version]\n", os.Args[0])
			fmt.Println("  install   - Install as Windows service")
			fmt.Println("  uninstall - Uninstall Windows service")
			fmt.Println("  start     - Start the service")
			fmt.Println("  stop      - Stop the service")
			fmt.Println("  console   - Run in console mode (for testing)")
			fmt.Println("  version   - Show version information")
		}
		return
	}

	// Default: try to run as service, fallback to console
	runService(serviceName, true)
}

func runService(name string, isDebug bool) {
	var err error
	var elog debug.Log

	if isDebug {
		elog = debug.New(name)
	} else {
		elog, err = eventlog.Open(name)
		if err != nil {
			return
		}
	}
	defer elog.Close()

	elog.Info(1, fmt.Sprintf("Starting %s service v%s", name, version))

	service := &edgeVideoService{elog: elog}

	if isDebug {
		err = debug.Run(name, service)
	} else {
		err = svc.Run(name, service)
	}

	if err != nil {
		elog.Error(1, fmt.Sprintf("Service failed: %v", err))
		return
	}

	elog.Info(1, fmt.Sprintf("%s service stopped", name))
}

// Execute implements svc.Handler interface
func (s *edgeVideoService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	// Start the edge video application
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.cancel = cancel

	go s.runEdgeVideo(ctx)

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	s.elog.Info(1, "Edge Video service is running")

	// Service control loop
loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				s.elog.Info(1, "Stopping Edge Video service...")
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				break loop
			default:
				s.elog.Error(1, fmt.Sprintf("Unexpected service control request: %v", c))
			}
		case <-ctx.Done():
			break loop
		}
	}

	// Wait a bit for graceful shutdown
	time.Sleep(2 * time.Second)
	changes <- svc.Status{State: svc.Stopped}
	return
}

func (s *edgeVideoService) runEdgeVideo(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			s.elog.Error(1, fmt.Sprintf("Edge Video application panicked: %v", r))
		}
	}()

	// Get executable directory for config file
	execPath, err := os.Executable()
	if err != nil {
		s.elog.Error(1, fmt.Sprintf("Failed to get executable path: %v", err))
		return
	}
	execDir := filepath.Dir(execPath)

	// Load configuration
	configPath := filepath.Join(execDir, "config", "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Fallback to same directory
		configPath = filepath.Join(execDir, "config.toml")
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		s.elog.Error(1, fmt.Sprintf("Failed to load config from %s: %v", configPath, err))
		return
	}

	s.elog.Info(1, fmt.Sprintf("Loaded configuration with %d cameras", len(cfg.Cameras)))

	// Extract vhost for tenant isolation
	vhost := cfg.ExtractVhostFromAMQP()
	s.elog.Info(1, fmt.Sprintf("Using vhost: %s", vhost))

	// Initialize registration client (if enabled)
	if cfg.Registration.Enabled {
		regClient := registration.NewClient(cfg.Registration.APIURL, cfg.Registration.Enabled)
		go regClient.RegisterWithRetry(ctx, cfg, "") // empty logger - service uses eventlog
		s.elog.Info(1, "Registration client started")
	}

	// Initialize Redis store (if enabled)
	if cfg.Redis.Enabled {
		_ = storage.NewRedisStore(
			cfg.Redis.Address,
			cfg.Redis.TTLSeconds,
			cfg.Redis.Prefix,
			vhost,
			cfg.Redis.Enabled,
			cfg.Redis.Username,
			cfg.Redis.Password,
		)
		s.elog.Info(1, "Redis store initialized")
	}

	// Initialize metadata publisher (if enabled)
	if cfg.Metadata.Enabled {
		s.elog.Info(1, "Metadata publisher would be initialized here")
		// TODO: Initialize metadata publisher when interface is stable
	}

	// Initialize worker pool
	_ = worker.NewPool(ctx, cfg.Optimization.MaxWorkers, cfg.Optimization.BufferSize)

	s.elog.Info(1, fmt.Sprintf("Worker pool initialized with %d workers", cfg.Optimization.MaxWorkers))

	// TODO: Initialize cameras when Camera interface is stable
	s.elog.Info(1, fmt.Sprintf("Would initialize %d cameras here", len(cfg.Cameras)))

	// Simplified camera placeholder - just log camera IDs
	for _, camCfg := range cfg.Cameras {
		s.elog.Info(1, fmt.Sprintf("Would start camera: %s -> %s", camCfg.ID, camCfg.URL))
	}

	s.elog.Info(1, fmt.Sprintf("All %d cameras configured successfully", len(cfg.Cameras)))

	// Wait for context cancellation (service stop)
	<-ctx.Done()
	s.elog.Info(1, "Context cancelled, stopping service...")

	s.elog.Info(1, "Service stopped gracefully")
}
