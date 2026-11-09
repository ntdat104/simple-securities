package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"simple-securities/internal/gateway/api"
	"simple-securities/pkg/apperr"
)

const (
	sendBuffer     = 256
	writeTimeout   = 10 * time.Second
	readTimeout    = 60 * time.Second
	pingPeriod     = 20 * time.Second
	maxMessageSize = 64 << 10
	closeGrace     = time.Second
)

// client is one WebSocket connection.
type client struct {
	hub    *Hub
	conn   *websocket.Conn
	log    *zap.Logger
	out    chan *websocket.PreparedMessage
	done   chan struct{}
	ctx    context.Context // cancelled when the connection closes
	cancel context.CancelFunc

	closeOnce sync.Once
	closeCode int
	closeText string

	// Guarded by hub.mu.
	streams map[string]struct{}
	userID  string

	// Owned by the reader goroutine.
	tokenExp time.Time
}

// ServeHTTP upgrades the request and serves the connection until it closes.
// Optional ?streams=a/b/c subscribes at connect time.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var initial []Stream
	if q := r.URL.Query().Get("streams"); q != "" {
		var err error
		if initial, err = parseStreams(strings.Split(q, "/")); err != nil {
			api.WriteError(w, err)
			return
		}
		if len(initial) > MaxStreams {
			api.WriteError(w, apperr.Invalid("TOO_MANY_STREAMS", "at most %d streams per connection", MaxStreams))
			return
		}
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // the upgrader already replied
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &client{
		hub: h, conn: conn, log: h.log, ctx: ctx, cancel: cancel,
		out:     make(chan *websocket.PreparedMessage, sendBuffer),
		done:    make(chan struct{}),
		streams: make(map[string]struct{}),
	}
	if err := h.register(c); err != nil {
		cancel()
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutdown"), time.Now().Add(closeGrace))
		_ = conn.Close()
		return
	}
	go c.writeLoop()
	if len(initial) > 0 {
		if err := h.subscribe(c, initial); err == nil {
			c.sendSnapshots(initial)
		}
	}
	c.readLoop()
	h.unregister(c)
	c.close(websocket.CloseNormalClosure, "")
}

func parseStreams(names []string) ([]Stream, error) {
	if len(names) == 0 {
		return nil, apperr.Invalid("INVALID_STREAM", "no streams given")
	}
	out := make([]Stream, 0, len(names))
	for _, n := range names {
		s, err := ParseStream(n)
		if err != nil {
			return nil, apperr.Invalid("INVALID_STREAM", "%s", err.Error())
		}
		out = append(out, s)
	}
	return out, nil
}

// close asks the writer to send a close frame and terminate the connection. Idempotent.
func (c *client) close(code int, text string) {
	c.closeOnce.Do(func() {
		c.closeCode, c.closeText = code, text
		close(c.done)
		c.cancel()
	})
}

// send enqueues without blocking; a full buffer drops the client (slow consumer).
func (c *client) send(pm *websocket.PreparedMessage) {
	select {
	case <-c.done:
		return
	default:
	}
	select {
	case c.out <- pm:
	default:
		_droppedClients.Inc()
		c.log.Info("dropping slow websocket consumer")
		c.close(websocket.ClosePolicyViolation, "slow consumer")
	}
}

func (c *client) sendJSON(v any) {
	pm, err := prepare(v)
	if err != nil {
		c.log.Error("encode ws message", zap.Error(err))
		return
	}
	c.send(pm)
}

func (c *client) writeLoop() {
	defer c.hub.wg.Done()
	defer c.conn.Close()
	ping := time.NewTicker(pingPeriod)
	defer ping.Stop()
	for {
		select {
		case <-c.done:
			_ = c.conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(c.closeCode, c.closeText), time.Now().Add(closeGrace))
			return
		case pm := <-c.out:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.conn.WritePreparedMessage(pm); err != nil {
				c.close(websocket.CloseAbnormalClosure, "")
				return
			}
		case <-ping.C:
			if err := c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout)); err != nil {
				c.close(websocket.CloseAbnormalClosure, "")
				return
			}
		}
	}
}

func (c *client) readLoop() {
	c.conn.SetReadLimit(maxMessageSize)
	extend := func() error { return c.conn.SetReadDeadline(time.Now().Add(readTimeout)) }
	_ = extend()
	c.conn.SetPongHandler(func(string) error { return extend() })
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		_ = extend()
		c.handle(msg)
	}
}

