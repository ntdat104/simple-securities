package grpc

import (
	"fmt"
	market "simple-securities/gen/market/v1"
	"simple-securities/pkg/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type MarketGrpcClient struct {
	client market.MarketServiceClient
	conn   *grpc.ClientConn
}

func NewMarketGrpcClient(target string) (*MarketGrpcClient, error) {
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(logger.ForwardMetadataInterceptor),
	)
	if err != nil {
		return nil, fmt.Errorf("could not connect to grpc server: %v", err)
	}

	return &MarketGrpcClient{
		client: market.NewMarketServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *MarketGrpcClient) Close() error {
	return c.conn.Close()
}
