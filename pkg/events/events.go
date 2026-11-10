// Package events is the single source of truth for the asynchronous contract between services:
// topic names, the envelope written to the outbox and every event payload.
package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Topics. Transactional topics are written through the outbox; market.* are derived,
// best-effort streams published directly (losing one is harmless, the next snapshot heals it).
const (
	TopicUserEvents     = "identity.user.events"  // key: user_id
	TopicOrderCommands  = "order.commands"        // key: symbol  -> matching
	TopicMatchingEvents = "matching.events"       // key: symbol  -> order, wallet, marketdata, gateway
	TopicOrderUpdates   = "order.updates"         // key: user_id -> gateway (executionReport)
	TopicWalletEvents   = "wallet.balance.events" // key: user_id -> gateway (account update)
	TopicMarketDepth    = "market.depth"          // key: symbol (direct, throttled)
	TopicMarketTicker   = "market.ticker"         // key: symbol (direct, throttled)
	TopicMarketKline    = "market.kline"          // key: symbol (direct, throttled)
)

// Futures topics (USDⓈ-M perpetuals). The futures market reuses the matching and marketdata
// services (deployed with MARKET=futures) on these topics.
const (
	TopicFuturesOrderCommands  = "futures.order.commands"  // key: symbol  -> matching (futures)
	TopicFuturesMatchingEvents = "futures.matching.events" // key: symbol  -> futures, marketdata (futures), gateway
	TopicFuturesOrderUpdates   = "futures.order.updates"   // key: user_id -> gateway (ORDER_TRADE_UPDATE)
	TopicFuturesAccountEvents  = "futures.account.events"  // key: user_id -> gateway (ACCOUNT_UPDATE)
	TopicFuturesMarketDepth    = "futures.market.depth"    // direct
	TopicFuturesMarketTicker   = "futures.market.ticker"   // direct
	TopicFuturesMarketKline    = "futures.market.kline"    // direct
	TopicFuturesMarkPrice      = "futures.mark_price"      // direct, key: symbol (mark/index/funding)
	TopicWalletTransfers       = "wallet.transfers"        // key: user_id -> futures (spot -> futures)
	TopicFuturesTransfers      = "futures.transfers"       // key: user_id -> wallet  (futures -> spot)
)

// MarketTopics groups the per-market topics used by matching, marketdata and gateway.
type MarketTopics struct {
	OrderCommands, MatchingEvents, Depth, Ticker, Kline string
}

// TopicsFor returns the topics of a market ("spot" | "futures").
func TopicsFor(market string) MarketTopics {
	if market == "futures" {
		return MarketTopics{TopicFuturesOrderCommands, TopicFuturesMatchingEvents, TopicFuturesMarketDepth,
			TopicFuturesMarketTicker, TopicFuturesMarketKline}
	}
	return MarketTopics{TopicOrderCommands, TopicMatchingEvents, TopicMarketDepth, TopicMarketTicker, TopicMarketKline}
}

// Partitions is the partition count of every topic. Services create ALL topics at startup so the
// start order of services does not matter (matching relies on this count for book preloading).
const Partitions int32 = 6

// AllTopics returns every topic of the platform with its partition count.
func AllTopics() map[string]int32 {
	out := map[string]int32{}
	for _, t := range []string{TopicUserEvents, TopicOrderCommands, TopicMatchingEvents, TopicOrderUpdates,
		TopicWalletEvents, TopicMarketDepth, TopicMarketTicker, TopicMarketKline,
		TopicFuturesOrderCommands, TopicFuturesMatchingEvents, TopicFuturesOrderUpdates, TopicFuturesAccountEvents,
		TopicFuturesMarketDepth, TopicFuturesMarketTicker, TopicFuturesMarketKline, TopicFuturesMarkPrice,
		TopicWalletTransfers, TopicFuturesTransfers} {
		out[t] = Partitions
	}
	return out
}

// Event types.
const (
	TypeUserRegistered       = "user.registered"
	TypeOrderAccepted        = "order.accepted"
	TypeOrderCancelRequested = "order.cancel_requested"
	TypeTradeExecuted        = "trade.executed"
	TypeOrderStatusChanged   = "order.status_changed"
	TypeOrderUpdated         = "order.updated"
	TypeBalanceUpdated       = "balance.updated"
	TypeDepthSnapshot        = "depth.snapshot"
	TypeTicker               = "ticker.24hr"
	TypeKline                = "kline"

	TypeSpotToFutures        = "transfer.spot_to_futures"
	TypeFuturesToSpot        = "transfer.futures_to_spot"
	TypeFuturesAccountUpdate = "futures.account_update"
	TypeMarkPrice            = "mark_price"
)

