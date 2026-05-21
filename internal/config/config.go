package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultRefreshInterval = 30 * time.Second

type Config struct {
	ServerURL       string
	APIKey          string
	RefreshInterval time.Duration
}

type rawConfig struct {
	ServerURL       string `yaml:"server_url"`
	APIKey          string `yaml:"api_key"`
	RefreshInterval int    `yaml:"refresh_interval,omitempty"` // seconds
}

func Load() (*Config, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine config directory: %w", err)
	}

	path := filepath.Join(dir, "terdut-tui", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found at %s\n\nCreate it with:\n  server_url: https://terdut.example.com\n  api_key: <your-api-key>", path)
		}
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid config file: %w", err)
	}

	if raw.ServerURL == "" {
		return nil, fmt.Errorf("config: 'server_url' is required")
	}
	if raw.APIKey == "" {
		return nil, fmt.Errorf("config: 'api_key' is required")
	}

	interval := defaultRefreshInterval
	if raw.RefreshInterval > 0 {
		interval = time.Duration(raw.RefreshInterval) * time.Second
	}

	return &Config{
		ServerURL:       raw.ServerURL,
		APIKey:          raw.APIKey,
		RefreshInterval: interval,
	}, nil
}
