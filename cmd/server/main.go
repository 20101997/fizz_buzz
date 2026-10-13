// Command server runs the fizz-buzz HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/20101997/fizz_buzz/internal/config"
	"github.com/20101997/fizz_buzz/internal/httpapi"
	"github.com/20101997/fizz_buzz/internal/stats"
)

// Timeouts protecting the server against slow or stalled clients.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, closeStore := newStatsStore(ctx, cfg, logger)
	defer closeStore()

	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewHandler(httpapi.Config{
			Logger:       logger,
			Stats:        store,
			Limits:       httpapi.Limits{MaxLimit: cfg.MaxLimit, MaxStringLength: cfg.MaxStringLength},
			StatsTimeout: cfg.StatsTimeout,
		}),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.ListenAndServe() }()
	logger.Info("server started", "addr", cfg.HTTPAddr, "stats_store", cfg.StatsStore)

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
		stop() // A second signal kills the process right away.
	}

	logger.Info("shutting down", "timeout", cfg.ShutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	logger.Info("server stopped")
	return nil
}

// newStatsStore returns the stats store selected by the configuration, and a
// function releasing its resources.
func newStatsStore(ctx context.Context, cfg config.Config, logger *slog.Logger) (httpapi.StatsStore, func()) {
	if cfg.StatsStore == config.StatsStoreMemory {
		logger.Warn("statistics are kept in memory: they are lost on restart and not shared between instances")
		return stats.NewMemoryStore(), func() {}
	}

	// The client's own log lines go through the structured logger. They are
	// kept at debug level because every failed call is already reported by
	// its caller.
	redis.SetLogger(redisLogger{logger})

	// Fail fast: a request must not wait on retries while Redis is down, as
	// it is served without its statistics anyway.
	client := redis.NewClient(&redis.Options{
		Addr:          cfg.RedisAddr,
		Password:      cfg.RedisPassword,
		DB:            cfg.RedisDB,
		DialTimeout:   cfg.StatsTimeout,
		ReadTimeout:   cfg.StatsTimeout,
		WriteTimeout:  cfg.StatsTimeout,
		DialerRetries: 1,
		MaxRetries:    -1,
	})

	// Statistics are best effort, so an unreachable Redis is reported but
	// does not prevent the service from starting: the client reconnects on
	// its own once Redis is back.
	pingCtx, cancel := context.WithTimeout(ctx, cfg.StatsTimeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		logger.Warn("redis is unreachable, statistics are unavailable until it comes back",
			"addr", cfg.RedisAddr, "error", err)
	}

	return stats.NewRedisStore(client), func() {
		if err := client.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
	}
}

// redisLogger adapts the structured logger to the interface go-redis logs to.
type redisLogger struct {
	logger *slog.Logger
}

func (l redisLogger) Printf(ctx context.Context, format string, args ...any) {
	l.logger.DebugContext(ctx, "redis: "+fmt.Sprintf(format, args...))
}
