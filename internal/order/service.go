package order

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"simple-securities/internal/order/walletclient"
	"simple-securities/pkg/apperr"
	"simple-securities/pkg/database"
	"simple-securities/pkg/events"
	"simple-securities/pkg/outbox"
)

// ServiceName is the consumer group, inbox consumer and event source name.
const ServiceName = "order"

const (
	defaultListLimit = 50
	maxListLimit     = 500

	recoveryInterval = 5 * time.Second
	recoveryAge      = 10 * time.Second
	recoveryBatch    = 100

	reasonInsufficientBalance = "INSUFFICIENT_BALANCE"
	reasonLockTimeout         = "LOCK_TIMEOUT"
	reasonLockReleased        = "LOCK_RELEASED"
)

var reasonPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// Notifier wakes the outbox dispatcher after a commit.
type Notifier interface{ Notify() }

// Service implements the order lifecycle.
type Service struct {
	db     *gorm.DB
	wallet walletclient.Client
	disp   Notifier
	log    *zap.Logger
	now    func() time.Time
}

// NewService builds the order service.
func NewService(db *gorm.DB, wallet walletclient.Client, disp Notifier, log *zap.Logger) *Service {
	return &Service{db: db, wallet: wallet, disp: disp, log: log,
		now: func() time.Time { return time.Now().UTC() }}
}

// PlaceOrder validates, persists (PENDING_NEW), locks funds and hands the order to matching.
func (s *Service) PlaceOrder(ctx context.Context, in PlaceInput) (Order, error) {
	v, err := Validate(in)
	if err != nil {
		return Order{}, err
	}
	id := uuid.Must(uuid.NewV7()).String()
	o := Order{
		ID:            id,
		ClientOrderID: v.ClientOrderID,
		UserID:        in.UserID,
		Symbol:        v.Symbol.Name,
		Side:          v.Side,
		Type:          v.Type,
		TimeInForce:   v.TimeInForce,
		Price:         v.Price,
		Quantity:      v.Quantity,
		QuoteQuantity: v.QuoteQuantity,
		Status:        StatusPendingNew,
		CreatedAt:     s.now(),
	}
	if o.ClientOrderID == "" {
		o.ClientOrderID = id
	}
	o.UpdatedAt = o.CreatedAt

	if err := insertOrder(ctx, s.db, o); err != nil {
		if database.IsUniqueViolation(err) {
			existing, gerr := getOrderByClientID(ctx, s.db, o.UserID, o.ClientOrderID)
			if gerr != nil {
				return Order{}, fmt.Errorf("load duplicate order: %w", gerr)
			}
			return existing, nil // idempotent retry
		}
		return Order{}, fmt.Errorf("insert order: %w", err)
	}

	if _, err := s.wallet.LockFunds(ctx, o.ID, o.UserID, v.LockAsset, v.LockAmount); err != nil {
		switch status.Code(err) {
		case codes.FailedPrecondition, codes.InvalidArgument:
			reason := walletReason(err)
			if _, rerr := s.reject(ctx, o.ID, reason); rerr != nil {
				s.log.Error("mark order rejected", zap.String("order_id", o.ID), zap.Error(rerr))
			}
			if status.Code(err) == codes.InvalidArgument {
				return Order{}, apperr.Invalid(reason, "funds lock rejected: %s", status.Convert(err).Message())
			}
			return Order{}, apperr.Precondition(reason, "insufficient %s balance", v.LockAsset)
		default:
			// Outcome unknown: the recovery job reconciles the PENDING_NEW order with the wallet.
			s.log.Warn("wallet lock failed, order left PENDING_NEW", zap.String("order_id", o.ID), zap.Error(err))
			return Order{}, &apperr.Error{Code: codes.Unavailable, Reason: "WALLET_UNAVAILABLE",
				Msg: "wallet unavailable, order " + o.ID + " will be reconciled"}
		}
	}

	accepted, err := s.accept(ctx, o.ID)
	switch {
	case errors.Is(err, errNotFound):
		// The recovery job already moved it on; return the current state.
		return s.mustGet(ctx, o.ID)
	case err != nil:
		s.log.Warn("accept order failed, recovery will retry", zap.String("order_id", o.ID), zap.Error(err))
		return Order{}, &apperr.Error{Code: codes.Unavailable, Reason: "ORDER_STORE_UNAVAILABLE",
			Msg: "order " + o.ID + " will be reconciled"}
	}
	return accepted, nil
}

