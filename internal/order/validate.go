package order

import (
	"strings"

	"github.com/shopspring/decimal"

	"simple-securities/pkg/apperr"
	"simple-securities/pkg/symbols"
)

// Order sides, types, time in force values and statuses.
const (
	SideBuy  = "BUY"
	SideSell = "SELL"

	TypeLimit      = "LIMIT"
	TypeMarket     = "MARKET"
	TypeLimitMaker = "LIMIT_MAKER"

	TIFGTC = "GTC"
	TIFIOC = "IOC"
	TIFFOK = "FOK"

	StatusPendingNew      = "PENDING_NEW"
	StatusNew             = "NEW"
	StatusPartiallyFilled = "PARTIALLY_FILLED"
	StatusFilled          = "FILLED"
	StatusCanceled        = "CANCELED"
	StatusExpired         = "EXPIRED"
	StatusRejected        = "REJECTED"
)

const (
	maxClientOrderIDLen = 64
	maxDecimals         = 18
)

// maxValue keeps values (and price*qty) well inside NUMERIC(36,18).
var maxValue = decimal.New(1, 15)

// PlaceInput is a raw place order request.
type PlaceInput struct {
	UserID        string
	Symbol        string
	Side          string
	Type          string
	TimeInForce   string
	Price         string
	Quantity      string
	QuoteQuantity string
	ClientOrderID string
}

// ValidOrder is a validated, normalized place request plus the funds to lock.
type ValidOrder struct {
	Symbol        symbols.Symbol
	Side          string
	Type          string
	TimeInForce   string
	Price         decimal.Decimal
	Quantity      decimal.Decimal
	QuoteQuantity decimal.Decimal
	ClientOrderID string
	LockAsset     string
	LockAmount    decimal.Decimal
}

