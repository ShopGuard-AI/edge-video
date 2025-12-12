package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"edge-video/v2/internal/config"
	"edge-video/v2/internal/memory"
	"edge-video/v2/internal/messaging"
	"edge-video/v2/internal/monitoring"
	"edge-video/v2/internal/resilience"
	"edge-video/v2/internal/storage"
	"edge-video/v2/internal/stream"

	"github.com/shirou/gopsutil/v3/cpu"
)

var startTime time.Time

// cleanupOrphanFFmpeg mata processos FFmpeg órfãos de execuções anteriores
func cleanupOrphanFFmpeg() {
	log.Println("🧹 Verificando processos FFmpeg órfãos...")

	var cmd *exec.Cmd

	// Detecta sistema operacional e usa comando apropriado
	if runtime.GOOS == "windows" {
		// Windows: taskkill /F /IM ffmpeg.exe
		cmd = exec.Command("taskkill", "/F", "/IM", "ffmpeg.exe")
	} else {
		// Linux/Unix: pkill -9 ffmpeg
		cmd = exec.Command("pkill", "-9", "ffmpeg")
	}

	// Executa comando de cleanup
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Se erro é "processo não encontrado", está tudo certo
		if runtime.GOOS == "windows" {
			// Windows retorna erro se processo não existe
			if len(output) > 0 && (string(output) == "" || len(output) < 10) {
				log.Println("✓ Nenhum processo FFmpeg órfão encontrado")
				return
			}
		} else {
			// Linux pkill retorna exit code 1 se nenhum processo foi encontrado
			log.Println("✓ Nenhum processo FFmpeg órfão encontrado")
			return
		}
	}

	// Se chegou aqui, processos foram mortos
	if len(output) > 0 {
		log.Printf("✓ Processos FFmpeg órfãos limpos: %s", string(output))
	} else {
		log.Println("✓ Processos FFmpeg órfãos limpos com sucesso")
	}
}

