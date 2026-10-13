package stats

import (
	"context"
	"sync"

	"github.com/20101997/fizz_buzz/internal/fizzbuzz"
)

// MemoryStore keeps hit counts in process memory.
//
// Counts are lost on restart and are not shared between instances, so it is
// meant for local development and tests. Use RedisStore in production.
type MemoryStore struct {
	mu   sync.Mutex
	hits map[fizzbuzz.Params]int64
	top  Entry
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{hits: make(map[fizzbuzz.Params]int64)}
}

// Record counts one hit for p.
func (s *MemoryStore) Record(_ context.Context, p fizzbuzz.Params) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.hits[p]++
	// The leader is tracked on write so that reads stay O(1). On a tie the
	// request that reached the count first is kept.
	if hits := s.hits[p]; hits > s.top.Hits {
		s.top = Entry{Request: p, Hits: hits}
	}
	return nil
}

// MostFrequent returns the request with the most hits, or ErrNoData.
func (s *MemoryStore) MostFrequent(context.Context) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.top.Hits == 0 {
		return Entry{}, ErrNoData
	}
	return s.top, nil
}
