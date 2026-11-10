package marketdata

import (
	"context"
	"sort"
	"sync"

	"go.uber.org/zap"

	"simple-securities/pkg/events"
	"simple-securities/pkg/kafka"
)

// Cache holds the latest depth snapshots and tickers in memory (fed by tail consumers).
type Cache struct {
	log     *zap.Logger
	mu      sync.RWMutex
	depth   map[string]events.DepthSnapshot
	tickers map[string]events.Ticker
}

// NewCache returns an empty cache.
func NewCache(log *zap.Logger) *Cache {
	return &Cache{log: log, depth: map[string]events.DepthSnapshot{}, tickers: map[string]events.Ticker{}}
}

// HandleTail is the TailConsumer handler for market.depth and market.ticker.
func (c *Cache) HandleTail(_ context.Context, m kafka.Message) {
	switch m.Envelope.Type {
	case events.TypeDepthSnapshot:
		var d events.DepthSnapshot
		if err := m.Envelope.Decode(&d); err != nil || d.Symbol == "" {
			c.log.Warn("bad depth snapshot", zap.Error(err))
			return
		}
		c.SetDepth(d)
	case events.TypeTicker:
		var t events.Ticker
		if err := m.Envelope.Decode(&t); err != nil || t.Symbol == "" {
			c.log.Warn("bad ticker", zap.Error(err))
			return
		}
		c.SetTicker(t)
	}
}

// SetDepth stores a snapshot unless an equal-or-newer one is cached.
func (c *Cache) SetDepth(d events.DepthSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cur, ok := c.depth[d.Symbol]; ok && cur.LastUpdateID > d.LastUpdateID && d.LastUpdateID != 0 {
		return
	}
	c.depth[d.Symbol] = d
}

// SetTicker stores a ticker unless a newer one is cached.
func (c *Cache) SetTicker(t events.Ticker) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cur, ok := c.tickers[t.Symbol]; ok && cur.CloseTime > t.CloseTime {
		return
	}
	c.tickers[t.Symbol] = t
}

// Depth returns the cached snapshot for symbol.
func (c *Cache) Depth(symbol string) (events.DepthSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	d, ok := c.depth[symbol]
	return d, ok
}

// Ticker returns the cached ticker for symbol.
func (c *Cache) Ticker(symbol string) (events.Ticker, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, ok := c.tickers[symbol]
	return t, ok
}

// Tickers returns every cached ticker sorted by symbol.
func (c *Cache) Tickers() []events.Ticker {
	c.mu.RLock()
	out := make([]events.Ticker, 0, len(c.tickers))
	for _, t := range c.tickers {
		out = append(out, t)
	}
	c.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}
