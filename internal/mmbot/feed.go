package mmbot

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

const (
	binanceTickerURL = "https://api.binance.com/api/v3/ticker/price"
	feedPeriod       = 2 * time.Second
	walkSigma        = 0.0005 // 0.05% per tick
)

// PriceFeed provides a reference price per symbol, either mirrored from Binance or a random walk
// (also the fallback whenever Binance is unreachable).
type PriceFeed struct {
	binance bool
	hc      *http.Client
	log     *zap.Logger
	syms    []string
	ticks   map[string]decimal.Decimal

	mu     sync.RWMutex
	prices map[string]decimal.Decimal
}

// NewPriceFeed seeds the feed with initial prices (symbol -> price) and tick sizes.
func NewPriceFeed(binance bool, seed, ticks map[string]decimal.Decimal, log *zap.Logger) *PriceFeed {
	f := &PriceFeed{binance: binance, hc: &http.Client{Timeout: 3 * time.Second}, log: log,
		ticks: ticks, prices: map[string]decimal.Decimal{}}
	for s, p := range seed {
		f.syms = append(f.syms, s)
		f.prices[s] = p
	}
	return f
}

// Price returns the current reference price.
func (f *PriceFeed) Price(symbol string) decimal.Decimal {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.prices[symbol]
}

// Run updates prices every 2s until ctx is cancelled.
func (f *PriceFeed) Run(ctx context.Context) {
	f.update(ctx)
	t := time.NewTicker(feedPeriod)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			f.update(ctx)
		}
	}
}

func (f *PriceFeed) update(ctx context.Context) {
	var live map[string]decimal.Decimal
	if f.binance {
		var err error
		if live, err = f.fetchBinance(ctx); err != nil && ctx.Err() == nil {
			f.log.Debug("binance price fetch failed, random walk", zap.Error(err))
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.syms {
		if p, ok := live[s]; ok && p.IsPositive() {
			f.prices[s] = p
			continue
		}
		f.prices[s] = randomWalk(f.prices[s], f.ticks[s], rand.NormFloat64())
	}
}

// randomWalk moves p by z standard deviations of walkSigma, rounded to tick and kept positive.
func randomWalk(p, tick decimal.Decimal, z float64) decimal.Decimal {
	next := p.Mul(decimal.NewFromFloat(1 + z*walkSigma))
	if tick.IsPositive() {
		next = next.Div(tick).Round(0).Mul(tick)
		if !next.IsPositive() {
			next = tick
		}
	}
	return next
}

func (f *PriceFeed) fetchBinance(ctx context.Context) (map[string]decimal.Decimal, error) {
	list, err := json.Marshal(f.syms)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		binanceTickerURL+"?symbols="+url.QueryEscape(string(list)), nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance status %d", resp.StatusCode)
	}
	var rows []struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}
	out := make(map[string]decimal.Decimal, len(rows))
	for _, r := range rows {
		if p, err := decimal.NewFromString(r.Price); err == nil {
			out[r.Symbol] = p
		}
	}
	return out, nil
}
