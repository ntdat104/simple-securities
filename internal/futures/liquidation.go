package futures

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"simple-securities/pkg/database"
)

// liquidationCooldown spaces liquidation attempts of one position (e.g. empty book → EXPIRED).
const liquidationCooldown = 3 * time.Second

// scanLiquidations evaluates every open position at mark price and liquidates the ones below
// maintenance (spec 3 "Liquidation"). The scan reads without locks; each candidate user is then
// re-checked under locks.
func (s *Service) scanLiquidations(ctx context.Context, st *leaderState) error {
	positions, err := listOpenPositions(ctx, s.db)
	if err != nil || len(positions) == 0 {
		return err
	}
	byUser := map[string][]Position{}
	for _, p := range positions {
		byUser[p.UserID] = append(byUser[p.UserID], p)
	}
	users := mapKeys(byUser)
	var accounts []Account
	if err := s.db.WithContext(ctx).Where("user_id IN ?", users).Find(&accounts).Error; err != nil {
		return err
	}
	wallets := make(map[string]Account, len(accounts))
	for _, a := range accounts {
		wallets[a.UserID] = a
	}
	now := s.now()
	for k, t := range st.cooldown {
		if now.Sub(t) >= liquidationCooldown {
			delete(st.cooldown, k)
		}
	}
	for _, u := range users {
		if !s.hasLiquidatable(st, wallets[u], byUser[u], now) {
			continue
		}
		if err := s.liquidate(ctx, st, u, now); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			s.log.Warn("liquidation failed", zap.String("user_id", u), zap.Error(err))
		}
	}
	return nil
}

// hasLiquidatable reports whether at least one position must be (re)liquidated now.
func (s *Service) hasLiquidatable(st *leaderState, acct Account, positions []Position, now time.Time) bool {
	targets := liquidationTargets(acct.WalletBalance, positions, s.prices.Price)
	for _, sym := range targets {
		p := findPosition(positions, sym)
		if p != nil && !p.Liquidating && now.Sub(st.cooldown[keyOf(*p)]) >= liquidationCooldown {
			return true
		}
	}
	return false
}

// liquidate re-checks a user under locks; for every position to liquidate it sets the
// liquidating flag, cancels the user's open orders of that symbol and places a MARKET IOC
// reduce-only liquidation order for |size| (through the outbox → matching).
func (s *Service) liquidate(ctx context.Context, st *leaderState, userID string, now time.Time) error {
	var placed []Order
	err := database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		placed = placed[:0]
		acct, err := lockAccount(ctx, tx, userID)
		if errors.Is(err, errNotFound) {
			acct = &Account{UserID: userID}
		} else if err != nil {
			return err
		}
		positions, err := lockPositions(ctx, tx, []string{userID})
		if err != nil {
			return err
		}
		for _, symbol := range liquidationTargets(acct.WalletBalance, positions, s.prices.Price) {
			pos := findPosition(positions, symbol)
			if pos == nil || pos.Liquidating || pos.Size.IsZero() ||
				now.Sub(st.cooldown[keyOf(*pos)]) < liquidationCooldown {
				continue
			}
			o, err := s.placeLiquidation(ctx, tx, pos)
			if err != nil {
				return err
			}
			placed = append(placed, o)
		}
		return nil
	})
	if err != nil || len(placed) == 0 {
		return err
	}
	s.disp.Notify()
	for _, o := range placed {
		st.cooldown[posKey{o.UserID, o.Symbol}] = now
		s.log.Warn("position liquidation started", zap.String("user_id", o.UserID), zap.String("symbol", o.Symbol),
			zap.String("side", o.Side), zap.String("quantity", o.Quantity.String()), zap.String("order_id", o.ID))
	}
	return nil
}

// placeLiquidation flags the (locked) position, cancels its symbol's open orders and inserts the
// liquidation order with its order.accepted command.
func (s *Service) placeLiquidation(ctx context.Context, tx *gorm.DB, pos *Position) (Order, error) {
	pos.Liquidating = true
	if err := savePosition(ctx, tx, pos); err != nil {
		return Order{}, err
	}
	open, err := listOpenOrders(ctx, tx, pos.UserID, pos.Symbol)
	if err != nil {
		return Order{}, err
	}
	for _, o := range open {
		if o.Liquidation {
			continue
		}
		if err := addCancel(ctx, tx, o); err != nil {
			return Order{}, err
		}
	}
	id := uuid.Must(uuid.NewV7()).String()
	side := SideSell
	if pos.Size.IsNegative() {
		side = SideBuy
	}
	o := Order{
		ID: id, ClientOrderID: "liquidation-" + id, UserID: pos.UserID, Symbol: pos.Symbol, Side: side,
		Type: TypeMarket, TimeInForce: TIFIOC, Quantity: pos.Size.Abs(), ReduceOnly: true, Liquidation: true,
		Status: StatusNew, CreatedAt: s.now(),
	}
	if err := insertOrder(ctx, tx, &o); err != nil {
		return Order{}, err
	}
	return o, addAccepted(ctx, tx, o)
}
