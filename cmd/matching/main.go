// Command matching runs the matching engine service.
package main

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"

	"simple-securities/internal/matching"
	"simple-securities/pkg/app"
	"simple-securities/pkg/config"
	"simple-securities/pkg/database"
	"simple-securities/pkg/events"
	"simple-securities/pkg/httpx"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/logger"
	"simple-securities/pkg/outbox"
	"simple-securities/pkg/symbols"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	market := config.String("MARKET", symbols.MarketSpot)
	if market != symbols.MarketSpot && market != symbols.MarketFutures {
		return fmt.Errorf("invalid MARKET %q (spot | futures)", market)
	}
	name := matching.ServiceNameFor(market)
	defHTTP, defDB := ":8104", "postgres://exchange:exchange@localhost:5432/matching?sslmode=disable"
	if market == symbols.MarketFutures {
		defHTTP, defDB = ":8107", "postgres://exchange:exchange@localhost:5432/futures_matching?sslmode=disable"
	}
	cfg := config.LoadCommon(name, "", defHTTP, defDB)
	log := logger.New(cfg.Service, cfg.Env, cfg.LogLevel)
	defer func() { _ = log.Sync() }()

	ctx, stop := app.Context()
	defer stop()

	db, err := database.Connect(ctx, database.Config{
		URL:          cfg.DatabaseURL,
		MaxOpenConns: config.Int("DB_MAX_CONNS", 10),
		LogLevel:     config.String("DB_LOG_LEVEL", "warn"),
	}, log)
	if err != nil {
		return err
	}
	defer database.Close(db)
	if err := database.Migrate(ctx, db, name, append(outbox.Models(), matching.Models()...)...); err != nil {
		return err
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

	svc := matching.NewService(matching.Config{
		Market:            market,
		CommandPartitions: config.Int("ORDER_COMMANDS_PARTITIONS", int(events.Partitions)),
	}, db, dispatcher, producer, log.Named("engine"))

	consumer, err := kafka.NewGroupConsumer(kafka.GroupConsumerConfig{
		Brokers:    cfg.Brokers,
		Group:      name,
		Topics:     []string{events.TopicsFor(market).OrderCommands},
		OnAssigned: svc.OnAssigned,
		OnRevoked:  svc.OnRevoked,
	}, svc.HandleBatch, log)
	if err != nil {
		return err
	}

	handler := httpx.ServiceHandler(nil, func(ctx context.Context) error { return database.Ping(ctx, db) })
	log.Info("matching starting", zap.String("market", market), zap.String("http", cfg.HTTPAddr))
	return app.Run(ctx, log,
		dispatcher.Run,
		consumer.Run,
		svc.RunDepthPublisher,
		func(ctx context.Context) error { return httpx.Serve(ctx, cfg.HTTPAddr, handler, log) },
	)
}
