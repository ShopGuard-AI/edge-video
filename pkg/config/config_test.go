package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	content := `
target_fps: 12.5
protocol: "amqp"
amqp:
  amqp_url: "amqp://guest:guest@localhost:5672/"
  exchange: "test_exchange"
  routing_key_prefix: "test."
cameras:
  - id: "cam1"
    url: "rtsp://test.com/1"
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString(content)
	assert.NoError(t, err)
	tmpfile.Close()

	cfg, err := LoadConfig(tmpfile.Name())
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	assert.Equal(t, 12.5, cfg.TargetFPS)
	assert.Equal(t, "amqp", cfg.Protocol)
	assert.Equal(t, "amqp://guest:guest@localhost:5672/", cfg.AMQP.AmqpURL)
	assert.Equal(t, "test_exchange", cfg.AMQP.Exchange)
	assert.Equal(t, "test.", cfg.AMQP.RoutingKeyPrefix)
	assert.Len(t, cfg.Cameras, 1)
	assert.Equal(t, "cam1", cfg.Cameras[0].ID)
	assert.Equal(t, "rtsp://test.com/1", cfg.Cameras[0].URL)
	assert.Equal(t, time.Duration(float64(time.Second)/12.5), cfg.GetFrameInterval())
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("non_existent_file.yaml")
	assert.Error(t, err)
}
