package config

import (
	"time"

	"github.com/spf13/viper"
)

type CameraConfig struct {
	ID   string `mapstructure:"id"`
	Name string `mapstructure:"name"`
	URL  string `mapstructure:"url"`
}

type AMQPConfig struct {
	AmqpURL          string `mapstructure:"amqp_url"`
	Exchange         string `mapstructure:"exchange"`
	RoutingKeyPrefix string `mapstructure:"routing_key_prefix"`
}

type MQTTConfig struct {
	Broker      string `mapstructure:"broker"`
	TopicPrefix string `mapstructure:"topic_prefix"`
}

type Compression struct {
	Enabled bool `mapstructure:"enabled"`
	Level   int  `mapstructure:"level"`
}

type Optimization struct {
	MaxWorkers         int    `mapstructure:"max_workers"`
	BufferSize         int    `mapstructure:"buffer_size"`
	FrameQuality       int    `mapstructure:"frame_quality"`
	FrameResolution    string `mapstructure:"frame_resolution"`
	UsePersistent      bool   `mapstructure:"use_persistent"`
	CircuitMaxFailures int    `mapstructure:"circuit_max_failures"`
	CircuitResetSec    int    `mapstructure:"circuit_reset_seconds"`
}

type RedisConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Address    string `mapstructure:"address"`
	TTLSeconds int    `mapstructure:"ttl_seconds"`
	Prefix     string `mapstructure:"prefix"`
}

type MetadataConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Exchange   string `mapstructure:"exchange"`
	RoutingKey string `mapstructure:"routing_key"`
}

type Config struct {
	TargetFPS           float64        `mapstructure:"target_fps"`
	Protocol            string         `mapstructure:"protocol"`
	UseOptimizedCapture bool           `mapstructure:"use_optimized_capture"`
	AMQP                AMQPConfig     `mapstructure:"amqp"`
	MQTT                MQTTConfig     `mapstructure:"mqtt"`
	Redis               RedisConfig    `mapstructure:"redis"`
	Metadata            MetadataConfig `mapstructure:"metadata"`
	Compression         Compression    `mapstructure:"compression"`
	Optimization        Optimization   `mapstructure:"optimization"`
	Cameras             []CameraConfig `mapstructure:"cameras"`
}

// GetFrameInterval calcula o intervalo de tempo entre os frames com base no TargetFPS.
// Retorna um intervalo padrão de 2 FPS se o valor for inválido.
func (c *Config) GetFrameInterval() time.Duration {
	if c.TargetFPS <= 0 {
		return time.Second / 2 // Padrão: 2 FPS
	}
	return time.Duration(float64(time.Second) / c.TargetFPS)
}

func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
