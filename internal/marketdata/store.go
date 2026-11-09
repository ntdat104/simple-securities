package marketdata

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"simple-securities/pkg/database"
	"simple-securities/pkg/inbox"
)

const (
	// insertBatchSize bounds rows per INSERT statement (11 params/kline row stays far below the
	// 65535 bind-parameter limit of the Postgres protocol).
	insertBatchSize = 1000
)

// Store is the marketdata repository.
type Store struct {
	db           *gorm.DB
	consumerName string // inbox consumer name
}

// NewStore returns a Store backed by db; consumerName is the inbox consumer (ServiceNameFor(market)).
func NewStore(db *gorm.DB, consumerName string) *Store {
	return &Store{db: db, consumerName: consumerName}
}

// tradeMsg is a trade together with the envelope identity used for inbox dedup.
type tradeMsg struct {
	EventID   string
	EventType string
	Trade     trade
}

// klineUpsert merges a batch delta into the stored kline bucket.
var klineUpsert = clause.OnConflict{
	Columns: []clause.Column{{Name: "symbol"}, {Name: "interval"}, {Name: "open_time"}},
	DoUpdates: clause.Assignments(map[string]any{
		"high":         gorm.Expr("GREATEST(klines.high, excluded.high)"),
		"low":          gorm.Expr("LEAST(klines.low, excluded.low)"),
		"close":        gorm.Expr("excluded.close"),
		"volume":       gorm.Expr("klines.volume + excluded.volume"),
		"quote_volume": gorm.Expr("klines.quote_volume + excluded.quote_volume"),
		"trade_count":  gorm.Expr("klines.trade_count + excluded.trade_count"),
	}),
}

// ApplyTrades stores a batch of trades in one transaction (inbox dedup, public trades, kline
// upserts) and returns the merged state of every touched kline bucket.
func (s *Store) ApplyTrades(ctx context.Context, msgs []tradeMsg) ([]*bucket, error) {
	var buckets []*bucket
	err := database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		trades := make([]trade, 0, len(msgs))
		for _, m := range msgs {
			ok, err := inbox.Claim(ctx, tx, s.consumerName, m.EventID, m.EventType)
			if err != nil {
				return err
			}
			if ok {
				trades = append(trades, m.Trade)
			}
		}
		if len(trades) == 0 {
			buckets = nil
			return nil
		}
		deltas := aggregate(trades)

		rows := make([]PublicTrade, len(trades))
		for i, t := range trades {
			rows[i] = newPublicTrade(t)
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
			CreateInBatches(&rows, insertBatchSize).Error; err != nil {
			return fmt.Errorf("insert public trades: %w", err)
		}

		klines := make([]Kline, len(deltas))
		for i, b := range deltas {
			klines[i] = newKline(b)
		}
		// RETURNING * writes the merged rows back into klines. Postgres does not formally
		// guarantee RETURNING order for multi-row inserts, so results are matched by key below.
		if err := tx.Clauses(klineUpsert, clause.Returning{}).
			CreateInBatches(&klines, insertBatchSize).Error; err != nil {
			return fmt.Errorf("upsert klines: %w", err)
		}
		merged := make(map[bucketKey]*bucket, len(klines))
		for _, k := range klines {
			b := k.toBucket()
			merged[b.bucketKey] = b
		}
		out := make([]*bucket, 0, len(deltas))
		for _, d := range deltas {
			m, ok := merged[d.bucketKey]
			if !ok {
				return fmt.Errorf("upsert klines: no merged row for %s %s %d", d.Symbol, d.Interval, d.OpenTime)
			}
			out = append(out, m)
		}
		buckets = out
		return nil
	})
	if err != nil {
		return nil, err
	}
	return buckets, nil
}

// tickerRow is one row of the 24h ticker aggregate.
type tickerRow struct {
	Symbol      string
	Open        decimal.Decimal
	High        decimal.Decimal
	Low         decimal.Decimal
	Last        decimal.Decimal
	Volume      decimal.Decimal
	QuoteVolume decimal.Decimal
	Count       int64
}

// TickerStats aggregates 1m klines since `since` for the given symbols. Symbols without klines in
// the window fall back to their last trade price (or zero when they never traded).
func (s *Store) TickerStats(ctx context.Context, syms []string, since time.Time) (map[string]tickerStats, error) {
	out := make(map[string]tickerStats, len(syms))
	if len(syms) == 0 {
		return out, nil
	}
	var rows []tickerRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT symbol,
		       (array_agg(open ORDER BY open_time ASC))[1]   AS open,
		       max(high)                                     AS high,
		       min(low)                                      AS low,
		       (array_agg(close ORDER BY open_time DESC))[1] AS last,
		       sum(volume)                                   AS volume,
		       sum(quote_volume)                             AS quote_volume,
		       sum(trade_count)::bigint                      AS count
		FROM klines
		WHERE interval = '1m' AND open_time >= ? AND symbol IN ?
		GROUP BY symbol`, since, syms).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("ticker stats: %w", err)
	}
	for _, r := range rows {
		out[r.Symbol] = tickerStats{Open: r.Open, High: r.High, Low: r.Low, Last: r.Last,
			Volume: r.Volume, QuoteVolume: r.QuoteVolume, Count: r.Count}
	}
	for _, sym := range syms {
		if _, ok := out[sym]; ok {
			continue
		}
		var prices []decimal.Decimal
		err := s.db.WithContext(ctx).Model(&PublicTrade{}).
			Where("symbol = ?", sym).Order("trade_id DESC").Limit(1).Pluck("price", &prices).Error
		switch {
		case err != nil:
			return nil, fmt.Errorf("last price: %w", err)
		case len(prices) == 0:
			out[sym] = tickerStats{}
		default:
			last := prices[0]
			out[sym] = tickerStats{Open: last, High: last, Low: last, Last: last}
		}
	}
	return out, nil
}

// Klines returns up to limit klines with open_time <= end, ascending by open_time.
func (s *Store) Klines(ctx context.Context, symbol string, iv Interval, end time.Time, limit int) ([]*bucket, error) {
	var rows []Kline
	err := s.db.WithContext(ctx).
		Where("symbol = ? AND interval = ? AND open_time <= ?", symbol, iv.Name, end).
		Order("open_time DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("query klines: %w", err)
	}
	out := make([]*bucket, len(rows))
	for i, k := range rows {
		out[len(rows)-1-i] = k.toBucket()
	}
	return out, nil
}

// Trades returns the last limit public trades, oldest first.
func (s *Store) Trades(ctx context.Context, symbol string, limit int) ([]trade, error) {
	var rows []PublicTrade
	err := s.db.WithContext(ctx).Where("symbol = ?", symbol).
		Order("trade_id DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("query trades: %w", err)
	}
	out := make([]trade, len(rows))
	for i, p := range rows {
		out[len(rows)-1-i] = p.toTrade()
	}
	return out, nil
}
