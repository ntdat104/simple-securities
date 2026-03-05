package main

import (
	"context"
	"log"
	"time"

	grpcClient "simple-securities/common/client/grpc"
	"simple-securities/common/constants"
	"simple-securities/config"
	user "simple-securities/gen/user/v1"
	"simple-securities/internal/user/application/service"
	grpcHandler "simple-securities/internal/user/handler/grpc"
	"simple-securities/internal/user/infras/repo"
	"simple-securities/pkg/conv"
	"simple-securities/pkg/db/sqlite"
	"simple-securities/pkg/db/txmanager"
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
		zap.String(constants.Service, config.GlobalConfig.App.Name),
		zap.String(constants.Version, config.GlobalConfig.App.Version),
		zap.String(constants.Port, conv.ConvertUInt32ToString(config.GlobalConfig.GrpcServer.Port)),
		zap.String(constants.Env, string(config.GlobalConfig.Env)))

	// db, err := mysql.NewMySQLClient(mysql.MySQLConfig{
	// 	User:         config.GlobalConfig.MySQL.User,
	// 	Password:     config.GlobalConfig.MySQL.Password,
	// 	Host:         config.GlobalConfig.MySQL.Host,
	// 	Port:         config.GlobalConfig.MySQL.Port,
	// 	Database:     config.GlobalConfig.MySQL.Database,
	// 	CharSet:      config.GlobalConfig.MySQL.CharSet,
	// 	ParseTime:    config.GlobalConfig.MySQL.ParseTime,
	// 	TimeZone:     config.GlobalConfig.MySQL.TimeZone,
	// 	MaxIdleConns: config.GlobalConfig.MySQL.MaxIdleConns,
	// 	MaxOpenConns: config.GlobalConfig.MySQL.MaxOpenConns,
	// 	MaxLifeTime:  30 * time.Minute,
	// 	MaxIdleTime:  10 * time.Minute,
	// })
	db, err := sqlite.NewSQLiteClient()
	if err != nil {
		log.Fatalf("Failed to connect to SQLite: %v", err)
	}
	defer db.Close(ctx)
	db.AutoMigrate([]string{
		"migrations/sqlite/000002_init_userdb.up.sql",
		"migrations/sqlite/000002_init_user_history_db.up.sql",
	})

	tx := txmanager.NewTxManager(db.DB)

	notiClient, err := grpcClient.NewNotificationGrpcClient(config.GlobalConfig.InternalService.NotificationService)
	if err != nil {
		log.Fatalf("failed to create notification grpc client: %v", err)
	}
	defer notiClient.Close()

	userRepo := repo.NewUserRepo(db.DB)
	userHistoryRepo := repo.NewUserHistoryRepo(db.DB)
	registerSvc := service.NewRegisterSvc(tx, userRepo, userHistoryRepo)
	loginSvc := service.NewLoginSvc(notiClient, userRepo)
	refreshTokenSvc := service.NewRefreshTokenSvc(userRepo)
	getUserProfileSvc := service.NewGetUserProfileSvc(notiClient, userRepo)
	userHandler := grpcHandler.NewUserGrpcHandler(
		grpcHandler.UserGrpcSvc{
			LoginSvc:          loginSvc,
			RegisterSvc:       registerSvc,
			GetUserProfileSvc: getUserProfileSvc,
			RefreshTokenSvc:   refreshTokenSvc,
		},
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
			UnaryInterceptor: logger.LoggingInterceptor,
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