func main() {
	// Marca início do sistema
	startTime = time.Now()

	// Parse flags
	configFile := flag.String("config", "config.yaml", "Arquivo de configuração")
	flag.Parse()

	// Banner
	log.Println("========================================")
	log.Println("  Edge Video V2 - Simple & Reliable")
	log.Println("========================================")

	// Carrega configuração
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("ERRO ao carregar config: %v", err)
	}

	log.Printf("Configuração carregada: %d câmeras, %d FPS, Quality %d",
		len(cfg.Cameras), cfg.FPS, cfg.Quality)

	// Cleanup de processos FFmpeg órfãos (de execuções anteriores)
	cleanupOrphanFFmpeg()

	// Inicia Prometheus metrics server
	metricsServer := monitoring.InitMetricsServer(cfg.Monitoring.MetricsPort, cfg.Monitoring.AutoPort)

	// Inicializa Redis client (se habilitado)
	redisClient, err := storage.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Fatalf("ERRO ao conectar Redis: %v", err)
	}
	if redisClient != nil {
		defer redisClient.Close()
		log.Println("✓ Redis habilitado - frames serão armazenados no Redis")
	} else {
		log.Println("✓ Redis desabilitado - frames vão direto pro RabbitMQ")
	}

	// Inicia pprof HTTP server para debugging de goroutines
	go func() {
		pprofPort := cfg.Monitoring.PprofPort
		autoPort := cfg.Monitoring.AutoPort
		maxRetries := 10

		for i := 0; i < maxRetries; i++ {
			addr := fmt.Sprintf("localhost:%d", pprofPort)

			if i == 0 {
				log.Printf("🔬 pprof server rodando em http://%s/debug/pprof/", addr)
				log.Printf("   Para ver goroutines: http://%s/debug/pprof/goroutine?debug=1", addr)
			} else if i > 0 {
				log.Printf("✓ pprof usando porta alternativa: http://%s/debug/pprof/", addr)
			}

			err := http.ListenAndServe(addr, nil)

			// Se não há erro, servidor parou por alguma razão - retornar
			if err == nil {
				return
			}

			// Se autoPort desabilitado, loga erro e retorna
			if !autoPort {
				log.Printf("⚠️  pprof server falhou: %v", err)
				return
			}

			// Se autoPort habilitado, tenta próxima porta
			if i < maxRetries-1 {
				log.Printf("⚠️  Porta %d ocupada, tentando pprof na porta %d...", pprofPort, pprofPort+1)
				pprofPort++
			}
		}

		log.Printf("❌ Falha ao iniciar pprof server após %d tentativas", maxRetries)
	}()

	// Cria e inicia câmeras usando FFmpeg stream
	// CRÍTICO: Cada câmera tem seu PRÓPRIO publisher para evitar race conditions!
	cameras := make([]*stream.CameraStream, 0, len(cfg.Cameras))
	publishers := make([]*messaging.Publisher, 0, len(cfg.Cameras))

	for _, camCfg := range cfg.Cameras {
		// Usa exchange e routing_key dedicados da câmera
		// Se não especificados, usa os globais como fallback
		exchange := camCfg.Exchange
		if exchange == "" {
			exchange = cfg.AMQP.Exchange
		}

		routingKey := camCfg.RoutingKey
		if routingKey == "" {
			routingKey = cfg.AMQP.RoutingKeyPrefix + camCfg.ID
		}

		// Cria publisher DEDICADO para esta câmera com exchange e routing_key únicos
		publisher, err := messaging.NewPublisher(
			cfg.AMQP.URL,
			exchange,
			routingKey,                       // Passa routing_key COMPLETA ao invés de prefixo
			cfg.AMQP.PrefetchCount,       // QoS: prefetch_count configurável via YAML
			cfg.AMQP.PublisherConfirms,   // Publisher Confirms (DEVE SER FALSE se não houver consumer!)
			redisClient,                      // Passa Redis client (pode ser nil se disabled)
		)
		if err != nil {
			log.Fatalf("ERRO ao criar publisher para %s: %v", camCfg.ID, err)
		}
		// IMPORTANTE: NÃO fecha publishers aqui - faremos isso DEPOIS que câmeras pararem
		publishers = append(publishers, publisher)

		// Registra métricas para esta câmera
		metricsServer.RegisterCamera(camCfg.ID)

		cam := stream.NewCameraStream(
			camCfg.ID,
			camCfg.URL,
			cfg.FPS,
			cfg.Quality,
			publisher,
			cfg.CircuitBreaker, // Passa config do circuit breaker
			metricsServer,         // Passa metrics server para tracking
		)

		cam.Start()
		cameras = append(cameras, cam)

		log.Printf("[%s] Câmera iniciada | Exchange: %s | RoutingKey: %s", camCfg.ID, exchange, routingKey)
	}

	// Monitor de estatísticas (usa primeiro publisher para contagem geral)
	// Cria contexto para statsMonitor poder ser cancelado
	statsCtx, statsCancel := context.WithCancel(context.Background())
	defer statsCancel()
	go statsMonitor(statsCtx, cameras, publishers[0], metricsServer)

	// Inicia profiling monitor
	monitoring.InitSystemStats() // Inicializa tracking de CPU/RAM
	monitoring.StartProfileMonitor()

	// Inicializa Memory Controller (se habilitado)
	var memController *memory.MemoryController
	if cfg.MemoryController.Enabled {
		log.Printf("Memory Controller HABILITADO (max: %d MB)", cfg.MemoryController.MaxMemoryMB)
		memController = memory.NewMemoryController(cfg.MemoryController)

		// Registra callback para tracking
		memController.RegisterCallback(memory.MemoryWarning, func(stats memory.MemoryStats) {
			log.Printf("⚠️  Memory WARNING: %.1f%% (%d MB / %d MB)",
				stats.UsagePercent, stats.AllocMB, cfg.MemoryController.MaxMemoryMB)
		})
		memController.RegisterCallback(memory.MemoryCritical, func(stats memory.MemoryStats) {
			log.Printf("🔴 Memory CRITICAL: %.1f%% (%d MB / %d MB)",
				stats.UsagePercent, stats.AllocMB, cfg.MemoryController.MaxMemoryMB)
		})
		memController.RegisterCallback(memory.MemoryEmergency, func(stats memory.MemoryStats) {
			log.Printf("💀 Memory EMERGENCY: %.1f%% (%d MB / %d MB)",
				stats.UsagePercent, stats.AllocMB, cfg.MemoryController.MaxMemoryMB)
		})

		memController.Start()
		defer memController.Stop()

		// Goroutine para atualizar stats de memory controller no profiling
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for range ticker.C {
				stats := memController.GetStats()
				monitoring.TrackMemoryController(stats.Level, stats.NumGC)
			}
		}()
	} else {
		log.Println("Memory Controller DESABILITADO (pode ser habilitado no config.yaml)")
	}

	// Aguarda sinal de término
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Println("\n✓ Sistema iniciado com sucesso!")
	log.Println("✓ Capturando frames... (Ctrl+C para parar)")

	<-sigChan

	// Shutdown graceful
	log.Println("\n\n🛑 Recebido sinal de término, parando...")

	// 0. Cancela statsMonitor ANTES de parar câmeras
	statsCancel()

	// 1. Para todas as câmeras PRIMEIRO (aguarda goroutines finalizarem)
	log.Println("📹 Parando câmeras...")
	for _, cam := range cameras {
		cam.Stop()
	}
	log.Println("✓ Todas as câmeras paradas")

	// 2. Fecha publishers DEPOIS que câmeras pararam
	log.Println("📤 Fechando publishers...")
	for i, publisher := range publishers {
		if err := publisher.Close(); err != nil {
			log.Printf("⚠️  Erro ao fechar publisher %d: %v", i, err)
		}
	}
	log.Println("✓ Todos os publishers fechados")

	// 3. Fecha Redis client
	if redisClient != nil {
		log.Println("🗄️  Fechando Redis client...")
		if err := redisClient.Close(); err != nil {
			log.Printf("⚠️  Erro ao fechar Redis: %v", err)
		}
		log.Println("✓ Redis client fechado")
	}

	// 4. Para memory controller se habilitado
	if memController != nil {
		log.Println("💾 Parando memory controller...")
		memController.Stop()
		log.Println("✓ Memory controller parado")
	}

	// RELATÓRIO FINAL DE ESTATÍSTICAS
	printFinalReport(cameras, publishers[0], cfg.FPS, metricsServer)

	// RELATÓRIO DE PROFILING
	monitoring.PrintProfileReport()

	// RELATÓRIO REDIS (se habilitado)
	if redisClient != nil {
		storeCount, storeErrors, getCount, getErrors := redisClient.Stats()
		log.Println("\n" + "================================================================")
		log.Println("                    RELATÓRIO REDIS")
		log.Println("================================================================")
		log.Printf("   Frames Armazenados:  %d", storeCount)
		log.Printf("   Erros de Store:      %d", storeErrors)
		log.Printf("   Gets Realizados:     %d", getCount)
		log.Printf("   Erros de Get:        %d", getErrors)
		if storeCount > 0 {
			successRate := float64(storeCount-storeErrors) / float64(storeCount) * 100
			log.Printf("   Taxa de Sucesso:     %.2f%%", successRate)
		}
		log.Println("================================================================")
	}

	log.Println("✓ Sistema encerrado com sucesso")
}

