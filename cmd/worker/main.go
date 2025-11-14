package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"simple-securities/pkg/db/cache"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Event struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	ID     int      `json:"id"`
}

const (
	DefaultURL = "wss://stream.binance.com/stream"
)

type BinanceWsClient struct {
	url    string
	conn   *websocket.Conn
	params map[string]uint16 // subscription reference counter
	mu     sync.Mutex        // protects params map & conn
}

// -------------------------------------------------------------
// Constructor
// -------------------------------------------------------------
func NewBinanceWsClient(baseURL ...string) (*BinanceWsClient, error) {
	url := DefaultURL
	if len(baseURL) > 0 {
		url = baseURL[0]
	}

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &BinanceWsClient{
		url:    url,
		conn:   conn,
		params: make(map[string]uint16),
	}, nil
}

// -------------------------------------------------------------
// Internal helper to send message over WS
// -------------------------------------------------------------
func (c *BinanceWsClient) sendWSMessage(event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event error: %w", err)
	}
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// -------------------------------------------------------------
// Subscribe / Unsubscribe
// -------------------------------------------------------------
func (c *BinanceWsClient) Send(ctx context.Context, event Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// SUBSCRIBE
	if event.Method == "SUBSCRIBE" {
		newSubParams := []string{}

		for _, p := range event.Params {
			if c.params[p] == 0 {
				newSubParams = append(newSubParams, p)
			}
			c.params[p]++
		}

		if len(newSubParams) > 0 {
			return c.sendWSMessage(Event{
				Method: "SUBSCRIBE",
				Params: newSubParams,
				ID:     1,
			})
		}
	}

	// UNSUBSCRIBE
	if event.Method == "UNSUBSCRIBE" {
		newUnSubParams := []string{}

		for _, p := range event.Params {
			if c.params[p] == 1 {
				newUnSubParams = append(newUnSubParams, p)
				delete(c.params, p)
			} else if c.params[p] > 1 {
				c.params[p]--
			}
		}

		if len(newUnSubParams) > 0 {
			return c.sendWSMessage(Event{
				Method: "UNSUBSCRIBE",
				Params: newUnSubParams,
				ID:     1,
			})
		}
	}

	return nil
}

// -------------------------------------------------------------
// Listen to Binance messages and publish to Redis
// -------------------------------------------------------------
func (c *BinanceWsClient) SubscribeBinance(ctx context.Context, redisClient *cache.RedisClient) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Binance subscription stopped.")
			return
		default:
			_, msg, err := c.conn.ReadMessage()
			if err != nil {
				log.Println("WS read error:", err)
				time.Sleep(1 * time.Second)
				continue
			}

			var raw map[string]any
			if err := json.Unmarshal(msg, &raw); err != nil {
				log.Println("JSON parse error:", err)
				continue
			}

			stream, ok := raw["stream"].(string)
			if !ok {
				log.Println("No stream field in message")
				continue
			}

			// Publish to Redis
			if err := redisClient.Client.Publish(redisClient.Client.Context(), stream, msg).Err(); err != nil {
				log.Println("Redis publish error:", err)
			}
		}
	}
}

// -------------------------------------------------------------
// Listen to Redis events and send to Binance
// -------------------------------------------------------------
func (c *BinanceWsClient) SubscribeRedis(ctx context.Context, redisClient *cache.RedisClient, channel string) {
	pubsub := redisClient.Client.Subscribe(redisClient.Client.Context(), channel)
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			log.Println("Redis subscription stopped.")
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}

			var event Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Println("Invalid event from Redis:", err)
				continue
			}

			if err := c.Send(ctx, event); err != nil {
				log.Println("Failed to send event to Binance:", err)
			}
		}
	}
}

// -------------------------------------------------------------
// Main
// -------------------------------------------------------------
func main() {
	ctx := context.Background()

	binanceClient, err := NewBinanceWsClient()
	if err != nil {
		panic(err)
	}

	redisClient, err := cache.NewRedisClient(nil)
	if err != nil {
		panic(err)
	}

	err = redisClient.HealthCheck(ctx)
	if err != nil {
		panic(err)
	}

	// Binance → Redis
	go binanceClient.SubscribeBinance(ctx, redisClient)

	// Redis → Binance (listen to "binance-events" channel)
	go binanceClient.SubscribeRedis(ctx, redisClient, "binance-events")

	// binanceClient.Send(ctx, Event{
	// 	Method: "SUBSCRIBE",
	// 	Params: []string{"btcusdt@kline_1m", "ethusdt@kline_1m", "bnbusdt@kline_1m"},
	// })

	select {}
}
