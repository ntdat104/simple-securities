// Command marketdata serves public market data: public trades, klines, 24h tickers and depth.
package main

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"

	marketdatav1 "simple-securities/gen/marketdata/v1"
	"simple-securities/internal/marketdata"
	"simple-securities/pkg/app"
	"simple-securities/pkg/config"
	"simple-securities/pkg/database"
	"simple-securities/pkg/events"
	"simple-securities/pkg/grpcx"
	"simple-securities/pkg/httpx"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/logger"
	"simple-securities/pkg/outbox"
	"simple-securities/pkg/symbols"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	market := config.String("MARKET", symbols.MarketSpot)
	if market != symbols.MarketSpot && market != symbols.MarketFutures {
		fmt.Fprintf(os.Stderr, "invalid MARKET %q (spot | futures)\n", market)
		return fmt.Errorf("invalid MARKET %q", market)
	}
	name := marketdata.ServiceNameFor(market)
	defGRPC, defHTTP := ":9105", ":8105"
	defDB := "postgres://exchange:exchange@localhost:5432/marketdata?sslmode=disable"
	if market == symbols.MarketFutures {
		defGRPC, defHTTP = ":9108", ":8108"
		defDB = "postgres://exchange:exchange@localhost:5432/futures_marketdata?sslmode=disable"
	}
	cfg := config.LoadCommon(name, defGRPC, defHTTP, defDB)
	topics := events.TopicsFor(market)
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
		log.Error("connect postgres", zap.Error(err))
		return err
	}
	defer database.Close(db)
	models := append(outbox.Models(), &marketdata.PublicTrade{}, &marketdata.Kline{})
	if err := database.Migrate(ctx, db, name, models...); err != nil {
		log.Error("migrate", zap.Error(err))
		return err
	}

	replication := int16(config.Int("KAFKA_REPLICATION", 1))
	if err := kafka.EnsureTopics(ctx, cfg.Brokers, events.AllTopics(), replication); err != nil {
		log.Error("ensure topics", zap.Error(err))
		return err
	}
	prod, err := kafka.NewProducer(cfg.Brokers, cfg.Service+"-"+cfg.Instance, log)
	if err != nil {
		log.Error("kafka producer", zap.Error(err))
		return err
	}
	defer prod.Close()

	store := marketdata.NewStore(db, name)
	cache := marketdata.NewCache(log.Named("cache"))
	proc := marketdata.NewProcessor(market, store, prod, cache, log.Named("processor"))

	consumer, err := kafka.NewGroupConsumer(kafka.GroupConsumerConfig{
		Brokers: cfg.Brokers,
		Group:   name,
		Topics:  []string{topics.MatchingEvents},
	}, proc.HandleBatch, log)
	if err != nil {
		log.Error("group consumer", zap.Error(err))
		return err
	}
	tail, err := kafka.NewTailConsumer(cfg.Brokers,
		[]string{topics.Depth, topics.Ticker}, cache.HandleTail, log)
	if err != nil {
		log.Error("tail consumer", zap.Error(err))
		return err
	}

	srv := marketdata.NewServer(market, store, cache)
	grpcSrv := grpcx.NewServer(log)
	marketdatav1.RegisterMarketDataServiceServer(grpcSrv, srv)
	mux := httpx.NewGatewayMux()
	if err := marketdatav1.RegisterMarketDataServiceHandlerServer(ctx, mux, srv); err != nil {
		log.Error("register gateway", zap.Error(err))
		return err
	}
	handler := httpx.ServiceHandler(mux, func(ctx context.Context) error { return database.Ping(ctx, db) })

	log.Info("marketdata starting", zap.String("market", market), zap.String("grpc", cfg.GRPCAddr),
		zap.String("http", cfg.HTTPAddr))
	return app.Run(ctx, log,
		consumer.Run,
		tail.Run,
		proc.Run,
		func(ctx context.Context) error { return grpcx.Serve(ctx, grpcSrv, cfg.GRPCAddr, log) },
		func(ctx context.Context) error { return httpx.Serve(ctx, cfg.HTTPAddr, handler, log) },
	)
}
