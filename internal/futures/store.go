package futures

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Lock ordering (deadlock freedom): every transaction locks futures_accounts rows first (sorted by
// user_id), then positions rows (sorted by user_id, symbol), then orders / insurance_fund rows.

var errNotFound = errors.New("not found")

func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errNotFound
	}
	return err
}

var forUpdate = clause.Locking{Strength: clause.LockingStrengthUpdate}

type posKey struct{ UserID, Symbol string }

func keyOf(p Position) posKey { return posKey{p.UserID, p.Symbol} }

// sortedUnique returns the distinct values sorted.
func sortedUnique(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return slices.Compact(out)
}

// ensureAccounts creates missing (zero balance) accounts, in user order.
func ensureAccounts(ctx context.Context, db *gorm.DB, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}
	now := time.Now().UTC()
	rows := make([]Account, 0, len(userIDs))
	for _, u := range sortedUnique(userIDs) {
		rows = append(rows, Account{UserID: u, CreatedAt: now, UpdatedAt: now})
	}
	return db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

// lockAccounts locks (FOR UPDATE, sorted by user_id) the existing accounts of the given users.
func lockAccounts(ctx context.Context, db *gorm.DB, userIDs []string) (map[string]*Account, error) {
	out := make(map[string]*Account, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []Account
	err := db.WithContext(ctx).Clauses(forUpdate).Where("user_id IN ?", sortedUnique(userIDs)).
		Order("user_id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for i := range rows {
		out[rows[i].UserID] = &rows[i]
	}
	return out, nil
}

// lockAccount locks one account; errNotFound when the user has none.
func lockAccount(ctx context.Context, db *gorm.DB, userID string) (*Account, error) {
	m, err := lockAccounts(ctx, db, []string{userID})
	if err != nil {
		return nil, err
	}
	a, ok := m[userID]
	if !ok {
		return nil, errNotFound
	}
	return a, nil
}

func getAccount(ctx context.Context, db *gorm.DB, userID string) (Account, error) {
	var a Account
	err := db.WithContext(ctx).Where("user_id = ?", userID).Take(&a).Error
	return a, notFound(err)
}

func saveAccount(ctx context.Context, db *gorm.DB, a *Account) error {
	a.UpdatedAt = time.Now().UTC()
	return db.WithContext(ctx).Model(&Account{}).Where("user_id = ?", a.UserID).
		Updates(map[string]any{"wallet_balance": a.WalletBalance, "updated_at": a.UpdatedAt}).Error
}

// ensurePosition creates the (flat) settings row of a user's contract with defaults.
func ensurePosition(ctx context.Context, db *gorm.DB, userID, symbol string, leverage int) error {
	now := time.Now().UTC()
	return db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&Position{
		UserID: userID, Symbol: symbol, MarginType: MarginCrossed, Leverage: leverage,
		CreatedAt: now, UpdatedAt: now,
	}).Error
}

// lockPositions locks every position row of the users, sorted by (user_id, symbol).
func lockPositions(ctx context.Context, db *gorm.DB, userIDs []string) ([]Position, error) {
	out := []Position{}
	if len(userIDs) == 0 {
		return out, nil
	}
	err := db.WithContext(ctx).Clauses(forUpdate).Where("user_id IN ?", sortedUnique(userIDs)).
		Order("user_id, symbol").Find(&out).Error
	return out, err
}

func listPositions(ctx context.Context, db *gorm.DB, userID string) ([]Position, error) {
	out := []Position{}
	err := db.WithContext(ctx).Where("user_id = ?", userID).Order("symbol").Find(&out).Error
	return out, err
}

func listOpenPositions(ctx context.Context, db *gorm.DB) ([]Position, error) {
	out := []Position{}
	err := db.WithContext(ctx).Where("size <> 0").Order("user_id, symbol").Find(&out).Error
	return out, err
}

// savePosition upserts all mutable columns of a position.
func savePosition(ctx context.Context, db *gorm.DB, p *Position) error {
	p.UpdatedAt = time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = p.UpdatedAt
	}
	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "symbol"}},
		DoUpdates: clause.AssignmentColumns([]string{"margin_type", "leverage", "size", "entry_price",
			"isolated_margin", "liquidating", "updated_at"}),
	}).Create(p).Error
}

func findPosition(positions []Position, symbol string) *Position {
	for i := range positions {
		if positions[i].Symbol == symbol {
			return &positions[i]
		}
	}
	return nil
}

// openOrderMargin sums the unreleased frozen margin of the user's open orders.
func openOrderMargin(ctx context.Context, db *gorm.DB, userID string) (decimal.Decimal, error) {
	m, err := openOrderMargins(ctx, db, []string{userID})
	return m[userID], err
}

func openOrderMargins(ctx context.Context, db *gorm.DB, userIDs []string) (map[string]decimal.Decimal, error) {
	out := make(map[string]decimal.Decimal, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		UserID string
		Margin decimal.Decimal
	}
	err := db.WithContext(ctx).Model(&Order{}).
		Select("user_id, COALESCE(SUM(margin_frozen - margin_released), 0) AS margin").
		Where("user_id IN ? AND status IN ?", sortedUnique(userIDs), openStatuses).
		Group("user_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.UserID] = r.Margin
	}
	return out, nil
}

