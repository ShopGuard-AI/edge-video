package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/T3-Labs/edge-video/pkg/camera"
	"github.com/T3-Labs/edge-video/pkg/config"
	"github.com/T3-Labs/edge-video/pkg/mq"
	"github.com/T3-Labs/edge-video/pkg/util"
)

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("erro ao carregar config: %v", err)
	}

	interval := time.Duration(cfg.IntervalMS) * time.Millisecond

	var publisher mq.Publisher
	if cfg.Protocol == "mqtt" {
		p, err := mq.NewMQTTPublisher(cfg.MQTT.Broker, cfg.MQTT.TopicPrefix)
		if err != nil {
			log.Fatalf("erro criar mqtt publisher: %v", err)
		}
		publisher = p
	} else {
		p, err := mq.NewAMQPPublisher(cfg.AMQP.AmqpURL, cfg.AMQP.Exchange, cfg.AMQP.RoutingKeyPrefix)
		if err != nil {
			log.Fatalf("erro criar amqp publisher: %v", err)
		}
		publisher = p
	}
	defer publisher.Close()

	var compressor *util.Compressor
	if cfg.Compression.Enabled {
		comp, err := util.NewCompressor(cfg.Compression.Level)
		if err != nil {
			log.Fatalf("erro criar compressor: %v", err)
		}
		compressor = comp
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, camCfg := range cfg.Cameras {
		cap := camera.NewCapture(ctx, camera.Config{ID: camCfg.ID, URL: camCfg.URL}, interval, compressor, publisher)
		cap.Start()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("recebido sinal, finalizando...")
	cancel()
	time.Sleep(500 * time.Millisecond)
}