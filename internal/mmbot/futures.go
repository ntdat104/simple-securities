package mmbot

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"simple-securities/pkg/symbols"
)

const (
	futuresPrefix = "/fapi/v1"

	futuresLeverage       = 5
	futuresPositionPeriod = 5 * time.Second
	futuresPremiumPeriod  = 2 * time.Second
	futuresMaxPremium     = 0.0005 // ±0.05% around the spot index
	futuresPremiumSigma   = 0.0001 // random-walk step of the premium
)

var (
	futuresTarget   = decimal.NewFromInt(2_000_000) // USDT kept in each bot's futures wallet
	futuresLowWater = decimal.NewFromInt(500_000)   // top up when availableBalance falls below
	maxPosNotional  = decimal.NewFromInt(200_000)   // shrink positions above this notional (USDT)
)

// futuresState is the futures side of the bot.
type futuresState struct {
	markets []market

	mu        sync.RWMutex
	premium   map[string]float64                      // symbol -> premium over the index
	positions map[*Session]map[string]decimal.Decimal // session -> symbol -> positionAmt (signed)
}

// apiPath maps a spot API path to the market's API.
func apiPath(m market, path string) string {
	if m.Futures {
		return futuresPrefix + path
	}
	return path
}

// refPrice is the price the bot quotes around: the spot reference, plus a small random premium for
// futures.
func (b *Bot) refPrice(m market) decimal.Decimal {
	if !m.Futures {
		return b.feed.Price(m.Symbol)
	}
	idx := b.feed.Price(m.Underlying)
	if !idx.IsPositive() {
		return decimal.Zero
	}
	b.fut.mu.RLock()
	p := b.fut.premium[m.Symbol]
	b.fut.mu.RUnlock()
	return idx.Mul(decimal.NewFromFloat(1 + p))
}

func (b *Bot) position(s *Session, symbol string) decimal.Decimal {
	b.fut.mu.RLock()
	defer b.fut.mu.RUnlock()
	return b.fut.positions[s][symbol]
}

// runFutures waits for the futures API, prepares the bot users (leverage, collateral) and starts
// the futures maker / taker loops. It returns once the loops are started.
func (b *Bot) runFutures(ctx context.Context, wg *sync.WaitGroup) {
	b.fut = &futuresState{premium: map[string]float64{}, positions: map[*Session]map[string]decimal.Decimal{}}
	if err := b.waitFuturesExchangeInfo(ctx); err != nil {
		return // cancelled
	}
	sessions := []*Session{b.maker, b.taker}
	for _, s := range sessions {
		for _, m := range b.fut.markets {
			body := map[string]any{"symbol": m.Symbol, "leverage": futuresLeverage}
			if err := s.Do(ctx, http.MethodPost, futuresPrefix+"/leverage", nil, body, nil); err != nil {
				s.log.Info("set futures leverage failed (ignored)", zap.String("symbol", m.Symbol), zap.Error(err))
			}
		}
		if err := b.futuresTopUp(ctx, s); err != nil {
			s.log.Warn("futures top up failed", zap.Error(err))
		}
	}
	_ = b.refreshPositions(ctx)

	wg.Go(func() {
		b.loop(ctx, "futures premium", constant(futuresPremiumPeriod),
			func(context.Context) error { b.walkPremium(); return nil })
	})
	wg.Go(func() {
		b.loop(ctx, "futures positions", constant(futuresPositionPeriod), b.refreshPositions)
	})
	for _, s := range sessions {
		wg.Go(func() {
			b.loop(ctx, "futures topup", constant(accountPeriod),
				func(ctx context.Context) error { return b.futuresTopUp(ctx, s) })
		})
	}
	for _, m := range b.fut.markets {
		cancelled := map[string]time.Time{}
		wg.Go(func() {
			b.loop(ctx, "futures maker "+m.Symbol, jitter(2*time.Second, 3*time.Second),
				func(ctx context.Context) error { return b.makerTick(ctx, m, cancelled) })
		})
		wg.Go(func() {
			b.loop(ctx, "futures taker "+m.Symbol, jitter(1500*time.Millisecond, 5*time.Second),
				func(ctx context.Context) error { return b.takerTick(ctx, m) })
		})
	}
	b.log.Info("futures market maker running", zap.Int("markets", len(b.fut.markets)))
}