func insertOrder(ctx context.Context, db *gorm.DB, o *Order) error {
	o.UpdatedAt = o.CreatedAt
	return db.WithContext(ctx).Create(o).Error
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

// loadOrders returns the orders of this service among ids (unknown ids are skipped).
func loadOrders(ctx context.Context, db *gorm.DB, ids []string) (map[string]*Order, error) {
	valid := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, err := uuid.Parse(id); err == nil {
			valid = append(valid, id)
		}
	}
	out := make(map[string]*Order, len(valid))
	if len(valid) == 0 {
		return out, nil
	}
	var rows []Order
	if err := db.WithContext(ctx).Where("order_id IN ?", sortedUnique(valid)).Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		out[rows[i].ID] = &rows[i]
	}
	return out, nil
}

// saveOrderProgress writes the fields advanced by matching events.
func saveOrderProgress(ctx context.Context, db *gorm.DB, o *Order) error {
	return db.WithContext(ctx).Model(&Order{}).Where("order_id = ?", o.ID).Updates(map[string]any{
		"status":          o.Status,
		"executed_qty":    o.ExecutedQty,
		"cum_quote":       o.CumQuote,
		"avg_price":       o.AvgPrice,
		"reject_reason":   o.RejectReason,
		"margin_released": o.MarginReleased,
		"realized_pnl":    o.RealizedPnl,
		"last_seq":        o.LastSeq,
		"updated_at":      o.UpdatedAt,
	}).Error
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

// insertTrade records a user trade; false when it was already recorded.
func insertTrade(ctx context.Context, db *gorm.DB, t *Trade) (bool, error) {
	res := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "trade_id"}, {Name: "symbol"}, {Name: "order_id"}},
		DoNothing: true,
	}).Create(t)
	return res.RowsAffected == 1, res.Error
}

func listTrades(ctx context.Context, db *gorm.DB, userID, symbol string, limit int) ([]Trade, error) {
	out := []Trade{}
	err := userScope(db.WithContext(ctx), userID, symbol).Order("time DESC, id DESC").Limit(limit).Find(&out).Error
	return out, err
}

func insertIncome(ctx context.Context, db *gorm.DB, rows []Income) error {
	if len(rows) == 0 {
		return nil
	}
	return db.WithContext(ctx).Create(&rows).Error
}

func listIncome(ctx context.Context, db *gorm.DB, userID, symbol, incomeType string, limit int) ([]Income, error) {
	out := []Income{}
	q := userScope(db.WithContext(ctx), userID, symbol)
	if incomeType != "" {
		q = q.Where("income_type = ?", incomeType)
	}
	err := q.Order("id DESC").Limit(limit).Find(&out).Error
	return out, err
}

func listFundingRates(ctx context.Context, db *gorm.DB, symbol string, limit int) ([]FundingRecord, error) {
	out := []FundingRecord{}
	q := db.WithContext(ctx)
	if symbol != "" {
		q = q.Where("symbol = ?", symbol)
	}
	err := q.Order("funding_time DESC, symbol").Limit(limit).Find(&out).Error
	return out, err
}

// lockInsuranceFund locks (creating if needed) the insurance fund row of the asset.
func lockInsuranceFund(ctx context.Context, db *gorm.DB, asset string) (*InsuranceFund, error) {
	err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&InsuranceFund{Asset: asset, UpdatedAt: time.Now().UTC()}).Error
	if err != nil {
		return nil, err
	}
	var f InsuranceFund
	if err := db.WithContext(ctx).Clauses(forUpdate).Where("asset = ?", asset).Take(&f).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func saveInsuranceFund(ctx context.Context, db *gorm.DB, f *InsuranceFund) error {
	return db.WithContext(ctx).Model(&InsuranceFund{}).Where("asset = ?", f.Asset).
		Updates(map[string]any{"balance": f.Balance, "updated_at": time.Now().UTC()}).Error
}

// SeedInsuranceFund inserts the initial insurance fund balance once (no-op when it exists).
func SeedInsuranceFund(ctx context.Context, db *gorm.DB, amount decimal.Decimal) error {
	return db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&InsuranceFund{Asset: Asset, Balance: amount, UpdatedAt: time.Now().UTC()}).Error
}

// insertTransfer records a transfer; false when the transfer id was already recorded.
func insertTransfer(ctx context.Context, db *gorm.DB, t *Transfer) (bool, error) {
	res := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "transfer_id"}}, DoNothing: true,
	}).Create(t)
	return res.RowsAffected == 1, res.Error
}

func getTransferByKey(ctx context.Context, db *gorm.DB, userID, key string) (Transfer, error) {
	var t Transfer
	err := db.WithContext(ctx).Where("user_id = ? AND idempotency_key = ?", userID, key).Take(&t).Error
	return t, notFound(err)
}
