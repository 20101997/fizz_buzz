// Package httpapi exposes the fizz-buzz service over HTTP.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/20101997/fizz_buzz/internal/fizzbuzz"
	"github.com/20101997/fizz_buzz/internal/stats"
)

// StatsStore records the requests served and reports the most frequent one.
type StatsStore interface {
	Record(ctx context.Context, p fizzbuzz.Params) error
	// MostFrequent returns stats.ErrNoData when nothing was recorded yet.
	MostFrequent(ctx context.Context) (stats.Entry, error)
}

// Limits bounds what a single request may ask for, so that the cost of a
// request stays predictable whatever the client sends.
type Limits struct {
	// MaxLimit is the highest accepted value for the limit parameter.
	MaxLimit int
	// MaxStringLength is the longest accepted str1 or str2, in characters.
	MaxStringLength int
}

// Config holds the dependencies and settings of the API.
type Config struct {
	Logger *slog.Logger
	Stats  StatsStore
	Limits Limits
	// StatsTimeout bounds every call to the stats store, so that a slow store
	// cannot hold requests up.
	StatsTimeout time.Duration
}

type server struct {
	logger       *slog.Logger
	stats        StatsStore
	limits       Limits
	statsTimeout time.Duration
}

// NewHandler returns the HTTP handler serving the whole API.
func NewHandler(cfg Config) http.Handler {
	s := &server{
		logger:       cfg.Logger,
		stats:        cfg.Stats,
		limits:       cfg.Limits,
		statsTimeout: cfg.StatsTimeout,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /fizzbuzz", s.handleFizzBuzz)
	mux.HandleFunc("GET /stats", s.handleStats)
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /openapi.yaml", handleOpenAPISpec)
	mux.HandleFunc("GET /docs", handleDocs)

	// Outermost first: every log line carries a request ID, and a recovered
	// panic is still reported by the access log as a 500.
	return withRequestID(accessLog(cfg.Logger)(recoverer(cfg.Logger)(mux)))
}
