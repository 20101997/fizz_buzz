package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/20101997/fizz_buzz/internal/fizzbuzz"
	"github.com/20101997/fizz_buzz/internal/httpapi"
	"github.com/20101997/fizz_buzz/internal/stats"
)

const classicQuery = "int1=3&int2=5&limit=15&str1=fizz&str2=buzz"

var classicParams = fizzbuzz.Params{Int1: 3, Int2: 5, Limit: 15, Str1: "fizz", Str2: "buzz"}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// failingStore is a stats store that is down.
type failingStore struct{}

func (failingStore) Record(context.Context, fizzbuzz.Params) error {
	return errors.New("store down")
}

func (failingStore) MostFrequent(context.Context) (stats.Entry, error) {
	return stats.Entry{}, errors.New("store down")
}

// panickingStore is a stats store with a bug.
type panickingStore struct{ failingStore }

func (panickingStore) Record(context.Context, fizzbuzz.Params) error {
	panic("boom")
}

func newHandler(store httpapi.StatsStore) http.Handler {
	return httpapi.NewHandler(httpapi.Config{
		Logger:       slog.New(slog.DiscardHandler),
		Stats:        store,
		Limits:       httpapi.Limits{MaxLimit: 100, MaxStringLength: 8},
		StatsTimeout: time.Second,
	})
}

func newRequest(t *testing.T, method, target string) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(t.Context(), method, target, nil)
}

func get(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, newRequest(t, http.MethodGet, target))
	return response
}

func decode[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want JSON", got)
	}
	var body T
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", response.Body, err)
	}
	return body
}

func assertStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d (body: %s)", response.Code, want, response.Body)
	}
}

func TestFizzBuzz(t *testing.T) {
	response := get(t, newHandler(stats.NewMemoryStore()), "/fizzbuzz?"+classicQuery)

	assertStatus(t, response, http.StatusOK)
	want := strings.Split("1,2,fizz,4,buzz,fizz,7,8,fizz,buzz,11,fizz,13,14,fizzbuzz", ",")
	if got := decode[[]string](t, response); !slices.Equal(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestFizzBuzzAcceptsValuesAtTheLimits(t *testing.T) {
	query := url.Values{
		"int1": {"1"}, "int2": {"1"}, "limit": {"100"},
		"str1": {"éééééééé"}, "str2": {"12345678"},
	}

	response := get(t, newHandler(stats.NewMemoryStore()), "/fizzbuzz?"+query.Encode())

	assertStatus(t, response, http.StatusOK)
	if got := decode[[]string](t, response); len(got) != 100 || got[0] != "éééééééé12345678" {
		t.Errorf("body = %v, want 100 times the two words", got)
	}
}

func TestFizzBuzzInvalidParameters(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		wantMessage string
	}{
		{"missing int1", "int2=5&limit=15&str1=fizz&str2=buzz", "int1 is required"},
		{"missing int2", "int1=3&limit=15&str1=fizz&str2=buzz", "int2 is required"},
		{"missing limit", "int1=3&int2=5&str1=fizz&str2=buzz", "limit is required"},
		{"missing str1", "int1=3&int2=5&limit=15&str2=buzz", "str1 is required"},
		{"missing str2", "int1=3&int2=5&limit=15&str1=fizz", "str2 is required"},
		{"empty str1", "int1=3&int2=5&limit=15&str1=&str2=buzz", "str1 is required"},
		{"non-numeric int1", "int1=abc&int2=5&limit=15&str1=fizz&str2=buzz", "int1 must be an integer"},
		{"decimal int2", "int1=3&int2=5.5&limit=15&str1=fizz&str2=buzz", "int2 must be an integer"},
		{"overflowing limit", "int1=3&int2=5&limit=99999999999999999999&str1=fizz&str2=buzz", "limit must be an integer"},
		{"zero int1", "int1=0&int2=5&limit=15&str1=fizz&str2=buzz", "int1 must be greater than or equal to 1"},
		{"negative int2", "int1=3&int2=-5&limit=15&str1=fizz&str2=buzz", "int2 must be greater than or equal to 1"},
		{"zero limit", "int1=3&int2=5&limit=0&str1=fizz&str2=buzz", "limit must be greater than or equal to 1"},
		{"limit above the maximum", "int1=3&int2=5&limit=101&str1=fizz&str2=buzz", "limit must be less than or equal to 100"},
		{"str1 too long", "int1=3&int2=5&limit=15&str1=123456789&str2=buzz", "str1 must be at most 8 characters long"},
		{"str2 too long", "int1=3&int2=5&limit=15&str1=fizz&str2=123456789", "str2 must be at most 8 characters long"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newHandler(stats.NewMemoryStore())

			response := get(t, handler, "/fizzbuzz?"+tt.query)

			assertStatus(t, response, http.StatusBadRequest)
			want := errorBody{Code: "invalid_parameter", Message: tt.wantMessage}
			if got := decode[errorBody](t, response); got != want {
				t.Errorf("body = %+v, want %+v", got, want)
			}
			// Invalid requests must not show up in the statistics.
			assertStatus(t, get(t, handler, "/stats"), http.StatusNotFound)
		})
	}
}

