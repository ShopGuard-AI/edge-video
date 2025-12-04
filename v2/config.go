package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config representa a configuração do sistema
type Config struct {
	FPS     int         `yaml:"fps"`
	Quality int         `yaml:"quality"`
	AMQP    AMQPConfig  `yaml:"amqp"`
	Cameras []CamConfig `yaml:"cameras"`
}

// AMQPConfig configuração do RabbitMQ
type AMQPConfig struct {
	URL              string `yaml:"url"`
	Exchange         string `yaml:"exchange"`
	RoutingKeyPrefix string `yaml:"routing_key_prefix"`
}

// CamConfig configuração de câmera
type CamConfig struct {
	ID  string `yaml:"id"`
	URL string `yaml:"url"`
}

// LoadConfig carrega configuração do arquivo YAML
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler arquivo: %w", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear YAML: %w", err)
	}

	// Validações
	if config.FPS <= 0 || config.FPS > 60 {
		return nil, fmt.Errorf("FPS inválido: %d (deve ser 1-60)", config.FPS)
	}

	if config.Quality < 2 || config.Quality > 31 {
		return nil, fmt.Errorf("Quality inválido: %d (deve ser 2-31)", config.Quality)
	}

	if len(config.Cameras) == 0 {
		return nil, fmt.Errorf("nenhuma câmera configurada")
	}

	return &config, nil
}
