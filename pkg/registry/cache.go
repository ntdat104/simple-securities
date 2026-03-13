package registry

import (
	"math/rand"
	"strings"
	"time"
)

type CacheRegistry struct {
	KeyPrefix string
	Key       string
	MemTTL    time.Duration
	RedisTTL  time.Duration
}

func NewDefaultCache(keyPrefix string) func(args ...string) CacheRegistry {
	return NewCache(keyPrefix, 1*time.Minute, 3*time.Minute)
}

func NewCache(keyPrefix string, memTTL time.Duration, redisTTL time.Duration) func(args ...string) CacheRegistry {
	return func(args ...string) CacheRegistry {
		key := keyPrefix
		if len(args) > 0 {
			key = keyPrefix + ":" + strings.Join(args, ":")
		}
		return CacheRegistry{
			KeyPrefix: keyPrefix,
			Key:       key,
			MemTTL:    memTTL,
			RedisTTL:  redisTTL + time.Duration(rand.Intn(30))*time.Second, // tránh cache avalanche
		}
	}
}