func (b *Bot) waitFuturesExchangeInfo(ctx context.Context) error {
	return retry(ctx, b.log, "futures exchangeInfo", func(ctx context.Context) error {
		var info struct {
			Symbols []struct {
				Symbol      string `json:"symbol"`
				BaseAsset   string `json:"baseAsset"`
				QuoteAsset  string `json:"quoteAsset"`
				TickSize    string `json:"tickSize"`
				StepSize    string `json:"stepSize"`
				MinNotional string `json:"minNotional"`
				Underlying  string `json:"underlying"`
				Status      string `json:"status"`
			} `json:"symbols"`
		}
		if err := b.c.Do(ctx, http.MethodGet, futuresPrefix+"/exchangeInfo", nil, "", nil, &info); err != nil {
			return err
		}
		var out []market
		for _, s := range info.Symbols {
			if s.Status != "" && s.Status != "TRADING" {
				continue
			}
			ref, ok := symbols.GetMarket(symbols.MarketFutures, s.Symbol)
			if !ok {
				continue
			}
			underlying := s.Underlying
			if underlying == "" {
				underlying = ref.Underlying
			}
			if !b.feed.Price(underlying).IsPositive() {
				continue // no spot reference for the index
			}
			out = append(out, market{Futures: true, Underlying: underlying, Symbol: s.Symbol,
				Base: s.BaseAsset, Quote: s.QuoteAsset,
				Tick: decOr(s.TickSize, ref.TickSize), Step: decOr(s.StepSize, ref.StepSize),
				MinNotional: decOr(s.MinNotional, ref.MinNotional)})
		}
		if len(out) == 0 {
			return fmt.Errorf("no tradable futures symbols")
		}
		b.fut.markets = out
		return nil
	})
}

// walkPremium moves each contract's premium over the index by a small bounded random walk.
func (b *Bot) walkPremium() {
	b.fut.mu.Lock()
	defer b.fut.mu.Unlock()
	for _, m := range b.fut.markets {
		p := b.fut.premium[m.Symbol] + rand.NormFloat64()*futuresPremiumSigma
		b.fut.premium[m.Symbol] = math.Max(-futuresMaxPremium, math.Min(futuresMaxPremium, p))
	}
}

// futuresTopUp keeps ~futuresTarget USDT in the user's futures wallet: when availableBalance is
// below futuresLowWater it deposits the difference to spot (faucet) and transfers it to futures.
func (b *Bot) futuresTopUp(ctx context.Context, s *Session) error {
	var acct struct {
		AvailableBalance string `json:"availableBalance"`
	}
	err := s.Do(ctx, http.MethodGet, futuresPrefix+"/account", nil, nil, &acct)
	if err != nil && statusOf(err) != http.StatusNotFound {
		return fmt.Errorf("futures account: %w", err)
	}
	avail := decOr(acct.AvailableBalance, decimal.Zero)
	if avail.GreaterThanOrEqual(futuresLowWater) {
		return nil
	}
	amount := futuresTarget.Sub(avail).Floor()
	if err := s.Do(ctx, http.MethodPost, "/wallet/deposit", nil,
		map[string]string{"asset": "USDT", "amount": amount.String()}, nil); err != nil {
		return fmt.Errorf("deposit USDT for futures: %w", err)
	}
	body := map[string]string{"asset": "USDT", "amount": amount.String(), "type": "SPOT_TO_FUTURES"}
	if err := s.Do(ctx, http.MethodPost, futuresPrefix+"/transfer", nil, body, nil); err != nil {
		return fmt.Errorf("transfer to futures: %w", err)
	}
	s.log.Info("transferred USDT to futures", zap.String("amount", amount.String()),
		zap.String("available_before", avail.String()))
	return nil
}

