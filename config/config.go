package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port             string
	SnapshotsDir     string
	APIKey           string
	DeleteAfterServe bool
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		SnapshotsDir:     getEnv("SNAPSHOTS_DIR", "/snapshots"),
		APIKey:           os.Getenv("API_KEY"),
		DeleteAfterServe: strings.ToLower(getEnv("DELETE_AFTER_SERVE", "true")) == "true",
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API_KEY environment variable is required")
	}

	info, err := os.Stat(cfg.SnapshotsDir)
	if err != nil {
		return nil, fmt.Errorf("SNAPSHOTS_DIR %q: %w", cfg.SnapshotsDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("SNAPSHOTS_DIR %q is not a directory", cfg.SnapshotsDir)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
