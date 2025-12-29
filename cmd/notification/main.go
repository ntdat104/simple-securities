package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"simple-securities/config"
	noti "simple-securities/gen/notification/v1"
	"simple-securities/internal/notification/application/service"
	grpcHandler "simple-securities/internal/notification/handler/grpc"
	"simple-securities/internal/notification/infras/repo"
	"simple-securities/pkg/binance"
	"simple-securities/pkg/conv"
	"simple-securities/pkg/datetime"
	"simple-securities/pkg/db/cache"
	"simple-securities/pkg/db/sqlite"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/logger"
	"simple-securities/pkg/server"
	"simple-securities/pkg/server/grpc"
	"simple-securities/pkg/uuid"

	"go.uber.org/zap"
	googleGrpc "google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func main() {
	ctx := context.Background()

	config.Init("./config", "notification")

	logger.Init()
	logger.Logger.Info("🚀 application starting",
		zap.String("service", config.GlobalConfig.App.Name),
		zap.String("version", config.GlobalConfig.App.Version),
		zap.String("port", conv.ConvertUInt32ToString(config.GlobalConfig.GrpcServer.Port)),
		zap.String("env", string(config.GlobalConfig.Env)))

	// Kafka
	kafkaCfg := kafka.Config{
		ServiceName: config.GlobalConfig.App.Name,
		Version:     config.GlobalConfig.App.Version,
		Port:        conv.ConvertUInt32ToString(config.GlobalConfig.GrpcServer.Port),
		Env:         string(config.GlobalConfig.Env),
	}
	kafkaBrokers := []string{"localhost:9092", "localhost:9093", "localhost:9094"}

	// Create Kafka Manager
	mgr := kafka.NewManager(kafkaCfg, kafkaBrokers, logger.Logger)
	defer mgr.Close()

	binanceClient := binance.NewClient("", "", "https://api.binance.com")

	// -----------------------------
	// Producer loop
	// -----------------------------
	go func() {
		for {
			ticker, err := binanceClient.NewTickerService().
				Symbol("BTCUSDT").Do(context.Background())
			if err != nil {
				fmt.Println(err)
				continue
			}

			now := datetime.Now()
			event := kafka.Event{
				Meta: kafka.Meta{
					ServiceName: config.GlobalConfig.App.Name,
					RequestID:   uuid.NewGoogleUUID(),
					Code:        200,
					Message:     "Success",
					Timestamp:   now.Unix(),
					Datetime:    datetime.ConvertTimeToString(now, datetime.YYYY_MM_DD_HH_MM_SS),
				},
				Data: ticker,
			}

			// Send to metrics
			if err := mgr.NewSendMessage().
				Topic("metrics").
				Key("metrics-key").
				Partition(1).
				Headers(map[string]string{
					"request_id": event.Meta.RequestID,
					"timestamp":  conv.ConvertInt64ToString(event.Meta.Timestamp),
					"datetime":   event.Meta.Datetime,
				}).
				Event(event).
				Do(context.Background()); err != nil {
				logger.Logger.Error("failed to send metrics event", zap.Error(err))
			}

			// Send to audit
			if err := mgr.NewSendMessage().
				Topic("audit").
				Key("audit-key").
				Headers(map[string]string{
					"request_id": event.Meta.RequestID,
					"timestamp":  conv.ConvertInt64ToString(event.Meta.Timestamp),
					"datetime":   event.Meta.Datetime,
				}).
				Event(event).
				Do(context.Background()); err != nil {
				logger.Logger.Error("failed to send audit event", zap.Error(err))
			}

			time.Sleep(4 * time.Second)
		}
	}()

	db, err := sqlite.NewSQLiteClient()
	if err != nil {
		log.Fatalf("Failed to connect to SQLite: %v", err)
	}
	defer db.Close(ctx)
	db.AutoMigrate([]string{
		"migrations/sqlite/000001_init_notificationdb.up.sql",
		"migrations/sqlite/000001_seed_notifications.up.sql",
	})

	rdb, err := cache.NewRedisClient(cache.DefaultRedisConfig())
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer rdb.Close()

	notiRepo := repo.NewNotificationRepo(db.DB)
	notiCacheRepo := repo.NewNotificationCacheRepo(rdb.Client)
	sendNotiSvc := service.NewSendNotiSvc(notiRepo, notiCacheRepo)
	getNotiSvc := service.NewGetNotiSvc(notiRepo, notiCacheRepo)
	getNotiByUserIdSvc := service.NewGetNotiByUserIdSvc(notiRepo, notiCacheRepo)
	notiHandler := grpcHandler.NewNotificationGrpcHandler(sendNotiSvc, getNotiSvc, getNotiByUserIdSvc)

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
			noti.RegisterNotificationServiceServer(server, notiHandler)
		},
	)

	// Add shutdown hook to trigger closer resources of service
	server.AddShutdownHook(grpcServer, db.DB)
}
