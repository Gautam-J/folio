package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AccessToken        string  `yaml:"access_token"`
	Port               int     `yaml:"port"`
	Latitude           float64 `yaml:"latitude"`
	Longitude          float64 `yaml:"longitude"`
	RefreshRateSeconds int     `yaml:"refresh_rate_seconds"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("config: access_token is required")
	}
	if cfg.Port == 0 {
		return nil, fmt.Errorf("config: port is required")
	}
	if cfg.RefreshRateSeconds <= 0 {
		return nil, fmt.Errorf("config: refresh_rate_seconds must be positive")
	}

	return &cfg, nil
}
