package main

import (
	"context"
	"log"
	"time"

	"simple-securities/cmd/user/app"
	"simple-securities/common/constants"
	"simple-securities/config"
	user "simple-securities/gen/user/v1"
	grpcHandler "simple-securities/internal/user/handler/grpc"
	"simple-securities/pkg/conv"
	"simple-securities/pkg/db/cache"
	"simple-securities/pkg/db/sqlite"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/logger"
	"simple-securities/pkg/server"
	pkgGrpc "simple-securities/pkg/server/grpc"

	"github.com/dgraph-io/ristretto/v2"
	"go.uber.org/zap"
	googleGrpc "google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func main() {
	ctx := context.Background()

	config.Init("./config", "user")

	logger.Init()
	logger.Logger.Info("application starting",
		zap.String(constants.Service, config.GlobalConfig.App.Name),
		zap.String(constants.Version, config.GlobalConfig.App.Version),
		zap.String(constants.Port, conv.ConvertUInt32ToString(config.GlobalConfig.GrpcServer.Port)),
		zap.String(constants.Env, string(config.GlobalConfig.Env)))

	db, err := sqlite.NewSQLiteClient()
	if err != nil {
		log.Fatalf("Failed to connect to SQLite: %v", err)
	}
	defer db.Close(ctx)
	db.AutoMigrate([]string{
		"migrations/sqlite/000002_init_userdb.up.sql",
		"migrations/sqlite/000002_init_user_history_db.up.sql",
	})

	redisCache, err := cache.NewRedisClient(cache.DefaultRedisConfig())
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisCache.Close()

	memCache, err := ristretto.NewCache(&ristretto.Config[string, any]{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache (1GB).
		BufferItems: 64,      // number of keys per Get buffer.
	})
	if err != nil {
		log.Fatalf("Failed to setup memCache: %v", err)
	}

	hybridCache := cache.NewHybridCache(memCache, redisCache.Client)

	// Kafka Setup
	kafkaCfg := kafka.Config{
		ServiceName: config.GlobalConfig.App.Name,
		Version:     config.GlobalConfig.App.Version,
		Port:        conv.ConvertUInt32ToString(config.GlobalConfig.GrpcServer.Port),
		Env:         string(config.GlobalConfig.Env),
	}
	kafkaBrokers := []string{"localhost:9092", "localhost:9093", "localhost:9094"}
	kafkaManager := kafka.NewManager(kafkaCfg, kafkaBrokers, logger.Logger)
	defer kafkaManager.Close()

	// Initialize cron scheduler via Wire
	cronScheduler := app.InitializeUserScheduler(db.DB, logger.Logger)
	cronScheduler.Start()

	// Initialize App via Wire (Optimized)
	// Tất cả gRPC Clients được khởi tạo và quản lý bên trong Wire
	userSvc, cleanup, err := app.InitializeUserHandler(
		db.DB,
		hybridCache,
		logger.Logger,
		kafkaManager,
		config.GlobalConfig,
	)
	if err != nil {
		log.Fatalf("failed to initialize user handler: %v", err)
	}
	defer cleanup() // TỰ ĐỘNG Close() tất cả gRPC clients (Notification, Market, Crypto,...)

	userHandler := grpcHandler.NewUserGrpcHandler(userSvc)

	grpcServer, err := pkgGrpc.NewGrpcServer(
		pkgGrpc.GrpcServerConfig{
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

	go grpcServer.Start(
		func(server *googleGrpc.Server) {
			user.RegisterUserServiceServer(server, userHandler)
		},
	)

	server.AddShutdownHook(grpcServer, cronScheduler, db.DB)
}
