package stats_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/20101997/fizz_buzz/internal/fizzbuzz"
	"github.com/20101997/fizz_buzz/internal/stats"
)

// store is the behaviour both implementations must share.
type store interface {
	Record(ctx context.Context, p fizzbuzz.Params) error
	MostFrequent(ctx context.Context) (stats.Entry, error)
}

var (
	classic = fizzbuzz.Params{Int1: 3, Int2: 5, Limit: 100, Str1: "fizz", Str2: "buzz"}
	other   = fizzbuzz.Params{Int1: 2, Int2: 7, Limit: 50, Str1: "foo", Str2: "bar"}
)

func newRedisStore(t *testing.T) (*stats.RedisStore, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	return stats.NewRedisStore(client), server
}

func TestStores(t *testing.T) {
	implementations := map[string]func(t *testing.T) store{
		"memory": func(*testing.T) store { return stats.NewMemoryStore() },
		"redis": func(t *testing.T) store {
			s, _ := newRedisStore(t)
			return s
		},
	}

	for name, newStore := range implementations {
		t.Run(name, func(t *testing.T) {
			t.Run("no data", func(t *testing.T) {
				_, err := newStore(t).MostFrequent(context.Background())
				if !errors.Is(err, stats.ErrNoData) {
					t.Errorf("MostFrequent() error = %v, want ErrNoData", err)
				}
			})

			t.Run("returns the most frequent request", func(t *testing.T) {
				s := newStore(t)
				record(t, s, other, 2)
				record(t, s, classic, 3)

				assertMostFrequent(t, s, stats.Entry{Request: classic, Hits: 3})

				record(t, s, other, 2)

				assertMostFrequent(t, s, stats.Entry{Request: other, Hits: 4})
			})

			t.Run("distinguishes requests that only differ by one parameter", func(t *testing.T) {
				s := newStore(t)
				swapped := classic
				swapped.Str1, swapped.Str2 = classic.Str2, classic.Str1
				record(t, s, classic, 2)
				record(t, s, swapped, 1)

				assertMostFrequent(t, s, stats.Entry{Request: classic, Hits: 2})
			})

			t.Run("keeps words with special characters intact", func(t *testing.T) {
				s := newStore(t)
				special := fizzbuzz.Params{Int1: 3, Int2: 5, Limit: 10, Str1: `a,"b":c`, Str2: "é\n世界"}
				record(t, s, special, 1)

				assertMostFrequent(t, s, stats.Entry{Request: special, Hits: 1})
			})

			t.Run("concurrent records are all counted", func(t *testing.T) {
				s := newStore(t)
				const goroutines, perGoroutine = 20, 25

				var wg sync.WaitGroup
				for range goroutines {
					wg.Go(func() {
						for range perGoroutine {
							if err := s.Record(context.Background(), classic); err != nil {
								t.Errorf("Record() error: %v", err)
							}
						}
					})
				}
				wg.Wait()

				assertMostFrequent(t, s, stats.Entry{Request: classic, Hits: goroutines * perGoroutine})
			})
		})
	}
}

func TestRedisStoreUnavailable(t *testing.T) {
	s, server := newRedisStore(t)
	server.Close()

	if err := s.Record(context.Background(), classic); err == nil {
		t.Error("Record() error = nil, want an error when Redis is down")
	}
	_, err := s.MostFrequent(context.Background())
	if err == nil || errors.Is(err, stats.ErrNoData) {
		t.Errorf("MostFrequent() error = %v, want a connection error when Redis is down", err)
	}
}

func TestRedisStoreCorruptedMember(t *testing.T) {
	s, server := newRedisStore(t)
	if _, err := server.ZAdd("fizzbuzz:hits", 1, "not json"); err != nil {
		t.Fatalf("seed Redis: %v", err)
	}

	_, err := s.MostFrequent(context.Background())
	if err == nil || errors.Is(err, stats.ErrNoData) {
		t.Errorf("MostFrequent() error = %v, want a decoding error", err)
	}
}

func record(t *testing.T, s store, p fizzbuzz.Params, times int) {
	t.Helper()
	for range times {
		if err := s.Record(context.Background(), p); err != nil {
			t.Fatalf("Record() error: %v", err)
		}
	}
}

func assertMostFrequent(t *testing.T, s store, want stats.Entry) {
	t.Helper()
	got, err := s.MostFrequent(context.Background())
	if err != nil {
		t.Fatalf("MostFrequent() error: %v", err)
	}
	if got != want {
		t.Errorf("MostFrequent() = %+v, want %+v", got, want)
	}
}