// statsMonitor exibe estatísticas periodicamente
func statsMonitor(ctx context.Context, cameras []*stream.CameraStream, publisher *messaging.Publisher, metricsServer *monitoring.MetricsServer) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("📊 Stats monitor parado")
			return
		case <-ticker.C:
		}
		sep := "============================================================"

		log.Println("\n" + sep)
		log.Println("ESTATÍSTICAS")
		log.Println(sep)

		// Stats do publisher
		pubCount, pubErrors := publisher.Stats()
		errorRate := 0.0
		if pubCount > 0 {
			errorRate = float64(pubErrors) / float64(pubCount) * 100
		}

		connStatus := "✓ CONECTADO"
		if !publisher.IsConnected() {
			connStatus = "⚠ DESCONECTADO"
		}

		log.Printf("Publisher: %s - %d publicados, %d erros (%.2f%%)",
			connStatus, pubCount, pubErrors, errorRate)

		// Stats das câmeras
		openCircuits := uint32(0)
		uptime := time.Since(startTime)

		for _, cam := range cameras {
			count, lastFrame := cam.Stats()
			age := time.Since(lastFrame)

			status := "OK"
			if age > 5*time.Second {
				status = "WARN"
			}

			// Circuit breaker status
			cbStats := cam.GetCircuitBreakerStats()
			if cbStats.Enabled && cbStats.State == resilience.StateOpen {
				status = "CB_OPEN"
				openCircuits++
			}

			cbInfo := ""
			if cbStats.Enabled {
				cbInfo = fmt.Sprintf(" | CB: %s", cbStats.State)
			}

			log.Printf("[%s] %s - Frames: %d, Último: %v atrás%s",
				cam.ID, status, count, age.Round(time.Second), cbInfo)

			// Calcula e atualiza FPS para métricas Prometheus
			if metricsServer != nil && uptime.Seconds() > 0 {
				frameCount, framesReceived, _, _, _ := cam.DetailedStats()
				avgFPSCamera := float64(framesReceived) / uptime.Seconds()
				metricsServer.UpdateFPS(cam.ID, avgFPSCamera)

				// Log detalhado (debug)
				_ = frameCount // Evita warning unused
			}
		}

		// Atualiza métrica de circuit breakers
		monitoring.TrackCircuitBreaker(openCircuits)

		// Calcula e atualiza CPU usage
		if metricsServer != nil {
			// cpu.Percent() retorna array com % de CPU (intervalo de 1 segundo)
			percentages, err := cpu.Percent(time.Second, false)
			if err == nil && len(percentages) > 0 {
				cpuPercent := percentages[0]
				monitoring.UpdateSystemCPU(cpuPercent)
				log.Printf("CPU: %.2f%%", cpuPercent)
			}
		}

		log.Println(sep + "\n")
	}
}

