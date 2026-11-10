package futures

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"simple-securities/pkg/events"
	"simple-securities/pkg/outbox"
)

// addAccepted emits the order.accepted command for the futures matching engine (key symbol).
// GTX (post-only) is sent as the engine type LIMIT_MAKER.
func addAccepted(ctx context.Context, db *gorm.DB, o Order) error {
	typ, tif, price := o.Type, o.TimeInForce, o.Price.String()
	switch {
	case o.Type == TypeLimit && o.TimeInForce == TIFGTX:
		typ, tif = engineTypeLimitMaker, TIFGTC
	case o.Type == TypeMarket:
		price = ""
	}
	cmd := events.OrderAccepted{
		OrderID: o.ID, ClientOrderID: o.ClientOrderID, UserID: o.UserID, Symbol: o.Symbol, Side: o.Side,
		Type: typ, TimeInForce: tif, Price: price, Quantity: o.Quantity.String(),
		ReduceOnly: o.ReduceOnly, Liquidation: o.Liquidation, CreatedAt: o.CreatedAt,
	}
	env, err := events.New(ServiceName, events.TypeOrderAccepted, o.ID, cmd)
	if err != nil {
		return err
	}
	return outbox.Add(ctx, db, events.TopicFuturesOrderCommands, o.Symbol, env)
}

// addCancel emits order.cancel_requested (key symbol).
func addCancel(ctx context.Context, db *gorm.DB, o Order) error {
	env, err := events.New(ServiceName, events.TypeOrderCancelRequested, o.ID, events.OrderCancelRequested{
		OrderID: o.ID, UserID: o.UserID, Symbol: o.Symbol,
	})
	if err != nil {
		return err
	}
	return outbox.Add(ctx, db, events.TopicFuturesOrderCommands, o.Symbol, env)
}

// addOrderUpdated emits order.updated (key user_id) for the gateway (ORDER_TRADE_UPDATE).
func addOrderUpdated(ctx context.Context, db *gorm.DB, o Order, lastTradeID int64) error {
	ev := events.OrderUpdated{
		OrderID: o.ID, ClientOrderID: o.ClientOrderID, UserID: o.UserID, Symbol: o.Symbol, Side: o.Side,
		Type: o.Type, TimeInForce: o.TimeInForce, Price: o.Price.String(), Quantity: o.Quantity.String(),
		ExecutedQty: o.ExecutedQty.String(), CumQuoteQty: o.CumQuote.String(), Status: o.Status,
		RejectReason: o.RejectReason, LastTradeID: lastTradeID, ReduceOnly: o.ReduceOnly,
		Liquidation: o.Liquidation, AvgPrice: o.AvgPrice.String(), RealizedPnl: o.RealizedPnl.String(),
		UpdatedAt: o.UpdatedAt,
	}
	env, err := events.New(ServiceName, events.TypeOrderUpdated, o.ID, ev)
	if err != nil {
		return err
	}
	return outbox.Add(ctx, db, events.TopicFuturesOrderUpdates, o.UserID, env)
}

// addAccountUpdate emits futures.account_update (key user_id) for the gateway (ACCOUNT_UPDATE).
// positions are the positions to report (changed or open ones).
func addAccountUpdate(ctx context.Context, db *gorm.DB, userID, reason string, wallet, available decimal.Decimal,
	positions []Position) error {
	items := make([]events.FuturesPositionItem, 0, len(positions))
	for _, p := range positions {
		items = append(items, events.FuturesPositionItem{
			Symbol: p.Symbol, PositionAmt: p.Size.String(), EntryPrice: p.EntryPrice.String(),
			MarginType: p.MarginType, IsolatedMargin: p.IsolatedMargin.String(), Leverage: p.Leverage,
		})
	}
	ev := events.FuturesAccountUpdate{UserID: userID, Reason: reason, WalletBalance: wallet.String(),
		AvailableBalance: available.String(), Positions: items, Time: time.Now().UTC()}
	env, err := events.New(ServiceName, events.TypeFuturesAccountUpdate, userID, ev)
	if err != nil {
		return err
	}
	return outbox.Add(ctx, db, events.TopicFuturesAccountEvents, userID, env)
}

// emitAccountUpdate computes the available balance (display prices) and emits the account update.
// report selects the positions to include (nil = every non-flat position).
func (s *Service) emitAccountUpdate(ctx context.Context, db *gorm.DB, acct *Account, positions []Position,
	orderMargin decimal.Decimal, reason string, report func(Position) bool) error {
	r, err := computeRisk(acct.WalletBalance, positions, orderMargin, s.prices.Price, false)
	if err != nil {
		return err
	}
	var items []Position
	for _, p := range positions {
		if (report == nil && !p.Size.IsZero()) || (report != nil && report(p)) {
			items = append(items, p)
		}
	}
	return addAccountUpdate(ctx, db, acct.UserID, reason, acct.WalletBalance, r.Available, items)
}

// emitAccountUpdateLoaded loads the user's positions and order margin, then emits the update.
func (s *Service) emitAccountUpdateLoaded(ctx context.Context, db *gorm.DB, acct *Account, reason string,
	report func(Position) bool) error {
	positions, err := listPositions(ctx, db, acct.UserID)
	if err != nil {
		return err
	}
	om, err := openOrderMargin(ctx, db, acct.UserID)
	if err != nil {
		return err
	}
	return s.emitAccountUpdate(ctx, db, acct, positions, om, reason, report)
}
