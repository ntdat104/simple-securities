package marketdata

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"simple-securities/pkg/events"
)

// Interval is a supported kline interval.
type Interval struct {
	Name string
	Dur  time.Duration
}

// Intervals lists every kline interval maintained by the service.
var Intervals = []Interval{
	{"1m", time.Minute},
	{"5m", 5 * time.Minute},
	{"15m", 15 * time.Minute},
	{"1h", time.Hour},
	{"4h", 4 * time.Hour},
	{"1d", 24 * time.Hour},
}

// ParseInterval looks up an interval by name.
func ParseInterval(name string) (Interval, bool) {
	for _, iv := range Intervals {
		if iv.Name == name {
			return iv, true
		}
	}
	return Interval{}, false
}

// BucketOpen returns the epoch-aligned (UTC) open time of the bucket containing t.
func BucketOpen(t time.Time, d time.Duration) time.Time {
	ms, dm := t.UnixMilli(), d.Milliseconds()
	return time.UnixMilli(ms - ms%dm).UTC()
}

// BucketClose returns the Binance-style close time (open + d - 1ms).
func BucketClose(open time.Time, d time.Duration) time.Time {
	return open.Add(d - time.Millisecond)
}

// trade is a parsed trade.executed event.
type trade struct {
	Symbol       string
	ID           int64
	Price        decimal.Decimal
	Qty          decimal.Decimal
	QuoteQty     decimal.Decimal
	BuyerIsMaker bool
	Time         time.Time
}

func parseTrade(e events.TradeExecuted) (trade, error) {
	price, err := decimal.NewFromString(e.Price)
	if err != nil {
		return trade{}, fmt.Errorf("price %q: %w", e.Price, err)
	}
	qty, err := decimal.NewFromString(e.Qty)
	if err != nil {
		return trade{}, fmt.Errorf("qty %q: %w", e.Qty, err)
	}
	quote := price.Mul(qty)
	if e.QuoteQty != "" {
		if quote, err = decimal.NewFromString(e.QuoteQty); err != nil {
			return trade{}, fmt.Errorf("quote_qty %q: %w", e.QuoteQty, err)
		}
	}
	if e.Symbol == "" {
		return trade{}, fmt.Errorf("empty symbol")
	}
	t := e.Time
	if t.IsZero() {
		t = time.Now()
	}
	return trade{Symbol: e.Symbol, ID: e.TradeID, Price: price, Qty: qty, QuoteQty: quote,
		BuyerIsMaker: e.BuyerIsMaker, Time: t.UTC()}, nil
}

type bucketKey struct {
	Symbol   string
	Interval string
	OpenTime int64 // epoch ms
}

// bucket is the aggregated state of one kline (either a batch delta or the merged DB state).
type bucket struct {
	bucketKey
	CloseTime   int64
	Open        decimal.Decimal
	High        decimal.Decimal
	Low         decimal.Decimal
	Close       decimal.Decimal
	Volume      decimal.Decimal
	QuoteVolume decimal.Decimal
	Count       int64
}

func (b *bucket) toEvent() events.Kline {
	return events.Kline{
		Symbol: b.Symbol, Interval: b.Interval, OpenTime: b.OpenTime, CloseTime: b.CloseTime,
		Open: b.Open.String(), High: b.High.String(), Low: b.Low.String(), Close: b.Close.String(),
		Volume: b.Volume.String(), QuoteVolume: b.QuoteVolume.String(), TradeCount: b.Count,
	}
}

// aggregate folds trades (in partition order) into one delta per (symbol, interval, open_time),
// returned in order of first appearance so upserts are deterministic.
func aggregate(trades []trade) []*bucket {
	idx := make(map[bucketKey]*bucket)
	var out []*bucket
	for _, t := range trades {
		for _, iv := range Intervals {
			open := BucketOpen(t.Time, iv.Dur)
			k := bucketKey{Symbol: t.Symbol, Interval: iv.Name, OpenTime: open.UnixMilli()}
			b, ok := idx[k]
			if !ok {
				b = &bucket{bucketKey: k, CloseTime: BucketClose(open, iv.Dur).UnixMilli(),
					Open: t.Price, High: t.Price, Low: t.Price, Close: t.Price}
				idx[k] = b
				out = append(out, b)
			}
			if t.Price.GreaterThan(b.High) {
				b.High = t.Price
			}
			if t.Price.LessThan(b.Low) {
				b.Low = t.Price
			}
			b.Close = t.Price
			b.Volume = b.Volume.Add(t.Qty)
			b.QuoteVolume = b.QuoteVolume.Add(t.QuoteQty)
			b.Count++
		}
	}
	return out
}
