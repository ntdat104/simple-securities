package main

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	noti "github.com/simple-securities/internal/pkg/gen/notification/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NotificationService struct {
	noti.UnimplementedNotificationServiceServer
}

func NewNotificationService() *NotificationService {
	return &NotificationService{}
}

func (s *NotificationService) Send(ctx context.Context, req *noti.SendRequest) (*noti.SendResponse, error) {
	return &noti.SendResponse{Success: true}, nil
}

func (s *NotificationService) Get(ctx context.Context, req *noti.GetRequest) (*noti.GetResponse, error) {
	return &noti.GetResponse{
		Notifications: []*noti.Notification{
			{Id: "1", UserId: "1", Message: "Notification 1", Timestamp: time.Now().Unix()},
			{Id: "2", UserId: "2", Message: "Notification 2", Timestamp: time.Now().Unix()},
		},
	}, nil
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "50052"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("❌ Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	// Connect to the service registry
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("❌ Could not connect to registry: %v", err)
	}
	defer conn.Close()

	log.Printf("🚀 Notification Service running on port %s", port)
	noti.RegisterNotificationServiceServer(grpcServer, NewNotificationService())
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("❌ Failed to serve: %v", err)
	}
}
