package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Address             string
	DatabaseURL         string
	AllowDebugEndpoints bool
	ShutdownTimeout     time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Address:         getenv("APP_ADDR", ":8080"),
		DatabaseURL:     getenv("DATABASE_URL", "postgres://energy:energy@localhost:5432/energy?sslmode=disable"),
		ShutdownTimeout: 10 * time.Second,
	}

	debug, err := strconv.ParseBool(getenv("ALLOW_DEBUG_ENDPOINTS", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("parse ALLOW_DEBUG_ENDPOINTS: %w", err)
	}
	cfg.AllowDebugEndpoints = debug

	shutdownTimeout, err := time.ParseDuration(getenv("SHUTDOWN_TIMEOUT", "10s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
	}
	cfg.ShutdownTimeout = shutdownTimeout

	return cfg, nil
}

func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
