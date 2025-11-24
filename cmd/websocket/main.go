package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	commonPb "simple-securities/gen/common/v1"

	"google.golang.org/protobuf/proto"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack/v5"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	// A client has its own subscribed stream list
	clients   = make(map[*websocket.Conn]map[string]bool)
	clientsMu sync.Mutex

	// Global reference counter like Binance worker
	globalSubs   = make(map[string]uint32)
	globalSubsMu sync.Mutex

	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6179",
	})
	ctx = context.Background()
)

type WSMessage struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)

	go func() {
		log.Println("WebSocket server listening on :8888")
		if err := http.ListenAndServe(":8888", nil); err != nil {
			log.Fatal(err)
		}
	}()

	go redisListener()

	select {}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Upgrade error:", err)
		return
	}

	clientsMu.Lock()
	clients[conn] = make(map[string]bool)
	clientsMu.Unlock()

	defer func() {
		cleanupClient(conn)
		conn.Close()
	}()

	for {
		messageType, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("WS closed:", err)
			return
		}

		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}

		var event WSMessage
		if err := json.Unmarshal(msg, &event); err != nil {
			log.Println("Invalid WS message:", err)
			continue
		}

		switch event.Method {
		case "SUBSCRIBE":
			handleSubscribe(conn, event.Params)
		case "UNSUBSCRIBE":
			handleUnsubscribe(conn, event.Params)
		default:
			log.Println("Unsupported WS method:", event.Method)
		}
	}
}

func handleSubscribe(conn *websocket.Conn, params []string) {
	clientsMu.Lock()
	clientSubs := clients[conn]
	clientsMu.Unlock()

	newSubs := []string{}

	globalSubsMu.Lock()
	for _, p := range params {
		if !clientSubs[p] {
			clientSubs[p] = true
			if globalSubs[p] == 0 {
				newSubs = append(newSubs, p)
			}
			globalSubs[p]++
		}
	}
	globalSubsMu.Unlock()

	if len(newSubs) > 0 {
		// Tell worker to subscribe on Binance
		data, _ := json.Marshal(WSMessage{
			Method: "SUBSCRIBE",
			Params: newSubs,
		})
		rdb.Publish(ctx, "binance-events", string(data))
	}

	log.Printf("Client subscribed: %v", params)
}

func handleUnsubscribe(conn *websocket.Conn, params []string) {
	clientsMu.Lock()
	clientSubs := clients[conn]
	clientsMu.Unlock()

	newUnsubs := []string{}

	globalSubsMu.Lock()
	for _, p := range params {
		if clientSubs[p] {
			delete(clientSubs, p)

			if globalSubs[p] == 1 {
				newUnsubs = append(newUnsubs, p)
				delete(globalSubs, p)
			} else if globalSubs[p] > 1 {
				globalSubs[p]--
			}
		}
	}
	globalSubsMu.Unlock()

	if len(newUnsubs) > 0 {
		// Tell worker to unsubscribe from Binance
		data, _ := json.Marshal(WSMessage{
			Method: "UNSUBSCRIBE",
			Params: newUnsubs,
		})
		rdb.Publish(ctx, "binance-events", string(data))
	}

	log.Printf("Client unsubscribed: %v", params)
}

func cleanupClient(conn *websocket.Conn) {
	clientsMu.Lock()
	clientSubs, ok := clients[conn]
	if !ok {
		clientsMu.Unlock()
		return
	}
	delete(clients, conn)
	clientsMu.Unlock()

	unsubs := []string{}

	globalSubsMu.Lock()
	for p := range clientSubs {
		if globalSubs[p] == 1 {
			unsubs = append(unsubs, p)
			delete(globalSubs, p)
		} else {
			globalSubs[p]--
		}
	}
	globalSubsMu.Unlock()

	if len(unsubs) > 0 {
		data, _ := json.Marshal(WSMessage{
			Method: "UNSUBSCRIBE",
			Params: unsubs,
		})
		rdb.Publish(ctx, "binance-events", string(data))
	}

	log.Printf("Client disconnected. Cleaned subscriptions: %v", unsubs)
}

func redisListener() {
	sub := rdb.PSubscribe(ctx, "*")
	defer sub.Close()

	ch := sub.Channel()

	log.Println("Listening Redis → WS...")

	for msg := range ch {
		broadcastProtobuf(msg.Channel, &commonPb.Balance{
			Asset:     "BTC",
			Available: "YES",
			Locked:    "TRUE",
		})
		broadcast(msg.Channel, msg.Payload)
		broadcastMsgPack(msg.Channel, msg.Payload)
		broadcastGzipMsgPack(msg.Channel, msg.Payload)
	}
}

func broadcast(stream string, payload string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	for conn, subs := range clients {
		if subs[stream] {
			if err := conn.WriteMessage(websocket.BinaryMessage, []byte(payload)); err != nil { // websocket.TextMessage || websocket.BinaryMessage
				log.Println("WS write error:", err)
				conn.Close()
				delete(clients, conn)
			}
		}
	}
}

func broadcastProtobuf(stream string, msg proto.Message) {
	// 1) Marshal to protobuf binary
	bin, err := proto.Marshal(msg)
	if err != nil {
		log.Println("Protobuf marshal error:", err)
		return
	}

	// 2) Broadcast to subscribed clients
	clientsMu.Lock()
	defer clientsMu.Unlock()

	for conn, subs := range clients {
		if subs[stream] {
			if err := conn.WriteMessage(websocket.BinaryMessage, bin); err != nil {
				log.Println("WS write error:", err)
				conn.Close()
				delete(clients, conn)
			}
		}
	}
}

func broadcastMsgPack(stream string, payload string) {
	// Decode JSON string -> map
	var data any
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		log.Println("Invalid JSON payload:", err)
		return
	}

	// Encode msgpack
	bin, err := msgpack.Marshal(data)
	if err != nil {
		log.Println("msgpack encode error:", err)
		return
	}

	clientsMu.Lock()
	defer clientsMu.Unlock()

	for conn, subs := range clients {
		if subs[stream] {
			if err := conn.WriteMessage(websocket.BinaryMessage, bin); err != nil {
				log.Println("WS write error:", err)
				conn.Close()
				delete(clients, conn)
			}
		}
	}
}

func broadcastGzipMsgPack(stream string, payload string) {
	data, err := toGzipMsgPack(payload)
	if err != nil {
		log.Println("encode error:", err)
		return
	}

	clientsMu.Lock()
	defer clientsMu.Unlock()

	for conn, subs := range clients {
		if subs[stream] {
			if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				log.Println("WS write error:", err)
				conn.Close()
				delete(clients, conn)
			}
		}
	}
}

func toGzipMsgPack(payload string) ([]byte, error) {
	// 1) JSON → map
	var data any
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return nil, err
	}

	// 2) map → msgpack
	bin, err := msgpack.Marshal(data)
	if err != nil {
		return nil, err
	}

	// 3) msgpack → gzip(msgpack)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err = zw.Write(bin)
	if err != nil {
		return nil, err
	}
	zw.Close()

	return buf.Bytes(), nil
}
