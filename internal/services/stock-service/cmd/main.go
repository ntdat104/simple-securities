package main

import (
	"context"
	"log"
	"net"
	"os"

	stock "github.com/simple-securities/internal/pkg/gen/stock/v1"
	"google.golang.org/grpc"
)

type StockService struct {
	stock.UnimplementedStockServiceServer
}

func NewStockService() *StockService {
	return &StockService{}
}

// Ping implements the StockService.Ping RPC
func (s *StockService) Ping(ctx context.Context, req *stock.PingRequest) (*stock.PingResponse, error) {
	return &stock.PingResponse{Message: "pong 🏓 from Stock Service"}, nil
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "50054" // default port for stock-service
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("❌ Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	stock.RegisterStockServiceServer(grpcServer, NewStockService())

	log.Printf("🚀 Stock Service running on port %s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("❌ Failed to serve: %v", err)
	}
}
