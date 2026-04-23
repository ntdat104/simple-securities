//go:build wireinject
// +build wireinject

package app

import (
	grpcClient "simple-securities/common/client/grpc"
	"simple-securities/config"
	"simple-securities/internal/user/di"
	grpcHandler "simple-securities/internal/user/handler/grpc"
	"simple-securities/internal/user/scheduler"
	"simple-securities/pkg/db/cache"
	"simple-securities/pkg/db/txmanager"
	"simple-securities/pkg/kafka"

	"github.com/google/wire"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

func InitializeCoreScheduler(
	db *sqlx.DB,
	log *zap.Logger,
) *scheduler.UserCronScheduler {
	wire.Build(
		di.RepositorySet,
		di.SchedulerSet,
	)
	return nil
}

func InitializeUserHandler(
	db *sqlx.DB,
	hybridCache *cache.HybridCache,
	log *zap.Logger,
	kafkaManager *kafka.Manager,
	cfg *config.Config,
) (grpcHandler.UserGrpcSvc, func(), error) {
	wire.Build(
		provideNotificationFunc,
		provideMarketFunc,
		provideCryptoFunc,
		di.RepositorySet,
		di.KafkaEventPublisherSet,
		txmanager.NewTxManager,
		di.ServiceSet,
		di.GrpcHandlerSet,
	)
	return grpcHandler.UserGrpcSvc{}, nil, nil
}

// grpc-clients
func provideNotificationFunc(cfg *config.Config) (*grpcClient.NotificationGrpcClient, func(), error) {
	c, err := grpcClient.NewNotificationGrpcClient(cfg.InternalService.NotificationService)
	if err != nil {
		return nil, nil, err
	}
	return c, func() { c.Close() }, nil
}

func provideMarketFunc(cfg *config.Config) (*grpcClient.MarketGrpcClient, func(), error) {
	c, err := grpcClient.NewMarketGrpcClient(cfg.InternalService.MarketService)
	if err != nil {
		return nil, nil, err
	}
	return c, func() { c.Close() }, nil
}

func provideCryptoFunc(cfg *config.Config) (*grpcClient.CryptoGrpcClient, func(), error) {
	c, err := grpcClient.NewCryptoGrpcClient(cfg.InternalService.CryptoService)
	if err != nil {
		return nil, nil, err
	}
	return c, func() { c.Close() }, nil
}
