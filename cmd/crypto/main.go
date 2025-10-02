package main

import (
	"context"
	"log"
	"time"

	"simple-securities/config"
	crypto "simple-securities/gen/crypto/v1"
	"simple-securities/internal/crypto/application/service"
	grpcHandler "simple-securities/internal/crypto/handler/grpc"
	"simple-securities/internal/crypto/middleware"
	"simple-securities/pkg/conv"
	"simple-securities/pkg/db/sqlite"
	"simple-securities/pkg/logger"
	"simple-securities/pkg/server"
	"simple-securities/pkg/server/grpc"

	"go.uber.org/zap"
	googleGrpc "google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func main() {
	ctx := context.Background()

	config.Init("./config", "crypto")

	logger.Init()
	logger.Logger.Info("🚀 Application starting",
		zap.String("service", config.GlobalConfig.App.Name),
		zap.String("version", config.GlobalConfig.App.Version),
		zap.String("port", conv.ConvertUInt32ToString(config.GlobalConfig.GrpcServer.Port)),
		zap.String("env", string(config.GlobalConfig.Env)))

	db, err := sqlite.NewSQLiteClient()
	if err != nil {
		log.Fatalf("Failed to connect to SQLite: %v", err)
	}
	defer db.Close(ctx)

	getServerTimeSvc := service.NewGetServerTimeSvc()
	getKlinesSvc := service.NewGetKlinesSvc()
	cryptoHandler := grpcHandler.NewCryptoGrpcHandler(getKlinesSvc, getServerTimeSvc)

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
			UnaryInterceptor: middleware.LoggingInterceptor,
		},
	)
	if err != nil {
		log.Fatalf("failed to new grpc server err=%s\n", err.Error())
	}

	// Start the gRPC server
	go grpcServer.Start(
		func(server *googleGrpc.Server) {
			crypto.RegisterCryptoServiceServer(server, cryptoHandler)
		},
	)

	// Add shutdown hook to trigger closer resources of service
	server.AddShutdownHook(grpcServer, db.DB)
}
