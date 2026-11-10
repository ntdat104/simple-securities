// Package domain contains the pure wallet rules: amount validation, balance arithmetic and the
// settlement math of a trade. It has no I/O so it can be unit tested exhaustively.
package domain

import (
	"errors"
	"fmt"
	"sort"

	"github.com/shopspring/decimal"

	"simple-securities/pkg/events"
)

// FeeAccountID is the pseudo user that receives trading fees.
const FeeAccountID = "00000000-0000-0000-0000-000000000001"

// Ledger reference types.
const (
	RefDeposit     = "DEPOSIT"
	RefWithdraw    = "WITHDRAW"
	RefOrderLock   = "ORDER_LOCK"
	RefOrderUnlock = "ORDER_UNLOCK"
	RefTrade       = "TRADE"
	RefFee         = "FEE"
	RefAirdrop     = "AIRDROP"

	RefFuturesTransferOut = "FUTURES_TRANSFER_OUT" // spot -> futures (ref_id = transfer id)
	RefFuturesTransferIn  = "FUTURES_TRANSFER_IN"  // futures -> spot (ref_id = transfer id)
)

// Order lock statuses.
const (
	LockActive   = "ACTIVE"
	LockReleased = "RELEASED"
)

// QuoteAsset is listed first in balance listings.
const QuoteAsset = "USDT"

// maxScale is the number of fractional digits of NUMERIC(36,18).
const maxScale = 18

// maxAmount bounds any single amount so it fits NUMERIC(36,18) (18 integer digits).
var maxAmount = decimal.New(1, 18)

var (
	// ErrInsufficient means a movement would make free or locked negative.
	ErrInsufficient = errors.New("insufficient balance")
	// ErrInvalidAmount means an amount is not a valid positive decimal.
	ErrInvalidAmount = errors.New("invalid amount")
)

// ParseAmount parses a strictly positive decimal with at most 18 fractional digits, below max
// (max zero means no extra bound).
func ParseAmount(s string, max decimal.Decimal) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, fmt.Errorf("%w: %q", ErrInvalidAmount, s)
	}
	if !d.IsPositive() {
		return decimal.Zero, fmt.Errorf("%w: must be positive", ErrInvalidAmount)
	}
	if !d.Equal(d.Truncate(maxScale)) {
		return decimal.Zero, fmt.Errorf("%w: at most %d decimal places", ErrInvalidAmount, maxScale)
	}
	if d.GreaterThanOrEqual(maxAmount) || (!max.IsZero() && d.GreaterThan(max)) {
		return decimal.Zero, fmt.Errorf("%w: too large", ErrInvalidAmount)
	}
	return d, nil
}

// Key identifies a balance row.
type Key struct {
	UserID string
	Asset  string
}

// Less orders keys by (user_id, asset); rows are always locked in this order.
func (k Key) Less(o Key) bool {
	if k.UserID != o.UserID {
		return k.UserID < o.UserID
	}
	return k.Asset < o.Asset
}

// SortKeys sorts and de-duplicates keys.
func SortKeys(keys []Key) []Key {
	sort.Slice(keys, func(i, j int) bool { return keys[i].Less(keys[j]) })
	out := keys[:0]
	for i, k := range keys {
		if i == 0 || k != keys[i-1] {
			out = append(out, k)
		}
	}
	return out
}

// Balance is the state of one (user, asset) row.
type Balance struct {
	Free   decimal.Decimal
	Locked decimal.Decimal
}

// Movement is a signed change of one balance row.
type Movement struct {
	UserID string
	Asset  string
	Free   decimal.Decimal
	Locked decimal.Decimal
}

// Key returns the balance row touched by the movement.
func (m Movement) Key() Key { return Key{UserID: m.UserID, Asset: m.Asset} }

// IsZero reports whether the movement changes nothing.
func (m Movement) IsZero() bool { return m.Free.IsZero() && m.Locked.IsZero() }

// Apply returns b after m, or ErrInsufficient if free or locked would become negative.
func (b Balance) Apply(m Movement) (Balance, error) {
	out := Balance{Free: b.Free.Add(m.Free), Locked: b.Locked.Add(m.Locked)}
	if out.Free.IsNegative() || out.Locked.IsNegative() {
		return b, fmt.Errorf("%w: %s", ErrInsufficient, m.Asset)
	}
	return out, nil
}

