package api

import (
	"time"

	identityv1 "simple-securities/gen/identity/v1"
	marketdatav1 "simple-securities/gen/marketdata/v1"
	orderv1 "simple-securities/gen/order/v1"
	walletv1 "simple-securities/gen/wallet/v1"
	"simple-securities/pkg/events"
)

// User is the public user.
type User struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
}

// Auth is the response of register/login/refresh.
type Auth struct {
	User         User   `json:"user"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
}

// Order is the public order shape.
type Order struct {
	OrderID             string `json:"orderId"`
	ClientOrderID       string `json:"clientOrderId"`
	Symbol              string `json:"symbol"`
	Side                string `json:"side"`
	Type                string `json:"type"`
	TimeInForce         string `json:"timeInForce"`
	Price               string `json:"price"`
	OrigQty             string `json:"origQty"`
	QuoteOrderQty       string `json:"quoteOrderQty"`
	ExecutedQty         string `json:"executedQty"`
	CummulativeQuoteQty string `json:"cummulativeQuoteQty"`
	Status              string `json:"status"`
	RejectReason        string `json:"rejectReason"`
	Time                int64  `json:"time"`
	UpdateTime          int64  `json:"updateTime"`
}

// MyTrade is a fill of the authenticated user.
type MyTrade struct {
	ID              int64  `json:"id"`
	OrderID         string `json:"orderId"`
	Symbol          string `json:"symbol"`
	Side            string `json:"side"`
	Price           string `json:"price"`
	Qty             string `json:"qty"`
	QuoteQty        string `json:"quoteQty"`
	Commission      string `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
	IsMaker         bool   `json:"isMaker"`
	Time            int64  `json:"time"`
}

// PublicTrade is an anonymous market trade.
type PublicTrade struct {
	ID           int64  `json:"id"`
	Price        string `json:"price"`
	Qty          string `json:"qty"`
	QuoteQty     string `json:"quoteQty"`
	Time         int64  `json:"time"`
	IsBuyerMaker bool   `json:"isBuyerMaker"`
}

