package main

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	crypto "simple-securities/gen/crypto/v1"

	"google.golang.org/grpc"
)

type CryptoService struct {
	crypto.UnimplementedCryptoServiceServer
}

func NewCryptoService() *CryptoService {
	return &CryptoService{}
}

// GetServerTime implements the gRPC method
func (s *CryptoService) GetServerTime(ctx context.Context, req *crypto.GetServerTimeRequest) (*crypto.GetServerTimeResponse, error) {
	serverTime := time.Now().Unix()
	return &crypto.GetServerTimeResponse{
		ServerTime: &serverTime,
	}, nil
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "50053" // default port for crypto-service
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("❌ Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	crypto.RegisterCryptoServiceServer(grpcServer, NewCryptoService())

	log.Printf("🚀 Crypto Service running on port %s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("❌ Failed to serve: %v", err)
	}
}
