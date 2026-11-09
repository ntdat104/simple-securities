package engine

import (
	"container/list"
	"errors"
	"fmt"
	"time"

	"github.com/google/btree"
	"github.com/shopspring/decimal"

	"simple-securities/pkg/symbols"
)

const btreeDegree = 32

// priceLevel is a FIFO queue of resting orders at one price.
type priceLevel struct {
	price  decimal.Decimal
	total  decimal.Decimal // sum of remaining quantities
	orders *list.List      // of *Order, oldest first
}

type restingRef struct {
	level *priceLevel
	elem  *list.Element
}

// OrderBook is the order book of one symbol. It is NOT safe for concurrent use.
type OrderBook struct {
	sym         symbols.Symbol
	bids        *btree.BTreeG[*priceLevel] // highest price first
	asks        *btree.BTreeG[*priceLevel] // lowest price first
	index       map[string]restingRef
	lastTradeID int64
	seq         int64
	dirty       bool
}

// NewOrderBook creates an empty book continuing the given trade id and update id sequences.
func NewOrderBook(sym symbols.Symbol, lastTradeID, lastUpdateID int64) *OrderBook {
	return &OrderBook{
		sym:         sym,
		bids:        btree.NewG(btreeDegree, func(a, b *priceLevel) bool { return a.price.GreaterThan(b.price) }),
		asks:        btree.NewG(btreeDegree, func(a, b *priceLevel) bool { return a.price.LessThan(b.price) }),
		index:       make(map[string]restingRef),
		lastTradeID: lastTradeID,
		seq:         lastUpdateID,
	}
}

// Symbol returns the symbol name.
func (b *OrderBook) Symbol() string { return b.sym.Name }

// Seq returns the last update id.
func (b *OrderBook) Seq() int64 { return b.seq }

// LastTradeID returns the last trade id.
func (b *OrderBook) LastTradeID() int64 { return b.lastTradeID }

// Len returns the number of resting orders.
func (b *OrderBook) Len() int { return len(b.index) }

// Dirty reports whether the visible book changed since ClearDirty.
func (b *OrderBook) Dirty() bool { return b.dirty }

// ClearDirty resets the dirty flag (after a depth snapshot was taken).
func (b *OrderBook) ClearDirty() { b.dirty = false }

// Order returns a copy of a resting order.
func (b *OrderBook) Order(id string) (Order, bool) {
	ref, ok := b.index[id]
	if !ok {
		return Order{}, false
	}
	return *ref.elem.Value.(*Order), true
}

// Orders returns copies of all resting orders, bids then asks, in priority order per level.
func (b *OrderBook) Orders() []Order {
	out := make([]Order, 0, len(b.index))
	collect := func(l *priceLevel) bool {
		for e := l.orders.Front(); e != nil; e = e.Next() {
			out = append(out, *e.Value.(*Order))
		}
		return true
	}
	b.bids.Ascend(collect)
	b.asks.Ascend(collect)
	return out
}

// BestBid returns the highest bid price.
func (b *OrderBook) BestBid() (decimal.Decimal, bool) { return best(b.bids) }

// BestAsk returns the lowest ask price.
func (b *OrderBook) BestAsk() (decimal.Decimal, bool) { return best(b.asks) }

func best(t *btree.BTreeG[*priceLevel]) (decimal.Decimal, bool) {
	l, ok := t.Min()
	if !ok {
		return decimal.Zero, false
	}
	return l.price, true
}

// Restore appends an already-accepted resting order (e.g. loaded from storage).
// Orders must be restored in priority order.
func (b *OrderBook) Restore(o Order) error {
	if _, ok := b.index[o.ID]; ok {
		return fmt.Errorf("order %s already in book", o.ID)
	}
	if o.Side != SideBuy && o.Side != SideSell {
		return fmt.Errorf("order %s: invalid side %q", o.ID, o.Side)
	}
	if !o.Price.IsPositive() || !o.Remaining.IsPositive() {
		return errors.New("restored order needs a positive price and remaining quantity")
	}
	b.rest(&o)
	return nil
}

