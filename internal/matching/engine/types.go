// Package engine is a pure, deterministic, in-memory spot matching engine with price-time priority.
// It performs no I/O: callers feed it commands and persist / publish the returned Result.
package engine

import (
	"time"

	"github.com/shopspring/decimal"
)

// Side of an order.
type Side string

// Order sides.
const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

// OrderType of an order.
type OrderType string

// Supported order types.
const (
	TypeLimit      OrderType = "LIMIT"
	TypeMarket     OrderType = "MARKET"
	TypeLimitMaker OrderType = "LIMIT_MAKER"
)

// TimeInForce of a limit order.
type TimeInForce string

// Supported time in force values.
const (
	GTC TimeInForce = "GTC"
	IOC TimeInForce = "IOC"
	FOK TimeInForce = "FOK"
)

// Status of an order as reported by the engine.
type Status string

// Order statuses.
const (
	StatusNew             Status = "NEW"
	StatusPartiallyFilled Status = "PARTIALLY_FILLED"
	StatusFilled          Status = "FILLED"
	StatusCanceled        Status = "CANCELED"
	StatusExpired         Status = "EXPIRED"
	StatusRejected        Status = "REJECTED"
)

// Reasons attached to status changes.
const (
	ReasonPostOnlyWouldCross  = "POST_ONLY_WOULD_CROSS"
	ReasonSelfTradePrevention = "SELF_TRADE_PREVENTION"
	ReasonInvalidOrder        = "INVALID_ORDER"
)

// feeScale is the number of decimals fees are truncated to.
const feeScale = 8

// Order is an order known to the engine. For a MARKET BUY spending a quote budget, Qty/Remaining
// are zero and QuoteBudget holds the quote amount to spend. On FeeInQuote (futures) symbols a
// MARKET BUY may instead carry a Qty and no QuoteBudget: it then matches by quantity like a SELL.
type Order struct {
	ID          string
	UserID      string
	Side        Side
	Type        OrderType
	TIF         TimeInForce
	Price       decimal.Decimal
	Qty         decimal.Decimal // original quantity
	Remaining   decimal.Decimal
	Executed    decimal.Decimal
	CumQuote    decimal.Decimal
	QuoteBudget decimal.Decimal // MARKET BUY only
	Priority    int64           // time priority (book sequence when the order was accepted)
	CreatedAt   time.Time

	byQuote bool // MARKET BUY matched against QuoteBudget (set by Place, never persisted)
}

// isMarketBuy reports whether o is a MARKET BUY matched by quote budget.
func (o *Order) isMarketBuy() bool { return o.byQuote }

// Trade is a single fill. Price is always the maker price.
type Trade struct {
	ID            int64
	Seq           int64
	Symbol        string
	Price         decimal.Decimal
	Qty           decimal.Decimal
	QuoteQty      decimal.Decimal
	BuyerOrderID  string
	SellerOrderID string
	BuyerUserID   string
	SellerUserID  string
	BuyerIsMaker  bool
	BuyerFee      decimal.Decimal // base asset (quote asset when the symbol is FeeInQuote)
	SellerFee     decimal.Decimal // quote asset
	Time          time.Time
}

// StatusChange reports the new state of an order touched by a command.
type StatusChange struct {
	Seq         int64
	OrderID     string
	UserID      string
	Symbol      string
	Status      Status
	ExecutedQty decimal.Decimal
	CumQuoteQty decimal.Decimal
	Reason      string
	Time        time.Time
}

// Result is the deterministic output of one command.
type Result struct {
	Seq     int64          // book update id assigned to this command (0 if nothing happened)
	Trades  []Trade        // in execution order
	Updates []StatusChange // makers in touch order, then the taker
	Resting []Order        // touched orders that are (still) resting: persist (upsert)
	Removed []string       // touched orders that left the book: delete from persistence
}

// Empty reports whether the command produced nothing (duplicate place, unknown cancel).
func (r Result) Empty() bool { return r.Seq == 0 }

// Level is an aggregated price level.
type Level struct {
	Price decimal.Decimal
	Qty   decimal.Decimal
}

// Depth is an aggregated view of the top of the book.
type Depth struct {
	LastUpdateID int64
	Bids         []Level // best (highest) first
	Asks         []Level // best (lowest) first
}
