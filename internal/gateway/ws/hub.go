// Package ws implements the gateway's WebSocket endpoint: Binance-style market streams fed by a
// Kafka tail consumer, user data streams and a small WS API for trading.
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"

	"simple-securities/internal/gateway/api"
	"simple-securities/pkg/apperr"
	"simple-securities/pkg/auth"
	"simple-securities/pkg/events"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/symbols"
)

const (
	// MaxStreams is the per-connection subscription limit.
	MaxStreams = 100

	tickerArrEvery  = time.Second
	shutdownTimeout = 3 * time.Second
)

// Topics are the Kafka topics the hub tail-consumes (spot, then futures).
var Topics = []string{
	events.TopicMatchingEvents,
	events.TopicMarketDepth,
	events.TopicMarketTicker,
	events.TopicMarketKline,
	events.TopicOrderUpdates,
	events.TopicWalletEvents,
	events.TopicsFor(symbols.MarketFutures).MatchingEvents,
	events.TopicsFor(symbols.MarketFutures).Depth,
	events.TopicsFor(symbols.MarketFutures).Ticker,
	events.TopicsFor(symbols.MarketFutures).Kline,
	events.TopicFuturesOrderUpdates,
	events.TopicFuturesAccountEvents,
	events.TopicFuturesMarkPrice,
}

// _futuresTopics are the topics whose events belong to the futures market.
var _futuresTopics = map[string]bool{
	events.TopicsFor(symbols.MarketFutures).MatchingEvents: true,
	events.TopicsFor(symbols.MarketFutures).Depth:          true,
	events.TopicsFor(symbols.MarketFutures).Ticker:         true,
	events.TopicsFor(symbols.MarketFutures).Kline:          true,
	events.TopicFuturesOrderUpdates:                        true,
	events.TopicFuturesAccountEvents:                       true,
	events.TopicFuturesMarkPrice:                           true,
}

// marketCache holds the latest depth and tickers of one market (guarded by Hub.cacheMu).
type marketCache struct {
	futures      bool
	depth        map[string]events.DepthSnapshot // symbol -> latest snapshot
	tickers      map[string]tickerMsg            // symbol -> latest ticker
	tickersDirty bool
}

func newMarketCache(futures bool) *marketCache {
	return &marketCache{futures: futures, depth: make(map[string]events.DepthSnapshot),
		tickers: make(map[string]tickerMsg)}
}

var (
	_activeConns = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gateway_ws_connections", Help: "Active WebSocket connections.",
	})
	_droppedClients = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gateway_ws_dropped_clients_total", Help: "WebSocket clients dropped as slow consumers.",
	})
)

var errShuttingDown = errors.New("shutting down")

// Verifier verifies access tokens.
type Verifier interface {
	Verify(token string) (*auth.Claims, error)
}

var _ http.Handler = (*Hub)(nil)

// Hub fans out Kafka events to WebSocket clients.
type Hub struct {
	log      *zap.Logger
	trader   *api.Trader
	verifier Verifier
	upgrader websocket.Upgrader

	mu      sync.RWMutex
	clients map[*client]struct{}
	streams map[string]map[*client]struct{} // stream -> subscribers
	users   map[string]map[*client]struct{} // user id -> logged-in connections
	closed  bool
	wg      sync.WaitGroup // writer goroutines

	cacheMu    sync.RWMutex
	spot       *marketCache
	fut        *marketCache
	markPrices map[string]markPriceMsg // futures symbol -> latest mark price
	marksDirty bool
}

// NewHub returns a hub. origins is the CORS allow-list ("*" allows any origin).
func NewHub(trader *api.Trader, verifier Verifier, origins []string, log *zap.Logger) *Hub {
	return &Hub{
		log:      log.Named("ws"),
		trader:   trader,
		verifier: verifier,
		upgrader: websocket.Upgrader{
			HandshakeTimeout:  10 * time.Second,
			ReadBufferSize:    4096,
			WriteBufferSize:   4096,
			WriteBufferPool:   &sync.Pool{},
			EnableCompression: true,
			CheckOrigin:       originChecker(origins),
		},
		clients: make(map[*client]struct{}),
		streams: make(map[string]map[*client]struct{}),
		users:   make(map[string]map[*client]struct{}),
		spot:    newMarketCache(false),
		fut:     newMarketCache(true),

		markPrices: make(map[string]markPriceMsg),
	}
}

func originChecker(origins []string) func(*http.Request) bool {
	if len(origins) == 0 || slices.Contains(origins, "*") {
		return func(*http.Request) bool { return true }
	}
	return func(r *http.Request) bool {
		o := r.Header.Get("Origin")
		if o == "" || slices.Contains(origins, o) {
			return true // non-browser clients send no Origin
		}
		u, err := url.Parse(o)
		return err == nil && strings.EqualFold(u.Host, r.Host)
	}
}