// Depth returns up to limit aggregated levels per side.
func (b *OrderBook) Depth(limit int) Depth {
	d := Depth{LastUpdateID: b.seq, Bids: []Level{}, Asks: []Level{}}
	walk := func(dst *[]Level) func(*priceLevel) bool {
		return func(l *priceLevel) bool {
			if len(*dst) >= limit {
				return false
			}
			*dst = append(*dst, Level{Price: l.price, Qty: l.total})
			return true
		}
	}
	b.bids.Ascend(walk(&d.Bids))
	b.asks.Ascend(walk(&d.Asks))
	return d
}

// Reject consumes a sequence number and reports the order as REJECTED (invalid command).
func (b *OrderBook) Reject(orderID, userID, reason string, now time.Time) Result {
	b.seq++
	res := Result{Seq: b.seq}
	res.Updates = append(res.Updates, StatusChange{
		Seq: b.seq, OrderID: orderID, UserID: userID, Symbol: b.sym.Name, Status: StatusRejected,
		ExecutedQty: decimal.Zero, CumQuoteQty: decimal.Zero, Reason: reason, Time: now,
	})
	return res
}

// Cancel removes a resting order. It returns false (and an empty result) when the order is not
// resting, e.g. it was filled in the meantime.
func (b *OrderBook) Cancel(orderID string, now time.Time) (Result, bool) {
	ref, ok := b.index[orderID]
	if !ok {
		return Result{}, false
	}
	o := ref.elem.Value.(*Order)
	b.remove(o.Side, ref)
	b.seq++
	res := Result{Seq: b.seq, Removed: []string{o.ID}}
	res.Updates = append(res.Updates, b.statusOf(o, StatusCanceled, "", now))
	return res, true
}

// Place matches a new order against the book and rests any GTC remainder.
// Placing an order id that is already resting is a no-op (empty result).
func (b *OrderBook) Place(in Order, now time.Time) Result {
	if _, exists := b.index[in.ID]; exists {
		return Result{}
	}
	b.seq++
	res := Result{Seq: b.seq}

	o := in
	o.Priority = b.seq
	o.Executed = decimal.Zero
	o.CumQuote = decimal.Zero
	o.byQuote = o.Type == TypeMarket && o.Side == SideBuy &&
		!(b.sym.FeeInQuote && !o.QuoteBudget.IsPositive() && o.Qty.IsPositive())
	if o.isMarketBuy() {
		o.Qty, o.Remaining = decimal.Zero, decimal.Zero
	} else {
		o.Remaining = o.Qty
	}
	if o.CreatedAt.IsZero() {
		o.CreatedAt = now
	}

	if !valid(&o) {
		res.Updates = append(res.Updates, b.statusOf(&o, StatusRejected, ReasonInvalidOrder, now))
		return res
	}

	switch {
	case o.Type == TypeLimitMaker:
		if b.crosses(&o) {
			res.Updates = append(res.Updates, b.statusOf(&o, StatusExpired, ReasonPostOnlyWouldCross, now))
			return res
		}
		b.rest(&o)
		res.Resting = append(res.Resting, o)
		res.Updates = append(res.Updates, b.statusOf(&o, StatusNew, "", now))
		return res
	case o.Type == TypeLimit && o.TIF == FOK:
		if !b.canFill(&o) {
			res.Updates = append(res.Updates, b.statusOf(&o, StatusExpired, "", now))
			return res
		}
	}

	m := b.match(&o, &res, now)

	for _, mk := range m.makers {
		if ref, resting := b.index[mk.ID]; resting {
			res.Resting = append(res.Resting, *ref.elem.Value.(*Order))
			res.Updates = append(res.Updates, b.statusOf(mk, StatusPartiallyFilled, "", now))
		} else {
			res.Removed = append(res.Removed, mk.ID)
			res.Updates = append(res.Updates, b.statusOf(mk, StatusFilled, "", now))
		}
	}

	var (
		status Status
		reason string
	)
	switch {
	case m.selfTrade:
		status, reason = StatusExpired, ReasonSelfTradePrevention
	case o.isMarketBuy():
		if m.budgetExhausted && o.Executed.IsPositive() {
			status = StatusFilled
		} else {
			status = StatusExpired
		}
	case o.Remaining.IsZero():
		status = StatusFilled
	case o.Type == TypeMarket || o.TIF == IOC || o.TIF == FOK:
		status = StatusExpired
	default: // GTC limit remainder rests
		b.rest(&o)
		res.Resting = append(res.Resting, o)
		status = StatusNew
		if o.Executed.IsPositive() {
			status = StatusPartiallyFilled
		}
	}
	res.Updates = append(res.Updates, b.statusOf(&o, status, reason, now))
	return res
}

