package main

import (
	"fmt"
	"github.com/T3-Labs/edge-video/pkg/config"
)

func main() {
	cfg, err := config.LoadConfig("config.toml")
	if err != nil {
		fmt.Printf("❌ Erro ao carregar config: %v\n", err)
		return
	}

	fmt.Println("✅ Configuração carregada com sucesso!")
	fmt.Println("\n=== Parâmetros Principais ===")
	fmt.Printf("Target FPS: %v\n", cfg.TargetFPS)
	fmt.Printf("Protocol: %s\n", cfg.Protocol)
	
	fmt.Println("\n=== AMQP ===")
	fmt.Printf("AMQP URL: %s\n", cfg.AMQP.AmqpURL)
	fmt.Printf("Exchange: %s\n", cfg.AMQP.Exchange)
	fmt.Printf("Routing Key Prefix: %s\n", cfg.AMQP.RoutingKeyPrefix)
	
	fmt.Println("\n=== MQTT ===")
	fmt.Printf("Broker: %s\n", cfg.MQTT.Broker)
	fmt.Printf("Topic Prefix: %s\n", cfg.MQTT.TopicPrefix)
	
	fmt.Println("\n=== Optimization ===")
	fmt.Printf("Max Workers: %d\n", cfg.Optimization.MaxWorkers)
	fmt.Printf("Buffer Size: %d\n", cfg.Optimization.BufferSize)
	fmt.Printf("Frame Quality: %d\n", cfg.Optimization.FrameQuality)
	fmt.Printf("Frame Resolution: %s\n", cfg.Optimization.FrameResolution)
	fmt.Printf("Use Persistent: %v\n", cfg.Optimization.UsePersistent)
	fmt.Printf("Circuit Max Failures: %d\n", cfg.Optimization.CircuitMaxFailures)
	fmt.Printf("Circuit Reset Seconds: %d\n", cfg.Optimization.CircuitResetSec)
	
	fmt.Println("\n=== Redis ===")
	fmt.Printf("Enabled: %v\n", cfg.Redis.Enabled)
	fmt.Printf("Address: %s\n", cfg.Redis.Address)
	fmt.Printf("TTL Seconds: %d\n", cfg.Redis.TTLSeconds)
	fmt.Printf("Prefix: %s\n", cfg.Redis.Prefix)
	
	fmt.Println("\n=== Metadata ===")
	fmt.Printf("Enabled: %v\n", cfg.Metadata.Enabled)
	fmt.Printf("Exchange: %s\n", cfg.Metadata.Exchange)
	fmt.Printf("Routing Key: %s\n", cfg.Metadata.RoutingKey)
	
	fmt.Println("\n=== Cameras ===")
	fmt.Printf("Total: %d câmeras\n", len(cfg.Cameras))
	for i, cam := range cfg.Cameras {
		fmt.Printf("  [%d] ID: %s\n", i+1, cam.ID)
	}
	
	interval := cfg.GetFrameInterval()
	fmt.Printf("\n=== Cálculo Derivado ===\n")
	fmt.Printf("Frame Interval: %v\n", interval)
	fmt.Printf("FPS Efetivo: %.2f\n", float64(1)/interval.Seconds())
}
