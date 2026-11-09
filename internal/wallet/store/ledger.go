// Package store holds the wallet GORM models and repositories. Everything takes a *gorm.DB so it
// works on the pool or inside the caller's transaction.
package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"simple-securities/internal/wallet/domain"
	"simple-securities/pkg/events"
)

// Ledger is a transaction-scoped working set of row-locked balances. It applies movements in
// memory, records a ledger row per movement and writes everything back on Flush.
//
// Deadlock avoidance: Lock takes all row locks of a unit of work in one statement, ordered by
// (user_id, asset). Callers must lock every key they need up front, before applying movements.
type Ledger struct {
	db      *gorm.DB
	bal     map[domain.Key]domain.Balance
	dirty   map[domain.Key]bool
	reason  map[string]string // user -> last ref type
	pending []LedgerEntry
}

// NewLedger returns a ledger bound to db (normally the caller's transaction).
func NewLedger(db *gorm.DB) *Ledger {
	return &Ledger{db: db, bal: map[domain.Key]domain.Balance{}, dirty: map[domain.Key]bool{},
		reason: map[string]string{}}
}

// Lock creates missing rows with zero balances and locks all keys with SELECT ... FOR UPDATE in
// deterministic (user_id, asset) order. Already locked keys are skipped.
func (l *Ledger) Lock(ctx context.Context, keys ...domain.Key) error {
	var todo []domain.Key
	for _, k := range keys {
		if _, ok := l.bal[k]; !ok {
			todo = append(todo, k)
		}
	}
	todo = domain.SortKeys(todo)
	if len(todo) == 0 {
		return nil
	}
	db := l.db.WithContext(ctx)
	rows := make([]Balance, len(todo))
	tuples := make([][]any, len(todo))
	for i, k := range todo {
		rows[i] = Balance{UserID: k.UserID, Asset: k.Asset, Free: decimal.Zero, Locked: decimal.Zero}
		tuples[i] = []any{k.UserID, k.Asset}
	}
	// Rows are inserted in key order so concurrent inserts of the same keys cannot deadlock.
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&rows, batchSize).Error; err != nil {
		return fmt.Errorf("ensure balances: %w", err)
	}
	var locked []Balance
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("(user_id, asset) IN ?", tuples).Order("user_id, asset").Find(&locked).Error
	if err != nil {
		return fmt.Errorf("lock balances: %w", err)
	}
	for _, b := range locked {
		l.bal[domain.Key{UserID: b.UserID, Asset: b.Asset}] = domain.Balance{Free: b.Free, Locked: b.Locked}
	}
	for _, k := range todo {
		if _, ok := l.bal[k]; !ok {
			return fmt.Errorf("balance %s/%s not locked", k.UserID, k.Asset)
		}
	}
	return nil
}

// Balance returns the current in-memory balance of a locked key.
func (l *Ledger) Balance(k domain.Key) (domain.Balance, bool) {
	b, ok := l.bal[k]
	return b, ok
}

// Apply applies m to its (locked) balance and records a ledger row. It returns
// domain.ErrInsufficient (wrapped) when free or locked would become negative.
func (l *Ledger) Apply(m domain.Movement, refType, refID string) error {
	if m.IsZero() {
		return nil
	}
	k := m.Key()
	cur, ok := l.bal[k]
	if !ok {
		return fmt.Errorf("balance %s/%s used without Lock", k.UserID, k.Asset)
	}
	next, err := cur.Apply(m)
	if err != nil {
		return err
	}
	l.bal[k] = next
	l.dirty[k] = true
	l.reason[k.UserID] = refType
	free, locked := next.Free, next.Locked
	l.pending = append(l.pending, LedgerEntry{UserID: k.UserID, Asset: k.Asset, DeltaFree: m.Free,
		DeltaLocked: m.Locked, FreeAfter: &free, LockedAfter: &locked, RefType: refType, RefID: refID})
	return nil
}

// CreditFee records a fee-account ledger row. The fee account has no balance row (its balance is
// the sum of fee_accruals), which keeps a single hot row out of every settlement transaction.
func (l *Ledger) CreditFee(f domain.Fee, refID string) {
	l.pending = append(l.pending, LedgerEntry{UserID: domain.FeeAccountID, Asset: f.Asset,
		DeltaFree: f.Amount, DeltaLocked: decimal.Zero, RefType: domain.RefFee, RefID: refID})
}

// Flush writes changed balances and pending ledger rows. It returns the inserted ledger entries.
func (l *Ledger) Flush(ctx context.Context) ([]LedgerEntry, error) {
	if err := l.writeBalances(ctx); err != nil {
		return nil, err
	}
	entries, err := l.writeEntries(ctx)
	if err != nil {
		return nil, err
	}
	l.pending = nil
	return entries, nil
}

