package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/T3-Labs/edge-video/internal/camera"
	"github.com/T3-Labs/edge-video/internal/mq"
	"github.com/T3-Labs/edge-video/internal/util"

	"github.com/spf13/viper"
)

type CameraConfig struct {
	ID  string `mapstructure:"id"`
	URL string `mapstructure:"url"`
}


type Config struct {
	IntervalMS          int `mapstructure:"interval_ms"`
	ProcessEveryNFrames int `mapstructure:"process_every_n_frames"`
	Protocol            string `mapstructure:"protocol"`
	AMQP                struct {
		AmqpURL          string `mapstructure:"amqp_url"`
		Exchange         string `mapstructure:"exchange"`
		RoutingKeyPrefix string `mapstructure:"routing_key_prefix"`
	} `mapstructure:"amqp"`
	MQTT struct {
		Broker      string `mapstructure:"broker"`
		TopicPrefix string `mapstructure:"topic_prefix"`
	} `mapstructure:"mqtt"`
	Compression struct {
		Enabled bool `mapstructure:"enabled"`
		Level   int  `mapstructure:"level"`
	} `mapstructure:"compression"`
	Cameras []CameraConfig `mapstructure:"cameras"`
}


func loadConfig() (*Config, error) {
viper.SetConfigFile("config.yaml")
if err := viper.ReadInConfig(); err != nil {
return nil, err
}
var cfg Config
if err := viper.Unmarshal(&cfg); err != nil {
return nil, err
}
return &cfg, nil
}


func main() {
cfg, err := loadConfig()
if err != nil {
log.Fatalf("erro ao carregar config: %v", err)
}


interval := time.Duration(cfg.IntervalMS) * time.Millisecond


// cria publisher
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


// compressor
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


// start captures
for _, camCfg := range cfg.Cameras {
cap := camera.NewCapture(ctx, camera.CameraConfig{ID: camCfg.ID, URL: camCfg.URL}, interval, compressor, publisher)
cap.Start()
}


// handle shutdown
sig := make(chan os.Signal, 1)
signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
<-sig
log.Println("recebido sinal, finalizando...")
cancel()
// small wait to let goroutines finish
time.Sleep(500 * time.Millisecond)
}