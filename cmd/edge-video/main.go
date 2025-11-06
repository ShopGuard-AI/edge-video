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
	"github.com/T3-Labs/edge-video/internal/metadata"
	"github.com/T3-Labs/edge-video/internal/storage"
)

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("erro ao carregar config: %v", err)
	}

	// O intervalo agora é calculado a partir de TargetFPS
	interval := cfg.GetFrameInterval()

	var publisher mq.Publisher
	var amqpPublisher *mq.AMQPPublisher
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
		amqpPublisher = p
	}
	defer publisher.Close()

	// Inicializa o Redis Store
	redisStore := storage.NewRedisStore(cfg.Redis.Address, cfg.Redis.TTLSeconds, cfg.Redis.Prefix, cfg.Redis.Enabled)

	// Inicializa o Metadata Publisher
	var metaPublisher *metadata.Publisher
	if amqpPublisher != nil {
		metaPublisher = metadata.NewPublisher(amqpPublisher.GetChannel(), cfg.Metadata.Exchange, cfg.Metadata.RoutingKey, cfg.Metadata.Enabled)
	} else {
		metaPublisher = metadata.NewPublisher(nil, "", "", false) // Desabilitado se não for AMQP
	}

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
		capture := camera.NewCapture(
			ctx,
			camera.Config{ID: camCfg.ID, URL: camCfg.URL},
			interval,
			compressor,
			publisher,
			redisStore,
			metaPublisher,
		)

		capture.Start()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("recebido sinal, finalizando...")
	cancel()
	time.Sleep(500 * time.Millisecond)
}
