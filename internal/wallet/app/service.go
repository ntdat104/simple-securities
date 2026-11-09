// Package app implements the wallet use cases. Every state change runs in one transaction together
// with its balance.updated outbox events; the dispatcher is notified after commit.
package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"simple-securities/internal/wallet/domain"
	"simple-securities/internal/wallet/store"
	"simple-securities/pkg/apperr"
	"simple-securities/pkg/database"
	"simple-securities/pkg/events"
	"simple-securities/pkg/outbox"
	"simple-securities/pkg/symbols"
)

const (
	source          = "wallet"
	defaultLedger   = 50
	maxLedger       = 500
	maxAddressBytes = 256
	maxKeyBytes     = 128
)

// Config holds the wallet business settings.
type Config struct {
	FaucetEnabled bool
	MaxDeposit    decimal.Decimal // per call
	AirdropUSDT   decimal.Decimal // credited on user.registered (0 disables)
}

// Service implements the wallet use cases.
type Service struct {
	db         *gorm.DB
	dispatcher *outbox.Dispatcher
	cfg        Config
	log        *zap.Logger
}

// NewService builds the service.
func NewService(db *gorm.DB, d *outbox.Dispatcher, cfg Config, log *zap.Logger) *Service {
	return &Service{db: db, dispatcher: d, cfg: cfg, log: log}
}

// Notify wakes the outbox dispatcher; call after a committed change.
func (s *Service) Notify() { s.dispatcher.Notify() }

// GetBalances returns every asset of the user (zero rows included), USDT first.
func (s *Service) GetBalances(ctx context.Context, userID string) ([]store.AssetBalance, error) {
	userID, err := parseID(userID, "INVALID_USER_ID")
	if err != nil {
		return nil, err
	}
	rows, err := store.ListBalances(ctx, s.db, userID)
	if err != nil {
		return nil, err
	}
	byAsset := map[string]store.AssetBalance{}
	for _, a := range symbols.Assets() {
		byAsset[a] = store.AssetBalance{Asset: a, Balance: domain.Balance{Free: decimal.Zero, Locked: decimal.Zero}}
	}
	for _, r := range rows {
		byAsset[r.Asset] = r
	}
	assets := make([]string, 0, len(byAsset))
	for a := range byAsset {
		assets = append(assets, a)
	}
	domain.SortAssets(assets)
	out := make([]store.AssetBalance, len(assets))
	for i, a := range assets {
		out[i] = byAsset[a]
	}
	return out, nil
}

// Deposit credits a simulated deposit (faucet). Idempotent on idempotencyKey.
func (s *Service) Deposit(ctx context.Context, userID, asset, amount, key string) (store.LedgerEntry, error) {
	if !s.cfg.FaucetEnabled {
		return store.LedgerEntry{}, apperr.Forbidden("FAUCET_DISABLED", "deposits are disabled")
	}
	t, ref, err := s.parseTransfer(userID, asset, amount, key, domain.RefDeposit, s.cfg.MaxDeposit)
	if err != nil {
		return store.LedgerEntry{}, err
	}
	return s.transfer(ctx, t, ref, domain.Movement{UserID: t.UserID, Asset: t.Asset, Free: t.Amount}, nil)
}

// Withdraw debits free funds immediately (simulated withdrawal). Idempotent on idempotencyKey.
func (s *Service) Withdraw(ctx context.Context, userID, asset, amount, address, key string) (store.LedgerEntry, error) {
	if address == "" || len(address) > maxAddressBytes {
		return store.LedgerEntry{}, apperr.Invalid("INVALID_ADDRESS", "address is required (max %d bytes)", maxAddressBytes)
	}
	t, ref, err := s.parseTransfer(userID, asset, amount, key, domain.RefWithdraw, decimal.Zero)
	if err != nil {
		return store.LedgerEntry{}, err
	}
	t.Address = address
	return s.transfer(ctx, t, ref, domain.Movement{UserID: t.UserID, Asset: t.Asset, Free: t.Amount.Neg()}, nil)
}

