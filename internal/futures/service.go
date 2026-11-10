package futures

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"simple-securities/pkg/apperr"
	"simple-securities/pkg/database"
	"simple-securities/pkg/events"
)

const (
	defaultListLimit = 50
	maxListLimit     = 1000
)

var (
	marketBuySlippage  = decimal.RequireFromString("1.005")
	marketSellSlippage = decimal.RequireFromString("0.995")
)

// Notifier wakes the outbox dispatcher after a commit.
type Notifier interface{ Notify() }

// Publisher publishes best-effort (non-outbox) messages, e.g. mark prices.
type Publisher interface {
	PublishAsync(ctx context.Context, topic, key string, env events.Envelope)
}

// Config tunes the service.
type Config struct {
	FundingInterval time.Duration // default 8h, aligned to the epoch
}

// Service implements the futures account, trading and risk logic.
type Service struct {
	db     *gorm.DB
	disp   Notifier
	pub    Publisher
	prices *Prices
	cfg    Config
	log    *zap.Logger
	now    func() time.Time

	leader *leaderState // owned by the leader goroutines
}

// NewService builds the futures service.
func NewService(db *gorm.DB, disp Notifier, pub Publisher, prices *Prices, cfg Config, log *zap.Logger) *Service {
	if cfg.FundingInterval <= 0 {
		cfg.FundingInterval = 8 * time.Hour
	}
	return &Service{db: db, disp: disp, pub: pub, prices: prices, cfg: cfg, log: log,
		now: func() time.Time { return time.Now().UTC() }}
}

// PlaceOrder validates the order, checks margin and hands it to matching, in one transaction.
func (s *Service) PlaceOrder(ctx context.Context, in PlaceInput) (Order, error) {
	v, err := validateOrder(in)
	if err != nil {
		return Order{}, err
	}
	return s.place(ctx, in.UserID, v, false)
}

// ClosePosition sends a reduce-only MARKET order for the whole position.
func (s *Service) ClosePosition(ctx context.Context, userID, symbol string) (Order, error) {
	if err := requireUser(userID); err != nil {
		return Order{}, err
	}
	sym, err := lookupSymbol(symbol)
	if err != nil {
		return Order{}, err
	}
	return s.place(ctx, userID, validOrder{Symbol: sym, Type: TypeMarket, TimeInForce: TIFIOC, ReduceOnly: true}, true)
}

// place runs the synchronous risk check (spec 3 "Placing an order"). With closeAll the side and
// quantity are derived from the locked position.
func (s *Service) place(ctx context.Context, userID string, v validOrder, closeAll bool) (Order, error) {
	sym := v.Symbol
	id := uuid.Must(uuid.NewV7()).String()
	o := Order{
		ID: id, ClientOrderID: v.ClientOrderID, UserID: userID, Symbol: sym.Name, Side: v.Side, Type: v.Type,
		TimeInForce: v.TimeInForce, Price: v.Price, Quantity: v.Quantity, ReduceOnly: v.ReduceOnly,
		Status: StatusNew, CreatedAt: s.now(),
	}
	if o.ClientOrderID == "" {
		o.ClientOrderID = id
	}
	err := database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		acct, err := lockAccount(ctx, tx, userID)
		if errors.Is(err, errNotFound) {
			return apperr.Precondition("INSUFFICIENT_MARGIN", "futures account has no balance")
		}
		if err != nil {
			return err
		}
		if err := ensurePosition(ctx, tx, userID, sym.Name, defaultLeverageFor(sym)); err != nil {
			return err
		}
		positions, err := lockPositions(ctx, tx, []string{userID})
		if err != nil {
			return err
		}
		pos := findPosition(positions, sym.Name)
		if pos == nil {
			return fmt.Errorf("position %s/%s missing after ensure", userID, sym.Name)
		}
		if pos.Liquidating {
			return apperr.Precondition("POSITION_LIQUIDATING", "position %s is being liquidated", sym.Name)
		}
		if closeAll {
			if pos.Size.IsZero() {
				return apperr.Precondition("NO_POSITION", "no open %s position", sym.Name)
			}
			o.Side = SideSell
			if pos.Size.IsNegative() {
				o.Side = SideBuy
			}
			o.Quantity = pos.Size.Abs()
		}

		// One-way netting.
		size := pos.Size.Abs()
		opposite := pos.Size.Sign()*sideSign(o.Side) < 0
		closeQty := decimal.Zero
		if opposite {
			closeQty = decimal.Min(o.Quantity, size)
		}
		if o.ReduceOnly {
			if !opposite {
				return apperr.Precondition("REDUCE_ONLY_REJECTED", "reduce-only order would not reduce the position")
			}
			o.Quantity = closeQty
		}
		openQty := o.Quantity.Sub(closeQty)

		ref := o.Price
		if o.Type == TypeMarket {
			mark, ok := s.prices.Price(sym.Name)
			if !ok {
				return errMarkUnavailable(sym.Name)
			}
			if err := checkMarketNotional(validOrder{Symbol: sym, Type: o.Type, Quantity: o.Quantity,
				ReduceOnly: o.ReduceOnly}, mark); err != nil {
				return err
			}
			ref = mark.Mul(marketSellSlippage)
			if o.Side == SideBuy {
				ref = mark.Mul(marketBuySlippage)
			}
		}
		o.MarginFrozen = money(openQty.Mul(ref).Div(leverageOf(*pos)).Add(o.Quantity.Mul(ref).Mul(sym.TakerFee)))

		orderMargin, err := openOrderMargin(ctx, tx, userID)
		if err != nil {
			return err
		}
		risk, err := computeRisk(acct.WalletBalance, positions, orderMargin, s.prices.Price, true)
		if err != nil {
			return err
		}
		if o.MarginFrozen.GreaterThan(risk.Available) {
			return apperr.Precondition("INSUFFICIENT_MARGIN", "order margin %s exceeds available balance %s",
				o.MarginFrozen, risk.Available)
		}
		if err := insertOrder(ctx, tx, &o); err != nil {
			return err
		}
		return addAccepted(ctx, tx, o)
	})
	if err != nil {
		if database.IsUniqueViolation(err) {
			existing, gerr := getOrderByClientID(ctx, s.db, userID, o.ClientOrderID)
			if gerr != nil {
				return Order{}, fmt.Errorf("load duplicate order: %w", gerr)
			}
			return existing, nil // idempotent retry
		}
		return Order{}, err
	}
	s.disp.Notify()
	return o, nil
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
	err = database.WithTx(ctx, s.db, func(tx *gorm.DB) error { return addCancel(ctx, tx, o) })
	if err != nil {
		return Order{}, fmt.Errorf("request cancel: %w", err)
	}
	s.disp.Notify()
	return o, nil
}

