// Package mmbot is a market-maker / liquidity simulator that trades through the public gateway API
// so the exchange looks alive: a maker keeps a ladder of limit orders around a reference price and
// a taker sends market orders that pull the last price towards that reference.
package mmbot

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"simple-securities/pkg/symbols"
)

// Config configures the bot.
type Config struct {
	GatewayURL   string
	EmailPrefix  string
	Password     string
	BinanceRef   bool          // mirror Binance prices (fallback: random walk)
	OrdersPerSec float64       // per-user pacing of order placement/cancellation
	OrderTTL     time.Duration // maker orders older than this are cancelled and replaced
	Futures      bool          // also make the USDⓈ-M perpetual futures markets
}

const (
	ladderLevels  = 10
	minSpread     = 0.0002 // 0.02%
	maxSpread     = 0.005  // 0.5%
	maxDrift      = 0.006  // cancel maker orders further than 0.6% from mid
	accountPeriod = 30 * time.Second
	lowWaterMark  = 0.3 // top up when free < 30% of target
)

var (
	quoteTarget    = decimal.NewFromInt(5_000_000)
	baseTargetQuot = decimal.NewFromInt(2_000_000)
)

type market struct {
	Futures     bool   // USDⓈ-M perpetual (API under /fapi/v1)
	Underlying  string // futures: spot symbol whose reference price is the index
	Symbol      string
	Base        string
	Quote       string
	Tick        decimal.Decimal
	Step        decimal.Decimal
	MinNotional decimal.Decimal
}

// Bot is the market maker.
type Bot struct {
	cfg     Config
	c       *Client
	log     *zap.Logger
	maker   *Session
	taker   *Session
	feed    *PriceFeed
	markets []market // spot
	fut     *futuresState
}

// New builds a bot.
func New(cfg Config, log *zap.Logger) *Bot {
	c := NewClient(cfg.GatewayURL)
	return &Bot{
		cfg: cfg, c: c, log: log,
		maker: NewSession(c, cfg.EmailPrefix+"-maker@bot.local", cfg.Password, cfg.OrdersPerSec, log),
		taker: NewSession(c, cfg.EmailPrefix+"-taker@bot.local", cfg.Password, cfg.OrdersPerSec, log),
	}
}

// Run blocks until ctx is cancelled. Errors are logged and retried, never fatal.
func (b *Bot) Run(ctx context.Context) error {
	if err := b.waitExchangeInfo(ctx); err != nil {
		return nil // cancelled
	}
	for _, s := range []*Session{b.maker, b.taker} {
		if err := retry(ctx, b.log, "login", s.Login); err != nil {
			return nil
		}
	}
	// Seed the reference from the exchange's own last spot price so a restart (or Binance being
	// unreachable) continues from the current market instead of the static registry price.
	var tickers []struct{ Symbol, LastPrice string }
	if err := b.c.Do(ctx, http.MethodGet, "/ticker/24hr", nil, "", nil, &tickers); err != nil {
		b.log.Warn("seed from last prices failed, using registry prices", zap.Error(err))
	}
	last := map[string]decimal.Decimal{}
	for _, t := range tickers {
		if p, err := decimal.NewFromString(t.LastPrice); err == nil && p.IsPositive() {
			last[t.Symbol] = p
		}
	}
	seed, ticks := map[string]decimal.Decimal{}, map[string]decimal.Decimal{}
	for _, m := range b.markets {
		ref, _ := symbols.Get(m.Symbol)
		seed[m.Symbol], ticks[m.Symbol] = ref.RefPrice, m.Tick
		if p, ok := last[m.Symbol]; ok {
			seed[m.Symbol] = p
		}
	}
	b.feed = NewPriceFeed(b.cfg.BinanceRef, seed, ticks, b.log.Named("feed"))
	b.feed.update(ctx)

	var wg sync.WaitGroup
	wg.Go(func() { b.feed.Run(ctx) })
	for _, s := range []*Session{b.maker, b.taker} {
		_ = b.topUp(ctx, s)
		wg.Go(func() {
			b.loop(ctx, "topup", constant(accountPeriod), func(ctx context.Context) error { return b.topUp(ctx, s) })
		})
	}
	for _, m := range b.markets {
		cancelled := map[string]time.Time{}
		wg.Go(func() {
			b.loop(ctx, "maker "+m.Symbol, jitter(1200*time.Millisecond, 1800*time.Millisecond),
				func(ctx context.Context) error { return b.makerTick(ctx, m, cancelled) })
		})
		wg.Go(func() {
			b.loop(ctx, "taker "+m.Symbol, jitter(500*time.Millisecond, 3*time.Second),
				func(ctx context.Context) error { return b.takerTick(ctx, m) })
		})
	}
	if b.cfg.Futures {
		wg.Go(func() { b.runFutures(ctx, &wg) })
	}
	b.log.Info("market maker running", zap.Int("markets", len(b.markets)), zap.Bool("binance_ref", b.cfg.BinanceRef),
		zap.Bool("futures", b.cfg.Futures))
	wg.Wait()
	return nil
}

