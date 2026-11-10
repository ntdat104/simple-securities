// Command gateway is the public edge: REST /api/v1, WebSocket /ws, JWT auth, rate limiting and the
// static trading UI.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"simple-securities/internal/gateway/api"
	"simple-securities/internal/gateway/ratelimit"
	"simple-securities/internal/gateway/rest"
	"simple-securities/internal/gateway/upstream"
	"simple-securities/internal/gateway/ws"
	"simple-securities/pkg/app"
	"simple-securities/pkg/auth"
	"simple-securities/pkg/config"
	"simple-securities/pkg/httpx"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/logger"
	"simple-securities/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gateway:", err)
		os.Exit(1)
	}
}

func run() error {
	common := config.LoadCommon("gateway", "", ":8080", "")
	log := logger.New(common.Service, common.Env, common.LogLevel)
	defer func() { _ = log.Sync() }()

	ctx, cancel := app.Context()
	defer cancel()

	ups, err := upstream.New(upstream.ConfigFromEnv())
	if err != nil {
		return err
	}
	defer func() { _ = ups.Close() }()

	rdb := redis.NewClient(&redis.Options{
		Addr:         config.String("REDIS_ADDR", "localhost:6379"),
		DialTimeout:  time.Second,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
		PoolSize:     100,
	})
	defer func() { _ = rdb.Close() }()

	limiter := ratelimit.New(ratelimit.NewRedisStore(rdb), log)
	jwt := auth.New(config.String("JWT_SECRET", "dev-secret-change-me-32-bytes-long!!"),
		config.Duration("JWT_TTL", 15*time.Minute))
	trader := api.NewTrader(ups.Order, ups.Futures, limiter)
	origins := config.List("CORS_ORIGINS", []string{"*"})

	hub := ws.NewHub(trader, jwt, origins, log)
	tail, err := kafka.NewTailConsumer(common.Brokers, ws.Topics, hub.HandleMessage, log.Named("tail"))
	if err != nil {
		return fmt.Errorf("tail consumer: %w", err)
	}

	srv := rest.NewServer(rest.Config{
		Origins:    origins,
		TrustProxy: config.Bool("TRUST_PROXY", false),
		WebDir:     config.String("WEB_DIR", ""),
		Index:      web.Index,
	}, ups, trader, jwt, limiter, hub, log)
	// Redis is optional (rate limiting fails open), so readiness does not depend on it.
	handler := httpx.ServiceHandler(srv.Handler(), nil)

	log.Info("gateway starting", zap.String("http", common.HTTPAddr),
		zap.String("upstream_transport", upstream.ConfigFromEnv().Transport))
	return app.Run(ctx, log,
		tail.Run,
		hub.Run,
		func(ctx context.Context) error { return httpx.Serve(ctx, common.HTTPAddr, handler, log) },
	)
}
