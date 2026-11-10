// Package ratelimit implements fixed-window rate limiting backed by Redis. It fails open: when the
// store is unavailable requests are allowed (availability over strictness) and the outage is logged
// at most once per logEvery.
package ratelimit

import (
	"simple-securities/pkg/config"

	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	storeTimeout = 200 * time.Millisecond
	logEvery     = 10 * time.Second
	// breakerOpen is how long the store is bypassed after an error so an outage does not add
	// storeTimeout of latency to every request.
	breakerOpen = time.Second
)

// Store counts hits in a fixed window.
type Store interface {
	// Incr increments key and returns the count in the current window and the time until it resets.
	Incr(ctx context.Context, key string, window time.Duration) (count int64, ttl time.Duration, err error)
}

// Rule is a limit per window.
type Rule struct {
	Name   string
	Limit  int64
	Window time.Duration
}

// Standard rules of the public API.
var (
	PerIP    = Rule{Name: "ip", Limit: int64(config.Int("RATE_LIMIT_IP_PER_MIN", 1200)), Window: time.Minute}
	PerOrder = Rule{Name: "order", Limit: int64(config.Int("RATE_LIMIT_ORDERS_PER_10S", 50)), Window: 10 * time.Second}
)

// Decision is the result of a check.
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

// Limiter applies rules on a Store.
type Limiter struct {
	store     Store
	log       *zap.Logger
	lastLog   atomic.Int64 // unix nanos of the last fail-open log
	openUntil atomic.Int64 // unix nanos until which the store is bypassed
	now       func() time.Time
}

// New returns a limiter.
func New(store Store, log *zap.Logger) *Limiter {
	return &Limiter{store: store, log: log, now: time.Now}
}

// Allow counts one hit of subject against rule.
func (l *Limiter) Allow(ctx context.Context, rule Rule, subject string) Decision {
	if l.now().UnixNano() < l.openUntil.Load() {
		return Decision{Allowed: true}
	}
	ctx, cancel := context.WithTimeout(ctx, storeTimeout)
	defer cancel()
	n, ttl, err := l.store.Incr(ctx, "rl:"+rule.Name+":"+subject, rule.Window)
	if err != nil {
		if !errors.Is(err, context.Canceled) { // the caller going away says nothing about the store
			l.openUntil.Store(l.now().Add(breakerOpen).UnixNano())
		}
		l.logFailOpen(err)
		return Decision{Allowed: true}
	}
	if n <= rule.Limit {
		return Decision{Allowed: true}
	}
	if ttl <= 0 || ttl > rule.Window {
		ttl = rule.Window
	}
	return Decision{RetryAfter: ttl}
}

func (l *Limiter) logFailOpen(err error) {
	now := l.now().UnixNano()
	last := l.lastLog.Load()
	if now-last < int64(logEvery) || !l.lastLog.CompareAndSwap(last, now) {
		return
	}
	l.log.Warn("rate limiter store unavailable, failing open", zap.Error(err))
}

// fixedWindow increments the counter and starts the window on the first hit.
// The PTTL guard heals keys that lost their expiry.
var fixedWindow = redis.NewScript(`
local c = redis.call('INCR', KEYS[1])
if c == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
local ttl = redis.call('PTTL', KEYS[1])
if ttl < 0 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
  ttl = tonumber(ARGV[1])
end
return {c, ttl}
`)

// RedisStore is a Store on Redis.
type RedisStore struct {
	rdb redis.Scripter
}

var _ Store = (*RedisStore)(nil)

// NewRedisStore returns a Redis-backed store.
func NewRedisStore(rdb redis.Scripter) *RedisStore { return &RedisStore{rdb: rdb} }

// Incr implements Store.
func (s *RedisStore) Incr(ctx context.Context, key string, window time.Duration) (int64, time.Duration, error) {
	res, err := fixedWindow.Run(ctx, s.rdb, []string{key}, window.Milliseconds()).Int64Slice()
	if err != nil {
		return 0, 0, err
	}
	if len(res) != 2 {
		return 0, 0, redis.Nil
	}
	return res[0], time.Duration(res[1]) * time.Millisecond, nil
}