// TransferToFutures debits spot free USDT and emits transfer.spot_to_futures (wallet.transfers) in
// the same transaction; the futures service credits the futures account asynchronously.
// Idempotent on idempotencyKey (a replay returns the original ledger entry, whose ref_id is the
// transfer id).
func (s *Service) TransferToFutures(ctx context.Context, userID, asset, amount, key string) (store.LedgerEntry, error) {
	if asset != domain.QuoteAsset {
		return store.LedgerEntry{}, apperr.Invalid("INVALID_ASSET", "only %s can be transferred to futures",
			domain.QuoteAsset)
	}
	t, _, err := s.parseTransfer(userID, asset, amount, key, domain.RefFuturesTransferOut, decimal.Zero)
	if err != nil {
		return store.LedgerEntry{}, err
	}
	transferID := uuid.Must(uuid.NewV7()).String()
	emit := func(tx *gorm.DB) error {
		p := events.Transfer{TransferID: transferID, UserID: t.UserID, Asset: t.Asset, Amount: t.Amount.String(),
			Time: time.Now().UTC()}
		env, err := events.New(source, events.TypeSpotToFutures, t.UserID, p)
		if err != nil {
			return fmt.Errorf("build transfer event: %w", err)
		}
		return outbox.Add(ctx, tx, events.TopicWalletTransfers, t.UserID, env)
	}
	return s.transfer(ctx, t, transferID, domain.Movement{UserID: t.UserID, Asset: t.Asset, Free: t.Amount.Neg()},
		emit)
}

// parseTransfer validates a deposit/withdraw request. The stored key is scoped by user and kind;
// the caller's key (or a generated one) is returned as the ledger ref_id.
func (s *Service) parseTransfer(userID, asset, amount, key, kind string, max decimal.Decimal,
) (t store.Transfer, refID string, err error) {
	if userID, err = parseID(userID, "INVALID_USER_ID"); err != nil {
		return t, "", err
	}
	if err := validateAsset(asset); err != nil {
		return t, "", err
	}
	amt, err := parseAmount(amount, max)
	if err != nil {
		return t, "", err
	}
	if len(key) > maxKeyBytes {
		return t, "", apperr.Invalid("INVALID_IDEMPOTENCY_KEY", "idempotency_key too long")
	}
	if key == "" {
		key = uuid.Must(uuid.NewV7()).String()
	}
	t = store.Transfer{Key: userID + ":" + kind + ":" + key, UserID: userID, Kind: kind, Asset: asset, Amount: amt}
	return t, key, nil
}

// transfer applies one idempotent single-balance movement. extra (optional) adds outbox events in
// the same transaction; it runs only for a first (non replayed) request.
func (s *Service) transfer(ctx context.Context, t store.Transfer, refID string, m domain.Movement,
	extra func(tx *gorm.DB) error,
) (store.LedgerEntry, error) {
	var (
		entry   store.LedgerEntry
		changed bool
	)
	err := database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		claimed, ledgerID, err := store.ClaimTransfer(ctx, tx, t)
		if err != nil {
			return err
		}
		if !claimed { // replay of an earlier request: return its result
			entry, err = store.LedgerByID(ctx, tx, ledgerID)
			return err
		}
		l := store.NewLedger(tx)
		if err := l.Lock(ctx, m.Key()); err != nil {
			return err
		}
		if err := l.Apply(m, t.Kind, refID); err != nil {
			return err
		}
		entries, err := l.Flush(ctx)
		if err != nil {
			return err
		}
		entry = entries[0]
		if err := store.SetTransferLedger(ctx, tx, t.Key, entry.ID); err != nil {
			return err
		}
		if extra != nil {
			if err := extra(tx); err != nil {
				return err
			}
		}
		changed = true
		return addBalanceEvents(ctx, tx, l)
	})
	if err != nil {
		return store.LedgerEntry{}, mapErr(err)
	}
	if changed {
		s.Notify()
	}
	return entry, nil
}