func (b *Bot) waitExchangeInfo(ctx context.Context) error {
	return retry(ctx, b.log, "exchangeInfo", func(ctx context.Context) error {
		var info struct {
			Symbols []struct {
				Symbol      string `json:"symbol"`
				BaseAsset   string `json:"baseAsset"`
				QuoteAsset  string `json:"quoteAsset"`
				TickSize    string `json:"tickSize"`
				StepSize    string `json:"stepSize"`
				MinNotional string `json:"minNotional"`
			} `json:"symbols"`
		}
		if err := b.c.Do(ctx, http.MethodGet, "/exchangeInfo", nil, "", nil, &info); err != nil {
			return err
		}
		b.markets = b.markets[:0]
		for _, s := range info.Symbols {
			ref, ok := symbols.Get(s.Symbol)
			if !ok || !ref.RefPrice.IsPositive() {
				continue
			}
			m := market{Symbol: s.Symbol, Base: s.BaseAsset, Quote: s.QuoteAsset,
				Tick: decOr(s.TickSize, ref.TickSize), Step: decOr(s.StepSize, ref.StepSize),
				MinNotional: decOr(s.MinNotional, ref.MinNotional)}
			b.markets = append(b.markets, m)
		}
		if len(b.markets) == 0 {
			return fmt.Errorf("no tradable symbols")
		}
		return nil
	})
}

// topUp deposits funds whenever a free balance falls below the low-water mark.
func (b *Bot) topUp(ctx context.Context, s *Session) error {
	var acct struct {
		Balances []struct {
			Asset string `json:"asset"`
			Free  string `json:"free"`
		} `json:"balances"`
	}
	if err := s.Do(ctx, http.MethodGet, "/account", nil, nil, &acct); err != nil {
		return fmt.Errorf("account: %w", err)
	}
	free := map[string]decimal.Decimal{}
	for _, bal := range acct.Balances {
		free[bal.Asset] = decOr(bal.Free, decimal.Zero)
	}
	targets := map[string]decimal.Decimal{}
	for _, m := range b.markets {
		targets[m.Quote] = quoteTarget
		if ref := b.feed.Price(m.Symbol); ref.IsPositive() {
			targets[m.Base] = floorStep(baseTargetQuot.Div(ref), m.Step)
		}
	}
	for asset, target := range targets {
		if free[asset].GreaterThanOrEqual(target.Mul(decimal.NewFromFloat(lowWaterMark))) {
			continue
		}
		amount := target.Sub(free[asset])
		err := s.Do(ctx, http.MethodPost, "/wallet/deposit", nil,
			map[string]string{"asset": asset, "amount": amount.String()}, nil)
		if err != nil {
			s.log.Warn("deposit failed", zap.String("asset", asset), zap.String("amount", amount.String()), zap.Error(err))
			continue
		}
		s.log.Info("deposited", zap.String("asset", asset), zap.String("amount", amount.String()))
	}
	return nil
}

