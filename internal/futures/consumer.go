package futures

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"simple-securities/pkg/database"
	"simple-securities/pkg/events"
	"simple-securities/pkg/inbox"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/symbols"
)

// ConsumerTopics are the topics of the "futures" consumer group.
func ConsumerTopics() []string {
	return []string{events.TopicFuturesMatchingEvents, events.TopicWalletTransfers}
}

// HandleEvents is the GroupConsumer handler; a batch always comes from one topic-partition.
func (s *Service) HandleEvents(ctx context.Context, msgs []kafka.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	if msgs[0].Topic == events.TopicWalletTransfers {
		return s.handleTransfers(ctx, msgs)
	}
	return s.handleMatching(ctx, msgs)
}

// matchEvent is a decoded matching event (exactly one field set).
type matchEvent struct {
	trade  *events.TradeExecuted
	status *events.OrderStatusChanged
}

// handleMatching applies a batch of futures.matching.events in one transaction (spec 3
// "Applying fills"): positions, wallet, income, user trades and order state, then order.updated
// and one futures.account_update per touched user through the outbox.
func (s *Service) handleMatching(ctx context.Context, msgs []kafka.Message) error {
	emitted := 0
	err := database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		emitted = 0
		evs, err := s.claimMatching(ctx, tx, msgs)
		if err != nil || len(evs) == 0 {
			return err
		}
		b := &fillBatch{s: s, tx: tx, now: s.now(), positions: map[posKey]*Position{},
			touchedPos: map[posKey]bool{}, touchedUsers: map[string]string{}, dirtyOrders: map[string]bool{},
			lastTrade: map[string]int64{}, flattened: map[posKey]bool{}}
		if err := b.load(ctx, evs); err != nil {
			return err
		}
		if len(b.orders) == 0 {
			return nil // none of the orders belong to this service
		}
		for _, ev := range evs {
			switch {
			case ev.trade != nil:
				err = b.applyTrade(ctx, *ev.trade)
			case ev.status != nil:
				err = b.applyStatus(ctx, *ev.status)
			}
			if err != nil {
				return err
			}
		}
		emitted, err = b.flush(ctx)
		return err
	})
	if err != nil {
		return err
	}
	if emitted > 0 {
		s.disp.Notify()
	}
	return nil
}

// claimMatching claims the inbox ids and decodes the new events.
func (s *Service) claimMatching(ctx context.Context, tx *gorm.DB, msgs []kafka.Message) ([]matchEvent, error) {
	out := make([]matchEvent, 0, len(msgs))
	for _, m := range msgs {
		env := m.Envelope
		ok, err := inbox.Claim(ctx, tx, ServiceName, env.ID, env.Type)
		if err != nil {
			return nil, err
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
			out = append(out, matchEvent{trade: &ev})
		case events.TypeOrderStatusChanged:
			var ev events.OrderStatusChanged
			if err := env.Decode(&ev); err != nil {
				s.log.Error("undecodable order.status_changed, skipped", zap.String("event_id", env.ID), zap.Error(err))
				continue
			}
			out = append(out, matchEvent{status: &ev})
		}
	}
	return out, nil
}

// fillBatch is the locked in-memory state of one matching batch.
type fillBatch struct {
	s   *Service
	tx  *gorm.DB
	now time.Time

	orders       map[string]*Order
	accounts     map[string]*Account
	positions    map[posKey]*Position
	insurance    *InsuranceFund
	incomes      []Income
	touchedPos   map[posKey]bool
	touchedUsers map[string]string // user -> account update reason
	dirtyOrders  map[string]bool
	lastTrade    map[string]int64 // order id -> last trade id
	flattened    map[posKey]bool
}

