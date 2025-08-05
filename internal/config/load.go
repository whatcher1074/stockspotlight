package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Config defines the app settings loaded from config/app.yaml
type Config struct {
	APIKey          string `yaml:"polygon_api_key"`
	CacheTTL        int    `yaml:"cache_ttl_seconds"`
	PollingInterval int    `yaml:"polling_interval_seconds"`
	TickerLimit     int    `yaml:"ticker_limit"`
}

// Load reads the YAML config file and returns the Config struct
func Load(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	err = yaml.Unmarshal(file, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("polygon_api_key is required in config")
	}

	return &cfg, nil
}
