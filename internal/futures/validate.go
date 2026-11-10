package futures

import (
	"strings"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"

	"simple-securities/pkg/apperr"
	"simple-securities/pkg/symbols"
)

// ServiceName is the consumer group, inbox consumer, event source and outbox service name.
const ServiceName = "futures"

// Asset is the only collateral asset.
const Asset = "USDT"

// Order sides, types, time in force values and statuses.
const (
	SideBuy  = "BUY"
	SideSell = "SELL"

	TypeLimit  = "LIMIT"
	TypeMarket = "MARKET"

	engineTypeLimitMaker = "LIMIT_MAKER"

	TIFGTC = "GTC"
	TIFIOC = "IOC"
	TIFFOK = "FOK"
	TIFGTX = "GTX" // post-only

	StatusNew             = "NEW"
	StatusPartiallyFilled = "PARTIALLY_FILLED"
	StatusFilled          = "FILLED"
	StatusCanceled        = "CANCELED"
	StatusExpired         = "EXPIRED"
	StatusRejected        = "REJECTED"
)

// Margin types.
const (
	MarginCrossed  = "CROSSED"
	MarginIsolated = "ISOLATED"
)

// Income types.
const (
	IncomeTransfer       = "TRANSFER"
	IncomeRealizedPnl    = "REALIZED_PNL"
	IncomeCommission     = "COMMISSION"
	IncomeFundingFee     = "FUNDING_FEE"
	IncomeInsuranceClear = "INSURANCE_CLEAR"
	IncomeIsolatedMargin = "ISOLATED_MARGIN"
)

// Account update reasons (events.FuturesAccountUpdate.Reason).
const (
	reasonOrder          = "ORDER"
	reasonFunding        = "FUNDING_FEE"
	reasonDeposit        = "DEPOSIT"
	reasonWithdraw       = "WITHDRAW"
	reasonMarginTransfer = "MARGIN_TRANSFER"
	reasonLiquidation    = "LIQUIDATION"
)

// Transfer directions.
const (
	directionSpotToFutures = "SPOT_TO_FUTURES"
	directionFuturesToSpot = "FUTURES_TO_SPOT"
)

const (
	maxClientOrderIDLen = 64
	maxDecimals         = 18
	moneyPlaces         = 8
	defaultLeverage     = 20
)

var (
	// maxValue keeps values (and price*qty) well inside NUMERIC(36,18).
	maxValue = decimal.New(1, 15)

	openStatuses = []string{StatusNew, StatusPartiallyFilled}
)

// money truncates a PnL / fee / margin amount to 8 decimals (toward zero).
func money(d decimal.Decimal) decimal.Decimal { return d.Truncate(moneyPlaces) }

// errMarkUnavailable is returned when no mark, index or last price is known for a contract.
func errMarkUnavailable(symbol string) error {
	return &apperr.Error{Code: codes.Unavailable, Reason: "MARK_PRICE_UNAVAILABLE",
		Msg: "no mark price available for " + symbol}
}

// PlaceInput is a raw place order request.
type PlaceInput struct {
	UserID        string
	Symbol        string
	Side          string
	Type          string
	TimeInForce   string
	Price         string
	Quantity      string
	ReduceOnly    bool
	ClientOrderID string
}

// validOrder is a syntactically valid order (contract filters checked except MARKET notional,
// which needs the mark price).
type validOrder struct {
	Symbol        symbols.Symbol
	Side          string
	Type          string
	TimeInForce   string
	Price         decimal.Decimal
	Quantity      decimal.Decimal
	ReduceOnly    bool
	ClientOrderID string
}

func requireUser(userID string) error {
	if strings.TrimSpace(userID) == "" {
		return apperr.Invalid("INVALID_USER", "user_id is required")
	}
	return nil
}

// lookupSymbol returns the futures contract.
func lookupSymbol(name string) (symbols.Symbol, error) {
	sym, ok := symbols.GetMarket(symbols.MarketFutures, strings.ToUpper(strings.TrimSpace(name)))
	if !ok {
		return symbols.Symbol{}, apperr.Invalid("INVALID_SYMBOL", "unknown futures symbol %q", name)
	}
	return sym, nil
}