// Ticker is the 24h rolling ticker.
type Ticker struct {
	Symbol             string `json:"symbol"`
	LastPrice          string `json:"lastPrice"`
	OpenPrice          string `json:"openPrice"`
	HighPrice          string `json:"highPrice"`
	LowPrice           string `json:"lowPrice"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
	PriceChange        string `json:"priceChange"`
	PriceChangePercent string `json:"priceChangePercent"`
	Count              int64  `json:"count"`
	CloseTime          int64  `json:"closeTime"`
}

// SymbolInfo is one entry of exchangeInfo.
type SymbolInfo struct {
	Symbol      string `json:"symbol"`
	BaseAsset   string `json:"baseAsset"`
	QuoteAsset  string `json:"quoteAsset"`
	TickSize    string `json:"tickSize"`
	StepSize    string `json:"stepSize"`
	MinNotional string `json:"minNotional"`
	MakerFee    string `json:"makerFee"`
	TakerFee    string `json:"takerFee"`
	Status      string `json:"status"`
}

// ExchangeInfo lists the tradable symbols.
type ExchangeInfo struct {
	ServerTime int64        `json:"serverTime"`
	Symbols    []SymbolInfo `json:"symbols"`
}

// Depth is an order book snapshot; levels are [price, qty].
type Depth struct {
	LastUpdateID int64       `json:"lastUpdateId"`
	Bids         [][2]string `json:"bids"`
	Asks         [][2]string `json:"asks"`
}

// Balance is one asset balance.
type Balance struct {
	Asset  string `json:"asset"`
	Free   string `json:"free"`
	Locked string `json:"locked"`
}

// Account holds the user's balances.
type Account struct {
	Balances []Balance `json:"balances"`
}

// LedgerEntry is an immutable balance movement.
type LedgerEntry struct {
	ID          int64  `json:"id"`
	Asset       string `json:"asset"`
	DeltaFree   string `json:"deltaFree"`
	DeltaLocked string `json:"deltaLocked"`
	FreeAfter   string `json:"freeAfter"`
	LockedAfter string `json:"lockedAfter"`
	RefType     string `json:"refType"`
	RefID       string `json:"refId"`
	Time        int64  `json:"time"`
}

// Millis converts an RFC3339 timestamp to epoch milliseconds (0 when empty or invalid).
func Millis(s string) int64 {
	if s == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

// AuthFrom maps an identity response.
func AuthFrom(r *identityv1.AuthResponse) Auth {
	return Auth{
		User:         User{UserID: r.GetUser().GetUserId(), Email: r.GetUser().GetEmail()},
		AccessToken:  r.GetAccessToken(),
		RefreshToken: r.GetRefreshToken(),
		ExpiresIn:    r.GetAccessExpiresIn(),
	}
}

// OrderFrom maps an order.
func OrderFrom(o *orderv1.Order) Order {
	return Order{
		OrderID:             o.GetOrderId(),
		ClientOrderID:       o.GetClientOrderId(),
		Symbol:              o.GetSymbol(),
		Side:                o.GetSide(),
		Type:                o.GetType(),
		TimeInForce:         o.GetTimeInForce(),
		Price:               o.GetPrice(),
		OrigQty:             o.GetQuantity(),
		QuoteOrderQty:       o.GetQuoteQuantity(),
		ExecutedQty:         o.GetExecutedQty(),
		CummulativeQuoteQty: o.GetCumQuoteQty(),
		Status:              o.GetStatus(),
		RejectReason:        o.GetRejectReason(),
		Time:                Millis(o.GetCreatedAt()),
		UpdateTime:          Millis(o.GetUpdatedAt()),
	}
}

// OrdersFrom maps a list of orders (never nil).
func OrdersFrom(orders []*orderv1.Order) []Order {
	out := make([]Order, 0, len(orders))
	for _, o := range orders {
		out = append(out, OrderFrom(o))
	}
	return out
}

// OrderFromEvent maps an order.updated event. The event carries no creation time, so time is the
// update time.
func OrderFromEvent(e events.OrderUpdated) Order {
	ms := e.UpdatedAt.UnixMilli()
	return Order{
		OrderID:             e.OrderID,
		ClientOrderID:       e.ClientOrderID,
		Symbol:              e.Symbol,
		Side:                e.Side,
		Type:                e.Type,
		TimeInForce:         e.TimeInForce,
		Price:               e.Price,
		OrigQty:             e.Quantity,
		QuoteOrderQty:       e.QuoteQuantity,
		ExecutedQty:         e.ExecutedQty,
		CummulativeQuoteQty: e.CumQuoteQty,
		Status:              e.Status,
		RejectReason:        e.RejectReason,
		Time:                ms,
		UpdateTime:          ms,
	}
}

// MyTradesFrom maps user trades (never nil).
func MyTradesFrom(ts []*orderv1.Trade) []MyTrade {
	out := make([]MyTrade, 0, len(ts))
	for _, t := range ts {
		out = append(out, MyTrade{
			ID:              t.GetTradeId(),
			OrderID:         t.GetOrderId(),
			Symbol:          t.GetSymbol(),
			Side:            t.GetSide(),
			Price:           t.GetPrice(),
			Qty:             t.GetQty(),
			QuoteQty:        t.GetQuoteQty(),
			Commission:      t.GetCommission(),
			CommissionAsset: t.GetCommissionAsset(),
			IsMaker:         t.GetIsMaker(),
			Time:            Millis(t.GetTime()),
		})
	}
	return out
}

// PublicTradesFrom maps market trades (never nil).
func PublicTradesFrom(ts []*marketdatav1.PublicTrade) []PublicTrade {
	out := make([]PublicTrade, 0, len(ts))
	for _, t := range ts {
		out = append(out, PublicTrade{
			ID:           t.GetTradeId(),
			Price:        t.GetPrice(),
			Qty:          t.GetQty(),
			QuoteQty:     t.GetQuoteQty(),
			Time:         t.GetTime(),
			IsBuyerMaker: t.GetIsBuyerMaker(),
		})
	}
	return out
}

// TickerFrom maps a ticker.
func TickerFrom(t *marketdatav1.Ticker) Ticker {
	return Ticker{
		Symbol:             t.GetSymbol(),
		LastPrice:          t.GetLastPrice(),
		OpenPrice:          t.GetOpenPrice(),
		HighPrice:          t.GetHighPrice(),
		LowPrice:           t.GetLowPrice(),
		Volume:             t.GetVolume(),
		QuoteVolume:        t.GetQuoteVolume(),
		PriceChange:        t.GetPriceChange(),
		PriceChangePercent: t.GetPriceChangePercent(),
		Count:              t.GetTradeCount(),
		CloseTime:          t.GetCloseTime(),
	}
}

// TickersFrom maps tickers (never nil).
func TickersFrom(ts []*marketdatav1.Ticker) []Ticker {
	out := make([]Ticker, 0, len(ts))
	for _, t := range ts {
		out = append(out, TickerFrom(t))
	}
	return out
}

// KlinesFrom maps klines to Binance arrays
// [openTime, open, high, low, close, volume, closeTime, quoteVolume, trades].
func KlinesFrom(ks []*marketdatav1.Kline) [][]any {
	out := make([][]any, 0, len(ks))
	for _, k := range ks {
		out = append(out, []any{k.GetOpenTime(), k.GetOpen(), k.GetHigh(), k.GetLow(), k.GetClose(),
			k.GetVolume(), k.GetCloseTime(), k.GetQuoteVolume(), k.GetTradeCount()})
	}
	return out
}

// ExchangeInfoFrom maps exchange info.
func ExchangeInfoFrom(e *marketdatav1.ExchangeInfo) ExchangeInfo {
	out := ExchangeInfo{ServerTime: e.GetServerTime(), Symbols: make([]SymbolInfo, 0, len(e.GetSymbols()))}
	if out.ServerTime == 0 {
		out.ServerTime = time.Now().UnixMilli()
	}
	for _, s := range e.GetSymbols() {
		out.Symbols = append(out.Symbols, SymbolInfo{
			Symbol:      s.GetSymbol(),
			BaseAsset:   s.GetBaseAsset(),
			QuoteAsset:  s.GetQuoteAsset(),
			TickSize:    s.GetTickSize(),
			StepSize:    s.GetStepSize(),
			MinNotional: s.GetMinNotional(),
			MakerFee:    s.GetMakerFee(),
			TakerFee:    s.GetTakerFee(),
			Status:      s.GetStatus(),
		})
	}
	return out
}

// DepthFrom maps a depth snapshot.
func DepthFrom(d *marketdatav1.Depth) Depth {
	levels := func(ls []*marketdatav1.PriceLevel) [][2]string {
		out := make([][2]string, 0, len(ls))
		for _, l := range ls {
			out = append(out, [2]string{l.GetPrice(), l.GetQty()})
		}
		return out
	}
	return Depth{LastUpdateID: d.GetLastUpdateId(), Bids: levels(d.GetBids()), Asks: levels(d.GetAsks())}
}

// AccountFrom maps balances.
func AccountFrom(r *walletv1.GetBalancesResponse) Account {
	out := Account{Balances: make([]Balance, 0, len(r.GetBalances()))}
	for _, b := range r.GetBalances() {
		out.Balances = append(out.Balances, Balance{Asset: b.GetAsset(), Free: b.GetFree(), Locked: b.GetLocked()})
	}
	return out
}

// LedgerEntryFrom maps a ledger entry.
func LedgerEntryFrom(e *walletv1.LedgerEntry) LedgerEntry {
	return LedgerEntry{
		ID:          e.GetId(),
		Asset:       e.GetAsset(),
		DeltaFree:   e.GetDeltaFree(),
		DeltaLocked: e.GetDeltaLocked(),
		FreeAfter:   e.GetFreeAfter(),
		LockedAfter: e.GetLockedAfter(),
		RefType:     e.GetRefType(),
		RefID:       e.GetRefId(),
		Time:        Millis(e.GetCreatedAt()),
	}
}

// LedgerFrom maps ledger entries (never nil).
func LedgerFrom(es []*walletv1.LedgerEntry) []LedgerEntry {
	out := make([]LedgerEntry, 0, len(es))
	for _, e := range es {
		out = append(out, LedgerEntryFrom(e))
	}
	return out
}
