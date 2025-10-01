package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisConfig holds Redis client configuration options
type RedisConfig struct {
	// Address is the Redis server address
	Address string
	// Password is the Redis server password
	Password string
	// DB is the Redis database index
	DB int
	// PoolSize is the maximum number of socket connections
	PoolSize int
	// MinIdleConns is the minimum number of idle connections
	MinIdleConns int
	// DialTimeout is the timeout for establishing new connections
	DialTimeout time.Duration
	// ReadTimeout is the timeout for socket reads
	ReadTimeout time.Duration
	// WriteTimeout is the timeout for socket writes
	WriteTimeout time.Duration
	// PoolTimeout is the timeout for getting a connection from the pool
	PoolTimeout time.Duration
	// IdleTimeout is the timeout for idle connections
	IdleTimeout time.Duration
	// MaxRetries is the maximum number of retries before giving up
	MaxRetries int
	// MinRetryBackoff is the minimum backoff between retries
	MinRetryBackoff time.Duration
	// MaxRetryBackoff is the maximum backoff between retries
	MaxRetryBackoff time.Duration
}

type RedisClient struct {
	Client *redis.Client
	config *RedisConfig
}

func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		Address:         "localhost:6379",
		Password:        "",
		DB:              0,
		PoolSize:        10,
		MinIdleConns:    5,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		PoolTimeout:     4 * time.Second,
		IdleTimeout:     5 * time.Minute,
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
	}
}

func NewRedisClient(conf *RedisConfig) (*RedisClient, error) {
	if conf == nil {
		conf = DefaultRedisConfig()
	}

	redisConf := &redis.Options{
		Addr:            conf.Address,
		Password:        conf.Password,
		DB:              conf.DB,
		PoolSize:        conf.PoolSize,
		MinIdleConns:    conf.MinIdleConns,
		DialTimeout:     conf.DialTimeout,
		ReadTimeout:     conf.ReadTimeout,
		WriteTimeout:    conf.WriteTimeout,
		PoolTimeout:     conf.PoolTimeout,
		IdleTimeout:     conf.IdleTimeout,
		MaxRetries:      conf.MaxRetries,
		MinRetryBackoff: conf.MinRetryBackoff,
		MaxRetryBackoff: conf.MaxRetryBackoff,
	}

	client := redis.NewClient(redisConf)
	redisClient := &RedisClient{
		Client: client,
		config: conf,
	}

	// Verify connection on creation
	if err := redisClient.HealthCheck(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return redisClient, nil
}

func (c *RedisClient) HealthCheck(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, c.config.DialTimeout)
	defer cancel()

	status := c.Client.Ping(timeoutCtx)
	if status.Err() != nil {
		return fmt.Errorf("redis health check failed: %w", status.Err())
	}
	return nil
}

func (c *RedisClient) Close() error {
	return c.Client.Close()
}

func (c *RedisClient) Stats() *redis.PoolStats {
	return c.Client.PoolStats()
}

func (c *RedisClient) WithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}
