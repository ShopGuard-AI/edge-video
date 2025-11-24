package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfigTOML(t *testing.T) {
	cfg, err := LoadConfig("../../config.toml")
	require.NoError(t, err, "Deveria carregar config.toml sem erros")
	require.NotNil(t, cfg, "Configuração não deveria ser nil")

	require.Greater(t, cfg.TargetFPS, float64(0))
	require.NotEmpty(t, cfg.Protocol)
	require.NotEmpty(t, cfg.AMQP.AmqpURL)
	require.NotEmpty(t, cfg.AMQP.Exchange)
	require.NotEmpty(t, cfg.MQTT.Broker)
	require.NotEmpty(t, cfg.MQTT.TopicPrefix)
	require.NotZero(t, cfg.Optimization.BufferSize)
	require.NotZero(t, cfg.Redis.TTLSeconds)
	require.NotZero(t, len(cfg.Cameras))

	interval := cfg.GetFrameInterval()
	require.NotZero(t, interval, "Interval não deveria ser zero")
}

func TestConfigParity(t *testing.T) {
	cfg, err := LoadConfig("../../config.toml")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Greater(t, cfg.Optimization.MaxWorkers, 0, "Max workers deve ser > 0")
	require.Greater(t, cfg.Optimization.BufferSize, 0, "Buffer size deve ser > 0")
	require.Greater(t, cfg.Redis.TTLSeconds, 0, "Redis TTL deve ser > 0")
	require.Greater(t, len(cfg.Cameras), 0, "Deve haver ao menos uma câmera configurada")
}
