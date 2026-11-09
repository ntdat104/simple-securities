package futures

import (
	"github.com/shopspring/decimal"
)

// fillResult is the effect of one fill on a position and on the cross wallet.
type fillResult struct {
	Position    Position
	WalletDelta decimal.Decimal
	Realized    decimal.Decimal
}

// computeFill applies a signed fill q (+BUY / -SELL) at price with fee to a position
// (spec 3 "Applying fills", steps 2-4). It is pure: the caller persists the result.
//
// Liquidation fills of an ISOLATED position settle PnL and fee inside the isolated margin (the
// remainder goes to the insurance fund when the liquidation completes) instead of the wallet.
func computeFill(pos Position, q, price, fee decimal.Decimal, liquidation bool) fillResult {
	res := fillResult{Position: pos}
	p := &res.Position
	isolated := p.MarginType == MarginIsolated
	settleInIsolated := liquidation && isolated

	if !p.Size.IsZero() && p.Size.Sign() != q.Sign() {
		size := p.Size.Abs()
		c := decimal.Min(q.Abs(), size)
		realized := money(c.Mul(price.Sub(p.EntryPrice)).Mul(decimal.NewFromInt(int64(p.Size.Sign()))))
		res.Realized = realized
		switch {
		case settleInIsolated:
			p.IsolatedMargin = p.IsolatedMargin.Add(realized)
		case isolated:
			release := p.IsolatedMargin
			if c.LessThan(size) {
				release = money(p.IsolatedMargin.Mul(c).Div(size))
			}
			p.IsolatedMargin = p.IsolatedMargin.Sub(release)
			res.WalletDelta = res.WalletDelta.Add(release).Add(realized)
		default:
			res.WalletDelta = res.WalletDelta.Add(realized)
		}
		rest := q.Abs().Sub(size)
		if rest.IsPositive() { // flip: the rest opens a new position at price
			p.Size, p.EntryPrice = decimal.Zero, decimal.Zero
			res.WalletDelta = res.WalletDelta.Add(openPosition(p, rest.Mul(decimal.NewFromInt(int64(q.Sign()))), price))
		} else {
			p.Size = p.Size.Add(q)
			if p.Size.IsZero() {
				p.EntryPrice = decimal.Zero
			}
		}
	} else {
		res.WalletDelta = res.WalletDelta.Add(openPosition(p, q, price))
	}

	if settleInIsolated {
		p.IsolatedMargin = p.IsolatedMargin.Sub(fee)
	} else {
		res.WalletDelta = res.WalletDelta.Sub(fee)
	}
	return res
}

// openPosition opens/increases a position (same direction or flat) and returns the wallet delta
// (the margin moved into an ISOLATED position).
func openPosition(p *Position, q, price decimal.Decimal) decimal.Decimal {
	size := p.Size.Abs()
	newSize := size.Add(q.Abs())
	p.EntryPrice = size.Mul(p.EntryPrice).Add(q.Abs().Mul(price)).DivRound(newSize, maxDecimals)
	p.Size = p.Size.Add(q)
	if p.MarginType != MarginIsolated {
		return decimal.Zero
	}
	margin := money(q.Abs().Mul(price).Div(leverageOf(*p)))
	p.IsolatedMargin = p.IsolatedMargin.Add(margin)
	return margin.Neg()
}

// releaseForFill is the part of an order's frozen margin released by a fill of fillQty.
func releaseForFill(o Order, fillQty decimal.Decimal) decimal.Decimal {
	if !o.Quantity.IsPositive() {
		return decimal.Zero
	}
	release := money(o.MarginFrozen.Mul(fillQty).Div(o.Quantity))
	return decimal.Min(release, o.openMargin())
}
