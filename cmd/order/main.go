// Command order runs the order service.
package main

import (
	"context"
	"fmt"
	"os"

	orderv1 "simple-securities/gen/order/v1"
	"simple-securities/internal/order"
	"simple-securities/internal/order/walletclient"
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
	cfg := config.LoadCommon(order.ServiceName, ":9103", ":8103",
		"postgres://exchange:exchange@localhost:5432/orders?sslmode=disable")
	log := logger.New(cfg.Service, cfg.Env, cfg.LogLevel)
	defer func() { _ = log.Sync() }()

	ctx, stop := app.Context()
	defer stop()

	db, err := database.Connect(ctx, database.Config{
		URL:          cfg.DatabaseURL,
		MaxOpenConns: config.Int("DB_MAX_CONNS", 20),
		LogLevel:     config.String("DB_LOG_LEVEL", "warn"),
	}, log)
	if err != nil {
		return err
	}
	defer database.Close(db)
	if err := database.Migrate(ctx, db, order.ServiceName, append(outbox.Models(), order.Models()...)...); err != nil {
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

	wallet, err := walletclient.New()
	if err != nil {
		return err
	}
	defer func() { _ = wallet.Close() }()

	svc := order.NewService(db, wallet, dispatcher, log)
	server := order.NewServer(svc)

	grpcServer := grpcx.NewServer(log)
	orderv1.RegisterOrderServiceServer(grpcServer, server)

	mux := httpx.NewGatewayMux()
	if err := orderv1.RegisterOrderServiceHandlerServer(ctx, mux, server); err != nil {
		return err
	}
	handler := httpx.ServiceHandler(mux, func(ctx context.Context) error { return database.Ping(ctx, db) })

	consumer, err := kafka.NewGroupConsumer(kafka.GroupConsumerConfig{
		Brokers: cfg.Brokers,
		Group:   order.ServiceName,
		Topics:  []string{events.TopicMatchingEvents},
	}, svc.HandleMatchingEvents, log)
	if err != nil {
		return err
	}

	return app.Run(ctx, log,
		dispatcher.Run,
		consumer.Run,
		svc.RunRecovery,
		func(ctx context.Context) error { return grpcx.Serve(ctx, grpcServer, cfg.GRPCAddr, log) },
		func(ctx context.Context) error { return httpx.Serve(ctx, cfg.HTTPAddr, handler, log) },
	)
}
