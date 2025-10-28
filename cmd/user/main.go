package main

import (
	"context"
	"log"
	"time"

	"simple-securities/config"
	user "simple-securities/gen/user/v1"
	"simple-securities/internal/user/application/service"
	grpcHandler "simple-securities/internal/user/handler/grpc"
	"simple-securities/internal/user/infras/repo"
	"simple-securities/internal/user/middleware"
	"simple-securities/pkg/conv"
	"simple-securities/pkg/db/mysql"
	"simple-securities/pkg/logger"
	"simple-securities/pkg/server"
	"simple-securities/pkg/server/grpc"

	"go.uber.org/zap"
	googleGrpc "google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func main() {
	ctx := context.Background()

	config.Init("./config", "user")

	logger.Init()
	logger.Logger.Info("🚀 application starting",
		zap.String("service", config.GlobalConfig.App.Name),
		zap.String("version", config.GlobalConfig.App.Version),
		zap.String("port", conv.ConvertUInt32ToString(config.GlobalConfig.GrpcServer.Port)),
		zap.String("env", string(config.GlobalConfig.Env)))

	db, err := mysql.NewMySQLClient(mysql.MySQLConfig{
		User:         config.GlobalConfig.MySQL.User,
		Password:     config.GlobalConfig.MySQL.Password,
		Host:         config.GlobalConfig.MySQL.Host,
		Port:         config.GlobalConfig.MySQL.Port,
		Database:     config.GlobalConfig.MySQL.Database,
		CharSet:      config.GlobalConfig.MySQL.CharSet,
		ParseTime:    config.GlobalConfig.MySQL.ParseTime,
		TimeZone:     config.GlobalConfig.MySQL.TimeZone,
		MaxIdleConns: config.GlobalConfig.MySQL.MaxIdleConns,
		MaxOpenConns: config.GlobalConfig.MySQL.MaxOpenConns,
		MaxLifeTime:  30 * time.Minute,
		MaxIdleTime:  10 * time.Minute,
	})
	if err != nil {
		log.Fatalf("Failed to connect to SQLite: %v", err)
	}
	defer db.Close(ctx)
	db.AutoMigrate([]string{
		"migrations/mysql/000002_init.userdb.up.sql",
	})

	userRepo := repo.NewUserRepo(db.DB)
	userRegisterSvc := service.NewUserRegisterSvc(userRepo)
	userLoginSvc := service.NewUserLoginSvc(userRepo)
	userGetProfileSvc := service.NewUserGetProfileSvc(userRepo)
	userHandler := grpcHandler.NewUserGrpcHandler(
		userRegisterSvc,
		userLoginSvc,
		userGetProfileSvc,
	)

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
			user.RegisterUserServiceServer(server, userHandler)
		},
	)

	// Add shutdown hook to trigger closer resources of service
	server.AddShutdownHook(grpcServer, db.DB)
}