type openOrder struct {
	OrderID flexString `json:"orderId"`
	Side    string     `json:"side"`
	Price   string     `json:"price"`
	Time    flexInt    `json:"time"`
}

type quote struct {
	Side  string
	Price decimal.Decimal
	Qty   decimal.Decimal
}

// makerTick cancels stale/drifted orders and re-places missing ladder levels.
func (b *Bot) makerTick(ctx context.Context, m market, cancelled map[string]time.Time) error {
	ref := b.refPrice(m)
	if !ref.IsPositive() {
		return nil
	}
	var open []openOrder
	if err := b.maker.Do(ctx, http.MethodGet, apiPath(m, "/openOrders"), url.Values{"symbol": {m.Symbol}}, nil, &open); err != nil {
		return fmt.Errorf("open orders: %w", err)
	}
	now := time.Now()
	for id, at := range cancelled {
		if now.Sub(at) > time.Minute {
			delete(cancelled, id)
		}
	}
	var live []quote
	var firstErr error
	for _, o := range open {
		id := string(o.OrderID)
		if _, ok := cancelled[id]; ok {
			continue
		}
		p := decOr(o.Price, decimal.Zero)
		age := now.Sub(time.UnixMilli(int64(o.Time)))
		if drift(p, ref) <= maxDrift && (o.Time == 0 || age <= b.cfg.OrderTTL) {
			live = append(live, quote{Side: o.Side, Price: p})
			continue
		}
		err := b.maker.Do(ctx, http.MethodDelete, apiPath(m, "/order"), url.Values{"orderId": {id}}, nil, nil)
		if err != nil && statusOf(err) != http.StatusNotFound && statusOf(err) != http.StatusBadRequest {
			firstErr = cmpErr(firstErr, fmt.Errorf("cancel %s: %w", id, err))
			continue
		}
		cancelled[id] = now
	}
	halfGap := ref.Mul(decimal.NewFromFloat((maxSpread - minSpread) / (ladderLevels - 1) / 2))
	for _, q := range ladder(ref, m, rand.Float64) {
		if occupied(live, q, halfGap) {
			continue
		}
		body := map[string]string{"symbol": m.Symbol, "side": q.Side, "type": "LIMIT", "timeInForce": "GTC",
			"price": q.Price.String(), "quantity": q.Qty.String(), "newClientOrderId": clientID()}
		if err := b.maker.Do(ctx, http.MethodPost, apiPath(m, "/order"), nil, body, nil); err != nil {
			firstErr = cmpErr(firstErr, fmt.Errorf("place %s %s@%s: %w", q.Side, q.Qty, q.Price, err))
			continue
		}
		live = append(live, q)
	}
	return firstErr
}

// takerTick sends one market order, biased so the last price follows the reference.
func (b *Bot) takerTick(ctx context.Context, m market) error {
	if m.Futures {
		return b.futuresTakerTick(ctx, m)
	}
	ref := b.feed.Price(m.Symbol)
	if !ref.IsPositive() {
		return nil
	}
	last := ref
	var trades []struct {
		Price string `json:"price"`
	}
	if err := b.c.Do(ctx, http.MethodGet, "/trades", url.Values{"symbol": {m.Symbol}, "limit": {"1"}}, "", nil, &trades); err == nil && len(trades) > 0 {
		last = decOr(trades[len(trades)-1].Price, ref)
	}
	pBuy := 0.5
	switch last.Cmp(ref) {
	case -1:
		pBuy = 0.75
	case 1:
		pBuy = 0.25
	}
	notional := decimal.NewFromFloat(50 + rand.Float64()*1450)
	body := map[string]string{"symbol": m.Symbol, "type": "MARKET", "newClientOrderId": clientID()}
	if rand.Float64() < pBuy {
		body["side"], body["quoteOrderQty"] = "BUY", notional.StringFixed(2)
	} else {
		qty := floorStep(notional.Div(ref), m.Step)
		if qty.Mul(ref).LessThan(m.MinNotional) {
			return nil
		}
		body["side"], body["quantity"] = "SELL", qty.String()
	}
	if err := b.taker.Do(ctx, http.MethodPost, "/order", nil, body, nil); err != nil {
		return fmt.Errorf("market %s: %w", body["side"], err)
	}
	return nil
}

