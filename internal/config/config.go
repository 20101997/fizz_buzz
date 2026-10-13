// Package config loads the service configuration from the environment.
package config

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"
)

// Stats store backends.
const (
	StatsStoreMemory = "memory"
	StatsStoreRedis  = "redis"
)

// Config is the configuration of the service.
type Config struct {
	// HTTPAddr is the address the HTTP server listens on.
	HTTPAddr string
	LogLevel slog.Level
	// ShutdownTimeout is how long in-flight requests get to complete when
	// the server stops.
	ShutdownTimeout time.Duration

	// MaxLimit is the highest accepted value for the limit parameter.
	MaxLimit int
	// MaxStringLength is the longest accepted str1 or str2, in characters.
	MaxStringLength int

	// StatsStore is the backend holding the statistics.
	StatsStore string
	// StatsTimeout bounds every call to the stats store.
	StatsTimeout time.Duration

	RedisAddr     string
	RedisPassword string
	RedisDB       int
}

// Load builds the configuration from environment variables, read through
// getenv (typically os.Getenv). Unset variables fall back to their default.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		HTTPAddr:      stringVar(getenv, "HTTP_ADDR", ":8080"),
		StatsStore:    stringVar(getenv, "STATS_STORE", StatsStoreMemory),
		RedisAddr:     stringVar(getenv, "REDIS_ADDR", "localhost:6379"),
		RedisPassword: getenv("REDIS_PASSWORD"),
	}

	var err error
	if err = cfg.LogLevel.UnmarshalText([]byte(stringVar(getenv, "LOG_LEVEL", "info"))); err != nil {
		return Config{}, fmt.Errorf("LOG_LEVEL: %w", err)
	}
	if cfg.ShutdownTimeout, err = durationVar(getenv, "SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.StatsTimeout, err = durationVar(getenv, "STATS_TIMEOUT", 200*time.Millisecond); err != nil {
		return Config{}, err
	}
	if cfg.MaxLimit, err = intVar(getenv, "MAX_LIMIT", 10_000, 1); err != nil {
		return Config{}, err
	}
	if cfg.MaxStringLength, err = intVar(getenv, "MAX_STRING_LENGTH", 64, 1); err != nil {
		return Config{}, err
	}
	if cfg.RedisDB, err = intVar(getenv, "REDIS_DB", 0, 0); err != nil {
		return Config{}, err
	}

	if cfg.StatsStore != StatsStoreMemory && cfg.StatsStore != StatsStoreRedis {
		return Config{}, fmt.Errorf("STATS_STORE: must be %q or %q, got %q",
			StatsStoreMemory, StatsStoreRedis, cfg.StatsStore)
	}
	return cfg, nil
}

func stringVar(getenv func(string) string, name, fallback string) string {
	if value := getenv(name); value != "" {
		return value
	}
	return fallback
}

func intVar(getenv func(string) string, name string, fallback, minimum int) (int, error) {
	raw := getenv(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: must be an integer, got %q", name, raw)
	}
	if value < minimum {
		return 0, fmt.Errorf("%s: must be greater than or equal to %d, got %d", name, minimum, value)
	}
	return value, nil
}

func durationVar(getenv func(string) string, name string, fallback time.Duration) (time.Duration, error) {
	raw := getenv(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: must be a duration such as 500ms or 10s, got %q", name, raw)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s: must be positive, got %s", name, value)
	}
	return value, nil
}
