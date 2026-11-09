package futures

import (
	"github.com/shopspring/decimal"

	"simple-securities/pkg/symbols"
)

// PriceFunc returns the risk price (mark, falling back to index / last) of a contract.
type PriceFunc func(symbol string) (decimal.Decimal, bool)

// positionRisk holds the derived values of one position at mark price M.
type positionRisk struct {
	Mark     decimal.Decimal
	Notional decimal.Decimal // |size| * M
	UPnL     decimal.Decimal // size * (M - entry)
	IM       decimal.Decimal // notional / leverage
	MM       decimal.Decimal // notional * mmr
}

// evalPosition computes notional, uPnL and margins of a position.
func evalPosition(p Position, sym symbols.Symbol, mark decimal.Decimal) positionRisk {
	notional := money(p.Size.Abs().Mul(mark))
	return positionRisk{
		Mark:     mark,
		Notional: notional,
		UPnL:     money(p.Size.Mul(mark.Sub(p.EntryPrice))),
		IM:       money(notional.Div(leverageOf(p))),
		MM:       money(notional.Mul(sym.MaintMarginRate)),
	}
}

func leverageOf(p Position) decimal.Decimal {
	return decimal.NewFromInt(int64(max(p.Leverage, 1)))
}

// accountRisk aggregates the account formulas of the spec (section 3).
type accountRisk struct {
	WalletBalance decimal.Decimal
	CrossUPnL     decimal.Decimal
	CrossIM       decimal.Decimal
	CrossMM       decimal.Decimal
	IsoMargin     decimal.Decimal
	IsoUPnL       decimal.Decimal
	TotalIM       decimal.Decimal
	TotalMM       decimal.Decimal
	OrderMargin   decimal.Decimal
	Available     decimal.Decimal // wallet + cross uPnL - cross IM - order margin, floored at 0
	MaxWithdraw   decimal.Decimal // min(available, wallet - cross IM - order margin), floored at 0
	MarginBalance decimal.Decimal // wallet + cross uPnL + isolated margin + isolated uPnL (display)
	Positions     map[string]positionRisk
}

// UPnL returns the total unrealized PnL (cross + isolated).
func (r accountRisk) UPnL() decimal.Decimal { return r.CrossUPnL.Add(r.IsoUPnL) }

// computeRisk evaluates an account. With strict=true a non-flat position without any price fails
// with MARK_PRICE_UNAVAILABLE (risk checks); otherwise the entry price is used (display).
func computeRisk(wallet decimal.Decimal, positions []Position, orderMargin decimal.Decimal,
	price PriceFunc, strict bool) (accountRisk, error) {
	r := accountRisk{WalletBalance: wallet, OrderMargin: orderMargin, Positions: map[string]positionRisk{}}
	for _, p := range positions {
		if p.Size.IsZero() {
			continue
		}
		sym, ok := symbols.GetMarket(symbols.MarketFutures, p.Symbol)
		if !ok {
			continue
		}
		mark, ok := price(p.Symbol)
		if !ok {
			if strict {
				return accountRisk{}, errMarkUnavailable(p.Symbol)
			}
			mark = p.EntryPrice
		}
		pr := evalPosition(p, sym, mark)
		r.Positions[p.Symbol] = pr
		r.TotalIM = r.TotalIM.Add(pr.IM)
		r.TotalMM = r.TotalMM.Add(pr.MM)
		if p.MarginType == MarginIsolated {
			r.IsoMargin = r.IsoMargin.Add(p.IsolatedMargin)
			r.IsoUPnL = r.IsoUPnL.Add(pr.UPnL)
			continue
		}
		r.CrossUPnL = r.CrossUPnL.Add(pr.UPnL)
		r.CrossIM = r.CrossIM.Add(pr.IM)
		r.CrossMM = r.CrossMM.Add(pr.MM)
	}
	r.Available = floorZero(wallet.Add(r.CrossUPnL).Sub(r.CrossIM).Sub(orderMargin))
	r.MaxWithdraw = floorZero(decimal.Min(r.Available, wallet.Sub(r.CrossIM).Sub(orderMargin)))
	r.MarginBalance = wallet.Add(r.CrossUPnL).Add(r.IsoMargin).Add(r.IsoUPnL)
	return r, nil
}

func floorZero(d decimal.Decimal) decimal.Decimal {
	if d.IsNegative() {
		return decimal.Zero
	}
	return d
}

// liquidationPrice implements the displayed liquidation price of the spec ("0" when none).
func liquidationPrice(p Position, sym symbols.Symbol, r accountRisk) decimal.Decimal {
	if p.Size.IsZero() {
		return decimal.Zero
	}
	var avail decimal.Decimal
	if p.MarginType == MarginIsolated {
		avail = p.IsolatedMargin
	} else {
		own := r.Positions[p.Symbol]
		avail = r.WalletBalance.Add(r.CrossUPnL.Sub(own.UPnL)).Sub(r.CrossMM.Sub(own.MM))
	}
	abs := p.Size.Abs()
	one := decimal.NewFromInt(1)
	var num, den decimal.Decimal
	if p.Size.IsPositive() {
		num = abs.Mul(p.EntryPrice).Sub(avail)
		den = abs.Mul(one.Sub(sym.MaintMarginRate))
	} else {
		num = abs.Mul(p.EntryPrice).Add(avail)
		den = abs.Mul(one.Add(sym.MaintMarginRate))
	}
	if !den.IsPositive() {
		return decimal.Zero
	}
	lp := num.DivRound(den, moneyPlaces)
	if !lp.IsPositive() {
		return decimal.Zero
	}
	return lp
}

// breakEvenPrice is the entry price adjusted for the taker fee of opening and closing.
func breakEvenPrice(p Position, sym symbols.Symbol) decimal.Decimal {
	if p.Size.IsZero() {
		return decimal.Zero
	}
	one := decimal.NewFromInt(1)
	ft := sym.TakerFee
	if p.Size.IsPositive() {
		return p.EntryPrice.Mul(one.Add(ft)).DivRound(one.Sub(ft), moneyPlaces)
	}
	return p.EntryPrice.Mul(one.Sub(ft)).DivRound(one.Add(ft), moneyPlaces)
}

// liquidationTargets returns the symbols of the positions that must be liquidated: isolated
// positions with isolated_margin + uPnL <= MM, and every cross position when
// wallet + cross uPnL <= cross MM. Positions without a price are ignored.
func liquidationTargets(wallet decimal.Decimal, positions []Position, price PriceFunc) []string {
	var (
		out                []string
		cross              []string
		crossUPnL, crossMM decimal.Decimal
		crossAllPriced     = true
	)
	for _, p := range positions {
		if p.Size.IsZero() {
			continue
		}
		sym, ok := symbols.GetMarket(symbols.MarketFutures, p.Symbol)
		if !ok {
			continue
		}
		mark, ok := price(p.Symbol)
		if !ok {
			if p.MarginType == MarginCrossed {
				crossAllPriced = false
			}
			continue
		}
		pr := evalPosition(p, sym, mark)
		if p.MarginType == MarginIsolated {
			if p.IsolatedMargin.Add(pr.UPnL).LessThanOrEqual(pr.MM) {
				out = append(out, p.Symbol)
			}
			continue
		}
		cross = append(cross, p.Symbol)
		crossUPnL = crossUPnL.Add(pr.UPnL)
		crossMM = crossMM.Add(pr.MM)
	}
	if len(cross) > 0 && crossAllPriced && wallet.Add(crossUPnL).LessThanOrEqual(crossMM) {
		out = append(out, cross...)
	}
	return out
}
