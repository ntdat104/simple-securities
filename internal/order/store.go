// Package order implements the order service.
package order

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"simple-securities/pkg/events"
)

// Order is the persisted order (table orders).
type Order struct {
	ID            string          `gorm:"column:order_id;type:uuid;primaryKey"`
	ClientOrderID string          `gorm:"not null;uniqueIndex:uq_orders_user_client_order,priority:2"`
	UserID        string          `gorm:"not null;uniqueIndex:uq_orders_user_client_order,priority:1;index:idx_orders_user_open,priority:1,where:status IN ('PENDING_NEW'\\, 'NEW'\\, 'PARTIALLY_FILLED');index:idx_orders_user_created,priority:1"`
	Symbol        string          `gorm:"not null"`
	Side          string          `gorm:"not null"`
	Type          string          `gorm:"not null"`
	TimeInForce   string          `gorm:"column:tif;not null;default:''"`
	Price         decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	Quantity      decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	QuoteQuantity decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	ExecutedQty   decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	CumQuoteQty   decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	Status        string          `gorm:"not null;index:idx_orders_user_open,priority:2;index:idx_orders_status_created,priority:1"`
	RejectReason  string          `gorm:"not null;default:''"`
	LastSeq       int64           `gorm:"not null;default:0"`
	CreatedAt     time.Time       `gorm:"not null;default:now();index:idx_orders_user_created,priority:2,sort:desc;index:idx_orders_status_created,priority:2"`
	UpdatedAt     time.Time       `gorm:"not null;default:now()"`
}

// TableName implements gorm.Tabler.
func (Order) TableName() string { return "orders" }

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
	Commission      decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	CommissionAsset string          `gorm:"not null"`
	IsMaker         bool            `gorm:"not null"`
	Time            time.Time       `gorm:"not null;index:idx_trades_user_time,priority:2,sort:desc"`
}

// TableName implements gorm.Tabler.
func (Trade) TableName() string { return "trades" }

// Models returns the service's own models (outbox/inbox are added by the caller).
func Models() []any { return []any{&Order{}, &Trade{}} }

var errNotFound = errors.New("not found")

var openStatuses = []string{StatusPendingNew, StatusNew, StatusPartiallyFilled}

func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errNotFound
	}
	return err
}

func insertOrder(ctx context.Context, db *gorm.DB, o Order) error {
	o.UpdatedAt = o.CreatedAt
	return db.WithContext(ctx).Create(&o).Error
}

func getOrder(ctx context.Context, db *gorm.DB, id string) (Order, error) {
	var o Order
	err := db.WithContext(ctx).Where("order_id = ?", id).Take(&o).Error
	return o, notFound(err)
}

func getOrderByClientID(ctx context.Context, db *gorm.DB, userID, clientID string) (Order, error) {
	var o Order
	err := db.WithContext(ctx).Where("user_id = ? AND client_order_id = ?", userID, clientID).Take(&o).Error
	return o, notFound(err)
}

// updateReturning runs a conditional UPDATE ... RETURNING * and returns the updated row, or
// errNotFound when no row matched the conditions.
func updateReturning(ctx context.Context, db *gorm.DB, values map[string]any, query string, args ...any) (Order, error) {
	var o Order
	res := db.WithContext(ctx).Model(&o).Clauses(clause.Returning{}).Where(query, args...).Updates(values)
	if res.Error != nil {
		return Order{}, res.Error
	}
	if res.RowsAffected == 0 {
		return Order{}, errNotFound
	}
	return o, nil
}

// transitionFromPending moves a PENDING_NEW order to status. It returns errNotFound when the order
// is no longer pending (e.g. the recovery job got there first).
func transitionFromPending(ctx context.Context, db *gorm.DB, id, status, reason string) (Order, error) {
	return updateReturning(ctx, db,
		map[string]any{"status": status, "reject_reason": reason, "updated_at": gorm.Expr("now()")},
		"order_id = ? AND status = ?", id, StatusPendingNew)
}

func lockPendingOrders(ctx context.Context, db *gorm.DB, olderThan time.Time, limit int) ([]Order, error) {
	out := []Order{}
	err := db.WithContext(ctx).
		Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate, Options: clause.LockingOptionsSkipLocked}).
		Where("status = ? AND created_at < ?", StatusPendingNew, olderThan).
		Order("created_at").Limit(limit).Find(&out).Error
	return out, err
}

// userScope filters by owner and, when non-empty, by symbol.
func userScope(db *gorm.DB, userID, symbol string) *gorm.DB {
	db = db.Where("user_id = ?", userID)
	if symbol != "" {
		db = db.Where("symbol = ?", symbol)
	}
	return db
}

func listOpenOrders(ctx context.Context, db *gorm.DB, userID, symbol string) ([]Order, error) {
	out := []Order{}
	err := userScope(db.WithContext(ctx), userID, symbol).Where("status IN ?", openStatuses).
		Order("created_at DESC").Find(&out).Error
	return out, err
}

func listOrders(ctx context.Context, db *gorm.DB, userID, symbol string, limit int) ([]Order, error) {
	out := []Order{}
	err := userScope(db.WithContext(ctx), userID, symbol).Order("created_at DESC").Limit(limit).Find(&out).Error
	return out, err
}

// applyStatusChange updates the order only when the event is newer than what was applied.
// It returns errNotFound for stale (or unknown) events.
func applyStatusChange(ctx context.Context, db *gorm.DB, ev events.OrderStatusChanged) (Order, error) {
	return updateReturning(ctx, db, map[string]any{
		"status":        ev.Status,
		"executed_qty":  ev.ExecutedQty,
		"cum_quote_qty": ev.CumQuoteQty,
		"reject_reason": ev.Reason,
		"last_seq":      ev.Seq,
		"updated_at":    ev.Time,
	}, "order_id = ? AND last_seq < ?", ev.OrderID, ev.Seq)
}

// insertTrades inserts trade rows, ignoring ones already recorded (idempotent).
func insertTrades(ctx context.Context, db *gorm.DB, trades []Trade) error {
	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "trade_id"}, {Name: "symbol"}, {Name: "order_id"}},
		DoNothing: true,
	}).Create(&trades).Error
}

func listTrades(ctx context.Context, db *gorm.DB, userID, symbol string, limit int) ([]Trade, error) {
	out := []Trade{}
	err := userScope(db.WithContext(ctx), userID, symbol).Order("time DESC, id DESC").Limit(limit).Find(&out).Error
	return out, err
}
