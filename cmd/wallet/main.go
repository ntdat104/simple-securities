// Command wallet runs the wallet service (balances, order locks, ledger, settlement).
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	walletv1 "simple-securities/gen/wallet/v1"
	"simple-securities/internal/wallet/app"
	"simple-securities/internal/wallet/consumer"
	"simple-securities/internal/wallet/grpcapi"
	"simple-securities/internal/wallet/store"
	pkgapp "simple-securities/pkg/app"
	"simple-securities/pkg/config"
	"simple-securities/pkg/database"
	"simple-securities/pkg/events"
	"simple-securities/pkg/grpcx"
	"simple-securities/pkg/httpx"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/logger"
	"simple-securities/pkg/outbox"
)

const service = "wallet"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.LoadCommon(service, ":9102", ":8102",
		"postgres://exchange:exchange@localhost:5432/wallet?sslmode=disable")
	log := logger.New(service, cfg.Env, cfg.LogLevel)
	defer func() { _ = log.Sync() }()

	airdrop, err := decimal.NewFromString(config.String("WALLET_AIRDROP_USDT", "10000"))
	if err != nil || airdrop.IsNegative() {
		return fmt.Errorf("invalid WALLET_AIRDROP_USDT")
	}
	maxDeposit, err := decimal.NewFromString(config.String("WALLET_MAX_DEPOSIT", "1000000"))
	if err != nil || !maxDeposit.IsPositive() {
		return fmt.Errorf("invalid WALLET_MAX_DEPOSIT")
	}
	walletCfg := app.Config{
		FaucetEnabled: config.Bool("WALLET_FAUCET_ENABLED", false),
		MaxDeposit:    maxDeposit,
		AirdropUSDT:   airdrop,
	}

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
	if err := store.MigrateConstraints(ctx, db); err != nil {
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

	svc := app.NewService(db, dispatcher, walletCfg, log)
	cons, err := consumer.New(cfg.Brokers, consumer.NewHandler(db, svc, log), log)
	if err != nil {
		return fmt.Errorf("kafka consumer: %w", err)
	}
	api := grpcapi.NewServer(svc)

	grpcSrv := grpcx.NewServer(log)
	walletv1.RegisterWalletServiceServer(grpcSrv, api)

	mux := httpx.NewGatewayMux()
	if err := walletv1.RegisterWalletServiceHandlerServer(ctx, mux, api); err != nil {
		return fmt.Errorf("register gateway: %w", err)
	}
	handler := httpx.ServiceHandler(mux, func(ctx context.Context) error { return database.Ping(ctx, db) })

	log.Info("starting", zap.String("grpc", cfg.GRPCAddr), zap.String("http", cfg.HTTPAddr),
		zap.Bool("faucet", walletCfg.FaucetEnabled), zap.String("airdrop_usdt", airdrop.String()))
	return pkgapp.Run(ctx, log,
		func(ctx context.Context) error { return grpcx.Serve(ctx, grpcSrv, cfg.GRPCAddr, log) },
		func(ctx context.Context) error { return httpx.Serve(ctx, cfg.HTTPAddr, handler, log) },
		dispatcher.Run,
		cons.Run,
	)
}
