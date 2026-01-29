package grpc

import (
	"context"
	"fmt"

	noti "simple-securities/gen/notification/v1"
	"simple-securities/pkg/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NotificationGrpcClient struct {
	client noti.NotificationServiceClient
	conn   *grpc.ClientConn
}

func NewNotificationGrpcClient(target string) (*NotificationGrpcClient, error) {
	conn, err := grpc.Dial(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(logger.ForwardMetadataInterceptor),
	)
	if err != nil {
		return nil, fmt.Errorf("could not connect to grpc server: %v", err)
	}

	return &NotificationGrpcClient{
		client: noti.NewNotificationServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *NotificationGrpcClient) Close() error {
	return c.conn.Close()
}

func (c *NotificationGrpcClient) Send(ctx context.Context, req *noti.SendRequest) (*noti.SendResponse, error) {
	return c.client.Send(ctx, req)
}

func (c *NotificationGrpcClient) Get(ctx context.Context, req *noti.GetRequest) (*noti.GetResponse, error) {
	return c.client.Get(ctx, req)
}

func (c *NotificationGrpcClient) GetByUserId(ctx context.Context, req *noti.GetByUserIdRequest) (*noti.GetByUserIdResponse, error) {
	return c.client.GetByUserId(ctx, req)
}
