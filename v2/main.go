package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Parse flags
	configFile := flag.String("config", "config.yaml", "Arquivo de configuração")
	flag.Parse()

	// Banner
	log.Println("========================================")
	log.Println("  Edge Video V2 - Simple & Reliable")
	log.Println("========================================")

	// Carrega configuração
	config, err := LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("ERRO ao carregar config: %v", err)
	}

	log.Printf("Configuração carregada: %d câmeras, %d FPS, Quality %d",
		len(config.Cameras), config.FPS, config.Quality)

	// Cria publisher
	publisher, err := NewPublisher(
		config.AMQP.URL,
		config.AMQP.Exchange,
		config.AMQP.RoutingKeyPrefix,
	)
	if err != nil {
		log.Fatalf("ERRO ao criar publisher: %v", err)
	}
	defer publisher.Close()

	// Cria e inicia câmeras usando FFmpeg stream
	cameras := make([]*CameraStream, 0, len(config.Cameras))

	for _, camCfg := range config.Cameras {
		cam := NewCameraStream(
			camCfg.ID,
			camCfg.URL,
			config.FPS,
			config.Quality,
			publisher,
		)

		cam.Start()
		cameras = append(cameras, cam)

		log.Printf("[%s] Câmera iniciada", camCfg.ID)
	}

	// Monitor de estatísticas
	go statsMonitor(cameras, publisher)

	// Aguarda sinal de término
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Println("\n✓ Sistema iniciado com sucesso!")
	log.Println("✓ Capturando frames... (Ctrl+C para parar)\n")

	<-sigChan

	// Shutdown graceful
	log.Println("\n\n🛑 Recebido sinal de término, parando...")

	for _, cam := range cameras {
		cam.Stop()
	}

	time.Sleep(500 * time.Millisecond)

	log.Println("✓ Sistema encerrado com sucesso")
}

// statsMonitor exibe estatísticas periodicamente
func statsMonitor(cameras []*CameraStream, publisher *Publisher) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
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

		log.Printf("Publisher: %d publicados, %d erros (%.2f%%)",
			pubCount, pubErrors, errorRate)

		// Stats das câmeras
		for _, cam := range cameras {
			count, lastFrame := cam.Stats()
			age := time.Since(lastFrame)

			status := "OK"
			if age > 5*time.Second {
				status = "WARN"
			}

			log.Printf("[%s] %s - Frames: %d, Último: %v atrás",
				cam.ID, status, count, age.Round(time.Second))
		}

		log.Println(sep + "\n")
	}
}
