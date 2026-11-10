package api

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	futuresv1 "simple-securities/gen/futures/v1"
	"simple-securities/pkg/apperr"
	"simple-securities/pkg/events"
	"simple-securities/pkg/symbols"
)

// ---------------------------------------------------------------- futures shapes (/fapi/v1)

// FuturesOrder is the public futures order shape.
type FuturesOrder struct {
	OrderID       string `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	Type          string `json:"type"`
	TimeInForce   string `json:"timeInForce"`
	Price         string `json:"price"`
	OrigQty       string `json:"origQty"`
	ExecutedQty   string `json:"executedQty"`
	CumQuote      string `json:"cumQuote"`
	AvgPrice      string `json:"avgPrice"`
	ReduceOnly    bool   `json:"reduceOnly"`
	Liquidation   bool   `json:"liquidation"`
	Status        string `json:"status"`
	RejectReason  string `json:"rejectReason"`
	RealizedPnl   string `json:"realizedPnl"`
	Time          int64  `json:"time"`
	UpdateTime    int64  `json:"updateTime"`
}

// Position is a futures position (one-way mode).
type Position struct {
	Symbol           string `json:"symbol"`
	PositionAmt      string `json:"positionAmt"`
	EntryPrice       string `json:"entryPrice"`
	BreakEvenPrice   string `json:"breakEvenPrice"`
	MarkPrice        string `json:"markPrice"`
	UnRealizedProfit string `json:"unRealizedProfit"`
	LiquidationPrice string `json:"liquidationPrice"`
	Leverage         int32  `json:"leverage"`
	MarginType       string `json:"marginType"` // cross | isolated
	IsolatedMargin   string `json:"isolatedMargin"`
	Notional         string `json:"notional"`
	InitialMargin    string `json:"initialMargin"`
	MaintMargin      string `json:"maintMargin"`
	UpdateTime       int64  `json:"updateTime"`
}

// FuturesAccount is the futures collateral account.
type FuturesAccount struct {
	Asset                string     `json:"asset"`
	WalletBalance        string     `json:"walletBalance"`
	UnrealizedProfit     string     `json:"unrealizedProfit"`
	MarginBalance        string     `json:"marginBalance"`
	AvailableBalance     string     `json:"availableBalance"`
	MaxWithdrawAmount    string     `json:"maxWithdrawAmount"`
	TotalInitialMargin   string     `json:"totalInitialMargin"`
	TotalMaintMargin     string     `json:"totalMaintMargin"`
	TotalOpenOrderMargin string     `json:"totalOpenOrderMargin"`
	Positions            []Position `json:"positions"`
}

// FuturesUserTrade is a futures fill of the authenticated user.
type FuturesUserTrade struct {
	ID              int64  `json:"id"`
	OrderID         string `json:"orderId"`
	Symbol          string `json:"symbol"`
	Side            string `json:"side"`
	Price           string `json:"price"`
	Qty             string `json:"qty"`
	QuoteQty        string `json:"quoteQty"`
	RealizedPnl     string `json:"realizedPnl"`
	Commission      string `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
	Maker           bool   `json:"maker"`
	Time            int64  `json:"time"`
}

// Income is one futures income record.
type Income struct {
	ID         int64  `json:"id"`
	Symbol     string `json:"symbol"`
	IncomeType string `json:"incomeType"`
	Income     string `json:"income"`
	Asset      string `json:"asset"`
	Info       string `json:"info"`
	Time       int64  `json:"time"`
}

// PremiumIndex is the mark/index price and funding of a contract.
type PremiumIndex struct {
	Symbol          string `json:"symbol"`
	MarkPrice       string `json:"markPrice"`
	IndexPrice      string `json:"indexPrice"`
	LastFundingRate string `json:"lastFundingRate"`
	NextFundingTime int64  `json:"nextFundingTime"`
	Time            int64  `json:"time"`
}

// FundingRate is a settled funding rate.
type FundingRate struct {
	Symbol      string `json:"symbol"`
	FundingRate string `json:"fundingRate"`
	FundingTime int64  `json:"fundingTime"`
	MarkPrice   string `json:"markPrice"`
}

