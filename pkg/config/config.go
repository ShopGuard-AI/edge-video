package config

import "github.com/spf13/viper"

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