// walletReason extracts an UPPER_SNAKE reason from a wallet error.
func walletReason(err error) string {
	if r := apperr.Reason(err); reasonPattern.MatchString(r) {
		return r
	}
	return reasonInsufficientBalance
}

// accept moves a PENDING_NEW order to NEW and emits order.accepted (one tx), then notifies.
func (s *Service) accept(ctx context.Context, id string) (Order, error) {
	var o Order
	err := database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		var err error
		if o, err = transitionFromPending(ctx, tx, id, StatusNew, ""); err != nil {
			return err
		}
		return s.addAccepted(ctx, tx, o)
	})
	if err != nil {
		return Order{}, err
	}
	s.disp.Notify()
	return o, nil
}

// reject moves a PENDING_NEW order to REJECTED and emits order.updated (one tx), then notifies.
func (s *Service) reject(ctx context.Context, id, reason string) (Order, error) {
	var o Order
	err := database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		var err error
		if o, err = transitionFromPending(ctx, tx, id, StatusRejected, reason); err != nil {
			return err
		}
		return s.addUpdated(ctx, tx, o, 0)
	})
	if err != nil {
		return Order{}, err
	}
	s.disp.Notify()
	return o, nil
}

func (s *Service) addAccepted(ctx context.Context, db *gorm.DB, o Order) error {
	cmd := events.OrderAccepted{
		OrderID: o.ID, ClientOrderID: o.ClientOrderID, UserID: o.UserID, Symbol: o.Symbol, Side: o.Side,
		Type: o.Type, TimeInForce: o.TimeInForce, Price: o.Price.String(), Quantity: o.Quantity.String(),
		QuoteQuantity: o.QuoteQuantity.String(), CreatedAt: o.CreatedAt,
	}
	env, err := events.New(ServiceName, events.TypeOrderAccepted, o.ID, cmd)
	if err != nil {
		return err
	}
	return outbox.Add(ctx, db, events.TopicOrderCommands, o.Symbol, env)
}

func (s *Service) addUpdated(ctx context.Context, db *gorm.DB, o Order, lastTradeID int64) error {
	env, err := events.New(ServiceName, events.TypeOrderUpdated, o.ID, orderUpdated(o, lastTradeID))
	if err != nil {
		return err
	}
	return outbox.Add(ctx, db, events.TopicOrderUpdates, o.UserID, env)
}

// CancelOrder requests cancellation; matching reports the outcome asynchronously.
func (s *Service) CancelOrder(ctx context.Context, userID, orderID string) (Order, error) {
	o, err := s.GetOrder(ctx, userID, orderID)
	if err != nil {
		return Order{}, err
	}
	if o.Status != StatusNew && o.Status != StatusPartiallyFilled {
		return Order{}, apperr.Precondition("ORDER_NOT_OPEN", "order is %s", o.Status)
	}
	env, err := events.New(ServiceName, events.TypeOrderCancelRequested, o.ID, events.OrderCancelRequested{
		OrderID: o.ID, UserID: o.UserID, Symbol: o.Symbol,
	})
	if err != nil {
		return Order{}, err
	}
	err = database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		return outbox.Add(ctx, tx, events.TopicOrderCommands, o.Symbol, env)
	})
	if err != nil {
		return Order{}, fmt.Errorf("request cancel: %w", err)
	}
	s.disp.Notify()
	return o, nil
}

// GetOrder returns an order owned by userID.
func (s *Service) GetOrder(ctx context.Context, userID, orderID string) (Order, error) {
	if userID == "" {
		return Order{}, apperr.Invalid("INVALID_USER", "user_id is required")
	}
	if _, err := uuid.Parse(orderID); err != nil {
		return Order{}, apperr.NotFound("ORDER_NOT_FOUND", "order not found")
	}
	o, err := getOrder(ctx, s.db, orderID)
	if errors.Is(err, errNotFound) || (err == nil && o.UserID != userID) {
		return Order{}, apperr.NotFound("ORDER_NOT_FOUND", "order not found")
	}
	return o, err
}

