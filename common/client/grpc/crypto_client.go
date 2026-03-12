package grpc

import (
	"fmt"
	crypto "simple-securities/gen/crypto/v1"
	"simple-securities/pkg/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CryptoGrpcClient struct {
	client crypto.CryptoServiceClient
	conn   *grpc.ClientConn
}

func NewCryptoGrpcClient(target string) (*CryptoGrpcClient, error) {
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(logger.ForwardMetadataInterceptor),
	)
	if err != nil {
		return nil, fmt.Errorf("could not connect to grpc server: %v", err)
	}

	return &CryptoGrpcClient{
		client: crypto.NewCryptoServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *CryptoGrpcClient) Close() error {
	return c.conn.Close()
}