func valid(o *Order) bool {
	if o.ID == "" || (o.Side != SideBuy && o.Side != SideSell) {
		return false
	}
	switch o.Type {
	case TypeLimit:
		if o.TIF == "" {
			o.TIF = GTC
		}
		if o.TIF != GTC && o.TIF != IOC && o.TIF != FOK {
			return false
		}
		return o.Price.IsPositive() && o.Qty.IsPositive()
	case TypeLimitMaker:
		if o.TIF != "" && o.TIF != GTC {
			return false
		}
		o.TIF = GTC
		return o.Price.IsPositive() && o.Qty.IsPositive()
	case TypeMarket:
		if o.isMarketBuy() {
			return o.QuoteBudget.IsPositive()
		}
		return o.Qty.IsPositive()
	}
	return false
}

func (b *OrderBook) opposite(s Side) *btree.BTreeG[*priceLevel] {
	if s == SideBuy {
		return b.asks
	}
	return b.bids
}

func (b *OrderBook) own(s Side) *btree.BTreeG[*priceLevel] {
	if s == SideBuy {
		return b.bids
	}
	return b.asks
}

// acceptable reports whether a taker may trade at a maker level price.
func acceptable(o *Order, price decimal.Decimal) bool {
	if o.Type == TypeMarket {
		return true
	}
	if o.Side == SideBuy {
		return price.LessThanOrEqual(o.Price)
	}
	return price.GreaterThanOrEqual(o.Price)
}

func (b *OrderBook) crosses(o *Order) bool {
	l, ok := b.opposite(o.Side).Min()
	return ok && acceptable(o, l.price)
}

// canFill pre-checks a FOK order: the fillable quantity before hitting an own order (which would
// trigger self-trade prevention) must cover the whole order.
func (b *OrderBook) canFill(o *Order) bool {
	need := o.Remaining
	acc := decimal.Zero
	ok := false
	b.opposite(o.Side).Ascend(func(l *priceLevel) bool {
		if !acceptable(o, l.price) {
			return false
		}
		for e := l.orders.Front(); e != nil; e = e.Next() {
			m := e.Value.(*Order)
			if m.UserID == o.UserID {
				return false
			}
			acc = acc.Add(m.Remaining)
			if acc.GreaterThanOrEqual(need) {
				ok = true
				return false
			}
		}
		return true
	})
	return ok
}

type matchOutcome struct {
	makers          []*Order // unique, in touch order
	selfTrade       bool
	budgetExhausted bool
}

func (b *OrderBook) match(o *Order, res *Result, now time.Time) matchOutcome {
	var out matchOutcome
	opp := b.opposite(o.Side)
	for {
		var budget decimal.Decimal
		if o.isMarketBuy() {
			budget = o.QuoteBudget.Sub(o.CumQuote)
			if !budget.IsPositive() {
				out.budgetExhausted = true
				break
			}
		} else if !o.Remaining.IsPositive() {
			break
		}
		lvl, ok := opp.Min()
		if !ok || !acceptable(o, lvl.price) {
			break
		}
		front := lvl.orders.Front()
		maker := front.Value.(*Order)
		if maker.UserID == o.UserID {
			out.selfTrade = true
			break
		}
		var qty decimal.Decimal
		if o.isMarketBuy() {
			afford := b.affordable(budget, lvl.price)
			if !afford.IsPositive() {
				out.budgetExhausted = true
				break
			}
			qty = decimal.Min(maker.Remaining, afford)
		} else {
			qty = decimal.Min(maker.Remaining, o.Remaining)
		}
		res.Trades = append(res.Trades, b.fill(o, maker, lvl, qty, now))
		if len(out.makers) == 0 || out.makers[len(out.makers)-1] != maker {
			out.makers = append(out.makers, maker)
		}
		if maker.Remaining.IsZero() {
			b.remove(maker.Side, restingRef{level: lvl, elem: front})
		}
	}
	return out
}

