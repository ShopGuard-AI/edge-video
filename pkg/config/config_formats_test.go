package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfigTOML(t *testing.T) {
	cfg, err := LoadConfig("../../config.toml")
	assert.NoError(t, err, "Deveria carregar config.toml sem erros")
	assert.NotNil(t, cfg, "Configuração não deveria ser nil")

	// Validar parâmetros principais
	assert.Equal(t, float64(30), cfg.TargetFPS, "Target FPS deveria ser 30")
	assert.Equal(t, "amqp", cfg.Protocol, "Protocol deveria ser amqp")

	// Validar AMQP
	assert.Equal(t, "amqp://user:password@rabbitmq:5672/supermercado_vhost", cfg.AMQP.AmqpURL)
	assert.Equal(t, "supermercado_exchange", cfg.AMQP.Exchange)
	assert.Equal(t, "camera.", cfg.AMQP.RoutingKeyPrefix)

	// Validar MQTT
	assert.Equal(t, "tcp://localhost:1883", cfg.MQTT.Broker)
	assert.Equal(t, "camera/", cfg.MQTT.TopicPrefix)

	// Validar Optimization
	assert.Equal(t, 20, cfg.Optimization.MaxWorkers, "Max workers deveria ser 20")
	assert.Equal(t, 200, cfg.Optimization.BufferSize, "Buffer size deveria ser 200")
	assert.Equal(t, 5, cfg.Optimization.FrameQuality)
	assert.Equal(t, "1280x720", cfg.Optimization.FrameResolution)
	assert.True(t, cfg.Optimization.UsePersistent)
	assert.Equal(t, 5, cfg.Optimization.CircuitMaxFailures)
	assert.Equal(t, 60, cfg.Optimization.CircuitResetSec)

	// Validar Redis
	assert.True(t, cfg.Redis.Enabled)
	assert.Equal(t, "redis:6379", cfg.Redis.Address)
	assert.Equal(t, 300, cfg.Redis.TTLSeconds)
	assert.Equal(t, "frames", cfg.Redis.Prefix)

	// Validar Metadata
	assert.True(t, cfg.Metadata.Enabled)
	assert.Equal(t, "camera.metadata", cfg.Metadata.Exchange)
	assert.Equal(t, "camera.metadata.event", cfg.Metadata.RoutingKey)

	// Validar Cameras
	assert.Len(t, cfg.Cameras, 5, "Deveria ter 5 câmeras")
	assert.Equal(t, "cam1", cfg.Cameras[0].ID)
	assert.Equal(t, "cam2", cfg.Cameras[1].ID)
	assert.Equal(t, "cam3", cfg.Cameras[2].ID)
	assert.Equal(t, "cam4", cfg.Cameras[3].ID)
	assert.Equal(t, "cam5", cfg.Cameras[4].ID)

	// Validar cálculo derivado
	interval := cfg.GetFrameInterval()
	assert.NotZero(t, interval, "Interval não deveria ser zero")
}

func TestLoadConfigYAML(t *testing.T) {
	cfg, err := LoadConfig("../../config.yaml")
	assert.NoError(t, err, "Deveria carregar config.yaml sem erros")
	assert.NotNil(t, cfg, "Configuração não deveria ser nil")

	// Validar parâmetros principais
	assert.Equal(t, float64(30), cfg.TargetFPS, "Target FPS deveria ser 30")
	assert.Equal(t, "amqp", cfg.Protocol, "Protocol deveria ser amqp")

	// Validar Optimization (valores atualizados)
	assert.Equal(t, 20, cfg.Optimization.MaxWorkers, "Max workers deveria ser 20")
	assert.Equal(t, 200, cfg.Optimization.BufferSize, "Buffer size deveria ser 200")
	assert.Equal(t, 5, cfg.Optimization.FrameQuality)
	assert.Equal(t, "1280x720", cfg.Optimization.FrameResolution)
	assert.True(t, cfg.Optimization.UsePersistent)
	assert.Equal(t, 5, cfg.Optimization.CircuitMaxFailures)
	assert.Equal(t, 60, cfg.Optimization.CircuitResetSec)

	// Validar Redis
	assert.True(t, cfg.Redis.Enabled)
	assert.Equal(t, "redis:6379", cfg.Redis.Address)
	assert.Equal(t, 300, cfg.Redis.TTLSeconds)
	assert.Equal(t, "frames", cfg.Redis.Prefix)
}

func TestConfigParity(t *testing.T) {
	cfgTOML, err := LoadConfig("../../config.toml")
	assert.NoError(t, err)

	cfgYAML, err := LoadConfig("../../config.yaml")
	assert.NoError(t, err)

	// Verificar paridade entre TOML e YAML
	assert.Equal(t, cfgYAML.TargetFPS, cfgTOML.TargetFPS, "Target FPS deveria ser igual em TOML e YAML")
	assert.Equal(t, cfgYAML.Protocol, cfgTOML.Protocol, "Protocol deveria ser igual em TOML e YAML")
	
	// Optimization
	assert.Equal(t, cfgYAML.Optimization.MaxWorkers, cfgTOML.Optimization.MaxWorkers, "Max workers deveria ser igual")
	assert.Equal(t, cfgYAML.Optimization.BufferSize, cfgTOML.Optimization.BufferSize, "Buffer size deveria ser igual")
	assert.Equal(t, cfgYAML.Optimization.FrameQuality, cfgTOML.Optimization.FrameQuality, "Frame quality deveria ser igual")
	assert.Equal(t, cfgYAML.Optimization.UsePersistent, cfgTOML.Optimization.UsePersistent, "Use persistent deveria ser igual")
	assert.Equal(t, cfgYAML.Optimization.CircuitMaxFailures, cfgTOML.Optimization.CircuitMaxFailures, "Circuit failures deveria ser igual")
	assert.Equal(t, cfgYAML.Optimization.CircuitResetSec, cfgTOML.Optimization.CircuitResetSec, "Circuit reset deveria ser igual")
	
	// Redis
	assert.Equal(t, cfgYAML.Redis.TTLSeconds, cfgTOML.Redis.TTLSeconds, "Redis TTL deveria ser igual")
	assert.Equal(t, cfgYAML.Redis.Address, cfgTOML.Redis.Address, "Redis address deveria ser igual")
	
	// Cameras
	assert.Equal(t, len(cfgYAML.Cameras), len(cfgTOML.Cameras), "Número de câmeras deveria ser igual")
}