// ladder builds ladderLevels bids and asks with spreads from minSpread to maxSpread, randomized
// sizes of ~200-3000 quote, rounded to the symbol filters; closest levels first.
func ladder(ref decimal.Decimal, m market, rnd func() float64) []quote {
	one := decimal.NewFromInt(1)
	out := make([]quote, 0, 2*ladderLevels)
	for i := range ladderLevels {
		spread := decimal.NewFromFloat(minSpread + float64(i)*(maxSpread-minSpread)/(ladderLevels-1))
		bid := floorStep(ref.Mul(one.Sub(spread)), m.Tick)
		ask := ceilStep(ref.Mul(one.Add(spread)), m.Tick)
		for _, q := range []quote{{Side: "BUY", Price: bid}, {Side: "SELL", Price: ask}} {
			if !q.Price.IsPositive() {
				continue
			}
			notional := decimal.NewFromFloat(200 + rnd()*2800)
			q.Qty = floorStep(notional.Div(q.Price), m.Step)
			if q.Qty.IsPositive() && q.Qty.Mul(q.Price).GreaterThanOrEqual(m.MinNotional) {
				out = append(out, q)
			}
		}
	}
	return out
}

func occupied(live []quote, q quote, halfGap decimal.Decimal) bool {
	for _, l := range live {
		if l.Side == q.Side && l.Price.Sub(q.Price).Abs().LessThanOrEqual(halfGap) {
			return true
		}
	}
	return false
}

func drift(p, ref decimal.Decimal) float64 {
	if !ref.IsPositive() {
		return 0
	}
	f, _ := p.Sub(ref).Abs().Div(ref).Float64()
	return f
}

// loop runs fn on a schedule with exponential backoff on errors until ctx is cancelled.
func (b *Bot) loop(ctx context.Context, name string, next func() time.Duration, fn func(context.Context) error) {
	var backoff time.Duration
	for {
		if !sleep(ctx, next()+backoff) {
			return
		}
		if err := fn(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			backoff = min(max(2*backoff, time.Second), 30*time.Second)
			b.log.Warn("tick failed", zap.String("loop", name), zap.Duration("backoff", backoff), zap.Error(err))
			continue
		}
		backoff = 0
	}
}

// retry calls fn until it succeeds or ctx is cancelled.
func retry(ctx context.Context, log *zap.Logger, what string, fn func(context.Context) error) error {
	backoff := time.Second
	for {
		err := fn(ctx)
		if err == nil {
			return nil
		}
		log.Info("waiting for gateway", zap.String("step", what), zap.Duration("retry_in", backoff), zap.Error(err))
		if !sleep(ctx, backoff) {
			return ctx.Err()
		}
		backoff = min(2*backoff, 15*time.Second)
	}
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func constant(d time.Duration) func() time.Duration { return func() time.Duration { return d } }

func jitter(lo, hi time.Duration) func() time.Duration {
	return func() time.Duration { return lo + rand.N(hi-lo) }
}

func floorStep(v, step decimal.Decimal) decimal.Decimal { return symbols.FloorToStep(v, step) }

func ceilStep(v, step decimal.Decimal) decimal.Decimal {
	if step.IsZero() {
		return v
	}
	return v.Div(step).Ceil().Mul(step)
}

func decOr(s string, def decimal.Decimal) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return def
	}
	return d
}

func cmpErr(first, err error) error {
	if first != nil {
		return first
	}
	return err
}

func clientID() string {
	var b [8]byte
	for i := range b {
		b[i] = byte(rand.UintN(256))
	}
	return "mm" + hex.EncodeToString(b[:])
}
