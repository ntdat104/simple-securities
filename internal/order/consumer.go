package order

import (
	"context"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"simple-securities/pkg/database"
	"simple-securities/pkg/events"
	"simple-securities/pkg/inbox"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/symbols"
)

// HandleMatchingEvents applies a batch of matching.events (one partition) in one transaction:
// user trades are recorded and order state is advanced (only by newer sequence numbers), emitting
// order.updated for every applied state change.
func (s *Service) HandleMatchingEvents(ctx context.Context, msgs []kafka.Message) error {
	emitted := 0
	err := database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		emitted = 0
		lastTrade := make(map[string]int64) // order id -> last trade id seen in this batch
		for _, m := range msgs {
			env := m.Envelope
			ok, err := inbox.Claim(ctx, tx, ServiceName, env.ID, env.Type)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			switch env.Type {
			case events.TypeTradeExecuted:
				var ev events.TradeExecuted
				if err := env.Decode(&ev); err != nil {
					s.log.Error("undecodable trade.executed, skipped", zap.String("event_id", env.ID), zap.Error(err))
					continue
				}
				if err := s.recordTrade(ctx, tx, ev); err != nil {
					return fmt.Errorf("trade %d: %w", ev.TradeID, err)
				}
				lastTrade[ev.BuyerOrderID] = ev.TradeID
				lastTrade[ev.SellerOrderID] = ev.TradeID
			case events.TypeOrderStatusChanged:
				var ev events.OrderStatusChanged
				if err := env.Decode(&ev); err != nil {
					s.log.Error("undecodable order.status_changed, skipped", zap.String("event_id", env.ID), zap.Error(err))
					continue
				}
				if ev.Time.IsZero() {
					ev.Time = s.now()
				}
				o, err := applyStatusChange(ctx, tx, ev)
				if errors.Is(err, errNotFound) {
					continue // stale (seq <= last_seq) or unknown order
				}
				if err != nil {
					return fmt.Errorf("status change %s: %w", ev.OrderID, err)
				}
				if err := s.addUpdated(ctx, tx, o, lastTrade[o.ID]); err != nil {
					return err
				}
				emitted++
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if emitted > 0 {
		s.disp.Notify()
	}
	return nil
}

// recordTrade inserts the buyer's and the seller's side of a fill (idempotent).
func (s *Service) recordTrade(ctx context.Context, db *gorm.DB, ev events.TradeExecuted) error {
	sides, err := tradeSides(ev)
	if err != nil {
		return err
	}
	return insertTrades(ctx, db, sides[:])
}

// tradeSides splits a fill into the buyer's (fee in base) and seller's (fee in quote) trades.
func tradeSides(ev events.TradeExecuted) ([2]Trade, error) {
	var out [2]Trade
	vals := make([]decimal.Decimal, 5)
	for i, s := range []string{ev.Price, ev.Qty, ev.QuoteQty, ev.BuyerFee, ev.SellerFee} {
		if s == "" {
			vals[i] = decimal.Zero
			continue
		}
		d, err := decimal.NewFromString(s)
		if err != nil {
			return out, fmt.Errorf("bad decimal %q: %w", s, err)
		}
		vals[i] = d
	}
	price, qty, quote, buyerFee, sellerFee := vals[0], vals[1], vals[2], vals[3], vals[4]
	var baseAsset, quoteAsset string
	if sym, ok := symbols.Get(ev.Symbol); ok {
		baseAsset, quoteAsset = sym.Base, sym.Quote
	}
	out[0] = Trade{
		TradeID: ev.TradeID, OrderID: ev.BuyerOrderID, UserID: ev.BuyerUserID, Symbol: ev.Symbol, Side: SideBuy,
		Price: price, Qty: qty, QuoteQty: quote, Commission: buyerFee, CommissionAsset: baseAsset,
		IsMaker: ev.BuyerIsMaker, Time: ev.Time,
	}
	out[1] = Trade{
		TradeID: ev.TradeID, OrderID: ev.SellerOrderID, UserID: ev.SellerUserID, Symbol: ev.Symbol, Side: SideSell,
		Price: price, Qty: qty, QuoteQty: quote, Commission: sellerFee, CommissionAsset: quoteAsset,
		IsMaker: !ev.BuyerIsMaker, Time: ev.Time,
	}
	return out, nil
}
