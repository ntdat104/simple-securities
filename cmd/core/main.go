package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"sync"

	crypto "simple-securities/gen/crypto/v1"
	market "simple-securities/gen/market/v1"
	noti "simple-securities/gen/notification/v1"
	stock "simple-securities/gen/stock/v1"
	user "simple-securities/gen/user/v1"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  *sync.Mutex
	r   rate.Limit
	b   int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		mu:  &sync.Mutex{},
		r:   r,
		b:   b,
	}
}

func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(i.r, i.b)
		i.ips[ip] = limiter
	}

	return limiter
}

func RateLimitMiddleware(limiter *IPRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract IP address from RemoteAddr (stripping the port)
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			// Fallback if SplitHostPort fails
			ip = r.RemoteAddr
		}

		if !limiter.GetLimiter(ip).Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

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

	userAddr := os.Getenv("USER_ADDR")
	if userAddr == "" {
		userAddr = "localhost:50054"
	}

	stockAddr := os.Getenv("STOCK_ADDR")
	if stockAddr == "" {
		stockAddr = "localhost:50055"
	}

	marketAddr := os.Getenv("MARKET_ADDR")
	if marketAddr == "" {
		marketAddr = "localhost:50055"
	}

	corePort := os.Getenv("CORE_PORT")
	if corePort == "" {
		corePort = ":8080"
	}

	mux := runtime.NewServeMux()

	// Create a limiter: 5 requests per second, burst of 10
	limiter := NewIPRateLimiter(20, 40)

	// Wrap the mux with the middleware
	handler := RateLimitMiddleware(limiter, mux)

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

	if err := market.RegisterMarketServiceHandlerFromEndpoint(ctx, mux, stockAddr, opts); err != nil {
		log.Fatalf("Failed to register StockService: %v", err)
	}

	if err := user.RegisterUserServiceHandlerFromEndpoint(ctx, mux, userAddr, opts); err != nil {
		log.Fatalf("Failed to register UserService: %v", err)
	}

	if err := http.ListenAndServe(corePort, handler); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