func (s *Service) mustGet(ctx context.Context, id string) (Order, error) {
	o, err := getOrder(ctx, s.db, id)
	if err != nil {
		return Order{}, fmt.Errorf("load order %s: %w", id, err)
	}
	return o, nil
}

// ListOpenOrders returns PENDING_NEW, NEW and PARTIALLY_FILLED orders, newest first.
func (s *Service) ListOpenOrders(ctx context.Context, userID, symbol string) ([]Order, error) {
	if userID == "" {
		return nil, apperr.Invalid("INVALID_USER", "user_id is required")
	}
	return listOpenOrders(ctx, s.db, userID, symbol)
}

// ListOrders returns the user's orders, newest first.
func (s *Service) ListOrders(ctx context.Context, userID, symbol string, limit int) ([]Order, error) {
	if userID == "" {
		return nil, apperr.Invalid("INVALID_USER", "user_id is required")
	}
	return listOrders(ctx, s.db, userID, symbol, clampLimit(limit))
}

// ListTrades returns the user's trades, newest first.
func (s *Service) ListTrades(ctx context.Context, userID, symbol string, limit int) ([]Trade, error) {
	if userID == "" {
		return nil, apperr.Invalid("INVALID_USER", "user_id is required")
	}
	return listTrades(ctx, s.db, userID, symbol, clampLimit(limit))
}

func clampLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultListLimit
	case limit > maxListLimit:
		return maxListLimit
	}
	return limit
}

// RunRecovery reconciles orders stuck in PENDING_NEW with the wallet every 5s.
func (s *Service) RunRecovery(ctx context.Context) error {
	t := time.NewTicker(recoveryInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if n, err := s.recoverOnce(ctx); err != nil && ctx.Err() == nil {
				s.log.Warn("pending order recovery failed", zap.Error(err))
			} else if n > 0 {
				s.log.Info("pending orders reconciled", zap.Int("count", n))
			}
		}
	}
}

func (s *Service) recoverOnce(ctx context.Context) (int, error) {
	n := 0
	err := database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		pending, err := lockPendingOrders(ctx, tx, s.now().Add(-recoveryAge), recoveryBatch)
		if err != nil {
			return err
		}
		for _, o := range pending {
			lock, err := s.wallet.GetLock(ctx, o.ID)
			if err != nil {
				// Wallet unreachable: keep what was reconciled so far, retry the rest later.
				s.log.Warn("recovery: wallet GetLock failed", zap.String("order_id", o.ID), zap.Error(err))
				return nil
			}
			var updated Order
			switch {
			case lock.Found && lock.Status == walletclient.LockActive:
				if updated, err = transitionFromPending(ctx, tx, o.ID, StatusNew, ""); err == nil {
					err = s.addAccepted(ctx, tx, updated)
				}
			default:
				reason := reasonLockTimeout
				if lock.Found {
					reason = reasonLockReleased
				}
				if updated, err = transitionFromPending(ctx, tx, o.ID, StatusRejected, reason); err == nil {
					err = s.addUpdated(ctx, tx, updated, 0)
				}
			}
			if err != nil {
				return fmt.Errorf("recover order %s: %w", o.ID, err)
			}
			n++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if n > 0 {
		s.disp.Notify()
	}
	return n, nil
}

func orderUpdated(o Order, lastTradeID int64) events.OrderUpdated {
	return events.OrderUpdated{
		OrderID: o.ID, ClientOrderID: o.ClientOrderID, UserID: o.UserID, Symbol: o.Symbol, Side: o.Side,
		Type: o.Type, TimeInForce: o.TimeInForce, Price: o.Price.String(), Quantity: o.Quantity.String(),
		QuoteQuantity: o.QuoteQuantity.String(), ExecutedQty: o.ExecutedQty.String(),
		CumQuoteQty: o.CumQuoteQty.String(), Status: o.Status, RejectReason: o.RejectReason,
		LastTradeID: lastTradeID, UpdatedAt: o.UpdatedAt,
	}
}