// load reads the orders of the batch that belong to this service and locks their owners'
// accounts and positions (accounts first, then positions; both sorted).
func (b *fillBatch) load(ctx context.Context, evs []matchEvent) error {
	var ids []string
	for _, ev := range evs {
		if ev.trade != nil {
			ids = append(ids, ev.trade.BuyerOrderID, ev.trade.SellerOrderID)
		} else {
			ids = append(ids, ev.status.OrderID)
		}
	}
	orders, err := loadOrders(ctx, b.tx, ids)
	if err != nil {
		return err
	}
	b.orders = orders
	users := make([]string, 0, len(orders))
	for _, o := range orders {
		users = append(users, o.UserID)
	}
	if err := ensureAccounts(ctx, b.tx, users); err != nil {
		return err
	}
	if b.accounts, err = lockAccounts(ctx, b.tx, users); err != nil {
		return err
	}
	positions, err := lockPositions(ctx, b.tx, users)
	if err != nil {
		return err
	}
	for i := range positions {
		b.positions[keyOf(positions[i])] = &positions[i]
	}
	return nil
}

// position returns the locked position, creating a default (flat) one when missing.
func (b *fillBatch) position(userID, symbol string) *Position {
	k := posKey{userID, symbol}
	if p, ok := b.positions[k]; ok {
		return p
	}
	sym, _ := symbols.GetMarket(symbols.MarketFutures, symbol)
	p := &Position{UserID: userID, Symbol: symbol, MarginType: MarginCrossed, Leverage: defaultLeverageFor(sym)}
	b.positions[k] = p
	return p
}

func (b *fillBatch) touch(userID, reason string) {
	if b.touchedUsers[userID] != reasonLiquidation {
		b.touchedUsers[userID] = reason
	}
}

func (b *fillBatch) addIncome(userID, symbol, typ string, amount decimal.Decimal, info string, t time.Time) {
	if amount.IsZero() {
		return
	}
	b.incomes = append(b.incomes, Income{UserID: userID, Symbol: symbol, IncomeType: typ, Income: amount,
		Asset: Asset, Info: info, Time: t})
}

func parseDecimals(vals ...string) ([]decimal.Decimal, error) {
	out := make([]decimal.Decimal, len(vals))
	for i, s := range vals {
		if s == "" {
			continue
		}
		d, err := decimal.NewFromString(s)
		if err != nil {
			return nil, fmt.Errorf("bad decimal %q: %w", s, err)
		}
		out[i] = d
	}
	return out, nil
}

// applyTrade applies both sides of a fill (sides whose order is not ours are skipped).
func (b *fillBatch) applyTrade(ctx context.Context, ev events.TradeExecuted) error {
	vals, err := parseDecimals(ev.Price, ev.Qty, ev.QuoteQty, ev.BuyerFee, ev.SellerFee)
	if err != nil {
		b.s.log.Error("malformed trade.executed, skipped", zap.Int64("trade_id", ev.TradeID), zap.Error(err))
		return nil
	}
	price, qty, quote, buyerFee, sellerFee := vals[0], vals[1], vals[2], vals[3], vals[4]
	if quote.IsZero() {
		quote = price.Mul(qty)
	}
	if ev.Time.IsZero() {
		ev.Time = b.now
	}
	sides := []struct {
		orderID string
		q, fee  decimal.Decimal
		maker   bool
	}{
		{ev.BuyerOrderID, qty, buyerFee, ev.BuyerIsMaker},
		{ev.SellerOrderID, qty.Neg(), sellerFee, !ev.BuyerIsMaker},
	}
	for _, sd := range sides {
		o, ok := b.orders[sd.orderID]
		if !ok {
			continue
		}
		if err := b.applyFill(ctx, o, ev, sd.q, price, quote, money(sd.fee), sd.maker); err != nil {
			return fmt.Errorf("trade %d order %s: %w", ev.TradeID, o.ID, err)
		}
	}
	return nil
}

