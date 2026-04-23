package service

import (
	"context"
	"time"
)

// DistributedLock is an abstraction over Redis-based locking.
// Returns an unlock function that must be deferred.
type DistributedLock interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (unlock func(), err error)
}

// IdempotencyStore caches completed operation responses to enable idempotent retries.
type IdempotencyStore interface {
	Get(ctx context.Context, key string) (any, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
}