// Run broadcasts the aggregated ticker arrays (spot, futures) and mark price array at most once per
// second and, when ctx ends, closes every connection with 1001 (going away).
func (h *Hub) Run(ctx context.Context) error {
	t := time.NewTicker(tickerArrEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			h.shutdown()
			return nil
		case <-t.C:
			h.flushTickers(h.spot)
			h.flushTickers(h.fut)
			h.flushMarkPrices()
		}
	}
}

func (h *Hub) shutdown() {
	h.mu.Lock()
	h.closed = true
	for c := range h.clients {
		c.close(websocket.CloseGoingAway, "server shutdown")
	}
	h.mu.Unlock()

	done := make(chan struct{})
	go func() { h.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(shutdownTimeout):
		h.log.Warn("timed out waiting for websocket writers")
	}
}

// HandleMessage is the TailConsumer handler. Spot and futures share event types; the topic tells
// the market apart.
func (h *Hub) HandleMessage(_ context.Context, m kafka.Message) {
	env := m.Envelope
	futures := _futuresTopics[m.Topic]
	mc := h.spot
	if futures {
		mc = h.fut
	}
	var err error
	switch env.Type {
	case events.TypeTradeExecuted:
		var t events.TradeExecuted
		if err = env.Decode(&t); err == nil {
			h.broadcast(marketStreamName(futures, t.Symbol, KindTrade), tradeFrom(t))
		}
	case events.TypeDepthSnapshot:
		var d events.DepthSnapshot
		if err = env.Decode(&d); err == nil {
			h.onDepth(mc, d)
		}
	case events.TypeTicker:
		var t events.Ticker
		if err = env.Decode(&t); err == nil {
			h.onTicker(mc, t)
		}
	case events.TypeKline:
		var k events.Kline
		if err = env.Decode(&k); err == nil {
			h.broadcast(marketStreamName(futures, k.Symbol, "kline_"+k.Interval), klineFrom(k))
		}
	case events.TypeOrderUpdated:
		var o events.OrderUpdated
		if err = env.Decode(&o); err == nil {
			if futures {
				h.sendUser(o.UserID, StreamOrderTradeUpdate, orderTradeUpdateFrom(o))
			} else {
				h.sendUser(o.UserID, StreamExecutionReport, executionReportFrom(o))
			}
		}
	case events.TypeFuturesAccountUpdate:
		var a events.FuturesAccountUpdate
		if err = env.Decode(&a); err == nil {
			h.sendUser(a.UserID, StreamAccountUpdate, accountUpdateFrom(a))
		}
	case events.TypeMarkPrice:
		var mp events.MarkPrice
		if err = env.Decode(&mp); err == nil {
			h.onMarkPrice(mp)
		}
	case events.TypeBalanceUpdated:
		var b events.BalanceUpdated
		if err = env.Decode(&b); err == nil {
			h.sendUser(b.UserID, StreamAccountPosition, accountPositionFrom(b))
		}
	}
	if err != nil {
		h.log.Warn("undecodable event", zap.String("type", env.Type), zap.String("topic", m.Topic), zap.Error(err))
	}
}

func (h *Hub) onDepth(mc *marketCache, d events.DepthSnapshot) {
	h.cacheMu.Lock()
	if prev, ok := mc.depth[d.Symbol]; ok && prev.LastUpdateID > d.LastUpdateID {
		h.cacheMu.Unlock()
		return // stale snapshot
	}
	mc.depth[d.Symbol] = d
	h.cacheMu.Unlock()
	for name, lv := range _depthLevels {
		h.broadcast(marketStreamName(mc.futures, d.Symbol, name), depthFrom(d, lv))
	}
}

func (h *Hub) onTicker(mc *marketCache, t events.Ticker) {
	msg := tickerFrom(t)
	h.cacheMu.Lock()
	mc.tickers[t.Symbol] = msg
	mc.tickersDirty = true
	h.cacheMu.Unlock()
	h.broadcast(marketStreamName(mc.futures, t.Symbol, KindTicker), msg)
}

func (h *Hub) onMarkPrice(m events.MarkPrice) {
	msg := markPriceFrom(m)
	h.cacheMu.Lock()
	if prev, ok := h.markPrices[m.Symbol]; ok && prev.Time > msg.Time {
		h.cacheMu.Unlock()
		return // stale
	}
	h.markPrices[m.Symbol] = msg
	h.marksDirty = true
	h.cacheMu.Unlock()
	h.broadcast(marketStreamName(true, m.Symbol, markPriceChannel), msg)
}

func (h *Hub) flushTickers(mc *marketCache) {
	h.cacheMu.Lock()
	if !mc.tickersDirty {
		h.cacheMu.Unlock()
		return
	}
	mc.tickersDirty = false
	h.cacheMu.Unlock()
	h.broadcast(tickerArrName(mc.futures), h.tickerArray(mc))
}

func (h *Hub) flushMarkPrices() {
	h.cacheMu.Lock()
	if !h.marksDirty {
		h.cacheMu.Unlock()
		return
	}
	h.marksDirty = false
	h.cacheMu.Unlock()
	h.broadcast(FuturesPrefix+AllMarkPrices, h.markPriceArray())
}

