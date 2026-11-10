package marketdata

import (
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"simple-securities/pkg/events"
)

var hundred = decimal.NewFromInt(100)

// tickerStats is the 24h aggregate of a symbol's 1m klines.
type tickerStats struct {
	Open        decimal.Decimal
	High        decimal.Decimal
	Low         decimal.Decimal
	Last        decimal.Decimal
	Volume      decimal.Decimal
	QuoteVolume decimal.Decimal
	Count       int64
}

// priceChange returns last-open and the percent change rounded to 2 decimals ("0.00" if open is 0).
func priceChange(open, last decimal.Decimal) (decimal.Decimal, string) {
	if open.IsZero() {
		return decimal.Zero, "0.00"
	}
	change := last.Sub(open)
	return change, change.Div(open).Mul(hundred).StringFixed(2)
}

func buildTicker(symbol string, s tickerStats, now time.Time) events.Ticker {
	change, pct := priceChange(s.Open, s.Last)
	return events.Ticker{
		Symbol:             symbol,
		LastPrice:          s.Last.String(),
		OpenPrice:          s.Open.String(),
		HighPrice:          s.High.String(),
		LowPrice:           s.Low.String(),
		Volume:             s.Volume.String(),
		QuoteVolume:        s.QuoteVolume.String(),
		PriceChange:        change.String(),
		PriceChangePercent: pct,
		TradeCount:         s.Count,
		CloseTime:          now.UnixMilli(),
	}
}

// klineThrottle coalesces kline updates so each (symbol, interval) is published at most once per
// period; the latest state wins and pending updates are flushed by due().
type klineThrottle struct {
	period  time.Duration
	mu      sync.Mutex
	last    map[[2]string]time.Time
	pending map[[2]string]events.Kline
}

func newKlineThrottle(period time.Duration) *klineThrottle {
	return &klineThrottle{period: period, last: map[[2]string]time.Time{}, pending: map[[2]string]events.Kline{}}
}

// offer returns true when k may be published immediately; otherwise it is kept as pending.
func (t *klineThrottle) offer(k events.Kline, now time.Time) bool {
	key := [2]string{k.Symbol, k.Interval}
	t.mu.Lock()
	defer t.mu.Unlock()
	if now.Sub(t.last[key]) >= t.period {
		t.last[key] = now
		delete(t.pending, key)
		return true
	}
	t.pending[key] = k
	return false
}

// due returns pending klines whose throttle window elapsed.
func (t *klineThrottle) due(now time.Time) []events.Kline {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []events.Kline
	for key, k := range t.pending {
		if now.Sub(t.last[key]) >= t.period {
			out = append(out, k)
			t.last[key] = now
			delete(t.pending, key)
		}
	}
	return out
}