// printFinalReport exibe relatório completo ao encerrar
func printFinalReport(cameras []*stream.CameraStream, publisher *messaging.Publisher, targetFPS int, metricsServer *monitoring.MetricsServer) {
	uptime := time.Since(startTime)

	sep := "================================================================"
	log.Println("\n" + sep)
	log.Println("                    RELATÓRIO FINAL")
	log.Println(sep)

	// Uptime
	log.Printf("⏱  Uptime Total: %v", uptime.Round(time.Second))
	log.Println("")

	// Stats do Publisher
	pubCount, pubErrors := publisher.Stats()
	errorRate := 0.0
	if pubCount > 0 {
		errorRate = float64(pubErrors) / float64(pubCount) * 100
	}

	log.Println("📤 PUBLISHER (RabbitMQ)")
	log.Printf("   Total Publicado:  %d frames", pubCount)
	log.Printf("   Erros:            %d (%.2f%%)", pubErrors, errorRate)

	// Publisher Confirms
	acks, nacks := publisher.ConfirmStats()
	totalConfirms := acks + nacks
	if totalConfirms > 0 {
		confirmRate := float64(acks) / float64(totalConfirms) * 100
		log.Printf("   Confirms (ACK):   %d (%.2f%%)", acks, confirmRate)
		log.Printf("   Rejeições (NACK): %d (%.2f%%)", nacks, 100-confirmRate)

		// Análise de perda
		if totalConfirms < pubCount {
			pending := pubCount - totalConfirms
			log.Printf("   ⏳ Pendentes:     %d frames", pending)
		}
		if nacks > 0 {
			log.Printf("   ⚠️  ALERTA: %d frames foram REJEITADOS pelo RabbitMQ!", nacks)
		}
		if acks == pubCount && nacks == 0 {
			log.Printf("   ✅ 100%% dos frames CONFIRMADOS pelo RabbitMQ!")
		}
	}

	// Throughput
	if uptime.Seconds() > 0 {
		fps := float64(pubCount) / uptime.Seconds()
		log.Printf("   Throughput:       %.2f frames/s", fps)
	}
	log.Println("")

	// Stats por câmera
	log.Println("📹 CÂMERAS")

	var totalFrames uint64
	var totalBytesEstimated uint64

	for _, cam := range cameras {
		frameCount, framesReceived, framesDropped, lastFrame, lastFrameReceived := cam.DetailedStats()
		totalFrames += frameCount

		// Estima tamanho médio (JPEG quality 5 ≈ 50KB)
		estimatedBytes := frameCount * 50000
		totalBytesEstimated += estimatedBytes

		// FPS médio de PUBLICAÇÃO
		avgFPSPublish := 0.0
		if uptime.Seconds() > 0 {
			avgFPSPublish = float64(frameCount) / uptime.Seconds()
		}

		// FPS médio da CÂMERA (recebido do FFmpeg)
		avgFPSCamera := 0.0
		if uptime.Seconds() > 0 {
			avgFPSCamera = float64(framesReceived) / uptime.Seconds()
		}

		// Atualiza métrica Prometheus de FPS
		if metricsServer != nil {
			metricsServer.UpdateFPS(cam.ID, avgFPSCamera)
		}

		// Efficiency (quanto do target FPS foi atingido)
		efficiency := 0.0
		if targetFPS > 0 {
			efficiency = (avgFPSPublish / float64(targetFPS)) * 100
		}

		// Tempo desde último frame
		lastFrameReceivedAge := time.Since(lastFrameReceived)
		status := "✓"
		if lastFrameReceivedAge > 5*time.Second {
			status = "⚠"
		}
		_ = lastFrame // Evita warning unused

		// Calcula % de frames descartados
		dropRate := 0.0
		if framesReceived > 0 {
			dropRate = (float64(framesDropped) / float64(framesReceived)) * 100
		}

		// Circuit Breaker stats
		cbStats := cam.GetCircuitBreakerStats()

		log.Printf("   %s [%s]", status, cam.ID)
		log.Printf("      Frames da Câmera:   %d (%.2f FPS real)", framesReceived, avgFPSCamera)
		log.Printf("      Frames Publicados:  %d (%.2f FPS)", frameCount, avgFPSPublish)
		log.Printf("      Frames Descartados: %d (%.1f%%)", framesDropped, dropRate)
		log.Printf("      FPS Target:         %d", targetFPS)
		log.Printf("      Eficiência:         %.1f%%", efficiency)
		log.Printf("      Volume Estimado:    %.2f MB", float64(estimatedBytes)/(1024*1024))
		log.Printf("      Último da Câmera:   %v atrás", lastFrameReceivedAge.Round(time.Second))

		// Circuit Breaker info
		if cbStats.Enabled {
			log.Printf("      Circuit Breaker:    %s | Calls: %d (✓%d ✗%d 🚫%d) | Changes: %d",
				cbStats.State, cbStats.TotalCalls, cbStats.TotalSuccesses,
				cbStats.TotalFailures, cbStats.TotalRejected, cbStats.StateChanges)
		} else {
			log.Printf("      Circuit Breaker:    DISABLED")
		}

		log.Println("")
	}

	// Totais gerais
	log.Println("📊 TOTAIS GERAIS")
	log.Printf("   Câmeras Ativas:        %d", len(cameras))
	log.Printf("   Total de Frames:       %d", totalFrames)
	log.Printf("   Volume Total Estimado: %.2f MB", float64(totalBytesEstimated)/(1024*1024))

	if uptime.Seconds() > 0 {
		totalFPS := float64(totalFrames) / uptime.Seconds()
		throughputMBps := (float64(totalBytesEstimated) / uptime.Seconds()) / (1024 * 1024)

		log.Printf("   FPS Total Sistema:     %.2f frames/s", totalFPS)
		log.Printf("   Throughput Total:      %.2f MB/s", throughputMBps)

		// Taxa de sucesso
		successRate := 100.0
		if pubCount > 0 {
			successRate = float64(pubCount-pubErrors) / float64(pubCount) * 100
		}
		log.Printf("   Taxa de Sucesso:       %.2f%%", successRate)
	}

	log.Println(sep)
	log.Println("")
}
