package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

type KlineMessage struct {
	Stream string `json:"stream"`
	Data   struct {
		EventType string `json:"e"`
		Time      int64  `json:"E"`
		Symbol    string `json:"s"`
		Kline     struct {
			StartTime    int64  `json:"t"`
			CloseTime    int64  `json:"T"`
			Interval     string `json:"i"`
			FirstTradeID int64  `json:"f"`
			LastTradeID  int64  `json:"L"`
			OpenPrice    string `json:"o"`
			ClosePrice   string `json:"c"`
			HighPrice    string `json:"h"`
			LowPrice     string `json:"l"`
			Volume       string `json:"v"`
			IsFinal      bool   `json:"x"`
		} `json:"k"`
	} `json:"data"`
}

const wsURL = "wss://stream.binance.com/stream"

type BinanceService struct {
	tickers []string
	rdb     *redis.Client
	conn    *websocket.Conn
}

// NewBinanceService creates a new BinanceService instance, accepting a pre-configured Redis client
func NewBinanceService(tickers []string, rdb *redis.Client) (*BinanceService, error) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	return &BinanceService{
		tickers: tickers,
		rdb:     rdb,
		conn:    conn,
	}, nil
}

// Subscribe subscribes to Binance WebSocket streams
func (b *BinanceService) Subscribe() error {
	const batchSize = 100
	// intervals := []string{
	// 	"1s",
	// 	"1m",
	// 	"3m",
	// 	"5m",
	// 	"15m",
	// 	"30m",
	// 	"1h",
	// 	"2h",
	// 	"4h",
	// 	"6h",
	// 	"8h",
	// 	"12h",
	// 	"1d",
	// 	"3d",
	// 	"1w",
	// 	"1M",
	// }
	intervals := []string{"1s"}

	for _, interval := range intervals {
		for i := 0; i < len(b.tickers); i += batchSize {
			end := i + batchSize
			if end > len(b.tickers) {
				end = len(b.tickers)
			}
			batch := b.tickers[i:end]
			params := make([]string, len(batch))
			for j, ticker := range batch {
				params[j] = fmt.Sprintf("%s@kline_%s", strings.ToLower(ticker), interval)
			}
			subMessage := map[string]interface{}{
				"method": "SUBSCRIBE",
				"params": params,
				"id":     1,
			}
			if err := b.conn.WriteJSON(subMessage); err != nil {
				return fmt.Errorf("failed to send subscription message for batch starting at %d with interval %s: %w", i, interval, err)
			}
			log.Printf("Subscribed to Binance WebSocket stream for tickers: %v with interval: %s\n", len(batch), interval)
			time.Sleep(300 * time.Millisecond)
		}
	}

	return nil
}

// ReadAndPublish reads messages from the WebSocket and publishes them to Redis
func (b *BinanceService) ReadAndPublish(ctx context.Context) {
	for {
		_, message, err := b.conn.ReadMessage()
		if err != nil {
			log.Fatalf("Failed to read WebSocket message: %v", err)
		}

		var klineMsg KlineMessage
		if err := json.Unmarshal(message, &klineMsg); err != nil {
			log.Printf("Failed to unmarshal WebSocket message: %v", err)
			continue
		}

		channel := strings.ToUpper(klineMsg.Data.Symbol)
		if channel == "" {
			continue
		}

		if err := b.rdb.Publish(ctx, channel, message).Err(); err != nil {
			log.Printf("Failed to publish to Redis channel %s: %v", channel, err)
			continue
		}
		// log.Printf("Published %s: %v\n", channel, klineMsg.Data.Kline.ClosePrice)
	}
}

// Close closes the WebSocket connection
func (b *BinanceService) Close() {
	if err := b.conn.Close(); err != nil {
		log.Printf("Failed to close WebSocket connection: %v", err)
	} else {
		log.Println("WebSocket connection closed successfully.")
	}
}

type ExchangeInfo struct {
	Symbols []struct {
		Symbol string `json:"symbol"`
	} `json:"symbols"`
}

func getAllCryptoSymbols() ([]string, error) {
	url := "https://api.binance.com/api/v3/exchangeInfo"

	// Send GET request
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read and parse response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var exchangeInfo ExchangeInfo
	err = json.Unmarshal(body, &exchangeInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	// Extract symbols
	var symbols []string
	for _, s := range exchangeInfo.Symbols {
		if strings.Contains(s.Symbol, "USDT") {
			symbols = append(symbols, s.Symbol)
		}
	}

	return symbols, nil
}

func main() {
	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Redis server address
	})

	// Tickers to subscribe to
	tickers, err := getAllCryptoSymbols()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	// tickers := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "ADAUSDT", "AVAXUSDT", "SUIUSDT", "GALAUSDT", "C98USDT"}

	// Create Binance service with the pre-configured Redis client
	service, err := NewBinanceService(tickers, rdb)
	if err != nil {
		log.Fatalf("Failed to create Binance service: %v", err)
	}
	defer service.Close() // Ensure the WebSocket connection is closed

	// Subscribe to streams
	if err := service.Subscribe(); err != nil {
		log.Fatalf("Failed to subscribe to Binance streams: %v", err)
	}

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go service.ReadAndPublish(ctx)

	// Wait for termination signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Service is running. Press Ctrl+C to stop.")
	<-sigs
	log.Println("Shutting down...")
}