// LockResult is the outcome of LockFunds.
type LockResult struct {
	OrderID       string
	Asset         string
	Amount        decimal.Decimal
	AlreadyLocked bool
}

// errLockRace means a concurrent LockFunds for the same order won; the call is retried.
var errLockRace = errors.New("order lock inserted concurrently")

// LockFunds moves amount from free to locked for an order. Idempotent on orderID.
func (s *Service) LockFunds(ctx context.Context, orderID, userID, asset, amount string) (LockResult, error) {
	orderID, err := parseID(orderID, "INVALID_ORDER_ID")
	if err != nil {
		return LockResult{}, err
	}
	if userID, err = parseID(userID, "INVALID_USER_ID"); err != nil {
		return LockResult{}, err
	}
	if err := validateAsset(asset); err != nil {
		return LockResult{}, err
	}
	amt, err := parseAmount(amount, decimal.Zero)
	if err != nil {
		return LockResult{}, err
	}
	want := store.OrderLock{OrderID: orderID, UserID: userID, Asset: asset, Amount: amt, Remaining: amt,
		Status: domain.LockActive}
	for attempt := 0; ; attempt++ {
		res, err := s.lockOnce(ctx, want)
		if errors.Is(err, errLockRace) && attempt == 0 {
			continue
		}
		return res, err
	}
}

func (s *Service) lockOnce(ctx context.Context, want store.OrderLock) (LockResult, error) {
	res := LockResult{OrderID: want.OrderID, Asset: want.Asset, Amount: want.Amount}
	changed := false
	err := database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		existing, err := store.LockOrders(ctx, tx, []string{want.OrderID})
		if err != nil {
			return err
		}
		if cur, ok := existing[want.OrderID]; ok {
			if cur.UserID != want.UserID || cur.Asset != want.Asset || !cur.Amount.Equal(want.Amount) {
				return apperr.Conflict("LOCK_MISMATCH", "order %s is already locked with different parameters", want.OrderID)
			}
			res.AlreadyLocked = true
			return nil
		}
		l := store.NewLedger(tx)
		m := domain.LockMovement(want.UserID, want.Asset, want.Amount)
		if err := l.Lock(ctx, m.Key()); err != nil {
			return err
		}
		if err := l.Apply(m, domain.RefOrderLock, want.OrderID); err != nil {
			return err
		}
		inserted, err := store.InsertOrderLock(ctx, tx, want)
		if err != nil {
			return err
		}
		if !inserted {
			return errLockRace
		}
		if _, err := l.Flush(ctx); err != nil {
			return err
		}
		changed = true
		return addBalanceEvents(ctx, tx, l)
	})
	if err != nil {
		return LockResult{}, mapErr(err)
	}
	if changed {
		s.Notify()
	}
	return res, nil
}

// UnlockFunds releases the remaining lock of an order. Idempotent; unknown orders release "0".
func (s *Service) UnlockFunds(ctx context.Context, orderID, reason string) (decimal.Decimal, error) {
	orderID, err := parseID(orderID, "INVALID_ORDER_ID")
	if err != nil {
		return decimal.Zero, err
	}
	released := decimal.Zero
	err = database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		locks, err := store.LockOrders(ctx, tx, []string{orderID})
		if err != nil {
			return err
		}
		lock, ok := locks[orderID]
		if !ok || lock.Status != domain.LockActive {
			return nil
		}
		l := store.NewLedger(tx)
		if err := l.Lock(ctx, domain.Key{UserID: lock.UserID, Asset: lock.Asset}); err != nil {
			return err
		}
		if released, err = releaseLock(l, lock); err != nil {
			return err
		}
		if err := store.UpdateOrderLocks(ctx, tx, []*store.OrderLock{lock}); err != nil {
			return err
		}
		if _, err := l.Flush(ctx); err != nil {
			return err
		}
		return addBalanceEvents(ctx, tx, l)
	})
	if err != nil {
		return decimal.Zero, mapErr(err)
	}
	if released.IsPositive() {
		s.log.Debug("lock released", zap.String("order_id", orderID), zap.String("reason", reason),
			zap.String("amount", released.String()))
		s.Notify()
	}
	return released, nil
}

