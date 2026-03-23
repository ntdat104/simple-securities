package idem

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisIdempotencyStore implements service.IdempotencyStore using Redis.
type RedisIdempotencyStore struct {
	client *redis.Client
	prefix string
}

func NewRedisIdempotencyStore(client *redis.Client) *RedisIdempotencyStore {
	return &RedisIdempotencyStore{client: client, prefix: "idem:credit:"}
}

func (s *RedisIdempotencyStore) Get(ctx context.Context, key string) (any, error) {
	raw, err := s.client.Get(ctx, s.prefix+key).Bytes()
	if err != nil {
		return nil, err // redis.Nil or actual error
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *RedisIdempotencyStore) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("idempotency marshal: %w", err)
	}
	return s.client.Set(ctx, s.prefix+key, raw, ttl).Err()
}