func (h *Hub) tickerArray(mc *marketCache) []tickerMsg {
	h.cacheMu.RLock()
	out := make([]tickerMsg, 0, len(mc.tickers))
	for _, t := range mc.tickers {
		out = append(out, t)
	}
	h.cacheMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}

func (h *Hub) markPriceArray() []markPriceMsg {
	h.cacheMu.RLock()
	out := make([]markPriceMsg, 0, len(h.markPrices))
	for _, m := range h.markPrices {
		out = append(out, m)
	}
	h.cacheMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}

// snapshot returns the cached payload for a freshly subscribed stream, if any.
func (h *Hub) snapshot(s Stream) (any, bool) {
	mc := h.spot
	if s.Futures {
		mc = h.fut
	}
	switch s.Kind {
	case KindDepth:
		h.cacheMu.RLock()
		d, ok := mc.depth[s.Symbol]
		h.cacheMu.RUnlock()
		if ok {
			return depthFrom(d, s.Levels), true
		}
	case KindTicker:
		h.cacheMu.RLock()
		t, ok := mc.tickers[s.Symbol]
		h.cacheMu.RUnlock()
		if ok {
			return t, true
		}
	case KindTickerArr:
		if arr := h.tickerArray(mc); len(arr) > 0 {
			return arr, true
		}
	case KindMarkPrice:
		h.cacheMu.RLock()
		m, ok := h.markPrices[s.Symbol]
		h.cacheMu.RUnlock()
		if ok {
			return m, true
		}
	case KindMarkPriceArr:
		if arr := h.markPriceArray(); len(arr) > 0 {
			return arr, true
		}
	}
	return nil, false
}

// broadcast serializes the push once and fans it out to the stream's subscribers.
func (h *Hub) broadcast(stream string, data any) {
	h.mu.RLock()
	targets := collect(h.streams[stream])
	h.mu.RUnlock()
	h.fanout(targets, stream, data)
}

func (h *Hub) sendUser(userID, stream string, data any) {
	if userID == "" {
		return
	}
	h.mu.RLock()
	targets := collect(h.users[userID])
	h.mu.RUnlock()
	h.fanout(targets, stream, data)
}

func collect(set map[*client]struct{}) []*client {
	if len(set) == 0 {
		return nil
	}
	out := make([]*client, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	return out
}

func (h *Hub) fanout(targets []*client, stream string, data any) {
	if len(targets) == 0 {
		return
	}
	pm, err := prepare(push{Stream: stream, Data: data})
	if err != nil {
		h.log.Error("encode push", zap.String("stream", stream), zap.Error(err))
		return
	}
	for _, c := range targets {
		c.send(pm)
	}
}

func prepare(v any) (*websocket.PreparedMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return websocket.NewPreparedMessage(websocket.TextMessage, b)
}

// ---------------------------------------------------------------- registry

func (h *Hub) register(c *client) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return errShuttingDown
	}
	h.clients[c] = struct{}{}
	h.wg.Add(1)
	_activeConns.Inc()
	return nil
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; !ok {
		return
	}
	delete(h.clients, c)
	for name := range c.streams {
		removeFrom(h.streams, name, c)
	}
	c.streams = nil
	if c.userID != "" {
		removeFrom(h.users, c.userID, c)
	}
	_activeConns.Dec()
}

func removeFrom(m map[string]map[*client]struct{}, key string, c *client) {
	set := m[key]
	delete(set, c)
	if len(set) == 0 {
		delete(m, key)
	}
}

func addTo(m map[string]map[*client]struct{}, key string, c *client) {
	set, ok := m[key]
	if !ok {
		set = make(map[*client]struct{})
		m[key] = set
	}
	set[c] = struct{}{}
}

// subscribe adds all streams or none (when the per-connection limit would be exceeded).
func (h *Hub) subscribe(c *client, streams []Stream) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	added := 0
	for _, s := range streams {
		if _, ok := c.streams[s.Name]; !ok {
			added++
		}
	}
	if len(c.streams)+added > MaxStreams {
		return apperr.Invalid("TOO_MANY_STREAMS", "at most %d streams per connection", MaxStreams)
	}
	for _, s := range streams {
		c.streams[s.Name] = struct{}{}
		addTo(h.streams, s.Name, c)
	}
	return nil
}

func (h *Hub) unsubscribe(c *client, streams []Stream) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range streams {
		if _, ok := c.streams[s.Name]; ok {
			delete(c.streams, s.Name)
			removeFrom(h.streams, s.Name, c)
		}
	}
}

func (h *Hub) subscriptions(c *client) []string {
	h.mu.RLock()
	out := make([]string, 0, len(c.streams))
	for name := range c.streams {
		out = append(out, name)
	}
	h.mu.RUnlock()
	sort.Strings(out)
	return out
}

// login binds the connection to userID (re-binding when another user logs in on it).
func (h *Hub) login(c *client, userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.userID != "" {
		removeFrom(h.users, c.userID, c)
	}
	c.userID = userID
	addTo(h.users, userID, c)
}

func (h *Hub) user(c *client) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return c.userID
}