// GetLock returns an order lock; found is false when the order has none.
func (s *Service) GetLock(ctx context.Context, orderID string) (store.OrderLock, bool, error) {
	orderID, err := parseID(orderID, "INVALID_ORDER_ID")
	if err != nil {
		return store.OrderLock{}, false, err
	}
	lock, err := store.GetOrderLock(ctx, s.db, orderID)
	if errors.Is(err, store.ErrNotFound) {
		return store.OrderLock{}, false, nil
	}
	if err != nil {
		return store.OrderLock{}, false, err
	}
	return lock, true, nil
}

// ListLedger returns the newest entries of a user (limit <= 500, default 50), before beforeID.
func (s *Service) ListLedger(ctx context.Context, userID string, limit int, beforeID int64) ([]store.LedgerEntry, error) {
	userID, err := parseID(userID, "INVALID_USER_ID")
	if err != nil {
		return nil, err
	}
	switch {
	case limit <= 0:
		limit = defaultLedger
	case limit > maxLedger:
		limit = maxLedger
	}
	return store.ListLedger(ctx, s.db, userID, beforeID, limit)
}

// releaseLock frees the remaining amount of an ACTIVE lock (its balance key must be locked in l).
func releaseLock(l *store.Ledger, lock *store.OrderLock) (decimal.Decimal, error) {
	if lock.Status != domain.LockActive {
		return decimal.Zero, nil
	}
	amt := lock.Remaining
	if amt.IsPositive() {
		if err := l.Apply(domain.UnlockMovement(lock.UserID, lock.Asset, amt), domain.RefOrderUnlock, lock.OrderID); err != nil {
			return decimal.Zero, fmt.Errorf("release order %s: %w", lock.OrderID, err)
		}
	}
	lock.Remaining = decimal.Zero
	lock.Status = domain.LockReleased
	return amt, nil
}

// addBalanceEvents adds one balance.updated event per touched user to the outbox.
func addBalanceEvents(ctx context.Context, tx *gorm.DB, l *store.Ledger) error {
	for _, u := range l.Updates(time.Now().UTC()) {
		env, err := events.New(source, events.TypeBalanceUpdated, u.UserID, u)
		if err != nil {
			return fmt.Errorf("build balance event: %w", err)
		}
		if err := outbox.Add(ctx, tx, events.TopicWalletEvents, u.UserID, env); err != nil {
			return err
		}
	}
	return nil
}

func parseID(s, reason string) (string, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return "", apperr.Invalid(reason, "invalid id %q", s)
	}
	return id.String(), nil
}

func validateAsset(asset string) error {
	if !slices.Contains(symbols.Assets(), asset) {
		return apperr.Invalid("INVALID_ASSET", "unknown asset %q", asset)
	}
	return nil
}

func parseAmount(s string, max decimal.Decimal) (decimal.Decimal, error) {
	d, err := domain.ParseAmount(s, max)
	if err != nil {
		return decimal.Zero, apperr.Invalid("INVALID_AMOUNT", "%s", err.Error())
	}
	return d, nil
}

// mapErr turns domain errors into apperr errors; others pass through.
func mapErr(err error) error {
	var ae *apperr.Error
	switch {
	case errors.As(err, &ae):
		return ae
	case errors.Is(err, domain.ErrInsufficient):
		return apperr.Precondition("INSUFFICIENT_BALANCE", "insufficient balance")
	}
	return err
}