// refreshPositions caches every bot user's futures positions.
func (b *Bot) refreshPositions(ctx context.Context) error {
	var firstErr error
	for _, s := range []*Session{b.maker, b.taker} {
		var rows []struct {
			Symbol      string `json:"symbol"`
			PositionAmt string `json:"positionAmt"`
		}
		if err := s.Do(ctx, http.MethodGet, futuresPrefix+"/positionRisk", nil, nil, &rows); err != nil {
			firstErr = cmpErr(firstErr, fmt.Errorf("positionRisk: %w", err))
			continue
		}
		pos := make(map[string]decimal.Decimal, len(rows))
		for _, r := range rows {
			pos[r.Symbol] = decOr(r.PositionAmt, decimal.Zero)
		}
		b.fut.mu.Lock()
		b.fut.positions[s] = pos
		b.fut.mu.Unlock()
	}
	return firstErr
}

// futuresTakerTick sends one MARKET order by quantity. The side follows the reference like spot,
// but positions are kept bounded: above maxPosNotional the taker shrinks its own position with a
// reduce-only order, or trades towards reducing the maker's position.
func (b *Bot) futuresTakerTick(ctx context.Context, m market) error {
	ref := b.refPrice(m)
	if !ref.IsPositive() {
		return nil
	}
	takerPos := b.position(b.taker, m.Symbol)
	makerPos := b.position(b.maker, m.Symbol)
	body := map[string]any{"symbol": m.Symbol, "type": "MARKET", "newClientOrderId": clientID()}

	var side string
	var qty decimal.Decimal
	switch {
	case takerPos.Abs().Mul(ref).GreaterThan(maxPosNotional):
		side = sideToReduce(takerPos)
		notional := decimal.NewFromFloat(5000 + rand.Float64()*15000)
		qty = decimal.Min(floorStep(notional.Div(ref), m.Step), takerPos.Abs())
		body["reduceOnly"] = true
	default:
		last := ref
		var trades []struct {
			Price string `json:"price"`
		}
		if err := b.c.Do(ctx, http.MethodGet, futuresPrefix+"/trades", url.Values{"symbol": {m.Symbol}, "limit": {"1"}},
			"", nil, &trades); err == nil && len(trades) > 0 {
			last = decOr(trades[len(trades)-1].Price, ref)
		}
		pBuy := 0.5
		switch last.Cmp(ref) {
		case -1:
			pBuy = 0.75
		case 1:
			pBuy = 0.25
		}
		if makerPos.Abs().Mul(ref).GreaterThan(maxPosNotional) {
			// Buying lifts the maker's asks (maker sells), selling hits its bids (maker buys).
			if makerPos.IsPositive() {
				pBuy = 0.9
			} else {
				pBuy = 0.1
			}
		}
		side = "SELL"
		if rand.Float64() < pBuy {
			side = "BUY"
		}
		notional := decimal.NewFromFloat(200 + rand.Float64()*2800)
		qty = floorStep(notional.Div(ref), m.Step)
		if qty.Mul(ref).LessThan(m.MinNotional) {
			qty = ceilStep(m.MinNotional.Mul(decimal.NewFromFloat(1.1)).Div(ref), m.Step)
		}
	}
	if !qty.IsPositive() {
		return nil
	}
	body["side"], body["quantity"] = side, qty.String()
	if err := b.taker.Do(ctx, http.MethodPost, futuresPrefix+"/order", nil, body, nil); err != nil {
		return fmt.Errorf("futures market %s %s: %w", side, qty, err)
	}
	return nil
}

func sideToReduce(pos decimal.Decimal) string {
	if pos.IsPositive() {
		return "SELL"
	}
	return "BUY"
}
