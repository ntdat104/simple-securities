// Package matching implements the matching engine service.
package matching

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"simple-securities/internal/matching/engine"
	"simple-securities/pkg/events"
	"simple-securities/pkg/outbox"
	"simple-securities/pkg/symbols"
)

// BookOrder is a resting order of an in-memory book (table book_orders).
type BookOrder struct {
	OrderID   string          `gorm:"type:uuid;primaryKey"`
	Symbol    string          `gorm:"not null;index:idx_book_orders_symbol_priority,priority:1"`
	UserID    string          `gorm:"not null"`
	Side      string          `gorm:"not null"`
	Type      string          `gorm:"not null"`
	TIF       string          `gorm:"column:tif;not null"`
	Price     decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	Qty       decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	Remaining decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	Executed  decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	CumQuote  decimal.Decimal `gorm:"type:numeric(36,18);not null;default:0"`
	Priority  int64           `gorm:"not null;index:idx_book_orders_symbol_priority,priority:2"`
	CreatedAt time.Time       `gorm:"not null;default:now()"`
}

// TableName implements gorm.Tabler.
func (BookOrder) TableName() string { return "book_orders" }

// SymbolState holds a book's id counters (table symbol_state).
type SymbolState struct {
	Symbol       string `gorm:"primaryKey"`
	LastTradeID  int64  `gorm:"not null;default:0"`
	LastUpdateID int64  `gorm:"not null;default:0"`
}

// TableName implements gorm.Tabler.
func (SymbolState) TableName() string { return "symbol_state" }

// Models returns the service's own models (outbox/inbox are added by the caller).
func Models() []any { return []any{&BookOrder{}, &SymbolState{}} }

// insertBatchSize bounds rows per INSERT (Postgres allows 65535 bind parameters per statement).
const insertBatchSize = 1000

func toBookOrder(symbol string, o engine.Order) BookOrder {
	return BookOrder{
		OrderID: o.ID, Symbol: symbol, UserID: o.UserID, Side: string(o.Side), Type: string(o.Type),
		TIF: string(o.TIF), Price: o.Price, Qty: o.Qty, Remaining: o.Remaining, Executed: o.Executed,
		CumQuote: o.CumQuote, Priority: o.Priority, CreatedAt: o.CreatedAt,
	}
}

func (r BookOrder) engineOrder() engine.Order {
	return engine.Order{
		ID: r.OrderID, UserID: r.UserID, Side: engine.Side(r.Side), Type: engine.OrderType(r.Type),
		TIF: engine.TimeInForce(r.TIF), Price: r.Price, Qty: r.Qty, Remaining: r.Remaining,
		Executed: r.Executed, CumQuote: r.CumQuote, Priority: r.Priority, CreatedAt: r.CreatedAt,
	}
}

// loadBook rebuilds a symbol's book from book_orders (priority order) and symbol_state.
func loadBook(ctx context.Context, db *gorm.DB, sym symbols.Symbol) (*engine.OrderBook, error) {
	db = db.WithContext(ctx)
	var st SymbolState
	if err := db.Where("symbol = ?", sym.Name).Take(&st).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("load symbol_state %s: %w", sym.Name, err)
	}
	book := engine.NewOrderBook(sym, st.LastTradeID, st.LastUpdateID)

	var rows []BookOrder
	if err := db.Where("symbol = ?", sym.Name).Order("priority").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load book_orders %s: %w", sym.Name, err)
	}
	for _, r := range rows {
		if err := book.Restore(r.engineOrder()); err != nil {
			return nil, fmt.Errorf("restore %s: %w", sym.Name, err)
		}
	}
	return book, nil
}

// batchWriter accumulates the writes of a whole Kafka batch (book diff, outbox rows) and flushes
// them with a handful of statements inside the batch transaction. Reads (book loads) go straight
// to the transaction.
type batchWriter struct {
	tx      *gorm.DB
	resting map[string]BookOrder // order id -> latest resting state
	order   []string             // first-touch order of resting ids (deterministic statements)
	removed map[string]bool      // order ids that left the book
	outbox  []outbox.Event
}

func newBatchWriter(tx *gorm.DB) *batchWriter {
	return &batchWriter{tx: tx, resting: make(map[string]BookOrder), removed: make(map[string]bool)}
}

// rest records an order (still) resting in the book; the latest state wins.
func (w *batchWriter) rest(symbol string, o engine.Order) {
	if _, ok := w.resting[o.ID]; !ok {
		w.order = append(w.order, o.ID)
	}
	w.resting[o.ID] = toBookOrder(symbol, o)
	delete(w.removed, o.ID)
}

// remove records an order that left the book.
func (w *batchWriter) remove(id string) {
	delete(w.resting, id)
	w.removed[id] = true
}

// addEvent queues an outbox row, built exactly like outbox.Add does.
func (w *batchWriter) addEvent(topic, key string, env events.Envelope) error {
	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	w.outbox = append(w.outbox, outbox.Event{EventID: env.ID, Topic: topic, PartitionKey: key,
		EventType: env.Type, AggregateID: env.AggregateID, Payload: payload, CreatedAt: time.Now().UTC()})
	return nil
}

// flush writes the accumulated book diff, the symbol states of the touched books and the outbox
// rows using the batch transaction.
func (w *batchWriter) flush(ctx context.Context, touched map[string]*engine.OrderBook) error {
	tx := w.tx.WithContext(ctx)
	if len(w.resting) > 0 {
		rows := make([]BookOrder, 0, len(w.resting))
		for _, id := range w.order {
			if r, ok := w.resting[id]; ok {
				rows = append(rows, r)
			}
		}
		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "order_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"remaining", "executed", "cum_quote"}),
		}).CreateInBatches(&rows, insertBatchSize).Error
		if err != nil {
			return fmt.Errorf("upsert %d book orders: %w", len(rows), err)
		}
	}
	if len(w.removed) > 0 {
		ids := make([]string, 0, len(w.removed))
		for id := range w.removed {
			ids = append(ids, id)
		}
		if err := tx.Where("order_id IN ?", ids).Delete(&BookOrder{}).Error; err != nil {
			return fmt.Errorf("delete %d book orders: %w", len(ids), err)
		}
	}
	if len(touched) > 0 {
		states := make([]SymbolState, 0, len(touched))
		for _, b := range touched {
			states = append(states, SymbolState{Symbol: b.Symbol(), LastTradeID: b.LastTradeID(), LastUpdateID: b.Seq()})
		}
		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "symbol"}},
			DoUpdates: clause.AssignmentColumns([]string{"last_trade_id", "last_update_id"}),
		}).Create(&states).Error
		if err != nil {
			return fmt.Errorf("save symbol_state: %w", err)
		}
	}
	if len(w.outbox) > 0 {
		if err := tx.CreateInBatches(&w.outbox, insertBatchSize).Error; err != nil {
			return fmt.Errorf("insert %d outbox events: %w", len(w.outbox), err)
		}
	}
	return nil
}
