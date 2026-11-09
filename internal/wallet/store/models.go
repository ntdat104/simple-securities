package store

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Balance is the hot balance row of one (user, asset) (table balances).
type Balance struct {
	UserID    string          `gorm:"type:uuid;primaryKey"`
	Asset     string          `gorm:"primaryKey"`
	Free      decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0;check:balances_free_check,free >= 0"`
	Locked    decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0;check:balances_locked_check,locked >= 0"`
	UpdatedAt time.Time       `gorm:"not null;default:now();autoUpdateTime:false"`
}

// TableName implements gorm.Tabler.
func (Balance) TableName() string { return "balances" }

// OrderLock is the funds reserved for one order (table order_locks).
type OrderLock struct {
	OrderID   string          `gorm:"type:uuid;primaryKey"`
	UserID    string          `gorm:"type:uuid;not null"`
	Asset     string          `gorm:"not null"`
	Amount    decimal.Decimal `gorm:"type:numeric(36,18);not null;check:order_locks_amount_check,amount > 0"`
	Remaining decimal.Decimal `gorm:"type:numeric(36,18);not null;check:order_locks_remaining_check,remaining >= 0"`
	Status    string          `gorm:"not null;check:order_locks_status_check,status IN ('ACTIVE', 'RELEASED')"`
	CreatedAt time.Time       `gorm:"not null;default:now();autoCreateTime:false"`
	UpdatedAt time.Time       `gorm:"not null;default:now();autoUpdateTime:false"`
}

// TableName implements gorm.Tabler.
func (OrderLock) TableName() string { return "order_locks" }

// LedgerEntry is one immutable balance movement (table ledger_entries). FreeAfter/LockedAfter are
// nil for fee-account rows, whose balance is the sum of fee_accruals (no hot balance row).
type LedgerEntry struct {
	ID          int64            `gorm:"primaryKey;autoIncrement;index:idx_ledger_user_id,priority:2,sort:desc"`
	UserID      string           `gorm:"type:uuid;not null;index:idx_ledger_user_id,priority:1"`
	Asset       string           `gorm:"not null"`
	DeltaFree   decimal.Decimal  `gorm:"type:numeric(36,18);not null"`
	DeltaLocked decimal.Decimal  `gorm:"type:numeric(36,18);not null"`
	FreeAfter   *decimal.Decimal `gorm:"type:numeric(36,18)"`
	LockedAfter *decimal.Decimal `gorm:"type:numeric(36,18)"`
	RefType     string           `gorm:"not null"`
	RefID       string           `gorm:"not null"`
	CreatedAt   time.Time        `gorm:"not null;default:now();autoCreateTime:false"`
}

// TableName implements gorm.Tabler.
func (LedgerEntry) TableName() string { return "ledger_entries" }

// Transfer is a deposit, withdrawal or spot -> futures transfer request (table transfers),
// idempotent on the user-scoped idempotency key.
type Transfer struct {
	ID        int64           `gorm:"primaryKey;autoIncrement"`
	Key       string          `gorm:"column:idempotency_key;not null;uniqueIndex:transfers_idempotency_key_key"`
	UserID    string          `gorm:"type:uuid;not null"`
	Kind      string          `gorm:"not null;check:transfers_kind_chk,kind IN ('DEPOSIT', 'WITHDRAW', 'FUTURES_TRANSFER_OUT')"` // DEPOSIT | WITHDRAW | FUTURES_TRANSFER_OUT
	Asset     string          `gorm:"not null"`
	Amount    decimal.Decimal `gorm:"type:numeric(36,18);not null;check:transfers_amount_check,amount > 0"`
	Address   string          `gorm:"not null;default:''"`
	LedgerID  *int64
	CreatedAt time.Time `gorm:"not null;default:now();autoCreateTime:false"`
}

// TableName implements gorm.Tabler.
func (Transfer) TableName() string { return "transfers" }

// FeeAccrual is a trading fee collected by the exchange (table fee_accruals, append-only, avoids a
// contended fee balance row).
type FeeAccrual struct {
	ID        int64           `gorm:"primaryKey;autoIncrement"`
	TradeID   int64           `gorm:"not null"`
	Symbol    string          `gorm:"not null"`
	Asset     string          `gorm:"not null;index:idx_fee_accruals_asset"`
	Amount    decimal.Decimal `gorm:"type:numeric(36,18);not null;check:fee_accruals_amount_check,amount > 0"`
	CreatedAt time.Time       `gorm:"not null;default:now();autoCreateTime:false"`
}

// TableName implements gorm.Tabler.
func (FeeAccrual) TableName() string { return "fee_accruals" }

// Models returns every wallet model, for database.Migrate.
func Models() []any {
	return []any{&Balance{}, &OrderLock{}, &LedgerEntry{}, &Transfer{}, &FeeAccrual{}}
}

// MigrateConstraints drops constraints superseded by renamed ones (AutoMigrate only adds missing
// named constraints, it never replaces a changed definition). Run after database.Migrate.
func MigrateConstraints(ctx context.Context, db *gorm.DB) error {
	// transfers_kind_check (DEPOSIT, WITHDRAW) was replaced by transfers_kind_chk.
	if err := db.WithContext(ctx).Exec(`ALTER TABLE transfers DROP CONSTRAINT IF EXISTS transfers_kind_check`).Error; err != nil {
		return fmt.Errorf("drop transfers_kind_check: %w", err)
	}
	return nil
}
