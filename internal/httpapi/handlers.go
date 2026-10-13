package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"unicode/utf8"

	"github.com/20101997/fizz_buzz/internal/fizzbuzz"
	"github.com/20101997/fizz_buzz/internal/stats"
)

// Error codes returned in the body of non-2xx responses.
const (
	codeInvalidParameter = "invalid_parameter"
	codeNoStats          = "no_stats"
	codeStatsUnavailable = "stats_unavailable"
	codeInternal         = "internal_error"
)

// errorResponse is the body of every non-2xx response.
type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (s *server) handleFizzBuzz(w http.ResponseWriter, r *http.Request) {
	params, err := parseParams(r.URL.Query(), s.limits)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidParameter, err.Error())
		return
	}
	result, err := fizzbuzz.Generate(params)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidParameter, err.Error())
		return
	}

	s.record(r.Context(), params)
	writeJSON(w, http.StatusOK, result)
}

// record counts the request in the stats store. Statistics are best effort:
// a failing store is logged and never fails the request being served.
func (s *server) record(ctx context.Context, params fizzbuzz.Params) {
	ctx, cancel := context.WithTimeout(ctx, s.statsTimeout)
	defer cancel()

	if err := s.stats.Record(ctx, params); err != nil {
		s.logger.WarnContext(ctx, "request not counted in stats",
			"error", err, "request_id", requestID(ctx))
	}
}

func (s *server) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.statsTimeout)
	defer cancel()

	entry, err := s.stats.MostFrequent(ctx)
	switch {
	case errors.Is(err, stats.ErrNoData):
		writeError(w, http.StatusNotFound, codeNoStats, "no fizz-buzz request has been served yet")
	case err != nil:
		s.logger.ErrorContext(ctx, "stats unavailable", "error", err, "request_id", requestID(ctx))
		writeError(w, http.StatusServiceUnavailable, codeStatsUnavailable, "statistics are temporarily unavailable")
	default:
		writeJSON(w, http.StatusOK, entry)
	}
}

// handleHealth is a liveness probe. It deliberately ignores the stats store:
// the service keeps serving fizz-buzz requests without it.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// parseParams reads and validates the fizz-buzz parameters of a query string.
// Every parameter is required. It returns a *fizzbuzz.ParamError describing
// the first invalid one.
func parseParams(query url.Values, limits Limits) (fizzbuzz.Params, error) {
	var (
		params fizzbuzz.Params
		err    error
	)
	if params.Int1, err = intParam(query, "int1"); err != nil {
		return fizzbuzz.Params{}, err
	}
	if params.Int2, err = intParam(query, "int2"); err != nil {
		return fizzbuzz.Params{}, err
	}
	if params.Limit, err = intParam(query, "limit"); err != nil {
		return fizzbuzz.Params{}, err
	}
	if params.Str1, err = stringParam(query, "str1", limits.MaxStringLength); err != nil {
		return fizzbuzz.Params{}, err
	}
	if params.Str2, err = stringParam(query, "str2", limits.MaxStringLength); err != nil {
		return fizzbuzz.Params{}, err
	}

	if err := params.Validate(); err != nil {
		return fizzbuzz.Params{}, err
	}
	if params.Limit > limits.MaxLimit {
		return fizzbuzz.Params{}, &fizzbuzz.ParamError{
			Param:  "limit",
			Reason: fmt.Sprintf("must be less than or equal to %d", limits.MaxLimit),
		}
	}
	return params, nil
}

func intParam(query url.Values, name string) (int, error) {
	raw := query.Get(name)
	if raw == "" {
		return 0, &fizzbuzz.ParamError{Param: name, Reason: "is required"}
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, &fizzbuzz.ParamError{Param: name, Reason: "must be an integer"}
	}
	return value, nil
}

func stringParam(query url.Values, name string, maxLength int) (string, error) {
	value := query.Get(name)
	if value == "" {
		return "", &fizzbuzz.ParamError{Param: name, Reason: "is required"}
	}
	if utf8.RuneCountInString(value) > maxLength {
		return "", &fizzbuzz.ParamError{
			Param:  name,
			Reason: fmt.Sprintf("must be at most %d characters long", maxLength),
		}
	}
	return value, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// The status line is already sent, so the only possible failure left is
	// a client that went away: there is nobody to report it to.
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Code: code, Message: message})
}