func (b *fillBatch) applyFill(ctx context.Context, o *Order, ev events.TradeExecuted, q, price, quote,
	fee decimal.Decimal, maker bool) error {
	pos := b.position(o.UserID, o.Symbol)
	acct, ok := b.accounts[o.UserID]
	if !ok {
		return fmt.Errorf("account %s not locked", o.UserID)
	}
	res := computeFill(*pos, q, price, fee, o.Liquidation)
	side := SideBuy
	if q.IsNegative() {
		side = SideSell
	}
	inserted, err := insertTrade(ctx, b.tx, &Trade{
		TradeID: ev.TradeID, OrderID: o.ID, UserID: o.UserID, Symbol: o.Symbol, Side: side, Price: price,
		Qty: q.Abs(), QuoteQty: quote, RealizedPnl: res.Realized, Commission: fee, CommissionAsset: Asset,
		IsMaker: maker, Time: ev.Time,
	})
	if err != nil {
		return err
	}
	if !inserted {
		b.s.log.Warn("duplicate fill ignored", zap.Int64("trade_id", ev.TradeID), zap.String("order_id", o.ID))
		return nil
	}
	*pos = res.Position
	acct.WalletBalance = acct.WalletBalance.Add(res.WalletDelta)
	o.RealizedPnl = o.RealizedPnl.Add(res.Realized)
	o.MarginReleased = o.MarginReleased.Add(releaseForFill(*o, q.Abs()))

	info := fmt.Sprintf("%d", ev.TradeID)
	b.addIncome(o.UserID, o.Symbol, IncomeRealizedPnl, res.Realized, info, ev.Time)
	b.addIncome(o.UserID, o.Symbol, IncomeCommission, fee.Neg(), info, ev.Time)

	k := keyOf(*pos)
	b.touchedPos[k] = true
	b.dirtyOrders[o.ID] = true
	b.lastTrade[o.ID] = ev.TradeID
	if pos.Size.IsZero() {
		b.flattened[k] = true
	}
	reason := reasonOrder
	if o.Liquidation {
		reason = reasonLiquidation
	}
	b.touch(o.UserID, reason)
	return nil
}

// applyStatus advances an order (only by a newer seq) and emits order.updated. A terminal status
// releases the remaining frozen margin and completes a liquidation.
func (b *fillBatch) applyStatus(ctx context.Context, ev events.OrderStatusChanged) error {
	o, ok := b.orders[ev.OrderID]
	if !ok || ev.Seq <= o.LastSeq {
		return nil // not ours, or stale
	}
	vals, err := parseDecimals(ev.ExecutedQty, ev.CumQuoteQty)
	if err != nil {
		b.s.log.Error("malformed order.status_changed, skipped", zap.String("order_id", ev.OrderID), zap.Error(err))
		return nil
	}
	if ev.Time.IsZero() {
		ev.Time = b.now
	}
	o.Status, o.ExecutedQty, o.CumQuote = ev.Status, vals[0], vals[1]
	o.RejectReason, o.LastSeq, o.UpdatedAt = ev.Reason, ev.Seq, ev.Time
	if o.ExecutedQty.IsPositive() {
		o.AvgPrice = o.CumQuote.DivRound(o.ExecutedQty, moneyPlaces)
	}
	terminal := isTerminal(o.Status)
	if terminal {
		o.MarginReleased = o.MarginFrozen
	}
	b.dirtyOrders[o.ID] = true
	if terminal { // the remaining frozen margin is released: available balance changes
		b.touch(o.UserID, reasonOrder)
	}
	if terminal && o.Liquidation {
		if err := b.completeLiquidation(ctx, o); err != nil {
			return err
		}
	}
	return addOrderUpdated(ctx, b.tx, *o, b.lastTrade[o.ID])
}

