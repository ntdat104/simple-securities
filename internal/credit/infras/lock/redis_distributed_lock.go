package lock

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"simple-securities/pkg/uuid"
)

// RedisDistributedLock implements service.DistributedLock using Redis SET NX PX.
type RedisDistributedLock struct {
	client *redis.Client
}

func NewRedisDistributedLock(client *redis.Client) *RedisDistributedLock {
	return &RedisDistributedLock{client: client}
}

// Acquire tries to obtain the lock. Returns unlock func or error if lock is held.
// Uses SET key value NX PX ttl — atomic, no Lua script needed for simple cases.
func (l *RedisDistributedLock) Acquire(ctx context.Context, key string, ttl time.Duration) (func(), error) {
	token := uuid.NewGoogleUUID() // unique owner token to prevent foreign release

	ok, err := l.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("redis lock acquire: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("resource locked: %s", key)
	}

	unlock := func() {
		// Only delete if we still own the lock (compare-and-delete via Lua).
		script := redis.NewScript(`
			if redis.call("GET", KEYS[1]) == ARGV[1] then
				return redis.call("DEL", KEYS[1])
			else
				return 0
			end
		`)
		script.Run(ctx, l.client, []string{key}, token)
	}

	return unlock, nil
}
