package app

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"simple-securities/internal/wallet/domain"
	"simple-securities/internal/wallet/store"
	"simple-securities/pkg/events"
	"simple-securities/pkg/symbols"
)

// Event is one decoded consumer event; exactly one field is set.
type Event struct {
	UserRegistered *events.UserRegistered
	Trade          *events.TradeExecuted
	OrderClosed    *events.OrderStatusChanged // terminal status only
	FromFutures    *events.Transfer           // transfer.futures_to_spot: credit spot free
}

// fromFutures is a validated futures -> spot transfer.
type fromFutures struct {
	userID, asset, transferID string
	amount                    decimal.Decimal
}

func parseFromFutures(t *events.Transfer) (fromFutures, error) {
	userID, err := parseID(t.UserID, "INVALID_USER_ID")
	if err != nil {
		return fromFutures{}, err
	}
	if t.TransferID == "" {
		return fromFutures{}, fmt.Errorf("missing transfer_id")
	}
	if t.Asset != domain.QuoteAsset {
		return fromFutures{}, fmt.Errorf("unsupported asset %q", t.Asset)
	}
	amt, err := parseAmount(t.Amount, decimal.Zero)
	if err != nil {
		return fromFutures{}, err
	}
	return fromFutures{userID: userID, asset: t.Asset, transferID: t.TransferID, amount: amt}, nil
}

// ApplyEvents applies a batch of events, in order, inside tx: airdrops for new users, trade
// settlement, release of the remaining lock of closed orders and futures -> spot credits.
//
// Lock order (deadlock free with the RPCs): every order_locks row of the batch first (by
// order_id), then every balance row of the batch (by user_id, asset), all before any change.
func (s *Service) ApplyEvents(ctx context.Context, tx *gorm.DB, evs []Event) error {
	type trade struct {
		ev  *events.TradeExecuted
		sym symbols.Symbol
	}
	var (
		orderIDs  []string
		trades    = map[int]trade{}
		transfers = map[int]fromFutures{}
	)
	for i := range evs {
		switch e := evs[i]; {
		case e.UserRegistered != nil:
			id, err := parseID(e.UserRegistered.UserID, "INVALID_USER_ID")
			if err != nil {
				return fmt.Errorf("user.registered: %w", err)
			}
			e.UserRegistered.UserID = id
		case e.Trade != nil:
			t := e.Trade
			sym, ok := symbols.Get(t.Symbol)
			if !ok {
				return fmt.Errorf("trade %d: unknown symbol %q", t.TradeID, t.Symbol)
			}
			for _, p := range []*string{&t.BuyerUserID, &t.SellerUserID, &t.BuyerOrderID, &t.SellerOrderID} {
				id, err := parseID(*p, "INVALID_ID")
				if err != nil {
					return fmt.Errorf("trade %s:%d: %w", t.Symbol, t.TradeID, err)
				}
				*p = id
			}
			trades[i] = trade{ev: t, sym: sym}
			orderIDs = append(orderIDs, t.BuyerOrderID, t.SellerOrderID)
		case e.OrderClosed != nil:
			id, err := parseID(e.OrderClosed.OrderID, "INVALID_ORDER_ID")
			if err != nil {
				return fmt.Errorf("order.status_changed: %w", err)
			}
			e.OrderClosed.OrderID = id
			orderIDs = append(orderIDs, id)
		case e.FromFutures != nil:
			t, err := parseFromFutures(e.FromFutures)
			if err != nil {
				return fmt.Errorf("transfer.futures_to_spot %s: %w", e.FromFutures.TransferID, err)
			}
			transfers[i] = t
		}
	}

	slices.Sort(orderIDs)
	locks, err := store.LockOrders(ctx, tx, slices.Compact(orderIDs))
	if err != nil {
		return err
	}

	var keys []domain.Key
	for i, e := range evs {
		switch {
		case e.UserRegistered != nil:
			for _, a := range symbols.Assets() {
				keys = append(keys, domain.Key{UserID: e.UserRegistered.UserID, Asset: a})
			}
		case e.Trade != nil:
			t := trades[i]
			keys = append(keys,
				domain.Key{UserID: t.ev.BuyerUserID, Asset: t.sym.Quote},
				domain.Key{UserID: t.ev.BuyerUserID, Asset: t.sym.Base},
				domain.Key{UserID: t.ev.SellerUserID, Asset: t.sym.Base},
				domain.Key{UserID: t.ev.SellerUserID, Asset: t.sym.Quote})
		case e.OrderClosed != nil:
			if l, ok := locks[e.OrderClosed.OrderID]; ok && l.Status == domain.LockActive {
				keys = append(keys, domain.Key{UserID: l.UserID, Asset: l.Asset})
			}
		case e.FromFutures != nil:
			t := transfers[i]
			keys = append(keys, domain.Key{UserID: t.userID, Asset: t.asset})
		}
	}
	l := store.NewLedger(tx)
	if err := l.Lock(ctx, keys...); err != nil {
		return err
	}

	var (
		fees  []store.FeeAccrual
		dirty = map[string]*store.OrderLock{}
	)
	for i, e := range evs {
		switch {
		case e.UserRegistered != nil:
			if s.cfg.AirdropUSDT.IsPositive() {
				u := e.UserRegistered.UserID
				m := domain.Movement{UserID: u, Asset: domain.QuoteAsset, Free: s.cfg.AirdropUSDT}
				if err := l.Apply(m, domain.RefAirdrop, u); err != nil {
					return err
				}
			}
		case e.Trade != nil:
			t := trades[i]
			fs, err := s.settleTrade(l, locks, dirty, t.ev, t.sym)
			if err != nil {
				return err
			}
			fees = append(fees, fs...)
		case e.OrderClosed != nil:
			lock, ok := locks[e.OrderClosed.OrderID]
			if !ok {
				s.log.Warn("terminal status for order without lock", zap.String("order_id", e.OrderClosed.OrderID),
					zap.String("status", e.OrderClosed.Status))
				continue
			}
			if lock.Status != domain.LockActive {
				continue
			}
			if _, err := releaseLock(l, lock); err != nil {
				return err
			}
			dirty[lock.OrderID] = lock
		case e.FromFutures != nil:
			t := transfers[i]
			m := domain.Movement{UserID: t.userID, Asset: t.asset, Free: t.amount}
			if err := l.Apply(m, domain.RefFuturesTransferIn, t.transferID); err != nil {
				return err
			}
		}
	}

	if _, err := l.Flush(ctx); err != nil {
		return err
	}
	changed := make([]*store.OrderLock, 0, len(dirty))
	for _, lock := range dirty {
		changed = append(changed, lock)
	}
	slices.SortFunc(changed, func(a, b *store.OrderLock) int { return strings.Compare(a.OrderID, b.OrderID) })
	if err := store.UpdateOrderLocks(ctx, tx, changed); err != nil {
		return err
	}
	if err := store.InsertFeeAccruals(ctx, tx, fees); err != nil {
		return err
	}
	return addBalanceEvents(ctx, tx, l)
}

