package futures

import (
	"context"
	"sync"

	"github.com/shopspring/decimal"

	"simple-securities/pkg/events"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/symbols"
)

// MarkInfo is the latest mark price state of a contract.
type MarkInfo struct {
	Symbol          string
	Mark            decimal.Decimal
	Index           decimal.Decimal
	FundingRate     decimal.Decimal // estimated rate of the next settlement
	NextFundingTime int64           // epoch ms
	Time            int64           // epoch ms
}

// Prices is the in-memory price state used by the risk hot path (RWMutex protected). Every
// instance tail-consumes the mark price topic (published by the leader) and the raw price feeds,
// so mark → index → last fallbacks work on followers too.
type Prices struct {
	mu       sync.RWMutex
	marks    map[string]MarkInfo        // futures symbol
	spotMid  map[string]decimal.Decimal // spot symbol
	spotLast map[string]decimal.Decimal // spot symbol
	futMid   map[string]decimal.Decimal // futures symbol
	futLast  map[string]decimal.Decimal // futures symbol
}

// NewPrices returns an empty price cache.
func NewPrices() *Prices {
	return &Prices{
		marks:    map[string]MarkInfo{},
		spotMid:  map[string]decimal.Decimal{},
		spotLast: map[string]decimal.Decimal{},
		futMid:   map[string]decimal.Decimal{},
		futLast:  map[string]decimal.Decimal{},
	}
}

// FeedTopics are the topics Prices.Handle understands.
func FeedTopics() []string {
	return []string{events.TopicFuturesMarkPrice, events.TopicMarketDepth, events.TopicMarketTicker,
		events.TopicFuturesMarketDepth, events.TopicFuturesMatchingEvents}
}

// SetMark stores a mark price unless a newer one is already cached.
func (p *Prices) SetMark(m MarkInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cur, ok := p.marks[m.Symbol]; ok && cur.Time > m.Time {
		return
	}
	p.marks[m.Symbol] = m
}

// Mark returns the cached mark price state.
func (p *Prices) Mark(symbol string) (MarkInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	m, ok := p.marks[symbol]
	return m, ok
}

// Marks returns every cached mark sorted by symbol.
func (p *Prices) Marks() []MarkInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]MarkInfo, 0, len(p.marks))
	for _, s := range symbols.AllMarket(symbols.MarketFutures) {
		if m, ok := p.marks[s.Name]; ok {
			out = append(out, m)
		}
	}
	return out
}

// Price returns the risk price of a contract: mark, else index, else futures last trade.
func (p *Prices) Price(symbol string) (decimal.Decimal, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if m, ok := p.marks[symbol]; ok && m.Mark.IsPositive() {
		return m.Mark, true
	}
	if sym, ok := symbols.GetMarket(symbols.MarketFutures, symbol); ok {
		if idx, ok := p.indexLocked(sym); ok {
			return idx, true
		}
	}
	last, ok := p.futLast[symbol]
	return last, ok
}

// Index returns the index price (spot mid, else spot last) of a contract.
func (p *Prices) Index(sym symbols.Symbol) (decimal.Decimal, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.indexLocked(sym)
}

func (p *Prices) indexLocked(sym symbols.Symbol) (decimal.Decimal, bool) {
	if v, ok := p.spotMid[sym.Underlying]; ok {
		return v, true
	}
	v, ok := p.spotLast[sym.Underlying]
	return v, ok
}

// futures returns the futures mid and last trade price of a contract.
func (p *Prices) futures(symbol string) (mid decimal.Decimal, hasMid bool, last decimal.Decimal, hasLast bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	mid, hasMid = p.futMid[symbol]
	last, hasLast = p.futLast[symbol]
	return mid, hasMid, last, hasLast
}

// Handle is the TailConsumer handler for FeedTopics.
func (p *Prices) Handle(_ context.Context, m kafka.Message) {
	env := m.Envelope
	switch m.Topic {
	case events.TopicFuturesMarkPrice:
		var ev events.MarkPrice
		if env.Type != events.TypeMarkPrice || env.Decode(&ev) != nil {
			return
		}
		if info, ok := markFromEvent(ev); ok {
			p.SetMark(info)
		}
	case events.TopicMarketDepth, events.TopicFuturesMarketDepth:
		var ev events.DepthSnapshot
		if env.Type != events.TypeDepthSnapshot || env.Decode(&ev) != nil {
			return
		}
		mid, ok := midPrice(ev)
		if !ok {
			return
		}
		target := p.spotMid
		if m.Topic == events.TopicFuturesMarketDepth {
			target = p.futMid
		}
		p.set(target, ev.Symbol, mid)
	case events.TopicMarketTicker:
		var ev events.Ticker
		if env.Type != events.TypeTicker || env.Decode(&ev) != nil {
			return
		}
		if last, err := decimal.NewFromString(ev.LastPrice); err == nil && last.IsPositive() {
			p.set(p.spotLast, ev.Symbol, last)
		}
	case events.TopicFuturesMatchingEvents:
		if env.Type != events.TypeTradeExecuted {
			return
		}
		var ev events.TradeExecuted
		if env.Decode(&ev) != nil {
			return
		}
		if price, err := decimal.NewFromString(ev.Price); err == nil && price.IsPositive() {
			p.set(p.futLast, ev.Symbol, price)
		}
	}
}

func (p *Prices) set(m map[string]decimal.Decimal, symbol string, v decimal.Decimal) {
	p.mu.Lock()
	m[symbol] = v
	p.mu.Unlock()
}

// midPrice is (best bid + best ask) / 2 when both sides exist.
func midPrice(ev events.DepthSnapshot) (decimal.Decimal, bool) {
	if len(ev.Bids) == 0 || len(ev.Asks) == 0 {
		return decimal.Zero, false
	}
	bid, err1 := decimal.NewFromString(ev.Bids[0][0])
	ask, err2 := decimal.NewFromString(ev.Asks[0][0])
	if err1 != nil || err2 != nil || !bid.IsPositive() || !ask.IsPositive() {
		return decimal.Zero, false
	}
	return bid.Add(ask).Div(decimal.NewFromInt(2)), true
}

func markFromEvent(ev events.MarkPrice) (MarkInfo, bool) {
	mark, err := decimal.NewFromString(ev.MarkPrice)
	if err != nil || !mark.IsPositive() {
		return MarkInfo{}, false
	}
	index, _ := decimal.NewFromString(ev.IndexPrice)
	rate, _ := decimal.NewFromString(ev.FundingRate)
	return MarkInfo{Symbol: ev.Symbol, Mark: mark, Index: index, FundingRate: rate,
		NextFundingTime: ev.NextFundingTime, Time: ev.Time}, true
}
