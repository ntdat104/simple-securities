// Package symbols is the trading-pair registry shared by every service.
package symbols

import (
	"sort"

	"github.com/shopspring/decimal"
)

// Symbol describes a spot trading pair and its filters.
type Symbol struct {
	Name        string
	Base        string
	Quote       string
	TickSize    decimal.Decimal // price increment
	StepSize    decimal.Decimal // quantity increment
	MinNotional decimal.Decimal // min price*qty (quote)
	MakerFee    decimal.Decimal
	TakerFee    decimal.Decimal
	RefPrice    decimal.Decimal // seed price for the market-maker bot

	// Futures (USDⓈ-M perpetual) only.
	FeeInQuote      bool            // both sides pay fees in the quote asset (futures); spot buyer pays in base
	MaxLeverage     int             // max leverage
	MaintMarginRate decimal.Decimal // maintenance margin rate (single bracket)
	Underlying      string          // spot symbol whose price is the index
}

// Markets.
const (
	MarketSpot    = "spot"
	MarketFutures = "futures"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

var (
	registry        = map[string]Symbol{}
	futuresRegistry = map[string]Symbol{}
)

func add(name, base, quote, tick, step, minNotional, ref string) {
	registry[name] = Symbol{Name: name, Base: base, Quote: quote, TickSize: d(tick), StepSize: d(step),
		MinNotional: d(minNotional), MakerFee: d("0.001"), TakerFee: d("0.001"), RefPrice: d(ref)}
}

func init() {
	add("BTCUSDT", "BTC", "USDT", "0.01", "0.00001", "5", "97000")
	add("ETHUSDT", "ETH", "USDT", "0.01", "0.0001", "5", "3500")
	add("BNBUSDT", "BNB", "USDT", "0.01", "0.001", "5", "650")
	add("SOLUSDT", "SOL", "USDT", "0.01", "0.001", "5", "200")
	add("XRPUSDT", "XRP", "USDT", "0.0001", "0.1", "5", "2.3")
	add("DOGEUSDT", "DOGE", "USDT", "0.00001", "1", "5", "0.35")

	// USDⓈ-M perpetual contracts (same names as their underlying spot pair, separate market).
	addPerp("BTCUSDT", "0.1", "0.001", "100", 125, "0.004")
	addPerp("ETHUSDT", "0.01", "0.001", "20", 100, "0.005")
	addPerp("BNBUSDT", "0.01", "0.01", "5", 75, "0.006")
	addPerp("SOLUSDT", "0.01", "0.01", "5", 75, "0.006")
	addPerp("XRPUSDT", "0.0001", "0.1", "5", 75, "0.006")
	addPerp("DOGEUSDT", "0.00001", "1", "5", 50, "0.01")
}

func addPerp(name, tick, step, minNotional string, maxLev int, mmr string) {
	spot := registry[name]
	futuresRegistry[name] = Symbol{Name: name, Base: spot.Base, Quote: "USDT", TickSize: d(tick), StepSize: d(step),
		MinNotional: d(minNotional), MakerFee: d("0.0002"), TakerFee: d("0.0005"), RefPrice: spot.RefPrice,
		FeeInQuote: true, MaxLeverage: maxLev, MaintMarginRate: d(mmr), Underlying: name}
}

func registryOf(market string) map[string]Symbol {
	if market == MarketFutures {
		return futuresRegistry
	}
	return registry
}

// GetMarket returns the symbol of a market (spot | futures).
func GetMarket(market, name string) (Symbol, bool) {
	s, ok := registryOf(market)[name]
	return s, ok
}

// AllMarket returns every symbol of a market sorted by name.
func AllMarket(market string) []Symbol {
	m := registryOf(market)
	out := make([]Symbol, 0, len(m))
	for _, s := range m {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get returns the SPOT symbol by name.
func Get(name string) (Symbol, bool) {
	s, ok := registry[name]
	return s, ok
}

// All returns every SPOT symbol sorted by name.
func All() []Symbol {
	out := make([]Symbol, 0, len(registry))
	for _, s := range registry {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Assets returns every distinct asset.
func Assets() []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range All() {
		for _, a := range []string{s.Quote, s.Base} {
			if !seen[a] {
				seen[a] = true
				out = append(out, a)
			}
		}
	}
	return out
}

// IsMultiple reports whether v is an exact multiple of step.
func IsMultiple(v, step decimal.Decimal) bool {
	if step.IsZero() {
		return true
	}
	return v.Mod(step).IsZero()
}

// FloorToStep rounds v down to a multiple of step.
func FloorToStep(v, step decimal.Decimal) decimal.Decimal {
	if step.IsZero() {
		return v
	}
	return v.Div(step).Floor().Mul(step)
}