func TestStats(t *testing.T) {
	handler := newHandler(stats.NewMemoryStore())

	response := get(t, handler, "/stats")
	assertStatus(t, response, http.StatusNotFound)
	if got := decode[errorBody](t, response); got.Code != "no_stats" {
		t.Errorf("code = %q, want no_stats", got.Code)
	}

	for range 3 {
		assertStatus(t, get(t, handler, "/fizzbuzz?"+classicQuery), http.StatusOK)
	}
	for range 2 {
		assertStatus(t, get(t, handler, "/fizzbuzz?int1=2&int2=7&limit=10&str1=foo&str2=bar"), http.StatusOK)
	}

	response = get(t, handler, "/stats")
	assertStatus(t, response, http.StatusOK)
	want := stats.Entry{Request: classicParams, Hits: 3}
	if got := decode[stats.Entry](t, response); got != want {
		t.Errorf("body = %+v, want %+v", got, want)
	}
	if !strings.Contains(response.Body.String(), `"request":{"int1":3,"int2":5,"limit":15,"str1":"fizz","str2":"buzz"},"hits":3`) {
		t.Errorf("unexpected JSON shape: %s", response.Body)
	}
}

func TestStatsStoreDown(t *testing.T) {
	handler := newHandler(failingStore{})

	// The main endpoint keeps working without its statistics.
	assertStatus(t, get(t, handler, "/fizzbuzz?"+classicQuery), http.StatusOK)

	response := get(t, handler, "/stats")
	assertStatus(t, response, http.StatusServiceUnavailable)
	if got := decode[errorBody](t, response); got.Code != "stats_unavailable" {
		t.Errorf("code = %q, want stats_unavailable", got.Code)
	}
}

func TestPanicIsRecovered(t *testing.T) {
	response := get(t, newHandler(panickingStore{}), "/fizzbuzz?"+classicQuery)

	assertStatus(t, response, http.StatusInternalServerError)
	if got := decode[errorBody](t, response); got.Code != "internal_error" {
		t.Errorf("code = %q, want internal_error", got.Code)
	}
}

func TestHealth(t *testing.T) {
	// Liveness must not depend on the stats store.
	response := get(t, newHandler(failingStore{}), "/healthz")

	assertStatus(t, response, http.StatusOK)
	if got := decode[map[string]string](t, response); got["status"] != "ok" {
		t.Errorf("body = %v, want status ok", got)
	}
}

func TestRequestID(t *testing.T) {
	handler := newHandler(stats.NewMemoryStore())

	t.Run("generated when absent", func(t *testing.T) {
		response := get(t, handler, "/healthz")
		if got := response.Header().Get("X-Request-ID"); got == "" {
			t.Error("X-Request-ID header is missing")
		}
	})

	t.Run("reused when provided", func(t *testing.T) {
		request := newRequest(t, http.MethodGet, "/healthz")
		request.Header.Set("X-Request-ID", "abc-123")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if got := response.Header().Get("X-Request-ID"); got != "abc-123" {
			t.Errorf("X-Request-ID = %q, want abc-123", got)
		}
	})

	t.Run("replaced when too long", func(t *testing.T) {
		tooLong := strings.Repeat("x", 65)
		request := newRequest(t, http.MethodGet, "/healthz")
		request.Header.Set("X-Request-ID", tooLong)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if got := response.Header().Get("X-Request-ID"); got == "" || got == tooLong {
			t.Errorf("X-Request-ID = %q, want a generated ID", got)
		}
	})
}

func TestDocs(t *testing.T) {
	handler := newHandler(stats.NewMemoryStore())

	spec := get(t, handler, "/openapi.yaml")
	assertStatus(t, spec, http.StatusOK)
	if !strings.HasPrefix(spec.Body.String(), "openapi: 3.") {
		t.Errorf("unexpected OpenAPI document: %.40s", spec.Body)
	}

	page := get(t, handler, "/docs")
	assertStatus(t, page, http.StatusOK)
	if !strings.Contains(page.Body.String(), "/openapi.yaml") {
		t.Error("the docs page does not load the OpenAPI document")
	}
}

func TestRouting(t *testing.T) {
	handler := newHandler(stats.NewMemoryStore())

	t.Run("unknown path", func(t *testing.T) {
		assertStatus(t, get(t, handler, "/unknown"), http.StatusNotFound)
	})

	t.Run("unsupported method", func(t *testing.T) {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, newRequest(t, http.MethodPost, "/fizzbuzz?"+classicQuery))
		assertStatus(t, response, http.StatusMethodNotAllowed)
	})
}