func (c *client) handle(msg []byte) {
	var req request
	if err := json.Unmarshal(msg, &req); err != nil {
		c.replyErr(nil, apperr.Invalid("BAD_REQUEST", "invalid JSON request"))
		return
	}
	switch strings.ToUpper(req.Method) {
	case "SUBSCRIBE":
		streams, err := c.streamParams(req.Params)
		if err == nil {
			err = c.hub.subscribe(c, streams)
		}
		if err != nil {
			c.replyErr(req.ID, err)
			return
		}
		c.reply(req.ID, nil)
		c.sendSnapshots(streams)
	case "UNSUBSCRIBE":
		streams, err := c.streamParams(req.Params)
		if err != nil {
			c.replyErr(req.ID, err)
			return
		}
		c.hub.unsubscribe(c, streams)
		c.reply(req.ID, nil)
	case "LIST_SUBSCRIPTIONS":
		c.reply(req.ID, c.hub.subscriptions(c))
	case "PING":
		c.reply(req.ID, "pong")
	case "LOGIN":
		c.login(req)
	case "ORDER.PLACE":
		c.placeOrder(req)
	case "ORDER.CANCEL":
		c.cancelOrder(req)
	case "FUTURES.ORDER.PLACE":
		c.placeFuturesOrder(req)
	case "FUTURES.ORDER.CANCEL":
		c.cancelFuturesOrder(req)
	default:
		c.replyErr(req.ID, apperr.Invalid("UNKNOWN_METHOD", "unknown method %q", req.Method))
	}
}

func (c *client) streamParams(raw json.RawMessage) ([]Stream, error) {
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		return nil, apperr.Invalid("INVALID_PARAMS", "params must be an array of stream names")
	}
	return parseStreams(names)
}

func (c *client) sendSnapshots(streams []Stream) {
	for _, s := range streams {
		if data, ok := c.hub.snapshot(s); ok {
			c.sendJSON(push{Stream: s.Name, Data: data})
		}
	}
}

func (c *client) login(req request) {
	var p struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil || p.Token == "" {
		c.replyErr(req.ID, apperr.Invalid("INVALID_PARAMS", "params.token is required"))
		return
	}
	claims, err := c.hub.verifier.Verify(p.Token)
	if err != nil {
		c.replyErr(req.ID, apperr.Unauthenticated("INVALID_TOKEN", "invalid or expired token"))
		return
	}
	if claims.ExpiresAt != nil {
		c.tokenExp = claims.ExpiresAt.Time
	}
	c.hub.login(c, claims.Subject)
	c.reply(req.ID, map[string]string{"userId": claims.Subject})
}

// authedUser returns the logged-in user whose token is still valid.
func (c *client) authedUser() (string, error) {
	uid := c.hub.user(c)
	if uid == "" {
		return "", apperr.Unauthenticated("UNAUTHENTICATED", "LOGIN required")
	}
	if !c.tokenExp.IsZero() && time.Now().After(c.tokenExp) {
		return "", apperr.Unauthenticated("TOKEN_EXPIRED", "access token expired, LOGIN again")
	}
	return uid, nil
}

func (c *client) placeOrder(req request) {
	uid, err := c.authedUser()
	if err != nil {
		c.replyErr(req.ID, err)
		return
	}
	var p api.PlaceOrderRequest
	if err := json.Unmarshal(req.Params, &p); err != nil {
		c.replyErr(req.ID, apperr.Invalid("INVALID_PARAMS", "invalid order params"))
		return
	}
	o, err := c.hub.trader.Place(c.ctx, uid, p)
	if err != nil {
		c.replyErr(req.ID, err)
		return
	}
	c.reply(req.ID, o)
}

func (c *client) cancelOrder(req request) {
	uid, err := c.authedUser()
	if err != nil {
		c.replyErr(req.ID, err)
		return
	}
	var p api.CancelOrderRequest
	if err := json.Unmarshal(req.Params, &p); err != nil {
		c.replyErr(req.ID, apperr.Invalid("INVALID_PARAMS", "invalid cancel params"))
		return
	}
	o, err := c.hub.trader.Cancel(c.ctx, uid, p.OrderID)
	if err != nil {
		c.replyErr(req.ID, err)
		return
	}
	c.reply(req.ID, o)
}

func (c *client) placeFuturesOrder(req request) {
	uid, err := c.authedUser()
	if err != nil {
		c.replyErr(req.ID, err)
		return
	}
	var p api.FuturesPlaceOrderRequest
	if err := json.Unmarshal(req.Params, &p); err != nil {
		c.replyErr(req.ID, apperr.Invalid("INVALID_PARAMS", "invalid order params"))
		return
	}
	o, err := c.hub.trader.PlaceFutures(c.ctx, uid, p)
	if err != nil {
		c.replyErr(req.ID, err)
		return
	}
	c.reply(req.ID, o)
}

func (c *client) cancelFuturesOrder(req request) {
	uid, err := c.authedUser()
	if err != nil {
		c.replyErr(req.ID, err)
		return
	}
	var p api.CancelOrderRequest
	if err := json.Unmarshal(req.Params, &p); err != nil {
		c.replyErr(req.ID, apperr.Invalid("INVALID_PARAMS", "invalid cancel params"))
		return
	}
	o, err := c.hub.trader.CancelFutures(c.ctx, uid, p.OrderID)
	if err != nil {
		c.replyErr(req.ID, err)
		return
	}
	c.reply(req.ID, o)
}

func (c *client) reply(id json.RawMessage, result any) {
	c.sendJSON(response{ID: id, Result: result})
}

func (c *client) replyErr(id json.RawMessage, err error) {
	if api.Internal(err) && !errors.Is(err, context.Canceled) {
		c.log.Warn("ws request failed", zap.Error(err))
	}
	_, body := api.FromError(err)
	c.sendJSON(errorResponse{ID: id, Error: body})
}
