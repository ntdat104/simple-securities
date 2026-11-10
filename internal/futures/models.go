// Package futures implements the USDⓈ-M perpetual futures service: collateral account, positions,
// orders with synchronous margin checks, fill application, mark/index price, funding and liquidation.
package futures

import (
	"time"

	"github.com/shopspring/decimal"
)

// Account is a user's USDT cross collateral (table futures_accounts).
type Account struct {
	UserID        string          `gorm:"primaryKey"`
	WalletBalance decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	CreatedAt     time.Time       `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt     time.Time       `gorm:"type:timestamptz;not null;default:now()"`
}

// TableName implements gorm.Tabler.
func (Account) TableName() string { return "futures_accounts" }

// Position is the one-way position of a user in a contract together with the user's symbol
// settings (margin type, leverage). A flat position keeps its settings (table positions).
type Position struct {
	UserID         string          `gorm:"primaryKey;index:idx_positions_open,priority:2,where:size <> 0"`
	Symbol         string          `gorm:"primaryKey;index:idx_positions_open,priority:1,where:size <> 0"`
	MarginType     string          `gorm:"not null;default:'CROSSED';check:chk_positions_margin_type,margin_type IN ('CROSSED','ISOLATED')"`
	Leverage       int             `gorm:"not null;default:20;check:chk_positions_leverage,leverage >= 1"`
	Size           decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"` // signed: + long, - short
	EntryPrice     decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	IsolatedMargin decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	Liquidating    bool            `gorm:"not null;default:false"`
	CreatedAt      time.Time       `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt      time.Time       `gorm:"type:timestamptz;not null;default:now()"`
}

// TableName implements gorm.Tabler.
func (Position) TableName() string { return "positions" }

// Order is a futures order (table orders). Status is advanced by matching events only when
// their sequence number is newer than last_seq.
type Order struct {
	ID             string          `gorm:"column:order_id;type:uuid;primaryKey"`
	ClientOrderID  string          `gorm:"not null;uniqueIndex:uq_orders_user_client_order,priority:2"`
	UserID         string          `gorm:"not null;uniqueIndex:uq_orders_user_client_order,priority:1;index:idx_orders_user_open,priority:1,where:status IN ('NEW'\\, 'PARTIALLY_FILLED');index:idx_orders_user_created,priority:1"`
	Symbol         string          `gorm:"not null;index:idx_orders_user_open,priority:2"`
	Side           string          `gorm:"not null;check:chk_orders_side,side IN ('BUY','SELL')"`
	Type           string          `gorm:"not null"`
	TimeInForce    string          `gorm:"column:tif;not null;default:''"`
	Price          decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	Quantity       decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	ExecutedQty    decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	CumQuote       decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	AvgPrice       decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	ReduceOnly     bool            `gorm:"not null;default:false"`
	Liquidation    bool            `gorm:"not null;default:false"`
	Status         string          `gorm:"not null"`
	RejectReason   string          `gorm:"not null;default:''"`
	MarginFrozen   decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	MarginReleased decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	RealizedPnl    decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	LastSeq        int64           `gorm:"not null;default:0"`
	CreatedAt      time.Time       `gorm:"type:timestamptz;not null;default:now();index:idx_orders_user_created,priority:2,sort:desc"`
	UpdatedAt      time.Time       `gorm:"type:timestamptz;not null;default:now()"`
}

// TableName implements gorm.Tabler.
func (Order) TableName() string { return "orders" }

// openMargin is the part of the frozen margin not released yet.
func (o Order) openMargin() decimal.Decimal { return o.MarginFrozen.Sub(o.MarginReleased) }

// Trade is a user's side of a fill (table trades).
type Trade struct {
	ID              int64           `gorm:"primaryKey;autoIncrement"`
	TradeID         int64           `gorm:"not null;uniqueIndex:uq_trades_trade_symbol_order,priority:1"`
	OrderID         string          `gorm:"type:uuid;not null;uniqueIndex:uq_trades_trade_symbol_order,priority:3"`
	UserID          string          `gorm:"not null;index:idx_trades_user_time,priority:1"`
	Symbol          string          `gorm:"not null;uniqueIndex:uq_trades_trade_symbol_order,priority:2"`
	Side            string          `gorm:"not null"`
	Price           decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	Qty             decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	QuoteQty        decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	RealizedPnl     decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	Commission      decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	CommissionAsset string          `gorm:"not null"`
	IsMaker         bool            `gorm:"not null"`
	Time            time.Time       `gorm:"type:timestamptz;not null;index:idx_trades_user_time,priority:2,sort:desc"`
}

// TableName implements gorm.Tabler.
func (Trade) TableName() string { return "trades" }

// Income is an immutable cash-flow record of the futures account (table income).
type Income struct {
	ID         int64           `gorm:"primaryKey;autoIncrement;index:idx_income_user_id,priority:2,sort:desc"`
	UserID     string          `gorm:"not null;index:idx_income_user_id,priority:1"`
	Symbol     string          `gorm:"not null;default:''"`
	IncomeType string          `gorm:"not null"`
	Income     decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	Asset      string          `gorm:"not null;default:'USDT'"`
	Info       string          `gorm:"not null;default:''"`
	Time       time.Time       `gorm:"type:timestamptz;not null;default:now()"`
}

// TableName implements gorm.Tabler.
func (Income) TableName() string { return "income" }

// FundingRecord is a settled funding rate; the primary key makes settlement idempotent
// (table funding_rates).
type FundingRecord struct {
	Symbol      string          `gorm:"primaryKey"`
	FundingTime time.Time       `gorm:"type:timestamptz;primaryKey"`
	Rate        decimal.Decimal `gorm:"column:funding_rate;type:numeric(36,18);not null"`
	MarkPrice   decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	CreatedAt   time.Time       `gorm:"type:timestamptz;not null;default:now()"`
}

// TableName implements gorm.Tabler.
func (FundingRecord) TableName() string { return "funding_rates" }

// InsuranceFund absorbs liquidation surpluses and deficits (table insurance_fund).
type InsuranceFund struct {
	Asset     string          `gorm:"primaryKey"`
	Balance   decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	UpdatedAt time.Time       `gorm:"type:timestamptz;not null;default:now()"`
}

// TableName implements gorm.Tabler.
func (InsuranceFund) TableName() string { return "insurance_fund" }

// Transfer records spot <-> futures collateral movements (table transfers). The primary key
// dedups incoming transfers, the partial unique index dedups outgoing requests by idempotency key.
type Transfer struct {
	TransferID     string          `gorm:"type:uuid;primaryKey"`
	UserID         string          `gorm:"not null;uniqueIndex:uq_transfers_user_key,priority:1,where:idempotency_key <> ''"`
	Direction      string          `gorm:"not null"` // SPOT_TO_FUTURES | FUTURES_TO_SPOT
	Asset          string          `gorm:"not null"`
	Amount         decimal.Decimal `gorm:"type:numeric(36,18);not null;check:chk_transfers_amount,amount > 0"`
	IdempotencyKey string          `gorm:"not null;default:'';uniqueIndex:uq_transfers_user_key,priority:2"`
	CreatedAt      time.Time       `gorm:"type:timestamptz;not null;default:now()"`
}

// TableName implements gorm.Tabler.
func (Transfer) TableName() string { return "transfers" }

// Models returns the service's own models (outbox/inbox are added by the caller).
func Models() []any {
	return []any{&Account{}, &Position{}, &Order{}, &Trade{}, &Income{}, &FundingRecord{},
		&InsuranceFund{}, &Transfer{}}
}
