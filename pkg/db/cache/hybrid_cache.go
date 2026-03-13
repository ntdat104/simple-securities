package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"simple-securities/common/constants"
	"simple-securities/pkg/logger"
	"simple-securities/pkg/registry"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

type HybridCache struct {
	mem   *ristretto.Cache[string, any]
	redis *redis.Client
	sf    singleflight.Group
}

func NewHybridCache(mem *ristretto.Cache[string, any], redis *redis.Client) *HybridCache {
	return &HybridCache{mem: mem, redis: redis}
}

// getVersion returns the current version for a prefix (default 1)
func (c *HybridCache) getVersion(ctx context.Context, prefix string) (int64, error) {
	versionKey := "cache:version:" + prefix
	v, err := c.redis.Get(ctx, versionKey).Int64()
	if err == redis.Nil {
		if err := c.redis.Set(ctx, versionKey, 1, 0).Err(); err != nil {
			return 0, err
		}
		return 1, nil
	}
	return v, err
}

// buildKey attaches the version to the logical key
func (c *HybridCache) buildKey(ctx context.Context, registry registry.CacheRegistry) (string, error) {
	version, err := c.getVersion(ctx, registry.KeyPrefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:v%d", registry.Key, version), nil
}

func Get[T any](
	c *HybridCache,
	ctx context.Context,
	cacheRegistry registry.CacheRegistry,
	fetcher func() (T, error),
) (T, error) {

	key, err := c.buildKey(ctx, cacheRegistry)
	if err != nil {
		var zero T
		return zero, err
	}

	// L1 cache
	if val, ok := c.mem.Get(key); ok {
		if v, ok := val.(T); ok {
			logger.Warn(ctx, "Memory cache hit", zap.String(constants.Key, key))
			return v, nil
		}
	}

	// L2 Redis
	val, err := c.redis.Get(ctx, key).Result()
	if err == nil {
		var data T
		if err := json.Unmarshal([]byte(val), &data); err == nil {
			c.mem.SetWithTTL(key, data, 1, cacheRegistry.MemTTL)
			logger.Warn(ctx, "Redis cache hit", zap.String(constants.Key, key))
			return data, nil
		}
	}

	// singleflight
	v, err, _ := c.sf.Do(key, func() (any, error) {
		if val, ok := c.mem.Get(key); ok {
			return val, nil
		}

		logger.Warn(ctx, "Cache miss", zap.String(constants.Key, key))
		data, err := fetcher()
		if err != nil {
			return nil, err
		}
		if err := Set(c, ctx, cacheRegistry, data); err != nil {
			return nil, err
		}
		return data, nil
	})

	if err != nil {
		var zero T
		return zero, err
	}

	return v.(T), nil
}

func Set[T any](c *HybridCache, ctx context.Context, cacheRegistry registry.CacheRegistry, value T) error {
	key, err := c.buildKey(ctx, cacheRegistry)
	if err != nil {
		return err
	}
	c.mem.SetWithTTL(key, value, 1, cacheRegistry.MemTTL)
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.redis.Set(ctx, key, data, cacheRegistry.RedisTTL).Err()
}

func Delete(c *HybridCache, ctx context.Context, cacheRegistry registry.CacheRegistry) error {
	// Build versioned key internally
	key, err := c.buildKey(ctx, cacheRegistry)
	if err != nil {
		return err
	}

	// Delete only this specific key
	c.mem.Del(key)
	c.sf.Forget(key)
	return c.redis.Del(ctx, key).Err()
}

func DeleteRegistry(c *HybridCache, ctx context.Context, cacheRegistry registry.CacheRegistry) error {
	// Increment version key
	versionKey := "cache:version:" + cacheRegistry.KeyPrefix
	if err := c.redis.Incr(ctx, versionKey).Err(); err != nil {
		return err
	}

	// Optionally remove old L1 cache entries for this prefix
	// (you can keep it minimal if L1 entries expire quickly)
	c.mem.Del(cacheRegistry.Key)
	c.sf.Forget(cacheRegistry.Key)

	return nil
}