// LockMovement moves amount from free to locked.
func LockMovement(userID, asset string, amount decimal.Decimal) Movement {
	return Movement{UserID: userID, Asset: asset, Free: amount.Neg(), Locked: amount}
}

// UnlockMovement moves amount from locked back to free.
func UnlockMovement(userID, asset string, amount decimal.Decimal) Movement {
	return Movement{UserID: userID, Asset: asset, Free: amount, Locked: amount.Neg()}
}

// Fee is a trading fee collected by the exchange.
type Fee struct {
	Asset  string
	Amount decimal.Decimal
}

// LockDecrement reduces an order lock's remaining amount after a fill.
type LockDecrement struct {
	OrderID string
	Amount  decimal.Decimal
}

// TradeSettlement is everything a trade changes in the wallet.
type TradeSettlement struct {
	Movements []Movement // user balance changes (zero movements omitted)
	Fees      []Fee      // credited to the fee account (zero fees omitted)
	Locks     []LockDecrement
}

// SettleTrade computes the balance deltas of a trade:
//
//	buyer:  locked quote -= quote_qty, free base  += qty - buyer_fee
//	seller: locked base  -= qty,       free quote += quote_qty - seller_fee
//
// and decrements the buyer's lock by quote_qty and the seller's lock by qty.
func SettleTrade(t events.TradeExecuted, base, quote string) (TradeSettlement, error) {
	qty, err := nonNegative("qty", t.Qty, true)
	if err != nil {
		return TradeSettlement{}, err
	}
	quoteQty, err := nonNegative("quote_qty", t.QuoteQty, true)
	if err != nil {
		return TradeSettlement{}, err
	}
	buyerFee, err := nonNegative("buyer_fee", t.BuyerFee, false)
	if err != nil {
		return TradeSettlement{}, err
	}
	sellerFee, err := nonNegative("seller_fee", t.SellerFee, false)
	if err != nil {
		return TradeSettlement{}, err
	}
	if buyerFee.GreaterThan(qty) || sellerFee.GreaterThan(quoteQty) {
		return TradeSettlement{}, fmt.Errorf("trade %d: fee exceeds proceeds", t.TradeID)
	}
	if t.BuyerUserID == "" || t.SellerUserID == "" {
		return TradeSettlement{}, fmt.Errorf("trade %d: missing user id", t.TradeID)
	}

	var s TradeSettlement
	for _, m := range []Movement{
		{UserID: t.BuyerUserID, Asset: quote, Locked: quoteQty.Neg()},
		{UserID: t.BuyerUserID, Asset: base, Free: qty.Sub(buyerFee)},
		{UserID: t.SellerUserID, Asset: base, Locked: qty.Neg()},
		{UserID: t.SellerUserID, Asset: quote, Free: quoteQty.Sub(sellerFee)},
	} {
		if !m.IsZero() {
			s.Movements = append(s.Movements, m)
		}
	}
	if buyerFee.IsPositive() {
		s.Fees = append(s.Fees, Fee{Asset: base, Amount: buyerFee})
	}
	if sellerFee.IsPositive() {
		s.Fees = append(s.Fees, Fee{Asset: quote, Amount: sellerFee})
	}
	s.Locks = []LockDecrement{
		{OrderID: t.BuyerOrderID, Amount: quoteQty},
		{OrderID: t.SellerOrderID, Amount: qty},
	}
	return s, nil
}

// DecrementRemaining subtracts dec from remaining, clamping at zero. clamped reports an overdraw,
// which indicates a bug upstream (a fill larger than what was locked).
func DecrementRemaining(remaining, dec decimal.Decimal) (out decimal.Decimal, clamped bool) {
	out = remaining.Sub(dec)
	if out.IsNegative() {
		return decimal.Zero, true
	}
	return out, false
}

// SortAssets orders assets with USDT first, then alphabetically.
func SortAssets(assets []string) {
	sort.Slice(assets, func(i, j int) bool {
		if (assets[i] == QuoteAsset) != (assets[j] == QuoteAsset) {
			return assets[i] == QuoteAsset
		}
		return assets[i] < assets[j]
	})
}

func nonNegative(name, s string, positive bool) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid %s %q: %w", name, s, err)
	}
	if d.IsNegative() || (positive && d.IsZero()) {
		return decimal.Zero, fmt.Errorf("invalid %s %q", name, s)
	}
	return d, nil
}
