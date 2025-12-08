package main

import (
	"log"
	"time"

	"simple-securities/config"
	marketpb "simple-securities/gen/market/v1"
	"simple-securities/internal/market/application/service"
	grpcHandler "simple-securities/internal/market/handler/grpc"
	"simple-securities/pkg/client"
	"simple-securities/pkg/conv"
	"simple-securities/pkg/logger"
	"simple-securities/pkg/server"
	"simple-securities/pkg/server/grpc"

	"go.uber.org/zap"
	googleGrpc "google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func main() {
	config.Init("./config", "market")

	logger.Init()
	logger.Logger.Info("🚀 Application starting",
		zap.String("service", config.GlobalConfig.App.Name),
		zap.String("version", config.GlobalConfig.App.Version),
		zap.String("port", conv.ConvertUInt32ToString(config.GlobalConfig.GrpcServer.Port)),
		zap.String("env", string(config.GlobalConfig.Env)))

	c := client.NewPublicClient(config.GlobalConfig.App.Name)

	pingSvc := service.NewPingSvc(c)
	serverTimeSvc := service.NewServerTimeSvc(c)
	exchangeInfoSvc := service.NewExchangeInfoSvc(c)
	orderBookSvc := service.NewOrderBookSvc(c)
	recentTradesSvc := service.NewRecentTradesSvc(c)
	historicalTradeLookupSvc := service.NewHistoricalTradeLookupSvc(c)
	aggTradesListSvc := service.NewAggTradesListSvc(c)
	klinesSvc := service.NewKlinesSvc(c)
	uiKlinesSvc := service.NewUiKlinesSvc(c)
	avgPriceSvc := service.NewAvgPriceSvc(c)
	ticker24hrSvc := service.NewTicker24hrSvc(c)
	tickerPriceSvc := service.NewTickerPriceSvc(c)
	tickerBookTickerSvc := service.NewTickerBookTickerSvc(c)
	tickerSvc := service.NewTickerSvc(c)

	marketHandler := grpcHandler.NewMarketGrpcHandler(grpcHandler.MarketGrpcSvc{
		PingSvc:                  pingSvc,
		ServerTimeSvc:            serverTimeSvc,
		ExchangeInfoSvc:          exchangeInfoSvc,
		OrderBookSvc:             orderBookSvc,
		RecentTradesSvc:          recentTradesSvc,
		HistoricalTradeLookupSvc: historicalTradeLookupSvc,
		AggTradesListSvc:         aggTradesListSvc,
		KlinesSvc:                klinesSvc,
		UiKlinesSvc:              uiKlinesSvc,
		AvgPriceSvc:              avgPriceSvc,
		Ticker24hrSvc:            ticker24hrSvc,
		TickerPriceSvc:           tickerPriceSvc,
		TickerBookTickerSvc:      tickerBookTickerSvc,
		TickerSvc:                tickerSvc,
	})

	// Create the gRPC server
	grpcServer, err := grpc.NewGrpcServer(
		grpc.GrpcServerConfig{
			Port: config.GlobalConfig.GrpcServer.Port,
			KeepaliveParams: keepalive.ServerParameters{
				MaxConnectionIdle:     time.Duration(config.GlobalConfig.GrpcServer.MaxConnectionIdle),
				MaxConnectionAge:      time.Duration(config.GlobalConfig.GrpcServer.MaxConnectionAge),
				MaxConnectionAgeGrace: time.Duration(config.GlobalConfig.GrpcServer.MaxConnectionAgeGrace),
				Time:                  time.Duration(config.GlobalConfig.GrpcServer.Time),
				Timeout:               time.Duration(config.GlobalConfig.GrpcServer.Timeout),
			},
			KeepalivePolicy: keepalive.EnforcementPolicy{
				MinTime:             time.Duration(config.GlobalConfig.GrpcServer.MinTime),
				PermitWithoutStream: config.GlobalConfig.GrpcServer.PermitWithoutStream,
			},
			UnaryInterceptor: logger.LoggingInterceptor,
		},
	)
	if err != nil {
		log.Fatalf("failed to new grpc server err=%s\n", err.Error())
	}

	// Start the gRPC server
	go grpcServer.Start(
		func(server *googleGrpc.Server) {
			marketpb.RegisterMarketServiceServer(server, marketHandler)
		},
	)

	// Add shutdown hook to trigger closer resources of service
	server.AddShutdownHook(grpcServer, nil)
}
