// Package stats counts fizz-buzz requests and reports the most frequent one.
package stats

import (
	"errors"

	"github.com/20101997/fizz_buzz/internal/fizzbuzz"
)

// ErrNoData is returned when no request has been recorded yet.
var ErrNoData = errors.New("stats: no request recorded yet")

// Entry is a request together with the number of times it was served.
type Entry struct {
	Request fizzbuzz.Params `json:"request"`
	Hits    int64           `json:"hits"`
}
