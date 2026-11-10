package futures

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"simple-securities/pkg/events"
	"simple-securities/pkg/symbols"
)

var (
	emaAlpha        = decimal.NewFromInt(2).Div(decimal.NewFromInt(61)) // EMA(60 samples of 1s)
	interestRate    = decimal.RequireFromString("0.0001")
	premiumClampMax = decimal.RequireFromString("0.0005")
	fundingRateCap  = decimal.RequireFromString("0.0075")
)

// markState is the per-contract premium window of the current funding interval and the basis EMA.
type markState struct {
	premiumSum decimal.Decimal
	samples    int64
	basisEMA   decimal.Decimal
	hasEMA     bool
}

func (st *leaderState) mark(symbol string) *markState {
	m, ok := st.marks[symbol]
	if !ok {
		m = &markState{}
		st.marks[symbol] = m
	}
	return m
}

// sample adds a premium / basis observation.
func (m *markState) sample(futuresPrice, index decimal.Decimal) {
	basis := futuresPrice.Sub(index)
	m.premiumSum = m.premiumSum.Add(basis.Div(index))
	m.samples++
	if !m.hasEMA {
		m.basisEMA, m.hasEMA = basis, true
		return
	}
	m.basisEMA = m.basisEMA.Add(emaAlpha.Mul(basis.Sub(m.basisEMA)))
}

// estimatedRate = clamp(avg_premium + clamp(0.0001 - avg_premium, ±0.0005), ±0.0075).
func (m *markState) estimatedRate() decimal.Decimal {
	avg := decimal.Zero
	if m.samples > 0 {
		avg = m.premiumSum.Div(decimal.NewFromInt(m.samples))
	}
	rate := avg.Add(clamp(interestRate.Sub(avg), premiumClampMax.Neg(), premiumClampMax))
	return money(clamp(rate, fundingRateCap.Neg(), fundingRateCap))
}

// resetWindow starts a new funding interval (the basis EMA is kept).
func (m *markState) resetWindow() {
	m.premiumSum, m.samples = decimal.Zero, 0
}

func clamp(v, lo, hi decimal.Decimal) decimal.Decimal {
	return decimal.Max(lo, decimal.Min(v, hi))
}

func median3(a, b, c decimal.Decimal) decimal.Decimal {
	return decimal.Max(decimal.Min(a, b), decimal.Min(decimal.Max(a, b), c))
}

func roundToTick(v, tick decimal.Decimal) decimal.Decimal {
	if !tick.IsPositive() {
		return v
	}
	return v.Div(tick).Round(0).Mul(tick)
}

// fundingBoundary returns the latest funding time <= now (intervals aligned to the Unix epoch).
func fundingBoundary(now time.Time, interval time.Duration) time.Time {
	ms, iv := now.UnixMilli(), interval.Milliseconds()
	return time.UnixMilli(ms - ms%iv).UTC()
}

// computeMarks samples the premium, computes the mark price of every contract and publishes it
// (spec: mark = median(index·(1 + est_rate·t/interval), index + basis_ema, futures_last), missing
// inputs → index).
func (s *Service) computeMarks(ctx context.Context, st *leaderState, now time.Time) {
	next := fundingBoundary(now, s.cfg.FundingInterval).Add(s.cfg.FundingInterval)
	left := decimal.NewFromInt(next.Sub(now).Milliseconds()).
		Div(decimal.NewFromInt(s.cfg.FundingInterval.Milliseconds()))
	for _, sym := range symbols.AllMarket(symbols.MarketFutures) {
		index, ok := s.prices.Index(sym)
		if !ok || !index.IsPositive() {
			continue
		}
		ms := st.mark(sym.Name)
		mid, hasMid, last, hasLast := s.prices.futures(sym.Name)
		switch {
		case hasMid:
			ms.sample(mid, index)
		case hasLast:
			ms.sample(last, index)
		}
		rate := ms.estimatedRate()
		p1 := index.Mul(decimal.NewFromInt(1).Add(rate.Mul(left)))
		p2 := index
		if ms.hasEMA {
			p2 = index.Add(ms.basisEMA)
		}
		p3 := index
		if hasLast {
			p3 = last
		}
		mark := roundToTick(median3(p1, p2, p3), sym.TickSize)
		if !mark.IsPositive() {
			continue
		}
		info := MarkInfo{Symbol: sym.Name, Mark: mark, Index: index, FundingRate: rate,
			NextFundingTime: next.UnixMilli(), Time: now.UnixMilli()}
		s.prices.SetMark(info)

		env, err := events.New(ServiceName, events.TypeMarkPrice, sym.Name, events.MarkPrice{
			Symbol: sym.Name, MarkPrice: mark.String(), IndexPrice: index.String(), FundingRate: rate.String(),
			NextFundingTime: info.NextFundingTime, Time: info.Time,
		})
		if err != nil {
			s.log.Error("encode mark price", zap.Error(err))
			continue
		}
		s.pub.PublishAsync(ctx, events.TopicFuturesMarkPrice, sym.Name, env)
	}
}