// affordable is the largest step multiple whose cost at price does not exceed budget.
func (b *OrderBook) affordable(budget, price decimal.Decimal) decimal.Decimal {
	step := b.sym.StepSize
	q := symbols.FloorToStep(budget.Div(price), step)
	if q.Mul(price).GreaterThan(budget) && step.IsPositive() {
		q = q.Sub(step)
	}
	if q.IsNegative() {
		return decimal.Zero
	}
	return q
}

func (b *OrderBook) fill(taker, maker *Order, lvl *priceLevel, qty decimal.Decimal, now time.Time) Trade {
	price := lvl.price
	quote := price.Mul(qty)
	for _, x := range []*Order{taker, maker} {
		if !x.isMarketBuy() {
			x.Remaining = x.Remaining.Sub(qty)
		}
		x.Executed = x.Executed.Add(qty)
		x.CumQuote = x.CumQuote.Add(quote)
	}
	lvl.total = lvl.total.Sub(qty)
	b.dirty = true
	b.lastTradeID++

	buyer, seller := taker, maker
	if taker.Side == SideSell {
		buyer, seller = maker, taker
	}
	buyerIsMaker := buyer == maker
	buyerRate, sellerRate := b.sym.TakerFee, b.sym.MakerFee
	if buyerIsMaker {
		buyerRate, sellerRate = b.sym.MakerFee, b.sym.TakerFee
	}
	buyerFeeBase := qty // spot: the buyer pays its fee in the base asset it receives
	if b.sym.FeeInQuote {
		buyerFeeBase = quote // futures: both sides pay in the quote asset
	}
	return Trade{
		ID:            b.lastTradeID,
		Seq:           b.seq,
		Symbol:        b.sym.Name,
		Price:         price,
		Qty:           qty,
		QuoteQty:      quote,
		BuyerOrderID:  buyer.ID,
		SellerOrderID: seller.ID,
		BuyerUserID:   buyer.UserID,
		SellerUserID:  seller.UserID,
		BuyerIsMaker:  buyerIsMaker,
		BuyerFee:      buyerFeeBase.Mul(buyerRate).Truncate(feeScale),
		SellerFee:     quote.Mul(sellerRate).Truncate(feeScale),
		Time:          now,
	}
}

// rest appends o (which must have a positive remaining) to its price level.
func (b *OrderBook) rest(o *Order) {
	tree := b.own(o.Side)
	lvl, ok := tree.Get(&priceLevel{price: o.Price})
	if !ok {
		lvl = &priceLevel{price: o.Price, total: decimal.Zero, orders: list.New()}
		tree.ReplaceOrInsert(lvl)
	}
	if o.Priority == 0 {
		o.Priority = b.seq
	}
	elem := lvl.orders.PushBack(o)
	lvl.total = lvl.total.Add(o.Remaining)
	b.index[o.ID] = restingRef{level: lvl, elem: elem}
	b.dirty = true
}

func (b *OrderBook) remove(side Side, ref restingRef) {
	o := ref.elem.Value.(*Order)
	ref.level.orders.Remove(ref.elem)
	ref.level.total = ref.level.total.Sub(o.Remaining)
	delete(b.index, o.ID)
	if ref.level.orders.Len() == 0 {
		b.own(side).Delete(ref.level)
	}
	b.dirty = true
}

func (b *OrderBook) statusOf(o *Order, s Status, reason string, now time.Time) StatusChange {
	return StatusChange{
		Seq: b.seq, OrderID: o.ID, UserID: o.UserID, Symbol: b.sym.Name, Status: s,
		ExecutedQty: o.Executed, CumQuoteQty: o.CumQuote, Reason: reason, Time: now,
	}
}
