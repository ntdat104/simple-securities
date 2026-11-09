package marketdata

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"simple-securities/pkg/events"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/symbols"
)

// ServiceName is the spot service name (consumer group, inbox consumer, event source).
const ServiceName = "marketdata"

// ServiceNameFor returns the service name of a market: "marketdata" (spot) or "marketdata-futures".
func ServiceNameFor(market string) string {
	if market == symbols.MarketFutures {
		return ServiceName + "-futures"
	}
	return ServiceName
}

const (
	tickerWindow       = 24 * time.Hour
	dirtyTickerPeriod  = time.Second
	fullTickerPeriod   = 10 * time.Second
	klineThrottlePause = 250 * time.Millisecond
	klineFlushPeriod   = 50 * time.Millisecond
)

// Processor consumes trades, maintains public trades/klines and publishes tickers and klines.
type Processor struct {
	market   string
	source   string
	topics   events.MarketTopics
	store    *Store
	prod     *kafka.Producer
	cache    *Cache
	log      *zap.Logger
	throttle *klineThrottle

	mu    sync.Mutex
	dirty map[string]struct{}
}

// NewProcessor wires a Processor for a market (symbols.MarketSpot | symbols.MarketFutures).
func NewProcessor(market string, store *Store, prod *kafka.Producer, cache *Cache, log *zap.Logger) *Processor {
	return &Processor{market: market, source: ServiceNameFor(market), topics: events.TopicsFor(market),
		store: store, prod: prod, cache: cache, log: log,
		throttle: newKlineThrottle(klineThrottlePause), dirty: map[string]struct{}{}}
}

// HandleBatch is the GroupConsumer handler for matching.events (one partition, in order).
func (p *Processor) HandleBatch(ctx context.Context, msgs []kafka.Message) error {
	tms := make([]tradeMsg, 0, len(msgs))
	for _, m := range msgs {
		if m.Envelope.Type != events.TypeTradeExecuted {
			continue
		}
		var te events.TradeExecuted
		if err := m.Envelope.Decode(&te); err != nil {
			p.log.Error("undecodable trade, skipped", zap.String("event_id", m.Envelope.ID), zap.Error(err))
			continue
		}
		t, err := parseTrade(te)
		if err != nil {
			p.log.Error("invalid trade, skipped", zap.String("event_id", m.Envelope.ID), zap.Error(err))
			continue
		}
		tms = append(tms, tradeMsg{EventID: m.Envelope.ID, EventType: m.Envelope.Type, Trade: t})
	}
	if len(tms) == 0 {
		return nil
	}
	buckets, err := p.store.ApplyTrades(ctx, tms)
	if err != nil {
		return err
	}
	p.mu.Lock()
	for _, b := range buckets {
		p.dirty[b.Symbol] = struct{}{}
	}
	p.mu.Unlock()
	now := time.Now()
	for _, b := range buckets {
		if k := b.toEvent(); p.throttle.offer(k, now) {
			p.publishKline(ctx, k)
		}
	}
	return nil
}

// Run drives the ticker and kline publishers until ctx is cancelled. It starts with a full
// ticker computation that also warms the cache.
func (p *Processor) Run(ctx context.Context) error {
	p.publishTickers(ctx, allSymbols(p.market))
	dirtyT := time.NewTicker(dirtyTickerPeriod)
	fullT := time.NewTicker(fullTickerPeriod)
	flushT := time.NewTicker(klineFlushPeriod)
	defer dirtyT.Stop()
	defer fullT.Stop()
	defer flushT.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-dirtyT.C:
			if syms := p.takeDirty(); len(syms) > 0 {
				p.publishTickers(ctx, syms)
			}
		case <-fullT.C:
			p.takeDirty()
			p.publishTickers(ctx, allSymbols(p.market))
		case now := <-flushT.C:
			for _, k := range p.throttle.due(now) {
				p.publishKline(ctx, k)
			}
		}
	}
}

func (p *Processor) takeDirty() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, 0, len(p.dirty))
	for s := range p.dirty {
		out = append(out, s)
	}
	clear(p.dirty)
	return out
}

func (p *Processor) publishTickers(ctx context.Context, syms []string) {
	now := time.Now()
	stats, err := p.store.TickerStats(ctx, syms, now.Add(-tickerWindow))
	if err != nil {
		if ctx.Err() == nil {
			p.log.Warn("compute tickers", zap.Error(err))
			p.mu.Lock()
			for _, s := range syms {
				p.dirty[s] = struct{}{} // retry next tick
			}
			p.mu.Unlock()
		}
		return
	}
	for sym, st := range stats {
		t := buildTicker(sym, st, now)
		p.cache.SetTicker(t)
		env, err := events.New(p.source, events.TypeTicker, sym, t)
		if err != nil {
			p.log.Error("encode ticker", zap.Error(err))
			continue
		}
		p.prod.PublishAsync(ctx, p.topics.Ticker, sym, env)
	}
}

func (p *Processor) publishKline(ctx context.Context, k events.Kline) {
	env, err := events.New(p.source, events.TypeKline, k.Symbol, k)
	if err != nil {
		p.log.Error("encode kline", zap.Error(err))
		return
	}
	p.prod.PublishAsync(ctx, p.topics.Kline, k.Symbol, env)
}

func allSymbols(market string) []string {
	all := symbols.AllMarket(market)
	out := make([]string, len(all))
	for i, s := range all {
		out[i] = s.Name
	}
	return out
}