// GetOrder returns an order owned by userID.
func (s *Service) GetOrder(ctx context.Context, userID, orderID string) (Order, error) {
	if err := requireUser(userID); err != nil {
		return Order{}, err
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

// ListOpenOrders returns NEW and PARTIALLY_FILLED orders, newest first.
func (s *Service) ListOpenOrders(ctx context.Context, userID, symbol string) ([]Order, error) {
	if err := requireUser(userID); err != nil {
		return nil, err
	}
	return listOpenOrders(ctx, s.db, userID, symbol)
}

// ListOrders returns the user's orders, newest first.
func (s *Service) ListOrders(ctx context.Context, userID, symbol string, limit int) ([]Order, error) {
	if err := requireUser(userID); err != nil {
		return nil, err
	}
	return listOrders(ctx, s.db, userID, symbol, clampLimit(limit))
}

// ListTrades returns the user's trades, newest first.
func (s *Service) ListTrades(ctx context.Context, userID, symbol string, limit int) ([]Trade, error) {
	if err := requireUser(userID); err != nil {
		return nil, err
	}
	return listTrades(ctx, s.db, userID, symbol, clampLimit(limit))
}

// ListIncome returns the user's income history, newest first.
func (s *Service) ListIncome(ctx context.Context, userID, symbol, incomeType string, limit int) ([]Income, error) {
	if err := requireUser(userID); err != nil {
		return nil, err
	}
	return listIncome(ctx, s.db, userID, symbol, incomeType, clampLimit(limit))
}

// ListFundingRates returns settled funding rates, newest first.
func (s *Service) ListFundingRates(ctx context.Context, symbol string, limit int) ([]FundingRecord, error) {
	if symbol != "" {
		sym, err := lookupSymbol(symbol)
		if err != nil {
			return nil, err
		}
		symbol = sym.Name
	}
	return listFundingRates(ctx, s.db, symbol, clampLimit(limit))
}

// MarkPrices returns the cached mark prices (one symbol, or all when symbol is empty).
func (s *Service) MarkPrices(symbol string) ([]MarkInfo, error) {
	if symbol == "" {
		return s.prices.Marks(), nil
	}
	sym, err := lookupSymbol(symbol)
	if err != nil {
		return nil, err
	}
	m, ok := s.prices.Mark(sym.Name)
	if !ok {
		return nil, errMarkUnavailable(sym.Name)
	}
	return []MarkInfo{m}, nil
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
