// Command futures runs the USDⓈ-M perpetual futures service.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	futuresv1 "simple-securities/gen/futures/v1"
	"simple-securities/internal/futures"
	"simple-securities/pkg/app"
	"simple-securities/pkg/config"
	"simple-securities/pkg/database"
	"simple-securities/pkg/events"
	"simple-securities/pkg/grpcx"
	"simple-securities/pkg/httpx"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/logger"
	"simple-securities/pkg/outbox"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.LoadCommon(futures.ServiceName, ":9106", ":8106",
		"postgres://exchange:exchange@localhost:5432/futures?sslmode=disable")
	log := logger.New(cfg.Service, cfg.Env, cfg.LogLevel)
	defer func() { _ = log.Sync() }()

	ctx, stop := app.Context()
	defer stop()

	insuranceSeed, err := decimal.NewFromString(config.String("INSURANCE_FUND_SEED", "1000000"))
	if err != nil {
		return fmt.Errorf("INSURANCE_FUND_SEED: %w", err)
	}
	fundingInterval := config.Duration("FUNDING_INTERVAL", 8*time.Hour)
	if fundingInterval < time.Minute {
		return fmt.Errorf("FUNDING_INTERVAL must be at least 1m, got %s", fundingInterval)
	}

	db, err := database.Connect(ctx, database.Config{
		URL:          cfg.DatabaseURL,
		MaxOpenConns: config.Int("DB_MAX_CONNS", 20),
		LogLevel:     config.String("DB_LOG_LEVEL", "warn"),
	}, log)
	if err != nil {
		return err
	}
	defer database.Close(db)
	if err := database.Migrate(ctx, db, futures.ServiceName, append(outbox.Models(), futures.Models()...)...); err != nil {
		return err
	}
	if err := futures.SeedInsuranceFund(ctx, db, insuranceSeed); err != nil {
		return fmt.Errorf("seed insurance fund: %w", err)
	}

	replication := int16(config.Int("KAFKA_REPLICATION", 1))
	if err := kafka.EnsureTopics(ctx, cfg.Brokers, events.AllTopics(), replication); err != nil {
		return fmt.Errorf("ensure topics: %w", err)
	}

	producer, err := kafka.NewProducer(cfg.Brokers, cfg.Service+"-"+cfg.Instance, log)
	if err != nil {
		return err
	}
	defer producer.Close()
	dispatcher := outbox.NewDispatcher(outbox.Config{Service: cfg.Service}, db, producer, log)

	prices := futures.NewPrices()
	svc := futures.NewService(db, dispatcher, producer, prices,
		futures.Config{FundingInterval: fundingInterval}, log)
	server := futures.NewServer(svc)

	grpcServer := grpcx.NewServer(log)
	futuresv1.RegisterFuturesServiceServer(grpcServer, server)

	mux := httpx.NewGatewayMux()
	if err := futuresv1.RegisterFuturesServiceHandlerServer(ctx, mux, server); err != nil {
		return err
	}
	handler := httpx.ServiceHandler(mux, func(ctx context.Context) error { return database.Ping(ctx, db) })

	consumer, err := kafka.NewGroupConsumer(kafka.GroupConsumerConfig{
		Brokers: cfg.Brokers,
		Group:   futures.ServiceName,
		Topics:  futures.ConsumerTopics(),
	}, svc.HandleEvents, log)
	if err != nil {
		return err
	}
	// Every instance tails the price feeds and the mark price topic so the risk checks have
	// prices; only the leader computes/publishes marks, settles funding and liquidates.
	feeds, err := kafka.NewTailConsumer(cfg.Brokers, futures.FeedTopics(), prices.Handle, log)
	if err != nil {
		return err
	}

	log.Info("futures service starting", zap.Duration("funding_interval", fundingInterval))
	return app.Run(ctx, log,
		dispatcher.Run,
		consumer.Run,
		feeds.Run,
		svc.RunLeader,
		func(ctx context.Context) error { return grpcx.Serve(ctx, grpcServer, cfg.GRPCAddr, log) },
		func(ctx context.Context) error { return httpx.Serve(ctx, cfg.HTTPAddr, handler, log) },
	)
}
