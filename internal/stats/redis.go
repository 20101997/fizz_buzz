package stats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/20101997/fizz_buzz/internal/fizzbuzz"
)

// hitsKey is the sorted set holding one member per distinct request, scored
// by its number of hits.
const hitsKey = "fizzbuzz:hits"

// RedisStore keeps hit counts in a Redis sorted set, which makes them durable
// and consistent across every instance of the service.
type RedisStore struct {
	client redis.Cmdable
}

// NewRedisStore returns a RedisStore using client. The caller keeps ownership
// of the client and is responsible for closing it.
func NewRedisStore(client redis.Cmdable) *RedisStore {
	return &RedisStore{client: client}
}

// Record counts one hit for p. ZINCRBY is atomic, so concurrent calls from
// any number of instances never lose an update.
func (s *RedisStore) Record(ctx context.Context, p fizzbuzz.Params) error {
	// JSON keeps the member unambiguous whatever characters the words hold,
	// and struct fields are always encoded in the same order.
	member, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("stats: encode request: %w", err)
	}
	if err := s.client.ZIncrBy(ctx, hitsKey, 1, string(member)).Err(); err != nil {
		return fmt.Errorf("stats: record request: %w", err)
	}
	return nil
}

// MostFrequent returns the request with the most hits, or ErrNoData. On a tie
// Redis orders members lexicographically and the last one is returned.
func (s *RedisStore) MostFrequent(ctx context.Context) (Entry, error) {
	top, err := s.client.ZRevRangeWithScores(ctx, hitsKey, 0, 0).Result()
	if err != nil {
		return Entry{}, fmt.Errorf("stats: read most frequent request: %w", err)
	}
	if len(top) == 0 {
		return Entry{}, ErrNoData
	}

	member, ok := top[0].Member.(string)
	if !ok {
		return Entry{}, fmt.Errorf("stats: unexpected member type %T", top[0].Member)
	}
	var request fizzbuzz.Params
	if err := json.Unmarshal([]byte(member), &request); err != nil {
		return Entry{}, fmt.Errorf("stats: decode request: %w", err)
	}
	return Entry{Request: request, Hits: int64(top[0].Score)}, nil
}
