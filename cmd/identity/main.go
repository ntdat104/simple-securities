// Command identity runs the identity service (users, credentials, sessions).
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	identityv1 "simple-securities/gen/identity/v1"
	"simple-securities/internal/identity/app"
	"simple-securities/internal/identity/grpcapi"
	"simple-securities/internal/identity/store"
	pkgapp "simple-securities/pkg/app"
	"simple-securities/pkg/auth"
	"simple-securities/pkg/config"
	"simple-securities/pkg/database"
	"simple-securities/pkg/events"
	"simple-securities/pkg/grpcx"
	"simple-securities/pkg/httpx"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/logger"
	"simple-securities/pkg/outbox"
)

const service = "identity"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.LoadCommon(service, ":9101", ":8101",
		"postgres://exchange:exchange@localhost:5432/identity?sslmode=disable")
	log := logger.New(service, cfg.Env, cfg.LogLevel)
	defer func() { _ = log.Sync() }()

	ctx, cancel := pkgapp.Context()
	defer cancel()

	db, err := database.Connect(ctx, database.Config{
		URL:          cfg.DatabaseURL,
		MaxOpenConns: config.Int("DB_MAX_CONNS", 20),
		LogLevel:     config.String("DB_LOG_LEVEL", "warn"),
	}, log)
	if err != nil {
		return err
	}
	defer database.Close(db)
	if err := database.Migrate(ctx, db, service, append(outbox.Models(), store.Models()...)...); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	replication := int16(config.Int("KAFKA_REPLICATION", 1))
	if err := kafka.EnsureTopics(ctx, cfg.Brokers, events.AllTopics(), replication); err != nil {
		return fmt.Errorf("ensure topics: %w", err)
	}
	prod, err := kafka.NewProducer(cfg.Brokers, service+"-"+cfg.Instance, log)
	if err != nil {
		return fmt.Errorf("kafka producer: %w", err)
	}
	defer prod.Close()
	dispatcher := outbox.NewDispatcher(outbox.Config{Service: service}, db, prod, log)

	jwt := auth.New(config.String("JWT_SECRET", "dev-secret-change-me-32-bytes-long!!"),
		config.Duration("JWT_TTL", 15*time.Minute))
	svc, err := app.NewService(db, jwt, dispatcher, config.Duration("REFRESH_TTL", 720*time.Hour), log)
	if err != nil {
		return err
	}
	api := grpcapi.NewServer(svc)

	grpcSrv := grpcx.NewServer(log)
	identityv1.RegisterIdentityServiceServer(grpcSrv, api)

	mux := httpx.NewGatewayMux()
	if err := identityv1.RegisterIdentityServiceHandlerServer(ctx, mux, api); err != nil {
		return fmt.Errorf("register gateway: %w", err)
	}
	handler := httpx.ServiceHandler(mux, func(ctx context.Context) error { return database.Ping(ctx, db) })

	log.Info("starting", zap.String("grpc", cfg.GRPCAddr), zap.String("http", cfg.HTTPAddr))
	return pkgapp.Run(ctx, log,
		func(ctx context.Context) error { return grpcx.Serve(ctx, grpcSrv, cfg.GRPCAddr, log) },
		func(ctx context.Context) error { return httpx.Serve(ctx, cfg.HTTPAddr, handler, log) },
		dispatcher.Run,
	)
}
