package marketdata

import (
	"time"

	"github.com/shopspring/decimal"
)

// PublicTrade is a public trade print (table public_trades), primary key (symbol, trade_id).
type PublicTrade struct {
	Symbol       string          `gorm:"type:text;primaryKey;not null"`
	TradeID      int64           `gorm:"type:bigint;primaryKey;autoIncrement:false;not null"`
	Price        decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	Qty          decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	QuoteQty     decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	IsBuyerMaker bool            `gorm:"type:boolean;not null"`
	Time         time.Time       `gorm:"type:timestamptz;not null"`
}

// TableName implements gorm.Tabler.
func (PublicTrade) TableName() string { return "public_trades" }

// Kline is one candlestick bucket (table klines), primary key (symbol, interval, open_time).
type Kline struct {
	Symbol      string          `gorm:"type:text;primaryKey;not null"`
	Interval    string          `gorm:"type:text;primaryKey;not null"`
	OpenTime    time.Time       `gorm:"type:timestamptz;primaryKey;not null"`
	CloseTime   time.Time       `gorm:"type:timestamptz;not null"`
	Open        decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	High        decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	Low         decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	Close       decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	Volume      decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	QuoteVolume decimal.Decimal `gorm:"type:numeric(36,18);not null"`
	TradeCount  int64           `gorm:"type:bigint;not null"`
}

// TableName implements gorm.Tabler.
func (Kline) TableName() string { return "klines" }

func newPublicTrade(t trade) PublicTrade {
	return PublicTrade{Symbol: t.Symbol, TradeID: t.ID, Price: t.Price, Qty: t.Qty,
		QuoteQty: t.QuoteQty, IsBuyerMaker: t.BuyerIsMaker, Time: t.Time.UTC()}
}

func (p PublicTrade) toTrade() trade {
	return trade{Symbol: p.Symbol, ID: p.TradeID, Price: p.Price, Qty: p.Qty, QuoteQty: p.QuoteQty,
		BuyerIsMaker: p.IsBuyerMaker, Time: p.Time}
}

func newKline(b *bucket) Kline {
	return Kline{Symbol: b.Symbol, Interval: b.Interval,
		OpenTime: time.UnixMilli(b.OpenTime).UTC(), CloseTime: time.UnixMilli(b.CloseTime).UTC(),
		Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, Volume: b.Volume,
		QuoteVolume: b.QuoteVolume, TradeCount: b.Count}
}

func (k Kline) toBucket() *bucket {
	return &bucket{
		bucketKey: bucketKey{Symbol: k.Symbol, Interval: k.Interval, OpenTime: k.OpenTime.UnixMilli()},
		CloseTime: k.CloseTime.UnixMilli(), Open: k.Open, High: k.High, Low: k.Low, Close: k.Close,
		Volume: k.Volume, QuoteVolume: k.QuoteVolume, Count: k.TradeCount,
	}
}
