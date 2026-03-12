//go:build wireinject
// +build wireinject

package app

import (
	grpcClient "simple-securities/common/client/grpc"
	"simple-securities/internal/user/di"
	grpcHandler "simple-securities/internal/user/handler/grpc"
	"simple-securities/pkg/db/txmanager"
	"simple-securities/pkg/kafka"

	"github.com/google/wire"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

func InitializeUserHandler(
	db *sqlx.DB,
	log *zap.Logger,
	kafkaManager *kafka.Manager,
	notiClient *grpcClient.NotificationGrpcClient,
	marketClient *grpcClient.MarketGrpcClient,
	cryptoClient *grpcClient.CryptoGrpcClient,
) (grpcHandler.UserGrpcSvc, error) {
	wire.Build(
		di.RepositorySet,
		di.KafkaEventPublisherSet,
		txmanager.NewTxManager,
		di.ServiceSet,
		di.GrpcHandlerSet,
	)
	return grpcHandler.UserGrpcSvc{}, nil
}
