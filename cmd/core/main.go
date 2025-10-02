package main

import (
	"context"
	"log"
	"net/http"
	"os"

	crypto "simple-securities/gen/crypto/v1"
	noti "simple-securities/gen/notification/v1"
	stock "simple-securities/gen/stock/v1"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	notiAddr := os.Getenv("NOTI_ADDR")
	if notiAddr == "" {
		notiAddr = "localhost:50052"
	}

	cryptoAddr := os.Getenv("CRYPTO_ADDR")
	if cryptoAddr == "" {
		cryptoAddr = "localhost:50053"
	}

	stockAddr := os.Getenv("STOCK_ADDR")
	if stockAddr == "" {
		stockAddr = "localhost:50054"
	}

	corePort := os.Getenv("CORE_PORT")
	if corePort == "" {
		corePort = ":8080"
	}

	mux := runtime.NewServeMux()

	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	if err := noti.RegisterNotificationServiceHandlerFromEndpoint(ctx, mux, notiAddr, opts); err != nil {
		log.Fatalf("Failed to register NotificationService: %v", err)
	}

	if err := crypto.RegisterCryptoServiceHandlerFromEndpoint(ctx, mux, cryptoAddr, opts); err != nil {
		log.Fatalf("Failed to register CryptoService: %v", err)
	}

	if err := stock.RegisterStockServiceHandlerFromEndpoint(ctx, mux, stockAddr, opts); err != nil {
		log.Fatalf("Failed to register StockService: %v", err)
	}

	if err := http.ListenAndServe(corePort, mux); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
