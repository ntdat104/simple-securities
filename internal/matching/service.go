package matching

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"simple-securities/internal/matching/engine"
	"simple-securities/pkg/database"
	"simple-securities/pkg/events"
	"simple-securities/pkg/inbox"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/symbols"
)

const (
	// ServiceName is the consumer group, inbox consumer and event source name of the spot market.
	ServiceName = "matching"

	depthLevels       = 50
	depthInterval     = 100 * time.Millisecond
	depthFullInterval = 2 * time.Second

	reasonUnknownSymbol  = "UNKNOWN_SYMBOL"
	reasonInvalidDecimal = "INVALID_DECIMAL"
	reasonInvalidOrderID = "INVALID_ORDER_ID"
)

// ServiceNameFor returns the service name of a market: "matching" (spot) or "matching-futures".
func ServiceNameFor(market string) string {
	if market == symbols.MarketFutures {
		return ServiceName + "-futures"
	}
	return ServiceName
}

// Notifier wakes the outbox dispatcher after a commit.
type Notifier interface{ Notify() }

// Publisher publishes best-effort market streams.
type Publisher interface {
	PublishAsync(ctx context.Context, topic, key string, env events.Envelope)
}

// Config configures the Service.
type Config struct {
	// Market is symbols.MarketSpot (default) or symbols.MarketFutures.
	Market string
	// CommandPartitions is the partition count of order.commands; used to preload the books of
	// assigned partitions. 0 disables preloading (books are then loaded lazily on first command).
	CommandPartitions int
}

// Service consumes order commands, runs the engine and persists / emits the results.
type Service struct {
	cfg    Config
	name   string // consumer group / inbox consumer / event source
	topics events.MarketTopics
	db     *gorm.DB
	disp   Notifier
	pub    Publisher
	log    *zap.Logger
	now    func() time.Time

	mu    sync.Mutex
	books map[string]*engine.OrderBook
	owner map[string]int32 // symbol -> order.commands partition
}

// NewService builds the matching service.
func NewService(cfg Config, db *gorm.DB, disp Notifier, pub Publisher, log *zap.Logger) *Service {
	if cfg.Market == "" {
		cfg.Market = symbols.MarketSpot
	}
	return &Service{
		cfg: cfg, db: db, disp: disp, pub: pub, log: log,
		name: ServiceNameFor(cfg.Market), topics: events.TopicsFor(cfg.Market),
		now:   func() time.Time { return time.Now().UTC() },
		books: make(map[string]*engine.OrderBook),
		owner: make(map[string]int32),
	}
}

// HandleBatch processes a batch from one order.commands partition in a single transaction.
func (s *Service) HandleBatch(ctx context.Context, msgs []kafka.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	touched := make(map[string]*engine.OrderBook)
	emitted := 0
	err := database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		w := newBatchWriter(tx)
		for _, m := range msgs {
			ok, err := inbox.Claim(ctx, tx, s.name, m.Envelope.ID, m.Envelope.Type)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			n, err := s.apply(ctx, w, m, touched)
			if err != nil {
				return fmt.Errorf("apply %s %s: %w", m.Envelope.Type, m.Envelope.ID, err)
			}
			emitted += n
		}
		return w.flush(ctx, touched)
	})
	if err != nil {
		// The in-memory books diverged from the database: discard them, they reload on retry.
		for sym := range touched {
			delete(s.books, sym)
			delete(s.owner, sym)
		}
		return err
	}
	if emitted > 0 {
		s.disp.Notify()
	}
	return nil
}

// apply handles one claimed command and returns the number of outbox events written.
func (s *Service) apply(ctx context.Context, w *batchWriter, m kafka.Message, touched map[string]*engine.OrderBook) (int, error) {
	env := m.Envelope
	switch env.Type {
	case events.TypeOrderAccepted:
		var cmd events.OrderAccepted
		if err := env.Decode(&cmd); err != nil {
			s.log.Error("undecodable order.accepted, skipped", zap.String("event_id", env.ID), zap.Error(err))
			return 0, nil
		}
		return s.place(ctx, w, m.Partition, cmd, touched)
	case events.TypeOrderCancelRequested:
		var cmd events.OrderCancelRequested
		if err := env.Decode(&cmd); err != nil {
			s.log.Error("undecodable order.cancel_requested, skipped", zap.String("event_id", env.ID), zap.Error(err))
			return 0, nil
		}
		return s.cancel(ctx, w, m.Partition, cmd, touched)
	default:
		return 0, nil // claimed and ignored
	}
}