// FuturesSymbolInfo is one contract of the futures exchangeInfo.
type FuturesSymbolInfo struct {
	Symbol          string `json:"symbol"`
	BaseAsset       string `json:"baseAsset"`
	QuoteAsset      string `json:"quoteAsset"`
	ContractType    string `json:"contractType"`
	TickSize        string `json:"tickSize"`
	StepSize        string `json:"stepSize"`
	MinNotional     string `json:"minNotional"`
	MakerFee        string `json:"makerFee"`
	TakerFee        string `json:"takerFee"`
	MaxLeverage     int    `json:"maxLeverage"`
	MaintMarginRate string `json:"maintMarginRate"`
	Underlying      string `json:"underlying"`
	Status          string `json:"status"`
}

// FuturesExchangeInfo lists the tradable contracts.
type FuturesExchangeInfo struct {
	ServerTime int64               `json:"serverTime"`
	Symbols    []FuturesSymbolInfo `json:"symbols"`
}

// LeverageResult is the response of POST /fapi/v1/leverage.
type LeverageResult struct {
	Symbol      string `json:"symbol"`
	Leverage    int32  `json:"leverage"`
	MaxNotional string `json:"maxNotional"`
}

// MarginTypeResult is the response of POST /fapi/v1/marginType.
type MarginTypeResult struct {
	Symbol     string `json:"symbol"`
	MarginType string `json:"marginType"`
}