// validateOrder checks a place request against the contract filters and type/TIF combinations.
func validateOrder(in PlaceInput) (validOrder, error) {
	var v validOrder
	if err := requireUser(in.UserID); err != nil {
		return v, err
	}
	sym, err := lookupSymbol(in.Symbol)
	if err != nil {
		return v, err
	}
	v.Symbol, v.ReduceOnly = sym, in.ReduceOnly
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
	if !qty.IsPositive() {
		return v, apperr.Invalid("INVALID_QUANTITY", "a positive quantity is required")
	}
	if !symbols.IsMultiple(qty, sym.StepSize) {
		return v, apperr.Invalid("INVALID_QUANTITY_STEP", "quantity must be a multiple of step size %s", sym.StepSize)
	}
	v.Quantity = qty

	v.Type = strings.ToUpper(strings.TrimSpace(in.Type))
	tif := strings.ToUpper(strings.TrimSpace(in.TimeInForce))
	switch v.Type {
	case TypeLimit:
		switch tif {
		case "":
			tif = TIFGTC
		case TIFGTC, TIFIOC, TIFFOK, TIFGTX:
		default:
			return v, apperr.Invalid("INVALID_TIF", "time_in_force must be GTC, IOC, FOK or GTX")
		}
		if !price.IsPositive() {
			return v, apperr.Invalid("INVALID_PRICE", "a positive price is required")
		}
		if !symbols.IsMultiple(price, sym.TickSize) {
			return v, apperr.Invalid("INVALID_PRICE_TICK", "price must be a multiple of tick size %s", sym.TickSize)
		}
		notional := price.Mul(qty)
		if notional.GreaterThan(maxValue) {
			return v, apperr.Invalid("MAX_NOTIONAL", "price*quantity too large")
		}
		if !v.ReduceOnly && notional.LessThan(sym.MinNotional) {
			return v, apperr.Invalid("MIN_NOTIONAL", "price*quantity below min notional %s", sym.MinNotional)
		}
		v.Price = price
	case TypeMarket:
		if tif != "" && tif != TIFIOC && tif != TIFGTC {
			return v, apperr.Invalid("INVALID_TIF", "MARKET orders do not take time_in_force %s", tif)
		}
		if !price.IsZero() {
			return v, apperr.Invalid("INVALID_PRICE", "MARKET orders do not take a price")
		}
		tif = TIFIOC
	default:
		return v, apperr.Invalid("INVALID_TYPE", "type must be LIMIT or MARKET")
	}
	v.TimeInForce = tif
	return v, nil
}

// checkMarketNotional applies minNotional to a MARKET order using the mark price.
func checkMarketNotional(v validOrder, mark decimal.Decimal) error {
	if v.Type != TypeMarket || v.ReduceOnly {
		return nil
	}
	if v.Quantity.Mul(mark).LessThan(v.Symbol.MinNotional) {
		return apperr.Invalid("MIN_NOTIONAL", "quantity*mark price below min notional %s", v.Symbol.MinNotional)
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

// parseMoney parses a positive amount with at most 8 decimals.
func parseMoney(s string) (decimal.Decimal, error) {
	d, err := parseAmount(s, "amount")
	if err != nil {
		return decimal.Zero, err
	}
	if !d.IsPositive() || d.Exponent() < -moneyPlaces {
		return decimal.Zero, apperr.Invalid("INVALID_AMOUNT", "amount must be positive with at most %d decimals", moneyPlaces)
	}
	return d, nil
}

// normalizeMarginType accepts CROSSED/CROSS and ISOLATED (case-insensitive).
func normalizeMarginType(s string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "CROSSED", "CROSS":
		return MarginCrossed, nil
	case "ISOLATED":
		return MarginIsolated, nil
	}
	return "", apperr.Invalid("INVALID_MARGIN_TYPE", "margin_type must be CROSSED or ISOLATED")
}

// defaultLeverageFor caps the default leverage to the contract maximum.
func defaultLeverageFor(sym symbols.Symbol) int {
	if sym.MaxLeverage > 0 && sym.MaxLeverage < defaultLeverage {
		return sym.MaxLeverage
	}
	return defaultLeverage
}

// sideSign returns +1 for BUY and -1 for SELL.
func sideSign(side string) int {
	if side == SideBuy {
		return 1
	}
	return -1
}

func oppositeSide(side string) string {
	if side == SideBuy {
		return SideSell
	}
	return SideBuy
}

func isTerminal(status string) bool {
	switch status {
	case StatusFilled, StatusCanceled, StatusExpired, StatusRejected:
		return true
	}
	return false
}