func (s *Service) place(ctx context.Context, w *batchWriter, partition int32, cmd events.OrderAccepted,
	touched map[string]*engine.OrderBook,
) (int, error) {
	now := s.now()
	sym, ok := symbols.GetMarket(s.cfg.Market, cmd.Symbol)
	if !ok {
		// No book (and so no sequence) exists for an unknown symbol; the order receives no other
		// event, so seq 1 is enough to pass the consumers' "seq > last_seq" check.
		res := engine.Result{Seq: 1, Updates: []engine.StatusChange{{
			Seq: 1, OrderID: cmd.OrderID, UserID: cmd.UserID, Symbol: cmd.Symbol,
			Status: engine.StatusRejected, Reason: reasonUnknownSymbol, Time: now,
		}}}
		return s.emit(ctx, w, cmd.Symbol, res)
	}
	book, err := s.book(ctx, w, sym, partition)
	if err != nil {
		return 0, err
	}
	touched[sym.Name] = book

	var res engine.Result
	if o, reason := toEngineOrder(cmd); reason != "" {
		res = book.Reject(cmd.OrderID, cmd.UserID, reason, now)
	} else {
		res = book.Place(o, now)
	}
	return s.persist(ctx, w, book, res)
}

func (s *Service) cancel(ctx context.Context, w *batchWriter, partition int32, cmd events.OrderCancelRequested,
	touched map[string]*engine.OrderBook,
) (int, error) {
	sym, ok := symbols.GetMarket(s.cfg.Market, cmd.Symbol)
	if !ok {
		return 0, nil
	}
	book, err := s.book(ctx, w, sym, partition)
	if err != nil {
		return 0, err
	}
	o, ok := book.Order(cmd.OrderID)
	if !ok || (cmd.UserID != "" && o.UserID != cmd.UserID) {
		return 0, nil // already gone (filled / canceled) or not owned: no-op
	}
	touched[sym.Name] = book
	res, _ := book.Cancel(cmd.OrderID, s.now())
	return s.persist(ctx, w, book, res)
}

// book returns the in-memory book, loading it from the database on first use.
func (s *Service) book(ctx context.Context, w *batchWriter, sym symbols.Symbol, partition int32) (*engine.OrderBook, error) {
	s.owner[sym.Name] = partition
	if b, ok := s.books[sym.Name]; ok {
		return b, nil
	}
	b, err := loadBook(ctx, w.tx, sym)
	if err != nil {
		return nil, err
	}
	s.books[sym.Name] = b
	s.log.Info("book loaded", zap.String("symbol", sym.Name), zap.Int("orders", b.Len()),
		zap.Int64("last_update_id", b.Seq()))
	return b, nil
}

// persist writes the book diff and the resulting events.
func (s *Service) persist(ctx context.Context, w *batchWriter, book *engine.OrderBook, res engine.Result) (int, error) {
	if res.Empty() {
		return 0, nil
	}
	for _, o := range res.Resting {
		w.rest(book.Symbol(), o)
	}
	for _, id := range res.Removed {
		w.remove(id)
	}
	return s.emit(ctx, w, book.Symbol(), res)
}

func (s *Service) emit(ctx context.Context, w *batchWriter, symbol string, res engine.Result) (int, error) {
	n := 0
	add := func(typ string, payload any) error {
		env, err := events.New(s.name, typ, symbol, payload)
		if err != nil {
			return err
		}
		n++
		return w.addEvent(s.topics.MatchingEvents, symbol, env)
	}
	for _, t := range res.Trades {
		if err := add(events.TypeTradeExecuted, tradeEvent(t)); err != nil {
			return 0, err
		}
	}
	for _, u := range res.Updates {
		if err := add(events.TypeOrderStatusChanged, statusEvent(u)); err != nil {
			return 0, err
		}
	}
	return n, nil
}

// OnAssigned preloads the books of symbols hashing to the assigned order.commands partitions so
// depth snapshots are available right after a (re)start, before the first command arrives.
func (s *Service) OnAssigned(ctx context.Context, assigned map[string][]int32) {
	n := s.cfg.CommandPartitions
	parts := assigned[s.topics.OrderCommands]
	if n <= 0 || len(parts) == 0 {
		return
	}
	set := make(map[int32]bool, len(parts))
	for _, p := range parts {
		set[p] = true
	}
	partitioner := kgo.StickyKeyPartitioner(nil).ForTopic(s.topics.OrderCommands)

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sym := range symbols.AllMarket(s.cfg.Market) {
		p := int32(partitioner.Partition(&kgo.Record{Key: []byte(sym.Name)}, n))
		if !set[p] {
			continue
		}
		if _, ok := s.books[sym.Name]; ok {
			continue
		}
		b, err := loadBook(ctx, s.db, sym)
		if err != nil {
			s.log.Warn("preload book failed (will load lazily)", zap.String("symbol", sym.Name), zap.Error(err))
			continue
		}
		s.books[sym.Name] = b
		s.owner[sym.Name] = p
		s.log.Info("book preloaded", zap.String("symbol", sym.Name), zap.Int32("partition", p),
			zap.Int("orders", b.Len()))
	}
}

