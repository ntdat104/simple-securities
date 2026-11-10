package ws

import (
	"fmt"
	"strings"

	"simple-securities/pkg/symbols"
)

// Stream kinds.
const (
	KindTrade        = "trade"
	KindDepth        = "depth"
	KindTicker       = "ticker"
	KindKline        = "kline"
	KindTickerArr    = "tickerArr"
	KindMarkPrice    = "markPrice"    // futures only
	KindMarkPriceArr = "markPriceArr" // futures only
)

// AllTickers is the aggregated ticker stream.
const AllTickers = "!ticker@arr"

// FuturesPrefix marks a stream of the USDⓈ-M futures market, e.g. "f.btcusdt@depth20".
const FuturesPrefix = "f."

// AllMarkPrices is the aggregated mark price stream (futures, without the prefix).
const AllMarkPrices = "!markPrice@arr"

// markPriceChannel is the canonical (mixed-case) per-symbol mark price channel.
const markPriceChannel = "markPrice"

var (
	_klineIntervals = map[string]bool{"1m": true, "5m": true, "15m": true, "1h": true, "4h": true, "1d": true}
	_depthLevels    = map[string]int{"depth": 0, "depth5": 5, "depth10": 10, "depth20": 20}
)

// Stream is a parsed, normalized stream name.
type Stream struct {
	Name     string // canonical name, e.g. btcusdt@depth20, f.btcusdt@markPrice
	Futures  bool   // futures market stream ("f." prefix)
	Kind     string
	Symbol   string // upper-case, e.g. BTCUSDT (empty for aggregated streams)
	Levels   int    // depth levels (0 = full snapshot)
	Interval string // kline interval
}

// ParseStream validates a stream name such as "btcusdt@trade", "btcusdt@depth5",
// "btcusdt@kline_1m", "btcusdt@ticker" or "!ticker@arr". Futures streams carry the "f." prefix
// and additionally support "<sym>@markPrice" and "!markPrice@arr". Symbols must be listed in the
// registry of the stream's market.
func ParseStream(name string) (Stream, error) {
	n := strings.ToLower(strings.TrimSpace(name))
	if rest, ok := strings.CutPrefix(n, FuturesPrefix); ok {
		s, err := parseMarketStream(rest, true, name)
		if err != nil {
			return Stream{}, err
		}
		s.Name = FuturesPrefix + s.Name
		return s, nil
	}
	return parseMarketStream(n, false, name)
}

// parseMarketStream parses a lower-cased stream name without market prefix.
func parseMarketStream(n string, futures bool, name string) (Stream, error) {
	if n == AllTickers {
		return Stream{Name: n, Futures: futures, Kind: KindTickerArr}, nil
	}
	if futures && n == strings.ToLower(AllMarkPrices) {
		return Stream{Name: AllMarkPrices, Futures: true, Kind: KindMarkPriceArr}, nil
	}
	sym, ch, ok := strings.Cut(n, "@")
	if !ok || sym == "" || ch == "" {
		return Stream{}, fmt.Errorf("invalid stream %q", name)
	}
	upper := strings.ToUpper(sym)
	market := symbols.MarketSpot
	if futures {
		market = symbols.MarketFutures
	}
	if _, ok := symbols.GetMarket(market, upper); !ok {
		return Stream{}, fmt.Errorf("unknown symbol in stream %q", name)
	}
	s := Stream{Name: n, Futures: futures, Symbol: upper}
	switch {
	case ch == KindTrade:
		s.Kind = KindTrade
	case ch == KindTicker:
		s.Kind = KindTicker
	case futures && ch == strings.ToLower(markPriceChannel):
		s.Kind, s.Name = KindMarkPrice, streamName(upper, markPriceChannel)
	case strings.HasPrefix(ch, "depth"):
		lv, ok := _depthLevels[ch]
		if !ok {
			return Stream{}, fmt.Errorf("invalid depth stream %q (use depth, depth5, depth10, depth20)", name)
		}
		s.Kind, s.Levels = KindDepth, lv
	case strings.HasPrefix(ch, "kline_"):
		iv := strings.TrimPrefix(ch, "kline_")
		if !_klineIntervals[iv] {
			return Stream{}, fmt.Errorf("invalid kline interval in %q", name)
		}
		s.Kind, s.Interval = KindKline, iv
	default:
		return Stream{}, fmt.Errorf("invalid stream %q", name)
	}
	return s, nil
}

// streamName builds the canonical name for a symbol channel.
func streamName(symbol, channel string) string { return strings.ToLower(symbol) + "@" + channel }

// marketStreamName builds the canonical name of a symbol channel of a market.
func marketStreamName(futures bool, symbol, channel string) string {
	if futures {
		return FuturesPrefix + streamName(symbol, channel)
	}
	return streamName(symbol, channel)
}

// tickerArrName is the aggregated ticker stream of a market.
func tickerArrName(futures bool) string {
	if futures {
		return FuturesPrefix + AllTickers
	}
	return AllTickers
}
