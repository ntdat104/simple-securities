package main

import (
	"context"
	"log"
	"os"
	"time"

	crypto "github.com/simple-securities/internal/pkg/gen/crypto/v1"
	noti "github.com/simple-securities/internal/pkg/gen/notification/v1"
	stock "github.com/simple-securities/internal/pkg/gen/stock/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CoreService struct {
	notiClient   noti.NotificationServiceClient
	cryptoClient crypto.CryptoServiceClient
	stockClient  stock.StockServiceClient
}

func NewCoreService(n noti.NotificationServiceClient, c crypto.CryptoServiceClient, s stock.StockServiceClient) *CoreService {
	return &CoreService{
		notiClient:   n,
		cryptoClient: c,
		stockClient:  s,
	}
}

func (c *CoreService) Run() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 🔔 Test Notification Service
	sendResp, err := c.notiClient.Send(ctx, &noti.SendRequest{
		UserId:  "123",
		Message: "Hello from Core Service 👋",
	})
	if err != nil {
		log.Fatalf("❌ Failed to send notification: %v", err)
	}
	log.Printf("✅ Notification sent: %v", sendResp.Success)

	getResp, err := c.notiClient.Get(ctx, &noti.GetRequest{UserId: "123"})
	if err != nil {
		log.Fatalf("❌ Failed to get notifications: %v", err)
	}
	for _, n := range getResp.Notifications {
		log.Printf("📩 Notification: ID=%s UserId=%s Msg=%s Time=%d",
			n.Id, n.UserId, n.Message, n.Timestamp)
	}

	// ⏰ Test Crypto Service
	cryptoResp, err := c.cryptoClient.GetServerTime(ctx, &crypto.GetServerTimeRequest{})
	if err != nil {
		log.Fatalf("❌ Failed to get server time: %v", err)
	}
	log.Printf("⏰ Crypto Service server time: %d", cryptoResp.GetServerTime())

	// 📡 Test Stock Service
	stockResp, err := c.stockClient.Ping(ctx, &stock.PingRequest{})
	if err != nil {
		log.Fatalf("❌ Failed to ping stock service: %v", err)
	}
	log.Printf("📡 Stock Service Ping response: %s", stockResp.Message)
}

func main() {
	// Notification service host/port
	notiAddr := os.Getenv("NOTI_ADDR")
	if notiAddr == "" {
		notiAddr = "localhost:50052"
	}

	// Crypto service host/port
	cryptoAddr := os.Getenv("CRYPTO_ADDR")
	if cryptoAddr == "" {
		cryptoAddr = "localhost:50053"
	}

	// Stock service host/port
	stockAddr := os.Getenv("STOCK_ADDR")
	if stockAddr == "" {
		stockAddr = "localhost:50054"
	}

	// Connect to Notification Service
	notiConn, err := grpc.Dial(notiAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("❌ Failed to connect to Notification Service: %v", err)
	}
	defer notiConn.Close()

	// Connect to Crypto Service
	cryptoConn, err := grpc.Dial(cryptoAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("❌ Failed to connect to Crypto Service: %v", err)
	}
	defer cryptoConn.Close()

	// Connect to Stock Service
	stockConn, err := grpc.Dial(stockAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("❌ Failed to connect to Stock Service: %v", err)
	}
	defer stockConn.Close()

	// Init CoreService with 3 clients
	coreSvc := NewCoreService(
		noti.NewNotificationServiceClient(notiConn),
		crypto.NewCryptoServiceClient(cryptoConn),
		stock.NewStockServiceClient(stockConn),
	)

	log.Println("🚀 Core Service started. Calling other services...")
	coreSvc.Run()
}
