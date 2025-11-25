package config

import (
	"net/url"
	"strings"
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
	WorkerQueueSize    int    `mapstructure:"worker_queue_size"`
	CameraBufferSize   int    `mapstructure:"camera_buffer_size"`
	PersistentBufSize  int    `mapstructure:"persistent_buffer_size"`
	FrameQuality       int    `mapstructure:"frame_quality"`
	FrameResolution    string `mapstructure:"frame_resolution"`
	UsePersistent      bool   `mapstructure:"use_persistent"`
	CircuitMaxFailures int    `mapstructure:"circuit_max_failures"`
	CircuitResetSec    int    `mapstructure:"circuit_reset_seconds"`
}

type RedisConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Address    string `mapstructure:"address"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	TTLSeconds int    `mapstructure:"ttl_seconds"`
	Prefix     string `mapstructure:"prefix"`
}

type MetadataConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Exchange   string `mapstructure:"exchange"`
	RoutingKey string `mapstructure:"routing_key"`
}

type RegistrationConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	APIURL  string `mapstructure:"api_url"`
}

type MemoryConfig struct {
	Enabled          bool    `mapstructure:"enabled"`
	MaxMemoryMB      uint64  `mapstructure:"max_memory_mb"`
	WarningPercent   float64 `mapstructure:"warning_percent"`
	CriticalPercent  float64 `mapstructure:"critical_percent"`
	EmergencyPercent float64 `mapstructure:"emergency_percent"`
	CheckInterval    int     `mapstructure:"check_interval_seconds"`
	GCTriggerPercent float64 `mapstructure:"gc_trigger_percent"`
}

type Config struct {
	TargetFPS           float64            `mapstructure:"target_fps"`
	Protocol            string             `mapstructure:"protocol"`
	UseOptimizedCapture bool               `mapstructure:"use_optimized_capture"`
	AMQP                AMQPConfig         `mapstructure:"amqp"`
	MQTT                MQTTConfig         `mapstructure:"mqtt"`
	Redis               RedisConfig        `mapstructure:"redis"`
	Metadata            MetadataConfig     `mapstructure:"metadata"`
	Registration        RegistrationConfig `mapstructure:"registration"`
	Compression         Compression        `mapstructure:"compression"`
	Optimization        Optimization       `mapstructure:"optimization"`
	Memory              MemoryConfig       `mapstructure:"memory"`
	Cameras             []CameraConfig     `mapstructure:"cameras"`
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

// ExtractVhostFromAMQP extrai o vhost da URL AMQP.
// Exemplo: amqp://user:pass@host:5672/myvhost -> "myvhost"
// Retorna "/" se nenhum vhost for especificado
func (c *Config) ExtractVhostFromAMQP() string {
	if c.AMQP.AmqpURL == "" {
		return "/"
	}

	parsedURL, err := url.Parse(c.AMQP.AmqpURL)
	if err != nil {
		return "/"
	}

	vhost := strings.TrimPrefix(parsedURL.Path, "/")
	if vhost == "" {
		return "/"
	}

	return vhost
}