// Envelope wraps every message on Kafka. ID is the idempotency key consumers store in inbox_events.
type Envelope struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Source      string          `json:"source"`
	AggregateID string          `json:"aggregate_id"`
	OccurredAt  time.Time       `json:"occurred_at"`
	Payload     json.RawMessage `json:"payload"`
}

// New builds an envelope with a fresh UUIDv7 (time ordered, index friendly).
func New(source, typ, aggregateID string, payload any) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        typ,
		Source:      source,
		AggregateID: aggregateID,
		OccurredAt:  time.Now().UTC(),
		Payload:     raw,
	}, nil
}

// Decode unmarshals the envelope payload into v.
func (e Envelope) Decode(v any) error { return json.Unmarshal(e.Payload, v) }

// ---- identity ----

type UserRegistered struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

// ---- order -> matching ----

// OrderAccepted is the command for the matching engine. Decimals are strings.
type OrderAccepted struct {
	OrderID       string    `json:"order_id"`
	ClientOrderID string    `json:"client_order_id"`
	UserID        string    `json:"user_id"`
	Symbol        string    `json:"symbol"`
	Side          string    `json:"side"`          // BUY | SELL
	Type          string    `json:"type"`          // LIMIT | MARKET | LIMIT_MAKER
	TimeInForce   string    `json:"time_in_force"` // GTC | IOC | FOK
	Price         string    `json:"price"`
	Quantity      string    `json:"quantity"`
	QuoteQuantity string    `json:"quote_quantity"`        // spot MARKET BUY budget; empty = quantity based
	ReduceOnly    bool      `json:"reduce_only,omitempty"` // futures (enforced by the futures service)
	Liquidation   bool      `json:"liquidation,omitempty"` // futures liquidation order
	CreatedAt     time.Time `json:"created_at"`
}

type OrderCancelRequested struct {
	OrderID string `json:"order_id"`
	UserID  string `json:"user_id"`
	Symbol  string `json:"symbol"`
}

// ---- matching -> * ----

// TradeExecuted is emitted once per fill. Fees are computed by the engine so every consumer agrees.
// Spot: buyer pays fee in base asset, seller in quote asset. Futures: both pay in the quote asset.
type TradeExecuted struct {
	TradeID       int64     `json:"trade_id"`
	Seq           int64     `json:"seq"`
	Symbol        string    `json:"symbol"`
	Price         string    `json:"price"`
	Qty           string    `json:"qty"`
	QuoteQty      string    `json:"quote_qty"`
	BuyerOrderID  string    `json:"buyer_order_id"`
	SellerOrderID string    `json:"seller_order_id"`
	BuyerUserID   string    `json:"buyer_user_id"`
	SellerUserID  string    `json:"seller_user_id"`
	BuyerIsMaker  bool      `json:"buyer_is_maker"`
	BuyerFee      string    `json:"buyer_fee"`  // spot: base asset, futures: quote asset
	SellerFee     string    `json:"seller_fee"` // quote asset
	Time          time.Time `json:"time"`
}

// OrderStatusChanged is emitted for every order touched by a command, after its trades.
type OrderStatusChanged struct {
	Seq         int64     `json:"seq"`
	OrderID     string    `json:"order_id"`
	UserID      string    `json:"user_id"`
	Symbol      string    `json:"symbol"`
	Status      string    `json:"status"` // NEW | PARTIALLY_FILLED | FILLED | CANCELED | EXPIRED | REJECTED
	ExecutedQty string    `json:"executed_qty"`
	CumQuoteQty string    `json:"cum_quote_qty"`
	Reason      string    `json:"reason,omitempty"`
	Time        time.Time `json:"time"`
}

// IsTerminal reports whether no further fills can happen for the order.
func IsTerminal(status string) bool {
	switch status {
	case "FILLED", "CANCELED", "EXPIRED", "REJECTED":
		return true
	}
	return false
}

// ---- order -> gateway ----