// TransferResult is the response of POST /fapi/v1/transfer.
type TransferResult struct {
	TranID string `json:"tranId"`
	Asset  string `json:"asset"`
	Amount string `json:"amount"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// Transfer directions.
const (
	TransferSpotToFutures = "SPOT_TO_FUTURES"
	TransferFuturesToSpot = "FUTURES_TO_SPOT"
)

// PublicMarginType maps the upstream margin type (CROSSED | ISOLATED) to "cross" | "isolated".
func PublicMarginType(s string) string {
	switch strings.ToUpper(s) {
	case "CROSSED", "CROSS":
		return "cross"
	case "ISOLATED":
		return "isolated"
	}
	return strings.ToLower(s)
}

// ParseMarginType accepts ISOLATED | CROSSED (and CROSS), case-insensitively, and returns the
// upstream value.
func ParseMarginType(s string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "ISOLATED":
		return "ISOLATED", nil
	case "CROSSED", "CROSS":
		return "CROSSED", nil
	}
	return "", apperr.Invalid("INVALID_MARGIN_TYPE", "marginType must be ISOLATED or CROSSED")
}

// FuturesOrderFrom maps a futures order.
func FuturesOrderFrom(o *futuresv1.Order) FuturesOrder {
	return FuturesOrder{
		OrderID:       o.GetOrderId(),
		ClientOrderID: o.GetClientOrderId(),
		Symbol:        o.GetSymbol(),
		Side:          o.GetSide(),
		Type:          o.GetType(),
		TimeInForce:   o.GetTimeInForce(),
		Price:         o.GetPrice(),
		OrigQty:       o.GetQuantity(),
		ExecutedQty:   o.GetExecutedQty(),
		CumQuote:      o.GetCumQuote(),
		AvgPrice:      o.GetAvgPrice(),
		ReduceOnly:    o.GetReduceOnly(),
		Liquidation:   o.GetLiquidation(),
		Status:        o.GetStatus(),
		RejectReason:  o.GetRejectReason(),
		RealizedPnl:   o.GetRealizedPnl(),
		Time:          Millis(o.GetCreatedAt()),
		UpdateTime:    Millis(o.GetUpdatedAt()),
	}
}

// FuturesOrdersFrom maps futures orders (never nil).
func FuturesOrdersFrom(os []*futuresv1.Order) []FuturesOrder {
	out := make([]FuturesOrder, 0, len(os))
	for _, o := range os {
		out = append(out, FuturesOrderFrom(o))
	}
	return out
}

// FuturesOrderFromEvent maps an order.updated event of the futures market. The event carries no
// creation time, so time is the update time.
func FuturesOrderFromEvent(e events.OrderUpdated) FuturesOrder {
	ms := e.UpdatedAt.UnixMilli()
	return FuturesOrder{
		OrderID:       e.OrderID,
		ClientOrderID: e.ClientOrderID,
		Symbol:        e.Symbol,
		Side:          e.Side,
		Type:          e.Type,
		TimeInForce:   e.TimeInForce,
		Price:         e.Price,
		OrigQty:       e.Quantity,
		ExecutedQty:   e.ExecutedQty,
		CumQuote:      e.CumQuoteQty,
		AvgPrice:      e.AvgPrice,
		ReduceOnly:    e.ReduceOnly,
		Liquidation:   e.Liquidation,
		Status:        e.Status,
		RejectReason:  e.RejectReason,
		RealizedPnl:   e.RealizedPnl,
		Time:          ms,
		UpdateTime:    ms,
	}
}

// PositionFrom maps a position.
func PositionFrom(p *futuresv1.Position) Position {
	return Position{
		Symbol:           p.GetSymbol(),
		PositionAmt:      p.GetPositionAmt(),
		EntryPrice:       p.GetEntryPrice(),
		BreakEvenPrice:   p.GetBreakEvenPrice(),
		MarkPrice:        p.GetMarkPrice(),
		UnRealizedProfit: p.GetUnrealizedPnl(),
		LiquidationPrice: p.GetLiquidationPrice(),
		Leverage:         p.GetLeverage(),
		MarginType:       PublicMarginType(p.GetMarginType()),
		IsolatedMargin:   p.GetIsolatedMargin(),
		Notional:         p.GetNotional(),
		InitialMargin:    p.GetInitialMargin(),
		MaintMargin:      p.GetMaintMargin(),
		UpdateTime:       Millis(p.GetUpdateTime()),
	}
}

// PositionsFrom maps positions (never nil).
func PositionsFrom(ps []*futuresv1.Position) []Position {
	out := make([]Position, 0, len(ps))
	for _, p := range ps {
		out = append(out, PositionFrom(p))
	}
	return out
}

// FuturesAccountFrom maps the futures account.
func FuturesAccountFrom(a *futuresv1.Account) FuturesAccount {
	return FuturesAccount{
		Asset:                a.GetAsset(),
		WalletBalance:        a.GetWalletBalance(),
		UnrealizedProfit:     a.GetUnrealizedPnl(),
		MarginBalance:        a.GetMarginBalance(),
		AvailableBalance:     a.GetAvailableBalance(),
		MaxWithdrawAmount:    a.GetMaxWithdrawAmount(),
		TotalInitialMargin:   a.GetTotalInitialMargin(),
		TotalMaintMargin:     a.GetTotalMaintMargin(),
		TotalOpenOrderMargin: a.GetTotalOpenOrderMargin(),
		Positions:            PositionsFrom(a.GetPositions()),
	}
}

// FuturesUserTradesFrom maps futures fills (never nil).
func FuturesUserTradesFrom(ts []*futuresv1.Trade) []FuturesUserTrade {
	out := make([]FuturesUserTrade, 0, len(ts))
	for _, t := range ts {
		out = append(out, FuturesUserTrade{
			ID:              t.GetTradeId(),
			OrderID:         t.GetOrderId(),
			Symbol:          t.GetSymbol(),
			Side:            t.GetSide(),
			Price:           t.GetPrice(),
			Qty:             t.GetQty(),
			QuoteQty:        t.GetQuoteQty(),
			RealizedPnl:     t.GetRealizedPnl(),
			Commission:      t.GetCommission(),
			CommissionAsset: t.GetCommissionAsset(),
			Maker:           t.GetIsMaker(),
			Time:            Millis(t.GetTime()),
		})
	}
	return out
}

// IncomeFrom maps income records (never nil).
func IncomeFrom(is []*futuresv1.Income) []Income {
	out := make([]Income, 0, len(is))
	for _, i := range is {
		out = append(out, Income{
			ID:         i.GetId(),
			Symbol:     i.GetSymbol(),
			IncomeType: i.GetIncomeType(),
			Income:     i.GetIncome(),
			Asset:      i.GetAsset(),
			Info:       i.GetInfo(),
			Time:       Millis(i.GetTime()),
		})
	}
	return out
}

// PremiumIndexFrom maps a mark price.
func PremiumIndexFrom(m *futuresv1.MarkPrice) PremiumIndex {
	return PremiumIndex{
		Symbol:          m.GetSymbol(),
		MarkPrice:       m.GetMarkPrice(),
		IndexPrice:      m.GetIndexPrice(),
		LastFundingRate: m.GetLastFundingRate(),
		NextFundingTime: m.GetNextFundingTime(),
		Time:            m.GetTime(),
	}
}

// PremiumIndexesFrom maps mark prices (never nil).
func PremiumIndexesFrom(ms []*futuresv1.MarkPrice) []PremiumIndex {
	out := make([]PremiumIndex, 0, len(ms))
	for _, m := range ms {
		out = append(out, PremiumIndexFrom(m))
	}
	return out
}

// FundingRatesFrom maps funding rates (never nil).
func FundingRatesFrom(rs []*futuresv1.FundingRate) []FundingRate {
	out := make([]FundingRate, 0, len(rs))
	for _, r := range rs {
		out = append(out, FundingRate{Symbol: r.GetSymbol(), FundingRate: r.GetFundingRate(),
			FundingTime: r.GetFundingTime(), MarkPrice: r.GetMarkPrice()})
	}
	return out
}

// FuturesExchangeInfoFrom builds the futures exchangeInfo from the contract registry.
func FuturesExchangeInfoFrom(syms []symbols.Symbol, now time.Time) FuturesExchangeInfo {
	out := FuturesExchangeInfo{ServerTime: now.UnixMilli(), Symbols: make([]FuturesSymbolInfo, 0, len(syms))}
	for _, s := range syms {
		out.Symbols = append(out.Symbols, FuturesSymbolInfo{
			Symbol:          s.Name,
			BaseAsset:       s.Base,
			QuoteAsset:      s.Quote,
			ContractType:    "PERPETUAL",
			TickSize:        s.TickSize.String(),
			StepSize:        s.StepSize.String(),
			MinNotional:     s.MinNotional.String(),
			MakerFee:        s.MakerFee.String(),
			TakerFee:        s.TakerFee.String(),
			MaxLeverage:     s.MaxLeverage,
			MaintMarginRate: s.MaintMarginRate.String(),
			Underlying:      s.Underlying,
			Status:          "TRADING",
		})
	}
	return out
}

// ---------------------------------------------------------------- lenient JSON scalars

// FlexBool decodes a JSON boolean or a "true"/"false" string.
type FlexBool bool

// UnmarshalJSON implements json.Unmarshaler.
func (b *FlexBool) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		if s == "" {
			*b = false
			return nil
		}
		v, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		*b = FlexBool(v)
		return nil
	}
	var v bool
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*b = FlexBool(v)
	return nil
}

// FlexInt decodes a JSON integer or a numeric string.
type FlexInt int64

// UnmarshalJSON implements json.Unmarshaler.
func (n *FlexInt) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		data = []byte(strings.TrimSpace(s))
	}
	v, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*n = FlexInt(v)
	return nil
}

// ---------------------------------------------------------------- futures orders

// FuturesPlaceOrderRequest is the public futures order request (POST /fapi/v1/order body and WS
// futures.order.place params).
type FuturesPlaceOrderRequest struct {
	Symbol           string   `json:"symbol"`
	Side             string   `json:"side"`
	Type             string   `json:"type"`
	TimeInForce      string   `json:"timeInForce"`
	Price            string   `json:"price"`
	Quantity         string   `json:"quantity"`
	ReduceOnly       FlexBool `json:"reduceOnly"`
	NewClientOrderID string   `json:"newClientOrderId"`
}

// ToProto validates the request shape and binds it to the authenticated user. Business rules
// (filters, margin, TIF combinations) are enforced by the futures service.
func (r FuturesPlaceOrderRequest) ToProto(userID string) (*futuresv1.PlaceOrderRequest, error) {
	sym, err := NormalizeSymbol(r.Symbol)
	if err != nil {
		return nil, err
	}
	side := strings.ToUpper(strings.TrimSpace(r.Side))
	if side != "BUY" && side != "SELL" {
		return nil, apperr.Invalid("INVALID_SIDE", "side must be BUY or SELL")
	}
	typ := strings.ToUpper(strings.TrimSpace(r.Type))
	if typ != "LIMIT" && typ != "MARKET" {
		return nil, apperr.Invalid("INVALID_ORDER_TYPE", "type must be LIMIT or MARKET")
	}
	if len(r.NewClientOrderID) > maxClientOrderIDLen {
		return nil, apperr.Invalid("INVALID_PARAMETER", "newClientOrderId longer than %d", maxClientOrderIDLen)
	}
	return &futuresv1.PlaceOrderRequest{
		UserId:        userID,
		Symbol:        sym,
		Side:          side,
		Type:          typ,
		TimeInForce:   strings.ToUpper(strings.TrimSpace(r.TimeInForce)),
		Price:         strings.TrimSpace(r.Price),
		Quantity:      strings.TrimSpace(r.Quantity),
		ReduceOnly:    bool(r.ReduceOnly),
		ClientOrderId: r.NewClientOrderID,
	}, nil
}
