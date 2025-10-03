package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	clients   = make(map[*websocket.Conn]map[string]bool)
	clientsMu sync.Mutex
	rdb       = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	ctx = context.Background()
)

type SubscriptionMessage struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)

	go func() {
		log.Println("WebSocket server started on :8888")
		if err := http.ListenAndServe(":8888", nil); err != nil {
			log.Fatalf("WebSocket server failed: %v", err)
		}
	}()

	go redisListener()

	select {}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	clientsMu.Lock()
	clients[conn] = make(map[string]bool)
	clientsMu.Unlock()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket connection closed: %v", err)
			break
		}

		var subMsg SubscriptionMessage
		if err := json.Unmarshal(msg, &subMsg); err != nil {
			log.Printf("Invalid message format: %v", err)
			continue
		}

		switch subMsg.Method {
		case "SUBSCRIBE":
			clientsMu.Lock()
			existingClients := clients[conn]
			newParams := subMsg.Params

			for _, newParam := range newParams {
				existingClients[newParam] = true
			}

			clientsMu.Unlock()
			log.Printf("Client subscribed to channels: %v", clients[conn])

		case "UNSUBSCRIBE":
			clientsMu.Lock()
			existingClients := clients[conn]
			unsubscribeParams := subMsg.Params

			for _, unsubscribeParam := range unsubscribeParams {
				delete(existingClients, unsubscribeParam)
			}

			clientsMu.Unlock()
			log.Printf("Client unsubscribed from channels: %v", unsubscribeParams)
			log.Printf("Remaining subscriptions: %v", clients[conn])

		default:
			log.Printf("Unsupported method: %s", subMsg.Method)
		}
	}

	clientsMu.Lock()
	delete(clients, conn)
	clientsMu.Unlock()
}

func redisListener() {
	sub := rdb.PSubscribe(ctx, "*")
	ch := sub.Channel()

	log.Println("Listening for Redis messages...")
	for msg := range ch {
		if len(clients) > 0 {
			broadcastToSubscribers(msg.Channel, msg.Payload)
		}
	}

	if err := sub.Close(); err != nil {
		log.Fatalf("Error closing subscription: %v", err)
	}
}

func broadcastToSubscribers(channel, message string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	var wg sync.WaitGroup

	for conn, subscribedChannels := range clients {
		for ch := range subscribedChannels {
			if ch == channel {
				wg.Add(1)
				go func(conn *websocket.Conn) {
					defer wg.Done()
					err := conn.WriteMessage(websocket.TextMessage, []byte(message))
					if err != nil {
						log.Printf("Error broadcasting message to WebSocket client: %v", err)
						conn.Close()

						// Safely remove the connection
						clientsMu.Lock()
						delete(clients, conn)
						clientsMu.Unlock()
					}
				}(conn)
				break
			}
		}
	}

	wg.Wait() // Wait for all broadcasts to complete
}
