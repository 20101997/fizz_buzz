package config_test

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/20101997/fizz_buzz/internal/config"
)

func env(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

func TestLoadDefaults(t *testing.T) {
	got, err := config.Load(env(nil))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	want := config.Config{
		HTTPAddr:        ":8080",
		LogLevel:        slog.LevelInfo,
		ShutdownTimeout: 10 * time.Second,
		MaxLimit:        10_000,
		MaxStringLength: 64,
		StatsStore:      config.StatsStoreMemory,
		StatsTimeout:    200 * time.Millisecond,
		RedisAddr:       "localhost:6379",
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadOverrides(t *testing.T) {
	got, err := config.Load(env(map[string]string{
		"HTTP_ADDR":         ":9090",
		"LOG_LEVEL":         "debug",
		"SHUTDOWN_TIMEOUT":  "30s",
		"MAX_LIMIT":         "500",
		"MAX_STRING_LENGTH": "16",
		"STATS_STORE":       "redis",
		"STATS_TIMEOUT":     "250ms",
		"REDIS_ADDR":        "redis:6379",
		"REDIS_PASSWORD":    "secret",
		"REDIS_DB":          "2",
	}))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	want := config.Config{
		HTTPAddr:        ":9090",
		LogLevel:        slog.LevelDebug,
		ShutdownTimeout: 30 * time.Second,
		MaxLimit:        500,
		MaxStringLength: 16,
		StatsStore:      config.StatsStoreRedis,
		StatsTimeout:    250 * time.Millisecond,
		RedisAddr:       "redis:6379",
		RedisPassword:   "secret",
		RedisDB:         2,
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadInvalid(t *testing.T) {
	tests := []struct{ name, value string }{
		{"LOG_LEVEL", "loud"},
		{"SHUTDOWN_TIMEOUT", "soon"},
		{"SHUTDOWN_TIMEOUT", "-1s"},
		{"STATS_TIMEOUT", "0s"},
		{"MAX_LIMIT", "many"},
		{"MAX_LIMIT", "0"},
		{"MAX_STRING_LENGTH", "-1"},
		{"REDIS_DB", "-1"},
		{"STATS_STORE", "postgres"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"="+tt.value, func(t *testing.T) {
			_, err := config.Load(env(map[string]string{tt.name: tt.value}))
			if err == nil {
				t.Fatal("Load() error = nil, want an error")
			}
			if !strings.Contains(err.Error(), tt.name) {
				t.Errorf("Load() error = %q, want it to name %s", err, tt.name)
			}
		})
	}
}