// completeLiquidation clears the liquidating flag and settles with the insurance fund: the
// remaining isolated margin of a closed ISOLATED position goes to the fund; a negative cross
// wallet (once no cross position is left) is covered by the fund.
func (b *fillBatch) completeLiquidation(ctx context.Context, o *Order) error {
	pos := b.position(o.UserID, o.Symbol)
	acct := b.accounts[o.UserID]
	pos.Liquidating = false
	b.touchedPos[keyOf(*pos)] = true
	b.touch(o.UserID, reasonLiquidation)

	var amount decimal.Decimal // moved from the user to the insurance fund
	switch {
	case pos.MarginType == MarginIsolated:
		if !pos.Size.IsZero() || pos.IsolatedMargin.IsZero() {
			return nil // partially filled: the scan retries
		}
		amount = pos.IsolatedMargin
		pos.IsolatedMargin = decimal.Zero
	case acct != nil && acct.WalletBalance.IsNegative() && !b.hasOpenCross(o.UserID):
		amount = acct.WalletBalance
		acct.WalletBalance = decimal.Zero
	default:
		return nil
	}
	fund, err := b.fund(ctx)
	if err != nil {
		return err
	}
	fund.Balance = fund.Balance.Add(amount)
	b.addIncome(o.UserID, o.Symbol, IncomeInsuranceClear, amount.Neg(), o.ID, b.now)
	b.s.log.Info("liquidation settled with insurance fund", zap.String("user_id", o.UserID),
		zap.String("symbol", o.Symbol), zap.String("amount", amount.String()))
	return nil
}

func (b *fillBatch) hasOpenCross(userID string) bool {
	for k, p := range b.positions {
		if k.UserID == userID && p.MarginType == MarginCrossed && !p.Size.IsZero() {
			return true
		}
	}
	return false
}

func (b *fillBatch) fund(ctx context.Context) (*InsuranceFund, error) {
	if b.insurance == nil {
		f, err := lockInsuranceFund(ctx, b.tx, Asset)
		if err != nil {
			return nil, err
		}
		b.insurance = f
	}
	return b.insurance, nil
}

// flush persists the batch, cancels reduce-only orders of flattened positions and emits one
// account update per touched user. It returns the number of outbox events added (approx.).
func (b *fillBatch) flush(ctx context.Context) (int, error) {
	tx := b.tx
	orderIDs := mapKeys(b.dirtyOrders)
	for _, id := range orderIDs {
		if err := saveOrderProgress(ctx, tx, b.orders[id]); err != nil {
			return 0, err
		}
	}
	keys := make([]posKey, 0, len(b.touchedPos))
	for k := range b.touchedPos {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, comparePosKey)
	for _, k := range keys {
		if err := savePosition(ctx, tx, b.positions[k]); err != nil {
			return 0, err
		}
	}
	users := mapKeys(b.touchedUsers)
	for _, u := range users {
		if err := saveAccount(ctx, tx, b.accounts[u]); err != nil {
			return 0, err
		}
	}
	if b.insurance != nil {
		if err := saveInsuranceFund(ctx, tx, b.insurance); err != nil {
			return 0, err
		}
	}
	if err := insertIncome(ctx, tx, b.incomes); err != nil {
		return 0, err
	}
	emitted := len(orderIDs)
	n, err := b.cancelReduceOnly(ctx)
	if err != nil {
		return 0, err
	}
	emitted += n

	margins, err := openOrderMargins(ctx, tx, users)
	if err != nil {
		return 0, err
	}
	for _, u := range users {
		var positions []Position
		for k, p := range b.positions {
			if k.UserID == u {
				positions = append(positions, *p)
			}
		}
		slices.SortFunc(positions, func(a, c Position) int { return comparePosKey(keyOf(a), keyOf(c)) })
		report := func(p Position) bool { return !p.Size.IsZero() || b.touchedPos[keyOf(p)] }
		if err := b.s.emitAccountUpdate(ctx, tx, b.accounts[u], positions, margins[u], b.touchedUsers[u], report); err != nil {
			return 0, err
		}
		emitted++
	}
	return emitted, nil
}

// cancelReduceOnly requests the cancellation of open reduce-only orders of positions that became
// flat in this batch.
func (b *fillBatch) cancelReduceOnly(ctx context.Context) (int, error) {
	n := 0
	for k := range b.flattened {
		if !b.positions[k].Size.IsZero() {
			continue
		}
		var orders []Order
		err := b.tx.WithContext(ctx).
			Where("user_id = ? AND symbol = ? AND status IN ? AND reduce_only AND NOT liquidation", k.UserID, k.Symbol, openStatuses).
			Find(&orders).Error
		if err != nil {
			return 0, err
		}
		for _, o := range orders {
			if err := addCancel(ctx, b.tx, o); err != nil {
				return 0, err
			}
			n++
		}
	}
	return n, nil
}

