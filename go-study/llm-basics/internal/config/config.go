package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	OpenRouterAPIKey string `json:"openrouter_api_key"`
}

func Load() (Config, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return Config{}, fmt.Errorf("locate config package")
	}

	configPath := filepath.Join(filepath.Dir(filename), "..", "..", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", configPath, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", configPath, err)
	}
	if strings.TrimSpace(cfg.OpenRouterAPIKey) == "" {
		return Config{}, fmt.Errorf("openrouter_api_key is required in %s", configPath)
	}

	return cfg, nil
}