// Updates returns one balance.updated payload per touched user (fee account excluded), carrying
// the current balances of every asset changed through this ledger.
func (l *Ledger) Updates(now time.Time) []events.BalanceUpdated {
	byUser := map[string][]domain.Key{}
	for k := range l.dirty {
		byUser[k.UserID] = append(byUser[k.UserID], k)
	}
	users := make([]string, 0, len(byUser))
	for u := range byUser {
		users = append(users, u)
	}
	sort.Strings(users)
	out := make([]events.BalanceUpdated, 0, len(users))
	for _, u := range users {
		keys := domain.SortKeys(byUser[u])
		items := make([]events.BalanceItem, 0, len(keys))
		for _, k := range keys {
			b := l.bal[k]
			items = append(items, events.BalanceItem{Asset: k.Asset, Free: b.Free.String(), Locked: b.Locked.String()})
		}
		out = append(out, events.BalanceUpdated{UserID: u, Balances: items, Reason: l.reason[u], Time: now})
	}
	return out
}

func (l *Ledger) writeBalances(ctx context.Context) error {
	if len(l.dirty) == 0 {
		return nil
	}
	keys := make([]domain.Key, 0, len(l.dirty))
	for k := range l.dirty {
		keys = append(keys, k)
	}
	keys = domain.SortKeys(keys)
	rows := make([]Balance, len(keys))
	for i, k := range keys {
		b := l.bal[k]
		rows[i] = Balance{UserID: k.UserID, Asset: k.Asset, Free: b.Free, Locked: b.Locked}
	}
	// Every row exists and is row-locked by Lock, so the upsert always takes its UPDATE branch:
	// one statement per batch instead of a round trip per balance.
	res := l.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "asset"}},
		DoUpdates: append(clause.AssignmentColumns([]string{"free", "locked"}),
			clause.Assignment{Column: clause.Column{Name: "updated_at"}, Value: gorm.Expr("now()")}),
	}).CreateInBatches(&rows, batchSize)
	if res.Error != nil {
		return fmt.Errorf("update balances: %w", res.Error)
	}
	if int(res.RowsAffected) != len(keys) {
		return fmt.Errorf("update balances: %d of %d rows", res.RowsAffected, len(keys))
	}
	return nil
}

func (l *Ledger) writeEntries(ctx context.Context) ([]LedgerEntry, error) {
	n := len(l.pending)
	if n == 0 {
		return nil, nil
	}
	rows := make([]LedgerEntry, n)
	for i, e := range l.pending {
		e.ID, e.CreatedAt = 0, time.Time{}
		rows[i] = e
	}
	// id and created_at come back through RETURNING.
	if err := l.db.WithContext(ctx).CreateInBatches(&rows, batchSize).Error; err != nil {
		return nil, fmt.Errorf("insert ledger: %w", err)
	}
	type ret struct {
		id int64
		at time.Time
	}
	rets := make([]ret, n)
	for i, r := range rows {
		if r.ID == 0 {
			return nil, fmt.Errorf("insert ledger: row %d has no id", i)
		}
		rets[i] = ret{id: r.ID, at: r.CreatedAt}
	}
	// Ids are assigned in insertion (VALUES) order; sort so the mapping does not depend on the
	// order RETURNING emits rows.
	sort.Slice(rets, func(i, j int) bool { return rets[i].id < rets[j].id })
	out := make([]LedgerEntry, n)
	for i, e := range l.pending {
		e.ID, e.CreatedAt = rets[i].id, rets[i].at
		out[i] = e
	}
	return out, nil
}

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("not found")

// LedgerByID loads one ledger entry.
func LedgerByID(ctx context.Context, db *gorm.DB, id int64) (LedgerEntry, error) {
	var e LedgerEntry
	err := db.WithContext(ctx).Where("id = ?", id).Take(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return LedgerEntry{}, ErrNotFound
	}
	if err != nil {
		return LedgerEntry{}, fmt.Errorf("select ledger: %w", err)
	}
	return e, nil
}

// ListLedger returns a user's entries newest first; beforeID > 0 pages backwards.
func ListLedger(ctx context.Context, db *gorm.DB, userID string, beforeID int64, limit int) ([]LedgerEntry, error) {
	q := db.WithContext(ctx).Where("user_id = ?", userID)
	if beforeID > 0 {
		q = q.Where("id < ?", beforeID)
	}
	var out []LedgerEntry
	if err := q.Order("id DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, fmt.Errorf("list ledger: %w", err)
	}
	return out, nil
}