// settleTrade applies one trade's movements, fee credits and lock decrements.
func (s *Service) settleTrade(l *store.Ledger, locks, dirty map[string]*store.OrderLock,
	t *events.TradeExecuted, sym symbols.Symbol,
) ([]store.FeeAccrual, error) {
	st, err := domain.SettleTrade(*t, sym.Base, sym.Quote)
	if err != nil {
		return nil, err
	}
	// Trade ids are sequenced per symbol, so the ledger reference includes the symbol.
	ref := t.Symbol + ":" + strconv.FormatInt(t.TradeID, 10)
	for _, m := range st.Movements {
		if err := l.Apply(m, domain.RefTrade, ref); err != nil {
			s.log.Error("trade settlement overdraws a balance", zap.String("trade", ref),
				zap.String("user_id", m.UserID), zap.String("asset", m.Asset), zap.Error(err))
			return nil, fmt.Errorf("settle trade %s: %w", ref, err)
		}
	}
	fees := make([]store.FeeAccrual, 0, len(st.Fees))
	for _, f := range st.Fees {
		l.CreditFee(f, ref)
		fees = append(fees, store.FeeAccrual{TradeID: t.TradeID, Symbol: t.Symbol, Asset: f.Asset, Amount: f.Amount})
	}
	for _, d := range st.Locks {
		lock, ok := locks[d.OrderID]
		if !ok {
			s.log.Error("trade for order without lock", zap.String("trade", ref), zap.String("order_id", d.OrderID))
			continue
		}
		rem, clamped := domain.DecrementRemaining(lock.Remaining, d.Amount)
		if clamped {
			s.log.Error("fill exceeds order lock remaining (clamped to 0)", zap.String("trade", ref),
				zap.String("order_id", d.OrderID), zap.String("remaining", lock.Remaining.String()),
				zap.String("fill", d.Amount.String()))
		}
		lock.Remaining = rem
		dirty[lock.OrderID] = lock
	}
	return fees, nil
}