type OrderUpdated struct {
	OrderID       string    `json:"order_id"`
	ClientOrderID string    `json:"client_order_id"`
	UserID        string    `json:"user_id"`
	Symbol        string    `json:"symbol"`
	Side          string    `json:"side"`
	Type          string    `json:"type"`
	TimeInForce   string    `json:"time_in_force"`
	Price         string    `json:"price"`
	Quantity      string    `json:"quantity"`
	QuoteQuantity string    `json:"quote_quantity"`
	ExecutedQty   string    `json:"executed_qty"`
	CumQuoteQty   string    `json:"cum_quote_qty"`
	Status        string    `json:"status"`
	RejectReason  string    `json:"reject_reason,omitempty"`
	LastTradeID   int64     `json:"last_trade_id,omitempty"`
	ReduceOnly    bool      `json:"reduce_only,omitempty"`  // futures
	Liquidation   bool      `json:"liquidation,omitempty"`  // futures
	AvgPrice      string    `json:"avg_price,omitempty"`    // futures
	RealizedPnl   string    `json:"realized_pnl,omitempty"` // futures: cumulative realized PnL of this order
	UpdatedAt     time.Time `json:"updated_at"`
}

// ---- wallet -> gateway ----

type BalanceItem struct {
	Asset  string `json:"asset"`
	Free   string `json:"free"`
	Locked string `json:"locked"`
}

type BalanceUpdated struct {
	UserID   string        `json:"user_id"`
	Balances []BalanceItem `json:"balances"`
	Reason   string        `json:"reason"`
	Time     time.Time     `json:"time"`
}

// ---- market streams (direct) ----

type PriceLevel [2]string // [price, qty]

type DepthSnapshot struct {
	Symbol       string       `json:"symbol"`
	LastUpdateID int64        `json:"last_update_id"`
	Bids         []PriceLevel `json:"bids"`
	Asks         []PriceLevel `json:"asks"`
	Time         time.Time    `json:"time"`
}

type Ticker struct {
	Symbol             string `json:"symbol"`
	LastPrice          string `json:"last_price"`
	OpenPrice          string `json:"open_price"`
	HighPrice          string `json:"high_price"`
	LowPrice           string `json:"low_price"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quote_volume"`
	PriceChange        string `json:"price_change"`
	PriceChangePercent string `json:"price_change_percent"`
	TradeCount         int64  `json:"trade_count"`
	CloseTime          int64  `json:"close_time"`
}

type Kline struct {
	Symbol      string `json:"symbol"`
	Interval    string `json:"interval"`
	OpenTime    int64  `json:"open_time"`
	CloseTime   int64  `json:"close_time"`
	Open        string `json:"open"`
	High        string `json:"high"`
	Low         string `json:"low"`
	Close       string `json:"close"`
	Volume      string `json:"volume"`
	QuoteVolume string `json:"quote_volume"`
	TradeCount  int64  `json:"trade_count"`
}

// ---- spot <-> futures transfers ----

// Transfer moves collateral between the spot wallet and the futures account (USDT).
type Transfer struct {
	TransferID string    `json:"transfer_id"`
	UserID     string    `json:"user_id"`
	Asset      string    `json:"asset"`
	Amount     string    `json:"amount"`
	Time       time.Time `json:"time"`
}

// ---- futures -> gateway ----

type FuturesPositionItem struct {
	Symbol         string `json:"symbol"`
	PositionAmt    string `json:"position_amt"` // signed: + long, - short
	EntryPrice     string `json:"entry_price"`
	MarginType     string `json:"margin_type"` // CROSSED | ISOLATED
	IsolatedMargin string `json:"isolated_margin"`
	Leverage       int    `json:"leverage"`
}

type FuturesAccountUpdate struct {
	UserID           string                `json:"user_id"`
	Reason           string                `json:"reason"` // ORDER | FUNDING_FEE | DEPOSIT | WITHDRAW | MARGIN_TRANSFER | LIQUIDATION | ADJUSTMENT
	WalletBalance    string                `json:"wallet_balance"`
	AvailableBalance string                `json:"available_balance"`
	Positions        []FuturesPositionItem `json:"positions"`
	Time             time.Time             `json:"time"`
}

// MarkPrice is published every second per contract by the futures service.
type MarkPrice struct {
	Symbol          string `json:"symbol"`
	MarkPrice       string `json:"mark_price"`
	IndexPrice      string `json:"index_price"`
	FundingRate     string `json:"funding_rate"` // estimated rate for the next settlement
	NextFundingTime int64  `json:"next_funding_time"`
	Time            int64  `json:"time"`
}
