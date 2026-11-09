package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"simple-securities/internal/wallet/domain"
)

// batchSize bounds the rows per INSERT statement of bulk writes.
const batchSize = 500

// AssetBalance is a balance row of one user.
type AssetBalance struct {
	Asset string
	domain.Balance
}

// ListBalances returns every balance row of a user.
func ListBalances(ctx context.Context, db *gorm.DB, userID string) ([]AssetBalance, error) {
	var rows []Balance
	if err := db.WithContext(ctx).Where("user_id = ?", userID).Order("asset").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list balances: %w", err)
	}
	out := make([]AssetBalance, len(rows))
	for i, r := range rows {
		out[i] = AssetBalance{Asset: r.Asset, Balance: domain.Balance{Free: r.Free, Locked: r.Locked}}
	}
	return out, nil
}

// GetOrderLock loads a lock without locking it.
func GetOrderLock(ctx context.Context, db *gorm.DB, orderID string) (OrderLock, error) {
	var l OrderLock
	err := db.WithContext(ctx).Where("order_id = ?", orderID).Take(&l).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OrderLock{}, ErrNotFound
	}
	if err != nil {
		return OrderLock{}, fmt.Errorf("select order lock: %w", err)
	}
	return l, nil
}

// LockOrders row-locks the given order locks (ordered by order_id) and returns the existing ones.
// Order locks are always locked before balances.
func LockOrders(ctx context.Context, db *gorm.DB, orderIDs []string) (map[string]*OrderLock, error) {
	out := map[string]*OrderLock{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	var locks []*OrderLock
	err := db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("order_id IN ?", orderIDs).Order("order_id").Find(&locks).Error
	if err != nil {
		return nil, fmt.Errorf("select order locks: %w", err)
	}
	for _, l := range locks {
		out[l.OrderID] = l
	}
	return out, nil
}

// InsertOrderLock inserts an ACTIVE lock; it returns false when the order already has one.
func InsertOrderLock(ctx context.Context, db *gorm.DB, l OrderLock) (bool, error) {
	res := db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "order_id"}}, DoNothing: true}).
		Create(&l)
	if res.Error != nil {
		return false, fmt.Errorf("insert order lock: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

// UpdateOrderLocks persists remaining/status of the given locks. The locks must have been loaded
// (and row-locked) through LockOrders, so the upsert always takes its UPDATE branch; one statement
// per batch replaces a round trip per lock.
func UpdateOrderLocks(ctx context.Context, db *gorm.DB, locks []*OrderLock) error {
	if len(locks) == 0 {
		return nil
	}
	rows := make([]OrderLock, len(locks))
	for i, l := range locks {
		rows[i] = *l
		rows[i].CreatedAt, rows[i].UpdatedAt = time.Time{}, time.Time{}
	}
	res := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "order_id"}},
		DoUpdates: append(clause.AssignmentColumns([]string{"remaining", "status"}),
			clause.Assignment{Column: clause.Column{Name: "updated_at"}, Value: gorm.Expr("now()")}),
	}).CreateInBatches(&rows, batchSize)
	if res.Error != nil {
		return fmt.Errorf("update order locks: %w", res.Error)
	}
	if int(res.RowsAffected) != len(rows) {
		return fmt.Errorf("update order locks: %d of %d rows", res.RowsAffected, len(rows))
	}
	return nil
}

// ClaimTransfer records t. If the key was already used it returns the ledger id of the original
// request and claimed=false. A concurrent request with the same key waits for the first to commit.
func ClaimTransfer(ctx context.Context, db *gorm.DB, t Transfer) (claimed bool, ledgerID int64, err error) {
	db = db.WithContext(ctx)
	row := Transfer{Key: t.Key, UserID: t.UserID, Kind: t.Kind, Asset: t.Asset, Amount: t.Amount, Address: t.Address}
	res := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "idempotency_key"}}, DoNothing: true}).
		Create(&row)
	if res.Error != nil {
		return false, 0, fmt.Errorf("insert transfer: %w", res.Error)
	}
	if res.RowsAffected == 1 {
		return true, 0, nil
	}
	var prev Transfer
	err = db.Select("ledger_id").Where("idempotency_key = ?", t.Key).Take(&prev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && prev.LedgerID == nil) {
		return false, 0, ErrNotFound
	}
	if err != nil {
		return false, 0, fmt.Errorf("select transfer: %w", err)
	}
	return false, *prev.LedgerID, nil
}

// SetTransferLedger links a transfer to its ledger entry.
func SetTransferLedger(ctx context.Context, db *gorm.DB, key string, ledgerID int64) error {
	err := db.WithContext(ctx).Model(&Transfer{}).Where("idempotency_key = ?", key).
		Update("ledger_id", ledgerID).Error
	if err != nil {
		return fmt.Errorf("update transfer: %w", err)
	}
	return nil
}

// InsertFeeAccruals appends fee rows.
func InsertFeeAccruals(ctx context.Context, db *gorm.DB, fees []FeeAccrual) error {
	if len(fees) == 0 {
		return nil
	}
	if err := db.WithContext(ctx).CreateInBatches(&fees, batchSize).Error; err != nil {
		return fmt.Errorf("insert fee accruals: %w", err)
	}
	return nil
}