// Validate checks a place request against the symbol filters and type/TIF combinations.
func Validate(in PlaceInput) (ValidOrder, error) {
	var v ValidOrder
	if strings.TrimSpace(in.UserID) == "" {
		return v, apperr.Invalid("INVALID_USER", "user_id is required")
	}
	sym, ok := symbols.Get(strings.ToUpper(strings.TrimSpace(in.Symbol)))
	if !ok {
		return v, apperr.Invalid("UNKNOWN_SYMBOL", "unknown symbol %q", in.Symbol)
	}
	v.Symbol = sym
	v.ClientOrderID = strings.TrimSpace(in.ClientOrderID)
	if len(v.ClientOrderID) > maxClientOrderIDLen {
		return v, apperr.Invalid("INVALID_CLIENT_ORDER_ID", "client_order_id longer than %d", maxClientOrderIDLen)
	}

	v.Side = strings.ToUpper(strings.TrimSpace(in.Side))
	if v.Side != SideBuy && v.Side != SideSell {
		return v, apperr.Invalid("INVALID_SIDE", "side must be BUY or SELL")
	}

	price, err := parseAmount(in.Price, "price")
	if err != nil {
		return v, err
	}
	qty, err := parseAmount(in.Quantity, "quantity")
	if err != nil {
		return v, err
	}
	quote, err := parseAmount(in.QuoteQuantity, "quote_quantity")
	if err != nil {
		return v, err
	}

	v.Type = strings.ToUpper(strings.TrimSpace(in.Type))
	tif := strings.ToUpper(strings.TrimSpace(in.TimeInForce))
	switch v.Type {
	case TypeLimit:
		switch tif {
		case "":
			tif = TIFGTC
		case TIFGTC, TIFIOC, TIFFOK:
		default:
			return v, apperr.Invalid("INVALID_TIME_IN_FORCE", "time_in_force must be GTC, IOC or FOK")
		}
		if err := requirePriceQty(price, qty); err != nil {
			return v, err
		}
		v.TimeInForce, v.Price, v.Quantity = tif, price, qty
	case TypeLimitMaker:
		if tif != "" && tif != TIFGTC {
			return v, apperr.Invalid("INVALID_TIME_IN_FORCE", "LIMIT_MAKER does not support time_in_force %s", tif)
		}
		if err := requirePriceQty(price, qty); err != nil {
			return v, err
		}
		v.TimeInForce, v.Price, v.Quantity = TIFGTC, price, qty
	case TypeMarket:
		if v.Side == SideBuy {
			if !qty.IsZero() {
				return v, apperr.Invalid("MARKET_BUY_REQUIRES_QUOTE_QTY", "MARKET BUY takes quote_quantity, not quantity")
			}
			if !quote.IsPositive() {
				return v, apperr.Invalid("MARKET_BUY_REQUIRES_QUOTE_QTY", "MARKET BUY requires a positive quote_quantity")
			}
			v.QuoteQuantity = quote
		} else {
			if !quote.IsZero() {
				return v, apperr.Invalid("MARKET_SELL_REQUIRES_QTY", "MARKET SELL takes quantity, not quote_quantity")
			}
			if !qty.IsPositive() {
				return v, apperr.Invalid("MARKET_SELL_REQUIRES_QTY", "MARKET SELL requires a positive quantity")
			}
			v.Quantity = qty
		}
	default:
		return v, apperr.Invalid("INVALID_ORDER_TYPE", "type must be LIMIT, MARKET or LIMIT_MAKER")
	}

	if v.Price.IsPositive() && !symbols.IsMultiple(v.Price, sym.TickSize) {
		return v, apperr.Invalid("INVALID_PRICE_TICK", "price must be a multiple of tick size %s", sym.TickSize)
	}
	if v.Quantity.IsPositive() && !symbols.IsMultiple(v.Quantity, sym.StepSize) {
		return v, apperr.Invalid("INVALID_QUANTITY_STEP", "quantity must be a multiple of step size %s", sym.StepSize)
	}

	switch {
	case v.Type == TypeMarket && v.Side == SideBuy:
		if v.QuoteQuantity.LessThan(sym.MinNotional) {
			return v, apperr.Invalid("MIN_NOTIONAL", "quote_quantity below min notional %s", sym.MinNotional)
		}
		v.LockAsset, v.LockAmount = sym.Quote, v.QuoteQuantity
	case v.Type == TypeMarket: // SELL: notional unknown up front
		v.LockAsset, v.LockAmount = sym.Base, v.Quantity
	default:
		notional := v.Price.Mul(v.Quantity)
		if notional.LessThan(sym.MinNotional) {
			return v, apperr.Invalid("MIN_NOTIONAL", "price*quantity below min notional %s", sym.MinNotional)
		}
		if notional.GreaterThan(maxValue) {
			return v, apperr.Invalid("MAX_NOTIONAL", "price*quantity too large")
		}
		if v.Side == SideBuy {
			v.LockAsset, v.LockAmount = sym.Quote, notional
		} else {
			v.LockAsset, v.LockAmount = sym.Base, v.Quantity
		}
	}
	return v, nil
}

func requirePriceQty(price, qty decimal.Decimal) error {
	if !price.IsPositive() {
		return apperr.Invalid("INVALID_PRICE", "a positive price is required")
	}
	if !qty.IsPositive() {
		return apperr.Invalid("INVALID_QUANTITY", "a positive quantity is required")
	}
	return nil
}

// parseAmount parses an optional non-negative decimal ("" means zero).
func parseAmount(s, name string) (decimal.Decimal, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return decimal.Zero, nil
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, apperr.Invalid("INVALID_"+strings.ToUpper(name), "%s is not a decimal", name)
	}
	if d.IsNegative() || d.GreaterThan(maxValue) || d.Exponent() < -maxDecimals {
		return decimal.Zero, apperr.Invalid("INVALID_"+strings.ToUpper(name), "%s is out of range", name)
	}
	return d, nil
}