func comparePosKey(a, c posKey) int {
	if a.UserID != c.UserID {
		if a.UserID < c.UserID {
			return -1
		}
		return 1
	}
	switch {
	case a.Symbol < c.Symbol:
		return -1
	case a.Symbol > c.Symbol:
		return 1
	}
	return 0
}

func mapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// handleTransfers credits spot -> futures transfers (wallet.transfers), one tx per batch.
func (s *Service) handleTransfers(ctx context.Context, msgs []kafka.Message) error {
	credited := 0
	err := database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		credited = 0
		var transfers []events.Transfer
		for _, m := range msgs {
			env := m.Envelope
			ok, err := inbox.Claim(ctx, tx, ServiceName, env.ID, env.Type)
			if err != nil {
				return err
			}
			if !ok || env.Type != events.TypeSpotToFutures {
				continue
			}
			var ev events.Transfer
			if err := env.Decode(&ev); err != nil {
				s.log.Error("undecodable transfer, skipped", zap.String("event_id", env.ID), zap.Error(err))
				continue
			}
			transfers = append(transfers, ev)
		}
		if len(transfers) == 0 {
			return nil
		}
		users := make([]string, 0, len(transfers))
		for _, t := range transfers {
			users = append(users, t.UserID)
		}
		if err := ensureAccounts(ctx, tx, users); err != nil {
			return err
		}
		accounts, err := lockAccounts(ctx, tx, users)
		if err != nil {
			return err
		}
		touched := map[string]bool{}
		var incomes []Income
		for _, t := range transfers {
			amt, err := s.creditTransfer(ctx, tx, accounts[t.UserID], t)
			if err != nil {
				return err
			}
			if amt.IsPositive() {
				touched[t.UserID] = true
				incomes = append(incomes, Income{UserID: t.UserID, IncomeType: IncomeTransfer, Income: amt,
					Asset: Asset, Info: t.TransferID, Time: s.now()})
			}
		}
		if err := insertIncome(ctx, tx, incomes); err != nil {
			return err
		}
		for _, u := range mapKeys(touched) {
			if err := saveAccount(ctx, tx, accounts[u]); err != nil {
				return err
			}
			if err := s.emitAccountUpdateLoaded(ctx, tx, accounts[u], reasonDeposit, nil); err != nil {
				return err
			}
			credited++
		}
		return nil
	})
	if err != nil {
		return err
	}
	if credited > 0 {
		s.disp.Notify()
	}
	return nil
}

// creditTransfer adds a spot -> futures transfer to the (locked) account and returns the
// credited amount (zero when invalid or already credited).
func (s *Service) creditTransfer(ctx context.Context, tx *gorm.DB, acct *Account, t events.Transfer) (decimal.Decimal, error) {
	amt, err := decimal.NewFromString(t.Amount)
	_, idErr := uuid.Parse(t.TransferID)
	if err != nil || idErr != nil || !amt.IsPositive() || amt.Exponent() < -maxDecimals || t.Asset != Asset || acct == nil {
		s.log.Error("invalid spot->futures transfer, skipped", zap.String("transfer_id", t.TransferID),
			zap.String("asset", t.Asset), zap.String("amount", t.Amount))
		return decimal.Zero, nil
	}
	created := t.Time
	if created.IsZero() {
		created = s.now()
	}
	inserted, err := insertTransfer(ctx, tx, &Transfer{TransferID: t.TransferID, UserID: t.UserID,
		Direction: directionSpotToFutures, Asset: Asset, Amount: amt, CreatedAt: created})
	if err != nil || !inserted {
		return decimal.Zero, err
	}
	acct.WalletBalance = acct.WalletBalance.Add(amt)
	return amt, nil
}