// OnRevoked drops the books of symbols owned by revoked partitions; another instance owns them now.
func (s *Service) OnRevoked(_ context.Context, revoked map[string][]int32) {
	parts := revoked[s.topics.OrderCommands]
	if len(parts) == 0 {
		return
	}
	set := make(map[int32]bool, len(parts))
	for _, p := range parts {
		set[p] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for sym, p := range s.owner {
		if set[p] {
			delete(s.books, sym)
			delete(s.owner, sym)
			s.log.Info("book dropped (partition revoked)", zap.String("symbol", sym), zap.Int32("partition", p))
		}
	}
}

// RunDepthPublisher publishes depth snapshots of changed books every 100ms and of every loaded
// book every 2s.
func (s *Service) RunDepthPublisher(ctx context.Context) error {
	tick := time.NewTicker(depthInterval)
	defer tick.Stop()
	full := time.NewTicker(depthFullInterval)
	defer full.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
			s.publishDepth(ctx, false)
		case <-full.C:
			s.publishDepth(ctx, true)
		}
	}
}

func (s *Service) publishDepth(ctx context.Context, all bool) {
	now := s.now()
	s.mu.Lock()
	snaps := make([]events.DepthSnapshot, 0, len(s.books))
	for _, b := range s.books {
		if !all && !b.Dirty() {
			continue
		}
		snaps = append(snaps, depthEvent(b.Symbol(), b.Depth(depthLevels), now))
		b.ClearDirty()
	}
	s.mu.Unlock()

	for _, snap := range snaps {
		env, err := events.New(s.name, events.TypeDepthSnapshot, snap.Symbol, snap)
		if err != nil {
			s.log.Error("encode depth", zap.Error(err))
			continue
		}
		s.pub.PublishAsync(ctx, s.topics.Depth, snap.Symbol, env)
	}
}

// toEngineOrder converts a command; a non-empty reason means the command is invalid.
func toEngineOrder(cmd events.OrderAccepted) (engine.Order, string) {
	if _, err := uuid.Parse(cmd.OrderID); err != nil {
		return engine.Order{}, reasonInvalidOrderID
	}
	var price, qty, quote decimal.Decimal
	for _, f := range []struct {
		s   string
		dst *decimal.Decimal
	}{{cmd.Price, &price}, {cmd.Quantity, &qty}, {cmd.QuoteQuantity, &quote}} {
		if f.s == "" {
			continue
		}
		v, err := decimal.NewFromString(f.s)
		if err != nil {
			return engine.Order{}, reasonInvalidDecimal
		}
		*f.dst = v
	}
	createdAt := cmd.CreatedAt.UTC()
	return engine.Order{
		ID:          cmd.OrderID,
		UserID:      cmd.UserID,
		Side:        engine.Side(cmd.Side),
		Type:        engine.OrderType(cmd.Type),
		TIF:         engine.TimeInForce(cmd.TimeInForce),
		Price:       price,
		Qty:         qty,
		QuoteBudget: quote,
		CreatedAt:   createdAt,
	}, ""
}

func tradeEvent(t engine.Trade) events.TradeExecuted {
	return events.TradeExecuted{
		TradeID: t.ID, Seq: t.Seq, Symbol: t.Symbol,
		Price: t.Price.String(), Qty: t.Qty.String(), QuoteQty: t.QuoteQty.String(),
		BuyerOrderID: t.BuyerOrderID, SellerOrderID: t.SellerOrderID,
		BuyerUserID: t.BuyerUserID, SellerUserID: t.SellerUserID, BuyerIsMaker: t.BuyerIsMaker,
		BuyerFee: t.BuyerFee.String(), SellerFee: t.SellerFee.String(), Time: t.Time,
	}
}

func statusEvent(u engine.StatusChange) events.OrderStatusChanged {
	return events.OrderStatusChanged{
		Seq: u.Seq, OrderID: u.OrderID, UserID: u.UserID, Symbol: u.Symbol, Status: string(u.Status),
		ExecutedQty: u.ExecutedQty.String(), CumQuoteQty: u.CumQuoteQty.String(), Reason: u.Reason,
		Time: u.Time,
	}
}

func depthEvent(symbol string, d engine.Depth, now time.Time) events.DepthSnapshot {
	conv := func(levels []engine.Level) []events.PriceLevel {
		out := make([]events.PriceLevel, len(levels))
		for i, l := range levels {
			out[i] = events.PriceLevel{l.Price.String(), l.Qty.String()}
		}
		return out
	}
	return events.DepthSnapshot{
		Symbol: symbol, LastUpdateID: d.LastUpdateID, Bids: conv(d.Bids), Asks: conv(d.Asks), Time: now,
	}
}
